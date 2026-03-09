package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/labstack/echo/v4"
)

// ToolSource represents an external tool source
type ToolSource struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	Type        string `json:"type"` // "registry", "github", "custom"
	Description string `json:"description,omitempty"`
	Enabled     bool   `json:"enabled"`
}

// RemoteTool represents a tool from an external source
type RemoteTool struct {
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

// ToolStoreHandler handles tool store HTTP requests
type ToolStoreHandler struct {
	registry    *tools.Registry
	sources     map[string]*ToolSource
	remoteTools map[string]*RemoteTool
	mu          sync.RWMutex
	httpClient  *http.Client
}

// NewToolStoreHandler creates a new tool store handler
func NewToolStoreHandler(registry *tools.Registry) *ToolStoreHandler {
	h := &ToolStoreHandler{
		registry:    registry,
		sources:     make(map[string]*ToolSource),
		remoteTools: make(map[string]*RemoteTool),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// Register default sources
	h.sources["toolhub"] = &ToolSource{
		ID:          "toolhub",
		Name:        "ToolHub",
		URL:         "https://toolhub.zimaos.com",
		Type:        "registry",
		Description: "Official ZimaOS tool marketplace",
		Enabled:     true,
	}

	return h
}

// RegisterRoutes registers tool store routes
func (h *ToolStoreHandler) RegisterRoutes(g *echo.Group) {
	// Tool management routes
	toolsGroup := g.Group("/tools")
	toolsGroup.GET("", h.ListTools)
	toolsGroup.GET("/:id", h.GetTool)
	toolsGroup.POST("/:id/enable", h.EnableTool)
	toolsGroup.POST("/:id/disable", h.DisableTool)

	// Tool store routes
	store := g.Group("/tool-store")
	store.GET("/sources", h.ListSources)
	store.POST("/sources", h.AddSource)
	store.DELETE("/sources/:id", h.RemoveSource)
	store.GET("/browse", h.BrowseTools)
	store.POST("/install/:id", h.InstallTool)
	store.POST("/uninstall/:id", h.UninstallTool)
	store.POST("/refresh", h.RefreshSources)
}

// ToolResponse represents a tool in API responses
type ToolResponse struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Version     string                 `json:"version"`
	Description string                 `json:"description"`
	Author      string                 `json:"author,omitempty"`
	Category    string                 `json:"category,omitempty"`
	Icon        string                 `json:"icon,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Enabled     bool                   `json:"enabled"`
	Builtin     bool                   `json:"builtin"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// toolsHiddenFromUI lists tools that should not appear on the plugins page.
// These are internal-only tools (e.g. memory is accessed via compat/internal routing).
var toolsHiddenFromUI = map[string]bool{
	"memory": true,
}

// skillsShownAsTools lists skills that should appear in the Tools tab instead
// of the Skills tab. Display-only — no logic changes.
var skillsShownAsTools = []ToolResponse{
	{ID: "browser", Name: "browser", Version: "1.0.0", Description: "Open a URL, read page content, interact with elements, take screenshots", Icon: "browser", Enabled: true, Builtin: true},
	{ID: "ui_reviewer", Name: "ui_reviewer", Version: "1.0.0", Description: "Score and audit UI/UX quality of a URL or screenshot", Icon: "eye", Enabled: true, Builtin: true},
	{ID: "analyze", Name: "analyze", Version: "1.0.0", Description: "Deep-dive analysis: gather data from URLs and web searches, generate HTML report", Icon: "analyze", Enabled: true, Builtin: true},
	{ID: "mediagen", Name: "mediagen", Version: "1.0.0", Description: "Generate images and videos using AI models", Icon: "mediagen", Enabled: true, Builtin: true},
	{ID: "reminder", Name: "reminder", Version: "2.0.0", Description: "Manage reminders and scheduled alerts", Icon: "notifications", Enabled: true, Builtin: true},
}

// ListTools returns all registered tools (including disabled ones)
func (h *ToolStoreHandler) ListTools(c echo.Context) error {
	toolNames := h.registry.List()
	response := make([]ToolResponse, 0, len(toolNames)+len(skillsShownAsTools))
	seen := make(map[string]struct{}, len(toolNames)+len(skillsShownAsTools))
	appendUnique := func(item ToolResponse) {
		if item.ID == "" {
			return
		}
		if _, ok := seen[item.ID]; ok {
			return
		}
		seen[item.ID] = struct{}{}
		response = append(response, item)
	}

	for _, name := range toolNames {
		if toolsHiddenFromUI[name] {
			continue
		}
		tool := h.registry.Get(name)
		if tool == nil {
			continue
		}
		def := tool.Definition()
		appendUnique(ToolResponse{
			ID:          def.Name,
			Name:        def.Name,
			Version:     "1.0.0",
			Description: def.Description,
			Icon:        def.Icon,
			Enabled:     true,
			Builtin:     true,
			Parameters:  def.Parameters,
		})
	}

	// Also include disabled tools (skip hidden ones)
	for _, name := range h.registry.ListDisabled() {
		if toolsHiddenFromUI[name] {
			continue
		}
		tool := h.registry.Get(name)
		if tool == nil {
			continue
		}
		def := tool.Definition()
		appendUnique(ToolResponse{
			ID:          def.Name,
			Name:        def.Name,
			Version:     "1.0.0",
			Description: def.Description,
			Icon:        def.Icon,
			Enabled:     false,
			Builtin:     true,
			Parameters:  def.Parameters,
		})
	}

	// Append skills that are displayed as tools in the UI.
	for _, item := range skillsShownAsTools {
		appendUnique(item)
	}

	return c.JSON(http.StatusOK, response)
}

// GetTool returns a specific tool
func (h *ToolStoreHandler) GetTool(c echo.Context) error {
	id := c.Param("id")
	tool := h.registry.Get(id)
	if tool == nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "tool not found",
		})
	}

	def := tool.Definition()
	return c.JSON(http.StatusOK, ToolResponse{
		ID:          def.Name,
		Name:        def.Name,
		Version:     "1.0.0",
		Description: def.Description,
		Icon:        def.Icon,
		Enabled:     !h.registry.IsDisabled(def.Name),
		Builtin:     true,
		Parameters:  def.Parameters,
	})
}

