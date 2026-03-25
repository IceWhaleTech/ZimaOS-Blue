package proxybridge

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
)

// fakeProxyHandler simulates the proxy handler's behavior:
// 1. Reads ResolvedRoute from context and populates it (like executeOnProvider does)
// 2. Streams SSE chunks back (like copyResponse does)
type fakeProxyHandler struct {
	providerName string
	modelID      string
	chunks       []string // raw SSE "data: ..." lines
	failFirst    int      // number of initial failures (HTTP 502) before success
	callCount    int
	lastHeader   string
	lastLocale   string
}

func TestEnsureTimeout_DefaultChatIs10Minutes(t *testing.T) {
	ctx, cancel := ensureTimeout(context.Background(), defaultChatTimeout)
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected deadline to be set")
	}

	remaining := time.Until(deadline)
	if remaining > 10*time.Minute+time.Second || remaining < 10*time.Minute-time.Second {
		t.Fatalf("unexpected chat timeout window: %s", remaining)
	}
}

func TestEnsureTimeout_DefaultStreamIs10Minutes(t *testing.T) {
	ctx, cancel := ensureTimeout(context.Background(), defaultStreamTimeout)
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected deadline to be set")
	}

	remaining := time.Until(deadline)
	if remaining > 10*time.Minute+time.Second || remaining < 10*time.Minute-time.Second {
		t.Fatalf("unexpected stream timeout window: %s", remaining)
	}
}

func (f *fakeProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.callCount++
	f.lastHeader = r.Header.Get(proxy.DisableResponsesContinuationHeader)
	f.lastLocale = r.Header.Get("Accept-Language")

	if f.callCount <= f.failFirst {
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}

	// Simulate executeOnProvider writing to ResolvedRoute via context
	if rr := proxy.GetResolvedRouteFromContext(r.Context()); rr != nil {
		rr.Provider = f.providerName
		rr.Model = f.modelID
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Actual-Provider", f.providerName)
	w.Header().Set("X-Actual-Model", f.modelID)
	w.WriteHeader(http.StatusOK)

	flusher, _ := w.(http.Flusher)
	for _, chunk := range f.chunks {
		fmt.Fprintf(w, "data: %s\n\n", chunk)
		if flusher != nil {
			flusher.Flush()
		}
	}
}

func TestBridgeChat_PropagatesAcceptLanguageHeader(t *testing.T) {
	var gotLocale string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotLocale = r.Header.Get("Accept-Language")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"id":"1","model":"","choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`)
	})

	bridge := NewBridge(handler)
	ctx := proxy.WithLocale(context.Background(), "zh-CN")
	_, err := bridge.Chat(ctx, llm.ChatRequest{
		Model:    "auto",
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
	if gotLocale != "zh-CN" {
		t.Fatalf("expected Accept-Language zh-CN, got %q", gotLocale)
	}
}

func TestBridgeChat_PropagatesBackgroundTaskHeader(t *testing.T) {
	var gotBackground string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBackground = r.Header.Get(proxy.BackgroundTaskHeader)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"id":"1","model":"","choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`)
	})

	bridge := NewBridge(handler)
	ctx := proxy.WithBackgroundTask(context.Background())
	_, err := bridge.Chat(ctx, llm.ChatRequest{
		Model:    "auto",
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
	if gotBackground != "true" {
		t.Fatalf("expected %s header=true, got %q", proxy.BackgroundTaskHeader, gotBackground)
	}
}

func TestMarshalChatRequestNormalizesToolSchema(t *testing.T) {
	data, err := MarshalChatRequest(llm.ChatRequest{
		Model: "gpt-5",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "hi"},
		},
		Tools: []llm.Tool{{
			Name: "browser",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url": map[string]interface{}{"type": "string"},
				},
				"nullable": true,
			},
		}},
	})
	if err != nil {
		t.Fatalf("MarshalChatRequest() error = %v", err)
	}

	var req bridgeRequest
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("decode request: %v", err)
	}
	if len(req.Tools) != 1 {
		t.Fatalf("len(req.Tools) = %d, want 1", len(req.Tools))
	}
	params := req.Tools[0].Function.Parameters
	if got, ok := params["additionalProperties"].(bool); !ok || got {
		t.Fatalf("additionalProperties = %v, want false", params["additionalProperties"])
	}
	if _, ok := params["nullable"]; ok {
		t.Fatalf("nullable should be removed from marshalled schema, got %v", params["nullable"])
	}
}

