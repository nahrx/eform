package httpapi

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/nahrx/eform/internal/config"
)

func TestIPAllowed(t *testing.T) {
	cases := []struct {
		nama    string
		ip      string
		allowed []string
		mau     bool
	}{
		{"daftar kosong = semua boleh", "103.10.1.5", nil, true},
		{"IP persis cocok", "103.10.1.5", []string{"103.10.1.5"}, true},
		{"IP tidak cocok", "103.10.1.6", []string{"103.10.1.5"}, false},
		{"CIDR mencakup", "103.10.2.77", []string{"103.10.2.0/24"}, true},
		{"CIDR tidak mencakup", "103.10.3.77", []string{"103.10.2.0/24"}, false},
		{"salah satu dari beberapa entri", "10.0.0.9", []string{"103.10.2.0/24", "10.0.0.9"}, true},
		{"spasi di sekitar entri tetap dikenali", "10.0.0.9", []string{"  10.0.0.9  "}, true},
		{"IPv6 loopback", "::1", []string{"::1"}, true},
		{"IPv6 lewat CIDR", "::1", []string{"::/0"}, true},
		{"IP pemanggil tidak bisa diurai", "bukan-ip", []string{"10.0.0.1"}, false},
		{"entri rusak diabaikan, tidak membuka akses", "10.0.0.1", []string{"ngawur"}, false},
	}
	for _, c := range cases {
		t.Run(c.nama, func(t *testing.T) {
			if got := ipAllowed(c.ip, c.allowed); got != c.mau {
				t.Fatalf("ipAllowed(%q,%v) = %v, mau %v", c.ip, c.allowed, got, c.mau)
			}
		})
	}
}

func TestHashAPIKey(t *testing.T) {
	k := apiKeyPrefix + "abcdefghijklmnop"
	h1, h2 := hashAPIKey(k), hashAPIKey(k)
	if h1 != h2 {
		t.Fatal("hash harus stabil untuk key yang sama")
	}
	if h1 == k || strings.Contains(h1, "abcdefghij") {
		t.Fatal("hash tidak boleh memuat key aslinya")
	}
	if len(h1) != 64 {
		t.Fatalf("SHA-256 hex harus 64 karakter, dapat %d", len(h1))
	}
	if hashAPIKey(k+"x") == h1 {
		t.Fatal("key berbeda harus menghasilkan hash berbeda")
	}
}

func TestSlidingWindowLimiter(t *testing.T) {
	l := &slidingWindowLimiter{attempts: map[string][]time.Time{}}

	for i := 0; i < 3; i++ {
		if !l.allow("k1", 3) {
			t.Fatalf("permintaan ke-%d masih dalam kuota, seharusnya lolos", i+1)
		}
	}
	if l.allow("k1", 3) {
		t.Fatal("permintaan ke-4 melebihi kuota, seharusnya ditolak")
	}
	// Kunci lain punya jatah sendiri.
	if !l.allow("k2", 3) {
		t.Fatal("kunci berbeda tidak boleh ikut terkena kuota kunci lain")
	}
}

