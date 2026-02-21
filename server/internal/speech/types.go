package speech

import (
	"time"
)

// Config holds unified speech configuration.
type Config struct {
	TTS TTSConfig `json:"tts" yaml:"tts" yaml:"tts"`
	ASR ASRConfig `json:"asr" yaml:"asr" yaml:"asr"`
}

// TTSConfig holds TTS-specific configuration.
type TTSConfig struct {
	Provider string  `json:"provider" yaml:"provider" yaml:"provider"`
	Model    string  `json:"model" yaml:"model" yaml:"model"`
	Speed    float32 `json:"speed" yaml:"speed" yaml:"speed"`    // Speech rate (0.5-2.0, default 1.0)
	Pitch    float32 `json:"pitch" yaml:"pitch" yaml:"pitch"`    // Pitch adjustment (-10 to 10, default 0)
	Volume   float32 `json:"volume" yaml:"volume" yaml:"volume"` // Volume (0-100, default 100)
}

// ASRConfig holds ASR-specific configuration.
type ASRConfig struct {
	Enabled        bool          `json:"enabled" yaml:"enabled" yaml:"enabled"`
	Provider       string        `json:"provider" yaml:"provider" yaml:"provider"`
	Model          string        `json:"model" yaml:"model" yaml:"model"`
	ModelDir       string        `json:"model_dir" yaml:"model_dir" yaml:"model_dir"`
	DefaultLang    string        `json:"default_language" yaml:"default_language" yaml:"default_language"`
	EditBeforeSend bool          `json:"edit_before_send" yaml:"edit_before_send" yaml:"edit_before_send"`
	MaxDuration    time.Duration `json:"max_duration" yaml:"max_duration" yaml:"max_duration"`
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

// EspeakStatus represents the eSpeak-NG runtime data status.
type EspeakStatus struct {
	Installed     bool   `json:"installed"`
	Path          string `json:"path,omitempty"`
	LanguageCount int    `json:"language_count"`
	DataSize      int64  `json:"data_size"`
	StaticLinked  bool   `json:"static_linked"`
}

// StatusResponse represents the unified speech status.
type StatusResponse struct {
	TTS    TTSStatus    `json:"tts"`
	ASR    ASRStatus    `json:"asr"`
	Espeak *EspeakStatus `json:"espeak,omitempty"`
}

// ComponentDownloadStatus represents the download/readiness status of a TTS component (e.g. Kokoro model, vocoder).
type ComponentDownloadStatus struct {
	Ready          bool    `json:"ready"`
	Downloading    bool    `json:"downloading"`
	Progress       float64 `json:"progress"`
	Error          string  `json:"error,omitempty"`
	InitStage      string  `json:"init_stage,omitempty"`
	Speed          string  `json:"speed,omitempty"`
	ETA            string  `json:"eta,omitempty"`
	File           string  `json:"file,omitempty"`
	FileIndex      int     `json:"file_index,omitempty"`
	TotalFiles     int     `json:"total_files,omitempty"`
	DownloadedSize string  `json:"downloaded_human,omitempty"`
}

// TTSStatus represents TTS status.
type TTSStatus struct {
	Ready              bool                                `json:"ready"`
	Provider           string                              `json:"provider"`
	ModelName          string                              `json:"model_name,omitempty"`
	AvailableProviders []string                            `json:"available_providers,omitempty"`
	Models             []interface{}                       `json:"models"`
	Components         map[string]*ComponentDownloadStatus `json:"components,omitempty"`
}

// ASRStatus represents ASR status.
type ASRStatus struct {
	Ready              bool              `json:"ready"`
	Provider           string            `json:"provider"`
	ModelName          string            `json:"model_name,omitempty"`
	StreamingSupported bool              `json:"streaming_supported"`
	EditBeforeSend     bool              `json:"edit_before_send"`
	Downloading        bool              `json:"downloading"`
	Progress           *Progress         `json:"progress,omitempty"`
	Downloads          []DownloadStatus  `json:"downloads,omitempty"`
	HasPending         bool              `json:"has_pending"`
	PermissionDenied   bool              `json:"permission_denied,omitempty"`
	PermissionError    string            `json:"permission_error,omitempty"`
	PermissionAppName  string            `json:"permission_app_name,omitempty"`
	OnDeviceSupported    bool              `json:"on_device_supported,omitempty"`
	OnDeviceOnly         bool              `json:"on_device_only,omitempty"`
	DictationAvailable   bool              `json:"dictation_available,omitempty"`
	OfflineLanguages     []string          `json:"offline_languages,omitempty"`
	Models               []interface{}     `json:"models"`
	AvailableProviders   []string          `json:"available_providers,omitempty"`
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
