package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

const (
	defaultGroupMaxConcurrency = 1
	defaultGroupMaxAttempts    = 1
	defaultGroupLeaseTTL       = 2 * time.Minute
	defaultGroupRetryBackoff   = 3 * time.Second
	defaultGroupPassThreshold  = 0.5
)

type groupReportContext struct {
	group           *RunGroup
	items           []RunGroupItem
	scorecards      []Scorecard
	latestCards     map[string]Scorecard
	linkedRuns      []Run
	runByID         map[string]*Run
	runtimeEvidence map[string][]RuntimeEvidenceEntry
	runtimeTraces   map[string]RunTrace
	artifacts       []ArtifactRef
	checkpoints     []CheckpointArtifact
	itemContracts   map[string]HarnessContract
	metrics         groupReportMetrics
}

type groupReportRuntimeBundle struct {
	linkedRuns      []Run
	runByID         map[string]*Run
	runtimeEvidence map[string][]RuntimeEvidenceEntry
	runtimeTraces   map[string]RunTrace
	artifacts       []ArtifactRef
	checkpoints     []CheckpointArtifact
}

type groupReportMetrics struct {
	statusCounts         map[string]int
	verdictCounts        map[string]int
	failureLabelCounts   map[string]int
	failedItems          []map[string]interface{}
	totalAttempts        int
	totalScore           float64
	ratedCount           int
	passedCount          int
	verificationPassed   int
	evidenceBackedPasses int
	retryRecoveredCount  int
	activeQueued         int
	activeRunning        int
	activeScoring        int
}

