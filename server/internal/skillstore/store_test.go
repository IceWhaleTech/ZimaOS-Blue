package skillstore

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) (*sql.DB, func()) {
	// Create temp database
	tmpFile, err := os.CreateTemp("", "skillstore_test_*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()

	// Use mattn/go-sqlite3 driver which supports FTS5
	db, err := sql.Open("sqlite3", tmpFile.Name())
	if err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("failed to open database: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.Remove(tmpFile.Name())
	}

	return db, cleanup
}

func TestStore_InitSchema(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store, err := NewStore(db)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	if store == nil {
		t.Fatal("store should not be nil")
	}

	// Verify tables exist (skills_fts requires FTS5 module which may not be available)
	for _, table := range []string{"skills", "skill_sync_status"} {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("table %s should exist: %v", table, err)
		}
	}
	// skills_fts is optional — only exists when SQLite has FTS5
	var ftsName string
	if err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='skills_fts'").Scan(&ftsName); err != nil {
		t.Logf("skills_fts not available (FTS5 module not loaded): %v", err)
	}
}

func TestStore_UpsertSkill(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store, err := NewStore(db)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()
	now := time.Now()

	skill := &Skill{
		ID:          "test-skill",
		Name:        "Test Skill",
		Version:     "1.0.0",
		Summary:     "A test skill for testing",
		Description: "This is a detailed description of the test skill",
		Author:      "Test Author",
		Category:    "testing",
		Tags:        "test,example",
		SourceID:    "clawhub",
		SourceName:  "ClawHub",
		Homepage:    "https://www.clawhub.ai/skills/test-skill",
		Stars:       100,
		Downloads:   500,
		CreatedAt:   now,
		UpdatedAt:   now,
		SyncedAt:    now,
	}

	// Insert
	err = store.UpsertSkill(ctx, skill)
	if err != nil {
		t.Fatalf("failed to insert skill: %v", err)
	}

	// Retrieve
	retrieved, err := store.GetSkill(ctx, "test-skill")
	if err != nil {
		t.Fatalf("failed to get skill: %v", err)
	}

	if retrieved == nil {
		t.Fatal("skill should not be nil")
	}

	if retrieved.Name != "Test Skill" {
		t.Errorf("expected name 'Test Skill', got '%s'", retrieved.Name)
	}
	if retrieved.Stars != 100 {
		t.Errorf("expected stars 100, got %d", retrieved.Stars)
	}

	// Update
	skill.Stars = 200
	skill.Version = "2.0.0"
	err = store.UpsertSkill(ctx, skill)
	if err != nil {
		t.Fatalf("failed to update skill: %v", err)
	}

	retrieved, err = store.GetSkill(ctx, "test-skill")
	if err != nil {
		t.Fatalf("failed to get updated skill: %v", err)
	}

	if retrieved.Stars != 200 {
		t.Errorf("expected stars 200, got %d", retrieved.Stars)
	}
	if retrieved.Version != "2.0.0" {
		t.Errorf("expected version '2.0.0', got '%s'", retrieved.Version)
	}
}

func TestStore_UpsertSkillBatch(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store, err := NewStore(db)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()
	now := time.Now()

	skills := []*Skill{
		{
			ID:        "skill-1",
			Name:      "Skill One",
			Version:   "1.0.0",
			Summary:   "First skill",
			SourceID:  "clawhub",
			Stars:     10,
			Downloads: 100,
			CreatedAt: now,
			UpdatedAt: now,
			SyncedAt:  now,
		},
		{
			ID:        "skill-2",
			Name:      "Skill Two",
			Version:   "2.0.0",
			Summary:   "Second skill",
			SourceID:  "clawhub",
			Stars:     20,
			Downloads: 200,
			CreatedAt: now,
			UpdatedAt: now,
			SyncedAt:  now,
		},
		{
			ID:        "skill-3",
			Name:      "Skill Three",
			Version:   "3.0.0",
			Summary:   "Third skill",
			SourceID:  "clawhub",
			Stars:     30,
			Downloads: 300,
			CreatedAt: now,
			UpdatedAt: now,
			SyncedAt:  now,
		},
	}

	err = store.UpsertSkillBatch(ctx, skills)
	if err != nil {
		t.Fatalf("failed to batch insert skills: %v", err)
	}

	// Verify all skills were inserted
	for _, skill := range skills {
		retrieved, err := store.GetSkill(ctx, skill.ID)
		if err != nil {
			t.Errorf("failed to get skill %s: %v", skill.ID, err)
			continue
		}
		if retrieved == nil {
			t.Errorf("skill %s should exist", skill.ID)
			continue
		}
		if retrieved.Name != skill.Name {
			t.Errorf("expected name '%s', got '%s'", skill.Name, retrieved.Name)
		}
	}
}

