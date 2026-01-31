package claudecode

import (
	"context"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/tools"
)

// TestProviderToolRegistryIntegration tests that tools are properly loaded into the CC CLI provider.
func TestProviderToolRegistryIntegration(t *testing.T) {
	// Create a tool registry with test tools
	registry := tools.NewRegistry()

	// Register a test tool using MockTool
	testTool := tools.NewMockTool("test_tool", "A test tool for integration testing")
	testTool.SetResult(map[string]string{"result": "test"})
	registry.Register(testTool)

	// Create provider with config
	config := &ClaudeCodeConfig{
		Enabled:      true,
		Command:      "claude",
		DefaultModel: "sonnet",
	}
	provider := NewProvider(config)

	// Set tool registry
	provider.SetToolRegistry(registry)

	// Verify tool registry is set on prompt builder
	if provider.promptBuilder.toolRegistry == nil {
		t.Error("expected tool registry to be set on prompt builder")
	}

	// Build system prompt and verify tools are included
	ctx := context.Background()
	systemPrompt := provider.promptBuilder.Build(ctx, "")

	if !strings.Contains(systemPrompt, "test_tool") {
		t.Error("expected system prompt to contain test_tool")
	}

	if !strings.Contains(systemPrompt, "A test tool for integration testing") {
		t.Error("expected system prompt to contain tool description")
	}

	if !strings.Contains(systemPrompt, "Available Tools") {
		t.Error("expected system prompt to contain 'Available Tools' section")
	}
}

// TestProviderMultipleToolsIntegration tests that multiple tools are properly loaded.
func TestProviderMultipleToolsIntegration(t *testing.T) {
	registry := tools.NewRegistry()

	// Register multiple test tools
	toolDefs := []struct {
		name        string
		description string
	}{
		{"search_web", "Search the web for information"},
		{"read_file", "Read contents of a file"},
		{"write_file", "Write contents to a file"},
		{"execute_code", "Execute code in a sandbox"},
	}

	for _, def := range toolDefs {
		tool := tools.NewMockTool(def.name, def.description)
		registry.Register(tool)
	}

	config := &ClaudeCodeConfig{
		Enabled:      true,
		Command:      "claude",
		DefaultModel: "sonnet",
	}
	provider := NewProvider(config)
	provider.SetToolRegistry(registry)

	ctx := context.Background()
	systemPrompt := provider.promptBuilder.Build(ctx, "")

	// Verify all tools are included
	for _, def := range toolDefs {
		if !strings.Contains(systemPrompt, def.name) {
			t.Errorf("expected system prompt to contain tool '%s'", def.name)
		}
		if !strings.Contains(systemPrompt, def.description) {
			t.Errorf("expected system prompt to contain description '%s'", def.description)
		}
	}
}

// TestProviderSystemPromptWithToolsAndExtraPrompt tests combining tools with extra system prompt.
func TestProviderSystemPromptWithToolsAndExtraPrompt(t *testing.T) {
	registry := tools.NewRegistry()

	tool := tools.NewMockTool("custom_tool", "A custom tool")
	registry.Register(tool)

	config := &ClaudeCodeConfig{
		Enabled:      true,
		Command:      "claude",
		DefaultModel: "sonnet",
	}
	provider := NewProvider(config)
	provider.SetToolRegistry(registry)

	ctx := context.Background()
	extraPrompt := "You are a helpful assistant specialized in coding tasks."
	systemPrompt := provider.promptBuilder.Build(ctx, extraPrompt)

	// Verify both tools and extra prompt are included
	if !strings.Contains(systemPrompt, "custom_tool") {
		t.Error("expected system prompt to contain custom_tool")
	}

	if !strings.Contains(systemPrompt, extraPrompt) {
		t.Error("expected system prompt to contain extra prompt")
	}

	// Verify runtime info is included
	if !strings.Contains(systemPrompt, "Runtime Information") {
		t.Error("expected system prompt to contain runtime information")
	}
}

