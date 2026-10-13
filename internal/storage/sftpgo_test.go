package storage

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"yandu/internal/cloud"
	"yandu/internal/inventory"
	"yandu/internal/model"
	"yandu/internal/toolchain"
)

func TestSFTPGoNativeContract(t *testing.T) {
	binary := os.Getenv("YANDU_TEST_SFTPGO")
	if binary == "" {
		t.Skip("native SFTPGo process required; set YANDU_TEST_SFTPGO")
	}
	binary, _ = filepath.Abs(binary)
	dir, _ := filepath.EvalSymlinks(t.TempDir())
	ln, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	templates := os.Getenv("YANDU_TEST_SFTPGO_TEMPLATES")
	if templates == "" {
		templates = filepath.Join(filepath.Dir(binary), "templates")
	}
	cfg := map[string]any{"sftpd": map[string]any{"bindings": []any{map[string]any{"port": 0}}}, "ftpd": map[string]any{"bindings": []any{map[string]any{"port": 0}}}, "webdavd": map[string]any{"bindings": []any{map[string]any{"port": 0}}}, "httpd": map[string]any{"bindings": []any{map[string]any{"port": port, "address": "127.0.0.1", "enable_web_admin": false, "enable_web_client": false, "enable_rest_api": true, "render_openapi": false}}, "templates_path": "", "static_files_path": "", "openapi_path": ""}, "data_provider": map[string]any{"driver": "sqlite", "name": "backend.db", "create_default_admin": true}, "smtp": map[string]any{"templates_path": templates}, "common": map[string]any{"idle_timeout": 5}}
	data, _ := json.Marshal(cfg)
	os.WriteFile(filepath.Join(dir, "sftpgo.json"), data, 0600)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	log, e := os.Create(filepath.Join(dir, "process.log"))
	if e != nil {
		t.Fatal(e)
	}
	defer log.Close()
	cmd := exec.CommandContext(ctx, binary, "serve", "--config-dir", dir, "--log-file-path", "")
	cmd.Env = append(os.Environ(), "SFTPGO_DEFAULT_ADMIN_USERNAME=yandu-provisioner", "SFTPGO_DEFAULT_ADMIN_PASSWORD="+strings.Repeat("native-contract-", 4))
	cmd.Stdout = log
	cmd.Stderr = log
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	defer func() { cancel(); cmd.Wait() }()
	endpoint := fmt.Sprintf("http://127.0.0.1:%d", port)
	var key string
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); {
		key, e = Bootstrap(ctx, endpoint, strings.Repeat("native-contract-", 4))
		if e == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if e != nil {
		data, _ = os.ReadFile(filepath.Join(dir, "process.log"))
		t.Fatalf("bootstrap: %v\n%s", e, data)
	}
	backend := &SFTPGo{URL: endpoint, AdminKey: key, AdminPassword: strings.Repeat("native-contract-", 4)}
	root := filepath.Join(dir, "中文 原件")
	os.Mkdir(root, 0700)
	options := []Option{}
	if os.Getenv("YANDU_TEST_MEDIA") == "1" {
		node, err := exec.LookPath("node")
		if err != nil {
			t.Fatal(err)
		}
		worker, _ := filepath.Abs("../../media/worker.mjs")
		if packagedWorker := os.Getenv("YANDU_TEST_MEDIA_WORKER"); packagedWorker != "" {
			worker, _ = filepath.Abs(packagedWorker)
		}
		options = append(options, WithMediaCommand([]string{node, "--max-old-space-size=512", worker}))
		if configPath := os.Getenv("YANDU_TEST_TOOLS_CONFIG"); configPath != "" {
			registered, err := toolchain.Load(configPath)
			if err != nil {
				t.Fatal(err)
			}
			options = append(options, WithMediaCommand([]string{registered.Tools["node"].Path, "--max-old-space-size=512", registered.MediaWorker}), WithMediaEnvironment(toolchain.Environment(registered)))
		}
	}
	s, e := Open(ctx, filepath.Join(dir, "metadata"), backend, options...)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	e = s.Configure(ctx, model.Project{ID: "native", Mode: "application_storage", RootPath: root, QuotaBytes: 1 << 30, Publication: "active"})
	if e != nil {
		t.Fatal(e)
	}
	v, e := s.credential("native", []string{"upload", "read", "delete", "restore"})
	if e != nil {
		t.Fatal(e)
	}
	f := fixture{s: s, token: v.(map[string]any)["token"].(string), project: "native"}
	intent := f.intent(t, "真实.txt", "", "")
	f.upload(t, intent, "abc")
	f.ready(t, intent["id"])
	t.Run("cloud inventory with native disk and SFTPGo usage", func(t *testing.T) {
		monitorToken := "ym_" + model.ID() + model.ID()
		s.InventorySource = func() (model.Snapshot, error) {
			return model.Snapshot{Profile: &model.Profile{DeviceID: "owner", Storage: &model.StorageTunnel{MonitorToken: monitorToken}}, Projects: []model.Project{{ID: "native", DisplayName: "中文原件", Mode: "application_storage", RootPath: root, Status: "active"}}}, nil
		}
		device := httptest.NewServer(s.PublicHandler())
		defer device.Close()
		h, err := cloud.NewGateway(cloud.GatewayConfig{Listen: "127.0.0.1:19091", Upstream: device.URL})
		if err != nil {
			t.Fatal(err)
		}
		gateway := httptest.NewServer(h)
		defer gateway.Close()
		report, err := cloud.FetchInventory(cloud.MonitorClient(30*time.Second), cloud.MonitorConfig{DeviceID: "owner", GatewayURL: gateway.URL, MonitorToken: monitorToken})
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Projects) != 1 || *report.Projects[0].UsedBytes != 3 || report.Summary.FreeBytes == nil || *report.Summary.FreeBytes <= 0 || report.Summary.TotalBytes == nil || *report.Summary.TotalBytes <= 0 {
			t.Fatalf("native inventory incorrect: %+v", report)
		}
		t.Logf("native cloud inventory: projectUsedBytes=%d poolCount=%d totalBytes=%d freeBytes=%d", *report.Projects[0].UsedBytes, report.Summary.DiskCount, *report.Summary.TotalBytes, *report.Summary.FreeBytes)
		if cli := os.Getenv("YANDU_TEST_CLOUD_CLI"); cli != "" {
			cli, err = filepath.Abs(cli)
			if err != nil {
				t.Fatal(err)
			}
			config := filepath.Join(dir, "monitor.json")
			data, _ := json.Marshal(cloud.MonitorConfig{DeviceID: "owner", GatewayURL: gateway.URL, MonitorToken: monitorToken})
			if err = os.WriteFile(config, data, 0600); err != nil {
				t.Fatal(err)
			}
			data, err = exec.Command(cli, "list", "--config", config, "--json").Output()
			if err != nil {
				t.Fatal("native CLI", err)
			}
			var result inventory.Result
			if err = json.Unmarshal(data, &result); err != nil {
				t.Fatal(err)
			}
			if !result.Online || result.Stale || len(result.Projects) != 1 || *result.Projects[0].UsedBytes != 3 {
				t.Fatal("actual CLI did not read native inventory")
			}
			t.Log("native CLI JSON received and parsed")
		}
	})
	response := f.call(t, "GET", "/objects/"+intent["objectId"]+"/content", nil, map[string]string{"Range": "bytes=1-2"})
	if response.Code != 206 || response.Body.String() != "bc" {
		t.Fatalf("native range %d %s", response.Code, response.Body)
	}
	// An internal user key cannot read another project's physical home.
	p, _ := s.project("native")
	r, e := backend.Download(ctx, p.Key, "../../backend.db", "")
	if e != nil {
		t.Fatal(e)
	}
	defer r.Body.Close()
	if r.StatusCode == 200 {
		t.Fatal("backend traversal accepted")
	}
	if os.Getenv("YANDU_TEST_MEDIA") == "1" {
		credentials, e := s.credential("native", []string{"upload", "read", "process", "delete"})
		if e != nil {
			t.Fatal(e)
		}
		f.token = credentials.(map[string]any)["token"].(string)
		t.Run("project MP4 limits and measured cloud downloads", func(t *testing.T) { nativeMP4Contract(t, f) })
		input, err := os.ReadFile("../../tests/fixtures/orientation-6.heic")
		if err != nil {
			t.Fatal(err)
		}
		request, _ := json.Marshal(map[string]any{"parentId": "", "name": "方向.heic", "length": len(input), "processing": map[string]any{"presets": []string{"webp-1280-v1"}}})
		v, e := s.credential("native", []string{"upload", "read", "process", "delete"})
		if e != nil {
			t.Fatal(e)
		}
		f.token = v.(map[string]any)["token"].(string)
		created := f.call(t, "POST", "/uploads", bytes.NewReader(request), map[string]string{"Idempotency-Key": "native-heic"})
		if created.Code != 201 {
			t.Fatal(created.Body.String())
		}
		var intent map[string]string
		json.Unmarshal(created.Body.Bytes(), &intent)
		headers := map[string]string{"Tus-Resumable": "1.0.0", "Upload-Length": fmt.Sprint(len(input)), "Upload-Metadata": "intentId " + base64.StdEncoding.EncodeToString([]byte(intent["id"]))}
		created = f.call(t, "POST", "/tus/", nil, headers)
		if created.Code != 201 {
			t.Fatal(created.Body.String())
		}
		patch := f.call(t, "PATCH", "/tus/"+intent["id"], bytes.NewReader(input), map[string]string{"Tus-Resumable": "1.0.0", "Upload-Offset": "0", "Content-Type": "application/offset+octet-stream"})
		if patch.Code != 204 {
			t.Fatal(patch.Body.String())
		}
		deadline := time.Now().Add(90 * time.Second)
		var jobState, jobError string
		for time.Now().Before(deadline) {
			s.DB.QueryRow("SELECT state,error FROM jobs WHERE object=?", intent["objectId"]).Scan(&jobState, &jobError)
			if jobState == "succeeded" {
				break
			}
			if jobState == "failed" {
				t.Fatalf("native job failed: %s", jobError)
			}
			time.Sleep(100 * time.Millisecond)
		}
		if jobState != "succeeded" {
			t.Fatalf("native media timeout: %s %s", jobState, jobError)
		}
		var derivative string
		s.DB.QueryRow("SELECT id FROM derivatives WHERE object=?", intent["objectId"]).Scan(&derivative)
		derived := f.call(t, "GET", "/derivatives/"+derivative, nil, nil)
		if derived.Code != http.StatusOK || len(derived.Body.Bytes()) < 20 {
			t.Fatalf("native derivative %d", derived.Code)
		}
		if f.call(t, "DELETE", "/entries/"+intent["objectId"], nil, nil).Code != 200 {
			t.Fatal("native delete")
		}
		if f.call(t, "GET", "/derivatives/"+derivative, nil, nil).Code != 404 {
			t.Fatal("deleted derivative still readable")
		}
		if n, _ := io.Copy(io.Discard, r.Body); n != 0 {
			t.Log("denied backend response bytes", n)
		}
	}
}
