package skillstore

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// SyncService handles on-demand synchronization of skills from remote sources.
// Sync is triggered via the /skill-store/refresh API when the user enters the Skill Store page.
type SyncService struct {
	store         *Store
	httpClient    *http.Client
	sources       map[string]*Source
	mu            sync.RWMutex
	stopCh        chan struct{}
	interval      time.Duration
	logger        *slog.Logger
	readmeFetcher *ReadmeFetcher
	progressMu    sync.RWMutex
	progress      map[string]*SyncProgress // in-memory progress per source
	startOnce     sync.Once
	backgroundCtx context.Context
}

// Source represents a skill source configuration.
type Source struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	Type        string `json:"type"` // "clawhub", "github", "custom"
	Enabled     bool   `json:"enabled"`
	Description string `json:"description"`
}

// SyncServiceConfig holds configuration for the sync service.
type SyncServiceConfig struct {
	Interval time.Duration // Sync interval (default: 1 hour)
	Timeout  time.Duration // HTTP timeout (default: 30 seconds)
}

// DefaultSyncServiceConfig returns default configuration.
func DefaultSyncServiceConfig() SyncServiceConfig {
	return SyncServiceConfig{
		Interval: 1 * time.Hour, // Sync every hour
		Timeout:  30 * time.Second,
	}
}

// NewSyncService creates a new sync service.
func NewSyncService(store *Store, config SyncServiceConfig, log *slog.Logger) *SyncService {
	if config.Interval == 0 {
		config.Interval = 1 * time.Hour // Default to 1 hour
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	svc := &SyncService{
		store: store,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		sources:  make(map[string]*Source),
		stopCh:   make(chan struct{}),
		interval: config.Interval,
		logger:   log,
		progress: make(map[string]*SyncProgress),
	}

	// Initialize README fetcher
	svc.readmeFetcher = NewReadmeFetcher(store, log)

	// Register default sources
	svc.RegisterSource(&Source{
		ID:          "clawhub",
		Name:        "ClawHub",
		URL:         "https://www.clawhub.ai",
		Type:        "clawhub",
		Enabled:     true,
		Description: "Official ClawHub skill marketplace",
	})

	return svc
}

// RegisterSource registers a new skill source.
func (s *SyncService) RegisterSource(source *Source) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sources[source.ID] = source
}

// UnregisterSource removes a skill source.
func (s *SyncService) UnregisterSource(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sources, id)
}

// GetSources returns all registered sources.
func (s *SyncService) GetSources() []*Source {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sources := make([]*Source, 0, len(s.sources))
	for _, src := range s.sources {
		sources = append(sources, src)
	}
	return sources
}

// Start binds the lifecycle context used by lazy background workers.
// Sync is still triggered on-demand via the /skill-store/refresh API.
func (s *SyncService) Start(ctx context.Context) {
	s.mu.Lock()
	s.backgroundCtx = ctx
	s.mu.Unlock()
}

// Stop stops the sync service.
func (s *SyncService) Stop() {
	close(s.stopCh)

	// Stop README fetcher
	if s.readmeFetcher != nil {
		s.readmeFetcher.Stop()
	}
}

func (s *SyncService) ensureWorkersStarted(ctx context.Context) {
	s.startOnce.Do(func() {
		if s.readmeFetcher == nil {
			return
		}
		startCtx := ctx
		s.mu.RLock()
		if s.backgroundCtx != nil {
			startCtx = s.backgroundCtx
		}
		s.mu.RUnlock()
		if startCtx == nil {
			startCtx = context.Background()
		}
		s.readmeFetcher.Start(startCtx)
	})
}

// SyncAll synchronizes all enabled sources.
func (s *SyncService) SyncAll(ctx context.Context) {
	s.mu.RLock()
	sources := make([]*Source, 0, len(s.sources))
	for _, src := range s.sources {
		if src.Enabled {
			sources = append(sources, src)
		}
	}
	s.mu.RUnlock()

	for _, source := range sources {
		if err := s.SyncSource(ctx, source.ID); err != nil {
			if s.logger != nil {
				s.logger.Error("failed to sync source", "source", source.ID, "error", err)
			}
		}
	}
}

