package builtin

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/remindertime"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// PushServiceInterface defines the interface for the native push notification service.
type PushServiceInterface interface {
	Add(ctx context.Context, ownerID, message string, fireAt time.Time, recurring, sessionID string) (PushInfo, error)
	List(ctx context.Context, ownerID string) ([]PushInfo, error)
	Delete(ctx context.Context, ownerID, id string) error
	Clear(ctx context.Context, ownerID string) (int64, error)
}

// PushInfo is the data returned by the push notification service interface.
type PushInfo struct {
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	FireAt    time.Time `json:"fire_at"`
	Recurring string    `json:"recurring,omitempty"`
	SessionID string    `json:"session_id,omitempty"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// PushItem represents a single push notification (in-memory fallback).
type PushItem struct {
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	Time      time.Time `json:"time"`
	Recurring string    `json:"recurring,omitempty"`
	Created   time.Time `json:"created"`
}

// Reminder is a built-in reminder/notification skill with optional native service backing.
type Reminder struct {
	manifest *skill.Manifest
	mu       sync.RWMutex
	svc      PushServiceInterface
	// In-memory fallback when svc is nil
	notifications map[string]*PushItem
	counter       int
}

// NewReminder creates a new reminder skill.
func NewReminder() *Reminder {
	return &Reminder{
		manifest: &skill.Manifest{
			ID:          "reminder",
			Name:        "Reminder",
			Version:     "2.0.0",
			Description: "Manage reminders and scheduled alerts. Delivers via SSE, Web Push, and native OS notifications (macOS Notification Center, Linux notify-send, Windows toast). Supports relative times (1h, 30m) and absolute times (2026-01-04 09:00, tomorrow 9:00).",
			Category:    "productivity",
			Icon:        "notifications",
			Tags:        []string{"reminder", "notification", "alert", "schedule", "productivity"},
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
					Description: "Notification message (required for add)",
					Required:    false,
				},
				{
					Name:        "time",
					Type:        "string",
					Description: "Notification time: relative duration (e.g., '1h', '30m', '2h30m') or RFC3339 (e.g., '2026-01-04T09:00:00+08:00')",
					Required:    false,
				},
				{
					Name:        "id",
					Type:        "string",
					Description: "Notification ID (required for delete)",
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
					Description: "Target conversation ID to deliver the notification to (optional, defaults to most recent)",
					Required:    false,
				},
				{
					Name:        "locale",
					Type:        "string",
					Description: "Language/locale code for localized responses (e.g., en-US, zh-CN)",
					Required:    false,
				},
			},
			Outputs: []skill.Parameter{
				{
					Name:        "notifications",
					Type:        "array",
					Description: "List of notifications",
				},
				{
					Name:        "notification",
					Type:        "object",
					Description: "Created/deleted notification",
				},
			},
		},
		notifications: make(map[string]*PushItem),
	}
}

// SetPushService injects the native push notification service.
func (p *Reminder) SetPushService(svc PushServiceInterface) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.svc = svc
}

// Manifest returns the skill manifest.
func (p *Reminder) Manifest() *skill.Manifest {
	return p.manifest
}

// Validate validates the input parameters.
func (p *Reminder) Validate(input map[string]any) error {
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

// Execute executes the push notification skill.
func (p *Reminder) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	action := input["action"].(string)

	p.mu.RLock()
	svc := p.svc
	p.mu.RUnlock()

	// Delegate to native service if available
	if svc != nil {
		return p.executeNative(ctx, svc, action, input)
	}

	// Fallback to in-memory
	switch action {
	case "add":
		return p.addFallback(input)
	case "list":
		return p.listFallback()
	case "delete":
		return p.deleteFallback(input)
	case "clear":
		return p.clearFallback()
	}

	return skill.NewErrorResult(fmt.Errorf("unknown action: %s", action)), nil
}

// executeNative delegates to the persistent push notification service.
func (p *Reminder) executeNative(ctx context.Context, svc PushServiceInterface, action string, input map[string]any) (*skill.Result, error) {
	ownerID := skill.GetUserID(ctx)
	if ownerID == "" {
		ownerID = "default"
	}

	switch action {
	case "add":
		return p.addNative(ctx, svc, ownerID, input)
	case "list":
		return p.listNative(ctx, svc, ownerID)
	case "delete":
		return p.deleteNative(ctx, svc, ownerID, input)
	case "clear":
		return p.clearNative(ctx, svc, ownerID)
	}

	return skill.NewErrorResult(fmt.Errorf("unknown action: %s", action)), nil
}

func (p *Reminder) addNative(ctx context.Context, svc PushServiceInterface, ownerID string, input map[string]any) (*skill.Result, error) {
	message := input["message"].(string)
	timeStr := input["time"].(string)

	fireAt, err := parsePushTime(timeStr)
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
		"notification": info,
		"message":      fmt.Sprintf("Notification set: %s — %s", message, fireAt.Format("2006-01-02 15:04")),
	}), nil
}

func (p *Reminder) listNative(ctx context.Context, svc PushServiceInterface, ownerID string) (*skill.Result, error) {
	list, err := svc.List(ctx, ownerID)
	if err != nil {
		return skill.NewErrorResult(err), nil
	}

	return skill.NewResult(map[string]any{
		"notifications": list,
		"count":         len(list),
	}), nil
}

func (p *Reminder) deleteNative(ctx context.Context, svc PushServiceInterface, ownerID string, input map[string]any) (*skill.Result, error) {
	id := input["id"].(string)

	if err := svc.Delete(ctx, ownerID, id); err != nil {
		return skill.NewErrorResult(err), nil
	}

	return skill.NewResult(map[string]any{
		"deleted": true,
		"id":      id,
		"message": fmt.Sprintf("Notification '%s' deleted", id),
	}), nil
}

func (p *Reminder) clearNative(ctx context.Context, svc PushServiceInterface, ownerID string) (*skill.Result, error) {
	count, err := svc.Clear(ctx, ownerID)
	if err != nil {
		return skill.NewErrorResult(err), nil
	}

	return skill.NewResult(map[string]any{
		"cleared": count,
		"message": fmt.Sprintf("Cleared %d notifications", count),
	}), nil
}

// parsePushTime parses a time string as either a duration or RFC3339.
func parsePushTime(s string) (time.Time, error) {
	return remindertime.Parse(s)
}

// --- In-memory fallback methods (used when native service is not wired) ---

func (p *Reminder) addFallback(input map[string]any) (*skill.Result, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	message := input["message"].(string)
	timeStr := input["time"].(string)

	notifTime, err := parsePushTime(timeStr)
	if err != nil {
		return skill.NewErrorResult(err), nil
	}

	p.counter++
	id := fmt.Sprintf("push-%d", p.counter)

	item := &PushItem{
		ID:      id,
		Message: message,
		Time:    notifTime,
		Created: timeutil.NowTime(),
	}
	if recurring, ok := input["recurring"].(string); ok {
		item.Recurring = recurring
	}

	p.notifications[id] = item

	return skill.NewResult(map[string]any{
		"notification": item,
		"message":      fmt.Sprintf("%s — %s (in-memory only, will not persist)", message, notifTime.Format("2006-01-02 15:04")),
	}), nil
}

func (p *Reminder) listFallback() (*skill.Result, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	items := make([]*PushItem, 0, len(p.notifications))
	for _, item := range p.notifications {
		items = append(items, item)
	}

	return skill.NewResult(map[string]any{
		"notifications": items,
		"count":         len(items),
	}), nil
}

func (p *Reminder) deleteFallback(input map[string]any) (*skill.Result, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	id := input["id"].(string)
	if item, ok := p.notifications[id]; ok {
		delete(p.notifications, id)
		return skill.NewResult(map[string]any{
			"deleted":      true,
			"notification": item,
			"message":      fmt.Sprintf("Notification '%s' deleted", id),
		}), nil
	}

	return skill.NewResult(map[string]any{
		"deleted": false,
		"message": fmt.Sprintf("Notification '%s' not found", id),
	}), nil
}

func (p *Reminder) clearFallback() (*skill.Result, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	count := len(p.notifications)
	p.notifications = make(map[string]*PushItem)

	return skill.NewResult(map[string]any{
		"cleared": count,
		"message": fmt.Sprintf("Cleared %d notifications", count),
	}), nil
}
