//go:build windows

package pathpolicy

import "golang.org/x/sys/windows"

func isReparse(p string) bool {
	s, e := windows.UTF16PtrFromString(p)
	if e != nil {
		return true
	}
	a, e := windows.GetFileAttributes(s)
	return e == nil && a&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0
}
