package toolchain

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestResolutionPriorityAndOnlyMissingInstall(t *testing.T) {
	root, _ := filepath.EvalSymlinks(t.TempDir())
	config := filepath.Join(root, "state", "tools.json")
	create := func(label string) string {
		p := filepath.Join(root, label, "中文 工具")
		os.MkdirAll(filepath.Dir(p), 0755)
		os.WriteFile(p, []byte(label), 0755)
		return p
	}
	system := create("system")
	configured := create("configured")
	bundled := filepath.Join(root, "bin", "frpc")
	os.MkdirAll(filepath.Dir(bundled), 0755)
	os.WriteFile(bundled, []byte("bundled"), 0755)
	if e := Save(config, Config{SchemaVersion: 1, Role: "readonly", Tools: map[string]Tool{"caddy": {Path: configured}, "frpc": {Path: configured}}}); e != nil {
		t.Fatal(e)
	}
	installed := []string{}
	options := Options{Role: "readonly", ConfigPath: config, ComponentRoot: root, Lookup: func(name string) (string, error) { return system, nil }, Check: func(context.Context, string, string) error { return nil }, Install: func(_ context.Context, name, _ string) (string, error) {
		installed = append(installed, name)
		return create("installed-" + name), nil
	}}
	c, e := Prepare(context.Background(), options)
	if e != nil {
		t.Fatal(e)
	}
	if c.Tools["caddy"].Path != system || c.Tools["caddy"].Source != "system" || len(installed) != 0 {
		t.Fatal("valid PATH did not beat configuration/bundle")
	}
	options.Check = func(_ context.Context, _ string, path string) error {
		if path == system {
			return fmt.Errorf("wrong version")
		}
		return nil
	}
	// Restore a user-configured path after the previous successful preparation.
	Save(config, Config{SchemaVersion: 1, Role: "readonly", Tools: map[string]Tool{"caddy": {Path: configured}, "frpc": {Path: configured}}})
	c, e = Prepare(context.Background(), options)
	if e != nil {
		t.Fatal(e)
	}
	if c.Tools["caddy"].Path != configured || c.Tools["caddy"].Source != "configured" || len(installed) != 0 {
		t.Fatal("bad PATH ignored valid configured tool")
	}
	os.Remove(bundled)
	Save(config, Config{SchemaVersion: 1, Role: "readonly", Tools: map[string]Tool{"caddy": {Path: configured}}})
	c, e = Prepare(context.Background(), options)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(installed, []string{"frpc"}) {
		t.Fatalf("reinstalled existing tool: %v", installed)
	}
	if c.Tools["frpc"].SHA256 == "" || !filepath.IsAbs(c.Tools["frpc"].Path) {
		t.Fatal("registration missing absolute path/hash")
	}
	if saved, e := Load(config); e != nil || saved.Tools["caddy"].Path != configured {
		t.Fatal("paths not persisted")
	}
}

func TestExplicitInvalidToolDoesNotSilentlyInstallAnother(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "bad")
	os.WriteFile(p, []byte("bad"), 0755)
	_, e := Prepare(context.Background(), Options{Role: "cloud", ConfigPath: filepath.Join(root, "tools.json"), ComponentRoot: root, Overrides: map[string]string{"frps": p}, Check: func(context.Context, string, string) error { return fmt.Errorf("wrong version") }, Install: func(context.Context, string, string) (string, error) {
		t.Fatal("explicit override replaced")
		return "", nil
	}})
	if e == nil || !strings.Contains(e.Error(), "wrong version") {
		t.Fatal("invalid explicit path accepted")
	}
}

func TestCorruptRegistryFailsWithoutOverwrite(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "tools.json")
	bad := []byte(`{"schemaVersion":999,"role":"cloud","tools":{}}`)
	os.WriteFile(p, bad, 0644)
	_, e := Prepare(context.Background(), Options{Role: "cloud", ConfigPath: p, ComponentRoot: root})
	if e == nil {
		t.Fatal("unknown registry schema accepted")
	}
	after, _ := os.ReadFile(p)
	if string(after) != string(bad) {
		t.Fatal("corrupt user configuration overwritten")
	}
}

