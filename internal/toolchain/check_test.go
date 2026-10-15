package toolchain

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCommandCheckValidatesNativeToolWithoutCreatingState(t *testing.T) {
	root := t.TempDir()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	name := "node"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	node := filepath.Join(root, name)
	if err = os.WriteFile(node, data, 0755); err != nil {
		t.Fatal(err)
	}
	npm := filepath.Join(root, "node_modules/npm/bin/npm-cli.js")
	if err = os.MkdirAll(filepath.Dir(npm), 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(npm, nil, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("YANDU_TEST_NODE_VERSION", "v24.3.1")
	state := filepath.Join(root, "uncreated-state")
	var out, diagnostics bytes.Buffer
	err = Command([]string{"prepare", "--check", "--role", "build", "--component-root", root, "--config", filepath.Join(state, "tools.json"), "--tool", "node=" + node}, "unused", "local", &out, &diagnostics)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(state); !os.IsNotExist(err) {
		t.Fatal("read-only check wrote state, configuration or lock files")
	}
	if !strings.Contains(out.String(), "24.3.1") || !strings.Contains(diagnostics.String(), node) {
		t.Fatal("actual version/path diagnostics missing")
	}
}

func TestReadOnlyFailureReportsCauseWithoutInstallationOrWrites(t *testing.T) {
	root := t.TempDir()
	bad := filepath.Join(root, "broken-frps")
	if err := os.WriteFile(bad, []byte("bad"), 0755); err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(root, "uncreated-state")
	var diagnostics bytes.Buffer
	_, err := Prepare(context.Background(), Options{Role: "cloud", ConfigPath: filepath.Join(state, "tools.json"), ComponentRoot: root, NoInstall: true, NoSave: true,
		Lookup: func(string) (string, error) { return bad, nil }, Check: func(context.Context, string, string) error { return fmt.Errorf("shared library unavailable") },
		Install: func(context.Context, string, string) (string, error) {
			t.Fatal("check attempted installation")
			return "", nil
		}, Progress: &diagnostics})
	if err == nil || !strings.Contains(err.Error(), "shared library unavailable") || !strings.Contains(diagnostics.String(), bad) {
		t.Fatalf("probe error was hidden: %v / %s", err, diagnostics.String())
	}
	if _, err = os.Stat(state); !os.IsNotExist(err) {
		t.Fatal("failed check mutated state")
	}
}
