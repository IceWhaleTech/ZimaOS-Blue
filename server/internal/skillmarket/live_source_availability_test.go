package skillmarket

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillstore"
)

func TestLiveSkillMarketAvailabilityReport(t *testing.T) {
	if testing.Short() || getenvTrimmed("LIVE_SKILLMARKET_AVAILABILITY") != "1" {
		t.Skip("set LIVE_SKILLMARKET_AVAILABILITY=1 to run the live 9-source availability report")
	}

	tempDir := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tempDir, "skillmarket-live.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	activeDir := filepath.Join(tempDir, "active")
	cfg := DefaultConfig(tempDir, activeDir)
	cfg.CacheRoot = filepath.Join(tempDir, "cache")
	cfg.CuratedConfigPath = filepath.Join(tempDir, "missing-curations.yaml")
	cfg.CuratedConfigURLs = nil
	cfg.SeedURLs = nil
	cfg.LightmakePageSize = 25
	cfg.GitHubSearchPageSize = 10
	cfg.IngestBatchSize = 25
	cfg.HTMLCatalogCrawlBatchPages = 2
	cfg.HTMLCatalogCrawlMaxPages = 80
	cfg.SeedPageMaxConcurrency = 1

	svc, err := NewService(db, Options{
		Config:       cfg,
		Registry:     skill.NewRegistry(),
		LocalScanner: skillstore.NewLocalSkillScanner(activeDir),
		Scanner:      NewScanner(nil),
		HTTPClient:   &http.Client{Timeout: 30 * time.Second},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	sources, err := svc.store.ListSources(context.Background())
	if err != nil {
		t.Fatalf("ListSources() error = %v", err)
	}
	if len(sources) != 9 {
		t.Fatalf("enabled source count = %d, want 9", len(sources))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	result, discoverErr := svc.Discover(ctx)
	if result == nil {
		t.Fatalf("Discover() returned nil result (err=%v)", discoverErr)
	}
	if discoverErr != nil {
		t.Logf("discover finished with aggregate error: %v", discoverErr)
	}

	t.Logf("skillmarket live report: discovered=%d updated=%d failed=%d processed=%d",
		result.Discovered, result.Updated, result.Failed, result.SourcesProcessed)
	for _, source := range result.SourceResults {
		t.Logf("source=%s status=%s partial=%v discovered=%d updated=%d failed=%d pages=%d requests=%d warnings=%v",
			source.SourceID, source.Status, source.Partial, source.Discovered, source.Updated, source.Failed, source.Pages, source.Requests, source.Warnings)
	}
}

func getenvTrimmed(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}
