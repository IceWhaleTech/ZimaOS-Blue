// Package notestore provides SQLite persistence for notes.
package notestore

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	z "github.com/IceWhaleTech/zorm"
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
	db     *sql.DB
	readDB *sql.DB
}

// NewStore creates the notes table and returns a Store.
func NewStore(db *sql.DB) (*Store, error) {
	return NewStoreWithReadDB(db, db)
}

// NewStoreWithReadDB creates the notes table and returns a Store with separate
// write and read database handles.
func NewStoreWithReadDB(writeDB, readDB *sql.DB) (*Store, error) {
	if readDB == nil {
		readDB = writeDB
	}
	_, err := writeDB.Exec(`
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
	return &Store{db: writeDB, readDB: readDB}, nil
}

func (s *Store) reader() *sql.DB {
	if s != nil && s.readDB != nil {
		return s.readDB
	}
	if s == nil {
		return nil
	}
	return s.db
}

func (s *Store) table(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.db, "notes")
}

func (s *Store) readTable(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.reader(), "notes")
}

type noteRow struct {
	ID        string `json:"id" zorm:"id"`
	OwnerID   string `json:"owner_id" zorm:"owner_id"`
	Title     string `json:"title" zorm:"title"`
	Content   string `json:"content" zorm:"content"`
	Tags      string `json:"tags" zorm:"tags"`
	CreatedAt string `json:"created_at" zorm:"created_at"`
	UpdatedAt string `json:"updated_at" zorm:"updated_at"`
}

func resolveOwnerScope(ownerID []string, fallback string) string {
	if len(ownerID) > 0 {
		return strings.TrimSpace(ownerID[0])
	}
	return strings.TrimSpace(fallback)
}

func formatNoteTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseNoteTime(raw string) time.Time {
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02T15:04:05.999999999-07:00",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func rowToNote(row noteRow) (*Note, error) {
	note := &Note{
		ID:      row.ID,
		OwnerID: row.OwnerID,
		Title:   row.Title,
		Content: row.Content,
		Created: parseNoteTime(row.CreatedAt),
		Updated: parseNoteTime(row.UpdatedAt),
	}
	if strings.TrimSpace(row.Tags) != "" {
		if err := json.Unmarshal([]byte(row.Tags), &note.Tags); err != nil {
			return nil, err
		}
	}
	return note, nil
}

func rowsToNotes(rows []noteRow) ([]*Note, error) {
	notes := make([]*Note, 0, len(rows))
	for i := range rows {
		note, err := rowToNote(rows[i])
		if err != nil {
			return nil, err
		}
		notes = append(notes, note)
	}
	return notes, nil
}

// Create inserts a new note.
func (s *Store) Create(ctx context.Context, n *Note) error {
	tagsJSON, err := json.Marshal(n.Tags)
	if err != nil {
		return err
	}
	_, err = s.table(ctx).Insert(map[string]interface{}{
		"id":         n.ID,
		"owner_id":   n.OwnerID,
		"title":      n.Title,
		"content":    n.Content,
		"tags":       string(tagsJSON),
		"created_at": formatNoteTime(n.Created),
		"updated_at": formatNoteTime(n.Updated),
	})
	return err
}

// Get retrieves a note by ID.
func (s *Store) Get(ctx context.Context, id string, ownerID ...string) (*Note, error) {
	scopedOwnerID := resolveOwnerScope(ownerID, "")
	conds := []interface{}{z.Eq("id", id)}
	if scopedOwnerID != "" {
		conds = append(conds, z.Eq("owner_id", scopedOwnerID))
	}

	var rows []noteRow
	_, err := s.readTable(ctx).Select(&rows, z.Where(conds...), z.Limit(1))
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	return rowToNote(rows[0])
}

// Update modifies an existing note.
func (s *Store) Update(ctx context.Context, n *Note, ownerID ...string) error {
	tagsJSON, err := json.Marshal(n.Tags)
	if err != nil {
		return err
	}
	scopedOwnerID := resolveOwnerScope(ownerID, n.OwnerID)
	conds := []interface{}{z.Eq("id", n.ID)}
	if scopedOwnerID != "" {
		conds = append(conds, z.Eq("owner_id", scopedOwnerID))
	}
	affected, err := s.table(ctx).Update(
		z.V{
			"title":      n.Title,
			"content":    n.Content,
			"tags":       string(tagsJSON),
			"updated_at": formatNoteTime(n.Updated),
		},
		z.Fields("title", "content", "tags", "updated_at"),
		z.Where(conds...),
	)
	if err != nil {
		return err
	}
	if scopedOwnerID != "" && affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// Delete removes a note by ID.
func (s *Store) Delete(ctx context.Context, id string, ownerID ...string) error {
	scopedOwnerID := resolveOwnerScope(ownerID, "")
	conds := []interface{}{z.Eq("id", id)}
	if scopedOwnerID != "" {
		conds = append(conds, z.Eq("owner_id", scopedOwnerID))
	}
	affected, err := s.table(ctx).Delete(z.Where(conds...))
	if err != nil {
		return err
	}
	if scopedOwnerID != "" && affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// ListByOwner returns all notes for an owner.
func (s *Store) ListByOwner(ctx context.Context, ownerID string) ([]*Note, error) {
	var rows []noteRow
	_, err := s.readTable(ctx).Select(&rows,
		z.Where(z.Eq("owner_id", ownerID)),
		z.OrderBy("updated_at DESC"),
	)
	if err != nil {
		return nil, err
	}
	return rowsToNotes(rows)
}

// Search finds notes matching a query (title or content).
func (s *Store) Search(ctx context.Context, ownerID, query string) ([]*Note, error) {
	like := "%" + query + "%"
	var rows []noteRow
	_, err := s.readTable(ctx).Select(&rows,
		z.Where(
			z.Eq("owner_id", ownerID),
			z.Or(
				z.Like("title", like),
				z.Like("content", like),
				z.Like("tags", like),
			),
		),
		z.OrderBy("updated_at DESC"),
	)
	if err != nil {
		return nil, err
	}
	return rowsToNotes(rows)
}
