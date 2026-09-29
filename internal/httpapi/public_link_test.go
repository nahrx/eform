package httpapi

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nahrx/eform/internal/models"
)

const publicLinkSchema = `{"pages":[{"kind":"page","components":[
  {"kind":"block","components":[
    {"kind":"field","name":"ktp","type":"file","publicLink":true},
    {"kind":"field","name":"rumah","type":"photo"},
    {"kind":"roster","name":"art","components":[
      {"kind":"field","name":"foto_art","type":"photo","publicLink":true},
      {"kind":"field","name":"kk_art","type":"file"}
    ]}
  ]}
]}]}`

func TestFieldWantsPublicLink(t *testing.T) {
	s := json.RawMessage(publicLinkSchema)
	cases := map[string]bool{
		"ktp":       true,
		"rumah":     false,
		"foto_art":  true, // inside a roster
		"kk_art":    false,
		"tidak_ada": false, // a name the instrument does not have
	}
	for name, want := range cases {
		if got := fieldWantsPublicLink(s, name); got != want {
			t.Errorf("%s: got %v, want %v", name, got, want)
		}
	}
	if fieldWantsPublicLink(nil, "ktp") || fieldWantsPublicLink(json.RawMessage(`not json`), "ktp") || fieldWantsPublicLink(s, "") {
		t.Error("a missing or unreadable schema must not grant a public link")
	}
}

func TestPublicUploadPathIsNotSigned(t *testing.T) {
	s := testServer()
	pub := "/uploads/public/2026/09/29/resp/abc.png"
	if got := s.signUploadURL(pub); got != pub {
		t.Fatalf("a public attachment must keep its plain path, got %q", got)
	}
	priv := "/uploads/2026/09/29/resp/abc.png"
	got := s.signUploadURL(priv)
	if !strings.Contains(got, "?e=") || !strings.Contains(got, "&s=") {
		t.Fatalf("a protected attachment must be signed, got %q", got)
	}
	if !isPublicUploadPath(pub) || isPublicUploadPath(priv) || isPublicUploadPath("/other/public/x.png") {
		t.Error("isPublicUploadPath does not match only the public subtree")
	}
}

func TestExportUploadURLIsAbsoluteAndLongLived(t *testing.T) {
	s := testServer()
	s.cfg.PublicBaseURL = "https://eform.example.go.id"

	priv := s.exportUploadURL("/uploads/2026/09/29/resp/abc.png")
	if !strings.HasPrefix(priv, "https://eform.example.go.id/uploads/") {
		t.Fatalf("an exported link must carry the host, got %q", priv)
	}
	if !strings.Contains(priv, "?e=") || !strings.Contains(priv, "&s=") {
		t.Fatalf("a protected attachment must still be signed in an export, got %q", priv)
	}
	// The signature has to outlive the two-hour link a page gets.
	u, err := url.Parse(priv)
	if err != nil {
		t.Fatal(err)
	}
	exp, err := strconv.ParseInt(u.Query().Get("e"), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	if time.Until(time.Unix(exp, 0)) < 24*time.Hour {
		t.Fatalf("an exported link expires too soon: %s", time.Until(time.Unix(exp, 0)))
	}
	if !s.verifyUploadURL(u.Path, u.Query()) {
		t.Fatal("an exported link must verify")
	}

	pub := s.exportUploadURL("/uploads/public/2026/09/29/resp/abc.png")
	if pub != "https://eform.example.go.id/uploads/public/2026/09/29/resp/abc.png" {
		t.Fatalf("a public attachment needs the host and nothing else, got %q", pub)
	}
	if got := s.exportUploadURL("hello"); got != "hello" {
		t.Fatalf("a value that is not an attachment must be left alone, got %q", got)
	}
}

func TestExportAnswersRewritesNestedAttachments(t *testing.T) {
	s := testServer()
	s.cfg.PublicBaseURL = "https://eform.example.go.id"
	rr := models.Response{Answers: json.RawMessage(`{"foto":["/uploads/a.png","/uploads/public/b.png"],"nama":"Budi"}`)}
	out := s.exportAnswers(rr)
	var got map[string]any
	if err := json.Unmarshal(out.Answers, &got); err != nil {
		t.Fatal(err)
	}
	list := got["foto"].([]any)
	for _, v := range list {
		if !strings.HasPrefix(v.(string), "https://eform.example.go.id/uploads/") {
			t.Fatalf("every attachment in a list must be rewritten, got %v", v)
		}
	}
	if !strings.Contains(list[0].(string), "&s=") || strings.Contains(list[1].(string), "&s=") {
		t.Fatalf("the protected one is signed, the public one is not: %v", list)
	}
	if got["nama"] != "Budi" {
		t.Fatalf("other answers must be untouched, got %v", got["nama"])
	}
}
