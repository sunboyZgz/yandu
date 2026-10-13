package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/pressly/goose/v3"
	"github.com/tus/tusd/v2/pkg/handler"
	"io"
	"io/fs"
	_ "modernc.org/sqlite"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"yandu/internal/model"
	"yandu/internal/platform"
)

//go:embed migrations/*.sql
var migrations embed.FS

type Service struct {
	DB                *sql.DB
	Backend           Backend
	Dir               string
	mu                sync.Mutex
	mediaMu           sync.Mutex
	mediaActive       map[string]bool
	tus               map[string]*handler.Handler
	ctx               context.Context
	cancel            context.CancelFunc
	done              chan struct{}
	MediaCommand      []string
	MediaEnvironment  map[string]string
	ProcessingVersion string
	InventorySource   func() (model.Snapshot, error)
}
type Project struct {
	ID, Root, Key, Volume, Pool     string
	MP4MaxFileBytes, PolicyRevision int64
	Quota                           int64
	Enabled, Quarantined            bool
}
type Principal struct {
	ID, Project string
	Scopes      []string
	Manager     bool
}
type ctxPrincipal struct{}
type apiError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Status  int            `json:"-"`
	Details map[string]any `json:"details,omitempty"`
}

func (e *apiError) Error() string { return e.Code + ": " + e.Message }
func fail(code, msg string, status int) error {
	return &apiError{Code: code, Message: msg, Status: status}
}
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func reject(w http.ResponseWriter, e error) {
	var a *apiError
	if !errors.As(e, &a) {
		a = &apiError{Code: "STORAGE_UNAVAILABLE", Message: "storage component unavailable; inspect local operation logs", Status: 503}
	}
	respond(w, a.Status, map[string]any{"error": a})
}
func decode(r *http.Request, v any) error {
	defer r.Body.Close()
	d := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 65536))
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return fail("INVALID_REQUEST", e.Error(), 400)
	}
	var extra any
	if e := d.Decode(&extra); e != io.EOF {
		return fail("INVALID_REQUEST", "one JSON document required", 400)
	}
	return nil
}
func digest(s string) string { sum := sha256.Sum256([]byte(s)); return hex.EncodeToString(sum[:]) }

type Option func(*Service)

func WithInventorySource(source func() (model.Snapshot, error)) Option {
	return func(s *Service) { s.InventorySource = source }
}

func WithMediaCommand(command []string) Option {
	return func(s *Service) { s.MediaCommand = append([]string{}, command...) }
}

