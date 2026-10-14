// Package toolchain resolves validated executable paths without changing the
// machine PATH or installing dependencies into system directories.
package toolchain

import (
	"context"
	"crypto/sha256"
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/gofrs/flock"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
	"yandu/release"
)

type Tool struct {
	Path    string `json:"path"`
	Version string `json:"version,omitempty"`
	SHA256  string `json:"sha256,omitempty"`
	Source  string `json:"source,omitempty"`
	Pinned  bool   `json:"pinned,omitempty"`
}
type Config struct {
	SchemaVersion   int             `json:"schemaVersion"`
	Role            string          `json:"role"`
	NoMedia         bool            `json:"noMedia,omitempty"`
	Tools           map[string]Tool `json:"tools"`
	SFTPGoTemplates string          `json:"sftpgoTemplates,omitempty"`
	MediaWorker     string          `json:"mediaWorker,omitempty"`
}
type spec struct {
	Version        string
	MinimumVersion string
	Args           []string
}

func specifications() map[string]spec {
	var dependencies map[string]struct {
		Version string `json:"version"`
	}
	var rawStorage map[string]json.RawMessage
	storage := map[string]struct {
		Version        string `json:"version"`
		MinimumVersion string `json:"minimumVersion"`
	}{}
	json.Unmarshal(release.Dependencies, &dependencies)
	json.Unmarshal(release.StorageComponents, &rawStorage)
	for _, name := range []string{"sftpgo", "node", "ffmpeg", "libheif"} {
		var v struct {
			Version        string `json:"version"`
			MinimumVersion string `json:"minimumVersion"`
		}
		json.Unmarshal(rawStorage[name], &v)
		storage[name] = v
	}
	return map[string]spec{
		"caddy":            {Version: dependencies["caddy"].Version, Args: []string{"version"}},
		"frpc":             {Version: dependencies["frp"].Version, Args: []string{"--version"}},
		"frps":             {Version: dependencies["frp"].Version, Args: []string{"--version"}},
		"sftpgo":           {Version: storage["sftpgo"].Version, Args: []string{"--version"}},
		"node":             {Version: storage["node"].Version, MinimumVersion: storage["node"].MinimumVersion, Args: []string{"--version"}},
		"ffmpeg":           {Version: storage["ffmpeg"].Version, Args: []string{"-version"}},
		"ffprobe":          {Version: storage["ffmpeg"].Version, Args: []string{"-version"}},
		"heif-thumbnailer": {Version: storage["libheif"].Version, Args: []string{"-v"}},
	}
}
func names(role string, noMedia bool) ([]string, error) {
	switch role {
	case "cloud":
		return []string{"frps"}, nil
	case "readonly":
		return []string{"caddy", "frpc"}, nil
	case "build":
		return []string{"node"}, nil
	case "local":
		n := []string{"caddy", "frpc", "sftpgo"}
		if !noMedia {
			n = append(n, "node", "ffmpeg", "ffprobe", "heif-thumbnailer")
		}
		return n, nil
	default:
		return nil, fmt.Errorf("tool role must be local, cloud, readonly or build")
	}
}
func Load(path string) (Config, error) {
	var c Config
	f, e := os.Open(path)
	if e != nil {
		return c, e
	}
	defer f.Close()
	d := json.NewDecoder(io.LimitReader(f, 1<<20))
	d.DisallowUnknownFields()
	if e = d.Decode(&c); e != nil {
		return c, e
	}
	if c.SchemaVersion != 1 || c.Tools == nil {
		return c, fmt.Errorf("invalid tools.json schema")
	}
	if _, e = names(c.Role, c.NoMedia); e != nil {
		return c, e
	}
	return c, nil
}
func Save(path string, c Config) error {
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	data, e := json.MarshalIndent(c, "", "  ")
	if e != nil {
		return e
	}
	data = append(data, '\n')
	if previous, e := os.ReadFile(path); e == nil && string(previous) == string(data) {
		return nil
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".tools-")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if e = f.Chmod(0644); e == nil {
		_, e = f.Write(data)
	}
	if e == nil {
		e = f.Sync()
	}
	closeError := f.Close()
	if e == nil {
		e = closeError
	}
	if e != nil {
		return e
	}
	if e = preserveAccess(path, tmp); e != nil {
		return e
	}
	return os.Rename(tmp, path)
}
func hash(path string) (string, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := sha256.New()
	_, e = io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil)), e
}
func nativeExecutable(path string) error {
	switch runtime.GOOS {
	case "darwin":
		if f, e := macho.Open(path); e == nil {
			defer f.Close()
			if f.Cpu == macho.CpuArm64 && runtime.GOARCH == "arm64" {
				return nil
			}
		}
		if f, e := macho.OpenFat(path); e == nil {
			defer f.Close()
			for _, a := range f.Arches {
				if a.Cpu == macho.CpuArm64 && runtime.GOARCH == "arm64" {
					return nil
				}
			}
		}
	case "windows":
		if f, e := pe.Open(path); e == nil {
			defer f.Close()
			if f.Machine == pe.IMAGE_FILE_MACHINE_AMD64 && runtime.GOARCH == "amd64" {
				return nil
			}
		}
	case "linux":
		if f, e := elf.Open(path); e == nil {
			defer f.Close()
			if f.Machine == elf.EM_X86_64 && runtime.GOARCH == "amd64" {
				return nil
			}
		}
	}
	return fmt.Errorf("executable is not native %s/%s", runtime.GOOS, runtime.GOARCH)
}
func output(ctx context.Context, path string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	b, e := exec.CommandContext(ctx, path, args...).CombinedOutput()
	if e != nil {
		return "", fmt.Errorf("tool cannot execute: %w", e)
	}
	return string(b), nil
}
func Validate(ctx context.Context, name, path string) error {
	_, e := inspect(ctx, name, path)
	return e
}

