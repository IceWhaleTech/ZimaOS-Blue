package builtin

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selfreflect"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
)

type SelfReflectExecutor interface {
	Reflect(ctx context.Context, input selfreflect.Input) (*selfreflect.Result, error)
}

type SelfReflect struct {
	manifest *skill.Manifest
	mu       sync.RWMutex
	executor SelfReflectExecutor
}

func NewSelfReflect() *SelfReflect {
	return &SelfReflect{
		manifest: &skill.Manifest{
			ID:          "self_reflect",
			Name:        "Self Reflect",
			Version:     "1.0.0",
			Description: "Extract grounded lessons learned from a completed task and optionally write them into memory for reuse.",
			Category:    "system",
			Icon:        "brain",
			Tags:        []string{"reflection", "lessons", "memory", "agent"},
			Inputs: []skill.Parameter{
				{Name: "goal", Type: "string", Description: "Task goal", Required: true},
				{Name: "task_id", Type: "string", Description: "Optional task identifier for memory tagging"},
				{Name: "plan", Type: "array", Description: "Step list with description/status/output"},
				{Name: "step_outputs", Type: "array", Description: "Optional step outputs to merge by index"},
				{Name: "verification_output", Type: "string", Description: "Verification stage output"},
				{Name: "final_status", Type: "string", Description: "completed|failed|partial", Required: true},
				{Name: "result_summary", Type: "string", Description: "Task outcome summary"},
				{Name: "failure_reason", Type: "string", Description: "Failure reason if the task failed"},
				{Name: "trigger_kind", Type: "string", Description: "Optional runtime reflection trigger kind"},
				{Name: "evidence_window", Type: "array", Description: "Optional runtime evidence window"},
				{Name: "runtime_signals", Type: "object", Description: "Optional runtime signal summary"},
				{Name: "disable_memory_write", Type: "boolean", Description: "Optional flag to suppress memory writes"},
				{Name: "source_kind", Type: "string", Description: "Optional source kind for human-review proposals"},
				{Name: "source_id", Type: "string", Description: "Optional source identifier for human-review proposals"},
				{Name: "evaluation_summary", Type: "object", Description: "Optional calibration or evaluator summary"},
				{Name: "proposal_candidates", Type: "array", Description: "Optional review-only AGENTS.md proposal candidates"},
				{Name: "proposal_mode", Type: "string", Description: "Optional proposal mode; only review_only is supported"},
			},
			Outputs: []skill.Parameter{
				{Name: "summary", Type: "string", Description: "Short reflection summary"},
				{Name: "lessons", Type: "array", Description: "Grounded reusable lessons"},
				{Name: "mutation_suggestions", Type: "array", Description: "Optional structured runtime mutation suggestions"},
				{Name: "reflection_signature", Type: "string", Description: "Stable signature for repeated runtime reflections"},
				{Name: "signal_strength", Type: "string", Description: "Low|medium|high confidence signal strength"},
				{Name: "memory_written", Type: "number", Description: "Number of memory entries written"},
				{Name: "skipped_reason", Type: "string", Description: "Reason reflection was skipped or filtered"},
				{Name: "proposal_count", Type: "number", Description: "Number of AGENTS.md review proposals created"},
				{Name: "proposal_ids", Type: "array", Description: "Created proposal ids"},
				{Name: "proposal_skipped_reason", Type: "string", Description: "Reason proposal creation was skipped"},
			},
		},
	}
}

func (s *SelfReflect) SetExecutor(executor SelfReflectExecutor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.executor = executor
}

func (s *SelfReflect) Manifest() *skill.Manifest { return s.manifest }

func (s *SelfReflect) Validate(input map[string]any) error {
	normalizeSelfReflectSkillInput(input)

	goal, _ := input["goal"].(string)
	resultSummary, _ := input["result_summary"].(string)
	failureReason, _ := input["failure_reason"].(string)
	_, hasProposalCandidates := input["proposal_candidates"].([]interface{})
	hasReflectionRecord := goal != "" || resultSummary != "" || failureReason != ""
	if !hasReflectionRecord && !hasProposalCandidates {
		return fmt.Errorf("goal is required")
	}
	status, _ := input["final_status"].(string)
	if hasReflectionRecord && status == "" {
		return fmt.Errorf("final_status is required")
	}
	if status == "" {
		return nil
	}
	switch status {
	case "completed", "failed", "partial", "running", "pending", "skipped":
	default:
		return fmt.Errorf("final_status must be one of: completed, failed, partial, running, pending, skipped")
	}
	return nil
}

func normalizeSelfReflectSkillInput(input map[string]any) {
	normalizeStringAlias(input, "goal", "task", "objective", "prompt")
	normalizeStringAlias(input, "final_status", "status")
	normalizeStringAlias(input, "result_summary", "summary", "outcome")
	normalizeStringAlias(input, "failure_reason", "error", "reason")
	normalizeStringAlias(input, "verification_output", "verification", "verification_result", "verificationResult")
	normalizeStringAlias(input, "task_id", "taskId")
	normalizeStringAlias(input, "trigger_kind", "triggerKind")
	normalizeStringAlias(input, "source_kind", "sourceKind")
	normalizeStringAlias(input, "source_id", "sourceId")
	normalizeStringAlias(input, "proposal_mode", "proposalMode")

	if _, ok := input["proposal_candidates"]; !ok {
		if raw, ok := input["proposalCandidates"]; ok {
			input["proposal_candidates"] = raw
		}
	}
	if rawStatus, ok := input["final_status"].(string); ok {
		status := strings.ToLower(strings.TrimSpace(rawStatus))
		if status != "" {
			input["final_status"] = status
		}
	}
	if _, ok := input["evidence_window"]; !ok {
		if raw, ok := input["evidenceWindow"]; ok {
			input["evidence_window"] = raw
		}
	}
	if _, ok := input["runtime_signals"]; !ok {
		if raw, ok := input["runtimeSignals"]; ok {
			input["runtime_signals"] = raw
		}
	}
	if _, ok := input["disable_memory_write"]; !ok {
		if raw, ok := input["disableMemoryWrite"]; ok {
			input["disable_memory_write"] = raw
		}
	}
}

