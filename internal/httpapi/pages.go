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

/* Serving HTML pages with the organisation identity injected.

   The HTML files under web/ and public/ use the {{ORG_NAME}} and {{ORG_NICK}}
   placeholders; the values are substituted here, as the file is served. This is
   deliberately not baked into the files: changing the organisation name only
   takes an environment change and a restart, without editing any HTML.

   Only .html files go through this. Other assets (CSS/JS) still use
   http.ServeFile as before — there is nothing to substitute in them. */

const (
	phOrgName = "{{ORG_NAME}}"
	phOrgNick = "{{ORG_NICK}}"
)

type renderedPage struct {
	body    []byte
	modTime time.Time
}

// pageCache holds the substituted result so files are not re-read on every
// request. It is keyed by path; an entry is dropped as soon as the file's modTime
// changes, so editing HTML during development still shows up immediately.
// The cache lives on the Server (not in a package variable) because the
// substituted values come from config — two Servers with different configs must
// not share it.
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

// serveHTML serves a single HTML file once its placeholders are substituted.
// It uses ServeContent (rather than a plain Write) so Last-Modified and
// conditional requests keep working exactly as they did with ServeFile.
func (s *Server) serveHTML(w http.ResponseWriter, r *http.Request, path string) {
	body, mod, err := s.renderPage(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeContent(w, r, filepath.Base(path), mod, bytes.NewReader(body))
}
