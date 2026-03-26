package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

// RuntimeState represents the explicit orchestrator FSM state.
type RuntimeState string

const (
	RuntimeStateIntake      RuntimeState = "INTAKE"
	RuntimeStateClarify     RuntimeState = "CLARIFY"
	RuntimeStatePlan        RuntimeState = "PLAN"
	RuntimeStateConfirmGate RuntimeState = "CONFIRM_GATE"
	RuntimeStateExecute     RuntimeState = "EXECUTE"
	RuntimeStateVerify      RuntimeState = "VERIFY"
	RuntimeStateReflect     RuntimeState = "REFLECT"
	RuntimeStateReport      RuntimeState = "REPORT"
	RuntimeStateRecover     RuntimeState = "RECOVER"
	RuntimeStateDone        RuntimeState = "DONE"
	RuntimeStateAborted     RuntimeState = "ABORTED"
)

// RuntimeAuditEvent records deterministic state transitions and routing decisions.
type RuntimeAuditEvent struct {
	Timestamp  time.Time       `json:"timestamp"`
	From       RuntimeState    `json:"from,omitempty"`
	To         RuntimeState    `json:"to,omitempty"`
	Reason     string          `json:"reason,omitempty"`
	Capability *CapabilityInfo `json:"capability,omitempty"`
	Error      string          `json:"error,omitempty"`
}

// CapabilityKind identifies where a call is routed.
type CapabilityKind string

const (
	CapabilityKindTool  CapabilityKind = "tool"
	CapabilityKindSkill CapabilityKind = "skill"
	CapabilityKindMCP   CapabilityKind = "mcp"
)

// CapabilityInfo standardizes call metadata for deterministic routing/auditing.
type CapabilityInfo struct {
	Name             string         `json:"name"`
	Kind             CapabilityKind `json:"kind"`
	RiskLevel        string         `json:"risk_level"`
	DeterminismScore float64        `json:"determinism_score"`
	Idempotent       bool           `json:"idempotent"`
	LatencyProfile   string         `json:"latency_profile,omitempty"`
	CostProfile      string         `json:"cost_profile,omitempty"`
}

var runtimeTransitions = map[RuntimeState]map[RuntimeState]struct{}{
	RuntimeStateIntake: {
		RuntimeStateClarify: {},
		RuntimeStatePlan:    {},
		RuntimeStateAborted: {},
	},
	RuntimeStateClarify: {
		RuntimeStatePlan:    {},
		RuntimeStateReflect: {},
		RuntimeStateReport:  {},
		RuntimeStateAborted: {},
	},
	RuntimeStatePlan: {
		RuntimeStateConfirmGate: {},
		RuntimeStateExecute:     {},
		RuntimeStateReflect:     {},
		RuntimeStateReport:      {},
		RuntimeStateAborted:     {},
	},
	RuntimeStateConfirmGate: {
		RuntimeStateExecute: {},
		RuntimeStatePlan:    {},
		RuntimeStateRecover: {},
		RuntimeStateReflect: {},
		RuntimeStateReport:  {},
		RuntimeStateAborted: {},
	},
	RuntimeStateExecute: {
		RuntimeStateVerify:      {},
		RuntimeStateConfirmGate: {},
		RuntimeStateRecover:     {},
		RuntimeStateReflect:     {},
		RuntimeStateReport:      {},
		RuntimeStateAborted:     {},
	},
	RuntimeStateVerify: {
		RuntimeStateReflect: {},
		RuntimeStateReport:  {},
		RuntimeStateRecover: {},
		RuntimeStateAborted: {},
	},
	RuntimeStateRecover: {
		RuntimeStateExecute:     {},
		RuntimeStateVerify:      {},
		RuntimeStateConfirmGate: {},
		RuntimeStateReflect:     {},
		RuntimeStateReport:      {},
		RuntimeStateAborted:     {},
	},
	RuntimeStateReflect: {
		RuntimeStateReport: {},
		RuntimeStateDone:   {},
	},
	RuntimeStateReport: {
		RuntimeStateDone: {},
	},
	RuntimeStateDone:    {},
	RuntimeStateAborted: {},
}

