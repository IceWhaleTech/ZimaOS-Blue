package billing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
)

func withUserClaims(c echo.Context, role string) {
	req := c.Request()
	claims := &auth.UserClaims{
		UserID:   "u_admin",
		Username: "admin",
		Role:     role,
	}
	ctx := context.WithValue(req.Context(), auth.UserContextKey, claims)
	c.SetRequest(req.WithContext(ctx))
}

func TestHandlerGetSummaryAuthGuards(t *testing.T) {
	h := NewHandler(NewService(&mockStorage{}))
	e := echo.New()

	// No auth -> 401
	reqUnauth := httptest.NewRequest(http.MethodGet, "/billing/summary", nil)
	recUnauth := httptest.NewRecorder()
	cUnauth := e.NewContext(reqUnauth, recUnauth)
	err := h.GetSummary(cUnauth)
	if err == nil {
		t.Fatalf("expected unauthorized error")
	}
	httpErr, ok := err.(*echo.HTTPError)
	if !ok || httpErr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %#v", err)
	}

	// Non-admin -> 403
	reqUser := httptest.NewRequest(http.MethodGet, "/billing/summary", nil)
	recUser := httptest.NewRecorder()
	cUser := e.NewContext(reqUser, recUser)
	withUserClaims(cUser, "user")
	err = h.GetSummary(cUser)
	if err == nil {
		t.Fatalf("expected forbidden error")
	}
	httpErr, ok = err.(*echo.HTTPError)
	if !ok || httpErr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %#v", err)
	}
}

func TestHandlerGetSummarySuccess(t *testing.T) {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	storage := &mockStorage{records: []*providerpool.UsageRecord{
		{
			ProviderID:    "p1",
			ModelID:       "m1",
			Timestamp:     now,
			InputTokens:   100,
			OutputTokens:  40,
			RequestCount:  1,
			Success:       true,
			EstimatedCost: 0.0123,
		},
	}}

	h := NewHandler(NewService(storage))
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/billing/summary?from=2026-02-28T00:00:00Z&to=2026-03-03T00:00:00Z&group_by=provider", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	withUserClaims(c, "admin")

	if err := h.GetSummary(c); err != nil {
		t.Fatalf("GetSummary returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", rec.Code)
	}

	var resp SummaryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.GroupBy != "provider" {
		t.Fatalf("group_by=%q, want provider", resp.GroupBy)
	}
	if resp.Totals.RequestCount != 1 {
		t.Fatalf("request_count=%d, want 1", resp.Totals.RequestCount)
	}
	if len(resp.Breakdown) != 1 || resp.Breakdown[0].ProviderID != "p1" {
		t.Fatalf("unexpected breakdown: %#v", resp.Breakdown)
	}
}

func TestHandlerGetSummaryBadRange(t *testing.T) {
	h := NewHandler(NewService(&mockStorage{}))
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/billing/summary?from=2026-03-02T00:00:00Z&to=2026-03-02T00:00:00Z", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	withUserClaims(c, "admin")

	err := h.GetSummary(c)
	if err == nil {
		t.Fatalf("expected bad request error")
	}
	httpErr, ok := err.(*echo.HTTPError)
	if !ok || httpErr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %#v", err)
	}
}

func TestHandlerExportCSV(t *testing.T) {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	storage := &mockStorage{records: []*providerpool.UsageRecord{
		{
			ProviderID:    "p1",
			ModelID:       "m1",
			Timestamp:     now,
			InputTokens:   22,
			OutputTokens:  11,
			RequestCount:  1,
			Success:       true,
			EstimatedCost: 0.001,
		},
	}}

	h := NewHandler(NewService(storage))
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/billing/export?from=2026-02-28T00:00:00Z&to=2026-03-03T00:00:00Z", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	withUserClaims(c, "admin")

	if err := h.Export(c); err != nil {
		t.Fatalf("Export returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Disposition"); !strings.Contains(got, "billing_20260228_20260302.csv") {
		t.Fatalf("unexpected content-disposition: %q", got)
	}
	if !strings.Contains(rec.Body.String(), "timestamp,provider_id,model_id") {
		t.Fatalf("csv header missing: %s", rec.Body.String())
	}
}
