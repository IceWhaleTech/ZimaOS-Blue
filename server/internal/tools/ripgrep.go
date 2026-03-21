package tools

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
)

const ripgrepReleaseBaseURL = "https://github.com/BurntSushi/ripgrep/releases/download"

//go:embed ripgrep_manifest.json
var ripgrepManifestBytes []byte

type ripgrepManifest struct {
	Version   string                             `json:"version"`
	Platforms map[string]ripgrepPlatformManifest `json:"platforms"`
}

type ripgrepPlatformManifest struct {
	Asset       string `json:"asset"`
	ArchiveType string `json:"archive_type"`
	SHA256      string `json:"sha256"`
	BinaryPath  string `json:"binary_path"`
}

type ripgrepBinary struct {
	Path   string
	Source string
}

type ripgrepResolver interface {
	Resolve(ctx context.Context) (*ripgrepBinary, error)
}

type ripgrepExecFunc func(ctx context.Context, binPath string, args []string, dir string) ([]byte, int, error)

type RipgrepManager struct {
	cfg         config.ToolCallingRipgrepConfig
	dataDir     string
	manifest    ripgrepManifest
	httpClient  *http.Client
	lookPath    func(string) (string, error)
	execCommand ripgrepExecFunc
	platformKey func() string

	mu sync.Mutex
}

func NewRipgrepManager(cfg config.ToolCallingRipgrepConfig, dataDir string) (*RipgrepManager, error) {
	var manifest ripgrepManifest
	if err := json.Unmarshal(ripgrepManifestBytes, &manifest); err != nil {
		return nil, fmt.Errorf("decode ripgrep manifest: %w", err)
	}
	if strings.TrimSpace(manifest.Version) == "" {
		return nil, errors.New("ripgrep manifest version is empty")
	}
	if len(manifest.Platforms) == 0 {
		return nil, errors.New("ripgrep manifest platforms are empty")
	}
	return &RipgrepManager{
		cfg:      cfg,
		dataDir:  strings.TrimSpace(dataDir),
		manifest: manifest,
		httpClient: &http.Client{
			Timeout: 10 * time.Minute,
		},
		lookPath:    exec.LookPath,
		execCommand: defaultRipgrepExec,
		platformKey: currentRipgrepPlatformKey,
	}, nil
}

func currentRipgrepPlatformKey() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}

func defaultRipgrepExec(ctx context.Context, binPath string, args []string, dir string) ([]byte, int, error) {
	cmd := exec.CommandContext(ctx, binPath, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		return out, 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return out, exitErr.ExitCode(), err
	}
	return out, -1, err
}

func (m *RipgrepManager) Resolve(ctx context.Context) (*ripgrepBinary, error) {
	if m == nil {
		return nil, errors.New("ripgrep runtime is not configured")
	}
	if !m.cfg.Enabled {
		return nil, errors.New("ripgrep runtime is disabled")
	}
	entry, platformKey, err := m.platformManifest()
	if err != nil {
		return nil, err
	}

	reasons := make([]string, 0, 4)
	if bin, err := m.resolveManagedBinary(ctx, platformKey, entry); err == nil {
		return bin, nil
	} else if err != nil {
		reasons = append(reasons, err.Error())
	}

	if m.cfg.AllowSystemBinary {
		if bin, err := m.resolveSystemBinary(ctx); err == nil {
			return bin, nil
		} else if err != nil {
			reasons = append(reasons, err.Error())
		}
	} else {
		reasons = append(reasons, "system rg is disabled")
	}

	if m.cfg.AutoDownload {
		if bin, err := m.downloadManagedBinary(ctx, platformKey, entry); err == nil {
			return bin, nil
		} else if err != nil {
			reasons = append(reasons, err.Error())
		}
	} else {
		reasons = append(reasons, "ripgrep auto-download is disabled")
	}

	if len(reasons) == 0 {
		return nil, errors.New("ripgrep is unavailable")
	}
	return nil, errors.New(strings.Join(reasons, "; "))
}

