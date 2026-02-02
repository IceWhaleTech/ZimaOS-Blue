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

	"github.com/labstack/echo/v4"
	"golang.org/x/sync/singleflight"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/cache"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/skillstore"
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
	registry            *skill.Registry
	store               *skillstore.Store                // Local database store for skills
	syncService         *skillstore.SyncService          // Sync service for periodic updates
	featuredLoader      *skillstore.FeaturedSkillsLoader // Featured skills fallback
	localScanner        *skillstore.LocalSkillScanner    // Local skill discovery
	sources             map[string]*SkillSource
	remoteSkills        map[string]*RemoteSkill
	mu                  sync.RWMutex
	httpClient          *http.Client
	useMockData         bool // When true, return mock data if API fails; when false, return error
	useFeaturedFallback bool // When true, use featured skills as fallback on API failure

	// singleflight for deduplicating concurrent requests
	sfGroup singleflight.Group

	// cache for frequently accessed data
	browseCache    *cache.GenericCache[string]
	statsCache     *cache.GenericCache[string]
	categoriesCache *cache.GenericCache[string]
}

// NewSkillHandler creates a new skill handler
func NewSkillHandler(registry *skill.Registry) *SkillHandler {
	h := &SkillHandler{
		registry:     registry,
		sources:      make(map[string]*SkillSource),
		remoteSkills: make(map[string]*RemoteSkill),
		httpClient: &http.Client{
			Timeout: 30 * time.Second, // Per-request timeout
		},
		useMockData:         false, // Default to not using mock data in production
		useFeaturedFallback: true,  // Default to using featured skills as fallback
		browseCache: cache.NewGenericCacheWithStats(cache.Config{
			MaxSize:    200,
			DefaultTTL: 60 * time.Second, // Skills list changes infrequently
		}, "skill_browse"),
		statsCache: cache.NewGenericCacheWithStats(cache.Config{
			MaxSize:    10,
			DefaultTTL: 30 * time.Second,
		}, "skill_stats"),
		categoriesCache: cache.NewGenericCacheWithStats(cache.Config{
			MaxSize:    10,
			DefaultTTL: 60 * time.Second,
		}, "skill_categories"),
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
	// Note: moltbot extensions source removed - extensions are now native

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

// SetFeaturedLoader sets the featured skills loader for fallback
func (h *SkillHandler) SetFeaturedLoader(loader *skillstore.FeaturedSkillsLoader) {
	h.featuredLoader = loader
}

// SetLocalScanner sets the local skill scanner
func (h *SkillHandler) SetLocalScanner(scanner *skillstore.LocalSkillScanner) {
	h.localScanner = scanner
}

// SetUseMockData enables or disables mock data fallback
// When enabled, mock data is returned if the API fails
// This is useful for development and demo purposes
func (h *SkillHandler) SetUseMockData(enabled bool) {
	h.useMockData = enabled
}

// SetUseFeaturedFallback enables or disables featured skills fallback
// When enabled, featured skills are returned if the API fails
func (h *SkillHandler) SetUseFeaturedFallback(enabled bool) {
	h.useFeaturedFallback = enabled
}

// RegisterRoutes registers skill routes
func (h *SkillHandler) RegisterRoutes(g *echo.Group) {
	skills := g.Group("/skills")
	skills.GET("", h.ListSkills)
	skills.GET("/:id", h.GetSkill)
	skills.GET("/:id/content", h.GetSkillContent) // Get skill content (SKILL.md)
	skills.POST("/:id/enable", h.EnableSkill)
	skills.POST("/:id/disable", h.DisableSkill)
	skills.GET("/local", h.ListLocalSkills)       // New: List local skills
	skills.POST("/local/scan", h.ScanLocalSkills) // New: Scan local skills
	skills.GET("/verify/:id", h.VerifySkill)      // New: Verify skill visibility
	skills.POST("/upload", h.UploadSkill)         // Upload skill package

	// Skill store routes
	store := g.Group("/skill-store")
	store.GET("/sources", h.ListSources)
	store.POST("/sources", h.AddSource)
	store.DELETE("/sources/:id", h.RemoveSource)
	store.GET("/browse", h.BrowseSkills)
	store.GET("/featured", h.GetFeaturedSkills) // New: Get featured skills
	store.GET("/search", h.SearchSkills)        // Full-text search
	store.GET("/categories", h.GetCategories)   // Get all categories
	store.GET("/popular", h.GetPopularSkills)   // Get popular skills by downloads
	store.GET("/recent", h.GetRecentSkills)     // Get recently updated skills
	store.GET("/stats", h.GetStats)             // Get statistics
	store.GET("/sync-status", h.GetSyncStatus)  // Get sync status
	store.POST("/install/:id", h.InstallSkill)
	store.POST("/install-url", h.InstallFromURL) // New: Install from URL
	store.POST("/uninstall/:id", h.UninstallSkill)
	store.POST("/refresh", h.RefreshSources)
	store.POST("/sync", h.TriggerSync) // Trigger manual sync
}

// SkillResponse represents a skill in API responses
type SkillResponse struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Author      string            `json:"author,omitempty"`
	Category    string            `json:"category,omitempty"`
	Icon        string            `json:"icon,omitempty"`
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
			Icon:        m.Icon,
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
		Icon:        m.Icon,
		Tags:        m.Tags,
		Enabled:     info.Enabled,
		Builtin:     info.Builtin,
		Inputs:      m.Inputs,
		Outputs:     m.Outputs,
	})
}

