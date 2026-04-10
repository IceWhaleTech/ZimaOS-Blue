package database_test

import (
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
	_ "github.com/mattn/go-sqlite3"
)

func TestIsSQLiteCorruptionError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "malformed", err: os.ErrInvalid, want: false},
		{name: "disk image malformed", err: errString("database disk image is malformed"), want: true},
		{name: "fts5 corruption", err: errString("fts5: corruption found in text index"), want: true},
		{name: "fts5 malformed inverted index", err: errString("malformed inverted index for FTS5 table main.skills_fts"), want: true},
		{name: "file is not database", err: errString("file is not a database"), want: true},
		{name: "sqlite corrupt", err: errString("SQLITE_CORRUPT: page checksum mismatch"), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := database.IsSQLiteCorruptionError(tt.err)
			if got != tt.want {
				t.Fatalf("IsSQLiteCorruptionError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOpenSQLiteSimpleWrapsCorruptionError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "corrupt.db")
	if err := os.WriteFile(path, []byte("not-a-sqlite-database"), 0o600); err != nil {
		t.Fatalf("write corrupt db: %v", err)
	}

	db, err := database.OpenSQLiteSimple(path)
	if db != nil {
		_ = db.Close()
		t.Fatal("expected open to fail for corrupt sqlite file")
	}
	if err == nil {
		t.Fatal("expected corruption error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "appears corrupted") {
		t.Fatalf("expected wrapped corruption error, got %v", err)
	}
}

func TestOpenSQLiteWithRecoveryAndRecreateRotatesCorruptDatabase(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "recreate.db")
	if err := os.WriteFile(path, []byte("not-a-sqlite-database"), 0o600); err != nil {
		t.Fatalf("write corrupt db: %v", err)
	}

	db, err := database.OpenSQLiteWithRecoveryAndRecreate(path, path, func(db *sql.DB) error {
		_, err := db.Exec(`CREATE TABLE IF NOT EXISTS entries (id INTEGER PRIMARY KEY, value TEXT)`)
		return err
	})
	if err != nil {
		t.Fatalf("expected recreate helper to recover by rebuilding db, got %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`INSERT INTO entries(value) VALUES('ok')`); err != nil {
		t.Fatalf("fresh db should accept writes: %v", err)
	}
	if matches, err := filepath.Glob(path + ".bak.*"); err != nil {
		t.Fatalf("glob recreated backup: %v", err)
	} else if len(matches) != 1 {
		t.Fatalf("expected one rotated .bak file, got %v", matches)
	}
}

func TestOpenSQLiteSimpleRepairsRecoverableCorruption(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "recoverable.db")
	writeSQLiteEntry(t, path, "hello")
	corruptSQLiteBytes(t, path, 8192, []byte("garbagegarbagegarbagegarbage"))

	db, err := database.OpenSQLiteSimple(path)
	if err != nil {
		t.Fatalf("expected recoverable db to open after repair, got %v", err)
	}
	defer db.Close()

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM entries").Scan(&count); err != nil {
		t.Fatalf("failed to count repaired rows: %v", err)
	}
	if count == 0 {
		t.Fatal("expected repaired db to retain at least one row")
	}
	if err := database.CheckDatabaseIntegrity(path); err != nil {
		t.Fatalf("repaired db failed integrity check: %v", err)
	}
	if matches, err := filepath.Glob(path + ".bak.*"); err != nil {
		t.Fatalf("glob repair backup: %v", err)
	} else if len(matches) != 0 {
		t.Fatalf("expected successful repair to remove temporary .bak files, got %v", matches)
	}
}

func TestOpenSQLiteSimpleRepairsRecoverableCorruptionFallsBackWithoutIgnoreFreelist(t *testing.T) {
	realSQLite, err := exec.LookPath("sqlite3")
	if err != nil {
		t.Skipf("sqlite3 CLI unavailable: %v", err)
	}

	wrapperDir := t.TempDir()
	wrapperPath := filepath.Join(wrapperDir, "sqlite3")
	wrapper := `#!/bin/sh
if [ "$#" -ge 3 ] && [ "$1" = "-batch" ] && [ "$3" = ".recover --ignore-freelist" ]; then
  echo "sql error: no such table: sqlite_dbpage (1)" >&2
  exit 1
fi
exec "$REAL_SQLITE3" "$@"
`
	if err := os.WriteFile(wrapperPath, []byte(wrapper), 0o755); err != nil {
		t.Fatalf("write sqlite3 wrapper: %v", err)
	}
	t.Setenv("REAL_SQLITE3", realSQLite)
	t.Setenv("PATH", wrapperDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	dir := t.TempDir()
	path := filepath.Join(dir, "recoverable-fallback.db")
	writeSQLiteEntry(t, path, "hello")
	corruptSQLiteBytes(t, path, 8192, []byte("garbagegarbagegarbagegarbage"))

	db, err := database.OpenSQLiteSimple(path)
	if err != nil {
		t.Fatalf("expected recoverable db to open after plain .recover fallback, got %v", err)
	}
	defer db.Close()

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM entries").Scan(&count); err != nil {
		t.Fatalf("failed to count repaired rows after fallback: %v", err)
	}
	if count == 0 {
		t.Fatal("expected fallback repair to retain at least one row")
	}
}

func TestOpenSQLiteSimpleRepairsRecoverableCorruptionFallsBackWhenIgnoreFreelistDropsSchema(t *testing.T) {
	realSQLite, err := exec.LookPath("sqlite3")
	if err != nil {
		t.Skipf("sqlite3 CLI unavailable: %v", err)
	}

	wrapperDir := t.TempDir()
	wrapperPath := filepath.Join(wrapperDir, "sqlite3")
	wrapper := `#!/bin/sh
if [ "$#" -ge 3 ] && [ "$1" = "-batch" ] && [ "$3" = ".recover --ignore-freelist" ]; then
  printf 'BEGIN;\nCOMMIT;\n'
  exit 0
fi
exec "$REAL_SQLITE3" "$@"
`
	if err := os.WriteFile(wrapperPath, []byte(wrapper), 0o755); err != nil {
		t.Fatalf("write sqlite3 wrapper: %v", err)
	}
	t.Setenv("REAL_SQLITE3", realSQLite)
	t.Setenv("PATH", wrapperDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	dir := t.TempDir()
	path := filepath.Join(dir, "recoverable-empty-schema-fallback.db")
	writeSQLiteEntry(t, path, "hello")
	corruptSQLiteBytes(t, path, 8192, []byte("garbagegarbagegarbagegarbage"))

	db, err := database.OpenSQLiteSimple(path)
	if err != nil {
		t.Fatalf("expected recoverable db to open after schema-loss fallback, got %v", err)
	}
	defer db.Close()

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM entries").Scan(&count); err != nil {
		t.Fatalf("failed to count repaired rows after schema-loss fallback: %v", err)
	}
	if count == 0 {
		t.Fatal("expected schema-loss fallback repair to retain at least one row")
	}
}

func TestRotateCorruptSQLiteDatabaseMovesPrimaryAndAuxFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "blue.db")
	writeSQLiteEntry(t, path, "hello")
	if err := os.WriteFile(path+"-wal", []byte("wal"), 0o600); err != nil {
		t.Fatalf("write wal sidecar: %v", err)
	}
	if err := os.WriteFile(path+"-shm", []byte("shm"), 0o600); err != nil {
		t.Fatalf("write shm sidecar: %v", err)
	}

	backupPath, err := database.RotateCorruptSQLiteDatabase(path)
	if err != nil {
		t.Fatalf("RotateCorruptSQLiteDatabase() error = %v", err)
	}
	if !strings.Contains(backupPath, ".bak.") {
		t.Fatalf("backup path = %q, want .bak timestamp suffix", backupPath)
	}

	for _, suffix := range []string{"", "-wal", "-shm"} {
		if _, err := os.Stat(path + suffix); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be rotated away, stat err = %v", path+suffix, err)
		}
		if _, err := os.Stat(backupPath + suffix); err != nil {
			t.Fatalf("expected rotated artifact %s to exist: %v", backupPath+suffix, err)
		}
	}
}