func TestMarshalChatRequestNormalizesTypedCompositeToolSchema(t *testing.T) {
	data, err := MarshalChatRequest(llm.ChatRequest{
		Model: "gpt-5",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "hi"},
		},
		Tools: []llm.Tool{{
			Name: "deep_research",
			Parameters: map[string]interface{}{
				"type": "object",
				"anyOf": []map[string]interface{}{
					{"required": []string{"query"}},
					{"required": []string{"job_id"}},
				},
			},
		}},
	})
	if err != nil {
		t.Fatalf("MarshalChatRequest() error = %v", err)
	}

	var req bridgeRequest
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("decode request: %v", err)
	}
	if len(req.Tools) != 1 {
		t.Fatalf("len(req.Tools) = %d, want 1", len(req.Tools))
	}
	anyOf, ok := req.Tools[0].Function.Parameters["anyOf"].([]interface{})
	if !ok {
		t.Fatalf("parameters.anyOf type = %T, want []interface{}", req.Tools[0].Function.Parameters["anyOf"])
	}
	if len(anyOf) != 2 {
		t.Fatalf("len(parameters.anyOf) = %d, want 2", len(anyOf))
	}
}

func TestMarshalChatRequest_PreservesEmptyRequiredOnNestedObjectSchemas(t *testing.T) {
	data, err := MarshalChatRequest(llm.ChatRequest{
		Model: "gpt-5",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "hi"},
		},
		Tools: []llm.Tool{{
			Name: "ask",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"questions": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"question": map[string]interface{}{"type": "string"},
								"options": map[string]interface{}{
									"type": "array",
									"items": map[string]interface{}{
										"oneOf": []interface{}{
											map[string]interface{}{"type": "string"},
											map[string]interface{}{
												"type": "object",
												"properties": map[string]interface{}{
													"label": map[string]interface{}{"type": "string"},
												},
											},
										},
									},
								},
							},
							"required": []string{"question"},
						},
					},
				},
			},
		}},
	})
	if err != nil {
		t.Fatalf("MarshalChatRequest() error = %v", err)
	}

	var req bridgeRequest
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("decode request: %v", err)
	}
	if len(req.Tools) != 1 {
		t.Fatalf("len(req.Tools) = %d, want 1", len(req.Tools))
	}

	topRequired, ok := req.Tools[0].Function.Parameters["required"].([]interface{})
	if !ok {
		t.Fatalf("top-level required type = %T, want []interface{}", req.Tools[0].Function.Parameters["required"])
	}
	if len(topRequired) != 0 {
		t.Fatalf("top-level required = %v, want empty array", topRequired)
	}

	props := req.Tools[0].Function.Parameters["properties"].(map[string]interface{})
	questions := props["questions"].(map[string]interface{})
	items := questions["items"].(map[string]interface{})
	options := items["properties"].(map[string]interface{})["options"].(map[string]interface{})
	oneOf := options["items"].(map[string]interface{})["oneOf"].([]interface{})
	nestedObject := oneOf[1].(map[string]interface{})
	nestedRequired, ok := nestedObject["required"].([]interface{})
	if !ok {
		t.Fatalf("nested required type = %T, want []interface{}", nestedObject["required"])
	}
	if len(nestedRequired) != 0 {
		t.Fatalf("nested required = %v, want empty array", nestedRequired)
	}
}

