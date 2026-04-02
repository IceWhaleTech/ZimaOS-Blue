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

func TestStoreGetFiltersMergesSourcesByGroup(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "skillmarket.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	store, err := NewStore(db)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	ctx := context.Background()

	fixtures := []SkillDocument{
		{
			ID:            "skillhub-alpha",
			Slug:          "skillhub-alpha",
			Name:          "SkillHub Alpha",
			Description:   "Grouped under skillhub",
			LatestVersion: "1.0.0",
			Installable:   true,
			InstallType:   InstallTypeRawSkill,
			ArtifactKind:  ArtifactKindOpenSource,
			Published:     true,
			SourceID:      "tencent-skillhub",
			SourceName:    "Tencent SkillHub",
			SourceGroup:   "skillhub",
			SourceType:    "lightmake_api",
		},
		{
			ID:            "skillhub-beta",
			Slug:          "skillhub-beta",
			Name:          "SkillHub Beta",
			Description:   "Also grouped under skillhub",
			LatestVersion: "1.0.0",
			Installable:   true,
			InstallType:   InstallTypeRawSkill,
			ArtifactKind:  ArtifactKindOpenSource,
			Published:     true,
			SourceID:      "skillhub-club",
			SourceName:    "SkillHub Club",
			SourceGroup:   "skillhub",
			SourceType:    "html_catalog",
		},
		{
			ID:            "custom-source-skill",
			Slug:          "custom-source-skill",
			Name:          "Custom Source Skill",
			Description:   "Falls back to source id when source_group is empty",
			LatestVersion: "1.0.0",
			Installable:   true,
			InstallType:   InstallTypeRawSkill,
			ArtifactKind:  ArtifactKindOpenSource,
			Published:     true,
			SourceID:      "custom-source",
			SourceName:    "Custom Source",
			SourceType:    "html_catalog",
		},
		{
			ID:            "skillstack-alpha",
			Slug:          "skillstack-alpha",
			Name:          "SkillStack Alpha",
			Description:   "Independent source group",
			LatestVersion: "1.0.0",
			Installable:   true,
			InstallType:   InstallTypeRawSkill,
			ArtifactKind:  ArtifactKindOpenSource,
			Published:     true,
			SourceID:      "skillstack",
			SourceName:    "SkillStack",
			SourceGroup:   "skillstack",
			SourceType:    "html_catalog",
		},
	}

	for i := range fixtures {
		doc := fixtures[i]
		if err := store.UpsertSkill(ctx, &doc, nil, nil); err != nil {
			t.Fatalf("UpsertSkill(%s) error = %v", doc.ID, err)
		}
	}

	filters, err := store.GetFilters(ctx)
	if err != nil {
		t.Fatalf("GetFilters() error = %v", err)
	}

	if len(filters.Sources) != 3 {
		t.Fatalf("filters.Sources len = %d, want 3 (%+v)", len(filters.Sources), filters.Sources)
	}
	if got := filters.Sources[0]; got.Value != "skillhub" || got.Count != 2 {
		t.Fatalf("filters.Sources[0] = %+v, want skillhub bucket with count 2", got)
	}
	if got := filters.Sources[1]; got.Value != "custom-source" || got.Count != 1 {
		t.Fatalf("filters.Sources[1] = %+v, want custom-source fallback bucket", got)
	}
	if got := filters.Sources[2]; got.Value != "skillstack" || got.Count != 1 {
		t.Fatalf("filters.Sources[2] = %+v, want skillstack bucket", got)
	}
}

