package httpapi

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bpskaltim/eform-backend/internal/config"
)

func testServer() *Server {
	return &Server{cfg: &config.Config{JWTSecret: []byte("rahasia-uji-tanda-tangan")}}
}

func TestSignDanVerifyURLLampiran(t *testing.T) {
	s := testServer()
	p := "/uploads/2026/06/25/abc/foto.jpg"

	signed := s.signUploadURL(p)
	if !strings.HasPrefix(signed, p+"?") {
		t.Fatalf("URL bertanda tangan harus tetap menunjuk path aslinya, dapat %q", signed)
	}
	u, err := url.Parse(signed)
	if err != nil {
		t.Fatal(err)
	}
	if !s.verifyUploadURL(p, u.Query()) {
		t.Fatal("URL yang baru ditandatangani seharusnya lolos verifikasi")
	}
}

func TestVerifyMenolakTandaTanganPalsu(t *testing.T) {
	s := testServer()
	p := "/uploads/2026/06/25/abc/foto.jpg"
	exp := strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10)

	cases := []struct {
		nama string
		q    url.Values
	}{
		{"tanpa parameter", url.Values{}},
		{"tanda tangan asal", url.Values{"e": {exp}, "s": {"ngawur"}}},
		{"tanpa kedaluwarsa", url.Values{"s": {s.uploadSig(p, 1)}}},
		{"sudah kedaluwarsa", url.Values{
			"e": {strconv.FormatInt(time.Now().Add(-time.Minute).Unix(), 10)},
			"s": {s.uploadSig(p, time.Now().Add(-time.Minute).Unix())},
		}},
	}
	for _, c := range cases {
		t.Run(c.nama, func(t *testing.T) {
			if s.verifyUploadURL(p, c.q) {
				t.Fatal("seharusnya ditolak")
			}
		})
	}
}

func TestTandaTanganTerikatPathTertentu(t *testing.T) {
	// Tanda tangan untuk satu berkas tidak boleh bisa dipakai membuka berkas lain.
	s := testServer()
	a := "/uploads/2026/06/25/abc/rahasia-a.jpg"
	b := "/uploads/2026/06/25/abc/rahasia-b.jpg"

	u, _ := url.Parse(s.signUploadURL(a))
	if s.verifyUploadURL(b, u.Query()) {
		t.Fatal("tanda tangan berkas A tidak boleh berlaku untuk berkas B")
	}
}

func TestTandaTanganTerikatSecretServer(t *testing.T) {
	a := testServer()
	b := &Server{cfg: &config.Config{JWTSecret: []byte("secret-lain")}}
	p := "/uploads/x/y.jpg"

	u, _ := url.Parse(a.signUploadURL(p))
	if b.verifyUploadURL(p, u.Query()) {
		t.Fatal("tanda tangan dari secret berbeda seharusnya ditolak")
	}
}

func TestSignAnswerUploadsMenelusuriStrukturBersarang(t *testing.T) {
	s := testServer()
	raw := json.RawMessage(`{
		"foto":"/uploads/a/b.jpg",
		"nama":"Budi",
		"berkas":["/uploads/a/c.pdf","bukan-path"],
		"nested":{"ktp":"/uploads/a/d.png"},
		"link":"https://contoh.id/gambar.jpg"
	}`)
	out := s.signAnswerUploads(raw)

	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m["foto"].(string), "?e=") {
		t.Error("path lampiran di tingkat atas harus ditandatangani")
	}
	if arr := m["berkas"].([]any); !strings.Contains(arr[0].(string), "?e=") {
		t.Error("path lampiran di dalam array harus ditandatangani")
	} else if arr[1].(string) != "bukan-path" {
		t.Error("nilai non-lampiran tidak boleh diubah")
	}
	if n := m["nested"].(map[string]any); !strings.Contains(n["ktp"].(string), "?e=") {
		t.Error("path lampiran di dalam objek bersarang harus ditandatangani")
	}
	if m["nama"] != "Budi" {
		t.Error("jawaban biasa tidak boleh berubah")
	}
	if m["link"] != "https://contoh.id/gambar.jpg" {
		t.Error("URL eksternal tidak boleh ikut ditandatangani")
	}
}

func TestSignAnswerUploadsTidakMengubahJawabanTanpaLampiran(t *testing.T) {
	s := testServer()
	raw := json.RawMessage(`{"a":"1","b":"2"}`)
	if got := string(s.signAnswerUploads(raw)); got != string(raw) {
		t.Fatalf("jawaban tanpa lampiran harus dikembalikan apa adanya, dapat %s", got)
	}
}
