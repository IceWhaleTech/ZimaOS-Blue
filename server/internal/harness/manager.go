package harness

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

type Controller struct {
	store          *SQLiteStore
	resolver       *PolicyResolver
	judgeEvaluator JudgeEvaluator
	reflector      ProposalReflector
	runTrace       RunTraceProvider

	mu          sync.RWMutex
	drivers     map[RunKind]Driver
	middlewares []ExecutionMiddleware
}

func NewController(store *SQLiteStore, resolver *PolicyResolver) *Controller {
	return &Controller{
		store:    store,
		resolver: resolver,
		drivers:  make(map[RunKind]Driver),
	}
}

func (c *Controller) RegisterDriver(driver Driver) {
	if c == nil || driver == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.drivers[driver.Kind()] = driver
}

func (c *Controller) GetRegisteredDriver(kind RunKind) Driver {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.drivers[kind]
}

func (c *Controller) UseExecutionMiddleware(mw ExecutionMiddleware) {
	if c == nil || mw == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.middlewares = append(c.middlewares, mw)
}

func (c *Controller) ExecutionMiddlewares() []ExecutionMiddleware {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.middlewares) == 0 {
		return nil
	}
	out := make([]ExecutionMiddleware, len(c.middlewares))
	copy(out, c.middlewares)
	return out
}

