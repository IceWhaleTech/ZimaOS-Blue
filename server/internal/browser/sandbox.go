package browser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// SandboxConfig contains sandbox configuration options.
type SandboxConfig struct {
	// Enabled enables sandbox mode.
	Enabled bool `json:"enabled" yaml:"enabled"`
	// TempDir is the base directory for temporary files.
	TempDir string `json:"temp_dir" yaml:"temp_dir"`
	// MaxMemoryMB is the maximum memory limit in MB (0 = unlimited).
	MaxMemoryMB int `json:"max_memory_mb" yaml:"max_memory_mb"`
	// MaxCPUPercent is the maximum CPU usage percentage (0 = unlimited).
	MaxCPUPercent int `json:"max_cpu_percent" yaml:"max_cpu_percent"`
	// NetworkIsolation enables network isolation.
	NetworkIsolation bool `json:"network_isolation" yaml:"network_isolation"`
	// AllowedHosts is a list of allowed hosts when network isolation is enabled.
	AllowedHosts []string `json:"allowed_hosts" yaml:"allowed_hosts"`
	// DisableGPU disables GPU acceleration.
	DisableGPU bool `json:"disable_gpu" yaml:"disable_gpu"`
	// DisableJavaScript disables JavaScript execution.
	DisableJavaScript bool `json:"disable_javascript" yaml:"disable_javascript"`
	// DisableImages disables image loading.
	DisableImages bool `json:"disable_images" yaml:"disable_images"`
	// DisablePlugins disables browser plugins.
	DisablePlugins bool `json:"disable_plugins" yaml:"disable_plugins"`
	// DisablePopups disables popup windows.
	DisablePopups bool `json:"disable_popups" yaml:"disable_popups"`
	// DisableDownloads disables file downloads.
	DisableDownloads bool `json:"disable_downloads" yaml:"disable_downloads"`
	// CleanupOnExit removes temporary files on exit.
	CleanupOnExit bool `json:"cleanup_on_exit" yaml:"cleanup_on_exit"`
	// SessionTimeout is the maximum session duration.
	SessionTimeout time.Duration `json:"session_timeout" yaml:"session_timeout"`
}

// DefaultSandboxConfig returns the default sandbox configuration.
func DefaultSandboxConfig() *SandboxConfig {
	return &SandboxConfig{
		Enabled:          true,
		TempDir:          filepath.Join(os.TempDir(), "zimaos-browser-sandbox"),
		MaxMemoryMB:      512,
		MaxCPUPercent:    50,
		NetworkIsolation: false,
		AllowedHosts:     []string{},
		DisableGPU:       true,
		DisableJavaScript: false,
		DisableImages:    false,
		DisablePlugins:   true,
		DisablePopups:    true,
		DisableDownloads: true,
		CleanupOnExit:    true,
		SessionTimeout:   30 * time.Minute,
	}
}

// Sandbox manages browser isolation and resource limits.
type Sandbox struct {
	config    *SandboxConfig
	sessions  map[string]*SandboxSession
	mu        sync.RWMutex
	cleanupCh chan struct{}
}

// SandboxSession represents an isolated browser session.
type SandboxSession struct {
	// ID is the unique session identifier.
	ID string `json:"id"`
	// ProfilePath is the path to the browser profile.
	ProfilePath string `json:"profile_path"`
	// DataDir is the path to the session data directory.
	DataDir string `json:"data_dir"`
	// CreatedAt is when the session was created.
	CreatedAt time.Time `json:"created_at"`
	// ExpiresAt is when the session expires.
	ExpiresAt time.Time `json:"expires_at"`
	// Active indicates if the session is active.
	Active bool `json:"active"`
	// ResourceUsage tracks resource usage.
	ResourceUsage *ResourceUsage `json:"resource_usage"`
}

// ResourceUsage tracks resource consumption.
type ResourceUsage struct {
	// MemoryMB is the current memory usage in MB.
	MemoryMB int `json:"memory_mb"`
	// CPUPercent is the current CPU usage percentage.
	CPUPercent float64 `json:"cpu_percent"`
	// NetworkBytesSent is the total bytes sent.
	NetworkBytesSent int64 `json:"network_bytes_sent"`
	// NetworkBytesReceived is the total bytes received.
	NetworkBytesReceived int64 `json:"network_bytes_received"`
	// LastUpdated is when the usage was last updated.
	LastUpdated time.Time `json:"last_updated"`
}

// NewSandbox creates a new sandbox manager.
func NewSandbox(config *SandboxConfig) (*Sandbox, error) {
	if config == nil {
		config = DefaultSandboxConfig()
	}

	// Create temp directory if it doesn't exist
	if err := os.MkdirAll(config.TempDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create sandbox temp dir: %w", err)
	}

	s := &Sandbox{
		config:    config,
		sessions:  make(map[string]*SandboxSession),
		cleanupCh: make(chan struct{}),
	}

	// Start cleanup goroutine
	go s.cleanupLoop()

	return s, nil
}

// CreateSession creates a new isolated browser session.
func (s *Sandbox) CreateSession(ctx context.Context, sessionID string) (*SandboxSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if session already exists
	if _, exists := s.sessions[sessionID]; exists {
		return nil, fmt.Errorf("session %s already exists", sessionID)
	}

	// Create session directories
	sessionDir := filepath.Join(s.config.TempDir, sessionID)
	profilePath := filepath.Join(sessionDir, "profile")
	dataDir := filepath.Join(sessionDir, "data")

	if err := os.MkdirAll(profilePath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create profile dir: %w", err)
	}
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create data dir: %w", err)
	}

	now := timeutil.NowTime()
	session := &SandboxSession{
		ID:          sessionID,
		ProfilePath: profilePath,
		DataDir:     dataDir,
		CreatedAt:   now,
		ExpiresAt:   now.Add(s.config.SessionTimeout),
		Active:      true,
		ResourceUsage: &ResourceUsage{
			LastUpdated: now,
		},
	}

	s.sessions[sessionID] = session
	return session, nil
}

