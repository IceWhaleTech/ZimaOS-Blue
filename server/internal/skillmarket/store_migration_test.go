package skillmarket

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

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
