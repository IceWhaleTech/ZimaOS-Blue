package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/labstack/echo/v4"
)

var activeHarnessRunStatuses = []string{
	"pending",
	"planning",
	"waiting_input",
	"executing",
	"verifying",
}

// RuntimeActivityTracker keeps a lightweight view of whether runtime mutations
// are still in flight and when the last mutation finished.
type RuntimeActivityTracker struct {
	inflightMutations atomic.Int64
	lastMutationUnix  atomic.Int64
}

func NewRuntimeActivityTracker() *RuntimeActivityTracker {
	t := &RuntimeActivityTracker{}
	t.markMutation(time.Now())
	return t
}

func (t *RuntimeActivityTracker) MutationMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !isMutationMethod(c.Request().Method) {
				return next(c)
			}
			done := t.BeginMutation()
			defer done()
			return next(c)
		}
	}
}

func (t *RuntimeActivityTracker) BeginMutation() func() {
	if t == nil {
		return func() {}
	}
	t.markMutation(time.Now())
	t.inflightMutations.Add(1)
	return func() {
		t.markMutation(time.Now())
		t.inflightMutations.Add(-1)
	}
}

func (t *RuntimeActivityTracker) HasInflightMutations() bool {
	return t != nil && t.inflightMutations.Load() > 0
}

func (t *RuntimeActivityTracker) IdleFor(now time.Time) time.Duration {
	if t == nil {
		return time.Hour
	}
	last := t.lastMutationUnix.Load()
	if last <= 0 {
		return time.Hour
	}
	return now.Sub(time.Unix(0, last))
}

func (t *RuntimeActivityTracker) CheckpointIdle(ctx context.Context, readDB *sql.DB, idleThreshold time.Duration) (bool, string, error) {
	if t == nil {
		return true, "", nil
	}
	if t.HasInflightMutations() {
		return false, "inflight_mutations", nil
	}

	idleFor := t.IdleFor(time.Now())
	if idleThreshold > 0 && idleFor < idleThreshold {
		return false, fmt.Sprintf("recent_mutation idle_for=%s threshold=%s", idleFor, idleThreshold), nil
	}

	activeRuns, err := countActiveHarnessRuns(ctx, readDB)
	if err != nil {
		return false, "", err
	}
	if activeRuns > 0 {
		return false, fmt.Sprintf("active_harness_runs=%d", activeRuns), nil
	}

	return true, "", nil
}

func (t *RuntimeActivityTracker) markMutation(now time.Time) {
	if t == nil {
		return
	}
	t.lastMutationUnix.Store(now.UnixNano())
}

func isMutationMethod(method string) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}

func countActiveHarnessRuns(ctx context.Context, db *sql.DB) (int, error) {
	if db == nil {
		return 0, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	query := `SELECT COUNT(*) FROM harness_runs WHERE status IN (?, ?, ?, ?, ?)`
	var count int
	err := db.QueryRowContext(
		ctx,
		query,
		activeHarnessRunStatuses[0],
		activeHarnessRunStatuses[1],
		activeHarnessRunStatuses[2],
		activeHarnessRunStatuses[3],
		activeHarnessRunStatuses[4],
	).Scan(&count)
	if err == nil {
		return count, nil
	}
	if errors.Is(err, sql.ErrNoRows) || strings.Contains(strings.ToLower(err.Error()), "no such table") {
		return 0, nil
	}
	return 0, err
}
