//go:build !whisper

package stt

import (
	"context"
	"fmt"
	"os"
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

// WhisperProvider is a stub when built without CGo.
// Use the external ASR plugin (libblue-asr) for actual whisper support.
type WhisperProvider struct {
	config       *WhisperConfig
	modelManager *WhisperModelManager
	mu           sync.Mutex
	initialized  bool
}

// NewWhisperProvider creates a stub Whisper provider (no CGo).
func NewWhisperProvider(cfg *WhisperConfig) *WhisperProvider {
	if cfg.MaxDuration == 0 {
		cfg.MaxDuration = 30 * time.Second
	}
	return &WhisperProvider{
		config:       cfg,
		modelManager: NewWhisperModelManager(cfg.ModelPath),
	}
}

func (p *WhisperProvider) Initialize(modelPath string) error {
	return fmt.Errorf("whisper not available: build with -tags whisper to enable, or use ASR plugin")
}

func (p *WhisperProvider) Close() {}

func (p *WhisperProvider) Name() string        { return "Whisper (stub)" }
func (p *WhisperProvider) Type() ProviderType  { return ProviderWhisper }

func (p *WhisperProvider) Transcribe(_ context.Context, _ *TranscribeRequest) (*TranscribeResponse, error) {
	return nil, fmt.Errorf("whisper not available: build with -tags whisper")
}

func (p *WhisperProvider) TranscribeStream(_ context.Context, _ *TranscribeRequest, _ StreamCallback) error {
	return fmt.Errorf("whisper not available: build with -tags whisper")
}

func (p *WhisperProvider) SupportedFormats() []AudioFormat {
	return nil
}

func (p *WhisperProvider) MaxDuration() time.Duration {
	return p.config.MaxDuration
}

func (p *WhisperProvider) IsInitialized() bool { return false }

func (p *WhisperProvider) GetModelStatus() ModelStatus {
	return p.modelManager.GetModelStatus()
}

func (p *WhisperProvider) ListModels() []interface{} {
	return p.modelManager.ListModels()
}

func (p *WhisperProvider) DownloadModel(ctx context.Context, modelType string) error {
	return p.modelManager.DownloadModel(ctx, modelType)
}

func (p *WhisperProvider) CancelDownload() {
	p.modelManager.CancelDownload()
}

func (p *WhisperProvider) SwitchModel(modelType string) error {
	modelPath := p.modelManager.GetModelPath(modelType)
	if modelPath == "" {
		return fmt.Errorf("unknown model: %s", modelType)
	}
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return fmt.Errorf("model not downloaded: %s", modelType)
	}
	return fmt.Errorf("whisper not available: build with -tags whisper")
}
