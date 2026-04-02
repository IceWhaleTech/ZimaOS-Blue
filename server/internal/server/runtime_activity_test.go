package server

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	_ "github.com/mattn/go-sqlite3"
)

func TestRuntimeActivityTrackerMutationMiddlewareTracksMutationRequests(t *testing.T) {
	tracker := NewRuntimeActivityTracker()
	e := echo.New()
	e.Use(tracker.MutationMiddleware())

	var sawInflight bool
	e.POST("/chat", func(c echo.Context) error {
		sawInflight = tracker.HasInflightMutations()
		return c.NoContent(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodPost, "/chat", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)
	if !sawInflight {
		t.Fatal("expected POST handler to observe inflight mutation")
	}
	if tracker.HasInflightMutations() {
		t.Fatal("expected no inflight mutation after request finished")
	}
}

func TestRuntimeActivityTrackerCheckpointIdleSkipsRecentMutationAndActiveHarnessRuns(t *testing.T) {
	tracker := NewRuntimeActivityTracker()

	idle, reason, err := tracker.CheckpointIdle(context.Background(), nil, 5*time.Minute)
	if err != nil {
		t.Fatalf("recent mutation check error: %v", err)
	}
	if idle {
		t.Fatalf("expected recent mutation to block idle checkpoint, reason=%q", reason)
	}
	if !strings.Contains(reason, "recent_mutation") {
		t.Fatalf("reason = %q, want recent_mutation", reason)
	}

	tracker.lastMutationUnix.Store(time.Now().Add(-10 * time.Minute).UnixNano())

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE harness_runs (status TEXT)`); err != nil {
		t.Fatalf("create harness_runs: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO harness_runs(status) VALUES ('executing')`); err != nil {
		t.Fatalf("insert harness run: %v", err)
	}

	idle, reason, err = tracker.CheckpointIdle(context.Background(), db, 5*time.Minute)
	if err != nil {
		t.Fatalf("active harness check error: %v", err)
	}
	if idle {
		t.Fatalf("expected active harness run to block idle checkpoint, reason=%q", reason)
	}
	if reason != "active_harness_runs=1" {
		t.Fatalf("reason = %q, want active_harness_runs=1", reason)
	}
}
