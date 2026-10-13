package storage

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"yandu/internal/model"
)

func mediaPolicy(t *testing.T, f fixture, limit, revision int64) {
	t.Helper()
	pr, e := f.s.project(f.project)
	if e != nil {
		t.Fatal(e)
	}
	var policy *model.MediaPolicy
	if limit > 0 {
		policy = &model.MediaPolicy{MP4MaxFileBytes: limit}
	}
	if e = f.s.Configure(context.Background(), model.Project{ID: f.project, Mode: "application_storage", RootPath: pr.Root, QuotaBytes: pr.Quota, Publication: "active", Revision: revision, MediaPolicy: policy}); e != nil {
		t.Fatal(e)
	}
}
func TestProjectMediaLimitsAndIntentSnapshot(t *testing.T) {
	f := setup(t, WithMediaCommand([]string{"worker-not-installed"}))
	mediaPolicy(t, f, 10000000, 1)
	request := `{"name":"video.mp4","length":3,"processing":{"presets":["mp4-720-v1"],"outputConstraints":{"mp4-720-v1":{"onOverflow":"keep-start"}}}}`
	w := f.call(t, "POST", "/uploads", bytes.NewBufferString(request), map[string]string{"Idempotency-Key": "upload-key"})
	if w.Code != 201 {
		t.Fatal(w.Body)
	}
	var intent map[string]string
	json.Unmarshal(w.Body.Bytes(), &intent)
	mediaPolicy(t, f, 20000000, 2)
	retry := f.call(t, "POST", "/uploads", bytes.NewBufferString(request), map[string]string{"Idempotency-Key": "upload-key"})
	if retry.Code != 200 {
		t.Fatal(retry.Body)
	}
	var again map[string]string
	json.Unmarshal(retry.Body.Bytes(), &again)
	if again["id"] != intent["id"] || again["versionId"] != intent["versionId"] {
		t.Fatal("retry changed source identity")
	}
	f.upload(t, intent, "abc")
	f.ready(t, intent["id"])
	refs := f.s.uploadJobs(f.project, intent["id"])
	if len(refs) != 1 {
		t.Fatalf("refs %+v", refs)
	}
	j, e := f.s.jobDetail(f.project, refs[0].ID)
	if e != nil || j.AppliedConstraints == nil || j.AppliedConstraints.MaxFileBytes != 10000000 || j.PolicyRevision != 1 || j.AppliedConstraints.OnOverflow != "keep-start" {
		t.Fatalf("snapshot %+v %v", j, e)
	}
	if len(f.s.uploadJobs(f.project, intent["id"])) != 1 {
		t.Fatal("duplicate automatic jobs")
	}
	if w = f.call(t, "GET", "/uploads/"+intent["id"], nil, nil); !bytes.Contains(w.Body.Bytes(), []byte(refs[0].ID)) {
		t.Fatal(w.Body)
	}
}
func TestMediaSubmissionIdempotencyVersionAndCancellation(t *testing.T) {
	f := setup(t, WithMediaCommand([]string{"worker-not-installed"}))
	mediaPolicy(t, f, 2000000, 1)
	source := f.intent(t, "video.mp4", "", "")
	f.upload(t, source, "abc")
	f.ready(t, source["id"])
	body := fmt.Sprintf(`{"objectId":%q,"expectedVersionId":%q,"presets":["mp4-720-v1"],"outputConstraints":{"mp4-720-v1":{"onOverflow":"keep-start"}}}`, source["objectId"], source["versionId"])
	w := f.call(t, "POST", "/jobs", bytes.NewBufferString(body), map[string]string{"Idempotency-Key": "process-key"})
	if w.Code != 200 {
		t.Fatal(w.Body)
	}
	var result struct {
		Jobs []JobReference `json:"jobs"`
	}
	json.Unmarshal(w.Body.Bytes(), &result)
	if len(result.Jobs) != 1 {
		t.Fatal(w.Body)
	}
	id := result.Jobs[0].ID
	mediaPolicy(t, f, 20000000, 2)
	w = f.call(t, "POST", "/jobs", bytes.NewBufferString(body), map[string]string{"Idempotency-Key": "process-key"})
	if w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte(id)) {
		t.Fatal(w.Body)
	}
	altered := bytes.ReplaceAll([]byte(body), []byte("keep-start"), []byte("reject"))
	if w = f.call(t, "POST", "/jobs", bytes.NewReader(altered), map[string]string{"Idempotency-Key": "process-key"}); w.Code != 409 {
		t.Fatal(w.Body)
	}
	if w = f.call(t, "POST", "/jobs", bytes.NewBufferString(body), nil); w.Code != 400 {
		t.Fatal("constraint submission without preconditions accepted")
	}
	if w = f.call(t, "POST", "/jobs/"+id+"/cancel", nil, nil); w.Code != 200 {
		t.Fatal(w.Body)
	}
	if w = f.call(t, "GET", "/jobs/"+id, nil, nil); w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte(`"cancelled"`)) {
		t.Fatal(w.Body)
	}
	f.project = "other"
	if w = f.call(t, "GET", "/jobs/"+id, nil, nil); w.Code == 200 {
		t.Fatal("cross-project job read")
	}
}
func TestMediaConstraintValidationAndProjectIsolation(t *testing.T) {
	f := setup(t, WithMediaCommand([]string{"worker-not-installed"}))
	for i, limit := range []int64{2000000, 10000000, 20000000} {
		project := fmt.Sprintf("project-%d", i)
		root := filepath.Join(filepath.Dir(f.s.Dir), project)
		if e := os.Mkdir(root, 0700); e != nil {
			t.Fatal(e)
		}
		if e := f.s.Configure(context.Background(), model.Project{ID: project, Mode: "application_storage", RootPath: root, QuotaBytes: 1 << 30, Publication: "active", MediaPolicy: &model.MediaPolicy{MP4MaxFileBytes: limit}}); e != nil {
			t.Fatal(e)
		}
		pr, _ := f.s.project(project)
		snap, e := f.s.processingSnapshot(pr, ProcessingInput{Presets: []string{"mp4-720-v1"}})
		if e != nil || snap.Constraints["mp4-720-v1"].MaxFileBytes != limit || snap.Constraints["mp4-720-v1"].OnOverflow != "reject" {
			t.Fatalf("project %s snapshot %+v %v", project, snap, e)
		}
	}
	invalid := []string{
		`{"presets":["mp4-720-v1"],"outputConstraints":{"mp4-720-v1":{"onOverflow":"keep-start"}}}`,
		`{"presets":["hls-720-v1"],"outputConstraints":{"hls-720-v1":{"onOverflow":"reject"}}}`,
		`{"presets":["mp4-720-v1"],"outputConstraints":{"mp4-720-v1":{"maxFileBytes":2,"onOverflow":"reject"}}}`,
		`{"presets":["mp4-720-v1"],"outputConstraints":{"mp4-720-v1":{"onOverflow":"unknown"}}}`,
	}
	for _, processing := range invalid {
		body := `{"name":"bad.mp4","length":3,"processing":` + processing + `}`
		w := f.call(t, "POST", "/uploads", bytes.NewBufferString(body), map[string]string{"Idempotency-Key": model.ID()})
		if w.Code < 400 {
			t.Fatal("invalid constraint accepted", w.Body)
		}
	}
	pr, _ := f.s.project(f.project)
	if e := f.s.Configure(context.Background(), model.Project{ID: f.project, Mode: "application_storage", RootPath: pr.Root, QuotaBytes: 1 << 30, MediaPolicy: &model.MediaPolicy{MP4MaxFileBytes: -1}}); e == nil {
		t.Fatal("negative policy accepted")
	}
	if e := f.s.Configure(context.Background(), model.Project{ID: f.project, Mode: "application_storage", RootPath: pr.Root, QuotaBytes: 1 << 30, MediaPolicy: &model.MediaPolicy{MP4MaxFileBytes: 2 << 30}}); e == nil {
		t.Fatal("policy above quota accepted")
	}
	w := f.call(t, "GET", "/capabilities", nil, nil)
	if !bytes.Contains(w.Body.Bytes(), []byte(`"constraintScope":"project"`)) {
		t.Fatal(w.Body)
	}
}

