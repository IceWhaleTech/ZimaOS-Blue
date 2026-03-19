package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/sync/singleflight"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cache"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmarket"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillstore"
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
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Version             string   `json:"version"`
	Description         string   `json:"description"`
	Author              string   `json:"author,omitempty"`
	Category            string   `json:"category,omitempty"`
	Tags                []string `json:"tags,omitempty"`
	SourceID            string   `json:"source_id"`
	SourceName          string   `json:"source_name"`
	SourceGroup         string   `json:"source_group,omitempty"`
	DownloadURL         string   `json:"download_url,omitempty"`
	Homepage            string   `json:"homepage,omitempty"`
	Stars               int      `json:"stars,omitempty"`
	Downloads           int      `json:"downloads,omitempty"`
	Installed           bool     `json:"installed"`
	RiskLevel           string   `json:"risk_level,omitempty"`
	SecurityBadge       string   `json:"security_badge,omitempty"`
	Installable         bool     `json:"installable,omitempty"`
	InstallType         string   `json:"install_type,omitempty"`
	ArtifactKind        string   `json:"artifact_kind,omitempty"`
	HasVulnerabilities  bool     `json:"has_vulnerabilities,omitempty"`
	HasPromptInjection  bool     `json:"has_prompt_injection,omitempty"`
	HasShellInjection   bool     `json:"has_shell_injection,omitempty"`
	HasDataExfiltration bool     `json:"has_data_exfiltration,omitempty"`
	CuratedRank         int      `json:"curated_rank,omitempty"`
	CuratedLabel        string   `json:"curated_label,omitempty"`
}

func (h *SkillHandler) remoteSkillFromDocument(doc skillmarket.SkillDocument) *RemoteSkill {
	return &RemoteSkill{
		ID:                  doc.ID,
		Name:                doc.Name,
		Version:             doc.LatestVersion,
		Description:         doc.Description,
		Author:              doc.Author,
		Category:            doc.Category,
		Tags:                doc.Tags,
		SourceID:            doc.SourceID,
		SourceName:          firstString(doc.SourceName, doc.SourceGroup, doc.SourceID),
		SourceGroup:         doc.SourceGroup,
		DownloadURL:         firstString(doc.DownloadURL, doc.RepoURL, doc.Homepage),
		Homepage:            firstString(doc.Homepage, doc.RepoURL, doc.DownloadURL),
		Stars:               doc.Stars,
		Downloads:           doc.Downloads,
		Installed:           h.registry.Get(doc.ID) != nil,
		RiskLevel:           doc.RiskLevel,
		SecurityBadge:       doc.SecurityBadge,
		Installable:         doc.Installable,
		InstallType:         doc.InstallType,
		ArtifactKind:        doc.ArtifactKind,
		HasVulnerabilities:  doc.HasVulnerabilities,
		HasPromptInjection:  doc.HasPromptInjection,
		HasShellInjection:   doc.HasShellInjection,
		HasDataExfiltration: doc.HasDataExfiltration,
		CuratedRank:         doc.CuratedRank,
		CuratedLabel:        doc.CuratedLabel,
	}
}

func firstString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func skillIDAliases(id string) []string {
	validatedID, err := skillmarket.ValidateSkillID(id)
	if err != nil {
		return nil
	}

	seen := make(map[string]struct{}, 3)
	aliases := make([]string, 0, 3)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, exists := seen[value]; exists {
			return
		}
		seen[value] = struct{}{}
		aliases = append(aliases, value)
	}

	add(validatedID)
	add(strings.ReplaceAll(validatedID, "-", "_"))
	add(strings.ReplaceAll(validatedID, "_", "-"))
	return aliases
}

func validatedSkillID(id string) (string, error) {
	return skillmarket.ValidateSkillID(id)
}

func normalizedSkillID(id string) (string, error) {
	normalized := skillmarket.NormalizeSkillID(id)
	if normalized == "" {
		return "", fmt.Errorf("skill id is required")
	}
	// Local skill installs historically used underscores as canonical separators.
	normalized = strings.ReplaceAll(normalized, "-", "_")
	return skillmarket.ValidateSkillID(normalized)
}

// SkillEventPublisher publishes events to connected SSE clients.
type SkillEventPublisher interface {
	Publish(userID string, eventType string, data any)
}

