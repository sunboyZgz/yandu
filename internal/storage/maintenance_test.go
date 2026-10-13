package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"yandu/internal/model"
	"yandu/internal/state"
)

func TestConsistentBackupRestoreRetainsIDsAndDisables(t *testing.T) {
	base, _ := filepath.EvalSymlinks(t.TempDir())
	dir := filepath.Join(base, "state")
	root := filepath.Join(base, "原件 空格")
	os.MkdirAll(root, 0700)
	st, e := state.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	p := model.Project{ID: "backup", Mode: "application_storage", RootPath: root, QuotaBytes: 1 << 30, Publication: "active"}
	st.Update(func(v *model.Snapshot) error { v.Roots = []string{root}; v.Projects = []model.Project{p}; return nil })
	st.Close()
	s, e := Open(context.Background(), filepath.Join(dir, "storage"), &testBackend{files: map[string][]byte{}})
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Configure(context.Background(), p); e != nil {
		t.Fatal(e)
	}
	content := []byte("original")
	hash := sha256.Sum256(content)
	sha := hex.EncodeToString(hash[:])
	path := "original/object/version/blob"
	file := filepath.Join(root, "files", filepath.FromSlash(path))
	os.MkdirAll(filepath.Dir(file), 0700)
	os.WriteFile(file, content, 0600)
	s.DB.Exec("INSERT INTO entries(id,project,parent,name,kind,current_version,created) VALUES('object','backup','','原件.txt','file','version',1)")
	if _, e = s.DB.Exec("INSERT INTO versions(id,project,object,size,sha256,mime,path,created) VALUES('version','backup','object',?,?,?,?,1)", len(content), sha, "text/plain", path); e != nil {
		t.Fatal(e)
	}
	s.Close()
	backupPath := filepath.Join(base, "backup")
	os.WriteFile(filepath.Join(dir, "storage", "runtime.json"), []byte(`{"backendUrl":"http://127.0.0.1:1"}`), 0600)
	if e = Maintenance(context.Background(), dir, "backup", "", backupPath); e != nil {
		t.Fatal(e)
	}
	os.Rename(root, root+"-old")
	os.Mkdir(root, 0700)
	restored := filepath.Join(base, "restored")
	if e = Maintenance(context.Background(), restored, "restore", "", backupPath); e != nil {
		t.Fatal(e)
	}
	got, _ := os.ReadFile(file)
	if string(got) != string(content) {
		t.Fatal("restored content")
	}
	st, e = state.Open(restored)
	if e != nil {
		t.Fatal(e)
	}
	defer st.Close()
	v, e := st.Read()
	if e != nil || v.Projects[0].ID != "backup" || v.Projects[0].Publication != "disabled" {
		t.Fatal("restore did not retain identity and disable projects")
	}
	// A corrupted backup is rejected before copying any state files.
	os.WriteFile(filepath.Join(backupPath, "roots", "backup", "files", filepath.FromSlash(path)), []byte("modified"), 0600)
	if e = Maintenance(context.Background(), filepath.Join(base, "rejected"), "restore", "", backupPath); e == nil {
		t.Fatal("tampered backup accepted")
	}
}
