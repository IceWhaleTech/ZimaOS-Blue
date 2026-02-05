package tts

import (
	"bytes"
	"context"
	"io"
	"testing"
)

// mockProvider is a mock implementation of the Provider interface for testing.
type mockProvider struct {
	name           string
	providerType   ProviderType
	synthesizeFunc func(ctx context.Context, req *SynthesizeRequest) (*SynthesizeResponse, error)
	streamFunc     func(ctx context.Context, req *SynthesizeRequest, callback StreamCallback) error
	voices         []Voice
	formats        []AudioFormat
	maxTextLength  int
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) Type() ProviderType {
	return m.providerType
}

func (m *mockProvider) Synthesize(ctx context.Context, req *SynthesizeRequest) (*SynthesizeResponse, error) {
	if m.synthesizeFunc != nil {
		return m.synthesizeFunc(ctx, req)
	}
	return &SynthesizeResponse{
		Audio:       io.NopCloser(bytes.NewReader([]byte("fake audio data"))),
		Format:      FormatMP3,
		ContentType: "audio/mpeg",
		Duration:    1.5,
	}, nil
}

func (m *mockProvider) SynthesizeStream(ctx context.Context, req *SynthesizeRequest, callback StreamCallback) error {
	if m.streamFunc != nil {
		return m.streamFunc(ctx, req, callback)
	}
	return callback([]byte("fake audio chunk"))
}

func (m *mockProvider) ListVoices(ctx context.Context) ([]Voice, error) {
	if m.voices != nil {
		return m.voices, nil
	}
	return []Voice{
		{ID: "voice1", Name: "Voice 1", Language: "en-US", Gender: "female"},
		{ID: "voice2", Name: "Voice 2", Language: "en-US", Gender: "male"},
	}, nil
}

func (m *mockProvider) SupportedFormats() []AudioFormat {
	if m.formats != nil {
		return m.formats
	}
	return []AudioFormat{FormatMP3, FormatWAV}
}