func TestMarshalChatRequest_EmptyAssistantContentStaysString(t *testing.T) {
	data, err := MarshalChatRequest(llm.ChatRequest{
		Model: "gpt-5",
		Messages: []llm.Message{
			{Role: llm.RoleAssistant, Content: ""},
			{Role: llm.RoleUser, Content: "继续"},
		},
	})
	if err != nil {
		t.Fatalf("MarshalChatRequest() error = %v", err)
	}
	if strings.Contains(string(data), `"content":null`) {
		t.Fatalf("request contains null content: %s", data)
	}

	var req bridgeRequest
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("decode request: %v", err)
	}
	if len(req.Messages) != 2 {
		t.Fatalf("len(req.Messages) = %d, want 2", len(req.Messages))
	}
	content, ok := req.Messages[0].Content.(string)
	if !ok {
		t.Fatalf("assistant content type = %T, want string", req.Messages[0].Content)
	}
	if content != "" {
		t.Fatalf("assistant content = %q, want empty string", content)
	}
}

func TestBridgeChatStream_PropagatesDisableResponsesContinuationHeader(t *testing.T) {
	handler := &fakeProxyHandler{
		providerName: "OpenAI",
		modelID:      "gpt-4o",
		chunks: []string{
			`{"id":"1","choices":[{"delta":{"content":"ok"},"finish_reason":"stop"}],"model":""}`,
		},
	}
	bridge := NewBridge(handler)

	ctx := proxy.WithDisableResponsesContinuation(context.Background())
	err := bridge.ChatStream(ctx, llm.ChatRequest{
		Model:    "auto",
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	}, func(chunk llm.StreamChunk) error { return nil })
	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}
	if handler.lastHeader != "1" {
		t.Fatalf("expected %s header=1, got %q", proxy.DisableResponsesContinuationHeader, handler.lastHeader)
	}
}

func TestProxyErrorIsOverloaded_IgnoresWrappedBuildFailures(t *testing.T) {
	err := &ProxyError{
		StatusCode: http.StatusBadGateway,
		Body:       `upstream 500: {"error":{"type":"overloaded_error","message":"构建请求失败"},"type":"error"}`,
	}
	if err.IsOverloaded() {
		t.Fatal("expected wrapped request-build failure to avoid overloaded classification")
	}

	overloaded := &ProxyError{
		StatusCode: http.StatusBadGateway,
		Body:       `upstream 500: {"error":{"type":"overloaded_error","message":"Overloaded"},"type":"error"}`,
	}
	if !overloaded.IsOverloaded() {
		t.Fatal("expected plain overloaded response to remain overloaded")
	}
}

func TestBridgeChatStream_PropagatesAcceptLanguageHeader(t *testing.T) {
	handler := &fakeProxyHandler{
		providerName: "OpenAI",
		modelID:      "gpt-4o",
		chunks: []string{
			`{"id":"1","choices":[{"delta":{"content":"ok"},"finish_reason":"stop"}],"model":""}`,
		},
	}
	bridge := NewBridge(handler)

	ctx := proxy.WithLocale(context.Background(), "ja-JP")
	err := bridge.ChatStream(ctx, llm.ChatRequest{
		Model:    "auto",
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	}, func(chunk llm.StreamChunk) error { return nil })
	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}
	if handler.lastLocale != "ja-JP" {
		t.Fatalf("expected Accept-Language ja-JP, got %q", handler.lastLocale)
	}
}

