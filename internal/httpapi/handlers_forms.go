package httpapi

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bpskaltim/eform-backend/internal/auth"
	"github.com/bpskaltim/eform-backend/internal/models"
	"github.com/bpskaltim/eform-backend/internal/store"
)

func (s *Server) listForms(w http.ResponseWriter, r *http.Request) {
	forms, err := s.st.ListForms(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil daftar")
		return
	}
	if forms == nil {
		forms = []models.Form{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"forms": forms})
}

func (s *Server) createForm(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title       string          `json:"title"`
		Description string          `json:"description"`
		Schema      json.RawMessage `json:"schema"`
		Version     string          `json:"version"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "format permintaan salah")
		return
	}
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		in.Title = "Kuesioner Baru"
	}
	if in.Version == "" {
		in.Version = "1.0.0"
	}
	slug := s.uniqueSlug(r, slugify(in.Title))
	uid := userFrom(r.Context()).Subject
	f, err := s.st.CreateForm(r.Context(), slug, in.Title, in.Description, in.Schema, in.Version, &uid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal menyimpan")
		return
	}
	writeJSON(w, http.StatusCreated, f)
}

func (s *Server) uniqueSlug(r *http.Request, base string) string {
	slug := base
	for i := 0; i < 5; i++ {
		exists, err := s.st.SlugExists(r.Context(), slug)
		if err != nil || !exists {
			return slug
		}
		slug = base + "-" + randToken(2)
	}
	return base + "-" + randToken(4)
}

func (s *Server) getForm(w http.ResponseWriter, r *http.Request) {
	f, err := s.st.GetForm(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "kuesioner tidak ditemukan")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	writeJSON(w, http.StatusOK, f)
}

func (s *Server) updateForm(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title       string          `json:"title"`
		Description string          `json:"description"`
		Schema      json.RawMessage `json:"schema"`
		Version     string          `json:"version"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "format permintaan salah")
		return
	}
	if in.Version == "" {
		in.Version = "1.0.0"
	}
	f, err := s.st.UpdateForm(r.Context(), r.PathValue("id"), in.Title, in.Description, in.Schema, in.Version)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "kuesioner tidak ditemukan")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal menyimpan")
		return
	}
	writeJSON(w, http.StatusOK, f)
}

func (s *Server) deleteForm(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	count, err := s.st.CountAllResponsesByForm(r.Context(), id, store.ResponseFilter{})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal memeriksa data jawaban")
		return
	}
	if count > 0 {
		writeErr(w, http.StatusConflict, "kuesioner tidak dapat dihapus karena sudah memiliki jawaban")
		return
	}
	if err := s.st.DeleteForm(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "kuesioner tidak ditemukan")
		return
	} else if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal menghapus")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) publishForm(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Status string `json:"status"`
	}
	_ = decodeJSON(r, &in)
	status := in.Status
	if status == "" {
		status = "published"
	}
	if status != "draft" && status != "published" && status != "archived" {
		writeErr(w, http.StatusBadRequest, "status tidak valid")
		return
	}
	err := s.st.SetFormStatus(r.Context(), r.PathValue("id"), status)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "kuesioner tidak ditemukan")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal memperbarui status")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": status})
}

/* ---------------- shares ---------------- */

func (s *Server) createShare(w http.ResponseWriter, r *http.Request) {
	formID := r.PathValue("id")
	if _, err := s.st.GetForm(r.Context(), formID); errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "kuesioner tidak ditemukan")
		return
	}
	var in struct {
		Label          string `json:"label"`
		AllowResponses *bool  `json:"allowResponses"`
		MultiResponse  bool   `json:"multiResponse"`
		AccessMode     string `json:"accessMode"`
		Password       string `json:"password"`
		ExpiresAt      string `json:"expiresAt"`
	}
	_ = decodeJSON(r, &in)

	allow := true
	if in.AllowResponses != nil {
		allow = *in.AllowResponses
	}
	if in.AccessMode != "public" && in.AccessMode != "restricted" {
		in.AccessMode = "public"
	}
	var ph *string
	if in.Password != "" {
		h, err := auth.HashPassword(in.Password)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "gagal memproses password")
			return
		}
		ph = &h
	}
	var exp *time.Time
	if in.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, in.ExpiresAt)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "format expiresAt harus RFC3339")
			return
		}
		exp = &t
	}
	uid := userFrom(r.Context()).Subject
	token := randToken(12)
	sh, err := s.st.CreateShare(r.Context(), formID, token, in.Label, allow, in.MultiResponse, in.AccessMode, ph, exp, &uid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal membuat share")
		return
	}
	writeJSON(w, http.StatusCreated, s.shareWithURL(sh))
}

