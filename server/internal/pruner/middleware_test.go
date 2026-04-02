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

func decodeMessages(t *testing.T, body []byte) []map[string]json.RawMessage {
	t.Helper()
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	var messages []map[string]json.RawMessage
	if err := json.Unmarshal(parsed["messages"], &messages); err != nil {
		t.Fatalf("unmarshal messages: %v", err)
	}
	return messages
}

func messageContent(t *testing.T, msg map[string]json.RawMessage) string {
	t.Helper()
	var content string
	if err := json.Unmarshal(msg["content"], &content); err != nil {
		t.Fatalf("unmarshal content: %v", err)
	}
	return content
}

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

	messages := decodeMessages(t, result)

	if got := messageContent(t, messages[1]); got != "(pruned)" {
		t.Errorf("expected pruned content, got %s", got)
	}

	snap := stats.Snapshot()
	if snap.PrunedRequests != 1 {
		t.Errorf("expected 1 pruned request, got %d", snap.PrunedRequests)
	}
	if snap.TokensSaved != 150 {
		t.Errorf("expected 150 tokens saved, got %d", snap.TokensSaved)
	}
}

func TestMiddleware_ProcessRequest_SkipsNonSavingPruneResult(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.MinLines = 5

	longCode := strings.Repeat("package main\nimport \"fmt\"\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n", 5)

	backend := &mockBackend{
		pruneFunc: func(ctx context.Context, req PruneRequest) (*PruneResponse, error) {
			return &PruneResponse{
				PrunedContent:  "(expanded-pruned-output)",
				PrunedCode:     "(expanded-pruned-output)",
				OriginalTokens: 120,
				PrunedTokens:   124,
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

	pruneStats := &RequestPruneStats{}
	result, err := mw.ProcessRequest(WithPruneStats(context.Background(), pruneStats), body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	messages := decodeMessages(t, result)

	if got := messageContent(t, messages[1]); got != longCode {
		t.Errorf("expected original content preserved when prune result does not save tokens, got %q", got)
	}

	if pruneStats.Pruned {
		t.Fatal("expected request prune stats to stay empty when non-saving prune result is skipped")
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

func TestMiddleware_ProcessRequest_AllowsLegacyPruneResultWithoutTokenCounts(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.MinLines = 5

	longCode := strings.Repeat("package main\nimport \"fmt\"\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n", 5)

	backend := &mockBackend{
		pruneFunc: func(ctx context.Context, req PruneRequest) (*PruneResponse, error) {
			return &PruneResponse{
				PrunedContent: "(legacy-pruned-output)",
				PrunedCode:    "(legacy-pruned-output)",
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

	messages := decodeMessages(t, result)
	if got := messageContent(t, messages[1]); got != "(legacy-pruned-output)" {
		t.Errorf("expected legacy prune result to still apply without token counts, got %q", got)
	}

	snap := stats.Snapshot()
	if snap.PrunedRequests != 1 {
		t.Fatalf("expected pruned_requests=1, got %d", snap.PrunedRequests)
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
	messages := decodeMessages(t, result)

	if got := messageContent(t, messages[1]); got != "(pruned-content)" {
		t.Errorf("expected pruned content from PrunedContent field, got %s", got)
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

	messages := decodeMessages(t, result)

	if got := messageContent(t, messages[0]); got != "(pruned-doc)" {
		t.Errorf("expected pruned user message, got %s", got)
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

func TestMiddleware_ProcessRequest_PreservesAssistantToolCalls(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.MinLines = 1

	backend := &mockBackend{
		pruneFunc: func(ctx context.Context, req PruneRequest) (*PruneResponse, error) {
			return &PruneResponse{
				PrunedContent:  "(pruned)",
				PrunedCode:     "(pruned)",
				OriginalTokens: 200,
				PrunedTokens:   100,
			}, nil
		},
	}
	mw := NewMiddleware(backend, cfg, NewStats())

	body := []byte(`{"messages":[{"role":"assistant","content":"","tool_calls":[{"id":"call_1","type":"function","function":{"name":"search","arguments":"{}"}}]},{"role":"tool","tool_call_id":"call_1","content":"package main\nfunc main(){\nreturn\n}\n"}]}`)
	result, err := mw.ProcessRequest(context.Background(), body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	var messages []map[string]json.RawMessage
	if err := json.Unmarshal(parsed["messages"], &messages); err != nil {
		t.Fatalf("unmarshal messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if _, ok := messages[0]["tool_calls"]; !ok {
		t.Fatal("assistant tool_calls should be preserved after pruning")
	}
	var toolContent string
	if err := json.Unmarshal(messages[1]["content"], &toolContent); err != nil {
		t.Fatalf("unmarshal tool content: %v", err)
	}
	if toolContent != "(pruned)" {
		t.Fatalf("expected pruned tool content, got %q", toolContent)
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