func (c *Controller) SubmitGroup(ctx context.Context, spec RunGroupSpec) (*RunGroup, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	spec = normalizeGroupSpec(spec)
	if err := validateGroupSpec(spec); err != nil {
		return nil, err
	}

	now := timeutil.NowTime()
	group := &RunGroup{
		ID:              uuid.NewString(),
		Kind:            spec.Kind,
		Title:           strings.TrimSpace(spec.Title),
		Status:          RunGroupStatusQueued,
		OwnerUserID:     strings.TrimSpace(spec.OwnerUserID),
		Subject:         strings.TrimSpace(spec.Subject),
		SchedulerConfig: spec.SchedulerConfig,
		ScoringConfig:   spec.ScoringConfig,
		Metadata:        cloneMetadataMap(spec.Metadata),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if len(spec.Items) == 0 {
		group.Status = RunGroupStatusCompleted
		group.StartedAt = &now
		group.FinishedAt = &now
		group.Summary = map[string]interface{}{
			"item_count": 0,
		}
	}
	if err := c.store.CreateGroup(ctx, group); err != nil {
		return nil, err
	}

	items := make([]RunGroupItem, 0, len(spec.Items))
	for i, itemSpec := range spec.Items {
		items = append(items, RunGroupItem{
			ID:          uuid.NewString(),
			GroupID:     group.ID,
			Index:       i,
			RunKind:     itemSpec.RunKind,
			Profile:     strings.TrimSpace(itemSpec.Profile),
			Input:       cloneMetadataMap(itemSpec.Input),
			Expected:    cloneMetadataMap(itemSpec.Expected),
			Metadata:    cloneMetadataMap(itemSpec.Metadata),
			Status:      RunGroupItemStatusQueued,
			MaxAttempts: group.SchedulerConfig.MaxAttempts,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
	}
	if err := c.store.CreateGroupItems(ctx, items); err != nil {
		return nil, err
	}
	if len(items) > 0 {
		refreshed, err := c.refreshGroupSummary(ctx, group.ID)
		if err != nil {
			return nil, err
		}
		return refreshed, nil
	}
	return group, nil
}

func (c *Controller) GetGroup(ctx context.Context, id string) (*RunGroup, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	group, err := c.store.GetGroup(ctx, id)
	if err != nil {
		return nil, err
	}
	return c.projectLoadedGroupSummary(ctx, group)
}

func (c *Controller) ListGroups(ctx context.Context, filter RunGroupFilter) ([]RunGroup, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	groups, err := c.store.ListGroups(ctx, filter)
	if err != nil {
		return nil, err
	}
	for i := range groups {
		refreshed, refreshErr := c.projectLoadedGroupSummary(ctx, &groups[i])
		if refreshErr != nil || refreshed == nil {
			continue
		}
		groups[i] = *refreshed
	}
	return groups, nil
}

func (c *Controller) ListGroupItems(ctx context.Context, groupID string) ([]RunGroupItem, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	return c.store.ListGroupItems(ctx, strings.TrimSpace(groupID))
}

func (c *Controller) GetGroupReport(ctx context.Context, id string) (*RunGroupReport, error) {
	group, err := c.GetGroup(ctx, id)
	if err != nil {
		return nil, err
	}
	return c.buildGroupReport(ctx, group)
}

func (c *Controller) buildGroupReport(ctx context.Context, group *RunGroup) (*RunGroupReport, error) {
	reportCtx, err := c.loadGroupReportContext(ctx, group)
	if err != nil {
		return nil, err
	}
	return buildGroupReportFromContext(reportCtx)
}

func (c *Controller) loadGroupReportContext(ctx context.Context, group *RunGroup) (*groupReportContext, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	if group == nil {
		return nil, fmt.Errorf("group is required")
	}
	items, err := c.store.ListGroupItems(ctx, group.ID)
	if err != nil {
		return nil, err
	}
	scorecards, err := c.store.ListScorecards(ctx, group.ID)
	if err != nil {
		return nil, err
	}
	latestCards := latestScorecardsByItem(scorecards)
	runtimeBundle, err := c.loadGroupReportRuntimeBundle(ctx, group.ID)
	if err != nil {
		return nil, err
	}
	reconciled, err := c.reconcileGroupReportState(ctx, group, items, latestCards, runtimeBundle)
	if err != nil {
		return nil, err
	}
	if reconciled {
		items, err = c.store.ListGroupItems(ctx, group.ID)
		if err != nil {
			return nil, err
		}
		scorecards, err = c.store.ListScorecards(ctx, group.ID)
		if err != nil {
			return nil, err
		}
		latestCards = latestScorecardsByItem(scorecards)
		if refreshed, refreshErr := c.refreshGroupSummary(ctx, group.ID); refreshErr == nil && refreshed != nil {
			group = refreshed
		}
	}
	metrics := buildGroupReportMetrics(items, latestCards, runtimeBundle.runByID)
	return &groupReportContext{
		group:           group,
		items:           items,
		scorecards:      scorecards,
		latestCards:     latestCards,
		linkedRuns:      runtimeBundle.linkedRuns,
		runByID:         runtimeBundle.runByID,
		runtimeEvidence: runtimeBundle.runtimeEvidence,
		runtimeTraces:   runtimeBundle.runtimeTraces,
		artifacts:       runtimeBundle.artifacts,
		checkpoints:     runtimeBundle.checkpoints,
		itemContracts:   buildGroupItemContracts(group, items),
		metrics:         metrics,
	}, nil
}

func (c *Controller) reconcileGroupReportState(ctx context.Context, group *RunGroup, items []RunGroupItem, latestCards map[string]Scorecard, runtimeBundle *groupReportRuntimeBundle) (bool, error) {
	if c == nil || c.store == nil || group == nil || runtimeBundle == nil {
		return false, nil
	}

	now := timeutil.NowTime()
	reconciled := false
	for i := range items {
		item := &items[i]
		// Avoid double-scoring while the dispatcher is actively running/scoring
		// this item under a valid lease. Reconciliation is meant to repair stale
		// state, not race the live dispatcher.
		if (item.Status == RunGroupItemStatusRunning || item.Status == RunGroupItemStatusScoring) &&
			strings.TrimSpace(item.LeaseOwner) != "" &&
			item.LeaseExpiresAt != nil &&
			item.LeaseExpiresAt.After(now) {
			continue
		}

		runID := strings.TrimSpace(item.LatestRunID)
		if runID == "" {
			continue
		}
		run := runtimeBundle.runByID[runID]
		if run == nil || run.Status != RunStatusCompleted {
			continue
		}

		card, hasCard := latestCards[item.ID]
		needsRescore := !hasCard
		if hasCard {
			if strings.TrimSpace(card.RunID) != run.ID {
				needsRescore = true
			}
			if item.Status != verdictToItemStatus(&card) {
				needsRescore = true
			}
			if run.UpdatedAt.After(card.CreatedAt) && card.Verdict != ScoreVerdictPass {
				needsRescore = true
			}
			if runLooksVerifiedSuccessful(run) && card.Verdict != ScoreVerdictPass {
				needsRescore = true
			}
		}
		if !needsRescore {
			continue
		}

		verification := c.verifyGroupRun(ctx, group, item, run)
		scorecard := c.scoreGroupRun(ctx, group, item, run, &verification)
		scorecard = c.annotateResearchProposalSummary(ctx, group, run, scorecard)
		if err := c.store.AttachScorecard(ctx, scorecard); err != nil {
			return reconciled, err
		}

		nextStatus := verdictToItemStatus(&scorecard)
		if item.Status != nextStatus || item.LeaseOwner != "" || item.LeaseExpiresAt != nil {
			item.Status = nextStatus
			item.LeaseOwner = ""
			item.LeaseExpiresAt = nil
			if err := c.store.UpdateGroupItem(ctx, item); err != nil {
				return reconciled, err
			}
		}
		latestCards[item.ID] = scorecard
		reconciled = true
	}
	return reconciled, nil
}

func runLooksVerifiedSuccessful(run *Run) bool {
	if run == nil {
		return false
	}
	corpus := strings.ToLower(strings.TrimSpace(strings.Join([]string{run.Result, run.Error}, "\n")))
	if corpus == "" {
		return false
	}
	return strings.Contains(corpus, "verification: pass") ||
		strings.Contains(corpus, "verification passed") ||
		strings.Contains(corpus, "grounded runtime checks passed") ||
		strings.Contains(corpus, "task completed with grounded")
}

func buildGroupReportFromContext(reportCtx *groupReportContext) (*RunGroupReport, error) {
	if reportCtx == nil {
		return nil, fmt.Errorf("group report context is required")
	}
	if reportCtx.group == nil {
		return nil, fmt.Errorf("group is required")
	}
	artifacts := make([]ArtifactRef, 0)
	checkpoints := make([]CheckpointArtifact, 0)
	artifacts = append(artifacts, reportCtx.artifacts...)
	checkpoints = append(checkpoints, reportCtx.checkpoints...)

	report := &RunGroupReport{
		Group:           reportCtx.group,
		Items:           reportCtx.items,
		VerdictCounts:   cloneIntMap(reportCtx.metrics.verdictCounts),
		LinkedRuns:      reportCtx.linkedRuns,
		Artifacts:       artifacts,
		Scorecards:      reportCtx.scorecards,
		Breakdown:       cloneMetadataMap(reportCtx.group.Summary),
		RuntimeEvidence: reportCtx.runtimeEvidence,
		RuntimeTraces:   cloneRunTraceMap(reportCtx.runtimeTraces),
		ItemContracts:   reportCtx.itemContracts,
		Checkpoints:     checkpoints,
	}

	report.FailedItems = cloneFailedItemEntries(reportCtx.metrics.failedItems)
	if reportCtx.metrics.ratedCount > 0 {
		report.OverallScore = reportCtx.metrics.totalScore / float64(reportCtx.metrics.ratedCount)
		report.PassRate = float64(reportCtx.metrics.passedCount) / float64(len(reportCtx.items))
	}
	return report, nil
}

func buildGroupReportMetrics(items []RunGroupItem, latestCards map[string]Scorecard, runByID map[string]*Run) groupReportMetrics {
	metrics := groupReportMetrics{
		statusCounts:       make(map[string]int),
		verdictCounts:      make(map[string]int),
		failureLabelCounts: make(map[string]int),
		failedItems:        make([]map[string]interface{}, 0),
	}
	for _, item := range items {
		metrics.statusCounts[string(item.Status)]++
		metrics.totalAttempts += item.AttemptCount
		switch item.Status {
		case RunGroupItemStatusQueued, RunGroupItemStatusPending:
			metrics.activeQueued++
		case RunGroupItemStatusRunning:
			metrics.activeRunning++
		case RunGroupItemStatusScoring:
			metrics.activeScoring++
		}

		card, ok := latestCards[item.ID]
		if !ok {
			continue
		}
		metrics.totalScore += card.Score
		metrics.ratedCount++
		metrics.verdictCounts[string(card.Verdict)]++
		metrics.statusCounts["verdict:"+string(card.Verdict)]++

		breakdown := decodeJSONMap(card.BreakdownJSON)
		if passed, ok := mapBool(breakdown, "verification_passed"); ok && passed {
			metrics.verificationPassed++
		}
		if label := metadataString(breakdown, "failure_label"); label != "" {
			metrics.failureLabelCounts[label]++
		}
		if card.Verdict == ScoreVerdictPass {
			metrics.passedCount++
			if evidenceScore, ok := breakdown["evidence_score"].(float64); ok && evidenceScore >= 0.8 {
				metrics.evidenceBackedPasses++
			} else if passed, ok := mapBool(breakdown, "verification_passed"); ok && passed {
				metrics.evidenceBackedPasses++
			}
			if item.AttemptCount > 1 {
				metrics.retryRecoveredCount++
			}
			continue
		}

		entry := map[string]interface{}{
			"item":      item,
			"scorecard": card,
		}
		if run, ok := runByID[item.LatestRunID]; ok && run != nil {
			entry["run"] = run
		}
		metrics.failedItems = append(metrics.failedItems, entry)
	}
	return metrics
}

func buildGroupItemContracts(group *RunGroup, items []RunGroupItem) map[string]HarnessContract {
	itemContracts := make(map[string]HarnessContract, len(items))
	if group == nil {
		return itemContracts
	}
	for _, item := range items {
		contract := DecodeHarnessContract(group.Metadata, item.Metadata, item.Expected, item.Input)
		if contractMeta := HarnessContractMetadata(contract); len(contractMeta) > 0 {
			itemContracts[item.ID] = contract
		}
	}
	return itemContracts
}

func (c *Controller) loadGroupReportRuntimeBundle(ctx context.Context, groupID string) (*groupReportRuntimeBundle, error) {
	linkedRuns, err := c.store.ListRuns(ctx, RunFilter{GroupID: groupID, Limit: 1000})
	if err != nil {
		return nil, err
	}
	bundle := &groupReportRuntimeBundle{
		linkedRuns:      linkedRuns,
		runByID:         make(map[string]*Run, len(linkedRuns)),
		runtimeEvidence: make(map[string][]RuntimeEvidenceEntry, len(linkedRuns)),
		runtimeTraces:   make(map[string]RunTrace, len(linkedRuns)),
		artifacts:       make([]ArtifactRef, 0),
		checkpoints:     make([]CheckpointArtifact, 0),
	}
	seenArtifacts := make(map[string]struct{})
	for i := range linkedRuns {
		if synced, syncErr := c.syncRun(ctx, &bundle.linkedRuns[i]); syncErr == nil && synced != nil {
			bundle.linkedRuns[i] = *synced
		}
		run := &bundle.linkedRuns[i]
		bundle.runByID[run.ID] = run
		if driver, driverErr := c.driverFor(run.Kind); driverErr == nil {
			if provider, ok := driver.(RuntimeEvidenceProvider); ok {
				if evidence, evidenceErr := provider.ListRuntimeEvidence(ctx, run); evidenceErr == nil && len(evidence) > 0 {
					bundle.runtimeEvidence[run.ID] = evidence
				}
			}
		}
		if trace, traceErr := c.RunTraceSnapshot(ctx, run.ID); traceErr == nil && trace != nil {
			bundle.runtimeTraces[run.ID] = *trace
		}
		runArtifacts, checkpoints := c.loadGroupReportRunArtifacts(ctx, run, seenArtifacts)
		bundle.artifacts = append(bundle.artifacts, runArtifacts...)
		bundle.checkpoints = append(bundle.checkpoints, checkpoints...)
	}
	return bundle, nil
}

func (c *Controller) loadGroupReportRunArtifacts(ctx context.Context, run *Run, seenArtifacts map[string]struct{}) ([]ArtifactRef, []CheckpointArtifact) {
	if c == nil || c.store == nil || run == nil {
		return nil, nil
	}
	runArtifacts, err := c.store.ListArtifacts(ctx, run.ID)
	if err != nil {
		return nil, nil
	}
	artifacts := make([]ArtifactRef, 0, len(runArtifacts))
	checkpoints := make([]CheckpointArtifact, 0)
	for _, artifact := range runArtifacts {
		if _, ok := seenArtifacts[artifact.ID]; ok {
			continue
		}
		seenArtifacts[artifact.ID] = struct{}{}
		artifacts = append(artifacts, artifact)
		if strings.TrimSpace(artifact.Kind) == "checkpoint" {
			checkpoints = append(checkpoints, CheckpointArtifact{
				RunID:       run.ID,
				GroupItemID: run.GroupItemID,
				Artifact:    artifact,
				Payload:     unmarshalMetadata(artifact.MetadataJSON),
			})
		}
	}
	return artifacts, checkpoints
}

func (c *Controller) CancelGroup(ctx context.Context, id string, reason string) error {
	group, err := c.store.GetGroup(ctx, id)
	if err != nil {
		return err
	}
	items, err := c.store.ListGroupItems(ctx, group.ID)
	if err != nil {
		return err
	}
	now := timeutil.NowTime()
	for i := range items {
		item := items[i]
		switch item.Status {
		case RunGroupItemStatusQueued, RunGroupItemStatusPending, RunGroupItemStatusRunning, RunGroupItemStatusScoring:
			if strings.TrimSpace(item.LatestRunID) != "" {
				_ = c.Cancel(ctx, item.LatestRunID, strings.TrimSpace(reason))
			}
			item.Status = RunGroupItemStatusCancelled
			item.LeaseOwner = ""
			item.LeaseExpiresAt = nil
			_ = c.store.UpdateGroupItem(ctx, &item)
		}
	}
	group.Status = RunGroupStatusCancelled
	group.FinishedAt = &now
	if group.StartedAt == nil {
		group.StartedAt = &now
	}
	if group.Summary == nil {
		group.Summary = map[string]interface{}{}
	}
	group.Summary["cancel_reason"] = strings.TrimSpace(reason)
	if err := c.store.UpdateGroup(ctx, group); err != nil {
		return err
	}
	_, err = c.refreshGroupSummary(ctx, group.ID)
	return err
}

func (c *Controller) PerformGroupAction(ctx context.Context, id string, action string, input map[string]interface{}) (*RunGroup, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	group, err := c.store.GetGroup(ctx, id)
	if err != nil {
		return nil, err
	}
	action = strings.ToLower(strings.TrimSpace(action))
	if action == "" {
		return nil, fmt.Errorf("action is required")
	}
	switch action {
	case "cancel":
		reason := strings.TrimSpace(runActionMetadataString(input, "reason"))
		if reason == "" {
			reason = "cancelled by user"
		}
		if err := c.CancelGroup(ctx, group.ID, reason); err != nil {
			return nil, err
		}
		return c.GetGroup(ctx, group.ID)
	default:
		return nil, fmt.Errorf("unsupported group action %q", action)
	}
}

func (c *Controller) RetryFailedGroup(ctx context.Context, id string) (int, error) {
	group, err := c.store.GetGroup(ctx, id)
	if err != nil {
		return 0, err
	}
	items, err := c.store.ListGroupItems(ctx, group.ID)
	if err != nil {
		return 0, err
	}
	now := timeutil.NowTime()
	retried := 0
	for i := range items {
		item := items[i]
		if item.Status != RunGroupItemStatusFailed && item.Status != RunGroupItemStatusError {
			continue
		}
		if item.MaxAttempts > 0 && item.AttemptCount >= item.MaxAttempts {
			continue
		}
		item.Status = RunGroupItemStatusQueued
		item.LeaseOwner = ""
		backoffUntil := now.Add(groupRetryBackoff(group))
		item.LeaseExpiresAt = &backoffUntil
		if err := c.store.UpdateGroupItem(ctx, &item); err != nil {
			return retried, err
		}
		retried++
	}
	if retried > 0 {
		group.Status = RunGroupStatusQueued
		group.FinishedAt = nil
		if err := c.store.UpdateGroup(ctx, group); err != nil {
			return retried, err
		}
		if _, err := c.refreshGroupSummary(ctx, group.ID); err != nil {
			return retried, err
		}
	}
	return retried, nil
}

func (c *Controller) refreshLoadedGroupSummary(ctx context.Context, group *RunGroup) (*RunGroup, error) {
	refreshed, _, err := c.projectGroupSummary(ctx, group)
	return refreshed, err
}

func (c *Controller) projectLoadedGroupSummary(ctx context.Context, group *RunGroup) (*RunGroup, error) {
	if !shouldRefreshStoredGroupSummary(group) {
		return group, nil
	}
	refreshed, _, err := c.projectGroupSummary(ctx, group)
	return refreshed, err
}

func (c *Controller) refreshGroupSummary(ctx context.Context, groupID string) (*RunGroup, error) {
	group, err := c.store.GetGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	refreshed, changed, err := c.projectGroupSummary(ctx, group)
	if err != nil {
		return nil, err
	}
	if !changed {
		return refreshed, nil
	}
	if err := c.store.UpdateGroup(ctx, refreshed); err != nil {
		return nil, err
	}
	return refreshed, nil
}

func (c *Controller) projectGroupSummary(ctx context.Context, group *RunGroup) (*RunGroup, bool, error) {
	if group == nil {
		return nil, false, nil
	}
	items, err := c.store.ListGroupItems(ctx, group.ID)
	if err != nil {
		return nil, false, err
	}
	scorecards, err := c.store.ListScorecards(ctx, group.ID)
	if err != nil {
		return nil, false, err
	}
	latestCards := latestScorecardsByItem(scorecards)
	metrics := buildGroupReportMetrics(items, latestCards, nil)
	summary := map[string]interface{}{
		"item_count": len(items),
	}
	summary["counts"] = intMapToMetadataMap(metrics.statusCounts)
	summary["total_attempts"] = metrics.totalAttempts
	if metrics.ratedCount > 0 {
		summary["overall_score"] = metrics.totalScore / float64(metrics.ratedCount)
		summary["pass_rate"] = float64(metrics.passedCount) / float64(len(items))
		summary["verification_pass_rate"] = float64(metrics.verificationPassed) / float64(metrics.ratedCount)
	}
	if metrics.passedCount > 0 {
		summary["evidence_backed_pass_rate"] = float64(metrics.evidenceBackedPasses) / float64(metrics.passedCount)
	}
	if metrics.retryRecoveredCount > 0 {
		summary["retry_recovered_count"] = metrics.retryRecoveredCount
	}
	if len(metrics.failureLabelCounts) > 0 {
		summary["failure_label_counts"] = intMapToMetadataMap(metrics.failureLabelCounts)
	}

	nextStatus := deriveGroupStatus(group.Status, metrics.statusCounts)
	summary = preserveGroupSummaryAnnotations(nextStatus, group.Summary, summary)
	nextStartedAt := group.StartedAt
	if nextStartedAt == nil && (metrics.activeRunning > 0 || metrics.activeScoring > 0 || metrics.totalAttempts > 0 || (isTerminalGroupStatus(nextStatus) && len(items) > 0)) {
		now := timeutil.NowTime()
		nextStartedAt = &now
	}
	nextFinishedAt := group.FinishedAt
	if isTerminalGroupStatus(nextStatus) && nextFinishedAt == nil {
		now := timeutil.NowTime()
		nextFinishedAt = &now
	}
	if !isTerminalGroupStatus(nextStatus) {
		nextFinishedAt = nil
	}
	if group.Status == nextStatus &&
		metadataMapsEqual(group.Summary, summary) &&
		timePointersEqual(group.StartedAt, nextStartedAt) &&
		timePointersEqual(group.FinishedAt, nextFinishedAt) {
		projected := *group
		projected.Summary = summary
		return &projected, false, nil
	}
	projected := *group
	projected.Summary = summary
	projected.Status = nextStatus
	projected.StartedAt = nextStartedAt
	projected.FinishedAt = nextFinishedAt
	return &projected, true, nil
}

func cloneFailedItemEntries(entries []map[string]interface{}) []map[string]interface{} {
	if len(entries) == 0 {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(entries))
	for _, entry := range entries {
		cloned := make(map[string]interface{}, len(entry))
		for key, value := range entry {
			cloned[key] = value
		}
		out = append(out, cloned)
	}
	return out
}

func cloneIntMap(values map[string]int) map[string]int {
	if len(values) == 0 {
		return map[string]int{}
	}
	out := make(map[string]int, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func preserveGroupSummaryAnnotations(status RunGroupStatus, currentSummary map[string]interface{}, nextSummary map[string]interface{}) map[string]interface{} {
	if len(nextSummary) == 0 {
		nextSummary = map[string]interface{}{}
	}
	if isTerminalGroupStatus(status) {
		for key, value := range currentSummary {
			if _, exists := nextSummary[key]; exists {
				continue
			}
			if isDerivedGroupSummaryKey(key) {
				continue
			}
			nextSummary[key] = value
		}
	}
	if status == RunGroupStatusCancelled {
		if reason := strings.TrimSpace(metadataString(currentSummary, "cancel_reason")); reason != "" {
			nextSummary["cancel_reason"] = reason
		}
	}
	return nextSummary
}

func isDerivedGroupSummaryKey(key string) bool {
	switch strings.TrimSpace(key) {
	case "item_count", "counts", "total_attempts", "overall_score", "pass_rate", "verification_pass_rate", "evidence_backed_pass_rate", "retry_recovered_count", "failure_label_counts", "cancel_reason":
		return true
	default:
		return false
	}
}

func shouldRefreshStoredGroupSummary(group *RunGroup) bool {
	if group == nil {
		return false
	}
	if !isTerminalGroupStatus(group.Status) {
		return true
	}
	itemCount := intMetadata(group.Summary["item_count"])
	if group.FinishedAt == nil {
		return true
	}
	if group.StartedAt == nil && itemCount > 0 {
		return true
	}
	if itemCount == 0 {
		return false
	}
	return len(metadataMapValue(group.Summary["counts"])) == 0
}

func intMapToMetadataMap(values map[string]int) map[string]interface{} {
	if len(values) == 0 {
		return map[string]interface{}{}
	}
	out := make(map[string]interface{}, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func timePointersEqual(left, right *time.Time) bool {
	switch {
	case left == nil && right == nil:
		return true
	case left == nil || right == nil:
		return false
	default:
		return left.Equal(*right)
	}
}

func normalizeGroupSpec(spec RunGroupSpec) RunGroupSpec {
	spec.Kind = RunGroupKind(strings.TrimSpace(string(spec.Kind)))
	if spec.Kind == "" {
		spec.Kind = RunGroupKindEval
	}
	spec.Title = strings.TrimSpace(spec.Title)
	spec.Subject = strings.TrimSpace(spec.Subject)
	if spec.SchedulerConfig.MaxConcurrency <= 0 {
		spec.SchedulerConfig.MaxConcurrency = defaultGroupMaxConcurrency
	}
	if spec.SchedulerConfig.MaxAttempts <= 0 {
		spec.SchedulerConfig.MaxAttempts = defaultGroupMaxAttempts
	}
	if spec.SchedulerConfig.LeaseTTL <= 0 {
		spec.SchedulerConfig.LeaseTTL = defaultGroupLeaseTTL
	}
	if spec.SchedulerConfig.RetryBackoff <= 0 {
		spec.SchedulerConfig.RetryBackoff = defaultGroupRetryBackoff
	}
	spec.ScoringConfig.Mode = ScoringMode(strings.TrimSpace(string(spec.ScoringConfig.Mode)))
	if spec.ScoringConfig.Mode == "" {
		spec.ScoringConfig.Mode = ScoringModeHybrid
	}
	spec.ScoringConfig.RuleProfile = strings.TrimSpace(spec.ScoringConfig.RuleProfile)
	spec.ScoringConfig.JudgeModel = strings.TrimSpace(spec.ScoringConfig.JudgeModel)
	if spec.ScoringConfig.PassThreshold <= 0 {
		spec.ScoringConfig.PassThreshold = defaultGroupPassThreshold
	}
	return spec
}

func validateGroupSpec(spec RunGroupSpec) error {
	switch spec.Kind {
	case RunGroupKindEval, RunGroupKindExperiment, RunGroupKindBatch:
	default:
		return fmt.Errorf("invalid group kind %q", spec.Kind)
	}
	switch spec.ScoringConfig.Mode {
	case ScoringModeRule, ScoringModeJudge, ScoringModeHybrid:
	default:
		return fmt.Errorf("invalid scoring mode %q", spec.ScoringConfig.Mode)
	}
	for i, item := range spec.Items {
		switch item.RunKind {
		case RunKindAgentTask, RunKindResearch, RunKindSubagent, RunKindWorkflow:
		default:
			return fmt.Errorf("items[%d].run_kind is invalid", i)
		}
	}
	return nil
}

func deriveGroupStatus(current RunGroupStatus, counts map[string]int) RunGroupStatus {
	if current == RunGroupStatusCancelled {
		return current
	}
	if counts[string(RunGroupItemStatusRunning)] > 0 {
		return RunGroupStatusRunning
	}
	if counts[string(RunGroupItemStatusScoring)] > 0 {
		return RunGroupStatusScoring
	}
	if counts[string(RunGroupItemStatusQueued)]+counts[string(RunGroupItemStatusPending)] > 0 {
		return RunGroupStatusQueued
	}
	passed := counts[string(RunGroupItemStatusPassed)]
	failed := counts[string(RunGroupItemStatusFailed)]
	errored := counts[string(RunGroupItemStatusError)]
	cancelled := counts[string(RunGroupItemStatusCancelled)]
	total := passed + failed + errored + cancelled
	if total == 0 {
		if strings.TrimSpace(string(current)) != "" {
			return current
		}
		return RunGroupStatusPending
	}
	switch {
	case passed == total:
		return RunGroupStatusCompleted
	case cancelled == total:
		return RunGroupStatusCancelled
	case passed == 0 && (failed+errored) == total:
		return RunGroupStatusFailed
	default:
		return RunGroupStatusPartial
	}
}

func isTerminalGroupStatus(status RunGroupStatus) bool {
	switch status {
	case RunGroupStatusCompleted, RunGroupStatusPartial, RunGroupStatusFailed, RunGroupStatusCancelled:
		return true
	default:
		return false
	}
}

func latestScorecardsByItem(cards []Scorecard) map[string]Scorecard {
	out := make(map[string]Scorecard, len(cards))
	for _, card := range cards {
		if _, ok := out[card.GroupItemID]; ok {
			continue
		}
		out[card.GroupItemID] = card
	}
	return out
}

func cloneScorecard(card *Scorecard) *Scorecard {
	if card == nil {
		return nil
	}
	cp := *card
	return &cp
}

func decodeJSONMap(raw string) map[string]interface{} {
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &out); err != nil {
		return nil
	}
	return out
}
