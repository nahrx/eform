/* Menambahkan kotak pencarian di atas semua elemen <select> di halaman,
   supaya dropdown dengan banyak opsi (mis. daftar wilayah dari API) mudah dicari.
   Bekerja secara progressive-enhancement: <select> asli tetap ada di DOM (disembunyikan
   secara visual) sebagai sumber nilai, supaya semua kode yang sudah ada (onchange,
   addEventListener("change", ...), pembacaan `.value` saat submit) tetap berfungsi
   tanpa perlu diubah. Deteksi <select> baru/berubah dilakukan otomatis lewat
   MutationObserver, jadi cukup include skrip ini sekali per halaman. */
(function () {
  if (window.__searchableSelectInit) return;
  window.__searchableSelectInit = true;

  const style = document.createElement("style");
  style.textContent = `
.ss-wrap{position:relative;max-width:100%}
.ss-native{position:absolute!important;left:-9999px!important;top:auto!important;width:1px!important;height:1px!important;opacity:0!important;pointer-events:none!important}
.ss-ctrl{display:flex;align-items:center;justify-content:space-between;gap:8px;
  border:1.5px solid var(--line,#dfe4ea);border-radius:var(--radius-s,7px);
  padding:9.5px 12px;background:var(--panel,#fff);color:var(--ink,#13171e);
  font-family:inherit;font-size:13.5px;cursor:pointer;text-align:left;box-sizing:border-box;
  transition:border-color .15s,box-shadow .15s}
.ss-ctrl.ss-open,.ss-ctrl:focus-visible{outline:none;border-color:var(--accent,#0e7490);box-shadow:0 0 0 3px var(--accent-soft,#d6edf1)}
.ss-ctrl[disabled]{opacity:.6;cursor:not-allowed;background:var(--panel-2,#f7f9fb)}
.ss-ctrl-label{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;flex:1}
.ss-ctrl-label.ss-placeholder{color:var(--muted,#79828f)}
.ss-ctrl-arrow{font-size:11px;color:var(--muted,#79828f);flex:none}
.ss-panel{position:absolute;top:calc(100% + 4px);left:0;right:0;z-index:200;
  background:var(--panel,#fff);border:1px solid var(--line,#dfe4ea);border-radius:var(--radius-s,7px);
  box-shadow:0 8px 30px -8px rgba(19,23,30,.28);overflow:hidden}
.ss-search{width:100%;box-sizing:border-box;border:none;border-bottom:1px solid var(--line,#dfe4ea);
  padding:9px 12px;font-size:13.5px;font-family:inherit;color:var(--ink,#13171e);outline:none;background:var(--panel,#fff)}
.ss-list{list-style:none;margin:0;padding:4px;max-height:230px;overflow-y:auto}
.ss-opt{padding:7px 10px;border-radius:6px;font-size:13.5px;cursor:pointer;color:var(--ink,#13171e)}
.ss-opt.ss-active{background:var(--accent-soft,#d6edf1);color:var(--accent-ink,#0b5563)}
.ss-opt.ss-selected{font-weight:600}
.ss-empty{padding:10px 12px;font-size:12.5px;color:var(--muted,#79828f);text-align:center}
/* --- pilihan ganda: tiap baris jadi kotak centang --- */
.ss-opt.ss-check{display:flex;align-items:center;gap:9px;font-weight:400}
.ss-opt.ss-check.ss-selected{font-weight:500}
.ss-box{flex:none;width:16px;height:16px;border:1.5px solid var(--line,#dfe4ea);border-radius:4px;
  display:grid;place-items:center;background:var(--panel,#fff);transition:background .12s,border-color .12s}
.ss-opt.ss-selected .ss-box{background:var(--accent,#0e7490);border-color:var(--accent,#0e7490)}
.ss-box::after{content:"";width:9px;height:5px;opacity:0;
  border-left:2px solid #fff;border-bottom:2px solid #fff;transform:rotate(-45deg) translateY(-1px)}
.ss-opt.ss-selected .ss-box::after{opacity:1}
.ss-foot{display:flex;align-items:center;justify-content:space-between;gap:8px;
  padding:7px 10px;border-top:1px solid var(--line,#dfe4ea);font-size:12px;color:var(--muted,#79828f)}
.ss-clear{border:none;background:none;color:var(--accent,#0e7490);font:inherit;font-size:12px;
  cursor:pointer;padding:3px 7px;border-radius:5px}
.ss-clear:hover{background:var(--accent-soft,#d6edf1)}
.ss-clear[disabled]{opacity:.45;cursor:default;background:none}
`;
  document.head.appendChild(style);

  function escHtml(s) {
    return String(s).replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
  }
  function optList(select) {
    return Array.from(select.options).map((o) => ({ value: o.value, label: o.textContent || "" }));
  }
  function closePanel(wrap) {
    wrap.classList.remove("ss-is-open");
    wrap.querySelector(".ss-ctrl").classList.remove("ss-open");
    wrap.querySelector(".ss-panel").hidden = true;
  }
  function closeAllExcept(except) {
    document.querySelectorAll(".ss-wrap.ss-is-open").forEach((w) => {
      if (w !== except) closePanel(w);
    });
  }
  // Nilai yang sedang terpilih. Untuk pilihan ganda bisa lebih dari satu.
  function selectedValues(select) {
    return Array.from(select.selectedOptions || []).map((o) => o.value);
  }

  function renderList(wrap, query) {
    const select = wrap.querySelector("select"),
      list = wrap.querySelector(".ss-list");
    const multi = select.multiple;
    const q = query.trim().toLowerCase();
    const opts = optList(select);
    const filtered = q ? opts.filter((o) => o.label.toLowerCase().includes(q)) : opts;
    if (!filtered.length) {
      list.innerHTML = '<li class="ss-empty">Tidak ditemukan</li>';
      updateFoot(wrap);
      return;
    }
    const chosen = new Set(multi ? selectedValues(select) : [select.value]);
    list.innerHTML = filtered
      .map((o, i) => {
        const sel = chosen.has(o.value);
        const cls =
          "ss-opt" + (multi ? " ss-check" : "") + (sel ? " ss-selected" : "") + (i === 0 ? " ss-active" : "");
        const body = multi
          ? `<span class="ss-box"></span><span>${escHtml(o.label) || "&nbsp;"}</span>`
          : escHtml(o.label) || "&nbsp;";
        return `<li class="${cls}" data-value="${escHtml(o.value)}" role="option" aria-selected="${sel}">${body}</li>`;
      })
      .join("");
    updateFoot(wrap);
  }

  // Kaki panel hanya dipakai mode pilihan ganda: jumlah terpilih + tombol bersihkan.
  function updateFoot(wrap) {
    const foot = wrap.querySelector(".ss-foot");
    if (!foot) return;
    const n = selectedValues(wrap.querySelector("select")).filter(Boolean).length;
    foot.querySelector(".ss-foot-count").textContent = n ? n + " dipilih" : "Belum ada dipilih";
    foot.querySelector(".ss-clear").disabled = n === 0;
  }
  function moveActive(list, dir) {
    const items = [...list.querySelectorAll(".ss-opt")];
    if (!items.length) return;
    let idx = items.findIndex((i) => i.classList.contains("ss-active"));
    if (idx >= 0) items[idx].classList.remove("ss-active");
    idx = (idx + dir + items.length) % items.length;
    items[idx].classList.add("ss-active");
    items[idx].scrollIntoView({ block: "nearest" });
  }
  function updateCtrl(wrap) {
    const select = wrap.querySelector("select"),
      label = wrap.querySelector(".ss-ctrl-label"),
      ctrl = wrap.querySelector(".ss-ctrl");

    if (select.multiple) {
      // Tampilkan nama pilihannya selama masih muat; lebih dari dua cukup jumlahnya.
      const chosen = Array.from(select.selectedOptions).filter((o) => o.value !== "");
      let txt;
      if (!chosen.length) txt = "— pilih —";
      else if (chosen.length <= 2) txt = chosen.map((o) => o.textContent).join(", ");
      else txt = chosen.length + " dipilih";
      label.textContent = txt;
      label.classList.toggle("ss-placeholder", chosen.length === 0);
      ctrl.disabled = select.disabled;
      updateFoot(wrap);
      return;
    }

    const opt = select.options[select.selectedIndex];
    const txt = opt ? opt.textContent : "";
    label.textContent = select.value ? txt : txt || "— pilih —";
    label.classList.toggle("ss-placeholder", !select.value);
    ctrl.disabled = select.disabled;
  }

  function fire(select) {
    select.dispatchEvent(new Event("input", { bubbles: true }));
    select.dispatchEvent(new Event("change", { bubbles: true }));
  }

  function pick(wrap, value) {
    const select = wrap.querySelector("select");

    // Pilihan ganda: centang/hapus centang, panel dibiarkan terbuka supaya beberapa
    // nilai bisa dipilih beruntun.
    if (select.multiple) {
      const opt = Array.from(select.options).find((o) => o.value === value);
      if (!opt) return;
      opt.selected = !opt.selected;
      fire(select);
      updateCtrl(wrap);
      renderList(wrap, wrap.querySelector(".ss-search").value);
      // Kembalikan penanda baris aktif ke opsi yang barusan dicentang, supaya
      // navigasi panah/Enter tidak melompat ke atas tiap kali mencentang.
      const items = [...wrap.querySelectorAll(".ss-list .ss-opt")];
      items.forEach((li) => li.classList.remove("ss-active"));
      const same = items.find((li) => li.dataset.value === value);
      if (same) same.classList.add("ss-active");
      return;
    }

    if (select.value !== value) {
      select.value = value;
      fire(select);
    }
    updateCtrl(wrap);
    closePanel(wrap);
  }

  function clearAll(wrap) {
    const select = wrap.querySelector("select");
    if (!selectedValues(select).filter(Boolean).length) return;
    Array.from(select.options).forEach((o) => (o.selected = false));
    fire(select);
    updateCtrl(wrap);
    renderList(wrap, wrap.querySelector(".ss-search").value);
  }
  function openPanel(wrap) {
    const ctrl = wrap.querySelector(".ss-ctrl");
    if (ctrl.disabled) return;
    closeAllExcept(wrap);
    const panel = wrap.querySelector(".ss-panel"),
      search = wrap.querySelector(".ss-search");
    wrap.classList.add("ss-is-open");
    ctrl.classList.add("ss-open");
    panel.hidden = false;
    search.value = "";
    renderList(wrap, "");
    requestAnimationFrame(() => {
      search.focus();
      // Kalau dropdown terbuka di dekat dasar wadah yang bisa digulir (mis. laci
      // filter), panelnya bisa tertutup kaki laci — geser secukupnya agar terlihat.
      panel.scrollIntoView({ block: "nearest" });
    });
  }
  function refresh(select) {
    const wrap = select.closest(".ss-wrap");
    if (!wrap) return;
    updateCtrl(wrap);
    if (wrap.classList.contains("ss-is-open")) renderList(wrap, wrap.querySelector(".ss-search").value);
  }
  function enhance(select) {
    if (!select || select.tagName !== "SELECT") return;
    if (select.closest(".ss-wrap")) return;

    // Salin class asli select (mis. "ff-input", "pv-in", "filter-sel") ke tombol
    // pengganti, supaya lebar/ukuran yang sudah diatur halaman (max-width, width
    // tetap, dst.) tetap berlaku — bukan selalu melebar penuh (width:100%).
    const originalClasses = select.className;

    const wrap = document.createElement("div");
    wrap.className = "ss-wrap";
    select.parentNode.insertBefore(wrap, select);
    wrap.appendChild(select);
    select.classList.add("ss-native");
    select.tabIndex = -1;

    const ctrl = document.createElement("button");
    ctrl.type = "button";
    ctrl.className = ("ss-ctrl " + originalClasses).trim();
    ctrl.innerHTML = '<span class="ss-ctrl-label"></span><span class="ss-ctrl-arrow">▾</span>';
    wrap.appendChild(ctrl);

    const panel = document.createElement("div");
    panel.className = "ss-panel";
    panel.hidden = true;
    panel.innerHTML =
      '<input type="text" class="ss-search" placeholder="Cari…" autocomplete="off">' +
      '<ul class="ss-list" role="listbox"' + (select.multiple ? ' aria-multiselectable="true"' : "") + "></ul>" +
      (select.multiple
        ? '<div class="ss-foot"><span class="ss-foot-count"></span><button type="button" class="ss-clear">Bersihkan</button></div>'
        : "");
    wrap.appendChild(panel);

    const search = panel.querySelector(".ss-search"),
      list = panel.querySelector(".ss-list"),
      clearBtn = panel.querySelector(".ss-clear");
    if (clearBtn) clearBtn.addEventListener("click", () => clearAll(wrap));

    ctrl.addEventListener("click", () => {
      if (wrap.classList.contains("ss-is-open")) closePanel(wrap);
      else openPanel(wrap);
    });
    search.addEventListener("input", () => renderList(wrap, search.value));
    search.addEventListener("keydown", (e) => {
      if (e.key === "Escape") {
        closePanel(wrap);
        ctrl.focus();
      } else if (e.key === "Enter") {
        e.preventDefault();
        const active = list.querySelector(".ss-active");
        if (active) pick(wrap, active.dataset.value);
      } else if (e.key === "ArrowDown") {
        e.preventDefault();
        moveActive(list, 1);
      } else if (e.key === "ArrowUp") {
        e.preventDefault();
        moveActive(list, -1);
      }
    });
    list.addEventListener("mousedown", (e) => {
      const li = e.target.closest(".ss-opt");
      if (li && li.dataset.value != null) pick(wrap, li.dataset.value);
    });

    updateCtrl(wrap);
  }
  function scan(root) {
    if (!root || !root.querySelectorAll) return;
    root.querySelectorAll("select").forEach(enhance);
  }

  document.addEventListener("click", (e) => {
    document.querySelectorAll(".ss-wrap.ss-is-open").forEach((wrap) => {
      if (!wrap.contains(e.target)) closePanel(wrap);
    });
  });

  const mo = new MutationObserver((muts) => {
    muts.forEach((m) => {
      if (m.type !== "childList") return;
      m.addedNodes.forEach((n) => {
        if (n.nodeType !== 1) return;
        if (n.tagName === "SELECT") enhance(n);
        else scan(n);
      });
      if (m.target && m.target.tagName === "SELECT") refresh(m.target);
    });
  });
  mo.observe(document.documentElement, { childList: true, subtree: true });

  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", () => scan(document));
  else scan(document);
})();
