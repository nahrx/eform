package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nahrx/eform/internal/auth"
	"github.com/nahrx/eform/internal/models"
	"github.com/nahrx/eform/internal/store"
)

// resolveShare validates the token: active, not expired, and (when required) the password matches.
func (s *Server) resolveShare(w http.ResponseWriter, r *http.Request) (*models.Share, bool) {
	token := r.PathValue("token")
	sh, err := s.st.GetShareByToken(r.Context(), token)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "link not found")
		return nil, false
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "server error")
		return nil, false
	}
	if !sh.IsActive {
		writeErr(w, http.StatusGone, "the link has been disabled")
		return nil, false
	}
	if sh.ExpiresAt != nil && time.Now().After(*sh.ExpiresAt) {
		writeErr(w, http.StatusGone, "the link has expired")
		return nil, false
	}
	if sh.PasswordHash != nil {
		pw := r.Header.Get("X-Share-Password")
		if !auth.CheckPassword(*sh.PasswordHash, pw) {
			writeErr(w, http.StatusUnauthorized, "incorrect link password")
			return nil, false
		}
	}
	return sh, true
}

// GET /api/public/forms/{token} — public access to the form schema (no login needed).
func (s *Server) publicGetForm(w http.ResponseWriter, r *http.Request) {
	sh, ok := s.resolveShare(w, r)
	if !ok {
		return
	}
	f, err := s.st.GetForm(r.Context(), sh.FormID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "form not found")
		return
	}
	if f.Status != "published" {
		writeErr(w, http.StatusForbidden, "the form has not been published")
		return
	}
	s.st.IncrementShareView(r.Context(), sh.ID)

	writeJSON(w, http.StatusOK, map[string]any{
		"id":             f.ID,
		"title":          f.Title,
		"description":    f.Description,
		"version":        f.Version,
		"schema":         f.Schema,
		"allowResponses": sh.AllowResponses,
		"multiResponse":  sh.MultiResponse,
		"accessMode":     sh.AccessMode,
		"requireAuth":    true,
		"googleEnabled":  s.cfg.GoogleClientID != "",
	})
}

// GET /api/public/me — details of the signed-in respondent.
func (s *Server) respondentMe(w http.ResponseWriter, r *http.Request) {
	rc := respondentFrom(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"id":      rc.RespondentID,
		"email":   rc.Email,
		"name":    rc.Name,
		"picture": rc.Picture,
	})
}

// GET /api/public/forms/{token}/my-response — the respondent's latest answer for this form.
func (s *Server) myResponse(w http.ResponseWriter, r *http.Request) {
	rc := respondentFrom(r.Context())
	sh, ok := s.resolveShare(w, r)
	if !ok {
		return
	}
	resp, err := s.st.GetResponseByFormAndRespondent(r.Context(), sh.FormID, rc.RespondentID)
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusOK, nil)
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "server error")
		return
	}
	writeJSON(w, http.StatusOK, s.signResponse(resp))
}

// GET /api/public/forms/{token}/my-responses — all of the respondent's answers (for multi-response).
func (s *Server) myResponses(w http.ResponseWriter, r *http.Request) {
	rc := respondentFrom(r.Context())
	sh, ok := s.resolveShare(w, r)
	if !ok {
		return
	}
	resps, err := s.st.ListResponsesByFormAndRespondent(r.Context(), sh.FormID, rc.RespondentID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "server error")
		return
	}
	if resps == nil {
		resps = []models.Response{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"responses": s.signResponses(resps)})
}

// GET /api/public/forms/{token}/check-access — check whether the respondent's email is allowed (restricted mode).
func (s *Server) checkAccess(w http.ResponseWriter, r *http.Request) {
	rc := respondentFrom(r.Context())
	sh, ok := s.resolveShare(w, r)
	if !ok {
		return
	}
	if sh.AccessMode != "restricted" {
		writeJSON(w, http.StatusOK, map[string]bool{"allowed": true})
		return
	}
	allowed, err := s.st.IsEmailAllowed(r.Context(), sh.ID, rc.Email)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"allowed": allowed})
}

