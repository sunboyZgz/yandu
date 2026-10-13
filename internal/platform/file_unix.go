//go:build !windows

package platform

import (
	"fmt"
	"os"
	"syscall"
)

func RegularFile(path string) (os.FileInfo, error) {
	v, e := os.Lstat(path)
	if e != nil {
		return nil, e
	}
	if !v.Mode().IsRegular() {
		return nil, fmt.Errorf("nonregular managed file")
	}
	if st, ok := v.Sys().(*syscall.Stat_t); !ok || st.Nlink != 1 {
		return nil, fmt.Errorf("hard linked managed file")
	}
	return v, nil
}
