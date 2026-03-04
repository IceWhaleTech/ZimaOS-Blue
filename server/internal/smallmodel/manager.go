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
	ModelID     = "qwen3.5-0.8b-onnx-q4"
	RuntimeType = "onnx_genai_python"

	onnxRepo = "onnx-community/Qwen3.5-0.8B-ONNX"
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
	modelDir := filepath.Join(dataDir, "models", ModelID)
	return &Manager{
		modelDir:   modelDir,
		downloader: downloader.NewModelDownloader(modelDir),
	}
}

func (m *Manager) ModelDir() string { return m.modelDir }

func (m *Manager) ModelPath() string {
	return filepath.Join(m.modelDir, "onnx", "decoder_model_merged_q4.onnx")
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
	if err := m.downloader.Download(ctx, requiredModelFiles()); err != nil {
		return fmt.Errorf("download small model: %w", err)
	}
	return nil
}

func (m *Manager) CancelDownload() {
	_ = m.downloader.Cancel()
}

func (m *Manager) GetStatus() Status {
	files := requiredModelFiles()
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
	for _, f := range requiredModelFiles() {
		if _, err := os.Stat(filepath.Join(m.modelDir, f.Filename)); err != nil {
			return false
		}
	}
	return true
}

func requiredModelFiles() []downloader.ModelFile {
	root := []downloader.ModelFile{
		modelFile("chat_template.jinja", "chat_template.jinja", "4.4KB"),
		modelFile("config.json", "config.json", "2.8KB"),
		modelFile("generation_config.json", "generation_config.json", "223B"),
		modelFile("preprocessor_config.json", "preprocessor_config.json", "502B"),
		modelFile("processor_config.json", "processor_config.json", "31.6KB"),
		modelFile("tokenizer.json", "tokenizer.json", "6.66MB"),
		modelFile("tokenizer_config.json", "tokenizer_config.json", "5.4KB"),
	}
	onnx := []downloader.ModelFile{
		modelFile("onnx/decoder_model_merged_q4.onnx", "onnx/decoder_model_merged_q4.onnx", "856KB"),
		modelFile("onnx/decoder_model_merged_q4.onnx_data", "onnx/decoder_model_merged_q4.onnx_data", "463MB"),
		modelFile("onnx/embed_tokens_q4.onnx", "onnx/embed_tokens_q4.onnx", "857B"),
		modelFile("onnx/embed_tokens_q4.onnx_data", "onnx/embed_tokens_q4.onnx_data", "155MB"),
		modelFile("onnx/vision_encoder_q4.onnx", "onnx/vision_encoder_q4.onnx", "181KB"),
		modelFile("onnx/vision_encoder_q4.onnx_data", "onnx/vision_encoder_q4.onnx_data", "65.1MB"),
	}
	return append(root, onnx...)
}

func modelFile(filename, repoPath, size string) downloader.ModelFile {
	return downloader.ModelFile{
		Filename: filename,
		URL:      "https://huggingface.co/" + onnxRepo + "/resolve/main/" + repoPath,
		Mirrors: []string{
			"https://hf-mirror.com/" + onnxRepo + "/resolve/main/" + repoPath,
			"https://modelscope.cn/models/" + onnxRepo + "/resolve/master/" + repoPath,
		},
		Size: size,
	}
}
