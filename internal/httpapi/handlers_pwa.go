package httpapi

import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"github.com/nahrx/eform/internal/models"
)

// offlineSettingsOf reads settings.offline.enabled from the instrument schema (free-form JSON).
func offlineSettingsOf(schema json.RawMessage) bool {
	if len(schema) == 0 {
		return false
	}
	var parsed struct {
		Settings struct {
			Offline struct {
				Enabled bool `json:"enabled"`
			} `json:"offline"`
		} `json:"settings"`
	}
	if err := json.Unmarshal(schema, &parsed); err != nil {
		return false
	}
	return parsed.Settings.Offline.Enabled
}

// resolvePWAForm validates the share + form and confirms that offline mode is genuinely enabled for
// this combination (form published, share set to multi-response, and the offline toggle on in the builder).
// The manifest/icon endpoints validate this server-side too (not only in the client) as defence in depth.
func (s *Server) resolvePWAForm(w http.ResponseWriter, r *http.Request) (*models.Form, bool) {
	sh, ok := s.resolveShare(w, r)
	if !ok {
		return nil, false
	}
	f, err := s.st.GetForm(r.Context(), sh.FormID)
	if err != nil {
		http.NotFound(w, r)
		return nil, false
	}
	if f.Status != "published" || !sh.MultiResponse || !offlineSettingsOf(f.Schema) {
		http.NotFound(w, r)
		return nil, false
	}
	return f, true
}

// shortIconName trims the title for the launcher label, counting runes rather than
// bytes. Slicing a Go string by bytes cuts a multi-byte character in half, and the
// broken byte becomes U+FFFD in the JSON — a "�" on the home screen under the icon.
func shortIconName(name string) string {
	const max = 24
	rs := []rune(name)
	if len(rs) <= max {
		return name
	}
	return strings.TrimSpace(string(rs[:max]))
}

// GET /api/public/forms/{token}/manifest.webmanifest — a per-share Web App Manifest,
// only available when offline mode is enabled on the instrument and the share is multi-response.
func (s *Server) publicManifest(w http.ResponseWriter, r *http.Request) {
	f, ok := s.resolvePWAForm(w, r)
	if !ok {
		return
	}
	token := r.PathValue("token")
	scope := "/f/" + token
	name := f.Title
	if name == "" {
		name = "Form"
	}
	icon := func(size int) string {
		return "/api/public/forms/" + token + "/icon.png?size=" + strconv.Itoa(size)
	}
	manifest := map[string]any{
		"id":               scope,
		"name":             name,
		"short_name":       shortIconName(name),
		"start_url":        scope,
		"scope":            scope,
		"display":          "standalone",
		"background_color": "#eef1f5",
		"theme_color":      "#0e7490",
		// "any" and "maskable" are listed as separate entries rather than one combined
		// "any maskable". Launchers that only understand one purpose silently skip an
		// entry declaring both, and skipping every entry means no icon at all.
		"icons": []map[string]any{
			{"src": icon(192), "sizes": "192x192", "type": "image/png", "purpose": "any"},
			{"src": icon(512), "sizes": "512x512", "type": "image/png", "purpose": "any"},
			{"src": icon(192), "sizes": "192x192", "type": "image/png", "purpose": "maskable"},
			{"src": icon(512), "sizes": "512x512", "type": "image/png", "purpose": "maskable"},
		},
	}
	w.Header().Set("Content-Type", "application/manifest+json; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_ = json.NewEncoder(w).Encode(manifest)
}

// GET /api/public/forms/{token}/icon.png?size=... — the icon is drawn on the server: a
// square in the accent colour with the form title's first letter/digit in the middle, so
// every form gets a distinct icon without an admin having to upload any image.
//
// Unlike the manifest, this is NOT restricted to offline-enabled shares. It is also the
// page's favicon and apple-touch-icon, and gating it behind offline mode was why a phone
// had no icon to fall back on: "Add to home screen" on a page Chrome has not judged
// installable creates a plain bookmark, and a plain bookmark uses the favicon.
func (s *Server) publicIcon(w http.ResponseWriter, r *http.Request) {
	sh, ok := s.resolveShare(w, r)
	if !ok {
		return
	}
	f, err := s.st.GetForm(r.Context(), sh.FormID)
	if err != nil || f.Status != "published" {
		http.NotFound(w, r)
		return
	}
	// A small allowlist rather than a free size: the endpoint is public and unauthenticated,
	// so an arbitrary ?size= would let anyone ask the server to render a huge PNG.
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	switch size {
	case 32, 180, 192, 512: // favicon, iOS home screen, Android, Play/splash
	default:
		size = 192
	}
	writeIconPNG(w, iconInitial(f.Title), size)
}

/* GET /favicon.ico — a generic "e" icon for every page in the app.

   Browsers ask for /favicon.ico before any script has run, so a <link rel="icon">
   added from JavaScript arrives too late to stop the request. That is why this used
   to 404 on every single page load, and why a phone bookmarking the form had nothing
   to draw. The per-form icon is still the better one when it is available; this is
   the floor. */
func (s *Server) faviconICO(w http.ResponseWriter, r *http.Request) {
	// PNG bytes under a .ico URL: every browser in use sniffs the content type, and a
	// real ICO container would buy nothing here.
	writeIconPNG(w, "e", 32)
}

// writeIconPNG renders the square icon and writes it as a PNG response.
func writeIconPNG(w http.ResponseWriter, letter string, size int) {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	bg := color.NRGBA{R: 0x0e, G: 0x74, B: 0x90, A: 0xff} // --accent
	fg := color.NRGBA{R: 0xd6, G: 0xed, B: 0xf1, A: 0xff} // --accent-soft
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.Set(x, y, bg)
		}
	}
	drawIconInitial(img, letter, fg, size)
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_ = png.Encode(w, img)
}

