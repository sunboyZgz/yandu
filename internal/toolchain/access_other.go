//go:build !windows

package toolchain

import "os"

func preserveAccess(from, to string) error {
	info, e := os.Lstat(from)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	if !info.Mode().IsRegular() {
		return os.ErrInvalid
	}
	return os.Chmod(to, info.Mode().Perm())
}
