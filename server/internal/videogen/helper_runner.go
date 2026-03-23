package videogen

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	videoHelperEnvVar    = "BLUE_VIDEO_HELPER"
	videoHelperBinary    = "blue-video-helper"
	videoHelperSwiftPath = "blue-video-helper.swift"
)

// HelperRunner resolves and launches the native video helper.
type HelperRunner struct {
	helperPath string
}

// NewHelperRunner creates a helper-backed native video generator.
func NewHelperRunner(helperPath string) *HelperRunner {
	return &HelperRunner{helperPath: strings.TrimSpace(helperPath)}
}

// Status reports whether a usable helper implementation is available.
func (r *HelperRunner) Status() HelperStatus {
	path, mode, err := r.resolve()
	if err != nil {
		return HelperStatus{Available: false, Error: err.Error()}
	}
	return HelperStatus{Available: true, Mode: mode, Path: path}
}

// Available reports helper availability.
func (r *HelperRunner) Available() bool {
	return r.Status().Available
}

// Generate renders a native video and streams coarse progress by polling the status file.
func (r *HelperRunner) Generate(
	ctx context.Context,
	job *Job,
	pollInterval time.Duration,
	stallTimeout time.Duration,
	maxRuntime time.Duration,
	observer ProgressObserver,
) (*Result, error) {
	if job == nil {
		return nil, fmt.Errorf("video job is required")
	}
	helperPath, mode, err := r.resolve()
	if err != nil {
		return nil, err
	}

	requestFile, err := os.CreateTemp("", "blue-video-helper-*.json")
	if err != nil {
		return nil, fmt.Errorf("create helper request: %w", err)
	}
	requestPath := requestFile.Name()
	defer os.Remove(requestPath)

	encoded, err := json.Marshal(job)
	if err != nil {
		requestFile.Close()
		return nil, fmt.Errorf("encode video helper payload: %w", err)
	}
	if _, err := requestFile.Write(encoded); err != nil {
		requestFile.Close()
		return nil, fmt.Errorf("write video helper payload: %w", err)
	}
	if err := requestFile.Close(); err != nil {
		return nil, fmt.Errorf("close video helper payload: %w", err)
	}

	args := []string{requestPath}
	executable := helperPath
	if mode == "swift" {
		executable = "swift"
		args = []string{helperPath, requestPath}
	}

	cmd := exec.Command(executable, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start video helper: %w", err)
	}

	startedAt := time.Now()
	lastSignalAt := startedAt
	lastStatusRaw := ""
	if observer != nil {
		observer(Progress{
			Stage:     "starting",
			Progress:  0.05,
			Message:   "Starting native video helper",
			UpdatedAt: startedAt,
		})
	}

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	if pollInterval <= 0 {
		pollInterval = 2 * time.Second
	}
	if stallTimeout <= 0 {
		stallTimeout = 15 * time.Minute
	}
	if maxRuntime <= 0 {
		maxRuntime = 6 * time.Hour
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	killProcess := func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}

	for {
		select {
		case <-ctx.Done():
			killProcess()
			<-waitCh
			return nil, ctx.Err()
		case err := <-waitCh:
			if err != nil {
				msg := strings.TrimSpace(stderr.String())
				if msg == "" {
					msg = strings.TrimSpace(stdout.String())
				}
				if msg == "" {
					msg = err.Error()
				}
				return nil, fmt.Errorf("video helper failed: %s", msg)
			}
			var resp Result
			if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
				return nil, fmt.Errorf("parse video helper response: %w (%s)", err, strings.TrimSpace(stdout.String()))
			}
			if resp.OutputPath == "" {
				return nil, fmt.Errorf("video helper returned no output path")
			}
			if observer != nil {
				observer(Progress{
					Stage:     "completed",
					Progress:  1,
					Message:   "Native video helper completed",
					UpdatedAt: time.Now(),
				})
			}
			return &resp, nil
		case <-ticker.C:
			if maxRuntime > 0 && time.Since(startedAt) > maxRuntime {
				killProcess()
				<-waitCh
				return nil, fmt.Errorf("video helper exceeded max runtime of %s", maxRuntime)
			}
			progress, raw, ok := readProgressFile(job.StatusPath)
			if ok && raw != "" && raw != lastStatusRaw {
				lastStatusRaw = raw
				if progress.UpdatedAt.IsZero() {
					progress.UpdatedAt = time.Now()
				}
				lastSignalAt = progress.UpdatedAt
				if observer != nil {
					observer(progress)
				}
			}
			if stallTimeout > 0 && time.Since(lastSignalAt) > stallTimeout {
				killProcess()
				<-waitCh
				return nil, fmt.Errorf("video helper stalled for %s", stallTimeout)
			}
		}
	}
}

func readProgressFile(path string) (Progress, string, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return Progress{}, "", false
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return Progress{}, "", false
	}
	raw := strings.TrimSpace(string(data))
	var progress Progress
	if err := json.Unmarshal(data, &progress); err != nil {
		return Progress{}, raw, false
	}
	return progress, raw, true
}

func (r *HelperRunner) resolve() (string, string, error) {
	if runtime.GOOS != "darwin" {
		return "", "", fmt.Errorf("native video helper is only available on macOS")
	}
	if p := strings.TrimSpace(r.helperPath); p != "" {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p, "binary", nil
		}
	}
	if envPath := strings.TrimSpace(os.Getenv(videoHelperEnvVar)); envPath != "" {
		if info, err := os.Stat(envPath); err == nil && !info.IsDir() {
			return envPath, "binary", nil
		}
	}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates := []string{
			filepath.Join(exeDir, videoHelperBinary),
			filepath.Join(exeDir, "..", "Resources", videoHelperBinary),
			filepath.Join(exeDir, "..", "..", "Resources", videoHelperBinary),
		}
		for _, candidate := range candidates {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate, "binary", nil
			}
		}
	}
	if _, err := exec.LookPath("swift"); err != nil {
		return "", "", fmt.Errorf("swift helper unavailable: %w", err)
	}
	wd, _ := os.Getwd()
	candidates := []string{
		filepath.Join(wd, "server", "internal", "videogen", "helper", videoHelperSwiftPath),
		filepath.Join(wd, "internal", "videogen", "helper", videoHelperSwiftPath),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, "swift", nil
		}
	}
	return "", "", fmt.Errorf("%s not found", videoHelperBinary)
}

var _ Generator = (*HelperRunner)(nil)
