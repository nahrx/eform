package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/nahrx/eform/internal/auth"
	"github.com/nahrx/eform/internal/models"
)

type ctxKey string

const userKey ctxKey = "user"
const respondentKey ctxKey = "respondent"
const apiKeyCtxKey ctxKey = "apiKey"

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
			"default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob: https://tile.openstreetmap.org https://*.googleusercontent.com; font-src 'self'; connect-src 'self'; frame-ancestors 'none'")
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
				writeErr(w, http.StatusInternalServerError, "server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// authMW verifies the admin Bearer token and puts the claims into the context.
func (s *Server) authMW(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			writeErr(w, http.StatusUnauthorized, "login required")
			return
		}
		claims, err := s.auth.Parse(strings.TrimPrefix(h, "Bearer "))
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "session is invalid or has expired")
			return
		}
		// Reject a respondent token used on the wrong endpoint
		if claims.Username == "" {
			writeErr(w, http.StatusUnauthorized, "invalid token for this endpoint")
			return
		}

		// A valid signature is not enough: the account may have been deactivated,
		// deleted, or had its password changed since the token was issued. A single lookup
		// by primary key on every request makes revocation take effect immediately.
		snap, err := s.st.GetAuthSnapshot(r.Context(), claims.Subject)
		if err != nil || !snap.IsActive || snap.TokenVersion != claims.TokenVersion {
			writeErr(w, http.StatusUnauthorized, "session is invalid or has expired")
			return
		}
		// The role comes from the database, not the token, so a downgrade also takes effect
		// applies without waiting for a new token.
		claims.Role = snap.Role

		ctx := context.WithValue(r.Context(), userKey, claims)
		next(w, r.WithContext(ctx))
	}
}

// respondentMW memverifikasi Bearer token respondent (Google OAuth).
func (s *Server) respondentMW(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			writeErr(w, http.StatusUnauthorized, "Google login required")
			return
		}
		claims, err := s.auth.ParseRespondent(strings.TrimPrefix(h, "Bearer "))
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "session is invalid or has expired")
			return
		}
		ctx := context.WithValue(r.Context(), respondentKey, claims)
		next(w, r.WithContext(ctx))
	}
}

/* ---------------- API key authentication (/api/v1) ---------------- */

const apiKeyPrefix = "eform_"

// hashAPIKey produces the SHA-256 hex digest of the key sent by the client.
//
// Deliberately different from passwords (bcrypt via auth.HashPassword). Bcrypt is slow by
// design to resist guessing of low-entropy human-chosen secrets; an API key here is
// 32 bytes from crypto/rand, so guessing is infeasible and bcrypt's cost would only be
// overhead on every request. SHA-256 also allows an O(1) lookup through the key_hash index.
func hashAPIKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// apiKeyFromContext retrieves the already-verified API key from the context.
func apiKeyFromContext(ctx context.Context) *models.FormAPIKey {
	k, _ := ctx.Value(apiKeyCtxKey).(*models.FormAPIKey)
	return k
}

// ipAllowed matches the caller's IP against the allowlist. An entry may be a single IP
// ("103.10.1.5") or a CIDR ("103.10.1.0/24"). An empty list means any IP is allowed.
func ipAllowed(ip string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, entry := range allowed {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if strings.Contains(entry, "/") {
			if _, netw, err := net.ParseCIDR(entry); err == nil && netw.Contains(parsed) {
				return true
			}
			continue
		}
		if other := net.ParseIP(entry); other != nil && other.Equal(parsed) {
			return true
		}
	}
	return false
}

// apiKeyMW verifies the API key for the /api/v1 endpoints and enforces all of its
// restrictions: active, not expired, IP allowed, and within the per-minute quota.
//
// Every rejection uses a generic message so it never reveals whether a key exists.
// Every request — including rejected ones — is recorded in api_access_logs.
func (s *Server) apiKeyMW(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := s.clientIP(r)
		raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		raw = strings.TrimSpace(raw)

		// The key prefix is recorded as-is for the audit trail; on a malformed key, record it empty.
		logPrefix := ""
		if strings.HasPrefix(raw, apiKeyPrefix) && len(raw) >= len(apiKeyPrefix)+10 {
			logPrefix = raw[len(apiKeyPrefix) : len(apiKeyPrefix)+10]
		}
		deny := func(status int, msg string) {
			s.logAPIAccess(r, nil, logPrefix, ip, status, 0, msg)
			writeErr(w, status, msg)
		}

		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") || !strings.HasPrefix(raw, apiKeyPrefix) {
			deny(http.StatusUnauthorized, "invalid API key")
			return
		}
		key, err := s.st.GetAPIKeyByHash(r.Context(), hashAPIKey(raw))
		if err != nil {
			deny(http.StatusUnauthorized, "invalid API key")
			return
		}
		if !key.IsActive {
			s.logAPIAccess(r, key, key.KeyPrefix, ip, http.StatusUnauthorized, 0, "key nonaktif")
			writeErr(w, http.StatusUnauthorized, "invalid API key")
			return
		}
		if key.ExpiresAt != nil && time.Now().After(*key.ExpiresAt) {
			s.logAPIAccess(r, key, key.KeyPrefix, ip, http.StatusUnauthorized, 0, "key kedaluwarsa")
			writeErr(w, http.StatusUnauthorized, "invalid API key")
			return
		}
		if !ipAllowed(ip, key.AllowedIPs) {
			s.logAPIAccess(r, key, key.KeyPrefix, ip, http.StatusForbidden, 0, "IP address is not allowed")
			writeErr(w, http.StatusForbidden, "access from this IP address is not allowed")
			return
		}
		quota := key.RateLimitPerMin
		if quota <= 0 {
			quota = 60
		}
		if !apiKeyRL.allow("api:"+key.ID, quota) {
			w.Header().Set("Retry-After", "60")
			s.logAPIAccess(r, key, key.KeyPrefix, ip, http.StatusTooManyRequests, 0, "kuota per menit terlampaui")
			writeErr(w, http.StatusTooManyRequests, "too many requests, please try again in 1 minute")
			return
		}

		go func(id, ip string) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = s.st.TouchAPIKey(ctx, id, ip)
		}(key.ID, ip)

		next(w, r.WithContext(context.WithValue(r.Context(), apiKeyCtxKey, key)))
	}
}

