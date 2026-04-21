package server

import (
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

func TestBuildComputerUseDesktopChatContinuationNudgeWithPolicy_AddsUnsupportedActionRecoveryHint(t *testing.T) {
	policy := resolvePromptPolicy("", "")
	got := buildComputerUseDesktopChatContinuationNudgeWithPolicy(policy, false, []llm.ToolCall{
		{
			ID:        "call_open_1",
			Name:      "computer_use",
			Arguments: `{"action":"open","app_name":"Feishu"}`,
		},
	}, []llm.Message{
		{
			Role:       llm.RoleTool,
			ToolCallID: "call_open_1",
			ToolName:   "computer_use",
			Content:    `{"status":"error","error":"invalid action: open","error_code":"unsupported_action"}`,
		},
	})

	required := []string{
		"desktop chat or messaging task",
		"`message`, `select`, and `type`",
		"Recovery hint:",
		"`open` is not a supported computer_use action",
		"do not repeat `open`",
	}
	for _, want := range required {
		if !strings.Contains(got, want) {
			t.Fatalf("expected nudge to include %q, got %q", want, got)
		}
	}
}

func TestBuildComputerUseDesktopChatContinuationNudgeWithPolicy_AddsMissingMessageValueRecoveryHint(t *testing.T) {
	policy := resolvePromptPolicy("", "")
	got := buildComputerUseDesktopChatContinuationNudgeWithPolicy(policy, false, []llm.ToolCall{
		{
			ID:        "call_message_1",
			Name:      "computer_use",
			Arguments: `{"action":"message","conversation":"后端之家","intent":"search"}`,
		},
	}, []llm.Message{
		{
			Role:       llm.RoleTool,
			ToolCallID: "call_message_1",
			ToolName:   "computer_use",
			Content:    `{"status":"error","error":"value is required for intent=message","error_code":"invalid_argument"}`,
		},
	})

	required := []string{
		"Recovery hint:",
		"`action=message`",
		"include a non-empty `value`",
	}
	for _, want := range required {
		if !strings.Contains(got, want) {
			t.Fatalf("expected nudge to include %q, got %q", want, got)
		}
	}
}

func TestBuildComputerUseDesktopChatContinuationNudgeWithPolicy_AddsTopLevelPressRecoveryHint(t *testing.T) {
	policy := resolvePromptPolicy("", "")
	got := buildComputerUseDesktopChatContinuationNudgeWithPolicy(policy, false, []llm.ToolCall{
		{
			ID:        "call_press_1",
			Name:      "computer_use",
			Arguments: `{"action":"press","submit_keys":["enter"]}`,
		},
	}, []llm.Message{
		{
			Role:       llm.RoleTool,
			ToolCallID: "call_press_1",
			ToolName:   "computer_use",
			Content:    `{"status":"error","error":"ref is required for act unless target_name or target_role is provided","error_code":"invalid_argument"}`,
		},
	})

	required := []string{
		"Recovery hint:",
		"top-level `press`",
		"`action=key`",
		"`keys` or `submit_keys`",
	}
	for _, want := range required {
		if !strings.Contains(got, want) {
			t.Fatalf("expected nudge to include %q, got %q", want, got)
		}
	}
}
