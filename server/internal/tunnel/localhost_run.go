// Package tunnel provides SSH-based tunnel implementations.
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

// LocalhostRunManager manages localhost.run tunnels via SSH.
type LocalhostRunManager struct {
	mu        sync.RWMutex
	running   bool
	url       string
	startedAt time.Time
	cmd       *exec.Cmd
	cancel    context.CancelFunc

	onURLChange func(url string)
	onError     func(err error)
}

// NewLocalhostRunManager creates a new localhost.run tunnel manager.
func NewLocalhostRunManager() *LocalhostRunManager {
	return &LocalhostRunManager{}
}

// Start starts the localhost.run tunnel.
func (m *LocalhostRunManager) Start(ctx context.Context, cfg *Config) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return fmt.Errorf("tunnel already running")
	}
	m.mu.Unlock()

	port := cfg.Port
	if port == 0 {
		port = 23456
	}

	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)

	// Start SSH tunnel to localhost.run
	// With subdomain: ssh -R subdomain:80:localhost:PORT nokey@localhost.run
	// Without: ssh -R 80:localhost:PORT nokey@localhost.run (service may assign e.g. admin)
	remoteSpec := fmt.Sprintf("80:localhost:%d", port)
	if cfg.Subdomain != "" {
		remoteSpec = fmt.Sprintf("%s:80:localhost:%d", cfg.Subdomain, port)
	}
	cmd := exec.CommandContext(ctx, "ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "ServerAliveInterval=60",
		"-R", remoteSpec,
		"nokey@localhost.run",
	)

	// Capture both stdout and stderr for URL extraction
	// localhost.run may output URL to either stream
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("failed to start ssh: %w", err)
	}

	m.mu.Lock()
	m.running = true
	m.cmd = cmd
	m.cancel = cancel
	m.startedAt = time.Now()
	m.mu.Unlock()

	// URL regex patterns - localhost.run uses various domains
	// Output format: "abc123.localhost.run tunneled with tls termination" or "https://abc123.lhr.life"
	// Match both with and without https:// prefix
	urlRegex := regexp.MustCompile(`(https?://)?([a-zA-Z0-9-]+\.(lhr\.life|localhost\.run))`)

	extractURL := func(line string) string {
		if matches := urlRegex.FindStringSubmatch(line); len(matches) > 2 {
			// Return full URL with https:// prefix
			return "https://" + matches[2]
		}
		return ""
	}

	// Parse URL from stdout in background
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			if url := extractURL(line); url != "" {
				m.mu.Lock()
				if m.url == "" {
					m.url = url
					m.mu.Unlock()
					if m.onURLChange != nil {
						m.onURLChange(url)
					}
				} else {
					m.mu.Unlock()
				}
				return
			}
		}
	}()

	// Parse URL from stderr in background
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			if url := extractURL(line); url != "" {
				m.mu.Lock()
				if m.url == "" {
					m.url = url
					m.mu.Unlock()
					if m.onURLChange != nil {
						m.onURLChange(url)
					}
				} else {
					m.mu.Unlock()
				}
				return
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
func (m *LocalhostRunManager) Stop() error {
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
func (m *LocalhostRunManager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// GetStatus returns the current tunnel status.
func (m *LocalhostRunManager) GetStatus() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return Status{
		Active:     m.running && m.url != "",
		Connecting: m.running && m.url == "",
		URL:        m.url,
		StartedAt:  m.startedAt,
		Provider:   ProviderLocalhostRun,
	}
}

// GetURL returns the current tunnel URL.
func (m *LocalhostRunManager) GetURL() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.url
}

// GetProvider returns the provider type.
func (m *LocalhostRunManager) GetProvider() Provider {
	return ProviderLocalhostRun
}

// SetOnURLChange sets a callback for URL changes.
func (m *LocalhostRunManager) SetOnURLChange(fn func(url string)) {
	m.onURLChange = fn
}

// SetOnError sets a callback for errors.
func (m *LocalhostRunManager) SetOnError(fn func(err error)) {
	m.onError = fn
}
