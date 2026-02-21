//go:build whisper

package stt

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/downloader"
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

// ModelStatus represents the status of a model.
type ModelStatus struct {
	Ready       bool                         `json:"ready"`
	ModelType   string                       `json:"model_type,omitempty"`
	Downloading bool                         `json:"downloading"`
	HasPending  bool                         `json:"has_pending"`
	Progress    *downloader.DownloadProgress `json:"progress,omitempty"`
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
		Recommended: true,
	},
}

// GetAvailableASRModels returns available whisper models.
func GetAvailableASRModels() []WhisperModelInfo {
	return whisperModels
}

// WhisperModelManager manages whisper model downloads.
type WhisperModelManager struct {
	modelDir    string
	activeModel string
	dl          *downloader.ModelDownloader
	mu          sync.Mutex
}

// modelConfig holds persisted model configuration.
type modelConfig struct {
	ActiveModel string `json:"active_model"`
}

// NewWhisperModelManager creates a new model manager.
func NewWhisperModelManager(modelDir string) *WhisperModelManager {
	m := &WhisperModelManager{
		modelDir: modelDir,
		dl:       downloader.NewModelDownloader(modelDir),
	}
	m.loadConfig()
	return m
}

func (m *WhisperModelManager) configPath() string {
	return filepath.Join(m.modelDir, "config.json")
}

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

	modelType := m.activeModel
	downloading := m.dl.IsDownloading()
	if downloading {
		if p := m.dl.GetProgress(); p != nil && p.File != "" {
			for _, model := range whisperModels {
				if model.Filename == p.File {
					modelType = model.ID
					break
				}
			}
		}
	}

	return ModelStatus{
		Ready:       m.isModelDownloaded(m.activeModel),
		ModelType:   modelType,
		Downloading: downloading,
		Progress:    m.dl.GetProgress(),
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

// DownloadModel downloads a whisper model using the shared downloader.
func (m *WhisperModelManager) DownloadModel(ctx context.Context, modelType string) error {
	log.Printf("[WhisperModelManager] Download request for model: %s", modelType)
	var modelInfo *WhisperModelInfo
	for _, model := range whisperModels {
		if model.ID == modelType {
			modelInfo = &model
			break
		}
	}
	if modelInfo == nil {
		log.Printf("[WhisperModelManager] Unknown model: %s", modelType)
		return fmt.Errorf("unknown model: %s", modelType)
	}

	log.Printf("[WhisperModelManager] Model info: %s (%s) - %s", modelInfo.Name, modelInfo.Filename, modelInfo.Size)
	log.Printf("[WhisperModelManager] Primary URL: %s", modelInfo.URL)

	// Add HuggingFace mirrors for fallback
	mirrors := []string{
		fmt.Sprintf("https://hf-mirror.com/ggerganov/whisper.cpp/resolve/main/%s", modelInfo.Filename),
		fmt.Sprintf("https://modelscope.cn/models/ggerganov/whisper.cpp/resolve/main/%s", modelInfo.Filename),
	}
	log.Printf("[WhisperModelManager] Configured %d mirrors:", len(mirrors))
	for i, mirror := range mirrors {
		log.Printf("[WhisperModelManager]   Mirror %d: %s", i+1, mirror)
	}

	files := []downloader.ModelFile{
		{
			Filename: modelInfo.Filename,
			URL:      modelInfo.URL,
			Size:     modelInfo.Size,
			Mirrors:  mirrors,
		},
	}
	log.Printf("[WhisperModelManager] Starting download...")
	err := m.dl.Download(ctx, files)
	if err != nil {
		log.Printf("[WhisperModelManager] Download failed: %v", err)
	} else {
		log.Printf("[WhisperModelManager] Download completed successfully")
	}
	return err
}

// CancelDownload cancels the current download.
func (m *WhisperModelManager) CancelDownload() {
	m.dl.Cancel()
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