func TestStore_Search(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store, err := NewStore(db)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()
	now := time.Now()

	// Insert test skills
	skills := []*Skill{
		{
			ID:        "smart-home",
			Name:      "Smart Home Controller",
			Summary:   "Control your smart home devices",
			Category:  "automation",
			SourceID:  "clawhub",
			Stars:     100,
			Downloads: 1000,
			CreatedAt: now,
			UpdatedAt: now,
			SyncedAt:  now,
		},
		{
			ID:        "ai-writer",
			Name:      "AI Writing Assistant",
			Summary:   "AI-powered writing helper",
			Category:  "productivity",
			SourceID:  "clawhub",
			Stars:     200,
			Downloads: 2000,
			CreatedAt: now,
			UpdatedAt: now,
			SyncedAt:  now,
		},
		{
			ID:        "code-review",
			Name:      "Code Review Helper",
			Summary:   "Automated code review tool",
			Category:  "development",
			SourceID:  "clawhub",
			Stars:     50,
			Downloads: 500,
			CreatedAt: now,
			UpdatedAt: now,
			SyncedAt:  now,
		},
	}

	err = store.UpsertSkillBatch(ctx, skills)
	if err != nil {
		t.Fatalf("failed to insert skills: %v", err)
	}

	t.Run("search by query", func(t *testing.T) {
		// FTS5 query search requires the fts5 SQLite module
		var ftsName string
		if err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='skills_fts'").Scan(&ftsName); err != nil {
			t.Skip("FTS5 module not available, skipping query search test")
		}

		opts := SearchOptions{
			Query:    "smart home",
			Page:     1,
			PageSize: 10,
		}

		result, err := store.Search(ctx, opts)
		if err != nil {
			t.Fatalf("search failed: %v", err)
		}

		if result.Total == 0 {
			t.Error("expected at least one result")
		}
	})

	t.Run("search by category", func(t *testing.T) {
		opts := SearchOptions{
			Categories: []string{"development"},
			Page:       1,
			PageSize:   10,
		}

		result, err := store.Search(ctx, opts)
		if err != nil {
			t.Fatalf("search failed: %v", err)
		}

		if result.Total != 1 {
			t.Errorf("expected 1 result, got %d", result.Total)
		}
	})

	t.Run("search with min stars", func(t *testing.T) {
		opts := SearchOptions{
			MinStars: 100,
			Page:     1,
			PageSize: 10,
		}

		result, err := store.Search(ctx, opts)
		if err != nil {
			t.Fatalf("search failed: %v", err)
		}

		if result.Total != 2 {
			t.Errorf("expected 2 results, got %d", result.Total)
		}
	})

	t.Run("search with pagination", func(t *testing.T) {
		opts := SearchOptions{
			Page:     1,
			PageSize: 2,
		}

		result, err := store.Search(ctx, opts)
		if err != nil {
			t.Fatalf("search failed: %v", err)
		}

		if len(result.Skills) != 2 {
			t.Errorf("expected 2 skills on page, got %d", len(result.Skills))
		}
		if result.TotalPages != 2 {
			t.Errorf("expected 2 total pages, got %d", result.TotalPages)
		}
	})

	t.Run("search sorted by stars", func(t *testing.T) {
		opts := SearchOptions{
			SortBy:    "stars",
			SortOrder: "desc",
			Page:      1,
			PageSize:  10,
		}

		result, err := store.Search(ctx, opts)
		if err != nil {
			t.Fatalf("search failed: %v", err)
		}

		if len(result.Skills) < 2 {
			t.Fatal("expected at least 2 results")
		}

		if result.Skills[0].Stars < result.Skills[1].Stars {
			t.Error("results should be sorted by stars descending")
		}
	})
}

func TestStore_ListBySource(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store, err := NewStore(db)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()
	now := time.Now()

	// Insert skills from different sources
	skills := []*Skill{
		{ID: "skill-1", Name: "Skill 1", SourceID: "clawhub", CreatedAt: now, UpdatedAt: now, SyncedAt: now},
		{ID: "skill-2", Name: "Skill 2", SourceID: "clawhub", CreatedAt: now, UpdatedAt: now, SyncedAt: now},
		{ID: "skill-3", Name: "Skill 3", SourceID: "github", CreatedAt: now, UpdatedAt: now, SyncedAt: now},
	}

	err = store.UpsertSkillBatch(ctx, skills)
	if err != nil {
		t.Fatalf("failed to insert skills: %v", err)
	}

	result, total, err := store.ListBySource(ctx, "clawhub", 1, 10)
	if err != nil {
		t.Fatalf("failed to list by source: %v", err)
	}

	if total != 2 {
		t.Errorf("expected 2 skills from clawhub, got %d", total)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 skills in result, got %d", len(result))
	}
}

