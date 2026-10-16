package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"yandu/internal/cloud"
	"yandu/internal/inventory"
	"yandu/internal/model"
)

func filterFixture() inventory.Report {
	report := inventory.Report{
		SchemaVersion: 1, DeviceID: "owner", Platform: "darwin/arm64", SampledAt: time.Now().UTC(),
		Projects: []inventory.Project{
			{ID: "blog", Name: "个人博客", DiskID: "shared", UsedBytes: inventory.Bytes(10), ReservedBytes: inventory.Bytes(2)},
			{ID: "browser-demo", Name: "浏览器 验收项目", DiskID: "shared", UsedBytes: inventory.Bytes(20), ReservedBytes: inventory.Bytes(3)},
			{ID: "archive", Name: "旧资料", DiskID: "other", UsedBytes: inventory.Bytes(30), ReservedBytes: inventory.Bytes(7)},
			{ID: "orphan", Name: "未知磁盘"},
		},
		Disks: []inventory.Disk{
			{ID: "shared", Filesystem: "APFS", MountPoints: []string{"/shared"}, ProjectIDs: []string{"blog", "browser-demo"}, TotalBytes: inventory.Bytes(1000), FreeBytes: inventory.Bytes(500), ReservedBytes: 5, AvailableAfterReservationsBytes: inventory.Bytes(400)},
			{ID: "other", Filesystem: "NTFS", MountPoints: []string{"D:/"}, ProjectIDs: []string{"archive"}, TotalBytes: inventory.Bytes(2000), FreeBytes: inventory.Bytes(1000), ReservedBytes: 7, AvailableAfterReservationsBytes: inventory.Bytes(900)},
		},
	}
	report.Summarize()
	return report
}

func TestFilterInventoryMatchesIDsNamesAndSharedDisks(t *testing.T) {
	report := filterFixture()
	before, _ := json.Marshal(report)
	for _, tc := range []struct {
		name      string
		selectors []string
		ids       []string
		diskCount int
	}{
		{"id", []string{"blog"}, []string{"blog"}, 1},
		{"name with spaces", []string{"浏览器 验收项目"}, []string{"browser-demo"}, 1},
		{"union deduplicates in snapshot order", []string{"浏览器 验收项目", "blog", "browser-demo"}, []string{"blog", "browser-demo"}, 1},
		{"different disks", []string{"archive", "blog"}, []string{"blog", "archive"}, 2},
		{"unknown disk", []string{"orphan"}, []string{"orphan"}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := filterInventory(report, tc.selectors)
			if err != nil {
				t.Fatal(err)
			}
			var ids []string
			for _, project := range result.Projects {
				ids = append(ids, project.ID)
			}
			if !slices.Equal(ids, tc.ids) || len(result.Disks) != tc.diskCount || result.Summary.ProjectCount != len(tc.ids) || result.Summary.DiskCount != tc.diskCount {
				t.Fatalf("incorrect scope: %+v", result)
			}
			for _, disk := range result.Disks {
				for _, id := range disk.ProjectIDs {
					if !slices.Contains(tc.ids, id) {
						t.Fatalf("unselected project %s remains in disk details", id)
					}
				}
			}
			if tc.name == "id" && (result.Summary.StorageUsedBytes != 10 || result.Summary.ReservedBytes != 5 || *result.Summary.TotalBytes != 1000 || *result.Disks[0].AvailableAfterReservationsBytes != 400) {
				t.Fatal("shared pool capacity or reservations were incorrectly narrowed to one project")
			}
			if tc.name == "unknown disk" && (result.Summary.TotalBytes != nil || result.Summary.UnassignedProjectCount != 1) {
				t.Fatal("unknown disk became a known capacity")
			}
		})
	}
	after, _ := json.Marshal(report)
	if !bytes.Equal(before, after) {
		t.Fatal("filter mutated the full snapshot")
	}
}

func TestFilterInventoryRejectsMissingAndAmbiguousNamesAndPrefersIDs(t *testing.T) {
	report := filterFixture()
	report.Projects = append(report.Projects,
		inventory.Project{ID: "same-name", Name: "个人博客"},
		inventory.Project{ID: "name-is-an-id", Name: "blog"})
	for _, selector := range []string{"个人博客", "missing", "Blog", "browser"} {
		result, err := filterInventory(report, []string{"archive", selector})
		if err == nil || len(result.Projects) != 0 || len(result.Disks) != 0 || result.Summary.ProjectCount != 0 {
			t.Fatalf("invalid selection returned data: %q %+v %v", selector, result, err)
		}
		if selector == "个人博客" && (!strings.Contains(err.Error(), "blog, same-name") || !strings.Contains(err.Error(), "改用 ID")) {
			t.Fatalf("ambiguity lacks candidate IDs: %v", err)
		}
	}
	result, err := filterInventory(report, []string{"blog"})
	if err != nil || len(result.Projects) != 1 || result.Projects[0].ID != "blog" {
		t.Fatalf("ID did not take priority: %+v %v", result, err)
	}
}

