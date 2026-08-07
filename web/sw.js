/* Service worker for the offline mode of multi-response forms (PWA).

   The queue itself lives in /offline-queue.js, shared with web/public.html — see the
   comment at the top of that file for why. This worker only decides what to cache
   and when to run a flush. */
importScripts("/offline-queue.js");

const Q = self.EformOfflineQueue;
const CACHE_NAME = "eform-v3";

/* Assets the form page needs but which are not part of any API response. Without
   them an offline page still renders, but region dropdowns lose their search box,
   geopoint fields lose their map, and photos queue up at full camera size — so they
   are fetched during install rather than waiting for a second visit. */
const PRECACHE = [
  "/offline-queue.js",
  "/searchable-select.js",
  "/geo-map.js",
  "/image-compress.js",
  "/vendor/leaflet/leaflet.js",
  "/vendor/leaflet/leaflet.css",
];

const CACHEABLE_GET = [
  /^\/f\/[^/]+$/,
  /^\/api\/public\/forms\/[^/]+$/,
  /^\/api\/wilayah(\?.*)?$/,
  /^\/api\/options-proxy\?.*$/,
  /^\/offline-queue\.js$/,
  /^\/searchable-select\.js$/,
  /^\/geo-map\.js$/,
  /^\/vendor\/leaflet\/.*$/,
];

self.addEventListener("install", (event) => {
  // Each asset is added individually: with addAll a single failure would throw
  // away the whole pre-cache.
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) =>
      Promise.all(PRECACHE.map((u) => cache.add(u).catch(() => {})))
    )
  );
  self.skipWaiting();
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((names) =>
        Promise.all(names.filter((n) => n !== CACHE_NAME).map((n) => caches.delete(n)))
      )
      .then(() => self.clients.claim())
  );
});

self.addEventListener("fetch", (event) => {
  const req = event.request;
  if (req.method !== "GET") return;
  const url = new URL(req.url);
  if (url.origin !== self.location.origin) return;
  const path = url.pathname + (url.search || "");
  if (!CACHEABLE_GET.some((re) => re.test(path))) return;

  event.respondWith(
    fetch(req)
      .then((res) => {
        if (res && res.ok) {
          const copy = res.clone();
          caches.open(CACHE_NAME).then((cache) => cache.put(req, copy));
        }
        return res;
      })
      .catch(async () => {
        const cached = await caches.match(req);
        if (cached) return cached;
        return new Response(JSON.stringify({ error: "offline" }), {
          status: 503,
          headers: { "Content-Type": "application/json" },
        });
      })
  );
});

self.addEventListener("sync", (event) => {
  if (event.tag === "eform-flush") event.waitUntil(runFlush());
});

async function runFlush() {
  let r;
  try {
    r = await Q.flush();
  } catch (_e) {
    return;
  }
  // The page keeps its own badge, but a background sync can run with no page open,
  // so the outcome is broadcast to whatever clients exist.
  if (r.sent || r.failed || r.uploaded) {
    const clients = await self.clients.matchAll({ includeUncontrolled: true, type: "window" });
    for (const c of clients) c.postMessage({ type: "eform-queue", ...r });
  }
}
