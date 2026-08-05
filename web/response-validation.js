/* Pemeriksaan validasi jawaban, untuk halaman detail respons.

   Dipakai bersama response-view.html (admin) dan portal-response-view.html
   (viewer/editor) supaya keduanya menilai jawaban dengan aturan yang sama.

   Berkas ini sengaja tidak menyentuh DOM halaman dan tidak membaca variabel
   global milik halaman. Semua yang dibutuhkannya — cara mengevaluasi ekspresi,
   menghitung baris roster, menerjemahkan teks — diterima lewat objek `h` dari
   pemanggil. Halaman detail sudah punya semua fungsi itu untuk merender jawaban,
   jadi tidak ada mesin ekspresi kedua yang harus dirawat di sini.

   Aturannya dijaga sama persis dengan yang dipakai form pengisian
   (collectAllErrors di public.html):
     - field wajib (required / requiredWhen) yang masih kosong
     - aturan di c.validations yang tidak terpenuhi
     - roster requiredRows yang belum punya baris
   Field yang tidak pernah tampil ke responden — karena visibleWhen, enableWhen,
   atau dilompati skip-to — tidak dihitung, mengikuti perilaku form pengisian.

   Diperiksa untuk draf MAUPUN jawaban terkirim. Aturan validasi di kuesioner
   bisa berubah setelah jawaban dikirim, sehingga jawaban yang dulu lolos bisa
   saja sekarang melanggar aturan yang baru — justru itu yang perlu terlihat. */
