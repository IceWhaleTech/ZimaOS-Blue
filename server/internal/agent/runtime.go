package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/remindertime"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/routingcue"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

var groundedURLPattern = regexp.MustCompile(`https?://[^\s<>"']+`)
var groundedReminderClockPattern = regexp.MustCompile(`(?i)(\d{1,2})(?::(\d{2}))?\s*(am|pm|a\.m\.|p\.m\.|点|點|时|時|시|uhr|ora|orara|ore|മണിക്ക്|h)?`)
var groundedReminderStandaloneHourPattern = regexp.MustCompile(`\b([01]?\d|2[0-3])\b`)
var groundedCanonicalCLINow = func() time.Time { return timeutil.NowTime().In(time.Local) }

var groundedReminderTomorrowCues = []string{
	"tomorrow",
	"mañana",
	"manana",
	"demain",
	"amárach",
	"amárach",
	"sutra",
	"holnap",
	"domani",
	"明日",
	"明早",
	"明天",
	"내일",
	"നാളെ",
	"i morgen",
	"morgen",
	"jutro",
	"amanhã",
	"amanha",
	"maine",
	"завтра",
	"zajtra",
	"i morgon",
}

var groundedReminderMorningCues = []string{
	"morning",
	"mañana",
	"matin",
	"amárach",
	"jutro",
	"morgen",
	"domani",
	"明早",
	"明日朝",
	"朝",
	"아침",
	"രാവിലെ",
	"上午",
}

var groundedReminderPMCues = []string{
	"pm",
	"p.m.",
	"afternoon",
	"evening",
	"tonight",
	"下午",
	"晚上",
	"今晚",
	"夕方",
	"夜",
	"저녁",
}

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

	trimmed := trimStructuredContent(content)

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
	case "read", "memory", "web_search", "web_query", "analyze", "ui_reviewer", "ask":
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
	return containsDangerousConfirmationCue(contextText)
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
		if containsDangerousConfirmationCue(step) {
			return true
		}
	}
	return false
}

