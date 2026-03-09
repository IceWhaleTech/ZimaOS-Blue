package tools

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestFileToolDefinitionsUseNewNames(t *testing.T) {
	if got := NewFileReadTool(nil, 0).Definition().Name; got != "read" {
		t.Fatalf("read tool name = %q, want read", got)
	}
	if got := NewFileWriteTool(nil, 0).Definition().Name; got != "write" {
		t.Fatalf("write tool name = %q, want write", got)
	}
}

func TestExecutorLegacyToolNameRemap(t *testing.T) {
	registry := NewRegistry()
	readTool := NewMockTool("read", "Read")
	readTool.SetResult("ok")
	writeTool := NewMockTool("write", "Write")
	writeTool.SetResult("ok")
	registry.Register(readTool)
	registry.Register(writeTool)

	executor := NewExecutor(registry)
	if _, err := executor.Execute(context.Background(), "file_read", map[string]interface{}{"path": "a.txt"}); err != nil {
		t.Fatalf("file_read compatibility execute failed: %v", err)
	}
	if _, err := executor.Execute(context.Background(), "file_write", map[string]interface{}{"path": "a.txt", "content": "x"}); err != nil {
		t.Fatalf("file_write compatibility execute failed: %v", err)
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

// Test RegisterBuiltinTools
func TestRegisterBuiltinTools(t *testing.T) {
	registry := NewRegistry()
	RegisterBuiltinTools(registry)

	expectedTools := []string{"read", "write", "edit", "grep", "find", "ls", "web_search", "web_fetch", "mcp"}
	for _, name := range expectedTools {
		if registry.Get(name) == nil {
			t.Errorf("expected tool '%s' to be registered", name)
		}
	}
	if registry.Get("file_read") != nil {
		t.Errorf("did not expect legacy tool name 'file_read' to be registered")
	}
	if registry.Get("file_write") != nil {
		t.Errorf("did not expect legacy tool name 'file_write' to be registered")
	}
}

func TestMCPToolDispatchesBuiltin(t *testing.T) {
	registry := NewRegistry()
	target := NewMockTool("read", "Read")
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
		"web_search",
		"web_fetch",
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
	if !strings.HasPrefix(cmd, "blue web_fetch") {
		t.Fatalf("command = %q, want prefix %q", cmd, "blue web_fetch")
	}
	if !strings.Contains(cmd, "url=https://example.com") {
		t.Fatalf("command = %q, want to contain %q", cmd, "url=https://example.com")
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

func readTestFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
