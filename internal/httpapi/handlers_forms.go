package httpapi

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bpskaltim/eform-backend/internal/auth"
	"github.com/bpskaltim/eform-backend/internal/models"
	"github.com/bpskaltim/eform-backend/internal/store"
)

func (s *Server) listForms(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeErr(w, http.StatusUnauthorized, "perlu login")
		return
	}
	var (
		forms []models.Form
		err   error
	)
	switch u.Role {
	case "superadmin":
		forms, err = s.st.ListForms(r.Context())
	case "admin":
		forms, err = s.st.ListFormsByOwner(r.Context(), u.Subject)
	default:
		// Editor tidak punya akses ke builder (lihat ensureFormAccess) — dia memakai
		// endpoint terpisah /api/editor/my-forms untuk melihat kuesioner yang ditugaskan.
		writeErr(w, http.StatusForbidden, "akses ditolak")
		return
	}
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
	u := userFrom(r.Context())
	if u == nil {
		writeErr(w, http.StatusUnauthorized, "perlu login")
		return
	}
	if u.Role != "superadmin" && u.Role != "admin" {
		writeErr(w, http.StatusForbidden, "akses ditolak")
		return
	}

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
	uid := u.Subject
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
	f, ok := s.ensureFormAccess(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, f)
}

func (s *Server) updateForm(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.ensureFormAccess(w, r, r.PathValue("id")); !ok {
		return
	}
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
	f, ok := s.ensureFormAccess(w, r, id)
	if !ok {
		return
	}
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
	s.audit(r, "form.delete", "form", id, f.Title, "", "")
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) publishForm(w http.ResponseWriter, r *http.Request) {
	f, ok := s.ensureFormAccess(w, r, r.PathValue("id"))
	if !ok {
		return
	}
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
	s.audit(r, "form.status", "form", f.ID, f.Title, f.ID, "status → "+status)
	writeJSON(w, http.StatusOK, map[string]string{"status": status})
}

/* ---------------- shares ---------------- */

func (s *Server) createShare(w http.ResponseWriter, r *http.Request) {
	formID := r.PathValue("id")
	if _, ok := s.ensureFormAccess(w, r, formID); !ok {
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
	s.audit(r, "share.create", "share", sh.ID, in.Label, formID, "mode="+in.AccessMode)
	writeJSON(w, http.StatusCreated, s.shareWithURL(sh))
}

func (s *Server) listShares(w http.ResponseWriter, r *http.Request) {
	formID := r.PathValue("id")
	if _, ok := s.ensureFormAccess(w, r, formID); !ok {
		return
	}
	shares, err := s.st.ListSharesByForm(r.Context(), formID)
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
	if _, ok := s.ensureShareAccess(w, r, r.PathValue("id")); !ok {
		return
	}
	err := s.st.RevokeShare(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "share tidak ditemukan")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mencabut share")
		return
	}
	s.audit(r, "share.revoke", "share", r.PathValue("id"), "", "", "")
	writeJSON(w, http.StatusOK, map[string]bool{"revoked": true})
}

func (s *Server) reactivateShare(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.ensureShareAccess(w, r, r.PathValue("id")); !ok {
		return
	}
	err := s.st.ReactivateShare(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "share tidak ditemukan")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengaktifkan share")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"reactivated": true})
}

func (s *Server) updateShare(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.ensureShareAccess(w, r, r.PathValue("id")); !ok {
		return
	}
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
		"expiresAt":   sh.ExpiresAt, "viewCount": sh.ViewCount, "createdAt": sh.CreatedAt,
		"shareUrl": s.cfg.PublicBaseURL + "/f/" + sh.Token,
		"apiUrl":   s.cfg.PublicBaseURL + "/api/public/forms/" + sh.Token,
	}
}

func (s *Server) deleteSharePermanent(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.ensureShareAccess(w, r, r.PathValue("id")); !ok {
		return
	}
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
	if _, ok := s.ensureShareAccess(w, r, r.PathValue("id")); !ok {
		return
	}
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
	if _, ok := s.ensureShareAccess(w, r, r.PathValue("id")); !ok {
		return
	}
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
	if _, ok := s.ensureShareEmailAccess(w, r, r.PathValue("id")); !ok {
		return
	}
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

// splitFilterValues memecah nilai filter "fea_" (checkbox/multiselect, dipisah koma dari
// frontend) menjadi slice bersih, dibatasi jumlahnya agar query tidak disalahgunakan.
func splitFilterValues(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
		if len(out) >= 20 {
			break
		}
	}
	return out
}

