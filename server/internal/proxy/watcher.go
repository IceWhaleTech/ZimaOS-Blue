package proxy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"gopkg.in/yaml.v3"
)

// WatcherConfig configuration watcher settings
type WatcherConfig struct {
	Enabled      bool          `json:"enabled"`
	PollInterval time.Duration `json:"poll_interval"`
}

// DefaultWatcherConfig returns default watcher configuration
func DefaultWatcherConfig() *WatcherConfig {
	return &WatcherConfig{
		Enabled:      true,
		PollInterval: 10 * time.Second,
	}
}

// ConfigWatcher watches for configuration changes
type ConfigWatcher struct {
	configPath   string
	pollInterval time.Duration
	lastModTime  time.Time
	lastHash     string
	mu           sync.RWMutex

	OnChange func(oldConfig, newConfig *ProxyConfig)
	OnError  func(error)

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Stats
	reloadCount int64
	errorCount  int64
	lastReload  time.Time
}

// NewConfigWatcher creates a new config watcher
func NewConfigWatcher(configPath string, pollInterval time.Duration) *ConfigWatcher {
	ctx, cancel := context.WithCancel(context.Background())

	if pollInterval <= 0 {
		pollInterval = 10 * time.Second
	}

	return &ConfigWatcher{
		configPath:   configPath,
		pollInterval: pollInterval,
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Start starts watching for config changes
func (cw *ConfigWatcher) Start() {
	cw.wg.Add(1)
	go cw.watch()
}

// Stop stops the config watcher
func (cw *ConfigWatcher) Stop() {
	cw.cancel()
	cw.wg.Wait()
}

// watch is the main watch loop
func (cw *ConfigWatcher) watch() {
	defer cw.wg.Done()

	ticker := time.NewTicker(cw.pollInterval)
	defer ticker.Stop()

	// Get initial state
	cw.updateLastState()

	for {
		select {
		case <-cw.ctx.Done():
			return
		case <-ticker.C:
			cw.checkForChanges()
		}
	}
}

// checkForChanges checks if config file has changed
func (cw *ConfigWatcher) checkForChanges() {
	info, err := os.Stat(cw.configPath)
	if err != nil {
		cw.recordError(err)
		return
	}

	cw.mu.RLock()
	lastModTime := cw.lastModTime
	lastHash := cw.lastHash
	cw.mu.RUnlock()

	// Check modification time first (fast check)
	if info.ModTime().Equal(lastModTime) {
		return
	}

	// Read and hash file (accurate check)
	data, err := os.ReadFile(cw.configPath)
	if err != nil {
		cw.recordError(err)
		return
	}

	hash := cw.hashContent(data)
	if hash == lastHash {
		cw.mu.Lock()
		cw.lastModTime = info.ModTime()
		cw.mu.Unlock()
		return
	}

	// Config changed, parse new config
	newConfig, err := cw.parseConfig(data)
	if err != nil {
		cw.recordError(err)
		return
	}

	// Load old config for comparison
	var oldConfig *ProxyConfig
	oldPath := cw.configPath + ".old"
	if oldData, err := os.ReadFile(oldPath); err == nil {
		oldConfig, _ = cw.parseConfig(oldData)
	}

	// Update state
	cw.mu.Lock()
	cw.lastModTime = info.ModTime()
	cw.lastHash = hash
	cw.reloadCount++
	cw.lastReload = timeutil.NowTime()
	cw.mu.Unlock()

	// Save current as old for next comparison
	os.WriteFile(oldPath, data, 0644)

	// Notify
	if cw.OnChange != nil {
		cw.OnChange(oldConfig, newConfig)
	}
}

// updateLastState updates the last known state
func (cw *ConfigWatcher) updateLastState() {
	info, err := os.Stat(cw.configPath)
	if err != nil {
		return
	}

	data, err := os.ReadFile(cw.configPath)
	if err != nil {
		return
	}

	cw.mu.Lock()
	defer cw.mu.Unlock()

	cw.lastModTime = info.ModTime()
	cw.lastHash = cw.hashContent(data)
}

// hashContent returns SHA256 hash of content
func (cw *ConfigWatcher) hashContent(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// parseConfig parses config from YAML
func (cw *ConfigWatcher) parseConfig(data []byte) (*ProxyConfig, error) {
	var config ProxyConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// recordError records an error
func (cw *ConfigWatcher) recordError(err error) {
	cw.mu.Lock()
	cw.errorCount++
	cw.mu.Unlock()

	if cw.OnError != nil {
		cw.OnError(err)
	}
}

// ForceReload forces a configuration reload
func (cw *ConfigWatcher) ForceReload() error {
	data, err := os.ReadFile(cw.configPath)
	if err != nil {
		return err
	}

	newConfig, err := cw.parseConfig(data)
	if err != nil {
		return err
	}

	// Load old config
	var oldConfig *ProxyConfig
	oldPath := cw.configPath + ".old"
	if oldData, err := os.ReadFile(oldPath); err == nil {
		oldConfig, _ = cw.parseConfig(oldData)
	}

	// Update state
	info, _ := os.Stat(cw.configPath)
	cw.mu.Lock()
	if info != nil {
		cw.lastModTime = info.ModTime()
	}
	cw.lastHash = cw.hashContent(data)
	cw.reloadCount++
	cw.lastReload = timeutil.NowTime()
	cw.mu.Unlock()

	// Save current as old
	os.WriteFile(oldPath, data, 0644)

	// Notify
	if cw.OnChange != nil {
		cw.OnChange(oldConfig, newConfig)
	}

	return nil
}

// GetConfigPath returns the config file path
func (cw *ConfigWatcher) GetConfigPath() string {
	return cw.configPath
}

// SetConfigPath sets a new config file path
func (cw *ConfigWatcher) SetConfigPath(path string) {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	cw.configPath = path
	cw.lastModTime = time.Time{}
	cw.lastHash = ""
}

// Stats returns watcher statistics
func (cw *ConfigWatcher) Stats() map[string]interface{} {
	cw.mu.RLock()
	defer cw.mu.RUnlock()

	return map[string]interface{}{
		"config_path":   cw.configPath,
		"poll_interval": cw.pollInterval.String(),
		"reload_count":  cw.reloadCount,
		"error_count":   cw.errorCount,
		"last_reload":   cw.lastReload,
		"last_hash":     cw.lastHash[:min(16, len(cw.lastHash))],
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// HotReloadableConfig defines which config sections can be hot-reloaded
type HotReloadableConfig struct {
	// Can be hot-reloaded
	Providers    []*ProviderConfig `json:"providers"`
	Failover     *FailoverConfig   `json:"failover"`
	Guard        *GuardConfig      `json:"guard"`
	Auth         *AuthConfig       `json:"auth"`
	RateLimit    *RateLimitConfig  `json:"rate_limit"`
	Mock         *MockConfig       `json:"mock"`
	ModelCompat  *ModelCompatConfig `json:"model_compat"`

	// Cannot be hot-reloaded (requires restart)
	// - Port binding
	// - TLS certificates
	// - Core proxy settings
}

// ConfigReloader handles applying configuration changes
type ConfigReloader struct {
	router   *Router
	failover *FailoverHandler
	guard    *PromptGuard
	auth     *Authenticator
	mock     *MockHandler
	compat   *ModelCompatLayer
	mu       sync.Mutex
}

// NewConfigReloader creates a new config reloader
func NewConfigReloader(
	router *Router,
	failover *FailoverHandler,
	guard *PromptGuard,
	auth *Authenticator,
	mock *MockHandler,
	compat *ModelCompatLayer,
) *ConfigReloader {
	return &ConfigReloader{
		router:   router,
		failover: failover,
		guard:    guard,
		auth:     auth,
		mock:     mock,
		compat:   compat,
	}
}

// ApplyConfig applies a new configuration
func (cr *ConfigReloader) ApplyConfig(config *ProxyConfig) error {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	// Update providers
	if config.Route != nil && cr.router != nil {
		for _, pc := range config.Route.Providers {
			if _, ok := cr.router.GetProvider(pc.Name); ok {
				// Update existing provider
				cr.router.SetProviderEnabled(pc.Name, pc.Enabled)
			} else {
				// Add new provider
				cr.router.AddProvider(pc)
			}
		}
	}

	// Update mock endpoints
	if config.Mock != nil && cr.mock != nil {
		cr.mock.SetGlobalEnabled(config.Mock.Enabled)
		for _, ep := range config.Mock.Endpoints {
			cr.mock.AddEndpoint(ep)
		}
	}

	// Update model compatibility
	if config.ModelCompat != nil && cr.compat != nil {
		for model, features := range config.ModelCompat.ModelOverrides {
			cr.compat.SetFeatures(model, features)
		}
	}

	return nil
}
