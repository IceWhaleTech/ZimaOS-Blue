package stt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

// WhisperLocalProvider implements the Provider interface for local Whisper via Ollama.
type WhisperLocalProvider struct {
	baseURL     string
	model       string
	httpClient  *http.Client
	maxDuration time.Duration
}

// WhisperLocalConfig holds the configuration for the local Whisper provider.
type WhisperLocalConfig struct {
	BaseURL     string
	Model       string
	MaxDuration time.Duration
}

// NewWhisperLocalProvider creates a new local Whisper provider.
func NewWhisperLocalProvider(cfg *WhisperLocalConfig) *WhisperLocalProvider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	model := cfg.Model
	if model == "" {
		model = "whisper"
	}
	maxDuration := cfg.MaxDuration
	if maxDuration == 0 {
		maxDuration = 10 * time.Minute
	}

	return &WhisperLocalProvider{
		baseURL:     baseURL,
		model:       model,
		maxDuration: maxDuration,
		httpClient: &http.Client{
			Timeout: 10 * time.Minute,
		},
	}
}

// Name returns the provider name.
func (p *WhisperLocalProvider) Name() string {
	return "Local Whisper (Ollama)"
}

// Type returns the provider type.
func (p *WhisperLocalProvider) Type() ProviderType {
	return ProviderWhisperLocal
}

// SupportedFormats returns the supported audio formats.
func (p *WhisperLocalProvider) SupportedFormats() []AudioFormat {
	return []AudioFormat{FormatWAV, FormatMP3, FormatOGG, FormatFLAC}
}

// MaxDuration returns the maximum audio duration.
func (p *WhisperLocalProvider) MaxDuration() time.Duration {
	return p.maxDuration
}

// ollamaTranscribeRequest represents the Ollama transcription request.
type ollamaTranscribeRequest struct {
	Model    string `json:"model"`
	Audio    string `json:"audio"` // Base64 encoded audio
	Language string `json:"language,omitempty"`
}

// ollamaTranscribeResponse represents the Ollama transcription response.
type ollamaTranscribeResponse struct {
	Text     string  `json:"text"`
	Language string  `json:"language,omitempty"`
	Duration float64 `json:"duration,omitempty"`
}

// Transcribe transcribes audio to text using local Whisper.
func (p *WhisperLocalProvider) Transcribe(ctx context.Context, req *TranscribeRequest) (*TranscribeResponse, error) {
	// Read audio data
	audioData, err := io.ReadAll(req.Audio)
	if err != nil {
		return nil, fmt.Errorf("failed to read audio: %w", err)
	}

	// Create multipart form for Ollama's audio endpoint
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add file
	filename := fmt.Sprintf("audio.%s", req.Format)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := part.Write(audioData); err != nil {
		return nil, fmt.Errorf("failed to write audio data: %w", err)
	}

	// Add model
	if err := writer.WriteField("model", p.model); err != nil {
		return nil, fmt.Errorf("failed to write model field: %w", err)
	}

	// Add language if specified
	if req.Language != "" {
		if err := writer.WriteField("language", req.Language); err != nil {
			return nil, fmt.Errorf("failed to write language field: %w", err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close writer: %w", err)
	}

	// Create request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/transcribe", &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	// Send request
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var ollamaResp ollamaTranscribeResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &TranscribeResponse{
		Text:     ollamaResp.Text,
		Language: ollamaResp.Language,
		Duration: ollamaResp.Duration,
	}, nil
}

// TranscribeStream transcribes audio with streaming results.
func (p *WhisperLocalProvider) TranscribeStream(ctx context.Context, req *TranscribeRequest, callback StreamCallback) error {
	result, err := p.Transcribe(ctx, req)
	if err != nil {
		return err
	}
	return callback(result)
}
