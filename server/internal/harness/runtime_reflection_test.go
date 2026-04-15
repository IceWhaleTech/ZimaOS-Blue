package harness

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	agentpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agent"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selfreflect"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type stubRuntimeReflectionReflector struct {
	inputs []selfreflect.Input
	result *selfreflect.Result
	err    error
}

func (s *stubRuntimeReflectionReflector) Reflect(_ context.Context, input selfreflect.Input) (*selfreflect.Result, error) {
	s.inputs = append(s.inputs, input)
	if s.err != nil {
		return nil, s.err
	}
	out := *s.result
	out.Lessons = append([]selfreflect.Lesson(nil), s.result.Lessons...)
	out.MutationSuggestions = append([]selfreflect.MutationSuggestion(nil), s.result.MutationSuggestions...)
	return &out, nil
}

func TestRuntimeReflectionCoordinator_TriggersIntervalReviewOnce(t *testing.T) {
	controller, run := newRuntimeReflectionControllerAndRun(t)
	reflector := &stubRuntimeReflectionReflector{result: runtimeReflectionTestResult("browser")}
	controller.SetReflector(reflector)
	observer := NewRuntimeObserver(controller)
	coordinator := NewRuntimeReflectionCoordinator(controller, config.RuntimeReflectionConfig{
		Enabled:              true,
		IntervalToolFinishes: 10,
		MinReviewGap:         0,
		RepeatWindow:         8,
		MaxEvidenceEvents:    8,
	})
	coordinator.launch = func(fn func()) { fn() }

	for i := 0; i < 10; i++ {
		event := runtimeReflectionToolFinishedEvent(run.ID, i, "", "browser")
		observer.OnToolFinished(event)
		coordinator.OnToolFinished(event)
	}

	assertRuntimeReflectionCount(t, controller, run.ID, "runtime_reflection_recorded", 1)
	artifacts, err := controller.ListArtifacts(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("ListArtifacts failed: %v", err)
	}
	if len(artifacts) != 1 || artifacts[0].Kind != "runtime_reflection" {
		t.Fatalf("artifacts = %#v, want one runtime_reflection artifact", artifacts)
	}
	payload := decodeRuntimeReflectionPayload(t, artifacts[0].MetadataJSON)
	if got := payload["trigger_kind"]; got != "interval_tool_finishes" {
		t.Fatalf("trigger_kind = %#v, want interval_tool_finishes", got)
	}
	if got := intValueForRuntimeReflectionPayload(payload["tool_count_window"]); got != 10 {
		t.Fatalf("tool_count_window = %d, want 10", got)
	}
	if len(reflector.inputs) != 1 {
		t.Fatalf("reflector inputs len = %d, want 1", len(reflector.inputs))
	}
	if reflector.inputs[0].DisableMemoryWrite != true {
		t.Fatal("expected non-terminal review to disable memory writes")
	}
}

func TestRuntimeReflectionCoordinator_TriggersRepeatedFailureAndQuestionGate(t *testing.T) {
	controller, run := newRuntimeReflectionControllerAndRun(t)
	reflector := &stubRuntimeReflectionReflector{result: runtimeReflectionTestResult("browser")}
	controller.SetReflector(reflector)
	observer := NewRuntimeObserver(controller)
	coordinator := NewRuntimeReflectionCoordinator(controller, config.RuntimeReflectionConfig{
		Enabled:              true,
		IntervalToolFinishes: 10,
		MinReviewGap:         0,
		RepeatWindow:         8,
		MaxEvidenceEvents:    8,
	})
	coordinator.launch = func(fn func()) { fn() }

	failOne := runtimeReflectionToolFinishedEvent(run.ID, 0, "missing_title", "browser")
	failTwo := runtimeReflectionToolFinishedEvent(run.ID, 1, "missing_title", "browser")
	observer.OnToolFinished(failOne)
	coordinator.OnToolFinished(failOne)
	observer.OnToolFinished(failTwo)
	coordinator.OnToolFinished(failTwo)

	assertRuntimeReflectionCount(t, controller, run.ID, "runtime_reflection_recorded", 1)

	for i := 2; i < 6; i++ {
		event := runtimeReflectionToolFinishedEvent(run.ID, i, "", "browser")
		observer.OnToolFinished(event)
		coordinator.OnToolFinished(event)
	}
	approval := tools.ApprovalRuntimeEvent{RunID: run.ID, StepIndex: 2, Kind: "tool", ToolName: "browser", ToolCallID: "approve-1"}
	coordinator.OnApprovalRequested(approval)
	assertRuntimeReflectionCount(t, controller, run.ID, "runtime_reflection_recorded", 1)

	fifth := runtimeReflectionToolFinishedEvent(run.ID, 6, "", "browser")
	observer.OnToolFinished(fifth)
	coordinator.OnToolFinished(fifth)

	question := tools.QuestionRuntimeEvent{RunID: run.ID, StepIndex: 2, ID: "q-1"}
	coordinator.OnQuestionRequested(question)
	assertRuntimeReflectionCount(t, controller, run.ID, "runtime_reflection_recorded", 2)
}