func TestPrivateArchiveRejectsTraversalAndOmitsLinks(t *testing.T) {
	for _, name := range []string{"../escape", "/escape", "C:escape", "dir\\escape"} {
		if _, e := safeArchivePath(t.TempDir(), name); e == nil {
			t.Fatal("unsafe archive path accepted", name)
		}
	}
	root := t.TempDir()
	archive := filepath.Join(root, "tools.zip")
	f, _ := os.Create(archive)
	z := zip.NewWriter(f)
	h := &zip.FileHeader{Name: "payload/link"}
	h.SetMode(os.ModeSymlink | 0755)
	w, _ := z.CreateHeader(h)
	w.Write([]byte("../../escape"))
	w, _ = z.Create("payload/node")
	w.Write([]byte("binary"))
	z.Close()
	f.Close()
	dest := filepath.Join(root, "private")
	if e := extractArchive(archive, dest); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Lstat(filepath.Join(dest, "payload/link")); !os.IsNotExist(e) {
		t.Fatal("archive created symlink")
	}
	if p, e := findExecutable(dest, "node"); e != nil || !strings.HasPrefix(p, dest) {
		t.Fatal("private extraction failed")
	}
}

func TestConfiguredHashDriftDoesNotExecuteTool(t *testing.T) {
	p := filepath.Join(t.TempDir(), "tool")
	os.WriteFile(p, []byte("old"), 0755)
	digest, _ := hash(p)
	os.WriteFile(p, []byte("new"), 0755)
	_, e := Effective(context.Background(), Config{Role: "cloud", Tools: map[string]Tool{"frps": {Path: p, SHA256: digest}}})
	if e == nil || !strings.Contains(e.Error(), "changed") {
		t.Fatal("modified registered executable accepted")
	}
}

func TestBundledFFprobeSiblingRequiresReceipt(t *testing.T) {
	root, _ := filepath.EvalSymlinks(t.TempDir())
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	os.MkdirAll(filepath.Join(root, "bin"), 0755)
	os.MkdirAll(filepath.Join(root, "share", "sftpgo", "templates", "email"), 0755)
	os.MkdirAll(filepath.Join(root, "media"), 0755)
	os.WriteFile(filepath.Join(root, "media", "worker.mjs"), []byte("worker"), 0644)
	files := map[string]string{}
	names, _ := names("local", false)
	for _, name := range names {
		p := filepath.Join(root, "bin", name+suffix)
		os.WriteFile(p, []byte(name), 0755)
		files["bin/"+name+suffix], _ = hash(p)
	}
	writeReceipt := func() {
		data, _ := json.Marshal(map[string]any{"files": files})
		if e := os.WriteFile(filepath.Join(root, "component-receipt.json"), data, 0644); e != nil {
			t.Fatal(e)
		}
	}
	writeReceipt()
	o := Options{Role: "local", ComponentRoot: root, ConfigPath: filepath.Join(root, "tools.json"), NoInstall: true,
		Lookup: func(string) (string, error) { return "", os.ErrNotExist }, Check: func(context.Context, string, string) error { return nil }}
	c, e := Prepare(context.Background(), o)
	if e != nil || c.Tools["ffprobe"].Source != "bundled" {
		t.Fatalf("bundled sibling not resolved: %v, %+v", e, c.Tools["ffprobe"])
	}
	files["bin/ffprobe"+suffix] = strings.Repeat("0", 64)
	writeReceipt()
	o.ConfigPath = filepath.Join(root, "fresh-tools.json")
	if _, e = Prepare(context.Background(), o); e == nil || !strings.Contains(e.Error(), "ffprobe unavailable") {
		t.Fatalf("unchecked bundled sibling accepted: %v", e)
	}
}
