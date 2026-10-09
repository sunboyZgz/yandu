package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"yandu/internal/cloudsync"
	"yandu/internal/model"
)

func setup(t *testing.T) (Helper, model.Manifest, *int) {
	t.Helper()
	dir := t.TempDir()
	reg := Registry{Sites: []RegisteredSite{{Site: model.Site{ID: "main", Domain: "example.com", AllowedPrefixes: []string{"/blog/"}, ProtectedPrefixes: []string{"/blog/api/"}}}}, Devices: map[string][]string{"owner": {"main"}}}
	b, _ := json.Marshal(reg)
	os.WriteFile(filepath.Join(dir, "registry.json"), b, 0600)
	count := new(int)
	h := Helper{Dir: dir, RegistryPath: filepath.Join(dir, "registry.json"), Nginx: "nginx", Runner: func(context.Context, string, ...string) error { *count++; return nil }, Verify: func(context.Context, RegisteredSite, []model.Route, string) error { return nil }}
	r := model.Route{ID: model.RouteID("blog", "/blog/xxx/"), ProjectID: "blog", Prefix: "/blog/xxx/", State: "active", AccessPolicy: "public_read"}
	return h, model.Manifest{SchemaVersion: 1, OperationID: model.ID(), SiteID: "main", SourceRevision: 1, Routes: []model.Route{r}}, count
}
func TestReplayRevisionAndRetainedDenial(t *testing.T) {
	h, m, count := setup(t)
	ctx := context.Background()
	r, e := h.Call(ctx, "owner", cloudsync.Request{Action: "apply", Manifest: &m})
	if e != nil || !r.Verified {
		t.Fatalf("%+v %v", r, e)
	}
	if *count != 2 {
		t.Fatalf("checks %d", *count)
	}
	r2, e := h.Call(ctx, "owner", cloudsync.Request{Action: "apply", Manifest: &m})
	if e != nil || r2.Generation != r.Generation || *count != 2 {
		t.Fatal("non-idempotent retry")
	}
	m.SourceRevision++
	if _, e = h.Call(ctx, "owner", cloudsync.Request{Action: "apply", Manifest: &m}); e == nil {
		t.Fatal("operation digest mismatch accepted")
	}
	m.OperationID = model.ID()
	if _, e = h.Call(ctx, "owner", cloudsync.Request{Action: "apply", Manifest: &m}); e == nil {
		t.Fatal("stale revision accepted")
	}
	m.ExpectedCloudRevision = 1
	m.Routes = nil
	r, e = h.Call(ctx, "owner", cloudsync.Request{Action: "apply", Manifest: &m})
	if e != nil {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(filepath.Join(h.Dir, "nginx", "current", "main.locations.conf"))
	if !contains(string(b), "return 404;") {
		t.Fatal("omission must preserve denial")
	}
	m.OperationID = model.ID()
	m.ExpectedCloudRevision = r.Revision
	m.Release = []string{model.RouteID("blog", "/blog/xxx/")}
	if _, e = h.Call(ctx, "owner", cloudsync.Request{Action: "apply", Manifest: &m}); e != nil {
		t.Fatal(e)
	}
	b, _ = os.ReadFile(filepath.Join(h.Dir, "nginx", "current", "main.locations.conf"))
	if contains(string(b), "location") {
		t.Fatal("release retained location")
	}
}
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
func TestNginxFailureRestoresPointer(t *testing.T) {
	h, m, _ := setup(t)
	if _, e := h.Call(context.Background(), "owner", cloudsync.Request{Action: "apply", Manifest: &m}); e != nil {
		t.Fatal(e)
	}
	before, _ := os.Readlink(filepath.Join(h.Dir, "nginx", "current"))
	h.Runner = func(context.Context, string, ...string) error { return fmt.Errorf("bad external config") }
	m.OperationID = model.ID()
	m.ExpectedCloudRevision = 1
	m.Routes[0].State = "disabled"
	if _, e := h.Call(context.Background(), "owner", cloudsync.Request{Action: "apply", Manifest: &m}); e == nil {
		t.Fatal("invalid configuration accepted")
	}
	after, _ := os.Readlink(filepath.Join(h.Dir, "nginx", "current"))
	if before != after {
		t.Fatal("pointer not restored")
	}
}
func TestAuthorizationAndDrift(t *testing.T) {
	h, m, _ := setup(t)
	if _, e := h.Call(context.Background(), "attacker", cloudsync.Request{Action: "apply", Manifest: &m}); e == nil {
		t.Fatal("unauthorized principal accepted")
	}
	m.Routes[0].Prefix = "/blog/api/"
	m.Routes[0].ID = model.RouteID("blog", m.Routes[0].Prefix)
	if _, e := h.Call(context.Background(), "owner", cloudsync.Request{Action: "apply", Manifest: &m}); e == nil {
		t.Fatal("protected prefix accepted")
	}
	m.Routes[0].Prefix = "/blog/xxx/"
	m.Routes[0].ID = model.RouteID("blog", m.Routes[0].Prefix)
	h.Call(context.Background(), "owner", cloudsync.Request{Action: "apply", Manifest: &m})
	os.WriteFile(filepath.Join(h.Dir, "nginx", "current", "main.locations.conf"), []byte("# tampered"), 0644)
	m.OperationID = model.ID()
	m.ExpectedCloudRevision = 1
	if _, e := h.Call(context.Background(), "owner", cloudsync.Request{Action: "apply", Manifest: &m}); e == nil {
		t.Fatal("drift accepted")
	}
}

func TestAdministratorBlockCannotBeOverridden(t *testing.T) {
	h, m, _ := setup(t)
	var reg Registry
	if e := readJSON(h.RegistryPath, &reg); e != nil {
		t.Fatal(e)
	}
	reg.Sites[0].Blocked = true
	b, _ := json.Marshal(reg)
	os.WriteFile(h.RegistryPath, b, 0600)
	if _, e := h.Call(context.Background(), "owner", cloudsync.Request{Action: "apply", Manifest: &m}); e == nil || !contains(e.Error(), "CLOUD_BLOCKED") {
		t.Fatalf("blocked site accepted ordinary sync: %v", e)
	}
}
