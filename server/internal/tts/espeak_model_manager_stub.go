//go:build !espeak || windows

package tts

import "context"

type EspeakModelManager struct{}

func NewEspeakModelManager(dataPath string) *EspeakModelManager {
	return &EspeakModelManager{}
}

func (m *EspeakModelManager) EnsureVoiceData(ctx context.Context) error {
	return nil
}
