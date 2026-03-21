package harness

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
)

type stubDriver struct {
	kind      RunKind
	started   []string
	cancelled []string
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
