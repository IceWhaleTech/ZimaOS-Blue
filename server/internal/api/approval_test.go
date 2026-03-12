package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestApprovalListPendingFiltersBySessionID(t *testing.T) {
	e := echo.New()
	h := NewApprovalHandler(nil)
	h.pending["req-1"] = &PendingRequest{
		ID:        "req-1",
		ToolName:  "browser",
		SessionID: "conv-1",
	}
	h.pending["req-2"] = &PendingRequest{
		ID:        "req-2",
		ToolName:  "exec",
		SessionID: "conv-2",
	}

	req := httptest.NewRequest(http.MethodGet, "/approval/pending?session_id=conv-2", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.ListPending(c); err != nil {
		t.Fatalf("ListPending() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var got []PendingRequest
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0].ID != "req-2" {
		t.Fatalf("got[0].ID = %q, want %q", got[0].ID, "req-2")
	}
	if got[0].SessionID != "conv-2" {
		t.Fatalf("got[0].SessionID = %q, want %q", got[0].SessionID, "conv-2")
	}
}