func WithMediaEnvironment(env map[string]string) Option {
	return func(s *Service) {
		s.MediaEnvironment = map[string]string{}
		for k, v := range env {
			s.MediaEnvironment[k] = v
		}
	}
}
func Open(ctx context.Context, dir string, backend Backend, options ...Option) (*Service, error) {
	if e := platform.PrivateDir(dir); e != nil {
		return nil, e
	}
	db, e := sql.Open("sqlite", filepath.Join(dir, "storage.db"))
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	if _, e = db.Exec("PRAGMA foreign_keys=ON; PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;"); e != nil {
		db.Close()
		return nil, e
	}
	provider, e := goose.NewProvider(goose.DialectSQLite3, db, mustSub())
	if e != nil {
		db.Close()
		return nil, e
	}
	if _, e = provider.Up(ctx); e != nil {
		db.Close()
		return nil, e
	}
	os.Chmod(filepath.Join(dir, "storage.db"), 0600)
	c, cancel := context.WithCancel(ctx)
	s := &Service{DB: db, Backend: backend, Dir: dir, tus: map[string]*handler.Handler{}, mediaActive: map[string]bool{}, ctx: c, cancel: cancel, done: make(chan struct{})}
	for _, option := range options {
		option(s)
	}
	s.ProcessingVersion = s.mediaFingerprint()
	go s.loop()
	return s, nil
}
func mustSub() fs.FS {
	sub, e := fs.Sub(migrations, "migrations")
	if e != nil {
		panic(e)
	}
	return sub
}
func (s *Service) Close() { s.cancel(); <-s.done; s.DB.Close() }
func (s *Service) project(id string) (Project, error) {
	var p Project
	e := s.DB.QueryRow("SELECT id,root,backend_key,quota,enabled,volume,pool,quarantined,mp4_max_file_bytes,policy_revision FROM projects WHERE id=?", id).Scan(&p.ID, &p.Root, &p.Key, &p.Quota, &p.Enabled, &p.Volume, &p.Pool, &p.Quarantined, &p.MP4MaxFileBytes, &p.PolicyRevision)
	if e != nil {
		return p, fail("PROJECT_NOT_FOUND", "project unavailable", 404)
	}
	return p, nil
}
func (s *Service) Configure(ctx context.Context, p model.Project) error {
	if !p.IsStorage() {
		return nil
	}
	if e := p.ValidateMediaPolicy(); e != nil {
		return e
	}
	var mediaLimit int64
	if p.MediaPolicy != nil {
		mediaLimit = p.MediaPolicy.MP4MaxFileBytes
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.QuotaBytes <= 0 {
		return fail("INVALID_QUOTA", "quota must be positive", 400)
	}
	volume, e := platform.CheckRoot(p.RootPath)
	if e != nil {
		return e
	}
	old, e := s.project(p.ID)
	if e == nil {
		if old.Root != p.RootPath || old.Volume != volume.ID || old.Pool != volume.Pool {
			return fail("VOLUME_CHANGED", "maintenance migration required", 409)
		}
		if old.Key != "" {
			if b, ok := s.Backend.(*SFTPGo); ok {
				if e = b.ValidateHome(ctx, p.ID, filepath.Join(p.RootPath, "files")); e != nil {
					return e
				}
			}
			_, e = s.DB.Exec("UPDATE projects SET quota=?,enabled=?,mp4_max_file_bytes=?,policy_revision=? WHERE id=?", p.QuotaBytes, p.Publication == "active", mediaLimit, p.Revision, p.ID)
			return e
		}
	} else {
		entries, err := os.ReadDir(p.RootPath)
		if err != nil {
			return err
		}
		if len(entries) > 0 && !preparedScaffold(p.RootPath) {
			return fail("IMPORT_REQUIRED", "managed root must be empty; import existing files explicitly", 409)
		}
		if _, e = s.DB.Exec("INSERT INTO projects(id,root,quota,enabled,backend_key,volume,pool) VALUES(?,?,?,0,'',?,?)", p.ID, p.RootPath, p.QuotaBytes, volume.ID, volume.Pool); e != nil {
			return e
		}
	}
	for _, dir := range []string{"files", "temporary/uploads", "temporary/jobs"} {
		if e = platform.PrivateDir(filepath.Join(p.RootPath, dir)); e != nil {
			return e
		}
	}
	key, e := s.Backend.Provision(ctx, p.ID, filepath.Join(p.RootPath, "files"), p.QuotaBytes)
	if e != nil {
		return e
	}
	for _, dir := range []string{"original", "derived", "trash", ".ingest"} {
		if e = s.mkdirs(ctx, key, dir); e != nil {
			return e
		}
	}
	_, e = s.DB.Exec("UPDATE projects SET backend_key=?,quota=?,enabled=?,mp4_max_file_bytes=?,policy_revision=? WHERE id=?", key, p.QuotaBytes, p.Publication == "active", mediaLimit, p.Revision, p.ID)
	return e
}
func (s *Service) Disable(id string) error {
	_, e := s.DB.Exec("UPDATE projects SET enabled=0 WHERE id=?", id)
	return e
}
func (s *Service) CheckProject(ctx context.Context, id string) error {
	p, e := s.project(id)
	if e != nil {
		return e
	}
	if p.Quarantined {
		return fail("INTEGRITY_QUARANTINE", "managed storage requires maintenance verification", 503)
	}
	if _, e = s.checkVolume(p); e != nil {
		return e
	}
	if b, ok := s.Backend.(*SFTPGo); ok {
		return b.ValidateHome(ctx, id, filepath.Join(p.Root, "files"))
	}
	return nil
}
func (s *Service) authenticate(r *http.Request, project string) (Principal, error) {
	p := Principal{Project: project}
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if token == r.Header.Get("Authorization") || len(token) < 32 {
		return p, fail("UNAUTHORIZED", "project credential required", 401)
	}
	var scopes string
	e := s.DB.QueryRow("SELECT id,scopes FROM credentials WHERE project=? AND digest=? AND revoked=0", project, digest(token)).Scan(&p.ID, &scopes)
	if e != nil {
		return p, fail("UNAUTHORIZED", "invalid project credential", 401)
	}
	json.Unmarshal([]byte(scopes), &p.Scopes)
	return p, nil
}
func (s *Service) allowed(ctx context.Context, p Principal, scope string) error {
	var enabled, quarantined bool
	var revoked bool
	if e := s.DB.QueryRowContext(ctx, "SELECT enabled,quarantined FROM projects WHERE id=?", p.Project).Scan(&enabled, &quarantined); e != nil {
		return fail("PROJECT_NOT_FOUND", "project unavailable", 404)
	}
	if !enabled || quarantined {
		return fail("PROJECT_DISABLED", "project disabled or quarantined", 403)
	}
	if p.Manager {
		return nil
	}
	if e := s.DB.QueryRowContext(ctx, "SELECT revoked FROM credentials WHERE id=? AND project=?", p.ID, p.Project).Scan(&revoked); e != nil || revoked {
		return fail("CREDENTIAL_REVOKED", "credential revoked", 401)
	}
	for _, v := range p.Scopes {
		if v == scope {
			return nil
		}
	}
	return fail("SCOPE_DENIED", "missing "+scope+" scope", 403)
}
func (s *Service) guardContext(r *http.Request, p Principal, scope string) (*http.Request, context.CancelFunc, error) {
	if e := s.allowed(r.Context(), p, scope); e != nil {
		return r, func() {}, e
	}
	ctx, cancel := context.WithCancel(r.Context())
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if s.allowed(ctx, p, scope) != nil {
					cancel()
					return
				}
				pr, e := s.project(p.Project)
				if e != nil {
					cancel()
					return
				}
				if _, e = s.checkVolume(pr); e != nil {
					cancel()
					return
				}
			}
		}
	}()
	return r.WithContext(context.WithValue(ctx, ctxPrincipal{}, p)), cancel, nil
}
func (s *Service) credential(project string, scopes []string) (any, error) {
	valid := map[string]bool{"read": true, "upload": true, "metadata": true, "delete": true, "restore": true, "process": true}
	if len(scopes) == 0 {
		return nil, fail("INVALID_SCOPE", "scopes required", 400)
	}
	for _, v := range scopes {
		if !valid[v] {
			return nil, fail("INVALID_SCOPE", v, 400)
		}
	}
	token := "yd_" + model.ID() + model.ID()
	id := model.ID()
	data, _ := json.Marshal(scopes)
	_, e := s.DB.Exec("INSERT INTO credentials VALUES(?,?,?,?,0,?)", id, project, digest(token), string(data), time.Now().Unix())
	return map[string]any{"id": id, "token": token, "scopes": scopes}, e
}
func (s *Service) checkVolume(p Project) (platform.Volume, error) {
	v, e := platform.CheckRoot(p.Root)
	if e != nil {
		return v, e
	}
	if v.ID != p.Volume || v.Pool != p.Pool {
		return v, fail("VOLUME_CHANGED", "volume identity changed", 503)
	}
	return v, nil
}
func (s *Service) loop() {
	defer close(s.done)
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-tick.C:
				s.runJob()
			}
		}
	}()
	defer func() { <-workerDone }()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-tick.C:
			s.recoverMediaCleanup()
			s.recoverUploads()
			s.recoverMoves()
		}
	}
}

func preparedScaffold(root string) bool {
	entries, e := os.ReadDir(root)
	if e != nil || len(entries) != 2 {
		return false
	}
	for _, v := range entries {
		if !v.IsDir() || (v.Name() != "files" && v.Name() != "temporary") {
			return false
		}
	}
	files, e := os.ReadDir(filepath.Join(root, "files"))
	if e != nil || len(files) > 0 {
		return false
	}
	return true
}
