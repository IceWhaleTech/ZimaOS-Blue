package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

// --- helpers ---

type mockTool struct {
	name   string
	desc   string
	result interface{}
}

func (m *mockTool) Definition() tools.ToolDefinition {
	return tools.ToolDefinition{
		Name:        m.name,
		Description: m.desc,
		Parameters:  map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
	}
}

func (m *mockTool) Execute(_ context.Context, _ map[string]interface{}) (interface{}, error) {
	return m.result, nil
}

// slowTool blocks until context is cancelled, simulating a long-running tool.
type slowTool struct {
	name string
}

func (s *slowTool) Definition() tools.ToolDefinition {
	return tools.ToolDefinition{
		Name:        s.name,
		Description: "slow tool for timeout testing",
		Parameters:  map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
	}
}

func (s *slowTool) Execute(ctx context.Context, _ map[string]interface{}) (interface{}, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

type mockWorkspace struct {
	files map[string]string
}

func (m *mockWorkspace) LoadContextFiles() map[string]string {
	return m.files
}

func testServer(t *testing.T) *Server {
	t.Helper()
	registry := tools.NewRegistry()
	registry.Register(&mockTool{name: "exec", desc: "run commands", result: "ok"})
	registry.Register(&mockTool{name: "memory_search", desc: "search memory", result: map[string]string{"found": "yes"}})
	executor := tools.NewExecutor(registry)
	return NewServer(registry, executor)
}

func rpcCall(t *testing.T, s *Server, sessID, method string, params interface{}) jsonRPCResponse {
	t.Helper()
	var rawParams json.RawMessage
	if params != nil {
		b, _ := json.Marshal(params)
		rawParams = b
	}
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  method,
		Params:  rawParams,
	}
	raw, _ := json.Marshal(req)
	resp, err := s.HandleMessage(context.Background(), sessID, raw)
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		return jsonRPCResponse{}
	}
	var rpcResp jsonRPCResponse
	if err := json.Unmarshal(resp, &rpcResp); err != nil {
		t.Fatal(err)
	}
	return rpcResp
}

// --- Session tests ---

func TestSession_CreateAndRemove(t *testing.T) {
	s := testServer(t)
	sess := s.CreateSession()
	if sess.ID == "" {
		t.Error("session ID should not be empty")
	}

	s.RemoveSession(sess.ID)
	// Removing again should not panic
	s.RemoveSession(sess.ID)
}

func TestSendToSession(t *testing.T) {
	s := testServer(t)
	sess := s.CreateSession()

	s.SendToSession(sess.ID, []byte("hello"))
	msg := <-sess.Messages
	if string(msg) != "hello" {
		t.Errorf("got %q, want %q", string(msg), "hello")
	}

	// Send to nonexistent session — should not panic
	s.SendToSession("nonexistent", []byte("nope"))
}

// --- Protocol tests ---

func TestInitialize(t *testing.T) {
	s := testServer(t)
	sess := s.CreateSession()
	resp := rpcCall(t, s, sess.ID, "initialize", nil)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result, _ := json.Marshal(resp.Result)
	var initResult initializeResult
	json.Unmarshal(result, &initResult)

	if initResult.ProtocolVersion != ProtocolVersion {
		t.Errorf("protocol version = %q, want %q", initResult.ProtocolVersion, ProtocolVersion)
	}
	if initResult.ServerInfo.Name != ServerName {
		t.Errorf("server name = %q, want %q", initResult.ServerInfo.Name, ServerName)
	}
}

func TestInitialized_NoResponse(t *testing.T) {
	s := testServer(t)
	sess := s.CreateSession()

	req := jsonRPCRequest{JSONRPC: "2.0", Method: "initialized"}
	raw, _ := json.Marshal(req)
	resp, err := s.HandleMessage(context.Background(), sess.ID, raw)
	if err != nil {
		t.Fatal(err)
	}
	if resp != nil {
		t.Errorf("expected nil response for initialized, got %s", string(resp))
	}
}

