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

func serverInstansi(nama, panggilan string) *Server {
	return &Server{cfg: &config.Config{
		OrganisationName:     nama,
		OrganisationNickname: panggilan,
	}}
}

func tulisHTML(t *testing.T, isi string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "halaman.html")
	if err := os.WriteFile(p, []byte(isi), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func sajikan(t *testing.T, s *Server, path string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	s.serveHTML(w, httptest.NewRequest(http.MethodGet, "/", nil), path)
	return w
}

func TestServeHTMLMenggantiPlaceholderInstansi(t *testing.T) {
	s := serverInstansi("BPS Provinsi Jawa Barat", "BPS Jabar")
	p := tulisHTML(t, "<title>eForm · {{ORG_NAME}}</title><h1>eForm {{ORG_NICK}}</h1>")

	body := sajikan(t, s, p).Body.String()

	if !strings.Contains(body, "eForm · BPS Provinsi Jawa Barat") {
		t.Errorf("nama instansi tidak tersubstitusi: %q", body)
	}
	if !strings.Contains(body, "eForm BPS Jabar") {
		t.Errorf("nama panggilan tidak tersubstitusi: %q", body)
	}
	// Yang paling penting: placeholder mentah tidak boleh sampai ke browser.
	if strings.Contains(body, "{{ORG_") {
		t.Errorf("masih ada placeholder yang belum diganti: %q", body)
	}
}

// Tiap placeholder bisa muncul berkali-kali dalam satu berkas (public.html
// memakainya di empat tempat), jadi semuanya harus ikut terganti.
func TestServeHTMLMenggantiSemuaKemunculan(t *testing.T) {
	s := serverInstansi("Instansi X", "IX")
	p := tulisHTML(t, "{{ORG_NAME}} a {{ORG_NAME}} b {{ORG_NICK}} c {{ORG_NICK}}")

	body := sajikan(t, s, p).Body.String()

	if got := strings.Count(body, "Instansi X"); got != 2 {
		t.Errorf("nama instansi muncul %d kali, ingin 2 — %q", got, body)
	}
	if got := strings.Count(body, "IX"); got != 2 {
		t.Errorf("nama panggilan muncul %d kali, ingin 2 — %q", got, body)
	}
}

// Cache dikunci modTime berkas; menyunting HTML saat pengembangan harus langsung
// terlihat tanpa perlu me-restart server.
func TestServeHTMLMenyegarkanCacheSaatBerkasBerubah(t *testing.T) {
	s := serverInstansi("Instansi X", "IX")
	p := tulisHTML(t, "<p>versi satu {{ORG_NICK}}</p>")

	if body := sajikan(t, s, p).Body.String(); !strings.Contains(body, "versi satu") {
		t.Fatalf("isi awal tidak tersaji: %q", body)
	}

	if err := os.WriteFile(p, []byte("<p>versi dua {{ORG_NICK}}</p>"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Pastikan modTime benar-benar berbeda walau berkasnya kecil dan cepat ditulis.
	baru := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(p, baru, baru); err != nil {
		t.Fatal(err)
	}

	body := sajikan(t, s, p).Body.String()
	if !strings.Contains(body, "versi dua") {
		t.Errorf("perubahan berkas tidak terbaca, cache basi: %q", body)
	}
	if strings.Contains(body, "{{ORG_") {
		t.Errorf("placeholder tidak diganti setelah cache disegarkan: %q", body)
	}
}

func TestServeHTMLBerkasTidakAdaJadi404(t *testing.T) {
	s := serverInstansi("Instansi X", "IX")
	w := sajikan(t, s, filepath.Join(t.TempDir(), "tidak-ada.html"))
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, ingin 404", w.Code)
	}
}