func (c *Controller) Submit(ctx context.Context, spec RunSpec) (*Run, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	if err := validateRunSpec(spec); err != nil {
		return nil, err
	}
	spec = c.resolver.Resolve(spec)
	if err := validateRunSpec(spec); err != nil {
		return nil, err
	}
	driver, err := c.driverFor(spec.Kind)
	if err != nil {
		return nil, err
	}
	slog.Info("Harness submit driver resolved",
		"controller", fmt.Sprintf("%p", c),
		"kind", spec.Kind,
		"driver_type", fmt.Sprintf("%T", driver),
		"metadata_driver", strings.TrimSpace(fmt.Sprint(spec.Metadata["driver"])),
		"group_id", strings.TrimSpace(spec.GroupID),
		"group_item_id", strings.TrimSpace(spec.GroupItemID),
	)
	if err := driver.Validate(spec); err != nil {
		return nil, err
	}

	now := timeutil.NowTime()
	run := &Run{
		ID:             uuid.NewString(),
		RootRunID:      "",
		ParentRunID:    strings.TrimSpace(spec.ParentRunID),
		GroupID:        strings.TrimSpace(spec.GroupID),
		GroupItemID:    strings.TrimSpace(spec.GroupItemID),
		AttemptIndex:   spec.AttemptIndex,
		Kind:           spec.Kind,
		Status:         initialStatusForKind(spec.Kind),
		UserID:         strings.TrimSpace(spec.UserID),
		ConversationID: strings.TrimSpace(spec.ConversationID),
		SessionID:      strings.TrimSpace(spec.SessionID),
		AgentID:        strings.TrimSpace(spec.AgentID),
		Goal:           strings.TrimSpace(spec.Goal),
		Model:          strings.TrimSpace(spec.Model),
		Depth:          0,
		WorkspaceRoot:  strings.TrimSpace(spec.WorkspaceRoot),
		ArtifactRoot:   c.resolver.ArtifactRoot(uuid.Nil.String()),
		SandboxMode:    strings.TrimSpace(spec.SandboxMode),
		ApprovalMode:   spec.ApprovalMode,
		MaxDuration:    spec.MaxDuration,
		MaxSteps:       spec.MaxSteps,
		MaxToolRounds:  spec.MaxToolRounds,
		MaxSubagents:   spec.MaxSubagents,
		MaxDepth:       spec.MaxDepth,
		Metadata:       cloneMetadataMap(spec.Metadata),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	run.ID = uuid.NewString()
	run.RootRunID = run.ID
	run.ArtifactRoot = c.resolver.ArtifactRoot(run.ID)
	if err := c.store.CreateRun(ctx, run); err != nil {
		return nil, err
	}
	if err := c.ensureWorkspaceRoot(run.WorkspaceRoot); err != nil {
		_ = c.store.DeleteRun(ctx, run.ID)
		return nil, err
	}
	if err := c.ensureArtifactRoot(run.ArtifactRoot); err != nil {
		_ = c.store.DeleteRun(ctx, run.ID)
		return nil, err
	}
	if err := c.AppendEvent(ctx, RunEvent{
		RunID:     run.ID,
		RootRunID: run.RootRunID,
		Type:      "run_created",
		Message:   "run created",
		CreatedAt: now,
	}); err != nil {
		return nil, err
	}
	_ = c.appendStageEvent(ctx, run, RuntimeStageNormalize, "run normalized", guardStagePayload(run))
	_ = c.appendStageEvent(ctx, run, RuntimeStagePolicy, "runtime policy resolved", guardStagePayload(run))
	_ = c.appendStageEvent(ctx, run, RuntimeStageExecute, "driver dispatch started", guardStagePayload(run))
	if err := c.startRun(ctx, run, nil, driver); err != nil {
		run.Status = RunStatusFailed
		run.Error = err.Error()
		finished := timeutil.NowTime()
		run.FinishedAt = &finished
		_ = c.store.UpdateRun(ctx, run)
		_ = c.appendStageEvent(ctx, run, RuntimeStageFinalize, "run dispatch failed", map[string]interface{}{
			"status": RunStatusFailed,
			"error":  strings.TrimSpace(err.Error()),
		})
		_ = c.AppendEvent(ctx, RunEvent{
			RunID:     run.ID,
			RootRunID: run.RootRunID,
			Type:      "run_failed",
			Message:   err.Error(),
			CreatedAt: finished,
		})
		return nil, err
	}
	return c.finishSubmittedRun(ctx, run)
}

func (c *Controller) SpawnChild(ctx context.Context, parentID string, spec RunSpec) (*Run, error) {
	parent, err := c.Get(ctx, parentID)
	if err != nil {
		return nil, err
	}
	children, err := c.store.ListRuns(ctx, RunFilter{ParentRunID: parentID, Limit: parent.MaxSubagents + 1})
	if err != nil {
		return nil, err
	}
	if parent.MaxSubagents > 0 && len(children) >= parent.MaxSubagents {
		return nil, newGuardPipelineError(RuntimeStagePolicy, "budget_exceeded", "max subagents exceeded", map[string]interface{}{
			"max_subagents": parent.MaxSubagents,
			"parent_run_id": parent.ID,
		})
	}
	if err := validateRunSpec(spec); err != nil {
		return nil, err
	}
	spec, err = c.resolver.ResolveChild(parent, spec)
	if err != nil {
		return nil, err
	}
	if err := validateRunSpec(spec); err != nil {
		return nil, err
	}
	driver, err := c.driverFor(spec.Kind)
	if err != nil {
		return nil, err
	}
	if err := driver.Validate(spec); err != nil {
		return nil, err
	}
	now := timeutil.NowTime()
	child := &Run{
		ID:             uuid.NewString(),
		RootRunID:      parent.RootRunID,
		ParentRunID:    parent.ID,
		GroupID:        spec.GroupID,
		GroupItemID:    spec.GroupItemID,
		AttemptIndex:   spec.AttemptIndex,
		Kind:           spec.Kind,
		Status:         initialStatusForKind(spec.Kind),
		UserID:         spec.UserID,
		ConversationID: spec.ConversationID,
		SessionID:      spec.SessionID,
		AgentID:        spec.AgentID,
		Goal:           spec.Goal,
		Model:          spec.Model,
		Depth:          parent.Depth + 1,
		WorkspaceRoot:  spec.WorkspaceRoot,
		ArtifactRoot:   c.resolver.ArtifactRoot(uuid.Nil.String()),
		SandboxMode:    spec.SandboxMode,
		ApprovalMode:   spec.ApprovalMode,
		MaxDuration:    spec.MaxDuration,
		MaxSteps:       spec.MaxSteps,
		MaxToolRounds:  spec.MaxToolRounds,
		MaxSubagents:   spec.MaxSubagents,
		MaxDepth:       spec.MaxDepth,
		Metadata:       cloneMetadataMap(spec.Metadata),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	child.ArtifactRoot = c.resolver.ArtifactRoot(child.ID)
	if err := c.store.CreateRun(ctx, child); err != nil {
		return nil, err
	}
	if err := c.ensureWorkspaceRoot(child.WorkspaceRoot); err != nil {
		_ = c.store.DeleteRun(ctx, child.ID)
		return nil, err
	}
	if err := c.ensureArtifactRoot(child.ArtifactRoot); err != nil {
		_ = c.store.DeleteRun(ctx, child.ID)
		return nil, err
	}
	_ = c.AppendEvent(ctx, RunEvent{RunID: child.ID, RootRunID: child.RootRunID, ParentRunID: child.ParentRunID, Type: "run_created", Message: "run created", CreatedAt: now})
	_ = c.AppendEvent(ctx, RunEvent{RunID: parent.ID, RootRunID: parent.RootRunID, ParentRunID: parent.ParentRunID, Type: "child_spawned", Message: child.ID, CreatedAt: now})
	_ = c.appendStageEvent(ctx, child, RuntimeStageNormalize, "child run normalized", guardStagePayload(child))
	_ = c.appendStageEvent(ctx, child, RuntimeStagePolicy, "child runtime policy resolved", guardStagePayload(child))
	_ = c.appendStageEvent(ctx, child, RuntimeStageExecute, "child driver dispatch started", guardStagePayload(child))
	if err := c.startRun(ctx, child, parent, driver); err != nil {
		child.Status = RunStatusFailed
		child.Error = err.Error()
		finished := timeutil.NowTime()
		child.FinishedAt = &finished
		_ = c.store.UpdateRun(ctx, child)
		_ = c.appendStageEvent(ctx, child, RuntimeStageFinalize, "child run dispatch failed", map[string]interface{}{
			"status": RunStatusFailed,
			"error":  strings.TrimSpace(err.Error()),
		})
		_ = c.AppendEvent(ctx, RunEvent{RunID: child.ID, RootRunID: child.RootRunID, ParentRunID: child.ParentRunID, Type: "run_failed", Message: err.Error(), CreatedAt: finished})
		return nil, err
	}
	return c.finishSubmittedRun(ctx, child)
}

func (c *Controller) finishSubmittedRun(ctx context.Context, run *Run) (*Run, error) {
	if run == nil {
		return nil, nil
	}
	finalRun, err := c.Get(ctx, run.ID)
	if err == nil {
		return finalRun, nil
	}
	if !errorsIsNoRows(err) {
		return nil, err
	}

	// Freshly inserted runs can be temporarily invisible on the reader handle
	// even though creation and driver start have already completed on the writer.
	stored, storedErr := c.GetStored(ctx, run.ID)
	if storedErr != nil {
		return nil, err
	}
	return c.syncRun(ctx, stored)
}

func (c *Controller) Get(ctx context.Context, id string) (*Run, error) {
	run, err := c.GetStored(ctx, id)
	if err != nil {
		return nil, err
	}
	return c.syncRun(ctx, run)
}

func (c *Controller) GetStored(ctx context.Context, id string) (*Run, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	return c.store.GetRun(ctx, id)
}

func (c *Controller) List(ctx context.Context, filter RunFilter) ([]Run, error) {
	runs, err := c.store.ListRuns(ctx, filter)
	if err != nil {
		return nil, err
	}
	for i := range runs {
		synced, err := c.syncRun(ctx, &runs[i])
		if err != nil {
			continue
		}
		runs[i] = *synced
	}
	return runs, nil
}

func (c *Controller) FindRunByMetadata(ctx context.Context, kind RunKind, key string, value string) (*Run, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	run, err := c.store.FindRunByMetadata(ctx, kind, key, value)
	if err != nil || run == nil {
		return run, err
	}
	return c.syncRun(ctx, run)
}

func (c *Controller) ListOne(ctx context.Context, id string) (*Run, error) {
	return c.GetStored(ctx, id)
}

func (c *Controller) Cancel(ctx context.Context, id string, reason string) error {
	run, err := c.store.GetRun(ctx, id)
	if err != nil {
		return err
	}
	children, err := c.store.ListRuns(ctx, RunFilter{ParentRunID: id, Limit: 1000})
	if err == nil {
		for i := range children {
			_ = c.Cancel(ctx, children[i].ID, "parent cancelled")
		}
	}
	driver, err := c.driverFor(run.Kind)
	if err != nil {
		return err
	}
	if err := driver.Cancel(ctx, run); err != nil {
		return err
	}
	now := timeutil.NowTime()
	run.Status = RunStatusCancelled
	run.Error = strings.TrimSpace(reason)
	run.FinishedAt = &now
	if err := c.store.UpdateRun(ctx, run); err != nil {
		return err
	}
	_ = c.appendStageEvent(ctx, run, RuntimeStageFinalize, "run cancellation finalized", map[string]interface{}{
		"status": RunStatusCancelled,
		"reason": strings.TrimSpace(reason),
	})
	return c.AppendEvent(ctx, RunEvent{
		RunID:       run.ID,
		RootRunID:   run.RootRunID,
		ParentRunID: run.ParentRunID,
		Type:        "run_cancelled",
		Message:     strings.TrimSpace(reason),
		CreatedAt:   now,
	})
}

func (c *Controller) PerformAction(ctx context.Context, id string, action string, input map[string]interface{}) (*Run, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	run, err := c.store.GetRun(ctx, id)
	if err != nil {
		return nil, err
	}
	action = strings.TrimSpace(action)
	if action == "" {
		return nil, fmt.Errorf("action is required")
	}
	if strings.EqualFold(action, "cancel") {
		reason := strings.TrimSpace(metadataString(input, "reason"))
		if reason == "" {
			reason = "cancelled by user"
		}
		if err := c.Cancel(ctx, run.ID, reason); err != nil {
			return nil, err
		}
		now := timeutil.NowTime()
		_ = c.AppendEvent(ctx, RunEvent{
			RunID:       run.ID,
			RootRunID:   run.RootRunID,
			ParentRunID: run.ParentRunID,
			Type:        "run_action",
			Message:     "cancel",
			CreatedAt:   now,
		})
		return c.Get(ctx, run.ID)
	}
	driver, err := c.driverFor(run.Kind)
	if err != nil {
		return nil, err
	}
	controller, ok := driver.(ActionDriver)
	if !ok {
		return nil, fmt.Errorf("run kind %q does not support action %q", run.Kind, action)
	}
	snapshot, err := controller.PerformAction(ctx, run, action, cloneMetadataMap(input))
	if err != nil {
		return nil, err
	}
	if snapshot != nil {
		if strings.TrimSpace(snapshot.ID) == "" {
			snapshot.ID = run.ID
		}
		if snapshot.Kind == "" {
			snapshot.Kind = run.Kind
		}
		if err := c.SyncSnapshot(ctx, snapshot); err != nil {
			return nil, err
		}
	}
	now := timeutil.NowTime()
	_ = c.AppendEvent(ctx, RunEvent{
		RunID:       run.ID,
		RootRunID:   run.RootRunID,
		ParentRunID: run.ParentRunID,
		Type:        "run_action",
		Message:     action,
		CreatedAt:   now,
	})
	return c.Get(ctx, run.ID)
}

func (c *Controller) AppendEvent(ctx context.Context, event RunEvent) error {
	if strings.TrimSpace(event.RunID) == "" {
		return fmt.Errorf("run_id is required")
	}
	if strings.TrimSpace(event.ID) == "" {
		event.ID = uuid.NewString()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = timeutil.NowTime()
	}
	if event.Type == "" {
		event.Type = "run_event"
	}
	run, err := c.store.GetRun(ctx, event.RunID)
	if err == nil {
		if event.RootRunID == "" {
			event.RootRunID = run.RootRunID
		}
		if event.ParentRunID == "" {
			event.ParentRunID = run.ParentRunID
		}
	}
	return c.store.AppendEvent(ctx, event)
}

func (c *Controller) AttachArtifact(ctx context.Context, ref ArtifactRef) error {
	if strings.TrimSpace(ref.ID) == "" {
		ref.ID = uuid.NewString()
	}
	if err := c.store.AttachArtifact(ctx, ref); err != nil {
		return err
	}
	return c.AppendEvent(ctx, RunEvent{
		RunID:       ref.RunID,
		Type:        "artifact_attached",
		Message:     strings.TrimSpace(ref.Label),
		PayloadJSON: ref.MetadataJSON,
		CreatedAt:   timeutil.NowTime(),
	})
}

func (c *Controller) ListEvents(ctx context.Context, runID string, limit int) ([]RunEvent, error) {
	return c.store.ListEvents(ctx, runID, limit)
}

func (c *Controller) ListArtifacts(ctx context.Context, runID string) ([]ArtifactRef, error) {
	return c.store.ListArtifacts(ctx, runID)
}

func (c *Controller) Delete(ctx context.Context, id string) error {
	if c == nil || c.store == nil {
		return fmt.Errorf("harness controller is not configured")
	}
	return c.store.DeleteRun(ctx, id)
}

func (c *Controller) SyncSnapshot(ctx context.Context, snapshot *Run) error {
	if snapshot == nil {
		return nil
	}
	current, err := c.store.GetRun(ctx, snapshot.ID)
	if err != nil {
		return err
	}
	snapshot.RootRunID = current.RootRunID
	snapshot.ParentRunID = current.ParentRunID
	snapshot.GroupID = current.GroupID
	snapshot.GroupItemID = current.GroupItemID
	if snapshot.AttemptIndex <= 0 {
		snapshot.AttemptIndex = current.AttemptIndex
	}
	if snapshot.CreatedAt.IsZero() {
		snapshot.CreatedAt = current.CreatedAt
	}
	if snapshot.ArtifactRoot == "" {
		snapshot.ArtifactRoot = current.ArtifactRoot
	}
	if snapshot.WorkspaceRoot == "" {
		snapshot.WorkspaceRoot = current.WorkspaceRoot
	}
	if snapshot.ApprovalMode == "" {
		snapshot.ApprovalMode = current.ApprovalMode
	}
	if snapshot.SandboxMode == "" {
		snapshot.SandboxMode = current.SandboxMode
	}
	if snapshot.MaxDuration <= 0 {
		snapshot.MaxDuration = current.MaxDuration
	}
	if snapshot.MaxSteps <= 0 {
		snapshot.MaxSteps = current.MaxSteps
	}
	if snapshot.MaxToolRounds <= 0 {
		snapshot.MaxToolRounds = current.MaxToolRounds
	}
	if snapshot.MaxSubagents <= 0 {
		snapshot.MaxSubagents = current.MaxSubagents
	}
	if snapshot.MaxDepth <= 0 {
		snapshot.MaxDepth = current.MaxDepth
	}
	if snapshot.Metadata == nil {
		snapshot.Metadata = current.Metadata
	}
	normalizeRunLifecycleTimes(current, snapshot)
	if err := c.store.UpdateRun(ctx, snapshot); err != nil {
		return err
	}
	if current.Status != snapshot.Status && isTerminalRunStatus(snapshot.Status) {
		_ = c.appendStageEvent(ctx, snapshot, RuntimeStageFinalize, "run terminal state synced", map[string]interface{}{
			"status": snapshot.Status,
			"error":  strings.TrimSpace(snapshot.Error),
		})
	}
	return nil
}

func (c *Controller) driverFor(kind RunKind) (Driver, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	driver := c.drivers[kind]
	if driver == nil {
		return nil, fmt.Errorf("driver not registered for kind %q", kind)
	}
	return driver, nil
}

func (c *Controller) syncRun(ctx context.Context, run *Run) (*Run, error) {
	if run == nil {
		return nil, nil
	}
	driver, err := c.driverFor(run.Kind)
	if err != nil {
		normalizeRunLifecycleTimes(nil, run)
		return run, nil
	}
	syncer, ok := driver.(SnapshotDriver)
	if !ok {
		normalizeRunLifecycleTimes(nil, run)
		return run, nil
	}
	updated, err := syncer.Sync(ctx, run)
	if err != nil || updated == nil {
		normalizeRunLifecycleTimes(nil, run)
		return run, nil
	}
	if normalizeRunLifecycleTimes(run, updated) {
		if updateErr := c.store.UpdateRun(ctx, updated); updateErr != nil {
			return updated, nil
		}
	}
	return updated, nil
}

func (c *Controller) startRun(ctx context.Context, run *Run, parent *Run, driver Driver) error {
	runCtx := &RunContext{
		Run:        run,
		Parent:     parent,
		Driver:     driver,
		Controller: c,
		Values:     make(map[string]interface{}),
	}
	ctx = WithRunContext(ctx, runCtx)
	ctx = annotateRunExecutionContext(ctx, run)
	middlewares := c.ExecutionMiddlewares()
	for _, mw := range middlewares {
		if err := mw.BeforeStart(ctx, runCtx); err != nil {
			for i := len(middlewares) - 1; i >= 0; i-- {
				middlewares[i].OnStartError(ctx, runCtx, err)
			}
			return err
		}
	}
	if err := driver.Start(ctx, run, RunEnv{Manager: c, RunContext: runCtx}); err != nil {
		for i := len(middlewares) - 1; i >= 0; i-- {
			middlewares[i].OnStartError(ctx, runCtx, err)
		}
		return err
	}
	for _, mw := range middlewares {
		mw.AfterStart(ctx, runCtx)
	}
	return nil
}

func initialStatusForKind(kind RunKind) RunStatus {
	switch kind {
	case RunKindResearch:
		return RunStatusPending
	default:
		return RunStatusPending
	}
}

func normalizeRunLifecycleTimes(current *Run, snapshot *Run) bool {
	if snapshot == nil {
		return false
	}
	changed := false
	if snapshot.Status != RunStatusPending && snapshot.StartedAt == nil {
		if current != nil && current.StartedAt != nil {
			snapshot.StartedAt = cloneRunTimePtr(current.StartedAt)
		} else {
			now := timeutil.NowTime()
			snapshot.StartedAt = &now
		}
		changed = true
	}
	if isTerminalRunStatus(snapshot.Status) && snapshot.FinishedAt == nil {
		if current != nil && current.FinishedAt != nil {
			snapshot.FinishedAt = cloneRunTimePtr(current.FinishedAt)
		} else {
			now := timeutil.NowTime()
			snapshot.FinishedAt = &now
		}
		changed = true
	}
	return changed
}

func cloneRunTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func (c *Controller) ensureArtifactRoot(path string) error {
	clean := strings.TrimSpace(path)
	if clean == "" {
		return nil
	}
	if err := os.MkdirAll(clean, 0o755); err != nil {
		return fmt.Errorf("create artifact root: %w", err)
	}
	return nil
}

func (c *Controller) ensureWorkspaceRoot(path string) error {
	clean := strings.TrimSpace(path)
	if clean == "" {
		return nil
	}
	if err := os.MkdirAll(clean, 0o755); err != nil {
		return fmt.Errorf("create workspace root: %w", err)
	}
	return nil
}
