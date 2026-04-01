package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestChannelFeatureIntentIRE2E_GatesAutoDeepResearchAndAgentMode(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	defer store.Close()

	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	providers := llm.NewProviderRegistry()
	capture := &requestCaptureProvider{}
	providers.Register(capture)

	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, providers, toolRegistry)
	handler.SetSettingsHandler(settings)

	// Ensure the forced-skill hint (when present) can actually be injected into
	// system prompt messages in this test harness.
	builder := agentcore.NewSystemPromptBuilder(&agentcore.Config{})
	builder.SetToolRegistry(toolRegistry)
	handler.SetSystemPromptBuilder(builder)

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

	findInMessages := func(msgs []llm.Message, needle string) bool {
		for _, m := range msgs {
			if strings.Contains(m.Content, needle) {
				return true
			}
		}
		return false
	}

	t.Run("deep-research hint is only auto-enabled when feature_intent_ir_enabled=true", func(t *testing.T) {
		msg := channel.Message{
			ID:          "m-research",
			ChannelName: "test",
			ChatID:      "c1",
			UserID:      "u1",
			Type:        channel.MessageTypeText,
			Content:     "Please do deep research on BlueAgent latest updates with sources",
			Metadata: map[string]interface{}{
				"language": "en-US",
			},
			Timestamp: time.Now(),
		}

		patch(t, `{"feature_intent_ir_enabled":false}`)
		if _, err := handler.ProcessChannelMessage(context.Background(), msg); err != nil {
			t.Fatalf("ProcessChannelMessage: %v", err)
		}
		req := capture.LastRequest()
		if findInMessages(req.Messages, "<selected_skill_candidates") {
			t.Fatalf("expected no forced skill hint when feature_intent_ir_enabled=false")
		}

		patch(t, `{"feature_intent_ir_enabled":true}`)
		if _, err := handler.ProcessChannelMessage(context.Background(), msg); err != nil {
			t.Fatalf("ProcessChannelMessage: %v", err)
		}
		req = capture.LastRequest()
		if !findInMessages(req.Messages, "<selected_skill_candidates") {
			t.Fatalf("expected forced skill hint when feature_intent_ir_enabled=true")
		}
	})

	t.Run("agent-mode system injection is gated by feature_intent_ir_enabled", func(t *testing.T) {
		msg := channel.Message{
			ID:          "m-agent",
			ChannelName: "test",
			ChatID:      "c2",
			UserID:      "u1",
			Type:        channel.MessageTypeText,
			Content:     "Please run in agent mode for this task",
			Metadata: map[string]interface{}{
				"language": "en-US",
			},
			Timestamp: time.Now(),
		}

		patch(t, `{"feature_intent_ir_enabled":false}`)
		if _, err := handler.ProcessChannelMessage(context.Background(), msg); err != nil {
			t.Fatalf("ProcessChannelMessage: %v", err)
		}
		req := capture.LastRequest()
		if findInMessages(req.Messages, "Agent Mode is auto-enabled by IR") {
			t.Fatalf("expected no agent-mode injection when feature_intent_ir_enabled=false")
		}

		patch(t, `{"feature_intent_ir_enabled":true}`)
		if _, err := handler.ProcessChannelMessage(context.Background(), msg); err != nil {
			t.Fatalf("ProcessChannelMessage: %v", err)
		}
		req = capture.LastRequest()
		if !findInMessages(req.Messages, "Agent Mode is auto-enabled by IR") {
			t.Fatalf("expected agent-mode injection when feature_intent_ir_enabled=true")
		}
	})
}
