package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// HotReloadConfig holds hot reload configuration
type HotReloadConfig struct {
	Enabled             bool          `yaml:"enabled"`
	WatchInterval       time.Duration `yaml:"watch_interval"`
	ValidateBeforeApply bool          `yaml:"validate_before_apply"`
}

// ReloadEvent represents a configuration reload event
type ReloadEvent struct {
	Time      time.Time
	Success   bool
	Error     error
	OldConfig *Config
	NewConfig *Config
}

// ReloadCallback is called when configuration is reloaded
type ReloadCallback func(event ReloadEvent)

// HotReloader manages configuration hot reloading
type HotReloader struct {
	configPath string
	config     atomic.Pointer[Config]
	hrConfig   *HotReloadConfig

	watcher   *fsnotify.Watcher
	callbacks []ReloadCallback

	mu            sync.RWMutex
	lastReload    time.Time
	reloadCount   int64
	lastError     error
	isReloading   atomic.Bool

	ctx    context.Context
	cancel context.CancelFunc
}

// NewHotReloader creates a new hot reloader
func NewHotReloader(configPath string, initialConfig *Config, hrConfig *HotReloadConfig) (*HotReloader, error) {
	if hrConfig == nil {
		hrConfig = &HotReloadConfig{
			Enabled:             true,
			WatchInterval:       time.Second,
			ValidateBeforeApply: true,
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	hr := &HotReloader{
		configPath: configPath,
		hrConfig:   hrConfig,
		ctx:        ctx,
		cancel:     cancel,
		lastReload: timeutil.NowTime(),
	}

	hr.config.Store(initialConfig)

	return hr, nil
}

// Start begins watching for configuration changes
func (hr *HotReloader) Start() error {
	if !hr.hrConfig.Enabled {
		return nil
	}

	// Setup file watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	hr.watcher = watcher

	if hr.configPath != "" {
		if err := watcher.Add(hr.configPath); err != nil {
			watcher.Close()
			return fmt.Errorf("failed to watch config file: %w", err)
		}
	}

	// Start watching goroutine
	go hr.watchLoop()

	// Setup SIGHUP handler
	go hr.signalHandler()

	log.Printf("[INFO] Hot reload started: config_path=%s validate_before_apply=%v",
		hr.configPath, hr.hrConfig.ValidateBeforeApply)

	return nil
}

// Stop stops the hot reloader
func (hr *HotReloader) Stop() error {
	hr.cancel()

	if hr.watcher != nil {
		return hr.watcher.Close()
	}
	return nil
}

// Config returns the current configuration
func (hr *HotReloader) Config() *Config {
	return hr.config.Load()
}

// Reload manually triggers a configuration reload
func (hr *HotReloader) Reload() error {
	return hr.reload()
}

// OnReload registers a callback for reload events
func (hr *HotReloader) OnReload(callback ReloadCallback) {
	hr.mu.Lock()
	defer hr.mu.Unlock()
	hr.callbacks = append(hr.callbacks, callback)
}

// Stats returns hot reload statistics
func (hr *HotReloader) Stats() map[string]interface{} {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	stats := map[string]interface{}{
		"enabled":       hr.hrConfig.Enabled,
		"config_path":   hr.configPath,
		"last_reload":   hr.lastReload,
		"reload_count":  atomic.LoadInt64(&hr.reloadCount),
		"is_reloading":  hr.isReloading.Load(),
	}

	if hr.lastError != nil {
		stats["last_error"] = hr.lastError.Error()
	}

	return stats
}

func (hr *HotReloader) watchLoop() {
	// Debounce timer
	var debounceTimer *time.Timer
	debounceDelay := hr.hrConfig.WatchInterval
	if debounceDelay < 100*time.Millisecond {
		debounceDelay = 100 * time.Millisecond
	}

	for {
		select {
		case <-hr.ctx.Done():
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return

		case event, ok := <-hr.watcher.Events:
			if !ok {
				return
			}

			// Only react to write and create events
			if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}

			log.Printf("[DEBUG] Config file changed: event=%s file=%s",
				event.Op.String(), event.Name)

			// Debounce rapid changes
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.AfterFunc(debounceDelay, func() {
				if err := hr.reload(); err != nil {
					log.Printf("[ERROR] Failed to reload config: %v", err)
				}
			})

		case err, ok := <-hr.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("[ERROR] Config watcher error: %v", err)
		}
	}
}

func (hr *HotReloader) signalHandler() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGHUP)

	for {
		select {
		case <-hr.ctx.Done():
			signal.Stop(sigChan)
			return
		case <-sigChan:
			log.Printf("[INFO] Received SIGHUP, reloading configuration")
			if err := hr.reload(); err != nil {
				log.Printf("[ERROR] Failed to reload config on SIGHUP: %v", err)
			}
		}
	}
}

func (hr *HotReloader) reload() error {
	// Prevent concurrent reloads
	if !hr.isReloading.CompareAndSwap(false, true) {
		return fmt.Errorf("reload already in progress")
	}
	defer hr.isReloading.Store(false)

	oldConfig := hr.config.Load()

	// Load new configuration
	newConfig, err := Load(hr.configPath)
	if err != nil {
		hr.mu.Lock()
		hr.lastError = err
		hr.mu.Unlock()

		hr.notifyCallbacks(ReloadEvent{
			Time:      timeutil.NowTime(),
			Success:   false,
			Error:     err,
			OldConfig: oldConfig,
		})

		return fmt.Errorf("failed to load config: %w", err)
	}

	// Validate if enabled
	if hr.hrConfig.ValidateBeforeApply {
		if err := hr.validateConfig(newConfig); err != nil {
			hr.mu.Lock()
			hr.lastError = err
			hr.mu.Unlock()

			hr.notifyCallbacks(ReloadEvent{
				Time:      timeutil.NowTime(),
				Success:   false,
				Error:     err,
				OldConfig: oldConfig,
				NewConfig: newConfig,
			})

			return fmt.Errorf("config validation failed: %w", err)
		}
	}

	// Apply new configuration
	hr.config.Store(newConfig)

	hr.mu.Lock()
	hr.lastReload = timeutil.NowTime()
	hr.lastError = nil
	hr.mu.Unlock()
	atomic.AddInt64(&hr.reloadCount, 1)

	log.Printf("[INFO] Configuration reloaded successfully: reload_count=%d",
		atomic.LoadInt64(&hr.reloadCount))

	hr.notifyCallbacks(ReloadEvent{
		Time:      timeutil.NowTime(),
		Success:   true,
		OldConfig: oldConfig,
		NewConfig: newConfig,
	})

	return nil
}

func (hr *HotReloader) validateConfig(cfg *Config) error {
	// Basic validation
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", cfg.Server.Port)
	}

	if cfg.Worker.PoolSize <= 0 {
		return fmt.Errorf("invalid worker pool size: %d", cfg.Worker.PoolSize)
	}

	return nil
}

func (hr *HotReloader) notifyCallbacks(event ReloadEvent) {
	hr.mu.RLock()
	callbacks := make([]ReloadCallback, len(hr.callbacks))
	copy(callbacks, hr.callbacks)
	hr.mu.RUnlock()

	for _, cb := range callbacks {
		go cb(event)
	}
}