func (m *RipgrepManager) platformManifest() (ripgrepPlatformManifest, string, error) {
	platformKey := currentRipgrepPlatformKey()
	if m != nil && m.platformKey != nil {
		platformKey = strings.TrimSpace(m.platformKey())
	}
	entry, ok := m.manifest.Platforms[platformKey]
	if !ok {
		return ripgrepPlatformManifest{}, platformKey, fmt.Errorf("ripgrep is unsupported on %s", platformKey)
	}
	return entry, platformKey, nil
}

func (m *RipgrepManager) resolveManagedBinary(ctx context.Context, platformKey string, entry ripgrepPlatformManifest) (*ripgrepBinary, error) {
	binaryPath := m.managedBinaryPath(platformKey, entry)
	info, err := os.Stat(binaryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("managed ripgrep is not cached")
		}
		return nil, fmt.Errorf("stat managed ripgrep: %w", err)
	}
	if info.IsDir() {
		_ = os.RemoveAll(filepath.Dir(binaryPath))
		return nil, errors.New("managed ripgrep cache is invalid")
	}
	if err := m.validateBinary(ctx, binaryPath); err != nil {
		_ = os.RemoveAll(filepath.Dir(binaryPath))
		return nil, fmt.Errorf("managed ripgrep cache is invalid: %w", err)
	}
	return &ripgrepBinary{Path: binaryPath, Source: "managed_cache"}, nil
}

func (m *RipgrepManager) resolveSystemBinary(ctx context.Context) (*ripgrepBinary, error) {
	if m == nil || m.lookPath == nil {
		return nil, errors.New("system rg lookup is unavailable")
	}
	binaryPath, err := m.lookPath("rg")
	if err != nil {
		return nil, errors.New("system rg is not available")
	}
	if err := m.validateBinary(ctx, binaryPath); err != nil {
		return nil, fmt.Errorf("system rg validation failed: %w", err)
	}
	return &ripgrepBinary{Path: binaryPath, Source: "system_path"}, nil
}

func (m *RipgrepManager) downloadManagedBinary(ctx context.Context, platformKey string, entry ripgrepPlatformManifest) (*ripgrepBinary, error) {
	if m == nil {
		return nil, errors.New("ripgrep runtime is not configured")
	}
	targetDir := m.managedPlatformDir(platformKey)
	targetBinary := m.managedBinaryPath(platformKey, entry)
	parentDir := filepath.Dir(targetDir)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return nil, fmt.Errorf("create ripgrep cache parent: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if bin, err := m.resolveManagedBinary(ctx, platformKey, entry); err == nil {
		return bin, nil
	}

	tmpDir := targetDir + ".partial"
	urls := m.downloadURLs(entry)
	var lastErr error
	for _, url := range urls {
		if err := os.RemoveAll(tmpDir); err != nil {
			lastErr = fmt.Errorf("reset ripgrep temp dir: %w", err)
			continue
		}
		if err := os.MkdirAll(tmpDir, 0o755); err != nil {
			lastErr = fmt.Errorf("create ripgrep temp dir: %w", err)
			continue
		}

		archivePath := filepath.Join(tmpDir, filepath.Base(entry.Asset)+".partial")
		if err := m.downloadArchive(ctx, url, archivePath, entry.SHA256); err != nil {
			lastErr = err
			continue
		}
		if err := extractRipgrepArchive(archivePath, tmpDir, entry); err != nil {
			lastErr = err
			continue
		}
		tmpBinary := filepath.Join(tmpDir, filepath.Base(entry.BinaryPath))
		if err := m.validateBinary(ctx, tmpBinary); err != nil {
			lastErr = fmt.Errorf("validate downloaded ripgrep: %w", err)
			continue
		}
		if err := os.RemoveAll(targetDir); err != nil {
			lastErr = fmt.Errorf("prepare ripgrep target dir: %w", err)
			continue
		}
		if err := os.Rename(tmpDir, targetDir); err != nil {
			if bin, cacheErr := m.resolveManagedBinary(ctx, platformKey, entry); cacheErr == nil {
				return bin, nil
			}
			lastErr = fmt.Errorf("publish ripgrep cache: %w", err)
			continue
		}
		return &ripgrepBinary{Path: targetBinary, Source: "managed_download"}, nil
	}

	_ = os.RemoveAll(tmpDir)
	if lastErr == nil {
		lastErr = errors.New("unknown download failure")
	}
	return nil, fmt.Errorf("all ripgrep download sources failed: %w", lastErr)
}

