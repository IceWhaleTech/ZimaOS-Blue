package ngrok

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// ErrNgrokNotInstalled is returned when ngrok is not installed.
var ErrNgrokNotInstalled = errors.New("ngrok is not installed")

// ErrTunnelAlreadyRunning is returned when trying to start a tunnel that's already running.
var ErrTunnelAlreadyRunning = errors.New("tunnel is already running")

// ErrTunnelNotRunning is returned when trying to stop a tunnel that's not running.
var ErrTunnelNotRunning = errors.New("tunnel is not running")

// TunnelStatus represents the current tunnel status.
type TunnelStatus struct {
	Active        bool      `json:"active"`
	Connecting    bool      `json:"connecting,omitempty"`
	URL           string    `json:"url,omitempty"`
	StartedAt     time.Time `json:"started_at,omitempty"`
	ExpiresAt     time.Time `json:"expires_at,omitempty"`
	RemainingTime string    `json:"remaining_time,omitempty"`
	RenewedCount  int       `json:"renewed_count"`
}

// NgrokStatus represents ngrok installation status (from PATH).
type NgrokStatus struct {
	Installed bool   `json:"installed"`
	Version   string `json:"version,omitempty"`
	Path      string `json:"path,omitempty"`
}

// GetNgrokStatus returns ngrok installation status by checking PATH.
func GetNgrokStatus(ctx context.Context) (*NgrokStatus, error) {
	path, err := exec.LookPath("ngrok")
	if err != nil {
		return &NgrokStatus{Installed: false}, nil
	}
	cmd := exec.CommandContext(ctx, path, "version")
	output, err := cmd.Output()
	if err != nil {
		return &NgrokStatus{Installed: true, Path: path}, nil
	}
	version := strings.TrimSpace(string(output))
	return &NgrokStatus{Installed: true, Version: version, Path: path}, nil
}

// TunnelManager manages ngrok tunnel lifecycle.
type TunnelManager struct {
	repository *Repository
	port       int
	authtoken  string

	mu           sync.RWMutex
	running      bool
	connecting   bool
	url          string
	startedAt    time.Time
	expiresAt    time.Time
	renewedCount int
	sessionID    string
	cmd          *exec.Cmd
	cancelFunc   context.CancelFunc

	// Callbacks
	OnURLChange func(url string)
	OnError     func(err error)
}

// NewTunnelManager creates a new tunnel manager (ngrok must be in PATH).
func NewTunnelManager() *TunnelManager {
	return &TunnelManager{}
}

// NewTunnelManagerWithRepo creates a new tunnel manager with repository.
func NewTunnelManagerWithRepo(repo *Repository) *TunnelManager {
	return &TunnelManager{
		repository: repo,
	}
}

// Start starts the ngrok tunnel.
func (tm *TunnelManager) Start(ctx context.Context, port int, authtoken string) error {
	tm.mu.Lock()
	if tm.running {
		tm.mu.Unlock()
		return ErrTunnelAlreadyRunning
	}
	tm.mu.Unlock()

	// Check if ngrok is in PATH
	ngrokPath, err := exec.LookPath("ngrok")
	if err != nil {
		return ErrNgrokNotInstalled
	}

	// Try to add firewall exception (Windows only, requires admin)
	// This is best-effort and won't fail if it doesn't work
	_ = AddFirewallException(ngrokPath)

	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)

	// Build command arguments
	args := []string{"http", "--log", "stdout", "--log-format", "json"}
	if authtoken != "" {
		args = append(args, "--authtoken", authtoken)
	}
	args = append(args, fmt.Sprintf("%d", port))

	cmd := exec.CommandContext(ctx, ngrokPath, args...)

	// Get stdout pipe for parsing URL
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	// Get stderr pipe for error logging
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("failed to start ngrok: %w", err)
	}

	tm.mu.Lock()
	tm.running = true
	tm.connecting = true
	tm.port = port
	tm.authtoken = authtoken
	tm.cmd = cmd
	tm.cancelFunc = cancel
	tm.startedAt = time.Now()
	tm.expiresAt = calculateExpiresAt(tm.startedAt)
	tm.mu.Unlock()

	// Create session record in database if repository is available
	if tm.repository != nil {
		session := &RemoteAccessSession{
			TunnelURL:    "", // URL not available yet
			StartedAt:    tm.startedAt,
			ExpiresAt:    tm.expiresAt,
			RenewedCount: 0,
			Status:       "connecting",
		}
		sessionID, err := tm.repository.CreateSession(ctx, session)
		if err == nil {
			tm.mu.Lock()
			tm.sessionID = sessionID
			tm.mu.Unlock()
		}
	}

	// Parse URL from output in background
	go tm.parseOutput(stdout)

	// Capture stderr for error logging
	go tm.captureStderr(stderr)

	// Monitor process in background
	go tm.monitorProcess(cmd)

	return nil
}

