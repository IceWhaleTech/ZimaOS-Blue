package contextpack

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	dbutil "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
	z "github.com/IceWhaleTech/zorm"
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
	readDB  *sql.DB
	ownsDB  bool
	closeMu sync.Mutex
	closed  bool
}

func openAnnotationReaderDB(path string) (*sql.DB, error) {
	path = strings.TrimSpace(path)
	if path == "" || path == ":memory:" {
		return nil, nil
	}
	dsn := fmt.Sprintf("file:%s?mode=ro", path)
	db, err := dbutil.OpenSQLiteWithRecovery(dsn, path, func(db *sql.DB) error {
		db.SetMaxOpenConns(4)
		db.SetMaxIdleConns(2)
		if _, err := db.Exec(`PRAGMA busy_timeout=5000`); err != nil {
			return fmt.Errorf("set annotation reader busy timeout: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return db, nil
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
	readDB, readErr := openAnnotationReaderDB(path)
	if readErr != nil || readDB == nil {
		readDB = db
	}
	return &AnnotationStore{db: db, readDB: readDB, ownsDB: true}, nil
}

func NewAnnotationStoreWithDB(db *sql.DB) (*AnnotationStore, error) {
	return NewAnnotationStoreWithReadDB(db, db)
}

func NewAnnotationStoreWithReadDB(writeDB, readDB *sql.DB) (*AnnotationStore, error) {
	if writeDB == nil {
		return nil, fmt.Errorf("annotation db is nil")
	}
	if readDB == nil {
		readDB = writeDB
	}
	if _, err := writeDB.Exec(annotationSchema); err != nil {
		return nil, err
	}
	return &AnnotationStore{db: writeDB, readDB: readDB}, nil
}

func (s *AnnotationStore) reader() *sql.DB {
	if s != nil && s.readDB != nil {
		return s.readDB
	}
	if s == nil {
		return nil
	}
	return s.db
}

func (s *AnnotationStore) table(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.db, "context_pack_annotations")
}

func (s *AnnotationStore) readTable(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.reader(), "context_pack_annotations")
}

type annotationRow struct {
	TenantID  string `json:"tenant_id" zorm:"tenant_id"`
	UserID    string `json:"user_id" zorm:"user_id"`
	EntryID   string `json:"entry_id" zorm:"entry_id"`
	Language  string `json:"language" zorm:"language"`
	Version   string `json:"version" zorm:"version"`
	File      string `json:"file" zorm:"file"`
	Note      string `json:"note" zorm:"note"`
	UpdatedAt string `json:"updated_at" zorm:"updated_at"`
}

func annotationValues(ann Annotation) z.V {
	return z.V{
		"tenant_id":  ann.TenantID,
		"user_id":    ann.UserID,
		"entry_id":   ann.EntryID,
		"language":   ann.Language,
		"version":    ann.Version,
		"file":       ann.File,
		"note":       ann.Note,
		"updated_at": ann.UpdatedAt.Format(time.RFC3339Nano),
	}
}

func parseAnnotationTime(raw string) time.Time {
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02T15:04:05.999999999-07:00",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func rowToAnnotation(row annotationRow) Annotation {
	return Annotation{
		TenantID:  row.TenantID,
		UserID:    row.UserID,
		EntryID:   row.EntryID,
		Language:  row.Language,
		Version:   row.Version,
		File:      row.File,
		Note:      row.Note,
		UpdatedAt: parseAnnotationTime(row.UpdatedAt),
	}
}

func rowsToAnnotations(rows []annotationRow) []Annotation {
	out := make([]Annotation, 0, len(rows))
	for i := range rows {
		out = append(out, rowToAnnotation(rows[i]))
	}
	return out
}

func (s *AnnotationStore) Close() error {
	if s == nil || !s.ownsDB || s.db == nil {
		return nil
	}
	s.closeMu.Lock()
	if s.closed {
		s.closeMu.Unlock()
		return nil
	}
	s.closed = true
	db := s.db
	readDB := s.readDB
	s.db = nil
	s.readDB = nil
	s.closeMu.Unlock()

	var firstErr error
	if readDB != nil && readDB != db {
		if err := readDB.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if db != nil {
		if err := db.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
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
	_, err := s.table(ctx).Insert(
		annotationValues(ann),
		z.OnConflictDoUpdateSet(
			[]string{"tenant_id", "user_id", "entry_id", "language", "version", "file"},
			[]string{"note", "updated_at"},
		),
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
	_, err := s.table(ctx).Delete(
		z.Where(
			z.Eq("tenant_id", filter.TenantID),
			z.Eq("user_id", filter.UserID),
			z.Eq("entry_id", filter.EntryID),
			z.Eq("language", filter.Language),
			z.Eq("version", filter.Version),
			z.Eq("file", filter.File),
		),
	)
	return err
}

func (s *AnnotationStore) List(ctx context.Context, filter AnnotationFilter) ([]Annotation, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("annotation store is not initialized")
	}
	filter = normalizeFilter(filter)
	conds := make([]interface{}, 0, 6)
	if filter.TenantID != "" {
		conds = append(conds, z.Eq("tenant_id", filter.TenantID))
	}
	if filter.UserID != "" {
		conds = append(conds, z.Eq("user_id", filter.UserID))
	}
	if filter.EntryID != "" {
		conds = append(conds, z.Eq("entry_id", filter.EntryID))
	}
	if filter.Language != "" {
		conds = append(conds, z.Eq("language", filter.Language))
	}
	if filter.Version != "" {
		conds = append(conds, z.Eq("version", filter.Version))
	}
	if filter.File != "" {
		conds = append(conds, z.Eq("file", filter.File))
	}
	opts := []z.ZormItem{z.OrderBy("updated_at DESC")}
	if len(conds) > 0 {
		opts = append([]z.ZormItem{z.Where(conds...)}, opts...)
	}
	var rows []annotationRow
	if _, err := s.readTable(ctx).Select(&rows, opts...); err != nil {
		return nil, err
	}
	return rowsToAnnotations(rows), nil
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
