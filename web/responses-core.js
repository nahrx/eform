/* Logic shared by the three response-list pages:
   responses.html (admin), viewer-responses.html, editor-responses.html.

   These functions used to be copy-pasted into each page and had already started to
   drift apart — only two pages handled the empty timestamp ("0001-01-01"), and only
   the admin page translated region codes into names in the filter summary. What is
   here is the best behaviour of all three, merged.

   This file loads as a classic script BEFORE each page's inline script, and reads a
   few of the page's global variables (SCHEMA, SEL_COLS, ALL_FIELDS, FIELD_*_FILTERS,
   and so on). Because they are only read when a function runs — not at load time —
   each page is still free to declare them with let/const.

   Optional hooks that are recognised:
     canFieldFilter()         → return false to hide the per-field filters; used by
                                the admin page, which only learns its role
                                once /api/auth/me has finished (default: allowed)
     FILTER_SHARE + SHARE_MAP → only present on the admin page */

const esc = s => String(s ?? "").replace(/[&<>"]/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c]));
const txt = v => v == null ? "" : (typeof v === "object" ? (v.id || Object.values(v)[0] || "") : String(v));

let _apiOptCache = {};

function parseAnswers(raw) {
  if (typeof raw === "object" && raw !== null) return raw;
  try { return JSON.parse(raw) || {}; } catch { return {}; }
}

// Go sends an empty timestamp as "0001-01-01T00:00:00Z" — render it as
// "—" rather than "01 Jan 1".
function formatDate(iso) {
  if (!iso || iso === "0001-01-01T00:00:00Z") return "—";
  try {
    return new Date(iso).toLocaleString("id-ID",
      { day: "2-digit", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit" });
  } catch { return iso; }
}

function truncate(s, n) {
  if (!s) return "";
  const str = String(s);
  return str.length > n ? str.slice(0, n) + "…" : str;
}

function valStr(v) {
  if (v == null || v === "") return "";
  if (Array.isArray(v)) return v.join(", ");
  return String(v);
}

function findField(schema, name) {
  let found = null;
  function walk(comps) {
    for (const c of comps || []) {
      if (c.kind === "field" && c.name === name) { found = c; return; }
      if (c.components) walk(c.components);
    }
  }
  for (const p of schema?.pages || []) if (!found) walk(p.components || []);
  return found;
}

/* ---- rendering answer values ---- */

// getOptionLabel turns a stored value (a code) into the label the respondent
// saw while filling in the form.
function getOptionLabel(c, val) {
  if (c.optionsRef && SCHEMA) {
    const t = (SCHEMA.referenceData || {})[c.optionsRef];
    if (t && t.items) {
      const item = t.items.find(it => String(it.code) === String(val));
      if (item) return txt(item.label);
    }
  }
  if (c.options) {
    const opt = c.options.find(o => String(o.value) === String(val));
    if (opt) return txt(opt.label) || String(val);
  }
  return String(val);
}

function formatFieldValue(c, val) {
  if (val == null || val === "") return "";
  if (Array.isArray(val)) {
    if (!val.length) return "";
    return val.map(v => getOptionLabel(c, v)).join(", ");
  }
  if (c.type === "boolean") return (val === "true" || val === true) ? "Yes" : "No";
  if (c.type === "select" || c.type === "radio") return getOptionLabel(c, val);
  return String(val);
}

/* ---- filter kind per field ---- */

// fieldFilterKind decides which filter control a field gets, and where its options
// come from (inline, referenceData, or an API — including cascading APIs).
function fieldFilterKind(c) {
  if (!c) return { kind: "text" };
  if (c.type === "boolean")
    return { kind: "select", opts: [{ value: "true", label: "Yes" }, { value: "false", label: "No" }] };
  if (c.type === "date") return { kind: "date" };
  if (c.type === "datetime") return { kind: "datetime" };
  if (c.type === "time") return { kind: "time" };
  if (NUMER_TYPES.has(c.type)) return { kind: "number" };

  if (c.type === "radio" || c.type === "select" || c.type === "checkbox" || c.type === "multiselect") {
    const multi = c.type === "checkbox" || c.type === "multiselect";

    // Sumber: optionsApi langsung di field
    if (c.optionsApi && c.optionsApi.url) {
      const deps = [...c.optionsApi.url.matchAll(/\{([^}]+)\}/g)].map(m => m[1]);
      const base = {
        valueField: c.optionsApi.valueField || "code",
        labelField: c.optionsApi.labelField || "label",
        path: c.optionsApi.path || null,
      };
      if (deps.length) return { kind: multi ? "multicascade" : "cascade", urlTpl: c.optionsApi.url, deps, ...base };
      return { kind: multi ? "multiapi" : "api", url: c.optionsApi.url, ...base };
    }

    // Source: a referenceData table named by optionsRef. Inline items only — the
    // "source":"api" table form was removed, since the form-filling page never
    // implemented it and no answer can have been recorded through one.
    if (c.optionsRef) {
      const tbl = (SCHEMA.referenceData || {})[c.optionsRef];
      if (tbl && tbl.items)
        return { kind: multi ? "multiselect" : "select", opts: tbl.items.map(it => ({ value: String(it.code), label: txt(it.label) })) };
    }

    // Source: options listed inline on the field
    if (c.options && c.options.length)
      return { kind: multi ? "multiselect" : "select", opts: c.options.map(o => ({ value: String(o.value), label: txt(o.label) || String(o.value) })) };
  }
  return { kind: "text" };
}

async function fetchApiOpts(src) {
  if (_apiOptCache[src.url]) return _apiOptCache[src.url];
  try {
    const r = await fetch("/api/options-proxy?url=" + encodeURIComponent(src.url));
    const d = await r.json();
    const getPath = (o, p) => String(p).split(".").reduce((x, k) => x == null ? x : x[k], o);
    const arr = src.path ? getPath(d, src.path) : (Array.isArray(d) ? d : (d.data || d.items || d.results || []));
    const vf = src.valueField || "code", lf = src.labelField || "label";
    const opts = (arr || []).map(it => ({ value: String(it[vf]), label: txt(it[lf]) || String(it[vf]) }));
    _apiOptCache[src.url] = opts;
    return opts;
  } catch { return []; }
}

/* ---- per-field filter bar (rendered into #ffBar inside the filter drawer) ---- */

function renderFieldFilters() {
  const bar = document.getElementById("ffBar");
  if (!bar) return;
  const allowed = typeof canFieldFilter === "function" ? canFieldFilter() : true;
  if (!allowed || !SEL_COLS.length) { bar.innerHTML = ""; return; }

  // Drop cascade values whose dependencies are not satisfied (transitively too)
  SEL_COLS.forEach(name => {
    const fk = fieldFilterKind(findField(SCHEMA, name));
    if (fk.kind === "cascade" && fk.deps.find(dep => !FIELD_EXACT_FILTERS[dep]))
      delete FIELD_EXACT_FILTERS[name];
    if (fk.kind === "multicascade" && fk.deps.find(dep => !FIELD_EXACT_FILTERS[dep]))
      delete FIELD_ANY_FILTERS[name];
  });

  const parts = SEL_COLS.map((name, i) => {
    const fd = findField(SCHEMA, name);
    const label = (ALL_FIELDS.find(ff => ff.name === name) || {}).label || name;
    const sep = i > 0 ? '<div class="filter-sep"></div>' : "";
    const fk = fieldFilterKind(fd);

    if (fk.kind === "select") {
      const curVal = FIELD_EXACT_FILTERS[name] || "";
      const optHtml = `<option value="">All</option>` +
        fk.opts.map(o => `<option value="${esc(o.value)}"${curVal === o.value ? " selected" : ""}>${esc(o.label)}</option>`).join("");
      return `${sep}<div class="ff-group">
        <span class="ff-lbl">${esc(label)}</span>
        <select class="ff-input" data-field="${esc(name)}" onchange="onFieldSelect(this)">${optHtml}</select>
      </div>`;
    }
    if (fk.kind === "multiselect") {
      const curVals = FIELD_ANY_FILTERS[name] || [];
      const optHtml = fk.opts.map(o => `<option value="${esc(o.value)}"${curVals.includes(o.value) ? " selected" : ""}>${esc(o.label)}</option>`).join("");
      return `${sep}<div class="ff-group">
        <span class="ff-lbl">${esc(label)}</span>
        <select class="ff-input" multiple data-field="${esc(name)}" onchange="onFieldMultiSelect(this)">${optHtml}</select>
      </div>`;
    }
    if (fk.kind === "api" || fk.kind === "multiapi") {
      // Render a placeholder first, fill it in after the fetch
      const multiAttr = fk.kind === "multiapi" ? " multiple" : "";
      return `${sep}<div class="ff-group">
        <span class="ff-lbl">${esc(label)}</span>
        <select class="ff-input"${multiAttr} data-field="${esc(name)}" data-api-src="${esc(JSON.stringify(fk))}"
                onchange="${fk.kind === "multiapi" ? "onFieldMultiSelect(this)" : "onFieldSelect(this)"}" disabled>
          <option value="">Loading…</option>
        </select>
      </div>`;
    }
    if (fk.kind === "cascade" || fk.kind === "multicascade") {
      const isMulti = fk.kind === "multicascade";
      const missingDep = fk.deps.find(dep => !FIELD_EXACT_FILTERS[dep]);
      if (missingDep) {
        const depLabel = (ALL_FIELDS.find(ff => ff.name === missingDep) || {}).label || missingDep;
        return `${sep}<div class="ff-group">
          <span class="ff-lbl">${esc(label)}</span>
          <select class="ff-input"${isMulti ? " multiple" : ""} data-field="${esc(name)}" disabled>
            <option value="">— select ${esc(depLabel)} first —</option>
          </select>
        </div>`;
      }
      const resolvedUrl = fk.urlTpl.replace(/\{([^}]+)\}/g, (_, k) => encodeURIComponent(FIELD_EXACT_FILTERS[k] || ""));
      const src = { url: resolvedUrl, valueField: fk.valueField, labelField: fk.labelField, path: fk.path };
      return `${sep}<div class="ff-group">
        <span class="ff-lbl">${esc(label)}</span>
        <select class="ff-input"${isMulti ? " multiple" : ""} data-field="${esc(name)}" data-api-src="${esc(JSON.stringify(src))}"
                onchange="${isMulti ? "onFieldMultiSelect(this)" : "onFieldSelect(this)"}" disabled>
          <option value="">Loading…</option>
        </select>
      </div>`;
    }
    if (fk.kind === "date" || fk.kind === "datetime" || fk.kind === "time") {
      const inputType = fk.kind === "datetime" ? "datetime-local" : fk.kind;
      const curVal = FIELD_EXACT_FILTERS[name] || "";
      return `${sep}<div class="ff-group">
        <span class="ff-lbl">${esc(label)}</span>
        <input type="${inputType}" class="ff-input" style="width:auto" value="${esc(curVal)}"
               data-field="${esc(name)}" onchange="onFieldSelect(this)">
      </div>`;
    }
    if (fk.kind === "number") {
      const cur = FIELD_RANGE_FILTERS[name] || { min: "", max: "" };
      return `${sep}<div class="ff-group">
        <span class="ff-lbl">${esc(label)}</span>
        <input type="number" class="ff-input" style="width:72px" placeholder="Min" value="${esc(cur.min)}"
               data-field="${esc(name)}" data-bound="min" oninput="onFieldRangeInput(this)">
        <span style="color:var(--muted)">–</span>
        <input type="number" class="ff-input" style="width:72px" placeholder="Max" value="${esc(cur.max)}"
               data-field="${esc(name)}" data-bound="max" oninput="onFieldRangeInput(this)">
      </div>`;
    }
    // text
    const curVal = FIELD_FILTERS[name] || "";
    return `${sep}<div class="ff-group">
      <span class="ff-lbl">${esc(label)}</span>
      <input class="ff-input" placeholder="Filter…" value="${esc(curVal)}"
             data-field="${esc(name)}" oninput="onFieldInput(this)">
    </div>`;
  });
  bar.innerHTML = parts.join("");

  // Populate API dropdowns asynchronously (both single and multi-select)
  bar.querySelectorAll("select[data-api-src]").forEach(async sel => {
    const src = JSON.parse(sel.dataset.apiSrc);
    const opts = await fetchApiOpts(src);
    if (sel.multiple) {
      const curVals = FIELD_ANY_FILTERS[sel.dataset.field] || [];
      sel.innerHTML = opts.map(o => `<option value="${esc(o.value)}"${curVals.includes(o.value) ? " selected" : ""}>${esc(o.label)}</option>`).join("");
    } else {
      const curVal = FIELD_EXACT_FILTERS[sel.dataset.field] || "";
      sel.innerHTML = `<option value="">All</option>` +
        opts.map(o => `<option value="${esc(o.value)}"${curVal === o.value ? " selected" : ""}>${esc(o.label)}</option>`).join("");
    }
    sel.disabled = false;
  });
}

/* ---- active filter summary ---- */

// labelForValue turns a filter value into human-readable text — region code "6472"
// becomes "SAMARINDA" — by consulting the options already fetched.
function labelForValue(fieldName, v) {
  const fk = fieldFilterKind(findField(SCHEMA, fieldName));
  if (fk.kind === "select" || fk.kind === "multiselect") {
    const o = (fk.opts || []).find(x => x.value === v);
    if (o) return o.label;
  } else if ((fk.kind === "api" || fk.kind === "multiapi") && _apiOptCache[fk.url]) {
    const o = _apiOptCache[fk.url].find(x => x.value === v);
    if (o) return o.label;
  } else if (fk.kind === "cascade" || fk.kind === "multicascade") {
    const ru = fk.urlTpl.replace(/\{([^}]+)\}/g, (_, dk) => encodeURIComponent(FIELD_EXACT_FILTERS[dk] || ""));
    if (_apiOptCache[ru]) {
      const o = _apiOptCache[ru].find(x => x.value === v);
      if (o) return o.label;
    }
  }
  return v;
}

function updateFilterUI() {
  const parts = [];
  const labelOf = k => {
    const f = ALL_FIELDS.find(ff => ff.name === k);
    return esc(f ? f.label : k);
  };

  if (FILTER_STATUS) parts.push(FILTER_STATUS === "submitted" ? "Submitted" : "Draft");
  // The share filter only exists on the admin page.
  if (typeof FILTER_SHARE !== "undefined" && FILTER_SHARE)
    parts.push(esc(SHARE_MAP[FILTER_SHARE] || "Share tertentu"));
  if (FILTER_SEARCH) parts.push(`"${esc(FILTER_SEARCH)}"`);

  Object.entries(FIELD_FILTERS).forEach(([k, v]) => parts.push(`${labelOf(k)}: "${esc(v)}"`));
  Object.entries(FIELD_EXACT_FILTERS).forEach(([k, v]) => parts.push(`${labelOf(k)}: ${esc(labelForValue(k, v))}`));
  Object.entries(FIELD_ANY_FILTERS).forEach(([k, v]) =>
    parts.push(`${labelOf(k)}: ${esc(v.map(val => labelForValue(k, val)).join(", "))}`));
  Object.entries(FIELD_RANGE_FILTERS).forEach(([k, v]) => {
    const text = v.min && v.max ? `${v.min}–${v.max}` : (v.min ? `≥ ${v.min}` : `≤ ${v.max}`);
    parts.push(`${labelOf(k)}: ${esc(text)}`);
  });

  // Shown as chips beneath the top bar plus a count on the Filter button
  // (see responses-ui.js).
  applyFilterSummary(parts);
}

/* ---- table header ---- */

// responsive-tables.js uses data-label as the column label when the table turns into
// cards on phones — without it the sort icon (↕) would end up in the label.
function sortTh(col, label, cls) {
  const active = SORT_BY === col;
  const ic = active ? (SORT_DIR === "asc" ? "↑" : "↓") : "↕";
  return `<th class="sortable${active ? " sort-active" : ""}${cls ? " " + cls : ""}" data-label="${label}" onclick="toggleSort('${col}')">${label}<span class="sort-ic">${ic}</span></th>`;
}