// logAPIAccess writes one audit row. A logging failure must never fail
// the request, so the error is only logged to stdout.
func (s *Server) logAPIAccess(r *http.Request, key *models.FormAPIKey, prefix, ip string, status, rowCount int, errMsg string) {
	l := &models.APIAccessLog{
		KeyPrefix: prefix,
		IP:        ip,
		Path:      r.URL.Path,
		Query:     r.URL.RawQuery,
		Status:    status,
		RowCount:  rowCount,
		Error:     errMsg,
	}
	if key != nil {
		l.APIKeyID = &key.ID
		l.FormID = &key.FormID
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.st.InsertAPIAccessLog(ctx, l); err != nil {
		log.Printf("[api-audit] failed to record access %s: %v", r.URL.Path, err)
	}
}

/* ---------------- audit aksi admin ---------------- */

// audit records one admin action in activity_logs. The actor comes from the context, so
// the caller only needs to name the action and its target.
//
// Deliberately best-effort: a logging failure must not fail an action that has already
// already succeeded, but the failure is still surfaced in the server log.
func (s *Server) audit(r *http.Request, action, targetType, targetID, targetLabel, formID, detail string) {
	l := &models.ActivityLog{
		Action:      action,
		TargetType:  targetType,
		TargetID:    targetID,
		TargetLabel: targetLabel,
		IP:          s.clientIP(r),
		Detail:      detail,
	}
	if u := userFrom(r.Context()); u != nil {
		id := u.Subject
		l.ActorID = &id
		l.ActorName = u.Username
		l.ActorRole = u.Role
	}
	if formID != "" {
		l.FormID = &formID
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.st.InsertActivityLog(ctx, l); err != nil {
		log.Printf("[audit] failed to record %s: %v", action, err)
	}
}

// requireRole restricts access to one of the permitted roles.
func (s *Server) requireRole(next http.HandlerFunc, roles ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := userFrom(r.Context())
		if u == nil {
			writeErr(w, http.StatusUnauthorized, "login required")
			return
		}
		for _, role := range roles {
			if u.Role == role {
				next(w, r)
				return
			}
		}
		writeErr(w, http.StatusForbidden, "access denied")
	}
}

// slidingWindowLimiter caps the number of events per key within a one-minute window.
// The key is arbitrary: an IP for login attempts, "api:<keyID>" for API key usage.
type slidingWindowLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
}

func newSlidingWindowLimiter() *slidingWindowLimiter {
	l := &slidingWindowLimiter{attempts: make(map[string][]time.Time)}
	go func() {
		for range time.Tick(5 * time.Minute) {
			l.sweep()
		}
	}()
	return l
}

func (l *slidingWindowLimiter) sweep() {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := time.Now().Add(-time.Minute)
	for k, ts := range l.attempts {
		var valid []time.Time
		for _, t := range ts {
			if t.After(cutoff) {
				valid = append(valid, t)
			}
		}
		if len(valid) == 0 {
			delete(l.attempts, k)
		} else {
			l.attempts[k] = valid
		}
	}
}

// allow records one event for the key and returns false when the per-minute quota
// has been exceeded.
func (l *slidingWindowLimiter) allow(key string, max int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := time.Now().Add(-time.Minute)
	var valid []time.Time
	for _, t := range l.attempts[key] {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	if len(valid) >= max {
		l.attempts[key] = valid
		return false
	}
	l.attempts[key] = append(valid, time.Now())
	return true
}

// loginRL limits login attempts per IP (max 10 per minute).
var loginRL = newSlidingWindowLimiter()

// apiKeyRL limits each API key's usage; the quota is per key (rate_limit_per_min).
var apiKeyRL = newSlidingWindowLimiter()

// publicRL rate-limits the public form-filling endpoints.
var publicRL = newSlidingWindowLimiter()

// limitRespondent rate-limits per respondent account and per IP at the same time.
//
// Two keys are used together: the per-respondent limit stops a single account flooding,
// the per-IP cap stops one machine from cycling through many accounts.
func (s *Server) limitRespondent(next http.HandlerFunc, perRespondent, perIP int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := s.clientIP(r)
		id := ""
		if rc := respondentFrom(r.Context()); rc != nil {
			id = rc.RespondentID
		}
		if id != "" && !publicRL.allow("resp:"+id, perRespondent) {
			w.Header().Set("Retry-After", "60")
			writeErr(w, http.StatusTooManyRequests, "too many requests, please try again in 1 minute")
			return
		}
		if !publicRL.allow("pip:"+ip, perIP) {
			w.Header().Set("Retry-After", "60")
			writeErr(w, http.StatusTooManyRequests, "too many requests from this network, please try again in 1 minute")
			return
		}
		next(w, r)
	}
}

func (l *slidingWindowLimiter) allowRequest(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return l.allow(host, 10)
}