func (s *Server) listShares(w http.ResponseWriter, r *http.Request) {
	shares, err := s.st.ListSharesByForm(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	out := make([]map[string]any, 0, len(shares))
	for i := range shares {
		out = append(out, s.shareWithURL(&shares[i]))
	}
	writeJSON(w, http.StatusOK, map[string]any{"shares": out})
}

func (s *Server) revokeShare(w http.ResponseWriter, r *http.Request) {
	err := s.st.RevokeShare(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "share tidak ditemukan")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mencabut share")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"revoked": true})
}

func (s *Server) updateShare(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Label          string `json:"label"`
		AllowResponses *bool  `json:"allowResponses"`
		MultiResponse  bool   `json:"multiResponse"`
		AccessMode     string `json:"accessMode"`
		UpdatePassword bool   `json:"updatePassword"`
		Password       string `json:"password"` // "" + updatePassword=true → hapus password
		UpdateExpiry   bool   `json:"updateExpiry"`
		ExpiresAt      string `json:"expiresAt"` // "" + updateExpiry=true → hapus expiry
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "format salah")
		return
	}
	allow := true
	if in.AllowResponses != nil {
		allow = *in.AllowResponses
	}
	if in.AccessMode != "public" && in.AccessMode != "restricted" {
		in.AccessMode = "public"
	}
	var newPH *string
	if in.UpdatePassword && in.Password != "" {
		h, err := auth.HashPassword(in.Password)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "gagal memproses password")
			return
		}
		newPH = &h
	}
	// UpdatePassword=true & Password="" → newPH stays nil → menghapus password
	var exp *time.Time
	if in.UpdateExpiry && in.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, in.ExpiresAt)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "format expiresAt harus RFC3339")
			return
		}
		exp = &t
	}
	sh, err := s.st.UpdateShare(r.Context(), r.PathValue("id"), in.Label, allow, in.MultiResponse, in.AccessMode, in.UpdatePassword, newPH, in.UpdateExpiry, exp)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "share tidak ditemukan")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal memperbarui share")
		return
	}
	writeJSON(w, http.StatusOK, s.shareWithURL(sh))
}

func (s *Server) shareWithURL(sh *models.Share) map[string]any {
	return map[string]any{
		"id": sh.ID, "formId": sh.FormID, "token": sh.Token, "label": sh.Label,
		"isActive": sh.IsActive, "allowResponses": sh.AllowResponses,
		"multiResponse": sh.MultiResponse, "accessMode": sh.AccessMode,
		"hasPassword": sh.HasPassword,
		"expiresAt": sh.ExpiresAt, "viewCount": sh.ViewCount, "createdAt": sh.CreatedAt,
		"shareUrl": s.cfg.PublicBaseURL + "/f/" + sh.Token,
		"apiUrl":   s.cfg.PublicBaseURL + "/api/public/forms/" + sh.Token,
	}
}

func (s *Server) deleteSharePermanent(w http.ResponseWriter, r *http.Request) {
	err := s.st.DeleteShare(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "share tidak ditemukan atau masih aktif")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal menghapus share")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) listAllowedEmails(w http.ResponseWriter, r *http.Request) {
	emails, err := s.st.ListShareAllowedEmails(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	if emails == nil {
		emails = []models.ShareAllowedEmail{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"emails": emails})
}

func (s *Server) addAllowedEmail(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email string `json:"email"`
		Note  string `json:"note"`
	}
	if err := decodeJSON(r, &in); err != nil || strings.TrimSpace(in.Email) == "" {
		writeErr(w, http.StatusBadRequest, "email tidak boleh kosong")
		return
	}
	e, err := s.st.CreateShareAllowedEmail(r.Context(), r.PathValue("id"), strings.TrimSpace(strings.ToLower(in.Email)), in.Note)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal menambah email")
		return
	}
	writeJSON(w, http.StatusCreated, e)
}

func (s *Server) removeAllowedEmail(w http.ResponseWriter, r *http.Request) {
	err := s.st.DeleteShareAllowedEmail(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "email tidak ditemukan")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal menghapus email")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

/* ---------------- responses ---------------- */

func (s *Server) listResponses(w http.ResponseWriter, r *http.Request) {
	formID := r.PathValue("id")
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	f := store.ResponseFilter{
		Status:  q.Get("status"),
		ShareID: q.Get("shareId"),
		Search:  strings.TrimSpace(q.Get("search")),
		SortBy:  q.Get("sortBy"),
		SortDir: q.Get("sortDir"),
	}
	resp, err := s.st.ListAllResponsesByForm(r.Context(), formID, f, limit, offset)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	count, _ := s.st.CountAllResponsesByForm(r.Context(), formID, f)
	if resp == nil {
		resp = []models.Response{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"responses": resp, "total": count})
}

func (s *Server) getResponseDetail(w http.ResponseWriter, r *http.Request) {
	formID := r.PathValue("id")
	resp, err := s.st.GetResponseByID(r.Context(), r.PathValue("responseId"))
	if errors.Is(err, store.ErrNotFound) || (err == nil && resp.FormID != formID) {
		writeErr(w, http.StatusNotFound, "jawaban tidak ditemukan")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) exportResponses(w http.ResponseWriter, r *http.Request) {
	formID := r.PathValue("id")
	resp, err := s.st.ListResponsesByForm(r.Context(), formID, 1000, 0)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	// kumpulkan union kolom dari semua jawaban
	colSet := map[string]bool{}
	parsed := make([]map[string]any, len(resp))
	for i, rr := range resp {
		m := map[string]any{}
		_ = json.Unmarshal(rr.Answers, &m)
		parsed[i] = m
		for k := range m {
			colSet[k] = true
		}
	}
	cols := make([]string, 0, len(colSet))
	for k := range colSet {
		cols = append(cols, k)
	}
	sort.Strings(cols)

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\"responses-"+formID+".csv\"")
	cw := csv.NewWriter(w)
	header := append([]string{"id", "submitted_at"}, cols...)
	_ = cw.Write(header)
	for i, rr := range resp {
		row := []string{rr.ID, rr.SubmittedAt.Format(time.RFC3339)}
		for _, c := range cols {
			row = append(row, toStr(parsed[i][c]))
		}
		_ = cw.Write(row)
	}
	cw.Flush()
}

func toStr(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}
