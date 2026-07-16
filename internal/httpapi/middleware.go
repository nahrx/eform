package httpapi

import (
	"context"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bpskaltim/eform-backend/internal/auth"
)

type ctxKey string

const userKey ctxKey = "user"
const respondentKey ctxKey = "respondent"

func userFrom(ctx context.Context) *auth.Claims {
	c, _ := ctx.Value(userKey).(*auth.Claims)
	return c
}

func respondentFrom(ctx context.Context) *auth.RespondentClaims {
	c, _ := ctx.Value(respondentKey).(*auth.RespondentClaims)
	return c
}

// chain middleware terluar: recover -> log -> cors -> securityHeaders -> mux
func (s *Server) wrap(h http.Handler) http.Handler {
	return s.recoverMW(s.logMW(s.corsMW(s.securityHeadersMW(h))))
}

func (s *Server) securityHeadersMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) corsMW(next http.Handler) http.Handler {
	allowAll := len(s.cfg.CORSOrigins) == 1 && s.cfg.CORSOrigins[0] == "*"
	allowed := map[string]bool{}
	for _, o := range s.cfg.CORSOrigins {
		allowed[o] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && (allowAll || allowed[origin]) {
			if allowAll {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Add("Vary", "Origin")
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type,X-Share-Password")
			w.Header().Set("Access-Control-Max-Age", "86400")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) logMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(sw, r)
		u := r.URL.Path
		if r.URL.RawQuery != "" {
			u += "?" + r.URL.RawQuery
		}
		log.Printf("%s %s -> %d (%s)", r.Method, u, sw.status, time.Since(start).Round(time.Millisecond))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(c int) { w.status = c; w.ResponseWriter.WriteHeader(c) }

func (s *Server) recoverMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("[panic] %s %s: %v", r.Method, r.URL.Path, rec)
				writeErr(w, http.StatusInternalServerError, "kesalahan server")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// authMW memverifikasi Bearer token admin dan menaruh claims di context.
func (s *Server) authMW(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			writeErr(w, http.StatusUnauthorized, "perlu login")
			return
		}
		claims, err := s.auth.Parse(strings.TrimPrefix(h, "Bearer "))
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "sesi tidak valid atau kedaluwarsa")
			return
		}
		// Tolak token respondent yang salah endpoint
		if claims.Username == "" {
			writeErr(w, http.StatusUnauthorized, "token tidak valid untuk endpoint ini")
			return
		}
		ctx := context.WithValue(r.Context(), userKey, claims)
		next(w, r.WithContext(ctx))
	}
}

// respondentMW memverifikasi Bearer token respondent (Google OAuth).
func (s *Server) respondentMW(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			writeErr(w, http.StatusUnauthorized, "perlu login Google")
			return
		}
		claims, err := s.auth.ParseRespondent(strings.TrimPrefix(h, "Bearer "))
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "sesi tidak valid atau kedaluwarsa")
			return
		}
		ctx := context.WithValue(r.Context(), respondentKey, claims)
		next(w, r.WithContext(ctx))
	}
}

// requireRole membatasi akses ke salah satu role yang diizinkan.
func (s *Server) requireRole(next http.HandlerFunc, roles ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := userFrom(r.Context())
		if u == nil {
			writeErr(w, http.StatusUnauthorized, "perlu login")
			return
		}
		for _, role := range roles {
			if u.Role == role {
				next(w, r)
				return
			}
		}
		writeErr(w, http.StatusForbidden, "akses ditolak")
	}
}

// loginLimiter membatasi percobaan login per IP (max 10 per menit).
type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
}

var loginRL = &loginLimiter{attempts: make(map[string][]time.Time)}

func init() {
	go func() {
		for range time.Tick(5 * time.Minute) {
			loginRL.mu.Lock()
			cutoff := time.Now().Add(-time.Minute)
			for ip, ts := range loginRL.attempts {
				var valid []time.Time
				for _, t := range ts {
					if t.After(cutoff) {
						valid = append(valid, t)
					}
				}
				if len(valid) == 0 {
					delete(loginRL.attempts, ip)
				} else {
					loginRL.attempts[ip] = valid
				}
			}
			loginRL.mu.Unlock()
		}
	}()
}

func (l *loginLimiter) allow(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := time.Now().Add(-time.Minute)
	var valid []time.Time
	for _, t := range l.attempts[host] {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	if len(valid) >= 10 {
		l.attempts[host] = valid
		return false
	}
	l.attempts[host] = append(valid, time.Now())
	return true
}
