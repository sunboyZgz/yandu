package storage

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"yandu/internal/inventory"
	"yandu/internal/model"
	"yandu/internal/platform"
)

func TestInventoryCredentialIsolationAndRotation(t *testing.T) {
	f := setup(t)
	pr, _ := f.s.project("one")
	token := "ym_" + model.ID() + model.ID()
	snapshot := model.Snapshot{Profile: &model.Profile{DeviceID: "owner", Storage: &model.StorageTunnel{MonitorToken: token}}, Projects: []model.Project{{ID: "one", DisplayName: "中文项目", Mode: "application_storage", RootPath: pr.Root, Status: "active", PasswordHash: "must-not-leak", ProbeToken: "must-not-leak"}}}
	f.s.InventorySource = func() (model.Snapshot, error) { return snapshot, nil }
	call := func(method, path, credential string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, nil)
		r.Header.Set("Authorization", "Bearer "+credential)
		w := httptest.NewRecorder()
		f.s.PublicHandler().ServeHTTP(w, r)
		return w
	}
	for _, bad := range []string{"", f.token, "ym_" + model.ID() + model.ID()} {
		if w := call("GET", "/storage/v1/monitor", bad); w.Code != 401 {
			t.Fatalf("inventory accepted non-monitor credential: %d %s", w.Code, w.Body)
		}
	}
	w := call("GET", "/storage/v1/monitor", token)
	if w.Code != 200 {
		t.Fatalf("inventory: %d %s", w.Code, w.Body)
	}
	var report inventory.Report
	if e := json.Unmarshal(w.Body.Bytes(), &report); e != nil {
		t.Fatal(e)
	}
	if len(report.Projects) != 1 || report.Projects[0].ID != "one" || len(report.Disks) != 1 || report.Disks[0].FreeBytes == nil {
		t.Fatalf("missing project/disk: %+v", report)
	}
	if strings.Contains(w.Body.String(), "must-not-leak") || strings.Contains(w.Body.String(), token) || strings.Contains(w.Body.String(), "backend_key") {
		t.Fatal("inventory leaked credential/config secrets")
	}
	for _, path := range []string{"/storage/v1/projects/one/capacity", "/storage/v1/projects/one/uploads/id", "/storage/v1/projects/one/objects/id/content"} {
		if w := call("GET", path, token); w.Code != 401 {
			t.Fatalf("monitor token reached project API %s: %d", path, w.Code)
		}
	}
	if w := call("POST", "/storage/v1/projects/one/folders", token); w.Code != 401 {
		t.Fatal("monitor token allowed project writes", w.Code)
	}
	if w := call("POST", "/storage/v1/monitor", token); w.Code != 405 {
		t.Fatal("monitor allowed writes")
	}
	old := token
	snapshot.Profile.Storage.MonitorToken = "ym_" + model.ID() + model.ID()
	if w := call("GET", "/storage/v1/monitor", old); w.Code != 401 {
		t.Fatal("old monitor token remains valid")
	}
	if w := call("GET", "/storage/v1/monitor", snapshot.Profile.Storage.MonitorToken); w.Code != 200 {
		t.Fatal("new token rejected")
	}
	// Old schema-v2 profiles remain usable, but cannot authorize inventory.
	snapshot.Profile.Storage.MonitorToken = ""
	if w := call("GET", "/storage/v1/monitor", old); w.Code != 503 {
		t.Fatal("old profile must request monitoring re-pair")
	}
}

func TestInventorySharedPoolDisabledUsageAndMissingDisk(t *testing.T) {
	f := setup(t)
	root := filepath.Join(filepath.Dir(f.s.Dir), "second")
	if e := os.Mkdir(root, 0700); e != nil {
		t.Fatal(e)
	}
	if e := f.s.Configure(f.s.ctx, model.Project{ID: "two", RootPath: root, Mode: "application_storage", QuotaBytes: 2 << 30, Publication: "disabled"}); e != nil {
		t.Fatal(e)
	}
	// Keep counters in a shared pool, including disabled project reservations.
	if _, e := f.s.DB.Exec("UPDATE projects SET volume=id,pool='shared'"); e != nil {
		t.Fatal(e)
	}
	if _, e := f.s.DB.Exec("INSERT INTO reservations VALUES('r1','one','shared',100),('r2','two','shared',200)"); e != nil {
		t.Fatal(e)
	}
	one, _ := f.s.project("one")
	snapshot := model.Snapshot{Profile: &model.Profile{DeviceID: "owner"}, Projects: []model.Project{{ID: "one", Mode: "application_storage", RootPath: one.Root}, {ID: "two", Mode: "application_storage", RootPath: root}, {ID: "legacy", RootPath: root}}}
	inspect := func(path string) (platform.Volume, error) {
		id := "one"
		free := int64(500 << 20)
		if path == root {
			id = "two"
			free = 400 << 20
		}
		return platform.Volume{ID: id, Pool: "shared", Mount: "/shared", Filesystem: "APFS", Total: 1 << 30, Free: free}, nil
	}
	r, e := f.s.collectInventory(f.s.ctx, snapshot, inspect)
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Disks) != 1 || *r.Summary.TotalBytes != 1<<30 || *r.Summary.FreeBytes != 400<<20 || r.Disks[0].ReservedBytes != 300 {
		t.Fatalf("shared disk counted twice or reservation lost: %+v", r)
	}
	if *r.Disks[0].AvailableAfterReservationsBytes != (400<<20)-300-inventory.SafetyBytes {
		t.Fatal("pool reservation/safety not accounted")
	}
	if r.Projects[0].Mode != "readonly_mapping" || r.Projects[0].UsedBytes != nil {
		t.Fatal("readonly mapping fabricated usage")
	}
	if r.Projects[2].StorageState != "disabled" || *r.Projects[2].ReservedBytes != 200 {
		t.Fatal("disabled project missing capacity")
	}
	// One missing mount prevents a misleading full total, even if a sibling can
	// still report the same shared APFS pool.
	r, e = f.s.collectInventory(f.s.ctx, snapshot, func(path string) (platform.Volume, error) {
		if path == root {
			return platform.Volume{}, fmt.Errorf("unmounted")
		}
		return inspect(path)
	})
	if e != nil {
		t.Fatal(e)
	}
	if r.Summary.FreeBytes != nil || r.Summary.UnavailableDiskCount != 1 || r.Disks[0].FreeBytes != nil {
		t.Fatal("missing disk silently reported as healthy")
	}
	r, e = f.s.collectInventory(f.s.ctx, snapshot, func(path string) (platform.Volume, error) { v, _ := inspect(path); v.Pool = "changed"; return v, nil })
	if e != nil {
		t.Fatal(e)
	}
	if r.Projects[1].Error != "VOLUME_CHANGED" || r.Summary.FreeBytes != nil {
		t.Fatal("changed volume accepted")
	}
}
