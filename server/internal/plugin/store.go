package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/logger"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// StoreSource represents a plugin source
type StoreSource string

const (
	SourceMoltbot   StoreSource = "moltbot"
	SourceClawdHub  StoreSource = "clawdhub"
	SourceGitHub    StoreSource = "github"
	SourceCustom    StoreSource = "custom"
)

// StorePlugin represents a plugin available in the store
type StorePlugin struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Version     string            `json:"version"`
	Author      string            `json:"author"`
	Source      StoreSource       `json:"source"`
	RepoURL     string            `json:"repo_url"`
	DownloadURL string            `json:"download_url"`
	Manifest    *Manifest         `json:"manifest,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Downloads   int               `json:"downloads"`
	Rating      float64           `json:"rating"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Installed   bool              `json:"installed"`
}

// StoreConfig holds plugin store configuration
type StoreConfig struct {
	CacheDir       string        `json:"cache_dir"`
	CacheDuration  time.Duration `json:"cache_duration"`
	MoltbotRepo    string        `json:"moltbot_repo"`
	MoltbotBranch  string        `json:"moltbot_branch"`
	CustomSources  []string      `json:"custom_sources"`
	UseJsdelivr    bool          `json:"use_jsdelivr"`    // If true, try jsdelivr CDN first for faster access
	JsdelivrMirror string        `json:"jsdelivr_mirror"` // Custom jsdelivr mirror (e.g., "fastly.jsdelivr.net", "gcore.jsdelivr.net")
}

// DefaultStoreConfig returns default store configuration
func DefaultStoreConfig() *StoreConfig {
	return &StoreConfig{
		CacheDir:       "./data/plugin-cache",
		CacheDuration:  1 * time.Hour,
		MoltbotRepo:    "moltbot/moltbot",
		MoltbotBranch:  "main",
		UseJsdelivr:    true,
		JsdelivrMirror: "",
	}
}

// Store manages the plugin marketplace
type Store struct {
	config       *StoreConfig
	registry     *Registry
	plugins      map[string]*StorePlugin
	cache        map[string][]byte
	cacheTimes   map[string]time.Time
	mu           sync.RWMutex
	httpClient   *http.Client
	httpClientV4 *http.Client // IPv4 only client
	httpClientV6 *http.Client // IPv6 only client
}

// createHTTPClient creates an HTTP client with optional network restriction
func createHTTPClient(network string) *http.Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
			dialer := &net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}
			if network != "" {
				return dialer.DialContext(ctx, network, addr)
			}
			return dialer.DialContext(ctx, "tcp", addr)
		},
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}
}

