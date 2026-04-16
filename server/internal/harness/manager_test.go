package harness

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type stubDriver struct {
	kind          RunKind
	started       []string
	cancelled     []string
	actions       []string
	actionInputs  []map[string]interface{}
	actionResults map[string]RunStatus
	actionErr     error
}

func (d *stubDriver) Kind() RunKind { return d.kind }
func (d *stubDriver) Validate(spec RunSpec) error {
	if spec.Goal == "" {
		return fmt.Errorf("goal is required")
	}
	return nil
}
func (d *stubDriver) Start(_ context.Context, run *Run, _ RunEnv) error {
	d.started = append(d.started, run.ID)
	return nil
}
func (d *stubDriver) Cancel(_ context.Context, run *Run) error {
	d.cancelled = append(d.cancelled, run.ID)
	return nil
}
func (d *stubDriver) PerformAction(_ context.Context, run *Run, action string, input map[string]interface{}) (*Run, error) {
	if d.actionErr != nil {
		return nil, d.actionErr
	}
	nextStatus, ok := d.actionResults[action]
	if !ok {
		return nil, fmt.Errorf("unsupported run action %q", action)
	}
	d.actions = append(d.actions, action)
	d.actionInputs = append(d.actionInputs, cloneMetadataMap(input))
	snapshot := *run
	snapshot.Metadata = cloneMetadataMap(run.Metadata)
	snapshot.Status = nextStatus
	return &snapshot, nil
}

type runContextDriver struct {
	kind             RunKind
	order            *[]string
	lastEnv          RunEnv
	lastRunCtx       *RunContext
	toolRunID        string
	toolUserID       string
	toolSession      string
	toolProvider     string
	toolModel        string
	toolAgentID      string
	toolArtifactRoot string
	artifactToEmit   *tools.ToolArtifact
	eventToEmit      *tools.ToolEvent
	emitArtifactErr  error
	emitEventErr     error
	startErr         error
	startedCount     int
}

func (d *runContextDriver) Kind() RunKind { return d.kind }
func (d *runContextDriver) Validate(spec RunSpec) error {
	if spec.Goal == "" {
		return fmt.Errorf("goal is required")
	}
	return nil
}
func (d *runContextDriver) Start(ctx context.Context, run *Run, env RunEnv) error {
	d.startedCount++
	d.lastEnv = env
	d.lastRunCtx = GetRunContext(ctx)
	d.toolRunID = tools.GetRunID(ctx)
	d.toolUserID = tools.GetUserID(ctx)
	d.toolSession = tools.GetSessionID(ctx)
	d.toolProvider = tools.GetProviderID(ctx)
	d.toolModel = tools.GetModel(ctx)
	d.toolAgentID = tools.GetAgentID(ctx)
	d.toolArtifactRoot = tools.GetRunArtifactRoot(ctx)
	if d.order != nil {
		*d.order = append(*d.order, "start")
	}
	if d.artifactToEmit != nil {
		d.emitArtifactErr = tools.EmitArtifact(ctx, *d.artifactToEmit)
	}
	if d.eventToEmit != nil {
		d.emitEventErr = tools.EmitEvent(ctx, *d.eventToEmit)
	}
	if d.startErr != nil {
		return d.startErr
	}
	if run != nil {
		return env.Manager.SyncSnapshot(ctx, run)
	}
	return nil
}
func (d *runContextDriver) Cancel(_ context.Context, _ *Run) error { return nil }

type recursiveListOneDriver struct {
	kind       RunKind
	controller *Controller
	syncCalls  int
}

func (d *recursiveListOneDriver) Kind() RunKind { return d.kind }
func (d *recursiveListOneDriver) Validate(spec RunSpec) error {
	if spec.Goal == "" {
		return fmt.Errorf("goal is required")
	}
	return nil
}
func (d *recursiveListOneDriver) Start(_ context.Context, _ *Run, _ RunEnv) error { return nil }
func (d *recursiveListOneDriver) Cancel(_ context.Context, _ *Run) error          { return nil }
func (d *recursiveListOneDriver) Sync(ctx context.Context, run *Run) (*Run, error) {
	d.syncCalls++
	if d.syncCalls > 8 {
		return nil, fmt.Errorf("recursive sync detected")
	}
	snapshot := *run
	snapshot.Status = RunStatusExecuting
	if err := d.controller.SyncSnapshot(ctx, &snapshot); err != nil {
		return nil, err
	}
	return d.controller.ListOne(ctx, run.ID)
}

type stagedSnapshotDriver struct {
	kind         RunKind
	controller   *Controller
	syncStatuses []RunStatus
	syncCalls    int
}

