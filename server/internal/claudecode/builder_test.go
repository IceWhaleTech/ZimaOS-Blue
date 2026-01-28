package claudecode

import (
	"testing"
)

func TestNewCommandBuilder(t *testing.T) {
	config := DefaultClaudeCodeBackend()
	builder := NewCommandBuilder(&config)

	if builder == nil {
		t.Fatal("expected non-nil builder")
	}

	if builder.config == nil {
		t.Fatal("expected non-nil config")
	}
}

func TestBuildArgs(t *testing.T) {
	config := DefaultClaudeCodeBackend()
	builder := NewCommandBuilder(&config)

	params := &RunParams{
		Prompt:         "Hello, world!",
		Model:          "opus",
		SessionId:      "",
		IsResume:       false,
		SystemPrompt:   "You are helpful",
		IsFirstMessage: true,
	}

	args := builder.BuildArgs(params)

	// Should contain base args
	if len(args) == 0 {
		t.Error("expected non-empty args")
	}

	// Should contain model arg
	containsModel := false
	for i, arg := range args {
		if arg == "--model" && i+1 < len(args) && args[i+1] == "opus" {
			containsModel = true
			break
		}
	}
	if !containsModel {
		t.Error("expected args to contain --model opus")
	}

	// Should contain system prompt arg (first message)
	containsSystemPrompt := false
	for i, arg := range args {
		if arg == "--append-system-prompt" && i+1 < len(args) {
			containsSystemPrompt = true
			break
		}
	}
	if !containsSystemPrompt {
		t.Error("expected args to contain --append-system-prompt")
	}
}

func TestBuildArgsResume(t *testing.T) {
	config := DefaultClaudeCodeBackend()
	builder := NewCommandBuilder(&config)

	params := &RunParams{
		Prompt:         "Continue",
		Model:          "opus",
		SessionId:      "session-123",
		IsResume:       true,
		IsFirstMessage: false,
	}

	args := builder.BuildArgs(params)

	// Should contain --resume with session ID
	containsResume := false
	for i, arg := range args {
		if arg == "--resume" && i+1 < len(args) && args[i+1] == "session-123" {
			containsResume = true
			break
		}
	}
	if !containsResume {
		t.Error("expected args to contain --resume session-123")
	}
}

func TestResolveInputMode(t *testing.T) {
	config := DefaultClaudeCodeBackend()
	builder := NewCommandBuilder(&config)

	// Short prompt should use arg mode
	mode := builder.ResolveInputMode("short prompt")
	if mode != InputModeArg {
		t.Errorf("expected InputModeArg for short prompt, got %s", mode)
	}

	// Long prompt should use stdin mode
	longPrompt := make([]byte, config.MaxPromptArgChars+1)
	for i := range longPrompt {
		longPrompt[i] = 'a'
	}
	mode = builder.ResolveInputMode(string(longPrompt))
	if mode != InputModeStdin {
		t.Errorf("expected InputModeStdin for long prompt, got %s", mode)
	}

	// Stdin config should always use stdin
	stdinConfig := config
	stdinConfig.Input = "stdin"
	stdinBuilder := NewCommandBuilder(&stdinConfig)
	mode = stdinBuilder.ResolveInputMode("short prompt")
	if mode != InputModeStdin {
		t.Errorf("expected InputModeStdin for stdin config, got %s", mode)
	}
}

func TestResolveSessionId(t *testing.T) {
	tests := []struct {
		name        string
		sessionMode string
		sessionId   string
		expectEmpty bool
	}{
		{
			name:        "always mode with no session",
			sessionMode: "always",
			sessionId:   "",
			expectEmpty: false, // Should generate new ID
		},
		{
			name:        "always mode with existing session",
			sessionMode: "always",
			sessionId:   "existing-123",
			expectEmpty: false,
		},
		{
			name:        "existing mode with no session",
			sessionMode: "existing",
			sessionId:   "",
			expectEmpty: true,
		},
		{
			name:        "existing mode with existing session",
			sessionMode: "existing",
			sessionId:   "existing-123",
			expectEmpty: false,
		},
		{
			name:        "none mode",
			sessionMode: "none",
			sessionId:   "existing-123",
			expectEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultClaudeCodeBackend()
			config.SessionMode = tt.sessionMode
			builder := NewCommandBuilder(&config)

			params := &RunParams{SessionId: tt.sessionId}
			result := builder.ResolveSessionId(params)

			if tt.expectEmpty && result != "" {
				t.Errorf("expected empty session ID, got '%s'", result)
			}
			if !tt.expectEmpty && result == "" {
				t.Error("expected non-empty session ID")
			}
			if tt.sessionId != "" && !tt.expectEmpty && result != tt.sessionId {
				// For existing mode with existing session, should return the same ID
				if tt.sessionMode == "existing" || tt.sessionMode == "always" {
					if result != tt.sessionId {
						t.Errorf("expected session ID '%s', got '%s'", tt.sessionId, result)
					}
				}
			}
		})
	}
}

