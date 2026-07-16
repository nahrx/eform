package httpapi

import (
	"net/http"
	"path/filepath"
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
	mux.Handle("DELETE /api/shares/{id}/permanent", s.authMW(s.requireRole(s.deleteSharePermanent, "superadmin", "admin")))
	mux.Handle("GET /api/shares/{id}/allowed-emails", s.authMW(s.listAllowedEmails))
	mux.Handle("POST /api/shares/{id}/allowed-emails", s.authMW(s.requireRole(s.addAllowedEmail, "superadmin", "admin")))
	mux.Handle("DELETE /api/share-emails/{id}", s.authMW(s.requireRole(s.removeAllowedEmail, "superadmin", "admin")))

	// --- responses ---
	mux.Handle("GET /api/forms/{id}/responses", s.authMW(s.listResponses))
	mux.Handle("GET /api/forms/{id}/responses/{responseId}", s.authMW(s.getResponseDetail))
	mux.Handle("DELETE /api/forms/{id}/responses/{responseId}", s.authMW(s.requireRole(s.deleteResponse, "superadmin", "admin")))
	mux.Handle("GET /api/forms/{id}/responses.csv", s.authMW(s.exportResponses))

	// --- users (khusus superadmin) ---
	mux.Handle("POST /api/users", s.authMW(s.requireRole(s.createUser, "superadmin")))
	mux.Handle("GET /api/users", s.authMW(s.requireRole(s.listUsers, "superadmin")))
	mux.Handle("PATCH /api/users/{id}", s.authMW(s.requireRole(s.patchAdminUser, "superadmin")))
	mux.Handle("DELETE /api/users/{id}", s.authMW(s.requireRole(s.deleteAdminUser, "superadmin")))

	// --- viewers (superadmin kelola akun viewer) ---
	mux.Handle("POST /api/viewers", s.authMW(s.requireRole(s.createViewer, "superadmin", "admin")))
	mux.Handle("GET /api/viewers", s.authMW(s.requireRole(s.listViewers, "superadmin", "admin")))
	mux.Handle("PATCH /api/viewers/{id}", s.authMW(s.requireRole(s.patchNoteWithRole("viewer"), "superadmin", "admin")))
	mux.Handle("DELETE /api/viewers/{id}", s.authMW(s.requireRole(s.deleteViewer, "superadmin", "admin")))

	// --- editors (superadmin kelola akun editor) ---
	mux.Handle("POST /api/editors", s.authMW(s.requireRole(s.createEditor, "superadmin", "admin")))
	mux.Handle("GET /api/editors", s.authMW(s.requireRole(s.listEditors, "superadmin", "admin")))
	mux.Handle("PATCH /api/editors/{id}", s.authMW(s.requireRole(s.patchNoteWithRole("editor"), "superadmin", "admin")))
	mux.Handle("DELETE /api/editors/{id}", s.authMW(s.requireRole(s.deleteEditor, "superadmin", "admin")))

	// --- viewer permissions per form (superadmin) ---
	mux.Handle("POST /api/forms/{id}/viewer-permissions", s.authMW(s.requireRole(s.createViewerPermission, "superadmin", "admin")))
	mux.Handle("GET /api/forms/{id}/viewer-permissions", s.authMW(s.requireRole(s.listFormViewerPermissions, "superadmin", "admin")))
	mux.Handle("GET /api/viewer-permissions/{permId}", s.authMW(s.requireRole(s.getViewerPermission, "superadmin", "admin")))
	mux.Handle("PUT /api/viewer-permissions/{permId}", s.authMW(s.requireRole(s.updateViewerPermission, "superadmin", "admin")))
	mux.Handle("DELETE /api/viewer-permissions/{permId}", s.authMW(s.requireRole(s.deleteViewerPermission, "superadmin", "admin")))

	// --- editor permissions per form (superadmin) ---
	mux.Handle("POST /api/forms/{id}/editor-permissions", s.authMW(s.requireRole(s.createEditorPermission, "superadmin", "admin")))
	mux.Handle("GET /api/forms/{id}/editor-permissions", s.authMW(s.requireRole(s.listFormEditorPermissions, "superadmin", "admin")))
	mux.Handle("GET /api/editor-permissions/{permId}", s.authMW(s.requireRole(s.getEditorPermission, "superadmin", "admin")))
	mux.Handle("PUT /api/editor-permissions/{permId}", s.authMW(s.requireRole(s.updateEditorPermission, "superadmin", "admin")))
	mux.Handle("DELETE /api/editor-permissions/{permId}", s.authMW(s.requireRole(s.deleteEditorPermission, "superadmin", "admin")))

	// --- allowed respondents per permission (superadmin) ---
	mux.Handle("GET /api/viewer-permissions/{permId}/respondents", s.authMW(s.requireRole(s.listViewerAllowedRespondents, "superadmin", "admin")))
	mux.Handle("POST /api/viewer-permissions/{permId}/respondents", s.authMW(s.requireRole(s.addViewerAllowedRespondent, "superadmin", "admin")))
	mux.Handle("DELETE /api/viewer-respondents/{id}", s.authMW(s.requireRole(s.removeViewerAllowedRespondent, "superadmin", "admin")))
	mux.Handle("GET /api/forms/{id}/respondents", s.authMW(s.requireRole(s.listFormRespondents, "superadmin", "admin")))

	// --- viewer portal (akses viewer yang sudah login) ---
	mux.Handle("GET /api/viewer/my-forms", s.authMW(s.requireRole(s.viewerMyForms, "viewer")))
	mux.Handle("GET /api/viewer/forms/{id}", s.authMW(s.requireRole(s.viewerGetForm, "viewer")))
	mux.Handle("GET /api/viewer/forms/{id}/permission", s.authMW(s.requireRole(s.viewerMyFormPermission, "viewer")))
	mux.Handle("GET /api/viewer/forms/{id}/responses", s.authMW(s.requireRole(s.viewerListResponses, "viewer")))
	mux.Handle("GET /api/viewer/forms/{id}/responses/{responseId}", s.authMW(s.requireRole(s.viewerGetResponse, "viewer")))

	// --- editor portal (akses editor yang sudah login) ---
	mux.Handle("GET /api/editor/my-forms", s.authMW(s.requireRole(s.editorMyForms, "editor")))
	mux.Handle("GET /api/editor/forms/{id}", s.authMW(s.requireRole(s.editorGetForm, "editor")))
	mux.Handle("GET /api/editor/forms/{id}/responses", s.authMW(s.requireRole(s.editorListResponses, "editor")))
	mux.Handle("GET /api/editor/forms/{id}/responses/{responseId}", s.authMW(s.requireRole(s.editorGetResponse, "editor")))
	mux.Handle("PATCH /api/editor/forms/{id}/responses/{responseId}", s.authMW(s.requireRole(s.editorUpdateResponse, "editor")))

	// --- publik: data referensi (tanpa login) ---
	mux.HandleFunc("GET /api/wilayah", s.wilayahList)

	// --- publik: akses kuesioner (tanpa login) ---
	mux.HandleFunc("GET /api/public/forms/{token}", s.publicGetForm)

	// --- publik: respondent (perlu JWT Google) ---
	mux.Handle("GET /api/public/me", s.respondentMW(s.respondentMe))
	mux.Handle("GET /api/public/forms/{token}/my-response", s.respondentMW(s.myResponse))
	mux.Handle("GET /api/public/forms/{token}/my-responses", s.respondentMW(s.myResponses))
	mux.Handle("GET /api/public/forms/{token}/check-access", s.respondentMW(s.checkAccess))
	mux.Handle("POST /api/public/forms/{token}/responses", s.respondentMW(s.publicSubmit))
	mux.Handle("POST /api/public/forms/{token}/responses/{responseId}/unsubmit", s.respondentMW(s.unsubmitResponse))
	mux.Handle("GET /api/public/forms/{token}/draft", s.respondentMW(s.myDraft))
	mux.Handle("POST /api/public/forms/{token}/draft", s.respondentMW(s.saveDraftHandler))

	// --- OAuth Google (redirect, tidak butuh JWT) ---
	mux.HandleFunc("GET /auth/google", s.googleLogin)
	mux.HandleFunc("GET /auth/google/viewer", s.googleViewerLogin)
	mux.HandleFunc("GET /auth/google/callback", s.googleCallback)

	// --- halaman ---
	mux.HandleFunc("GET /login", s.page("login.html"))
	mux.HandleFunc("GET /admin", s.page("admin.html"))
	mux.HandleFunc("GET /builder", s.page("builder.html"))
	mux.HandleFunc("GET /f/{token}", s.page("public.html"))                  // halaman isi kuesioner publik
	mux.HandleFunc("GET /responses", s.page("responses.html"))               // halaman daftar jawaban
	mux.HandleFunc("GET /response-view", s.page("response-view.html"))       // halaman lihat detail jawaban
	mux.HandleFunc("GET /viewer-portal", s.page("viewer-portal.html"))               // portal viewer & editor
	mux.HandleFunc("GET /viewer-responses", s.page("viewer-responses.html"))         // jawaban terbatas viewer
	mux.HandleFunc("GET /editor-responses", s.page("editor-responses.html"))         // jawaban editor
	mux.HandleFunc("GET /portal-response-view", s.page("portal-response-view.html")) // detail jawaban viewer/editor
	mux.HandleFunc("GET /auth/google/done", s.page("google-done.html"))              // landing setelah OAuth

	// aset statis tiap halaman (CSS/JS terpisah dari HTML)
	for _, f := range []string{
		"login.css", "login.js",
		"admin.css", "admin.js",
		"builder.css", "builder.js", "builder-bridge.js",
	} {
		mux.HandleFunc("GET /"+f, s.page(f))
	}

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
