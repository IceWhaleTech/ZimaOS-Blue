package bootstrap

import (
	"context"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestExecApprovalAdapter_MapsResolveStatuses(t *testing.T) {
	broker := sse.NewBroker()
	defer broker.Close()

	mgr := tools.NewApprovalManager(broker)
	adapter := execApprovalAdapter{mgr: mgr}

	if result := adapter.ResolveApproval("missing", string(tools.ApprovalAllowOnce), ""); result.Resolved || result.BindingMismatch {
		t.Fatalf("unexpected missing approval result: %#v", result)
	}

	done := make(chan tools.ApprovalDecision, 1)
	ctx := tools.WithSessionID(context.Background(), "approval-lane-test")
	go func() {
		decision, _ := mgr.RequestApproval(ctx, tools.ApprovalRequest{
			Type:     "command",
			Command:  "echo approval lane",
			Workdir:  t.TempDir(),
			UserID:   "test-user",
			Security: "allowlist",
		})
		done <- decision
	}()

	var pending *tools.ApprovalRequest
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if req := mgr.GetPendingBySession("approval-lane-test"); req != nil {
			pending = req
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pending == nil {
		t.Fatal("expected pending approval")
	}

	if result := adapter.ResolveApproval(pending.ID, string(tools.ApprovalAllowOnce), "wrong-hash"); result.Resolved || !result.BindingMismatch {
		t.Fatalf("unexpected binding mismatch result: %#v", result)
	}

	if result := adapter.ResolveApproval(pending.ID, string(tools.ApprovalAllowOnce), pending.BindingHash); !result.Resolved || result.BindingMismatch {
		t.Fatalf("unexpected success result: %#v", result)
	}

	select {
	case decision := <-done:
		if decision != tools.ApprovalAllowOnce {
			t.Fatalf("decision=%q, want %q", decision, tools.ApprovalAllowOnce)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for approval resolution")
	}
}