// TestBridgeResolvedRoute_BasicPropagation verifies that the bridge propagates
// provider/model from the ResolvedRoute context into StreamChunk fields.
func TestBridgeResolvedRoute_BasicPropagation(t *testing.T) {
	handler := &fakeProxyHandler{
		providerName: "OpenAI",
		modelID:      "gpt-4o",
		chunks: []string{
			`{"id":"1","choices":[{"delta":{"content":"Hello"},"finish_reason":null}],"model":""}`,
			`{"id":"1","choices":[{"delta":{"content":" world"},"finish_reason":"stop"}],"model":""}`,
		},
	}

	bridge := NewBridge(handler)

	var capturedProvider, capturedModel string
	var chunkCount int

	err := bridge.ChatStream(context.Background(), llm.ChatRequest{
		Model:    "auto",
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	}, func(chunk llm.StreamChunk) error {
		chunkCount++
		// Capture from first chunk
		if chunkCount == 1 {
			capturedProvider = chunk.Provider
			capturedModel = chunk.Model
		}
		return nil
	})

	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}
	if chunkCount != 2 {
		t.Fatalf("Expected 2 chunks, got %d", chunkCount)
	}
	if capturedProvider != "OpenAI" {
		t.Errorf("Expected provider 'OpenAI', got %q", capturedProvider)
	}
	if capturedModel != "gpt-4o" {
		t.Errorf("Expected model 'gpt-4o', got %q", capturedModel)
	}
}

// TestBridgeResolvedRoute_ResolvedModelTakesPrecedence verifies that we always
// keep the routed model for stream consistency across tool rounds.
func TestBridgeResolvedRoute_ResolvedModelTakesPrecedence(t *testing.T) {
	handler := &fakeProxyHandler{
		providerName: "Anthropic",
		modelID:      "claude-sonnet-4-5",
		chunks: []string{
			// Upstream returns model in the SSE chunk itself
			`{"id":"1","choices":[{"delta":{"content":"Hi"},"finish_reason":"stop"}],"model":"claude-sonnet-4-5-20250514"}`,
		},
	}

	bridge := NewBridge(handler)

	var capturedModel string
	err := bridge.ChatStream(context.Background(), llm.ChatRequest{
		Model:    "auto",
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	}, func(chunk llm.StreamChunk) error {
		capturedModel = chunk.Model
		return nil
	})

	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}
	// Resolved route model should take precedence.
	if capturedModel != "claude-sonnet-4-5" {
		t.Errorf("Expected resolved model 'claude-sonnet-4-5', got %q", capturedModel)
	}
}

// TestBridgeResolvedRoute_AllChunksGetProvider verifies that every chunk
// (not just the first) gets the provider/model injected.
func TestBridgeResolvedRoute_AllChunksGetProvider(t *testing.T) {
	handler := &fakeProxyHandler{
		providerName: "MyRelay",
		modelID:      "deepseek-r1",
		chunks: []string{
			`{"id":"1","choices":[{"delta":{"content":"A"},"finish_reason":null}],"model":""}`,
			`{"id":"1","choices":[{"delta":{"content":"B"},"finish_reason":null}],"model":""}`,
			`{"id":"1","choices":[{"delta":{"content":"C"},"finish_reason":"stop"}],"model":""}`,
		},
	}

	bridge := NewBridge(handler)

	var providers, models []string
	err := bridge.ChatStream(context.Background(), llm.ChatRequest{
		Model:    "auto",
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	}, func(chunk llm.StreamChunk) error {
		providers = append(providers, chunk.Provider)
		models = append(models, chunk.Model)
		return nil
	})

	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}
	for i, p := range providers {
		if p != "MyRelay" {
			t.Errorf("Chunk %d: expected provider 'MyRelay', got %q", i, p)
		}
	}
	for i, m := range models {
		if m != "deepseek-r1" {
			t.Errorf("Chunk %d: expected model 'deepseek-r1', got %q", i, m)
		}
	}
}

// TestBridgeResolvedRoute_HandlerError verifies that when the handler returns
// an error (e.g. 502), the ResolvedRoute stays empty and the error propagates.
func TestBridgeResolvedRoute_HandlerError(t *testing.T) {
	handler := &fakeProxyHandler{
		providerName: "FailProvider",
		modelID:      "fail-model",
		failFirst:    999, // always fail
	}

	bridge := NewBridge(handler)

	var callbackCalled bool
	err := bridge.ChatStream(context.Background(), llm.ChatRequest{
		Model:    "auto",
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	}, func(chunk llm.StreamChunk) error {
		callbackCalled = true
		return nil
	})

	if err == nil {
		t.Fatal("Expected error from handler, got nil")
	}
	if !strings.Contains(err.Error(), "502") {
		t.Errorf("Expected 502 error, got: %v", err)
	}
	if callbackCalled {
		t.Error("Callback should not be called when handler returns error")
	}
}