// SkillHandler handles skill-related HTTP requests
type SkillHandler struct {
	registry            *skill.Registry
	store               *skillstore.Store                // Local database store for skills (browse/search only)
	market              *skillmarket.Service             // Authoritative marketplace service
	syncService         *skillstore.SyncService          // Sync service for periodic updates
	featuredLoader      *skillstore.FeaturedSkillsLoader // Featured skills fallback
	localScanner        *skillstore.LocalSkillScanner    // Local skill discovery
	skillsDir           string                           // {dataDir}/workspace/.claude/skills/
	eventBroker         SkillEventPublisher              // Unified SSE event broker
	sources             map[string]*SkillSource
	remoteSkills        map[string]*RemoteSkill
	mu                  sync.RWMutex
	httpClient          *http.Client
	useMockData         bool // When true, return mock data if API fails; when false, return error
	useFeaturedFallback bool // When true, use featured skills as fallback on API failure

	// singleflight for deduplicating concurrent requests
	sfGroup singleflight.Group

	// cache for frequently accessed data
	browseCache     *cache.GenericCache[string]
	statsCache      *cache.GenericCache[string]
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

// SetMarketplace sets the authoritative marketplace service.
func (h *SkillHandler) SetMarketplace(market *skillmarket.Service) {
	h.market = market
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

// SetSkillsDir sets the directory where skills are installed as {name}/SKILL.md.
func (h *SkillHandler) SetSkillsDir(dir string) {
	h.skillsDir = dir
}

// SetEventBroker sets the unified SSE event broker for publishing install progress.
func (h *SkillHandler) SetEventBroker(broker SkillEventPublisher) {
	h.eventBroker = broker
}

// publishEvent publishes an event to all connected SSE clients for the given user.
// If no broker is configured or userID is empty, the event is silently dropped.
func (h *SkillHandler) publishEvent(userID, eventType string, data any) {
	if h.eventBroker == nil {
		return
	}
	if userID == "" {
		userID = "default"
	}
	h.eventBroker.Publish(userID, eventType, data)
}

// getUserID extracts the user ID from the echo context JWT claims.
func getUserIDFromContext(c echo.Context) string {
	if claims, ok := c.Get("user").(*auth.Claims); ok && claims != nil {
		return claims.UserID
	}
	return "default"
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
	skills.GET("/search", h.MarketSearchSkills)
	skills.GET("/trending", h.MarketTrendingSkills)
	skills.GET("/featured", h.MarketFeaturedSkills)
	skills.GET("/filters", h.MarketFilters)
	skills.GET("/security/:id", h.MarketSecurityReport)
	skills.POST("/install", h.MarketInstallSkill)
	skills.GET("/installed", h.MarketInstalledSkills)
	skills.GET("/discover", h.MarketDiscoverSkills)
	skills.GET("/discover/status", h.MarketDiscoverStatus)
	skills.POST("/discover/refresh", h.MarketDiscoverSkills)
	skills.GET("/updates", h.MarketListUpdates)
	skills.GET("/local", h.ListLocalSkills)       // New: List local skills
	skills.POST("/local/scan", h.ScanLocalSkills) // New: Scan local skills
	skills.GET("/verify/:id", h.VerifySkill)      // New: Verify skill visibility
	skills.POST("/upload", h.UploadSkill)         // Upload skill package
	skills.POST("/:id/uninstall", h.MarketUninstallSkill)
	skills.POST("/:id/update", h.MarketUpdateSkill)
	skills.GET("/:id/content", h.GetSkillContent) // Get skill content (SKILL.md)
	skills.POST("/:id/enable", h.EnableSkill)
	skills.POST("/:id/disable", h.DisableSkill)
	skills.GET("/:id", h.GetSkill)

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

// skillsHiddenFromSkillTab lists skill IDs that are displayed in the Tools tab
// instead of the Skills tab. Display-only change — the skills still function normally.
var skillsHiddenFromSkillTab = map[string]bool{
	"browser":           true,
	"ui_reviewer":       true,
	"analyze":           true,
	"mediagen":          true,
	"reminder":          true,
	"push-notification": true, // legacy name for reminder
}

// ListSkills returns all skills from the .claude/skills/ directory.
func (h *SkillHandler) ListSkills(c echo.Context) error {
	response := make([]SkillResponse, 0)
	seen := make(map[string]struct{})

	if h.localScanner != nil {
		for _, ls := range h.localScanner.GetAll() {
			if skillsHiddenFromSkillTab[ls.ID] {
				continue
			}
			response = append(response, SkillResponse{
				ID:          ls.ID,
				Name:        ls.Name,
				Version:     ls.Version,
				Description: ls.Description,
				Author:      ls.Author,
				Category:    ls.Category,
				Tags:        ls.Tags,
				Enabled:     true,
			})
			seen[ls.ID] = struct{}{}
		}
	}

	// Compatibility fallback: when local scanner is unavailable (or incomplete),
	// expose skills from in-memory registry so installs are visible immediately.
	for _, info := range h.registry.List() {
		if info == nil || info.Manifest == nil {
			continue
		}
		id := info.Manifest.ID
		if id == "" || skillsHiddenFromSkillTab[id] {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		response = append(response, SkillResponse{
			ID:          id,
			Name:        info.Manifest.Name,
			Version:     info.Manifest.Version,
			Description: info.Manifest.Description,
			Author:      info.Manifest.Author,
			Category:    info.Manifest.Category,
			Tags:        info.Manifest.Tags,
			Enabled:     info.Enabled,
			Builtin:     info.Builtin,
			Inputs:      info.Manifest.Inputs,
			Outputs:     info.Manifest.Outputs,
		})
	}

	return c.JSON(http.StatusOK, response)
}

// GetSkill returns a specific skill from the .claude/skills/ directory.
func (h *SkillHandler) GetSkill(c echo.Context) error {
	id, err := validatedSkillID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}
	ctx := c.Request().Context()

	if h.market != nil {
		for _, candidate := range skillIDAliases(id) {
			if detail, err := h.market.GetSkill(ctx, candidate); err == nil && detail != nil {
				return c.JSON(http.StatusOK, detail)
			}
		}
	}

	if h.localScanner != nil {
		for _, candidate := range skillIDAliases(id) {
			if ls := h.localScanner.Get(candidate); ls != nil {
				return c.JSON(http.StatusOK, marketDetailCompatibilityPayload(&RemoteSkill{
					ID:            ls.ID,
					Name:          ls.Name,
					Version:       ls.Version,
					Description:   ls.Description,
					Author:        ls.Author,
					Category:      ls.Category,
					Tags:          ls.Tags,
					SourceID:      "local",
					SourceName:    "Local",
					SourceGroup:   "local",
					Installed:     true,
					SecurityBadge: skillmarket.BadgeYellow,
					RiskLevel:     "unknown",
				}, true, true, false))
			}
		}
	}

	// Compatibility fallback: return registry-backed skill details when scanner
	// is absent or does not include this skill.
	for _, candidate := range skillIDAliases(id) {
		if info := h.registry.GetInfo(candidate); info != nil && info.Manifest != nil {
			return c.JSON(http.StatusOK, marketDetailCompatibilityPayload(&RemoteSkill{
				ID:            info.Manifest.ID,
				Name:          info.Manifest.Name,
				Version:       info.Manifest.Version,
				Description:   info.Manifest.Description,
				Author:        info.Manifest.Author,
				Category:      info.Manifest.Category,
				Tags:          info.Manifest.Tags,
				SourceID:      "registry",
				SourceName:    "Installed",
				SourceGroup:   "registry",
				Installed:     true,
				SecurityBadge: skillmarket.BadgeYellow,
				RiskLevel:     "unknown",
			}, true, info.Enabled, info.Builtin))
		}
	}

	return c.JSON(http.StatusNotFound, map[string]string{
		"error": "skill not found",
	})
}

func marketDetailCompatibilityPayload(skill *RemoteSkill, installed, enabled, builtin bool) map[string]interface{} {
	if skill == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":          skill.ID,
		"name":        skill.Name,
		"version":     skill.Version,
		"description": skill.Description,
		"author":      skill.Author,
		"category":    skill.Category,
		"tags":        skill.Tags,
		"enabled":     enabled,
		"builtin":     builtin,
		"installed":   installed,
		"skill":       skill,
	}
}

// GetSkillContent returns the content (SKILL.md/readme) of a skill
func (h *SkillHandler) GetSkillContent(c echo.Context) error {
	id, err := validatedSkillID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}
	ctx := c.Request().Context()

	// First check if skill exists in registry
	var info *skill.SkillInfo
	for _, candidate := range skillIDAliases(id) {
		if info = h.registry.GetInfo(candidate); info != nil {
			break
		}
	}

	// Try to read SKILL.md from directory
	if h.skillsDir != "" {
		for _, candidate := range skillIDAliases(id) {
			path := filepath.Join(h.skillsDir, candidate, "SKILL.md")
			if data, err := os.ReadFile(path); err == nil {
				name := candidate
				if info == nil {
					info = h.registry.GetInfo(candidate)
				}
				if info != nil {
					name = info.Manifest.Name
				}
				return c.JSON(http.StatusOK, map[string]interface{}{
					"id":      candidate,
					"name":    name,
					"content": string(data),
					"source":  "directory",
				})
			}
		}
	}

	if info == nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "skill not found",
		})
	}

	// Try to get content from database store
	if h.store != nil {
		for _, candidate := range skillIDAliases(id) {
			skill, err := h.store.GetSkill(ctx, candidate)
			if err == nil && skill != nil && skill.Readme != "" {
				return c.JSON(http.StatusOK, map[string]interface{}{
					"id":      candidate,
					"name":    info.Manifest.Name,
					"content": skill.Readme,
					"source":  "database",
				})
			}
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
			"id":      m.ID,
			"name":    m.Name,
			"content": content,
			"source":  "builtin",
		})
	}

	// No content available
	return c.JSON(http.StatusOK, map[string]interface{}{
		"id":      info.Manifest.ID,
		"name":    info.Manifest.Name,
		"content": "",
		"source":  "none",
	})
}

