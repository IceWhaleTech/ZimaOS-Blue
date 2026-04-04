package skillmarket

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/embedding"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/network"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillbundle"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmanifest"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

const maxArchiveDownloadBytes = 128 << 20
const maxSemanticCandidates = 200
const vercelSkillsSourceURL = "https://github.com/vercel-labs/skills/tree/main/skills"

var deprecatedDefaultGitHubCodeSearchSources = []Source{
	{ID: "github-claude-md", Type: "github_code_search", BaseURL: "filename:CLAUDE.md"},
	{ID: "github-agent-md", Type: "github_code_search", BaseURL: "filename:AGENT.md"},
}

type Options struct {
	Config                   Config
	Logger                   *zap.Logger
	Registry                 *skill.Registry
	LocalScanner             *skillstore.LocalSkillScanner
	EmbeddingProvider        embedding.Provider
	HTTPClient               *http.Client
	RemoteReader             RemoteReader
	Scanner                  *Scanner
	DiscoverEventBroadcaster func(eventType string, data any)
}

type Service struct {
	store               *Store
	cfg                 Config
	logger              *zap.Logger
	registry            *skill.Registry
	localScanner        *skillstore.LocalSkillScanner
	embeddingProvider   embedding.Provider
	httpClient          *http.Client
	remoteReader        RemoteReader
	scanner             *Scanner
	discoverBroadcaster func(eventType string, data any)
	stopOnce            sync.Once
	stopCh              chan struct{}
	discoverMu          sync.Mutex
	embeddingBackfillMu sync.Mutex
	embeddingStatusMu   sync.Mutex
	discoverStatus      DiscoverStatus
	embeddingStatus     EmbeddingStatus
	ownsDB              bool
}

func NewService(db *sql.DB, opts Options) (*Service, error) {
	return NewServiceWithReadDB(db, db, opts)
}

func NewServiceWithReadDB(writeDB, readDB *sql.DB, opts Options) (*Service, error) {
	return newService(writeDB, readDB, opts, false)
}

func NewServiceWithDBPath(dbPath string, opts Options) (*Service, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open skillmarket db: %w", err)
	}
	// Legacy databases may still contain FTS5 objects from an earlier build.
	// If the current binary lacks FTS5 support, PRAGMA setup can fail before
	// NewStore gets a chance to disable legacy triggers, so retry after init.
	configureErr := configureSkillMarketSQLiteDB(db)
	if configureErr != nil && !isMissingFTS5ModuleError(configureErr) {
		_ = db.Close()
		return nil, fmt.Errorf("configure skillmarket db: %w", configureErr)
	}
	svc, err := newService(db, db, opts, true)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	if configureErr != nil {
		if err := configureSkillMarketSQLiteDB(db); err != nil {
			_ = svc.Close()
			return nil, fmt.Errorf("reconfigure skillmarket db after legacy FTS cleanup: %w", err)
		}
	}
	readDB, readErr := openSkillMarketReaderDB(dbPath)
	if readErr == nil && readDB != nil && svc.store != nil {
		svc.store.readDB = readDB
	}
	return svc, nil
}

func newService(writeDB, readDB *sql.DB, opts Options, ownsDB bool) (*Service, error) {
	store, err := NewStoreWithReadDB(writeDB, readDB)
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
		store:               store,
		cfg:                 cfg,
		logger:              opts.Logger,
		registry:            opts.Registry,
		localScanner:        opts.LocalScanner,
		embeddingProvider:   opts.EmbeddingProvider,
		httpClient:          opts.HTTPClient,
		remoteReader:        opts.RemoteReader,
		scanner:             opts.Scanner,
		discoverBroadcaster: opts.DiscoverEventBroadcaster,
		stopCh:              make(chan struct{}),
		ownsDB:              ownsDB,
	}
	if svc.httpClient == nil {
		svc.httpClient = network.NewPooledHTTPClient(5 * time.Minute)
	}
	if svc.remoteReader == nil {
		svc.remoteReader = newHTTPRemoteReader(svc.httpClient)
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

func configureSkillMarketSQLiteDB(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}

	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(time.Hour)
	db.SetConnMaxIdleTime(30 * time.Minute)

	pragmas := []string{
		"PRAGMA busy_timeout=5000",
		"PRAGMA journal_mode=WAL",
		"PRAGMA cache_size=-2000",
		"PRAGMA synchronous=FULL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA wal_autocheckpoint=1000",
	}
	if runtime.GOOS == "darwin" {
		pragmas = append(pragmas,
			"PRAGMA fullfsync=ON",
			"PRAGMA checkpoint_fullfsync=ON",
		)
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return fmt.Errorf("exec %q: %w", pragma, err)
		}
	}
	return nil
}

func openSkillMarketReaderDB(dbPath string) (*sql.DB, error) {
	dbPath = strings.TrimSpace(dbPath)
	if dbPath == "" || dbPath == ":memory:" {
		return nil, nil
	}

	dsn := fmt.Sprintf("file:%s?mode=ro&_busy_timeout=5000", dbPath)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open skillmarket reader db: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(time.Hour)
	db.SetConnMaxIdleTime(10 * time.Minute)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping skillmarket reader db: %w", err)
	}
	return db, nil
}

func isMissingFTS5ModuleError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such module") && strings.Contains(msg, "fts5")
}

func (s *Service) Store() *Store {
	return s.store
}

func (s *Service) Start(ctx context.Context) {
	if s.embeddingProvider != nil {
		go func() {
			if err := s.backfillSkillEmbeddings(ctx, SearchQuery{}); err != nil && s.logger != nil {
				s.logger.Warn("skillmarket embedding backfill failed", zap.Error(err))
			}
		}()
	}
	go func() {
		if err := s.syncCurations(ctx); err != nil && s.logger != nil {
			s.logger.Warn("skillmarket curation sync failed", zap.Error(err))
		}
	}()
	go func() {
		sources, err := s.store.ListSources(ctx)
		if err != nil {
			if s.logger != nil {
				s.logger.Warn("skillmarket source bootstrap check failed", zap.Error(err))
			}
			return
		}
		needsBootstrap := false
		for _, source := range sources {
			if !source.Enabled || source.Type != "lightmake_api" || !source.LastSuccessAt.IsZero() {
				continue
			}
			needsBootstrap = true
			break
		}
		if !needsBootstrap {
			return
		}
		if _, err := s.Discover(ctx); err != nil && s.logger != nil {
			s.logger.Warn("skillmarket initial bootstrap discover failed", zap.Error(err))
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

func noopCancel() {}

func withOptionalTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return ctx, noopCancel
	}
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 || remaining <= timeout {
			return ctx, noopCancel
		}
	}
	return context.WithTimeout(ctx, timeout)
}

func (s *Service) discoverTimeout() time.Duration {
	if s.cfg.DiscoverTimeout > 0 {
		return s.cfg.DiscoverTimeout
	}
	return DefaultDiscoverTimeout
}

func (s *Service) discoverStepTimeout(source Source) time.Duration {
	if s.cfg.DiscoverStepTimeout > 0 {
		return s.cfg.DiscoverStepTimeout
	}
	switch source.Type {
	case "html_catalog", "seed_page":
		return 45 * time.Second
	default:
		return DefaultDiscoverStepTimeout
	}
}

func (s *Service) newDiscoverContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return withOptionalTimeout(ctx, s.discoverTimeout())
}

func (s *Service) newDiscoverStepContext(ctx context.Context, source Source) (context.Context, context.CancelFunc) {
	return withOptionalTimeout(ctx, s.discoverStepTimeout(source))
}

func (s *Service) Close() error {
	s.Stop()
	if !s.ownsDB || s.store == nil || s.store.db == nil {
		return nil
	}
	return closeSkillMarketDBs(s.store.db, s.store.readDB)
}

func closeSkillMarketDBs(writeDB, readDB *sql.DB) error {
	if readDB != nil && readDB != writeDB {
		_ = readDB.Close()
	}
	if writeDB == nil {
		return nil
	}
	return writeDB.Close()
}

func (s *Service) GetDiscoverStatus() DiscoverStatus {
	s.discoverMu.Lock()
	defer s.discoverMu.Unlock()
	return cloneDiscoverStatus(s.discoverStatus)
}

func (s *Service) GetEmbeddingStatus() EmbeddingStatus {
	s.embeddingStatusMu.Lock()
	defer s.embeddingStatusMu.Unlock()
	return cloneEmbeddingStatus(s.embeddingStatus)
}

func (s *Service) emitDiscoverEvent(phase string, batchInserted, batchUpdated, batchFailed int) {
	if s.discoverBroadcaster == nil {
		return
	}
	s.discoverMu.Lock()
	status := cloneDiscoverStatus(s.discoverStatus)
	s.discoverMu.Unlock()
	s.discoverBroadcaster("skill.market.discover.progress", DiscoverProgressEvent{
		Running:           status.Running,
		StartedAt:         status.StartedAt,
		FinishedAt:        status.FinishedAt,
		LastError:         status.LastError,
		TotalSources:      status.TotalSources,
		ProcessedSources:  status.ProcessedSources,
		CurrentSourceID:   status.CurrentSourceID,
		CurrentSourceName: status.CurrentSourceName,
		SourceResults:     cloneSourceResults(status.SourceResults),
		Result:            status.Result,
		BatchInserted:     batchInserted,
		BatchUpdated:      batchUpdated,
		BatchFailed:       batchFailed,
		Phase:             phase,
	})
}

