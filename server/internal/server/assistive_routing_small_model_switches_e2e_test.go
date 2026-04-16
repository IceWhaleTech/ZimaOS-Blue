package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/smallmodel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestAssistiveRoutingAndSmallModelSwitchesHTTPE2E_SettingsPatchGetAndRouting(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	defer store.Close()

	settings := NewSettingsHandler(kvstore.NewMemoryStore())

	registry := llm.NewProviderRegistry()
	capture := &requestCaptureProvider{}
	registry.Register(capture)

	handler := NewChatHandler(store, registry, tools.NewRegistry())
	handler.SetSettingsHandler(settings)
	handler.SetSmallModelRuntime(&smallModelRuntimeMock{respText: "SM"})

	e := echo.New()
	api := e.Group("/api/v1")
	settings.RegisterRoutes(api)
	conversations := api.Group("/conversations")
	conversations.POST("/:id/messages", handler.SendMessage)
	api.GET("/small-model/stats", handler.SmallModelStatsHandler)
	api.POST("/small-model/stats/reset", handler.ResetSmallModelStatsHandler)

	srv := httptest.NewServer(e)
	defer srv.Close()

	doPatch := func(t *testing.T, body string) map[string]any {
		t.Helper()
		req, err := http.NewRequest(http.MethodPatch, srv.URL+"/api/v1/settings", strings.NewReader(body))
		if err != nil {
			t.Fatalf("new PATCH request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatalf("PATCH /settings: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("PATCH /settings status=%d, want 200", resp.StatusCode)
		}
		var out map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("decode PATCH response: %v", err)
		}
		return out
	}

	doGet := func(t *testing.T) map[string]any {
		t.Helper()
		resp, err := srv.Client().Get(srv.URL + "/api/v1/settings")
		if err != nil {
			t.Fatalf("GET /settings: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /settings status=%d, want 200", resp.StatusCode)
		}
		var out map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("decode GET response: %v", err)
		}
		return out
	}

	resetStats := func(t *testing.T) {
		t.Helper()
		resp, err := srv.Client().Post(srv.URL+"/api/v1/small-model/stats/reset", "application/json", strings.NewReader(`{}`))
		if err != nil {
			t.Fatalf("POST stats reset: %v", err)
		}
		_ = resp.Body.Close()
	}

	send := func(t *testing.T, convID string, payload map[string]any) SendMessageResponse {
		t.Helper()
		body, _ := json.Marshal(payload)
		resp, err := http.Post(srv.URL+"/api/v1/conversations/"+convID+"/messages", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("POST SendMessage: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("SendMessage status=%d, want 200", resp.StatusCode)
		}
		var out SendMessageResponse
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("decode SendMessageResponse: %v", err)
		}
		return out
	}

	conv, err := store.CreateConversation(context.Background(), "switches e2e")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	t.Run("patch/get covers all relevant keys", func(t *testing.T) {
		doPatch(t, `{
		  "small_model_enabled": true,
		  "small_model_route_short_qa_enabled": true,
		  "small_model_summary_enabled": true,
		  "small_model_context_compress_enabled": true,
		  "small_model_doc_extract_enabled": true,
		  "small_model_context_prune_enabled": true,
		  "small_model_media_intent_enabled": true,
		  "offline_ir_fallback_enabled": true,
		  "feature_intent_ir_enabled": true,
		  "context_compression_mode": "auto"
		}`)

		got := doGet(t)
		for _, k := range []string{
			"small_model_enabled",
			"small_model_route_short_qa_enabled",
			"small_model_summary_enabled",
			"small_model_context_compress_enabled",
			"small_model_doc_extract_enabled",
			"small_model_context_prune_enabled",
			"small_model_media_intent_enabled",
			"offline_ir_fallback_enabled",
			"feature_intent_ir_enabled",
			"context_compression_mode",
		} {
			if _, ok := got[k]; !ok {
				t.Fatalf("GET /settings missing key %q, got=%v", k, got)
			}
		}
	})

	t.Run("short-qa routes only when both switches are enabled and runtime is ready", func(t *testing.T) {
		// Reset stats to keep expectations stable.
		resetStats(t)

		// Route switch enabled, but global gate disabled => should NOT route.
		doPatch(t, `{"small_model_enabled":false,"small_model_route_short_qa_enabled":true}`)
		resp := send(t, conv.ID, map[string]any{
			"message":  "hi?",
			"provider": "capture",
			"model":    "capture-model",
		})
		if resp.Provider == "smallmodel" || resp.Content == "SM" {
			t.Fatalf("expected main model path when small_model_enabled=false, got provider=%q content=%q", resp.Provider, resp.Content)
		}

		// Global gate enabled, route switch disabled => should NOT route.
		doPatch(t, `{"small_model_enabled":true,"small_model_route_short_qa_enabled":false}`)
		resp = send(t, conv.ID, map[string]any{
			"message":  "hi?",
			"provider": "capture",
			"model":    "capture-model",
		})
		if resp.Provider == "smallmodel" || resp.Content == "SM" {
			t.Fatalf("expected main model path when small_model_route_short_qa_enabled=false, got provider=%q content=%q", resp.Provider, resp.Content)
		}

		// Both enabled + ready => should route to small model.
		doPatch(t, `{"small_model_enabled":true,"small_model_route_short_qa_enabled":true}`)
		resp = send(t, conv.ID, map[string]any{
			"message":  "hi?",
			"provider": "capture",
			"model":    "capture-model",
		})
		if resp.Provider != "smallmodel" || resp.Content != "SM" {
			t.Fatalf("expected small model response, got provider=%q content=%q", resp.Provider, resp.Content)
		}

		// Verify stats increment.
		statsResp, err := srv.Client().Get(srv.URL + "/api/v1/small-model/stats")
		if err != nil {
			t.Fatalf("GET stats: %v", err)
		}
		defer statsResp.Body.Close()
		var snap SmallModelStats
		if err := json.NewDecoder(statsResp.Body).Decode(&snap); err != nil {
			t.Fatalf("decode stats: %v", err)
		}
		if snap.ShortQARouteAttempts < 1 || snap.ShortQARouteSuccess < 1 {
			t.Fatalf("expected short_qa stats to increment, got attempts=%d success=%d", snap.ShortQARouteAttempts, snap.ShortQARouteSuccess)
		}
	})

	t.Run("image attachments never route to small-model vision reasoning", func(t *testing.T) {
		resetStats(t)

		// Enable global gate + short-qa route; do not set small_model_route_image_qa_enabled.
		doPatch(t, `{"small_model_enabled":true,"small_model_route_short_qa_enabled":true}`)

		resp := send(t, conv.ID, map[string]any{
			"message":    "what is this?",
			"provider":   "capture",
			"model":      "capture-model",
			"max_tokens": 64,
			"attachments": []map[string]any{
				{
					"type":      "image",
					"mime_type": "image/png",
					"data":      inlinePNGBase64ForChatTest(),
				},
			},
		})
		if resp.Provider == "smallmodel" || resp.Content == "SM" {
			t.Fatalf("expected image request to stay on main model path, got provider=%q content=%q", resp.Provider, resp.Content)
		}
	})

	t.Run("offline IR fallback is gated by offline_ir_fallback_enabled", func(t *testing.T) {
		conv2, err := store.CreateConversation(context.Background(), "ir fallback e2e")
		if err != nil {
			t.Fatalf("CreateConversation: %v", err)
		}
		_, err = store.AddMessage(context.Background(), conv2.ID, memory.Message{
			Role:    "assistant",
			Content: "FooBar is the internal codename for the release.",
		})
		if err != nil {
			t.Fatalf("AddMessage assistant: %v", err)
		}

		doPatch(t, `{"offline_ir_fallback_enabled":false}`)
		_, err = handler.runLocalIRFallback(context.Background(), conv2.ID, "FooBar", false)
		if err == nil {
			t.Fatalf("expected err when offline_ir_fallback_enabled=false")
		}

		doPatch(t, `{"offline_ir_fallback_enabled":true}`)
		out, err := handler.runLocalIRFallback(context.Background(), conv2.ID, "FooBar", false)
		if err != nil {
			t.Fatalf("expected IR fallback to succeed when enabled, err=%v", err)
		}
		if !strings.Contains(out, "IR-only fallback") || !strings.Contains(out, "FooBar") {
			t.Fatalf("unexpected IR fallback output: %q", out)
		}
	})

	t.Run("explicit image-qa disable overrides short-qa inheritance", func(t *testing.T) {
		resetStats(t)

		// Enable global + short-qa, but explicitly disable image-qa route.
		doPatch(t, `{"small_model_enabled":true,"small_model_route_short_qa_enabled":true,"small_model_route_image_qa_enabled":false}`)

		resp := send(t, conv.ID, map[string]any{
			"message":    "what is this?",
			"provider":   "capture",
			"model":      "capture-model",
			"max_tokens": 64,
			"attachments": []map[string]any{
				{
					"type":      "image",
					"mime_type": "image/png",
					"data":      inlinePNGBase64ForChatTest(),
				},
			},
		})
		if resp.Provider == "smallmodel" {
			t.Fatalf("expected image-qa route to be disabled, got provider=%q content=%q", resp.Provider, resp.Content)
		}
	})

	t.Run("IR-first takeover happens when short-qa routing fails and offline IR fallback is enabled", func(t *testing.T) {
		ready := true
		handler.SetSmallModelRuntime(&smallModelRuntimeMock{
			ready: &ready,
			err:   context.DeadlineExceeded,
		})

		conv4, err := store.CreateConversation(context.Background(), "ir-first takeover e2e")
		if err != nil {
			t.Fatalf("CreateConversation: %v", err)
		}
		_, err = store.AddMessage(context.Background(), conv4.ID, memory.Message{
			Role:    "assistant",
			Content: "FooBar is the internal codename for the release.",
		})
		if err != nil {
			t.Fatalf("AddMessage assistant: %v", err)
		}

		doPatch(t, `{"small_model_enabled":true,"small_model_route_short_qa_enabled":true,"offline_ir_fallback_enabled":true}`)

		resp := send(t, conv4.ID, map[string]any{
			"message":  "FooBar?",
			"provider": "capture",
			"model":    "capture-model",
		})
		if resp.Provider != "ir" || !strings.Contains(resp.Content, "IR-only fallback") {
			t.Fatalf("expected IR-first takeover response, got provider=%q content=%q", resp.Provider, resp.Content)
		}
	})

	t.Run("when offline IR fallback is disabled, short-qa routing failure falls back to main model", func(t *testing.T) {
		ready := true
		handler.SetSmallModelRuntime(&smallModelRuntimeMock{
			ready: &ready,
			err:   smallmodel.ErrCircuitOpen,
		})

		conv5, err := store.CreateConversation(context.Background(), "no ir takeover e2e")
		if err != nil {
			t.Fatalf("CreateConversation: %v", err)
		}

		doPatch(t, `{"small_model_enabled":true,"small_model_route_short_qa_enabled":true,"offline_ir_fallback_enabled":false}`)

		resp := send(t, conv5.ID, map[string]any{
			"message":  "FooBar?",
			"provider": "capture",
			"model":    "capture-model",
		})
		if resp.Provider == "ir" {
			t.Fatalf("expected no IR takeover when offline_ir_fallback_enabled=false, got provider=%q content=%q", resp.Provider, resp.Content)
		}
	})

	t.Run("small_model_summary_enabled enables small-model title generation", func(t *testing.T) {
		// Ensure short-qa doesn't consume the small-model call.
		doPatch(t, `{"small_model_enabled":true,"small_model_summary_enabled":true,"small_model_route_short_qa_enabled":false}`)

		sm := &smallModelRuntimeMock{
			respText: "MyTitle",
			calledCh: make(chan struct{}, 1),
		}
		handler.SetSmallModelRuntime(sm)

		// Use the default title placeholder so generateConversationTitle is allowed to overwrite it.
		conv3, err := store.CreateConversation(context.Background(), "New Conversation")
		if err != nil {
			t.Fatalf("CreateConversation: %v", err)
		}

		longMsg := strings.Repeat("this is a long message for title generation ", 5) + "end."

		// Title generation is wired for the streaming endpoint. For deterministic
		// e2e coverage here, call it directly.
		handler.generateConversationTitle(conv3.ID, "", longMsg, "ok", "en")

		select {
		case <-sm.calledCh:
			// ok
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for small-model title generation call")
		}

		convNow, err := store.GetConversation(context.Background(), conv3.ID)
		if err != nil || convNow == nil {
			t.Fatalf("GetConversation: %v", err)
		}
		if strings.TrimSpace(convNow.Title) != "MyTitle" {
			t.Fatalf("expected conversation title to be updated to MyTitle, got %q", convNow.Title)
		}
	})
}
