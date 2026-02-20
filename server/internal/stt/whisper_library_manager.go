//go:build whisper

package stt

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/downloader"
)

// WhisperLibraryManager manages whisper/opus library downloads
type WhisperLibraryManager struct {
	downloader *downloader.ModelDownloader
	libPath    string
}

// NewWhisperLibraryManager creates a new library manager
func NewWhisperLibraryManager(dataPath string) *WhisperLibraryManager {
	libDir := filepath.Join(dataPath, "libs")
	os.MkdirAll(libDir, 0755)
	return &WhisperLibraryManager{
		downloader: downloader.NewModelDownloader(libDir),
		libPath:    libDir,
	}
}

// GetLibPath returns the library directory path
func (m *WhisperLibraryManager) GetLibPath() string {
	return m.libPath
}

// EnsureLibraries ensures all required libraries are downloaded
func (m *WhisperLibraryManager) EnsureLibraries() error {
	libs := m.getRequiredLibraries()
	for _, lib := range libs {
		if !m.libraryExists(lib) {
			if err := m.downloadLibrary(lib); err != nil {
				return fmt.Errorf("downloading %s: %w", lib, err)
			}
		}
	}
	return nil
}

func (m *WhisperLibraryManager) getRequiredLibraries() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{"whisper.dll", "opus.dll", "ggml.dll"}
	case "darwin":
		return []string{"libwhisper.dylib", "libopus.dylib", "libggml.dylib"}
	case "linux":
		return []string{"libwhisper.so", "libopus.so", "libggml.so"}
	default:
		return []string{}
	}
}

func (m *WhisperLibraryManager) libraryExists(lib string) bool {
	path := filepath.Join(m.libPath, lib)
	_, err := os.Stat(path)
	return err == nil
}

func (m *WhisperLibraryManager) downloadLibrary(lib string) error {
	// TODO: Implement CDN download using m.downloader.Download()
	// For now, libraries should be manually placed or built from third_party
	return fmt.Errorf("library %s not found in %s - please build from third_party or download manually", lib, m.libPath)
}
