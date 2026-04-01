package harness

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

type evalCaseSnapshot struct {
	Key           string
	Label         string
	ItemIndex     int
	Profile       string
	Locale        string
	PrimaryRoute  string
	Verdict       string
	Status        string
	Score         float64
	RunID         string
	Reason        string
	FailureLabel  string
	Verification  string
	EvidenceScore float64
	CompareFields []string
	Structured    map[string]interface{}
	Critical      bool
}

type comparisonReportView struct {
	report            *EvalRunReport
	summary           map[string]interface{}
	latestCards       map[string]Scorecard
	runByID           map[string]Run
	caseSnapshotByKey map[string]evalCaseSnapshot
	derivedMetrics    *comparisonViewDerivedMetrics
}

type comparisonBaseSelection struct {
	evalRunID string
	baseline  *Baseline
}

type comparisonCaseRollup struct {
	regressions      []ComparisonCaseDelta
	improvements     []ComparisonCaseDelta
	changedCases     int
	unstableCases    int
	newFailures      int
	resolvedFailures int
}

type comparisonViewMetricSnapshot struct {
	overallScore           float64
	passRate               float64
	verificationPassRate   float64
	evidenceBackedPassRate float64
	retryRecoveredCount    int
	linkedRunCount         int
	artifactCount          int
}

type comparisonStructuralDelta struct {
	failureLabelDelta       map[string]interface{}
	verdictCountDelta       map[string]interface{}
	linkedRunCountDelta     int
	artifactCountDelta      int
	routeCaseCount          int
	routeAgreementCount     int
	routeCompatibleCount    int
	routeImprovementCount   int
	routeDisagreementCount  int
	criticalRouteCaseCount  int
	criticalRegressionCount int
	baseClarifyCount        int
	targetClarifyCount      int
	localeBreakdown         map[string]SelectorGateSegmentMetrics
	primaryRouteBreakdown   map[string]SelectorGateSegmentMetrics
}

type comparisonMetricBundle struct {
	base       comparisonViewMetricSnapshot
	target     comparisonViewMetricSnapshot
	structural comparisonStructuralDelta
}

type comparisonExecutionContext struct {
	targetEvalRun *EvalRun
	baseEvalRun   *EvalRun
	baseline      *Baseline
	targetReport  *EvalRunReport
	baseReport    *EvalRunReport
}

type comparisonReportContextPair struct {
	target   *evalRunReportContext
	base     *evalRunReportContext
	baseline *Baseline
}

type comparisonStorageBinding struct {
	ownerUserID     string
	baselineID      string
	evalSpecID      string
	baseEvalRunID   string
	targetEvalRunID string
	createdAt       time.Time
}

type comparisonReportBundle struct {
	baseView   *comparisonReportView
	targetView *comparisonReportView
	rollup     comparisonCaseRollup
	metrics    comparisonMetricBundle
}

type comparisonViewDerivedMetrics struct {
	verificationPassRate float64
	evidenceBackedRate   float64
	retryRecoveredCount  int
	verdictCounts        map[string]int
	failureLabelCounts   map[string]int
}

type comparisonCaseAssessment struct {
	delta      ComparisonCaseDelta
	hasBase    bool
	hasTarget  bool
	baseRank   int
	targetRank int
}

type comparisonRouteDelta struct {
	routeCaseCount          int
	routeAgreementCount     int
	routeCompatibleCount    int
	routeImprovementCount   int
	routeDisagreementCount  int
	criticalRouteCaseCount  int
	criticalRegressionCount int
	baseClarifyCount        int
	targetClarifyCount      int
	localeBreakdown         map[string]*comparisonRouteBreakdown
	primaryRouteBreakdown   map[string]*comparisonRouteBreakdown
}

type comparisonMetricDeltaSet struct {
	overallScoreDelta       float64
	passRateDelta           float64
	verificationRateDelta   float64
	evidenceBackedRateDelta float64
	retryRecoveredDelta     int
}

type comparisonSummaryIdentity struct {
	comparisonKind string
	baselineName   string
	baseTitle      string
	targetTitle    string
	baseGroupID    string
	targetGroupID  string
}

type comparisonSummaryRollup struct {
	changedCaseCount     int
	regressionCount      int
	improvementCount     int
	unstableCaseCount    int
	newFailureCount      int
	resolvedFailureCount int
}

type comparisonScorerDeltaContext struct {
	baseOverallScore             float64
	targetOverallScore           float64
	overallScoreDelta            float64
	basePassRate                 float64
	targetPassRate               float64
	passRateDelta                float64
	baseVerificationPassRate     float64
	targetVerificationPassRate   float64
	verificationRateDelta        float64
	baseEvidenceBackedPassRate   float64
	targetEvidenceBackedPassRate float64
	evidenceBackedRateDelta      float64
	baseRetryRecoveredCount      int
	targetRetryRecoveredCount    int
	retryRecoveredDelta          int
	failureLabelDelta            map[string]interface{}
	verdictCountDelta            map[string]interface{}
	linkedRunCountDelta          int
	artifactCountDelta           int
	routeCaseCount               int
	routeAgreementCount          int
	routeAgreementRate           float64
	routeCompatibleCount         int
	routeCompatibleRate          float64
	routeImprovementCount        int
	routeDisagreementCount       int
	criticalRouteCaseCount       int
	criticalRegressionCount      int
	baseClarifyRate              float64
	targetClarifyRate            float64
	clarifyRateDelta             float64
	localeBreakdown              map[string]SelectorGateSegmentMetrics
	primaryRouteBreakdown        map[string]SelectorGateSegmentMetrics
}

type comparisonRouteBreakdown struct {
	caseCount               int
	routeAgreementCount     int
	routeCompatibleCount    int
	routeImprovementCount   int
	routeDisagreementCount  int
	criticalCaseCount       int
	criticalRegressionCount int
	baseClarifyCount        int
	targetClarifyCount      int
}

type comparisonReportBody struct {
	regressions  []ComparisonCaseDelta
	improvements []ComparisonCaseDelta
}

type comparisonReportSections struct {
	summary     map[string]interface{}
	body        comparisonReportBody
	scorerDelta map[string]interface{}
}

type comparisonPresentationContext struct {
	identity    comparisonSummaryIdentity
	rollup      comparisonSummaryRollup
	metricDelta comparisonMetricDeltaSet
	scorer      comparisonScorerDeltaContext
	body        comparisonReportBody
}

func newComparisonReportView(report *EvalRunReport) *comparisonReportView {
	view := &comparisonReportView{report: report}
	if report == nil || report.GroupReport == nil {
		return view
	}
	if report.GroupReport.Group != nil {
		view.summary = report.GroupReport.Group.Summary
	}
	view.latestCards = latestScorecardsByItem(report.GroupReport.Scorecards)
	if len(report.GroupReport.LinkedRuns) > 0 {
		view.runByID = make(map[string]Run, len(report.GroupReport.LinkedRuns))
		for _, run := range report.GroupReport.LinkedRuns {
			view.runByID[run.ID] = run
		}
	}
	return view
}

