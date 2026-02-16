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
)

// NgrokManager manages ngrok tunnels by spawning the ngrok binary.
type NgrokManager struct {
	mu           sync.RWMutex
	running      bool
	connecting   bool
	url          string
	startedAt    time.Time
	expiresAt    time.Time
	renewedCount int
	cancelFunc   context.CancelFunc
	cmd          *exec.Cmd

	onURLChange func(url string)
	onError     func(err error)
}

// NewNgrokManager creates a new ngrok tunnel manager.
func NewNgrokManager() *NgrokManager {
	return &NgrokManager{}
}

// Start starts the ngrok tunnel by spawning the ngrok binary.
func (m *NgrokManager) Start(ctx context.Context, cfg *Config) error {
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

	bin, err := ensureNgrokBinary()
	if err != nil {
		return fmt.Errorf("ngrok not available: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)

	args := []string{"http", fmt.Sprintf("%d", port), "--log", "stdout", "--log-format", "term"}
	if cfg.NgrokAuthtoken != "" {
		args = append(args, "--authtoken", cfg.NgrokAuthtoken)
	}
	if cfg.NgrokDomain != "" {
		args = append(args, "--domain", cfg.NgrokDomain)
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return err
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("failed to start ngrok: %w", err)
	}

	urlCh := make(chan string, 1)
	go func() {
		re := regexp.MustCompile(`url=(https://[a-zA-Z0-9._-]+\.ngrok[a-zA-Z0-9._-]*)`)
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if matches := re.FindStringSubmatch(scanner.Text()); len(matches) > 1 {
				select {
				case urlCh <- matches[1]:
				default:
				}
			}
		}
	}()

	var url string
	select {
	case url = <-urlCh:
	case <-time.After(15 * time.Second):
		cancel()
		return fmt.Errorf("ngrok: timeout waiting for URL")
	case <-ctx.Done():
		cancel()
		return ctx.Err()
	}

	m.mu.Lock()
	m.running = true
	m.connecting = false
	m.url = url
	m.cmd = cmd
	m.cancelFunc = cancel
	m.startedAt = time.Now()
	m.expiresAt = m.startedAt.Add(8 * time.Hour)
	m.mu.Unlock()

	if m.onURLChange != nil {
		m.onURLChange(url)
	}

	go m.monitorTunnel(ctx)

	return nil
}

// monitorTunnel monitors the tunnel process.
func (m *NgrokManager) monitorTunnel(ctx context.Context) {
	if m.cmd != nil {
		m.cmd.Wait()
	} else {
		<-ctx.Done()
	}

	m.mu.Lock()
	m.running = false
	m.connecting = false
	m.mu.Unlock()
}

// Stop stops the tunnel.
func (m *NgrokManager) Stop() error {
	m.mu.Lock()
	wasRunning := m.running
	cancelFunc := m.cancelFunc
	m.mu.Unlock()

	if !wasRunning {
		return nil
	}

	if cancelFunc != nil {
		cancelFunc()
	}

	m.mu.Lock()
	m.running = false
	m.connecting = false
	m.url = ""
	m.cmd = nil
	m.cancelFunc = nil
	m.mu.Unlock()

	return nil
}

// IsRunning returns true if the tunnel is running.
func (m *NgrokManager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// GetStatus returns the current tunnel status.
func (m *NgrokManager) GetStatus() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.running {
		return Status{Active: false, Provider: ProviderNgrok}
	}

	remaining := m.expiresAt.Sub(time.Now())
	remainingStr := ""
	if remaining > 0 {
		hours := int(remaining.Hours())
		minutes := int(remaining.Minutes()) % 60
		if hours > 0 {
			remainingStr = fmt.Sprintf("%dh %dm", hours, minutes)
		} else {
			remainingStr = fmt.Sprintf("%dm", minutes)
		}
	} else {
		remainingStr = "expired"
	}

	return Status{
		Active:        m.running,
		Connecting:    m.connecting,
		URL:           m.url,
		StartedAt:     m.startedAt,
		ExpiresAt:     m.expiresAt,
		RemainingTime: remainingStr,
		RenewedCount:  m.renewedCount,
		Provider:      ProviderNgrok,
	}
}

// GetURL returns the current tunnel URL.
func (m *NgrokManager) GetURL() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.url
}

// GetProvider returns the provider type.
func (m *NgrokManager) GetProvider() Provider {
	return ProviderNgrok
}

// SetOnURLChange sets a callback for URL changes.
func (m *NgrokManager) SetOnURLChange(fn func(url string)) {
	m.onURLChange = fn
}

// SetOnError sets a callback for errors.
func (m *NgrokManager) SetOnError(fn func(err error)) {
	m.onError = fn
}

// IncrementRenewedCount increments the renewal counter.
func (m *NgrokManager) IncrementRenewedCount() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.renewedCount++
}

// ResetExpiry resets the expiry time.
func (m *NgrokManager) ResetExpiry() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startedAt = time.Now()
	m.expiresAt = m.startedAt.Add(8 * time.Hour)
}

// ensureNgrokBinary finds or downloads the ngrok binary.
func ensureNgrokBinary() (string, error) {
	if p, err := exec.LookPath("ngrok"); err == nil {
		return p, nil
	}
	cacheDir, _ := os.UserCacheDir()
	binName := "ngrok"
	if runtime.GOOS == "windows" {
		binName = "ngrok.exe"
	}
	cached := filepath.Join(cacheDir, "zimaos-blue", binName)
	if _, err := os.Stat(cached); err == nil {
		return cached, nil
	}
	dlURL := ngrokBinaryDownloadURL()
	if dlURL == "" {
		return "", fmt.Errorf("unsupported platform %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	os.MkdirAll(filepath.Dir(cached), 0o755)
	resp, err := http.Get(dlURL) //nolint:gosec // trusted URL
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

func ngrokBinaryDownloadURL() string {
	const base = "https://bin.equinox.io/c/bNyj1mQVY4c/ngrok-v3-stable-"
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "linux/amd64":
		return base + "linux-amd64.tgz"
	case "linux/arm64":
		return base + "linux-arm64.tgz"
	case "darwin/amd64":
		return base + "darwin-amd64.zip"
	case "darwin/arm64":
		return base + "darwin-arm64.zip"
	case "windows/amd64":
		return base + "windows-amd64.zip"
	default:
		return ""
	}
}