(function () {
  if (window.ResponseValidation) return;

  const style = document.createElement("style");
  style.textContent = `
/* ---- tombol pembuka di kartu meta ---- */
.vc-btn{display:inline-flex;align-items:center;gap:6px;padding:6px 11px;
  border:1px solid var(--line,#dfe4ea);border-radius:8px;background:var(--panel,#fff);
  color:var(--ink,#13171e);font:inherit;font-size:12.5px;font-weight:600;cursor:pointer;
  line-height:1.2;white-space:nowrap}
.vc-btn:hover{background:var(--panel-2,#f7f9fb)}
.vc-btn.has-err{border-color:#fecaca;background:#fef2f2;color:#b91c1c}
.vc-btn.has-err:hover{background:#fee2e2}
.vc-badge{display:inline-flex;align-items:center;justify-content:center;min-width:18px;
  height:18px;padding:0 5px;border-radius:999px;background:#dc2626;color:#fff;
  font-size:10.5px;font-weight:700}
/* display:inline-flex di atas mengalahkan gaya bawaan [hidden] milik browser,
   jadi keadaan tersembunyinya harus dinyatakan sendiri. */
.vc-badge[hidden]{display:none}

/* ---- modal ---- */
/* Ukurannya dipatok ke 100vw/100vh, bukan inset:0. Kalau halaman di bawahnya
   meluap ke samping, containing block untuk position:fixed ikut selebar luapan
   itu — sehingga inset:0 membuat modal lebih lebar dari layar. Satuan vw/vh
   selalu mengacu ke viewport awal, jadi modal tetap pas berapa pun luapannya. */
.vc-overlay{position:fixed;top:0;left:0;width:100vw;height:100vh;z-index:400;
  display:flex;align-items:center;justify-content:center;padding:20px;
  background:rgba(19,23,30,.42)}
/* dvh mengikuti bilah alamat browser HP yang muncul-hilang */
@supports(height:100dvh){.vc-overlay{height:100dvh}}
.vc-overlay[hidden]{display:none}
.vc-modal{display:flex;flex-direction:column;width:min(680px,100%);max-height:min(78vh,720px);
  background:#fff;border-radius:12px;overflow:hidden;box-shadow:0 24px 60px -12px rgba(19,23,30,.4)}
.vc-head{display:flex;align-items:center;gap:9px;padding:14px 18px;flex:none;
  border-bottom:1px solid var(--line,#dfe4ea);font-size:14.5px;font-weight:700;color:#13171e}
.vc-head .vc-badge{background:#dc2626}
.vc-x{margin-left:auto;border:none;background:none;color:#79828f;cursor:pointer;
  font-size:19px;line-height:1;padding:4px 8px;border-radius:7px}
.vc-x:hover{background:#f1f3f6;color:#13171e}
.vc-body{flex:1;overflow-y:auto;padding:6px 18px 16px}
.vc-foot{flex:none;display:flex;justify-content:flex-end;padding:11px 18px;
  border-top:1px solid var(--line,#dfe4ea);background:var(--panel-2,#f7f9fb)}
.vc-foot button{padding:7px 15px;border:1px solid var(--line,#dfe4ea);border-radius:8px;
  background:#fff;font:inherit;font-size:13px;cursor:pointer}
.vc-foot button:hover{background:var(--panel-2,#f7f9fb)}

.vc-grp{margin-top:14px}
.vc-grp-h{font-size:11px;font-weight:700;text-transform:uppercase;letter-spacing:.05em;
  color:#79828f;margin-bottom:5px}
.vc-list{list-style:none;margin:0;padding:0}
.vc-li{padding:7px 0;border-bottom:1px solid #f1f3f6;font-size:12.5px;line-height:1.5}
.vc-li:last-child{border-bottom:none}
.vc-jump{background:none;border:none;padding:0;font:inherit;font-weight:600;color:#1d4ed8;
  cursor:pointer;text-align:left;text-decoration:underline;text-underline-offset:2px;
  word-break:break-word}
.vc-jump:hover{color:#1e3a8a}
.vc-why{color:#b91c1c}
.vc-warn .vc-why{color:#92400e}
.vc-pg{color:#79828f;font-size:11.5px}
.vc-ok{padding:26px 0;text-align:center;color:#15803d;font-size:13.5px;font-weight:600}
.vc-ok span{display:block;margin-top:4px;color:#79828f;font-size:12.5px;font-weight:400}

/* ---- penanda di tempat, pada field yang bermasalah ----
   margin vertikal penting: tanpa itu beberapa field bermasalah yang berurutan
   menyatu jadi satu blok merah panjang dan batas antar-pertanyaan hilang. */
.rv-issue{position:relative;border-left:3px solid #dc2626;padding:8px 10px 6px 11px;
  margin:8px 0;background:#fef2f2;border-radius:0 7px 7px 0}
.rv-issue-warning{border-left-color:#d97706;background:#fffbeb}
/* Pesan diletakkan sesudah isi field, sehingga terbaca
   "Provinsi / — / Wajib diisi" — bukan peringatan yang menggantung di atas label.
   align-items:flex-start supaya ikon sejajar baris pertama pesan yang panjang. */
.rv-issue-msg{display:flex;align-items:flex-start;gap:5px;margin-top:5px;
  font-size:11.5px;font-weight:700;line-height:1.45;color:#b91c1c}
.rv-issue-msg::before{content:"!";display:inline-flex;align-items:center;justify-content:center;
  width:13px;height:13px;margin-top:1px;border-radius:50%;background:#dc2626;color:#fff;
  font-size:9.5px;font-weight:700;flex:none}
.rv-issue-warning .rv-issue-msg{color:#92400e}
.rv-issue-warning .rv-issue-msg::before{background:#d97706}
.rv-issue .rv-field:last-child{margin-bottom:0}
.rv-issue-flash{animation:vcFlash 1.4s ease-out}
@keyframes vcFlash{0%,55%{background:#fde2e2}100%{background:#fef2f2}}

@media(max-width:640px){
  .vc-overlay{padding:0;align-items:flex-end}
  .vc-modal{width:100%;max-height:88vh;border-radius:14px 14px 0 0}
  .vc-body{padding:6px 14px 14px}
  .vc-head{padding:13px 14px}
  .vc-pg{display:block}
  .rv-issue{padding:7px 8px 6px 9px}
}
@media(prefers-reduced-motion:reduce){.rv-issue-flash{animation:none}}
`;
  document.head.appendChild(style);

  const esc = s => String(s ?? "").replace(/[&<>"]/g,
    c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c]));

  const isEmpty = v => v == null || v === "" || (Array.isArray(v) && v.length === 0);

  // Tipe yang tidak menyimpan jawaban, jadi tidak pernah punya error.
  // `calculated` tanpa autofill ikut dikecualikan, sama seperti form pengisian.
  function skipType(c) {
    return c.type === "note" || c.type === "markdown" || c.type === "hidden" ||
      (c.type === "calculated" && !c.autofill);
  }

  /* Kumpulkan field yang benar-benar berlaku pada satu halaman, termasuk
     menguraikan tiap baris roster jadi field tersendiri. Cerminan
     pageValidationTargets + walkRosterComps di public.html. */
  function targetsOfPage(page, answers, hidden, h) {
    const out = [];

    function walkRoster(comps, rp, rowSkip) {
      (comps || []).forEach(c => {
        if (!h.evalVisible(c.visibleWhen, rp, answers)) return;
        if (c.kind === "field") {
          if (!rowSkip.has(c.name)) out.push({ c, rp });
          return;
        }
        if (c.kind === "roster") { expandRoster(c, rp); return; }
        walkRoster(c.components || [], rp, rowSkip);
      });
    }

    function expandRoster(r, rp) {
      const n = h.getRosterCount(r, answers, rp);
      for (let i = 0; i < n; i++) {
        const childRp = h.rosterRowPrefix(r, i, rp);
        walkRoster(r.components || [], childRp,
          h.computeRosterRowSkipState(r, i, answers, rp));
      }
    }

    (function walk(node, rp) {
      (node.components || []).forEach(c => {
        if (!h.evalVisible(c.visibleWhen, rp, answers)) return;
        if (c.kind === "field") {
          if (!hidden.has(c.name)) out.push({ c, rp });
        } else if (c.kind === "roster") {
          expandRoster(c, rp);
        } else {
          walk(c, rp);
        }
      });
    })(page, "");

    return out;
  }

  // Roster yang wajib berisi tapi belum punya baris sama sekali.
  function emptyRequiredRosters(page, answers, h) {
    const out = [];
    (function walk(comps, rp) {
      (comps || []).forEach(c => {
        if (!h.evalVisible(c.visibleWhen, rp, answers)) return;
        if (c.kind === "roster") {
          if (c.requiredRows && h.getRosterCount(c, answers, rp) === 0) out.push(c);
        } else if (c.kind !== "field" && c.components) {
          walk(c.components, rp);
        }
      });
    })(page.components || [], "");
    return out;
  }

  // Id DOM yang stabil untuk satu temuan; kunci roster memuat "#" dan titik,
  // yang tidak aman dipakai langsung sebagai id.
  let seq = 0;
  const nextId = () => "vc-i" + (++seq);

  /* collect mengembalikan daftar temuan yang sudah urut halaman.
     h wajib berisi: evalVisible, computePageSkipState, computeRosterRowSkipState,
     getRosterCount, rosterRowPrefix, txt, visitedPages; opsional: canSee. */
  function collect(schema, answers, h) {
    const pages = (schema && schema.pages) || [];
    const canSee = h.canSee || (() => true);
    const issues = [];
    seq = 0;

    for (const pi of h.visitedPages || []) {
      const page = pages[pi];
      if (!page) continue;
      const hidden = h.computePageSkipState(page, answers).hidden;

      for (const { c, rp } of targetsOfPage(page, answers, hidden, h)) {
        if (skipType(c)) continue;
        // Variabel di luar jatah viewer: jawabannya sudah disamarkan server,
        // jadi menilainya pasti keliru — dan labelnya pun tidak boleh tampil.
        if (!canSee(c.name)) continue;
        // enableWhen tidak terpenuhi berarti field terkunci; tidak dinilai.
        if (!h.evalVisible(c.enableWhen, rp, answers)) continue;

        const key = rp + c.name;
        const label = h.txt(c.label) || c.name;
        const required = !!c.required ||
          !!(c.requiredWhen && h.evalVisible(c.requiredWhen, rp, answers));

        if (required && isEmpty(answers[key])) {
          issues.push({
            pageIdx: pi, key, id: nextId(), label,
            why: "Wajib diisi", kind: "required", severity: "error",
          });
          continue;
        }
        if (isEmpty(answers[key])) continue;

        for (const v of c.validations || []) {
          if (!v.test) continue;
          // evalVisible mengembalikan true saat ekspresi gagal diurai, sehingga
          // aturan yang rusak tidak pernah menuduh datanya salah.
          if (h.evalVisible(v.test, rp, answers)) continue;
          issues.push({
            pageIdx: pi, key, id: nextId(), label,
            why: h.txt(v.message) || "Tidak memenuhi aturan validasi",
            kind: "rule",
            severity: v.severity === "warning" ? "warning" : "error",
          });
          break;
        }
      }

      for (const r of emptyRequiredRosters(page, answers, h)) {
        issues.push({
          pageIdx: pi, key: "", id: nextId(),
          label: h.txt(r.title) || r.name,
          why: "Belum ada baris yang ditambahkan",
          kind: "roster", severity: "error",
        });
      }
    }
    return issues;
  }

  // Peta kunci → temuan, dipakai halaman untuk menandai field saat merender.
  // Kalau satu field punya lebih dari satu temuan, yang pertama yang dipakai.
  function indexByKey(issues) {
    const m = new Map();
    for (const it of issues) if (it.key && !m.has(it.key)) m.set(it.key, it);
    return m;
  }

  // Membungkus HTML satu field dengan penanda. Dipanggil halaman dari renderNode,
  // satu-satunya titik yang dilewati semua jenis field.
  function wrapField(html, issue) {
    if (!html || !issue) return html;
    return `<div class="rv-issue rv-issue-${issue.severity}" id="${issue.id}">` +
      `${html}<div class="rv-issue-msg">${esc(issue.why)}</div></div>`;
  }

  /* ---- keadaan & modal ---- */

  let state = { issues: [], schema: null, h: null };
  let overlay = null, lastFocus = null;

  function pageTitle(i) {
    const p = ((state.schema && state.schema.pages) || [])[i];
    if (!p) return "Halaman " + (i + 1);
    return state.h.txt(p.title) || p.name || "Halaman " + (i + 1);
  }

  /* Temuan dipisah jadi tiga kelompok karena artinya berbeda: "tidak sesuai
     aturan" berarti data yang sudah diisi memang bermasalah, sedangkan "belum
     diisi" wajar untuk draf yang belum selesai. */
  function bodyHTML() {
    const iss = state.issues;
    if (!iss.length) {
      return `<div class="vc-ok">Tidak ada error validasi
        <span>Semua jawaban yang tampil memenuhi aturan kuesioner.</span></div>`;
    }
    const groups = [
      { title: "Tidak sesuai aturan", cls: "",
        items: iss.filter(i => i.kind !== "required" && i.severity === "error") },
      { title: "Perlu dicek", cls: "vc-warn",
        items: iss.filter(i => i.severity === "warning") },
      { title: "Belum diisi", cls: "",
        items: iss.filter(i => i.kind === "required") },
    ];
    return groups.filter(g => g.items.length).map(g => `
      <div class="vc-grp ${g.cls}">
        <div class="vc-grp-h">${esc(g.title)} (${g.items.length})</div>
        <ul class="vc-list">${g.items.map(it => `
          <li class="vc-li">
            <button type="button" class="vc-jump"
              onclick="ResponseValidation.jump(${it.pageIdx},'${it.id}')">${esc(it.label)}</button>
            — <span class="vc-why">${esc(it.why)}</span>
            <span class="vc-pg">${esc(pageTitle(it.pageIdx))}</span>
          </li>`).join("")}</ul>
      </div>`).join("");
  }

  // Modal dipasang di <body>, bukan di dalam #content — halaman merender ulang
  // #content tiap ganti halaman, yang akan ikut menghapus modalnya.
  function ensureOverlay() {
    if (overlay) return overlay;
    overlay = document.createElement("div");
    overlay.className = "vc-overlay";
    overlay.hidden = true;
    overlay.innerHTML = `
      <div class="vc-modal" role="dialog" aria-modal="true" aria-labelledby="vcTitle">
        <div class="vc-head">
          <svg viewBox="0 0 20 20" width="15" height="15" fill="none" stroke="currentColor"
            stroke-width="1.8" stroke-linecap="round"><path d="M10 3.5 1.8 16.5h16.4z"/>
            <path d="M10 8v3.6"/><path d="M10 14.2v.1"/></svg>
          <span id="vcTitle">Daftar Error Validasi</span>
          <span class="vc-badge" id="vcModalCount"></span>
          <button class="vc-x" type="button" aria-label="Tutup">&times;</button>
        </div>
        <div class="vc-body" id="vcBody"></div>
        <div class="vc-foot"><button type="button">Tutup</button></div>
      </div>`;
    overlay.addEventListener("click", e => {
      // klik latar (di luar kotak) menutup modal
      if (e.target === overlay || e.target.closest(".vc-x, .vc-foot button")) close();
    });
    document.addEventListener("keydown", e => {
      if (e.key === "Escape" && !overlay.hidden) close();
    });
    document.body.appendChild(overlay);
    return overlay;
  }

  function open() {
    const ov = ensureOverlay();
    ov.querySelector("#vcBody").innerHTML = bodyHTML();
    const badge = ov.querySelector("#vcModalCount");
    badge.textContent = state.issues.length;
    badge.hidden = state.issues.length === 0;
    lastFocus = document.activeElement;
    ov.hidden = false;
    // Daftar temuan bisa panjang; tanpa ini isi modal kadang muncul dalam
    // keadaan tergulir, bukan dari temuan pertama.
    ov.querySelector("#vcBody").scrollTop = 0;
    ov.querySelector(".vc-x").focus({ preventScroll: true });
  }

  function close() {
    if (!overlay || overlay.hidden) return;
    overlay.hidden = true;
    // Kembalikan fokus ke tombol pembuka, kecuali tombol itu sudah hilang
    // karena halaman dirender ulang saat melompat ke temuan.
    if (lastFocus && lastFocus.isConnected) lastFocus.focus({ preventScroll: true });
    lastFocus = null;
  }

  /* prepare menyimpan hasil pemeriksaan dan mengembalikan HTML tombol pembuka.
     Dipanggil halaman tiap kali merender, sebelum isi jawaban dirakit. */
  function prepare(issues, schema, h) {
    state = { issues: issues || [], schema, h };
    if (overlay && !overlay.hidden) open(); // segarkan isi modal yang sedang terbuka
    const n = state.issues.length;
    return `<button type="button" class="vc-btn${n ? " has-err" : ""}"
      onclick="ResponseValidation.open()">
      <svg viewBox="0 0 20 20" width="14" height="14" fill="none" stroke="currentColor"
        stroke-width="1.8" stroke-linecap="round"><path d="M10 3.5 1.8 16.5h16.4z"/>
        <path d="M10 8v3.6"/><path d="M10 14.2v.1"/></svg>
      Daftar error${n ? `<span class="vc-badge">${n}</span>` : ""}
    </button>`;
  }

  /* Lompat ke temuan. Kalau field-nya ada di halaman yang sedang tampil, cukup
     digulir; kalau tidak, pindah halaman dulu lalu gulir setelah render.

     Id temuan konsisten antar-render: collect() menomori ulang dari nol dengan
     urutan penelusuran yang sama, jadi temuan yang sama selalu dapat id yang
     sama — termasuk setelah goPage merender ulang halaman. */
  function jump(pageIdx, id) {
    close();
    const go = () => {
      const el = document.getElementById(id);
      if (!el) return;
      el.scrollIntoView({ behavior: "smooth", block: "center" });
      el.classList.remove("rv-issue-flash");
      void el.offsetWidth; // paksa animasi diputar ulang saat diklik berkali-kali
      el.classList.add("rv-issue-flash");
    };
    if (document.getElementById(id)) { go(); return; }
    if (typeof window.goPage === "function") {
      window.goPage(pageIdx);
      setTimeout(go, 60);
    }
  }

  window.ResponseValidation = {
    collect, indexByKey, wrapField, prepare, open, close, jump,
  };
})();
