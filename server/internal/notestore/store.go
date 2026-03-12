// Package notestore provides SQLite persistence for notes.
package notestore

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

// Note represents a persisted note.
type Note struct {
	ID      string    `json:"id"`
	OwnerID string    `json:"owner_id"`
	Title   string    `json:"title"`
	Content string    `json:"content"`
	Tags    []string  `json:"tags,omitempty"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
}

// Store provides SQLite-backed note persistence.
type Store struct {
	db *sql.DB
}

// NewStore creates the notes table and returns a Store.
func NewStore(db *sql.DB) (*Store, error) {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS notes (
			id TEXT PRIMARY KEY,
			owner_id TEXT NOT NULL DEFAULT 'default',
			title TEXT NOT NULL DEFAULT '',
			content TEXT NOT NULL,
			tags TEXT DEFAULT '[]',
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_notes_owner ON notes(owner_id);
	`)
	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func resolveOwnerScope(ownerID []string, fallback string) string {
	if len(ownerID) > 0 {
		return strings.TrimSpace(ownerID[0])
	}
	return strings.TrimSpace(fallback)
}

// Create inserts a new note.
func (s *Store) Create(ctx context.Context, n *Note) error {
	tagsJSON, _ := json.Marshal(n.Tags)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO notes (id, owner_id, title, content, tags, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		n.ID, n.OwnerID, n.Title, n.Content, string(tagsJSON), n.Created, n.Updated)
	return err
}

// Get retrieves a note by ID.
func (s *Store) Get(ctx context.Context, id string, ownerID ...string) (*Note, error) {
	var n Note
	var tagsJSON string
	scopedOwnerID := resolveOwnerScope(ownerID, "")
	var err error
	if scopedOwnerID != "" {
		err = s.db.QueryRowContext(ctx,
			`SELECT id, owner_id, title, content, tags, created_at, updated_at FROM notes WHERE id = ? AND owner_id = ?`, id, scopedOwnerID).
			Scan(&n.ID, &n.OwnerID, &n.Title, &n.Content, &tagsJSON, &n.Created, &n.Updated)
	} else {
		err = s.db.QueryRowContext(ctx,
			`SELECT id, owner_id, title, content, tags, created_at, updated_at FROM notes WHERE id = ?`, id).
			Scan(&n.ID, &n.OwnerID, &n.Title, &n.Content, &tagsJSON, &n.Created, &n.Updated)
	}
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(tagsJSON), &n.Tags)
	return &n, nil
}

// Update modifies an existing note.
func (s *Store) Update(ctx context.Context, n *Note, ownerID ...string) error {
	tagsJSON, _ := json.Marshal(n.Tags)
	scopedOwnerID := resolveOwnerScope(ownerID, n.OwnerID)
	var (
		res sql.Result
		err error
	)
	if scopedOwnerID != "" {
		res, err = s.db.ExecContext(ctx,
			`UPDATE notes SET title = ?, content = ?, tags = ?, updated_at = ? WHERE id = ? AND owner_id = ?`,
			n.Title, n.Content, string(tagsJSON), n.Updated, n.ID, scopedOwnerID)
	} else {
		res, err = s.db.ExecContext(ctx,
			`UPDATE notes SET title = ?, content = ?, tags = ?, updated_at = ? WHERE id = ?`,
			n.Title, n.Content, string(tagsJSON), n.Updated, n.ID)
	}
	if err != nil {
		return err
	}
	if scopedOwnerID != "" {
		if affected, rowsErr := res.RowsAffected(); rowsErr == nil && affected == 0 {
			return sql.ErrNoRows
		}
	}
	return nil
}

// Delete removes a note by ID.
func (s *Store) Delete(ctx context.Context, id string, ownerID ...string) error {
	scopedOwnerID := resolveOwnerScope(ownerID, "")
	var (
		res sql.Result
		err error
	)
	if scopedOwnerID != "" {
		res, err = s.db.ExecContext(ctx, `DELETE FROM notes WHERE id = ? AND owner_id = ?`, id, scopedOwnerID)
	} else {
		res, err = s.db.ExecContext(ctx, `DELETE FROM notes WHERE id = ?`, id)
	}
	if err != nil {
		return err
	}
	if scopedOwnerID != "" {
		if affected, rowsErr := res.RowsAffected(); rowsErr == nil && affected == 0 {
			return sql.ErrNoRows
		}
	}
	return nil
}

// ListByOwner returns all notes for an owner.
func (s *Store) ListByOwner(ctx context.Context, ownerID string) ([]*Note, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, owner_id, title, content, tags, created_at, updated_at
		 FROM notes WHERE owner_id = ? ORDER BY updated_at DESC`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNotes(rows)
}

// Search finds notes matching a query (title or content).
func (s *Store) Search(ctx context.Context, ownerID, query string) ([]*Note, error) {
	like := "%" + query + "%"
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, owner_id, title, content, tags, created_at, updated_at
		 FROM notes WHERE owner_id = ? AND (title LIKE ? OR content LIKE ? OR tags LIKE ?)
		 ORDER BY updated_at DESC`, ownerID, like, like, like)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNotes(rows)
}

func scanNotes(rows *sql.Rows) ([]*Note, error) {
	var notes []*Note
	for rows.Next() {
		var n Note
		var tagsJSON string
		if err := rows.Scan(&n.ID, &n.OwnerID, &n.Title, &n.Content, &tagsJSON, &n.Created, &n.Updated); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(tagsJSON), &n.Tags)
		notes = append(notes, &n)
	}
	return notes, rows.Err()
}
