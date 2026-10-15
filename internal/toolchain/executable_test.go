package toolchain

import (
	"debug/pe"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"yandu/release"
)

func portablePEFixture(t *testing.T, path string, machine uint16) {
	t.Helper()
	// A minimal PE/COFF header is enough for architecture inspection; it is
	// deliberately never executed by this cross-platform archive test.
	data := make([]byte, 128)
	copy(data, "MZ")
	binary.LittleEndian.PutUint32(data[60:], 64)
	copy(data[64:], "PE\x00\x00")
	binary.LittleEndian.PutUint16(data[68:], machine)
	if e := os.MkdirAll(filepath.Dir(path), 0755); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(path, data, 0755); e != nil {
		t.Fatal(e)
	}
}

func TestPortableArchiveSelectsWindowsX64ByHeader(t *testing.T) {
	root := t.TempDir()
	portablePEFixture(t, filepath.Join(root, "arm64", "sftpgo.exe"), pe.IMAGE_FILE_MACHINE_ARM64)
	portablePEFixture(t, filepath.Join(root, "x86", "sftpgo.exe"), pe.IMAGE_FILE_MACHINE_I386)
	expected := filepath.Join(root, "sftpgo.exe")
	portablePEFixture(t, expected, pe.IMAGE_FILE_MACHINE_AMD64)
	actual, e := findExecutableFor(root, "sftpgo.exe", "windows", "amd64")
	if e != nil || actual != expected {
		t.Fatalf("multi-architecture portable package failed: %s, %v", actual, e)
	}
	// Folder names are not evidence of executable architecture.
	if e := os.Rename(filepath.Join(root, "arm64"), filepath.Join(root, "misleading-x64")); e != nil {
		t.Fatal(e)
	}
	actual, e = findExecutableFor(root, "sftpgo.exe", "windows", "amd64")
	if e != nil || actual != expected {
		t.Fatalf("selected by folder name instead of PE header: %s, %v", actual, e)
	}
}

func TestPortableArchiveRejectsAmbiguousOrMissingX64(t *testing.T) {
	for _, machine := range []uint16{pe.IMAGE_FILE_MACHINE_AMD64, pe.IMAGE_FILE_MACHINE_ARM64} {
		t.Run(fmt.Sprintf("machine-%x", machine), func(t *testing.T) {
			root := t.TempDir()
			portablePEFixture(t, filepath.Join(root, "one", "sftpgo.exe"), machine)
			portablePEFixture(t, filepath.Join(root, "two", "sftpgo.exe"), machine)
			if _, e := findExecutableFor(root, "sftpgo.exe", "windows", "amd64"); e == nil || !strings.Contains(e.Error(), "matching architecture") {
				t.Fatalf("ambiguous or missing architecture accepted: %v", e)
			}
		})
	}
}

func TestLockedSFTPGoWindowsPortableArchive(t *testing.T) {
	archive := os.Getenv("YANDU_TEST_SFTPGO_PORTABLE")
	if archive == "" {
		t.Skip("set YANDU_TEST_SFTPGO_PORTABLE to test the real checksum-locked upstream ZIP")
	}
	var lock struct {
		SFTPGo struct {
			Windows asset `json:"windowsPortable"`
		} `json:"sftpgo"`
	}
	if e := json.Unmarshal(release.StorageComponents, &lock); e != nil {
		t.Fatal(e)
	}
	if actual, e := hash(archive); e != nil || actual != lock.SFTPGo.Windows.SHA256 {
		t.Fatalf("upstream fixture does not match committed checksum: %s, %v", actual, e)
	}
	root := t.TempDir()
	if e := extractArchive(archive, root); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		selected, e := findExecutableFor(root, "sftpgo.exe", "windows", "amd64")
		if e != nil || selected != filepath.Join(root, "sftpgo.exe") {
			t.Fatalf("locked multi-architecture ZIP resolution failed: %s, %v", selected, e)
		}
		if actual := templates(selected, Config{}, root); actual != filepath.Join(root, "templates") {
			t.Fatalf("selected executable cannot locate original email templates: %s", actual)
		}
	}
}
