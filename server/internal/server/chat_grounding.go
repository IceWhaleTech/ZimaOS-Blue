package server

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agent"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

const chatGroundingVerificationFlagName = "chat_grounding_verification"

type chatFlagEvaluator interface {
	HasFlag(flagName string) bool
	IsEnabled(flagName string, ctx *config.EvaluationContext) bool
}

type chatGroundingRequest struct {
	Model      string
	Provider   string
	ProviderID string
	SessionID  string
	UserID     string
	Locale     string
	Goal       string
	Draft      string
	Messages   []llm.Message
}

type chatGroundingOutcome struct {
	Used       bool
	Status     string
	Output     string
	Violations []string
}

type chatGroundingLLMCaller struct {
	handler    *ChatHandler
	model      string
	providerID string
}

func (c chatGroundingLLMCaller) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if c.handler == nil {
		return nil, context.Canceled
	}
	req.Model = nonEmptyChatGroundingString(strings.TrimSpace(req.Model), strings.TrimSpace(c.model), "auto")
	req.Tools = nil
	req.Stream = false
	if providerID := strings.TrimSpace(c.providerID); providerID != "" {
		ctx = proxy.WithPinnedProvider(ctx, providerID)
	}
	return c.handler.chatOnce(ctx, req)
}

// SetFlagEvaluator wires runtime feature flags into chat behavior.
func (h *ChatHandler) SetFlagEvaluator(evaluator chatFlagEvaluator) {
	if h == nil {
		return
	}
	h.flagEvaluator = evaluator
}

func (h *ChatHandler) runChatGroundingVerification(ctx context.Context, req chatGroundingRequest) chatGroundingOutcome {
	draft := stripGroundingTypelessBlocks(req.Draft)
	if h == nil || strings.TrimSpace(draft) == "" {
		return chatGroundingOutcome{}
	}
	if !h.chatGroundingEnabled(req.UserID, req.SessionID) {
		return chatGroundingOutcome{}
	}

	state, toolCallIDs := buildChatGroundTruthState(req.Messages)
	if state == nil || len(toolCallIDs) == 0 {
		return chatGroundingOutcome{}
	}

	baseCtx := ctx
	if baseCtx == nil || baseCtx.Err() != nil {
		baseCtx = context.Background()
	}
	baseCtx = tools.WithRouteKind(baseCtx, tools.ToolRouteKindChat)
	if sessionID := strings.TrimSpace(req.SessionID); sessionID != "" {
		baseCtx = tools.WithSessionID(baseCtx, sessionID)
	}
	if userID := strings.TrimSpace(req.UserID); userID != "" {
		baseCtx = tools.WithUserID(baseCtx, userID)
	}
	groundingCtx, cancel := context.WithTimeout(baseCtx, 12*time.Second)
	defer cancel()

	verifier := agent.NewGroundedVerifier(nil)
	responderInput := agent.ResponderInput{
		Goal:             nonEmptyChatGroundingString(strings.TrimSpace(req.Goal), "Produce a grounded final answer."),
		Step:             agent.PlanStep{Description: strings.TrimSpace(draft)},
		GroundState:      state,
		PriorToolCallIDs: toolCallIDs,
	}
	llmCaller := chatGroundingLLMCaller{
		handler:    h,
		model:      nonEmptyChatGroundingString(strings.TrimSpace(req.Model), "auto"),
		providerID: strings.TrimSpace(req.ProviderID),
	}

	decision, violations, failureReason := h.runChatGroundingDecision(groundingCtx, verifier, llmCaller, responderInput, state)
	if failureReason != "" || decision.Status != agent.GroundingStatusGrounded {
		h.recordChatRuntimeCounter("grounding_fallback_total", map[string]string{
			"route_kind":  string(tools.ToolRouteKindChat),
			"provider":    strings.TrimSpace(req.Provider),
			"provider_id": strings.TrimSpace(req.ProviderID),
			"model":       strings.TrimSpace(req.Model),
			"status":      strings.TrimSpace(decision.Status),
			"reason":      nonEmptyChatGroundingString(strings.TrimSpace(failureReason), strings.TrimSpace(decision.Status), agent.GroundingStatusFallback),
		})
	}

	return chatGroundingOutcome{
		Used:       true,
		Status:     decision.Status,
		Output:     formatChatGroundingDecision(req.Locale, decision),
		Violations: violations,
	}
}

