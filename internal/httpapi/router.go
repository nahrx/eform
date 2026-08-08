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

	// --- forms (login required) ---
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
	mux.Handle("GET /api/forms/{id}/responses.xlsx", s.authMW(s.exportResponsesXLSX))
	mux.Handle("GET /api/forms/{id}/fields/{fieldName}/suggested-values", s.authMW(s.requireRole(s.suggestedFieldValues, "superadmin", "admin")))
	// Which devices are still holding unsent answers. Restricted to superadmin and the
	// owning admin, not viewers or editors: the report names the respondent, and a
	// viewer's respondent scope does not necessarily cover everyone who is stuck.
	mux.Handle("GET /api/forms/{id}/queue-reports", s.authMW(s.requireRole(s.listQueueReports, "superadmin", "admin")))

	// --- users (superadmin only) ---
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

	// --- API key management per form (superadmin / owning admin) ---
	mux.Handle("POST /api/forms/{id}/api-keys", s.authMW(s.requireRole(s.createAPIKey, "superadmin", "admin")))
	mux.Handle("GET /api/forms/{id}/api-keys", s.authMW(s.requireRole(s.listAPIKeys, "superadmin", "admin")))
	mux.Handle("PUT /api/api-keys/{keyId}", s.authMW(s.requireRole(s.updateAPIKey, "superadmin", "admin")))
	mux.Handle("DELETE /api/api-keys/{keyId}", s.authMW(s.requireRole(s.deleteAPIKey, "superadmin", "admin")))
	mux.Handle("POST /api/api-keys/{keyId}/rotate", s.authMW(s.requireRole(s.rotateAPIKey, "superadmin", "admin")))
	mux.Handle("GET /api/api-keys/{keyId}/respondents", s.authMW(s.requireRole(s.listAPIKeyRespondents, "superadmin", "admin")))
	mux.Handle("POST /api/api-keys/{keyId}/respondents", s.authMW(s.requireRole(s.addAPIKeyRespondent, "superadmin", "admin")))
	mux.Handle("DELETE /api/api-key-respondents/{id}", s.authMW(s.requireRole(s.removeAPIKeyRespondent, "superadmin", "admin")))
	mux.Handle("GET /api/api-keys/{keyId}/logs", s.authMW(s.requireRole(s.listAPIKeyLogs, "superadmin", "admin")))

	// --- admin action history (audit) ---
	mux.Handle("GET /api/activity-logs", s.authMW(s.requireRole(s.listActivityLogs, "superadmin", "admin")))

	// answer change history: the form's owning admin, or an authorised editor
	mux.Handle("GET /api/forms/{id}/responses/{responseId}/revisions", s.authMW(s.requireRole(s.listResponseRevisions, "superadmin", "admin", "editor", "viewer")))

	// which instrument this response was filled against, and whether the form has been
	// edited since — read by every response detail page to warn that what it renders
	// may no longer be what was asked
	mux.Handle("GET /api/forms/{id}/responses/{responseId}/schema-version", s.authMW(s.requireRole(s.responseSchemaVersion, "superadmin", "admin", "editor", "viewer")))

	// --- Public API for external systems: API key authentication, read-only ---
	// Every restriction (active, expiry, IP, quota) lives in apiKeyMW; the data scope
	// lives in store.APIKeyScope. There are no write endpoints here, and none will be added.
	mux.Handle("GET /api/v1/me", s.apiKeyMW(s.apiMe))
	mux.Handle("GET /api/v1/forms/{formId}/responses", s.apiKeyMW(s.apiListResponses))
	mux.Handle("GET /api/v1/forms/{formId}/responses.csv", s.apiKeyMW(s.apiExportResponses))
	mux.Handle("GET /api/v1/forms/{formId}/responses/{responseId}", s.apiKeyMW(s.apiGetResponse))

	// --- viewer portal (for signed-in viewers) ---
	// the role check is widened to "viewer","editor" because one account can now be a viewer
	// on one form and an editor on another — the real per-form authorisation
	// is still checked inside each handler via GetViewerPermission.
	mux.Handle("GET /api/viewer/my-forms", s.authMW(s.requireRole(s.viewerMyForms, "viewer", "editor")))
	mux.Handle("GET /api/viewer/forms/{id}", s.authMW(s.requireRole(s.viewerGetForm, "viewer", "editor")))
	mux.Handle("GET /api/viewer/forms/{id}/permission", s.authMW(s.requireRole(s.viewerMyFormPermission, "viewer", "editor")))
	mux.Handle("GET /api/viewer/forms/{id}/responses", s.authMW(s.requireRole(s.viewerListResponses, "viewer", "editor")))
	mux.Handle("GET /api/viewer/forms/{id}/responses/{responseId}", s.authMW(s.requireRole(s.viewerGetResponse, "viewer", "editor")))
	mux.Handle("GET /api/viewer/forms/{id}/responses.csv", s.authMW(s.requireRole(s.viewerExportResponses, "viewer", "editor")))
	mux.Handle("GET /api/viewer/forms/{id}/responses.xlsx", s.authMW(s.requireRole(s.viewerExportResponsesXLSX, "viewer", "editor")))

	// --- editor portal (for signed-in editors) ---
	mux.Handle("GET /api/editor/my-forms", s.authMW(s.requireRole(s.editorMyForms, "editor", "viewer")))
	mux.Handle("GET /api/editor/forms/{id}", s.authMW(s.requireRole(s.editorGetForm, "editor", "viewer")))
	mux.Handle("GET /api/editor/forms/{id}/responses", s.authMW(s.requireRole(s.editorListResponses, "editor", "viewer")))
	mux.Handle("GET /api/editor/forms/{id}/responses/{responseId}", s.authMW(s.requireRole(s.editorGetResponse, "editor", "viewer")))
	mux.Handle("PATCH /api/editor/forms/{id}/responses/{responseId}", s.authMW(s.requireRole(s.editorUpdateResponse, "editor", "viewer")))
	mux.Handle("GET /api/editor/forms/{id}/responses.csv", s.authMW(s.requireRole(s.editorExportResponses, "editor", "viewer")))
	mux.Handle("GET /api/editor/forms/{id}/responses.xlsx", s.authMW(s.requireRole(s.editorExportResponsesXLSX, "editor", "viewer")))

	// --- public: reference data (no login) ---
	mux.HandleFunc("GET /api/wilayah", s.wilayahList)
	mux.HandleFunc("GET /api/options-proxy", s.optionsProxy)

	// --- public: form access (no login) ---
	mux.HandleFunc("GET /api/public/forms/{token}", s.publicGetForm)

	// --- public: PWA (offline mode, multi-response forms only) ---
	mux.HandleFunc("GET /api/public/forms/{token}/manifest.webmanifest", s.publicManifest)
	mux.HandleFunc("GET /api/public/forms/{token}/icon.png", s.publicIcon)
	mux.HandleFunc("GET /sw.js", s.page("sw.js"))
	// Requested by the browser before any script runs, so a <link rel="icon"> added
	// from JavaScript cannot prevent it. Without this it 404s on every page load.
	mux.HandleFunc("GET /favicon.ico", s.faviconICO)

	// --- public: respondent (Google JWT required) ---
	mux.Handle("GET /api/public/me", s.respondentMW(s.respondentMe))
	mux.Handle("GET /api/public/forms/{token}/my-response", s.respondentMW(s.myResponse))
	mux.Handle("GET /api/public/forms/{token}/my-responses", s.respondentMW(s.myResponses))
	mux.Handle("GET /api/public/forms/{token}/check-access", s.respondentMW(s.checkAccess))
	// Endpoints that write data are rate-limited: per respondent account and per IP.
	// Drafts are saved automatically while filling in, so their quota is the most generous.
	mux.Handle("POST /api/public/forms/{token}/uploads", s.respondentMW(s.limitRespondent(s.publicUpload, 30, 90)))
	mux.Handle("POST /api/public/forms/{token}/responses", s.respondentMW(s.limitRespondent(s.publicSubmit, 20, 60)))
	mux.Handle("POST /api/public/forms/{token}/responses/{responseId}/unsubmit", s.respondentMW(s.limitRespondent(s.unsubmitResponse, 20, 60)))
	mux.Handle("GET /api/public/forms/{token}/draft", s.respondentMW(s.myDraft))
	mux.Handle("POST /api/public/forms/{token}/draft", s.respondentMW(s.limitRespondent(s.saveDraftHandler, 120, 300)))
	// Offline devices report what their queue is still holding. Sent after every flush
	// and on load, so the last known state survives the respondent's token expiring.
	mux.Handle("POST /api/public/forms/{token}/queue-report", s.respondentMW(s.limitRespondent(s.queueReport, 30, 90)))

	// --- Google OAuth (redirects, no JWT required) ---
	mux.HandleFunc("GET /auth/google", s.googleLogin)
	mux.HandleFunc("GET /auth/google/viewer", s.googleViewerLogin)
	mux.HandleFunc("GET /auth/google/callback", s.googleCallback)

	// --- pages ---
	mux.HandleFunc("GET /login", s.page("login.html"))
	mux.HandleFunc("GET /admin", s.page("admin.html"))
	mux.HandleFunc("GET /manage", s.page("manage.html")) // single-form management page (builder/share/access)
	mux.HandleFunc("GET /builder", s.page("builder.html"))
	mux.HandleFunc("GET /f/{token}", s.page("public.html"))                          // public form-filling page
	mux.HandleFunc("GET /responses", s.page("responses.html"))                       // response list page
	mux.HandleFunc("GET /response-view", s.page("response-view.html"))               // response detail page
	mux.HandleFunc("GET /viewer-portal", s.page("viewer-portal.html"))               // viewer & editor portal
	mux.HandleFunc("GET /viewer-responses", s.page("viewer-responses.html"))         // viewer's restricted responses
	mux.HandleFunc("GET /editor-responses", s.page("editor-responses.html"))         // editor's responses
	mux.HandleFunc("GET /portal-response-view", s.page("portal-response-view.html")) // viewer/editor response detail
	mux.HandleFunc("GET /auth/google/done", s.page("google-done.html"))              // landing page after OAuth

	// per-page static assets (CSS/JS kept separate from the HTML)
	for _, f := range []string{
		"login.css", "login.js",
		"admin.css", "admin.js",
		"manage.css", "manage.js",
		"responses.css", "responses-ui.js", "responses-core.js",
		"builder.css", "builder.js", "builder-bridge.js",
		"searchable-select.js", "geo-map.js", "revision-history.js",
		"response-validation.js", "offline-queue.js", "image-compress.js",
		"schema-version-notice.js",
		"i18n.js", "responsive-tables.js",
	} {
		mux.HandleFunc("GET /"+f, s.page(f))
	}

	// vendored third-party library assets (Leaflet, for instance) — served from web/vendor/
	mux.Handle("GET /vendor/", http.StripPrefix("/vendor/", http.FileServer(http.Dir(filepath.Join(s.cfg.WebDir, "vendor")))))

	// uploads: only files may be fetched directly; directory listings are refused.
	mux.HandleFunc("GET /uploads/", s.uploadFileOnly)
	mux.HandleFunc("HEAD /uploads/", s.uploadFileOnly)

	// public landing page: serve the PublicDir folder (index.html at "/").
	// The "GET /{$}" pattern matches only "/" exactly, so index.html can go
	// through serveHTML (with the organisation placeholders filled in) while other
	// other assets in PublicDir stay with the FileServer.
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		s.serveHTML(w, r, filepath.Join(s.cfg.PublicDir, "index.html"))
	})
	// The "GET /" pattern is a catch-all; the more specific routes above still win.
	fileServer := http.FileServer(http.Dir(s.cfg.PublicDir))
	mux.Handle("GET /", fileServer)

	return s.wrap(mux)
}

func (s *Server) page(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(s.cfg.WebDir, name)
		// HTML files go through serveHTML so the organisation placeholders get
		// filled in; other assets (CSS/JS) contain no placeholders at all.
		if strings.HasSuffix(name, ".html") {
			s.serveHTML(w, r, path)
			return
		}
		http.ServeFile(w, r, path)
	}
}

func (s *Server) uploadFileOnly(w http.ResponseWriter, r *http.Request) {
	p := path.Clean("/" + strings.TrimPrefix(r.URL.Path, "/"))
	if p == "/uploads" || strings.HasSuffix(r.URL.Path, "/") {
		http.NotFound(w, r)
		return
	}

	// Attachments can only be fetched through a signed URL issued when the response is
	// served to someone who is actually entitled to it (see uploads_sign.go).
	// Deliberately 404 rather than 403, so the file's existence is not confirmed.
	if !s.verifyUploadURL(p, r.URL.Query()) {
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
