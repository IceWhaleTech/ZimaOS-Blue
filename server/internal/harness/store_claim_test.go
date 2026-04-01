package harness

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestSQLiteStore_ClaimNextGroupItem(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "harness-claim.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	store, err := NewSQLiteStore(db)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}

	group := &RunGroup{
		ID:          "group-claim",
		Kind:        RunGroupKindEval,
		Status:      RunGroupStatusQueued,
		OwnerUserID: "user-1",
	}
	if err := store.CreateGroup(ctx, group); err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	expiredLease := now.Add(-time.Minute)
	items := []RunGroupItem{
		{
			ID:          "item-busy",
			GroupID:     group.ID,
			Index:       0,
			RunKind:     RunKindAgentTask,
			Status:      RunGroupItemStatusQueued,
			LeaseOwner:  "worker-old",
			MaxAttempts: 1,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:             "item-ready",
			GroupID:        group.ID,
			Index:          1,
			RunKind:        RunKindAgentTask,
			Status:         RunGroupItemStatusQueued,
			LeaseExpiresAt: &expiredLease,
			MaxAttempts:    1,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
	}
	if err := store.CreateGroupItems(ctx, items); err != nil {
		t.Fatalf("CreateGroupItems failed: %v", err)
	}

	claimed, err := store.ClaimNextGroupItem(ctx, group.ID, "worker-1", 30*time.Second, now)
	if err != nil {
		t.Fatalf("ClaimNextGroupItem failed: %v", err)
	}
	if claimed.ID != "item-busy" {
		t.Fatalf("claimed item = %q, want %q", claimed.ID, "item-busy")
	}
	if claimed.Status != RunGroupItemStatusRunning {
		t.Fatalf("claimed status = %q, want %q", claimed.Status, RunGroupItemStatusRunning)
	}
	if claimed.LeaseOwner != "worker-1" {
		t.Fatalf("claimed lease owner = %q, want %q", claimed.LeaseOwner, "worker-1")
	}
	if claimed.LeaseExpiresAt == nil || !claimed.LeaseExpiresAt.Equal(now.Add(30*time.Second)) {
		t.Fatalf("claimed lease expires at = %#v, want %v", claimed.LeaseExpiresAt, now.Add(30*time.Second))
	}

	stored, err := store.GetGroupItem(ctx, "item-busy")
	if err != nil {
		t.Fatalf("GetGroupItem failed: %v", err)
	}
	if stored.Status != RunGroupItemStatusRunning {
		t.Fatalf("stored status = %q, want %q", stored.Status, RunGroupItemStatusRunning)
	}
	if stored.LeaseOwner != "worker-1" {
		t.Fatalf("stored lease owner = %q, want %q", stored.LeaseOwner, "worker-1")
	}

	claimed, err = store.ClaimNextGroupItem(ctx, group.ID, "worker-2", 45*time.Second, now.Add(time.Second))
	if err != nil {
		t.Fatalf("second ClaimNextGroupItem failed: %v", err)
	}
	if claimed.ID != "item-ready" {
		t.Fatalf("second claimed item = %q, want %q", claimed.ID, "item-ready")
	}

	_, err = store.ClaimNextGroupItem(ctx, group.ID, "worker-3", time.Minute, now.Add(2*time.Second))
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("third ClaimNextGroupItem error = %v, want sql.ErrNoRows", err)
	}
}
