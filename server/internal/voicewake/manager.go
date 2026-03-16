package voicewake

import (
	"context"
	"errors"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"
)

type Manager struct {
	mu        sync.RWMutex
	settings  SettingsSource
	submitter ChatSubmitter
	runtime   runtimeController
	supported bool
	platform  string
	status    Status
	activeCfg managerRuntimeConfig
}

type ManagerConfig struct {
	Settings  SettingsSource
	Submitter ChatSubmitter
	Supported bool
	Runtime   runtimeController
}

type managerRuntimeConfig struct {
	Enabled              bool
	Triggers             []string
	Locale               string
	TargetConversationID string
}

func NewManager(cfg ManagerConfig) *Manager {
	rt := cfg.Runtime
	if rt == nil {
		rt = newRuntime()
	}
	m := &Manager{
		settings:  cfg.Settings,
		submitter: cfg.Submitter,
		runtime:   rt,
		supported: cfg.Supported,
		platform:  runtime.GOOS,
	}
	m.status.Platform = m.platform
	m.status.Supported = m.supported
	if !m.supported {
		m.status.Reason = "unsupported"
	}
	return m
}

func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.Running = false
	m.activeCfg = managerRuntimeConfig{}
	return m.runtime.Stop()
}

func (m *Manager) Status() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := m.status
	if len(out.Triggers) > 0 {
		out.Triggers = append([]string(nil), out.Triggers...)
	}
	return out
}

func (m *Manager) Refresh(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.refreshLocked(ctx, false)
}

func (m *Manager) Restart(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	_ = m.runtime.Stop()
	m.status.Running = false
	return m.refreshLocked(ctx, true)
}

func (m *Manager) refreshLocked(_ context.Context, force bool) error {
	cfg := m.snapshotLocked()
	permissions := permissionStatus{}
	if m.supported {
		permissions = probePermissionStatus()
	}
	m.status.Platform = m.platform
	m.status.Supported = m.supported
	m.status.Enabled = cfg.Enabled
	m.status.TargetConversationID = cfg.TargetConversationID
	m.status.Triggers = append([]string(nil), cfg.Triggers...)
	m.status.Locale = cfg.Locale
	m.status.SpeechAuthorized = permissions.SpeechAuthorized
	m.status.MicrophoneReady = permissions.MicrophoneReady

	if !m.supported {
		m.status.Running = false
		m.status.Reason = "unsupported"
		m.activeCfg = managerRuntimeConfig{}
		_ = m.runtime.Stop()
		return nil
	}
	if !cfg.Enabled {
		m.status.Running = false
		m.status.Reason = "disabled"
		m.status.LastError = ""
		m.activeCfg = managerRuntimeConfig{}
		_ = m.runtime.Stop()
		return nil
	}
	if strings.TrimSpace(cfg.TargetConversationID) == "" {
		m.status.Running = false
		m.status.Reason = "target_missing"
		m.status.LastError = ""
		m.activeCfg = managerRuntimeConfig{}
		_ = m.runtime.Stop()
		return nil
	}
	if !force && m.runtime.Running() {
		if m.runtimeConfigMatchesLocked(cfg) {
			m.status.Running = true
			m.status.Reason = "running"
			m.status.SpeechAuthorized = true
			m.status.MicrophoneReady = true
			return nil
		}
		_ = m.runtime.Stop()
		m.status.Running = false
	}

	startCfg := RuntimeConfig{
		Locale:             cfg.Locale,
		Triggers:           cfg.Triggers,
		MinPostTriggerGap:  DefaultMinPostTriggerGap,
		PostTriggerSilence: DefaultPostTriggerSilence,
		TriggerOnlySilence: DefaultTriggerOnlySilence,
		HardStop:           DefaultHardStop,
		PreDetectSilence:   DefaultPreDetectSilence,
		TriggerPauseWindow: DefaultTriggerPauseWindow,
		PostSendDebounce:   DefaultPostSendDebounce,
		OnTriggered:        m.handleTriggered,
		OnFinalized: func(text string) {
			m.handleFinalized(cfg.TargetConversationID, text)
		},
		OnError: m.handleRuntimeError,
	}
	if err := m.runtime.Start(startCfg); err != nil {
		m.status.Running = false
		m.status.LastError = err.Error()
		m.activeCfg = managerRuntimeConfig{}
		switch {
		case errors.Is(err, ErrSpeechUnauthorized):
			m.status.Reason = "speech_permission_denied"
			m.status.SpeechAuthorized = false
			m.status.MicrophoneReady = false
		case errors.Is(err, ErrMicrophoneUnavailable):
			m.status.Reason = "microphone_unavailable"
			m.status.SpeechAuthorized = true
			m.status.MicrophoneReady = false
		default:
			m.status.Reason = "start_failed"
			m.status.SpeechAuthorized = false
			m.status.MicrophoneReady = false
		}
		return err
	}
	m.status.Running = true
	m.status.Reason = "running"
	m.status.LastError = ""
	m.status.SpeechAuthorized = true
	m.status.MicrophoneReady = true
	m.activeCfg = cfg
	return nil
}

