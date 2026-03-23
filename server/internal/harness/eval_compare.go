package harness

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

type evalCaseSnapshot struct {
	Key           string
	Label         string
	ItemIndex     int
	Profile       string
	Verdict       string
	Status        string
	Score         float64
	RunID         string
	Reason        string
	FailureLabel  string
	Verification  string
	EvidenceScore float64
}

func (c *Controller) CompareEvalRun(ctx context.Context, targetEvalRunID string, req CompareEvalRunRequest) (*ComparisonReport, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}

	targetEvalRun, err := c.GetEvalRun(ctx, strings.TrimSpace(targetEvalRunID))
	if err != nil {
		return nil, err
	}

	baseEvalRun, baseline, err := c.resolveComparisonBase(ctx, targetEvalRun, req)
	if err != nil {
		return nil, err
	}
	if baseEvalRun == nil {
		return nil, fmt.Errorf("comparison base is required")
	}
	if baseEvalRun.ID == targetEvalRun.ID {
		return nil, fmt.Errorf("comparison base must differ from target eval run")
	}
	if baseEvalRun.EvalSpecID != targetEvalRun.EvalSpecID {
		return nil, fmt.Errorf("eval runs must belong to the same eval spec")
	}

	baseReport, err := c.GetEvalRunReport(ctx, baseEvalRun.ID)
	if err != nil {
		return nil, err
	}
	targetReport, err := c.GetEvalRunReport(ctx, targetEvalRun.ID)
	if err != nil {
		return nil, err
	}

	comparison := buildComparisonReport(baseReport, targetReport, baseline)
	comparison.ID = uuid.NewString()
	comparison.OwnerUserID = firstNonEmpty(targetEvalRun.OwnerUserID, baseEvalRun.OwnerUserID)
	comparison.EvalSpecID = targetEvalRun.EvalSpecID
	comparison.BaseEvalRunID = baseEvalRun.ID
	comparison.TargetEvalRunID = targetEvalRun.ID
	if baseline != nil {
		comparison.BaselineID = baseline.ID
	}
	comparison.CreatedAt = timeutil.NowTime()

	if err := c.store.CreateComparisonReport(ctx, comparison); err != nil {
		return nil, err
	}
	return comparison, nil
}

func (c *Controller) resolveComparisonBase(ctx context.Context, targetEvalRun *EvalRun, req CompareEvalRunRequest) (*EvalRun, *Baseline, error) {
	if targetEvalRun == nil {
		return nil, nil, fmt.Errorf("target eval run is required")
	}

	if baselineID := strings.TrimSpace(req.BaselineID); baselineID != "" {
		baseline, err := c.store.GetBaseline(ctx, baselineID)
		if err != nil {
			return nil, nil, err
		}
		baseEvalRun, err := c.GetEvalRun(ctx, baseline.EvalRunID)
		if err != nil {
			return nil, nil, err
		}
		return baseEvalRun, baseline, nil
	}

	baseEvalRunID := strings.TrimSpace(req.BaseEvalRunID)
	if baseEvalRunID == "" {
		baseEvalRunID = strings.TrimSpace(targetEvalRun.BaselineEvalRunID)
	}
	if baseEvalRunID == "" {
		baselines, err := c.store.ListBaselines(ctx, BaselineFilter{
			OwnerUserID: targetEvalRun.OwnerUserID,
			EvalSpecID:  targetEvalRun.EvalSpecID,
			Limit:       100,
		})
		if err != nil {
			return nil, nil, err
		}
		for i := range baselines {
			if baselines[i].IsDefault {
				baseEvalRunID = baselines[i].EvalRunID
				baseline := baselines[i]
				baseEvalRun, err := c.GetEvalRun(ctx, baseline.EvalRunID)
				if err != nil {
					return nil, nil, err
				}
				return baseEvalRun, &baseline, nil
			}
		}
	}
	if baseEvalRunID == "" {
		return nil, nil, fmt.Errorf("no baseline or base eval run selected")
	}

	baseEvalRun, err := c.GetEvalRun(ctx, baseEvalRunID)
	if err != nil {
		return nil, nil, err
	}
	return baseEvalRun, nil, nil
}

