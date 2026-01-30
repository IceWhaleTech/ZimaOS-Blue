package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/skillstore"
	"github.com/labstack/echo/v4"
)

// SkillSource represents an external skill source
type SkillSource struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	Type        string `json:"type"` // "clawdhub", "github", "custom"
	Description string `json:"description,omitempty"`
	Enabled     bool   `json:"enabled"`
}

// RemoteSkill represents a skill from an external source
type RemoteSkill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description"`
	Author      string   `json:"author,omitempty"`
	Category    string   `json:"category,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	SourceID    string   `json:"source_id"`
	SourceName  string   `json:"source_name"`
	DownloadURL string   `json:"download_url,omitempty"`
	Homepage    string   `json:"homepage,omitempty"`
	Stars       int      `json:"stars,omitempty"`
	Downloads   int      `json:"downloads,omitempty"`
	Installed   bool     `json:"installed"`
}

// SkillHandler handles skill-related HTTP requests
type SkillHandler struct {
	registry      *skill.Registry
	store         *skillstore.Store       // Local database store for skills
	syncService   *skillstore.SyncService // Sync service for periodic updates
	sources       map[string]*SkillSource
	remoteSkills  map[string]*RemoteSkill
	mu            sync.RWMutex
	httpClient    *http.Client
	useMockData   bool // When true, return mock data if API fails; when false, return error
}

// NewSkillHandler creates a new skill handler
func NewSkillHandler(registry *skill.Registry) *SkillHandler {
	h := &SkillHandler{
		registry:     registry,
		sources:      make(map[string]*SkillSource),
		remoteSkills: make(map[string]*RemoteSkill),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		useMockData: false, // Default to not using mock data in production
	}

	// Register default sources
	h.sources["clawhub"] = &SkillSource{
		ID:          "clawhub",
		Name:        "ClawHub",
		URL:         "https://www.clawhub.ai",
		Type:        "clawhub",
		Description: "Official ClawHub skill marketplace",
		Enabled:     true,
	}
	h.sources["moltbot"] = &SkillSource{
		ID:          "moltbot",
		Name:        "Community Extensions",
		URL:         "https://api.github.com/repos/moltbot/moltbot/contents/extensions",
		Type:        "github",
		Description: "Community extensions",
		Enabled:     true,
	}

	return h
}

// SetStore sets the skill store for database persistence
func (h *SkillHandler) SetStore(store *skillstore.Store) {
	h.store = store
}

// SetSyncService sets the sync service for periodic updates
func (h *SkillHandler) SetSyncService(syncService *skillstore.SyncService) {
	h.syncService = syncService
}

// SetUseMockData enables or disables mock data fallback
// When enabled, mock data is returned if the API fails
// This is useful for development and demo purposes
func (h *SkillHandler) SetUseMockData(enabled bool) {
	h.useMockData = enabled
}

// RegisterRoutes registers skill routes
func (h *SkillHandler) RegisterRoutes(g *echo.Group) {
	skills := g.Group("/skills")
	skills.GET("", h.ListSkills)
	skills.GET("/:id", h.GetSkill)
	skills.POST("/:id/enable", h.EnableSkill)
	skills.POST("/:id/disable", h.DisableSkill)

	// Skill store routes
	store := g.Group("/skill-store")
	store.GET("/sources", h.ListSources)
	store.POST("/sources", h.AddSource)
	store.DELETE("/sources/:id", h.RemoveSource)
	store.GET("/browse", h.BrowseSkills)
	store.GET("/search", h.SearchSkills)       // New: Full-text search
	store.GET("/categories", h.GetCategories)  // New: Get all categories
	store.GET("/stats", h.GetStats)            // New: Get statistics
	store.GET("/sync-status", h.GetSyncStatus) // New: Get sync status
	store.POST("/install/:id", h.InstallSkill)
	store.POST("/uninstall/:id", h.UninstallSkill)
	store.POST("/refresh", h.RefreshSources)
	store.POST("/sync", h.TriggerSync) // New: Trigger manual sync
}

// SkillResponse represents a skill in API responses
type SkillResponse struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Author      string            `json:"author,omitempty"`
	Category    string            `json:"category,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Enabled     bool              `json:"enabled"`
	Builtin     bool              `json:"builtin"`
	Inputs      []skill.Parameter `json:"inputs,omitempty"`
	Outputs     []skill.Parameter `json:"outputs,omitempty"`
}