func (s *Service) emitEmbeddingEvent() {
	if s.discoverBroadcaster == nil {
		return
	}
	s.embeddingStatusMu.Lock()
	status := cloneEmbeddingStatus(s.embeddingStatus)
	s.embeddingStatusMu.Unlock()
	s.discoverBroadcaster("skill.market.embedding.progress", EmbeddingProgressEvent{
		Running:          status.Running,
		StartedAt:        status.StartedAt,
		FinishedAt:       status.FinishedAt,
		LastError:        status.LastError,
		TotalSkills:      status.TotalSkills,
		ProcessedSkills:  status.ProcessedSkills,
		EmbeddedSkills:   status.EmbeddedSkills,
		FailedSkills:     status.FailedSkills,
		CurrentSkillID:   status.CurrentSkillID,
		CurrentSkillName: status.CurrentSkillName,
		Phase:            status.Phase,
	})
}

func (s *Service) StartDiscoverAsync() (DiscoverStatus, bool) {
	s.discoverMu.Lock()
	if s.discoverStatus.Running {
		status := cloneDiscoverStatus(s.discoverStatus)
		s.discoverMu.Unlock()
		return status, false
	}
	s.discoverStatus = DiscoverStatus{
		Running:   true,
		StartedAt: timeutil.NowTime(),
		Result:    &DiscoverResult{},
	}
	status := cloneDiscoverStatus(s.discoverStatus)
	s.discoverMu.Unlock()
	s.emitDiscoverEvent("started", 0, 0, 0)

	go func() {
		ctx, cancel := s.newDiscoverContext(context.Background())
		defer cancel()
		result, err := s.discoverOnce(ctx)
		s.finishDiscover(result, err)
	}()
	return status, true
}

func (s *Service) Search(ctx context.Context, query SearchQuery) (*SearchResponse, error) {
	if !query.Semantic || s.embeddingProvider == nil || strings.TrimSpace(query.Query) == "" {
		return s.store.Search(ctx, query)
	}

	queryVector, err := s.embeddingProvider.Embed(ctx, query.Query)
	if err != nil || len(queryVector) == 0 {
		return s.store.Search(ctx, query)
	}

	candidates, err := s.store.ListFilteredSkills(ctx, query)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return emptySearchResponse(query), nil
	}

	// Limit candidates for semantic search
	if len(candidates) > maxSemanticCandidates {
		candidates = candidates[:maxSemanticCandidates]
	}

	candidates, backfillErr := s.ensureSkillEmbeddings(ctx, candidates)
	if backfillErr != nil && s.logger != nil {
		s.logger.Warn("skillmarket semantic search embedding backfill partially failed", zap.Error(backfillErr))
	}

	keywordResults, err := s.store.SearchKeywordCandidates(ctx, query)
	if err != nil {
		return nil, err
	}
	return s.combineSemanticResults(query, queryVector, candidates, keywordResults), nil
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
		return s.installFromGitHub(ctx, req.GitHub, req.AckRisk, req.ForceInstall)
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
	installFromArchive := shouldInstallFromArchive(detail.Skill, version)
	catalogReport, err := s.store.GetSecurityReport(ctx, req.ID, version.Version)
	if err != nil {
		return nil, err
	}
	if err := enforceInstallSecurityPolicy(catalogReport, req.AckRisk, req.ForceInstall); err != nil {
		return nil, err
	}
	report := catalogReport

	cacheDir := filepath.Join(s.cfg.CacheRoot, req.ID, version.Version)
	activeDir := filepath.Join(s.cfg.ActiveSkillsDir, req.ID)
	tempDir := filepath.Join(os.TempDir(), "skillmarket-"+uuid.NewString())
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)

	installRoot := tempDir
	installVersion := *version
	installDoc := detail.Skill
	archiveWarnings := []string{}
	if installFromArchive {
		installRoot, installVersion, archiveWarnings, err = s.materializeArchiveVersion(ctx, detail.Skill, version, tempDir)
		if err != nil {
			return nil, err
		}
		cacheDir = filepath.Join(s.cfg.CacheRoot, req.ID, installVersion.Version)
		parsed, err := parseSkillMarkdown(installVersion.RawSkillMD, req.ID)
		if err != nil {
			return nil, err
		}
		recomputed, err := s.scanInstalledPayload(ctx, req.ID, installVersion.Version, installRoot, parsed.Manifest.Permissions)
		if err != nil {
			return nil, err
		}
		report = recomputed
	} else {
		if err := s.materializeVersion(tempDir, detail.Skill, version); err != nil {
			return nil, err
		}
		if err := s.verifyInstallPayload(version.Checksum, filepath.Join(tempDir, "SKILL.md")); err != nil {
			return nil, err
		}
		installRoot = tempDir
		if report == nil {
			recomputed, err := s.scanInstalledPayload(ctx, req.ID, version.Version, installRoot, nil)
			if err != nil {
				return nil, err
			}
			report = recomputed
		}
	}
	if report == nil {
		return nil, fmt.Errorf("security report unavailable")
	}
	if installFromArchive {
		installDoc, err = buildInstalledDocument(detail.Skill, &installVersion, report)
		if err != nil {
			return nil, err
		}
		if err := s.store.UpsertSkill(ctx, &installDoc, &installVersion, report); err != nil {
			return nil, err
		}
	}
	warnings := installWarningsForReport(catalogReport, report, req.ForceInstall)
	if len(archiveWarnings) > 0 {
		warnings = append(warnings, archiveWarnings...)
	}
	warnings = dedupeStrings(warnings)

	if err := s.promoteInstall(installRoot, cacheDir, activeDir); err != nil {
		return nil, err
	}
	if err := s.registerInstalledSkill(ctx, installDoc, &installVersion, report); err != nil {
		return nil, err
	}
	_ = s.store.RecordTelemetry(ctx, req.ID, "download")
	_ = s.store.RecordTelemetry(ctx, req.ID, "install")

	return &InstallResult{
		SkillID:     req.ID,
		Version:     installVersion.Version,
		Path:        activeDir,
		CachePath:   cacheDir,
		Warnings:    warnings,
		Security:    report,
		InstalledAt: timeutil.NowTime(),
	}, nil
}

func enforceInstallSecurityPolicy(report *SecurityReport, ackRisk bool, forceInstall bool) error {
	if report == nil {
		return nil
	}
	if isInstallBlockedBySecurityPolicy(report) && !forceInstall {
		return fmt.Errorf("installation blocked by security policy: %s risk", report.RiskLevel)
	}
	return nil
}

func isInstallBlockedBySecurityPolicy(report *SecurityReport) bool {
	return report != nil && (report.RiskLevel == RiskCritical || report.RiskLevel == RiskHigh || report.SecurityBadge == BadgeRed)
}

func requiresInstallRiskAcknowledgement(report *SecurityReport) bool {
	return report != nil && report.SecurityBadge == BadgeYellow
}

func installWarningsForReport(catalogReport, installedReport *SecurityReport, forceInstall bool) []string {
	if installedReport == nil {
		return nil
	}
	warnings := []string{}
	if forceInstall && isInstallBlockedBySecurityPolicy(installedReport) {
		warnings = append(warnings, "Installed after explicitly overriding a blocked high-risk skill. Review the security report before using this skill.")
	}
	switch {
	case isInstallBlockedBySecurityPolicy(installedReport):
		if installPolicyRank(installedReport) > installPolicyRank(catalogReport) {
			warnings = append(warnings, fmt.Sprintf(
				"Installed payload scan escalated this skill from %s to %s. Review the security report before using this skill.",
				installPolicyLabel(catalogReport),
				installPolicyLabel(installedReport),
			))
			return warnings
		}
		warnings = append(warnings, fmt.Sprintf(
			"Installed payload scan flagged this skill as %s. Review the security report before using this skill.",
			installPolicyLabel(installedReport),
		))
		return warnings
	case requiresInstallRiskAcknowledgement(installedReport):
		if installPolicyRank(installedReport) > installPolicyRank(catalogReport) {
			warnings = append(warnings, fmt.Sprintf(
				"Installed payload scan escalated this skill from %s to %s. Review the security report before enabling auto-update.",
				installPolicyLabel(catalogReport),
				installPolicyLabel(installedReport),
			))
			return warnings
		}
		warnings = append(warnings, "Skill requires medium-risk permissions. Review the security report before enabling auto-update.")
		return warnings
	default:
		return warnings
	}
}

func installPolicyRank(report *SecurityReport) int {
	switch {
	case report == nil:
		return 0
	case isInstallBlockedBySecurityPolicy(report):
		return 3
	case requiresInstallRiskAcknowledgement(report):
		return 2
	default:
		return 1
	}
}

func installPolicyLabel(report *SecurityReport) string {
	if report == nil {
		return "unknown risk"
	}
	switch {
	case report.RiskLevel == RiskCritical:
		return "critical risk"
	case report.RiskLevel == RiskHigh:
		return "high risk"
	case report.SecurityBadge == BadgeRed && report.RiskLevel != "" && report.RiskLevel != RiskHigh && report.RiskLevel != RiskCritical:
		return fmt.Sprintf("%s risk with a red security badge", report.RiskLevel)
	case report.SecurityBadge == BadgeRed:
		return "red security badge"
	case report.RiskLevel == RiskMedium || report.SecurityBadge == BadgeYellow:
		return "medium risk"
	case report.RiskLevel == RiskLow || report.SecurityBadge == BadgeGreen:
		return "low risk"
	default:
		return "elevated risk"
	}
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
	if !s.beginDiscover() {
		status := s.GetDiscoverStatus()
		return status.Result, fmt.Errorf("discover already running")
	}
	s.emitDiscoverEvent("started", 0, 0, 0)
	defer func() {
		if r := recover(); r != nil {
			s.finishDiscover(nil, fmt.Errorf("discover panic: %v", r))
			panic(r)
		}
	}()
	discoverCtx, cancel := s.newDiscoverContext(ctx)
	defer cancel()
	result, err := s.discoverOnce(discoverCtx)
	s.finishDiscover(result, err)
	return result, err
}

