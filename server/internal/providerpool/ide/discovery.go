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
)

// IDEInfo contains information about a discovered IDE
type IDEInfo struct {
	Type        IDEType   `json:"type"`
	Name        string    `json:"name"`
	Version     string    `json:"version,omitempty"`
	ConfigPath  string    `json:"config_path"`
	ProxyURL    string    `json:"proxy_url,omitempty"`
	Connected   bool      `json:"connected"`
	Models      []string  `json:"models,omitempty"`
	LastChecked time.Time `json:"last_checked"`
	Error       string    `json:"error,omitempty"`
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

// Scan scans for all supported IDEs
func (d *Discovery) Scan(ctx context.Context) ([]*IDEInfo, error) {
	var results []*IDEInfo
	var wg sync.WaitGroup
	var mu sync.Mutex

	ideTypes := []IDEType{IDETypeAntigravity, IDETypeCursor, IDETypeWindsurf}

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

	// Try common local ports based on IDE type
	switch ideType {
	case IDETypeAntigravity:
		return tryPorts([]int{9876, 9877, 9878})
	case IDETypeCursor:
		return tryPorts([]int{9999, 9998, 9997})
	case IDETypeWindsurf:
		return tryPorts([]int{9800, 9801, 9802})
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
			"~/Library/Application Support/Antigravity/config.json",
			"~/.config/antigravity/config.json",
		}
	case "linux":
		return []string{
			"~/.config/antigravity/config.json",
			"~/.antigravity/config.json",
		}
	case "windows":
		return []string{
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
			"~/Library/Application Support/Windsurf/config.json",
			"~/.windsurf/config.json",
		}
	case "linux":
		return []string{
			"~/.config/windsurf/config.json",
			"~/.windsurf/config.json",
		}
	case "windows":
		return []string{
			"%APPDATA%/Windsurf/config.json",
			"%LOCALAPPDATA%/Windsurf/config.json",
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