func (s *SelfReflect) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	if err := s.Validate(input); err != nil {
		return skill.NewErrorResult(err), nil
	}

	s.mu.RLock()
	exec := s.executor
	s.mu.RUnlock()
	if exec == nil {
		return skill.NewErrorResult(fmt.Errorf("self reflect backend not available")), nil
	}

	parsed := selfreflect.Input{
		TaskID:             asString(input["task_id"]),
		Goal:               asString(input["goal"]),
		Plan:               parseReflectionPlan(input["plan"]),
		VerificationOutput: asString(input["verification_output"]),
		FinalStatus:        asString(input["final_status"]),
		ResultSummary:      asString(input["result_summary"]),
		FailureReason:      asString(input["failure_reason"]),
		TriggerKind:        asString(input["trigger_kind"]),
		EvidenceWindow:     parseRuntimeEvidenceWindow(input["evidence_window"]),
		RuntimeSignals:     asMap(input["runtime_signals"]),
		DisableMemoryWrite: asBool(input["disable_memory_write"]),
		SourceKind:         asString(input["source_kind"]),
		SourceID:           asString(input["source_id"]),
		EvaluationSummary:  asMap(input["evaluation_summary"]),
		ProposalCandidates: parseProposalCandidates(input["proposal_candidates"]),
		ProposalMode:       selfreflect.ProposalMode(asString(input["proposal_mode"])),
	}
	mergeStepOutputs(parsed.Plan, input["step_outputs"])

	result, err := exec.Reflect(ctx, parsed)
	if err != nil {
		return skill.NewErrorResult(err), nil
	}
	return skill.NewResult(map[string]any{
		"summary":                 result.Summary,
		"lessons":                 result.Lessons,
		"mutation_suggestions":    result.MutationSuggestions,
		"reflection_signature":    result.ReflectionSignature,
		"signal_strength":         result.SignalStrength,
		"memory_written":          result.MemoryWritten,
		"skipped_reason":          result.SkippedReason,
		"proposal_count":          result.ProposalCount,
		"proposal_ids":            result.ProposalIDs,
		"proposal_skipped_reason": result.ProposalSkippedReason,
	}), nil
}

func parseReflectionPlan(raw any) []selfreflect.Step {
	items, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	steps := make([]selfreflect.Step, 0, len(items))
	for _, item := range items {
		switch value := item.(type) {
		case string:
			steps = append(steps, selfreflect.Step{Description: value})
		case map[string]any:
			steps = append(steps, selfreflect.Step{
				Description: asString(value["description"]),
				Status:      asString(value["status"]),
				Output:      asString(value["output"]),
			})
		}
	}
	return steps
}

func mergeStepOutputs(steps []selfreflect.Step, raw any) {
	items, ok := raw.([]interface{})
	if !ok {
		return
	}
	for i := range items {
		if i >= len(steps) {
			break
		}
		if steps[i].Output != "" {
			continue
		}
		switch value := items[i].(type) {
		case string:
			steps[i].Output = value
		case map[string]any:
			steps[i].Output = asString(value["output"])
		}
	}
}

func asString(value any) string {
	s, _ := value.(string)
	return s
}

func asMap(value any) map[string]any {
	m, _ := value.(map[string]any)
	return m
}

func asBool(value any) bool {
	v, _ := value.(bool)
	return v
}

func parseProposalCandidates(raw any) []selfreflect.ProposalCandidate {
	items, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	out := make([]selfreflect.ProposalCandidate, 0, len(items))
	for _, item := range items {
		value, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, selfreflect.ProposalCandidate{
			Lesson:      asString(value["lesson"]),
			WhenToApply: asString(value["when_to_apply"]),
			Evidence:    asString(value["evidence"]),
			EvidenceIDs: parseStringArray(value["evidence_ids"]),
			TargetFile:  asString(value["target_file"]),
		})
	}
	return out
}

func parseStringArray(raw any) []string {
	items, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if value := asString(item); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func parseRuntimeEvidenceWindow(raw any) []selfreflect.RuntimeEvidenceItem {
	items, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	out := make([]selfreflect.RuntimeEvidenceItem, 0, len(items))
	for _, item := range items {
		value, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, selfreflect.RuntimeEvidenceItem{
			ID:           asString(value["id"]),
			EventType:    asString(value["event_type"]),
			StepIndex:    asInt(value["step_index"]),
			PlannerRound: asInt(value["planner_round"]),
			Summary:      asString(value["summary"]),
			PayloadJSON:  asString(value["payload_json"]),
		})
	}
	return out
}

func asInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case float64:
		return int(typed)
	default:
		return 0
	}
}
