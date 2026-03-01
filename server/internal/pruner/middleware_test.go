package pruner

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// mockBackend is a test double for the Backend interface.
type mockBackend struct {
	pruneFunc  func(ctx context.Context, req PruneRequest) (*PruneResponse, error)
	healthFunc func(ctx context.Context) error
}

func (m *mockBackend) Prune(ctx context.Context, req PruneRequest) (*PruneResponse, error) {
	if m.pruneFunc != nil {
		return m.pruneFunc(ctx, req)
	}
	return &PruneResponse{
		PrunedCode:     req.Code,
		OriginalTokens: 100,
		PrunedTokens:   50,
	}, nil
}

func (m *mockBackend) Health(ctx context.Context) error {
	if m.healthFunc != nil {
		return m.healthFunc(ctx)
	}
	return nil
}

func (m *mockBackend) Close() error { return nil }

func TestMiddleware_ProcessRequest_Disabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = false
	mw := NewMiddleware(&mockBackend{}, cfg, NewStats())

	body := []byte(`{"messages":[{"role":"tool","content":"code"}]}`)
	result, err := mw.ProcessRequest(context.Background(), body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result) != string(body) {
		t.Error("expected body unchanged when disabled")
	}
}

func TestWithPrunerDisabled(t *testing.T) {
	ctx := context.Background()
	if IsPrunerDisabled(ctx) {
		t.Fatal("expected default context not disabled")
	}
	ctx = WithPrunerDisabled(ctx, true)
	if !IsPrunerDisabled(ctx) {
		t.Fatal("expected disabled context")
	}
}

func TestMiddleware_ProcessRequest_SkipsNonToolMessages(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.MinLines = 5

	called := false
	backend := &mockBackend{
		pruneFunc: func(ctx context.Context, req PruneRequest) (*PruneResponse, error) {
			called = true
			return &PruneResponse{PrunedCode: req.Code}, nil
		},
	}
	mw := NewMiddleware(backend, cfg, NewStats())

	body := []byte(`{"messages":[{"role":"user","content":"hello"},{"role":"assistant","content":"hi"}]}`)
	_, err := mw.ProcessRequest(context.Background(), body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("backend should not be called for non-tool messages")
	}
}

func TestMiddleware_ProcessRequest_PrunesToolMessage(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.MinLines = 5

	longCode := strings.Repeat("package main\nimport \"fmt\"\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n", 5)

	backend := &mockBackend{
		pruneFunc: func(ctx context.Context, req PruneRequest) (*PruneResponse, error) {
			return &PruneResponse{
				PrunedCode:     "(pruned)",
				OriginalTokens: 200,
				PrunedTokens:   50,
			}, nil
		},
	}
	stats := NewStats()
	mw := NewMiddleware(backend, cfg, stats)

	msg := map[string]interface{}{
		"messages": []map[string]interface{}{
			{"role": "user", "content": "find the bug"},
			{"role": "tool", "content": longCode, "tool_call_id": "call_1"},
		},
	}
	body, _ := json.Marshal(msg)

	result, err := mw.ProcessRequest(context.Background(), body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]json.RawMessage
	json.Unmarshal(result, &parsed)
	var messages []openaiMessage
	json.Unmarshal(parsed["messages"], &messages)

	if messages[1].Content != "(pruned)" {
		t.Errorf("expected pruned content, got %s", messages[1].Content)
	}

	snap := stats.Snapshot()
	if snap.PrunedRequests != 1 {
		t.Errorf("expected 1 pruned request, got %d", snap.PrunedRequests)
	}
	if snap.TokensSaved != 150 {
		t.Errorf("expected 150 tokens saved, got %d", snap.TokensSaved)
	}
}

