package harness

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

const (
	defaultSkillCutoverRequiredConsecutiveRuns = 2
	defaultSkillCutoverAssessmentLimit         = 5
	defaultSkillCutoverRunListLimit            = 50

	skillCutoverCandidateMetadataKey = "candidate_id"
)

type SkillCutoverReadinessRequest struct {
	OwnerUserID             string                      `json:"owner_user_id,omitempty"`
	CandidateID             string                      `json:"candidate_id,omitempty"`
	SelectorEvalSpecID      string                      `json:"selector_eval_spec_id,omitempty"`
	ExecutionEvalSpecID     string                      `json:"execution_eval_spec_id,omitempty"`
	RequiredConsecutiveRuns int                         `json:"required_consecutive_runs,omitempty"`
	MaxAssessments          int                         `json:"max_assessments,omitempty"`
	Selector                SelectorGateRequest         `json:"selector,omitempty"`
	Execution               ExecutionEquivalenceRequest `json:"execution,omitempty"`
	Budget                  SkillCutoverBudgetRequest   `json:"budget,omitempty"`
}

type SkillCutoverLaneAssessment struct {
	EvalRunID          string    `json:"eval_run_id"`
	CandidateID        string    `json:"candidate_id,omitempty"`
	Title              string    `json:"title,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	Passed             bool      `json:"passed"`
	BaseEvalRunID      string    `json:"base_eval_run_id,omitempty"`
	BaselineID         string    `json:"baseline_id,omitempty"`
	ComparisonReportID string    `json:"comparison_report_id,omitempty"`
	FailedChecks       []string  `json:"failed_checks,omitempty"`
}

type SkillCutoverLaneReadiness struct {
	EvalSpecID              string                       `json:"eval_spec_id,omitempty"`
	CandidateID             string                       `json:"candidate_id,omitempty"`
	RequiredConsecutiveRuns int                          `json:"required_consecutive_runs"`
	CandidateRunCount       int                          `json:"candidate_run_count"`
	ConsecutivePassCount    int                          `json:"consecutive_pass_count"`
	Ready                   bool                         `json:"ready"`
	BaseEvalRunID           string                       `json:"base_eval_run_id,omitempty"`
	BaselineID              string                       `json:"baseline_id,omitempty"`
	Error                   string                       `json:"error,omitempty"`
	Assessments             []SkillCutoverLaneAssessment `json:"assessments,omitempty"`
}

type SkillCutoverReadinessReport struct {
	CandidateID             string                    `json:"candidate_id,omitempty"`
	RequiredConsecutiveRuns int                       `json:"required_consecutive_runs"`
	EvaluatedGatesReady     bool                      `json:"evaluated_gates_ready"`
	Ready                   bool                      `json:"ready"`
	Selector                SkillCutoverLaneReadiness `json:"selector"`
	Execution               SkillCutoverLaneReadiness `json:"execution"`
	Budget                  SkillCutoverLaneReadiness `json:"budget"`
	UnverifiedRequirements  []string                  `json:"unverified_requirements,omitempty"`
	BlockingReasons         []string                  `json:"blocking_reasons,omitempty"`
	CreatedAt               time.Time                 `json:"created_at"`
}

func (c *Controller) EvaluateSkillCutoverReadiness(ctx context.Context, req SkillCutoverReadinessRequest) (*SkillCutoverReadinessReport, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}

	ownerUserID := strings.TrimSpace(req.OwnerUserID)
	if ownerUserID == "" {
		return nil, fmt.Errorf("owner_user_id is required")
	}

	requiredConsecutive := req.RequiredConsecutiveRuns
	if requiredConsecutive <= 0 {
		requiredConsecutive = defaultSkillCutoverRequiredConsecutiveRuns
	}
	maxAssessments := req.MaxAssessments
	if maxAssessments <= 0 {
		maxAssessments = defaultSkillCutoverAssessmentLimit
	}
	runListLimit := defaultSkillCutoverRunListLimit
	if maxAssessments > runListLimit {
		runListLimit = maxAssessments
	}

	selectorEvalSpecID, executionEvalSpecID, err := c.resolveSkillCutoverEvalSpecIDs(ctx, ownerUserID, req)
	if err != nil {
		return nil, err
	}

	selectorRuns, err := c.ListEvalRuns(ctx, EvalRunFilter{
		OwnerUserID: ownerUserID,
		EvalSpecID:  selectorEvalSpecID,
		Statuses:    skillCutoverTerminalStatuses(),
		Limit:       runListLimit,
	})
	if err != nil {
		return nil, err
	}
	selectorRuns = sortSkillCutoverEvalRuns(selectorRuns)
	executionRuns, err := c.ListEvalRuns(ctx, EvalRunFilter{
		OwnerUserID: ownerUserID,
		EvalSpecID:  executionEvalSpecID,
		Statuses:    skillCutoverTerminalStatuses(),
		Limit:       runListLimit,
	})
	if err != nil {
		return nil, err
	}
	executionRuns = sortSkillCutoverEvalRuns(executionRuns)

	candidateID := strings.TrimSpace(req.CandidateID)
	if candidateID == "" {
		candidateID = resolveSkillCutoverCandidateID(selectorRuns, executionRuns)
	}

	report := &SkillCutoverReadinessReport{
		CandidateID:             candidateID,
		RequiredConsecutiveRuns: requiredConsecutive,
		CreatedAt:               timeutil.NowTime(),
	}

	report.Selector = c.evaluateSkillCutoverSelectorLane(ctx, selectorEvalSpecID, candidateID, selectorRuns, req.Selector, requiredConsecutive, maxAssessments)
	report.Execution = c.evaluateSkillCutoverExecutionLane(ctx, executionEvalSpecID, candidateID, executionRuns, req.Execution, requiredConsecutive, maxAssessments)
	report.Budget = c.evaluateSkillCutoverBudgetLane(ctx, selectorEvalSpecID, candidateID, selectorRuns, resolveSkillCutoverBudgetRequest(req.Selector, req.Budget), requiredConsecutive, maxAssessments)

	if candidateID == "" {
		report.BlockingReasons = append(report.BlockingReasons, "no candidate_id found on recent selector or execution eval runs")
	}
	if candidateID != "" {
		appendSkillCutoverLaneBlockingReason(&report.BlockingReasons, "selector", report.Selector, requiredConsecutive)
		appendSkillCutoverLaneBlockingReason(&report.BlockingReasons, "execution", report.Execution, requiredConsecutive)
		appendSkillCutoverLaneBlockingReason(&report.BlockingReasons, "budget", report.Budget, requiredConsecutive)
	}

	report.EvaluatedGatesReady = candidateID != "" &&
		report.Selector.Error == "" &&
		report.Execution.Error == "" &&
		report.Budget.Error == "" &&
		report.Selector.Ready &&
		report.Execution.Ready &&
		report.Budget.Ready
	report.Ready = report.EvaluatedGatesReady &&
		len(report.BlockingReasons) == 0 &&
		len(report.UnverifiedRequirements) == 0
	c.emitOptimizationTrigger(ctx, OptimizationTrigger{
		Reason:      cutoverReadinessReason(report.Ready),
		CandidateID: report.CandidateID,
		Metadata: map[string]interface{}{
			"blocking_reasons": append([]string(nil), report.BlockingReasons...),
		},
	})
	return report, nil
}

func cutoverReadinessReason(ready bool) OptimizationReason {
	if ready {
		return OptimizationReasonBudgetGatePassed
	}
	return OptimizationReasonCutoverBlocking
}

func (c *Controller) resolveSkillCutoverEvalSpecIDs(ctx context.Context, ownerUserID string, req SkillCutoverReadinessRequest) (string, string, error) {
	selectorEvalSpecID := strings.TrimSpace(req.SelectorEvalSpecID)
	executionEvalSpecID := strings.TrimSpace(req.ExecutionEvalSpecID)

	if selectorEvalSpecID == "" {
		assets, err := c.EnsureSelectorCuratedAssets(ctx, ownerUserID)
		if err != nil {
			return "", "", err
		}
		if assets == nil || assets.EvalSpec == nil || strings.TrimSpace(assets.EvalSpec.ID) == "" {
			return "", "", fmt.Errorf("selector curated eval spec is not available")
		}
		selectorEvalSpecID = strings.TrimSpace(assets.EvalSpec.ID)
	}
	if executionEvalSpecID == "" {
		assets, err := c.EnsureBatch1ExecutionAssets(ctx, ownerUserID)
		if err != nil {
			return "", "", err
		}
		if assets == nil || assets.EvalSpec == nil || strings.TrimSpace(assets.EvalSpec.ID) == "" {
			return "", "", fmt.Errorf("batch-1 execution eval spec is not available")
		}
		executionEvalSpecID = strings.TrimSpace(assets.EvalSpec.ID)
	}

	return selectorEvalSpecID, executionEvalSpecID, nil
}

func (c *Controller) evaluateSkillCutoverSelectorLane(ctx context.Context, evalSpecID string, candidateID string, evalRuns []EvalRun, req SelectorGateRequest, requiredConsecutive int, maxAssessments int) SkillCutoverLaneReadiness {
	lane := SkillCutoverLaneReadiness{
		EvalSpecID:              strings.TrimSpace(evalSpecID),
		CandidateID:             strings.TrimSpace(candidateID),
		RequiredConsecutiveRuns: requiredConsecutive,
		BaseEvalRunID:           strings.TrimSpace(req.BaseEvalRunID),
		BaselineID:              strings.TrimSpace(req.BaselineID),
	}
	if lane.CandidateID == "" {
		return lane
	}

	streakActive := true
	for _, evalRun := range evalRuns {
		if evalRunCandidateID(&evalRun) != lane.CandidateID {
			continue
		}
		lane.CandidateRunCount++
		report, err := c.EvaluateSelectorGate(ctx, evalRun.ID, req)
		if err != nil {
			lane.Error = err.Error()
			lane.Ready = false
			return lane
		}
		assessment := SkillCutoverLaneAssessment{
			EvalRunID:          evalRun.ID,
			CandidateID:        lane.CandidateID,
			Title:              evalRun.Title,
			CreatedAt:          evalRun.CreatedAt,
			Passed:             report.Passed,
			BaseEvalRunID:      report.BaseEvalRunID,
			BaselineID:         report.BaselineID,
			ComparisonReportID: report.ComparisonReportID,
			FailedChecks:       failedSkillCutoverSelectorChecks(report.Checks),
		}
		if lane.BaseEvalRunID == "" {
			lane.BaseEvalRunID = assessment.BaseEvalRunID
		}
		if lane.BaselineID == "" {
			lane.BaselineID = assessment.BaselineID
		}
		if len(lane.Assessments) < maxAssessments {
			lane.Assessments = append(lane.Assessments, assessment)
		}
		if streakActive && report.Passed {
			lane.ConsecutivePassCount++
		} else {
			streakActive = false
		}
	}

	lane.Ready = lane.CandidateRunCount > 0 && lane.ConsecutivePassCount >= lane.RequiredConsecutiveRuns
	return lane
}

func (c *Controller) evaluateSkillCutoverExecutionLane(ctx context.Context, evalSpecID string, candidateID string, evalRuns []EvalRun, req ExecutionEquivalenceRequest, requiredConsecutive int, maxAssessments int) SkillCutoverLaneReadiness {
	lane := SkillCutoverLaneReadiness{
		EvalSpecID:              strings.TrimSpace(evalSpecID),
		CandidateID:             strings.TrimSpace(candidateID),
		RequiredConsecutiveRuns: requiredConsecutive,
		BaseEvalRunID:           strings.TrimSpace(req.BaseEvalRunID),
		BaselineID:              strings.TrimSpace(req.BaselineID),
	}
	if lane.CandidateID == "" {
		return lane
	}

	streakActive := true
	for _, evalRun := range evalRuns {
		if evalRunCandidateID(&evalRun) != lane.CandidateID {
			continue
		}
		lane.CandidateRunCount++
		report, err := c.EvaluateExecutionEquivalence(ctx, evalRun.ID, req)
		if err != nil {
			lane.Error = err.Error()
			lane.Ready = false
			return lane
		}
		assessment := SkillCutoverLaneAssessment{
			EvalRunID:          evalRun.ID,
			CandidateID:        lane.CandidateID,
			Title:              evalRun.Title,
			CreatedAt:          evalRun.CreatedAt,
			Passed:             report.Passed,
			BaseEvalRunID:      report.BaseEvalRunID,
			BaselineID:         report.BaselineID,
			ComparisonReportID: report.ComparisonReportID,
			FailedChecks:       failedSkillCutoverExecutionChecks(report.Checks),
		}
		if lane.BaseEvalRunID == "" {
			lane.BaseEvalRunID = assessment.BaseEvalRunID
		}
		if lane.BaselineID == "" {
			lane.BaselineID = assessment.BaselineID
		}
		if len(lane.Assessments) < maxAssessments {
			lane.Assessments = append(lane.Assessments, assessment)
		}
		if streakActive && report.Passed {
			lane.ConsecutivePassCount++
		} else {
			streakActive = false
		}
	}

	lane.Ready = lane.CandidateRunCount > 0 && lane.ConsecutivePassCount >= lane.RequiredConsecutiveRuns
	return lane
}

func (c *Controller) evaluateSkillCutoverBudgetLane(ctx context.Context, evalSpecID string, candidateID string, evalRuns []EvalRun, req SkillCutoverBudgetRequest, requiredConsecutive int, maxAssessments int) SkillCutoverLaneReadiness {
	lane := SkillCutoverLaneReadiness{
		EvalSpecID:              strings.TrimSpace(evalSpecID),
		CandidateID:             strings.TrimSpace(candidateID),
		RequiredConsecutiveRuns: requiredConsecutive,
		BaseEvalRunID:           strings.TrimSpace(req.BaseEvalRunID),
		BaselineID:              strings.TrimSpace(req.BaselineID),
	}
	if lane.CandidateID == "" {
		return lane
	}

	streakActive := true
	for _, evalRun := range evalRuns {
		if evalRunCandidateID(&evalRun) != lane.CandidateID {
			continue
		}
		lane.CandidateRunCount++
		report, err := c.EvaluateSkillCutoverBudgetGate(ctx, evalRun.ID, req)
		if err != nil {
			lane.Error = err.Error()
			lane.Ready = false
			return lane
		}
		assessment := SkillCutoverLaneAssessment{
			EvalRunID:     evalRun.ID,
			CandidateID:   lane.CandidateID,
			Title:         evalRun.Title,
			CreatedAt:     evalRun.CreatedAt,
			Passed:        report.Passed,
			BaseEvalRunID: report.BaseEvalRunID,
			BaselineID:    report.BaselineID,
			FailedChecks:  failedSkillCutoverBudgetChecks(report.Checks),
		}
		if lane.BaseEvalRunID == "" {
			lane.BaseEvalRunID = assessment.BaseEvalRunID
		}
		if lane.BaselineID == "" {
			lane.BaselineID = assessment.BaselineID
		}
		if len(lane.Assessments) < maxAssessments {
			lane.Assessments = append(lane.Assessments, assessment)
		}
		if streakActive && report.Passed {
			lane.ConsecutivePassCount++
		} else {
			streakActive = false
		}
	}

	lane.Ready = lane.CandidateRunCount > 0 && lane.ConsecutivePassCount >= lane.RequiredConsecutiveRuns
	return lane
}

func resolveSkillCutoverCandidateID(selectorRuns []EvalRun, executionRuns []EvalRun) string {
	type candidatePresence struct {
		hasSelector  bool
		hasExecution bool
		latestAt     time.Time
	}

	candidates := make(map[string]candidatePresence)
	record := func(run EvalRun, isSelector bool) {
		candidateID := evalRunCandidateID(&run)
		if candidateID == "" {
			return
		}
		presence := candidates[candidateID]
		if isSelector {
			presence.hasSelector = true
		} else {
			presence.hasExecution = true
		}
		runAt := skillCutoverEvalRunCandidateTime(&run)
		if runAt.After(presence.latestAt) {
			presence.latestAt = runAt
		}
		candidates[candidateID] = presence
	}
	for _, run := range selectorRuns {
		record(run, true)
	}
	for _, run := range executionRuns {
		record(run, false)
	}

	bestSharedID := ""
	bestSharedAt := time.Time{}
	bestAnyID := ""
	bestAnyAt := time.Time{}
	for candidateID, presence := range candidates {
		if presence.latestAt.After(bestAnyAt) {
			bestAnyID = candidateID
			bestAnyAt = presence.latestAt
		}
		if presence.hasSelector && presence.hasExecution && presence.latestAt.After(bestSharedAt) {
			bestSharedID = candidateID
			bestSharedAt = presence.latestAt
		}
	}
	if bestSharedID != "" {
		return bestSharedID
	}
	return bestAnyID
}

func sortSkillCutoverEvalRuns(evalRuns []EvalRun) []EvalRun {
	if len(evalRuns) <= 1 {
		return evalRuns
	}
	out := append([]EvalRun(nil), evalRuns...)
	sort.SliceStable(out, func(i, j int) bool {
		left := skillCutoverEvalRunCandidateTime(&out[i])
		right := skillCutoverEvalRunCandidateTime(&out[j])
		if left.Equal(right) {
			if !out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
				return out[i].UpdatedAt.After(out[j].UpdatedAt)
			}
			// Preserve the existing store order for truly identical timestamps so
			// repeated candidate assessments stay stable across runs.
			return false
		}
		return left.After(right)
	})
	return out
}

func skillCutoverEvalRunCandidateTime(evalRun *EvalRun) time.Time {
	if evalRun == nil {
		return time.Time{}
	}
	if !evalRun.CreatedAt.IsZero() {
		return evalRun.CreatedAt
	}
	if evalRun.FinishedAt != nil && !evalRun.FinishedAt.IsZero() {
		return *evalRun.FinishedAt
	}
	if !evalRun.UpdatedAt.IsZero() {
		return evalRun.UpdatedAt
	}
	if evalRun.StartedAt != nil && !evalRun.StartedAt.IsZero() {
		return *evalRun.StartedAt
	}
	if !evalRun.CreatedAt.IsZero() {
		return evalRun.CreatedAt
	}
	return time.Time{}
}

func skillCutoverTerminalStatuses() []RunGroupStatus {
	return []RunGroupStatus{
		RunGroupStatusCompleted,
		RunGroupStatusPartial,
		RunGroupStatusFailed,
		RunGroupStatusCancelled,
	}
}

func evalRunCandidateID(evalRun *EvalRun) string {
	if evalRun == nil {
		return ""
	}
	return strings.TrimSpace(metadataString(evalRun.Metadata, skillCutoverCandidateMetadataKey))
}

func appendSkillCutoverLaneBlockingReason(reasons *[]string, laneName string, lane SkillCutoverLaneReadiness, requiredConsecutive int) {
	if reasons == nil {
		return
	}
	laneName = strings.TrimSpace(laneName)
	if lane.Error != "" {
		*reasons = append(*reasons, fmt.Sprintf("%s gate evaluation is unavailable: %s", laneName, lane.Error))
		return
	}
	if lane.CandidateRunCount == 0 {
		*reasons = append(*reasons, fmt.Sprintf("no %s eval runs found for candidate %q", laneName, lane.CandidateID))
		return
	}
	if lane.ConsecutivePassCount < requiredConsecutive {
		*reasons = append(*reasons, fmt.Sprintf("%s has %d consecutive green runs for candidate %q; need %d", laneName, lane.ConsecutivePassCount, lane.CandidateID, requiredConsecutive))
	}
}

func failedSkillCutoverSelectorChecks(checks []SelectorGateCheck) []string {
	out := make([]string, 0, len(checks))
	for _, check := range checks {
		if check.Passed {
			continue
		}
		name := strings.TrimSpace(check.Name)
		if name == "" {
			name = "unnamed_check"
		}
		out = append(out, name)
	}
	return out
}

func failedSkillCutoverExecutionChecks(checks []ExecutionEquivalenceCheck) []string {
	out := make([]string, 0, len(checks))
	for _, check := range checks {
		if check.Passed {
			continue
		}
		name := strings.TrimSpace(check.Name)
		if name == "" {
			name = "unnamed_check"
		}
		out = append(out, name)
	}
	return out
}

func failedSkillCutoverBudgetChecks(checks []SkillCutoverBudgetCheck) []string {
	out := make([]string, 0, len(checks))
	for _, check := range checks {
		if check.Passed {
			continue
		}
		name := strings.TrimSpace(check.Name)
		if name == "" {
			name = "unnamed_check"
		}
		out = append(out, name)
	}
	return out
}

func resolveSkillCutoverBudgetRequest(selector SelectorGateRequest, budget SkillCutoverBudgetRequest) SkillCutoverBudgetRequest {
	if strings.TrimSpace(budget.BaseEvalRunID) == "" {
		budget.BaseEvalRunID = strings.TrimSpace(selector.BaseEvalRunID)
	}
	if strings.TrimSpace(budget.BaselineID) == "" {
		budget.BaselineID = strings.TrimSpace(selector.BaselineID)
	}
	return budget
}