// GetSkillContent returns the content (SKILL.md/readme) of a skill
func (h *SkillHandler) GetSkillContent(c echo.Context) error {
	id := c.Param("id")
	ctx := c.Request().Context()

	// First check if skill exists in registry
	info := h.registry.GetInfo(id)
	if info == nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "skill not found",
		})
	}

	// Try to get content from database store
	if h.store != nil {
		skill, err := h.store.GetSkill(ctx, id)
		if err == nil && skill != nil && skill.Readme != "" {
			return c.JSON(http.StatusOK, map[string]interface{}{
				"id":      id,
				"name":    info.Manifest.Name,
				"content": skill.Readme,
				"source":  "database",
			})
		}
	}

	// For builtin skills, return the description as content
	if info.Builtin {
		m := info.Manifest
		content := fmt.Sprintf("# %s\n\n%s\n\n", m.Name, m.Description)
		if m.Author != "" {
			content += fmt.Sprintf("**Author:** %s\n\n", m.Author)
		}
		if m.Version != "" {
			content += fmt.Sprintf("**Version:** %s\n\n", m.Version)
		}
		if m.Category != "" {
			content += fmt.Sprintf("**Category:** %s\n\n", m.Category)
		}
		if len(m.Tags) > 0 {
			content += fmt.Sprintf("**Tags:** %s\n\n", strings.Join(m.Tags, ", "))
		}
		if len(m.Inputs) > 0 {
			content += "## Inputs\n\n"
			for _, input := range m.Inputs {
				required := ""
				if input.Required {
					required = " (required)"
				}
				content += fmt.Sprintf("- **%s**%s: %s\n", input.Name, required, input.Description)
			}
			content += "\n"
		}
		if len(m.Outputs) > 0 {
			content += "## Outputs\n\n"
			for _, output := range m.Outputs {
				content += fmt.Sprintf("- **%s**: %s\n", output.Name, output.Description)
			}
		}
		return c.JSON(http.StatusOK, map[string]interface{}{
			"id":      id,
			"name":    m.Name,
			"content": content,
			"source":  "builtin",
		})
	}

	// No content available
	return c.JSON(http.StatusOK, map[string]interface{}{
		"id":      id,
		"name":    info.Manifest.Name,
		"content": "",
		"source":  "none",
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

// UploadSkill handles skill package upload
// TODO: Full implementation requires skill package format specification
func (h *SkillHandler) UploadSkill(c echo.Context) error {
	// Get uploaded file
	file, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "no file uploaded",
		})
	}

	// Open the file
	src, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "failed to open uploaded file",
		})
	}
	defer src.Close()

	// Read file content
	content, err := io.ReadAll(src)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "failed to read uploaded file",
		})
	}

	// Try to parse as JSON manifest
	var manifest skill.Manifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "invalid skill manifest: " + err.Error(),
		})
	}

	// Validate manifest
	if manifest.ID == "" || manifest.Name == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "skill manifest must have id and name",
		})
	}

	// Check if already installed
	if h.registry.Get(manifest.ID) != nil {
		return c.JSON(http.StatusConflict, map[string]interface{}{
			"success": false,
			"message": "skill already installed",
		})
	}

	// For now, return not implemented as skills require executable code
	// Full implementation would need to:
	// 1. Extract skill package (zip/tar)
	// 2. Validate skill structure
	// 3. Copy to skills directory
	// 4. Load and register the skill
	return c.JSON(http.StatusNotImplemented, map[string]interface{}{
		"success": false,
		"message": "skill upload not yet implemented - use install from URL instead",
		"skill": map[string]string{
			"id":      manifest.ID,
			"name":    manifest.Name,
			"version": manifest.Version,
		},
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

	// If no skills found and featured fallback is enabled, use featured skills
	if len(skills) == 0 && h.useFeaturedFallback && h.featuredLoader != nil {
		featuredSkills := h.getFeaturedSkillsFallback()
		for _, fs := range featuredSkills {
			// Apply filters
			if category != "" && fs.Category != category {
				continue
			}
			if search != "" {
				if !containsIgnoreCase(fs.Name, search) && !containsIgnoreCase(fs.Description, search) {
					continue
				}
			}
			fs.Installed = installedIDs[fs.ID]
			skills = append(skills, fs)
		}
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

// InstallSkill installs a skill from a remote source with retry logic
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

	// Download skill manifest from source with retry logic
	const maxRetries = 3
	var manifest *skill.Manifest
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		manifest, lastErr = h.downloadSkillManifest(ctx, rs)
		if lastErr == nil {
			break
		}

		// Check if context is cancelled
		if ctx.Err() != nil {
			return c.JSON(http.StatusRequestTimeout, map[string]string{
				"error":   "request cancelled",
				"attempt": fmt.Sprintf("%d/%d", attempt, maxRetries),
			})
		}

		// Wait before retry (exponential backoff: 1s, 2s, 4s)
		if attempt < maxRetries {
			select {
			case <-ctx.Done():
				return c.JSON(http.StatusRequestTimeout, map[string]string{
					"error":   "request cancelled during retry",
					"attempt": fmt.Sprintf("%d/%d", attempt, maxRetries),
				})
			case <-time.After(time.Duration(1<<(attempt-1)) * time.Second):
				// Continue to next retry
			}
		}
	}

	if lastErr != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error":       fmt.Sprintf("failed to download skill after %d attempts: %v", maxRetries, lastErr),
			"retry_count": fmt.Sprintf("%d", maxRetries),
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
	// If this fails, rollback the registration
	if h.store != nil {
		if err := h.store.SetInstalled(ctx, id, true); err != nil {
			// Rollback: unregister the skill
			h.registry.Unregister(id)
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error":    fmt.Sprintf("failed to update database: %v", err),
				"rollback": "skill registration rolled back",
			})
		}
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
	ctx := c.Request().Context()

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

	// Update installed status in database
	if h.store != nil {
		// Ignore error - skill is already unregistered from memory
		_ = h.store.SetInstalled(ctx, id, false)
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

	// Create a context with timeout for external API calls (60 seconds for full pagination)
	ctx, cancel := context.WithTimeout(c.Request().Context(), 60*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	var mu sync.Mutex
	errors := make([]string, 0)
	fetchedAny := false

	for _, source := range h.sources {
		if !source.Enabled {
			continue
		}

		wg.Add(1)
		go func(src *SkillSource) {
			defer wg.Done()

			skills, err := h.fetchSkillsFromSource(ctx, src)
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
			if len(skills) > 0 {
				fetchedAny = true
			}
			mu.Unlock()
		}(source)
	}

	wg.Wait()

	// If no skills were fetched and featured fallback is enabled, use featured skills
	if !fetchedAny && h.useFeaturedFallback && h.featuredLoader != nil {
		featuredSkills := h.getFeaturedSkillsFallback()
		for _, s := range featuredSkills {
			h.remoteSkills[s.ID] = s
		}
	}

	response := map[string]interface{}{
		"success":      len(h.remoteSkills) > 0,
		"skills_count": len(h.remoteSkills),
	}

	if len(errors) > 0 {
		response["errors"] = errors
		if len(h.remoteSkills) > 0 {
			response["message"] = "partial success with featured fallback"
		}
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
	maxPages := 100     // Limit to prevent infinite loops
	_ = 24              // ClawHub returns 24 items per page (for reference)
	emptyPageCount := 0 // Track consecutive empty pages
	maxEmptyPages := 2  // Stop after 2 consecutive empty pages

	for page := 1; page <= maxPages; page++ {
		// Check if context is cancelled (timeout)
		select {
		case <-ctx.Done():
			if len(allSkills) > 0 {
				// Return what we have so far
				fmt.Printf("[ClawHub] Context cancelled, returning %d skills fetched so far\n", len(allSkills))
				return allSkills, nil
			}
			// If no skills fetched yet, try featured fallback
			if h.useFeaturedFallback && h.featuredLoader != nil {
				return h.getFeaturedSkillsFallback(), nil
			}
			return nil, ctx.Err()
		default:
		}

		// Build URL with page parameter
		apiURL := fmt.Sprintf("%s?page=%d", baseURL, page)

		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			fmt.Printf("[ClawHub] Failed to create request: %v\n", err)
			if h.useMockData && len(allSkills) == 0 {
				return h.getMockClawHubSkills(source), nil
			}
			if len(allSkills) > 0 {
				// Return what we have so far
				return allSkills, nil
			}
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// Parse ClawHub API response
		var apiResp ClawHubAPIResponse
		resp, err := h.httpClient.Do(req)
		if err == nil {
			if resp.StatusCode != http.StatusOK {
				fmt.Printf("[ClawHub] API returned status %d from %s\n", resp.StatusCode, source.Name)
				resp.Body.Close()
			}

			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			_ = json.Unmarshal(body, &apiResp)

			// Debug: Log pagination info
			fmt.Printf("[ClawHub] Page %d: fetched %d items, total so far=%d\n",
				page, len(apiResp.Items), len(allSkills)+len(apiResp.Items))
		}

		// Check if page is empty
		if len(apiResp.Items) == 0 {
			emptyPageCount++
			if emptyPageCount >= maxEmptyPages {
				fmt.Printf("[ClawHub] %d consecutive empty pages, stopping pagination. Total skills: %d\n", maxEmptyPages, len(allSkills))
				break
			}
			continue
		}
		emptyPageCount = 0 // Reset counter on non-empty page

		// Convert ClawHub skills to RemoteSkill format
		for _, item := range apiResp.Items {
			allSkills = append(allSkills, h.convertClawHubSkill(item, source))
		}
	}

	fmt.Printf("[ClawHub] Pagination complete. Total skills fetched: %d\n", len(allSkills))
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

	// Parse cursor-based pagination
	opts.Cursor = c.QueryParam("cursor")
	if count := c.QueryParam("count"); count != "" {
		if v, err := strconv.Atoi(count); err == nil {
			opts.Count = v
		}
	}

	// Set defaults
	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.PageSize < 1 || opts.PageSize > 100 {
		opts.PageSize = 24
	}
	// Use count for cursor-based pagination
	if opts.Cursor != "" && opts.Count > 0 {
		opts.PageSize = opts.Count
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

	// Try cache first
	cacheKey := "skill_categories"
	if cached, ok := h.categoriesCache.Get(cacheKey); ok {
		return c.JSON(http.StatusOK, cached)
	}

	// Use singleflight to deduplicate concurrent requests
	result, err, _ := h.sfGroup.Do("get_skill_categories", func() (interface{}, error) {
		categories, err := h.store.GetCategories(c.Request().Context())
		if err != nil {
			return nil, err
		}
		// Cache the result
		h.categoriesCache.Put(cacheKey, categories)
		return categories, nil
	})

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to get categories: %v", err),
		})
	}

	return c.JSON(http.StatusOK, result)
}

// GetStats returns skill statistics
func (h *SkillHandler) GetStats(c echo.Context) error {
	if h.store == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "skill store not initialized",
		})
	}

	// Try cache first
	cacheKey := "skill_stats"
	if cached, ok := h.statsCache.Get(cacheKey); ok {
		return c.JSON(http.StatusOK, cached)
	}

	// Use singleflight to deduplicate concurrent requests
	result, err, _ := h.sfGroup.Do("get_skill_stats", func() (interface{}, error) {
		stats, err := h.store.GetStats(c.Request().Context())
		if err != nil {
			return nil, err
		}
		// Cache the result
		h.statsCache.Put(cacheKey, stats)
		return stats, nil
	})

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to get stats: %v", err),
		})
	}

	return c.JSON(http.StatusOK, result)
}

