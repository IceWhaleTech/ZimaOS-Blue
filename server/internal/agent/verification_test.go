package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/knowledge"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
)

func TestInferTaskKind(t *testing.T) {
	testCases := []struct {
		name     string
		goal     string
		steps    []PlanStep
		criteria []string
		want     TaskKind
	}{
		{
			name:     "code",
			goal:     "fix parser regression and run tests",
			steps:    []PlanStep{{Description: "update parser.go"}, {Description: "run focused test"}},
			criteria: []string{"tests pass"},
			want:     TaskKindCode,
		},
		{
			name:     "docs",
			goal:     "update README and changelog copy",
			steps:    []PlanStep{{Description: "edit README.md"}},
			criteria: []string{"documentation reflects the new flow"},
			want:     TaskKindDocs,
		},
		{
			name:     "research",
			goal:     "research provider options and collect evidence",
			steps:    []PlanStep{{Description: "search for sources"}},
			criteria: []string{"report cites evidence"},
			want:     TaskKindResearch,
		},
		{
			name:     "ops",
			goal:     "check service config and restart the worker",
			steps:    []PlanStep{{Description: "inspect logs"}},
			criteria: []string{"service is healthy"},
			want:     TaskKindOps,
		},
		{
			name:     "generic",
			goal:     "organize the deliverable",
			steps:    []PlanStep{{Description: "prepare the final response"}},
			criteria: []string{"deliverable is complete"},
			want:     TaskKindGeneric,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := inferTaskKind(tc.goal, tc.steps, tc.criteria); got != tc.want {
				t.Fatalf("inferTaskKind() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBuildVerificationSystemPrompt_ByTaskKind(t *testing.T) {
	docsPrompt := buildVerificationSystemPrompt(TaskKindDocs, nil)
	if !containsAll(docsPrompt, []string{
		"strict verification engine",
		"Do not default to build or tests",
		"criteria_results",
	}) {
		t.Fatalf("unexpected docs verification prompt: %q", docsPrompt)
	}

	codePrompt := buildVerificationSystemPrompt(TaskKindCode, nil)
	if !containsAll(codePrompt, []string{
		"confirm the intended deliverable landed",
		"build/test/check commands",
	}) {
		t.Fatalf("unexpected code verification prompt: %q", codePrompt)
	}
}

func TestCompileVerificationContext_UsesEffectiveDefaults(t *testing.T) {
	task := &Task{
		Goal: "organize the deliverable",
		Plan: []PlanStep{{Description: "prepare the final response"}},
	}

	ctx := compileVerificationContext(task)
	if got, want := ctx.SuccessCriteria, defaultTaskSuccessCriteria; len(got) != len(want) {
		t.Fatalf("expected default success criteria count %d, got %d", len(want), len(got))
	}
	if got, want := ctx.FallbackPlan, defaultTaskFallbackPlan; len(got) != len(want) {
		t.Fatalf("expected default fallback plan count %d, got %d", len(want), len(got))
	}
}

func TestParseVerificationResult_StrictContract(t *testing.T) {
	if _, err := parseVerificationResult(`{"status":"unknown","summary":"x","criteria_results":[{"criterion":"a","status":"pass","evidence":"ok"}],"executed_checks":["review"]}`); err == nil {
		t.Fatal("expected unsupported status to fail parsing")
	}

	if _, err := parseVerificationResult(`{"status":"pass","summary":"","criteria_results":[{"criterion":"a","status":"pass","evidence":"ok"}],"executed_checks":["review"]}`); err == nil {
		t.Fatal("expected empty summary to fail parsing")
	}
}

func TestRunVerification_RequiresAllCriteriaPass(t *testing.T) {
	runner := &Runner{
		llm: &captureResponseLLM{response: `{"status":"pass","summary":"Only one criterion was checked.","criteria_results":[{"criterion":"tests pass","status":"pass","evidence":"Focused tests passed."}],"executed_checks":["run focused tests"]}`},
	}
	task := &Task{
		ID:              "verify-task",
		Goal:            "fix parser",
		SuccessCriteria: []string{"tests pass", "no blocking errors in final result"},
		Plan: []PlanStep{
			{Description: "apply parser fix", Status: StepStatusCompleted, Output: "patched parser"},
		},
	}

	result, output, err := runner.runVerification(context.Background(), task, compileVerificationContext(task))
	if err == nil {
		t.Fatal("expected verification to fail when a criterion is missing")
	}
	if result == nil {
		t.Fatal("expected verification result even on failure")
	}
	if !containsAll(output, []string{"Missing criteria coverage:", "no blocking errors in final result"}) {
		t.Fatalf("expected output to mention missing criteria coverage, got %q", output)
	}
}

func TestMergeTaskSuccessCriteria_LocksExecutionEquivalenceContract(t *testing.T) {
	task := &Task{
		Metadata: map[string]interface{}{
			"gate_type":             "execution_equivalence",
			"task_success_criteria": []interface{}{"evidence_tool_used"},
			"task_fallback_plan":    []interface{}{"run blue web_search first"},
		},
	}

	gotCriteria := mergeTaskSuccessCriteria(task, []string{"planner invented another criterion"})
	if len(gotCriteria) != 1 || gotCriteria[0] != "evidence_tool_used" {
		t.Fatalf("mergeTaskSuccessCriteria() = %#v, want [evidence_tool_used]", gotCriteria)
	}

	gotFallback := mergeTaskFallbackPlan(task, []string{"planner invented another fallback"})
	if len(gotFallback) != 1 || gotFallback[0] != "run blue web_search first" {
		t.Fatalf("mergeTaskFallbackPlan() = %#v, want [run blue web_search first]", gotFallback)
	}
}

func TestMergeTaskSuccessCriteria_UsesRequiredObservationsForExecutionEquivalence(t *testing.T) {
	task := &Task{
		Metadata: map[string]interface{}{
			"routing_contract": map[string]interface{}{
				"gate_type": "execution_equivalence",
			},
			"harness_contract": map[string]interface{}{
				"required_observations": []interface{}{"evidence_tool_used", "planner_memory_skipped"},
			},
		},
	}

	got := mergeTaskSuccessCriteria(task, []string{"planner invented another criterion"})
	want := []string{"evidence_tool_used", "planner_memory_skipped"}
	if len(got) != len(want) {
		t.Fatalf("mergeTaskSuccessCriteria() len = %d, want %d (%#v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("mergeTaskSuccessCriteria()[%d] = %q, want %q (%#v)", i, got[i], want[i], got)
		}
	}
}

func TestMergeTaskSuccessCriteria_DefaultsExecutionEquivalenceToContractSentinel(t *testing.T) {
	task := &Task{
		Metadata: map[string]interface{}{
			"gate_type": "execution_equivalence",
		},
	}

	got := mergeTaskSuccessCriteria(task, []string{"planner invented another criterion"})
	if len(got) != 1 || got[0] != executionContractSatisfiedCriterion {
		t.Fatalf("mergeTaskSuccessCriteria() = %#v, want [%s]", got, executionContractSatisfiedCriterion)
	}
}

func TestPreferredTaskModel_UsesGroupInputModel(t *testing.T) {
	task := &Task{
		Metadata: map[string]interface{}{
			"group_input": map[string]interface{}{
				"model": "claude-haiku-4-5-20251001",
			},
		},
	}

	if got := preferredTaskModel(task); got != "claude-haiku-4-5-20251001" {
		t.Fatalf("preferredTaskModel() = %q, want claude-haiku-4-5-20251001", got)
	}
}

func TestPreferredTaskModel_FallsBackToPolicyModelHint(t *testing.T) {
	task := &Task{
		Metadata: map[string]interface{}{
			"policy_model_hint": "claude-sonnet-4-6",
		},
	}

	if got := preferredTaskModel(task); got != "claude-sonnet-4-6" {
		t.Fatalf("preferredTaskModel() = %q, want claude-sonnet-4-6", got)
	}
}

func TestGeneratePlanForTask_UsesPreferredTaskModel(t *testing.T) {
	llmStub := &captureRequestLLM{response: `{"goal":"test","subtasks":[{"description":"step one"}]}`}
	runner := &Runner{llm: llmStub}
	task := &Task{
		Metadata: map[string]interface{}{
			"group_input": map[string]interface{}{
				"model": "claude-haiku-4-5-20251001",
			},
		},
	}

	if _, err := runner.generatePlanForTask(context.Background(), task, "test", ""); err != nil {
		t.Fatalf("generatePlanForTask failed: %v", err)
	}
	if llmStub.lastModel != "claude-haiku-4-5-20251001" {
		t.Fatalf("planner model = %q, want claude-haiku-4-5-20251001", llmStub.lastModel)
	}
}

func TestGeneratePlanForTask_PinsPreferredProviderInContext(t *testing.T) {
	llmStub := &captureRequestLLM{response: `{"goal":"test","subtasks":[{"description":"step one"}]}`}
	runner := &Runner{llm: llmStub}
	task := &Task{
		Metadata: map[string]interface{}{
			"group_input": map[string]interface{}{
				"provider_id": "openai-prod",
			},
		},
	}

	if _, err := runner.generatePlanForTask(context.Background(), task, "test", ""); err != nil {
		t.Fatalf("generatePlanForTask failed: %v", err)
	}
	if llmStub.lastProviderID != "openai-prod" {
		t.Fatalf("planner provider_id = %q, want openai-prod", llmStub.lastProviderID)
	}
}

func TestGeneratePlanForTask_UsesPolicyModelHintWhenExplicitModelMissing(t *testing.T) {
	llmStub := &captureRequestLLM{response: `{"goal":"test","subtasks":[{"description":"step one"}]}`}
	runner := &Runner{llm: llmStub}
	task := &Task{
		Metadata: map[string]interface{}{
			"policy_model_hint": "claude-sonnet-4-6",
		},
	}

	if _, err := runner.generatePlanForTask(context.Background(), task, "test", ""); err != nil {
		t.Fatalf("generatePlanForTask failed: %v", err)
	}
	if llmStub.lastModel != "claude-sonnet-4-6" {
		t.Fatalf("planner model = %q, want claude-sonnet-4-6", llmStub.lastModel)
	}
}

func TestGroundedPlannerAndVerifier_UseProvidedModel(t *testing.T) {
	plannerLLM := &captureRequestLLM{response: `{"status":"complete","reason":"enough evidence"}`}
	planner := NewGroundedPlanner(plannerLLM)
	if _, err := planner.Decide(context.Background(), PlannerInput{Model: "claude-haiku-4-5-20251001"}); err != nil {
		t.Fatalf("GroundedPlanner.Decide failed: %v", err)
	}
	if plannerLLM.lastModel != "claude-haiku-4-5-20251001" {
		t.Fatalf("grounded planner model = %q, want claude-haiku-4-5-20251001", plannerLLM.lastModel)
	}

	verifierLLM := &captureRequestLLM{response: `{"summary":"ok","claims":[{"type":"unknown","text":"unknown"}]}`}
	verifier := NewGroundedVerifier(nil)
	if _, err := verifier.Respond(context.Background(), verifierLLM, ResponderInput{Model: "claude-haiku-4-5-20251001"}); err != nil {
		t.Fatalf("GroundedVerifier.Respond failed: %v", err)
	}
	if verifierLLM.lastModel != "claude-haiku-4-5-20251001" {
		t.Fatalf("grounded verifier model = %q, want claude-haiku-4-5-20251001", verifierLLM.lastModel)
	}
}

func TestGroundedVerifierRespond_UsesDeterministicStructuredWebEvidenceWithoutLLM(t *testing.T) {
	state := NewGroundTruthState()
	toolCallID := "task/deterministic-web/tc/1"
	state.Calls[toolCallID] = GroundedToolCall{
		ToolCallID: toolCallID,
		Tool:       "bash",
		Args: map[string]any{
			"command": `blue web_query input="OpenAI Responses API latest docs"`,
		},
	}
	state.Results[toolCallID] = GroundedToolResult{
		ToolCallID: toolCallID,
		Tool:       "bash",
		ExitCode:   0,
		OK:         true,
		Result: map[string]any{
			"stdout": "status: ok",
			"data": map[string]any{
				"status":    "ok",
				"final_url": "https://developers.openai.com/api/reference/resources/responses",
				"title":     "Responses | OpenAI API Reference",
				"content":   "Responses | OpenAI API Reference\nBuild stateful interactions with the Responses API.",
			},
		},
	}

	verifier := NewGroundedVerifier(nil)
	response, err := verifier.Respond(context.Background(), nil, ResponderInput{
		GroundState:      state,
		PriorToolCallIDs: []string{toolCallID},
	})
	if err != nil {
		t.Fatalf("GroundedVerifier.Respond returned error: %v", err)
	}
	if response == nil || len(response.Claims) == 0 {
		t.Fatalf("expected deterministic grounded claims, got %#v", response)
	}
	decision := verifier.Verify(state, response)
	if !decision.Valid {
		t.Fatalf("deterministic response should verify, got violations=%v output=%q", decision.Violations, decision.Output)
	}
	for _, want := range []string{
		"Responses | OpenAI API Reference",
		"https://developers.openai.com/api/reference/resources/responses",
	} {
		if !strings.Contains(decision.Output, want) {
			t.Fatalf("expected output to contain %q, got %q", want, decision.Output)
		}
	}
}

func TestRunVerificationAndRecovery_DisableModelRoutingInContext(t *testing.T) {
	llmStub := &routingDisableCaptureLLM{}
	runner := &Runner{llm: llmStub}
	task := &Task{
		ID:   "verification-routing-opt-out",
		Goal: "Search the latest docs and verify the result.",
		Plan: []PlanStep{
			{
				Index:       0,
				Description: "Run the web query",
				Status:      StepStatusCompleted,
				Output:      "Found the latest docs URL.",
			},
		},
	}
	verificationCtx := VerificationContext{
		TaskKind:        TaskKindResearch,
		Goal:            task.Goal,
		SuccessCriteria: []string{"evidence is grounded"},
		FallbackPlan:    []string{"retry with a narrower verification scope"},
		PlannedSteps:    []string{"Run the web query"},
	}

	if _, _, err := runner.runVerification(context.Background(), task, verificationCtx); err != nil {
		t.Fatalf("runVerification failed: %v", err)
	}
	if _, err := runner.runRecovery(context.Background(), task, verificationCtx, &VerificationResult{
		Status:            "fail",
		Summary:           "Need one narrower retry.",
		CriteriaResults:   []CriterionResult{{Criterion: "evidence is grounded", Status: "fail", Evidence: "Need one narrower retry."}},
		SuggestedRecovery: "retry with a narrower verification scope",
		ExecutedChecks:    []string{"review task record"},
	}); err != nil {
		t.Fatalf("runRecovery failed: %v", err)
	}
	if !llmStub.verificationDisabled {
		t.Fatal("expected verification loop to disable proxy model routing in context")
	}
	if !llmStub.recoveryDisabled {
		t.Fatal("expected recovery loop to disable proxy model routing in context")
	}
}

func TestRunner_RunRecovery_InjectsLoopKnowledgeContext(t *testing.T) {
	llmStub := &captureResponseLLM{response: "Recovered with a narrower retry."}
	resolver := &mockLoopKnowledgeResolver{
		result: &knowledge.LoopContextResult{
			Context:   "<loop_knowledge>\n- [knowledge slug=recovery-guidance page_type=decision status=active confidence=high] When verification fails on missing evidence, use the first fallback item and keep the retry narrow.\n</loop_knowledge>",
			UsedCount: 1,
			UsedSlugs: []string{"recovery-guidance"},
		},
	}
	runner := &Runner{llm: llmStub}
	runner.SetKnowledgeResolver(resolver)

	task := &Task{
		ID:   "recovery-knowledge",
		Goal: "stabilize the parser retry path",
		Plan: []PlanStep{
			{Description: "apply parser fix", Status: StepStatusCompleted, Output: "patched parser"},
		},
	}
	verificationCtx := VerificationContext{
		TaskKind:        TaskKindCode,
		Goal:            task.Goal,
		SuccessCriteria: []string{"tests pass"},
		FallbackPlan:    []string{"inspect the failing test first", "retry with narrower parser scope"},
	}
	verificationResult := &VerificationResult{
		Status:            "fail",
		Summary:           "The retry is blocked by missing evidence from the focused test.",
		SuggestedRecovery: "inspect the failing test first",
		CriteriaResults: []CriterionResult{{
			Criterion: "tests pass",
			Status:    "fail",
			Evidence:  "Missing evidence from the focused parser test output.",
		}},
	}

	if _, err := runner.runRecovery(context.Background(), task, verificationCtx, verificationResult); err != nil {
		t.Fatalf("runRecovery failed: %v", err)
	}
	if resolver.called != 1 {
		t.Fatalf("resolver calls = %d, want 1", resolver.called)
	}
	if resolver.lastReq.Stage != knowledge.LoopContextStageRecovery {
		t.Fatalf("resolver stage = %q, want recovery", resolver.lastReq.Stage)
	}
	userPrompt := llmStub.requests[0].Messages[1].Content
	for _, want := range []string{"Relevant knowledge:", "<loop_knowledge>", "recovery-guidance"} {
		if !strings.Contains(userPrompt, want) {
			t.Fatalf("expected recovery user prompt to contain %q, got %q", want, userPrompt)
		}
	}
}

func TestGroundedVerifierRespond_UsesDeterministicAnalyzeEvidenceWithoutLLM(t *testing.T) {
	state := NewGroundTruthState()
	toolCallID := "task/deterministic-analyze/tc/1"
	state.Calls[toolCallID] = GroundedToolCall{
		ToolCallID: toolCallID,
		Tool:       "bash",
		Args: map[string]any{
			"command": `blue analyze topic="Summarize and extract the key points" url="https://example.com/blog"`,
		},
	}
	state.Results[toolCallID] = GroundedToolResult{
		ToolCallID: toolCallID,
		Tool:       "bash",
		ExitCode:   0,
		OK:         true,
		Result: map[string]any{
			"stdout": "answer: Analysis completed for Summarize and extract the key points.",
			"data": map[string]any{
				"answer":      "Analysis completed for Summarize and extract the key points.",
				"message":     "Analysis ready: Summarize and extract the key points",
				"output_mode": "inline",
				"topic":       "Summarize and extract the key points",
			},
		},
	}

	verifier := NewGroundedVerifier(nil)
	response, err := verifier.Respond(context.Background(), nil, ResponderInput{
		GroundState:      state,
		PriorToolCallIDs: []string{toolCallID},
	})
	if err != nil {
		t.Fatalf("GroundedVerifier.Respond returned error: %v", err)
	}
	if response == nil || len(response.Claims) == 0 {
		t.Fatalf("expected deterministic grounded claims, got %#v", response)
	}
	decision := verifier.Verify(state, response)
	if !decision.Valid {
		t.Fatalf("deterministic analyze response should verify, got violations=%v output=%q", decision.Violations, decision.Output)
	}
	if !strings.Contains(decision.Output, "Analysis completed for Summarize and extract the key points.") {
		t.Fatalf("expected output to contain deterministic analyze answer, got %q", decision.Output)
	}
}

func TestBuildRecoveryUserPrompt_UsesFallbackPlanOrder(t *testing.T) {
	task := &Task{
		Goal: "fix parser",
		Plan: []PlanStep{
			{Description: "apply parser fix", Status: StepStatusCompleted, Output: "patched parser"},
			{Description: "verify parser fix", Status: StepStatusFailed, Output: "tests still failing"},
		},
	}
	verificationCtx := VerificationContext{
		TaskKind:        TaskKindCode,
		Goal:            task.Goal,
		SuccessCriteria: []string{"tests pass"},
		FallbackPlan:    []string{"inspect the failing test first", "retry with narrower parser scope"},
	}
	verificationResult := &VerificationResult{
		Status:            "fail",
		Summary:           "Focused verification failed.",
		SuggestedRecovery: "review the failing parser branch",
		CriteriaResults: []CriterionResult{{
			Criterion: "tests pass",
			Status:    "fail",
			Evidence:  "Focused parser tests still fail.",
		}},
		ExecutedChecks: []string{"run focused parser tests"},
	}

	prompt := buildRecoveryUserPrompt(task, verificationCtx, verificationResult, "")
	first := "1. inspect the failing test first"
	second := "2. retry with narrower parser scope"
	firstIndex := strings.Index(prompt, first)
	secondIndex := strings.Index(prompt, second)
	if firstIndex < 0 || secondIndex < 0 || firstIndex > secondIndex {
		t.Fatalf("expected fallback plan order to be preserved, got %q", prompt)
	}
	if !containsAll(prompt, []string{
		"Verifier suggested recovery (supporting constraint only): review the failing parser branch",
		"Recent successful steps:",
	}) {
		t.Fatalf("expected recovery prompt to include verifier suggestion and recent successes, got %q", prompt)
	}
}

func TestRunner_ExternalVerificationRecoveryFlow(t *testing.T) {
	store := testStore(t)
	llmStub := &externalRecoveryFlowLLM{}
	runner := NewRunner(store, llmStub, nil, nil, nil, RunnerConfig{TaskTimeout: 10 * time.Second})
	t.Cleanup(func() { runner.Shutdown() })

	task := &Task{
		ID:     "external-qa-recovery",
		UserID: "u1",
		Goal:   "stabilize parser",
		Metadata: map[string]interface{}{
			"enable_external_qa":    true,
			"max_recovery_attempts": 1,
		},
	}
	if _, err := runner.SubmitTask(context.Background(), task, ""); err != nil {
		t.Fatalf("SubmitTask failed: %v", err)
	}

	got := waitForTerminalTask(t, store, task.ID, 5*time.Second)
	if got.Status != TaskStatusCompleted {
		t.Fatalf("status = %q, want completed", got.Status)
	}
	if !hasRuntimeTransition(got.RuntimeAudit, RuntimeStateVerify, RuntimeStateRecover) {
		t.Fatal("expected runtime transition VERIFY -> RECOVER")
	}
	if !hasRuntimeTransition(got.RuntimeAudit, RuntimeStateRecover, RuntimeStateVerify) {
		t.Fatal("expected runtime transition RECOVER -> VERIFY")
	}
	if len(got.VerificationErrors) == 0 {
		t.Fatal("expected first failed external verification to be retained in verification errors")
	}

	sawRecovery := false
	sawRetryExternalVerify := false
	for _, step := range got.Plan {
		switch {
		case strings.Contains(step.Description, "Recovery:"):
			sawRecovery = true
		case strings.Contains(step.Description, "[external QA]") && strings.Contains(step.Description, "bounded retry"):
			sawRetryExternalVerify = true
		}
	}
	if !sawRecovery {
		t.Fatalf("expected recovery step in plan, got %#v", got.Plan)
	}
	if !sawRetryExternalVerify {
		t.Fatalf("expected bounded retry external verification step in plan, got %#v", got.Plan)
	}
}

type externalRecoveryFlowLLM struct {
	verificationCalls int
}

type routingDisableCaptureLLM struct {
	verificationDisabled bool
	recoveryDisabled     bool
}

type captureRequestLLM struct {
	response       string
	lastModel      string
	lastProviderID string
}

func (m *captureRequestLLM) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	m.lastModel = req.Model
	m.lastProviderID = proxy.GetPinnedProvider(ctx)
	return &llm.ChatResponse{
		Message: llm.Message{Role: llm.RoleAssistant, Content: m.response},
	}, nil
}

func (m *externalRecoveryFlowLLM) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	switch {
	case len(req.Messages) > 0 && strings.Contains(req.Messages[0].Content, "bounded recovery engine"):
		return &llm.ChatResponse{
			Message: llm.Message{Role: llm.RoleAssistant, Content: "Applied the narrower parser fix and refreshed the deliverable."},
		}, nil
	case isVerificationPrompt(req):
		m.verificationCalls++
		if m.verificationCalls == 1 {
			return &llm.ChatResponse{
				Message: llm.Message{Role: llm.RoleAssistant, Content: `{"status":"fail","summary":"External QA found that the parser fix is still incomplete.","criteria_results":[{"criterion":"parser fix is complete","status":"fail","evidence":"The deliverable still needs the narrower fix."}],"suggested_recovery":"apply the narrower parser fix","executed_checks":["review task record"]}`},
			}, nil
		}
		return &llm.ChatResponse{
			Message: llm.Message{Role: llm.RoleAssistant, Content: `{"status":"pass","summary":"External QA passed after the recovery attempt.","criteria_results":[{"criterion":"parser fix is complete","status":"pass","evidence":"The narrowed fix was applied and the deliverable was refreshed."}],"executed_checks":["review task record"]}`},
		}, nil
	case len(req.Messages) > 0 && strings.Contains(req.Messages[0].Content, "deterministic task planner"):
		return &llm.ChatResponse{
			Message: llm.Message{Role: llm.RoleAssistant, Content: `{"goal":"stabilize parser","subtasks":[{"description":"apply parser fix"}],"success_criteria":["parser fix is complete"],"fallback_plan":["apply the narrower parser fix"]}`},
		}, nil
	default:
		return &llm.ChatResponse{
			Message: llm.Message{Role: llm.RoleAssistant, Content: defaultResponseForRequest(req)},
		}, nil
	}
}

func (m *routingDisableCaptureLLM) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	switch {
	case len(req.Messages) > 0 && strings.Contains(req.Messages[0].Content, "bounded recovery engine"):
		m.recoveryDisabled = proxy.DisableModelRoutingFromContext(ctx)
		return &llm.ChatResponse{
			Message: llm.Message{Role: llm.RoleAssistant, Content: "Applied one bounded recovery retry."},
		}, nil
	case isVerificationPrompt(req):
		m.verificationDisabled = proxy.DisableModelRoutingFromContext(ctx)
		result := VerificationResult{
			Status:          "pass",
			Summary:         "External verification passed.",
			CriteriaResults: []CriterionResult{{Criterion: "evidence is grounded", Status: "pass", Evidence: "The task record includes grounded evidence."}},
			ExecutedChecks:  []string{"review task record"},
		}
		raw, _ := json.Marshal(result)
		return &llm.ChatResponse{
			Message: llm.Message{Role: llm.RoleAssistant, Content: string(raw)},
		}, nil
	default:
		return &llm.ChatResponse{
			Message: llm.Message{Role: llm.RoleAssistant, Content: defaultResponseForRequest(req)},
		}, nil
	}
}

func TestProgressSignatureState_DetectsRepeatedNoProgress(t *testing.T) {
	state := &ProgressSignatureState{}
	for i := 0; i < 2; i++ {
		if state.Observe("exec:{\"command\":\"go test ./...\"}", "retry tests", []string{"error:command exited with code #"}) {
			t.Fatalf("unexpected no-progress trigger at iteration %d", i)
		}
	}
	if !state.Observe("exec:{\"command\":\"go test ./...\"}", "retry tests", []string{"error:command exited with code #"}) {
		t.Fatal("expected third identical progress signature to trigger no-progress detection")
	}
}

func TestProgressSignatureState_DetectsSearchFamilyNoProgressWithVariantQueries(t *testing.T) {
	state := &ProgressSignatureState{}
	queries := []string{
		`web_query:target=openai responses api docs`,
		`web_query:target=latest openai responses api documentation`,
		`web_query:target=openai responses api latest docs`,
	}
	for i := 0; i < len(queries)-1; i++ {
		if detection := state.ObserveDetailed(queries[i], "continue searching", []string{"status:ok|mode:search_read|target_url:https://platform.openai.com/docs/api-reference/responses"}); detection.Abort {
			t.Fatalf("unexpected family no-progress trigger at iteration %d: %#v", i, detection)
		}
	}
	if detection := state.ObserveDetailed(queries[len(queries)-1], "continue searching", []string{"status:ok|mode:search_read|target_url:https://platform.openai.com/docs/api-reference/responses"}); !detection.Abort {
		t.Fatal("expected repeated web search family outcome to trigger no-progress detection")
	}
}

func TestTransitionState_PersistsSingleUpdate(t *testing.T) {
	store := testStore(t)
	task := &Task{
		ID:           "transition-task",
		UserID:       "u1",
		Goal:         "plan task",
		Status:       TaskStatusPending,
		RuntimeState: RuntimeStateIntake,
	}
	if err := store.Create(context.Background(), task); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	before := sqliteTotalChanges(t, store)
	runner := NewRunner(store, nil, nil, nil, nil, RunnerConfig{})
	if err := runner.transitionState(context.Background(), task, RuntimeStatePlan, "start planning", nil, TaskStatusPlanning); err != nil {
		t.Fatalf("transitionState failed: %v", err)
	}
	after := sqliteTotalChanges(t, store)
	if got := after - before; got != 1 {
		t.Fatalf("transitionState should persist exactly one update, got delta=%d", got)
	}
}

func TestBuildBaseResultSummary_UsesEffectiveCriteriaCount(t *testing.T) {
	task := &Task{
		Goal: "organize the deliverable",
		Plan: []PlanStep{{Description: "prepare the final response", Status: StepStatusCompleted}},
	}

	summary := buildBaseResultSummary(task, TaskStatusCompleted, "")
	if !strings.Contains(summary, "verification passed for 2 success criterion/criteria") {
		t.Fatalf("expected summary to use effective criteria defaults, got %q", summary)
	}
}

func sqliteTotalChanges(t *testing.T, store *Store) int {
	t.Helper()
	var changes int
	if err := store.db.QueryRow(`SELECT total_changes()`).Scan(&changes); err != nil {
		t.Fatalf("total_changes query failed: %v", err)
	}
	return changes
}

func containsAll(haystack string, needles []string) bool {
	for _, needle := range needles {
		if !strings.Contains(haystack, needle) {
			return false
		}
	}
	return true
}
