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

	"github.com/nahrx/eform/internal/auth"
	"github.com/nahrx/eform/internal/models"
	"github.com/nahrx/eform/internal/store"
)

// createEditorPermission grants an editor access to one form (superadmin only).
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
		writeErr(w, http.StatusBadRequest, "invalid request format")
		return
	}
	if in.EditorID == "" {
		writeErr(w, http.StatusBadRequest, "editorId is required")
		return
	}
	if in.RespondentAccess != "all" && in.RespondentAccess != "selected" {
		in.RespondentAccess = "all"
	}

	createdBy := userFrom(r.Context()).Subject
	p, err := s.st.CreateEditorPermission(r.Context(), in.EditorID, formID, in.RespondentAccess, in.FieldFilters, &createdBy)
	if err != nil {
		writeErr(w, http.StatusConflict, "the editor may already have access to this form")
		return
	}
	s.audit(r, "permission.editor.create", "permission", p.ID, in.EditorID, formID, "access="+in.RespondentAccess)
	writeJSON(w, http.StatusCreated, p)
}

// listFormEditorPermissions returns every editor with access to one form.
func (s *Server) listFormEditorPermissions(w http.ResponseWriter, r *http.Request) {
	formID := r.PathValue("id")
	if _, ok := s.ensureFormAccess(w, r, formID); !ok {
		return
	}
	perms, err := s.st.ListFormEditorPermissions(r.Context(), formID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"permissions": perms})
}

