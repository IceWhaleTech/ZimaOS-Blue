package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	a11yruntime "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/a11y"
)

type a11yChatStage string

const (
	a11yChatStageActivateApp         a11yChatStage = "activate_app"
	a11yChatStageAcquireWindow       a11yChatStage = "acquire_window"
	a11yChatStageLocateConversation  a11yChatStage = "locate_conversation"
	a11yChatStageConfirmConversation a11yChatStage = "confirm_conversation"
	a11yChatStageLocateComposer      a11yChatStage = "locate_composer"
	a11yChatStageTypeOrSend          a11yChatStage = "type_or_send"
	a11yChatStageVerifyOutcome       a11yChatStage = "verify_outcome"
)

type a11yChatStageStatus string

const (
	a11yChatStageStatusOK               a11yChatStageStatus = "ok"
	a11yChatStageStatusRetryableFailure a11yChatStageStatus = "retryable_failure"
	a11yChatStageStatusTerminalFailure  a11yChatStageStatus = "terminal_failure"
)

type a11yChatGroundingTaskHint string

const (
	a11yChatGroundingTaskLocateConversation a11yChatGroundingTaskHint = "locate_conversation"
	a11yChatGroundingTaskLocateComposer     a11yChatGroundingTaskHint = "locate_composer"
	a11yChatGroundingTaskVerifySend         a11yChatGroundingTaskHint = "verify_send"
	a11yChatGroundingTaskVerifyDraft        a11yChatGroundingTaskHint = "verify_draft"
)

type a11yChatGroundingRequest struct {
	WindowID         string
	WindowScreenshot []byte
	CropScreenshot   []byte
	TaskHint         a11yChatGroundingTaskHint
	AppProfile       string
	TargetText       string
}

type a11yChatGroundingCandidate struct {
	Role          string                     `json:"role,omitempty"`
	Label         string                     `json:"label,omitempty"`
	Bounds        a11yruntime.NormalizedRect `json:"bounds,omitempty"`
	Confidence    float64                    `json:"confidence,omitempty"`
	RationaleTags []string                   `json:"rationale_tags,omitempty"`
}

type a11yChatGroundingResult struct {
	Source       string                       `json:"source,omitempty"`
	Candidates   []a11yChatGroundingCandidate `json:"candidates,omitempty"`
	Verification map[string]interface{}       `json:"verification,omitempty"`
}

type a11yChatGrounder interface {
	Ground(ctx context.Context, req a11yChatGroundingRequest) (a11yChatGroundingResult, error)
}

type a11yChatStageRecord struct {
	Stage           string                 `json:"stage,omitempty"`
	Status          string                 `json:"status,omitempty"`
	Strategy        string                 `json:"strategy,omitempty"`
	GroundingSource string                 `json:"grounding_source,omitempty"`
	FailureCode     string                 `json:"failure_code,omitempty"`
	Verification    map[string]interface{} `json:"verification,omitempty"`
}

type a11yChatStageMemory struct {
	mu         sync.RWMutex
	strategies map[string]string
	anchors    map[string]string
}

func newA11yChatStageMemory() *a11yChatStageMemory {
	return &a11yChatStageMemory{
		strategies: make(map[string]string),
		anchors:    make(map[string]string),
	}
}

func (m *a11yChatStageMemory) Remember(platform string, appProfile string, intent string, stage string, strategy string) {
	if m == nil {
		return
	}
	key := a11yChatStageMemoryKey(platform, appProfile, intent, stage)
	strategy = strings.TrimSpace(strategy)
	if key == "" || strategy == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.strategies[key] = strategy
}

func (m *a11yChatStageMemory) RememberAnchor(platform string, appProfile string, intent string, stage string, stableID string) {
	if m == nil {
		return
	}
	key := a11yChatStageMemoryKey(platform, appProfile, intent, stage)
	stableID = strings.TrimSpace(stableID)
	if key == "" || stableID == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.anchors[key] = stableID
}

func (m *a11yChatStageMemory) OrderConversationPlans(platform string, appProfile string, intent string, plans []a11yConversationSearchPlan) []a11yConversationSearchPlan {
	if len(plans) == 0 {
		return nil
	}
	ordered := append([]a11yConversationSearchPlan(nil), plans...)
	if m == nil {
		return ordered
	}
	preferred := m.strategyFor(platform, appProfile, intent, string(a11yChatStageLocateConversation))
	if preferred == "" {
		return ordered
	}
	idx := -1
	for i, plan := range ordered {
		if strings.EqualFold(strings.TrimSpace(plan.Name), preferred) {
			idx = i
			break
		}
	}
	if idx <= 0 {
		return ordered
	}
	selected := ordered[idx]
	copy(ordered[1:idx+1], ordered[0:idx])
	ordered[0] = selected
	return ordered
}

