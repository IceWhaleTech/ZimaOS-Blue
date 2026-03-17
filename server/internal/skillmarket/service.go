package skillmarket

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/crawler"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/embedding"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

type Options struct {
	Config            Config
	Logger            *zap.Logger
	Registry          *skill.Registry
	LocalScanner      *skillstore.LocalSkillScanner
	EmbeddingProvider embedding.Provider
	HTTPClient        *http.Client
	Scanner           *Scanner
}

type Service struct {
	store             *Store
	cfg               Config
	logger            *zap.Logger
	registry          *skill.Registry
	localScanner      *skillstore.LocalSkillScanner
	embeddingProvider embedding.Provider
	httpClient        *http.Client
	scanner           *Scanner
	stopOnce          sync.Once
	stopCh            chan struct{}
}

func NewService(db *sql.DB, opts Options) (*Service, error) {
	store, err := NewStore(db)
	if err != nil {
		return nil, err
	}
	cfg := opts.Config
	if cfg.SearchCandidateLimit <= 0 {
		cfg.SearchCandidateLimit = DefaultSearchLimit
	}
	if cfg.SemanticRatio <= 0 {
		cfg.SemanticRatio = 0.35
	}
	svc := &Service{
		store:             store,
		cfg:               cfg,
		logger:            opts.Logger,
		registry:          opts.Registry,
		localScanner:      opts.LocalScanner,
		embeddingProvider: opts.EmbeddingProvider,
		httpClient:        opts.HTTPClient,
		scanner:           opts.Scanner,
		stopCh:            make(chan struct{}),
	}
	if svc.httpClient == nil {
		svc.httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	if svc.scanner == nil {
		svc.scanner = NewScanner(nil)
	}
	if err := svc.ensureDefaultSources(context.Background()); err != nil {
		return nil, err
	}
	_ = svc.syncCurations(context.Background())
	return svc, nil
}

func (s *Service) Store() *Store {
	return s.store
}

func (s *Service) Start(ctx context.Context) {
	go func() {
		if err := s.syncCurations(ctx); err != nil && s.logger != nil {
			s.logger.Warn("skillmarket curation sync failed", zap.Error(err))
		}
	}()
	if s.cfg.CrawlIncrementalInterval > 0 {
		go s.runPeriodic(ctx, s.cfg.CrawlIncrementalInterval, func(runCtx context.Context) {
			if _, err := s.Discover(runCtx); err != nil && s.logger != nil {
				s.logger.Warn("skillmarket discover failed", zap.Error(err))
			}
		})
	}
	if s.cfg.CrawlFullInterval > 0 {
		go s.runPeriodic(ctx, s.cfg.CrawlFullInterval, func(runCtx context.Context) {
			if err := s.syncCurations(runCtx); err != nil && s.logger != nil {
				s.logger.Warn("skillmarket curation refresh failed", zap.Error(err))
			}
		})
	}
	if s.cfg.UpdateCheckInterval > 0 {
		go s.runPeriodic(ctx, s.cfg.UpdateCheckInterval, func(runCtx context.Context) {
			if _, err := s.CheckForUpdates(runCtx, true); err != nil && s.logger != nil {
				s.logger.Warn("skillmarket update check failed", zap.Error(err))
			}
		})
	}
}

func (s *Service) runPeriodic(ctx context.Context, interval time.Duration, fn func(context.Context)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			fn(ctx)
		}
	}
}

func (s *Service) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
}

func (s *Service) Search(ctx context.Context, query SearchQuery) (*SearchResponse, error) {
	response, err := s.store.Search(ctx, query)
	if err != nil {
		return nil, err
	}
	if !query.Semantic || s.embeddingProvider == nil || len(response.Skills) == 0 {
		return response, nil
	}
	queryVector, err := s.embeddingProvider.Embed(ctx, query.Query)
	if err != nil || len(queryVector) == 0 {
		return response, nil
	}
	response.Skills = s.store.RerankSemantic(queryVector, response.Skills)
	return response, nil
}

func (s *Service) Trending(ctx context.Context, category string, limit int) ([]SkillDocument, error) {
	return s.store.ListTrending(ctx, category, limit)
}

func (s *Service) Featured(ctx context.Context, category, source string, limit int) ([]SkillDocument, error) {
	return s.store.ListFeatured(ctx, category, source, limit)
}

func (s *Service) Filters(ctx context.Context) (*SkillFilters, error) {
	return s.store.GetFilters(ctx)
}

func (s *Service) GetSkill(ctx context.Context, id string) (*SkillDetail, error) {
	return s.store.GetSkill(ctx, id)
}

func (s *Service) GetSecurity(ctx context.Context, id, version string) (*SecurityReport, error) {
	if strings.TrimSpace(version) == "" {
		latest, err := s.store.GetLatestVersion(ctx, id)
		if err != nil {
			return nil, err
		}
		if latest == nil {
			return nil, nil
		}
		version = latest.Version
	}
	return s.store.GetSecurityReport(ctx, id, version)
}

func (s *Service) ListInstalled(ctx context.Context) ([]InstalledSkill, error) {
	items, err := s.store.ListInstalledSkills(ctx)
	if err != nil {
		return nil, err
	}
	for i := range items {
		_ = s.store.RecordTelemetry(ctx, items[i].SkillID, "active")
	}
	return items, nil
}

func (s *Service) ListUpdates(ctx context.Context) ([]AvailableUpdate, error) {
	return s.store.ListUpdates(ctx)
}

