package speech

import (
	"time"
)

// Config holds unified speech configuration.
type Config struct {
	TTS TTSConfig `json:"tts" yaml:"tts" mapstructure:"tts"`
	ASR ASRConfig `json:"asr" yaml:"asr" mapstructure:"asr"`
}

// TTSConfig holds TTS-specific configuration.
type TTSConfig struct {
	Provider string  `json:"provider" yaml:"provider" mapstructure:"provider"`
	Model    string  `json:"model" yaml:"model" mapstructure:"model"`
	Speed    float32 `json:"speed" yaml:"speed" mapstructure:"speed"`    // Speech rate (0.5-2.0, default 1.0)
	Pitch    float32 `json:"pitch" yaml:"pitch" mapstructure:"pitch"`    // Pitch adjustment (-10 to 10, default 0)
	Volume   float32 `json:"volume" yaml:"volume" mapstructure:"volume"` // Volume (0-100, default 100)
}

// ASRConfig holds ASR-specific configuration.
type ASRConfig struct {
	Enabled        bool          `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
	Provider       string        `json:"provider" yaml:"provider" mapstructure:"provider"`
	Model          string        `json:"model" yaml:"model" mapstructure:"model"`
	ModelDir       string        `json:"model_dir" yaml:"model_dir" mapstructure:"model_dir"`
	DefaultLang    string        `json:"default_language" yaml:"default_language" mapstructure:"default_language"`
	EditBeforeSend bool          `json:"edit_before_send" yaml:"edit_before_send" mapstructure:"edit_before_send"`
	MaxDuration    time.Duration `json:"max_duration" yaml:"max_duration" mapstructure:"max_duration"`
}

// TranscriptionResult with edit support.
type TranscriptionResult struct {
	Text       string  `json:"text"`
	Language   string  `json:"language,omitempty"`
	Duration   float64 `json:"duration,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
	Editable   bool    `json:"editable"`
	SessionID  string  `json:"session_id,omitempty"`
}

// ConfirmRequest represents a request to confirm edited transcription.
type ConfirmRequest struct {
	SessionID string `json:"session_id"`
	Text      string `json:"text"`
}

// StatusResponse represents the unified speech status.
type StatusResponse struct {
	TTS TTSStatus `json:"tts"`
	ASR ASRStatus `json:"asr"`
}

// TTSStatus represents TTS status.
type TTSStatus struct {
	Ready     bool   `json:"ready"`
	Provider  string `json:"provider"`
	ModelType string `json:"model_type,omitempty"`
}

// ASRStatus represents ASR status.
type ASRStatus struct {
	Ready              bool              `json:"ready"`
	Provider           string            `json:"provider"`
	ModelType          string            `json:"model_type,omitempty"`
	StreamingSupported bool              `json:"streaming_supported"`
	EditBeforeSend     bool              `json:"edit_before_send"`
	Downloading        bool              `json:"downloading"`
	Progress           *Progress         `json:"progress,omitempty"`
	Downloads          []DownloadStatus  `json:"downloads,omitempty"` // All active downloads
	HasPending         bool              `json:"has_pending"`
}

// DownloadStatus represents a single model download status.
type DownloadStatus struct {
	ModelType  string   `json:"model_type"`
	Progress   Progress `json:"progress"`
}

// Progress represents download progress.
type Progress struct {
	File       string  `json:"file"`
	Downloaded int64   `json:"downloaded"`
	Total      int64   `json:"total"`
	Percentage float64 `json:"percentage"`
	SpeedHuman string  `json:"speed_human"`
	ETA        string  `json:"eta"`
}

// ModelInfo represents information about a speech model.
type ModelInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Type        string   `json:"type"` // "tts" or "asr"
	Languages   []string `json:"languages,omitempty"`
	Size        string   `json:"size"`
	Streaming   bool     `json:"streaming,omitempty"`
	Downloaded  bool     `json:"downloaded"`
}

// ModelsResponse represents the list of available models.
type ModelsResponse struct {
	TTS []ModelInfo `json:"tts"`
	ASR []ModelInfo `json:"asr"`
}

// DownloadRequest represents a model download request.
type DownloadRequest struct {
	ModelType string `json:"model_type"`
}

// SwitchRequest represents a model switch request.
type SwitchRequest struct {
	ModelType string `json:"model_type"`
}

// SwitchProviderRequest represents a TTS provider switch request.
type SwitchProviderRequest struct {
	Provider string `json:"provider"`
}
