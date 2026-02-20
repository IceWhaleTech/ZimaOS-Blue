//go:build !cgo || !whisper

package stt

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// WhisperConfig holds the configuration for the Whisper provider.
type WhisperConfig struct {
	ModelPath   string
	DefaultLang string
	MaxDuration time.Duration
	Threads     int
}

// WhisperProvider is a stub when whisper is not compiled in.
type WhisperProvider struct {
	config       *WhisperConfig
	modelManager *WhisperModelManager
	mu           sync.Mutex
	initialized  bool
}

// NewWhisperProvider creates a stub Whisper provider (whisper not compiled in).
func NewWhisperProvider(cfg *WhisperConfig) *WhisperProvider {
	return &WhisperProvider{
		config:       cfg,
		modelManager: NewWhisperModelManager(cfg.ModelPath),
	}
}

func (p *WhisperProvider) Initialize(modelPath string) error {
	return fmt.Errorf("whisper not compiled in")
}

func (p *WhisperProvider) Close() {}

func (p *WhisperProvider) Name() string       { return "Whisper (stub)" }
func (p *WhisperProvider) Type() ProviderType { return ProviderWhisper }

func (p *WhisperProvider) Transcribe(ctx context.Context, req *TranscribeRequest) (*TranscribeResponse, error) {
	return nil, fmt.Errorf("whisper not compiled in")
}

func (p *WhisperProvider) TranscribeStream(ctx context.Context, req *TranscribeRequest, callback StreamCallback) error {
	return fmt.Errorf("whisper not compiled in")
}

func (p *WhisperProvider) SupportedFormats() []AudioFormat {
	return []AudioFormat{FormatWAV, FormatPCM}
}

func (p *WhisperProvider) MaxDuration() time.Duration {
	if p.config != nil && p.config.MaxDuration > 0 {
		return p.config.MaxDuration
	}
	return 30 * time.Second
}

func (p *WhisperProvider) IsInitialized() bool { return false }

func (p *WhisperProvider) GetModelStatus() ModelStatus {
	return ModelStatus{Ready: false}
}

func (p *WhisperProvider) ListModels() []interface{} {
	return p.modelManager.ListModels()
}

func (p *WhisperProvider) DownloadModel(ctx context.Context, modelType string) error {
	return fmt.Errorf("whisper not compiled in")
}

func (p *WhisperProvider) CancelDownload() {}

func (p *WhisperProvider) SwitchModel(modelType string) error {
	return fmt.Errorf("whisper not compiled in")
}
