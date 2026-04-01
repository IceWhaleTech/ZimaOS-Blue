package skillstore

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestStoreUsesReaderDBForSearchAndStatsReads(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "skillstore-reader.db")

	writeDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open(write): %v", err)
	}
	t.Cleanup(func() {
		if writeDB != nil {
			_ = writeDB.Close()
		}
	})

	if _, err := NewStore(writeDB); err != nil {
		t.Fatalf("NewStore(bootstrap): %v", err)
	}

	readDB, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro")
	if err != nil {
		t.Fatalf("sql.Open(read): %v", err)
	}
	defer readDB.Close()

	store, err := NewStoreWithReadDB(writeDB, readDB)
	if err != nil {
		t.Fatalf("NewStoreWithReadDB: %v", err)
	}
	if store.readDB == nil || store.readDB == store.db {
		t.Fatal("expected dedicated reader db")
	}
	store.ftsEnabled = false

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	if err := store.UpsertSkillBatch(ctx, []*Skill{
		{
			ID:          "automation-starter",
			Name:        "Automation Starter",
			Version:     "1.0.0",
			Summary:     "Starter templates for automation workflows",
			Description: "Automation templates and starter workflows",
			Category:    "automation",
			SourceID:    "clawhub",
			SourceName:  "ClawHub",
			Stars:       120,
			Downloads:   2400,
			Installed:   true,
			Enabled:     true,
			CreatedAt:   now,
			UpdatedAt:   now,
			SyncedAt:    now,
		},
		{
			ID:          "review-helper",
			Name:        "Review Helper",
			Version:     "2.0.0",
			Summary:     "Code review support",
			Description: "Review pull requests and diffs",
			Category:    "development",
			SourceID:    "github",
			SourceName:  "GitHub",
			Stars:       40,
			Downloads:   300,
			Installed:   false,
			Enabled:     false,
			CreatedAt:   now,
			UpdatedAt:   now.Add(time.Minute),
			SyncedAt:    now.Add(time.Minute),
		},
	}); err != nil {
		t.Fatalf("UpsertSkillBatch: %v", err)
	}

	if err := writeDB.Close(); err != nil {
		t.Fatalf("close write db: %v", err)
	}
	writeDB = nil

	browse, err := store.Search(ctx, SearchOptions{
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("Search(browse): %v", err)
	}
	if browse.Total != 2 || len(browse.Skills) != 2 {
		t.Fatalf("unexpected browse search via reader: %+v", browse)
	}
	if browse.Skills[0].Skill.ID != "automation-starter" {
		t.Fatalf("unexpected browse ordering via reader: %+v", browse.Skills)
	}

	search, err := store.Search(ctx, SearchOptions{
		Query:    "automation starter",
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("Search(fallback): %v", err)
	}
	if search.Total != 1 || len(search.Skills) != 1 || search.Skills[0].Skill.ID != "automation-starter" {
		t.Fatalf("unexpected fallback search via reader: %+v", search)
	}

	stats, err := store.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if stats["total_skills"].(int64) != 2 {
		t.Fatalf("unexpected total_skills via reader: %+v", stats)
	}
	if stats["installed"].(int64) != 1 {
		t.Fatalf("unexpected installed count via reader: %+v", stats)
	}
	bySource := stats["by_source"].(map[string]int64)
	if bySource["clawhub"] != 1 || bySource["github"] != 1 {
		t.Fatalf("unexpected by_source via reader: %+v", bySource)
	}
}

func TestStoreUsesReaderDBForFTSSearch(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "skillstore-fts-reader.db")

	writeDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open(write): %v", err)
	}
	t.Cleanup(func() {
		if writeDB != nil {
			_ = writeDB.Close()
		}
	})

	if _, err := NewStore(writeDB); err != nil {
		t.Fatalf("NewStore(bootstrap): %v", err)
	}

	readDB, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro")
	if err != nil {
		t.Fatalf("sql.Open(read): %v", err)
	}
	defer readDB.Close()

	store, err := NewStoreWithReadDB(writeDB, readDB)
	if err != nil {
		t.Fatalf("NewStoreWithReadDB: %v", err)
	}
	if !store.ftsEnabled {
		t.Skip("fts is not available in this sqlite build")
	}

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	if err := store.UpsertSkillBatch(ctx, []*Skill{
		{
			ID:          "git-rebase-pro",
			Name:        "Git Rebase Pro",
			Version:     "1.0.0",
			Summary:     "Rebase and branch cleanup helper",
			Description: "Fix rebases, review branches, and tidy history",
			Category:    "development",
			SourceID:    "clawhub",
			SourceName:  "ClawHub",
			Stars:       80,
			Downloads:   1200,
			CreatedAt:   now,
			UpdatedAt:   now,
			SyncedAt:    now,
		},
		{
			ID:          "recipe-helper",
			Name:        "Recipe Helper",
			Version:     "1.0.0",
			Summary:     "Meal planning assistant",
			Description: "Recipes, meal prep, and grocery planning",
			Category:    "lifestyle",
			SourceID:    "github",
			SourceName:  "GitHub",
			Stars:       10,
			Downloads:   90,
			CreatedAt:   now,
			UpdatedAt:   now.Add(time.Minute),
			SyncedAt:    now.Add(time.Minute),
		},
	}); err != nil {
		t.Fatalf("UpsertSkillBatch: %v", err)
	}

	if err := writeDB.Close(); err != nil {
		t.Fatalf("close write db: %v", err)
	}
	writeDB = nil

	result, err := store.Search(ctx, SearchOptions{
		Query:    "rebase",
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("Search(fts): %v", err)
	}
	if !store.ftsEnabled {
		t.Fatal("ftsEnabled disabled during reader-backed fts search")
	}
	if result.Total != 1 || len(result.Skills) != 1 || result.Skills[0].Skill.ID != "git-rebase-pro" {
		t.Fatalf("unexpected fts search via reader: %+v", result)
	}
}