// exportEditorPermissionsCSV downloads one form's editor access list as CSV.
func (s *Server) exportEditorPermissionsCSV(w http.ResponseWriter, r *http.Request) {
	formID := r.PathValue("id")
	if _, ok := s.ensureFormAccess(w, r, formID); !ok {
		return
	}
	perms, err := s.st.ListFormEditorPermissions(r.Context(), formID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
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
		writeErr(w, http.StatusNotFound, "permission not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
		return
	}
	if _, ok := s.ensureFormAccess(w, r, p.FormID); !ok {
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// updateEditorPermission updates an editor permission's field_filters.
func (s *Server) updateEditorPermission(w http.ResponseWriter, r *http.Request) {
	perm, err := s.st.GetEditorPermissionByID(r.Context(), r.PathValue("permId"))
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "permission not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
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
		writeErr(w, http.StatusBadRequest, "invalid request format")
		return
	}
	if in.RespondentAccess != "all" && in.RespondentAccess != "selected" {
		in.RespondentAccess = "all"
	}
	p, err := s.st.UpdateEditorPermission(r.Context(), r.PathValue("permId"), in.RespondentAccess, in.FieldFilters)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "permission not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, "failed to update")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// listEditorAllowedRespondents returns every respondent allowed under one editor permission.
func (s *Server) listEditorAllowedRespondents(w http.ResponseWriter, r *http.Request) {
	perm, err := s.st.GetEditorPermissionByID(r.Context(), r.PathValue("permId"))
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "permission not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
		return
	}
	if _, ok := s.ensureFormAccess(w, r, perm.FormID); !ok {
		return
	}

	items, err := s.st.ListEditorAllowedRespondents(r.Context(), r.PathValue("permId"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"respondents": items})
}

// addEditorAllowedRespondent adds one respondent to an editor's allowed list.
func (s *Server) addEditorAllowedRespondent(w http.ResponseWriter, r *http.Request) {
	perm, err := s.st.GetEditorPermissionByID(r.Context(), r.PathValue("permId"))
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "permission not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
		return
	}
	if _, ok := s.ensureFormAccess(w, r, perm.FormID); !ok {
		return
	}

	var in struct {
		RespondentID string `json:"respondentId"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request format")
		return
	}
	if in.RespondentID == "" {
		writeErr(w, http.StatusBadRequest, "respondentId is required")
		return
	}
	item, err := s.st.AddEditorAllowedRespondent(r.Context(), r.PathValue("permId"), in.RespondentID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to add")
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

// removeEditorAllowedRespondent removes one respondent from an editor's allowed list.
func (s *Server) removeEditorAllowedRespondent(w http.ResponseWriter, r *http.Request) {
	item, err := s.st.GetEditorAllowedRespondentByID(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "data not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
		return
	}
	perm, err := s.st.GetEditorPermissionByID(r.Context(), item.PermissionID)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "permission not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
		return
	}
	if _, ok := s.ensureFormAccess(w, r, perm.FormID); !ok {
		return
	}

	if err := s.st.RemoveEditorAllowedRespondent(r.Context(), r.PathValue("id")); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "data not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, "failed to delete")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// bulkAssignEditorPermissions grants or updates editor access to one form
// for many accounts at once (accounts are created automatically if the email is not registered yet).
// Each row is independent — one failing row does not fail the others.
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
		writeErr(w, http.StatusBadRequest, "invalid request format")
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
			res["error"] = "email is required"
			results[i] = res
			continue
		}
		u, err := s.st.GetUserByUsername(r.Context(), email)
		if errors.Is(err, store.ErrNotFound) {
			b := make([]byte, 24)
			if _, rerr := rand.Read(b); rerr != nil {
				res["status"] = "error"
				res["error"] = "failed to generate a random password"
				results[i] = res
				continue
			}
			hash, herr := auth.HashPassword(base64.RawURLEncoding.EncodeToString(b))
			if herr != nil {
				res["status"] = "error"
				res["error"] = "failed to process password"
				results[i] = res
				continue
			}
			u, err = s.st.CreateUser(r.Context(), email, email, hash, "editor", strings.TrimSpace(item.Note))
			if err != nil {
				res["status"] = "error"
				res["error"] = "failed to create editor account"
				results[i] = res
				continue
			}
		} else if err != nil {
			res["status"] = "error"
			res["error"] = "failed to check the account"
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
				res["error"] = "failed to create access"
				results[i] = res
				continue
			}
			res["status"] = "created"
			res["permissionId"] = p.ID
		} else if err != nil {
			res["status"] = "error"
			res["error"] = "failed to check access"
			results[i] = res
			continue
		} else {
			p, uerr := s.st.UpdateEditorPermission(r.Context(), existing.ID, respondentAccess, item.FieldFilters)
			if uerr != nil {
				res["status"] = "error"
				res["error"] = "failed to update access"
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

// bulkDeleteEditorPermissions removes many editor permissions at once for one form.
func (s *Server) bulkDeleteEditorPermissions(w http.ResponseWriter, r *http.Request) {
	formID := r.PathValue("id")
	if _, ok := s.ensureFormAccess(w, r, formID); !ok {
		return
	}
	var in struct {
		PermissionIDs []string `json:"permissionIds"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request format")
		return
	}
	results := make([]map[string]any, 0, len(in.PermissionIDs))
	for _, id := range in.PermissionIDs {
		perm, err := s.st.GetEditorPermissionByID(r.Context(), id)
		if err != nil || perm.FormID != formID {
			results = append(results, map[string]any{"id": id, "status": "error", "error": "permission not found"})
			continue
		}
		if err := s.st.DeleteEditorPermission(r.Context(), id); err != nil {
			results = append(results, map[string]any{"id": id, "status": "error", "error": "failed to delete"})
			continue
		}
		results = append(results, map[string]any{"id": id, "status": "deleted"})
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

// deleteEditorPermission revokes an editor's access to one form.
func (s *Server) deleteEditorPermission(w http.ResponseWriter, r *http.Request) {
	perm, err := s.st.GetEditorPermissionByID(r.Context(), r.PathValue("permId"))
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "permission not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
		return
	}
	if _, ok := s.ensureFormAccess(w, r, perm.FormID); !ok {
		return
	}

	if err := s.st.DeleteEditorPermission(r.Context(), r.PathValue("permId")); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "permission not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, "failed to delete")
		return
	}
	s.audit(r, "permission.editor.delete", "permission", perm.ID, perm.EditorName, perm.FormID, "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// convertEditorToViewer turns one editor permission into a viewer permission for the same account &
// account & form — carrying respondentAccess, fieldFilters, and the selected respondents into the new permission
// (visibleFields is left empty = every field visible, since editors have no such concept), then
// removes the old editor permission. The account role (users.role) is left alone.
func (s *Server) convertEditorToViewer(w http.ResponseWriter, r *http.Request) {
	old, err := s.st.GetEditorPermissionByID(r.Context(), r.PathValue("permId"))
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "permission not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
		return
	}
	if _, ok := s.ensureFormAccess(w, r, old.FormID); !ok {
		return
	}
	if _, err := s.st.GetViewerPermission(r.Context(), old.EditorID, old.FormID); err == nil {
		writeErr(w, http.StatusConflict, "this account already has viewer access to this form")
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusInternalServerError, "failed to check access")
		return
	}

	allowed, err := s.st.ListEditorAllowedRespondents(r.Context(), old.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
		return
	}

	createdBy := userFrom(r.Context()).Subject
	newPerm, err := s.st.CreateViewerPermission(r.Context(), old.EditorID, old.FormID, old.RespondentAccess, nil, old.FieldFilters, &createdBy)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to create viewer access")
		return
	}
	for _, a := range allowed {
		if _, err := s.st.AddViewerAllowedRespondent(r.Context(), newPerm.ID, a.RespondentID); err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to copy the respondent list")
			return
		}
	}
	if err := s.st.DeleteEditorPermission(r.Context(), old.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to remove the previous editor access")
		return
	}
	writeJSON(w, http.StatusOK, newPerm)
}

