package httpapi

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/bpskaltim/eform-backend/internal/auth"
	"github.com/bpskaltim/eform-backend/internal/store"
)

/* ================================================================
   SUPERADMIN — kelola akun viewer
   ================================================================ */

func (s *Server) createViewer(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Note     string `json:"note"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "format permintaan salah")
		return
	}
	in.Username = strings.TrimSpace(in.Username)
	in.Email = strings.TrimSpace(in.Email)
	in.Note = strings.TrimSpace(in.Note)
	if in.Username == "" || in.Email == "" {
		writeErr(w, http.StatusBadRequest, "username dan email wajib diisi")
		return
	}
	// Viewer login via Google, jadi buat password acak (tidak dipakai untuk login)
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	randomPwd := base64.RawURLEncoding.EncodeToString(b)
	hash, err := auth.HashPassword(randomPwd)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal memproses password")
		return
	}
	u, err := s.st.CreateUser(r.Context(), in.Username, in.Email, hash, "viewer", in.Note)
	if err != nil {
		writeErr(w, http.StatusConflict, "username/email mungkin sudah dipakai")
		return
	}
	writeJSON(w, http.StatusCreated, u)
}

func (s *Server) listViewers(w http.ResponseWriter, r *http.Request) {
	viewers, err := s.st.ListViewers(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"viewers": viewers})
}

func (s *Server) deleteViewer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.st.DeleteUser(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "viewer tidak ditemukan")
			return
		}
		writeErr(w, http.StatusInternalServerError, "gagal menghapus")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

/* ================================================================
   SUPERADMIN — kelola permission viewer per kuesioner
   ================================================================ */

func (s *Server) createViewerPermission(w http.ResponseWriter, r *http.Request) {
	formID := r.PathValue("id")
	var in struct {
		ViewerID         string   `json:"viewerId"`
		RespondentAccess string   `json:"respondentAccess"`
		VisibleFields    []string `json:"visibleFields"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "format permintaan salah")
		return
	}
	if in.ViewerID == "" {
		writeErr(w, http.StatusBadRequest, "viewerId wajib diisi")
		return
	}
	if in.RespondentAccess != "all" && in.RespondentAccess != "selected" {
		in.RespondentAccess = "all"
	}
	createdBy := userFrom(r.Context()).Subject
	p, err := s.st.CreateViewerPermission(r.Context(), in.ViewerID, formID, in.RespondentAccess, in.VisibleFields, &createdBy)
	if err != nil {
		writeErr(w, http.StatusConflict, "viewer mungkin sudah memiliki akses ke kuesioner ini")
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) listFormViewerPermissions(w http.ResponseWriter, r *http.Request) {
	perms, err := s.st.ListFormViewerPermissions(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"permissions": perms})
}

func (s *Server) getViewerPermission(w http.ResponseWriter, r *http.Request) {
	p, err := s.st.GetViewerPermissionByID(r.Context(), r.PathValue("permId"))
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "permission tidak ditemukan")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) updateViewerPermission(w http.ResponseWriter, r *http.Request) {
	var in struct {
		RespondentAccess string   `json:"respondentAccess"`
		VisibleFields    []string `json:"visibleFields"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "format permintaan salah")
		return
	}
	if in.RespondentAccess != "all" && in.RespondentAccess != "selected" {
		in.RespondentAccess = "all"
	}
	p, err := s.st.UpdateViewerPermission(r.Context(), r.PathValue("permId"), in.RespondentAccess, in.VisibleFields)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "permission tidak ditemukan")
			return
		}
		writeErr(w, http.StatusInternalServerError, "gagal memperbarui")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) deleteViewerPermission(w http.ResponseWriter, r *http.Request) {
	if err := s.st.DeleteViewerPermission(r.Context(), r.PathValue("permId")); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "permission tidak ditemukan")
			return
		}
		writeErr(w, http.StatusInternalServerError, "gagal menghapus")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

/* ================================================================
   SUPERADMIN — kelola allowed respondents per permission
   ================================================================ */

func (s *Server) listViewerAllowedRespondents(w http.ResponseWriter, r *http.Request) {
	items, err := s.st.ListViewerAllowedRespondents(r.Context(), r.PathValue("permId"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"respondents": items})
}

func (s *Server) addViewerAllowedRespondent(w http.ResponseWriter, r *http.Request) {
	var in struct {
		RespondentID string `json:"respondentId"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "format permintaan salah")
		return
	}
	if in.RespondentID == "" {
		writeErr(w, http.StatusBadRequest, "respondentId wajib diisi")
		return
	}
	item, err := s.st.AddViewerAllowedRespondent(r.Context(), r.PathValue("permId"), in.RespondentID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal menambahkan")
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) removeViewerAllowedRespondent(w http.ResponseWriter, r *http.Request) {
	if err := s.st.RemoveViewerAllowedRespondent(r.Context(), r.PathValue("id")); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "data tidak ditemukan")
			return
		}
		writeErr(w, http.StatusInternalServerError, "gagal menghapus")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// listFormRespondents digunakan superadmin untuk memilih responden saat konfigurasi 'selected'.
func (s *Server) listFormRespondents(w http.ResponseWriter, r *http.Request) {
	respondents, err := s.st.ListFormRespondents(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"respondents": respondents})
}

/* ================================================================
   VIEWER — endpoint yang dipanggil viewer setelah login
   ================================================================ */

// viewerMyForms mengembalikan semua kuesioner yang boleh dilihat viewer yang sedang login.
func (s *Server) viewerMyForms(w http.ResponseWriter, r *http.Request) {
	viewerID := userFrom(r.Context()).Subject
	perms, err := s.st.ListViewerForms(r.Context(), viewerID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"forms": perms})
}

// viewerMyFormPermission mengembalikan detail permission viewer untuk satu kuesioner.
func (s *Server) viewerMyFormPermission(w http.ResponseWriter, r *http.Request) {
	viewerID := userFrom(r.Context()).Subject
	formID := r.PathValue("id")
	perm, err := s.st.GetViewerPermission(r.Context(), viewerID, formID)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusForbidden, "tidak memiliki akses ke kuesioner ini")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	writeJSON(w, http.StatusOK, perm)
}

// viewerListResponses melayani daftar jawaban yang boleh dilihat viewer (dengan pembatasan).
func (s *Server) viewerListResponses(w http.ResponseWriter, r *http.Request) {
	viewerID := userFrom(r.Context()).Subject
	formID := r.PathValue("id")

	// Pastikan viewer punya akses
	if _, err := s.st.GetViewerPermission(r.Context(), viewerID, formID); errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusForbidden, "tidak memiliki akses ke kuesioner ini")
		return
	}

	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	f := store.ResponseFilter{
		Search:  strings.TrimSpace(q.Get("search")),
		SortBy:  q.Get("sortBy"),
		SortDir: q.Get("sortDir"),
	}

	resp, err := s.st.ListViewerResponses(r.Context(), viewerID, formID, f, limit, offset)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	count, _ := s.st.CountViewerResponses(r.Context(), viewerID, formID, f)
	writeJSON(w, http.StatusOK, map[string]any{"responses": resp, "total": count})
}
