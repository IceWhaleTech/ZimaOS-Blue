package harness

import (
	"context"
	"fmt"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

const (
	defaultExecutionEquivalenceMaxPassRateDrop            = 0.01
	defaultExecutionEquivalenceMaxCriticalRegressionCount = 0
)

type normalizedExecutionEquivalenceThresholds struct {
	maxPassRateDrop                  float64
	maxCriticalRegressionCount       int
	maxVerificationPassRateDrop      float64
	hasMaxVerificationPassRateDrop   bool
	maxEvidenceBackedPassRateDrop    float64
	hasMaxEvidenceBackedPassRateDrop bool
}

type executionCaseAttributes struct {
	Locale       string
	PrimaryRoute string
	Critical     bool
}

type executionReportSummary struct {
	caseCount              int
	passedCount            int
	criticalCaseCount      int
	criticalPassedCount    int
	infraBlockedCount      int
	verificationPassRate   float64
	evidenceBackedPassRate float64
	localeBreakdown        map[string]*ExecutionEquivalenceSegmentMetrics
	primaryRouteBreakdown  map[string]*ExecutionEquivalenceSegmentMetrics
}

func DefaultExecutionEquivalenceThresholds() ExecutionEquivalenceThresholds {
	maxPassRateDrop := defaultExecutionEquivalenceMaxPassRateDrop
	maxCriticalRegressionCount := defaultExecutionEquivalenceMaxCriticalRegressionCount
	return ExecutionEquivalenceThresholds{
		MaxPassRateDrop:            &maxPassRateDrop,
		MaxCriticalRegressionCount: &maxCriticalRegressionCount,
	}
}

func (c *Controller) EnsureBatch1ExecutionAssets(ctx context.Context, ownerUserID string) (*Batch1ExecutionAssets, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	if strings.TrimSpace(ownerUserID) == "" {
		return nil, fmt.Errorf("owner_user_id is required")
	}

	dataset, err := c.ensureBatch1ExecutionDataset(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	version, err := c.ensureBatch1ExecutionDatasetVersion(ctx, dataset, ownerUserID)
	if err != nil {
		return nil, err
	}
	evalSpec, err := c.ensureBatch1ExecutionEvalSpec(ctx, dataset, version, ownerUserID)
	if err != nil {
		return nil, err
	}
	return &Batch1ExecutionAssets{
		Dataset:        dataset,
		DatasetVersion: version,
		EvalSpec:       evalSpec,
	}, nil
}

func (c *Controller) EvaluateExecutionEquivalence(ctx context.Context, targetEvalRunID string, req ExecutionEquivalenceRequest) (*ExecutionEquivalenceReport, error) {
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
	baseReport, err := c.GetEvalRunReport(ctx, comparison.BaseEvalRunID)
	if err != nil {
		return nil, err
	}
	if baseReport == nil || baseReport.EvalRun == nil || baseReport.GroupReport == nil {
		return nil, fmt.Errorf("base eval run report is incomplete")
	}

	thresholds := normalizeExecutionEquivalenceThresholds(req.Thresholds)
	metrics := buildExecutionEquivalenceMetrics(baseReport, targetReport, comparison)
	checks := evaluateExecutionEquivalenceChecks(metrics, thresholds)

	report := &ExecutionEquivalenceReport{
		TargetEvalRunID:    targetReport.EvalRun.ID,
		BaseEvalRunID:      comparison.BaseEvalRunID,
		BaselineID:         comparison.BaselineID,
		ComparisonReportID: comparison.ID,
		Metrics:            metrics,
		Thresholds:         executionEquivalenceThresholdMap(thresholds),
		Checks:             checks,
		Passed:             executionEquivalenceChecksPassed(checks),
		CreatedAt:          timeutil.NowTime(),
	}
	c.emitOptimizationTrigger(ctx, OptimizationTrigger{
		Reason:              executionGateReason(report.Passed),
		CandidateID:         evalRunCandidateID(targetReport.EvalRun),
		EvalRunID:           targetReport.EvalRun.ID,
		BaseEvalRunID:       comparison.BaseEvalRunID,
		OptimizationRun:     evalRunIsOptimizationChild(targetReport.EvalRun),
		OptimizationSurface: evalRunOptimizationSurface(targetReport.EvalRun),
		Metadata:            evalRunOptimizationMetadata(targetReport.EvalRun),
	})
	return report, nil
}

func executionGateReason(passed bool) OptimizationReason {
	if passed {
		return OptimizationReasonExecutionGatePassed
	}
	return OptimizationReasonExecutionGateFailed
}

func (c *Controller) ensureBatch1ExecutionDataset(ctx context.Context, ownerUserID string) (*Dataset, error) {
	datasets, err := c.ListDatasets(ctx, DatasetFilter{
		OwnerUserID: strings.TrimSpace(ownerUserID),
		Limit:       200,
	})
	if err != nil {
		return nil, err
	}
	for i := range datasets {
		if batch1ExecutionDatasetMatches(&datasets[i]) {
			return &datasets[i], nil
		}
	}
	return c.CreateDataset(ctx, Batch1ExecutionDatasetSpec(ownerUserID))
}

func (c *Controller) ensureBatch1ExecutionDatasetVersion(ctx context.Context, dataset *Dataset, createdBy string) (*DatasetVersion, error) {
	if dataset == nil {
		return nil, fmt.Errorf("dataset is required")
	}
	versionSpec, err := Batch1ExecutionDatasetVersionSpec(createdBy)
	if err != nil {
		return nil, err
	}
	return c.ensureBuiltinDatasetVersion(ctx, dataset, versionSpec)
}

func (c *Controller) ensureBatch1ExecutionEvalSpec(ctx context.Context, dataset *Dataset, version *DatasetVersion, ownerUserID string) (*EvalSpec, error) {
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
	var versionMatch *EvalSpec
	var versionMatchWithHistory *EvalSpec
	var lineage *EvalSpec
	var lineageWithHistory *EvalSpec
	for i := range specs {
		if !batch1ExecutionEvalSpecLineageMatches(&specs[i]) {
			continue
		}
		hasHistory, historyErr := c.batch1ExecutionEvalSpecHasHistory(ctx, &specs[i])
		if historyErr != nil {
			return nil, historyErr
		}
		if batch1ExecutionEvalSpecMatches(&specs[i], version.ID) {
			candidate := specs[i]
			if versionMatch == nil {
				versionMatch = &candidate
			}
			if hasHistory && versionMatchWithHistory == nil {
				versionMatchWithHistory = &candidate
			}
			continue
		}
		candidate := specs[i]
		if lineage == nil {
			lineage = &candidate
		}
		if hasHistory && lineageWithHistory == nil {
			lineageWithHistory = &candidate
		}
	}
	if lineageWithHistory != nil {
		return c.updateBatch1ExecutionEvalSpec(ctx, lineageWithHistory, dataset, version, ownerUserID)
	}
	if versionMatchWithHistory != nil {
		return versionMatchWithHistory, nil
	}
	if versionMatch != nil {
		return versionMatch, nil
	}
	if lineage != nil {
		return c.updateBatch1ExecutionEvalSpec(ctx, lineage, dataset, version, ownerUserID)
	}
	return c.CreateEvalSpec(ctx, Batch1ExecutionEvalSpecSpec(dataset.ID, version.ID, ownerUserID))
}

func (c *Controller) updateBatch1ExecutionEvalSpec(ctx context.Context, spec *EvalSpec, dataset *Dataset, version *DatasetVersion, ownerUserID string) (*EvalSpec, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	if spec == nil || dataset == nil || version == nil {
		return nil, fmt.Errorf("spec, dataset, and version are required")
	}
	updated := Batch1ExecutionEvalSpecSpec(dataset.ID, version.ID, ownerUserID)
	next := *spec
	next.Name = updated.Name
	next.OwnerUserID = updated.OwnerUserID
	next.Subject = updated.Subject
	next.RunKind = updated.RunKind
	next.Profile = updated.Profile
	next.DatasetID = strings.TrimSpace(dataset.ID)
	next.DatasetVersionID = strings.TrimSpace(version.ID)
	next.SchedulerConfig = updated.SchedulerConfig
	next.ScoringConfig = updated.ScoringConfig
	next.RuntimePolicy = cloneMetadataMap(updated.RuntimePolicy)
	next.Metadata = cloneMetadataMap(updated.Metadata)
	if err := c.store.UpdateEvalSpec(ctx, &next); err != nil {
		return nil, err
	}
	return c.store.GetEvalSpec(ctx, next.ID)
}

func (c *Controller) batch1ExecutionEvalSpecHasHistory(ctx context.Context, spec *EvalSpec) (bool, error) {
	if c == nil || c.store == nil {
		return false, fmt.Errorf("harness controller is not configured")
	}
	if spec == nil {
		return false, nil
	}
	baselines, err := c.store.ListBaselines(ctx, BaselineFilter{
		EvalSpecID: strings.TrimSpace(spec.ID),
		Limit:      1,
	})
	if err != nil {
		return false, err
	}
	if len(baselines) > 0 {
		return true, nil
	}
	evalRuns, err := c.store.ListEvalRuns(ctx, EvalRunFilter{
		EvalSpecID: strings.TrimSpace(spec.ID),
		Limit:      1,
	})
	if err != nil {
		return false, err
	}
	return len(evalRuns) > 0, nil
}

func batch1ExecutionDatasetMatches(dataset *Dataset) bool {
	if dataset == nil {
		return false
	}
	return strings.TrimSpace(dataset.Name) == Batch1ExecutionDatasetName &&
		strings.TrimSpace(dataset.Subject) == Batch1ExecutionDatasetSubject &&
		metadataString(dataset.Metadata, "dataset_family") == "builtin_batch1_execution"
}

func batch1ExecutionEvalSpecMatches(spec *EvalSpec, versionID string) bool {
	if spec == nil {
		return false
	}
	return batch1ExecutionEvalSpecLineageMatches(spec) &&
		strings.TrimSpace(spec.DatasetVersionID) == strings.TrimSpace(versionID)
}

func batch1ExecutionEvalSpecLineageMatches(spec *EvalSpec) bool {
	if spec == nil {
		return false
	}
	return strings.TrimSpace(spec.Name) == Batch1ExecutionEvalName &&
		strings.TrimSpace(spec.Subject) == Batch1ExecutionDatasetSubject &&
		strings.TrimSpace(spec.Profile) == batch1ExecutionProfile &&
		metadataString(spec.Metadata, "gate_type") == "execution_equivalence"
}

func (c *Controller) ensureBuiltinDatasetVersion(ctx context.Context, dataset *Dataset, versionSpec DatasetVersionSpec) (*DatasetVersion, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	if dataset == nil {
		return nil, fmt.Errorf("dataset is required")
	}

	manifestHash := manifestSHA256(versionSpec.Manifest)
	match, conflict, err := c.findBuiltinDatasetVersion(ctx, dataset.ID, versionSpec, manifestHash)
	if err != nil {
		return nil, err
	}
	if match != nil {
		return c.activateBuiltinDatasetVersion(ctx, dataset, match.ID)
	}
	if conflict != nil {
		return nil, builtinDatasetVersionConflictError(dataset, versionSpec, conflict)
	}

	version, err := c.CreateDatasetVersion(ctx, dataset.ID, versionSpec)
	if err == nil {
		return c.activateBuiltinDatasetVersion(ctx, dataset, version.ID)
	}
	if !isHarnessUniqueConstraintError(err) {
		return nil, err
	}

	match, conflict, retryErr := c.findBuiltinDatasetVersion(ctx, dataset.ID, versionSpec, manifestHash)
	if retryErr != nil {
		return nil, err
	}
	if match != nil {
		return c.activateBuiltinDatasetVersion(ctx, dataset, match.ID)
	}
	if conflict != nil {
		return nil, builtinDatasetVersionConflictError(dataset, versionSpec, conflict)
	}
	return nil, err
}

func (c *Controller) findBuiltinDatasetVersion(ctx context.Context, datasetID string, versionSpec DatasetVersionSpec, manifestHash string) (*DatasetVersion, *DatasetVersion, error) {
	versions, err := c.ListDatasetVersions(ctx, datasetID, 200)
	if err != nil && !strings.Contains(err.Error(), "no rows") {
		return nil, nil, err
	}
	var conflict *DatasetVersion
	for i := range versions {
		version := versions[i]
		if strings.TrimSpace(version.Version) != strings.TrimSpace(versionSpec.Version) {
			continue
		}
		if strings.TrimSpace(version.SourceType) != strings.TrimSpace(versionSpec.SourceType) {
			continue
		}
		if strings.TrimSpace(version.ManifestSHA256) == strings.TrimSpace(manifestHash) {
			return &version, nil, nil
		}
		if conflict == nil {
			conflict = &version
		}
	}
	return nil, conflict, nil
}

func (c *Controller) activateBuiltinDatasetVersion(ctx context.Context, dataset *Dataset, versionID string) (*DatasetVersion, error) {
	if dataset == nil {
		return nil, fmt.Errorf("dataset is required")
	}
	if dataset.ActiveVersionID != versionID {
		dataset.ActiveVersionID = versionID
		if err := c.store.UpdateDataset(ctx, dataset); err != nil {
			return nil, err
		}
	}
	return c.GetDatasetVersion(ctx, versionID)
}

func builtinDatasetVersionConflictError(dataset *Dataset, versionSpec DatasetVersionSpec, conflict *DatasetVersion) error {
	datasetID := ""
	if dataset != nil {
		datasetID = strings.TrimSpace(dataset.ID)
	}
	conflictHash := ""
	if conflict != nil {
		conflictHash = strings.TrimSpace(conflict.ManifestSHA256)
	}
	return fmt.Errorf(
		"builtin dataset version %s for dataset %s already exists with a different manifest hash (%s); bump the builtin dataset version before re-running ensure",
		strings.TrimSpace(versionSpec.Version),
		datasetID,
		conflictHash,
	)
}

func isHarnessUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(lower, "unique constraint failed") || strings.Contains(lower, "constraint failed")
}

func normalizeExecutionEquivalenceThresholds(raw ExecutionEquivalenceThresholds) normalizedExecutionEquivalenceThresholds {
	normalized := normalizedExecutionEquivalenceThresholds{
		maxPassRateDrop:            defaultExecutionEquivalenceMaxPassRateDrop,
		maxCriticalRegressionCount: defaultExecutionEquivalenceMaxCriticalRegressionCount,
	}
	if raw.MaxPassRateDrop != nil {
		normalized.maxPassRateDrop = *raw.MaxPassRateDrop
	}
	if raw.MaxCriticalRegressionCount != nil {
		normalized.maxCriticalRegressionCount = *raw.MaxCriticalRegressionCount
	}
	if raw.MaxVerificationPassRateDrop != nil {
		normalized.maxVerificationPassRateDrop = *raw.MaxVerificationPassRateDrop
		normalized.hasMaxVerificationPassRateDrop = true
	}
	if raw.MaxEvidenceBackedPassRateDrop != nil {
		normalized.maxEvidenceBackedPassRateDrop = *raw.MaxEvidenceBackedPassRateDrop
		normalized.hasMaxEvidenceBackedPassRateDrop = true
	}
	return normalized
}

func executionEquivalenceThresholdMap(thresholds normalizedExecutionEquivalenceThresholds) map[string]interface{} {
	out := map[string]interface{}{
		"max_pass_rate_drop":            thresholds.maxPassRateDrop,
		"max_critical_regression_count": thresholds.maxCriticalRegressionCount,
	}
	if thresholds.hasMaxVerificationPassRateDrop {
		out["max_verification_pass_rate_drop"] = thresholds.maxVerificationPassRateDrop
	}
	if thresholds.hasMaxEvidenceBackedPassRateDrop {
		out["max_evidence_backed_pass_rate_drop"] = thresholds.maxEvidenceBackedPassRateDrop
	}
	return out
}

func buildExecutionEquivalenceMetrics(baseReport *EvalRunReport, targetReport *EvalRunReport, comparison *ComparisonReport) ExecutionEquivalenceMetrics {
	baseSummary := buildExecutionReportSummary(baseReport)
	targetSummary := buildExecutionReportSummary(targetReport)
	basePassRate := safeComparisonRate(baseSummary.passedCount, baseSummary.caseCount)
	baseCriticalPassRate := safeComparisonRate(baseSummary.criticalPassedCount, baseSummary.criticalCaseCount)
	targetPassRate := executionSummaryPassRate(targetSummary, basePassRate)
	targetCriticalPassRate := executionSummaryCriticalPassRate(targetSummary, baseCriticalPassRate)
	baseVerificationPassRate := baseSummary.verificationPassRate
	targetVerificationPassRate := executionSummaryVerificationPassRate(targetSummary, baseVerificationPassRate)
	baseEvidenceBackedPassRate := baseSummary.evidenceBackedPassRate
	targetEvidenceBackedPassRate := executionSummaryEvidenceBackedPassRate(targetSummary, baseEvidenceBackedPassRate)

	metrics := ExecutionEquivalenceMetrics{
		CaseCount:                    targetSummary.caseCount,
		PassedCount:                  targetSummary.passedCount,
		PassRate:                     targetPassRate,
		CriticalCaseCount:            targetSummary.criticalCaseCount,
		CriticalPassedCount:          targetSummary.criticalPassedCount,
		CriticalPassRate:             targetCriticalPassRate,
		InfraBlockedCount:            targetSummary.infraBlockedCount,
		BaseInfraBlockedCount:        baseSummary.infraBlockedCount,
		TargetInfraBlockedCount:      targetSummary.infraBlockedCount,
		BasePassRate:                 basePassRate,
		TargetPassRate:               targetPassRate,
		BaseVerificationPassRate:     baseVerificationPassRate,
		TargetVerificationPassRate:   targetVerificationPassRate,
		BaseEvidenceBackedPassRate:   baseEvidenceBackedPassRate,
		TargetEvidenceBackedPassRate: targetEvidenceBackedPassRate,
		LocaleBreakdown:              executionSegmentMetricsMapValue(targetSummary.localeBreakdown),
		PrimaryRouteBreakdown:        executionSegmentMetricsMapValue(targetSummary.primaryRouteBreakdown),
	}
	if metrics.LocaleBreakdown == nil {
		metrics.LocaleBreakdown = map[string]ExecutionEquivalenceSegmentMetrics{}
	}
	if metrics.PrimaryRouteBreakdown == nil {
		metrics.PrimaryRouteBreakdown = map[string]ExecutionEquivalenceSegmentMetrics{}
	}
	metrics.InfraBlockedDelta = metrics.TargetInfraBlockedCount - metrics.BaseInfraBlockedCount
	metrics.PassRateDelta = metrics.TargetPassRate - metrics.BasePassRate
	metrics.VerificationPassRateDelta = metrics.TargetVerificationPassRate - metrics.BaseVerificationPassRate
	metrics.EvidenceBackedPassRateDelta = metrics.TargetEvidenceBackedPassRate - metrics.BaseEvidenceBackedPassRate
	applyExecutionEquivalenceComparison(metrics.LocaleBreakdown, metrics.PrimaryRouteBreakdown, baseReport, targetReport, comparison, &metrics)
	return metrics
}

func executionSummaryPassRate(summary executionReportSummary, fallback float64) float64 {
	if summary.caseCount == 0 && summary.infraBlockedCount > 0 {
		return fallback
	}
	return safeComparisonRate(summary.passedCount, summary.caseCount)
}

func executionSummaryCriticalPassRate(summary executionReportSummary, fallback float64) float64 {
	if summary.criticalCaseCount == 0 && summary.infraBlockedCount > 0 {
		return fallback
	}
	return safeComparisonRate(summary.criticalPassedCount, summary.criticalCaseCount)
}

func executionSummaryVerificationPassRate(summary executionReportSummary, fallback float64) float64 {
	if summary.caseCount == 0 && summary.infraBlockedCount > 0 {
		return fallback
	}
	return summary.verificationPassRate
}

func executionSummaryEvidenceBackedPassRate(summary executionReportSummary, fallback float64) float64 {
	if summary.caseCount == 0 && summary.infraBlockedCount > 0 {
		return fallback
	}
	return summary.evidenceBackedPassRate
}

func buildExecutionReportSummary(report *EvalRunReport) executionReportSummary {
	summary := executionReportSummary{
		localeBreakdown:       map[string]*ExecutionEquivalenceSegmentMetrics{},
		primaryRouteBreakdown: map[string]*ExecutionEquivalenceSegmentMetrics{},
	}
	if report == nil || report.GroupReport == nil {
		return summary
	}

	latestCards := latestScorecardsByItem(report.GroupReport.Scorecards)
	for _, item := range report.GroupReport.Items {
		card := latestCards[item.ID]
		if executionEquivalenceItemInfraBlocked(item, card) {
			summary.infraBlockedCount++
			executionSegmentAddInfraBlocked(summary.localeBreakdown, metadataString(item.Metadata, "locale"))
			executionSegmentAddInfraBlocked(summary.primaryRouteBreakdown, metadataString(item.Metadata, "primary_route"))
			continue
		}
		passed := executionEquivalenceItemPassed(item, card)
		critical := metadataBoolValue(item.Metadata, "critical")

		summary.caseCount++
		if passed {
			summary.passedCount++
		}
		if critical {
			summary.criticalCaseCount++
			if passed {
				summary.criticalPassedCount++
			}
		}

		executionSegmentAddCase(summary.localeBreakdown, metadataString(item.Metadata, "locale"), passed, critical)
		executionSegmentAddCase(summary.primaryRouteBreakdown, metadataString(item.Metadata, "primary_route"), passed, critical)
	}
	summary.verificationPassRate = computeComparisonVerificationPassRate(report.GroupReport.Items, latestCards)
	summary.evidenceBackedPassRate = computeComparisonEvidenceBackedPassRate(report.GroupReport.Items, latestCards)
	finalizeExecutionSegmentMetrics(summary.localeBreakdown)
	finalizeExecutionSegmentMetrics(summary.primaryRouteBreakdown)
	return summary
}

func applyExecutionEquivalenceComparison(localeBreakdown map[string]ExecutionEquivalenceSegmentMetrics, primaryRouteBreakdown map[string]ExecutionEquivalenceSegmentMetrics, baseReport *EvalRunReport, targetReport *EvalRunReport, comparison *ComparisonReport, metrics *ExecutionEquivalenceMetrics) {
	if comparison == nil || metrics == nil {
		return
	}
	baseIndex := executionCaseAttributeIndex(baseReport)
	targetIndex := executionCaseAttributeIndex(targetReport)

	for _, delta := range comparison.Regressions {
		if executionEquivalenceInfraBlockedDelta(delta) {
			continue
		}
		metrics.RegressionCount++
		attrs := executionAttributesForDelta(delta, targetIndex, baseIndex)
		if executionEquivalenceNewFailure(delta) {
			metrics.NewFailureCount++
		}
		if attrs.Critical {
			metrics.CriticalRegressionCount++
		}
		executionSegmentApplyDelta(localeBreakdown, attrs.Locale, delta, attrs.Critical, "regression")
		executionSegmentApplyDelta(primaryRouteBreakdown, attrs.PrimaryRoute, delta, attrs.Critical, "regression")
	}
	for _, delta := range comparison.Improvements {
		if executionEquivalenceInfraBlockedDelta(delta) {
			continue
		}
		metrics.ImprovementCount++
		attrs := executionAttributesForDelta(delta, targetIndex, baseIndex)
		if executionEquivalenceResolvedFailure(delta) {
			metrics.ResolvedFailureCount++
		}
		executionSegmentApplyDelta(localeBreakdown, attrs.Locale, delta, attrs.Critical, "improvement")
		executionSegmentApplyDelta(primaryRouteBreakdown, attrs.PrimaryRoute, delta, attrs.Critical, "improvement")
	}
}

func executionEquivalenceItemPassed(item RunGroupItem, card Scorecard) bool {
	if strings.TrimSpace(string(card.Verdict)) != "" {
		return card.Verdict == ScoreVerdictPass
	}
	return item.Status == RunGroupItemStatusPassed
}

func executionSegmentAddCase(m map[string]*ExecutionEquivalenceSegmentMetrics, key string, passed bool, critical bool) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	metrics := executionSegmentMetricsRef(m, key)
	metrics.CaseCount++
	if passed {
		metrics.PassedCount++
	}
	if critical {
		metrics.CriticalCaseCount++
		if passed {
			metrics.CriticalPassedCount++
		}
	}
}

