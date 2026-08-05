package httpapi

import (
	"encoding/csv"
	"errors"
	"net/http"
	"strconv"

	"github.com/nahrx/eform/internal/models"
	"github.com/nahrx/eform/internal/store"
)

/* Endpoint yang dipakai sistem eksternal (/api/v1), diautentikasi dengan API key
   lewat apiKeyMW — bukan JWT. Semuanya read-only: tidak ada jalur untuk mengirim
   atau mengubah jawaban.

   Semua pembatasan data (responden terpilih, filter nilai variabel, masking kolom)
   diterapkan di layer store lewat store.APIKeyScope, jadi handler di sini tidak bisa
   "lupa" memfilter. Yang tersisa di sini cuma penyembunyian identitas responden,
   karena itu memang bagian dari bentuk keluaran. */

const apiMaxLimit = 500

// apiScope mengambil API key dari context lalu memastikan formId di URL memang milik
// key tersebut. Ketidakcocokan dijawab 404, bukan 403, supaya sebuah key tidak bisa
// dipakai memetakan kuesioner lain yang ada di sistem.
func (s *Server) apiScope(w http.ResponseWriter, r *http.Request) (*models.FormAPIKey, store.ResponseScope, bool) {
	key := apiKeyFromContext(r.Context())
	if key == nil {
		writeErr(w, http.StatusUnauthorized, "API key tidak valid")
		return nil, store.ResponseScope{}, false
	}
	if formID := r.PathValue("formId"); formID != "" && formID != key.FormID {
		s.logAPIAccess(r, key, key.KeyPrefix, s.clientIP(r), http.StatusNotFound, 0, "formId di luar cakupan key")
		writeErr(w, http.StatusNotFound, "kuesioner tidak ditemukan")
		return nil, store.ResponseScope{}, false
	}
	return key, store.APIKeyScope(key), true
}

// apiPresent menyiapkan satu jawaban untuk dikirim keluar. Kalau key tidak berhak
// melihat identitas responden, respondentId dan meta (nama/email/IP) dibuang.
func apiPresent(rr models.Response, includeRespondent bool) models.Response {
	if !includeRespondent {
		rr.RespondentID = nil
		rr.Meta = nil
	}
	return rr
}

// apiMe mengembalikan metadata key yang sedang dipakai — untuk klien memastikan
// konfigurasinya benar tanpa perlu menarik data.
func (s *Server) apiMe(w http.ResponseWriter, r *http.Request) {
	key, _, ok := s.apiScope(w, r)
	if !ok {
		return
	}
	form, err := s.st.GetForm(r.Context(), key.FormID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
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

// apiListResponses mengembalikan jawaban dalam cakupan key, berhalaman.
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
	// Draft tidak pernah ikut: itu dipaksakan oleh scope (ResponseScope.clauses), bukan
	// oleh filter di sini, supaya berlaku sama untuk semua jalur termasuk ekspor CSV.
	f := parseResponseFilter(q)

	rows, err := s.st.ListScopedResponses(r.Context(), scope, f, limit, offset)
	if err != nil {
		s.logAPIAccess(r, key, key.KeyPrefix, s.clientIP(r), http.StatusInternalServerError, 0, err.Error())
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	total, err := s.st.CountScopedResponses(r.Context(), scope, f)
	if err != nil {
		s.logAPIAccess(r, key, key.KeyPrefix, s.clientIP(r), http.StatusInternalServerError, 0, err.Error())
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
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

// apiGetResponse mengembalikan satu jawaban dalam cakupan key.
func (s *Server) apiGetResponse(w http.ResponseWriter, r *http.Request) {
	key, scope, ok := s.apiScope(w, r)
	if !ok {
		return
	}
	rr, err := s.st.GetScopedResponseByID(r.Context(), scope, r.PathValue("responseId"))
	if errors.Is(err, store.ErrNotFound) {
		s.logAPIAccess(r, key, key.KeyPrefix, s.clientIP(r), http.StatusNotFound, 0, "jawaban di luar cakupan")
		writeErr(w, http.StatusNotFound, "jawaban tidak ditemukan")
		return
	}
	if err != nil {
		s.logAPIAccess(r, key, key.KeyPrefix, s.clientIP(r), http.StatusInternalServerError, 0, err.Error())
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	s.logAPIAccess(r, key, key.KeyPrefix, s.clientIP(r), http.StatusOK, 1, "")
	presented := apiPresent(*rr, key.IncludeRespondent)
	writeJSON(w, http.StatusOK, s.signResponse(&presented))
}

// apiExportResponses men-stream seluruh jawaban dalam cakupan key sebagai CSV.
func (s *Server) apiExportResponses(w http.ResponseWriter, r *http.Request) {
	key, scope, ok := s.apiScope(w, r)
	if !ok {
		return
	}
	cols, err := s.st.GetFormAnswerColumns(r.Context(), key.FormID)
	if err != nil {
		s.logAPIAccess(r, key, key.KeyPrefix, s.clientIP(r), http.StatusInternalServerError, 0, err.Error())
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	// Kolom di luar visibleFields tidak boleh muncul sebagai header sekalipun kosong —
	// nama variabel sendiri sudah merupakan informasi.
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