func canTransition(from, to RuntimeState) bool {
	if from == "" {
		return to == RuntimeStateIntake
	}
	allowed, ok := runtimeTransitions[from]
	if !ok {
		return false
	}
	_, ok = allowed[to]
	return ok
}

type runtimePlan struct {
	Goal                 string   `json:"goal"`
	Subtasks             []string `json:"subtasks"`
	RequiresConfirmation []string `json:"requires_confirmation"`
	SuccessCriteria      []string `json:"success_criteria"`
	FallbackPlan         []string `json:"fallback_plan"`
}

func parseRuntimePlan(content string) (runtimePlan, error) {
	var out runtimePlan

	trimmed := strings.TrimSpace(content)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)

	var rawObj struct {
		Goal     string `json:"goal"`
		Subtasks []struct {
			Description string `json:"description"`
		} `json:"subtasks"`
		RequiresConfirmation []string `json:"requires_confirmation"`
		SuccessCriteria      []string `json:"success_criteria"`
		FallbackPlan         []string `json:"fallback_plan"`
	}
	if err := json.Unmarshal([]byte(trimmed), &rawObj); err == nil && len(rawObj.Subtasks) > 0 {
		out.Goal = strings.TrimSpace(rawObj.Goal)
		for _, st := range rawObj.Subtasks {
			if d := strings.TrimSpace(st.Description); d != "" {
				out.Subtasks = append(out.Subtasks, d)
			}
		}
		out.RequiresConfirmation = dedupeStrings(rawObj.RequiresConfirmation)
		out.SuccessCriteria = dedupeStrings(rawObj.SuccessCriteria)
		out.FallbackPlan = dedupeStrings(rawObj.FallbackPlan)
		if len(out.Subtasks) > 0 {
			return fillRuntimePlanDefaults(out), nil
		}
	}

	var rawSteps []struct {
		Description string `json:"description"`
	}
	if err := json.Unmarshal([]byte(trimmed), &rawSteps); err != nil {
		return runtimePlan{}, fmt.Errorf("failed to parse runtime plan: %w", err)
	}
	for _, step := range rawSteps {
		if d := strings.TrimSpace(step.Description); d != "" {
			out.Subtasks = append(out.Subtasks, d)
		}
	}
	if len(out.Subtasks) == 0 {
		return runtimePlan{}, fmt.Errorf("runtime plan has no subtasks")
	}
	return fillRuntimePlanDefaults(out), nil
}

func fillRuntimePlanDefaults(p runtimePlan) runtimePlan {
	p.SuccessCriteria = effectiveSuccessCriteria(p.SuccessCriteria)
	p.FallbackPlan = effectiveFallbackPlan(p.FallbackPlan)
	return p
}

func dedupeStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, raw := range in {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func classifyCapability(name, argsJSON string) CapabilityInfo {
	c := CapabilityInfo{
		Name:             name,
		Kind:             CapabilityKindTool,
		RiskLevel:        "low",
		DeterminismScore: 0.92,
		Idempotent:       true,
		LatencyProfile:   "low",
		CostProfile:      "low",
	}
	if strings.HasPrefix(name, "mcp_") || strings.HasPrefix(name, "mcp.") {
		c.Kind = CapabilityKindMCP
		c.DeterminismScore = 0.62
		c.LatencyProfile = "medium"
		c.CostProfile = "medium"
	}
	if normalizeGroundToolName(name) == "bash" {
		cmd := parseExecCommand(argsJSON)
		if strings.HasPrefix(cmd, "blue ") {
			c.Kind = CapabilityKindSkill
			c.DeterminismScore = 0.78
			c.LatencyProfile = "medium"
		}
		risk := tools.AnalyzeRisk(cmd)
		c.RiskLevel = string(risk.Level)
		c.Idempotent = risk.Total < 30
		if risk.Total >= 60 {
			c.CostProfile = "high"
		}
	}
	if !isReadLikeTool(name) {
		c.Idempotent = false
	}
	return c
}

func parseExecCommand(argsJSON string) string {
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(argsJSON), &raw); err != nil {
		return ""
	}
	cmd, _ := raw["command"].(string)
	return strings.TrimSpace(cmd)
}

