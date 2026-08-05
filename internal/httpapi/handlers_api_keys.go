package httpapi

import (
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bpskaltim/eform-backend/internal/models"
	"github.com/bpskaltim/eform-backend/internal/store"
)

/* Pengelolaan API key dari panel /manage (menu "API").
   Semua handler di file ini butuh JWT admin + ensureFormAccess — artinya superadmin,
   atau admin pemilik kuesioner. Endpoint yang dipakai sistem eksternal ada di
   handlers_api_public.go dan diautentikasi dengan API key, bukan JWT. */

// apiKeyInput adalah bentuk permintaan buat/ubah API key dari panel.
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

// toModel memvalidasi input dan menyalinnya ke model. Nilai di luar rentang wajar
// dinormalkan, bukan ditolak — kecuali yang benar-benar salah bentuk.
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
				return nil, errors.New("format CIDR salah: " + entry)
			}
		} else if net.ParseIP(entry) == nil {
			return nil, errors.New("format alamat IP salah: " + entry)
		}
		k.AllowedIPs = append(k.AllowedIPs, entry)
	}
	if in.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, in.ExpiresAt)
		if err != nil {
			return nil, errors.New("format expiresAt harus RFC3339")
		}
		k.ExpiresAt = &t
	}
	return k, nil
}

// newAPIKeyCredential menerbitkan kredensial baru: key plaintext untuk ditampilkan
// sekali, prefix untuk identifikasi, dan hash untuk disimpan.
func newAPIKeyCredential() (plaintext, prefix, hash string) {
	plaintext = apiKeyPrefix + randToken(32)
	body := strings.TrimPrefix(plaintext, apiKeyPrefix)
	return plaintext, body[:10], hashAPIKey(plaintext)
}

// createAPIKey membuat API key baru. Ini satu-satunya respons yang memuat key
// plaintext — setelah ini hanya prefix-nya yang bisa dilihat lagi.
func (s *Server) createAPIKey(w http.ResponseWriter, r *http.Request) {
	formID := r.PathValue("id")
	if _, ok := s.ensureFormAccess(w, r, formID); !ok {
		return
	}
	var in apiKeyInput
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "format permintaan salah")
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
		writeErr(w, http.StatusInternalServerError, "gagal membuat API key")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"apiKey": out, "key": plaintext})
}

// listAPIKeys mengembalikan semua API key satu kuesioner (tanpa key plaintext maupun hash).
func (s *Server) listAPIKeys(w http.ResponseWriter, r *http.Request) {
	formID := r.PathValue("id")
	if _, ok := s.ensureFormAccess(w, r, formID); !ok {
		return
	}
	keys, err := s.st.ListAPIKeysByForm(r.Context(), formID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"apiKeys": keys})
}

// updateAPIKey memperbarui label dan pengaturan cakupan/keamanan satu key.
func (s *Server) updateAPIKey(w http.ResponseWriter, r *http.Request) {
	key, ok := s.ensureAPIKeyAccess(w, r, r.PathValue("keyId"))
	if !ok {
		return
	}
	var in apiKeyInput
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "format permintaan salah")
		return
	}
	next, err := in.toModel(key.FormID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	out, err := s.st.UpdateAPIKey(r.Context(), key.ID, next)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "API key tidak ditemukan")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal memperbarui API key")
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// rotateAPIKey menerbitkan kredensial baru untuk key yang sama. Key lama langsung
// tidak berlaku, sementara seluruh pengaturan cakupannya tetap.
func (s *Server) rotateAPIKey(w http.ResponseWriter, r *http.Request) {
	key, ok := s.ensureAPIKeyAccess(w, r, r.PathValue("keyId"))
	if !ok {
		return
	}
	plaintext, prefix, hash := newAPIKeyCredential()
	out, err := s.st.RotateAPIKey(r.Context(), key.ID, prefix, hash)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal merotasi API key")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"apiKey": out, "key": plaintext})
}