func (s *Service) beginDiscover() bool {
	s.discoverMu.Lock()
	defer s.discoverMu.Unlock()
	if s.discoverStatus.Running {
		return false
	}
	s.discoverStatus = DiscoverStatus{
		Running:   true,
		StartedAt: timeutil.NowTime(),
		Result:    &DiscoverResult{},
	}
	return true
}

func (s *Service) setDiscoverTotals(totalSources int) {
	s.discoverMu.Lock()
	if !s.discoverStatus.Running {
		s.discoverMu.Unlock()
		return
	}
	s.discoverStatus.TotalSources = totalSources
	if s.discoverStatus.Result == nil {
		s.discoverStatus.Result = &DiscoverResult{}
	}
	s.discoverStatus.Result.SourcesProcessed = 0
	s.discoverMu.Unlock()
	s.emitDiscoverEvent("started", 0, 0, 0)
}

func (s *Service) setDiscoverProgress(source Source, processedSources int, result *DiscoverResult) {
	s.discoverMu.Lock()
	if !s.discoverStatus.Running {
		s.discoverMu.Unlock()
		return
	}
	s.discoverStatus.ProcessedSources = processedSources
	s.discoverStatus.CurrentSourceID = source.ID
	s.discoverStatus.CurrentSourceName = defaultString(source.DisplayName, source.ID)
	s.discoverStatus.Result = cloneDiscoverResult(result)
	if s.discoverStatus.Result == nil {
		s.discoverStatus.Result = &DiscoverResult{}
	}
	s.discoverStatus.SourceResults = cloneSourceResults(s.discoverStatus.Result.SourceResults)
	s.discoverMu.Unlock()
}

func (s *Service) finishDiscover(result *DiscoverResult, err error) {
	s.discoverMu.Lock()
	s.discoverStatus.Running = false
	s.discoverStatus.FinishedAt = timeutil.NowTime()
	if result != nil && s.discoverStatus.TotalSources == 0 {
		s.discoverStatus.TotalSources = result.SourcesProcessed
	}
	if s.discoverStatus.TotalSources > 0 {
		s.discoverStatus.ProcessedSources = s.discoverStatus.TotalSources
	}
	s.discoverStatus.CurrentSourceID = ""
	s.discoverStatus.CurrentSourceName = ""
	s.discoverStatus.Result = cloneDiscoverResult(result)
	if s.discoverStatus.Result != nil {
		s.discoverStatus.SourceResults = cloneSourceResults(s.discoverStatus.Result.SourceResults)
	} else {
		s.discoverStatus.SourceResults = nil
	}
	if err != nil {
		s.discoverStatus.LastError = err.Error()
	} else {
		s.discoverStatus.LastError = ""
	}
	s.discoverMu.Unlock()
	if err != nil {
		s.emitDiscoverEvent("error", 0, 0, 0)
		return
	}
	s.emitDiscoverEvent("completed", 0, 0, 0)
}

func cloneDiscoverStatus(status DiscoverStatus) DiscoverStatus {
	status.Result = cloneDiscoverResult(status.Result)
	status.SourceResults = cloneSourceResults(status.SourceResults)
	return status
}

func cloneEmbeddingStatus(status EmbeddingStatus) EmbeddingStatus {
	return status
}

func (s *Service) startEmbeddingProgress(total int) {
	s.embeddingStatusMu.Lock()
	s.embeddingStatus = EmbeddingStatus{
		Running:     true,
		StartedAt:   timeutil.NowTime(),
		TotalSkills: total,
		Phase:       "started",
	}
	s.embeddingStatusMu.Unlock()
	s.emitEmbeddingEvent()
}

func (s *Service) advanceEmbeddingProgress(doc SkillDocument, embedded bool, err error) {
	s.embeddingStatusMu.Lock()
	if !s.embeddingStatus.Running {
		s.embeddingStatusMu.Unlock()
		return
	}
	s.embeddingStatus.ProcessedSkills++
	s.embeddingStatus.CurrentSkillID = doc.ID
	s.embeddingStatus.CurrentSkillName = firstNonBlank(doc.Name, doc.ID)
	s.embeddingStatus.Phase = "progress"
	if embedded {
		s.embeddingStatus.EmbeddedSkills++
	}
	if err != nil {
		s.embeddingStatus.FailedSkills++
		s.embeddingStatus.LastError = err.Error()
	}
	s.embeddingStatusMu.Unlock()
	s.emitEmbeddingEvent()
}

func (s *Service) finishEmbeddingProgress(err error) {
	s.embeddingStatusMu.Lock()
	if !s.embeddingStatus.Running {
		s.embeddingStatusMu.Unlock()
		return
	}
	s.embeddingStatus.Running = false
	s.embeddingStatus.FinishedAt = timeutil.NowTime()
	s.embeddingStatus.CurrentSkillID = ""
	s.embeddingStatus.CurrentSkillName = ""
	if err != nil {
		s.embeddingStatus.LastError = err.Error()
		s.embeddingStatus.Phase = "error"
	} else {
		s.embeddingStatus.LastError = ""
		s.embeddingStatus.Phase = "completed"
	}
	s.embeddingStatusMu.Unlock()
	s.emitEmbeddingEvent()
}

func cloneDiscoverResult(result *DiscoverResult) *DiscoverResult {
	if result == nil {
		return nil
	}
	cloned := *result
	cloned.SourceResults = cloneSourceResults(result.SourceResults)
	return &cloned
}

func (s *Service) discoverOnce(ctx context.Context) (*DiscoverResult, error) {
	_ = s.syncCurations(ctx)
	sources, err := s.store.ListSources(ctx)
	if err != nil {
		return nil, err
	}
	s.setDiscoverTotals(len(sources))
	jobs := make([]discoverJob, 0, len(sources))
	for _, source := range sources {
		run, err := s.store.BeginCrawlRun(ctx, source.ID)
		if err != nil {
			return nil, err
		}
		job, err := buildDiscoverJob(s, source, run)
		if err != nil {
			run.Status = "failed"
			run.ErrorText = err.Error()
			if err2 := s.store.CompleteCrawlRun(ctx, run); err2 != nil && s.logger != nil {
				s.logger.Warn("complete crawl run failed", zap.Error(err2))
			}
			return nil, err
		}
		jobs = append(jobs, job)
	}
	if len(jobs) == 0 {
		return &DiscoverResult{}, nil
	}

	completed := 0
	for completed < len(jobs) {
		progressed := false
		for _, job := range jobs {
			if job.Done() {
				continue
			}
			progressed = true
			s.setDiscoverProgress(job.Source(), countCompletedJobs(jobs), aggregateDiscoverResult(jobs))
			stepCtx, cancel := s.newDiscoverStepContext(ctx, job.Source())
			stats, err := job.Step(stepCtx)
			cancel()
			s.setDiscoverProgress(job.Source(), countCompletedJobs(jobs), aggregateDiscoverResult(jobs))
			s.emitDiscoverEvent("batch", stats.Inserted, stats.Updated, stats.Failed)
			if err != nil && s.logger != nil {
				s.logger.Warn(
					"skillmarket discover source step failed",
					zap.String("source_id", job.Source().ID),
					zap.String("source_type", job.Source().Type),
					zap.Error(err),
				)
			}
			if job.Done() {
				s.completeDiscoverJob(ctx, job, err)
				completed++
				s.setDiscoverProgress(job.Source(), countCompletedJobs(jobs), aggregateDiscoverResult(jobs))
				s.emitDiscoverEvent("source_complete", 0, 0, 0)
			}
		}
		if !progressed {
			break
		}
	}
	return aggregateDiscoverResult(jobs), nil
}

