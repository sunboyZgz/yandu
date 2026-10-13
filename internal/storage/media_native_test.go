package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
	"yandu/internal/cloud"
	"yandu/internal/model"
)

func nativeMediaUpload(t *testing.T, f fixture, input []byte, name, key, policy string) (map[string]string, JobDetail) {
	t.Helper()
	processing := map[string]any{"presets": []string{"mp4-720-v1"}}
	if policy != "" {
		processing["outputConstraints"] = map[string]any{"mp4-720-v1": map[string]string{"onOverflow": policy}}
	}
	body, _ := json.Marshal(map[string]any{"name": name, "length": len(input), "processing": processing})
	w := f.call(t, "POST", "/uploads", bytes.NewReader(body), map[string]string{"Idempotency-Key": key})
	if w.Code != 201 {
		t.Fatal(w.Body)
	}
	var intent map[string]string
	json.Unmarshal(w.Body.Bytes(), &intent)
	retry := f.call(t, "POST", "/uploads", bytes.NewReader(body), map[string]string{"Idempotency-Key": key})
	if retry.Code != 200 || !bytes.Contains(retry.Body.Bytes(), []byte(intent["id"])) {
		t.Fatal("lost response retry", retry.Body)
	}
	headers := map[string]string{"Tus-Resumable": "1.0.0", "Upload-Length": fmt.Sprint(len(input)), "Upload-Metadata": "intentId " + base64.StdEncoding.EncodeToString([]byte(intent["id"]))}
	if w = f.call(t, "POST", "/tus/", nil, headers); w.Code != 201 {
		t.Fatal(w.Body)
	}
	offset := 0
	for _, end := range []int{len(input) / 2, len(input)} {
		w = f.call(t, "PATCH", "/tus/"+intent["id"], bytes.NewReader(input[offset:end]), map[string]string{"Tus-Resumable": "1.0.0", "Upload-Offset": fmt.Sprint(offset), "Content-Type": "application/offset+octet-stream"})
		if w.Code != 204 {
			t.Fatal(w.Body)
		}
		offset = end
		w = f.call(t, "HEAD", "/tus/"+intent["id"], nil, map[string]string{"Tus-Resumable": "1.0.0"})
		if w.Code != 200 || w.Header().Get("Upload-Offset") != fmt.Sprint(offset) {
			t.Fatal("resume offset", w.Code, w.Header())
		}
	}
	f.ready(t, intent["id"])
	var j JobDetail
	for deadline := time.Now().Add(90 * time.Second); time.Now().Before(deadline); {
		refs := f.s.uploadJobs(f.project, intent["id"])
		if len(refs) == 1 {
			j, _ = f.s.jobDetail(f.project, refs[0].ID)
			if j.State == "succeeded" || j.State == "failed" || j.State == "cancelled" {
				return intent, j
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("native media task timed out", j)
	return intent, j
}
func nativeMP4Contract(t *testing.T, f fixture) {
	t.Helper()
	ffmpeg := os.Getenv("YANDU_FFMPEG")
	if ffmpeg == "" {
		var e error
		ffmpeg, e = exec.LookPath("ffmpeg")
		if e != nil {
			t.Fatal(e)
		}
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "有声 原件.mp4")
	if output, e := exec.Command(ffmpeg, "-nostdin", "-y", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size=640x360:rate=30:duration=6", "-f", "lavfi", "-i", "sine=frequency=1000:sample_rate=48000:duration=6", "-c:v", "libx264", "-crf", "12", "-g", "90", "-pix_fmt", "yuv420p", "-c:a", "aac", "-movflags", "+faststart", source).CombinedOutput(); e != nil {
		t.Fatal(e, string(output))
	}
	input, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	hash := sha256.Sum256(input)
	sourceSHA := hex.EncodeToString(hash[:])
	mediaPolicy(t, f, 0, 1)
	original, full := nativeMediaUpload(t, f, input, "full.mp4", "native-full", "")
	if full.State != "succeeded" || full.Result == nil || full.SourceSHA256 != sourceSHA {
		t.Fatalf("full %+v", full)
	}
	// Traverse the generated public handler and the real Go cloud streaming gateway.
	upstream := httptest.NewServer(f.s.PublicHandler())
	defer upstream.Close()
	gateway, e := cloud.NewGateway(cloud.GatewayConfig{Listen: "127.0.0.1:19091", Upstream: upstream.URL})
	if e != nil {
		t.Fatal(e)
	}
	type nativeRecord struct {
		ProjectID string    `json:"projectId"`
		Job       JobDetail `json:"job"`
	}
	records := []nativeRecord{}
	verify := func(j JobDetail) {
		records = append(records, nativeRecord{f.project, j})
		req := httptest.NewRequest("GET", "/storage/v1/projects/"+f.project+"/derivatives/"+j.Result.DerivativeID, nil)
		req.RemoteAddr = "127.0.0.1:12345"
		req.Header.Set("Authorization", "Bearer "+f.token)
		w := httptest.NewRecorder()
		gateway.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatal(w.Code, w.Body)
		}
		hash := sha256.Sum256(w.Body.Bytes())
		if int64(w.Body.Len()) != j.Result.OutputBytes || hex.EncodeToString(hash[:]) != j.Result.OutputSHA256 {
			t.Fatal("download/result mismatch")
		}
		file := filepath.Join(dir, j.ID+".mp4")
		os.WriteFile(file, w.Body.Bytes(), 0600)
		if data, e := exec.Command(ffmpeg, "-nostdin", "-v", "error", "-xerror", "-i", file, "-f", "null", "-").CombinedOutput(); e != nil {
			t.Fatal("download cannot decode", e, string(data))
		}
		req = httptest.NewRequest("GET", "/storage/v1/projects/"+f.project+"/jobs/"+j.ID, nil)
		req.RemoteAddr = "127.0.0.1:12345"
		req.Header.Set("Authorization", "Bearer "+f.token)
		w = httptest.NewRecorder()
		gateway.ServeHTTP(w, req)
		if w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte(j.Result.OutputSHA256)) {
			t.Fatal("generated GET job", w.Body)
		}
	}
	verify(full)
	mediaPolicy(t, f, full.Result.OutputBytes, 2)
	_, equal := nativeMediaUpload(t, f, input, "equal.mp4", "native-equal", "reject")
	if equal.State != "succeeded" || equal.Result.OutputBytes != full.Result.OutputBytes || equal.Result.Trimmed {
		t.Fatalf("equal %+v", equal)
	}
	verify(equal)
	mediaPolicy(t, f, full.Result.OutputBytes-1, 3)
	rejected, reject := nativeMediaUpload(t, f, input, "reject.mp4", "native-reject", "")
	if reject.State != "failed" || reject.Error == nil || reject.Error.Code != "OUTPUT_LIMIT_EXCEEDED" || reject.Attempts != 1 {
		t.Fatalf("reject %+v", reject)
	}
	w := f.call(t, "GET", "/objects/"+rejected["objectId"]+"/content", nil, nil)
	if !bytes.Equal(w.Body.Bytes(), input) {
		t.Fatal("reject changed original")
	}
	records = append(records, nativeRecord{f.project, reject})
	mediaPolicy(t, f, full.Result.OutputBytes*3/5, 4)
	_, trim := nativeMediaUpload(t, f, input, "trim.mp4", "native-trim", "keep-start")
	if trim.State != "succeeded" || trim.Result == nil || !trim.Result.Trimmed || trim.Result.OutputBytes > full.Result.OutputBytes*3/5 || trim.SourceSHA256 != sourceSHA {
		t.Fatalf("trim %+v", trim)
	}
	verify(trim)
	// A new logical task for the same source gets a separate immutable physical path.
	body := fmt.Sprintf(`{"objectId":%q,"expectedVersionId":%q,"presets":["mp4-720-v1"],"outputConstraints":{"mp4-720-v1":{"onOverflow":"keep-start"}}}`, original["objectId"], original["versionId"])
	w = f.call(t, "POST", "/jobs", bytes.NewBufferString(body), map[string]string{"Idempotency-Key": "native-second-logical-task"})
	if w.Code != 200 {
		t.Fatal(w.Body)
	}
	var refs struct {
		Jobs []JobReference `json:"jobs"`
	}
	json.Unmarshal(w.Body.Bytes(), &refs)
	for deadline := time.Now().Add(90 * time.Second); time.Now().Before(deadline); {
		j, _ := f.s.jobDetail(f.project, refs.Jobs[0].ID)
		if j.State == "succeeded" {
			verify(j)
			break
		}
		if j.State == "failed" {
			t.Fatal(j.Error)
		}
		time.Sleep(100 * time.Millisecond)
	}
	var paths int
	f.s.DB.QueryRow("SELECT COUNT(DISTINCT path) FROM derivatives WHERE object=? AND mime='video/mp4'", original["objectId"]).Scan(&paths)
	if paths != 2 {
		t.Fatal("task paths collided", paths)
	}
	for i, limit := range []int64{2000000, 10000000, 20000000} {
		project := fmt.Sprintf("media-business-%d", i)
		home := filepath.Join(dir, project)
		os.Mkdir(home, 0700)
		home, _ = filepath.EvalSymlinks(home)
		if e = f.s.Configure(context.Background(), model.Project{ID: project, Mode: "application_storage", RootPath: home, QuotaBytes: 1 << 30, Publication: "active", Revision: 1, MediaPolicy: &model.MediaPolicy{MP4MaxFileBytes: limit}}); e != nil {
			t.Fatal(e)
		}
		credential, e := f.s.credential(project, []string{"upload", "read", "process"})
		if e != nil {
			t.Fatal(e)
		}
		other := fixture{s: f.s, project: project, token: credential.(map[string]any)["token"].(string)}
		_, job := nativeMediaUpload(t, other, input, "same video.mp4", "native-project-upload", "reject")
		records = append(records, nativeRecord{project, job})
		if job.State != "succeeded" || job.AppliedConstraints.MaxFileBytes != limit || job.Result.OutputBytes > limit || job.SourceSHA256 != sourceSHA {
			t.Fatalf("native project isolation %+v", job)
		}
		if response := other.call(t, "GET", "/jobs/"+full.ID, nil, nil); response.Code != 404 {
			t.Fatal("cross-project task disclosure", response.Code)
		}
		if response := other.call(t, "GET", "/derivatives/"+full.Result.DerivativeID, nil, nil); response.Code != 404 {
			t.Fatal("cross-project derivative disclosure", response.Code)
		}
	}

	var pending int
	f.s.DB.QueryRow("SELECT COUNT(*) FROM media_candidates WHERE project=?", f.project).Scan(&pending)
	if pending != 0 {
		t.Fatal("committed candidates not cleaned")
	}
	reportDir, _ := filepath.Abs("../../.local/mp4-acceptance")
	os.MkdirAll(reportDir, 0700)
	contract, _ := os.ReadFile("../../schemas/storage.openapi.json")
	contractHash := sha256.Sum256(contract)
	report, _ := json.MarshalIndent(map[string]any{"serviceVersion": model.Version, "platform": runtime.GOOS, "arch": runtime.GOARCH, "go": runtime.Version(), "contractSha256": hex.EncodeToString(contractHash[:]), "generatedAt": time.Now().UTC().Format(time.RFC3339), "jobs": records}, "", "  ")
	if e = os.WriteFile(filepath.Join(reportDir, "native-report.json"), append(report, '\n'), 0600); e != nil {
		t.Fatal(e)
	}

}
