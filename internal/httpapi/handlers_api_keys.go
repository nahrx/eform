package httpapi

import (
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nahrx/eform/internal/models"
	"github.com/nahrx/eform/internal/store"
)

/* API key management from the /manage panel (the "API" menu).
   Every handler in this file requires an admin JWT + ensureFormAccess — meaning a
   superadmin, or the admin who owns the form. The endpoints external systems call
   live in handlers_api_public.go and authenticate with an API key, not a JWT. */

// apiKeyInput is the create/update API key request shape sent by the panel.
type apiKeyInput struct {
	Label             string            `json:"label"`
	RespondentAccess  string            `json:"respondentAccess"`
	VisibleFields     []string          `json:"visibleFields"`
	FieldFilters      map[string]string `json:"fieldFilters"`
	IncludeRespondent bool              `json:"includeRespondent"`
	AllowedIPs        []string          `json:"allowedIps"`
	RateLimitPerMin   int               `json:"rateLimitPerMin"`
	IsActive          *bool             `json:"isActive"`
	ExpiresAt         string            `json:"expiresAt"`
}

// toModel validates the input and copies it into the model. Values outside a sensible range
// are normalised rather than rejected — except those that are genuinely malformed.
func (in apiKeyInput) toModel(formID string) (*models.FormAPIKey, error) {
	k := &models.FormAPIKey{
		FormID:            formID,
		Label:             strings.TrimSpace(in.Label),
		RespondentAccess:  in.RespondentAccess,
		VisibleFields:     in.VisibleFields,
		FieldFilters:      in.FieldFilters,
		IncludeRespondent: in.IncludeRespondent,
		RateLimitPerMin:   in.RateLimitPerMin,
		IsActive:          true,
	}
	if k.RespondentAccess != "all" && k.RespondentAccess != "selected" {
		k.RespondentAccess = "all"
	}
	if k.RateLimitPerMin <= 0 || k.RateLimitPerMin > 6000 {
		k.RateLimitPerMin = 60
	}
	if in.IsActive != nil {
		k.IsActive = *in.IsActive
	}
	for _, raw := range in.AllowedIPs {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}
		if strings.Contains(entry, "/") {
			if _, _, err := net.ParseCIDR(entry); err != nil {
				return nil, errors.New("invalid CIDR format: " + entry)
			}
		} else if net.ParseIP(entry) == nil {
			return nil, errors.New("invalid IP address format: " + entry)
		}
		k.AllowedIPs = append(k.AllowedIPs, entry)
	}
	if in.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, in.ExpiresAt)
		if err != nil {
			return nil, errors.New("expiresAt must be in RFC3339 format")
		}
		k.ExpiresAt = &t
	}
	return k, nil
}

// newAPIKeyCredential issues a fresh credential: the plaintext key to show once, a prefix
// for identification, and a hash to store.
func newAPIKeyCredential() (plaintext, prefix, hash string) {
	plaintext = apiKeyPrefix + randToken(32)
	body := strings.TrimPrefix(plaintext, apiKeyPrefix)
	return plaintext, body[:10], hashAPIKey(plaintext)
}

// createAPIKey creates a new API key. This is the only response that ever carries the
// plaintext key — afterwards only its prefix can be seen again.
func (s *Server) createAPIKey(w http.ResponseWriter, r *http.Request) {
	formID := r.PathValue("id")
	if _, ok := s.ensureFormAccess(w, r, formID); !ok {
		return
	}
	var in apiKeyInput
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request format")
		return
	}
	k, err := in.toModel(formID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	plaintext, prefix, hash := newAPIKeyCredential()
	createdBy := userFrom(r.Context()).Subject
	out, err := s.st.CreateAPIKey(r.Context(), k, prefix, hash, &createdBy)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to create API key")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"apiKey": out, "key": plaintext})
}

// listAPIKeys returns every API key for one form (without the plaintext key or its hash).
func (s *Server) listAPIKeys(w http.ResponseWriter, r *http.Request) {
	formID := r.PathValue("id")
	if _, ok := s.ensureFormAccess(w, r, formID); !ok {
		return
	}
	keys, err := s.st.ListAPIKeysByForm(r.Context(), formID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"apiKeys": keys})
}