// ListSkills returns all registered skills
func (h *SkillHandler) ListSkills(c echo.Context) error {
	skills := h.registry.List()
	response := make([]SkillResponse, 0, len(skills))

	for _, s := range skills {
		m := s.Manifest
		response = append(response, SkillResponse{
			ID:          m.ID,
			Name:        m.Name,
			Version:     m.Version,
			Description: m.Description,
			Author:      m.Author,
			Category:    m.Category,
			Tags:        m.Tags,
			Enabled:     s.Enabled,
			Builtin:     s.Builtin,
			Inputs:      m.Inputs,
			Outputs:     m.Outputs,
		})
	}

	return c.JSON(http.StatusOK, response)
}

// GetSkill returns a specific skill
func (h *SkillHandler) GetSkill(c echo.Context) error {
	id := c.Param("id")
	info := h.registry.GetInfo(id)
	if info == nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "skill not found",
		})
	}

	m := info.Manifest
	return c.JSON(http.StatusOK, SkillResponse{
		ID:          m.ID,
		Name:        m.Name,
		Version:     m.Version,
		Description: m.Description,
		Author:      m.Author,
		Category:    m.Category,
		Tags:        m.Tags,
		Enabled:     info.Enabled,
		Builtin:     info.Builtin,
		Inputs:      m.Inputs,
		Outputs:     m.Outputs,
	})
}

// EnableSkill enables a skill
func (h *SkillHandler) EnableSkill(c echo.Context) error {
	id := c.Param("id")
	if err := h.registry.Enable(id); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": err.Error(),
		})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "skill enabled",
	})
}

// DisableSkill disables a skill
func (h *SkillHandler) DisableSkill(c echo.Context) error {
	id := c.Param("id")
	if err := h.registry.Disable(id); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": err.Error(),
		})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "skill disabled",
	})
}

// ListSources returns all skill sources
func (h *SkillHandler) ListSources(c echo.Context) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	sources := make([]*SkillSource, 0, len(h.sources))
	for _, s := range h.sources {
		sources = append(sources, s)
	}
	return c.JSON(http.StatusOK, sources)
}

// AddSource adds a new skill source
func (h *SkillHandler) AddSource(c echo.Context) error {
	var source SkillSource
	if err := c.Bind(&source); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
	}

	if source.ID == "" || source.URL == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "id and url are required",
		})
	}

	h.mu.Lock()
	h.sources[source.ID] = &source
	h.mu.Unlock()

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "source added",
	})
}

// RemoveSource removes a skill source
func (h *SkillHandler) RemoveSource(c echo.Context) error {
	id := c.Param("id")

	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.sources[id]; !exists {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "source not found",
		})
	}

	// Don't allow removing default sources
	if id == "clawhub" || id == "moltbot" {
		return c.JSON(http.StatusForbidden, map[string]string{
			"error": "cannot remove default source",
		})
	}

	delete(h.sources, id)
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "source removed",
	})
}

// BrowseSkills returns skills from all enabled sources
func (h *SkillHandler) BrowseSkills(c echo.Context) error {
	// If store is available, use database
	if h.store != nil {
		return h.browseSkillsFromStore(c)
	}

	// Fallback to in-memory storage
	return h.browseSkillsFromMemory(c)
}

