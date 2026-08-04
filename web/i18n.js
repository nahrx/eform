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
    "Angka": "Number",
    "Desimal": "Decimal",
    "Terhitung": "Calculated",
    "Tersembunyi": "Hidden",
    "Ya/Tidak": "Yes/No",
    "Tanggal": "Date",
    "Jam": "Time",
    "Tanggal+jam": "Date+time",
    "Foto": "Photo",
    "Berkas": "File",
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
    "tanpa label": "no label",
    "judul block (opsional)": "block title (optional)",
    "judul section (opsional)": "section title (optional)",
    "judul page (opsional)": "page title (optional)",
    "judul roster (opsional)": "roster title (optional)",
    "Pilihan": "Choices",
    "Lainnya": "Other",
    "Block → Section → field. Roster bisa di Block/Section. Section bisa di dalam Roster. Inline tampil di halaman ini; subhalaman muncul di panel Halaman.":
      "Block → Section → field. Roster can be inside a Block/Section. Section can be inside a Roster. Inline shows on this page; subpage appears in the Pages panel.",
    "Template baris roster": "Roster row template",
    "buka →": "open →",

    // ---- builder: panel properti — field/page/block/section/roster ----
    "Nama (dataKey) ": "Name (dataKey) ",
    "Nama (dataKey)": "Name (dataKey)",
    "unik, kolom output": "unique, output column",
    "otomatis & unik, bisa diubah": "automatic & unique, can be changed",
    "unik": "unique",
    "Tampil bila (visibleWhen)": "Visible when (visibleWhen)",
    "Label pertanyaan": "Question label",
    "Petunjuk (hint)": "Hint",
    "Konten HTML": "HTML content",
    "# Petunjuk Pengisian\n\nIsi sesuai **kondisi sebenarnya**. Lihat:\n- poin pertama\n- poin kedua\n\n> Catatan penting.":
      "# Filling Instructions\n\nFill in according to **actual conditions**. See:\n- first point\n- second point\n\n> Important note.",
    "Mendukung: # judul, **tebal**, *miring*, `kode`, list (- / 1.), > kutipan, [teks](url), --- garis.":
      "Supports: # heading, **bold**, *italic*, `code`, list (- / 1.), > quote, [text](url), --- line.",
    "Rumus (calculate)": "Formula (calculate)",
    "Autofill — isi otomatis tapi bisa diedit": "Autofill — filled automatically but can be edited",
    "Min": "Min",
    "Maks": "Max",
    "Satuan": "Unit",
    "Dari": "From",
    "Sampai": "To",
    "Pola (regex)": "Pattern (regex)",
    "Nilai awal (default)": "Default value",
    "— tidak ada —": "— none —",
    "Perilaku": "Behavior",
    "Wajib diisi": "Required",
    "Hanya baca": "Read-only",
    "Izinkan catatan": "Allow remarks",
    "Ditanyakan saat tambah baris": "Prompted when adding a row",
    "Kondisi & alur": "Conditions & flow",
    "Validasi": "Validation",
    "+ Tambah aturan": "+ Add rule",
    "Lompatan (skips)": "Skips",
    "+ Tambah lompatan": "+ Add skip",
    "pesan": "message",
    "Duplikat": "Duplicate",
    "Salin field yang disalin": "Copy the copied field",
    "Salin block yang disalin": "Copy the copied block",
    "Salin section yang disalin": "Copy the copied section",
    "Salin page yang disalin": "Copy the copied page",
    "Salin roster yang disalin": "Copy the copied roster",
    "Salin tautan": "Copy link",
    "Pilihan · sumber": "Choices · source",
    "+ Tambah opsi": "+ Add option",
    "Tabel sumber (variabel)": "Source table (field)",
    "— pilih tabel —": "— select a table —",
    "Belum ada tabel inline. Definisikan dulu di pengaturan instrumen → Reference data, lalu pilih di sini.":
      "No inline tables yet. Define one first in instrument settings → Reference data, then select it here.",
    "Filter berjenjang (field induk)": "Cascading filter (parent field)",
    "URL API ": "API URL ",
    "gunakan {dataKey} untuk substitusi nilai field": "use {dataKey} to substitute the field's value",
    "Trigger dataKey ": "Trigger dataKey ",
    "dataKey yang memicu fetch ulang & harus terisi dulu — pisah koma": "dataKey that triggers a refetch & must be filled first — comma-separated",
    "Value field": "Value field",
    "Label field": "Label field",
    "Parent param ": "Parent param ",
    "Path respons ": "Response path ",
    "opsional": "optional",
    "{dataKey} di URL diganti nilai field tersebut. Trigger dataKey memblokir fetch & mereset pilihan saat belum terisi. path bila array bersarang.":
      "{dataKey} in the URL is replaced with that field's value. Trigger dataKey blocks the fetch & resets the choice until filled. path is for nested arrays.",
    "Jenis roster": "Roster type",
    "Subhalaman": "Subpage",
    "Judul roster (opsional)": "Roster title (optional)",
    "Judul baris roster ": "Roster row title ",
    'mis. "Usaha" — dipakai di tombol & popup tambah baris': 'e.g. "Business" — used in the button & add-row popup',
    "Min baris": "Min rows",
    "Maks baris": "Max rows",
    "Jumlah baris dari field (countFrom) ": "Row count from field (countFrom) ",
    "Wajib ada penambahan baris (minimal 1 baris)": "At least one row must be added (minimum 1 row)",
    "Label tiap baris (itemLabel)": "Per-row label (itemLabel)",
    "Field tampil di daftar baris": "Fields shown in the row list",
    "Tambah field ke roster dulu.": "Add a field to the roster first.",
    "Untuk roster subhalaman: nilai field ini jadi ringkasan tiap baris di halaman utama.":
      "For subpage rosters: this field's value becomes the summary for each row on the main page.",
    "Nilai awal baris (auto isi field pertama)": "Default row value (auto-fills the first field)",
    "Nilai ini otomatis diisi ke field pertama tiap baris yang dibuat dari Min baris. Tidak menimpa nilai yang sudah Anda ubah manual.":
      "This value is auto-filled into the first field of each row created from Min rows. It won't overwrite values you've already edited manually.",
    "Nilai awal baris": "Default row value",
    "Isi Min baris dulu agar editor per baris muncul. Anda juga bisa isi cepat dalam format 1 baris = 1 nilai.":
      "Fill in Min rows first so the per-row editor appears. You can also fill it quickly in a 1-line-per-value format.",
    "Buka editor template roster →": "Open the roster template editor →",

    // ---- builder: panel pengaturan instrumen (offline, reference data, navigasi) ----
    "Tidak ada yang dipilih — pengaturan instrumen.": "Nothing selected — instrument settings.",
    "ID instrumen": "Instrument ID",
    "Versi": "Version",
    "Akronim": "Acronym",
    "Locales": "Locales",
    "Locale utama": "Default locale",
    "Navigasi": "Navigation",
    "Wajib selesai sebelum lanjut": "Must be completed before continuing",
    "Mode Offline (PWA)": "Offline Mode (PWA)",
    "Aktifkan mode offline": "Enable offline mode",
    "Kuesioner bisa di-install seperti aplikasi native di ponsel dan diisi tanpa internet — jawaban tersimpan di perangkat lalu terkirim otomatis saat online kembali. ":
      "The form can be installed like a native app on a phone and filled out without internet — answers are stored on the device and sent automatically once back online. ",
    "Hanya berlaku": "Only applies",
    " untuk tautan share yang diatur sebagai ": " to share links set as ",
    "multi-respons": "multi-response",
    "Sumber lookup / Reference data (JSON)": "Lookup source / Reference data (JSON)",
    "Tiap tabel bisa ": "Each table can be ",
    "inline": "inline",
    " (pakai ": " (using ",
    ") atau ": ") or ",
    "=key di respons; ": "=key in the response; ",
    " atau ": " or ",
    " untuk cascading; ": " for cascading; ",
    " bila array bersarang. Rujuk dari field lewat ": " if it's a nested array. Reference it from a field using ",

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
    "eForm · Kelola Kuesioner": "eForm · Manage Form",
    "← Kembali ke Dashboard": "← Back to Dashboard",
    "Kembali ke Dashboard": "Back to Dashboard",
    "Ringkasan": "Overview",
    "Buka di Halaman Lain": "Open on Another Page",
    "Buka Builder ↗": "Open Builder ↗",
    "Lihat Jawaban ↗": "View Responses ↗",
    "Buka Builder": "Open Builder",
    "Lihat Jawaban": "View Responses",
    "Bagikan Kuesioner": "Share Form",
    "+ Buat Tautan Share": "+ Create Share Link",
    "Buat Tautan Share": "Create Share Link",
    "Tautan share dibuat": "Share link created",
    "Tautan disalin": "Link copied",
    "Gagal menyalin": "Copy failed",
    "Aktifkan Kembali": "Reactivate",
    "Tautan diaktifkan kembali": "Link reactivated",
    "Password": "Password",
    "Hapus Kuesioner": "Delete Form",
    "Memuat jumlah jawaban…": "Loading response count…",

    // ---- menu API (kelola kuesioner) ----
    "+ Buat API Key": "+ Create API Key",
    "Buat API Key": "Create API Key",
    "Konfigurasi API Key": "Configure API Key",
    "API Key Baru": "New API Key",
    "API key dipakai sistem lain untuk menarik jawaban kuesioner ini lewat endpoint read-only. Karena jawaban bisa bersifat rahasia, batasi tiap key seketat mungkin: pilih variabel yang boleh terbaca, batasi barisnya, kunci ke alamat IP tertentu, dan beri masa berlaku.":
      "API keys let other systems pull this form's responses through read-only endpoints. Because responses can be confidential, scope every key as tightly as possible: pick which fields are readable, limit the rows, lock it to specific IP addresses, and set an expiry.",
    "Belum ada API key. Klik \"+ Buat API Key\" untuk membuat yang pertama.": "No API keys yet. Click \"+ Create API Key\" to create the first one.",
    "Cara memakai": "How to use",
    "Kirim key lewat header Authorization. Semua endpoint hanya bisa membaca.": "Send the key in the Authorization header. All endpoints are read-only.",
    "Contoh pemakaian": "Usage example",
    "Cakupan Data": "Data Scope",
    "Variabel yang Dapat Dibaca": "Readable Fields",
    "Keamanan": "Security",
    "Alamat IP": "IP address",
    "Kedaluwarsa": "Expires",
    "Kuota": "Quota",
    "permintaan per menit": "requests per minute",
    "Kosong = tanpa batas": "Empty = no limit",
    "Key aktif": "Key active",
    "Sertakan identitas responden (nama, email, IP)": "Include respondent identity (name, email, IP)",
    "Biarkan mati bila penerima data cukup butuh jawabannya saja.": "Leave off if the recipient only needs the answers themselves.",
    "⚠️ Bila aplikasi berjalan di belakang reverse proxy, semua permintaan terlihat berasal dari IP proxy sehingga pembatasan ini tidak berpengaruh.":
      "⚠️ If the app runs behind a reverse proxy, every request appears to come from the proxy's IP, so this restriction has no effect.",
    "Salin sekarang — key ini tidak akan ditampilkan lagi. Kalau hilang, buat key baru atau rotasi key ini. Perlakukan seperti password: jangan kirim lewat chat atau email biasa.":
      "Copy it now — this key will not be shown again. If you lose it, create a new key or rotate this one. Treat it like a password: don't send it over chat or plain email.",
    "Sudah disalin": "I've copied it",
    "Salin": "Copy",
    "API key disalin": "API key copied",
    "API key diperbarui": "API key updated",
    "API key dihapus": "API key deleted",
    "Rotasi": "Rotate",
    "Log Akses": "Access Log",
    "Log Akses · ": "Access Log · ",
    "Semua panggilan tercatat, termasuk yang ditolak. 100 terbaru ditampilkan.": "Every call is logged, including rejected ones. Showing the latest 100.",
    "Belum ada panggilan API.": "No API calls yet.",
    "Waktu": "Time",
    "Endpoint": "Endpoint",
    "Baris": "Rows",
    "Semua responden": "All respondents",
    "Semua variabel": "All fields",
    "Tanpa identitas": "No identity",
    "Belum pernah dipakai": "Never used",
    "Nonaktif": "Inactive",

    "+ Kuesioner baru": "+ New form",
    "Kuesioner baru": "New form",
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
    "email@contoh.com": "email@example.com",
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
    'Belum ada tautan share. Klik "+ Buat Tautan Share" untuk membuat yang pertama.':
      'No share links yet. Click "+ Create Share Link" to make the first one.',
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
    "Akses Variabel": "Field Access",
    "+ Tambah User": "+ Add User",
    "Tambah User": "Add User",
    "Tambah": "Add",
    "Viewer bisa melihat jawaban kuesioner ini, editor bisa mengelola & mengedit jawabannya. Akun dibuat otomatis saat ditambahkan bila belum terdaftar.":
      "Viewers can see this form's responses, editors can manage & edit them. Accounts are created automatically when added if not yet registered.",
    "Daftar User Kuesioner Ini": "This Form's User List",
    "Akun akan dibuat otomatis bila emailnya belum terdaftar.": "The account will be created automatically if the email isn't registered yet.",
    "Belum ada user yang ditambahkan.": "No users added yet.",
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
    "Ubah jadi Editor": "Switch to Editor",
    "Ubah jadi Viewer": "Switch to Viewer",
    "Ubah akses ini menjadi Editor? Akses viewer yang lama akan dihapus dan digantikan akses editor baru dengan pengaturan yang sama.":
      "Switch this access to Editor? The old viewer access will be removed and replaced with a new editor access carrying the same settings.",
    "Ubah akses ini menjadi Viewer? Akses editor yang lama akan dihapus dan digantikan akses viewer baru dengan pengaturan yang sama.":
      "Switch this access to Viewer? The old editor access will be removed and replaced with a new viewer access carrying the same settings.",
    "Akses diubah menjadi editor": "Access switched to editor",
    "Akses diubah menjadi viewer": "Access switched to viewer",
    "Email wajib diisi": "Email is required",
    "Format email tidak valid": "Invalid email format",
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
    '"? Semua akses form editor ini akan ikut dihapus.': '"? All of this editor\'s access to this form will be removed too.',
    'Cabut akses editor "': 'Revoke editor access "',
    'Cabut akses "': 'Revoke access "',
    '" dari kuesioner ini?': '" from this form?',
    "Jawaban (": "Responses (",
    "Gagal unduh: ": "Download failed: ",
    "Cabut akses ": "Revoke access for ",
    " responden dipilih": " respondents selected",
    " variabel": " field(s)",
    " akses viewer dihapus": " viewer access(es) removed",
    " akses editor dicabut": " editor access(es) revoked",
    "— pilih —": "— select —",
    " baris": " row(s)",
    "Diperbarui ": "Updated ",
    " jawaban": " responses",
    "× dibuka": "× opened",
    // ---- menu API: badge & baris meta kartu key (selalu mengandung angka) ----
    "Belum pernah dipakai": "Never used",
    "Terakhir dipakai ": "Last used ",
    " dari ": " from ",
    "× permintaan": " requests",
    " berlaku sampai ": " valid until ",
    "Semua responden": "All respondents",
    "Semua variabel": "All fields",
    "Tanpa identitas": "No identity",
    " responden": " respondents",
    " filter": " filter(s)",
    // ---- builder: panel properti (teks interpolasi, dibatasi ke #paneProps) ----
    "Baris ": "Row ",
    "Contoh: ": "Example: ",
    "nilai bisa dipanggil di label dengan ": "the value can be referenced in the label with ",
    'otomatis generate baris; kosongkan untuk pakai tombol "': 'auto-generates rows; leave blank to use the "',
    '" dengan popup': '" button with a popup',
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
  // .share-badges sengaja disebut spesifik (bukan #apiKeyList seluruhnya) karena kartu
  // API key juga memuat label buatan pengguna — label tidak boleh ikut ter-substring-replace.
  const FRAGMENT_SAFE_SEL = "#paneJson, #ebb-toast, .ebb-toast, #healthTxt, .pv-modal-sub, #adminToast, #confirmMsg, .acts, #ovUpdated, #ovResponses, .share-meta, .share-badges, #paneProps, #userPermList";

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
        let translated = translateText(orig);
        if (translated === orig && node.closest && node.closest(FRAGMENT_SAFE_SEL)) {
          translated = fragmentSubstitute(orig);
        }
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

  // Jadi true begitu pengguna klik tombol bahasa secara manual — dipakai supaya
  // respons syncLangFromServer() yang terlambat datang (mis. race dengan PATCH
  // yang belum selesai) tidak menimpa balik pilihan yang baru saja dibuat.
  var userChangedLang = false;

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
      userChangedLang = true;
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
        if (userChangedLang) return; // pilihan manual terjadi selagi fetch ini berjalan — jangan ditimpa
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
