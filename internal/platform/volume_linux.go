package platform

import (
	"fmt"
	"golang.org/x/sys/unix"
)

// Linux is supported for cloud and contract tests, not a production local storage target.
func InspectVolume(root string) (Volume, error) {
	var s unix.Statfs_t
	if e := unix.Statfs(root, &s); e != nil {
		return Volume{}, e
	}
	id := fmt.Sprint(s.Fsid)
	return Volume{ID: id, Pool: id, Filesystem: "development", Free: int64(s.Bavail) * int64(s.Bsize), Total: int64(s.Blocks) * int64(s.Bsize), Mount: root}, nil
}
