package httpapi

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/bpskaltim/eform-backend/internal/auth"
	"github.com/bpskaltim/eform-backend/internal/models"
	"github.com/bpskaltim/eform-backend/internal/store"
)

// createEditorPermission memberikan akses editor ke satu kuesioner (khusus superadmin).
func (s *Server) createEditorPermission(w http.ResponseWriter, r *http.Request) {
	formID := r.PathValue("id")
	if _, ok := s.ensureFormAccess(w, r, formID); !ok {
		return
	}

	var in struct {
		EditorID         string            `json:"editorId"`
		RespondentAccess string            `json:"respondentAccess"`
		FieldFilters     map[string]string `json:"fieldFilters"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "format permintaan salah")
		return
	}
	if in.EditorID == "" {
		writeErr(w, http.StatusBadRequest, "editorId wajib diisi")
		return
	}
	if in.RespondentAccess != "all" && in.RespondentAccess != "selected" {
		in.RespondentAccess = "all"
	}

	createdBy := userFrom(r.Context()).Subject
	p, err := s.st.CreateEditorPermission(r.Context(), in.EditorID, formID, in.RespondentAccess, in.FieldFilters, &createdBy)
	if err != nil {
		writeErr(w, http.StatusConflict, "editor mungkin sudah memiliki akses ke kuesioner ini")
		return
	}
	s.audit(r, "permission.editor.create", "permission", p.ID, in.EditorID, formID, "akses="+in.RespondentAccess)
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

// exportEditorPermissionsCSV mengunduh daftar akses editor satu kuesioner sebagai CSV.
func (s *Server) exportEditorPermissionsCSV(w http.ResponseWriter, r *http.Request) {
	formID := r.PathValue("id")
	if _, ok := s.ensureFormAccess(w, r, formID); !ok {
		return
	}
	perms, err := s.st.ListFormEditorPermissions(r.Context(), formID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\"editor-permissions-"+formID+".csv\"")
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"editor_name", "respondent_access", "field_filters"})
	for _, p := range perms {
		ff := make([]string, 0, len(p.FieldFilters))
		for k, v := range p.FieldFilters {
			ff = append(ff, k+"="+v)
		}
		_ = cw.Write([]string{p.EditorName, p.RespondentAccess, strings.Join(ff, ";")})
	}
	cw.Flush()
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
		RespondentAccess string            `json:"respondentAccess"`
		FieldFilters     map[string]string `json:"fieldFilters"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "format permintaan salah")
		return
	}
	if in.RespondentAccess != "all" && in.RespondentAccess != "selected" {
		in.RespondentAccess = "all"
	}
	p, err := s.st.UpdateEditorPermission(r.Context(), r.PathValue("permId"), in.RespondentAccess, in.FieldFilters)
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

// listEditorAllowedRespondents mengembalikan semua responden yang diizinkan untuk satu permission editor.
func (s *Server) listEditorAllowedRespondents(w http.ResponseWriter, r *http.Request) {
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

	items, err := s.st.ListEditorAllowedRespondents(r.Context(), r.PathValue("permId"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"respondents": items})
}

