package storage

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/tus/tusd/v2/pkg/filelocker"
	"github.com/tus/tusd/v2/pkg/filestore"
	"github.com/tus/tusd/v2/pkg/handler"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"yandu/internal/model"
	"yandu/internal/pathpolicy"
	"yandu/internal/platform"
)

type Intent struct {
	ID, Project, Credential, Parent, Name, Object, Version, Expected, State, Presets, Snapshot string
	Length                                                                                     int64
}
type IntentRequest struct {
	Parent     string          `json:"parentId"`
	Name       string          `json:"name"`
	Length     int64           `json:"length"`
	Object     string          `json:"objectId,omitempty"`
	Expected   string          `json:"expectedVersionId,omitempty"`
	Processing ProcessingInput `json:"processing"`
}

func (s *Service) intent(id, project string) (Intent, error) {
	var v Intent
	e := s.DB.QueryRow("SELECT id,project,credential,parent,name,object,version,expected,length,state,presets,processing_snapshot FROM uploads WHERE id=? AND project=?", id, project).Scan(&v.ID, &v.Project, &v.Credential, &v.Parent, &v.Name, &v.Object, &v.Version, &v.Expected, &v.Length, &v.State, &v.Presets, &v.Snapshot)
	return v, e
}
func (s *Service) createIntent(w http.ResponseWriter, r *http.Request, p Principal, pr Project) {
	var v IntentRequest
	if e := decode(r, &v); e != nil {
		reject(w, e)
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if len(key) < 1 || len(key) > 128 {
		reject(w, fail("IDEMPOTENCY_REQUIRED", "Idempotency-Key (1–128 characters) required", 400))
		return
	}
	if !validName(v.Name) || v.Length < 0 || v.Length > 1<<50 {
		reject(w, fail("INVALID_INTENT", "invalid name or length", 400))
		return
	}
	if e := validatePresets(v.Processing.Presets); e != nil {
		reject(w, e)
		return
	}
	if len(v.Processing.Presets) > 0 {
		if e := s.allowed(r.Context(), p, "process"); e != nil {
			reject(w, e)
			return
		}
		if len(s.MediaCommand) == 0 {
			reject(w, fail("MEDIA_UNAVAILABLE", "media components not configured", 503))
			return
		}
	}
	data, _ := json.Marshal(v)
	hash := digest(string(data))
	s.mu.Lock()
	defer s.mu.Unlock()
	var existing, oldhash, existingObject, existingVersion string
	e := s.DB.QueryRow("SELECT id,request_hash,object,version FROM uploads WHERE project=? AND credential=? AND idempotency=?", pr.ID, p.ID, key).Scan(&existing, &oldhash, &existingObject, &existingVersion)
	if e == nil {
		if oldhash != hash {
			reject(w, fail("IDEMPOTENCY_CONFLICT", "key already used for another intent", 409))
			return
		}
		respond(w, 200, map[string]string{"id": existing, "objectId": existingObject, "versionId": existingVersion, "tusEndpoint": "/storage/v1/projects/" + pr.ID + "/tus/"})
		return
	}
	volume, e := s.checkVolume(pr)
	if e != nil {
		reject(w, e)
		return
	}
	tx, e := s.DB.BeginTx(r.Context(), nil)
	if e != nil {
		reject(w, e)
		return
	}
	defer tx.Rollback()
	if e = tx.QueryRow("SELECT mp4_max_file_bytes,policy_revision FROM projects WHERE id=?", pr.ID).Scan(&pr.MP4MaxFileBytes, &pr.PolicyRevision); e != nil {
		reject(w, e)
		return
	}
	snapshot, e := s.processingSnapshot(pr, v.Processing)
	if e != nil {
		reject(w, e)
		return
	}
	snapshotData, _ := json.Marshal(snapshot)
	if e = s.parent(tx, pr.ID, v.Parent); e != nil {
		reject(w, e)
		return
	}
	if e = s.validCommit(tx, p, "upload"); e != nil {
		reject(w, e)
		return
	}
	reserve := v.Length * 2
	if len(v.Processing.Presets) > 0 {
		reserve += v.Length*4 + 512<<20
	}
	var used, reserved, poolReserved int64
	tx.QueryRow("SELECT COALESCE((SELECT SUM(size) FROM versions WHERE project=?),0)+COALESCE((SELECT SUM(size) FROM derivatives WHERE project=?),0)", pr.ID, pr.ID).Scan(&used)
	tx.QueryRow("SELECT COALESCE(SUM(bytes),0) FROM reservations WHERE project=?", pr.ID).Scan(&reserved)
	tx.QueryRow("SELECT COALESCE(SUM(bytes),0) FROM reservations WHERE pool=?", pr.Pool).Scan(&poolReserved)
	if used+reserved+reserve > pr.Quota || poolReserved+reserve+(64<<20) > volume.Free {
		reject(w, fail("INSUFFICIENT_STORAGE", "quota or shared volume reservation exhausted", 507))
		return
	}
	object := v.Object
	if object == "" {
		object = model.ID()
		_, e = tx.Exec("INSERT INTO entries(id,project,parent,name,kind,created) VALUES(?,?,?,?,'file',?)", object, pr.ID, v.Parent, v.Name, time.Now().Unix())
		if e != nil {
			reject(w, fail("NAME_CONFLICT", "name already exists", 409))
			return
		}
	} else {
		var current string
		var deleted int64
		e = tx.QueryRow("SELECT current_version,deleted FROM entries WHERE id=? AND project=? AND kind='file'", object, pr.ID).Scan(&current, &deleted)
		if e != nil || deleted != 0 {
			reject(w, fail("NOT_FOUND", "object not found", 404))
			return
		}
		if v.Expected == "" || v.Expected != current {
			reject(w, fail("VERSION_CONFLICT", "expectedVersionId required and must match", 409))
			return
		}
	}
	id, version := model.ID(), model.ID()
	presets, _ := json.Marshal(v.Processing.Presets)
	_, e = tx.Exec("INSERT INTO uploads(id,project,credential,parent,name,object,version,expected,length,state,presets,request_hash,idempotency,created,processing_snapshot) VALUES(?,?,?,?,?,?,?,?,?,'uploading',?,?,?,?,?)", id, pr.ID, p.ID, v.Parent, v.Name, object, version, v.Expected, v.Length, string(presets), hash, key, time.Now().Unix(), string(snapshotData))
	if e == nil {
		_, e = tx.Exec("INSERT INTO reservations VALUES(?,?,?,?)", id, pr.ID, pr.Pool, reserve)
	}
	if e == nil {
		e = tx.Commit()
	}
	if e != nil {
		reject(w, e)
		return
	}
	respond(w, 201, map[string]string{"id": id, "objectId": object, "versionId": version, "tusEndpoint": "/storage/v1/projects/" + pr.ID + "/tus/"})
}
func (s *Service) tusHandler(pr Project) (*handler.Handler, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if h := s.tus[pr.ID]; h != nil {
		return h, nil
	}
	dir := filepath.Join(pr.Root, "temporary", "uploads")
	store := filestore.New(dir)
	locker := filelocker.New(dir)
	composer := handler.NewStoreComposer()
	store.UseIn(composer)
	locker.UseIn(composer)
	h, e := handler.NewHandler(handler.Config{BasePath: "/storage/v1/projects/" + pr.ID + "/tus/", StoreComposer: composer, DisableDownload: true, DisableTermination: true, DisableConcatenation: true, GracefulRequestCompletionTimeout: time.Second, AcquireLockTimeout: time.Second, MaxSize: 1 << 50,
		PreUploadCreateCallback: func(h handler.HookEvent) (handler.HTTPResponse, handler.FileInfoChanges, error) {
			id := h.Upload.MetaData["intentId"]
			v, e := s.intent(id, pr.ID)
			if e != nil || v.State != "uploading" || h.Upload.SizeIsDeferred || v.Length != h.Upload.Size {
				return handler.HTTPResponse{}, handler.FileInfoChanges{}, handler.NewError("INVALID_INTENT", "valid intent and exact Upload-Length required", 400)
			}
			p, ok := h.Context.Value(ctxPrincipal{}).(Principal)
			if !ok || (!p.Manager && p.ID != v.Credential) {
				return handler.HTTPResponse{}, handler.FileInfoChanges{}, handler.NewError("OWNER_DENIED", "intent owner required", 403)
			}
			return handler.HTTPResponse{}, handler.FileInfoChanges{ID: id, MetaData: handler.MetaData{"intentId": id}}, nil
		},
		PreFinishResponseCallback: func(h handler.HookEvent) (handler.HTTPResponse, error) {
			_, e := s.DB.Exec("UPDATE uploads SET state='pending' WHERE id=? AND project=? AND state='uploading'", h.Upload.ID, pr.ID)
			return handler.HTTPResponse{}, e
		}})
	if e == nil {
		s.tus[pr.ID] = h
	}
	return h, e
}
func (s *Service) serveTus(w http.ResponseWriter, r *http.Request, p Principal, pr Project, parts []string) {
	id := ""
	if len(parts) == 6 {
		id = parts[5]
	} else if len(parts) > 6 {
		http.NotFound(w, r)
		return
	}
	if r.Method == "POST" {
		for _, v := range strings.Split(r.Header.Get("Upload-Metadata"), ",") {
			bits := strings.Fields(v)
			if len(bits) == 2 && bits[0] == "intentId" {
				data, _ := base64.StdEncoding.DecodeString(bits[1])
				id = string(data)
			}
		}
	}
	if r.Method != "OPTIONS" {
		intent, e := s.intent(id, pr.ID)
		if e != nil || (!p.Manager && intent.Credential != p.ID) {
			reject(w, fail("NOT_FOUND", "upload not found", 404))
			return
		}
		if intent.State == "failed" || intent.State == "cancelled" {
			reject(w, fail("UPLOAD_CLOSED", "upload closed", 409))
			return
		}
		if r.Method == "PATCH" && intent.State != "uploading" {
			reject(w, fail("UPLOAD_CLOSED", "upload already completed", 409))
			return
		}
		if r.Method == "POST" {
			if _, e = os.Stat(filepath.Join(pr.Root, "temporary", "uploads", id+".info")); e == nil {
				w.Header().Set("Tus-Resumable", "1.0.0")
				w.Header().Set("Location", "/storage/v1/projects/"+pr.ID+"/tus/"+id)
				w.WriteHeader(201)
				return
			}
		}
	}
	h, e := s.tusHandler(pr)
	if e != nil {
		reject(w, e)
		return
	}
	prefix := "/storage/v1/projects/" + pr.ID + "/tus/"
	http.StripPrefix(strings.TrimSuffix(prefix, "/"), h).ServeHTTP(w, r)
}
func (s *Service) recoverUploads() {
	rows, e := s.DB.Query("SELECT id,project FROM uploads WHERE state IN ('uploading','pending','committing','cleanup') ORDER BY created LIMIT 50")
	if e != nil {
		return
	}
	type ref struct{ id, project string }
	refs := []ref{}
	for rows.Next() {
		var v ref
		rows.Scan(&v.id, &v.project)
		refs = append(refs, v)
	}
	rows.Close()
	for _, ref := range refs {
		if s.ctx.Err() != nil {
			return
		}
		v, e := s.intent(ref.id, ref.project)
		if e != nil {
			continue
		}
		pr, e := s.project(ref.project)
		if e != nil {
			continue
		}
		if v.State == "cleanup" {
			s.failUpload(pr, v, fail("UPLOAD_CANCELLED", "cleaning failed candidate", 409))
			continue
		}
		if v.State == "uploading" {
			st := filestore.New(filepath.Join(pr.Root, "temporary", "uploads"))
			up, e := st.GetUpload(s.ctx, v.ID)
			if e != nil {
				continue
			}
			info, e := up.GetInfo(s.ctx)
			if e != nil || info.Offset != info.Size {
				continue
			}
			s.DB.Exec("UPDATE uploads SET state='pending' WHERE id=?", v.ID)
		}
		e = s.commitUpload(s.ctx, pr, v)
		if e != nil {
			s.DB.Exec("UPDATE uploads SET error=? WHERE id=?", e.Error(), v.ID)
			var a *apiError
			if errors.As(e, &a) && a.Status < 500 {
				s.failUpload(pr, v, e)
			}
		}
	}
}
func (s *Service) failUpload(pr Project, v Intent, e error) {
	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
	defer cancel()
	// Failed candidates have immutable unique version IDs. Remove them before releasing disk reservations.
	var registered int
	s.DB.QueryRow("SELECT COUNT(*) FROM versions WHERE id=?", v.Version).Scan(&registered)
	if registered == 0 {
		if err := s.Backend.Remove(ctx, pr.Key, "original/"+v.Object+"/"+v.Version+"/blob"); err != nil {
			s.DB.Exec("UPDATE uploads SET state='cleanup',error=? WHERE id=?", e.Error(), v.ID)
			return
		}
	}
	if err := s.Backend.Remove(ctx, pr.Key, ".ingest/"+v.ID); err != nil {
		s.DB.Exec("UPDATE uploads SET state='cleanup',error=? WHERE id=?", e.Error(), v.ID)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.DB.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()
	tx.Exec("UPDATE uploads SET state='failed',error=? WHERE id=?", e.Error(), v.ID)
	tx.Exec("DELETE FROM reservations WHERE id=?", v.ID)
	tx.Exec("DELETE FROM entries WHERE id=? AND current_version=''", v.Object)
	tx.Commit()
	st := filestore.New(filepath.Join(pr.Root, "temporary", "uploads"))
	if u, err := st.GetUpload(ctx, v.ID); err == nil {
		st.AsTerminatableUpload(u).Terminate(ctx)
	}
}
func (s *Service) mkdirs(ctx context.Context, key, dir string) error {
	parts := strings.Split(strings.Trim(dir, "/"), "/")
	for i := range parts {
		p := strings.Join(parts[:i+1], "/")
		if e := s.Backend.Mkdir(ctx, key, p); e != nil { // API mkdir is idempotent only if directory exists; validate via a content read is not useful.
			if b, ok := s.Backend.(*SFTPGo); ok {
				res, check := b.call(ctx, key, "GET", "/api/v2/user/dirs?path="+url.QueryEscape(p), nil)
				if check == nil {
					res.Body.Close()
					continue
				}
			}
			return e
		}
	}
	return nil
}
func (s *Service) verify(ctx context.Context, pr Project, path, sha string, size int64) error {
	res, e := s.Backend.Download(ctx, pr.Key, path, "")
	if e != nil {
		return e
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("formal object missing HTTP %d", res.StatusCode)
	}
	h := sha256.New()
	n, e := io.CopyBuffer(h, res.Body, make([]byte, 64<<10))
	if e != nil {
		return e
	}
	if n != size || hex.EncodeToString(h.Sum(nil)) != sha {
		return fail("INTEGRITY_FAILED", "formal object checksum mismatch", 503)
	}
	return nil
}
func (s *Service) commitUpload(ctx context.Context, pr Project, v Intent) error {
	p, e := s.credentialPrincipal(pr.ID, v.Credential)
	if e != nil {
		return fail("CREDENTIAL_REVOKED", "upload credential revoked", 401)
	}
	req, _ := http.NewRequestWithContext(ctx, "POST", "http://localhost", nil)
	req, cancel, e := s.guardContext(req, p, "upload")
	if e != nil {
		return e
	}
	defer cancel()
	ctx = req.Context()
	if _, e = s.checkVolume(pr); e != nil {
		return e
	}
	file := filepath.Join(pr.Root, "temporary", "uploads", v.ID)
	if e = pathpolicy.NoLinks(file); e != nil {
		return e
	}
	if _, e = platform.RegularFile(file); e != nil {
		return e
	}
	f, e := os.Open(file)
	if e != nil {
		return e
	}
	defer f.Close()
	h := sha256.New()
	head := make([]byte, 512)
	n, _ := f.Read(head)
	f.Seek(0, 0)
	size, e := io.CopyBuffer(h, f, make([]byte, 64<<10))
	if e != nil {
		return e
	}
	if size != v.Length {
		return fail("LENGTH_MISMATCH", "length mismatch", 400)
	}
	sha := hex.EncodeToString(h.Sum(nil))
	mime := http.DetectContentType(head[:n])
	source := ".ingest/" + v.ID
	target := "original/" + v.Object + "/" + v.Version + "/blob"
	s.DB.Exec("UPDATE uploads SET state='committing' WHERE id=?", v.ID)
	// A successful move may have lost its response. The immutable target checksum makes replay safe.
	if e = s.verify(ctx, pr, target, sha, size); e != nil {
		f.Seek(0, 0)
		if e = s.Backend.Upload(ctx, pr.Key, source, f, size); e != nil {
			return e
		}
		if e = s.verify(ctx, pr, source, sha, size); e != nil {
			return e
		}
		if e = s.mkdirs(ctx, pr.Key, path.Dir(target)); e != nil {
			return e
		}
		if e = s.Backend.Move(ctx, pr.Key, source, target); e != nil {
			if verify := s.verify(ctx, pr, target, sha, size); verify != nil {
				return e
			}
		}
	}
	stamp, e := s.formalStamp(pr, target, size)
	if e != nil {
		return e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if e = s.validCommit(tx, p, "upload"); e != nil {
		return e
	}
	var current string
	var deleted int64
	e = tx.QueryRow("SELECT current_version,deleted FROM entries WHERE id=? AND project=?", v.Object, pr.ID).Scan(&current, &deleted)
	if e != nil || deleted != 0 {
		return fail("OBJECT_DELETED", "object deleted during upload", 409)
	}
	if current != v.Expected {
		return fail("VERSION_CONFLICT", "another version won replacement", 409)
	}
	_, e = tx.Exec("INSERT INTO versions(id,project,object,size,sha256,mime,path,created,stamp) VALUES(?,?,?,?,?,?,?,?,?)", v.Version, pr.ID, v.Object, size, sha, mime, target, time.Now().Unix(), stamp)
	if e != nil {
		return e
	}
	_, e = tx.Exec("UPDATE entries SET current_version=?,generation=generation+1 WHERE id=?", v.Version, v.Object)
	if e != nil {
		return e
	}
	tx.Exec("UPDATE uploads SET state='READY',error='' WHERE id=?", v.ID)
	var presets []string
	json.Unmarshal([]byte(v.Presets), &presets)
	if len(presets) > 0 {
		if e = s.validCommit(tx, p, "process"); e != nil {
			return e
		}
		var snap ProcessingSnapshot
		if e = json.Unmarshal([]byte(v.Snapshot), &snap); e != nil {
			return e
		}
		if snap.ProcessingVersion == "" {
			snap.ProcessingVersion = s.ProcessingVersion
		}
		if _, e = s.insertJobs(tx, p, v.Object, v.Version, v.ID, "upload:"+v.ID, presets, snap); e != nil {
			return e
		}
	}
	tx.Exec("DELETE FROM reservations WHERE id=?", v.ID)
	payload, _ := json.Marshal(map[string]string{"objectId": v.Object, "versionId": v.Version})
	tx.Exec("INSERT INTO outbox VALUES(?,?,'object.ready',?,?)", model.ID(), pr.ID, string(payload), time.Now().Unix())
	if e = tx.Commit(); e != nil {
		return e
	}
	f.Close()
	st := filestore.New(filepath.Join(pr.Root, "temporary", "uploads"))
	if up, e := st.GetUpload(ctx, v.ID); e == nil {
		st.AsTerminatableUpload(up).Terminate(ctx)
	}
	return nil
}
func (s *Service) recoverMoves() {
	rows, e := s.DB.Query("SELECT id,project,object,source,target FROM operations WHERE state='pending' LIMIT 100")
	if e != nil {
		return
	}
	type op struct{ id, project, object, source, target string }
	ops := []op{}
	for rows.Next() {
		var v op
		rows.Scan(&v.id, &v.project, &v.object, &v.source, &v.target)
		ops = append(ops, v)
	}
	rows.Close()
	for _, v := range ops {
		pr, e := s.project(v.project)
		if e != nil {
			continue
		}
		if _, e = s.checkVolume(pr); e != nil {
			continue
		}
		if e = s.mkdirs(s.ctx, pr.Key, path.Dir(v.target)); e == nil {
			e = s.Backend.Move(s.ctx, pr.Key, v.source, v.target)
		}
		if e != nil {
			var sha string
			var size int64
			s.DB.QueryRow("SELECT sha256,size FROM versions WHERE project=? AND path=?", pr.ID, v.source).Scan(&sha, &size)
			if s.verify(s.ctx, pr, v.target, sha, size) != nil {
				s.DB.Exec("UPDATE operations SET error=? WHERE id=?", e.Error(), v.id)
				continue
			}
		}
		tx, e := s.DB.Begin()
		if e != nil {
			continue
		}
		var size int64
		tx.QueryRow("SELECT size FROM versions WHERE project=? AND path=?", pr.ID, v.source).Scan(&size)
		stamp, e := s.formalStamp(pr, v.target, size)
		if e != nil {
			tx.Rollback()
			continue
		}
		tx.Exec("UPDATE versions SET path=?,stamp=? WHERE project=? AND path=?", v.target, stamp, pr.ID, v.source)
		tx.Exec("UPDATE operations SET state='done',error='' WHERE id=?", v.id)
		tx.Commit()
	}
}
