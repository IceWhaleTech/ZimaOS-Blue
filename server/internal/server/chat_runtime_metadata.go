package server

import (
	"errors"
	"fmt"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type chatRecoveryAction struct {
	Code string `json:"code"`
}

type chatExecutionPlan struct {
	Summary           string               `json:"summary,omitempty"`
	SelectedTools     []string             `json:"selected_tools,omitempty"`
	NativeSurfaceMode string               `json:"native_surface_mode,omitempty"`
	ClarifyReason     string               `json:"clarify_reason,omitempty"`
	FallbackReason    string               `json:"fallback_reason,omitempty"`
	RecoveryActions   []chatRecoveryAction `json:"recovery_actions,omitempty"`
}

type chatExecutionPlanInput struct {
	UserMessage       string
	SelectedTools     []string
	NativeSurfaceMode string
	ClarifyReason     string
	FallbackReason    string
}

type chatRuntimeErrorEnvelope struct {
	Code            string               `json:"code,omitempty"`
	Message         string               `json:"message,omitempty"`
	Detail          string               `json:"detail,omitempty"`
	RetryKind       string               `json:"retry_kind,omitempty"`
	RecoveryActions []chatRecoveryAction `json:"recovery_actions,omitempty"`
}

type chatRuntimeErrorContext struct {
	RetryKind string
}

func buildStructuredClarifyQuestion(userMessage, lang string) (tools.QuestionRequest, bool) {
	if !looksLikeMixedWorkspaceAndWebIntent(userMessage) {
		return tools.QuestionRequest{}, false
	}
	zh := strings.HasPrefix(strings.ToLower(strings.TrimSpace(lang)), "zh")
	return tools.QuestionRequest{
		Questions: []tools.QuestionItem{{
			ID:       "execution_path",
			Header:   chatRuntimeText(zh, "下一步", "Next step"),
			Question: chatRuntimeText(zh, "我应该先从哪一边开始？", "Which path should I start with?"),
			Detail: chatRuntimeText(
				zh,
				"我已经识别到这次请求同时涉及本地工作区和实时网页信息。先选一个起点，我会按这个顺序规划工具和后续恢复动作。",
				"I detected both local workspace work and live web research in this request. Pick the first lane and I will plan tools and recovery around it.",
			),
			Options: []tools.QuestionOption{
				{
					Label:       chatRuntimeText(zh, "先看本地", "Start local first"),
					Description: chatRuntimeText(zh, "先检查仓库、文件和工作区上下文。", "Inspect the repo, files, and workspace context first."),
					Value:       "prioritize_workspace",
				},
				{
					Label:       chatRuntimeText(zh, "先搜网页", "Search web first"),
					Description: chatRuntimeText(zh, "先看最新官网、文档或实时网页信息。", "Check the latest docs, official pages, or live web sources first."),
					Value:       "prioritize_web",
				},
				{
					Label:       chatRuntimeText(zh, "两者都做，但先本地", "Do both, local first"),
					Description: chatRuntimeText(zh, "两边都处理，但把本地工作区作为第一步。", "Use both, but begin with the local workspace."),
					Value:       "do_both_local_first",
				},
			},
		}},
		Context: map[string]interface{}{
			"kind":          "execution_plan_clarify",
			"original_goal": strings.TrimSpace(userMessage),
		},
	}, true
}

func buildChatExecutionPlan(input chatExecutionPlanInput) *chatExecutionPlan {
	if strings.TrimSpace(input.NativeSurfaceMode) == "" &&
		len(input.SelectedTools) == 0 &&
		strings.TrimSpace(input.ClarifyReason) == "" &&
		strings.TrimSpace(input.FallbackReason) == "" {
		return nil
	}
	plan := &chatExecutionPlan{
		Summary:           buildExecutionPlanSummary(input),
		SelectedTools:     append([]string(nil), input.SelectedTools...),
		NativeSurfaceMode: strings.TrimSpace(input.NativeSurfaceMode),
		ClarifyReason:     strings.TrimSpace(input.ClarifyReason),
		FallbackReason:    strings.TrimSpace(input.FallbackReason),
	}
	if _, ok := buildStructuredClarifyQuestion(input.UserMessage, ""); ok {
		plan.RecoveryActions = []chatRecoveryAction{
			{Code: "prioritize_workspace"},
			{Code: "prioritize_web"},
			{Code: "do_both_local_first"},
		}
	}
	return plan
}

func buildExecutionPlanSummary(input chatExecutionPlanInput) string {
	if strings.TrimSpace(input.ClarifyReason) != "" {
		return "Need a clear first step before running tools."
	}
	if strings.TrimSpace(input.FallbackReason) != "" {
		return "The runtime is adjusting the route before it continues."
	}
	if len(input.SelectedTools) > 0 {
		return fmt.Sprintf("The runtime plans to use %d selected tool(s).", len(input.SelectedTools))
	}
	return "The runtime prepared an execution plan."
}

func buildChatRuntimeErrorEnvelope(err error, ctx chatRuntimeErrorContext) *chatRuntimeErrorEnvelope {
	if err == nil {
		return nil
	}
	envelope := &chatRuntimeErrorEnvelope{
		Message:   strings.TrimSpace(err.Error()),
		Detail:    strings.TrimSpace(err.Error()),
		RetryKind: strings.TrimSpace(ctx.RetryKind),
	}

	var runtimeErr interface {
		ToolRuntimeCode() string
		ToolRuntimeDetails() map[string]interface{}
	}
	if errors.As(err, &runtimeErr) && strings.TrimSpace(runtimeErr.ToolRuntimeCode()) != "" {
		envelope.Code = strings.TrimSpace(runtimeErr.ToolRuntimeCode())
		if details := runtimeErr.ToolRuntimeDetails(); len(details) > 0 {
			envelope.Detail = fmt.Sprintf("%v", details)
		}
	} else if isModelUnavailableErrorTextServer(err.Error()) {
		envelope.Code = "model_unavailable"
	} else if strings.Contains(strings.ToLower(err.Error()), providerFailoverConfirmationRequiredError) {
		envelope.Code = providerFailoverConfirmationRequiredError
	} else {
		switch llm.NewDefaultErrorClassifier().Classify(err) {
		case llm.ErrorTypeModelUnavailable:
			envelope.Code = "model_unavailable"
		default:
			envelope.Code = "runtime_error"
		}
	}
	envelope.RecoveryActions = runtimeErrorRecoveryActions(envelope.Code)
	return envelope
}

func runtimeErrorRecoveryActions(code string) []chatRecoveryAction {
	switch strings.TrimSpace(code) {
	case "model_unavailable":
		return []chatRecoveryAction{{Code: "switch_to_auto_retry"}, {Code: "open_settings"}, {Code: "dismiss"}}
	case providerFailoverConfirmationRequiredError:
		return []chatRecoveryAction{{Code: "switch_to_auto_retry"}, {Code: "choose_route_manually"}, {Code: "dismiss"}}
	case "tool_schema_incompatible", "tool_approval_delivery_unavailable", "exec_approval_delivery_unavailable":
		return []chatRecoveryAction{{Code: "open_settings"}, {Code: "dismiss"}}
	default:
		return []chatRecoveryAction{{Code: "retry"}, {Code: "dismiss"}}
	}
}

func chatRuntimeText(zh bool, zhText, enText string) string {
	if zh {
		return zhText
	}
	return enText
}

func looksLikeMixedWorkspaceAndWebIntent(message string) bool {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	hasWorkspace := containsAnyFold(lower, "workspace", "repo", "repository", "readme", "local file", "folder", "directory") ||
		containsAnyFold(trimmed, "工作区", "仓库", "README", "本地", "目录", "文件")
	hasWeb := containsAnyFold(lower, "latest", "docs", "documentation", "official", "api", "web", "website", "search", "responses api") ||
		containsAnyFold(trimmed, "最新", "文档", "官网", "官方", "搜索", "网页", "网站")
	return hasWorkspace && hasWeb
}

func isModelUnavailableErrorTextServer(message string) bool {
	normalized := strings.TrimSpace(message)
	if normalized == "" {
		return false
	}
	lower := strings.ToLower(normalized)
	return strings.Contains(lower, "no available ai provider for model") ||
		strings.Contains(lower, "model unavailable") ||
		strings.Contains(lower, "model is not available") ||
		strings.Contains(lower, "model not configured") ||
		strings.Contains(lower, "model not found") ||
		strings.Contains(lower, "unknown model") ||
		strings.Contains(lower, "model_not_found") ||
		strings.Contains(normalized, "无可用渠道") ||
		strings.Contains(normalized, "未配置") ||
		strings.Contains(normalized, "未启用") ||
		strings.Contains(normalized, "模型不存在") ||
		strings.Contains(normalized, "未找到模型")
}
