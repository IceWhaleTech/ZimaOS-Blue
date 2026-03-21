package contextpack

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const LegacyAnnotationDBFilename = "contextpacks.db"

var sqliteArtifactSuffixes = []string{"", "-wal", "-shm"}

// LegacyAnnotationMigrationResult describes a one-time import from the legacy
// standalone context annotation database into the shared blue.db store.
type LegacyAnnotationMigrationResult struct {
	SourcePath   string
	ArchivedPath string
	RowsImported int
}

// LegacyAnnotationDBPath returns the legacy standalone context annotation DB.
func LegacyAnnotationDBPath(dataDir string) string {
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		return LegacyAnnotationDBFilename
	}
	return filepath.Join(dataDir, LegacyAnnotationDBFilename)
}

// MigrateLegacyAnnotations imports rows from the legacy contextpacks.db file
// into the shared SQLite database and archives the standalone file afterward.
func MigrateLegacyAnnotations(ctx context.Context, db *sql.DB, dataDir string) (*LegacyAnnotationMigrationResult, error) {
	if db == nil || strings.TrimSpace(dataDir) == "" {
		return nil, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if _, err := NewAnnotationStoreWithDB(db); err != nil {
		return nil, fmt.Errorf("prepare shared context annotation schema: %w", err)
	}

	legacyPath := LegacyAnnotationDBPath(dataDir)
	anns, exists, err := readLegacyAnnotations(legacyPath)
	if err != nil {
		return nil, fmt.Errorf("read legacy context annotations %s: %w", legacyPath, err)
	}
	if !exists {
		return nil, nil
	}

	rowsImported, err := importLegacyAnnotations(ctx, db, anns)
	if err != nil {
		return nil, fmt.Errorf("import legacy context annotations from %s: %w", legacyPath, err)
	}

	result := &LegacyAnnotationMigrationResult{
		SourcePath:   legacyPath,
		RowsImported: rowsImported,
	}
	if archivedPath, err := archiveLegacyAnnotationDB(legacyPath); err == nil {
		result.ArchivedPath = archivedPath
	}
	return result, nil
}

func readLegacyAnnotations(path string) ([]Annotation, bool, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, true, err
	}
	defer db.Close()

	var tableExists int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='context_pack_annotations'`).Scan(&tableExists); err != nil {
		return nil, true, err
	}
	if tableExists == 0 {
		return nil, true, nil
	}

	rows, err := db.Query(`SELECT tenant_id, user_id, entry_id, language, version, file, note, updated_at FROM context_pack_annotations ORDER BY updated_at DESC`)
	if err != nil {
		return nil, true, err
	}
	defer rows.Close()

	out := make([]Annotation, 0, 16)
	for rows.Next() {
		var ann Annotation
		var updated string
		if err := rows.Scan(&ann.TenantID, &ann.UserID, &ann.EntryID, &ann.Language, &ann.Version, &ann.File, &ann.Note, &updated); err != nil {
			return nil, true, err
		}
		ann.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
		out = append(out, ann)
	}
	return out, true, rows.Err()
}

func importLegacyAnnotations(ctx context.Context, db *sql.DB, anns []Annotation) (int, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	inserted := 0
	for _, ann := range anns {
		ann = normalizeAnnotation(ann)
		if ann.EntryID == "" || ann.Note == "" {
			continue
		}
		if ann.UpdatedAt.IsZero() {
			ann.UpdatedAt = time.Now().UTC()
		}
		res, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO context_pack_annotations (tenant_id, user_id, entry_id, language, version, file, note, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			ann.TenantID, ann.UserID, ann.EntryID, ann.Language, ann.Version, ann.File, ann.Note, ann.UpdatedAt.Format(time.RFC3339Nano),
		)
		if err != nil {
			return 0, err
		}
		if n, err := res.RowsAffected(); err == nil {
			inserted += int(n)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return inserted, nil
}

func archiveLegacyAnnotationDB(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("legacy context annotation path is empty")
	}
	archivedPath := path + ".migrated"
	if _, err := os.Stat(archivedPath); err == nil {
		archivedPath = fmt.Sprintf("%s.%s", archivedPath, time.Now().UTC().Format("20060102T150405.000000000"))
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := moveSQLiteArtifacts(path, archivedPath); err != nil {
		return "", err
	}
	return archivedPath, nil
}

func moveSQLiteArtifacts(srcBase, dstBase string) error {
	moved := make([]string, 0, len(sqliteArtifactSuffixes))
	for _, suffix := range sqliteArtifactSuffixes {
		src := srcBase + suffix
		dst := dstBase + suffix
		if _, err := os.Stat(src); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if err := os.Rename(src, dst); err != nil {
			for i := len(moved) - 1; i >= 0; i-- {
				_ = os.Rename(dstBase+moved[i], srcBase+moved[i])
			}
			return err
		}
		moved = append(moved, suffix)
	}
	if len(moved) == 0 {
		return fmt.Errorf("no sqlite files found at %s", srcBase)
	}
	return nil
}
