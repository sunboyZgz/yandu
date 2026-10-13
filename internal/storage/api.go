package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
	"yandu/internal/model"
)

type Entry struct {
	ID         string `json:"id"`
	Parent     string `json:"parentId"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Version    string `json:"versionId"`
	Deleted    int64  `json:"deletedAt"`
	Generation int64  `json:"generation"`
	Size       int64  `json:"size"`
}

func validName(n string) bool {
	return n != "" && n != "." && n != ".." && utf8.ValidString(n) && len(n) <= 255 && !strings.ContainsAny(n, "/\\\x00\r\n") && strings.TrimSpace(n) == n
}
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.serve(w, r, false) }
func (s *Service) Manager(w http.ResponseWriter, r *http.Request) {
	r = r.Clone(r.Context())
	r.URL.Path = strings.Replace(r.URL.Path, "/api/v1/storage/", "/storage/v1/", 1)
	if strings.Contains(r.URL.Path, "/tus/") || strings.Contains(r.URL.Path, "/content") {
		controller := http.NewResponseController(w)
		controller.SetReadDeadline(time.Time{})
		controller.SetWriteDeadline(time.Time{})
	}
	s.serve(managementResponse{w}, r, true)
}
func (s *Service) serve(w http.ResponseWriter, r *http.Request, manager bool) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 || parts[0] != "storage" || parts[1] != "v1" || parts[2] != "projects" {
		http.NotFound(w, r)
		return
	}
	id := parts[3]
	p := Principal{ID: "manager", Project: id, Manager: true}
	var e error
	if !manager {
		p, e = s.authenticate(r, id)
		if e != nil {
			reject(w, e)
			return
		}
	}
	if manager && len(parts) >= 5 && parts[4] == "credentials" {
		s.manageCredentials(w, r, id, parts)
		return
	}
	scope := "read"
	if len(parts) > 4 {
		switch parts[4] {
		case "uploads", "tus":
			scope = "upload"
		case "folders", "entries":
			if r.Method != "GET" {
				scope = "metadata"
			}
		case "trash":
			scope = "restore"
		case "jobs":
			if r.Method != "GET" {
				scope = "process"
			}
		}
	}
	if r.Method == "DELETE" && scope != "upload" {
		scope = "delete"
	}
	r, cancel, e := s.guardContext(r, p, scope)
	if e != nil {
		reject(w, e)
		return
	}
	defer cancel()
	// Cancelling a request context alone does not interrupt a blocked socket Read.
	// Revocation must also wake Tus when the sender stops supplying its body.
	finished := make(chan struct{})
	stopped := make(chan struct{})
	defer func() { close(finished); <-stopped }()
	go func() {
		defer close(stopped)
		select {
		case <-finished:
		case <-r.Context().Done():
			controller := http.NewResponseController(w)
			controller.SetReadDeadline(time.Now())
			controller.SetWriteDeadline(time.Now())
		}
	}()
	pr, e := s.project(id)
	if e != nil {
		reject(w, e)
		return
	}
	if _, e = s.checkVolume(pr); e != nil {
		reject(w, e)
		return
	}
	if len(parts) < 5 {
		respond(w, 200, map[string]any{"id": id, "mode": "application_storage"})
		return
	}
	switch parts[4] {
	case "capabilities":
		respond(w, 200, map[string]any{"tus": "1.0.0", "range": true, "versions": true, "trash": true, "presets": Presets, "mediaReady": len(s.MediaCommand) > 0, "outputConstraintCapabilities": s.mediaCapabilities(pr)})
		return
	case "capacity":
		var used, reserved int64
		s.DB.QueryRow("SELECT COALESCE((SELECT SUM(size) FROM versions WHERE project=?),0)+COALESCE((SELECT SUM(size) FROM derivatives WHERE project=?),0)", id, id).Scan(&used)
		s.DB.QueryRow("SELECT COALESCE(SUM(bytes),0) FROM reservations WHERE project=?", id).Scan(&reserved)
		v, _ := s.checkVolume(pr)
		respond(w, 200, map[string]any{"quotaBytes": pr.Quota, "usedBytes": used, "reservedBytes": reserved, "freeBytes": v.Free})
		return
	case "entries", "folders":
		if len(parts) == 5 && r.Method == "GET" {
			s.list(w, r, id)
			return
		}
		if parts[4] == "folders" && len(parts) == 5 && r.Method == "POST" {
			var v struct {
				Parent string `json:"parentId"`
				Name   string `json:"name"`
			}
			e = decode(r, &v)
			if e == nil {
				var result any
				result, e = s.folder(id, v.Parent, v.Name)
				if e == nil {
					respond(w, 201, result)
					return
				}
			}
		}
		if len(parts) == 6 {
			switch r.Method {
			case "GET":
				var entry Entry
				entry, e = s.entry(id, parts[5], false)
				if e == nil {
					respond(w, 200, entry)
					return
				}
			case "PATCH":
				var v struct {
					Parent     string `json:"parentId"`
					Name       string `json:"name"`
					Generation int64  `json:"expectedGeneration"`
				}
				e = decode(r, &v)
				if e == nil {
					e = s.moveEntry(id, parts[5], v.Parent, v.Name, v.Generation)
				}
			case "DELETE":
				e = s.deleteEntry(id, parts[5], r.URL.Query().Get("recursive") == "true")
			}
		}
	case "trash":
		if len(parts) == 5 && r.Method == "GET" {
			s.list(w, r, id)
			return
		}
		if len(parts) == 7 && parts[6] == "restore" && r.Method == "POST" {
			e = s.restoreEntry(id, parts[5])
		}
	case "uploads":
		if len(parts) == 5 && r.Method == "POST" {
			s.createIntent(w, r, p, pr)
			return
		}
		if len(parts) == 6 && r.Method == "GET" {
			var state, ver, obj, errText string
			e = s.DB.QueryRow("SELECT state,version,object,error FROM uploads WHERE id=? AND project=?", parts[5], id).Scan(&state, &ver, &obj, &errText)
			if e == nil {
				if !manager && errText != "" {
					errText = "upload requires local inspection"
				}
				respond(w, 200, map[string]any{"id": parts[5], "state": state, "objectId": obj, "versionId": ver, "error": errText, "jobs": s.uploadJobs(id, parts[5])})
				return
			}
		}
	case "tus":
		s.serveTus(w, r, p, pr, parts)
		return
	case "objects":
		if len(parts) == 7 && parts[6] == "derivatives" && r.Method == "GET" {
			s.listDerivatives(w, id, parts[5])
			return
		}
		if len(parts) == 7 && parts[6] == "content" && (r.Method == "GET" || r.Method == "HEAD") {
			s.download(w, r, pr, parts[5])
			return
		}
		if len(parts) == 7 && parts[6] == "versions" && r.Method == "GET" {
			s.rowsJSON(w, r, "SELECT id,size,sha256,mime,created FROM versions WHERE project=? AND object=? ORDER BY created", id, parts[5])
			return
		}
	case "jobs":
		if len(parts) == 6 && r.Method == "GET" {
			var j JobDetail
			j, e = s.jobDetail(id, parts[5])
			if e == nil {
				respond(w, 200, j)
				return
			}
		}
		if len(parts) == 7 && parts[6] == "cancel" && r.Method == "POST" {
			e = s.cancelJob(r.Context(), p, parts[5])
			break
		}
		if len(parts) == 5 && r.Method == "GET" {
			s.rowsJSON(w, r, "SELECT id,object,version,preset,state,attempts,error FROM jobs WHERE project=? ORDER BY rowid DESC LIMIT 100", id)
			return
		}
		if len(parts) == 5 && r.Method == "POST" {
			var v ProcessRequest
			e = decode(r, &v)
			if e == nil {
				var refs []JobReference
				refs, e = s.submitMediaJobs(r.Context(), p, v, r.Header.Get("Idempotency-Key"))
				if e == nil {
					respond(w, 200, map[string]any{"ok": true, "jobs": refs})
					return
				}
			}
		} else if e == nil {
			http.NotFound(w, r)
			return
		}
	case "derivatives":
		if len(parts) >= 6 && (r.Method == "GET" || r.Method == "HEAD") {
			s.downloadDerived(w, r, pr, parts[5])
			return
		}
	case "operations":
		s.rowsJSON(w, r, "SELECT id,object,kind,state,error FROM operations WHERE project=? ORDER BY rowid DESC LIMIT 100", id)
		return
	default:
		http.NotFound(w, r)
		return
	}
	if e != nil {
		if e == sql.ErrNoRows {
			e = fail("NOT_FOUND", "resource not found", 404)
		}
		reject(w, e)
	} else {
		respond(w, 200, map[string]bool{"ok": true})
	}
}
func (s *Service) manageCredentials(w http.ResponseWriter, r *http.Request, id string, parts []string) {
	if _, e := s.project(id); e != nil {
		reject(w, e)
		return
	}
	if len(parts) == 5 && r.Method == "POST" {
		var v struct {
			Scopes []string `json:"scopes"`
		}
		if e := decode(r, &v); e != nil {
			reject(w, e)
			return
		}
		out, e := s.credential(id, v.Scopes)
		if e != nil {
			reject(w, e)
		} else {
			respond(w, 201, out)
		}
		return
	}
	if len(parts) == 6 && r.Method == "DELETE" {
		res, e := s.DB.Exec("UPDATE credentials SET revoked=1 WHERE id=? AND project=?", parts[5], id)
		if e != nil {
			reject(w, e)
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			reject(w, fail("NOT_FOUND", "credential not found", 404))
			return
		}
		respond(w, 200, map[string]bool{"revoked": true})
		return
	}
	if len(parts) == 5 && r.Method == "GET" {
		s.rowsJSON(w, r, "SELECT id,scopes,revoked,created FROM credentials WHERE project=?", id)
		return
	}
	w.WriteHeader(405)
}
func (s *Service) rowsJSON(w http.ResponseWriter, r *http.Request, q string, args ...any) {
	rows, e := s.DB.Query(q, args...)
	if e != nil {
		reject(w, e)
		return
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	out := []map[string]any{}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptr := make([]any, len(cols))
		for i := range vals {
			ptr[i] = &vals[i]
		}
		if e = rows.Scan(ptr...); e != nil {
			reject(w, e)
			return
		}
		v := map[string]any{}
		for i, c := range cols {
			v[c] = vals[i]
			if principal, ok := r.Context().Value(ctxPrincipal{}).(Principal); ok && !principal.Manager && c == "error" && vals[i] != nil && vals[i] != "" {
				v[c] = "operation requires local inspection"
			}
		}
		out = append(out, v)
	}
	respond(w, 200, map[string]any{"items": out})
}
func (s *Service) entry(project, id string, deleted bool) (Entry, error) {
	var v Entry
	e := s.DB.QueryRow("SELECT e.id,parent,name,kind,current_version,deleted,generation,COALESCE(v.size,0) FROM entries e LEFT JOIN versions v ON e.current_version=v.id WHERE e.id=? AND e.project=?", id, project).Scan(&v.ID, &v.Parent, &v.Name, &v.Kind, &v.Version, &v.Deleted, &v.Generation, &v.Size)
	if e != nil || (!deleted && v.Deleted != 0) {
		return v, fail("NOT_FOUND", "entry not found", 404)
	}
	return v, nil
}
func (s *Service) parent(tx *sql.Tx, project, id string) error {
	if id == "" {
		return nil
	}
	var n int
	if e := tx.QueryRow("SELECT COUNT(*) FROM entries WHERE id=? AND project=? AND kind='folder' AND deleted=0", id, project).Scan(&n); e != nil {
		return e
	}
	if n != 1 {
		return fail("NOT_FOUND", "parent folder not found", 404)
	}
	return nil
}
func (s *Service) folder(project, parent, name string) (any, error) {
	if !validName(name) {
		return nil, fail("INVALID_NAME", "invalid name", 400)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, e := s.DB.Begin()
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	if e = s.parent(tx, project, parent); e != nil {
		return nil, e
	}
	id := model.ID()
	_, e = tx.Exec("INSERT INTO entries(id,project,parent,name,kind,created) VALUES(?,?,?,?,'folder',?)", id, project, parent, name, time.Now().Unix())
	if e != nil {
		return nil, fail("NAME_CONFLICT", "name already exists", 409)
	}
	e = tx.Commit()
	return Entry{ID: id, Name: name, Parent: parent, Kind: "folder", Generation: 1}, e
}
func (s *Service) list(w http.ResponseWriter, r *http.Request, id string) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 200 {
		limit = 100
	}
	deleted := 0
	if strings.Contains(r.URL.Path, "/trash") {
		deleted = 1
	}
	rows, e := s.DB.Query("SELECT e.id,parent,name,kind,current_version,deleted,generation,COALESCE(v.size,0) FROM entries e LEFT JOIN versions v ON e.current_version=v.id WHERE e.project=? AND (?=1 AND deleted>0 OR ?=0 AND deleted=0 AND parent=?) AND e.id>? AND (kind='folder' OR current_version!='') ORDER BY e.id LIMIT ?", id, deleted, deleted, r.URL.Query().Get("parentId"), r.URL.Query().Get("cursor"), limit+1)
	if e != nil {
		reject(w, e)
		return
	}
	defer rows.Close()
	items := []Entry{}
	for rows.Next() {
		var v Entry
		if e = rows.Scan(&v.ID, &v.Parent, &v.Name, &v.Kind, &v.Version, &v.Deleted, &v.Generation, &v.Size); e != nil {
			reject(w, e)
			return
		}
		items = append(items, v)
	}
	cursor := ""
	if len(items) > limit {
		items = items[:limit]
		cursor = items[len(items)-1].ID
	}
	respond(w, 200, map[string]any{"items": items, "nextCursor": cursor})
}
func (s *Service) moveEntry(project, id, parent, name string, generation int64) error {
	if !validName(name) {
		return fail("INVALID_NAME", "invalid name", 400)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, e := s.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if e = s.parent(tx, project, parent); e != nil {
		return e
	}
	var n int
	tx.QueryRow("WITH RECURSIVE tree(id) AS (SELECT id FROM entries WHERE id=? AND project=? UNION ALL SELECT e.id FROM entries e JOIN tree ON e.parent=tree.id WHERE e.project=?) SELECT COUNT(*) FROM tree WHERE id=?", id, project, project, parent).Scan(&n)
	if n > 0 {
		return fail("FOLDER_CYCLE", "folder move forms a cycle", 409)
	}
	res, e := tx.Exec("UPDATE entries SET parent=?,name=?,generation=generation+1 WHERE id=? AND project=? AND deleted=0 AND generation=?", parent, name, id, project, generation)
	if e != nil {
		return fail("NAME_CONFLICT", "name already exists", 409)
	}
	n64, _ := res.RowsAffected()
	if n64 != 1 {
		return fail("VERSION_CONFLICT", "entry generation changed", 409)
	}
	return tx.Commit()
}
func (s *Service) deleteEntry(project, id string, recursive bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.entry(project, id, false)
	if e != nil {
		return e
	}
	var n int
	s.DB.QueryRow("SELECT COUNT(*) FROM entries WHERE parent=? AND project=? AND deleted=0", id, project).Scan(&n)
	if n > 0 && !recursive {
		return fail("DIRECTORY_NOT_EMPTY", "explicit recursive=true required", 409)
	}
	tx, e := s.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	stamp := time.Now().UnixNano()
	_, e = tx.Exec("WITH RECURSIVE tree(id) AS (SELECT id FROM entries WHERE id=? AND project=? UNION ALL SELECT e.id FROM entries e JOIN tree ON e.parent=tree.id WHERE e.project=? AND e.deleted=0) UPDATE entries SET deleted=?,generation=generation+1 WHERE id IN tree", v.ID, project, project, stamp)
	if e != nil {
		return e
	}
	_, e = tx.Exec("INSERT INTO operations SELECT lower(hex(randomblob(16))),v.project,v.object,'trash','pending',v.path,'trash/'||v.object||'/'||v.id||'/blob','' FROM versions v JOIN entries e ON v.object=e.id WHERE e.project=? AND e.deleted=?", project, stamp)
	if e != nil {
		return e
	}
	_, e = tx.Exec("UPDATE jobs SET state='cancelled' WHERE project=? AND object IN (SELECT id FROM entries WHERE project=? AND deleted=?)", project, project, stamp)
	if e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Service) restoreEntry(project, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.entry(project, id, true)
	if e != nil {
		return e
	}
	if v.Deleted == 0 {
		return fail("NOT_DELETED", "entry is not deleted", 409)
	}
	tx, e := s.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if e = s.parent(tx, project, v.Parent); e != nil {
		return e
	}
	var n int
	tx.QueryRow("SELECT COUNT(*) FROM operations WHERE project=? AND state!='done'", project).Scan(&n)
	if n > 0 {
		return fail("OPERATION_PENDING", "wait for pending moves before restore", 409)
	}
	_, e = tx.Exec("UPDATE entries SET deleted=0,generation=generation+1 WHERE project=? AND deleted=?", project, v.Deleted)
	if e != nil {
		return fail("NAME_CONFLICT", "restore conflicts with a current name", 409)
	}
	_, e = tx.Exec("INSERT INTO operations SELECT lower(hex(randomblob(16))),v.project,v.object,'restore','pending',v.path,'original/'||v.object||'/'||v.id||'/blob','' FROM versions v JOIN entries e ON e.id=v.object WHERE e.project=? AND v.path LIKE 'trash/%' AND e.deleted=0", project)
	if e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Service) download(w http.ResponseWriter, r *http.Request, p Project, object string) {
	r, done := s.guardObject(w, r, p.ID, object)
	defer done()
	var path, mime, sha string
	var size, stamp int64
	version := r.URL.Query().Get("versionId")
	q := "SELECT v.path,v.mime,v.sha256,v.size,v.stamp FROM versions v JOIN entries e ON e.id=v.object WHERE e.project=? AND e.id=? AND e.deleted=0 AND v.id=e.current_version"
	args := []any{p.ID, object}
	if version != "" {
		q = "SELECT v.path,v.mime,v.sha256,v.size,v.stamp FROM versions v JOIN entries e ON e.id=v.object WHERE e.project=? AND e.id=? AND e.deleted=0 AND v.id=?"
		args = append(args, version)
	}
	if e := s.DB.QueryRowContext(r.Context(), q, args...).Scan(&path, &mime, &sha, &size, &stamp); e != nil {
		reject(w, fail("NOT_FOUND", "object not ready or deleted", 404))
		return
	}
	if e := s.checkFormal(p, path, size, stamp); e != nil {
		reject(w, e)
		return
	}
	s.stream(w, r, p, path, mime, sha, size)
}
func (s *Service) downloadDerived(w http.ResponseWriter, r *http.Request, p Project, id string) {
	var path, mime, sha, object string
	var size, stamp int64
	e := s.DB.QueryRow("SELECT d.path,d.mime,d.sha256,d.size,d.stamp,d.object FROM derivatives d JOIN entries e ON e.id=d.object WHERE d.id=? AND d.project=? AND e.deleted=0 AND e.current_version=d.version", id, p.ID).Scan(&path, &mime, &sha, &size, &stamp, &object)
	if e != nil {
		reject(w, fail("NOT_FOUND", "derivative not found", 404))
		return
	}
	r, done := s.guardObject(w, r, p.ID, object)
	defer done()
	if e := s.checkFormal(p, path, size, stamp); e != nil {
		reject(w, e)
		return
	}
	if mime == "application/vnd.apple.mpegurl" && r.Method == "GET" {
		if e := s.hls(w, r, p, path); e != nil {
			reject(w, e)
		}
		return
	}
	s.stream(w, r, p, path, mime, sha, size)
}
func (s *Service) stream(w http.ResponseWriter, r *http.Request, p Project, path, mime, sha string, size int64) {
	etag := "\"" + sha + "\""
	w.Header().Set("ETag", etag)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Type", mime)
	if match := r.Header.Get("If-Match"); match != "" {
		matched := false
		for _, tag := range strings.Split(match, ",") {
			tag = strings.TrimSpace(tag)
			if tag == "*" || tag == etag {
				matched = true
				break
			}
		}
		if !matched {
			w.WriteHeader(412)
			return
		}
	}
	for _, tag := range strings.Split(r.Header.Get("If-None-Match"), ",") {
		tag = strings.TrimSpace(tag)
		if strings.TrimPrefix(tag, "W/") == etag || tag == "*" {
			w.WriteHeader(304)
			return
		}
	}
	if r.Method == "HEAD" {
		w.Header().Set("Content-Length", fmt.Sprint(size))
		w.WriteHeader(200)
		return
	}
	rangeHeader := r.Header.Get("Range")
	if ir := r.Header.Get("If-Range"); ir != "" && ir != etag {
		rangeHeader = ""
	}
	res, e := s.Backend.Download(r.Context(), p.Key, path, rangeHeader)
	if e != nil {
		reject(w, e)
		return
	}
	defer res.Body.Close()
	if res.StatusCode != 200 && res.StatusCode != 206 && res.StatusCode != 416 {
		reject(w, fail("BACKEND_UNAVAILABLE", "formal file unavailable", 503))
		return
	}
	for _, h := range []string{"Content-Length", "Content-Range"} {
		if v := res.Header.Get(h); v != "" {
			w.Header().Set(h, v)
		}
	}
	w.WriteHeader(res.StatusCode)
	io.CopyBuffer(w, res.Body, make([]byte, 64<<10))
}
func (s *Service) validCommit(tx *sql.Tx, p Principal, scope string) error {
	var enabled, quarantine bool
	if e := tx.QueryRow("SELECT enabled,quarantined FROM projects WHERE id=?", p.Project).Scan(&enabled, &quarantine); e != nil || !enabled || quarantine {
		return fail("PROJECT_DISABLED", "project disabled", 403)
	}
	if p.Manager {
		return nil
	}
	var n int
	tx.QueryRow("SELECT COUNT(*) FROM credentials WHERE id=? AND project=? AND revoked=0", p.ID, p.Project).Scan(&n)
	if n != 1 {
		return fail("CREDENTIAL_REVOKED", "credential revoked", 401)
	}
	return nil
}
func (s *Service) credentialPrincipal(project, id string) (Principal, error) {
	p := Principal{Project: project, ID: id, Manager: id == "manager"}
	if p.Manager {
		return p, nil
	}
	var scopes string
	e := s.DB.QueryRow("SELECT scopes FROM credentials WHERE project=? AND id=? AND revoked=0", project, id).Scan(&scopes)
	json.Unmarshal([]byte(scopes), &p.Scopes)
	return p, e
}

var _ = context.Background

func (s *Service) listDerivatives(w http.ResponseWriter, project, object string) {
	rows, e := s.DB.Query("SELECT d.id,d.version,d.preset,d.path,d.size,d.mime FROM derivatives d JOIN entries e ON e.id=d.object WHERE d.project=? AND d.object=? AND e.deleted=0 AND e.current_version=d.version", project, object)
	if e != nil {
		reject(w, e)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, version, preset, file, mime string
		var size int64
		rows.Scan(&id, &version, &preset, &file, &size, &mime)
		items = append(items, map[string]any{"id": id, "versionId": version, "preset": preset, "name": path.Base(file), "size": size, "mime": mime})
	}
	respond(w, 200, map[string]any{"items": items})
}
