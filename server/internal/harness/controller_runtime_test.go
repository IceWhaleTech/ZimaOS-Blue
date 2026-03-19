package harness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	networkapi "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/api"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/labstack/echo/v4"
)

type blockingQuestionDriver struct {
	kind    RunKind
	mgr     *tools.QuestionManager
	cancel  context.CancelFunc
	done    chan error
	lastErr error
}

func (d *blockingQuestionDriver) Kind() RunKind { return d.kind }

func (d *blockingQuestionDriver) Validate(spec RunSpec) error {
	if spec.Goal == "" {
		return fmt.Errorf("goal is required")
	}
	return nil
}

func (d *blockingQuestionDriver) Start(_ context.Context, run *Run, _ RunEnv) error {
	runCtx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel
	d.done = make(chan error, 1)
	go func() {
		ctx := tools.WithRunID(runCtx, run.ID)
		ctx = tools.WithRunStep(ctx, 2)
		ctx = tools.WithSessionID(ctx, run.SessionID)
		ctx = tools.WithChannel(ctx, "web")
		_, _, err := d.mgr.AskQuestionsWithContext(ctx, run.UserID, run.SessionID, []tools.QuestionItem{{
			ID:       "q1",
			Question: "Need confirmation",
			Header:   "Confirm",
			Options: []tools.QuestionOption{
				{Label: "Continue", Value: "continue"},
				{Label: "Stop", Value: "stop"},
			},
		}}, map[string]interface{}{"source": "harness-test"})
		d.done <- err
	}()
	return nil
}

func (d *blockingQuestionDriver) Cancel(_ context.Context, _ *Run) error {
	if d.cancel != nil {
		d.cancel()
	}
	select {
	case err := <-d.done:
		d.lastErr = err
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	case <-time.After(2 * time.Second):
		return fmt.Errorf("timed out waiting for question driver shutdown")
	}
}

type blockingToolApprovalDriver struct {
	kind      RunKind
	approvals *networkapi.ApprovalHandler
	cancel    context.CancelFunc
	done      chan error
	lastErr   error
}

func (d *blockingToolApprovalDriver) Kind() RunKind { return d.kind }

func (d *blockingToolApprovalDriver) Validate(spec RunSpec) error {
	if spec.Goal == "" {
		return fmt.Errorf("goal is required")
	}
	return nil
}

func (d *blockingToolApprovalDriver) Start(_ context.Context, run *Run, _ RunEnv) error {
	runCtx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel
	d.done = make(chan error, 1)
	go func() {
		ctx := tools.WithRunID(runCtx, run.ID)
		ctx = tools.WithRunStep(ctx, 5)
		ctx = tools.WithUserID(ctx, run.UserID)
		ctx = tools.WithSessionID(ctx, run.SessionID)
		_, err := d.approvals.AuthorizeToolCall(ctx, tools.ToolApprovalRequest{
			ToolName:    "browser",
			RouteKind:   tools.ToolRouteKindAgent,
			SessionID:   run.SessionID,
			UserID:      run.UserID,
			BindingHash: "binding-test",
			RiskLevel:   "high",
		})
		d.done <- err
	}()
	return nil
}

func (d *blockingToolApprovalDriver) Cancel(_ context.Context, _ *Run) error {
	if d.cancel != nil {
		d.cancel()
	}
	select {
	case err := <-d.done:
		d.lastErr = err
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	case <-time.After(2 * time.Second):
		return fmt.Errorf("timed out waiting for approval driver shutdown")
	}
}