func buildComparisonReport(baseReport *EvalRunReport, targetReport *EvalRunReport, baseline *Baseline) *ComparisonReport {
	baseCases := buildEvalCaseSnapshots(baseReport)
	targetCases := buildEvalCaseSnapshots(targetReport)
	keys := sortedCaseKeys(baseCases, targetCases)

	regressions := make([]ComparisonCaseDelta, 0)
	improvements := make([]ComparisonCaseDelta, 0)

	changedCases := 0
	unstableCases := 0
	newFailures := 0
	resolvedFailures := 0

	for _, key := range keys {
		baseCase, baseOK := baseCases[key]
		targetCase, targetOK := targetCases[key]
		if !baseOK && !targetOK {
			continue
		}
		delta, classification, changed := classifyComparisonCase(baseCase, baseOK, targetCase, targetOK)
		if !changed {
			continue
		}
		changedCases++
		switch classification {
		case "regression":
			regressions = append(regressions, delta)
			if isFailingCase(targetCase, targetOK) && !isFailingCase(baseCase, baseOK) {
				newFailures++
			}
		case "improvement":
			improvements = append(improvements, delta)
			if !isFailingCase(targetCase, targetOK) && isFailingCase(baseCase, baseOK) {
				resolvedFailures++
			}
		default:
			unstableCases++
		}
	}

	baseOverallScore := reportOverallScore(baseReport)
	targetOverallScore := reportOverallScore(targetReport)
	basePassRate := reportPassRate(baseReport)
	targetPassRate := reportPassRate(targetReport)
	baseVerificationPassRate := reportVerificationPassRate(baseReport)
	targetVerificationPassRate := reportVerificationPassRate(targetReport)
	baseEvidenceBackedPassRate := reportEvidenceBackedPassRate(baseReport)
	targetEvidenceBackedPassRate := reportEvidenceBackedPassRate(targetReport)
	baseRetryRecoveredCount := reportRetryRecoveredCount(baseReport)
	targetRetryRecoveredCount := reportRetryRecoveredCount(targetReport)
	failureLabelDelta := failureLabelDelta(baseReport, targetReport)

	summary := map[string]interface{}{
		"comparison_kind":                 comparisonKind(baseline),
		"baseline_name":                   baselineName(baseline, baseReport),
		"base_title":                      evalRunTitle(baseReport),
		"target_title":                    evalRunTitle(targetReport),
		"base_group_id":                   reportGroupID(baseReport),
		"target_group_id":                 reportGroupID(targetReport),
		"overall_score_delta":             targetOverallScore - baseOverallScore,
		"pass_rate_delta":                 targetPassRate - basePassRate,
		"verification_pass_rate_delta":    targetVerificationPassRate - baseVerificationPassRate,
		"evidence_backed_pass_rate_delta": targetEvidenceBackedPassRate - baseEvidenceBackedPassRate,
		"retry_recovered_delta":           targetRetryRecoveredCount - baseRetryRecoveredCount,
		"changed_case_count":              changedCases,
		"regression_count":                len(regressions),
		"improvement_count":               len(improvements),
		"unstable_case_count":             unstableCases,
		"new_failure_count":               newFailures,
		"resolved_failure_count":          resolvedFailures,
	}
	if len(failureLabelDelta) > 0 {
		summary["failure_label_delta"] = failureLabelDelta
	}

	return &ComparisonReport{
		BaselineID:   baselineID(baseline),
		Summary:      summary,
		Regressions:  regressions,
		Improvements: improvements,
		ScorerDelta: map[string]interface{}{
			"base_overall_score":               baseOverallScore,
			"target_overall_score":             targetOverallScore,
			"overall_score_delta":              targetOverallScore - baseOverallScore,
			"base_pass_rate":                   basePassRate,
			"target_pass_rate":                 targetPassRate,
			"pass_rate_delta":                  targetPassRate - basePassRate,
			"base_verification_pass_rate":      baseVerificationPassRate,
			"target_verification_pass_rate":    targetVerificationPassRate,
			"verification_pass_rate_delta":     targetVerificationPassRate - baseVerificationPassRate,
			"base_evidence_backed_pass_rate":   baseEvidenceBackedPassRate,
			"target_evidence_backed_pass_rate": targetEvidenceBackedPassRate,
			"evidence_backed_pass_rate_delta":  targetEvidenceBackedPassRate - baseEvidenceBackedPassRate,
			"base_retry_recovered_count":       baseRetryRecoveredCount,
			"target_retry_recovered_count":     targetRetryRecoveredCount,
			"retry_recovered_delta":            targetRetryRecoveredCount - baseRetryRecoveredCount,
			"failure_label_delta":              failureLabelDelta,
			"verdict_count_delta":              verdictCountDelta(baseReport, targetReport),
			"linked_run_count_delta":           reportLinkedRunCount(targetReport) - reportLinkedRunCount(baseReport),
			"artifact_count_delta":             reportArtifactCount(targetReport) - reportArtifactCount(baseReport),
		},
	}
}

