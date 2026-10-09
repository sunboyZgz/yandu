package resource

import (
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"yandu/internal/pathpolicy"
	"yandu/internal/state"
)

type Guard struct{ Store *state.Store }

var media = map[string]string{".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif", ".webp": "image/webp", ".avif": "image/avif", ".ico": "image/x-icon", ".mp4": "video/mp4", ".webm": "video/webm", ".mp3": "audio/mpeg", ".wav": "audio/wav", ".ogg": "audio/ogg", ".flac": "audio/flac"}

func (g Guard) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.URL.Path != "/authorize" || r.Method != "GET" {
		http.NotFound(w, r)
		return
	}
	method := r.Header.Get("X-Forwarded-Method")
	if method != "GET" && method != "HEAD" {
		w.WriteHeader(405)
		return
	}
	raw := r.Header.Get("X-Forwarded-Uri")
	u, e := url.ParseRequestURI(raw)
	if e != nil {
		w.WriteHeader(400)
		return
	}
	s, e := g.Store.Read()
	if e != nil {
		w.WriteHeader(503)
		return
	}
	host := r.Host
	if h := r.Header.Get("X-Forwarded-Host"); h != "" {
		host = h
	}
	for _, p := range s.Effective {
		site, ok := s.Site(p.SiteID)
		if !ok || !strings.EqualFold(host, site.Domain) || p.Publication != "active" || r.Header.Get("X-Yandu-Generation") != p.Generation {
			continue
		}
		for _, pre := range p.ResourcePrefixes {
			if !strings.HasPrefix(u.Path, pre) {
				continue
			}
			if p.AccessPolicy == "basic_auth" {
				user, password, ok := r.BasicAuth()
				if !ok || user != p.Username || bcrypt.CompareHashAndPassword([]byte(p.PasswordHash), []byte(password)) != nil {
					w.Header().Set("WWW-Authenticate", `Basic realm="Yandu resources", charset="UTF-8"`)
					w.WriteHeader(401)
					return
				}
			}
			if u.Path == pre+".yandu-probe-"+p.ProbeToken && p.ProbeToken != "" {
				w.Header().Set("X-Yandu-Type", "text/plain; charset=utf-8")
				w.WriteHeader(204)
				return
			}
			rel, e := pathpolicy.Relative(raw, p.ProjectBase, pre)
			if e != nil {
				w.WriteHeader(400)
				return
			}
			file, e := pathpolicy.File(p.RootPath, rel)
			if e != nil {
				w.WriteHeader(404)
				return
			}
			kind := media[strings.ToLower(filepath.Ext(file))]
			if kind == "" {
				w.WriteHeader(403)
				return
			}
			f, e := os.Open(file)
			if e != nil {
				w.WriteHeader(403)
				return
			}
			buf := make([]byte, 512)
			n, _ := f.Read(buf)
			f.Close()
			sniff := http.DetectContentType(buf[:n])
			// A misleading media suffix must never serve an active HTML or script payload.
			if strings.Contains(sniff, "text/") || strings.Contains(sniff, "xml") {
				w.WriteHeader(403)
				return
			}
			if strings.HasPrefix(kind, "image/") && sniff != kind && kind != "image/avif" {
				w.WriteHeader(403)
				return
			}
			w.Header().Set("X-Yandu-Type", kind)
			w.WriteHeader(204)
			return
		}
	}
	w.WriteHeader(404)
}
