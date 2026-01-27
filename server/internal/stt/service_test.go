package stt

import (
	"bytes"
	"context"
	"testing"
	"time"
)

// mockProvider is a mock implementation of the Provider interface for testing.
type mockProvider struct {
	name            string
	providerType    ProviderType
	transcribeFunc  func(ctx context.Context, req *TranscribeRequest) (*TranscribeResponse, error)
	streamFunc      func(ctx context.Context, req *TranscribeRequest, callback StreamCallback) error
	formats         []AudioFormat
	maxDuration     time.Duration
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) Type() ProviderType {
	return m.providerType
}

func (m *mockProvider) Transcribe(ctx context.Context, req *TranscribeRequest) (*TranscribeResponse, error) {
	if m.transcribeFunc != nil {
		return m.transcribeFunc(ctx, req)
	}
	return &TranscribeResponse{
		Text:     "Hello, world!",
		Language: "en",
		Duration: 1.5,
	}, nil
}

func (m *mockProvider) TranscribeStream(ctx context.Context, req *TranscribeRequest, callback StreamCallback) error {
	if m.streamFunc != nil {
		return m.streamFunc(ctx, req, callback)
	}
	return callback(&TranscribeResponse{
		Text:     "Hello, world!",
		Language: "en",
	})
}

func (m *mockProvider) SupportedFormats() []AudioFormat {
	if m.formats != nil {
		return m.formats
	}
	return []AudioFormat{FormatWAV, FormatMP3}
}

func (m *mockProvider) MaxDuration() time.Duration {
	if m.maxDuration > 0 {
		return m.maxDuration
	}
	return 30 * time.Second
}