// GetPopularSkills returns the most popular skills by downloads
func (h *SkillHandler) GetPopularSkills(c echo.Context) error {
	if h.store == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "skill store not initialized",
		})
	}

	limit := 20
	if l := c.QueryParam("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}

	skills, err := h.store.GetPopular(c.Request().Context(), limit)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to get popular skills: %v", err),
		})
	}

	return c.JSON(http.StatusOK, skills)
}

// GetRecentSkills returns the most recently updated skills
func (h *SkillHandler) GetRecentSkills(c echo.Context) error {
	if h.store == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "skill store not initialized",
		})
	}

	limit := 20
	if l := c.QueryParam("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}

	skills, err := h.store.GetRecent(c.Request().Context(), limit)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to get recent skills: %v", err),
		})
	}

	return c.JSON(http.StatusOK, skills)
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

// =============================================================================
// New v0.10.8 Endpoints
// =============================================================================

// GetFeaturedSkills returns the curated list of featured skills
// This serves as a fallback when external APIs fail
func (h *SkillHandler) GetFeaturedSkills(c echo.Context) error {
	if h.featuredLoader == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "featured skills not configured",
		})
	}

	if !h.featuredLoader.IsLoaded() {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "featured skills not loaded",
		})
	}

	// Get optional category filter
	category := c.QueryParam("category")

	var skills []*skillstore.FeaturedSkill
	if category != "" {
		skills = h.featuredLoader.GetByCategory(category)
	} else {
		skills = h.featuredLoader.GetAll()
	}

	// Convert to RemoteSkill format for consistency
	result := make([]*RemoteSkill, 0, len(skills))
	for _, s := range skills {
		result = append(result, &RemoteSkill{
			ID:          s.ID,
			Name:        s.Name,
			Version:     s.Version,
			Description: s.Description,
			Author:      s.Author,
			Category:    s.Category,
			Tags:        s.Tags,
			SourceID:    "featured",
			SourceName:  "Featured",
			Homepage:    s.Homepage,
			DownloadURL: s.SourceURL,
			Stars:       s.Stars,
			Installed:   h.registry.Get(s.ID) != nil,
		})
	}

	return c.JSON(http.StatusOK, result)
}