func TestRunningMediaStopsWithoutPublishing(t *testing.T) {
	for _, action := range []string{"cancel", "revoke", "disable", "delete", "replace"} {
		t.Run(action, func(t *testing.T) {
			f := setup(t, WithMediaCommand([]string{"worker-must-not-start-before-source-copy"}))
			source := f.intent(t, "original.mp4", "", "")
			f.upload(t, source, "abc")
			f.ready(t, source["id"])
			var path string
			f.s.DB.QueryRow("SELECT path FROM versions WHERE id=?", source["versionId"]).Scan(&path)
			f.b.mu.Lock()
			f.b.blockedPath = path
			f.b.readStarted = make(chan struct{}, 1)
			started := f.b.readStarted
			f.b.mu.Unlock()
			body := fmt.Sprintf(`{"objectId":%q,"presets":["mp4-720-v1"]}`, source["objectId"])
			w := f.call(t, "POST", "/jobs", bytes.NewBufferString(body), nil)
			if w.Code != 200 {
				t.Fatal(w.Body)
			}
			var r struct {
				Jobs []JobReference `json:"jobs"`
			}
			json.Unmarshal(w.Body.Bytes(), &r)
			id := r.Jobs[0].ID
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("source read did not start")
			}
			begin := time.Now()
			switch action {
			case "cancel":
				if w = f.call(t, "POST", "/jobs/"+id+"/cancel", nil, nil); w.Code != 200 {
					t.Fatal(w.Body)
				}
			case "revoke":
				f.s.DB.Exec("UPDATE credentials SET revoked=1 WHERE project=?", f.project)
			case "disable":
				f.s.Disable(f.project)
			case "delete":
				f.s.DB.Exec("UPDATE entries SET deleted=1 WHERE id=?", source["objectId"])
			case "replace":
				f.s.DB.Exec("UPDATE entries SET current_version='replaced-version' WHERE id=?", source["objectId"])
			}
			for time.Since(begin) < 5*time.Second {
				j, _ := f.s.jobDetail(f.project, id)
				var reserve int
				f.s.DB.QueryRow("SELECT COUNT(*) FROM reservations WHERE id=?", id).Scan(&reserve)
				if j.State == "cancelled" && reserve == 0 {
					break
				}
				time.Sleep(50 * time.Millisecond)
			}
			j, _ := f.s.jobDetail(f.project, id)
			if j.State != "cancelled" || time.Since(begin) >= 5*time.Second {
				t.Fatalf("active task not stopped: %+v", j)
			}
			var outputs int
			f.s.DB.QueryRow("SELECT COUNT(*) FROM derivatives WHERE job=?", id).Scan(&outputs)
			if outputs != 0 {
				t.Fatal("cancelled task published files")
			}
		})
	}
}