// editorMyForms returns the forms assigned to the signed-in editor.
func (s *Server) editorMyForms(w http.ResponseWriter, r *http.Request) {
	editorID := userFrom(r.Context()).Subject
	forms, err := s.st.ListFormsByEditor(r.Context(), editorID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"forms": forms})
}

// editorGetForm returns the form schema for an editor who holds a permission.
func (s *Server) editorGetForm(w http.ResponseWriter, r *http.Request) {
	editorID := userFrom(r.Context()).Subject
	formID := r.PathValue("id")
	ok, err := s.st.HasEditorFormPermission(r.Context(), editorID, formID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
		return
	}
	if !ok {
		writeErr(w, http.StatusForbidden, "you do not have access to this form")
		return
	}
	f, err := s.st.GetForm(r.Context(), formID)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "form not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
		return
	}
	writeJSON(w, http.StatusOK, f)
}

// editorListResponses returns every response (submitted & draft) for a form assigned to an editor.
func (s *Server) editorListResponses(w http.ResponseWriter, r *http.Request) {
	editorID := userFrom(r.Context()).Subject
	formID := r.PathValue("id")
	if _, err := s.st.GetEditorPermissionByEditorAndForm(r.Context(), editorID, formID); errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusForbidden, "you do not have access to this form")
		return
	} else if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
		return
	}
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	f := parseResponseFilter(q)
	resp, err := s.st.ListEditorResponses(r.Context(), editorID, formID, f, limit, offset)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
		return
	}
	count, _ := s.st.CountEditorResponses(r.Context(), editorID, formID, f)
	writeJSON(w, http.StatusOK, map[string]any{"responses": s.signResponses(resp), "total": count})
}

// editorExportResponses produces a CSV of the responses for a form assigned to an editor,
// restricted by the permission's field_filters (rows, not columns — editors see every column).
func (s *Server) editorExportResponses(w http.ResponseWriter, r *http.Request) {
	editorID := userFrom(r.Context()).Subject
	formID := r.PathValue("id")
	if _, err := s.st.GetEditorPermissionByEditorAndForm(r.Context(), editorID, formID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusForbidden, "you do not have access to this form")
		} else {
			writeErr(w, http.StatusInternalServerError, "failed to check access")
		}
		return
	}
	cols, err := s.st.GetFormAnswerColumns(r.Context(), formID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
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
	s.audit(r, "export.csv", "form", formID, "", formID, fmt.Sprintf("%d rows (editor)", n))
}

