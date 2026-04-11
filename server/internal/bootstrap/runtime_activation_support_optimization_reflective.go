package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/optimization"
)

const (
	reflectiveInitialMutationCount = 4
	reflectiveMaxGenerations       = 3
	reflectiveFrontierSize         = 2
	reflectiveMaxEvaluations       = 8

	reflectiveRuntimeWindowSize     = 50
	reflectiveRuntimeMinimumSamples = 20
)

type reflectiveObjectiveVector struct {
	ExecutionPassRateDelta      float64 `json:"execution_pass_rate_delta,omitempty"`
	VerificationPassRateDelta   float64 `json:"verification_pass_rate_delta,omitempty"`
	EvidenceBackedPassRateDelta float64 `json:"evidence_backed_pass_rate_delta,omitempty"`
	MedianLatencyIncreaseRate   float64 `json:"median_latency_increase_rate,omitempty"`
	RepeatFailureRecurrence     float64 `json:"repeat_failure_recurrence,omitempty"`
}

type reflectiveEvaluatedCandidate struct {
	CandidateID     string                    `json:"candidate_id,omitempty"`
	Generation      int                       `json:"generation,omitempty"`
	HardPass        bool                      `json:"hard_pass,omitempty"`
	FollowupGate    string                    `json:"followup_gate,omitempty"`
	FollowupState   string                    `json:"followup_state,omitempty"`
	FollowupEvalRun string                    `json:"followup_eval_run_id,omitempty"`
	Summary         string                    `json:"summary,omitempty"`
	DiffSize        int                       `json:"diff_size,omitempty"`
	Objectives      reflectiveObjectiveVector `json:"objectives,omitempty"`
	SkillCandidate  map[string]interface{}    `json:"skill_candidate,omitempty"`
}

type runtimeValueWindowMetrics struct {
	BeforeSampleCount         int
	AfterSampleCount          int
	BeforeFailureRecurrence   float64
	AfterFailureRecurrence    float64
	BeforeMedianDurationMs    float64
	AfterMedianDurationMs     float64
	BeforeMedianTotalTokens   float64
	AfterMedianTotalTokens    float64
	BeforeCaptureQualityScore float64
	AfterCaptureQualityScore  float64
	BeforeValidationQuality   float64
	AfterValidationQuality    float64
}

type reflectiveRuntimeValueReport struct {
	Status                     string   `json:"status,omitempty"`
	BeforeSampleCount          int      `json:"before_sample_count,omitempty"`
	AfterSampleCount           int      `json:"after_sample_count,omitempty"`
	FailureRecurrenceDelta     float64  `json:"failure_recurrence_delta,omitempty"`
	MedianDurationDeltaRate    float64  `json:"median_duration_delta_rate,omitempty"`
	MedianTotalTokensDeltaRate float64  `json:"median_total_tokens_delta_rate,omitempty"`
	CaptureQualityDelta        float64  `json:"capture_quality_delta,omitempty"`
	ValidationQualityDelta     float64  `json:"validation_quality_delta,omitempty"`
	ValueSummary               string   `json:"value_summary,omitempty"`
	TopImprovements            []string `json:"top_improvements,omitempty"`
	TopTradeoffs               []string `json:"top_tradeoffs,omitempty"`
	Confidence                 string   `json:"confidence,omitempty"`
}

func defaultReflectiveSearchConfig() map[string]interface{} {
	return map[string]interface{}{
		"mode":                   "reflective_search_v1",
		"initial_mutation_count": reflectiveInitialMutationCount,
		"max_generations":        reflectiveMaxGenerations,
		"frontier_size":          reflectiveFrontierSize,
		"max_evaluations":        reflectiveMaxEvaluations,
		"optimized_parts":        []string{string(harness.OptimizationSurfaceSkillDefinition)},
	}
}

func defaultReflectiveValueReport(offlineRecommendation string) map[string]interface{} {
	recommendation := strings.TrimSpace(offlineRecommendation)
	if recommendation == "" {
		recommendation = "hold"
	}
	return map[string]interface{}{
		"offline_recommendation": recommendation,
		"runtime_status":         "not_started",
		"value_summary":          "Reflective search submitted candidate evaluations and is waiting for follow-up evidence.",
		"top_improvements":       []string{},
		"top_tradeoffs":          []string{"Runtime value has not been observed yet."},
		"confidence":             "low",
	}
}

func defaultRuntimeValueReport(status string) reflectiveRuntimeValueReport {
	normalized := strings.TrimSpace(status)
	if normalized == "" {
		normalized = "not_started"
	}
	report := reflectiveRuntimeValueReport{
		Status:     normalized,
		Confidence: "low",
	}
	switch normalized {
	case "not_started":
		report.ValueSummary = "Runtime validation has not started."
	case "provisional":
		report.ValueSummary = "Runtime validation has started, but the observation window is still small."
	case "confirmed":
		report.ValueSummary = "Runtime validation confirms the improvement is holding in real usage."
	case "regressing":
		report.ValueSummary = "Runtime signals are regressing after promotion."
	}
	return report
}

func runtimeValueReportToMap(report reflectiveRuntimeValueReport) map[string]interface{} {
	improvements := append([]string(nil), report.TopImprovements...)
	tradeoffs := append([]string(nil), report.TopTradeoffs...)
	return map[string]interface{}{
		"status":                         strings.TrimSpace(report.Status),
		"before_sample_count":            report.BeforeSampleCount,
		"after_sample_count":             report.AfterSampleCount,
		"failure_recurrence_delta":       report.FailureRecurrenceDelta,
		"median_duration_delta_rate":     report.MedianDurationDeltaRate,
		"median_total_tokens_delta_rate": report.MedianTotalTokensDeltaRate,
		"capture_quality_delta":          report.CaptureQualityDelta,
		"validation_quality_delta":       report.ValidationQualityDelta,
		"value_summary":                  strings.TrimSpace(report.ValueSummary),
		"top_improvements":               improvements,
		"top_tradeoffs":                  tradeoffs,
		"confidence":                     strings.TrimSpace(report.Confidence),
	}
}

func computeReflectiveParetoSelection(candidates []reflectiveEvaluatedCandidate, frontierSize int) ([]string, string) {
	if len(candidates) == 0 {
		return nil, ""
	}
	hardPass := make([]reflectiveEvaluatedCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if !candidate.HardPass || strings.TrimSpace(candidate.CandidateID) == "" {
			continue
		}
		hardPass = append(hardPass, candidate)
	}
	if len(hardPass) == 0 {
		return nil, ""
	}

	frontier := make([]reflectiveEvaluatedCandidate, 0, len(hardPass))
	for i, candidate := range hardPass {
		dominated := false
		for j, other := range hardPass {
			if i == j {
				continue
			}
			if reflectiveCandidateDominates(other, candidate) {
				dominated = true
				break
			}
		}
		if !dominated {
			frontier = append(frontier, candidate)
		}
	}
	sort.SliceStable(frontier, func(i, j int) bool {
		left := frontier[i]
		right := frontier[j]
		if left.Objectives.ExecutionPassRateDelta != right.Objectives.ExecutionPassRateDelta {
			return left.Objectives.ExecutionPassRateDelta > right.Objectives.ExecutionPassRateDelta
		}
		if left.Objectives.VerificationPassRateDelta != right.Objectives.VerificationPassRateDelta {
			return left.Objectives.VerificationPassRateDelta > right.Objectives.VerificationPassRateDelta
		}
		if left.Objectives.EvidenceBackedPassRateDelta != right.Objectives.EvidenceBackedPassRateDelta {
			return left.Objectives.EvidenceBackedPassRateDelta > right.Objectives.EvidenceBackedPassRateDelta
		}
		if left.Objectives.MedianLatencyIncreaseRate != right.Objectives.MedianLatencyIncreaseRate {
			return left.Objectives.MedianLatencyIncreaseRate < right.Objectives.MedianLatencyIncreaseRate
		}
		if left.Objectives.RepeatFailureRecurrence != right.Objectives.RepeatFailureRecurrence {
			return left.Objectives.RepeatFailureRecurrence < right.Objectives.RepeatFailureRecurrence
		}
		if left.DiffSize != right.DiffSize {
			return left.DiffSize < right.DiffSize
		}
		return left.CandidateID < right.CandidateID
	})
	if frontierSize > 0 && len(frontier) > frontierSize {
		frontier = frontier[:frontierSize]
	}
	frontierIDs := make([]string, 0, len(frontier))
	for _, candidate := range frontier {
		frontierIDs = append(frontierIDs, candidate.CandidateID)
	}
	selected := ""
	if len(frontier) > 0 {
		selected = strings.TrimSpace(frontier[0].CandidateID)
	}
	return frontierIDs, selected
}

