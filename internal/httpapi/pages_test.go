package httpapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nahrx/eform/internal/config"
)

func orgServer(name, nickname string) *Server {
	return &Server{cfg: &config.Config{
		OrganisationName:     name,
		OrganisationNickname: nickname,
	}}
}

func writeHTML(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "page.html")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func serveOnce(t *testing.T, s *Server, path string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	s.serveHTML(w, httptest.NewRequest(http.MethodGet, "/", nil), path)
	return w
}

func TestServeHTMLSubstitutesOrganisationPlaceholders(t *testing.T) {
	s := orgServer("BPS Provinsi Jawa Barat", "BPS Jabar")
	p := writeHTML(t, "<title>eForm · {{ORG_NAME}}</title><h1>eForm {{ORG_NICK}}</h1>")

	body := serveOnce(t, s, p).Body.String()

	if !strings.Contains(body, "eForm · BPS Provinsi Jawa Barat") {
		t.Errorf("organisation name was not substituted: %q", body)
	}
	if !strings.Contains(body, "eForm BPS Jabar") {
		t.Errorf("organisation nickname was not substituted: %q", body)
	}
	// Most important of all: a raw placeholder must never reach the browser.
	if strings.Contains(body, "{{ORG_") {
		t.Errorf("an unsubstituted placeholder remains: %q", body)
	}
}

// A placeholder can appear many times in one file (public.html uses them in four
// places), so every occurrence must be substituted.
func TestServeHTMLSubstitutesEveryOccurrence(t *testing.T) {
	s := orgServer("Organisation X", "IX")
	p := writeHTML(t, "{{ORG_NAME}} a {{ORG_NAME}} b {{ORG_NICK}} c {{ORG_NICK}}")

	body := serveOnce(t, s, p).Body.String()

	if got := strings.Count(body, "Organisation X"); got != 2 {
		t.Errorf("organisation name appears %d times, want 2 — %q", got, body)
	}
	if got := strings.Count(body, "IX"); got != 2 {
		t.Errorf("organisation nickname appears %d times, want 2 — %q", got, body)
	}
}

// The cache is keyed by the file modTime; editing HTML during development must show
// up immediately without restarting the server.
func TestServeHTMLRefreshesCacheWhenFileChanges(t *testing.T) {
	s := orgServer("Organisation X", "IX")
	p := writeHTML(t, "<p>version one {{ORG_NICK}}</p>")

	if body := serveOnce(t, s, p).Body.String(); !strings.Contains(body, "version one") {
		t.Fatalf("initial content was not served: %q", body)
	}

	if err := os.WriteFile(p, []byte("<p>version two {{ORG_NICK}}</p>"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Make sure the modTime really differs, even though the file is small and written fast.
	newer := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(p, newer, newer); err != nil {
		t.Fatal(err)
	}

	body := serveOnce(t, s, p).Body.String()
	if !strings.Contains(body, "version two") {
		t.Errorf("file change was not picked up, stale cache: %q", body)
	}
	if strings.Contains(body, "{{ORG_") {
		t.Errorf("placeholder was not substituted after the cache refreshed: %q", body)
	}
}

func TestServeHTMLMissingFileIs404(t *testing.T) {
	s := orgServer("Organisation X", "IX")
	w := serveOnce(t, s, filepath.Join(t.TempDir(), "does-not-exist.html"))
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}