func (m *a11yChatStageMemory) strategyFor(platform string, appProfile string, intent string, stage string) string {
	if m == nil {
		return ""
	}
	key := a11yChatStageMemoryKey(platform, appProfile, intent, stage)
	if key == "" {
		return ""
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return strings.TrimSpace(m.strategies[key])
}

func (m *a11yChatStageMemory) anchorFor(platform string, appProfile string, intent string, stage string) string {
	if m == nil {
		return ""
	}
	key := a11yChatStageMemoryKey(platform, appProfile, intent, stage)
	if key == "" {
		return ""
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return strings.TrimSpace(m.anchors[key])
}

func a11yChatStageMemoryKey(platform string, appProfile string, intent string, stage string) string {
	parts := []string{
		strings.TrimSpace(strings.ToLower(platform)),
		strings.TrimSpace(strings.ToLower(appProfile)),
		strings.TrimSpace(strings.ToLower(intent)),
		strings.TrimSpace(strings.ToLower(stage)),
	}
	for _, part := range parts {
		if part == "" {
			return ""
		}
	}
	return strings.Join(parts, "|")
}

type a11yChatExecutionState struct {
	platform        string
	appProfile      string
	intent          string
	conversation    string
	stage           a11yChatStage
	strategy        string
	groundingSource string
	attemptCount    int
	verification    map[string]interface{}
	submitEvidence  map[string]interface{}
	failureCode     string
	artifactPaths   []string
	taskStages      []a11yChatStageRecord
}

type a11yChatContextKey string

const a11yChatExecutionStateKey a11yChatContextKey = "a11y_chat_execution_state"

var a11yChatComposerConfirmationTimeout = 5 * time.Second
var a11yChatComposerConfirmationPollInterval = 250 * time.Millisecond
var a11yChatVerifyOutcomeTimeout = 5 * time.Second
var a11yChatVerifyOutcomePollInterval = 250 * time.Millisecond
var a11yChatVerifyOutcomeStructuredPollAttempts = 2

func withA11yChatExecutionState(ctx context.Context, state *a11yChatExecutionState) context.Context {
	if state == nil {
		return ctx
	}
	return context.WithValue(ctx, a11yChatExecutionStateKey, state)
}

func getA11yChatExecutionState(ctx context.Context) *a11yChatExecutionState {
	if ctx == nil {
		return nil
	}
	state, _ := ctx.Value(a11yChatExecutionStateKey).(*a11yChatExecutionState)
	return state
}

func newA11yChatExecutionState(platform string, appProfile string, intent string, conversation string) *a11yChatExecutionState {
	return &a11yChatExecutionState{
		platform:     strings.TrimSpace(strings.ToLower(platform)),
		appProfile:   strings.TrimSpace(strings.ToLower(appProfile)),
		intent:       strings.TrimSpace(strings.ToLower(intent)),
		conversation: strings.TrimSpace(conversation),
	}
}

func (s *a11yChatExecutionState) record(stage a11yChatStage, status a11yChatStageStatus, strategy string, groundingSource string, failureCode string, verification map[string]interface{}) {
	if s == nil {
		return
	}
	record := a11yChatStageRecord{
		Stage:           string(stage),
		Status:          string(status),
		Strategy:        strings.TrimSpace(strategy),
		GroundingSource: strings.TrimSpace(groundingSource),
		FailureCode:     strings.TrimSpace(failureCode),
	}
	if len(verification) > 0 {
		record.Verification = cloneA11yJSONMap(verification)
	}
	s.attemptCount++
	if stage != "" {
		s.stage = stage
	}
	if record.Strategy != "" {
		s.strategy = record.Strategy
	}
	if record.GroundingSource != "" {
		s.groundingSource = record.GroundingSource
	}
	if record.FailureCode != "" {
		s.failureCode = record.FailureCode
	}
	if len(record.Verification) > 0 {
		s.verification = cloneA11yJSONMap(record.Verification)
	}
	s.taskStages = append(s.taskStages, record)
}

func a11yRecordChatStage(ctx context.Context, state *a11yChatExecutionState, stage a11yChatStage, status a11yChatStageStatus, strategy string, groundingSource string, failureCode string, verification map[string]interface{}) {
	if state == nil {
		return
	}
	state.record(stage, status, strategy, groundingSource, failureCode, verification)
	payload := map[string]interface{}{
		"stage":         "computer_use_chat." + string(stage),
		"status":        string(status),
		"chat_stage":    string(stage),
		"attempt_count": state.attemptCount,
	}
	if trimmed := strings.TrimSpace(strategy); trimmed != "" {
		payload["strategy"] = trimmed
	}
	if trimmed := strings.TrimSpace(groundingSource); trimmed != "" {
		payload["grounding_source"] = trimmed
	}
	if trimmed := strings.TrimSpace(failureCode); trimmed != "" {
		payload["failure_code"] = trimmed
	}
	if len(verification) > 0 {
		payload["verification"] = cloneA11yJSONMap(verification)
	}
	_ = EmitEvent(ctx, ToolEvent{
		Type:     "stage_changed",
		ToolName: "computer_use",
		Message:  fmt.Sprintf("computer_use chat %s %s", stage, status),
		Payload:  payload,
	})
}

func (s *a11yChatExecutionState) addArtifactPath(path string) {
	if s == nil {
		return
	}
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return
	}
	for _, existing := range s.artifactPaths {
		if existing == trimmed {
			return
		}
	}
	s.artifactPaths = append(s.artifactPaths, trimmed)
}

func (s *a11yChatExecutionState) rememberSubmitEvidence(verification map[string]interface{}) {
	if s == nil {
		return
	}
	s.submitEvidence = cloneA11yJSONMap(verification)
}

func (s *a11yChatExecutionState) submitEvidenceSnapshot() map[string]interface{} {
	if s == nil {
		return nil
	}
	return cloneA11yJSONMap(s.submitEvidence)
}

func (s *a11yChatExecutionState) applyToPayload(payload map[string]interface{}) {
	if s == nil || payload == nil {
		return
	}
	if s.stage != "" {
		payload["stage"] = string(s.stage)
	}
	if s.strategy != "" {
		payload["strategy"] = s.strategy
	}
	if s.attemptCount > 0 {
		payload["attempt_count"] = s.attemptCount
	}
	if s.groundingSource != "" {
		payload["grounding_source"] = s.groundingSource
	}
	if s.appProfile != "" {
		payload["app_profile"] = s.appProfile
	}
	if len(s.verification) > 0 {
		payload["verification"] = cloneA11yJSONMap(s.verification)
	}
	if len(s.taskStages) > 0 {
		stages := make([]map[string]interface{}, 0, len(s.taskStages))
		for _, stage := range s.taskStages {
			item := map[string]interface{}{
				"stage":  stage.Stage,
				"status": stage.Status,
			}
			if stage.Strategy != "" {
				item["strategy"] = stage.Strategy
			}
			if stage.GroundingSource != "" {
				item["grounding_source"] = stage.GroundingSource
			}
			if stage.FailureCode != "" {
				item["failure_code"] = stage.FailureCode
			}
			if len(stage.Verification) > 0 {
				item["verification"] = cloneA11yJSONMap(stage.Verification)
			}
			stages = append(stages, item)
		}
		payload["task_stages"] = stages
	}
	if s.failureCode != "" {
		payload["failure_code"] = s.failureCode
	}
	if len(s.artifactPaths) > 0 {
		payload["artifact_paths"] = append([]string(nil), s.artifactPaths...)
	}
}

func cloneA11yJSONMap(input map[string]interface{}) map[string]interface{} {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func a11yAnnotateChatError(err error, state *a11yChatExecutionState) error {
	if err == nil || state == nil {
		return err
	}
	runtimeErr, ok := err.(*a11yruntime.RuntimeError)
	if !ok {
		runtimeErr = a11yruntime.NewError("backend_unavailable", err.Error(), nil)
	}
	details := make(map[string]interface{}, len(runtimeErr.Details)+8)
	for key, value := range runtimeErr.Details {
		details[key] = value
	}
	state.applyToPayload(details)
	phase := a11yFirstNonEmptyString(details["phase"], a11yChatStagePhase(state.stage))
	if phase != "" {
		details["phase"] = phase
	}
	if failureCode := strings.TrimSpace(a11yFirstNonEmptyString(details["failure_code"])); failureCode != "" {
		state.failureCode = failureCode
	} else if derived := a11yChatFailureCode(runtimeErr, state, phase); derived != "" {
		details["failure_code"] = derived
		state.failureCode = derived
	}
	if state.stage != "" && strings.TrimSpace(a11yFirstNonEmptyString(details["stage"])) == "" {
		details["stage"] = string(state.stage)
	}
	return a11yruntime.NewError(runtimeErr.Code, runtimeErr.Message, details)
}

func a11yChatFailureCode(runtimeErr *a11yruntime.RuntimeError, state *a11yChatExecutionState, phase string) string {
	if state != nil && strings.TrimSpace(state.failureCode) != "" {
		return strings.TrimSpace(state.failureCode)
	}
	if runtimeErr == nil {
		return ""
	}
	confirmation := strings.TrimSpace(a11yFirstNonEmptyString(runtimeErr.Details["confirmation"]))
	switch strings.TrimSpace(strings.ToLower(phase)) {
	case "conversation":
		switch {
		case confirmation == "search_box_still_active":
			return "search_box_still_active"
		case runtimeErr.Code == "fallback_exhausted":
			return "strategy_budget_exhausted"
		case runtimeErr.Code == "target_not_found":
			return "conversation_not_found"
		case runtimeErr.Code == "confirmation_failed":
			return "conversation_not_confirmed"
		}
	case "composer":
		switch runtimeErr.Code {
		case "grounding_unavailable":
			return "grounding_unavailable"
		case "target_not_found":
			if state != nil && strings.TrimSpace(state.groundingSource) != "" {
				return "composer_not_confirmed"
			}
			return "composer_not_found"
		case "confirmation_failed":
			return "composer_not_confirmed"
		}
	case "submit":
		return "send_not_verified"
	}
	if runtimeErr.Code == "fallback_exhausted" {
		return "strategy_budget_exhausted"
	}
	return ""
}

func a11yChatStagePhase(stage a11yChatStage) string {
	switch stage {
	case a11yChatStageLocateConversation, a11yChatStageConfirmConversation:
		return "conversation"
	case a11yChatStageLocateComposer:
		return "composer"
	case a11yChatStageTypeOrSend, a11yChatStageVerifyOutcome:
		return "submit"
	default:
		return ""
	}
}

func a11yFirstNonEmptyString(values ...interface{}) string {
	for _, value := range values {
		switch v := value.(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		}
	}
	return ""
}

func a11yChatHasPositiveSendVerificationCue(verification map[string]interface{}) bool {
	if len(verification) == 0 {
		return false
	}
	return compatBoolValue(verification["composer_cleared"], false)
}

func a11yMaybeWriteChatArtifact(ctx context.Context, state *a11yChatExecutionState, stage a11yChatStage, label string, data []byte) {
	if state == nil || len(data) == 0 {
		return
	}
	runID := strings.TrimSpace(GetRunID(ctx))
	if runID == "" {
		runID = "adhoc"
	}
	label = normalizeA11yTargetName(label)
	if label == "" {
		label = "artifact"
	}
	label = strings.ReplaceAll(label, " ", "_")
	path := filepath.Join(
		"artifacts",
		"computer_use",
		runID,
		fmt.Sprintf("step-%02d", GetRunStep(ctx)),
		fmt.Sprintf("%02d-%s-%s.bin", state.attemptCount+1, stage, label),
	)
	relPath, absPath, ok := a11yWriteChatArtifactFile(ctx, path, data)
	if !ok {
		return
	}
	state.addArtifactPath(relPath)
	_ = EmitArtifact(ctx, ToolArtifact{
		Kind:      "file",
		Label:     "key_screenshot",
		PathOrURL: absPath,
		MimeType:  "application/octet-stream",
		SizeBytes: int64(len(data)),
		Metadata: map[string]interface{}{
			"stage":          string(stage),
			"artifact_label": label,
		},
	})
}

func a11yMaybeWriteChatJSONArtifact(ctx context.Context, state *a11yChatExecutionState, label string, payload map[string]interface{}) {
	if state == nil || len(payload) == 0 {
		return
	}
	blob, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return
	}
	runID := strings.TrimSpace(GetRunID(ctx))
	if runID == "" {
		runID = "adhoc"
	}
	cleanLabel := strings.ReplaceAll(normalizeA11yTargetName(label), " ", "_")
	if cleanLabel == "" {
		cleanLabel = "artifact"
	}
	path := filepath.Join(
		"artifacts",
		"computer_use",
		runID,
		fmt.Sprintf("step-%02d", GetRunStep(ctx)),
		fmt.Sprintf("%02d-%s.json", state.attemptCount+1, cleanLabel),
	)
	relPath, absPath, ok := a11yWriteChatArtifactFile(ctx, path, blob)
	if !ok {
		return
	}
	state.addArtifactPath(relPath)
	_ = EmitArtifact(ctx, ToolArtifact{
		Kind:      "file",
		Label:     cleanLabel,
		PathOrURL: absPath,
		MimeType:  "application/json",
		SizeBytes: int64(len(blob)),
		Metadata: map[string]interface{}{
			"artifact_label": cleanLabel,
		},
	})
}

func a11yPersistChatTrajectoryArtifacts(ctx context.Context, state *a11yChatExecutionState, payload map[string]interface{}) {
	if state == nil || len(payload) == 0 {
		return
	}
	stageTrace := map[string]interface{}{
		"task_stages":   payload["task_stages"],
		"stage":         payload["stage"],
		"strategy":      payload["strategy"],
		"attempt_count": payload["attempt_count"],
	}
	a11yMaybeWriteChatJSONArtifact(ctx, state, "stage_trace", stageTrace)
	a11yMaybeWriteChatJSONArtifact(ctx, state, "final_result", payload)
	failure := map[string]interface{}{
		"error_code":       payload["error_code"],
		"failure_code":     payload["failure_code"],
		"phase":            payload["phase"],
		"stage":            payload["stage"],
		"grounding_source": payload["grounding_source"],
		"verification":     payload["verification"],
	}
	a11yMaybeWriteChatJSONArtifact(ctx, state, "failure_classification", failure)
}

func a11yWriteChatArtifactFile(ctx context.Context, relPath string, data []byte) (string, string, bool) {
	if relPath == "" || len(data) == 0 {
		return "", "", false
	}
	roots, aliases := GetFSScope(ctx)
	artifactRoot := strings.TrimSpace(GetRunArtifactRoot(ctx))
	if len(roots) == 0 && len(aliases) == 0 && artifactRoot == "" {
		return "", "", false
	}
	if len(roots) > 0 || len(aliases) > 0 {
		if writtenPath, err := WriteBinaryArtifact(ctx, relPath, data); err == nil {
			return writtenPath, filepath.Clean(filepath.Join(a11yArtifactBasePath(ctx), writtenPath)), true
		}
	}
	if artifactRoot == "" {
		return "", "", false
	}
	absPath := filepath.Join(artifactRoot, relPath)
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return "", "", false
	}
	if err := os.WriteFile(absPath, data, 0o644); err != nil {
		return "", "", false
	}
	return relPath, absPath, true
}

