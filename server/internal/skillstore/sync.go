package skillstore

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// SyncService handles periodic synchronization of skills from remote sources.
type SyncService struct {
	store      *Store
	httpClient *http.Client
	sources    map[string]*Source
	mu         sync.RWMutex
	stopCh     chan struct{}
	wg         sync.WaitGroup
	interval   time.Duration
	logger     *slog.Logger
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
	Interval time.Duration // Sync interval (default: 24 hours)
	Timeout  time.Duration // HTTP timeout (default: 30 seconds)
}

// DefaultSyncServiceConfig returns default configuration.
func DefaultSyncServiceConfig() SyncServiceConfig {
	return SyncServiceConfig{
		Interval: 24 * time.Hour,
		Timeout:  30 * time.Second,
	}
}

// NewSyncService creates a new sync service.
func NewSyncService(store *Store, config SyncServiceConfig, log *slog.Logger) *SyncService {
	if config.Interval == 0 {
		config.Interval = 24 * time.Hour
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
	}

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

// Start starts the periodic sync service.
func (s *SyncService) Start(ctx context.Context) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.run(ctx)
	}()
}

// Stop stops the sync service.
func (s *SyncService) Stop() {
	close(s.stopCh)
	s.wg.Wait()
}

// run is the main sync loop.
func (s *SyncService) run(ctx context.Context) {
	// Initial sync on startup
	s.SyncAll(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.SyncAll(ctx)
		}
	}
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

	// Check if already synced successfully today
	if synced, _ := s.hasSyncedToday(ctx, sourceID); synced {
		if s.logger != nil {
			s.logger.Info("skipping sync, already synced today", "source", sourceID)
		}
		return nil
	}

	startTime := time.Now()

	// Update status to in_progress
	status := &SyncStatus{
		SourceID:   sourceID,
		LastSyncAt: startTime,
		Status:     "in_progress",
		NextSyncAt: s.getNextSyncTime(startTime),
	}
	s.store.UpdateSyncStatus(ctx, status)

	var skills []*Skill
	var err error

	switch source.Type {
	case "clawhub":
		skills, err = s.fetchClawHubSkills(ctx, source)
	default:
		err = fmt.Errorf("unsupported source type: %s", source.Type)
	}

	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		status.Status = "failed"
		status.ErrorMessage = err.Error()
		status.SyncDuration = duration
		s.store.UpdateSyncStatus(ctx, status)
		return err
	}

	// Save skills to database
	if err := s.store.UpsertSkillBatch(ctx, skills); err != nil {
		status.Status = "failed"
		status.ErrorMessage = err.Error()
		status.SyncDuration = duration
		s.store.UpdateSyncStatus(ctx, status)
		return err
	}

	// Update status to success
	status.Status = "success"
	status.SkillCount = len(skills)
	status.SyncDuration = duration
	status.ErrorMessage = ""
	s.store.UpdateSyncStatus(ctx, status)

	if s.logger != nil {
		s.logger.Info("synced skills from source",
			"source", sourceID,
			"count", len(skills),
			"duration_ms", duration,
		)
	}

	return nil
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
	startTime := time.Now()

	// Update status to in_progress
	status := &SyncStatus{
		SourceID:   source.ID,
		LastSyncAt: startTime,
		Status:     "in_progress",
		NextSyncAt: s.getNextSyncTime(startTime),
	}
	s.store.UpdateSyncStatus(ctx, status)

	var skills []*Skill
	var err error

	switch source.Type {
	case "clawhub":
		skills, err = s.fetchClawHubSkills(ctx, source)
	default:
		err = fmt.Errorf("unsupported source type: %s", source.Type)
	}

	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		status.Status = "failed"
		status.ErrorMessage = err.Error()
		status.SyncDuration = duration
		s.store.UpdateSyncStatus(ctx, status)
		return err
	}

	// Save skills to database
	if err := s.store.UpsertSkillBatch(ctx, skills); err != nil {
		status.Status = "failed"
		status.ErrorMessage = err.Error()
		status.SyncDuration = duration
		s.store.UpdateSyncStatus(ctx, status)
		return err
	}

	// Update status to success
	status.Status = "success"
	status.SkillCount = len(skills)
	status.SyncDuration = duration
	status.ErrorMessage = ""
	s.store.UpdateSyncStatus(ctx, status)

	if s.logger != nil {
		s.logger.Info("synced skills from source",
			"source", source.ID,
			"count", len(skills),
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
	now := time.Now()
	lastSync := status.LastSyncAt

	return isSameDay(now, lastSync), nil
}

// isSameDay checks if two times are on the same calendar day.
func isSameDay(t1, t2 time.Time) bool {
	y1, m1, d1 := t1.Date()
	y2, m2, d2 := t2.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

// getNextSyncTime calculates the next sync time (tomorrow at the same time).
func (s *SyncService) getNextSyncTime(from time.Time) time.Time {
	// Next sync is tomorrow at the same time
	return from.Add(24 * time.Hour)
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

// fetchClawHubSkills fetches all skills from ClawHub with pagination.
func (s *SyncService) fetchClawHubSkills(ctx context.Context, source *Source) ([]*Skill, error) {
	var allSkills []*Skill
	baseURL := source.URL + "/api/v1/skills"
	cursor := ""
	maxPages := 100 // Safety limit

	for page := 0; page < maxPages; page++ {
		select {
		case <-ctx.Done():
			return allSkills, ctx.Err()
		default:
		}

		// Build URL with cursor
		apiURL := baseURL
		if cursor != "" {
			apiURL = fmt.Sprintf("%s?cursor=%s", baseURL, cursor)
		}

		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			if len(allSkills) > 0 {
				return allSkills, nil // Return what we have
			}
			return nil, err
		}

		resp, err := s.httpClient.Do(req)
		if err != nil {
			if len(allSkills) > 0 {
				return allSkills, nil
			}
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			if len(allSkills) > 0 {
				return allSkills, nil
			}
			return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			if len(allSkills) > 0 {
				return allSkills, nil
			}
			return nil, err
		}

		var apiResp ClawHubAPIResponse
		if err := json.Unmarshal(body, &apiResp); err != nil {
			if len(allSkills) > 0 {
				return allSkills, nil
			}
			return nil, err
		}

		// Convert to local skill format
		now := time.Now()
		for _, item := range apiResp.Items {
			skill := &Skill{
				ID:          item.Slug,
				Name:        item.DisplayName,
				Version:     item.LatestVersion.Version,
				Summary:     item.Summary,
				Category:    "skill",
				SourceID:    source.ID,
				SourceName:  source.Name,
				Homepage:    fmt.Sprintf("%s/skills/%s", source.URL, item.Slug),
				DownloadURL: fmt.Sprintf("%s/skills/%s", source.URL, item.Slug),
				Stars:       item.Stats.Stars,
				Downloads:   item.Stats.Downloads,
				Versions:    item.Stats.Versions,
				Changelog:   item.LatestVersion.Changelog,
				UpdatedAt:   now,
				SyncedAt:    now,
			}

			// Parse tags from latest version tag
			if item.Tags.Latest != "" {
				skill.Tags = item.Tags.Latest
			}

			allSkills = append(allSkills, skill)
		}

		// Check for more pages
		if apiResp.NextCursor == "" {
			break
		}
		cursor = apiResp.NextCursor
	}

	return allSkills, nil
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
	Tags []string `json:"tags"`
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

	now := time.Now()
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
			statuses = append(statuses, status)
		}
	}
	return statuses, nil
}
