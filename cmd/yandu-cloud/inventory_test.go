package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
	"yandu/internal/cloud"
	"yandu/internal/inventory"
	"yandu/internal/model"
)

func TestCloudListLiveOfflineAndRevoked(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("cloud configuration/cache requires Unix permissions; production cloud target is Ubuntu")
	}
	token := "ym_" + model.ID() + model.ID()
	status := 200
	report := inventory.Report{SchemaVersion: 1, DeviceID: "owner", Platform: "darwin/arm64", SampledAt: time.Now().UTC(), Projects: []inventory.Project{{ID: "demo", Name: "中文 空格", Mode: "application_storage", State: "active", RootPath: "/测试 空格", DiskID: "pool", QuotaBytes: inventory.Bytes(1000), UsedBytes: inventory.Bytes(100), ReservedBytes: inventory.Bytes(20), RemainingQuotaBytes: inventory.Bytes(880)}}, Disks: []inventory.Disk{{ID: "pool", MountPoints: []string{"/"}, Filesystem: "APFS", TotalBytes: inventory.Bytes(2 << 30), FreeBytes: inventory.Bytes(1 << 30), AvailableAfterReservationsBytes: inventory.Bytes(900 << 20)}}}
	report.Summarize()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/storage/v1/monitor" || r.Header.Get("Authorization") != "Bearer "+token {
			t.Error("wrong path/authorization")
		}
		w.Header().Set("Content-Type", "application/json")
		if status == 200 {
			json.NewEncoder(w).Encode(report)
			return
		}
		w.WriteHeader(status)
		code := "DEVICE_OFFLINE"
		if status == 401 {
			code = "UNAUTHORIZED"
		}
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code}})
	}))
	defer server.Close()
	config := filepath.Join(t.TempDir(), "monitor.json")
	data, _ := json.Marshal(cloud.MonitorConfig{DeviceID: "owner", GatewayURL: server.URL, MonitorToken: token})
	if e := os.WriteFile(config, data, 0600); e != nil {
		t.Fatal(e)
	}
	run := func(command string, args ...string) (string, error) {
		var out, errors bytes.Buffer
		e := inventoryCommand(command, append([]string{"--config", config}, args...), &out, &errors)
		return out.String(), e
	}
	out, e := run("list", "--json")
	if e != nil {
		t.Fatal(e)
	}
	var result inventory.Result
	if e = json.Unmarshal([]byte(out), &result); e != nil {
		t.Fatal(e)
	}
	if !result.Online || result.Stale || len(result.Projects) != 1 {
		t.Fatal("live snapshot missing")
	}
	if strings.Contains(out, token) {
		t.Fatal("CLI leaked secret")
	}
	info, e := os.Stat(filepath.Join(filepath.Dir(config), "monitor-last.json"))
	if e != nil || info.Mode().Perm() != 0600 {
		t.Fatal("cache not private")
	}
	out, e = run("list")
	if e != nil || !strings.Contains(out, "中文 空格") || !strings.Contains(out, "剩余额度") {
		t.Fatal("human table missing project")
	}
	out, e = run("disks")
	if e != nil || strings.Contains(out, "中文 空格") || !strings.Contains(out, "APFS") {
		t.Fatal("disk view incorrect")
	}
	status = 503
	out, e = run("list", "--json")
	if e == nil {
		t.Fatal("offline exit status success")
	}
	if e = json.Unmarshal([]byte(out), &result); e != nil {
		t.Fatal("offline stdout is not one JSON document")
	}
	if result.Online || !result.Stale || len(result.Projects) != 1 || result.Error == "" {
		t.Fatal("offline cache not clearly marked")
	}
	status = 401
	out, e = run("list", "--json")
	if e == nil {
		t.Fatal("revoked credential succeeded")
	}
	result = inventory.Result{}
	json.Unmarshal([]byte(out), &result)
	if result.Stale || len(result.Projects) != 0 {
		t.Fatal("revocation exposed old authorized cache")
	}
	status = 503
	os.Remove(filepath.Join(filepath.Dir(config), "monitor-last.json"))
	out, e = run("list", "--json")
	if e == nil {
		t.Fatal("offline without cache succeeded")
	}
	result = inventory.Result{}
	json.Unmarshal([]byte(out), &result)
	if result.Stale || len(result.Projects) != 0 {
		t.Fatal("no-cache result incorrect")
	}
}

func TestMonitorClientDoesNotFollowRedirectWithSecret(t *testing.T) {
	called := false
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	defer foreign.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, foreign.URL, 302) }))
	defer server.Close()
	_, e := cloud.FetchInventory(cloud.MonitorClient(time.Second), cloud.MonitorConfig{DeviceID: "owner", GatewayURL: server.URL, MonitorToken: "ym_" + model.ID() + model.ID()})
	if e == nil || called {
		t.Fatal("monitor follows redirect or loses redirect failure")
	}
}
