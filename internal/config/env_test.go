package config

import (
	"os"
	"path/filepath"
	"testing"
)

// Loading .env via godotenv: values are read, inline comments & the export prefix
// handled, and a few keys are set.
func TestLoadDotEnv(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	content := "# komentar\nPORT=9999\nPUBLIC_DIR=mypublic # inline\nexport CORS_ORIGINS=https://a.test,https://b.test\n"
	if err := os.WriteFile(envFile, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"PORT", "PUBLIC_DIR", "CORS_ORIGINS"} {
		os.Unsetenv(k)
	}
	t.Cleanup(func() {
		for _, k := range []string{"PORT", "PUBLIC_DIR", "CORS_ORIGINS"} {
			os.Unsetenv(k)
		}
	})
	t.Setenv("ENV_FILE", envFile)

	cfg := Load()
	if cfg.Port != "9999" {
		t.Errorf("Port = %q; mau 9999", cfg.Port)
	}
	if cfg.PublicDir != "mypublic" {
		t.Errorf("PublicDir = %q; want mypublic (the inline comment should be trimmed)", cfg.PublicDir)
	}
	if len(cfg.CORSOrigins) != 2 || cfg.CORSOrigins[0] != "https://a.test" {
		t.Errorf("CORSOrigins = %v; mau [https://a.test https://b.test]", cfg.CORSOrigins)
	}
}

// Real OS environment variables must win over values in .env.
func TestOSEnvWinsOverDotEnv(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	if err := os.WriteFile(envFile, []byte("PORT=9999\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ENV_FILE", envFile)
	t.Setenv("PORT", "7777")

	if cfg := Load(); cfg.Port != "7777" {
		t.Errorf("Port = %q; mau 7777 (env OS menang)", cfg.Port)
	}
}

// A missing .env file must not make Load fail.
func TestMissingDotEnvOK(t *testing.T) {
	t.Setenv("ENV_FILE", filepath.Join(t.TempDir(), "does-not-exist.env"))
	if cfg := Load(); cfg == nil {
		t.Fatal("Load() returned nil even though .env is absent")
	}
}
