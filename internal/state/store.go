package state

import (
	"database/sql"
	"encoding/json"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"sync"
	"yandu/internal/model"
)

type Store struct {
	db *sql.DB
	mu sync.Mutex
}

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, "yandu.db"))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	for _, q := range []string{"PRAGMA journal_mode=WAL", "PRAGMA busy_timeout=5000", "CREATE TABLE IF NOT EXISTS state(id INTEGER PRIMARY KEY CHECK(id=1), body BLOB NOT NULL)", "CREATE TABLE IF NOT EXISTS journal(id INTEGER PRIMARY KEY AUTOINCREMENT, revision INTEGER NOT NULL, body BLOB NOT NULL, created_at TEXT DEFAULT CURRENT_TIMESTAMP)"} {
		if _, err = db.Exec(q); err != nil {
			db.Close()
			return nil, err
		}
	}
	b, _ := json.Marshal(model.Empty())
	_, err = db.Exec("INSERT OR IGNORE INTO state(id,body) VALUES(1,?)", b)
	if err != nil {
		return nil, err
	}
	os.Chmod(filepath.Join(dir, "yandu.db"), 0600)
	return &Store{db: db}, nil
}
func (s *Store) Close() error { return s.db.Close() }
func (s *Store) Read() (model.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var b []byte
	err := s.db.QueryRow("SELECT body FROM state WHERE id=1").Scan(&b)
	v := model.Empty()
	if err == nil {
		err = json.Unmarshal(b, &v)
	}
	return v, err
}
func (s *Store) Update(f func(*model.Snapshot) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var b []byte
	if err = tx.QueryRow("SELECT body FROM state WHERE id=1").Scan(&b); err != nil {
		return err
	}
	v := model.Empty()
	if err = json.Unmarshal(b, &v); err != nil {
		return err
	}
	if err = f(&v); err != nil {
		return err
	}
	b, err = json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err = tx.Exec("UPDATE state SET body=? WHERE id=1", b); err != nil {
		return err
	}
	// Transactional recovery journal contains no separate source of desired configuration.
	if _, err = tx.Exec("INSERT INTO journal(revision,body) VALUES(?,?)", v.Revision, b); err != nil {
		return err
	}
	_, err = tx.Exec("DELETE FROM journal WHERE id NOT IN (SELECT id FROM journal ORDER BY id DESC LIMIT 20)")
	if err != nil {
		return err
	}
	return tx.Commit()
}
