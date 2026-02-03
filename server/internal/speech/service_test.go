package speech

import (
	"testing"
)

func TestNewService(t *testing.T) {
	cfg := &Config{
		TTS: TTSConfig{Provider: "sherpa-onnx"},
		ASR: ASRConfig{Provider: "sherpa"},
	}

	svc := NewService(cfg, nil, nil)
	if svc == nil {
		t.Fatal("NewService returned nil")
	}

	// Service is initialized when created with NewService
	if svc.IsInitialized() {
		t.Error("Service should not be initialized without providers")
	}
}

func TestNewServiceWithProviders(t *testing.T) {
	cfg := &Config{
		TTS: TTSConfig{Provider: "sherpa-onnx"},
		ASR: ASRConfig{Provider: "sherpa"},
	}

	svc := NewServiceWithProviders(cfg, nil, nil, nil, nil)
	if svc == nil {
		t.Fatal("NewServiceWithProviders returned nil")
	}

	if svc.IsInitialized() {
		t.Error("Service should not be initialized with nil providers")
	}
}

func TestGetStatus(t *testing.T) {
	cfg := &Config{
		TTS: TTSConfig{Provider: "edge-tts"},
		ASR: ASRConfig{Provider: "sherpa", EditBeforeSend: true},
	}

	svc := NewService(cfg, nil, nil)
	status := svc.GetStatus()

	if status == nil {
		t.Fatal("GetStatus returned nil")
	}

	if status.TTS.Provider != "edge-tts" {
		t.Errorf("Expected TTS provider 'edge-tts', got '%s'", status.TTS.Provider)
	}

	if status.ASR.Provider != "sherpa" {
		t.Errorf("Expected ASR provider 'sherpa', got '%s'", status.ASR.Provider)
	}

	if !status.ASR.EditBeforeSend {
		t.Error("EditBeforeSend should be true")
	}
}

func TestIsEditBeforeSendEnabled(t *testing.T) {
	cfg := &Config{
		ASR: ASRConfig{EditBeforeSend: true},
	}

	svc := NewService(cfg, nil, nil)
	if !svc.IsEditBeforeSendEnabled() {
		t.Error("IsEditBeforeSendEnabled should return true")
	}

	cfg.ASR.EditBeforeSend = false
	svc = NewService(cfg, nil, nil)
	if svc.IsEditBeforeSendEnabled() {
		t.Error("IsEditBeforeSendEnabled should return false")
	}
}

func TestGetModels(t *testing.T) {
	cfg := &Config{
		TTS: TTSConfig{Provider: "sherpa-onnx"},
		ASR: ASRConfig{Provider: "sherpa"},
	}

	svc := NewService(cfg, nil, nil)
	models := svc.GetModels()

	if models == nil {
		t.Fatal("GetModels returned nil")
	}

	if models.TTS == nil || models.ASR == nil {
		t.Error("GetModels should return non-nil model lists")
	}
}

func TestGetProviders(t *testing.T) {
	cfg := &Config{
		TTS: TTSConfig{Provider: "edge-tts"},
		ASR: ASRConfig{Provider: "sherpa"},
	}

	svc := NewService(cfg, nil, nil)

	asrProvider := svc.GetASRProvider()
	// ASR provider should be nil without initialization
	if asrProvider != nil {
		t.Error("ASR provider should be nil without initialization")
	}

	ttsProvider := svc.GetTTSProvider()
	// TTS provider should be nil without initialization
	if ttsProvider != nil {
		t.Error("TTS provider should be nil without initialization")
	}
}