// EnableSkill enables a skill
func (h *SkillHandler) EnableSkill(c echo.Context) error {
	id, err := validatedSkillID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}
	for _, candidate := range skillIDAliases(id) {
		if err := h.registry.Enable(candidate); err == nil {
			return c.JSON(http.StatusOK, map[string]interface{}{
				"success": true,
				"message": "skill enabled",
			})
		}
	}
	return c.JSON(http.StatusNotFound, map[string]string{
		"error": fmt.Sprintf("skill %s not found", id),
	})
}

// DisableSkill disables a skill
func (h *SkillHandler) DisableSkill(c echo.Context) error {
	id, err := validatedSkillID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}
	for _, candidate := range skillIDAliases(id) {
		if err := h.registry.Disable(candidate); err == nil {
			return c.JSON(http.StatusOK, map[string]interface{}{
				"success": true,
				"message": "skill disabled",
			})
		}
	}
	return c.JSON(http.StatusNotFound, map[string]string{
		"error": fmt.Sprintf("skill %s not found", id),
	})
}

// UploadSkill handles skill package upload
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

	// Register as a manifest-only skill
	s := skill.NewManifestSkill(&manifest)
	if err := h.registry.Register(s, false); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "failed to register skill: " + err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "skill uploaded and registered",
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
		ID:            s.ID,
		Name:          s.Name,
		Version:       s.Version,
		Description:   s.Summary,
		Author:        s.Author,
		Category:      s.Category,
		Tags:          tags,
		SourceID:      s.SourceID,
		SourceName:    s.SourceName,
		SourceGroup:   s.SourceID,
		DownloadURL:   s.DownloadURL,
		Homepage:      s.Homepage,
		Stars:         s.Stars,
		Downloads:     s.Downloads,
		Installed:     installedIDs[s.ID],
		RiskLevel:     "unknown",
		SecurityBadge: skillmarket.BadgeYellow,
		Installable:   strings.TrimSpace(s.DownloadURL) != "" || strings.TrimSpace(s.Homepage) != "",
		InstallType:   legacyInstallTypeFromURLs(s.Homepage, s.DownloadURL),
		ArtifactKind:  skillmarket.ArtifactKindUnknown,
	}
}

// gitHubContentEntry represents a file/dir entry from GitHub Contents API.
type gitHubContentEntry struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Type        string `json:"type"` // "file" or "dir"
	DownloadURL string `json:"download_url,omitempty"`
	Content     string `json:"content,omitempty"`
	Encoding    string `json:"encoding,omitempty"`
	Size        int    `json:"size"`
	URL         string `json:"url"` // API URL for this entry
}

