package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/bpskaltim/eform-backend/internal/auth"
	"github.com/bpskaltim/eform-backend/internal/models"
	"github.com/bpskaltim/eform-backend/internal/store"
)

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "format permintaan salah")
		return
	}
	in.Username = strings.TrimSpace(in.Username)
	if in.Username == "" || in.Password == "" {
		writeErr(w, http.StatusBadRequest, "username dan password wajib diisi")
		return
	}
	u, err := s.st.GetUserByUsername(r.Context(), in.Username)
	if err != nil || !u.IsActive || !auth.CheckPassword(u.PasswordHash, in.Password) {
		writeErr(w, http.StatusUnauthorized, "username atau password salah")
		return
	}
	token, err := s.auth.Generate(u.ID, u.Username, u.Role)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal membuat token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "user": u})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	c := userFrom(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"id": c.Subject, "username": c.Username, "role": c.Role,
	})
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "format permintaan salah")
		return
	}
	in.Username = strings.TrimSpace(in.Username)
	if in.Username == "" || len(in.Password) < 6 {
		writeErr(w, http.StatusBadRequest, "username wajib, password minimal 6 karakter")
		return
	}
	if in.Role == "" {
		in.Role = "admin"
	}
	if in.Role != "superadmin" && in.Role != "admin" {
		writeErr(w, http.StatusBadRequest, "role tidak valid")
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal hashing password")
		return
	}
	u, err := s.st.CreateUser(r.Context(), in.Username, in.Email, hash, in.Role, "")
	if err != nil {
		writeErr(w, http.StatusConflict, "username/email mungkin sudah dipakai")
		return
	}
	writeJSON(w, http.StatusCreated, u)
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.st.ListUsers(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	if users == nil {
		users = []models.User{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}

func (s *Server) patchAdminUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Role     string `json:"role"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "format permintaan salah")
		return
	}
	in.Username = strings.TrimSpace(in.Username)
	in.Email = strings.TrimSpace(in.Email)
	in.Password = strings.TrimSpace(in.Password)
	if in.Username == "" {
		writeErr(w, http.StatusBadRequest, "username wajib diisi")
		return
	}
	if in.Role != "admin" && in.Role != "superadmin" {
		in.Role = "admin"
	}
	if in.Password != "" && len(in.Password) < 6 {
		writeErr(w, http.StatusBadRequest, "password minimal 6 karakter")
		return
	}
	if err := s.st.UpdateAdminUser(r.Context(), id, in.Username, in.Email, in.Role); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "user tidak ditemukan")
			return
		}
		writeErr(w, http.StatusConflict, "username/email mungkin sudah dipakai")
		return
	}
	if in.Password != "" {
		hash, err := auth.HashPassword(in.Password)
		if err == nil {
			_ = s.st.UpdateUserPassword(r.Context(), id, hash)
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) deleteAdminUser(w http.ResponseWriter, r *http.Request) {
	caller := userFrom(r.Context())
	id := r.PathValue("id")
	if caller != nil && caller.Subject == id {
		writeErr(w, http.StatusBadRequest, "tidak bisa menghapus akun sendiri")
		return
	}
	if err := s.st.DeleteUser(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "user tidak ditemukan")
			return
		}
		writeErr(w, http.StatusInternalServerError, "gagal menghapus")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