// NewStore creates a new plugin store
func NewStore(config *StoreConfig, registry *Registry) *Store {
	if config == nil {
		config = DefaultStoreConfig()
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(config.CacheDir, 0755); err != nil {
		logger.Warn().Err(err).Msg("Failed to create plugin cache directory")
	}

	return &Store{
		config:       config,
		registry:     registry,
		plugins:      make(map[string]*StorePlugin),
		cache:        make(map[string][]byte),
		cacheTimes:   make(map[string]time.Time),
		httpClient:   createHTTPClient(""),     // Default: try both IPv4 and IPv6
		httpClientV4: createHTTPClient("tcp4"), // IPv4 only
		httpClientV6: createHTTPClient("tcp6"), // IPv6 only
	}
}

// ListPlugins returns all available plugins from all sources
func (s *Store) ListPlugins(ctx context.Context) ([]*StorePlugin, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Refresh from sources
	if err := s.refreshMoltbotPlugins(ctx); err != nil {
		logger.Warn().Err(err).Msg("Failed to refresh moltbot plugins")
	}

	// Mark installed plugins
	installedPlugins := s.registry.ListPlugins()
	installedIDs := make(map[string]bool)
	for _, p := range installedPlugins {
		installedIDs[p.Manifest.ID] = true
	}

	result := make([]*StorePlugin, 0, len(s.plugins))
	for _, p := range s.plugins {
		p.Installed = installedIDs[p.ID]
		result = append(result, p)
	}

	return result, nil
}

// GetPlugin returns a specific plugin from the store
func (s *Store) GetPlugin(ctx context.Context, id string) (*StorePlugin, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	plugin, ok := s.plugins[id]
	if !ok {
		return nil, fmt.Errorf("plugin %s not found in store", id)
	}

	return plugin, nil
}

// InstallPlugin downloads and installs a plugin
func (s *Store) InstallPlugin(ctx context.Context, id string) error {
	plugin, err := s.GetPlugin(ctx, id)
	if err != nil {
		return err
	}

	// Download plugin files
	pluginDir := filepath.Join(s.config.CacheDir, "installed", id)
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugin directory: %w", err)
	}

	// Download manifest
	manifestURL := s.getManifestURL(plugin)
	manifestData, err := s.fetchWithFallback(ctx, manifestURL)
	if err != nil {
		return fmt.Errorf("failed to download manifest: %w", err)
	}

	manifestPath := filepath.Join(pluginDir, "clawdbot.plugin.json")
	if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	// Download index.ts/index.js
	indexURL := s.getIndexURL(plugin)
	indexData, err := s.fetchWithFallback(ctx, indexURL)
	if err != nil {
		return fmt.Errorf("failed to download index: %w", err)
	}

	indexPath := filepath.Join(pluginDir, "index.ts")
	if err := os.WriteFile(indexPath, indexData, 0644); err != nil {
		return fmt.Errorf("failed to write index: %w", err)
	}

	// Download package.json
	packageURL := s.getPackageURL(plugin)
	packageData, err := s.fetchWithFallback(ctx, packageURL)
	if err != nil {
		logger.Warn().Err(err).Str("plugin", id).Msg("Failed to download package.json, continuing without it")
	} else {
		packagePath := filepath.Join(pluginDir, "package.json")
		if err := os.WriteFile(packagePath, packageData, 0644); err != nil {
			logger.Warn().Err(err).Msg("Failed to write package.json")
		}
	}

	// Load the plugin into registry
	if err := s.registry.loadPlugin(ctx, pluginDir, OriginWorkspace); err != nil {
		return fmt.Errorf("failed to load plugin: %w", err)
	}

	logger.Info().Str("plugin", id).Msg("Plugin installed successfully")
	return nil
}

// UninstallPlugin removes an installed plugin
func (s *Store) UninstallPlugin(ctx context.Context, id string) error {
	// Stop the plugin if running
	if err := s.registry.StopPlugin(ctx, id); err != nil {
		logger.Warn().Err(err).Str("plugin", id).Msg("Failed to stop plugin before uninstall")
	}

	// Remove from registry
	s.registry.UnregisterPlugin(id)

	// Remove plugin directory
	pluginDir := filepath.Join(s.config.CacheDir, "installed", id)
	if err := os.RemoveAll(pluginDir); err != nil {
		return fmt.Errorf("failed to remove plugin directory: %w", err)
	}

	logger.Info().Str("plugin", id).Msg("Plugin uninstalled successfully")
	return nil
}

// SearchPlugins searches for plugins by query
func (s *Store) SearchPlugins(ctx context.Context, query string) ([]*StorePlugin, error) {
	plugins, err := s.ListPlugins(ctx)
	if err != nil {
		return nil, err
	}

	query = strings.ToLower(query)
	result := make([]*StorePlugin, 0)

	for _, p := range plugins {
		if strings.Contains(strings.ToLower(p.Name), query) ||
			strings.Contains(strings.ToLower(p.Description), query) ||
			strings.Contains(strings.ToLower(p.ID), query) {
			result = append(result, p)
		}

		// Check tags
		for _, tag := range p.Tags {
			if strings.Contains(strings.ToLower(tag), query) {
				result = append(result, p)
				break
			}
		}
	}

	return result, nil
}

// refreshMoltbotPlugins fetches plugin list from moltbot repository
func (s *Store) refreshMoltbotPlugins(ctx context.Context) error {
	// Check cache
	cacheKey := "moltbot_extensions"
	if data, ok := s.cache[cacheKey]; ok {
		if timeutil.SinceTime(s.cacheTimes[cacheKey]) < s.config.CacheDuration {
			return s.parseMoltbotExtensions(data)
		}
	}

	// Fetch extensions list from GitHub API
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/contents/extensions?ref=%s",
		s.config.MoltbotRepo, s.config.MoltbotBranch)

	data, err := s.fetchWithFallback(ctx, apiURL)
	if err != nil {
		return err
	}

	s.cache[cacheKey] = data
	s.cacheTimes[cacheKey] = timeutil.NowTime()

	return s.parseMoltbotExtensions(data)
}

