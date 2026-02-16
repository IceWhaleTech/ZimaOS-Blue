//go:build !embedded_cloudflared

package tunnel

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// CloudflareManager manages Cloudflare Tunnel connections via cloudflared subprocess.
// Auto-downloads the cloudflared binary if not found.
type CloudflareManager struct {
	mu        sync.RWMutex
	running   bool
	url       string
	startedAt time.Time
	cancel    context.CancelFunc
	cmd       *exec.Cmd

	sf singleflight.Group

	onURLChange func(url string)
	onError     func(err error)
}

func NewCloudflareManager() *CloudflareManager {
	return &CloudflareManager{}
}

func (m *CloudflareManager) Start(ctx context.Context, cfg *Config) error {
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

	_, err, _ := m.sf.Do("cloudflare-tunnel", func() (interface{}, error) {
		return m.startTunnelInternal(ctx, port)
	})
	return err
}

func (m *CloudflareManager) startTunnelInternal(ctx context.Context, port int) (interface{}, error) {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return nil, nil
	}
	m.mu.Unlock()

	bin, err := ensureCloudflared()
	if err != nil {
		return nil, fmt.Errorf("cloudflared not available: %w", err)
	}

	tunnelCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(tunnelCtx, bin, "tunnel", "--url", fmt.Sprintf("http://localhost:%d", port), "--no-autoupdate")
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start cloudflared: %w", err)
	}

	m.mu.Lock()
	m.running = true
	m.cancel = cancel
	m.cmd = cmd
	m.startedAt = time.Now()
	m.mu.Unlock()

	// Parse URL from stderr output
	urlCh := make(chan string, 1)
	go func() {
		re := regexp.MustCompile(`https://[a-zA-Z0-9-]+\.trycloudflare\.com`)
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			if match := re.FindString(line); match != "" {
				select {
				case urlCh <- match:
				default:
				}
			}
		}
	}()

	// Wait for URL or timeout
	select {
	case tunnelURL := <-urlCh:
		m.mu.Lock()
		m.url = tunnelURL
		m.mu.Unlock()
		if m.onURLChange != nil {
			m.onURLChange(tunnelURL)
		}
	case <-time.After(30 * time.Second):
		// Timeout but process may still be starting
	case <-tunnelCtx.Done():
		return nil, tunnelCtx.Err()
	}

	// Monitor process
	go func() {
		err := cmd.Wait()
		m.mu.Lock()
		m.running = false
		m.url = ""
		m.mu.Unlock()
		if err != nil && tunnelCtx.Err() == nil && m.onError != nil {
			m.onError(fmt.Errorf("cloudflared exited: %w", err))
		}
	}()

	return nil, nil
}

func (m *CloudflareManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return nil
	}
	if m.cancel != nil {
		m.cancel()
	}
	m.running = false
	m.url = ""
	return nil
}

func (m *CloudflareManager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

func (m *CloudflareManager) GetStatus() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return Status{Active: m.running && m.url != "", URL: m.url, StartedAt: m.startedAt, Provider: ProviderCloudflare}
}

func (m *CloudflareManager) GetURL() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.url
}

func (m *CloudflareManager) GetProvider() Provider    { return ProviderCloudflare }
func (m *CloudflareManager) SetOnURLChange(fn func(string)) { m.onURLChange = fn }
func (m *CloudflareManager) SetOnError(fn func(error))      { m.onError = fn }

// ensureCloudflared finds or downloads the cloudflared binary.
func ensureCloudflared() (string, error) {
	// Check PATH first
	if p, err := exec.LookPath("cloudflared"); err == nil {
		return p, nil
	}

	// Check local cache
	cacheDir, _ := os.UserCacheDir()
	binName := "cloudflared"
	if runtime.GOOS == "windows" {
		binName = "cloudflared.exe"
	}
	cached := filepath.Join(cacheDir, "zimaos-blue", binName)
	if _, err := os.Stat(cached); err == nil {
		return cached, nil
	}

	// Download
	dlURL := cloudflaredDownloadURL()
	if dlURL == "" {
		return "", fmt.Errorf("unsupported platform %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	os.MkdirAll(filepath.Dir(cached), 0o755)
	resp, err := http.Get(dlURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	tmp := cached + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return "", err
	}
	f.Close()
	os.Chmod(tmp, 0o755)
	if err := os.Rename(tmp, cached); err != nil {
		return "", err
	}
	return cached, nil
}

func cloudflaredDownloadURL() string {
	const base = "https://github.com/cloudflare/cloudflared/releases/latest/download/"
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "linux/amd64":
		return base + "cloudflared-linux-amd64"
	case "linux/arm64":
		return base + "cloudflared-linux-arm64"
	case "linux/arm":
		return base + "cloudflared-linux-arm"
	case "darwin/amd64":
		return base + "cloudflared-darwin-amd64.tgz"
	case "darwin/arm64":
		return base + "cloudflared-darwin-amd64.tgz"
	case "windows/amd64":
		return base + "cloudflared-windows-amd64.exe"
	default:
		return ""
	}
}
