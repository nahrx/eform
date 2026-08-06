package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/url"

	"github.com/nahrx/eform/internal/models"
	"strconv"
	"strings"
	"time"
)

/* Signed links to attachment files.

   /uploads/... used to be served with no checks at all: the file names are random,
   but anyone who ever held a link could download it forever, and that link was not
   tied to the viewer/editor/API key restrictions in any way. In other words, the
   column-masking and row-filtering rules simply did not apply to attachments.

   Now: the permission check happens once, when the response is served, and that
   decision is then "carried" by a short-lived HMAC signature attached to the URL.
   This shape was chosen because attachments are rendered through <img src> and
   <a href>, neither of which can send an Authorization header. */

// uploadURLTTL is how long an attachment link stays valid. Long enough to open and
// read one response page, short enough that a leaked link expires quickly.
const uploadURLTTL = 2 * time.Hour

// uploadSigKey derives the signing key from the JWT secret, so operators do not have
// to configure yet another secret.
func (s *Server) uploadSigKey() []byte {
	sum := sha256.Sum256(append([]byte("eform-upload-sig|"), s.cfg.JWTSecret...))
	return sum[:]
}

func (s *Server) uploadSig(path string, exp int64) string {
	mac := hmac.New(sha256.New, s.uploadSigKey())
	mac.Write([]byte(path))
	mac.Write([]byte("|"))
	mac.Write([]byte(strconv.FormatInt(exp, 10)))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// signUploadURL menambahkan parameter kedaluwarsa + tanda tangan ke path lampiran.
// Values that are not /uploads/ paths are returned unchanged.
func (s *Server) signUploadURL(raw string) string {
	if !isUploadPath(raw) {
		return raw
	}
	// Drop any existing query so signatures do not stack up when data is passed through twice.
	path := raw
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path = path[:i]
	}
	exp := time.Now().Add(uploadURLTTL).Unix()
	return path + "?e=" + strconv.FormatInt(exp, 10) + "&s=" + s.uploadSig(path, exp)
}

// verifyUploadURL checks the signature on a file request.
func (s *Server) verifyUploadURL(path string, q url.Values) bool {
	expStr, sig := q.Get("e"), q.Get("s")
	if expStr == "" || sig == "" {
		return false
	}
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return false
	}
	// hmac.Equal rather than ==, so comparison timing cannot leak the signature.
	return hmac.Equal([]byte(sig), []byte(s.uploadSig(path, exp)))
}

func isUploadPath(v string) bool {
	return strings.HasPrefix(v, "/uploads/")
}

/* ---- signing attachments inside an answer ---- */

// signAnswerUploads rewrites every /uploads/ path inside the answer JSON into a
// signed URL. Called RIGHT AFTER authorisation, at the point the answer is serialised.
//
// The answer structure is free-form (values may be strings, arrays, or objects), so the walk
// is recursive and only touches strings that look like upload paths.
func (s *Server) signAnswerUploads(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return raw
	}
	out, changed := s.signAny(v)
	if !changed {
		return raw
	}
	b, err := json.Marshal(out)
	if err != nil {
		return raw
	}
	return b
}

// signResponse signs the attachments on a single response.
func (s *Server) signResponse(rr *models.Response) *models.Response {
	if rr != nil {
		rr.Answers = s.signAnswerUploads(rr.Answers)
	}
	return rr
}

// signResponses signs the attachments on a set of responses.
func (s *Server) signResponses(rows []models.Response) []models.Response {
	for i := range rows {
		rows[i].Answers = s.signAnswerUploads(rows[i].Answers)
	}
	return rows
}

func (s *Server) signAny(v any) (any, bool) {
	switch t := v.(type) {
	case string:
		if isUploadPath(t) {
			return s.signUploadURL(t), true
		}
	case []any:
		changed := false
		for i, item := range t {
			nv, c := s.signAny(item)
			if c {
				t[i] = nv
				changed = true
			}
		}
		return t, changed
	case map[string]any:
		changed := false
		for k, item := range t {
			nv, c := s.signAny(item)
			if c {
				t[k] = nv
				changed = true
			}
		}
		return t, changed
	}
	return v, false
}
