package adapters

import (
	"context"
	"encoding/json"
	"testing"
)

// MockProvider is a mock LLM provider for testing.
type MockProvider struct {
	Response *ChatResponse
	Error    error
}

func (m *MockProvider) Chat(ctx context.Context, messages []Message, options *ChatOptions) (*ChatResponse, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return m.Response, nil
}

func TestCLIProxyAdapter_RegisterTool(t *testing.T) {
	adapter := NewCLIProxyAdapter(nil)

	tool := Tool{
		Name:        "test_tool",
		Description: "A test tool",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"param1": map[string]interface{}{
					"type":        "string",
					"description": "A test parameter",
				},
			},
		},
	}

	adapter.RegisterTool(tool)

	if _, ok := adapter.Tools["test_tool"]; !ok {
		t.Error("Tool should be registered")
	}
}

func TestCLIProxyAdapter_RegisterTools(t *testing.T) {
	adapter := NewCLIProxyAdapter(nil)

	tools := []Tool{
		{Name: "tool1", Description: "Tool 1"},
		{Name: "tool2", Description: "Tool 2"},
		{Name: "tool3", Description: "Tool 3"},
	}

	adapter.RegisterTools(tools)

	if len(adapter.Tools) != 3 {
		t.Errorf("Expected 3 tools, got %d", len(adapter.Tools))
	}
}

func TestCLIProxyAdapter_BuildToolPrompt(t *testing.T) {
	adapter := NewCLIProxyAdapter(nil)

	adapter.RegisterTool(Tool{
		Name:        "read_file",
		Description: "Read a file from disk",
		Parameters: map[string]interface{}{
			"path": "string",
		},
	})

	prompt := adapter.BuildToolPrompt("Read the file test.txt")

	if prompt == "" {
		t.Error("BuildToolPrompt should return non-empty string")
	}

	if !contains(prompt, "read_file") {
		t.Error("Prompt should contain tool name")
	}

	if !contains(prompt, "Read the file test.txt") {
		t.Error("Prompt should contain user request")
	}
}

func TestCLIProxyAdapter_ParseToolCall(t *testing.T) {
	adapter := NewCLIProxyAdapter(nil)

	tests := []struct {
		name     string
		response string
		wantTool string
		wantNil  bool
	}{
		{
			name:     "valid tool call",
			response: `I'll help you with that. {"tool": "read_file", "input": {"path": "test.txt"}}`,
			wantTool: "read_file",
			wantNil:  false,
		},
		{
			name:     "no tool call",
			response: "I don't need to use any tools for this.",
			wantTool: "",
			wantNil:  true,
		},
		{
			name:     "tool call with extra text",
			response: `Let me read that file for you.
{"tool": "read_file", "input": {"path": "/home/user/file.txt"}}
I'll process the contents.`,
			wantTool: "read_file",
			wantNil:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			call, err := adapter.ParseToolCall(tt.response)
			if err != nil {
				t.Fatalf("ParseToolCall error: %v", err)
			}

			if tt.wantNil {
				if call != nil {
					t.Error("Expected nil tool call")
				}
			} else {
				if call == nil {
					t.Fatal("Expected non-nil tool call")
				}
				if call.Name != tt.wantTool {
					t.Errorf("Tool name = %q, want %q", call.Name, tt.wantTool)
				}
			}
		})
	}
}

func TestCLIProxyAdapter_Chat(t *testing.T) {
	mockProvider := &MockProvider{
		Response: &ChatResponse{
			Content: `{"tool": "test_tool", "input": {"param": "value"}}`,
		},
	}

	adapter := NewCLIProxyAdapter(mockProvider)
	adapter.RegisterTool(Tool{
		Name:        "test_tool",
		Description: "A test tool",
	})

	messages := []Message{
		{Role: "user", Content: "Use the test tool"},
	}

	resp, err := adapter.Chat(context.Background(), messages, nil)
	if err != nil {
		t.Fatalf("Chat error: %v", err)
	}

	if resp == nil {
		t.Fatal("Expected non-nil response")
	}
}

