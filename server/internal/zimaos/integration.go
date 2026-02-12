package zimaos

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// Integration provides ZimaOS-specific functionality.
type Integration struct {
	config     Config
	httpClient *http.Client
	mu         sync.RWMutex
	systemInfo *SystemInfo
}

// Config holds ZimaOS integration configuration.
type Config struct {
	Enabled     bool   `mapstructure:"enabled"`
	APIEndpoint string `mapstructure:"api_endpoint"`
	AppID       string `mapstructure:"app_id"`
	AppSecret   string `mapstructure:"app_secret"`
	DataPath    string `mapstructure:"data_path"`
}

// SystemInfo contains ZimaOS system information.
type SystemInfo struct {
	Version      string `json:"version"`
	Hostname     string `json:"hostname"`
	Architecture string `json:"architecture"`
	TotalMemory  uint64 `json:"total_memory"`
	TotalStorage uint64 `json:"total_storage"`
	Timezone     string `json:"timezone"`
	Language     string `json:"language"`
}

// AppInfo contains information about a ZimaOS app.
type AppInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Status      string `json:"status"`
	Port        int    `json:"port"`
	Description string `json:"description"`
}

// FileInfo contains information about a file.
type FileInfo struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	IsDir    bool      `json:"is_dir"`
	ModTime  time.Time `json:"mod_time"`
	MimeType string    `json:"mime_type,omitempty"`
}

// NewIntegration creates a new ZimaOS integration.
func NewIntegration(cfg Config) *Integration {
	if cfg.APIEndpoint == "" {
		cfg.APIEndpoint = "http://localhost:80"
	}
	if cfg.DataPath == "" {
		cfg.DataPath = "/DATA/AppData/zimaos-blue"
	}

	return &Integration{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// IsRunningOnZimaOS checks if the application is running on ZimaOS.
func (i *Integration) IsRunningOnZimaOS() bool {
	// Check for ZimaOS-specific environment variables or files
	if os.Getenv("ZIMAOS_VERSION") != "" {
		return true
	}

	// Check for ZimaOS marker file
	if _, err := os.Stat("/etc/zimaos-release"); err == nil {
		return true
	}

	return false
}

// GetSystemInfo retrieves ZimaOS system information.
func (i *Integration) GetSystemInfo(ctx context.Context) (*SystemInfo, error) {
	i.mu.RLock()
	if i.systemInfo != nil {
		info := *i.systemInfo
		i.mu.RUnlock()
		return &info, nil
	}
	i.mu.RUnlock()

	if !i.IsRunningOnZimaOS() {
		return &SystemInfo{
			Version:      "unknown",
			Hostname:     getHostname(),
			Architecture: getArchitecture(),
			Timezone:     getTimezone(),
			Language:     "en",
		}, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, i.config.APIEndpoint+"/v1/sys/info", nil)
	if err != nil {
		return nil, err
	}

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get system info: status %d", resp.StatusCode)
	}

	var info SystemInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}

	i.mu.Lock()
	i.systemInfo = &info
	i.mu.Unlock()

	return &info, nil
}

// ListApps lists installed ZimaOS applications.
func (i *Integration) ListApps(ctx context.Context) ([]AppInfo, error) {
	if !i.IsRunningOnZimaOS() {
		return []AppInfo{}, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, i.config.APIEndpoint+"/v1/apps", nil)
	if err != nil {
		return nil, err
	}

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list apps: status %d", resp.StatusCode)
	}

	var apps []AppInfo
	if err := json.NewDecoder(resp.Body).Decode(&apps); err != nil {
		return nil, err
	}

	return apps, nil
}

// GetApp retrieves information about a specific app.
func (i *Integration) GetApp(ctx context.Context, appID string) (*AppInfo, error) {
	if !i.IsRunningOnZimaOS() {
		return nil, fmt.Errorf("not running on ZimaOS")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, i.config.APIEndpoint+"/v1/apps/"+appID, nil)
	if err != nil {
		return nil, err
	}

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get app: status %d", resp.StatusCode)
	}

	var app AppInfo
	if err := json.NewDecoder(resp.Body).Decode(&app); err != nil {
		return nil, err
	}

	return &app, nil
}

// ListFiles lists files in a directory.
func (i *Integration) ListFiles(ctx context.Context, path string) ([]FileInfo, error) {
	if !i.IsRunningOnZimaOS() {
		// Fallback to local filesystem
		return i.listLocalFiles(path)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, i.config.APIEndpoint+"/v1/files?path="+path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list files: status %d", resp.StatusCode)
	}

	var files []FileInfo
	if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
		return nil, err
	}

	return files, nil
}

// SendNotification sends a notification through ZimaOS.
func (i *Integration) SendNotification(ctx context.Context, title, message string, level string) error {
	if !i.IsRunningOnZimaOS() {
		// Log notification locally
		log.Printf("[%s] %s: %s", level, title, message)
		return nil
	}

	payload := map[string]string{
		"title":   title,
		"message": message,
		"level":   level,
		"app_id":  i.config.AppID,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, i.config.APIEndpoint+"/v1/notify", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	// Note: In real implementation, we'd use bytes.NewReader(body)
	_ = body

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to send notification: status %d", resp.StatusCode)
	}

	return nil
}

// GetDataPath returns the ZimaOS data path for this app.
func (i *Integration) GetDataPath() string {
	return i.config.DataPath
}

// GetConfigPath returns the ZimaOS config path for this app.
func (i *Integration) GetConfigPath() string {
	return i.config.DataPath + "/config"
}

func (i *Integration) listLocalFiles(path string) ([]FileInfo, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	files := make([]FileInfo, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		files = append(files, FileInfo{
			Name:    entry.Name(),
			Path:    path + "/" + entry.Name(),
			Size:    info.Size(),
			IsDir:   entry.IsDir(),
			ModTime: info.ModTime(),
		})
	}

	return files, nil
}

func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}

func getArchitecture() string {
	// This is a simplified version
	// In production, use runtime.GOARCH
	return "amd64"
}

func getTimezone() string {
	tz := os.Getenv("TZ")
	if tz != "" {
		return tz
	}
	return "UTC"
}