// applyRangeFilter menangani parameter "fer_<field>_min"/"fer_<field>_max" (filter rentang
// angka, untuk field number/integer/decimal/currency/range/rating). Mengembalikan true bila
// key cocok pola ini (dipakai/tidaknya nilai tetap dihitung "sudah ditangani").
func applyRangeFilter(f *store.ResponseFilter, key, val string) bool {
	if !strings.HasPrefix(key, "fer_") {
		return false
	}
	rest := key[4:]
	var fieldName string
	bound := 0
	if strings.HasSuffix(rest, "_min") {
		fieldName = strings.TrimSuffix(rest, "_min")
	} else if strings.HasSuffix(rest, "_max") {
		fieldName = strings.TrimSuffix(rest, "_max")
		bound = 1
	} else {
		return true
	}
	if f.FieldRangeFilters == nil {
		f.FieldRangeFilters = make(map[string][2]string)
	}
	if _, exists := f.FieldRangeFilters[fieldName]; !exists && len(f.FieldRangeFilters) >= 10 {
		return true
	}
	cur := f.FieldRangeFilters[fieldName]
	cur[bound] = val
	f.FieldRangeFilters[fieldName] = cur
	return true
}

func (s *Server) listResponses(w http.ResponseWriter, r *http.Request) {
	if !s.ensureResultAccess(w, r) {
		return
	}
	formID := r.PathValue("id")
	if _, ok := s.ensureFormAccess(w, r, formID); !ok {
		return
	}
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
	// Parse filter per-field schema:
	//   "f_namafield=nilai"           → ILIKE (teks bebas)
	//   "fe_namafield=nilai"          → exact match (dropdown/radio/tanggal)
	//   "fea_namafield=a,b,c"         → cocok salah satu (checkbox/multiselect)
	//   "fer_namafield_min/_max=nilai"→ rentang angka (number/integer/decimal/dst)
	for key, vals := range q {
		if len(vals) == 0 || strings.TrimSpace(vals[0]) == "" {
			continue
		}
		val := strings.TrimSpace(vals[0])
		if applyRangeFilter(&f, key, val) {
			// sudah ditangani
		} else if strings.HasPrefix(key, "fea_") {
			fieldName := key[4:]
			if f.FieldAnyFilters == nil {
				f.FieldAnyFilters = make(map[string][]string)
			}
			if len(f.FieldAnyFilters) < 10 {
				f.FieldAnyFilters[fieldName] = splitFilterValues(val)
			}
		} else if strings.HasPrefix(key, "fe_") {
			fieldName := key[3:]
			if f.FieldExactFilters == nil {
				f.FieldExactFilters = make(map[string]string)
			}
			if len(f.FieldExactFilters) < 10 {
				f.FieldExactFilters[fieldName] = val
			}
		} else if strings.HasPrefix(key, "f_") {
			fieldName := key[2:]
			if f.FieldFilters == nil {
				f.FieldFilters = make(map[string]string)
			}
			if len(f.FieldFilters) < 10 {
				f.FieldFilters[fieldName] = val
			}
		}
	}
	resp, err := s.st.ListAllResponsesByForm(r.Context(), formID, f, limit, offset)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	count, err := s.st.CountAllResponsesByForm(r.Context(), formID, f)
	if err != nil {
		count = int64(len(resp))
	}
	if resp == nil {
		resp = []models.Response{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"responses": s.signResponses(resp), "total": count})
}