func TestCloudListProjectFiltersFlagsCacheAndErrors(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("cloud monitor config requires Unix permissions")
	}
	report := filterFixture()
	var status, requests atomic.Int32
	status.Store(http.StatusOK)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if code := status.Load(); code != http.StatusOK {
			w.WriteHeader(int(code))
			json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "DEVICE_OFFLINE"}})
			return
		}
		json.NewEncoder(w).Encode(report)
	}))
	defer server.Close()
	config := filepath.Join(t.TempDir(), "monitor with spaces.json")
	c := cloud.MonitorConfig{DeviceID: report.DeviceID, GatewayURL: server.URL, MonitorToken: "ym_" + model.ID() + model.ID()}
	data, _ := json.Marshal(c)
	if err := os.WriteFile(config, data, 0600); err != nil {
		t.Fatal(err)
	}
	run := func(command string, args ...string) (string, error) {
		var out, errOut bytes.Buffer
		// Put --config after selectors to verify flags work in either position.
		err := inventoryCommand(command, append(args, "--config", config), &out, &errOut)
		return out.String(), err
	}
	decode := func(out string) inventory.Result {
		t.Helper()
		var result inventory.Result
		if err := json.Unmarshal([]byte(out), &result); err != nil {
			t.Fatalf("stdout is not one JSON document: %v: %s", err, out)
		}
		return result
	}
	for _, args := range [][]string{
		{"blog", "--json"}, {"--json", "个人博客"},
		{"blog,浏览器 验收项目", "--json", "--timeout", "5s"},
		{" browser-demo, blog ", "--json", "blog"},
	} {
		out, err := run("list", args...)
		if err != nil {
			t.Fatal(args, err)
		}
		result := decode(out)
		want := 1
		if strings.Contains(strings.Join(args, " "), ",") {
			want = 2
		}
		if !result.Online || len(result.Projects) != want || len(result.Disks) != 1 || result.Summary.ProjectCount != want || strings.Contains(out, "archive") {
			t.Fatalf("wrong filtered response: %+v", result)
		}
	}
	out, err := run("list", "blog", "--json=false")
	if err != nil || !strings.Contains(out, "个人博客") || strings.Contains(out, "browser-demo") || strings.Contains(out, "旧资料") {
		t.Fatalf("text filter: %v\n%s", err, out)
	}
	cached, err := cloud.CachedInventory(config, c)
	if err != nil || len(cached.Projects) != 4 || len(cached.Disks) != 2 {
		t.Fatal("filtered query narrowed the persisted offline snapshot", err)
	}
	status.Store(http.StatusServiceUnavailable)
	out, err = run("list", "浏览器 验收项目", "--json")
	result := decode(out)
	if err == nil || result.Online || !result.Stale || len(result.Projects) != 1 || result.Projects[0].ID != "browser-demo" {
		t.Fatalf("offline filter lost stale status or selected project: %+v %v", result, err)
	}
	out, err = run("list", "--json")
	result = decode(out)
	if err == nil || len(result.Projects) != 4 || len(result.Disks) != 2 {
		t.Fatal("filtered query damaged the later unfiltered cache result")
	}
	status.Store(http.StatusUnauthorized)
	out, err = run("list", "blog", "--json")
	result = decode(out)
	if err == nil || result.Stale || len(result.Projects) != 0 || strings.Contains(result.Error, "未找到项目") {
		t.Fatal("revocation exposed cached projects or became a selection error")
	}
	status.Store(http.StatusOK)
	for _, selector := range []string{"missing", "blog,missing"} {
		out, err = run("list", selector, "--json")
		result = decode(out)
		if err == nil || !result.Online || len(result.Projects) != 0 || len(result.Disks) != 0 || !strings.Contains(result.Error, "未找到项目") {
			t.Fatalf("missing selector returned full or partial data: %+v %v", result, err)
		}
	}
	out, err = run("list", "missing")
	if err == nil || out != "" {
		t.Fatal("missing text selector should fail without listing projects")
	}
	before := requests.Load()
	for _, args := range [][]string{{""}, {","}, {"blog,"}, {"blog,,archive"}, {"blog", "--timeout", "0s"}, {"blog", "--unknown"}} {
		if _, err := run("list", args...); err == nil {
			t.Fatal("invalid arguments accepted", args)
		}
	}
	if _, err := run("disks", "blog"); err == nil || requests.Load() != before {
		t.Fatal("invalid arguments fetched a snapshot, or disks accepted project selectors")
	}
}
