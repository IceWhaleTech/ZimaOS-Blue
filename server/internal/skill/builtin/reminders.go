package builtin

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// ReminderServiceInterface defines the interface for the native reminder service.
type ReminderServiceInterface interface {
	Add(ctx context.Context, ownerID, message string, fireAt time.Time, recurring, sessionID string) (ReminderInfo, error)
	List(ctx context.Context, ownerID string) ([]ReminderInfo, error)
	Delete(ctx context.Context, ownerID, id string) error
	Clear(ctx context.Context, ownerID string) (int64, error)
}

// ReminderInfo is the data returned by the reminder service interface.
type ReminderInfo struct {
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	FireAt    time.Time `json:"fire_at"`
	Recurring string    `json:"recurring,omitempty"`
	SessionID string    `json:"session_id,omitempty"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// ReminderItem represents a single reminder (in-memory fallback).
type ReminderItem struct {
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	Time      time.Time `json:"time"`
	Recurring string    `json:"recurring,omitempty"`
	Created   time.Time `json:"created"`
}

// Reminders is a built-in reminders skill with optional native service backing.
type Reminders struct {
	manifest  *skill.Manifest
	mu        sync.RWMutex
	svc       ReminderServiceInterface
	// In-memory fallback when svc is nil
	reminders map[string]*ReminderItem
	counter   int
}

// NewReminders creates a new reminders skill.
func NewReminders() *Reminders {
	return &Reminders{
		manifest: &skill.Manifest{
			ID:          "push_notification",
			Name:        "Push Notification",
			Version:     "2.0.0",
			Description: "Send push notifications and manage scheduled alerts. Delivers via SSE, Web Push, and native OS notifications (macOS Notification Center, Linux notify-send, Windows toast). Supports relative times (1h, 30m) and absolute times (2026-01-04 09:00, tomorrow 9:00).",
			Category:    "productivity",
			Icon:        "notifications",
			Tags:        []string{"push", "notification", "alert", "schedule", "productivity"},
			Inputs: []skill.Parameter{
				{
					Name:        "action",
					Type:        "string",
					Description: "Action to perform: add, list, delete, clear",
					Required:    true,
				},
				{
					Name:        "message",
					Type:        "string",
					Description: "Reminder message (required for add)",
					Required:    false,
				},
				{
					Name:        "time",
					Type:        "string",
					Description: "Reminder time: relative duration (e.g., '1h', '30m', '2h30m') or RFC3339 (e.g., '2026-01-04T09:00:00+08:00')",
					Required:    false,
				},
				{
					Name:        "id",
					Type:        "string",
					Description: "Reminder ID (required for delete)",
					Required:    false,
				},
				{
					Name:        "recurring",
					Type:        "string",
					Description: "Recurring schedule: daily, weekly, monthly (optional for add)",
					Required:    false,
				},
				{
					Name:        "session_id",
					Type:        "string",
					Description: "Target conversation ID to deliver the reminder to (optional, defaults to most recent)",
					Required:    false,
				},
			},
			Outputs: []skill.Parameter{
				{
					Name:        "reminders",
					Type:        "array",
					Description: "List of reminders",
				},
				{
					Name:        "reminder",
					Type:        "object",
					Description: "Created/deleted reminder",
				},
			},
		},
		reminders: make(map[string]*ReminderItem),
	}
}

// SetReminderService injects the native reminder service.
func (r *Reminders) SetReminderService(svc ReminderServiceInterface) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.svc = svc
}

// Manifest returns the skill manifest.
func (r *Reminders) Manifest() *skill.Manifest {
	return r.manifest
}

// Validate validates the input parameters.
func (r *Reminders) Validate(input map[string]any) error {
	action, ok := input["action"]
	if !ok {
		return fmt.Errorf("action is required")
	}

	actionStr, ok := action.(string)
	if !ok {
		return fmt.Errorf("action must be a string")
	}

	validActions := map[string]bool{"add": true, "list": true, "delete": true, "clear": true}
	if !validActions[actionStr] {
		return fmt.Errorf("invalid action: %s", actionStr)
	}

	if actionStr == "add" {
		if _, ok := input["message"]; !ok {
			return fmt.Errorf("message is required for add action")
		}
		if _, ok := input["time"]; !ok {
			return fmt.Errorf("time is required for add action")
		}
	}

	if actionStr == "delete" {
		if _, ok := input["id"]; !ok {
			return fmt.Errorf("id is required for delete action")
		}
	}

	return nil
}

// Execute executes the reminders skill.
func (r *Reminders) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	action := input["action"].(string)

	r.mu.RLock()
	svc := r.svc
	r.mu.RUnlock()

	// Delegate to native service if available
	if svc != nil {
		return r.executeNative(ctx, svc, action, input)
	}

	// Fallback to in-memory
	switch action {
	case "add":
		return r.addReminderFallback(input)
	case "list":
		return r.listRemindersFallback()
	case "delete":
		return r.deleteReminderFallback(input)
	case "clear":
		return r.clearRemindersFallback()
	}

	return skill.NewErrorResult(fmt.Errorf("unknown action: %s", action)), nil
}

// executeNative delegates to the persistent reminder service.
func (r *Reminders) executeNative(ctx context.Context, svc ReminderServiceInterface, action string, input map[string]any) (*skill.Result, error) {
	ownerID := skill.GetUserID(ctx)
	if ownerID == "" {
		ownerID = "default"
	}

	switch action {
	case "add":
		return r.addReminderNative(ctx, svc, ownerID, input)
	case "list":
		return r.listRemindersNative(ctx, svc, ownerID)
	case "delete":
		return r.deleteReminderNative(ctx, svc, ownerID, input)
	case "clear":
		return r.clearRemindersNative(ctx, svc, ownerID)
	}

	return skill.NewErrorResult(fmt.Errorf("unknown action: %s", action)), nil
}

func (r *Reminders) addReminderNative(ctx context.Context, svc ReminderServiceInterface, ownerID string, input map[string]any) (*skill.Result, error) {
	message := input["message"].(string)
	timeStr := input["time"].(string)

	fireAt, err := parseReminderTime(timeStr)
	if err != nil {
		return skill.NewErrorResult(err), nil
	}

	recurring := ""
	if rec, ok := input["recurring"].(string); ok {
		recurring = rec
	}
	sessionID := ""
	if sid, ok := input["session_id"].(string); ok {
		sessionID = sid
	}

	info, err := svc.Add(ctx, ownerID, message, fireAt, recurring, sessionID)
	if err != nil {
		return skill.NewErrorResult(fmt.Errorf("failed to add reminder: %w", err)), nil
	}

	return skill.NewResult(map[string]any{
		"reminder": info,
		"message":  fmt.Sprintf("Reminder set: %s — %s", message, fireAt.Format("2006-01-02 15:04")),
	}), nil
}

func (r *Reminders) listRemindersNative(ctx context.Context, svc ReminderServiceInterface, ownerID string) (*skill.Result, error) {
	list, err := svc.List(ctx, ownerID)
	if err != nil {
		return skill.NewErrorResult(err), nil
	}

	return skill.NewResult(map[string]any{
		"reminders": list,
		"count":     len(list),
	}), nil
}

func (r *Reminders) deleteReminderNative(ctx context.Context, svc ReminderServiceInterface, ownerID string, input map[string]any) (*skill.Result, error) {
	id := input["id"].(string)

	if err := svc.Delete(ctx, ownerID, id); err != nil {
		return skill.NewErrorResult(err), nil
	}

	return skill.NewResult(map[string]any{
		"deleted": true,
		"id":      id,
		"message": fmt.Sprintf("Reminder '%s' deleted", id),
	}), nil
}

func (r *Reminders) clearRemindersNative(ctx context.Context, svc ReminderServiceInterface, ownerID string) (*skill.Result, error) {
	count, err := svc.Clear(ctx, ownerID)
	if err != nil {
		return skill.NewErrorResult(err), nil
	}

	return skill.NewResult(map[string]any{
		"cleared": count,
		"message": fmt.Sprintf("Cleared %d reminders", count),
	}), nil
}

// parseReminderTime parses a time string as either a duration or RFC3339.
func parseReminderTime(s string) (time.Time, error) {
	// Try relative duration first
	if d, err := time.ParseDuration(s); err == nil {
		return timeutil.NowTime().Add(d), nil
	}
	// Try RFC3339
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	// Try common format without timezone
	if t, err := time.ParseInLocation("2006-01-02 15:04", s, time.Local); err == nil {
		return t, nil
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.Local); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid time format: %s (use duration like '1h30m', RFC3339, or 'YYYY-MM-DD HH:MM')", s)
}

// --- In-memory fallback methods (used when native service is not wired) ---

func (r *Reminders) addReminderFallback(input map[string]any) (*skill.Result, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	message := input["message"].(string)
	timeStr := input["time"].(string)

	reminderTime, err := parseReminderTime(timeStr)
	if err != nil {
		return skill.NewErrorResult(err), nil
	}

	r.counter++
	id := fmt.Sprintf("reminder-%d", r.counter)

	reminder := &ReminderItem{
		ID:      id,
		Message: message,
		Time:    reminderTime,
		Created: timeutil.NowTime(),
	}
	if recurring, ok := input["recurring"].(string); ok {
		reminder.Recurring = recurring
	}

	r.reminders[id] = reminder

	return skill.NewResult(map[string]any{
		"reminder": reminder,
		"message":  fmt.Sprintf("%s — %s (in-memory only, will not persist)", message, reminderTime.Format("2006-01-02 15:04")),
	}), nil
}

func (r *Reminders) listRemindersFallback() (*skill.Result, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	reminders := make([]*ReminderItem, 0, len(r.reminders))
	for _, reminder := range r.reminders {
		reminders = append(reminders, reminder)
	}

	return skill.NewResult(map[string]any{
		"reminders": reminders,
		"count":     len(reminders),
	}), nil
}

func (r *Reminders) deleteReminderFallback(input map[string]any) (*skill.Result, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := input["id"].(string)
	if reminder, ok := r.reminders[id]; ok {
		delete(r.reminders, id)
		return skill.NewResult(map[string]any{
			"deleted":  true,
			"reminder": reminder,
			"message":  fmt.Sprintf("Reminder '%s' deleted", id),
		}), nil
	}

	return skill.NewResult(map[string]any{
		"deleted": false,
		"message": fmt.Sprintf("Reminder '%s' not found", id),
	}), nil
}

func (r *Reminders) clearRemindersFallback() (*skill.Result, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	count := len(r.reminders)
	r.reminders = make(map[string]*ReminderItem)

	return skill.NewResult(map[string]any{
		"cleared": count,
		"message": fmt.Sprintf("Cleared %d reminders", count),
	}), nil
}