// gitHubURLPattern matches github.com/{owner}/{repo}/tree/{ref}/{path}
var gitHubURLPattern = regexp.MustCompile(`^https?://github\.com/([^/]+)/([^/]+)/tree/([^/]+)/(.+)$`)
var gitHubBlobURLPattern = regexp.MustCompile(`^https?://github\.com/([^/]+)/([^/]+)/blob/([^/]+)/(.+)$`)
var errHTMLSkillDocument = errors.New("downloaded content is an HTML page, not a skill file")

// parseGitHubDirURL parses a GitHub tree URL into API components.
// Returns (owner, repo, ref, path, ok).
func parseGitHubDirURL(u string) (string, string, string, string, bool) {
	m := gitHubURLPattern.FindStringSubmatch(u)
	if m == nil {
		return "", "", "", "", false
	}
	return m[1], m[2], m[3], m[4], true
}

// isGitHubDirURL returns true if the URL points to a GitHub directory (tree).
func isGitHubDirURL(u string) bool {
	_, _, _, _, ok := parseGitHubDirURL(u)
	return ok
}

// parseGitHubBlobURL parses a GitHub blob URL into components.
// Returns (owner, repo, ref, path, ok).
func parseGitHubBlobURL(u string) (string, string, string, string, bool) {
	m := gitHubBlobURLPattern.FindStringSubmatch(u)
	if m == nil {
		return "", "", "", "", false
	}
	return m[1], m[2], m[3], m[4], true
}

func rawGitHubBlobURL(owner, repo, ref, path string) string {
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repo, ref, path)
}

func normalizeSkillInstallURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if owner, repo, ref, path, ok := parseGitHubBlobURL(rawURL); ok {
		return rawGitHubBlobURL(owner, repo, ref, path)
	}
	return rawURL
}

func looksLikeHTMLDocument(body []byte, contentType string) bool {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if strings.Contains(contentType, "text/html") || strings.Contains(contentType, "application/xhtml+xml") {
		return true
	}
	snippet := strings.ToLower(strings.TrimSpace(string(body)))
	if len(snippet) > 2048 {
		snippet = snippet[:2048]
	}
	if snippet == "" {
		return false
	}
	if strings.HasPrefix(snippet, "<!doctype html") || strings.HasPrefix(snippet, "<html") {
		return true
	}
	return strings.Contains(snippet, "<head") && strings.Contains(snippet, "<body")
}

func validateDownloadedSkillContent(body []byte, contentType, sourceURL string) error {
	if looksLikeHTMLDocument(body, contentType) {
		if _, _, _, _, ok := parseGitHubBlobURL(sourceURL); ok {
			return fmt.Errorf("%w (%s). Please use the raw GitHub file URL or a GitHub tree URL", errHTMLSkillDocument, sourceURL)
		}
		return fmt.Errorf("%w (%s)", errHTMLSkillDocument, sourceURL)
	}
	return nil
}

func readInstalledSkillMarkdown(dir string) ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("installed directory does not contain SKILL.md")
		}
		return nil, err
	}
	return data, nil
}

func (h *SkillHandler) registerInstalledSkill(manifest *skill.Manifest) error {
	if manifest == nil {
		return nil
	}
	if err := h.registry.Register(NewRemoteSkillAdapter(manifest), false); err != nil {
		return err
	}
	if h.localScanner != nil {
		_ = h.localScanner.Scan()
	}
	return nil
}

// downloadGitHubDirectory downloads all files from a GitHub directory into destDir.
// It calls progressFn(downloaded, total) after each file is written.
func (h *SkillHandler) downloadGitHubDirectory(ctx context.Context, ghURL, destDir string, progressFn func(downloaded, total int)) error {
	owner, repo, ref, path, ok := parseGitHubDirURL(ghURL)
	if !ok {
		return fmt.Errorf("not a valid GitHub directory URL: %s", ghURL)
	}

	// Collect all files first (recursive)
	type fileEntry struct {
		RelPath     string // relative to the skill root
		DownloadURL string
	}
	var files []fileEntry

	var walk func(apiPath, relBase string) error
	walk = func(apiPath, relBase string) error {
		apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s?ref=%s", owner, repo, apiPath, ref)
		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "application/vnd.github.v3+json")

		resp, err := h.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("GitHub API request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("GitHub API returned %d for %s", resp.StatusCode, apiURL)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		var entries []gitHubContentEntry
		if err := json.Unmarshal(body, &entries); err != nil {
			// Might be a single file response
			var single gitHubContentEntry
			if err2 := json.Unmarshal(body, &single); err2 == nil && single.Type == "file" {
				files = append(files, fileEntry{RelPath: filepath.Join(relBase, single.Name), DownloadURL: single.DownloadURL})
				return nil
			}
			return fmt.Errorf("failed to parse GitHub contents: %w", err)
		}

		for _, e := range entries {
			switch e.Type {
			case "file":
				files = append(files, fileEntry{RelPath: filepath.Join(relBase, e.Name), DownloadURL: e.DownloadURL})
			case "dir":
				if err := walk(e.Path, filepath.Join(relBase, e.Name)); err != nil {
					return err
				}
			}
		}
		return nil
	}

	if err := walk(path, ""); err != nil {
		return err
	}

	if len(files) == 0 {
		return fmt.Errorf("no files found in GitHub directory")
	}

	// Download each file
	for i, f := range files {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		destPath := filepath.Join(destDir, f.RelPath)
		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return err
		}

		req, err := http.NewRequestWithContext(ctx, "GET", f.DownloadURL, nil)
		if err != nil {
			return err
		}
		resp, err := h.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to download %s: %w", f.RelPath, err)
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", f.RelPath, err)
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("HTTP %d downloading %s", resp.StatusCode, f.RelPath)
		}

		if err := os.WriteFile(destPath, data, 0o644); err != nil {
			return err
		}

		if progressFn != nil {
			progressFn(i+1, len(files))
		}
	}

	return nil
}

