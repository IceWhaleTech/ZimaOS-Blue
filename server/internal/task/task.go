// Package task provides shared base types for async task lifecycle management.
// Each domain (mediagen, scheduler, etc.) keeps its own DB table and store;
// this package only defines the common status enum, base struct, and helpers.
package task

import "time"

// Status represents the lifecycle state of an async task.
type Status string

const (
	StatusPending    Status = "pending"
	StatusProcessing Status = "processing"
	StatusSucceeded  Status = "succeeded"
	StatusFailed     Status = "failed"
	StatusCancelled  Status = "cancelled"
)

// IsTerminal returns true if the status is a final state (no further transitions).
func (s Status) IsTerminal() bool {
	return s == StatusSucceeded || s == StatusFailed || s == StatusCancelled
}

// BaseTask contains the fields common to all async task types.
// Domain-specific task structs should embed this.
type BaseTask struct {
	ID          string     `json:"id"`
	Status      Status     `json:"status"`
	Error       string     `json:"error,omitempty"`
	Progress    float64    `json:"progress"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// SetFailed marks the task as failed with the given error message.
func (t *BaseTask) SetFailed(errMsg string) {
	t.Status = StatusFailed
	t.Error = errMsg
	now := time.Now()
	t.CompletedAt = &now
	t.UpdatedAt = now
}

// SetCompleted marks the task as succeeded.
func (t *BaseTask) SetCompleted() {
	t.Status = StatusSucceeded
	t.Progress = 1.0
	now := time.Now()
	t.CompletedAt = &now
	t.UpdatedAt = now
}

// SetProgress updates the progress value (clamped to 0.0–1.0).
func (t *BaseTask) SetProgress(p float64) {
	if p < 0 {
		p = 0
	}
	if p > 1 {
		p = 1
	}
	t.Progress = p
	t.UpdatedAt = time.Now()
}
