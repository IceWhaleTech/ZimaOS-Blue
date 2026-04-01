package harness

import (
	"context"
	"fmt"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

const (
	defaultSelectorGateMinPassRate         = 0.98
	defaultSelectorGateMinCriticalPassRate = 1.0
)

type normalizedSelectorGateThresholds struct {
	minPassRate                   float64
	minCriticalPassRate           float64
	minRouteAgreementRate         float64
	hasMinRouteAgreementRate      bool
	minRouteCompatibleRate        float64
	hasMinRouteCompatibleRate     bool
	maxClarifyRateDelta           float64
	hasMaxClarifyRateDelta        bool
	maxCriticalRegressionCount    int
	hasMaxCriticalRegressionCount bool
}

func DefaultSelectorGateThresholds() SelectorGateThresholds {
	minPassRate := defaultSelectorGateMinPassRate
	minCriticalPassRate := defaultSelectorGateMinCriticalPassRate
	return SelectorGateThresholds{
		MinPassRate:         &minPassRate,
		MinCriticalPassRate: &minCriticalPassRate,
	}
}

func (c *Controller) EnsureSelectorCuratedAssets(ctx context.Context, ownerUserID string) (*SelectorCuratedAssets, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	if strings.TrimSpace(ownerUserID) == "" {
		return nil, fmt.Errorf("owner_user_id is required")
	}

	dataset, err := c.ensureSelectorCuratedDataset(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	version, err := c.ensureSelectorCuratedDatasetVersion(ctx, dataset, ownerUserID)
	if err != nil {
		return nil, err
	}
	evalSpec, err := c.ensureSelectorCuratedEvalSpec(ctx, dataset, version, ownerUserID)
	if err != nil {
		return nil, err
	}
	return &SelectorCuratedAssets{
		Dataset:        dataset,
		DatasetVersion: version,
		EvalSpec:       evalSpec,
	}, nil
}

func (c *Controller) EvaluateSelectorGate(ctx context.Context, targetEvalRunID string, req SelectorGateRequest) (*SelectorGateReport, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	targetReport, err := c.GetEvalRunReport(ctx, strings.TrimSpace(targetEvalRunID))
	if err != nil {
		return nil, err
	}
	if targetReport == nil || targetReport.EvalRun == nil || targetReport.GroupReport == nil {
		return nil, fmt.Errorf("target eval run report is incomplete")
	}

	comparison, err := c.CompareEvalRun(ctx, targetReport.EvalRun.ID, CompareEvalRunRequest{
		BaseEvalRunID: strings.TrimSpace(req.BaseEvalRunID),
		BaselineID:    strings.TrimSpace(req.BaselineID),
	})
	if err != nil {
		return nil, err
	}

	thresholds := normalizeSelectorGateThresholds(req.Thresholds)
	metrics := buildSelectorGateMetrics(targetReport)
	applySelectorGateComparisonMetrics(&metrics, comparison)
	checks := evaluateSelectorGateChecks(metrics, thresholds)

	report := &SelectorGateReport{
		TargetEvalRunID:    targetReport.EvalRun.ID,
		BaseEvalRunID:      comparison.BaseEvalRunID,
		BaselineID:         comparison.BaselineID,
		ComparisonReportID: comparison.ID,
		Metrics:            metrics,
		Thresholds:         selectorGateThresholdMap(thresholds),
		Checks:             checks,
		Passed:             selectorGateChecksPassed(checks),
		CreatedAt:          timeutil.NowTime(),
	}
	return report, nil
}

func (c *Controller) ensureSelectorCuratedDataset(ctx context.Context, ownerUserID string) (*Dataset, error) {
	datasets, err := c.ListDatasets(ctx, DatasetFilter{
		OwnerUserID: strings.TrimSpace(ownerUserID),
		Limit:       200,
	})
	if err != nil {
		return nil, err
	}
	for i := range datasets {
		if selectorCuratedDatasetMatches(&datasets[i]) {
			return &datasets[i], nil
		}
	}
	return c.CreateDataset(ctx, SelectorCuratedDatasetSpec(ownerUserID))
}

func (c *Controller) ensureSelectorCuratedDatasetVersion(ctx context.Context, dataset *Dataset, createdBy string) (*DatasetVersion, error) {
	if dataset == nil {
		return nil, fmt.Errorf("dataset is required")
	}
	versionSpec, err := SelectorCuratedDatasetVersionSpec(createdBy)
	if err != nil {
		return nil, err
	}
	return c.ensureBuiltinDatasetVersion(ctx, dataset, versionSpec)
}

func (c *Controller) ensureSelectorCuratedEvalSpec(ctx context.Context, dataset *Dataset, version *DatasetVersion, ownerUserID string) (*EvalSpec, error) {
	if dataset == nil || version == nil {
		return nil, fmt.Errorf("dataset and version are required")
	}
	specs, err := c.ListEvalSpecs(ctx, EvalSpecFilter{
		OwnerUserID: strings.TrimSpace(ownerUserID),
		DatasetID:   dataset.ID,
		Limit:       200,
	})
	if err != nil {
		return nil, err
	}
	for i := range specs {
		if selectorCuratedEvalSpecMatches(&specs[i], version.ID) {
			return &specs[i], nil
		}
	}
	return c.CreateEvalSpec(ctx, SelectorCuratedEvalSpecSpec(dataset.ID, version.ID, ownerUserID))
}

func selectorCuratedDatasetMatches(dataset *Dataset) bool {
	if dataset == nil {
		return false
	}
	return strings.TrimSpace(dataset.Name) == SelectorCuratedDatasetName &&
		strings.TrimSpace(dataset.Subject) == SelectorCuratedDatasetSubject &&
		metadataString(dataset.Metadata, "dataset_family") == "builtin_selector_curated"
}

func selectorCuratedEvalSpecMatches(spec *EvalSpec, versionID string) bool {
	if spec == nil {
		return false
	}
	return strings.TrimSpace(spec.Name) == SelectorCuratedEvalName &&
		strings.TrimSpace(spec.Subject) == SelectorCuratedDatasetSubject &&
		strings.TrimSpace(spec.Profile) == selectorCuratedProfile &&
		strings.TrimSpace(spec.DatasetVersionID) == strings.TrimSpace(versionID) &&
		metadataString(spec.Metadata, "gate_type") == "selection"
}

func normalizeSelectorGateThresholds(raw SelectorGateThresholds) normalizedSelectorGateThresholds {
	normalized := normalizedSelectorGateThresholds{
		minPassRate:         defaultSelectorGateMinPassRate,
		minCriticalPassRate: defaultSelectorGateMinCriticalPassRate,
	}
	if raw.MinPassRate != nil {
		normalized.minPassRate = *raw.MinPassRate
	}
	if raw.MinCriticalPassRate != nil {
		normalized.minCriticalPassRate = *raw.MinCriticalPassRate
	}
	if raw.MinRouteAgreementRate != nil {
		normalized.minRouteAgreementRate = *raw.MinRouteAgreementRate
		normalized.hasMinRouteAgreementRate = true
	}
	if raw.MinRouteCompatibleRate != nil {
		normalized.minRouteCompatibleRate = *raw.MinRouteCompatibleRate
		normalized.hasMinRouteCompatibleRate = true
	}
	if raw.MaxClarifyRateDelta != nil {
		normalized.maxClarifyRateDelta = *raw.MaxClarifyRateDelta
		normalized.hasMaxClarifyRateDelta = true
	}
	if raw.MaxCriticalRegressionCount != nil {
		normalized.maxCriticalRegressionCount = *raw.MaxCriticalRegressionCount
		normalized.hasMaxCriticalRegressionCount = true
	}
	return normalized
}

func selectorGateThresholdMap(thresholds normalizedSelectorGateThresholds) map[string]interface{} {
	out := map[string]interface{}{
		"min_pass_rate":          thresholds.minPassRate,
		"min_critical_pass_rate": thresholds.minCriticalPassRate,
	}
	if thresholds.hasMinRouteAgreementRate {
		out["min_route_agreement_rate"] = thresholds.minRouteAgreementRate
	}
	if thresholds.hasMinRouteCompatibleRate {
		out["min_route_compatible_rate"] = thresholds.minRouteCompatibleRate
	}
	if thresholds.hasMaxClarifyRateDelta {
		out["max_clarify_rate_delta"] = thresholds.maxClarifyRateDelta
	}
	if thresholds.hasMaxCriticalRegressionCount {
		out["max_critical_regression_count"] = thresholds.maxCriticalRegressionCount
	}
	return out
}

func buildSelectorGateMetrics(report *EvalRunReport) SelectorGateMetrics {
	metrics := SelectorGateMetrics{}
	if report == nil || report.GroupReport == nil {
		return metrics
	}

	runByID := make(map[string]Run, len(report.GroupReport.LinkedRuns))
	for _, run := range report.GroupReport.LinkedRuns {
		runByID[run.ID] = run
	}
	latestCards := latestScorecardsByItem(report.GroupReport.Scorecards)

	considered := 0
	passed := 0
	criticalTotal := 0
	criticalPassed := 0
	clarifyCount := 0

	for _, item := range report.GroupReport.Items {
		if len(decodeStringSlice(item.Metadata["compare_fields"])) == 0 {
			continue
		}
		considered++
		itemPassed := selectorGateItemPassed(item, latestCards[item.ID])
		if itemPassed {
			passed++
		}
		if metadataBoolValue(item.Metadata, "critical") {
			criticalTotal++
			if itemPassed {
				criticalPassed++
			}
		}

		runID := strings.TrimSpace(item.LatestRunID)
		if card, ok := latestCards[item.ID]; ok && strings.TrimSpace(card.RunID) != "" {
			runID = strings.TrimSpace(card.RunID)
		}
		if run, ok := runByID[runID]; ok {
			structured := structuredRunResult(&run)
			if comparisonStructuredBool(structured, "skill_need_clarify") {
				clarifyCount++
			}
			observation := decodeDiscoverFirstObservation(structured)
			incrementBreakdownValue(&metrics.SelectedCanonicalSkillBreakdown, observation.SelectedCanonicalSkill)
			incrementBreakdownValue(&metrics.NativeSurfaceModeBreakdown, observation.NativeSurfaceMode)
			incrementBreakdownValue(&metrics.NativeSurfaceReasonBreakdown, observation.NativeSurfaceReason)
			incrementBreakdownValue(&metrics.ExecutionProfileBreakdown, observation.ExecutionProfile)
		}
	}

	if considered == 0 {
		considered = len(report.GroupReport.Items)
		for _, item := range report.GroupReport.Items {
			itemPassed := selectorGateItemPassed(item, latestCards[item.ID])
			if itemPassed {
				passed++
			}
			if metadataBoolValue(item.Metadata, "critical") {
				criticalTotal++
				if itemPassed {
					criticalPassed++
				}
			}
			runID := strings.TrimSpace(item.LatestRunID)
			if card, ok := latestCards[item.ID]; ok && strings.TrimSpace(card.RunID) != "" {
				runID = strings.TrimSpace(card.RunID)
			}
			if run, ok := runByID[runID]; ok {
				structured := structuredRunResult(&run)
				observation := decodeDiscoverFirstObservation(structured)
				incrementBreakdownValue(&metrics.SelectedCanonicalSkillBreakdown, observation.SelectedCanonicalSkill)
				incrementBreakdownValue(&metrics.NativeSurfaceModeBreakdown, observation.NativeSurfaceMode)
				incrementBreakdownValue(&metrics.NativeSurfaceReasonBreakdown, observation.NativeSurfaceReason)
				incrementBreakdownValue(&metrics.ExecutionProfileBreakdown, observation.ExecutionProfile)
			}
		}
	}

	metrics.CaseCount = considered
	metrics.PassedCount = passed
	metrics.PassRate = safeComparisonRate(passed, considered)
	metrics.CriticalCaseCount = criticalTotal
	metrics.CriticalPassedCount = criticalPassed
	if criticalTotal > 0 {
		metrics.CriticalPassRate = safeComparisonRate(criticalPassed, criticalTotal)
	}
	metrics.TargetClarifyRate = safeComparisonRate(clarifyCount, considered)
	return metrics
}

func selectorGateItemPassed(item RunGroupItem, card Scorecard) bool {
	if strings.TrimSpace(string(card.Verdict)) != "" {
		return card.Verdict == ScoreVerdictPass
	}
	return item.Status == RunGroupItemStatusPassed
}

func applySelectorGateComparisonMetrics(metrics *SelectorGateMetrics, comparison *ComparisonReport) {
	if metrics == nil || comparison == nil {
		return
	}
	metrics.RouteCaseCount = selectorGateIntValue(comparison.Summary["route_case_count"], metrics.CaseCount)
	metrics.RouteAgreementCount = selectorGateIntValue(comparison.Summary["route_agreement_count"], 0)
	metrics.RouteAgreementRate = selectorGateFloatValue(comparison.Summary["route_agreement_rate"], 0)
	metrics.RouteCompatibleCount = selectorGateIntValue(comparison.Summary["route_compatible_count"], metrics.RouteAgreementCount)
	metrics.RouteCompatibleRate = selectorGateFloatValue(comparison.Summary["route_compatible_rate"], safeComparisonRate(metrics.RouteCompatibleCount, metrics.RouteCaseCount))
	metrics.RouteImprovementCount = selectorGateIntValue(comparison.Summary["route_improvement_count"], 0)
	metrics.RouteDisagreementCount = selectorGateIntValue(comparison.Summary["route_disagreement_count"], 0)
	metrics.CriticalRegressionCount = selectorGateIntValue(comparison.Summary["critical_regression_count"], 0)
	metrics.BaseClarifyRate = selectorGateFloatValue(comparison.Summary["base_clarify_rate"], 0)
	metrics.TargetClarifyRate = selectorGateFloatValue(comparison.Summary["target_clarify_rate"], metrics.TargetClarifyRate)
	metrics.ClarifyRateDelta = selectorGateFloatValue(comparison.Summary["clarify_rate_delta"], metrics.TargetClarifyRate-metrics.BaseClarifyRate)
	metrics.LocaleBreakdown = selectorGateSegmentMetricsMapValue(comparison.Summary["locale_breakdown"])
	metrics.PrimaryRouteBreakdown = selectorGateSegmentMetricsMapValue(comparison.Summary["primary_route_breakdown"])
}

func evaluateSelectorGateChecks(metrics SelectorGateMetrics, thresholds normalizedSelectorGateThresholds) []SelectorGateCheck {
	checks := []SelectorGateCheck{
		{
			Name:     "pass_rate",
			Passed:   metrics.PassRate >= thresholds.minPassRate,
			Actual:   metrics.PassRate,
			Expected: thresholds.minPassRate,
		},
		{
			Name:     "critical_pass_rate",
			Passed:   metrics.CriticalCaseCount == 0 || metrics.CriticalPassRate >= thresholds.minCriticalPassRate,
			Actual:   metrics.CriticalPassRate,
			Expected: thresholds.minCriticalPassRate,
			Details: map[string]interface{}{
				"critical_case_count":   metrics.CriticalCaseCount,
				"critical_passed_count": metrics.CriticalPassedCount,
			},
		},
	}
	if thresholds.hasMinRouteAgreementRate {
		check := SelectorGateCheck{
			Name:     "route_agreement_rate",
			Passed:   metrics.RouteCaseCount > 0 && metrics.RouteAgreementRate >= thresholds.minRouteAgreementRate,
			Actual:   metrics.RouteAgreementRate,
			Expected: thresholds.minRouteAgreementRate,
			Details: map[string]interface{}{
				"route_case_count":      metrics.RouteCaseCount,
				"route_agreement_count": metrics.RouteAgreementCount,
			},
		}
		checks = append(checks, check)
	}
	if thresholds.hasMinRouteCompatibleRate {
		check := SelectorGateCheck{
			Name:     "route_compatible_rate",
			Passed:   metrics.RouteCaseCount > 0 && metrics.RouteCompatibleRate >= thresholds.minRouteCompatibleRate,
			Actual:   metrics.RouteCompatibleRate,
			Expected: thresholds.minRouteCompatibleRate,
			Details: map[string]interface{}{
				"route_case_count":        metrics.RouteCaseCount,
				"route_compatible_count":  metrics.RouteCompatibleCount,
				"route_improvement_count": metrics.RouteImprovementCount,
			},
		}
		checks = append(checks, check)
	}
	if thresholds.hasMaxClarifyRateDelta {
		check := SelectorGateCheck{
			Name:     "clarify_rate_delta",
			Passed:   metrics.ClarifyRateDelta <= thresholds.maxClarifyRateDelta,
			Actual:   metrics.ClarifyRateDelta,
			Expected: thresholds.maxClarifyRateDelta,
		}
		checks = append(checks, check)
	}
	if thresholds.hasMaxCriticalRegressionCount {
		check := SelectorGateCheck{
			Name:     "critical_regression_count",
			Passed:   metrics.CriticalRegressionCount <= thresholds.maxCriticalRegressionCount,
			Actual:   metrics.CriticalRegressionCount,
			Expected: thresholds.maxCriticalRegressionCount,
		}
		checks = append(checks, check)
	}
	return checks
}

func selectorGateChecksPassed(checks []SelectorGateCheck) bool {
	for _, check := range checks {
		if !check.Passed {
			return false
		}
	}
	return true
}

func selectorGateIntValue(raw interface{}, fallback int) int {
	switch typed := raw.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	default:
		return fallback
	}
}

func selectorGateFloatValue(raw interface{}, fallback float64) float64 {
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
		return fallback
	}
}