func (s *Service) Install(ctx context.Context, req InstallRequest) (*InstallResult, error) {
	if strings.TrimSpace(req.ID) == "" && strings.TrimSpace(req.GitHub) == "" {
		return nil, fmt.Errorf("skill id or github repo is required")
	}
	if strings.TrimSpace(req.ID) != "" {
		validatedID, err := ValidateSkillID(req.ID)
		if err != nil {
			return nil, err
		}
		req.ID = validatedID
	}

	if req.GitHub != "" {
		return s.installFromGitHub(ctx, req.GitHub, req.AckRisk)
	}

	detail, err := s.store.GetSkill(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if detail == nil || detail.Version == nil {
		return nil, fmt.Errorf("skill not found: %s", req.ID)
	}
	version := detail.Version
	if req.Version != "" && req.Version != version.Version {
		version, err = s.store.GetSkillVersion(ctx, req.ID, req.Version)
		if err != nil {
			return nil, err
		}
		if version == nil {
			return nil, fmt.Errorf("version not found: %s@%s", req.ID, req.Version)
		}
	}
	if !detail.Skill.Installable {
		return nil, fmt.Errorf("skill is catalog-only and cannot be installed automatically")
	}
	report, err := s.store.GetSecurityReport(ctx, req.ID, version.Version)
	if err != nil {
		return nil, err
	}
	if report != nil && (report.RiskLevel == RiskCritical || report.RiskLevel == RiskHigh || report.SecurityBadge == BadgeRed) {
		return nil, fmt.Errorf("installation blocked by security policy: %s risk", report.RiskLevel)
	}
	if report != nil && report.SecurityBadge == BadgeYellow && !req.AckRisk {
		return nil, fmt.Errorf("installation requires risk acknowledgement")
	}

	cacheDir := filepath.Join(s.cfg.CacheRoot, req.ID, version.Version)
	activeDir := filepath.Join(s.cfg.ActiveSkillsDir, req.ID)
	tempDir := filepath.Join(os.TempDir(), "skillmarket-"+uuid.NewString())
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)

	warnings := []string{}
	if report != nil && report.SecurityBadge == BadgeYellow {
		warnings = append(warnings, "Skill requires medium-risk permissions. Review the security report before enabling auto-update.")
	}

	if err := s.materializeVersion(tempDir, detail.Skill, version); err != nil {
		return nil, err
	}
	if err := s.verifyInstallPayload(version.Checksum, filepath.Join(tempDir, "SKILL.md")); err != nil {
		return nil, err
	}
	if report == nil {
		recomputed, err := s.scanInstalledPayload(ctx, req.ID, version.Version, tempDir, nil)
		if err != nil {
			return nil, err
		}
		report = recomputed
		if report.SecurityBadge == BadgeRed || report.RiskLevel == RiskCritical || report.RiskLevel == RiskHigh {
			return nil, fmt.Errorf("installation blocked by security policy: %s risk", report.RiskLevel)
		}
		if report.SecurityBadge == BadgeYellow && !req.AckRisk {
			return nil, fmt.Errorf("installation requires risk acknowledgement")
		}
	}
	if err := s.promoteInstall(tempDir, cacheDir, activeDir); err != nil {
		return nil, err
	}
	if err := s.registerInstalledSkill(ctx, detail.Skill, version, report); err != nil {
		return nil, err
	}
	_ = s.store.RecordTelemetry(ctx, req.ID, "download")
	_ = s.store.RecordTelemetry(ctx, req.ID, "install")

	return &InstallResult{
		SkillID:     req.ID,
		Version:     version.Version,
		Path:        activeDir,
		CachePath:   cacheDir,
		Warnings:    warnings,
		Security:    report,
		InstalledAt: timeutil.NowTime(),
	}, nil
}

func (s *Service) Uninstall(ctx context.Context, skillID string) error {
	validatedID, err := ValidateSkillID(skillID)
	if err != nil {
		return err
	}
	skillID = validatedID

	activeDir := filepath.Join(s.cfg.ActiveSkillsDir, skillID)
	cacheRoot := filepath.Join(s.cfg.CacheRoot, skillID)
	_ = os.RemoveAll(activeDir)
	_ = os.RemoveAll(cacheRoot)
	if s.registry != nil {
		_ = s.registry.Unregister(skillID)
	}
	if s.localScanner != nil {
		_ = s.localScanner.Scan()
	}
	return s.store.RemoveInstalledSkill(ctx, skillID)
}

func (s *Service) Update(ctx context.Context, skillID string) (*InstallResult, error) {
	return s.Install(ctx, InstallRequest{ID: skillID})
}

func (s *Service) CheckForUpdates(ctx context.Context, apply bool) ([]AvailableUpdate, error) {
	installed, err := s.store.ListInstalledSkills(ctx)
	if err != nil {
		return nil, err
	}
	var updates []AvailableUpdate
	for _, item := range installed {
		latest, err := s.store.GetLatestVersion(ctx, item.SkillID)
		if err != nil || latest == nil {
			continue
		}
		action := "up_to_date"
		if latest.Version != item.InstalledVersion || latest.Checksum != item.Checksum {
			action = "available"
			report, _ := s.store.GetSecurityReport(ctx, item.SkillID, latest.Version)
			if report != nil && (report.RiskLevel == RiskHigh || report.RiskLevel == RiskCritical) {
				action = "blocked_risk"
			} else if apply && item.AutoUpdate {
				if _, err := s.Install(ctx, InstallRequest{ID: item.SkillID, Version: latest.Version, AckRisk: true}); err == nil {
					action = "auto_updated"
				}
			}
		}
		update := AvailableUpdate{
			SkillID:         item.SkillID,
			CurrentVersion:  item.InstalledVersion,
			LatestVersion:   latest.Version,
			CurrentChecksum: item.Checksum,
			LatestChecksum:  latest.Checksum,
			Action:          action,
			CheckedAt:       timeutil.NowTime(),
		}
		_ = s.store.RecordUpdateCheck(ctx, update)
		if action != "up_to_date" {
			updates = append(updates, update)
		}
	}
	return updates, nil
}

func (s *Service) Discover(ctx context.Context) (*DiscoverResult, error) {
	_ = s.syncCurations(ctx)
	sources, err := s.store.ListSources(ctx)
	if err != nil {
		return nil, err
	}
	result := &DiscoverResult{SourcesProcessed: len(sources)}
	for _, source := range sources {
		run, err := s.store.BeginCrawlRun(ctx, source.ID)
		if err != nil {
			return nil, err
		}
		switch source.Type {
		case "github_code_search":
			err = s.discoverFromGitHub(ctx, source, run)
		case "clawhub":
			err = s.discoverFromClawHub(ctx, source, run)
		case "html_catalog":
			err = s.discoverFromHTMLCatalog(ctx, source, run)
		case "seed_page":
			err = s.discoverFromSeedPage(ctx, source, run)
		default:
			err = fmt.Errorf("unsupported source type: %s", source.Type)
		}
		if err != nil {
			run.Status = "failed"
			run.ErrorText = err.Error()
		} else {
			run.Status = "success"
		}
		if err2 := s.store.CompleteCrawlRun(ctx, run); err2 != nil && s.logger != nil {
			s.logger.Warn("complete crawl run failed", zap.Error(err2))
		}
		result.Discovered += run.Discovered
		result.Updated += run.Updated
		result.Failed += run.Failed
	}
	return result, nil
}

