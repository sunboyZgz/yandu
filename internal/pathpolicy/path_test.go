package pathpolicy

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"yandu/internal/model"
)

func TestExactMappingAndHostIndependentPaths(t *testing.T) {
	for _, c := range []struct{ raw, base, pre, want string }{{"/blog/xxx/xxxxx/xx.png", "/blog", "/blog/xxx/", "xxx/xxxxx/xx.png"}, {"/blog/xxx/%E4%B8%AD%E6%96%87%20a.png?signature=x", "/blog", "/blog/xxx/", "xxx/中文 a.png"}, {"/love/photos/a.jpg", "/love", "/love/photos/", "photos/a.jpg"}, {"/blog/xxx/blog/a.png", "/blog", "/blog/xxx/", "xxx/blog/a.png"}} {
		t.Run(c.raw, func(t *testing.T) {
			v, e := Relative(c.raw, c.base, c.pre)
			if e != nil || filepath.ToSlash(v) != c.want {
				t.Fatalf("got %q, %v", v, e)
			}
		})
	}
}
func TestRejectAmbiguousURLs(t *testing.T) {
	for _, raw := range []string{"/blog", "/blog/xxxevil/a.png", "/blog/api/x.png", "/blog/xxx/../a.png", "/blog/xxx/%2e%2e/a.png", "/blog/xxx/%2Fetc.png", "/blog/xxx/%5cfoo.png", "/blog/xxx/%252e%252e/a.png", "/blog/xxx/a%00.png", "/blog/xxx/a.png:secret", "/blog/xxx/CON.png", "/blog/xxx/COM1.txt", "/blog/xxx/a./b.png", "/blog/xxx/a%20/b.png", "/blog/xxx//a.png", "/blog/xxx/a\\b.png", "/blog/xxx/a%0a.png"} {
		t.Run(raw, func(t *testing.T) {
			if v, e := Relative(raw, "/blog", "/blog/xxx/"); e == nil {
				t.Fatalf("accepted %s", v)
			}
		})
	}
}
func fixture(t *testing.T) (model.Project, model.Snapshot) {
	root, _ := filepath.EvalSymlinks(t.TempDir())
	p := model.Project{ID: "blog", DisplayName: "博客", SiteID: "main", ProjectBase: "/blog", RootPath: root, ResourcePrefixes: []string{"/blog/xxx/"}, Publication: "disabled", AccessPolicy: "public_read", CachePolicy: "revalidate", ContentPolicy: "media_only"}
	s := model.Empty()
	s.Roots = []string{root}
	s.Profile = &model.Profile{Sites: []model.Site{{ID: "main", Domain: "example.com", AllowedPrefixes: []string{"/blog/", "/love/"}}}}
	return p, s
}
func TestProjectOwnership(t *testing.T) {
	p, s := fixture(t)
	if e := ValidateProject(p, s); e != nil {
		t.Fatal(e)
	}
	for _, pre := range []string{"/blog/", "/blog/api/", "/blog/assets/", "/evil/media/", "/blog/xxx", "/blog/a.b/"} {
		q := p
		q.ResourcePrefixes = []string{pre}
		if e := ValidateProject(q, s); e == nil {
			t.Errorf("accepted %s", pre)
		}
	}
	s.Projects = []model.Project{{ID: "other", SiteID: "main", ResourcePrefixes: []string{"/blog/xxx/deep/"}}}
	if e := ValidateProject(p, s); e == nil {
		t.Fatal("nested conflict accepted")
	}
}
func TestLinksAndRootEscape(t *testing.T) {
	root, _ := filepath.EvalSymlinks(t.TempDir())
	outside := t.TempDir()
	os.WriteFile(filepath.Join(outside, "a.png"), []byte("x"), 0600)
	if runtime.GOOS != "windows" {
		os.Symlink(outside, filepath.Join(root, "escape"))
		if _, e := File(root, "escape/a.png"); e == nil {
			t.Fatal("link accepted")
		}
	}
	if _, e := File(root, "../a.png"); e == nil {
		t.Fatal("escape accepted")
	}
	if Within(root, root+"evil") {
		t.Fatal("sibling accepted")
	}
}