// TestBridgeResolvedRoute_SimulatedFailoverContext simulates the scenario where
// the chat handler calls bridge.ChatStream, the first provider fails, and the
// router retries with a different provider. The ResolvedRoute should reflect
// the provider that actually succeeded.
//
// In the real system, failover happens inside ProxyHandler.ServeHTTP via
// RouteWithFallback. The bridge sees a single ServeHTTP call that internally
// tries multiple providers. The ResolvedRoute is written by executeOnProvider
// which only runs on success.
func TestBridgeResolvedRoute_SimulatedFailoverContext(t *testing.T) {
	// This handler simulates a proxy that internally failed over:
	// - The first provider ("BadProvider") failed internally
	// - The second provider ("GoodProvider") succeeded
	// - The handler writes the successful provider to ResolvedRoute
	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		// Simulate: proxy handler internally tried BadProvider, failed,
		// then tried GoodProvider which succeeded.
		// Only the successful provider is written to ResolvedRoute.
		if rr := proxy.GetResolvedRouteFromContext(r.Context()); rr != nil {
			rr.Provider = "GoodProvider"
			rr.Model = "good-model-v2"
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "data: %s\n\n",
			`{"id":"1","choices":[{"delta":{"content":"OK"},"finish_reason":"stop"}],"model":""}`)
	})

	bridge := NewBridge(handler)

	var capturedProvider, capturedModel string
	err := bridge.ChatStream(context.Background(), llm.ChatRequest{
		Model:    "auto",
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	}, func(chunk llm.StreamChunk) error {
		capturedProvider = chunk.Provider
		capturedModel = chunk.Model
		return nil
	})

	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}
	if capturedProvider != "GoodProvider" {
		t.Errorf("Expected 'GoodProvider' after failover, got %q", capturedProvider)
	}
	if capturedModel != "good-model-v2" {
		t.Errorf("Expected 'good-model-v2' after failover, got %q", capturedModel)
	}
}

// TestBridgeResolvedRoute_AutoModelResolved verifies that when the request model
// is "auto", the bridge still propagates the actual resolved provider/model.
func TestBridgeResolvedRoute_AutoModelResolved(t *testing.T) {
	handler := &fakeProxyHandler{
		providerName: "CloudProvider",
		modelID:      "gpt-4o-mini",
		chunks: []string{
			`{"id":"1","choices":[{"delta":{"content":"Hi"},"finish_reason":"stop"}],"model":""}`,
		},
	}

	bridge := NewBridge(handler)

	var capturedProvider, capturedModel string
	err := bridge.ChatStream(context.Background(), llm.ChatRequest{
		Model:    "auto",
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	}, func(chunk llm.StreamChunk) error {
		capturedProvider = chunk.Provider
		capturedModel = chunk.Model
		return nil
	})

	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}
	if capturedProvider != "CloudProvider" {
		t.Errorf("auto model: expected provider 'CloudProvider', got %q", capturedProvider)
	}
	if capturedModel != "gpt-4o-mini" {
		t.Errorf("auto model: expected model 'gpt-4o-mini', got %q", capturedModel)
	}
}

