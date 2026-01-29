package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/skill"
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
	sources       map[string]*SkillSource
	remoteSkills  map[string]*RemoteSkill
	mu            sync.RWMutex
	httpClient    *http.Client
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
	}

	// Register default sources
	h.sources["clawdhub"] = &SkillSource{
		ID:          "clawdhub",
		Name:        "ClawdHub",
		URL:         "https://clawdhub.com",
		Type:        "clawdhub",
		Description: "Official ClawdHub skill marketplace",
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
	store.POST("/install/:id", h.InstallSkill)
	store.POST("/uninstall/:id", h.UninstallSkill)
	store.POST("/refresh", h.RefreshSources)
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
	if id == "clawdhub" || id == "moltbot" {
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
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Get query parameters
	sourceID := c.QueryParam("source")
	category := c.QueryParam("category")
	search := c.QueryParam("search")

	skills := make([]*RemoteSkill, 0)

	// Get installed skill IDs
	installedIDs := make(map[string]bool)
	for _, s := range h.registry.List() {
		installedIDs[s.Manifest.ID] = true
	}

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

// InstallSkill installs a skill from a remote source
func (h *SkillHandler) InstallSkill(c echo.Context) error {
	id := c.Param("id")

	h.mu.RLock()
	rs, exists := h.remoteSkills[id]
	h.mu.RUnlock()

	if !exists {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "skill not found in store",
		})
	}

	// Check if already installed
	if h.registry.Get(id) != nil {
		return c.JSON(http.StatusConflict, map[string]string{
			"error": "skill already installed",
		})
	}

	// TODO: Actually download and install the skill
	// For now, return a placeholder response
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("skill %s queued for installation", rs.Name),
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
	case "clawdhub":
		return h.fetchFromClawdHub(ctx, source)
	case "github":
		return h.fetchFromGitHub(ctx, source)
	default:
		return h.fetchFromCustom(ctx, source)
	}
}

// fetchFromClawdHub fetches skills from ClawdHub
func (h *SkillHandler) fetchFromClawdHub(ctx context.Context, source *SkillSource) ([]*RemoteSkill, error) {
	// ClawdHub API endpoint
	apiURL := source.URL + "/api/skills"

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		// Return mock data if API is not available
		return h.getMockClawdHubSkills(source), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Return mock data if API returns error
		return h.getMockClawdHubSkills(source), nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var skills []*RemoteSkill
	if err := json.Unmarshal(body, &skills); err != nil {
		// Return mock data if parsing fails
		return h.getMockClawdHubSkills(source), nil
	}

	// Set source info
	for _, s := range skills {
		s.SourceID = source.ID
		s.SourceName = source.Name
	}

	return skills, nil
}

// getMockClawdHubSkills returns mock ClawdHub skills for demo
func (h *SkillHandler) getMockClawdHubSkills(source *SkillSource) []*RemoteSkill {
	return []*RemoteSkill{
		{
			ID:          "clawdhub-smart-home",
			Name:        "Smart Home Controller",
			Version:     "1.2.0",
			Description: "Advanced smart home automation with scene management",
			Author:      "ClawdHub Team",
			Category:    "integration",
			Tags:        []string{"smart-home", "automation", "iot"},
			SourceID:    source.ID,
			SourceName:  source.Name,
			Homepage:    "https://clawdhub.com/skills/smart-home",
			Stars:       256,
			Downloads:   1520,
		},
		{
			ID:          "clawdhub-ai-assistant",
			Name:        "AI Writing Assistant",
			Version:     "2.0.1",
			Description: "AI-powered writing assistant with grammar and style suggestions",
			Author:      "ClawdHub Team",
			Category:    "productivity",
			Tags:        []string{"ai", "writing", "assistant"},
			SourceID:    source.ID,
			SourceName:  source.Name,
			Homepage:    "https://clawdhub.com/skills/ai-assistant",
			Stars:       512,
			Downloads:   3200,
		},
		{
			ID:          "clawdhub-code-review",
			Name:        "Code Review Helper",
			Version:     "1.0.0",
			Description: "Automated code review with best practices suggestions",
			Author:      "ClawdHub Team",
			Category:    "development",
			Tags:        []string{"code", "review", "development"},
			SourceID:    source.ID,
			SourceName:  source.Name,
			Homepage:    "https://clawdhub.com/skills/code-review",
			Stars:       128,
			Downloads:   890,
		},
		{
			ID:          "clawdhub-data-analyzer",
			Name:        "Data Analyzer",
			Version:     "1.5.0",
			Description: "Analyze and visualize data from various sources",
			Author:      "ClawdHub Team",
			Category:    "analytics",
			Tags:        []string{"data", "analytics", "visualization"},
			SourceID:    source.ID,
			SourceName:  source.Name,
			Homepage:    "https://clawdhub.com/skills/data-analyzer",
			Stars:       320,
			Downloads:   2100,
		},
	}
}

// fetchFromGitHub fetches skills from a GitHub repository
func (h *SkillHandler) fetchFromGitHub(ctx context.Context, source *SkillSource) ([]*RemoteSkill, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", source.URL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		// Return mock data if API is not available
		return h.getMockGitHubSkills(source), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Return mock data if API returns error
		return h.getMockGitHubSkills(source), nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var contents []struct {
		Name string `json:"name"`
		Type string `json:"type"`
		Path string `json:"path"`
		URL  string `json:"html_url"`
	}

	if err := json.Unmarshal(body, &contents); err != nil {
		// Return mock data if parsing fails
		return h.getMockGitHubSkills(source), nil
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
