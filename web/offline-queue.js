/* Offline queue shared by the form page (web/public.html) and the service worker
   (web/sw.js).

   Both need the exact same flush semantics: the service worker runs it from a
   Background Sync with no page open, the page runs it on the "online" event. Two
   copies of this logic would eventually disagree about when a record is retried
   versus given up on — and disagreeing about that means either lost answers or a
   queue that never drains. So it lives here and both sides load it (the worker via
   importScripts, the page via a <script> tag). `self` is the global in both.

   IndexedDB layout (version 2):
     "queue" — keyPath "id", autoIncrement
       {id, url, method, headers, body, ts, attempts}
       plus, once permanently rejected: {failed:true, status, error, failedAt}
     "files" — keyPath "id", autoIncrement
       {id, url, headers, blob, name, fieldType, ts, attempts}
       plus the same failure fields

   Version 1 had only "queue". The upgrade adds "files" and leaves existing queued
   answers untouched — a user mid-survey must not lose them to a deploy.

   A record marked `failed` is never retried and never deleted: it holds answers or
   a photo a user already captured, so it stays until a human decides. The page
   surfaces the counts so it cannot go unnoticed. */
(function (global) {
  "use strict";

  const DB_NAME = "eform-offline";
  const DB_VERSION = 2;
  const QUEUE = "queue";
  const FILES = "files";

  /* A photo cannot be uploaded while offline, but the answer still has to reference
     it. The answer stores this placeholder instead, and flush() swaps in the real
     URL once the file reaches the server. The scheme is deliberately odd-looking so
     that free text a respondent happens to type can never collide with it. */
  const LOCAL_PREFIX = "eform-local://";
  const LOCAL_RE = /eform-local:\/\/(\d+)/g;

  // Give up after this many attempts, or once a record is this old. Giving up means
  // marking it failed and moving on — never discarding it.
  const MAX_ATTEMPTS = 25;
  const MAX_AGE_MS = 30 * 24 * 60 * 60 * 1000;

  function openDB() {
    return new Promise((resolve, reject) => {
      const req = indexedDB.open(DB_NAME, DB_VERSION);
      req.onupgradeneeded = () => {
        const db = req.result;
        if (!db.objectStoreNames.contains(QUEUE)) {
          db.createObjectStore(QUEUE, { keyPath: "id", autoIncrement: true });
        }
        if (!db.objectStoreNames.contains(FILES)) {
          db.createObjectStore(FILES, { keyPath: "id", autoIncrement: true });
        }
      };
      req.onsuccess = () => resolve(req.result);
      req.onerror = () => reject(req.error);
    });
  }

  function runTx(db, store, mode, fn) {
    return new Promise((resolve, reject) => {
      const tx = db.transaction(store, mode);
      const req = fn(tx.objectStore(store));
      tx.oncomplete = () => resolve(req && "result" in req ? req.result : undefined);
      tx.onerror = () => reject(tx.error);
    });
  }

  const getAll = (db, store) => runTx(db, store, "readonly", (s) => s.getAll());
  const put = (db, store, rec) => runTx(db, store, "readwrite", (s) => s.put(rec));
  const del = (db, store, id) => runTx(db, store, "readwrite", (s) => s.delete(id));

  function add(db, store, rec) {
    return new Promise((resolve, reject) => {
      const tx = db.transaction(store, "readwrite");
      const req = tx.objectStore(store).add(rec);
      tx.oncomplete = () => resolve(req.result);
      tx.onerror = () => reject(tx.error);
    });
  }

  /* Store a file that could not be uploaded, and return the placeholder the answer
     should carry until it can be. */
  async function storeFile(url, headers, blob, name, fieldType) {
    const db = await openDB();
    const id = await add(db, FILES, {
      url, headers, blob, name, fieldType, ts: Date.now(), attempts: 0,
    });
    return LOCAL_PREFIX + id;
  }

  async function getFile(id) {
    const db = await openDB();
    return runTx(db, FILES, "readonly", (s) => s.get(Number(id)));
  }

  async function queueRequest(url, headers, body) {
    const db = await openDB();
    await add(db, QUEUE, { url, method: "POST", headers, body, ts: Date.now(), attempts: 0 });
  }

  /* Counts for the badge, in units of "things the user has to deal with".

     A rejected photo also fails the answer that references it. Adding both up would
     report two problems for one, and send someone looking for a second fault that
     does not exist — so a failed file is only counted when nothing in the queue
     points at it (a photo captured before the answer was ever submitted). Files
     still waiting are likewise not counted separately from their answer. */
  async function stats() {
    try {
      const db = await openDB();
      const [q, f] = await Promise.all([getAll(db, QUEUE), getAll(db, FILES)]);
      const referenced = new Set();
      for (const r of q) {
        if (typeof r.body !== "string") continue;
        LOCAL_RE.lastIndex = 0;
        let m;
        while ((m = LOCAL_RE.exec(r.body))) referenced.add(m[1]);
      }
      return {
        pending: q.filter((r) => !r.failed).length,
        failed:
          q.filter((r) => r.failed).length +
          f.filter((r) => r.failed && !referenced.has(String(r.id))).length,
        files: f.filter((r) => !r.failed).length,
      };
    } catch (_e) {
      return { pending: 0, failed: 0, files: 0 };
    }
  }

  function expired(rec) {
    if ((rec.attempts || 0) >= MAX_ATTEMPTS) return "Giving up after too many attempts.";
    if (Date.now() - (rec.ts || 0) > MAX_AGE_MS) return "Too old to send.";
    return "";
  }

  async function markFailed(db, store, rec, status, error) {
    await put(db, store, { ...rec, failed: true, status, error, failedAt: Date.now() });
  }

  async function bumpAttempt(db, store, rec) {
    await put(db, store, { ...rec, attempts: (rec.attempts || 0) + 1 });
  }

  /* Phase 1: push the stored files up, oldest first, and build id → real URL.

     This has to finish before any answer that references a file is sent, otherwise
     the server would receive a dead "eform-local://" string in place of a photo. */
  async function flushFiles(db) {
    const files = (await getAll(db, FILES)).sort((a, b) => a.ts - b.ts);
    const map = Object.create(null);
    let uploaded = 0;
    let failed = 0;

    for (const f of files) {
      if (f.failed) {
        map[f.id] = { failed: true, error: f.error || "Attachment was rejected." };
        continue;
      }
      const stale = expired(f);
      if (stale) {
        await markFailed(db, FILES, f, 0, stale);
        map[f.id] = { failed: true, error: stale };
        failed++;
        continue;
      }

      const fd = new FormData();
      fd.append("file", f.blob, f.name || "upload-" + f.id);
      fd.append("fieldType", f.fieldType || "file");

      let res;
      try {
        res = await fetch(f.url, { method: "POST", headers: f.headers || {}, body: fd });
      } catch (_e) {
        await bumpAttempt(db, FILES, f);
        return { map, stalled: true, uploaded, failed }; // still offline
      }

      if (res.ok) {
        let url = "";
        try {
          url = ((await res.json()) || {}).url || "";
        } catch (_e) {
          /* not JSON */
        }
        if (url) {
          map[f.id] = { url };
          await del(db, FILES, f.id);
          uploaded++;
          continue;
        }
        // 200 with no URL: the answer would end up pointing nowhere, which is worse
        // than saying so out loud.
        await markFailed(db, FILES, f, res.status, "Upload returned no URL.");
        map[f.id] = { failed: true, error: "Upload returned no URL." };
        failed++;
        continue;
      }

      if (res.status >= 500) {
        await bumpAttempt(db, FILES, f);
        return { map, stalled: true, uploaded, failed }; // server problem, retry later
      }

      let detail = "";
      try {
        detail = ((await res.json()) || {}).error || "";
      } catch (_e) {
        /* not JSON */
      }
      await markFailed(db, FILES, f, res.status, detail || "Attachment was rejected.");
      map[f.id] = { failed: true, error: detail || "Attachment was rejected." };
      failed++;
    }
    return { map, stalled: false, uploaded, failed };
  }

  /* Swap placeholders for real URLs.

     Returns {ok:true, body} when every reference resolved, {wait:true} when a file
     simply has not been uploaded yet, or {error} when one was rejected for good and
     the answer can never be completed. It is done in a single pass so that
     eform-local://1 cannot eat the prefix of eform-local://12. */
  function resolveRefs(body, map) {
    if (typeof body !== "string" || body.indexOf(LOCAL_PREFIX) < 0) return { ok: true, body };
    let wait = false;
    let error = "";
    LOCAL_RE.lastIndex = 0;
    const out = body.replace(LOCAL_RE, (whole, id) => {
      const hit = map[id];
      if (!hit) {
        wait = true;
        return whole;
      }
      if (hit.failed) {
        error = hit.error;
        return whole;
      }
      return hit.url;
    });
    if (error) return { error };
    if (wait) return { wait: true };
    return { ok: true, body: out };
  }

  /* Phase 2: send the queued answers, oldest first.

     The distinction that matters is transient versus permanent:
       - network error or 5xx → unreachable or broken. Stop; the whole queue is
         retried on the next sync, order preserved.
       - 4xx → the server understood and refused (validation, an expired respondent
         token, a revoked share). Retrying cannot help, so the record is marked and
         the queue MOVES ON. One such record used to block every later answer. */
  async function flush() {
    const db = await openDB();
    const fileResult = await flushFiles(db);
    const records = (await getAll(db, QUEUE)).sort((a, b) => a.ts - b.ts);

    let sent = 0;
    let failed = fileResult.failed;

    for (const rec of records) {
      if (rec.failed) continue; // already refused; kept for someone to look at

      const refs = resolveRefs(rec.body, fileResult.map);
      if (refs.wait) {
        // Still unresolved. If the upload pass stopped early the photo simply has
        // not had its turn yet, so leave the answer for the next flush. If it ran to
        // completion, every stored file was handled and the reference points at a
        // record that no longer exists — waiting forever would hide that.
        if (fileResult.stalled) continue;
        await markFailed(db, QUEUE, rec, 0, "Its attachment is missing from this device.");
        failed++;
        continue;
      }
      if (refs.error) {
        // The attachment will never arrive, so this answer can never be sent as it
        // stands. Surfacing it beats retrying forever in silence.
        await markFailed(db, QUEUE, rec, 0, refs.error);
        failed++;
        continue;
      }

      const stale = expired(rec);
      if (stale) {
        await markFailed(db, QUEUE, rec, 0, stale);
        failed++;
        continue;
      }

      let res;
      try {
        res = await fetch(rec.url, {
          method: rec.method || "POST",
          headers: rec.headers || {},
          body: refs.body,
        });
      } catch (_e) {
        await bumpAttempt(db, QUEUE, rec);
        break; // still offline — the rest is retried when connectivity returns
      }

      if (res.ok) {
        await del(db, QUEUE, rec.id);
        sent++;
        continue;
      }
      if (res.status >= 500) {
        await bumpAttempt(db, QUEUE, rec);
        break; // server-side problem — keep the order, retry later
      }

      let detail = "";
      try {
        detail = ((await res.json()) || {}).error || "";
      } catch (_e) {
        /* not JSON */
      }
      await markFailed(db, QUEUE, rec, res.status, detail);
      failed++;
    }

    // The map goes back to the caller so the page can replace the placeholders still
    // sitting in its own answers and local draft. Without that, a later edit would
    // queue an answer pointing at a file record that upload already deleted.
    return {
      sent, failed, uploaded: fileResult.uploaded,
      stalled: fileResult.stalled, map: fileResult.map,
    };
  }

  /* ---- inspecting and acting on what was refused ----

     Up to here a rejected record was preserved but unreachable: the user saw a
     count and could do nothing about it. These are the operations the page needs to
     make it actionable. */

  /* Everything needed to describe a failure, with the blob left out — the caller
     renders a list, and a page of photo bytes would be wasted work. */
  async function listFailed() {
    const db = await openDB();
    const [q, f] = await Promise.all([getAll(db, QUEUE), getAll(db, FILES)]);
    const out = [];
    for (const r of q.filter((x) => x.failed)) {
      let answers = null;
      try {
        answers = JSON.parse(r.body || "{}").answers || null;
      } catch (_e) {
        /* body is not JSON — the raw record is still in the export */
      }
      out.push({
        kind: "response", id: r.id, url: r.url, ts: r.ts, failedAt: r.failedAt,
        status: r.status, error: r.error, attempts: r.attempts || 0,
        answerCount: answers ? Object.keys(answers).length : null,
      });
    }
    for (const r of f.filter((x) => x.failed)) {
      out.push({
        kind: "file", id: r.id, url: r.url, ts: r.ts, failedAt: r.failedAt,
        status: r.status, error: r.error, attempts: r.attempts || 0,
        name: r.name || "", size: r.blob ? r.blob.size : 0, fieldType: r.fieldType || "",
      });
    }
    return out.sort((a, b) => (b.failedAt || 0) - (a.failedAt || 0));
  }

  const storeOf = (kind) => (kind === "file" ? FILES : QUEUE);

  /* Put a record back in the queue. The attempt counter is reset too: the reason
     for retrying is that something changed (a fresh login, a corrected share), so
     holding the old attempts against it would just fail it again immediately. */
  async function retryFailed(kind, id) {
    const db = await openDB();
    const store = storeOf(kind);
    const rec = await runTx(db, store, "readonly", (s) => s.get(Number(id)));
    if (!rec) return false;
    delete rec.failed;
    delete rec.status;
    delete rec.error;
    delete rec.failedAt;
    rec.attempts = 0;
    rec.ts = Date.now(); // also clears the age limit
    await put(db, store, rec);
    return true;
  }

  async function retryAllFailed() {
    const items = await listFailed();
    for (const it of items) await retryFailed(it.kind, it.id);
    return items.length;
  }

  async function discardFailed(kind, id) {
    const db = await openDB();
    await del(db, storeOf(kind), Number(id));
  }

  /* A rejected answer is field data that exists nowhere else. Before anyone deletes
     one, they should be able to get it off the device — so the export carries the
     full record including the body, not just the summary. */
  async function exportFailed() {
    const db = await openDB();
    const [q, f] = await Promise.all([getAll(db, QUEUE), getAll(db, FILES)]);
    return {
      exportedAt: new Date().toISOString(),
      responses: q.filter((x) => x.failed).map((r) => ({
        id: r.id, url: r.url, queuedAt: new Date(r.ts || 0).toISOString(),
        failedAt: r.failedAt ? new Date(r.failedAt).toISOString() : null,
        status: r.status, error: r.error, body: r.body,
      })),
      files: f.filter((x) => x.failed).map((r) => ({
        id: r.id, name: r.name, fieldType: r.fieldType,
        size: r.blob ? r.blob.size : 0, type: r.blob ? r.blob.type : "",
        queuedAt: new Date(r.ts || 0).toISOString(),
        failedAt: r.failedAt ? new Date(r.failedAt).toISOString() : null,
        status: r.status, error: r.error,
      })),
    };
  }

  /* What this device would tell the server about its backlog.

     Metadata only — counts, statuses, error texts, timestamps. The answers themselves
     stay here: the server refused them, so shipping them over a side channel would
     put data into the system that its own validation turned away. The report exists
     so somebody at the office knows to go and find this user. */
  const REPORT_ITEM_CAP = 50;

  async function reportPayload(deviceId) {
    const db = await openDB();
    const [q, f] = await Promise.all([getAll(db, QUEUE), getAll(db, FILES)]);
    const s = await stats();

    let oldest = null;
    for (const r of q.concat(f)) {
      if (r.ts && (oldest === null || r.ts < oldest)) oldest = r.ts;
    }

    // Only the failures are described one by one. A pending item is not yet a
    // problem; it is waiting for a signal, and listing hundreds of them would bury
    // the handful that need a person.
    const items = [];
    for (const r of q.concat(f)) {
      if (!r.failed || items.length >= REPORT_ITEM_CAP) continue;
      items.push({
        kind: r.blob ? "file" : "response",
        status: r.status || 0,
        error: r.error || "",
        queuedAt: r.ts ? new Date(r.ts).toISOString() : null,
        failedAt: r.failedAt ? new Date(r.failedAt).toISOString() : null,
        attempts: r.attempts || 0,
      });
    }

    return {
      deviceId,
      pending: s.pending,
      failed: s.failed,
      files: s.files,
      oldestQueuedAt: oldest ? new Date(oldest).toISOString() : null,
      items,
    };
  }

  global.EformOfflineQueue = {
    DB_NAME, DB_VERSION, QUEUE, FILES, LOCAL_PREFIX, LOCAL_RE,
    MAX_ATTEMPTS, MAX_AGE_MS,
    openDB, runTx, getAll, add, put, del,
    storeFile, getFile, queueRequest, stats, resolveRefs, flush,
    listFailed, retryFailed, retryAllFailed, discardFailed, exportFailed,
    reportPayload,
  };
})(self);
