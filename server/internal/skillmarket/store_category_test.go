package skillmarket

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestStoreNormalizeLegacyVersionLikeCategories(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "skillmarket.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	store, err := NewStore(db)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	if _, err := db.Exec(`
		INSERT INTO skills (
			id, slug, name, description, author, repo_url, homepage, download_url, stars, downloads, tags, category,
			security_score, permissions, latest_version, risk_level, security_badge, installable, install_type,
			artifact_kind, vulnerability_status, has_vulnerabilities, has_prompt_injection, has_shell_injection,
			has_data_exfiltration, has_binary, has_scripts, popularity_score, trending_score, scan_status,
			content_sha256, published, source_id, source_name, source_group, source_type, skill_path,
			skill_content, embedding_json, embedding_model, curated_rank, curated_boost, curated_label,
			curated_reason, last_updated, last_crawled_at, created_at, updated_at
		) VALUES (
			'legacy-git', 'legacy-git', 'Legacy Git', 'legacy fixture', '', '', '', '', 0, 0, '["git"]', '1.0.0',
			100, '[]', '1.0.0', 'low', 'green', 1, 'raw_skill',
			'open_source', 'not_applicable', 0, 0, 0,
			0, 0, 0, 0, 0, '',
			'', 1, 'legacy', 'Legacy', 'legacy', 'catalog', 'SKILL.md',
			'Git history helper for rebases and pull requests.', '', '', 0, 0, '',
			'', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
		)
	`); err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}

	if err := store.normalizeLegacyCategories(context.Background()); err != nil {
		t.Fatalf("normalizeLegacyCategories() error = %v", err)
	}

	detail, err := store.GetSkill(context.Background(), "legacy-git")
	if err != nil {
		t.Fatalf("GetSkill() error = %v", err)
	}
	if got := detail.Skill.Category; got != "development_tools" {
		t.Fatalf("category = %q, want development_tools", got)
	}

	filters, err := store.GetFilters(context.Background())
	if err != nil {
		t.Fatalf("GetFilters() error = %v", err)
	}
	if len(filters.Categories) != 1 || filters.Categories[0].Value != "development_tools" {
		t.Fatalf("filters.Categories = %+v, want development_tools bucket", filters.Categories)
	}
}