func (s *Service) ensureDefaultSources(ctx context.Context) error {
	defaults := []Source{
		{ID: "github-skill-md", Type: "github_code_search", BaseURL: "filename:SKILL.md", DisplayName: "GitHub SKILL.md", SourceGroup: "github", AuthMode: "optional_token", Enabled: true, RateLimitPerMinute: 30, Priority: 30},
		{ID: "github-claude-md", Type: "github_code_search", BaseURL: "filename:CLAUDE.md", DisplayName: "GitHub CLAUDE.md", SourceGroup: "github", AuthMode: "optional_token", Enabled: true, RateLimitPerMinute: 30, Priority: 31},
		{ID: "github-agent-md", Type: "github_code_search", BaseURL: "filename:AGENT.md", DisplayName: "GitHub AGENT.md", SourceGroup: "github", AuthMode: "optional_token", Enabled: true, RateLimitPerMinute: 30, Priority: 32},
		{ID: "clawhub", Type: "clawhub", BaseURL: strings.TrimRight(s.cfg.ClawHubBaseURL, "/"), DisplayName: "ClawHub", SourceGroup: "clawhub", AuthMode: "none", Enabled: true, RateLimitPerMinute: 60, Priority: 10},
		{ID: "skillhub-club", Type: "html_catalog", BaseURL: strings.TrimRight(s.cfg.SkillHubBaseURL, "/"), DisplayName: "SkillHub Club", SourceGroup: "skillhub", AuthMode: "optional_api_key", Enabled: true, RateLimitPerMinute: 20, Priority: 40},
		{ID: "skillstack", Type: "html_catalog", BaseURL: strings.TrimRight(s.cfg.SkillStackBaseURL, "/"), DisplayName: "SkillStack", SourceGroup: "skillstack", AuthMode: "none", Enabled: true, RateLimitPerMinute: 20, Priority: 41},
		{ID: "skillsmp", Type: "html_catalog", BaseURL: strings.TrimRight(s.cfg.SkillsMPBaseURL, "/"), DisplayName: "SkillsMP", SourceGroup: "skillsmp", AuthMode: "optional_api_key", Enabled: true, RateLimitPerMinute: 20, Priority: 42},
		{ID: "llmskills", Type: "html_catalog", BaseURL: strings.TrimRight(s.cfg.LLMSkillsBaseURL, "/"), DisplayName: "LLMSkills", SourceGroup: "llmskills", AuthMode: "none", Enabled: true, RateLimitPerMinute: 20, Priority: 43},
	}
	if token := strings.TrimSpace(s.cfg.SkillHubAPIKey); token != "" {
		defaults[4].Headers = map[string]string{
			"Authorization": "Bearer " + token,
			"X-API-Key":     token,
		}
	}
	if token := strings.TrimSpace(s.cfg.SkillsMPAPIKey); token != "" {
		defaults[6].Headers = map[string]string{
			"Authorization": "Bearer " + token,
			"X-API-Key":     token,
		}
	}
	for i, mirrorURL := range s.cfg.ClawHubMirrorBaseURLs {
		defaults = append(defaults, Source{
			ID:                 fmt.Sprintf("clawhub-mirror-%d", i+1),
			Type:               "clawhub",
			BaseURL:            strings.TrimRight(mirrorURL, "/"),
			DisplayName:        "ClawHub Mirror",
			SourceGroup:        "clawhub",
			MirrorOf:           "clawhub",
			AuthMode:           "none",
			Enabled:            true,
			RateLimitPerMinute: 60,
			Priority:           11 + i,
		})
	}
	for i, seed := range s.cfg.SeedURLs {
		defaults = append(defaults, Source{
			ID:                 fmt.Sprintf("seed-%d", i+1),
			Type:               "seed_page",
			BaseURL:            seed,
			DisplayName:        fmt.Sprintf("Seed %d", i+1),
			SourceGroup:        "seed",
			AuthMode:           "none",
			Enabled:            true,
			RateLimitPerMinute: 10,
			Priority:           100 + i,
		})
	}
	for _, source := range defaults {
		if err := s.store.UpsertSource(ctx, source); err != nil {
			return err
		}
	}
	return nil
}

type gitHubCodeSearchResponse struct {
	Items []struct {
		Name       string `json:"name"`
		Path       string `json:"path"`
		URL        string `json:"url"`
		HTMLURL    string `json:"html_url"`
		SHA        string `json:"sha"`
		Repository struct {
			FullName        string    `json:"full_name"`
			HTMLURL         string    `json:"html_url"`
			StargazersCount int       `json:"stargazers_count"`
			UpdatedAt       time.Time `json:"updated_at"`
		} `json:"repository"`
	} `json:"items"`
}

type gitHubContentResponse struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
	Path     string `json:"path"`
}

func (s *Service) discoverFromGitHub(ctx context.Context, source Source, run *CrawlRun) error {
	apiURL := strings.TrimRight(s.cfg.GitHubAPIBaseURL, "/") + "/search/code?q=" + urlQueryEscape(source.BaseURL) + "&per_page=25"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if token := strings.TrimSpace(s.cfg.GitHubToken); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("github code search: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var payload gitHubCodeSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return err
	}
	for _, item := range payload.Items {
		raw, err := s.fetchGitHubBlob(ctx, item.URL)
		if err != nil {
			run.Failed++
			continue
		}
		updated, err := s.ingestSkillContent(ctx, ingestRequest{
			SourceID:       source.ID,
			SourceName:     defaultString(source.DisplayName, source.ID),
			SourceGroup:    defaultString(source.SourceGroup, source.ID),
			SourceType:     source.Type,
			RepoURL:        item.Repository.HTMLURL,
			Homepage:       item.Repository.HTMLURL,
			DownloadURL:    item.HTMLURL,
			SourceURL:      item.HTMLURL,
			SkillPath:      item.Path,
			SkillContent:   raw,
			Stars:          item.Repository.StargazersCount,
			LastUpdated:    item.Repository.UpdatedAt,
			CommitHash:     item.SHA,
			Downloads:      0,
			DefaultSkillID: pathSkillID(item.Repository.HTMLURL, item.Path, item.Name),
			Installable:    true,
			InstallType:    InstallTypeGitRepo,
			ArtifactKind:   ArtifactKindOpenSource,
		})
		if err != nil {
			run.Failed++
			continue
		}
		if updated {
			run.Updated++
		} else {
			run.Discovered++
		}
	}
	return nil
}

func (s *Service) fetchGitHubBlob(ctx context.Context, apiURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if token := strings.TrimSpace(s.cfg.GitHubToken); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github blob status: %d", resp.StatusCode)
	}
	var payload gitHubContentResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if strings.EqualFold(payload.Encoding, "base64") {
		data, err := decodeBase64(strings.ReplaceAll(payload.Content, "\n", ""))
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	return payload.Content, nil
}

type clawHubListResponse struct {
	Items []struct {
		Slug        string `json:"slug"`
		DisplayName string `json:"displayName"`
		Summary     string `json:"summary"`
		CreatedAt   int64  `json:"createdAt"`
		UpdatedAt   int64  `json:"updatedAt"`
		Stats       struct {
			Stars     int `json:"stars"`
			Downloads int `json:"downloads"`
		} `json:"stats"`
		LatestVersion struct {
			Version string `json:"version"`
		} `json:"latestVersion"`
	} `json:"items"`
	NextCursor string `json:"nextCursor,omitempty"`
}

