//go:build !windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func startService() error {
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	if e = os.MkdirAll(stateDir, 0700); e != nil {
		return e
	}
	f, e := os.OpenFile(filepath.Join(stateDir, "agent.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if e != nil {
		return e
	}
	defer f.Close()
	c := exec.Command(exe, "--state-dir", stateDir, "serve")
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	c.Stdout = f
	c.Stderr = f
	if e = c.Start(); e != nil {
		return e
	}
	return c.Process.Release()
}
