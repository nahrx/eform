package httpapi

import (
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const maxPublicUploadSize = 10 << 20

var uploadExtRe = regexp.MustCompile(`^[a-z0-9]{1,8}$`)

func (s *Server) publicUpload(w http.ResponseWriter, r *http.Request) {
	sh, ok := s.resolveShare(w, r)
	if !ok {
		return
	}
	if !sh.AllowResponses {
		writeErr(w, http.StatusForbidden, "this link does not accept uploads")
		return
	}
	rc := respondentFrom(r.Context())

	if sh.AccessMode == "restricted" {
		allowed, err := s.st.IsEmailAllowed(r.Context(), sh.ID, rc.Email)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "server error")
			return
		}
		if !allowed {
			writeErr(w, http.StatusForbidden, "your email is not on the access list for this form")
			return
		}
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPublicUploadSize+1024)
	if err := r.ParseMultipartForm(maxPublicUploadSize); err != nil {
		writeErr(w, http.StatusBadRequest, "file is too large or the upload format is invalid")
		return
	}

	fieldType := strings.TrimSpace(strings.ToLower(r.FormValue("fieldType")))
	if fieldType == "" {
		fieldType = "file"
	}

	src, header, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "uploaded file not found")
		return
	}
	defer src.Close()

	data, err := io.ReadAll(io.LimitReader(src, maxPublicUploadSize+1))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to read file")
		return
	}
	if len(data) == 0 {
		writeErr(w, http.StatusBadRequest, "empty file")
		return
	}
	if len(data) > maxPublicUploadSize {
		writeErr(w, http.StatusBadRequest, "maximum file size is 10 MB")
		return
	}

	contentType := http.DetectContentType(data)
	if (fieldType == "photo" || fieldType == "signature") && !strings.HasPrefix(contentType, "image/") {
		writeErr(w, http.StatusBadRequest, "the file must be an image")
		return
	}

	relDir := filepath.ToSlash(filepath.Join("uploads", time.Now().Format("2006/01/02"), rc.RespondentID))
	absDir := filepath.Join(s.cfg.PublicDir, filepath.FromSlash(relDir))
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to prepare the upload folder")
		return
	}

	filename := randToken(10) + safeUploadExt(header.Filename, contentType)
	relPath := "/" + strings.TrimLeft(filepath.ToSlash(filepath.Join(relDir, filename)), "/")
	absPath := filepath.Join(s.cfg.PublicDir, filepath.FromSlash(strings.TrimPrefix(relPath, "/")))
	if err := os.WriteFile(absPath, data, 0o644); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to save file")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"url":         s.signUploadURL(relPath),
		"contentType": contentType,
		"size":        len(data),
	})
}

func safeUploadExt(filename, contentType string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), "."))
	if uploadExtRe.MatchString(ext) {
		return "." + ext
	}
	if exts, err := mime.ExtensionsByType(contentType); err == nil {
		for _, candidate := range exts {
			clean := strings.ToLower(strings.TrimPrefix(candidate, "."))
			if uploadExtRe.MatchString(clean) {
				return "." + clean
			}
		}
	}
	return defaultUploadExt(contentType)
}

func defaultUploadExt(contentType string) string {
	switch {
	case strings.HasPrefix(contentType, "image/jpeg"):
		return ".jpg"
	case strings.HasPrefix(contentType, "image/png"):
		return ".png"
	case strings.HasPrefix(contentType, "image/webp"):
		return ".webp"
	case strings.HasPrefix(contentType, "image/gif"):
		return ".gif"
	case strings.HasPrefix(contentType, "image/heic"):
		return ".heic"
	case strings.HasPrefix(contentType, "application/pdf"):
		return ".pdf"
	default:
		return ".bin"
	}
}
