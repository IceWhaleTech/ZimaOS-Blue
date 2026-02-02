package skillstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func setupSyncTestDB(t *testing.T) (*sql.DB, *Store, func()) {
	tmpFile, err := os.CreateTemp("", "skillstore_sync_test_*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()

	db, err := sql.Open("sqlite", tmpFile.Name())
	if err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("failed to open database: %v", err)
	}

	store, err := NewStore(db)
	if err != nil {
		db.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("failed to create store: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.Remove(tmpFile.Name())
	}

	return db, store, cleanup
}

// mockClawHubServer creates a mock server that returns paginated skills
func mockClawHubServer(t *testing.T, pages map[int][]ClawHubSkill) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v1/skills") {
			http.NotFound(w, r)
			return
		}

		pageStr := r.URL.Query().Get("page")
		page := 1
		if pageStr != "" {
			var err error
			page, err = strconv.Atoi(pageStr)
			if err != nil {
				http.Error(w, "invalid page", http.StatusBadRequest)
				return
			}
		}

		skills, ok := pages[page]
		if !ok {
			skills = []ClawHubSkill{} // Empty page
		}

		resp := ClawHubAPIResponse{
			Items: skills,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestSyncService_FetchClawHubSkillsWithInsert_MultiplePages(t *testing.T) {
	_, store, cleanup := setupSyncTestDB(t)
	defer cleanup()

	// Create mock server with 3 pages of skills (24 each)
	pages := map[int][]ClawHubSkill{
		1: makeTestSkills(1, 24),
		2: makeTestSkills(25, 24),
		3: makeTestSkills(49, 12), // Last page with fewer items
	}

	server := mockClawHubServer(t, pages)
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	syncSvc := NewSyncService(store, DefaultSyncServiceConfig(), logger)

	// Register test source pointing to mock server
	syncSvc.RegisterSource(&Source{
		ID:      "test-clawhub",
		Name:    "Test ClawHub",
		URL:     server.URL,
		Type:    "clawhub",
		Enabled: true,
	})

	ctx := context.Background()
	count, err := syncSvc.fetchClawHubSkillsWithInsert(ctx, &Source{
		ID:   "test-clawhub",
		Name: "Test ClawHub",
		URL:  server.URL,
		Type: "clawhub",
	})

	if err != nil {
		t.Fatalf("fetchClawHubSkillsWithInsert failed: %v", err)
	}

	// Should have fetched all 60 skills (24 + 24 + 12)
	if count != 60 {
		t.Errorf("expected 60 skills, got %d", count)
	}

	// Verify skills are in database
	stats, err := store.GetStats(ctx)
	if err != nil {
		t.Fatalf("failed to get stats: %v", err)
	}

	totalSkills := stats["total_skills"].(int64)
	if totalSkills != 60 {
		t.Errorf("expected 60 skills in database, got %d", totalSkills)
	}
}

func TestSyncService_FetchClawHubSkillsWithInsert_EmptyPageHandling(t *testing.T) {
	_, store, cleanup := setupSyncTestDB(t)
	defer cleanup()

	// Create mock server with an empty page in the middle
	pages := map[int][]ClawHubSkill{
		1: makeTestSkills(1, 24),
		2: {}, // Empty page - should continue
		3: makeTestSkills(25, 24),
		4: {}, // Empty page
		5: {}, // Second consecutive empty page - should stop
	}

	server := mockClawHubServer(t, pages)
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	syncSvc := NewSyncService(store, DefaultSyncServiceConfig(), logger)

	ctx := context.Background()
	count, err := syncSvc.fetchClawHubSkillsWithInsert(ctx, &Source{
		ID:   "test-clawhub",
		Name: "Test ClawHub",
		URL:  server.URL,
		Type: "clawhub",
	})

	if err != nil {
		t.Fatalf("fetchClawHubSkillsWithInsert failed: %v", err)
	}

	// Should have fetched 48 skills (24 from page 1 + 24 from page 3)
	// Page 2 was empty but continued, page 4 and 5 were consecutive empty so stopped
	if count != 48 {
		t.Errorf("expected 48 skills, got %d", count)
	}
}

func TestSyncService_FetchClawHubSkillsWithInsert_PageByPageInsertion(t *testing.T) {
	_, store, cleanup := setupSyncTestDB(t)
	defer cleanup()

	insertCount := 0
	pages := map[int][]ClawHubSkill{
		1: makeTestSkills(1, 24),
		2: makeTestSkills(25, 24),
	}

	// Create a server that tracks when pages are fetched
	pagesFetched := make([]int, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageStr := r.URL.Query().Get("page")
		page := 1
		if pageStr != "" {
			page, _ = strconv.Atoi(pageStr)
		}

		pagesFetched = append(pagesFetched, page)

		// After page 1 is fetched, check if skills are already in DB
		if page == 2 {
			ctx := context.Background()
			stats, _ := store.GetStats(ctx)
			insertCount = int(stats["total_skills"].(int64))
		}

		skills, ok := pages[page]
		if !ok {
			skills = []ClawHubSkill{}
		}

		resp := ClawHubAPIResponse{Items: skills}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	syncSvc := NewSyncService(store, DefaultSyncServiceConfig(), logger)

	ctx := context.Background()
	_, err := syncSvc.fetchClawHubSkillsWithInsert(ctx, &Source{
		ID:   "test-clawhub",
		Name: "Test ClawHub",
		URL:  server.URL,
		Type: "clawhub",
	})

	if err != nil {
		t.Fatalf("fetchClawHubSkillsWithInsert failed: %v", err)
	}

	// Verify page 1 skills were inserted before page 2 was fetched
	if insertCount < 24 {
		t.Errorf("expected at least 24 skills inserted before page 2 fetch, got %d", insertCount)
	}
}

func TestSyncService_DoSync_PartialSuccess(t *testing.T) {
	_, store, cleanup := setupSyncTestDB(t)
	defer cleanup()

	// Create mock server that fails on page 2
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		pageStr := r.URL.Query().Get("page")
		page := 1
		if pageStr != "" {
			page, _ = strconv.Atoi(pageStr)
		}

		if page == 2 {
			// Fail on page 2 after retries
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}

		skills := makeTestSkills(1, 24)
		resp := ClawHubAPIResponse{Items: skills}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	syncSvc := NewSyncService(store, DefaultSyncServiceConfig(), logger)

	source := &Source{
		ID:      "test-clawhub",
		Name:    "Test ClawHub",
		URL:     server.URL,
		Type:    "clawhub",
		Enabled: true,
	}
	syncSvc.RegisterSource(source)

	ctx := context.Background()
	err := syncSvc.doSync(ctx, source)

	// Should succeed with partial data (page 1 was saved)
	if err != nil {
		t.Errorf("expected nil error for partial success, got: %v", err)
	}

	// Verify page 1 skills are in database
	stats, _ := store.GetStats(ctx)
	totalSkills := stats["total_skills"].(int64)
	if totalSkills != 24 {
		t.Errorf("expected 24 skills from page 1, got %d", totalSkills)
	}

	// Verify sync status shows partial success
	status, _ := store.GetSyncStatus(ctx, "test-clawhub")
	if status == nil {
		t.Fatal("sync status should exist")
	}
	if status.Status != "success" {
		t.Errorf("expected status 'success' for partial sync, got '%s'", status.Status)
	}
	if status.SkillCount != 24 {
		t.Errorf("expected skill count 24, got %d", status.SkillCount)
	}
}

func TestSyncService_DoSync_CompleteFailure(t *testing.T) {
	_, store, cleanup := setupSyncTestDB(t)
	defer cleanup()

	// Create mock server that always fails
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	config := SyncServiceConfig{
		Interval: time.Hour,
		Timeout:  time.Second, // Short timeout for test
	}
	syncSvc := NewSyncService(store, config, logger)

	source := &Source{
		ID:      "test-clawhub",
		Name:    "Test ClawHub",
		URL:     server.URL,
		Type:    "clawhub",
		Enabled: true,
	}
	syncSvc.RegisterSource(source)

	ctx := context.Background()
	err := syncSvc.doSync(ctx, source)

	// Should fail completely
	if err == nil {
		t.Error("expected error for complete failure")
	}

	// Verify sync status shows failure
	status, _ := store.GetSyncStatus(ctx, "test-clawhub")
	if status == nil {
		t.Fatal("sync status should exist")
	}
	if status.Status != "failed" {
		t.Errorf("expected status 'failed', got '%s'", status.Status)
	}
}

// makeTestSkills creates test ClawHubSkill items
func makeTestSkills(startID, count int) []ClawHubSkill {
	skills := make([]ClawHubSkill, count)
	for i := 0; i < count; i++ {
		id := startID + i
		skills[i] = ClawHubSkill{
			Slug:        "test-skill-" + strconv.Itoa(id),
			DisplayName: "Test Skill " + strconv.Itoa(id),
			Summary:     "A test skill for testing",
			Stats: struct {
				Comments        int `json:"comments"`
				Downloads       int `json:"downloads"`
				InstallsAllTime int `json:"installsAllTime"`
				InstallsCurrent int `json:"installsCurrent"`
				Stars           int `json:"stars"`
				Versions        int `json:"versions"`
			}{
				Stars:     id * 10,
				Downloads: id * 100,
			},
			LatestVersion: struct {
				Version   string `json:"version"`
				CreatedAt int64  `json:"createdAt"`
				Changelog string `json:"changelog"`
			}{
				Version: "1.0.0",
			},
		}
	}
	return skills
}
