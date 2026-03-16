package voicewake

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeSettingsSource struct {
	enabled    bool
	triggers   []string
	locale     string
	target     string
	baseLocale string
}

func (s *fakeSettingsSource) GetVoiceWakeEnabled() bool { return s.enabled }
func (s *fakeSettingsSource) GetVoiceWakeTriggers() []string {
	return append([]string(nil), s.triggers...)
}
func (s *fakeSettingsSource) GetVoiceWakeLocale() string               { return s.locale }
func (s *fakeSettingsSource) GetVoiceWakeTargetConversationID() string { return s.target }
func (s *fakeSettingsSource) GetLocale() string                        { return s.baseLocale }

type fakeRuntime struct {
	mu        sync.Mutex
	startErr  error
	running   bool
	starts    []RuntimeConfig
	stopCount int
}

func (r *fakeRuntime) Start(cfg RuntimeConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.starts = append(r.starts, cfg)
	if r.startErr != nil {
		r.running = false
		return r.startErr
	}
	r.running = true
	return nil
}

func (r *fakeRuntime) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopCount++
	r.running = false
	return nil
}

func (r *fakeRuntime) Running() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running
}

func (r *fakeRuntime) StartCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.starts)
}

func (r *fakeRuntime) StopCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stopCount
}

func (r *fakeRuntime) LastStart() RuntimeConfig {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.starts) == 0 {
		return RuntimeConfig{}
	}
	return r.starts[len(r.starts)-1]
}

type fakeSubmitter struct {
	mu    sync.Mutex
	err   error
	calls []struct {
		conversationID string
		text           string
	}
}

func (s *fakeSubmitter) SubmitVoiceWakeMessage(_ context.Context, conversationID, text string) error {
	s.mu.Lock()
	s.calls = append(s.calls, struct {
		conversationID string
		text           string
	}{conversationID: conversationID, text: text})
	err := s.err
	s.mu.Unlock()
	return err
}

func waitForCondition(t *testing.T, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not satisfied before timeout")
}

func TestManagerRefreshStateTransitions(t *testing.T) {
	tests := []struct {
		name       string
		supported  bool
		settings   *fakeSettingsSource
		startErr   error
		wantReason string
		wantRun    bool
		wantStart  int
	}{
		{
			name:      "unsupported",
			supported: false,
			settings: &fakeSettingsSource{
				enabled: true,
				target:  "conv-1",
			},
			wantReason: "unsupported",
			wantRun:    false,
			wantStart:  0,
		},
		{
			name:      "disabled",
			supported: true,
			settings: &fakeSettingsSource{
				enabled: false,
				target:  "conv-1",
			},
			wantReason: "disabled",
			wantRun:    false,
			wantStart:  0,
		},
		{
			name:      "target missing",
			supported: true,
			settings: &fakeSettingsSource{
				enabled: true,
				target:  "",
			},
			wantReason: "target_missing",
			wantRun:    false,
			wantStart:  0,
		},
		{
			name:      "running",
			supported: true,
			settings: &fakeSettingsSource{
				enabled:  true,
				target:   "conv-1",
				triggers: []string{"Blue"},
			},
			wantReason: "running",
			wantRun:    true,
			wantStart:  1,
		},
		{
			name:      "speech denied",
			supported: true,
			settings: &fakeSettingsSource{
				enabled: true,
				target:  "conv-1",
			},
			startErr:   ErrSpeechUnauthorized,
			wantReason: "speech_permission_denied",
			wantRun:    false,
			wantStart:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := &fakeRuntime{startErr: tt.startErr}
			manager := NewManager(ManagerConfig{
				Settings:  tt.settings,
				Supported: tt.supported,
				Runtime:   rt,
			})
			_ = manager.Refresh(context.Background())
			status := manager.Status()
			if status.Reason != tt.wantReason {
				t.Fatalf("status.Reason = %q, want %q", status.Reason, tt.wantReason)
			}
			if status.Running != tt.wantRun {
				t.Fatalf("status.Running = %v, want %v", status.Running, tt.wantRun)
			}
			if got := rt.StartCount(); got != tt.wantStart {
				t.Fatalf("runtime.StartCount() = %d, want %d", got, tt.wantStart)
			}
		})
	}
}

func TestManagerRefreshPreservesProbedPermissionsWhenDisabled(t *testing.T) {
	previousProbe := probePermissionStatus
	probePermissionStatus = func() permissionStatus {
		return permissionStatus{
			SpeechAuthorized: true,
			MicrophoneReady:  true,
		}
	}
	defer func() {
		probePermissionStatus = previousProbe
	}()

	manager := NewManager(ManagerConfig{
		Settings: &fakeSettingsSource{
			enabled: false,
			target:  "conv-1",
		},
		Supported: true,
		Runtime:   &fakeRuntime{},
	})

	if err := manager.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	status := manager.Status()
	if !status.SpeechAuthorized {
		t.Fatal("expected speech authorization to reflect probed permission state")
	}
	if !status.MicrophoneReady {
		t.Fatal("expected microphone readiness to reflect probed permission state")
	}
}

func TestManagerRefreshRestartsWhenConfigChanges(t *testing.T) {
	settings := &fakeSettingsSource{
		enabled:  true,
		target:   "conv-1",
		triggers: []string{"Blue"},
		locale:   "en-US",
	}
	rt := &fakeRuntime{}
	manager := NewManager(ManagerConfig{
		Settings:  settings,
		Supported: true,
		Runtime:   rt,
	})

	if err := manager.Refresh(context.Background()); err != nil {
		t.Fatalf("initial Refresh() error = %v", err)
	}
	settings.locale = "zh-CN"

	if err := manager.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() after locale change error = %v", err)
	}
	if got := rt.StartCount(); got != 2 {
		t.Fatalf("runtime.StartCount() = %d, want 2", got)
	}
	if got := rt.StopCount(); got != 1 {
		t.Fatalf("runtime.StopCount() = %d, want 1", got)
	}
	if got := rt.LastStart().Locale; got != "zh-CN" {
		t.Fatalf("last runtime locale = %q, want %q", got, "zh-CN")
	}
}

func TestManagerHandleFinalizedStopsOnTargetUnavailable(t *testing.T) {
	settings := &fakeSettingsSource{
		enabled:  true,
		target:   "conv-1",
		triggers: []string{"Blue"},
	}
	rt := &fakeRuntime{}
	submitter := &fakeSubmitter{err: ErrTargetUnavailable}
	manager := NewManager(ManagerConfig{
		Settings:  settings,
		Submitter: submitter,
		Supported: true,
		Runtime:   rt,
	})

	if err := manager.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	manager.handleFinalized("conv-1", "open settings")

	waitForCondition(t, func() bool {
		return manager.Status().Reason == "target_unavailable"
	})

	status := manager.Status()
	if status.Running {
		t.Fatal("expected runtime stopped after target_unavailable")
	}
	if got := rt.StopCount(); got == 0 {
		t.Fatal("expected runtime stop after target_unavailable")
	}
}

func TestManagerHandleRuntimeErrorMarksStatus(t *testing.T) {
	manager := NewManager(ManagerConfig{
		Settings:  &fakeSettingsSource{},
		Supported: true,
		Runtime:   &fakeRuntime{},
	})

	manager.handleRuntimeError(errors.New("boom"))
	status := manager.Status()
	if status.Reason != "runtime_error" {
		t.Fatalf("status.Reason = %q, want runtime_error", status.Reason)
	}
	if status.LastError != "boom" {
		t.Fatalf("status.LastError = %q, want %q", status.LastError, "boom")
	}
}