// browseSkillsFromStore returns skills from the database store
func (h *SkillHandler) browseSkillsFromStore(c echo.Context) error {
	// Get query parameters
	sourceID := c.QueryParam("source")
	search := c.QueryParam("search")
	pageStr := c.QueryParam("page")
	pageSizeStr := c.QueryParam("page_size")

	page := 1
	pageSize := 24
	if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
		page = p
	}
	if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
		pageSize = ps
	}

	// If search query provided, use full-text search
	if search != "" {
		opts := skillstore.SearchOptions{
			Query:    search,
			Page:     page,
			PageSize: pageSize,
		}
		if sourceID != "" {
			opts.Sources = []string{sourceID}
		}

		result, err := h.store.Search(c.Request().Context(), opts)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("search failed: %v", err),
			})
		}

		// Convert to RemoteSkill format for backward compatibility
		skills := make([]*RemoteSkill, 0, len(result.Skills))
		installedIDs := h.getInstalledSkillIDs()
		for _, s := range result.Skills {
			skills = append(skills, h.skillToRemoteSkill(&s.Skill, installedIDs))
		}

		return c.JSON(http.StatusOK, map[string]interface{}{
			"skills":      skills,
			"total":       result.Total,
			"page":        result.Page,
			"page_size":   result.PageSize,
			"total_pages": result.TotalPages,
		})
	}

	// List by source
	src := sourceID
	if src == "" {
		src = "clawhub" // Default source
	}

	skills, total, err := h.store.ListBySource(c.Request().Context(), src, page, pageSize)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to list skills: %v", err),
		})
	}

	// Convert to RemoteSkill format
	remoteSkills := make([]*RemoteSkill, 0, len(skills))
	installedIDs := h.getInstalledSkillIDs()
	for _, s := range skills {
		remoteSkills = append(remoteSkills, h.skillToRemoteSkill(s, installedIDs))
	}

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))

	return c.JSON(http.StatusOK, map[string]interface{}{
		"skills":      remoteSkills,
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
		"total_pages": totalPages,
	})
}

// browseSkillsFromMemory returns skills from in-memory storage (fallback)
func (h *SkillHandler) browseSkillsFromMemory(c echo.Context) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Get query parameters
	sourceID := c.QueryParam("source")
	category := c.QueryParam("category")
	search := c.QueryParam("search")

	skills := make([]*RemoteSkill, 0)

	// Get installed skill IDs
	installedIDs := h.getInstalledSkillIDs()

	for _, rs := range h.remoteSkills {
		// Filter by source
		if sourceID != "" && rs.SourceID != sourceID {
			continue
		}

		// Filter by category
		if category != "" && rs.Category != category {
			continue
		}

		// Filter by search
		if search != "" {
			// Simple search in name and description
			if !containsIgnoreCase(rs.Name, search) && !containsIgnoreCase(rs.Description, search) {
				continue
			}
		}

		// Mark as installed if in registry
		rs.Installed = installedIDs[rs.ID]
		skills = append(skills, rs)
	}

	return c.JSON(http.StatusOK, skills)
}

// getInstalledSkillIDs returns a map of installed skill IDs
func (h *SkillHandler) getInstalledSkillIDs() map[string]bool {
	installedIDs := make(map[string]bool)
	for _, s := range h.registry.List() {
		installedIDs[s.Manifest.ID] = true
	}
	return installedIDs
}

// skillToRemoteSkill converts a skillstore.Skill to RemoteSkill
func (h *SkillHandler) skillToRemoteSkill(s *skillstore.Skill, installedIDs map[string]bool) *RemoteSkill {
	var tags []string
	if s.Tags != "" {
		tags = strings.Split(s.Tags, ",")
	}
	return &RemoteSkill{
		ID:          s.ID,
		Name:        s.Name,
		Version:     s.Version,
		Description: s.Summary,
		Author:      s.Author,
		Category:    s.Category,
		Tags:        tags,
		SourceID:    s.SourceID,
		SourceName:  s.SourceName,
		DownloadURL: s.DownloadURL,
		Homepage:    s.Homepage,
		Stars:       s.Stars,
		Downloads:   s.Downloads,
		Installed:   installedIDs[s.ID],
	}
}

