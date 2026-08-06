/* Column labels for tables that turn into cards on phone screens.

   On narrow screens, <table class="cards"> is shown as a list of cards
   (see the @media block in admin.css). Because <thead> is hidden, every <td>
   needs its own label — this script copies the <th> text into a data-l attribute
   so the CSS can render it via ::before.

   It deliberately reads the already-rendered <th> rather than a hard-coded list,
   so the labels follow along when i18n.js switches the page
   language. */
(function () {
  function syncTable(table) {
    // Columns hidden via an inline style (the "Responses" column for the editor role,
    // for instance) are skipped, so the label order stays aligned.
    const labels = [];
    table.querySelectorAll("thead th").forEach(th => {
      if (th.style.display === "none") return;
      // data-label is used when the <th> contains decorative elements that must not
      // become part of the label — the "↕" sort icon on a sortable header, say.
      labels.push((th.getAttribute("data-label") || th.textContent).trim());
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
    // The table body is re-rendered by admin.js/manage.js after a fetch, and the
    // The <th> changes when the language switches — watch for both.
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
