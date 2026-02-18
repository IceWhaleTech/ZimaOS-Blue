// Package ide provides discovery and integration with local IDE tools
// that have LLM provider capabilities (Antigravity, Cursor, Windsurf, etc.)
package ide

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// IDEType represents the type of IDE
type IDEType string

const (
	IDETypeAntigravity IDEType = "antigravity"
	IDETypeCursor      IDEType = "cursor"
	IDETypeWindsurf    IDEType = "windsurf"
	IDETypeClaudeCode  IDEType = "claude-code"
	IDETypeQoder       IDEType = "qoder"
	IDETypeTRAE        IDEType = "trae"
	IDETypeKiro        IDEType = "kiro"
	IDETypeCopilot     IDEType = "copilot"
)

// IDEInfo contains information about a discovered IDE
type IDEInfo struct {
	Type        IDEType   `json:"type"`
	Name        string    `json:"name"`
	Version     string    `json:"version,omitempty"`
	ConfigPath  string    `json:"config_path"`
	ProxyURL    string    `json:"proxy_url,omitempty"`
	APIKey      string    `json:"api_key,omitempty"` // Masked key for display
	Connected   bool      `json:"connected"`
	Models      []string  `json:"models,omitempty"`
	LastChecked time.Time `json:"last_checked"`
	Error       string    `json:"error,omitempty"`
	// Usage information
	Usage *IDEUsageInfo `json:"usage,omitempty"`
}

// IDEUsageInfo contains usage information for an IDE
type IDEUsageInfo struct {
	TotalTokens      int64     `json:"total_tokens,omitempty"`
	InputTokens      int64     `json:"input_tokens,omitempty"`
	OutputTokens     int64     `json:"output_tokens,omitempty"`
	TotalRequests    int64     `json:"total_requests,omitempty"`
	EstimatedCost    float64   `json:"estimated_cost,omitempty"`
	RemainingCredits float64   `json:"remaining_credits,omitempty"`
	UsageLimit       float64   `json:"usage_limit,omitempty"`
	UsagePeriod      string    `json:"usage_period,omitempty"` // "daily", "monthly", etc.
	LastUpdated      time.Time `json:"last_updated,omitempty"`
}

// Discovery handles IDE provider discovery
type Discovery struct {
	client      *http.Client
	discovered  map[IDEType]*IDEInfo
	mu          sync.RWMutex
	scanTimeout time.Duration
}

// NewDiscovery creates a new IDE Discovery instance
func NewDiscovery(timeout time.Duration) *Discovery {
	return &Discovery{
		client: &http.Client{
			Timeout: timeout,
		},
		discovered:  make(map[IDEType]*IDEInfo),
		scanTimeout: timeout,
	}
}

// ScanResult contains the result of scanning for an IDE
type ScanResult struct {
	IDEType       IDEType  `json:"ide_type"`
	IDEName       string   `json:"ide_name"`
	Found         bool     `json:"found"`
	ConfigPath    string   `json:"config_path,omitempty"`
	SearchedPaths []string `json:"searched_paths,omitempty"`
	Error         string   `json:"error,omitempty"`
}

// Scan scans for all supported IDEs
func (d *Discovery) Scan(ctx context.Context) ([]*IDEInfo, error) {
	var results []*IDEInfo
	var wg sync.WaitGroup
	var mu sync.Mutex

	ideTypes := []IDEType{
		IDETypeAntigravity,
		IDETypeCursor,
		IDETypeWindsurf,
		IDETypeClaudeCode,
		IDETypeQoder,
		IDETypeTRAE,
		IDETypeKiro,
		IDETypeCopilot,
	}

	for _, ideType := range ideTypes {
		wg.Add(1)
		go func(t IDEType) {
			defer wg.Done()

			info, err := d.scanIDE(ctx, t)
			if err != nil {
				return
			}

			mu.Lock()
			results = append(results, info)
			mu.Unlock()

			d.mu.Lock()
			d.discovered[t] = info
			d.mu.Unlock()
		}(ideType)
	}

	wg.Wait()
	return results, nil
}