// POST /api/public/forms/{token}/responses — submit or save a draft response.
// Body: {answers, draft?: bool, responseId?: string}
// Memerlukan JWT respondent (login Google).
func (s *Server) publicSubmit(w http.ResponseWriter, r *http.Request) {
	sh, ok := s.resolveShare(w, r)
	if !ok {
		return
	}
	if !sh.AllowResponses {
		writeErr(w, http.StatusForbidden, "this link does not accept responses")
		return
	}
	rc := respondentFrom(r.Context())

	// Check access when the share is in restricted mode
	if sh.AccessMode == "restricted" {
		allowed, err := s.st.IsEmailAllowed(r.Context(), sh.ID, rc.Email)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "server error")
			return
		}
		if !allowed {
			writeErr(w, http.StatusForbidden, "your email is not on the access list for this form")
			return
		}
	}

	var in struct {
		Answers    json.RawMessage `json:"answers"`
		Draft      bool            `json:"draft"`
		ResponseID string          `json:"responseId"`
	}
	if err := decodeJSON(r, &in); err != nil || len(in.Answers) == 0 {
		writeErr(w, http.StatusBadRequest, "empty response or invalid format")
		return
	}
	meta := map[string]any{
		"ip":         s.clientIP(r),
		"userAgent":  r.UserAgent(),
		"receivedAt": time.Now().Format(time.RFC3339),
		"email":      rc.Email,
		"name":       rc.Name,
	}
	metaJSON, _ := json.Marshal(meta)
	sid := sh.ID
	var resp *models.Response
	var err error

	if sh.MultiResponse {
		status := "submitted"
		if in.Draft {
			status = "draft"
		}
		if in.ResponseID != "" {
			// Update the existing draft (it must still be 'draft' and belong to this respondent)
			resp, err = s.st.UpdateMultiResponseDraft(r.Context(), in.ResponseID, rc.RespondentID, sh.FormID, status, in.Answers, metaJSON)
			if errors.Is(err, store.ErrNotFound) {
				writeErr(w, http.StatusNotFound, "response not found or does not belong to you")
				return
			}
		} else {
			// Create a new row — make sure no unfinished draft is still active
			if !in.Draft {
				hasDraft, chkErr := s.st.HasDraftResponse(r.Context(), sh.FormID, rc.RespondentID)
				if chkErr != nil {
					writeErr(w, http.StatusInternalServerError, "server error")
					return
				}
				if hasDraft {
					writeErr(w, http.StatusConflict, "You still have an unfinished draft — please continue or discard it first")
					return
				}
			}
			resp, err = s.st.CreateMultiResponseRow(r.Context(), sh.FormID, &sid, rc.RespondentID, status, in.Answers, metaJSON)
		}
	} else if in.ResponseID != "" {
		// Single-response: re-completing a response that was previously unsubmitted
		// was unsubmitted (status 'draft') back to 'submitted'.
		resp, err = s.st.UpdateMultiResponseDraft(r.Context(), in.ResponseID, rc.RespondentID, sh.FormID, "submitted", in.Answers, metaJSON)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "response not found or does not belong to you")
			return
		}
	} else {
		// Single-response: upsert (satu respons per respondent)
		resp, err = s.st.UpsertResponse(r.Context(), sh.FormID, &sid, rc.RespondentID, in.Answers, metaJSON)
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to save response")
		return
	}
	if !in.Draft {
		_ = s.st.DeleteDraft(r.Context(), sh.FormID, rc.RespondentID)
	}
	code := http.StatusCreated
	if in.Draft {
		code = http.StatusOK
	}
	writeJSON(w, code, map[string]any{"id": resp.ID, "status": resp.Status, "submittedAt": resp.SubmittedAt})
}

// POST /api/public/forms/{token}/responses/{responseId}/unsubmit
// Moves an already-submitted response back to draft so it can be edited.
func (s *Server) unsubmitResponse(w http.ResponseWriter, r *http.Request) {
	rc := respondentFrom(r.Context())
	sh, ok := s.resolveShare(w, r)
	if !ok {
		return
	}
	if !sh.AllowResponses {
		writeErr(w, http.StatusForbidden, "this link does not accept response edits")
		return
	}
	responseID := r.PathValue("responseId")
	resp, err := s.st.UnsubmitResponse(r.Context(), responseID, rc.RespondentID, sh.FormID)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "response not found or does not belong to you")
		return
	}
	if err != nil {
		writeErr(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": resp.ID, "status": resp.Status})
}

// GET /api/public/forms/{token}/draft — fetch the draft stored on the server.
func (s *Server) myDraft(w http.ResponseWriter, r *http.Request) {
	rc := respondentFrom(r.Context())
	sh, ok := s.resolveShare(w, r)
	if !ok {
		return
	}
	draft, err := s.st.GetDraftByFormAndRespondent(r.Context(), sh.FormID, rc.RespondentID)
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusOK, nil)
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "server error")
		return
	}
	draft.Answers = s.signAnswerUploads(draft.Answers)
	writeJSON(w, http.StatusOK, draft)
}

