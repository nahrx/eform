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

	"github.com/bpskaltim/eform-backend/internal/auth"
	"github.com/bpskaltim/eform-backend/internal/models"
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

		// Tanda tangan yang sah belum cukup: akun bisa saja sudah dinonaktifkan,
		// dihapus, atau passwordnya diganti setelah token diterbitkan. Satu lookup
		// primary key per permintaan membuat pencabutan berlaku seketika.
		snap, err := s.st.GetAuthSnapshot(r.Context(), claims.Subject)
		if err != nil || !snap.IsActive || snap.TokenVersion != claims.TokenVersion {
			writeErr(w, http.StatusUnauthorized, "sesi tidak valid atau kedaluwarsa")
			return
		}
		// Role diambil dari DB, bukan dari token, supaya penurunan hak juga langsung
		// berlaku tanpa menunggu token baru.
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

/* ---------------- autentikasi API key (/api/v1) ---------------- */

const apiKeyPrefix = "eform_"

// hashAPIKey menghasilkan SHA-256 hex dari key yang dikirim klien.
//
// Sengaja berbeda dari password (bcrypt lewat auth.HashPassword). Bcrypt dibuat lambat
// untuk melawan tebakan pada rahasia berentropi rendah buatan manusia; API key di sini
// adalah 32 byte dari crypto/rand, jadi menebaknya mustahil dan biaya bcrypt hanya jadi
// beban di setiap permintaan. SHA-256 juga memungkinkan lookup O(1) lewat index key_hash.
func hashAPIKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// apiKeyFromContext mengambil API key yang sudah terverifikasi dari context.
func apiKeyFromContext(ctx context.Context) *models.FormAPIKey {
	k, _ := ctx.Value(apiKeyCtxKey).(*models.FormAPIKey)
	return k
}

// ipAllowed mencocokkan IP pemanggil dengan daftar izin. Entri boleh berupa satu IP
// ("103.10.1.5") atau CIDR ("103.10.1.0/24"). Daftar kosong berarti semua IP boleh.
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

// apiKeyMW memverifikasi API key untuk endpoint /api/v1 dan menerapkan seluruh
// pembatasannya: aktif, belum kedaluwarsa, IP diizinkan, dan kuota per menit.
//
// Semua penolakan memakai pesan generik supaya tidak bocor apakah suatu key ada.
// Setiap permintaan — termasuk yang ditolak — dicatat ke api_access_logs.
func (s *Server) apiKeyMW(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		raw = strings.TrimSpace(raw)

		// Prefix key dicatat apa adanya untuk audit; kalau formatnya salah, catat kosong.
		logPrefix := ""
		if strings.HasPrefix(raw, apiKeyPrefix) && len(raw) >= len(apiKeyPrefix)+10 {
			logPrefix = raw[len(apiKeyPrefix) : len(apiKeyPrefix)+10]
		}
		deny := func(status int, msg string) {
			s.logAPIAccess(r, nil, logPrefix, ip, status, 0, msg)
			writeErr(w, status, msg)
		}

		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") || !strings.HasPrefix(raw, apiKeyPrefix) {
			deny(http.StatusUnauthorized, "API key tidak valid")
			return
		}
		key, err := s.st.GetAPIKeyByHash(r.Context(), hashAPIKey(raw))
		if err != nil {
			deny(http.StatusUnauthorized, "API key tidak valid")
			return
		}
		if !key.IsActive {
			s.logAPIAccess(r, key, key.KeyPrefix, ip, http.StatusUnauthorized, 0, "key nonaktif")
			writeErr(w, http.StatusUnauthorized, "API key tidak valid")
			return
		}
		if key.ExpiresAt != nil && time.Now().After(*key.ExpiresAt) {
			s.logAPIAccess(r, key, key.KeyPrefix, ip, http.StatusUnauthorized, 0, "key kedaluwarsa")
			writeErr(w, http.StatusUnauthorized, "API key tidak valid")
			return
		}
		if !ipAllowed(ip, key.AllowedIPs) {
			s.logAPIAccess(r, key, key.KeyPrefix, ip, http.StatusForbidden, 0, "IP tidak diizinkan")
			writeErr(w, http.StatusForbidden, "akses dari alamat IP ini tidak diizinkan")
			return
		}
		quota := key.RateLimitPerMin
		if quota <= 0 {
			quota = 60
		}
		if !apiKeyRL.allow("api:"+key.ID, quota) {
			w.Header().Set("Retry-After", "60")
			s.logAPIAccess(r, key, key.KeyPrefix, ip, http.StatusTooManyRequests, 0, "kuota per menit terlampaui")
			writeErr(w, http.StatusTooManyRequests, "terlalu banyak permintaan, coba lagi dalam 1 menit")
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

// logAPIAccess menulis satu baris audit. Gagal mencatat tidak boleh menggagalkan
// permintaan, jadi errornya cuma di-log ke stdout.
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
		log.Printf("[api-audit] gagal mencatat akses %s: %v", r.URL.Path, err)
	}
}

/* ---------------- audit aksi admin ---------------- */

// audit mencatat satu aksi admin ke activity_logs. Pelaku diambil dari context, jadi
// pemanggil cukup menyebut aksi dan sasarannya.
//
// Sengaja best-effort: gagal mencatat tidak boleh menggagalkan aksi yang sudah
// terlanjur berhasil, tapi kegagalannya tetap muncul di log server.
func (s *Server) audit(r *http.Request, action, targetType, targetID, targetLabel, formID, detail string) {
	l := &models.ActivityLog{
		Action:      action,
		TargetType:  targetType,
		TargetID:    targetID,
		TargetLabel: targetLabel,
		IP:          clientIP(r),
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
		log.Printf("[audit] gagal mencatat %s: %v", action, err)
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

// slidingWindowLimiter membatasi jumlah kejadian per kunci dalam jendela satu menit.
// Kuncinya bebas: IP untuk percobaan login, "api:<keyID>" untuk pemakaian API key.
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

// allow mencatat satu kejadian untuk key dan mengembalikan false bila kuota per menit
// sudah terlampaui.
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

// loginRL membatasi percobaan login per IP (max 10 per menit).
var loginRL = newSlidingWindowLimiter()

// apiKeyRL membatasi pemakaian tiap API key; kuotanya per key (rate_limit_per_min).
var apiKeyRL = newSlidingWindowLimiter()

// publicRL membatasi endpoint pengisian kuesioner publik.
var publicRL = newSlidingWindowLimiter()

// limitRespondent membatasi laju per akun responden sekaligus per IP.
//
// Dua kunci dipakai bersamaan: batas per responden mencegah satu akun membanjiri,
// batas per IP mencegah satu mesin memakai banyak akun sekaligus.
func (s *Server) limitRespondent(next http.HandlerFunc, perRespondent, perIP int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		id := ""
		if rc := respondentFrom(r.Context()); rc != nil {
			id = rc.RespondentID
		}
		if id != "" && !publicRL.allow("resp:"+id, perRespondent) {
			w.Header().Set("Retry-After", "60")
			writeErr(w, http.StatusTooManyRequests, "terlalu banyak permintaan, coba lagi dalam 1 menit")
			return
		}
		if !publicRL.allow("pip:"+ip, perIP) {
			w.Header().Set("Retry-After", "60")
			writeErr(w, http.StatusTooManyRequests, "terlalu banyak permintaan dari jaringan ini, coba lagi dalam 1 menit")
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
