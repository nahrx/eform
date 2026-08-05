package auth

import (
	"strings"
	"testing"
	"time"
)

func TestValidatePassword(t *testing.T) {
	cases := []struct {
		nama     string
		pw       string
		username string
		email    string
		mauLolos bool
	}{
		{"panjang & tidak tertebak", "kopi-tubruk-pagi", "admin64", "admin@bps.go.id", true},
		{"tepat batas minimum", strings.Repeat("x", MinPasswordLen) + "y", "admin64", "", true},
		{"terlalu pendek", "rahasia9", "admin64", "", false},
		{"password umum", "password123", "admin64", "", false},
		{"memuat username", "admin64hebat", "admin64", "", false},
		{"memuat bagian email", "budi-sekali-kuat", "u1", "budi@bps.go.id", false},
		{"satu karakter diulang", strings.Repeat("a", 12), "u1", "", false},
		{"kosong", "", "u1", "", false},
	}
	for _, c := range cases {
		t.Run(c.nama, func(t *testing.T) {
			err := ValidatePassword(c.pw, c.username, c.email)
			if c.mauLolos && err != nil {
				t.Fatalf("harusnya diterima, ditolak: %v", err)
			}
			if !c.mauLolos && err == nil {
				t.Fatal("harusnya ditolak, tapi diterima")
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
		t.Fatal("password tersimpan dalam bentuk asli")
	}
	if !CheckPassword(h, "kopi-tubruk-pagi") {
		t.Error("password benar seharusnya cocok")
	}
	if CheckPassword(h, "kopi-tubruk-sore") {
		t.Error("password salah seharusnya ditolak")
	}
}

func TestTokenVersionIkutDiTokenDanDiperiksa(t *testing.T) {
	m := NewManager([]byte("rahasia-uji"), []byte("rahasia-responden"), time.Hour)

	tok, err := m.Generate("u1", "admin64", "superadmin", 7)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := m.Parse(tok)
	if err != nil {
		t.Fatal(err)
	}
	if claims.TokenVersion != 7 {
		t.Fatalf("token_version harus terbawa, dapat %d", claims.TokenVersion)
	}
	if claims.Subject != "u1" || claims.Role != "superadmin" {
		t.Fatalf("klaim tidak sesuai: %+v", claims)
	}
}

func TestTokenDariSecretLainDitolak(t *testing.T) {
	a := NewManager([]byte("secret-a"), []byte("resp-a"), time.Hour)
	b := NewManager([]byte("secret-b"), []byte("resp-b"), time.Hour)

	tok, err := a.Generate("u1", "admin", "admin", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Parse(tok); err == nil {
		t.Fatal("token yang ditandatangani secret lain seharusnya ditolak")
	}
}

func TestTokenKedaluwarsaDitolak(t *testing.T) {
	m := NewManager([]byte("rahasia-uji"), []byte("resp"), -time.Minute) // sudah lewat
	tok, err := m.Generate("u1", "admin", "admin", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Parse(tok); err == nil {
		t.Fatal("token kedaluwarsa seharusnya ditolak")
	}
}

// Token respondent memakai secret terpisah; keduanya tidak boleh saling menerima.
func TestTokenRespondentTerpisahDariTokenAdmin(t *testing.T) {
	m := NewManager([]byte("secret-admin"), []byte("secret-resp"), time.Hour)

	adminTok, _ := m.Generate("u1", "admin", "admin", 0)
	if _, err := m.ParseRespondent(adminTok); err == nil {
		t.Error("token admin tidak boleh lolos sebagai token responden")
	}

	respTok, _ := m.GenerateRespondent("r1", "a@b.c", "A", "")
	if _, err := m.Parse(respTok); err == nil {
		t.Error("token responden tidak boleh lolos sebagai token admin")
	}
}