// parseMoltbotExtensions parses the GitHub API response for extensions
func (s *Store) parseMoltbotExtensions(data []byte) error {
	var entries []struct {
		Name string `json:"name"`
		Type string `json:"type"`
		Path string `json:"path"`
	}

	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("failed to parse extensions list: %w", err)
	}

	for _, entry := range entries {
		if entry.Type != "dir" {
			continue
		}

		pluginID := entry.Name
		s.plugins[pluginID] = &StorePlugin{
			ID:          pluginID,
			Name:        formatPluginName(pluginID),
			Description: fmt.Sprintf("%s plugin", formatPluginName(pluginID)),
			Version:     "latest",
			Author:      "moltbot",
			Source:      SourceMoltbot,
			RepoURL:     fmt.Sprintf("https://github.com/%s/tree/%s/extensions/%s", s.config.MoltbotRepo, s.config.MoltbotBranch, pluginID),
			Tags:        inferPluginTags(pluginID),
			UpdatedAt:   timeutil.NowTime(),
		}
	}

	return nil
}

// fetchWithFallback fetches URL with jsdelivr fallback or primary
func (s *Store) fetchWithFallback(ctx context.Context, url string) ([]byte, error) {
	// If UseJsdelivr is enabled and URL is from GitHub, try jsdelivr first
	if s.config.UseJsdelivr && strings.Contains(url, "github") {
		jsdelivrURL := s.convertToJsdelivr(url)
		if jsdelivrURL != "" {
			// Try jsdelivr mirror first if configured
			if s.config.JsdelivrMirror != "" {
				mirrorURL := strings.Replace(jsdelivrURL, "cdn.jsdelivr.net", s.config.JsdelivrMirror, 1)
				data, err := s.fetch(ctx, mirrorURL)
				if err == nil {
					return data, nil
				}
				logger.Debug().Err(err).Str("url", mirrorURL).Msg("jsdelivr mirror fetch failed")
			}

			// Try jsdelivr CDN
			data, err := s.fetch(ctx, jsdelivrURL)
			if err == nil {
				return data, nil
			}
			logger.Debug().Err(err).Str("url", jsdelivrURL).Msg("jsdelivr fetch failed")
		}
	}

	// Try primary URL (raw.githubusercontent.com or other)
	data, err := s.fetch(ctx, url)
	if err == nil {
		return data, nil
	}

	logger.Debug().Err(err).Str("url", url).Msg("Primary fetch failed")

	// If we haven't tried jsdelivr yet (UseJsdelivr was false), try it as fallback
	if !s.config.UseJsdelivr && strings.Contains(url, "github") {
		jsdelivrURL := s.convertToJsdelivr(url)
		if jsdelivrURL != "" {
			data, err = s.fetch(ctx, jsdelivrURL)
			if err == nil {
				return data, nil
			}
			logger.Debug().Err(err).Str("url", jsdelivrURL).Msg("jsdelivr fallback fetch failed")
		}
	}

	return nil, fmt.Errorf("all fetch attempts failed for %s", url)
}

// fetch performs HTTP GET request with IPv4/IPv6 fallback
func (s *Store) fetch(ctx context.Context, url string) ([]byte, error) {
	// Try with default client first (system preference)
	data, err := s.doFetch(ctx, url, s.httpClient)
	if err == nil {
		return data, nil
	}

	// If failed, try IPv4 explicitly
	data, err4 := s.doFetch(ctx, url, s.httpClientV4)
	if err4 == nil {
		return data, nil
	}
	logger.Debug().Err(err4).Str("url", url).Msg("IPv4 fetch failed")

	// Try IPv6 explicitly
	data, err6 := s.doFetch(ctx, url, s.httpClientV6)
	if err6 == nil {
		return data, nil
	}
	logger.Debug().Err(err6).Str("url", url).Msg("IPv6 fetch failed")

	return nil, fmt.Errorf("fetch failed (default: %v, ipv4: %v, ipv6: %v)", err, err4, err6)
}

// doFetch performs the actual HTTP GET request with a specific client
func (s *Store) doFetch(ctx context.Context, url string, client *http.Client) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "ZimaOS-Blue/1.0")
	req.Header.Set("Accept", "application/json, text/plain, */*")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return io.ReadAll(resp.Body)
}

