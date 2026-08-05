/* Laci filter (right sidebar) bersama untuk ketiga halaman daftar jawaban:
   responses.html (admin), viewer-responses.html, dan editor-responses.html.

   Pembagian tugasnya: logika filter — nilai apa yang aktif, bagaimana memuat ulang
   data — tetap milik masing-masing halaman. File ini hanya mengurus tampilannya:
   buka/tutup laci, dan menampilkan ringkasan filter yang sedang aktif.

   Halaman memanggil applyFilterSummary(parts) dari updateFilterUI miliknya. */
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
      // Fokuskan kolom isian pertama supaya laci langsung bisa dipakai dari keyboard.
      const first = drawer.querySelector("select, input");
      if (first) first.focus({ preventScroll: true });
    } else if (el("btnFilter")) {
      el("btnFilter").focus({ preventScroll: true });
    }
  }

  window.openFilterDrawer = () => setDrawer(true);
  window.closeFilterDrawer = () => setDrawer(false);

  /* applyFilterSummary menampilkan filter yang sedang aktif di dua tempat:
     angka kecil pada tombol Filter, dan deretan chip di bawah bilah atas.
     `parts` berisi potongan teks yang sudah aman untuk HTML (dirakit pemanggil
     dengan esc()). */
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

  // Tombol Ekspor menggantikan dua tombol terpisah (CSV & Excel) supaya bilah atas
  // tidak penuh — terutama di HP. Halaman menyediakan downloadExport(ext); di sini
  // hanya urusan buka/tutup menunya.
  function setExportMenu(open) {
    const menu = el("exportMenu"), btn = el("btnExport");
    if (!menu || !btn) return;
    menu.hidden = !open;
    btn.setAttribute("aria-expanded", open ? "true" : "false");
    if (!open) return;

    // Menu rata kanan terhadap tombolnya. Di layar sempit tombol Ekspor bisa
    // berada dekat tepi kiri, sehingga menu justru keluar layar di sebelah kiri —
    // pada kasus itu penjangkarannya dibalik ke kiri.
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
