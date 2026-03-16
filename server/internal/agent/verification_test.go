package agent

import (
	"context"
	"strings"
	"testing"
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

	prompt := buildRecoveryUserPrompt(task, verificationCtx, verificationResult)
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