func TestController_SubmitDefaultsWorkspaceRootAndCreatesIt(t *testing.T) {
	controller := newTestController(t)
	driver := &stubDriver{kind: RunKindAgentTask}
	controller.RegisterDriver(driver)

	run, err := controller.Submit(context.Background(), RunSpec{
		Kind:   RunKindAgentTask,
		Goal:   "default workspace",
		UserID: "user-1",
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}
	want := filepath.Join(filepath.Dir(controller.resolver.defaults.StorePath), "workspace")
	if run.WorkspaceRoot != want {
		t.Fatalf("WorkspaceRoot = %q, want %q", run.WorkspaceRoot, want)
	}
	info, statErr := os.Stat(run.WorkspaceRoot)
	if statErr != nil {
		t.Fatalf("workspace root not created: %v", statErr)
	}
	if !info.IsDir() {
		t.Fatalf("workspace root is not a directory: %q", run.WorkspaceRoot)
	}
}

func TestController_CancelClearsPendingQuestionWhileBlocked(t *testing.T) {
	controller := newTestController(t)
	broker := sse.NewBroker()
	client := broker.Subscribe("user-1")
	defer broker.Unsubscribe("user-1", client)

	questions := tools.NewQuestionManager(broker, func() bool { return false }, 2*time.Minute)
	questions.SetObserver(NewRuntimeObserver(controller))

	driver := &blockingQuestionDriver{
		kind: RunKindAgentTask,
		mgr:  questions,
	}
	controller.RegisterDriver(driver)

	run, err := controller.Submit(context.Background(), RunSpec{
		Kind:          RunKindAgentTask,
		Goal:          "wait for question",
		UserID:        "user-1",
		SessionID:     "session-question",
		ApprovalMode:  ApprovalModeAsk,
		MaxSubagents:  1,
		MaxDepth:      1,
		MaxSteps:      4,
		MaxToolRounds: 4,
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	waitForCondition(t, "pending question", func() bool {
		return questions.GetPendingByRun(run.ID) != nil
	})

	if err := controller.Cancel(context.Background(), run.ID, "cancelled while waiting on question"); err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}
	if !errors.Is(driver.lastErr, context.Canceled) {
		t.Fatalf("question driver error = %v, want context canceled", driver.lastErr)
	}
	if pending := questions.GetPendingByRun(run.ID); pending != nil {
		t.Fatalf("expected no pending question after cancel, got %#v", pending)
	}

	stored, err := controller.GetStored(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("GetStored failed: %v", err)
	}
	if stored.Status != RunStatusCancelled {
		t.Fatalf("status = %q, want %q", stored.Status, RunStatusCancelled)
	}

	events, err := controller.ListEvents(context.Background(), run.ID, 20)
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}
	assertEventTypes(t, events, "question_requested", "question_resolved", "run_cancelled")
}

func TestController_CancelClearsPendingToolApprovalWhileBlocked(t *testing.T) {
	controller := newTestController(t)
	approvals := networkapi.NewApprovalHandler(nil)
	approvals.SetObserver(NewRuntimeObserver(controller))
	updateApprovalConfig(t, approvals, networkapi.ApprovalConfig{
		Enabled:       true,
		DefaultPolicy: "ask",
		ToolPolicies:  map[string]string{},
	})

	driver := &blockingToolApprovalDriver{
		kind:      RunKindAgentTask,
		approvals: approvals,
	}
	controller.RegisterDriver(driver)

	run, err := controller.Submit(context.Background(), RunSpec{
		Kind:          RunKindAgentTask,
		Goal:          "wait for tool approval",
		UserID:        "user-1",
		SessionID:     "session-approval",
		ApprovalMode:  ApprovalModeAsk,
		MaxSubagents:  1,
		MaxDepth:      1,
		MaxSteps:      4,
		MaxToolRounds: 4,
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	waitForCondition(t, "pending tool approval", func() bool {
		return approvals.GetPendingByRun(run.ID) != nil
	})

	if err := controller.Cancel(context.Background(), run.ID, "cancelled while waiting on approval"); err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}
	if !errors.Is(driver.lastErr, context.Canceled) {
		t.Fatalf("approval driver error = %v, want context canceled", driver.lastErr)
	}
	if pending := approvals.GetPendingByRun(run.ID); pending != nil {
		t.Fatalf("expected no pending approval after cancel, got %#v", pending)
	}

	stored, err := controller.GetStored(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("GetStored failed: %v", err)
	}
	if stored.Status != RunStatusCancelled {
		t.Fatalf("status = %q, want %q", stored.Status, RunStatusCancelled)
	}

	events, err := controller.ListEvents(context.Background(), run.ID, 20)
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}
	assertEventTypes(t, events, "approval_requested", "approval_resolved", "run_cancelled")
}

func waitForCondition(t *testing.T, label string, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", label)
}

func assertEventTypes(t *testing.T, events []RunEvent, want ...string) {
	t.Helper()
	seen := make(map[string]bool, len(events))
	for _, event := range events {
		seen[event.Type] = true
	}
	for _, eventType := range want {
		if !seen[eventType] {
			t.Fatalf("missing event type %q in %#v", eventType, events)
		}
	}
}

func updateApprovalConfig(t *testing.T, handler *networkapi.ApprovalHandler, cfg networkapi.ApprovalConfig) {
	t.Helper()
	e := echo.New()
	body, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal approval config: %v", err)
	}
	req := httptest.NewRequest(http.MethodPut, "/approval/config", strings.NewReader(string(body)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := handler.UpdateConfig(c); err != nil {
		t.Fatalf("UpdateConfig returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("UpdateConfig status = %d, want %d", rec.Code, http.StatusOK)
	}
}
