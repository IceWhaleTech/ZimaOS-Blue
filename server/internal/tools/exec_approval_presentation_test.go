package tools

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
)

func TestApprovalManagerRequestApproval_PopulatesPresentationFields(t *testing.T) {
	broker := sse.NewBroker()
	defer broker.Close()
	sub := broker.Subscribe("user-1")
	defer broker.Unsubscribe("user-1", sub)

	mgr := NewApprovalManager(broker)

	done := make(chan error, 1)
	go func() {
		_, err := mgr.RequestApproval(context.Background(), ApprovalRequest{
			Type:            "command",
			Command:         "python scripts/sync.py --write",
			Workdir:         "/tmp/demo-workdir",
			UserID:          "user-1",
			SessionID:       "conv-exec-presentation",
			RiskLevel:       "high",
			Security:        "filesystem-write,network",
			ReferencedPaths: []string{"/tmp/demo-workdir/config.yaml"},
		})
		done <- err
	}()

	var pending *ApprovalRequest
	for i := 0; i < 100; i++ {
		pending = mgr.GetPendingBySession("conv-exec-presentation")
		if pending != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pending == nil {
		t.Fatal("expected pending exec approval request")
	}
	if strings.TrimSpace(pending.Purpose) == "" {
		t.Fatal("expected purpose presentation field")
	}
	if strings.TrimSpace(pending.RiskSummary) == "" {
		t.Fatal("expected risk_summary presentation field")
	}
	if strings.TrimSpace(pending.ScopeSummary) == "" {
		t.Fatal("expected scope_summary presentation field")
	}
	if strings.TrimSpace(pending.ExpectedEffects) == "" {
		t.Fatal("expected expected_effects presentation field")
	}
	if len(pending.AffectedTargets) == 0 {
		t.Fatalf("expected affected_targets, got %#v", pending.AffectedTargets)
	}

	if ok := mgr.ResolveApprovalWithBinding(pending.ID, ApprovalAllowOnce, pending.BindingHash); !ok {
		t.Fatal("expected exec approval resolution to succeed")
	}
	if err := <-done; err != nil {
		t.Fatalf("RequestApproval() error = %v", err)
	}
}
