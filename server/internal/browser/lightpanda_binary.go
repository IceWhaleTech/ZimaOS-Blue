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
	lightpandaGitHubNightly = "https://github.com/lightpanda-io/browser/releases/download/nightly"
	lightpandaGitHubLatest  = "https://github.com/lightpanda-io/browser/releases/latest/download"
	// Keep this to three sources and intentionally exclude jsdelivr because the
	// binary payload is too large for that CDN path.
	lightpandaMirrorGhProxyNightly  = "https://mirror.ghproxy.com/https://github.com/lightpanda-io/browser/releases/download/nightly"
	lightpandaMirrorGhProxyLatest   = "https://mirror.ghproxy.com/https://github.com/lightpanda-io/browser/releases/latest/download"
	lightpandaMirrorKKGitHubNightly = "https://kkgithub.com/lightpanda-io/browser/releases/download/nightly"
	lightpandaMirrorKKGitHubLatest  = "https://kkgithub.com/lightpanda-io/browser/releases/latest/download"

	lightpandaReleaseNightly = "nightly"
	lightpandaReleaseLatest  = "latest"
)

type lightpandaBinaryAsset struct {
	Name    string
	Release string
}

type lightpandaBinaryDownloadCandidate struct {
	AssetName string
	URL       string
}

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

	targetPath, candidates, err := m.downloadCandidatesForCurrentPlatform()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return "", err
	}

	var lastErr error
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.URL) == "" {
			continue
		}
		tmpArchive := downloadTempPath(filepath.Dir(targetPath), candidate.AssetName)
		if err := m.downloadFile(ctx, candidate.URL, tmpArchive); err != nil {
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

	targetPath, err := targetPathForCurrentPlatform(runtime.GOOS, runtime.GOARCH)
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

func (m *LightpandaBinaryManager) downloadSources(release string) []string {
	switch strings.TrimSpace(release) {
	case lightpandaReleaseNightly:
		return []string{
			lightpandaGitHubNightly,
			lightpandaMirrorGhProxyNightly,
			lightpandaMirrorKKGitHubNightly,
		}
	default:
		return []string{
			lightpandaGitHubLatest,
			lightpandaMirrorGhProxyLatest,
			lightpandaMirrorKKGitHubLatest,
		}
	}
}

func (m *LightpandaBinaryManager) downloadCandidatesForCurrentPlatform() (string, []lightpandaBinaryDownloadCandidate, error) {
	targetPath, err := targetPathForCurrentPlatform(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", nil, err
	}
	assets, err := lightpandaAssetsForPlatform(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", nil, err
	}
	candidates := make([]lightpandaBinaryDownloadCandidate, 0, len(assets)*3)
	for _, asset := range assets {
		for _, baseURL := range m.downloadSources(asset.Release) {
			if strings.TrimSpace(baseURL) == "" {
				continue
			}
			candidates = append(candidates, lightpandaBinaryDownloadCandidate{
				AssetName: asset.Name,
				URL:       baseURL + "/" + asset.Name,
			})
		}
	}
	return targetPath, candidates, nil
}

func targetPathForCurrentPlatform(goos, goarch string) (string, error) {
	binaryName, err := lightpandaBinaryName(goos)
	if err != nil {
		return "", err
	}
	platform := goos + "-" + goarch
	cacheDir, cacheErr := os.UserCacheDir()
	if cacheErr != nil || strings.TrimSpace(cacheDir) == "" {
		cacheDir = os.TempDir()
	}
	return filepath.Join(cacheDir, "zimaos-blue", "browser", "lightpanda", platform, binaryName), nil
}

func lightpandaBinaryName(goos string) (string, error) {
	switch goos {
	case "darwin", "linux":
		return "lightpanda", nil
	case "windows":
		return "lightpanda.exe", nil
	default:
		return "", fmt.Errorf("lightpanda is unsupported on %s", goos)
	}
}

func lightpandaAssetsForPlatform(goos, goarch string) ([]lightpandaBinaryAsset, error) {
	legacyAsset, err := lightpandaLegacyArchiveForPlatform(goos, goarch)
	if err != nil {
		return nil, err
	}
	assets := make([]lightpandaBinaryAsset, 0, 2)
	if nightlyAsset := lightpandaNightlyAssetForPlatform(goos, goarch); nightlyAsset != "" {
		assets = append(assets, lightpandaBinaryAsset{
			Name:    nightlyAsset,
			Release: lightpandaReleaseNightly,
		})
	}
	assets = append(assets, lightpandaBinaryAsset{
		Name:    legacyAsset,
		Release: lightpandaReleaseLatest,
	})
	return assets, nil
}

func lightpandaNightlyAssetForPlatform(goos, goarch string) string {
	switch {
	case goos == "darwin" && goarch == "arm64":
		return "lightpanda-aarch64-macos"
	case goos == "linux" && goarch == "amd64":
		return "lightpanda-x86_64-linux"
	default:
		return ""
	}
}

func lightpandaLegacyArchiveForPlatform(goos, goarch string) (string, error) {
	platform := goos + "-" + goarch
	switch goos {
	case "darwin", "linux":
		return "lightpanda-" + platform + ".tar.gz", nil
	case "windows":
		return "lightpanda-" + platform + ".zip", nil
	default:
		return "", fmt.Errorf("lightpanda is unsupported on %s/%s", goos, goarch)
	}
}

func downloadTempPath(dir, assetName string) string {
	name := strings.TrimSpace(assetName)
	if name == "" {
		name = "lightpanda.download"
	}
	return filepath.Join(dir, "download-"+name)
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
