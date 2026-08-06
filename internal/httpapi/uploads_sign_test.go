package httpapi

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nahrx/eform/internal/config"
)

func testServer() *Server {
	return &Server{cfg: &config.Config{JWTSecret: []byte("signature-test-secret")}}
}

func TestSignAndVerifyAttachmentURL(t *testing.T) {
	s := testServer()
	p := "/uploads/2026/06/25/abc/foto.jpg"

	signed := s.signUploadURL(p)
	if !strings.HasPrefix(signed, p+"?") {
		t.Fatalf("the signed URL must still point at the original path, got %q", signed)
	}
	u, err := url.Parse(signed)
	if err != nil {
		t.Fatal(err)
	}
	if !s.verifyUploadURL(p, u.Query()) {
		t.Fatal("a freshly signed URL should pass verification")
	}
}

func TestVerifyRejectsForgedSignatures(t *testing.T) {
	s := testServer()
	p := "/uploads/2026/06/25/abc/foto.jpg"
	exp := strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10)

	cases := []struct {
		name string
		q    url.Values
	}{
		{"no parameters", url.Values{}},
		{"tanda tangan asal", url.Values{"e": {exp}, "s": {"ngawur"}}},
		{"no expiry", url.Values{"s": {s.uploadSig(p, 1)}}},
		{"already expired", url.Values{
			"e": {strconv.FormatInt(time.Now().Add(-time.Minute).Unix(), 10)},
			"s": {s.uploadSig(p, time.Now().Add(-time.Minute).Unix())},
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if s.verifyUploadURL(p, c.q) {
				t.Fatal("should be rejected")
			}
		})
	}
}

func TestSignatureIsBoundToOnePath(t *testing.T) {
	// A signature for one file must not open a different file.
	s := testServer()
	a := "/uploads/2026/06/25/abc/rahasia-a.jpg"
	b := "/uploads/2026/06/25/abc/rahasia-b.jpg"

	u, _ := url.Parse(s.signUploadURL(a))
	if s.verifyUploadURL(b, u.Query()) {
		t.Fatal("file A's signature must not be valid for file B")
	}
}

func TestSignatureIsBoundToServerSecret(t *testing.T) {
	a := testServer()
	b := &Server{cfg: &config.Config{JWTSecret: []byte("secret-lain")}}
	p := "/uploads/x/y.jpg"

	u, _ := url.Parse(a.signUploadURL(p))
	if b.verifyUploadURL(p, u.Query()) {
		t.Fatal("a signature from a different secret should be rejected")
	}
}

func TestSignAnswerUploadsWalksNestedStructures(t *testing.T) {
	s := testServer()
	raw := json.RawMessage(`{
		"foto":"/uploads/a/b.jpg",
		"name":"Budi",
		"files":["/uploads/a/c.pdf","not-a-path"],
		"nested":{"ktp":"/uploads/a/d.png"},
		"link":"https://contoh.id/gambar.jpg"
	}`)
	out := s.signAnswerUploads(raw)

	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m["foto"].(string), "?e=") {
		t.Error("a top-level attachment path must be signed")
	}
	if arr := m["files"].([]any); !strings.Contains(arr[0].(string), "?e=") {
		t.Error("an attachment path inside an array must be signed")
	} else if arr[1].(string) != "not-a-path" {
		t.Error("non-attachment values must not be changed")
	}
	if n := m["nested"].(map[string]any); !strings.Contains(n["ktp"].(string), "?e=") {
		t.Error("an attachment path inside a nested object must be signed")
	}
	if m["name"] != "Budi" {
		t.Error("ordinary answers must not change")
	}
	if m["link"] != "https://contoh.id/gambar.jpg" {
		t.Error("external URLs must not be signed")
	}
}

func TestSignAnswerUploadsLeavesAttachmentFreeAnswersAlone(t *testing.T) {
	s := testServer()
	raw := json.RawMessage(`{"a":"1","b":"2"}`)
	if got := string(s.signAnswerUploads(raw)); got != string(raw) {
		t.Fatalf("answers without attachments must be returned unchanged, got %s", got)
	}
}
