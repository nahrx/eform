/* Answer validation review, for the response detail pages.

   Shared by response-view.html (admin) and portal-response-view.html
   (viewer/editor) so both judge an answer by exactly the same rules.

   This file deliberately never touches the page's DOM and never reads the page's
   global variables. Everything it needs — how to evaluate an expression, how to
   count roster rows, how to resolve localised text — arrives through the `h`
   object supplied by the caller. The detail pages already have all of those
   functions in order to render answers, so there is no second expression engine
   to maintain here.

   The rules are kept identical to the ones the respondent-facing form applies
   (collectAllErrors in public.html):
     - required fields (required / requiredWhen) that are still empty
     - rules in c.validations that are not satisfied
     - rosters with requiredRows that have no rows yet
   Fields the respondent never saw — because of visibleWhen, enableWhen, or a
   skip-to jump — are not counted, matching the form's own behaviour.

   Both drafts AND submitted answers are checked. A form's validation rules can
   change after an answer was submitted, so an answer that once passed may now
   break a newer rule — which is precisely what needs to be visible. */
(function () {
  if (window.ResponseValidation) return;

  const style = document.createElement("style");
  style.textContent = `
/* ---- opening button on the meta card ---- */
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
/* The display:inline-flex above beats the browser's built-in [hidden] style,
   so the hidden state has to be declared explicitly. */
.vc-badge[hidden]{display:none}

/* ---- modal ---- */
/* The size is pinned to 100vw/100vh rather than inset:0. If the page underneath
   overflows sideways, the containing block for position:fixed grows to match that
   overflow — so inset:0 would make the modal wider than the screen. The vw/vh units
   always refer to the initial viewport, so the modal fits whatever the overflow. */
.vc-overlay{position:fixed;top:0;left:0;width:100vw;height:100vh;z-index:400;
  display:flex;align-items:center;justify-content:center;padding:20px;
  background:rgba(19,23,30,.42)}
/* dvh follows the mobile browser's address bar as it shows and hides */
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

/* ---- in-place markers on the offending fields ----
   The vertical margin matters: without it, several consecutive flagged fields merge
   into one long red block and the boundary between questions disappears. */
.rv-issue{position:relative;border-left:3px solid #dc2626;padding:8px 10px 6px 11px;
  margin:8px 0;background:#fef2f2;border-radius:0 7px 7px 0}
.rv-issue-warning{border-left-color:#d97706;background:#fffbeb}
/* The message sits after the field's content, so it reads
   "Province / — / Required" rather than a warning dangling above the label.
   align-items:flex-start keeps the icon level with the first line of a long message. */
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

  // Types that store no answer, and therefore can never have an error.
  // `calculated` without autofill is excluded too, matching the respondent form.
  function skipType(c) {
    return c.type === "note" || c.type === "markdown" || c.type === "hidden" ||
      (c.type === "calculated" && !c.autofill);
  }

  /* Collect the fields that genuinely apply on one page, expanding every roster row
     into its own set of fields. Mirrors pageValidationTargets + walkRosterComps in
     public.html. */
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

  // Rosters that must contain rows but have none at all.
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

  // A stable DOM id for one finding; roster keys contain "#" and dots,
  // which are not safe to use directly as an id.
  let seq = 0;
  const nextId = () => "vc-i" + (++seq);

  /* collect returns the findings, already ordered by page.
     h must provide: evalVisible, computePageSkipState, computeRosterRowSkipState,
     getRosterCount, rosterRowPrefix, txt, visitedPages; canSee is optional. */
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
        // Fields outside the viewer's allowance: the server already masked the answer,
        // so judging it would be wrong — and its label must not be shown either.
        if (!canSee(c.name)) continue;
        // An unsatisfied enableWhen means the field is locked; it is not judged.
        if (!h.evalVisible(c.enableWhen, rp, answers)) continue;

        const key = rp + c.name;
        const label = h.txt(c.label) || c.name;
        const required = !!c.required ||
          !!(c.requiredWhen && h.evalVisible(c.requiredWhen, rp, answers));

        if (required && isEmpty(answers[key])) {
          issues.push({
            pageIdx: pi, key, id: nextId(), label,
            why: "Required", kind: "required", severity: "error",
          });
          continue;
        }
        if (isEmpty(answers[key])) continue;

        for (const v of c.validations || []) {
          if (!v.test) continue;
          // evalVisible returns true when an expression fails to parse, so a
          // broken rule never accuses the data of being wrong.
          if (h.evalVisible(v.test, rp, answers)) continue;
          issues.push({
            pageIdx: pi, key, id: nextId(), label,
            why: h.txt(v.message) || "Does not meet the validation rule",
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
          why: "No rows have been added",
          kind: "roster", severity: "error",
        });
      }
    }
    return issues;
  }

  // A key → finding map, used by the page to mark fields as it renders.
  // When one field has more than one finding, the first is used.
  function indexByKey(issues) {
    const m = new Map();
    for (const it of issues) if (it.key && !m.has(it.key)) m.set(it.key, it);
    return m;
  }

  // Wraps one field's HTML with a marker. The page calls this from renderNode,
  // the single point every kind of field passes through.
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
    if (!p) return "Page " + (i + 1);
    return state.h.txt(p.title) || p.name || "Page " + (i + 1);
  }

  /* Findings are split into three groups because they mean different things: a
     "rule violation" means data that was filled in is genuinely wrong, whereas
     "not filled in" is expected for a draft that is not finished yet. */
  function bodyHTML() {
    const iss = state.issues;
    if (!iss.length) {
      return `<div class="vc-ok">No validation errors
        <span>Every answer shown satisfies the form's rules.</span></div>`;
    }
    const groups = [
      { title: "Rule violations", cls: "",
        items: iss.filter(i => i.kind !== "required" && i.severity === "error") },
      { title: "Needs checking", cls: "vc-warn",
        items: iss.filter(i => i.severity === "warning") },
      { title: "Not filled in", cls: "",
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

  // The modal is mounted on <body>, not inside #content — the page re-renders
  // #content on every page change, which would take the modal with it.
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
          <span id="vcTitle">Validation Errors</span>
          <span class="vc-badge" id="vcModalCount"></span>
          <button class="vc-x" type="button" aria-label="Close">&times;</button>
        </div>
        <div class="vc-body" id="vcBody"></div>
        <div class="vc-foot"><button type="button">Close</button></div>
      </div>`;
    overlay.addEventListener("click", e => {
      // clicking the backdrop (outside the box) closes the modal
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
    // The findings list can be long; without this the modal sometimes opens
    // already scrolled, rather than at the first finding.
    ov.querySelector("#vcBody").scrollTop = 0;
    ov.querySelector(".vc-x").focus({ preventScroll: true });
  }

  function close() {
    if (!overlay || overlay.hidden) return;
    overlay.hidden = true;
    // Return focus to the opening button, unless that button is gone because
    // the page re-rendered while jumping to a finding.
    if (lastFocus && lastFocus.isConnected) lastFocus.focus({ preventScroll: true });
    lastFocus = null;
  }

  /* prepare stores the review result and returns the opening button's HTML.
     The page calls it on every render, before the answer body is assembled. */
  function prepare(issues, schema, h) {
    state = { issues: issues || [], schema, h };
    if (overlay && !overlay.hidden) open(); // refresh the contents of an already-open modal
    const n = state.issues.length;
    return `<button type="button" class="vc-btn${n ? " has-err" : ""}"
      onclick="ResponseValidation.open()">
      <svg viewBox="0 0 20 20" width="14" height="14" fill="none" stroke="currentColor"
        stroke-width="1.8" stroke-linecap="round"><path d="M10 3.5 1.8 16.5h16.4z"/>
        <path d="M10 8v3.6"/><path d="M10 14.2v.1"/></svg>
      Error list${n ? `<span class="vc-badge">${n}</span>` : ""}
    </button>`;
  }

  /* Jump to a finding. If its field is on the page currently shown, just scroll to
     it; otherwise switch pages first and scroll once the render is done.

     Finding ids stay consistent across renders: collect() renumbers from zero using
     the same traversal order, so the same finding always gets the same id —
     including after goPage re-renders the page. */
  function jump(pageIdx, id) {
    close();
    const go = () => {
      const el = document.getElementById(id);
      if (!el) return;
      el.scrollIntoView({ behavior: "smooth", block: "center" });
      el.classList.remove("rv-issue-flash");
      void el.offsetWidth; // force the animation to replay on repeated clicks
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