func (m *Manager) runtimeConfigMatchesLocked(cfg managerRuntimeConfig) bool {
	return m.activeCfg.Enabled == cfg.Enabled &&
		m.activeCfg.Locale == cfg.Locale &&
		m.activeCfg.TargetConversationID == cfg.TargetConversationID &&
		slices.Equal(m.activeCfg.Triggers, cfg.Triggers)
}

func (m *Manager) snapshotLocked() managerRuntimeConfig {
	locale := ""
	if m.settings != nil {
		locale = strings.TrimSpace(m.settings.GetVoiceWakeLocale())
		if locale == "" {
			locale = strings.TrimSpace(m.settings.GetLocale())
		}
	}
	if locale == "" {
		locale = "en-US"
	}
	triggers := []string{DefaultTrigger}
	if m.settings != nil {
		triggers = defaultTriggers(m.settings.GetVoiceWakeTriggers())
	}
	return managerRuntimeConfig{
		Enabled:  m.settings != nil && m.settings.GetVoiceWakeEnabled(),
		Triggers: append([]string(nil), triggers...),
		Locale:   locale,
		TargetConversationID: strings.TrimSpace(func() string {
			if m.settings == nil {
				return ""
			}
			return m.settings.GetVoiceWakeTargetConversationID()
		}()),
	}
}

func (m *Manager) handleTriggered(match GateMatch) {
	now := time.Now().UTC()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.LastTriggeredAt = &now
	m.status.Reason = "running"
	m.status.LastError = ""
}

func (m *Manager) handleFinalized(targetConversationID, text string) {
	text = strings.TrimSpace(text)
	if text == "" || m.submitter == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		err := m.submitter.SubmitVoiceWakeMessage(ctx, targetConversationID, text)
		m.mu.Lock()
		defer m.mu.Unlock()
		if err != nil {
			m.status.LastError = err.Error()
			if errors.Is(err, ErrTargetUnavailable) {
				m.status.Reason = "target_unavailable"
				m.status.Running = false
				m.status.MicrophoneReady = false
				m.activeCfg = managerRuntimeConfig{}
				_ = m.runtime.Stop()
				return
			}
			m.status.Reason = "send_failed"
			return
		}
		now := time.Now().UTC()
		m.status.LastSentAt = &now
		m.status.Reason = "running"
		m.status.LastError = ""
	}()
}

func (m *Manager) handleRuntimeError(err error) {
	if err == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.LastError = err.Error()
	m.status.Running = false
	switch {
	case errors.Is(err, ErrSpeechUnauthorized):
		m.status.Reason = "speech_permission_denied"
		m.status.SpeechAuthorized = false
		m.status.MicrophoneReady = false
	case errors.Is(err, ErrMicrophoneUnavailable):
		m.status.Reason = "microphone_unavailable"
		m.status.SpeechAuthorized = true
		m.status.MicrophoneReady = false
	default:
		m.status.Reason = "runtime_error"
	}
	m.activeCfg = managerRuntimeConfig{}
}
