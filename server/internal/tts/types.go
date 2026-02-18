// Package tts provides text-to-speech functionality.
// This package implements TTS provider interfaces for synthesizing speech from text.
package tts

import (
	"context"
	"errors"
	"io"
)

var (
	// ErrProviderNotFound is returned when a provider is not found.
	ErrProviderNotFound = errors.New("TTS provider not found")
	// ErrProviderDisabled is returned when a provider is disabled.
	ErrProviderDisabled = errors.New("TTS provider disabled")
	// ErrInvalidText is returned when the text is invalid.
	ErrInvalidText = errors.New("invalid text")
	// ErrSynthesisFailed is returned when synthesis fails.
	ErrSynthesisFailed = errors.New("synthesis failed")
	// ErrTextTooLong is returned when the text is too long.
	ErrTextTooLong = errors.New("text too long")
	// ErrVoiceNotFound is returned when the voice is not found.
	ErrVoiceNotFound = errors.New("voice not found")
	// ErrNoProviderConfigured is returned when no provider is configured.
	ErrNoProviderConfigured = errors.New("no TTS provider configured, please configure one in settings")
)

// ProviderType represents the type of TTS provider.
type ProviderType string

const (
	// ProviderOpenAI is OpenAI's TTS API.
	ProviderOpenAI ProviderType = "openai"
	// ProviderElevenLabs is ElevenLabs TTS.
	ProviderElevenLabs ProviderType = "elevenlabs"
	// ProviderPiper is local Piper TTS.
	ProviderPiper ProviderType = "piper"
	// ProviderKokoro is local Kokoro TTS (legacy, Python-based).
	ProviderKokoro ProviderType = "kokoro"
	// ProviderSherpa is local TTS using sherpa-onnx (native Go, no Python).
	ProviderSherpa ProviderType = "sherpa"
	// ProviderEspeakNG is local eSpeak-NG TTS.
	ProviderEspeakNG ProviderType = "espeak-ng"
	// ProviderEdge is Microsoft Edge TTS.
	ProviderEdge ProviderType = "edge-tts"
	// ProviderMacOSNative is macOS native TTS using AVSpeechSynthesizer.
	ProviderMacOSNative ProviderType = "macos-native"
)

// AudioFormat represents the output audio format.
type AudioFormat string

const (
	// FormatMP3 is MP3 audio format.
	FormatMP3 AudioFormat = "mp3"
	// FormatOPUS is OPUS audio format.
	FormatOPUS AudioFormat = "opus"
	// FormatAAC is AAC audio format.
	FormatAAC AudioFormat = "aac"
	// FormatFLAC is FLAC audio format.
	FormatFLAC AudioFormat = "flac"
	// FormatWAV is WAV audio format.
	FormatWAV AudioFormat = "wav"
	// FormatPCM is raw PCM audio format.
	FormatPCM AudioFormat = "pcm"
)

// Voice represents a TTS voice.
type Voice struct {
	// ID is the voice identifier.
	ID string `json:"id"`
	// Name is the display name.
	Name string `json:"name"`
	// Language is the voice language.
	Language string `json:"language"`
	// Gender is the voice gender.
	Gender string `json:"gender,omitempty"`
	// Description is the voice description.
	Description string `json:"description,omitempty"`
	// PreviewURL is a URL to preview the voice.
	PreviewURL string `json:"preview_url,omitempty"`
	// Provider is the TTS provider name (e.g., "Kokoro", "eSpeak-NG (Robotic)").
	Provider string `json:"provider,omitempty"`
	// Quality indicates voice quality level (e.g., "high", "medium", "low").
	Quality string `json:"quality,omitempty"`
}

// SynthesizeRequest represents a synthesis request.
type SynthesizeRequest struct {
	// Text is the text to synthesize.
	Text string
	// Voice is the voice ID to use.
	Voice string
	// Format is the output audio format.
	Format AudioFormat
	// Speed is the speech speed (0.25-4.0, default 1.0).
	Speed float32
	// Pitch is the speech pitch adjustment.
	Pitch float32
	// Volume is the speech volume (0-200, default 100).
	Volume float32
}