func TestStore_GetCategories(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store, err := NewStore(db)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()
	now := time.Now()

	skills := []*Skill{
		{ID: "skill-1", Name: "Skill 1", Category: "automation", SourceID: "clawhub", CreatedAt: now, UpdatedAt: now, SyncedAt: now},
		{ID: "skill-2", Name: "Skill 2", Category: "productivity", SourceID: "clawhub", CreatedAt: now, UpdatedAt: now, SyncedAt: now},
		{ID: "skill-3", Name: "Skill 3", Category: "automation", SourceID: "clawhub", CreatedAt: now, UpdatedAt: now, SyncedAt: now},
	}

	err = store.UpsertSkillBatch(ctx, skills)
	if err != nil {
		t.Fatalf("failed to insert skills: %v", err)
	}

	categories, err := store.GetCategories(ctx)
	if err != nil {
		t.Fatalf("failed to get categories: %v", err)
	}

	if len(categories) != 2 {
		t.Errorf("expected 2 unique categories, got %d", len(categories))
	}
}

func TestStore_GetStats(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store, err := NewStore(db)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()
	now := time.Now()

	skills := []*Skill{
		{ID: "skill-1", Name: "Skill 1", SourceID: "clawhub", Installed: true, CreatedAt: now, UpdatedAt: now, SyncedAt: now},
		{ID: "skill-2", Name: "Skill 2", SourceID: "clawhub", Installed: false, CreatedAt: now, UpdatedAt: now, SyncedAt: now},
		{ID: "skill-3", Name: "Skill 3", SourceID: "github", Installed: true, CreatedAt: now, UpdatedAt: now, SyncedAt: now},
	}

	err = store.UpsertSkillBatch(ctx, skills)
	if err != nil {
		t.Fatalf("failed to insert skills: %v", err)
	}

	stats, err := store.GetStats(ctx)
	if err != nil {
		t.Fatalf("failed to get stats: %v", err)
	}

	if stats["total_skills"].(int64) != 3 {
		t.Errorf("expected 3 total skills, got %v", stats["total_skills"])
	}

	if stats["installed"].(int64) != 2 {
		t.Errorf("expected 2 installed skills, got %v", stats["installed"])
	}

	bySource := stats["by_source"].(map[string]int64)
	if bySource["clawhub"] != 2 {
		t.Errorf("expected 2 clawhub skills, got %d", bySource["clawhub"])
	}
}

func TestStore_SetInstalled(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store, err := NewStore(db)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()
	now := time.Now()

	skill := &Skill{
		ID:        "test-skill",
		Name:      "Test Skill",
		SourceID:  "clawhub",
		Installed: false,
		CreatedAt: now,
		UpdatedAt: now,
		SyncedAt:  now,
	}

	err = store.UpsertSkill(ctx, skill)
	if err != nil {
		t.Fatalf("failed to insert skill: %v", err)
	}

	// Set installed
	err = store.SetInstalled(ctx, "test-skill", true)
	if err != nil {
		t.Fatalf("failed to set installed: %v", err)
	}

	retrieved, _ := store.GetSkill(ctx, "test-skill")
	if !retrieved.Installed {
		t.Error("skill should be installed")
	}

	// Unset installed
	err = store.SetInstalled(ctx, "test-skill", false)
	if err != nil {
		t.Fatalf("failed to unset installed: %v", err)
	}

	retrieved, _ = store.GetSkill(ctx, "test-skill")
	if retrieved.Installed {
		t.Error("skill should not be installed")
	}
}

func TestStore_SyncStatus(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store, err := NewStore(db)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()
	now := time.Now()

	status := &SyncStatus{
		SourceID:     "clawhub",
		LastSyncAt:   now,
		SkillCount:   100,
		SyncDuration: 5000,
		Status:       "success",
		NextSyncAt:   now.Add(24 * time.Hour),
	}

	err = store.UpdateSyncStatus(ctx, status)
	if err != nil {
		t.Fatalf("failed to update sync status: %v", err)
	}

	retrieved, err := store.GetSyncStatus(ctx, "clawhub")
	if err != nil {
		t.Fatalf("failed to get sync status: %v", err)
	}

	if retrieved == nil {
		t.Fatal("sync status should not be nil")
	}

	if retrieved.SkillCount != 100 {
		t.Errorf("expected skill count 100, got %d", retrieved.SkillCount)
	}
	if retrieved.Status != "success" {
		t.Errorf("expected status 'success', got '%s'", retrieved.Status)
	}
}

