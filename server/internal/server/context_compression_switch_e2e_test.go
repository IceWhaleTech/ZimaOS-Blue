package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestContextCompressionSwitchesE2E_ModeAndSmallModelGate(t *testing.T) {
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(nil, llm.NewProviderRegistry(), toolRegistry)
	handler.SetSettingsHandler(settings)

	marker := "SM_COMPRESS_MARKER_123"
	ready := true
	sm := &smallModelRuntimeMock{
		respText: "Goal\n- " + marker + "\nInstructions\n- Keep it short\nDiscoveries\n- ok\nAccomplished\n- ok\n",
		ready:    &ready,
	}
	handler.SetSmallModelRuntime(sm)

	patch := func(t *testing.T, body string) {
		t.Helper()
		e := echo.New()
		req := httptest.NewRequest(http.MethodPatch, "/api/settings", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		if err := settings.Patch(e.NewContext(req, rec)); err != nil {
			t.Fatalf("settings Patch() error = %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("settings Patch() status=%d, want 200", rec.Code)
		}
	}

	all := []memory.Message{
		{Role: "user", Content: "Goal: build a switch matrix for assistant toggles."},
		{Role: "assistant", Content: "Ok."},
		{Role: "user", Content: "Discovery: Proxy pruner trims tool output to reduce prompt size."},
		{Role: "assistant", Content: "Noted."},
		{Role: "user", Content: "We should verify offline fallback works when providers are down."},
		{Role: "assistant", Content: "Understood."},
		{Role: "user", Content: "Now summarize the earlier context for reference."},
		{Role: "assistant", Content: "Sure."},
	}

	t.Run("context_compression_mode=offline never calls small model even when enabled", func(t *testing.T) {
		sm.calls = 0
		patch(t, `{"small_model_enabled":true,"small_model_context_compress_enabled":true,"small_model_summary_enabled":true,"context_compression_mode":"offline"}`)
		got := handler.generateSummarySync(context.Background(), "conv-offline", all, nil)
		if sm.calls != 0 {
			t.Fatalf("small model calls=%d, want 0 when context_compression_mode=offline", sm.calls)
		}
		if strings.Contains(got, marker) {
			t.Fatalf("expected offline compression not to include small-model marker, got=%q", got)
		}
		if strings.TrimSpace(got) == "" {
			t.Fatalf("expected non-empty offline compression summary")
		}
	})

	t.Run("small_model_context_compress_enabled=false disables small-model context compression (auto mode)", func(t *testing.T) {
		sm.calls = 0
		patch(t, `{"small_model_enabled":true,"small_model_context_compress_enabled":false,"small_model_summary_enabled":false,"context_compression_mode":"auto"}`)
		got := handler.generateSummarySync(context.Background(), "conv-auto-off", all, nil)
		if sm.calls != 0 {
			t.Fatalf("small model calls=%d, want 0 when small_model_context_compress_enabled=false", sm.calls)
		}
		if strings.Contains(got, marker) {
			t.Fatalf("expected offline compression not to include small-model marker, got=%q", got)
		}
		if strings.TrimSpace(got) == "" {
			t.Fatalf("expected non-empty offline compression summary")
		}
	})

	t.Run("small-model context compression runs in auto mode when enabled + ready", func(t *testing.T) {
		sm.calls = 0
		patch(t, `{"small_model_enabled":true,"small_model_context_compress_enabled":true,"small_model_summary_enabled":false,"context_compression_mode":"auto"}`)
		got := handler.generateSummarySync(context.Background(), "conv-auto-on", all, nil)
		if sm.calls < 1 {
			t.Fatalf("expected small model to be called when context compression enabled, calls=%d", sm.calls)
		}
		if !strings.Contains(got, marker) {
			t.Fatalf("expected small-model compression output to include marker, got=%q", got)
		}
	})

	t.Run("small_model_enabled=false prevents small-model context compression even if switch is on", func(t *testing.T) {
		sm.calls = 0
		patch(t, `{"small_model_enabled":false,"small_model_context_compress_enabled":true,"small_model_summary_enabled":false,"context_compression_mode":"auto"}`)
		got := handler.generateSummarySync(context.Background(), "conv-disabled", all, nil)
		if sm.calls != 0 {
			t.Fatalf("small model calls=%d, want 0 when small_model_enabled=false", sm.calls)
		}
		if strings.Contains(got, marker) {
			t.Fatalf("expected offline compression not to include small-model marker, got=%q", got)
		}
	})
}
