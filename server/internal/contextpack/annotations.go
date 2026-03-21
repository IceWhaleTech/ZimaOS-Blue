package contextpack

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	dbutil "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
	_ "github.com/mattn/go-sqlite3"
)

const annotationSchema = `
CREATE TABLE IF NOT EXISTS context_pack_annotations (
	tenant_id  TEXT NOT NULL DEFAULT '',
	user_id    TEXT NOT NULL DEFAULT '',
	entry_id   TEXT NOT NULL,
	language   TEXT NOT NULL DEFAULT '',
	version    TEXT NOT NULL DEFAULT '',
	file       TEXT NOT NULL DEFAULT '',
	note       TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (tenant_id, user_id, entry_id, language, version, file)
);

CREATE INDEX IF NOT EXISTS idx_context_pack_annotations_entry
	ON context_pack_annotations(entry_id, updated_at DESC);
`

type AnnotationStore struct {
	db      *sql.DB
	ownsDB  bool
	closeMu sync.Mutex
	closed  bool
}

func NewAnnotationStore(path string) (*AnnotationStore, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("annotation db path is empty")
	}
	db, err := dbutil.OpenSQLiteWithRecoveryAndRecreate(path, path, func(db *sql.DB) error {
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
		if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
			return err
		}
		if _, err := db.Exec(`PRAGMA busy_timeout=5000`); err != nil {
			return err
		}
		if _, err := db.Exec(`PRAGMA synchronous=FULL`); err != nil {
			return err
		}
		if _, err := db.Exec(`PRAGMA wal_autocheckpoint=1000`); err != nil {
			return err
		}
		if _, err := db.Exec(annotationSchema); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &AnnotationStore{db: db, ownsDB: true}, nil
}

func NewAnnotationStoreWithDB(db *sql.DB) (*AnnotationStore, error) {
	if db == nil {
		return nil, fmt.Errorf("annotation db is nil")
	}
	if _, err := db.Exec(annotationSchema); err != nil {
		return nil, err
	}
	return &AnnotationStore{db: db}, nil
}

func (s *AnnotationStore) Close() error {
	if s == nil || !s.ownsDB || s.db == nil {
		return nil
	}
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	return s.db.Close()
}

func (s *AnnotationStore) Upsert(ctx context.Context, ann Annotation) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("annotation store is not initialized")
	}
	ann = normalizeAnnotation(ann)
	if ann.EntryID == "" {
		return fmt.Errorf("entry_id is required")
	}
	if ann.Note == "" {
		return fmt.Errorf("note is required")
	}
	if ann.UpdatedAt.IsZero() {
		ann.UpdatedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO context_pack_annotations (tenant_id, user_id, entry_id, language, version, file, note, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(tenant_id, user_id, entry_id, language, version, file)
		 DO UPDATE SET note = excluded.note, updated_at = excluded.updated_at`,
		ann.TenantID, ann.UserID, ann.EntryID, ann.Language, ann.Version, ann.File, ann.Note, ann.UpdatedAt.Format(time.RFC3339Nano),
	)
	return err
}

func (s *AnnotationStore) Delete(ctx context.Context, filter AnnotationFilter) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("annotation store is not initialized")
	}
	filter = normalizeFilter(filter)
	if filter.EntryID == "" {
		return fmt.Errorf("entry_id is required")
	}
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM context_pack_annotations
		 WHERE tenant_id = ? AND user_id = ? AND entry_id = ? AND language = ? AND version = ? AND file = ?`,
		filter.TenantID, filter.UserID, filter.EntryID, filter.Language, filter.Version, filter.File,
	)
	return err
}

func (s *AnnotationStore) List(ctx context.Context, filter AnnotationFilter) ([]Annotation, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("annotation store is not initialized")
	}
	filter = normalizeFilter(filter)
	query := `SELECT tenant_id, user_id, entry_id, language, version, file, note, updated_at FROM context_pack_annotations WHERE 1=1`
	args := make([]interface{}, 0, 6)
	if filter.TenantID != "" {
		query += ` AND tenant_id = ?`
		args = append(args, filter.TenantID)
	}
	if filter.UserID != "" {
		query += ` AND user_id = ?`
		args = append(args, filter.UserID)
	}
	if filter.EntryID != "" {
		query += ` AND entry_id = ?`
		args = append(args, filter.EntryID)
	}
	if filter.Language != "" {
		query += ` AND language = ?`
		args = append(args, filter.Language)
	}
	if filter.Version != "" {
		query += ` AND version = ?`
		args = append(args, filter.Version)
	}
	if filter.File != "" {
		query += ` AND file = ?`
		args = append(args, filter.File)
	}
	query += ` ORDER BY updated_at DESC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Annotation, 0, 8)
	for rows.Next() {
		var ann Annotation
		var updated string
		if err := rows.Scan(&ann.TenantID, &ann.UserID, &ann.EntryID, &ann.Language, &ann.Version, &ann.File, &ann.Note, &updated); err != nil {
			return nil, err
		}
		ann.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
		out = append(out, ann)
	}
	return out, rows.Err()
}

func (s *AnnotationStore) Applicable(ctx context.Context, filter AnnotationFilter) ([]Annotation, error) {
	filter = normalizeFilter(filter)
	anns, err := s.List(ctx, AnnotationFilter{
		TenantID: filter.TenantID,
		UserID:   filter.UserID,
		EntryID:  filter.EntryID,
		Language: filter.Language,
		Version:  filter.Version,
	})
	if err != nil {
		return nil, err
	}
	out := make([]Annotation, 0, len(anns))
	for _, ann := range anns {
		if ann.File == "" || ann.File == filter.File {
			out = append(out, ann)
		}
	}
	return out, nil
}

func normalizeAnnotation(ann Annotation) Annotation {
	ann.TenantID = normalizeScopeValue(ann.TenantID)
	ann.UserID = normalizeScopeValue(ann.UserID)
	ann.EntryID = strings.TrimSpace(ann.EntryID)
	ann.Language = strings.TrimSpace(ann.Language)
	ann.Version = strings.TrimSpace(ann.Version)
	ann.File = filepathish(ann.File)
	ann.Note = strings.TrimSpace(ann.Note)
	return ann
}

func normalizeFilter(filter AnnotationFilter) AnnotationFilter {
	filter.TenantID = normalizeScopeValue(filter.TenantID)
	filter.UserID = normalizeScopeValue(filter.UserID)
	filter.EntryID = strings.TrimSpace(filter.EntryID)
	filter.Language = strings.TrimSpace(filter.Language)
	filter.Version = strings.TrimSpace(filter.Version)
	filter.File = filepathish(filter.File)
	return filter
}

func normalizeScopeValue(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "local"
	}
	return v
}

func filepathish(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "./")
	v = strings.ReplaceAll(v, "\\", "/")
	return v
}
