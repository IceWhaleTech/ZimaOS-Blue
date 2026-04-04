package skillmarket

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillstore"
)

func TestSyncCurationsAppliesFeaturedAndHidden(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "skillmarket_curated.yaml")
	if err := os.WriteFile(configPath, []byte(`
version: "1"
featured:
  - skill_id: git-expert
    rank: 1
    label: Editor's pick
    reason: Best reviewed workflow helper
boosts: []
hidden:
  - hidden-skill
additional_seeds: []
`), 0o644); err != nil {
		t.Fatalf("write curated config: %v", err)
	}

	db, err := sql.Open("sqlite3", filepath.Join(tempDir, "skillmarket.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	activeDir := filepath.Join(tempDir, "active")
	cfg := DefaultConfig(tempDir, activeDir)
	cfg.CacheRoot = filepath.Join(tempDir, "cache")
	cfg.CuratedConfigPath = configPath
	cfg.CuratedConfigURLs = nil
	cfg.DiscoveryPageURLs = nil

	svc, err := NewService(db, Options{
		Config:       cfg,
		Registry:     skill.NewRegistry(),
		LocalScanner: skillstore.NewLocalSkillScanner(activeDir),
		Scanner:      NewScanner(nil),
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	insertSkillFixture(t, svc, "git-expert", "1.0.0", `---
id: git-expert
name: Git Expert
description: Featured git workflows
---
Git helper.`, RiskLow, 92)
	insertSkillFixture(t, svc, "hidden-skill", "1.0.0", `---
id: hidden-skill
name: Hidden Skill
description: Hidden fixture
---
Hidden helper.`, RiskLow, 90)

	if err := svc.syncCurations(ctx); err != nil {
		t.Fatalf("sync curations: %v", err)
	}

	featured, err := svc.Featured(ctx, "", "", 10)
	if err != nil {
		t.Fatalf("featured: %v", err)
	}
	if len(featured) != 1 || featured[0].ID != "git-expert" {
		t.Fatalf("featured = %#v, want git-expert only", featured)
	}
	if featured[0].CuratedRank != 1 {
		t.Fatalf("curated rank = %d, want 1", featured[0].CuratedRank)
	}
	if featured[0].CuratedLabel != "Editor's pick" {
		t.Fatalf("curated label = %q", featured[0].CuratedLabel)
	}

	search, err := svc.Search(ctx, SearchQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	for _, item := range search.Skills {
		if item.Skill.ID == "hidden-skill" {
			t.Fatal("hidden skill should not appear in published search results")
		}
	}

	state, err := svc.store.GetCurationSyncState(ctx, "default")
	if err != nil {
		t.Fatalf("get sync state: %v", err)
	}
	if state.SourceURL != configPath {
		t.Fatalf("source url = %q, want %q", state.SourceURL, configPath)
	}
	if state.Checksum == "" {
		t.Fatal("expected curation checksum to be recorded")
	}
}

func TestSyncCurationsRegistersCuratedSeedPageSources(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "skillmarket_curated.yaml")
	if err := os.WriteFile(configPath, []byte(`
version: "1"
featured: []
boosts: []
hidden: []
additional_seeds:
  - type: seed_page
    id: github-awesome-composio
    display_name: GitHub Awesome Skills
    source_group: github-awesome-skills
    origin_name: ComposioHQ Awesome Claude Skills
    value: https://github.com/ComposioHQ/awesome-claude-skills
`), 0o644); err != nil {
		t.Fatalf("write curated config: %v", err)
	}

	db, err := sql.Open("sqlite3", filepath.Join(tempDir, "skillmarket.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	activeDir := filepath.Join(tempDir, "active")
	cfg := DefaultConfig(tempDir, activeDir)
	cfg.CacheRoot = filepath.Join(tempDir, "cache")
	cfg.CuratedConfigPath = configPath
	cfg.CuratedConfigURLs = nil
	cfg.DiscoveryPageURLs = nil

	svc, err := NewService(db, Options{
		Config:       cfg,
		Registry:     skill.NewRegistry(),
		LocalScanner: skillstore.NewLocalSkillScanner(activeDir),
		Scanner:      NewScanner(nil),
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if err := svc.syncCurations(ctx); err != nil {
		t.Fatalf("sync curations: %v", err)
	}

	sources, err := svc.store.ListSources(ctx)
	if err != nil {
		t.Fatalf("ListSources() error = %v", err)
	}

	var found *Source
	for i := range sources {
		if sources[i].ID == "github-awesome-composio" {
			found = &sources[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("expected curated seed page source to be registered, got %+v", sources)
	}
	if found.Type != "seed_page" {
		t.Fatalf("source type = %q, want seed_page", found.Type)
	}
	if found.BaseURL != "https://github.com/ComposioHQ/awesome-claude-skills" {
		t.Fatalf("base url = %q", found.BaseURL)
	}
	if found.DisplayName != "ComposioHQ Awesome Claude Skills" {
		t.Fatalf("display name = %q", found.DisplayName)
	}
	if found.SourceGroup != "github-awesome-skills" {
		t.Fatalf("source group = %q", found.SourceGroup)
	}
}
