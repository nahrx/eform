package httpapi

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/csv"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/bpskaltim/eform-backend/internal/auth"
	"github.com/bpskaltim/eform-backend/internal/models"
	"github.com/bpskaltim/eform-backend/internal/store"
)

/* ================================================================
   SUPERADMIN — kelola permission viewer per kuesioner
   ================================================================ */

func (s *Server) createViewerPermission(w http.ResponseWriter, r *http.Request) {
	formID := r.PathValue("id")
	if _, ok := s.ensureFormAccess(w, r, formID); !ok {
		return
	}
	var in struct {
		ViewerID         string            `json:"viewerId"`
		RespondentAccess string            `json:"respondentAccess"`
		VisibleFields    []string          `json:"visibleFields"`
		FieldFilters     map[string]string `json:"fieldFilters"`
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
	p, err := s.st.CreateViewerPermission(r.Context(), in.ViewerID, formID, in.RespondentAccess, in.VisibleFields, in.FieldFilters, &createdBy)
	if err != nil {
		writeErr(w, http.StatusConflict, "viewer mungkin sudah memiliki akses ke kuesioner ini")
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) listFormViewerPermissions(w http.ResponseWriter, r *http.Request) {
	formID := r.PathValue("id")
	if _, ok := s.ensureFormAccess(w, r, formID); !ok {
		return
	}
	perms, err := s.st.ListFormViewerPermissions(r.Context(), formID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"permissions": perms})
}

// exportViewerPermissionsCSV mengunduh daftar akses viewer satu kuesioner sebagai CSV
// (untuk arsip/edit di Excel — bukan untuk diimpor balik langsung karena field_filters
// bisa berisi lebih dari satu variabel).
func (s *Server) exportViewerPermissionsCSV(w http.ResponseWriter, r *http.Request) {
	formID := r.PathValue("id")
	if _, ok := s.ensureFormAccess(w, r, formID); !ok {
		return
	}
	perms, err := s.st.ListFormViewerPermissions(r.Context(), formID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\"viewer-permissions-"+formID+".csv\"")
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"username", "respondent_access", "visible_fields", "field_filters"})
	for _, p := range perms {
		ff := make([]string, 0, len(p.FieldFilters))
		for k, v := range p.FieldFilters {
			ff = append(ff, k+"="+v)
		}
		_ = cw.Write([]string{p.ViewerUsername, p.RespondentAccess, strings.Join(p.VisibleFields, ";"), strings.Join(ff, ";")})
	}
	cw.Flush()
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
	if _, ok := s.ensureFormAccess(w, r, p.FormID); !ok {
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) updateViewerPermission(w http.ResponseWriter, r *http.Request) {
	perm, err := s.st.GetViewerPermissionByID(r.Context(), r.PathValue("permId"))
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
		RespondentAccess string            `json:"respondentAccess"`
		VisibleFields    []string          `json:"visibleFields"`
		FieldFilters     map[string]string `json:"fieldFilters"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "format permintaan salah")
		return
	}
	if in.RespondentAccess != "all" && in.RespondentAccess != "selected" {
		in.RespondentAccess = "all"
	}
	p, err := s.st.UpdateViewerPermission(r.Context(), r.PathValue("permId"), in.RespondentAccess, in.VisibleFields, in.FieldFilters)
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
	perm, err := s.st.GetViewerPermissionByID(r.Context(), r.PathValue("permId"))
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

// convertViewerToEditor mengubah satu permission viewer menjadi editor untuk akun & kuesioner yang
// sama — membawa respondentAccess, fieldFilters, dan daftar responden terpilih ke permission baru,
// lalu menghapus permission viewer yang lama. Role akun (users.role) tidak diubah — role bukan lagi
// gerbang eksklusif, hanya label default; kapabilitas sebenarnya ditentukan oleh tabel permission ini.
func (s *Server) convertViewerToEditor(w http.ResponseWriter, r *http.Request) {
	old, err := s.st.GetViewerPermissionByID(r.Context(), r.PathValue("permId"))
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "permission tidak ditemukan")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	if _, ok := s.ensureFormAccess(w, r, old.FormID); !ok {
		return
	}
	if _, err := s.st.GetEditorPermissionByEditorAndForm(r.Context(), old.ViewerID, old.FormID); err == nil {
		writeErr(w, http.StatusConflict, "akun ini sudah memiliki akses editor ke kuesioner ini")
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusInternalServerError, "gagal memeriksa akses")
		return
	}

	allowed, err := s.st.ListViewerAllowedRespondents(r.Context(), old.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}

	createdBy := userFrom(r.Context()).Subject
	newPerm, err := s.st.CreateEditorPermission(r.Context(), old.ViewerID, old.FormID, old.RespondentAccess, old.FieldFilters, &createdBy)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal membuat akses editor")
		return
	}
	for _, a := range allowed {
		if _, err := s.st.AddEditorAllowedRespondent(r.Context(), newPerm.ID, a.RespondentID); err != nil {
			writeErr(w, http.StatusInternalServerError, "gagal menyalin daftar responden")
			return
		}
	}
	if err := s.st.DeleteViewerPermission(r.Context(), old.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal menghapus akses viewer lama")
		return
	}
	writeJSON(w, http.StatusOK, newPerm)
}

/* ================================================================
   SUPERADMIN — bulk assign/hapus permission viewer (banyak sekaligus)
   ================================================================ */

// bulkAssignViewerPermissions memberi/memperbarui akses viewer ke satu kuesioner
// untuk banyak akun sekaligus (dibuat otomatis jika emailnya belum terdaftar).
// Setiap baris independen — satu baris gagal tidak menggagalkan baris lain.
func (s *Server) bulkAssignViewerPermissions(w http.ResponseWriter, r *http.Request) {
	formID := r.PathValue("id")
	if _, ok := s.ensureFormAccess(w, r, formID); !ok {
		return
	}
	var in struct {
		Items []struct {
			Email            string            `json:"email"`
			Note             string            `json:"note"`
			RespondentAccess string            `json:"respondentAccess"`
			VisibleFields    []string          `json:"visibleFields"`
			FieldFilters     map[string]string `json:"fieldFilters"`
		} `json:"items"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "format permintaan salah")
		return
	}
	createdBy := userFrom(r.Context()).Subject
	results := make([]map[string]any, len(in.Items))
	for i, item := range in.Items {
		res := map[string]any{"index": i}
		email := strings.TrimSpace(strings.ToLower(item.Email))
		res["email"] = email
		if email == "" {
			res["status"] = "error"
			res["error"] = "email wajib diisi"
			results[i] = res
			continue
		}
		u, err := s.st.GetUserByUsername(r.Context(), email)
		if errors.Is(err, store.ErrNotFound) {
			b := make([]byte, 24)
			if _, rerr := rand.Read(b); rerr != nil {
				res["status"] = "error"
				res["error"] = "gagal membuat password acak"
				results[i] = res
				continue
			}
			hash, herr := auth.HashPassword(base64.RawURLEncoding.EncodeToString(b))
			if herr != nil {
				res["status"] = "error"
				res["error"] = "gagal memproses password"
				results[i] = res
				continue
			}
			u, err = s.st.CreateUser(r.Context(), email, email, hash, "viewer", strings.TrimSpace(item.Note))
			if err != nil {
				res["status"] = "error"
				res["error"] = "gagal membuat akun viewer"
				results[i] = res
				continue
			}
		} else if err != nil {
			res["status"] = "error"
			res["error"] = "gagal memeriksa akun"
			results[i] = res
			continue
		} else if u.Role == "superadmin" || u.Role == "admin" {
			res["status"] = "error"
			res["error"] = "email terdaftar sebagai akun admin"
			results[i] = res
			continue
		}
		res["viewerId"] = u.ID
		respondentAccess := item.RespondentAccess
		if respondentAccess != "all" && respondentAccess != "selected" {
			respondentAccess = "all"
		}
		existing, err := s.st.GetViewerPermission(r.Context(), u.ID, formID)
		if errors.Is(err, store.ErrNotFound) {
			p, cerr := s.st.CreateViewerPermission(r.Context(), u.ID, formID, respondentAccess, item.VisibleFields, item.FieldFilters, &createdBy)
			if cerr != nil {
				res["status"] = "error"
				res["error"] = "gagal membuat akses"
				results[i] = res
				continue
			}
			res["status"] = "created"
			res["permissionId"] = p.ID
		} else if err != nil {
			res["status"] = "error"
			res["error"] = "gagal memeriksa akses"
			results[i] = res
			continue
		} else {
			p, uerr := s.st.UpdateViewerPermission(r.Context(), existing.ID, respondentAccess, item.VisibleFields, item.FieldFilters)
			if uerr != nil {
				res["status"] = "error"
				res["error"] = "gagal memperbarui akses"
				results[i] = res
				continue
			}
			res["status"] = "updated"
			res["permissionId"] = p.ID
		}
		results[i] = res
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

// bulkDeleteViewerPermissions menghapus banyak permission viewer sekaligus untuk satu kuesioner.
func (s *Server) bulkDeleteViewerPermissions(w http.ResponseWriter, r *http.Request) {
	formID := r.PathValue("id")
	if _, ok := s.ensureFormAccess(w, r, formID); !ok {
		return
	}
	var in struct {
		PermissionIDs []string `json:"permissionIds"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "format permintaan salah")
		return
	}
	results := make([]map[string]any, 0, len(in.PermissionIDs))
	for _, id := range in.PermissionIDs {
		perm, err := s.st.GetViewerPermissionByID(r.Context(), id)
		if err != nil || perm.FormID != formID {
			results = append(results, map[string]any{"id": id, "status": "error", "error": "permission tidak ditemukan"})
			continue
		}
		if err := s.st.DeleteViewerPermission(r.Context(), id); err != nil {
			results = append(results, map[string]any{"id": id, "status": "error", "error": "gagal menghapus"})
			continue
		}
		results = append(results, map[string]any{"id": id, "status": "deleted"})
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

/* ================================================================
   SUPERADMIN — kelola allowed respondents per permission
   ================================================================ */

func (s *Server) listViewerAllowedRespondents(w http.ResponseWriter, r *http.Request) {
	perm, err := s.st.GetViewerPermissionByID(r.Context(), r.PathValue("permId"))
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

	items, err := s.st.ListViewerAllowedRespondents(r.Context(), r.PathValue("permId"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"respondents": items})
}

func (s *Server) addViewerAllowedRespondent(w http.ResponseWriter, r *http.Request) {
	perm, err := s.st.GetViewerPermissionByID(r.Context(), r.PathValue("permId"))
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
	item, err := s.st.GetViewerAllowedRespondentByID(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "data tidak ditemukan")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	perm, err := s.st.GetViewerPermissionByID(r.Context(), item.PermissionID)
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
	formID := r.PathValue("id")
	if _, ok := s.ensureFormAccess(w, r, formID); !ok {
		return
	}
	respondents, err := s.st.ListFormRespondents(r.Context(), formID)
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

// viewerGetForm mengembalikan data form untuk viewer yang punya permission.
func (s *Server) viewerGetForm(w http.ResponseWriter, r *http.Request) {
	viewerID := userFrom(r.Context()).Subject
	formID := r.PathValue("id")
	if _, err := s.st.GetViewerPermission(r.Context(), viewerID, formID); errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusForbidden, "tidak memiliki akses ke kuesioner ini")
		return
	} else if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
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

// viewerListResponses melayani daftar jawaban yang boleh dilihat viewer (dengan pembatasan).
func (s *Server) viewerListResponses(w http.ResponseWriter, r *http.Request) {
	viewerID := userFrom(r.Context()).Subject
	formID := r.PathValue("id")

	// Pastikan viewer punya akses
	if _, err := s.st.GetViewerPermission(r.Context(), viewerID, formID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusForbidden, "tidak memiliki akses ke kuesioner ini")
		} else {
			writeErr(w, http.StatusInternalServerError, "gagal memeriksa akses")
		}
		return
	}

	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	f := parseResponseFilter(q)

	resp, err := s.st.ListViewerResponses(r.Context(), viewerID, formID, f, limit, offset)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	count, _ := s.st.CountViewerResponses(r.Context(), viewerID, formID, f)
	writeJSON(w, http.StatusOK, map[string]any{"responses": resp, "total": count})
}

// viewerExportResponses menghasilkan CSV jawaban yang boleh dilihat viewer, mengikuti pembatasan
// permission-nya (respondentAccess, fieldFilters per baris, visibleFields per kolom).
func (s *Server) viewerExportResponses(w http.ResponseWriter, r *http.Request) {
	viewerID := userFrom(r.Context()).Subject
	formID := r.PathValue("id")

	perm, err := s.st.GetViewerPermission(r.Context(), viewerID, formID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusForbidden, "tidak memiliki akses ke kuesioner ini")
		} else {
			writeErr(w, http.StatusInternalServerError, "gagal memeriksa akses")
		}
		return
	}
	cols, err := s.st.GetFormAnswerColumns(r.Context(), formID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	if len(perm.VisibleFields) > 0 {
		allowed := make(map[string]bool, len(perm.VisibleFields))
		for _, f := range perm.VisibleFields {
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
	w.Header().Set("Content-Disposition", "attachment; filename=\"responses-"+formID+".csv\"")
	cw := csv.NewWriter(w)
	_ = cw.Write(append(csvBaseHeader(true), cols...))
	_ = s.st.ForEachViewerResponse(r.Context(), viewerID, formID, func(rr models.Response) error {
		writeCSVRow(cw, rr, cols, true)
		return nil
	})
}

// viewerGetResponse mengembalikan detail satu respons yang boleh dilihat viewer.
func (s *Server) viewerGetResponse(w http.ResponseWriter, r *http.Request) {
	viewerID := userFrom(r.Context()).Subject
	formID := r.PathValue("id")
	responseID := r.PathValue("responseId")

	resp, err := s.st.GetViewerResponseByID(r.Context(), viewerID, formID, responseID)
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