func executionSegmentAddInfraBlocked(m map[string]*ExecutionEquivalenceSegmentMetrics, key string) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	metrics := executionSegmentMetricsRef(m, key)
	metrics.InfraBlockedCount++
}

func finalizeExecutionSegmentMetrics(m map[string]*ExecutionEquivalenceSegmentMetrics) {
	for _, metrics := range m {
		if metrics == nil {
			continue
		}
		metrics.PassRate = safeComparisonRate(metrics.PassedCount, metrics.CaseCount)
		metrics.CriticalPassRate = safeComparisonRate(metrics.CriticalPassedCount, metrics.CriticalCaseCount)
	}
}

func executionSegmentMetricsRef(m map[string]*ExecutionEquivalenceSegmentMetrics, key string) *ExecutionEquivalenceSegmentMetrics {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	metrics := m[key]
	if metrics == nil {
		metrics = &ExecutionEquivalenceSegmentMetrics{}
		m[key] = metrics
	}
	return metrics
}

func executionSegmentMetricsMapValue(raw map[string]*ExecutionEquivalenceSegmentMetrics) map[string]ExecutionEquivalenceSegmentMetrics {
	if len(raw) == 0 {
		return nil
	}
	out := make(map[string]ExecutionEquivalenceSegmentMetrics, len(raw))
	for key, value := range raw {
		if value == nil {
			continue
		}
		out[key] = *value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func executionCaseAttributeIndex(report *EvalRunReport) map[string]executionCaseAttributes {
	out := make(map[string]executionCaseAttributes)
	if report == nil || report.GroupReport == nil {
		return out
	}
	for _, item := range report.GroupReport.Items {
		out[comparisonCaseKey(item)] = executionCaseAttributes{
			Locale:       metadataString(item.Metadata, "locale"),
			PrimaryRoute: metadataString(item.Metadata, "primary_route"),
			Critical:     metadataBoolValue(item.Metadata, "critical"),
		}
	}
	return out
}

func executionAttributesForDelta(delta ComparisonCaseDelta, targetIndex map[string]executionCaseAttributes, baseIndex map[string]executionCaseAttributes) executionCaseAttributes {
	if attrs, ok := targetIndex[delta.Key]; ok {
		return attrs
	}
	if attrs, ok := baseIndex[delta.Key]; ok {
		return attrs
	}
	return executionCaseAttributes{}
}

func executionSegmentApplyDelta(m map[string]ExecutionEquivalenceSegmentMetrics, key string, delta ComparisonCaseDelta, critical bool, classification string) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	metrics := m[key]
	switch strings.TrimSpace(classification) {
	case "regression":
		metrics.RegressionCount++
		if executionEquivalenceNewFailure(delta) {
			metrics.NewFailureCount++
		}
		if critical {
			metrics.CriticalRegressionCount++
		}
	case "improvement":
		metrics.ImprovementCount++
		if executionEquivalenceResolvedFailure(delta) {
			metrics.ResolvedFailureCount++
		}
	}
	m[key] = metrics
}

func executionEquivalenceNewFailure(delta ComparisonCaseDelta) bool {
	return isFailingComparisonVerdict(delta.TargetVerdict) && !isFailingComparisonVerdict(delta.BaseVerdict)
}

func executionEquivalenceResolvedFailure(delta ComparisonCaseDelta) bool {
	return !isFailingComparisonVerdict(delta.TargetVerdict) && isFailingComparisonVerdict(delta.BaseVerdict)
}

func executionEquivalenceInfraBlockedDelta(delta ComparisonCaseDelta) bool {
	return isInfraFailureLabel(delta.BaseFailureLabel) || isInfraFailureLabel(delta.TargetFailureLabel)
}

func executionEquivalenceItemFailureLabel(card Scorecard) string {
	return metadataString(decodeJSONMap(card.BreakdownJSON), "failure_label")
}

func executionEquivalenceItemInfraBlocked(_ RunGroupItem, card Scorecard) bool {
	return isInfraFailureLabel(executionEquivalenceItemFailureLabel(card))
}

func evaluateExecutionEquivalenceChecks(metrics ExecutionEquivalenceMetrics, thresholds normalizedExecutionEquivalenceThresholds) []ExecutionEquivalenceCheck {
	passRateDrop := metrics.BasePassRate - metrics.TargetPassRate
	if passRateDrop < 0 {
		passRateDrop = 0
	}
	checks := []ExecutionEquivalenceCheck{
		{
			Name:     "pass_rate_drop",
			Passed:   passRateDrop <= thresholds.maxPassRateDrop,
			Actual:   passRateDrop,
			Expected: thresholds.maxPassRateDrop,
			Details: map[string]interface{}{
				"base_pass_rate":   metrics.BasePassRate,
				"target_pass_rate": metrics.TargetPassRate,
			},
		},
		{
			Name:     "critical_regression_count",
			Passed:   metrics.CriticalRegressionCount <= thresholds.maxCriticalRegressionCount,
			Actual:   metrics.CriticalRegressionCount,
			Expected: thresholds.maxCriticalRegressionCount,
			Details: map[string]interface{}{
				"critical_case_count":   metrics.CriticalCaseCount,
				"critical_passed_count": metrics.CriticalPassedCount,
			},
		},
		{
			Name:     "infra_blocked_count",
			Passed:   metrics.TargetInfraBlockedCount == 0,
			Actual:   metrics.TargetInfraBlockedCount,
			Expected: 0,
			Details: map[string]interface{}{
				"base_infra_blocked_count":   metrics.BaseInfraBlockedCount,
				"target_infra_blocked_count": metrics.TargetInfraBlockedCount,
			},
		},
	}

	if thresholds.hasMaxVerificationPassRateDrop {
		verificationDrop := metrics.BaseVerificationPassRate - metrics.TargetVerificationPassRate
		if verificationDrop < 0 {
			verificationDrop = 0
		}
		checks = append(checks, ExecutionEquivalenceCheck{
			Name:     "verification_pass_rate_drop",
			Passed:   verificationDrop <= thresholds.maxVerificationPassRateDrop,
			Actual:   verificationDrop,
			Expected: thresholds.maxVerificationPassRateDrop,
			Details: map[string]interface{}{
				"base_verification_pass_rate":   metrics.BaseVerificationPassRate,
				"target_verification_pass_rate": metrics.TargetVerificationPassRate,
			},
		})
	}

	if thresholds.hasMaxEvidenceBackedPassRateDrop {
		evidenceDrop := metrics.BaseEvidenceBackedPassRate - metrics.TargetEvidenceBackedPassRate
		if evidenceDrop < 0 {
			evidenceDrop = 0
		}
		checks = append(checks, ExecutionEquivalenceCheck{
			Name:     "evidence_backed_pass_rate_drop",
			Passed:   evidenceDrop <= thresholds.maxEvidenceBackedPassRateDrop,
			Actual:   evidenceDrop,
			Expected: thresholds.maxEvidenceBackedPassRateDrop,
			Details: map[string]interface{}{
				"base_evidence_backed_pass_rate":   metrics.BaseEvidenceBackedPassRate,
				"target_evidence_backed_pass_rate": metrics.TargetEvidenceBackedPassRate,
			},
		})
	}

	return checks
}

func executionEquivalenceChecksPassed(checks []ExecutionEquivalenceCheck) bool {
	for _, check := range checks {
		if !check.Passed {
			return false
		}
	}
	return true
}
