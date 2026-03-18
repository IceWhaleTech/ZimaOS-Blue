package skillmarket

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestRefreshLiveSkillHubClubSource(t *testing.T) {
	if os.Getenv("LIVE_SKILLHUB_REFRESH") != "1" {
		t.Skip("set LIVE_SKILLHUB_REFRESH=1 to run live skillhub refresh")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("resolve home dir: %v", err)
	}
	dataDir := filepath.Join(home, ".zimaos-blue", "data")
	dbPath := filepath.Join(dataDir, "blue.db")
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("stat live db: %v", err)
	}

	dsn := fmt.Sprintf("file:%s?_busy_timeout=60000&_journal_mode=WAL", dbPath)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		t.Fatalf("open live db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	beforeUnknownSkills := mustCountSkillHubUnknownArtifacts(ctx, t, db)
	beforeUnknownReports := mustCountSkillHubUnknownReports(ctx, t, db)

	cfg := DefaultConfig(dataDir, filepath.Join(dataDir, "workspace", ".claude", "skills"))
	cfg.CacheRoot = filepath.Join(os.TempDir(), "skillmarket-live-refresh-cache")
	cfg.CuratedConfigPath = filepath.Join(moduleRoot(t), "skillmarket_curated.yaml")
	cfg.CuratedConfigURLs = nil
	cfg.SeedURLs = nil
	cfg.CrawlIncrementalInterval = 0
	cfg.CrawlFullInterval = 0
	cfg.UpdateCheckInterval = 0
	cfg.TelemetryRollupInterval = 0

	svc, err := NewService(db, Options{
		Config:     cfg,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		Scanner:    NewScanner(nil),
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	var source Source
	found := false
	sources, err := svc.store.ListSources(ctx)
	if err != nil {
		t.Fatalf("list sources: %v", err)
	}
	for _, candidate := range sources {
		if candidate.ID == "skillhub-club" {
			source = candidate
			found = true
			break
		}
	}
	if !found {
		t.Fatal("skillhub-club source not found")
	}

	run, err := svc.store.BeginCrawlRun(ctx, source.ID)
	if err != nil {
		t.Fatalf("begin crawl run: %v", err)
	}

	err = svc.discoverFromHTMLCatalog(ctx, source, run)
	if err != nil {
		run.Status = "failed"
		run.ErrorText = err.Error()
	} else {
		run.Status = "success"
	}
	if completeErr := svc.store.CompleteCrawlRun(ctx, run); completeErr != nil {
		t.Fatalf("complete crawl run: %v", completeErr)
	}
	if err != nil {
		t.Fatalf("refresh skillhub-club: %v", err)
	}

	afterUnknownSkills := mustCountSkillHubUnknownArtifacts(ctx, t, db)
	afterUnknownReports := mustCountSkillHubUnknownReports(ctx, t, db)

	t.Logf("skillhub-club refresh finished: discovered=%d updated=%d failed=%d", run.Discovered, run.Updated, run.Failed)
	t.Logf("unknown artifact_kind in skills: before=%d after=%d", beforeUnknownSkills, afterUnknownSkills)
	t.Logf("unknown artifact_kind in reports: before=%d after=%d", beforeUnknownReports, afterUnknownReports)
}

func TestDiagnoseSkillMarketInitOnCopy(t *testing.T) {
	if os.Getenv("LIVE_SKILLMARKET_DIAG") != "1" {
		t.Skip("set LIVE_SKILLMARKET_DIAG=1 to run skillmarket init diagnostics")
	}

	dbPath := firstNonEmptyEnv("SKILLMARKET_COPY_DB", "/tmp/skillmarket-debug-copy-2.db")
	dsn := fmt.Sprintf("file:%s?_busy_timeout=10000&_journal_mode=WAL", dbPath)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		t.Fatalf("open copy db: %v", err)
	}
	defer db.Close()

	columnDefs := map[string]map[string]string{
		"skills": {
			"slug":                  "TEXT DEFAULT ''",
			"repo_url":              "TEXT DEFAULT ''",
			"homepage":              "TEXT",
			"download_url":          "TEXT",
			"security_score":        "INTEGER DEFAULT 100",
			"permissions":           "TEXT DEFAULT '[]'",
			"latest_version":        "TEXT DEFAULT ''",
			"risk_level":            "TEXT DEFAULT 'unknown'",
			"source_name":           "TEXT",
			"source_group":          "TEXT",
			"source_type":           "TEXT DEFAULT ''",
			"security_badge":        "TEXT DEFAULT 'yellow'",
			"installable":           "INTEGER DEFAULT 0",
			"install_type":          "TEXT DEFAULT 'manual_external'",
			"artifact_kind":         "TEXT DEFAULT 'unknown'",
			"vulnerability_status":  "TEXT DEFAULT 'unknown'",
			"has_vulnerabilities":   "INTEGER DEFAULT 0",
			"has_prompt_injection":  "INTEGER DEFAULT 0",
			"has_shell_injection":   "INTEGER DEFAULT 0",
			"has_data_exfiltration": "INTEGER DEFAULT 0",
			"has_binary":            "INTEGER DEFAULT 0",
			"has_scripts":           "INTEGER DEFAULT 0",
			"popularity_score":      "REAL DEFAULT 0",
			"trending_score":        "REAL DEFAULT 0",
			"scan_status":           "TEXT DEFAULT ''",
			"content_sha256":        "TEXT DEFAULT ''",
			"published":             "INTEGER DEFAULT 1",
			"skill_path":            "TEXT DEFAULT ''",
			"skill_content":         "TEXT DEFAULT ''",
			"embedding_json":        "TEXT DEFAULT ''",
			"embedding_model":       "TEXT DEFAULT ''",
			"curated_rank":          "INTEGER DEFAULT 0",
			"curated_boost":         "REAL DEFAULT 0",
			"curated_label":         "TEXT",
			"curated_reason":        "TEXT",
			"last_updated":          "DATETIME",
			"last_crawled_at":       "DATETIME",
		},
		"skill_security_reports": {
			"security_badge":        "TEXT DEFAULT 'yellow'",
			"vulnerability_status":  "TEXT DEFAULT 'unknown'",
			"vulnerabilities_json":  "TEXT",
			"install_surface_json":  "TEXT",
			"evidence_json":         "TEXT",
			"artifact_kind":         "TEXT",
			"has_prompt_injection":  "INTEGER DEFAULT 0",
			"has_shell_injection":   "INTEGER DEFAULT 0",
			"has_data_exfiltration": "INTEGER DEFAULT 0",
		},
		"skill_sources": {
			"display_name": "TEXT",
			"source_group": "TEXT",
			"mirror_of":    "TEXT",
			"headers_json": "TEXT",
			"priority":     "INTEGER DEFAULT 100",
		},
	}
	indexSchema := `
	CREATE INDEX IF NOT EXISTS idx_skillmarket_category ON skills(category);
	CREATE INDEX IF NOT EXISTS idx_skillmarket_trending ON skills(trending_score DESC);
	CREATE INDEX IF NOT EXISTS idx_skillmarket_updated ON skills(last_updated DESC);
	CREATE INDEX IF NOT EXISTS idx_skillmarket_downloads ON skills(downloads DESC);
	CREATE INDEX IF NOT EXISTS idx_skillmarket_latest_version ON skills(latest_version);
	CREATE INDEX IF NOT EXISTS idx_skillmarket_source_group ON skills(source_group);
	CREATE INDEX IF NOT EXISTS idx_skillmarket_source_name ON skills(source_name);
	CREATE INDEX IF NOT EXISTS idx_skillmarket_badge ON skills(security_badge);
	CREATE INDEX IF NOT EXISTS idx_skillmarket_install_type ON skills(install_type);
	CREATE INDEX IF NOT EXISTS idx_skillmarket_artifact_kind ON skills(artifact_kind);
	CREATE INDEX IF NOT EXISTS idx_skillmarket_curated_rank ON skills(curated_rank DESC);
	CREATE INDEX IF NOT EXISTS idx_skillmarket_versions_skill ON skill_versions(skill_id);
	CREATE INDEX IF NOT EXISTS idx_skillmarket_reports_skill ON skill_security_reports(skill_id);
	`
	ftsStatements := []string{
		`CREATE VIRTUAL TABLE IF NOT EXISTS skillmarket_fts USING fts5(
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
		`CREATE TRIGGER IF NOT EXISTS skillmarket_ai AFTER INSERT ON skills BEGIN
			INSERT INTO skillmarket_fts(rowid, id, name, description, author, category, tags, skill_content)
			VALUES (new.rowid, new.id, new.name, new.description, new.author, new.category, new.tags, new.skill_content);
		END`,
		`CREATE TRIGGER IF NOT EXISTS skillmarket_ad AFTER DELETE ON skills BEGIN
			INSERT INTO skillmarket_fts(skillmarket_fts, rowid, id, name, description, author, category, tags, skill_content)
			VALUES ('delete', old.rowid, old.id, old.name, old.description, old.author, old.category, old.tags, old.skill_content);
		END`,
		`CREATE TRIGGER IF NOT EXISTS skillmarket_au AFTER UPDATE ON skills BEGIN
			INSERT INTO skillmarket_fts(skillmarket_fts, rowid, id, name, description, author, category, tags, skill_content)
			VALUES ('delete', old.rowid, old.id, old.name, old.description, old.author, old.category, old.tags, old.skill_content);
			INSERT INTO skillmarket_fts(rowid, id, name, description, author, category, tags, skill_content)
			VALUES (new.rowid, new.id, new.name, new.description, new.author, new.category, new.tags, new.skill_content);
		END`,
	}

	for _, table := range sortedKeys(columnDefs) {
		defs := columnDefs[table]
		for _, column := range sortedKeys(defs) {
			if err := ensureColumn(db, table, column, defs[column]); err != nil {
				t.Fatalf("ensureColumn(%s.%s): %v", table, column, err)
			}
		}
	}
	t.Log("ensureColumn ok")

	if _, err := db.Exec(`UPDATE skills SET slug = id WHERE COALESCE(slug, '') = ''`); err != nil {
		t.Fatalf("slug backfill: %v", err)
	}
	t.Log("slug backfill ok")

	if _, err := db.Exec(indexSchema); err != nil {
		t.Fatalf("index schema: %v", err)
	}
	t.Log("index schema ok")

	if err := dropFTSTriggers(db); err != nil {
		t.Fatalf("drop skillmarket FTS triggers: %v", err)
	}
	t.Log("drop skillmarket FTS triggers ok")

	if supportsFTS5(db) {
		for i, stmt := range ftsStatements {
			if _, err := db.Exec(stmt); err != nil {
				t.Fatalf("fts statement %d: %v", i, err)
			}
		}
		t.Log("skillmarket FTS setup ok")
	}

	store := &Store{db: db}
	if err := store.normalizeLegacyCategories(context.Background()); err != nil {
		diagnoseSkillsRowScan(t, db)
		diagnoseSkillsStream(t, db)
		if dropErr := dropFTSTriggers(db); dropErr != nil {
			t.Logf("drop skillmarket FTS triggers failed: %v", dropErr)
		} else if retryErr := store.normalizeLegacyCategories(context.Background()); retryErr != nil {
			t.Logf("normalizeLegacyCategories after dropping skillmarket triggers still failed: %v", retryErr)
		} else {
			t.Log("normalizeLegacyCategories succeeded after dropping skillmarket triggers")
		}
		if dropErr := dropLegacySkillsFTSTriggers(db); dropErr != nil {
			t.Logf("drop legacy skills FTS triggers failed: %v", dropErr)
		} else if retryErr := store.normalizeLegacyCategories(context.Background()); retryErr != nil {
			t.Logf("normalizeLegacyCategories after dropping legacy skills triggers still failed: %v", retryErr)
		} else {
			t.Log("normalizeLegacyCategories succeeded after dropping legacy skills FTS triggers")
		}
		if dropErr := dropAllSkillFTSTriggers(db); dropErr != nil {
			t.Logf("drop all skill FTS triggers failed: %v", dropErr)
		} else if retryErr := store.normalizeLegacyCategories(context.Background()); retryErr != nil {
			t.Logf("normalizeLegacyCategories after dropping all skill FTS triggers still failed: %v", retryErr)
		} else {
			t.Log("normalizeLegacyCategories succeeded after dropping all skill FTS triggers")
		}
		t.Fatalf("normalizeLegacyCategories: %v", err)
	}
	t.Log("normalizeLegacyCategories ok")
}

func mustCountSkillHubUnknownArtifacts(ctx context.Context, t *testing.T, db *sql.DB) int {
	t.Helper()
	var count int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM skills
		WHERE source_id = 'skillhub-club' AND artifact_kind = 'unknown'
	`).Scan(&count)
	if err != nil {
		t.Fatalf("count unknown skill artifacts: %v", err)
	}
	return count
}

func mustCountSkillHubUnknownReports(ctx context.Context, t *testing.T, db *sql.DB) int {
	t.Helper()
	var count int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM skill_security_reports
		WHERE skill_id IN (SELECT id FROM skills WHERE source_id = 'skillhub-club')
		  AND artifact_kind = 'unknown'
	`).Scan(&count)
	if err != nil {
		t.Fatalf("count unknown security report artifacts: %v", err)
	}
	return count
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return wd
}

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func diagnoseSkillsRowScan(t *testing.T, db *sql.DB) {
	t.Helper()
	var total int
	if err := db.QueryRow(`SELECT COUNT(*) FROM skills`).Scan(&total); err != nil {
		t.Logf("count skills failed during diagnosis: %v", err)
		return
	}
	for offset := 0; offset < total; offset++ {
		var rowid int64
		var id, category, tags, content string
		err := db.QueryRow(`
			SELECT rowid, id, COALESCE(category, ''), COALESCE(tags, '[]'), COALESCE(skill_content, '')
			FROM skills
			ORDER BY rowid
			LIMIT 1 OFFSET ?
		`, offset).Scan(&rowid, &id, &category, &tags, &content)
		if err != nil {
			t.Logf("row scan failed at offset=%d: %v", offset, err)
			return
		}
	}
	t.Log("row-by-row scan over skills completed without reproducing the error")
}

func diagnoseSkillsStream(t *testing.T, db *sql.DB) {
	t.Helper()
	rows, err := db.Query(`
		SELECT id, COALESCE(category, ''), COALESCE(tags, '[]'), COALESCE(skill_content, '')
		FROM skills
	`)
	if err != nil {
		t.Logf("stream query failed: %v", err)
		return
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id, category, tags, content string
		if err := rows.Scan(&id, &category, &tags, &content); err != nil {
			t.Logf("stream scan failed after %d rows: %v", count, err)
			return
		}
		count++
	}
	if err := rows.Err(); err != nil {
		t.Logf("stream rows.Err after %d rows: %v", count, err)
		return
	}
	t.Logf("stream scan completed all %d rows without reproducing the error", count)
}

func dropLegacySkillsFTSTriggers(db *sql.DB) error {
	for _, stmt := range []string{
		`DROP TRIGGER IF EXISTS skills_ai`,
		`DROP TRIGGER IF EXISTS skills_ad`,
		`DROP TRIGGER IF EXISTS skills_au`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func dropAllSkillFTSTriggers(db *sql.DB) error {
	for _, stmt := range []string{
		`DROP TRIGGER IF EXISTS skills_ai`,
		`DROP TRIGGER IF EXISTS skills_ad`,
		`DROP TRIGGER IF EXISTS skills_au`,
		`DROP TRIGGER IF EXISTS skillmarket_ai`,
		`DROP TRIGGER IF EXISTS skillmarket_ad`,
		`DROP TRIGGER IF EXISTS skillmarket_au`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}
