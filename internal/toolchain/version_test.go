package toolchain

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	// A copy of this native test binary acts as Node for resolver tests. It
	// exercises executable validation and version probing without downloads.
	if version := os.Getenv("YANDU_TEST_NODE_VERSION"); version != "" && len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Println(version)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestNodeMinimumVersion(t *testing.T) {
	for _, tc := range []struct {
		output string
		valid  bool
	}{
		{"v22.12.0", true}, {"v22.12.1", true}, {"v22.23.1", true},
		{"v24.0.0", true}, {"v26.1.0", true}, {"v100.0.0", true},
		{"v24.0.0+vendor.1", true}, {"v22.9.0", false}, {"v22.11.99", false},
		{"v20.99.0", false}, {"v22.12.0-rc.1", false}, {"v26.0.0-nightly", false},
		{"", false}, {"v22.12", false}, {"unrecognized version", false},
	} {
		t.Run(tc.output, func(t *testing.T) {
			version, e := versionFromOutput("node", tc.output, specifications()["node"])
			if (e == nil) != tc.valid {
				t.Fatalf("output %q: version=%q, error=%v", tc.output, version, e)
			}
			if tc.valid && version != strings.TrimPrefix(tc.output, "v") {
				t.Fatalf("actual version lost: %q", version)
			}
		})
	}
}

func TestNewerNodeFromPATHIsReusedAndItsActualVersionIsSaved(t *testing.T) {
	for _, version := range []string{"v22.12.0", "v24.3.1", "v26.0.0", "v22.9.0"} {
		t.Run(version, func(t *testing.T) {
			root, e := filepath.EvalSymlinks(t.TempDir())
			if e != nil {
				t.Fatal(e)
			}
			executable, e := os.Executable()
			if e != nil {
				t.Fatal(e)
			}
			data, e := os.ReadFile(executable)
			if e != nil {
				t.Fatal(e)
			}
			name := "node"
			if runtime.GOOS == "windows" {
				name += ".exe"
			}
			node := filepath.Join(root, name)
			if e = os.WriteFile(node, data, 0755); e != nil {
				t.Fatal(e)
			}
			npm := filepath.Join(root, "node_modules", "npm", "bin", "npm-cli.js")
			if e = os.MkdirAll(filepath.Dir(npm), 0755); e != nil {
				t.Fatal(e)
			}
			if e = os.WriteFile(npm, nil, 0644); e != nil {
				t.Fatal(e)
			}
			t.Setenv("YANDU_TEST_NODE_VERSION", version)
			config := filepath.Join(root, "build-tools.json")
			c, e := Prepare(context.Background(), Options{
				Role: "build", ConfigPath: config, ComponentRoot: root, NoInstall: true,
				Lookup: func(string) (string, error) { return node, nil },
			})
			if version == "v22.9.0" {
				if e == nil || !strings.Contains(e.Error(), "unavailable") {
					t.Fatalf("obsolete Node accepted: %v", e)
				}
				if _, e := os.Stat(config); !os.IsNotExist(e) {
					t.Fatal("invalid runtime was registered")
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			selected := c.Tools["node"]
			if selected.Path != node || selected.Source != "system" || selected.Version != version[1:] || selected.SHA256 == "" {
				t.Fatalf("existing newer runtime not registered correctly: %+v", selected)
			}
			saved, e := Load(config)
			if e != nil || saved.Tools["node"].Version != version[1:] {
				t.Fatalf("actual runtime version not persisted: %+v, %v", saved, e)
			}
		})
	}
}

func TestRuntimeMinimumDoesNotChangeProtocolComponentVersions(t *testing.T) {
	s := specifications()["frps"]
	if _, e := versionFromOutput("frps", s.Version, s); e != nil {
		t.Fatal(e)
	}
	if _, e := versionFromOutput("frps", "99.0.0", s); e == nil {
		t.Fatal("runtime policy unexpectedly changed protocol component validation")
	}
}
