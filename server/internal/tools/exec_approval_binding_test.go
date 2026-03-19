package tools

import (
	"context"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
)

func TestApprovalManagerBindingHashRequiredForResolveWithBinding(t *testing.T) {
	broker := sse.NewBroker()
	defer broker.Close()

	mgr := NewApprovalManager(broker)
	ctx := WithSessionID(context.Background(), "conv-1")

	done := make(chan ApprovalDecision, 1)
	go func() {
		decision, _ := mgr.RequestApproval(ctx, ApprovalRequest{
			Type:     "command",
			Command:  "echo hello",
			Workdir:  t.TempDir(),
			UserID:   "test-user",
			Security: "allowlist",
		})
		done <- decision
	}()

	deadline := time.Now().Add(2 * time.Second)
	var pending *ApprovalRequest
	for time.Now().Before(deadline) {
		if req := mgr.GetPendingBySession("conv-1"); req != nil {
			pending = req
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pending == nil {
		t.Fatal("expected pending approval")
	}
	if pending.BindingHash == "" {
		t.Fatal("expected binding hash to be populated")
	}
	if mgr.ResolveApprovalWithBinding(pending.ID, ApprovalAllowOnce, "wrong-hash") {
		t.Fatal("expected resolve with wrong binding hash to fail")
	}
	if !mgr.ResolveApprovalWithBinding(pending.ID, ApprovalAllowOnce, pending.BindingHash) {
		t.Fatal("expected resolve with correct binding hash to succeed")
	}
	select {
	case decision := <-done:
		if decision != ApprovalAllowOnce {
			t.Fatalf("decision = %s, want %s", decision, ApprovalAllowOnce)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for approval resolution")
	}
}

func TestApprovalManagerObserverReceivesLifecycleEvents(t *testing.T) {
	broker := sse.NewBroker()
	defer broker.Close()

	mgr := NewApprovalManager(broker)
	observer := &runtimeObserverStub{}
	mgr.SetObserver(observer)
	ctx := WithSessionID(context.Background(), "conv-2")
	ctx = WithRunID(ctx, "run-approval")
	ctx = WithRunStep(ctx, 6)

	done := make(chan ApprovalDecision, 1)
	go func() {
		decision, _ := mgr.RequestApproval(ctx, ApprovalRequest{
			Type:     "command",
			Command:  "echo observer",
			Workdir:  t.TempDir(),
			UserID:   "test-user",
			Security: "allowlist",
		})
		done <- decision
	}()

	deadline := time.Now().Add(2 * time.Second)
	var pending *ApprovalRequest
	for time.Now().Before(deadline) {
		if req := mgr.GetPendingBySession("conv-2"); req != nil {
			pending = req
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pending == nil {
		t.Fatal("expected pending approval")
	}
	if !mgr.ResolveApprovalWithBinding(pending.ID, ApprovalAllowOnce, pending.BindingHash) {
		t.Fatal("expected resolve with correct binding hash to succeed")
	}
	select {
	case decision := <-done:
		if decision != ApprovalAllowOnce {
			t.Fatalf("decision = %s, want %s", decision, ApprovalAllowOnce)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for approval resolution")
	}

	if len(observer.approvalRequested) != 1 {
		t.Fatalf("approvalRequested len = %d, want 1", len(observer.approvalRequested))
	}
	if len(observer.approvalResolved) != 1 {
		t.Fatalf("approvalResolved len = %d, want 1", len(observer.approvalResolved))
	}
	if observer.approvalRequested[0].RunID != "run-approval" || observer.approvalResolved[0].RunID != "run-approval" {
		t.Fatalf("unexpected run ids: req=%q res=%q", observer.approvalRequested[0].RunID, observer.approvalResolved[0].RunID)
	}
	if observer.approvalResolved[0].Decision != string(ApprovalAllowOnce) {
		t.Fatalf("decision = %q, want %q", observer.approvalResolved[0].Decision, string(ApprovalAllowOnce))
	}
}
