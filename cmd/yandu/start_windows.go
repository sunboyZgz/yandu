//go:build windows

package main

import (
	"errors"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc/mgr"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func startService() error {
	m, e := mgr.Connect()
	if e == nil {
		defer m.Disconnect()
		s, e := m.OpenService("Yandu")
		if e == nil {
			defer s.Close()
			err := s.Start()
			if errors.Is(err, windows.ERROR_SERVICE_ALREADY_RUNNING) {
				return nil
			}
			return err
		}
	}
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	os.MkdirAll(stateDir, 0700)
	f, e := os.OpenFile(filepath.Join(stateDir, "agent.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if e != nil {
		return e
	}
	defer f.Close()
	c := exec.Command(exe, "--state-dir", stateDir, "serve")
	c.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000008 | 0x00000200, HideWindow: true}
	c.Stdout = f
	c.Stderr = f
	if e = c.Start(); e != nil {
		return e
	}
	return c.Process.Release()
}