func TestShouldSendSystemPrompt(t *testing.T) {
	tests := []struct {
		name           string
		when           string
		isFirstMessage bool
		expected       bool
	}{
		{
			name:           "first mode, first message",
			when:           "first",
			isFirstMessage: true,
			expected:       true,
		},
		{
			name:           "first mode, not first message",
			when:           "first",
			isFirstMessage: false,
			expected:       false,
		},
		{
			name:           "always mode, first message",
			when:           "always",
			isFirstMessage: true,
			expected:       true,
		},
		{
			name:           "always mode, not first message",
			when:           "always",
			isFirstMessage: false,
			expected:       true,
		},
		{
			name:           "never mode, first message",
			when:           "never",
			isFirstMessage: true,
			expected:       false,
		},
		{
			name:           "never mode, not first message",
			when:           "never",
			isFirstMessage: false,
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultClaudeCodeBackend()
			config.SystemPromptWhen = tt.when
			builder := NewCommandBuilder(&config)

			params := &RunParams{IsFirstMessage: tt.isFirstMessage}
			result := builder.ShouldSendSystemPrompt(params)

			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestNormalizeModel(t *testing.T) {
	config := DefaultClaudeCodeBackend()
	builder := NewCommandBuilder(&config)

	tests := []struct {
		input    string
		expected string
	}{
		{"opus", "opus"},
		{"sonnet", "sonnet"},
		{"haiku", "haiku"},
		{"claude-opus-4-5-20251101", "opus"},
		{"unknown-model", "unknown-model"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := builder.NormalizeModel(tt.input)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestGetOutputFormat(t *testing.T) {
	config := DefaultClaudeCodeBackend()
	builder := NewCommandBuilder(&config)

	// Normal run - default is now text format
	format := builder.GetOutputFormat(false)
	if format != OutputFormatText {
		t.Errorf("expected OutputFormatText, got %s", format)
	}

	// Resume run - default is now text format
	format = builder.GetOutputFormat(true)
	if format != OutputFormatText {
		t.Errorf("expected OutputFormatText for resume, got %s", format)
	}

	// Custom resume output
	config.ResumeOutput = "jsonl"
	builder2 := NewCommandBuilder(&config)
	format = builder2.GetOutputFormat(true)
	if format != OutputFormatJSONL {
		t.Errorf("expected OutputFormatJSONL for resume, got %s", format)
	}
}

func TestBuildEnv(t *testing.T) {
	config := DefaultClaudeCodeBackend()
	config.Env = map[string]string{
		"CUSTOM_VAR": "custom_value",
	}
	builder := NewCommandBuilder(&config)

	baseEnv := []string{
		"PATH=/usr/bin",
		"HOME=/home/user",
		"ANTHROPIC_API_KEY=secret",
	}

	result := builder.BuildEnv(baseEnv)

	// Should contain PATH and HOME
	containsPath := false
	containsHome := false
	containsCustom := false
	containsApiKey := false

	for _, env := range result {
		if env == "PATH=/usr/bin" {
			containsPath = true
		}
		if env == "HOME=/home/user" {
			containsHome = true
		}
		if env == "CUSTOM_VAR=custom_value" {
			containsCustom = true
		}
		if env == "ANTHROPIC_API_KEY=secret" {
			containsApiKey = true
		}
	}

	if !containsPath {
		t.Error("expected PATH in result")
	}
	if !containsHome {
		t.Error("expected HOME in result")
	}
	if !containsCustom {
		t.Error("expected CUSTOM_VAR in result")
	}
	if containsApiKey {
		t.Error("expected ANTHROPIC_API_KEY to be cleared")
	}
}

func TestBuildImageArgs(t *testing.T) {
	config := DefaultClaudeCodeBackend()
	config.ImageArg = "--image"
	config.ImageMode = "repeat"
	builder := NewCommandBuilder(&config)

	params := &RunParams{
		Prompt:     "Describe these images",
		ImagePaths: []string{"/path/to/image1.png", "/path/to/image2.png"},
	}

	args := builder.BuildArgs(params)

	// Count image args
	imageCount := 0
	for i, arg := range args {
		if arg == "--image" && i+1 < len(args) {
			imageCount++
		}
	}

	if imageCount != 2 {
		t.Errorf("expected 2 image args, got %d", imageCount)
	}

	// Test list mode
	config.ImageMode = "list"
	builder2 := NewCommandBuilder(&config)
	args2 := builder2.BuildArgs(params)

	// Should have single image arg with comma-separated list
	imageCount = 0
	for _, arg := range args2 {
		if arg == "--image" {
			imageCount++
		}
	}

	if imageCount != 1 {
		t.Errorf("expected 1 image arg in list mode, got %d", imageCount)
	}
}
