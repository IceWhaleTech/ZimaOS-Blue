package tools

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

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

// Test Calculator tool
func TestCalculatorTool(t *testing.T) {
	calc := NewCalculatorTool()

	tests := []struct {
		name       string
		expression string
		expected   float64
		wantErr    bool
	}{
		{"addition", "2 + 3", 5, false},
		{"subtraction", "10 - 4", 6, false},
		{"multiplication", "3 * 4", 12, false},
		{"division", "15 / 3", 5, false},
		{"complex", "2 + 3 * 4", 14, false},
		{"parentheses", "(2 + 3) * 4", 20, false},
		{"decimal", "3.14 * 2", 6.28, false},
		{"negative", "-5 + 10", 5, false},
		{"invalid", "abc", 0, true},
		{"empty", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := calc.Execute(context.Background(), map[string]interface{}{
				"expression": tt.expression,
			})

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Parse result
			var resultMap map[string]interface{}
			if err := json.Unmarshal([]byte(result.(string)), &resultMap); err != nil {
				t.Fatalf("failed to parse result: %v", err)
			}

			resultValue, ok := resultMap["result"].(float64)
			if !ok {
				t.Fatalf("result is not a float64: %v", resultMap["result"])
			}

			if resultValue != tt.expected {
				t.Errorf("expected %f, got %f", tt.expected, resultValue)
			}
		})
	}
}

// Test Calculator tool missing expression
func TestCalculatorToolMissingExpression(t *testing.T) {
	calc := NewCalculatorTool()
	_, err := calc.Execute(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("expected error for missing expression")
	}
}

// Test SystemInfo tool
func TestSystemInfoTool(t *testing.T) {
	sysInfo := NewSystemInfoTool()

	result, err := sysInfo.Execute(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Parse result
	var info map[string]interface{}
	if err := json.Unmarshal([]byte(result.(string)), &info); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	// Check required fields
	requiredFields := []string{"os", "arch", "hostname", "num_cpu", "go_version"}
	for _, field := range requiredFields {
		if _, ok := info[field]; !ok {
			t.Errorf("missing required field: %s", field)
		}
	}
}

// Test CurrentTime tool
func TestCurrentTimeTool(t *testing.T) {
	timeTool := NewCurrentTimeTool()

	result, err := timeTool.Execute(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Parse result
	var timeInfo map[string]interface{}
	if err := json.Unmarshal([]byte(result.(string)), &timeInfo); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	// Check required fields
	if _, ok := timeInfo["utc"]; !ok {
		t.Error("missing utc field")
	}
	if _, ok := timeInfo["local"]; !ok {
		t.Error("missing local field")
	}
	if _, ok := timeInfo["unix"]; !ok {
		t.Error("missing unix field")
	}
}

// Test CurrentTime tool with timezone
func TestCurrentTimeToolWithTimezone(t *testing.T) {
	timeTool := NewCurrentTimeTool()

	result, err := timeTool.Execute(context.Background(), map[string]interface{}{
		"timezone": "America/New_York",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var timeInfo map[string]interface{}
	if err := json.Unmarshal([]byte(result.(string)), &timeInfo); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	if _, ok := timeInfo["requested_timezone"]; !ok {
		t.Error("missing requested_timezone field")
	}
}

// Test CurrentTime tool with invalid timezone
func TestCurrentTimeToolInvalidTimezone(t *testing.T) {
	timeTool := NewCurrentTimeTool()

	_, err := timeTool.Execute(context.Background(), map[string]interface{}{
		"timezone": "Invalid/Timezone",
	})
	if err == nil {
		t.Error("expected error for invalid timezone")
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

	tool := NewFileReadTool(nil, 0)

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

	tool := NewFileWriteTool(nil, 0)

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

	tool := NewFileWriteTool(nil, 0)

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

// Test RegisterBuiltinTools
func TestRegisterBuiltinTools(t *testing.T) {
	registry := NewRegistry()
	RegisterBuiltinTools(registry)

	expectedTools := []string{"calculator", "system_info", "current_time", "file_read", "file_write"}
	for _, name := range expectedTools {
		if registry.Get(name) == nil {
			t.Errorf("expected tool '%s' to be registered", name)
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
