package tools

import (
	"context"
	"fmt"
	"time"
)

// PushServiceInterface defines the interface for the push notification service.
// This avoids circular imports with the push package.
type PushServiceInterface interface {
	Add(ctx context.Context, ownerID, message string, fireAt time.Time, recurring, sessionID string) (PushResult, error)
	List(ctx context.Context, ownerID string) ([]PushResult, error)
	Delete(ctx context.Context, ownerID, id string) error
	Clear(ctx context.Context, ownerID string) (int64, error)
}

// PushResult is the data returned by the push notification service.
type PushResult struct {
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	FireAt    time.Time `json:"fire_at"`
	Recurring string    `json:"recurring,omitempty"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// PushTool is a native tool for managing push notifications.
type PushTool struct {
	svc PushServiceInterface
}

// NewPushTool creates a new push notification tool.
func NewPushTool(svc PushServiceInterface) *PushTool {
	return &PushTool{svc: svc}
}

// Definition returns the tool definition.
func (t *PushTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name: "push_notification",
		Description: `Send push notifications and manage scheduled alerts. Delivers via SSE, Web Push, and native OS notifications (macOS Notification Center, Linux notify-send, Windows toast). Actions:
- add: Schedule a push notification (requires message + time)
- list: List all scheduled notifications
- delete: Delete a notification by ID
- clear: Delete all notifications`,
		Icon: "notifications",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"add", "list", "delete", "clear"},
					"description": "Action to perform",
				},
				"message": map[string]interface{}{
					"type":        "string",
					"description": "Notification message (required for add)",
				},
				"time": map[string]interface{}{
					"type":        "string",
					"description": "When to fire: relative duration (1h, 30m, 2h30m) or absolute (2026-01-04 09:00) or RFC3339",
				},
				"id": map[string]interface{}{
					"type":        "string",
					"description": "Notification ID (required for delete)",
				},
				"recurring": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"", "daily", "weekly", "monthly"},
					"description": "Recurring schedule (optional for add)",
				},
			},
			"required": []string{"action"},
		},
	}
}

// Execute dispatches to the appropriate action.
func (t *PushTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	action, _ := args["action"].(string)
	if action == "" {
		return nil, fmt.Errorf("action is required")
	}

	if t.svc == nil {
		return nil, fmt.Errorf("push service not available")
	}

	userID := GetUserID(ctx)
	if userID == "" {
		userID = "default"
	}

	switch action {
	case "add":
		return t.executeAdd(ctx, userID, args)
	case "list":
		return t.executeList(ctx, userID)
	case "delete":
		return t.executeDelete(ctx, userID, args)
	case "clear":
		return t.executeClear(ctx, userID)
	default:
		return nil, fmt.Errorf("unknown action: %s", action)
	}
}

func (t *PushTool) executeAdd(ctx context.Context, userID string, args map[string]interface{}) (interface{}, error) {
	message, _ := args["message"].(string)
	if message == "" {
		return nil, fmt.Errorf("message is required for add")
	}
	timeStr, _ := args["time"].(string)
	if timeStr == "" {
		return nil, fmt.Errorf("time is required for add")
	}

	fireAt, err := parsePushTime(timeStr)
	if err != nil {
		return nil, err
	}

	recurring, _ := args["recurring"].(string)

	result, err := t.svc.Add(ctx, userID, message, fireAt, recurring, "")
	if err != nil {
		return nil, fmt.Errorf("failed to add push notification: %w", err)
	}

	return map[string]any{
		"notification": result,
		"message":      fmt.Sprintf("Notification set: %s — %s", message, fireAt.Format("2006-01-02 15:04")),
	}, nil
}

func (t *PushTool) executeList(ctx context.Context, userID string) (interface{}, error) {
	list, err := t.svc.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"notifications": list,
		"count":         len(list),
	}, nil
}

func (t *PushTool) executeDelete(ctx context.Context, userID string, args map[string]interface{}) (interface{}, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return nil, fmt.Errorf("id is required for delete")
	}
	if err := t.svc.Delete(ctx, userID, id); err != nil {
		return nil, err
	}
	return map[string]any{
		"deleted": true,
		"id":      id,
		"message": fmt.Sprintf("Notification '%s' deleted", id),
	}, nil
}

func (t *PushTool) executeClear(ctx context.Context, userID string) (interface{}, error) {
	count, err := t.svc.Clear(ctx, userID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"cleared": count,
		"message": fmt.Sprintf("Cleared %d notifications", count),
	}, nil
}

// parsePushTime parses a time string as duration, RFC3339, or common format.
func parsePushTime(s string) (time.Time, error) {
	if d, err := time.ParseDuration(s); err == nil {
		return time.Now().Add(d), nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04", s, time.Local); err == nil {
		return t, nil
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.Local); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid time format: %s (use duration like '1h30m', RFC3339, or 'YYYY-MM-DD HH:MM')", s)
}

// RegisterPushTool registers the push notification tool with the registry.
func RegisterPushTool(registry *Registry, svc PushServiceInterface) {
	if svc == nil {
		return
	}
	registry.Register(NewPushTool(svc))
}
