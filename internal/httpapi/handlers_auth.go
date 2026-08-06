package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/nahrx/eform/internal/auth"
	"github.com/nahrx/eform/internal/models"
	"github.com/nahrx/eform/internal/store"
)

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request format")
		return
	}
	in.Username = strings.TrimSpace(in.Username)
	if in.Username == "" || in.Password == "" {
		writeErr(w, http.StatusBadRequest, "username and password are required")
		return
	}
	if !loginRL.allowRequest(r) {
		writeErr(w, http.StatusTooManyRequests, "too many login attempts, please try again in 1 minute")
		return
	}
	u, err := s.st.GetUserByUsername(r.Context(), in.Username)
	if err != nil || !u.IsActive || !auth.CheckPassword(u.PasswordHash, in.Password) {
		writeErr(w, http.StatusUnauthorized, "incorrect username or password")
		return
	}
	token, err := s.auth.Generate(u.ID, u.Username, u.Role, u.TokenVersion)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to create token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "user": u})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	c := userFrom(r.Context())
	lang := "id"
	if u, err := s.st.GetUser(r.Context(), c.Subject); err == nil {
		lang = u.PreferredLanguage
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": c.Subject, "username": c.Username, "role": c.Role, "preferredLanguage": lang,
	})
}

// updateMyLanguage changes the UI language preference (builder/dashboard) of the signed-in account.
func (s *Server) updateMyLanguage(w http.ResponseWriter, r *http.Request) {
	c := userFrom(r.Context())
	var in struct {
		Language string `json:"language"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request format")
		return
	}
	if in.Language != "id" && in.Language != "en" {
		writeErr(w, http.StatusBadRequest, "unsupported language")
		return
	}
	if err := s.st.UpdateUserLanguage(r.Context(), c.Subject, in.Language); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to save language preference")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"preferredLanguage": in.Language})
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request format")
		return
	}
	in.Username = strings.TrimSpace(in.Username)
	if in.Username == "" {
		writeErr(w, http.StatusBadRequest, "username is required")
		return
	}
	if err := auth.ValidatePassword(in.Password, in.Username, in.Email); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.Role == "" {
		in.Role = "admin"
	}
	if in.Role != "superadmin" && in.Role != "admin" {
		writeErr(w, http.StatusBadRequest, "invalid role")
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to hash password")
		return
	}
	u, err := s.st.CreateUser(r.Context(), in.Username, in.Email, hash, in.Role, "")
	if err != nil {
		writeErr(w, http.StatusConflict, "username/email may already be taken")
		return
	}
	s.audit(r, "user.create", "user", u.ID, u.Username, "", "role="+u.Role)
	writeJSON(w, http.StatusCreated, u)
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.st.ListUsers(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
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
		writeErr(w, http.StatusBadRequest, "invalid request format")
		return
	}
	in.Username = strings.TrimSpace(in.Username)
	in.Email = strings.TrimSpace(in.Email)
	in.Password = strings.TrimSpace(in.Password)
	if in.Username == "" {
		writeErr(w, http.StatusBadRequest, "username is required")
		return
	}
	if in.Role != "admin" && in.Role != "superadmin" {
		in.Role = "admin"
	}
	if in.Password != "" {
		if err := auth.ValidatePassword(in.Password, in.Username, in.Email); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	// Make sure the target is an admin/superadmin — prevents viewer/editor privilege escalation
	target, err := s.st.GetUser(r.Context(), id)
	if err != nil || (target.Role != "admin" && target.Role != "superadmin") {
		writeErr(w, http.StatusNotFound, "user not found")
		return
	}
	if err := s.st.UpdateAdminUser(r.Context(), id, in.Username, in.Email, in.Role); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "user not found")
			return
		}
		writeErr(w, http.StatusConflict, "username/email may already be taken")
		return
	}
	pwChanged := false
	if in.Password != "" {
		hash, err := auth.HashPassword(in.Password)
		if err == nil {
			_ = s.st.UpdateUserPassword(r.Context(), id, hash)
			pwChanged = true
		}
	}
	detail := "role=" + in.Role
	if pwChanged {
		detail += ", password changed (existing sessions revoked)"
	}
	s.audit(r, "user.update", "user", id, in.Username, "", detail)
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) deleteAdminUser(w http.ResponseWriter, r *http.Request) {
	caller := userFrom(r.Context())
	id := r.PathValue("id")
	if caller != nil && caller.Subject == id {
		writeErr(w, http.StatusBadRequest, "you cannot delete your own account")
		return
	}
	if err := s.st.DeleteAdminUser(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "user not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, "failed to delete")
		return
	}
	s.audit(r, "user.delete", "user", id, "", "", "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
