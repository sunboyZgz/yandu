package service

import (
	"os"
	"path/filepath"
	"runtime"
)

func StateDir() string {
	if p := os.Getenv("YANDU_STATE_DIR"); p != "" {
		return p
	}
	if runtime.GOOS == "windows" {
		p := os.Getenv("ProgramData")
		if p == "" {
			p = `C:\ProgramData`
		}
		return filepath.Join(p, "Yandu")
	}
	p, _ := os.UserConfigDir()
	return filepath.Join(p, "yandu")
}
