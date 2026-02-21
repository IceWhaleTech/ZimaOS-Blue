//go:build !kokoro || windows

package tts

import "context"

type KokoroModelManager struct{}

func NewKokoroModelManager(dataPath string) *KokoroModelManager {
	return &KokoroModelManager{}
}

func (m *KokoroModelManager) DownloadModel(ctx context.Context) error {
	return ErrProviderDisabled
}

func (m *KokoroModelManager) GetDownloadProgress() (float64, string, error) {
	return 0, "", ErrProviderDisabled
}

func (m *KokoroModelManager) CancelDownload() error {
	return ErrProviderDisabled
}

func (m *KokoroModelManager) IsModelReady() bool {
	return false
}

func (m *KokoroModelManager) GetModelPath() string {
	return ""
}

func (m *KokoroModelManager) GetModelStatus() map[string]interface{} {
	return map[string]interface{}{
		"ready":      false,
		"downloaded": false,
		"error":      "Kokoro provider not compiled in",
	}
}