// POST /api/public/forms/{token}/draft — save a draft to the server (upsert).
func (s *Server) saveDraftHandler(w http.ResponseWriter, r *http.Request) {
	sh, ok := s.resolveShare(w, r)
	if !ok {
		return
	}
	rc := respondentFrom(r.Context())

	var in struct {
		Answers json.RawMessage `json:"answers"`
		CurPage int             `json:"curPage"`
	}
	if err := decodeJSON(r, &in); err != nil || len(in.Answers) == 0 {
		writeErr(w, http.StatusBadRequest, "invalid format or empty response")
		return
	}
	sid := sh.ID
	draft, err := s.st.UpsertDraft(r.Context(), sh.FormID, &sid, rc.RespondentID, in.Answers, in.CurPage)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to save draft")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": draft.ID, "savedAt": draft.SavedAt})
}

// GET /api/wilayah?prov=&kab=&kec=
// Returns the child regions of the most specific parameter supplied:
//   - prov only   → the regencies/cities under that province
//   - prov + kab  → the districts under that regency
//   - prov + kab + kec → the villages under that district
//   - prov + kab + kec + desa → the SLS units under that village
//   - prov + kab + kec + desa + sls → the Sub-SLS units under that SLS
//
// The parameter value is a kode_wilayah (for example "64", "6401", "6401010").
// This endpoint requires no authentication.
func (s *Server) wilayahList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	prov := q.Get("prov")
	kab := q.Get("kab")
	kec := q.Get("kec")
	desa := q.Get("desa")
	sls := q.Get("sls")

	// Use the most specific parameter as kode_parent
	var parent string
	switch {
	case sls != "":
		parent = sls
	case desa != "":
		parent = desa
	case kec != "":
		parent = kec
	case kab != "":
		parent = kab
	case prov != "":
		parent = prov
	default:
		// No parameter → return every province
	}

	items, err := s.st.GetWilayahByParent(r.Context(), parent)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "server error")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

var optionsProxyClient = &http.Client{
	Timeout: 10 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("too many redirects")
		}
		return ensurePublicHost(req.URL.Hostname())
	},
}

// ensurePublicHost rejects hosts that point at private or local networks, so this
// proxy endpoint cannot be abused to scan the internal network (SSRF).
func ensurePublicHost(host string) error {
	if host == "" {
		return errors.New("empty host")
	}
	var ips []net.IP
	if ip := net.ParseIP(host); ip != nil {
		ips = []net.IP{ip}
	} else {
		resolved, err := net.LookupIP(host)
		if err != nil {
			return err
		}
		ips = resolved
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
			ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
			return errors.New("host address is not allowed")
		}
	}
	return nil
}

// GET /api/options-proxy?url=... — a server-side proxy for dynamic dropdown option
// sources (optionsApi in the form schema) that point at an external API.
// It exists so the browser never needs connect-src straight to a third-party domain
// (which varies with whatever the form author configured), and at the same time keeps
// mencegah token internal (Authorization admin/viewer/editor) ikut terkirim
// to a third-party server. This endpoint requires no authentication because it
// only forwards a GET request to a URL already fixed in the schema.
func (s *Server) optionsProxy(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("url")
	if target == "" {
		writeErr(w, http.StatusBadRequest, "the url parameter is required")
		return
	}
	u, err := url.Parse(target)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		writeErr(w, http.StatusBadRequest, "invalid url")
		return
	}
	if err := ensurePublicHost(u.Hostname()); err != nil {
		writeErr(w, http.StatusBadRequest, "url is not allowed")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "failed to build the request")
		return
	}
	req.Header.Set("Accept", "application/json")

	resp, err := optionsProxyClient.Do(req)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "failed to reach the external API")
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if err != nil {
		writeErr(w, http.StatusBadGateway, "failed to read the external API response")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}

// clientIP returns the caller's real IP address.
//
// X-Forwarded-For is trusted only when the connection genuinely comes from a proxy
// listed in TRUSTED_PROXIES — otherwise anyone could forge that header
// and IP-based restrictions (an API key allowlist, say) become meaningless.
func (s *Server) clientIP(r *http.Request) string {
	host := remoteHost(r)
	if len(s.cfg.TrustedProxies) == 0 || !ipInAny(host, s.cfg.TrustedProxies) {
		return host
	}
	// Take the right-most entry that is NOT a trusted proxy: that is the real client.
	// Entries to its left may have been written by the client itself.
	parts := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(parts) - 1; i >= 0; i-- {
		ip := strings.TrimSpace(parts[i])
		if ip == "" {
			continue
		}
		if !ipInAny(ip, s.cfg.TrustedProxies) {
			return ip
		}
	}
	return host
}

func remoteHost(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ipInAny reports whether ip falls within any entry (a single IP or a CIDR).
func ipInAny(ip string, entries []string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, e := range entries {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		if strings.Contains(e, "/") {
			if _, netw, err := net.ParseCIDR(e); err == nil && netw.Contains(parsed) {
				return true
			}
			continue
		}
		if other := net.ParseIP(e); other != nil && other.Equal(parsed) {
			return true
		}
	}
	return false
}
