package tools

import (
	"database/sql"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// DirAllowlistStore persists approved directories in a SQLite table.
type DirAllowlistStore struct {
	db *sql.DB
}

// DirAllowlistEntry represents a single approved directory.
type DirAllowlistEntry struct {
	ID         string
	Path       string // absolute, cleaned
	AddedAt    time.Time
	LastUsed   time.Time
	ApprovedBy string // user ID
}

// NewDirAllowlistStore creates the table and returns a store.
func NewDirAllowlistStore(db *sql.DB) (*DirAllowlistStore, error) {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS exec_dir_allowlist (
		id          TEXT PRIMARY KEY,
		path        TEXT NOT NULL UNIQUE,
		added_at    TEXT NOT NULL,
		last_used   TEXT NOT NULL,
		approved_by TEXT NOT NULL DEFAULT ''
	)`)
	if err != nil {
		return nil, err
	}
	return &DirAllowlistStore{db: db}, nil
}

// Add inserts a directory. Does nothing if the exact path already exists.
// Parent/child entries are NOT merged — both are kept.
func (s *DirAllowlistStore) Add(absPath, userID string) error {
	absPath = filepath.Clean(absPath)
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO exec_dir_allowlist (id, path, added_at, last_used, approved_by) VALUES (?, ?, ?, ?, ?)`,
		uuid.New().String(), absPath, now, now, userID,
	)
	return err
}

// Match checks if absDir is equal to or a subdirectory of any approved path.
// Returns the matching entry or nil.
func (s *DirAllowlistStore) Match(absDir string) *DirAllowlistEntry {
	absDir = filepath.Clean(absDir)

	rows, err := s.db.Query(`SELECT id, path, added_at, last_used, approved_by FROM exec_dir_allowlist`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		var e DirAllowlistEntry
		var addedAt, lastUsed string
		if err := rows.Scan(&e.ID, &e.Path, &addedAt, &lastUsed, &e.ApprovedBy); err != nil {
			continue
		}
		e.AddedAt, _ = time.Parse(time.RFC3339, addedAt)
		e.LastUsed, _ = time.Parse(time.RFC3339, lastUsed)

		entryPath := filepath.Clean(e.Path)
		if absDir == entryPath || strings.HasPrefix(absDir, entryPath+string(filepath.Separator)) {
			// Update last_used.
			now := time.Now().UTC().Format(time.RFC3339)
			s.db.Exec(`UPDATE exec_dir_allowlist SET last_used = ? WHERE id = ?`, now, e.ID)
			return &e
		}
	}
	return nil
}

// List returns all approved directories.
func (s *DirAllowlistStore) List() ([]DirAllowlistEntry, error) {
	rows, err := s.db.Query(`SELECT id, path, added_at, last_used, approved_by FROM exec_dir_allowlist ORDER BY added_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []DirAllowlistEntry
	for rows.Next() {
		var e DirAllowlistEntry
		var addedAt, lastUsed string
		if err := rows.Scan(&e.ID, &e.Path, &addedAt, &lastUsed, &e.ApprovedBy); err != nil {
			continue
		}
		e.AddedAt, _ = time.Parse(time.RFC3339, addedAt)
		e.LastUsed, _ = time.Parse(time.RFC3339, lastUsed)
		entries = append(entries, e)
	}
	return entries, nil
}

// Delete removes an entry by ID.
func (s *DirAllowlistStore) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM exec_dir_allowlist WHERE id = ?`, id)
	return err
}
