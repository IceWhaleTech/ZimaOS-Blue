package whatsapp

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	wacliReleaseBaseURL = "https://github.com/steipete/wacli/releases/latest/download"
	wacliGoInstallRef   = "github.com/steipete/wacli/cmd/wacli@latest"
)

var wacliInstallMu sync.Mutex

func ensureWACLI(ctx context.Context) (string, error) {
	wacliInstallMu.Lock()
	defer wacliInstallMu.Unlock()

	if resolved, err := exec.LookPath("wacli"); err == nil {
		return resolved, nil
	}
	if cached, ok := existingCachedWACLIPath(); ok {
		return cached, nil
	}

	cachedPath, err := cachedWACLIPath()
	if err != nil {
		return "", err
	}

	attempts := make([]string, 0, 2)
	if err := installWACLIFromRelease(ctx, cachedPath); err == nil {
		return cachedPath, nil
	} else {
		attempts = append(attempts, "download: "+err.Error())
	}

	if err := installWACLIWithGo(ctx, filepath.Dir(cachedPath)); err == nil {
		if _, err := os.Stat(cachedPath); err == nil {
			return cachedPath, nil
		}
		if resolved, err := exec.LookPath("wacli"); err == nil {
			return resolved, nil
		}
		attempts = append(attempts, "go install: binary not found after installation")
	} else {
		attempts = append(attempts, "go install: "+err.Error())
	}

	return "", fmt.Errorf("failed to install wacli automatically (%s). %s", strings.Join(attempts, "; "), manualWACLIInstallHint())
}

func existingCachedWACLIPath() (string, bool) {
	cachedPath, err := cachedWACLIPath()
	if err != nil {
		return "", false
	}
	if _, err := os.Stat(cachedPath); err != nil {
		return "", false
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(cachedPath, 0o755)
	}
	return cachedPath, true
}

func cachedWACLIPath() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve user cache directory: %w", err)
	}
	return filepath.Join(cacheDir, "zimaos-blue", wacliBinaryName()), nil
}

func wacliBinaryName() string {
	if runtime.GOOS == "windows" {
		return "wacli.exe"
	}
	return "wacli"
}

func installWACLIFromRelease(ctx context.Context, destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("prepare cache directory: %w", err)
	}

	urls := wacliReleaseURLs()
	if len(urls) == 0 {
		return fmt.Errorf("unsupported platform %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	var errs []string
	for _, url := range urls {
		if err := downloadAndInstallWACLI(ctx, url, destPath); err == nil {
			return nil
		} else {
			errs = append(errs, fmt.Sprintf("%s: %v", filepath.Base(url), err))
		}
	}
	return fmt.Errorf("%s", strings.Join(errs, "; "))
}

func wacliReleaseURLs() []string {
	base := wacliReleaseBaseURL + "/"
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "linux/amd64":
		return []string{base + "wacli_Linux_x86_64.tar.gz"}
	case "linux/arm64":
		return []string{base + "wacli_Linux_arm64.tar.gz", base + "wacli_Linux_aarch64.tar.gz"}
	case "darwin/amd64":
		return []string{base + "wacli_Darwin_x86_64.tar.gz", base + "wacli_Darwin_x86_64.zip"}
	case "darwin/arm64":
		return []string{base + "wacli_Darwin_arm64.tar.gz", base + "wacli_Darwin_arm64.zip"}
	case "windows/amd64":
		return []string{base + "wacli_Windows_x86_64.zip"}
	case "windows/arm64":
		return []string{base + "wacli_Windows_arm64.zip"}
	default:
		return nil
	}
}

func downloadAndInstallWACLI(ctx context.Context, url, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	resp, err := (&http.Client{Timeout: 2 * time.Minute}).Do(req)
	if err != nil {
		return fmt.Errorf("download archive: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	switch {
	case strings.HasSuffix(url, ".tar.gz"):
		return extractTarGZBinary(resp.Body, destPath, wacliBinaryName())
	case strings.HasSuffix(url, ".zip"):
		return extractZipBinary(resp.Body, destPath, wacliBinaryName())
	default:
		return installBinary(destPath, resp.Body, 0o755)
	}
}

func extractTarGZBinary(r io.Reader, destPath, binaryName string) error {
	gzReader, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("open gzip stream: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar archive: %w", err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		if filepath.Base(header.Name) != binaryName {
			continue
		}
		mode := os.FileMode(header.Mode)
		if mode == 0 {
			mode = 0o755
		}
		return installBinary(destPath, tarReader, mode)
	}
	return fmt.Errorf("binary %q not found in archive", binaryName)
}

func extractZipBinary(r io.Reader, destPath, binaryName string) error {
	body, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read zip archive: %w", err)
	}
	zipReader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return fmt.Errorf("open zip archive: %w", err)
	}
	for _, f := range zipReader.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if filepath.Base(f.Name) != binaryName {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("open zip entry: %w", err)
		}
		defer rc.Close()
		mode := f.Mode()
		if mode == 0 {
			mode = 0o755
		}
		return installBinary(destPath, rc, mode)
	}
	return fmt.Errorf("binary %q not found in archive", binaryName)
}

func installBinary(destPath string, r io.Reader, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("prepare install directory: %w", err)
	}

	tmpPath := destPath + ".tmp"
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return fmt.Errorf("create temp binary: %w", err)
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write temp binary: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp binary: %w", err)
	}
	if mode == 0 {
		mode = 0o755
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmpPath, mode); err != nil {
			_ = os.Remove(tmpPath)
			return fmt.Errorf("chmod temp binary: %w", err)
		}
	}
	if runtime.GOOS == "windows" {
		_ = os.Remove(destPath)
	}
	if err := os.Rename(tmpPath, destPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("activate binary: %w", err)
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(destPath, mode)
	}
	return nil
}

func installWACLIWithGo(ctx context.Context, destDir string) error {
	goPath, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("go toolchain not found: %w", err)
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("prepare install directory: %w", err)
	}
	cmd := exec.CommandContext(ctx, goPath, "install", wacliGoInstallRef)
	cmd.Env = append(os.Environ(), "GOBIN="+destDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(output))
		if trimmed != "" {
			return fmt.Errorf("%w: %s", err, trimmed)
		}
		return err
	}
	installedPath := filepath.Join(destDir, wacliBinaryName())
	if _, err := os.Stat(installedPath); err != nil {
		return fmt.Errorf("installed binary missing: %w", err)
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(installedPath, 0o755)
	}
	return nil
}

func manualWACLIInstallHint() string {
	if runtime.GOOS == "darwin" {
		return "Install it manually with `brew install steipete/tap/wacli` or `go install github.com/steipete/wacli/cmd/wacli@latest`."
	}
	return "Install it manually from the latest GitHub release or run `go install github.com/steipete/wacli/cmd/wacli@latest`."
}