// findRemoteSkill looks up a RemoteSkill by ID from store, memory, or featured.
func (h *SkillHandler) findRemoteSkill(ctx context.Context, id string) *RemoteSkill {
	if h.store != nil {
		for _, candidate := range skillIDAliases(id) {
			sk, err := h.store.GetSkill(ctx, candidate)
			if err == nil && sk != nil {
				installedIDs := h.getInstalledSkillIDs()
				return h.skillToRemoteSkill(sk, installedIDs)
			}
		}
	}
	for _, candidate := range skillIDAliases(id) {
		h.mu.RLock()
		memSkill, exists := h.remoteSkills[candidate]
		h.mu.RUnlock()
		if exists {
			return memSkill
		}
	}
	return nil
}

// downloadSkillMD downloads SKILL.md content for a skill with retry.
func (h *SkillHandler) downloadSkillMD(ctx context.Context, id string, rs *RemoteSkill) ([]byte, error) {
	const maxRetries = 3
	var lastErr error
	sawHTMLDocument := false
	urls := make([]string, 0, 2)

	if rs.DownloadURL != "" {
		urls = append(urls, normalizeSkillInstallURL(rs.DownloadURL))
	}
	if rs.SourceID == "clawhub" || rs.SourceGroup == "clawhub" || len(urls) == 0 {
		urls = append(urls, fmt.Sprintf("https://www.clawhub.ai/api/v1/skills/%s/skill-md", id))
	}

	for attempt := 1; attempt <= maxRetries; attempt++ {
		for _, u := range urls {
			req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
			if err != nil {
				continue
			}
			resp, err := h.httpClient.Do(req)
			if err != nil {
				lastErr = err
				continue
			}
			if resp.StatusCode == http.StatusOK {
				data, err := io.ReadAll(resp.Body)
				contentType := resp.Header.Get("Content-Type")
				resp.Body.Close()
				if err == nil {
					if err := validateDownloadedSkillContent(data, contentType, u); err != nil {
						lastErr = err
						sawHTMLDocument = true
						continue
					}
					return data, nil
				}
				lastErr = err
			} else {
				resp.Body.Close()
				lastErr = fmt.Errorf("HTTP %d from %s", resp.StatusCode, u)
			}
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if attempt < maxRetries {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(1<<(attempt-1)) * time.Second):
			}
		}
	}

	if sawHTMLDocument && lastErr != nil {
		return nil, lastErr
	}

	// Fallback: generate minimal SKILL.md
	_ = lastErr // all download attempts failed, use fallback
	return []byte(fmt.Sprintf("---\nid: %s\nname: %s\ndescription: %s\nversion: %s\n---\n\n# %s\n\n%s\n",
		id, rs.Name, rs.Description, firstNonEmpty(rs.Version, "1.0.0"), rs.Name, rs.Description)), nil
}