// TestBridgeResolvedRoute_CloudModelResolved verifies that when the request model
// is "cloud" (a routing hint), the bridge propagates the actual resolved values.
func TestBridgeResolvedRoute_CloudModelResolved(t *testing.T) {
	handler := &fakeProxyHandler{
		providerName: "AzureOpenAI",
		modelID:      "gpt-4-turbo",
		chunks: []string{
			`{"id":"1","choices":[{"delta":{"content":"Hello"},"finish_reason":"stop"}],"model":""}`,
		},
	}

	bridge := NewBridge(handler)

	var capturedProvider, capturedModel string
	err := bridge.ChatStream(context.Background(), llm.ChatRequest{
		Model:    "cloud",
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	}, func(chunk llm.StreamChunk) error {
		capturedProvider = chunk.Provider
		capturedModel = chunk.Model
		return nil
	})

	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}
	if capturedProvider != "AzureOpenAI" {
		t.Errorf("cloud model: expected provider 'AzureOpenAI', got %q", capturedProvider)
	}
	if capturedModel != "gpt-4-turbo" {
		t.Errorf("cloud model: expected model 'gpt-4-turbo', got %q", capturedModel)
	}
}

// TestBridgeResolvedRoute_LocalModelResolved verifies that when the request model
// is "local" (a routing hint), the bridge propagates the actual resolved values.
func TestBridgeResolvedRoute_LocalModelResolved(t *testing.T) {
	handler := &fakeProxyHandler{
		providerName: "Ollama",
		modelID:      "llama3:8b",
		chunks: []string{
			`{"id":"1","choices":[{"delta":{"content":"Hey"},"finish_reason":"stop"}],"model":""}`,
		},
	}

	bridge := NewBridge(handler)

	var capturedProvider, capturedModel string
	err := bridge.ChatStream(context.Background(), llm.ChatRequest{
		Model:    "local",
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	}, func(chunk llm.StreamChunk) error {
		capturedProvider = chunk.Provider
		capturedModel = chunk.Model
		return nil
	})

	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}
	if capturedProvider != "Ollama" {
		t.Errorf("local model: expected provider 'Ollama', got %q", capturedProvider)
	}
	if capturedModel != "llama3:8b" {
		t.Errorf("local model: expected model 'llama3:8b', got %q", capturedModel)
	}
}