func buildEvalCaseSnapshots(report *EvalRunReport) map[string]evalCaseSnapshot {
	out := make(map[string]evalCaseSnapshot)
	if report == nil || report.GroupReport == nil {
		return out
	}

	runByID := make(map[string]Run, len(report.GroupReport.LinkedRuns))
	for _, run := range report.GroupReport.LinkedRuns {
		runByID[run.ID] = run
	}
	latestCards := latestScorecardsByItem(report.GroupReport.Scorecards)

	for _, item := range report.GroupReport.Items {
		key := metadataString(item.Metadata, "dataset_case_id")
		if key == "" {
			key = fmt.Sprintf("item-%d", item.Index)
		}
		label := firstNonEmpty(metadataString(item.Input, "goal"), key)
		snapshot := evalCaseSnapshot{
			Key:       key,
			Label:     label,
			ItemIndex: item.Index,
			Profile:   item.Profile,
			Status:    string(item.Status),
			RunID:     strings.TrimSpace(item.LatestRunID),
		}
		if card, ok := latestCards[item.ID]; ok {
			breakdown := decodeJSONMap(card.BreakdownJSON)
			snapshot.Verdict = string(card.Verdict)
			snapshot.Score = card.Score
			snapshot.FailureLabel = metadataString(breakdown, "failure_label")
			snapshot.Verification = comparisonVerificationStatus(breakdown)
			snapshot.EvidenceScore = comparisonNumericValue(breakdown["evidence_score"])
			if runID := strings.TrimSpace(card.RunID); runID != "" {
				snapshot.RunID = runID
			}
			snapshot.Reason = comparisonReason(&card, runByID[snapshot.RunID])
		}
		if snapshot.Verdict == "" {
			snapshot.Verdict = verdictFromItemStatus(item.Status)
		}
		if snapshot.Reason == "" {
			snapshot.Reason = comparisonReason(nil, runByID[snapshot.RunID])
		}
		out[key] = snapshot
	}
	return out
}