func TestPing(t *testing.T) {
	s := testServer(t)
	sess := s.CreateSession()
	resp := rpcCall(t, s, sess.ID, "ping", nil)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestToolsList(t *testing.T) {
	s := testServer(t)
	sess := s.CreateSession()
	resp := rpcCall(t, s, sess.ID, "tools/list", nil)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result, _ := json.Marshal(resp.Result)
	var listResult toolsListResult
	json.Unmarshal(result, &listResult)

	if len(listResult.Tools) != 2 {
		t.Fatalf("got %d tools, want 2", len(listResult.Tools))
	}

	names := map[string]bool{}
	for _, tool := range listResult.Tools {
		names[tool.Name] = true
	}
	if !names["exec"] || !names["memory_search"] {
		t.Errorf("expected exec and memory_search, got %v", names)
	}
}

func TestToolsCall(t *testing.T) {
	s := testServer(t)
	sess := s.CreateSession()
	resp := rpcCall(t, s, sess.ID, "tools/call", toolCallParams{
		Name:      "exec",
		Arguments: map[string]interface{}{"command": "echo hi"},
	})

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result, _ := json.Marshal(resp.Result)
	var callResult toolCallResult
	json.Unmarshal(result, &callResult)

	if callResult.IsError {
		t.Error("expected no error")
	}
	if len(callResult.Content) == 0 {
		t.Fatal("expected content")
	}
	if callResult.Content[0].Text != "ok" {
		t.Errorf("text = %q, want %q", callResult.Content[0].Text, "ok")
	}
}

func TestToolsCall_NonexistentTool(t *testing.T) {
	s := testServer(t)
	sess := s.CreateSession()
	resp := rpcCall(t, s, sess.ID, "tools/call", toolCallParams{
		Name: "nonexistent",
	})

	if resp.Error != nil {
		t.Fatal("expected success response with isError=true")
	}

	result, _ := json.Marshal(resp.Result)
	var callResult toolCallResult
	json.Unmarshal(result, &callResult)

	if !callResult.IsError {
		t.Error("expected isError=true for nonexistent tool")
	}
}

func TestMethodNotFound(t *testing.T) {
	s := testServer(t)
	sess := s.CreateSession()
	resp := rpcCall(t, s, sess.ID, "unknown/method", nil)

	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("error code = %d, want -32601", resp.Error.Code)
	}
}

func TestParseError(t *testing.T) {
	s := testServer(t)
	sess := s.CreateSession()

	resp, err := s.HandleMessage(context.Background(), sess.ID, []byte("not json"))
	if err != nil {
		t.Fatal(err)
	}

	var rpcResp jsonRPCResponse
	json.Unmarshal(resp, &rpcResp)
	if rpcResp.Error == nil || rpcResp.Error.Code != -32700 {
		t.Errorf("expected parse error (-32700), got %+v", rpcResp.Error)
	}
}

// --- Resource tests ---

func TestResourcesList_NoWorkspace(t *testing.T) {
	s := testServer(t)
	sess := s.CreateSession()
	resp := rpcCall(t, s, sess.ID, "resources/list", nil)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result, _ := json.Marshal(resp.Result)
	var listResult resourcesListResult
	json.Unmarshal(result, &listResult)

	if len(listResult.Resources) != 0 {
		t.Errorf("expected 0 resources without workspace, got %d", len(listResult.Resources))
	}
}

func TestResourcesList_WithWorkspace(t *testing.T) {
	s := testServer(t)
	s.SetWorkspace(&mockWorkspace{files: map[string]string{
		"SOUL.md":   "soul content",
		"MEMORY.md": "memory content",
	}})
	sess := s.CreateSession()
	resp := rpcCall(t, s, sess.ID, "resources/list", nil)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result, _ := json.Marshal(resp.Result)
	var listResult resourcesListResult
	json.Unmarshal(result, &listResult)

	if len(listResult.Resources) != 2 {
		t.Errorf("expected 2 resources, got %d", len(listResult.Resources))
	}
}

func TestResourcesRead(t *testing.T) {
	s := testServer(t)
	s.SetWorkspace(&mockWorkspace{files: map[string]string{
		"SOUL.md": "I am Blue",
	}})
	sess := s.CreateSession()
	resp := rpcCall(t, s, sess.ID, "resources/read", resourceReadParams{
		URI: "workspace:///SOUL.md",
	})

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result, _ := json.Marshal(resp.Result)
	var readResult resourceReadResult
	json.Unmarshal(result, &readResult)

	if len(readResult.Contents) != 1 {
		t.Fatalf("expected 1 content, got %d", len(readResult.Contents))
	}
	if readResult.Contents[0].Text != "I am Blue" {
		t.Errorf("text = %q, want %q", readResult.Contents[0].Text, "I am Blue")
	}
}

