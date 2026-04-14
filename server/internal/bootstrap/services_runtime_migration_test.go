package bootstrap

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agent"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentsessions"
	dbutil "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
)

func TestPrepareRuntimeDatabase_MovesRuntimeTablesOutOfBlueDB(t *testing.T) {
	dataDir := t.TempDir()
	primaryConn, err := dbutil.OpenSQLite(filepath.Join(dataDir, "blue.db"), &dbutil.SQLiteOpenOpts{
		SkipIntegrityCheckOnOpen: true,
	})
	if err != nil {
		t.Fatalf("open primary db: %v", err)
	}
	defer primaryConn.Close()
	runtimeConn, err := dbutil.OpenSQLite(filepath.Join(dataDir, "runtime.db"), &dbutil.SQLiteOpenOpts{
		SkipIntegrityCheckOnOpen: true,
	})
	if err != nil {
		t.Fatalf("open runtime db: %v", err)
	}
	defer runtimeConn.Close()
	if _, err := harness.NewSQLiteStore(primaryConn.Writer); err != nil {
		t.Fatalf("prepare primary harness schema: %v", err)
	}
	if _, err := agent.NewStore(primaryConn.Writer); err != nil {
		t.Fatalf("prepare primary agent schema: %v", err)
	}
	if _, err := agentsessions.NewSQLiteStore(primaryConn.Writer); err != nil {
		t.Fatalf("prepare primary agent sessions schema: %v", err)
	}
	if _, err := primaryConn.Writer.Exec(`
		INSERT INTO harness_runs (id, root_run_id, kind, status, goal, created_at, updated_at)
		VALUES ('run-1', 'run-1', 'agent_task', 'completed', 'ship it', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`); err != nil {
		t.Fatalf("insert harness run: %v", err)
	}
	if _, err := primaryConn.Writer.Exec(`
		INSERT INTO agent_tasks (id, user_id, goal, status, created_at, updated_at)
		VALUES ('task-1', 'user-1', 'ship it', 'completed', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`); err != nil {
		t.Fatalf("insert agent task: %v", err)
	}
	if _, err := primaryConn.Writer.Exec(`
		INSERT INTO agent_profiles (id, protocol, name, builtin, data, updated_at)
		VALUES ('profile-1', 'acp', 'ACP', 0, '{"id":"profile-1"}', CURRENT_TIMESTAMP)
	`); err != nil {
		t.Fatalf("insert agent profile: %v", err)
	}
	if _, err := primaryConn.Writer.Exec(`
		INSERT INTO agent_sessions (id, profile_id, protocol, user_id, name, status, data, created_at, updated_at)
		VALUES ('session-1', 'profile-1', 'acp', 'user-1', 'sess', 'idle', '{"id":"session-1"}', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`); err != nil {
		t.Fatalf("insert agent session: %v", err)
	}
	if _, err := primaryConn.Writer.Exec(`
		INSERT INTO agent_runs (id, session_id, status, data, created_at, updated_at)
		VALUES ('run-session-1', 'session-1', 'completed', '{"id":"run-session-1"}', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`); err != nil {
		t.Fatalf("insert agent run: %v", err)
	}
	if _, err := primaryConn.Writer.Exec(`
		INSERT INTO agent_run_events (session_id, run_id, event_index, type, payload, created_at)
		VALUES ('session-1', 'run-session-1', 0, 'message', '{"chunk":"hi"}', CURRENT_TIMESTAMP)
	`); err != nil {
		t.Fatalf("insert agent run event: %v", err)
	}

	result, err := PrepareRuntimeDatabase(context.Background(), dataDir, primaryConn, runtimeConn, zap.NewNop())
	if err != nil {
		t.Fatalf("PrepareRuntimeDatabase() error = %v", err)
	}
	if result == nil || result.RowsMoved != 6 {
		t.Fatalf("PrepareRuntimeDatabase() result = %#v, want 6 moved rows", result)
	}

	assertTableCount(t, runtimeConn.Writer, "harness_runs", 1)
	assertTableCount(t, runtimeConn.Writer, "agent_tasks", 1)
	assertTableCount(t, runtimeConn.Writer, "agent_profiles", 1)
	assertTableCount(t, runtimeConn.Writer, "agent_sessions", 1)
	assertTableCount(t, runtimeConn.Writer, "agent_runs", 1)
	assertTableCount(t, runtimeConn.Writer, "agent_run_events", 1)
	assertTableMissing(t, primaryConn.Writer, "harness_runs")
	assertTableMissing(t, primaryConn.Writer, "agent_tasks")
	assertTableMissing(t, primaryConn.Writer, "agent_profiles")
	assertTableMissing(t, primaryConn.Writer, "agent_sessions")
	assertTableMissing(t, primaryConn.Writer, "agent_runs")
	assertTableMissing(t, primaryConn.Writer, "agent_run_events")
}

func assertTableCount(t *testing.T, db *sql.DB, table string, want int) {
	t.Helper()
	var got int
	if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&got); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if got != want {
		t.Fatalf("%s row count = %d, want %d", table, got, want)
	}
}

func assertTableMissing(t *testing.T, db *sql.DB, table string) {
	t.Helper()
	var count int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?",
		table,
	).Scan(&count); err != nil {
		t.Fatalf("check %s existence: %v", table, err)
	}
	if count != 0 {
		t.Fatalf("expected %s to be removed from blue.db", table)
	}
}