// iconInitial takes the first meaningful letter/digit from the form title, capitalised,
// as the icon's visual identity; "K" as the fallback.
func iconInitial(title string) string {
	for _, r := range strings.TrimSpace(title) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return strings.ToUpper(string(r))
		}
	}
	return "K"
}

// drawIconInitial draws a capital letter in the centre of the square icon, with a small
// "- eForm -" label beneath it (the two are treated as one group and centred together
// vertically). It fails quietly (the icon stays a plain square) if the font cannot be
// loaded or rendered, so the icon endpoint never fails merely because of a
// text-rendering problem.
func drawIconInitial(img *image.NRGBA, letter string, fg color.NRGBA, size int) {
	fnt, err := opentype.Parse(goregular.TTF)
	if err != nil {
		return
	}

	// At favicon sizes the caption would render around three pixels tall — unreadable
	// smudge that only muddies the letter. Below this the letter goes it alone, and
	// grows to fill the space the caption would have taken.
	withCaption := size >= 96
	letterScale := 0.46
	if !withCaption {
		letterScale = 0.62
	}

	letterFace, err := opentype.NewFace(fnt, &opentype.FaceOptions{
		Size:    float64(size) * letterScale,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return
	}
	defer letterFace.Close()

	letterMetrics := letterFace.Metrics()
	letterDrawer := &font.Drawer{Dst: img, Src: image.NewUniform(fg), Face: letterFace}
	letterWidth := letterDrawer.MeasureString(letter)

	if !withCaption {
		top := (fixed.I(size) - (letterMetrics.Ascent + letterMetrics.Descent)) / 2
		letterDrawer.Dot = fixed.Point26_6{X: (fixed.I(size) - letterWidth) / 2, Y: top + letterMetrics.Ascent}
		letterDrawer.DrawString(letter)
		return
	}

	captionFace, err := opentype.NewFace(fnt, &opentype.FaceOptions{
		Size:    float64(size) * 0.1,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return
	}
	defer captionFace.Close()

	const caption = "- eForm -"
	gap := fixed.I(size) / 24

	captionMetrics := captionFace.Metrics()
	groupHeight := letterMetrics.Ascent + letterMetrics.Descent + gap + captionMetrics.Ascent + captionMetrics.Descent
	top := (fixed.I(size) - groupHeight) / 2

	letterBaseline := top + letterMetrics.Ascent
	letterDrawer.Dot = fixed.Point26_6{X: (fixed.I(size) - letterWidth) / 2, Y: letterBaseline}
	letterDrawer.DrawString(letter)

	captionDrawer := &font.Drawer{Dst: img, Src: image.NewUniform(fg), Face: captionFace}
	captionWidth := captionDrawer.MeasureString(caption)
	captionBaseline := top + letterMetrics.Ascent + letterMetrics.Descent + gap + captionMetrics.Ascent
	captionDrawer.Dot = fixed.Point26_6{X: (fixed.I(size) - captionWidth) / 2, Y: captionBaseline}
	captionDrawer.DrawString(caption)
}