func TestResourcesRead_NotFound(t *testing.T) {
	s := testServer(t)
	s.SetWorkspace(&mockWorkspace{files: map[string]string{}})
	sess := s.CreateSession()
	resp := rpcCall(t, s, sess.ID, "resources/read", resourceReadParams{
		URI: "workspace:///NONEXISTENT.md",
	})

	if resp.Error == nil {
		t.Fatal("expected error for nonexistent resource")
	}
}

func TestResourcesRead_InvalidURI(t *testing.T) {
	s := testServer(t)
	s.SetWorkspace(&mockWorkspace{files: map[string]string{}})
	sess := s.CreateSession()
	resp := rpcCall(t, s, sess.ID, "resources/read", resourceReadParams{
		URI: "invalid://uri",
	})

	if resp.Error == nil {
		t.Fatal("expected error for invalid URI")
	}
}

func TestResourcesRead_NoWorkspace(t *testing.T) {
	s := testServer(t)
	sess := s.CreateSession()
	resp := rpcCall(t, s, sess.ID, "resources/read", resourceReadParams{
		URI: "workspace:///SOUL.md",
	})

	if resp.Error == nil {
		t.Fatal("expected error when workspace not configured")
	}
}

// --- Prompt tests ---

func TestPromptsList(t *testing.T) {
	s := testServer(t)
	sess := s.CreateSession()
	resp := rpcCall(t, s, sess.ID, "prompts/list", nil)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result, _ := json.Marshal(resp.Result)
	var listResult promptsListResult
	json.Unmarshal(result, &listResult)

	if len(listResult.Prompts) != 3 {
		t.Fatalf("got %d prompts, want 3", len(listResult.Prompts))
	}

	names := map[string]bool{}
	for _, p := range listResult.Prompts {
		names[p.Name] = true
	}
	if !names["agent_task"] || !names["code_review"] || !names["summarize"] {
		t.Errorf("expected agent_task, code_review, summarize; got %v", names)
	}
}

func TestPromptsGet_AgentTask(t *testing.T) {
	s := testServer(t)
	sess := s.CreateSession()
	resp := rpcCall(t, s, sess.ID, "prompts/get", promptGetParams{
		Name:      "agent_task",
		Arguments: map[string]string{"goal": "deploy the app"},
	})

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result, _ := json.Marshal(resp.Result)
	var getResult promptGetResult
	json.Unmarshal(result, &getResult)

	if len(getResult.Messages) != 1 {
		t.Fatalf("got %d messages, want 1", len(getResult.Messages))
	}
	if getResult.Messages[0].Role != "user" {
		t.Errorf("role = %q, want user", getResult.Messages[0].Role)
	}
}

func TestPromptsGet_MissingArg(t *testing.T) {
	s := testServer(t)
	sess := s.CreateSession()
	resp := rpcCall(t, s, sess.ID, "prompts/get", promptGetParams{
		Name: "agent_task",
	})

	if resp.Error == nil {
		t.Fatal("expected error for missing goal argument")
	}
}

func TestPromptsGet_Unknown(t *testing.T) {
	s := testServer(t)
	sess := s.CreateSession()
	resp := rpcCall(t, s, sess.ID, "prompts/get", promptGetParams{
		Name: "nonexistent",
	})

	if resp.Error == nil {
		t.Fatal("expected error for unknown prompt")
	}
}

// --- Additional tool tests ---

func TestToolsCall_WithArguments(t *testing.T) {
	s := testServer(t)
	sess := s.CreateSession()
	resp := rpcCall(t, s, sess.ID, "tools/call", toolCallParams{
		Name:      "memory_search",
		Arguments: map[string]interface{}{"query": "test search"},
	})

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result, _ := json.Marshal(resp.Result)
	var callResult toolCallResult
	json.Unmarshal(result, &callResult)

	if callResult.IsError {
		t.Error("expected no error")
	}
	if len(callResult.Content) == 0 {
		t.Fatal("expected content")
	}
}

func TestToolsCall_MissingName(t *testing.T) {
	s := testServer(t)
	sess := s.CreateSession()
	resp := rpcCall(t, s, sess.ID, "tools/call", toolCallParams{
		Name: "",
	})

	if resp.Error != nil {
		return // error response is acceptable
	}
	result, _ := json.Marshal(resp.Result)
	var callResult toolCallResult
	json.Unmarshal(result, &callResult)
	if !callResult.IsError {
		t.Error("expected isError=true for empty tool name")
	}
}

