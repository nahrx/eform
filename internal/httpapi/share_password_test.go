package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nahrx/eform/internal/config"
	"github.com/nahrx/eform/internal/models"
)

func pwShare(id, hash string) *models.Share {
	return &models.Share{ID: id, PasswordHash: &hash}
}

// withCookies replays the Set-Cookie headers of a response onto a new request, the way a
// browser would on the next load.
func withCookies(rec *httptest.ResponseRecorder) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/api/public/forms/tok/manifest.webmanifest", nil)
	for _, c := range rec.Result().Cookies() {
		r.AddCookie(c)
	}
	return r
}

func TestSharePWCookieRoundTrip(t *testing.T) {
	s := testServer()
	sh := pwShare("11111111-1111-1111-1111-111111111111", "$2a$10$hash-a")

	rec := httptest.NewRecorder()
	s.setSharePWCookie(rec, sh)
	if !s.sharePWCookieOK(withCookies(rec), sh) {
		t.Fatal("a cookie just issued for this share should be accepted")
	}
}

func TestSharePWCookieRejects(t *testing.T) {
	s := testServer()
	sh := pwShare("11111111-1111-1111-1111-111111111111", "$2a$10$hash-a")

	rec := httptest.NewRecorder()
	s.setSharePWCookie(rec, sh)

	t.Run("no cookie at all", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/api/public/forms/tok/manifest.webmanifest", nil)
		if s.sharePWCookieOK(r, sh) {
			t.Fatal("a request with no cookie must not unlock the share")
		}
	})

	t.Run("another share", func(t *testing.T) {
		other := pwShare("22222222-2222-2222-2222-222222222222", "$2a$10$hash-a")
		if s.sharePWCookieOK(withCookies(rec), other) {
			t.Fatal("a cookie for one share must not unlock another")
		}
	})

	t.Run("password changed", func(t *testing.T) {
		rotated := pwShare(sh.ID, "$2a$10$hash-b")
		if s.sharePWCookieOK(withCookies(rec), rotated) {
			t.Fatal("changing the share password must invalidate cookies already issued")
		}
	})

	t.Run("another server secret", func(t *testing.T) {
		other := &Server{cfg: &config.Config{JWTSecret: []byte("a-different-secret")}}
		if other.sharePWCookieOK(withCookies(rec), sh) {
			t.Fatal("a cookie must not verify under a different JWT secret")
		}
	})
}