func selectorGateSegmentMetricsMapValue(raw interface{}) map[string]SelectorGateSegmentMetrics {
	switch typed := raw.(type) {
	case map[string]SelectorGateSegmentMetrics:
		if len(typed) == 0 {
			return nil
		}
		out := make(map[string]SelectorGateSegmentMetrics, len(typed))
		for key, value := range typed {
			out[key] = value
		}
		return out
	case map[string]*SelectorGateSegmentMetrics:
		if len(typed) == 0 {
			return nil
		}
		out := make(map[string]SelectorGateSegmentMetrics, len(typed))
		for key, value := range typed {
			if value == nil {
				continue
			}
			out[key] = *value
		}
		if len(out) == 0 {
			return nil
		}
		return out
	case map[string]interface{}:
		if len(typed) == 0 {
			return nil
		}
		out := make(map[string]SelectorGateSegmentMetrics, len(typed))
		for key, value := range typed {
			if metrics, ok := selectorGateSegmentMetricsValue(value); ok {
				out[key] = metrics
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	default:
		return nil
	}
}

func selectorGateSegmentMetricsValue(raw interface{}) (SelectorGateSegmentMetrics, bool) {
	switch typed := raw.(type) {
	case SelectorGateSegmentMetrics:
		return typed, true
	case *SelectorGateSegmentMetrics:
		if typed == nil {
			return SelectorGateSegmentMetrics{}, false
		}
		return *typed, true
	case map[string]interface{}:
		caseCount := selectorGateIntValue(typed["case_count"], 0)
		routeAgreementCount := selectorGateIntValue(typed["route_agreement_count"], 0)
		routeCompatibleCount := selectorGateIntValue(typed["route_compatible_count"], 0)
		baseClarifyCount := selectorGateIntValue(typed["base_clarify_count"], 0)
		targetClarifyCount := selectorGateIntValue(typed["target_clarify_count"], 0)
		baseClarifyRate := selectorGateFloatValue(typed["base_clarify_rate"], safeComparisonRate(baseClarifyCount, caseCount))
		targetClarifyRate := selectorGateFloatValue(typed["target_clarify_rate"], safeComparisonRate(targetClarifyCount, caseCount))
		return SelectorGateSegmentMetrics{
			CaseCount:               caseCount,
			RouteAgreementCount:     routeAgreementCount,
			RouteAgreementRate:      selectorGateFloatValue(typed["route_agreement_rate"], safeComparisonRate(routeAgreementCount, caseCount)),
			RouteCompatibleCount:    routeCompatibleCount,
			RouteCompatibleRate:     selectorGateFloatValue(typed["route_compatible_rate"], safeComparisonRate(routeCompatibleCount, caseCount)),
			RouteImprovementCount:   selectorGateIntValue(typed["route_improvement_count"], 0),
			RouteDisagreementCount:  selectorGateIntValue(typed["route_disagreement_count"], 0),
			CriticalCaseCount:       selectorGateIntValue(typed["critical_case_count"], 0),
			CriticalRegressionCount: selectorGateIntValue(typed["critical_regression_count"], 0),
			BaseClarifyCount:        baseClarifyCount,
			BaseClarifyRate:         baseClarifyRate,
			TargetClarifyCount:      targetClarifyCount,
			TargetClarifyRate:       targetClarifyRate,
			ClarifyRateDelta:        selectorGateFloatValue(typed["clarify_rate_delta"], targetClarifyRate-baseClarifyRate),
		}, true
	default:
		return SelectorGateSegmentMetrics{}, false
	}
}