func reflectiveCandidateDominates(left, right reflectiveEvaluatedCandidate) bool {
	allNoWorse := left.Objectives.ExecutionPassRateDelta >= right.Objectives.ExecutionPassRateDelta &&
		left.Objectives.VerificationPassRateDelta >= right.Objectives.VerificationPassRateDelta &&
		left.Objectives.EvidenceBackedPassRateDelta >= right.Objectives.EvidenceBackedPassRateDelta &&
		left.Objectives.MedianLatencyIncreaseRate <= right.Objectives.MedianLatencyIncreaseRate &&
		left.Objectives.RepeatFailureRecurrence <= right.Objectives.RepeatFailureRecurrence
	if !allNoWorse {
		return false
	}
	return left.Objectives.ExecutionPassRateDelta > right.Objectives.ExecutionPassRateDelta ||
		left.Objectives.VerificationPassRateDelta > right.Objectives.VerificationPassRateDelta ||
		left.Objectives.EvidenceBackedPassRateDelta > right.Objectives.EvidenceBackedPassRateDelta ||
		left.Objectives.MedianLatencyIncreaseRate < right.Objectives.MedianLatencyIncreaseRate ||
		left.Objectives.RepeatFailureRecurrence < right.Objectives.RepeatFailureRecurrence ||
		left.DiffSize < right.DiffSize
}

func buildRuntimeValueReportFromWindows(metrics runtimeValueWindowMetrics) reflectiveRuntimeValueReport {
	report := reflectiveRuntimeValueReport{
		BeforeSampleCount:          metrics.BeforeSampleCount,
		AfterSampleCount:           metrics.AfterSampleCount,
		FailureRecurrenceDelta:     metrics.AfterFailureRecurrence - metrics.BeforeFailureRecurrence,
		MedianDurationDeltaRate:    ratioDelta(metrics.BeforeMedianDurationMs, metrics.AfterMedianDurationMs),
		MedianTotalTokensDeltaRate: ratioDelta(metrics.BeforeMedianTotalTokens, metrics.AfterMedianTotalTokens),
		CaptureQualityDelta:        metrics.AfterCaptureQualityScore - metrics.BeforeCaptureQualityScore,
		ValidationQualityDelta:     metrics.AfterValidationQuality - metrics.BeforeValidationQuality,
		Confidence:                 "medium",
	}
	improvements := make([]string, 0, 4)
	tradeoffs := make([]string, 0, 4)

	if report.FailureRecurrenceDelta < 0 {
		improvements = append(improvements, fmt.Sprintf("Repeat failure recurrence improved by %.2f", math.Abs(report.FailureRecurrenceDelta)))
	} else if report.FailureRecurrenceDelta > 0 {
		tradeoffs = append(tradeoffs, fmt.Sprintf("Repeat failure recurrence worsened by %.2f", report.FailureRecurrenceDelta))
	}
	if report.MedianDurationDeltaRate < -0.05 {
		improvements = append(improvements, fmt.Sprintf("Median runtime duration improved by %.0f%%", math.Abs(report.MedianDurationDeltaRate)*100))
	} else if report.MedianDurationDeltaRate > 0.15 {
		tradeoffs = append(tradeoffs, fmt.Sprintf("Median runtime duration regressed by %.0f%%", report.MedianDurationDeltaRate*100))
	}
	if report.MedianTotalTokensDeltaRate < -0.05 {
		improvements = append(improvements, fmt.Sprintf("Median token usage improved by %.0f%%", math.Abs(report.MedianTotalTokensDeltaRate)*100))
	} else if report.MedianTotalTokensDeltaRate > 0.15 {
		tradeoffs = append(tradeoffs, fmt.Sprintf("Median token usage regressed by %.0f%%", report.MedianTotalTokensDeltaRate*100))
	}
	if report.CaptureQualityDelta > 0.05 {
		improvements = append(improvements, fmt.Sprintf("Runtime capture quality improved by %.2f", report.CaptureQualityDelta))
	} else if report.CaptureQualityDelta < -0.05 {
		tradeoffs = append(tradeoffs, fmt.Sprintf("Runtime capture quality dropped by %.2f", math.Abs(report.CaptureQualityDelta)))
	}
	if report.ValidationQualityDelta > 0.05 {
		improvements = append(improvements, fmt.Sprintf("Runtime validation quality improved by %.2f", report.ValidationQualityDelta))
	} else if report.ValidationQualityDelta < -0.05 {
		tradeoffs = append(tradeoffs, fmt.Sprintf("Runtime validation quality dropped by %.2f", math.Abs(report.ValidationQualityDelta)))
	}

	switch {
	case metrics.AfterSampleCount < reflectiveRuntimeMinimumSamples || metrics.BeforeSampleCount < reflectiveRuntimeMinimumSamples:
		report.Status = "provisional"
		report.Confidence = "low"
		report.ValueSummary = fmt.Sprintf(
			"Runtime window is still provisional (%d before / %d after; need %d after samples).",
			metrics.BeforeSampleCount,
			metrics.AfterSampleCount,
			reflectiveRuntimeMinimumSamples,
		)
	case len(tradeoffs) > 0 && (report.FailureRecurrenceDelta > 0.02 || report.MedianDurationDeltaRate > 0.15 || report.MedianTotalTokensDeltaRate > 0.15 || report.CaptureQualityDelta < -0.05 || report.ValidationQualityDelta < -0.05):
		report.Status = "regressing"
		report.Confidence = "high"
		report.ValueSummary = "Runtime regression signals outweigh the offline gains."
	default:
		report.Status = "confirmed"
		if len(improvements) == 0 {
			improvements = append(improvements, "Runtime behavior remains stable within the observation window.")
		}
		report.ValueSummary = "Runtime observation confirms the candidate is holding its gains."
	}

	report.TopImprovements = improvements
	report.TopTradeoffs = tradeoffs
	return report
}

func ratioDelta(before, after float64) float64 {
	if before <= 0 || after < 0 {
		return 0
	}
	return (after - before) / before
}

func estimateReflectiveDiffSize(baseContent, candidateContent string) int {
	base := baseContent
	target := candidateContent
	prefix := 0
	for prefix < len(base) && prefix < len(target) && base[prefix] == target[prefix] {
		prefix++
	}
	base = base[prefix:]
	target = target[prefix:]
	suffix := 0
	for suffix < len(base) && suffix < len(target) && base[len(base)-1-suffix] == target[len(target)-1-suffix] {
		suffix++
	}
	if suffix > 0 {
		base = base[:len(base)-suffix]
		target = target[:len(target)-suffix]
	}
	return len(base) + len(target)
}

func reflectiveOptimizationRecord(record optimization.OptimizationRunRecord) bool {
	if len(record) == 0 {
		return false
	}
	if proposals := optimizationMetadataSliceOfMaps(record, "proposal_set"); len(proposals) > 0 {
		return true
	}
	if candidates := optimizationMetadataSliceOfMaps(record, "evaluated_candidates"); len(candidates) > 0 {
		return true
	}
	searchConfig := optimizationMetadataMap(record, "search_config")
	return strings.EqualFold(optimizationMetadataString(searchConfig, "mode"), "reflective_search_v1")
}

