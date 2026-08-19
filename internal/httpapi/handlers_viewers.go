package httpapi

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/nahrx/eform/internal/auth"
	"github.com/nahrx/eform/internal/models"
	"github.com/nahrx/eform/internal/store"
)

/* ================================================================
   SUPERADMIN — manage viewer permissions per form
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
		writeErr(w, http.StatusBadRequest, "invalid request format")
		return
	}
	if in.ViewerID == "" {
		writeErr(w, http.StatusBadRequest, "viewerId is required")
		return
	}
	if in.RespondentAccess != "all" && in.RespondentAccess != "selected" {
		in.RespondentAccess = "all"
	}
	createdBy := userFrom(r.Context()).Subject
	p, err := s.st.CreateViewerPermission(r.Context(), in.ViewerID, formID, in.RespondentAccess, in.VisibleFields, in.FieldFilters, &createdBy)
	if err != nil {
		writeErr(w, http.StatusConflict, "the viewer may already have access to this form")
		return
	}
	s.audit(r, "permission.viewer.create", "permission", p.ID, in.ViewerID, formID, "access="+in.RespondentAccess)
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) listFormViewerPermissions(w http.ResponseWriter, r *http.Request) {
	formID := r.PathValue("id")
	if _, ok := s.ensureFormAccess(w, r, formID); !ok {
		return
	}
	perms, err := s.st.ListFormViewerPermissions(r.Context(), formID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"permissions": perms})
}

// exportViewerPermissionsCSV downloads one form's viewer access list as CSV
// (for archiving/editing in Excel — not for direct re-import, because field_filters
// may hold more than one field).
func (s *Server) exportViewerPermissionsCSV(w http.ResponseWriter, r *http.Request) {
	formID := r.PathValue("id")
	if _, ok := s.ensureFormAccess(w, r, formID); !ok {
		return
	}
	perms, err := s.st.ListFormViewerPermissions(r.Context(), formID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
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

func (s *Server) updateViewerPermission(w http.ResponseWriter, r *http.Request) {
	perm, err := s.st.GetViewerPermissionByID(r.Context(), r.PathValue("permId"))
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
		VisibleFields    []string          `json:"visibleFields"`
		FieldFilters     map[string]string `json:"fieldFilters"`
		// A pointer so that leaving the field out means "unchanged" rather than
		// "clear it" — the note belongs to the account, and another caller updating
		// only the access rules must not wipe it as a side effect.
		Note *string `json:"note"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request format")
		return
	}
	if in.RespondentAccess != "all" && in.RespondentAccess != "selected" {
		in.RespondentAccess = "all"
	}
	// The note lives on the user account, so this changes what every form shows for
	// them — which is what it is for: it says who the person is, not what they may see
	// here. Access to the form is what authorises the edit (ensureFormAccess above).
	if in.Note != nil {
		if err := s.st.UpdateUserNote(r.Context(), perm.ViewerID, strings.TrimSpace(*in.Note)); err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to save the note")
			return
		}
	}
	p, err := s.st.UpdateViewerPermission(r.Context(), r.PathValue("permId"), in.RespondentAccess, in.VisibleFields, in.FieldFilters)
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

func (s *Server) deleteViewerPermission(w http.ResponseWriter, r *http.Request) {
	perm, err := s.st.GetViewerPermissionByID(r.Context(), r.PathValue("permId"))
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

	if err := s.st.DeleteViewerPermission(r.Context(), r.PathValue("permId")); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "permission not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, "failed to delete")
		return
	}
	s.audit(r, "permission.viewer.delete", "permission", perm.ID, perm.ViewerUsername, perm.FormID, "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// convertViewerToEditor turns one viewer permission into an editor permission for the same account &
// account & form — carrying respondentAccess, fieldFilters, and the selected respondents into the new permission,
// form, then removes the old viewer permission. The account role (users.role) is left alone — the role is no longer
// an exclusive gate, only a default label; the real capability comes from this permission table.
func (s *Server) convertViewerToEditor(w http.ResponseWriter, r *http.Request) {
	old, err := s.st.GetViewerPermissionByID(r.Context(), r.PathValue("permId"))
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
	if _, err := s.st.GetEditorPermissionByEditorAndForm(r.Context(), old.ViewerID, old.FormID); err == nil {
		writeErr(w, http.StatusConflict, "this account already has editor access to this form")
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusInternalServerError, "failed to check access")
		return
	}

	allowed, err := s.st.ListViewerAllowedRespondents(r.Context(), old.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
		return
	}

	createdBy := userFrom(r.Context()).Subject
	newPerm, err := s.st.CreateEditorPermission(r.Context(), old.ViewerID, old.FormID, old.RespondentAccess, old.FieldFilters, &createdBy)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to create editor access")
		return
	}
	for _, a := range allowed {
		if _, err := s.st.AddEditorAllowedRespondent(r.Context(), newPerm.ID, a.RespondentID); err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to copy the respondent list")
			return
		}
	}
	if err := s.st.DeleteViewerPermission(r.Context(), old.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to remove the previous viewer access")
		return
	}
	writeJSON(w, http.StatusOK, newPerm)
}

/* ================================================================
   SUPERADMIN — bulk assign/remove viewer permissions (many at once)
   ================================================================ */

