package server

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/logger"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
)

const autoHarnessRecentMessageLimit = 64

type AutoHarnessQuickEvalScoring struct {
	RuleProfile   string
	PassThreshold float64
}

type AutoHarnessQuickEvalItemSpec struct {
	RunKind  string
	Profile  string
	Input    map[string]interface{}
	Expected map[string]interface{}
	Metadata map[string]interface{}
}

type AutoHarnessQuickEvalSpec struct {
	Title       string
	Subject     string
	OwnerUserID string
	Metadata    map[string]interface{}
	Scoring     AutoHarnessQuickEvalScoring
	Items       []AutoHarnessQuickEvalItemSpec
}

type AutoHarnessSubmitter interface {
	SubmitAutoHarnessConversationQuickEval(ctx context.Context, spec AutoHarnessQuickEvalSpec) (string, error)
}

type autoHarnessPreset struct {
	Name          string
	Label         string
	RunKind       string
	Profile       string
	Subject       string
	RuleProfile   string
	PassThreshold float64
}

type autoHarnessTurn struct {
	Conversation *memory.Conversation
	UserMessage  *memory.Message
	ToolNames    []string
}

// AutoHarnessTurnHook turns eligible completed chat turns into ephemeral harness quick evals.
type AutoHarnessTurnHook struct {
	handler   *ChatHandler
	submitter AutoHarnessSubmitter

	seenMu  sync.Mutex
	seenIDs map[string]struct{}
}

func NewAutoHarnessTurnHook(handler *ChatHandler, submitter AutoHarnessSubmitter) *AutoHarnessTurnHook {
	if handler == nil || submitter == nil {
		return nil
	}
	return &AutoHarnessTurnHook{
		handler:   handler,
		submitter: submitter,
		seenIDs:   make(map[string]struct{}),
	}
}

func (h *AutoHarnessTurnHook) BeforeModelCall(_ context.Context, _ TurnContext) ([]llm.Message, error) {
	return nil, nil
}

func (h *AutoHarnessTurnHook) AfterAssistantPersisted(_ context.Context, turn TurnContext, assistantMsg *memory.Message) error {
	if h == nil || h.handler == nil || h.handler.store == nil || h.submitter == nil || assistantMsg == nil {
		return nil
	}
	messageID := strings.TrimSpace(assistantMsg.ID)
	if messageID == "" || !h.markSeen(messageID) {
		return nil
	}
	if turn.IsRegenerate {
		return nil
	}

	provider := strings.ToLower(strings.TrimSpace(assistantMsg.Provider))
	model := strings.ToLower(strings.TrimSpace(firstNonEmpty(assistantMsg.Model, turn.Model)))
	if provider == "local" || model == "command" || model == "offline" || model == "local-workspace-orchestration" {
		return nil
	}

	opCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	spec, triggerReasons, err := h.buildGroupSpec(opCtx, turn, assistantMsg)
	if err != nil || spec == nil {
		return err
	}
	groupID, err := h.submitter.SubmitAutoHarnessConversationQuickEval(opCtx, *spec)
	if err != nil {
		return err
	}
	quickEvalPreset := ""
	if spec.Metadata != nil {
		if value, ok := spec.Metadata["quick_eval_preset"].(string); ok {
			quickEvalPreset = strings.TrimSpace(value)
		}
	}
	h.publishTaskCreatedEvent(spec.OwnerUserID, groupID, assistantMsg, quickEvalPreset)

	logger.Info().
		Str("conversation_id", strings.TrimSpace(assistantMsg.ConversationID)).
		Str("assistant_message_id", messageID).
		Str("group_id", strings.TrimSpace(groupID)).
		Str("preset", quickEvalPreset).
		Strs("trigger_reasons", triggerReasons).
		Msg("[chat] auto harness submitted")
	return nil
}

func (h *AutoHarnessTurnHook) publishTaskCreatedEvent(userID, groupID string, assistantMsg *memory.Message, preset string) {
	if h == nil || h.handler == nil || h.handler.sseBroker == nil {
		return
	}
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		userID = "default"
	}

	payload := map[string]interface{}{
		"task_id":              groupID,
		"conversation_id":      strings.TrimSpace(assistantMsg.ConversationID),
		"assistant_message_id": strings.TrimSpace(assistantMsg.ID),
		"auto_harness":         true,
	}
	if strings.TrimSpace(preset) != "" {
		payload["quick_eval_preset"] = strings.TrimSpace(preset)
	}
	h.handler.sseBroker.Publish(userID, "task_created", payload)
}