func (s *Service) ensureDefaultSources(ctx context.Context) error {
	defaults := []Source{
		{ID: "tencent-skillhub", Type: "lightmake_api", BaseURL: strings.TrimRight(s.cfg.TencentSkillHubAPIBaseURL, "/"), DisplayName: "Tencent SkillHub", SourceGroup: "skillhub", AuthMode: "none", Enabled: true, RateLimitPerMinute: 120, Priority: 5},
		{ID: "vercel", Type: "seed_page", BaseURL: vercelSkillsSourceURL, DisplayName: "Vercel", SourceGroup: "vercel", AuthMode: "none", Enabled: true, RateLimitPerMinute: 10, Priority: 29},
		{ID: "clawhub", Type: "clawhub", BaseURL: strings.TrimRight(s.cfg.ClawHubBaseURL, "/"), DisplayName: "ClawHub", SourceGroup: "clawhub", AuthMode: "none", Enabled: true, RateLimitPerMinute: 60, Priority: 10},
		{ID: "skillhub-club", Type: "html_catalog", BaseURL: strings.TrimRight(s.cfg.SkillHubBaseURL, "/"), DisplayName: "SkillHub Club", SourceGroup: "skillhub", AuthMode: "optional_api_key", Enabled: true, RateLimitPerMinute: 20, Priority: 40},
		{ID: "skillstack", Type: "html_catalog", BaseURL: strings.TrimRight(s.cfg.SkillStackBaseURL, "/"), DisplayName: "SkillStack", SourceGroup: "skillstack", AuthMode: "none", Enabled: true, RateLimitPerMinute: 20, Priority: 41},
		{ID: "llmskills", Type: "html_catalog", BaseURL: strings.TrimRight(s.cfg.LLMSkillsBaseURL, "/"), DisplayName: "LLMSkills", SourceGroup: "llmskills", AuthMode: "none", Enabled: true, RateLimitPerMinute: 20, Priority: 43},
	}
	if token := strings.TrimSpace(s.cfg.SkillHubAPIKey); token != "" {
		for i := range defaults {
			if defaults[i].ID != "skillhub-club" {
				continue
			}
			defaults[i].Headers = map[string]string{
				"Authorization": "Bearer " + token,
				"X-API-Key":     token,
			}
			break
		}
	}
	for i, mirrorURL := range s.cfg.ClawHubMirrorBaseURLs {
		enabled := true
		if isTencentSkillHubMirrorURL(mirrorURL) {
			enabled = false
		}
		defaults = append(defaults, Source{
			ID:                 fmt.Sprintf("clawhub-mirror-%d", i+1),
			Type:               "clawhub",
			BaseURL:            strings.TrimRight(mirrorURL, "/"),
			DisplayName:        "ClawHub Mirror",
			SourceGroup:        "clawhub",
			MirrorOf:           "clawhub",
			AuthMode:           "none",
			Enabled:            enabled,
			RateLimitPerMinute: 60,
			Priority:           11 + i,
		})
	}
	for i, discoveryPageURL := range s.cfg.DiscoveryPageURLs {
		defaults = append(defaults, Source{
			ID:                 fmt.Sprintf("seed-%d", i+1),
			Type:               "seed_page",
			BaseURL:            discoveryPageURL,
			DisplayName:        fmt.Sprintf("Discovery Page %d", i+1),
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
	return s.disableDeprecatedDefaultSources(ctx)
}

func (s *Service) disableDeprecatedDefaultSources(ctx context.Context) error {
	for _, source := range deprecatedDefaultGitHubCodeSearchSources {
		if err := s.store.SetSourceEnabledByIdentity(ctx, source.ID, source.Type, source.BaseURL, false); err != nil {
			return err
		}
	}
	return s.disableDeprecatedDefaultAwesomeSeeds(ctx)
}

func (s *Service) disableDeprecatedDefaultAwesomeSeeds(ctx context.Context) error {
	currentDiscoveryPages := make(map[string]struct{}, len(s.cfg.DiscoveryPageURLs))
	for _, discoveryPageURL := range s.cfg.DiscoveryPageURLs {
		normalized := strings.TrimRight(strings.TrimSpace(discoveryPageURL), "/")
		if normalized == "" {
			continue
		}
		currentDiscoveryPages[normalized] = struct{}{}
	}

	for i, discoveryPageURL := range deprecatedAwesomeDiscoveryPageURLs {
		normalized := strings.TrimRight(strings.TrimSpace(discoveryPageURL), "/")
		if normalized == "" {
			continue
		}
		if _, keep := currentDiscoveryPages[normalized]; keep {
			continue
		}
		if err := s.store.SetSourceEnabledByIdentity(
			ctx,
			fmt.Sprintf("seed-%d", len(defaultDiscoveryPageURLs)+i+1),
			"seed_page",
			discoveryPageURL,
			false,
		); err != nil {
			return err
		}
	}
	return nil
}

func isTencentSkillHubMirrorURL(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, "lightmake.site") || strings.Contains(lower, "skillhub.tencent.com") {
		return true
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return strings.Contains(host, "lightmake.site") || strings.Contains(host, "skillhub.tencent.com")
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

const nonPagedIngestBatchSize = 50

func (s *Service) flushPreparedBatch(
	ctx context.Context,
	run *CrawlRun,
	batch []*SkillUpsertRecord,
) (int, int, int, error) {
	if len(batch) == 0 {
		return 0, 0, 0, nil
	}
	chunkSize := s.cfg.IngestBatchSize
	if chunkSize <= 0 {
		chunkSize = nonPagedIngestBatchSize
	}
	inserted, updated := 0, 0
	for start := 0; start < len(batch); start += chunkSize {
		end := start + chunkSize
		if end > len(batch) {
			end = len(batch)
		}
		result, err := s.store.UpsertSkillBatch(ctx, batch[start:end])
		if err != nil {
			if run != nil {
				run.Failed += end - start
			}
			return inserted, updated, len(batch[start:end]), err
		}
		if result == nil {
			continue
		}
		inserted += result.Inserted
		updated += result.Updated
		if run != nil {
			run.Discovered += result.Inserted
			run.Updated += result.Updated
		}
	}
	return inserted, updated, 0, nil
}

func (s *Service) discoverFromGitHub(ctx context.Context, source Source, processedSources int, total *DiscoverResult, run *CrawlRun) error {
	job, err := buildDiscoverJob(s, source, run)
	if err != nil {
		return err
	}
	return s.runDiscoverJobToCompletion(ctx, job)
}

func (s *Service) fetchGitHubBlob(ctx context.Context, apiURL string) (string, error) {
	raw, _, err := s.fetchGitHubBlobWithFallback(ctx, apiURL, "")
	return raw, err
}

func (s *Service) fetchGitHubBlobWithFallback(ctx context.Context, apiURL, htmlURL string) (string, int, error) {
	requests := 0
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", requests, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if token := strings.TrimSpace(s.cfg.GitHubToken); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	requests++
	resp, err := s.httpClient.Do(req)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			var payload gitHubContentResponse
			if err := json.NewDecoder(resp.Body).Decode(&payload); err == nil {
				if strings.EqualFold(payload.Encoding, "base64") {
					data, decodeErr := decodeBase64(strings.ReplaceAll(payload.Content, "\n", ""))
					if decodeErr == nil {
						return string(data), requests, nil
					}
					err = decodeErr
				} else {
					return payload.Content, requests, nil
				}
			}
		} else {
			err = fmt.Errorf("github blob status: %d", resp.StatusCode)
		}
	}
	if owner, repo, ref, path, ok := skillbundle.ParseGitHubBlobURL(htmlURL); ok {
		raw, fallbackRequests, fallbackErr := s.fetchGitHubRawContent(ctx, owner, repo, ref, path)
		requests += fallbackRequests
		if fallbackErr == nil {
			return raw, requests, nil
		}
		if err != nil {
			return "", requests, fmt.Errorf("%w (github raw fallback failed: %v)", err, fallbackErr)
		}
		return "", requests, fallbackErr
	}
	return "", requests, err
}

func (s *Service) fetchGitHubRawContent(ctx context.Context, owner, repo, ref, path string) (string, int, error) {
	requests := 0
	var lastErr error
	for _, rawURL := range skillbundle.GitHubRawURLCandidates(owner, repo, ref, path) {
		requests++
		read, err := s.remoteReader.ReadURL(ctx, RemoteReadRequest{
			URL:      rawURL,
			Format:   "text",
			MaxChars: 256_000,
		})
		if err != nil {
			lastErr = err
			continue
		}
		return read.Content, requests, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("github raw content unavailable")
	}
	return "", requests, lastErr
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

type lightmakeListResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Skills []lightmakeSkill `json:"skills"`
		Total  int              `json:"total"`
	} `json:"data"`
}

type lightmakeSkill struct {
	Category      string   `json:"category"`
	Description   string   `json:"description"`
	DescriptionZH string   `json:"description_zh"`
	Downloads     int      `json:"downloads"`
	Homepage      string   `json:"homepage"`
	Installs      int      `json:"installs"`
	Name          string   `json:"name"`
	OwnerName     string   `json:"ownerName"`
	Score         float64  `json:"score"`
	Slug          string   `json:"slug"`
	Stars         int      `json:"stars"`
	Tags          []string `json:"tags"`
	UpdatedAt     int64    `json:"updated_at"`
	Version       string   `json:"version"`
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

func (s *Service) discoverFromClawHub(ctx context.Context, source Source, processedSources int, total *DiscoverResult, run *CrawlRun) error {
	job, err := buildDiscoverJob(s, source, run)
	if err != nil {
		return err
	}
	return s.runDiscoverJobToCompletion(ctx, job)
}

func (s *Service) discoverFromLightmake(ctx context.Context, source Source, processedSources int, total *DiscoverResult, run *CrawlRun) error {
	job, err := buildDiscoverJob(s, source, run)
	if err != nil {
		return err
	}
	return s.runDiscoverJobToCompletion(ctx, job)
}

func lightmakeDescription(item lightmakeSkill) string {
	description := strings.TrimSpace(item.Description)
	if description == "" {
		description = strings.TrimSpace(item.DescriptionZH)
	}
	if description == "" {
		description = "Tencent SkillHub catalog entry"
	}
	return description
}

func lightmakeSkillMarkdown(item lightmakeSkill) string {
	name := strings.TrimSpace(defaultString(item.Name, item.Slug))
	description := escapeYAMLText(lightmakeDescription(item))
	version := escapeYAMLText(defaultString(strings.TrimSpace(item.Version), "catalog"))
	category := escapeYAMLText(strings.TrimSpace(item.Category))
	if category == "" {
		category = "productivity"
	}
	tags := ""
	if len(item.Tags) > 0 {
		normalizedTags := make([]string, 0, len(item.Tags))
		for _, tag := range item.Tags {
			tag = escapeYAMLText(tag)
			if tag == "" {
				continue
			}
			normalizedTags = append(normalizedTags, tag)
		}
		if len(normalizedTags) > 0 {
			tags = "\ntags: [" + strings.Join(normalizedTags, ", ") + "]"
		}
	}
	return fmt.Sprintf(`---
id: %s
name: %s
version: %s
description: %s
author: %s
category: %s%s
---

# %s

%s
`, normalizeSkillID(item.Slug), escapeYAMLText(name), version, description, escapeYAMLText(item.OwnerName), category, tags, name, lightmakeDescription(item))
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
	// Extract security labels from response if present (e.g., ["Benign"]).
	// ClawHub may attach labels at the top level or under latestVersion.
	if labels := collectNamedStringList(raw, "securityLabels", "security_labels"); len(labels) > 0 {
		if badgeFromLabels := securityLabelsToBadge(labels); badgeFromLabels != "" {
			signals.SecurityBadge = moreSevereBadge(signals.SecurityBadge, badgeFromLabels)
		}
	}
	if latest := namedMap(raw, "latestVersion"); len(latest) > 0 {
		if labels := collectNamedStringList(latest, "securityLabels", "security_labels"); len(labels) > 0 {
			if badgeFromLabels := securityLabelsToBadge(labels); badgeFromLabels != "" {
				signals.SecurityBadge = moreSevereBadge(signals.SecurityBadge, badgeFromLabels)
			}
		}
	}
	if !hasSourceSecuritySignals(signals) {
		return nil
	}
	ensureClawHubSecuritySummary(signals)
	signals.Findings = dedupeSecurityFindings(signals.Findings)
	signals.Evidence = dedupeSecurityEvidence(signals.Evidence)
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
	if labels := collectNamedStringList(node, "securityLabels", "security_labels", "securityLabel", "security_label"); len(labels) > 0 {
		if badgeFromLabels := securityLabelsToBadge(labels); badgeFromLabels != "" {
			signals.SecurityBadge = moreSevereBadge(signals.SecurityBadge, badgeFromLabels)
		}
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
	signals.Findings = append(signals.Findings, parseClawHubFindings(node, "findings")...)
	signals.Findings = append(signals.Findings, parseClawHubFindings(node, "issues")...)
	signals.Findings = append(signals.Findings, parseClawHubFindings(node, "risks")...)
	signals.Findings = append(signals.Findings, parseClawHubFindings(node, "warnings")...)
	if summary := extractClawHubSecuritySummary(node); summary != "" {
		signals.Evidence = append(signals.Evidence, SecurityEvidence{
			Type:        "security_summary",
			Severity:    "low",
			Title:       "Source scan summary",
			Description: summary,
			Value:       summary,
		})
	}
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

// securityLabelsToBadge converts security labels (e.g., ["Benign", "Verified"]) to badge values
func securityLabelsToBadge(labels []string) string {
	for _, label := range labels {
		switch strings.TrimSpace(strings.ToLower(label)) {
		case "benign", "safe", "verified", "trusted":
			return BadgeGreen
		case "suspicious", "caution", "review":
			return BadgeYellow
		case "malicious", "dangerous", "blocked", "banned":
			return BadgeRed
		}
	}
	return ""
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

func parseClawHubFindings(node map[string]interface{}, key string) []SecurityFinding {
	value, ok := node[key]
	if !ok {
		return nil
	}
	defaultType := normalizeExternalFindingType(key)
	defaultSeverity := defaultExternalFindingSeverity(key)
	switch v := value.(type) {
	case []interface{}:
		out := make([]SecurityFinding, 0, len(v))
		for _, item := range v {
			switch finding := item.(type) {
			case string:
				message := strings.TrimSpace(finding)
				if message == "" {
					continue
				}
				out = append(out, SecurityFinding{
					Type:     defaultType,
					Severity: defaultSeverity,
					Message:  message,
				})
			case map[string]interface{}:
				message := firstNonBlank(
					stringFromValue(finding["message"]),
					stringFromValue(finding["title"]),
					stringFromValue(finding["description"]),
					stringFromValue(finding["detail"]),
					stringFromValue(finding["value"]),
				)
				if message == "" {
					continue
				}
				out = append(out, SecurityFinding{
					Type:       firstNonBlank(normalizeExternalFindingType(namedString(finding, "type", "kind", "category", "id")), defaultType),
					Severity:   normalizeExternalFindingSeverity(namedString(finding, "severity", "level", "status"), defaultSeverity),
					Pattern:    namedString(finding, "pattern", "rule", "code"),
					Message:    message,
					Command:    namedString(finding, "command", "cmd"),
					Permission: namedString(finding, "permission", "scope"),
				})
			}
		}
		return out
	case string:
		message := strings.TrimSpace(v)
		if message == "" {
			return nil
		}
		return []SecurityFinding{{
			Type:     defaultType,
			Severity: defaultSeverity,
			Message:  message,
		}}
	default:
		return nil
	}
}

func extractClawHubSecuritySummary(node map[string]interface{}) string {
	if len(node) == 0 {
		return ""
	}
	for _, key := range []string{"riskSummary", "risk_summary", "overview", "verdict", "conclusion", "note"} {
		if text := namedString(node, key); text != "" {
			return text
		}
	}
	if value, ok := node["summary"]; ok {
		if text := clawHubSummaryText(value); text != "" {
			return text
		}
	}
	if value, ok := node["notes"]; ok {
		if text := clawHubSummaryText(value); text != "" {
			return text
		}
	}
	return ""
}

func clawHubSummaryText(value interface{}) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case []interface{}:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if text := clawHubSummaryText(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(dedupeStrings(parts), "; ")
	case map[string]interface{}:
		return firstNonBlank(
			namedString(v, "message", "text", "description", "detail", "summary", "result", "verdict", "reason", "status"),
			stringFromValue(v["value"]),
		)
	default:
		return ""
	}
}

func ensureClawHubSecuritySummary(signals *SourceSecuritySignals) {
	if signals == nil {
		return
	}
	severity := clawHubSecuritySummarySeverity(signals)
	for i := range signals.Evidence {
		if signals.Evidence[i].Type != "security_summary" {
			continue
		}
		signals.Evidence[i].Severity = severity
		if strings.TrimSpace(signals.Evidence[i].Title) == "" {
			signals.Evidence[i].Title = "Source scan summary"
		}
		signals.Evidence[i].Description = firstNonBlank(signals.Evidence[i].Description, signals.Evidence[i].Value)
		signals.Evidence[i].Value = firstNonBlank(signals.Evidence[i].Value, signals.Evidence[i].Description)
		return
	}
	summary := synthesizeClawHubSecuritySummary(signals)
	if summary == "" {
		return
	}
	signals.Evidence = append(signals.Evidence, SecurityEvidence{
		Type:        "security_summary",
		Severity:    severity,
		Title:       "Source scan summary",
		Description: summary,
		Value:       summary,
	})
}

func synthesizeClawHubSecuritySummary(signals *SourceSecuritySignals) string {
	if signals == nil {
		return ""
	}
	reasons := make([]string, 0, 5)
	if signals.HasVulnerabilities {
		reasons = append(reasons, "dependency vulnerabilities")
	}
	if signals.HasPromptInjection {
		reasons = append(reasons, "prompt injection")
	}
	if signals.HasShellInjection {
		reasons = append(reasons, "command injection")
	}
	if signals.HasDataExfiltration {
		reasons = append(reasons, "data exfiltration")
	}
	if signals.HasBinary {
		reasons = append(reasons, "binary artifacts")
	}
	if len(reasons) > 0 {
		return fmt.Sprintf("ClawHub source scan flagged risk: %s.", strings.Join(reasons, ", "))
	}
	switch signals.RiskLevel {
	case RiskCritical:
		return "ClawHub source scan marked this skill as critical risk."
	case RiskHigh:
		return "ClawHub source scan marked this skill as high risk."
	case RiskMedium:
		return "ClawHub source scan marked this skill as medium risk."
	case RiskLow:
		return "ClawHub source scan did not flag major risks."
	}
	switch signals.SecurityBadge {
	case BadgeRed:
		return "ClawHub source scan marked this skill as blocked or high risk."
	case BadgeYellow:
		return "ClawHub source scan flagged potential risks for review."
	case BadgeGreen:
		return "ClawHub source scan did not flag major risks."
	}
	if signals.Score != nil {
		switch {
		case *signals.Score >= 85:
			return "ClawHub source scan did not flag major risks."
		case *signals.Score >= 70:
			return "ClawHub source scan flagged potential risks for review."
		default:
			return "ClawHub source scan flagged potential risks."
		}
	}
	return "ClawHub source scan did not provide an explicit risk verdict."
}

func clawHubSecuritySummarySeverity(signals *SourceSecuritySignals) string {
	if signals == nil {
		return "low"
	}
	switch {
	case signals.HasVulnerabilities || signals.HasPromptInjection || signals.HasShellInjection || signals.HasDataExfiltration:
		return "high"
	case signals.RiskLevel == RiskCritical || signals.RiskLevel == RiskHigh || signals.SecurityBadge == BadgeRed:
		return "high"
	case signals.RiskLevel == RiskMedium || signals.SecurityBadge == BadgeYellow || signals.HasBinary:
		return "medium"
	default:
		return "low"
	}
}

func normalizeExternalFindingType(value string) string {
	normalized := strings.TrimSpace(strings.ToLower(value))
	normalized = strings.ReplaceAll(normalized, " ", "_")
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, "/", "_")
	if normalized == "" {
		return "external_finding"
	}
	switch normalized {
	case "finding", "findings", "issue", "issues", "risk", "risks", "warning", "warnings":
		return "external_finding"
	default:
		return normalized
	}
}

func normalizeExternalFindingSeverity(value, fallback string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "critical", "high", "medium", "low", "info":
		return strings.TrimSpace(strings.ToLower(value))
	}
	switch normalizeExternalRiskLevel(value) {
	case RiskCritical:
		return "critical"
	case RiskHigh:
		return "high"
	case RiskMedium:
		return "medium"
	case RiskLow:
		return "low"
	default:
		return fallback
	}
}

func defaultExternalFindingSeverity(key string) string {
	switch strings.TrimSpace(strings.ToLower(key)) {
	case "risks", "issues":
		return "high"
	case "warnings":
		return "medium"
	default:
		return "medium"
	}
}

func dedupeSecurityFindings(items []SecurityFinding) []SecurityFinding {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	out := make([]SecurityFinding, 0, len(items))
	for _, item := range items {
		key := strings.ToLower(strings.Join([]string{
			item.Type,
			item.Severity,
			item.Pattern,
			item.Message,
			item.Command,
			item.Permission,
		}, "\x00"))
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func dedupeSecurityEvidence(items []SecurityEvidence) []SecurityEvidence {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	out := make([]SecurityEvidence, 0, len(items))
	for _, item := range items {
		key := strings.ToLower(strings.Join([]string{
			item.Type,
			item.Severity,
			item.Title,
			item.Description,
			item.Value,
		}, "\x00"))
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func emptySearchResponse(query SearchQuery) *SearchResponse {
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 || pageSize > DefaultSearchLimit {
		pageSize = DefaultPageSize
	}
	return &SearchResponse{
		Skills:     nil,
		Total:      0,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: 0,
	}
}

func buildSkillSearchDoc(doc *SkillDocument) string {
	if doc == nil {
		return ""
	}

	name := strings.TrimSpace(doc.Name)
	description := strings.TrimSpace(doc.Description)
	author := strings.TrimSpace(doc.Author)
	category := strings.TrimSpace(doc.Category)
	tags := append([]string(nil), doc.Tags...)
	content := strings.TrimSpace(doc.SkillContent)

	if parsed, err := parseSkillMarkdown(content, firstNonBlank(doc.ID, doc.Slug)); err == nil && parsed != nil {
		content = strings.TrimSpace(parsed.Content)
		if name == "" {
			name = strings.TrimSpace(parsed.Manifest.Name)
		}
		if description == "" {
			description = strings.TrimSpace(parsed.Manifest.Description)
		}
		if author == "" {
			author = strings.TrimSpace(parsed.Manifest.Author)
		}
		if category == "" {
			category = strings.TrimSpace(parsed.Manifest.Category)
		}
		if len(tags) == 0 {
			tags = append(tags, parsed.Manifest.Tags...)
		}
	}

	return strings.TrimSpace(strings.Join([]string{
		name,
		description,
		author,
		category,
		strings.Join(tags, " "),
		content,
	}, "\n"))
}

func skillEmbeddingNeedsRefresh(doc SkillDocument, currentModel string) bool {
	if strings.TrimSpace(doc.EmbeddingJSON) == "" {
		return true
	}
	currentModel = strings.TrimSpace(currentModel)
	if currentModel == "" {
		return false
	}
	return strings.TrimSpace(doc.EmbeddingModel) != currentModel
}

func (s *Service) ensureSkillEmbeddings(ctx context.Context, docs []SkillDocument) ([]SkillDocument, error) {
	if s.embeddingProvider == nil || len(docs) == 0 {
		return docs, nil
	}

	s.embeddingBackfillMu.Lock()
	defer s.embeddingBackfillMu.Unlock()

	currentModel := strings.TrimSpace(s.embeddingProvider.Model())
	type pendingEmbedding struct {
		index     int
		searchDoc string
	}
	pending := make([]pendingEmbedding, 0, len(docs))
	for i := range docs {
		if !skillEmbeddingNeedsRefresh(docs[i], currentModel) {
			continue
		}
		searchDoc := buildSkillSearchDoc(&docs[i])
		if strings.TrimSpace(searchDoc) == "" {
			continue
		}
		pending = append(pending, pendingEmbedding{index: i, searchDoc: searchDoc})
	}
	if len(pending) == 0 {
		return docs, nil
	}

	s.startEmbeddingProgress(len(pending))
	var firstErr error

	for _, item := range pending {
		doc := &docs[item.index]
		vec, err := s.embeddingProvider.Embed(ctx, item.searchDoc)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			s.advanceEmbeddingProgress(*doc, false, err)
			continue
		}
		if len(vec) == 0 {
			err := fmt.Errorf("embedding provider returned empty vector for %s", firstNonBlank(doc.Name, doc.ID))
			if firstErr == nil {
				firstErr = err
			}
			s.advanceEmbeddingProgress(*doc, false, err)
			continue
		}
		data, err := json.Marshal(vec)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			s.advanceEmbeddingProgress(*doc, false, err)
			continue
		}
		doc.EmbeddingJSON = string(data)
		doc.EmbeddingModel = currentModel
		if err := s.store.UpdateSkillEmbedding(ctx, doc.ID, doc.EmbeddingJSON, doc.EmbeddingModel); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			s.advanceEmbeddingProgress(*doc, false, err)
			continue
		}
		s.advanceEmbeddingProgress(*doc, true, nil)
	}

	s.finishEmbeddingProgress(firstErr)
	return docs, firstErr
}

func (s *Service) backfillSkillEmbeddings(ctx context.Context, query SearchQuery) error {
	candidates, err := s.store.ListFilteredSkills(ctx, query)
	if err != nil {
		return err
	}
	_, err = s.ensureSkillEmbeddings(ctx, candidates)
	return err
}

func (s *Service) combineSemanticResults(query SearchQuery, queryVector []float32, candidates []SkillDocument, keywordResults []SearchResult) *SearchResponse {
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 || pageSize > DefaultSearchLimit {
		pageSize = DefaultPageSize
	}

	semanticRatio := s.cfg.SemanticRatio
	if semanticRatio <= 0 || semanticRatio >= 1 {
		semanticRatio = 0.35
	}
	keywordWeight := 1 - semanticRatio

	merged := make(map[string]SearchResult, len(keywordResults)+len(candidates))
	for _, result := range keywordResults {
		merged[result.Skill.ID] = result
	}

	for _, doc := range candidates {
		if strings.TrimSpace(doc.EmbeddingJSON) == "" {
			continue
		}
		var candidate []float32
		if err := json.Unmarshal([]byte(doc.EmbeddingJSON), &candidate); err != nil || len(candidate) == 0 {
			continue
		}
		score, err := embedding.CosineSimilarity(queryVector, candidate)
		if err != nil || score <= 0 {
			continue
		}

		result, exists := merged[doc.ID]
		if !exists {
			result = SearchResult{
				Skill:       doc,
				Score:       float64(score) * 100,
				MatchSource: "semantic",
			}
		} else {
			result.Skill = doc
		}

		result.SemanticScore = float64(score)
		semanticScore := float64(score) * 100
		if exists {
			result.Score = (result.KeywordScore * keywordWeight) + (semanticScore * semanticRatio)
			result.MatchSource = "hybrid"
		} else {
			result.Score = semanticScore
		}
		merged[doc.ID] = result
	}

	results := make([]SearchResult, 0, len(merged))
	for _, result := range merged {
		results = append(results, result)
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			leftRank := results[i].Skill.CuratedRank
			rightRank := results[j].Skill.CuratedRank
			if leftRank != rightRank {
				if leftRank == 0 {
					return false
				}
				if rightRank == 0 {
					return true
				}
				return leftRank < rightRank
			}
			leftTrend := results[i].Skill.TrendingScore + results[i].Skill.CuratedBoost
			rightTrend := results[j].Skill.TrendingScore + results[j].Skill.CuratedBoost
			if leftTrend != rightTrend {
				return leftTrend > rightTrend
			}
			return results[i].Skill.Name < results[j].Skill.Name
		}
		return results[i].Score > results[j].Score
	})

	total := len(results)
	start := (page - 1) * pageSize
	if start >= total {
		return &SearchResponse{
			Skills:     []SearchResult{},
			Total:      total,
			Page:       page,
			PageSize:   pageSize,
			TotalPages: int(math.Ceil(float64(total) / float64(pageSize))),
		}
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	return &SearchResponse{
		Skills:     results[start:end],
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: int(math.Ceil(float64(total) / float64(pageSize))),
	}
}

func (s *Service) discoverFromSeedPage(ctx context.Context, source Source, processedSources int, total *DiscoverResult, run *CrawlRun) error {
	job, err := buildDiscoverJob(s, source, run)
	if err != nil {
		return err
	}
	return s.runDiscoverJobToCompletion(ctx, job)
}

type ingestRequest struct {
	SourceID         string
	SourceName       string
	SourceGroup      string
	OriginSourceID   string
	OriginSourceName string
	OriginSourceURL  string
	SourceType       string
	RepoURL          string
	Homepage         string
	DownloadURL      string
	SourceURL        string
	SkillPath        string
	SkillContent     string
	CommitHash       string
	Stars            int
	Downloads        int
	LastUpdated      time.Time
	DefaultSkillID   string
	ExplicitID       string
	ExplicitName     string
	ExplicitVersion  string
	ExplicitAuthor   string
	DescriptionHint  string
	CategoryHint     string
	AdditionalTags   []string
	SecuritySignals  *SourceSecuritySignals
	Installable      bool
	InstallType      string
	ArtifactKind     string
	HasBinary        bool
	HasScripts       bool
}

func (s *Service) prepareIngestRecord(ctx context.Context, req ingestRequest) (*SkillUpsertRecord, error) {
	fallbackID := req.ExplicitID
	if fallbackID == "" {
		fallbackID = req.DefaultSkillID
	}
	parsed, err := parseSkillMarkdown(req.SkillContent, fallbackID)
	if err != nil {
		return nil, err
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
	sourceID, sourceName, sourceGroup := normalizeDiscoveredSkillSource(req)
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
		SourceID:            sourceID,
		SourceName:          sourceName,
		SourceGroup:         sourceGroup,
		OriginSourceID:      req.OriginSourceID,
		OriginSourceName:    req.OriginSourceName,
		OriginSourceURL:     req.OriginSourceURL,
		SourceType:          req.SourceType,
		SkillPath:           req.SkillPath,
		SkillContent:        req.SkillContent,
		LastUpdated:         req.LastUpdated,
		LastCrawledAt:       timeutil.NowTime(),
	}
	if s.embeddingProvider != nil {
		vec, err := s.embeddingProvider.Embed(ctx, buildSkillSearchDoc(doc))
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
	return &SkillUpsertRecord{
		Doc:     doc,
		Version: version,
		Report:  report,
	}, nil
}

func normalizeDiscoveredSkillSource(req ingestRequest) (string, string, string) {
	if isVercelSkillsReference(req.RepoURL, req.Homepage, req.DownloadURL, req.SourceURL) {
		return "vercel", "Vercel", "vercel"
	}
	if strings.TrimSpace(req.SourceGroup) == GitHubAwesomeSkillsSourceGroup {
		return GitHubAwesomeSkillsSourceID, GitHubAwesomeSkillsSourceName, GitHubAwesomeSkillsSourceGroup
	}
	return req.SourceID, req.SourceName, req.SourceGroup
}

func isVercelSkillsReference(values ...string) bool {
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if normalized == "" {
			continue
		}
		if strings.Contains(normalized, "github.com/vercel-labs/skills") {
			return true
		}
		if strings.Contains(normalized, "raw.githubusercontent.com/vercel-labs/skills/") {
			return true
		}
		if strings.Contains(normalized, "raw.gitmirror.com/vercel-labs/skills/") {
			return true
		}
		if strings.Contains(normalized, "cdn.jsdelivr.net/gh/vercel-labs/skills@") {
			return true
		}
	}
	return false
}

func (s *Service) ingestSkillContent(ctx context.Context, req ingestRequest) (bool, error) {
	record, err := s.prepareIngestRecord(ctx, req)
	if err != nil {
		return false, err
	}
	result, err := s.store.UpsertSkillBatch(ctx, []*SkillUpsertRecord{record})
	if err != nil {
		return false, err
	}
	return result != nil && result.Updated > 0, nil
}

func (s *Service) materializeVersion(tempDir string, doc SkillDocument, version *SkillVersion) error {
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return err
	}
	if version.RawSkillMD != "" {
		entryPath := filepath.Join(tempDir, "SKILL.md")
		if trimmedPath := strings.TrimSpace(version.SkillPath); trimmedPath != "" && skillbundle.IsEntryDocumentName(filepath.Base(trimmedPath)) {
			relativePath := filepath.Clean(strings.TrimPrefix(trimmedPath, "/"))
			entryPath = filepath.Join(tempDir, relativePath)
			if err := os.MkdirAll(filepath.Dir(entryPath), 0o755); err != nil {
				return err
			}
		}
		if err := os.WriteFile(entryPath, []byte(version.RawSkillMD), 0o644); err != nil {
			return err
		}
		if _, err := skillbundle.EnsureCompatibilitySkillDoc(tempDir, entryPath); err != nil {
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

func shouldInstallFromArchive(doc SkillDocument, version *SkillVersion) bool {
	if normalizeInstallType(doc.InstallType) == InstallTypeSourceArchive {
		return true
	}
	if looksLikeArchiveURL(doc.DownloadURL) {
		return true
	}
	if version == nil {
		return false
	}
	return looksLikeArchiveURL(version.SourceURL)
}

func (s *Service) materializeArchiveVersion(ctx context.Context, doc SkillDocument, version *SkillVersion, tempDir string) (string, SkillVersion, []string, error) {
	downloadURL := strings.TrimSpace(doc.DownloadURL)
	if downloadURL == "" && version != nil {
		downloadURL = strings.TrimSpace(version.SourceURL)
	}
	if downloadURL == "" {
		return "", SkillVersion{}, nil, fmt.Errorf("skill archive download url unavailable")
	}

	archivePath, finalURL, contentType, err := s.downloadArchive(ctx, downloadURL, tempDir)
	if err != nil {
		return "", SkillVersion{}, nil, err
	}
	extractDir := filepath.Join(tempDir, "archive")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		return "", SkillVersion{}, nil, err
	}
	if err := extractArchiveFile(archivePath, extractDir, finalURL, contentType); err != nil {
		return "", SkillVersion{}, nil, err
	}

	bundle, err := skillmanifest.ValidateArchiveInstallRoot(extractDir, doc.ID, skillmanifest.Options{
		RequireContract:     false,
		AllowLegacyFallback: true,
	})
	if err != nil {
		return "", SkillVersion{}, nil, err
	}
	installRoot := bundle.Root
	rawBytes := bundle.Raw
	parsed, err := parseSkillMarkdown(string(rawBytes), doc.ID)
	if err != nil {
		return "", SkillVersion{}, nil, err
	}
	if normalizeSkillID(parsed.Manifest.ID) != normalizeSkillID(doc.ID) {
		return "", SkillVersion{}, nil, fmt.Errorf("archive skill id mismatch: expected %s, found %s", doc.ID, parsed.Manifest.ID)
	}
	archiveWarnings := []string{}
	if bundle != nil && bundle.Document.Manifest != nil && bundle.Document.Manifest.Metadata != nil {
		raw := strings.TrimSpace(bundle.Document.Manifest.Metadata["validation_notes"])
		if raw != "" {
			archiveWarnings = dedupeStrings(append(archiveWarnings, strings.Split(raw, "\n")...))
			parsed.Manifest.Metadata["validation_notes"] = raw
		}
	}
	if err := writeManifestJSON(installRoot, parsed.Manifest); err != nil {
		return "", SkillVersion{}, nil, err
	}

	installVersion := SkillVersion{}
	if version != nil {
		installVersion = *version
	}
	if installVersion.Version != "" && installVersion.Version != parsed.Manifest.Version {
		installVersion.ID = uuid.NewString()
		installVersion.CreatedAt = time.Time{}
		installVersion.UpdatedAt = time.Time{}
		installVersion.ReleasedAt = timeutil.NowTime()
	}
	installVersion.SkillID = doc.ID
	installVersion.Version = parsed.Manifest.Version
	installVersion.SourceURL = downloadURL
	installVersion.Checksum = parsed.Checksum
	installVersion.SkillPath = "SKILL.md"
	installVersion.RawSkillMD = string(rawBytes)
	installVersion.ManifestJSON = manifestJSON(parsed.Manifest)
	if installVersion.ID == "" {
		installVersion.ID = uuid.NewString()
	}
	if installVersion.ReleasedAt.IsZero() {
		installVersion.ReleasedAt = timeutil.NowTime()
	}
	installVersion.ScannedAt = timeutil.NowTime()

	return installRoot, installVersion, archiveWarnings, nil
}

func buildInstalledDocument(doc SkillDocument, version *SkillVersion, report *SecurityReport) (SkillDocument, error) {
	if version == nil {
		return doc, fmt.Errorf("skill version is required")
	}
	parsed, err := parseSkillMarkdown(version.RawSkillMD, doc.ID)
	if err != nil {
		return doc, err
	}
	doc.Name = defaultString(strings.TrimSpace(parsed.Manifest.Name), doc.Name)
	doc.Description = defaultString(strings.TrimSpace(parsed.Manifest.Description), doc.Description)
	if strings.TrimSpace(parsed.Manifest.Author) != "" {
		doc.Author = strings.TrimSpace(parsed.Manifest.Author)
	}
	doc.Category = normalizeCategory(parsed.Manifest.Category, version.RawSkillMD, parsed.Manifest.Tags)
	doc.Tags = normalizeTags(doc.Category, parsed.Manifest.Tags)
	doc.LatestVersion = version.Version
	doc.ContentSHA256 = version.Checksum
	doc.SkillPath = version.SkillPath
	doc.SkillContent = version.RawSkillMD
	doc.LastCrawledAt = timeutil.NowTime()
	doc.Installable = true
	if report != nil {
		doc.SecurityScore = report.Score
		doc.Permissions = append([]string(nil), report.Permissions...)
		doc.RiskLevel = report.RiskLevel
		doc.SecurityBadge = report.SecurityBadge
		doc.VulnerabilityStatus = report.VulnerabilityStatus
		doc.HasVulnerabilities = report.HasVulnerabilities
		doc.HasPromptInjection = report.HasPromptInjection
		doc.HasShellInjection = report.HasShellInjection
		doc.HasDataExfiltration = report.HasDataExfiltration
		doc.HasBinary = report.InstallSurface.HasBinary
		doc.HasScripts = report.InstallSurface.HasScripts
		doc.InstallType = report.InstallSurface.InstallType
		doc.ArtifactKind = report.InstallSurface.ArtifactKind
		doc.ScanStatus = report.LLMStatus
	}
	return doc, nil
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
	seenFiles := make(map[string]struct{})
	for _, fileName := range []string{"SKILL.md", "manifest.json"} {
		path := filepath.Join(dir, fileName)
		data, err := os.ReadFile(path)
		if err == nil {
			parts = append(parts, string(data))
			seenFiles[path] = struct{}{}
		}
	}
	if entryDoc, err := skillbundle.FindEntryDocumentInDir(dir); err == nil {
		if _, exists := seenFiles[entryDoc.Path]; !exists {
			if data, readErr := os.ReadFile(entryDoc.Path); readErr == nil {
				parts = append(parts, string(data))
			}
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

func (s *Service) downloadArchive(ctx context.Context, downloadURL, tempDir string) (string, string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return "", "", "", err
	}
	req.Header.Set("Accept", "application/zip, application/gzip, application/octet-stream")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", "", "", fmt.Errorf("archive download status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	finalURL := downloadURL
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	archivePath := filepath.Join(tempDir, "download"+archiveExtension(finalURL, downloadURL, resp.Header.Get("Content-Type")))
	file, err := os.OpenFile(archivePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return "", "", "", err
	}
	defer file.Close()

	written, err := io.Copy(file, io.LimitReader(resp.Body, maxArchiveDownloadBytes+1))
	if err != nil {
		return "", "", "", err
	}
	if written > maxArchiveDownloadBytes {
		return "", "", "", fmt.Errorf("archive exceeds %d bytes", maxArchiveDownloadBytes)
	}

	return archivePath, finalURL, resp.Header.Get("Content-Type"), nil
}

func archiveExtension(primaryURL, fallbackURL, contentType string) string {
	for _, raw := range []string{primaryURL, fallbackURL} {
		lower := strings.ToLower(strings.TrimSpace(raw))
		switch {
		case strings.Contains(lower, ".tar.gz"):
			return ".tar.gz"
		case strings.Contains(lower, ".tgz"):
			return ".tgz"
		case strings.Contains(lower, ".skill"):
			return ".skill"
		case strings.Contains(lower, ".zip"):
			return ".zip"
		}
	}
	lowerType := strings.ToLower(contentType)
	switch {
	case strings.Contains(lowerType, "zip"):
		return ".zip"
	case strings.Contains(lowerType, "gzip"), strings.Contains(lowerType, "tar"):
		return ".tar.gz"
	default:
		return ".archive"
	}
}

func extractArchiveFile(archivePath, extractDir, downloadURL, contentType string) error {
	format, err := detectArchiveFormat(archivePath, downloadURL, contentType)
	if err != nil {
		return err
	}
	switch format {
	case "zip":
		return extractZipArchive(archivePath, extractDir)
	case "tar.gz":
		return extractTarGzArchive(archivePath, extractDir)
	default:
		return fmt.Errorf("unsupported archive format: %s", format)
	}
}

func detectArchiveFormat(archivePath, downloadURL, contentType string) (string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	header := make([]byte, 4)
	n, err := file.Read(header)
	if err != nil && err != io.EOF {
		return "", err
	}
	header = header[:n]
	if len(header) >= 2 && header[0] == 'P' && header[1] == 'K' {
		return "zip", nil
	}
	if len(header) >= 2 && header[0] == 0x1f && header[1] == 0x8b {
		return "tar.gz", nil
	}

	lowerURL := strings.ToLower(strings.TrimSpace(downloadURL))
	switch {
	case strings.Contains(lowerURL, ".zip"):
		return "zip", nil
	case strings.Contains(lowerURL, ".tar.gz"), strings.Contains(lowerURL, ".tgz"):
		return "tar.gz", nil
	}

	lowerType := strings.ToLower(contentType)
	switch {
	case strings.Contains(lowerType, "zip"):
		return "zip", nil
	case strings.Contains(lowerType, "gzip"), strings.Contains(lowerType, "tar"):
		return "tar.gz", nil
	default:
		return "", fmt.Errorf("unsupported archive format for %s", downloadURL)
	}
}

func extractZipArchive(archivePath, extractDir string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		target, err := safeArchiveTarget(extractDir, file.Name)
		if err != nil {
			return err
		}
		if target == "" {
			continue
		}
		mode := file.Mode()
		if mode&os.ModeSymlink != 0 {
			return fmt.Errorf("zip symlink entry is not supported: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := file.Open()
		if err != nil {
			return err
		}
		perm := mode.Perm()
		if perm == 0 {
			perm = 0o644
		}
		dst, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := io.Copy(dst, rc); err != nil {
			dst.Close()
			rc.Close()
			return err
		}
		if err := dst.Close(); err != nil {
			rc.Close()
			return err
		}
		if err := rc.Close(); err != nil {
			return err
		}
	}
	return nil
}

func extractTarGzArchive(archivePath, extractDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzipReader.Close()

	reader := tar.NewReader(gzipReader)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target, err := safeArchiveTarget(extractDir, header.Name)
		if err != nil {
			return err
		}
		if target == "" {
			continue
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			perm := os.FileMode(header.Mode).Perm()
			if perm == 0 {
				perm = 0o644
			}
			dst, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
			if err != nil {
				return err
			}
			if _, err := io.Copy(dst, reader); err != nil {
				dst.Close()
				return err
			}
			if err := dst.Close(); err != nil {
				return err
			}
		case tar.TypeSymlink, tar.TypeLink:
			return fmt.Errorf("tar symlink entry is not supported: %s", header.Name)
		}
	}
}

func safeArchiveTarget(root, name string) (string, error) {
	cleaned := strings.TrimSpace(strings.ReplaceAll(name, "\\", "/"))
	cleaned = strings.TrimPrefix(cleaned, "/")
	cleaned = filepath.Clean(filepath.FromSlash(cleaned))
	if cleaned == "." || cleaned == "" {
		return "", nil
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("archive entry escapes destination: %s", name)
	}
	target := filepath.Join(root, cleaned)
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("archive entry escapes destination: %s", name)
	}
	return target, nil
}

type archiveSkillCandidate struct {
	dir       string
	path      string
	name      string
	depth     int
	matchesID bool
}

func findArchiveInstallRoot(extractDir, skillID string) (string, string, error) {
	candidates := make([]archiveSkillCandidate, 0, 4)
	if err := filepath.Walk(extractDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		base := info.Name()
		if !strings.EqualFold(base, "SKILL.md") && !strings.EqualFold(base, "CLAUDE.md") && !strings.EqualFold(base, "AGENT.md") {
			return nil
		}
		dir := filepath.Dir(path)
		rel, err := filepath.Rel(extractDir, dir)
		if err != nil {
			return nil
		}
		depth := 0
		if rel != "." {
			depth = len(strings.Split(filepath.ToSlash(rel), "/"))
		}
		candidates = append(candidates, archiveSkillCandidate{
			dir:       dir,
			path:      path,
			name:      base,
			depth:     depth,
			matchesID: normalizeSkillID(filepath.Base(dir)) == normalizeSkillID(skillID),
		})
		return nil
	}); err != nil {
		return "", "", err
	}
	if len(candidates) == 0 {
		return "", "", fmt.Errorf("archive does not contain SKILL.md")
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].matchesID != candidates[j].matchesID {
			return candidates[i].matchesID
		}
		leftIsSkill := strings.EqualFold(candidates[i].name, "SKILL.md")
		rightIsSkill := strings.EqualFold(candidates[j].name, "SKILL.md")
		if leftIsSkill != rightIsSkill {
			return leftIsSkill
		}
		if candidates[i].depth != candidates[j].depth {
			return candidates[i].depth < candidates[j].depth
		}
		return candidates[i].path < candidates[j].path
	})

	selected := candidates[0]
	skillFile := filepath.Join(selected.dir, "SKILL.md")
	if !strings.EqualFold(selected.name, "SKILL.md") || selected.path != skillFile {
		raw, err := os.ReadFile(selected.path)
		if err != nil {
			return "", "", err
		}
		if err := os.WriteFile(skillFile, raw, 0o644); err != nil {
			return "", "", err
		}
	}
	return selected.dir, skillFile, nil
}

func writeManifestJSON(root string, manifest *skill.Manifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "manifest.json"), data, 0o644)
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