// InstallSkill installs a skill from a remote source
func (h *SkillHandler) InstallSkill(c echo.Context) error {
	id := c.Param("id")
	ctx := c.Request().Context()

	// Check if already installed
	if h.registry.Get(id) != nil {
		return c.JSON(http.StatusConflict, map[string]string{
			"error": "skill already installed",
		})
	}

	var rs *RemoteSkill

	// Try to get skill from database first
	if h.store != nil {
		skill, err := h.store.GetSkill(ctx, id)
		if err == nil && skill != nil {
			installedIDs := h.getInstalledSkillIDs()
			rs = h.skillToRemoteSkill(skill, installedIDs)
		}
	}

	// Fallback to in-memory storage
	if rs == nil {
		h.mu.RLock()
		memSkill, exists := h.remoteSkills[id]
		h.mu.RUnlock()

		if !exists {
			return c.JSON(http.StatusNotFound, map[string]string{
				"error": "skill not found in store",
			})
		}
		rs = memSkill
	}

	// Download skill manifest from source
	manifest, err := h.downloadSkillManifest(ctx, rs)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to download skill: %v", err),
		})
	}

	// Create and register the skill
	remoteSkill := NewRemoteSkillAdapter(manifest)
	if err := h.registry.Register(remoteSkill, false); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to register skill: %v", err),
		})
	}

	// Update installed status in database
	if h.store != nil {
		h.store.SetInstalled(ctx, id, true)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("skill %s installed successfully", rs.Name),
		"skill":   rs,
	})
}

// UninstallSkill uninstalls a skill
func (h *SkillHandler) UninstallSkill(c echo.Context) error {
	id := c.Param("id")

	info := h.registry.GetInfo(id)
	if info == nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "skill not found",
		})
	}

	// Don't allow uninstalling builtin skills
	if info.Builtin {
		return c.JSON(http.StatusForbidden, map[string]string{
			"error": "cannot uninstall builtin skill",
		})
	}

	if err := h.registry.Unregister(id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "skill uninstalled",
	})
}

// RefreshSources refreshes skills from all enabled sources
func (h *SkillHandler) RefreshSources(c echo.Context) error {
	// If sync service is available, use it for database sync
	if h.syncService != nil {
		return h.refreshSourcesWithSync(c)
	}

	// Fallback to in-memory refresh
	return h.refreshSourcesInMemory(c)
}

// refreshSourcesWithSync uses the sync service to refresh skills into database
func (h *SkillHandler) refreshSourcesWithSync(c echo.Context) error {
	ctx := c.Request().Context()

	// Force sync (ignore today's sync check for manual refresh)
	forceSync := c.QueryParam("force") == "true"

	// Get current sync status before starting
	statuses, _ := h.syncService.GetAllSyncStatus(ctx)
	statusMap := make(map[string]*skillstore.SyncStatus)
	for _, s := range statuses {
		statusMap[s.SourceID] = s
	}

	// Check if any source needs sync
	sources := h.syncService.GetSources()
	needsSync := false
	for _, src := range sources {
		if !src.Enabled {
			continue
		}
		if forceSync {
			needsSync = true
			break
		}
		if need, _ := h.syncService.NeedSync(ctx, src.ID); need {
			needsSync = true
			break
		}
	}

	if !needsSync {
		// Already synced today, return current status
		stats, _ := h.store.GetStats(ctx)
		totalSkills := int64(0)
		if v, ok := stats["total_skills"].(int64); ok {
			totalSkills = v
		}

		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":      true,
			"skills_count": totalSkills,
			"message":      "already synced today",
			"sync_status":  statuses,
		})
	}

	// Start sync in background and return immediately with status
	go func() {
		bgCtx := context.Background()
		if forceSync {
			h.syncService.ForceSyncAll(bgCtx)
		} else {
			h.syncService.SyncAll(bgCtx)
		}
	}()

	// Return current status - client can poll for updates
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":     true,
		"message":     "sync started",
		"sync_status": statuses,
		"syncing":     true,
	})
}

// refreshSourcesInMemory refreshes skills into in-memory storage (fallback)
func (h *SkillHandler) refreshSourcesInMemory(c echo.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Clear existing remote skills
	h.remoteSkills = make(map[string]*RemoteSkill)

	var wg sync.WaitGroup
	var mu sync.Mutex
	errors := make([]string, 0)

	for _, source := range h.sources {
		if !source.Enabled {
			continue
		}

		wg.Add(1)
		go func(src *SkillSource) {
			defer wg.Done()

			skills, err := h.fetchSkillsFromSource(c.Request().Context(), src)
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("%s: %v", src.Name, err))
				mu.Unlock()
				return
			}

			mu.Lock()
			for _, s := range skills {
				h.remoteSkills[s.ID] = s
			}
			mu.Unlock()
		}(source)
	}

	wg.Wait()

	response := map[string]interface{}{
		"success":      len(errors) == 0,
		"skills_count": len(h.remoteSkills),
	}

	if len(errors) > 0 {
		response["errors"] = errors
	}

	return c.JSON(http.StatusOK, response)
}