func (h *AutoHarnessTurnHook) markSeen(messageID string) bool {
	h.seenMu.Lock()
	defer h.seenMu.Unlock()
	if _, exists := h.seenIDs[messageID]; exists {
		return false
	}
	h.seenIDs[messageID] = struct{}{}
	if len(h.seenIDs) > 4096 {
		for key := range h.seenIDs {
			if key == messageID {
				continue
			}
			delete(h.seenIDs, key)
			break
		}
	}
	return true
}

func (h *AutoHarnessTurnHook) buildGroupSpec(ctx context.Context, turn TurnContext, assistantMsg *memory.Message) (*AutoHarnessQuickEvalSpec, []string, error) {
	autoTurn, state, err := h.loadTurn(ctx, turn, assistantMsg)
	if err != nil || autoTurn == nil {
		return nil, nil, err
	}
	if state.Offline {
		return nil, nil, nil
	}

	assistantContent := strings.TrimSpace(assistantMsg.Content)
	if assistantContent == "" || isAwaitingUserInput(assistantContent) {
		return nil, nil, nil
	}
	if looksLikeInternalEvaluatorConversationTitle(autoTurn.Conversation.Title) ||
		looksLikeStructuredEvaluatorPrompt(assistantContent) {
		return nil, nil, nil
	}

	goal := strings.TrimSpace(firstNonEmpty(
		contentFromMessage(autoTurn.UserMessage),
		turn.UserMessage,
	))
	if goal == "" || len(autoTurn.ToolNames) == 0 {
		return nil, nil, nil
	}

	triggerReasons := autoHarnessTriggerReasons(goal, assistantContent)
	if len(triggerReasons) == 0 {
		return nil, nil, nil
	}

	preset := autoHarnessPresetConfig(state.DeepResearchEnabled || autoHarnessUsesResearchTools(autoTurn.ToolNames))
	conversationTitle := autoHarnessConversationTitle(autoTurn.Conversation)
	datasetName := fmt.Sprintf("%s %s Auto Harness", conversationTitle, preset.Label)
	groupTitle := datasetName
	itemMetadata := map[string]interface{}{
		"auto_harness":            true,
		"conversation_id":         strings.TrimSpace(autoTurn.Conversation.ID),
		"conversation_title":      conversationTitle,
		"assistant_message_id":    strings.TrimSpace(assistantMsg.ID),
		"assistant_created_at":    assistantMsg.CreatedAt.UTC().Format(time.RFC3339),
		"observed_tool_names":     append([]string(nil), autoTurn.ToolNames...),
		"auto_harness_reasons":    append([]string(nil), triggerReasons...),
		"auto_harness_goal":       truncateRunes(goal, 280),
		"model":                   strings.TrimSpace(firstNonEmpty(assistantMsg.Model, turn.Model)),
		"provider":                strings.TrimSpace(assistantMsg.Provider),
		"source_mode":             "conversation",
		"source_ref":              strings.TrimSpace(autoTurn.Conversation.ID),
		"quick_eval_preset":       preset.Name,
		"quick_eval_dataset_name": datasetName,
	}
	if autoTurn.UserMessage != nil {
		itemMetadata["user_message_id"] = strings.TrimSpace(autoTurn.UserMessage.ID)
	}

	contractMeta, successCriteria := autoHarnessContractMetadata(goal, autoTurn.ToolNames)
	if len(contractMeta) > 0 {
		itemMetadata["harness_contract"] = contractMeta
	}
	if len(successCriteria) > 0 {
		itemMetadata["task_success_criteria"] = append([]string(nil), successCriteria...)
	}
	expected := map[string]interface{}{
		"status": "completed",
	}
	if value, ok := contractMeta["required_observations"]; ok {
		expected["required_observations"] = value
	}
	if value, ok := contractMeta["required_checks"]; ok {
		expected["required_checks"] = value
	}
	if value, ok := contractMeta["browser_checks"]; ok {
		expected["browser_checks"] = value
	}
	if value, ok := contractMeta["api_checks"]; ok {
		expected["api_checks"] = value
	}

	groupMetadata := map[string]interface{}{
		"auto_harness":            true,
		"quick_eval":              true,
		"ephemeral":               true,
		"source_mode":             "conversation",
		"source_ref":              strings.TrimSpace(autoTurn.Conversation.ID),
		"conversation_id":         strings.TrimSpace(autoTurn.Conversation.ID),
		"conversation_title":      conversationTitle,
		"assistant_message_id":    strings.TrimSpace(assistantMsg.ID),
		"quick_eval_preset":       preset.Name,
		"quick_eval_dataset_name": datasetName,
		"quick_eval_eval_name":    datasetName + " Eval",
		"trigger_kind":            "auto_harness",
		"trigger_reasons":         append([]string(nil), triggerReasons...),
	}

	spec := &AutoHarnessQuickEvalSpec{
		Title:       groupTitle,
		Subject:     preset.Subject,
		OwnerUserID: strings.TrimSpace(autoTurn.Conversation.UserID),
		Metadata:    groupMetadata,
		Scoring: AutoHarnessQuickEvalScoring{
			RuleProfile:   preset.RuleProfile,
			PassThreshold: preset.PassThreshold,
		},
		Items: []AutoHarnessQuickEvalItemSpec{{
			RunKind: preset.RunKind,
			Profile: preset.Profile,
			Input: map[string]interface{}{
				"goal":            goal,
				"conversation_id": strings.TrimSpace(autoTurn.Conversation.ID),
			},
			Expected: expected,
			Metadata: itemMetadata,
		}},
	}
	return spec, triggerReasons, nil
}