func (c *Controller) CompareEvalRun(ctx context.Context, targetEvalRunID string, req CompareEvalRunRequest) (*ComparisonReport, error) {
	execCtx, err := c.loadComparisonExecutionContext(ctx, targetEvalRunID, req)
	if err != nil {
		return nil, err
	}
	comparison, err := c.createComparisonReport(ctx, execCtx)
	if err != nil {
		return nil, err
	}
	return comparison, nil
}

func (c *Controller) createComparisonReport(ctx context.Context, execCtx *comparisonExecutionContext) (*ComparisonReport, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	if execCtx == nil {
		return nil, fmt.Errorf("comparison execution context is required")
	}
	comparison := buildStoredComparisonReport(execCtx)
	if err := c.store.CreateComparisonReport(ctx, comparison); err != nil {
		return nil, err
	}
	return comparison, nil
}

func buildStoredComparisonReport(execCtx *comparisonExecutionContext) *ComparisonReport {
	if execCtx == nil {
		return nil
	}
	comparison := buildComparisonReport(execCtx.baseReport, execCtx.targetReport, execCtx.baseline)
	return bindStoredComparisonReport(comparison, buildComparisonStorageBinding(execCtx))
}

func (c *Controller) loadComparisonExecutionContext(ctx context.Context, targetEvalRunID string, req CompareEvalRunRequest) (*comparisonExecutionContext, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	reportPair, err := c.loadComparisonReportContextPair(ctx, targetEvalRunID, req)
	if err != nil {
		return nil, err
	}
	baseReport, err := c.buildEvalRunReportFromContext(reportPair.base)
	if err != nil {
		return nil, err
	}
	targetReport, err := c.buildEvalRunReportFromContext(reportPair.target)
	if err != nil {
		return nil, err
	}
	return &comparisonExecutionContext{
		targetEvalRun: reportPair.target.evalRun,
		baseEvalRun:   reportPair.base.evalRun,
		baseline:      reportPair.baseline,
		targetReport:  targetReport,
		baseReport:    baseReport,
	}, nil
}

func (c *Controller) loadComparisonReportContextPair(ctx context.Context, targetEvalRunID string, req CompareEvalRunRequest) (*comparisonReportContextPair, error) {
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
	if err := validateComparisonEvalRunPair(baseEvalRun, targetEvalRun); err != nil {
		return nil, err
	}
	targetReportCtx, err := c.loadEvalRunReportContext(ctx, targetEvalRun)
	if err != nil {
		return nil, err
	}
	baseReportCtx, err := c.loadEvalRunReportContext(ctx, baseEvalRun)
	if err != nil {
		return nil, err
	}
	if err := validateComparisonReportContextPair(baseReportCtx, targetReportCtx); err != nil {
		return nil, err
	}
	return &comparisonReportContextPair{
		target:   targetReportCtx,
		base:     baseReportCtx,
		baseline: baseline,
	}, nil
}

func (c *Controller) resolveComparisonBase(ctx context.Context, targetEvalRun *EvalRun, req CompareEvalRunRequest) (*EvalRun, *Baseline, error) {
	selection, err := c.resolveComparisonBaseSelection(ctx, targetEvalRun, req)
	if err != nil {
		return nil, nil, err
	}
	baseEvalRun, err := c.GetEvalRun(ctx, selection.evalRunID)
	if err != nil {
		return nil, nil, err
	}
	return baseEvalRun, selection.baseline, nil
}

func (c *Controller) resolveComparisonBaseSelection(ctx context.Context, targetEvalRun *EvalRun, req CompareEvalRunRequest) (*comparisonBaseSelection, error) {
	if targetEvalRun == nil {
		return nil, fmt.Errorf("target eval run is required")
	}

	if baselineID := strings.TrimSpace(req.BaselineID); baselineID != "" {
		baseline, err := c.store.GetBaseline(ctx, baselineID)
		if err != nil {
			return nil, err
		}
		return &comparisonBaseSelection{
			evalRunID: strings.TrimSpace(baseline.EvalRunID),
			baseline:  baseline,
		}, nil
	}

	if baseEvalRunID := preferredComparisonBaseEvalRunID(targetEvalRun, req); baseEvalRunID != "" {
		return &comparisonBaseSelection{evalRunID: baseEvalRunID}, nil
	}

	baseline, err := c.resolveDefaultComparisonBaseline(ctx, targetEvalRun)
	if err != nil {
		return nil, err
	}
	if baseline != nil {
		return &comparisonBaseSelection{
			evalRunID: strings.TrimSpace(baseline.EvalRunID),
			baseline:  baseline,
		}, nil
	}
	return nil, fmt.Errorf("no baseline or base eval run selected")
}

func preferredComparisonBaseEvalRunID(targetEvalRun *EvalRun, req CompareEvalRunRequest) string {
	if targetEvalRun == nil {
		return ""
	}
	return firstNonEmpty(
		strings.TrimSpace(req.BaseEvalRunID),
		strings.TrimSpace(targetEvalRun.BaselineEvalRunID),
	)
}

func (c *Controller) resolveDefaultComparisonBaseline(ctx context.Context, targetEvalRun *EvalRun) (*Baseline, error) {
	if targetEvalRun == nil {
		return nil, nil
	}
	baselines, err := c.store.ListBaselines(ctx, BaselineFilter{
		OwnerUserID: targetEvalRun.OwnerUserID,
		EvalSpecID:  targetEvalRun.EvalSpecID,
		Limit:       100,
	})
	if err != nil {
		return nil, err
	}
	for i := range baselines {
		if !baselines[i].IsDefault {
			continue
		}
		baseline := baselines[i]
		return &baseline, nil
	}
	return nil, nil
}

func validateComparisonReportContextPair(baseReportCtx *evalRunReportContext, targetReportCtx *evalRunReportContext) error {
	return validateComparisonEvalRunPair(evalRunFromReportContext(baseReportCtx), evalRunFromReportContext(targetReportCtx))
}

func validateComparisonEvalRunPair(baseEvalRun *EvalRun, targetEvalRun *EvalRun) error {
	switch {
	case targetEvalRun == nil:
		return fmt.Errorf("target eval run is required")
	case baseEvalRun == nil:
		return fmt.Errorf("comparison base is required")
	case baseEvalRun.ID == targetEvalRun.ID:
		return fmt.Errorf("comparison base must differ from target eval run")
	case baseEvalRun.EvalSpecID != targetEvalRun.EvalSpecID:
		return fmt.Errorf("eval runs must belong to the same eval spec")
	default:
		return nil
	}
}

func evalRunFromReportContext(reportCtx *evalRunReportContext) *EvalRun {
	if reportCtx == nil {
		return nil
	}
	return reportCtx.evalRun
}

