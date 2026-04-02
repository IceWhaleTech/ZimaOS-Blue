package harness

import (
	"context"
	"errors"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

func TestRunTraceCollectorMiddlewareAppendsTraceEvents(t *testing.T) {
	controller := newTestController(t)
	tracer := NewRunTraceCollector(controller)
	controller.UseExecutionMiddleware(tracer.Middleware())
	driver := &stubDriver{kind: RunKindAgentTask}
	controller.RegisterDriver(driver)

	run, err := controller.Submit(context.Background(), RunSpec{
		Kind:   RunKindAgentTask,
		Goal:   "trace this run",
		UserID: "user-1",
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	events, err := controller.ListEvents(context.Background(), run.ID, 20)
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}
	if !hasRunEventType(events, "trace_requested") || !hasRunEventType(events, "trace_started") {
		t.Fatalf("expected trace lifecycle events, got %#v", events)
	}
}

func TestRunTraceCollectorCapturesStartError(t *testing.T) {
	controller := newTestController(t)
	tracer := NewRunTraceCollector(controller)
	controller.UseExecutionMiddleware(tracer.Middleware())
	driver := &runContextDriver{kind: RunKindAgentTask, startErr: errors.New("boom")}
	controller.RegisterDriver(driver)

	if _, err := controller.Submit(context.Background(), RunSpec{
		Kind: RunKindAgentTask,
		Goal: "trace failing run",
	}); err == nil {
		t.Fatal("expected submit failure")
	}

	runs, err := controller.List(context.Background(), RunFilter{Kind: RunKindAgentTask, Limit: 10})
	if err != nil || len(runs) != 1 {
		t.Fatalf("List failed: %v runs=%#v", err, runs)
	}
	events, err := controller.ListEvents(context.Background(), runs[0].ID, 20)
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}
	if !hasRunEventType(events, "trace_start_failed") {
		t.Fatalf("expected trace_start_failed event, got %#v", events)
	}
}

func TestRunTraceCollectorSnapshotIncludesStagesAndArtifacts(t *testing.T) {
	controller := newTestController(t)
	tracer := NewRunTraceCollector(controller)
	controller.UseExecutionMiddleware(tracer.Middleware())
	driver := &stubDriver{kind: RunKindAgentTask}
	controller.RegisterDriver(driver)

	run, err := controller.Submit(context.Background(), RunSpec{
		Kind:   RunKindAgentTask,
		Goal:   "trace snapshot run",
		UserID: "user-1",
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	snapshot := *run
	snapshot.Status = RunStatusCompleted
	snapshot.Result = "done"
	now := timeutil.NowTime()
	snapshot.StartedAt = &now
	snapshot.FinishedAt = &now
	if err := controller.SyncSnapshot(context.Background(), &snapshot); err != nil {
		t.Fatalf("SyncSnapshot failed: %v", err)
	}
	if err := controller.AttachArtifact(context.Background(), ArtifactRef{
		RunID:     run.ID,
		Kind:      "trace",
		Label:     "trace log",
		PathOrURL: "/tmp/trace.log",
	}); err != nil {
		t.Fatalf("AttachArtifact failed: %v", err)
	}

	trace, err := tracer.Snapshot(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	if trace.RunID != run.ID || trace.Status != RunStatusCompleted {
		t.Fatalf("unexpected trace header: %#v", trace)
	}
	if len(trace.Artifacts) != 1 || trace.Artifacts[0].Label != "trace log" {
		t.Fatalf("unexpected trace artifacts: %#v", trace.Artifacts)
	}
	if !hasTraceStage(trace.Stages, RuntimeStageFinalize) {
		t.Fatalf("expected finalize stage in trace, got %#v", trace.Stages)
	}
	if !hasTraceEvent(trace.Events, "artifact_attached") {
		t.Fatalf("expected artifact_attached event in trace, got %#v", trace.Events)
	}
}

func hasRunEventType(events []RunEvent, want string) bool {
	for _, event := range events {
		if event.Type == want {
			return true
		}
	}
	return false
}

func hasTraceStage(stages []RunTraceStage, want RuntimeStage) bool {
	for _, stage := range stages {
		if stage.Stage == want {
			return true
		}
	}
	return false
}

func hasTraceEvent(events []RunTraceEvent, want string) bool {
	for _, event := range events {
		if event.Type == want {
			return true
		}
	}
	return false
}