// TestProviderRequestRoutingThroughCLI tests that requests are properly routed through CC CLI.
func TestProviderRequestRoutingThroughCLI(t *testing.T) {
	config := &ClaudeCodeConfig{
		Enabled:      true,
		Command:      "claude",
		DefaultModel: "sonnet",
		WorkspaceDir: "/tmp/test-workspace",
	}
	provider := NewProvider(config)

	// Verify provider name
	if provider.Name() != "claude-code" {
		t.Errorf("expected provider name 'claude-code', got '%s'", provider.Name())
	}

	// Verify models are available
	models := provider.Models()
	if len(models) == 0 {
		t.Error("expected at least one model")
	}

	expectedModels := []string{"opus", "sonnet", "haiku"}
	for _, expected := range expectedModels {
		found := false
		for _, model := range models {
			if model == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected model '%s' to be available", expected)
		}
	}
}

// TestProviderBuildRunParams tests that run parameters are correctly built from LLM request.
func TestProviderBuildRunParams(t *testing.T) {
	config := &ClaudeCodeConfig{
		Enabled:      true,
		Command:      "claude",
		DefaultModel: "sonnet",
		WorkspaceDir: "/tmp/workspace",
	}
	provider := NewProvider(config)

	// Set up tool registry
	registry := tools.NewRegistry()
	tool := tools.NewMockTool("test_tool", "Test tool")
	registry.Register(tool)
	provider.SetToolRegistry(registry)

	ctx := context.Background()
	req := llm.ChatRequest{
		Model: "opus",
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "You are a helpful assistant."},
			{Role: llm.RoleUser, Content: "Hello, world!"},
		},
	}

	params, err := provider.buildRunParams(ctx, req)
	if err != nil {
		t.Fatalf("buildRunParams() error = %v", err)
	}

	// Verify prompt extraction
	if params.Prompt != "Hello, world!" {
		t.Errorf("expected prompt 'Hello, world!', got '%s'", params.Prompt)
	}

	// Verify model
	if params.Model != "opus" {
		t.Errorf("expected model 'opus', got '%s'", params.Model)
	}

	// Verify workspace dir
	if params.WorkspaceDir != "/tmp/workspace" {
		t.Errorf("expected workspace '/tmp/workspace', got '%s'", params.WorkspaceDir)
	}

	// Verify system prompt contains tools
	if !strings.Contains(params.SystemPrompt, "test_tool") {
		t.Error("expected system prompt to contain test_tool")
	}

	// Verify system prompt contains user's system message
	if !strings.Contains(params.SystemPrompt, "You are a helpful assistant.") {
		t.Error("expected system prompt to contain user's system message")
	}
}

// TestProviderSessionContextManagement tests session context is properly managed.
func TestProviderSessionContextManagement(t *testing.T) {
	config := &ClaudeCodeConfig{
		Enabled:      true,
		Command:      "claude",
		DefaultModel: "sonnet",
	}
	provider := NewProvider(config)

	// First request should create new session context
	req := llm.ChatRequest{
		Model: "opus",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "First message"},
		},
	}

	ctx := context.Background()
	params1, _ := provider.buildRunParams(ctx, req)

	// First message should have IsFirstMessage = true
	if !params1.IsFirstMessage {
		t.Error("expected IsFirstMessage to be true for first request")
	}

	// Simulate session update
	provider.updateSessionContext(params1, &RunResult{
		SessionId: "test-session-123",
		Output:    &CliOutput{Text: "Response"},
	})

	// Second request should use existing session context
	params2, _ := provider.buildRunParams(ctx, req)

	// Second message should have IsFirstMessage = false
	if params2.IsFirstMessage {
		t.Error("expected IsFirstMessage to be false for second request")
	}

	// Session ID should be set
	if params2.SessionId != "test-session-123" {
		t.Errorf("expected session ID 'test-session-123', got '%s'", params2.SessionId)
	}
}

// TestProviderResetSession tests session reset functionality.
func TestProviderResetSession(t *testing.T) {
	config := &ClaudeCodeConfig{
		Enabled:      true,
		Command:      "claude",
		DefaultModel: "sonnet",
	}
	provider := NewProvider(config)

	// Create session context
	req := llm.ChatRequest{
		Model: "opus",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "Test"},
		},
	}

	ctx := context.Background()
	provider.buildRunParams(ctx, req)
	provider.updateSessionContext(&RunParams{Model: "opus"}, &RunResult{
		SessionId: "session-to-reset",
		Output:    &CliOutput{Text: "Response"},
	})

	// Reset session
	provider.ResetSession("opus")

	// Next request should create new session context
	params, _ := provider.buildRunParams(ctx, req)
	if !params.IsFirstMessage {
		t.Error("expected IsFirstMessage to be true after reset")
	}
}

