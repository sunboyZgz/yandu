package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"yandu/internal/model"
	"yandu/internal/pathpolicy"
	"yandu/internal/platform"
	"yandu/internal/state"
)

func fileHash(file string) (string, error) {
	if _, e := platform.RegularFile(file); e != nil {
		return "", e
	}
	if e := pathpolicy.NoLinks(file); e != nil {
		return "", e
	}
	f, e := os.Open(file)
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := sha256.New()
	_, e = io.CopyBuffer(h, f, make([]byte, 64<<10))
	return hex.EncodeToString(h.Sum(nil)), e
}

// Restoration never overwrites a live state or storage root. Original paths and IDs are retained,
// then every project is disabled until its service permissions and volume are verified again.
func restoreBackup(ctx context.Context, dir, source string) error {
	if source == "" {
		return fmt.Errorf("backup source required in --destination")
	}
	source, e := filepath.Abs(source)
	if e != nil {
		return e
	}
	if pathpolicy.RootsOverlap(dir, source) {
		return fmt.Errorf("backup and restore state must not overlap")
	}
	entries, e := os.ReadDir(dir)
	if e != nil {
		return e
	}
	for _, v := range entries {
		if v.Name() != "agent.lock" {
			return fmt.Errorf("restore requires an empty state directory")
		}
	}
	var manifest struct {
		Schema int               `json:"schemaVersion"`
		Roots  map[string]string `json:"roots"`
		Files  map[string]string `json:"files"`
	}
	data, e := os.ReadFile(filepath.Join(source, "backup.json"))
	if e != nil {
		return e
	}
	if e = json.Unmarshal(data, &manifest); e != nil {
		return e
	}
	if manifest.Schema != 2 || manifest.Files["storage/storage.db"] == "" || manifest.Files["yandu.db"] == "" {
		return fmt.Errorf("complete schema v2 backup with checksums required")
	}
	for name, sha := range manifest.Files {
		if filepath.IsAbs(name) || strings.ContainsAny(name, "\\\x00") || filepath.ToSlash(filepath.Clean(name)) != name || strings.HasPrefix(name, "../") {
			return fmt.Errorf("invalid backup path")
		}
		actual, err := fileHash(filepath.Join(source, filepath.FromSlash(name)))
		if err != nil {
			return err
		}
		if actual != sha {
			return fmt.Errorf("backup checksum mismatch: %s", name)
		}
	}
	volumes := map[string]platform.Volume{}
	for id, root := range manifest.Roots {
		if !pathpolicy.Identifier(id) || !filepath.IsAbs(root) || pathpolicy.RootsOverlap(source, root) || pathpolicy.RootsOverlap(dir, root) {
			return fmt.Errorf("invalid or overlapping original storage root")
		}
		volume, err := platform.CheckRoot(root)
		if err != nil {
			return err
		}
		items, err := os.ReadDir(root)
		if err != nil || len(items) != 0 {
			return fmt.Errorf("original root %s must exist and be empty", id)
		}
		volumes[id] = volume
		if err = pathpolicy.NoLinks(filepath.Join(source, "roots", id)); err != nil {
			return err
		}
	}
	for name := range manifest.Files {
		if strings.HasPrefix(name, "roots/") {
			continue
		}
		if e = copyFile(filepath.Join(source, filepath.FromSlash(name)), filepath.Join(dir, filepath.FromSlash(name))); e != nil {
			return e
		}
	}
	for id, root := range manifest.Roots {
		if e = copyTree(filepath.Join(source, "roots", id), root); e != nil {
			return e
		}
	}
	db, e := sql.Open("sqlite", filepath.Join(dir, "storage", "storage.db"))
	if e != nil {
		return e
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for id, v := range volumes {
		if _, e = db.ExecContext(ctx, "UPDATE projects SET enabled=0,volume=?,pool=? WHERE id=?", v.ID, v.Pool, id); e != nil {
			return e
		}
		if e = inspectSnapshotFiles(ctx, db, id, manifest.Roots[id], true); e != nil {
			return e
		}
	}
	if _, e = db.Exec("UPDATE jobs SET state='queued',lease=0 WHERE state='running'"); e != nil {
		return e
	}
	store, e := state.Open(dir)
	if e != nil {
		return e
	}
	defer store.Close()
	return store.Update(func(s *model.Snapshot) error {
		for i := range s.Projects {
			s.Projects[i].Publication = "disabled"
			s.Projects[i].Status = "disabled"
		}
		return nil
	})
}

func inspectSnapshotFiles(ctx context.Context, db *sql.DB, project, root string, update bool) error {
	for _, table := range []string{"versions", "derivatives"} {
		rows, e := db.QueryContext(ctx, "SELECT id,path,size,sha256 FROM "+table+" WHERE project=?", project)
		if e != nil {
			return e
		}
		type item struct {
			id, path, sha string
			size          int64
		}
		items := []item{}
		for rows.Next() {
			var v item
			if e = rows.Scan(&v.id, &v.path, &v.size, &v.sha); e != nil {
				rows.Close()
				return e
			}
			items = append(items, v)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		for _, v := range items {
			if e = ctx.Err(); e != nil {
				return e
			}
			file := filepath.Join(root, "files", filepath.FromSlash(v.path))
			info, e := platform.RegularFile(file)
			if e != nil || info.Size() != v.size {
				return fmt.Errorf("registered object missing or length changed")
			}
			hash, e := fileHash(file)
			if e != nil {
				return e
			}
			if hash != v.sha {
				return fmt.Errorf("registered object checksum mismatch")
			}
			if update {
				if _, e = db.ExecContext(ctx, "UPDATE "+table+" SET stamp=? WHERE id=?", info.ModTime().UnixNano(), v.id); e != nil {
					return e
				}
			}
		}
	}
	return nil
}