func isReadLikeTool(name string) bool {
	switch normalizeGroundToolName(name) {
	case "read", "memory", "web_search", "analyze", "ui_reviewer", "ask":
		return true
	default:
		return false
	}
}

func shouldRequireConfirm(cap CapabilityInfo, contextText string) bool {
	if cap.Name == "ask" {
		return false
	}
	if cap.RiskLevel == "high" || cap.RiskLevel == "critical" {
		return true
	}
	lower := strings.ToLower(contextText)
	if strings.Contains(lower, "production") || strings.Contains(lower, "delete") || strings.Contains(lower, "drop") || strings.Contains(lower, "publish") {
		return true
	}
	return false
}

func requiresClarification(goal string) bool {
	g := strings.TrimSpace(goal)
	if g == "" {
		return true
	}
	lower := strings.ToLower(g)
	return strings.Contains(lower, "tbd") || strings.Contains(lower, "to be decided")
}

func planNeedsConfirmation(plan runtimePlan) bool {
	if len(plan.RequiresConfirmation) > 0 {
		return true
	}
	for _, step := range plan.Subtasks {
		lower := strings.ToLower(step)
		if strings.Contains(lower, "production") || strings.Contains(lower, "delete") || strings.Contains(lower, "drop") || strings.Contains(lower, "publish") {
			return true
		}
	}
	return false
}

type GroundedRuntimeConfig struct {
	PlannerLLM       LLMCaller
	ResponderLLM     LLMCaller
	Registry         *tools.Registry
	Executor         *tools.Executor
	Store            *Store
	StateStore       *GroundTruthStateStore
	Planner          *GroundedPlanner
	GroundedExecutor *GroundedExecutor
	Verifier         *GroundedVerifier
	AskHandler       func(ctx context.Context, task *Task, argsJSON string) string
	ConfirmToolCall  func(ctx context.Context, task *Task, step PlanStep, nextTool PlannerToolCall) error
	Secret           []byte
	MaxPlannerRounds int
}

type GroundedRuntime struct {
	planner          *GroundedPlanner
	responderLLM     LLMCaller
	registry         *tools.Registry
	executor         *GroundedExecutor
	store            *Store
	stateStore       *GroundTruthStateStore
	verifier         *GroundedVerifier
	confirmToolCall  func(ctx context.Context, task *Task, step PlanStep, nextTool PlannerToolCall) error
	maxPlannerRounds int
}

type GroundedStepResult struct {
	Output             string
	VerifiedOutput     string
	GroundingStatus    string
	VerificationErrors []string
}

func NewGroundedRuntime(cfg GroundedRuntimeConfig) *GroundedRuntime {
	stateStore := cfg.StateStore
	if stateStore == nil {
		stateStore = NewGroundTruthStateStore()
	}
	executor := cfg.GroundedExecutor
	if executor == nil {
		executor = NewGroundedExecutor(cfg.Executor, cfg.Registry, stateStore, cfg.Secret, cfg.AskHandler)
	}
	planner := cfg.Planner
	if planner == nil {
		planner = NewGroundedPlanner(cfg.PlannerLLM)
	}
	verifier := cfg.Verifier
	if verifier == nil {
		verifier = NewGroundedVerifier(executor.VerifyResult)
	}
	maxPlannerRounds := cfg.MaxPlannerRounds
	if maxPlannerRounds <= 0 {
		maxPlannerRounds = 6
	}
	return &GroundedRuntime{
		planner:          planner,
		responderLLM:     cfg.ResponderLLM,
		registry:         cfg.Registry,
		executor:         executor,
		store:            cfg.Store,
		stateStore:       stateStore,
		verifier:         verifier,
		confirmToolCall:  cfg.ConfirmToolCall,
		maxPlannerRounds: maxPlannerRounds,
	}
}