func (h *AutoHarnessTurnHook) loadTurn(ctx context.Context, turn TurnContext, assistantMsg *memory.Message) (*autoHarnessTurn, memory.ConversationCommandState, error) {
	state := memory.ConversationCommandState{}
	if h == nil || h.handler == nil || h.handler.store == nil || assistantMsg == nil {
		return nil, state, nil
	}
	conversationID := strings.TrimSpace(firstNonEmpty(assistantMsg.ConversationID, turn.ConversationID))
	if conversationID == "" {
		return nil, state, nil
	}
	conversation, err := h.handler.store.GetConversation(ctx, conversationID)
	if err != nil || conversation == nil {
		return nil, state, err
	}

	state = h.handler.conversationCommandStateOrDefault(ctx, conversationID)
	messages, err := h.handler.store.GetRecentMessagesLite(ctx, conversationID, autoHarnessRecentMessageLimit)
	if err != nil {
		return nil, state, err
	}
	turnWindow, ok := autoHarnessCurrentTurn(messages, strings.TrimSpace(assistantMsg.ID))
	if !ok {
		return nil, state, nil
	}
	return &autoHarnessTurn{
		Conversation: conversation,
		UserMessage:  turnWindow.UserMessage,
		ToolNames:    turnWindow.ToolNames,
	}, state, nil
}

func autoHarnessCurrentTurn(messages []memory.Message, assistantMessageID string) (*autoHarnessTurn, bool) {
	assistantMessageID = strings.TrimSpace(assistantMessageID)
	if assistantMessageID == "" || len(messages) == 0 {
		return nil, false
	}
	index := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if strings.TrimSpace(messages[i].ID) == assistantMessageID {
			index = i
			break
		}
	}
	if index < 0 {
		return nil, false
	}

	var userMessage *memory.Message
	toolSet := make(map[string]struct{})
	for i := index - 1; i >= 0; i-- {
		msg := messages[i]
		if strings.EqualFold(strings.TrimSpace(msg.Role), "user") {
			captured := msg
			userMessage = &captured
			break
		}
		for _, name := range autoHarnessToolNamesFromMessage(msg) {
			if name == "" {
				continue
			}
			toolSet[name] = struct{}{}
		}
	}
	if userMessage == nil || len(toolSet) == 0 {
		return nil, false
	}

	toolNames := make([]string, 0, len(toolSet))
	for name := range toolSet {
		toolNames = append(toolNames, name)
	}
	sort.Strings(toolNames)
	return &autoHarnessTurn{
		UserMessage: userMessage,
		ToolNames:   toolNames,
	}, true
}

func autoHarnessToolNamesFromMessage(message memory.Message) []string {
	names := make([]string, 0, len(message.ToolCalls)+1)
	if name := strings.TrimSpace(message.ToolName); name != "" {
		names = append(names, name)
	}
	for _, call := range message.ToolCalls {
		if name := strings.TrimSpace(call.Name); name != "" {
			names = append(names, name)
		}
	}
	return names
}

func autoHarnessTriggerReasons(goal, assistantContent string) []string {
	reasons := make([]string, 0, 2)
	if autoHarnessHasCompletionCue(assistantContent) {
		reasons = append(reasons, "assistant_completion_cue")
	}
	if autoHarnessHasUserVerificationCue(goal) {
		reasons = append(reasons, "user_verification_cue")
	}
	return reasons
}