func containsDangerousConfirmationCue(text string) bool {
	var token strings.Builder
	flush := func() bool {
		if token.Len() == 0 {
			return false
		}
		word := token.String()
		token.Reset()
		switch word {
		case "production", "delete", "drop", "publish":
			return true
		default:
			return false
		}
	}
	for _, r := range strings.ToLower(text) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			token.WriteRune(r)
			continue
		}
		if flush() {
			return true
		}
	}
	return flush()
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
	progressState := ProgressSignatureState{}
	var forcedNextTool *PlannerToolCall

	for round := 1; round <= maxToolRounds; round++ {
		var decision *PlannerDecision
		if forcedNextTool != nil {
			decision = &PlannerDecision{
				Status:   PlannerDecisionContinue,
				Reason:   "web_query requested browser escalation",
				NextTool: forcedNextTool,
			}
			forcedNextTool = nil
		} else {
			planned, err := r.planner.Decide(ctx, PlannerInput{
				Model:              preferredTaskModel(task),
				Goal:               task.Goal,
				PlanSummary:        planSummary,
				Step:               step,
				PlannerRound:       round,
				MaxRounds:          maxToolRounds,
				GroundState:        task.GroundState,
				ToolCatalog:        toolCatalog,
				PriorToolCallIDs:   toolCallIDs,
				PreviousViolations: previousViolations,
				RoutingContract:    buildTaskRoutingContractContext(plannerTaskMetadata(task)),
				CoordinationCtx:    buildTaskCoordinationPromptContext(task),
			})
			if err != nil {
				if fallback, note := groundedPlannerErrorFallback(task, toolCatalog, err); fallback != nil && ctx.Err() == nil {
					decision = fallback
					if strings.TrimSpace(note) != "" {
						previousViolations = append(previousViolations, note)
					}
				} else {
					return &GroundedStepResult{
						Output:             "unknown",
						VerifiedOutput:     "unknown",
						GroundingStatus:    GroundingStatusRejected,
						VerificationErrors: []string{err.Error()},
					}, err
				}
			} else {
				decision = planned
			}
		}
		decisionStatus = decision.Status
		allAssertions = append(allAssertions, decision.Assertions...)
		if overridden, note := groundedCanonicalCLIOverride(task, toolCatalog, decision); overridden != nil {
			decision = overridden
			decisionStatus = decision.Status
			if strings.TrimSpace(note) != "" {
				previousViolations = append(previousViolations, note)
			}
		}
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
		if groundedExecutionSatisfiesExecutionContract(task, exec) || groundedShouldCompleteExecutionContract(task) {
			previousViolations = append(previousViolations, "execution routing contract evidence is already satisfied; stop after grounded web evidence instead of additional route hops")
			decisionStatus = PlannerDecisionComplete
			if r.store != nil {
				_ = r.store.Update(ctx, task)
			}
			break
		}
		if groundedShouldSuppressBrowserEscalation(task, exec) {
			previousViolations = append(previousViolations, fmt.Sprintf("web_query requested browser escalation for %s after canonical web_search already succeeded; summarize existing evidence or report the blocker instead of forcing browser", firstNonEmptyURL(exec)))
		} else if escalation := groundedBrowserEscalationTool(exec); escalation != nil {
			forcedNextTool = escalation
			previousViolations = append(previousViolations, fmt.Sprintf("web_query requested browser escalation for %s; switch to browser immediately", firstNonEmptyURL(exec)))
		}
		if detection := progressState.ObserveDetailed(groundedPlannerToolSignature(*decision.NextTool), decision.Reason, []string{groundedExecutionProgressSummary(exec)}); detection.Abort {
			previousViolations = append(previousViolations, fmt.Sprintf("tool loop detected (%s): %s", detection.Reason, detection.Signature))
			r.persistLoopDetection(ctx, task, step.Index, round, detection)
			if detection.Reason == tools.ToolLoopReasonErrorRepeat {
				decisionStatus = PlannerDecisionBlocked
			} else {
				decisionStatus = PlannerDecisionComplete
			}
			if r.store != nil {
				_ = r.store.Update(ctx, task)
			}
			break
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
		Model:              preferredTaskModel(task),
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

func groundedPlannerToolSignature(call PlannerToolCall) string {
	argsJSON, err := json.Marshal(call.Args)
	if err != nil {
		return strings.TrimSpace(call.Tool)
	}
	return agentToolLoopCallSignature(llm.ToolCall{
		Name:      strings.TrimSpace(call.Tool),
		Arguments: string(argsJSON),
	})
}

func groundedCanonicalCLIOverride(task *Task, toolCatalog []llm.Tool, decision *PlannerDecision) (*PlannerDecision, string) {
	expectedCLIAction, enforce := groundedExpectedCLIAction(task)
	if !enforce || expectedCLIAction == "" {
		return nil, ""
	}
	command := groundedCanonicalCLICommand(task, expectedCLIAction, decision)
	if command == "" {
		return nil, ""
	}
	if groundedCanonicalCLIAlreadyAttempted(task, command) {
		return nil, ""
	}
	if groundedPlannerDecisionUsesCanonicalCLI(decision, command) {
		return nil, ""
	}
	toolName := groundedCanonicalCLIToolName(toolCatalog)
	if toolName == "" {
		return nil, ""
	}
	return &PlannerDecision{
		Status: PlannerDecisionContinue,
		Reason: "execution routing contract requires the canonical CLI action first",
		NextTool: &PlannerToolCall{
			Tool: toolName,
			Args: map[string]any{"command": command},
		},
		Assertions: decisionAssertions(decision),
	}, fmt.Sprintf("execution routing contract forced canonical CLI action %q before alternate route %q", command, groundedPlannerDecisionToolName(decision))
}

func groundedPlannerErrorFallback(task *Task, toolCatalog []llm.Tool, plannerErr error) (*PlannerDecision, string) {
	expectedCLIAction, enforce := groundedExpectedCLIAction(task)
	if !enforce || expectedCLIAction == "" {
		return nil, ""
	}
	command := groundedCanonicalCLICommand(task, expectedCLIAction, nil)
	if command == "" || groundedCanonicalCLIAlreadyAttempted(task, command) {
		return nil, ""
	}
	toolName := groundedCanonicalCLIToolName(toolCatalog)
	if toolName == "" {
		return nil, ""
	}
	return &PlannerDecision{
		Status: PlannerDecisionContinue,
		Reason: "execution routing contract recovered from planner failure with canonical CLI action",
		NextTool: &PlannerToolCall{
			Tool: toolName,
			Args: map[string]any{"command": command},
		},
	}, fmt.Sprintf("planner error %q triggered canonical CLI fallback %q", strings.TrimSpace(plannerErr.Error()), command)
}

func groundedExpectedCLIAction(task *Task) (string, bool) {
	meta := plannerTaskMetadata(task)
	if len(meta) == 0 {
		return "", false
	}
	contract := metadataMapValue(meta, "routing_contract")
	if len(contract) == 0 {
		contract = meta
	}
	expectedCLIAction := firstNonEmptyString(metadataStringValue(contract, "expected_cli_action"), metadataStringValue(meta, "expected_cli_action"))
	if expectedCLIAction == "" {
		return "", false
	}
	if enforce, ok := metadataBoolFromMaps([]map[string]interface{}{contract, meta}, "enforce_cli_route"); ok {
		return expectedCLIAction, enforce
	}
	gateType := firstNonEmptyString(metadataStringValue(contract, "gate_type"), metadataStringValue(meta, "gate_type"))
	return expectedCLIAction, gateType == "execution_equivalence"
}

func groundedCanonicalCLIToolName(toolCatalog []llm.Tool) string {
	var fallback string
	for _, tool := range toolCatalog {
		name := strings.TrimSpace(tool.Name)
		if name == "" || normalizeGroundToolName(name) != "bash" {
			continue
		}
		if strings.EqualFold(name, "exec") {
			return name
		}
		if fallback == "" {
			fallback = name
		}
	}
	return fallback
}

func groundedPlannerDecisionUsesCanonicalCLI(decision *PlannerDecision, expectedCLIAction string) bool {
	if decision == nil || decision.NextTool == nil {
		return false
	}
	if normalizeGroundToolName(decision.NextTool.Tool) != "bash" {
		return false
	}
	actual := firstNonEmptyString(
		strings.TrimSpace(asString(decision.NextTool.Args["command"])),
		strings.TrimSpace(asString(decision.NextTool.Args["cmd"])),
	)
	return groundedCLICommandMatches(actual, expectedCLIAction)
}

func groundedCanonicalCLIAlreadyAttempted(task *Task, expectedCLIAction string) bool {
	if task == nil || task.GroundState == nil || expectedCLIAction == "" {
		return false
	}
	for _, fact := range task.GroundState.Commands {
		if normalizeGroundToolName(fact.Tool) != "bash" {
			continue
		}
		if groundedCLICommandMatches(fact.Command, expectedCLIAction) {
			return true
		}
	}
	for _, call := range task.GroundState.Calls {
		if normalizeGroundToolName(call.Tool) != "bash" {
			continue
		}
		command := firstNonEmptyString(
			strings.TrimSpace(asString(call.Args["command"])),
			strings.TrimSpace(asString(call.Args["cmd"])),
		)
		if groundedCLICommandMatches(command, expectedCLIAction) {
			return true
		}
	}
	return false
}

func groundedCLICommandMatches(actual, expected string) bool {
	actual = normalizeGroundedCLICommand(actual)
	expected = normalizeGroundedCLICommand(expected)
	if actual == "" || expected == "" {
		return false
	}
	return actual == expected || strings.HasPrefix(actual, expected+" ")
}

func normalizeGroundedCLICommand(command string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(command)), " ")
}