func (h *ChatHandler) runChatGroundingDecision(
	ctx context.Context,
	verifier *agent.GroundedVerifier,
	llmCaller chatGroundingLLMCaller,
	input agent.ResponderInput,
	state *agent.GroundTruthState,
) (agent.VerificationDecision, []string, string) {
	response, err := verifier.Respond(ctx, llmCaller, input)
	if err != nil {
		return agent.VerificationDecision{
			Valid:  false,
			Status: agent.GroundingStatusFallback,
			Output: "unknown",
		}, []string{err.Error()}, "respond_error"
	}

	decision := verifier.Verify(state, response)
	if decision.Valid {
		return decision, nil, ""
	}

	retryInput := input
	retryInput.PreviousViolations = append([]string(nil), decision.Violations...)
	retryResponse, retryErr := verifier.Respond(ctx, llmCaller, retryInput)
	if retryErr == nil {
		retryDecision := verifier.Verify(state, retryResponse)
		if retryDecision.Valid {
			return retryDecision, nil, ""
		}
		return retryDecision, append([]string(nil), retryDecision.Violations...), "verification_failed"
	}

	return decision, append([]string(nil), decision.Violations...), "retry_respond_error"
}

func buildChatGroundTruthState(messages []llm.Message) (*agent.GroundTruthState, []string) {
	if len(messages) == 0 {
		return nil, nil
	}
	stateStore := agent.NewGroundTruthStateStore()
	state := agent.NewGroundTruthState()
	calls := make(map[string]agent.GroundedToolCall)
	toolCallIDs := make([]string, 0)

	for _, msg := range messages {
		if msg.Role != llm.RoleAssistant || len(msg.ToolCalls) == 0 {
			continue
		}
		for _, tc := range msg.ToolCalls {
			callID := strings.TrimSpace(tc.ID)
			if callID == "" {
				continue
			}
			calls[callID] = buildChatGroundedToolCall(callID, strings.TrimSpace(tc.Name), tc.Arguments)
		}
	}

	seenResults := make(map[string]struct{})
	for _, msg := range messages {
		if msg.Role != llm.RoleTool {
			continue
		}
		callID := strings.TrimSpace(msg.ToolCallID)
		if callID == "" {
			continue
		}
		call := calls[callID]
		if call.ToolCallID == "" {
			call = buildChatGroundedToolCall(callID, strings.TrimSpace(msg.ToolName), "{}")
		} else if strings.TrimSpace(call.Tool) == "" && strings.TrimSpace(msg.ToolName) != "" {
			call.Tool = strings.TrimSpace(msg.ToolName)
		}
		resultValue := decodeChatGroundedToolResult(msg.Content)
		now := time.Now().UTC()
		result := agent.GroundedToolResult{
			ToolCallID: callID,
			Tool:       strings.TrimSpace(call.Tool),
			OK:         chatGroundedResultOK(resultValue),
			Result:     resultValue,
			ExitCode:   chatGroundedResultExitCode(resultValue),
			StartedAt:  now,
			FinishedAt: now,
		}
		exec := &agent.GroundedExecution{
			Call:   call,
			Result: result,
		}
		exec.Updates = stateStore.DeriveStateUpdates(call, result)
		stateStore.ApplyExecution(state, exec)
		if _, ok := seenResults[callID]; !ok {
			toolCallIDs = append(toolCallIDs, callID)
			seenResults[callID] = struct{}{}
		}
	}

	if len(toolCallIDs) == 0 {
		return nil, nil
	}
	return state, toolCallIDs
}

func buildChatGroundedToolCall(callID, toolName, arguments string) agent.GroundedToolCall {
	args := make(map[string]any)
	if parsed, ok := parseChatGroundedJSONObject(arguments); ok {
		args = parsed
	}
	return agent.GroundedToolCall{
		ToolCallID: callID,
		Tool:       strings.TrimSpace(toolName),
		Args:       args,
		CreatedAt:  time.Now().UTC(),
	}
}

func parseChatGroundedJSONObject(raw string) (map[string]any, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return map[string]any{}, true
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(trimmed), &obj); err != nil {
		return nil, false
	}
	if obj == nil {
		return map[string]any{}, true
	}
	return obj, true
}

func decodeChatGroundedToolResult(raw string) any {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return map[string]any{}
	}
	var decoded any
	if json.Unmarshal([]byte(trimmed), &decoded) == nil {
		return decoded
	}
	return trimmed
}

func chatGroundedResultOK(result any) bool {
	if asMap, ok := result.(map[string]any); ok {
		if errValue := strings.TrimSpace(anyToGroundedString(asMap["error"])); errValue != "" {
			return false
		}
		if exitCode := chatGroundedResultExitCode(result); exitCode != 0 {
			return false
		}
	}
	return true
}

