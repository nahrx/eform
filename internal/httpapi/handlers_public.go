package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/bpskaltim/eform-backend/internal/auth"
	"github.com/bpskaltim/eform-backend/internal/store"
)

// resolveShare memvalidasi token: aktif, belum kedaluwarsa, dan (jika perlu) password cocok.
func (s *Server) resolveShare(w http.ResponseWriter, r *http.Request) (formID, shareID string, allowResponses bool, ok bool) {
	token := r.PathValue("token")
	sh, err := s.st.GetShareByToken(r.Context(), token)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "tautan tidak ditemukan")
		return "", "", false, false
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "kesalahan server")
		return "", "", false, false
	}
	if !sh.IsActive {
		writeErr(w, http.StatusGone, "tautan sudah dinonaktifkan")
		return "", "", false, false
	}
	if sh.ExpiresAt != nil && time.Now().After(*sh.ExpiresAt) {
		writeErr(w, http.StatusGone, "tautan sudah kedaluwarsa")
		return "", "", false, false
	}
	if sh.PasswordHash != nil {
		pw := r.Header.Get("X-Share-Password")
		if pw == "" {
			pw = r.URL.Query().Get("password")
		}
		if !auth.CheckPassword(*sh.PasswordHash, pw) {
			writeErr(w, http.StatusUnauthorized, "password tautan salah")
			return "", "", false, false
		}
	}
	return sh.FormID, sh.ID, sh.AllowResponses, true
}

// GET /api/public/forms/{token} — akses skema kuesioner secara publik (tidak perlu login).
func (s *Server) publicGetForm(w http.ResponseWriter, r *http.Request) {
	formID, shareID, allowResponses, ok := s.resolveShare(w, r)
	if !ok {
		return
	}
	f, err := s.st.GetForm(r.Context(), formID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "kuesioner tidak ditemukan")
		return
	}
	if f.Status != "published" {
		writeErr(w, http.StatusForbidden, "kuesioner belum dipublikasikan")
		return
	}
	s.st.IncrementShareView(r.Context(), shareID)

	// Sertakan flag apakah Google OAuth dikonfigurasi agar frontend tahu
	writeJSON(w, http.StatusOK, map[string]any{
		"id":             f.ID,
		"title":          f.Title,
		"description":    f.Description,
		"version":        f.Version,
		"schema":         f.Schema,
		"allowResponses": allowResponses,
		"requireAuth":    true, // selalu true: login Google wajib untuk mengisi
		"googleEnabled":  s.cfg.GoogleClientID != "",
	})
}

// GET /api/public/me — info respondent yang sedang login.
func (s *Server) respondentMe(w http.ResponseWriter, r *http.Request) {
	rc := respondentFrom(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"id":      rc.RespondentID,
		"email":   rc.Email,
		"name":    rc.Name,
		"picture": rc.Picture,
	})
}

// GET /api/public/forms/{token}/my-response — jawaban terakhir respondent untuk form ini.
func (s *Server) myResponse(w http.ResponseWriter, r *http.Request) {
	rc := respondentFrom(r.Context())
	formID, _, _, ok := s.resolveShare(w, r)
	if !ok {
		return
	}
	resp, err := s.st.GetResponseByFormAndRespondent(r.Context(), formID, rc.RespondentID)
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusOK, nil) // belum pernah mengisi
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "kesalahan server")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// POST /api/public/forms/{token}/responses — kirim atau perbarui jawaban.
// Memerlukan JWT respondent (login Google).
func (s *Server) publicSubmit(w http.ResponseWriter, r *http.Request) {
	formID, shareID, allowResponses, ok := s.resolveShare(w, r)
	if !ok {
		return
	}
	if !allowResponses {
		writeErr(w, http.StatusForbidden, "tautan ini tidak menerima jawaban")
		return
	}
	rc := respondentFrom(r.Context())

	var in struct {
		Answers json.RawMessage `json:"answers"`
	}
	if err := decodeJSON(r, &in); err != nil || len(in.Answers) == 0 {
		writeErr(w, http.StatusBadRequest, "jawaban kosong atau format salah")
		return
	}
	meta := map[string]any{
		"ip":         clientIP(r),
		"userAgent":  r.UserAgent(),
		"receivedAt": time.Now().Format(time.RFC3339),
		"email":      rc.Email,
		"name":       rc.Name,
	}
	metaJSON, _ := json.Marshal(meta)
	sid := shareID
	resp, err := s.st.UpsertResponse(r.Context(), formID, &sid, rc.RespondentID, in.Answers, metaJSON)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal menyimpan jawaban")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": resp.ID, "submittedAt": resp.SubmittedAt})
}

// GET /api/wilayah?prov=&kab=&kec=
// Mengembalikan daftar wilayah anak dari parameter paling spesifik yang diberikan:
//   - hanya prov  → daftar kabupaten/kota di bawah provinsi tersebut
//   - prov + kab  → daftar kecamatan di bawah kabupaten tersebut
//   - prov + kab + kec → daftar desa/kelurahan di bawah kecamatan tersebut
//
// Nilai parameter adalah kode_wilayah (misal "64", "6401", "6401010").
// Endpoint ini tidak memerlukan autentikasi.
func (s *Server) wilayahList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	prov := q.Get("prov")
	kab  := q.Get("kab")
	kec  := q.Get("kec")

	// Gunakan parameter paling spesifik sebagai kode_parent
	var parent string
	switch {
	case kec != "":
		parent = kec
	case kab != "":
		parent = kab
	case prov != "":
		parent = prov
	default:
		// Tidak ada parameter → kembalikan semua provinsi
	}

	items, err := s.st.GetWilayahByParent(r.Context(), parent)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "kesalahan server")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	return r.RemoteAddr
}
