package httpapi

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/bpskaltim/eform-backend/internal/auth"
	"github.com/bpskaltim/eform-backend/internal/config"
	"github.com/bpskaltim/eform-backend/internal/store"
)

type Server struct {
	cfg  *config.Config
	st   *store.Store
	auth *auth.Manager
}

func New(cfg *config.Config, st *store.Store, am *auth.Manager) *Server {
	return &Server{cfg: cfg, st: st, auth: am}
}

/* ---------- helpers ---------- */

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 8<<20)) // batas 8 MB
	return dec.Decode(v)
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 60 {
		s = s[:60]
	}
	if s == "" {
		s = "kuesioner"
	}
	return s
}

// randToken menghasilkan string acak URL-safe (base32 tanpa padding).
func randToken(nbytes int) string {
	b := make([]byte, nbytes)
	_, _ = rand.Read(b)
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b))
}
