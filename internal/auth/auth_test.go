package auth

import (
	"strings"
	"testing"
	"time"
)

func TestValidatePassword(t *testing.T) {
	cases := []struct {
		name     string
		pw       string
		username string
		email    string
		mauLolos bool
	}{
		{"long & unguessable", "kopi-tubruk-pagi", "admin64", "admin@bps.go.id", true},
		{"tepat batas minimum", strings.Repeat("x", MinPasswordLen) + "y", "admin64", "", true},
		{"too short", "rahasia9", "admin64", "", false},
		{"common password", "password123", "admin64", "", false},
		{"contains the username", "admin64hebat", "admin64", "", false},
		{"contains part of the email", "budi-sekali-kuat", "u1", "budi@bps.go.id", false},
		{"one repeated character", strings.Repeat("a", 12), "u1", "", false},
		{"empty", "", "u1", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidatePassword(c.pw, c.username, c.email)
			if c.mauLolos && err != nil {
				t.Fatalf("should have been accepted, was rejected: %v", err)
			}
			if !c.mauLolos && err == nil {
				t.Fatal("should have been rejected, but was accepted")
			}
		})
	}
}

func TestPasswordHashRoundTrip(t *testing.T) {
	h, err := HashPassword("kopi-tubruk-pagi")
	if err != nil {
		t.Fatal(err)
	}
	if h == "kopi-tubruk-pagi" {
		t.Fatal("the password was stored in plain form")
	}
	if !CheckPassword(h, "kopi-tubruk-pagi") {
		t.Error("the correct password should match")
	}
	if CheckPassword(h, "kopi-tubruk-sore") {
		t.Error("a wrong password should be rejected")
	}
}

func TestTokenVersionTravelsInTokenAndIsChecked(t *testing.T) {
	m := NewManager([]byte("test-secret"), []byte("respondent-secret"), time.Hour)

	tok, err := m.Generate("u1", "admin64", "superadmin", 7)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := m.Parse(tok)
	if err != nil {
		t.Fatal(err)
	}
	if claims.TokenVersion != 7 {
		t.Fatalf("token_version should be carried through, got %d", claims.TokenVersion)
	}
	if claims.Subject != "u1" || claims.Role != "superadmin" {
		t.Fatalf("claims do not match: %+v", claims)
	}
}

func TestTokenFromAnotherSecretIsRejected(t *testing.T) {
	a := NewManager([]byte("secret-a"), []byte("resp-a"), time.Hour)
	b := NewManager([]byte("secret-b"), []byte("resp-b"), time.Hour)

	tok, err := a.Generate("u1", "admin", "admin", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Parse(tok); err == nil {
		t.Fatal("a token signed with another secret should be rejected")
	}
}

func TestExpiredTokenIsRejected(t *testing.T) {
	m := NewManager([]byte("test-secret"), []byte("resp"), -time.Minute) // already in the past
	tok, err := m.Generate("u1", "admin", "admin", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Parse(tok); err == nil {
		t.Fatal("an expired token should be rejected")
	}
}

// Respondent tokens use a separate secret; neither side may accept the other.
func TestRespondentTokenIsSeparateFromAdminToken(t *testing.T) {
	m := NewManager([]byte("secret-admin"), []byte("secret-resp"), time.Hour)

	adminTok, _ := m.Generate("u1", "admin", "admin", 0)
	if _, err := m.ParseRespondent(adminTok); err == nil {
		t.Error("an admin token must not pass as a respondent token")
	}

	respTok, _ := m.GenerateRespondent("r1", "a@b.c", "A", "")
	if _, err := m.Parse(respTok); err == nil {
		t.Error("a respondent token must not pass as an admin token")
	}
}
