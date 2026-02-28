package claudecode

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/downloader"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/onnx"
)

const (
	defaultSkillRerankerRepo = "cross-encoder/ms-marco-MiniLM-L6-v2"
	legacySkillRerankerRepo  = "cross-encoder/ms-marco-MiniLM-L-6-v2"
)

// SkillRerankerModelDownloadProgress tracks ONNX model download progress.
type SkillRerankerModelDownloadProgress struct {
	File       string  `json:"file"`
	FileIndex  int     `json:"file_index"`
	TotalFiles int     `json:"total_files"`
	Downloaded int64   `json:"downloaded"`
	Total      int64   `json:"total"`
	Percentage float64 `json:"percentage"`
	SpeedHuman string  `json:"speed_human"`
	ETA        string  `json:"eta"`
}

// SkillRerankerFileStatus reports single file readiness for UI display.
type SkillRerankerFileStatus struct {
	Filename   string `json:"filename"`
	Downloaded bool   `json:"downloaded"`
	Size       string `json:"size"`
}

// SkillRerankerModelStatus is the API payload for model download/ready status.
type SkillRerankerModelStatus struct {
	Ready       bool                                `json:"ready"`
	Downloading bool                                `json:"downloading"`
	State       string                              `json:"state,omitempty"`
	Error       string                              `json:"error,omitempty"`
	Progress    *SkillRerankerModelDownloadProgress `json:"progress,omitempty"`
	Files       []SkillRerankerFileStatus           `json:"files,omitempty"`
}

// SkillRerankerModelManager manages ONNX reranker model download and load validation.
type SkillRerankerModelManager struct {
	modelDir   string
	repo       string
	downloader *downloader.ModelDownloader
	mu         sync.Mutex
}

func NewSkillRerankerModelManager(dataDir, repo string) *SkillRerankerModelManager {
	repo = strings.TrimSpace(repo)
	switch repo {
	case "":
		repo = defaultSkillRerankerRepo
	case legacySkillRerankerRepo:
		repo = defaultSkillRerankerRepo
	}
	modelDir := filepath.Join(dataDir, "skill-reranker")
	return &SkillRerankerModelManager{
		modelDir:   modelDir,
		repo:       repo,
		downloader: downloader.NewModelDownloader(modelDir),
	}
}

func (m *SkillRerankerModelManager) ModelPath() string {
	return filepath.Join(m.modelDir, "model.onnx")
}

