/* Mounts an interactive map (Leaflet + OpenStreetMap tiles) onto every
   <div class="geo-map" data-lat="-1.23" data-lng="116.4"></div> that appears on
   the page — used for geopoint fields, both while filling in a form and in the
   response detail views (admin/viewer/editor).
   New and changed elements are detected automatically via MutationObserver, just like
   searchable-select.js. Must be loaded AFTER /vendor/leaflet/leaflet.js. */
(function () {
  if (window.__geoMapInit) return;
  window.__geoMapInit = true;

  const style = document.createElement("style");
  style.textContent = `
.geo-map{width:100%;height:220px;border-radius:var(--radius-s,7px);border:1.5px solid var(--line,#dfe4ea);overflow:hidden;margin-top:8px;background:var(--panel-2,#f7f9fb)}
.geo-map .leaflet-control-attribution{font-size:10px}
`;
  document.head.appendChild(style);

  function setDefaultIcon() {
    if (window.L && window.L.Icon && window.L.Icon.Default) {
      // Icon.Default._getIconUrl concatenates imagePath + the option's file name
      // (iconUrl/iconRetinaUrl/shadowUrl) — do not point those options at a path
      // absolute path; just set imagePath and leave the default file names alone.
      window.L.Icon.Default.mergeOptions({
        imagePath: "/vendor/leaflet/images/",
      });
      return true;
    }
    return false;
  }

  function parseCoord(v) {
    const n = Number(v);
    return Number.isFinite(n) ? n : null;
  }

  function mount(el) {
    if (el._geoMap || typeof window.L === "undefined") return;
    const lat = parseCoord(el.dataset.lat),
      lng = parseCoord(el.dataset.lng);
    if (lat == null || lng == null) return;
    const map = window.L.map(el, { attributionControl: true }).setView([lat, lng], 16);
    window.L.tileLayer("https://tile.openstreetmap.org/{z}/{x}/{y}.png", {
      maxZoom: 19,
      attribution:
        '&copy; <a href="https://www.openstreetmap.org/copyright" target="_blank" rel="noopener">OpenStreetMap</a> contributors',
    }).addTo(map);
    el._geoMap = map;
    el._geoMarker = window.L.marker([lat, lng]).addTo(map);
  }

  function refresh(el) {
    if (!el._geoMap) {
      mount(el);
      return;
    }
    const lat = parseCoord(el.dataset.lat),
      lng = parseCoord(el.dataset.lng);
    if (lat == null || lng == null) return;
    el._geoMap.setView([lat, lng], el._geoMap.getZoom());
    el._geoMarker.setLatLng([lat, lng]);
  }

  function scan(root) {
    if (!root || !root.querySelectorAll) return;
    root.querySelectorAll(".geo-map").forEach(mount);
  }

  const mo = new MutationObserver((muts) => {
    muts.forEach((m) => {
      if (m.type === "childList") {
        m.addedNodes.forEach((n) => {
          if (n.nodeType !== 1) return;
          if (n.classList && n.classList.contains("geo-map")) mount(n);
          else scan(n);
        });
      } else if (
        m.type === "attributes" &&
        m.target.classList &&
        m.target.classList.contains("geo-map")
      ) {
        refresh(m.target);
      }
    });
  });
  mo.observe(document.documentElement, {
    childList: true,
    subtree: true,
    attributes: true,
    attributeFilter: ["data-lat", "data-lng"],
  });

  function boot() {
    if (setDefaultIcon()) {
      scan(document);
      return;
    }
    // leaflet.js has not loaded yet (script order, for instance) — retry shortly.
    let tries = 0;
    const t = setInterval(() => {
      tries++;
      if (setDefaultIcon()) {
        scan(document);
        clearInterval(t);
      } else if (tries > 20) {
        clearInterval(t);
      }
    }, 150);
  }
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", boot);
  else boot();
})();
