package resource

import (
	"encoding/base64"
	"golang.org/x/crypto/bcrypt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"yandu/internal/model"
	"yandu/internal/state"
)

func TestGuardDeniedContentMethodsAndStaleGeneration(t *testing.T) {
	root, _ := filepath.EvalSymlinks(t.TempDir())
	os.Mkdir(filepath.Join(root, "xxx"), 0700)
	png, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a6V8AAAAASUVORK5CYII=")
	os.WriteFile(filepath.Join(root, "xxx", "a.png"), png, 0600)
	os.WriteFile(filepath.Join(root, "xxx", "evil.png"), []byte("<html><script>alert(1)</script></html>"), 0600)
	os.WriteFile(filepath.Join(root, "xxx", "evil.svg"), []byte("<svg/>"), 0600)
	s, e := state.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	p := model.Project{ID: "blog", SiteID: "main", ProjectBase: "/blog", RootPath: root, ResourcePrefixes: []string{"/blog/xxx/"}, Publication: "active", AccessPolicy: "public_read", Generation: "gen"}
	s.Update(func(v *model.Snapshot) error {
		v.Profile = &model.Profile{Sites: []model.Site{{ID: "main", Domain: "example.com"}}}
		v.Effective = []model.Project{p}
		return nil
	})
	g := Guard{s}
	call := func(path, method, gen, auth string) int {
		r := httptest.NewRequest("GET", "http://example.com/authorize", nil)
		r.Header.Set("X-Forwarded-Method", method)
		r.Header.Set("X-Forwarded-Uri", path)
		r.Header.Set("X-Yandu-Generation", gen)
		r.Header.Set("Authorization", auth)
		w := httptest.NewRecorder()
		g.ServeHTTP(w, r)
		return w.Code
	}
	for _, c := range []struct {
		path, method, gen string
		want              int
	}{{"/blog/xxx/a.png", "GET", "gen", 204}, {"/blog/xxx/a.png", "HEAD", "gen", 204}, {"/blog/xxx/a.png", "POST", "gen", 405}, {"/blog/xxx/a.png", "GET", "stale", 404}, {"/blog/xxx/evil.png", "GET", "gen", 403}, {"/blog/xxx/evil.svg", "GET", "gen", 403}, {"/blog/xxx/missing.png", "GET", "gen", 404}, {"/blog/api/x.png", "GET", "gen", 404}} {
		if got := call(c.path, c.method, c.gen, ""); got != c.want {
			t.Errorf("%s: got %d want %d", c.path, got, c.want)
		}
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	s.Update(func(v *model.Snapshot) error {
		v.Effective[0].AccessPolicy = "basic_auth"
		v.Effective[0].Username = "reader"
		v.Effective[0].PasswordHash = string(hash)
		return nil
	})
	if call("/blog/xxx/a.png", "GET", "gen", "") != 401 {
		t.Fatal("password required")
	}
	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte("reader:correct-password"))
	if call("/blog/xxx/a.png", "GET", "gen", auth) != 204 {
		t.Fatal("password rejected")
	}
	s.Update(func(v *model.Snapshot) error { v.Effective[0].Publication = "disabled"; return nil })
	if call("/blog/xxx/a.png", "GET", "gen", auth) != 404 {
		t.Fatal("disable leaked resource")
	}
}