func (m *RipgrepManager) validateBinary(ctx context.Context, binaryPath string) error {
	if m == nil || m.execCommand == nil {
		return errors.New("ripgrep command runner is unavailable")
	}
	out, exitCode, err := m.execCommand(ctx, binaryPath, []string{"--version"}, "")
	if err != nil && exitCode != 0 {
		return fmt.Errorf("execute --version: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("unexpected exit code %d", exitCode)
	}
	line := strings.ToLower(strings.TrimSpace(string(out)))
	if !strings.HasPrefix(line, "ripgrep ") {
		return fmt.Errorf("unexpected version output: %q", strings.TrimSpace(string(out)))
	}
	return nil
}

func (m *RipgrepManager) managedPlatformDir(platformKey string) string {
	platformDir := strings.ReplaceAll(platformKey, "/", "-")
	return filepath.Join(m.cacheDir(), m.manifest.Version, platformDir)
}

func (m *RipgrepManager) managedBinaryPath(platformKey string, entry ripgrepPlatformManifest) string {
	return filepath.Join(m.managedPlatformDir(platformKey), filepath.Base(entry.BinaryPath))
}

func (m *RipgrepManager) cacheDir() string {
	if cacheDir := strings.TrimSpace(m.cfg.CacheDir); cacheDir != "" {
		return cacheDir
	}
	if strings.TrimSpace(m.dataDir) != "" {
		return filepath.Join(m.dataDir, "tools", "ripgrep")
	}
	return filepath.Join(".", "data", "tools", "ripgrep")
}

func (m *RipgrepManager) downloadURLs(entry ripgrepPlatformManifest) []string {
	githubURL := m.githubReleaseURL(entry)
	seen := map[string]struct{}{}
	out := make([]string, 0, 4)
	for _, base := range m.effectiveMirrorBaseURLs() {
		url := buildRipgrepMirrorURL(base, m.manifest.Version, entry.Asset, githubURL)
		if url == "" {
			continue
		}
		if _, ok := seen[url]; ok {
			continue
		}
		seen[url] = struct{}{}
		out = append(out, url)
	}
	if _, ok := seen[githubURL]; !ok {
		out = append(out, githubURL)
	}
	return out
}

func (m *RipgrepManager) githubReleaseURL(entry ripgrepPlatformManifest) string {
	return fmt.Sprintf("%s/%s/%s", ripgrepReleaseBaseURL, m.manifest.Version, entry.Asset)
}

func (m *RipgrepManager) effectiveMirrorBaseURLs() []string {
	out := make([]string, 0, 4)
	seen := make(map[string]struct{}, 4)
	appendURL := func(raw string) {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			return
		}
		if _, ok := seen[trimmed]; ok {
			return
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	appendURL(m.cfg.MirrorBaseURL)
	for _, raw := range m.cfg.MirrorBaseURLs {
		appendURL(raw)
	}
	if len(out) == 0 {
		appendURL("https://mirror.ghproxy.com")
		appendURL("https://gh-proxy.com")
		appendURL("https://downloads.sourceforge.net/project/ripgrep.mirror")
	}
	return out
}

func buildRipgrepMirrorURL(baseURL, version, asset, githubURL string) string {
	baseURL = strings.TrimSpace(strings.TrimRight(baseURL, "/"))
	if baseURL == "" {
		return ""
	}
	replaced := strings.NewReplacer(
		"{github_release_url}", githubURL,
		"{version}", version,
		"{asset}", asset,
	).Replace(baseURL)
	if replaced != baseURL {
		return replaced
	}
	if strings.Contains(baseURL, "sourceforge.net/project/") {
		return fmt.Sprintf("%s/%s/%s", baseURL, version, asset)
	}
	if strings.HasPrefix(baseURL, "https://") || strings.HasPrefix(baseURL, "http://") {
		return baseURL + "/" + githubURL
	}
	return ""
}

func (m *RipgrepManager) downloadArchive(ctx context.Context, url, destPath, expectedSHA string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build request for %s: %w", url, err)
	}
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}

	file, err := os.OpenFile(destPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("create %s: %w", destPath, err)
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(io.MultiWriter(file, hasher), resp.Body); err != nil {
		_ = os.Remove(destPath)
		return fmt.Errorf("write %s: %w", destPath, err)
	}
	actualSHA := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(actualSHA, strings.TrimSpace(expectedSHA)) {
		_ = os.Remove(destPath)
		return fmt.Errorf("checksum mismatch for %s: expected %s, got %s", filepath.Base(destPath), expectedSHA, actualSHA)
	}
	return nil
}

func extractRipgrepArchive(archivePath, destDir string, entry ripgrepPlatformManifest) error {
	switch entry.ArchiveType {
	case "tar.gz":
		return extractRipgrepTarGz(archivePath, destDir, entry.BinaryPath)
	case "zip":
		return extractRipgrepZip(archivePath, destDir, entry.BinaryPath)
	default:
		return fmt.Errorf("unsupported ripgrep archive type: %s", entry.ArchiveType)
	}
}

func extractRipgrepTarGz(archivePath, destDir, binaryPath string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("open gzip archive: %w", err)
	}
	defer gzipReader.Close()

	targetPath := filepath.Join(destDir, filepath.Base(binaryPath))
	want := normalizeArchivePath(binaryPath)
	tr := tar.NewReader(gzipReader)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar entry: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if normalizeArchivePath(hdr.Name) != want {
			continue
		}
		if err := writeArchiveBinary(targetPath, tr); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("binary %s not found in archive", binaryPath)
}

