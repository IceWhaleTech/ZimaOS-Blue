package drivers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type researchModeExecutor interface {
	Execute(ctx context.Context, args map[string]interface{}) (interface{}, error)
}

type localResearchRunState struct {
	cancel      context.CancelFunc
	steps       map[string]string
	completions int
}

type ResearchDriver struct {
	service  *deepresearch.Service
	manager  *harness.Controller
	analyze  researchModeExecutor
	uiReview researchModeExecutor
	next     deepresearch.EventPublisher
	mu       sync.Mutex
	local    map[string]*localResearchRunState
}

func NewResearchDriver(service *deepresearch.Service, manager *harness.Controller) *ResearchDriver {
	return &ResearchDriver{service: service, manager: manager}
}

func (d *ResearchDriver) Kind() harness.RunKind { return harness.RunKindResearch }

func (d *ResearchDriver) Validate(spec harness.RunSpec) error {
	if d == nil || d.manager == nil {
		return fmt.Errorf("research runtime is not available")
	}
	if strings.TrimSpace(spec.Goal) == "" {
		return fmt.Errorf("goal is required")
	}
	switch resolveResearchFamilyMode(spec.Goal, spec.Metadata) {
	case "analyze":
		if isNilResearchModeExecutor(d.analyze) {
			return researchModeUnavailableError("analyze")
		}
	case "ui_review":
		if isNilResearchModeExecutor(d.uiReview) {
			return researchModeUnavailableError("ui_review")
		}
	default:
		if d.service == nil {
			return fmt.Errorf("research runtime is not available")
		}
	}
	return nil
}

func (d *ResearchDriver) Start(ctx context.Context, run *harness.Run, env harness.RunEnv) error {
	switch resolveResearchFamilyMode(run.Goal, run.Metadata) {
	case "analyze":
		return d.startLocalMode(ctx, run, env, "analyze", d.analyze)
	case "ui_review":
		return d.startLocalMode(ctx, run, env, "ui_review", d.uiReview)
	default:
		return d.startDeepResearch(ctx, run)
	}
}

func (d *ResearchDriver) startDeepResearch(ctx context.Context, run *harness.Run) error {
	if d == nil || d.service == nil {
		return fmt.Errorf("research runtime is not available")
	}
	req := deepresearch.CreateJobRequest{
		RequestedID:    run.ID,
		UserID:         run.UserID,
		ConversationID: run.ConversationID,
		ProviderID:     run.ProviderID,
		Query:          run.Goal,
		RetryContext:   metadataString(run.Metadata, "retry_context"),
		RetryFeedback:  metadataMap(run.Metadata["retry_feedback"]),
		Mode:           deepresearch.Mode(resolveResearchDepth(run.Metadata)),
		RouteMode:      deepresearch.RouteMode(metadataString(run.Metadata, "route_mode")),
		Lang:           metadataString(run.Metadata, "lang"),
		ReportStyle:    metadataString(run.Metadata, "report_style"),
		TimeWindows:    metadataStringSlice(run.Metadata["time_windows"]),
	}
	if maxSources := metadataInt(run.Metadata["max_sources"]); maxSources > 0 || run.MaxSteps > 0 || run.MaxDuration > 0 {
		req.Budget = &deepresearch.Budget{
			MaxSteps:   run.MaxSteps,
			MaxSources: maxSources,
			MaxSeconds: maxDurationSeconds(run.MaxDuration, run.Metadata["max_seconds"]),
		}
	}
	if strict, ok := metadataBool(run.Metadata["strict_entity"]); ok {
		req.StrictEntity = &strict
	}
	_, err := d.service.CreateJob(ctx, req)
	return err
}

func (d *ResearchDriver) Cancel(_ context.Context, run *harness.Run) error {
	if d == nil || run == nil {
		return fmt.Errorf("research runtime is not available")
	}
	switch resolveResearchFamilyMode(run.Goal, run.Metadata) {
	case "analyze", "ui_review":
		d.cancelLocalRun(run.ID)
		cancelled := cloneRunSnapshot(run)
		cancelled.Status = harness.RunStatusCancelled
		cancelled.Progress = 100
		cancelled.UpdatedAt = timeutil.NowTime()
		finished := cancelled.UpdatedAt
		cancelled.FinishedAt = &finished
		d.publishResearchJobEvent(cancelled, "deep_research.job_cancelled")
		return nil
	}
	if d.service == nil {
		return fmt.Errorf("research runtime is not available")
	}
	if strings.TrimSpace(run.UserID) != "" {
		return d.service.CancelJobForUser(run.ID, run.UserID, "")
	}
	return d.service.CancelJob(run.ID)
}

