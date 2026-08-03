/* Penerjemah UI (bukan konten kuesioner) untuk eForm Builder & Dashboard.
   Pendekatan: kamus frasa Indonesia -> Inggris, diterapkan ke DOM yang SUDAH
   dirender lewat MutationObserver — sengaja tidak mengubah kode render
   builder.js/admin.js sama sekali, supaya nol risiko terhadap fungsionalitas
   yang sudah berjalan. Cakupan: label/tombol/menu statis, placeholder, title,
   dan pesan alert/confirm. Konten kuesioner (label pertanyaan dsb, yang sudah
   punya skema {id,en} sendiri) TIDAK disentuh oleh skrip ini. */
(function () {
  if (window.__i18nInit) return;
  window.__i18nInit = true;

  const DICT_EN = {
    // ---- umum / aksi ----
    "Simpan": "Save",
    "Batal": "Cancel",
    "Hapus": "Delete",
    "Edit": "Edit",
    "Tutup": "Close",
    "Tutup ✕": "Close ✕",
    "Buka": "Open",
    "Ya": "Yes",
    "Tidak": "No",
    "Ya, Lanjutkan": "Yes, Continue",
    "Keluar": "Log out",
    "Kembali": "Back",
    "← Kembali ke Admin": "← Back to Admin",
    "Memuat…": "Loading…",
    "Memuat variabel…": "Loading fields…",
    "Tambahkan": "Add",
    "+ Tambah": "+ Add",
    "Semua": "All",
    "Tidak Ada": "None",
    "Nilai": "Value",
    "Catatan": "Note",
    "Catatan (opsional)": "Note (optional)",
    "Catatan (HTML)": "Note (HTML)",
    "Email (opsional)": "Email (optional)",
    "Password (opsional)": "Password (optional)",
    "Password (min. 6 karakter)": "Password (min. 6 characters)",
    "Username": "Username",
    "Email": "Email",
    "Role": "Role",
    "Status": "Status",
    "Dibuat": "Created",
    "Diperbarui": "Updated",
    "Aktif": "Active",
    "Nonaktif": "Inactive",
    "nonaktif": "inactive",
    "Tarik": "Unpublish",
    "Publikasikan": "Publish",
    "Kuesioner sudah ": "The form is already ",
    " — tautan bisa langsung diakses publik.": " — the link can be accessed publicly right away.",
    "⚠️ Kuesioner masih ": "⚠️ The form is still ",
    ". Tautan dibuat, tapi publik baru bisa membuka setelah dipublikasikan.": ". The link has been created, but the public can only open it once it's published.",
    "(opsional)": "(optional)",
    "↻ Muat ulang": "↻ Reload",
    "Tersimpan": "Saved",
    "sesi habis": "session expired",
    "fungsi builder tak ditemukan": "builder function not found",
    "Menyimpan…": "Saving…",

    // ---- topbar builder ----
    "eForm - Builder": "eForm - Builder",
    "Judul instrumen…": "Instrument title…",
    "Judul instrumen": "Instrument title",
    "Valid": "Valid",
    "Impor JSON": "Import JSON",
    "Lihat Kuesioner": "Preview Form",
    "Ekspor JSON": "Export JSON",
    "Builder · Komponen": "Builder · Components",
    "Komponen": "Components",
    "Kuesioner · Halaman": "Form · Pages",
    "+ Tambah halaman": "+ Add page",
    "Halaman": "Page",
    "Daftar halaman": "Page list",
    "Properti": "Properties",
    "JSON & Validasi": "JSON & Validation",
    "Kuesioner": "Form",
    "Per halaman": "Per page",
    "Scroll": "Scroll",
    "Mode navigasi": "Navigation mode",
    "Instrumen Baru": "New Instrument",
    "Bahasa": "Language",
    "PREVIEW": "PREVIEW",

    // ---- builder: properti field/komponen ----
    "Aktif bila": "Enabled when",
    "Tampil bila": "Visible when",
    "Wajib bila": "Required when",
    "Bilangan bulat": "Whole number",
    "Block (card)": "Block (card)",
    "Section (border)": "Section (border)",
    "Field per layar": "Fields per screen",
    "Section per halaman": "Sections per page",
    "Judul block (opsional)": "Block title (optional)",
    "Judul halaman": "Page title",
    "Judul section (opsional)": "Section title (optional)",
    "Keterangan (Markdown)": "Description (Markdown)",
    "Maks karakter": "Max characters",
    "Mata uang": "Currency",
    "Media & Lokasi": "Media & Location",
    "Minimal satu halaman.": "At least one page is required.",
    "Opsi ": "Option ",
    "Opsi 1": "Option 1",
    "Nilai tidak valid": "Invalid value",
    "Pilih banyak": "Multiple choice",
    "Roster — inline": "Roster — inline",
    "Roster — subhalaman": "Roster — subpage",
    "Tanda tangan": "Signature",
    "Tanggal & Waktu": "Date & Time",
    "Teks panjang": "Long text",
    "Teks singkat": "Short text",
    "Titik GPS": "GPS point",
    "Daftar baris di halaman utama; isi tiap baris di halaman terpisah.": "List rows on the main page; fill each row on a separate page.",
    "Input di halaman yang sama.": "Input on the same page.",
    "Seret Block ke halaman ini": "Drag a Block onto this page",
    "Seret Block, Section, atau field — diulang tiap baris": "Drag a Block, Section, or field — repeated per row",
    "Seret Section, Roster, atau field ke dalam block": "Drag a Section, Roster, or field into the block",
    "Seret Section, field, atau Roster ke dalam section": "Drag a Section, field, or Roster into the section",
    "nama field induk (opsional)": "parent field name (optional)",
    "prov (opsional)": "prov (optional)",
    "skipTo (opsional)": "skipTo (optional)",
    "bila (ekspresi)": "when (expression)",
    "lompat ke / __end": "jump to / __end",
    "test (TRUE=lolos)": "test (TRUE=pass)",
    "error — blokir": "error — blocks",
    "warning — boleh lanjut": "warning — can continue",
    "scan / ketik kode": "scan / type code",
    "(keterangan kosong)": "(empty label)",

    // ---- builder: dialog & aksi halaman/komponen ----
    "Hapus halaman": "Delete page",
    "Hapus halaman ini?": "Delete this page?",
    "Hapus ini?": "Delete this?",
    "Tersalin ✓": "Copied ✓",
    "Cek lagi, terlalu besar.": "Please check again, file too large.",
    "Tidak ada lokasi yang cocok untuk elemen ini. Pilih dulu section/block/halaman tujuan, lalu tempel.":
      "No matching location for this element. Select a target section/block/page first, then paste.",
    "Tidak bisa mengakses kamera: ": "Cannot access camera: ",
    "Gagal mengambil lokasi: ": "Failed to get location: ",
    "Geolocation tidak didukung browser ini.": "Geolocation is not supported by this browser.",
    "Izin lokasi ditolak.": "Location permission denied.",
    "Pemindaian otomatis tidak didukung browser ini — isi manual.": "Automatic scanning is not supported by this browser — enter manually.",
    "Pindai Barcode": "Scan Barcode",
    "Arahkan kamera ke barcode/QR.": "Point the camera at the barcode/QR code.",
    "Mencari…": "Searching…",
    "Lengkapi pertanyaan wajib / perbaiki isian yang tidak valid sebelum melanjutkan.":
      "Complete required questions / fix invalid entries before continuing.",
    "Lengkapi pertanyaan wajib / perbaiki isian yang tidak valid sebelum mengirim.":
      "Complete required questions / fix invalid entries before submitting.",

    // ---- builder: mesin ekspresi (pesan validasi) ----
    "diharapkan '": "expected '",
    "ekspresi terpotong": "expression truncated",
    "ekspresi tidak valid: ": "invalid expression: ",
    "fungsi tak dikenal: ": "unknown function: ",
    "karakter tak dikenal: ": "unknown character: ",
    "teks tidak ditutup": "unterminated string",
    "token tak terduga": "unexpected token",
    "ada token sisa di akhir": "unexpected trailing token",

    // ---- dashboard admin ----
    "eForm · Dashboard": "eForm · Dashboard",
    "+ Kuesioner baru": "+ New form",
    "User": "Users",
    "Daftar Kuesioner": "Form List",
    "Judul": "Title",
    "Jawaban": "Responses",
    "Manajemen User": "User Management",
    "Admin": "Admin",
    "Viewer": "Viewer",
    "Editor": "Editor",
    "Buat User Admin": "Create Admin User",
    "+ Buat User": "+ Create User",
    "Tambah Akun Viewer": "Add Viewer Account",
    "Viewer login dengan akun Google. Username otomatis menggunakan email yang didaftarkan.":
      "Viewers log in with a Google account. Username automatically uses the registered email.",
    "Email Google viewer": "Viewer's Google email",
    "Nama / Catatan (opsional)": "Name / Note (optional)",
    "+ Tambah Viewer": "+ Add Viewer",
    "+ Buat Viewer": "+ Create Viewer",
    "email@contoh.com": "email@example.com",
    "Tambah Akun Editor": "Add Editor Account",
    "Editor login dengan akun Google. Username otomatis menggunakan email yang didaftarkan.":
      "Editors log in with a Google account. Username automatically uses the registered email.",
    "Editor login dengan akun Google. Username otomatis menggunakan email.":
      "Editors log in with a Google account. Username automatically uses the email.",
    "Email Google editor": "Editor's Google email",
    "+ Tambah Editor": "+ Add Editor",
    "+ Buat Editor": "+ Create Editor",
    "Tidak bisa menghapus akun sendiri": "Cannot delete your own account",
    "Tidak bisa menghapus akun sendiri.": "Cannot delete your own account.",
    "Tidak dapat dihapus karena sudah ada jawaban": "Cannot be deleted because it already has responses",
    "Email sudah ada di daftar": "Email is already in the list",
    "Belum ada user.": "No users yet.",
    "Password baru ": "New password ",
    "(kosongkan jika tidak diubah)": "(leave blank if unchanged)",
    "min. 6 karakter": "min. 6 characters",
    "Username wajib diisi.": "Username is required.",
    "Password minimal 6 karakter.": "Password must be at least 6 characters.",
    "Membuat…": "Creating…",
    "User berhasil dibuat.": "User created successfully.",
    "Belum ada tautan.": "No links yet.",
    "Belum ada viewer.": "No viewers yet.",
    "Email wajib diisi.": "Email is required.",
    "Menambahkan…": "Adding…",
    "Viewer berhasil ditambahkan.": "Viewer added successfully.",
    "Belum ada editor.": "No editors yet.",
    "Editor berhasil ditambahkan.": "Editor added successfully.",
    "Belum ada editor yang ditambahkan.": "No editors added yet.",
    "Belum ada batasan filter.": "No filter restrictions yet.",
    "Belum ada viewer yang ditambahkan.": "No viewers added yet.",
    "Belum ada responden dipilih.": "No respondents selected yet.",
    "Tidak ada variabel di kuesioner ini.": "There are no fields in this form.",
    "Hapus permanen tautan ini beserta semua konfigurasinya?": "Permanently delete this link and all its configuration?",
    "Email Google wajib diisi": "Google email is required",
    "Pilih editor terlebih dahulu": "Please select an editor first",
    "Pilih viewer terlebih dahulu": "Please select a viewer first",

    // ---- dashboard admin: dialog share ----
    "Bagikan kuesioner": "Share form",
    "Label (opsional, mis. 'Petugas Lapangan')": "Label (optional, e.g. 'Field Officer')",
    "Terima jawaban": "Accepting responses",
    "Izinkan multi-respons (satu akun bisa kirim lebih dari satu jawaban)":
      "Allow multiple responses (one account can submit more than one response)",
    "Publik (siapa saja bisa mengisi)": "Public (anyone can fill it out)",
    "Terbatas (hanya akun terdaftar)": "Restricted (registered accounts only)",
    "Akun yang diizinkan mengisi": "Accounts allowed to respond",
    "Buat tautan share": "Create share link",
    "Bagikan": "Share",
    "Belum ada kuesioner. Klik “+ Kuesioner baru”.": "No forms yet. Click “+ New form”.",
    "Cabut": "Revoke",
    "Konfigurasi": "Configure",

    // ---- dashboard admin: dialog akses viewer/editor ----
    "Akses Viewer": "Viewer Access",
    "Akses Viewer · ": "Viewer Access · ",
    "Pilih viewer yang boleh melihat jawaban kuesioner ini dan konfigurasi batasan aksesnya.":
      "Choose which viewers may see this form's responses and configure their access limits.",
    "Akun Viewer": "Viewer Accounts",
    "Viewer login memakai akun Google — masukkan email Google-nya. Username otomatis menggunakan email.":
      "Viewers log in with a Google account — enter their Google email. Username automatically uses the email.",
    "Tambah Viewer ke Kuesioner Ini": "Add Viewer to This Form",
    "— pilih viewer —": "— select viewer —",
    "Akses responden": "Respondent access",
    "Akses Responden": "Respondent Access",
    "Semua responden": "All respondents",
    "Responden tertentu saja": "Selected respondents only",
    "Filter Variabel yang Dapat Dilihat": "Visible Field Filter",
    "Centang variabel yang boleh dilihat. Jika semua dicentang, semua variabel terlihat.":
      "Check the fields that may be viewed. If all are checked, all fields are visible.",
    "Batasan Filter Variabel": "Field Value Restriction",
    "Batasan Filter Variabel (opsional)": "Field Value Restriction (optional)",
    "Hanya tampilkan data yang nilai variabelnya sesuai nilai yang ditentukan.":
      "Only show data whose field value matches the specified value.",
    "— variabel —": "— field —",
    "Konfigurasi Akses · ": "Access Configuration · ",
    "Responden yang diizinkan": "Allowed respondents",
    "Tambah dari responden yang sudah mengisi:": "Add from respondents who have already responded:",
    "— pilih responden —": "— select respondent —",
    "Variabel yang Dapat Dilihat": "Visible Fields",
    "Centang variabel yang boleh dilihat viewer. Jika semua dicentang, semua variabel terlihat.":
      "Check the fields the viewer may see. If all are checked, all fields are visible.",
    "Akses Editor": "Editor Access",
    "Akses Editor · ": "Editor Access · ",
    "Pilih editor yang boleh mengelola kuesioner ini.": "Choose which editors may manage this form.",
    "Akun Editor": "Editor Accounts",
    "Tambah Editor ke Kuesioner Ini": "Add Editor to This Form",
    "— pilih editor —": "— select editor —",
    "Konfigurasi Editor · ": "Editor Configuration · ",
    "Batasi data yang dapat dilihat dan diedit editor ini hanya pada data yang nilai variabelnya sesuai.":
      "Restrict what this editor can view and edit to only data whose field value matches.",

    // ---- placeholder umum ----
    "Pilih variabel dan masukkan nilai": "Select a field and enter a value",

    // ---- pesan dinamis (preview & impor) ----
    "Preview selesai. Ini hanya tampilan — data tidak disimpan.":
      "Preview finished. This is a view only — no data was saved.",
  };

  // Fragmen untuk pesan alert()/confirm() yang mengandung interpolasi (mis. angka,
  // "block"/"section"), sehingga tidak bisa dicocokkan persis lewat DICT_EN.
  // Dipakai HANYA oleh translateDynamicMessage (bukan walkTranslate), supaya teks
  // konten kuesioner buatan pengguna di DOM tidak ikut ter-substring-replace.
  const DICT_FRAGMENTS = {
    "Hapus ": "Delete ",
    " ini beserta isinya?": " and its contents?",
    " item yang dipilih?": " selected item(s)?",
    "Maksimal ": "Maximum ",
    " baris.": " rows.",
    "Gagal impor: ": "Import failed: ",
    "JSON tidak valid: ": "Invalid JSON: ",
    // pesan panel JSON & Validasi (builder) — selalu berisi interpolasi (nama/angka),
    // jadi tak bisa dicocokkan persis. Aman diterapkan lewat substring karena hanya
    // dipakai di elemen sistem (#paneJson), bukan konten kuesioner buatan pengguna.
    "Nama '": "Name '",
    "' dipakai ": "' used ",
    "tidak ada di referenceData": "does not exist in referenceData",
    "bukan field yang ada": "is not an existing field",
    "target lompatan ": "jump target ",
    " tidak ditemukan": " not found",
    "ekspresi merujuk ": "expression references ",
    " yang tidak ada": " which does not exist",
    "locale utama ": "default locale ",
    " tidak ada di locales": " is not in locales",
    " masalah": " issue(s)",
    "Tidak bisa mengakses kamera: ": "Cannot access camera: ",
    // ---- admin.js: toast & konfirmasi dinamis ----
    "Gagal: ": "Failed: ",
    "Gagal memuat: ": "Failed to load: ",
    "Gagal menyimpan: ": "Failed to save: ",
    'Hapus user "': 'Delete user "',
    '"? Tindakan ini tidak bisa dibatalkan.': '"? This action cannot be undone.',
    'Hapus kuesioner "': 'Delete form "',
    '"? Kuesioner hanya dapat dihapus jika belum ada jawaban.': '"? A form can only be deleted if it has no responses yet.',
    'Hapus viewer "': 'Delete viewer "',
    '"? Semua akses kuesioner viewer ini akan ikut dihapus.': '"? All of this viewer\'s form access will be removed too.',
    'Hapus editor "': 'Delete editor "',
    '"? Semua akses kuesioner editor ini akan ikut dihapus.': '"? All of this editor\'s form access will be removed too.',
    '"? Semua akses form editor ini akan ikut dihapus.': '"? All of this editor\'s access to this form will be removed too.',
    'Cabut akses editor "': 'Revoke editor access "',
    'Cabut akses "': 'Revoke access "',
    '" dari kuesioner ini?': '" from this form?',
    "Jawaban (": "Responses (",
  };
  const FRAGMENT_KEYS = Object.keys(DICT_FRAGMENTS).sort((a, b) => b.length - a.length);

  function fragmentSubstitute(s) {
    let out = s;
    for (const k of FRAGMENT_KEYS) {
      if (out.indexOf(k) >= 0) out = out.split(k).join(DICT_FRAGMENTS[k]);
    }
    return out;
  }

  function translateDynamicMessage(s) {
    if (currentLang() !== "en") return s;
    if (Object.prototype.hasOwnProperty.call(DICT_EN, s)) return DICT_EN[s];
    return fragmentSubstitute(s);
  }

  // Elemen "sistem" yang isinya selalu pesan buatan aplikasi (bukan konten
  // kuesioner buatan pengguna) — aman untuk fallback substring-replace.
  const FRAGMENT_SAFE_SEL = "#paneJson, #ebb-toast, .ebb-toast, #healthTxt, .pv-modal-sub, #adminToast, #confirmMsg, .acts";

  function currentLang() {
    return (window.CURRENT_LANG || localStorage.getItem("eform_lang") || "id").toLowerCase() === "en" ? "en" : "id";
  }

  // Toast/pesan status sering diawali ikon + spasi (mis. "✓ Tersimpan", "⚠ sesi habis").
  // Pisahkan ikonnya dulu supaya sisa teksnya masih bisa cocok persis di kamus.
  const ICON_PREFIX_RE = /^([^\w\sÀ-ÿ]+ )(.*)$/;

  function translateText(s) {
    if (currentLang() !== "en") return s;
    const trimmed = s;
    if (Object.prototype.hasOwnProperty.call(DICT_EN, trimmed)) return DICT_EN[trimmed];
    // varian dengan spasi/koma di ujung terpotong node teks — coba versi trim
    const t2 = trimmed.trim();
    if (t2 !== trimmed && Object.prototype.hasOwnProperty.call(DICT_EN, t2)) {
      return trimmed.replace(t2, DICT_EN[t2]);
    }
    const iconMatch = ICON_PREFIX_RE.exec(trimmed);
    if (iconMatch && Object.prototype.hasOwnProperty.call(DICT_EN, iconMatch[2])) {
      return iconMatch[1] + DICT_EN[iconMatch[2]];
    }
    return s;
  }

  const SKIP_TAGS = new Set(["SCRIPT", "STYLE", "TEXTAREA"]);

  function walkTranslate(node) {
    if (node.nodeType === 3) {
      // text node
      const parentTag = node.parentElement && node.parentElement.tagName;
      if (parentTag && SKIP_TAGS.has(parentTag)) return;
      const orig = node.__i18nOrig != null ? node.__i18nOrig : node.nodeValue;
      node.__i18nOrig = orig;
      let translated = translateText(orig);
      if (translated === orig && node.parentElement && node.parentElement.closest(FRAGMENT_SAFE_SEL)) {
        translated = fragmentSubstitute(orig);
      }
      if (node.nodeValue !== translated) node.nodeValue = translated;
      return;
    }
    if (node.nodeType !== 1) return;
    if (SKIP_TAGS.has(node.tagName)) return;

    // atribut yang mengandung teks tampil ke pengguna
    for (const attr of ["placeholder", "title", "aria-label"]) {
      if (node.hasAttribute && node.hasAttribute(attr)) {
        const key = "__i18nOrig_" + attr;
        const orig = node[key] != null ? node[key] : node.getAttribute(attr);
        node[key] = orig;
        const translated = translateText(orig);
        if (node.getAttribute(attr) !== translated) node.setAttribute(attr, translated);
      }
    }
    // <option> pakai textContent (ditangani lewat childNodes di bawah), tapi <input type=button/submit> pakai value
    if (node.tagName === "INPUT" && (node.type === "button" || node.type === "submit")) {
      const key = "__i18nOrig_value";
      const orig = node[key] != null ? node[key] : node.value;
      node[key] = orig;
      const translated = translateText(orig);
      if (node.value !== translated) node.value = translated;
    }

    for (const child of node.childNodes) walkTranslate(child);
  }

  function translateAll() {
    walkTranslate(document.body);
    document.documentElement.lang = currentLang();
  }

  // Kebalikan dari walkTranslate: kembalikan teks/atribut ke Bahasa Indonesia
  // asli dari tanda __i18nOrig* yang disimpan saat diterjemahkan — dipakai saat
  // beralih balik ke ID, TANPA reload halaman (supaya perubahan yang belum
  // disimpan di builder tidak hilang).
  function walkRevert(node) {
    if (node.nodeType === 3) {
      if (node.__i18nOrig != null) node.nodeValue = node.__i18nOrig;
      return;
    }
    if (node.nodeType !== 1) return;
    if (SKIP_TAGS.has(node.tagName)) return;
    for (const attr of ["placeholder", "title", "aria-label"]) {
      const key = "__i18nOrig_" + attr;
      if (node[key] != null) node.setAttribute(attr, node[key]);
    }
    if (node.tagName === "INPUT" && (node.type === "button" || node.type === "submit")) {
      if (node.__i18nOrig_value != null) node.value = node.__i18nOrig_value;
    }
    for (const child of node.childNodes) walkRevert(child);
  }
  function revertAll() {
    walkRevert(document.body);
    document.documentElement.lang = "id";
  }

  const mo = new MutationObserver((muts) => {
    if (currentLang() !== "en") return;
    for (const m of muts) {
      if (m.type === "childList") {
        m.addedNodes.forEach((n) => walkTranslate(n));
      } else if (m.type === "characterData") {
        walkTranslate(m.target);
      } else if (m.type === "attributes") {
        walkTranslate(m.target);
      }
    }
  });

  function startObserving() {
    mo.observe(document.body, {
      childList: true,
      subtree: true,
      characterData: true,
      attributes: true,
      attributeFilter: ["placeholder", "title", "aria-label", "value"],
    });
  }

  // alert()/confirm() bawaan browser — terjemahkan pesannya sebelum tampil.
  const origAlert = window.alert.bind(window);
  window.alert = (msg) => origAlert(translateDynamicMessage(String(msg)));
  const origConfirm = window.confirm.bind(window);
  window.confirm = (msg) => origConfirm(translateDynamicMessage(String(msg)));

  // Dipanggil oleh switcher bahasa (lihat profile menu) setiap kali user ganti pilihan.
  // Tidak pernah reload halaman — supaya perubahan yang belum disimpan di builder aman.
  window.setUILang = function (lang) {
    lang = lang === "en" ? "en" : "id";
    window.CURRENT_LANG = lang;
    localStorage.setItem("eform_lang", lang);
    if (lang === "en") translateAll();
    else revertAll();
    document.dispatchEvent(new CustomEvent("i18n:changed", { detail: { lang } }));
  };
  window.getUILang = currentLang;

  // Switcher bahasa generik: elemen apa pun ber-atribut [data-lang-btn="id"|"en"]
  // otomatis jadi tombol pilih bahasa (dipakai di dropdown profil builder & admin).
  function markActiveLangBtns() {
    var lang = currentLang();
    document.querySelectorAll("[data-lang-btn]").forEach(function (btn) {
      btn.classList.toggle("active", btn.getAttribute("data-lang-btn") === lang);
    });
  }

  function persistLangToServer(lang) {
    var token = localStorage.getItem("eform_token");
    if (!token) return;
    fetch("/api/auth/me/language", {
      method: "PATCH",
      headers: { "Authorization": "Bearer " + token, "Content-Type": "application/json" },
      body: JSON.stringify({ language: lang }),
    }).catch(function () {});
  }

  function wireLangSwitcher() {
    document.addEventListener("click", function (e) {
      var btn = e.target.closest && e.target.closest("[data-lang-btn]");
      if (!btn) return;
      var lang = btn.getAttribute("data-lang-btn") === "en" ? "en" : "id";
      window.setUILang(lang);
      markActiveLangBtns();
      persistLangToServer(lang);
    });
    document.addEventListener("i18n:changed", markActiveLangBtns);
    markActiveLangBtns();
  }

  // Ambil preferensi bahasa yang tersimpan di server (akun bisa login di perangkat
  // lain) dan terapkan — sumber kebenaran tetap server, localStorage cuma cache.
  function syncLangFromServer() {
    var token = localStorage.getItem("eform_token");
    if (!token) return;
    fetch("/api/auth/me", { headers: { "Authorization": "Bearer " + token } })
      .then(function (r) { return r.ok ? r.json() : null; })
      .then(function (me) {
        if (me && (me.preferredLanguage === "en" || me.preferredLanguage === "id")) {
          window.setUILang(me.preferredLanguage);
          markActiveLangBtns();
        }
      })
      .catch(function () {});
  }

  function boot() {
    startObserving();
    if (currentLang() === "en") translateAll();
    wireLangSwitcher();
    syncLangFromServer();
  }
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", boot);
  else boot();
})();
