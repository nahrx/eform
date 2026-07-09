package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// googleStartOAuth adalah helper bersama yang memulai alur OAuth Google.
// mode: "" untuk responden, "viewer" untuk viewer.
func (s *Server) googleStartOAuth(w http.ResponseWriter, r *http.Request, next, mode string) {
	if s.cfg.GoogleClientID == "" {
		writeErr(w, http.StatusNotImplemented, "Login Google belum dikonfigurasi (GOOGLE_CLIENT_ID kosong)")
		return
	}
	if next == "" || !strings.HasPrefix(next, "/") {
		next = "/"
	}

	b := make([]byte, 16)
	_, _ = rand.Read(b)
	nonce := base64.RawURLEncoding.EncodeToString(b)

	stateBytes, _ := json.Marshal(map[string]string{"n": nonce, "next": next, "mode": mode})
	state := base64.RawURLEncoding.EncodeToString(stateBytes)

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    nonce,
		Path:     "/",
		MaxAge:   300,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	authURL := "https://accounts.google.com/o/oauth2/v2/auth?" + url.Values{
		"client_id":     {s.cfg.GoogleClientID},
		"redirect_uri":  {s.googleRedirectURI()},
		"response_type": {"code"},
		"scope":         {"openid email profile"},
		"state":         {state},
		"access_type":   {"online"},
		"prompt":        {"select_account"},
	}.Encode()
	http.Redirect(w, r, authURL, http.StatusFound)
}

// GET /auth/google?next=...  — alur login untuk responden publik.
func (s *Server) googleLogin(w http.ResponseWriter, r *http.Request) {
	s.googleStartOAuth(w, r, r.URL.Query().Get("next"), "")
}

// GET /auth/google/viewer  — alur login Google khusus untuk viewer.
func (s *Server) googleViewerLogin(w http.ResponseWriter, r *http.Request) {
	s.googleStartOAuth(w, r, "/viewer-portal", "viewer")
}

// GET /auth/google/callback?code=...&state=...
// Google mengarahkan kembali ke sini setelah login (dipakai oleh responden maupun viewer).
func (s *Server) googleCallback(w http.ResponseWriter, r *http.Request) {
	if s.cfg.GoogleClientID == "" {
		writeErr(w, http.StatusNotImplemented, "Login Google belum dikonfigurasi")
		return
	}

	// Verifikasi state & CSRF
	stateParam := r.URL.Query().Get("state")
	stateBytes, err := base64.RawURLEncoding.DecodeString(stateParam)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "state OAuth tidak valid")
		return
	}
	var stateData struct {
		N    string `json:"n"`
		Next string `json:"next"`
		Mode string `json:"mode"` // "" = responden, "viewer" = viewer
	}
	if err := json.Unmarshal(stateBytes, &stateData); err != nil {
		writeErr(w, http.StatusBadRequest, "state OAuth tidak valid")
		return
	}
	cookie, err := r.Cookie("oauth_state")
	if err != nil || cookie.Value != stateData.N {
		writeErr(w, http.StatusBadRequest, "verifikasi CSRF gagal — coba login ulang")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "oauth_state", Value: "", Path: "/", MaxAge: -1})

	if errParam := r.URL.Query().Get("error"); errParam != "" {
		writeErr(w, http.StatusUnauthorized, "login Google dibatalkan: "+errParam)
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		writeErr(w, http.StatusBadRequest, "kode OAuth tidak ditemukan")
		return
	}

	// Tukar code dengan access token
	tokenResp, err := exchangeGoogleCode(r.Context(), s.cfg.GoogleClientID, s.cfg.GoogleClientSecret, s.googleRedirectURI(), code)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal menukar kode Google: "+err.Error())
		return
	}

	// Ambil info profil dari Google
	gUser, err := getGoogleUserInfo(r.Context(), tokenResp.AccessToken)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengambil profil Google")
		return
	}

	next := stateData.Next
	if next == "" || !strings.HasPrefix(next, "/") {
		next = "/"
	}

	// ── Alur Staff (viewer/editor): cari user berdasarkan email Google ──
	if stateData.Mode == "viewer" {
		user, err := s.st.GetUserByEmail(r.Context(), gUser.Email)
		if err != nil || (user.Role != "viewer" && user.Role != "editor") || !user.IsActive {
			// Email tidak terdaftar sebagai viewer/editor aktif
			http.Redirect(w, r, "/viewer-portal?error=not_authorized&email="+url.QueryEscape(gUser.Email), http.StatusFound)
			return
		}
		jwtToken, err := s.auth.Generate(user.ID, user.Username, user.Role)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "gagal menerbitkan token")
			return
		}
		nextPage := "/viewer-portal"
		typeParam := "viewer"
		if user.Role == "editor" {
			nextPage = "/admin"
			typeParam = "editor"
		}
		// Simpan ke localStorage via done page
		doneURL := "/auth/google/done?" + url.Values{
			"token": {jwtToken},
			"next":  {nextPage},
			"type":  {typeParam},
		}.Encode()
		http.Redirect(w, r, doneURL, http.StatusFound)
		return
	}

	// ── Alur Responden (default) ──
	respondent, err := s.st.UpsertRespondent(r.Context(), gUser.ID, gUser.Email, gUser.Name, gUser.Picture)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal menyimpan data responden")
		return
	}
	jwtToken, err := s.auth.GenerateRespondent(respondent.ID, respondent.Email, respondent.Name, respondent.Picture)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal menerbitkan token")
		return
	}
	doneURL := "/auth/google/done?" + url.Values{
		"token": {jwtToken},
		"next":  {next},
	}.Encode()
	http.Redirect(w, r, doneURL, http.StatusFound)
}

// --- Google API helpers ---

type googleTokenResp struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

func exchangeGoogleCode(ctx context.Context, clientID, clientSecret, redirectURI, code string) (*googleTokenResp, error) {
	resp, err := http.PostForm("https://oauth2.googleapis.com/token", url.Values{
		"code":          {code},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"redirect_uri":  {redirectURI},
		"grant_type":    {"authorization_code"},
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var t googleTokenResp
	if err := json.NewDecoder(resp.Body).Decode(&t); err != nil {
		return nil, err
	}
	if t.AccessToken == "" {
		return nil, fmt.Errorf("access_token kosong dari Google")
	}
	return &t, nil
}

type googleUserInfo struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

func getGoogleUserInfo(ctx context.Context, accessToken string) (*googleUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var u googleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, err
	}
	if u.ID == "" {
		return nil, fmt.Errorf("ID Google kosong dalam respons")
	}
	return &u, nil
}

func (s *Server) googleRedirectURI() string {
	if s.cfg.GoogleRedirectURL != "" {
		return s.cfg.GoogleRedirectURL
	}
	return s.cfg.PublicBaseURL + "/auth/google/callback"
}