func (d *ResearchDriver) Sync(ctx context.Context, run *harness.Run) (*harness.Run, error) {
	if d == nil || run == nil {
		return run, nil
	}
	switch resolveResearchFamilyMode(run.Goal, run.Metadata) {
	case "analyze", "ui_review":
		controller := researchDriverManager(d.manager, nil)
		if controller == nil {
			return run, nil
		}
		stored, err := controller.GetStored(ctx, run.ID)
		if err != nil {
			return run, nil
		}
		return stored, nil
	}
	if d.service == nil {
		return run, nil
	}
	job, err := d.service.GetJob(run.ID)
	if err != nil {
		return run, nil
	}
	snapshot := jobToRun(run, job)
	if err := d.manager.SyncSnapshot(ctx, snapshot); err != nil {
		return nil, err
	}
	return d.manager.GetStored(ctx, run.ID)
}

func (d *ResearchDriver) SetNextPublisher(next deepresearch.EventPublisher) {
	d.next = next
}

func (d *ResearchDriver) SetAnalyzeExecutor(executor researchModeExecutor) {
	if d == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if isNilResearchModeExecutor(executor) {
		d.analyze = nil
		return
	}
	d.analyze = executor
}

func (d *ResearchDriver) SetUIReviewExecutor(executor researchModeExecutor) {
	if d == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if isNilResearchModeExecutor(executor) {
		d.uiReview = nil
		return
	}
	d.uiReview = executor
}

func (d *ResearchDriver) Publish(userID string, eventType string, data any) {
	if d.next != nil {
		d.next.Publish(userID, eventType, data)
	}
	if d == nil || d.manager == nil {
		return
	}
	jobID := extractResearchJobID(data)
	if jobID == "" {
		return
	}
	run, err := d.manager.GetStored(context.Background(), jobID)
	if err == nil {
		if _, syncErr := d.Sync(context.Background(), run); syncErr == nil {
			run, _ = d.manager.GetStored(context.Background(), jobID)
		}
	}
	payloadJSON := ""
	if raw, err := json.Marshal(data); err == nil {
		payloadJSON = string(raw)
	}
	_ = d.manager.AppendEvent(context.Background(), harness.RunEvent{
		RunID:       jobID,
		Type:        mapResearchEventType(eventType),
		Message:     strings.TrimSpace(eventType),
		PayloadJSON: payloadJSON,
		CreatedAt:   timeutil.NowTime(),
	})
}

func (d *ResearchDriver) startLocalMode(
	ctx context.Context,
	run *harness.Run,
	env harness.RunEnv,
	mode string,
	executor researchModeExecutor,
) error {
	if d == nil || run == nil {
		return fmt.Errorf("research runtime is not available")
	}
	if isNilResearchModeExecutor(executor) {
		return researchModeUnavailableError(mode)
	}
	controller := researchDriverManager(d.manager, env.Manager)
	if controller == nil {
		return fmt.Errorf("research runtime is not available")
	}

	execCtx, cancel := context.WithCancel(withLocalResearchToolContext(context.WithoutCancel(ctx), run))
	d.storeLocalRun(run.ID, cancel)

	starting := cloneRunSnapshot(run)
	if starting.Metadata == nil {
		starting.Metadata = map[string]interface{}{}
	}
	starting.Status = harness.RunStatusExecuting
	starting.Progress = max(starting.Progress, 1)
	starting.Metadata["mode"] = mode
	starting.UpdatedAt = timeutil.NowTime()
	if starting.StartedAt == nil {
		started := starting.UpdatedAt
		starting.StartedAt = &started
	}
	if err := controller.SyncSnapshot(ctx, starting); err != nil {
		d.clearLocalRun(run.ID)
		cancel()
		return err
	}
	d.publishResearchJobEvent(starting, "deep_research.job_created")

	execCtx = tools.WithCardEmitter(execCtx, func(card map[string]interface{}) {
		d.handleLocalProgress(controller, run.ID, mode, card)
	})
	go d.executeLocalMode(execCtx, controller, run, mode, executor)
	return nil
}