func sortedCaseKeys(groups ...map[string]evalCaseSnapshot) []string {
	seen := make(map[string]struct{})
	keys := make([]string, 0)
	for _, group := range groups {
		for key := range group {
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func classifyComparisonCase(base evalCaseSnapshot, hasBase bool, target evalCaseSnapshot, hasTarget bool) (ComparisonCaseDelta, string, bool) {
	delta := ComparisonCaseDelta{
		Key:                 firstNonEmpty(target.Key, base.Key),
		Label:               firstNonEmpty(target.Label, base.Label, target.Key, base.Key),
		ItemIndex:           firstNonZero(target.ItemIndex, base.ItemIndex),
		Profile:             firstNonEmpty(target.Profile, base.Profile),
		BaseVerdict:         normalizedVerdict(base, hasBase),
		TargetVerdict:       normalizedVerdict(target, hasTarget),
		BaseStatus:          normalizedStatus(base, hasBase),
		TargetStatus:        normalizedStatus(target, hasTarget),
		BaseScore:           base.Score,
		TargetScore:         target.Score,
		DeltaScore:          target.Score - base.Score,
		BaseRunID:           strings.TrimSpace(base.RunID),
		TargetRunID:         strings.TrimSpace(target.RunID),
		BaseReason:          strings.TrimSpace(base.Reason),
		TargetReason:        strings.TrimSpace(target.Reason),
		BaseFailureLabel:    strings.TrimSpace(base.FailureLabel),
		TargetFailureLabel:  strings.TrimSpace(target.FailureLabel),
		BaseVerification:    normalizedVerification(base, hasBase),
		TargetVerification:  normalizedVerification(target, hasTarget),
		BaseEvidenceScore:   base.EvidenceScore,
		TargetEvidenceScore: target.EvidenceScore,
	}

	if !hasBase || !hasTarget {
		switch {
		case !hasBase && hasTarget && isFailingCase(target, true):
			return delta, "regression", true
		case hasBase && !hasTarget && isFailingCase(base, true):
			return delta, "improvement", true
		default:
			return delta, "unstable", true
		}
	}

	baseRank := caseRank(base, hasBase)
	targetRank := caseRank(target, hasTarget)
	sameVerdict := delta.BaseVerdict == delta.TargetVerdict
	sameStatus := delta.BaseStatus == delta.TargetStatus
	scoreChanged := math.Abs(delta.DeltaScore) >= 0.0001
	evidenceChanged := math.Abs(delta.TargetEvidenceScore-delta.BaseEvidenceScore) >= 0.0001
	sameVerification := delta.BaseVerification == delta.TargetVerification
	sameFailureLabel := delta.BaseFailureLabel == delta.TargetFailureLabel

	if baseRank == targetRank && sameVerdict && sameStatus && !scoreChanged && !evidenceChanged && sameVerification && sameFailureLabel {
		return delta, "", false
	}
	if targetRank < baseRank {
		return delta, "regression", true
	}
	if targetRank > baseRank {
		return delta, "improvement", true
	}
	return delta, "unstable", true
}

func normalizedVerdict(snapshot evalCaseSnapshot, ok bool) string {
	if !ok {
		return "missing"
	}
	if verdict := strings.TrimSpace(snapshot.Verdict); verdict != "" {
		return verdict
	}
	return verdictFromStatus(snapshot.Status)
}

func normalizedStatus(snapshot evalCaseSnapshot, ok bool) string {
	if !ok {
		return "missing"
	}
	if status := strings.TrimSpace(snapshot.Status); status != "" {
		return status
	}
	return "unknown"
}

func normalizedVerification(snapshot evalCaseSnapshot, ok bool) string {
	if !ok {
		return "missing"
	}
	if status := strings.TrimSpace(snapshot.Verification); status != "" {
		return status
	}
	return "unknown"
}

func caseRank(snapshot evalCaseSnapshot, ok bool) int {
	switch normalizedVerdict(snapshot, ok) {
	case "pass":
		return 4
	case "partial":
		return 3
	case "fail":
		return 2
	case "error":
		return 1
	case "missing":
		return 0
	default:
		return 0
	}
}

func isFailingCase(snapshot evalCaseSnapshot, ok bool) bool {
	switch normalizedVerdict(snapshot, ok) {
	case "fail", "error", "partial":
		return true
	default:
		return false
	}
}

func verdictFromItemStatus(status RunGroupItemStatus) string {
	return verdictFromStatus(string(status))
}

func verdictFromStatus(status string) string {
	switch strings.TrimSpace(status) {
	case "passed", "completed", "pass":
		return "pass"
	case "partial":
		return "partial"
	case "failed", "fail":
		return "fail"
	case "error", "cancelled", "aborted":
		return "error"
	default:
		return ""
	}
}

func comparisonReason(card *Scorecard, run Run) string {
	if card != nil {
		breakdown := decodeJSONMap(card.BreakdownJSON)
		if reason := metadataString(breakdown, "reason"); reason != "" {
			return reason
		}
		trace := decodeJSONMap(card.JudgeTraceJSON)
		if reason := metadataString(trace, "reason"); reason != "" {
			return reason
		}
	}
	if msg := strings.TrimSpace(run.Error); msg != "" {
		return msg
	}
	if msg := strings.TrimSpace(run.Result); msg != "" {
		if len(msg) > 160 {
			return msg[:157] + "..."
		}
		return msg
	}
	return ""
}

func reportOverallScore(report *EvalRunReport) float64 {
	if report == nil || report.GroupReport == nil {
		return 0
	}
	return report.GroupReport.OverallScore
}

func reportPassRate(report *EvalRunReport) float64 {
	if report == nil || report.GroupReport == nil {
		return 0
	}
	return report.GroupReport.PassRate
}

func reportVerificationPassRate(report *EvalRunReport) float64 {
	if value, ok := reportSummaryFloat(report, "verification_pass_rate"); ok {
		return value
	}
	if report == nil || report.GroupReport == nil {
		return 0
	}
	latestCards := latestScorecardsByItem(report.GroupReport.Scorecards)
	if len(latestCards) == 0 {
		return 0
	}
	passed := 0
	for _, item := range report.GroupReport.Items {
		card, ok := latestCards[item.ID]
		if !ok {
			continue
		}
		if verified, ok := mapBool(decodeJSONMap(card.BreakdownJSON), "verification_passed"); ok && verified {
			passed++
		}
	}
	return float64(passed) / float64(len(latestCards))
}

func reportEvidenceBackedPassRate(report *EvalRunReport) float64 {
	if value, ok := reportSummaryFloat(report, "evidence_backed_pass_rate"); ok {
		return value
	}
	if report == nil || report.GroupReport == nil {
		return 0
	}
	latestCards := latestScorecardsByItem(report.GroupReport.Scorecards)
	passCount := 0
	evidenceBacked := 0
	for _, item := range report.GroupReport.Items {
		card, ok := latestCards[item.ID]
		if !ok || card.Verdict != ScoreVerdictPass {
			continue
		}
		passCount++
		breakdown := decodeJSONMap(card.BreakdownJSON)
		if score := comparisonNumericValue(breakdown["evidence_score"]); score >= 0.8 {
			evidenceBacked++
			continue
		}
		if verified, ok := mapBool(breakdown, "verification_passed"); ok && verified {
			evidenceBacked++
		}
	}
	if passCount == 0 {
		return 0
	}
	return float64(evidenceBacked) / float64(passCount)
}

func reportRetryRecoveredCount(report *EvalRunReport) int {
	if value, ok := reportSummaryInt(report, "retry_recovered_count"); ok {
		return value
	}
	if report == nil || report.GroupReport == nil {
		return 0
	}
	latestCards := latestScorecardsByItem(report.GroupReport.Scorecards)
	recovered := 0
	for _, item := range report.GroupReport.Items {
		card, ok := latestCards[item.ID]
		if !ok || card.Verdict != ScoreVerdictPass {
			continue
		}
		if item.AttemptCount > 1 {
			recovered++
		}
	}
	return recovered
}

func reportGroupID(report *EvalRunReport) string {
	if report == nil || report.GroupReport == nil || report.GroupReport.Group == nil {
		return ""
	}
	return report.GroupReport.Group.ID
}

func reportLinkedRunCount(report *EvalRunReport) int {
	if report == nil || report.GroupReport == nil {
		return 0
	}
	return len(report.GroupReport.LinkedRuns)
}

func reportArtifactCount(report *EvalRunReport) int {
	if report == nil || report.GroupReport == nil {
		return 0
	}
	return len(report.GroupReport.Artifacts)
}

func failureLabelDelta(baseReport *EvalRunReport, targetReport *EvalRunReport) map[string]interface{} {
	baseCounts := reportFailureLabelCounts(baseReport)
	targetCounts := reportFailureLabelCounts(targetReport)
	keys := make(map[string]struct{}, len(baseCounts)+len(targetCounts))
	for key := range baseCounts {
		keys[key] = struct{}{}
	}
	for key := range targetCounts {
		keys[key] = struct{}{}
	}
	out := make(map[string]interface{}, len(keys))
	for key := range keys {
		out[key] = targetCounts[key] - baseCounts[key]
	}
	return out
}

func verdictCountDelta(baseReport *EvalRunReport, targetReport *EvalRunReport) map[string]interface{} {
	keys := make(map[string]struct{})
	out := make(map[string]interface{})
	if baseReport != nil && baseReport.GroupReport != nil {
		for key := range baseReport.GroupReport.VerdictCounts {
			keys[key] = struct{}{}
		}
	}
	if targetReport != nil && targetReport.GroupReport != nil {
		for key := range targetReport.GroupReport.VerdictCounts {
			keys[key] = struct{}{}
		}
	}
	for key := range keys {
		baseCount := 0
		targetCount := 0
		if baseReport != nil && baseReport.GroupReport != nil {
			baseCount = baseReport.GroupReport.VerdictCounts[key]
		}
		if targetReport != nil && targetReport.GroupReport != nil {
			targetCount = targetReport.GroupReport.VerdictCounts[key]
		}
		out[key] = targetCount - baseCount
	}
	return out
}

func comparisonKind(baseline *Baseline) string {
	if baseline != nil {
		return "baseline"
	}
	return "eval_run"
}

func baselineName(baseline *Baseline, baseReport *EvalRunReport) string {
	if baseline != nil && strings.TrimSpace(baseline.Name) != "" {
		return baseline.Name
	}
	return evalRunTitle(baseReport)
}

func baselineID(baseline *Baseline) string {
	if baseline == nil {
		return ""
	}
	return baseline.ID
}

func evalRunTitle(report *EvalRunReport) string {
	if report == nil || report.EvalRun == nil {
		return ""
	}
	return firstNonEmpty(report.EvalRun.Title, report.EvalRun.ID)
}

func firstNonZero(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func reportFailureLabelCounts(report *EvalRunReport) map[string]int {
	if value, ok := reportSummaryIntMap(report, "failure_label_counts"); ok {
		return value
	}
	out := map[string]int{}
	if report == nil || report.GroupReport == nil {
		return out
	}
	latestCards := latestScorecardsByItem(report.GroupReport.Scorecards)
	for _, item := range report.GroupReport.Items {
		card, ok := latestCards[item.ID]
		if !ok {
			continue
		}
		if label := metadataString(decodeJSONMap(card.BreakdownJSON), "failure_label"); label != "" {
			out[label]++
		}
	}
	return out
}

func reportSummaryFloat(report *EvalRunReport, key string) (float64, bool) {
	if report == nil || report.GroupReport == nil || report.GroupReport.Group == nil {
		return 0, false
	}
	raw, ok := report.GroupReport.Group.Summary[key]
	if !ok {
		return 0, false
	}
	value := comparisonNumericValue(raw)
	return value, true
}

func reportSummaryInt(report *EvalRunReport, key string) (int, bool) {
	if report == nil || report.GroupReport == nil || report.GroupReport.Group == nil {
		return 0, false
	}
	raw, ok := report.GroupReport.Group.Summary[key]
	if !ok {
		return 0, false
	}
	return int(comparisonNumericValue(raw)), true
}

func reportSummaryIntMap(report *EvalRunReport, key string) (map[string]int, bool) {
	if report == nil || report.GroupReport == nil || report.GroupReport.Group == nil {
		return nil, false
	}
	raw, ok := report.GroupReport.Group.Summary[key]
	if !ok {
		return nil, false
	}
	typed, ok := raw.(map[string]interface{})
	if !ok {
		return nil, false
	}
	out := make(map[string]int, len(typed))
	for label, value := range typed {
		out[label] = int(comparisonNumericValue(value))
	}
	return out, true
}

func comparisonNumericValue(raw interface{}) float64 {
	switch typed := raw.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int32:
		return float64(typed)
	case int64:
		return float64(typed)
	default:
		return 0
	}
}

func comparisonVerificationStatus(breakdown map[string]interface{}) string {
	if len(breakdown) == 0 {
		return ""
	}
	if passed, ok := mapBool(breakdown, "verification_passed"); ok {
		if passed {
			return "passed"
		}
		return "failed"
	}
	return ""
}
