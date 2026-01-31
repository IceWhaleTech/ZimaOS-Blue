package tunnel

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"sync"
	"time"
)

// CloudflareManager manages Cloudflare Tunnel connections.
type CloudflareManager struct {
	mu        sync.RWMutex
	running   bool
	url       string
	startedAt time.Time
	cmd       *exec.Cmd
	cancel    context.CancelFunc

	onURLChange func(url string)
	onError     func(err error)
}

// NewCloudflareManager creates a new Cloudflare Tunnel manager.
func NewCloudflareManager() *CloudflareManager {
	return &CloudflareManager{}
}

// Start starts the Cloudflare Tunnel.
func (m *CloudflareManager) Start(ctx context.Context, cfg *Config) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return fmt.Errorf("tunnel already running")
	}
	m.mu.Unlock()

	port := cfg.Port
	if port == 0 {
		port = 8080
	}

	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)

	var cmd *exec.Cmd

	if cfg.CloudflareToken != "" {
		// Use token-based authentication (recommended)
		// cloudflared tunnel run --token <TOKEN>
		cmd = exec.CommandContext(ctx, "cloudflared",
			"tunnel", "run",
			"--token", cfg.CloudflareToken,
		)
	} else {
		// Use quick tunnel (no account required, temporary URL)
		// cloudflared tunnel --url http://localhost:PORT
		cmd = exec.CommandContext(ctx, "cloudflared",
			"tunnel",
			"--url", fmt.Sprintf("http://localhost:%d", port),
		)
	}

	// Capture stderr for URL extraction
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("failed to start cloudflared: %w", err)
	}

	m.mu.Lock()
	m.running = true
	m.cmd = cmd
	m.cancel = cancel
	m.startedAt = time.Now()
	m.mu.Unlock()

	// Parse URL from output in background
	go func() {
		scanner := bufio.NewScanner(stderr)
		// Quick tunnel outputs: https://xxxxx.trycloudflare.com
		// Token-based outputs the configured domain
		urlRegex := regexp.MustCompile(`https?://[a-zA-Z0-9.-]+\.(trycloudflare\.com|[a-zA-Z0-9.-]+)`)

		for scanner.Scan() {
			line := scanner.Text()
			if matches := urlRegex.FindString(line); matches != "" {
				m.mu.Lock()
				m.url = matches
				m.mu.Unlock()

				if m.onURLChange != nil {
					m.onURLChange(matches)
				}
				// Don't break - cloudflared may output multiple URLs
			}
		}
	}()

	// Monitor process
	go func() {
		err := cmd.Wait()
		m.mu.Lock()
		m.running = false
		m.url = ""
		m.mu.Unlock()

		if err != nil && m.onError != nil {
			m.onError(err)
		}
	}()

	return nil
}

// Stop stops the tunnel.
func (m *CloudflareManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	if m.cancel != nil {
		m.cancel()
	}

	if m.cmd != nil && m.cmd.Process != nil {
		m.cmd.Process.Kill()
	}

	m.running = false
	m.url = ""
	return nil
}

// IsRunning returns true if the tunnel is running.
func (m *CloudflareManager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// GetStatus returns the current tunnel status.
func (m *CloudflareManager) GetStatus() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return Status{
		Active:    m.running && m.url != "",
		URL:       m.url,
		StartedAt: m.startedAt,
		Provider:  ProviderCloudflare,
	}
}

// GetURL returns the current tunnel URL.
func (m *CloudflareManager) GetURL() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.url
}

// GetProvider returns the provider type.
func (m *CloudflareManager) GetProvider() Provider {
	return ProviderCloudflare
}

// SetOnURLChange sets a callback for URL changes.
func (m *CloudflareManager) SetOnURLChange(fn func(url string)) {
	m.onURLChange = fn
}

// SetOnError sets a callback for errors.
func (m *CloudflareManager) SetOnError(fn func(err error)) {
	m.onError = fn
}

// CheckCloudflaredInstalled checks if cloudflared is installed.
func CheckCloudflaredInstalled() bool {
	_, err := exec.LookPath("cloudflared")
	return err == nil
}