// SyncSource synchronizes a specific source.
func (s *SyncService) SyncSource(ctx context.Context, sourceID string) error {
	s.mu.RLock()
	source, exists := s.sources[sourceID]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("source not found: %s", sourceID)
	}

	// Check if already synced today (24-hour rate limiting)
	synced, err := s.hasSyncedToday(ctx, sourceID)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("failed to check sync status, proceeding with sync", "source", sourceID, "error", err)
		}
	} else if synced {
		if s.logger != nil {
			s.logger.Info("source already synced today, skipping", "source", sourceID)
		}
		return nil
	}

	return s.doSync(ctx, source)
}

// ForceSyncSource synchronizes a specific source, ignoring today's sync check.
func (s *SyncService) ForceSyncSource(ctx context.Context, sourceID string) error {
	s.mu.RLock()
	source, exists := s.sources[sourceID]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("source not found: %s", sourceID)
	}

	return s.doSync(ctx, source)
}

// ForceSyncAll synchronizes all enabled sources, ignoring today's sync check.
func (s *SyncService) ForceSyncAll(ctx context.Context) {
	s.mu.RLock()
	sources := make([]*Source, 0, len(s.sources))
	for _, src := range s.sources {
		if src.Enabled {
			sources = append(sources, src)
		}
	}
	s.mu.RUnlock()

	for _, source := range sources {
		if err := s.doSync(ctx, source); err != nil {
			if s.logger != nil {
				s.logger.Error("failed to sync source", "source", source.ID, "error", err)
			}
		}
	}
}

// doSync performs the actual sync operation for a source.
func (s *SyncService) doSync(ctx context.Context, source *Source) error {
	s.ensureWorkersStarted(ctx)
	startTime := timeutil.NowTime()

	// Set in-memory progress
	s.progressMu.Lock()
	s.progress[source.ID] = &SyncProgress{StartedAt: startTime}
	s.progressMu.Unlock()
	defer func() {
		s.progressMu.Lock()
		delete(s.progress, source.ID)
		s.progressMu.Unlock()
	}()

	// Update status to in_progress
	status := &SyncStatus{
		SourceID:   source.ID,
		LastSyncAt: startTime,
		Status:     "in_progress",
		NextSyncAt: s.getNextSyncTime(startTime),
	}
	s.store.UpdateSyncStatus(ctx, status)

	var totalCount int
	var err error

	switch source.Type {
	case "clawhub":
		totalCount, err = s.fetchClawHubSkillsWithInsert(ctx, source)
	default:
		err = fmt.Errorf("unsupported source type: %s", source.Type)
	}

	duration := timeutil.SinceTime(startTime).Milliseconds()

	if err != nil {
		// Even if there's an error, we may have inserted some skills
		// Update status with partial success if we got some skills
		if totalCount > 0 {
			status.Status = "success"
			status.SkillCount = totalCount
			status.ErrorMessage = fmt.Sprintf("partial sync: %v", err)
		} else {
			status.Status = "failed"
			status.ErrorMessage = err.Error()
		}
		status.SyncDuration = duration
		s.store.UpdateSyncStatus(ctx, status)
		if totalCount > 0 {
			if s.logger != nil {
				s.logger.Warn("partial sync completed",
					"source", source.ID,
					"count", totalCount,
					"error", err,
				)
			}
			return nil // Return nil since we got some data
		}
		return err
	}

	// Update status to success
	status.Status = "success"
	status.SkillCount = totalCount
	status.SyncDuration = duration
	status.ErrorMessage = ""
	s.store.UpdateSyncStatus(ctx, status)

	if s.logger != nil {
		s.logger.Info("synced skills from source",
			"source", source.ID,
			"count", totalCount,
			"duration_ms", duration,
		)
	}

	return nil
}

// hasSyncedToday checks if the source has been successfully synced today.
func (s *SyncService) hasSyncedToday(ctx context.Context, sourceID string) (bool, error) {
	status, err := s.store.GetSyncStatus(ctx, sourceID)
	if err != nil {
		return false, err
	}
	if status == nil {
		return false, nil
	}

	// Check if last successful sync was today
	if status.Status != "success" {
		return false, nil
	}

	// Compare dates (same day in local timezone)
	now := timeutil.NowTime()
	lastSync := status.LastSyncAt

	return isSameDay(now, lastSync), nil
}

