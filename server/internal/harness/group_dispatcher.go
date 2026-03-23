package harness

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

const (
	defaultGroupDispatchPollInterval = 1 * time.Second
	defaultGroupRunPollInterval      = 300 * time.Millisecond
)

type GroupDispatcher struct {
	manager         *Controller
	workerID        string
	pollInterval    time.Duration
	runPollInterval time.Duration

	mu sync.Mutex
}

func NewGroupDispatcher(manager *Controller) *GroupDispatcher {
	return &GroupDispatcher{
		manager:         manager,
		workerID:        "local-dispatcher:" + uuid.NewString(),
		pollInterval:    defaultGroupDispatchPollInterval,
		runPollInterval: defaultGroupRunPollInterval,
	}
}

func (d *GroupDispatcher) SetPollInterval(interval time.Duration) {
	if d == nil || interval <= 0 {
		return
	}
	d.pollInterval = interval
}

func (d *GroupDispatcher) SetRunPollInterval(interval time.Duration) {
	if d == nil || interval <= 0 {
		return
	}
	d.runPollInterval = interval
}

func (d *GroupDispatcher) Start(ctx context.Context) {
	if d == nil || d.manager == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	_ = d.DispatchOnce(ctx)
	ticker := time.NewTicker(d.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = d.DispatchOnce(ctx)
		}
	}
}

