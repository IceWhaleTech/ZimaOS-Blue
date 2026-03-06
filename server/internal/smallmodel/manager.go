package smallmodel

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/downloader"
)

const (
	// Fixed model/runtime per product decision.
	ModelID     = "qwen3.5-0.8b-gguf-q4km"
	RuntimeType = "llama.cpp"

	defaultHFRepo         = "Qwen/Qwen3.5-0.8B-GGUF"
	defaultModelFilename  = "qwen3.5-0.8b-q4_k_m.gguf"
	defaultMMProjFilename = "mmproj-model-bf16.gguf"
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

type modelAssetConfig struct {
	repo           string
	modelFilename  string
	mmprojFilename string
}

// Manager controls fixed small-model artifacts.
type Manager struct {
	modelDir   string
	assets     modelAssetConfig
	downloader *downloader.ModelDownloader
	mu         sync.Mutex
}

func NewManager(dataDir string) *Manager {
	modelDir := filepath.Join(dataDir, "models", ModelID)
	assets := resolveModelAssetConfig()
	return &Manager{
		modelDir:   modelDir,
		assets:     assets,
		downloader: downloader.NewModelDownloader(modelDir),
	}
}

func (m *Manager) ModelDir() string { return m.modelDir }

func (m *Manager) ModelPath() string {
	return filepath.Join(m.modelDir, m.assets.modelFilename)
}

func (m *Manager) MMProjPath() string {
	return filepath.Join(m.modelDir, m.assets.mmprojFilename)
}

func (m *Manager) IsReady() bool {
	st := m.GetStatus()
	return st.Ready
}

func (m *Manager) EnsureReady(ctx context.Context, allowDownload bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.filesReadyLocked() {
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
	if err := m.downloader.Download(ctx, requiredModelFiles(m.assets)); err != nil {
		return fmt.Errorf("download small model: %w", err)
	}
	return nil
}

func (m *Manager) CancelDownload() {
	_ = m.downloader.Cancel()
}

func (m *Manager) GetStatus() Status {
	files := requiredModelFiles(m.assets)
	allDownloaded := true
	fileStatuses := make([]FileStatus, 0, len(files))
	for _, f := range files {
		_, statErr := os.Stat(filepath.Join(m.modelDir, f.Filename))
		downloaded := statErr == nil
		allDownloaded = allDownloaded && downloaded
		fileStatuses = append(fileStatuses, FileStatus{
			Filename:   f.Filename,
			Downloaded: downloaded,
			Size:       f.Size,
		})
	}
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
		Ready:       allDownloaded && !downloading,
		Downloading: downloading,
		State:       state,
		Error:       lastError,
		ModelID:     ModelID,
		Runtime:     RuntimeType,
		ModelPath:   m.ModelPath(),
		Progress:    progress,
		Files:       fileStatuses,
	}
}

func (m *Manager) filesReadyLocked() bool {
	for _, f := range requiredModelFiles(m.assets) {
		if _, err := os.Stat(filepath.Join(m.modelDir, f.Filename)); err != nil {
			return false
		}
	}
	return true
}

func requiredModelFiles(assets modelAssetConfig) []downloader.ModelFile {
	return []downloader.ModelFile{
		modelFile(assets.repo, assets.modelFilename, assets.modelFilename, "2.0GB"),
		modelFile(assets.repo, assets.mmprojFilename, assets.mmprojFilename, "1.0GB"),
	}
}

func modelFile(repo, filename, repoPath, size string) downloader.ModelFile {
	return downloader.ModelFile{
		Filename: filename,
		URL:      "https://huggingface.co/" + repo + "/resolve/main/" + repoPath,
		Mirrors: []string{
			"https://hf-mirror.com/" + repo + "/resolve/main/" + repoPath,
			"https://modelscope.cn/models/" + repo + "/resolve/master/" + repoPath,
		},
		Size: size,
	}
}

func resolveModelAssetConfig() modelAssetConfig {
	repo := strings.TrimSpace(os.Getenv("SMALL_MODEL_HF_REPO"))
	if repo == "" {
		repo = defaultHFRepo
	}
	modelFilename := strings.TrimSpace(os.Getenv("SMALL_MODEL_GGUF_FILENAME"))
	if modelFilename == "" {
		modelFilename = defaultModelFilename
	}
	mmprojFilename := strings.TrimSpace(os.Getenv("SMALL_MODEL_MMPROJ_FILENAME"))
	if mmprojFilename == "" {
		mmprojFilename = defaultMMProjFilename
	}
	return modelAssetConfig{
		repo:           repo,
		modelFilename:  modelFilename,
		mmprojFilename: mmprojFilename,
	}
}
