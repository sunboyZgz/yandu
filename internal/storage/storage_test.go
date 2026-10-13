package storage

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"yandu/internal/model"
)

type testBackend struct {
	mu          sync.Mutex
	files       map[string][]byte
	loseMove    bool
	blockedPath string
	readStarted chan struct{}
}

func (b *testBackend) Provision(_ context.Context, id, root string, q int64) (string, error) {
	return id, nil
}
func (b *testBackend) Mkdir(context.Context, string, string) error { return nil }
func (b *testBackend) Upload(ctx context.Context, key, path string, r io.Reader, n int64) error {
	data, e := io.ReadAll(r)
	if e != nil {
		return e
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.files[key+":"+path] = data
	return nil
}
func (b *testBackend) Download(ctx context.Context, key, path, rangeHeader string) (*http.Response, error) {
	b.mu.Lock()
	data, ok := b.files[key+":"+path]
	blocked, started := b.blockedPath == path, b.readStarted
	b.mu.Unlock()
	if blocked && started != nil {
		select {
		case started <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if !ok {
		return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
	}
	status := 200
	header := http.Header{}
	if rangeHeader == "bytes=1-2" && len(data) >= 3 {
		header.Set("Content-Range", fmt.Sprintf("bytes 1-2/%d", len(data)))
		data = data[1:3]
		status = 206
	}
	header.Set("Content-Length", fmt.Sprint(len(data)))
	return &http.Response{StatusCode: status, Body: io.NopCloser(bytes.NewReader(data)), Header: header}, nil
}
func (b *testBackend) Move(ctx context.Context, key, from, to string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	data, ok := b.files[key+":"+from]
	if !ok {
		return fmt.Errorf("missing source")
	}
	b.files[key+":"+to] = data
	delete(b.files, key+":"+from)
	if b.loseMove {
		b.loseMove = false
		return fmt.Errorf("response lost")
	}
	return nil
}
func (b *testBackend) Remove(ctx context.Context, key, path string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.files, key+":"+path)
	return nil
}

type fixture struct {
	s       *Service
	b       *testBackend
	token   string
	project string
}

func setup(t *testing.T, options ...Option) fixture {
	t.Helper()
	dir, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	b := &testBackend{files: map[string][]byte{}}
	s, e := Open(context.Background(), filepath.Join(dir, "state"), b, options...)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(s.Close)
	root := filepath.Join(dir, "中文 存储")
	os.Mkdir(root, 0700)
	if e = s.Configure(context.Background(), model.Project{ID: "one", Mode: "application_storage", RootPath: root, QuotaBytes: 1 << 30, Publication: "active"}); e != nil {
		t.Fatal(e)
	}
	v, e := s.credential("one", []string{"read", "upload", "metadata", "delete", "restore", "process"})
	if e != nil {
		t.Fatal(e)
	}
	return fixture{s, b, v.(map[string]any)["token"].(string), "one"}
}
func (f fixture) call(t *testing.T, method, path string, body io.Reader, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, "/storage/v1/projects/"+f.project+path, body)
	r.Header.Set("Authorization", "Bearer "+f.token)
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	f.s.ServeHTTP(w, r)
	return w
}
func (f fixture) intent(t *testing.T, name, object, expected string) map[string]string {
	t.Helper()
	data, _ := json.Marshal(IntentRequest{Name: name, Length: 3, Object: object, Expected: expected})
	w := f.call(t, "POST", "/uploads", bytes.NewReader(data), map[string]string{"Idempotency-Key": model.ID()})
	if w.Code != 201 {
		t.Fatalf("intent %d %s", w.Code, w.Body)
	}
	var v map[string]string
	json.Unmarshal(w.Body.Bytes(), &v)
	return v
}
func (f fixture) upload(t *testing.T, v map[string]string, data string) {
	t.Helper()
	headers := map[string]string{"Tus-Resumable": "1.0.0", "Upload-Length": "3", "Upload-Metadata": "intentId " + base64.StdEncoding.EncodeToString([]byte(v["id"]))}
	w := f.call(t, "POST", "/tus/", nil, headers)
	if w.Code != 201 {
		t.Fatalf("tus create %d %s", w.Code, w.Body)
	}
	headers = map[string]string{"Tus-Resumable": "1.0.0", "Upload-Offset": "0", "Content-Type": "application/offset+octet-stream"}
	w = f.call(t, "PATCH", "/tus/"+v["id"], strings.NewReader(data), headers)
	if w.Code != 204 {
		t.Fatalf("tus patch %d %s", w.Code, w.Body)
	}
}
func (f fixture) ready(t *testing.T, id string) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		v, e := f.s.intent(id, f.project)
		if e == nil && v.State == "READY" {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	v, _ := f.s.intent(id, f.project)
	t.Fatalf("upload did not become ready: %+v", v)
}
func TestTusVersionRangeTrashAndLostResponse(t *testing.T) {
	f := setup(t)
	v := f.intent(t, "照片.txt", "", "")
	f.b.loseMove = true
	f.upload(t, v, "abc")
	f.ready(t, v["id"])
	w := f.call(t, "GET", "/objects/"+v["objectId"]+"/content", nil, map[string]string{"Range": "bytes=1-2"})
	if w.Code != 206 || w.Body.String() != "bc" {
		t.Fatalf("range: %d %s", w.Code, w.Body)
	}
	etag := w.Header().Get("ETag")
	for _, condition := range []struct {
		header, value string
		status        int
	}{
		{"If-Match", `"missing", ` + etag, 200},
		{"If-Match", "W/" + etag, 412},
		{"If-None-Match", `"missing", W/` + etag, 304},
		{"If-None-Match", " * ", 304},
	} {
		response := f.call(t, "GET", "/objects/"+v["objectId"]+"/content", nil, map[string]string{condition.header: condition.value})
		if response.Code != condition.status {
			t.Fatalf("%s %s: %d", condition.header, condition.value, response.Code)
		}
	}
	replacement := f.intent(t, "照片.txt", v["objectId"], v["versionId"])
	loser := f.intent(t, "照片.txt", v["objectId"], v["versionId"])
	f.upload(t, replacement, "def")
	f.ready(t, replacement["id"])
	f.upload(t, loser, "ghi")
	time.Sleep(1500 * time.Millisecond)
	intent, _ := f.s.intent(loser["id"], "one")
	if intent.State != "failed" {
		t.Fatalf("CAS loser %+v", intent)
	}
	w = f.call(t, "GET", "/objects/"+v["objectId"]+"/content", nil, nil)
	if w.Body.String() != "def" {
		t.Fatal(w.Body)
	}
	w = f.call(t, "DELETE", "/entries/"+v["objectId"], nil, nil)
	if w.Code != 200 {
		t.Fatal(w.Body)
	}
	w = f.call(t, "GET", "/objects/"+v["objectId"]+"/content", nil, nil)
	if w.Code != 404 {
		t.Fatal("deleted file readable")
	}
	f.s.recoverMoves()
	w = f.call(t, "POST", "/trash/"+v["objectId"]+"/restore", nil, nil)
	if w.Code != 200 {
		t.Fatal(w.Body)
	}
	f.s.recoverMoves()
	w = f.call(t, "GET", "/objects/"+v["objectId"]+"/content", nil, nil)
	if w.Body.String() != "def" {
		t.Fatal(w.Body)
	}
}

func TestWaitingMediaJobDoesNotBlockUploadCommit(t *testing.T) {
	f := setup(t, WithMediaCommand([]string{"worker-must-not-start-before-input-is-read"}))
	source := f.intent(t, "media.png", "", "")
	f.upload(t, source, "abc")
	f.ready(t, source["id"])
	var sourcePath string
	if err := f.s.DB.QueryRow("SELECT path FROM versions WHERE id=?", source["versionId"]).Scan(&sourcePath); err != nil {
		t.Fatal(err)
	}
	f.b.mu.Lock()
	f.b.blockedPath, f.b.readStarted = sourcePath, make(chan struct{}, 1)
	started := f.b.readStarted
	f.b.mu.Unlock()
	request, _ := json.Marshal(map[string]any{"objectId": source["objectId"], "presets": []string{"thumbnail-256-v1"}})
	if response := f.call(t, "POST", "/jobs", bytes.NewReader(request), nil); response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("media job did not begin reading")
	}
	next := f.intent(t, "another.txt", "", "")
	f.upload(t, next, "def")
	f.ready(t, next["id"])
}
func TestIsolationRevocationQuotaAndFolderCycle(t *testing.T) {
	f := setup(t)
	w := f.call(t, "POST", "/folders", strings.NewReader(`{"name":"a","parentId":""}`), nil)
	var a map[string]string
	json.Unmarshal(w.Body.Bytes(), &a)
	w = f.call(t, "PATCH", "/entries/"+a["id"], strings.NewReader(fmt.Sprintf(`{"name":"a","parentId":%q,"expectedGeneration":1}`, a["id"])), nil)
	if w.Code != 409 {
		t.Fatal(w.Body)
	}
	w = f.call(t, "POST", "/folders", strings.NewReader(`{"name":"../escape","parentId":""}`), nil)
	if w.Code != 400 {
		t.Fatal(w.Body)
	}
	v := f.intent(t, "pending.txt", "", "")
	f.upload(t, v, "abc")
	f.s.DB.Exec("UPDATE credentials SET revoked=1")
	time.Sleep(1500 * time.Millisecond)
	intent, _ := f.s.intent(v["id"], "one")
	if intent.State == "READY" {
		t.Fatal("revoked upload committed")
	}
	w = f.call(t, "GET", "/capacity", nil, nil)
	if w.Code != 401 {
		t.Fatal(w.Body)
	}
	other := f
	other.project = "two"
	w = other.call(t, "GET", "/capacity", nil, nil)
	if w.Code != 401 {
		t.Fatal("cross project auth accepted")
	}
}
func TestIdempotencyAndResume(t *testing.T) {
	f := setup(t)
	body := `{"name":"resume.bin","length":3}`
	h := map[string]string{"Idempotency-Key": "stable"}
	one := f.call(t, "POST", "/uploads", strings.NewReader(body), h)
	two := f.call(t, "POST", "/uploads", strings.NewReader(body), h)
	var v, v2 map[string]string
	json.Unmarshal(one.Body.Bytes(), &v)
	json.Unmarshal(two.Body.Bytes(), &v2)
	if one.Code != 201 || two.Code != 200 || v["id"] != v2["id"] {
		t.Fatalf("idempotency %s %s", one.Body, two.Body)
	}
	w := f.call(t, "POST", "/uploads", strings.NewReader(`{"name":"different","length":3}`), h)
	if w.Code != 409 {
		t.Fatal(w.Body)
	}
	f.upload(t, v, "a")
	w = f.call(t, "HEAD", "/tus/"+v["id"], nil, map[string]string{"Tus-Resumable": "1.0.0"})
	if w.Code != 200 || w.Header().Get("Upload-Offset") != "1" {
		t.Fatalf("resume head %d %s", w.Code, w.Header())
	}
	w = f.call(t, "PATCH", "/tus/"+v["id"], strings.NewReader("bc"), map[string]string{"Tus-Resumable": "1.0.0", "Upload-Offset": "1", "Content-Type": "application/offset+octet-stream"})
	if w.Code != 204 {
		t.Fatal(w.Body)
	}
	f.ready(t, v["id"])
}

