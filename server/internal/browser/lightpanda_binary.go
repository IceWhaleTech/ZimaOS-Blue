package browser

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	lightpandaGitHubRepo = "https://github.com/lightpanda-io/browser/releases/latest/download"
	// Keep this to three sources and intentionally exclude jsdelivr because the
	// binary payload is too large for that CDN path.
	lightpandaMirrorGhProxy  = "https://mirror.ghproxy.com/https://github.com/lightpanda-io/browser/releases/latest/download"
	lightpandaMirrorKKGitHub = "https://kkgithub.com/lightpanda-io/browser/releases/latest/download"
)

// LightpandaBinaryManager resolves or downloads the Lightpanda binary.
type LightpandaBinaryManager struct {
	config     *Config
	httpClient *http.Client
}

// NewLightpandaBinaryManager creates a new Lightpanda binary manager.
func NewLightpandaBinaryManager(config *Config) *LightpandaBinaryManager {
	return &LightpandaBinaryManager{
		config: config.Clone(),
		httpClient: &http.Client{
			Timeout: 15 * time.Minute,
		},
	}
}

// Ensure returns a usable Lightpanda binary path, downloading and extracting a
// cached binary when no explicit binary_path is configured.
func (m *LightpandaBinaryManager) Ensure(ctx context.Context) (string, error) {
	if m == nil || m.config == nil {
		return "", nil
	}
	if readyPath, ok, err := m.ReadyPath(); err != nil || ok {
		return readyPath, err
	}

	targetPath, archiveName, err := m.pathsForCurrentPlatform()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return "", err
	}

	var lastErr error
	for _, baseURL := range m.downloadSources() {
		if strings.TrimSpace(baseURL) == "" {
			continue
		}
		tmpArchive := targetPath + ".download"
		if err := m.downloadFile(ctx, baseURL+"/"+archiveName, tmpArchive); err != nil {
			lastErr = err
			_ = os.Remove(tmpArchive)
			continue
		}
		if err := extractLightpandaArchive(tmpArchive, targetPath); err != nil {
			lastErr = err
			_ = os.Remove(tmpArchive)
			continue
		}
		_ = os.Remove(tmpArchive)
		return targetPath, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("lightpanda download sources are unavailable")
	}
	return "", lastErr
}

// ReadyPath returns a usable Lightpanda binary path when one is already present
// locally, without attempting any network downloads.
func (m *LightpandaBinaryManager) ReadyPath() (string, bool, error) {
	if m == nil || m.config == nil {
		return "", false, nil
	}
	if binaryPath := strings.TrimSpace(m.config.Lightpanda.BinaryPath); binaryPath != "" {
		info, err := os.Stat(binaryPath)
		if err == nil && !info.IsDir() {
			return binaryPath, true, nil
		}
		if err != nil && os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}

	targetPath, _, err := m.pathsForCurrentPlatform()
	if err != nil {
		return "", false, err
	}
	info, err := os.Stat(targetPath)
	if err == nil && !info.IsDir() {
		return targetPath, true, nil
	}
	if err != nil && os.IsNotExist(err) {
		return "", false, nil
	}
	return "", false, err
}

func (m *LightpandaBinaryManager) downloadSources() []string {
	return []string{
		lightpandaGitHubRepo,
		lightpandaMirrorGhProxy,
		lightpandaMirrorKKGitHub,
	}
}

func (m *LightpandaBinaryManager) pathsForCurrentPlatform() (string, string, error) {
	platform := runtime.GOOS + "-" + runtime.GOARCH
	binaryName := "lightpanda"
	archiveName := ""
	switch runtime.GOOS {
	case "darwin", "linux":
		archiveName = "lightpanda-" + platform + ".tar.gz"
	case "windows":
		binaryName += ".exe"
		archiveName = "lightpanda-" + platform + ".zip"
	default:
		return "", "", fmt.Errorf("lightpanda is unsupported on %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil || strings.TrimSpace(cacheDir) == "" {
		cacheDir = os.TempDir()
	}
	targetPath := filepath.Join(cacheDir, "zimaos-blue", "browser", "lightpanda", platform, binaryName)
	return targetPath, archiveName, nil
}

func (m *LightpandaBinaryManager) downloadFile(ctx context.Context, rawURL string, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("download failed from %s: http %d", rawURL, resp.StatusCode)
	}
	file, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := io.Copy(file, resp.Body); err != nil {
		return err
	}
	return nil
}

func extractLightpandaArchive(archivePath, targetPath string) error {
	switch {
	case strings.HasSuffix(strings.ToLower(archivePath), ".tar.gz"):
		return extractLightpandaTarGz(archivePath, targetPath)
	case strings.HasSuffix(strings.ToLower(archivePath), ".zip"):
		return extractLightpandaZip(archivePath, targetPath)
	default:
		if err := copyFile(archivePath, targetPath); err != nil {
			return err
		}
		return os.Chmod(targetPath, 0o755)
	}
}

func extractLightpandaTarGz(archivePath, targetPath string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gz.Close()

	reader := tar.NewReader(gz)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if header == nil || header.FileInfo().IsDir() {
			continue
		}
		name := strings.ToLower(filepath.Base(header.Name))
		if !strings.Contains(name, "lightpanda") {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		out, err := os.Create(targetPath)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, reader); err != nil {
			out.Close()
			return err
		}
		if err := out.Close(); err != nil {
			return err
		}
		return os.Chmod(targetPath, 0o755)
	}
	return fmt.Errorf("lightpanda binary not found in %s", archivePath)
}

func extractLightpandaZip(archivePath, targetPath string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		if file == nil || file.FileInfo().IsDir() {
			continue
		}
		name := strings.ToLower(filepath.Base(file.Name))
		if !strings.Contains(name, "lightpanda") {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			rc.Close()
			return err
		}
		out, err := os.Create(targetPath)
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			rc.Close()
			out.Close()
			return err
		}
		rc.Close()
		if err := out.Close(); err != nil {
			return err
		}
		return os.Chmod(targetPath, 0o755)
	}
	return fmt.Errorf("lightpanda binary not found in %s", archivePath)
}

func copyFile(srcPath, destPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	dest, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer dest.Close()
	_, err = io.Copy(dest, src)
	return err
}