func TestMiddleware_ProcessRequest_UsesContentField(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.MinLines = 5

	longCode := strings.Repeat("package main\nimport \"fmt\"\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n", 5)

	var gotReq PruneRequest
	backend := &mockBackend{
		pruneFunc: func(ctx context.Context, req PruneRequest) (*PruneResponse, error) {
			gotReq = req
			return &PruneResponse{
				PrunedContent:  "(pruned-content)",
				PrunedCode:     "(pruned-content)",
				OriginalTokens: 200,
				PrunedTokens:   50,
			}, nil
		},
	}
	mw := NewMiddleware(backend, cfg, NewStats())

	msg := map[string]interface{}{
		"messages": []map[string]interface{}{
			{"role": "user", "content": "find the bug"},
			{"role": "tool", "content": longCode, "tool_call_id": "call_1"},
		},
	}
	body, _ := json.Marshal(msg)

	result, err := mw.ProcessRequest(context.Background(), body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should use Content field, not Code
	if gotReq.Content == "" {
		t.Error("expected Content field to be set in PruneRequest")
	}

	// Output should use PrunedContent
	var parsed map[string]json.RawMessage
	json.Unmarshal(result, &parsed)
	var messages []openaiMessage
	json.Unmarshal(parsed["messages"], &messages)

	if messages[1].Content != "(pruned-content)" {
		t.Errorf("expected pruned content from PrunedContent field, got %s", messages[1].Content)
	}
}

func TestMiddleware_ProcessRequest_PrunesLongUserMessage(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.MinLines = 5

	longDoc := strings.Repeat("## Section\n\nThis is a paragraph about something.\n\n", 10)

	var gotReq PruneRequest
	backend := &mockBackend{
		pruneFunc: func(ctx context.Context, req PruneRequest) (*PruneResponse, error) {
			gotReq = req
			return &PruneResponse{
				PrunedContent:  "(pruned-doc)",
				PrunedCode:     "(pruned-doc)",
				OriginalTokens: 300,
				PrunedTokens:   100,
			}, nil
		},
	}
	mw := NewMiddleware(backend, cfg, NewStats())

	msg := map[string]interface{}{
		"messages": []map[string]interface{}{
			{"role": "user", "content": longDoc},
		},
	}
	body, _ := json.Marshal(msg)

	result, err := mw.ProcessRequest(context.Background(), body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have been called for long user message
	if gotReq.Content == "" {
		t.Error("expected backend to be called for long user message")
	}

	// For non-tool messages, query should be self-derived (first N chars)
	if gotReq.Query == "" {
		t.Error("expected self-query for user message")
	}

	var parsed map[string]json.RawMessage
	json.Unmarshal(result, &parsed)
	var messages []openaiMessage
	json.Unmarshal(parsed["messages"], &messages)

	if messages[0].Content != "(pruned-doc)" {
		t.Errorf("expected pruned user message, got %s", messages[0].Content)
	}
}

func TestMiddleware_ProcessRequest_SkipsShortUserMessage(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.MinLines = 5

	called := false
	backend := &mockBackend{
		pruneFunc: func(ctx context.Context, req PruneRequest) (*PruneResponse, error) {
			called = true
			return &PruneResponse{PrunedContent: req.GetContent()}, nil
		},
	}
	mw := NewMiddleware(backend, cfg, NewStats())

	body := []byte(`{"messages":[{"role":"user","content":"hello world"}]}`)
	_, err := mw.ProcessRequest(context.Background(), body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("backend should not be called for short user messages")
	}
}

func TestMiddleware_ProcessRequest_SelfQueryTruncation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.MinLines = 5

	// Build a long doc where the first 500 chars are about authentication
	longDoc := "## Authentication\n\n" + strings.Repeat("The auth system handles login and session tokens. ", 20) + "\n\n" +
		strings.Repeat("## Other Section\n\nUnrelated content here.\n\n", 10)

	var gotQuery string
	backend := &mockBackend{
		pruneFunc: func(ctx context.Context, req PruneRequest) (*PruneResponse, error) {
			gotQuery = req.Query
			return &PruneResponse{
				PrunedContent:  "(pruned)",
				PrunedCode:     "(pruned)",
				OriginalTokens: 200,
				PrunedTokens:   100,
			}, nil
		},
	}
	mw := NewMiddleware(backend, cfg, NewStats())

	msg := map[string]interface{}{
		"messages": []map[string]interface{}{
			{"role": "user", "content": longDoc},
		},
	}
	body, _ := json.Marshal(msg)

	mw.ProcessRequest(context.Background(), body)

	if len(gotQuery) > 500 {
		t.Errorf("self-query should be truncated to 500 chars, got %d", len(gotQuery))
	}
	if gotQuery == "" {
		t.Error("expected non-empty self-query")
	}
}

func TestMiddleware_Enabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	mw := NewMiddleware(&mockBackend{}, cfg, NewStats())
	if !mw.Enabled() {
		t.Error("expected Enabled()=true")
	}

	cfg.Enabled = false
	mw2 := NewMiddleware(&mockBackend{}, cfg, NewStats())
	if mw2.Enabled() {
		t.Error("expected Enabled()=false")
	}
}

func TestMiddleware_ProcessRequest_RecordsPassthrough(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.MinLines = 5

	stats := NewStats()
	mw := NewMiddleware(&mockBackend{}, cfg, stats)

	body := []byte(`{"messages":[{"role":"user","content":"hello world"}]}`)
	_, err := mw.ProcessRequest(context.Background(), body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	snap := stats.Snapshot()
	if snap.TotalRequests != 1 {
		t.Fatalf("expected total_requests=1, got %d", snap.TotalRequests)
	}
	if snap.PassthroughRequests != 1 {
		t.Fatalf("expected passthrough_requests=1, got %d", snap.PassthroughRequests)
	}
	if snap.PrunedRequests != 0 {
		t.Fatalf("expected pruned_requests=0, got %d", snap.PrunedRequests)
	}
}