func (d *ResearchDriver) executeLocalMode(
	ctx context.Context,
	controller *harness.Controller,
	run *harness.Run,
	mode string,
	executor researchModeExecutor,
) {
	defer d.clearLocalRun(run.ID)

	result, err := executor.Execute(ctx, researchToolArgs(run, mode))
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		failed := d.loadLocalSnapshot(controller, run)
		if failed.Metadata == nil {
			failed.Metadata = map[string]interface{}{}
		}
		failed.Metadata["mode"] = mode
		failed.Status = harness.RunStatusFailed
		failed.Error = strings.TrimSpace(err.Error())
		failed.Progress = 100
		failed.UpdatedAt = timeutil.NowTime()
		if failed.StartedAt == nil {
			started := failed.UpdatedAt
			failed.StartedAt = &started
		}
		finished := failed.UpdatedAt
		failed.FinishedAt = &finished
		_ = controller.SyncSnapshot(context.Background(), failed)
		d.publishResearchJobEvent(failed, "deep_research.job_failed")
		return
	}

	completed := d.loadLocalSnapshot(controller, run)
	if completed.Metadata == nil {
		completed.Metadata = map[string]interface{}{}
	}
	completed.Metadata["mode"] = mode
	completed.Status = harness.RunStatusCompleted
	completed.Result = normalizeResearchExecutorResult(result)
	completed.Error = ""
	completed.Progress = 100
	completed.UpdatedAt = timeutil.NowTime()
	if completed.StartedAt == nil {
		started := completed.UpdatedAt
		completed.StartedAt = &started
	}
	finished := completed.UpdatedAt
	completed.FinishedAt = &finished
	if err := controller.SyncSnapshot(context.Background(), completed); err == nil {
		if stored, getErr := controller.GetStored(context.Background(), run.ID); getErr == nil && stored != nil {
			completed = stored
		}
	}
	d.publishResearchJobEvent(completed, "deep_research.job_completed")
}

func researchModeUnavailableError(mode string) error {
	switch strings.TrimSpace(mode) {
	case "analyze":
		return fmt.Errorf("analyze runtime is not available")
	case "ui_review":
		return fmt.Errorf("ui review runtime is not available")
	default:
		return fmt.Errorf("research runtime is not available")
	}
}