// InstallSkill installs a skill by downloading files to the skills directory.
// Supports both single SKILL.md downloads and full GitHub directory downloads.
// Publishes progress events via the unified SSE broker.
func (h *SkillHandler) InstallSkill(c echo.Context) error {
	if h.market != nil {
		id, err := validatedSkillID(c.Param("id"))
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		result, err := h.market.Install(c.Request().Context(), skillmarket.InstallRequest{ID: id})
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, result)
	}

	id, err := validatedSkillID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	userID := getUserIDFromContext(c)

	if h.skillsDir == "" {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "skills directory not configured",
		})
	}

	// Check if already installed (directory exists with SKILL.md) or already
	// present in registry.
	if existingID, installed := h.resolveInstalledSkillID(id); installed {
		return c.JSON(http.StatusConflict, map[string]string{
			"error": fmt.Sprintf("skill already installed as %s", existingID),
		})
	}
	skillDir := filepath.Join(h.skillsDir, id)

	ctx := c.Request().Context()
	rs := h.findRemoteSkill(ctx, id)
	if rs == nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "skill not found in store",
		})
	}

	// Publish start event
	h.publishEvent(userID, "skill.install.progress", map[string]interface{}{
		"id": id, "percent": 0, "message": "Starting install...",
	})

	// Check if this is a GitHub directory skill
	ghURL := rs.Homepage
	if ghURL == "" {
		ghURL = rs.DownloadURL
	}

	if isGitHubDirURL(ghURL) {
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			h.publishEvent(userID, "skill.install.error", map[string]interface{}{"id": id, "error": err.Error()})
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("failed to create skill directory: %v", err),
			})
		}
		err := h.downloadGitHubDirectory(ctx, ghURL, skillDir, func(downloaded, total int) {
			pct := int(float64(downloaded) / float64(total) * 100)
			h.publishEvent(userID, "skill.install.progress", map[string]interface{}{
				"id": id, "percent": pct, "message": fmt.Sprintf("Downloading %d/%d", downloaded, total),
			})
		})
		if err != nil {
			os.RemoveAll(skillDir)
			h.publishEvent(userID, "skill.install.error", map[string]interface{}{"id": id, "error": err.Error()})
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("failed to download from GitHub: %v", err),
			})
		}
		skillContent, err := readInstalledSkillMarkdown(skillDir)
		if err != nil {
			_ = os.RemoveAll(skillDir)
			h.publishEvent(userID, "skill.install.error", map[string]interface{}{"id": id, "error": err.Error()})
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": err.Error(),
			})
		}
		_, manifest, err := h.parseSkillContent(string(skillContent), ghURL, rs.Name, rs.Description)
		if err != nil {
			_ = os.RemoveAll(skillDir)
			h.publishEvent(userID, "skill.install.error", map[string]interface{}{"id": id, "error": err.Error()})
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("failed to parse SKILL.md: %v", err),
			})
		}
		manifest.ID = id
		if err := h.registerInstalledSkill(manifest); err != nil {
			_ = os.RemoveAll(skillDir)
			h.publishEvent(userID, "skill.install.error", map[string]interface{}{"id": id, "error": err.Error()})
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("failed to register skill: %v", err),
			})
		}
		h.publishEvent(userID, "skill.install.complete", map[string]interface{}{"id": id})
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success": true,
			"message": fmt.Sprintf("skill %s installed from GitHub", rs.Name),
			"skill":   rs,
		})
	}

	// Non-GitHub: download SKILL.md
	h.publishEvent(userID, "skill.install.progress", map[string]interface{}{
		"id": id, "percent": 30, "message": "Downloading SKILL.md...",
	})
	skillContent, err := h.downloadSkillMD(ctx, id, rs)
	if err != nil {
		h.publishEvent(userID, "skill.install.error", map[string]interface{}{"id": id, "error": err.Error()})
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to download: %v", err),
		})
	}

	h.publishEvent(userID, "skill.install.progress", map[string]interface{}{
		"id": id, "percent": 80, "message": "Writing files...",
	})

	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		h.publishEvent(userID, "skill.install.error", map[string]interface{}{"id": id, "error": err.Error()})
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to create skill directory: %v", err),
		})
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), skillContent, 0o644); err != nil {
		os.RemoveAll(skillDir)
		h.publishEvent(userID, "skill.install.error", map[string]interface{}{"id": id, "error": err.Error()})
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to write SKILL.md: %v", err),
		})
	}
	_, manifest, err := h.parseSkillContent(string(skillContent), firstString(rs.DownloadURL, rs.Homepage), rs.Name, rs.Description)
	if err != nil {
		_ = os.RemoveAll(skillDir)
		h.publishEvent(userID, "skill.install.error", map[string]interface{}{"id": id, "error": err.Error()})
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("failed to parse SKILL.md: %v", err),
		})
	}
	manifest.ID = id
	if err := h.registerInstalledSkill(manifest); err != nil {
		_ = os.RemoveAll(skillDir)
		h.publishEvent(userID, "skill.install.error", map[string]interface{}{"id": id, "error": err.Error()})
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to register skill: %v", err),
		})
	}

	h.publishEvent(userID, "skill.install.complete", map[string]interface{}{"id": id})
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("skill %s installed to %s", rs.Name, skillDir),
		"skill":   rs,
	})
}

