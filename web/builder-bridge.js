/* eForm Builder ↔ backend bridge.
   Injected into builder.html. Uses the builder's globals: serialize() & importJSON(). */
(function () {
  var TOKEN = localStorage.getItem("eform_token");
  if (!TOKEN) { location.replace("/login"); return; }

  var H = { "Authorization": "Bearer " + TOKEN, "Content-Type": "application/json" };
  var currentId = new URLSearchParams(location.search).get("id");

  function api(path, opts) {
    opts = opts || {};
    opts.headers = Object.assign({}, H, opts.headers || {});
    return fetch(path, opts).then(function (r) {
      if (r.status === 401) { localStorage.removeItem("eform_token"); location.replace("/login"); throw new Error("session expired"); }
      var ct = r.headers.get("content-type") || "";
      var p = ct.indexOf("json") >= 0 ? r.json() : Promise.resolve(null);
      return p.then(function (data) {
        if (!r.ok) throw new Error((data && data.error) || ("HTTP " + r.status));
        return data;
      });
    });
  }

  function titleOf(inst) {
    var t = inst && inst.title;
    if (!t) return "";
    if (typeof t === "string") return t;
    for (var k in t) if (t[k]) return t[k];
    return "";
  }

  var _toastTimer = null;
  function toast(msg, err) {
    var el = document.getElementById("ebb-toast");
    if (!el) return;
    el.textContent = (err ? "⚠ " : "✓ ") + msg;
    el.classList.toggle("err", !!err);
    el.classList.add("show");
    clearTimeout(_toastTimer);
    _toastTimer = setTimeout(function () { el.classList.remove("show"); }, 3500);
  }

  var _saving = false;
  function doSave() {
    if (_saving) return;
    if (typeof serialize !== "function") { toast("builder function not found", true); return; }
    var btn = document.getElementById("btnSave");
    var origText = btn ? btn.textContent : "";
    _saving = true;
    if (btn) { btn.disabled = true; btn.textContent = "Saving…"; }
    var inst = serialize();
    var body = JSON.stringify({
      title: titleOf(inst),
      description: (inst.description && (inst.description.id || inst.description)) || "",
      schema: inst,
      version: inst.version || "1.0.0"
    });
    var req = currentId
      ? api("/api/forms/" + currentId, { method: "PUT", body: body })
      : api("/api/forms", { method: "POST", body: body });
    req.then(function (f) {
      currentId = f.id;
      history.replaceState(null, "", "/builder?id=" + f.id);
      // What is on screen now matches the server: clear the dirty marker and drop the
      // local copy, so the next visit is not offered a stale draft to restore.
      if (window.Draft) Draft.markSaved(currentId);
      toast("Saved");
    }).catch(function (e) { toast(e.message, true); })
    .finally(function () {
      _saving = false;
      if (btn) { btn.disabled = false; btn.textContent = origText; }
    });
  }

  function doLogout() {
    localStorage.removeItem("eform_token");
    localStorage.removeItem("eform_user");
    location.replace("/login");
  }

  function on(id, ev, fn) { var e = document.getElementById(id); if (e) e.addEventListener(ev, fn); }

  document.addEventListener("DOMContentLoaded", function () {
    // Render the user's profile details from localStorage
    try {
      var u = JSON.parse(localStorage.getItem("eform_user") || "null");
      if (u) {
        var uname = u.username || "";
        var urole = u.role || "";
        var av = document.getElementById("userAvatar"); if (av) av.textContent = uname.charAt(0).toUpperCase() || "?";
        var un = document.getElementById("userName");   if (un) un.textContent = uname;
        var ur = document.getElementById("userRole");   if (ur) ur.textContent = urole;
        var dn = document.getElementById("uddName");    if (dn) dn.textContent = uname;
        var dr = document.getElementById("uddRole");    if (dr) dr.textContent = urole;
      }
    } catch (_) {}

    // Load the form when the URL carries an id. Draft.init runs only once the loaded
    // instrument is in place — starting it earlier would take the blank instrument as
    // the baseline and report the freshly loaded one as unsaved.
    // Undoes the pre-paint gate in builder.html. A 401 has already redirected inside
    // api(), so the shell stays hidden and the stale builder is never shown; anything
    // else has to reveal it rather than leave a blank page.
    var reveal = function () { document.documentElement.classList.remove("auth-checking"); };
    if (currentId) {
      api("/api/forms/" + currentId).then(function (f) {
        if (f.schema && typeof importJSON === "function") importJSON(f.schema);
      }).catch(function (e) { toast(e.message, true); })
      .finally(function () { if (window.Draft) Draft.init(currentId); reveal(); });
    } else {
      // A new instrument loads nothing, so nothing would have discovered an expired
      // session until Save — after the editor had already built something. One cheap
      // check turns that into a redirect before any work is done.
      api("/api/auth/me").catch(function () {})
        .finally(function () { if (window.Draft) Draft.init(null); reveal(); });
    }

    // Wire up the buttons
    on("btnSave",   "click", doSave);
    on("btnLogout", "click", doLogout);

    // Toggle dropdown profil
    var userBtn  = document.getElementById("userBtn");
    var dropdown = document.getElementById("userDropdown");
    if (userBtn && dropdown) {
      userBtn.addEventListener("click", function (e) {
        e.stopPropagation();
        dropdown.hidden = !dropdown.hidden;
      });
      document.addEventListener("click", function (e) {
        if (!dropdown.hidden && !dropdown.contains(e.target)) dropdown.hidden = true;
      });
    }
  });
})();
