package harness

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/google/uuid"
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
	approvals *testToolApprovalHandler
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

type testToolApprovalConfig struct {
	Enabled       bool
	DefaultPolicy string
	ToolPolicies  map[string]string
}

type testPendingToolApproval struct {
	ID           string
	RunID        string
	StepIndex    int
	ToolName     string
	ToolCallID   string
	SessionID    string
	UserID       string
	PolicySource string
	RiskLevel    string
	BindingHash  string
	ExpiresAt    int64
}

type testToolApprovalHandler struct {
	mu       sync.Mutex
	config   testToolApprovalConfig
	pending  map[string]*testPendingToolApproval
	waiters  map[string]chan string
	timeout  time.Duration
	observer tools.RuntimeEventObserver
}

func newTestToolApprovalHandler() *testToolApprovalHandler {
	return &testToolApprovalHandler{
		config: testToolApprovalConfig{
			Enabled:       true,
			DefaultPolicy: "auto",
			ToolPolicies:  map[string]string{},
		},
		pending: make(map[string]*testPendingToolApproval),
		waiters: make(map[string]chan string),
		timeout: 2 * time.Minute,
	}
}

func (h *testToolApprovalHandler) SetObserver(observer tools.RuntimeEventObserver) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.observer = observer
}

func (h *testToolApprovalHandler) SetConfig(cfg testToolApprovalConfig) {
	if cfg.ToolPolicies == nil {
		cfg.ToolPolicies = map[string]string{}
	}
	h.mu.Lock()
	h.config = cfg
	h.mu.Unlock()
}

func (h *testToolApprovalHandler) GetPendingByRun(runID string) *testPendingToolApproval {
	if runID == "" {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, pending := range h.pending {
		if pending != nil && pending.RunID == runID {
			out := *pending
			return &out
		}
	}
	return nil
}

func (h *testToolApprovalHandler) AuthorizeToolCall(ctx context.Context, req tools.ToolApprovalRequest) (tools.ToolApprovalDecision, error) {
	h.mu.Lock()
	cfg := h.config
	observer := h.observer
	h.mu.Unlock()

	mode := cfg.DefaultPolicy
	if policy, ok := cfg.ToolPolicies[req.ToolName]; ok && policy != "" {
		mode = policy
	}
	if !cfg.Enabled && mode == "" {
		mode = "auto"
	}
	if mode == "" {
		mode = "auto"
	}

	decision := tools.ToolApprovalDecision{
		Allowed: true,
		Approval: tools.ToolApprovalEnvelope{
			Mode:         mode,
			PolicySource: "harness_test",
			RiskLevel:    req.RiskLevel,
			BindingHash:  req.BindingHash,
		},
	}
	switch mode {
	case "deny":
		decision.Allowed = false
		decision.Approval.Required = true
		decision.Approval.Reason = fmt.Sprintf("tool %q blocked by approval policy", req.ToolName)
		return decision, nil
	case "ask":
		pending := &testPendingToolApproval{
			ID:           uuid.NewString(),
			RunID:        tools.GetRunID(ctx),
			StepIndex:    tools.GetRunStep(ctx),
			ToolName:     req.ToolName,
			ToolCallID:   req.ToolCallID,
			SessionID:    req.SessionID,
			UserID:       firstNonEmptyRuntimeApprovalTestValue(req.UserID, tools.GetUserID(ctx), "default"),
			PolicySource: "harness_test",
			RiskLevel:    req.RiskLevel,
			BindingHash:  req.BindingHash,
			ExpiresAt:    time.Now().Add(h.timeout).UnixMilli(),
		}
		ch := make(chan string, 1)
		h.mu.Lock()
		h.pending[pending.ID] = pending
		h.waiters[pending.ID] = ch
		h.mu.Unlock()
		defer func() {
			h.mu.Lock()
			delete(h.pending, pending.ID)
			delete(h.waiters, pending.ID)
			h.mu.Unlock()
		}()

		decision.Approval.Required = true
		decision.Approval.ID = pending.ID
		decision.Approval.ExpiresAt = pending.ExpiresAt

		if observer != nil {
			observer.OnApprovalRequested(tools.ApprovalRuntimeEvent{
				RunID:        pending.RunID,
				StepIndex:    pending.StepIndex,
				Kind:         "tool",
				ID:           pending.ID,
				ToolName:     pending.ToolName,
				ToolCallID:   pending.ToolCallID,
				SessionID:    pending.SessionID,
				UserID:       pending.UserID,
				PolicySource: pending.PolicySource,
				RiskLevel:    pending.RiskLevel,
				BindingHash:  pending.BindingHash,
				ExpiresAt:    pending.ExpiresAt,
			})
		}

		select {
		case resolution := <-ch:
			allowed := resolution == "approve" || resolution == "allow" || resolution == "allow-once" || resolution == "allow-always"
			decision.Allowed = allowed
			if !allowed {
				decision.Approval.Reason = "tool approval denied"
			}
			if observer != nil {
				observer.OnApprovalResolved(tools.ApprovalRuntimeEvent{
					RunID:        pending.RunID,
					StepIndex:    pending.StepIndex,
					Kind:         "tool",
					ID:           pending.ID,
					ToolName:     pending.ToolName,
					ToolCallID:   pending.ToolCallID,
					SessionID:    pending.SessionID,
					UserID:       pending.UserID,
					PolicySource: pending.PolicySource,
					RiskLevel:    pending.RiskLevel,
					BindingHash:  pending.BindingHash,
					ExpiresAt:    pending.ExpiresAt,
					Decision:     resolution,
				})
			}
			return decision, nil
		case <-ctx.Done():
			decision.Allowed = false
			decision.Approval.Reason = ctx.Err().Error()
			if observer != nil {
				observer.OnApprovalResolved(tools.ApprovalRuntimeEvent{
					RunID:        pending.RunID,
					StepIndex:    pending.StepIndex,
					Kind:         "tool",
					ID:           pending.ID,
					ToolName:     pending.ToolName,
					ToolCallID:   pending.ToolCallID,
					SessionID:    pending.SessionID,
					UserID:       pending.UserID,
					PolicySource: pending.PolicySource,
					RiskLevel:    pending.RiskLevel,
					BindingHash:  pending.BindingHash,
					ExpiresAt:    pending.ExpiresAt,
					Decision:     "deny",
					Error:        ctx.Err().Error(),
				})
			}
			return decision, ctx.Err()
		}
	default:
		return decision, nil
	}
}

func firstNonEmptyRuntimeApprovalTestValue(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
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
	approvals := newTestToolApprovalHandler()
	approvals.SetObserver(NewRuntimeObserver(controller))
	approvals.SetConfig(testToolApprovalConfig{
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