// fetchSkillsFromSource fetches skills from a specific source
func (h *SkillHandler) fetchSkillsFromSource(ctx context.Context, source *SkillSource) ([]*RemoteSkill, error) {
	switch source.Type {
	case "clawhub":
		return h.fetchFromClawHub(ctx, source)
	case "github":
		return h.fetchFromGitHub(ctx, source)
	default:
		return h.fetchFromCustom(ctx, source)
	}
}

// ClawHubAPIResponse represents the response from ClawHub API
type ClawHubAPIResponse struct {
	Items      []ClawHubSkill `json:"items"`
	NextCursor string         `json:"nextCursor,omitempty"`
}

// ClawHubSkill represents a skill from ClawHub API
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

// fetchFromClawHub fetches skills from ClawHub with pagination support
func (h *SkillHandler) fetchFromClawHub(ctx context.Context, source *SkillSource) ([]*RemoteSkill, error) {
	var allSkills []*RemoteSkill
	baseURL := source.URL + "/api/v1/skills"
	cursor := ""
	maxPages := 100 // Limit to prevent infinite loops (supports ~2400 skills at 24 per page)

	for page := 0; page < maxPages; page++ {
		// Build URL with cursor if available
		apiURL := baseURL
		if cursor != "" {
			apiURL = fmt.Sprintf("%s?cursor=%s", baseURL, cursor)
		}

		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			if h.useMockData && len(allSkills) == 0 {
				return h.getMockClawHubSkills(source), nil
			}
			if len(allSkills) > 0 {
				// Return what we have so far
				return allSkills, nil
			}
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := h.httpClient.Do(req)
		if err != nil {
			if h.useMockData && len(allSkills) == 0 {
				return h.getMockClawHubSkills(source), nil
			}
			if len(allSkills) > 0 {
				return allSkills, nil
			}
			return nil, fmt.Errorf("failed to fetch skills from %s: %w", source.Name, err)
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			if h.useMockData && len(allSkills) == 0 {
				return h.getMockClawHubSkills(source), nil
			}
			if len(allSkills) > 0 {
				return allSkills, nil
			}
			return nil, fmt.Errorf("API returned status %d from %s", resp.StatusCode, source.Name)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			if len(allSkills) > 0 {
				return allSkills, nil
			}
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		// Parse ClawHub API response
		var apiResp ClawHubAPIResponse
		if err := json.Unmarshal(body, &apiResp); err != nil {
			if h.useMockData && len(allSkills) == 0 {
				return h.getMockClawHubSkills(source), nil
			}
			if len(allSkills) > 0 {
				return allSkills, nil
			}
			return nil, fmt.Errorf("failed to parse skills response: %w", err)
		}

		// Convert ClawHub skills to RemoteSkill format
		for _, item := range apiResp.Items {
			allSkills = append(allSkills, h.convertClawHubSkill(item, source))
		}

		// Check if there are more pages
		if apiResp.NextCursor == "" {
			break
		}
		cursor = apiResp.NextCursor
	}

	return allSkills, nil
}

// convertClawHubSkill converts a ClawHubSkill to RemoteSkill
func (h *SkillHandler) convertClawHubSkill(item ClawHubSkill, source *SkillSource) *RemoteSkill {
	return &RemoteSkill{
		ID:          item.Slug,
		Name:        item.DisplayName,
		Version:     item.LatestVersion.Version,
		Description: item.Summary,
		Author:      "", // ClawHub API doesn't provide author in list response
		Category:    "skill",
		Tags:        []string{},
		SourceID:    source.ID,
		SourceName:  source.Name,
		DownloadURL: fmt.Sprintf("%s/skills/%s", source.URL, item.Slug),
		Homepage:    fmt.Sprintf("%s/skills/%s", source.URL, item.Slug),
		Stars:       item.Stats.Stars,
		Downloads:   item.Stats.Downloads,
	}
}

// getMockClawHubSkills returns mock ClawHub skills for demo
func (h *SkillHandler) getMockClawHubSkills(source *SkillSource) []*RemoteSkill {
	return []*RemoteSkill{
		{
			ID:          "clawhub-smart-home",
			Name:        "Smart Home Controller",
			Version:     "1.2.0",
			Description: "Advanced smart home automation with scene management",
			Author:      "ClawHub Team",
			Category:    "integration",
			Tags:        []string{"smart-home", "automation", "iot"},
			SourceID:    source.ID,
			SourceName:  source.Name,
			Homepage:    "https://www.clawhub.ai/skills/smart-home",
			Stars:       256,
			Downloads:   1520,
		},
		{
			ID:          "clawhub-ai-assistant",
			Name:        "AI Writing Assistant",
			Version:     "2.0.1",
			Description: "AI-powered writing assistant with grammar and style suggestions",
			Author:      "ClawHub Team",
			Category:    "productivity",
			Tags:        []string{"ai", "writing", "assistant"},
			SourceID:    source.ID,
			SourceName:  source.Name,
			Homepage:    "https://www.clawhub.ai/skills/ai-assistant",
			Stars:       512,
			Downloads:   3200,
		},
		{
			ID:          "clawhub-code-review",
			Name:        "Code Review Helper",
			Version:     "1.0.0",
			Description: "Automated code review with best practices suggestions",
			Author:      "ClawHub Team",
			Category:    "development",
			Tags:        []string{"code", "review", "development"},
			SourceID:    source.ID,
			SourceName:  source.Name,
			Homepage:    "https://www.clawhub.ai/skills/code-review",
			Stars:       128,
			Downloads:   890,
		},
		{
			ID:          "clawhub-data-analyzer",
			Name:        "Data Analyzer",
			Version:     "1.5.0",
			Description: "Analyze and visualize data from various sources",
			Author:      "ClawHub Team",
			Category:    "analytics",
			Tags:        []string{"data", "analytics", "visualization"},
			SourceID:    source.ID,
			SourceName:  source.Name,
			Homepage:    "https://www.clawhub.ai/skills/data-analyzer",
			Stars:       320,
			Downloads:   2100,
		},
	}
}

// fetchFromGitHub fetches skills from a GitHub repository
func (h *SkillHandler) fetchFromGitHub(ctx context.Context, source *SkillSource) ([]*RemoteSkill, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", source.URL, nil)
	if err != nil {
		if h.useMockData {
			return h.getMockGitHubSkills(source), nil
		}
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		if h.useMockData {
			return h.getMockGitHubSkills(source), nil
		}
		return nil, fmt.Errorf("failed to fetch skills from %s: %w", source.Name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if h.useMockData {
			return h.getMockGitHubSkills(source), nil
		}
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var contents []struct {
		Name string `json:"name"`
		Type string `json:"type"`
		Path string `json:"path"`
		URL  string `json:"html_url"`
	}

	if err := json.Unmarshal(body, &contents); err != nil {
		if h.useMockData {
			return h.getMockGitHubSkills(source), nil
		}
		return nil, fmt.Errorf("failed to parse GitHub response: %w", err)
	}

	skills := make([]*RemoteSkill, 0)
	for _, item := range contents {
		if item.Type == "dir" {
			skills = append(skills, &RemoteSkill{
				ID:          fmt.Sprintf("moltbot-%s", item.Name),
				Name:        item.Name,
				Version:     "1.0.0",
				Description: fmt.Sprintf("MoltBot extension: %s", item.Name),
				Author:      "MoltBot Community",
				Category:    "extension",
				Tags:        []string{"moltbot", "extension"},
				SourceID:    source.ID,
				SourceName:  source.Name,
				Homepage:    item.URL,
			})
		}
	}

	return skills, nil
}

// getMockGitHubSkills returns mock GitHub skills for demo
func (h *SkillHandler) getMockGitHubSkills(source *SkillSource) []*RemoteSkill {
	return []*RemoteSkill{
		{
			ID:          "moltbot-weather",
			Name:        "Weather Extension",
			Version:     "1.0.0",
			Description: "Get weather information from multiple providers",
			Author:      "MoltBot Community",
			Category:    "extension",
			Tags:        []string{"moltbot", "weather", "extension"},
			SourceID:    source.ID,
			SourceName:  source.Name,
			Homepage:    "https://github.com/moltbot/moltbot/tree/main/extensions/weather",
		},
		{
			ID:          "moltbot-reminder",
			Name:        "Reminder Extension",
			Version:     "1.0.0",
			Description: "Set and manage reminders with natural language",
			Author:      "MoltBot Community",
			Category:    "extension",
			Tags:        []string{"moltbot", "reminder", "extension"},
			SourceID:    source.ID,
			SourceName:  source.Name,
			Homepage:    "https://github.com/moltbot/moltbot/tree/main/extensions/reminder",
		},
		{
			ID:          "moltbot-translator",
			Name:        "Translator Extension",
			Version:     "1.0.0",
			Description: "Translate text between multiple languages",
			Author:      "MoltBot Community",
			Category:    "extension",
			Tags:        []string{"moltbot", "translator", "extension"},
			SourceID:    source.ID,
			SourceName:  source.Name,
			Homepage:    "https://github.com/moltbot/moltbot/tree/main/extensions/translator",
		},
	}
}

// fetchFromCustom fetches skills from a custom source
func (h *SkillHandler) fetchFromCustom(ctx context.Context, source *SkillSource) ([]*RemoteSkill, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", source.URL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var skills []*RemoteSkill
	if err := json.Unmarshal(body, &skills); err != nil {
		return nil, err
	}

	// Set source info
	for _, s := range skills {
		s.SourceID = source.ID
		s.SourceName = source.Name
	}

	return skills, nil
}

// containsIgnoreCase checks if s contains substr (case-insensitive)
func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(substr) == 0 ||
		(len(s) > 0 && containsIgnoreCaseImpl(s, substr)))
}

func containsIgnoreCaseImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalFoldAt(s, i, substr) {
			return true
		}
	}
	return false
}

