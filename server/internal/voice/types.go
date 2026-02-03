// Package voice provides voice assistant functionality.
// This package combines STT and TTS services with WebSocket streaming
// for real-time voice interaction.
package voice

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrSessionNotFound is returned when a session is not found.
	ErrSessionNotFound = errors.New("voice session not found")
	// ErrSessionExpired is returned when a session has expired.
	ErrSessionExpired = errors.New("voice session expired")
	// ErrInvalidAudio is returned when the audio is invalid.
	ErrInvalidAudio = errors.New("invalid audio data")
	// ErrProcessingFailed is returned when processing fails.
	ErrProcessingFailed = errors.New("voice processing failed")
)

// SessionState represents the state of a voice session.
type SessionState string

const (
	// StateIdle indicates the session is idle.
	StateIdle SessionState = "idle"
	// StateListening indicates the session is listening for audio.
	StateListening SessionState = "listening"
	// StateProcessing indicates the session is processing audio.
	StateProcessing SessionState = "processing"
	// StateSpeaking indicates the session is playing audio response.
	StateSpeaking SessionState = "speaking"
)

// Session represents a voice session.
type Session struct {
	// ID is the session identifier.
	ID string `json:"id"`
	// UserID is the user identifier.
	UserID string `json:"user_id"`
	// State is the current session state.
	State SessionState `json:"state"`
	// Language is the session language.
	Language string `json:"language"`
	// Voice is the TTS voice ID.
	Voice string `json:"voice"`
	// CreatedAt is when the session was created.
	CreatedAt time.Time `json:"created_at"`
	// LastActivity is the last activity time.
	LastActivity time.Time `json:"last_activity"`
	// ConversationID is the linked conversation ID.
	ConversationID string `json:"conversation_id,omitempty"`
}

// VoiceMessage represents a voice message in the conversation.
type VoiceMessage struct {
	// ID is the message identifier.
	ID string `json:"id"`
	// SessionID is the session identifier.
	SessionID string `json:"session_id"`
	// Role is the message role (user or assistant).
	Role string `json:"role"`
	// Text is the transcribed or generated text.
	Text string `json:"text"`
	// AudioURL is the URL to the audio file (if stored).
	AudioURL string `json:"audio_url,omitempty"`
	// Duration is the audio duration in seconds.
	Duration float64 `json:"duration,omitempty"`
	// Timestamp is when the message was created.
	Timestamp time.Time `json:"timestamp"`
}

// TranscribeRequest represents a transcription request.
type TranscribeRequest struct {
	// SessionID is the session identifier.
	SessionID string `json:"session_id,omitempty"`
	// Language is the language code.
	Language string `json:"language,omitempty"`
	// Format is the audio format.
	Format string `json:"format"`
}

// TranscribeResponse represents a transcription response.
type TranscribeResponse struct {
	// Text is the transcribed text.
	Text string `json:"text"`
	// Language is the detected language.
	Language string `json:"language,omitempty"`
	// Duration is the audio duration.
	Duration float64 `json:"duration,omitempty"`
	// Confidence is the confidence score.
	Confidence float64 `json:"confidence,omitempty"`
}

// SynthesizeRequest represents a synthesis request.
type SynthesizeRequest struct {
	// Text is the text to synthesize.
	Text string `json:"text"`
	// Voice is the voice ID.
	Voice string `json:"voice,omitempty"`
	// Format is the output format.
	Format string `json:"format,omitempty"`
	// Speed is the speech speed.
	Speed float32 `json:"speed,omitempty"`
	// Provider is the TTS provider (espeak-ng, edge-tts, sherpa-onnx).
	Provider string `json:"provider,omitempty"`
}

// VoiceConfig represents voice configuration.
type VoiceConfig struct {
	// Language is the default language.
	Language string `json:"language"`
	// Voice is the default TTS voice.
	Voice string `json:"voice"`
	// WakeWord is the wake word (optional).
	WakeWord string `json:"wake_word,omitempty"`
	// WakeWordEnabled indicates if wake word detection is enabled.
	WakeWordEnabled bool `json:"wake_word_enabled"`
	// ContinuousListening indicates if continuous listening is enabled.
	ContinuousListening bool `json:"continuous_listening"`
	// AutoPlayResponse indicates if responses should auto-play.
	AutoPlayResponse bool `json:"auto_play_response"`
}

// WebSocketMessage represents a WebSocket message.
type WebSocketMessage struct {
	// Type is the message type.
	Type string `json:"type"`
	// Data is the message data.
	Data interface{} `json:"data,omitempty"`
	// Error is the error message (if any).
	Error string `json:"error,omitempty"`
}

// WebSocket message types
const (
	// MsgTypeAudio is an audio data message.
	MsgTypeAudio = "audio"
	// MsgTypeTranscript is a transcription result message.
	MsgTypeTranscript = "transcript"
	// MsgTypeResponse is an assistant response message.
	MsgTypeResponse = "response"
	// MsgTypeAudioResponse is an audio response message.
	MsgTypeAudioResponse = "audio_response"
	// MsgTypeStateChange is a state change message.
	MsgTypeStateChange = "state_change"
	// MsgTypeError is an error message.
	MsgTypeError = "error"
	// MsgTypeConfig is a configuration message.
	MsgTypeConfig = "config"
	// MsgTypeWakeWord is a wake word detected message.
	MsgTypeWakeWord = "wake_word"
	// MsgTypePing is a ping message.
	MsgTypePing = "ping"
	// MsgTypePong is a pong message.
	MsgTypePong = "pong"
)

// Service defines the voice service interface.
type Service interface {
	// CreateSession creates a new voice session.
	CreateSession(ctx context.Context, userID string, config *VoiceConfig) (*Session, error)
	// GetSession gets a session by ID.
	GetSession(ctx context.Context, sessionID string) (*Session, error)
	// UpdateSessionState updates the session state.
	UpdateSessionState(ctx context.Context, sessionID string, state SessionState) error
	// CloseSession closes a session.
	CloseSession(ctx context.Context, sessionID string) error
	// Transcribe transcribes audio to text.
	Transcribe(ctx context.Context, req *TranscribeRequest, audio []byte) (*TranscribeResponse, error)
	// Synthesize synthesizes text to speech.
	Synthesize(ctx context.Context, req *SynthesizeRequest) ([]byte, string, error)
	// ProcessVoiceInput processes voice input and returns a response.
	ProcessVoiceInput(ctx context.Context, sessionID string, text string) (string, error)
}