// ScanAll scans for all supported IDEs and returns results for all (including not found)
func (d *Discovery) ScanAll(ctx context.Context) ([]*ScanResult, error) {
	var results []*ScanResult
	var wg sync.WaitGroup
	var mu sync.Mutex

	ideTypes := []IDEType{
		IDETypeAntigravity,
		IDETypeCursor,
		IDETypeWindsurf,
		IDETypeClaudeCode,
		IDETypeQoder,
		IDETypeTRAE,
		IDETypeKiro,
		IDETypeCopilot,
	}

	for _, ideType := range ideTypes {
		wg.Add(1)
		go func(t IDEType) {
			defer wg.Done()

			result := &ScanResult{
				IDEType: t,
				IDEName: getIDEName(t),
			}

			// Get searched paths
			result.SearchedPaths = getSearchedPaths(t)

			info, err := d.scanIDE(ctx, t)
			if err != nil {
				result.Found = false
				result.Error = err.Error()
			} else {
				result.Found = true
				result.ConfigPath = info.ConfigPath

				d.mu.Lock()
				d.discovered[t] = info
				d.mu.Unlock()
			}

			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}(ideType)
	}

	wg.Wait()
	return results, nil
}

// getSearchedPaths returns the paths that will be searched for an IDE
func getSearchedPaths(ideType IDEType) []string {
	var paths []string

	switch ideType {
	case IDETypeAntigravity:
		paths = getAntigravityPaths()
	case IDETypeCursor:
		paths = getCursorPaths()
	case IDETypeWindsurf:
		paths = getWindsurfPaths()
	case IDETypeClaudeCode:
		paths = getClaudeCodePaths()
	case IDETypeQoder:
		paths = getQoderPaths()
	case IDETypeTRAE:
		paths = getTRAEPaths()
	case IDETypeKiro:
		paths = getKiroPaths()
	case IDETypeCopilot:
		paths = getCopilotPaths()
	}

	// Expand paths for display
	expanded := make([]string, 0, len(paths))
	for _, p := range paths {
		expanded = append(expanded, expandPath(p))
	}
	return expanded
}

// GetDiscovered returns all discovered IDEs
func (d *Discovery) GetDiscovered() []*IDEInfo {
	d.mu.RLock()
	defer d.mu.RUnlock()

	results := make([]*IDEInfo, 0, len(d.discovered))
	for _, info := range d.discovered {
		results = append(results, info)
	}
	return results
}

// GetIDE returns information about a specific IDE
func (d *Discovery) GetIDE(ideType IDEType) (*IDEInfo, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	info, exists := d.discovered[ideType]
	return info, exists
}

// scanIDE scans for a specific IDE
func (d *Discovery) scanIDE(ctx context.Context, ideType IDEType) (*IDEInfo, error) {
	info := &IDEInfo{
		Type:        ideType,
		Name:        getIDEName(ideType),
		LastChecked: time.Now(),
	}

	// Find config path
	configPath := findConfigPath(ideType)
	if configPath == "" {
		return nil, fmt.Errorf("config path not found for %s", ideType)
	}
	info.ConfigPath = configPath

	// Check if config exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config not found at %s", configPath)
	}

	// Try to find proxy URL
	proxyURL := findProxyURL(ideType, configPath)
	if proxyURL != "" {
		info.ProxyURL = proxyURL

		// Try to connect and get models
		models, err := d.fetchModels(ctx, proxyURL)
		if err == nil {
			info.Connected = true
			info.Models = models
		} else {
			info.Error = err.Error()
		}
	}

	return info, nil
}

// Connect attempts to connect to an IDE's proxy
func (d *Discovery) Connect(ctx context.Context, ideType IDEType) (*IDEInfo, error) {
	info, exists := d.GetIDE(ideType)
	if !exists {
		// Try to scan first
		scanned, err := d.scanIDE(ctx, ideType)
		if err != nil {
			return nil, err
		}
		info = scanned
	}

	if info.ProxyURL == "" {
		return nil, fmt.Errorf("no proxy URL found for %s", ideType)
	}

	// Try to fetch models
	models, err := d.fetchModels(ctx, info.ProxyURL)
	if err != nil {
		info.Connected = false
		info.Error = err.Error()
		return info, err
	}

	info.Connected = true
	info.Models = models
	info.Error = ""
	info.LastChecked = time.Now()

	d.mu.Lock()
	d.discovered[ideType] = info
	d.mu.Unlock()

	return info, nil
}

// fetchModels fetches available models from an IDE proxy
func (d *Discovery) fetchModels(ctx context.Context, proxyURL string) ([]string, error) {
	url := strings.TrimSuffix(proxyURL, "/") + "/models"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	models := make([]string, 0, len(result.Data))
	for _, m := range result.Data {
		models = append(models, m.ID)
	}

	return models, nil
}