// GetSession returns a session by ID.
func (s *Sandbox) GetSession(sessionID string) (*SandboxSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	return session, nil
}

// DestroySession destroys a session and cleans up resources.
func (s *Sandbox) DestroySession(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	// Clean up session directory
	sessionDir := filepath.Join(s.config.TempDir, sessionID)
	if s.config.CleanupOnExit {
		if err := os.RemoveAll(sessionDir); err != nil {
			// Log error but don't fail
			fmt.Printf("warning: failed to cleanup session dir: %v\n", err)
		}
	}

	session.Active = false
	delete(s.sessions, sessionID)
	return nil
}

// ListSessions returns all active sessions.
func (s *Sandbox) ListSessions() []*SandboxSession {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessions := make([]*SandboxSession, 0, len(s.sessions))
	for _, session := range s.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}

// GetBrowserArgs returns browser launch arguments for sandbox mode.
func (s *Sandbox) GetBrowserArgs(session *SandboxSession) []string {
	args := []string{
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-background-networking",
		"--disable-sync",
		"--disable-translate",
		"--metrics-recording-only",
		"--safebrowsing-disable-auto-update",
		"--disable-component-update",
		"--disable-default-apps",
		"--disable-extensions",
		"--disable-features=TranslateUI",
		"--disable-ipc-flooding-protection",
		"--disable-renderer-backgrounding",
		"--enable-features=NetworkService,NetworkServiceInProcess",
		"--force-color-profile=srgb",
		"--disable-dev-shm-usage",
	}

	// Set user data directory
	args = append(args, fmt.Sprintf("--user-data-dir=%s", session.ProfilePath))

	// Sandbox-specific flags
	if s.config.Enabled {
		// Enable Chrome's built-in sandbox
		if runtime.GOOS == "linux" {
			args = append(args, "--no-sandbox") // Required for Docker/containers
		}
		args = append(args, "--disable-setuid-sandbox")
	}

	// GPU settings
	if s.config.DisableGPU {
		args = append(args, "--disable-gpu")
		args = append(args, "--disable-software-rasterizer")
	}

	// JavaScript settings
	if s.config.DisableJavaScript {
		args = append(args, "--disable-javascript")
	}

	// Image settings
	if s.config.DisableImages {
		args = append(args, "--blink-settings=imagesEnabled=false")
	}

	// Plugin settings
	if s.config.DisablePlugins {
		args = append(args, "--disable-plugins")
		args = append(args, "--disable-plugins-discovery")
	}

	// Popup settings
	if s.config.DisablePopups {
		args = append(args, "--disable-popup-blocking")
		args = append(args, "--block-new-web-contents")
	}

	// Download settings
	if s.config.DisableDownloads {
		args = append(args, "--disable-features=DownloadBubble,DownloadBubbleV2")
	}

	// Memory limit (approximate via renderer process limit)
	if s.config.MaxMemoryMB > 0 {
		args = append(args, fmt.Sprintf("--renderer-process-limit=%d", s.config.MaxMemoryMB/128))
	}

	return args
}

// CheckResourceLimits checks if resource limits are exceeded.
func (s *Sandbox) CheckResourceLimits(session *SandboxSession) error {
	if session.ResourceUsage == nil {
		return nil
	}

	if s.config.MaxMemoryMB > 0 && session.ResourceUsage.MemoryMB > s.config.MaxMemoryMB {
		return ErrResourceLimitExceeded
	}

	if s.config.MaxCPUPercent > 0 && session.ResourceUsage.CPUPercent > float64(s.config.MaxCPUPercent) {
		return ErrResourceLimitExceeded
	}

	return nil
}

// UpdateResourceUsage updates the resource usage for a session.
func (s *Sandbox) UpdateResourceUsage(sessionID string, usage *ResourceUsage) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	usage.LastUpdated = timeutil.NowTime()
	session.ResourceUsage = usage
	return nil
}

// cleanupLoop periodically cleans up expired sessions.
func (s *Sandbox) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.cleanupExpiredSessions()
		case <-s.cleanupCh:
			return
		}
	}
}

// cleanupExpiredSessions removes expired sessions.
func (s *Sandbox) cleanupExpiredSessions() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := timeutil.NowTime()
	for id, session := range s.sessions {
		if now.After(session.ExpiresAt) {
			// Clean up session directory
			sessionDir := filepath.Join(s.config.TempDir, id)
			if s.config.CleanupOnExit {
				os.RemoveAll(sessionDir)
			}
			delete(s.sessions, id)
		}
	}
}

// Close closes the sandbox and cleans up all sessions.
func (s *Sandbox) Close() error {
	close(s.cleanupCh)

	s.mu.Lock()
	defer s.mu.Unlock()

	// Clean up all sessions
	if s.config.CleanupOnExit {
		for id := range s.sessions {
			sessionDir := filepath.Join(s.config.TempDir, id)
			os.RemoveAll(sessionDir)
		}
	}

	s.sessions = make(map[string]*SandboxSession)
	return nil
}

// IsNetworkAllowed checks if a host is allowed under network isolation.
func (s *Sandbox) IsNetworkAllowed(host string) bool {
	if !s.config.NetworkIsolation {
		return true
	}

	for _, allowed := range s.config.AllowedHosts {
		if host == allowed {
			return true
		}
	}

	return false
}
