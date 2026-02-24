package push

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestStoreCreateAndGet(t *testing.T) {
	db := testDB(t)
	store, err := NewStore(db)
	if err != nil {
		t.Fatal(err)
	}

	r := &PushNotification{
		ID:      "rem-1",
		OwnerID: "user-1",
		Message: "Take medicine",
		FireAt:  time.Now().Add(time.Hour),
	}
	if err := store.Create(context.Background(), r); err != nil {
		t.Fatal(err)
	}

	got, err := store.Get(context.Background(), "rem-1")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected notification, got nil")
	}
	if got.Message != "Take medicine" {
		t.Errorf("message = %q, want %q", got.Message, "Take medicine")
	}
	if got.Status != StatusPending {
		t.Errorf("status = %q, want %q", got.Status, StatusPending)
	}
}

func TestStoreListByOwner(t *testing.T) {
	db := testDB(t)
	store, err := NewStore(db)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	for i, msg := range []string{"A", "B", "C"} {
		store.Create(ctx, &PushNotification{
			ID:      fmt.Sprintf("rem-%d", i),
			OwnerID: "user-1",
			Message: msg,
			FireAt:  time.Now().Add(time.Duration(i) * time.Hour),
		})
	}
	// Different owner
	store.Create(ctx, &PushNotification{
		ID:      "rem-other",
		OwnerID: "user-2",
		Message: "Other",
		FireAt:  time.Now(),
	})

	list, err := store.ListByOwner(ctx, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Errorf("got %d notifications, want 3", len(list))
	}

	// Filter by status
	now := time.Now()
	store.UpdateStatus(ctx, "rem-0", StatusFired, &now)
	pending, err := store.ListByOwner(ctx, "user-1", StatusPending)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 2 {
		t.Errorf("got %d pending, want 2", len(pending))
	}
}

func TestStoreDelete(t *testing.T) {
	db := testDB(t)
	store, err := NewStore(db)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	store.Create(ctx, &PushNotification{
		ID:      "rem-1",
		OwnerID: "user-1",
		Message: "Test",
		FireAt:  time.Now(),
	})

	if err := store.Delete(ctx, "rem-1", "user-1"); err != nil {
		t.Fatal(err)
	}

	got, err := store.Get(ctx, "rem-1")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Error("expected nil after delete")
	}

	// Wrong owner
	store.Create(ctx, &PushNotification{
		ID:      "rem-2",
		OwnerID: "user-1",
		Message: "Test",
		FireAt:  time.Now(),
	})
	if err := store.Delete(ctx, "rem-2", "user-2"); err == nil {
		t.Error("expected error deleting with wrong owner")
	}
}

func TestStoreDeleteByOwner(t *testing.T) {
	db := testDB(t)
	store, err := NewStore(db)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	for i := 0; i < 3; i++ {
		store.Create(ctx, &PushNotification{
			ID:      fmt.Sprintf("rem-%d", i),
			OwnerID: "user-1",
			Message: "Test",
			FireAt:  time.Now(),
		})
	}

	n, err := store.DeleteByOwner(ctx, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("deleted %d, want 3", n)
	}
}

func TestStoreListPending(t *testing.T) {
	db := testDB(t)
	store, err := NewStore(db)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	store.Create(ctx, &PushNotification{ID: "rem-1", OwnerID: "u1", Message: "A", FireAt: time.Now()})
	store.Create(ctx, &PushNotification{ID: "rem-2", OwnerID: "u2", Message: "B", FireAt: time.Now()})

	now := time.Now()
	store.UpdateStatus(ctx, "rem-1", StatusFired, &now)

	pending, err := store.ListPending(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 {
		t.Errorf("got %d pending, want 1", len(pending))
	}
	if pending[0].ID != "rem-2" {
		t.Errorf("got ID %q, want rem-2", pending[0].ID)
	}
}

func TestStoreUpdateCronJobID(t *testing.T) {
	db := testDB(t)
	store, err := NewStore(db)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	store.Create(ctx, &PushNotification{ID: "rem-1", OwnerID: "u1", Message: "A", FireAt: time.Now()})

	if err := store.UpdateCronJobID(ctx, "rem-1", "job-abc"); err != nil {
		t.Fatal(err)
	}

	got, _ := store.Get(ctx, "rem-1")
	if got.CronJobID != "job-abc" {
		t.Errorf("cron_job_id = %q, want job-abc", got.CronJobID)
	}
}