// editorGetResponse returns the details of one response for an editor, with the field_filters check.
func (s *Server) editorGetResponse(w http.ResponseWriter, r *http.Request) {
	editorID := userFrom(r.Context()).Subject
	formID := r.PathValue("id")
	responseID := r.PathValue("responseId")

	resp, err := s.st.GetEditorResponseByID(r.Context(), editorID, formID, responseID)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "response not found or access is not allowed")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
		return
	}
	writeJSON(w, http.StatusOK, s.signResponse(resp))
}

// editorUpdateResponse updates one response's answers on behalf of an editor. Access is checked against
// the response AS ALREADY STORED (respondentAccess + fieldFilters), never against the payload from
// the client — a payload could be forged to slip past fieldFilters if it were checked against itself.
func (s *Server) editorUpdateResponse(w http.ResponseWriter, r *http.Request) {
	editorID := userFrom(r.Context()).Subject
	formID := r.PathValue("id")
	responseID := r.PathValue("responseId")

	// The old values serve twice: they confirm the editor is entitled, and they are stored
	// sebagai pembanding di riwayat perubahan.
	before, err := s.st.GetEditorResponseByID(r.Context(), editorID, formID, responseID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "response not found or access is not allowed")
			return
		}
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
		return
	}

	var in struct {
		Answers json.RawMessage `json:"answers"`
		Status  string          `json:"status"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request format")
		return
	}
	if len(in.Answers) == 0 {
		writeErr(w, http.StatusBadRequest, "answers is required")
		return
	}
	if in.Status != "draft" && in.Status != "submitted" {
		writeErr(w, http.StatusBadRequest, "invalid status")
		return
	}
	if err := s.st.UpdateResponseAnswers(r.Context(), formID, responseID, in.Answers); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "response not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, "failed to save")
		return
	}

	// Data corrections must stay traceable: store the before & after values, then record
	// the action in the admin history. A logging failure does not undo the save that has
	// already succeeded, but it is still surfaced in the server log.
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
		log.Printf("[revision] failed to record answer change %s: %v", responseID, err)
	}
	changed := store.ChangedAnswerFields(before.Answers, in.Answers)
	s.audit(r, "response.edit", "response", responseID, "", formID,
		fmt.Sprintf("%d fields changed: %s", len(changed), strings.Join(changed, ", ")))

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// listResponseRevisions returns one answer's change history.
// Open to the form's owning admin and to any editor entitled to that response.
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
			writeErr(w, http.StatusNotFound, "response not found or access is not allowed")
			return
		}
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	revs, err := s.st.ListResponseRevisions(r.Context(), responseID, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"revisions": revs})
}

// editorExportResponsesXLSX downloads the responses as Excel for an editor.
func (s *Server) editorExportResponsesXLSX(w http.ResponseWriter, r *http.Request) {
	editorID := userFrom(r.Context()).Subject
	formID := r.PathValue("id")

	if _, err := s.st.GetEditorPermissionByEditorAndForm(r.Context(), editorID, formID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusForbidden, "you do not have access to this form")
		} else {
			writeErr(w, http.StatusInternalServerError, "failed to check access")
		}
		return
	}
	cols, err := s.st.GetFormAnswerColumns(r.Context(), formID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
		return
	}
	n := s.streamXLSX(w, formID, "Responses", cols, true, func(fn func(models.Response) error) error {
		return s.st.ForEachEditorResponse(r.Context(), editorID, formID, fn)
	})
	s.audit(r, "export.xlsx", "form", formID, "", formID, fmt.Sprintf("%d rows (editor)", n))
}