type clawHubSkillDetailResponse struct {
	Slug        string   `json:"slug"`
	DisplayName string   `json:"displayName"`
	Summary     string   `json:"summary"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Categories  []string `json:"categories"`
	Author      struct {
		Username string `json:"username"`
		Name     string `json:"name"`
	} `json:"author"`
	Tags  []string `json:"tags"`
	Stats struct {
		Stars     int `json:"stars"`
		Downloads int `json:"downloads"`
		Versions  int `json:"versions"`
	} `json:"stats"`
	LatestVersion struct {
		Version    string   `json:"version"`
		CreatedAt  int64    `json:"createdAt"`
		Changelog  string   `json:"changelog"`
		Category   string   `json:"category"`
		Categories []string `json:"categories"`
	} `json:"latestVersion"`
}

type clawHubSkillEnrichment struct {
	Description     string
	Author          string
	CategoryHint    string
	AdditionalTags  []string
	SecuritySignals *SourceSecuritySignals
}

func (s *Service) discoverFromClawHub(ctx context.Context, source Source, run *CrawlRun) error {
	const maxPages = 100
	seenSlugs := make(map[string]struct{})
	baseURL := strings.TrimRight(source.BaseURL, "/") + "/api/v1/skills"

	for page := 1; page <= maxPages; page++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s?page=%d", baseURL, page), nil)
		if err != nil {
			return err
		}
		resp, err := s.httpClient.Do(req)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return fmt.Errorf("clawhub list status: %d", resp.StatusCode)
		}
		var payload clawHubListResponse
		err = json.NewDecoder(resp.Body).Decode(&payload)
		resp.Body.Close()
		if err != nil {
			return err
		}
		if len(payload.Items) == 0 {
			break
		}

		newItems := 0
		for _, item := range payload.Items {
			if _, ok := seenSlugs[item.Slug]; ok {
				continue
			}
			seenSlugs[item.Slug] = struct{}{}
			newItems++

			enrichment, err := s.fetchClawHubSkillEnrichment(ctx, source, item.Slug)
			if err != nil && s.logger != nil {
				s.logger.Debug("skillmarket clawhub detail fetch failed", zap.String("skill", item.Slug), zap.String("source", source.ID), zap.Error(err))
			}
			explicitAuthor := ""
			descriptionHint := ""
			categoryHint := ""
			var additionalTags []string
			var securitySignals *SourceSecuritySignals
			if enrichment != nil {
				explicitAuthor = enrichment.Author
				descriptionHint = enrichment.Description
				categoryHint = enrichment.CategoryHint
				additionalTags = enrichment.AdditionalTags
				securitySignals = enrichment.SecuritySignals
			}
			skillURL := fmt.Sprintf("%s/api/v1/skills/%s/skill-md", strings.TrimRight(source.BaseURL, "/"), item.Slug)
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, skillURL, nil)
			if err != nil {
				run.Failed++
				continue
			}
			resp, err := s.httpClient.Do(req)
			if err != nil {
				run.Failed++
				continue
			}
			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				run.Failed++
				continue
			}
			raw, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				run.Failed++
				continue
			}
			updated, err := s.ingestSkillContent(ctx, ingestRequest{
				SourceID:        source.ID,
				SourceName:      defaultString(source.DisplayName, source.ID),
				SourceGroup:     defaultString(source.SourceGroup, source.ID),
				SourceType:      source.Type,
				RepoURL:         strings.TrimRight(source.BaseURL, "/") + "/skills/" + item.Slug,
				Homepage:        strings.TrimRight(source.BaseURL, "/") + "/skills/" + item.Slug,
				DownloadURL:     skillURL,
				SourceURL:       skillURL,
				SkillPath:       "SKILL.md",
				SkillContent:    string(raw),
				Stars:           item.Stats.Stars,
				Downloads:       item.Stats.Downloads,
				LastUpdated:     time.UnixMilli(item.UpdatedAt),
				ExplicitName:    item.DisplayName,
				ExplicitID:      normalizeSkillID(item.Slug),
				ExplicitVersion: defaultString(item.LatestVersion.Version, "0.1.0"),
				ExplicitAuthor:  explicitAuthor,
				DescriptionHint: descriptionHint,
				CategoryHint:    categoryHint,
				AdditionalTags:  additionalTags,
				SecuritySignals: securitySignals,
				Installable:     true,
				InstallType:     InstallTypeRawSkill,
				ArtifactKind:    ArtifactKindOpenSource,
			})
			if err != nil {
				run.Failed++
				continue
			}
			if updated {
				run.Updated++
			} else {
				run.Discovered++
			}
		}

		if newItems == 0 {
			break
		}
		if payload.NextCursor == "" && len(payload.Items) < 100 {
			break
		}
	}
	return nil
}

func (s *Service) fetchClawHubSkillEnrichment(ctx context.Context, source Source, skillID string) (*clawHubSkillEnrichment, error) {
	apiURL := fmt.Sprintf("%s/api/v1/skills/%s", strings.TrimRight(source.BaseURL, "/"), skillID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("clawhub detail status: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var detail clawHubSkillDetailResponse
	if err := json.Unmarshal(body, &detail); err != nil {
		return nil, err
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	tags := dedupeStrings(append(detail.Tags, collectNamedStringList(raw, "tags", "labels")...))
	categoryHint := pickClawHubCategory(detail, raw, tags)
	enrichment := &clawHubSkillEnrichment{
		Description:     strings.TrimSpace(defaultString(detail.Description, detail.Summary)),
		Author:          strings.TrimSpace(defaultString(detail.Author.Name, detail.Author.Username)),
		CategoryHint:    categoryHint,
		AdditionalTags:  tags,
		SecuritySignals: extractClawHubSecuritySignals(raw),
	}
	if strings.TrimSpace(enrichment.Description) == "" &&
		strings.TrimSpace(enrichment.Author) == "" &&
		strings.TrimSpace(enrichment.CategoryHint) == "" &&
		len(enrichment.AdditionalTags) == 0 &&
		enrichment.SecuritySignals == nil {
		return nil, nil
	}
	return enrichment, nil
}

func pickClawHubCategory(detail clawHubSkillDetailResponse, raw map[string]interface{}, tags []string) string {
	candidates := make([]string, 0, 8)
	candidates = append(candidates, detail.Category)
	candidates = append(candidates, detail.Categories...)
	candidates = append(candidates, detail.LatestVersion.Category)
	candidates = append(candidates, detail.LatestVersion.Categories...)
	candidates = append(candidates, collectNamedStringList(raw, "category", "categories")...)
	text := strings.TrimSpace(strings.Join([]string{detail.DisplayName, detail.Summary, detail.Description}, "\n"))
	hasExplicitCategory := false
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		hasExplicitCategory = true
		normalized := normalizeCategory(candidate, text, tags)
		if normalized != "" && normalized != "other" {
			return normalized
		}
	}
	if !hasExplicitCategory && len(tags) == 0 && strings.TrimSpace(text) == "" {
		return ""
	}
	inferred := normalizeCategory("", text, tags)
	if !hasExplicitCategory && inferred == "productivity" {
		return ""
	}
	return inferred
}

func extractClawHubSecuritySignals(raw map[string]interface{}) *SourceSecuritySignals {
	candidates := []map[string]interface{}{
		namedMap(raw, "securityScan", "security_scan"),
		namedMap(raw, "security", "scan", "securityReport", "security_report", "securitySummary", "security_summary"),
		namedNestedMap(raw, "latestVersion", "securityScan", "security_scan"),
		namedNestedMap(raw, "latestVersion", "security", "scan", "securityReport", "security_report", "securitySummary", "security_summary"),
	}
	signals := &SourceSecuritySignals{ScannerVersion: "clawhub-security-scan"}
	for _, candidate := range candidates {
		if len(candidate) == 0 {
			continue
		}
		mergeClawHubSecurityCandidate(signals, candidate)
	}
	if !hasSourceSecuritySignals(signals) {
		return nil
	}
	return signals
}

func mergeClawHubSecurityCandidate(signals *SourceSecuritySignals, node map[string]interface{}) {
	if signals == nil || len(node) == 0 {
		return
	}
	if score, ok := namedInt(node, "score", "securityScore", "security_score"); ok {
		if signals.Score == nil || score < *signals.Score {
			signals.Score = &score
		}
	}
	if risk := normalizeExternalRiskLevel(namedString(node, "riskLevel", "risk_level", "severity", "level")); risk != "" {
		signals.RiskLevel = moreSevereRiskLevel(signals.RiskLevel, risk)
	}
	if badge := normalizeExternalSecurityBadge(namedString(node, "securityBadge", "security_badge", "badge", "status")); badge != "" {
		signals.SecurityBadge = moreSevereBadge(signals.SecurityBadge, badge)
	}
	if vulnStatus := normalizeVulnerabilityStatus(namedString(node, "vulnerabilityStatus", "vulnerability_status")); vulnStatus != "" {
		signals.VulnerabilityStatus = mergeVulnerabilityStatus(signals.VulnerabilityStatus, vulnStatus)
	}
	signals.Permissions = append(signals.Permissions, collectNamedStringList(node, "permissions", "requiredPermissions", "capabilities", "scopes")...)
	signals.Vulnerabilities = append(signals.Vulnerabilities, collectNamedStringList(node, "vulnerabilities", "cves")...)
	signals.HasVulnerabilities = signals.HasVulnerabilities || len(signals.Vulnerabilities) > 0 || namedBoolSignal(node, "hasVulnerabilities", "has_vulnerabilities", "vulnerabilitiesDetected", "vulnerabilities_found")
	signals.HasPromptInjection = signals.HasPromptInjection || namedBoolSignal(node, "hasPromptInjection", "has_prompt_injection", "promptInjection", "prompt_injection")
	signals.HasShellInjection = signals.HasShellInjection || namedBoolSignal(node, "hasShellInjection", "has_shell_injection", "shellInjection", "shell_injection", "commandInjection", "command_injection")
	signals.HasDataExfiltration = signals.HasDataExfiltration || namedBoolSignal(node, "hasDataExfiltration", "has_data_exfiltration", "dataExfiltration", "data_exfiltration", "exfiltration", "exfiltrate")
	signals.HasBinary = signals.HasBinary || namedBoolSignal(node, "hasBinary", "has_binary", "binary", "containsBinary", "contains_binary")
	signals.HasScripts = signals.HasScripts || namedBoolSignal(node, "hasScripts", "has_scripts", "scripts", "containsScripts", "contains_scripts")

	if surface := namedMap(node, "installSurface", "install_surface", "surface"); len(surface) > 0 {
		if signals.InstallType == "" {
			signals.InstallType = normalizeInstallType(namedString(surface, "installType", "install_type", "type"))
		}
		if signals.ArtifactKind == "" {
			signals.ArtifactKind = normalizeArtifactKind(namedString(surface, "artifactKind", "artifact_kind", "artifact"))
		}
		if installable, ok := namedBool(surface, "installable"); ok && signals.Installable == nil {
			signals.Installable = &installable
		}
		signals.HasBinary = signals.HasBinary || namedBoolSignal(surface, "hasBinary", "has_binary", "binary")
		signals.HasScripts = signals.HasScripts || namedBoolSignal(surface, "hasScripts", "has_scripts", "scripts")
	}
	if signals.InstallType == "" {
		signals.InstallType = normalizeInstallType(namedString(node, "installType", "install_type"))
	}
	if signals.ArtifactKind == "" {
		signals.ArtifactKind = normalizeArtifactKind(namedString(node, "artifactKind", "artifact_kind"))
	}
	if installable, ok := namedBool(node, "installable"); ok && signals.Installable == nil {
		signals.Installable = &installable
	}

	signals.Evidence = append(signals.Evidence, parseClawHubEvidence(node, "evidence")...)
	signals.Evidence = append(signals.Evidence, parseClawHubEvidence(node, "findings")...)
	signals.Evidence = append(signals.Evidence, parseClawHubEvidence(node, "issues")...)
	signals.Evidence = append(signals.Evidence, parseClawHubEvidence(node, "risks")...)
	signals.Evidence = append(signals.Evidence, parseClawHubEvidence(node, "warnings")...)
	if len(signals.Vulnerabilities) > 0 && signals.VulnerabilityStatus == "" {
		signals.VulnerabilityStatus = VulnerabilityStatusDetected
	}
	if signals.HasVulnerabilities && signals.VulnerabilityStatus == "" {
		signals.VulnerabilityStatus = VulnerabilityStatusSuspected
	}

	if signals.HasPromptInjection {
		signals.Findings = append(signals.Findings, SecurityFinding{Type: "prompt_injection", Severity: "high", Message: "Source scan flagged prompt injection risk"})
	}
	if signals.HasShellInjection {
		signals.Findings = append(signals.Findings, SecurityFinding{Type: "shell_injection", Severity: "high", Message: "Source scan flagged shell or command injection risk"})
	}
	if signals.HasDataExfiltration {
		signals.Findings = append(signals.Findings, SecurityFinding{Type: "data_exfiltration", Severity: "high", Message: "Source scan flagged data exfiltration risk"})
	}
	if signals.HasVulnerabilities {
		signals.Findings = append(signals.Findings, SecurityFinding{Type: "vulnerability", Severity: "high", Message: "Source scan flagged dependency vulnerability risk"})
	}
	if signals.HasBinary {
		signals.Evidence = append(signals.Evidence, SecurityEvidence{
			Type:        "binary_artifact",
			Severity:    "medium",
			Title:       "Source scan found binary artifact",
			Description: "ClawHub security scan reported binary or opaque install payloads",
		})
	}
	signals.Permissions = dedupeStrings(signals.Permissions)
	signals.Vulnerabilities = dedupeStrings(signals.Vulnerabilities)
}

func hasSourceSecuritySignals(signals *SourceSecuritySignals) bool {
	if signals == nil {
		return false
	}
	return signals.Score != nil ||
		signals.RiskLevel != "" ||
		signals.SecurityBadge != "" ||
		signals.VulnerabilityStatus != "" ||
		len(signals.Permissions) > 0 ||
		len(signals.Vulnerabilities) > 0 ||
		signals.HasVulnerabilities ||
		signals.HasPromptInjection ||
		signals.HasShellInjection ||
		signals.HasDataExfiltration ||
		signals.HasBinary ||
		signals.HasScripts ||
		signals.InstallType != "" ||
		signals.ArtifactKind != "" ||
		signals.Installable != nil ||
		len(signals.Evidence) > 0
}

func normalizeExternalRiskLevel(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "critical", "blocked":
		return RiskCritical
	case "high", "danger":
		return RiskHigh
	case "medium", "warn", "warning", "yellow":
		return RiskMedium
	case "low", "safe", "clean", "green", "pass", "passed":
		return RiskLow
	default:
		return ""
	}
}

func normalizeExternalSecurityBadge(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "critical", "high", "red", "blocked":
		return BadgeRed
	case "medium", "warn", "warning", "yellow":
		return BadgeYellow
	case "low", "safe", "clean", "green", "pass", "passed":
		return BadgeGreen
	default:
		return ""
	}
}

func namedString(node map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := node[key]; ok {
			if text := stringFromValue(value); text != "" {
				return text
			}
		}
	}
	return ""
}

func namedInt(node map[string]interface{}, keys ...string) (int, bool) {
	for _, key := range keys {
		if value, ok := node[key]; ok {
			switch v := value.(type) {
			case int:
				return v, true
			case int32:
				return int(v), true
			case int64:
				return int(v), true
			case float64:
				return int(v), true
			}
		}
	}
	return 0, false
}

func namedBool(node map[string]interface{}, keys ...string) (bool, bool) {
	for _, key := range keys {
		if value, ok := node[key]; ok {
			if parsed, ok := parseSecurityBool(value); ok {
				return parsed, true
			}
		}
	}
	return false, false
}

func namedBoolSignal(node map[string]interface{}, keys ...string) bool {
	if value, ok := namedBool(node, keys...); ok && value {
		return true
	}
	for _, container := range []string{"signals", "checks", "summary", "results"} {
		if nested := namedMap(node, container); len(nested) > 0 {
			if value, ok := namedBool(nested, keys...); ok && value {
				return true
			}
		}
	}
	return false
}

func namedMap(node map[string]interface{}, keys ...string) map[string]interface{} {
	for _, key := range keys {
		if value, ok := node[key]; ok {
			if mapped, ok := value.(map[string]interface{}); ok {
				return mapped
			}
		}
	}
	return nil
}

func namedNestedMap(node map[string]interface{}, parent string, keys ...string) map[string]interface{} {
	if nested := namedMap(node, parent); len(nested) > 0 {
		return namedMap(nested, keys...)
	}
	return nil
}

func collectNamedStringList(node map[string]interface{}, keys ...string) []string {
	out := make([]string, 0)
	for _, key := range keys {
		if value, ok := node[key]; ok {
			out = append(out, stringListFromValue(value)...)
		}
	}
	return dedupeStrings(out)
}

func stringListFromValue(value interface{}) []string {
	switch v := value.(type) {
	case string:
		parts := strings.FieldsFunc(v, func(r rune) bool {
			return r == ',' || r == '\n' || r == ';'
		})
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				out = append(out, part)
			}
		}
		return out
	case []string:
		return dedupeStrings(v)
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if text := stringFromValue(item); text != "" {
				out = append(out, text)
			}
		}
		return dedupeStrings(out)
	case map[string]interface{}:
		if text := stringFromValue(v["value"]); text != "" {
			return []string{text}
		}
		if text := stringFromValue(v["name"]); text != "" {
			return []string{text}
		}
		if text := stringFromValue(v["label"]); text != "" {
			return []string{text}
		}
	}
	return nil
}

func stringFromValue(value interface{}) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	case map[string]interface{}:
		for _, key := range []string{"name", "label", "title", "value", "slug", "id"} {
			if text := stringFromValue(v[key]); text != "" {
				return text
			}
		}
	case []interface{}:
		for _, item := range v {
			if text := stringFromValue(item); text != "" {
				return text
			}
		}
	default:
		if text := strings.TrimSpace(fmt.Sprint(value)); text != "" && text != "<nil>" {
			return text
		}
	}
	return ""
}

func parseSecurityBool(value interface{}) (bool, bool) {
	switch v := value.(type) {
	case bool:
		return v, true
	case float64:
		return v > 0, true
	case int:
		return v > 0, true
	case string:
		switch strings.TrimSpace(strings.ToLower(v)) {
		case "true", "yes", "1", "detected", "found", "warn", "warning", "medium", "high", "critical", "blocked", "unsafe", "present", "suspected", "yellow", "red":
			return true, true
		case "false", "no", "0", "none", "safe", "clean", "pass", "passed", "green", "not_applicable", "not-applicable", "n/a", "low":
			return false, true
		}
	case map[string]interface{}:
		for _, key := range []string{"status", "result", "severity", "level", "risk", "flagged", "detected", "present", "value"} {
			if nested, ok := v[key]; ok {
				if parsed, ok := parseSecurityBool(nested); ok {
					return parsed, true
				}
			}
		}
	}
	return false, false
}

func parseClawHubEvidence(node map[string]interface{}, key string) []SecurityEvidence {
	value, ok := node[key]
	if !ok {
		return nil
	}
	switch v := value.(type) {
	case []interface{}:
		out := make([]SecurityEvidence, 0, len(v))
		for _, item := range v {
			switch evidence := item.(type) {
			case string:
				if strings.TrimSpace(evidence) == "" {
					continue
				}
				out = append(out, SecurityEvidence{
					Type:        key,
					Severity:    "medium",
					Title:       "Source scan note",
					Description: strings.TrimSpace(evidence),
					Value:       strings.TrimSpace(evidence),
				})
			case map[string]interface{}:
				title := firstNonBlank(stringFromValue(evidence["title"]), stringFromValue(evidence["name"]), stringFromValue(evidence["message"]), "Source scan note")
				description := firstNonBlank(stringFromValue(evidence["description"]), stringFromValue(evidence["detail"]))
				severity := firstNonBlank(strings.TrimSpace(strings.ToLower(stringFromValue(evidence["severity"]))), "medium")
				value := firstNonBlank(stringFromValue(evidence["value"]), stringFromValue(evidence["evidence"]), stringFromValue(evidence["code"]))
				out = append(out, SecurityEvidence{
					Type:        key,
					Severity:    severity,
					Title:       title,
					Description: description,
					Value:       value,
				})
			}
		}
		return out
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		return []SecurityEvidence{{
			Type:        key,
			Severity:    "medium",
			Title:       "Source scan note",
			Description: strings.TrimSpace(v),
			Value:       strings.TrimSpace(v),
		}}
	default:
		return nil
	}
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (s *Service) discoverFromSeedPage(ctx context.Context, source Source, run *CrawlRun) error {
	pageCrawler := crawler.New(crawler.Config{
		MaxDepth:       0,
		MaxConcurrency: 2,
		RequestTimeout: 20 * time.Second,
		UserAgent:      "ZimaOS-SkillMarket/1.0",
	})
	results := pageCrawler.CrawlSync(ctx, []string{source.BaseURL})
	for _, page := range results {
		for _, link := range page.Links {
			if !looksLikeSkillURL(link) {
				continue
			}
			raw, skillPath, repoURL, err := s.fetchGenericSkillURL(ctx, link)
			if err != nil {
				run.Failed++
				continue
			}
			updated, err := s.ingestSkillContent(ctx, ingestRequest{
				SourceID:       source.ID,
				SourceName:     defaultString(source.DisplayName, source.ID),
				SourceGroup:    defaultString(source.SourceGroup, source.ID),
				SourceType:     source.Type,
				RepoURL:        repoURL,
				Homepage:       link,
				DownloadURL:    link,
				SourceURL:      link,
				SkillPath:      skillPath,
				SkillContent:   raw,
				DefaultSkillID: pathSkillID(repoURL, skillPath, filepath.Base(skillPath)),
				LastUpdated:    timeutil.NowTime(),
				Installable:    true,
				InstallType:    InstallTypeRawSkill,
				ArtifactKind:   ArtifactKindOpenSource,
			})
			if err != nil {
				run.Failed++
				continue
			}
			if updated {
				run.Updated++
			} else {
				run.Discovered++
			}
		}
	}
	return nil
}

type ingestRequest struct {
	SourceID        string
	SourceName      string
	SourceGroup     string
	SourceType      string
	RepoURL         string
	Homepage        string
	DownloadURL     string
	SourceURL       string
	SkillPath       string
	SkillContent    string
	CommitHash      string
	Stars           int
	Downloads       int
	LastUpdated     time.Time
	DefaultSkillID  string
	ExplicitID      string
	ExplicitName    string
	ExplicitVersion string
	ExplicitAuthor  string
	DescriptionHint string
	CategoryHint    string
	AdditionalTags  []string
	SecuritySignals *SourceSecuritySignals
	Installable     bool
	InstallType     string
	ArtifactKind    string
	HasBinary       bool
	HasScripts      bool
}

func (s *Service) ingestSkillContent(ctx context.Context, req ingestRequest) (bool, error) {
	fallbackID := req.ExplicitID
	if fallbackID == "" {
		fallbackID = req.DefaultSkillID
	}
	parsed, err := parseSkillMarkdown(req.SkillContent, fallbackID)
	if err != nil {
		return false, err
	}
	if req.ExplicitName != "" {
		parsed.Manifest.Name = req.ExplicitName
	}
	if req.ExplicitVersion != "" {
		parsed.Manifest.Version = req.ExplicitVersion
	}
	if strings.TrimSpace(parsed.Manifest.Author) == "" && strings.TrimSpace(req.ExplicitAuthor) != "" {
		parsed.Manifest.Author = strings.TrimSpace(req.ExplicitAuthor)
	}
	if strings.TrimSpace(req.DescriptionHint) != "" {
		inferred := inferDescription(parsed.Content)
		if parsed.Manifest.Description == "" || parsed.Manifest.Description == inferred || strings.EqualFold(parsed.Manifest.Description, parsed.Manifest.Name) {
			parsed.Manifest.Description = strings.TrimSpace(req.DescriptionHint)
		}
	}
	if parsed.Manifest.ID == "" {
		parsed.Manifest.ID = fallbackID
	}
	if parsed.Manifest.ID == "" {
		parsed.Manifest.ID = normalizeSkillID(uuid.NewString())
	}
	categorySeed := parsed.Manifest.Category
	if strings.TrimSpace(req.CategoryHint) != "" {
		categorySeed = req.CategoryHint
	}
	parsed.Manifest.Category = normalizeCategory(categorySeed, req.SkillContent, append(parsed.Manifest.Tags, req.AdditionalTags...))
	parsed.Manifest.Tags = normalizeTags(parsed.Manifest.Category, append(parsed.Manifest.Tags, req.AdditionalTags...))

	report := s.scanner.ScanWithSurface(ctx, parsed.Manifest.ID, parsed.Version, req.SkillContent, parsed.Manifest.Permissions, InstallSurface{
		InstallType:  defaultString(req.InstallType, InstallTypeRawSkill),
		ArtifactKind: defaultString(req.ArtifactKind, ArtifactKindOpenSource),
		Installable:  req.Installable || req.InstallType == InstallTypeRawSkill || req.InstallType == InstallTypeGitRepo,
		HasBinary:    req.HasBinary,
		HasScripts:   req.HasScripts,
	})
	report = mergeExternalSecuritySignals(report, req.SecuritySignals)
	doc := &SkillDocument{
		ID:                  parsed.Manifest.ID,
		Slug:                normalizeSkillID(parsed.Manifest.ID),
		Name:                parsed.Manifest.Name,
		Description:         parsed.Manifest.Description,
		Author:              parsed.Manifest.Author,
		RepoURL:             req.RepoURL,
		Homepage:            defaultString(req.Homepage, req.SourceURL),
		DownloadURL:         req.DownloadURL,
		Stars:               req.Stars,
		Downloads:           req.Downloads,
		Tags:                parsed.Manifest.Tags,
		Category:            parsed.Manifest.Category,
		SecurityScore:       report.Score,
		Permissions:         report.Permissions,
		LatestVersion:       parsed.Manifest.Version,
		RiskLevel:           report.RiskLevel,
		SecurityBadge:       report.SecurityBadge,
		Installable:         report.InstallSurface.Installable,
		InstallType:         report.InstallSurface.InstallType,
		ArtifactKind:        report.InstallSurface.ArtifactKind,
		VulnerabilityStatus: report.VulnerabilityStatus,
		HasVulnerabilities:  report.HasVulnerabilities,
		HasPromptInjection:  report.HasPromptInjection,
		HasShellInjection:   report.HasShellInjection,
		HasDataExfiltration: report.HasDataExfiltration,
		HasBinary:           report.InstallSurface.HasBinary,
		HasScripts:          report.InstallSurface.HasScripts,
		ScanStatus:          report.LLMStatus,
		ContentSHA256:       parsed.Checksum,
		Published:           true,
		SourceID:            req.SourceID,
		SourceName:          req.SourceName,
		SourceGroup:         req.SourceGroup,
		SourceType:          req.SourceType,
		SkillPath:           req.SkillPath,
		SkillContent:        req.SkillContent,
		LastUpdated:         req.LastUpdated,
		LastCrawledAt:       timeutil.NowTime(),
	}
	if s.embeddingProvider != nil {
		vec, err := s.embeddingProvider.Embed(ctx, parsed.SearchDoc)
		if err == nil && len(vec) > 0 {
			data, _ := json.Marshal(vec)
			doc.EmbeddingJSON = string(data)
			doc.EmbeddingModel = s.embeddingProvider.Model()
		}
	}
	version := &SkillVersion{
		ID:           uuid.NewString(),
		SkillID:      doc.ID,
		Version:      parsed.Manifest.Version,
		CommitHash:   req.CommitHash,
		SourceURL:    req.SourceURL,
		Checksum:     parsed.Checksum,
		SkillPath:    req.SkillPath,
		RawSkillMD:   req.SkillContent,
		ManifestJSON: manifestJSON(parsed.Manifest),
		ReleasedAt:   req.LastUpdated,
		ScannedAt:    timeutil.NowTime(),
	}
	existing, _ := s.store.GetSkill(ctx, doc.ID)
	updated := existing != nil
	return updated, s.store.UpsertSkill(ctx, doc, version, report)
}

func (s *Service) materializeVersion(tempDir string, doc SkillDocument, version *SkillVersion) error {
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return err
	}
	if version.RawSkillMD != "" {
		if err := os.WriteFile(filepath.Join(tempDir, "SKILL.md"), []byte(version.RawSkillMD), 0o644); err != nil {
			return err
		}
	}
	var manifest skill.Manifest
	if err := json.Unmarshal([]byte(version.ManifestJSON), &manifest); err == nil && manifest.ID != "" {
		data, _ := json.MarshalIndent(manifest, "", "  ")
		if err := os.WriteFile(filepath.Join(tempDir, "manifest.json"), data, 0o644); err != nil {
			return err
		}
	} else {
		parsed, err := parseSkillMarkdown(version.RawSkillMD, doc.ID)
		if err == nil {
			data, _ := json.MarshalIndent(parsed.Manifest, "", "  ")
			if err := os.WriteFile(filepath.Join(tempDir, "manifest.json"), data, 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) verifyInstallPayload(expectedChecksum, skillFile string) error {
	if strings.TrimSpace(expectedChecksum) == "" {
		return nil
	}
	data, err := os.ReadFile(skillFile)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != expectedChecksum {
		return fmt.Errorf("checksum mismatch for %s", skillFile)
	}
	return nil
}

func (s *Service) scanInstalledPayload(ctx context.Context, skillID, version, dir string, declaredPermissions []string) (*SecurityReport, error) {
	parts := []string{}
	for _, fileName := range []string{"SKILL.md", "manifest.json"} {
		path := filepath.Join(dir, fileName)
		data, err := os.ReadFile(path)
		if err == nil {
			parts = append(parts, string(data))
		}
	}
	scriptsDir := filepath.Join(dir, "scripts")
	if entries, err := os.ReadDir(scriptsDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			data, err := os.ReadFile(filepath.Join(scriptsDir, entry.Name()))
			if err == nil {
				parts = append(parts, string(data))
			}
		}
	}
	surface := detectInstallSurface(dir)
	report := s.scanner.ScanWithSurface(ctx, skillID, version, strings.Join(parts, "\n\n"), declaredPermissions, surface)
	return report, nil
}

func (s *Service) promoteInstall(tempDir, cacheDir, activeDir string) error {
	if err := os.MkdirAll(filepath.Dir(cacheDir), 0o755); err != nil {
		return err
	}
	if err := os.RemoveAll(cacheDir); err != nil {
		return err
	}
	if err := copyDir(tempDir, cacheDir); err != nil {
		return err
	}
	activeTmp := activeDir + ".tmp-" + uuid.NewString()
	if err := os.RemoveAll(activeTmp); err != nil {
		return err
	}
	if err := copyDir(cacheDir, activeTmp); err != nil {
		return err
	}
	if err := os.RemoveAll(activeDir); err != nil {
		return err
	}
	return os.Rename(activeTmp, activeDir)
}

func (s *Service) registerInstalledSkill(ctx context.Context, doc SkillDocument, version *SkillVersion, report *SecurityReport) error {
	parsed, err := parseSkillMarkdown(version.RawSkillMD, doc.ID)
	if err != nil {
		return err
	}
	if s.registry != nil {
		_ = s.registry.Unregister(doc.ID)
		_ = s.registry.Register(skill.NewManifestSkill(parsed.Manifest), false)
	}
	if s.localScanner != nil {
		_ = s.localScanner.Scan()
	}
	return s.store.SetInstalledSkill(ctx, InstalledSkill{
		SkillID:           doc.ID,
		Name:              doc.Name,
		InstalledVersion:  version.Version,
		Checksum:          version.Checksum,
		SourceURL:         version.SourceURL,
		Enabled:           true,
		AutoUpdate:        false,
		InstalledAt:       timeutil.NowTime(),
		UpdatedAt:         timeutil.NowTime(),
		LastSecurityScore: report.Score,
	})
}

func (s *Service) installFromGitHub(ctx context.Context, repo string, ackRisk bool) (*InstallResult, error) {
	repo = strings.TrimPrefix(repo, "github:")
	repo = strings.TrimSpace(repo)
	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("github repo must be owner/repo")
	}

	metaURL := fmt.Sprintf("%s/repos/%s/%s", strings.TrimRight(s.cfg.GitHubAPIBaseURL, "/"), parts[0], parts[1])
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metaURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if token := strings.TrimSpace(s.cfg.GitHubToken); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github repo status: %d", resp.StatusCode)
	}
	var repoMeta struct {
		HTMLURL       string    `json:"html_url"`
		DefaultBranch string    `json:"default_branch"`
		UpdatedAt     time.Time `json:"updated_at"`
		Stargazers    int       `json:"stargazers_count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&repoMeta); err != nil {
		return nil, err
	}

	contents, err := s.fetchGitHubRepoSkill(ctx, parts[0], parts[1], repoMeta.DefaultBranch)
	if err != nil {
		return nil, err
	}
	updated, err := s.ingestSkillContent(ctx, ingestRequest{
		SourceID:       "direct-github",
		SourceName:     "GitHub",
		SourceGroup:    "github",
		SourceType:     "github_repo",
		RepoURL:        repoMeta.HTMLURL,
		Homepage:       repoMeta.HTMLURL,
		DownloadURL:    rawGitHubBlobURL(parts[0], parts[1], repoMeta.DefaultBranch, contents.Path),
		SourceURL:      repoMeta.HTMLURL,
		SkillPath:      contents.Path,
		SkillContent:   contents.Raw,
		CommitHash:     contents.Commit,
		Stars:          repoMeta.Stargazers,
		LastUpdated:    repoMeta.UpdatedAt,
		DefaultSkillID: pathSkillID(repoMeta.HTMLURL, contents.Path, parts[1]),
		Installable:    true,
		InstallType:    InstallTypeGitRepo,
		ArtifactKind:   ArtifactKindOpenSource,
	})
	if err != nil {
		return nil, err
	}
	_ = updated
	detail, err := s.store.GetSkill(ctx, pathSkillID(repoMeta.HTMLURL, contents.Path, parts[1]))
	if err != nil || detail == nil {
		return nil, fmt.Errorf("github skill import failed")
	}
	return s.Install(ctx, InstallRequest{ID: detail.Skill.ID, AckRisk: ackRisk})
}

