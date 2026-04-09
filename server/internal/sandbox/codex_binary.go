package sandbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	blueCodexGitHubLatestDownload           = "https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest/download"
	blueCodexMirrorGhProxyLatestDownload    = "https://mirror.ghproxy.com/https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest/download"
	blueCodexMirrorKKGitHubLatestDownload   = "https://kkgithub.com/IceWhaleTech/ZimaOS-Blue/releases/latest/download"
	openAICodexGitHubLatestDownload         = "https://github.com/openai/codex/releases/latest/download"
	openAICodexMirrorGhProxyLatestDownload  = "https://mirror.ghproxy.com/https://github.com/openai/codex/releases/latest/download"
	openAICodexMirrorKKGitHubLatestDownload = "https://kkgithub.com/openai/codex/releases/latest/download"
	defaultCodexDownloadTimeout             = 20 * time.Minute
)

type codexBinaryResolver interface {
	ReadyPath() (string, bool, error)
	Ensure(ctx context.Context) (string, error)
	Download(ctx context.Context) (string, error)
	AutoDownloadEnabled() bool
}

type codexBinaryDownloadCandidate struct {
	AssetName string
	URL       string
}

// CodexBinaryManagerOptions allows test-time injection for managed Codex
// binary resolution and download.
type CodexBinaryManagerOptions struct {
	HTTPClient      *http.Client
	LookPath        func(string) (string, error)
	UserCacheDir    func() (string, error)
	GOOS            string
	GOARCH          string
	DownloadSources []string
}

// CodexBinaryManager resolves or downloads the Windows Codex binary Blue uses
// for the strong sandbox tier.
type CodexBinaryManager struct {
	config                  *Config
	httpClient              *http.Client
	lookPath                func(string) (string, error)
	userCacheDir            func() (string, error)
	goos                    string
	goarch                  string
	downloadSourcesOverride []string
}

func NewCodexBinaryManager(config *Config, options CodexBinaryManagerOptions) *CodexBinaryManager {
	if config == nil {
		config = DefaultConfig()
	}
	manager := &CodexBinaryManager{
		config:                  config,
		httpClient:              options.HTTPClient,
		lookPath:                options.LookPath,
		userCacheDir:            options.UserCacheDir,
		goos:                    strings.TrimSpace(options.GOOS),
		goarch:                  strings.TrimSpace(options.GOARCH),
		downloadSourcesOverride: append([]string(nil), options.DownloadSources...),
	}
	if manager.httpClient == nil {
		manager.httpClient = &http.Client{Timeout: manager.DownloadTimeout()}
	}
	if manager.lookPath == nil {
		manager.lookPath = exec.LookPath
	}
	if manager.userCacheDir == nil {
		manager.userCacheDir = os.UserCacheDir
	}
	if manager.goos == "" {
		manager.goos = runtime.GOOS
	}
	if manager.goarch == "" {
		manager.goarch = runtime.GOARCH
	}
	return manager
}

func (m *CodexBinaryManager) ReadyPath() (string, bool, error) {
	if m == nil {
		return "", false, nil
	}

	if explicit := strings.TrimSpace(m.config.WindowsCodexExecutable); explicit != "" {
		if resolved, ok, err := m.resolveExplicitPath(explicit); err != nil || ok {
			return resolved, ok, err
		}
	}

	if cachedPath, err := m.targetPathForCurrentPlatform(); err == nil {
		info, statErr := os.Stat(cachedPath)
		if statErr == nil && !info.IsDir() {
			return cachedPath, true, nil
		}
		if statErr != nil && !os.IsNotExist(statErr) {
			return "", false, statErr
		}
	} else if !errorsIsSandboxUnsupported(err) {
		return "", false, err
	}

	if resolved, err := m.lookPath("codex"); err == nil && strings.TrimSpace(resolved) != "" {
		return resolved, true, nil
	}
	return "", false, nil
}

func (m *CodexBinaryManager) Ensure(ctx context.Context) (string, error) {
	if m == nil {
		return "", fmt.Errorf("%w: windows codex binary manager is not configured", ErrSandboxNotSupported)
	}
	if readyPath, ok, err := m.ReadyPath(); err != nil || ok {
		return readyPath, err
	}
	return m.Download(ctx)
}

