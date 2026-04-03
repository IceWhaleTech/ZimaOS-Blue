package harness

import (
	"context"
	"testing"
)

type recordingOptimizationTriggerer struct {
	events []OptimizationTrigger
}

func (r *recordingOptimizationTriggerer) TriggerOptimization(_ context.Context, event OptimizationTrigger) error {
	r.events = append(r.events, event)
	return nil
}

func TestControllerGateFailuresTriggerOptimization(t *testing.T) {
	triggerer := &recordingOptimizationTriggerer{}
	controller := &Controller{}
	controller.SetOptimizationTriggerer(triggerer)

	controller.emitOptimizationTrigger(context.Background(), OptimizationTrigger{
		Reason:      OptimizationReasonSelectorGateFailed,
		CandidateID: "candidate-1",
	})

	if len(triggerer.events) != 1 {
		t.Fatalf("trigger count = %d, want 1", len(triggerer.events))
	}
	if triggerer.events[0].Reason != OptimizationReasonSelectorGateFailed {
		t.Fatalf("reason = %q", triggerer.events[0].Reason)
	}
}

func TestControllerSuccessDoesNotTriggerOptimization(t *testing.T) {
	triggerer := &recordingOptimizationTriggerer{}
	controller := &Controller{}
	controller.SetOptimizationTriggerer(triggerer)

	controller.emitOptimizationTrigger(context.Background(), OptimizationTrigger{
		Reason: OptimizationReasonSelectorGatePassed,
	})

	if len(triggerer.events) != 0 {
		t.Fatalf("trigger count = %d, want 0", len(triggerer.events))
	}
}

func TestControllerOptimizationChildRunDoesNotTriggerOptimization(t *testing.T) {
	triggerer := &recordingOptimizationTriggerer{}
	controller := &Controller{}
	controller.SetOptimizationTriggerer(triggerer)

	controller.emitOptimizationTrigger(context.Background(), OptimizationTrigger{
		Reason:              OptimizationReasonExecutionGateFailed,
		CandidateID:         "candidate-1",
		OptimizationRun:     true,
		OptimizationSurface: OptimizationSurfaceRunnerCode,
	})

	if len(triggerer.events) != 0 {
		t.Fatalf("trigger count = %d, want 0", len(triggerer.events))
	}
}

func TestEvalRunOptimizationSurfaceDefaultsToSkillDefinitionWhenSkillCandidateExists(t *testing.T) {
	evalRun := &EvalRun{
		Metadata: map[string]interface{}{
			"skill_candidate": map[string]interface{}{
				"skill_id": "browser",
				"content":  "# Browser\n",
			},
		},
	}

	if got := evalRunOptimizationSurface(evalRun); got != OptimizationSurfaceSkillDefinition {
		t.Fatalf("evalRunOptimizationSurface = %q, want %q", got, OptimizationSurfaceSkillDefinition)
	}
}

func TestEvalRunOptimizationSurfacePrefersExplicitMetadata(t *testing.T) {
	evalRun := &EvalRun{
		Metadata: map[string]interface{}{
			"optimization_surface": string(OptimizationSurfaceRunnerCode),
			"skill_candidate": map[string]interface{}{
				"skill_id": "browser",
			},
		},
	}

	if got := evalRunOptimizationSurface(evalRun); got != OptimizationSurfaceRunnerCode {
		t.Fatalf("evalRunOptimizationSurface = %q, want %q", got, OptimizationSurfaceRunnerCode)
	}
}
