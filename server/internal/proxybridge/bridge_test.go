package proxybridge

import (
	"context"
	"fmt"
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
}

func TestEnsureTimeout_DefaultIs30Seconds(t *testing.T) {
	ctx, cancel := ensureTimeout(context.Background())
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected deadline to be set")
	}

	remaining := time.Until(deadline)
	if remaining > 31*time.Second || remaining < 29*time.Second {
		t.Fatalf("unexpected timeout window: %s", remaining)
	}
}

func (f *fakeProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.callCount++
	f.lastHeader = r.Header.Get(proxy.DisableResponsesContinuationHeader)

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

// TestBridgeResolvedRoute_UpstreamModelTakesPrecedence verifies that when the
// upstream SSE chunk already contains a model field, it takes precedence over
// the resolved route (the resolved route only fills empty fields).
func TestBridgeResolvedRoute_UpstreamModelTakesPrecedence(t *testing.T) {
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
	// Upstream model should take precedence
	if capturedModel != "claude-sonnet-4-5-20250514" {
		t.Errorf("Expected upstream model 'claude-sonnet-4-5-20250514', got %q", capturedModel)
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

// TestBridgeChat_UpstreamModelPreserved verifies that when the upstream response
// already contains a model field, it takes precedence over the resolved route.
func TestBridgeChat_UpstreamModelPreserved(t *testing.T) {
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
	// Upstream model should be preserved (not overwritten by resolved route)
	if resp.Model != "gpt-4o-2024-08-06" {
		t.Errorf("Expected upstream model 'gpt-4o-2024-08-06', got %q", resp.Model)
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
