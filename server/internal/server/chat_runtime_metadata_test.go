package server

import (
	"errors"
	"testing"
)

type stubToolRuntimeError struct {
	message string
	code    string
	details map[string]interface{}
}

func (e stubToolRuntimeError) Error() string { return e.message }

func (e stubToolRuntimeError) ToolRuntimeCode() string { return e.code }

func (e stubToolRuntimeError) ToolRuntimeDetails() map[string]interface{} { return e.details }

func TestBuildStructuredClarifyQuestionForMixedIntent(t *testing.T) {
	question, ok := buildStructuredClarifyQuestion(
		"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？",
		"zh-CN",
	)
	if !ok {
		t.Fatal("expected mixed workspace/web clarify question")
	}
	if len(question.Questions) != 1 {
		t.Fatalf("Questions len = %d, want 1", len(question.Questions))
	}
	options := question.Questions[0].Options
	if len(options) != 3 {
		t.Fatalf("Options len = %d, want 3", len(options))
	}
	if got := options[0].Value; got != "prioritize_workspace" {
		t.Fatalf("options[0].value = %q, want prioritize_workspace", got)
	}
	if got := options[1].Value; got != "prioritize_web" {
		t.Fatalf("options[1].value = %q, want prioritize_web", got)
	}
	if got := options[2].Value; got != "do_both_local_first" {
		t.Fatalf("options[2].value = %q, want do_both_local_first", got)
	}
}

func TestBuildChatExecutionPlanForClarifyNone(t *testing.T) {
	plan := buildChatExecutionPlan(chatExecutionPlanInput{
		UserMessage:       "看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？",
		SelectedTools:     []string{"tool_search"},
		NativeSurfaceMode: string(chatNativeToolSurfaceModeClarifyNone),
		ClarifyReason:     "Need the user to choose between local workspace inspection and live web research before proceeding.",
	})
	if plan == nil {
		t.Fatal("expected execution plan")
	}
	if plan.NativeSurfaceMode != string(chatNativeToolSurfaceModeClarifyNone) {
		t.Fatalf("NativeSurfaceMode = %q, want %q", plan.NativeSurfaceMode, chatNativeToolSurfaceModeClarifyNone)
	}
	if len(plan.RecoveryActions) != 3 {
		t.Fatalf("RecoveryActions len = %d, want 3", len(plan.RecoveryActions))
	}
	if plan.RecoveryActions[0].Code != "prioritize_workspace" {
		t.Fatalf("RecoveryActions[0].Code = %q, want prioritize_workspace", plan.RecoveryActions[0].Code)
	}
}

func TestBuildChatRuntimeErrorEnvelope_ModelUnavailable(t *testing.T) {
	envelope := buildChatRuntimeErrorEnvelope(errors.New("No available AI provider for model 'claude-opus-4-5-20251101' across all groups checked."), chatRuntimeErrorContext{
		RetryKind: "continue",
	})
	if envelope == nil {
		t.Fatal("expected runtime error envelope")
	}
	if envelope.Code != "model_unavailable" {
		t.Fatalf("Code = %q, want model_unavailable", envelope.Code)
	}
	if envelope.RetryKind != "continue" {
		t.Fatalf("RetryKind = %q, want continue", envelope.RetryKind)
	}
	if len(envelope.RecoveryActions) == 0 || envelope.RecoveryActions[0].Code != "switch_to_auto_retry" {
		t.Fatalf("RecoveryActions = %#v, want switch_to_auto_retry first", envelope.RecoveryActions)
	}
}

func TestBuildChatRuntimeErrorEnvelope_PreservesStructuredToolCodes(t *testing.T) {
	envelope := buildChatRuntimeErrorEnvelope(stubToolRuntimeError{
		message: "tool schema incompatible",
		code:    "tool_schema_incompatible",
		details: map[string]interface{}{"schema_bytes": 98304},
	}, chatRuntimeErrorContext{
		RetryKind: "send",
	})
	if envelope == nil {
		t.Fatal("expected runtime error envelope")
	}
	if envelope.Code != "tool_schema_incompatible" {
		t.Fatalf("Code = %q, want tool_schema_incompatible", envelope.Code)
	}
	if envelope.Detail == "" {
		t.Fatal("Detail should not be empty")
	}
	if len(envelope.RecoveryActions) == 0 {
		t.Fatal("RecoveryActions should not be empty")
	}
}