func TestParseResponseFilter(t *testing.T) {
	t.Run("membaca semua jenis filter", func(t *testing.T) {
		q := url.Values{
			"status":       {"submitted"},
			"shareId":      {"s1"},
			"search":       {"  budi  "},
			"f_nama":       {"bud"},
			"fe_kabupaten": {"6472"},
			"fea_kategori": {"a,b"},
			"sortBy":       {"waktu"},
			"sortDir":      {"asc"},
		}
		f := parseResponseFilter(q)
		if f.Status != "submitted" || f.ShareID != "s1" {
			t.Errorf("status/share salah: %+v", f)
		}
		if f.Search != "budi" {
			t.Errorf("search harus dipangkas spasinya, dapat %q", f.Search)
		}
		if f.FieldFilters["nama"] != "bud" {
			t.Errorf("f_ tidak terbaca: %v", f.FieldFilters)
		}
		if f.FieldExactFilters["kabupaten"] != "6472" {
			t.Errorf("fe_ tidak terbaca: %v", f.FieldExactFilters)
		}
		if len(f.FieldAnyFilters["kategori"]) != 2 {
			t.Errorf("fea_ tidak terbaca: %v", f.FieldAnyFilters)
		}
	})

	t.Run("nilai kosong diabaikan", func(t *testing.T) {
		f := parseResponseFilter(url.Values{"f_nama": {"   "}, "fe_kab": {""}})
		if len(f.FieldFilters) != 0 || len(f.FieldExactFilters) != 0 {
			t.Fatalf("filter kosong seharusnya dibuang: %+v", f)
		}
	})

	t.Run("jumlah filter dibatasi", func(t *testing.T) {
		// Query string tidak boleh dipakai menyusun WHERE raksasa.
		q := url.Values{}
		for i := 0; i < 40; i++ {
			q.Set("f_kolom"+string(rune('a'+i%26))+string(rune('0'+i/26)), "x")
		}
		f := parseResponseFilter(q)
		if len(f.FieldFilters) > 10 {
			t.Fatalf("maksimal 10 filter per jenis, dapat %d", len(f.FieldFilters))
		}
	})
}

func TestCSVBaseHeader(t *testing.T) {
	dengan := csvBaseHeader(true)
	tanpa := csvBaseHeader(false)

	for _, kolom := range []string{"respondent_id", "nama", "email"} {
		if !contains(dengan, kolom) {
			t.Errorf("header dengan identitas harus memuat %q", kolom)
		}
		if contains(tanpa, kolom) {
			t.Errorf("header tanpa identitas TIDAK boleh memuat %q", kolom)
		}
	}
	// Kolom netral tetap ada di kedua bentuk.
	for _, kolom := range []string{"id", "status", "waktu_kirim"} {
		if !contains(dengan, kolom) || !contains(tanpa, kolom) {
			t.Errorf("%q harus ada di kedua bentuk header", kolom)
		}
	}
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func TestClientIPTanpaProxyTepercaya(t *testing.T) {
	// Tanpa daftar proxy, X-Forwarded-For tidak boleh dipercaya sama sekali.
	s := &Server{cfg: &config.Config{}}
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "203.0.113.9:1234"
	r.Header.Set("X-Forwarded-For", "1.2.3.4")
	if got := s.clientIP(r); got != "203.0.113.9" {
		t.Fatalf("harus memakai RemoteAddr, dapat %q", got)
	}
}

func TestClientIPDenganProxyTepercaya(t *testing.T) {
	s := &Server{cfg: &config.Config{TrustedProxies: []string{"10.0.0.0/8"}}}

	t.Run("koneksi dari proxy tepercaya -> pakai XFF", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = "10.0.0.5:1234"
		r.Header.Set("X-Forwarded-For", "203.0.113.7")
		if got := s.clientIP(r); got != "203.0.113.7" {
			t.Fatalf("dapat %q", got)
		}
	})

	t.Run("rantai proxy: ambil klien sebenarnya", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = "10.0.0.5:1234"
		// Klien menulis sendiri "9.9.9.9" untuk mengelabui; proxy menambahkan IP asli
		// di sebelah kanannya.
		r.Header.Set("X-Forwarded-For", "9.9.9.9, 203.0.113.7, 10.0.0.9")
		if got := s.clientIP(r); got != "203.0.113.7" {
			t.Fatalf("harus mengambil entri kanan yang bukan proxy, dapat %q", got)
		}
	})

	t.Run("koneksi dari IP tak tepercaya -> XFF diabaikan", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = "203.0.113.50:1234"
		r.Header.Set("X-Forwarded-For", "1.1.1.1")
		if got := s.clientIP(r); got != "203.0.113.50" {
			t.Fatalf("dapat %q", got)
		}
	})
}
