package httpapi

import (
	"encoding/json"
	"strings"
	"testing"
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