func (m *mockProvider) MaxTextLength() int {
	if m.maxTextLength > 0 {
		return m.maxTextLength
	}
	return 4096
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
					Type:    ProviderOpenAI,
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

func TestService_Synthesize(t *testing.T) {
	svc := &service{
		providers: map[ProviderType]Provider{
			"mock": &mockProvider{
				name:         "Mock Provider",
				providerType: "mock",
			},
		},
		defaultProvider: "mock",
	}

	t.Run("successful synthesis", func(t *testing.T) {
		req := &SynthesizeRequest{
			Text:   "Hello, world!",
			Voice:  "voice1",
			Format: FormatMP3,
		}

		resp, err := svc.Synthesize(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Format != FormatMP3 {
			t.Errorf("expected format MP3, got %s", resp.Format)
		}
		if resp.Audio == nil {
			t.Error("expected non-nil audio")
		}
		resp.Audio.Close()
	})

	t.Run("provider not found", func(t *testing.T) {
		emptySvc := &service{
			providers:       map[ProviderType]Provider{},
			defaultProvider: "nonexistent",
		}

		req := &SynthesizeRequest{
			Text: "Hello",
		}

		_, err := emptySvc.Synthesize(context.Background(), req)
		if err != ErrProviderNotFound {
			t.Errorf("expected ErrProviderNotFound, got %v", err)
		}
	})
}

func TestService_SynthesizeWithProvider(t *testing.T) {
	svc := &service{
		providers: map[ProviderType]Provider{
			"mock1": &mockProvider{
				name:         "Mock Provider 1",
				providerType: "mock1",
				synthesizeFunc: func(ctx context.Context, req *SynthesizeRequest) (*SynthesizeResponse, error) {
					return &SynthesizeResponse{
						Audio:       io.NopCloser(bytes.NewReader([]byte("from mock1"))),
						Format:      FormatMP3,
						ContentType: "audio/mpeg",
					}, nil
				},
			},
			"mock2": &mockProvider{
				name:         "Mock Provider 2",
				providerType: "mock2",
				synthesizeFunc: func(ctx context.Context, req *SynthesizeRequest) (*SynthesizeResponse, error) {
					return &SynthesizeResponse{
						Audio:       io.NopCloser(bytes.NewReader([]byte("from mock2"))),
						Format:      FormatWAV,
						ContentType: "audio/wav",
					}, nil
				},
			},
		},
		defaultProvider: "mock1",
	}

	t.Run("use specific provider", func(t *testing.T) {
		req := &SynthesizeRequest{
			Text: "Hello",
		}

		resp, err := svc.SynthesizeWithProvider(context.Background(), "mock2", req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Format != FormatWAV {
			t.Errorf("expected format WAV, got %s", resp.Format)
		}
		resp.Audio.Close()
	})

	t.Run("provider not found", func(t *testing.T) {
		req := &SynthesizeRequest{
			Text: "Hello",
		}

		_, err := svc.SynthesizeWithProvider(context.Background(), "nonexistent", req)
		if err != ErrProviderNotFound {
			t.Errorf("expected ErrProviderNotFound, got %v", err)
		}
	})
}

func TestService_SynthesizeStream(t *testing.T) {
	svc := &service{
		providers: map[ProviderType]Provider{
			"mock": &mockProvider{
				name:         "Mock Provider",
				providerType: "mock",
				streamFunc: func(ctx context.Context, req *SynthesizeRequest, callback StreamCallback) error {
					callback([]byte("chunk1"))
					callback([]byte("chunk2"))
					callback([]byte("chunk3"))
					return nil
				},
			},
		},
		defaultProvider: "mock",
	}

	t.Run("streaming synthesis", func(t *testing.T) {
		req := &SynthesizeRequest{
			Text: "Hello, world!",
		}

		var chunks [][]byte
		err := svc.SynthesizeStream(context.Background(), req, func(chunk []byte) error {
			chunks = append(chunks, chunk)
			return nil
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(chunks) != 3 {
			t.Errorf("expected 3 chunks, got %d", len(chunks))
		}
	})
}

func TestService_ListVoices(t *testing.T) {
	svc := &service{
		providers: map[ProviderType]Provider{
			"mock": &mockProvider{
				name:         "Mock Provider",
				providerType: "mock",
				voices: []Voice{
					{ID: "v1", Name: "Voice 1", Language: "en-US"},
					{ID: "v2", Name: "Voice 2", Language: "zh-CN"},
				},
			},
		},
		defaultProvider: "mock",
	}

	t.Run("list voices", func(t *testing.T) {
		voices, err := svc.ListVoices(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(voices) != 2 {
			t.Errorf("expected 2 voices, got %d", len(voices))
		}
	})

	t.Run("provider not found", func(t *testing.T) {
		emptySvc := &service{
			providers:       map[ProviderType]Provider{},
			defaultProvider: "nonexistent",
		}

		_, err := emptySvc.ListVoices(context.Background())
		if err != ErrProviderNotFound {
			t.Errorf("expected ErrProviderNotFound, got %v", err)
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
		defaultProvider: "openai",
	}

	defaultProvider := svc.GetDefaultProvider()
	if defaultProvider != "openai" {
		t.Errorf("expected 'openai', got '%s'", defaultProvider)
	}
}

func TestVoice(t *testing.T) {
	voice := Voice{
		ID:          "alloy",
		Name:        "Alloy",
		Language:    "en-US",
		Gender:      "neutral",
		Description: "A neutral voice",
		PreviewURL:  "https://example.com/preview.mp3",
	}

	if voice.ID != "alloy" {
		t.Errorf("expected ID 'alloy', got '%s'", voice.ID)
	}
	if voice.Language != "en-US" {
		t.Errorf("expected language 'en-US', got '%s'", voice.Language)
	}
}

func TestSynthesizeRequest(t *testing.T) {
	req := &SynthesizeRequest{
		Text:   "Hello, world!",
		Voice:  "alloy",
		Format: FormatMP3,
		Speed:  1.0,
		Pitch:  0.0,
	}

	if req.Text != "Hello, world!" {
		t.Errorf("expected text 'Hello, world!', got '%s'", req.Text)
	}
	if req.Format != FormatMP3 {
		t.Errorf("expected format MP3, got %s", req.Format)
	}
}

func TestAudioFormats(t *testing.T) {
	formats := []AudioFormat{FormatMP3, FormatOPUS, FormatAAC, FormatFLAC, FormatWAV, FormatPCM}

	for _, format := range formats {
		if format == "" {
			t.Error("format should not be empty")
		}
	}
}

func TestProviderTypes(t *testing.T) {
	types := []ProviderType{ProviderOpenAI, ProviderElevenLabs, ProviderPiper, ProviderEspeakNG}

	for _, pt := range types {
		if pt == "" {
			t.Error("provider type should not be empty")
		}
	}
}