// EnableTool enables a tool
func (h *ToolStoreHandler) EnableTool(c echo.Context) error {
	id := c.Param("id")
	if !h.registry.Enable(id) {
		// Not in disabled set — check if it exists at all
		if h.registry.Get(id) == nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "tool not found"})
		}
		// Already enabled
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "tool enabled",
	})
}

// DisableTool disables a tool
func (h *ToolStoreHandler) DisableTool(c echo.Context) error {
	id := c.Param("id")
	if !h.registry.Disable(id) {
		// Not in active set — check if it exists at all
		if h.registry.Get(id) == nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "tool not found"})
		}
		// Already disabled
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "tool disabled",
	})
}

// ListSources returns all tool sources
func (h *ToolStoreHandler) ListSources(c echo.Context) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	sources := make([]*ToolSource, 0, len(h.sources))
	for _, s := range h.sources {
		sources = append(sources, s)
	}
	return c.JSON(http.StatusOK, sources)
}

// AddSource adds a new tool source
func (h *ToolStoreHandler) AddSource(c echo.Context) error {
	var source ToolSource
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

// RemoveSource removes a tool source
func (h *ToolStoreHandler) RemoveSource(c echo.Context) error {
	id := c.Param("id")

	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.sources[id]; !exists {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "source not found",
		})
	}

	// Don't allow removing default sources
	if id == "toolhub" || id == "moltbot" {
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

// BrowseTools returns tools from all enabled sources
func (h *ToolStoreHandler) BrowseTools(c echo.Context) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Get query parameters
	sourceID := c.QueryParam("source")
	category := c.QueryParam("category")
	search := c.QueryParam("search")

	toolsList := make([]*RemoteTool, 0)

	// Get installed tool IDs
	installedIDs := make(map[string]bool)
	for _, name := range h.registry.List() {
		installedIDs[name] = true
	}

	for _, rt := range h.remoteTools {
		// Filter by source
		if sourceID != "" && rt.SourceID != sourceID {
			continue
		}

		// Filter by category
		if category != "" && rt.Category != category {
			continue
		}

		// Filter by search
		if search != "" {
			if !containsIgnoreCase(rt.Name, search) && !containsIgnoreCase(rt.Description, search) {
				continue
			}
		}

		// Mark as installed if in registry
		rt.Installed = installedIDs[rt.ID]
		toolsList = append(toolsList, rt)
	}

	return c.JSON(http.StatusOK, toolsList)
}