func TestMediaMigrationPreservesLegacyCandidates(t *testing.T) {
	dir, _ := filepath.EvalSymlinks(t.TempDir())
	db, e := sql.Open("sqlite", filepath.Join(dir, "storage.db"))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("PRAGMA foreign_keys=ON"); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"00001_storage.sql", "00002_integrity.sql", "00003_media_candidates.sql"} {
		data, _ := migrations.ReadFile("migrations/" + name)
		up := strings.Split(string(data), "-- +goose Down")[0]
		if _, e = db.Exec(up); e != nil {
			t.Fatal(e)
		}
	}
	_, e = db.Exec(`INSERT INTO projects(id,root,quota,backend_key,volume,pool) VALUES('legacy','/legacy',1000000,'key','volume','pool'); INSERT INTO entries(id,project,parent,name,kind,created,current_version) VALUES('object','legacy','','video','file',1,'version'); INSERT INTO versions(id,project,object,size,sha256,mime,path,created) VALUES('version','legacy','object',3,'source-sha','video/mp4','original/path',1); INSERT INTO jobs(id,project,object,version,credential,preset,state) VALUES('job','legacy','object','version','credential','mp4-720-v1','succeeded'); INSERT INTO media_candidates VALUES('job','legacy','derived/legacy'); INSERT INTO derivatives(id,project,object,version,preset,path,size,mime,sha256) VALUES('derivative','legacy','object','version','mp4-720-v1','derived/path',3,'video/mp4','output-sha');`)
	if e != nil {
		t.Fatal(e)
	}
	data, _ := migrations.ReadFile("migrations/00004_media_constraints.sql")
	up := strings.Split(string(data), "-- +goose Down")[0]
	tx, e := db.Begin()
	if e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec(up); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	var state, path, source, processing, result string
	db.QueryRow("SELECT state,source_sha256,processing_version,result_json FROM jobs WHERE id='job'").Scan(&state, &source, &processing, &result)
	db.QueryRow("SELECT path FROM derivatives WHERE id='derivative'").Scan(&path)
	if state != "succeeded" || source != "source-sha" || processing != "legacy-v0.2" || result != "" || path != "derived/path" {
		t.Fatal("migration changed legacy identity or fabricated measurements")
	}
	var count int
	db.QueryRow("SELECT COUNT(*) FROM media_candidates WHERE job='job'").Scan(&count)
	if count != 1 {
		t.Fatal("lost cleanup records")
	}
	db.Close()
}

type mediaFaultBackend struct {
	*testBackend
	faultMu                sync.Mutex
	failUpload, failRemove bool
}

