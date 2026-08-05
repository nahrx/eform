package httpapi

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// --- health ---
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// --- auth ---
	mux.HandleFunc("POST /api/auth/login", s.login)
	mux.Handle("GET /api/auth/me", s.authMW(s.me))
	mux.Handle("PATCH /api/auth/me/language", s.authMW(s.updateMyLanguage))

	// --- forms (perlu login) ---
	mux.Handle("GET /api/forms", s.authMW(s.listForms))
	mux.Handle("POST /api/forms", s.authMW(s.createForm))
	mux.Handle("GET /api/forms/{id}", s.authMW(s.getForm))
	mux.Handle("PUT /api/forms/{id}", s.authMW(s.requireRole(s.updateForm, "superadmin", "admin")))
	mux.Handle("DELETE /api/forms/{id}", s.authMW(s.requireRole(s.deleteForm, "superadmin", "admin")))
	mux.Handle("POST /api/forms/{id}/publish", s.authMW(s.requireRole(s.publishForm, "superadmin", "admin")))
	mux.Handle("PUT /api/forms/{id}/column-config", s.authMW(s.requireRole(s.saveFormColumnConfig, "superadmin", "admin")))

	// --- shares ---
	mux.Handle("POST /api/forms/{id}/shares", s.authMW(s.requireRole(s.createShare, "superadmin", "admin")))
	mux.Handle("GET /api/forms/{id}/shares", s.authMW(s.listShares))
	mux.Handle("PATCH /api/shares/{id}", s.authMW(s.requireRole(s.updateShare, "superadmin", "admin")))
	mux.Handle("DELETE /api/shares/{id}", s.authMW(s.requireRole(s.revokeShare, "superadmin", "admin")))
	mux.Handle("POST /api/shares/{id}/reactivate", s.authMW(s.requireRole(s.reactivateShare, "superadmin", "admin")))
	mux.Handle("DELETE /api/shares/{id}/permanent", s.authMW(s.requireRole(s.deleteSharePermanent, "superadmin", "admin")))
	mux.Handle("GET /api/shares/{id}/allowed-emails", s.authMW(s.listAllowedEmails))
	mux.Handle("POST /api/shares/{id}/allowed-emails", s.authMW(s.requireRole(s.addAllowedEmail, "superadmin", "admin")))
	mux.Handle("DELETE /api/share-emails/{id}", s.authMW(s.requireRole(s.removeAllowedEmail, "superadmin", "admin")))

	// --- responses ---
	mux.Handle("GET /api/forms/{id}/responses", s.authMW(s.listResponses))
	mux.Handle("GET /api/forms/{id}/responses/{responseId}", s.authMW(s.getResponseDetail))
	mux.Handle("PATCH /api/forms/{id}/responses/{responseId}", s.authMW(s.requireRole(s.updateResponse, "superadmin", "admin")))
	mux.Handle("DELETE /api/forms/{id}/responses/{responseId}", s.authMW(s.requireRole(s.deleteResponse, "superadmin", "admin")))
	mux.Handle("GET /api/forms/{id}/responses.csv", s.authMW(s.exportResponses))
	mux.Handle("GET /api/forms/{id}/fields/{fieldName}/suggested-values", s.authMW(s.requireRole(s.suggestedFieldValues, "superadmin", "admin")))

	// --- users (khusus superadmin) ---
	mux.Handle("POST /api/users", s.authMW(s.requireRole(s.createUser, "superadmin")))
	mux.Handle("GET /api/users", s.authMW(s.requireRole(s.listUsers, "superadmin")))
	mux.Handle("PATCH /api/users/{id}", s.authMW(s.requireRole(s.patchAdminUser, "superadmin")))
	mux.Handle("DELETE /api/users/{id}", s.authMW(s.requireRole(s.deleteAdminUser, "superadmin")))

	// --- viewer permissions per form (superadmin) ---
	mux.Handle("POST /api/forms/{id}/viewer-permissions", s.authMW(s.requireRole(s.createViewerPermission, "superadmin", "admin")))
	mux.Handle("GET /api/forms/{id}/viewer-permissions", s.authMW(s.requireRole(s.listFormViewerPermissions, "superadmin", "admin")))
	mux.Handle("POST /api/forms/{id}/viewer-permissions/bulk", s.authMW(s.requireRole(s.bulkAssignViewerPermissions, "superadmin", "admin")))
	mux.Handle("DELETE /api/forms/{id}/viewer-permissions/bulk", s.authMW(s.requireRole(s.bulkDeleteViewerPermissions, "superadmin", "admin")))
	mux.Handle("GET /api/forms/{id}/viewer-permissions.csv", s.authMW(s.requireRole(s.exportViewerPermissionsCSV, "superadmin", "admin")))
	mux.Handle("GET /api/viewer-permissions/{permId}", s.authMW(s.requireRole(s.getViewerPermission, "superadmin", "admin")))
	mux.Handle("PUT /api/viewer-permissions/{permId}", s.authMW(s.requireRole(s.updateViewerPermission, "superadmin", "admin")))
	mux.Handle("DELETE /api/viewer-permissions/{permId}", s.authMW(s.requireRole(s.deleteViewerPermission, "superadmin", "admin")))

	// --- editor permissions per form (superadmin) ---
	mux.Handle("POST /api/forms/{id}/editor-permissions", s.authMW(s.requireRole(s.createEditorPermission, "superadmin", "admin")))
	mux.Handle("GET /api/forms/{id}/editor-permissions", s.authMW(s.requireRole(s.listFormEditorPermissions, "superadmin", "admin")))
	mux.Handle("POST /api/forms/{id}/editor-permissions/bulk", s.authMW(s.requireRole(s.bulkAssignEditorPermissions, "superadmin", "admin")))
	mux.Handle("DELETE /api/forms/{id}/editor-permissions/bulk", s.authMW(s.requireRole(s.bulkDeleteEditorPermissions, "superadmin", "admin")))
	mux.Handle("GET /api/forms/{id}/editor-permissions.csv", s.authMW(s.requireRole(s.exportEditorPermissionsCSV, "superadmin", "admin")))
	mux.Handle("GET /api/editor-permissions/{permId}", s.authMW(s.requireRole(s.getEditorPermission, "superadmin", "admin")))
	mux.Handle("PUT /api/editor-permissions/{permId}", s.authMW(s.requireRole(s.updateEditorPermission, "superadmin", "admin")))
	mux.Handle("DELETE /api/editor-permissions/{permId}", s.authMW(s.requireRole(s.deleteEditorPermission, "superadmin", "admin")))
	mux.Handle("POST /api/viewer-permissions/{permId}/convert-to-editor", s.authMW(s.requireRole(s.convertViewerToEditor, "superadmin", "admin")))
	mux.Handle("POST /api/editor-permissions/{permId}/convert-to-viewer", s.authMW(s.requireRole(s.convertEditorToViewer, "superadmin", "admin")))

	// --- allowed respondents per permission (superadmin) ---
	mux.Handle("GET /api/viewer-permissions/{permId}/respondents", s.authMW(s.requireRole(s.listViewerAllowedRespondents, "superadmin", "admin")))
	mux.Handle("POST /api/viewer-permissions/{permId}/respondents", s.authMW(s.requireRole(s.addViewerAllowedRespondent, "superadmin", "admin")))
	mux.Handle("DELETE /api/viewer-respondents/{id}", s.authMW(s.requireRole(s.removeViewerAllowedRespondent, "superadmin", "admin")))
	mux.Handle("GET /api/editor-permissions/{permId}/respondents", s.authMW(s.requireRole(s.listEditorAllowedRespondents, "superadmin", "admin")))
	mux.Handle("POST /api/editor-permissions/{permId}/respondents", s.authMW(s.requireRole(s.addEditorAllowedRespondent, "superadmin", "admin")))
	mux.Handle("DELETE /api/editor-respondents/{id}", s.authMW(s.requireRole(s.removeEditorAllowedRespondent, "superadmin", "admin")))
	mux.Handle("GET /api/forms/{id}/respondents", s.authMW(s.requireRole(s.listFormRespondents, "superadmin", "admin")))

	// --- pengelolaan API key per kuesioner (superadmin / admin pemilik) ---
	mux.Handle("POST /api/forms/{id}/api-keys", s.authMW(s.requireRole(s.createAPIKey, "superadmin", "admin")))
	mux.Handle("GET /api/forms/{id}/api-keys", s.authMW(s.requireRole(s.listAPIKeys, "superadmin", "admin")))
	mux.Handle("PUT /api/api-keys/{keyId}", s.authMW(s.requireRole(s.updateAPIKey, "superadmin", "admin")))
	mux.Handle("DELETE /api/api-keys/{keyId}", s.authMW(s.requireRole(s.deleteAPIKey, "superadmin", "admin")))
	mux.Handle("POST /api/api-keys/{keyId}/rotate", s.authMW(s.requireRole(s.rotateAPIKey, "superadmin", "admin")))
	mux.Handle("GET /api/api-keys/{keyId}/respondents", s.authMW(s.requireRole(s.listAPIKeyRespondents, "superadmin", "admin")))
	mux.Handle("POST /api/api-keys/{keyId}/respondents", s.authMW(s.requireRole(s.addAPIKeyRespondent, "superadmin", "admin")))
	mux.Handle("DELETE /api/api-key-respondents/{id}", s.authMW(s.requireRole(s.removeAPIKeyRespondent, "superadmin", "admin")))
	mux.Handle("GET /api/api-keys/{keyId}/logs", s.authMW(s.requireRole(s.listAPIKeyLogs, "superadmin", "admin")))

	// --- riwayat aksi admin (audit) ---
	mux.Handle("GET /api/activity-logs", s.authMW(s.requireRole(s.listActivityLogs, "superadmin", "admin")))

	// --- API publik untuk sistem eksternal: autentikasi API key, read-only ---
	// Seluruh pembatasan (aktif, kedaluwarsa, IP, kuota) ada di apiKeyMW; cakupan datanya
	// di store.APIKeyScope. Tidak ada endpoint tulis di sini, dan tidak akan ditambahkan.
	mux.Handle("GET /api/v1/me", s.apiKeyMW(s.apiMe))
	mux.Handle("GET /api/v1/forms/{formId}/responses", s.apiKeyMW(s.apiListResponses))
	mux.Handle("GET /api/v1/forms/{formId}/responses.csv", s.apiKeyMW(s.apiExportResponses))
	mux.Handle("GET /api/v1/forms/{formId}/responses/{responseId}", s.apiKeyMW(s.apiGetResponse))

	// --- viewer portal (akses viewer yang sudah login) ---
	// role diperlonggar jadi "viewer","editor" karena satu akun sekarang bisa jadi viewer
	// di satu kuesioner dan editor di kuesioner lain — otorisasi per-form yang sebenarnya
	// tetap dicek di dalam masing-masing handler lewat GetViewerPermission.
	mux.Handle("GET /api/viewer/my-forms", s.authMW(s.requireRole(s.viewerMyForms, "viewer", "editor")))
	mux.Handle("GET /api/viewer/forms/{id}", s.authMW(s.requireRole(s.viewerGetForm, "viewer", "editor")))
	mux.Handle("GET /api/viewer/forms/{id}/permission", s.authMW(s.requireRole(s.viewerMyFormPermission, "viewer", "editor")))
	mux.Handle("GET /api/viewer/forms/{id}/responses", s.authMW(s.requireRole(s.viewerListResponses, "viewer", "editor")))
	mux.Handle("GET /api/viewer/forms/{id}/responses/{responseId}", s.authMW(s.requireRole(s.viewerGetResponse, "viewer", "editor")))
	mux.Handle("GET /api/viewer/forms/{id}/responses.csv", s.authMW(s.requireRole(s.viewerExportResponses, "viewer", "editor")))

	// --- editor portal (akses editor yang sudah login) ---
	mux.Handle("GET /api/editor/my-forms", s.authMW(s.requireRole(s.editorMyForms, "editor", "viewer")))
	mux.Handle("GET /api/editor/forms/{id}", s.authMW(s.requireRole(s.editorGetForm, "editor", "viewer")))
	mux.Handle("GET /api/editor/forms/{id}/responses", s.authMW(s.requireRole(s.editorListResponses, "editor", "viewer")))
	mux.Handle("GET /api/editor/forms/{id}/responses/{responseId}", s.authMW(s.requireRole(s.editorGetResponse, "editor", "viewer")))
	mux.Handle("PATCH /api/editor/forms/{id}/responses/{responseId}", s.authMW(s.requireRole(s.editorUpdateResponse, "editor", "viewer")))
	mux.Handle("GET /api/editor/forms/{id}/responses.csv", s.authMW(s.requireRole(s.editorExportResponses, "editor", "viewer")))

	// --- publik: data referensi (tanpa login) ---
	mux.HandleFunc("GET /api/wilayah", s.wilayahList)
	mux.HandleFunc("GET /api/options-proxy", s.optionsProxy)

	// --- publik: akses kuesioner (tanpa login) ---
	mux.HandleFunc("GET /api/public/forms/{token}", s.publicGetForm)

	// --- publik: PWA (mode offline khusus kuesioner multi-respons) ---
	mux.HandleFunc("GET /api/public/forms/{token}/manifest.webmanifest", s.publicManifest)
	mux.HandleFunc("GET /api/public/forms/{token}/icon.png", s.publicIcon)
	mux.HandleFunc("GET /sw.js", s.page("sw.js"))

	// --- publik: respondent (perlu JWT Google) ---
	mux.Handle("GET /api/public/me", s.respondentMW(s.respondentMe))
	mux.Handle("GET /api/public/forms/{token}/my-response", s.respondentMW(s.myResponse))
	mux.Handle("GET /api/public/forms/{token}/my-responses", s.respondentMW(s.myResponses))
	mux.Handle("GET /api/public/forms/{token}/check-access", s.respondentMW(s.checkAccess))
	// Endpoint yang menulis data dibatasi lajunya: per akun responden dan per IP.
	// Draft disimpan otomatis saat mengisi, jadi kuotanya paling longgar.
	mux.Handle("POST /api/public/forms/{token}/uploads", s.respondentMW(s.limitRespondent(s.publicUpload, 30, 90)))
	mux.Handle("POST /api/public/forms/{token}/responses", s.respondentMW(s.limitRespondent(s.publicSubmit, 20, 60)))
	mux.Handle("POST /api/public/forms/{token}/responses/{responseId}/unsubmit", s.respondentMW(s.limitRespondent(s.unsubmitResponse, 20, 60)))
	mux.Handle("GET /api/public/forms/{token}/draft", s.respondentMW(s.myDraft))
	mux.Handle("POST /api/public/forms/{token}/draft", s.respondentMW(s.limitRespondent(s.saveDraftHandler, 120, 300)))

	// --- OAuth Google (redirect, tidak butuh JWT) ---
	mux.HandleFunc("GET /auth/google", s.googleLogin)
	mux.HandleFunc("GET /auth/google/viewer", s.googleViewerLogin)
	mux.HandleFunc("GET /auth/google/callback", s.googleCallback)

	// --- halaman ---
	mux.HandleFunc("GET /login", s.page("login.html"))
	mux.HandleFunc("GET /admin", s.page("admin.html"))
	mux.HandleFunc("GET /manage", s.page("manage.html")) // halaman pengelolaan satu kuesioner (builder/share/akses)
	mux.HandleFunc("GET /builder", s.page("builder.html"))
	mux.HandleFunc("GET /f/{token}", s.page("public.html"))                          // halaman isi kuesioner publik
	mux.HandleFunc("GET /responses", s.page("responses.html"))                       // halaman daftar jawaban
	mux.HandleFunc("GET /response-view", s.page("response-view.html"))               // halaman lihat detail jawaban
	mux.HandleFunc("GET /viewer-portal", s.page("viewer-portal.html"))               // portal viewer & editor
	mux.HandleFunc("GET /viewer-responses", s.page("viewer-responses.html"))         // jawaban terbatas viewer
	mux.HandleFunc("GET /editor-responses", s.page("editor-responses.html"))         // jawaban editor
	mux.HandleFunc("GET /portal-response-view", s.page("portal-response-view.html")) // detail jawaban viewer/editor
	mux.HandleFunc("GET /auth/google/done", s.page("google-done.html"))              // landing setelah OAuth

	// aset statis tiap halaman (CSS/JS terpisah dari HTML)
	for _, f := range []string{
		"login.css", "login.js",
		"admin.css", "admin.js",
		"manage.css", "manage.js",
		"responses.css", "responses-ui.js", "responses-core.js",
		"builder.css", "builder.js", "builder-bridge.js",
		"searchable-select.js", "geo-map.js",
		"i18n.js", "responsive-tables.js",
	} {
		mux.HandleFunc("GET /"+f, s.page(f))
	}

	// aset pustaka pihak ketiga yang di-vendor (mis. Leaflet) — disajikan dari web/vendor/
	mux.Handle("GET /vendor/", http.StripPrefix("/vendor/", http.FileServer(http.Dir(filepath.Join(s.cfg.WebDir, "vendor")))))

	// uploads: hanya file yang boleh diakses langsung, listing folder ditolak.
	mux.HandleFunc("GET /uploads/", s.uploadFileOnly)
	mux.HandleFunc("HEAD /uploads/", s.uploadFileOnly)

	// halaman depan publik: sajikan folder PublicDir (index.html di "/").
	// Pola "GET /" bersifat catch-all; rute lebih spesifik di atas tetap menang.
	fileServer := http.FileServer(http.Dir(s.cfg.PublicDir))
	mux.Handle("GET /", fileServer)

	return s.wrap(mux)
}

func (s *Server) page(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(s.cfg.WebDir, name))
	}
}

func (s *Server) uploadFileOnly(w http.ResponseWriter, r *http.Request) {
	p := path.Clean("/" + strings.TrimPrefix(r.URL.Path, "/"))
	if p == "/uploads" || strings.HasSuffix(r.URL.Path, "/") {
		http.NotFound(w, r)
		return
	}

	rel := strings.TrimPrefix(p, "/")
	if !strings.HasPrefix(rel, "uploads/") {
		http.NotFound(w, r)
		return
	}

	abs := filepath.Join(s.cfg.PublicDir, filepath.FromSlash(rel))
	publicAbs, err := filepath.Abs(s.cfg.PublicDir)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	absPath, err := filepath.Abs(abs)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if absPath != publicAbs && !strings.HasPrefix(absPath, publicAbs+string(filepath.Separator)) {
		http.NotFound(w, r)
		return
	}

	st, err := os.Stat(absPath)
	if err != nil || st.IsDir() {
		http.NotFound(w, r)
		return
	}

	http.ServeFile(w, r, absPath)
}
