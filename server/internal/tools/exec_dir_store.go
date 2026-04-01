package tools

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"time"

	z "github.com/IceWhaleTech/zorm"
	"github.com/google/uuid"
)

// DirAllowlistStore persists approved directories in a SQLite table.
type DirAllowlistStore struct {
	db     *sql.DB
	readDB *sql.DB
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
	return NewDirAllowlistStoreWithReadDB(db, db)
}

// NewDirAllowlistStoreWithReadDB creates the table and returns a store with
// separate write and read database handles.
func NewDirAllowlistStoreWithReadDB(writeDB, readDB *sql.DB) (*DirAllowlistStore, error) {
	if writeDB == nil {
		return nil, sql.ErrConnDone
	}
	if readDB == nil {
		readDB = writeDB
	}
	_, err := writeDB.Exec(`CREATE TABLE IF NOT EXISTS exec_dir_allowlist (
		id          TEXT PRIMARY KEY,
		path        TEXT NOT NULL UNIQUE,
		added_at    TEXT NOT NULL,
		last_used   TEXT NOT NULL,
		approved_by TEXT NOT NULL DEFAULT ''
	)`)
	if err != nil {
		return nil, err
	}
	return &DirAllowlistStore{db: writeDB, readDB: readDB}, nil
}

func (s *DirAllowlistStore) reader() *sql.DB {
	if s != nil && s.readDB != nil {
		return s.readDB
	}
	if s == nil {
		return nil
	}
	return s.db
}

func (s *DirAllowlistStore) table(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.db, "exec_dir_allowlist")
}

func (s *DirAllowlistStore) readTable(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.reader(), "exec_dir_allowlist")
}

type dirAllowlistRow struct {
	ID         string `json:"id" zorm:"id"`
	Path       string `json:"path" zorm:"path"`
	AddedAt    string `json:"added_at" zorm:"added_at"`
	LastUsed   string `json:"last_used" zorm:"last_used"`
	ApprovedBy string `json:"approved_by" zorm:"approved_by"`
}

func rowToDirAllowlistEntry(row dirAllowlistRow) DirAllowlistEntry {
	entry := DirAllowlistEntry{
		ID:         row.ID,
		Path:       row.Path,
		ApprovedBy: row.ApprovedBy,
	}
	entry.AddedAt, _ = time.Parse(time.RFC3339, row.AddedAt)
	entry.LastUsed, _ = time.Parse(time.RFC3339, row.LastUsed)
	return entry
}

// Add inserts a directory. Does nothing if the exact path already exists.
// Parent/child entries are NOT merged — both are kept.
func (s *DirAllowlistStore) Add(absPath, userID string) error {
	absPath = filepath.Clean(absPath)
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.table(context.Background()).InsertIgnore(map[string]interface{}{
		"id":          uuid.New().String(),
		"path":        absPath,
		"added_at":    now,
		"last_used":   now,
		"approved_by": userID,
	})
	return err
}

// Match checks if absDir is equal to or a subdirectory of any approved path.
// Returns the matching entry or nil.
func (s *DirAllowlistStore) Match(absDir string) *DirAllowlistEntry {
	absDir = filepath.Clean(absDir)

	var rows []dirAllowlistRow
	_, err := s.readTable(context.Background()).Select(&rows)
	if err != nil {
		return nil
	}

	for i := range rows {
		e := rowToDirAllowlistEntry(rows[i])
		entryPath := filepath.Clean(e.Path)
		if absDir == entryPath || strings.HasPrefix(absDir, entryPath+string(filepath.Separator)) {
			// Update last_used.
			now := time.Now().UTC().Format(time.RFC3339)
			_, _ = s.table(context.Background()).Update(
				z.V{"last_used": now},
				z.Fields("last_used"),
				z.Where(z.Eq("id", e.ID)),
			)
			return &e
		}
	}
	return nil
}

// List returns all approved directories.
func (s *DirAllowlistStore) List() ([]DirAllowlistEntry, error) {
	var rows []dirAllowlistRow
	_, err := s.readTable(context.Background()).Select(&rows, z.OrderBy("added_at"))
	if err != nil {
		return nil, err
	}

	entries := make([]DirAllowlistEntry, 0, len(rows))
	for i := range rows {
		entries = append(entries, rowToDirAllowlistEntry(rows[i]))
	}
	return entries, nil
}

// Delete removes an entry by ID.
func (s *DirAllowlistStore) Delete(id string) error {
	_, err := s.table(context.Background()).Delete(z.Where(z.Eq("id", id)))
	return err
}
