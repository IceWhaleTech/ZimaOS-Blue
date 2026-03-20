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
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	_ "github.com/mattn/go-sqlite3"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmarket"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillstore"
)

func TestMarketSearchSkills(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "market.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	activeDir := filepath.Join(t.TempDir(), "active")
	market, err := skillmarket.NewService(db, skillmarket.Options{
		Config:       skillmarket.DefaultConfig(t.TempDir(), activeDir),
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
	market, err := skillmarket.NewService(db, skillmarket.Options{
		Config:       skillmarket.DefaultConfig(t.TempDir(), activeDir),
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

	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "market.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	activeDir := filepath.Join(t.TempDir(), "active")
	cfg := skillmarket.DefaultConfig(t.TempDir(), activeDir)
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

	<-requestStarted

	statusReq := httptest.NewRequest(http.MethodGet, "/skills/discover/status", nil)
	statusRec := httptest.NewRecorder()
	statusCtx := e.NewContext(statusReq, statusRec)
	if err := handler.MarketDiscoverStatus(statusCtx); err != nil {
		t.Fatalf("MarketDiscoverStatus() error = %v", err)
	}
	if statusRec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", statusRec.Code, http.StatusOK)
	}
	var statusPayload struct {
		Running          bool   `json:"running"`
		TotalSources     int    `json:"total_sources"`
		ProcessedSources int    `json:"processed_sources"`
		CurrentSourceID  string `json:"current_source_id"`
	}
	if err := json.Unmarshal(statusRec.Body.Bytes(), &statusPayload); err != nil {
		t.Fatalf("decode status payload: %v", err)
	}
	if !statusPayload.Running {
		t.Fatalf("expected running status, payload=%s", statusRec.Body.String())
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

	close(release)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		statusRec = httptest.NewRecorder()
		statusCtx = e.NewContext(statusReq, statusRec)
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
	market, err := skillmarket.NewService(db, skillmarket.Options{
		Config:       skillmarket.DefaultConfig(t.TempDir(), activeDir),
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