func TestNewService(t *testing.T) {
	t.Run("create service with no providers", func(t *testing.T) {
		cfg := &ServiceConfig{
			Providers: []ProviderConfig{},
		}
		svc, err := NewService(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if svc == nil {
			t.Fatal("expected non-nil service")
		}
	})

	t.Run("create service with disabled provider", func(t *testing.T) {
		cfg := &ServiceConfig{
			Providers: []ProviderConfig{
				{
					Type:    ProviderWhisperAPI,
					Enabled: false,
				},
			},
		}
		svc, err := NewService(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		providers := svc.ListProviders()
		if len(providers) != 0 {
			t.Errorf("expected 0 providers, got %d", len(providers))
		}
	})
}

func TestService_Transcribe(t *testing.T) {
	// Create a service with a mock provider
	svc := &service{
		providers: map[ProviderType]Provider{
			"mock": &mockProvider{
				name:         "Mock Provider",
				providerType: "mock",
			},
		},
		defaultProvider: "mock",
	}

	t.Run("successful transcription", func(t *testing.T) {
		req := &TranscribeRequest{
			Audio:    bytes.NewReader([]byte("fake audio data")),
			Format:   FormatWAV,
			Language: "en",
		}

		resp, err := svc.Transcribe(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Text != "Hello, world!" {
			t.Errorf("expected text 'Hello, world!', got '%s'", resp.Text)
		}
	})

	t.Run("provider not found", func(t *testing.T) {
		emptySvc := &service{
			providers:       map[ProviderType]Provider{},
			defaultProvider: "nonexistent",
		}

		req := &TranscribeRequest{
			Audio:  bytes.NewReader([]byte("fake audio data")),
			Format: FormatWAV,
		}

		_, err := emptySvc.Transcribe(context.Background(), req)
		if err != ErrProviderNotFound {
			t.Errorf("expected ErrProviderNotFound, got %v", err)
		}
	})
}

func TestService_TranscribeWithProvider(t *testing.T) {
	svc := &service{
		providers: map[ProviderType]Provider{
			"mock1": &mockProvider{
				name:         "Mock Provider 1",
				providerType: "mock1",
				transcribeFunc: func(ctx context.Context, req *TranscribeRequest) (*TranscribeResponse, error) {
					return &TranscribeResponse{Text: "From mock1"}, nil
				},
			},
			"mock2": &mockProvider{
				name:         "Mock Provider 2",
				providerType: "mock2",
				transcribeFunc: func(ctx context.Context, req *TranscribeRequest) (*TranscribeResponse, error) {
					return &TranscribeResponse{Text: "From mock2"}, nil
				},
			},
		},
		defaultProvider: "mock1",
	}

	t.Run("use specific provider", func(t *testing.T) {
		req := &TranscribeRequest{
			Audio:  bytes.NewReader([]byte("fake audio data")),
			Format: FormatWAV,
		}

		resp, err := svc.TranscribeWithProvider(context.Background(), "mock2", req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Text != "From mock2" {
			t.Errorf("expected text 'From mock2', got '%s'", resp.Text)
		}
	})

	t.Run("provider not found", func(t *testing.T) {
		req := &TranscribeRequest{
			Audio:  bytes.NewReader([]byte("fake audio data")),
			Format: FormatWAV,
		}

		_, err := svc.TranscribeWithProvider(context.Background(), "nonexistent", req)
		if err != ErrProviderNotFound {
			t.Errorf("expected ErrProviderNotFound, got %v", err)
		}
	})
}

func TestService_TranscribeStream(t *testing.T) {
	svc := &service{
		providers: map[ProviderType]Provider{
			"mock": &mockProvider{
				name:         "Mock Provider",
				providerType: "mock",
				streamFunc: func(ctx context.Context, req *TranscribeRequest, callback StreamCallback) error {
					// Simulate streaming with multiple callbacks
					callback(&TranscribeResponse{Text: "Hello"})
					callback(&TranscribeResponse{Text: "Hello, world"})
					callback(&TranscribeResponse{Text: "Hello, world!"})
					return nil
				},
			},
		},
		defaultProvider: "mock",
	}

	t.Run("streaming transcription", func(t *testing.T) {
		req := &TranscribeRequest{
			Audio:  bytes.NewReader([]byte("fake audio data")),
			Format: FormatWAV,
		}

		var results []string
		err := svc.TranscribeStream(context.Background(), req, func(partial *TranscribeResponse) error {
			results = append(results, partial.Text)
			return nil
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 3 {
			t.Errorf("expected 3 results, got %d", len(results))
		}
	})
}

func TestService_ListProviders(t *testing.T) {
	svc := &service{
		providers: map[ProviderType]Provider{
			"mock1": &mockProvider{providerType: "mock1"},
			"mock2": &mockProvider{providerType: "mock2"},
		},
		defaultProvider: "mock1",
	}

	providers := svc.ListProviders()
	if len(providers) != 2 {
		t.Errorf("expected 2 providers, got %d", len(providers))
	}
}

func TestService_GetDefaultProvider(t *testing.T) {
	svc := &service{
		providers:       map[ProviderType]Provider{},
		defaultProvider: "whisper_api",
	}

	defaultProvider := svc.GetDefaultProvider()
	if defaultProvider != "whisper_api" {
		t.Errorf("expected 'whisper_api', got '%s'", defaultProvider)
	}
}

func TestTranscribeRequest(t *testing.T) {
	t.Run("create request", func(t *testing.T) {
		req := &TranscribeRequest{
			Audio:       bytes.NewReader([]byte("audio data")),
			Format:      FormatWAV,
			Language:    "en",
			Prompt:      "transcribe this",
			Temperature: 0.5,
		}

		if req.Format != FormatWAV {
			t.Errorf("expected format WAV, got %s", req.Format)
		}
		if req.Language != "en" {
			t.Errorf("expected language 'en', got '%s'", req.Language)
		}
	})
}

func TestTranscribeResponse(t *testing.T) {
	t.Run("response with segments", func(t *testing.T) {
		resp := &TranscribeResponse{
			Text:     "Hello, world!",
			Language: "en",
			Duration: 2.5,
			Segments: []Segment{
				{ID: 0, Start: 0.0, End: 1.0, Text: "Hello,", Confidence: 0.95},
				{ID: 1, Start: 1.0, End: 2.5, Text: "world!", Confidence: 0.98},
			},
			Confidence: 0.96,
		}

		if len(resp.Segments) != 2 {
			t.Errorf("expected 2 segments, got %d", len(resp.Segments))
		}
		if resp.Segments[0].Text != "Hello," {
			t.Errorf("expected first segment 'Hello,', got '%s'", resp.Segments[0].Text)
		}
	})
}

func TestAudioFormats(t *testing.T) {
	formats := []AudioFormat{FormatWAV, FormatMP3, FormatOGG, FormatWebM, FormatFLAC, FormatPCM}

	for _, format := range formats {
		if format == "" {
			t.Error("format should not be empty")
		}
	}
}

func TestProviderTypes(t *testing.T) {
	types := []ProviderType{ProviderWhisperAPI, ProviderWhisperLocal, ProviderGoogleSTT, ProviderAzureSTT}

	for _, pt := range types {
		if pt == "" {
			t.Error("provider type should not be empty")
		}
	}
}
