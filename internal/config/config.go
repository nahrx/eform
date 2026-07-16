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
	Port                 string
	DatabaseURL          string
	JWTSecret            []byte
	JWTRespondentSecret  []byte
	JWTTTL               time.Duration
	CORSOrigins          []string
	PublicBaseURL string // dipakai untuk membentuk URL share, mis. https://eform.bpskaltim.go.id
	WebDir        string // folder berisi login.html, admin.html, public.html, builder.html
	PublicDir     string // folder berisi landing page publik (index.html), disajikan di "/"
	Seed          SeedConfig

	// Google OAuth (untuk responden publik)
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
	// Muat .env (jika ada) sebelum membaca env apa pun.
	// File diabaikan kalau tidak ada; env asli OS tidak ditimpa.
	loadDotEnv(env("ENV_FILE", ".env"))

	c := &Config{
		Port:        env("PORT", "8080"),
		DatabaseURL: resolveDBURL(),
		PublicBaseURL: strings.TrimRight(env("PUBLIC_BASE_URL", "http://localhost:8080"), "/"),
		WebDir:        env("WEB_DIR", "web"),
		PublicDir:     env("PUBLIC_DIR", "public"),
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
		log.Println("[WARN] JWT_SECRET kosong — memakai secret acak (token akan invalid setelah restart). Set JWT_SECRET di produksi.")
	}
	c.JWTSecret = []byte(secret)

	respSecret := os.Getenv("JWT_RESPONDENT_SECRET")
	if respSecret == "" {
		b := make([]byte, 32)
		_, _ = rand.Read(b)
		respSecret = hex.EncodeToString(b)
		log.Println("[WARN] JWT_RESPONDENT_SECRET kosong — memakai secret acak terpisah. Set JWT_RESPONDENT_SECRET di produksi.")
	}
	c.JWTRespondentSecret = []byte(respSecret)

	ttlHours := 24
	if v := os.Getenv("JWT_TTL_HOURS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			ttlHours = n
		}
	}
	c.JWTTTL = time.Duration(ttlHours) * time.Hour

	origins := env("CORS_ORIGINS", "*")
	for _, o := range strings.Split(origins, ",") {
		if o = strings.TrimSpace(o); o != "" {
			c.CORSOrigins = append(c.CORSOrigins, o)
		}
	}
	return c
}

// resolveDBURL mengembalikan connection string PostgreSQL.
// Prioritas: DATABASE_URL (jika diset) → rakitan dari POSTGRES_* vars.
func resolveDBURL() string {
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return url
	}
	host := env("POSTGRES_HOST", "localhost")
	port := env("POSTGRES_PORT", "5432")
	user := env("POSTGRES_USER", "postgres")
	pass := env("POSTGRES_PASSWORD", "postgres")
	name := env("POSTGRES_DB", "eform")
	ssl  := env("POSTGRES_SSLMODE", "disable")
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", user, pass, host, port, name, ssl)
}

// resolveSuperadminPassword mengembalikan password superadmin dari env, atau
// men-generate password acak sekali pakai jika env tidak disetel (cetak ke stdout).
func resolveSuperadminPassword() string {
	if pw := os.Getenv("SUPERADMIN_PASSWORD"); pw != "" {
		return pw
	}
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	pw := hex.EncodeToString(b)
	log.Printf("[WARN] SUPERADMIN_PASSWORD tidak disetel — password sementara: %s (set env untuk produksi)", pw)
	return pw
}

// loadDotEnv memuat file .env (jika ada) memakai joho/godotenv.
// godotenv.Load TIDAK menimpa variabel yang sudah ada di environment OS,
// jadi env asli tetap diutamakan. File yang tidak ada diabaikan diam-diam.
func loadDotEnv(path string) {
	if err := godotenv.Load(path); err != nil {
		if !os.IsNotExist(err) {
			log.Printf("[config] gagal memuat %s: %v", path, err)
		}
		return
	}
	log.Printf("[config] env dimuat dari %s", path)
}