func buildComparisonStorageBinding(execCtx *comparisonExecutionContext) *comparisonStorageBinding {
	if execCtx == nil {
		return nil
	}
	return &comparisonStorageBinding{
		ownerUserID:     firstNonEmpty(ownerUserIDFromEvalRun(execCtx.targetEvalRun), ownerUserIDFromEvalRun(execCtx.baseEvalRun)),
		baselineID:      baselineID(execCtx.baseline),
		evalSpecID:      evalSpecIDFromEvalRun(execCtx.targetEvalRun),
		baseEvalRunID:   evalRunID(execCtx.baseEvalRun),
		targetEvalRunID: evalRunID(execCtx.targetEvalRun),
		createdAt:       timeutil.NowTime(),
	}
}

func bindStoredComparisonReport(report *ComparisonReport, binding *comparisonStorageBinding) *ComparisonReport {
	if report == nil || binding == nil {
		return report
	}
	report.ID = uuid.NewString()
	report.OwnerUserID = binding.ownerUserID
	report.BaselineID = binding.baselineID
	report.EvalSpecID = binding.evalSpecID
	report.BaseEvalRunID = binding.baseEvalRunID
	report.TargetEvalRunID = binding.targetEvalRunID
	report.CreatedAt = binding.createdAt
	return report
}

func ownerUserIDFromEvalRun(evalRun *EvalRun) string {
	if evalRun == nil {
		return ""
	}
	return strings.TrimSpace(evalRun.OwnerUserID)
}

func evalSpecIDFromEvalRun(evalRun *EvalRun) string {
	if evalRun == nil {
		return ""
	}
	return strings.TrimSpace(evalRun.EvalSpecID)
}

func evalRunID(evalRun *EvalRun) string {
	if evalRun == nil {
		return ""
	}
	return strings.TrimSpace(evalRun.ID)
}

func buildComparisonReport(baseReport *EvalRunReport, targetReport *EvalRunReport, baseline *Baseline) *ComparisonReport {
	return buildComparisonReportFromBundle(buildComparisonReportBundle(baseReport, targetReport), baseline)
}

func buildComparisonReportBundle(baseReport *EvalRunReport, targetReport *EvalRunReport) *comparisonReportBundle {
	baseView := newComparisonReportView(baseReport)
	targetView := newComparisonReportView(targetReport)
	return &comparisonReportBundle{
		baseView:   baseView,
		targetView: targetView,
		rollup:     buildComparisonCaseRollup(baseView, targetView),
		metrics:    buildComparisonMetricBundle(baseView, targetView),
	}
}

func buildComparisonReportFromBundle(bundle *comparisonReportBundle, baseline *Baseline) *ComparisonReport {
	sections := buildComparisonReportSections(baseline, bundle)

	return &ComparisonReport{
		BaselineID:   baselineID(baseline),
		Summary:      sections.summary,
		Regressions:  sections.body.regressions,
		Improvements: sections.body.improvements,
		ScorerDelta:  sections.scorerDelta,
	}
}

func buildComparisonCaseRollup(baseView *comparisonReportView, targetView *comparisonReportView) comparisonCaseRollup {
	baseCases := baseView.caseSnapshots()
	targetCases := targetView.caseSnapshots()
	keys := sortedCaseKeys(baseCases, targetCases)
	rollup := comparisonCaseRollup{
		regressions:  make([]ComparisonCaseDelta, 0),
		improvements: make([]ComparisonCaseDelta, 0),
	}
	for _, key := range keys {
		baseCase, baseOK := baseCases[key]
		targetCase, targetOK := targetCases[key]
		if !baseOK && !targetOK {
			continue
		}
		assessment := assessComparisonCase(baseCase, baseOK, targetCase, targetOK)
		classification, changed := classifyComparisonCaseAssessment(assessment)
		if !changed {
			continue
		}
		applyComparisonCaseRollupChange(&rollup, assessment.delta, classification)
	}
	return rollup
}

func buildComparisonMetricBundle(baseView *comparisonReportView, targetView *comparisonReportView) comparisonMetricBundle {
	baseMetrics := buildComparisonViewMetricSnapshot(baseView)
	targetMetrics := buildComparisonViewMetricSnapshot(targetView)
	return comparisonMetricBundle{
		base:       baseMetrics,
		target:     targetMetrics,
		structural: buildComparisonStructuralDelta(baseView, targetView, baseMetrics, targetMetrics),
	}
}

func buildComparisonSummary(baseline *Baseline, bundle *comparisonReportBundle) map[string]interface{} {
	return buildComparisonSummaryFromContext(buildComparisonPresentationContext(baseline, bundle))
}

func buildComparisonSummaryFromContext(context comparisonPresentationContext) map[string]interface{} {
	summary := map[string]interface{}{
		"comparison_kind":                 context.identity.comparisonKind,
		"baseline_name":                   context.identity.baselineName,
		"base_title":                      context.identity.baseTitle,
		"target_title":                    context.identity.targetTitle,
		"base_group_id":                   context.identity.baseGroupID,
		"target_group_id":                 context.identity.targetGroupID,
		"overall_score_delta":             context.metricDelta.overallScoreDelta,
		"pass_rate_delta":                 context.metricDelta.passRateDelta,
		"verification_pass_rate_delta":    context.metricDelta.verificationRateDelta,
		"evidence_backed_pass_rate_delta": context.metricDelta.evidenceBackedRateDelta,
		"retry_recovered_delta":           context.metricDelta.retryRecoveredDelta,
		"changed_case_count":              context.rollup.changedCaseCount,
		"regression_count":                context.rollup.regressionCount,
		"improvement_count":               context.rollup.improvementCount,
		"unstable_case_count":             context.rollup.unstableCaseCount,
		"new_failure_count":               context.rollup.newFailureCount,
		"resolved_failure_count":          context.rollup.resolvedFailureCount,
	}
	if len(context.scorer.failureLabelDelta) > 0 {
		summary["failure_label_delta"] = context.scorer.failureLabelDelta
	}
	if context.scorer.routeCaseCount > 0 {
		summary["route_case_count"] = context.scorer.routeCaseCount
		summary["route_agreement_count"] = context.scorer.routeAgreementCount
		summary["route_agreement_rate"] = context.scorer.routeAgreementRate
		summary["route_compatible_count"] = context.scorer.routeCompatibleCount
		summary["route_compatible_rate"] = context.scorer.routeCompatibleRate
		summary["route_improvement_count"] = context.scorer.routeImprovementCount
		summary["route_disagreement_count"] = context.scorer.routeDisagreementCount
		summary["critical_route_case_count"] = context.scorer.criticalRouteCaseCount
		summary["critical_regression_count"] = context.scorer.criticalRegressionCount
		summary["base_clarify_rate"] = context.scorer.baseClarifyRate
		summary["target_clarify_rate"] = context.scorer.targetClarifyRate
		summary["clarify_rate_delta"] = context.scorer.clarifyRateDelta
		if len(context.scorer.localeBreakdown) > 0 {
			summary["locale_breakdown"] = context.scorer.localeBreakdown
		}
		if len(context.scorer.primaryRouteBreakdown) > 0 {
			summary["primary_route_breakdown"] = context.scorer.primaryRouteBreakdown
		}
	}
	return summary
}

