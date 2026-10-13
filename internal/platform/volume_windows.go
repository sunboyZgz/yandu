package platform

import (
	"fmt"
	"golang.org/x/sys/windows"
	"strings"
)

func InspectVolume(root string) (Volume, error) {
	p, e := windows.UTF16PtrFromString(root)
	if e != nil {
		return Volume{}, e
	}
	mount := make([]uint16, windows.MAX_PATH+1)
	if e = windows.GetVolumePathName(p, &mount[0], uint32(len(mount))); e != nil {
		return Volume{}, e
	}
	if windows.GetDriveType(&mount[0]) != windows.DRIVE_FIXED {
		return Volume{}, fmt.Errorf("managed storage requires a fixed drive")
	}
	fs := make([]uint16, 64)
	var serial, max, flags uint32
	if e = windows.GetVolumeInformation(&mount[0], nil, 0, &serial, &max, &flags, &fs[0], uint32(len(fs))); e != nil {
		return Volume{}, e
	}
	if !strings.EqualFold(windows.UTF16ToString(fs), "NTFS") {
		return Volume{}, fmt.Errorf("managed storage requires NTFS")
	}
	id := make([]uint16, 128)
	if e = windows.GetVolumeNameForVolumeMountPoint(&mount[0], &id[0], uint32(len(id))); e != nil {
		return Volume{}, e
	}
	var free, total, available uint64
	if e = windows.GetDiskFreeSpaceEx(&mount[0], &available, &total, &free); e != nil {
		return Volume{}, e
	}
	return Volume{ID: windows.UTF16ToString(id), Pool: windows.UTF16ToString(id), Filesystem: "NTFS", Free: int64(available), Total: int64(total), Mount: windows.UTF16ToString(mount)}, nil
}