// convertToJsdelivr converts GitHub URL to jsdelivr CDN URL
func (s *Store) convertToJsdelivr(url string) string {
	// Handle raw.githubusercontent.com URLs
	// https://raw.githubusercontent.com/owner/repo/branch/path
	// -> https://cdn.jsdelivr.net/gh/owner/repo@branch/path
	if strings.Contains(url, "raw.githubusercontent.com") {
		url = strings.Replace(url, "https://raw.githubusercontent.com/", "", 1)
		parts := strings.SplitN(url, "/", 3)
		if len(parts) >= 3 {
			owner := parts[0]
			repo := parts[1]
			rest := parts[2]
			// rest is "branch/path"
			branchPath := strings.SplitN(rest, "/", 2)
			if len(branchPath) >= 2 {
				branch := branchPath[0]
				path := branchPath[1]
				return fmt.Sprintf("https://cdn.jsdelivr.net/gh/%s/%s@%s/%s", owner, repo, branch, path)
			}
		}
	}

	// Handle api.github.com URLs - these can't be converted to jsdelivr
	if strings.Contains(url, "api.github.com") {
		return ""
	}

	// Handle github.com blob/tree URLs
	// https://github.com/owner/repo/blob/branch/path
	// -> https://cdn.jsdelivr.net/gh/owner/repo@branch/path
	if strings.Contains(url, "github.com") && (strings.Contains(url, "/blob/") || strings.Contains(url, "/tree/")) {
		url = strings.Replace(url, "https://github.com/", "", 1)
		url = strings.Replace(url, "/blob/", "/", 1)
		url = strings.Replace(url, "/tree/", "/", 1)
		parts := strings.SplitN(url, "/", 3)
		if len(parts) >= 3 {
			owner := parts[0]
			repo := parts[1]
			rest := parts[2]
			branchPath := strings.SplitN(rest, "/", 2)
			if len(branchPath) >= 2 {
				branch := branchPath[0]
				path := branchPath[1]
				return fmt.Sprintf("https://cdn.jsdelivr.net/gh/%s/%s@%s/%s", owner, repo, branch, path)
			}
		}
	}

	return ""
}

// getManifestURL returns the URL for plugin manifest
func (s *Store) getManifestURL(plugin *StorePlugin) string {
	switch plugin.Source {
	case SourceMoltbot:
		return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/extensions/%s/clawdbot.plugin.json",
			s.config.MoltbotRepo, s.config.MoltbotBranch, plugin.ID)
	default:
		return plugin.DownloadURL + "/clawdbot.plugin.json"
	}
}

// getIndexURL returns the URL for plugin index file
func (s *Store) getIndexURL(plugin *StorePlugin) string {
	switch plugin.Source {
	case SourceMoltbot:
		return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/extensions/%s/index.ts",
			s.config.MoltbotRepo, s.config.MoltbotBranch, plugin.ID)
	default:
		return plugin.DownloadURL + "/index.ts"
	}
}

// getPackageURL returns the URL for plugin package.json
func (s *Store) getPackageURL(plugin *StorePlugin) string {
	switch plugin.Source {
	case SourceMoltbot:
		return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/extensions/%s/package.json",
			s.config.MoltbotRepo, s.config.MoltbotBranch, plugin.ID)
	default:
		return plugin.DownloadURL + "/package.json"
	}
}

// inferPluginTags infers tags from plugin ID
func inferPluginTags(id string) []string {
	tags := []string{}

	// Channel plugins
	channels := []string{"telegram", "discord", "slack", "matrix", "whatsapp", "signal",
		"line", "msteams", "googlechat", "imessage", "bluebubbles", "twitch", "zalo", "nostr", "tlon"}
	for _, ch := range channels {
		if strings.Contains(id, ch) {
			tags = append(tags, "channel", ch)
			break
		}
	}

	// Auth plugins
	if strings.Contains(id, "auth") {
		tags = append(tags, "auth")
	}

	// Memory plugins
	if strings.Contains(id, "memory") {
		tags = append(tags, "memory")
	}

	// LLM plugins
	if strings.Contains(id, "llm") || strings.Contains(id, "copilot") {
		tags = append(tags, "llm")
	}

	// Voice plugins
	if strings.Contains(id, "voice") {
		tags = append(tags, "voice")
	}

	// Diagnostics plugins
	if strings.Contains(id, "diagnostics") || strings.Contains(id, "otel") {
		tags = append(tags, "diagnostics", "monitoring")
	}

	return tags
}