func buildComparisonScorerDelta(bundle *comparisonReportBundle) map[string]interface{} {
	return buildComparisonScorerDeltaFromContext(buildComparisonPresentationContext(nil, bundle))
}

func buildComparisonScorerDeltaFromContext(scorerContext comparisonPresentationContext) map[string]interface{} {
	delta := map[string]interface{}{
		"base_overall_score":               scorerContext.scorer.baseOverallScore,
		"target_overall_score":             scorerContext.scorer.targetOverallScore,
		"overall_score_delta":              scorerContext.scorer.overallScoreDelta,
		"base_pass_rate":                   scorerContext.scorer.basePassRate,
		"target_pass_rate":                 scorerContext.scorer.targetPassRate,
		"pass_rate_delta":                  scorerContext.scorer.passRateDelta,
		"base_verification_pass_rate":      scorerContext.scorer.baseVerificationPassRate,
		"target_verification_pass_rate":    scorerContext.scorer.targetVerificationPassRate,
		"verification_pass_rate_delta":     scorerContext.scorer.verificationRateDelta,
		"base_evidence_backed_pass_rate":   scorerContext.scorer.baseEvidenceBackedPassRate,
		"target_evidence_backed_pass_rate": scorerContext.scorer.targetEvidenceBackedPassRate,
		"evidence_backed_pass_rate_delta":  scorerContext.scorer.evidenceBackedRateDelta,
		"base_retry_recovered_count":       scorerContext.scorer.baseRetryRecoveredCount,
		"target_retry_recovered_count":     scorerContext.scorer.targetRetryRecoveredCount,
		"retry_recovered_delta":            scorerContext.scorer.retryRecoveredDelta,
		"failure_label_delta":              scorerContext.scorer.failureLabelDelta,
		"verdict_count_delta":              scorerContext.scorer.verdictCountDelta,
		"linked_run_count_delta":           scorerContext.scorer.linkedRunCountDelta,
		"artifact_count_delta":             scorerContext.scorer.artifactCountDelta,
	}
	if scorerContext.scorer.routeCaseCount > 0 {
		delta["route_case_count"] = scorerContext.scorer.routeCaseCount
		delta["route_agreement_count"] = scorerContext.scorer.routeAgreementCount
		delta["route_agreement_rate"] = scorerContext.scorer.routeAgreementRate
		delta["route_compatible_count"] = scorerContext.scorer.routeCompatibleCount
		delta["route_compatible_rate"] = scorerContext.scorer.routeCompatibleRate
		delta["route_improvement_count"] = scorerContext.scorer.routeImprovementCount
		delta["route_disagreement_count"] = scorerContext.scorer.routeDisagreementCount
		delta["critical_route_case_count"] = scorerContext.scorer.criticalRouteCaseCount
		delta["critical_regression_count"] = scorerContext.scorer.criticalRegressionCount
		delta["base_clarify_rate"] = scorerContext.scorer.baseClarifyRate
		delta["target_clarify_rate"] = scorerContext.scorer.targetClarifyRate
		delta["clarify_rate_delta"] = scorerContext.scorer.clarifyRateDelta
		if len(scorerContext.scorer.localeBreakdown) > 0 {
			delta["locale_breakdown"] = scorerContext.scorer.localeBreakdown
		}
		if len(scorerContext.scorer.primaryRouteBreakdown) > 0 {
			delta["primary_route_breakdown"] = scorerContext.scorer.primaryRouteBreakdown
		}
	}
	return delta
}

func buildComparisonReportSections(baseline *Baseline, bundle *comparisonReportBundle) comparisonReportSections {
	context := buildComparisonPresentationContext(baseline, bundle)
	return comparisonReportSections{
		summary:     buildComparisonSummaryFromContext(context),
		body:        context.body,
		scorerDelta: buildComparisonScorerDeltaFromContext(context),
	}
}

func normalizeComparisonReportBundle(bundle *comparisonReportBundle) *comparisonReportBundle {
	if bundle == nil {
		return buildComparisonReportBundle(nil, nil)
	}
	return bundle
}

func buildComparisonReportBody(rollup comparisonCaseRollup) comparisonReportBody {
	return comparisonReportBody{
		regressions:  rollup.regressions,
		improvements: rollup.improvements,
	}
}

func buildComparisonPresentationContext(baseline *Baseline, bundle *comparisonReportBundle) comparisonPresentationContext {
	bundle = normalizeComparisonReportBundle(bundle)
	return comparisonPresentationContext{
		identity:    buildComparisonSummaryIdentity(baseline, bundle),
		rollup:      buildComparisonSummaryRollup(bundle.rollup),
		metricDelta: buildComparisonMetricDeltaSet(bundle.metrics),
		scorer:      buildComparisonScorerDeltaContext(bundle.metrics),
		body:        buildComparisonReportBody(bundle.rollup),
	}
}

func buildComparisonSummaryIdentity(baseline *Baseline, bundle *comparisonReportBundle) comparisonSummaryIdentity {
	bundle = normalizeComparisonReportBundle(bundle)
	return comparisonSummaryIdentity{
		comparisonKind: comparisonKind(baseline),
		baselineName:   baselineName(baseline, bundle.baseView.title()),
		baseTitle:      bundle.baseView.title(),
		targetTitle:    bundle.targetView.title(),
		baseGroupID:    bundle.baseView.groupID(),
		targetGroupID:  bundle.targetView.groupID(),
	}
}

func buildComparisonSummaryRollup(rollup comparisonCaseRollup) comparisonSummaryRollup {
	return comparisonSummaryRollup{
		changedCaseCount:     rollup.changedCases,
		regressionCount:      len(rollup.regressions),
		improvementCount:     len(rollup.improvements),
		unstableCaseCount:    rollup.unstableCases,
		newFailureCount:      rollup.newFailures,
		resolvedFailureCount: rollup.resolvedFailures,
	}
}