func TestRepairKnownFTSIndexesRebuildsSkillStoreFTS(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "skills.db")

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer db.Close()

	if !sqliteHasFTS5(t, db) {
		t.Skip("sqlite build does not include FTS5")
	}

	statements := []string{
		`CREATE TABLE skills (
			id TEXT PRIMARY KEY,
			name TEXT,
			summary TEXT,
			description TEXT,
			author TEXT,
			category TEXT,
			tags TEXT,
			readme TEXT
		)`,
		`INSERT INTO skills(id, name, summary, description, author, category, tags, readme)
		 VALUES ('skill-1', 'Alpha Skill', 'summary', 'description', 'author', 'category', '["tag"]', 'readme')`,
		`CREATE VIRTUAL TABLE skills_fts USING fts5(
			id, name, summary, description, author, category, tags, readme,
			content='skills', content_rowid='rowid'
		)`,
		`CREATE TRIGGER skills_ai AFTER INSERT ON skills BEGIN
			INSERT INTO skills_fts(rowid, id, name, summary, description, author, category, tags, readme)
			VALUES (new.rowid, new.id, new.name, new.summary, new.description, new.author, new.category, new.tags, new.readme);
		END`,
		`CREATE TRIGGER skills_ad AFTER DELETE ON skills BEGIN
			INSERT INTO skills_fts(skills_fts, rowid, id, name, summary, description, author, category, tags, readme)
			VALUES ('delete', old.rowid, old.id, old.name, old.summary, old.description, old.author, old.category, old.tags, old.readme);
		END`,
		`CREATE TRIGGER skills_au AFTER UPDATE ON skills BEGIN
			INSERT INTO skills_fts(skills_fts, rowid, id, name, summary, description, author, category, tags, readme)
			VALUES ('delete', old.rowid, old.id, old.name, old.summary, old.description, old.author, old.category, old.tags, old.readme);
			INSERT INTO skills_fts(rowid, id, name, summary, description, author, category, tags, readme)
			VALUES (new.rowid, new.id, new.name, new.summary, new.description, new.author, new.category, new.tags, new.readme);
		END`,
	}
	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("setup statement failed: %v", err)
		}
	}

	// External-content FTS tables can expose source rows before the index is rebuilt;
	// assert on searchability instead of raw row count.
	assertSQLiteCount(t, db, `SELECT COUNT(*) FROM skills_fts WHERE skills_fts MATCH 'alpha'`, 0)

	repaired, err := database.RepairKnownFTSIndexes(path)
	if err != nil {
		t.Fatalf("RepairKnownFTSIndexes() error = %v", err)
	}
	if !containsString(repaired, "skills_fts") {
		t.Fatalf("expected repaired tables to include skills_fts, got %v", repaired)
	}

	assertSQLiteCount(t, db, `SELECT COUNT(*) FROM skills_fts`, 1)
	assertSQLiteCount(t, db, `SELECT COUNT(*) FROM skills_fts WHERE skills_fts MATCH 'alpha'`, 1)
}

