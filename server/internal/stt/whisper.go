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

// WhisperAPIProvider implements the Provider interface for OpenAI's Whisper API.
type WhisperAPIProvider struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
	maxDuration time.Duration
}

// WhisperAPIConfig holds the configuration for the Whisper API provider.
type WhisperAPIConfig struct {
	APIKey      string
	BaseURL     string
	Model       string
	MaxDuration time.Duration
}

// NewWhisperAPIProvider creates a new Whisper API provider.
func NewWhisperAPIProvider(cfg *WhisperAPIConfig) *WhisperAPIProvider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	model := cfg.Model
	if model == "" {
		model = "whisper-1"
	}
	maxDuration := cfg.MaxDuration
	if maxDuration == 0 {
		maxDuration = 30 * time.Minute // Whisper API supports up to 25MB files
	}

	return &WhisperAPIProvider{
		apiKey:      cfg.APIKey,
		baseURL:     baseURL,
		model:       model,
		maxDuration: maxDuration,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

// Name returns the provider name.
func (p *WhisperAPIProvider) Name() string {
	return "OpenAI Whisper API"
}

// Type returns the provider type.
func (p *WhisperAPIProvider) Type() ProviderType {
	return ProviderWhisperAPI
}

// SupportedFormats returns the supported audio formats.
func (p *WhisperAPIProvider) SupportedFormats() []AudioFormat {
	return []AudioFormat{FormatMP3, FormatWAV, FormatWebM, FormatOGG, FormatFLAC}
}

// MaxDuration returns the maximum audio duration.
func (p *WhisperAPIProvider) MaxDuration() time.Duration {
	return p.maxDuration
}

// whisperResponse represents the Whisper API response.
type whisperResponse struct {
	Text     string `json:"text"`
	Language string `json:"language,omitempty"`
	Duration float64 `json:"duration,omitempty"`
	Segments []struct {
		ID         int     `json:"id"`
		Start      float64 `json:"start"`
		End        float64 `json:"end"`
		Text       string  `json:"text"`
		Confidence float64 `json:"confidence,omitempty"`
	} `json:"segments,omitempty"`
}

// Transcribe transcribes audio to text.
func (p *WhisperAPIProvider) Transcribe(ctx context.Context, req *TranscribeRequest) (*TranscribeResponse, error) {
	// Read audio data
	audioData, err := io.ReadAll(req.Audio)
	if err != nil {
		return nil, fmt.Errorf("failed to read audio: %w", err)
	}

	// Create multipart form
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

	// Add optional fields
	if req.Language != "" {
		if err := writer.WriteField("language", req.Language); err != nil {
			return nil, fmt.Errorf("failed to write language field: %w", err)
		}
	}
	if req.Prompt != "" {
		if err := writer.WriteField("prompt", req.Prompt); err != nil {
			return nil, fmt.Errorf("failed to write prompt field: %w", err)
		}
	}
	if req.Temperature > 0 {
		if err := writer.WriteField("temperature", fmt.Sprintf("%.2f", req.Temperature)); err != nil {
			return nil, fmt.Errorf("failed to write temperature field: %w", err)
		}
	}

	// Request verbose JSON for segments
	if err := writer.WriteField("response_format", "verbose_json"); err != nil {
		return nil, fmt.Errorf("failed to write response_format field: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close writer: %w", err)
	}

	// Create request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/audio/transcriptions", &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
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
	var whisperResp whisperResponse
	if err := json.NewDecoder(resp.Body).Decode(&whisperResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to TranscribeResponse
	result := &TranscribeResponse{
		Text:     whisperResp.Text,
		Language: whisperResp.Language,
		Duration: whisperResp.Duration,
	}

	for _, seg := range whisperResp.Segments {
		result.Segments = append(result.Segments, Segment{
			ID:         seg.ID,
			Start:      seg.Start,
			End:        seg.End,
			Text:       seg.Text,
			Confidence: seg.Confidence,
		})
	}

	return result, nil
}

// TranscribeStream transcribes audio with streaming results.
// Note: Whisper API doesn't support streaming, so this falls back to regular transcription.
func (p *WhisperAPIProvider) TranscribeStream(ctx context.Context, req *TranscribeRequest, callback StreamCallback) error {
	result, err := p.Transcribe(ctx, req)
	if err != nil {
		return err
	}
	return callback(result)
}
