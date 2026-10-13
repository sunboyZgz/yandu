package reconcile

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"yandu/internal/model"
	"yandu/internal/storage"
)

type storageBackend struct{}

func (storageBackend) Provision(context.Context, string, string, int64) (string, error) {
	return "internal-user-key", nil
}
func (storageBackend) Mkdir(context.Context, string, string) error                    { return nil }
func (storageBackend) Upload(context.Context, string, string, io.Reader, int64) error { return nil }
func (storageBackend) Download(context.Context, string, string, string) (*http.Response, error) {
	return &http.Response{StatusCode: 404, Body: http.NoBody}, nil
}
func (storageBackend) Move(context.Context, string, string, string) error { return nil }
func (storageBackend) Remove(context.Context, string, string) error       { return nil }
func TestStorageRetryKeepsActiveProjectEnabled(t *testing.T) {
	e, p := fixture(t)
	p.Mode = "application_storage"
	p.QuotaBytes = 1 << 30
	p.ResourcePrefixes = nil
	p.SiteID = ""
	p.ProjectBase = ""
	root, _ := filepath.EvalSymlinks(t.TempDir())
	os.MkdirAll(root, 0700)
	p.RootPath = root
	e.Store.Update(func(s *model.Snapshot) error { s.Roots = []string{root}; s.Projects = []model.Project{p}; return nil })
	metadata, _ := filepath.EvalSymlinks(t.TempDir())
	service, err := storage.Open(context.Background(), metadata, storageBackend{})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	e.Storage = service
	if err = e.applyLocked(context.Background(), p.ID, "retry-operation", "retry"); err != nil {
		t.Fatal(err)
	}
	var enabled bool
	if err = service.DB.QueryRow("SELECT enabled FROM projects WHERE id=?", p.ID).Scan(&enabled); err != nil || !enabled {
		t.Fatalf("retry disabled active storage: %v", err)
	}
}