// bulkAssignViewerPermissions grants or updates viewer access to one form
// for many accounts at once (accounts are created automatically if the email is not registered yet).
// Each row is independent — one failing row does not fail the others.
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
			u, err = s.st.CreateUser(r.Context(), email, email, hash, "viewer", strings.TrimSpace(item.Note))
			if err != nil {
				res["status"] = "error"
				res["error"] = "failed to create viewer account"
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
			res["error"] = "the email is registered as an admin account"
			results[i] = res
			continue
		} else if n := strings.TrimSpace(item.Note); n != "" {
			// The account already existed, so CreateUser above did not run and the note
			// typed in the dialog would otherwise be dropped. Only a non-empty note is
			// written: adding someone again without filling the box must not erase what
			// is already recorded about them.
			if nerr := s.st.UpdateUserNote(r.Context(), u.ID, n); nerr != nil {
				res["status"] = "error"
				res["error"] = "failed to save the note"
				results[i] = res
				continue
			}
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
			p, uerr := s.st.UpdateViewerPermission(r.Context(), existing.ID, respondentAccess, item.VisibleFields, item.FieldFilters)
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

// bulkDeleteViewerPermissions removes many viewer permissions at once for one form.
func (s *Server) bulkDeleteViewerPermissions(w http.ResponseWriter, r *http.Request) {
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
		perm, err := s.st.GetViewerPermissionByID(r.Context(), id)
		if err != nil || perm.FormID != formID {
			results = append(results, map[string]any{"id": id, "status": "error", "error": "permission not found"})
			continue
		}
		if err := s.st.DeleteViewerPermission(r.Context(), id); err != nil {
			results = append(results, map[string]any{"id": id, "status": "error", "error": "failed to delete"})
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

	items, err := s.st.ListViewerAllowedRespondents(r.Context(), r.PathValue("permId"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"respondents": items})
}

func (s *Server) addViewerAllowedRespondent(w http.ResponseWriter, r *http.Request) {
	perm, err := s.st.GetViewerPermissionByID(r.Context(), r.PathValue("permId"))
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
	item, err := s.st.AddViewerAllowedRespondent(r.Context(), r.PathValue("permId"), in.RespondentID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to add")
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) removeViewerAllowedRespondent(w http.ResponseWriter, r *http.Request) {
	item, err := s.st.GetViewerAllowedRespondentByID(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "data not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
		return
	}
	perm, err := s.st.GetViewerPermissionByID(r.Context(), item.PermissionID)
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

	if err := s.st.RemoveViewerAllowedRespondent(r.Context(), r.PathValue("id")); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "data not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, "failed to delete")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// listFormRespondents is used by a superadmin to pick respondents when configuring 'selected' mode.
func (s *Server) listFormRespondents(w http.ResponseWriter, r *http.Request) {
	formID := r.PathValue("id")
	if _, ok := s.ensureFormAccess(w, r, formID); !ok {
		return
	}
	respondents, err := s.st.ListFormRespondents(r.Context(), formID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"respondents": respondents})
}

/* ================================================================
   VIEWER — the endpoints a viewer calls after signing in
   ================================================================ */

// viewerMyForms returns every form the signed-in viewer may see.
func (s *Server) viewerMyForms(w http.ResponseWriter, r *http.Request) {
	viewerID := userFrom(r.Context()).Subject
	perms, err := s.st.ListViewerForms(r.Context(), viewerID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"forms": perms})
}

// viewerMyFormPermission returns the viewer's permission details for one form.
func (s *Server) viewerMyFormPermission(w http.ResponseWriter, r *http.Request) {
	viewerID := userFrom(r.Context()).Subject
	formID := r.PathValue("id")
	perm, err := s.st.GetViewerPermission(r.Context(), viewerID, formID)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusForbidden, "you do not have access to this form")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
		return
	}
	writeJSON(w, http.StatusOK, perm)
}