// deleteAPIKey menghapus API key beserta daftar responden dan mengosongkan kaitannya di audit log.
func (s *Server) deleteAPIKey(w http.ResponseWriter, r *http.Request) {
	key, ok := s.ensureAPIKeyAccess(w, r, r.PathValue("keyId"))
	if !ok {
		return
	}
	if err := s.st.DeleteAPIKey(r.Context(), key.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal menghapus API key")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// listAPIKeyRespondents mengembalikan daftar responden yang boleh diakses key ini.
func (s *Server) listAPIKeyRespondents(w http.ResponseWriter, r *http.Request) {
	key, ok := s.ensureAPIKeyAccess(w, r, r.PathValue("keyId"))
	if !ok {
		return
	}
	list, err := s.st.ListAPIKeyAllowedRespondents(r.Context(), key.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"respondents": list})
}

// addAPIKeyRespondent menambahkan satu responden ke daftar yang diizinkan.
func (s *Server) addAPIKeyRespondent(w http.ResponseWriter, r *http.Request) {
	key, ok := s.ensureAPIKeyAccess(w, r, r.PathValue("keyId"))
	if !ok {
		return
	}
	var in struct {
		RespondentID string `json:"respondentId"`
	}
	if err := decodeJSON(r, &in); err != nil || in.RespondentID == "" {
		writeErr(w, http.StatusBadRequest, "respondentId wajib diisi")
		return
	}
	ar, err := s.st.AddAPIKeyAllowedRespondent(r.Context(), key.ID, in.RespondentID)
	if errors.Is(err, store.ErrNotFound) {
		// ON CONFLICT DO NOTHING — responden sudah ada di daftar, bukan kegagalan.
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal menambahkan responden")
		return
	}
	writeJSON(w, http.StatusCreated, ar)
}

// removeAPIKeyRespondent menghapus satu responden dari daftar yang diizinkan.
func (s *Server) removeAPIKeyRespondent(w http.ResponseWriter, r *http.Request) {
	ar, err := s.st.GetAPIKeyAllowedRespondentByID(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "data tidak ditemukan")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	if _, ok := s.ensureAPIKeyAccess(w, r, ar.PermissionID); !ok {
		return
	}
	if err := s.st.RemoveAPIKeyAllowedRespondent(r.Context(), ar.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal menghapus responden")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// listAPIKeyLogs mengembalikan riwayat panggilan API untuk satu key.
func (s *Server) listAPIKeyLogs(w http.ResponseWriter, r *http.Request) {
	key, ok := s.ensureAPIKeyAccess(w, r, r.PathValue("keyId"))
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	logs, err := s.st.ListAPIAccessLogs(r.Context(), key.ID, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"logs": logs})
}

// listActivityLogs menampilkan riwayat aksi admin. Tanpa parameter form, hanya
// superadmin yang boleh melihat seluruh sistem; dengan ?formId=, admin pemilik
// kuesioner boleh melihat jejak kuesionernya sendiri.
func (s *Server) listActivityLogs(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	formID := r.URL.Query().Get("formId")

	if formID == "" {
		if u.Role != "superadmin" {
			writeErr(w, http.StatusForbidden, "akses ditolak")
			return
		}
	} else if _, ok := s.ensureFormAccess(w, r, formID); !ok {
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	logs, total, err := s.st.ListActivityLogs(r.Context(), formID, limit, offset)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"logs": logs, "total": total})
}

// ensureAPIKeyAccess memastikan key ada dan pemanggil berhak mengelola kuesionernya.
func (s *Server) ensureAPIKeyAccess(w http.ResponseWriter, r *http.Request, keyID string) (*models.FormAPIKey, bool) {
	k, err := s.st.GetAPIKeyByID(r.Context(), keyID)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "API key tidak ditemukan")
		return nil, false
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return nil, false
	}
	if _, ok := s.ensureFormAccess(w, r, k.FormID); !ok {
		return nil, false
	}
	return k, true
}
