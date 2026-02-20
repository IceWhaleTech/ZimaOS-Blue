//go:build !espeak

package tts

import "context"

// VocoderModelManager stub when espeak is not compiled in.
type VocoderModelManager struct{}

func NewVocoderModelManager(dataPath string) *VocoderModelManager {
	return &VocoderModelManager{}
}

func (m *VocoderModelManager) GetModelStatus() map[string]interface{} {
	return map[string]interface{}{
		"ready":       false,
		"downloading": false,
		"error":       "vocoder not compiled in",
	}
}

func (m *VocoderModelManager) DownloadModel(ctx context.Context) error {
	return ErrProviderDisabled
}

func (m *VocoderModelManager) CancelDownload() {}

func (m *VocoderModelManager) IsReady() bool { return false }
