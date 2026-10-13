package platform

import (
	"context"
	"fmt"
	"golang.org/x/sys/unix"
	"howett.net/plist"
	"os/exec"
	"strings"
	"time"
)

func InspectVolume(root string) (Volume, error) {
	var stat unix.Statfs_t
	if e := unix.Statfs(root, &stat); e != nil {
		return Volume{}, e
	}
	mount := strings.TrimRight(string(stat.Mntonname[:]), "\x00")
	data, e := diskutil("info", "-plist", mount)
	if e != nil {
		return Volume{}, e
	}
	var info struct {
		FilesystemType         string
		VolumeUUID             string
		APFSVolumeUUID         string
		APFSContainerReference string
		MountPoint             string
	}
	if _, e = plist.Unmarshal(data, &info); e != nil {
		return Volume{}, e
	}
	if info.FilesystemType != "apfs" {
		return Volume{}, fmt.Errorf("managed storage requires APFS")
	}
	data, e = diskutil("apfs", "list", "-plist", info.APFSContainerReference)
	if e != nil {
		return Volume{}, e
	}
	var pools struct {
		Containers []struct {
			APFSContainerUUID string
			CapacityFree      uint64
			CapacityCeiling   uint64
		}
	}
	if _, e = plist.Unmarshal(data, &pools); e != nil || len(pools.Containers) != 1 {
		return Volume{}, fmt.Errorf("APFS container unavailable: %v", e)
	}
	id := info.APFSVolumeUUID
	if id == "" {
		id = info.VolumeUUID
	}
	if id == "" {
		return Volume{}, fmt.Errorf("volume identity missing")
	}
	free := int64(pools.Containers[0].CapacityFree)
	available := int64(stat.Bavail) * int64(stat.Bsize)
	if available < free {
		free = available
	}
	return Volume{ID: id, Pool: pools.Containers[0].APFSContainerUUID, Filesystem: "APFS", Free: free, Total: int64(pools.Containers[0].CapacityCeiling), Mount: mount}, nil
}

func diskutil(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "/usr/sbin/diskutil", args...).Output()
}