type gitHubRepoSkill struct {
	Path   string
	Raw    string
	Commit string
}

func (s *Service) fetchGitHubRepoSkill(ctx context.Context, owner, repo, branch string) (*gitHubRepoSkill, error) {
	type contentItem struct {
		Name string `json:"name"`
		Path string `json:"path"`
		Type string `json:"type"`
		URL  string `json:"url"`
	}

	var walk func(path string) (*gitHubRepoSkill, error)
	walk = func(path string) (*gitHubRepoSkill, error) {
		apiURL := fmt.Sprintf("%s/repos/%s/%s/contents/%s?ref=%s", strings.TrimRight(s.cfg.GitHubAPIBaseURL, "/"), owner, repo, path, branch)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		if token := strings.TrimSpace(s.cfg.GitHubToken); token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := s.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("github contents status: %d", resp.StatusCode)
		}
		var items []contentItem
		if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
			return nil, err
		}
		sort.Slice(items, func(i, j int) bool {
			return items[i].Path < items[j].Path
		})
		for _, item := range items {
			if item.Type == "file" && (item.Name == "SKILL.md" || item.Name == "CLAUDE.md" || item.Name == "AGENT.md") {
				raw, err := s.fetchGitHubBlob(ctx, item.URL)
				if err != nil {
					return nil, err
				}
				return &gitHubRepoSkill{Path: item.Path, Raw: raw, Commit: branch}, nil
			}
		}
		for _, item := range items {
			if item.Type == "dir" {
				found, err := walk(item.Path)
				if err == nil && found != nil {
					return found, nil
				}
			}
		}
		return nil, fmt.Errorf("no skill file found in github repo")
	}
	return walk("")
}

func (s *Service) fetchGenericSkillURL(ctx context.Context, rawURL string) (content, skillPath, repoURL string, err error) {
	body, path, repo, _, _, err := s.fetchSkillReference(ctx, rawURL)
	if err != nil {
		return "", "", "", err
	}
	return body, path, repo, nil
}

func looksLikeSkillURL(link string) bool {
	lower := strings.ToLower(link)
	return strings.Contains(lower, "skill.md") ||
		strings.Contains(lower, "claude.md") ||
		strings.Contains(lower, "agent.md") ||
		strings.Contains(lower, "github.com")
}

func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}
		data, err := os.ReadFile(srcPath)
		if err != nil {
			return err
		}
		if err := os.WriteFile(dstPath, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func decodeBase64(value string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(value)
}

func urlQueryEscape(value string) string {
	return url.QueryEscape(value)
}