// isSameDay checks if two times are on the same calendar day.
func isSameDay(t1, t2 time.Time) bool {
	y1, m1, d1 := t1.Date()
	y2, m2, d2 := t2.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

// getNextSyncTime calculates the next sync time.
func (s *SyncService) getNextSyncTime(from time.Time) time.Time {
	// Next sync is after the configured interval (default: 1 hour)
	return from.Add(s.interval)
}

// ClawHubAPIResponse represents the response from ClawHub API.
type ClawHubAPIResponse struct {
	Items      []ClawHubSkill `json:"items"`
	NextCursor string         `json:"nextCursor,omitempty"`
}

// ClawHubSkill represents a skill from ClawHub API.
type ClawHubSkill struct {
	Slug        string `json:"slug"`
	DisplayName string `json:"displayName"`
	Summary     string `json:"summary"`
	Tags        struct {
		Latest string `json:"latest"`
	} `json:"tags"`
	Stats struct {
		Comments        int `json:"comments"`
		Downloads       int `json:"downloads"`
		InstallsAllTime int `json:"installsAllTime"`
		InstallsCurrent int `json:"installsCurrent"`
		Stars           int `json:"stars"`
		Versions        int `json:"versions"`
	} `json:"stats"`
	CreatedAt     int64 `json:"createdAt"`
	UpdatedAt     int64 `json:"updatedAt"`
	LatestVersion struct {
		Version   string `json:"version"`
		CreatedAt int64  `json:"createdAt"`
		Changelog string `json:"changelog"`
	} `json:"latestVersion"`
}

// fetchClawHubSkillsWithInsert fetches skills from ClawHub and inserts them page by page.
// This ensures that even if sync fails midway, we still have the skills from previous pages.
func (s *SyncService) fetchClawHubSkillsWithInsert(ctx context.Context, source *Source) (int, error) {
	totalCount := 0
	baseURL := source.URL + "/api/v1/skills"
	maxPages := 100          // Safety limit
	pageSize := 24           // ClawHub returns 24 items per page
	maxRetries := 3          // Max retries per page
	maxConsecutiveEmpty := 2 // Stop after 2 consecutive empty pages
	baseBackoff := time.Second
	pageDelay := 200 * time.Millisecond // Rate limiting between pages

	if s.logger != nil {
		s.logger.Info("starting skill sync from ClawHub", "source", source.ID, "url", baseURL)
	}

	consecutiveEmpty := 0

	for page := 1; page <= maxPages; page++ {
		select {
		case <-ctx.Done():
			if totalCount > 0 {
				if s.logger != nil {
					s.logger.Info("sync cancelled, partial results saved", "skills_saved", totalCount)
				}
				return totalCount, nil
			}
			return 0, ctx.Err()
		default:
		}

		// Rate limiting between pages (skip for first page)
		if page > 1 {
			select {
			case <-ctx.Done():
				return totalCount, nil
			case <-time.After(pageDelay):
			}
		}

		// Fetch page with retry
		var apiResp ClawHubAPIResponse
		var lastErr error
		for retry := 0; retry < maxRetries; retry++ {
			if retry > 0 {
				// Exponential backoff: 1s, 2s, 4s
				backoff := baseBackoff * time.Duration(1<<retry)
				select {
				case <-ctx.Done():
					return totalCount, ctx.Err()
				case <-time.After(backoff):
				}
				if s.logger != nil {
					s.logger.Warn("retrying page fetch", "page", page, "retry", retry+1, "backoff", backoff, "error", lastErr)
				}
			}

			apiURL := fmt.Sprintf("%s?page=%d", baseURL, page)
			req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
			if err != nil {
				lastErr = err
				continue
			}

			resp, err := s.httpClient.Do(req)
			if err != nil {
				lastErr = err
				continue
			}

			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				lastErr = fmt.Errorf("API returned status %d", resp.StatusCode)
				// For 429 (rate limit), use longer backoff
				if resp.StatusCode == 429 {
					backoff := baseBackoff * time.Duration(1<<(retry+2)) // 4s, 8s, 16s
					select {
					case <-ctx.Done():
						return totalCount, ctx.Err()
					case <-time.After(backoff):
					}
				}
				continue
			}

			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				lastErr = err
				continue
			}

			if err := json.Unmarshal(body, &apiResp); err != nil {
				lastErr = err
				continue
			}

			lastErr = nil
			break
		}

		// If all retries failed, return what we have or error
		if lastErr != nil {
			if totalCount > 0 {
				if s.logger != nil {
					s.logger.Warn("page fetch failed after retries, partial results saved", "page", page, "error", lastErr, "skills_saved", totalCount)
				}
				return totalCount, lastErr
			}
			return 0, lastErr
		}

		// Handle empty page - don't stop immediately, allow a few consecutive empty pages
		if len(apiResp.Items) == 0 {
			consecutiveEmpty++
			if s.logger != nil {
				s.logger.Warn("empty page received", "page", page, "consecutive_empty", consecutiveEmpty)
			}
			if consecutiveEmpty >= maxConsecutiveEmpty {
				if s.logger != nil {
					s.logger.Info("reached end of data after consecutive empty pages", "page", page, "total_skills", totalCount)
				}
				break
			}
			continue // Try next page
		}

		// Reset consecutive empty counter on successful page
		consecutiveEmpty = 0

		// Convert to local skill format
		now := timeutil.NowTime()
		var pageSkills []*Skill
		for _, item := range apiResp.Items {
			// Extract categories from tags (up to 3 tags as categories)
			category := "skill"
			if item.Tags.Latest != "" {
				tags := strings.Split(item.Tags.Latest, ",")
				var categories []string
				maxCategories := 3
				for i, tag := range tags {
					if i >= maxCategories {
						break
					}
					trimmed := strings.TrimSpace(tag)
					if trimmed != "" {
						categories = append(categories, strings.ToLower(trimmed))
					}
				}
				if len(categories) > 0 {
					category = strings.Join(categories, ",")
				}
			}

			skill := &Skill{
				ID:          item.Slug,
				Name:        item.DisplayName,
				Version:     item.LatestVersion.Version,
				Summary:     item.Summary,
				Category:    category,
				SourceID:    source.ID,
				SourceName:  source.Name,
				Homepage:    fmt.Sprintf("%s/skills/%s", source.URL, item.Slug),
				DownloadURL: fmt.Sprintf("%s/skills/%s", source.URL, item.Slug),
				Stars:       item.Stats.Stars,
				Downloads:   item.Stats.Downloads,
				Reviews:     item.Stats.Comments,
				Versions:    item.Stats.Versions,
				Changelog:   item.LatestVersion.Changelog,
				UpdatedAt:   now,
				SyncedAt:    now,
			}

			// Parse tags from latest version tag
			if item.Tags.Latest != "" {
				skill.Tags = item.Tags.Latest
			}

			// Generate dedup key
			if skill.DedupKey == "" {
				skill.DedupKey = GenerateDedupKey(skill.Name, skill.Author)
			}

			pageSkills = append(pageSkills, skill)
		}

		// Deduplicate within page
		pageSkills = DeduplicateSkills(pageSkills)

		// Insert this page's skills immediately
		if len(pageSkills) > 0 {
			if err := s.store.UpsertSkillBatch(ctx, pageSkills); err != nil {
				if s.logger != nil {
					s.logger.Error("failed to insert page skills", "page", page, "error", err)
				}
				// Continue to next page even if insert fails
			} else {
				totalCount += len(pageSkills)
				// Update in-memory progress
				s.progressMu.Lock()
				if p, ok := s.progress[source.ID]; ok {
					p.CurrentPage = page
					p.SkillsSynced = totalCount
				}
				s.progressMu.Unlock()
				if s.logger != nil {
					s.logger.Debug("inserted page skills", "page", page, "count", len(pageSkills), "total", totalCount)
				}
			}

			// Enqueue for README fetching
			if s.readmeFetcher != nil {
				s.readmeFetcher.EnqueueBatch(pageSkills)
			}
		}

		// Log progress every 5 pages
		if s.logger != nil && page%5 == 0 {
			s.logger.Info("sync progress", "page", page, "skills_saved", totalCount)
		}

		// If we got fewer items than page size, we've reached the last page
		if len(apiResp.Items) < pageSize {
			break
		}
	}

	if s.logger != nil {
		s.logger.Info("fetched and saved skills from ClawHub", "count", totalCount)
	}

	return totalCount, nil
}