// addEditorAllowedRespondent menambahkan satu responden ke daftar yang diizinkan untuk editor.
func (s *Server) addEditorAllowedRespondent(w http.ResponseWriter, r *http.Request) {
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
	item, err := s.st.AddEditorAllowedRespondent(r.Context(), r.PathValue("permId"), in.RespondentID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal menambahkan")
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

// removeEditorAllowedRespondent menghapus satu responden dari daftar yang diizinkan untuk editor.
func (s *Server) removeEditorAllowedRespondent(w http.ResponseWriter, r *http.Request) {
	item, err := s.st.GetEditorAllowedRespondentByID(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "data tidak ditemukan")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	perm, err := s.st.GetEditorPermissionByID(r.Context(), item.PermissionID)
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

	if err := s.st.RemoveEditorAllowedRespondent(r.Context(), r.PathValue("id")); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "data tidak ditemukan")
			return
		}
		writeErr(w, http.StatusInternalServerError, "gagal menghapus")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// bulkAssignEditorPermissions memberi/memperbarui akses editor ke satu kuesioner
// untuk banyak akun sekaligus (dibuat otomatis jika emailnya belum terdaftar).
// Setiap baris independen — satu baris gagal tidak menggagalkan baris lain.
func (s *Server) bulkAssignEditorPermissions(w http.ResponseWriter, r *http.Request) {
	formID := r.PathValue("id")
	if _, ok := s.ensureFormAccess(w, r, formID); !ok {
		return
	}
	var in struct {
		Items []struct {
			Email            string            `json:"email"`
			Note             string            `json:"note"`
			RespondentAccess string            `json:"respondentAccess"`
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
			u, err = s.st.CreateUser(r.Context(), email, email, hash, "editor", strings.TrimSpace(item.Note))
			if err != nil {
				res["status"] = "error"
				res["error"] = "gagal membuat akun editor"
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
		res["editorId"] = u.ID
		respondentAccess := item.RespondentAccess
		if respondentAccess != "all" && respondentAccess != "selected" {
			respondentAccess = "all"
		}
		existing, err := s.st.GetEditorPermissionByEditorAndForm(r.Context(), u.ID, formID)
		if errors.Is(err, store.ErrNotFound) {
			p, cerr := s.st.CreateEditorPermission(r.Context(), u.ID, formID, respondentAccess, item.FieldFilters, &createdBy)
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
			p, uerr := s.st.UpdateEditorPermission(r.Context(), existing.ID, respondentAccess, item.FieldFilters)
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

// bulkDeleteEditorPermissions menghapus banyak permission editor sekaligus untuk satu kuesioner.
func (s *Server) bulkDeleteEditorPermissions(w http.ResponseWriter, r *http.Request) {
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
		perm, err := s.st.GetEditorPermissionByID(r.Context(), id)
		if err != nil || perm.FormID != formID {
			results = append(results, map[string]any{"id": id, "status": "error", "error": "permission tidak ditemukan"})
			continue
		}
		if err := s.st.DeleteEditorPermission(r.Context(), id); err != nil {
			results = append(results, map[string]any{"id": id, "status": "error", "error": "gagal menghapus"})
			continue
		}
		results = append(results, map[string]any{"id": id, "status": "deleted"})
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
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
	s.audit(r, "permission.editor.delete", "permission", perm.ID, perm.EditorName, perm.FormID, "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// convertEditorToViewer mengubah satu permission editor menjadi viewer untuk akun & kuesioner yang
// sama — membawa respondentAccess, fieldFilters, dan daftar responden terpilih ke permission baru
// (visibleFields dikosongkan = semua variabel terlihat, karena editor tidak punya konsep itu), lalu
// menghapus permission editor yang lama. Role akun (users.role) tidak diubah.
func (s *Server) convertEditorToViewer(w http.ResponseWriter, r *http.Request) {
	old, err := s.st.GetEditorPermissionByID(r.Context(), r.PathValue("permId"))
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
	if _, err := s.st.GetViewerPermission(r.Context(), old.EditorID, old.FormID); err == nil {
		writeErr(w, http.StatusConflict, "akun ini sudah memiliki akses viewer ke kuesioner ini")
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusInternalServerError, "gagal memeriksa akses")
		return
	}

	allowed, err := s.st.ListEditorAllowedRespondents(r.Context(), old.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}

	createdBy := userFrom(r.Context()).Subject
	newPerm, err := s.st.CreateViewerPermission(r.Context(), old.EditorID, old.FormID, old.RespondentAccess, nil, old.FieldFilters, &createdBy)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal membuat akses viewer")
		return
	}
	for _, a := range allowed {
		if _, err := s.st.AddViewerAllowedRespondent(r.Context(), newPerm.ID, a.RespondentID); err != nil {
			writeErr(w, http.StatusInternalServerError, "gagal menyalin daftar responden")
			return
		}
	}
	if err := s.st.DeleteEditorPermission(r.Context(), old.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal menghapus akses editor lama")
		return
	}
	writeJSON(w, http.StatusOK, newPerm)
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
	if _, err := s.st.GetEditorPermissionByEditorAndForm(r.Context(), editorID, formID); errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusForbidden, "tidak memiliki akses ke kuesioner ini")
		return
	} else if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	f := parseResponseFilter(q)
	resp, err := s.st.ListEditorResponses(r.Context(), editorID, formID, f, limit, offset)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	count, _ := s.st.CountEditorResponses(r.Context(), editorID, formID, f)
	writeJSON(w, http.StatusOK, map[string]any{"responses": s.signResponses(resp), "total": count})
}

// editorExportResponses menghasilkan CSV jawaban untuk form yang ditugaskan ke editor,
// dibatasi field_filters permission (membatasi baris, bukan kolom -- editor melihat semua kolom).
func (s *Server) editorExportResponses(w http.ResponseWriter, r *http.Request) {
	editorID := userFrom(r.Context()).Subject
	formID := r.PathValue("id")
	if _, err := s.st.GetEditorPermissionByEditorAndForm(r.Context(), editorID, formID); err != nil {
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
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\"responses-"+formID+".csv\"")
	cw := csv.NewWriter(w)
	_ = cw.Write(append(csvBaseHeader(true), cols...))
	n := 0
	_ = s.st.ForEachEditorResponse(r.Context(), editorID, formID, func(rr models.Response) error {
		writeCSVRow(cw, rr, cols, true)
		n++
		return nil
	})
	s.audit(r, "export.csv", "form", formID, "", formID, fmt.Sprintf("%d baris (editor)", n))
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
	writeJSON(w, http.StatusOK, s.signResponse(resp))
}

// editorUpdateResponse memperbarui jawaban satu respons oleh editor. Akses dicek terhadap
// respons yang SUDAH TERSIMPAN (respondentAccess + fieldFilters), bukan terhadap payload dari
// klien — payload bisa dipalsukan untuk melewati fieldFilters kalau dicek terhadap dirinya sendiri.
func (s *Server) editorUpdateResponse(w http.ResponseWriter, r *http.Request) {
	editorID := userFrom(r.Context()).Subject
	formID := r.PathValue("id")
	responseID := r.PathValue("responseId")

	// Nilai lama dipakai dua kali: memastikan editor memang berhak, dan disimpan
	// sebagai pembanding di riwayat perubahan.
	before, err := s.st.GetEditorResponseByID(r.Context(), editorID, formID, responseID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "respons tidak ditemukan atau akses tidak diizinkan")
			return
		}
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
	if err := s.st.UpdateResponseAnswers(r.Context(), formID, responseID, in.Answers); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "respons tidak ditemukan")
			return
		}
		writeErr(w, http.StatusInternalServerError, "gagal menyimpan")
		return
	}

	// Koreksi data harus bisa ditelusuri: simpan nilai sebelum & sesudah, lalu catat
	// aksinya di riwayat admin. Kegagalan mencatat tidak membatalkan penyimpanan yang
	// sudah berhasil, tapi tetap muncul di log server.
	u := userFrom(r.Context())
	rev := &models.ResponseRevision{
		ResponseID:    responseID,
		FormID:        formID,
		EditorID:      &editorID,
		EditorName:    u.Username,
		AnswersBefore: before.Answers,
		AnswersAfter:  in.Answers,
		IP:            s.clientIP(r),
	}
	if err := s.st.InsertResponseRevision(r.Context(), rev); err != nil {
		log.Printf("[revisi] gagal mencatat perubahan jawaban %s: %v", responseID, err)
	}
	changed := store.ChangedAnswerFields(before.Answers, in.Answers)
	s.audit(r, "response.edit", "response", responseID, "", formID,
		fmt.Sprintf("%d variabel diubah: %s", len(changed), strings.Join(changed, ", ")))

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// listResponseRevisions mengembalikan riwayat perubahan satu jawaban.
// Dibuka untuk admin pemilik kuesioner dan untuk editor yang berhak atas jawaban itu.
func (s *Server) listResponseRevisions(w http.ResponseWriter, r *http.Request) {
	formID := r.PathValue("id")
	responseID := r.PathValue("responseId")
	u := userFrom(r.Context())

	switch u.Role {
	case "superadmin", "admin":
		if _, ok := s.ensureFormAccess(w, r, formID); !ok {
			return
		}
	default:
		if _, err := s.st.GetEditorResponseByID(r.Context(), u.Subject, formID, responseID); err != nil {
			writeErr(w, http.StatusNotFound, "respons tidak ditemukan atau akses tidak diizinkan")
			return
		}
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	revs, err := s.st.ListResponseRevisions(r.Context(), responseID, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil data")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"revisions": revs})
}
