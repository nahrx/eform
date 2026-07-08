package httpapi

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"

	"github.com/bpskaltim/eform-backend/internal/auth"
	"github.com/bpskaltim/eform-backend/internal/store"
)

/* ================================================================
   SUPERADMIN — kelola akun editor
   ================================================================ */

func (s *Server) createEditor(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Note     string `json:"note"`
		FormID   string `json:"formId"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "format permintaan salah")
		return
	}
	if !s.ensureAdminFormScope(w, r, strings.TrimSpace(in.FormID)) {
		return
	}
	in.Username = strings.TrimSpace(in.Username)
	in.Email = strings.TrimSpace(in.Email)
	in.Note = strings.TrimSpace(in.Note)
	if in.Username == "" || in.Email == "" {
		writeErr(w, http.StatusBadRequest, "username dan email wajib diisi")
		return
	}

	// Editor dipersiapkan berbasis akun Google, jadi password dibuat acak.
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	randomPwd := base64.RawURLEncoding.EncodeToString(b)
	hash, err := auth.HashPassword(randomPwd)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal memproses password")
		return
	}

	u, err := s.st.CreateUser(r.Context(), in.Username, in.Email, hash, "editor", in.Note)
	if err != nil {
		writeErr(w, http.StatusConflict, "username/email mungkin sudah dipakai")
		return
	}
	writeJSON(w, http.StatusCreated, u)
}

func (s *Server) listEditors(w http.ResponseWriter, r *http.Request) {
	if !s.ensureAdminFormScope(w, r, strings.TrimSpace(r.URL.Query().Get("formId"))) {
		return
	}
	editors, err := s.st.ListEditors(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"editors": editors})
}

func (s *Server) deleteEditor(w http.ResponseWriter, r *http.Request) {
	if !s.ensureAdminFormScope(w, r, strings.TrimSpace(r.URL.Query().Get("formId"))) {
		return
	}
	id := r.PathValue("id")
	if err := s.st.DeleteUser(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "editor tidak ditemukan")
			return
		}
		writeErr(w, http.StatusInternalServerError, "gagal menghapus")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// createEditorPermission memberikan akses editor ke satu kuesioner (khusus superadmin).
func (s *Server) createEditorPermission(w http.ResponseWriter, r *http.Request) {
	formID := r.PathValue("id")
	if _, ok := s.ensureFormAccess(w, r, formID); !ok {
		return
	}

	var in struct {
		EditorID string `json:"editorId"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "format permintaan salah")
		return
	}
	if in.EditorID == "" {
		writeErr(w, http.StatusBadRequest, "editorId wajib diisi")
		return
	}

	createdBy := userFrom(r.Context()).Subject
	p, err := s.st.CreateEditorPermission(r.Context(), in.EditorID, formID, &createdBy)
	if err != nil {
		writeErr(w, http.StatusConflict, "editor mungkin sudah memiliki akses ke kuesioner ini")
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

// listFormEditorPermissions mengembalikan semua editor yang punya akses ke satu kuesioner.
func (s *Server) listFormEditorPermissions(w http.ResponseWriter, r *http.Request) {
	formID := r.PathValue("id")
	if _, ok := s.ensureFormAccess(w, r, formID); !ok {
		return
	}
	perms, err := s.st.ListFormEditorPermissions(r.Context(), formID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"permissions": perms})
}

// deleteEditorPermission mencabut akses editor dari satu kuesioner.
func (s *Server) deleteEditorPermission(w http.ResponseWriter, r *http.Request) {
	perm, err := s.st.GetEditorPermissionByID(r.Context(), r.PathValue("permId"))
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "permission tidak ditemukan")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	if _, ok := s.ensureFormAccess(w, r, perm.FormID); !ok {
		return
	}

	if err := s.st.DeleteEditorPermission(r.Context(), r.PathValue("permId")); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "permission tidak ditemukan")
			return
		}
		writeErr(w, http.StatusInternalServerError, "gagal menghapus")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// editorMyForms mengembalikan form yang ditugaskan ke editor login.
func (s *Server) editorMyForms(w http.ResponseWriter, r *http.Request) {
	editorID := userFrom(r.Context()).Subject
	forms, err := s.st.ListFormsByEditor(r.Context(), editorID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"forms": forms})
}
