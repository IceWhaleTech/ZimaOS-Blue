package harness

import (
	"context"
	"strings"
)

type OptimizationSurface string

const (
	OptimizationSurfaceConstraints OptimizationSurface = "constraints"
	OptimizationSurfaceRunnerCode OptimizationSurface = "runner_code"
	OptimizationSurfaceBuildRecipe OptimizationSurface = "build_recipe"
)

type OptimizationReason string

const (
	OptimizationReasonSelectorGateFailed   OptimizationReason = "selector_gate_failed"
	OptimizationReasonExecutionGateFailed  OptimizationReason = "execution_gate_failed"
	OptimizationReasonBudgetGateFailed     OptimizationReason = "budget_gate_failed"
	OptimizationReasonCutoverBlocking      OptimizationReason = "cutover_readiness_blocking"
	OptimizationReasonSelectorGatePassed   OptimizationReason = "selector_gate_passed"
	OptimizationReasonExecutionGatePassed  OptimizationReason = "execution_gate_passed"
	OptimizationReasonBudgetGatePassed     OptimizationReason = "budget_gate_passed"
)

type OptimizationTrigger struct {
	Reason              OptimizationReason      `json:"reason"`
	CandidateID         string                  `json:"candidate_id,omitempty"`
	EvalRunID           string                  `json:"eval_run_id,omitempty"`
	BaseEvalRunID       string                  `json:"base_eval_run_id,omitempty"`
	OptimizationRun     bool                    `json:"optimization_run,omitempty"`
	OptimizationSurface OptimizationSurface     `json:"optimization_surface,omitempty"`
	Metadata            map[string]interface{}  `json:"metadata,omitempty"`
}

type OptimizationTriggerer interface {
	TriggerOptimization(ctx context.Context, event OptimizationTrigger) error
}

func (c *Controller) SetOptimizationTriggerer(triggerer OptimizationTriggerer) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.optimization = triggerer
}

func (c *Controller) emitOptimizationTrigger(ctx context.Context, event OptimizationTrigger) {
	if c == nil || !shouldTriggerOptimization(event) {
		return
	}
	c.mu.RLock()
	triggerer := c.optimization
	c.mu.RUnlock()
	if triggerer == nil {
		return
	}
	_ = triggerer.TriggerOptimization(ctx, event)
}

func shouldTriggerOptimization(event OptimizationTrigger) bool {
	if event.OptimizationRun {
		return false
	}
	switch event.Reason {
	case OptimizationReasonSelectorGateFailed,
		OptimizationReasonExecutionGateFailed,
		OptimizationReasonBudgetGateFailed,
		OptimizationReasonCutoverBlocking:
		return true
	default:
		return false
	}
}

func evalRunIsOptimizationChild(evalRun *EvalRun) bool {
	if evalRun == nil {
		return false
	}
	if metadataBoolValue(evalRun.Metadata, "optimization_run") {
		return true
	}
	if strings.TrimSpace(metadataString(evalRun.Metadata, "optimization_parent_run_id")) != "" {
		return true
	}
	return false
}