func groundedCanonicalCLICommand(task *Task, expectedCLIAction string, decision *PlannerDecision) string {
	command := normalizeGroundedCLICommand(expectedCLIAction)
	if command == "" || groundedCLICommandHasArgs(command) {
		return command
	}
	skillToken := groundedCLICommandSkillToken(command)
	baseSkill, skillAction := groundedCLICommandSkillParts(command)
	if skillToken == "" || baseSkill == "" {
		return command
	}
	query := groundedCanonicalCLIQuery(task, baseSkill, decision)
	switch strings.ToLower(baseSkill) {
	case "web_search", "web_query":
		if query != "" {
			return command + " query=" + strconv.Quote(query)
		}
	case "deep_research", "research_run":
		if query != "" {
			return command + " query=" + strconv.Quote(query)
		}
	case "analyze":
		topic, url := groundedCanonicalAnalyzeCLIArgs(task, decision)
		if topic != "" {
			command += " topic=" + strconv.Quote(topic)
		}
		if url != "" {
			command += " url=" + strconv.Quote(url)
		}
		return command
	case "reminder":
		args := groundedCanonicalReminderCLIArgs(task, decision, skillAction)
		if args.Message != "" {
			command += " message=" + strconv.Quote(args.Message)
		}
		if args.Time != "" {
			command += " time=" + strconv.Quote(args.Time)
		}
		if args.Every != "" {
			command += " every=" + strconv.Quote(args.Every)
		}
		if args.Until != "" {
			command += " until=" + strconv.Quote(args.Until)
		}
		if args.Recurring != "" {
			command += " recurring=" + strconv.Quote(args.Recurring)
		}
		if args.SessionID != "" {
			command += " session_id=" + strconv.Quote(args.SessionID)
		}
		return command
	}
	return command
}

type groundedReminderCLIArgs struct {
	Message   string
	Time      string
	Every     string
	Until     string
	Recurring string
	SessionID string
}

func groundedCanonicalAnalyzeCLIArgs(task *Task, decision *PlannerDecision) (string, string) {
	topic := groundedCanonicalCLIQuery(task, "analyze", decision)
	url := groundedCanonicalAnalyzeURL(task, decision)
	if url != "" {
		if stripped := groundedStripURLs(topic); stripped != "" {
			topic = stripped
		}
	}
	if topic == "" {
		topic = url
	}
	return topic, url
}

func groundedCanonicalCLIQuery(task *Task, skillToken string, decision *PlannerDecision) string {
	if query := groundedPlannerSuggestedCLIQuery(skillToken, decision); query != "" {
		return query
	}
	return groundedTaskQuery(task)
}

func groundedPlannerSuggestedCLIQuery(skillToken string, decision *PlannerDecision) string {
	if decision == nil || decision.NextTool == nil {
		return ""
	}
	args := decision.NextTool.Args
	switch strings.ToLower(strings.TrimSpace(skillToken)) {
	case "web_search", "web_query":
		if !isGroundedWebToolFamily(decision.NextTool.Tool) {
			return ""
		}
		return groundedPlannerArgString(args, "query", "input", "url", "href", "target")
	case "analyze":
		if normalizeGroundToolName(decision.NextTool.Tool) != "analyze" {
			return ""
		}
		return groundedPlannerArgString(args, "topic", "query", "input", "path", "target")
	case "reminder":
		if normalizeGroundToolName(decision.NextTool.Tool) != "reminder" {
			return ""
		}
		return groundedPlannerArgString(args, "message", "content", "text", "query", "input")
	default:
		return ""
	}
}

func groundedCanonicalReminderCLIArgs(task *Task, decision *PlannerDecision, skillAction string) groundedReminderCLIArgs {
	args := groundedPlannerSuggestedReminderArgs(decision)
	query := groundedTaskQuery(task)

	if args.Message == "" {
		args.Message = groundedCanonicalReminderMessage(query)
	}
	if args.Time == "" && args.Every == "" {
		args.Time = groundedCanonicalReminderTime(query)
	}
	if args.Message == "" {
		args.Message = strings.TrimSpace(query)
	}
	args.Time = groundedNormalizeReminderTimeArg(firstNonEmptyString(args.Time, query))
	args.Every = groundedNormalizeReminderIntervalArg(args.Every)
	args.Until = groundedNormalizeReminderTimeArg(args.Until)

	if args.Time == "" && args.Every == "" && strings.EqualFold(strings.TrimSpace(skillAction), "add") {
		args.Time = groundedNormalizeReminderTimeArg(query)
	}
	if args.SessionID == "" {
		args.SessionID = groundedTaskSessionID(task)
	}
	return args
}