func (t *harnessOptimizationTriggerer) submitReflectiveOptimizationProposalSet(
	ctx context.Context,
	optimizationRunID string,
	event harness.OptimizationTrigger,
	outcome *optimizationFollowupOutcome,
	response *optimizationSkillCandidateResponse,
) (*optimizationFollowupOutcome, error) {
	if outcome == nil {
		outcome = &optimizationFollowupOutcome{}
	}
	outcome.SearchConfig = defaultReflectiveSearchConfig()
	outcome.ReflectionSummary = cloneOptimizationMetadata(response.ReflectionSummary)
	outcome.OfflineRecommendation = "hold"
	outcome.RuntimeStatus = "not_started"
	outcome.OfflineValueReport = defaultReflectiveValueReport(outcome.OfflineRecommendation)
	outcome.RuntimeValueReport = runtimeValueReportToMap(defaultRuntimeValueReport(outcome.RuntimeStatus))

	var parentEvalRun *harness.EvalRun
	var err error
	if strings.TrimSpace(event.EvalRunID) != "" {
		parentEvalRun, err = t.controller.GetEvalRun(ctx, event.EvalRunID)
		if err != nil {
			return outcome, err
		}
	}
	followupGate := firstNonEmptyOptimizationValue(
		optimizationMetadataString(event.Metadata, "followup_gate"),
		optimizationFollowupGate(map[string]interface{}{"reason": string(event.Reason)}),
	)
	followupTarget, err := t.resolveOptimizationFollowupTarget(ctx, event, parentEvalRun, followupGate)
	if err != nil {
		outcome.State = "skipped"
		outcome.SkippedReason = strings.TrimSpace(err.Error())
		outcome.Message = firstNonEmptyOptimizationValue(outcome.Message, "runtime follow-up assets are not ready")
		return outcome, nil
	}

	baseSkillCandidate := optimizationMetadataMap(event.Metadata, "skill_candidate")
	baseContent := optimizationMetadataRawString(baseSkillCandidate, "content")
	proposals := response.ProposalSet
	if len(proposals) > reflectiveMaxEvaluations {
		proposals = proposals[:reflectiveMaxEvaluations]
	}

	outcome.ProposalSet = make([]map[string]interface{}, 0, len(proposals))
	outcome.EvaluatedCandidates = make([]map[string]interface{}, 0, len(proposals))
	for _, proposal := range proposals {
		if proposal.SkillCandidate == nil {
			continue
		}
		materialized, materializeErr := materializeOptimizationSkillCandidate(event, proposal.SkillCandidate)
		if materializeErr != nil {
			outcome.EvaluatedCandidates = append(outcome.EvaluatedCandidates, map[string]interface{}{
				"candidate_id":    firstNonEmptyOptimizationValue(strings.TrimSpace(proposal.CandidateID), strings.TrimSpace(event.CandidateID)),
				"generation":      proposal.Generation,
				"rationale":       strings.TrimSpace(proposal.Rationale),
				"followup_gate":   followupGate,
				"followup_state":  "candidate_invalid",
				"followup_error":  strings.TrimSpace(materializeErr.Error()),
				"hard_pass":       false,
				"diff_size":       0,
				"skill_candidate": cloneOptimizationMetadata(baseSkillCandidate),
			})
			continue
		}
		candidateID := firstNonEmptyOptimizationValue(strings.TrimSpace(proposal.CandidateID), optimizationMetadataString(materialized, "candidate_id"))
		proposalEntry := map[string]interface{}{
			"candidate_id":    candidateID,
			"generation":      proposal.Generation,
			"rationale":       strings.TrimSpace(proposal.Rationale),
			"skill_candidate": materialized,
		}
		outcome.ProposalSet = append(outcome.ProposalSet, proposalEntry)

		evalRun, submitErr := t.controller.SubmitEvalRun(ctx, harness.EvalRunSpec{
			EvalSpecID:        strings.TrimSpace(followupTarget.EvalSpecID),
			BaselineEvalRunID: strings.TrimSpace(followupTarget.BaseEvalRunID),
			Title:             buildOptimizationFollowupTitle(parentEvalRun, followupTarget.TitleBase, candidateID),
			OwnerUserID:       strings.TrimSpace(followupTarget.OwnerUserID),
			TriggerKind:       "optimization_followup",
			TriggerRef:        strings.TrimSpace(optimizationRunID),
			Metadata:          buildOptimizationFollowupMetadata(event, optimizationRunID, materialized, nil),
		})
		evaluatedEntry := map[string]interface{}{
			"candidate_id":    candidateID,
			"generation":      proposal.Generation,
			"rationale":       strings.TrimSpace(proposal.Rationale),
			"followup_gate":   followupGate,
			"followup_state":  "submitted",
			"hard_pass":       false,
			"diff_size":       estimateReflectiveDiffSize(baseContent, optimizationMetadataRawString(materialized, "content")),
			"skill_candidate": materialized,
		}
		if submitErr != nil {
			evaluatedEntry["followup_state"] = "submit_error"
			evaluatedEntry["followup_error"] = strings.TrimSpace(submitErr.Error())
			outcome.EvaluatedCandidates = append(outcome.EvaluatedCandidates, evaluatedEntry)
			continue
		}
		if outcome.EvalRun == nil {
			outcome.EvalRun = evalRun
		}
		evaluatedEntry["followup_eval_run_id"] = strings.TrimSpace(evalRun.ID)
		evaluatedEntry["followup_group_id"] = strings.TrimSpace(evalRun.GroupID)
		evaluatedEntry["followup_eval_spec_id"] = strings.TrimSpace(evalRun.EvalSpecID)
		outcome.EvaluatedCandidates = append(outcome.EvaluatedCandidates, evaluatedEntry)
	}
	if len(outcome.ProposalSet) == 0 {
		outcome.State = "no_change"
		outcome.Message = firstNonEmptyOptimizationValue(outcome.Message, "no reflective proposals were valid")
		return outcome, nil
	}
	outcome.State = "submitted"
	outcome.Message = firstNonEmptyOptimizationValue(
		strings.TrimSpace(response.Message),
		fmt.Sprintf("Submitted %d reflective follow-up evaluations.", len(outcome.EvaluatedCandidates)),
	)
	outcome.SampleEfficiencyReport = map[string]interface{}{
		"proposal_count":            len(outcome.ProposalSet),
		"evaluated_candidate_count": len(outcome.EvaluatedCandidates),
		"frontier_size":             reflectiveFrontierSize,
		"max_evaluations":           reflectiveMaxEvaluations,
	}
	return outcome, nil
}

func optimizationMetadataSliceOfMaps(meta map[string]interface{}, key string) []map[string]interface{} {
	if len(meta) == 0 {
		return nil
	}
	raw, ok := meta[key]
	if !ok {
		return nil
	}
	items, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		typed, ok := item.(map[string]interface{})
		if !ok || len(typed) == 0 {
			continue
		}
		out = append(out, cloneOptimizationMetadata(typed))
	}
	return out
}

func optimizationRecordTime(value interface{}) (time.Time, bool) {
	switch typed := value.(type) {
	case time.Time:
		return typed.UTC(), true
	case string:
		if strings.TrimSpace(typed) == "" {
			return time.Time{}, false
		}
		parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(typed))
		if err != nil {
			return time.Time{}, false
		}
		return parsed.UTC(), true
	default:
		return time.Time{}, false
	}
}