// UninstallSkill uninstalls a skill by removing its directory.
func (h *SkillHandler) UninstallSkill(c echo.Context) error {
	id, err := validatedSkillID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if resolvedID, ok := h.resolveInstalledSkillID(id); ok {
		id = resolvedID
	}

	if h.skillsDir == "" {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "skills directory not configured",
		})
	}

	if info := h.registry.GetInfo(id); info != nil && info.Builtin {
		return c.JSON(http.StatusForbidden, map[string]string{
			"error": "cannot uninstall builtin skill",
		})
	}

	if h.market != nil {
		if err := h.market.Uninstall(c.Request().Context(), id); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":  true,
			"skill_id": id,
		})
	}

	skillDir := filepath.Join(h.skillsDir, id)
	_, dirErr := os.Stat(skillDir)
	inRegistry := h.registry.Get(id) != nil
	if os.IsNotExist(dirErr) && !inRegistry {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "skill not installed",
		})
	}

	if dirErr == nil {
		if err := os.RemoveAll(skillDir); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("failed to remove skill: %v", err),
			})
		}
	}

	// Also unregister from in-memory registry if present
	_ = h.registry.Unregister(id)
	if h.localScanner != nil {
		_ = h.localScanner.Scan()
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

func (h *SkillHandler) resolveInstalledSkillID(id string) (string, bool) {
	for _, candidate := range skillIDAliases(id) {
		if h.registry.Get(candidate) != nil {
			return candidate, true
		}
		if h.localScanner != nil && h.localScanner.Get(candidate) != nil {
			return candidate, true
		}
		if h.skillsDir != "" {
			if _, err := os.Stat(filepath.Join(h.skillsDir, candidate, "SKILL.md")); err == nil {
				return candidate, true
			}
		}
	}
	return strings.TrimSpace(id), false
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
	if h.market != nil {
		page, _ := strconv.Atoi(c.QueryParam("page"))
		pageSize, _ := strconv.Atoi(c.QueryParam("page_size"))
		if count, _ := strconv.Atoi(c.QueryParam("count")); count > 0 && pageSize == 0 {
			pageSize = count
		}
		result, err := h.market.Search(c.Request().Context(), skillmarket.SearchQuery{
			Query:               c.QueryParam("q"),
			Category:            c.QueryParam("category"),
			Categories:          parseCSV(c.QueryParam("categories")),
			Sources:             parseCSV(c.QueryParam("sources")),
			Sort:                firstNonEmpty(c.QueryParam("sort"), c.QueryParam("sort_by")),
			Page:                page,
			PageSize:            pageSize,
			Semantic:            parseTruthy(c.QueryParam("semantic")),
			RiskBadges:          parseCSV(c.QueryParam("risk_badges")),
			InstallTypes:        parseCSV(c.QueryParam("install_types")),
			ArtifactKinds:       parseCSV(c.QueryParam("artifact_kinds")),
			Installable:         parseBoolPtr(c.QueryParam("installable")),
			Curated:             parseBoolPtr(c.QueryParam("curated")),
			OpenSourceOnly:      parseTruthy(c.QueryParam("open_source_only")),
			HasVulnerabilities:  parseBoolPtr(c.QueryParam("has_vulnerabilities")),
			HasPromptInjection:  parseBoolPtr(c.QueryParam("has_prompt_injection")),
			HasShellInjection:   parseBoolPtr(c.QueryParam("has_shell_injection")),
			HasDataExfiltration: parseBoolPtr(c.QueryParam("has_data_exfiltration")),
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		legacySkills := make([]map[string]interface{}, 0, len(result.Skills))
		for _, item := range result.Skills {
			doc := item.Skill
			legacySkills = append(legacySkills, map[string]interface{}{
				"id":                    doc.ID,
				"name":                  doc.Name,
				"version":               doc.LatestVersion,
				"summary":               doc.Description,
				"description":           doc.Description,
				"author":                doc.Author,
				"category":              doc.Category,
				"tags":                  strings.Join(doc.Tags, ","),
				"source_id":             doc.SourceID,
				"source_name":           firstString(doc.SourceName, doc.SourceGroup, doc.SourceID),
				"homepage":              doc.Homepage,
				"download_url":          firstString(doc.DownloadURL, doc.RepoURL, doc.Homepage),
				"stars":                 doc.Stars,
				"downloads":             doc.Downloads,
				"installed":             h.registry.Get(doc.ID) != nil,
				"enabled":               h.registry.Get(doc.ID) != nil,
				"score":                 item.Score,
				"updated_at":            doc.LastUpdated,
				"risk_level":            doc.RiskLevel,
				"security_badge":        doc.SecurityBadge,
				"install_type":          doc.InstallType,
				"artifact_kind":         doc.ArtifactKind,
				"installable":           doc.Installable,
				"source_group":          doc.SourceGroup,
				"curated_rank":          doc.CuratedRank,
				"curated_label":         doc.CuratedLabel,
				"has_vulnerabilities":   doc.HasVulnerabilities,
				"has_prompt_injection":  doc.HasPromptInjection,
				"has_shell_injection":   doc.HasShellInjection,
				"has_data_exfiltration": doc.HasDataExfiltration,
			})
		}
		return c.JSON(http.StatusOK, map[string]interface{}{
			"skills":      legacySkills,
			"total":       result.Total,
			"page":        result.Page,
			"page_size":   result.PageSize,
			"total_pages": result.TotalPages,
			"has_more":    result.Page < result.TotalPages,
		})
	}
	if h.store == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"skills":       []interface{}{},
			"total":        0,
			"has_more":     false,
			"initializing": true,
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
	if h.market != nil {
		filters, err := h.market.Filters(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		categories := make([]string, 0, len(filters.Categories))
		for _, category := range filters.Categories {
			categories = append(categories, category.Value)
		}
		return c.JSON(http.StatusOK, categories)
	}
	if h.store == nil {
		return c.JSON(http.StatusOK, []string{})
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
		return c.JSON(http.StatusOK, map[string]interface{}{
			"total_skills": 0,
			"installed":    0,
			"initializing": true,
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
	if h.market != nil {
		limit := 20
		if l := c.QueryParam("limit"); l != "" {
			if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
				limit = v
			}
		}
		search, err := h.market.Search(c.Request().Context(), skillmarket.SearchQuery{
			Sort:     "most_used",
			Page:     1,
			PageSize: limit,
			Sources:  parseCSV(c.QueryParam("sources")),
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		items := make([]*RemoteSkill, 0, len(search.Skills))
		for _, item := range search.Skills {
			items = append(items, h.remoteSkillFromDocument(item.Skill))
		}
		return c.JSON(http.StatusOK, items)
	}
	if h.store == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"skills":       []interface{}{},
			"initializing": true,
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
	if h.market != nil {
		search, err := h.market.Search(c.Request().Context(), skillmarket.SearchQuery{
			Sort:     "newest",
			Page:     1,
			PageSize: 20,
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		items := make([]*RemoteSkill, 0, len(search.Skills))
		for _, item := range search.Skills {
			items = append(items, h.remoteSkillFromDocument(item.Skill))
		}
		return c.JSON(http.StatusOK, items)
	}
	if h.store == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"skills":       []interface{}{},
			"initializing": true,
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
	if h.market != nil {
		limit, _ := strconv.Atoi(c.QueryParam("limit"))
		skills, err := h.market.Featured(c.Request().Context(), c.QueryParam("category"), c.QueryParam("source"), limit)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		result := make([]*RemoteSkill, 0, len(skills))
		for _, skill := range skills {
			result = append(result, h.remoteSkillFromDocument(skill))
		}
		return c.JSON(http.StatusOK, result)
	}
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
	id, err := validatedSkillID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
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

// InstallFromURL installs a skill from a URL. Supports both direct SKILL.md URLs
// and GitHub directory URLs (downloads all files in the directory).
func (h *SkillHandler) InstallFromURL(c echo.Context) error {
	var req InstallFromURLRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	installURL := normalizeSkillInstallURL(req.URL)
	if installURL == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "URL is required"})
	}
	if h.skillsDir == "" {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "skills directory not configured"})
	}
	if err := os.MkdirAll(h.skillsDir, 0o755); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("mkdir skills dir: %v", err)})
	}

	ctx := c.Request().Context()

	// Check if this is a GitHub directory URL
	if isGitHubDirURL(installURL) {
		tempDir, err := os.MkdirTemp(h.skillsDir, ".skill-install-*")
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("mkdir temp: %v", err)})
		}
		cleanupDir := tempDir
		defer func() {
			if cleanupDir != "" {
				_ = os.RemoveAll(cleanupDir)
			}
		}()

		if err := h.downloadGitHubDirectory(ctx, installURL, tempDir, nil); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("GitHub download failed: %v", err)})
		}

		body, err := readInstalledSkillMarkdown(tempDir)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		if err := validateDownloadedSkillContent(body, "", installURL); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		skillID, manifest, err := h.parseSkillContent(string(body), installURL, req.Name, req.Description)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("failed to parse skill: %v", err)})
		}

		if existingID, installed := h.resolveInstalledSkillID(skillID); installed {
			return c.JSON(http.StatusConflict, map[string]string{"error": fmt.Sprintf("skill already installed as %s", existingID)})
		}

		skillDir := filepath.Join(h.skillsDir, skillID)
		if _, err := os.Stat(skillDir); err == nil {
			return c.JSON(http.StatusConflict, map[string]string{"error": "skill already installed"})
		}
		if err := os.Rename(tempDir, skillDir); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("move: %v", err)})
		}
		cleanupDir = ""
		if err := h.registerInstalledSkill(manifest); err != nil {
			_ = os.RemoveAll(skillDir)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("register: %v", err)})
		}

		return c.JSON(http.StatusOK, map[string]interface{}{
			"success": true,
			"skill":   map[string]interface{}{"id": skillID, "path": skillDir},
		})
	}

	// Non-GitHub: download single file
	const maxRetries = 3
	var body []byte
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		httpReq, err := http.NewRequestWithContext(ctx, "GET", installURL, nil)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid URL: %v", err)})
		}
		resp, err := h.httpClient.Do(httpReq)
		if err != nil {
			lastErr = err
		} else if resp.StatusCode == http.StatusOK {
			body, lastErr = io.ReadAll(resp.Body)
			contentType := resp.Header.Get("Content-Type")
			resp.Body.Close()
			if lastErr == nil {
				if err := validateDownloadedSkillContent(body, contentType, installURL); err != nil {
					return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
				}
				break
			}
		} else {
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		if ctx.Err() != nil {
			return c.JSON(http.StatusRequestTimeout, map[string]string{"error": "request cancelled"})
		}
		if attempt < maxRetries {
			select {
			case <-ctx.Done():
				return c.JSON(http.StatusRequestTimeout, map[string]string{"error": "request cancelled"})
			case <-time.After(time.Duration(1<<(attempt-1)) * time.Second):
			}
		}
	}
	if lastErr != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": fmt.Sprintf("failed after %d attempts: %v", maxRetries, lastErr)})
	}

	skillID, manifest, err := h.parseSkillContent(string(body), installURL, req.Name, req.Description)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("failed to parse skill: %v", err)})
	}

	if existingID, installed := h.resolveInstalledSkillID(skillID); installed {
		return c.JSON(http.StatusConflict, map[string]string{"error": fmt.Sprintf("skill already installed as %s", existingID)})
	}

	skillDir := filepath.Join(h.skillsDir, skillID)
	if _, err := os.Stat(filepath.Join(skillDir, "SKILL.md")); err == nil {
		return c.JSON(http.StatusConflict, map[string]string{"error": "skill already installed"})
	}

	tempDir, err := os.MkdirTemp(h.skillsDir, ".skill-install-*")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("mkdir temp: %v", err)})
	}
	cleanupDir := tempDir
	defer func() {
		if cleanupDir != "" {
			_ = os.RemoveAll(cleanupDir)
		}
	}()
	if err := os.WriteFile(filepath.Join(tempDir, "SKILL.md"), body, 0o644); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("write: %v", err)})
	}
	if err := os.Rename(tempDir, skillDir); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("move: %v", err)})
	}
	cleanupDir = ""
	if err := h.registerInstalledSkill(manifest); err != nil {
		_ = os.RemoveAll(skillDir)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("register: %v", err)})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"skill":   map[string]interface{}{"id": skillID, "path": skillDir},
	})
}

