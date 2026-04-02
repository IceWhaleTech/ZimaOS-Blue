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
