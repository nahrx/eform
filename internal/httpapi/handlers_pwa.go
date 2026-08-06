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

// GET /api/public/forms/{token}/manifest.webmanifest -- Web App Manifest dinamis per share,
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
	shortName := name
	if len(shortName) > 24 {
		shortName = shortName[:24]
	}
	manifest := map[string]any{
		"id":               scope,
		"name":             name,
		"short_name":       shortName,
		"start_url":        scope,
		"scope":            scope,
		"display":          "standalone",
		"background_color": "#eef1f5",
		"theme_color":      "#0e7490",
		"icons": []map[string]any{
			{"src": "/api/public/forms/" + token + "/icon.png?size=192", "sizes": "192x192", "type": "image/png", "purpose": "any maskable"},
			{"src": "/api/public/forms/" + token + "/icon.png?size=512", "sizes": "512x512", "type": "image/png", "purpose": "any maskable"},
		},
	}
	w.Header().Set("Content-Type", "application/manifest+json; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_ = json.NewEncoder(w).Encode(manifest)
}

// GET /api/public/forms/{token}/icon.png?size=192|512 -- ikon PWA digambar di server: kotak warna
// accent colour with the form title's first letter/digit in the middle, so every installed form
// gets a distinct icon without an admin having to upload any image.
func (s *Server) publicIcon(w http.ResponseWriter, r *http.Request) {
	f, ok := s.resolvePWAForm(w, r)
	if !ok {
		return
	}
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if size != 512 {
		size = 192
	}
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	bg := color.NRGBA{R: 0x0e, G: 0x74, B: 0x90, A: 0xff} // --accent
	fg := color.NRGBA{R: 0xd6, G: 0xed, B: 0xf1, A: 0xff} // --accent-soft
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.Set(x, y, bg)
		}
	}
	drawIconInitial(img, iconInitial(f.Title), fg, size)
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400, immutable")
	_ = png.Encode(w, img)
}

// iconInitial takes the first meaningful letter/digit from the form title
// (dikapitalkan) sebagai identitas visual ikon; "F" (Form) as the fallback.
func iconInitial(title string) string {
	for _, r := range strings.TrimSpace(title) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return strings.ToUpper(string(r))
		}
	}
	return "K"
}

// drawIconInitial draws a capital letter in the centre of the square icon, with a small
// "- eForm -" label beneath it (the two are treated as one group and
// centred together vertically). It fails quietly (the icon stays a plain
// plain) if the font cannot be loaded or rendered, so the icon endpoint never
// fails merely because of a text-rendering problem.
func drawIconInitial(img *image.NRGBA, letter string, fg color.NRGBA, size int) {
	fnt, err := opentype.Parse(goregular.TTF)
	if err != nil {
		return
	}

	letterFace, err := opentype.NewFace(fnt, &opentype.FaceOptions{
		Size:    float64(size) * 0.46,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return
	}
	defer letterFace.Close()

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

	letterMetrics := letterFace.Metrics()
	captionMetrics := captionFace.Metrics()
	groupHeight := letterMetrics.Ascent + letterMetrics.Descent + gap + captionMetrics.Ascent + captionMetrics.Descent
	top := (fixed.I(size) - groupHeight) / 2

	letterDrawer := &font.Drawer{Dst: img, Src: image.NewUniform(fg), Face: letterFace}
	letterWidth := letterDrawer.MeasureString(letter)
	letterBaseline := top + letterMetrics.Ascent
	letterDrawer.Dot = fixed.Point26_6{X: (fixed.I(size) - letterWidth) / 2, Y: letterBaseline}
	letterDrawer.DrawString(letter)

	captionDrawer := &font.Drawer{Dst: img, Src: image.NewUniform(fg), Face: captionFace}
	captionWidth := captionDrawer.MeasureString(caption)
	captionBaseline := top + letterMetrics.Ascent + letterMetrics.Descent + gap + captionMetrics.Ascent
	captionDrawer.Dot = fixed.Point26_6{X: (fixed.I(size) - captionWidth) / 2, Y: captionBaseline}
	captionDrawer.DrawString(caption)
}
