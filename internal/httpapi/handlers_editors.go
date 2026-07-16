package httpapi

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/bpskaltim/eform-backend/internal/auth"
	"github.com/bpskaltim/eform-backend/internal/store"
)

/* ================================================================
   SUPERADMIN — kelola akun editor
   ================================================================ */

func (s *Server) createEditor(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email string `json:"email"`
		Note  string `json:"note"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "format permintaan salah")
		return
	}
	in.Email = strings.TrimSpace(strings.ToLower(in.Email))
	in.Note = strings.TrimSpace(in.Note)
	if in.Email == "" {
		writeErr(w, http.StatusBadRequest, "email wajib diisi")
		return
	}

	// Editor login via Google — username = email, password acak (tidak dipakai untuk login)
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal membuat password acak")
		return
	}
	randomPwd := base64.RawURLEncoding.EncodeToString(b)
	hash, err := auth.HashPassword(randomPwd)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal memproses password")
		return
	}

	u, err := s.st.CreateUser(r.Context(), in.Email, in.Email, hash, "editor", in.Note)
	if err != nil {
		writeErr(w, http.StatusConflict, "email mungkin sudah terdaftar")
		return
	}
	writeJSON(w, http.StatusCreated, u)
}

func (s *Server) listEditors(w http.ResponseWriter, r *http.Request) {
	editors, err := s.st.ListEditors(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"editors": editors})
}

func (s *Server) deleteEditor(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.st.DeleteUserByRole(r.Context(), id, "editor"); err != nil {
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
		EditorID     string            `json:"editorId"`
		FieldFilters map[string]string `json:"fieldFilters"`
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
	p, err := s.st.CreateEditorPermission(r.Context(), in.EditorID, formID, in.FieldFilters, &createdBy)
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

// getEditorPermission mengambil detail satu permission editor.
func (s *Server) getEditorPermission(w http.ResponseWriter, r *http.Request) {
	p, err := s.st.GetEditorPermissionByID(r.Context(), r.PathValue("permId"))
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "permission tidak ditemukan")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	if _, ok := s.ensureFormAccess(w, r, p.FormID); !ok {
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// updateEditorPermission memperbarui field_filters permission editor.
func (s *Server) updateEditorPermission(w http.ResponseWriter, r *http.Request) {
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
	var in struct {
		FieldFilters map[string]string `json:"fieldFilters"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "format permintaan salah")
		return
	}
	p, err := s.st.UpdateEditorPermission(r.Context(), r.PathValue("permId"), in.FieldFilters)
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

// editorGetForm mengembalikan schema form untuk editor yang punya permission.
func (s *Server) editorGetForm(w http.ResponseWriter, r *http.Request) {
	editorID := userFrom(r.Context()).Subject
	formID := r.PathValue("id")
	ok, err := s.st.HasEditorFormPermission(r.Context(), editorID, formID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	if !ok {
		writeErr(w, http.StatusForbidden, "tidak memiliki akses ke kuesioner ini")
		return
	}
	f, err := s.st.GetForm(r.Context(), formID)
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

// editorListResponses mengembalikan semua jawaban (submitted & draft) untuk form yang ditugaskan ke editor.
func (s *Server) editorListResponses(w http.ResponseWriter, r *http.Request) {
	editorID := userFrom(r.Context()).Subject
	formID := r.PathValue("id")
	perm, err := s.st.GetEditorPermissionByEditorAndForm(r.Context(), editorID, formID)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusForbidden, "tidak memiliki akses ke kuesioner ini")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
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
	for key, vals := range q {
		if len(vals) == 0 || strings.TrimSpace(vals[0]) == "" {
			continue
		}
		val := strings.TrimSpace(vals[0])
		if strings.HasPrefix(key, "fe_") {
			if f.FieldExactFilters == nil {
				f.FieldExactFilters = make(map[string]string)
			}
			if len(f.FieldExactFilters) < 10 {
				f.FieldExactFilters[key[3:]] = val
			}
		} else if strings.HasPrefix(key, "f_") {
			if f.FieldFilters == nil {
				f.FieldFilters = make(map[string]string)
			}
			if len(f.FieldFilters) < 10 {
				f.FieldFilters[key[2:]] = val
			}
		}
	}
	// Terapkan field_filters permission editor sebagai filter wajib (tidak bisa di-override user)
	for k, v := range perm.FieldFilters {
		if f.FieldExactFilters == nil {
			f.FieldExactFilters = make(map[string]string)
		}
		f.FieldExactFilters[k] = v
	}
	resp, err := s.st.ListAllResponsesByForm(r.Context(), formID, f, limit, offset)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	count, _ := s.st.CountAllResponsesByForm(r.Context(), formID, f)
	writeJSON(w, http.StatusOK, map[string]any{"responses": resp, "total": count})
}

// editorGetResponse mengembalikan detail satu respons untuk editor, dengan cek field_filters.
func (s *Server) editorGetResponse(w http.ResponseWriter, r *http.Request) {
	editorID := userFrom(r.Context()).Subject
	formID := r.PathValue("id")
	responseID := r.PathValue("responseId")

	resp, err := s.st.GetEditorResponseByID(r.Context(), editorID, formID, responseID)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "respons tidak ditemukan atau akses tidak diizinkan")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// editorUpdateResponse memperbarui jawaban satu respons oleh editor.
func (s *Server) editorUpdateResponse(w http.ResponseWriter, r *http.Request) {
	editorID := userFrom(r.Context()).Subject
	formID := r.PathValue("id")
	responseID := r.PathValue("responseId")

	perm, err := s.st.GetEditorPermissionByEditorAndForm(r.Context(), editorID, formID)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusForbidden, "tidak memiliki akses ke kuesioner ini")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}

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
	if len(perm.FieldFilters) > 0 {
		var m map[string]any
		if err := json.Unmarshal(in.Answers, &m); err != nil {
			writeErr(w, http.StatusBadRequest, "format jawaban tidak valid")
			return
		}
		for field, required := range perm.FieldFilters {
			if v, ok := m[field]; !ok || fmt.Sprint(v) != required {
				writeErr(w, http.StatusForbidden, "jawaban tidak sesuai dengan izin edit yang diberikan")
				return
			}
		}
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