func (a *harnessOptimizationManagerAdapter) reconcileReflectiveOptimizationRecord(ctx context.Context, record optimization.OptimizationRunRecord) (optimization.OptimizationRunRecord, error) {
	if a == nil || a.manager == nil || a.controller == nil || len(record) == 0 {
		return record, nil
	}
	updated := optimization.OptimizationRunRecord(cloneOptimizationMetadata(record))
	rawCandidates := optimizationMetadataSliceOfMaps(updated, "evaluated_candidates")
	if len(rawCandidates) == 0 {
		return updated, nil
	}

	evaluated := make([]map[string]interface{}, 0, len(rawCandidates))
	candidates := make([]reflectiveEvaluatedCandidate, 0, len(rawCandidates))
	anyRunning := false
	allTerminal := true
	for _, rawCandidate := range rawCandidates {
		candidate, normalized, running, terminal := a.reconcileReflectiveEvaluatedCandidate(ctx, updated, rawCandidate)
		evaluated = append(evaluated, normalized)
		candidates = append(candidates, candidate)
		anyRunning = anyRunning || running
		allTerminal = allTerminal && terminal
	}
	updated["evaluated_candidates"] = evaluated

	frontierIDs, selectedID := computeReflectiveParetoSelection(candidates, reflectiveFrontierSize)
	if len(frontierIDs) > 0 {
		updated["pareto_frontier"] = frontierIDs
	} else {
		delete(updated, "pareto_frontier")
	}

	selected := reflectiveCandidateByID(candidates, selectedID)
	if selected != nil {
		updated["selected_candidate"] = reflectiveEvaluatedCandidateToMap(*selected)
	} else {
		delete(updated, "selected_candidate")
	}

	offline := a.buildReflectiveOfflineValueReport(ctx, updated, candidates, frontierIDs, selected, anyRunning, allTerminal)
	updated["offline_value_report"] = offline
	updated["offline_recommendation"] = optimizationMetadataString(offline, "offline_recommendation")
	updated["sample_efficiency_report"] = buildReflectiveSampleEfficiencyReport(candidates, frontierIDs, selected)

	switch {
	case anyRunning:
		updated["followup_state"] = "running"
		updated["followup_decision"] = "running"
		updated["followup_summary"] = "Reflective follow-up evaluations are still running."
		if optimizationMetadataString(updated, "promotion_state") != "promoted" {
			updated["promotion_state"] = string(harness.SkillRevisionStatusCandidate)
		}
	case selected != nil:
		updated["followup_state"] = "accepted"
		updated["followup_decision"] = "accepted"
		updated["followup_gate"] = firstNonEmptyOptimizationValue(selected.FollowupGate, optimizationFollowupGate(updated))
		updated["followup_gate_passed"] = true
		updated["followup_summary"] = firstNonEmptyOptimizationValue(selected.Summary, "Selected reflective candidate passed the hard follow-up gate.")
		if optimizationMetadataString(updated, "promotion_state") != "promoted" {
			updated["promotion_state"] = string(harness.SkillRevisionStatusAccepted)
		}
		evolutionCase, revision, err := a.ensureReflectiveSelectedCandidateRevision(ctx, updated, *selected)
		if err != nil {
			updated["followup_revision_error"] = strings.TrimSpace(err.Error())
		} else {
			delete(updated, "followup_revision_error")
			if evolutionCase != nil {
				updated["skill_evolution_case_id"] = strings.TrimSpace(evolutionCase.ID)
			}
			if revision != nil {
				updated["skill_revision_id"] = strings.TrimSpace(revision.ID)
				if selectedMap, ok := updated["selected_candidate"].(map[string]interface{}); ok {
					selectedMap["skill_revision_id"] = strings.TrimSpace(revision.ID)
				}
			}
		}
	default:
		updated["followup_state"] = "no_improvement"
		updated["followup_decision"] = "no_improvement"
		updated["followup_gate_passed"] = false
		updated["followup_summary"] = "No reflective candidate passed the hard follow-up gate."
		if optimizationMetadataString(updated, "promotion_state") != "promoted" {
			updated["promotion_state"] = string(harness.SkillRevisionStatusRejected)
		}
	}

	runtimeReport := a.buildReflectiveRuntimeValueReport(ctx, updated, selected)
	updated["runtime_value_report"] = runtimeReport
	updated["runtime_status"] = optimizationMetadataString(runtimeReport, "status")

	return a.persistReconciledOptimizationRecord(ctx, updated)
}

func (a *harnessOptimizationManagerAdapter) reconcileReflectiveEvaluatedCandidate(
	ctx context.Context,
	record optimization.OptimizationRunRecord,
	raw map[string]interface{},
) (reflectiveEvaluatedCandidate, map[string]interface{}, bool, bool) {
	normalized := cloneOptimizationMetadata(raw)
	candidate := reflectiveEvaluatedCandidateFromMap(normalized)
	candidate.FollowupGate = firstNonEmptyOptimizationValue(candidate.FollowupGate, optimizationMetadataString(normalized, "followup_gate"), optimizationFollowupGate(record))
	candidate.FollowupEvalRun = firstNonEmptyOptimizationValue(candidate.FollowupEvalRun, optimizationMetadataString(normalized, "followup_eval_run_id"))

	if candidate.DiffSize <= 0 {
		baseContent := optimizationMetadataRawString(optimizationMetadataMap(optimizationMetadataMap(record, "metadata"), "skill_candidate"), "content")
		candidate.DiffSize = estimateReflectiveDiffSize(baseContent, optimizationMetadataRawString(candidate.SkillCandidate, "content"))
	}
	normalized["candidate_id"] = candidate.CandidateID
	normalized["followup_gate"] = candidate.FollowupGate
	normalized["diff_size"] = candidate.DiffSize

	if candidate.FollowupEvalRun == "" {
		candidate.FollowupState = firstNonEmptyOptimizationValue(candidate.FollowupState, "pending")
		normalized["followup_state"] = candidate.FollowupState
		return candidate, normalized, false, false
	}

	evalRun, err := a.controller.GetEvalRun(ctx, candidate.FollowupEvalRun)
	if err != nil {
		candidate.FollowupState = "error"
		normalized["followup_state"] = candidate.FollowupState
		normalized["followup_error"] = strings.TrimSpace(err.Error())
		return candidate, normalized, false, true
	}
	if evalRun == nil {
		candidate.FollowupState = "error"
		normalized["followup_state"] = candidate.FollowupState
		normalized["followup_error"] = fmt.Sprintf("follow-up eval run %q not found", candidate.FollowupEvalRun)
		return candidate, normalized, false, true
	}
	normalized["followup_eval_status"] = normalizeOptimizationFollowupEvalStatus(evalRun.Status)
	if isOptimizationFollowupActive(evalRun.Status) {
		candidate.FollowupState = "running"
		normalized["followup_state"] = candidate.FollowupState
		return candidate, normalized, true, false
	}
	if evalRun.Status != harness.RunGroupStatusCompleted {
		candidate.FollowupState = "rejected"
		normalized["followup_state"] = candidate.FollowupState
		normalized["hard_pass"] = false
		return candidate, normalized, false, true
	}

	assessment, err := a.assessCompletedFollowupEval(ctx, optimization.OptimizationRunRecord{
		"reason":           optimizationMetadataString(record, "reason"),
		"base_eval_run_id": optimizationMetadataString(record, "base_eval_run_id"),
		"followup_gate":    candidate.FollowupGate,
	}, evalRun)
	if err != nil {
		candidate.FollowupState = "error"
		normalized["followup_state"] = candidate.FollowupState
		normalized["followup_error"] = strings.TrimSpace(err.Error())
		return candidate, normalized, false, true
	}

	candidate.HardPass = assessment.Passed
	candidate.FollowupState = assessment.Decision
	candidate.Summary = assessment.Summary
	candidate.Objectives = a.collectReflectiveObjectiveVector(ctx, record, evalRun.ID)

	normalized["followup_state"] = assessment.Decision
	normalized["followup_summary"] = assessment.Summary
	normalized["followup_gate"] = assessment.Gate
	normalized["hard_pass"] = assessment.Passed
	normalized["objectives"] = reflectiveObjectiveVectorToMap(candidate.Objectives)
	return candidate, normalized, false, true
}

func (a *harnessOptimizationManagerAdapter) collectReflectiveObjectiveVector(
	ctx context.Context,
	record optimization.OptimizationRunRecord,
	targetEvalRunID string,
) reflectiveObjectiveVector {
	vector := reflectiveObjectiveVector{}
	baseEvalRunID := optimizationMetadataString(record, "base_eval_run_id")
	if strings.TrimSpace(targetEvalRunID) == "" || strings.TrimSpace(baseEvalRunID) == "" || a == nil || a.controller == nil {
		return vector
	}
	if report, err := a.controller.EvaluateExecutionEquivalence(ctx, targetEvalRunID, harness.ExecutionEquivalenceRequest{
		BaseEvalRunID: baseEvalRunID,
	}); err == nil && report != nil {
		vector.ExecutionPassRateDelta = report.Metrics.PassRateDelta
		vector.VerificationPassRateDelta = report.Metrics.VerificationPassRateDelta
		vector.EvidenceBackedPassRateDelta = report.Metrics.EvidenceBackedPassRateDelta
	}
	if report, err := a.controller.EvaluateSkillCutoverBudgetGate(ctx, targetEvalRunID, harness.SkillCutoverBudgetRequest{
		BaseEvalRunID: baseEvalRunID,
	}); err == nil && report != nil {
		vector.MedianLatencyIncreaseRate = report.Metrics.MedianLatencyIncreaseRate
	}
	return vector
}