func (r *GroundedRuntime) ExecuteStep(ctx context.Context, task *Task, step PlanStep, plan []PlanStep, maxToolRounds int) (*GroundedStepResult, error) {
	if task == nil {
		return nil, fmt.Errorf("task is required")
	}
	if task.GroundState == nil {
		task.GroundState = NewGroundTruthState()
	}
	if maxToolRounds <= 0 {
		maxToolRounds = r.maxPlannerRounds
	}
	toolCatalog := groundedToolCatalog(r.registry)
	planSummary := buildGroundedPlanSummary(plan)
	var toolCallIDs []string
	var previousViolations []string
	var allAssertions []PlannerAssertion
	decisionStatus := PlannerDecisionUnknown

	for round := 1; round <= maxToolRounds; round++ {
		decision, err := r.planner.Decide(ctx, PlannerInput{
			Goal:               task.Goal,
			PlanSummary:        planSummary,
			Step:               step,
			PlannerRound:       round,
			MaxRounds:          maxToolRounds,
			GroundState:        task.GroundState,
			ToolCatalog:        toolCatalog,
			PriorToolCallIDs:   toolCallIDs,
			PreviousViolations: previousViolations,
		})
		if err != nil {
			return &GroundedStepResult{
				Output:             "unknown",
				VerifiedOutput:     "unknown",
				GroundingStatus:    GroundingStatusRejected,
				VerificationErrors: []string{err.Error()},
			}, err
		}
		decisionStatus = decision.Status
		allAssertions = append(allAssertions, decision.Assertions...)
		if decision.Status != PlannerDecisionContinue {
			break
		}

		if r.confirmToolCall != nil {
			if err := r.confirmToolCall(ctx, task, step, *decision.NextTool); err != nil {
				return &GroundedStepResult{
					Output:             "unknown",
					VerifiedOutput:     "unknown",
					GroundingStatus:    GroundingStatusRejected,
					VerificationErrors: []string{err.Error()},
				}, err
			}
		}

		exec, execErr := r.executor.Execute(ctx, task, step.Index, round, *decision.NextTool)
		if exec == nil {
			return &GroundedStepResult{
				Output:             "unknown",
				VerifiedOutput:     "unknown",
				GroundingStatus:    GroundingStatusRejected,
				VerificationErrors: []string{"executor returned no execution record"},
			}, fmt.Errorf("executor returned no execution record")
		}

		r.stateStore.ApplyExecution(task.GroundState, exec)
		toolCallIDs = append(toolCallIDs, exec.Call.ToolCallID)
		previousViolations = append(previousViolations, r.persistExecution(ctx, task, exec)...)
		if execErr != nil {
			previousViolations = append(previousViolations, fmt.Sprintf("tool %s failed: %v", exec.Call.Tool, execErr))
		}
		if failures := EvaluateAssertions(task.GroundState, decision.Assertions); len(failures) > 0 {
			return &GroundedStepResult{
				Output:             "unknown",
				VerifiedOutput:     "unknown",
				GroundingStatus:    GroundingStatusRejected,
				VerificationErrors: failures,
			}, fmt.Errorf("%s", strings.Join(failures, "; "))
		}
		if r.store != nil {
			_ = r.store.Update(ctx, task)
		}
	}

	if failures := EvaluateAssertions(task.GroundState, allAssertions); len(failures) > 0 {
		return &GroundedStepResult{
			Output:             "unknown",
			VerifiedOutput:     "unknown",
			GroundingStatus:    GroundingStatusRejected,
			VerificationErrors: failures,
		}, fmt.Errorf("%s", strings.Join(failures, "; "))
	}

	responderInput := ResponderInput{
		Goal:               task.Goal,
		Step:               step,
		GroundState:        task.GroundState,
		PriorToolCallIDs:   toolCallIDs,
		PreviousViolations: previousViolations,
	}
	response, err := r.verifier.Respond(ctx, r.responderLLM, responderInput)
	if err != nil {
		return &GroundedStepResult{
			Output:             "unknown",
			VerifiedOutput:     "unknown",
			GroundingStatus:    GroundingStatusFallback,
			VerificationErrors: []string{err.Error()},
		}, nil
	}
	decision := r.verifier.Verify(task.GroundState, response)
	if !decision.Valid {
		retryInput := responderInput
		retryInput.PreviousViolations = append(retryInput.PreviousViolations, decision.Violations...)
		retryResponse, retryErr := r.verifier.Respond(ctx, r.responderLLM, retryInput)
		if retryErr == nil {
			retryDecision := r.verifier.Verify(task.GroundState, retryResponse)
			if retryDecision.Valid {
				return &GroundedStepResult{
					Output:             retryDecision.Output,
					VerifiedOutput:     retryDecision.Output,
					GroundingStatus:    retryDecision.Status,
					VerificationErrors: nil,
				}, nil
			}
			decision = retryDecision
		}
		return &GroundedStepResult{
			Output:             decision.Output,
			VerifiedOutput:     decision.Output,
			GroundingStatus:    GroundingStatusFallback,
			VerificationErrors: append([]string(nil), decision.Violations...),
		}, nil
	}

	status := decision.Status
	if decisionStatus == PlannerDecisionUnknown && status == GroundingStatusGrounded {
		status = GroundingStatusUnknown
	}
	return &GroundedStepResult{
		Output:             decision.Output,
		VerifiedOutput:     decision.Output,
		GroundingStatus:    status,
		VerificationErrors: nil,
	}, nil
}