func groundedPlannerSuggestedReminderArgs(decision *PlannerDecision) groundedReminderCLIArgs {
	if decision == nil || decision.NextTool == nil || normalizeGroundToolName(decision.NextTool.Tool) != "reminder" {
		return groundedReminderCLIArgs{}
	}
	args := decision.NextTool.Args
	return groundedReminderCLIArgs{
		Message:   groundedPlannerArgString(args, "message", "content", "text", "input", "query"),
		Time:      groundedPlannerArgString(args, "time", "when", "at", "fire_at", "fireAt", "delay", "in"),
		Every:     groundedPlannerArgString(args, "every", "interval"),
		Until:     groundedPlannerArgString(args, "until", "until_at", "untilAt"),
		Recurring: groundedPlannerArgString(args, "recurring", "repeat", "recurrence"),
		SessionID: groundedPlannerArgString(args, "session_id", "sessionId", "session", "conversation_id", "conversationId"),
	}
}

func groundedCanonicalReminderMessage(query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return ""
	}

	candidate := query
	if _, end, ok := groundedReminderTimeSpan(query); ok && end < len(query) {
		if trailing := strings.TrimSpace(query[end:]); trailing != "" {
			candidate = trailing
		}
	}

	candidate = groundedStripURLs(candidate)
	candidate = groundedStripReminderCueTerms(candidate)
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return query
	}
	return candidate
}

func groundedStripReminderCueTerms(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	cue := routingcue.SkillTerms("reminder")
	terms := make([]string, 0, len(cue.Actions)+len(cue.Objects)+len(groundedReminderTomorrowCues))
	terms = append(terms, cue.Actions...)
	terms = append(terms, cue.Objects...)
	terms = append(terms, groundedReminderTomorrowCues...)
	for _, term := range terms {
		trimmed := strings.TrimSpace(term)
		if trimmed == "" {
			continue
		}
		text = strings.ReplaceAll(text, trimmed, " ")
	}
	replacer := strings.NewReplacer(
		"帮我", " ",
		"幫我", " ",
		"提醒我", " ",
		"请", " ",
		"請", " ",
		"一个", " ",
		"一個", " ",
		"的", " ",
		"我", " ",
		"來", " ",
		"来", " ",
		"for", " ",
		"to", " ",
		"about", " ",
		"at", " ",
		"para", " ",
		"pour", " ",
	)
	text = replacer.Replace(text)
	return strings.Join(strings.Fields(text), " ")
}

func groundedCanonicalReminderTime(query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return ""
	}
	return groundedNormalizeReminderTimeArg(query)
}

func groundedNormalizeReminderIntervalArg(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if _, err := remindertime.ParseDuration(value); err == nil {
		return value
	}
	return ""
}

func groundedNormalizeReminderTimeArg(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	referenceNow := groundedCanonicalCLINow().In(time.Local)
	if parsed, err := remindertime.ParseAt(value, referenceNow); err == nil {
		return parsed.In(referenceNow.Location()).Format("2006-01-02 15:04")
	}
	if absolute := groundedCanonicalReminderAbsoluteTime(value); absolute != "" {
		return absolute
	}
	return ""
}

func groundedCanonicalReminderAbsoluteTime(query string) string {
	if !groundedContainsAnyFold(query, groundedReminderTomorrowCues...) {
		return ""
	}
	hour, minute, ok := groundedExtractReminderClock(query)
	if !ok {
		return ""
	}
	lower := strings.ToLower(query)
	if groundedContainsAnyFold(lower, groundedReminderPMCues...) && hour < 12 {
		hour += 12
	}
	if groundedContainsAnyFold(lower, groundedReminderMorningCues...) && hour == 12 {
		hour = 0
	}
	now := groundedCanonicalCLINow()
	target := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location()).Add(24 * time.Hour)
	return target.Format("2006-01-02 15:04")
}

func groundedReminderTimeSpan(query string) (int, int, bool) {
	if !groundedContainsAnyFold(query, groundedReminderTomorrowCues...) {
		return 0, 0, false
	}
	if loc := groundedReminderClockPattern.FindStringIndex(query); len(loc) == 2 {
		return loc[0], loc[1], true
	}
	return 0, 0, false
}

func groundedExtractReminderClock(query string) (int, int, bool) {
	matches := groundedReminderClockPattern.FindAllStringSubmatch(query, -1)
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}
		hour, err := strconv.Atoi(strings.TrimSpace(match[1]))
		if err != nil || hour < 0 || hour > 23 {
			continue
		}
		minute := 0
		if strings.TrimSpace(match[2]) != "" {
			minute, err = strconv.Atoi(strings.TrimSpace(match[2]))
			if err != nil || minute < 0 || minute > 59 {
				continue
			}
		}
		suffix := strings.ToLower(strings.TrimSpace(match[3]))
		if strings.HasPrefix(suffix, "p") && hour < 12 {
			hour += 12
		}
		if strings.HasPrefix(suffix, "a") && hour == 12 {
			hour = 0
		}
		return hour, minute, true
	}
	if match := groundedReminderStandaloneHourPattern.FindStringSubmatch(query); len(match) >= 2 {
		hour, err := strconv.Atoi(strings.TrimSpace(match[1]))
		if err == nil && hour >= 0 && hour <= 23 {
			return hour, 0, true
		}
	}
	return 0, 0, false
}

func groundedContainsAnyFold(text string, needles ...string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	for _, needle := range needles {
		if trimmed := strings.ToLower(strings.TrimSpace(needle)); trimmed != "" && strings.Contains(lower, trimmed) {
			return true
		}
	}
	return false
}

