//go:build espeak && !windows

package tts

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/downloader"
)

// EspeakModelManager manages espeak-ng voice data downloads
type EspeakModelManager struct {
	downloader *downloader.ModelDownloader
	dataPath   string
}

// NewEspeakModelManager creates a new espeak model manager
func NewEspeakModelManager(dataPath string) *EspeakModelManager {
	destDir := filepath.Join(dataPath, "espeak-ng-data")
	return &EspeakModelManager{
		downloader: downloader.NewModelDownloader(destDir),
		dataPath:   destDir,
	}
}

// GetModelStatus returns the current model status
func (m *EspeakModelManager) GetModelStatus() map[string]interface{} {
	voicesDir := filepath.Join(m.dataPath, "voices")
	ready := false
	if _, err := os.Stat(voicesDir); err == nil {
		ready = true
	}

	state, errMsg := m.downloader.GetState()
	return map[string]interface{}{
		"ready":       ready,
		"path":        m.dataPath,
		"state":       state,
		"error":       errMsg,
		"progress":    m.downloader.GetProgress(),
		"downloading": m.downloader.IsDownloading(),
	}
}

// DownloadModel starts downloading espeak-ng voice data
func (m *EspeakModelManager) DownloadModel(ctx context.Context) error {
	if err := os.MkdirAll(m.dataPath, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	files := []downloader.ModelFile{
		{
			Filename: "espeak-ng-data.tar.gz",
			URL:      "https://github.com/espeak-ng/espeak-ng/releases/download/1.51/espeak-ng-data.tar.gz",
			Mirrors: []string{
				"https://mirror.ghproxy.com/https://github.com/espeak-ng/espeak-ng/releases/download/1.51/espeak-ng-data.tar.gz",
			},
			Size: "~3MB",
			PostProcess: func(tarPath string) error {
				return extractTarGz(tarPath, m.dataPath)
			},
		},
	}

	return m.downloader.Download(ctx, files)
}

// CancelDownload cancels the current download
func (m *EspeakModelManager) CancelDownload() {
	m.downloader.Cancel()
}

// extractTarGz extracts a tar.gz file to destination
func extractTarGz(tarPath, dest string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(dest, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}
	return nil
}

