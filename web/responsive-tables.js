/* Label kolom untuk tabel yang berubah jadi kartu di layar HP.

   Di layar sempit, <table class="cards"> ditampilkan sebagai daftar kartu
   (lihat blok @media di admin.css). Karena <thead> disembunyikan, tiap <td>
   perlu labelnya sendiri — skrip ini menyalin teks <th> ke atribut data-l
   supaya CSS bisa menampilkannya lewat ::before.

   Sengaja dibaca dari <th> yang sudah dirender, bukan dari daftar teks yang
   ditulis ulang di sini, supaya label ikut berubah saat i18n.js mengganti
   bahasa halaman. */
(function () {
  function syncTable(table) {
    // Kolom yang disembunyikan lewat style inline (mis. kolom "Jawaban" untuk
    // role editor) tidak ikut dihitung, supaya urutan label tetap pas.
    const labels = [];
    table.querySelectorAll("thead th").forEach(th => {
      if (th.style.display === "none") return;
      labels.push(th.textContent.trim());
    });
    table.querySelectorAll("tbody tr").forEach(tr => {
      Array.prototype.forEach.call(tr.children, (td, i) => {
        const label = td.colSpan > 1 ? "" : labels[i] || "";
        if (label) {
          if (td.getAttribute("data-l") !== label) td.setAttribute("data-l", label);
        } else if (td.hasAttribute("data-l")) {
          td.removeAttribute("data-l");
        }
      });
    });
  }

  function syncAll() {
    document.querySelectorAll("table.cards").forEach(syncTable);
  }
  window.syncCardLabels = syncAll;

  let timer = null;
  function schedule() {
    clearTimeout(timer);
    timer = setTimeout(syncAll, 60);
  }

  function start() {
    syncAll();
    // Isi tabel dirender ulang oleh admin.js/manage.js setelah fetch, dan teks
    // <th> berubah saat ganti bahasa — pantau keduanya.
    new MutationObserver(schedule).observe(document.body, {
      childList: true,
      subtree: true,
      characterData: true,
    });
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", start);
  } else {
    start();
  }
})();