// InstallTool installs a tool from a remote source
func (h *ToolStoreHandler) InstallTool(c echo.Context) error {
	id := c.Param("id")

	h.mu.RLock()
	rt, exists := h.remoteTools[id]
	h.mu.RUnlock()

	if !exists {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "tool not found in store",
		})
	}

	// Check if already installed
	if h.registry.Get(id) != nil {
		return c.JSON(http.StatusConflict, map[string]string{
			"error": "tool already installed",
		})
	}

	// TODO: Actually download and install the tool
	// For now, return a placeholder response
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("tool %s queued for installation", rt.Name),
		"tool":    rt,
	})
}

// UninstallTool uninstalls a tool
func (h *ToolStoreHandler) UninstallTool(c echo.Context) error {
	id := c.Param("id")

	tool := h.registry.Get(id)
	if tool == nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "tool not found",
		})
	}

	// Don't allow uninstalling builtin tools
	// TODO: Add builtin flag to tools
	return c.JSON(http.StatusForbidden, map[string]string{
		"error": "cannot uninstall builtin tool",
	})
}

// RefreshSources refreshes tools from all enabled sources
func (h *ToolStoreHandler) RefreshSources(c echo.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Clear existing remote tools
	h.remoteTools = make(map[string]*RemoteTool)

	var wg sync.WaitGroup
	var mu sync.Mutex
	errors := make([]string, 0)

	for _, source := range h.sources {
		if !source.Enabled {
			continue
		}

		wg.Add(1)
		go func(src *ToolSource) {
			defer wg.Done()

			toolsList, err := h.fetchToolsFromSource(c.Request().Context(), src)
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("%s: %v", src.Name, err))
				mu.Unlock()
				return
			}

			mu.Lock()
			for _, t := range toolsList {
				h.remoteTools[t.ID] = t
			}
			mu.Unlock()
		}(source)
	}

	wg.Wait()

	response := map[string]interface{}{
		"success":     len(errors) == 0,
		"tools_count": len(h.remoteTools),
	}

	if len(errors) > 0 {
		response["errors"] = errors
	}

	return c.JSON(http.StatusOK, response)
}

// fetchToolsFromSource fetches tools from a specific source
func (h *ToolStoreHandler) fetchToolsFromSource(ctx context.Context, source *ToolSource) ([]*RemoteTool, error) {
	switch source.Type {
	case "registry":
		return h.fetchFromRegistry(ctx, source)
	case "github":
		return h.fetchFromGitHub(ctx, source)
	default:
		return h.fetchFromCustom(ctx, source)
	}
}

// fetchFromRegistry fetches tools from a registry
func (h *ToolStoreHandler) fetchFromRegistry(ctx context.Context, source *ToolSource) ([]*RemoteTool, error) {
	apiURL := source.URL + "/api/tools"

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		// Return mock data if API is not available
		return h.getMockRegistryTools(source), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return h.getMockRegistryTools(source), nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var toolsList []*RemoteTool
	if err := json.Unmarshal(body, &toolsList); err != nil {
		return h.getMockRegistryTools(source), nil
	}

	for _, t := range toolsList {
		t.SourceID = source.ID
		t.SourceName = source.Name
	}

	return toolsList, nil
}

// getMockRegistryTools returns empty list when registry is unavailable
func (h *ToolStoreHandler) getMockRegistryTools(source *ToolSource) []*RemoteTool {
	return []*RemoteTool{}
}

// fetchFromGitHub fetches tools from a GitHub repository
func (h *ToolStoreHandler) fetchFromGitHub(ctx context.Context, source *ToolSource) ([]*RemoteTool, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", source.URL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return h.getMockGitHubTools(source), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return h.getMockGitHubTools(source), nil
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
		return h.getMockGitHubTools(source), nil
	}

	toolsList := make([]*RemoteTool, 0)
	for _, item := range contents {
		if item.Type == "dir" {
			toolsList = append(toolsList, &RemoteTool{
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

	return toolsList, nil
}

// getMockGitHubTools returns empty list when GitHub is unavailable
func (h *ToolStoreHandler) getMockGitHubTools(source *ToolSource) []*RemoteTool {
	return []*RemoteTool{}
}

// fetchFromCustom fetches tools from a custom source
func (h *ToolStoreHandler) fetchFromCustom(ctx context.Context, source *ToolSource) ([]*RemoteTool, error) {
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

	var toolsList []*RemoteTool
	if err := json.Unmarshal(body, &toolsList); err != nil {
		return nil, err
	}

	for _, t := range toolsList {
		t.SourceID = source.ID
		t.SourceName = source.Name
	}

	return toolsList, nil
}
