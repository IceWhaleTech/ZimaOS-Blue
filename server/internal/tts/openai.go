package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// OpenAIProvider implements the Provider interface for OpenAI's TTS API.
type OpenAIProvider struct {
	apiKey        string
	baseURL       string
	defaultVoice  string
	defaultFormat AudioFormat
	httpClient    *http.Client
	maxTextLength int
}

// OpenAIConfig holds the configuration for the OpenAI TTS provider.
type OpenAIConfig struct {
	APIKey        string
	BaseURL       string
	DefaultVoice  string
	DefaultFormat AudioFormat
	MaxTextLength int
}

// NewOpenAIProvider creates a new OpenAI TTS provider.
func NewOpenAIProvider(cfg *OpenAIConfig) *OpenAIProvider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	defaultVoice := cfg.DefaultVoice
	if defaultVoice == "" {
		defaultVoice = "alloy"
	}
	defaultFormat := cfg.DefaultFormat
	if defaultFormat == "" {
		defaultFormat = FormatMP3
	}
	maxTextLength := cfg.MaxTextLength
	if maxTextLength == 0 {
		maxTextLength = 4096
	}

	return &OpenAIProvider{
		apiKey:        cfg.APIKey,
		baseURL:       baseURL,
		defaultVoice:  defaultVoice,
		defaultFormat: defaultFormat,
		maxTextLength: maxTextLength,
		httpClient:    &http.Client{},
	}
}

// Name returns the provider name.
func (p *OpenAIProvider) Name() string {
	return "OpenAI TTS"
}

// Type returns the provider type.
func (p *OpenAIProvider) Type() ProviderType {
	return ProviderOpenAI
}

// SupportedFormats returns the supported audio formats.
func (p *OpenAIProvider) SupportedFormats() []AudioFormat {
	return []AudioFormat{FormatMP3, FormatOPUS, FormatAAC, FormatFLAC, FormatWAV, FormatPCM}
}

// MaxTextLength returns the maximum text length.
func (p *OpenAIProvider) MaxTextLength() int {
	return p.maxTextLength
}

// openAITTSRequest represents the OpenAI TTS request.
type openAITTSRequest struct {
	Model          string  `json:"model"`
	Input          string  `json:"input"`
	Voice          string  `json:"voice"`
	ResponseFormat string  `json:"response_format,omitempty"`
	Speed          float32 `json:"speed,omitempty"`
}

// Synthesize synthesizes text to speech.
func (p *OpenAIProvider) Synthesize(ctx context.Context, req *SynthesizeRequest) (*SynthesizeResponse, error) {
	if len(req.Text) > p.maxTextLength {
		return nil, ErrTextTooLong
	}

	voice := req.Voice
	if voice == "" {
		voice = p.defaultVoice
	}

	format := req.Format
	if format == "" {
		format = p.defaultFormat
	}

	speed := req.Speed
	if speed == 0 {
		speed = 1.0
	}

	// Create request body
	ttsReq := openAITTSRequest{
		Model:          "tts-1",
		Input:          req.Text,
		Voice:          voice,
		ResponseFormat: string(format),
		Speed:          speed,
	}

	body, err := json.Marshal(ttsReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/audio/speech", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Check status
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Determine content type
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = getContentType(format)
	}

	return &SynthesizeResponse{
		Audio:       resp.Body,
		Format:      format,
		ContentType: contentType,
	}, nil
}

// SynthesizeStream synthesizes text with streaming audio output.
func (p *OpenAIProvider) SynthesizeStream(ctx context.Context, req *SynthesizeRequest, callback StreamCallback) error {
	resp, err := p.Synthesize(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Audio.Close()

	// Stream chunks
	buf := make([]byte, 4096)
	for {
		n, err := resp.Audio.Read(buf)
		if n > 0 {
			if err := callback(buf[:n]); err != nil {
				return err
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read audio: %w", err)
		}
	}

	return nil
}

// ListVoices returns available voices.
func (p *OpenAIProvider) ListVoices(ctx context.Context) ([]Voice, error) {
	// OpenAI has fixed voices
	return []Voice{
		{ID: "alloy", Name: "Alloy", Language: "en", Gender: "neutral", Description: "Neutral and balanced"},
		{ID: "echo", Name: "Echo", Language: "en", Gender: "male", Description: "Warm and engaging"},
		{ID: "fable", Name: "Fable", Language: "en", Gender: "neutral", Description: "Expressive and dramatic"},
		{ID: "onyx", Name: "Onyx", Language: "en", Gender: "male", Description: "Deep and authoritative"},
		{ID: "nova", Name: "Nova", Language: "en", Gender: "female", Description: "Friendly and upbeat"},
		{ID: "shimmer", Name: "Shimmer", Language: "en", Gender: "female", Description: "Clear and pleasant"},
	}, nil
}

// getContentType returns the MIME content type for an audio format.
func getContentType(format AudioFormat) string {
	switch format {
	case FormatMP3:
		return "audio/mpeg"
	case FormatOPUS:
		return "audio/opus"
	case FormatAAC:
		return "audio/aac"
	case FormatFLAC:
		return "audio/flac"
	case FormatWAV:
		return "audio/wav"
	case FormatPCM:
		return "audio/pcm"
	default:
		return "audio/mpeg"
	}
}