// getIDEName returns the display name for an IDE type
func getIDEName(ideType IDEType) string {
	switch ideType {
	case IDETypeAntigravity:
		return "Antigravity (Google)"
	case IDETypeCursor:
		return "Cursor"
	case IDETypeWindsurf:
		return "Windsurf"
	case IDETypeClaudeCode:
		return "Claude Code"
	case IDETypeQoder:
		return "Qoder"
	case IDETypeTRAE:
		return "TRAE"
	case IDETypeKiro:
		return "Kiro"
	case IDETypeCopilot:
		return "GitHub Copilot"
	default:
		return string(ideType)
	}
}

// findConfigPath finds the config path for an IDE
func findConfigPath(ideType IDEType) string {
	var paths []string

	switch ideType {
	case IDETypeAntigravity:
		paths = getAntigravityPaths()
	case IDETypeCursor:
		paths = getCursorPaths()
	case IDETypeWindsurf:
		paths = getWindsurfPaths()
	case IDETypeClaudeCode:
		paths = getClaudeCodePaths()
	case IDETypeQoder:
		paths = getQoderPaths()
	case IDETypeTRAE:
		paths = getTRAEPaths()
	case IDETypeKiro:
		paths = getKiroPaths()
	case IDETypeCopilot:
		paths = getCopilotPaths()
	}

	for _, path := range paths {
		expanded := expandPath(path)
		if _, err := os.Stat(expanded); err == nil {
			return expanded
		}
	}

	return ""
}

// findProxyURL finds the proxy URL for an IDE
func findProxyURL(ideType IDEType, configPath string) string {
	// Try to read config file for proxy settings
	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return ""
	}

	// Look for proxy URL in config
	if proxyURL, ok := config["proxy_url"].(string); ok {
		return proxyURL
	}

	// Look for base_url or api_url
	if baseURL, ok := config["base_url"].(string); ok {
		return baseURL
	}
	if apiURL, ok := config["api_url"].(string); ok {
		return apiURL
	}

	// Try common local ports based on IDE type
	switch ideType {
	case IDETypeAntigravity:
		return tryPorts([]int{9876, 9877, 9878})
	case IDETypeCursor:
		return tryPorts([]int{9999, 9998, 9997})
	case IDETypeWindsurf:
		return tryPorts([]int{9800, 9801, 9802})
	case IDETypeClaudeCode:
		return tryPorts([]int{9000, 9001, 9002, 80})
	case IDETypeQoder:
		return tryPorts([]int{9500, 9501, 9502})
	case IDETypeTRAE:
		return tryPorts([]int{9600, 9601, 9602})
	case IDETypeKiro:
		return tryPorts([]int{65432, 65433})
	case IDETypeCopilot:
		return tryPorts([]int{9700, 9701})
	}

	return ""
}

// tryPorts tries to connect to common local ports
func tryPorts(ports []int) string {
	client := &http.Client{Timeout: 2 * time.Second}

	for _, port := range ports {
		url := fmt.Sprintf("http://127.0.0.1:%d/v1", port)
		resp, err := client.Get(url + "/models")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusUnauthorized {
				return url
			}
		}
	}

	return ""
}

// getAntigravityPaths returns possible config paths for Antigravity
func getAntigravityPaths() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"~/Library/Application Support/Antigravity/User/settings.json",
			"~/Library/Application Support/Antigravity/config.json",
			"~/.config/antigravity/config.json",
		}
	case "linux":
		return []string{
			"~/.config/Antigravity/User/settings.json",
			"~/.config/antigravity/config.json",
			"~/.antigravity/config.json",
		}
	case "windows":
		return []string{
			"%APPDATA%/Antigravity/User/settings.json",
			"%APPDATA%/Antigravity/config.json",
			"%LOCALAPPDATA%/Antigravity/config.json",
		}
	}
	return nil
}

// getCursorPaths returns possible config paths for Cursor
func getCursorPaths() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"~/Library/Application Support/Cursor/User/settings.json",
			"~/.cursor/config.json",
		}
	case "linux":
		return []string{
			"~/.config/Cursor/User/settings.json",
			"~/.cursor/config.json",
		}
	case "windows":
		return []string{
			"%APPDATA%/Cursor/User/settings.json",
			"%LOCALAPPDATA%/Cursor/config.json",
		}
	}
	return nil
}