func TestRuntimeReflectionCoordinator_TerminalFlushRequiresUnreviewedToolActivity(t *testing.T) {
	controller, run := newRuntimeReflectionControllerAndRun(t)
	reflector := &stubRuntimeReflectionReflector{result: runtimeReflectionTestResult("browser")}
	controller.SetReflector(reflector)
	observer := NewRuntimeObserver(controller)
	coordinator := NewRuntimeReflectionCoordinator(controller, config.RuntimeReflectionConfig{
		Enabled:              true,
		IntervalToolFinishes: 10,
		MinReviewGap:         0,
		RepeatWindow:         8,
		MaxEvidenceEvents:    8,
	})
	coordinator.launch = func(fn func()) { fn() }
	controller.SetRuntimeReflectionCoordinator(coordinator)

	for i := 0; i < 3; i++ {
		event := runtimeReflectionToolFinishedEvent(run.ID, i, "", "browser")
		observer.OnToolFinished(event)
		coordinator.OnToolFinished(event)
	}

	snapshot := *run
	snapshot.Status = RunStatusCompleted
	snapshot.RuntimeState = agentpkg.RuntimeStateDone
	snapshot.Result = "Runtime task completed."
	if err := controller.SyncSnapshot(context.Background(), &snapshot); err != nil {
		t.Fatalf("SyncSnapshot failed: %v", err)
	}

	assertRuntimeReflectionCount(t, controller, run.ID, "runtime_reflection_recorded", 1)
	events, err := controller.ListEvents(context.Background(), run.ID, 200)
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}
	var found bool
	for _, event := range events {
		if event.Type != "runtime_reflection_recorded" {
			continue
		}
		payload := decodeRuntimeReflectionPayload(t, event.PayloadJSON)
		if payload["trigger_kind"] == "terminal_flush" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected terminal_flush runtime reflection record")
	}
	if len(reflector.inputs) != 1 || reflector.inputs[0].DisableMemoryWrite {
		t.Fatalf("terminal flush should allow memory writes, inputs=%#v", reflector.inputs)
	}
}

func newRuntimeReflectionControllerAndRun(t *testing.T) (*Controller, *Run) {
	t.Helper()
	controller := newTestController(t)
	driver := &stubDriver{kind: RunKindAgentTask}
	controller.RegisterDriver(driver)
	run, err := controller.Submit(context.Background(), RunSpec{
		Kind:   RunKindAgentTask,
		Goal:   "Investigate browser regression",
		UserID: "user-1",
		Metadata: map[string]interface{}{
			"selected_canonical_skill": "browser",
		},
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}
	snapshot := *run
	snapshot.Status = RunStatusExecuting
	snapshot.RuntimeState = agentpkg.RuntimeStateExecute
	started := time.Now().UTC()
	snapshot.StartedAt = &started
	if err := controller.SyncSnapshot(context.Background(), &snapshot); err != nil {
		t.Fatalf("SyncSnapshot(executing) failed: %v", err)
	}
	stored, err := controller.GetStored(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("GetStored failed: %v", err)
	}
	return controller, stored
}

func runtimeReflectionToolFinishedEvent(runID string, idx int, errText string, toolName string) tools.ToolRuntimeEvent {
	return tools.ToolRuntimeEvent{
		RunID:          runID,
		StepIndex:      2,
		ToolCallID:     toolName + "-call-" + string(rune('a'+idx)),
		ToolName:       toolName,
		CapabilityKind: toolName,
		Result:         map[string]interface{}{"seq": idx},
		Error:          errText,
	}
}

func runtimeReflectionTestResult(skillID string) *selfreflect.Result {
	return &selfreflect.Result{
		Summary: "Runtime reflection complete.",
		Lessons: []selfreflect.Lesson{{
			Kind:        selfreflect.LessonKindGuardrail,
			Lesson:      "Validate browser title extraction before summarizing.",
			WhenToApply: "When browser navigation returns partial content.",
			Evidence:    "Repeated runtime events showed missing title extraction before summary generation.",
		}},
		MutationSuggestions: []selfreflect.MutationSuggestion{{
			Kind:          "guardrail_add",
			TargetSkillID: skillID,
			Rationale:     "Prevent summarization without title extraction.",
			SuggestedText: "Abort summary generation when browser title extraction is missing.",
			EvidenceIDs:   []string{"ev-1", "ev-2"},
		}},
		SignalStrength: "high",
	}
}

func assertRuntimeReflectionCount(t *testing.T, controller *Controller, runID string, eventType string, want int) {
	t.Helper()
	events, err := controller.ListEvents(context.Background(), runID, 200)
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}
	got := 0
	for _, event := range events {
		if event.Type == eventType {
			got++
		}
	}
	if got != want {
		t.Fatalf("%s count = %d, want %d", eventType, got, want)
	}
}

func decodeRuntimeReflectionPayload(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	return payload
}

func intValueForRuntimeReflectionPayload(raw interface{}) int {
	switch typed := raw.(type) {
	case int:
		return typed
	case float64:
		return int(typed)
	default:
		return 0
	}
}