func (r *GroundedRuntime) VerifyTask(task *Task) (*VerificationResult, string, error) {
	verificationCtx := compileVerificationContext(task)
	result := &VerificationResult{
		Status:         "pass",
		Summary:        "Grounded runtime checks passed.",
		ExecutedChecks: []string{"grounded runtime status review", "ground truth assertion replay"},
	}
	if task == nil {
		result.Status = "fail"
		result.Summary = "Task is missing."
		return result, formatVerificationOutput(verificationCtx, result, nil, verificationCtx.SuccessCriteria), fmt.Errorf("task is missing")
	}
	if task.GroundingStatus == GroundingStatusRejected {
		result.Status = "fail"
		result.Summary = "Grounded runtime rejected at least one unverified claim or failed assertion."
	}
	for _, criterion := range effectiveSuccessCriteria(task.SuccessCriteria) {
		item := CriterionResult{
			Criterion: criterion,
			Status:    "pass",
			Evidence:  groundedEvidenceForTask(task),
		}
		if hasFailedOrSkippedSteps(task.Plan) || task.GroundingStatus == GroundingStatusRejected {
			item.Status = "fail"
			item.Evidence = "Task contains failed/skipped work or rejected grounded output."
		}
		result.CriteriaResults = append(result.CriteriaResults, item)
	}
	ordered, failed, missing := evaluateVerificationResult(result, verificationCtx.SuccessCriteria)
	output := formatVerificationOutput(verificationCtx, result, ordered, missing)
	if !verificationPassed(result, failed, missing) {
		return result, output, fmt.Errorf("grounded verification failed")
	}
	return result, output, nil
}

