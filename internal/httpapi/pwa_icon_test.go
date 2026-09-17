package httpapi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http/httptest"
	"testing"

	"github.com/nahrx/eform/internal/models"
)

// A 40x20 red image with a blue centre square, as the data: URL the builder stores.
func testIconDataURL(t *testing.T) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 40, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 40; x++ {
			img.Set(x, y, color.NRGBA{R: 255, A: 255})
		}
	}
	for y := 5; y < 15; y++ {
		for x := 15; x < 25; x++ {
			img.Set(x, y, color.NRGBA{B: 255, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestOfflineSettingsReadsIcon(t *testing.T) {
	schema := json.RawMessage(`{"settings":{"offline":{"enabled":true,"icon":"data:image/png;base64,AAAA"}}}`)
	enabled, icon := offlineSettings(schema)
	if !enabled || icon != "data:image/png;base64,AAAA" {
		t.Fatalf("got enabled=%v icon=%q", enabled, icon)
	}
	if e, i := offlineSettings(json.RawMessage(`{"settings":{"offline":{"enabled":true}}}`)); !e || i != "" {
		t.Fatalf("no icon: got enabled=%v icon=%q", e, i)
	}
}

func TestDecodeDataURLImageRejectsJunk(t *testing.T) {
	for _, bad := range []string{"", "http://x/y.png", "data:image/png;base64,!!!", "data:text/plain;base64,aGVsbG8="} {
		if decodeDataURLImage(bad) != nil {
			t.Errorf("%q decoded to an image", bad)
		}
	}
	if decodeDataURLImage(testIconDataURL(t)) == nil {
		t.Fatal("a valid PNG data URL did not decode")
	}
}

func TestWriteScaledIconCropsToSquare(t *testing.T) {
	src := decodeDataURLImage(testIconDataURL(t))
	rec := httptest.NewRecorder()
	writeScaledIcon(rec, src, 64)
	if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("content type %q", ct)
	}
	out, err := png.Decode(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if b := out.Bounds(); b.Dx() != 64 || b.Dy() != 64 {
		t.Fatalf("size %dx%d, want 64x64", b.Dx(), b.Dy())
	}
	// The centre crop of the 40x20 source is x 10..30: the blue square (x 15..25) sits
	// in the middle of the output and red fills the edges — nothing squashed.
	r, _, b, _ := out.At(32, 32).RGBA()
	if b < r {
		t.Fatalf("centre pixel is not blue: r=%d b=%d", r, b)
	}
	r, _, b, _ = out.At(2, 32).RGBA()
	if r < b {
		t.Fatalf("edge pixel is not red: r=%d b=%d", r, b)
	}
}

func TestIconVersionFollowsTheIcon(t *testing.T) {
	none := &models.Form{Schema: json.RawMessage(`{"settings":{"offline":{"enabled":true}}}`)}
	a := &models.Form{Schema: json.RawMessage(`{"settings":{"offline":{"enabled":true,"icon":"data:image/png;base64,AAAA"}}}`)}
	b := &models.Form{Schema: json.RawMessage(`{"settings":{"offline":{"enabled":true,"icon":"data:image/png;base64,BBBB"}}}`)}
	if v := iconVersion(none); v != "0" {
		t.Fatalf("no icon → %q, want 0", v)
	}
	if iconVersion(a) == "0" || iconVersion(a) == iconVersion(b) {
		t.Fatalf("versions a=%q b=%q should be distinct and not 0", iconVersion(a), iconVersion(b))
	}
	if iconVersion(a) != iconVersion(a) {
		t.Fatal("version is not stable")
	}
}