// updateAPIKey updates one key's label and its scope/security settings.
func (s *Server) updateAPIKey(w http.ResponseWriter, r *http.Request) {
	key, ok := s.ensureAPIKeyAccess(w, r, r.PathValue("keyId"))
	if !ok {
		return
	}
	var in apiKeyInput
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request format")
		return
	}
	next, err := in.toModel(key.FormID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	out, err := s.st.UpdateAPIKey(r.Context(), key.ID, next)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "API key not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to update API key")
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// rotateAPIKey issues a new credential for the same key. The old key stops working
// immediately, while all of its scope settings are preserved.
func (s *Server) rotateAPIKey(w http.ResponseWriter, r *http.Request) {
	key, ok := s.ensureAPIKeyAccess(w, r, r.PathValue("keyId"))
	if !ok {
		return
	}
	plaintext, prefix, hash := newAPIKeyCredential()
	out, err := s.st.RotateAPIKey(r.Context(), key.ID, prefix, hash)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to rotate API key")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"apiKey": out, "key": plaintext})
}

// deleteAPIKey removes an API key along with its respondent list and clears its reference in the audit log.
func (s *Server) deleteAPIKey(w http.ResponseWriter, r *http.Request) {
	key, ok := s.ensureAPIKeyAccess(w, r, r.PathValue("keyId"))
	if !ok {
		return
	}
	if err := s.st.DeleteAPIKey(r.Context(), key.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to delete API key")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// listAPIKeyRespondents returns the respondents this key may access.
func (s *Server) listAPIKeyRespondents(w http.ResponseWriter, r *http.Request) {
	key, ok := s.ensureAPIKeyAccess(w, r, r.PathValue("keyId"))
	if !ok {
		return
	}
	list, err := s.st.ListAPIKeyAllowedRespondents(r.Context(), key.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"respondents": list})
}

// addAPIKeyRespondent adds one respondent to the allowed list.
func (s *Server) addAPIKeyRespondent(w http.ResponseWriter, r *http.Request) {
	key, ok := s.ensureAPIKeyAccess(w, r, r.PathValue("keyId"))
	if !ok {
		return
	}
	var in struct {
		RespondentID string `json:"respondentId"`
	}
	if err := decodeJSON(r, &in); err != nil || in.RespondentID == "" {
		writeErr(w, http.StatusBadRequest, "respondentId is required")
		return
	}
	ar, err := s.st.AddAPIKeyAllowedRespondent(r.Context(), key.ID, in.RespondentID)
	if errors.Is(err, store.ErrNotFound) {
		// ON CONFLICT DO NOTHING — the respondent is already on the list, which is not a failure.
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to add respondent")
		return
	}
	writeJSON(w, http.StatusCreated, ar)
}

// removeAPIKeyRespondent removes one respondent from the allowed list.
func (s *Server) removeAPIKeyRespondent(w http.ResponseWriter, r *http.Request) {
	ar, err := s.st.GetAPIKeyAllowedRespondentByID(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "data not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
		return
	}
	if _, ok := s.ensureAPIKeyAccess(w, r, ar.PermissionID); !ok {
		return
	}
	if err := s.st.RemoveAPIKeyAllowedRespondent(r.Context(), ar.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to remove respondent")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// listAPIKeyLogs returns the API call history for one key.
func (s *Server) listAPIKeyLogs(w http.ResponseWriter, r *http.Request) {
	key, ok := s.ensureAPIKeyAccess(w, r, r.PathValue("keyId"))
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	logs, err := s.st.ListAPIAccessLogs(r.Context(), key.ID, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"logs": logs})
}

// listActivityLogs shows the admin action history. Without a form parameter, only a
// superadmin may see the whole system; with ?formId=, the form's owning admin may
// see the trail for their own form.
func (s *Server) listActivityLogs(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	formID := r.URL.Query().Get("formId")

	if formID == "" {
		if u.Role != "superadmin" {
			writeErr(w, http.StatusForbidden, "access denied")
			return
		}
	} else if _, ok := s.ensureFormAccess(w, r, formID); !ok {
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	logs, total, err := s.st.ListActivityLogs(r.Context(), formID, limit, offset)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"logs": logs, "total": total})
}

// ensureAPIKeyAccess confirms the key exists and that the caller may manage its form.
func (s *Server) ensureAPIKeyAccess(w http.ResponseWriter, r *http.Request, keyID string) (*models.FormAPIKey, bool) {
	k, err := s.st.GetAPIKeyByID(r.Context(), keyID)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "API key not found")
		return nil, false
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
		return nil, false
	}
	if _, ok := s.ensureFormAccess(w, r, k.FormID); !ok {
		return nil, false
	}
	return k, true
}
