package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"yandu/internal/model"
	"yandu/internal/platform"
	"yandu/internal/toolchain"
)

func BootstrapRuntime(ctx context.Context, dir, componentRoot string) error {
	if e := VerifyComponents(componentRoot); e != nil {
		return e
	}
	resolved, e := toolchain.Prepare(ctx, toolchain.Options{Role: "local", ConfigPath: filepath.Join(dir, "tools.json"), ComponentRoot: componentRoot, CheckMediaModules: true})
	if e != nil {
		return e
	}
	if e := platform.PrivateDir(filepath.Join(dir, "storage")); e != nil {
		return e
	}
	if e := platform.PrivateDir(filepath.Join(dir, "sftpgo")); e != nil {
		return e
	}
	binary := resolved.Tools["sftpgo"].Path
	version, e := exec.Command(binary, "--version").Output()
	if e != nil || !strings.HasPrefix(string(version), "SFTPGo 2.7.6 ") {
		return fmt.Errorf("native SFTPGo 2.7.6 required")
	}
	private := filepath.Join(dir, "storage")
	c := runtimeConfig(private)
	if c.AdminPassword == "" {
		c.AdminPassword = model.ID() + model.ID()
	}
	c.BackendURL = "http://127.0.0.1:18771"
	c.MediaCommand = []string{resolved.Tools["node"].Path, "--max-old-space-size=512", resolved.MediaWorker}
	save := func() error {
		data, _ := json.Marshal(c)
		return os.WriteFile(filepath.Join(private, "runtime.json"), data, 0600)
	}
	if e = save(); e != nil {
		return e
	}
	config := map[string]any{"sftpd": map[string]any{"bindings": []any{map[string]any{"port": 0}}}, "ftpd": map[string]any{"bindings": []any{map[string]any{"port": 0}}}, "webdavd": map[string]any{"bindings": []any{map[string]any{"port": 0}}}, "httpd": map[string]any{"bindings": []any{map[string]any{"port": 18771, "address": "127.0.0.1", "enable_web_admin": false, "enable_web_client": false, "enable_rest_api": true, "render_openapi": false}}, "templates_path": "", "static_files_path": "", "openapi_path": ""}, "smtp": map[string]any{"templates_path": filepath.Join(componentRoot, "share", "sftpgo", "templates")}, "data_provider": map[string]any{"driver": "sqlite", "name": "sftpgo.db", "create_default_admin": true}, "common": map[string]any{"umask": "0027", "symlink_mode": 1}}
	data, _ := json.Marshal(config)
	backendDir := filepath.Join(dir, "sftpgo")
	if e = os.WriteFile(filepath.Join(backendDir, "sftpgo.json"), data, 0600); e != nil {
		return e
	}
	ln, e := net.Listen("tcp", model.SFTPGoAddr)
	if e != nil {
		return fmt.Errorf("stop SFTPGo before bootstrapping: %w", e)
	}
	ln.Close()
	childContext, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(childContext, binary, "serve", "--config-dir", backendDir, "--log-file-path", "")
	cmd.Env = append(os.Environ(), "SFTPGO_DEFAULT_ADMIN_USERNAME=yandu-provisioner", "SFTPGO_DEFAULT_ADMIN_PASSWORD="+c.AdminPassword)
	log, e := os.OpenFile(filepath.Join(backendDir, "bootstrap.log"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if e != nil {
		return e
	}
	defer log.Close()
	cmd.Stdout = log
	cmd.Stderr = log
	if e = cmd.Start(); e != nil {
		return e
	}
	defer func() { cancel(); cmd.Wait() }()
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if c.AdminKey == "" {
			c.AdminKey, e = Bootstrap(ctx, c.BackendURL, c.AdminPassword)
		} else {
			_, e = (&SFTPGo{URL: c.BackendURL, AdminPassword: c.AdminPassword}).adminBearer(ctx)
		}
		if e == nil {
			return save()
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("SFTPGo bootstrap failed; inspect the protected bootstrap log: %w", e)
}