func inspect(ctx context.Context, name, path string) (string, error) {
	s, ok := specifications()[name]
	if !ok {
		return "", fmt.Errorf("unknown tool %s", name)
	}
	info, e := os.Stat(path)
	if e != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("tool is not a regular executable")
	}
	if e = nativeExecutable(path); e != nil {
		return "", e
	}
	v, e := output(ctx, path, s.Args...)
	if e != nil {
		return "", e
	}
	version, e := versionFromOutput(name, strings.SplitN(v, "\n", 2)[0], s)
	if e != nil {
		return "", e
	}
	if name == "ffmpeg" {
		for flag, required := range map[string][]string{"-encoders": {"libx264", "aac", "mjpeg"}, "-muxers": {"mp4", "hls"}, "-filters": {"scale", "setsar"}} {
			v, e = output(ctx, path, "-hide_banner", flag)
			if e != nil {
				return "", e
			}
			for _, capability := range required {
				if !regexp.MustCompile(`\s` + regexp.QuoteMeta(capability) + `(?:\s|,)`).MatchString(v) {
					return "", fmt.Errorf("ffmpeg missing %s", capability)
				}
			}
		}
	}
	return version, nil
}

type Options struct {
	Role, ConfigPath, ComponentRoot string
	NoMedia                         bool
	CheckMediaModules               bool
	NoInstall                       bool
	NoSave                          bool
	// Source development may provide explicit paths; failed explicit overrides
	// fail rather than silently running another binary.
	Overrides  map[string]string
	Candidates map[string][]string
	Lookup     func(string) (string, error)
	Check      func(context.Context, string, string) error
	Install    func(context.Context, string, string) (string, error)
}

