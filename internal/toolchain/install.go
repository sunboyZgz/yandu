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
func installTool(ctx context.Context, name, dest, sourceRoot string, progress io.Writer) (string, error) {
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
		if e = downloadArchive(ctx, a, archive, &http.Client{Timeout: 5 * time.Minute}, progress); e != nil {
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

func downloadArchive(ctx context.Context, a asset, archive string, client *http.Client, progress io.Writer) error {
	if progress == nil {
		progress = io.Discard
	}
	const attempts = 3
	for attempt := 1; attempt <= attempts; attempt++ {
		if e := ctx.Err(); e != nil {
			return e
		}
		fmt.Fprintf(progress, "下载 %s（第 %d/%d 次，每次最长 %s）\n", a.URL, attempt, attempts, client.Timeout)
		retry, e := downloadAttempt(ctx, a, archive, client)
		if e == nil {
			fmt.Fprintf(progress, "下载并校验完成：%s\n", archive)
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !retry || attempt == attempts {
			return fmt.Errorf("下载 %s 失败（%d 次尝试）：%w；请检查到该地址的出站 HTTPS、DNS 或代理，或预置组件后重试", a.URL, attempt, e)
		}
		fmt.Fprintf(progress, "下载失败：%v；%d 秒后重试。\n", e, attempt)
		timer := time.NewTimer(time.Duration(attempt) * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	panic("unreachable")
}

// Only retry network failures and transient HTTP errors. A failed checksum or
// local write must never publish a payload or be hidden by another download.
func downloadAttempt(ctx context.Context, a asset, archive string, client *http.Client) (bool, error) {
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
	if e != nil {
		return false, e
	}
	res, e := client.Do(req)
	if e != nil {
		return true, e
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return res.StatusCode == 408 || res.StatusCode == 429 || res.StatusCode >= 500, fmt.Errorf("HTTP %d", res.StatusCode)
	}
	f, e := os.CreateTemp(filepath.Dir(archive), ".download-")
	if e != nil {
		return false, e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	h := sha256.New()
	n, e := io.Copy(io.MultiWriter(f, h), io.LimitReader(res.Body, (1<<30)+1))
	closeError := f.Close()
	if e != nil {
		if _, local := e.(*os.PathError); local {
			return false, e
		}
		return true, e
	}
	if closeError != nil {
		return false, closeError
	}
	if n > 1<<30 || hex.EncodeToString(h.Sum(nil)) != a.SHA256 {
		return false, fmt.Errorf("upstream tool archive SHA256 mismatch")
	}
	return false, os.Rename(tmp, archive)
}
func findExecutable(root, name string) (string, error) {
	return findExecutableFor(root, name, runtime.GOOS, runtime.GOARCH)
}

func findExecutableFor(root, name, system, arch string) (string, error) {
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
	if len(found) == 1 {
		return found[0], nil
	}
	// Portable archives can contain x64, ARM64 and x86 copies with the same
	// basename. Select by executable headers, never directory iteration order.
	var native []string
	for _, path := range found {
		if executableFor(path, system, arch) == nil {
			native = append(native, path)
		}
	}
	if len(native) == 1 {
		return native[0], nil
	}
	return "", fmt.Errorf("expected one %s executable for %s/%s in private payload (found %d files, %d matching architecture)", name, system, arch, len(found), len(native))
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
	python := os.Getenv("YANDU_BUILD_PYTHON")
	if python != "" {
		if !filepath.IsAbs(python) {
			return "", fmt.Errorf("YANDU_BUILD_PYTHON must be an absolute interpreter path")
		}
		if info, e := os.Stat(python); e != nil || !info.Mode().IsRegular() {
			return "", fmt.Errorf("configured native build Python is unavailable: %s", python)
		}
	} else {
		var e error
		python, e = exec.LookPath("python3")
		if e != nil {
			python, e = exec.LookPath("python")
		}
		if e != nil {
			return "", fmt.Errorf("native tool missing; use a complete bundle or source Python/native build prerequisites")
		}
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
		if e := os.MkdirAll(dest, 0755); e != nil {
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
	if _, e := os.Stat(args[0]); e != nil {
		return "", fmt.Errorf("native %s missing; no source builder in this installation", name)
	}
	cmd := exec.CommandContext(ctx, python, args...)
	cmd.Dir = root
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if e := cmd.Run(); e != nil {
		return "", fmt.Errorf("native %s build failed; check source compiler prerequisites: %w", name, e)
	}
	return result, nil
}
