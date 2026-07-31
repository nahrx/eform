/* Service worker untuk mode offline kuesioner multi-respons (PWA).
   Kontrak IndexedDB (dipakai bareng web/public.html):
   - DB: "eform-offline", versi 1
   - Object store: "queue", keyPath "id" (autoIncrement)
   - Record: {id, url, method, headers, body, ts} */
const CACHE_NAME = "eform-v1";
const DB_NAME = "eform-offline";
const STORE_NAME = "queue";

const CACHEABLE_GET = [
  /^\/f\/[^/]+$/,
  /^\/api\/public\/forms\/[^/]+$/,
  /^\/api\/wilayah(\?.*)?$/,
];

self.addEventListener("install", () => {
  self.skipWaiting();
});

self.addEventListener("activate", (event) => {
  event.waitUntil(self.clients.claim());
});

self.addEventListener("fetch", (event) => {
  const req = event.request;
  if (req.method !== "GET") return;
  const url = new URL(req.url);
  if (url.origin !== self.location.origin) return;
  const path = url.pathname + (url.search || "");
  if (!CACHEABLE_GET.some((re) => re.test(path))) return;

  event.respondWith(
    fetch(req)
      .then((res) => {
        if (res && res.ok) {
          const copy = res.clone();
          caches.open(CACHE_NAME).then((cache) => cache.put(req, copy));
        }
        return res;
      })
      .catch(async () => {
        const cached = await caches.match(req);
        if (cached) return cached;
        return new Response(JSON.stringify({ error: "offline" }), {
          status: 503,
          headers: { "Content-Type": "application/json" },
        });
      })
  );
});

self.addEventListener("sync", (event) => {
  if (event.tag === "eform-flush") event.waitUntil(flushQueue());
});

function openDB() {
  return new Promise((resolve, reject) => {
    const req = indexedDB.open(DB_NAME, 1);
    req.onupgradeneeded = () => {
      if (!req.result.objectStoreNames.contains(STORE_NAME)) {
        req.result.createObjectStore(STORE_NAME, { keyPath: "id", autoIncrement: true });
      }
    };
    req.onsuccess = () => resolve(req.result);
    req.onerror = () => reject(req.error);
  });
}

async function flushQueue() {
  const db = await openDB();
  const records = await new Promise((resolve, reject) => {
    const tx = db.transaction(STORE_NAME, "readonly");
    const req = tx.objectStore(STORE_NAME).getAll();
    req.onsuccess = () => resolve(req.result || []);
    req.onerror = () => reject(req.error);
  });
  records.sort((a, b) => a.ts - b.ts);
  for (const rec of records) {
    try {
      const res = await fetch(rec.url, {
        method: rec.method || "POST",
        headers: rec.headers || {},
        body: rec.body,
      });
      if (!res.ok) break; // masih ada masalah (mis. server error) — jangan hapus, coba lagi nanti
      await new Promise((resolve, reject) => {
        const tx = db.transaction(STORE_NAME, "readwrite");
        tx.objectStore(STORE_NAME).delete(rec.id);
        tx.oncomplete = resolve;
        tx.onerror = () => reject(tx.error);
      });
    } catch (_e) {
      break; // masih offline — hentikan, sisanya dicoba lagi saat sync/online berikutnya
    }
  }
}