func (a *harnessOptimizationManagerAdapter) buildReflectiveOfflineValueReport(
	ctx context.Context,
	record optimization.OptimizationRunRecord,
	candidates []reflectiveEvaluatedCandidate,
	frontierIDs []string,
	selected *reflectiveEvaluatedCandidate,
	anyRunning bool,
	allTerminal bool,
) map[string]interface{} {
	improvements := make([]string, 0, 4)
	tradeoffs := make([]string, 0, 4)
	recommendation := "hold"
	confidence := "low"
	valueSummary := "Reflective search submitted candidate evaluations and is waiting for follow-up evidence."
	cutoverReady := false

	if anyRunning {
		valueSummary = "Reflective search is still collecting follow-up evidence."
	} else if selected == nil {
		if allTerminal {
			recommendation = "reject"
			confidence = "medium"
			valueSummary = "No reflective candidate produced a gate-backed improvement."
			tradeoffs = append(tradeoffs, "No candidate passed the hard follow-up gate.")
		}
	} else {
		confidence = "medium"
		if selected.Objectives.ExecutionPassRateDelta > 0 {
			improvements = append(improvements, fmt.Sprintf("Execution pass rate improved by %.2f", selected.Objectives.ExecutionPassRateDelta))
		}
		if selected.Objectives.VerificationPassRateDelta > 0 {
			improvements = append(improvements, fmt.Sprintf("Verification pass rate improved by %.2f", selected.Objectives.VerificationPassRateDelta))
		}
		if selected.Objectives.EvidenceBackedPassRateDelta > 0 {
			improvements = append(improvements, fmt.Sprintf("Evidence-backed pass rate improved by %.2f", selected.Objectives.EvidenceBackedPassRateDelta))
		}
		if selected.Objectives.VerificationPassRateDelta < 0 {
			tradeoffs = append(tradeoffs, fmt.Sprintf("Verification pass rate regressed by %.2f", math.Abs(selected.Objectives.VerificationPassRateDelta)))
		}
		if selected.Objectives.EvidenceBackedPassRateDelta < 0 {
			tradeoffs = append(tradeoffs, fmt.Sprintf("Evidence-backed pass rate regressed by %.2f", math.Abs(selected.Objectives.EvidenceBackedPassRateDelta)))
		}
		if selected.Objectives.MedianLatencyIncreaseRate > 0.05 {
			tradeoffs = append(tradeoffs, fmt.Sprintf("Median latency increased by %.0f%%", selected.Objectives.MedianLatencyIncreaseRate*100))
		}
		cutoverReady, blockingReasons := a.reflectiveCutoverReadiness(ctx, record, selected.CandidateID)
		if !cutoverReady {
			tradeoffs = append(tradeoffs, "Cutover readiness is not green yet")
			if len(blockingReasons) > 0 {
				tradeoffs = append(tradeoffs, blockingReasons...)
			}
		}
		switch {
		case selected.Objectives.ExecutionPassRateDelta > 0 && !cutoverReady:
			valueSummary = "Execution improved, but cutover readiness still needs more evidence."
		case selected.Objectives.ExecutionPassRateDelta > 0 && cutoverReady:
			valueSummary = "Execution improved and offline cutover evidence is ready for manual promotion."
		default:
			valueSummary = "Selected candidate passed the hard gate, but more offline evidence is still needed."
		}
	}

	return map[string]interface{}{
		"offline_recommendation":    recommendation,
		"value_summary":             valueSummary,
		"top_improvements":          improvements,
		"top_tradeoffs":             tradeoffs,
		"confidence":                confidence,
		"pareto_frontier":           append([]string(nil), frontierIDs...),
		"candidate_count":           len(candidates),
		"hard_pass_candidate_count": countReflectiveHardPass(candidates),
		"cutover_ready":             cutoverReady,
	}
}

func buildReflectiveSampleEfficiencyReport(
	candidates []reflectiveEvaluatedCandidate,
	frontierIDs []string,
	selected *reflectiveEvaluatedCandidate,
) map[string]interface{} {
	report := map[string]interface{}{
		"evaluated_candidate_count": len(candidates),
		"hard_pass_candidate_count": countReflectiveHardPass(candidates),
		"pareto_frontier_size":      len(frontierIDs),
	}
	if selected != nil {
		report["selected_candidate_id"] = strings.TrimSpace(selected.CandidateID)
		report["selected_execution_delta"] = selected.Objectives.ExecutionPassRateDelta
	}
	return report
}

func countReflectiveHardPass(candidates []reflectiveEvaluatedCandidate) int {
	count := 0
	for _, candidate := range candidates {
		if candidate.HardPass {
			count++
		}
	}
	return count
}

func reflectiveEvaluatedCandidateFromMap(raw map[string]interface{}) reflectiveEvaluatedCandidate {
	candidate := reflectiveEvaluatedCandidate{
		CandidateID:     optimizationMetadataString(raw, "candidate_id"),
		Generation:      optimizationMetadataInt(raw["generation"]),
		HardPass:        optimizationMetadataBool(raw["hard_pass"]),
		FollowupGate:    optimizationMetadataString(raw, "followup_gate"),
		FollowupState:   optimizationMetadataString(raw, "followup_state"),
		FollowupEvalRun: optimizationMetadataString(raw, "followup_eval_run_id"),
		Summary:         optimizationMetadataString(raw, "followup_summary"),
		DiffSize:        optimizationMetadataInt(raw["diff_size"]),
		SkillCandidate:  optimizationMetadataMap(raw, "skill_candidate"),
	}
	if objectives := optimizationMetadataMap(raw, "objectives"); len(objectives) > 0 {
		candidate.Objectives = reflectiveObjectiveVector{
			ExecutionPassRateDelta:      optimizationMetadataFloat(objectives["execution_pass_rate_delta"]),
			VerificationPassRateDelta:   optimizationMetadataFloat(objectives["verification_pass_rate_delta"]),
			EvidenceBackedPassRateDelta: optimizationMetadataFloat(objectives["evidence_backed_pass_rate_delta"]),
			MedianLatencyIncreaseRate:   optimizationMetadataFloat(objectives["median_latency_increase_rate"]),
			RepeatFailureRecurrence:     optimizationMetadataFloat(objectives["repeat_failure_recurrence"]),
		}
	}
	return candidate
}

func reflectiveEvaluatedCandidateToMap(candidate reflectiveEvaluatedCandidate) map[string]interface{} {
	return map[string]interface{}{
		"candidate_id":         strings.TrimSpace(candidate.CandidateID),
		"generation":           candidate.Generation,
		"hard_pass":            candidate.HardPass,
		"followup_gate":        strings.TrimSpace(candidate.FollowupGate),
		"followup_state":       strings.TrimSpace(candidate.FollowupState),
		"followup_eval_run_id": strings.TrimSpace(candidate.FollowupEvalRun),
		"followup_summary":     strings.TrimSpace(candidate.Summary),
		"diff_size":            candidate.DiffSize,
		"objectives":           reflectiveObjectiveVectorToMap(candidate.Objectives),
		"skill_candidate":      cloneOptimizationMetadata(candidate.SkillCandidate),
	}
}

func reflectiveObjectiveVectorToMap(vector reflectiveObjectiveVector) map[string]interface{} {
	return map[string]interface{}{
		"execution_pass_rate_delta":       vector.ExecutionPassRateDelta,
		"verification_pass_rate_delta":    vector.VerificationPassRateDelta,
		"evidence_backed_pass_rate_delta": vector.EvidenceBackedPassRateDelta,
		"median_latency_increase_rate":    vector.MedianLatencyIncreaseRate,
		"repeat_failure_recurrence":       vector.RepeatFailureRecurrence,
	}
}

func reflectiveCandidateByID(candidates []reflectiveEvaluatedCandidate, candidateID string) *reflectiveEvaluatedCandidate {
	candidateID = strings.TrimSpace(candidateID)
	if candidateID == "" {
		return nil
	}
	for i := range candidates {
		if strings.TrimSpace(candidates[i].CandidateID) == candidateID {
			return &candidates[i]
		}
	}
	return nil
}

