/* eForm Builder ↔ backend bridge.
   Disuntikkan ke builder.html. Memakai fungsi global builder: serialize() & importJSON(). */
(function () {
  var TOKEN = localStorage.getItem("eform_token");
  if (!TOKEN) { location.replace("/login"); return; }

  var H = { "Authorization": "Bearer " + TOKEN, "Content-Type": "application/json" };
  var currentId = new URLSearchParams(location.search).get("id");

  function api(path, opts) {
    opts = opts || {};
    opts.headers = Object.assign({}, H, opts.headers || {});
    return fetch(path, opts).then(function (r) {
      if (r.status === 401) { localStorage.removeItem("eform_token"); location.replace("/login"); throw new Error("sesi habis"); }
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
    if (typeof serialize !== "function") { toast("fungsi builder tak ditemukan", true); return; }
    var btn = document.getElementById("btnSave");
    var origText = btn ? btn.textContent : "";
    _saving = true;
    if (btn) { btn.disabled = true; btn.textContent = "Menyimpan…"; }
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
      toast("Tersimpan");
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
    // Render info profil user dari localStorage
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

    // Muat kuesioner jika ada id di URL
    if (currentId) {
      api("/api/forms/" + currentId).then(function (f) {
        if (f.schema && typeof importJSON === "function") importJSON(f.schema);
      }).catch(function (e) { toast(e.message, true); });
    }

    // Wire tombol
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
      document.addEventListener("click", function () { dropdown.hidden = true; });
      dropdown.addEventListener("click", function (e) { e.stopPropagation(); });
    }
  });
})();
