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

  function wire() {
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