func equalFoldAt(s string, start int, substr string) bool {
	for j := 0; j < len(substr); j++ {
		c1 := s[start+j]
		c2 := substr[j]
		if c1 != c2 {
			// Simple ASCII case-insensitive comparison
			if c1 >= 'A' && c1 <= 'Z' {
				c1 += 'a' - 'A'
			}
			if c2 >= 'A' && c2 <= 'Z' {
				c2 += 'a' - 'A'
			}
			if c1 != c2 {
				return false
			}
		}
	}
	return true
}

// downloadSkillManifest downloads the skill manifest from the remote source
func (h *SkillHandler) downloadSkillManifest(ctx context.Context, rs *RemoteSkill) (*skill.Manifest, error) {
	// If download URL is available, try to fetch the manifest
	if rs.DownloadURL != "" {
		req, err := http.NewRequestWithContext(ctx, "GET", rs.DownloadURL, nil)
		if err != nil {
			return nil, err
		}

		resp, err := h.httpClient.Do(req)
		if err != nil {
			// Fall back to creating manifest from remote skill info
			return h.createManifestFromRemoteSkill(rs), nil
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return h.createManifestFromRemoteSkill(rs), nil
			}

			var manifest skill.Manifest
			if err := json.Unmarshal(body, &manifest); err != nil {
				return h.createManifestFromRemoteSkill(rs), nil
			}
			return &manifest, nil
		}
	}

	// Create manifest from remote skill info
	return h.createManifestFromRemoteSkill(rs), nil
}