func (r *GroundedRuntime) persistExecution(ctx context.Context, task *Task, exec *GroundedExecution) []string {
	if r.store == nil || task == nil || exec == nil {
		return nil
	}
	var failures []string
	events := []struct {
		eventType string
		payload   any
	}{
		{eventType: "tool_call", payload: exec.Call},
		{eventType: "tool_result", payload: exec.Result},
	}
	for _, update := range exec.Updates {
		events = append(events, struct {
			eventType string
			payload   any
		}{eventType: "state_update", payload: update})
	}
	for _, item := range events {
		payloadJSON, err := json.Marshal(item.payload)
		if err != nil {
			failures = append(failures, err.Error())
			continue
		}
		if err := r.store.AppendRuntimeEvent(ctx, RuntimeEvent{
			ID:           fmt.Sprintf("%s/%s/%d", task.ID, item.eventType, time.Now().UTC().UnixNano()),
			TaskID:       task.ID,
			StepIndex:    exec.Call.StepIndex,
			PlannerRound: exec.Call.PlannerRound,
			EventType:    item.eventType,
			PayloadJSON:  string(payloadJSON),
			CreatedAt:    time.Now().UTC(),
		}); err != nil {
			failures = append(failures, err.Error())
		}
	}
	return failures
}

func buildGroundedPlanSummary(plan []PlanStep) string {
	var sb strings.Builder
	for _, step := range plan {
		sb.WriteString(fmt.Sprintf("- [%s] %s\n", step.Status, strings.TrimSpace(step.Description)))
	}
	return strings.TrimSpace(sb.String())
}

func groundedToolCatalog(registry *tools.Registry) []llm.Tool {
	if registry == nil {
		return nil
	}
	defs := registry.DefinitionsForRoute(tools.ToolRouteKindAgent)
	out := make([]llm.Tool, 0, len(defs)+2)
	for _, def := range defs {
		switch normalizeGroundToolName(def.Name) {
		case "read":
			out = append(out, llm.Tool{
				Name:        "read",
				Description: "Read a local workspace file. Accepts path aliases such as path, file_path, filePath, and filename.",
				Parameters:  groundedFileToolParameters(def.Parameters, false),
			})
		case "write":
			out = append(out, llm.Tool{
				Name:        "write",
				Description: "Write a local workspace file. Accepts path aliases such as path, file_path, filePath, and filename, plus content aliases such as content, text, body, and value.",
				Parameters:  groundedFileToolParameters(def.Parameters, true),
			})
		default:
			out = append(out, llm.Tool{Name: def.Name, Description: def.Description, Parameters: def.Parameters})
		}
	}
	return out
}

func groundedFileToolParameters(parameters map[string]any, isWrite bool) map[string]any {
	cloned := cloneGroundedJSONValue(parameters)
	schema, ok := cloned.(map[string]any)
	if !ok || schema == nil {
		return parameters
	}
	props, _ := schema["properties"].(map[string]any)
	if props == nil {
		props = make(map[string]any)
		schema["properties"] = props
	}
	pathSchema, _ := props["path"].(map[string]any)
	if pathSchema == nil {
		pathSchema = map[string]any{"type": "string", "description": "Workspace file path."}
		props["path"] = pathSchema
	}
	props["file_path"] = cloneGroundedJSONValue(pathSchema)
	props["filePath"] = cloneGroundedJSONValue(pathSchema)
	props["filename"] = cloneGroundedJSONValue(pathSchema)

	delete(schema, "required")
	requirePath := map[string]any{
		"anyOf": []any{
			map[string]any{"required": []any{"path"}},
			map[string]any{"required": []any{"file_path"}},
			map[string]any{"required": []any{"filePath"}},
			map[string]any{"required": []any{"filename"}},
		},
	}
	if !isWrite {
		schema["allOf"] = []any{requirePath}
		return schema
	}

	contentSchema, _ := props["content"].(map[string]any)
	if contentSchema == nil {
		contentSchema = map[string]any{"type": "string", "description": "File content."}
		props["content"] = contentSchema
	}
	props["text"] = cloneGroundedJSONValue(contentSchema)
	props["body"] = cloneGroundedJSONValue(contentSchema)
	props["value"] = cloneGroundedJSONValue(contentSchema)
	schema["allOf"] = []any{
		requirePath,
		map[string]any{
			"anyOf": []any{
				map[string]any{"required": []any{"content"}},
				map[string]any{"required": []any{"text"}},
				map[string]any{"required": []any{"body"}},
				map[string]any{"required": []any{"value"}},
			},
		},
	}
	return schema
}