func TestRepairKnownFTSIndexesRepairsCorruptedSkillStoreFTS(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "skills-corrupt.db")

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer db.Close()

	if !sqliteHasFTS5(t, db) {
		t.Skip("sqlite build does not include FTS5")
	}

	setup := []string{
		`CREATE TABLE skills (
			id TEXT PRIMARY KEY,
			name TEXT,
			summary TEXT,
			description TEXT,
			author TEXT,
			category TEXT,
			tags TEXT,
			readme TEXT
		)`,
		`WITH RECURSIVE cnt(x) AS (
			SELECT 1
			UNION ALL
			SELECT x + 1 FROM cnt WHERE x < 500
		)
		INSERT INTO skills(id, name, summary, description, author, category, tags, readme)
		SELECT
			printf('skill-%03d', x),
			printf('Alpha Skill %03d', x),
			'summary',
			'description',
			'author',
			'category',
			'["tag"]',
			'readme'
		FROM cnt`,
		`CREATE VIRTUAL TABLE skills_fts USING fts5(
			id, name, summary, description, author, category, tags, readme,
			content='skills', content_rowid='rowid'
		)`,
		`CREATE TRIGGER skills_ai AFTER INSERT ON skills BEGIN
			INSERT INTO skills_fts(rowid, id, name, summary, description, author, category, tags, readme)
			VALUES (new.rowid, new.id, new.name, new.summary, new.description, new.author, new.category, new.tags, new.readme);
		END`,
		`CREATE TRIGGER skills_ad AFTER DELETE ON skills BEGIN
			INSERT INTO skills_fts(skills_fts, rowid, id, name, summary, description, author, category, tags, readme)
			VALUES ('delete', old.rowid, old.id, old.name, old.summary, old.description, old.author, old.category, old.tags, old.readme);
		END`,
		`CREATE TRIGGER skills_au AFTER UPDATE ON skills BEGIN
			INSERT INTO skills_fts(skills_fts, rowid, id, name, summary, description, author, category, tags, readme)
			VALUES ('delete', old.rowid, old.id, old.name, old.summary, old.description, old.author, old.category, old.tags, old.readme);
			INSERT INTO skills_fts(rowid, id, name, summary, description, author, category, tags, readme)
			VALUES (new.rowid, new.id, new.name, new.summary, new.description, new.author, new.category, new.tags, new.readme);
		END`,
		`INSERT INTO skills_fts(skills_fts) VALUES('rebuild')`,
	}
	for _, stmt := range setup {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("setup statement failed: %v", err)
		}
	}

	var shadowRowID int64
	if err := db.QueryRow(`SELECT id FROM skills_fts_data WHERE id > 100 ORDER BY id LIMIT 1`).Scan(&shadowRowID); err != nil {
		t.Fatalf("select corruptible FTS shadow row: %v", err)
	}
	if _, err := db.Exec(`UPDATE skills_fts_data SET block = X'00' WHERE id = ?`, shadowRowID); err != nil {
		t.Fatalf("corrupt FTS shadow row: %v", err)
	}

	if err := database.QuickCheckDatabase(path); err == nil {
		t.Fatal("expected quick check to fail after corrupting FTS shadow data")
	}

	repaired, err := database.RepairKnownFTSIndexes(path)
	if err != nil {
		t.Fatalf("RepairKnownFTSIndexes() error = %v", err)
	}
	if !containsString(repaired, "skills_fts") {
		t.Fatalf("expected repaired tables to include skills_fts, got %v", repaired)
	}

	if err := database.QuickCheckDatabase(path); err != nil {
		t.Fatalf("expected repaired db to pass quick check, got %v", err)
	}
	assertSQLiteCount(t, db, `SELECT COUNT(*) FROM skills_fts WHERE skills_fts MATCH 'alpha'`, 500)
}