func buildComparisonScorerDeltaContext(metrics comparisonMetricBundle) comparisonScorerDeltaContext {
	metricDeltas := buildComparisonMetricDeltaSet(metrics)
	routeAgreementRate := safeComparisonRate(metrics.structural.routeAgreementCount, metrics.structural.routeCaseCount)
	routeCompatibleRate := safeComparisonRate(metrics.structural.routeCompatibleCount, metrics.structural.routeCaseCount)
	baseClarifyRate := safeComparisonRate(metrics.structural.baseClarifyCount, metrics.structural.routeCaseCount)
	targetClarifyRate := safeComparisonRate(metrics.structural.targetClarifyCount, metrics.structural.routeCaseCount)
	return comparisonScorerDeltaContext{
		baseOverallScore:             metrics.base.overallScore,
		targetOverallScore:           metrics.target.overallScore,
		overallScoreDelta:            metricDeltas.overallScoreDelta,
		basePassRate:                 metrics.base.passRate,
		targetPassRate:               metrics.target.passRate,
		passRateDelta:                metricDeltas.passRateDelta,
		baseVerificationPassRate:     metrics.base.verificationPassRate,
		targetVerificationPassRate:   metrics.target.verificationPassRate,
		verificationRateDelta:        metricDeltas.verificationRateDelta,
		baseEvidenceBackedPassRate:   metrics.base.evidenceBackedPassRate,
		targetEvidenceBackedPassRate: metrics.target.evidenceBackedPassRate,
		evidenceBackedRateDelta:      metricDeltas.evidenceBackedRateDelta,
		baseRetryRecoveredCount:      metrics.base.retryRecoveredCount,
		targetRetryRecoveredCount:    metrics.target.retryRecoveredCount,
		retryRecoveredDelta:          metricDeltas.retryRecoveredDelta,
		failureLabelDelta:            metrics.structural.failureLabelDelta,
		verdictCountDelta:            metrics.structural.verdictCountDelta,
		linkedRunCountDelta:          metrics.structural.linkedRunCountDelta,
		artifactCountDelta:           metrics.structural.artifactCountDelta,
		routeCaseCount:               metrics.structural.routeCaseCount,
		routeAgreementCount:          metrics.structural.routeAgreementCount,
		routeAgreementRate:           routeAgreementRate,
		routeCompatibleCount:         metrics.structural.routeCompatibleCount,
		routeCompatibleRate:          routeCompatibleRate,
		routeImprovementCount:        metrics.structural.routeImprovementCount,
		routeDisagreementCount:       metrics.structural.routeDisagreementCount,
		criticalRouteCaseCount:       metrics.structural.criticalRouteCaseCount,
		criticalRegressionCount:      metrics.structural.criticalRegressionCount,
		baseClarifyRate:              baseClarifyRate,
		targetClarifyRate:            targetClarifyRate,
		clarifyRateDelta:             targetClarifyRate - baseClarifyRate,
		localeBreakdown:              cloneSelectorGateSegmentMetricsMap(metrics.structural.localeBreakdown),
		primaryRouteBreakdown:        cloneSelectorGateSegmentMetricsMap(metrics.structural.primaryRouteBreakdown),
	}
}

func buildComparisonMetricDeltaSet(metrics comparisonMetricBundle) comparisonMetricDeltaSet {
	return comparisonMetricDeltaSet{
		overallScoreDelta:       metrics.target.overallScore - metrics.base.overallScore,
		passRateDelta:           metrics.target.passRate - metrics.base.passRate,
		verificationRateDelta:   metrics.target.verificationPassRate - metrics.base.verificationPassRate,
		evidenceBackedRateDelta: metrics.target.evidenceBackedPassRate - metrics.base.evidenceBackedPassRate,
		retryRecoveredDelta:     metrics.target.retryRecoveredCount - metrics.base.retryRecoveredCount,
	}
}

func buildComparisonViewMetricSnapshot(view *comparisonReportView) comparisonViewMetricSnapshot {
	return comparisonViewMetricSnapshot{
		overallScore:           view.overallScore(),
		passRate:               view.passRate(),
		verificationPassRate:   view.verificationPassRate(),
		evidenceBackedPassRate: view.evidenceBackedPassRate(),
		retryRecoveredCount:    view.retryRecoveredCount(),
		linkedRunCount:         view.linkedRunCount(),
		artifactCount:          view.artifactCount(),
	}
}

func buildComparisonStructuralDelta(baseView *comparisonReportView, targetView *comparisonReportView, baseMetrics comparisonViewMetricSnapshot, targetMetrics comparisonViewMetricSnapshot) comparisonStructuralDelta {
	routeDelta := buildComparisonRouteDelta(baseView, targetView)
	return comparisonStructuralDelta{
		failureLabelDelta:       failureLabelDeltaViews(baseView, targetView),
		verdictCountDelta:       verdictCountDeltaViews(baseView, targetView),
		linkedRunCountDelta:     targetMetrics.linkedRunCount - baseMetrics.linkedRunCount,
		artifactCountDelta:      targetMetrics.artifactCount - baseMetrics.artifactCount,
		routeCaseCount:          routeDelta.routeCaseCount,
		routeAgreementCount:     routeDelta.routeAgreementCount,
		routeCompatibleCount:    routeDelta.routeCompatibleCount,
		routeImprovementCount:   routeDelta.routeImprovementCount,
		routeDisagreementCount:  routeDelta.routeDisagreementCount,
		criticalRouteCaseCount:  routeDelta.criticalRouteCaseCount,
		criticalRegressionCount: routeDelta.criticalRegressionCount,
		baseClarifyCount:        routeDelta.baseClarifyCount,
		targetClarifyCount:      routeDelta.targetClarifyCount,
		localeBreakdown:         finalizeComparisonRouteBreakdownMap(routeDelta.localeBreakdown),
		primaryRouteBreakdown:   finalizeComparisonRouteBreakdownMap(routeDelta.primaryRouteBreakdown),
	}
}

func buildComparisonRouteDelta(baseView *comparisonReportView, targetView *comparisonReportView) comparisonRouteDelta {
	baseCases := baseView.caseSnapshots()
	targetCases := targetView.caseSnapshots()
	keys := sortedCaseKeys(baseCases, targetCases)
	delta := comparisonRouteDelta{
		localeBreakdown:       make(map[string]*comparisonRouteBreakdown),
		primaryRouteBreakdown: make(map[string]*comparisonRouteBreakdown),
	}
	for _, key := range keys {
		baseCase, baseOK := baseCases[key]
		targetCase, targetOK := targetCases[key]
		compareFields := comparisonRouteFields(baseCase, baseOK, targetCase, targetOK)
		if len(compareFields) == 0 {
			continue
		}
		delta.routeCaseCount++
		baseClarify := comparisonNeedsClarify(baseCase, baseOK)
		targetClarify := comparisonNeedsClarify(targetCase, targetOK)
		if baseClarify {
			delta.baseClarifyCount++
		}
		if targetClarify {
			delta.targetClarifyCount++
		}
		critical := comparisonRouteCaseCritical(baseCase, baseOK, targetCase, targetOK)
		if critical {
			delta.criticalRouteCaseCount++
		}
		matched := comparisonRouteMatches(compareFields, baseCase, baseOK, targetCase, targetOK)
		assessment := assessComparisonCase(baseCase, baseOK, targetCase, targetOK)
		classification, changed := classifyComparisonCaseAssessment(assessment)
		if matched {
			delta.routeAgreementCount++
			delta.routeCompatibleCount++
		} else {
			delta.routeDisagreementCount++
			if changed && classification == "improvement" {
				delta.routeImprovementCount++
				delta.routeCompatibleCount++
			}
			if critical && (!changed || classification != "improvement") {
				delta.criticalRegressionCount++
			}
		}
		recordComparisonRouteBreakdown(delta.localeBreakdown, comparisonRouteBreakdownKey(baseCase.Locale, targetCase.Locale), matched, changed, classification, critical, baseClarify, targetClarify)
		recordComparisonRouteBreakdown(delta.primaryRouteBreakdown, comparisonRouteBreakdownKey(baseCase.PrimaryRoute, targetCase.PrimaryRoute), matched, changed, classification, critical, baseClarify, targetClarify)
	}
	return delta
}