// createManifestFromRemoteSkill creates a skill manifest from remote skill info
func (h *SkillHandler) createManifestFromRemoteSkill(rs *RemoteSkill) *skill.Manifest {
	return &skill.Manifest{
		ID:          rs.ID,
		Name:        rs.Name,
		Version:     rs.Version,
		Description: rs.Description,
		Author:      rs.Author,
		Category:    rs.Category,
		Tags:        rs.Tags,
		Metadata: map[string]string{
			"source_id":   rs.SourceID,
			"source_name": rs.SourceName,
			"homepage":    rs.Homepage,
		},
	}
}

// RemoteSkillAdapter adapts a remote skill manifest to the Skill interface
type RemoteSkillAdapter struct {
	manifest *skill.Manifest
}

// NewRemoteSkillAdapter creates a new remote skill adapter
func NewRemoteSkillAdapter(manifest *skill.Manifest) *RemoteSkillAdapter {
	return &RemoteSkillAdapter{manifest: manifest}
}

// Manifest returns the skill manifest
func (r *RemoteSkillAdapter) Manifest() *skill.Manifest {
	return r.manifest
}

// Validate validates the input parameters
func (r *RemoteSkillAdapter) Validate(input map[string]any) error {
	// Remote skills have no validation by default
	return nil
}