func TestRepairKnownFTSIndexesRebuildsSkillMarketFTS(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "skillmarket.db")

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer db.Close()

	if !sqliteHasFTS5(t, db) {
		t.Skip("sqlite build does not include FTS5")
	}

	statements := []string{
		`CREATE TABLE skills (
			id TEXT PRIMARY KEY,
			name TEXT,
			description TEXT,
			author TEXT,
			category TEXT,
			tags TEXT,
			skill_content TEXT
		)`,
		`INSERT INTO skills(id, name, description, author, category, tags, skill_content)
		 VALUES ('market-1', 'Weather Wizard', 'desc', 'author', 'utility', '["weather"]', 'weather automation helper')`,
		`CREATE VIRTUAL TABLE skillmarket_fts USING fts5(
			id UNINDEXED,
			name,
			description,
			author,
			category,
			tags,
			skill_content,
			content='skills',
			content_rowid='rowid'
		)`,
		`CREATE TRIGGER skillmarket_ai AFTER INSERT ON skills BEGIN
			INSERT INTO skillmarket_fts(rowid, id, name, description, author, category, tags, skill_content)
			VALUES (new.rowid, new.id, new.name, new.description, new.author, new.category, new.tags, new.skill_content);
		END`,
		`CREATE TRIGGER skillmarket_ad AFTER DELETE ON skills BEGIN
			INSERT INTO skillmarket_fts(skillmarket_fts, rowid, id, name, description, author, category, tags, skill_content)
			VALUES ('delete', old.rowid, old.id, old.name, old.description, old.author, old.category, old.tags, old.skill_content);
		END`,
		`CREATE TRIGGER skillmarket_au AFTER UPDATE ON skills BEGIN
			INSERT INTO skillmarket_fts(skillmarket_fts, rowid, id, name, description, author, category, tags, skill_content)
			VALUES ('delete', old.rowid, old.id, old.name, old.description, old.author, old.category, old.tags, old.skill_content);
			INSERT INTO skillmarket_fts(rowid, id, name, description, author, category, tags, skill_content)
			VALUES (new.rowid, new.id, new.name, new.description, new.author, new.category, new.tags, new.skill_content);
		END`,
	}
	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("setup statement failed: %v", err)
		}
	}

	// External-content FTS tables can expose source rows before the index is rebuilt;
	// assert on searchability instead of raw row count.
	assertSQLiteCount(t, db, `SELECT COUNT(*) FROM skillmarket_fts WHERE skillmarket_fts MATCH 'weather'`, 0)

	repaired, err := database.RepairKnownFTSIndexes(path)
	if err != nil {
		t.Fatalf("RepairKnownFTSIndexes() error = %v", err)
	}
	if !containsString(repaired, "skillmarket_fts") {
		t.Fatalf("expected repaired tables to include skillmarket_fts, got %v", repaired)
	}

	assertSQLiteCount(t, db, `SELECT COUNT(*) FROM skillmarket_fts`, 1)
	assertSQLiteCount(t, db, `SELECT COUNT(*) FROM skillmarket_fts WHERE skillmarket_fts MATCH 'weather'`, 1)
}

