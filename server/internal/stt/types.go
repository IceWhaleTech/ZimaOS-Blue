// Package stt provides speech-to-text functionality.
// This package implements STT provider interfaces for transcribing audio to text.
package stt

import (
	"context"
	"errors"
	"io"
	"time"
)

var (
	// ErrProviderNotFound is returned when a provider is not found.
	ErrProviderNotFound = errors.New("STT provider not found")
	// ErrProviderDisabled is returned when a provider is disabled.
	ErrProviderDisabled = errors.New("STT provider disabled")
	// ErrInvalidAudio is returned when the audio format is invalid.
	ErrInvalidAudio = errors.New("invalid audio format")
	// ErrTranscriptionFailed is returned when transcription fails.
	ErrTranscriptionFailed = errors.New("transcription failed")
	// ErrAudioTooLong is returned when the audio is too long.
	ErrAudioTooLong = errors.New("audio too long")
	// ErrAudioTooShort is returned when the audio is too short.
	ErrAudioTooShort = errors.New("audio too short")
)

// ProviderType represents the type of STT provider.
type ProviderType string

const (
	// ProviderWhisper is local whisper.cpp ASR.
	ProviderWhisper ProviderType = "whisper"
)

// AudioFormat represents the audio format.
type AudioFormat string

const (
	// FormatWAV is WAV audio format.
	FormatWAV AudioFormat = "wav"
	// FormatMP3 is MP3 audio format.
	FormatMP3 AudioFormat = "mp3"
	// FormatOGG is OGG audio format.
	FormatOGG AudioFormat = "ogg"
	// FormatWebM is WebM audio format.
	FormatWebM AudioFormat = "webm"
	// FormatFLAC is FLAC audio format.
	FormatFLAC AudioFormat = "flac"
	// FormatPCM is raw PCM audio format.
	FormatPCM AudioFormat = "pcm"
)

// TranscribeRequest represents a transcription request.
type TranscribeRequest struct {
	// Audio is the audio data to transcribe.
	Audio io.Reader
	// Format is the audio format.
	Format AudioFormat
	// Language is the language code (e.g., "en", "zh").
	Language string
	// Prompt is an optional prompt to guide transcription.
	Prompt string
	// Temperature controls randomness (0.0-1.0).
	Temperature float32
}

// TranscribeResponse represents a transcription response.
type TranscribeResponse struct {
	// Text is the transcribed text.
	Text string `json:"text"`
	// Language is the detected language.
	Language string `json:"language,omitempty"`
	// Duration is the audio duration in seconds.
	Duration float64 `json:"duration,omitempty"`
	// Segments contains word-level timing information.
	Segments []Segment `json:"segments,omitempty"`
	// Confidence is the overall confidence score (0.0-1.0).
	Confidence float64 `json:"confidence,omitempty"`
}

// Segment represents a transcription segment with timing.
type Segment struct {
	// ID is the segment ID.
	ID int `json:"id"`
	// Start is the start time in seconds.
	Start float64 `json:"start"`
	// End is the end time in seconds.
	End float64 `json:"end"`
	// Text is the segment text.
	Text string `json:"text"`
	// Confidence is the segment confidence score.
	Confidence float64 `json:"confidence,omitempty"`
}

// StreamCallback is called when streaming transcription results are available.
type StreamCallback func(partial *TranscribeResponse) error

// Provider defines the STT provider interface.
type Provider interface {
	// Name returns the provider name.
	Name() string
	// Type returns the provider type.
	Type() ProviderType
	// Transcribe transcribes audio to text.
	Transcribe(ctx context.Context, req *TranscribeRequest) (*TranscribeResponse, error)
	// TranscribeStream transcribes audio with streaming results.
	TranscribeStream(ctx context.Context, req *TranscribeRequest, callback StreamCallback) error
	// SupportedFormats returns the supported audio formats.
	SupportedFormats() []AudioFormat
	// MaxDuration returns the maximum audio duration in seconds.
	MaxDuration() time.Duration
}

// ProviderConfig holds the configuration for an STT provider.
type ProviderConfig struct {
	// Type is the provider type.
	Type ProviderType `json:"type" yaml:"type"`
	// Enabled indicates if this provider is enabled.
	Enabled bool `json:"enabled" yaml:"enabled"`
	// APIKey is the API key for the provider.
	APIKey string `json:"api_key,omitempty" yaml:"api_key,omitempty"`
	// BaseURL is the base URL for the API.
	BaseURL string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	// Model is the model to use.
	Model string `json:"model,omitempty" yaml:"model,omitempty"`
	// ModelDir is the directory for local models (Sherpa).
	ModelDir string `json:"model_dir,omitempty" yaml:"model_dir,omitempty"`
	// DefaultLanguage is the default language.
	DefaultLanguage string `json:"default_language,omitempty" yaml:"default_language,omitempty"`
	// MaxDuration is the maximum audio duration.
	MaxDuration time.Duration `json:"max_duration,omitempty" yaml:"max_duration,omitempty"`
}

// Service defines the STT service interface.
type Service interface {
	// Transcribe transcribes audio using the default provider.
	Transcribe(ctx context.Context, req *TranscribeRequest) (*TranscribeResponse, error)
	// TranscribeWithProvider transcribes audio using a specific provider.
	TranscribeWithProvider(ctx context.Context, providerType ProviderType, req *TranscribeRequest) (*TranscribeResponse, error)
	// TranscribeStream transcribes audio with streaming results.
	TranscribeStream(ctx context.Context, req *TranscribeRequest, callback StreamCallback) error
	// ListProviders returns all available providers.
	ListProviders() []ProviderType
	// GetDefaultProvider returns the default provider type.
	GetDefaultProvider() ProviderType
	// GetWhisperProvider returns the Whisper ASR provider if available.
	GetWhisperProvider() *WhisperProvider
	// PeekWhisperProvider returns the current Whisper ASR provider without forcing a full warm init.
	PeekWhisperProvider() *WhisperProvider
	// Close releases any held resources.
	Close() error
}