// parseSkillContent parses SKILL.md content and returns skill ID and manifest
func (h *SkillHandler) parseSkillContent(content, sourceURL, overrideName, overrideDesc string) (string, *skill.Manifest, error) {
	if err := validateDownloadedSkillContent([]byte(content), "", sourceURL); err != nil {
		return "", nil, err
	}

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
				value := strings.Trim(strings.TrimSpace(line[colonIdx+1:]), `"'`)

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
	validatedID, err := normalizedSkillID(manifest.ID)
	if err != nil {
		return "", nil, err
	}
	manifest.ID = validatedID

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
			ID:            s.ID,
			Name:          s.Name,
			Version:       s.Version,
			Description:   s.Description,
			Author:        s.Author,
			Category:      s.Category,
			Tags:          s.Tags,
			SourceID:      "featured",
			SourceName:    "Featured",
			SourceGroup:   "featured",
			Homepage:      s.Homepage,
			DownloadURL:   s.SourceURL,
			Stars:         s.Stars,
			Installed:     h.registry.Get(s.ID) != nil,
			RiskLevel:     "unknown",
			SecurityBadge: skillmarket.BadgeYellow,
			Installable:   strings.TrimSpace(s.SourceURL) != "" || strings.TrimSpace(s.Homepage) != "",
			InstallType:   legacyInstallTypeFromURLs(s.Homepage, s.SourceURL),
			ArtifactKind:  skillmarket.ArtifactKindUnknown,
		})
	}
	return result
}

func legacyInstallTypeFromURLs(homepage, downloadURL string) string {
	if isGitHubDirURL(firstString(homepage, downloadURL)) {
		return skillmarket.InstallTypeGitRepo
	}
	if strings.TrimSpace(downloadURL) != "" {
		return skillmarket.InstallTypeRawSkill
	}
	return skillmarket.InstallTypeManualExternal
}