func TestRepairKnownFTSIndexesNoopsWhenNoKnownFTSObjectsExist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plain.db")
	writeSQLiteEntry(t, path, "plain")

	repaired, err := database.RepairKnownFTSIndexes(path)
	if err != nil {
		t.Fatalf("RepairKnownFTSIndexes() error = %v", err)
	}
	if len(repaired) != 0 {
		t.Fatalf("expected no repaired FTS tables, got %v", repaired)
	}
}

func writeSQLiteEntry(t *testing.T, path, value string) {
	t.Helper()

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open sqlite db %s: %v", path, err)
	}
	defer db.Close()

	if _, err := db.Exec("DROP TABLE IF EXISTS entries"); err != nil {
		t.Fatalf("drop entries table in %s: %v", path, err)
	}
	if _, err := db.Exec("CREATE TABLE entries (id INTEGER PRIMARY KEY, value TEXT NOT NULL)"); err != nil {
		t.Fatalf("create entries table in %s: %v", path, err)
	}

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin insert transaction for %s: %v", path, err)
	}
	stmt, err := tx.Prepare("INSERT INTO entries(id, value) VALUES (?, ?)")
	if err != nil {
		t.Fatalf("prepare insert statement for %s: %v", path, err)
	}
	defer stmt.Close()

	for i := 1; i <= 2000; i++ {
		entryValue := value
		if i > 1 {
			entryValue = strings.Repeat("x", 200)
		}
		if _, err := stmt.Exec(i, entryValue); err != nil {
			_ = tx.Rollback()
			t.Fatalf("insert entry %d into %s: %v", i, path, err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit inserts for %s: %v", path, err)
	}
}

func corruptSQLiteBytes(t *testing.T, path string, offset int64, payload []byte) {
	t.Helper()

	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open sqlite db for corruption %s: %v", path, err)
	}
	defer f.Close()

	if _, err := f.WriteAt(payload, offset); err != nil {
		t.Fatalf("corrupt sqlite db %s: %v", path, err)
	}
}

func sqliteHasFTS5(t *testing.T, db *sql.DB) bool {
	t.Helper()

	rows, err := db.Query(`SELECT name FROM pragma_module_list WHERE name = 'fts5'`)
	if err == nil {
		defer rows.Close()
		return rows.Next()
	}

	if _, err := db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS temp.test_fts5_probe USING fts5(content)`); err != nil {
		return false
	}
	_, _ = db.Exec(`DROP TABLE IF EXISTS temp.test_fts5_probe`)
	return true
}

func assertSQLiteCount(t *testing.T, db *sql.DB, query string, want int) {
	t.Helper()

	var got int
	if err := db.QueryRow(query).Scan(&got); err != nil {
		t.Fatalf("query %q failed: %v", query, err)
	}
	if got != want {
		t.Fatalf("query %q count = %d, want %d", query, got, want)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestStartupIntegritySessionTracksCleanAndDirtyRuns(t *testing.T) {
	tmpDir := t.TempDir()
	database.SetStartupQuickCheckEnabled(true)
	t.Cleanup(func() {
		database.SetStartupQuickCheckEnabled(true)
	})

	previousClean, err := database.BeginStartupIntegritySession(tmpDir)
	if err != nil {
		t.Fatalf("BeginStartupIntegritySession() error = %v", err)
	}
	if previousClean {
		t.Fatal("expected missing marker to be treated as not clean")
	}

	if err := database.MarkStartupIntegrityClean(tmpDir); err != nil {
		t.Fatalf("MarkStartupIntegrityClean() error = %v", err)
	}

	previousClean, err = database.BeginStartupIntegritySession(tmpDir)
	if err != nil {
		t.Fatalf("BeginStartupIntegritySession() after clean error = %v", err)
	}
	if !previousClean {
		t.Fatal("expected previous run to be treated as clean after clean marker")
	}
}

type errString string

func (e errString) Error() string { return string(e) }