// getWindsurfPaths returns possible config paths for Windsurf
func getWindsurfPaths() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"~/Library/Application Support/Windsurf/User/settings.json",
			"~/Library/Application Support/Windsurf/config.json",
			"~/.windsurf/config.json",
		}
	case "linux":
		return []string{
			"~/.config/Windsurf/User/settings.json",
			"~/.config/windsurf/config.json",
			"~/.windsurf/config.json",
		}
	case "windows":
		return []string{
			"%APPDATA%/Windsurf/User/settings.json",
			"%APPDATA%/Windsurf/config.json",
			"%LOCALAPPDATA%/Windsurf/config.json",
		}
	}
	return nil
}

// getClaudeCodePaths returns possible config paths for Claude Code CLI
func getClaudeCodePaths() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"~/.claude/settings.json",
			"~/.claude/config.json",
			"~/.config/claude/settings.json",
		}
	case "linux":
		return []string{
			"~/.claude/settings.json",
			"~/.claude/config.json",
			"~/.config/claude/settings.json",
		}
	case "windows":
		return []string{
			"%USERPROFILE%/.claude/settings.json",
			"%APPDATA%/claude/settings.json",
			"%LOCALAPPDATA%/claude/settings.json",
		}
	}
	return nil
}

// getQoderPaths returns possible config paths for Qoder
func getQoderPaths() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"~/Library/Application Support/Qoder/User/settings.json",
			"~/Library/Application Support/Qoder/config.json",
			"~/.qoder/config.json",
			"~/.config/qoder/config.json",
		}
	case "linux":
		return []string{
			"~/.config/Qoder/User/settings.json",
			"~/.config/qoder/config.json",
			"~/.qoder/config.json",
		}
	case "windows":
		return []string{
			"%APPDATA%/Qoder/User/settings.json",
			"%APPDATA%/Qoder/config.json",
			"%LOCALAPPDATA%/Qoder/config.json",
		}
	}
	return nil
}

// getTRAEPaths returns possible config paths for TRAE (ByteDance)
func getTRAEPaths() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"~/Library/Application Support/TRAE/User/settings.json",
			"~/Library/Application Support/TRAE/config.json",
			"~/.trae/config.json",
			"~/.config/trae/config.json",
		}
	case "linux":
		return []string{
			"~/.config/TRAE/User/settings.json",
			"~/.config/trae/config.json",
			"~/.trae/config.json",
		}
	case "windows":
		return []string{
			"%APPDATA%/TRAE/User/settings.json",
			"%APPDATA%/TRAE/config.json",
			"%LOCALAPPDATA%/TRAE/config.json",
		}
	}
	return nil
}

// getKiroPaths returns possible config paths for Kiro
func getKiroPaths() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"~/.kiro/config.json",
			"~/Library/Application Support/Kiro/User/settings.json",
			"~/Library/Application Support/Kiro/config.json",
		}
	case "linux":
		return []string{
			"~/.kiro/config.json",
			"~/.config/Kiro/User/settings.json",
			"~/.config/kiro/config.json",
		}
	case "windows":
		return []string{
			"%USERPROFILE%/.kiro/config.json",
			"%APPDATA%/Kiro/User/settings.json",
			"%APPDATA%/Kiro/config.json",
		}
	}
	return nil
}

// getCopilotPaths returns possible config paths for GitHub Copilot
func getCopilotPaths() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"~/.config/github-copilot/hosts.json",
			"~/Library/Application Support/GitHub Copilot/hosts.json",
		}
	case "linux":
		return []string{
			"~/.config/github-copilot/hosts.json",
		}
	case "windows":
		return []string{
			"%LOCALAPPDATA%/github-copilot/hosts.json",
			"%APPDATA%/GitHub Copilot/hosts.json",
		}
	}
	return nil
}

// expandPath expands ~ and environment variables in a path
func expandPath(path string) string {
	// Expand ~
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[2:])
		}
	}

	// Expand environment variables
	path = os.ExpandEnv(path)

	return path
}

// ExtractAPIKey extracts API key from IDE config (returns masked version for display)
func (d *Discovery) ExtractAPIKey(ideType IDEType, configPath string) (key string, masked string) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", ""
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return "", ""
	}

	// Try different key names based on IDE type
	keyNames := []string{"api_key", "apiKey", "key", "anthropic_api_key", "openai_api_key"}

	for _, keyName := range keyNames {
		if apiKey, ok := config[keyName].(string); ok && apiKey != "" {
			return apiKey, maskAPIKey(apiKey)
		}
	}

	// For Claude Code, check nested structure
	if ideType == IDETypeClaudeCode {
		if providers, ok := config["providers"].(map[string]interface{}); ok {
			if anthropic, ok := providers["anthropic"].(map[string]interface{}); ok {
				if apiKey, ok := anthropic["api_key"].(string); ok && apiKey != "" {
					return apiKey, maskAPIKey(apiKey)
				}
			}
		}
	}

	return "", ""
}

