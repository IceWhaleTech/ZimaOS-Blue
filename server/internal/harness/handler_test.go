package harness

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestHandler_ListRunsSupportsTreeFilters(t *testing.T) {
	controller := newTestController(t)
	parentDriver := &stubDriver{kind: RunKindAgentTask}
	childDriver := &stubDriver{kind: RunKindSubagent}
	controller.RegisterDriver(parentDriver)
	controller.RegisterDriver(childDriver)

	parent, err := controller.Submit(context.Background(), RunSpec{
		Kind:         RunKindAgentTask,
		Goal:         "parent",
		UserID:       "user-1",
		ApprovalMode: ApprovalModeAsk,
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
	child.Status = RunStatusCompleted
	if err := controller.store.UpdateRun(context.Background(), child); err != nil {
		t.Fatalf("UpdateRun failed: %v", err)
	}

	if _, err := controller.Submit(context.Background(), RunSpec{
		Kind:   RunKindAgentTask,
		Goal:   "other",
		UserID: "user-1",
	}); err != nil {
		t.Fatalf("Submit sibling failed: %v", err)
	}

	handler := NewHandler(controller)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/runs?root_run_id="+parent.RootRunID+"&parent_run_id="+parent.ID+"&statuses=completed&kinds=subagent", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.ListRuns(c); err != nil {
		t.Fatalf("ListRuns returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var runs []Run
	if err := json.Unmarshal(rec.Body.Bytes(), &runs); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("runs len = %d, want 1; body=%s", len(runs), rec.Body.String())
	}
	if runs[0].ID != child.ID {
		t.Fatalf("run id = %q, want %q", runs[0].ID, child.ID)
	}
}