func canonical(path string) (string, error) {
	p, e := filepath.Abs(path)
	if e != nil {
		return "", e
	}
	return filepath.EvalSymlinks(p)
}
func templates(binary string, c Config, root string) string {
	for _, p := range []string{filepath.Join(root, "share", "sftpgo", "templates"), c.SFTPGoTemplates, filepath.Join(filepath.Dir(binary), "templates"), filepath.Join(filepath.Dir(binary), "..", "share", "sftpgo", "templates")} {
		if p != "" {
			if v, e := os.Stat(filepath.Join(p, "email")); e == nil && v.IsDir() {
				v, _ := filepath.Abs(p)
				return v
			}
		}
	}
	return ""
}
func Prepare(ctx context.Context, o Options) (Config, error) {
	if !o.NoSave {
		if e := os.MkdirAll(filepath.Dir(o.ConfigPath), 0700); e != nil {
			return Config{}, e
		}
		lock := flock.New(o.ConfigPath + ".lock")
		if ok, e := lock.TryLockContext(ctx, 100*time.Millisecond); e != nil || !ok {
			if e == nil {
				e = fmt.Errorf("tools configuration is locked")
			}
			return Config{}, e
		}
		defer lock.Unlock()
	}
	c, e := Load(o.ConfigPath)
	if e != nil && !os.IsNotExist(e) {
		return c, e
	}
	if c.Tools == nil {
		c.Tools = map[string]Tool{}
	}
	c.SchemaVersion = 1
	c.Role = o.Role
	c.NoMedia = o.NoMedia
	required, e := names(o.Role, o.NoMedia)
	if e != nil {
		return c, e
	}
	lookup := o.Lookup
	if lookup == nil {
		lookup = exec.LookPath
	}
	root, _ := filepath.Abs(o.ComponentRoot)
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	if o.Candidates == nil {
		o.Candidates = map[string][]string{}
	}
	for name, relative := range map[string][]string{"sftpgo": {".local/storage-bin/sftpgo" + suffix}, "ffmpeg": {".local/ffmpeg-9.0.2/bin/ffmpeg" + suffix}, "ffprobe": {".local/ffmpeg-9.0.2/bin/ffprobe" + suffix}, "heif-thumbnailer": {".local/heif-sealed/bin/heif-thumbnailer" + suffix}} {
		for _, p := range relative {
			o.Candidates[name] = append(o.Candidates[name], filepath.Join(root, p))
		}
	}
	for _, name := range required {
		for _, prefix := range []string{".local/bin/bin", ".local/storage-bin", ".local/native-components/bin", ".local/native-components-final/bin"} {
			o.Candidates[name] = append(o.Candidates[name], filepath.Join(root, prefix, name+suffix))
		}
	}
	resolved := map[string]Tool{}
	for _, name := range required {
		type candidate struct{ path, source string }
		list := []candidate{}
		explicit := o.Overrides[name]
		if explicit == "" && c.Tools[name].Pinned {
			explicit = c.Tools[name].Path
		}
		if explicit != "" {
			list = append(list, candidate{explicit, "configured"})
		} else {
			if p, e := lookup(name); e == nil {
				list = append(list, candidate{p, "system"})
			}
			if o.Lookup == nil {
				for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
					if dir != "" && dir != "." {
						list = append(list, candidate{filepath.Join(dir, name+suffix), "system"})
					}
				}
			}
			if p := c.Tools[name].Path; p != "" {
				list = append(list, candidate{p, "configured"})
			}
			if name == "ffprobe" {
				if ffmpeg := resolved["ffmpeg"]; ffmpeg.Path != "" {
					source := "private"
					if ffmpeg.Source == "bundled" {
						source = "bundled"
					}
					list = append(list, candidate{filepath.Join(filepath.Dir(ffmpeg.Path), "ffprobe"+suffix), source})
				}
			}
			for _, p := range o.Candidates[name] {
				list = append(list, candidate{p, "private"})
			}
			list = append(list, candidate{filepath.Join(root, "bin", name+suffix), "bundled"}, candidate{filepath.Join(root, name+suffix), "bundled"})
		}
		last := error(nil)
		found := false
		accept := func(item candidate) bool {
			path, e := canonical(item.path)
			if e != nil {
				last = e
				return false
			}
			if item.source == "bundled" {
				if e = verifyBundled(root, path); e != nil {
					last = e
					return false
				}
			}
			version := specifications()[name].Version
			if o.Check != nil {
				e = o.Check(ctx, name, path)
			} else {
				version, e = inspect(ctx, name, path)
			}
			if e != nil {
				last = e
				return false
			}
			if o.Role == "build" && name == "node" {
				if _, e = NPMEntry(path); e != nil {
					last = e
					return false
				}
			}
			if name == "sftpgo" {
				if p := templates(path, c, root); p != "" {
					c.SFTPGoTemplates = p
				} else {
					last = fmt.Errorf("original SFTPGo templates/email missing")
					return false
				}
			}
			digest, e := hash(path)
			if e != nil {
				last = e
				return false
			}
			resolved[name] = Tool{Path: path, Version: version, SHA256: digest, Source: item.source, Pinned: explicit != ""}
			return true
		}
		for _, item := range list {
			if accept(item) {
				found = true
				break
			}
		}
		if !found && explicit == "" && !o.NoInstall {
			install := o.Install
			if install == nil {
				install = func(ctx context.Context, name, dest string) (string, error) {
					return installTool(ctx, name, dest, root)
				}
			}
			path, e := install(ctx, name, filepath.Join(filepath.Dir(o.ConfigPath), "tools"))
			if e == nil {
				found = accept(candidate{path, "private"})
			} else {
				last = e
			}
		}
		if !found {
			return c, fmt.Errorf("%s unavailable: %w", name, last)
		}
	}
	c.Tools = resolved
	if o.Role == "local" && !o.NoMedia {
		worker := c.MediaWorker
		if worker == "" {
			worker = filepath.Join(root, "media", "worker.mjs")
		}
		worker, e = canonical(worker)
		if e != nil {
			return c, fmt.Errorf("media worker missing: %w", e)
		}
		c.MediaWorker = worker
		if o.CheckMediaModules {
			if e = VerifyMedia(ctx, c); e != nil {
				return c, e
			}
		}
	}
	if !o.NoSave {
		if e = Save(o.ConfigPath, c); e != nil {
			return c, e
		}
	}
	return c, nil
}

