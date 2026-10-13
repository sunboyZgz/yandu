package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"yandu/internal/pathpolicy"
	"yandu/internal/platform"
)

// VerifyComponents checks bundle integrity; publisher identity still requires release signing.
func VerifyComponents(root string) error {
	var receipt struct {
		Schema   int               `json:"schemaVersion"`
		Platform string            `json:"platform"`
		Files    map[string]string `json:"files"`
	}
	data, err := os.ReadFile(filepath.Join(root, "component-receipt.json"))
	if err != nil {
		return err
	}
	if err = json.Unmarshal(data, &receipt); err != nil {
		return err
	}
	if receipt.Schema != 1 || receipt.Platform != runtime.GOOS+"-"+runtime.GOARCH {
		return fmt.Errorf("component bundle does not match the native platform")
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	for _, name := range []string{"sftpgo", "node", "ffmpeg", "ffprobe", "heif-thumbnailer", "frpc", "caddy"} {
		if receipt.Files["bin/"+name+suffix] == "" {
			return fmt.Errorf("component receipt missing %s", name)
		}
	}
	if receipt.Files["media/worker.mjs"] == "" {
		return fmt.Errorf("media worker missing from receipt")
	}
	for name, expected := range receipt.Files {
		if strings.ContainsAny(name, "\\\x00") || filepath.IsAbs(name) || strings.HasPrefix(name, "../") || filepath.ToSlash(filepath.Clean(name)) != name {
			return fmt.Errorf("invalid component receipt path")
		}
		file := filepath.Join(root, filepath.FromSlash(name))
		if err = pathpolicy.NoLinks(file); err != nil {
			return err
		}
		if _, err = platform.RegularFile(file); err != nil {
			return err
		}
		f, err := os.Open(file)
		if err != nil {
			return err
		}
		hash := sha256.New()
		_, err = io.CopyBuffer(hash, f, make([]byte, 64<<10))
		f.Close()
		if err != nil {
			return err
		}
		if hex.EncodeToString(hash.Sum(nil)) != expected {
			return fmt.Errorf("component checksum mismatch: %s", name)
		}
	}
	return nil
}