func cloneGroundedJSONValue(v any) any {
	switch typed := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, value := range typed {
			out[key] = cloneGroundedJSONValue(value)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, value := range typed {
			out[i] = cloneGroundedJSONValue(value)
		}
		return out
	default:
		return typed
	}
}

func combineGroundingStatus(current, next string) string {
	order := map[string]int{
		"":                      0,
		GroundingStatusGrounded: 1,
		GroundingStatusUnknown:  2,
		GroundingStatusFallback: 3,
		GroundingStatusRejected: 4,
	}
	if order[next] > order[current] {
		return next
	}
	return current
}

func mergeVerificationErrors(existing []string, additional []string) []string {
	seen := make(map[string]struct{}, len(existing)+len(additional))
	out := make([]string, 0, len(existing)+len(additional))
	for _, group := range [][]string{existing, additional} {
		for _, item := range group {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			out = append(out, item)
		}
	}
	return out
}

func groundedEvidenceForTask(task *Task) string {
	if task == nil {
		return "No grounded evidence."
	}
	if strings.TrimSpace(task.VerifiedOutput) != "" {
		return truncate(task.VerifiedOutput, 220)
	}
	switch task.GroundingStatus {
	case GroundingStatusRejected:
		return "Grounded runtime rejected the response."
	case GroundingStatusFallback:
		return "Grounded runtime used deterministic fallback output."
	case GroundingStatusUnknown:
		return "Grounded runtime reported unknown where evidence was missing."
	default:
		return "Grounded runtime produced verified step outputs."
	}
}

func hasFailedOrSkippedSteps(plan []PlanStep) bool {
	for _, step := range plan {
		if isRuntimeMetaStep(step.Description) {
			continue
		}
		if step.Status == StepStatusFailed || step.Status == StepStatusSkipped {
			return true
		}
	}
	return false
}

func isRuntimeMetaStep(description string) bool {
	description = strings.ToLower(strings.TrimSpace(description))
	switch {
	case strings.HasPrefix(description, "verify "),
		strings.HasPrefix(description, "recovery:"),
		strings.HasPrefix(description, "reflect "):
		return true
	default:
		return false
	}
}

func buildGroundedTaskReport(task *Task) string {
	if task == nil {
		return "unknown"
	}
	var sb strings.Builder
	sb.WriteString("Summary: ")
	switch task.Status {
	case TaskStatusFailed:
		sb.WriteString("The task failed before all grounded checks passed.")
	case TaskStatusCompleted:
		sb.WriteString("The task completed with grounded, evidence-bound outputs.")
	default:
		sb.WriteString("The task finished with partial grounded output.")
	}
	sb.WriteString("\n\nVerified:\n")
	wrote := false
	for _, step := range task.Plan {
		if step.Status != StepStatusCompleted {
			continue
		}
		output := strings.TrimSpace(step.Output)
		if output == "" {
			continue
		}
		sb.WriteString(fmt.Sprintf("%d. %s\n", step.Index+1, truncate(output, 500)))
		wrote = true
	}
	if !wrote {
		sb.WriteString("1. unknown\n")
	}
	if len(task.VerificationErrors) > 0 {
		sb.WriteString("\nVerification errors:\n")
		for _, item := range task.VerificationErrors {
			sb.WriteString("- ")
			sb.WriteString(item)
			sb.WriteByte('\n')
		}
	}
	return strings.TrimSpace(sb.String())
}
