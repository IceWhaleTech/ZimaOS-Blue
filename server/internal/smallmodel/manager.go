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

	defaultHFRepo         = "unsloth/Qwen3.5-0.8B-GGUF"
	defaultModelFilename  = "Qwen3.5-0.8B.Q4_K_M.gguf"
	defaultMMProjFilename = "mmproj-F16.gguf"
	defaultModelSize      = "528MB"
	defaultMMProjSize     = "209.5MB"
)

var defaultModelDownloadCandidates = []string{
	"https://huggingface.co/unsloth/Qwen3.5-0.8B-GGUF/resolve/main/Qwen3.5-0.8B-Q4_K_M.gguf",
	"https://hf-mirror.com/unsloth/Qwen3.5-0.8B-GGUF/resolve/main/Qwen3.5-0.8B-Q4_K_M.gguf",
	"https://modelscope.cn/models/unsloth/Qwen3.5-0.8B-GGUF/resolve/master/Qwen3.5-0.8B-Q4_K_M.gguf",
}

var legacyModelDownloadRepos = map[string]struct{}{
	defaultHFRepo: {},
}

var legacyModelDownloadFilenames = map[string]struct{}{
	defaultModelFilename:       {},
	"Qwen3.5-0.8B-Q4_K_M.gguf": {},
	"qwen3.5-0.8b-q4_k_m.gguf": {},
}

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
	if strings.TrimSpace(m.assets.mmprojFilename) == "" {
		return ""
	}
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
	files := []downloader.ModelFile{
		modelDownloadFile(assets),
	}
	if strings.TrimSpace(assets.mmprojFilename) != "" {
		files = append(files, modelFile(assets.repo, assets.mmprojFilename, assets.mmprojFilename, defaultMMProjSize))
	}
	return files
}

func modelDownloadFile(assets modelAssetConfig) downloader.ModelFile {
	if usesDefaultModelDownloadCandidates(assets) {
		return modelFileWithURLs(assets.modelFilename, defaultModelSize, defaultModelDownloadCandidates...)
	}
	return modelFile(assets.repo, assets.modelFilename, assets.modelFilename, defaultModelSize)
}

func modelFileWithURLs(filename, size string, urls ...string) downloader.ModelFile {
	if len(urls) == 0 {
		return downloader.ModelFile{Filename: filename, Size: size}
	}
	return downloader.ModelFile{
		Filename: filename,
		URL:      urls[0],
		Mirrors:  append([]string(nil), urls[1:]...),
		Size:     size,
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

func usesDefaultModelDownloadCandidates(assets modelAssetConfig) bool {
	_, repoOK := legacyModelDownloadRepos[strings.TrimSpace(assets.repo)]
	_, filenameOK := legacyModelDownloadFilenames[strings.TrimSpace(assets.modelFilename)]
	return repoOK && filenameOK
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