// TestProviderResetAllSessions tests resetting all sessions.
func TestProviderResetAllSessions(t *testing.T) {
	config := &ClaudeCodeConfig{
		Enabled:      true,
		Command:      "claude",
		DefaultModel: "sonnet",
	}
	provider := NewProvider(config)

	ctx := context.Background()

	// Create multiple session contexts
	models := []string{"opus", "sonnet", "haiku"}
	for _, model := range models {
		req := llm.ChatRequest{
			Model: model,
			Messages: []llm.Message{
				{Role: llm.RoleUser, Content: "Test"},
			},
		}
		provider.buildRunParams(ctx, req)
		provider.updateSessionContext(&RunParams{Model: model}, &RunResult{
			SessionId: "session-" + model,
			Output:    &CliOutput{Text: "Response"},
		})
	}

	// Reset all sessions
	provider.ResetAllSessions()

	// All sessions should be reset
	for _, model := range models {
		req := llm.ChatRequest{
			Model: model,
			Messages: []llm.Message{
				{Role: llm.RoleUser, Content: "Test"},
			},
		}
		params, _ := provider.buildRunParams(ctx, req)
		if !params.IsFirstMessage {
			t.Errorf("expected IsFirstMessage to be true for model '%s' after reset all", model)
		}
	}
}

// TestProviderEmptyToolRegistry tests behavior with empty tool registry.
func TestProviderEmptyToolRegistry(t *testing.T) {
	registry := tools.NewRegistry()
	// Don't register any tools

	config := &ClaudeCodeConfig{
		Enabled:      true,
		Command:      "claude",
		DefaultModel: "sonnet",
	}
	provider := NewProvider(config)
	provider.SetToolRegistry(registry)

	ctx := context.Background()
	systemPrompt := provider.promptBuilder.Build(ctx, "")

	// Should not contain "Available Tools" section when no tools
	if strings.Contains(systemPrompt, "Available Tools") {
		t.Error("expected system prompt to NOT contain 'Available Tools' when registry is empty")
	}

	// Should still contain runtime info
	if !strings.Contains(systemPrompt, "Runtime Information") {
		t.Error("expected system prompt to contain runtime information")
	}
}

// TestProviderNilToolRegistry tests behavior with nil tool registry.
func TestProviderNilToolRegistry(t *testing.T) {
	config := &ClaudeCodeConfig{
		Enabled:      true,
		Command:      "claude",
		DefaultModel: "sonnet",
	}
	provider := NewProvider(config)
	// Don't set tool registry

	ctx := context.Background()
	systemPrompt := provider.promptBuilder.Build(ctx, "")

	// Should not contain "Available Tools" section
	if strings.Contains(systemPrompt, "Available Tools") {
		t.Error("expected system prompt to NOT contain 'Available Tools' when registry is nil")
	}

	// Should still contain runtime info
	if !strings.Contains(systemPrompt, "Runtime Information") {
		t.Error("expected system prompt to contain runtime information")
	}
}

// TestProviderToolsIncludedInChatRequest tests that tools are included when building chat request params.
func TestProviderToolsIncludedInChatRequest(t *testing.T) {
	registry := tools.NewRegistry()

	// Register multiple tools
	toolNames := []string{"tool_a", "tool_b", "tool_c"}
	for _, name := range toolNames {
		tool := tools.NewMockTool(name, "Description for "+name)
		registry.Register(tool)
	}

	config := &ClaudeCodeConfig{
		Enabled:      true,
		Command:      "claude",
		DefaultModel: "sonnet",
		WorkspaceDir: "/workspace",
	}
	provider := NewProvider(config)
	provider.SetToolRegistry(registry)

	ctx := context.Background()
	req := llm.ChatRequest{
		Model: "opus",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "Use the tools"},
		},
	}

	params, err := provider.buildRunParams(ctx, req)
	if err != nil {
		t.Fatalf("buildRunParams() error = %v", err)
	}

	// Verify all tools are in the system prompt
	for _, name := range toolNames {
		if !strings.Contains(params.SystemPrompt, name) {
			t.Errorf("expected system prompt to contain tool '%s'", name)
		}
	}
}