func (d *stagedSnapshotDriver) Kind() RunKind { return d.kind }
func (d *stagedSnapshotDriver) Validate(spec RunSpec) error {
	if spec.Goal == "" {
		return fmt.Errorf("goal is required")
	}
	return nil
}
func (d *stagedSnapshotDriver) Start(_ context.Context, _ *Run, _ RunEnv) error { return nil }
func (d *stagedSnapshotDriver) Cancel(_ context.Context, _ *Run) error          { return nil }
func (d *stagedSnapshotDriver) Sync(ctx context.Context, run *Run) (*Run, error) {
	if run == nil {
		return nil, nil
	}
	snapshot := *run
	if len(d.syncStatuses) > 0 {
		index := d.syncCalls
		if index >= len(d.syncStatuses) {
			index = len(d.syncStatuses) - 1
		}
		snapshot.Status = d.syncStatuses[index]
		d.syncCalls++
	}
	if snapshot.Status != RunStatusPending && snapshot.StartedAt == nil {
		started := time.Now().UTC()
		snapshot.StartedAt = &started
	}
	if isTerminalRunStatus(snapshot.Status) && snapshot.FinishedAt == nil {
		finished := time.Now().UTC()
		snapshot.FinishedAt = &finished
	}
	if err := d.controller.SyncSnapshot(ctx, &snapshot); err != nil {
		return nil, err
	}
	return d.controller.ListOne(ctx, run.ID)
}

type fixedSnapshotDriver struct {
	kind       RunKind
	startState RunStatus
	syncResult *Run
}

func (d *fixedSnapshotDriver) Kind() RunKind { return d.kind }
func (d *fixedSnapshotDriver) Validate(spec RunSpec) error {
	if spec.Goal == "" {
		return fmt.Errorf("goal is required")
	}
	return nil
}
func (d *fixedSnapshotDriver) Start(ctx context.Context, run *Run, env RunEnv) error {
	if run == nil {
		return nil
	}
	snapshot := *run
	snapshot.Status = d.startState
	if snapshot.Status == "" {
		snapshot.Status = RunStatusExecuting
	}
	snapshot.Progress = 1
	started := time.Now().UTC()
	snapshot.StartedAt = &started
	return env.Manager.SyncSnapshot(ctx, &snapshot)
}
func (d *fixedSnapshotDriver) Cancel(_ context.Context, _ *Run) error { return nil }
func (d *fixedSnapshotDriver) Sync(_ context.Context, run *Run) (*Run, error) {
	if d.syncResult == nil {
		if run == nil {
			return nil, nil
		}
		snapshot := *run
		return &snapshot, nil
	}
	snapshot := *d.syncResult
	snapshot.Metadata = cloneMetadataMap(d.syncResult.Metadata)
	return &snapshot, nil
}

func newTestController(t *testing.T) *Controller {
	t.Helper()
	tmpDir := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tmpDir, "harness-test.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store, err := NewSQLiteStore(db)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	harnessCfg := *config.DefaultHarnessConfig()
	harnessCfg.StorePath = filepath.Join(tmpDir, "blue.db")
	harnessCfg.ArtifactRoot = filepath.Join(tmpDir, "artifacts")
	return NewController(store, NewPolicyResolver(harnessCfg, nil))
}

