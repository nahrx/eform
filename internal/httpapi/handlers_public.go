package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/bpskaltim/eform-backend/internal/auth"
	"github.com/bpskaltim/eform-backend/internal/models"
	"github.com/bpskaltim/eform-backend/internal/store"
)

// resolveShare memvalidasi token: aktif, belum kedaluwarsa, dan (jika perlu) password cocok.
func (s *Server) resolveShare(w http.ResponseWriter, r *http.Request) (*models.Share, bool) {
	token := r.PathValue("token")
	sh, err := s.st.GetShareByToken(r.Context(), token)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "tautan tidak ditemukan")
		return nil, false
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "kesalahan server")
		return nil, false
	}
	if !sh.IsActive {
		writeErr(w, http.StatusGone, "tautan sudah dinonaktifkan")
		return nil, false
	}
	if sh.ExpiresAt != nil && time.Now().After(*sh.ExpiresAt) {
		writeErr(w, http.StatusGone, "tautan sudah kedaluwarsa")
		return nil, false
	}
	if sh.PasswordHash != nil {
		pw := r.Header.Get("X-Share-Password")
		if pw == "" {
			pw = r.URL.Query().Get("password")
		}
		if !auth.CheckPassword(*sh.PasswordHash, pw) {
			writeErr(w, http.StatusUnauthorized, "password tautan salah")
			return nil, false
		}
	}
	return sh, true
}

// GET /api/public/forms/{token} — akses skema kuesioner secara publik (tidak perlu login).
func (s *Server) publicGetForm(w http.ResponseWriter, r *http.Request) {
	sh, ok := s.resolveShare(w, r)
	if !ok {
		return
	}
	f, err := s.st.GetForm(r.Context(), sh.FormID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "kuesioner tidak ditemukan")
		return
	}
	if f.Status != "published" {
		writeErr(w, http.StatusForbidden, "kuesioner belum dipublikasikan")
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
		writeErr(w, http.StatusInternalServerError, "kesalahan server")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// GET /api/public/forms/{token}/my-responses — semua jawaban respondent (untuk multi-response).
func (s *Server) myResponses(w http.ResponseWriter, r *http.Request) {
	rc := respondentFrom(r.Context())
	sh, ok := s.resolveShare(w, r)
	if !ok {
		return
	}
	resps, err := s.st.ListResponsesByFormAndRespondent(r.Context(), sh.FormID, rc.RespondentID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "kesalahan server")
		return
	}
	if resps == nil {
		resps = []models.Response{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"responses": resps})
}

// GET /api/public/forms/{token}/check-access — cek apakah email respondent diizinkan (restricted mode).
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
		writeErr(w, http.StatusInternalServerError, "kesalahan server")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"allowed": allowed})
}

// POST /api/public/forms/{token}/responses — kirim atau simpan draf jawaban.
// Body: {answers, draft?: bool, responseId?: string}
// Memerlukan JWT respondent (login Google).
func (s *Server) publicSubmit(w http.ResponseWriter, r *http.Request) {
	sh, ok := s.resolveShare(w, r)
	if !ok {
		return
	}
	if !sh.AllowResponses {
		writeErr(w, http.StatusForbidden, "tautan ini tidak menerima jawaban")
		return
	}
	rc := respondentFrom(r.Context())

	// Cek akses jika mode restricted
	if sh.AccessMode == "restricted" {
		allowed, err := s.st.IsEmailAllowed(r.Context(), sh.ID, rc.Email)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "kesalahan server")
			return
		}
		if !allowed {
			writeErr(w, http.StatusForbidden, "email Anda tidak terdaftar dalam daftar akses kuesioner ini")
			return
		}
	}

	var in struct {
		Answers    json.RawMessage `json:"answers"`
		Draft      bool            `json:"draft"`
		ResponseID string          `json:"responseId"`
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
	sid := sh.ID
	var resp *models.Response
	var err error

	if sh.MultiResponse {
		status := "submitted"
		if in.Draft {
			status = "draft"
		}
		if in.ResponseID != "" {
			// Perbarui draf yang ada (harus masih berstatus 'draft' dan milik respondent ini)
			resp, err = s.st.UpdateMultiResponseDraft(r.Context(), in.ResponseID, rc.RespondentID, sh.FormID, status, in.Answers, metaJSON)
			if errors.Is(err, store.ErrNotFound) {
				writeErr(w, http.StatusNotFound, "respons tidak ditemukan atau bukan milik Anda")
				return
			}
		} else {
			// Buat baris baru — pastikan tidak ada draf aktif yang belum diselesaikan
			if !in.Draft {
				hasDraft, chkErr := s.st.HasDraftResponse(r.Context(), sh.FormID, rc.RespondentID)
				if chkErr != nil {
					writeErr(w, http.StatusInternalServerError, "kesalahan server")
					return
				}
				if hasDraft {
					writeErr(w, http.StatusConflict, "Anda masih memiliki draf yang belum diselesaikan — lanjutkan atau batalkan draf tersebut terlebih dahulu")
					return
				}
			}
			resp, err = s.st.CreateMultiResponseRow(r.Context(), sh.FormID, &sid, rc.RespondentID, status, in.Answers, metaJSON)
		}
	} else {
		// Single-response: upsert (satu respons per respondent)
		resp, err = s.st.UpsertResponse(r.Context(), sh.FormID, &sid, rc.RespondentID, in.Answers, metaJSON)
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal menyimpan jawaban")
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
// Mengubah respons yang sudah dikirim kembali menjadi draf sehingga bisa diedit.
func (s *Server) unsubmitResponse(w http.ResponseWriter, r *http.Request) {
	rc := respondentFrom(r.Context())
	sh, ok := s.resolveShare(w, r)
	if !ok {
		return
	}
	if !sh.AllowResponses {
		writeErr(w, http.StatusForbidden, "tautan ini tidak menerima perubahan jawaban")
		return
	}
	responseID := r.PathValue("responseId")
	resp, err := s.st.UnsubmitResponse(r.Context(), responseID, rc.RespondentID, sh.FormID)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "respons tidak ditemukan atau bukan milik Anda")
		return
	}
	if err != nil {
		writeErr(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": resp.ID, "status": resp.Status})
}

// GET /api/public/forms/{token}/draft — ambil draf tersimpan di server.
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
		writeErr(w, http.StatusInternalServerError, "kesalahan server")
		return
	}
	writeJSON(w, http.StatusOK, draft)
}

// POST /api/public/forms/{token}/draft — simpan draf ke server (upsert).
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
		writeErr(w, http.StatusBadRequest, "format salah atau jawaban kosong")
		return
	}
	sid := sh.ID
	draft, err := s.st.UpsertDraft(r.Context(), sh.FormID, &sid, rc.RespondentID, in.Answers, in.CurPage)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal menyimpan draf")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": draft.ID, "savedAt": draft.SavedAt})
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
