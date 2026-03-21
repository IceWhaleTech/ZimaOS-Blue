package speech

import (
	"context"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/stt"
)

type fakeSpeechSTTService struct {
	provider  *stt.WhisperProvider
	getCalls  int
	peekCalls int
}

func (f *fakeSpeechSTTService) Transcribe(context.Context, *stt.TranscribeRequest) (*stt.TranscribeResponse, error) {
	return nil, nil
}

func (f *fakeSpeechSTTService) TranscribeWithProvider(context.Context, stt.ProviderType, *stt.TranscribeRequest) (*stt.TranscribeResponse, error) {
	return nil, nil
}

func (f *fakeSpeechSTTService) TranscribeStream(context.Context, *stt.TranscribeRequest, stt.StreamCallback) error {
	return nil
}

func (f *fakeSpeechSTTService) ListProviders() []stt.ProviderType {
	return []stt.ProviderType{stt.ProviderWhisper}
}
func (f *fakeSpeechSTTService) GetDefaultProvider() stt.ProviderType {
	return stt.ProviderWhisper
}
func (f *fakeSpeechSTTService) GetWhisperProvider() *stt.WhisperProvider {
	f.getCalls++
	return f.provider
}
func (f *fakeSpeechSTTService) PeekWhisperProvider() *stt.WhisperProvider {
	f.peekCalls++
	return f.provider
}
func (f *fakeSpeechSTTService) Close() error { return nil }

func TestGetStatusDoesNotPrewarmSTTWhenDisabled(t *testing.T) {
	fakeSTT := &fakeSpeechSTTService{provider: &stt.WhisperProvider{}}
	svc := NewService(&Config{
		TTS: TTSConfig{Provider: "edge-tts"},
		ASR: ASRConfig{Provider: "whisper", EditBeforeSend: true},
	}, fakeSTT, nil)
	svc.SetStatusPrewarmEnabled(false)

	status := svc.GetStatus()
	if status == nil {
		t.Fatal("GetStatus() returned nil")
	}
	if fakeSTT.getCalls != 0 {
		t.Fatalf("GetWhisperProvider() calls = %d, want 0", fakeSTT.getCalls)
	}
	if fakeSTT.peekCalls == 0 {
		t.Fatal("expected GetStatus() to consult PeekWhisperProvider()")
	}
}