// Execute executes the skill
func (r *RemoteSkillAdapter) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	// Remote skills are placeholders - actual execution depends on skill type
	return skill.NewResult(map[string]any{
		"message": fmt.Sprintf("Skill %s executed", r.manifest.Name),
		"input":   input,
	}), nil
}

// SearchSkills performs full-text search on skills in the local database
func (h *SkillHandler) SearchSkills(c echo.Context) error {
	if h.store == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "skill store not initialized",
		})
	}

	// Parse query parameters
	opts := skillstore.SearchOptions{
		Query:     c.QueryParam("q"),
		SortBy:    c.QueryParam("sort_by"),
		SortOrder: c.QueryParam("sort_order"),
	}

	// Parse categories
	if cats := c.QueryParam("categories"); cats != "" {
		opts.Categories = strings.Split(cats, ",")
	}

	// Parse sources
	if sources := c.QueryParam("sources"); sources != "" {
		opts.Sources = strings.Split(sources, ",")
	}

	// Parse min_stars
	if minStars := c.QueryParam("min_stars"); minStars != "" {
		if v, err := strconv.Atoi(minStars); err == nil {
			opts.MinStars = v
		}
	}

	// Parse pagination
	if page := c.QueryParam("page"); page != "" {
		if v, err := strconv.Atoi(page); err == nil {
			opts.Page = v
		}
	}
	if pageSize := c.QueryParam("page_size"); pageSize != "" {
		if v, err := strconv.Atoi(pageSize); err == nil {
			opts.PageSize = v
		}
	}

	// Set defaults
	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.PageSize < 1 || opts.PageSize > 100 {
		opts.PageSize = 24
	}

	result, err := h.store.Search(c.Request().Context(), opts)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("search failed: %v", err),
		})
	}

	return c.JSON(http.StatusOK, result)
}

// GetCategories returns all unique skill categories
func (h *SkillHandler) GetCategories(c echo.Context) error {
	if h.store == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "skill store not initialized",
		})
	}

	categories, err := h.store.GetCategories(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to get categories: %v", err),
		})
	}

	return c.JSON(http.StatusOK, categories)
}

// GetStats returns skill statistics
func (h *SkillHandler) GetStats(c echo.Context) error {
	if h.store == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "skill store not initialized",
		})
	}

	stats, err := h.store.GetStats(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to get stats: %v", err),
		})
	}

	return c.JSON(http.StatusOK, stats)
}

// GetSyncStatus returns the synchronization status for all sources
func (h *SkillHandler) GetSyncStatus(c echo.Context) error {
	if h.syncService == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "sync service not initialized",
		})
	}

	statuses, err := h.syncService.GetAllSyncStatus(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to get sync status: %v", err),
		})
	}

	return c.JSON(http.StatusOK, statuses)
}

// TriggerSync triggers a manual synchronization of all sources
func (h *SkillHandler) TriggerSync(c echo.Context) error {
	if h.syncService == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "sync service not initialized",
		})
	}

	// Get optional source ID parameter
	sourceID := c.QueryParam("source")

	if sourceID != "" {
		// Sync specific source
		if err := h.syncService.SyncSource(c.Request().Context(), sourceID); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("sync failed: %v", err),
			})
		}
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success": true,
			"message": fmt.Sprintf("sync triggered for source: %s", sourceID),
		})
	}

	// Sync all sources
	h.syncService.SyncAll(c.Request().Context())

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "sync triggered for all sources",
	})
}
