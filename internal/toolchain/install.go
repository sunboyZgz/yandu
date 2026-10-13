package toolchain

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"yandu/release"
)

type asset struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

func lockedAsset(name string) (asset, error) {
	var deps map[string]struct {
		Version string           `json:"version"`
		Assets  map[string]asset `json:"assets"`
	}
	json.Unmarshal(release.Dependencies, &deps)
	target := runtime.GOOS + "-" + runtime.GOARCH
	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}
	if name == "frpc" || name == "frps" || name == "caddy" {
		group := name
		if group != "caddy" {
			group = "frp"
		}
		system := runtime.GOOS
		if group == "caddy" && system == "darwin" {
			system = "mac"
		}
		key := group + "_" + deps[group].Version + "_" + system + "_" + runtime.GOARCH + ext
		if a, ok := deps[group].Assets[key]; ok {
			return a, nil
		}
	}
	var storage struct {
		Node   map[string]json.RawMessage `json:"node"`
		SFTPGo struct {
			Windows asset `json:"windowsPortable"`
		} `json:"sftpgo"`
	}
	json.Unmarshal(release.StorageComponents, &storage)
	if name == "node" {
		var a asset
		if e := json.Unmarshal(storage.Node[target], &a); e == nil && a.URL != "" {
			return a, nil
		}
	}
	if name == "sftpgo" && runtime.GOOS == "windows" {
		return storage.SFTPGo.Windows, nil
	}
	return asset{}, fmt.Errorf("no binary asset for %s on %s; native source builder or full bundle required", name, target)
}
func installTool(ctx context.Context, name, dest, sourceRoot string) (string, error) {
	a, e := lockedAsset(name)
	if e != nil {
		return buildTool(ctx, name, dest, sourceRoot)
	}
	version := specifications()[name].Version
	group := name
	if name == "frpc" || name == "frps" {
		group = "frp"
	}
	dir := filepath.Join(dest, group+"-"+version+"-"+runtime.GOOS+"-"+runtime.GOARCH)
	if e = os.MkdirAll(dir, 0755); e != nil {
		return "", e
	}
	archive := filepath.Join(dir, filepath.Base(a.URL))
	if actual, e := hash(archive); e != nil || actual != a.SHA256 {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
		req, e := http.NewRequestWithContext(ctx, "GET", a.URL, nil)
		if e != nil {
			return "", e
		}
		res, e := (&http.Client{Timeout: 5 * time.Minute}).Do(req)
		if e != nil {
			return "", e
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			return "", fmt.Errorf("tool download HTTP %d", res.StatusCode)
		}
		f, e := os.CreateTemp(dir, ".download-")
		if e != nil {
			return "", e
		}
		tmp := f.Name()
		defer os.Remove(tmp)
		h := sha256.New()
		n, e := io.Copy(io.MultiWriter(f, h), io.LimitReader(res.Body, (1<<30)+1))
		f.Close()
		if e != nil {
			return "", e
		}
		if n > 1<<30 || hex.EncodeToString(h.Sum(nil)) != a.SHA256 {
			return "", fmt.Errorf("upstream tool archive SHA256 mismatch")
		}
		if e = os.Rename(tmp, archive); e != nil {
			return "", e
		}
	}
	extracted := filepath.Join(dir, "payload")
	// Archive input is checksum-locked; traversal, symlink and hardlink entries
	// are still rejected/omitted and no files are installed outside this prefix.
	if _, e = os.Stat(filepath.Join(extracted, ".complete")); os.IsNotExist(e) {
		stage, e := os.MkdirTemp(dir, ".extract-")
		if e != nil {
			return "", e
		}
		defer os.RemoveAll(stage)
		if e = extractArchive(archive, stage); e != nil {
			return "", e
		}
		if e = os.WriteFile(filepath.Join(stage, ".complete"), []byte(a.SHA256), 0644); e != nil {
			return "", e
		}
		if e = os.Rename(stage, extracted); e != nil {
			return "", e
		}
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	return findExecutable(extracted, name+suffix)
}
func findExecutable(root, name string) (string, error) {
	var found []string
	e := filepath.WalkDir(root, func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if !d.IsDir() && d.Type()&os.ModeSymlink == 0 && d.Name() == name {
			found = append(found, path)
		}
		return nil
	})
	if e != nil {
		return "", e
	}
	if len(found) != 1 {
		return "", fmt.Errorf("expected one %s executable in private payload", name)
	}
	return found[0], nil
}
func safeArchivePath(root, name string) (string, error) {
	if strings.Contains(name, "\\") || strings.ContainsAny(name, ":\x00\r\n") || filepath.IsAbs(name) {
		return "", fmt.Errorf("unsafe archive path")
	}
	name = filepath.FromSlash(name)
	clean := filepath.Clean(name)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("archive traversal")
	}
	return filepath.Join(root, clean), nil
}
func extractArchive(archive, root string) error {
	write := func(name string, mode os.FileMode, r io.Reader) error {
		path, e := safeArchivePath(root, name)
		if e != nil {
			return e
		}
		if e = os.MkdirAll(filepath.Dir(path), 0755); e != nil {
			return e
		}
		f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode.Perm())
		if e != nil {
			return e
		}
		_, e = io.Copy(f, r)
		ce := f.Close()
		if e == nil {
			e = ce
		}
		return e
	}
	if strings.HasSuffix(archive, ".zip") {
		z, e := zip.OpenReader(archive)
		if e != nil {
			return e
		}
		defer z.Close()
		for _, f := range z.File {
			if _, e = safeArchivePath(root, f.Name); e != nil {
				return e
			}
			if f.FileInfo().IsDir() || !f.Mode().IsRegular() {
				continue
			}
			r, e := f.Open()
			if e != nil {
				return e
			}
			e = write(f.Name, f.Mode(), r)
			r.Close()
			if e != nil {
				return e
			}
		}
		return nil
	}
	f, e := os.Open(archive)
	if e != nil {
		return e
	}
	defer f.Close()
	g, e := gzip.NewReader(f)
	if e != nil {
		return e
	}
	defer g.Close()
	t := tar.NewReader(g)
	for {
		h, e := t.Next()
		if e == io.EOF {
			return nil
		}
		if e != nil {
			return e
		}
		if _, e = safeArchivePath(root, h.Name); e != nil {
			return e
		}
		if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeRegA {
			continue
		}
		if e = write(h.Name, os.FileMode(h.Mode), t); e != nil {
			return e
		}
	}
}
func buildTool(ctx context.Context, name, dest, root string) (string, error) {
	// Reuse the existing locked upstream builders; deployments with complete
	// bundles never need compilers, Homebrew, pip or a machine-wide npm install.
	python, e := exec.LookPath("python3")
	if e != nil {
		python, e = exec.LookPath("python")
	}
	if e != nil {
		return "", fmt.Errorf("native tool missing; use a complete bundle or source Python/native build prerequisites")
	}
	var args []string
	var result string
	if name == "sftpgo" && runtime.GOOS == "darwin" {
		args = []string{filepath.Join(root, "scripts/build-sftpgo.py")}
		result = filepath.Join(root, ".local/storage-bin/sftpgo")
	} else if runtime.GOOS == "darwin" && (name == "ffmpeg" || name == "ffprobe" || name == "heif-thumbnailer") {
		group := "ffmpeg"
		result = filepath.Join(root, ".local/ffmpeg-9.0.2/bin", name)
		if name == "heif-thumbnailer" {
			group = "heif"
			result = filepath.Join(root, ".local/heif-sealed/bin", name)
		}
		args = []string{filepath.Join(root, "scripts/build-native-codecs.py"), "--only", group}
	} else if runtime.GOOS == "windows" && (name == "ffmpeg" || name == "ffprobe" || name == "heif-thumbnailer") {
		group := "ffmpeg"
		if name == "heif-thumbnailer" {
			group = "heif"
		}
		if e = os.MkdirAll(dest, 0755); e != nil {
			return "", e
		}
		output, e := os.MkdirTemp(dest, "native-"+group+"-")
		if e != nil {
			return "", e
		}
		if e = os.Remove(output); e != nil {
			return "", e
		}
		args = []string{filepath.Join(root, "release/prepare-native-components.py"), "--only", group, "--output", output}
		result = filepath.Join(output, "bin", name+".exe")
	} else {
		return "", fmt.Errorf("missing native %s; supply a complete platform bundle or configured path", name)
	}
	if _, e = os.Stat(args[0]); e != nil {
		return "", fmt.Errorf("native %s missing; no source builder in this installation", name)
	}
	cmd := exec.CommandContext(ctx, python, args...)
	cmd.Dir = root
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if e = cmd.Run(); e != nil {
		return "", fmt.Errorf("native %s build failed; check source compiler prerequisites: %w", name, e)
	}
	return result, nil
}
