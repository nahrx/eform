package httpapi

import (
	"encoding/csv"
	"errors"
	"net/http"
	"strconv"

	"github.com/nahrx/eform/internal/models"
	"github.com/nahrx/eform/internal/store"
)

/* The endpoints external systems call (/api/v1), authenticated with an API key via
   apiKeyMW — not a JWT. Everything here is read-only: there is no path to submit or
   modify a response.

   Every data restriction (selected respondents, field-value filters, column masking)
   is applied in the store layer through store.APIKeyScope, so a handler here cannot
   "forget" to filter. All that remains here is hiding the respondent's identity,
   because that genuinely is part of the output shape. */

const apiMaxLimit = 500

// apiScope takes the API key from the context and confirms the formId in the URL really
// belongs to that key. A mismatch answers 404 rather than 403, so a key cannot be used
// to map out other forms in the system.
func (s *Server) apiScope(w http.ResponseWriter, r *http.Request) (*models.FormAPIKey, store.ResponseScope, bool) {
	key := apiKeyFromContext(r.Context())
	if key == nil {
		writeErr(w, http.StatusUnauthorized, "invalid API key")
		return nil, store.ResponseScope{}, false
	}
	if formID := r.PathValue("formId"); formID != "" && formID != key.FormID {
		s.logAPIAccess(r, key, key.KeyPrefix, s.clientIP(r), http.StatusNotFound, 0, "formId is outside this key's scope")
		writeErr(w, http.StatusNotFound, "form not found")
		return nil, store.ResponseScope{}, false
	}
	return key, store.APIKeyScope(key), true
}

// apiPresent prepares one response for output. If the key is not entitled to see the
// respondent's identity, respondentId and meta (name/email/IP) are stripped.
func apiPresent(rr models.Response, includeRespondent bool) models.Response {
	if !includeRespondent {
		rr.RespondentID = nil
		rr.Meta = nil
	}
	return rr
}

// apiMe returns metadata about the key in use — so a client can confirm its
// configuration is right without pulling any data.
func (s *Server) apiMe(w http.ResponseWriter, r *http.Request) {
	key, _, ok := s.apiScope(w, r)
	if !ok {
		return
	}
	form, err := s.st.GetForm(r.Context(), key.FormID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
		return
	}
	out := map[string]any{
		"formId":            key.FormID,
		"formTitle":         form.Title,
		"label":             key.Label,
		"keyPrefix":         key.KeyPrefix,
		"respondentAccess":  key.RespondentAccess,
		"visibleFields":     key.VisibleFields,
		"includeRespondent": key.IncludeRespondent,
		"rateLimitPerMin":   key.RateLimitPerMin,
		"expiresAt":         key.ExpiresAt,
	}
	s.logAPIAccess(r, key, key.KeyPrefix, s.clientIP(r), http.StatusOK, 0, "")
	writeJSON(w, http.StatusOK, out)
}

// apiListResponses returns the responses within the key's scope, paginated.
func (s *Server) apiListResponses(w http.ResponseWriter, r *http.Request) {
	key, scope, ok := s.apiScope(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	if limit <= 0 {
		limit = 50
	}
	if limit > apiMaxLimit {
		limit = apiMaxLimit
	}
	// Drafts are never included: that is enforced by the scope (ResponseScope.clauses), not
	// by a filter here, so it holds for every path including the CSV export.
	f := parseResponseFilter(q)

	rows, err := s.st.ListScopedResponses(r.Context(), scope, f, limit, offset)
	if err != nil {
		s.logAPIAccess(r, key, key.KeyPrefix, s.clientIP(r), http.StatusInternalServerError, 0, err.Error())
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
		return
	}
	total, err := s.st.CountScopedResponses(r.Context(), scope, f)
	if err != nil {
		s.logAPIAccess(r, key, key.KeyPrefix, s.clientIP(r), http.StatusInternalServerError, 0, err.Error())
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
		return
	}

	data := make([]models.Response, 0, len(rows))
	for _, rr := range rows {
		presented := apiPresent(rr, key.IncludeRespondent)
		presented.Answers = s.signAnswerUploads(presented.Answers)
		data = append(data, presented)
	}
	s.logAPIAccess(r, key, key.KeyPrefix, s.clientIP(r), http.StatusOK, len(data), "")
	writeJSON(w, http.StatusOK, map[string]any{
		"data":   data,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// apiGetResponse returns a single response within the key's scope.
func (s *Server) apiGetResponse(w http.ResponseWriter, r *http.Request) {
	key, scope, ok := s.apiScope(w, r)
	if !ok {
		return
	}
	rr, err := s.st.GetScopedResponseByID(r.Context(), scope, r.PathValue("responseId"))
	if errors.Is(err, store.ErrNotFound) {
		s.logAPIAccess(r, key, key.KeyPrefix, s.clientIP(r), http.StatusNotFound, 0, "response is out of scope")
		writeErr(w, http.StatusNotFound, "response not found")
		return
	}
	if err != nil {
		s.logAPIAccess(r, key, key.KeyPrefix, s.clientIP(r), http.StatusInternalServerError, 0, err.Error())
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
		return
	}
	s.logAPIAccess(r, key, key.KeyPrefix, s.clientIP(r), http.StatusOK, 1, "")
	presented := apiPresent(*rr, key.IncludeRespondent)
	writeJSON(w, http.StatusOK, s.signResponse(&presented))
}

// apiExportResponses streams every response within the key's scope as CSV.
func (s *Server) apiExportResponses(w http.ResponseWriter, r *http.Request) {
	key, scope, ok := s.apiScope(w, r)
	if !ok {
		return
	}
	cols, err := s.st.GetFormAnswerColumns(r.Context(), key.FormID)
	if err != nil {
		s.logAPIAccess(r, key, key.KeyPrefix, s.clientIP(r), http.StatusInternalServerError, 0, err.Error())
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
		return
	}
	// Columns outside visibleFields must not appear as headers even when empty —
	// the field name is itself information.
	if len(key.VisibleFields) > 0 {
		allowed := make(map[string]bool, len(key.VisibleFields))
		for _, f := range key.VisibleFields {
			allowed[f] = true
		}
		filtered := make([]string, 0, len(cols))
		for _, c := range cols {
			if allowed[c] {
				filtered = append(filtered, c)
			}
		}
		cols = filtered
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\"responses-"+key.FormID+".csv\"")
	cw := csv.NewWriter(w)
	_ = cw.Write(append(csvBaseHeader(key.IncludeRespondent), cols...))
	n := 0
	_ = s.st.ForEachScopedResponse(r.Context(), scope, func(rr models.Response) error {
		writeCSVRow(cw, rr, cols, key.IncludeRespondent)
		n++
		return nil
	})
	s.logAPIAccess(r, key, key.KeyPrefix, s.clientIP(r), http.StatusOK, n, "")
}