func groundedCanonicalAnalyzeURL(task *Task, decision *PlannerDecision) string {
	if decision != nil && decision.NextTool != nil && normalizeGroundToolName(decision.NextTool.Tool) == "analyze" {
		if url := groundedPlannerSuggestedURL(decision.NextTool.Args, "url", "href", "target", "source", "urls", "input", "query", "topic"); url != "" {
			return url
		}
	}
	return groundedExtractFirstURL(groundedTaskQuery(task))
}

func groundedPlannerSuggestedURL(args map[string]any, keys ...string) string {
	if len(args) == 0 {
		return ""
	}
	for _, key := range keys {
		if url := groundedExtractFirstURLFromAny(args[key]); url != "" {
			return url
		}
	}
	return ""
}

func groundedPlannerArgString(args map[string]any, keys ...string) string {
	if len(args) == 0 {
		return ""
	}
	for _, key := range keys {
		if value := strings.TrimSpace(asString(args[key])); value != "" {
			return value
		}
	}
	return ""
}

func groundedTaskQuery(task *Task) string {
	meta := plannerTaskMetadata(task)
	groupInput := metadataMapValue(meta, "group_input")
	return firstNonEmptyString(
		metadataStringValue(groupInput, "query"),
		metadataStringValue(groupInput, "input"),
		metadataStringValue(groupInput, "goal"),
		task.Goal,
	)
}

func groundedTaskSessionID(task *Task) string {
	meta := plannerTaskMetadata(task)
	groupInput := metadataMapValue(meta, "group_input")
	return firstNonEmptyString(
		metadataStringValue(groupInput, "session_id"),
		metadataStringValue(groupInput, "sessionId"),
		metadataStringValue(meta, "session_id"),
		metadataStringValue(meta, "sessionId"),
	)
}

func groundedExtractFirstURLFromAny(value any) string {
	switch typed := value.(type) {
	case []string:
		for _, item := range typed {
			if url := groundedExtractFirstURL(item); url != "" {
				return url
			}
		}
	case []any:
		for _, item := range typed {
			if url := groundedExtractFirstURLFromAny(item); url != "" {
				return url
			}
		}
	}
	return groundedExtractFirstURL(asString(value))
}

func groundedExtractFirstURL(text string) string {
	matches := groundedURLPattern.FindAllString(text, -1)
	for _, match := range matches {
		if cleaned := groundedNormalizeURLToken(match); cleaned != "" {
			return cleaned
		}
	}
	return ""
}

func groundedStripURLs(text string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	cleaned := groundedURLPattern.ReplaceAllString(text, " ")
	cleaned = strings.Join(strings.Fields(cleaned), " ")
	cleaned = strings.Trim(cleaned, `"'“”‘’()[]{}<>.,!?;:，。！？；：、）】》〉」』］〕`)
	return strings.TrimSpace(cleaned)
}

func groundedNormalizeURLToken(raw string) string {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.TrimRight(cleaned, `"'“”‘’.,!?;:)]}>,，。！？；：、）】》〉」』］〕`)
	if cleaned == "" {
		return ""
	}
	return cleaned
}

func groundedCLICommandSkillToken(command string) string {
	fields := strings.Fields(strings.TrimSpace(command))
	if len(fields) >= 2 && fields[0] == "blue" {
		return fields[1]
	}
	return ""
}

func groundedCLICommandSkillParts(command string) (string, string) {
	fields := strings.Fields(strings.TrimSpace(command))
	if len(fields) < 2 || fields[0] != "blue" {
		return "", ""
	}

	token := strings.TrimSpace(fields[1])
	if token == "" {
		return "", ""
	}

	// Dotted alias: blue reminder.add ...
	if dot := strings.Index(token, "."); dot > 0 && dot < len(token)-1 {
		return strings.TrimSpace(token[:dot]), strings.TrimSpace(token[dot+1:])
	}

	// Positional action: blue reminder add ...
	if len(fields) >= 3 && groundedSkillSupportsPositionalAction(token) {
		if action := groundedNormalizePositionalAction(token, fields[2]); action != "" {
			return token, action
		}
	}

	return token, ""
}

func groundedCLICommandHasArgs(command string) bool {
	fields := strings.Fields(strings.TrimSpace(command))
	if len(fields) < 2 || fields[0] != "blue" {
		return false
	}

	skillToken := strings.TrimSpace(fields[1])
	if skillToken == "" {
		return false
	}

	// For action-oriented skills, treat `blue reminder add` as the canonical
	// action form (not "having args") so callers can still have us ground
	// message/time args onto it.
	if len(fields) >= 3 {
		baseSkill, skillAction := groundedCLICommandSkillParts(command)
		if groundedSkillSupportsPositionalAction(baseSkill) && skillAction != "" {
			// Any tokens after the action count as args.
			return len(fields) > 3
		}
	}

	// Default: any tokens after `blue <skill>` count as args.
	return len(fields) > 2
}

func groundedSkillSupportsPositionalAction(skill string) bool {
	switch strings.ToLower(strings.TrimSpace(skill)) {
	case "reminder":
		return true
	default:
		return false
	}
}

func groundedNormalizePositionalAction(skill, raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" || strings.HasPrefix(raw, "-") || strings.Contains(raw, "=") {
		return ""
	}

	switch strings.ToLower(strings.TrimSpace(skill)) {
	case "reminder":
		switch raw {
		case "list", "get", "status":
			return "list"
		case "add", "create", "send", "notify":
			return "add"
		case "delete", "remove", "rm":
			return "delete"
		case "clear":
			return "clear"
		default:
			return ""
		}
	default:
		return ""
	}
}

func groundedPlannerDecisionToolName(decision *PlannerDecision) string {
	if decision == nil || decision.NextTool == nil {
		return "none"
	}
	return strings.TrimSpace(decision.NextTool.Tool)
}

