package speech

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/stt"
	"github.com/labstack/echo/v4"
)

type testASRProvider struct{}

func (p *testASRProvider) Name() string { return "test-asr" }

func (p *testASRProvider) Type() stt.ProviderType { return "test-asr" }

func (p *testASRProvider) Transcribe(_ context.Context, _ *stt.TranscribeRequest) (*stt.TranscribeResponse, error) {
	return &stt.TranscribeResponse{}, nil
}

func (p *testASRProvider) TranscribeStream(_ context.Context, _ *stt.TranscribeRequest, _ stt.StreamCallback) error {
	return nil
}

func (p *testASRProvider) SupportedFormats() []stt.AudioFormat { return nil }

func (p *testASRProvider) MaxDuration() time.Duration { return time.Second }

func TestMacOSOfflineDictationLanguages(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS-only Objective-C path")
	}

	provider := NewMacOSNativeSTT()
	langs := provider.OfflineDictationLanguages()
	if langs == nil {
		langs = []string{}
	}

	if _, err := json.Marshal(map[string][]string{"offline_languages": langs}); err != nil {
		t.Fatalf("marshal offline languages: %v", err)
	}
}

func TestGetOfflineLanguagesHandlerWithMacOSProvider(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS-only Objective-C path")
	}

	svc := NewServiceWithProviders(&Config{
		ASR: ASRConfig{Provider: "macos-native"},
	}, nil, nil, NewMacOSNativeSTT(), nil)

	h := &Handler{service: svc}
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/speech/asr/offline-languages", nil)
	rec := httptest.NewRecorder()

	if err := h.GetOfflineLanguages(e.NewContext(req, rec)); err != nil {
		t.Fatalf("GetOfflineLanguages() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload struct {
		OfflineLanguages []string `json:"offline_languages"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal body: %v; body=%s", err, rec.Body.String())
	}
	if payload.OfflineLanguages == nil {
		t.Fatalf("offline_languages = nil; body=%s", rec.Body.String())
	}
}

func TestServiceProviderAccessConcurrent(t *testing.T) {
	cfg := &Config{
		TTS: TTSConfig{Provider: "edge-tts"},
		ASR: ASRConfig{Provider: "test-asr", EditBeforeSend: true},
	}

	rawSvc := NewService(cfg, nil, nil)
	svc, ok := rawSvc.(*service)
	if !ok {
		t.Fatalf("service type = %T, want *service", rawSvc)
	}

	provider := &testASRProvider{}
	const iterations = 2000

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				svc.SetASRProvider(provider)
				svc.SetASRProvider(nil)
			}
		}()
	}

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = svc.GetASRProvider()
				_ = svc.GetStatus()
			}
		}()
	}

	wg.Wait()
}