func TestCLIProxyAdapter_ExecuteToolCall(t *testing.T) {
	adapter := NewCLIProxyAdapter(nil)
	adapter.RegisterTool(Tool{
		Name:        "test_tool",
		Description: "A test tool",
	})

	// Test successful execution
	call := &ToolCall{
		Name:  "test_tool",
		Input: map[string]interface{}{"param": "value"},
	}

	executor := func(ctx context.Context, c *ToolCall) (interface{}, error) {
		return "success", nil
	}

	result, err := adapter.ExecuteToolCall(context.Background(), call, executor)
	if err != nil {
		t.Fatalf("ExecuteToolCall error: %v", err)
	}

	if !result.Success {
		t.Error("Expected successful result")
	}

	// Test unknown tool
	unknownCall := &ToolCall{
		Name:  "unknown_tool",
		Input: map[string]interface{}{},
	}

	result, err = adapter.ExecuteToolCall(context.Background(), unknownCall, executor)
	if err != nil {
		t.Fatalf("ExecuteToolCall error: %v", err)
	}

	if result.Success {
		t.Error("Expected failed result for unknown tool")
	}
}

func TestCLIProxyAdapter_BuildToolResultPrompt(t *testing.T) {
	adapter := NewCLIProxyAdapter(nil)

	// Test successful result
	successResult := &ToolResult{
		Name:    "test_tool",
		Output:  "file contents",
		Success: true,
	}

	prompt := adapter.BuildToolResultPrompt(successResult)
	if !contains(prompt, "test_tool") {
		t.Error("Prompt should contain tool name")
	}
	if !contains(prompt, "file contents") {
		t.Error("Prompt should contain output")
	}

	// Test failed result
	failedResult := &ToolResult{
		Name:    "test_tool",
		Error:   "file not found",
		Success: false,
	}

	prompt = adapter.BuildToolResultPrompt(failedResult)
	if !contains(prompt, "failed") {
		t.Error("Prompt should indicate failure")
	}
	if !contains(prompt, "file not found") {
		t.Error("Prompt should contain error message")
	}
}

// ccNexus adapter tests

func TestCCNexusAdapter_ToOpenAIFormat(t *testing.T) {
	adapter := NewCCNexusAdapter("anthropic", "openai")

	tool := Tool{
		Name:        "test_tool",
		Description: "A test tool",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"param1": map[string]interface{}{
					"type": "string",
				},
			},
		},
	}

	result := adapter.ToOpenAIFormat(tool)

	if result.Type != "function" {
		t.Errorf("Type = %q, want %q", result.Type, "function")
	}

	if result.Function.Name != "test_tool" {
		t.Errorf("Name = %q, want %q", result.Function.Name, "test_tool")
	}
}

func TestCCNexusAdapter_ToAnthropicFormat(t *testing.T) {
	adapter := NewCCNexusAdapter("openai", "anthropic")

	tool := Tool{
		Name:        "test_tool",
		Description: "A test tool",
		Parameters: map[string]interface{}{
			"type": "object",
		},
	}

	result := adapter.ToAnthropicFormat(tool)

	if result.Name != "test_tool" {
		t.Errorf("Name = %q, want %q", result.Name, "test_tool")
	}

	if result.InputSchema == nil {
		t.Error("InputSchema should not be nil")
	}
}

func TestCCNexusAdapter_ToOllamaFormat(t *testing.T) {
	adapter := NewCCNexusAdapter("openai", "ollama")

	tool := Tool{
		Name:        "test_tool",
		Description: "A test tool",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"param1": map[string]interface{}{
					"type":        "string",
					"description": "A parameter",
				},
			},
			"required": []interface{}{"param1"},
		},
	}

	result := adapter.ToOllamaFormat(tool)

	if result.Type != "function" {
		t.Errorf("Type = %q, want %q", result.Type, "function")
	}

	if result.Function.Name != "test_tool" {
		t.Errorf("Name = %q, want %q", result.Function.Name, "test_tool")
	}

	if len(result.Function.Parameters.Properties) != 1 {
		t.Errorf("Expected 1 property, got %d", len(result.Function.Parameters.Properties))
	}

	if len(result.Function.Parameters.Required) != 1 {
		t.Errorf("Expected 1 required, got %d", len(result.Function.Parameters.Required))
	}
}

func TestCCNexusAdapter_ToGeminiFormat(t *testing.T) {
	adapter := NewCCNexusAdapter("openai", "gemini")

	tool := Tool{
		Name:        "test_tool",
		Description: "A test tool",
		Parameters: map[string]interface{}{
			"type": "object",
		},
	}

	result := adapter.ToGeminiFormat(tool)

	if len(result.FunctionDeclarations) != 1 {
		t.Errorf("Expected 1 function declaration, got %d", len(result.FunctionDeclarations))
	}

	if result.FunctionDeclarations[0].Name != "test_tool" {
		t.Errorf("Name = %q, want %q", result.FunctionDeclarations[0].Name, "test_tool")
	}
}

