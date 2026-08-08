/* Warns, on a response detail page, that the instrument has been edited since this
   response was given.

   Every response now pins the schema snapshot it was actually filled against, but these
   pages still render answers through the form's CURRENT schema. Where the two differ the
   page can be quietly wrong — an option removed since, a field renamed, a question
   inserted — and nothing on screen would say so. This notice is what says so.

   Follows revision-history.js: shared by response-view.html (admin) and
   portal-response-view.html (viewer/editor), reading the same ?form=&resp= parameters
   and the same token, so a <script src> is all either page needs. */
(function () {
  if (window.__schemaNoticeInit) return;
  window.__schemaNoticeInit = true;

  const style = document.createElement("style");
  style.textContent = `
.sv-note{display:flex;gap:10px;align-items:flex-start;margin:0 0 18px;padding:12px 16px;
  border:1px solid #fde68a;border-left-width:4px;border-radius:8px;background:#fffbeb;
  font-size:13px;line-height:1.55;color:#78350f}
.sv-note.sv-unknown{border-color:#e2e8f0;border-left-color:#94a3b8;background:#f8fafc;color:#475569}
.sv-ic{flex:none;font-size:15px;line-height:1.4}
.sv-note strong{font-weight:700}
.sv-when{white-space:nowrap}
@media(max-width:640px){.sv-note{padding:10px 12px;font-size:12.5px}}
`;
  document.head.appendChild(style);

  const esc = s => String(s ?? "").replace(/[&<>"]/g,
    c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c]));

  function fmtDate(iso) {
    const d = new Date(iso);
    if (isNaN(d)) return "";
    return d.toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" });
  }

  function render(info) {
    const el = document.createElement("div");
    if (!info.known || info.backfilled) {
      // Backfilled means the pin was assigned retroactively, so it describes today's
      // instrument, not the one this respondent saw. Saying "matches" would be a
      // stronger claim than the data supports.
      el.className = "sv-note sv-unknown";
      el.innerHTML = `<span class="sv-ic">•</span><div>This response predates instrument version tracking. Which questions were actually put cannot be established — it was never recorded.</div>`;
      return el;
    }
    const label = info.version ? `<strong>${esc(info.version)}</strong>` : "an earlier version";
    const when = info.pinnedAt ? ` (${esc(fmtDate(info.pinnedAt))})` : "";
    el.className = "sv-note";
    el.innerHTML = `<span class="sv-ic">⚠</span><div>Filled against instrument version ${label}<span class="sv-when">${when}</span>, which has been edited since. The answers below are laid out using the current questions, so labels and options may not match what was actually asked.</div>`;
    return el;
  }

  // The page re-renders #content on every page change and when edit mode is toggled,
  // which drops this notice. Re-mounted on each change rather than guessing when the
  // render settles — and prepended, since a warning below the answers is a warning
  // arriving too late.
  function mount(note) {
    const host = document.getElementById("content") || document.body;
    const ensure = () => {
      if (!note.isConnected) host.insertBefore(note, host.firstChild);
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
      const r = await fetch(
        `/api/forms/${encodeURIComponent(formId)}/responses/${encodeURIComponent(respId)}/schema-version`,
        { headers: { Authorization: "Bearer " + token } });
      if (!r.ok) return; // not permitted — stay quiet
      const info = await r.json();
      // Nothing is said when the response matches the instrument on screen: that is the
      // normal case, and a banner on every response would soon stop being read.
      if (info && (info.outdated || info.backfilled || !info.known)) mount(render(info));
    } catch { /* supplementary; its failure must not disturb the page */ }
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", () => setTimeout(load, 300));
  } else {
    setTimeout(load, 300);
  }
})();