func comparisonRouteFields(baseCase evalCaseSnapshot, baseOK bool, targetCase evalCaseSnapshot, targetOK bool) []string {
	if targetOK && len(targetCase.CompareFields) > 0 {
		return append([]string(nil), targetCase.CompareFields...)
	}
	if baseOK && len(baseCase.CompareFields) > 0 {
		return append([]string(nil), baseCase.CompareFields...)
	}
	return nil
}

func comparisonRouteCaseCritical(baseCase evalCaseSnapshot, baseOK bool, targetCase evalCaseSnapshot, targetOK bool) bool {
	return (baseOK && baseCase.Critical) || (targetOK && targetCase.Critical)
}

func comparisonNeedsClarify(snapshot evalCaseSnapshot, ok bool) bool {
	if !ok {
		return false
	}
	return comparisonStructuredBool(snapshot.Structured, "skill_need_clarify")
}

func comparisonRouteMatches(compareFields []string, baseCase evalCaseSnapshot, baseOK bool, targetCase evalCaseSnapshot, targetOK bool) bool {
	if !baseOK || !targetOK {
		return false
	}
	for _, field := range compareFields {
		if !comparisonStructuredFieldEqual(baseCase.Structured, targetCase.Structured, field) {
			return false
		}
	}
	return true
}

func comparisonRouteBreakdownKey(baseValue, targetValue string) string {
	value := strings.TrimSpace(targetValue)
	if value != "" {
		return value
	}
	value = strings.TrimSpace(baseValue)
	if value != "" {
		return value
	}
	return "unknown"
}

func recordComparisonRouteBreakdown(breakdowns map[string]*comparisonRouteBreakdown, key string, matched bool, changed bool, classification string, critical bool, baseClarify bool, targetClarify bool) {
	if breakdowns == nil {
		return
	}
	key = strings.TrimSpace(key)
	if key == "" {
		key = "unknown"
	}
	bucket, ok := breakdowns[key]
	if !ok {
		bucket = &comparisonRouteBreakdown{}
		breakdowns[key] = bucket
	}

	bucket.caseCount++
	if baseClarify {
		bucket.baseClarifyCount++
	}
	if targetClarify {
		bucket.targetClarifyCount++
	}
	if critical {
		bucket.criticalCaseCount++
	}
	if matched {
		bucket.routeAgreementCount++
		bucket.routeCompatibleCount++
		return
	}

	bucket.routeDisagreementCount++
	if changed && classification == "improvement" {
		bucket.routeImprovementCount++
		bucket.routeCompatibleCount++
		return
	}
	if critical {
		bucket.criticalRegressionCount++
	}
}

