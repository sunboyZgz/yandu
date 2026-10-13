package storage

import (
	"fmt"
	"path/filepath"
	"yandu/internal/pathpolicy"
	"yandu/internal/platform"
)

func (s *Service) formalStamp(pr Project, path string, size int64) (int64, error) {
	if _, ok := s.Backend.(*SFTPGo); !ok {
		return 0, nil
	} // Contract fakes have no physical home.
	file := filepath.Join(pr.Root, "files", filepath.FromSlash(path))
	if e := pathpolicy.NoLinks(file); e != nil {
		return 0, e
	}
	v, e := platform.RegularFile(file)
	if e != nil {
		return 0, e
	}
	if v.Size() != size {
		return 0, fmt.Errorf("formal file length changed")
	}
	return v.ModTime().UnixNano(), nil
}
func (s *Service) checkFormal(pr Project, path string, size, stamp int64) error {
	actual, e := s.formalStamp(pr, path, size)
	if e != nil || stamp != 0 && actual != stamp {
		s.DB.Exec("UPDATE projects SET quarantined=1 WHERE id=?", pr.ID)
		return fail("INTEGRITY_QUARANTINE", "managed file changed or linked; maintenance inspection required", 503)
	}
	return nil
}
