package httpapi

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

/* Penyajian halaman HTML dengan identitas instansi disuntikkan.

   Berkas HTML di web/ dan public/ memakai placeholder {{ORG_NAME}} dan
   {{ORG_NICK}}; nilainya diganti di sini, saat berkas disajikan. Sengaja tidak
   di-build ke dalam berkas: mengganti nama instansi cukup mengubah env lalu
   me-restart server, tanpa menyunting satu berkas HTML pun.

   Hanya berkas .html yang diproses. Aset lain (CSS/JS) tetap lewat
   http.ServeFile seperti sebelumnya — di sana tidak ada yang perlu diganti. */

const (
	phOrgName = "{{ORG_NAME}}"
	phOrgNick = "{{ORG_NICK}}"
)

type renderedPage struct {
	body    []byte
	modTime time.Time
}

// pageCache menyimpan hasil substitusi supaya berkas tidak dibaca ulang tiap
// permintaan. Kuncinya path; entri dibuang begitu modTime berkasnya berubah,
// jadi menyunting HTML saat pengembangan tetap langsung terlihat.
// Cache menempel di Server (bukan variabel paket) karena nilai substitusinya
// berasal dari config — dua Server dengan config berbeda tidak boleh berbagi.
type pageCache struct{ m sync.Map }

func (s *Server) renderPage(path string) ([]byte, time.Time, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, time.Time{}, err
	}
	if v, ok := s.pages.m.Load(path); ok {
		if p := v.(renderedPage); p.modTime.Equal(fi.ModTime()) {
			return p.body, p.modTime, nil
		}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, time.Time{}, err
	}
	body := []byte(strings.NewReplacer(
		phOrgName, s.cfg.OrganisationName,
		phOrgNick, s.cfg.OrganisationNickname,
	).Replace(string(raw)))
	s.pages.m.Store(path, renderedPage{body: body, modTime: fi.ModTime()})
	return body, fi.ModTime(), nil
}

// serveHTML menyajikan satu berkas HTML setelah placeholder-nya diganti.
// Memakai ServeContent (bukan Write biasa) supaya Last-Modified dan permintaan
// bersyarat tetap berjalan seperti saat masih ServeFile.
func (s *Server) serveHTML(w http.ResponseWriter, r *http.Request, path string) {
	body, mod, err := s.renderPage(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeContent(w, r, filepath.Base(path), mod, bytes.NewReader(body))
}