func TestCCNexusAdapter_ConvertTools(t *testing.T) {
	adapter := NewCCNexusAdapter("openai", "anthropic")

	tools := []Tool{
		{Name: "tool1", Description: "Tool 1"},
		{Name: "tool2", Description: "Tool 2"},
	}

	result, err := adapter.ConvertTools(tools)
	if err != nil {
		t.Fatalf("ConvertTools error: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(result))
	}
}

func TestCCNexusAdapter_ConvertToolCall_OpenAI(t *testing.T) {
	adapter := NewCCNexusAdapter("openai", "anthropic")

	response := `{
		"choices": [{
			"message": {
				"tool_calls": [{
					"function": {
						"name": "test_tool",
						"arguments": "{\"param\": \"value\"}"
					}
				}]
			}
		}]
	}`

	call, err := adapter.ConvertToolCall([]byte(response), "openai")
	if err != nil {
		t.Fatalf("ConvertToolCall error: %v", err)
	}

	if call == nil {
		t.Fatal("Expected non-nil tool call")
	}

	if call.Name != "test_tool" {
		t.Errorf("Name = %q, want %q", call.Name, "test_tool")
	}
}

func TestCCNexusAdapter_ConvertToolCall_Anthropic(t *testing.T) {
	adapter := NewCCNexusAdapter("anthropic", "openai")

	response := `{
		"content": [{
			"type": "tool_use",
			"name": "test_tool",
			"input": {"param": "value"}
		}]
	}`

	call, err := adapter.ConvertToolCall([]byte(response), "anthropic")
	if err != nil {
		t.Fatalf("ConvertToolCall error: %v", err)
	}

	if call == nil {
		t.Fatal("Expected non-nil tool call")
	}

	if call.Name != "test_tool" {
		t.Errorf("Name = %q, want %q", call.Name, "test_tool")
	}
}

func TestCCNexusAdapter_ConvertToolCall_Ollama(t *testing.T) {
	adapter := NewCCNexusAdapter("ollama", "openai")

	response := `{
		"message": {
			"tool_calls": [{
				"function": {
					"name": "test_tool",
					"arguments": {"param": "value"}
				}
			}]
		}
	}`

	call, err := adapter.ConvertToolCall([]byte(response), "ollama")
	if err != nil {
		t.Fatalf("ConvertToolCall error: %v", err)
	}

	if call == nil {
		t.Fatal("Expected non-nil tool call")
	}

	if call.Name != "test_tool" {
		t.Errorf("Name = %q, want %q", call.Name, "test_tool")
	}
}

func TestCCNexusAdapter_BuildToolResult(t *testing.T) {
	tests := []struct {
		name         string
		targetFormat string
		result       *ToolResult
		checkKey     string
	}{
		{
			name:         "openai format",
			targetFormat: "openai",
			result:       &ToolResult{Name: "test", Output: "output", Success: true},
			checkKey:     "tool_call_id",
		},
		{
			name:         "anthropic format",
			targetFormat: "anthropic",
			result:       &ToolResult{Name: "test", Output: "output", Success: true},
			checkKey:     "tool_use_id",
		},
		{
			name:         "ollama format",
			targetFormat: "ollama",
			result:       &ToolResult{Name: "test", Output: "output", Success: true},
			checkKey:     "role",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewCCNexusAdapter("openai", tt.targetFormat)
			built, err := adapter.BuildToolResult(tt.result)
			if err != nil {
				t.Fatalf("BuildToolResult error: %v", err)
			}

			resultMap, ok := built.(map[string]interface{})
			if !ok {
				t.Fatal("Expected map result")
			}

			if _, ok := resultMap[tt.checkKey]; !ok {
				t.Errorf("Expected key %q in result", tt.checkKey)
			}
		})
	}
}

