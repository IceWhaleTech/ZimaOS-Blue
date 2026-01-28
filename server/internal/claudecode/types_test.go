package claudecode

import (
	"testing"
	"time"
)

func TestCliUsage(t *testing.T) {
	usage := CliUsage{
		InputTokens:     100,
		OutputTokens:    50,
		CacheReadTokens: 10,
		TotalTokens:     150,
	}

	if usage.InputTokens != 100 {
		t.Errorf("expected InputTokens 100, got %d", usage.InputTokens)
	}

	if usage.OutputTokens != 50 {
		t.Errorf("expected OutputTokens 50, got %d", usage.OutputTokens)
	}

	if usage.TotalTokens != 150 {
		t.Errorf("expected TotalTokens 150, got %d", usage.TotalTokens)
	}
}

func TestCliOutput(t *testing.T) {
	output := CliOutput{
		Text:      "Hello, world!",
		SessionId: "test-session-123",
		Usage: CliUsage{
			InputTokens:  100,
			OutputTokens: 50,
			TotalTokens:  150,
		},
	}

	if output.Text != "Hello, world!" {
		t.Errorf("expected Text 'Hello, world!', got '%s'", output.Text)
	}

	if output.SessionId != "test-session-123" {
		t.Errorf("expected SessionId 'test-session-123', got '%s'", output.SessionId)
	}
}

func TestRunParams(t *testing.T) {
	params := RunParams{
		Prompt:         "Test prompt",
		Model:          "opus",
		SessionId:      "session-123",
		IsResume:       true,
		SystemPrompt:   "You are a helpful assistant",
		IsFirstMessage: false,
		WorkspaceDir:   "/tmp/workspace",
		Timeout:        5 * time.Minute,
	}

	if params.Prompt != "Test prompt" {
		t.Errorf("expected Prompt 'Test prompt', got '%s'", params.Prompt)
	}

	if params.Model != "opus" {
		t.Errorf("expected Model 'opus', got '%s'", params.Model)
	}

	if !params.IsResume {
		t.Error("expected IsResume to be true")
	}

	if params.IsFirstMessage {
		t.Error("expected IsFirstMessage to be false")
	}
}

func TestCliStreamChunk(t *testing.T) {
	chunk := CliStreamChunk{
		Text:      "Hello",
		SessionId: "session-123",
		Done:      false,
	}

	if chunk.Text != "Hello" {
		t.Errorf("expected Text 'Hello', got '%s'", chunk.Text)
	}

	if chunk.Done {
		t.Error("expected Done to be false")
	}

	// Test with usage
	usage := &CliUsage{TotalTokens: 100}
	chunkWithUsage := CliStreamChunk{
		Text:  "World",
		Usage: usage,
		Done:  true,
	}

	if chunkWithUsage.Usage == nil {
		t.Error("expected Usage to be set")
	}

	if !chunkWithUsage.Done {
		t.Error("expected Done to be true")
	}
}

func TestErrInvalidConfig(t *testing.T) {
	err := ErrInvalidConfig{
		Field:   "command",
		Message: "command is required",
	}

	expected := "invalid config: command: command is required"
	if err.Error() != expected {
		t.Errorf("expected error '%s', got '%s'", expected, err.Error())
	}
}

func TestErrCliExecution(t *testing.T) {
	// Test with cause
	err := ErrCliExecution{
		Command:  "claude",
		ExitCode: 1,
		Cause:    ErrTimeout{Duration: 5 * time.Minute},
	}

	if err.Unwrap() == nil {
		t.Error("expected Unwrap to return cause")
	}

	// Test with stderr
	err2 := ErrCliExecution{
		Command:  "claude",
		ExitCode: 1,
		Stderr:   "error message",
	}

	if err2.Error() == "" {
		t.Error("expected non-empty error message")
	}

	// Test without cause or stderr
	err3 := ErrCliExecution{
		Command:  "claude",
		ExitCode: 1,
	}

	expected := "CLI execution failed: claude (exit code 1)"
	if err3.Error() != expected {
		t.Errorf("expected error '%s', got '%s'", expected, err3.Error())
	}
}

func TestErrTimeout(t *testing.T) {
	err := ErrTimeout{Duration: 5 * time.Minute}

	expected := "CLI execution timed out after 5m0s"
	if err.Error() != expected {
		t.Errorf("expected error '%s', got '%s'", expected, err.Error())
	}
}

func TestErrSessionNotFound(t *testing.T) {
	err := ErrSessionNotFound{SessionId: "session-123"}

	expected := "session not found: session-123"
	if err.Error() != expected {
		t.Errorf("expected error '%s', got '%s'", expected, err.Error())
	}
}

func TestErrParseOutput(t *testing.T) {
	// Test with cause
	err := ErrParseOutput{
		Format: "json",
		Cause:  ErrInvalidConfig{Field: "test", Message: "test error"},
	}

	if err.Unwrap() == nil {
		t.Error("expected Unwrap to return cause")
	}

	// Test without cause
	err2 := ErrParseOutput{
		Format: "json",
	}

	expected := "failed to parse json output"
	if err2.Error() != expected {
		t.Errorf("expected error '%s', got '%s'", expected, err2.Error())
	}
}

func TestInputMode(t *testing.T) {
	if InputModeArg != "arg" {
		t.Errorf("expected InputModeArg 'arg', got '%s'", InputModeArg)
	}

	if InputModeStdin != "stdin" {
		t.Errorf("expected InputModeStdin 'stdin', got '%s'", InputModeStdin)
	}
}

func TestOutputFormat(t *testing.T) {
	if OutputFormatJSON != "json" {
		t.Errorf("expected OutputFormatJSON 'json', got '%s'", OutputFormatJSON)
	}

	if OutputFormatJSONL != "jsonl" {
		t.Errorf("expected OutputFormatJSONL 'jsonl', got '%s'", OutputFormatJSONL)
	}

	if OutputFormatText != "text" {
		t.Errorf("expected OutputFormatText 'text', got '%s'", OutputFormatText)
	}
}

func TestSessionMode(t *testing.T) {
	if SessionModeAlways != "always" {
		t.Errorf("expected SessionModeAlways 'always', got '%s'", SessionModeAlways)
	}

	if SessionModeExisting != "existing" {
		t.Errorf("expected SessionModeExisting 'existing', got '%s'", SessionModeExisting)
	}

	if SessionModeNone != "none" {
		t.Errorf("expected SessionModeNone 'none', got '%s'", SessionModeNone)
	}
}

func TestSystemPromptWhen(t *testing.T) {
	if SystemPromptWhenFirst != "first" {
		t.Errorf("expected SystemPromptWhenFirst 'first', got '%s'", SystemPromptWhenFirst)
	}

	if SystemPromptWhenAlways != "always" {
		t.Errorf("expected SystemPromptWhenAlways 'always', got '%s'", SystemPromptWhenAlways)
	}

	if SystemPromptWhenNever != "never" {
		t.Errorf("expected SystemPromptWhenNever 'never', got '%s'", SystemPromptWhenNever)
	}
}