func TestActiveRequestRevocationAndQuota(t *testing.T) {
	f := setup(t)
	p, e := f.s.authenticate(httptest.NewRequest("GET", "/", nil), "one")
	_ = p
	_ = e
	request := httptest.NewRequest("GET", "/", nil)
	request.Header.Set("Authorization", "Bearer "+f.token)
	p, e = f.s.authenticate(request, "one")
	if e != nil {
		t.Fatal(e)
	}
	guarded, cancel, e := f.s.guardContext(request, p, "upload")
	if e != nil {
		t.Fatal(e)
	}
	defer cancel()
	start := time.Now()
	f.s.DB.Exec("UPDATE credentials SET revoked=1 WHERE id=?", p.ID)
	select {
	case <-guarded.Context().Done():
		if time.Since(start) > 5*time.Second {
			t.Fatal("revocation took too long")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("active request was not cancelled")
	}
	fresh, e := f.s.credential("one", []string{"upload"})
	if e != nil {
		t.Fatal(e)
	}
	f.token = fresh.(map[string]any)["token"].(string)
	f.s.DB.Exec("UPDATE projects SET quota=1 WHERE id='one'")
	w := f.call(t, "POST", "/uploads", strings.NewReader(`{"name":"too-large","length":3}`), map[string]string{"Idempotency-Key": "quota"})
	if w.Code != 507 {
		t.Fatalf("quota %d %s", w.Code, w.Body)
	}
}
func TestRecursiveDeleteRestoreDoesNotRestoreOlderTrash(t *testing.T) {
	f := setup(t)
	folder, e := f.s.folder("one", "", "parent")
	if e != nil {
		t.Fatal(e)
	}
	id := folder.(Entry).ID
	child, e := f.s.folder("one", id, "child")
	if e != nil {
		t.Fatal(e)
	}
	childID := child.(Entry).ID
	if e = f.s.deleteEntry("one", childID, false); e != nil {
		t.Fatal(e)
	}
	if e = f.s.deleteEntry("one", id, false); e != nil {
		t.Fatal(e)
	}
	if e = f.s.restoreEntry("one", id); e != nil {
		t.Fatal(e)
	}
	if _, e = f.s.entry("one", childID, false); e == nil {
		t.Fatal("older trash resurrected")
	}
}
func TestUploadRecoveryWithoutFinishNotification(t *testing.T) {
	f := setup(t)
	v := f.intent(t, "crash.txt", "", "")
	f.upload(t, v, "abc")
	f.s.DB.Exec("UPDATE uploads SET state='uploading' WHERE id=?", v["id"])
	f.s.recoverUploads()
	f.ready(t, v["id"])
}

func TestCapabilitiesAndCapacityAreSingleJSONResponses(t *testing.T) {
	f := setup(t)
	for _, path := range []string{"/capabilities", "/capacity"} {
		response := f.call(t, "GET", path, nil, nil)
		var value map[string]any
		if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &value) != nil {
			t.Fatalf("invalid single JSON response for %s: %s", path, response.Body)
		}
	}
}