// GetStatus returns current model file/download status for UI.
func (m *SkillRerankerModelManager) GetStatus() SkillRerankerModelStatus {
	modelFile := m.modelFile()
	_, statErr := os.Stat(m.ModelPath())
	downloaded := statErr == nil

	state, lastError := m.downloader.GetState()
	downloading := m.downloader.IsDownloading()

	var progress *SkillRerankerModelDownloadProgress
	if p := m.downloader.GetProgress(); p != nil {
		progress = &SkillRerankerModelDownloadProgress{
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

	return SkillRerankerModelStatus{
		Ready:       downloaded && !downloading,
		Downloading: downloading,
		State:       state,
		Error:       lastError,
		Progress:    progress,
		Files: []SkillRerankerFileStatus{
			{
				Filename:   modelFile.Filename,
				Downloaded: downloaded,
				Size:       modelFile.Size,
			},
		},
	}
}

func (m *SkillRerankerModelManager) IsReady() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := os.Stat(m.ModelPath()); err != nil {
		return false
	}
	return m.validateModelLoad() == nil
}

func (m *SkillRerankerModelManager) EnsureReady(ctx context.Context, allowDownload bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ensureReadyInner(ctx, allowDownload)
}

func (m *SkillRerankerModelManager) ensureReadyInner(ctx context.Context, allowDownload bool) error {
	if _, err := os.Stat(m.ModelPath()); err == nil {
		if err := m.validateModelLoad(); err == nil {
			return nil
		}
		// Corrupted or incompatible local model; remove and re-download.
		_ = os.Remove(m.ModelPath())
	}
	if !allowDownload {
		return fmt.Errorf("skill reranker model is not ready and auto download is disabled")
	}

	return m.downloadModelInner(ctx)
}

// Download starts downloading/validating the ONNX reranker model.
func (m *SkillRerankerModelManager) Download(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, err := os.Stat(m.ModelPath()); err == nil {
		if err := m.validateModelLoad(); err == nil {
			return nil
		}
		// Corrupted or incompatible local model; remove and re-download.
		_ = os.Remove(m.ModelPath())
	}

	return m.downloadModelInner(ctx)
}

// CancelDownload cancels current background download.
func (m *SkillRerankerModelManager) CancelDownload() {
	_ = m.downloader.Cancel()
}

func (m *SkillRerankerModelManager) downloadModelInner(ctx context.Context) error {
	if err := os.MkdirAll(m.modelDir, 0o755); err != nil {
		return fmt.Errorf("create skill reranker dir: %w", err)
	}

	files := []downloader.ModelFile{m.modelFile()}
	if err := m.downloader.Download(ctx, files); err != nil {
		return fmt.Errorf("download skill reranker model: %w", err)
	}
	if err := m.validateModelLoad(); err != nil {
		_ = os.Remove(m.ModelPath())
		return fmt.Errorf("load-check skill reranker model: %w", err)
	}
	return nil
}

func (m *SkillRerankerModelManager) modelFile() downloader.ModelFile {
	hfOnnxURL := fmt.Sprintf("https://huggingface.co/%s/resolve/main/onnx/model.onnx", m.repo)
	modelscopeOnnxURL := fmt.Sprintf("https://modelscope.cn/models/%s/resolve/master/onnx/model.onnx", m.repo)
	hfMirrorOnnxURL := fmt.Sprintf("https://hf-mirror.com/%s/resolve/main/onnx/model.onnx", m.repo)
	hfRootURL := fmt.Sprintf("https://huggingface.co/%s/resolve/main/model.onnx", m.repo)
	modelscopeRootURL := fmt.Sprintf("https://modelscope.cn/models/%s/resolve/master/model.onnx", m.repo)
	hfMirrorRootURL := fmt.Sprintf("https://hf-mirror.com/%s/resolve/main/model.onnx", m.repo)

	return downloader.ModelFile{
		Filename: "model.onnx",
		URL:      hfOnnxURL,
		Mirrors: []string{
			modelscopeOnnxURL,
			hfMirrorOnnxURL,
			modelscopeRootURL,
			hfRootURL,
			hfMirrorRootURL,
		},
		Size: "~90MB",
	}
}

// validateModelLoad checks whether downloaded model can be opened by ONNX Runtime.
// This is the acceptance criterion instead of checksum.
func (m *SkillRerankerModelManager) validateModelLoad() error {
	modelPath := m.ModelPath()
	if _, err := os.Stat(modelPath); err != nil {
		return err
	}

	dataPath := filepath.Dir(m.modelDir)
	onnx.SetDataDir(dataPath)
	if libPath := onnx.RuntimeLibPath(dataPath); libPath != "" {
		onnx.SetLibraryPath(libPath)
	}

	inputNameSets := [][]string{
		{"input_ids", "attention_mask", "token_type_ids"},
		{"input_ids", "attention_mask"},
		{"input_ids"},
	}
	outputNameSets := [][]string{
		{"logits"},
		{"output_0"},
		{"output"},
	}

	var lastErr error
	for _, in := range inputNameSets {
		for _, out := range outputNameSets {
			s, err := onnx.NewDynamicSession(modelPath, in, out)
			if err == nil {
				_ = s.Close()
				return nil
			}
			lastErr = err
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("unknown model load error")
	}
	return lastErr
}

func (m *SkillRerankerModelManager) WarmupAsync(allowDownload bool) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		_ = m.EnsureReady(ctx, allowDownload)
	}()
}