// --- Session edge cases ---

func TestSession_MultipleSessions(t *testing.T) {
	s := testServer(t)
	sess1 := s.CreateSession()
	sess2 := s.CreateSession()

	if sess1.ID == sess2.ID {
		t.Error("sessions should have unique IDs")
	}

	resp1 := rpcCall(t, s, sess1.ID, "ping", nil)
	resp2 := rpcCall(t, s, sess2.ID, "ping", nil)

	if resp1.Error != nil || resp2.Error != nil {
		t.Error("both sessions should handle ping")
	}
}

func TestHandleMessage_InvalidSession(t *testing.T) {
	s := testServer(t)
	req := jsonRPCRequest{JSONRPC: "2.0", ID: 1, Method: "ping"}
	raw, _ := json.Marshal(req)
	resp, err := s.HandleMessage(context.Background(), "nonexistent-session", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
}

// --- Protocol edge cases ---

func TestInitialize_Twice(t *testing.T) {
	s := testServer(t)
	sess := s.CreateSession()

	resp1 := rpcCall(t, s, sess.ID, "initialize", nil)
	if resp1.Error != nil {
		t.Fatalf("first initialize failed: %v", resp1.Error)
	}

	resp2 := rpcCall(t, s, sess.ID, "initialize", nil)
	if resp2.Error != nil {
		t.Fatalf("second initialize failed: %v", resp2.Error)
	}
}

func TestToolsList_EmptyRegistry(t *testing.T) {
	registry := tools.NewRegistry()
	executor := tools.NewExecutor(registry)
	s := NewServer(registry, executor)
	sess := s.CreateSession()

	resp := rpcCall(t, s, sess.ID, "tools/list", nil)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result, _ := json.Marshal(resp.Result)
	var listResult toolsListResult
	json.Unmarshal(result, &listResult)

	if len(listResult.Tools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(listResult.Tools))
	}
}

// --- Prompts edge cases ---

func TestPromptsGet_CodeReview(t *testing.T) {
	s := testServer(t)
	sess := s.CreateSession()
	resp := rpcCall(t, s, sess.ID, "prompts/get", promptGetParams{
		Name:      "code_review",
		Arguments: map[string]string{"code": "func main() {}"},
	})

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result, _ := json.Marshal(resp.Result)
	var getResult promptGetResult
	json.Unmarshal(result, &getResult)

	if len(getResult.Messages) != 1 {
		t.Fatalf("got %d messages, want 1", len(getResult.Messages))
	}
}

func TestPromptsGet_Summarize(t *testing.T) {
	s := testServer(t)
	sess := s.CreateSession()
	resp := rpcCall(t, s, sess.ID, "prompts/get", promptGetParams{
		Name:      "summarize",
		Arguments: map[string]string{"text": "This is a long text that needs summarizing."},
	})

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result, _ := json.Marshal(resp.Result)
	var getResult promptGetResult
	json.Unmarshal(result, &getResult)

	if len(getResult.Messages) != 1 {
		t.Fatalf("got %d messages, want 1", len(getResult.Messages))
	}
}

func TestPromptsGet_CodeReview_MissingArg(t *testing.T) {
	s := testServer(t)
	sess := s.CreateSession()
	resp := rpcCall(t, s, sess.ID, "prompts/get", promptGetParams{
		Name: "code_review",
	})

	if resp.Error == nil {
		t.Fatal("expected error for missing code argument")
	}
}

func TestPromptsGet_Summarize_MissingArg(t *testing.T) {
	s := testServer(t)
	sess := s.CreateSession()
	resp := rpcCall(t, s, sess.ID, "prompts/get", promptGetParams{
		Name: "summarize",
	})

	if resp.Error == nil {
		t.Fatal("expected error for missing text argument")
	}
}

// --- Resource edge cases ---

func TestResourcesList_EmptyWorkspace(t *testing.T) {
	s := testServer(t)
	s.SetWorkspace(&mockWorkspace{files: map[string]string{}})
	sess := s.CreateSession()
	resp := rpcCall(t, s, sess.ID, "resources/list", nil)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result, _ := json.Marshal(resp.Result)
	var listResult resourcesListResult
	json.Unmarshal(result, &listResult)

	if len(listResult.Resources) != 0 {
		t.Errorf("expected 0 resources for empty workspace, got %d", len(listResult.Resources))
	}
}

