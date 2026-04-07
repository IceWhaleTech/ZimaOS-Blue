package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type RuntimeStage string

const (
	RuntimeStageNormalize RuntimeStage = "normalize"
	RuntimeStagePolicy    RuntimeStage = "policy"
	RuntimeStageApproval  RuntimeStage = "approval"
	RuntimeStageExecute   RuntimeStage = "execute"
	RuntimeStageFinalize  RuntimeStage = "finalize"
)

// GuardPipelineError normalizes preflight failures before a run is dispatched.
type GuardPipelineError struct {
	Stage   RuntimeStage           `json:"stage"`
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
	cause   error
}

func (e *GuardPipelineError) Error() string {
	if e == nil {
		return ""
	}
	return strings.TrimSpace(e.Message)
}

func (e *GuardPipelineError) ToolRuntimeCode() string {
	if e == nil {
		return ""
	}
	return strings.TrimSpace(e.Code)
}

func (e *GuardPipelineError) ToolRuntimeDetails() map[string]interface{} {
	if e == nil {
		return nil
	}
	details := cloneMetadataMap(e.Details)
	if stage := strings.TrimSpace(string(e.Stage)); stage != "" {
		if details == nil {
			details = map[string]interface{}{}
		}
		if _, exists := details["stage"]; !exists {
			details["stage"] = stage
		}
	}
	return details
}

func (e *GuardPipelineError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func newGuardPipelineError(stage RuntimeStage, code, message string, details map[string]interface{}) error {
	return newGuardPipelineErrorWithCause(stage, code, message, nil, details)
}

func newGuardPipelineErrorWithCause(stage RuntimeStage, code, message string, cause error, details map[string]interface{}) error {
	return &GuardPipelineError{
		Stage:   stage,
		Code:    strings.TrimSpace(code),
		Message: strings.TrimSpace(message),
		Details: cloneMetadataMap(details),
		cause:   cause,
	}
}

func validateRunSpec(spec RunSpec) error {
	if strings.TrimSpace(string(spec.Kind)) == "" {
		return newGuardPipelineError(RuntimeStageNormalize, "kind_required", "kind is required", nil)
	}
	if strings.TrimSpace(spec.Goal) == "" {
		return newGuardPipelineError(RuntimeStageNormalize, "goal_required", "goal is required", nil)
	}
	for field, value := range map[string]int{
		"max_steps":       spec.MaxSteps,
		"max_tool_rounds": spec.MaxToolRounds,
		"max_subagents":   spec.MaxSubagents,
		"max_depth":       spec.MaxDepth,
	} {
		if value < 0 {
			return newGuardPipelineError(RuntimeStageNormalize, "invalid_budget", fmt.Sprintf("%s cannot be negative", field), map[string]interface{}{
				"field": field,
				"value": value,
			})
		}
	}
	if spec.MaxDuration < 0 {
		return newGuardPipelineError(RuntimeStageNormalize, "invalid_budget", "max_duration cannot be negative", map[string]interface{}{
			"field": "max_duration",
		})
	}
	switch spec.ApprovalMode {
	case "", ApprovalModeAsk, ApprovalModeAllow, ApprovalModeDeny:
		return nil
	default:
		return newGuardPipelineError(RuntimeStageApproval, "invalid_approval_mode", fmt.Sprintf("approval_mode %q is not supported", spec.ApprovalMode), map[string]interface{}{
			"approval_mode": string(spec.ApprovalMode),
		})
	}
}

func guardStagePayload(run *Run) map[string]interface{} {
	if run == nil {
		return nil
	}
	payload := map[string]interface{}{
		"kind":            string(run.Kind),
		"approval_mode":   string(run.ApprovalMode),
		"sandbox_mode":    strings.TrimSpace(run.SandboxMode),
		"max_duration_ns": run.MaxDuration.Nanoseconds(),
		"max_steps":       run.MaxSteps,
		"max_tool_rounds": run.MaxToolRounds,
		"max_subagents":   run.MaxSubagents,
		"max_depth":       run.MaxDepth,
	}
	if providerID := strings.TrimSpace(run.ProviderID); providerID != "" {
		payload["provider_id"] = providerID
	}
	if run.ParentRunID != "" {
		payload["parent_run_id"] = run.ParentRunID
	}
	return payload
}

func (c *Controller) appendStageEvent(ctx context.Context, run *Run, stage RuntimeStage, message string, payload map[string]interface{}) error {
	if c == nil || run == nil || strings.TrimSpace(run.ID) == "" {
		return nil
	}
	if payload == nil {
		payload = map[string]interface{}{}
	}
	payload["stage"] = string(stage)
	if _, exists := payload["status"]; !exists && strings.TrimSpace(string(run.Status)) != "" {
		payload["status"] = string(run.Status)
	}
	raw, _ := json.Marshal(payload)
	return c.AppendEvent(ctx, RunEvent{
		RunID:       run.ID,
		RootRunID:   run.RootRunID,
		ParentRunID: run.ParentRunID,
		Type:        "stage_changed",
		Message:     strings.TrimSpace(message),
		PayloadJSON: string(raw),
	})
}
