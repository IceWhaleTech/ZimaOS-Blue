package voicewake

import (
	"context"
	"errors"
	"time"
)

var (
	ErrTargetUnavailable     = errors.New("voice wake target unavailable")
	ErrSpeechUnauthorized    = errors.New("voice wake speech authorization missing")
	ErrMicrophoneUnavailable = errors.New("voice wake microphone unavailable")
	ErrRecognizerUnavailable = errors.New("voice wake recognizer unavailable")
)

const (
	DefaultTrigger            = "Hey Blue"
	DefaultMinPostTriggerGap  = 450 * time.Millisecond
	DefaultPostTriggerSilence = 2 * time.Second
	DefaultTriggerOnlySilence = 5 * time.Second
	DefaultHardStop           = 120 * time.Second
	DefaultPreDetectSilence   = 1 * time.Second
	DefaultTriggerPauseWindow = 550 * time.Millisecond
	DefaultPostSendDebounce   = 350 * time.Millisecond
	defaultMinCommandLength   = 1
)

type SettingsSource interface {
	GetVoiceWakeEnabled() bool
	GetVoiceWakeTriggers() []string
	GetVoiceWakeLocale() string
	GetVoiceWakeTargetConversationID() string
	GetLocale() string
}

type ChatSubmitter interface {
	SubmitVoiceWakeMessage(ctx context.Context, conversationID, text string) error
}

type Status struct {
	Supported            bool       `json:"supported"`
	Enabled              bool       `json:"enabled"`
	Running              bool       `json:"running"`
	Platform             string     `json:"platform"`
	TargetConversationID string     `json:"target_conversation_id,omitempty"`
	Triggers             []string   `json:"triggers,omitempty"`
	Locale               string     `json:"locale,omitempty"`
	SpeechAuthorized     bool       `json:"speech_authorized"`
	MicrophoneReady      bool       `json:"microphone_ready"`
	Reason               string     `json:"reason,omitempty"`
	LastError            string     `json:"last_error,omitempty"`
	LastTriggeredAt      *time.Time `json:"last_triggered_at,omitempty"`
	LastSentAt           *time.Time `json:"last_sent_at,omitempty"`
}

type RuntimeConfig struct {
	Locale             string
	Triggers           []string
	MinPostTriggerGap  time.Duration
	PostTriggerSilence time.Duration
	TriggerOnlySilence time.Duration
	HardStop           time.Duration
	PreDetectSilence   time.Duration
	TriggerPauseWindow time.Duration
	PostSendDebounce   time.Duration
	OnTriggered        func(match GateMatch)
	OnFinalized        func(text string)
	OnError            func(error)
}

type runtimeController interface {
	Start(cfg RuntimeConfig) error
	Stop() error
	Running() bool
}

func defaultTriggers(values []string) []string {
	if len(values) == 0 {
		return []string{DefaultTrigger}
	}
	return values
}

func DefaultTriggers(values []string) []string {
	return defaultTriggers(values)
}
