package httpapi

import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"strconv"

	"github.com/bpskaltim/eform-backend/internal/models"
)

// offlineSettingsOf membaca settings.offline.enabled dari schema instrumen (JSON bebas bentuk).
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

// resolvePWAForm memvalidasi share + form dan memastikan mode offline benar-benar aktif untuk
// kombinasi ini (form published, share multi-respons, dan toggle offline diaktifkan di builder).
// Endpoint manifest/ikon divalidasi di sini juga (bukan hanya di sisi client) sebagai defense-in-depth.
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
// hanya tersedia bila mode offline diaktifkan di instrumen dan share-nya multi-respons.
func (s *Server) publicManifest(w http.ResponseWriter, r *http.Request) {
	f, ok := s.resolvePWAForm(w, r)
	if !ok {
		return
	}
	token := r.PathValue("token")
	scope := "/f/" + token
	name := f.Title
	if name == "" {
		name = "Kuesioner"
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

// GET /api/public/forms/{token}/icon.png?size=192|512 -- ikon PWA sederhana, digambar di server
// (persegi warna aksen dengan kotak lebih terang di tengah) tanpa perlu aset gambar tambahan.
func (s *Server) publicIcon(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.resolvePWAForm(w, r); !ok {
		return
	}
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if size != 512 {
		size = 192
	}
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	bg := color.NRGBA{R: 0x0e, G: 0x74, B: 0x90, A: 0xff} // --accent
	fg := color.NRGBA{R: 0xd6, G: 0xed, B: 0xf1, A: 0xff} // --accent-soft
	margin := size / 5
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if x >= margin && x < size-margin && y >= margin && y < size-margin {
				img.Set(x, y, fg)
			} else {
				img.Set(x, y, bg)
			}
		}
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400, immutable")
	_ = png.Encode(w, img)
}
