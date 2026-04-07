package smallmodel

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/downloader"
)

const llamaCppRuntimeRelease = "b8690"

// Release archives are GitHub release assets rather than raw repository files,
// so the standard raw.githubusercontent.com 4-source chain does not apply.
// Use the official release asset first, then explicit binary mirrors.
func llamaCppRuntimeDownloadURLs(assetName string) []string {
	if strings.TrimSpace(assetName) == "" {
		return nil
	}
	official := fmt.Sprintf("https://github.com/ggml-org/llama.cpp/releases/download/%s/%s", llamaCppRuntimeRelease, assetName)
	return []string{
		official,
		"https://mirror.ghproxy.com/" + official,
		fmt.Sprintf("https://kkgithub.com/ggml-org/llama.cpp/releases/download/%s/%s", llamaCppRuntimeRelease, assetName),
	}
}

func llamaCppRuntimeArchiveName(goos, goarch string) (string, error) {
	switch goos + "/" + goarch {
	case "darwin/arm64":
		return fmt.Sprintf("llama-%s-bin-macos-arm64.tar.gz", llamaCppRuntimeRelease), nil
	case "darwin/amd64":
		return fmt.Sprintf("llama-%s-bin-macos-x64.tar.gz", llamaCppRuntimeRelease), nil
	case "linux/amd64":
		return fmt.Sprintf("llama-%s-bin-ubuntu-x64.tar.gz", llamaCppRuntimeRelease), nil
	case "linux/arm64":
		return fmt.Sprintf("llama-%s-bin-ubuntu-arm64.tar.gz", llamaCppRuntimeRelease), nil
	default:
		return "", fmt.Errorf("llama.cpp runtime archive is unsupported on %s/%s", goos, goarch)
	}
}

func llamaCppRuntimeInstallDir(dataDir, goos, goarch string) string {
	root := strings.TrimSpace(dataDir)
	if root == "" {
		if cacheDir, err := os.UserCacheDir(); err == nil && strings.TrimSpace(cacheDir) != "" {
			root = filepath.Join(cacheDir, "zimaos-blue")
		} else {
			root = os.TempDir()
		}
	}
	return filepath.Join(root, "llama-runtime", llamaCppRuntimeRelease, goos+"-"+goarch)
}

func extractLlamaCppRuntimeArchive(archivePath, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	switch {
	case strings.HasSuffix(strings.ToLower(archivePath), ".zip"):
		return extractLlamaCppRuntimeZip(archivePath, destDir)
	default:
		return extractLlamaCppRuntimeTarGz(archivePath, destDir)
	}
}

func extractLlamaCppRuntimeTarGz(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		base := filepath.Base(strings.TrimSpace(hdr.Name))
		if !shouldExtractLlamaCppRuntimeEntry(base) {
			continue
		}
		target := filepath.Join(destDir, base)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, fileModeOrDefault(hdr.Mode))
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return err
		}
		if err := out.Close(); err != nil {
			return err
		}
	}
}

func extractLlamaCppRuntimeZip(archivePath, destDir string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer zr.Close()

	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		base := filepath.Base(strings.TrimSpace(f.Name))
		if !shouldExtractLlamaCppRuntimeEntry(base) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		target := filepath.Join(destDir, base)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			rc.Close()
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, fileModeOrDefault(int64(f.Mode())))
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			out.Close()
			rc.Close()
			return err
		}
		if err := out.Close(); err != nil {
			rc.Close()
			return err
		}
		if err := rc.Close(); err != nil {
			return err
		}
	}
	return nil
}

func shouldExtractLlamaCppRuntimeEntry(base string) bool {
	base = strings.TrimSpace(base)
	if base == "" {
		return false
	}
	lower := strings.ToLower(base)
	return strings.HasPrefix(lower, "llama-") ||
		strings.HasPrefix(lower, "libllama") ||
		strings.HasPrefix(lower, "libmtmd") ||
		strings.HasPrefix(lower, "libggml") ||
		lower == "license"
}

func fileModeOrDefault(mode int64) os.FileMode {
	if mode <= 0 {
		return 0o755
	}
	return os.FileMode(mode)
}

func llamaCppExecutableName(base, goos string) string {
	if goos == "windows" {
		return base + ".exe"
	}
	return base
}

type llamaCppRuntimePackageManager struct {
	dataDir    string
	goos       string
	goarch     string
	downloader *downloader.ModelDownloader
}

