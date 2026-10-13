package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/gofrs/flock"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"yandu/internal/model"
	"yandu/internal/pathpolicy"
	"yandu/internal/platform"
	"yandu/internal/state"
)

// Maintenance requires the Agent to be stopped. SQLite VACUUM INTO produces an independent consistent snapshot.
func Maintenance(ctx context.Context, dir, action, project, destination string) error {
	if e := platform.PrivateDir(dir); e != nil {
		return e
	}
	lock := flock.New(filepath.Join(dir, "agent.lock"))
	ok, e := lock.TryLock()
	if e != nil {
		return e
	}
	if !ok {
		return fmt.Errorf("stop the Agent before maintenance")
	}
	defer lock.Unlock()
	if action == "restore" {
		return restoreBackup(ctx, dir, destination)
	}
	db, e := sql.Open("sqlite", filepath.Join(dir, "storage", "storage.db"))
	if e != nil {
		return e
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if action == "backup" {
		u, _ := url.Parse(runtimeConfig(filepath.Join(dir, "storage")).BackendURL)
		if u != nil {
			if conn, err := net.DialTimeout("tcp", u.Host, time.Second); err == nil {
				conn.Close()
				return fmt.Errorf("stop SFTPGo before a consistent backup")
			}
		}
		return backup(ctx, dir, destination, db)
	}
	var p Project
	e = db.QueryRow("SELECT id,root,backend_key,quota,enabled,volume,pool,quarantined FROM projects WHERE id=?", project).Scan(&p.ID, &p.Root, &p.Key, &p.Quota, &p.Enabled, &p.Volume, &p.Pool, &p.Quarantined)
	if e != nil {
		return e
	}
	backend := RuntimeBackend(filepath.Join(dir, "storage"))
	if action == "verify" {
		rows, e := db.Query("SELECT 'versions',id,path,sha256,size FROM versions WHERE project=? UNION ALL SELECT 'derivatives',id,path,sha256,size FROM derivatives WHERE project=?", project, project)
		if e != nil {
			return e
		}
		type item struct {
			table, id, path, sha string
			size                 int64
		}
		items := []item{}
		for rows.Next() {
			var v item
			if e = rows.Scan(&v.table, &v.id, &v.path, &v.sha, &v.size); e != nil {
				rows.Close()
				return e
			}
			items = append(items, v)
		}
		rows.Close()
		s := &Service{DB: db, Backend: backend}
		for _, v := range items {
			stamp, err := s.formalStamp(p, v.path, v.size)
			if err != nil {
				return err
			}
			if e = s.verify(ctx, p, v.path, v.sha, v.size); e != nil {
				return e
			}
			if _, e = db.Exec("UPDATE "+v.table+" SET stamp=? WHERE id=?", stamp, v.id); e != nil {
				return e
			}
		}
		_, e = db.Exec("UPDATE projects SET quarantined=0 WHERE id=?", project)
		return e
	}
	if action != "migrate" {
		return fmt.Errorf("maintenance action must be backup, restore, verify or migrate")
	}
	dest, e := filepath.Abs(destination)
	if e != nil {
		return e
	}
	if pathpolicy.Within(p.Root, dest) || pathpolicy.Within(dest, p.Root) {
		return fmt.Errorf("migration roots must not overlap")
	}
	volume, e := platform.CheckRoot(dest)
	if e != nil {
		return e
	}
	entries, e := os.ReadDir(dest)
	if e != nil || len(entries) > 0 {
		return fmt.Errorf("migration destination must be an empty prepared volume directory")
	}
	if e = inspectSnapshotFiles(ctx, db, p.ID, p.Root, false); e != nil {
		return e
	}
	if e = copyTree(p.Root, dest); e != nil {
		return e
	}
	sftpgo, ok := backend.(*SFTPGo)
	if !ok {
		return fmt.Errorf("SFTPGo runtime required")
	}
	var user map[string]any
	res, e := sftpgo.call(ctx, sftpgo.AdminKey, "GET", "/api/v2/users/yandu-"+project, nil)
	if e != nil {
		return e
	}
	e = json.NewDecoder(res.Body).Decode(&user)
	res.Body.Close()
	if e != nil {
		return e
	}
	user["home_dir"] = filepath.Join(dest, "files")
	if e = sftpgo.json(ctx, "PUT", "/api/v2/users/yandu-"+project, user, nil); e != nil {
		return e
	}
	if _, e = db.Exec("UPDATE projects SET root=?,volume=?,pool=?,enabled=0 WHERE id=?", dest, volume.ID, volume.Pool, project); e != nil {
		return e
	}
	// Copied files receive new modification timestamps; the copy checksums have been verified.
	if e = inspectSnapshotFiles(ctx, db, p.ID, dest, true); e != nil {
		return e
	}
	store, e := state.Open(dir)
	if e != nil {
		return e
	}
	defer store.Close()
	return store.Update(func(snapshot *model.Snapshot) error {
		for i := range snapshot.Projects {
			if snapshot.Projects[i].ID == project {
				snapshot.Projects[i].RootPath = dest
				snapshot.Projects[i].Publication = "disabled"
				snapshot.Projects[i].Status = "disabled"
			}
		}
		snapshot.Roots = append(snapshot.Roots, dest)
		return nil
	})
}
func backup(ctx context.Context, dir, dest string, storageDB *sql.DB) error {
	dest, e := filepath.Abs(dest)
	if e != nil {
		return e
	}
	if pathpolicy.Within(dir, dest) {
		return fmt.Errorf("backup must be outside the state directory")
	}
	if e = platform.PrivateDir(dest); e != nil {
		return e
	}
	entries, e := os.ReadDir(dest)
	if e != nil || len(entries) > 0 {
		return fmt.Errorf("backup destination must be empty")
	}
	rows, e := storageDB.Query("SELECT id,root FROM projects")
	if e != nil {
		return e
	}
	roots := map[string]string{}
	for rows.Next() {
		var id, root string
		rows.Scan(&id, &root)
		roots[id] = root
	}
	rows.Close()
	for _, root := range roots {
		if pathpolicy.Within(root, dest) || pathpolicy.Within(dest, root) {
			return fmt.Errorf("backup destination overlaps managed storage")
		}
	}
	for _, name := range []string{"yandu.db", "storage/storage.db", "sftpgo/sftpgo.db"} {
		source := filepath.Join(dir, name)
		if _, e = os.Stat(source); os.IsNotExist(e) {
			continue
		}
		target := filepath.Join(dest, name)
		if e = os.MkdirAll(filepath.Dir(target), 0700); e != nil {
			return e
		}
		db, e := sql.Open("sqlite", source)
		if e != nil {
			return e
		}
		_, e = db.ExecContext(ctx, "VACUUM INTO ?", target)
		db.Close()
		if e != nil {
			return e
		}
		os.Chmod(target, 0600)
	}
	for _, name := range []string{"storage/runtime.json", "sftpgo/sftpgo.json", "local.token", "Management/local.token", "installation.json"} {
		src := filepath.Join(dir, name)
		if _, e = os.Stat(src); os.IsNotExist(e) {
			continue
		}
		if e = copyFile(src, filepath.Join(dest, name)); e != nil {
			return e
		}
	}
	for id, root := range roots {
		if e = inspectSnapshotFiles(ctx, storageDB, id, root, false); e != nil {
			return e
		}
		if e = copyTree(root, filepath.Join(dest, "roots", id)); e != nil {
			return e
		}
	}
	hashes := map[string]string{}
	if e = filepath.WalkDir(dest, func(file string, item os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if item.IsDir() {
			return nil
		}
		hash, err := fileHash(file)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dest, file)
		hashes[filepath.ToSlash(rel)] = hash
		return nil
	}); e != nil {
		return e
	}
	data, _ := json.MarshalIndent(map[string]any{"schemaVersion": 2, "roots": roots, "files": hashes}, "", "  ")
	return os.WriteFile(filepath.Join(dest, "backup.json"), data, 0600)
}
func copyTree(src, dest string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if e = pathpolicy.NoLinks(path); e != nil {
			return e
		}
		rel, e := filepath.Rel(src, path)
		if e != nil || strings.HasPrefix(rel, "..") {
			return fmt.Errorf("invalid source path")
		}
		out := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(out, 0700)
		}
		return copyFile(path, out)
	})
}
func copyFile(src, dest string) error {
	if _, e := platform.RegularFile(src); e != nil {
		return e
	}
	if e := os.MkdirAll(filepath.Dir(dest), 0700); e != nil {
		return e
	}
	source, e := os.Open(src)
	if e != nil {
		return e
	}
	defer source.Close()
	out, e := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	h := sha256.New()
	_, e = io.CopyBuffer(io.MultiWriter(out, h), source, make([]byte, 64<<10))
	if e == nil {
		e = out.Sync()
	}
	out.Close()
	if e != nil {
		return e
	}
	check, e := os.Open(dest)
	if e != nil {
		return e
	}
	defer check.Close()
	other := sha256.New()
	io.CopyBuffer(other, check, make([]byte, 64<<10))
	if hex.EncodeToString(h.Sum(nil)) != hex.EncodeToString(other.Sum(nil)) {
		return fmt.Errorf("copy checksum mismatch")
	}
	return nil
}
