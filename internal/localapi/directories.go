package localapi

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"yandu/internal/model"
	"yandu/internal/pathpolicy"
)

type directory struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// Browsing and picking never grant access. The user must still authorize the root.
func (a *API) browseDirectories(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	if p == "" {
		p, _ = os.UserHomeDir()
	}
	p = filepath.Clean(p)
	if !filepath.IsAbs(p) {
		failure(w, model.Err("PATH_OUTSIDE_ROOT", "请输入本机目录的绝对路径"))
		return
	}
	if err := pathpolicy.CheckDirectory(p); err != nil {
		failure(w, err)
		return
	}
	entries, err := os.ReadDir(p)
	if err != nil {
		failure(w, model.Err("ACCESS_DENIED", "无法读取此目录，请选择有权限的文件夹"))
		return
	}
	dirs := []directory{}
	for _, entry := range entries {
		child := filepath.Join(p, entry.Name())
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || pathpolicy.NoLinks(child) != nil {
			continue
		}
		dirs = append(dirs, directory{Name: entry.Name(), Path: child})
	}
	shortcuts := []directory{}
	if home, err := os.UserHomeDir(); err == nil {
		shortcuts = append(shortcuts, directory{Name: "用户目录", Path: home})
	}
	if runtime.GOOS == "windows" {
		for drive := 'A'; drive <= 'Z'; drive++ {
			root := string(drive) + ":\\"
			if info, err := os.Stat(root); err == nil && info.IsDir() {
				shortcuts = append(shortcuts, directory{Name: root, Path: root})
			}
		}
	}
	selectable := a.directorySelectable(p) == nil
	send(w, 200, map[string]any{"path": p, "parent": filepath.Dir(p), "directories": dirs, "shortcuts": shortcuts, "selectable": selectable})
}

func (a *API) directorySelectable(p string) error {
	if !filepath.IsAbs(p) || filepath.Dir(p) == p || pathpolicy.Within(p, a.Engine.Dir) || pathpolicy.Within(a.Engine.Dir, p) {
		return model.Err("PATH_OUTSIDE_ROOT", "请选择独立的素材目录，不能选择磁盘根或工具状态目录")
	}
	return pathpolicy.CheckDirectory(p)
}

func (a *API) pickDirectory(w http.ResponseWriter, r *http.Request) {
	if !a.pickerMu.TryLock() {
		failure(w, model.Err("OPERATION_CONFLICT", "文件夹选择窗口已打开，请先完成选择"))
		return
	}
	defer a.pickerMu.Unlock()
	// Give the user time to browse without changing timeouts for other API calls.
	timeout := 5 * time.Minute
	if err := http.NewResponseController(w).SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		timeout = 50 * time.Second
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	picker := a.directoryPicker
	if picker == nil {
		picker = nativeDirectoryPicker
	}
	p, err := picker(ctx)
	if err != nil {
		failure(w, model.Err("PICKER_UNAVAILABLE", "系统选择器未能完成选择，请使用下方的目录浏览器"))
		return
	}
	if p == "" {
		send(w, 200, map[string]any{"cancelled": true})
		return
	}
	p = filepath.Clean(p)
	if err := a.directorySelectable(p); err != nil {
		failure(w, err)
		return
	}
	send(w, 200, map[string]string{"path": p})
}

func nativeDirectoryPicker(ctx context.Context) (string, error) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, "osascript", "-e", `try
tell application "System Events" to activate
return POSIX path of (choose folder with prompt "选择檐渡使用的本地文件夹")
on error number -128
return ""
end try`)
	case "windows":
		cmd = exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-STA", "-Command", `[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
if (-not [Environment]::UserInteractive) { exit 2 }
Add-Type -AssemblyName System.Windows.Forms
$picker = New-Object System.Windows.Forms.FolderBrowserDialog
$picker.Description = '选择檐渡使用的本地文件夹'
$picker.ShowNewFolderButton = $true
try { if ($picker.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) { [Console]::Write($picker.SelectedPath) } } finally { $picker.Dispose() }`)
	default:
		if _, err := exec.LookPath("zenity"); err != nil {
			return "", err
		}
		cmd = exec.CommandContext(ctx, "zenity", "--file-selection", "--directory", "--title=选择檐渡使用的本地文件夹")
	}
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if runtime.GOOS != "windows" && runtime.GOOS != "darwin" && errors.As(err, &exit) && exit.ExitCode() == 1 {
			return "", nil // Zenity's cancel button.
		}
		return "", err
	}
	return strings.TrimSuffix(strings.TrimSuffix(strings.TrimPrefix(string(out), "\ufeff"), "\n"), "\r"), nil
}
