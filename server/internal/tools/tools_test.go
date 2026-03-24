package tools

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	convertpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/convert"
	pdfextract "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pdf"
)

type captureArgsTool struct {
	def  ToolDefinition
	args map[string]interface{}
}

func (t *captureArgsTool) Definition() ToolDefinition {
	return t.def
}

func (t *captureArgsTool) Execute(_ context.Context, args map[string]interface{}) (interface{}, error) {
	t.args = args
	return "ok", nil
}

type stubDocumentReadService struct {
	result   *convertpkg.DocumentReadResult
	err      error
	lastPath string
}

func (s *stubDocumentReadService) ReadDocument(_ context.Context, path string) (*convertpkg.DocumentReadResult, error) {
	s.lastPath = path
	if s.err != nil {
		return nil, s.err
	}
	return s.result, nil
}

type testRoundTripper func(req *http.Request) (*http.Response, error)

func (fn testRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

type stubHTTPNativeClient struct {
	mu        sync.Mutex
	available bool
	calls     []webFetchHTTPNativeRequest
	do        func(context.Context, webFetchHTTPNativeRequest) (webFetchHTTPNativeResponse, error)
}

func (s *stubHTTPNativeClient) Available() bool {
	return s != nil && s.available
}

func (s *stubHTTPNativeClient) Do(ctx context.Context, req webFetchHTTPNativeRequest) (webFetchHTTPNativeResponse, error) {
	if s == nil {
		return webFetchHTTPNativeResponse{}, errWebFetchHTTPNativeUnavailable
	}
	cloned := req
	if len(req.Headers) > 0 {
		cloned.Headers = make(map[string]string, len(req.Headers))
		for key, value := range req.Headers {
			cloned.Headers[key] = value
		}
	}
	s.mu.Lock()
	s.calls = append(s.calls, cloned)
	s.mu.Unlock()
	if s.do != nil {
		return s.do(ctx, req)
	}
	return webFetchHTTPNativeResponse{}, nil
}

func (s *stubHTTPNativeClient) callCount() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

// Test Tool interface
func TestToolInterface(t *testing.T) {
	var _ Tool = (*MockTool)(nil)
}

// Test ToolDefinition struct
func TestToolDefinition(t *testing.T) {
	def := ToolDefinition{
		Name:        "calculator",
		Description: "Performs basic arithmetic operations",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"expression": map[string]interface{}{
					"type":        "string",
					"description": "The arithmetic expression to evaluate",
				},
			},
			"required": []string{"expression"},
		},
	}

	if def.Name != "calculator" {
		t.Errorf("expected name 'calculator', got '%s'", def.Name)
	}
	if def.Description != "Performs basic arithmetic operations" {
		t.Errorf("expected description 'Performs basic arithmetic operations', got '%s'", def.Description)
	}
}

// Test MockTool
func TestMockTool(t *testing.T) {
	tool := NewMockTool("test_tool", "A test tool")
	tool.SetResult("test result")

	if tool.Definition().Name != "test_tool" {
		t.Errorf("expected name 'test_tool', got '%s'", tool.Definition().Name)
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "test result" {
		t.Errorf("expected result 'test result', got '%v'", result)
	}
}

// Test MockTool error
func TestMockToolError(t *testing.T) {
	tool := NewMockTool("test_tool", "A test tool")
	expectedErr := errors.New("test error")
	tool.SetError(expectedErr)

	_, err := tool.Execute(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

// Test Registry creation
func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()
	if registry == nil {
		t.Fatal("expected registry, got nil")
	}
}

// Test Registry Register and Get
func TestRegistryRegisterAndGet(t *testing.T) {
	registry := NewRegistry()
	tool := NewMockTool("test_tool", "A test tool")

	registry.Register(tool)

	retrieved := registry.Get("test_tool")
	if retrieved == nil {
		t.Fatal("expected tool, got nil")
	}
	if retrieved.Definition().Name != "test_tool" {
		t.Errorf("expected name 'test_tool', got '%s'", retrieved.Definition().Name)
	}
}

// Test Registry Get nonexistent
func TestRegistryGetNonexistent(t *testing.T) {
	registry := NewRegistry()
	retrieved := registry.Get("nonexistent")
	if retrieved != nil {
		t.Error("expected nil for nonexistent tool")
	}
}

// Test Registry List
func TestRegistryList(t *testing.T) {
	registry := NewRegistry()
	registry.Register(NewMockTool("tool1", "Tool 1"))
	registry.Register(NewMockTool("tool2", "Tool 2"))

	tools := registry.List()
	if len(tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(tools))
	}
}

// Test Registry Definitions
func TestRegistryDefinitions(t *testing.T) {
	registry := NewRegistry()
	registry.Register(NewMockTool("tool1", "Tool 1"))
	registry.Register(NewMockTool("tool2", "Tool 2"))

	defs := registry.Definitions()
	if len(defs) != 2 {
		t.Errorf("expected 2 definitions, got %d", len(defs))
	}
}

func TestRegistryExposeDefinition(t *testing.T) {
	registry := NewRegistry()
	registry.ExposeDefinition(ToolDefinition{
		Name:        "sessions_list",
		Description: "List sessions",
	})

	if got := registry.List(); len(got) != 0 {
		t.Fatalf("active tool list = %v, want empty for exposed-only definition", got)
	}
	if got := registry.Get("sessions_list"); got != nil {
		t.Fatalf("Get returned %v for exposed-only definition, want nil", got)
	}

	defs := registry.Definitions()
	if len(defs) != 1 {
		t.Fatalf("definitions len = %d, want 1", len(defs))
	}
	if defs[0].Name != "sessions_list" {
		t.Fatalf("definition name = %q, want sessions_list", defs[0].Name)
	}
}

func TestRegistryExposeDefinition_ActiveToolWins(t *testing.T) {
	registry := NewRegistry()
	registry.ExposeDefinition(ToolDefinition{
		Name:        "web_search",
		Description: "compat",
	})
	registry.Register(NewMockTool("web_search", "native"))

	defs := registry.Definitions()
	if len(defs) != 1 {
		t.Fatalf("definitions len = %d, want 1", len(defs))
	}
	if defs[0].Description != "native" {
		t.Fatalf("definition description = %q, want native", defs[0].Description)
	}
}

// Test Registry duplicate registration
func TestRegistryDuplicateRegistration(t *testing.T) {
	registry := NewRegistry()
	tool1 := NewMockTool("test_tool", "Tool 1")
	tool2 := NewMockTool("test_tool", "Tool 2")

	registry.Register(tool1)
	registry.Register(tool2) // Should overwrite

	tools := registry.List()
	if len(tools) != 1 {
		t.Errorf("expected 1 tool after duplicate registration, got %d", len(tools))
	}

	retrieved := registry.Get("test_tool")
	if retrieved.Definition().Description != "Tool 2" {
		t.Error("expected second tool to overwrite first")
	}
}

// Test Executor creation
func TestNewExecutor(t *testing.T) {
	registry := NewRegistry()
	executor := NewExecutor(registry)
	if executor == nil {
		t.Fatal("expected executor, got nil")
	}
}

// Test Executor Execute
func TestExecutorExecute(t *testing.T) {
	registry := NewRegistry()
	tool := NewMockTool("test_tool", "A test tool")
	tool.SetResult("executed")
	registry.Register(tool)

	executor := NewExecutor(registry)
	result, err := executor.Execute(context.Background(), "test_tool", map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "executed" {
		t.Errorf("expected result 'executed', got '%v'", result)
	}
}

// Test Executor Execute with JSON arguments
func TestExecutorExecuteWithJSONArgs(t *testing.T) {
	registry := NewRegistry()
	tool := NewMockTool("test_tool", "A test tool")
	tool.SetResult("executed")
	registry.Register(tool)

	executor := NewExecutor(registry)
	argsJSON := `{"key": "value"}`
	result, err := executor.ExecuteJSON(context.Background(), "test_tool", argsJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "executed" {
		t.Errorf("expected result 'executed', got '%v'", result)
	}
}

func TestExecutorExecuteJSONArgs_KeyValueFallback(t *testing.T) {
	registry := NewRegistry()
	tool := &captureArgsTool{
		def: ToolDefinition{
			Name:        "exec",
			Description: "Execute command",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{"type": "string"},
					"cwd":     map[string]interface{}{"type": "string"},
				},
				"required": []string{"command"},
			},
		},
	}
	registry.Register(tool)

	executor := NewExecutor(registry)
	_, err := executor.ExecuteJSON(context.Background(), "exec", `command="ls -la" cwd=/tmp`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := tool.args["command"]; got != "ls -la" {
		t.Fatalf("command = %v, want %q", got, "ls -la")
	}
	if got := tool.args["cwd"]; got != "/tmp" {
		t.Fatalf("cwd = %v, want %q", got, "/tmp")
	}
}

func TestExecutorExecuteJSONArgs_RawStringFallbackToRequiredField(t *testing.T) {
	registry := NewRegistry()
	tool := &captureArgsTool{
		def: ToolDefinition{
			Name:        "exec",
			Description: "Execute command",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{"type": "string"},
					"cwd":     map[string]interface{}{"type": "string"},
				},
				"required": []interface{}{"command"},
			},
		},
	}
	registry.Register(tool)

	executor := NewExecutor(registry)
	_, err := executor.ExecuteJSON(context.Background(), "exec", `ls -la`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := tool.args["command"]; got != "ls -la" {
		t.Fatalf("command = %v, want %q", got, "ls -la")
	}
}

func TestExecutorExecuteJSONArgs_JSONStringFallbackToRequiredField(t *testing.T) {
	registry := NewRegistry()
	tool := &captureArgsTool{
		def: ToolDefinition{
			Name:        "exec",
			Description: "Execute command",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{"type": "string"},
				},
				"required": []string{"command"},
			},
		},
	}
	registry.Register(tool)

	executor := NewExecutor(registry)
	_, err := executor.ExecuteJSON(context.Background(), "exec", `"pwd"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := tool.args["command"]; got != "pwd" {
		t.Fatalf("command = %v, want %q", got, "pwd")
	}
}

func TestExecutorExecuteJSONArgs_LooseJSONObjectFallback(t *testing.T) {
	registry := NewRegistry()
	tool := &captureArgsTool{
		def: ToolDefinition{
			Name:        "exec",
			Description: "Execute command",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{"type": "string"},
					"cwd":     map[string]interface{}{"type": "string"},
				},
				"required": []string{"command"},
			},
		},
	}
	registry.Register(tool)

	executor := NewExecutor(registry)
	_, err := executor.ExecuteJSON(context.Background(), "exec", `{command:'ls -la', cwd:'/tmp',}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := tool.args["command"]; got != "ls -la" {
		t.Fatalf("command = %v, want %q", got, "ls -la")
	}
	if got := tool.args["cwd"]; got != "/tmp" {
		t.Fatalf("cwd = %v, want %q", got, "/tmp")
	}
}