func optimizationMetadataFloat(raw interface{}) float64 {
	switch typed := raw.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		value, _ := typed.Float64()
		return value
	case string:
		value, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err == nil {
			return value
		}
	}
	return 0
}

func optimizationMetadataInt(raw interface{}) int {
	return int(math.Round(optimizationMetadataFloat(raw)))
}

func optimizationMetadataBool(raw interface{}) bool {
	switch typed := raw.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	default:
		return false
	}
}

func (a *harnessOptimizationManagerAdapter) reflectiveCutoverReadiness(
	ctx context.Context,
	record optimization.OptimizationRunRecord,
	candidateID string,
) (bool, []string) {
	metadata := optimizationMetadataMap(record, "metadata")
	ownerUserID := optimizationMetadataString(metadata, "owner_user_id")
	if strings.TrimSpace(ownerUserID) == "" || strings.TrimSpace(candidateID) == "" || a == nil || a.controller == nil {
		return false, nil
	}
	report, err := a.controller.EvaluateSkillCutoverReadiness(ctx, harness.SkillCutoverReadinessRequest{
		OwnerUserID: ownerUserID,
		CandidateID: strings.TrimSpace(candidateID),
	})
	if err != nil || report == nil {
		return false, nil
	}
	return report.Ready, append([]string(nil), report.BlockingReasons...)
}

func (a *harnessOptimizationManagerAdapter) ensureReflectiveSelectedCandidateRevision(
	ctx context.Context,
	record optimization.OptimizationRunRecord,
	selected reflectiveEvaluatedCandidate,
) (*harness.SkillEvolutionCase, *harness.SkillRevision, error) {
	if a == nil || a.controller == nil || len(selected.SkillCandidate) == 0 {
		return nil, nil, nil
	}
	if revisionID := optimizationMetadataString(record, "skill_revision_id"); revisionID != "" {
		revision, err := a.controller.GetSkillRevision(ctx, revisionID)
		if err != nil {
			return nil, nil, err
		}
		if revision != nil {
			var evolutionCase *harness.SkillEvolutionCase
			if caseID := optimizationMetadataString(record, "skill_evolution_case_id"); caseID != "" {
				evolutionCase, _ = a.controller.GetSkillEvolutionCase(ctx, caseID)
			}
			return evolutionCase, revision, nil
		}
	}

	var parentEvalRun *harness.EvalRun
	var err error
	if evalRunID := optimizationMetadataString(record, "eval_run_id"); evalRunID != "" {
		parentEvalRun, err = a.controller.GetEvalRun(ctx, evalRunID)
		if err != nil {
			return nil, nil, err
		}
	}
	event := reflectiveOptimizationTriggerFromRecord(record, selected)
	sourceState, err := harness.ResolveWritableSkillSourceState(
		optimizationMetadataString(selected.SkillCandidate, "skill_id"),
		optimizationMetadataString(selected.SkillCandidate, "source_path"),
	)
	if err != nil {
		return nil, nil, err
	}

	triggerer := &harnessOptimizationTriggerer{controller: a.controller}
	evolutionCase, created, err := triggerer.ensureOptimizationSkillEvolutionCase(
		ctx,
		optimizationMetadataString(record, "id"),
		event,
		parentEvalRun,
		selected.SkillCandidate,
		sourceState,
	)
	if err != nil {
		return nil, nil, err
	}
	if !created && evolutionCase != nil && strings.TrimSpace(evolutionCase.RevisionID) != "" {
		revision, getErr := a.controller.GetSkillRevision(ctx, evolutionCase.RevisionID)
		return evolutionCase, revision, getErr
	}

	revision, err := a.controller.CreateSkillRevision(ctx, harness.SkillRevision{
		SkillID:             optimizationMetadataString(selected.SkillCandidate, "skill_id"),
		Status:              harness.SkillRevisionStatusAccepted,
		SourcePath:          strings.TrimSpace(sourceState.NormalizedPath),
		CandidateID:         strings.TrimSpace(selected.CandidateID),
		BaseContentSHA256:   strings.TrimSpace(sourceState.ContentSHA256),
		OriginCaseID:        optimizationMetadataStringMapValue(evolutionCase, func(value *harness.SkillEvolutionCase) string { return value.ID }),
		EvalRunID:           strings.TrimSpace(selected.FollowupEvalRun),
		OptimizationRunID:   optimizationMetadataString(record, "id"),
		FollowupGate:        firstNonEmptyOptimizationValue(selected.FollowupGate, optimizationFollowupGate(record)),
		OptimizationSurface: harness.OptimizationSurface(firstNonEmptyOptimizationValue(optimizationMetadataString(record, "optimization_surface"), string(harness.OptimizationSurfaceSkillDefinition))),
		Content:             optimizationMetadataRawString(selected.SkillCandidate, "content"),
	})
	if err != nil {
		return evolutionCase, nil, err
	}
	if evolutionCase != nil {
		evolutionCase.CandidateID = strings.TrimSpace(selected.CandidateID)
		evolutionCase.RevisionID = strings.TrimSpace(revision.ID)
		evolutionCase.Status = harness.SkillEvolutionCaseStatusAccepted
		evolutionCase.UpdatedAt = time.Now().UTC()
		evolutionCase, err = a.controller.UpdateSkillEvolutionCase(ctx, *evolutionCase)
		if err != nil {
			return evolutionCase, revision, err
		}
	}
	return evolutionCase, revision, nil
}

func reflectiveOptimizationTriggerFromRecord(record optimization.OptimizationRunRecord, selected reflectiveEvaluatedCandidate) harness.OptimizationTrigger {
	metadata := cloneOptimizationMetadata(optimizationMetadataMap(record, "metadata"))
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	metadata["skill_candidate"] = cloneOptimizationMetadata(selected.SkillCandidate)
	metadata["candidate_id"] = strings.TrimSpace(selected.CandidateID)
	if gate := firstNonEmptyOptimizationValue(selected.FollowupGate, optimizationFollowupGate(record)); gate != "" {
		metadata["followup_gate"] = gate
	}
	return harness.OptimizationTrigger{
		Reason:              harness.OptimizationReason(optimizationMetadataString(record, "reason")),
		CandidateID:         strings.TrimSpace(selected.CandidateID),
		EvalRunID:           optimizationMetadataString(record, "eval_run_id"),
		BaseEvalRunID:       optimizationMetadataString(record, "base_eval_run_id"),
		OptimizationSurface: harness.OptimizationSurface(firstNonEmptyOptimizationValue(optimizationMetadataString(record, "optimization_surface"), string(harness.OptimizationSurfaceSkillDefinition))),
		Metadata:            metadata,
	}
}

func optimizationMetadataStringMapValue[T any](value *T, getter func(*T) string) string {
	if value == nil || getter == nil {
		return ""
	}
	return strings.TrimSpace(getter(value))
}

func (a *harnessOptimizationManagerAdapter) buildReflectiveRuntimeValueReport(
	ctx context.Context,
	record optimization.OptimizationRunRecord,
	selected *reflectiveEvaluatedCandidate,
) map[string]interface{} {
	if optimizationMetadataString(record, "promotion_state") != "promoted" {
		return runtimeValueReportToMap(defaultRuntimeValueReport("not_started"))
	}
	promotedAt, ok := optimizationRecordTime(record["promoted_at"])
	if !ok {
		if revisionID := optimizationMetadataString(record, "skill_revision_id"); revisionID != "" && a != nil && a.controller != nil {
			revision, err := a.controller.GetSkillRevision(ctx, revisionID)
			if err == nil && revision != nil && revision.PromotedAt != nil {
				promotedAt = revision.PromotedAt.UTC()
				ok = true
			}
		}
	}
	if !ok {
		report := defaultRuntimeValueReport("provisional")
		report.ValueSummary = "Promoted candidate is missing a promotion timestamp, so runtime confirmation is provisional."
		return runtimeValueReportToMap(report)
	}

	metadata := optimizationMetadataMap(record, "metadata")
	ownerUserID := optimizationMetadataString(metadata, "owner_user_id")
	skillID := reflectiveRuntimeSkillID(selected, record)
	if strings.TrimSpace(ownerUserID) == "" || strings.TrimSpace(skillID) == "" {
		report := defaultRuntimeValueReport("provisional")
		report.ValueSummary = "Promoted candidate is missing enough runtime context to evaluate before/after windows."
		return runtimeValueReportToMap(report)
	}

	createdAt, ok := optimizationRecordTime(record["created_at"])
	if !ok {
		createdAt = promotedAt
	}
	failureSignature := firstNonEmptyOptimizationValue(
		optimizationMetadataString(metadata, "failure_signature"),
		optimizationMetadataString(metadata, "runtime_failure_signature"),
		optimizationMetadataString(metadata, "capture_signature"),
	)

	metrics, err := a.collectReflectiveRuntimeWindowMetrics(ctx, ownerUserID, skillID, failureSignature, createdAt, promotedAt)
	if err != nil {
		report := defaultRuntimeValueReport("provisional")
		report.ValueSummary = strings.TrimSpace(err.Error())
		return runtimeValueReportToMap(report)
	}
	return runtimeValueReportToMap(buildRuntimeValueReportFromWindows(metrics))
}