func (m *CodexBinaryManager) Download(ctx context.Context) (string, error) {
	if m == nil {
		return "", fmt.Errorf("%w: windows codex binary manager is not configured", ErrSandboxNotSupported)
	}
	if !m.AutoDownloadEnabled() {
		return "", fmt.Errorf("%w: windows codex binary is not ready and auto_download is disabled", ErrSandboxNotSupported)
	}

	targetPath, candidates, err := m.downloadCandidatesForCurrentPlatform()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return "", err
	}

	var lastErr error
	for _, candidate := range candidates {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, candidate.URL, nil)
		if err != nil {
			lastErr = err
			continue
		}
		resp, err := m.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode >= http.StatusBadRequest {
			lastErr = fmt.Errorf("download failed from %s: http %d", candidate.URL, resp.StatusCode)
			_ = resp.Body.Close()
			continue
		}
		err = installDownloadedCodexBinary(targetPath, resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		return targetPath, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("windows codex binary download sources are unavailable")
	}
	return "", fmt.Errorf("%w: %v", ErrSandboxNotSupported, lastErr)
}

func (m *CodexBinaryManager) AutoDownloadEnabled() bool {
	if m == nil || m.config == nil {
		return false
	}
	return m.config.WindowsCodexAutoDownload
}

func (m *CodexBinaryManager) DownloadTimeout() time.Duration {
	if m == nil || m.config == nil || m.config.WindowsCodexDownloadTimeout <= 0 {
		return defaultCodexDownloadTimeout
	}
	return m.config.WindowsCodexDownloadTimeout
}

func (m *CodexBinaryManager) downloadSources() []string {
	if len(m.downloadSourcesOverride) > 0 {
		return append([]string(nil), m.downloadSourcesOverride...)
	}
	// GitHub raw mirrors do not apply to release assets, so we define an
	// explicit release-asset fallback list here instead.
	return []string{
		blueCodexGitHubLatestDownload,
		blueCodexMirrorGhProxyLatestDownload,
		blueCodexMirrorKKGitHubLatestDownload,
		openAICodexGitHubLatestDownload,
		openAICodexMirrorGhProxyLatestDownload,
		openAICodexMirrorKKGitHubLatestDownload,
	}
}

func (m *CodexBinaryManager) downloadCandidatesForCurrentPlatform() (string, []codexBinaryDownloadCandidate, error) {
	targetPath, err := m.targetPathForCurrentPlatform()
	if err != nil {
		return "", nil, err
	}
	assetName, err := codexBinaryAssetName(m.goos, m.goarch)
	if err != nil {
		return "", nil, err
	}
	sources := m.downloadSources()
	candidates := make([]codexBinaryDownloadCandidate, 0, len(sources))
	for _, source := range sources {
		source = strings.TrimSpace(source)
		if source == "" {
			continue
		}
		candidates = append(candidates, codexBinaryDownloadCandidate{
			AssetName: assetName,
			URL:       source + "/" + assetName,
		})
	}
	return targetPath, candidates, nil
}

func (m *CodexBinaryManager) targetPathForCurrentPlatform() (string, error) {
	if strings.TrimSpace(m.goos) != "windows" {
		return "", fmt.Errorf("%w: managed codex binary is unsupported on %s", ErrSandboxNotSupported, m.goos)
	}
	cacheDir, err := m.userCacheDir()
	if err != nil || strings.TrimSpace(cacheDir) == "" {
		return "", fmt.Errorf("resolve user cache directory: %w", err)
	}
	return filepath.Join(cacheDir, "zimaos-blue", "sandbox", "codex", m.goarch, "codex.exe"), nil
}

func (m *CodexBinaryManager) resolveExplicitPath(explicit string) (string, bool, error) {
	if strings.ContainsAny(explicit, `/\`) {
		info, err := os.Stat(explicit)
		if err == nil && !info.IsDir() {
			return explicit, true, nil
		}
		if err != nil && os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	if resolved, err := m.lookPath(explicit); err == nil && strings.TrimSpace(resolved) != "" {
		return resolved, true, nil
	}
	return "", false, nil
}

func codexBinaryAssetName(goos, goarch string) (string, error) {
	if strings.TrimSpace(goos) != "windows" {
		return "", fmt.Errorf("%w: codex managed asset is unsupported on %s", ErrSandboxNotSupported, goos)
	}
	switch strings.TrimSpace(goarch) {
	case "amd64":
		return "codex-x86_64-pc-windows-msvc.exe", nil
	case "arm64":
		return "codex-aarch64-pc-windows-msvc.exe", nil
	default:
		return "", fmt.Errorf("%w: codex managed asset is unsupported on windows/%s", ErrSandboxNotSupported, goarch)
	}
}

func installDownloadedCodexBinary(destPath string, r io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("prepare codex install directory: %w", err)
	}
	tmpPath := destPath + ".tmp"
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return fmt.Errorf("create temp codex binary: %w", err)
	}
	if _, err := io.Copy(f, r); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write temp codex binary: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp codex binary: %w", err)
	}
	_ = os.Remove(destPath)
	if err := os.Rename(tmpPath, destPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("activate codex binary: %w", err)
	}
	return nil
}

func errorsIsSandboxUnsupported(err error) bool {
	return errors.Is(err, ErrSandboxNotSupported)
}