func finalizeComparisonRouteBreakdownMap(raw map[string]*comparisonRouteBreakdown) map[string]SelectorGateSegmentMetrics {
	if len(raw) == 0 {
		return nil
	}
	out := make(map[string]SelectorGateSegmentMetrics, len(raw))
	for key, bucket := range raw {
		if bucket == nil {
			continue
		}
		baseClarifyRate := safeComparisonRate(bucket.baseClarifyCount, bucket.caseCount)
		targetClarifyRate := safeComparisonRate(bucket.targetClarifyCount, bucket.caseCount)
		out[key] = SelectorGateSegmentMetrics{
			CaseCount:               bucket.caseCount,
			RouteAgreementCount:     bucket.routeAgreementCount,
			RouteAgreementRate:      safeComparisonRate(bucket.routeAgreementCount, bucket.caseCount),
			RouteCompatibleCount:    bucket.routeCompatibleCount,
			RouteCompatibleRate:     safeComparisonRate(bucket.routeCompatibleCount, bucket.caseCount),
			RouteImprovementCount:   bucket.routeImprovementCount,
			RouteDisagreementCount:  bucket.routeDisagreementCount,
			CriticalCaseCount:       bucket.criticalCaseCount,
			CriticalRegressionCount: bucket.criticalRegressionCount,
			BaseClarifyCount:        bucket.baseClarifyCount,
			BaseClarifyRate:         baseClarifyRate,
			TargetClarifyCount:      bucket.targetClarifyCount,
			TargetClarifyRate:       targetClarifyRate,
			ClarifyRateDelta:        targetClarifyRate - baseClarifyRate,
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func cloneSelectorGateSegmentMetricsMap(raw map[string]SelectorGateSegmentMetrics) map[string]SelectorGateSegmentMetrics {
	if len(raw) == 0 {
		return nil
	}
	out := make(map[string]SelectorGateSegmentMetrics, len(raw))
	for key, value := range raw {
		out[key] = value
	}
	return out
}

func comparisonStructuredFieldEqual(base map[string]interface{}, target map[string]interface{}, field string) bool {
	baseValue, baseOK := comparisonStructuredValue(base, field)
	targetValue, targetOK := comparisonStructuredValue(target, field)
	switch {
	case !baseOK && !targetOK:
		return true
	case baseOK != targetOK:
		return false
	default:
		return reflect.DeepEqual(normalizeComparisonStructuredValue(baseValue), normalizeComparisonStructuredValue(targetValue))
	}
}

func comparisonStructuredValue(meta map[string]interface{}, field string) (interface{}, bool) {
	if len(meta) == 0 {
		return nil, false
	}
	value, ok := meta[field]
	return value, ok
}

func normalizeComparisonStructuredValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case nil:
		return nil
	case string:
		return normalizeText(typed)
	case bool:
		return typed
	case int:
		return typed
	case int8:
		return int(typed)
	case int16:
		return int(typed)
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float32:
		return float64(typed)
	case float64:
		return typed
	case []string:
		out := make([]interface{}, 0, len(typed))
		for _, item := range typed {
			out = append(out, normalizeComparisonStructuredValue(item))
		}
		return out
	case []interface{}:
		out := make([]interface{}, 0, len(typed))
		for _, item := range typed {
			out = append(out, normalizeComparisonStructuredValue(item))
		}
		return out
	case map[string]interface{}:
		out := make(map[string]interface{}, len(typed))
		for key, item := range typed {
			out[key] = normalizeComparisonStructuredValue(item)
		}
		return out
	default:
		return normalizeText(fmt.Sprint(value))
	}
}

func comparisonStructuredBool(meta map[string]interface{}, field string) bool {
	if len(meta) == 0 {
		return false
	}
	value, ok := meta[field]
	if !ok {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	default:
		return false
	}
}

func metadataBoolValue(meta map[string]interface{}, key string) bool {
	if len(meta) == 0 {
		return false
	}
	value, ok := meta[key]
	if !ok {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	default:
		return false
	}
}

func safeComparisonRate(count int, total int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(count) / float64(total)
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
	assessment := assessComparisonCase(base, hasBase, target, hasTarget)
	classification, changed := classifyComparisonCaseAssessment(assessment)
	return assessment.delta, classification, changed
}

func assessComparisonCase(base evalCaseSnapshot, hasBase bool, target evalCaseSnapshot, hasTarget bool) comparisonCaseAssessment {
	delta := buildComparisonCaseDelta(base, hasBase, target, hasTarget)
	return comparisonCaseAssessment{
		delta:      delta,
		hasBase:    hasBase,
		hasTarget:  hasTarget,
		baseRank:   caseRankForVerdict(delta.BaseVerdict),
		targetRank: caseRankForVerdict(delta.TargetVerdict),
	}
}

func buildComparisonCaseDelta(base evalCaseSnapshot, hasBase bool, target evalCaseSnapshot, hasTarget bool) ComparisonCaseDelta {
	return ComparisonCaseDelta{
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
}

func (a comparisonCaseAssessment) classifyMissingBoundary() string {
	switch {
	case !a.hasBase && a.hasTarget && isFailingComparisonVerdict(a.delta.TargetVerdict):
		return "regression"
	case a.hasBase && !a.hasTarget && isFailingComparisonVerdict(a.delta.BaseVerdict):
		return "improvement"
	default:
		return "unstable"
	}
}

func classifyComparisonCaseAssessment(assessment comparisonCaseAssessment) (string, bool) {
	if !assessment.hasBase || !assessment.hasTarget {
		return assessment.classifyMissingBoundary(), true
	}
	if assessment.isUnchanged() {
		return "", false
	}
	if assessment.targetRank < assessment.baseRank {
		return "regression", true
	}
	if assessment.targetRank > assessment.baseRank {
		return "improvement", true
	}
	return "unstable", true
}

func applyComparisonCaseRollupChange(rollup *comparisonCaseRollup, delta ComparisonCaseDelta, classification string) {
	if rollup == nil {
		return
	}
	rollup.changedCases++
	switch classification {
	case "regression":
		rollup.regressions = append(rollup.regressions, delta)
		if isFailingComparisonVerdict(delta.TargetVerdict) && !isFailingComparisonVerdict(delta.BaseVerdict) {
			rollup.newFailures++
		}
	case "improvement":
		rollup.improvements = append(rollup.improvements, delta)
		if !isFailingComparisonVerdict(delta.TargetVerdict) && isFailingComparisonVerdict(delta.BaseVerdict) {
			rollup.resolvedFailures++
		}
	default:
		rollup.unstableCases++
	}
}

func (a comparisonCaseAssessment) isUnchanged() bool {
	return a.baseRank == a.targetRank &&
		a.delta.BaseVerdict == a.delta.TargetVerdict &&
		a.delta.BaseStatus == a.delta.TargetStatus &&
		!comparisonFloatChanged(a.delta.BaseScore, a.delta.TargetScore) &&
		!comparisonFloatChanged(a.delta.BaseEvidenceScore, a.delta.TargetEvidenceScore) &&
		a.delta.BaseVerification == a.delta.TargetVerification &&
		a.delta.BaseFailureLabel == a.delta.TargetFailureLabel
}

func comparisonFloatChanged(base float64, target float64) bool {
	return math.Abs(target-base) >= 0.0001
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
	return caseRankForVerdict(normalizedVerdict(snapshot, ok))
}

func caseRankForVerdict(verdict string) int {
	switch strings.TrimSpace(verdict) {
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
	return isFailingComparisonVerdict(normalizedVerdict(snapshot, ok))
}

func isFailingComparisonVerdict(verdict string) bool {
	switch strings.TrimSpace(verdict) {
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

func failureLabelDeltaViews(baseView *comparisonReportView, targetView *comparisonReportView) map[string]interface{} {
	baseCounts := baseView.failureLabelCounts()
	targetCounts := targetView.failureLabelCounts()
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

func verdictCountDeltaViews(baseView *comparisonReportView, targetView *comparisonReportView) map[string]interface{} {
	keys := make(map[string]struct{})
	out := make(map[string]interface{})
	for key := range baseView.verdictCounts() {
		keys[key] = struct{}{}
	}
	for key := range targetView.verdictCounts() {
		keys[key] = struct{}{}
	}
	for key := range keys {
		baseCount := baseView.verdictCounts()[key]
		targetCount := targetView.verdictCounts()[key]
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

func baselineName(baseline *Baseline, fallback string) string {
	if baseline != nil && strings.TrimSpace(baseline.Name) != "" {
		return baseline.Name
	}
	return strings.TrimSpace(fallback)
}

func baselineID(baseline *Baseline) string {
	if baseline == nil {
		return ""
	}
	return baseline.ID
}

func firstNonZero(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func (v *comparisonReportView) caseSnapshots() map[string]evalCaseSnapshot {
	if v == nil {
		return map[string]evalCaseSnapshot{}
	}
	if v.caseSnapshotByKey != nil {
		return v.caseSnapshotByKey
	}
	out := make(map[string]evalCaseSnapshot)
	if v.report == nil || v.report.GroupReport == nil {
		v.caseSnapshotByKey = out
		return out
	}
	for _, item := range v.report.GroupReport.Items {
		snapshot := buildComparisonCaseSnapshot(item)
		if card, ok := v.latestCards[item.ID]; ok {
			snapshot = buildComparisonCaseSnapshotFromScorecard(snapshot, card)
			snapshot.Reason = comparisonReason(&card, v.runByID[snapshot.RunID])
		}
		out[snapshot.Key] = finalizeComparisonCaseSnapshot(snapshot, item, v.runByID[snapshot.RunID])
	}
	v.caseSnapshotByKey = out
	return out
}

func buildComparisonCaseSnapshot(item RunGroupItem) evalCaseSnapshot {
	key := comparisonCaseKey(item)
	return evalCaseSnapshot{
		Key:           key,
		Label:         comparisonCaseLabel(item, key),
		ItemIndex:     item.Index,
		Profile:       item.Profile,
		Locale:        metadataString(item.Metadata, "locale"),
		PrimaryRoute:  metadataString(item.Metadata, "primary_route"),
		Status:        string(item.Status),
		RunID:         strings.TrimSpace(item.LatestRunID),
		CompareFields: decodeStringSlice(item.Metadata["compare_fields"]),
		Critical:      metadataBoolValue(item.Metadata, "critical"),
	}
}

func comparisonCaseKey(item RunGroupItem) string {
	if key := metadataString(item.Metadata, "dataset_case_id"); key != "" {
		return key
	}
	return fmt.Sprintf("item-%d", item.Index)
}

func comparisonCaseLabel(item RunGroupItem, fallback string) string {
	return firstNonEmpty(metadataString(item.Input, "goal"), fallback)
}

func buildComparisonCaseSnapshotFromScorecard(snapshot evalCaseSnapshot, card Scorecard) evalCaseSnapshot {
	breakdown := decodeJSONMap(card.BreakdownJSON)
	snapshot.Verdict = string(card.Verdict)
	snapshot.Score = card.Score
	snapshot.FailureLabel = metadataString(breakdown, "failure_label")
	snapshot.Verification = comparisonVerificationStatus(breakdown)
	snapshot.EvidenceScore = comparisonNumericValue(breakdown["evidence_score"])
	if runID := strings.TrimSpace(card.RunID); runID != "" {
		snapshot.RunID = runID
	}
	return snapshot
}

func finalizeComparisonCaseSnapshot(snapshot evalCaseSnapshot, item RunGroupItem, run Run) evalCaseSnapshot {
	if snapshot.Verdict == "" {
		snapshot.Verdict = verdictFromItemStatus(item.Status)
	}
	if structured := structuredRunResult(&run); len(structured) > 0 {
		snapshot.Structured = structured
	}
	if snapshot.Reason == "" {
		snapshot.Reason = comparisonReason(nil, run)
	}
	return snapshot
}

func (v *comparisonReportView) overallScore() float64 {
	if v == nil || v.report == nil || v.report.GroupReport == nil {
		return 0
	}
	return v.report.GroupReport.OverallScore
}

func (v *comparisonReportView) passRate() float64 {
	if v == nil || v.report == nil || v.report.GroupReport == nil {
		return 0
	}
	return v.report.GroupReport.PassRate
}

func (v *comparisonReportView) verificationPassRate() float64 {
	return v.derivedViewMetrics().verificationPassRate
}

func (v *comparisonReportView) evidenceBackedPassRate() float64 {
	return v.derivedViewMetrics().evidenceBackedRate
}

func (v *comparisonReportView) retryRecoveredCount() int {
	return v.derivedViewMetrics().retryRecoveredCount
}

func (v *comparisonReportView) groupID() string {
	if v == nil || v.report == nil || v.report.GroupReport == nil || v.report.GroupReport.Group == nil {
		return ""
	}
	return v.report.GroupReport.Group.ID
}

func (v *comparisonReportView) linkedRunCount() int {
	if v == nil || v.report == nil || v.report.GroupReport == nil {
		return 0
	}
	return len(v.report.GroupReport.LinkedRuns)
}

func (v *comparisonReportView) artifactCount() int {
	if v == nil || v.report == nil || v.report.GroupReport == nil {
		return 0
	}
	return len(v.report.GroupReport.Artifacts)
}

func (v *comparisonReportView) title() string {
	if v == nil || v.report == nil || v.report.EvalRun == nil {
		return ""
	}
	return firstNonEmpty(v.report.EvalRun.Title, v.report.EvalRun.ID)
}

func (v *comparisonReportView) verdictCounts() map[string]int {
	return v.derivedViewMetrics().verdictCounts
}

func (v *comparisonReportView) failureLabelCounts() map[string]int {
	return v.derivedViewMetrics().failureLabelCounts
}

func (v *comparisonReportView) derivedViewMetrics() *comparisonViewDerivedMetrics {
	if v == nil {
		return &comparisonViewDerivedMetrics{
			verdictCounts:      map[string]int{},
			failureLabelCounts: map[string]int{},
		}
	}
	if v.derivedMetrics != nil {
		return v.derivedMetrics
	}
	metrics := &comparisonViewDerivedMetrics{
		verdictCounts:      map[string]int{},
		failureLabelCounts: map[string]int{},
	}
	if v.report == nil || v.report.GroupReport == nil {
		v.derivedMetrics = metrics
		return metrics
	}
	if v.report.GroupReport.VerdictCounts != nil {
		metrics.verdictCounts = v.report.GroupReport.VerdictCounts
	}
	if value, ok := v.summaryFloat("verification_pass_rate"); ok {
		metrics.verificationPassRate = value
	} else {
		metrics.verificationPassRate = computeComparisonVerificationPassRate(v.report.GroupReport.Items, v.latestCards)
	}
	if value, ok := v.summaryFloat("evidence_backed_pass_rate"); ok {
		metrics.evidenceBackedRate = value
	} else {
		metrics.evidenceBackedRate = computeComparisonEvidenceBackedPassRate(v.report.GroupReport.Items, v.latestCards)
	}
	if value, ok := v.summaryInt("retry_recovered_count"); ok {
		metrics.retryRecoveredCount = value
	} else {
		metrics.retryRecoveredCount = computeComparisonRetryRecoveredCount(v.report.GroupReport.Items, v.latestCards)
	}
	if value, ok := v.summaryIntMap("failure_label_counts"); ok {
		metrics.failureLabelCounts = value
	} else {
		metrics.failureLabelCounts = computeComparisonFailureLabelCounts(v.report.GroupReport.Items, v.latestCards)
	}
	v.derivedMetrics = metrics
	return metrics
}

func (v *comparisonReportView) summaryFloat(key string) (float64, bool) {
	raw, ok := v.summaryValue(key)
	if !ok {
		return 0, false
	}
	return comparisonNumericValue(raw), true
}

func (v *comparisonReportView) summaryInt(key string) (int, bool) {
	raw, ok := v.summaryValue(key)
	if !ok {
		return 0, false
	}
	return int(comparisonNumericValue(raw)), true
}

func (v *comparisonReportView) summaryIntMap(key string) (map[string]int, bool) {
	raw, ok := v.summaryValue(key)
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

func (v *comparisonReportView) summaryValue(key string) (interface{}, bool) {
	if v == nil || len(v.summary) == 0 {
		return nil, false
	}
	raw, ok := v.summary[key]
	return raw, ok
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

func computeComparisonVerificationPassRate(items []RunGroupItem, latestCards map[string]Scorecard) float64 {
	if len(items) == 0 || len(latestCards) == 0 {
		return 0
	}
	passed := 0
	total := 0
	for _, item := range items {
		card, ok := latestCards[item.ID]
		if !ok {
			continue
		}
		if executionEquivalenceItemInfraBlocked(item, card) {
			continue
		}
		total++
		if verified, ok := mapBool(decodeJSONMap(card.BreakdownJSON), "verification_passed"); ok && verified {
			passed++
		}
	}
	if total == 0 {
		return 0
	}
	return float64(passed) / float64(total)
}

func computeComparisonEvidenceBackedPassRate(items []RunGroupItem, latestCards map[string]Scorecard) float64 {
	if len(items) == 0 || len(latestCards) == 0 {
		return 0
	}
	passCount := 0
	evidenceBacked := 0
	for _, item := range items {
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

func computeComparisonRetryRecoveredCount(items []RunGroupItem, latestCards map[string]Scorecard) int {
	if len(items) == 0 || len(latestCards) == 0 {
		return 0
	}
	recovered := 0
	for _, item := range items {
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

func computeComparisonFailureLabelCounts(items []RunGroupItem, latestCards map[string]Scorecard) map[string]int {
	out := map[string]int{}
	if len(items) == 0 || len(latestCards) == 0 {
		return out
	}
	for _, item := range items {
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