// TestBridgeChat_ResolvedRoute verifies that the non-streaming Chat method
// also propagates resolved provider/model into the response.
func TestBridgeChat_ResolvedRoute(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rr := proxy.GetResolvedRouteFromContext(r.Context()); rr != nil {
			rr.Provider = "Anthropic"
			rr.Model = "claude-sonnet-4-5"
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"id":"1","model":"","choices":[{"message":{"role":"assistant","content":"Hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`)
	})

	bridge := NewBridge(handler)
	resp, err := bridge.Chat(context.Background(), llm.ChatRequest{
		Model:    "auto",
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})

	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
	if resp.Provider != "Anthropic" {
		t.Errorf("Expected provider 'Anthropic', got %q", resp.Provider)
	}
	if resp.Model != "claude-sonnet-4-5" {
		t.Errorf("Expected model 'claude-sonnet-4-5', got %q", resp.Model)
	}
}

// TestBridgeChat_ResolvedModelPreserved verifies that chat responses preserve
// resolved routing model rather than upstream model aliases/versions.
func TestBridgeChat_ResolvedModelPreserved(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rr := proxy.GetResolvedRouteFromContext(r.Context()); rr != nil {
			rr.Provider = "OpenAI"
			rr.Model = "gpt-4o"
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Upstream returns a specific model version
		fmt.Fprint(w, `{"id":"1","model":"gpt-4o-2024-08-06","choices":[{"message":{"role":"assistant","content":"Hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`)
	})

	bridge := NewBridge(handler)
	resp, err := bridge.Chat(context.Background(), llm.ChatRequest{
		Model:    "auto",
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})

	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
	if resp.Provider != "OpenAI" {
		t.Errorf("Expected provider 'OpenAI', got %q", resp.Provider)
	}
	// Resolved route model should be preserved for consistent routing.
	if resp.Model != "gpt-4o" {
		t.Errorf("Expected resolved model 'gpt-4o', got %q", resp.Model)
	}
}

func TestBridgeChat_ParseErrorIncludesRawResponseBody(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "【invalid-non-json-payload】")
	})

	bridge := NewBridge(handler)
	_, err := bridge.Chat(context.Background(), llm.ChatRequest{
		Model:    "gpt-5.3-codex-spark",
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
	if !strings.Contains(err.Error(), "raw response:") {
		t.Fatalf("expected raw response marker in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "【invalid-non-json-payload】") {
		t.Fatalf("expected raw response body in error, got: %v", err)
	}
}

func TestBridgeChat_DecodesGzipResponseBody(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		_, _ = io.WriteString(zw, `{"id":"1","model":"gpt-5.3-codex-spark","choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`)
		_ = zw.Close()

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(buf.Bytes())
	})

	bridge := NewBridge(handler)
	resp, err := bridge.Chat(context.Background(), llm.ChatRequest{
		Model:    "gpt-5.3-codex-spark",
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
	if resp == nil || strings.TrimSpace(resp.Message.Content) != "ok" {
		t.Fatalf("unexpected response: %#v", resp)
	}
}

func TestBridgeChatStream_AcceptsSSEEventLines(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rr := proxy.GetResolvedRouteFromContext(r.Context()); rr != nil {
			rr.Provider = "OpenAI"
			rr.Model = "o3"
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		fmt.Fprint(w, "event: response.output_text.delta\n")
		fmt.Fprint(w, `data: {"type":"response.output_text.delta","response_id":"resp_1","delta":"Hel"}`+"\n\n")
		fmt.Fprint(w, "event: response.output_text.delta\n")
		fmt.Fprint(w, `data: {"type":"response.output_text.delta","response_id":"resp_1","delta":"lo"}`+"\n\n")
		fmt.Fprint(w, "event: response.completed\n")
		fmt.Fprint(w, `data: {"type":"response.completed","response":{"id":"resp_1","model":"o3","usage":{"input_tokens":10,"output_tokens":2,"total_tokens":12}}}`+"\n\n")
	})

	bridge := NewBridge(handler)
	var out strings.Builder
	var done bool
	var usage *llm.Usage

	err := bridge.ChatStream(context.Background(), llm.ChatRequest{
		Model:    "auto",
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	}, func(chunk llm.StreamChunk) error {
		out.WriteString(chunk.Delta)
		if chunk.Usage != nil {
			usage = chunk.Usage
		}
		if chunk.Done {
			done = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}
	if got := out.String(); got != "Hello" {
		t.Fatalf("expected merged delta 'Hello', got %q", got)
	}
	if !done {
		t.Fatal("expected done chunk")
	}
	if usage == nil || usage.TotalTokens != 12 {
		t.Fatalf("expected usage with total_tokens=12, got %#v", usage)
	}
}

func TestBridgeChatStream_CompletedCarriesFinalText(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rr := proxy.GetResolvedRouteFromContext(r.Context()); rr != nil {
			rr.Provider = "OpenAI"
			rr.Model = "o3"
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		fmt.Fprint(w, "event: response.completed\n")
		fmt.Fprint(w, `data: {"type":"response.completed","response":{"id":"resp_short","model":"o3","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"A"}]}]}}`+"\n\n")
	})

	bridge := NewBridge(handler)
	var out strings.Builder

	err := bridge.ChatStream(context.Background(), llm.ChatRequest{
		Model:    "auto",
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	}, func(chunk llm.StreamChunk) error {
		out.WriteString(chunk.Delta)
		return nil
	})
	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}
	if got := out.String(); got != "A" {
		t.Fatalf("expected output %q, got %q", "A", got)
	}
}

func TestBridgeChatStream_CompletedSnapshotDoesNotDuplicateDelta(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rr := proxy.GetResolvedRouteFromContext(r.Context()); rr != nil {
			rr.Provider = "OpenAI"
			rr.Model = "o3"
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		fmt.Fprint(w, "event: response.output_text.delta\n")
		fmt.Fprint(w, `data: {"type":"response.output_text.delta","response_id":"resp_dup","delta":"Hel"}`+"\n\n")
		fmt.Fprint(w, "event: response.completed\n")
		fmt.Fprint(w, `data: {"type":"response.completed","response":{"id":"resp_dup","model":"o3","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Hello"}]}]}}`+"\n\n")
	})

	bridge := NewBridge(handler)
	var out strings.Builder

	err := bridge.ChatStream(context.Background(), llm.ChatRequest{
		Model:    "auto",
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	}, func(chunk llm.StreamChunk) error {
		out.WriteString(chunk.Delta)
		return nil
	})
	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}
	if got := out.String(); got != "Hello" {
		t.Fatalf("expected output %q, got %q", "Hello", got)
	}
}