func chatGroundedResultExitCode(result any) int {
	asMap, ok := result.(map[string]any)
	if !ok {
		return 0
	}
	switch value := asMap["exit_code"].(type) {
	case float64:
		return int(value)
	case int:
		return value
	case int64:
		return int(value)
	}
	return 0
}

func stripGroundingTypelessBlocks(content string) string {
	trimmed := reTypelessBlock.ReplaceAllString(content, "")
	return strings.TrimSpace(trimmed)
}

func formatChatGroundingDecision(locale string, decision agent.VerificationDecision) string {
	lines := splitChatGroundingLines(decision.Output)
	if len(lines.verified) == 0 && len(lines.unknown) == 0 {
		lines.unknown = []string{"unknown"}
	}
	verifiedLabel, unknownLabel, noteLabel := chatGroundingLabels(locale)
	var out strings.Builder
	if decision.Status == agent.GroundingStatusGrounded {
		out.WriteString(noteLabel)
		out.WriteString(chatGroundingColon(locale))
		out.WriteByte('\n')
	}
	if len(lines.verified) > 0 {
		out.WriteString(verifiedLabel)
		out.WriteString(chatGroundingColon(locale))
		out.WriteByte('\n')
		for _, line := range lines.verified {
			out.WriteString("- ")
			out.WriteString(line)
			out.WriteByte('\n')
		}
	}
	if len(lines.unknown) > 0 {
		if out.Len() > 0 {
			out.WriteByte('\n')
		}
		out.WriteString(unknownLabel)
		out.WriteString(chatGroundingColon(locale))
		out.WriteByte('\n')
		for _, line := range lines.unknown {
			out.WriteString("- ")
			out.WriteString(line)
			out.WriteByte('\n')
		}
	}
	return strings.TrimSpace(out.String())
}

func formatStreamingChatGroundingNote(locale string, decision agent.VerificationDecision) string {
	body := formatChatGroundingDecision(locale, decision)
	return wrapStreamingChatGroundingBody(locale, body)
}

func wrapStreamingChatGroundingBody(locale, body string) string {
	if strings.TrimSpace(body) == "" {
		return ""
	}
	noteLabel := "Grounding Note"
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(locale)), "zh") {
		noteLabel = "校验说明"
	}
	if strings.HasPrefix(body, noteLabel+chatGroundingColon(locale)) {
		return body
	}
	return noteLabel + chatGroundingColon(locale) + "\n" + body
}

func splitChatGroundingLines(output string) struct {
	verified []string
	unknown  []string
} {
	var lines struct {
		verified []string
		unknown  []string
	}
	seenVerified := make(map[string]struct{})
	seenUnknown := make(map[string]struct{})
	for _, rawLine := range strings.Split(strings.TrimSpace(output), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		if strings.EqualFold(line, "unknown") {
			if _, ok := seenUnknown[line]; !ok {
				lines.unknown = append(lines.unknown, "unknown")
				seenUnknown[line] = struct{}{}
			}
			continue
		}
		if _, ok := seenVerified[line]; ok {
			continue
		}
		lines.verified = append(lines.verified, line)
		seenVerified[line] = struct{}{}
	}
	return lines
}

func chatGroundingLabels(locale string) (verified, unknown, note string) {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(locale)), "zh") {
		return "已验证内容", "未验证内容", "已验证结论"
	}
	return "Verified", "Unverified", "Grounded Summary"
}

func chatGroundingColon(locale string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(locale)), "zh") {
		return "："
	}
	return ":"
}

func (h *ChatHandler) chatGroundingEnabled(userID, sessionID string) bool {
	if h == nil || h.flagEvaluator == nil {
		return false
	}
	if !h.flagEvaluator.HasFlag(chatGroundingVerificationFlagName) {
		return false
	}
	return h.flagEvaluator.IsEnabled(chatGroundingVerificationFlagName, &config.EvaluationContext{
		UserID: strings.TrimSpace(userID),
		Attributes: map[string]string{
			"route_kind": string(tools.ToolRouteKindChat),
			"session_id": strings.TrimSpace(sessionID),
		},
	})
}

func (h *ChatHandler) recordChatRuntimeCounter(name string, tags map[string]string) {
	if h == nil {
		return
	}
	recorder, ok := h.metricsRecorder.(runtimeCounterRecorder)
	if !ok || recorder == nil {
		return
	}
	if tags == nil {
		tags = make(map[string]string)
	}
	if _, ok := tags["route_kind"]; !ok {
		tags["route_kind"] = string(tools.ToolRouteKindChat)
	}
	recorder.RecordCounter(name, 1, tags)
}

func nonEmptyChatGroundingString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func anyToGroundedString(value any) string {
	if s, ok := value.(string); ok {
		return strings.TrimSpace(s)
	}
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