// parseOutput parses ngrok JSON output to extract the tunnel URL.
func (tm *TunnelManager) parseOutput(stdout interface{ Read([]byte) (int, error) }) {
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		url, err := parseNgrokURL(line)
		if err == nil && url != "" {
			tm.mu.Lock()
			oldURL := tm.url
			tm.url = url
			tm.connecting = false // URL is now available, no longer connecting
			sessionID := tm.sessionID
			tm.mu.Unlock()

			// Update session in database with URL and active status
			if tm.repository != nil && sessionID != "" {
				session := &RemoteAccessSession{
					ID:           sessionID,
					TunnelURL:    url,
					StartedAt:    tm.startedAt,
					ExpiresAt:    tm.expiresAt,
					RenewedCount: tm.renewedCount,
					Status:       "active",
				}
				tm.repository.UpdateSession(context.Background(), session)
			}

			if oldURL != url && tm.OnURLChange != nil {
				tm.OnURLChange(url)
			}
		}
	}
}

// captureStderr captures stderr output from ngrok for error logging.
func (tm *TunnelManager) captureStderr(stderr interface{ Read([]byte) (int, error) }) {
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		// Log stderr output (errors, warnings, etc.)
		// This helps diagnose issues like antivirus blocking, firewall issues, etc.
		if line != "" {
			tm.mu.Lock()
			sessionID := tm.sessionID
			tm.mu.Unlock()

			// Log to database if repository is available
			if tm.repository != nil && sessionID != "" {
				tm.repository.AddLog(context.Background(), sessionID, "stderr", line, nil)
			}
		}
	}
}

// monitorProcess monitors the ngrok process and handles exit.
func (tm *TunnelManager) monitorProcess(cmd *exec.Cmd) {
	err := cmd.Wait()

	tm.mu.Lock()
	sessionID := tm.sessionID
	tm.running = false
	tm.connecting = false
	tm.cmd = nil
	tm.mu.Unlock()

	// Mark session as ended in database
	if tm.repository != nil && sessionID != "" {
		status := "stopped"
		errorMsg := ""
		if err != nil {
			status = "error"
			errorMsg = err.Error()
		}
		tm.repository.EndSession(context.Background(), sessionID, status, errorMsg)
	}

	if err != nil && tm.OnError != nil {
		tm.OnError(err)
	}
}

// Stop stops the ngrok tunnel.
func (tm *TunnelManager) Stop() error {
	tm.mu.Lock()
	sessionID := tm.sessionID
	wasRunning := tm.running
	tm.mu.Unlock()

	if !wasRunning {
		return nil // Not an error to stop when not running
	}

	tm.mu.Lock()
	if tm.cancelFunc != nil {
		tm.cancelFunc()
		tm.cancelFunc = nil
	}

	if tm.cmd != nil && tm.cmd.Process != nil {
		tm.cmd.Process.Kill()
	}

	tm.running = false
	tm.connecting = false
	tm.url = ""
	tm.cmd = nil
	tm.mu.Unlock()

	// Mark session as stopped in database
	if tm.repository != nil && sessionID != "" {
		tm.repository.EndSession(context.Background(), sessionID, "stopped", "")
	}

	return nil
}

// IsRunning returns true if the tunnel is running.
func (tm *TunnelManager) IsRunning() bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.running
}

// GetStatus returns the current tunnel status.
func (tm *TunnelManager) GetStatus() TunnelStatus {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if !tm.running {
		return TunnelStatus{Active: false}
	}

	remaining := calculateRemainingTime(tm.expiresAt)

	return TunnelStatus{
		Active:        tm.running,
		Connecting:    tm.connecting,
		URL:           tm.url,
		StartedAt:     tm.startedAt,
		ExpiresAt:     tm.expiresAt,
		RemainingTime: formatRemainingTime(remaining),
		RenewedCount:  tm.renewedCount,
	}
}

// GetURL returns the current tunnel URL.
func (tm *TunnelManager) GetURL() string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.url
}

// IncrementRenewedCount increments the renewal counter.
func (tm *TunnelManager) IncrementRenewedCount() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.renewedCount++
}

// ResetExpiry resets the expiry time (called after renewal).
func (tm *TunnelManager) ResetExpiry() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.startedAt = time.Now()
	tm.expiresAt = calculateExpiresAt(tm.startedAt)
}

// ngrokLogEntry represents a JSON log entry from ngrok.
type ngrokLogEntry struct {
	Level   string `json:"lvl"`
	Message string `json:"msg"`
	URL     string `json:"url"`
	Addr    string `json:"addr"`
}

// parseNgrokURL parses the tunnel URL from ngrok JSON log output.
func parseNgrokURL(line string) (string, error) {
	if line == "" {
		return "", errors.New("empty line")
	}

	var entry ngrokLogEntry
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		return "", err
	}

	if entry.URL == "" {
		return "", errors.New("no URL in log entry")
	}

	return entry.URL, nil
}

// calculateExpiresAt calculates the expiry time (8 hours from start).
func calculateExpiresAt(startTime time.Time) time.Time {
	return startTime.Add(8 * time.Hour)
}

// calculateRemainingTime calculates the remaining time until expiry.
func calculateRemainingTime(expiresAt time.Time) time.Duration {
	return time.Until(expiresAt)
}

// formatRemainingTime formats the remaining time as a human-readable string.
func formatRemainingTime(d time.Duration) string {
	if d < 0 {
		return "expired"
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}

	return fmt.Sprintf("%dm", minutes)
}