func newLlamaCppRuntimePackageManager(dataDir string) *llamaCppRuntimePackageManager {
	return &llamaCppRuntimePackageManager{
		dataDir:    strings.TrimSpace(dataDir),
		goos:       runtime.GOOS,
		goarch:     runtime.GOARCH,
		downloader: downloader.NewModelDownloader(llamaCppRuntimeInstallDir(dataDir, runtime.GOOS, runtime.GOARCH)),
	}
}

func (m *llamaCppRuntimePackageManager) installDir() string {
	if m == nil {
		return ""
	}
	return llamaCppRuntimeInstallDir(m.dataDir, m.goos, m.goarch)
}

func (m *llamaCppRuntimePackageManager) supported() bool {
	if m == nil {
		return false
	}
	_, err := llamaCppRuntimeArchiveName(m.goos, m.goarch)
	return err == nil
}

func (m *llamaCppRuntimePackageManager) assetFile() (downloader.ModelFile, error) {
	if m == nil {
		return downloader.ModelFile{}, fmt.Errorf("llama.cpp runtime package manager is nil")
	}
	assetName, err := llamaCppRuntimeArchiveName(m.goos, m.goarch)
	if err != nil {
		return downloader.ModelFile{}, err
	}
	urls := llamaCppRuntimeDownloadURLs(assetName)
	if len(urls) == 0 {
		return downloader.ModelFile{}, fmt.Errorf("no llama.cpp runtime download URLs for %s", assetName)
	}
	installDir := m.installDir()
	return downloader.ModelFile{
		Filename: assetName,
		URL:      urls[0],
		Mirrors:  append([]string(nil), urls[1:]...),
		Size:     "",
		PostProcess: func(archivePath string) error {
			if err := extractLlamaCppRuntimeArchive(archivePath, installDir); err != nil {
				return err
			}
			_ = os.Remove(archivePath)
			return nil
		},
	}, nil
}

func (m *llamaCppRuntimePackageManager) readyPath(base string) (string, bool) {
	if m == nil {
		return "", false
	}
	target := filepath.Join(m.installDir(), llamaCppExecutableName(base, m.goos))
	info, err := os.Stat(target)
	if err == nil && !info.IsDir() {
		m.applyRuntimeEnv()
		return target, true
	}
	return "", false
}

func (m *llamaCppRuntimePackageManager) readyLibraryDir() (string, bool) {
	if m == nil {
		return "", false
	}
	libFile := libraryFilename(m.installDir(), "llama")
	info, err := os.Stat(libFile)
	if err == nil && !info.IsDir() {
		m.applyRuntimeEnv()
		return m.installDir(), true
	}
	return "", false
}

func (m *llamaCppRuntimePackageManager) isDownloading() bool {
	return m != nil && m.downloader != nil && m.downloader.IsDownloading()
}

func (m *llamaCppRuntimePackageManager) WarmupAsync() {
	if m == nil || !m.supported() || m.downloader == nil {
		return
	}
	if _, ok := m.readyPath("llama-server"); ok {
		return
	}
	if _, ok := m.readyPath("llama-cli"); ok {
		return
	}
	if _, ok := m.readyLibraryDir(); ok {
		return
	}
	if m.downloader.IsDownloading() {
		return
	}
	file, err := m.assetFile()
	if err != nil {
		return
	}
	go func() {
		_ = m.downloader.Download(context.Background(), []downloader.ModelFile{file})
		m.applyRuntimeEnv()
	}()
}

func (m *llamaCppRuntimePackageManager) applyRuntimeEnv() {
	if m == nil {
		return
	}
	if _, ok := m.readyLibraryDirWithoutEnv(); !ok {
		return
	}
	if dir := m.installDir(); dir != "" {
		if strings.TrimSpace(os.Getenv(smallModelLlamaLibDirEnv)) == "" {
			_ = os.Setenv(smallModelLlamaLibDirEnv, dir)
		}
		if strings.TrimSpace(os.Getenv(smallModelLlamaLoaderLibDirEnv)) == "" {
			_ = os.Setenv(smallModelLlamaLoaderLibDirEnv, dir)
		}
	}
}

func (m *llamaCppRuntimePackageManager) readyLibraryDirWithoutEnv() (string, bool) {
	if m == nil {
		return "", false
	}
	libFile := libraryFilename(m.installDir(), "llama")
	info, err := os.Stat(libFile)
	if err == nil && !info.IsDir() {
		return m.installDir(), true
	}
	return "", false
}