func TestCCNexusAdapter_BuildToolResult_Error(t *testing.T) {
	adapter := NewCCNexusAdapter("openai", "openai")

	result := &ToolResult{
		Name:    "test",
		Error:   "something went wrong",
		Success: false,
	}

	built, err := adapter.BuildToolResult(result)
	if err != nil {
		t.Fatalf("BuildToolResult error: %v", err)
	}

	resultMap := built.(map[string]interface{})
	content := resultMap["content"].(string)

	if content != "something went wrong" {
		t.Errorf("Content = %q, want %q", content, "something went wrong")
	}
}

// Router tests

func TestCapabilityRouter_RegisterProvider(t *testing.T) {
	router := NewCapabilityRouter(nil)

	mockProvider := &MockProvider{}
	router.RegisterProvider("test", mockProvider)

	if _, ok := router.providers["test"]; !ok {
		t.Error("Provider should be registered")
	}

	if router.cliProxyAdapters["test"] == nil {
		t.Error("CLIProxy adapter should be created")
	}

	if router.ccNexusAdapters["test"] == nil {
		t.Error("ccNexus adapter should be created")
	}
}

func TestCapabilityRouter_RegisterTools(t *testing.T) {
	router := NewCapabilityRouter(nil)

	mockProvider := &MockProvider{}
	router.RegisterProvider("test", mockProvider)

	tools := []Tool{
		{Name: "tool1", Description: "Tool 1"},
		{Name: "tool2", Description: "Tool 2"},
	}

	router.RegisterTools(tools)

	adapter := router.GetCLIProxyAdapter("test")
	if adapter == nil {
		t.Fatal("CLIProxy adapter should exist")
	}

	if len(adapter.Tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(adapter.Tools))
	}
}

func TestCapabilityRouter_SelectAdapter(t *testing.T) {
	router := NewCapabilityRouter(nil)

	mockProvider := &MockProvider{}
	router.RegisterProvider("anthropic", mockProvider)
	router.RegisterProvider("ollama", mockProvider)
	router.RegisterProvider("local", mockProvider)

	tests := []struct {
		provider    string
		wantAdapter string
	}{
		{"anthropic", "native"},
		{"ollama", "ccnexus"},
		{"local", "cliproxy"},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			_, adapterType, err := router.SelectAdapter(tt.provider)
			if err != nil && tt.wantAdapter != "" {
				t.Fatalf("SelectAdapter error: %v", err)
			}

			if adapterType != tt.wantAdapter {
				t.Errorf("AdapterType = %q, want %q", adapterType, tt.wantAdapter)
			}
		})
	}
}

func TestCapabilityRouter_GetCapability(t *testing.T) {
	router := NewCapabilityRouter(nil)

	cap, found := router.GetCapability("anthropic")
	if !found {
		t.Error("Anthropic capability should be found")
	}

	if cap == nil {
		t.Fatal("Capability should not be nil")
	}
}

func TestCapabilityRouter_GetCapabilityMatrix(t *testing.T) {
	router := NewCapabilityRouter(nil)

	matrix := router.GetCapabilityMatrix()
	if len(matrix) == 0 {
		t.Error("Capability matrix should not be empty")
	}
}

func TestCapabilityRouter_NeedsAdapter(t *testing.T) {
	router := NewCapabilityRouter(nil)

	if router.NeedsAdapter("anthropic") {
		t.Error("Anthropic should not need adapter")
	}

	if !router.NeedsAdapter("local") {
		t.Error("Local should need adapter")
	}
}

func TestCapabilityRouter_ConvertToolsForProvider(t *testing.T) {
	router := NewCapabilityRouter(nil)

	tools := []Tool{
		{Name: "tool1", Description: "Tool 1"},
	}

	converted, err := router.ConvertToolsForProvider("anthropic", tools)
	if err != nil {
		t.Fatalf("ConvertToolsForProvider error: %v", err)
	}

	if len(converted) != 1 {
		t.Errorf("Expected 1 converted tool, got %d", len(converted))
	}
}

func TestDefaultRouterConfig(t *testing.T) {
	config := DefaultRouterConfig()

	if config == nil {
		t.Fatal("DefaultRouterConfig should not return nil")
	}

	if !config.AutoDetect {
		t.Error("AutoDetect should be true by default")
	}

	if config.DefaultAdapter != "cliproxy" {
		t.Errorf("DefaultAdapter = %q, want %q", config.DefaultAdapter, "cliproxy")
	}

	if !config.FallbackEnabled {
		t.Error("FallbackEnabled should be true by default")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// JSON marshal helper for testing
func mustMarshal(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
