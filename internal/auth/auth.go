package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type Manager struct {
	secret           []byte
	respondentSecret []byte
	ttl              time.Duration
	respondentTTL    time.Duration
}

func NewManager(secret, respondentSecret []byte, ttl time.Duration) *Manager {
	return &Manager{secret: secret, respondentSecret: respondentSecret, ttl: ttl, respondentTTL: 30 * 24 * time.Hour}
}

func (m *Manager) WithRespondentTTL(d time.Duration) *Manager {
	m.respondentTTL = d
	return m
}

type Claims struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	// TokenVersion mengikat token ke users.token_version. Server menaikkan angka itu
	// saat akun dinonaktifkan atau passwordnya diganti, sehingga token lama langsung
	// tidak berlaku tanpa perlu menunggu masa berlakunya habis.
	TokenVersion int `json:"tv"`
	jwt.RegisteredClaims
}

// MinPasswordLen adalah panjang minimum password akun admin/superadmin.
const MinPasswordLen = 10

// commonPasswords adalah daftar pendek password yang paling sering dipakai/ditebak.
// Sengaja tidak panjang: tujuannya menjaring pilihan yang jelas buruk, bukan
// menggantikan daftar kebocoran yang lengkap.
var commonPasswords = map[string]bool{
	"password": true, "password1": true, "password123": true, "passw0rd": true,
	"qwerty123": true, "1234567890": true, "123456789": true, "12345678": true,
	"admin12345": true, "administrator": true, "adminadmin": true, "rahasia123": true,
	"indonesia": true, "bismillah": true, "letmein123": true, "welcome123": true,
	"iloveyou1": true, "abcd123456": true, "qwertyuiop": true, "asdfghjkl": true,
}

// ValidatePassword menerapkan kebijakan password akun admin.
//
// Mengikuti anjuran NIST: yang ditekankan panjang dan tidak mudah ditebak, bukan
// kombinasi huruf besar/angka/simbol yang justru mendorong pola seperti "Passw0rd!".
func ValidatePassword(pw, username, email string) error {
	if len([]rune(pw)) < MinPasswordLen {
		return fmt.Errorf("password minimal %d karakter", MinPasswordLen)
	}
	low := strings.ToLower(pw)
	if commonPasswords[low] {
		return errors.New("password terlalu umum, pilih yang lain")
	}
	if u := strings.ToLower(strings.TrimSpace(username)); u != "" && strings.Contains(low, u) {
		return errors.New("password tidak boleh memuat username")
	}
	if e := strings.ToLower(strings.TrimSpace(email)); e != "" {
		if local, _, ok := strings.Cut(e, "@"); ok && local != "" && strings.Contains(low, local) {
			return errors.New("password tidak boleh memuat alamat email")
		}
	}
	// Satu karakter berulang ("aaaaaaaaaa") panjangnya memenuhi syarat tapi tidak aman.
	if isSingleRuneRepeat(pw) {
		return errors.New("password tidak boleh berupa satu karakter yang diulang")
	}
	return nil
}

func isSingleRuneRepeat(pw string) bool {
	rs := []rune(pw)
	for _, r := range rs[1:] {
		if r != rs[0] {
			return false
		}
	}
	return len(rs) > 0
}

func HashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(b), err
}

func CheckPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

func (m *Manager) Generate(userID, username, role string, tokenVersion int) (string, error) {
	now := time.Now()
	claims := Claims{
		Username:     username,
		Role:         role,
		TokenVersion: tokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

var ErrInvalidToken = errors.New("token tidak valid")

func (m *Manager) Parse(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	tok, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return m.secret, nil
	})
	if err != nil || !tok.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// --- Respondent (publik, login via Google OAuth) ---

type RespondentClaims struct {
	RespondentID string `json:"respondentId"`
	Email        string `json:"email"`
	Name         string `json:"name"`
	Picture      string `json:"picture"`
	jwt.RegisteredClaims
}

// GenerateRespondent menerbitkan JWT untuk responden publik, TTL dikonfigurasi via Manager.
func (m *Manager) GenerateRespondent(respondentID, email, name, picture string) (string, error) {
	now := time.Now()
	claims := RespondentClaims{
		RespondentID: respondentID,
		Email:        email,
		Name:         name,
		Picture:      picture,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   respondentID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.respondentTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.respondentSecret)
}

func (m *Manager) ParseRespondent(tokenStr string) (*RespondentClaims, error) {
	claims := &RespondentClaims{}
	tok, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return m.respondentSecret, nil
	})
	if err != nil || !tok.Valid || claims.RespondentID == "" {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
