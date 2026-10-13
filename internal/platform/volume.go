package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"yandu/internal/pathpolicy"
)

type Volume struct {
	ID         string `json:"id"`
	Pool       string `json:"pool"`
	Filesystem string `json:"filesystem"`
	Free       int64  `json:"freeBytes"`
	Total      int64  `json:"totalBytes"`
	Mount      string `json:"mountPoint"`
}

func CheckRoot(root string) (Volume, error) {
	if !filepath.IsAbs(root) {
		return Volume{}, fmt.Errorf("storage root must be absolute")
	}
	if e := pathpolicy.CheckDirectory(root); e != nil {
		return Volume{}, e
	}
	return InspectVolume(root)
}
func PrivateDir(path string) error {
	parent := path
	for {
		_, e := os.Lstat(parent)
		if e == nil {
			break
		}
		if !os.IsNotExist(e) {
			return e
		}
		next := filepath.Dir(parent)
		if next == parent {
			return e
		}
		parent = next
	}
	if e := pathpolicy.NoLinks(parent); e != nil {
		return e
	}
	if e := os.MkdirAll(path, 0700); e != nil {
		return e
	}
	return pathpolicy.NoLinks(path)
}
