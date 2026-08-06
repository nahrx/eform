/* The shared filter drawer (right sidebar) for the three response-list pages:
   responses.html (admin), viewer-responses.html, and editor-responses.html.

   The division of labour: the filter logic — which values are active, how data is
   reloaded — stays with each page. This file only handles the presentation: opening
   and closing the drawer, and showing a summary of the active filters.

   Each page calls applyFilterSummary(parts) from its own updateFilterUI. */
(function () {
  const el = id => document.getElementById(id);

  function setDrawer(open) {
    const drawer = el("filterDrawer");
    const backdrop = el("drawerBackdrop");
    if (!drawer) return;
    drawer.classList.toggle("open", open);
    drawer.setAttribute("aria-hidden", open ? "false" : "true");
    if (backdrop) backdrop.classList.toggle("open", open);
    document.body.classList.toggle("drawer-open", open);
    if (open) {
      // Focus the first input so the drawer is immediately usable from the keyboard.
      const first = drawer.querySelector("select, input");
      if (first) first.focus({ preventScroll: true });
    } else if (el("btnFilter")) {
      el("btnFilter").focus({ preventScroll: true });
    }
  }

  window.openFilterDrawer = () => setDrawer(true);
  window.closeFilterDrawer = () => setDrawer(false);

  /* applyFilterSummary shows the active filters in two places: the small count on
     the Filter button, and the row of chips beneath the top bar.
     `parts` holds HTML-safe fragments, already escaped by the caller with esc(). */
  window.applyFilterSummary = function (parts) {
    parts = parts || [];
    const count = el("filterCount");
    if (count) {
      count.textContent = parts.length;
      count.hidden = parts.length === 0;
    }
    const btn = el("btnFilter");
    if (btn) btn.classList.toggle("has-filter", parts.length > 0);

    const bar = el("activeBar");
    const chips = el("filterChips");
    if (!bar || !chips) return;
    bar.hidden = parts.length === 0;
    chips.innerHTML = parts.map(p => `<span class="filter-chip">${p}</span>`).join("");
  };

  /* ---- dropdown pilihan format ekspor ---- */

  // The Export button replaces two separate buttons (CSV & Excel) to keep the top bar
  // from filling up, especially on phones. The page supplies downloadExport(ext); this
  // file only handles opening and closing the menu.
  function setExportMenu(open) {
    const menu = el("exportMenu"), btn = el("btnExport");
    if (!menu || !btn) return;
    menu.hidden = !open;
    btn.setAttribute("aria-expanded", open ? "true" : "false");
    if (!open) return;

    // The menu is right-aligned to its button. On a narrow screen the Export button can
    // sit near the left edge, which would push the menu off-screen to the left —
    // in that case the anchoring is flipped.
    menu.style.left = "";
    menu.style.right = "";
    const r = menu.getBoundingClientRect();
    if (r.left < 4) {
      menu.style.left = "0";
      menu.style.right = "auto";
    } else if (r.right > document.documentElement.clientWidth - 4) {
      menu.style.left = "auto";
      menu.style.right = "0";
    }
    menu.querySelector(".export-item")?.focus({ preventScroll: true });
  }

  function wireExport() {
    const btn = el("btnExport"), menu = el("exportMenu");
    if (!btn || !menu) return;

    btn.addEventListener("click", e => {
      e.stopPropagation();
      setExportMenu(menu.hidden);
    });
    menu.addEventListener("click", e => {
      const item = e.target.closest(".export-item");
      if (!item) return;
      setExportMenu(false);
      if (typeof window.downloadExport === "function") window.downloadExport(item.dataset.ext);
    });
    document.addEventListener("click", e => {
      if (!menu.hidden && !menu.contains(e.target) && e.target !== btn) setExportMenu(false);
    });
    document.addEventListener("keydown", e => {
      if (e.key === "Escape" && !menu.hidden) {
        setExportMenu(false);
        btn.focus({ preventScroll: true });
      }
    });
  }

  function wire() {
    wireExport();
    el("btnFilter")?.addEventListener("click", () => setDrawer(true));
    el("btnDrawerClose")?.addEventListener("click", () => setDrawer(false));
    el("btnDrawerDone")?.addEventListener("click", () => setDrawer(false));
    el("drawerBackdrop")?.addEventListener("click", () => setDrawer(false));
    document.addEventListener("keydown", e => {
      if (e.key === "Escape" && el("filterDrawer")?.classList.contains("open")) setDrawer(false);
    });
  }

  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", wire);
  else wire();
})();