func (s *Service) installFromGitHub(ctx context.Context, repo string, ackRisk bool, forceInstall bool) (*InstallResult, error) {
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
	return s.Install(ctx, InstallRequest{
		ID:           detail.Skill.ID,
		AckRisk:      ackRisk,
		ForceInstall: forceInstall,
	})
}

type gitHubRepoSkill struct {
	Path   string
	Raw    string
	Commit string
}

func (s *Service) fetchGitHubRepoSkill(ctx context.Context, owner, repo, branch string) (*gitHubRepoSkill, error) {
	return s.fetchGitHubRepoSkillAtPath(ctx, owner, repo, branch, "")
}

func (s *Service) fetchGitHubRepoSkillAtPath(ctx context.Context, owner, repo, branch, rootPath string) (*gitHubRepoSkill, error) {
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
			leftRank := skillbundle.EntryDocumentPriority(items[i].Name)
			rightRank := skillbundle.EntryDocumentPriority(items[j].Name)
			if leftRank != rightRank {
				return leftRank < rightRank
			}
			return items[i].Path < items[j].Path
		})
		for _, item := range items {
			if item.Type == "file" && skillbundle.IsEntryDocumentName(item.Name) {
				htmlURL := fmt.Sprintf("https://github.com/%s/%s/blob/%s/%s", owner, repo, branch, strings.TrimPrefix(item.Path, "/"))
				raw, _, err := s.fetchGitHubBlobWithFallback(ctx, item.URL, htmlURL)
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
	return walk(strings.Trim(strings.TrimSpace(rootPath), "/"))
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