// FetchSkillDetail fetches detailed information for a specific skill.
func (s *SyncService) FetchSkillDetail(ctx context.Context, sourceID, skillID string) (*Skill, error) {
	s.mu.RLock()
	source, exists := s.sources[sourceID]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("source not found: %s", sourceID)
	}

	switch source.Type {
	case "clawhub":
		return s.fetchClawHubSkillDetail(ctx, source, skillID)
	default:
		return nil, fmt.Errorf("unsupported source type: %s", source.Type)
	}
}

// ClawHubSkillDetail represents detailed skill info from ClawHub.
type ClawHubSkillDetail struct {
	Slug        string `json:"slug"`
	DisplayName string `json:"displayName"`
	Summary     string `json:"summary"`
	Description string `json:"description"`
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
		Version   string `json:"version"`
		Changelog string `json:"changelog"`
	} `json:"latestVersion"`
}

// fetchClawHubSkillDetail fetches detailed skill info from ClawHub.
func (s *SyncService) fetchClawHubSkillDetail(ctx context.Context, source *Source, skillID string) (*Skill, error) {
	apiURL := fmt.Sprintf("%s/api/v1/skills/%s", source.URL, skillID)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var detail ClawHubSkillDetail
	if err := json.Unmarshal(body, &detail); err != nil {
		return nil, err
	}

	now := timeutil.NowTime()
	skill := &Skill{
		ID:          detail.Slug,
		Name:        detail.DisplayName,
		Version:     detail.LatestVersion.Version,
		Summary:     detail.Summary,
		Description: detail.Description,
		Author:      detail.Author.Name,
		Category:    "skill",
		Tags:        strings.Join(detail.Tags, ","),
		SourceID:    source.ID,
		SourceName:  source.Name,
		Homepage:    fmt.Sprintf("%s/skills/%s", source.URL, detail.Slug),
		DownloadURL: fmt.Sprintf("%s/skills/%s", source.URL, detail.Slug),
		Stars:       detail.Stats.Stars,
		Downloads:   detail.Stats.Downloads,
		Versions:    detail.Stats.Versions,
		Changelog:   detail.LatestVersion.Changelog,
		UpdatedAt:   now,
		SyncedAt:    now,
	}

	return skill, nil
}

