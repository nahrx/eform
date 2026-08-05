/* Panel "Riwayat Perubahan" untuk halaman detail jawaban.

   Dipakai bersama response-view.html (admin) dan portal-response-view.html
   (viewer/editor). Keduanya memakai parameter URL yang sama (?form=&resp=) dan
   token yang sama di localStorage, jadi berkas ini bisa berdiri sendiri: cukup
   disertakan lewat <script src>, tanpa perlu mengubah kode halamannya.

   Panel hanya muncul kalau jawaban tersebut memang pernah diubah editor. Jawaban
   yang tidak pernah disunting tidak menampilkan apa-apa, supaya tidak menambah
   kebisingan di halaman yang sudah padat. */
(function () {
  if (window.__revHistInit) return;
  window.__revHistInit = true;

  const style = document.createElement("style");
  style.textContent = `
.rh-wrap{margin:24px 0 8px;border:1px solid var(--line,#dfe4ea);border-radius:10px;background:#fff;overflow:hidden}
.rh-head{display:flex;align-items:center;gap:8px;padding:12px 16px;background:#fffbeb;border-bottom:1px solid #fde68a;
  font-size:13.5px;font-weight:700;color:#92400e}
.rh-count{margin-left:auto;font-weight:600;font-size:12px;background:#fde68a;color:#92400e;padding:2px 9px;border-radius:999px}
.rh-body{padding:6px 16px 14px}
.rh-item{padding:12px 0;border-bottom:1px solid #eef1f5}
.rh-item:last-child{border-bottom:none}
.rh-meta{display:flex;flex-wrap:wrap;gap:6px 14px;align-items:baseline;font-size:12px;color:#79828f;margin-bottom:8px}
.rh-who{font-weight:700;color:#13171e;font-size:13px}
.rh-diff{display:grid;grid-template-columns:minmax(90px,auto) 1fr;gap:4px 12px;font-size:12.5px}
.rh-fname{color:#79828f;font-weight:600;word-break:break-word}
.rh-vals{min-width:0}
.rh-old{color:#b91c1c;text-decoration:line-through;word-break:break-word}
.rh-new{color:#15803d;word-break:break-word}
.rh-arrow{color:#79828f;margin:0 5px}
.rh-empty{color:#79828f;font-style:italic}
@media(max-width:640px){.rh-diff{grid-template-columns:1fr;gap:2px}.rh-body{padding:6px 12px 12px}}
`;
  document.head.appendChild(style);

  const esc = s => String(s ?? "").replace(/[&<>"]/g,
    c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c]));

  // Tampilkan nilai apa adanya tapi dipangkas; nilai kosong ditandai eksplisit
  // supaya "diisi" vs "dikosongkan" tetap terbaca.
  function showVal(v) {
    if (v === undefined) return '<span class="rh-empty">(tidak ada)</span>';
    if (v === null || v === "") return '<span class="rh-empty">(kosong)</span>';
    let s = typeof v === "object" ? JSON.stringify(v) : String(v);
    if (s.length > 160) s = s.slice(0, 160) + "…";
    return esc(s);
  }

  // Cari label pertanyaan dari skema halaman kalau tersedia, supaya yang tampil
  // bukan nama variabel mentah. Skema milik halaman (let SCHEMA) — dibaca lewat
  // try/catch karena mungkin belum terisi.
  function fieldLabel(name) {
    let schema = null;
    try { schema = typeof SCHEMA !== "undefined" ? SCHEMA : null; } catch { schema = null; }
    if (!schema) return name;
    let found = null;
    const walk = comps => {
      for (const c of comps || []) {
        if (found) return;
        if (c.kind === "field" && c.name === name) { found = c; return; }
        if (c.components) walk(c.components);
      }
    };
    for (const p of schema.pages || []) walk(p.components || []);
    if (!found) return name;
    const l = found.label;
    const text = typeof l === "string" ? l : (l && (l.id || l.en)) || name;
    return text || name;
  }

  function render(revisions) {
    const items = revisions.map(rev => {
      const before = rev.answersBefore || {};
      const after = rev.answersAfter || {};
      const fields = rev.changedFields && rev.changedFields.length
        ? rev.changedFields
        : Object.keys({ ...before, ...after });

      const rows = fields.map(f => `
        <div class="rh-fname">${esc(fieldLabel(f))}</div>
        <div class="rh-vals"><span class="rh-old">${showVal(before[f])}</span>` +
        `<span class="rh-arrow">→</span><span class="rh-new">${showVal(after[f])}</span></div>`).join("");

      const waktu = new Date(rev.createdAt).toLocaleString("id-ID");
      return `<div class="rh-item">
        <div class="rh-meta">
          <span class="rh-who">${esc(rev.editorName || "—")}</span>
          <span>${esc(waktu)}</span>
          ${rev.ip ? `<span>IP ${esc(rev.ip)}</span>` : ""}
          <span>${fields.length} variabel</span>
        </div>
        <div class="rh-diff">${rows}</div>
      </div>`;
    }).join("");

    const el = document.createElement("div");
    el.className = "rh-wrap";
    el.innerHTML = `
      <div class="rh-head">
        <svg viewBox="0 0 20 20" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.7"
             stroke-linecap="round" stroke-linejoin="round"><circle cx="10" cy="10" r="7.5"/><polyline points="10,5.5 10,10 13,12"/></svg>
        Riwayat Perubahan
        <span class="rh-count">${revisions.length}</span>
      </div>
      <div class="rh-body">${items}</div>`;
    return el;
  }

  // Halaman detail merender ulang #content setiap ganti halaman kuesioner atau
  // masuk/keluar mode edit, yang menghapus panel ini. Daripada menebak kapan
  // render selesai, panelnya dipasang ulang setiap kali #content berubah.
  function mount(panel) {
    const host = document.getElementById("content") || document.body;
    const ensure = () => {
      if (!panel.isConnected) host.appendChild(panel);
    };
    ensure();
    new MutationObserver(ensure).observe(host, { childList: true });
  }

  async function load() {
    const qp = new URLSearchParams(location.search);
    const formId = qp.get("form"), respId = qp.get("resp");
    const token = localStorage.getItem("eform_token");
    if (!formId || !respId || !token) return;
    try {
      const r = await fetch(`/api/forms/${encodeURIComponent(formId)}/responses/${encodeURIComponent(respId)}/revisions`,
        { headers: { Authorization: "Bearer " + token } });
      if (!r.ok) return; // tidak berhak / belum ada — cukup diam
      const d = await r.json();
      if (d.revisions && d.revisions.length) mount(render(d.revisions));
    } catch { /* panel ini pelengkap; kegagalannya tidak boleh mengganggu halaman */ }
  }

  // Ditunda sebentar supaya halaman selesai merender isinya lebih dulu.
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", () => setTimeout(load, 300));
  } else {
    setTimeout(load, 300);
  }
})();
