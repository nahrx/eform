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
	// TokenVersion ties the token to users.token_version. The server bumps that number
	// when an account is deactivated or its password changes, so older tokens stop
	// working immediately instead of lingering until they expire.
	TokenVersion int `json:"tv"`
	jwt.RegisteredClaims
}

// MinPasswordLen is the minimum password length for admin/superadmin accounts.
const MinPasswordLen = 10

// commonPasswords is a short list of the most frequently used and guessed passwords.
// Deliberately short: it exists to catch obviously bad choices, not to replace a
// full breach corpus.
var commonPasswords = map[string]bool{
	"password": true, "password1": true, "password123": true, "passw0rd": true,
	"qwerty123": true, "1234567890": true, "123456789": true, "12345678": true,
	"admin12345": true, "administrator": true, "adminadmin": true, "rahasia123": true,
	"indonesia": true, "bismillah": true, "letmein123": true, "welcome123": true,
	"iloveyou1": true, "abcd123456": true, "qwertyuiop": true, "asdfghjkl": true,
}

// ValidatePassword enforces the password policy for admin accounts.
//
// Follows NIST guidance: length and unpredictability matter, not a mix of upper case,
// digits, and symbols, which mostly encourages patterns like "Passw0rd!".
func ValidatePassword(pw, username, email string) error {
	if len([]rune(pw)) < MinPasswordLen {
		return fmt.Errorf("password must be at least %d characters", MinPasswordLen)
	}
	low := strings.ToLower(pw)
	if commonPasswords[low] {
		return errors.New("password is too common, please choose another")
	}
	if u := strings.ToLower(strings.TrimSpace(username)); u != "" && strings.Contains(low, u) {
		return errors.New("password must not contain the username")
	}
	if e := strings.ToLower(strings.TrimSpace(email)); e != "" {
		if local, _, ok := strings.Cut(e, "@"); ok && local != "" && strings.Contains(low, local) {
			return errors.New("password must not contain the email address")
		}
	}
	// A single repeated character ("aaaaaaaaaa") satisfies the length rule but is not safe.
	if isSingleRuneRepeat(pw) {
		return errors.New("password must not be a single repeated character")
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

var ErrInvalidToken = errors.New("invalid token")

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

// --- Respondent (public, signs in via Google OAuth) ---

type RespondentClaims struct {
	RespondentID string `json:"respondentId"`
	Email        string `json:"email"`
	Name         string `json:"name"`
	Picture      string `json:"picture"`
	jwt.RegisteredClaims
}

// GenerateRespondent issues a JWT for a public respondent; the TTL is configured on the Manager.
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
