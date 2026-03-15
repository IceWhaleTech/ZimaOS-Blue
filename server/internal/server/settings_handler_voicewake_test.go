package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/voicewake"
	"github.com/labstack/echo/v4"
)

type voiceWakeRuntimeSpy struct {
	starts    []voicewake.RuntimeConfig
	running   bool
	stopCount int
}

func (r *voiceWakeRuntimeSpy) Start(cfg voicewake.RuntimeConfig) error {
	r.starts = append(r.starts, cfg)
	r.running = true
	return nil
}

func (r *voiceWakeRuntimeSpy) Stop() error {
	r.stopCount++
	r.running = false
	return nil
}

func (r *voiceWakeRuntimeSpy) Running() bool {
	return r.running
}

func TestSettingsHandlerPatchVoiceWakePersistsAndRefreshes(t *testing.T) {
	store := kvstore.NewMemoryStore()
	handler := NewSettingsHandler(store)
	runtime := &voiceWakeRuntimeSpy{}
	manager := voicewake.NewManager(voicewake.ManagerConfig{
		Settings:  handler,
		Supported: true,
		Runtime:   runtime,
	})
	handler.SetVoiceWakeManager(manager)

	body := `{"voice_wake_enabled":true,"voice_wake_triggers":[" Blue ","blue","","Jarvis","` + strings.Repeat("x", 80) + `"],"voice_wake_locale":" en-US ","voice_wake_target_conversation_id":" conv-1 "}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/settings", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)

	if err := handler.Patch(c); err != nil {
		t.Fatalf("Patch() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("Patch() status = %d, want 200", rec.Code)
	}

	var got Settings
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.VoiceWakeEnabled == nil || !*got.VoiceWakeEnabled {
		t.Fatal("expected voice_wake_enabled=true")
	}
	if len(got.VoiceWakeTriggers) != 2 || got.VoiceWakeTriggers[0] != "Blue" || got.VoiceWakeTriggers[1] != "Jarvis" {
		t.Fatalf("voice_wake_triggers = %#v", got.VoiceWakeTriggers)
	}
	if got.VoiceWakeLocale != "en-US" {
		t.Fatalf("voice_wake_locale = %q, want %q", got.VoiceWakeLocale, "en-US")
	}
	if got.VoiceWakeTargetConversationID != "conv-1" {
		t.Fatalf("voice_wake_target_conversation_id = %q, want %q", got.VoiceWakeTargetConversationID, "conv-1")
	}
	if len(runtime.starts) != 1 {
		t.Fatalf("runtime starts = %d, want 1", len(runtime.starts))
	}

	reloaded := NewSettingsHandler(store)
	if !reloaded.GetVoiceWakeEnabled() {
		t.Fatal("expected reloaded voice wake enabled")
	}
	if got := reloaded.GetVoiceWakeTriggers(); len(got) != 2 || got[0] != "Blue" || got[1] != "Jarvis" {
		t.Fatalf("reloaded triggers = %#v", got)
	}
	if got := reloaded.GetVoiceWakeLocale(); got != "en-US" {
		t.Fatalf("reloaded locale = %q", got)
	}
	if got := reloaded.GetVoiceWakeTargetConversationID(); got != "conv-1" {
		t.Fatalf("reloaded target = %q", got)
	}
}

func TestSettingsHandlerUpdateVoiceWakeClearsFields(t *testing.T) {
	store := kvstore.NewMemoryStore()
	handler := NewSettingsHandler(store)
	enabled := true
	handler.settings.VoiceWakeEnabled = &enabled
	handler.settings.VoiceWakeTriggers = []string{"Blue", "Jarvis"}
	handler.settings.VoiceWakeLocale = "en-US"
	handler.settings.VoiceWakeTargetConversationID = "conv-1"
	if err := store.SetJSON(context.Background(), settingsKVKey, handler.settings, 0); err != nil {
		t.Fatalf("seed settings: %v", err)
	}

	body := `{"voice_wake_enabled":false,"voice_wake_triggers":[],"voice_wake_locale":" ","voice_wake_target_conversation_id":" "}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/settings", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)

	if err := handler.Update(c); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("Update() status = %d, want 200", rec.Code)
	}

	reloaded := NewSettingsHandler(store)
	if reloaded.GetVoiceWakeEnabled() {
		t.Fatal("expected voice wake disabled")
	}
	if got := reloaded.GetVoiceWakeTriggers(); len(got) != 1 || got[0] != "Blue" {
		t.Fatalf("GetVoiceWakeTriggers() = %#v, want default [Blue]", got)
	}
	if got := reloaded.GetVoiceWakeLocale(); got != "" {
		t.Fatalf("GetVoiceWakeLocale() = %q, want empty", got)
	}
	if got := reloaded.GetVoiceWakeTargetConversationID(); got != "" {
		t.Fatalf("GetVoiceWakeTargetConversationID() = %q, want empty", got)
	}
}
