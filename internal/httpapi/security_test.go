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
		name    string
		ip      string
		allowed []string
		mau     bool
	}{
		{"empty list = everything allowed", "103.10.1.5", nil, true},
		{"IP persis cocok", "103.10.1.5", []string{"103.10.1.5"}, true},
		{"IP does not match", "103.10.1.6", []string{"103.10.1.5"}, false},
		{"CIDR mencakup", "103.10.2.77", []string{"103.10.2.0/24"}, true},
		{"CIDR does not cover it", "103.10.3.77", []string{"103.10.2.0/24"}, false},
		{"one of several entries", "10.0.0.9", []string{"103.10.2.0/24", "10.0.0.9"}, true},
		{"surrounding whitespace is still recognised", "10.0.0.9", []string{"  10.0.0.9  "}, true},
		{"IPv6 loopback", "::1", []string{"::1"}, true},
		{"IPv6 via CIDR", "::1", []string{"::/0"}, true},
		{"caller IP cannot be parsed", "not-an-ip", []string{"10.0.0.1"}, false},
		{"a malformed entry is ignored, never grants access", "10.0.0.1", []string{"ngawur"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
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
		t.Fatal("the hash must be stable for the same key")
	}
	if h1 == k || strings.Contains(h1, "abcdefghij") {
		t.Fatal("the hash must not contain the original key")
	}
	if len(h1) != 64 {
		t.Fatalf("SHA-256 hex must be 64 characters, got %d", len(h1))
	}
	if hashAPIKey(k+"x") == h1 {
		t.Fatal("different keys must produce different hashes")
	}
}

func TestSlidingWindowLimiter(t *testing.T) {
	l := &slidingWindowLimiter{attempts: map[string][]time.Time{}}

	for i := 0; i < 3; i++ {
		if !l.allow("k1", 3) {
			t.Fatalf("request %d is still within quota and should pass", i+1)
		}
	}
	if l.allow("k1", 3) {
		t.Fatal("the 4th request exceeds the quota and should be rejected")
	}
	// A different key has its own quota.
	if !l.allow("k2", 3) {
		t.Fatal("a different key must not be affected by another key quota")
	}
}

func TestParseResponseFilter(t *testing.T) {
	t.Run("parses every filter kind", func(t *testing.T) {
		q := url.Values{
			"status":       {"submitted"},
			"shareId":      {"s1"},
			"search":       {"  budi  "},
			"f_name":       {"bud"},
			"fe_kabupaten": {"6472"},
			"fea_kategori": {"a,b"},
			"sortBy":       {"time"},
			"sortDir":      {"asc"},
		}
		f := parseResponseFilter(q)
		if f.Status != "submitted" || f.ShareID != "s1" {
			t.Errorf("status/share wrong: %+v", f)
		}
		if f.Search != "budi" {
			t.Errorf("search should be whitespace-trimmed, got %q", f.Search)
		}
		if f.FieldFilters["name"] != "bud" {
			t.Errorf("f_ was not parsed: %v", f.FieldFilters)
		}
		if f.FieldExactFilters["kabupaten"] != "6472" {
			t.Errorf("fe_ was not parsed: %v", f.FieldExactFilters)
		}
		if len(f.FieldAnyFilters["kategori"]) != 2 {
			t.Errorf("fea_ was not parsed: %v", f.FieldAnyFilters)
		}
	})

	t.Run("empty values are ignored", func(t *testing.T) {
		f := parseResponseFilter(url.Values{"f_nama": {"   "}, "fe_kab": {""}})
		if len(f.FieldFilters) != 0 || len(f.FieldExactFilters) != 0 {
			t.Fatalf("empty filters should be dropped: %+v", f)
		}
	})

	t.Run("the number of filters is capped", func(t *testing.T) {
		// A query string must not be usable to build a gigantic WHERE clause.
		q := url.Values{}
		for i := 0; i < 40; i++ {
			q.Set("f_col"+string(rune('a'+i%26))+string(rune('0'+i/26)), "x")
		}
		f := parseResponseFilter(q)
		if len(f.FieldFilters) > 10 {
			t.Fatalf("at most 10 filters per kind, got %d", len(f.FieldFilters))
		}
	})
}

func TestCSVBaseHeader(t *testing.T) {
	withID := csvBaseHeader(true)
	withoutID := csvBaseHeader(false)

	for _, col := range []string{"respondent_id", "name", "email"} {
		if !contains(withID, col) {
			t.Errorf("the header with identity must contain %q", col)
		}
		if contains(withoutID, col) {
			t.Errorf("the identity-free header must NOT contain %q", col)
		}
	}
	// Neutral columns appear in both variants.
	for _, col := range []string{"id", "status", "submitted_at"} {
		if !contains(withID, col) || !contains(withoutID, col) {
			t.Errorf("%q must appear in both header variants", col)
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

func TestClientIPWithoutTrustedProxy(t *testing.T) {
	// Without a proxy list, X-Forwarded-For must not be trusted at all.
	s := &Server{cfg: &config.Config{}}
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "203.0.113.9:1234"
	r.Header.Set("X-Forwarded-For", "1.2.3.4")
	if got := s.clientIP(r); got != "203.0.113.9" {
		t.Fatalf("should use RemoteAddr, got %q", got)
	}
}

func TestClientIPWithTrustedProxy(t *testing.T) {
	s := &Server{cfg: &config.Config{TrustedProxies: []string{"10.0.0.0/8"}}}

	t.Run("connection from a trusted proxy -> use XFF", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = "10.0.0.5:1234"
		r.Header.Set("X-Forwarded-For", "203.0.113.7")
		if got := s.clientIP(r); got != "203.0.113.7" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("proxy chain: pick the real client", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = "10.0.0.5:1234"
		// The client writes "9.9.9.9" itself to spoof; the proxy appends the real IP
		// di sebelah kanannya.
		r.Header.Set("X-Forwarded-For", "9.9.9.9, 203.0.113.7, 10.0.0.9")
		if got := s.clientIP(r); got != "203.0.113.7" {
			t.Fatalf("should take the right-most non-proxy entry, got %q", got)
		}
	})

	t.Run("connection from an untrusted IP -> XFF ignored", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = "203.0.113.50:1234"
		r.Header.Set("X-Forwarded-For", "1.1.1.1")
		if got := s.clientIP(r); got != "203.0.113.50" {
			t.Fatalf("got %q", got)
		}
	})
}