// ExtractBaseURL extracts the base URL from IDE config file
func (d *Discovery) ExtractBaseURL(ideType IDEType, configPath string) string {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return ""
	}

	// Check top-level base_url / api_base_url
	for _, key := range []string{"base_url", "api_base_url", "apiBaseUrl"} {
		if url, ok := config[key].(string); ok && url != "" {
			return url
		}
	}

	// For Claude Code, check nested structure and env-style keys
	if ideType == IDETypeClaudeCode {
		if providers, ok := config["providers"].(map[string]interface{}); ok {
			if anthropic, ok := providers["anthropic"].(map[string]interface{}); ok {
				if url, ok := anthropic["base_url"].(string); ok && url != "" {
					return url
				}
			}
		}
	}

	return ""
}

// maskAPIKey masks an API key for display (shows first 4 and last 4 characters)
func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "..." + key[len(key)-4:]
}

// GetAPIKeyFromEnv gets API key from environment variables for an IDE
func GetAPIKeyFromEnv(ideType IDEType) (key string, envVar string) {
	envVars := getEnvVarsForIDE(ideType)
	for _, env := range envVars {
		if val := os.Getenv(env); val != "" {
			return val, env
		}
	}
	return "", ""
}

// getEnvVarsForIDE returns environment variable names to check for an IDE
func getEnvVarsForIDE(ideType IDEType) []string {
	switch ideType {
	case IDETypeClaudeCode:
		return []string{"ANTHROPIC_API_KEY", "CLAUDE_API_KEY"}
	case IDETypeCursor:
		return []string{"CURSOR_API_KEY", "OPENAI_API_KEY"}
	case IDETypeWindsurf:
		return []string{"WINDSURF_API_KEY", "OPENAI_API_KEY"}
	case IDETypeAntigravity:
		return []string{"ANTIGRAVITY_API_KEY", "GOOGLE_API_KEY"}
	case IDETypeQoder:
		return []string{"QODER_API_KEY", "OPENAI_API_KEY"}
	case IDETypeTRAE:
		return []string{"TRAE_API_KEY"}
	case IDETypeKiro:
		return []string{"KIRO_API_KEY", "OPENAI_API_KEY", "ANTHROPIC_API_KEY"}
	case IDETypeCopilot:
		return []string{"GITHUB_TOKEN", "COPILOT_TOKEN"}
	}
	return nil
}

// ImportConfig represents configuration that can be imported from an IDE
type ImportConfig struct {
	IDEType    IDEType  `json:"ide_type"`
	IDEName    string   `json:"ide_name"`
	APIKey     string   `json:"api_key,omitempty"`
	BaseURL    string   `json:"base_url,omitempty"`
	Models     []string `json:"models,omitempty"`
	Provider   string   `json:"provider,omitempty"` // "anthropic", "openai", etc.
	ConfigPath string   `json:"config_path,omitempty"`
	EnvVar     string   `json:"env_var,omitempty"` // Which env var the key came from
	Source          string   `json:"source"`            // "config", "env", "cc-switch", "extension"
	CanImport       bool     `json:"can_import"`        // Whether this config can be imported
	AlreadyImported bool     `json:"already_imported"`  // Whether this key already exists in provider pool

	// Claude Code extension config (from IDE settings.json claudeCode.environmentVariables)
	ExtensionConfig *ClaudeCodeExtConfig `json:"extension_config,omitempty"`
}

// ClaudeCodeExtConfig represents Claude Code extension settings found in an IDE's settings.json
type ClaudeCodeExtConfig struct {
	EnvVars       []ClaudeCodeEnvVar `json:"env_vars"`
	SelectedModel string             `json:"selected_model,omitempty"`
}

