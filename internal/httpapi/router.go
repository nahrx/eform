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
	mux.Handle("PUT /api/forms/{id}", s.authMW(s.updateForm))
	mux.Handle("DELETE /api/forms/{id}", s.authMW(s.deleteForm))
	mux.Handle("POST /api/forms/{id}/publish", s.authMW(s.publishForm))

	// --- shares ---
	mux.Handle("POST /api/forms/{id}/shares", s.authMW(s.createShare))
	mux.Handle("GET /api/forms/{id}/shares", s.authMW(s.listShares))
	mux.Handle("PATCH /api/shares/{id}", s.authMW(s.updateShare))
	mux.Handle("DELETE /api/shares/{id}", s.authMW(s.revokeShare))
	mux.Handle("DELETE /api/shares/{id}/permanent", s.authMW(s.deleteSharePermanent))
	mux.Handle("GET /api/shares/{id}/allowed-emails", s.authMW(s.listAllowedEmails))
	mux.Handle("POST /api/shares/{id}/allowed-emails", s.authMW(s.addAllowedEmail))
	mux.Handle("DELETE /api/share-emails/{id}", s.authMW(s.removeAllowedEmail))

	// --- responses ---
	mux.Handle("GET /api/forms/{id}/responses", s.authMW(s.listResponses))
	mux.Handle("GET /api/forms/{id}/responses/{responseId}", s.authMW(s.getResponseDetail))
	mux.Handle("GET /api/forms/{id}/responses.csv", s.authMW(s.exportResponses))

	// --- users (khusus superadmin) ---
	mux.Handle("POST /api/users", s.authMW(s.requireRole(s.createUser, "superadmin")))
	mux.Handle("GET /api/users", s.authMW(s.requireRole(s.listUsers, "superadmin")))

	// --- viewers (superadmin kelola akun viewer) ---
	mux.Handle("POST /api/viewers", s.authMW(s.requireRole(s.createViewer, "superadmin")))
	mux.Handle("GET /api/viewers", s.authMW(s.requireRole(s.listViewers, "superadmin")))
	mux.Handle("DELETE /api/viewers/{id}", s.authMW(s.requireRole(s.deleteViewer, "superadmin")))

	// --- viewer permissions per form (superadmin) ---
	mux.Handle("POST /api/forms/{id}/viewer-permissions", s.authMW(s.requireRole(s.createViewerPermission, "superadmin")))
	mux.Handle("GET /api/forms/{id}/viewer-permissions", s.authMW(s.requireRole(s.listFormViewerPermissions, "superadmin")))
	mux.Handle("GET /api/viewer-permissions/{permId}", s.authMW(s.requireRole(s.getViewerPermission, "superadmin")))
	mux.Handle("PUT /api/viewer-permissions/{permId}", s.authMW(s.requireRole(s.updateViewerPermission, "superadmin")))
	mux.Handle("DELETE /api/viewer-permissions/{permId}", s.authMW(s.requireRole(s.deleteViewerPermission, "superadmin")))

	// --- allowed respondents per permission (superadmin) ---
	mux.Handle("GET /api/viewer-permissions/{permId}/respondents", s.authMW(s.requireRole(s.listViewerAllowedRespondents, "superadmin")))
	mux.Handle("POST /api/viewer-permissions/{permId}/respondents", s.authMW(s.requireRole(s.addViewerAllowedRespondent, "superadmin")))
	mux.Handle("DELETE /api/viewer-respondents/{id}", s.authMW(s.requireRole(s.removeViewerAllowedRespondent, "superadmin")))
	mux.Handle("GET /api/forms/{id}/respondents", s.authMW(s.requireRole(s.listFormRespondents, "superadmin")))

	// --- viewer portal (akses viewer yang sudah login) ---
	mux.Handle("GET /api/viewer/my-forms", s.authMW(s.requireRole(s.viewerMyForms, "viewer")))
	mux.Handle("GET /api/viewer/forms/{id}/permission", s.authMW(s.requireRole(s.viewerMyFormPermission, "viewer")))
	mux.Handle("GET /api/viewer/forms/{id}/responses", s.authMW(s.requireRole(s.viewerListResponses, "viewer")))

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
	mux.HandleFunc("GET /f/{token}", s.page("public.html"))       // halaman isi kuesioner publik
	mux.HandleFunc("GET /responses", s.page("responses.html"))           // halaman daftar jawaban
	mux.HandleFunc("GET /response-view", s.page("response-view.html"))  // halaman lihat detail jawaban
	mux.HandleFunc("GET /viewer-portal", s.page("viewer-portal.html"))  // portal viewer
	mux.HandleFunc("GET /viewer-responses", s.page("viewer-responses.html")) // jawaban terbatas viewer
	mux.HandleFunc("GET /auth/google/done", s.page("google-done.html")) // landing setelah OAuth

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