func TestBridgeChatStream_SynthesizesDoneWhenTerminalMarkerMissing(t *testing.T) {
	handler := &fakeProxyHandler{
		providerName: "OpenAI",
		modelID:      "o3",
		chunks: []string{
			`{"id":"1","choices":[{"delta":{"content":"Hello"},"finish_reason":null}],"model":""}`,
		},
	}

	bridge := NewBridge(handler)
	var out strings.Builder
	var gotDone bool
	var doneProvider, doneModel string

	err := bridge.ChatStream(context.Background(), llm.ChatRequest{
		Model:    "auto",
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	}, func(chunk llm.StreamChunk) error {
		out.WriteString(chunk.Delta)
		if chunk.Done {
			gotDone = true
			doneProvider = chunk.Provider
			doneModel = chunk.Model
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}
	if got := out.String(); got != "Hello" {
		t.Fatalf("expected output %q, got %q", "Hello", got)
	}
	if !gotDone {
		t.Fatal("expected synthesized done chunk")
	}
	if doneProvider != "OpenAI" {
		t.Fatalf("expected synthesized done provider %q, got %q", "OpenAI", doneProvider)
	}
	if doneModel != "o3" {
		t.Fatalf("expected synthesized done model %q, got %q", "o3", doneModel)
	}
}

func TestBridgeChatStream_ZeroChunksReturnsError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rr := proxy.GetResolvedRouteFromContext(r.Context()); rr != nil {
			rr.Provider = "OpenAI"
			rr.Model = "gpt-5"
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		// Intentionally no SSE data frames.
	})

	bridge := NewBridge(handler)
	err := bridge.ChatStream(context.Background(), llm.ChatRequest{
		Model:    "auto",
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	}, func(chunk llm.StreamChunk) error { return nil })
	if err == nil {
		t.Fatal("expected zero-chunk stream error, got nil")
	}
	if !strings.Contains(err.Error(), "502") {
		t.Fatalf("expected 502-style proxy error, got: %v", err)
	}
}

func TestBridgeChatStream_MetadataEventCountsAsProgressChunk(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "event: response.created\n")
		fmt.Fprint(w, `data: {"type":"response.created","response":{"id":"resp_meta","model":"o3"}}`+"\n\n")
	})

	bridge := NewBridge(handler)
	var got []llm.StreamChunk
	err := bridge.ChatStream(context.Background(), llm.ChatRequest{
		Model:    "auto",
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	}, func(chunk llm.StreamChunk) error {
		got = append(got, chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("expected metadata progress stream to succeed, got: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("callback chunks = %d, want 1", len(got))
	}
	if got[0].Progress != "response.created" {
		t.Fatalf("progress = %q, want %q", got[0].Progress, "response.created")
	}
}

func TestBridgeChatStream_NonSSEBodyReturnsErrorBody(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"error":"not sse"}`)
	})

	bridge := NewBridge(handler)
	err := bridge.ChatStream(context.Background(), llm.ChatRequest{
		Model:    "auto",
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	}, func(chunk llm.StreamChunk) error { return nil })
	if err == nil {
		t.Fatal("expected malformed stream error, got nil")
	}
	if !strings.Contains(err.Error(), "not sse") {
		t.Fatalf("expected error body to include non-SSE payload, got: %v", err)
	}
}
