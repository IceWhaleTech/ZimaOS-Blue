package task

import (
	"database/sql"
	"testing"
	"time"
)

func TestStatus_IsTerminal(t *testing.T) {
	tests := []struct {
		status   Status
		terminal bool
	}{
		{StatusPending, false},
		{StatusProcessing, false},
		{StatusSucceeded, true},
		{StatusFailed, true},
		{StatusCancelled, true},
	}
	for _, tt := range tests {
		if got := tt.status.IsTerminal(); got != tt.terminal {
			t.Errorf("Status(%q).IsTerminal() = %v, want %v", tt.status, got, tt.terminal)
		}
	}
}

func TestBaseTask_SetFailed(t *testing.T) {
	bt := &BaseTask{ID: "t1", Status: StatusProcessing}
	bt.SetFailed("something broke")

	if bt.Status != StatusFailed {
		t.Errorf("status = %q, want %q", bt.Status, StatusFailed)
	}
	if bt.Error != "something broke" {
		t.Errorf("error = %q, want %q", bt.Error, "something broke")
	}
	if bt.CompletedAt == nil {
		t.Fatal("CompletedAt should be set")
	}
}

func TestBaseTask_SetCompleted(t *testing.T) {
	bt := &BaseTask{ID: "t2", Status: StatusProcessing, Progress: 0.5}
	bt.SetCompleted()

	if bt.Status != StatusSucceeded {
		t.Errorf("status = %q, want %q", bt.Status, StatusSucceeded)
	}
	if bt.Progress != 1.0 {
		t.Errorf("progress = %f, want 1.0", bt.Progress)
	}
	if bt.CompletedAt == nil {
		t.Fatal("CompletedAt should be set")
	}
}

func TestBaseTask_SetProgress(t *testing.T) {
	bt := &BaseTask{ID: "t3"}

	bt.SetProgress(0.5)
	if bt.Progress != 0.5 {
		t.Errorf("progress = %f, want 0.5", bt.Progress)
	}

	// Clamp below 0
	bt.SetProgress(-1)
	if bt.Progress != 0 {
		t.Errorf("progress = %f, want 0", bt.Progress)
	}

	// Clamp above 1
	bt.SetProgress(2.0)
	if bt.Progress != 1.0 {
		t.Errorf("progress = %f, want 1.0", bt.Progress)
	}
}

func TestTimeConversion(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	s := TimeToSQL(now)
	got := TimeFromSQL(s)
	if !got.Equal(now) {
		t.Errorf("TimeFromSQL(TimeToSQL(%v)) = %v", now, got)
	}

	// Empty string → zero time
	zero := TimeFromSQL("")
	if !zero.IsZero() {
		t.Errorf("TimeFromSQL(\"\") should be zero, got %v", zero)
	}
}

func TestNullTimeConversion(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	// Non-nil
	ns := NullTimeToSQL(&now)
	if !ns.Valid {
		t.Fatal("NullTimeToSQL should be valid for non-nil time")
	}
	got := NullTimeFromSQL(ns)
	if got == nil || !got.Equal(now) {
		t.Errorf("round-trip failed: got %v, want %v", got, now)
	}

	// Nil
	ns2 := NullTimeToSQL(nil)
	if ns2.Valid {
		t.Error("NullTimeToSQL(nil) should be invalid")
	}
	got2 := NullTimeFromSQL(sql.NullString{})
	if got2 != nil {
		t.Errorf("NullTimeFromSQL(invalid) should be nil, got %v", got2)
	}
}
