//go:build !whisper

package stt

import (
	"context"

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

// GetAvailableASRModels returns empty list when whisper is not compiled in.
func GetAvailableASRModels() []WhisperModelInfo {
	return []WhisperModelInfo{}
}

// WhisperModelManager stub when whisper is not compiled in.
type WhisperModelManager struct{}

func NewWhisperModelManager(modelDir string) *WhisperModelManager {
	return &WhisperModelManager{}
}

func (m *WhisperModelManager) SetActiveModel(modelType string) {}

func (m *WhisperModelManager) GetActiveModel() string {
	return ""
}

func (m *WhisperModelManager) GetModelStatus() ModelStatus {
	return ModelStatus{Ready: false}
}

func (m *WhisperModelManager) ListModels() []interface{} {
	return []interface{}{}
}

func (m *WhisperModelManager) DownloadModel(ctx context.Context, modelType string) error {
	return nil
}

func (m *WhisperModelManager) CancelDownload() {}

func (m *WhisperModelManager) GetModelPath(modelType string) string {
	return ""
}

func (m *WhisperModelManager) GetModelIDFromPath(modelPath string) string {
	return ""
}