// ClaudeCodeEnvVar represents a single env var from claudeCode.environmentVariables
type ClaudeCodeEnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`  // Masked for display
}

// GetImportableConfigs returns all configurations that can be imported
func (d *Discovery) GetImportableConfigs(ctx context.Context) ([]*ImportConfig, error) {
	var configs []*ImportConfig

	// Scan all IDEs
	ides, err := d.Scan(ctx)
	if err != nil {
		return nil, err
	}

	for _, ide := range ides {
		// Try to get API key from config
		key, masked := d.ExtractAPIKey(ide.Type, ide.ConfigPath)
		canImport := key != ""

		// Try to get base URL from config file, fall back to proxy URL
		baseURL := d.ExtractBaseURL(ide.Type, ide.ConfigPath)
		if baseURL == "" {
			baseURL = ide.ProxyURL
		}

		config := &ImportConfig{
			IDEType:    ide.Type,
			IDEName:    ide.Name,
			BaseURL:    baseURL,
			Models:     ide.Models,
			ConfigPath: ide.ConfigPath,
			Source:     "config",
			Provider:   getProviderForIDE(ide.Type),
			CanImport:  canImport,
		}

		// For Antigravity, the proxy URL itself is sufficient for import
		// (OAuth tokens are managed by the app, not stored in config files)
		if ide.Type == IDETypeAntigravity && ide.ProxyURL != "" && ide.Connected {
			config.CanImport = true
			config.Source = "config"
		}

		if canImport {
			config.APIKey = masked // Only return masked version
		}

		// For VS Code forks, try to extract Claude Code extension config
		if isVSCodeFork(ide.Type) {
			extConfig, err := ExtractClaudeCodeExtConfig(ide.ConfigPath)
			if err == nil && extConfig != nil && len(extConfig.EnvVars) > 0 {
				config.ExtensionConfig = extConfig
				config.Source = "extension"
				config.CanImport = true
			}
		}

		configs = append(configs, config)
	}

	// Also check environment variables
	for _, ideType := range []IDEType{IDETypeClaudeCode, IDETypeCursor, IDETypeWindsurf, IDETypeAntigravity, IDETypeQoder, IDETypeTRAE} {
		if key, envVar := GetAPIKeyFromEnv(ideType); key != "" {
			envBaseURL := GetBaseURLFromEnv(ideType)
			// Check if we already have this IDE from config
			found := false
			for _, c := range configs {
				if c.IDEType == ideType {
					// Update existing config with env key if it didn't have one
					if !c.CanImport {
						c.APIKey = maskAPIKey(key)
						c.EnvVar = envVar
						c.Source = "env"
						c.CanImport = true
					}
					// Always prefer env base URL if set
					if envBaseURL != "" {
						c.BaseURL = envBaseURL
					}
					found = true
					break
				}
			}
			if !found {
				configs = append(configs, &ImportConfig{
					IDEType:   ideType,
					IDEName:   getIDEName(ideType),
					APIKey:    maskAPIKey(key),
					BaseURL:   envBaseURL,
					EnvVar:    envVar,
					Provider:  getProviderForIDE(ideType),
					Source:    "env",
					CanImport: true,
				})
			}
		}
	}

	// Deduplicate extension configs: if multiple IDEs have the same env vars,
	// mark duplicates so the UI doesn't show redundant import options.
	seen := make(map[string]bool) // key = "provider:maskedKey"
	for _, c := range configs {
		if c.ExtensionConfig != nil && c.CanImport && !c.AlreadyImported {
			for _, ev := range c.ExtensionConfig.EnvVars {
				info, ok := claudeCodeEnvVarProviders[ev.Name]
				if !ok || info.FieldType == "base_url" {
					continue
				}
				dedup := info.ProviderID + ":" + ev.Value
				if seen[dedup] {
					c.AlreadyImported = true
					break
				}
				seen[dedup] = true
			}
		}
		// Also deduplicate by masked API key + provider for non-extension configs
		if c.APIKey != "" && c.CanImport && !c.AlreadyImported && c.Source != "extension" {
			dedup := c.Provider + ":" + c.APIKey
			if seen[dedup] {
				c.AlreadyImported = true
			} else {
				seen[dedup] = true
			}
		}
	}

	return configs, nil
}

// getProviderForIDE returns the LLM provider type for an IDE
func getProviderForIDE(ideType IDEType) string {
	switch ideType {
	case IDETypeClaudeCode:
		return "anthropic"
	case IDETypeCursor, IDETypeWindsurf, IDETypeQoder, IDETypeKiro:
		return "openai"
	case IDETypeAntigravity:
		return "google"
	case IDETypeTRAE:
		return "custom"
	case IDETypeCopilot:
		return "github"
	}
	return "openai"
}

// ImportFromClaudeCodeSwitch imports configuration from Claude Code's cc switch command output
// The cc switch command outputs provider configurations that can be parsed
func (d *Discovery) ImportFromClaudeCodeSwitch(switchOutput string) (*ImportConfig, error) {
	// Parse the cc switch output format
	// Expected format varies, but typically includes provider info and API key
	lines := strings.Split(switchOutput, "\n")

	config := &ImportConfig{
		IDEType: IDETypeClaudeCode,
		IDEName: "Claude Code",
		Source:  "cc-switch",
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Look for API key patterns
		if strings.Contains(line, "api_key") || strings.Contains(line, "API_KEY") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[1])
				key = strings.Trim(key, "\"'")
				if key != "" {
					config.APIKey = maskAPIKey(key)
				}
			}
		}

		// Look for base URL
		if strings.Contains(line, "base_url") || strings.Contains(line, "BASE_URL") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				url := strings.TrimSpace(parts[1])
				url = strings.Trim(url, "\"'")
				if url != "" {
					config.BaseURL = url
				}
			}
		}

		// Look for provider
		if strings.Contains(line, "provider") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				provider := strings.TrimSpace(parts[1])
				provider = strings.Trim(provider, "\"'")
				if provider != "" {
					config.Provider = provider
				}
			}
		}
	}

	if config.APIKey == "" && config.BaseURL == "" {
		return nil, fmt.Errorf("no valid configuration found in cc switch output")
	}

	return config, nil
}

// GetRealAPIKey returns the actual (unmasked) API key for import
// This should only be called when actually importing the configuration
func (d *Discovery) GetRealAPIKey(ideType IDEType) (string, error) {
	// First try environment variable
	if key, _ := GetAPIKeyFromEnv(ideType); key != "" {
		return key, nil
	}

	// Then try config file
	configPath := findConfigPath(ideType)
	if configPath == "" {
		return "", fmt.Errorf("config path not found for %s", ideType)
	}

	key, _ := d.ExtractAPIKey(ideType, configPath)
	if key == "" {
		return "", fmt.Errorf("no API key found for %s", ideType)
	}

	return key, nil
}

// GetRealBaseURL returns the base URL configured for an IDE.
// Checks env vars first (e.g. ANTHROPIC_BASE_URL), then config file.
func (d *Discovery) GetRealBaseURL(ideType IDEType) string {
	// Check env var first
	if url := GetBaseURLFromEnv(ideType); url != "" {
		return url
	}

	// Then try config file
	configPath := findConfigPath(ideType)
	if configPath == "" {
		return ""
	}
	return d.ExtractBaseURL(ideType, configPath)
}

// GetBaseURLFromEnv gets base URL from environment variables for an IDE
func GetBaseURLFromEnv(ideType IDEType) string {
	envVars := getBaseURLEnvVarsForIDE(ideType)
	for _, env := range envVars {
		if val := os.Getenv(env); val != "" {
			return val
		}
	}
	return ""
}

// getBaseURLEnvVarsForIDE returns environment variable names for base URL
func getBaseURLEnvVarsForIDE(ideType IDEType) []string {
	switch ideType {
	case IDETypeClaudeCode:
		return []string{"ANTHROPIC_BASE_URL"}
	case IDETypeCursor, IDETypeWindsurf, IDETypeQoder, IDETypeKiro:
		return []string{"OPENAI_BASE_URL"}
	case IDETypeAntigravity:
		return []string{"GOOGLE_BASE_URL"}
	}
	return nil
}

// claudeCodeEnvVarProviders maps Claude Code extension env var names to provider info
var claudeCodeEnvVarProviders = map[string]struct {
	ProviderID string
	FieldType  string // "api_key", "base_url", "auth_token"
}{
	"ANTHROPIC_API_KEY":         {ProviderID: "anthropic", FieldType: "api_key"},
	"ANTHROPIC_AUTH_TOKEN":      {ProviderID: "anthropic", FieldType: "auth_token"},
	"ANTHROPIC_BASE_URL":        {ProviderID: "anthropic", FieldType: "base_url"},
	"OPENAI_API_KEY":            {ProviderID: "openai", FieldType: "api_key"},
	"OPENAI_BASE_URL":           {ProviderID: "openai", FieldType: "base_url"},
	"GOOGLE_API_KEY":            {ProviderID: "google", FieldType: "api_key"},
	"AZURE_OPENAI_API_KEY":      {ProviderID: "azure-openai", FieldType: "api_key"},
	"AZURE_OPENAI_BASE_URL":     {ProviderID: "azure-openai", FieldType: "base_url"},
	"AZURE_OPENAI_ENDPOINT":     {ProviderID: "azure-openai", FieldType: "base_url"},
	"DEEPSEEK_API_KEY":          {ProviderID: "deepseek", FieldType: "api_key"},
	"MISTRAL_API_KEY":           {ProviderID: "mistral", FieldType: "api_key"},
	"GROQ_API_KEY":              {ProviderID: "groq", FieldType: "api_key"},
}

// ExtractClaudeCodeExtConfig reads Claude Code extension config from an IDE's settings.json
// It looks for claudeCode.environmentVariables and claudeCode.selectedModel
func ExtractClaudeCodeExtConfig(settingsPath string) (*ClaudeCodeExtConfig, error) {
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return nil, err
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, err
	}

	config := &ClaudeCodeExtConfig{}

	// Extract claudeCode.environmentVariables
	if envVars, ok := settings["claudeCode.environmentVariables"]; ok {
		if envList, ok := envVars.([]interface{}); ok {
			for _, item := range envList {
				envMap, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				name, _ := envMap["name"].(string)
				value, _ := envMap["value"].(string)
				if name == "" || value == "" {
					continue
				}
				// Only include known env vars
				if _, known := claudeCodeEnvVarProviders[name]; known {
					config.EnvVars = append(config.EnvVars, ClaudeCodeEnvVar{
						Name:  name,
						Value: maskEnvValue(name, value),
					})
				}
			}
		}
	}

	// Extract claudeCode.selectedModel
	if model, ok := settings["claudeCode.selectedModel"].(string); ok {
		config.SelectedModel = model
	}

	if len(config.EnvVars) == 0 && config.SelectedModel == "" {
		return nil, nil // No Claude Code config found
	}

	return config, nil
}

// getRealClaudeCodeExtEnvVars reads the actual (unmasked) env vars from IDE settings.json
func getRealClaudeCodeExtEnvVars(settingsPath string) ([]ClaudeCodeEnvVar, error) {
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return nil, err
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, err
	}

	var envVarsList []ClaudeCodeEnvVar

	if envVars, ok := settings["claudeCode.environmentVariables"]; ok {
		if envList, ok := envVars.([]interface{}); ok {
			for _, item := range envList {
				envMap, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				name, _ := envMap["name"].(string)
				value, _ := envMap["value"].(string)
				if name == "" || value == "" {
					continue
				}
				if _, known := claudeCodeEnvVarProviders[name]; known {
					envVarsList = append(envVarsList, ClaudeCodeEnvVar{
						Name:  name,
						Value: value, // Real value, not masked
					})
				}
			}
		}
	}

	return envVarsList, nil
}

// maskEnvValue masks a value based on the env var type
func maskEnvValue(name, value string) string {
	info, ok := claudeCodeEnvVarProviders[name]
	if !ok {
		return "****"
	}
	// Don't mask URLs
	if info.FieldType == "base_url" {
		return value
	}
	// Mask API keys and tokens
	return maskAPIKey(value)
}

// isVSCodeFork returns true if the IDE is a VS Code fork that may have Claude Code extension
func isVSCodeFork(ideType IDEType) bool {
	switch ideType {
	case IDETypeCursor, IDETypeWindsurf, IDETypeAntigravity, IDETypeQoder, IDETypeTRAE, IDETypeKiro:
		return true
	}
	return false
}

// EnvVarProviderInfo contains provider mapping info for a Claude Code env var
type EnvVarProviderInfo struct {
	ProviderID string
	FieldType  string // "api_key", "base_url", "auth_token"
}

// GetClaudeCodeEnvVarProvider returns the provider info for a Claude Code extension env var name
func GetClaudeCodeEnvVarProvider(name string) (EnvVarProviderInfo, bool) {
	info, ok := claudeCodeEnvVarProviders[name]
	if !ok {
		return EnvVarProviderInfo{}, false
	}
	return EnvVarProviderInfo{ProviderID: info.ProviderID, FieldType: info.FieldType}, true
}

// GetRealExtensionConfig returns the real (unmasked) extension config for import
func (d *Discovery) GetRealExtensionConfig(ideType IDEType) ([]ClaudeCodeEnvVar, error) {
	configPath := findConfigPath(ideType)
	if configPath == "" {
		return nil, fmt.Errorf("config path not found for %s", ideType)
	}
	return getRealClaudeCodeExtEnvVars(configPath)
}