// ListLocalSkills returns skills discovered from local directory
func (h *SkillHandler) ListLocalSkills(c echo.Context) error {
	if h.localScanner == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "local skill scanner not configured",
		})
	}

	skills := h.localScanner.GetAll()

	// Convert to response format
	result := make([]map[string]interface{}, 0, len(skills))
	for _, s := range skills {
		result = append(result, map[string]interface{}{
			"id":            s.ID,
			"name":          s.Name,
			"description":   s.Description,
			"version":       s.Version,
			"author":        s.Author,
			"category":      s.Category,
			"tags":          s.Tags,
			"file_path":     s.FilePath,
			"discovered_at": s.DiscoveredAt,
			"last_modified": s.LastModified,
			"installed":     h.registry.Get(s.ID) != nil,
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"skills": result,
		"count":  len(result),
	})
}

// ScanLocalSkills triggers a scan of the local skills directory
func (h *SkillHandler) ScanLocalSkills(c echo.Context) error {
	if h.localScanner == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "local skill scanner not configured",
		})
	}

	if err := h.localScanner.Scan(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("scan failed: %v", err),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":      true,
		"skills_found": h.localScanner.Count(),
	})
}

// VerifySkill verifies that a skill is properly registered and visible
func (h *SkillHandler) VerifySkill(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "skill ID is required",
		})
	}

	// Check if skill exists in registry
	skill := h.registry.Get(id)
	if skill == nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"id":      id,
			"visible": false,
			"error":   "skill not found in registry",
		})
	}

	// Get skill info
	info := h.registry.GetInfo(id)
	manifest := skill.Manifest()

	return c.JSON(http.StatusOK, map[string]interface{}{
		"id":          id,
		"visible":     true,
		"enabled":     info.Enabled,
		"builtin":     info.Builtin,
		"name":        manifest.Name,
		"version":     manifest.Version,
		"description": manifest.Description,
	})
}

