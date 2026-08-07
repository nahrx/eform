/* Shrinks a photo in the browser before it is uploaded.

   A phone camera shot is typically 3–5 MB at 4000×3000. Enumerators work over
   mobile data and often queue several responses offline first, so sending the
   original costs them airtime and fills the device's IndexedDB backlog. The
   answer only ever needs to be legible, not archival.

   Quality alone cannot get a 4000×3000 frame down to 200 KB without it turning
   to mush, so the long edge is capped first and the JPEG quality is walked down
   within each size. The first combination that lands under the budget wins,
   which keeps the largest dimensions the budget can afford.

   Loaded by public.html (form filling) and builder.html (the preview), and
   precached by the service worker so it still works offline. */
(function () {
  if (window.ImageCompress) return;

  const DEFAULT_MAX_KB = 200;
  // Tried in order, widest first — the loop stops at the first size that fits.
  const EDGE_STEPS = [1920, 1600, 1280, 1024, 800, 640, 480];
  const QUALITY_STEPS = [0.82, 0.7, 0.6, 0.5, 0.42, 0.35];

  function isImage(file) {
    return !!file && /^image\//i.test(file.type || "");
  }

  function jpgName(name) {
    const base = String(name || "photo").replace(/\.[^.]+$/, "");
    return (base || "photo") + ".jpg";
  }

  async function loadBitmap(file) {
    if (window.createImageBitmap) {
      // Phone cameras record rotation in EXIF rather than in the pixels; without
      // this the canvas would bake in a sideways photo.
      try {
        return await createImageBitmap(file, { imageOrientation: "from-image" });
      } catch (_e) {}
      try {
        return await createImageBitmap(file);
      } catch (_e) {}
    }
    return await new Promise((resolve, reject) => {
      const url = URL.createObjectURL(file);
      const img = new Image();
      img.onload = () => { URL.revokeObjectURL(url); resolve(img); };
      img.onerror = () => { URL.revokeObjectURL(url); reject(new Error("decode failed")); };
      img.src = url;
    });
  }

  function drawScaled(bmp, edge) {
    const w0 = bmp.width || bmp.naturalWidth;
    const h0 = bmp.height || bmp.naturalHeight;
    const scale = Math.min(1, edge / Math.max(w0, h0));
    const w = Math.max(1, Math.round(w0 * scale));
    const h = Math.max(1, Math.round(h0 * scale));
    const cv = document.createElement("canvas");
    cv.width = w; cv.height = h;
    const ctx = cv.getContext("2d");
    // JPEG carries no alpha channel, so a transparent PNG would come out black.
    ctx.fillStyle = "#fff";
    ctx.fillRect(0, 0, w, h);
    ctx.drawImage(bmp, 0, 0, w, h);
    return cv;
  }

  function toBlob(cv, q) {
    return new Promise((resolve) => {
      try { cv.toBlob((b) => resolve(b), "image/jpeg", q); }
      catch (_e) { resolve(null); }
    });
  }

  /* Resolves to { file, changed, reason, originalSize, reachedTarget }.
     Never rejects and never returns nothing: if anything goes wrong the caller
     gets the untouched file back, because losing the enumerator's photo would
     be far worse than uploading it at full size. */
  async function compress(file, maxKB) {
    const raw = Number(maxKB);
    const limit = Number.isFinite(raw) && raw >= 0 ? raw : DEFAULT_MAX_KB;
    if (limit === 0) return { file, changed: false, reason: "compression is off" };
    const maxBytes = limit * 1024;

    if (!isImage(file)) return { file, changed: false, reason: "not an image" };
    if (file.size <= maxBytes) return { file, changed: false, reason: "already within the limit" };

    let bmp;
    try { bmp = await loadBitmap(file); }
    catch (_e) { return { file, changed: false, reason: "the image could not be read" }; }

    let best = null;
    try {
      for (const edge of EDGE_STEPS) {
        const cv = drawScaled(bmp, edge);
        for (const q of QUALITY_STEPS) {
          const blob = await toBlob(cv, q);
          if (!blob) continue;
          if (!best || blob.size < best.size) best = blob;
          if (blob.size <= maxBytes) break;
        }
        if (best && best.size <= maxBytes) break;
      }
    } catch (_e) {
      return { file, changed: false, reason: "the image could not be processed" };
    } finally {
      if (bmp && bmp.close) bmp.close();
    }

    // Already-optimised images can come out bigger after a re-encode.
    if (!best || best.size >= file.size) {
      return { file, changed: false, reason: "could not be made any smaller" };
    }

    let out;
    try {
      out = new File([best], jpgName(file.name), { type: "image/jpeg", lastModified: Date.now() });
    } catch (_e) {
      return { file, changed: false, reason: "the compressed file could not be built" };
    }
    return {
      file: out,
      changed: true,
      originalSize: file.size,
      // False when even the smallest setting could not reach the budget. The
      // caller still uploads it — see the note in public.html.
      reachedTarget: out.size <= maxBytes,
    };
  }

  /* Returns the budget in KB, or 0 when the admin switched compression off.

     Both properties are absent until an admin touches them, and absent means "on at
     the default" — that is what makes the budget apply to photo fields designed
     before this option existed. Only the off state is written to the schema. */
  function maxKBFor(field) {
    if (!field) return DEFAULT_MAX_KB;
    if (field.autoCompress === false) return 0;
    const raw = field.maxPhotoKB;
    if (raw === "" || raw == null) return DEFAULT_MAX_KB;
    const n = Number(raw);
    return Number.isFinite(n) && n > 0 ? n : DEFAULT_MAX_KB;
  }

  window.ImageCompress = { compress, maxKBFor, DEFAULT_MAX_KB };
})();
