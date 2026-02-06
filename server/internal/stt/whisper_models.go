package stt

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// WhisperModelInfo contains metadata about a whisper model.
type WhisperModelInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Size        string `json:"size"`
	URL         string `json:"url"`
	Filename    string `json:"filename"`
	Downloaded  bool   `json:"downloaded"`
	Active      bool   `json:"active"`
	Recommended bool   `json:"recommended"`
}

// DownloadProgress tracks download progress.
type DownloadProgress struct {
	File       string  `json:"file"`
	Downloaded int64   `json:"downloaded"`
	Total      int64   `json:"total"`
	Percentage float64 `json:"percentage"`
	SpeedHuman string  `json:"speed_human"`
	ETA        string  `json:"eta"`
}

// ModelStatus represents the status of a model.
type ModelStatus struct {
	Ready       bool              `json:"ready"`
	ModelType   string            `json:"model_type,omitempty"`
	Downloading bool              `json:"downloading"`
	HasPending  bool              `json:"has_pending"`
	Progress    *DownloadProgress `json:"progress,omitempty"`
}

var whisperModels = []WhisperModelInfo{
	{
		ID:          "whisper-tiny",
		Name:        "speech.asrModelInfo.whisperTiny.name",
		Description: "speech.asrModelInfo.whisperTiny.description",
		Size:        "75 MB",
		URL:         "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.bin",
		Filename:    "ggml-tiny.bin",
	},
	{
		ID:          "whisper-base",
		Name:        "speech.asrModelInfo.whisperBase.name",
		Description: "speech.asrModelInfo.whisperBase.description",
		Size:        "142 MB",
		URL:         "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.bin",
		Filename:    "ggml-base.bin",
		Recommended: true,
	},
	{
		ID:          "whisper-small",
		Name:        "speech.asrModelInfo.whisperSmall.name",
		Description: "speech.asrModelInfo.whisperSmall.description",
		Size:        "466 MB",
		URL:         "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin",
		Filename:    "ggml-small.bin",
	},
	{
		ID:          "whisper-large-v3-turbo",
		Name:        "speech.asrModelInfo.whisperLargeTurbo.name",
		Description: "speech.asrModelInfo.whisperLargeTurbo.description",
		Size:        "547 MB",
		URL:         "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3-turbo-q5_0.bin",
		Filename:    "ggml-large-v3-turbo-q5_0.bin",
	},
}

// GetAvailableASRModels returns available whisper models.
func GetAvailableASRModels() []WhisperModelInfo {
	return whisperModels
}

// WhisperModelManager manages whisper model downloads.
type WhisperModelManager struct {
	modelDir     string
	activeModel  string
	downloading  bool
	currentModel string
	progress     *DownloadProgress
	cancelFunc   context.CancelFunc
	mu           sync.Mutex
}

// modelConfig holds persisted model configuration.
type modelConfig struct {
	ActiveModel string `json:"active_model"`
}

// NewWhisperModelManager creates a new model manager.
func NewWhisperModelManager(modelDir string) *WhisperModelManager {
	m := &WhisperModelManager{modelDir: modelDir}
	m.loadConfig()
	return m
}

// configPath returns the path to the config file.
func (m *WhisperModelManager) configPath() string {
	return filepath.Join(m.modelDir, "config.json")
}

// loadConfig loads the persisted configuration.
func (m *WhisperModelManager) loadConfig() {
	data, err := os.ReadFile(m.configPath())
	if err != nil {
		return
	}
	var cfg modelConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return
	}
	m.activeModel = cfg.ActiveModel
}

// saveConfig persists the current configuration.
func (m *WhisperModelManager) saveConfig() {
	cfg := modelConfig{ActiveModel: m.activeModel}
	data, err := json.Marshal(cfg)
	if err != nil {
		return
	}
	os.MkdirAll(m.modelDir, 0755)
	os.WriteFile(m.configPath(), data, 0644)
}

// SetActiveModel sets the currently active model and persists it.
func (m *WhisperModelManager) SetActiveModel(modelType string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activeModel = modelType
	m.saveConfig()
}

// GetActiveModel returns the currently active model.
func (m *WhisperModelManager) GetActiveModel() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.activeModel
}

// GetModelStatus returns the status of the active model.
func (m *WhisperModelManager) GetModelStatus() ModelStatus {
	m.mu.Lock()
	defer m.mu.Unlock()

	// When downloading, return the model being downloaded, not the active model
	modelType := m.activeModel
	if m.downloading && m.currentModel != "" {
		modelType = m.currentModel
	}

	return ModelStatus{
		Ready:       m.isModelDownloaded(m.activeModel),
		ModelType:   modelType,
		Downloading: m.downloading,
		Progress:    m.progress,
	}
}