func newTestControllerWithReadDB(t *testing.T, readDB *sql.DB) *Controller {
	t.Helper()
	tmpDir := t.TempDir()
	writeDB, err := sql.Open("sqlite3", filepath.Join(tmpDir, "harness-test.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = writeDB.Close() })
	store, err := NewSQLiteStoreWithReadDB(writeDB, readDB)
	if err != nil {
		t.Fatalf("NewSQLiteStoreWithReadDB failed: %v", err)
	}
	harnessCfg := *config.DefaultHarnessConfig()
	harnessCfg.StorePath = filepath.Join(tmpDir, "blue.db")
	harnessCfg.ArtifactRoot = filepath.Join(tmpDir, "artifacts")
	return NewController(store, NewPolicyResolver(harnessCfg, nil))
}

func TestController_SubmitAndList(t *testing.T) {
	controller := newTestController(t)
	driver := &stubDriver{kind: RunKindAgentTask}
	controller.RegisterDriver(driver)
	run, err := controller.Submit(context.Background(), RunSpec{
		Kind:   RunKindAgentTask,
		Goal:   "ship harness",
		UserID: "user-1",
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}
	if run.Kind != RunKindAgentTask || run.RootRunID != run.ID {
		t.Fatalf("unexpected run: %#v", run)
	}
	if len(driver.started) != 1 || driver.started[0] != run.ID {
		t.Fatalf("driver Start was not called correctly: %#v", driver.started)
	}
	runs, err := controller.List(context.Background(), RunFilter{UserID: "user-1", Kind: RunKindAgentTask, Limit: 10})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(runs) != 1 || runs[0].ID != run.ID {
		t.Fatalf("List returned %#v", runs)
	}
}

func TestController_SubmitDefaultsHarnessRunsToSilentMode(t *testing.T) {
	controller := newTestController(t)
	driver := &stubDriver{kind: RunKindAgentTask}
	controller.RegisterDriver(driver)

	run, err := controller.Submit(context.Background(), RunSpec{
		Kind:   RunKindAgentTask,
		Goal:   "run silently",
		UserID: "user-1",
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}
	if run.ApprovalMode != ApprovalModeAllow {
		t.Fatalf("ApprovalMode = %q, want %q", run.ApprovalMode, ApprovalModeAllow)
	}
	for _, key := range []string{"harness_silent_mode", "non_interactive", "skip_hil"} {
		value, ok := run.Metadata[key].(bool)
		if !ok || !value {
			t.Fatalf("metadata[%q] = %#v, want true", key, run.Metadata[key])
		}
	}
}

func TestControllerSyncRun_DoesNotRegressNewerTerminalState(t *testing.T) {
	controller := newTestController(t)
	driver := &fixedSnapshotDriver{kind: RunKindResearch, startState: RunStatusExecuting}
	controller.RegisterDriver(driver)

	run, err := controller.Submit(context.Background(), RunSpec{
		Kind:   RunKindResearch,
		Goal:   "Should we replace Python with Go?",
		UserID: "user-1",
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	stored, err := controller.GetStored(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("GetStored failed: %v", err)
	}
	if stored.Status != RunStatusExecuting {
		t.Fatalf("initial stored status = %q, want %q", stored.Status, RunStatusExecuting)
	}

	completed := *stored
	completed.Status = RunStatusCompleted
	completed.Progress = 100
	completed.Result = `{"recommendation":"Prefer Go for hot paths"}`
	finished := time.Now().UTC()
	completed.FinishedAt = &finished
	if err := controller.SyncSnapshot(context.Background(), &completed); err != nil {
		t.Fatalf("SyncSnapshot(completed) failed: %v", err)
	}

	stale := *stored
	stale.StartedAt = nil
	driver.syncResult = &stale

	updated, err := controller.syncRun(context.Background(), stored)
	if err != nil {
		t.Fatalf("syncRun failed: %v", err)
	}
	if updated.Status != RunStatusCompleted {
		t.Fatalf("syncRun returned status = %q, want %q", updated.Status, RunStatusCompleted)
	}

	current, err := controller.GetStored(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("GetStored(after syncRun) failed: %v", err)
	}
	if current.Status != RunStatusCompleted {
		t.Fatalf("stored status after syncRun = %q, want %q", current.Status, RunStatusCompleted)
	}
	if current.Result != completed.Result {
		t.Fatalf("stored result after syncRun = %q, want %q", current.Result, completed.Result)
	}
}

func TestController_SubmitPreservesExplicitApprovalMode(t *testing.T) {
	controller := newTestController(t)
	driver := &stubDriver{kind: RunKindAgentTask}
	controller.RegisterDriver(driver)

	run, err := controller.Submit(context.Background(), RunSpec{
		Kind:         RunKindAgentTask,
		Goal:         "respect explicit approval",
		UserID:       "user-1",
		ApprovalMode: ApprovalModeAsk,
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}
	if run.ApprovalMode != ApprovalModeAsk {
		t.Fatalf("ApprovalMode = %q, want %q", run.ApprovalMode, ApprovalModeAsk)
	}
}

func TestController_SubmitFallsBackWhenReaderMissesFreshRun(t *testing.T) {
	tmpDir := t.TempDir()
	staleReader, err := sql.Open("sqlite3", filepath.Join(tmpDir, "stale-reader.db"))
	if err != nil {
		t.Fatalf("open stale reader sqlite: %v", err)
	}
	t.Cleanup(func() { _ = staleReader.Close() })
	if _, err := NewSQLiteStoreWithReadDB(staleReader, staleReader); err != nil {
		t.Fatalf("initialize stale reader schema: %v", err)
	}

	controller := newTestControllerWithReadDB(t, staleReader)
	driver := &stubDriver{kind: RunKindAgentTask}
	controller.RegisterDriver(driver)

	run, err := controller.Submit(context.Background(), RunSpec{
		Kind:   RunKindAgentTask,
		Goal:   "reader fallback",
		UserID: "user-1",
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}
	if run == nil || strings.TrimSpace(run.ID) == "" {
		t.Fatalf("Submit returned %#v", run)
	}
	if len(driver.started) != 1 || driver.started[0] != run.ID {
		t.Fatalf("driver Start was not called correctly: %#v", driver.started)
	}
	stored, err := controller.GetStored(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("GetStored failed: %v", err)
	}
	if stored.ID != run.ID {
		t.Fatalf("stored run = %#v, want id %q", stored, run.ID)
	}
}

func TestController_SpawnChildFallsBackWhenReaderMissesFreshRun(t *testing.T) {
	tmpDir := t.TempDir()
	staleReader, err := sql.Open("sqlite3", filepath.Join(tmpDir, "stale-reader.db"))
	if err != nil {
		t.Fatalf("open stale reader sqlite: %v", err)
	}
	t.Cleanup(func() { _ = staleReader.Close() })
	if _, err := NewSQLiteStoreWithReadDB(staleReader, staleReader); err != nil {
		t.Fatalf("initialize stale reader schema: %v", err)
	}

	controller := newTestControllerWithReadDB(t, staleReader)
	driver := &stubDriver{kind: RunKindAgentTask}
	controller.RegisterDriver(driver)

	parent, err := controller.Submit(context.Background(), RunSpec{
		Kind:         RunKindAgentTask,
		Goal:         "parent",
		UserID:       "user-1",
		MaxSubagents: 2,
	})
	if err != nil {
		t.Fatalf("Submit(parent) failed: %v", err)
	}

	child, err := controller.SpawnChild(context.Background(), parent.ID, RunSpec{
		Kind:   RunKindAgentTask,
		Goal:   "child",
		UserID: "user-1",
	})
	if err != nil {
		t.Fatalf("SpawnChild failed: %v", err)
	}
	if child == nil || strings.TrimSpace(child.ID) == "" {
		t.Fatalf("SpawnChild returned %#v", child)
	}
	if child.ParentRunID != parent.ID {
		t.Fatalf("child parent_run_id = %q, want %q", child.ParentRunID, parent.ID)
	}
	stored, err := controller.GetStored(context.Background(), child.ID)
	if err != nil {
		t.Fatalf("GetStored(child) failed: %v", err)
	}
	if stored.ID != child.ID {
		t.Fatalf("stored child = %#v, want id %q", stored, child.ID)
	}
}

func TestController_PerformActionCancelUsesCancelControlPath(t *testing.T) {
	controller := newTestController(t)
	driver := &stubDriver{kind: RunKindAgentTask}
	controller.RegisterDriver(driver)

	run, err := controller.Submit(context.Background(), RunSpec{
		Kind:   RunKindAgentTask,
		Goal:   "cancel via action",
		UserID: "user-1",
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}
	run.Status = RunStatusExecuting
	if err := controller.store.UpdateRun(context.Background(), run); err != nil {
		t.Fatalf("UpdateRun failed: %v", err)
	}

	updated, err := controller.PerformAction(context.Background(), run.ID, "cancel", map[string]interface{}{
		"reason": "cancelled by test",
	})
	if err != nil {
		t.Fatalf("PerformAction(cancel) failed: %v", err)
	}
	if len(driver.cancelled) != 1 || driver.cancelled[0] != run.ID {
		t.Fatalf("driver cancel calls = %#v, want [%q]", driver.cancelled, run.ID)
	}
	if len(driver.actions) != 0 {
		t.Fatalf("driver action calls = %#v, want none for cancel shim", driver.actions)
	}
	if updated == nil || updated.Status != RunStatusCancelled {
		t.Fatalf("updated run = %#v, want cancelled snapshot", updated)
	}
	if strings.TrimSpace(updated.Error) != "cancelled by test" {
		t.Fatalf("updated error = %q, want cancelled by test", updated.Error)
	}
	events, err := controller.ListEvents(context.Background(), run.ID, 20)
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}
	runActionSeen := false
	runCancelledSeen := false
	for _, event := range events {
		if event.Type == "run_action" && event.Message == "cancel" {
			runActionSeen = true
		}
		if event.Type == "run_cancelled" && event.Message == "cancelled by test" {
			runCancelledSeen = true
		}
	}
	if !runActionSeen || !runCancelledSeen {
		t.Fatalf("events = %#v, want run_action(cancel) and run_cancelled(cancelled by test)", events)
	}
}

func TestController_SubmitExposesRunContextToMiddlewareAndDriver(t *testing.T) {
	controller := newTestController(t)
	order := []string{}
	driver := &runContextDriver{kind: RunKindAgentTask, order: &order}
	controller.UseExecutionMiddleware(ExecutionMiddlewareHooks{
		BeforeStartFunc: func(ctx context.Context, runCtx *RunContext) error {
			order = append(order, "before")
			if runCtx == nil || runCtx.Run == nil {
				t.Fatal("expected run context in BeforeStart")
			}
			if got := GetRunContext(ctx); got != runCtx {
				t.Fatalf("context runCtx = %#v, want %#v", got, runCtx)
			}
			runCtx.Values["phase"] = "before"
			return nil
		},
		AfterStartFunc: func(_ context.Context, runCtx *RunContext) {
			order = append(order, "after")
			if runCtx == nil || runCtx.Values["phase"] != "before" {
				t.Fatalf("runCtx.Values = %#v, want phase=before", runCtx.Values)
			}
		},
	})
	controller.RegisterDriver(driver)

	run, err := controller.Submit(context.Background(), RunSpec{
		Kind:       RunKindAgentTask,
		Goal:       "ship harness middleware",
		UserID:     "user-1",
		SessionID:  "session-1",
		ProviderID: "openai-prod",
		Model:      "gpt-test",
		AgentID:    "main",
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}
	if want := []string{"before", "start", "after"}; len(order) != len(want) {
		t.Fatalf("order = %#v, want %#v", order, want)
	} else {
		for i := range want {
			if order[i] != want[i] {
				t.Fatalf("order = %#v, want %#v", order, want)
			}
		}
	}
	if driver.lastEnv.RunContext == nil {
		t.Fatal("expected RunEnv.RunContext")
	}
	if driver.lastRunCtx != driver.lastEnv.RunContext {
		t.Fatalf("ctx runCtx = %#v, env runCtx = %#v", driver.lastRunCtx, driver.lastEnv.RunContext)
	}
	if driver.lastEnv.RunContext.Run == nil || driver.lastEnv.RunContext.Run.ID != run.ID {
		t.Fatalf("run context run = %#v, want run %q", driver.lastEnv.RunContext.Run, run.ID)
	}
	if driver.lastEnv.RunContext.Controller != controller {
		t.Fatal("expected controller on run context")
	}
	if driver.lastEnv.RunContext.Values["phase"] != "before" {
		t.Fatalf("run context values = %#v, want phase=before", driver.lastEnv.RunContext.Values)
	}
	if driver.toolRunID != run.ID {
		t.Fatalf("tools run_id = %q, want %q", driver.toolRunID, run.ID)
	}
	if driver.toolUserID != "user-1" {
		t.Fatalf("tools user_id = %q, want %q", driver.toolUserID, "user-1")
	}
	if driver.toolSession != "session-1" {
		t.Fatalf("tools session_id = %q, want %q", driver.toolSession, "session-1")
	}
	if driver.toolProvider != "openai-prod" {
		t.Fatalf("tools provider_id = %q, want %q", driver.toolProvider, "openai-prod")
	}
	if driver.toolModel != "gpt-test" {
		t.Fatalf("tools model = %q, want %q", driver.toolModel, "gpt-test")
	}
	if driver.toolAgentID != "main" {
		t.Fatalf("tools agent_id = %q, want %q", driver.toolAgentID, "main")
	}
	if driver.toolArtifactRoot != run.ArtifactRoot {
		t.Fatalf("tools artifact_root = %q, want %q", driver.toolArtifactRoot, run.ArtifactRoot)
	}
}

func TestController_RunContextAnnotatesToolArtifactEmitter(t *testing.T) {
	controller := newTestController(t)
	artifact := tools.ToolArtifact{
		Kind:      "file",
		Label:     "stage_trace",
		PathOrURL: "/tmp/stage-trace.json",
		MimeType:  "application/json",
		SizeBytes: 42,
		Metadata: map[string]interface{}{
			"stage": "locate_conversation",
		},
	}
	driver := &runContextDriver{
		kind:           RunKindAgentTask,
		artifactToEmit: &artifact,
	}
	controller.RegisterDriver(driver)

	run, err := controller.Submit(context.Background(), RunSpec{
		Kind:      RunKindAgentTask,
		Goal:      "emit tool artifact",
		UserID:    "user-1",
		SessionID: "session-1",
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}
	if driver.emitArtifactErr != nil {
		t.Fatalf("EmitArtifact returned error: %v", driver.emitArtifactErr)
	}
	artifacts, err := controller.ListArtifacts(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("ListArtifacts failed: %v", err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("artifacts = %#v, want one attached artifact", artifacts)
	}
	if artifacts[0].Label != "stage_trace" {
		t.Fatalf("artifact label = %q, want stage_trace", artifacts[0].Label)
	}
	if artifacts[0].PathOrURL != artifact.PathOrURL {
		t.Fatalf("artifact path = %q, want %q", artifacts[0].PathOrURL, artifact.PathOrURL)
	}
	if !strings.Contains(artifacts[0].MetadataJSON, "locate_conversation") {
		t.Fatalf("artifact metadata = %q, want stage metadata", artifacts[0].MetadataJSON)
	}
}

func TestController_RunContextAnnotatesToolEventEmitter(t *testing.T) {
	controller := newTestController(t)
	event := tools.ToolEvent{
		Type:     "stage_changed",
		ToolName: "computer_use",
		Message:  "chat stage updated",
		Payload: map[string]interface{}{
			"stage":    "computer_use_chat.locate_conversation",
			"status":   "ok",
			"strategy": "visual_sidebar_hit",
		},
	}
	driver := &runContextDriver{
		kind:        RunKindAgentTask,
		eventToEmit: &event,
	}
	controller.RegisterDriver(driver)

	run, err := controller.Submit(context.Background(), RunSpec{
		Kind:      RunKindAgentTask,
		Goal:      "emit tool event",
		UserID:    "user-1",
		SessionID: "session-1",
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}
	if driver.emitEventErr != nil {
		t.Fatalf("EmitEvent returned error: %v", driver.emitEventErr)
	}
	events, err := controller.ListEvents(context.Background(), run.ID, 20)
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}
	found := false
	for _, got := range events {
		if got.Type != "stage_changed" || got.ToolName != "computer_use" {
			continue
		}
		if !strings.Contains(got.PayloadJSON, "computer_use_chat.locate_conversation") {
			continue
		}
		found = true
		break
	}
	if !found {
		t.Fatalf("events = %#v, want emitted computer_use stage_changed event", events)
	}
}

func TestController_SubmitFailsWhenExecutionMiddlewareBlocksStart(t *testing.T) {
	controller := newTestController(t)
	driver := &stubDriver{kind: RunKindAgentTask}
	onStartErrorCalls := 0
	controller.UseExecutionMiddleware(ExecutionMiddlewareHooks{
		BeforeStartFunc: func(_ context.Context, _ *RunContext) error {
			return fmt.Errorf("blocked by middleware")
		},
		OnStartErrorFunc: func(_ context.Context, _ *RunContext, runErr error) {
			onStartErrorCalls++
			if runErr == nil || !strings.Contains(runErr.Error(), "blocked by middleware") {
				t.Fatalf("unexpected runErr = %v", runErr)
			}
		},
	})
	controller.RegisterDriver(driver)

	run, err := controller.Submit(context.Background(), RunSpec{
		Kind:   RunKindAgentTask,
		Goal:   "blocked dispatch",
		UserID: "user-1",
	})
	if err == nil {
		t.Fatal("expected Submit to fail")
	}
	if run != nil {
		t.Fatalf("expected nil run on Submit failure, got %#v", run)
	}
	if len(driver.started) != 0 {
		t.Fatalf("driver should not have started, got %#v", driver.started)
	}
	if onStartErrorCalls != 1 {
		t.Fatalf("OnStartError calls = %d, want 1", onStartErrorCalls)
	}

	runs, listErr := controller.List(context.Background(), RunFilter{UserID: "user-1", Kind: RunKindAgentTask, Limit: 10})
	if listErr != nil {
		t.Fatalf("List failed: %v", listErr)
	}
	if len(runs) != 1 {
		t.Fatalf("runs = %#v, want one failed run", runs)
	}
	if runs[0].Status != RunStatusFailed {
		t.Fatalf("run status = %q, want %q", runs[0].Status, RunStatusFailed)
	}
	if !strings.Contains(runs[0].Error, "blocked by middleware") {
		t.Fatalf("run error = %q, want middleware error", runs[0].Error)
	}
}

func TestController_SpawnChildExposesParentRunContext(t *testing.T) {
	controller := newTestController(t)
	parentDriver := &stubDriver{kind: RunKindAgentTask}
	childDriver := &runContextDriver{kind: RunKindSubagent}
	controller.RegisterDriver(parentDriver)
	controller.RegisterDriver(childDriver)

	parent, err := controller.Submit(context.Background(), RunSpec{
		Kind:         RunKindAgentTask,
		Goal:         "root",
		UserID:       "user-1",
		ApprovalMode: ApprovalModeAsk,
		MaxDepth:     2,
		MaxSubagents: 1,
	})
	if err != nil {
		t.Fatalf("Submit parent failed: %v", err)
	}

	controller.UseExecutionMiddleware(ExecutionMiddlewareHooks{
		BeforeStartFunc: func(_ context.Context, runCtx *RunContext) error {
			if runCtx == nil || runCtx.Run == nil {
				t.Fatal("expected child run context")
			}
			if runCtx.Run.Kind == RunKindSubagent {
				if runCtx.Parent == nil || runCtx.Parent.ID != parent.ID {
					t.Fatalf("parent = %#v, want %q", runCtx.Parent, parent.ID)
				}
			}
			return nil
		},
	})

	child, err := controller.SpawnChild(context.Background(), parent.ID, RunSpec{
		Kind:         RunKindSubagent,
		Goal:         "child",
		ApprovalMode: ApprovalModeAsk,
	})
	if err != nil {
		t.Fatalf("SpawnChild failed: %v", err)
	}
	if childDriver.lastEnv.RunContext == nil {
		t.Fatal("expected child RunContext")
	}
	if childDriver.lastEnv.RunContext.Parent == nil || childDriver.lastEnv.RunContext.Parent.ID != parent.ID {
		t.Fatalf("child parent = %#v, want %q", childDriver.lastEnv.RunContext.Parent, parent.ID)
	}
	if child.ParentRunID != parent.ID {
		t.Fatalf("child.ParentRunID = %q, want %q", child.ParentRunID, parent.ID)
	}
}

func TestController_SpawnChildEnforcesPolicy(t *testing.T) {
	controller := newTestController(t)
	parentDriver := &stubDriver{kind: RunKindAgentTask}
	childDriver := &stubDriver{kind: RunKindSubagent}
	controller.RegisterDriver(parentDriver)
	controller.RegisterDriver(childDriver)
	parent, err := controller.Submit(context.Background(), RunSpec{
		Kind:         RunKindAgentTask,
		Goal:         "root",
		UserID:       "user-1",
		ApprovalMode: ApprovalModeAsk,
		MaxDepth:     1,
		MaxSubagents: 1,
	})
	if err != nil {
		t.Fatalf("Submit parent failed: %v", err)
	}
	child, err := controller.SpawnChild(context.Background(), parent.ID, RunSpec{
		Kind:         RunKindSubagent,
		Goal:         "child",
		ApprovalMode: ApprovalModeAllow,
	})
	if err == nil {
		t.Fatalf("expected widening approval mode to fail, got child %#v", child)
	}

	okChild, err := controller.SpawnChild(context.Background(), parent.ID, RunSpec{
		Kind:         RunKindSubagent,
		Goal:         "child",
		ApprovalMode: ApprovalModeAsk,
	})
	if err != nil {
		t.Fatalf("SpawnChild failed: %v", err)
	}
	if okChild.Depth != 1 || okChild.ParentRunID != parent.ID || okChild.RootRunID != parent.RootRunID {
		t.Fatalf("unexpected child: %#v", okChild)
	}
	if _, err := controller.SpawnChild(context.Background(), parent.ID, RunSpec{
		Kind:         RunKindSubagent,
		Goal:         "extra child",
		ApprovalMode: ApprovalModeAsk,
	}); err == nil {
		t.Fatal("expected max subagents enforcement")
	}
}

func TestController_SubmitRejectsNegativeBudgetWithGuardError(t *testing.T) {
	controller := newTestController(t)
	driver := &stubDriver{kind: RunKindAgentTask}
	controller.RegisterDriver(driver)

	_, err := controller.Submit(context.Background(), RunSpec{
		Kind:     RunKindAgentTask,
		Goal:     "bad budget",
		UserID:   "user-1",
		MaxSteps: -1,
	})
	if err == nil {
		t.Fatal("expected negative budget to fail")
	}
	var guardErr *GuardPipelineError
	if !errors.As(err, &guardErr) {
		t.Fatalf("expected GuardPipelineError, got %T: %v", err, err)
	}
	if guardErr.Stage != RuntimeStageNormalize || guardErr.Code != "invalid_budget" {
		t.Fatalf("unexpected guard error: %#v", guardErr)
	}
}

func TestController_SpawnChildRejectsNegativeBudgetWithGuardError(t *testing.T) {
	controller := newTestController(t)
	parentDriver := &stubDriver{kind: RunKindAgentTask}
	childDriver := &stubDriver{kind: RunKindSubagent}
	controller.RegisterDriver(parentDriver)
	controller.RegisterDriver(childDriver)

	parent, err := controller.Submit(context.Background(), RunSpec{
		Kind:         RunKindAgentTask,
		Goal:         "root",
		UserID:       "user-1",
		ApprovalMode: ApprovalModeAsk,
		MaxDepth:     2,
		MaxSubagents: 2,
	})
	if err != nil {
		t.Fatalf("Submit parent failed: %v", err)
	}

	_, err = controller.SpawnChild(context.Background(), parent.ID, RunSpec{
		Kind:          RunKindSubagent,
		Goal:          "bad child budget",
		ApprovalMode:  ApprovalModeAsk,
		MaxToolRounds: -1,
	})
	if err == nil {
		t.Fatal("expected negative child budget to fail")
	}
	var guardErr *GuardPipelineError
	if !errors.As(err, &guardErr) {
		t.Fatalf("expected GuardPipelineError, got %T: %v", err, err)
	}
	if guardErr.Stage != RuntimeStageNormalize || guardErr.Code != "invalid_budget" {
		t.Fatalf("unexpected guard error: %#v", guardErr)
	}
}

func TestController_SpawnChildEmitsStageChangedEvents(t *testing.T) {
	controller := newTestController(t)
	parentDriver := &stubDriver{kind: RunKindAgentTask}
	childDriver := &stubDriver{kind: RunKindSubagent}
	controller.RegisterDriver(parentDriver)
	controller.RegisterDriver(childDriver)

	parent, err := controller.Submit(context.Background(), RunSpec{
		Kind:         RunKindAgentTask,
		Goal:         "root",
		UserID:       "user-1",
		ApprovalMode: ApprovalModeAsk,
		MaxDepth:     2,
		MaxSubagents: 2,
		MaxSteps:     9,
	})
	if err != nil {
		t.Fatalf("Submit parent failed: %v", err)
	}

	child, err := controller.SpawnChild(context.Background(), parent.ID, RunSpec{
		Kind:         RunKindSubagent,
		Goal:         "child",
		ApprovalMode: ApprovalModeAsk,
	})
	if err != nil {
		t.Fatalf("SpawnChild failed: %v", err)
	}

	events, err := controller.ListEvents(context.Background(), child.ID, 20)
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}
	var stages []string
	for _, event := range events {
		if event.Type != "stage_changed" {
			continue
		}
		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(event.PayloadJSON), &payload); err != nil {
			t.Fatalf("invalid stage payload: %v", err)
		}
		stage, _ := payload["stage"].(string)
		if stage != "" {
			stages = append(stages, stage)
		}
	}
	for _, want := range []string{
		string(RuntimeStageNormalize),
		string(RuntimeStagePolicy),
		string(RuntimeStageExecute),
	} {
		if !containsString(stages, want) {
			t.Fatalf("missing stage %q in events: %#v", want, stages)
		}
	}
}

func TestController_CancelCascadesToChildren(t *testing.T) {
	controller := newTestController(t)
	parentDriver := &stubDriver{kind: RunKindAgentTask}
	childDriver := &stubDriver{kind: RunKindSubagent}
	controller.RegisterDriver(parentDriver)
	controller.RegisterDriver(childDriver)

	parent, err := controller.Submit(context.Background(), RunSpec{
		Kind:         RunKindAgentTask,
		Goal:         "root",
		UserID:       "user-1",
		ApprovalMode: ApprovalModeAsk,
		MaxDepth:     2,
		MaxSubagents: 2,
	})
	if err != nil {
		t.Fatalf("Submit parent failed: %v", err)
	}
	child, err := controller.SpawnChild(context.Background(), parent.ID, RunSpec{
		Kind:         RunKindSubagent,
		Goal:         "child",
		ApprovalMode: ApprovalModeAsk,
	})
	if err != nil {
		t.Fatalf("SpawnChild failed: %v", err)
	}
	if err := controller.Cancel(context.Background(), parent.ID, "cancel root"); err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}
	if len(parentDriver.cancelled) != 1 || parentDriver.cancelled[0] != parent.ID {
		t.Fatalf("expected parent cancel, got %#v", parentDriver.cancelled)
	}
	if len(childDriver.cancelled) != 1 || childDriver.cancelled[0] != child.ID {
		t.Fatalf("expected child cancel, got %#v", childDriver.cancelled)
	}
	events, err := controller.ListEvents(context.Background(), parent.ID, 20)
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}
	foundCancel := false
	for _, event := range events {
		if event.Type == "run_cancelled" {
			foundCancel = true
			break
		}
	}
	if !foundCancel {
		t.Fatalf("expected run_cancelled event, got %#v", events)
	}
}

func TestController_SpawnChildInheritsApprovalAndSandbox(t *testing.T) {
	controller := newTestController(t)
	parentDriver := &stubDriver{kind: RunKindAgentTask}
	childDriver := &stubDriver{kind: RunKindSubagent}
	controller.RegisterDriver(parentDriver)
	controller.RegisterDriver(childDriver)

	parent, err := controller.Submit(context.Background(), RunSpec{
		Kind:         RunKindAgentTask,
		Goal:         "root",
		UserID:       "user-1",
		ProviderID:   "openai-prod",
		ApprovalMode: ApprovalModeDeny,
		SandboxMode:  "workspace",
		MaxDepth:     2,
		MaxSubagents: 2,
	})
	if err != nil {
		t.Fatalf("Submit parent failed: %v", err)
	}

	child, err := controller.SpawnChild(context.Background(), parent.ID, RunSpec{
		Kind: RunKindSubagent,
		Goal: "child",
	})
	if err != nil {
		t.Fatalf("SpawnChild failed: %v", err)
	}
	if child.ApprovalMode != ApprovalModeDeny {
		t.Fatalf("ApprovalMode = %q, want %q", child.ApprovalMode, ApprovalModeDeny)
	}
	if child.SandboxMode != "workspace" {
		t.Fatalf("SandboxMode = %q, want %q", child.SandboxMode, "workspace")
	}
	if child.ProviderID != "openai-prod" {
		t.Fatalf("ProviderID = %q, want %q", child.ProviderID, "openai-prod")
	}
}

func TestController_ListOneDoesNotTriggerSyncRecursion(t *testing.T) {
	controller := newTestController(t)
	driver := &recursiveListOneDriver{
		kind:       RunKindResearch,
		controller: controller,
	}
	controller.RegisterDriver(driver)

	run, err := controller.Submit(context.Background(), RunSpec{
		Kind:   RunKindResearch,
		Goal:   "inspect recursion",
		UserID: "user-1",
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}
	if driver.syncCalls != 1 {
		t.Fatalf("expected one sync during submit, got %d", driver.syncCalls)
	}
	if run.Status != RunStatusExecuting {
		t.Fatalf("expected synced status, got %s", run.Status)
	}

	got, err := controller.Get(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if driver.syncCalls != 2 {
		t.Fatalf("expected one sync per Get call, got %d total", driver.syncCalls)
	}
	if got.Status != RunStatusExecuting {
		t.Fatalf("expected executing status after sync, got %s", got.Status)
	}
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func TestController_CreatesArtifactRootForRuns(t *testing.T) {
	controller := newTestController(t)
	parentDriver := &stubDriver{kind: RunKindAgentTask}
	childDriver := &stubDriver{kind: RunKindSubagent}
	controller.RegisterDriver(parentDriver)
	controller.RegisterDriver(childDriver)

	parent, err := controller.Submit(context.Background(), RunSpec{
		Kind:         RunKindAgentTask,
		Goal:         "root",
		UserID:       "user-1",
		ApprovalMode: ApprovalModeAsk,
		MaxDepth:     2,
		MaxSubagents: 2,
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}
	if info, statErr := os.Stat(parent.ArtifactRoot); statErr != nil || !info.IsDir() {
		t.Fatalf("parent artifact root not created: %v", statErr)
	}

	child, err := controller.SpawnChild(context.Background(), parent.ID, RunSpec{
		Kind: RunKindSubagent,
		Goal: "child",
	})
	if err != nil {
		t.Fatalf("SpawnChild failed: %v", err)
	}
	if info, statErr := os.Stat(child.ArtifactRoot); statErr != nil || !info.IsDir() {
		t.Fatalf("child artifact root not created: %v", statErr)
	}
}