func TestResourcesRead_MultipleFiles(t *testing.T) {
	s := testServer(t)
	s.SetWorkspace(&mockWorkspace{files: map[string]string{
		"SOUL.md":     "soul",
		"MEMORY.md":   "memory",
		"IDENTITY.md": "identity",
	}})
	sess := s.CreateSession()

	for name, expected := range map[string]string{
		"SOUL.md":     "soul",
		"MEMORY.md":   "memory",
		"IDENTITY.md": "identity",
	} {
		resp := rpcCall(t, s, sess.ID, "resources/read", resourceReadParams{
			URI: "workspace:///" + name,
		})
		if resp.Error != nil {
			t.Fatalf("error reading %s: %v", name, resp.Error)
		}
		result, _ := json.Marshal(resp.Result)
		var readResult resourceReadResult
		json.Unmarshal(result, &readResult)
		if len(readResult.Contents) != 1 || readResult.Contents[0].Text != expected {
			t.Errorf("%s: got %q, want %q", name, readResult.Contents[0].Text, expected)
		}
	}
}

// --- JSON-RPC edge cases ---

func TestEmptyMessage(t *testing.T) {
	s := testServer(t)
	sess := s.CreateSession()

	resp, err := s.HandleMessage(context.Background(), sess.ID, []byte(""))
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		return
	}
	var rpcResp jsonRPCResponse
	json.Unmarshal(resp, &rpcResp)
	if rpcResp.Error == nil {
		t.Error("expected error for empty message")
	}
}

func TestNullMessage(t *testing.T) {
	s := testServer(t)
	sess := s.CreateSession()

	resp, err := s.HandleMessage(context.Background(), sess.ID, []byte("null"))
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		return
	}
	var rpcResp jsonRPCResponse
	json.Unmarshal(resp, &rpcResp)
	if rpcResp.Error == nil {
		t.Error("expected error for null message")
	}
}

// --- Tools cache tests ---

func TestToolsList_CacheHit(t *testing.T) {
	s := testServer(t)
	sess := s.CreateSession()

	// First call populates cache
	resp1 := rpcCall(t, s, sess.ID, "tools/list", nil)
	if resp1.Error != nil {
		t.Fatalf("unexpected error: %v", resp1.Error)
	}

	// Second call should hit cache — verify same result
	resp2 := rpcCall(t, s, sess.ID, "tools/list", nil)
	if resp2.Error != nil {
		t.Fatalf("unexpected error: %v", resp2.Error)
	}

	b1, _ := json.Marshal(resp1.Result)
	b2, _ := json.Marshal(resp2.Result)
	if string(b1) != string(b2) {
		t.Errorf("cached response differs:\n  first:  %s\n  second: %s", b1, b2)
	}
}

func TestToolsList_CacheInvalidation(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(&mockTool{name: "tool_a", desc: "a", result: "ok"})
	executor := tools.NewExecutor(registry)
	s := NewServer(registry, executor)
	sess := s.CreateSession()

	resp1 := rpcCall(t, s, sess.ID, "tools/list", nil)
	result1, _ := json.Marshal(resp1.Result)
	var list1 toolsListResult
	json.Unmarshal(result1, &list1)
	if len(list1.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(list1.Tools))
	}

	// Register a new tool — cache should be invalidated
	registry.Register(&mockTool{name: "tool_b", desc: "b", result: "ok"})

	resp2 := rpcCall(t, s, sess.ID, "tools/list", nil)
	result2, _ := json.Marshal(resp2.Result)
	var list2 toolsListResult
	json.Unmarshal(result2, &list2)
	if len(list2.Tools) != 2 {
		t.Fatalf("expected 2 tools after cache invalidation, got %d", len(list2.Tools))
	}
}

// --- Close tests ---

func TestServer_Close(t *testing.T) {
	s := testServer(t)
	sess := s.CreateSession()

	s.Close()

	// Session's done channel should be closed
	select {
	case <-sess.done:
		// expected
	default:
		t.Error("session done channel should be closed after Server.Close()")
	}

	// Double close should not panic
	s.Close()
}