// ListModels returns all models with download status.
func (m *WhisperModelManager) ListModels() []interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]interface{}, len(whisperModels))
	for i, model := range whisperModels {
		model.Downloaded = m.isModelDownloaded(model.ID)
		model.Active = model.ID == m.activeModel
		result[i] = model
	}
	return result
}

func (m *WhisperModelManager) isModelDownloaded(modelType string) bool {
	for _, model := range whisperModels {
		if model.ID == modelType {
			path := filepath.Join(m.modelDir, model.Filename)
			if _, err := os.Stat(path); err == nil {
				return true
			}
		}
	}
	return false
}

// DownloadModel downloads a whisper model.
func (m *WhisperModelManager) DownloadModel(ctx context.Context, modelType string) error {
	m.mu.Lock()
	if m.downloading {
		m.mu.Unlock()
		return fmt.Errorf("download already in progress")
	}

	var modelInfo *WhisperModelInfo
	for _, model := range whisperModels {
		if model.ID == modelType {
			modelInfo = &model
			break
		}
	}
	if modelInfo == nil {
		m.mu.Unlock()
		return fmt.Errorf("unknown model: %s", modelType)
	}

	m.downloading = true
	m.currentModel = modelType
	m.progress = &DownloadProgress{File: modelInfo.Filename}

	ctx, m.cancelFunc = context.WithCancel(ctx)
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.downloading = false
		m.cancelFunc = nil
		m.mu.Unlock()
	}()

	if err := os.MkdirAll(m.modelDir, 0755); err != nil {
		return err
	}

	destPath := filepath.Join(m.modelDir, modelInfo.Filename)
	return m.downloadFile(ctx, modelInfo.URL, destPath)
}

func (m *WhisperModelManager) downloadFile(ctx context.Context, url, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	out, err := os.Create(destPath + ".tmp")
	if err != nil {
		return err
	}
	defer out.Close()

	m.mu.Lock()
	m.progress.Total = resp.ContentLength
	m.mu.Unlock()

	startTime := time.Now()
	buf := make([]byte, 32*1024)
	var downloaded int64

	for {
		select {
		case <-ctx.Done():
			os.Remove(destPath + ".tmp")
			return ctx.Err()
		default:
		}

		n, err := resp.Body.Read(buf)
		if n > 0 {
			out.Write(buf[:n])
			downloaded += int64(n)

			m.mu.Lock()
			m.progress.Downloaded = downloaded
			if m.progress.Total > 0 {
				m.progress.Percentage = float64(downloaded) / float64(m.progress.Total) * 100
			}
			elapsed := time.Since(startTime).Seconds()
			if elapsed > 0 {
				speed := float64(downloaded) / elapsed
				m.progress.SpeedHuman = formatSpeed(speed)
				if speed > 0 && m.progress.Total > 0 {
					remaining := float64(m.progress.Total-downloaded) / speed
					m.progress.ETA = formatDuration(remaining)
				}
			}
			m.mu.Unlock()
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			os.Remove(destPath + ".tmp")
			return err
		}
	}

	return os.Rename(destPath+".tmp", destPath)
}

// CancelDownload cancels the current download.
func (m *WhisperModelManager) CancelDownload() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancelFunc != nil {
		m.cancelFunc()
	}
}

// GetModelPath returns the path to a downloaded model.
func (m *WhisperModelManager) GetModelPath(modelType string) string {
	for _, model := range whisperModels {
		if model.ID == modelType {
			return filepath.Join(m.modelDir, model.Filename)
		}
	}
	return ""
}

// GetModelIDFromPath returns the model ID from a file path.
func (m *WhisperModelManager) GetModelIDFromPath(modelPath string) string {
	filename := filepath.Base(modelPath)
	for _, model := range whisperModels {
		if model.Filename == filename {
			return model.ID
		}
	}
	return ""
}

func formatSpeed(bytesPerSec float64) string {
	if bytesPerSec >= 1024*1024 {
		return fmt.Sprintf("%.1f MB/s", bytesPerSec/1024/1024)
	}
	return fmt.Sprintf("%.1f KB/s", bytesPerSec/1024)
}

func formatDuration(seconds float64) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", int(seconds))
	}
	return fmt.Sprintf("%dm%ds", int(seconds)/60, int(seconds)%60)
}