// NeedSync checks if a source needs synchronization (not synced successfully today).
func (s *SyncService) NeedSync(ctx context.Context, sourceID string) (bool, error) {
	synced, err := s.hasSyncedToday(ctx, sourceID)
	if err != nil {
		return true, err
	}
	return !synced, nil
}

// GetSyncStatus returns the sync status for a source.
func (s *SyncService) GetSyncStatus(ctx context.Context, sourceID string) (*SyncStatus, error) {
	return s.store.GetSyncStatus(ctx, sourceID)
}

// GetAllSyncStatus returns sync status for all sources.
func (s *SyncService) GetAllSyncStatus(ctx context.Context) ([]*SyncStatus, error) {
	s.mu.RLock()
	sourceIDs := make([]string, 0, len(s.sources))
	for id := range s.sources {
		sourceIDs = append(sourceIDs, id)
	}
	s.mu.RUnlock()

	var statuses []*SyncStatus
	for _, id := range sourceIDs {
		status, err := s.store.GetSyncStatus(ctx, id)
		if err != nil {
			continue
		}
		if status != nil {
			// Attach in-memory progress if syncing
			s.progressMu.RLock()
			if p, ok := s.progress[id]; ok {
				cp := *p
				status.Progress = &cp
			}
			s.progressMu.RUnlock()
			statuses = append(statuses, status)
		}
	}
	return statuses, nil
}