// SynthesizeResponse represents a synthesis response.
type SynthesizeResponse struct {
	// Audio is the synthesized audio data.
	Audio io.ReadCloser
	// Format is the audio format.
	Format AudioFormat
	// ContentType is the MIME content type.
	ContentType string
	// Duration is the estimated audio duration in seconds.
	Duration float64
}

// StreamCallback is called when streaming audio chunks are available.
type StreamCallback func(chunk []byte) error

// Provider defines the TTS provider interface.
type Provider interface {
	// Name returns the provider name.
	Name() string
	// Type returns the provider type.
	Type() ProviderType
	// Synthesize synthesizes text to speech.
	Synthesize(ctx context.Context, req *SynthesizeRequest) (*SynthesizeResponse, error)
	// SynthesizeStream synthesizes text with streaming audio output.
	SynthesizeStream(ctx context.Context, req *SynthesizeRequest, callback StreamCallback) error
	// ListVoices returns available voices.
	ListVoices(ctx context.Context) ([]Voice, error)
	// SupportedFormats returns the supported audio formats.
	SupportedFormats() []AudioFormat
	// MaxTextLength returns the maximum text length.
	MaxTextLength() int
}

// ProviderConfig holds the configuration for a TTS provider.
type ProviderConfig struct {
	// Type is the provider type.
	Type ProviderType `json:"type" yaml:"type"`
	// Enabled indicates if this provider is enabled.
	Enabled bool `json:"enabled" yaml:"enabled"`
	// APIKey is the API key for the provider.
	APIKey string `json:"api_key,omitempty" yaml:"api_key,omitempty"`
	// BaseURL is the base URL for the API.
	BaseURL string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	// DefaultVoice is the default voice ID.
	DefaultVoice string `json:"default_voice,omitempty" yaml:"default_voice,omitempty"`
	// DefaultFormat is the default audio format.
	DefaultFormat AudioFormat `json:"default_format,omitempty" yaml:"default_format,omitempty"`
	// MaxTextLength is the maximum text length.
	MaxTextLength int `json:"max_text_length,omitempty" yaml:"max_text_length,omitempty"`
}

// Service defines the TTS service interface.
type Service interface {
	// Synthesize synthesizes text using the default provider.
	Synthesize(ctx context.Context, req *SynthesizeRequest) (*SynthesizeResponse, error)
	// SynthesizeWithProvider synthesizes text using a specific provider.
	SynthesizeWithProvider(ctx context.Context, providerType ProviderType, req *SynthesizeRequest) (*SynthesizeResponse, error)
	// SynthesizeStream synthesizes text with streaming audio output.
	SynthesizeStream(ctx context.Context, req *SynthesizeRequest, callback StreamCallback) error
	// ListVoices returns available voices from the default provider.
	ListVoices(ctx context.Context) ([]Voice, error)
	// ListProviders returns all available providers.
	ListProviders() []ProviderType
	// GetDefaultProvider returns the default provider type.
	GetDefaultProvider() ProviderType
	// SetDefaultProvider sets the default provider type.
	SetDefaultProvider(providerType ProviderType) error
	// GetProvider returns the provider instance for a given provider type.
	GetProvider(providerType ProviderType) Provider
	// GetConfig returns the current TTS configuration (speed, pitch, volume).
	GetConfig() (speed, pitch, volume float32)
	// SetConfig sets the TTS configuration (speed, pitch, volume).
	SetConfig(speed, pitch, volume float32)
	// GetVocoderStatus returns vocoder model status.
	GetVocoderStatus() map[string]interface{}
	// DownloadVocoderModel starts downloading the vocoder model.
	DownloadVocoderModel(ctx context.Context) error
	// CancelVocoderDownload cancels the vocoder download.
	CancelVocoderDownload()
	// GetKokoroStatus returns Kokoro model status.
	GetKokoroStatus() map[string]interface{}
	// DownloadKokoroModel starts downloading the Kokoro model.
	DownloadKokoroModel(ctx context.Context) error
	// CancelKokoroDownload cancels the Kokoro download.
	CancelKokoroDownload()
	// Close cleans up all provider resources.
	Close()
}