func extractRipgrepZip(archivePath, destDir, binaryPath string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open zip archive: %w", err)
	}
	defer reader.Close()

	targetPath := filepath.Join(destDir, filepath.Base(binaryPath))
	want := normalizeArchivePath(binaryPath)
	for _, file := range reader.File {
		if normalizeArchivePath(file.Name) != want {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return fmt.Errorf("open zip entry: %w", err)
		}
		writeErr := writeArchiveBinary(targetPath, rc)
		rc.Close()
		if writeErr != nil {
			return writeErr
		}
		return nil
	}
	return fmt.Errorf("binary %s not found in archive", binaryPath)
}

func normalizeArchivePath(raw string) string {
	cleaned := strings.ReplaceAll(strings.TrimSpace(raw), "\\", "/")
	cleaned = strings.TrimPrefix(path.Clean(cleaned), "./")
	return strings.TrimPrefix(cleaned, "/")
}

func writeArchiveBinary(targetPath string, src io.Reader) error {
	file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return fmt.Errorf("create extracted binary: %w", err)
	}
	if _, err := io.Copy(file, src); err != nil {
		file.Close()
		_ = os.Remove(targetPath)
		return fmt.Errorf("write extracted binary: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(targetPath)
		return fmt.Errorf("close extracted binary: %w", err)
	}
	if err := os.Chmod(targetPath, 0o755); err != nil {
		return fmt.Errorf("chmod extracted binary: %w", err)
	}
	return nil
}