func TestStoreKeepsHigherPrioritySourceWhenLowerPriorityDuplicateArrives(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "skillmarket.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	store, err := NewStore(db)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	ctx := context.Background()

	for _, source := range []Source{
		{ID: "tencent-skillhub", Type: "lightmake_api", BaseURL: "https://lightmake.site", Enabled: true, Priority: 5},
		{ID: "clawhub", Type: "clawhub", BaseURL: "https://www.clawhub.ai", Enabled: true, Priority: 10},
	} {
		if err := store.UpsertSource(ctx, source); err != nil {
			t.Fatalf("UpsertSource(%s) error = %v", source.ID, err)
		}
	}

	if err := store.UpsertSkill(ctx, &SkillDocument{
		ID:            "shared-skill",
		Slug:          "shared-skill",
		Name:          "Shared Skill",
		Description:   "Tencent catalog copy",
		LatestVersion: "1.0.0",
		Installable:   true,
		InstallType:   InstallTypeSourceArchive,
		ArtifactKind:  ArtifactKindUnknown,
		Published:     true,
		SourceID:      "tencent-skillhub",
		SourceName:    "Tencent SkillHub",
		SourceGroup:   "skillhub",
		SourceType:    "lightmake_api",
		DownloadURL:   "https://lightmake.site/api/v1/download?slug=shared-skill",
	}, nil, nil); err != nil {
		t.Fatalf("UpsertSkill(tencent) error = %v", err)
	}

	if err := store.UpsertSkill(ctx, &SkillDocument{
		ID:            "shared-skill",
		Slug:          "shared-skill",
		Name:          "Shared Skill",
		Description:   "ClawHub raw copy",
		LatestVersion: "1.0.1",
		Installable:   true,
		InstallType:   InstallTypeRawSkill,
		ArtifactKind:  ArtifactKindOpenSource,
		Published:     true,
		SourceID:      "clawhub",
		SourceName:    "ClawHub",
		SourceGroup:   "clawhub",
		SourceType:    "clawhub",
		DownloadURL:   "https://www.clawhub.ai/api/v1/skills/shared-skill/skill-md",
	}, nil, nil); err != nil {
		t.Fatalf("UpsertSkill(clawhub) error = %v", err)
	}

	detail, err := store.GetSkill(ctx, "shared-skill")
	if err != nil {
		t.Fatalf("GetSkill() error = %v", err)
	}
	if detail == nil {
		t.Fatal("expected shared-skill to exist")
	}
	if detail.Skill.SourceID != "tencent-skillhub" {
		t.Fatalf("source id = %q, want tencent-skillhub", detail.Skill.SourceID)
	}
	if detail.Skill.InstallType != InstallTypeSourceArchive {
		t.Fatalf("install type = %q, want %q", detail.Skill.InstallType, InstallTypeSourceArchive)
	}
	if detail.Skill.Description != "Tencent catalog copy" {
		t.Fatalf("description = %q, want Tencent catalog copy", detail.Skill.Description)
	}
}

func TestStoreAllowsHigherPrioritySourceToReplaceLowerPriorityDuplicate(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "skillmarket.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	store, err := NewStore(db)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	ctx := context.Background()

	for _, source := range []Source{
		{ID: "tencent-skillhub", Type: "lightmake_api", BaseURL: "https://lightmake.site", Enabled: true, Priority: 5},
		{ID: "clawhub", Type: "clawhub", BaseURL: "https://www.clawhub.ai", Enabled: true, Priority: 10},
	} {
		if err := store.UpsertSource(ctx, source); err != nil {
			t.Fatalf("UpsertSource(%s) error = %v", source.ID, err)
		}
	}

	if err := store.UpsertSkill(ctx, &SkillDocument{
		ID:            "shared-skill",
		Slug:          "shared-skill",
		Name:          "Shared Skill",
		Description:   "ClawHub raw copy",
		LatestVersion: "1.0.1",
		Installable:   true,
		InstallType:   InstallTypeRawSkill,
		ArtifactKind:  ArtifactKindOpenSource,
		Published:     true,
		SourceID:      "clawhub",
		SourceName:    "ClawHub",
		SourceGroup:   "clawhub",
		SourceType:    "clawhub",
		DownloadURL:   "https://www.clawhub.ai/api/v1/skills/shared-skill/skill-md",
	}, nil, nil); err != nil {
		t.Fatalf("UpsertSkill(clawhub) error = %v", err)
	}

	if err := store.UpsertSkill(ctx, &SkillDocument{
		ID:            "shared-skill",
		Slug:          "shared-skill",
		Name:          "Shared Skill",
		Description:   "Tencent catalog copy",
		LatestVersion: "1.0.0",
		Installable:   true,
		InstallType:   InstallTypeSourceArchive,
		ArtifactKind:  ArtifactKindUnknown,
		Published:     true,
		SourceID:      "tencent-skillhub",
		SourceName:    "Tencent SkillHub",
		SourceGroup:   "skillhub",
		SourceType:    "lightmake_api",
		DownloadURL:   "https://lightmake.site/api/v1/download?slug=shared-skill",
	}, nil, nil); err != nil {
		t.Fatalf("UpsertSkill(tencent) error = %v", err)
	}

	detail, err := store.GetSkill(ctx, "shared-skill")
	if err != nil {
		t.Fatalf("GetSkill() error = %v", err)
	}
	if detail == nil {
		t.Fatal("expected shared-skill to exist")
	}
	if detail.Skill.SourceID != "tencent-skillhub" {
		t.Fatalf("source id = %q, want tencent-skillhub", detail.Skill.SourceID)
	}
	if detail.Skill.InstallType != InstallTypeSourceArchive {
		t.Fatalf("install type = %q, want %q", detail.Skill.InstallType, InstallTypeSourceArchive)
	}
	if detail.Skill.Description != "Tencent catalog copy" {
		t.Fatalf("description = %q, want Tencent catalog copy", detail.Skill.Description)
	}
}