func (b *mediaFaultBackend) Upload(ctx context.Context, key, path string, r io.Reader, n int64) error {
	if e := b.testBackend.Upload(ctx, key, path, r, n); e != nil {
		return e
	}
	b.faultMu.Lock()
	defer b.faultMu.Unlock()
	if b.failUpload && strings.HasPrefix(path, "derived/") {
		return fmt.Errorf("upload response lost after bytes were written")
	}
	return nil
}
func (b *mediaFaultBackend) Remove(ctx context.Context, key, path string) error {
	b.faultMu.Lock()
	fail := b.failRemove
	b.faultMu.Unlock()
	if fail && strings.HasPrefix(path, "derived/") {
		return fmt.Errorf("backend unavailable during cleanup")
	}
	return b.testBackend.Remove(ctx, key, path)
}
func TestMediaCandidateCleanupAndAtomicCommit(t *testing.T) {
	node, e := exec.LookPath("node")
	if e != nil {
		t.Skip("Node needed for media process contract")
	}
	worker := `const fs=require('node:fs');let d='';process.stdin.on('data',b=>d+=b);process.stdin.on('end',()=>{const j=JSON.parse(d);fs.writeFileSync(j.directory+'/thumbnail.webp',Buffer.from('unit-output'));process.stdout.write(JSON.stringify({files:[{name:'thumbnail.webp',mime:'image/webp'}]}));});`
	for _, fault := range []bool{false, true} {
		t.Run(fmt.Sprint(fault), func(t *testing.T) {
			b := &mediaFaultBackend{failUpload: fault, failRemove: fault}
			f := setup(t, WithMediaCommand([]string{node, "-e", worker}), func(s *Service) { b.testBackend = s.Backend.(*testBackend); s.Backend = b })
			source := f.intent(t, "image.png", "", "")
			f.upload(t, source, "abc")
			f.ready(t, source["id"])
			principal := Principal{ID: credentialID(t, f), Project: f.project, Scopes: []string{"process"}}
			tx, e := f.s.DB.Begin()
			if e != nil {
				t.Fatal(e)
			}
			refs, e := f.s.insertJobs(tx, principal, source["objectId"], source["versionId"], "", "atomic-test", []string{"thumbnail-256-v1"}, ProcessingSnapshot{Constraints: map[string]OutputConstraint{}, ProcessingVersion: f.s.ProcessingVersion})
			if e != nil {
				t.Fatal(e)
			}
			id := refs[0].ID
			if _, e = tx.Exec("UPDATE jobs SET state='running',attempts=1,lease=? WHERE id=?", time.Now().Add(time.Hour).Unix(), id); e != nil {
				t.Fatal(e)
			}
			if e = tx.Commit(); e != nil {
				t.Fatal(e)
			}
			err := f.s.executeJob(id, f.project, source["objectId"], source["versionId"], credentialID(t, f), "thumbnail-256-v1")
			var candidates, reservations, derivatives int
			f.s.DB.QueryRow("SELECT COUNT(*) FROM media_candidates WHERE job=?", id).Scan(&candidates)
			f.s.DB.QueryRow("SELECT COUNT(*) FROM reservations WHERE id=?", id).Scan(&reservations)
			f.s.DB.QueryRow("SELECT COUNT(*) FROM derivatives WHERE job=?", id).Scan(&derivatives)
			if !fault {
				j, _ := f.s.jobDetail(f.project, id)
				if err != nil || j.State != "succeeded" || derivatives != 1 || candidates != 0 || reservations != 0 {
					t.Fatalf("non-atomic commit: %+v %v %d/%d/%d", j, err, derivatives, candidates, reservations)
				}
				return
			}
			if err == nil || derivatives != 0 || candidates != 1 || reservations != 1 {
				t.Fatalf("failed candidate became public: %v %d/%d/%d", err, derivatives, candidates, reservations)
			}
			f.s.DB.Exec("UPDATE jobs SET state='failed',lease=0 WHERE id=?", id)
			f.s.DB.Exec("UPDATE credentials SET revoked=1 WHERE project=?", f.project)
			b.faultMu.Lock()
			b.failRemove = false
			b.faultMu.Unlock()
			f.s.recoverMediaCleanup()
			f.s.DB.QueryRow("SELECT COUNT(*) FROM media_candidates WHERE job=?", id).Scan(&candidates)
			f.s.DB.QueryRow("SELECT COUNT(*) FROM reservations WHERE id=?", id).Scan(&reservations)
			if candidates != 0 || reservations != 0 {
				t.Fatal("revocation blocked durable internal cleanup")
			}
		})
	}
}
func credentialID(t *testing.T, f fixture) string {
	t.Helper()
	var id string
	if e := f.s.DB.QueryRow("SELECT id FROM credentials WHERE project=? AND digest=?", f.project, digest(f.token)).Scan(&id); e != nil {
		t.Fatal(e)
	}
	return id
}
