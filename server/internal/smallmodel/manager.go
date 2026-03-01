package smallmodel

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/downloader"
)

const (
	// Fixed model/runtime per product decision.
	ModelID     = "lfm2.5-1.2b-instruct-q4km"
	RuntimeType = "llama_cpp_native"

	modelFilename = "LFM2.5-1.2B-Instruct-Q4_K_M.gguf"
)

// Fallback reason codes for observability.
const (
	FallbackReasonModelUnready  = "model_unready"
	FallbackReasonTimeout       = "timeout"
	FallbackReasonCircuitOpen   = "circuit_open"
	FallbackReasonLowConfidence = "low_confidence"
	FallbackReasonSchemaInvalid = "schema_invalid"
	FallbackReasonResourceGuard = "resource_guard"
)

type DownloadProgress struct {
	File       string  `json:"file"`
	FileIndex  int     `json:"file_index"`
	TotalFiles int     `json:"total_files"`
	Downloaded int64   `json:"downloaded"`
	Total      int64   `json:"total"`
	Percentage float64 `json:"percentage"`
	SpeedHuman string  `json:"speed_human"`
	ETA        string  `json:"eta"`
}

type FileStatus struct {
	Filename   string `json:"filename"`
	Downloaded bool   `json:"downloaded"`
	Size       string `json:"size"`
}

type Status struct {
	Ready       bool              `json:"ready"`
	Downloading bool              `json:"downloading"`
	State       string            `json:"state,omitempty"`
	Error       string            `json:"error,omitempty"`
	ModelID     string            `json:"model_id"`
	Runtime     string            `json:"runtime"`
	ModelPath   string            `json:"model_path"`
	Progress    *DownloadProgress `json:"progress,omitempty"`
	Files       []FileStatus      `json:"files,omitempty"`
}

// Manager controls fixed small-model artifacts.
type Manager struct {
	modelDir   string
	downloader *downloader.ModelDownloader
	mu         sync.Mutex
}

func NewManager(dataDir string) *Manager {
	modelDir := filepath.Join(dataDir, "small-model", ModelID)
	return &Manager{
		modelDir:   modelDir,
		downloader: downloader.NewModelDownloader(modelDir),
	}
}

func (m *Manager) ModelDir() string { return m.modelDir }

func (m *Manager) ModelPath() string {
	return filepath.Join(m.modelDir, modelFilename)
}

func (m *Manager) IsReady() bool {
	st := m.GetStatus()
	return st.Ready
}

func (m *Manager) EnsureReady(ctx context.Context, allowDownload bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := os.Stat(m.ModelPath()); err == nil {
		return nil
	}
	if !allowDownload {
		return fmt.Errorf("small model is not ready and auto download is disabled")
	}
	return m.downloadLocked(ctx)
}

func (m *Manager) Download(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.downloadLocked(ctx)
}

func (m *Manager) downloadLocked(ctx context.Context) error {
	if err := os.MkdirAll(m.modelDir, 0o755); err != nil {
		return fmt.Errorf("create small model dir: %w", err)
	}
	if err := m.downloader.Download(ctx, []downloader.ModelFile{m.modelFile()}); err != nil {
		return fmt.Errorf("download small model: %w", err)
	}
	return nil
}

func (m *Manager) CancelDownload() {
	_ = m.downloader.Cancel()
}

func (m *Manager) GetStatus() Status {
	_, statErr := os.Stat(m.ModelPath())
	downloaded := statErr == nil
	state, lastError := m.downloader.GetState()
	downloading := m.downloader.IsDownloading()

	var progress *DownloadProgress
	if p := m.downloader.GetProgress(); p != nil {
		progress = &DownloadProgress{
			File:       p.File,
			FileIndex:  p.FileIndex,
			TotalFiles: p.TotalFiles,
			Downloaded: p.Downloaded,
			Total:      p.Total,
			Percentage: p.Percentage,
			SpeedHuman: p.SpeedHuman,
			ETA:        p.ETA,
		}
	}

	return Status{
		Ready:       downloaded && !downloading,
		Downloading: downloading,
		State:       state,
		Error:       lastError,
		ModelID:     ModelID,
		Runtime:     RuntimeType,
		ModelPath:   m.ModelPath(),
		Progress:    progress,
		Files: []FileStatus{
			{
				Filename:   modelFilename,
				Downloaded: downloaded,
				Size:       "~0.8GB",
			},
		},
	}
}

func (m *Manager) modelFile() downloader.ModelFile {
	return downloader.ModelFile{
		Filename: modelFilename,
		URL:      "https://huggingface.co/LiquidAI/LFM2.5-1.2B-Instruct-GGUF/resolve/main/LFM2.5-1.2B-Instruct-Q4_K_M.gguf",
		Mirrors: []string{
			"https://hf-mirror.com/LiquidAI/LFM2.5-1.2B-Instruct-GGUF/resolve/main/LFM2.5-1.2B-Instruct-Q4_K_M.gguf",
			"https://modelscope.cn/models/LiquidAI/LFM2.5-1.2B-Instruct-GGUF/resolve/master/LFM2.5-1.2B-Instruct-Q4_K_M.gguf",
		},
		Size: "~0.8GB",
	}
}