// TestProviderModelSelection tests that model selection works correctly.
func TestProviderModelSelection(t *testing.T) {
	config := &ClaudeCodeConfig{
		Enabled:      true,
		Command:      "claude",
		DefaultModel: "sonnet",
	}
	provider := NewProvider(config)

	ctx := context.Background()

	// Test with explicit model
	req1 := llm.ChatRequest{
		Model: "opus",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "Test"},
		},
	}
	params1, _ := provider.buildRunParams(ctx, req1)
	if params1.Model != "opus" {
		t.Errorf("expected model 'opus', got '%s'", params1.Model)
	}

	// Test with empty model (should use default)
	req2 := llm.ChatRequest{
		Model: "",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "Test"},
		},
	}
	params2, _ := provider.buildRunParams(ctx, req2)
	if params2.Model != "sonnet" {
		t.Errorf("expected default model 'sonnet', got '%s'", params2.Model)
	}
}

// TestProviderExtractUserPrompt tests user prompt extraction from messages.
func TestProviderExtractUserPrompt(t *testing.T) {
	config := &ClaudeCodeConfig{
		Enabled:      true,
		Command:      "claude",
		DefaultModel: "sonnet",
	}
	provider := NewProvider(config)

	tests := []struct {
		name     string
		messages []llm.Message
		expected string
	}{
		{
			name: "single user message",
			messages: []llm.Message{
				{Role: llm.RoleUser, Content: "Hello"},
			},
			expected: "Hello",
		},
		{
			name: "user message with system",
			messages: []llm.Message{
				{Role: llm.RoleSystem, Content: "System prompt"},
				{Role: llm.RoleUser, Content: "User message"},
			},
			expected: "User message",
		},
		{
			name: "multiple user messages - takes last",
			messages: []llm.Message{
				{Role: llm.RoleUser, Content: "First"},
				{Role: llm.RoleAssistant, Content: "Response"},
				{Role: llm.RoleUser, Content: "Second"},
			},
			expected: "Second",
		},
		{
			name: "conversation with system",
			messages: []llm.Message{
				{Role: llm.RoleSystem, Content: "Be helpful"},
				{Role: llm.RoleUser, Content: "Question 1"},
				{Role: llm.RoleAssistant, Content: "Answer 1"},
				{Role: llm.RoleUser, Content: "Question 2"},
			},
			expected: "Question 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.extractUserPrompt(tt.messages)
			if result != tt.expected {
				t.Errorf("extractUserPrompt() = '%s', want '%s'", result, tt.expected)
			}
		})
	}
}

// TestProviderSandboxStatus tests sandbox status reporting.
func TestProviderSandboxStatus(t *testing.T) {
	config := &ClaudeCodeConfig{
		Enabled:      true,
		Command:      "claude",
		DefaultModel: "sonnet",
	}
	provider := NewProvider(config)

	status := provider.GetSandboxStatus()

	// Status should be a map with expected keys
	if _, ok := status["enabled"]; !ok {
		t.Error("expected sandbox status to have 'enabled' key")
	}
	if _, ok := status["active"]; !ok {
		t.Error("expected sandbox status to have 'active' key")
	}
	if _, ok := status["supported"]; !ok {
		t.Error("expected sandbox status to have 'supported' key")
	}
}

// TestProviderCredentialsUpdate tests credential update functionality.
func TestProviderCredentialsUpdate(t *testing.T) {
	config := &ClaudeCodeConfig{
		Enabled:      true,
		Command:      "claude",
		DefaultModel: "sonnet",
		APIKey:       "old-key",
		BaseURL:      "https://old.api.com",
	}
	provider := NewProvider(config)

	// Update credentials
	provider.UpdateCredentials("new-key", "https://new.api.com")

	// Verify credentials are updated
	apiKey, baseURL := provider.GetAPICredentials()
	if apiKey != "new-key" {
		t.Errorf("expected API key 'new-key', got '%s'", apiKey)
	}
	if baseURL != "https://new.api.com" {
		t.Errorf("expected base URL 'https://new.api.com', got '%s'", baseURL)
	}
}