func decisionAssertions(decision *PlannerDecision) []PlannerAssertion {
	if decision == nil || len(decision.Assertions) == 0 {
		return nil
	}
	out := make([]PlannerAssertion, len(decision.Assertions))
	copy(out, decision.Assertions)
	return out
}

func groundedExecutionProgressSummary(exec *GroundedExecution) string {
	if exec == nil {
		return "empty"
	}
	payload := exec.Result.Result
	if !exec.Result.OK && strings.TrimSpace(exec.Result.Stderr) != "" {
		payload = map[string]any{"error": exec.Result.Stderr}
	}
	switch typed := payload.(type) {
	case string:
		return tools.NormalizeToolProgressSummary(typed)
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return tools.NormalizeToolProgressSummary(fmt.Sprintf("%v", typed))
		}
		return tools.NormalizeToolProgressSummary(string(encoded))
	}
}

func (r *GroundedRuntime) persistLoopDetection(ctx context.Context, task *Task, stepIndex, plannerRound int, detection tools.ToolLoopDetection) {
	if r == nil || r.store == nil || task == nil {
		return
	}
	payloadJSON, err := json.Marshal(detection)
	if err != nil {
		return
	}
	_ = r.store.AppendRuntimeEvent(ctx, RuntimeEvent{
		ID:           fmt.Sprintf("%s/loop_detection/%d", task.ID, time.Now().UTC().UnixNano()),
		TaskID:       task.ID,
		StepIndex:    stepIndex,
		PlannerRound: plannerRound,
		EventType:    "loop_detection",
		PayloadJSON:  string(payloadJSON),
		CreatedAt:    time.Now().UTC(),
	})
}

func groundedBrowserEscalationTool(exec *GroundedExecution) *PlannerToolCall {
	if exec == nil || normalizeGroundToolName(exec.Call.Tool) != "web_query" {
		return nil
	}
	target := firstNonEmptyURL(exec)
	if target == "" {
		return nil
	}
	if !groundedNeedsBrowser(exec.Result.Result) {
		return nil
	}
	return &PlannerToolCall{
		Tool: "browser",
		Args: map[string]any{
			"action": "navigate",
			"url":    target,
		},
	}
}

func groundedShouldSuppressBrowserEscalation(task *Task, exec *GroundedExecution) bool {
	if task == nil || task.GroundState == nil || exec == nil {
		return false
	}
	if normalizeGroundToolName(exec.Call.Tool) != "web_query" || !groundedNeedsBrowser(exec.Result.Result) {
		return false
	}
	expectedCLIAction, enforce := groundedExpectedCLIAction(task)
	if !enforce {
		return false
	}
	if normalizeGroundToolName(groundedCLICommandSkillToken(expectedCLIAction)) != "web_search" {
		return false
	}
	return groundedCommandHistorySucceeded(task.GroundState, expectedCLIAction)
}