func NPMEntry(node string) (string, error) {
	dir := filepath.Dir(node)
	for _, p := range []string{filepath.Join(dir, "node_modules", "npm", "bin", "npm-cli.js"), filepath.Join(dir, "..", "lib", "node_modules", "npm", "bin", "npm-cli.js"), filepath.Join(dir, "..", "libexec", "lib", "node_modules", "npm", "bin", "npm-cli.js")} {
		if v, e := os.Stat(p); e == nil && v.Mode().IsRegular() {
			return canonical(p)
		}
	}
	return "", fmt.Errorf("Node build runtime requires its bundled npm-cli.js")
}

func verifyBundled(root, path string) error {
	data, e := os.ReadFile(filepath.Join(root, "component-receipt.json"))
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	var receipt struct {
		Files map[string]string `json:"files"`
	}
	if e = json.Unmarshal(data, &receipt); e != nil {
		return e
	}
	rel, e := filepath.Rel(root, path)
	if e != nil {
		return e
	}
	expected := receipt.Files[filepath.ToSlash(rel)]
	if expected == "" {
		return fmt.Errorf("bundled tool missing from receipt")
	}
	actual, e := hash(path)
	if e != nil || actual != expected {
		return fmt.Errorf("bundled tool checksum mismatch")
	}
	return nil
}
func Effective(ctx context.Context, c Config) (map[string]Tool, error) {
	n, e := names(c.Role, c.NoMedia)
	if e != nil {
		return nil, e
	}
	result := map[string]Tool{}
	for _, name := range n {
		v, ok := c.Tools[name]
		if !ok {
			return nil, fmt.Errorf("tool config missing %s", name)
		}
		digest, e := hash(v.Path)
		if e != nil || digest != v.SHA256 {
			return nil, fmt.Errorf("%s changed; rerun tools prepare", name)
		}
		if e = Validate(ctx, name, v.Path); e != nil {
			return nil, e
		}
		result[name] = v
	}
	return result, nil
}
func Sorted(c Config) []string {
	n := make([]string, 0, len(c.Tools))
	for name := range c.Tools {
		n = append(n, name)
	}
	sort.Strings(n)
	return n
}
func Environment(c Config) map[string]string {
	env := map[string]string{}
	for _, n := range []string{"ffmpeg", "ffprobe", "heif-thumbnailer"} {
		if t, ok := c.Tools[n]; ok {
			env["YANDU_"+strings.ToUpper(strings.ReplaceAll(n, "-", "_"))] = t.Path
		}
	}
	return env
}

func VerifyMedia(ctx context.Context, c Config) error {
	if c.Role != "local" || c.NoMedia {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.Tools["node"].Path, "--input-type=module", "-e", `await import('sharp'); await import('exiftool-vendored');`)
	cmd.Dir = filepath.Dir(c.MediaWorker)
	if e := cmd.Run(); e != nil {
		return fmt.Errorf("Sharp/ExifTool dependencies cannot load from worker directory: %w", e)
	}
	return nil
}

func Executable(ctx context.Context, c Config, name, root string) (string, error) {
	if t, ok := c.Tools[name]; ok {
		if digest, e := hash(t.Path); e == nil && digest == t.SHA256 {
			if e = Validate(ctx, name, t.Path); e == nil {
				return t.Path, nil
			}
		}
	}
	if c.Tools[name].Pinned {
		return "", fmt.Errorf("pinned %s cannot execute under this identity", name)
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	for _, path := range []string{filepath.Join(root, "bin", name+suffix), filepath.Join(root, name+suffix)} {
		if e := verifyBundled(root, path); e == nil {
			if e = Validate(ctx, name, path); e == nil {
				return path, nil
			}
		}
	}
	return "", fmt.Errorf("%s cannot run under this identity; rerun tools prepare", name)
}