type reflectiveRuntimeObservation struct {
	CreatedAt         time.Time
	FailureMatched    bool
	DurationMs        float64
	TotalTokens       float64
	CaptureQuality    float64
	ValidationQuality float64
}

func (a *harnessOptimizationManagerAdapter) collectReflectiveRuntimeWindowMetrics(
	ctx context.Context,
	ownerUserID string,
	skillID string,
	failureSignature string,
	beforeCutoff time.Time,
	afterCutoff time.Time,
) (runtimeValueWindowMetrics, error) {
	if a == nil || a.controller == nil {
		return runtimeValueWindowMetrics{}, fmt.Errorf("runtime controller is not configured")
	}
	runs, err := a.controller.List(ctx, harness.RunFilter{
		UserID: ownerUserID,
		Statuses: []harness.RunStatus{
			harness.RunStatusCompleted,
			harness.RunStatusFailed,
			harness.RunStatusCancelled,
			harness.RunStatusAborted,
		},
		Limit: reflectiveRuntimeWindowSize * 8,
	})
	if err != nil {
		return runtimeValueWindowMetrics{}, err
	}

	observations := make([]reflectiveRuntimeObservation, 0, len(runs))
	for _, run := range runs {
		if optimizationRuntimeIsOptimizationChild(run.Metadata) {
			continue
		}
		if optimizationRuntimeCanonicalSkillID(&run) != skillID {
			continue
		}
		events, eventsErr := a.controller.ListEvents(ctx, run.ID, 64)
		if eventsErr != nil {
			events = nil
		}
		observations = append(observations, buildReflectiveRuntimeObservation(run, events, failureSignature))
	}

	sort.SliceStable(observations, func(i, j int) bool {
		return observations[i].CreatedAt.Before(observations[j].CreatedAt)
	})
	beforeWindow := make([]reflectiveRuntimeObservation, 0, reflectiveRuntimeWindowSize)
	afterWindow := make([]reflectiveRuntimeObservation, 0, reflectiveRuntimeWindowSize)
	for _, observation := range observations {
		switch {
		case observation.CreatedAt.Before(beforeCutoff):
			beforeWindow = append(beforeWindow, observation)
			if len(beforeWindow) > reflectiveRuntimeWindowSize {
				beforeWindow = beforeWindow[len(beforeWindow)-reflectiveRuntimeWindowSize:]
			}
		case !observation.CreatedAt.Before(afterCutoff):
			if len(afterWindow) < reflectiveRuntimeWindowSize {
				afterWindow = append(afterWindow, observation)
			}
		}
	}

	return runtimeValueWindowMetrics{
		BeforeSampleCount:         len(beforeWindow),
		AfterSampleCount:          len(afterWindow),
		BeforeFailureRecurrence:   reflectiveFailureRecurrence(beforeWindow),
		AfterFailureRecurrence:    reflectiveFailureRecurrence(afterWindow),
		BeforeMedianDurationMs:    reflectiveMedian(beforeWindow, func(item reflectiveRuntimeObservation) float64 { return item.DurationMs }),
		AfterMedianDurationMs:     reflectiveMedian(afterWindow, func(item reflectiveRuntimeObservation) float64 { return item.DurationMs }),
		BeforeMedianTotalTokens:   reflectiveMedian(beforeWindow, func(item reflectiveRuntimeObservation) float64 { return item.TotalTokens }),
		AfterMedianTotalTokens:    reflectiveMedian(afterWindow, func(item reflectiveRuntimeObservation) float64 { return item.TotalTokens }),
		BeforeCaptureQualityScore: reflectiveAverage(beforeWindow, func(item reflectiveRuntimeObservation) float64 { return item.CaptureQuality }),
		AfterCaptureQualityScore:  reflectiveAverage(afterWindow, func(item reflectiveRuntimeObservation) float64 { return item.CaptureQuality }),
		BeforeValidationQuality:   reflectiveAverage(beforeWindow, func(item reflectiveRuntimeObservation) float64 { return item.ValidationQuality }),
		AfterValidationQuality:    reflectiveAverage(afterWindow, func(item reflectiveRuntimeObservation) float64 { return item.ValidationQuality }),
	}, nil
}

func buildReflectiveRuntimeObservation(
	run harness.Run,
	events []harness.RunEvent,
	failureSignature string,
) reflectiveRuntimeObservation {
	observation := reflectiveRuntimeObservation{
		CreatedAt:         run.CreatedAt.UTC(),
		DurationMs:        optimizationRuntimeDurationMs(&run, events),
		TotalTokens:       optimizationRuntimeTotalTokens(&run, events),
		CaptureQuality:    optimizationRuntimeCaptureQualityScore(&run, events),
		ValidationQuality: optimizationRuntimeValidationQualityScore(&run, events),
	}
	if strings.TrimSpace(failureSignature) == "" {
		observation.FailureMatched = run.Status == harness.RunStatusFailed || run.Status == harness.RunStatusAborted
		return observation
	}
	observation.FailureMatched = optimizationRuntimeFailureSignature(&run, events) == strings.TrimSpace(failureSignature)
	return observation
}

func reflectiveFailureRecurrence(items []reflectiveRuntimeObservation) float64 {
	if len(items) == 0 {
		return 0
	}
	failures := 0
	for _, item := range items {
		if item.FailureMatched {
			failures++
		}
	}
	return float64(failures) / float64(len(items))
}

func reflectiveMedian(items []reflectiveRuntimeObservation, selector func(reflectiveRuntimeObservation) float64) float64 {
	if len(items) == 0 || selector == nil {
		return 0
	}
	values := make([]float64, 0, len(items))
	for _, item := range items {
		value := selector(item)
		if value >= 0 {
			values = append(values, value)
		}
	}
	if len(values) == 0 {
		return 0
	}
	sort.Float64s(values)
	mid := len(values) / 2
	if len(values)%2 == 1 {
		return values[mid]
	}
	return (values[mid-1] + values[mid]) / 2
}

func reflectiveAverage(items []reflectiveRuntimeObservation, selector func(reflectiveRuntimeObservation) float64) float64 {
	if len(items) == 0 || selector == nil {
		return 0
	}
	total := 0.0
	count := 0
	for _, item := range items {
		total += selector(item)
		count++
	}
	if count == 0 {
		return 0
	}
	return total / float64(count)
}

func reflectiveRuntimeSkillID(selected *reflectiveEvaluatedCandidate, record optimization.OptimizationRunRecord) string {
	if selected != nil {
		if skillID := optimizationMetadataString(selected.SkillCandidate, "skill_id"); skillID != "" {
			return skillID
		}
	}
	if selectedCandidate := optimizationMetadataMap(record, "selected_candidate"); len(selectedCandidate) > 0 {
		if skillID := optimizationMetadataString(optimizationMetadataMap(selectedCandidate, "skill_candidate"), "skill_id"); skillID != "" {
			return skillID
		}
	}
	metadata := optimizationMetadataMap(record, "metadata")
	return optimizationMetadataString(optimizationMetadataMap(metadata, "skill_candidate"), "skill_id")
}

