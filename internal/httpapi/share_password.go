package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"github.com/nahrx/eform/internal/models"
)

/* Unlocking the manifest and the icons of a password-protected share.

   The share password travels in the X-Share-Password header, which only the page's own
   fetch() calls can set. The manifest and the icons are not fetched by the page: the
   browser requests them itself, from <link rel="manifest"> and <link rel="icon">, and a
   request a browser makes on its own carries no custom headers. On a protected share
   both therefore came back 401 — and a page whose manifest does not load is not
   installable, so Chrome never fired beforeinstallprompt and the "Install App" button
   never appeared. The same 401 left the home-screen icon blank.

   A cookie is the one credential the browser does attach to those requests. It is issued
   only after the password has been checked properly, and it unlocks nothing beyond the
   manifest and the icon: every other endpoint still demands the header, so opening the
   form still asks for the password. */

const (
	sharePWCookiePrefix = "eform_pw_"
	sharePWCookieTTL    = 30 * 24 * time.Hour
)

func sharePWCookieName(shareID string) string { return sharePWCookiePrefix + shareID }

// sharePWToken is derived from the JWT secret (the same trick uploads_sign.go uses, so
// operators do not have to configure another one) and bound to both the share and its
// current password hash — changing a share's password invalidates every cookie already
// handed out for it.
func (s *Server) sharePWToken(sh *models.Share) string {
	key := sha256.Sum256(append([]byte("eform-share-pw|"), s.cfg.JWTSecret...))
	mac := hmac.New(sha256.New, key[:])
	mac.Write([]byte(sh.ID))
	mac.Write([]byte("|"))
	if sh.PasswordHash != nil {
		mac.Write([]byte(*sh.PasswordHash))
	}
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// setSharePWCookie is called right after a correct password, on whichever request carried it.
func (s *Server) setSharePWCookie(w http.ResponseWriter, sh *models.Share) {
	http.SetCookie(w, &http.Cookie{
		Name:     sharePWCookieName(sh.ID),
		Value:    s.sharePWToken(sh),
		Path:     "/",
		MaxAge:   int(sharePWCookieTTL / time.Second),
		HttpOnly: true,
		// Plain http is only ever a development setup; a Secure cookie would simply not
		// be stored there, and the manifest would stay locked for no reason.
		Secure:   strings.HasPrefix(s.cfg.PublicBaseURL, "https://"),
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) sharePWCookieOK(r *http.Request, sh *models.Share) bool {
	c, err := r.Cookie(sharePWCookieName(sh.ID))
	if err != nil || c.Value == "" {
		return false
	}
	// hmac.Equal rather than ==, so comparison timing cannot leak the token.
	return hmac.Equal([]byte(c.Value), []byte(s.sharePWToken(sh)))
}