func TestExecutorExecuteJSONArgs_IncompleteJSONObjectFallback(t *testing.T) {
	registry := NewRegistry()
	tool := &captureArgsTool{
		def: ToolDefinition{
			Name:        "exec",
			Description: "Execute command",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{"type": "string"},
				},
				"required": []string{"command"},
			},
		},
	}
	registry.Register(tool)

	executor := NewExecutor(registry)
	_, err := executor.ExecuteJSON(context.Background(), "exec", `{"command":"ls -la"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := tool.args["command"]; got != "ls -la" {
		t.Fatalf("command = %v, want %q", got, "ls -la")
	}
}

func TestExecutorExecuteJSONArgs_ExecCmdAliasFallback(t *testing.T) {
	registry := NewRegistry()
	tool := &captureArgsTool{
		def: ToolDefinition{
			Name:        "exec",
			Description: "Execute command",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{"type": "string"},
				},
				"required": []string{"command"},
			},
		},
	}
	registry.Register(tool)

	executor := NewExecutor(registry)
	_, err := executor.ExecuteJSON(context.Background(), "exec", `{"cmd":"pwd"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := tool.args["command"]; got != "pwd" {
		t.Fatalf("command = %v, want %q", got, "pwd")
	}
	if _, ok := tool.args["cmd"]; ok {
		t.Fatalf("expected cmd alias to be normalized away, got args=%v", tool.args)
	}
}

func TestExecutorExecuteJSONArgs_ExecToolAliasFallback(t *testing.T) {
	registry := NewRegistry()
	tool := &captureArgsTool{
		def: ToolDefinition{
			Name:        "exec",
			Description: "Execute command",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{"type": "string"},
				},
				"required": []string{"command"},
			},
		},
	}
	registry.Register(tool)

	executor := NewExecutor(registry)
	_, err := executor.ExecuteJSON(context.Background(), "exec", `{"tool":"pwd"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := tool.args["command"]; got != "pwd" {
		t.Fatalf("command = %v, want %q", got, "pwd")
	}
}

func TestExecutorExecuteJSONArgs_ExecInputCommandWrappedFallback(t *testing.T) {
	registry := NewRegistry()
	tool := &captureArgsTool{
		def: ToolDefinition{
			Name:        "exec",
			Description: "Execute command",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{"type": "string"},
				},
				"required": []string{"command"},
			},
		},
	}
	registry.Register(tool)

	executor := NewExecutor(registry)
	_, err := executor.ExecuteJSON(context.Background(), "exec", `{"input":{"command":"pwd"}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := tool.args["command"]; got != "pwd" {
		t.Fatalf("command = %v, want %q", got, "pwd")
	}
}

func TestExecutorExecuteJSONArgs_ExecPayloadCommandWrappedFallback(t *testing.T) {
	registry := NewRegistry()
	tool := &captureArgsTool{
		def: ToolDefinition{
			Name:        "exec",
			Description: "Execute command",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{"type": "string"},
				},
				"required": []string{"command"},
			},
		},
	}
	registry.Register(tool)

	executor := NewExecutor(registry)
	_, err := executor.ExecuteJSON(context.Background(), "exec", `{"payload":{"command":"pwd"}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := tool.args["command"]; got != "pwd" {
		t.Fatalf("command = %v, want %q", got, "pwd")
	}
}

func TestExecutorExecuteJSONArgs_ExecArgumentsWrappedFallback(t *testing.T) {
	registry := NewRegistry()
	tool := &captureArgsTool{
		def: ToolDefinition{
			Name:        "exec",
			Description: "Execute command",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{"type": "string"},
				},
				"required": []string{"command"},
			},
		},
	}
	registry.Register(tool)

	executor := NewExecutor(registry)
	_, err := executor.ExecuteJSON(context.Background(), "exec", `{"tool":"exec","arguments":{"cmd":"pwd"}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := tool.args["command"]; got != "pwd" {
		t.Fatalf("command = %v, want %q", got, "pwd")
	}
}

func TestExecutorExecuteJSONArgs_FileWriteArgumentsWrappedFallback(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "wrapped.txt")

	registry := NewRegistry()
	registry.Register(NewFileWriteTool([]string{tmpDir}, 0))
	executor := NewExecutor(registry)

	_, err := executor.ExecuteJSON(context.Background(), "file_write", `{"arguments":{"path":"`+target+`","content":"hello"}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := readTestFile(target)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if content != "hello" {
		t.Fatalf("content = %q, want %q", content, "hello")
	}
}

func TestExecutorExecuteJSONArgs_FileWriteArgumentsAliasFallback(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "alias.txt")

	registry := NewRegistry()
	registry.Register(NewFileWriteTool([]string{tmpDir}, 0))
	executor := NewExecutor(registry)

	_, err := executor.ExecuteJSON(context.Background(), "write", `{"input":{"file_path":"`+target+`","text":"hi"}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := readTestFile(target)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if content != "hi" {
		t.Fatalf("content = %q, want %q", content, "hi")
	}
}

func TestExecutorExecuteJSONArgs_FileWriteCamelCasePathFallback(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "camel.txt")

	registry := NewRegistry()
	registry.Register(NewFileWriteTool([]string{tmpDir}, 0))
	executor := NewExecutor(registry)

	_, err := executor.ExecuteJSON(context.Background(), "write", `{"input":{"filePath":"`+target+`","text":"hello"}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := readTestFile(target)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if content != "hello" {
		t.Fatalf("content = %q, want %q", content, "hello")
	}
}

func TestExecutorExecuteJSONArgs_FileReadCamelCasePathFallback(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "read-camel.txt")
	if err := os.WriteFile(target, []byte("a\nb\nc"), 0o644); err != nil {
		t.Fatalf("failed to seed read target: %v", err)
	}

	registry := NewRegistry()
	registry.Register(NewFileReadTool([]string{tmpDir}, 0))
	executor := NewExecutor(registry)

	result, err := executor.ExecuteJSON(context.Background(), "read", `{"input":{"filePath":"`+target+`","startLine":2,"endLine":2}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	raw, ok := result.(string)
	if !ok {
		t.Fatalf("result type = %T, want string", result)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("failed to decode read payload: %v", err)
	}
	if got := payload["content"]; got != "b" {
		t.Fatalf("content = %v, want %q", got, "b")
	}
	if got := payload["path"]; got != "read-camel.txt" {
		t.Fatalf("path = %v, want %q", got, "read-camel.txt")
	}
}

func TestExecutorExecuteJSONArgs_EditCamelCaseArgsFallback(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "edit-camel.txt")
	if err := os.WriteFile(target, []byte("old old"), 0o644); err != nil {
		t.Fatalf("failed to seed edit target: %v", err)
	}

	registry := NewRegistry()
	registry.Register(NewEditTool([]string{tmpDir}, 0))
	executor := NewExecutor(registry)

	_, err := executor.ExecuteJSON(context.Background(), "edit", `{"input":{"filePath":"`+target+`","oldText":"old","newText":"new","replaceAll":true}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := readTestFile(target)
	if err != nil {
		t.Fatalf("failed to read edited file: %v", err)
	}
	if content != "new new" {
		t.Fatalf("content = %q, want %q", content, "new new")
	}
}

func TestExecutorExecuteJSONArgs_GrepCamelCaseArgsFallback(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "grep.txt"), []byte("hello\nHELLO"), 0o644); err != nil {
		t.Fatalf("failed to seed grep target: %v", err)
	}

	registry := NewRegistry()
	registry.Register(NewGrepTool([]string{tmpDir}, 0))
	executor := NewExecutor(registry)

	result, err := executor.ExecuteJSON(context.Background(), "grep", `{"input":{"filePath":".","regex":"hello","maxResults":1}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	raw, ok := result.(string)
	if !ok {
		t.Fatalf("result type = %T, want string", result)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("failed to decode grep payload: %v", err)
	}
	if got := int(payload["count"].(float64)); got != 1 {
		t.Fatalf("count = %d, want 1", got)
	}
}

func TestExecutorExecuteJSONArgs_FindCamelCaseArgsFallback(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "sub", "note.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("failed to seed find target: %v", err)
	}

	registry := NewRegistry()
	registry.Register(NewFindTool([]string{tmpDir}))
	executor := NewExecutor(registry)

	result, err := executor.ExecuteJSON(context.Background(), "find", `{"input":{"filePath":".","glob":"*.txt","maxDepth":5,"fileType":"file"}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	raw, ok := result.(string)
	if !ok {
		t.Fatalf("result type = %T, want string", result)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("failed to decode find payload: %v", err)
	}
	if got := int(payload["count"].(float64)); got < 1 {
		t.Fatalf("count = %d, want >= 1", got)
	}
}

func TestExecutorExecuteJSONArgs_LsCamelCaseArgsFallback(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "sub", "note.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("failed to seed ls target: %v", err)
	}

	registry := NewRegistry()
	registry.Register(NewLsTool([]string{tmpDir}))
	executor := NewExecutor(registry)

	result, err := executor.ExecuteJSON(context.Background(), "ls", `{"input":{"filePath":".","maxDepth":5,"includeHidden":false,"maxEntries":1}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	raw, ok := result.(string)
	if !ok {
		t.Fatalf("result type = %T, want string", result)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("failed to decode ls payload: %v", err)
	}
	if got := int(payload["max_entries"].(float64)); got != 1 {
		t.Fatalf("max_entries = %d, want 1", got)
	}
	if got := int(payload["count"].(float64)); got != 1 {
		t.Fatalf("count = %d, want 1", got)
	}
	if got, _ := payload["truncated"].(bool); !got {
		t.Fatalf("truncated = %v, want true", payload["truncated"])
	}
}

func TestExecutorExecuteJSONArgs_WebSearchNestedCamelCaseFallback(t *testing.T) {
	registry := NewRegistry()
	tool := &captureArgsTool{def: ToolDefinition{Name: "web_search", Description: "search"}}
	registry.Register(tool)

	executor := NewExecutor(registry)
	_, err := executor.ExecuteJSON(context.Background(), "web_search", `{"input":{"query":"ZimaOS","maxResults":7,"region":"us-en","format":"json"}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := tool.args["query"]; got != "ZimaOS" {
		t.Fatalf("query = %v, want %q", got, "ZimaOS")
	}
	if got := tool.args["max_results"]; got != 7 {
		t.Fatalf("max_results = %v, want %d", got, 7)
	}
	if got := tool.args["region"]; got != "us-en" {
		t.Fatalf("region = %v, want %q", got, "us-en")
	}
	if got := tool.args["format"]; got != "json" {
		t.Fatalf("format = %v, want %q", got, "json")
	}
}

func TestExecutorExecuteJSONArgs_BrowserNestedCamelCaseFallback(t *testing.T) {
	registry := NewRegistry()
	tool := &captureArgsTool{def: ToolDefinition{Name: "browser", Description: "browser"}}
	registry.Register(tool)

	executor := NewExecutor(registry)
	_, err := executor.ExecuteJSON(context.Background(), "browser", `{"input":{"action":"act","ref":3,"actType":"click","targetId":"tab_1"}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := tool.args["action"]; got != "act" {
		t.Fatalf("action = %v, want %q", got, "act")
	}
	if got := tool.args["act_type"]; got != "click" {
		t.Fatalf("act_type = %v, want %q", got, "click")
	}
	if got := tool.args["target_id"]; got != "tab_1" {
		t.Fatalf("target_id = %v, want %q", got, "tab_1")
	}
	if got := tool.args["ref"]; got != float64(3) {
		t.Fatalf("ref = %v, want %v", got, float64(3))
	}
}

func TestExecutorExecuteJSONArgs_BrowserLegacyTopLevelActionFallback(t *testing.T) {
	registry := NewRegistry()
	tool := &captureArgsTool{def: ToolDefinition{Name: "browser", Description: "browser"}}
	registry.Register(tool)

	executor := NewExecutor(registry)
	_, err := executor.ExecuteJSON(context.Background(), "browser", `{"input":{"action":"scroll","ref":3,"targetId":"tab_1"}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := tool.args["action"]; got != "act" {
		t.Fatalf("action = %v, want %q", got, "act")
	}
	if got := tool.args["act_type"]; got != "scroll" {
		t.Fatalf("act_type = %v, want %q", got, "scroll")
	}
	if got := tool.args["target_id"]; got != "tab_1" {
		t.Fatalf("target_id = %v, want %q", got, "tab_1")
	}
	if got := tool.args["ref"]; got != float64(3) {
		t.Fatalf("ref = %v, want %v", got, float64(3))
	}
}

func TestExecutorExecuteJSONArgs_WebFetchNestedCamelCaseFallback(t *testing.T) {
	registry := NewRegistry()
	tool := &captureArgsTool{def: ToolDefinition{Name: "web_fetch", Description: "fetch"}}
	registry.Register(tool)

	executor := NewExecutor(registry)
	_, err := executor.ExecuteJSON(context.Background(), "web_fetch", `{"input":{"url":"https://example.com","extractMode":"text","maxChars":321,"requestHeaders":{"X-Test":"1"},"authBearer":"tok","browserTargetId":"tab_1"}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := tool.args["url"]; got != "https://example.com" {
		t.Fatalf("url = %v, want %q", got, "https://example.com")
	}
	if got := tool.args["extract_mode"]; got != "text" {
		t.Fatalf("extract_mode = %v, want %q", got, "text")
	}
	if got := tool.args["max_chars"]; got != 321 {
		t.Fatalf("max_chars = %v, want %d", got, 321)
	}
	headers, ok := tool.args["headers"].(map[string]interface{})
	if !ok || headers["X-Test"] != "1" {
		t.Fatalf("headers = %#v, want X-Test=1", tool.args["headers"])
	}
	if got := tool.args["auth_bearer"]; got != "tok" {
		t.Fatalf("auth_bearer = %v, want %q", got, "tok")
	}
	if got := tool.args["browser_target_id"]; got != "tab_1" {
		t.Fatalf("browser_target_id = %v, want %q", got, "tab_1")
	}
}

func TestExecutorExecuteJSONArgs_LooseJSONObjectInsideJSONStringFallback(t *testing.T) {
	registry := NewRegistry()
	tool := &captureArgsTool{
		def: ToolDefinition{
			Name:        "exec",
			Description: "Execute command",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{"type": "string"},
				},
				"required": []string{"command"},
			},
		},
	}
	registry.Register(tool)

	executor := NewExecutor(registry)
	_, err := executor.ExecuteJSON(context.Background(), "exec", `"{command:'pwd'}"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := tool.args["command"]; got != "pwd" {
		t.Fatalf("command = %v, want %q", got, "pwd")
	}
}

func TestExecutorExecuteJSONArgs_CodeFenceJSONFallback(t *testing.T) {
	registry := NewRegistry()
	tool := &captureArgsTool{
		def: ToolDefinition{
			Name:        "exec",
			Description: "Execute command",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{"type": "string"},
				},
				"required": []string{"command"},
			},
		},
	}
	registry.Register(tool)

	executor := NewExecutor(registry)
	args := "```json\n{\"command\":\"pwd\"}\n```"
	_, err := executor.ExecuteJSON(context.Background(), "exec", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := tool.args["command"]; got != "pwd" {
		t.Fatalf("command = %v, want %q", got, "pwd")
	}
}

func TestExecutorExecuteJSONArgs_ExtractJSONObjectFromText(t *testing.T) {
	registry := NewRegistry()
	tool := &captureArgsTool{
		def: ToolDefinition{
			Name:        "exec",
			Description: "Execute command",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{"type": "string"},
				},
				"required": []string{"command"},
			},
		},
	}
	registry.Register(tool)

	executor := NewExecutor(registry)
	args := `Use this payload: {"command":"ls -la"} thanks`
	_, err := executor.ExecuteJSON(context.Background(), "exec", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := tool.args["command"]; got != "ls -la" {
		t.Fatalf("command = %v, want %q", got, "ls -la")
	}
}

func TestExecutorExecuteJSONArgs_UsesLastConcatenatedJSONObject(t *testing.T) {
	registry := NewRegistry()
	tool := &captureArgsTool{
		def: ToolDefinition{
			Name:        "exec",
			Description: "Execute command",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{"type": "string"},
				},
				"required": []string{"command"},
			},
		},
	}
	registry.Register(tool)

	executor := NewExecutor(registry)
	_, err := executor.ExecuteJSON(context.Background(), "exec", `{}{"command":"pwd"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := tool.args["command"]; got != "pwd" {
		t.Fatalf("command = %v, want %q", got, "pwd")
	}
}

func TestExecutorExecuteJSONArgs_ColonValueLinesFallback(t *testing.T) {
	registry := NewRegistry()
	tool := &captureArgsTool{
		def: ToolDefinition{
			Name:        "exec",
			Description: "Execute command",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{"type": "string"},
					"cwd":     map[string]interface{}{"type": "string"},
				},
				"required": []string{"command"},
			},
		},
	}
	registry.Register(tool)

	executor := NewExecutor(registry)
	args := "command: ls -la\ncwd: /tmp"
	_, err := executor.ExecuteJSON(context.Background(), "exec", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := tool.args["command"]; got != "ls -la" {
		t.Fatalf("command = %v, want %q", got, "ls -la")
	}
	if got := tool.args["cwd"]; got != "/tmp" {
		t.Fatalf("cwd = %v, want %q", got, "/tmp")
	}
}

// Test Executor Execute nonexistent tool
func TestExecutorExecuteNonexistent(t *testing.T) {
	registry := NewRegistry()
	executor := NewExecutor(registry)

	_, err := executor.Execute(context.Background(), "nonexistent", map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for nonexistent tool")
	}
	if !errors.Is(err, ErrToolNotFound) {
		t.Errorf("expected ErrToolNotFound, got %v", err)
	}
}

// Test Executor context cancellation
func TestExecutorContextCancellation(t *testing.T) {
	registry := NewRegistry()
	tool := NewMockTool("slow_tool", "A slow tool")
	tool.SetResult("should not return")
	tool.SetDelay(true) // Make tool wait for context
	registry.Register(tool)

	executor := NewExecutor(registry)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := executor.Execute(ctx, "slow_tool", map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error due to cancelled context")
	}
}

// Test FileRead tool
func TestFileReadTool(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	testFile := tmpDir + "/test.txt"
	testContent := "Hello, World!"
	if err := writeTestFile(testFile, testContent); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := NewFileReadTool([]string{tmpDir}, 0)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"path": testFile,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resultMap map[string]interface{}
	if err := json.Unmarshal([]byte(result.(string)), &resultMap); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	if resultMap["content"] != testContent {
		t.Errorf("expected content '%s', got '%s'", testContent, resultMap["content"])
	}
}

func TestFileReadToolAddsTabularSummaryForCSV(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "sales.csv")
	testContent := strings.Join([]string{
		"Region,Product,Revenue,Cost",
		"East,Widget B,3000,1800",
		"West,Widget A,1250,750",
		"East,Widget B,1800,1080",
	}, "\n")
	if err := writeTestFile(testFile, testContent); err != nil {
		t.Fatalf("failed to create csv file: %v", err)
	}

	tool := NewFileReadTool([]string{tmpDir}, 0)
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"path": testFile,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resultMap map[string]interface{}
	if err := json.Unmarshal([]byte(result.(string)), &resultMap); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	rawSummary, ok := resultMap["tabular_summary"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected tabular_summary, got %#v", resultMap["tabular_summary"])
	}
	numericTotals, ok := rawSummary["numeric_totals"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected numeric_totals, got %#v", rawSummary["numeric_totals"])
	}
	if got := numericTotals["Revenue"]; got != float64(6050) {
		t.Fatalf("Revenue total = %v, want 6050", got)
	}
	if got := numericTotals["Profit"]; got != float64(2420) {
		t.Fatalf("Profit total = %v, want 2420", got)
	}
	highlights, ok := rawSummary["highlights"].([]interface{})
	if !ok || len(highlights) == 0 {
		t.Fatalf("expected highlights, got %#v", rawSummary["highlights"])
	}
	if !strings.Contains(fmt.Sprint(highlights), "Top Revenue by Region: East (4,800)") {
		t.Fatalf("unexpected highlights: %#v", highlights)
	}
}

func TestFileReadToolDelegatesPDFToPDFService(t *testing.T) {
	path := writeTestPDF(t, "report.pdf", 256)
	svc := &stubPDFService{extract: pdfextract.ExtractResult{Text: "hello pdf", Document: pdfextract.DocumentInfo{FileName: "report.pdf"}}}
	tool := NewFileReadTool([]string{filepath.Dir(path)}, 0)
	tool.SetPDFService(svc)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"path":      path,
		"page":      2,
		"max_chars": 5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	payload, ok := result.(pdfextract.ExtractResult)
	if !ok {
		t.Fatalf("result type = %T, want pdfextract.ExtractResult", result)
	}
	if payload.Text != "hello pdf" {
		t.Fatalf("text = %q, want hello pdf", payload.Text)
	}
	if svc.lastExtractReq.Path != path {
		t.Fatalf("extract path = %q, want %q", svc.lastExtractReq.Path, path)
	}
	if len(svc.lastExtractReq.Pages) != 1 || svc.lastExtractReq.Pages[0] != 2 {
		t.Fatalf("pages = %#v, want [2]", svc.lastExtractReq.Pages)
	}
	if svc.lastExtractReq.MaxChars != 5 {
		t.Fatalf("max_chars = %d, want 5", svc.lastExtractReq.MaxChars)
	}
}

func TestFileReadToolReadsXLSXDocuments(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "budget.xlsx")
	writeTestSpreadsheetXLSX(t, path)

	tool := NewFileReadTool([]string{tmpDir}, 0)
	result, err := tool.Execute(context.Background(), map[string]interface{}{"path": path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(result.(string)), &payload); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if payload["document_format"] != "xlsx" {
		t.Fatalf("document_format = %v, want xlsx", payload["document_format"])
	}
	if payload["extracted_via"] != "local_spreadsheet" {
		t.Fatalf("extracted_via = %v, want local_spreadsheet", payload["extracted_via"])
	}
	if !strings.Contains(payload["content"].(string), "Sheet: Budget") {
		t.Fatalf("content = %q, want spreadsheet sheet header", payload["content"])
	}
	summary, ok := payload["tabular_summary"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected tabular_summary, got %#v", payload["tabular_summary"])
	}
	sheets, ok := summary["sheet_summaries"].([]interface{})
	if !ok || len(sheets) != 1 {
		t.Fatalf("sheet_summaries = %#v, want one summary", summary["sheet_summaries"])
	}
}

func TestFileReadToolReadsOfficeDocumentsViaDocumentReader(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "notes.docx")
	if err := writeTestFile(path, "stub"); err != nil {
		t.Fatalf("failed to create docx placeholder: %v", err)
	}

	tool := NewFileReadTool([]string{tmpDir}, 0)
	reader := &stubDocumentReadService{
		result: &convertpkg.DocumentReadResult{
			Format:       "docx",
			Text:         "Title\nBody",
			ExtractedVia: "stub:txt",
		},
	}
	tool.SetDocumentReadService(reader)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"path":       path,
		"start_line": 2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(result.(string)), &payload); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if payload["content"] != "Body" {
		t.Fatalf("content = %q, want Body", payload["content"])
	}
	if payload["document_format"] != "docx" {
		t.Fatalf("document_format = %v, want docx", payload["document_format"])
	}
	if payload["extracted_via"] != "stub:txt" {
		t.Fatalf("extracted_via = %v, want stub:txt", payload["extracted_via"])
	}
	if reader.lastPath != path {
		t.Fatalf("reader path = %q, want %q", reader.lastPath, path)
	}
}

func TestFileReadToolReadsExpandedOfficeFormatsViaDocumentReader(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "notes.odt")
	if err := writeTestFile(path, "stub"); err != nil {
		t.Fatalf("failed to create odt placeholder: %v", err)
	}

	tool := NewFileReadTool([]string{tmpDir}, 0)
	reader := &stubDocumentReadService{
		result: &convertpkg.DocumentReadResult{
			Format:       "odt",
			Text:         "Meeting notes",
			ExtractedVia: "stub:txt",
		},
	}
	tool.SetDocumentReadService(reader)

	result, err := tool.Execute(context.Background(), map[string]interface{}{"path": path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(result.(string)), &payload); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if payload["document_format"] != "odt" {
		t.Fatalf("document_format = %v, want odt", payload["document_format"])
	}
	if reader.lastPath != path {
		t.Fatalf("reader path = %q, want %q", reader.lastPath, path)
	}
}

func TestFileReadToolReturnsDocumentRuntimeErrorWhenReaderUnavailable(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "slides.pptx")
	if err := writeTestFile(path, "stub"); err != nil {
		t.Fatalf("failed to create pptx placeholder: %v", err)
	}

	tool := NewFileReadTool([]string{tmpDir}, 0)
	tool.SetDocumentReadService(nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{"path": path})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "document extraction is unavailable") {
		t.Fatalf("error = %v, want unavailable message", err)
	}
}

// Test FileRead tool with allowed paths
func TestFileReadToolAllowedPaths(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := tmpDir + "/test.txt"
	if err := writeTestFile(testFile, "test"); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Tool with restricted paths
	tool := NewFileReadTool([]string{tmpDir}, 0)

	// Should succeed for allowed path
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"path": testFile,
	})
	if err != nil {
		t.Errorf("expected success for allowed path, got error: %v", err)
	}

	// Should fail for disallowed path
	_, err = tool.Execute(context.Background(), map[string]interface{}{
		"path": "/etc/passwd",
	})
	if err == nil {
		t.Error("expected error for disallowed path")
	}
}

func TestFileReadToolFSRootOverride(t *testing.T) {
	baseDir := t.TempDir()
	runDir := filepath.Join(baseDir, "run")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	if err := writeTestFile(filepath.Join(baseDir, "outer.txt"), "outer"); err != nil {
		t.Fatalf("write outer file: %v", err)
	}
	if err := writeTestFile(filepath.Join(runDir, "inner.txt"), "inner"); err != nil {
		t.Fatalf("write inner file: %v", err)
	}

	tool := NewFileReadTool([]string{baseDir}, 0)
	ctx := WithFSRootOverride(context.Background(), []string{runDir}, map[string]string{"workspace": runDir})

	result, err := tool.Execute(ctx, map[string]interface{}{
		"path": "inner.txt",
	})
	if err != nil {
		t.Fatalf("expected inner.txt to resolve within override root: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(result.(string)), &payload); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	if payload["content"] != "inner" {
		t.Fatalf("content = %q, want inner", payload["content"])
	}

	if _, err := tool.Execute(ctx, map[string]interface{}{"path": "../outer.txt"}); err == nil {
		t.Fatal("expected override root to reject parent traversal")
	}
}

// Test FileRead tool file not found
func TestFileReadToolNotFound(t *testing.T) {
	tool := NewFileReadTool(nil, 0)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"path": "/nonexistent/file.txt",
	})
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestFileReadToolAcceptsFilenameAlias(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "filename-read.txt")
	if err := writeTestFile(target, "hello"); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	tool := NewFileReadTool([]string{tmpDir}, 0)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"filename": target,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(result.(string)), &payload); err != nil {
		t.Fatalf("failed to decode read result: %v", err)
	}
	if got := payload["content"]; got != "hello" {
		t.Fatalf("content = %v, want %q", got, "hello")
	}
}

// Test FileRead tool missing path
func TestFileReadToolMissingPath(t *testing.T) {
	tool := NewFileReadTool(nil, 0)

	_, err := tool.Execute(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("expected error for missing path")
	}
}

// Test FileRead tool file too large
func TestFileReadToolFileTooLarge(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := tmpDir + "/large.txt"
	// Create a file larger than the limit
	largeContent := make([]byte, 1024)
	for i := range largeContent {
		largeContent[i] = 'x'
	}
	if err := writeTestFile(testFile, string(largeContent)); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Tool with small max size
	tool := NewFileReadTool(nil, 100)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"path": testFile,
	})
	if err == nil {
		t.Error("expected error for file too large")
	}
}

// Test FileWrite tool
func TestFileWriteTool(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := tmpDir + "/output.txt"
	testContent := "Hello, World!"

	tool := NewFileWriteTool([]string{tmpDir}, 0)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"path":    testFile,
		"content": testContent,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resultMap map[string]interface{}
	if err := json.Unmarshal([]byte(result.(string)), &resultMap); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	if resultMap["success"] != true {
		t.Error("expected success=true")
	}

	// Verify file content
	content, err := readTestFile(testFile)
	if err != nil {
		t.Fatalf("failed to read test file: %v", err)
	}
	if content != testContent {
		t.Errorf("expected content '%s', got '%s'", testContent, content)
	}
}

// Test FileWrite tool append mode
func TestFileWriteToolAppend(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := tmpDir + "/append.txt"

	tool := NewFileWriteTool([]string{tmpDir}, 0)

	// Write initial content
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"path":    testFile,
		"content": "Hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Append more content
	_, err = tool.Execute(context.Background(), map[string]interface{}{
		"path":    testFile,
		"content": ", World!",
		"append":  true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file content
	content, err := readTestFile(testFile)
	if err != nil {
		t.Fatalf("failed to read test file: %v", err)
	}
	if content != "Hello, World!" {
		t.Errorf("expected content 'Hello, World!', got '%s'", content)
	}
}

func TestFileWriteToolAcceptsFilenameAlias(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "filename-write.txt")
	tool := NewFileWriteTool([]string{tmpDir}, 0)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"filename": target,
		"content":  "hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := readTestFile(target)
	if err != nil {
		t.Fatalf("failed to read test file: %v", err)
	}
	if content != "hello" {
		t.Fatalf("content = %q, want %q", content, "hello")
	}
}

func TestFileDeleteTool(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "delete-me.txt")
	if err := writeTestFile(target, "bye"); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := NewFileDeleteTool([]string{tmpDir})
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"path": target,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resultMap map[string]interface{}
	if err := json.Unmarshal([]byte(result.(string)), &resultMap); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if resultMap["success"] != true || resultMap["deleted"] != true {
		t.Fatalf("unexpected delete result: %+v", resultMap)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("expected target to be deleted, stat err=%v", err)
	}
}

func TestFileDeleteToolRecursiveDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "nested")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("failed to create test dir: %v", err)
	}
	if err := writeTestFile(filepath.Join(targetDir, "note.txt"), "bye"); err != nil {
		t.Fatalf("failed to create nested file: %v", err)
	}

	tool := NewFileDeleteTool([]string{tmpDir})
	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"path":      targetDir,
		"recursive": true,
	}); err != nil {
		t.Fatalf("unexpected recursive delete error: %v", err)
	}

	if _, err := os.Stat(targetDir); !os.IsNotExist(err) {
		t.Fatalf("expected directory to be deleted, stat err=%v", err)
	}
}

func TestTransactionalWriteToolsCommit(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "nested", "large.txt")
	manager := NewWriteSessionManager(0)
	beginTool := NewFileWriteBeginTool([]string{tmpDir}, manager)
	chunkTool := NewFileWriteChunkTool(manager)
	commitTool := NewFileWriteCommitTool(manager)

	beginResult, err := beginTool.Execute(context.Background(), map[string]interface{}{
		"path": target,
	})
	if err != nil {
		t.Fatalf("write_begin failed: %v", err)
	}

	var beginPayload map[string]interface{}
	if err := json.Unmarshal([]byte(beginResult.(string)), &beginPayload); err != nil {
		t.Fatalf("parse write_begin payload: %v", err)
	}
	sessionID, _ := beginPayload["session_id"].(string)
	if strings.TrimSpace(sessionID) == "" {
		t.Fatalf("write_begin returned empty session_id: %#v", beginPayload)
	}

	chunks := []string{"Hello", ", ", "transactional world!"}
	for _, chunk := range chunks {
		if _, err := chunkTool.Execute(context.Background(), map[string]interface{}{
			"session_id": sessionID,
			"content":    chunk,
		}); err != nil {
			t.Fatalf("write_chunk failed for %q: %v", chunk, err)
		}
	}

	want := strings.Join(chunks, "")
	wantSHA := fmt.Sprintf("%x", sha256.Sum256([]byte(want)))
	commitResult, err := commitTool.Execute(context.Background(), map[string]interface{}{
		"session_id":      sessionID,
		"expected_bytes":  len(want),
		"expected_sha256": wantSHA,
	})
	if err != nil {
		t.Fatalf("write_commit failed: %v", err)
	}

	var commitPayload map[string]interface{}
	if err := json.Unmarshal([]byte(commitResult.(string)), &commitPayload); err != nil {
		t.Fatalf("parse write_commit payload: %v", err)
	}
	if got, _ := commitPayload["sha256"].(string); got != wantSHA {
		t.Fatalf("write_commit sha256 = %q, want %q", got, wantSHA)
	}

	got, err := readTestFile(target)
	if err != nil {
		t.Fatalf("read committed file: %v", err)
	}
	if got != want {
		t.Fatalf("committed file content = %q, want %q", got, want)
	}
}

func TestTransactionalWriteToolsRejectOversizedChunk(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewWriteSessionManager(0)
	beginTool := NewFileWriteBeginTool([]string{tmpDir}, manager)
	chunkTool := NewFileWriteChunkTool(manager)

	beginResult, err := beginTool.Execute(context.Background(), map[string]interface{}{
		"path": filepath.Join(tmpDir, "oversized.txt"),
	})
	if err != nil {
		t.Fatalf("write_begin failed: %v", err)
	}

	var beginPayload map[string]interface{}
	if err := json.Unmarshal([]byte(beginResult.(string)), &beginPayload); err != nil {
		t.Fatalf("parse write_begin payload: %v", err)
	}
	sessionID, _ := beginPayload["session_id"].(string)

	_, err = chunkTool.Execute(context.Background(), map[string]interface{}{
		"session_id": sessionID,
		"content":    strings.Repeat("x", maxFileWriteChunkBytes+1),
	})
	if err == nil {
		t.Fatal("expected oversized write_chunk to fail")
	}
	if !strings.Contains(err.Error(), "write_chunk") && !strings.Contains(err.Error(), "smaller chunks") {
		t.Fatalf("expected chunk guidance error, got %v", err)
	}
}

func TestTransactionalWriteToolsAbort(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewWriteSessionManager(0)
	beginTool := NewFileWriteBeginTool([]string{tmpDir}, manager)
	chunkTool := NewFileWriteChunkTool(manager)
	abortTool := NewFileWriteAbortTool(manager)

	beginResult, err := beginTool.Execute(context.Background(), map[string]interface{}{
		"path": filepath.Join(tmpDir, "abort.txt"),
	})
	if err != nil {
		t.Fatalf("write_begin failed: %v", err)
	}

	var beginPayload map[string]interface{}
	if err := json.Unmarshal([]byte(beginResult.(string)), &beginPayload); err != nil {
		t.Fatalf("parse write_begin payload: %v", err)
	}
	sessionID, _ := beginPayload["session_id"].(string)

	if _, err := chunkTool.Execute(context.Background(), map[string]interface{}{
		"session_id": sessionID,
		"content":    "partial",
	}); err != nil {
		t.Fatalf("write_chunk failed: %v", err)
	}

	if _, err := abortTool.Execute(context.Background(), map[string]interface{}{
		"session_id": sessionID,
	}); err != nil {
		t.Fatalf("write_abort failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "abort.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected no committed file after abort, stat err=%v", err)
	}
}

func TestTransactionalWriteToolsFallbackToSingleActiveSession(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "single-session.txt")
	manager := NewWriteSessionManager(0)
	beginTool := NewFileWriteBeginTool([]string{tmpDir}, manager)
	chunkTool := NewFileWriteChunkTool(manager)
	commitTool := NewFileWriteCommitTool(manager)

	if _, err := beginTool.Execute(context.Background(), map[string]interface{}{
		"path": target,
	}); err != nil {
		t.Fatalf("write_begin failed: %v", err)
	}

	if _, err := chunkTool.Execute(context.Background(), map[string]interface{}{
		"content": "hello fallback",
	}); err != nil {
		t.Fatalf("write_chunk without session_id failed: %v", err)
	}

	commitResult, err := commitTool.Execute(context.Background(), map[string]interface{}{
		"expected_bytes": len("hello fallback"),
	})
	if err != nil {
		t.Fatalf("write_commit without session_id failed: %v", err)
	}

	var commitPayload map[string]interface{}
	if err := json.Unmarshal([]byte(commitResult.(string)), &commitPayload); err != nil {
		t.Fatalf("parse write_commit payload: %v", err)
	}
	if got, _ := commitPayload["size"].(float64); int(got) != len("hello fallback") {
		t.Fatalf("write_commit size = %v, want %d", commitPayload["size"], len("hello fallback"))
	}

	got, err := readTestFile(target)
	if err != nil {
		t.Fatalf("read committed file: %v", err)
	}
	if got != "hello fallback" {
		t.Fatalf("committed file content = %q, want %q", got, "hello fallback")
	}
}

func TestTransactionalWriteToolsMissingSessionIDAmbiguousWithMultipleActiveSessions(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewWriteSessionManager(0)
	beginTool := NewFileWriteBeginTool([]string{tmpDir}, manager)
	chunkTool := NewFileWriteChunkTool(manager)

	targets := []string{
		filepath.Join(tmpDir, "first.txt"),
		filepath.Join(tmpDir, "second.txt"),
	}
	for _, target := range targets {
		if _, err := beginTool.Execute(context.Background(), map[string]interface{}{
			"path": target,
		}); err != nil {
			t.Fatalf("write_begin failed for %q: %v", target, err)
		}
	}

	_, err := chunkTool.Execute(context.Background(), map[string]interface{}{
		"content": "ambiguous",
	})
	if err == nil {
		t.Fatal("expected missing session_id with multiple sessions to fail")
	}
	if !strings.Contains(err.Error(), "multiple active write sessions exist") {
		t.Fatalf("expected ambiguity guidance error, got %v", err)
	}
	if !strings.Contains(err.Error(), "first.txt") || !strings.Contains(err.Error(), "second.txt") {
		t.Fatalf("expected error to mention active session paths, got %v", err)
	}
}

func TestExecutorTransactionalWriteFlowWithCompatArgs(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "nested", "executor.txt")

	registry := NewRegistry()
	RegisterBuiltinToolsWithConfig(registry, WebSearchConfig{}, WebFetchConfig{}, []string{tmpDir}, 0)
	executor := NewExecutor(registry)

	beginArgs := fmt.Sprintf(`{"input":{"filePath":%q,"createDirs":true}}`, target)
	beginResult, err := executor.ExecuteJSON(context.Background(), "write_begin", beginArgs)
	if err != nil {
		t.Fatalf("executor write_begin failed: %v", err)
	}

	var beginPayload map[string]interface{}
	if err := json.Unmarshal([]byte(beginResult.(string)), &beginPayload); err != nil {
		t.Fatalf("parse write_begin payload: %v", err)
	}
	sessionID, _ := beginPayload["session_id"].(string)
	if strings.TrimSpace(sessionID) == "" {
		t.Fatalf("write_begin returned empty session_id: %#v", beginPayload)
	}

	chunks := []struct {
		name string
		args string
	}{
		{
			name: "arguments+text",
			args: fmt.Sprintf(`{"arguments":{"sessionId":%q,"text":%q}}`, sessionID, "Hello"),
		},
		{
			name: "payload+body",
			args: fmt.Sprintf(`{"payload":{"session_id":%q,"body":%q}}`, sessionID, " via "),
		},
		{
			name: "top-level content",
			args: fmt.Sprintf(`{"sessionId":%q,"content":%q}`, sessionID, "executor"),
		},
		{
			name: "input+chunk",
			args: fmt.Sprintf(`{"input":{"sessionId":%q,"chunk":%q}}`, sessionID, "!"),
		},
	}
	for _, chunk := range chunks {
		if _, err := executor.ExecuteJSON(context.Background(), "write_chunk", chunk.args); err != nil {
			t.Fatalf("executor write_chunk failed for %s: %v", chunk.name, err)
		}
	}

	want := "Hello via executor!"
	wantSHA := fmt.Sprintf("%x", sha256.Sum256([]byte(want)))
	commitArgs := fmt.Sprintf(`{"input":{"sessionId":%q,"expectedBytes":%d,"expectedSha256":%q}}`, sessionID, len(want), wantSHA)
	commitResult, err := executor.ExecuteJSON(context.Background(), "write_commit", commitArgs)
	if err != nil {
		t.Fatalf("executor write_commit failed: %v", err)
	}

	var commitPayload map[string]interface{}
	if err := json.Unmarshal([]byte(commitResult.(string)), &commitPayload); err != nil {
		t.Fatalf("parse write_commit payload: %v", err)
	}
	if got, _ := commitPayload["sha256"].(string); got != wantSHA {
		t.Fatalf("write_commit sha256 = %q, want %q", got, wantSHA)
	}

	got, err := readTestFile(target)
	if err != nil {
		t.Fatalf("read committed file: %v", err)
	}
	if got != want {
		t.Fatalf("committed file content = %q, want %q", got, want)
	}
}

func TestExecutorTransactionalWriteAbortWithCompatArgs(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "abort-via-executor.txt")

	registry := NewRegistry()
	RegisterBuiltinToolsWithConfig(registry, WebSearchConfig{}, WebFetchConfig{}, []string{tmpDir}, 0)
	executor := NewExecutor(registry)

	beginResult, err := executor.ExecuteJSON(context.Background(), "write_begin", fmt.Sprintf(`{"path":%q}`, target))
	if err != nil {
		t.Fatalf("executor write_begin failed: %v", err)
	}

	var beginPayload map[string]interface{}
	if err := json.Unmarshal([]byte(beginResult.(string)), &beginPayload); err != nil {
		t.Fatalf("parse write_begin payload: %v", err)
	}
	sessionID, _ := beginPayload["session_id"].(string)

	if _, err := executor.ExecuteJSON(context.Background(), "write_chunk", fmt.Sprintf(`{"input":{"sessionId":%q,"text":"partial"}}`, sessionID)); err != nil {
		t.Fatalf("executor write_chunk failed: %v", err)
	}
	if _, err := executor.ExecuteJSON(context.Background(), "write_abort", fmt.Sprintf(`{"arguments":{"sessionId":%q}}`, sessionID)); err != nil {
		t.Fatalf("executor write_abort failed: %v", err)
	}

	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("expected no committed file after abort, stat err=%v", err)
	}
}

func TestFileWriteToolContentCoercion(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "coerce.json")
	tool := NewFileWriteTool([]string{tmpDir}, 0)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"path":    testFile,
		"content": map[string]interface{}{"ok": true},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := readTestFile(testFile)
	if err != nil {
		t.Fatalf("failed to read test file: %v", err)
	}
	if content != `{"ok":true}` {
		t.Fatalf("expected JSON content, got %q", content)
	}

	_, err = tool.Execute(context.Background(), map[string]interface{}{
		"path":    testFile,
		"content": 42,
	})
	if err != nil {
		t.Fatalf("unexpected numeric coercion error: %v", err)
	}
	content, err = readTestFile(testFile)
	if err != nil {
		t.Fatalf("failed to read test file: %v", err)
	}
	if content != "42" {
		t.Fatalf("expected numeric content to be coerced to string, got %q", content)
	}
}

func TestFileWriteToolSupportsNestedCamelCaseArgs(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "nested-write.txt")
	tool := NewFileWriteTool([]string{tmpDir}, 0)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": map[string]interface{}{
			"filePath": target,
			"text":     "hello",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := readTestFile(target)
	if err != nil {
		t.Fatalf("failed to read test file: %v", err)
	}
	if content != "hello" {
		t.Fatalf("content = %q, want %q", content, "hello")
	}
}

func TestFileReadToolSupportsNestedCamelCaseArgs(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "nested-read.txt")
	if err := writeTestFile(target, "a\nb\nc"); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	tool := NewFileReadTool([]string{tmpDir}, 0)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": map[string]interface{}{
			"filePath":  target,
			"startLine": 2,
			"endLine":   2,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(result.(string)), &payload); err != nil {
		t.Fatalf("failed to decode read result: %v", err)
	}
	if got := payload["content"]; got != "b" {
		t.Fatalf("content = %v, want %q", got, "b")
	}
}

func TestEditToolSupportsNestedCamelCaseArgs(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "nested-edit.txt")
	if err := writeTestFile(target, "old old"); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	tool := NewEditTool([]string{tmpDir}, 0)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": map[string]interface{}{
			"filePath":   target,
			"oldText":    "old",
			"newText":    "new",
			"replaceAll": true,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := readTestFile(target)
	if err != nil {
		t.Fatalf("failed to read edited file: %v", err)
	}
	if content != "new new" {
		t.Fatalf("content = %q, want %q", content, "new new")
	}
}

// Test FileWrite tool with allowed paths
func TestFileWriteToolAllowedPaths(t *testing.T) {
	tmpDir := t.TempDir()

	// Tool with restricted paths
	tool := NewFileWriteTool([]string{tmpDir}, 0)

	// Should succeed for allowed path
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"path":    tmpDir + "/allowed.txt",
		"content": "test",
	})
	if err != nil {
		t.Errorf("expected success for allowed path, got error: %v", err)
	}

	// Should fail for disallowed path
	_, err = tool.Execute(context.Background(), map[string]interface{}{
		"path":    "/tmp/disallowed.txt",
		"content": "test",
	})
	if err == nil {
		t.Error("expected error for disallowed path")
	}
}

// Test FileWrite tool missing path
func TestFileWriteToolMissingPath(t *testing.T) {
	tool := NewFileWriteTool(nil, 0)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"content": "test",
	})
	if err == nil {
		t.Error("expected error for missing path")
	}
}

// Test FileWrite tool missing content
func TestFileWriteToolMissingContent(t *testing.T) {
	tool := NewFileWriteTool(nil, 0)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"path": "/tmp/test.txt",
	})
	if err == nil {
		t.Error("expected error for missing content")
	}
}

// Test FileWrite tool content too large
func TestFileWriteToolContentTooLarge(t *testing.T) {
	tmpDir := t.TempDir()
	largeContent := make([]byte, 1024)
	for i := range largeContent {
		largeContent[i] = 'x'
	}

	// Tool with small max size
	tool := NewFileWriteTool(nil, 100)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"path":    tmpDir + "/large.txt",
		"content": string(largeContent),
	})
	if err == nil {
		t.Error("expected error for content too large")
	}
}

func TestFileWriteToolChunkTooLargeSuggestsAppend(t *testing.T) {
	tmpDir := t.TempDir()
	largeContent := strings.Repeat("x", maxFileWriteChunkBytes+1)

	tool := NewFileWriteTool([]string{tmpDir}, 0)
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"path":    filepath.Join(tmpDir, "chunked.txt"),
		"content": largeContent,
	})
	if err == nil {
		t.Fatal("expected error for oversized write chunk")
	}
	if !strings.Contains(err.Error(), "append=true") {
		t.Fatalf("expected append guidance, got %v", err)
	}
	if !strings.Contains(err.Error(), "split into smaller chunks") {
		t.Fatalf("expected chunking guidance, got %v", err)
	}
}

func TestFileToolDefinitionsUseNewNames(t *testing.T) {
	if got := NewFileReadTool(nil, 0).Definition().Name; got != "file_read" {
		t.Fatalf("read tool name = %q, want file_read", got)
	}
	if got := NewFileWriteTool(nil, 0).Definition().Name; got != "file_write" {
		t.Fatalf("write tool name = %q, want file_write", got)
	}
	if got := NewFileDeleteTool(nil).Definition().Name; got != "file_delete" {
		t.Fatalf("delete tool name = %q, want file_delete", got)
	}
}

func TestExecutorLegacyToolNameRemap(t *testing.T) {
	registry := NewRegistry()
	readTool := NewMockTool("file_read", "Read")
	readTool.SetResult("ok")
	writeTool := NewMockTool("file_write", "Write")
	writeTool.SetResult("ok")
	deleteTool := NewMockTool("file_delete", "Delete")
	deleteTool.SetResult("ok")
	registry.Register(readTool)
	registry.Register(writeTool)
	registry.Register(deleteTool)

	executor := NewExecutor(registry)
	if _, err := executor.Execute(context.Background(), "read", map[string]interface{}{"path": "a.txt"}); err != nil {
		t.Fatalf("read compatibility execute failed: %v", err)
	}
	if _, err := executor.Execute(context.Background(), "write", map[string]interface{}{"path": "a.txt", "content": "x"}); err != nil {
		t.Fatalf("write compatibility execute failed: %v", err)
	}
	if _, err := executor.Execute(context.Background(), "delete", map[string]interface{}{"path": "a.txt"}); err != nil {
		t.Fatalf("delete compatibility execute failed: %v", err)
	}
}

func TestExecutorMemoryCompatToolNameRemapAndArgs(t *testing.T) {
	registry := NewRegistry()
	memoryTool := &captureArgsTool{
		def: ToolDefinition{
			Name:        "memory",
			Description: "Memory operations",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action":  map[string]interface{}{"type": "string"},
					"query":   map[string]interface{}{"type": "string"},
					"id":      map[string]interface{}{"type": "string"},
					"content": map[string]interface{}{"type": "string"},
					"tags": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
				},
				"required": []string{"action"},
			},
		},
	}
	registry.Register(memoryTool)

	executor := NewExecutor(registry)

	if _, err := executor.Execute(context.Background(), "memory_write", map[string]interface{}{
		"text":     "remember this",
		"category": "prefs",
	}); err != nil {
		t.Fatalf("memory_write compatibility execute failed: %v", err)
	}
	if got := memoryTool.args["action"]; got != "remember" {
		t.Fatalf("action = %v, want %q", got, "remember")
	}
	if got := memoryTool.args["content"]; got != "remember this" {
		t.Fatalf("content = %v, want %q", got, "remember this")
	}
	tags, ok := memoryTool.args["tags"].([]interface{})
	if !ok || len(tags) != 1 || tags[0] != "prefs" {
		t.Fatalf("tags = %v, want [prefs]", memoryTool.args["tags"])
	}

	if _, err := executor.Execute(context.Background(), "memory_search", map[string]interface{}{
		"q": "compat context",
	}); err != nil {
		t.Fatalf("memory_search compatibility execute failed: %v", err)
	}
	if got := memoryTool.args["action"]; got != "search" {
		t.Fatalf("action = %v, want %q", got, "search")
	}
	if got := memoryTool.args["query"]; got != "compat context" {
		t.Fatalf("query = %v, want %q", got, "compat context")
	}

	if _, err := executor.Execute(context.Background(), "memory_get", map[string]interface{}{
		"path": "memory/2026-03-05.md",
	}); err != nil {
		t.Fatalf("memory_get compatibility execute failed: %v", err)
	}
	if got := memoryTool.args["action"]; got != "get" {
		t.Fatalf("action = %v, want %q", got, "get")
	}
	if got := memoryTool.args["id"]; got != "memory/2026-03-05.md" {
		t.Fatalf("id = %v, want %q", got, "memory/2026-03-05.md")
	}
}

func TestExecutorWebFetchCompatNormalizesArgs(t *testing.T) {
	registry := NewRegistry()
	webFetchTool := &captureArgsTool{
		def: ToolDefinition{
			Name:        "web_fetch",
			Description: "Web fetch",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url":               map[string]interface{}{"type": "string"},
					"extract_mode":      map[string]interface{}{"type": "string"},
					"max_chars":         map[string]interface{}{"type": "integer"},
					"browser_target_id": map[string]interface{}{"type": "string"},
				},
			},
		},
	}
	registry.Register(webFetchTool)

	executor := NewExecutor(registry)
	if _, err := executor.Execute(context.Background(), "web_fetch", map[string]interface{}{
		"href":            "https://example.com/docs",
		"extractMode":     "text",
		"maxChars":        321,
		"browserTargetId": "tab-99",
	}); err != nil {
		t.Fatalf("web_fetch compatibility execute failed: %v", err)
	}
	if got := webFetchTool.args["url"]; got != "https://example.com/docs" {
		t.Fatalf("url = %v, want %q", got, "https://example.com/docs")
	}
	if got := webFetchTool.args["extract_mode"]; got != "text" {
		t.Fatalf("extract_mode = %v, want %q", got, "text")
	}
	if got := webFetchTool.args["max_chars"]; got != 321 {
		t.Fatalf("max_chars = %v, want %d", got, 321)
	}
	if got := webFetchTool.args["browser_target_id"]; got != "tab-99" {
		t.Fatalf("browser_target_id = %v, want %q", got, "tab-99")
	}
}

func TestExecutorWebSearchCompatQueryNormalization(t *testing.T) {
	registry := NewRegistry()
	webTool := &captureArgsTool{
		def: ToolDefinition{
			Name:        "web_search",
			Description: "Web search",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query":       map[string]interface{}{"type": "string"},
					"max_results": map[string]interface{}{"type": "integer"},
				},
			},
		},
	}
	registry.Register(webTool)

	executor := NewExecutor(registry)
	if _, err := executor.Execute(context.Background(), "web_search", map[string]interface{}{
		"q":     "blue compat routing",
		"limit": 7,
	}); err != nil {
		t.Fatalf("web_search compatibility execute failed: %v", err)
	}
	if got := webTool.args["query"]; got != "blue compat routing" {
		t.Fatalf("query = %v, want %q", got, "blue compat routing")
	}
	if got := webTool.args["max_results"]; got != 7 {
		t.Fatalf("max_results = %v, want %d", got, 7)
	}
}

func TestFileWriteToolLineModeSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "line.txt")
	if err := writeTestFile(target, "alpha\nbeta\ngamma\n"); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	tool := NewFileWriteTool([]string{tmpDir}, 0)
	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"path":    target,
		"content": "BETA",
		"line":    2,
	}); err != nil {
		t.Fatalf("line mode write failed: %v", err)
	}

	got, err := readTestFile(target)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if got != "alpha\nBETA\ngamma\n" {
		t.Fatalf("line mode content = %q, want %q", got, "alpha\nBETA\ngamma\n")
	}
}

func TestFileWriteToolLineModeOutOfRange(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "line.txt")
	if err := writeTestFile(target, "only one line\n"); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	tool := NewFileWriteTool([]string{tmpDir}, 0)
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"path":    target,
		"content": "x",
		"line":    3,
	})
	if err == nil {
		t.Fatal("expected out-of-range error")
	}
	if got, want := err.Error(), "line 3 out of range (total lines: 1)"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestFileWriteToolLineModeMultiLineContent(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "line.txt")
	if err := writeTestFile(target, "alpha\nbeta\ngamma\n"); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	tool := NewFileWriteTool([]string{tmpDir}, 0)
	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"path":    target,
		"content": "BETA-1\nBETA-2",
		"line":    2,
	}); err != nil {
		t.Fatalf("line mode multi-line write failed: %v", err)
	}

	got, err := readTestFile(target)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if got != "alpha\nBETA-1\nBETA-2\ngamma\n" {
		t.Fatalf("line mode multi-line content = %q, want %q", got, "alpha\nBETA-1\nBETA-2\ngamma\n")
	}
}

func TestFileWriteToolLineModeAppendConflict(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "line.txt")
	if err := writeTestFile(target, "hello\n"); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	tool := NewFileWriteTool([]string{tmpDir}, 0)
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"path":    target,
		"content": "x",
		"line":    1,
		"append":  true,
	})
	if err == nil || !strings.Contains(err.Error(), "line mode cannot be used with append=true") {
		t.Fatalf("expected append conflict error, got %v", err)
	}
}

func TestFileWriteToolLineModeMissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "new-line.txt")
	tool := NewFileWriteTool([]string{tmpDir}, 0)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"path":    target,
		"content": "first",
		"line":    1,
	}); err != nil {
		t.Fatalf("line mode create first line failed: %v", err)
	}
	got, err := readTestFile(target)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if got != "first" {
		t.Fatalf("content = %q, want %q", got, "first")
	}

	_, err = tool.Execute(context.Background(), map[string]interface{}{
		"path":    filepath.Join(tmpDir, "missing.txt"),
		"content": "x",
		"line":    2,
	})
	if err == nil {
		t.Fatal("expected out-of-range error for missing file line>1")
	}
	if got, want := err.Error(), "line 2 out of range (total lines: 0)"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestEditTool(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "edit.txt")
	if err := writeTestFile(target, "hello world"); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	tool := NewEditTool([]string{tmpDir}, 0)
	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"path":     target,
		"old_text": "world",
		"new_text": "blue",
	}); err != nil {
		t.Fatalf("edit failed: %v", err)
	}

	got, err := readTestFile(target)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if got != "hello blue" {
		t.Fatalf("content = %q, want %q", got, "hello blue")
	}
}

func TestGrepTool(t *testing.T) {
	tmpDir := t.TempDir()
	if err := writeTestFile(filepath.Join(tmpDir, "a.txt"), "hello\nworld\n"); err != nil {
		t.Fatalf("write a.txt: %v", err)
	}
	if err := writeTestFile(filepath.Join(tmpDir, "b.txt"), "HELLO again\n"); err != nil {
		t.Fatalf("write b.txt: %v", err)
	}

	tool := NewGrepTool([]string{tmpDir}, 0)
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"pattern": "hello",
		"path":    ".",
	})
	if err != nil {
		t.Fatalf("grep failed: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(result.(string)), &payload); err != nil {
		t.Fatalf("decode grep result: %v", err)
	}
	if count, _ := payload["count"].(float64); count < 2 {
		t.Fatalf("grep count = %v, want >= 2", payload["count"])
	}
	if got, _ := payload["backend"].(string); got != "builtin" {
		t.Fatalf("grep backend = %q, want builtin", got)
	}
}

func TestFindTool(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	if err := writeTestFile(filepath.Join(tmpDir, "sub", "note.txt"), "x"); err != nil {
		t.Fatalf("write note.txt: %v", err)
	}

	tool := NewFindTool([]string{tmpDir})
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"path":      ".",
		"pattern":   "*.txt",
		"type":      "file",
		"max_depth": 5,
	})
	if err != nil {
		t.Fatalf("find failed: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(result.(string)), &payload); err != nil {
		t.Fatalf("decode find result: %v", err)
	}
	if count, _ := payload["count"].(float64); count < 1 {
		t.Fatalf("find count = %v, want >= 1", payload["count"])
	}
	if got, _ := payload["backend"].(string); got != "builtin" {
		t.Fatalf("find backend = %q, want builtin", got)
	}
}

func TestRgTool(t *testing.T) {
	tmpDir := t.TempDir()
	if err := writeTestFile(filepath.Join(tmpDir, "a.txt"), "hello from rg\n"); err != nil {
		t.Fatalf("write a.txt: %v", err)
	}

	tool := NewRgTool([]string{tmpDir}, 0)
	if got := tool.Definition().Name; got != "rg" {
		t.Fatalf("definition name = %q, want rg", got)
	}
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"pattern": "hello",
		"path":    ".",
	})
	if err != nil {
		t.Fatalf("rg failed: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(result.(string)), &payload); err != nil {
		t.Fatalf("decode rg result: %v", err)
	}
	if got, _ := payload["backend"].(string); got != "builtin" {
		t.Fatalf("rg backend = %q, want builtin", got)
	}
}

func TestLsTool(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	if err := writeTestFile(filepath.Join(tmpDir, "sub", "note.txt"), "x"); err != nil {
		t.Fatalf("write note.txt: %v", err)
	}

	tool := NewLsTool([]string{tmpDir})
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"path":      ".",
		"max_depth": 5,
	})
	if err != nil {
		t.Fatalf("ls failed: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(result.(string)), &payload); err != nil {
		t.Fatalf("decode ls result: %v", err)
	}
	if count, _ := payload["count"].(float64); count < 1 {
		t.Fatalf("ls count = %v, want >= 1", payload["count"])
	}
}

func TestLsTool_DefaultsToCurrentDirectoryOnly(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	if err := writeTestFile(filepath.Join(tmpDir, "root.txt"), "root"); err != nil {
		t.Fatalf("write root.txt: %v", err)
	}
	if err := writeTestFile(filepath.Join(tmpDir, "sub", "note.txt"), "nested"); err != nil {
		t.Fatalf("write sub/note.txt: %v", err)
	}

	tool := NewLsTool([]string{tmpDir})
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"path": ".",
	})
	if err != nil {
		t.Fatalf("ls failed: %v", err)
	}

	var payload struct {
		Count    int `json:"count"`
		MaxDepth int `json:"max_depth"`
		Entries  []struct {
			Path string `json:"path"`
			Type string `json:"type"`
		} `json:"entries"`
	}
	if err := json.Unmarshal([]byte(result.(string)), &payload); err != nil {
		t.Fatalf("decode ls result: %v", err)
	}
	if payload.MaxDepth != defaultFSToolLsDepth {
		t.Fatalf("max_depth = %d, want %d", payload.MaxDepth, defaultFSToolLsDepth)
	}
	seen := make(map[string]struct{}, len(payload.Entries))
	for _, entry := range payload.Entries {
		seen[entry.Path] = struct{}{}
	}
	if _, ok := seen["sub/note.txt"]; ok {
		t.Fatalf("default ls should not include nested entries: %+v", payload.Entries)
	}
	if _, ok := seen["sub/"]; !ok {
		t.Fatalf("expected top-level directory 'sub/' in entries: %+v", payload.Entries)
	}
	if _, ok := seen["root.txt"]; !ok {
		t.Fatalf("expected top-level file 'root.txt' in entries: %+v", payload.Entries)
	}
	if payload.Count != len(payload.Entries) {
		t.Fatalf("count = %d, want %d", payload.Count, len(payload.Entries))
	}
}

func TestLsTool_LongFormatIncludesMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	target := filepath.Join(tmpDir, "root.txt")
	if err := writeTestFile(target, "root"); err != nil {
		t.Fatalf("write root.txt: %v", err)
	}

	tool := NewLsTool([]string{tmpDir})
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"path": ".",
	})
	if err != nil {
		t.Fatalf("ls failed: %v", err)
	}

	var payload struct {
		Format  string `json:"format"`
		Entries []struct {
			Path       string `json:"path"`
			Type       string `json:"type"`
			Mode       string `json:"mode"`
			Size       int64  `json:"size"`
			ModifiedAt string `json:"modified_at"`
			Display    string `json:"display"`
		} `json:"entries"`
	}
	if err := json.Unmarshal([]byte(result.(string)), &payload); err != nil {
		t.Fatalf("decode ls result: %v", err)
	}
	if payload.Format != "long" {
		t.Fatalf("format = %q, want long", payload.Format)
	}

	seenFile := false
	seenDir := false
	for _, entry := range payload.Entries {
		switch entry.Path {
		case "root.txt":
			seenFile = true
			if entry.Type != "file" {
				t.Fatalf("root.txt type = %q, want file", entry.Type)
			}
			if entry.Mode == "" {
				t.Fatalf("root.txt mode should not be empty")
			}
			if entry.Size != 4 {
				t.Fatalf("root.txt size = %d, want 4", entry.Size)
			}
			if entry.ModifiedAt == "" {
				t.Fatalf("root.txt modified_at should not be empty")
			}
			if !strings.Contains(entry.Display, "root.txt") || !strings.Contains(entry.Display, entry.Mode) {
				t.Fatalf("root.txt display = %q, want long listing with filename and mode", entry.Display)
			}
		case "sub/":
			seenDir = true
			if entry.Type != "dir" {
				t.Fatalf("sub/ type = %q, want dir", entry.Type)
			}
			if entry.Mode == "" {
				t.Fatalf("sub/ mode should not be empty")
			}
			if !strings.Contains(entry.Display, "sub/") {
				t.Fatalf("sub/ display = %q, want directory suffix", entry.Display)
			}
		}
	}
	if !seenFile {
		t.Fatalf("expected root.txt entry, got %+v", payload.Entries)
	}
	if !seenDir {
		t.Fatalf("expected sub/ entry, got %+v", payload.Entries)
	}
}

func TestLsTool_DefaultMaxEntriesTruncates(t *testing.T) {
	tmpDir := t.TempDir()
	for i := 0; i < defaultFSToolLsEntries+25; i++ {
		name := filepath.Join(tmpDir, fmt.Sprintf("file-%03d.txt", i))
		if err := writeTestFile(name, "x"); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	tool := NewLsTool([]string{tmpDir})
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"path": ".",
	})
	if err != nil {
		t.Fatalf("ls failed: %v", err)
	}

	var payload struct {
		Count      int  `json:"count"`
		Truncated  bool `json:"truncated"`
		MaxEntries int  `json:"max_entries"`
		Entries    []struct {
			Path string `json:"path"`
		} `json:"entries"`
	}
	if err := json.Unmarshal([]byte(result.(string)), &payload); err != nil {
		t.Fatalf("decode ls result: %v", err)
	}
	if payload.MaxEntries != defaultFSToolLsEntries {
		t.Fatalf("max_entries = %d, want %d", payload.MaxEntries, defaultFSToolLsEntries)
	}
	if !payload.Truncated {
		t.Fatalf("truncated = false, want true")
	}
	if payload.Count != defaultFSToolLsEntries {
		t.Fatalf("count = %d, want %d", payload.Count, defaultFSToolLsEntries)
	}
	if len(payload.Entries) != defaultFSToolLsEntries {
		t.Fatalf("entries len = %d, want %d", len(payload.Entries), defaultFSToolLsEntries)
	}
}

// Test RegisterBuiltinTools
func TestRegisterBuiltinTools(t *testing.T) {
	registry := NewRegistry()
	RegisterBuiltinTools(registry)

	expectedTools := []string{"read", "write", "edit", "grep", "find", "ls", "web_query", "mcp"}
	for _, name := range expectedTools {
		if registry.Get(name) == nil {
			t.Errorf("expected tool '%s' to be registered", name)
		}
	}
	visible := make(map[string]struct{})
	for _, def := range registry.Definitions() {
		visible[def.Name] = struct{}{}
	}
	for _, name := range []string{"read", "write"} {
		if _, ok := visible[name]; !ok {
			t.Errorf("expected visible tool definition %q", name)
		}
	}
	for _, name := range []string{"file_read", "file_write", "file_delete", "write_begin", "write_chunk", "write_commit", "write_abort", "rg"} {
		if _, ok := visible[name]; ok {
			t.Errorf("did not expect legacy/internal tool %q in visible definitions", name)
		}
	}
	for _, name := range []string{"web", "web_search", "web_fetch", "web_read", "web_extract", "web_crawl"} {
		if registry.Get(name) == nil {
			t.Errorf("expected hidden compat tool '%s' to remain registered", name)
		}
		if !registry.IsDisabled(name) {
			t.Errorf("expected hidden compat tool '%s' to be disabled", name)
		}
	}
	for _, name := range []string{"file_read", "file_write", "file_delete", "write_begin", "write_chunk", "write_commit", "write_abort", "rg"} {
		if registry.Get(name) == nil {
			t.Errorf("expected legacy/internal tool %q to remain executable", name)
		}
		if !registry.IsDisabled(name) {
			t.Errorf("expected legacy/internal tool %q to be hidden", name)
		}
	}
}

func TestRegisterApprovalAwareFileTools_PreservesExistingReadServices(t *testing.T) {
	registry := NewRegistry()
	RegisterBuiltinTools(registry)

	pdfSvc := &stubPDFService{
		extract: pdfextract.ExtractResult{
			Text: "hello from pdf",
		},
	}
	docSvc := &stubDocumentReadService{
		result: &convertpkg.DocumentReadResult{
			Text:   "hello from docx",
			Format: "docx",
		},
	}
	AttachPDFServiceToWebTools(registry, pdfSvc)
	if read, ok := registry.Get("file_read").(*FileReadTool); ok {
		read.SetDocumentReadService(docSvc)
	} else {
		t.Fatalf("expected file_read tool")
	}

	RegisterApprovalAwareFileTools(registry, nil, 0, nil, nil)

	read, ok := registry.Get("file_read").(*FileReadTool)
	if !ok {
		t.Fatalf("expected re-registered file_read tool")
	}
	if read.pdfService != pdfSvc {
		t.Fatalf("pdf service was not preserved")
	}
	if read.documentReader != docSvc {
		t.Fatalf("document reader was not preserved")
	}
}

func TestExecutorNormalizesCompatSessionsAndWebAliasesToUnifiedTools(t *testing.T) {
	registry := NewRegistry()
	sessionsTool := &captureArgsTool{
		def: ToolDefinition{
			Name:        "sessions",
			Description: "sessions",
			Parameters: map[string]interface{}{
				"type":                 "object",
				"properties":           map[string]interface{}{},
				"additionalProperties": true,
			},
		},
	}
	webTool := &captureArgsTool{
		def: ToolDefinition{
			Name:        "web_query",
			Description: "web",
			Parameters: map[string]interface{}{
				"type":                 "object",
				"properties":           map[string]interface{}{},
				"additionalProperties": true,
			},
		},
	}
	registry.Register(sessionsTool)
	registry.Register(webTool)

	executor := NewExecutor(registry)
	if _, err := executor.Execute(context.Background(), "sessions_history", map[string]interface{}{"id": "conv_1", "limit": 3}); err != nil {
		t.Fatalf("execute sessions_history failed: %v", err)
	}
	if got := sessionsTool.args["action"]; got != "history" {
		t.Fatalf("sessions action = %v, want history", got)
	}
	if got := sessionsTool.args["id"]; got != "conv_1" {
		t.Fatalf("sessions id = %v, want conv_1", got)
	}

	if _, err := executor.Execute(context.Background(), "web_fetch", map[string]interface{}{"href": "https://example.com"}); err != nil {
		t.Fatalf("execute web_fetch failed: %v", err)
	}
	if got := webTool.args["action"]; got != "fetch" {
		t.Fatalf("web action = %v, want fetch", got)
	}
	if got := webTool.args["url"]; got != "https://example.com" {
		t.Fatalf("web url = %v, want https://example.com", got)
	}
}

func TestMCPToolDispatchesBuiltin(t *testing.T) {
	registry := NewRegistry()
	target := NewMockTool("file_read", "Read")
	target.SetResult("ok")
	registry.Register(target)
	registry.Register(NewMCPTool(registry))

	executor := NewExecutor(registry)
	got, err := executor.Execute(context.Background(), "mcp", map[string]interface{}{
		"tool": "read",
		"params": map[string]interface{}{
			"path": "a.txt",
		},
	})
	if err != nil {
		t.Fatalf("mcp dispatch failed: %v", err)
	}
	if got != "ok" {
		t.Fatalf("result = %v, want ok", got)
	}
}

func TestMCPToolSupportsNestedParamsArgs(t *testing.T) {
	registry := NewRegistry()
	target := &captureArgsTool{def: ToolDefinition{Name: "file_read", Description: "Read", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"path": map[string]interface{}{"type": "string"}}}}}
	registry.Register(target)
	registry.Register(NewMCPTool(registry))

	executor := NewExecutor(registry)
	_, err := executor.Execute(context.Background(), "mcp", map[string]interface{}{
		"input": map[string]interface{}{
			"tool":   "read",
			"params": map[string]interface{}{"path": "nested.txt"},
		},
	})
	if err != nil {
		t.Fatalf("mcp dispatch failed: %v", err)
	}
	if got := target.args["path"]; got != "nested.txt" {
		t.Fatalf("path = %v, want nested.txt", got)
	}
}

func TestMCPToolFactoryToolValidationReturnsStructuredResult(t *testing.T) {
	registry := NewRegistry()
	RegisterFactoryToolDefinitions(registry)
	registry.Register(NewMCPTool(registry))
	execTool := &captureArgsTool{
		def: ToolDefinition{
			Name:        "exec",
			Description: "Execute command",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{"type": "string"},
				},
				"required": []string{"command"},
			},
		},
	}
	registry.Register(execTool)

	executor := NewExecutor(registry)
	result, err := executor.Execute(context.Background(), "mcp", map[string]interface{}{
		"tool": "sessions_send",
		"params": map[string]interface{}{
			"limit": 5,
		},
	})
	if err != nil {
		t.Fatalf("mcp fallback failed: %v", err)
	}
	if execTool.args != nil {
		t.Fatalf("exec should not be called for invalid alias args, args=%v", execTool.args)
	}
	raw, ok := result.(string)
	if !ok {
		t.Fatalf("result type = %T, want string", result)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if payload["code"] != "invalid_arguments" {
		t.Fatalf("code = %v, want invalid_arguments", payload["code"])
	}
	if payload["tool"] != "sessions_send" {
		t.Fatalf("tool = %v, want sessions_send", payload["tool"])
	}
}

func TestMCPToolValidation(t *testing.T) {
	registry := NewRegistry()
	registry.Register(NewMCPTool(registry))
	executor := NewExecutor(registry)

	if _, err := executor.Execute(context.Background(), "mcp", map[string]interface{}{}); err == nil {
		t.Fatal("expected error for missing tool")
	}
	if _, err := executor.Execute(context.Background(), "mcp", map[string]interface{}{
		"tool":   "read",
		"params": "bad",
	}); err == nil {
		t.Fatal("expected error for non-object params")
	}
	if _, err := executor.Execute(context.Background(), "mcp", map[string]interface{}{
		"tool": "mcp",
	}); err == nil {
		t.Fatal("expected error for recursive mcp call")
	}
}

func TestRegisterFactoryToolDefinitions(t *testing.T) {
	registry := NewRegistry()
	RegisterFactoryToolDefinitions(registry)

	if got := registry.List(); len(got) != 0 {
		t.Fatalf("active tool list = %v, want empty for exposed-only definitions", got)
	}

	defs := registry.Definitions()
	names := make(map[string]struct{}, len(defs))
	for _, def := range defs {
		names[def.Name] = struct{}{}
	}
	for _, name := range []string{
		"browser",
		"canvas",
		"nodes",
		"cron",
		"message",
		"tts",
		"gateway",
		"agents_list",
		"sessions_list",
		"sessions_history",
		"sessions_send",
		"sessions_spawn",
		"subagents",
		"session_status",
		"memory_search",
		"memory_get",
		"memory_write",
		"memory_forget",
		"web_query",
		"image",
		"pdf",
	} {
		if _, ok := names[name]; !ok {
			t.Fatalf("expected definition %q to be exposed", name)
		}
	}
}

func TestExecutorFactoryWebFetchFallbackUsesWebFetchCommand(t *testing.T) {
	registry := NewRegistry()
	RegisterFactoryToolDefinitions(registry)
	execTool := &captureArgsTool{
		def: ToolDefinition{
			Name:        "exec",
			Description: "Execute command",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{"type": "string"},
				},
				"required": []string{"command"},
			},
		},
	}
	registry.Register(execTool)

	executor := NewExecutor(registry)
	if _, err := executor.Execute(context.Background(), "web_fetch", map[string]interface{}{"url": "https://example.com"}); err != nil {
		t.Fatalf("execute web_fetch failed: %v", err)
	}
	cmd, _ := execTool.args["command"].(string)
	if !strings.HasPrefix(cmd, "blue web_query") {
		t.Fatalf("command = %q, want prefix %q", cmd, "blue web_query")
	}
	if !strings.Contains(cmd, "url=https://example.com") {
		t.Fatalf("command = %q, want to contain %q", cmd, "url=https://example.com")
	}
}

func TestExecutorFactorySessionsListSupportsNestedActiveArgs(t *testing.T) {
	registry := NewRegistry()
	RegisterFactoryToolDefinitions(registry)
	execTool := &captureArgsTool{
		def: ToolDefinition{Name: "exec", Description: "Execute command", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"command": map[string]interface{}{"type": "string"}}, "required": []string{"command"}}},
	}
	registry.Register(execTool)

	executor := NewExecutor(registry)
	if _, err := executor.Execute(context.Background(), "sessions_list", map[string]interface{}{
		"input": map[string]interface{}{"active": true},
	}); err != nil {
		t.Fatalf("execute sessions_list failed: %v", err)
	}
	cmd, _ := execTool.args["command"].(string)
	if !strings.Contains(cmd, "blue sessions list --active") {
		t.Fatalf("command = %q, want active flag", cmd)
	}
}

func TestExecutorFactorySessionsListMapsToCLICommand(t *testing.T) {
	registry := NewRegistry()
	RegisterFactoryToolDefinitions(registry)
	execTool := &captureArgsTool{
		def: ToolDefinition{
			Name:        "exec",
			Description: "Execute command",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{"type": "string"},
				},
				"required": []string{"command"},
			},
		},
	}
	registry.Register(execTool)

	executor := NewExecutor(registry)
	if _, err := executor.Execute(context.Background(), "sessions_list", map[string]interface{}{"active": true}); err != nil {
		t.Fatalf("execute sessions_list failed: %v", err)
	}
	cmd, _ := execTool.args["command"].(string)
	if cmd != "blue sessions list --active" {
		t.Fatalf("command = %q, want %q", cmd, "blue sessions list --active")
	}
}

func TestExecutorFactorySessionsHistoryRequiresID(t *testing.T) {
	registry := NewRegistry()
	RegisterFactoryToolDefinitions(registry)

	executor := NewExecutor(registry)
	result, err := executor.Execute(context.Background(), "sessions_history", map[string]interface{}{})
	if err != nil {
		t.Fatalf("execute sessions_history failed: %v", err)
	}
	out, ok := result.(string)
	if !ok {
		t.Fatalf("result type = %T, want string", result)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if payload["code"] != "invalid_arguments" {
		t.Fatalf("code = %v, want invalid_arguments", payload["code"])
	}
	if payload["tool"] != "sessions_history" {
		t.Fatalf("tool = %v, want sessions_history", payload["tool"])
	}
}

func TestExecutorFactoryCronAndGatewayMapToCLICommand(t *testing.T) {
	registry := NewRegistry()
	RegisterFactoryToolDefinitions(registry)
	execTool := &captureArgsTool{
		def: ToolDefinition{
			Name:        "exec",
			Description: "Execute command",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{"type": "string"},
				},
				"required": []string{"command"},
			},
		},
	}
	registry.Register(execTool)
	executor := NewExecutor(registry)

	if _, err := executor.Execute(context.Background(), "cron", map[string]interface{}{"action": "status"}); err != nil {
		t.Fatalf("execute cron status failed: %v", err)
	}
	if got, _ := execTool.args["command"].(string); got != "blue cron status" {
		t.Fatalf("cron command = %q, want %q", got, "blue cron status")
	}

	if _, err := executor.Execute(context.Background(), "gateway", map[string]interface{}{"action": "restart"}); err != nil {
		t.Fatalf("execute gateway restart failed: %v", err)
	}
	if got, _ := execTool.args["command"].(string); got != "blue gateway restart" {
		t.Fatalf("gateway command = %q, want %q", got, "blue gateway restart")
	}
}

func TestExecutorFactoryMessageMapsToReminderSkillCommand(t *testing.T) {
	registry := NewRegistry()
	RegisterFactoryToolDefinitions(registry)
	execTool := &captureArgsTool{
		def: ToolDefinition{
			Name:        "exec",
			Description: "Execute command",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{"type": "string"},
				},
				"required": []string{"command"},
			},
		},
	}
	registry.Register(execTool)
	executor := NewExecutor(registry)

	if _, err := executor.Execute(context.Background(), "message", map[string]interface{}{
		"action":  "send",
		"message": "drink water",
		"time":    "10m",
	}); err != nil {
		t.Fatalf("execute message send failed: %v", err)
	}
	cmd, _ := execTool.args["command"].(string)
	if !strings.HasPrefix(cmd, "blue reminder") {
		t.Fatalf("command = %q, want prefix %q", cmd, "blue reminder")
	}
	if !strings.Contains(cmd, "action=add") {
		t.Fatalf("command = %q, want action=add", cmd)
	}
	if !strings.Contains(cmd, "message=\"drink water\"") {
		t.Fatalf("command = %q, want message arg", cmd)
	}
	if !strings.Contains(cmd, "time=10m") {
		t.Fatalf("command = %q, want time arg", cmd)
	}
}

func TestExecutorFactorySessionsSendAndSpawnMapToAPICommands(t *testing.T) {
	registry := NewRegistry()
	RegisterFactoryToolDefinitions(registry)
	execTool := &captureArgsTool{
		def: ToolDefinition{
			Name:        "exec",
			Description: "Execute command",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{"type": "string"},
				},
				"required": []string{"command"},
			},
		},
	}
	registry.Register(execTool)

	executor := NewExecutor(registry)
	if _, err := executor.Execute(context.Background(), "sessions_spawn", map[string]interface{}{"title": "demo"}); err != nil {
		t.Fatalf("execute sessions_spawn failed: %v", err)
	}
	cmd, _ := execTool.args["command"].(string)
	if !strings.Contains(cmd, "-X POST") || !strings.Contains(cmd, "/api/v1/conversations") {
		t.Fatalf("spawn command = %q, want POST /api/v1/conversations", cmd)
	}

	if _, err := executor.Execute(context.Background(), "sessions_send", map[string]interface{}{"id": "abc", "message": "hello"}); err != nil {
		t.Fatalf("execute sessions_send failed: %v", err)
	}
	cmd, _ = execTool.args["command"].(string)
	if !strings.Contains(cmd, "/api/v1/conversations/abc/messages") {
		t.Fatalf("send command = %q, want path /api/v1/conversations/abc/messages", cmd)
	}
	if !strings.Contains(cmd, "--data") {
		t.Fatalf("send command = %q, want --data payload", cmd)
	}
}

func TestExecutorFactoryMemoryWriteSupportsNestedTagsArgs(t *testing.T) {
	registry := NewRegistry()
	RegisterFactoryToolDefinitions(registry)
	execTool := &captureArgsTool{
		def: ToolDefinition{Name: "exec", Description: "Execute command", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"command": map[string]interface{}{"type": "string"}}, "required": []string{"command"}}},
	}
	registry.Register(execTool)

	executor := NewExecutor(registry)
	if _, err := executor.Execute(context.Background(), "memory_write", map[string]interface{}{
		"input": map[string]interface{}{
			"text": "hello",
			"tags": []interface{}{"prefs"},
		},
	}); err != nil {
		t.Fatalf("execute memory_write failed: %v", err)
	}
	cmd, _ := execTool.args["command"].(string)
	if !strings.Contains(cmd, "/api/v1/memory/store") || !strings.Contains(cmd, "prefs") {
		t.Fatalf("command = %q, want memory store tags payload", cmd)
	}
}

func TestExecutorFactoryCronSupportsNestedPayloadArgs(t *testing.T) {
	registry := NewRegistry()
	RegisterFactoryToolDefinitions(registry)
	execTool := &captureArgsTool{
		def: ToolDefinition{Name: "exec", Description: "Execute command", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"command": map[string]interface{}{"type": "string"}}, "required": []string{"command"}}},
	}
	registry.Register(execTool)

	executor := NewExecutor(registry)
	if _, err := executor.Execute(context.Background(), "cron", map[string]interface{}{
		"input": map[string]interface{}{
			"action":   "create",
			"name":     "Health Check",
			"schedule": "0 * * * *",
			"payload":  map[string]interface{}{"url": "https://example.com"},
		},
	}); err != nil {
		t.Fatalf("execute cron failed: %v", err)
	}
	cmd, _ := execTool.args["command"].(string)
	if !strings.Contains(cmd, "blue cron add") || !strings.Contains(cmd, "--payload") || !strings.Contains(cmd, "https://example.com") {
		t.Fatalf("command = %q, want cron payload", cmd)
	}
}

func TestExecutorFactoryNodesSupportsNestedBodyArgs(t *testing.T) {
	registry := NewRegistry()
	RegisterFactoryToolDefinitions(registry)
	execTool := &captureArgsTool{
		def: ToolDefinition{Name: "exec", Description: "Execute command", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"command": map[string]interface{}{"type": "string"}}, "required": []string{"command"}}},
	}
	registry.Register(execTool)

	executor := NewExecutor(registry)
	if _, err := executor.Execute(context.Background(), "nodes", map[string]interface{}{
		"input": map[string]interface{}{
			"name":  "Nested Flow",
			"nodes": []interface{}{map[string]interface{}{"id": "n1", "type": "start"}},
		},
	}); err != nil {
		t.Fatalf("execute nodes failed: %v", err)
	}
	cmd, _ := execTool.args["command"].(string)
	if !strings.Contains(cmd, "/api/v1/workflows") || !strings.Contains(cmd, "Nested Flow") || !strings.Contains(cmd, "n1") || !strings.Contains(cmd, "start") {
		t.Fatalf("command = %q, want nested workflow payload", cmd)
	}
}

func TestExecutorFactoryAdditionalAliasesMapToCommands(t *testing.T) {
	registry := NewRegistry()
	RegisterFactoryToolDefinitions(registry)
	execTool := &captureArgsTool{
		def: ToolDefinition{
			Name:        "exec",
			Description: "Execute command",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{"type": "string"},
				},
				"required": []string{"command"},
			},
		},
	}
	registry.Register(execTool)

	executor := NewExecutor(registry)
	cases := []struct {
		name     string
		args     map[string]interface{}
		contains string
		prefix   string
	}{
		{name: "canvas", args: map[string]interface{}{"action": "list"}, contains: "/api/v1/workflows"},
		{name: "nodes", args: map[string]interface{}{"action": "templates"}, contains: "/api/v1/workflows/templates"},
		{name: "tts", args: map[string]interface{}{"text": "hello"}, contains: "/api/v1/voice/synthesize"},
		{name: "memory_search", args: map[string]interface{}{"query": "foo"}, contains: "/api/v1/memory/search"},
		{name: "memory_get", args: map[string]interface{}{"id": "m1"}, contains: "/api/v1/memory/m1"},
		{name: "memory_write", args: map[string]interface{}{"content": "hello"}, contains: "/api/v1/memory/store"},
		{name: "memory_forget", args: map[string]interface{}{"id": "m1"}, contains: "/api/v1/memory/m1"},
		{name: "image", args: map[string]interface{}{"prompt": "a cat"}, prefix: "blue media generate"},
	}

	for _, tc := range cases {
		_, err := executor.Execute(context.Background(), tc.name, tc.args)
		if err != nil {
			t.Fatalf("execute %s failed: %v", tc.name, err)
		}
		cmd, _ := execTool.args["command"].(string)
		if tc.prefix != "" && !strings.HasPrefix(cmd, tc.prefix) {
			t.Fatalf("%s command = %q, want prefix %q", tc.name, cmd, tc.prefix)
		}
		if tc.contains != "" && !strings.Contains(cmd, tc.contains) {
			t.Fatalf("%s command = %q, want to contain %q", tc.name, cmd, tc.contains)
		}
	}
}

func TestExecutorFactoryUnsupportedAliasesReturnStructuredResult(t *testing.T) {
	registry := NewRegistry()
	RegisterFactoryToolDefinitions(registry)

	executor := NewExecutor(registry)
	for _, name := range []string{"agents_list", "subagents", "pdf"} {
		result, err := executor.Execute(context.Background(), name, map[string]interface{}{"input": "test"})
		if err != nil {
			t.Fatalf("execute %s failed: %v", name, err)
		}
		out, ok := result.(string)
		if !ok {
			t.Fatalf("%s result type = %T, want string", name, result)
		}
		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(out), &payload); err != nil {
			t.Fatalf("%s decode result: %v", name, err)
		}
		if payload["code"] != "tool_unavailable" {
			t.Fatalf("%s code = %v, want tool_unavailable", name, payload["code"])
		}
		if payload["tool"] != name {
			t.Fatalf("%s tool = %v, want %s", name, payload["tool"], name)
		}
	}
}

func TestExecutorFactorySessionsSendRequiresMessage(t *testing.T) {
	registry := NewRegistry()
	RegisterFactoryToolDefinitions(registry)

	executor := NewExecutor(registry)
	result, err := executor.Execute(context.Background(), "sessions_send", map[string]interface{}{"id": "abc"})
	if err != nil {
		t.Fatalf("execute sessions_send failed: %v", err)
	}
	out, ok := result.(string)
	if !ok {
		t.Fatalf("result type = %T, want string", result)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if payload["code"] != "invalid_arguments" {
		t.Fatalf("code = %v, want invalid_arguments", payload["code"])
	}
	if payload["tool"] != "sessions_send" {
		t.Fatalf("tool = %v, want sessions_send", payload["tool"])
	}
}

func TestExecutorFactoryCompatCoverage_NoErrToolNotFound(t *testing.T) {
	registry := NewRegistry()
	RegisterFactoryToolDefinitions(registry)
	execTool := &captureArgsTool{
		def: ToolDefinition{
			Name:        "exec",
			Description: "Execute command",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{"type": "string"},
				},
				"required": []string{"command"},
			},
		},
	}
	registry.Register(execTool)
	executor := NewExecutor(registry)

	for _, name := range factoryToolNames {
		result, err := executor.Execute(context.Background(), name, map[string]interface{}{"input": "test"})
		if errors.Is(err, ErrToolNotFound) {
			t.Fatalf("tool %q returned ErrToolNotFound", name)
		}
		if raw, ok := result.(string); ok && strings.HasPrefix(strings.TrimSpace(raw), "{") {
			var payload map[string]interface{}
			if json.Unmarshal([]byte(raw), &payload) == nil {
				if payload["code"] == "tool_unavailable" && name != "agents_list" && name != "subagents" && name != "pdf" {
					t.Fatalf("tool %q returned unavailable payload: %s", name, raw)
				}
			}
		}
	}
}

// Helper functions for file tests
func writeTestFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

func writeTestSpreadsheetXLSX(t *testing.T, path string) {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create xlsx: %v", err)
	}
	defer file.Close()

	zw := zip.NewWriter(file)
	writeZipEntry := func(name, content string) {
		t.Helper()
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}

	writeZipEntry("xl/workbook.xml", `<?xml version="1.0" encoding="UTF-8"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets>
    <sheet name="Budget" sheetId="1" r:id="rId1"/>
  </sheets>
</workbook>`)
	writeZipEntry("xl/_rels/workbook.xml.rels", `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
</Relationships>`)
	writeZipEntry("xl/sharedStrings.xml", `<?xml version="1.0" encoding="UTF-8"?>
<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="3" uniqueCount="3">
  <si><t>Department</t></si>
  <si><t>Department</t></si>
  <si><t>Owner</t></si>
</sst>`)
	writeZipEntry("xl/worksheets/sheet1.xml", `<?xml version="1.0" encoding="UTF-8"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData>
    <row r="1">
      <c r="A1" t="s"><v>0</v></c>
      <c r="B1" t="s"><v>1</v></c>
      <c r="D1" t="s"><v>2</v></c>
    </row>
    <row r="2">
      <c r="A2" t="inlineStr"><is><t>Finance</t></is></c>
      <c r="B2" t="inlineStr"><is><t>Platform</t></is></c>
      <c r="C2"><v>1200</v></c>
      <c r="D2" t="inlineStr"><is><t>Alice</t></is></c>
    </row>
  </sheetData>
</worksheet>`)

	if err := zw.Close(); err != nil {
		t.Fatalf("close xlsx zip: %v", err)
	}
}

func readTestFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
