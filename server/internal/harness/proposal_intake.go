package harness

import (
	"context"
	"fmt"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selfreflect"
)

func (c *Controller) annotateResearchProposalSummary(ctx context.Context, group *RunGroup, run *Run, scorecard Scorecard) Scorecard {
	_, reflector := c.integrations()
	if reflector == nil {
		return annotateProposalSummary(scorecard, 0, nil, "proposal reflector is not configured")
	}
	candidates, skipReason := eligibleResearchProposalCandidates(run, scorecard)
	if len(candidates) == 0 {
		return annotateProposalSummary(scorecard, 0, nil, skipReason)
	}
	sourceID := ""
	if group != nil {
		sourceID = strings.TrimSpace(group.ID)
	}
	if sourceID == "" && run != nil {
		sourceID = strings.TrimSpace(run.ID)
	}
	result, err := reflector.Reflect(ctx, selfreflect.Input{
		OwnerUserID:        runOwnerUserID(run),
		SourceKind:         "harness_group",
		SourceID:           sourceID,
		EvaluationSummary:  buildProposalEvaluationSummary(run, scorecard),
		ProposalCandidates: candidates,
		ProposalMode:       selfreflect.ProposalModeReviewOnly,
	})
	if err != nil {
		return annotateProposalSummary(scorecard, 0, nil, strings.TrimSpace(err.Error()))
	}
	return annotateProposalSummary(scorecard, result.ProposalCount, result.ProposalIDs, result.ProposalSkippedReason)
}

func eligibleResearchProposalCandidates(run *Run, scorecard Scorecard) ([]selfreflect.ProposalCandidate, string) {
	if run == nil || run.Kind != RunKindResearch {
		return nil, "proposal intake only runs for research outputs"
	}
	calibration := runCalibrationSummary(run)
	if len(calibration) == 0 {
		return nil, "research run does not include calibration metadata"
	}
	if confidence := proposalFloat(calibration["confidence"]); confidence < 0.75 {
		return nil, fmt.Sprintf("calibration confidence %.2f is below 0.75", confidence)
	}
	if strings.EqualFold(strings.TrimSpace(fmt.Sprint(calibration["conflict_risk"])), "blocking") {
		return nil, "conflict_risk is blocking"
	}
	if scorecard.Verdict == ScoreVerdictFail {
		return nil, "evaluator verdict is fail"
	}

	rawCandidates := runMetadata(run)["takeaway_candidates"]
	candidates := decodeProposalCandidates(rawCandidates)
	if len(candidates) == 0 {
		return nil, "research run does not include takeaway candidates"
	}
	filtered := make([]selfreflect.ProposalCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.Lesson) == "" || strings.TrimSpace(candidate.Evidence) == "" {
			continue
		}
		if len(candidate.EvidenceIDs) == 0 {
			continue
		}
		target := strings.TrimSpace(candidate.TargetFile)
		if target != "" && target != "AGENTS.md" {
			continue
		}
		if target == "" {
			candidate.TargetFile = "AGENTS.md"
		}
		filtered = append(filtered, candidate)
	}
	if len(filtered) == 0 {
		return nil, "no takeaway candidate met the evidence-traceable proposal gate"
	}
	return filtered, ""
}

func annotateProposalSummary(card Scorecard, count int, ids []string, skippedReason string) Scorecard {
	breakdown := decodeJSONMap(card.BreakdownJSON)
	if breakdown == nil {
		breakdown = map[string]interface{}{}
	}
	breakdown["proposal_count"] = count
	if len(ids) > 0 {
		breakdown["proposal_ids"] = append([]string(nil), ids...)
	}
	if strings.TrimSpace(skippedReason) != "" {
		breakdown["proposal_skipped_reason"] = strings.TrimSpace(skippedReason)
	}
	card.BreakdownJSON = marshalInterface(breakdown)

	trace := decodeJSONMap(card.JudgeTraceJSON)
	if trace == nil {
		trace = map[string]interface{}{}
	}
	trace["proposal_count"] = count
	if len(ids) > 0 {
		trace["proposal_ids"] = append([]string(nil), ids...)
	}
	if strings.TrimSpace(skippedReason) != "" {
		trace["proposal_skipped_reason"] = strings.TrimSpace(skippedReason)
	}
	card.JudgeTraceJSON = marshalInterface(trace)
	return card
}

func buildProposalEvaluationSummary(run *Run, scorecard Scorecard) map[string]interface{} {
	summary := map[string]interface{}{
		"verdict":                scorecard.Verdict,
		"score":                  scorecard.Score,
		"judge_backend":          metadataString(decodeJSONMap(scorecard.JudgeTraceJSON), "judge_backend"),
		"judge_model":            metadataString(decodeJSONMap(scorecard.JudgeTraceJSON), "judge_model"),
		"calibration_ref":        metadataString(runMetadata(run), "calibration_ref"),
		"takeaway_candidate_count": takeawayCandidateCount(run),
	}
	if calibration := runCalibrationSummary(run); len(calibration) > 0 {
		summary["calibration"] = calibration
		for _, key := range []string{"coverage", "groundedness", "freshness", "conflict_risk", "confidence", "recommended_action"} {
			if value, ok := calibration[key]; ok {
				summary[key] = value
			}
		}
	}
	return summary
}

func decodeProposalCandidates(raw interface{}) []selfreflect.ProposalCandidate {
	switch typed := raw.(type) {
	case []selfreflect.ProposalCandidate:
		return append([]selfreflect.ProposalCandidate(nil), typed...)
	case []interface{}:
		out := make([]selfreflect.ProposalCandidate, 0, len(typed))
		for _, item := range typed {
			candidateMap, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			out = append(out, selfreflect.ProposalCandidate{
				Lesson:      strings.TrimSpace(fmt.Sprint(candidateMap["lesson"])),
				WhenToApply: strings.TrimSpace(fmt.Sprint(candidateMap["when_to_apply"])),
				Evidence:    strings.TrimSpace(fmt.Sprint(candidateMap["evidence"])),
				EvidenceIDs: decodeStringSlice(candidateMap["evidence_ids"]),
				Confidence:  proposalFloat(candidateMap["confidence"]),
				TargetFile:  strings.TrimSpace(fmt.Sprint(candidateMap["target_file"])),
			})
		}
		return out
	default:
		return nil
	}
}

func decodeStringSlice(raw interface{}) []string {
	switch typed := raw.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []interface{}:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if value := strings.TrimSpace(fmt.Sprint(item)); value != "" {
				out = append(out, value)
			}
		}
		return out
	default:
		return nil
	}
}

func proposalFloat(raw interface{}) float64 {
	switch value := raw.(type) {
	case float64:
		return value
	case float32:
		return float64(value)
	case int:
		return float64(value)
	case int64:
		return float64(value)
	default:
		return 0
	}
}

func runOwnerUserID(run *Run) string {
	if run == nil {
		return ""
	}
	return strings.TrimSpace(run.UserID)
}