func isNilResearchModeExecutor(executor researchModeExecutor) bool {
	if executor == nil {
		return true
	}
	value := reflect.ValueOf(executor)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func (d *ResearchDriver) loadLocalSnapshot(controller *harness.Controller, run *harness.Run) *harness.Run {
	if controller != nil && run != nil {
		if stored, err := controller.GetStored(context.Background(), run.ID); err == nil && stored != nil {
			return cloneRunSnapshot(stored)
		}
	}
	return cloneRunSnapshot(run)
}

func (d *ResearchDriver) handleLocalProgress(
	controller *harness.Controller,
	runID string,
	mode string,
	card map[string]interface{},
) {
	if controller == nil || strings.TrimSpace(runID) == "" || len(card) == 0 {
		return
	}
	run, err := controller.GetStored(context.Background(), runID)
	if err != nil || run == nil {
		return
	}
	snapshot := cloneRunSnapshot(run)
	if snapshot.Metadata == nil {
		snapshot.Metadata = map[string]interface{}{}
	}
	snapshot.Metadata["mode"] = mode
	if step := strings.TrimSpace(fmt.Sprint(card["step"])); step != "" {
		snapshot.Metadata["stage"] = step
	}
	if name := strings.TrimSpace(fmt.Sprint(card["name"])); name != "" {
		snapshot.Metadata["latest_action"] = name
	}
	snapshot.Status = harness.RunStatusExecuting
	snapshot.Progress = d.trackLocalProgress(runID, strings.TrimSpace(fmt.Sprint(card["step"])), strings.TrimSpace(fmt.Sprint(card["status"])))
	snapshot.UpdatedAt = timeutil.NowTime()
	_ = controller.SyncSnapshot(context.Background(), snapshot)
	d.appendLocalProgressEvent(controller, snapshot, card)
	d.publishResearchJobEvent(snapshot, "deep_research.job_updated")
}

func (d *ResearchDriver) appendLocalProgressEvent(
	controller *harness.Controller,
	run *harness.Run,
	card map[string]interface{},
) {
	if controller == nil || run == nil || len(card) == 0 {
		return
	}
	payloadJSON := ""
	if raw, err := json.Marshal(card); err == nil {
		payloadJSON = string(raw)
	}
	status := strings.TrimSpace(fmt.Sprint(card["status"]))
	eventType := "state_changed"
	switch status {
	case "running":
		eventType = "step_started"
	case "success", "skipped":
		eventType = "step_finished"
	case "failed":
		eventType = "step_failed"
	}
	_ = controller.AppendEvent(context.Background(), harness.RunEvent{
		RunID:       run.ID,
		RootRunID:   run.RootRunID,
		ParentRunID: run.ParentRunID,
		Type:        eventType,
		Message:     strings.TrimSpace(fmt.Sprint(card["name"])),
		PayloadJSON: payloadJSON,
		CreatedAt:   timeutil.NowTime(),
	})
}

func (d *ResearchDriver) storeLocalRun(runID string, cancel context.CancelFunc) {
	if d == nil || strings.TrimSpace(runID) == "" || cancel == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.local == nil {
		d.local = make(map[string]*localResearchRunState)
	}
	d.local[runID] = &localResearchRunState{
		cancel: cancel,
		steps:  map[string]string{},
	}
}

func (d *ResearchDriver) clearLocalRun(runID string) {
	if d == nil || strings.TrimSpace(runID) == "" {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.local, runID)
}

func (d *ResearchDriver) cancelLocalRun(runID string) {
	if d == nil || strings.TrimSpace(runID) == "" {
		return
	}
	d.mu.Lock()
	state := d.local[runID]
	d.mu.Unlock()
	if state != nil && state.cancel != nil {
		state.cancel()
	}
}

func (d *ResearchDriver) trackLocalProgress(runID string, step string, status string) int {
	if d == nil || strings.TrimSpace(runID) == "" {
		return 0
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.local == nil {
		return 0
	}
	state := d.local[runID]
	if state == nil {
		return 0
	}
	if state.steps == nil {
		state.steps = map[string]string{}
	}
	step = strings.TrimSpace(step)
	status = strings.TrimSpace(status)
	if step != "" {
		previous := state.steps[step]
		state.steps[step] = status
		if previous != status && (status == "success" || status == "skipped" || status == "failed") {
			state.completions++
		}
	}
	total := len(state.steps)
	if total == 0 {
		return 1
	}
	progress := (state.completions * 90) / total
	if progress < 1 {
		progress = 1
	}
	if progress > 95 {
		progress = 95
	}
	return progress
}

func (d *ResearchDriver) publishResearchJobEvent(run *harness.Run, eventType string) {
	if d == nil || run == nil || strings.TrimSpace(run.ID) == "" {
		return
	}
	payload := map[string]interface{}{
		"id":              run.ID,
		"job_id":          run.ID,
		"query":           strings.TrimSpace(run.Goal),
		"status":          strings.TrimSpace(string(run.Status)),
		"stage":           metadataString(run.Metadata, "stage"),
		"progress":        run.Progress,
		"iteration":       metadataInt(run.Metadata["iteration"]),
		"latest_action":   metadataString(run.Metadata, "latest_action"),
		"latest_gap":      metadataString(run.Metadata, "latest_gap"),
		"conversation_id": strings.TrimSpace(run.ConversationID),
		"updated_at":      run.UpdatedAt,
	}
	d.Publish(run.UserID, eventType, payload)
}

func mapResearchEventType(eventType string) string {
	switch strings.TrimSpace(eventType) {
	case "deep_research.job_created":
		return "run_created"
	case "deep_research.job_updated":
		return "state_changed"
	case "deep_research.job_completed":
		return "run_completed"
	case "deep_research.job_failed":
		return "run_failed"
	case "deep_research.job_cancelled":
		return "run_cancelled"
	default:
		return eventType
	}
}

func extractResearchJobID(data any) string {
	switch payload := data.(type) {
	case map[string]interface{}:
		if v, ok := payload["job_id"].(string); ok {
			return strings.TrimSpace(v)
		}
		if v, ok := payload["id"].(string); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func jobToRun(existing *harness.Run, job *deepresearch.Job) *harness.Run {
	run := &harness.Run{}
	if existing != nil {
		*run = *existing
		run.Metadata = cloneMap(existing.Metadata)
	}
	run.ID = job.ID
	run.Kind = harness.RunKindResearch
	run.UserID = job.UserID
	run.ConversationID = job.ConversationID
	run.SessionID = job.ConversationID
	run.Goal = job.Query
	run.ProviderID = strings.TrimSpace(job.ProviderID)
	run.Status = jobStatusToRunStatus(job.Status)
	run.Progress = job.Progress
	run.Result = ""
	if job.Report != nil {
		run.Result = strings.TrimSpace(job.Report.Answer)
	}
	run.Error = strings.TrimSpace(job.Error)
	run.CreatedAt = job.CreatedAt
	run.UpdatedAt = job.UpdatedAt
	if !job.CreatedAt.IsZero() {
		started := job.CreatedAt
		run.StartedAt = &started
	}
	if job.CompletedAt != nil && !job.CompletedAt.IsZero() {
		finished := *job.CompletedAt
		run.FinishedAt = &finished
	}
	if run.Metadata == nil {
		run.Metadata = map[string]interface{}{}
	}
	run.Metadata["stage"] = strings.TrimSpace(job.Stage)
	run.Metadata["iteration"] = job.Iteration
	run.Metadata["latest_action"] = strings.TrimSpace(job.LatestAction)
	run.Metadata["latest_gap"] = strings.TrimSpace(job.LatestGap)
	if strings.TrimSpace(string(job.Mode)) != "" {
		run.Metadata["mode"] = "deep_research"
		run.Metadata["research_depth"] = string(job.Mode)
	}
	if strings.TrimSpace(string(job.RequestedRouteMode)) != "" {
		run.Metadata["route_mode"] = string(job.RequestedRouteMode)
	}
	if strings.TrimSpace(job.Lang) != "" {
		run.Metadata["lang"] = strings.TrimSpace(job.Lang)
	}
	if strings.TrimSpace(job.ReportStyle) != "" {
		run.Metadata["report_style"] = strings.TrimSpace(job.ReportStyle)
	}
	if strings.TrimSpace(job.RetryContext) != "" {
		run.Metadata["retry_context"] = strings.TrimSpace(job.RetryContext)
	}
	if strings.TrimSpace(job.ProviderID) != "" {
		run.Metadata["provider_id"] = strings.TrimSpace(job.ProviderID)
	}
	if len(job.RetryFeedback) > 0 {
		run.Metadata["retry_feedback"] = cloneMap(job.RetryFeedback)
	}
	if sources := researchSourceMetadata(job.Evidence); len(sources) > 0 {
		run.Metadata["source_inventory"] = sources
	} else if job.Report != nil {
		if citations := researchCitationMetadata(job.Report.Citations); len(citations) > 0 {
			run.Metadata["citations"] = citations
		}
	}
	if job.Report != nil && job.Report.Calibration != nil {
		run.Metadata["calibration"] = calibrationMetadata(job.Report.Calibration)
		run.Metadata["calibration_ref"] = "deep_research:" + strings.TrimSpace(job.ID) + ":calibration"
		run.Metadata["takeaway_candidates"] = takeawayCandidateMetadata(job.Report.Calibration.TakeawayCandidates)
	}
	return run
}

func jobStatusToRunStatus(status deepresearch.JobStatus) harness.RunStatus {
	switch status {
	case deepresearch.JobStatusRunning, deepresearch.JobStatusSynthesizing:
		return harness.RunStatusExecuting
	case deepresearch.JobStatusCompleted:
		return harness.RunStatusCompleted
	case deepresearch.JobStatusFailed:
		return harness.RunStatusFailed
	case deepresearch.JobStatusCancelled:
		return harness.RunStatusCancelled
	default:
		return harness.RunStatusPending
	}
}

func resolveResearchFamilyMode(goal string, metadata map[string]interface{}) string {
	mode := strings.ToLower(strings.TrimSpace(metadataString(metadata, "mode")))
	switch mode {
	case "analyze", "ui_review", "deep_research":
		return mode
	case "fast", "standard", "deep":
		if metadata != nil && strings.TrimSpace(metadataString(metadata, "research_depth")) == "" {
			metadata["research_depth"] = mode
		}
		return "deep_research"
	case "auto", "":
		if looksLikeUIReviewMode(goal, metadata) {
			return "ui_review"
		}
		if looksLikeAnalyzeMode(goal, metadata) {
			return "analyze"
		}
		return "deep_research"
	default:
		return mode
	}
}

func resolveResearchDepth(metadata map[string]interface{}) string {
	depth := strings.ToLower(strings.TrimSpace(metadataString(metadata, "research_depth")))
	switch depth {
	case "fast", "standard", "deep":
		return depth
	}
	mode := strings.ToLower(strings.TrimSpace(metadataString(metadata, "mode")))
	switch mode {
	case "fast", "standard", "deep":
		return mode
	default:
		return ""
	}
}

func looksLikeAnalyzeMode(goal string, metadata map[string]interface{}) bool {
	if strings.TrimSpace(metadataString(metadata, "topic")) != "" ||
		len(metadataStringSlice(metadata["search_queries"])) > 0 ||
		len(metadataStringSlice(metadata["urls"])) > 0 ||
		strings.TrimSpace(metadataString(metadata, "text")) != "" {
		return true
	}
	q := strings.ToLower(strings.TrimSpace(goal))
	return strings.Contains(q, "analyze") || strings.Contains(q, "analysis") || strings.Contains(q, "report") || strings.Contains(q, "summarize")
}

func looksLikeUIReviewMode(goal string, metadata map[string]interface{}) bool {
	if strings.TrimSpace(metadataString(metadata, "image")) != "" {
		return true
	}
	action := strings.ToLower(strings.TrimSpace(metadataString(metadata, "action")))
	switch action {
	case "review_url", "review_image", "check_accessibility":
		return true
	}
	q := strings.ToLower(strings.TrimSpace(goal))
	return strings.Contains(q, "ui") || strings.Contains(q, "ux") || strings.Contains(q, "accessibility") || strings.Contains(q, "screenshot")
}

func withLocalResearchToolContext(ctx context.Context, run *harness.Run) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if run == nil {
		return ctx
	}
	if lang := strings.TrimSpace(metadataString(run.Metadata, "lang")); lang != "" {
		ctx = tools.WithLang(ctx, lang)
	}
	if channel := strings.TrimSpace(metadataString(run.Metadata, "channel")); channel != "" {
		ctx = tools.WithChannel(ctx, channel)
	}
	if device := strings.TrimSpace(metadataString(run.Metadata, "device")); device != "" {
		ctx = tools.WithDevice(ctx, device)
	}
	return ctx
}

func researchToolArgs(run *harness.Run, mode string) map[string]interface{} {
	args := map[string]interface{}{}
	if run != nil && run.Metadata != nil {
		args = cloneMap(run.Metadata)
	}
	if args == nil {
		args = map[string]interface{}{}
	}
	args["mode"] = mode
	switch mode {
	case "analyze":
		if strings.TrimSpace(metadataString(args, "topic")) == "" && run != nil {
			args["topic"] = strings.TrimSpace(run.Goal)
		}
	case "ui_review":
		if strings.TrimSpace(metadataString(args, "url")) == "" && run != nil && looksLikeURL(run.Goal) {
			args["url"] = strings.TrimSpace(run.Goal)
		}
	}
	return args
}

func normalizeResearchExecutorResult(result interface{}) string {
	switch typed := result.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case []byte:
		return strings.TrimSpace(string(typed))
	default:
		raw, err := json.Marshal(typed)
		if err != nil {
			return strings.TrimSpace(fmt.Sprint(result))
		}
		return string(raw)
	}
}

func cloneRunSnapshot(run *harness.Run) *harness.Run {
	if run == nil {
		return nil
	}
	snapshot := *run
	snapshot.Metadata = cloneMap(run.Metadata)
	return &snapshot
}

func researchDriverManager(primary *harness.Controller, secondary *harness.Controller) *harness.Controller {
	if secondary != nil {
		return secondary
	}
	return primary
}

func looksLikeURL(value string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://")
}

func metadataStringSlice(raw any) []string {
	items, ok := raw.([]interface{})
	if !ok {
		if typed, ok := raw.([]string); ok {
			return append([]string(nil), typed...)
		}
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if value := strings.TrimSpace(fmt.Sprint(item)); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func metadataInt(raw any) int {
	switch value := raw.(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return 0
	}
}

func metadataMap(raw any) map[string]interface{} {
	typed, _ := raw.(map[string]interface{})
	return cloneMap(typed)
}

func metadataBool(raw any) (bool, bool) {
	v, ok := raw.(bool)
	return v, ok
}

func maxDurationSeconds(duration time.Duration, fallback any) int {
	if duration > 0 {
		return int(duration.Seconds())
	}
	return metadataInt(fallback)
}

func calibrationMetadata(calibration *deepresearch.Calibration) map[string]interface{} {
	if calibration == nil {
		return nil
	}
	return map[string]interface{}{
		"coverage":           calibration.Coverage,
		"groundedness":       calibration.Groundedness,
		"freshness":          calibration.Freshness,
		"conflict_risk":      strings.TrimSpace(calibration.ConflictRisk),
		"confidence":         calibration.Confidence,
		"recommended_action": strings.TrimSpace(calibration.RecommendedAction),
	}
}

func takeawayCandidateMetadata(candidates []deepresearch.TakeawayCandidate) []map[string]interface{} {
	if len(candidates) == 0 {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(candidates))
	for _, candidate := range candidates {
		out = append(out, map[string]interface{}{
			"lesson":        strings.TrimSpace(candidate.Lesson),
			"when_to_apply": strings.TrimSpace(candidate.WhenToApply),
			"evidence":      strings.TrimSpace(candidate.Evidence),
			"evidence_ids":  append([]string(nil), candidate.EvidenceIDs...),
			"confidence":    candidate.Confidence,
			"target_file":   strings.TrimSpace(candidate.TargetFile),
		})
	}
	return out
}

func researchSourceMetadata(evidence []deepresearch.Evidence) []map[string]interface{} {
	if len(evidence) == 0 {
		return nil
	}
	out := make([]map[string]interface{}, 0, min(len(evidence), 8))
	seen := make(map[string]struct{}, len(evidence))
	for _, item := range evidence {
		url := strings.TrimSpace(item.URL)
		title := strings.TrimSpace(item.Title)
		key := url
		if key == "" {
			key = title
		}
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		record := map[string]interface{}{
			"title":             title,
			"url":               url,
			"domain":            strings.TrimSpace(item.Domain),
			"source_type":       strings.TrimSpace(item.Source),
			"relevance_score":   item.RelevanceScore,
			"credibility_score": item.CredibilityScore,
		}
		if !item.FetchedAt.IsZero() {
			record["fetched_at"] = item.FetchedAt.UTC().Format(time.RFC3339)
		}
		if item.PublishedAt != nil && !item.PublishedAt.IsZero() {
			record["published_at"] = item.PublishedAt.UTC().Format(time.RFC3339)
		}
		out = append(out, record)
		if len(out) >= 8 {
			break
		}
	}
	return out
}

func researchCitationMetadata(citations []deepresearch.Citation) []map[string]interface{} {
	if len(citations) == 0 {
		return nil
	}
	out := make([]map[string]interface{}, 0, min(len(citations), 8))
	seen := make(map[string]struct{}, len(citations))
	for _, item := range citations {
		url := strings.TrimSpace(item.URL)
		title := strings.TrimSpace(item.Title)
		key := url
		if key == "" {
			key = title
		}
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, map[string]interface{}{
			"title": title,
			"url":   url,
		})
		if len(out) >= 8 {
			break
		}
	}
	return out
}