func a11yArtifactBasePath(ctx context.Context) string {
	roots, _ := GetFSScope(ctx)
	if len(roots) > 0 {
		return roots[0]
	}
	return strings.TrimSpace(GetRunArtifactRoot(ctx))
}

func (t *A11yTool) maybeStartA11yChatExecution(args map[string]interface{}, intent string, hostOS string) (*a11yChatExecutionState, *a11yConversationAppProfile) {
	if t == nil || !strings.EqualFold(strings.TrimSpace(hostOS), "darwin") {
		return nil, nil
	}
	intent = normalizeA11yActIntent(intent)
	if intent != "message" && intent != "select" {
		return nil, nil
	}
	profile := lookupA11yConversationAppProfile(args)
	if profile == nil {
		return nil, nil
	}
	return newA11yChatExecutionState(hostOS, profile.ID, intent, firstCompatString(args, "conversation", "thread", "chat", "contact")), profile
}

func (t *A11yTool) a11yChatGrounder() a11yChatGrounder {
	if t == nil {
		return nil
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.chatGrounder
}

func (t *A11yTool) a11yChatMemory() *a11yChatStageMemory {
	if t == nil {
		return nil
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.chatMemory
}

func (t *A11yTool) rememberA11yChatStrategy(state *a11yChatExecutionState, stage a11yChatStage, strategy string) {
	if t == nil || state == nil {
		return
	}
	if memory := t.a11yChatMemory(); memory != nil {
		memory.Remember(state.platform, state.appProfile, state.intent, string(stage), strategy)
	}
}

func (t *A11yTool) rememberA11yChatAnchor(state *a11yChatExecutionState, stage a11yChatStage, stableID string) {
	if t == nil || state == nil {
		return
	}
	if memory := t.a11yChatMemory(); memory != nil {
		memory.RememberAnchor(state.platform, state.appProfile, state.intent, string(stage), stableID)
	}
}

func (t *A11yTool) preferredA11yChatAnchor(state *a11yChatExecutionState, stage a11yChatStage) string {
	if state == nil {
		return ""
	}
	if memory := t.a11yChatMemory(); memory != nil {
		return memory.anchorFor(state.platform, state.appProfile, state.intent, string(stage))
	}
	return ""
}

func (t *A11yTool) orderA11yChatConversationPlans(state *a11yChatExecutionState, plans []a11yConversationSearchPlan) []a11yConversationSearchPlan {
	if state == nil {
		return append([]a11yConversationSearchPlan(nil), plans...)
	}
	if memory := t.a11yChatMemory(); memory != nil {
		return memory.OrderConversationPlans(state.platform, state.appProfile, state.intent, plans)
	}
	return append([]a11yConversationSearchPlan(nil), plans...)
}

func (t *A11yTool) groundA11yChat(ctx context.Context, req a11yChatGroundingRequest) (a11yChatGroundingResult, bool, error) {
	grounder := t.a11yChatGrounder()
	if grounder == nil {
		return a11yChatGroundingResult{}, false, nil
	}
	result, err := grounder.Ground(ctx, req)
	return result, true, err
}

func (t *A11yTool) confirmA11yChatComposerReady(ctx context.Context, backend a11yruntime.Backend, windowID string) (string, error) {
	state := getA11yChatExecutionState(ctx)
	if state == nil || state.intent != "message" {
		return strings.TrimSpace(windowID), nil
	}
	resolvedWindow := strings.TrimSpace(windowID)
	t.mu.RLock()
	cachedWindow := strings.TrimSpace(t.lastWindow)
	cachedEntries := cloneA11ySnapshotEntries(t.lastRefs)
	t.mu.RUnlock()

	entries := cachedEntries
	if cachedWindow != "" && (resolvedWindow == "" || cachedWindow == resolvedWindow) {
		resolvedWindow = cachedWindow
	} else if resolvedWindow != "" && cachedWindow != "" && cachedWindow != resolvedWindow {
		entries = nil
	}

	timeout := a11yChatComposerConfirmationTimeout
	deadline := time.Now().Add(timeout)
	snapshotAttempts := 0
	forceSnapshotRefresh := false
	visualRecoveryAttempted := false
	for {
		if snapshot, structuredWindow, ok := t.currentA11yChatStructuredSnapshot(backend, resolvedWindow); ok {
			if verification, ok := a11yStructuredSnapshotFocusedComposerVerification(snapshot); ok {
				resolvedWindow = valueOrDefault(structuredWindow, resolvedWindow)
				if stableID := strings.TrimSpace(a11yFirstNonEmptyString(verification["element_stable_id"])); stableID != "" {
					t.rememberA11yChatAnchor(state, a11yChatStageLocateComposer, stableID)
				}
				a11yRecordChatStage(ctx, state, a11yChatStageLocateComposer, a11yChatStageStatusOK, "structured_snapshot", "", "", verification)
				return resolvedWindow, nil
			}
		}
		if forceSnapshotRefresh || !a11ySnapshotHasComposer(entries) {
			if snapshotAttempts > 0 && !forceSnapshotRefresh {
				if timeout <= 0 || time.Now().After(deadline) {
					a11yRecordChatStage(ctx, state, a11yChatStageLocateComposer, a11yChatStageStatusTerminalFailure, "structured_snapshot", "", "composer_not_found", nil)
					return "", a11yruntime.NewError("target_not_found", "composer could not be confirmed", map[string]interface{}{"phase": "composer"})
				}
				a11yRecordChatStage(ctx, state, a11yChatStageLocateComposer, a11yChatStageStatusRetryableFailure, "structured_snapshot", "", "", nil)
				if err := a11yWaitForPollInterval(ctx, a11yChatComposerConfirmationPollInterval); err != nil {
					return "", err
				}
			}
			result, err := backend.SnapshotInteractive(ctx, strings.TrimSpace(windowID))
			if err != nil {
				a11yRecordChatStage(ctx, state, a11yChatStageLocateComposer, a11yChatStageStatusTerminalFailure, "structured_snapshot", "", "composer_not_found", nil)
				return "", a11yruntime.NewError("target_not_found", "composer could not be confirmed", map[string]interface{}{"phase": "composer"})
			}
			resolvedWindow = strings.TrimSpace(valueOrDefault(result.WindowID, windowID))
			t.cacheSnapshotContext(resolvedWindow, result.RefMap, result.Tree)
			entries = parseA11ySnapshotEntriesWithTokens(result.Tree, result.RefMap)
			snapshotAttempts++
			if !a11ySnapshotHasComposer(entries) && !visualRecoveryAttempted {
				recoveredWindow, attempted, recovered, recoveryErr := t.tryA11yChatComposerStructuredRecovery(ctx, backend, resolvedWindow)
				if recoveryErr != nil {
					return "", recoveryErr
				}
				if attempted && recovered {
					resolvedWindow = recoveredWindow
					forceSnapshotRefresh = true
					continue
				}
				recoveredWindow, attempted, recovered, recoveryErr = t.tryA11yChatComposerVisualRecovery(ctx, backend, resolvedWindow)
				if recoveryErr != nil {
					return "", recoveryErr
				}
				if attempted {
					visualRecoveryAttempted = true
				}
				if recovered {
					resolvedWindow = recoveredWindow
					forceSnapshotRefresh = true
					continue
				}
			}
			forceSnapshotRefresh = false
			continue
		}
		screenshot := a11yChatGroundingRequest{
			WindowID:   resolvedWindow,
			TaskHint:   a11yChatGroundingTaskLocateComposer,
			AppProfile: state.appProfile,
			TargetText: state.conversation,
		}
		if t.a11yChatGrounder() != nil {
			if grounding, ok := backend.(a11yruntime.GroundingScreenshotter); ok {
				if shot, shotErr := grounding.ScreenshotForGrounding(ctx, resolvedWindow); shotErr == nil {
					screenshot.WindowScreenshot = append([]byte(nil), shot.ImageBytes...)
					a11yMaybeWriteChatArtifact(ctx, state, a11yChatStageLocateComposer, "composer_grounding", shot.ImageBytes)
				}
			}
		}
		groundingResult, used, groundErr := t.groundA11yChat(ctx, screenshot)
		if groundErr == nil && used {
			source := strings.TrimSpace(valueOrDefault(groundingResult.Source, "grounding_model"))
			verification := cloneA11yJSONMap(groundingResult.Verification)
			if verification == nil {
				verification = map[string]interface{}{}
			}
			explicitReject := false
			if value, ok := groundingResult.Verification["rejected"].(bool); ok && value {
				explicitReject = true
			}
			searchFieldMismatch := false
			composerConfirmed := compatBoolValue(verification["composer_confirmed"], false)
			for _, candidate := range groundingResult.Candidates {
				role := normalizeA11yTargetRole(candidate.Role)
				if role == "search_field" || role == "search" {
					searchFieldMismatch = true
					continue
				}
				if role == "composer" || a11ySnapshotRoleCouldBeComposer(role, candidate.Label) {
					composerConfirmed = true
				}
			}
			if composerConfirmed {
				verification["composer_confirmed"] = true
			}
			if explicitReject {
				a11yRecordChatStage(ctx, state, a11yChatStageLocateComposer, a11yChatStageStatusTerminalFailure, "visual_grounding_check", source, "composer_not_confirmed", verification)
				return "", a11yruntime.NewError("confirmation_failed", "composer could not be confirmed", map[string]interface{}{
					"phase":            "composer",
					"grounding_source": source,
					"verification":     verification,
				})
			}
			if searchFieldMismatch && !composerConfirmed {
				if timeout <= 0 || time.Now().After(deadline) {
					a11yRecordChatStage(ctx, state, a11yChatStageLocateComposer, a11yChatStageStatusTerminalFailure, "visual_grounding_check", source, "composer_not_confirmed", verification)
					return "", a11yruntime.NewError("confirmation_failed", "composer could not be confirmed", map[string]interface{}{
						"phase":            "composer",
						"grounding_source": source,
						"verification":     verification,
					})
				}
				a11yRecordChatStage(ctx, state, a11yChatStageLocateComposer, a11yChatStageStatusRetryableFailure, "visual_grounding_check", source, "", verification)
				if err := a11yWaitForPollInterval(ctx, a11yChatComposerConfirmationPollInterval); err != nil {
					return "", err
				}
				forceSnapshotRefresh = true
				continue
			}
			a11yRecordChatStage(ctx, state, a11yChatStageLocateComposer, a11yChatStageStatusOK, "visual_grounding_check", source, "", verification)
			return resolvedWindow, nil
		}
		a11yRecordChatStage(ctx, state, a11yChatStageLocateComposer, a11yChatStageStatusOK, "structured_snapshot", "", "", nil)
		return resolvedWindow, nil
	}
}

func (t *A11yTool) tryA11yChatComposerStructuredRecovery(ctx context.Context, backend a11yruntime.Backend, windowID string) (string, bool, bool, error) {
	state := getA11yChatExecutionState(ctx)
	if state == nil || state.intent != "message" {
		return strings.TrimSpace(windowID), false, false, nil
	}
	provider, ok := backend.(a11yStructuredSnapshotProvider)
	if !ok {
		return strings.TrimSpace(windowID), false, false, nil
	}
	snapshot, ok := provider.CurrentStructuredSnapshot(strings.TrimSpace(windowID))
	if !ok || snapshot == nil || a11yStructuredSnapshotHasFocusedSearchField(snapshot) {
		return strings.TrimSpace(windowID), false, false, nil
	}
	preferredStableID := t.preferredA11yChatAnchor(state, a11yChatStageLocateComposer)
	hit, stableID, ok := resolveA11yChatComposerStructuredRecoveryHit(snapshot, preferredStableID)
	if !ok {
		return strings.TrimSpace(windowID), false, false, nil
	}
	verification := map[string]interface{}{
		"composer_confirmed": true,
		"confirmation":       "structured_bounds_recovery",
	}
	if stableID != "" {
		verification["element_stable_id"] = stableID
	}
	result, err := a11yRunActionResultWithTimeout(ctx, "point_click", windowID, func(actionCtx context.Context) (a11yruntime.ActionResult, error) {
		return backend.ClickWindowPoint(actionCtx, windowID, hit.Point, a11yruntime.DefaultHoldMS)
	})
	if err != nil {
		return "", true, false, err
	}
	resolvedWindow := strings.TrimSpace(valueOrDefault(result.WindowID, windowID))
	t.syncWindowContext(resolvedWindow)
	t.clearSnapshotRefs()
	if stableID != "" {
		t.rememberA11yChatAnchor(state, a11yChatStageLocateComposer, stableID)
	}
	t.rememberA11yChatStrategy(state, a11yChatStageLocateComposer, "structured_bounds_recovery")
	a11yRecordChatStage(ctx, state, a11yChatStageLocateComposer, a11yChatStageStatusOK, "structured_bounds_recovery", "", "", verification)
	return resolvedWindow, true, true, nil
}

func resolveA11yChatComposerStructuredRecoveryHit(snapshot *a11yruntime.Snapshot, preferredStableID string) (a11yConversationVisualHit, string, bool) {
	if snapshot == nil || len(snapshot.Nodes) == 0 {
		return a11yConversationVisualHit{}, "", false
	}
	candidates := make([]a11yConversationVisualHit, 0, 2)
	candidateStableIDs := make([]string, 0, 2)
	focusedCandidates := make([]a11yConversationVisualHit, 0, 1)
	focusedStableIDs := make([]string, 0, 1)
	for _, node := range snapshot.Nodes {
		if !a11yStructuredNodeCouldBeComposerRecoveryTarget(node) {
			continue
		}
		if node.Bounds.Width <= 0 || node.Bounds.Height <= 0 {
			continue
		}
		hit := a11yConversationVisualHit{
			Point: a11yruntime.NormalizedPoint{
				X: node.Bounds.X + node.Bounds.Width/2,
				Y: node.Bounds.Y + node.Bounds.Height/2,
			},
			Confidence: 1,
		}
		candidates = append(candidates, hit)
		candidateStableIDs = append(candidateStableIDs, strings.TrimSpace(node.StableID))
		if a11yStructuredSnapshotNodeHasExplicitFocus(node) {
			focusedCandidates = append(focusedCandidates, hit)
			focusedStableIDs = append(focusedStableIDs, strings.TrimSpace(node.StableID))
		}
	}
	preferredStableID = strings.TrimSpace(preferredStableID)
	if preferredStableID != "" {
		for idx, stableID := range focusedStableIDs {
			if stableID == preferredStableID {
				return focusedCandidates[idx], stableID, true
			}
		}
		for idx, stableID := range candidateStableIDs {
			if stableID == preferredStableID {
				return candidates[idx], stableID, true
			}
		}
	}
	if len(focusedCandidates) == 1 {
		return focusedCandidates[0], focusedStableIDs[0], true
	}
	if len(focusedCandidates) > 1 {
		return a11yConversationVisualHit{}, "", false
	}
	if len(candidates) == 1 {
		return candidates[0], candidateStableIDs[0], true
	}
	return a11yConversationVisualHit{}, "", false
}

func a11yStructuredNodeCouldBeComposerRecoveryTarget(node a11yruntime.FlatNode) bool {
	label := a11yStructuredSnapshotNodeLabel(node)
	role := normalizeA11yTargetRole(node.Role)
	if !a11ySnapshotRoleCouldBeComposer(role, label) {
		return false
	}
	if a11ySnapshotRoleLooksLikeConversationSearch(role, label) {
		return false
	}
	return node.Visible && node.Enabled
}

func a11yStructuredSnapshotConfirmsFocusedComposer(backend a11yruntime.Backend, windowID string) (map[string]interface{}, bool) {
	provider, ok := backend.(a11yStructuredSnapshotProvider)
	if !ok {
		return nil, false
	}
	snapshot, ok := provider.CurrentStructuredSnapshot(strings.TrimSpace(windowID))
	if !ok || snapshot == nil {
		return nil, false
	}
	return a11yStructuredSnapshotFocusedComposerVerification(snapshot)
}

func a11yStructuredSnapshotFocusedComposerVerification(snapshot *a11yruntime.Snapshot) (map[string]interface{}, bool) {
	if snapshot == nil {
		return nil, false
	}
	if a11yStructuredSnapshotHasFocusedSearchField(snapshot) {
		return nil, false
	}
	composer, ok := a11yStructuredSnapshotFocusedComposerNode(snapshot)
	if !ok {
		return nil, false
	}
	verification := map[string]interface{}{
		"composer_confirmed": true,
		"confirmation":       "structured_focused_composer",
	}
	if stableID := strings.TrimSpace(composer.StableID); stableID != "" {
		verification["element_stable_id"] = stableID
	}
	return verification, true
}

func a11yStructuredSnapshotHasFocusedSearchField(snapshot *a11yruntime.Snapshot) bool {
	if snapshot == nil {
		return false
	}
	for _, node := range snapshot.Nodes {
		if !a11ySnapshotRoleLooksLikeConversationSearch(node.Role, a11yStructuredSnapshotNodeLabel(node)) {
			continue
		}
		if a11yStructuredSnapshotNodeHasExplicitFocus(node) {
			return true
		}
	}
	return false
}

func a11yStructuredSnapshotHasFocusedComposer(snapshot *a11yruntime.Snapshot) bool {
	_, ok := a11yStructuredSnapshotFocusedComposerNode(snapshot)
	return ok
}

func a11yStructuredSnapshotFocusedComposerNode(snapshot *a11yruntime.Snapshot) (a11yruntime.FlatNode, bool) {
	if snapshot == nil {
		return a11yruntime.FlatNode{}, false
	}
	for _, node := range snapshot.Nodes {
		if !a11ySnapshotRoleCouldBeComposer(node.Role, a11yStructuredSnapshotNodeLabel(node)) {
			continue
		}
		if a11yStructuredSnapshotNodeHasExplicitFocus(node) {
			return node, true
		}
	}
	return a11yruntime.FlatNode{}, false
}

func a11yStructuredSnapshotComposerNodeByStableID(snapshot *a11yruntime.Snapshot, stableID string) (a11yruntime.FlatNode, bool) {
	if snapshot == nil {
		return a11yruntime.FlatNode{}, false
	}
	stableID = strings.TrimSpace(stableID)
	if stableID == "" {
		return a11yruntime.FlatNode{}, false
	}
	match := a11yruntime.FlatNode{}
	found := false
	for _, node := range snapshot.Nodes {
		if strings.TrimSpace(node.StableID) != stableID {
			continue
		}
		label := a11yStructuredSnapshotNodeLabel(node)
		if !a11ySnapshotRoleCouldBeComposer(node.Role, label) || !node.Visible || !node.Enabled {
			continue
		}
		if found {
			return a11yruntime.FlatNode{}, false
		}
		match = node
		found = true
	}
	return match, found
}

func a11yStructuredSnapshotUniqueComposerNode(snapshot *a11yruntime.Snapshot) (a11yruntime.FlatNode, bool) {
	if snapshot == nil {
		return a11yruntime.FlatNode{}, false
	}
	match := a11yruntime.FlatNode{}
	found := false
	for _, node := range snapshot.Nodes {
		label := a11yStructuredSnapshotNodeLabel(node)
		if !a11ySnapshotRoleCouldBeComposer(node.Role, label) || !node.Visible || !node.Enabled {
			continue
		}
		if normalizeA11yTargetRole(node.Role) == "search_field" || a11ySnapshotRoleLooksLikeConversationSearch(node.Role, label) {
			continue
		}
		if found {
			return a11yruntime.FlatNode{}, false
		}
		match = node
		found = true
	}
	return match, found
}

func a11yStructuredSnapshotNodeHasExplicitFocus(node a11yruntime.FlatNode) bool {
	state := normalizeA11yTargetName(strings.TrimSpace(node.Description + " " + node.State))
	return a11yTargetLabelContainsAny(state, "focused", "active", "current", "selected")
}

func a11yStructuredSnapshotNodeLabel(node a11yruntime.FlatNode) string {
	switch {
	case strings.TrimSpace(node.Name) != "":
		return strings.TrimSpace(node.Name)
	case strings.TrimSpace(node.Value) != "":
		return strings.TrimSpace(node.Value)
	default:
		return strings.TrimSpace(node.Description)
	}
}

func (t *A11yTool) tryA11yChatComposerVisualRecovery(ctx context.Context, backend a11yruntime.Backend, windowID string) (string, bool, bool, error) {
	state := getA11yChatExecutionState(ctx)
	if state == nil || state.intent != "message" {
		return strings.TrimSpace(windowID), false, false, nil
	}

	req := a11yChatGroundingRequest{
		WindowID:   strings.TrimSpace(windowID),
		TaskHint:   a11yChatGroundingTaskLocateComposer,
		AppProfile: state.appProfile,
		TargetText: state.conversation,
	}
	if t.a11yChatGrounder() != nil {
		if grounding, ok := backend.(a11yruntime.GroundingScreenshotter); ok {
			if shot, shotErr := grounding.ScreenshotForGrounding(ctx, windowID); shotErr == nil {
				req.WindowScreenshot = append([]byte(nil), shot.ImageBytes...)
				a11yMaybeWriteChatArtifact(ctx, state, a11yChatStageLocateComposer, "composer_grounding_recovery", shot.ImageBytes)
			}
		}
	}

	groundingResult, used, groundErr := t.groundA11yChat(ctx, req)
	if !used || groundErr != nil {
		return strings.TrimSpace(windowID), false, false, nil
	}

	if strings.TrimSpace(groundingResult.Source) == "" && len(groundingResult.Verification) == 0 && len(groundingResult.Candidates) == 0 {
		return strings.TrimSpace(windowID), false, false, nil
	}

	source := strings.TrimSpace(valueOrDefault(groundingResult.Source, "grounding_model"))
	verification := cloneA11yJSONMap(groundingResult.Verification)
	if verification == nil {
		verification = map[string]interface{}{}
	}
	if compatBoolValue(verification["rejected"], false) {
		a11yRecordChatStage(ctx, state, a11yChatStageLocateComposer, a11yChatStageStatusTerminalFailure, "visual_grounding_recovery", source, "composer_not_confirmed", verification)
		return "", true, false, a11yruntime.NewError("confirmation_failed", "composer could not be confirmed", map[string]interface{}{
			"phase":            "composer",
			"grounding_source": source,
			"verification":     verification,
		})
	}

	hit, ok := resolveA11yChatComposerVisualHitFromGroundingCandidates(groundingResult.Candidates)
	if !ok {
		a11yRecordChatStage(ctx, state, a11yChatStageLocateComposer, a11yChatStageStatusRetryableFailure, "visual_grounding_recovery", source, "", verification)
		return strings.TrimSpace(windowID), true, false, nil
	}
	verification["composer_confirmed"] = true

	result, err := a11yRunActionResultWithTimeout(ctx, "point_click", windowID, func(actionCtx context.Context) (a11yruntime.ActionResult, error) {
		return backend.ClickWindowPoint(actionCtx, windowID, hit.Point, a11yruntime.DefaultHoldMS)
	})
	if err != nil {
		return "", true, false, err
	}
	resolvedWindow := strings.TrimSpace(valueOrDefault(result.WindowID, windowID))
	t.syncWindowContext(resolvedWindow)
	t.clearSnapshotRefs()
	t.rememberA11yChatStrategy(state, a11yChatStageLocateComposer, "visual_grounding_recovery")
	a11yRecordChatStage(ctx, state, a11yChatStageLocateComposer, a11yChatStageStatusOK, "visual_grounding_recovery", source, "", verification)
	return resolvedWindow, true, true, nil
}

func resolveA11yChatComposerVisualHitFromGroundingCandidates(candidates []a11yChatGroundingCandidate) (a11yConversationVisualHit, bool) {
	best := a11yConversationVisualHit{}
	bestScore := 0
	tied := false
	for _, candidate := range candidates {
		score := a11yChatComposerGroundingCandidateScore(candidate)
		if score <= 0 {
			continue
		}
		hit := a11yConversationVisualHit{
			Point: a11yruntime.NormalizedPoint{
				X: candidate.Bounds.X + candidate.Bounds.Width/2,
				Y: candidate.Bounds.Y + candidate.Bounds.Height/2,
			},
			Confidence: candidate.Confidence,
		}
		switch {
		case score > bestScore:
			best = hit
			bestScore = score
			tied = false
		case score == bestScore:
			tied = true
		}
	}
	if bestScore <= 0 || tied {
		return a11yConversationVisualHit{}, false
	}
	return best, true
}

func a11yChatComposerGroundingCandidateScore(candidate a11yChatGroundingCandidate) int {
	if candidate.Confidence < a11yConversationVisualConfidenceThreshold {
		return 0
	}
	if candidate.Bounds.Width <= 0 || candidate.Bounds.Height <= 0 {
		return 0
	}
	if candidate.Bounds.X < 0 || candidate.Bounds.Y < 0 || candidate.Bounds.X+candidate.Bounds.Width > 1 || candidate.Bounds.Y+candidate.Bounds.Height > 1 {
		return 0
	}
	role := normalizeA11yTargetRole(candidate.Role)
	if role == "search_field" || role == "search" {
		return 0
	}
	if role != "composer" && !a11ySnapshotRoleCouldBeComposer(role, candidate.Label) {
		return 0
	}

	score := 100
	if role == "composer" {
		score += 80
	}
	if a11yTargetLabelContainsAny(normalizeA11yTargetName(candidate.Label), "message", "reply", "compose", "input", "chat", "消息", "回复", "输入", "发送") {
		score += 20
	}
	for _, tag := range candidate.RationaleTags {
		switch normalizeA11yTargetName(tag) {
		case "active", "current", "focused", "selected":
			score += 15
		}
	}
	return score
}

func (t *A11yTool) verifyA11yChatOutcome(ctx context.Context, backend a11yruntime.Backend, windowID string, sent bool, typedValue string) error {
	state := getA11yChatExecutionState(ctx)
	if state == nil {
		return nil
	}
	if !sent {
		a11yRecordChatStage(ctx, state, a11yChatStageVerifyOutcome, a11yChatStageStatusOK, "draft_no_outcome_verify", "", "", map[string]interface{}{"status": "drafted"})
		return nil
	}
	if verification := state.submitEvidenceSnapshot(); len(verification) > 0 {
		if _, ok := verification["status"]; !ok {
			verification["status"] = "sent"
		}
		a11yRecordChatStage(ctx, state, a11yChatStageVerifyOutcome, a11yChatStageStatusOK, "submit_phase_confirmation", "", "", verification)
		return nil
	}
	if verification, ok := t.awaitA11yStructuredPostSubmitVerification(ctx, backend, windowID, typedValue); ok {
		a11yRecordChatStage(ctx, state, a11yChatStageVerifyOutcome, a11yChatStageStatusOK, "structured_post_submit_confirmation", "", "", verification)
		return nil
	}
	if verification, ok := t.a11yStructuredPostSubmitVerification(ctx, backend, windowID, typedValue); ok {
		a11yRecordChatStage(ctx, state, a11yChatStageVerifyOutcome, a11yChatStageStatusOK, "structured_post_submit_confirmation", "", "", verification)
		return nil
	}
	taskHint := a11yChatGroundingTaskVerifyDraft
	status := "drafted"
	if sent {
		taskHint = a11yChatGroundingTaskVerifySend
		status = "sent"
	}
	timeout := a11yChatVerifyOutcomeTimeout
	deadline := time.Now().Add(timeout)
	verificationAttempts := 0
	for {
		req := a11yChatGroundingRequest{
			WindowID:   strings.TrimSpace(windowID),
			TaskHint:   taskHint,
			AppProfile: state.appProfile,
			TargetText: state.conversation,
		}
		if t.a11yChatGrounder() != nil {
			if grounding, ok := backend.(a11yruntime.GroundingScreenshotter); ok {
				if shot, shotErr := grounding.ScreenshotForGrounding(ctx, windowID); shotErr == nil {
					req.WindowScreenshot = append([]byte(nil), shot.ImageBytes...)
					a11yMaybeWriteChatArtifact(ctx, state, a11yChatStageVerifyOutcome, "verify_outcome", shot.ImageBytes)
				}
			}
		}
		groundingResult, used, groundErr := t.groundA11yChat(ctx, req)
		if groundErr == nil && used {
			source := strings.TrimSpace(valueOrDefault(groundingResult.Source, "grounding_model"))
			verification := cloneA11yJSONMap(groundingResult.Verification)
			if verification == nil {
				verification = map[string]interface{}{}
			}
			if sent {
				if statusValue := strings.TrimSpace(a11yFirstNonEmptyString(verification["status"])); statusValue != "" {
					switch a11yChatSendVerificationDisposition(state.appProfile, statusValue) {
					case "success":
						// Accept profile-specific success aliases like "delivered".
					case "retry":
						if verificationAttempts > 0 && (timeout <= 0 || time.Now().After(deadline)) {
							a11yRecordChatStage(ctx, state, a11yChatStageVerifyOutcome, a11yChatStageStatusTerminalFailure, "visual_verification", source, "send_not_verified", verification)
							return a11yruntime.NewError("confirmation_failed", "send outcome could not be verified", map[string]interface{}{
								"phase":            "submit",
								"grounding_source": source,
								"verification":     verification,
							})
						}
						a11yRecordChatStage(ctx, state, a11yChatStageVerifyOutcome, a11yChatStageStatusRetryableFailure, "visual_verification", source, "", verification)
						verificationAttempts++
						if err := a11yWaitForPollInterval(ctx, a11yChatVerifyOutcomePollInterval); err != nil {
							return err
						}
						continue
					default:
						a11yRecordChatStage(ctx, state, a11yChatStageVerifyOutcome, a11yChatStageStatusTerminalFailure, "visual_verification", source, "send_not_verified", verification)
						return a11yruntime.NewError("confirmation_failed", "send outcome could not be verified", map[string]interface{}{
							"phase":            "submit",
							"grounding_source": source,
							"verification":     verification,
						})
					}
				} else if !a11yChatHasPositiveSendVerificationCue(verification) {
					a11yRecordChatStage(ctx, state, a11yChatStageVerifyOutcome, a11yChatStageStatusTerminalFailure, "visual_verification", source, "send_not_verified", verification)
					return a11yruntime.NewError("confirmation_failed", "send outcome could not be verified", map[string]interface{}{
						"phase":            "submit",
						"grounding_source": source,
						"verification":     verification,
					})
				}
			}
			if _, ok := verification["status"]; !ok {
				verification["status"] = status
			}
			a11yRecordChatStage(ctx, state, a11yChatStageVerifyOutcome, a11yChatStageStatusOK, "visual_verification", source, "", verification)
			return nil
		}
		verification := map[string]interface{}{}
		if groundErr != nil {
			verification["reason"] = groundErr.Error()
		}
		a11yRecordChatStage(ctx, state, a11yChatStageVerifyOutcome, a11yChatStageStatusTerminalFailure, "visual_verification", "", "send_not_verified", verification)
		return a11yruntime.NewError("grounding_unavailable", "send outcome could not be verified", map[string]interface{}{
			"phase":        "submit",
			"verification": verification,
		})
	}
}

func (t *A11yTool) awaitA11yStructuredPostSubmitVerification(ctx context.Context, backend a11yruntime.Backend, windowID string, typedValue string) (map[string]interface{}, bool) {
	expected := normalizeA11ySubmitConfirmationText(strings.TrimSpace(typedValue))
	if expected == "" {
		return nil, false
	}
	state := getA11yChatExecutionState(ctx)
	conversation := ""
	preferredStableID := ""
	if state != nil {
		conversation = strings.TrimSpace(state.conversation)
		preferredStableID = t.preferredA11yChatAnchor(state, a11yChatStageLocateComposer)
	}
	resolvedWindow := strings.TrimSpace(windowID)
	attempts := a11yChatVerifyOutcomeStructuredPollAttempts
	if attempts < 1 {
		attempts = 1
	}
	for attempt := 0; attempt < attempts; attempt++ {
		snapshot, structuredWindow, ok := t.currentA11yChatStructuredSnapshot(backend, resolvedWindow)
		if !ok {
			break
		}
		resolvedWindow = valueOrDefault(structuredWindow, resolvedWindow)
		if verification, ok := a11yStructuredPostSubmitVerificationFromStructuredSnapshot(snapshot, expected, conversation, preferredStableID); ok {
			return verification, true
		}
		if attempt == attempts-1 {
			break
		}
		if err := a11yWaitForPollInterval(ctx, a11yChatVerifyOutcomePollInterval); err != nil {
			return nil, false
		}
	}
	return nil, false
}

func (t *A11yTool) a11yStructuredPostSubmitVerification(ctx context.Context, backend a11yruntime.Backend, windowID string, typedValue string) (map[string]interface{}, bool) {
	expected := normalizeA11ySubmitConfirmationText(strings.TrimSpace(typedValue))
	if expected == "" {
		return nil, false
	}
	state := getA11yChatExecutionState(ctx)
	conversation := ""
	resolvedWindow := strings.TrimSpace(windowID)
	preferredStableID := ""
	if state != nil {
		conversation = strings.TrimSpace(state.conversation)
		preferredStableID = t.preferredA11yChatAnchor(state, a11yChatStageLocateComposer)
	}
	if snapshot, structuredWindow, ok := t.currentA11yChatStructuredSnapshot(backend, resolvedWindow); ok {
		resolvedWindow = valueOrDefault(structuredWindow, resolvedWindow)
		if verification, ok := a11yStructuredPostSubmitVerificationFromStructuredSnapshot(snapshot, expected, conversation, preferredStableID); ok {
			return verification, true
		}
	}
	result, err := backend.SnapshotInteractive(ctx, resolvedWindow)
	if err != nil {
		return nil, false
	}
	resolvedWindow = strings.TrimSpace(valueOrDefault(result.WindowID, resolvedWindow))
	t.cacheSnapshotContext(resolvedWindow, result.RefMap, result.Tree)
	entries := parseA11ySnapshotEntriesWithTokens(result.Tree, result.RefMap)
	if verification, _, ok := t.currentA11yChatStructuredPostSubmitVerification(backend, resolvedWindow, expected, conversation, preferredStableID); ok {
		return verification, true
	}
	if len(entries) == 0 || !a11ySnapshotHasComposer(entries) {
		return nil, false
	}
	confirmation := buildA11ySubmitConfirmation(typedValue, 0, nil)
	if a11ySubmitSnapshotShowsPendingTypedValue(entries, confirmation, expected) {
		return nil, false
	}
	return map[string]interface{}{
		"status":            "sent",
		"composer_cleared":  true,
		"confirmation":      "structured_post_submit_confirmation",
		"grounding_skipped": true,
	}, true
}

func (t *A11yTool) currentA11yChatStructuredPostSubmitVerification(backend a11yruntime.Backend, windowID string, expected string, conversation string, preferredStableID string) (map[string]interface{}, string, bool) {
	snapshot, structuredWindow, ok := t.currentA11yChatStructuredSnapshot(backend, windowID)
	if !ok || snapshot == nil {
		return nil, "", false
	}
	verification, ok := a11yStructuredPostSubmitVerificationFromStructuredSnapshot(snapshot, expected, conversation, preferredStableID)
	if !ok {
		return nil, "", false
	}
	return verification, structuredWindow, true
}

func (t *A11yTool) a11yStructuredPostSubmitVerificationFromSnapshot(backend a11yruntime.Backend, windowID string, expected string, conversation string, preferredStableID string) (map[string]interface{}, bool) {
	provider, ok := backend.(a11yStructuredSnapshotProvider)
	if !ok {
		return nil, false
	}
	snapshot, ok := provider.CurrentStructuredSnapshot(strings.TrimSpace(windowID))
	if !ok || snapshot == nil {
		return nil, false
	}
	return a11yStructuredPostSubmitVerificationFromStructuredSnapshot(snapshot, expected, conversation, preferredStableID)
}

func a11yStructuredPostSubmitVerificationFromStructuredSnapshot(snapshot *a11yruntime.Snapshot, expected string, conversation string, preferredStableID string) (map[string]interface{}, bool) {
	if snapshot == nil {
		return nil, false
	}
	if a11yStructuredSnapshotHasFocusedSearchField(snapshot) {
		return nil, false
	}
	if a11yStructuredSnapshotHasVisibleSentMessage(snapshot, expected, conversation) {
		return map[string]interface{}{
			"status":               "sent",
			"message_list_matched": true,
			"confirmation":         "structured_sent_message_visible",
			"grounding_skipped":    true,
		}, true
	}
	if verification, ok := a11yStructuredPostSubmitComposerAnchorVerification(snapshot, preferredStableID, expected); ok {
		return verification, true
	}
	if verification, ok := a11yStructuredPostSubmitUniqueComposerVerification(snapshot, expected); ok {
		return verification, true
	}
	return a11yStructuredPostSubmitFocusedComposerVerification(snapshot, expected)
}

func a11yStructuredPostSubmitComposerAnchorVerification(snapshot *a11yruntime.Snapshot, preferredStableID string, expected string) (map[string]interface{}, bool) {
	preferredStableID = strings.TrimSpace(preferredStableID)
	if snapshot == nil || preferredStableID == "" || expected == "" {
		return nil, false
	}
	composer, ok := a11yStructuredSnapshotComposerNodeByStableID(snapshot, preferredStableID)
	if !ok {
		return nil, false
	}
	if a11ySubmitObservedTextMatches(a11yStructuredSnapshotNodeLabel(composer), expected) {
		return nil, false
	}
	return map[string]interface{}{
		"status":            "sent",
		"composer_cleared":  true,
		"confirmation":      "structured_post_submit_anchor_confirmation",
		"grounding_skipped": true,
		"element_stable_id": preferredStableID,
	}, true
}

func a11yStructuredPostSubmitFocusedComposerVerification(snapshot *a11yruntime.Snapshot, expected string) (map[string]interface{}, bool) {
	if snapshot == nil || expected == "" {
		return nil, false
	}
	composer, ok := a11yStructuredSnapshotFocusedComposerNode(snapshot)
	if !ok {
		return nil, false
	}
	if a11ySubmitObservedTextMatches(a11yStructuredSnapshotNodeLabel(composer), expected) {
		return nil, false
	}
	verification := map[string]interface{}{
		"status":            "sent",
		"composer_cleared":  true,
		"confirmation":      "structured_post_submit_confirmation",
		"grounding_skipped": true,
	}
	if stableID := strings.TrimSpace(composer.StableID); stableID != "" {
		verification["element_stable_id"] = stableID
	}
	return verification, true
}

func a11yStructuredPostSubmitUniqueComposerVerification(snapshot *a11yruntime.Snapshot, expected string) (map[string]interface{}, bool) {
	if snapshot == nil || expected == "" {
		return nil, false
	}
	composer, ok := a11yStructuredSnapshotUniqueComposerNode(snapshot)
	if !ok {
		return nil, false
	}
	if a11ySubmitObservedTextMatches(a11yStructuredSnapshotNodeLabel(composer), expected) {
		return nil, false
	}
	verification := map[string]interface{}{
		"status":            "sent",
		"composer_cleared":  true,
		"confirmation":      "structured_post_submit_confirmation",
		"grounding_skipped": true,
	}
	if stableID := strings.TrimSpace(composer.StableID); stableID != "" {
		verification["element_stable_id"] = stableID
	}
	return verification, true
}

func a11yStructuredSnapshotHasVisibleSentMessage(snapshot *a11yruntime.Snapshot, expected string, conversation string) bool {
	if snapshot == nil || expected == "" || len(snapshot.Nodes) == 0 {
		return false
	}
	nodesByID := make(map[int]a11yruntime.FlatNode, len(snapshot.Nodes))
	for _, node := range snapshot.Nodes {
		nodesByID[node.NodeID] = node
	}
	for _, node := range snapshot.Nodes {
		if !a11yStructuredNodeCouldBeSentMessageText(node) {
			continue
		}
		if a11yStructuredNodeMatchesConversationSidebar(snapshot, nodesByID, node, conversation) {
			continue
		}
		if a11ySubmitObservedTextMatches(a11yStructuredSnapshotNodeLabel(node), expected) {
			return true
		}
	}
	return false
}

func a11yStructuredNodeCouldBeSentMessageText(node a11yruntime.FlatNode) bool {
	role := normalizeA11yTargetRole(node.Role)
	label := a11yStructuredSnapshotNodeLabel(node)
	if strings.TrimSpace(label) == "" {
		return false
	}
	if a11ySnapshotRoleCouldBeComposer(role, label) || a11ySnapshotRoleLooksLikeConversationSearch(role, label) {
		return false
	}
	switch role {
	case "static_text", "text", "label":
		return true
	}
	return false
}

func a11yStructuredNodeMatchesConversationSidebar(snapshot *a11yruntime.Snapshot, nodesByID map[int]a11yruntime.FlatNode, node a11yruntime.FlatNode, conversation string) bool {
	if snapshot == nil || len(nodesByID) == 0 {
		return false
	}
	normalizedConversation := normalizeA11yTargetName(conversation)
	if normalizedConversation == "" {
		return false
	}
	parentID := node.ParentID
	for parentID != 0 {
		parent, ok := nodesByID[parentID]
		if !ok {
			return false
		}
		label := normalizeA11yTargetName(a11yStructuredSnapshotNodeLabel(parent))
		if label == normalizedConversation || strings.Contains(label, normalizedConversation) {
			if a11ySnapshotRoleIsConversation(parent.Role) || parent.Interactive {
				return true
			}
		}
		parentID = parent.ParentID
	}
	return false
}
