package skillmarket

import (
	"context"
	"database/sql"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func createSQLiteFixtureWithFTS(t *testing.T, dbPath, schema string) {
	t.Helper()

	if _, err := exec.LookPath("sqlite3"); err != nil {
		t.Skipf("sqlite3 CLI unavailable: %v", err)
	}

	cmd := exec.Command("sqlite3", dbPath)
	cmd.Stdin = strings.NewReader(schema)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("sqlite3 fixture setup failed: %v\n%s", err, output)
	}
}

func TestNewStoreMigratesLegacySkillstoreSchema(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "legacy-market.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer db.Close()

	legacySchema := `
	CREATE TABLE skills (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		version TEXT,
		summary TEXT,
		description TEXT,
		author TEXT,
		category TEXT,
		tags TEXT,
		source_id TEXT NOT NULL,
		source_name TEXT,
		homepage TEXT,
		download_url TEXT,
		stars INTEGER DEFAULT 0,
		downloads INTEGER DEFAULT 0,
		reviews INTEGER DEFAULT 0,
		rating REAL DEFAULT 0.0,
		versions INTEGER DEFAULT 0,
		changelog TEXT,
		readme TEXT,
		readme_hash TEXT,
		dedup_key TEXT,
		installed INTEGER DEFAULT 0,
		enabled INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		synced_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		search_content TEXT
	);
	CREATE TABLE skill_security_reports (
		id TEXT PRIMARY KEY,
		skill_version_id TEXT NOT NULL,
		skill_id TEXT NOT NULL,
		version TEXT NOT NULL,
		score INTEGER NOT NULL,
		risk_level TEXT NOT NULL,
		risks_json TEXT,
		permissions_json TEXT,
		secrets_json TEXT,
		scanner_version TEXT,
		llm_status TEXT,
		llm_verdict_json TEXT,
		created_at DATETIME,
		updated_at DATETIME
	);
	CREATE TABLE skill_sources (
		id TEXT PRIMARY KEY,
		type TEXT NOT NULL,
		base_url TEXT NOT NULL,
		auth_mode TEXT,
		enabled INTEGER DEFAULT 1,
		last_cursor TEXT,
		etag TEXT,
		rate_limit_per_minute INTEGER DEFAULT 30,
		last_success_at DATETIME,
		created_at DATETIME,
		updated_at DATETIME
	);
	`
	if _, err := db.Exec(legacySchema); err != nil {
		t.Fatalf("db.Exec(legacySchema) error = %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO skills (
			id, name, version, summary, description, author, category, tags, source_id, source_name,
			homepage, download_url, stars, downloads, search_content
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "legacy-skill", "Legacy Skill", "1.0.0", "legacy summary", "legacy description", "legacy-author",
		"development", "git,legacy", "legacy-source", "Legacy Source", "https://example.com",
		"https://example.com/archive.zip", 12, 34, "legacy skill legacy description git legacy"); err != nil {
		t.Fatalf("insert legacy skill error = %v", err)
	}

	store, err := NewStore(db)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	result, err := store.Search(context.Background(), SearchQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if result.Total < 1 {
		t.Fatalf("expected migrated skills to remain queryable, total=%d", result.Total)
	}
	if len(result.Skills) == 0 {
		t.Fatalf("expected at least one search result")
	}
	if result.Skills[0].Skill.ID != "legacy-skill" {
		t.Fatalf("expected legacy skill to be returned, got %q", result.Skills[0].Skill.ID)
	}
	if result.Skills[0].Skill.Slug != "legacy-skill" {
		t.Fatalf("expected legacy skill slug backfill, got %q", result.Skills[0].Skill.Slug)
	}
}

func TestNewStoreMigratesLegacySchemaWithoutFTSModule(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy-market-fts.db")
	createSQLiteFixtureWithFTS(t, dbPath, `
	CREATE TABLE skills (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		version TEXT,
		summary TEXT,
		description TEXT,
		author TEXT,
		category TEXT,
		tags TEXT,
		source_id TEXT NOT NULL,
		source_name TEXT,
		homepage TEXT,
		download_url TEXT,
		stars INTEGER DEFAULT 0,
		downloads INTEGER DEFAULT 0,
		reviews INTEGER DEFAULT 0,
		rating REAL DEFAULT 0.0,
		versions INTEGER DEFAULT 0,
		changelog TEXT,
		readme TEXT,
		readme_hash TEXT,
		dedup_key TEXT,
		installed INTEGER DEFAULT 0,
		enabled INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		synced_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		search_content TEXT,
		skill_content TEXT
	);
	INSERT INTO skills (
		id, name, version, summary, description, author, category, tags, source_id, source_name,
		homepage, download_url, stars, downloads, reviews, rating, versions, changelog, readme,
		readme_hash, dedup_key, installed, enabled, search_content, skill_content
	) VALUES (
		'legacy-skill', 'Legacy Skill', '1.0.0', 'legacy summary', 'legacy description', 'legacy-author',
		'development', 'git,legacy', 'legacy-source', 'Legacy Source', 'https://example.com',
		'https://example.com/archive.zip', 12, 34, 0, 0.0, 0, '', '# Legacy', '', '', 0, 0,
		'legacy skill legacy description git legacy', '# Legacy'
	);
	CREATE VIRTUAL TABLE skills_fts USING fts5(
		id, name, summary, description, author, category, tags, readme,
		content='skills', content_rowid='rowid'
	);
	CREATE TRIGGER skills_au AFTER UPDATE ON skills BEGIN
		INSERT INTO skills_fts(skills_fts, rowid, id, name, summary, description, author, category, tags, readme)
		VALUES ('delete', old.rowid, old.id, old.name, old.summary, old.description, old.author, old.category, old.tags, old.readme);
		INSERT INTO skills_fts(rowid, id, name, summary, description, author, category, tags, readme)
		VALUES (new.rowid, new.id, new.name, new.summary, new.description, new.author, new.category, new.tags, new.readme);
	END;
	CREATE VIRTUAL TABLE skillmarket_fts USING fts5(
		id UNINDEXED,
		name,
		description,
		author,
		category,
		tags,
		skill_content,
		content='skills',
		content_rowid='rowid'
	);
	CREATE TRIGGER skillmarket_au AFTER UPDATE ON skills BEGIN
		INSERT INTO skillmarket_fts(skillmarket_fts, rowid, id, name, description, author, category, tags, skill_content)
		VALUES ('delete', old.rowid, old.id, old.name, old.description, old.author, old.category, old.tags, old.skill_content);
		INSERT INTO skillmarket_fts(rowid, id, name, description, author, category, tags, skill_content)
		VALUES (new.rowid, new.id, new.name, new.description, new.author, new.category, new.tags, new.skill_content);
	END;
	`)

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer db.Close()

	if supportsFTS5(db) {
		t.Skip("test requires a SQLite build without FTS5 support")
	}

	store, err := NewStore(db)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	if store.ftsEnabled {
		t.Fatalf("ftsEnabled = true, want false")
	}

	var slug string
	if err := db.QueryRow(`SELECT slug FROM skills WHERE id = ?`, "legacy-skill").Scan(&slug); err != nil {
		t.Fatalf("query slug error = %v", err)
	}
	if slug != "legacy-skill" {
		t.Fatalf("slug = %q, want legacy-skill", slug)
	}

	for _, trigger := range []string{"skills_au", "skillmarket_au"} {
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'trigger' AND name = ?`, trigger).Scan(&count); err != nil {
			t.Fatalf("query trigger %s error = %v", trigger, err)
		}
		if count != 0 {
			t.Fatalf("trigger %s should have been dropped", trigger)
		}
	}
}

func TestNewStoreAddsOriginSourceColumns(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "origin-columns.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer db.Close()

	legacySchema := `
	CREATE TABLE skills (
		id TEXT PRIMARY KEY,
		slug TEXT,
		name TEXT NOT NULL,
		description TEXT,
		category TEXT,
		tags TEXT,
		skill_content TEXT,
		trending_score REAL DEFAULT 0,
		last_updated DATETIME,
		downloads INTEGER DEFAULT 0,
		latest_version TEXT,
		source_id TEXT,
		source_name TEXT,
		source_group TEXT,
		security_badge TEXT DEFAULT 'yellow',
		install_type TEXT DEFAULT 'manual_external',
		artifact_kind TEXT DEFAULT 'unknown',
		curated_rank INTEGER DEFAULT 0,
		created_at DATETIME,
		updated_at DATETIME
	);
	`
	if _, err := db.Exec(legacySchema); err != nil {
		t.Fatalf("db.Exec(legacySchema) error = %v", err)
	}

	if _, err := NewStore(db); err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	for _, column := range []string{"origin_source_id", "origin_source_name", "origin_source_url"} {
		var count int
		if err := db.QueryRow(
			`SELECT COUNT(*) FROM pragma_table_info('skills') WHERE name = ?`,
			column,
		).Scan(&count); err != nil {
			t.Fatalf("query pragma for %s error = %v", column, err)
		}
		if count != 1 {
			t.Fatalf("expected column %s to be present, count=%d", column, count)
		}
	}
}
