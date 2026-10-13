package toolchain

import (
	"golang.org/x/sys/windows"
	"os"
	"runtime"
)

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
	sd, e := windows.GetNamedSecurityInfo(from, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if e != nil {
		return e
	}
	dacl, _, e := sd.DACL()
	if e != nil {
		return e
	}
	if dacl == nil {
		return os.ErrPermission
	}
	control, _, e := sd.Control()
	if e != nil {
		return e
	}
	flags := windows.SECURITY_INFORMATION(windows.DACL_SECURITY_INFORMATION | windows.UNPROTECTED_DACL_SECURITY_INFORMATION)
	if control&windows.SE_DACL_PROTECTED != 0 {
		flags = windows.DACL_SECURITY_INFORMATION | windows.PROTECTED_DACL_SECURITY_INFORMATION
	}
	e = windows.SetNamedSecurityInfo(to, windows.SE_FILE_OBJECT, flags, nil, nil, dacl, nil)
	runtime.KeepAlive(sd)
	return e
}
