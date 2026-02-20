//go:build espeak

package tts

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/downloader"
)

// VocoderModelManager manages HiFi-GAN vocoder model downloads
type VocoderModelManager struct {
	downloader *downloader.ModelDownloader
	modelPath  string
	mu         sync.Mutex
}

// NewVocoderModelManager creates a new vocoder model manager
func NewVocoderModelManager(dataPath string) *VocoderModelManager {
	destDir := filepath.Join(dataPath, "vocoder")
	return &VocoderModelManager{
		downloader: downloader.NewModelDownloader(destDir),
		modelPath:  filepath.Join(destDir, "hifigan_v3.onnx"),
	}
}

// GetModelStatus returns the current model status
func (m *VocoderModelManager) GetModelStatus() map[string]interface{} {
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

// DownloadModel starts downloading the vocoder model
func (m *VocoderModelManager) DownloadModel(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Create destination directory
	destDir := filepath.Dir(m.modelPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create vocoder directory: %w", err)
	}

	// Define model files with mirrors
	// HiFi-GAN V3 ONNX model converted from jik876/hifi-gan LJ_V3 checkpoint
	// Mel params: sr=22050, n_fft=1024, hop=256, win=1024, n_mels=80, fmin=0, fmax=8000
	files := []downloader.ModelFile{
		{
			Filename: "hifigan_v3.onnx",
			URL:      "https://huggingface.co/orca-zhang/hifigan-v3-onnx/resolve/main/hifigan_v3.onnx",
			Mirrors: []string{
				"https://hf-mirror.com/orca-zhang/hifigan-v3-onnx/resolve/main/hifigan_v3.onnx",
				"https://modelscope.cn/models/orcazhang/hifigan-v3-onnx/resolve/master/hifigan_v3.onnx",
			},
			Size: "~5MB",
		},
	}

	return m.downloader.Download(ctx, files)
}

// CancelDownload cancels the current download
func (m *VocoderModelManager) CancelDownload() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.downloader.Cancel()
}

// IsReady checks if the vocoder model is ready
func (m *VocoderModelManager) IsReady() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, err := os.Stat(m.modelPath)
	return err == nil
}