func TestEscapeFTS5Query(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", `"hello"*`},
		{"hello world", `"hello"* OR "world"*`},
		{"", ""},
		{"  spaces  ", `"spaces"*`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := escapeFTS5Query(tt.input)
			if result != tt.expected {
				t.Errorf("escapeFTS5Query(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestStore_SearchFallbackWithoutFTS(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store, err := NewStore(db)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	store.ftsEnabled = false

	ctx := context.Background()
	now := time.Now()
	if err := store.UpsertSkillBatch(ctx, []*Skill{
		{
			ID:        "smart-home",
			Name:      "Smart Home Controller",
			Summary:   "Control your smart home devices",
			Category:  "automation",
			SourceID:  "clawhub",
			Stars:     100,
			Downloads: 1000,
			CreatedAt: now,
			UpdatedAt: now,
			SyncedAt:  now,
		},
		{
			ID:        "ai-writer",
			Name:      "AI Writing Assistant",
			Summary:   "AI-powered writing helper",
			Category:  "productivity",
			SourceID:  "clawhub",
			Stars:     200,
			Downloads: 2000,
			CreatedAt: now,
			UpdatedAt: now,
			SyncedAt:  now,
		},
	}); err != nil {
		t.Fatalf("failed to insert skills: %v", err)
	}

	result, err := store.Search(ctx, SearchOptions{Query: "smart home", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("search fallback failed: %v", err)
	}
	if result.Total == 0 {
		t.Fatal("expected fallback search results")
	}
	if result.Skills[0].Skill.ID != "smart-home" {
		t.Fatalf("unexpected top fallback result: %+v", result.Skills[0].Skill)
	}
}

func TestStore_SearchHandlesNullableTextColumns(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store, err := NewStore(db)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	store.ftsEnabled = false

	if _, err := db.Exec(`
		INSERT INTO skills (
			id, name, source_id, search_content, created_at, updated_at, synced_at
		) VALUES (
			?, ?, ?, ?, datetime('now'), datetime('now'), datetime('now')
		)
	`, "nullable-skill", "Nullable Skill", "clawhub", "nullable skill smoke query"); err != nil {
		t.Fatalf("failed to insert nullable skill row: %v", err)
	}

	result, err := store.Search(context.Background(), SearchOptions{Query: "nullable", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("search with nullable columns failed: %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("expected one result, got %d", result.Total)
	}
	if result.Skills[0].Skill.ID != "nullable-skill" {
		t.Fatalf("unexpected skill id: %+v", result.Skills[0].Skill)
	}
	if result.Skills[0].Skill.Changelog != "" {
		t.Fatalf("expected empty changelog, got %q", result.Skills[0].Skill.Changelog)
	}
	if result.Skills[0].Skill.Summary != "" {
		t.Fatalf("expected empty summary, got %q", result.Skills[0].Skill.Summary)
	}
}

func TestStore_InitSchemaDropsStaleFTSTriggers(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	coreSchema := `
	CREATE TABLE IF NOT EXISTS skills (
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
	);`
	if _, err := db.Exec(coreSchema); err != nil {
		t.Fatalf("create core schema: %v", err)
	}
	if _, err := db.Exec(`
		CREATE TRIGGER skills_ai AFTER INSERT ON skills BEGIN
			SELECT RAISE(FAIL, 'stale trigger');
		END;
	`); err != nil {
		t.Fatalf("create stale trigger: %v", err)
	}

	store := &Store{db: db, zorm: NewZormStore(db)}
	if err := store.initSchema(); err != nil {
		t.Fatalf("initSchema failed: %v", err)
	}

	ctx := context.Background()
	now := time.Now()
	if err := store.UpsertSkill(ctx, &Skill{
		ID:        "post-init",
		Name:      "Post Init Skill",
		Summary:   "Works after stale trigger cleanup",
		Category:  "testing",
		SourceID:  "clawhub",
		CreatedAt: now,
		UpdatedAt: now,
		SyncedAt:  now,
	}); err != nil {
		t.Fatalf("upsert after trigger cleanup failed: %v", err)
	}
}