func (s *Server) getResponseDetail(w http.ResponseWriter, r *http.Request) {
	if !s.ensureResultAccess(w, r) {
		return
	}
	formID := r.PathValue("id")
	if _, ok := s.ensureFormAccess(w, r, formID); !ok {
		return
	}
	resp, err := s.st.GetResponseByID(r.Context(), r.PathValue("responseId"))
	if errors.Is(err, store.ErrNotFound) || (err == nil && resp.FormID != formID) {
		writeErr(w, http.StatusNotFound, "jawaban tidak ditemukan")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	writeJSON(w, http.StatusOK, s.signResponse(resp))
}

func (s *Server) deleteResponse(w http.ResponseWriter, r *http.Request) {
	formID := r.PathValue("id")
	if _, ok := s.ensureFormAccess(w, r, formID); !ok {
		return
	}
	responseID := r.PathValue("responseId")
	if err := s.st.DeleteResponseByID(r.Context(), formID, responseID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "jawaban tidak ditemukan")
			return
		}
		writeErr(w, http.StatusInternalServerError, "gagal menghapus jawaban")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// updateResponse mengubah jawaban suatu respons — hanya superadmin/admin (lihat router).
func (s *Server) updateResponse(w http.ResponseWriter, r *http.Request) {
	formID := r.PathValue("id")
	if _, ok := s.ensureFormAccess(w, r, formID); !ok {
		return
	}
	responseID := r.PathValue("responseId")

	var in struct {
		Answers json.RawMessage `json:"answers"`
		Status  string          `json:"status"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "format permintaan salah")
		return
	}
	if len(in.Answers) == 0 {
		writeErr(w, http.StatusBadRequest, "answers wajib diisi")
		return
	}
	if in.Status != "draft" && in.Status != "submitted" {
		writeErr(w, http.StatusBadRequest, "status tidak valid")
		return
	}
	if err := s.st.UpdateResponseAnswers(r.Context(), formID, responseID, in.Answers); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "respons tidak ditemukan")
			return
		}
		writeErr(w, http.StatusInternalServerError, "gagal menyimpan")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// suggestedFieldValues mengembalikan nilai unik yang benar-benar pernah terisi untuk satu
// variabel — dipakai sebagai saran (datalist) saat mengisi nilai filter di dialog
// tambah/kelola akses massal, khusus untuk variabel tanpa daftar pilihan tetap.
func (s *Server) suggestedFieldValues(w http.ResponseWriter, r *http.Request) {
	formID := r.PathValue("id")
	if _, ok := s.ensureFormAccess(w, r, formID); !ok {
		return
	}
	fieldName := r.PathValue("fieldName")
	if fieldName == "" {
		writeErr(w, http.StatusBadRequest, "nama variabel wajib diisi")
		return
	}
	values, err := s.st.GetDistinctFieldValues(r.Context(), formID, fieldName)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"values": values})
}

func (s *Server) exportResponses(w http.ResponseWriter, r *http.Request) {
	if !s.ensureResultAccess(w, r) {
		return
	}
	formID := r.PathValue("id")
	if _, ok := s.ensureFormAccess(w, r, formID); !ok {
		return
	}
	// 1. Ambil nama kolom via query ringan (tanpa memuat semua baris)
	cols, err := s.st.GetFormAnswerColumns(r.Context(), formID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	// 2. Set header sebelum streaming dimulai
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\"responses-"+formID+".csv\"")
	cw := csv.NewWriter(w)
	_ = cw.Write(append(csvBaseHeader(true), cols...))
	// 3. Stream setiap baris langsung ke CSV tanpa buffering di memori
	n := 0
	_ = s.st.ForEachResponseByForm(r.Context(), formID, func(rr models.Response) error {
		writeCSVRow(cw, rr, cols, true)
		n++
		return nil
	})
	// Ekspor CSV adalah jalur keluarnya data terbesar — selalu dicatat.
	s.audit(r, "export.csv", "form", formID, "", formID, fmt.Sprintf("%d baris (admin)", n))
}

// parseResponseFilter membaca filter & sort daftar jawaban dari query string.
// Konvensinya: f_<field> (ILIKE), fe_<field> (sama persis), fea_<field> (salah satu dari
// daftar), plus filter rentang yang ditangani applyRangeFilter.
// Dipakai bersama oleh daftar jawaban admin, viewer, editor, dan API key.
func parseResponseFilter(q url.Values) store.ResponseFilter {
	f := store.ResponseFilter{
		Status:  q.Get("status"),
		ShareID: q.Get("shareId"),
		Search:  strings.TrimSpace(q.Get("search")),
		SortBy:  q.Get("sortBy"),
		SortDir: q.Get("sortDir"),
	}
	// Tiap jenis filter dibatasi 10 entri supaya query string tidak bisa dipakai
	// menyusun WHERE raksasa.
	const maxPerKind = 10
	for key, vals := range q {
		if len(vals) == 0 {
			continue
		}
		val := strings.TrimSpace(vals[0])
		if val == "" {
			continue
		}
		switch {
		case applyRangeFilter(&f, key, val):
			// sudah ditangani
		case strings.HasPrefix(key, "fea_"):
			if f.FieldAnyFilters == nil {
				f.FieldAnyFilters = make(map[string][]string)
			}
			if len(f.FieldAnyFilters) < maxPerKind {
				f.FieldAnyFilters[strings.TrimPrefix(key, "fea_")] = splitFilterValues(val)
			}
		case strings.HasPrefix(key, "fe_"):
			if f.FieldExactFilters == nil {
				f.FieldExactFilters = make(map[string]string)
			}
			if len(f.FieldExactFilters) < maxPerKind {
				f.FieldExactFilters[strings.TrimPrefix(key, "fe_")] = val
			}
		case strings.HasPrefix(key, "f_"):
			if f.FieldFilters == nil {
				f.FieldFilters = make(map[string]string)
			}
			if len(f.FieldFilters) < maxPerKind {
				f.FieldFilters[strings.TrimPrefix(key, "f_")] = val
			}
		}
	}
	return f
}

// csvBaseHeader adalah kolom tetap sebelum kolom jawaban. includeRespondent=false
// dipakai jalur API key yang tidak boleh membocorkan identitas responden.
func csvBaseHeader(includeRespondent bool) []string {
	if includeRespondent {
		return []string{"id", "respondent_id", "nama", "email", "status", "waktu_kirim"}
	}
	return []string{"id", "status", "waktu_kirim"}
}

// writeCSVRow menulis satu baris respons ke csv.Writer sesuai daftar kolom jawaban yang diberikan.
// Dipakai bersama oleh ekspor CSV admin, viewer, editor, dan API key.
func writeCSVRow(cw *csv.Writer, rr models.Response, cols []string, includeRespondent bool) {
	a := map[string]any{}
	_ = json.Unmarshal(rr.Answers, &a)
	waktu := ""
	if !rr.SubmittedAt.IsZero() {
		waktu = rr.SubmittedAt.Format(time.RFC3339)
	}
	var row []string
	if includeRespondent {
		m := map[string]any{}
		_ = json.Unmarshal(rr.Meta, &m)
		respondentID := ""
		if rr.RespondentID != nil {
			respondentID = *rr.RespondentID
		}
		row = []string{rr.ID, respondentID, toStr(m["name"]), toStr(m["email"]), rr.Status, waktu}
	} else {
		row = []string{rr.ID, rr.Status, waktu}
	}
	for _, c := range cols {
		v := a[c]
		if sv, ok := v.(string); ok && len(sv) > 200 && (strings.HasPrefix(sv, "data:image") || strings.HasPrefix(sv, "data:application")) {
			row = append(row, "")
		} else {
			row = append(row, toStr(v))
		}
	}
	_ = cw.Write(row)
	cw.Flush()
}

// ensureResultAccess membatasi akses hasil/jawaban: editor tidak boleh mengakses.
func (s *Server) ensureResultAccess(w http.ResponseWriter, r *http.Request) bool {
	u := userFrom(r.Context())
	if u == nil {
		writeErr(w, http.StatusUnauthorized, "perlu login")
		return false
	}
	if u.Role == "editor" {
		writeErr(w, http.StatusForbidden, "editor tidak memiliki akses ke hasil")
		return false
	}
	return true
}

// saveFormColumnConfig menyimpan konfigurasi kolom tabel jawaban yang dipilih admin/superadmin.
func (s *Server) saveFormColumnConfig(w http.ResponseWriter, r *http.Request) {
	formID := r.PathValue("id")
	if _, ok := s.ensureFormAccess(w, r, formID); !ok {
		return
	}
	var in struct {
		Cols json.RawMessage `json:"cols"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "format permintaan salah")
		return
	}
	if err := s.st.SaveFormColumnConfig(r.Context(), formID, in.Cols); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "kuesioner tidak ditemukan")
			return
		}
		writeErr(w, http.StatusInternalServerError, "gagal menyimpan konfigurasi kolom")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) ensureFormAccess(w http.ResponseWriter, r *http.Request, formID string) (*models.Form, bool) {
	u := userFrom(r.Context())
	if u == nil {
		writeErr(w, http.StatusUnauthorized, "perlu login")
		return nil, false
	}

	f, err := s.st.GetForm(r.Context(), formID)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "kuesioner tidak ditemukan")
		return nil, false
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return nil, false
	}

	switch u.Role {
	case "superadmin":
		return f, true
	case "admin":
		if f.OwnerID == nil || *f.OwnerID != u.Subject {
			writeErr(w, http.StatusForbidden, "akses ditolak")
			return nil, false
		}
		return f, true
	default:
		// Editor (seperti viewer) tidak punya akses ke builder/skema kuesioner —
		// dia hanya boleh melihat & mengedit jawaban lewat endpoint /api/editor/...
		// (editorGetForm, editorGetResponse, editorUpdateResponse, dst).
		writeErr(w, http.StatusForbidden, "akses ditolak")
		return nil, false
	}
}

func (s *Server) ensureShareAccess(w http.ResponseWriter, r *http.Request, shareID string) (*models.Share, bool) {
	sh, err := s.st.GetShareByID(r.Context(), shareID)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "share tidak ditemukan")
		return nil, false
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return nil, false
	}
	if _, ok := s.ensureFormAccess(w, r, sh.FormID); !ok {
		return nil, false
	}
	return sh, true
}

func (s *Server) ensureShareEmailAccess(w http.ResponseWriter, r *http.Request, shareEmailID string) (*models.ShareAllowedEmail, bool) {
	e, err := s.st.GetShareAllowedEmailByID(r.Context(), shareEmailID)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "email tidak ditemukan")
		return nil, false
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return nil, false
	}
	if _, ok := s.ensureShareAccess(w, r, e.ShareID); !ok {
		return nil, false
	}
	return e, true
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