// --- Timeout test ---

func TestToolsCall_Timeout(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(&slowTool{name: "slow"})
	executor := tools.NewExecutor(registry)
	s := NewServer(registry, executor)
	s.SetToolCallTimeout(50 * time.Millisecond) // very short timeout
	sess := s.CreateSession()

	start := time.Now()
	resp := rpcCall(t, s, sess.ID, "tools/call", toolCallParams{
		Name: "slow",
	})
	elapsed := time.Since(start)

	if resp.Error != nil {
		t.Fatalf("expected success response with isError=true, got rpc error: %v", resp.Error)
	}

	result, _ := json.Marshal(resp.Result)
	var callResult toolCallResult
	json.Unmarshal(result, &callResult)

	if !callResult.IsError {
		t.Error("expected isError=true for timed-out tool")
	}
	if elapsed > 2*time.Second {
		t.Errorf("timeout took too long: %v (expected ~50ms)", elapsed)
	}
}

// --- Registry version test ---

func TestRegistry_Version(t *testing.T) {
	r := tools.NewRegistry()
	v0 := r.Version()

	r.Register(&mockTool{name: "a", desc: "a"})
	v1 := r.Version()
	if v1 <= v0 {
		t.Errorf("version should increase after Register: %d -> %d", v0, v1)
	}

	r.Disable("a")
	v2 := r.Version()
	if v2 <= v1 {
		t.Errorf("version should increase after Disable: %d -> %d", v1, v2)
	}

	r.Enable("a")
	v3 := r.Version()
	if v3 <= v2 {
		t.Errorf("version should increase after Enable: %d -> %d", v2, v3)
	}
}

// --- Buffer overflow test ---

func TestSendToSession_BufferOverflow(t *testing.T) {
	s := testServer(t)
	sess := s.CreateSession()

	// Fill the buffer (sessionBufferSize = 64)
	for i := 0; i < sessionBufferSize; i++ {
		s.SendToSession(sess.ID, []byte("msg"))
	}

	// Next send should be dropped (not block)
	done := make(chan struct{})
	go func() {
		s.SendToSession(sess.ID, []byte("overflow"))
		close(done)
	}()

	select {
	case <-done:
		// good — didn't block
	case <-time.After(time.Second):
		t.Fatal("SendToSession blocked on full buffer")
	}

	// Drain and verify we got exactly sessionBufferSize messages
	count := 0
	for {
		select {
		case <-sess.Messages:
			count++
		default:
			goto done_drain
		}
	}
done_drain:
	if count != sessionBufferSize {
		t.Errorf("expected %d messages, got %d", sessionBufferSize, count)
	}
}

// --- Concurrent cache access test ---

func TestToolsList_ConcurrentAccess(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(&mockTool{name: "tool_a", desc: "a", result: "ok"})
	executor := tools.NewExecutor(registry)
	s := NewServer(registry, executor)
	sess := s.CreateSession()

	// Hammer tools/list from multiple goroutines while mutating registry
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 100; i++ {
			rpcCall(t, s, sess.ID, "tools/list", nil)
		}
	}()

	// Mutate registry concurrently
	for i := 0; i < 50; i++ {
		name := fmt.Sprintf("tool_%d", i)
		registry.Register(&mockTool{name: name, desc: name, result: "ok"})
	}

	<-done
}

// --- Timeout default test ---

func TestToolCallTimeout_Default(t *testing.T) {
	s := testServer(t)
	// Don't call SetToolCallTimeout — should use default
	d := s.getToolCallTimeout()
	if d != defaultToolCallTimeout {
		t.Errorf("default timeout = %v, want %v", d, defaultToolCallTimeout)
	}
}

// --- Path traversal tests ---

func TestResourcesRead_PathTraversal(t *testing.T) {
	s := testServer(t)
	s.SetWorkspace(&mockWorkspace{files: map[string]string{"SOUL.md": "content"}})
	sess := s.CreateSession()

	cases := []string{
		"workspace:///../../etc/passwd",
		"workspace:///../secret",
		"workspace:///sub/dir/file",
		"workspace:///back\\slash",
	}

	for _, uri := range cases {
		resp := rpcCall(t, s, sess.ID, "resources/read", resourceReadParams{URI: uri})
		if resp.Error == nil {
			t.Errorf("expected error for traversal URI %q, got success", uri)
		}
	}
}