// viewerGetForm returns the form data for a viewer who holds a permission.
func (s *Server) viewerGetForm(w http.ResponseWriter, r *http.Request) {
	viewerID := userFrom(r.Context()).Subject
	formID := r.PathValue("id")
	if _, err := s.st.GetViewerPermission(r.Context(), viewerID, formID); errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusForbidden, "you do not have access to this form")
		return
	} else if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
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

// viewerListResponses serves the response list a viewer may see, with the restrictions applied.
func (s *Server) viewerListResponses(w http.ResponseWriter, r *http.Request) {
	viewerID := userFrom(r.Context()).Subject
	formID := r.PathValue("id")

	// Make sure the viewer actually has access
	if _, err := s.st.GetViewerPermission(r.Context(), viewerID, formID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusForbidden, "you do not have access to this form")
		} else {
			writeErr(w, http.StatusInternalServerError, "failed to check access")
		}
		return
	}

	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	f := parseResponseFilter(q)

	resp, err := s.st.ListViewerResponses(r.Context(), viewerID, formID, f, limit, offset)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch data")
		return
	}
	count, _ := s.st.CountViewerResponses(r.Context(), viewerID, formID, f)
	writeJSON(w, http.StatusOK, map[string]any{"responses": s.signResponses(resp), "total": count})
}

// viewerExportResponses produces a CSV of the responses a viewer may see, honouring the restrictions
// its permission (respondentAccess, fieldFilters for rows, visibleFields for columns).
func (s *Server) viewerExportResponses(w http.ResponseWriter, r *http.Request) {
	viewerID := userFrom(r.Context()).Subject
	formID := r.PathValue("id")

	perm, err := s.st.GetViewerPermission(r.Context(), viewerID, formID)
	if err != nil {
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
	n := 0
	_ = s.st.ForEachViewerResponse(r.Context(), viewerID, formID, func(rr models.Response) error {
		writeCSVRow(cw, rr, cols, true)
		n++
		return nil
	})
	s.audit(r, "export.csv", "form", formID, "", formID, fmt.Sprintf("%d rows (viewer)", n))
}

// viewerGetResponse returns the details of one response a viewer may see.
func (s *Server) viewerGetResponse(w http.ResponseWriter, r *http.Request) {
	viewerID := userFrom(r.Context()).Subject
	formID := r.PathValue("id")
	responseID := r.PathValue("responseId")

	resp, err := s.st.GetViewerResponseByID(r.Context(), viewerID, formID, responseID)
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

// viewerExportResponsesXLSX downloads the responses as Excel, honouring the restrictions
// the viewer's permission exactly as its CSV export does.
func (s *Server) viewerExportResponsesXLSX(w http.ResponseWriter, r *http.Request) {
	viewerID := userFrom(r.Context()).Subject
	formID := r.PathValue("id")

	perm, err := s.st.GetViewerPermission(r.Context(), viewerID, formID)
	if err != nil {
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
	cols = keepVisibleColumns(cols, perm.VisibleFields)

	n := s.streamXLSX(w, formID, "Responses", cols, true, func(fn func(models.Response) error) error {
		return s.st.ForEachViewerResponse(r.Context(), viewerID, formID, fn)
	})
	s.audit(r, "export.xlsx", "form", formID, "", formID, fmt.Sprintf("%d rows (viewer)", n))
}

// keepVisibleColumns filters the column list down to the fields that may
// be seen. An empty list means every column is allowed.
func keepVisibleColumns(cols, visible []string) []string {
	if len(visible) == 0 {
		return cols
	}
	allowed := make(map[string]bool, len(visible))
	for _, f := range visible {
		allowed[f] = true
	}
	out := make([]string, 0, len(cols))
	for _, c := range cols {
		if allowed[c] {
			out = append(out, c)
		}
	}
	return out
}