func (d *GroupDispatcher) DispatchOnce(ctx context.Context) error {
	if d == nil || d.manager == nil || d.manager.store == nil {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	groups, err := d.manager.store.ListGroups(ctx, RunGroupFilter{
		Statuses: []RunGroupStatus{
			RunGroupStatusPending,
			RunGroupStatusQueued,
			RunGroupStatusRunning,
			RunGroupStatusScoring,
		},
		Limit: 200,
	})
	if err != nil {
		return err
	}

	var firstErr error
	for i := range groups {
		if err := d.dispatchGroup(ctx, &groups[i]); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (d *GroupDispatcher) dispatchGroup(ctx context.Context, group *RunGroup) error {
	if group == nil {
		return nil
	}
	if _, err := d.recoverExpiredItems(ctx, group.ID); err != nil {
		return err
	}
	refreshed, err := d.manager.refreshGroupSummary(ctx, group.ID)
	if err != nil {
		return err
	}
	group = refreshed
	if group == nil || isTerminalGroupStatus(group.Status) {
		return nil
	}

	active, err := d.manager.store.CountGroupItemsByStatuses(ctx, group.ID, []RunGroupItemStatus{
		RunGroupItemStatusRunning,
		RunGroupItemStatusScoring,
	})
	if err != nil {
		return err
	}
	maxConcurrency := group.SchedulerConfig.MaxConcurrency
	if maxConcurrency <= 0 {
		maxConcurrency = defaultGroupMaxConcurrency
	}

	for active < maxConcurrency {
		item, claimErr := d.manager.store.ClaimNextGroupItem(ctx, group.ID, d.workerID, groupLeaseTTL(group), timeutil.NowTime())
		if claimErr != nil {
			if claimErr == sql.ErrNoRows {
				break
			}
			return claimErr
		}
		active++
		go d.processClaimedItem(item.ID, group.ID)
	}
	return nil
}

func (d *GroupDispatcher) processClaimedItem(itemID string, groupID string) {
	ctx := context.Background()
	group, err := d.manager.store.GetGroup(ctx, groupID)
	if err != nil {
		return
	}
	item, err := d.manager.store.GetGroupItem(ctx, itemID)
	if err != nil {
		return
	}

	runSpec, err := d.buildGroupItemRunSpec(ctx, group, item)
	if err != nil {
		d.failAttempt(ctx, group, item, nil, err, true)
		return
	}

	run, err := d.manager.Submit(ctx, runSpec)
	if err != nil {
		d.failAttempt(ctx, group, item, nil, err, true)
		return
	}

	item, err = d.manager.store.GetGroupItem(ctx, item.ID)
	if err != nil {
		return
	}
	item.AttemptCount++
	item.LatestRunID = run.ID
	item.Status = RunGroupItemStatusRunning
	item.LeaseOwner = d.workerID
	leaseUntil := timeutil.NowTime().Add(groupLeaseTTL(group))
	item.LeaseExpiresAt = &leaseUntil
	if err := d.manager.store.UpdateGroupItem(ctx, item); err != nil {
		return
	}
	if _, err := d.manager.refreshGroupSummary(ctx, group.ID); err != nil {
		return
	}

	terminalRun, waitErr := d.waitForRun(ctx, group, item.ID, run.ID)
	if waitErr != nil {
		d.failAttempt(ctx, group, item, run, waitErr, false)
		return
	}

	item, err = d.manager.store.GetGroupItem(ctx, item.ID)
	if err != nil {
		return
	}
	if item.Status == RunGroupItemStatusCancelled {
		_, _ = d.manager.refreshGroupSummary(ctx, group.ID)
		return
	}
	item.Status = RunGroupItemStatusScoring
	item.LeaseOwner = d.workerID
	nextLease := timeutil.NowTime().Add(groupLeaseTTL(group))
	item.LeaseExpiresAt = &nextLease
	if err := d.manager.store.UpdateGroupItem(ctx, item); err != nil {
		return
	}

	verification := d.manager.verifyGroupRun(ctx, group, item, terminalRun)
	scorecard := d.manager.scoreGroupRun(ctx, group, item, terminalRun, &verification)
	scorecard = d.manager.annotateResearchProposalSummary(ctx, group, terminalRun, scorecard)
	if err := d.manager.store.AttachScorecard(ctx, scorecard); err != nil {
		d.failAttempt(ctx, group, item, terminalRun, err, false)
		return
	}
	d.finalizeAttempt(ctx, group, item, terminalRun, &scorecard)
}

func (d *GroupDispatcher) waitForRun(ctx context.Context, group *RunGroup, itemID string, runID string) (*Run, error) {
	ticker := time.NewTicker(d.runPollInterval)
	defer ticker.Stop()

	leaseRenewAfter := timeutil.NowTime().Add(groupLeaseTTL(group) / 2)
	for {
		run, err := d.manager.Get(ctx, runID)
		if err == nil && run != nil && isTerminalRunStatus(run.Status) {
			return run, nil
		}

		item, itemErr := d.manager.store.GetGroupItem(ctx, itemID)
		if itemErr == nil {
			if item.Status == RunGroupItemStatusCancelled {
				return run, nil
			}
			if item.LeaseOwner != d.workerID {
				return run, fmt.Errorf("group item lease moved to another worker")
			}
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			if timeutil.NowTime().After(leaseRenewAfter) {
				if renewErr := d.renewItemLease(ctx, itemID, groupLeaseTTL(group)); renewErr == nil {
					leaseRenewAfter = timeutil.NowTime().Add(groupLeaseTTL(group) / 2)
				}
			}
		}
	}
}

func (d *GroupDispatcher) renewItemLease(ctx context.Context, itemID string, leaseTTL time.Duration) error {
	if leaseTTL <= 0 {
		return nil
	}
	item, err := d.manager.store.GetGroupItem(ctx, itemID)
	if err != nil {
		return err
	}
	if item.LeaseOwner != d.workerID {
		return fmt.Errorf("lease owner mismatch")
	}
	leaseUntil := timeutil.NowTime().Add(leaseTTL)
	item.LeaseExpiresAt = &leaseUntil
	return d.manager.store.UpdateGroupItem(ctx, item)
}

func (d *GroupDispatcher) recoverExpiredItems(ctx context.Context, groupID string) (int, error) {
	items, err := d.manager.store.ListGroupItems(ctx, groupID)
	if err != nil {
		return 0, err
	}
	now := timeutil.NowTime()
	recovered := 0
	for i := range items {
		item := items[i]
		if item.LeaseExpiresAt == nil || item.LeaseExpiresAt.After(now) {
			continue
		}
		switch item.Status {
		case RunGroupItemStatusRunning, RunGroupItemStatusScoring:
		default:
			continue
		}
		item.Status = RunGroupItemStatusQueued
		item.LeaseOwner = ""
		item.LeaseExpiresAt = nil
		if err := d.manager.store.UpdateGroupItem(ctx, &item); err != nil {
			return recovered, err
		}
		recovered++
	}
	return recovered, nil
}

func (d *GroupDispatcher) failAttempt(ctx context.Context, group *RunGroup, item *RunGroupItem, run *Run, cause error, submissionFailed bool) {
	if item == nil || group == nil {
		return
	}
	current, err := d.manager.store.GetGroupItem(ctx, item.ID)
	if err == nil {
		item = current
	}
	if submissionFailed {
		item.AttemptCount++
	}
	scorecard := makeScorecard(group, item, run, ScoringModeRule, ScoreVerdictError, 0, map[string]interface{}{
		"scorer": "dispatcher",
		"reason": strings.TrimSpace(cause.Error()),
	}, scoreEvidence(item, run), map[string]interface{}{
		"judge":  "dispatcher",
		"reason": strings.TrimSpace(cause.Error()),
	})
	_ = d.manager.store.AttachScorecard(ctx, scorecard)

	if shouldRetryAttempt(item) {
		item.Status = RunGroupItemStatusQueued
		backoffUntil := timeutil.NowTime().Add(groupRetryBackoff(group))
		item.LeaseExpiresAt = &backoffUntil
	} else {
		item.Status = RunGroupItemStatusError
		item.LeaseExpiresAt = nil
	}
	item.LeaseOwner = ""
	_ = d.manager.store.UpdateGroupItem(ctx, item)
	_, _ = d.manager.refreshGroupSummary(ctx, group.ID)
}

func (d *GroupDispatcher) finalizeAttempt(ctx context.Context, group *RunGroup, item *RunGroupItem, run *Run, scorecard *Scorecard) {
	if group == nil || item == nil {
		return
	}
	current, err := d.manager.store.GetGroupItem(ctx, item.ID)
	if err == nil {
		item = current
	}
	item.LeaseOwner = ""
	item.LeaseExpiresAt = nil

	if run != nil && run.Status == RunStatusCancelled {
		item.Status = RunGroupItemStatusCancelled
		_ = d.manager.store.UpdateGroupItem(ctx, item)
		_, _ = d.manager.refreshGroupSummary(ctx, group.ID)
		return
	}

	item.Status = verdictToItemStatus(scorecard)
	if (item.Status == RunGroupItemStatusFailed || item.Status == RunGroupItemStatusError) && shouldRetryScoredAttempt(item, scorecard) {
		item.Status = RunGroupItemStatusQueued
		backoffUntil := timeutil.NowTime().Add(groupRetryBackoff(group))
		item.LeaseExpiresAt = &backoffUntil
	}
	_ = d.manager.store.UpdateGroupItem(ctx, item)
	_, _ = d.manager.refreshGroupSummary(ctx, group.ID)
}

func (d *GroupDispatcher) buildGroupItemRunSpec(ctx context.Context, group *RunGroup, item *RunGroupItem) (RunSpec, error) {
	spec, err := buildGroupItemRunSpec(group, item)
	if err != nil {
		return RunSpec{}, err
	}
	retryContext, retryFeedback := d.retryFeedbackForItem(ctx, item)
	if retryContext == "" && len(retryFeedback) == 0 {
		return spec, nil
	}
	if spec.Metadata == nil {
		spec.Metadata = map[string]interface{}{}
	}
	if retryContext != "" {
		spec.Metadata["retry_context"] = retryContext
	}
	if len(retryFeedback) > 0 {
		spec.Metadata["retry_feedback"] = retryFeedback
	}
	return spec, nil
}

func (d *GroupDispatcher) retryFeedbackForItem(ctx context.Context, item *RunGroupItem) (string, map[string]interface{}) {
	if d == nil || d.manager == nil || d.manager.store == nil || item == nil || item.AttemptCount <= 0 {
		return "", nil
	}
	card, err := d.manager.store.LatestScorecardForItem(ctx, item.ID)
	if err != nil || card == nil {
		return "", nil
	}
	previousRunID := strings.TrimSpace(card.RunID)
	if previousRunID == "" {
		previousRunID = strings.TrimSpace(item.LatestRunID)
	}
	var previousRun *Run
	if previousRunID != "" {
		previousRun, _ = d.manager.store.GetRun(ctx, previousRunID)
	}
	retryContext, retryFeedback := buildRetryFeedback(card, previousRun)
	if retryContext == "" && len(retryFeedback) == 0 {
		return "", nil
	}
	return retryContext, retryFeedback
}

func buildGroupItemRunSpec(group *RunGroup, item *RunGroupItem) (RunSpec, error) {
	if group == nil || item == nil {
		return RunSpec{}, fmt.Errorf("group and item are required")
	}
	metadata := mergeMetadataMaps(group.Metadata, item.Metadata)
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	metadata["group_profile"] = strings.TrimSpace(item.Profile)
	metadata["group_input"] = cloneMetadataMap(item.Input)
	metadata["group_expected"] = cloneMetadataMap(item.Expected)

	goal := firstMapString(item.Input, "goal", "query", "prompt")
	if goal == "" {
		goal = firstMapString(metadata, "goal", "query", "prompt")
	}
	if goal == "" {
		goal = strings.TrimSpace(group.Subject)
	}
	if goal == "" {
		goal = strings.TrimSpace(group.Title)
	}
	if goal == "" {
		return RunSpec{}, fmt.Errorf("group item %s does not define a goal/query/prompt", item.ID)
	}

	if v := firstMapString(item.Input, "context"); v != "" && metadataString(metadata, "context") == "" {
		metadata["context"] = v
	}
	for _, key := range []string{"mode", "route_mode", "lang", "report_style"} {
		if v := firstMapString(item.Input, key); v != "" && metadataString(metadata, key) == "" {
			metadata[key] = v
		}
	}
	if _, ok := metadata["time_windows"]; !ok {
		if raw, ok := item.Input["time_windows"]; ok {
			metadata["time_windows"] = raw
		}
	}

	spec := RunSpec{
		Kind:   item.RunKind,
		Goal:   goal,
		UserID: strings.TrimSpace(group.OwnerUserID),
		ConversationID: firstNonEmpty(
			firstMapString(item.Input, "conversation_id", "conversationId"),
			firstMapString(metadata, "conversation_id", "conversationId"),
		),
		SessionID: firstNonEmpty(
			firstMapString(item.Input, "session_id", "sessionId"),
			firstMapString(metadata, "session_id", "sessionId"),
		),
		AgentID: firstNonEmpty(
			firstMapString(item.Input, "agent_id", "agentId"),
			firstMapString(metadata, "agent_id", "agentId"),
		),
		Model: firstNonEmpty(
			firstMapString(item.Input, "model"),
			firstMapString(metadata, "model"),
		),
		WorkspaceRoot: firstNonEmpty(
			firstMapString(item.Input, "workspace_root", "workspaceRoot"),
			firstMapString(metadata, "workspace_root", "workspaceRoot"),
		),
		SandboxMode: firstNonEmpty(
			firstMapString(item.Input, "sandbox_mode", "sandboxMode"),
			firstMapString(metadata, "sandbox_mode", "sandboxMode"),
		),
		GroupID:       group.ID,
		GroupItemID:   item.ID,
		AttemptIndex:  item.AttemptCount + 1,
		Metadata:      metadata,
		MaxDuration:   firstPositiveDuration(item.Input, metadata),
		MaxSteps:      firstPositiveInt(item.Input, metadata, "max_steps", "maxSteps"),
		MaxToolRounds: firstPositiveInt(item.Input, metadata, "max_tool_rounds", "maxToolRounds"),
		MaxSubagents:  firstPositiveInt(item.Input, metadata, "max_subagents", "maxSubagents"),
		MaxDepth:      firstPositiveInt(item.Input, metadata, "max_depth", "maxDepth"),
	}
	if approval := firstNonEmpty(
		firstMapString(item.Input, "approval_mode", "approvalMode"),
		firstMapString(metadata, "approval_mode", "approvalMode"),
	); approval != "" {
		spec.ApprovalMode = ApprovalMode(strings.TrimSpace(approval))
	}
	return spec, nil
}

func mergeMetadataMaps(base map[string]interface{}, override map[string]interface{}) map[string]interface{} {
	if len(base) == 0 && len(override) == 0 {
		return nil
	}
	out := cloneMetadataMap(base)
	if out == nil {
		out = make(map[string]interface{}, len(override))
	}
	for key, value := range override {
		out[key] = value
	}
	return out
}

func buildRetryFeedback(card *Scorecard, previousRun *Run) (string, map[string]interface{}) {
	if card == nil {
		return "", nil
	}
	breakdown := decodeJSONMap(card.BreakdownJSON)
	evidence := decodeJSONMap(card.EvidenceJSON)
	trace := decodeJSONMap(card.JudgeTraceJSON)
	verification := nestedMetadataMap(evidence, "verification")
	if verification == nil {
		verification = nestedMetadataMap(trace, "verification")
	}

	failureLabel := firstNonEmpty(metadataString(verification, "failure_label"), metadataString(breakdown, "failure_label"))
	summary := firstNonEmpty(metadataString(verification, "summary"), metadataString(breakdown, "reason"), metadataString(trace, "reason"))
	failedChecks := failedVerificationChecks(verification)
	failedArtifacts := failedVerificationArtifacts(verification)
	retryContext := renderRetryContext(previousRun, card, failureLabel, summary, failedChecks, failedArtifacts)
	if retryContext == "" {
		return "", nil
	}

	feedback := map[string]interface{}{
		"previous_run_id":  strings.TrimSpace(card.RunID),
		"previous_verdict": string(card.Verdict),
		"previous_score":   card.Score,
	}
	if previousRun != nil {
		feedback["previous_attempt_index"] = previousRun.AttemptIndex
		if result := truncateRetryText(previousRun.Result, 240); result != "" {
			feedback["previous_result"] = result
		}
		if errText := truncateRetryText(previousRun.Error, 240); errText != "" {
			feedback["previous_error"] = errText
		}
	}
	if failureLabel != "" {
		feedback["failure_label"] = failureLabel
	}
	if summary != "" {
		feedback["summary"] = summary
	}
	if retryable, ok := mapBool(breakdown, "retryable"); ok {
		feedback["retryable"] = retryable
	}
	if len(failedChecks) > 0 {
		feedback["failed_checks"] = append([]string(nil), failedChecks...)
	}
	if len(failedArtifacts) > 0 {
		feedback["failed_artifacts"] = append([]string(nil), failedArtifacts...)
	}
	return retryContext, feedback
}

func nestedMetadataMap(meta map[string]interface{}, key string) map[string]interface{} {
	if len(meta) == 0 {
		return nil
	}
	raw, ok := meta[key]
	if !ok {
		return nil
	}
	nested, _ := raw.(map[string]interface{})
	return nested
}

func failedVerificationChecks(verification map[string]interface{}) []string {
	if len(verification) == 0 {
		return nil
	}
	raw, ok := verification["checks"].([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		record, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if passed, ok := record["passed"].(bool); ok && passed {
			continue
		}
		name := firstNonEmpty(retryText(record["name"]), "verification check")
		expected := retryText(record["expected"])
		actual := retryText(record["actual"])
		switch {
		case expected != "" && actual != "":
			out = append(out, fmt.Sprintf("%s (expected: %s; actual: %s)", name, expected, actual))
		case expected != "":
			out = append(out, fmt.Sprintf("%s (expected: %s)", name, expected))
		case actual != "":
			out = append(out, fmt.Sprintf("%s (actual: %s)", name, actual))
		default:
			out = append(out, name)
		}
	}
	return out
}

func failedVerificationArtifacts(verification map[string]interface{}) []string {
	if len(verification) == 0 {
		return nil
	}
	raw, ok := verification["artifacts"].([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		record, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if passed, ok := record["passed"].(bool); ok && passed {
			continue
		}
		target := firstNonEmpty(retryText(record["path"]), retryText(record["label"]), retryText(record["target"]), "artifact")
		actual := retryText(record["actual"])
		if actual != "" {
			out = append(out, fmt.Sprintf("%s (actual: %s)", target, actual))
			continue
		}
		out = append(out, target)
	}
	return out
}

func renderRetryContext(previousRun *Run, card *Scorecard, failureLabel string, summary string, failedChecks []string, failedArtifacts []string) string {
	lines := []string{"Harness retry guidance from the previous attempt:"}
	if previousRun != nil && previousRun.AttemptIndex > 0 {
		lines = append(lines, fmt.Sprintf("- Previous attempt: %d", previousRun.AttemptIndex))
	}
	if card != nil {
		lines = append(lines, fmt.Sprintf("- Previous verdict: %s (score %.2f)", card.Verdict, card.Score))
	}
	if failureLabel != "" {
		lines = append(lines, fmt.Sprintf("- Failure label: %s", failureLabel))
	}
	if summary != "" {
		lines = append(lines, fmt.Sprintf("- Failure summary: %s", summary))
	}
	if correction := retryCorrectionHint(failureLabel, failedChecks, failedArtifacts); correction != "" {
		lines = append(lines, fmt.Sprintf("- Correction target: %s", correction))
	}
	for _, detail := range limitRetryItems(failedArtifacts, 2) {
		lines = append(lines, fmt.Sprintf("- Artifact issue: %s", detail))
	}
	for _, detail := range limitRetryItems(failedChecks, 3) {
		lines = append(lines, fmt.Sprintf("- Failed check: %s", detail))
	}
	if previousRun != nil {
		if result := truncateRetryText(previousRun.Result, 220); result != "" {
			lines = append(lines, fmt.Sprintf("- Previous result excerpt: %s", result))
		}
		if errText := truncateRetryText(previousRun.Error, 220); errText != "" {
			lines = append(lines, fmt.Sprintf("- Previous error: %s", errText))
		}
	}
	lines = append(lines, "- Please correct the issue above before declaring this retry complete.")
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func retryCorrectionHint(failureLabel string, failedChecks []string, failedArtifacts []string) string {
	switch strings.TrimSpace(failureLabel) {
	case "missing_artifact":
		if len(failedArtifacts) > 0 {
			return "produce the missing artifact before finishing"
		}
		return "emit the expected artifact before finishing"
	case "required_check_missing":
		if len(failedChecks) > 0 {
			return "make the missing verification evidence visible in the result or emitted events"
		}
		return "satisfy the required verification check before finishing"
	case "tool_selection_error":
		return "use the required tool call before finishing"
	case "forbidden_tool_used":
		return "avoid the forbidden tool on this retry and choose a safer alternative"
	case "run_failed", "run_not_completed", "timeout", "verification_failed":
		return "keep this retry smaller and more deterministic so it completes successfully"
	default:
		if len(failedArtifacts) > 0 {
			return "fix the missing artifact output before finishing"
		}
		if len(failedChecks) > 0 {
			return "fix the failing verification checks before finishing"
		}
		return ""
	}
}

func limitRetryItems(items []string, maxItems int) []string {
	if len(items) == 0 || maxItems <= 0 {
		return nil
	}
	if len(items) <= maxItems {
		return append([]string(nil), items...)
	}
	return append([]string(nil), items[:maxItems]...)
}

func truncateRetryText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.Join(strings.Fields(value), " ")
	if limit <= 0 || len(value) <= limit {
		return value
	}
	if limit <= 3 {
		return value[:limit]
	}
	return value[:limit-3] + "..."
}

func retryText(raw interface{}) string {
	if raw == nil {
		return ""
	}
	text := strings.TrimSpace(fmt.Sprint(raw))
	if text == "" || text == "<nil>" {
		return ""
	}
	return text
}

func firstPositiveInt(primary map[string]interface{}, secondary map[string]interface{}, keys ...string) int {
	for _, source := range []map[string]interface{}{primary, secondary} {
		for _, key := range keys {
			raw, ok := source[key]
			if !ok {
				continue
			}
			switch value := raw.(type) {
			case int:
				if value > 0 {
					return value
				}
			case int32:
				if value > 0 {
					return int(value)
				}
			case int64:
				if value > 0 {
					return int(value)
				}
			case float64:
				if value > 0 {
					return int(value)
				}
			}
		}
	}
	return 0
}

func firstPositiveDuration(primary map[string]interface{}, secondary map[string]interface{}) time.Duration {
	if seconds := firstPositiveInt(primary, secondary, "max_duration_seconds", "maxDurationSeconds", "duration_seconds", "durationSeconds"); seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	for _, source := range []map[string]interface{}{primary, secondary} {
		for _, key := range []string{"max_duration_ns", "maxDurationNs"} {
			raw, ok := source[key]
			if !ok {
				continue
			}
			switch value := raw.(type) {
			case int:
				if value > 0 {
					return time.Duration(value)
				}
			case int64:
				if value > 0 {
					return time.Duration(value)
				}
			case float64:
				if value > 0 {
					return time.Duration(value)
				}
			}
		}
	}
	return 0
}

func verdictToItemStatus(scorecard *Scorecard) RunGroupItemStatus {
	if scorecard == nil {
		return RunGroupItemStatusError
	}
	switch scorecard.Verdict {
	case ScoreVerdictPass:
		return RunGroupItemStatusPassed
	case ScoreVerdictFail, ScoreVerdictPartial:
		return RunGroupItemStatusFailed
	default:
		return RunGroupItemStatusError
	}
}

func shouldRetryAttempt(item *RunGroupItem) bool {
	if item == nil {
		return false
	}
	return item.MaxAttempts <= 0 || item.AttemptCount < item.MaxAttempts
}

func shouldRetryScoredAttempt(item *RunGroupItem, scorecard *Scorecard) bool {
	if !shouldRetryAttempt(item) {
		return false
	}
	if scorecard == nil {
		return true
	}
	breakdown := decodeJSONMap(scorecard.BreakdownJSON)
	if len(breakdown) == 0 {
		return true
	}
	if retryable, ok := mapBool(breakdown, "retryable"); ok {
		return retryable
	}
	return true
}

func groupLeaseTTL(group *RunGroup) time.Duration {
	if group != nil && group.SchedulerConfig.LeaseTTL > 0 {
		return group.SchedulerConfig.LeaseTTL
	}
	return defaultGroupLeaseTTL
}

func groupRetryBackoff(group *RunGroup) time.Duration {
	if group != nil && group.SchedulerConfig.RetryBackoff > 0 {
		return group.SchedulerConfig.RetryBackoff
	}
	return defaultGroupRetryBackoff
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
