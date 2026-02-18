package tts

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/downloader"
)

// KokoroModelManager manages Kokoro ONNX model downloads
type KokoroModelManager struct {
	downloader *downloader.ModelDownloader
	modelPath  string
	mu         sync.Mutex
}

// NewKokoroModelManager creates a new Kokoro model manager
func NewKokoroModelManager(dataPath string) *KokoroModelManager {
	destDir := filepath.Join(dataPath, "kokoro")
	return &KokoroModelManager{
		downloader: downloader.NewModelDownloader(destDir),
		modelPath:  filepath.Join(destDir, "model_q8f16.onnx"),
	}
}

// GetModelStatus returns the current model status
func (m *KokoroModelManager) GetModelStatus() map[string]interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()

	ready := false
	if _, err := os.Stat(m.modelPath); err == nil {
		ready = true
	}

	state, errMsg := m.downloader.GetState()
	return map[string]interface{}{
		"ready":       ready,
		"path":        m.modelPath,
		"state":       state,
		"error":       errMsg,
		"progress":    m.downloader.GetProgress(),
		"downloading": m.downloader.IsDownloading(),
	}
}

// DownloadModel starts downloading the Kokoro model
func (m *KokoroModelManager) DownloadModel(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Create destination directory
	destDir := filepath.Dir(m.modelPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create kokoro directory: %w", err)
	}

	// Define model file with mirrors
	files := []downloader.ModelFile{
		{
			Filename: "model_q8f16.onnx",
			URL:      "https://huggingface.co/onnx-community/Kokoro-82M-v1.0-ONNX/resolve/main/onnx/model_q8f16.onnx",
			Mirrors: []string{
				"https://hf-mirror.com/onnx-community/Kokoro-82M-v1.0-ONNX/resolve/main/onnx/model_q8f16.onnx",
			},
			Size: "~50MB",
		},
	}

	return m.downloader.Download(ctx, files)
}

// CancelDownload cancels the current download
func (m *KokoroModelManager) CancelDownload() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.downloader.Cancel()
}

// IsReady checks if the Kokoro model is ready
func (m *KokoroModelManager) IsReady() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, err := os.Stat(m.modelPath)
	return err == nil
}
