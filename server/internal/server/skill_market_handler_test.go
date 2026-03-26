package server

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	_ "github.com/mattn/go-sqlite3"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skilladvisor"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmarket"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillstore"
)

type stubInstalledSkillSelector struct {
	decision agentcore.Decision
	err      error
	called   int
	query    string
	opts     agentcore.SelectOptions
}

func (s *stubInstalledSkillSelector) Select(_ context.Context, query string, opts agentcore.SelectOptions) (agentcore.Decision, error) {
	s.called++
	s.query = query
	s.opts = opts
	if s.err != nil {
		return agentcore.Decision{}, s.err
	}
	return s.decision, nil
}

func testSkillMarketConfig(dataDir, activeDir string) skillmarket.Config {
	cfg := skillmarket.DefaultConfig(dataDir, activeDir)
	cfg.CuratedConfigPath = filepath.Join(dataDir, "missing-curations.yaml")
	cfg.CuratedConfigURLs = nil
	cfg.SeedURLs = nil
	return cfg
}

func TestMarketSearchSkills(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "market.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	activeDir := filepath.Join(t.TempDir(), "active")
	cfg := testSkillMarketConfig(t.TempDir(), activeDir)
	market, err := skillmarket.NewService(db, skillmarket.Options{
		Config:       cfg,
		Registry:     skill.NewRegistry(),
		LocalScanner: skillstore.NewLocalSkillScanner(activeDir),
		Scanner:      skillmarket.NewScanner(nil),
	})
	if err != nil {
		t.Fatalf("new market: %v", err)
	}

	raw := `---
id: git-expert
name: Git Expert
version: 1.0.0
description: Git search fixture
---
Git branch review and rebase helper.`
	report := market.Store().UpsertSkill(context.Background(),
		&skillmarket.SkillDocument{
			ID:            "git-expert",
			Slug:          "git-expert",
			Name:          "Git Expert",
			Description:   "Git search fixture",
			Category:      "development",
			LatestVersion: "1.0.0",
			RiskLevel:     skillmarket.RiskLow,
			SecurityScore: 90,
			Published:     true,
			SkillContent:  raw,
		},
		&skillmarket.SkillVersion{
			ID:           "git-expert-version",
			SkillID:      "git-expert",
			Version:      "1.0.0",
			Checksum:     checksum(raw),
			RawSkillMD:   raw,
			ManifestJSON: `{"id":"git-expert","name":"Git Expert","version":"1.0.0"}`,
		},
		&skillmarket.SecurityReport{
			ID:             "git-expert-report",
			SkillID:        "git-expert",
			Version:        "1.0.0",
			Score:          90,
			RiskLevel:      skillmarket.RiskLow,
			ScannerVersion: skillmarket.ScannerVersion,
			LLMStatus:      "skipped",
		},
	)
	if report != nil {
		t.Fatalf("upsert skill: %v", report)
	}

	handler := NewSkillHandler(skill.NewRegistry())
	handler.SetMarketplace(market)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/skills/search?q=rebase", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.MarketSearchSkills(c); err != nil {
		t.Fatalf("search handler: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestMarketAdviseSkillsUsesSelectorAndMarketplace(t *testing.T) {
	handler := NewSkillHandler(skill.NewRegistry())
	selector := &stubInstalledSkillSelector{
		decision: agentcore.Decision{
			Query:         "帮我做 GitHub Actions 自动发版",
			SelectedSkill: "",
			Confidence:    0.34,
			NeedClarify:   true,
			Reason:        "no_skill_docs",
			Stage:         "ir",
		},
	}
	handler.SetSkillSelector(selector)
	handler.SetSkillSelectorOptionsProvider(func() agentcore.SelectOptions {
		return agentcore.SelectOptions{
			Mode:                agentcore.SkillSelectorModeHybrid,
			EnableRerank:        true,
			ConfidenceThreshold: 0.88,
		}
	})
	handler.SetSkillAdvisor(skilladvisor.NewService(skilladvisor.SearchFunc(func(ctx context.Context, query skillmarket.SearchQuery) (*skillmarket.SearchResponse, error) {
		return &skillmarket.SearchResponse{
			Skills: []skillmarket.SearchResult{{
				Skill: skillmarket.SkillDocument{
					ID:            "gh-release-bot",
					Name:          "GitHub Release Bot",
					Description:   "Automate GitHub releases and changelog generation.",
					Installable:   true,
					SecurityBadge: skillmarket.BadgeGreen,
					RiskLevel:     skillmarket.RiskLow,
					Tags:          []string{"github-actions", "release", "changelog"},
				},
				Score: 24,
			}},
			Total: 1,
		}, nil
	})))

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/skills/advise", strings.NewReader(`{"query":"帮我做 GitHub Actions 自动发版"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.MarketAdviseSkills(c); err != nil {
		t.Fatalf("MarketAdviseSkills error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if selector.called != 1 {
		t.Fatalf("selector called %d times, want 1", selector.called)
	}
	if selector.query != "帮我做 GitHub Actions 自动发版" {
		t.Fatalf("selector query = %q", selector.query)
	}
	if selector.opts.Mode != agentcore.SkillSelectorModeHybrid || !selector.opts.EnableRerank || selector.opts.ConfidenceThreshold != 0.88 {
		t.Fatalf("unexpected selector opts: %+v", selector.opts)
	}

	var body struct {
		Query             string              `json:"query"`
		InstalledDecision *agentcore.Decision `json:"installed_decision"`
		NeedStoreSearch   bool                `json:"need_store_search"`
		RecommendedIDs    []string            `json:"recommended_ids"`
		SearchQueries     []string            `json:"search_queries"`
		SearchError       string              `json:"search_error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Query != "帮我做 GitHub Actions 自动发版" {
		t.Fatalf("query = %q", body.Query)
	}
	if body.SearchError != "" {
		t.Fatalf("unexpected search_error: %q", body.SearchError)
	}
	if body.InstalledDecision == nil || !body.InstalledDecision.NeedClarify {
		t.Fatalf("expected installed decision from selector, got %+v", body.InstalledDecision)
	}
	if !body.NeedStoreSearch {
		t.Fatalf("expected need_store_search=true, got false")
	}
	if len(body.RecommendedIDs) == 0 || body.RecommendedIDs[0] != "gh-release-bot" {
		t.Fatalf("recommended_ids = %+v, want gh-release-bot", body.RecommendedIDs)
	}
	if len(body.SearchQueries) == 0 {
		t.Fatalf("expected expanded search queries in response")
	}
}

func TestMarketAdviseSkillsHonorsProvidedInstalledDecision(t *testing.T) {
	searchCalls := 0
	handler := NewSkillHandler(skill.NewRegistry())
	handler.SetSkillAdvisor(skilladvisor.NewService(skilladvisor.SearchFunc(func(ctx context.Context, query skillmarket.SearchQuery) (*skillmarket.SearchResponse, error) {
		searchCalls++
		return &skillmarket.SearchResponse{}, nil
	})))

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/skills/advise", strings.NewReader(`{
		"query":"帮我搜索最新新闻",
		"installed_decision":{
			"query":"帮我搜索最新新闻",
			"selected_skill":"web_search",
			"confidence":0.93,
			"need_clarify":false,
			"reason":"strong_match",
			"stage":"ir"
		}
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.MarketAdviseSkills(c); err != nil {
		t.Fatalf("MarketAdviseSkills error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if searchCalls != 0 {
		t.Fatalf("searchCalls = %d, want 0", searchCalls)
	}

	var body struct {
		NeedStoreSearch   bool                `json:"need_store_search"`
		Reason            string              `json:"reason"`
		InstalledDecision *agentcore.Decision `json:"installed_decision"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.NeedStoreSearch {
		t.Fatalf("expected need_store_search=false, got true")
	}
	if body.Reason != "installed_skill_is_sufficient" {
		t.Fatalf("reason = %q, want installed_skill_is_sufficient", body.Reason)
	}
	if body.InstalledDecision == nil || body.InstalledDecision.SelectedSkill != "web_search" {
		t.Fatalf("expected provided installed decision to round-trip, got %+v", body.InstalledDecision)
	}
}

func TestMarketSearchSkillsLazyFactoryInitializesOnce(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "market.db")

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	activeDir := filepath.Join(tempDir, "active")
	cfg := testSkillMarketConfig(tempDir, activeDir)

	seedMarket, err := skillmarket.NewService(db, skillmarket.Options{
		Config:       cfg,
		Registry:     skill.NewRegistry(),
		LocalScanner: skillstore.NewLocalSkillScanner(activeDir),
		Scanner:      skillmarket.NewScanner(nil),
	})
	if err != nil {
		t.Fatalf("seed market: %v", err)
	}

	raw := `---
id: lazy-market-skill
name: Lazy Market Skill
version: 1.0.0
description: Lazy init fixture
---
Search fixture content.`
	if err := seedMarket.Store().UpsertSkill(context.Background(),
		&skillmarket.SkillDocument{
			ID:            "lazy-market-skill",
			Slug:          "lazy-market-skill",
			Name:          "Lazy Market Skill",
			Description:   "Lazy init fixture",
			Category:      "development",
			LatestVersion: "1.0.0",
			RiskLevel:     skillmarket.RiskLow,
			SecurityScore: 91,
			Published:     true,
			SkillContent:  raw,
		},
		&skillmarket.SkillVersion{
			ID:           "lazy-market-skill-version",
			SkillID:      "lazy-market-skill",
			Version:      "1.0.0",
			Checksum:     checksum(raw),
			RawSkillMD:   raw,
			ManifestJSON: `{"id":"lazy-market-skill","name":"Lazy Market Skill","version":"1.0.0"}`,
		},
		&skillmarket.SecurityReport{
			ID:             "lazy-market-skill-report",
			SkillID:        "lazy-market-skill",
			Version:        "1.0.0",
			Score:          91,
			RiskLevel:      skillmarket.RiskLow,
			ScannerVersion: skillmarket.ScannerVersion,
			LLMStatus:      "skipped",
		},
	); err != nil {
		t.Fatalf("upsert lazy fixture: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close seed db: %v", err)
	}

	handler := NewSkillHandler(skill.NewRegistry())
	defer handler.Close()

	var factoryCalls int
	handler.SetMarketplaceFactory(func() (*skillmarket.Service, error) {
		factoryCalls++
		return skillmarket.NewServiceWithDBPath(dbPath, skillmarket.Options{
			Config:       cfg,
			Registry:     skill.NewRegistry(),
			LocalScanner: skillstore.NewLocalSkillScanner(activeDir),
			Scanner:      skillmarket.NewScanner(nil),
		})
	})

	if factoryCalls != 0 {
		t.Fatalf("factory called before request: %d", factoryCalls)
	}

	e := echo.New()
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/skills/search?q=lazy", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		if err := handler.MarketSearchSkills(c); err != nil {
			t.Fatalf("search handler run %d: %v", i+1, err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("run %d status = %d, want %d", i+1, rec.Code, http.StatusOK)
		}
	}

	if factoryCalls != 1 {
		t.Fatalf("factoryCalls = %d, want 1", factoryCalls)
	}
}

func TestMarketSearchSkillsFallbackUsesLegacyStore(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "legacy-skill-store.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	store, err := skillstore.NewStore(db)
	if err != nil {
		t.Fatalf("new legacy store: %v", err)
	}
	if err := store.UpsertSkill(context.Background(), &skillstore.Skill{
		ID:            "git-expert",
		Name:          "Git Expert",
		Version:       "1.0.0",
		Summary:       "Git helper",
		Description:   "Git helper",
		Author:        "tester",
		Category:      "development",
		Tags:          "git,rebase",
		SourceID:      "clawhub",
		SourceName:    "ClawHub",
		Homepage:      "https://example.com/git-expert",
		DownloadURL:   "https://example.com/git-expert/SKILL.md",
		Stars:         10,
		Downloads:     200,
		SearchContent: "git rebase helper",
	}); err != nil {
		t.Fatalf("upsert legacy skill: %v", err)
	}

	handler := NewSkillHandler(skill.NewRegistry())
	handler.SetStore(store)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/skills/search?q=rebase", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.MarketSearchSkills(c); err != nil {
		t.Fatalf("fallback search handler: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var payload struct {
		Skills []struct {
			Skill RemoteSkill `json:"skill"`
		} `json:"skills"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode fallback payload: %v", err)
	}
	if payload.Total == 0 || len(payload.Skills) == 0 {
		t.Fatalf("expected fallback search results, payload=%s", rec.Body.String())
	}
	if payload.Skills[0].Skill.ID != "git-expert" {
		t.Fatalf("top fallback skill = %q, want git-expert", payload.Skills[0].Skill.ID)
	}
	if !payload.Skills[0].Skill.Installable {
		t.Fatalf("expected fallback skill to be installable, payload=%+v", payload.Skills[0].Skill)
	}
}

func TestMarketSearchSkillsFallbackPaginatesBeyondFirstHundred(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "legacy-skill-store-paginated.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	store, err := skillstore.NewStore(db)
	if err != nil {
		t.Fatalf("new legacy store: %v", err)
	}
	for i := 1; i <= 150; i++ {
		id := fmt.Sprintf("skill-%03d", i)
		if err := store.UpsertSkill(context.Background(), &skillstore.Skill{
			ID:            id,
			Name:          fmt.Sprintf("Skill %03d", i),
			Version:       "1.0.0",
			Summary:       "Pagination fixture",
			Description:   "Pagination fixture",
			Author:        "tester",
			Category:      "development",
			Tags:          "pagination",
			SourceID:      "clawhub",
			SourceName:    "ClawHub",
			Homepage:      "https://example.com/" + id,
			DownloadURL:   "https://example.com/" + id + "/SKILL.md",
			Stars:         i,
			Downloads:     1000 - i,
			SearchContent: "pagination fixture",
		}); err != nil {
			t.Fatalf("upsert legacy skill %s: %v", id, err)
		}
	}

	handler := NewSkillHandler(skill.NewRegistry())
	handler.SetStore(store)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/skills/search", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	results, err := handler.collectFallbackMarketResults(c.Request().Context(), skillmarket.SearchQuery{})
	if err != nil {
		t.Fatalf("collectFallbackMarketResults() error = %v", err)
	}
	if len(results) != 150 {
		t.Fatalf("len(results) = %d, want 150", len(results))
	}
}

func TestMarketFeaturedAndFilters(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "market.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	activeDir := filepath.Join(t.TempDir(), "active")
	cfg := testSkillMarketConfig(t.TempDir(), activeDir)
	market, err := skillmarket.NewService(db, skillmarket.Options{
		Config:       cfg,
		Registry:     skill.NewRegistry(),
		LocalScanner: skillstore.NewLocalSkillScanner(activeDir),
		Scanner:      skillmarket.NewScanner(nil),
	})
	if err != nil {
		t.Fatalf("new market: %v", err)
	}

	raw := `---
id: featured-skill
name: Featured Skill
version: 1.0.0
description: Featured fixture
category: development
---
Featured fixture.`
	if err := market.Store().UpsertSkill(context.Background(),
		&skillmarket.SkillDocument{
			ID:            "featured-skill",
			Slug:          "featured-skill",
			Name:          "Featured Skill",
			Description:   "Featured fixture",
			Category:      "development",
			LatestVersion: "1.0.0",
			RiskLevel:     skillmarket.RiskLow,
			SecurityBadge: skillmarket.BadgeGreen,
			SecurityScore: 96,
			Installable:   true,
			InstallType:   skillmarket.InstallTypeGitRepo,
			ArtifactKind:  skillmarket.ArtifactKindOpenSource,
			Published:     true,
			SkillContent:  raw,
			SourceName:    "SkillHub Club",
			SourceGroup:   "skillhub",
		},
		&skillmarket.SkillVersion{
			ID:           "featured-skill-version",
			SkillID:      "featured-skill",
			Version:      "1.0.0",
			Checksum:     checksum(raw),
			RawSkillMD:   raw,
			ManifestJSON: `{"id":"featured-skill","name":"Featured Skill","version":"1.0.0"}`,
		},
		&skillmarket.SecurityReport{
			ID:                  "featured-skill-report",
			SkillID:             "featured-skill",
			Version:             "1.0.0",
			Score:               96,
			RiskLevel:           skillmarket.RiskLow,
			SecurityBadge:       skillmarket.BadgeGreen,
			VulnerabilityStatus: skillmarket.VulnerabilityStatusNotApplicable,
			ScannerVersion:      skillmarket.ScannerVersion,
			LLMStatus:           "skipped",
		},
	); err != nil {
		t.Fatalf("upsert skill: %v", err)
	}
	if err := market.Store().ReplaceCurations(context.Background(), []skillmarket.SkillCuration{
		{SkillID: "featured-skill", FeaturedRank: 1, Label: "Featured", Reason: "Curated for tests"},
	}); err != nil {
		t.Fatalf("replace curations: %v", err)
	}

	handler := NewSkillHandler(skill.NewRegistry())
	handler.SetMarketplace(market)
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/skills/featured", nil)
	rec := httptest.NewRecorder()
	if err := handler.MarketFeaturedSkills(e.NewContext(req, rec)); err != nil {
		t.Fatalf("featured handler: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("featured status = %d, want %d", rec.Code, http.StatusOK)
	}
	var featured struct {
		Skills []skillmarket.SkillDocument `json:"skills"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &featured); err != nil {
		t.Fatalf("decode featured: %v", err)
	}
	if len(featured.Skills) != 1 || featured.Skills[0].ID != "featured-skill" {
		t.Fatalf("featured payload = %#v, want featured-skill", featured.Skills)
	}

	req = httptest.NewRequest(http.MethodGet, "/skills/filters", nil)
	rec = httptest.NewRecorder()
	if err := handler.MarketFilters(e.NewContext(req, rec)); err != nil {
		t.Fatalf("filters handler: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("filters status = %d, want %d", rec.Code, http.StatusOK)
	}
	var filters skillmarket.SkillFilters
	if err := json.Unmarshal(rec.Body.Bytes(), &filters); err != nil {
		t.Fatalf("decode filters: %v", err)
	}
	if len(filters.Categories) == 0 {
		t.Fatal("expected category filters to be populated")
	}
	if len(filters.Sources) == 0 {
		t.Fatal("expected source filters to be populated")
	}
}

func TestMarketDiscoverSkillsStartsAsyncAndReportsStatus(t *testing.T) {
	requestStarted := make(chan struct{}, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	closeRelease := func() {
		releaseOnce.Do(func() {
			close(release)
		})
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/skills":
			select {
			case requestStarted <- struct{}{}:
			default:
			}
			<-release
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"total":0,"skills":[]}}`))
		case "/search/code":
			_, _ = w.Write([]byte(`{"items":[]}`))
		case "/api/v1/skills":
			_, _ = w.Write([]byte(`{"items":[]}`))
		case "/":
			_, _ = w.Write([]byte(`<html><body>empty catalog</body></html>`))
		default:
			http.NotFound(w, r)
			return
		}
	}))
	defer server.Close()
	defer closeRelease()

	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "market.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	activeDir := filepath.Join(t.TempDir(), "active")
	cfg := testSkillMarketConfig(t.TempDir(), activeDir)
	cfg.TencentSkillHubAPIBaseURL = server.URL
	cfg.GitHubAPIBaseURL = server.URL
	cfg.ClawHubBaseURL = server.URL
	cfg.SkillHubBaseURL = server.URL
	cfg.SkillStackBaseURL = server.URL
	cfg.SkillsMPBaseURL = server.URL
	cfg.LLMSkillsBaseURL = server.URL
	cfg.SeedURLs = nil
	market, err := skillmarket.NewService(db, skillmarket.Options{
		Config:       cfg,
		Registry:     skill.NewRegistry(),
		LocalScanner: skillstore.NewLocalSkillScanner(activeDir),
		HTTPClient:   server.Client(),
		Scanner:      skillmarket.NewScanner(nil),
	})
	if err != nil {
		t.Fatalf("new market: %v", err)
	}
	defer market.Close()
	if _, err := db.Exec(`UPDATE skill_sources SET enabled = 0`); err != nil {
		t.Fatalf("disable default sources: %v", err)
	}
	if err := market.Store().UpsertSource(context.Background(), skillmarket.Source{
		ID:          "tencent-skillhub",
		Type:        "lightmake_api",
		BaseURL:     server.URL,
		DisplayName: "Tencent SkillHub",
		SourceGroup: "skillhub",
		Enabled:     true,
		Priority:    5,
	}); err != nil {
		t.Fatalf("UpsertSource(tencent-skillhub) error = %v", err)
	}

	handler := NewSkillHandler(skill.NewRegistry())
	handler.SetMarketplace(market)
	e := echo.New()

	req := httptest.NewRequest(http.MethodPost, "/skills/discover/refresh", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := handler.MarketDiscoverSkills(c); err != nil {
		t.Fatalf("MarketDiscoverSkills() error = %v", err)
	}
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}
	var startPayload struct {
		Accepted bool `json:"accepted"`
		Running  bool `json:"running"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &startPayload); err != nil {
		t.Fatalf("decode start payload: %v", err)
	}
	if !startPayload.Accepted || !startPayload.Running {
		t.Fatalf("unexpected start payload: %s", rec.Body.String())
	}

	select {
	case <-requestStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for discover request to reach upstream source")
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/skills/discover/status", nil)
	var statusPayload struct {
		Running          bool   `json:"running"`
		TotalSources     int    `json:"total_sources"`
		ProcessedSources int    `json:"processed_sources"`
		CurrentSourceID  string `json:"current_source_id"`
		SourceResults    []struct {
			SourceID string `json:"source_id"`
			Status   string `json:"status"`
		} `json:"source_results"`
	}
	waitDeadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(waitDeadline) {
		statusRec := httptest.NewRecorder()
		statusCtx := e.NewContext(statusReq, statusRec)
		if err := handler.MarketDiscoverStatus(statusCtx); err != nil {
			t.Fatalf("MarketDiscoverStatus() error = %v", err)
		}
		if statusRec.Code != http.StatusOK {
			t.Fatalf("status code = %d, want %d", statusRec.Code, http.StatusOK)
		}
		if err := json.Unmarshal(statusRec.Body.Bytes(), &statusPayload); err != nil {
			t.Fatalf("decode status payload: %v", err)
		}
		if statusPayload.Running && statusPayload.TotalSources == 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !statusPayload.Running {
		t.Fatalf("expected running status, payload=%+v", statusPayload)
	}
	if statusPayload.TotalSources != 1 {
		t.Fatalf("total_sources = %d, want 1", statusPayload.TotalSources)
	}
	if statusPayload.ProcessedSources != 0 {
		t.Fatalf("processed_sources = %d, want 0", statusPayload.ProcessedSources)
	}
	if statusPayload.CurrentSourceID != "tencent-skillhub" {
		t.Fatalf("current_source_id = %q, want %q", statusPayload.CurrentSourceID, "tencent-skillhub")
	}
	if len(statusPayload.SourceResults) != 1 || statusPayload.SourceResults[0].SourceID != "tencent-skillhub" {
		t.Fatalf("source_results = %+v, want tencent-skillhub entry", statusPayload.SourceResults)
	}

	closeRelease()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		statusRec := httptest.NewRecorder()
		statusCtx := e.NewContext(statusReq, statusRec)
		if err := handler.MarketDiscoverStatus(statusCtx); err != nil {
			t.Fatalf("MarketDiscoverStatus() error = %v", err)
		}
		if err := json.Unmarshal(statusRec.Body.Bytes(), &statusPayload); err != nil {
			t.Fatalf("decode final status payload: %v", err)
		}
		if !statusPayload.Running {
			if statusPayload.ProcessedSources != 1 {
				t.Fatalf("processed_sources = %d, want 1", statusPayload.ProcessedSources)
			}
			if len(statusPayload.SourceResults) != 1 || statusPayload.SourceResults[0].Status != "success" {
				t.Fatalf("final source_results = %+v, want successful tencent-skillhub entry", statusPayload.SourceResults)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("timed out waiting for discover status to complete")
}

func TestMarketFeaturedAndFiltersFallback(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "legacy-featured.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	store, err := skillstore.NewStore(db)
	if err != nil {
		t.Fatalf("new legacy store: %v", err)
	}
	if err := store.UpsertSkill(context.Background(), &skillstore.Skill{
		ID:            "featured-skill",
		Name:          "Featured Skill",
		Version:       "1.0.0",
		Summary:       "Featured fallback fixture",
		Description:   "Featured fallback fixture",
		Author:        "tester",
		Category:      "development",
		Tags:          "featured",
		SourceID:      "skillhub",
		SourceName:    "SkillHub Club",
		Homepage:      "https://example.com/featured-skill",
		DownloadURL:   "https://example.com/featured-skill/SKILL.md",
		Stars:         12,
		Downloads:     320,
		SearchContent: "featured fallback fixture",
	}); err != nil {
		t.Fatalf("upsert legacy skill: %v", err)
	}

	handler := NewSkillHandler(skill.NewRegistry())
	handler.SetStore(store)
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/skills/featured", nil)
	rec := httptest.NewRecorder()
	if err := handler.MarketFeaturedSkills(e.NewContext(req, rec)); err != nil {
		t.Fatalf("fallback featured handler: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("featured status = %d, want %d", rec.Code, http.StatusOK)
	}
	var featured struct {
		Skills []RemoteSkill `json:"skills"`
		Count  int           `json:"count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &featured); err != nil {
		t.Fatalf("decode fallback featured: %v", err)
	}
	if featured.Count != 1 || len(featured.Skills) != 1 || featured.Skills[0].ID != "featured-skill" {
		t.Fatalf("featured payload = %+v", featured)
	}

	req = httptest.NewRequest(http.MethodGet, "/skills/filters", nil)
	rec = httptest.NewRecorder()
	if err := handler.MarketFilters(e.NewContext(req, rec)); err != nil {
		t.Fatalf("fallback filters handler: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("filters status = %d, want %d", rec.Code, http.StatusOK)
	}
	var filters skillmarket.SkillFilters
	if err := json.Unmarshal(rec.Body.Bytes(), &filters); err != nil {
		t.Fatalf("decode fallback filters: %v", err)
	}
	if len(filters.Categories) == 0 {
		t.Fatal("expected fallback category filters to be populated")
	}
	if len(filters.Sources) == 0 {
		t.Fatal("expected fallback source filters to be populated")
	}
}

func TestMarketInstallRequiresAckForYellowSkill(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "market.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	activeDir := filepath.Join(t.TempDir(), "active")
	cfg := testSkillMarketConfig(t.TempDir(), activeDir)
	market, err := skillmarket.NewService(db, skillmarket.Options{
		Config:       cfg,
		Registry:     skill.NewRegistry(),
		LocalScanner: skillstore.NewLocalSkillScanner(activeDir),
		Scanner:      skillmarket.NewScanner(nil),
	})
	if err != nil {
		t.Fatalf("new market: %v", err)
	}

	raw := `---
id: yellow-skill
name: Yellow Skill
version: 1.0.0
description: Medium risk fixture
---
This skill can read files and call APIs.`
	if err := market.Store().UpsertSkill(context.Background(),
		&skillmarket.SkillDocument{
			ID:            "yellow-skill",
			Slug:          "yellow-skill",
			Name:          "Yellow Skill",
			Description:   "Medium risk fixture",
			Category:      "development",
			LatestVersion: "1.0.0",
			RiskLevel:     skillmarket.RiskMedium,
			SecurityBadge: skillmarket.BadgeYellow,
			SecurityScore: 72,
			Installable:   true,
			InstallType:   skillmarket.InstallTypeScriptPackage,
			ArtifactKind:  skillmarket.ArtifactKindOpenSource,
			Published:     true,
			SkillContent:  raw,
		},
		&skillmarket.SkillVersion{
			ID:           "yellow-skill-version",
			SkillID:      "yellow-skill",
			Version:      "1.0.0",
			Checksum:     checksum(raw),
			RawSkillMD:   raw,
			ManifestJSON: `{"id":"yellow-skill","name":"Yellow Skill","version":"1.0.0"}`,
		},
		&skillmarket.SecurityReport{
			ID:                  "yellow-skill-report",
			SkillID:             "yellow-skill",
			Version:             "1.0.0",
			Score:               72,
			RiskLevel:           skillmarket.RiskMedium,
			SecurityBadge:       skillmarket.BadgeYellow,
			VulnerabilityStatus: skillmarket.VulnerabilityStatusUnknown,
			InstallSurface: skillmarket.InstallSurface{
				InstallType:  skillmarket.InstallTypeScriptPackage,
				ArtifactKind: skillmarket.ArtifactKindOpenSource,
				Installable:  true,
				HasScripts:   true,
			},
			ScannerVersion: skillmarket.ScannerVersion,
			LLMStatus:      "skipped",
		},
	); err != nil {
		t.Fatalf("upsert skill: %v", err)
	}

	handler := NewSkillHandler(skill.NewRegistry())
	handler.SetMarketplace(market)
	e := echo.New()

	req := httptest.NewRequest(http.MethodPost, "/skills/install", strings.NewReader(`{"id":"yellow-skill"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	if err := handler.MarketInstallSkill(e.NewContext(req, rec)); err != nil {
		t.Fatalf("install handler: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("install status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	req = httptest.NewRequest(http.MethodPost, "/skills/install?ack_risk=true", strings.NewReader(`{"id":"yellow-skill"}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	if err := handler.MarketInstallSkill(e.NewContext(req, rec)); err != nil {
		t.Fatalf("install with ack handler: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("install with ack status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestGetSkillFallbackIncludesMarketplaceCompatibilityFields(t *testing.T) {
	registry := skill.NewRegistry()
	handler := NewSkillHandler(registry)

	testSkill := NewRemoteSkillAdapter(&skill.Manifest{
		ID:          "compat-skill",
		Name:        "Compat Skill",
		Version:     "2.0.0",
		Description: "Compatibility fixture",
		Author:      "tester",
		Category:    "development",
		Tags:        []string{"compat"},
	})
	registry.Register(testSkill, false)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/skills/compat-skill", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("compat-skill")

	if err := handler.GetSkill(c); err != nil {
		t.Fatalf("GetSkill fallback handler: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode fallback detail: %v", err)
	}
	if payload["id"] != "compat-skill" {
		t.Fatalf("top-level id = %#v, want compat-skill", payload["id"])
	}
	nested, ok := payload["skill"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected nested skill payload, body=%s", rec.Body.String())
	}
	if nested["id"] != "compat-skill" {
		t.Fatalf("nested skill id = %#v, want compat-skill", nested["id"])
	}
}

func checksum(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
