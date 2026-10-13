package storage

import (
	"io"
	"net/http"
	"path"
	"strings"
)

func (s *Service) hls(w http.ResponseWriter, r *http.Request, p Project, file string) error {
	res, e := s.Backend.Download(r.Context(), p.Key, file, "")
	if e != nil {
		return e
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fail("BACKEND_UNAVAILABLE", "playlist unavailable", 503)
	}
	data, e := io.ReadAll(io.LimitReader(res.Body, (4<<20)+1))
	if e != nil || len(data) > 4<<20 {
		return fail("INVALID_PLAYLIST", "playlist too large", 503)
	}
	lines := strings.Split(string(data), "\n")
	base := "/storage/v1/projects/" + p.ID + "/derivatives/"
	if principal, ok := r.Context().Value(ctxPrincipal{}).(Principal); ok && principal.Manager {
		base = "/api/v1/storage/projects/" + p.ID + "/derivatives/"
	}
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.ContainsAny(line, "/\\:") || line == ".." {
			return fail("INVALID_PLAYLIST", "invalid segment URI", 503)
		}
		var id string
		e = s.DB.QueryRow("SELECT id FROM derivatives WHERE project=? AND path=?", p.ID, path.Join(path.Dir(file), line)).Scan(&id)
		if e != nil {
			return fail("NOT_READY", "segment not committed", 503)
		}
		lines[i] = base + id
	}
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Del("Content-Length")
	w.WriteHeader(200)
	_, e = io.WriteString(w, strings.Join(lines, "\n"))
	return e
}
