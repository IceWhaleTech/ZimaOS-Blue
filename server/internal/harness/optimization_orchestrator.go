package harness

import (
	"context"
	"strings"
)

type OptimizationSurface string

const (
	OptimizationSurfaceConstraints     OptimizationSurface = "constraints"
	OptimizationSurfaceRunnerCode      OptimizationSurface = "runner_code"
	OptimizationSurfaceBuildRecipe     OptimizationSurface = "build_recipe"
	OptimizationSurfaceSkillDefinition OptimizationSurface = "skill_definition"
)

type OptimizationReason string

const (
	OptimizationReasonSelectorGateFailed  OptimizationReason = "selector_gate_failed"
	OptimizationReasonExecutionGateFailed OptimizationReason = "execution_gate_failed"
	OptimizationReasonBudgetGateFailed    OptimizationReason = "budget_gate_failed"
	OptimizationReasonCutoverBlocking     OptimizationReason = "cutover_readiness_blocking"
	OptimizationReasonManualSkillOptimize OptimizationReason = "manual_skill_optimize"
	OptimizationReasonSelectorGatePassed  OptimizationReason = "selector_gate_passed"
	OptimizationReasonExecutionGatePassed OptimizationReason = "execution_gate_passed"
	OptimizationReasonBudgetGatePassed    OptimizationReason = "budget_gate_passed"
)

type OptimizationTrigger struct {
	Reason              OptimizationReason     `json:"reason"`
	CandidateID         string                 `json:"candidate_id,omitempty"`
	EvalRunID           string                 `json:"eval_run_id,omitempty"`
	BaseEvalRunID       string                 `json:"base_eval_run_id,omitempty"`
	OptimizationRun     bool                   `json:"optimization_run,omitempty"`
	OptimizationSurface OptimizationSurface    `json:"optimization_surface,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
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

func evalRunOptimizationSurface(evalRun *EvalRun) OptimizationSurface {
	if evalRun == nil {
		return OptimizationSurfaceConstraints
	}
	return optimizationSurfaceFromMetadata(evalRun.Metadata)
}

func optimizationSurfaceFromMetadata(metadata map[string]interface{}) OptimizationSurface {
	if raw := strings.TrimSpace(metadataString(metadata, "optimization_surface")); raw != "" {
		return OptimizationSurface(raw)
	}
	if candidate := nestedMetadataMap(metadata, "skill_candidate"); len(candidate) > 0 {
		return OptimizationSurfaceSkillDefinition
	}
	return OptimizationSurfaceConstraints
}

func evalRunOptimizationMetadata(evalRun *EvalRun) map[string]interface{} {
	if evalRun == nil {
		return nil
	}
	metadata := cloneMetadataMap(evalRun.Metadata)
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	if candidateID := evalRunCandidateID(evalRun); candidateID != "" {
		metadata["candidate_id"] = candidateID
	}
	if surface := evalRunOptimizationSurface(evalRun); strings.TrimSpace(string(surface)) != "" {
		metadata["optimization_surface"] = string(surface)
	}
	appendToolSurfaceAuditMetadata(metadata, evalRun.Metadata)
	if len(metadata) == 0 {
		return nil
	}
	return metadata
}

func appendToolSurfaceAuditMetadata(dest map[string]interface{}, source map[string]interface{}) {
	if dest == nil || len(source) == 0 {
		return
	}
	structured := nestedMetadataMap(source, "selector_dry_run_response")
	discoveryRuntime := nestedMetadataMap(structured, "discovery_runtime")
	for _, key := range []string{
		"tool_surface_alias_rewrite_count",
		"tool_surface_cache_invalidation_count",
		"tool_surface_exec_cutover_count",
	} {
		if value, ok := firstToolSurfaceAuditMetadataValue(key, source, structured, discoveryRuntime); ok {
			dest[key] = value
		}
	}
}

func firstToolSurfaceAuditMetadataValue(key string, sources ...map[string]interface{}) (int, bool) {
	key = strings.TrimSpace(key)
	if key == "" {
		return 0, false
	}
	for _, source := range sources {
		if len(source) == 0 {
			continue
		}
		if value := intMetadataPtr(source[key]); value != nil {
			return *value, true
		}
	}
	return 0, false
}
