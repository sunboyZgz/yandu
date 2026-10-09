package reconcile

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
	"yandu/internal/cloudsync"
	"yandu/internal/model"
	"yandu/internal/state"
)

func fixture(t *testing.T) (*Engine, model.Project) {
	root, _ := filepath.EvalSymlinks(t.TempDir())
	s, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	s.Update(func(v *model.Snapshot) error {
		v.Roots = []string{root}
		v.Profile = &model.Profile{Sites: []model.Site{{ID: "main", Domain: "example.com", AllowedPrefixes: []string{"/blog/"}}}}
		return nil
	})
	e := &Engine{Store: s, Ctx: context.Background()}
	p := model.Project{ID: "blog", DisplayName: "blog", SiteID: "main", RootPath: root, ProjectBase: "/blog", ResourcePrefixes: []string{"/blog/xxx/"}, Publication: "active", AccessPolicy: "public_read", CachePolicy: "revalidate", ContentPolicy: "media_only"}
	return e, p
}
func TestSaveRequiresConfirmationAndEnforcesRevisions(t *testing.T) {
	e, p := fixture(t)
	p, err := e.Save(p, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if p.Publication != "disabled" || p.Status != "draft" {
		t.Fatal("creating a project published it")
	}
	if _, err = e.Queue(p.ID, "apply", p.Revision, false); err == nil {
		t.Fatal("publication accepted without confirmation")
	}
	if _, err = e.Save(p, "", p.Revision+1); err == nil {
		t.Fatal("stale revision accepted")
	}
	s, _ := e.Store.Read()
	if len(s.Effective) != 0 {
		t.Fatal("saving a draft changed the runtime")
	}
}
func TestDraftImportIsAtomic(t *testing.T) {
	e, p := fixture(t)
	bad := p
	bad.ID = "invalid"
	bad.ResourcePrefixes = []string{"/blog/api/"}
	if err := e.ImportDrafts([]model.Project{p, bad}); err == nil {
		t.Fatal("invalid batch accepted")
	}
	s, _ := e.Store.Read()
	if len(s.Projects) != 0 {
		t.Fatal("partial draft import committed")
	}
	if _, err := os.Stat(p.RootPath); err != nil {
		t.Fatal("import changed resource folder")
	}
}

func TestCloudBanNeverPublishesOrAutomaticallyRecovers(t *testing.T) {
	e, p := fixture(t)
	e.Sync = func(context.Context, model.Profile, cloudsync.Request) (model.Receipt, error) {
		return model.Receipt{}, model.Err("CLOUD_BLOCKED", "管理员已封禁")
	}
	p, err := e.Save(p, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	id, err := e.Queue(p.ID, "apply", p.Revision, true)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		s, _ := e.Store.Read()
		for _, op := range s.Operations {
			if op.ID == id && op.Status == "failed" {
				if s.Projects[0].Publication != "disabled" || s.Projects[0].Status != "cloud_blocked" {
					t.Fatal("cloud ban left publication enabled")
				}
				for _, p := range s.Effective {
					if p.Publication == "active" {
						t.Fatal("cloud ban enabled runtime")
					}
				}
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("ban operation did not finish")
}
