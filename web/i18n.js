/* UI translator (not questionnaire content) for the eForm Builder & Dashboard.

   English is the source language: every page renders in English, and this
   script translates the DOM into Indonesian only when the user asks for it.
   The approach is a phrase dictionary applied to the ALREADY rendered DOM via a
   MutationObserver — deliberately leaving the render code in builder.js/admin.js
   untouched, so there is zero risk to behaviour that already works.

   Scope: static labels/buttons/menus, placeholders, titles, and alert/confirm
   messages. Questionnaire content (question labels and the like, which carry
   their own {id,en} schema) is NEVER touched by this script. */
(function () {
  if (window.__i18nInit) return;
  window.__i18nInit = true;

  const DICT_ID = {
    // ---- general / actions ----
    "Save": "Simpan",
    "Cancel": "Batal",
    "⬇ Install App": "⬇ Instal Aplikasi",
    "Delete": "Hapus",
    "Edit": "Edit",
    "Close": "Tutup",
    "Close ✕": "Tutup ✕",
    "Open": "Buka",
    "Yes": "Ya",
    "No": "Tidak",
    "Yes, Continue": "Ya, Lanjutkan",
    "Log out": "Keluar",
    "Back": "Kembali",
    "← Back to Admin": "← Kembali ke Admin",
    "Loading…": "Memuat…",
    "Loading fields…": "Memuat variabel…",
    "+ Add": "+ Tambah",
    "All": "Semua",
    "None": "Tidak Ada",
    "Value": "Nilai",
    "Note": "Catatan",
    "Note (optional)": "Catatan (opsional)",
    "Note (HTML)": "Catatan (HTML)",
    "Email (optional)": "Email (opsional)",
    "Password (optional)": "Password (opsional)",
    "Password (min. 6 characters)": "Password (min. 6 karakter)",
    "Username": "Username",
    "Email": "Email",
    "Role": "Role",
    "Status": "Status",
    "Created": "Dibuat",
    "Updated": "Diperbarui",
    "Active": "Aktif",
    "Inactive": "Nonaktif",
    "inactive": "nonaktif",
    "Unpublish": "Tarik",
    "Publish": "Publikasikan",
    "The form is already ": "Kuesioner sudah ",
    " — the link can be accessed publicly right away.": " — tautan bisa langsung diakses publik.",
    "⚠️ The form is still ": "⚠️ Kuesioner masih ",
    ". The link has been created, but the public can only open it once it's published.": ". Tautan dibuat, tapi publik baru bisa membuka setelah dipublikasikan.",
    "(optional)": "(opsional)",
    "↻ Reload": "↻ Muat ulang",
    "Saved": "Tersimpan",
    "session expired": "sesi habis",
    "builder function not found": "fungsi builder tak ditemukan",
    "Saving…": "Menyimpan…",

    // ---- builder topbar ----
    "eForm - Builder": "eForm - Builder",
    "Instrument title…": "Judul instrumen…",
    "Instrument title": "Judul instrumen",
    "Valid": "Valid",
    "Import JSON": "Impor JSON",
    "Preview Form": "Lihat Kuesioner",
    "Export JSON": "Ekspor JSON",
    "Builder · Components": "Builder · Komponen",
    "Components": "Komponen",
    "Form · Pages": "Kuesioner · Halaman",
    "+ Add page": "+ Tambah halaman",
    "Page": "Halaman",
    "Page list": "Daftar halaman",
    "Properties": "Properti",
    "JSON & Validation": "JSON & Validasi",
    "Form": "Kuesioner",
    "Per page": "Per halaman",
    "Scroll": "Scroll",
    "Navigation mode": "Mode navigasi",
    "New Instrument": "Instrumen Baru",
    "Language": "Bahasa",
    "PREVIEW": "PREVIEW",

    // ---- builder: field/component properties ----
    "Enabled when": "Aktif bila",
    "Visible when": "Tampil bila",
    "Required when": "Wajib bila",
    "Whole number": "Bilangan bulat",
    "Block (card)": "Block (card)",
    "Section (border)": "Section (border)",
    "Fields per screen": "Field per layar",
    "Sections per page": "Section per halaman",
    "Block title (optional)": "Judul block (opsional)",
    "Page title": "Judul halaman",
    "Section title (optional)": "Judul section (opsional)",
    "Description (Markdown)": "Keterangan (Markdown)",
    "Max characters": "Maks karakter",
    "Currency": "Mata uang",
    "Media & Location": "Media & Lokasi",
    "At least one page is required.": "Minimal satu halaman.",
    "Option ": "Opsi ",
    "Option 1": "Opsi 1",
    "Invalid value": "Nilai tidak valid",
    "Multiple choice": "Pilih banyak",
    "Roster — inline": "Roster — inline",
    "Roster — subpage": "Roster — subhalaman",
    "Signature": "Tanda tangan",
    "Date & Time": "Tanggal & Waktu",
    "Long text": "Teks panjang",
    "Short text": "Teks singkat",
    "GPS point": "Titik GPS",
    "Number": "Angka",
    "Decimal": "Desimal",
    "Calculated": "Terhitung",
    "Hidden": "Tersembunyi",
    "Yes/No": "Ya/Tidak",
    "Date": "Tanggal",
    "Date+time": "Tanggal+jam",
    "Photo": "Foto",
    "File": "Berkas",
    "List rows on the main page; fill each row on a separate page.": "Daftar baris di halaman utama; isi tiap baris di halaman terpisah.",
    "Input on the same page.": "Input di halaman yang sama.",
    "Drag a Block onto this page": "Seret Block ke halaman ini",
    "Drag a Block, Section, or field — repeated per row": "Seret Block, Section, atau field — diulang tiap baris",
    "Drag a Section, Roster, or field into the block": "Seret Section, Roster, atau field ke dalam block",
    "Drag a Section, field, or Roster into the section": "Seret Section, field, atau Roster ke dalam section",
    "parent field name (optional)": "nama field induk (opsional)",
    "prov (optional)": "prov (opsional)",
    "skipTo (optional)": "skipTo (opsional)",
    "when (expression)": "bila (ekspresi)",
    "jump to / __end": "lompat ke / __end",
    "test (TRUE=pass)": "test (TRUE=lolos)",
    "error — blocks": "error — blokir",
    "warning — can continue": "warning — boleh lanjut",
    "scan / type code": "scan / ketik kode",
    "(empty label)": "(keterangan kosong)",
    "no label": "tanpa label",
    "block title (optional)": "judul block (opsional)",
    "section title (optional)": "judul section (opsional)",
    "page title (optional)": "judul page (opsional)",
    "roster title (optional)": "judul roster (opsional)",
    "Choices": "Pilihan",
    "Other": "Lainnya",
    "Block → Section → field. Roster bisa di Block/Section. Section bisa di dalam Roster. Inline tampil di halaman ini; subhalaman muncul di panel Halaman.":
      "Block → Section → field. Roster can be inside a Block/Section. Section can be inside a Roster. Inline shows on this page; subpage appears in the Pages panel.",
    "Roster row template": "Template baris roster",
    "open →": "buka →",

    // ---- builder: properties panel — field/page/block/section/roster ----
    "Name (dataKey) ": "Nama (dataKey) ",
    "Name (dataKey)": "Nama (dataKey)",
    "unique, output column": "unik, kolom output",
    "automatic & unique, can be changed": "otomatis & unik, bisa diubah",
    "unique": "unik",
    "Visible when (visibleWhen)": "Tampil bila (visibleWhen)",
    "Question label": "Label pertanyaan",
    "Hint": "Petunjuk (hint)",
    "HTML content": "Konten HTML",
    "# Petunjuk Pengisian\n\nIsi sesuai **kondisi sebenarnya**. Lihat:\n- poin pertama\n- poin kedua\n\n> Catatan penting.":
      "# Filling Instructions\n\nFill in according to **actual conditions**. See:\n- first point\n- second point\n\n> Important note.",
    "Mendukung: # judul, **tebal**, *miring*, `kode`, list (- / 1.), > kutipan, [teks](url), --- garis.":
      "Supports: # heading, **bold**, *italic*, `code`, list (- / 1.), > quote, [text](url), --- line.",
    "Formula (calculate)": "Rumus (calculate)",
    "Autofill — filled automatically but can be edited": "Autofill — isi otomatis tapi bisa diedit",
    "Min": "Min",
    "Max": "Maks",
    "Unit": "Satuan",
    "From": "Dari",
    "To": "Sampai",
    "Pattern (regex)": "Pola (regex)",
    "Default value": "Nilai awal (default)",
    "— none —": "— tidak ada —",
    "Behavior": "Perilaku",
    "Required": "Wajib diisi",
    "Read-only": "Hanya baca",
    "Allow remarks": "Izinkan catatan",
    "Prompted when adding a row": "Ditanyakan saat tambah baris",
    "Conditions & flow": "Kondisi & alur",
    "Validation": "Validasi",
    "+ Add rule": "+ Tambah aturan",
    "Skips": "Lompatan (skips)",
    "+ Add skip": "+ Tambah lompatan",
    "message": "pesan",
    "Duplicate": "Duplikat",
    "Copy the copied field": "Salin field yang disalin",
    "Copy the copied block": "Salin block yang disalin",
    "Copy the copied section": "Salin section yang disalin",
    "Copy the copied page": "Salin page yang disalin",
    "Copy the copied roster": "Salin roster yang disalin",
    "Copy link": "Salin tautan",
    "Choices · source": "Pilihan · sumber",
    "+ Add option": "+ Tambah opsi",
    "Source table (field)": "Tabel sumber (variabel)",
    "— select a table —": "— pilih tabel —",
    "Belum ada tabel inline. Definisikan dulu di pengaturan instrumen → Reference data, lalu pilih di sini.":
      "No inline tables yet. Define one first in instrument settings → Reference data, then select it here.",
    "Cascading filter (parent field)": "Filter berjenjang (field induk)",
    "API URL ": "URL API ",
    "use {dataKey} to substitute the field's value": "gunakan {dataKey} untuk substitusi nilai field",
    "Trigger dataKey ": "Trigger dataKey ",
    "dataKey that triggers a refetch & must be filled first — comma-separated": "dataKey yang memicu fetch ulang & harus terisi dulu — pisah koma",
    "Value field": "Value field",
    "Label field": "Label field",
    "Parent param ": "Parent param ",
    "Response path ": "Path respons ",
    "optional": "opsional",
    "{dataKey} di URL diganti nilai field tersebut. Trigger dataKey memblokir fetch & mereset pilihan saat belum terisi. path bila array bersarang.":
      "{dataKey} in the URL is replaced with that field's value. Trigger dataKey blocks the fetch & resets the choice until filled. path is for nested arrays.",
    "Roster type": "Jenis roster",
    "Subpage": "Subhalaman",
    "Roster title (optional)": "Judul roster (opsional)",
    "Roster row title ": "Judul baris roster ",
    'mis. "Usaha" — dipakai di tombol & popup tambah baris': 'e.g. "Business" — used in the button & add-row popup',
    "Min rows": "Min baris",
    "Max rows": "Maks baris",
    "Row count from field (countFrom) ": "Jumlah baris dari field (countFrom) ",
    "At least one row must be added (minimum 1 row)": "Wajib ada penambahan baris (minimal 1 baris)",
    "Per-row label (itemLabel)": "Label tiap baris (itemLabel)",
    "Fields shown in the row list": "Field tampil di daftar baris",
    "Add a field to the roster first.": "Tambah field ke roster dulu.",
    "Untuk roster subhalaman: nilai field ini jadi ringkasan tiap baris di halaman utama.":
      "For subpage rosters: this field's value becomes the summary for each row on the main page.",
    "Default row value (auto-fills the first field)": "Nilai awal baris (auto isi field pertama)",
    "Nilai ini otomatis diisi ke field pertama tiap baris yang dibuat dari Min baris. Tidak menimpa nilai yang sudah Anda ubah manual.":
      "This value is auto-filled into the first field of each row created from Min rows. It won't overwrite values you've already edited manually.",
    "Default row value": "Nilai awal baris",
    "Isi Min baris dulu agar editor per baris muncul. Anda juga bisa isi cepat dalam format 1 baris = 1 nilai.":
      "Fill in Min rows first so the per-row editor appears. You can also fill it quickly in a 1-line-per-value format.",
    "Open the roster template editor →": "Buka editor template roster →",

    // ---- builder: instrument settings panel (offline, reference data, navigation) ----
    "Nothing selected — instrument settings.": "Tidak ada yang dipilih — pengaturan instrumen.",
    "Instrument ID": "ID instrumen",
    "Version": "Versi",
    "Acronym": "Akronim",
    "Locales": "Locales",
    "Default locale": "Locale utama",
    "Navigation": "Navigasi",
    "Must be completed before continuing": "Wajib selesai sebelum lanjut",
    "Offline Mode (PWA)": "Mode Offline (PWA)",
    "Enable offline mode": "Aktifkan mode offline",
    "Kuesioner bisa di-install seperti aplikasi native di ponsel dan diisi tanpa internet — jawaban tersimpan di perangkat lalu terkirim otomatis saat online kembali. ":
      "The form can be installed like a native app on a phone and filled out without internet — answers are stored on the device and sent automatically once back online. ",
    "Only applies": "Hanya berlaku",
    " to share links set as ": " untuk tautan share yang diatur sebagai ",
    "multi-response": "multi-respons",
    "Lookup source / Reference data (JSON)": "Sumber lookup / Reference data (JSON)",
    "Each table can be ": "Tiap tabel bisa ",
    "inline": "inline",
    " (using ": " (pakai ",
    ") or ": ") atau ",
    "=key in the response; ": "=key di respons; ",
    " or ": " atau ",
    " for cascading; ": " untuk cascading; ",
    " if it's a nested array. Reference it from a field using ": " bila array bersarang. Rujuk dari field lewat ",

    // ---- builder: page/component dialogs & actions ----
    "Delete page": "Hapus halaman",
    "Delete this page?": "Hapus halaman ini?",
    "Delete this?": "Hapus ini?",
    "Copied ✓": "Tersalin ✓",
    "Please check again, file too large.": "Cek lagi, terlalu besar.",
    "Tidak ada lokasi yang cocok untuk elemen ini. Pilih dulu section/block/halaman tujuan, lalu tempel.":
      "No matching location for this element. Select a target section/block/page first, then paste.",
    "Cannot access camera: ": "Tidak bisa mengakses kamera: ",
    "Failed to get location: ": "Gagal mengambil lokasi: ",
    "Geolocation is not supported by this browser.": "Geolocation tidak didukung browser ini.",
    "Location permission denied.": "Izin lokasi ditolak.",
    "Automatic scanning is not supported by this browser — enter manually.": "Pemindaian otomatis tidak didukung browser ini — isi manual.",
    "Scan Barcode": "Pindai Barcode",
    "Point the camera at the barcode/QR code.": "Arahkan kamera ke barcode/QR.",
    "Searching…": "Mencari…",
    "Lengkapi pertanyaan wajib / perbaiki isian yang tidak valid sebelum melanjutkan.":
      "Complete required questions / fix invalid entries before continuing.",
    "Lengkapi pertanyaan wajib / perbaiki isian yang tidak valid sebelum mengirim.":
      "Complete required questions / fix invalid entries before submitting.",

    // ---- builder: expression engine (validation messages) ----
    "expected '": "diharapkan '",
    "expression truncated": "ekspresi terpotong",
    "invalid expression: ": "ekspresi tidak valid: ",
    "unknown function: ": "fungsi tak dikenal: ",
    "unknown character: ": "karakter tak dikenal: ",
    "unterminated string": "teks tidak ditutup",
    "unexpected token": "token tak terduga",
    "unexpected trailing token": "ada token sisa di akhir",

    // ---- admin dashboard ----
    "eForm · Dashboard": "eForm · Dashboard",
    "eForm · Manage Form": "eForm · Kelola Kuesioner",
    "← Back to Dashboard": "← Kembali ke Dashboard",
    "Back to Dashboard": "Kembali ke Dashboard",
    "Overview": "Ringkasan",
    "Open on Another Page": "Buka di Halaman Lain",
    "Open Builder ↗": "Buka Builder ↗",
    "View Responses ↗": "Lihat Jawaban ↗",
    "Open Builder": "Buka Builder",
    "View Responses": "Lihat Jawaban",
    "Share Form": "Bagikan Kuesioner",
    "+ Create Share Link": "+ Buat Tautan Share",
    "Create Share Link": "Buat Tautan Share",
    "Share link created": "Tautan share dibuat",
    "Link copied": "Tautan disalin",
    "Copy failed": "Gagal menyalin",
    "Reactivate": "Aktifkan Kembali",
    "Link reactivated": "Tautan diaktifkan kembali",
    "Password": "Password",
    "Delete Form": "Hapus Kuesioner",
    "Loading response count…": "Memuat jumlah jawaban…",

    // ---- activity history (audit) ----
    "Export": "Ekspor",
    "Change History": "Riwayat Perubahan",
    "Activity Log": "Riwayat Aksi",
    "Jejak aksi yang mengubah data atau mengeluarkan data dari sistem, termasuk unduhan CSV. Tercatat otomatis dan tidak dapat diubah dari aplikasi.":
      "A trail of actions that change data or take data out of the system, including CSV downloads. Recorded automatically and not editable from the app.",
    "No actions recorded yet.": "Belum ada aksi tercatat.",
    "Actor": "Pelaku",
    "Action": "Aksi",
    "Target": "Sasaran",
    "Details": "Keterangan",
    "‹ Previous": "‹ Sebelumnya",
    "Next ›": "Berikutnya ›",

    // ---- API menu (form management) ----
    "+ Create API Key": "+ Buat API Key",
    "Create API Key": "Buat API Key",
    "Configure API Key": "Konfigurasi API Key",
    "New API Key": "API Key Baru",
    "API key dipakai sistem lain untuk menarik jawaban kuesioner ini lewat endpoint read-only. Karena jawaban bisa bersifat rahasia, batasi tiap key seketat mungkin: pilih variabel yang boleh terbaca, batasi barisnya, kunci ke alamat IP tertentu, dan beri masa berlaku.":
      "API keys let other systems pull this form's responses through read-only endpoints. Because responses can be confidential, scope every key as tightly as possible: pick which fields are readable, limit the rows, lock it to specific IP addresses, and set an expiry.",
    "No API keys yet. Click \"+ Create API Key\" to create the first one.": "Belum ada API key. Klik \"+ Buat API Key\" untuk membuat yang pertama.",
    "How to use": "Cara memakai",
    "Send the key in the Authorization header. All endpoints are read-only.": "Kirim key lewat header Authorization. Semua endpoint hanya bisa membaca.",
    "Usage example": "Contoh pemakaian",
    "Data Scope": "Cakupan Data",
    "Readable Fields": "Variabel yang Dapat Dibaca",
    "Security": "Keamanan",
    "IP address": "Alamat IP",
    "Expires": "Kedaluwarsa",
    "Quota": "Kuota",
    "requests per minute": "permintaan per menit",
    "Empty = no limit": "Kosong = tanpa batas",
    "Key active": "Key aktif",
    "Include respondent identity (name, email, IP)": "Sertakan identitas responden (nama, email, IP)",
    "Leave off if the recipient only needs the answers themselves.": "Biarkan mati bila penerima data cukup butuh jawabannya saja.",
    "Bila aplikasi berjalan di belakang reverse proxy, isi TRUSTED_PROXIES di server dengan alamat proxy tersebut — tanpa itu semua permintaan terlihat berasal dari IP proxy dan pembatasan ini tidak berpengaruh.":
      "If the app runs behind a reverse proxy, set TRUSTED_PROXIES on the server to that proxy's address — without it every request appears to come from the proxy's IP and this restriction has no effect.",
    "Salin sekarang — key ini tidak akan ditampilkan lagi. Kalau hilang, buat key baru atau rotasi key ini. Perlakukan seperti password: jangan kirim lewat chat atau email biasa.":
      "Copy it now — this key will not be shown again. If you lose it, create a new key or rotate this one. Treat it like a password: don't send it over chat or plain email.",
    "I've copied it": "Sudah disalin",
    "Copy": "Salin",
    "API key copied": "API key disalin",
    "API key updated": "API key diperbarui",
    "API key deleted": "API key dihapus",
    "Rotate": "Rotasi",
    "Access Log": "Log Akses",
    "Access Log · ": "Log Akses · ",
    "Every call is logged, including rejected ones. Showing the latest 100.": "Semua panggilan tercatat, termasuk yang ditolak. 100 terbaru ditampilkan.",
    "No API calls yet.": "Belum ada panggilan API.",
    "Time": "Waktu",
    "Endpoint": "Endpoint",
    "Rows": "Baris",
    "All respondents": "Semua responden",
    "All fields": "Semua variabel",
    "No identity": "Tanpa identitas",
    "Never used": "Belum pernah dipakai",

    "+ New form": "+ Kuesioner baru",
    "New form": "Kuesioner baru",
    "Users": "User",
    "Form List": "Daftar Kuesioner",
    "Title": "Judul",
    "Responses": "Jawaban",
    "User Management": "Manajemen User",
    "Admin": "Admin",
    "Viewer": "Viewer",
    "Editor": "Editor",
    "Create Admin User": "Buat User Admin",
    "+ Create User": "+ Buat User",
    "email@example.com": "email@contoh.com",
    "Cannot delete your own account": "Tidak bisa menghapus akun sendiri",
    "Cannot delete your own account.": "Tidak bisa menghapus akun sendiri.",
    "Cannot be deleted because it already has responses": "Tidak dapat dihapus karena sudah ada jawaban",
    "Email is already in the list": "Email sudah ada di daftar",
    "No users yet.": "Belum ada user.",
    "New password ": "Password baru ",
    "(leave blank if unchanged)": "(kosongkan jika tidak diubah)",
    "min. 6 characters": "min. 6 karakter",
    "Username is required.": "Username wajib diisi.",
    "Password must be at least 6 characters.": "Password minimal 6 karakter.",
    "Creating…": "Membuat…",
    "User created successfully.": "User berhasil dibuat.",
    "No links yet.": "Belum ada tautan.",
    'Belum ada tautan share. Klik "+ Buat Tautan Share" untuk membuat yang pertama.':
      'No share links yet. Click "+ Create Share Link" to make the first one.',
    "No editors added yet.": "Belum ada editor yang ditambahkan.",
    "No filter restrictions yet.": "Belum ada batasan filter.",
    "No viewers added yet.": "Belum ada viewer yang ditambahkan.",
    "No respondents selected yet.": "Belum ada responden dipilih.",
    "There are no fields in this form.": "Tidak ada variabel di kuesioner ini.",
    "Permanently delete this link and all its configuration?": "Hapus permanen tautan ini beserta semua konfigurasinya?",
    "Google email is required": "Email Google wajib diisi",
    "Please select an editor first": "Pilih editor terlebih dahulu",
    "Please select a viewer first": "Pilih viewer terlebih dahulu",

    // ---- admin dashboard: share dialog ----
    "Share form": "Bagikan kuesioner",
    "Label (optional, e.g. 'Field User')": "Label (opsional, mis. 'User Lapangan')",
    "Accepting responses": "Terima jawaban",
    "Izinkan multi-respons (satu akun bisa kirim lebih dari satu jawaban)":
      "Allow multiple responses (one account can submit more than one response)",
    "Public (anyone can fill it out)": "Publik (siapa saja bisa mengisi)",
    "Restricted (registered accounts only)": "Terbatas (hanya akun terdaftar)",
    "Accounts allowed to respond": "Akun yang diizinkan mengisi",
    "Create share link": "Buat tautan share",
    "Share": "Bagikan",
    "No forms yet. Click “+ New form”.": "Belum ada kuesioner. Klik “+ Kuesioner baru”.",
    "Revoke": "Cabut",
    "Configure": "Konfigurasi",

    // ---- admin dashboard: viewer/editor access dialog ----
    "Viewer Access": "Akses Viewer",
    "Viewer Access · ": "Akses Viewer · ",
    "Pilih viewer yang boleh melihat jawaban kuesioner ini dan konfigurasi batasan aksesnya.":
      "Choose which viewers may see this form's responses and configure their access limits.",
    "Viewer Accounts": "Akun Viewer",
    "Viewer login memakai akun Google — masukkan email Google-nya. Username otomatis menggunakan email.":
      "Viewers log in with a Google account — enter their Google email. Username automatically uses the email.",
    "Add Viewer to This Form": "Tambah Viewer ke Kuesioner Ini",
    "— select viewer —": "— pilih viewer —",
    "Respondent access": "Akses responden",
    "Respondent Access": "Akses Responden",
    "Field Access": "Akses Variabel",
    "+ Add User": "+ Tambah User",
    "Add User": "Tambah User",
    "Add": "Tambah",
    "Viewer bisa melihat jawaban kuesioner ini, editor bisa mengelola & mengedit jawabannya. Akun dibuat otomatis saat ditambahkan bila belum terdaftar.":
      "Viewers can see this form's responses, editors can manage & edit them. Accounts are created automatically when added if not yet registered.",
    "This Form's User List": "Daftar User Kuesioner Ini",
    "The account will be created automatically if the email isn't registered yet.": "Akun akan dibuat otomatis bila emailnya belum terdaftar.",
    "No users added yet.": "Belum ada user yang ditambahkan.",
    "Selected respondents only": "Responden tertentu saja",
    "Visible Field Filter": "Filter Variabel yang Dapat Dilihat",
    "Centang variabel yang boleh dilihat. Jika semua dicentang, semua variabel terlihat.":
      "Check the fields that may be viewed. If all are checked, all fields are visible.",
    "Field Value Restriction": "Batasan Filter Variabel",
    "Field Value Restriction (optional)": "Batasan Filter Variabel (opsional)",
    "Hanya tampilkan data yang nilai variabelnya sesuai nilai yang ditentukan.":
      "Only show data whose field value matches the specified value.",
    "— field —": "— variabel —",
    "Access Configuration · ": "Konfigurasi Akses · ",
    "Allowed respondents": "Responden yang diizinkan",
    "Add from respondents who have already responded:": "Tambah dari responden yang sudah mengisi:",
    "— select respondent —": "— pilih responden —",
    "Visible Fields": "Variabel yang Dapat Dilihat",
    "Centang variabel yang boleh dilihat viewer. Jika semua dicentang, semua variabel terlihat.":
      "Check the fields the viewer may see. If all are checked, all fields are visible.",
    "Editor Access": "Akses Editor",
    "Editor Access · ": "Akses Editor · ",
    "Switch to Editor": "Ubah jadi Editor",
    "Switch to Viewer": "Ubah jadi Viewer",
    "Ubah akses ini menjadi Editor? Akses viewer yang lama akan dihapus dan digantikan akses editor baru dengan pengaturan yang sama.":
      "Switch this access to Editor? The old viewer access will be removed and replaced with a new editor access carrying the same settings.",
    "Ubah akses ini menjadi Viewer? Akses editor yang lama akan dihapus dan digantikan akses viewer baru dengan pengaturan yang sama.":
      "Switch this access to Viewer? The old editor access will be removed and replaced with a new viewer access carrying the same settings.",
    "Access switched to editor": "Akses diubah menjadi editor",
    "Access switched to viewer": "Akses diubah menjadi viewer",
    "Email is required": "Email wajib diisi",
    "Invalid email format": "Format email tidak valid",
    "Choose which editors may manage this form.": "Pilih editor yang boleh mengelola kuesioner ini.",
    "Editor Accounts": "Akun Editor",
    "Add Editor to This Form": "Tambah Editor ke Kuesioner Ini",
    "— select editor —": "— pilih editor —",
    "Editor Configuration · ": "Konfigurasi Editor · ",
    "Batasi data yang dapat dilihat dan diedit editor ini hanya pada data yang nilai variabelnya sesuai.":
      "Restrict what this editor can view and edit to only data whose field value matches.",

    // ---- text that had no translation before the switch to English ----
    "(no label)": "(tanpa label)",
    "+ Add row": "+ Tambah baris",
    "103.10.1.5, 103.10.2.0/24 — empty = any IP": "103.10.1.5, 103.10.2.0/24 — kosong = semua IP",
    "? A form can only be deleted while it has no responses.": "? Kuesioner hanya dapat dihapus jika belum ada jawaban.",
    "A trail of actions that change data or take data out of the system, including CSV downloads. Recorded automatically and cannot be altered from the application.": "Jejak aksi yang mengubah data atau mengeluarkan data dari sistem, termasuk unduhan CSV. Tercatat otomatis dan tidak dapat diubah dari aplikasi.",
    "API keys let other systems pull this form's responses through a read-only endpoint. Because responses can be confidential, scope every key as tightly as possible: choose which fields are readable, limit the rows, lock it to specific IP addresses, and give it an expiry date.": "API key dipakai sistem lain untuk menarik jawaban kuesioner ini lewat endpoint read-only. Karena jawaban bisa bersifat rahasia, batasi tiap key seketat mungkin: pilih variabel yang boleh terbaca, batasi barisnya, kunci ke alamat IP tertentu, dan beri masa berlaku.",
    "Allow multiple responses (one account may submit more than once)": "Izinkan multi-respons (satu akun bisa kirim lebih dari satu jawaban)",
    "Any business?": "Ada usaha?",
    "Block → Section → field. A Roster can sit in a Block/Section. A Section can sit inside a Roster. Inline shows on this page; subpages appear in the Pages panel.": "Block → Section → field. Roster bisa di Block/Section. Section bisa di dalam Roster. Inline tampil di halaman ini; subhalaman muncul di panel Halaman.",
    "Business at the building": "Usaha pada bangunan",
    "Business name": "Nama Usaha",
    "Clear signature": "Hapus tanda tangan",
    "Complete the required questions / fix the invalid entries before continuing.": "Lengkapi pertanyaan wajib / perbaiki isian yang tidak valid sebelum melanjutkan.",
    "Complete the required questions / fix the invalid entries before submitting.": "Lengkapi pertanyaan wajib / perbaiki isian yang tidak valid sebelum mengirim.",
    "Convert this access to Editor? The existing viewer access is removed and replaced by a new editor access with the same settings.": "Ubah akses ini menjadi Editor? Akses viewer yang lama akan dihapus dan digantikan akses editor baru dengan pengaturan yang sama.",
    "Convert this access to Viewer? The existing editor access is removed and replaced by a new viewer access with the same settings.": "Ubah akses ini menjadi Viewer? Akses editor yang lama akan dihapus dan digantikan akses viewer baru dengan pengaturan yang sama.",
    "Copy it now — this key will not be shown again. If you lose it, create a new key or rotate this one. Treat it like a password: never send it over ordinary chat or email.": "Salin sekarang — key ini tidak akan ditampilkan lagi. Kalau hilang, buat key baru atau rotasi key ini. Perlakukan seperti password: jangan kirim lewat chat atau email biasa.",
    "Failed to load form:": "Gagal memuat kuesioner:",
    "For subpage rosters: this field's value becomes each row's summary on the main page.": "Untuk roster subhalaman: nilai field ini jadi ringkasan tiap baris di halaman utama.",
    "If the application runs behind a reverse proxy, set TRUSTED_PROXIES on the server to that proxy's address — without it every request appears to come from the proxy IP and this restriction has no effect.": "Bila aplikasi berjalan di belakang reverse proxy, isi TRUSTED_PROXIES di server dengan alamat proxy tersebut — tanpa itu semua permintaan terlihat berasal dari IP proxy dan pembatasan ini tidak berpengaruh.",
    "Invalid JSON:": "JSON tidak valid:",
    "Last used ": "Terakhir dipakai ",
    "Leave empty = no limit": "Kosongkan = tidak ada batas",
    "No accounts registered yet.": "Belum ada akun terdaftar.",
    "No emails added yet.": "Belum ada email ditambahkan.",
    "No inline tables yet. Define one in instrument settings → Reference data first, then pick it here.": "Belum ada tabel inline. Definisikan dulu di pengaturan instrumen → Reference data, lalu pilih di sini.",
    "No pages to show yet.": "Belum ada halaman untuk ditampilkan.",
    "No rows yet.": "Belum ada baris.",
    "Only show rows whose field values match the values given here.": "Hanya tampilkan data yang nilai variabelnya sesuai nilai yang ditentukan.",
    "Page list": "Daftar Halaman",
    "Preview finished. This is display only — nothing is saved.": "Preview selesai. Ini hanya tampilan — data tidak disimpan.",
    "Remove photo": "Hapus foto",
    "Remove the existing password": "Hapus password yang ada",
    "Cancel selection": "Batalkan pilihan",
    "Optional": "Opsional",
    "Finish": "Selesai",
    "Open image": "Buka gambar",
    "Searching…": "Mencari…",
    "(calculated automatically)": "(dihitung otomatis)",
    "No rows have been added yet": "Belum ada baris yang ditambahkan",
    "📍 Get Location": "📍 Ambil Lokasi",
    "Multi-response": "Multi-respons",
    "Sign-in failed": "Gagal masuk",
    "Rejected items": "Item yang ditolak",
    "Details": "Rincian",
    "Devices holding unsent answers": "Perangkat yang menahan jawaban belum terkirim",
    "These answers are on the users' phones and nowhere else. Contact them to get the data sent. A device that has not reported recently has stopped talking to the server entirely — usually an expired sign-in — so its backlog cannot be assumed cleared.": "Jawaban ini ada di ponsel user dan tidak di tempat lain. Hubungi mereka agar datanya dikirim. Perangkat yang lama tidak melapor sudah berhenti berkomunikasi dengan server — biasanya karena sesi masuk kedaluwarsa — jadi tumpukannya tidak bisa dianggap sudah beres.",
    "Backlog": "Tumpukan",
    "Oldest item": "Item tertua",
    "Last reported": "Terakhir melapor",
    "Nothing stranded.": "Tidak ada yang tertahan.",
    "The server refused these, so they were kept on this device instead of being lost. Retry after fixing the cause — signing in again, for instance. Download a copy before discarding anything.": "Server menolak item berikut, jadi semuanya disimpan di perangkat ini agar tidak hilang. Coba lagi setelah penyebabnya diperbaiki — misalnya masuk ulang. Unduh salinannya sebelum membuang apa pun.",
    "Nothing was rejected.": "Tidak ada yang ditolak.",
    "Download copy": "Unduh salinan",
    "Retry all": "Coba lagi semua",
    "Retry": "Coba lagi",
    "Save photo": "Simpan foto",
    "Discard": "Buang",
    "Point the camera at the barcode or QR code.": "Arahkan kamera ke barcode/QR.",
    "Uploading the file…": "Mengupload file…",
    "File is ready to use.": "File siap digunakan.",
    "File selected. Press Upload to store it on the server.": "File dipilih. Tekan Upload untuk menyimpannya di server.",
    "Attachment saved on this device": "Lampiran tersimpan di perangkat ini",
    "Saved on this device — it will be uploaded once you are back online.": "Tersimpan di perangkat ini — akan diunggah begitu Anda kembali online.",
    "Signature saved on this device — it will be sent once you are back online.": "Tanda tangan tersimpan di perangkat ini — akan dikirim begitu Anda kembali online.",
    "Set Min rows first so the per-row editor appears. You can also fill it quickly using one value per line.": "Isi Min baris dulu agar editor per baris muncul. Anda juga bisa isi cepat dalam format 1 baris = 1 nilai.",
    "Supports: # heading, **bold**, *italic*, ": "Mendukung: # judul, **tebal**, *miring*, ",
    "The form can be installed like a native app on a phone and filled in offline — responses are stored on the device and sent automatically once back online.": "Kuesioner bisa di-install seperti aplikasi native di ponsel dan diisi tanpa internet — jawaban tersimpan di perangkat lalu terkirim otomatis saat online kembali.",
    "There is no valid location for this element. Select a target section/block/page first, then paste.": "Tidak ada lokasi yang cocok untuk elemen ini. Pilih dulu section/block/halaman tujuan, lalu tempel.",
    "This value is prefilled into the first field of every row created by Min rows. It never overwrites a value you changed by hand.": "Nilai ini otomatis diisi ke field pertama tiap baris yang dibuat dari Min baris. Tidak menimpa nilai yang sudah Anda ubah manual.",
    "Tick the fields the viewer may see. If everything is ticked, all fields are visible.": "Centang variabel yang boleh dilihat viewer. Jika semua dicentang, semua variabel terlihat.",
    "any one item to move them all.": "salah satu item untuk memindahkan semua.",
    "if the array is nested.": "bila array bersarang.",
    "name@domain.com": "nama@domain.com",
    "the value can be referenced in labels with": "nilai bisa dipanggil di label dengan",
    "to add/remove options.": "untuk tambah/kurangi pilihan.",
    "— select ": "— pilih ",
    " first —": " dahulu —",
    "— fill in the previous field first —": "— isi kolom sebelumnya dahulu —",
    "⏳ Loading options from the API…": "⏳ Memuat pilihan dari API…",
    "✓ Clean — no issues found.": "✓ Bersih — tidak ada masalah.",
    "📎 Choose File": "📎 Pilih File",
    "📷 Take / Choose Photo": "📷 Ambil / Pilih Foto",

    "Viewers can see this form's responses; editors can manage and edit them. An account is created automatically when someone is added who is not registered yet.": "Viewer bisa melihat jawaban kuesioner ini, editor bisa mengelola & mengedit jawabannya. Akun dibuat otomatis saat ditambahkan bila belum terdaftar.",
    "Limit what this editor can see and edit to rows whose field values match.": "Batasi data yang dapat dilihat dan diedit editor ini hanya pada data yang nilai variabelnya sesuai.",
    "responses": "jawaban",
    "Allowed accounts": "Akun yang diizinkan",
    "Settings": "Atur",
    "Download .json": "Unduh .json",
    // ---- pages that had no translation before the switch to English ----
    "or": "atau",
    "Login successful…": "Login berhasil…",
    "Login successful, redirecting…": "Login berhasil, mengalihkan…",
    "Failed": "Gagal",
    "A platform for building and distributing digital questionnaires — design instruments with drag-and-drop, publish them, and share them with respondents via a link.": "Platform penyusun dan distribusi kuesioner digital — rancang instrumen secara drag-and-drop, publikasikan, dan bagikan ke responden lewat tautan.",
    "Build": "Susun",
    "Build a form with pages, blocks, sections, rosters, and many field types — including manual, inline, or API option sources.": "Bangun kuesioner dengan halaman, blok, seksi, roster, dan beragam tipe field — termasuk sumber pilihan manual, inline, atau API.",
    "Create share links with optional passwords and expiry dates. Respondents fill them in without needing an account.": "Buat tautan share dengan opsi password dan masa berlaku. Responden mengisi tanpa perlu akun.",
    "Collect": "Kumpulkan",
    "Responses are saved automatically and can be downloaded as CSV from the admin dashboard.": "Jawaban tersimpan otomatis dan bisa diunduh sebagai CSV dari dashboard admin.",
    "Have a form link?": "Punya tautan kuesioner?",
    "Paste the token code or share URL to open it straight away.": "Tempel kode token atau URL share untuk langsung membukanya.",
    "Open Form": "Buka Kuesioner",
    "Sign in": "Masuk",
    "Form Access Portal": "Portal Akses Form",
    "Forms You Can Access": "Kuesioner yang Dapat Anda Akses",
    "Sign in with the Google account that has been registered as a Viewer or Editor.": "Masuk menggunakan akun Google yang telah didaftarkan sebagai Viewer atau Editor.",
    "All Respondents": "Semua Responden",
    "Selected Respondents": "Responden Terpilih",
    "All fields visible": "Semua variabel terlihat",
    "Columns ▾": "Kolom ▾",
    "Choose extra columns": "Pilih kolom tambahan",
    "✕ Reset all": "✕ Reset semua",
    "Filter responses": "Filter jawaban",
    "Filter": "Filter",
    "Close filter": "Tutup filter",
    "Search": "Cari",
    "Search name / email…": "Cari nama / email…",
    "Submitted": "Terkirim",
    "Draft": "Draf",
    "Done": "Selesai",
    "Respondent": "Responden",
    "View": "Lihat",
    "← Previous": "← Sebelumnya",
    "Next →": "Selanjutnya →",
    "No fields available.": "Tidak ada field tersedia.",
    "No responses match the filter.": "Tidak ada jawaban yang cocok dengan filter.",
    "No responses yet.": "Belum ada jawaban.",
    "You do not have access to this form.": "Anda tidak memiliki akses ke kuesioner ini.",
    "all respondents": "semua responden",
    "selected respondents": "responden terpilih",
    "No forms are accessible yet.": "Belum ada kuesioner yang dapat diakses.",
    "Validation Errors": "Daftar Error Validasi",
    "Does not meet the validation rule": "Tidak memenuhi aturan validasi",
    "No rows have been added": "Belum ada baris yang ditambahkan",
    "Rule violations": "Tidak sesuai aturan",
    "Needs checking": "Perlu dicek",
    "Not filled in": "Belum diisi",
    "No validation errors": "Tidak ada error validasi",
    "Every answer shown satisfies the form's rules.": "Semua jawaban yang tampil memenuhi aturan kuesioner.",
    "(none)": "(tidak ada)",
    "(empty)": "(kosong)",
    "No matches": "Tidak ditemukan",
    "Nothing selected yet": "Belum ada dipilih",
    "Search…": "Cari…",

    "Response Detail": "Detail Jawaban",
    "Pages": "Halaman",
    "Read Only": "Hanya Baca",
    "✕ Close": "✕ Tutup",
    "Close page list": "Tutup daftar halaman",
    "Session expired. Please log in again.": "Sesi habis. Silakan login ulang.",
    "Incomplete parameters.": "Parameter tidak lengkap.",
    "👁 View Mode": "👁 Mode Lihat",
    "✏️ Edit Mode": "✏️ Mode Edit",
    "Response saved successfully": "Jawaban berhasil disimpan",
    "— Select —": "— Pilih —",
    "Failed to load options.": "Gagal memuat opsi.",
    "Loading options…": "Memuat opsi…",
    "Delete Response": "Hapus Jawaban",
    "Permanently delete this response? This action cannot be undone.": "Hapus jawaban ini secara permanen? Tindakan ini tidak dapat dibatalkan.",
    "Response Deleted": "Jawaban Dihapus",
    "The response was deleted successfully.": "Jawaban berhasil dihapus.",
    "You can close this tab.": "Anda dapat menutup tab ini.",
    "Delete Failed": "Gagal Menghapus",
    "Click to enlarge": "Klik untuk perbesar",
    "(cannot be edited here)": "(tidak dapat diubah di sini)",
    "(coordinates, cannot be edited here)": "(koordinat, tidak dapat diubah di sini)",
    "No data": "Tidak ada data",
    "Roster data cannot be edited from this view.": "Data roster tidak dapat diubah melalui tampilan ini.",
    "Sign in with Google": "Masuk dengan Google",
    "New password": "Password baru",
    "Error list": "Daftar error",
    "Manage Form": "Kelola Kuesioner",
    "Deleting…": "Menghapus…",
    "Signature saved.": "Tanda tangan tersimpan.",
    // ---- common placeholders ----
    "Select a field and enter a value": "Pilih variabel dan masukkan nilai",

    // ---- dynamic messages (preview & import) ----
    "Preview selesai. Ini hanya tampilan — data tidak disimpan.":
      "Preview finished. This is a view only — no data was saved.",
  };

  // Fragments for alert()/confirm() messages that contain interpolation (numbers,
  // "block"/"section", ...), which therefore cannot be matched exactly via DICT_ID.
  // Used ONLY by translateDynamicMessage (never walkTranslate), so that
  // user-authored questionnaire text in the DOM is never substring-replaced.
  const DICT_FRAGMENTS = {
    "Delete ": "Hapus ",
    " and its contents?": " ini beserta isinya?",
    " selected item(s)?": " item yang dipilih?",
    "Maximum ": "Maksimal ",
    " rows.": " baris.",
    "Import failed: ": "Gagal impor: ",
    "Invalid JSON: ": "JSON tidak valid: ",
    "Validation — ": "Validasi — ",
    // JSON & Validation panel messages (builder) — always contain interpolation
    // (names/numbers), so they cannot be matched exactly. Substring replacement is
    // safe here because they only appear in system elements (#paneJson), never in
    "Name '": "Nama '",
    "' used ": "' dipakai ",
    "does not exist in referenceData": "tidak ada di referenceData",
    "is not an existing field": "bukan field yang ada",
    "jump target ": "target lompatan ",
    " not found": " tidak ditemukan",
    "expression references ": "ekspresi merujuk ",
    " which does not exist": " yang tidak ada",
    "default locale ": "locale utama ",
    " is not in locales": " tidak ada di locales",
    " issue(s)": " masalah",
    "Cannot access camera: ": "Tidak bisa mengakses kamera: ",
    // ---- admin.js: dynamic toasts & confirmations ----
    "Failed: ": "Gagal: ",
    "Failed to load: ": "Gagal memuat: ",
    "Failed to save: ": "Gagal menyimpan: ",
    'Hapus user "': 'Delete user "',
    '"? Tindakan ini tidak bisa dibatalkan.': '"? This action cannot be undone.',
    'Hapus kuesioner "': 'Delete form "',
    '"? Kuesioner hanya dapat dihapus jika belum ada jawaban.': '"? A form can only be deleted if it has no responses yet.',
    '"? Semua akses form editor ini akan ikut dihapus.': '"? All of this editor\'s access to this form will be removed too.',
    'Cabut akses editor "': 'Revoke editor access "',
    'Cabut akses "': 'Revoke access "',
    '" dari kuesioner ini?': '" from this form?',
    "Responses (": "Jawaban (",
    "Download failed: ": "Gagal unduh: ",
    "Revoke access for ": "Cabut akses ",
    " respondents selected": " responden dipilih",
    " field(s)": " variabel",
    " viewer access(es) removed": " akses viewer dihapus",
    " editor access(es) revoked": " akses editor dicabut",
    "— select —": "— pilih —",
    " row(s)": " baris",
    "Updated ": "Diperbarui ",
    " responses": " jawaban",
    "× opened": "× dibuka",
    // ---- API menu: key card badges & meta rows (always contain numbers) ----
    "Never used": "Belum pernah dipakai",
    "Last used ": "Terakhir dipakai ",
    " from ": " dari ",
    " requests": "× permintaan",
    " valid until ": " berlaku sampai ",
    "All respondents": "Semua responden",
    "All fields": "Semua variabel",
    "No identity": "Tanpa identitas",
    " respondents": " responden",
    " filter(s)": " filter",
    // ---- builder: properties panel (interpolated text, limited to #paneProps) ----
    "Row ": "Baris ",
    "Example: ": "Contoh: ",
    "the value can be referenced in the label with ": "nilai bisa dipanggil di label dengan ",
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
    if (currentLang() !== "id") return s;
    if (Object.prototype.hasOwnProperty.call(DICT_ID, s)) return DICT_ID[s];
    return fragmentSubstitute(s);
  }

  // "System" elements whose contents are always application-generated messages
  // (never user-authored questionnaire content) — safe for the substring-replace
  // fallback. .share-badges is named specifically rather than all of #apiKeyList
  // because API key cards also carry user-authored labels, and those must not be
  // substring-replaced.
  const FRAGMENT_SAFE_SEL = "#paneJson, #ebb-toast, .ebb-toast, #healthTxt, .pv-modal-sub, #adminToast, #confirmMsg, .acts, #ovUpdated, #ovResponses, .share-meta, .share-badges, #paneProps, #userPermList";

  function currentLang() {
    return (window.CURRENT_LANG || localStorage.getItem("eform_lang") || "en").toLowerCase() === "id" ? "id" : "en";
  }

  // Toasts and status messages often start with an icon + space (e.g. "✓ Saved",
  // "⚠ session expired"). Strip the icon first so the remaining text can still
  // match the dictionary exactly.
  const ICON_PREFIX_RE = /^([^\w\sÀ-ÿ]+ )(.*)$/;

  // "Normalised" copy of the dictionary: runs of whitespace (including newlines
  // and HTML indentation) collapse to a single space. Without this, a sentence
  // wrapped across several lines in the markup would never match its DICT_ID key
  // — a bug that kept recurring whenever new text was added.
  const norm = t => t.replace(/\s+/g, " ").trim();
  const DICT_NORM = Object.create(null);
  for (const k of Object.keys(DICT_ID)) {
    const nk = norm(k);
    if (!(nk in DICT_NORM)) DICT_NORM[nk] = DICT_ID[k];
  }

  // English text that has no Indonesian translation yet. Inspect it from the
  // console via window.__i18nMissing() to find dictionary gaps without guessing.
  const missing = new Set();
  window.__i18nMissing = () => [...missing].sort();

  function lookup(t) {
    if (Object.prototype.hasOwnProperty.call(DICT_ID, t)) return DICT_ID[t];
    const nt = norm(t);
    if (Object.prototype.hasOwnProperty.call(DICT_NORM, nt)) return DICT_NORM[nt];
    return null;
  }

  function translateText(s) {
    if (currentLang() !== "id") return s;
    const hit = lookup(s);
    if (hit != null) return hit;

    // text nodes can be split with stray leading/trailing whitespace — try trimmed
    const t2 = s.trim();
    if (t2 !== s) {
      const hit2 = lookup(t2);
      if (hit2 != null) return s.replace(t2, hit2);
    }
    const iconMatch = ICON_PREFIX_RE.exec(s);
    if (iconMatch) {
      const hit3 = lookup(iconMatch[2]);
      if (hit3 != null) return iconMatch[1] + hit3;
    }
    // Only record text that actually contains letters, so numbers and punctuation
    // do not flood the list.
    if (t2.length > 1 && /[a-zA-Z]/.test(t2)) missing.add(t2);
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

    // attributes that carry user-visible text
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
    // <option> uses textContent (handled via childNodes below), but <input type=button/submit> uses value
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

  // The inverse of walkTranslate: restore text/attributes to the original English
  // recorded in the __i18nOrig* markers during translation — used when switching
  // back to English WITHOUT reloading the page, so unsaved builder edits survive.
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
    document.documentElement.lang = "en";
  }

  const mo = new MutationObserver((muts) => {
    if (currentLang() !== "id") return;
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

  // Native browser alert()/confirm() — translate the message before it is shown.
  const origAlert = window.alert.bind(window);
  window.alert = (msg) => origAlert(translateDynamicMessage(String(msg)));
  const origConfirm = window.confirm.bind(window);
  window.confirm = (msg) => origConfirm(translateDynamicMessage(String(msg)));

  // Called by the language switcher (see the profile menu) whenever the user
  // changes their choice. Never reloads the page, so unsaved builder edits are safe.
  window.setUILang = function (lang) {
    lang = lang === "id" ? "id" : "en";
    window.CURRENT_LANG = lang;
    localStorage.setItem("eform_lang", lang);
    if (lang === "id") translateAll();
    else revertAll();
    document.dispatchEvent(new CustomEvent("i18n:changed", { detail: { lang } }));
  };
  window.getUILang = currentLang;

  // Generic language switcher: any element carrying [data-lang-btn="id"|"en"]
  // automatically becomes a language button (used in the builder & admin profile menus).
  function markActiveLangBtns() {
    var lang = currentLang();
    document.querySelectorAll("[data-lang-btn]").forEach(function (btn) {
      btn.classList.toggle("active", btn.getAttribute("data-lang-btn") === lang);
    });
  }

  // Becomes true as soon as the user clicks a language button — used so that a
  // late syncLangFromServer() response (e.g. racing an in-flight PATCH) cannot
  // overwrite the choice they just made.
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
      var lang = btn.getAttribute("data-lang-btn") === "id" ? "id" : "en";
      userChangedLang = true;
      window.setUILang(lang);
      markActiveLangBtns();
      persistLangToServer(lang);
    });
    document.addEventListener("i18n:changed", markActiveLangBtns);
    markActiveLangBtns();
  }

  // Fetch the language preference stored on the server (the account may be logged
  // in on another device) and apply it — the server stays the source of truth,
  // localStorage is only a cache.
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
    if (currentLang() === "id") translateAll();
    wireLangSwitcher();
    syncLangFromServer();
  }
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", boot);
  else boot();
})();
