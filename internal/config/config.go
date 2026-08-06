package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                string
	DatabaseURL         string
	JWTSecret           []byte
	JWTRespondentSecret []byte
	JWTTTL              time.Duration
	CORSOrigins         []string
	// TrustedProxies: reverse-proxy IPs/CIDRs whose X-Forwarded-For header may be trusted.
	TrustedProxies []string
	// LogRetentionDays: maximum age of rows in activity_logs & api_access_logs.
	LogRetentionDays int
	PublicBaseURL    string // used to build share URLs, e.g. https://eform.bpskaltim.go.id
	WebDir           string // folder holding login.html, admin.html, public.html, builder.html
	PublicDir        string // folder holding the public landing page (index.html), served at "/"

	// Organisation identity shown on the pages. The values are injected into the
	// HTML as it is served (see httpapi/pages.go), so another organisation can
	// use this application without editing a single file.
	OrganisationName     string // full name, e.g. "BPS Provinsi Kalimantan Timur"
	OrganisationNickname string // short name for the tab title, e.g. "BPS Kaltim"

	Seed SeedConfig

	// Google OAuth (for public respondents)
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string // default: {PublicBaseURL}/auth/google/callback
}

type SeedConfig struct {
	Username string
	Email    string
	Password string
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func Load() *Config {
	// Load .env (if present) before reading any environment variable.
	// A missing file is ignored; real OS environment variables are never overwritten.
	loadDotEnv(env("ENV_FILE", ".env"))

	c := &Config{
		Port:          env("PORT", "8080"),
		DatabaseURL:   resolveDBURL(),
		PublicBaseURL: strings.TrimRight(env("PUBLIC_BASE_URL", "http://localhost:8080"), "/"),
		WebDir:        env("WEB_DIR", "web"),
		PublicDir:     env("PUBLIC_DIR", "public"),
		// The defaults deliberately carry the values that used to be hard-coded in the
		// HTML, so an existing deployment looks unchanged while the environment
		// variables are not set yet.
		OrganisationName:     env("ORGANISATION_NAME", "BPS Provinsi Kalimantan Timur"),
		OrganisationNickname: env("ORGANISATION_NICKNAME", "BPS Kaltim"),
		Seed: SeedConfig{
			Username: env("SUPERADMIN_USERNAME", "admin"),
			Email:    env("SUPERADMIN_EMAIL", "admin@bps.go.id"),
			Password: resolveSuperadminPassword(),
		},
		GoogleClientID:     env("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: env("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL:  env("GOOGLE_REDIRECT_URL", ""),
	}

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		b := make([]byte, 32)
		_, _ = rand.Read(b)
		secret = hex.EncodeToString(b)
		log.Println("[WARN] JWT_SECRET is empty — using a random secret (tokens become invalid after a restart). Set JWT_SECRET in production.")
	}
	c.JWTSecret = []byte(secret)

	respSecret := os.Getenv("JWT_RESPONDENT_SECRET")
	if respSecret == "" {
		b := make([]byte, 32)
		_, _ = rand.Read(b)
		respSecret = hex.EncodeToString(b)
		log.Println("[WARN] JWT_RESPONDENT_SECRET is empty — using a separate random secret. Set JWT_RESPONDENT_SECRET in production.")
	}
	c.JWTRespondentSecret = []byte(respSecret)

	ttlHours := 24
	if v := os.Getenv("JWT_TTL_HOURS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			ttlHours = n
		}
	}
	c.JWTTTL = time.Duration(ttlHours) * time.Hour

	// The default is deliberately PublicBaseURL alone, not "*". The application serves its
	// own UI from the same origin and therefore needs no open CORS; if access from another
	// origin really is required, set CORS_ORIGINS explicitly.
	origins := env("CORS_ORIGINS", c.PublicBaseURL)
	for _, o := range strings.Split(origins, ",") {
		if o = strings.TrimSpace(o); o != "" {
			c.CORSOrigins = append(c.CORSOrigins, o)
		}
	}
	for _, p := range strings.Split(env("TRUSTED_PROXIES", ""), ",") {
		if p = strings.TrimSpace(p); p != "" {
			c.TrustedProxies = append(c.TrustedProxies, p)
		}
	}

	c.LogRetentionDays = 180
	if v := os.Getenv("LOG_RETENTION_DAYS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			c.LogRetentionDays = n
		}
	}

	if len(c.CORSOrigins) == 1 && c.CORSOrigins[0] == "*" {
		log.Println("[WARN] CORS_ORIGINS=* allows every origin — list the origins explicitly in production.")
	}
	return c
}

// resolveDBURL returns the PostgreSQL connection string.
// Priority: DATABASE_URL (when set) → assembled from the POSTGRES_* variables.
func resolveDBURL() string {
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return url
	}
	host := env("POSTGRES_HOST", "localhost")
	port := env("POSTGRES_PORT", "5432")
	user := env("POSTGRES_USER", "postgres")
	pass := env("POSTGRES_PASSWORD", "postgres")
	name := env("POSTGRES_DB", "eform")
	ssl := env("POSTGRES_SSLMODE", "disable")
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", user, pass, host, port, name, ssl)
}

// resolveSuperadminPassword returns the superadmin password from the environment, or
// generates a one-off random password when it is unset (printed to stdout).
func resolveSuperadminPassword() string {
	if pw := os.Getenv("SUPERADMIN_PASSWORD"); pw != "" {
		return pw
	}
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	pw := hex.EncodeToString(b)
	log.Printf("[WARN] SUPERADMIN_PASSWORD is not set — temporary password: %s (set the env var in production)", pw)
	return pw
}

// loadDotEnv loads the .env file (if present) using joho/godotenv.
// godotenv.Load does NOT overwrite variables that already exist in the OS environment,
// so real environment variables keep priority. A missing file is ignored silently.
func loadDotEnv(path string) {
	if err := godotenv.Load(path); err != nil {
		if !os.IsNotExist(err) {
			log.Printf("[config] failed to load %s: %v", path, err)
		}
		return
	}
	log.Printf("[config] env loaded from %s", path)
}
