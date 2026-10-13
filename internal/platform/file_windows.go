package platform

import (
	"fmt"
	"golang.org/x/sys/windows"
	"os"
)

func RegularFile(path string) (os.FileInfo, error) {
	v, e := os.Lstat(path)
	if e != nil {
		return nil, e
	}
	if !v.Mode().IsRegular() {
		return nil, fmt.Errorf("nonregular managed file")
	}
	ptr, e := windows.UTF16PtrFromString(path)
	if e != nil {
		return nil, e
	}
	h, e := windows.CreateFile(ptr, windows.FILE_READ_ATTRIBUTES, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if e != nil {
		return nil, e
	}
	defer windows.CloseHandle(h)
	var info windows.ByHandleFileInformation
	if e = windows.GetFileInformationByHandle(h, &info); e != nil {
		return nil, e
	}
	if info.NumberOfLinks != 1 || info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return nil, fmt.Errorf("linked managed file")
	}
	return v, nil
}