func optimizationRuntimeIsOptimizationChild(metadata map[string]interface{}) bool {
	if len(metadata) == 0 {
		return false
	}
	return optimizationMetadataBool(metadata["optimization_run"]) || optimizationMetadataString(metadata, "optimization_parent_run_id") != ""
}

func optimizationRuntimeCanonicalSkillID(run *harness.Run) string {
	if run == nil {
		return ""
	}
	meta := run.Metadata
	for _, raw := range []string{
		optimizationMetadataString(meta, "selected_canonical_skill"),
		optimizationMetadataString(meta, "canonical_skill_id"),
		optimizationMetadataString(optimizationMetadataMap(meta, "contextpack_snapshot"), "selected_skill"),
		optimizationMetadataString(meta, "selected_skill"),
		optimizationMetadataString(optimizationMetadataMap(meta, "selector_dry_run_response"), "selected_canonical_skill"),
	} {
		normalized := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
		if normalized != "" {
			return normalized
		}
	}
	return ""
}

func optimizationRuntimeFailureSignature(run *harness.Run, events []harness.RunEvent) string {
	if run == nil {
		return "runtime-failure"
	}
	for i := len(events) - 1; i >= 0; i-- {
		event := events[i]
		if strings.TrimSpace(event.Type) == "run_failed" && strings.TrimSpace(event.Message) != "" {
			return optimizationRuntimeNormalizeSignature("failed:" + event.Message)
		}
	}
	if errText := strings.TrimSpace(run.Error); errText != "" {
		return optimizationRuntimeNormalizeSignature("failed:" + errText)
	}
	if runtimeState := strings.TrimSpace(string(run.RuntimeState)); runtimeState != "" {
		return optimizationRuntimeNormalizeSignature("failed:" + runtimeState)
	}
	return "runtime-failure"
}

func optimizationRuntimeNormalizeSignature(raw string) string {
	parts := strings.Fields(strings.ToLower(strings.TrimSpace(raw)))
	if len(parts) == 0 {
		return ""
	}
	signature := strings.Join(parts, "-")
	if len(signature) > 160 {
		signature = signature[:160]
	}
	return signature
}

func optimizationRuntimeDurationMs(run *harness.Run, events []harness.RunEvent) float64 {
	lifecycle := optimizationRuntimeLifecycleDurationMs(run)
	sources := optimizationRuntimeMetadataSources(run, events)
	signal := optimizationRuntimeMaxNumber(sources, "duration_ms", "latency_ms", "elapsed_ms", "total_duration_ms")
	switch {
	case signal > lifecycle:
		return signal
	case lifecycle > 0:
		return lifecycle
	default:
		return signal
	}
}

func optimizationRuntimeLifecycleDurationMs(run *harness.Run) float64 {
	if run == nil {
		return 0
	}
	if run.StartedAt != nil && run.FinishedAt != nil && !run.FinishedAt.Before(*run.StartedAt) {
		return float64(run.FinishedAt.Sub(*run.StartedAt).Milliseconds())
	}
	if run.StartedAt != nil && !run.UpdatedAt.Before(*run.StartedAt) {
		return float64(run.UpdatedAt.Sub(*run.StartedAt).Milliseconds())
	}
	if !run.UpdatedAt.Before(run.CreatedAt) {
		return float64(run.UpdatedAt.Sub(run.CreatedAt).Milliseconds())
	}
	return 0
}

func optimizationRuntimeTotalTokens(run *harness.Run, events []harness.RunEvent) float64 {
	sources := optimizationRuntimeMetadataSources(run, events)
	if value, ok := optimizationRuntimeFindNumber(sources, "total_tokens"); ok {
		return value
	}
	inputTokens, hasInput := optimizationRuntimeFindNumber(sources, "input_tokens", "prompt_tokens")
	outputTokens, hasOutput := optimizationRuntimeFindNumber(sources, "output_tokens", "completion_tokens")
	if hasInput || hasOutput {
		return inputTokens + outputTokens
	}
	return 0
}

func optimizationRuntimeCaptureQualityScore(run *harness.Run, events []harness.RunEvent) float64 {
	sources := optimizationRuntimeMetadataSources(run, events)
	scores := make([]float64, 0, 4)
	for _, key := range []string{"evidence_score", "outcome_score", "execution_score"} {
		if value, ok := optimizationRuntimeFindNumber(sources, key); ok {
			scores = append(scores, value)
		}
	}
	if value, ok := optimizationRuntimeFindBool(sources, "verification_passed"); ok {
		if value {
			scores = append(scores, 1)
		} else {
			scores = append(scores, 0)
		}
	}
	if len(scores) == 0 {
		return 0
	}
	total := 0.0
	for _, score := range scores {
		total += score
	}
	return total / float64(len(scores))
}

func optimizationRuntimeValidationQualityScore(run *harness.Run, events []harness.RunEvent) float64 {
	sources := optimizationRuntimeMetadataSources(run, events)
	scores := make([]float64, 0, 2)
	if value, ok := optimizationRuntimeFindBool(sources, "recovered"); ok {
		if value {
			scores = append(scores, 1)
		} else {
			scores = append(scores, 0)
		}
	}
	if failures, ok := optimizationRuntimeFindNumber(sources, "failure_count"); ok {
		scores = append(scores, 1/(1+failures))
	}
	if len(scores) == 0 {
		return 0
	}
	total := 0.0
	for _, score := range scores {
		total += score
	}
	return total / float64(len(scores))
}

func optimizationRuntimeMetadataSources(run *harness.Run, events []harness.RunEvent) []map[string]interface{} {
	sources := make([]map[string]interface{}, 0, len(events)+1)
	for i := len(events) - 1; i >= 0; i-- {
		optimizationRuntimeCollectMaps(decodeJSONMap(events[i].PayloadJSON), &sources)
	}
	if run != nil {
		optimizationRuntimeCollectMaps(run.Metadata, &sources)
	}
	return sources
}

func optimizationRuntimeCollectMaps(raw map[string]interface{}, out *[]map[string]interface{}) {
	if len(raw) == 0 || out == nil {
		return
	}
	*out = append(*out, raw)
	for _, value := range raw {
		optimizationRuntimeCollectNestedMaps(value, out)
	}
}

func optimizationRuntimeCollectNestedMaps(raw interface{}, out *[]map[string]interface{}) {
	switch typed := raw.(type) {
	case map[string]interface{}:
		optimizationRuntimeCollectMaps(typed, out)
	case []interface{}:
		for _, item := range typed {
			optimizationRuntimeCollectNestedMaps(item, out)
		}
	}
}

func optimizationRuntimeFindNumber(sources []map[string]interface{}, keys ...string) (float64, bool) {
	for _, source := range sources {
		for _, key := range keys {
			if value, ok := source[key]; ok {
				parsed := optimizationMetadataFloat(value)
				switch value.(type) {
				case float64, float32, int, int64, json.Number:
					return parsed, true
				case string:
					if strings.TrimSpace(value.(string)) != "" {
						return parsed, true
					}
				}
			}
		}
	}
	return 0, false
}

func optimizationRuntimeMaxNumber(sources []map[string]interface{}, keys ...string) float64 {
	max := 0.0
	for _, source := range sources {
		for _, key := range keys {
			value, ok := source[key]
			if !ok {
				continue
			}
			parsed := optimizationMetadataFloat(value)
			switch typed := value.(type) {
			case float64, float32, int, int64, json.Number:
				if parsed > max {
					max = parsed
				}
			case string:
				if strings.TrimSpace(typed) != "" && parsed > max {
					max = parsed
				}
			}
		}
	}
	return max
}

func optimizationRuntimeFindBool(sources []map[string]interface{}, keys ...string) (bool, bool) {
	for _, source := range sources {
		for _, key := range keys {
			value, ok := source[key]
			if !ok {
				continue
			}
			switch typed := value.(type) {
			case bool:
				return typed, true
			case string:
				if strings.EqualFold(strings.TrimSpace(typed), "true") {
					return true, true
				}
				if strings.EqualFold(strings.TrimSpace(typed), "false") {
					return false, true
				}
			}
		}
	}
	return false, false
}

func decodeJSONMap(raw string) map[string]interface{} {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return nil
	}
	return parsed
}
