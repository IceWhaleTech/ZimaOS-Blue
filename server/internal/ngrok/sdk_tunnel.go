package ngrok

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

// SDKTunnelManager manages ngrok tunnels via ngrok subprocess.
// Auto-downloads the ngrok binary if not found.
type SDKTunnelManager struct {
	repository *Repository
	authtoken  string

	mu           sync.RWMutex
	running      bool
	connecting   bool
	url          string
	startedAt    time.Time
	expiresAt    time.Time
	renewedCount int
	sessionID    string
	cancelFunc   context.CancelFunc
	cmd          *exec.Cmd

	OnURLChange func(url string)
	OnError     func(err error)
}

func NewSDKTunnelManager(repo *Repository) *SDKTunnelManager {
	return &SDKTunnelManager{repository: repo}
}

func (tm *SDKTunnelManager) Start(ctx context.Context, port int, authtoken string) error {
	tm.mu.Lock()
	if tm.running {
		tm.mu.Unlock()
		return ErrTunnelAlreadyRunning
	}
	tm.mu.Unlock()

	bin, err := ensureNgrok()
	if err != nil {
		return fmt.Errorf("ngrok not available: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)

	args := []string{"http", fmt.Sprintf("%d", port), "--log", "stdout", "--log-format", "term"}
	if authtoken != "" {
		args = append(args, "--authtoken", authtoken)
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
			if m := re.FindStringSubmatch(scanner.Text()); len(m) > 1 {
				select {
				case urlCh <- m[1]:
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

	tm.mu.Lock()
	tm.running = true
	tm.connecting = false
	tm.url = url
	tm.authtoken = authtoken
	tm.cancelFunc = cancel
	tm.cmd = cmd
	tm.startedAt = time.Now()
	tm.expiresAt = calculateExpiresAt(tm.startedAt)
	tm.mu.Unlock()

	if tm.repository != nil {
		session := &RemoteAccessSession{TunnelURL: url, StartedAt: tm.startedAt, ExpiresAt: tm.expiresAt, Status: "active"}
		if sid, err := tm.repository.CreateSession(ctx, session); err == nil {
			tm.mu.Lock()
			tm.sessionID = sid
			tm.mu.Unlock()
		}
	}
	if tm.OnURLChange != nil {
		tm.OnURLChange(url)
	}
	go tm.monitorTunnel(ctx)
	return nil
}

func (tm *SDKTunnelManager) monitorTunnel(ctx context.Context) {
	if tm.cmd != nil {
		tm.cmd.Wait()
	} else {
		<-ctx.Done()
	}
	tm.mu.Lock()
	sessionID := tm.sessionID
	tm.running = false
	tm.connecting = false
	tm.mu.Unlock()
	if tm.repository != nil && sessionID != "" {
		tm.repository.EndSession(context.Background(), sessionID, "stopped", "")
	}
}

func (tm *SDKTunnelManager) Stop() error {
	tm.mu.Lock()
	sessionID := tm.sessionID
	wasRunning := tm.running
	cancelFunc := tm.cancelFunc
	tm.mu.Unlock()
	if !wasRunning {
		return nil
	}
	if cancelFunc != nil {
		cancelFunc()
	}
	tm.mu.Lock()
	tm.running = false
	tm.connecting = false
	tm.url = ""
	tm.cmd = nil
	tm.cancelFunc = nil
	tm.mu.Unlock()
	if tm.repository != nil && sessionID != "" {
		tm.repository.EndSession(context.Background(), sessionID, "stopped", "")
	}
	return nil
}

func (tm *SDKTunnelManager) IsRunning() bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.running
}

func (tm *SDKTunnelManager) GetStatus() TunnelStatus {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	if !tm.running {
		return TunnelStatus{Active: false}
	}
	return TunnelStatus{
		Active: true, Connecting: tm.connecting, URL: tm.url,
		StartedAt: tm.startedAt, ExpiresAt: tm.expiresAt,
		RemainingTime: formatRemainingTime(calculateRemainingTime(tm.expiresAt)),
		RenewedCount:  tm.renewedCount,
	}
}

func (tm *SDKTunnelManager) GetURL() string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.url
}

func (tm *SDKTunnelManager) IncrementRenewedCount() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.renewedCount++
}

func (tm *SDKTunnelManager) ResetExpiry() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.startedAt = time.Now()
	tm.expiresAt = calculateExpiresAt(tm.startedAt)
}

func ensureNgrok() (string, error) {
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
	dlURL := ngrokDownloadURL()
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

func ngrokDownloadURL() string {
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