func autoHarnessHasCompletionCue(content string) bool {
	normalized := strings.ToLower(strings.TrimSpace(content))
	if normalized == "" {
		return false
	}
	for _, cue := range []string{
		"done", "completed", "finished", "implemented", "fixed", "resolved", "verified",
		"updated", "created", "generated", "passed", "all set", "successfully",
		"已完成", "完成了", "已经完成", "已修复", "修复了", "已经修复", "已验证", "验证通过",
		"已更新", "已创建", "现在可以", "处理完了",
	} {
		if strings.Contains(normalized, cue) {
			return true
		}
	}
	return false
}

func autoHarnessHasUserVerificationCue(content string) bool {
	normalized := strings.ToLower(strings.TrimSpace(content))
	if normalized == "" {
		return false
	}
	for _, cue := range []string{
		"verify", "verification", "validate", "check", "compare", "regression", "smoke test",
		"review", "fix", "implement", "ship", "验证", "检查", "对比", "回归", "测试", "修复", "实现", "完成",
	} {
		if strings.Contains(normalized, cue) {
			return true
		}
	}
	return false
}

func autoHarnessUsesResearchTools(toolNames []string) bool {
	for _, name := range toolNames {
		normalized := strings.ToLower(strings.TrimSpace(name))
		switch {
		case strings.HasPrefix(normalized, "web_"),
			strings.Contains(normalized, "research"),
			strings.Contains(normalized, "search"),
			strings.Contains(normalized, "crawl"),
			strings.Contains(normalized, "fetch"):
			return true
		}
	}
	return false
}

func autoHarnessUsesBrowserTools(toolNames []string) bool {
	for _, name := range toolNames {
		normalized := strings.ToLower(strings.TrimSpace(name))
		switch {
		case strings.Contains(normalized, "browser"),
			strings.Contains(normalized, "playwright"),
			strings.Contains(normalized, "puppeteer"),
			strings.Contains(normalized, "screenshot"),
			strings.Contains(normalized, "dom"):
			return true
		}
	}
	return false
}

func autoHarnessPresetConfig(research bool) autoHarnessPreset {
	if research {
		return autoHarnessPreset{
			Name:          "research",
			Label:         "Research",
			RunKind:       "research",
			Profile:       "research",
			Subject:       "research",
			RuleProfile:   "research",
			PassThreshold: 0.6,
		}
	}
	return autoHarnessPreset{
		Name:          "smoke",
		Label:         "Smoke",
		RunKind:       "agent_task",
		Profile:       "smoke",
		Subject:       "agent_task",
		RuleProfile:   "smoke",
		PassThreshold: 0.5,
	}
}

func autoHarnessConversationTitle(conversation *memory.Conversation) string {
	if conversation == nil {
		return "Conversation"
	}
	return firstNonEmpty(strings.TrimSpace(conversation.Title), "Conversation")
}

func autoHarnessContractMetadata(goal string, toolNames []string) (map[string]interface{}, []string) {
	criteria := []string{
		fmt.Sprintf("Resolve the user goal: %s", truncateRunes(strings.TrimSpace(goal), 220)),
	}
	requiredObservations := make([]string, 0, 2)
	evaluatorHints := []string{
		"This contract was auto-generated from a completed conversation turn.",
		"Prefer skeptical QA over self-attested completion.",
	}
	if autoHarnessUsesResearchTools(toolNames) {
		requiredObservations = append(requiredObservations, "evidence_tool_used")
		evaluatorHints = append(evaluatorHints, "Verify that the answer is grounded in external evidence.")
	}
	if autoHarnessUsesBrowserTools(toolNames) {
		requiredObservations = append(requiredObservations, "browser_used")
		evaluatorHints = append(evaluatorHints, "Check the browser flow instead of accepting a text-only success claim.")
	}
	metadata := map[string]interface{}{
		"success_criteria": criteria,
		"evaluator_hints":  evaluatorHints,
		"risk_level":       autoHarnessRiskLevel(toolNames),
	}
	if len(requiredObservations) > 0 {
		metadata["required_observations"] = append([]string(nil), requiredObservations...)
	}
	return metadata, criteria
}

func autoHarnessRiskLevel(toolNames []string) string {
	if autoHarnessUsesBrowserTools(toolNames) || autoHarnessUsesResearchTools(toolNames) {
		return "high"
	}
	return "medium"
}

func contentFromMessage(message *memory.Message) string {
	if message == nil {
		return ""
	}
	return strings.TrimSpace(message.Content)
}