// InstallFromURLRequest represents a request to install a skill from URL
type InstallFromURLRequest struct {
	URL         string `json:"url"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// InstallFromURL installs a skill from a URL (GitHub raw URL or direct link) with retry logic
func (h *SkillHandler) InstallFromURL(c echo.Context) error {
	var req InstallFromURLRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
	}

	if req.URL == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "URL is required",
		})
	}

	ctx := c.Request().Context()

	// Fetch the skill content from URL with retry logic
	const maxRetries = 3
	var body []byte
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		httpReq, err := http.NewRequestWithContext(ctx, "GET", req.URL, nil)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("invalid URL: %v", err),
			})
		}

		resp, err := h.httpClient.Do(httpReq)
		if err != nil {
			lastErr = fmt.Errorf("failed to fetch skill: %v", err)
		} else {
			if resp.StatusCode == http.StatusOK {
				body, lastErr = io.ReadAll(resp.Body)
				resp.Body.Close()
				if lastErr == nil {
					break // Success
				}
				lastErr = fmt.Errorf("failed to read skill content: %v", lastErr)
			} else {
				resp.Body.Close()
				lastErr = fmt.Errorf("failed to fetch skill: HTTP %d", resp.StatusCode)
			}
		}

		// Check if context is cancelled
		if ctx.Err() != nil {
			return c.JSON(http.StatusRequestTimeout, map[string]string{
				"error":   "request cancelled",
				"attempt": fmt.Sprintf("%d/%d", attempt, maxRetries),
			})
		}

		// Wait before retry (exponential backoff: 1s, 2s, 4s)
		if attempt < maxRetries {
			select {
			case <-ctx.Done():
				return c.JSON(http.StatusRequestTimeout, map[string]string{
					"error":   "request cancelled during retry",
					"attempt": fmt.Sprintf("%d/%d", attempt, maxRetries),
				})
			case <-time.After(time.Duration(1<<(attempt-1)) * time.Second):
				// Continue to next retry
			}
		}
	}

	if lastErr != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{
			"error":       fmt.Sprintf("failed after %d attempts: %v", maxRetries, lastErr),
			"retry_count": fmt.Sprintf("%d", maxRetries),
		})
	}

	// Parse the skill content (expecting SKILL.md format)
	skillID, skillManifest, err := h.parseSkillContent(string(body), req.URL, req.Name, req.Description)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("failed to parse skill: %v", err),
		})
	}

	// Check if already installed
	if h.registry.Get(skillID) != nil {
		return c.JSON(http.StatusConflict, map[string]string{
			"error": "skill already installed",
		})
	}

	// Create and register the skill
	adapter := NewRemoteSkillAdapter(skillManifest)
	if err := h.registry.Register(adapter, false); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to register skill: %v", err),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"skill": map[string]interface{}{
			"id":          skillManifest.ID,
			"name":        skillManifest.Name,
			"version":     skillManifest.Version,
			"description": skillManifest.Description,
		},
	})
}

// parseSkillContent parses SKILL.md content and returns skill ID and manifest
func (h *SkillHandler) parseSkillContent(content, sourceURL, overrideName, overrideDesc string) (string, *skill.Manifest, error) {
	manifest := &skill.Manifest{
		Version:  "1.0.0",
		Metadata: make(map[string]string),
	}

	// Parse YAML frontmatter if present
	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) >= 3 {
			frontmatter := parts[1]

			// Parse frontmatter lines
			for _, line := range strings.Split(frontmatter, "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}

				colonIdx := strings.Index(line, ":")
				if colonIdx == -1 {
					continue
				}

				key := strings.TrimSpace(line[:colonIdx])
				value := strings.TrimSpace(line[colonIdx+1:])

				switch key {
				case "name":
					manifest.Name = value
					if manifest.ID == "" {
						manifest.ID = value
					}
				case "id":
					manifest.ID = value
				case "description":
					manifest.Description = value
				case "version":
					manifest.Version = value
				case "author":
					manifest.Author = value
				case "category":
					manifest.Category = value
				case "tags":
					// Parse tags array [tag1, tag2]
					value = strings.Trim(value, "[]")
					for _, tag := range strings.Split(value, ",") {
						tag = strings.TrimSpace(tag)
						if tag != "" {
							manifest.Tags = append(manifest.Tags, tag)
						}
					}
				}
			}
		}
	}

	// Apply overrides
	if overrideName != "" {
		manifest.Name = overrideName
	}
	if overrideDesc != "" {
		manifest.Description = overrideDesc
	}

	// Generate ID from URL if not set
	if manifest.ID == "" {
		// Extract ID from URL path
		parts := strings.Split(sourceURL, "/")
		for i := len(parts) - 1; i >= 0; i-- {
			part := strings.TrimSuffix(parts[i], ".md")
			part = strings.TrimSuffix(part, ".MD")
			if part != "" && part != "SKILL" {
				manifest.ID = strings.ToLower(part)
				break
			}
		}
	}

	// Use ID as name if not set
	if manifest.Name == "" {
		manifest.Name = manifest.ID
	}

	if manifest.ID == "" {
		return "", nil, fmt.Errorf("could not determine skill ID")
	}

	// Store source URL in metadata
	manifest.Metadata["source_url"] = sourceURL

	return manifest.ID, manifest, nil
}

// getFeaturedSkillsFallback returns featured skills when API fails
func (h *SkillHandler) getFeaturedSkillsFallback() []*RemoteSkill {
	if h.featuredLoader == nil || !h.featuredLoader.IsLoaded() {
		return nil
	}

	skills := h.featuredLoader.GetAll()
	result := make([]*RemoteSkill, 0, len(skills))
	for _, s := range skills {
		result = append(result, &RemoteSkill{
			ID:          s.ID,
			Name:        s.Name,
			Version:     s.Version,
			Description: s.Description,
			Author:      s.Author,
			Category:    s.Category,
			Tags:        s.Tags,
			SourceID:    "featured",
			SourceName:  "Featured",
			Homepage:    s.Homepage,
			DownloadURL: s.SourceURL,
			Stars:       s.Stars,
			Installed:   h.registry.Get(s.ID) != nil,
		})
	}
	return result
}
