package harness

import (
	"context"
	"database/sql"
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
		if _, err := c.refreshGroupSummary(ctx, group.ID); err != nil {
			return nil, err
		}
	}
	return c.store.GetGroup(ctx, group.ID)
}

func (c *Controller) GetGroup(ctx context.Context, id string) (*RunGroup, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	if _, err := c.refreshGroupSummary(ctx, id); err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	return c.store.GetGroup(ctx, id)
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
		refreshed, refreshErr := c.refreshGroupSummary(ctx, groups[i].ID)
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
	items, err := c.store.ListGroupItems(ctx, group.ID)
	if err != nil {
		return nil, err
	}
	scorecards, err := c.store.ListScorecards(ctx, group.ID)
	if err != nil {
		return nil, err
	}
	latestCards := latestScorecardsByItem(scorecards)

	linkedRuns, err := c.store.ListRuns(ctx, RunFilter{GroupID: group.ID, Limit: 1000})
	if err != nil {
		return nil, err
	}
	runByID := make(map[string]*Run, len(linkedRuns))
	for i := range linkedRuns {
		run := linkedRuns[i]
		runByID[run.ID] = &run
	}

	artifacts := make([]ArtifactRef, 0)
	seenArtifacts := make(map[string]struct{})
	for i := range linkedRuns {
		runArtifacts, artErr := c.store.ListArtifacts(ctx, linkedRuns[i].ID)
		if artErr != nil {
			continue
		}
		for _, artifact := range runArtifacts {
			if _, ok := seenArtifacts[artifact.ID]; ok {
				continue
			}
			seenArtifacts[artifact.ID] = struct{}{}
			artifacts = append(artifacts, artifact)
		}
	}

	report := &RunGroupReport{
		Group:         group,
		Items:         items,
		VerdictCounts: make(map[string]int),
		LinkedRuns:    linkedRuns,
		Artifacts:     artifacts,
		Scorecards:    scorecards,
		Breakdown:     cloneMetadataMap(group.Summary),
	}

	var totalScore float64
	var totalRated int
	var passed int
	for _, item := range items {
		card, ok := latestCards[item.ID]
		if !ok {
			continue
		}
		report.VerdictCounts[string(card.Verdict)]++
		totalScore += card.Score
		totalRated++
		if card.Verdict == ScoreVerdictPass {
			passed++
			continue
		}
		entry := map[string]interface{}{
			"item":      item,
			"scorecard": card,
		}
		if run, ok := runByID[item.LatestRunID]; ok {
			entry["run"] = run
		}
		report.FailedItems = append(report.FailedItems, entry)
	}
	if totalRated > 0 {
		report.OverallScore = totalScore / float64(totalRated)
		report.PassRate = float64(passed) / float64(len(items))
	}
	return report, nil
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
	return c.store.UpdateGroup(ctx, group)
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
		item.LeaseExpiresAt = &now
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
	}
	return retried, nil
}

func (c *Controller) refreshGroupSummary(ctx context.Context, groupID string) (*RunGroup, error) {
	group, err := c.store.GetGroup(ctx, groupID)
	if err != nil {
		return nil, err
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
	summary := map[string]interface{}{
		"item_count": len(items),
	}
	counts := make(map[string]int)
	var (
		activeQueued  int
		activeRunning int
		activeScoring int
		totalAttempts int
		totalScore    float64
		ratedCount    int
		passedCount   int
	)
	for _, item := range items {
		counts[string(item.Status)]++
		totalAttempts += item.AttemptCount
		switch item.Status {
		case RunGroupItemStatusQueued, RunGroupItemStatusPending:
			activeQueued++
		case RunGroupItemStatusRunning:
			activeRunning++
		case RunGroupItemStatusScoring:
			activeScoring++
		}
		if card, ok := latestCards[item.ID]; ok {
			totalScore += card.Score
			ratedCount++
			if card.Verdict == ScoreVerdictPass {
				passedCount++
			}
			counts["verdict:"+string(card.Verdict)]++
		}
	}
	summary["counts"] = counts
	summary["total_attempts"] = totalAttempts
	if ratedCount > 0 {
		summary["overall_score"] = totalScore / float64(ratedCount)
		summary["pass_rate"] = float64(passedCount) / float64(len(items))
	}
	group.Summary = summary
	if group.StartedAt == nil && (activeRunning > 0 || activeScoring > 0 || totalAttempts > 0) {
		now := timeutil.NowTime()
		group.StartedAt = &now
	}
	group.Status = deriveGroupStatus(group.Status, counts)
	if isTerminalGroupStatus(group.Status) {
		now := timeutil.NowTime()
		group.FinishedAt = &now
	}
	if err := c.store.UpdateGroup(ctx, group); err != nil {
		return nil, err
	}
	return group, nil
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
		case RunKindAgentTask, RunKindResearch, RunKindSubagent:
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