func groundedShouldCompleteExecutionContract(task *Task) bool {
	if task == nil || task.GroundState == nil {
		return false
	}
	expectedCLIAction, enforce := groundedExpectedCLIAction(task)
	if !enforce {
		return false
	}
	skillToken := normalizeGroundToolName(groundedCLICommandSkillToken(expectedCLIAction))
	if skillToken == "" {
		return false
	}
	if !groundedCommandHistorySucceeded(task.GroundState, expectedCLIAction) {
		return false
	}
	if skillToken == "analyze" {
		return groundedHasSuccessfulAnalyzeEvidence(task.GroundState)
	}
	if skillToken == "reminder" {
		criteria := groundedExecutionObservationCriteria(task)
		if len(criteria) == 0 {
			return groundedHasSuccessfulReminderEvidence(task.GroundState, groundedTaskSessionID(task))
		}
		for _, criterion := range criteria {
			switch normalizeGroundedExecutionCriterion(criterion) {
			case "session_context_propagated":
				if !groundedHasSuccessfulReminderEvidence(task.GroundState, groundedTaskSessionID(task)) {
					return false
				}
			default:
				return false
			}
		}
		return true
	}
	if !isGroundedWebToolFamily(skillToken) {
		return false
	}
	criteria := groundedExecutionObservationCriteria(task)
	if len(criteria) == 0 {
		return false
	}
	for _, criterion := range criteria {
		switch normalizeGroundedExecutionCriterion(criterion) {
		case "evidence_tool_used":
			if !groundedHasSuccessfulWebEvidence(task.GroundState) {
				return false
			}
		case "planner_memory_skipped":
			if !shouldSkipPlannerMemory(groundedTaskQuery(task)) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func groundedExecutionSatisfiesExecutionContract(task *Task, exec *GroundedExecution) bool {
	if task == nil || exec == nil {
		return false
	}
	expectedCLIAction, enforce := groundedExpectedCLIAction(task)
	if !enforce {
		return false
	}
	command := firstNonEmptyString(
		strings.TrimSpace(asString(exec.Call.Args["command"])),
		strings.TrimSpace(asString(exec.Call.Args["cmd"])),
	)
	if !groundedCLICommandMatches(command, expectedCLIAction) {
		return false
	}
	if !exec.Result.OK || exec.Result.ExitCode != 0 {
		return false
	}
	skillToken := normalizeGroundToolName(groundedCLICommandSkillToken(expectedCLIAction))
	if skillToken == "" {
		return false
	}
	if skillToken == "analyze" {
		return groundedResultCarriesAnalyzeEvidence(exec.Call, exec.Result) &&
			groundedResultHasAnalyzeEvidence(exec.Result.Result)
	}
	if skillToken == "reminder" {
		criteria := groundedExecutionObservationCriteria(task)
		if len(criteria) == 0 {
			return groundedResultCarriesReminderEvidence(exec.Call, exec.Result) &&
				groundedReminderResultReady(exec.Result.Result) &&
				groundedReminderSessionPropagated(exec.Call, exec.Result, groundedTaskSessionID(task))
		}
		for _, criterion := range criteria {
			switch normalizeGroundedExecutionCriterion(criterion) {
			case "session_context_propagated":
				if !(groundedResultCarriesReminderEvidence(exec.Call, exec.Result) &&
					groundedReminderResultReady(exec.Result.Result) &&
					groundedReminderSessionPropagated(exec.Call, exec.Result, groundedTaskSessionID(task))) {
					return false
				}
			default:
				return false
			}
		}
		return true
	}
	if !isGroundedWebToolFamily(skillToken) {
		return false
	}
	criteria := groundedExecutionObservationCriteria(task)
	if len(criteria) == 0 {
		return false
	}
	for _, criterion := range criteria {
		switch normalizeGroundedExecutionCriterion(criterion) {
		case "evidence_tool_used":
			if !(groundedResultCarriesWebEvidence(exec.Call, exec.Result) &&
				groundedResultHasUsefulEvidence(exec.Result)) {
				return false
			}
		case "planner_memory_skipped":
			if !shouldSkipPlannerMemory(groundedTaskQuery(task)) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func groundedExecutionObservationCriteria(task *Task) []string {
	if task == nil {
		return nil
	}
	var values []string
	for _, source := range taskMetadataSources(task) {
		values = append(values, metadataStringSlice(source, "required_observations")...)
		values = append(values, metadataStringSlice(source, "task_success_criteria")...)
	}
	return dedupeStrings(values)
}

func normalizeGroundedExecutionCriterion(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func groundedHasSuccessfulWebEvidence(state *GroundTruthState) bool {
	if state == nil {
		return false
	}
	for toolCallID, result := range state.Results {
		if !result.OK {
			continue
		}
		call := state.Calls[toolCallID]
		if !groundedResultCarriesWebEvidence(call, result) {
			continue
		}
		if groundedResultHasUsefulEvidence(result) {
			return true
		}
	}
	return false
}

func groundedHasSuccessfulAnalyzeEvidence(state *GroundTruthState) bool {
	if state == nil {
		return false
	}
	for toolCallID, result := range state.Results {
		if !result.OK {
			continue
		}
		call := state.Calls[toolCallID]
		if !groundedResultCarriesAnalyzeEvidence(call, result) {
			continue
		}
		if groundedResultHasAnalyzeEvidence(result.Result) {
			return true
		}
	}
	return false
}

func groundedHasSuccessfulReminderEvidence(state *GroundTruthState, expectedSessionID string) bool {
	if state == nil {
		return false
	}
	for toolCallID, result := range state.Results {
		if !result.OK || result.ExitCode != 0 {
			continue
		}
		call := state.Calls[toolCallID]
		if !groundedResultCarriesReminderEvidence(call, result) {
			continue
		}
		if !groundedReminderResultReady(result.Result) {
			continue
		}
		if groundedReminderSessionPropagated(call, result, expectedSessionID) {
			return true
		}
	}
	return false
}

func groundedResultCarriesWebEvidence(call GroundedToolCall, result GroundedToolResult) bool {
	if isGroundedWebToolFamily(firstNonEmptyString(call.Tool, result.Tool)) {
		return true
	}
	if normalizeGroundToolName(firstNonEmptyString(call.Tool, result.Tool)) != "bash" {
		return false
	}
	command := firstNonEmptyString(
		strings.TrimSpace(asString(call.Args["command"])),
		strings.TrimSpace(asString(call.Args["cmd"])),
	)
	if command == "" {
		return false
	}
	return isGroundedWebToolFamily(groundedCLICommandSkillToken(command))
}

func groundedResultCarriesAnalyzeEvidence(call GroundedToolCall, result GroundedToolResult) bool {
	if normalizeGroundToolName(firstNonEmptyString(call.Tool, result.Tool)) == "analyze" {
		return true
	}
	if normalizeGroundToolName(firstNonEmptyString(call.Tool, result.Tool)) != "bash" {
		return false
	}
	command := firstNonEmptyString(
		strings.TrimSpace(asString(call.Args["command"])),
		strings.TrimSpace(asString(call.Args["cmd"])),
	)
	if command == "" {
		return false
	}
	return normalizeGroundToolName(groundedCLICommandSkillToken(command)) == "analyze"
}

func groundedResultCarriesReminderEvidence(call GroundedToolCall, result GroundedToolResult) bool {
	if normalizeGroundToolName(firstNonEmptyString(call.Tool, result.Tool)) == "reminder" {
		return true
	}
	if normalizeGroundToolName(firstNonEmptyString(call.Tool, result.Tool)) != "bash" {
		return false
	}
	command := firstNonEmptyString(
		strings.TrimSpace(asString(call.Args["command"])),
		strings.TrimSpace(asString(call.Args["cmd"])),
	)
	if command == "" {
		return false
	}
	return normalizeGroundToolName(groundedCLICommandSkillToken(command)) == "reminder"
}

func groundedResultHasUsefulEvidence(result GroundedToolResult) bool {
	if groundedResultTitle(result.Result) != "" {
		return true
	}
	if groundedResultURL(result.Result) != "" {
		return true
	}
	return strings.TrimSpace(groundedResultSnippet(result)) != ""
}

func groundedResultHasAnalyzeEvidence(value any) bool {
	for _, payload := range deterministicStructuredPayloads(value) {
		if deterministicAnalyzeResultReady(payload) {
			return true
		}
	}
	return false
}

func groundedReminderResultReady(value any) bool {
	for _, payload := range deterministicStructuredPayloads(value) {
		if firstNonEmptyString(
			strings.TrimSpace(asString(extractField(payload, "message"))),
			strings.TrimSpace(asString(extractField(payload, "stdout"))),
		) != "" {
			return true
		}
		if reminder := parseGroundedNestedPayload(extractField(payload, "reminder")); reminder != nil {
			if firstNonEmptyString(
				strings.TrimSpace(asString(extractField(reminder, "message"))),
				strings.TrimSpace(asString(extractField(reminder, "fire_at"))),
				strings.TrimSpace(asString(extractField(reminder, "fireAt"))),
			) != "" {
				return true
			}
		}
	}
	return false
}

func groundedReminderSessionPropagated(call GroundedToolCall, result GroundedToolResult, expectedSessionID string) bool {
	if strings.TrimSpace(expectedSessionID) == "" {
		return true
	}
	if strings.Contains(serializedResult(result.Result), expectedSessionID) {
		return true
	}
	command := firstNonEmptyString(
		strings.TrimSpace(asString(call.Args["command"])),
		strings.TrimSpace(asString(call.Args["cmd"])),
	)
	return strings.Contains(command, expectedSessionID)
}

func groundedCommandHistorySucceeded(state *GroundTruthState, expectedCLIAction string) bool {
	if state == nil || strings.TrimSpace(expectedCLIAction) == "" {
		return false
	}
	for _, fact := range state.Commands {
		if normalizeGroundToolName(fact.Tool) != "bash" || fact.ExitCode != 0 {
			continue
		}
		if groundedCLICommandMatches(fact.Command, expectedCLIAction) {
			return true
		}
	}
	for toolCallID, call := range state.Calls {
		if normalizeGroundToolName(call.Tool) != "bash" {
			continue
		}
		command := firstNonEmptyString(
			strings.TrimSpace(asString(call.Args["command"])),
			strings.TrimSpace(asString(call.Args["cmd"])),
		)
		if !groundedCLICommandMatches(command, expectedCLIAction) {
			continue
		}
		result, ok := state.Results[toolCallID]
		if !ok {
			continue
		}
		if result.OK && result.ExitCode == 0 {
			return true
		}
	}
	return false
}

func groundedNeedsBrowser(result any) bool {
	encoded, err := json.Marshal(result)
	if err != nil {
		return false
	}
	normalized := strings.ToLower(string(encoded))
	for _, needle := range []string{"retry_browser", "browser_required", "needs_browser", "login_wall", "challenge"} {
		if strings.Contains(normalized, needle) {
			return true
		}
	}
	return false
}

func firstNonEmptyURL(exec *GroundedExecution) string {
	if exec == nil {
		return ""
	}
	for _, source := range []any{
		exec.Result.Result,
		extractField(exec.Result.Result, "result"),
		exec.Call.Args,
	} {
		for _, key := range []string{"final_url", "target_url", "url", "input"} {
			if value := strings.TrimSpace(asString(extractField(source, key))); value != "" {
				return value
			}
		}
		if nested := parseGroundedNestedPayload(source); nested != nil {
			for _, key := range []string{"final_url", "target_url", "url", "input"} {
				if value := strings.TrimSpace(asString(extractField(nested, key))); value != "" {
					return value
				}
			}
		}
	}
	return ""
}

func parseGroundedNestedPayload(value any) map[string]any {
	raw := strings.TrimSpace(asString(value))
	if raw == "" {
		return nil
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil || len(decoded) == 0 {
		return nil
	}
	return decoded
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
		if groundedVerificationHasBlockingPlanState(task) || task.GroundingStatus == GroundingStatusRejected {
			item.Status = "fail"
			item.Evidence = groundedVerificationFailureEvidence(task)
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

func groundedVerificationHasBlockingPlanState(task *Task) bool {
	failed, skipped := groundedPlanFailureFlags(task)
	if failed {
		return true
	}
	return skipped && !groundedVerificationAllowsSkippedWork(task)
}

func groundedVerificationFailureEvidence(task *Task) string {
	if task == nil {
		return "Task is missing grounded verification context."
	}
	failed, skipped := groundedPlanFailureFlags(task)
	switch {
	case task.GroundingStatus == GroundingStatusRejected && failed:
		return "Task contains failed work and rejected grounded output."
	case task.GroundingStatus == GroundingStatusRejected:
		return "Grounded runtime rejected the output."
	case failed:
		return "Task contains failed work that blocks completion."
	case skipped && groundedVerificationAllowsSkippedWork(task):
		return "Task skipped only post-evidence expansion work after satisfying the execution contract."
	case skipped:
		return "Task contains skipped work that blocks completion."
	default:
		return "Task did not satisfy grounded verification checks."
	}
}

func groundedPlanFailureFlags(task *Task) (failed bool, skipped bool) {
	if task == nil {
		return false, false
	}
	for _, step := range task.Plan {
		if isRuntimeMetaStep(step.Description) {
			continue
		}
		switch step.Status {
		case StepStatusFailed:
			failed = true
		case StepStatusSkipped:
			skipped = true
		}
	}
	return failed, skipped
}

func groundedVerificationAllowsSkippedWork(task *Task) bool {
	return taskUsesExecutionEquivalenceGate(task) && groundedShouldCompleteExecutionContract(task)
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
