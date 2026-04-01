package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/remindertime"
)

// PushServiceInterface defines the interface for the reminder delivery service.
// This avoids circular imports with the push package.
type PushServiceInterface interface {
	Add(ctx context.Context, ownerID, message string, fireAt time.Time, recurring, sessionID string, untilAt *time.Time) (PushResult, error)
	List(ctx context.Context, ownerID string) ([]PushResult, error)
	Delete(ctx context.Context, ownerID, id string) error
	Clear(ctx context.Context, ownerID string) (int64, error)
}

// PushResult is the data returned by the reminder delivery service.
type PushResult struct {
	ID        string     `json:"id"`
	Message   string     `json:"message"`
	FireAt    time.Time  `json:"fire_at"`
	Recurring string     `json:"recurring,omitempty"`
	UntilAt   *time.Time `json:"until_at,omitempty"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
}

// PushTool is a native tool for managing reminders.
type PushTool struct {
	svc PushServiceInterface
}

// NewPushTool creates a new reminder tool.
func NewPushTool(svc PushServiceInterface) *PushTool {
	return &PushTool{svc: svc}
}

// Definition returns the tool definition.
func (t *PushTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name: "reminder",
		Description: `Manage reminders and scheduled alerts. Delivers via SSE, Web Push, and native OS alerts (macOS Notification Center, Linux notify-send, Windows toast). Actions:
- add: Schedule a reminder (requires message + time or every)
- list: List all scheduled reminders
- delete: Delete a reminder by ID
- clear: Delete all reminders`,
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
					"description": "Reminder message (required for add)",
				},
				"time": map[string]interface{}{
					"type":        "string",
					"description": "When to fire: relative duration (1h, 30m, 2h30m) or absolute (2026-01-04 09:00) or RFC3339",
				},
				"every": map[string]interface{}{
					"type":        "string",
					"description": "Repeat interval for user reminders, e.g. 2m or 1h30m. Can be combined with time to control the first fire.",
				},
				"until": map[string]interface{}{
					"type":        "string",
					"description": "Optional end time for repeating reminders, e.g. 2026-03-17 22:00 or 2h",
				},
				"id": map[string]interface{}{
					"type":        "string",
					"description": "Reminder ID (required for delete)",
				},
				"recurring": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"", "daily", "weekly", "monthly"},
					"description": "Calendar recurrence for reminders (daily, weekly, monthly). Use every for minute/hour intervals.",
				},
			},
			"required": []string{"action"},
		},
	}
}

// Execute dispatches to the appropriate action.
func (t *PushTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	action := firstCompatString(args, "action", "op", "operation", "command")
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
	message := firstCompatString(args, "message", "content", "text")
	if message == "" {
		return nil, fmt.Errorf("message is required for add")
	}
	timeStr := firstCompatString(args, "time", "fire_at", "fireAt", "when")
	everyStr := firstCompatString(args, "every", "interval")
	recurring := firstCompatString(args, "recurring", "repeat", "recurrence")
	if timeStr == "" && everyStr == "" {
		if inferredTime, ok := remindertime.InferTimeStringFromMessage(message); ok {
			timeStr = inferredTime
		}
	}
	if timeStr == "" && everyStr == "" {
		return nil, fmt.Errorf("time or every is required for add")
	}
	if everyStr != "" && recurring != "" {
		return nil, fmt.Errorf("every cannot be combined with recurring")
	}

	var fireAt time.Time
	var err error
	if timeStr != "" {
		fireAt, err = parsePushTime(timeStr)
		if err != nil {
			return nil, err
		}
	}
	if everyStr != "" {
		interval, intervalErr := remindertime.ParseDuration(everyStr)
		if intervalErr != nil {
			return nil, intervalErr
		}
		if interval < time.Minute {
			return nil, fmt.Errorf("every must be at least 1 minute")
		}
		recurring = "interval:" + interval.String()
		if timeStr == "" {
			fireAt = time.Now().Add(interval)
		}
	}

	var untilAt *time.Time
	if untilStr := firstCompatString(args, "until", "until_at", "untilAt"); untilStr != "" {
		parsedUntil, untilErr := parsePushTime(untilStr)
		if untilErr != nil {
			return nil, untilErr
		}
		untilAt = &parsedUntil
	}
	sessionID := GetSessionID(ctx)

	result, err := t.svc.Add(ctx, userID, message, fireAt, recurring, sessionID, untilAt)
	if err != nil {
		return nil, fmt.Errorf("failed to add reminder: %w", err)
	}

	// Emit a success alert card so the user sees immediate visual confirmation.
	EmitCard(ctx, map[string]interface{}{
		"type":      "alert",
		"icon":      "⏰",
		"variant":   "success",
		"title_key": "push.reminderSet",
		"message":   fmt.Sprintf("%s — %s", message, fireAt.Format("2006-01-02 15:04")),
	})

	return map[string]any{
		"status":   "success",
		"reminder": result,
		"message":  fmt.Sprintf("Reminder set: %s — %s", message, fireAt.Format("2006-01-02 15:04")),
	}, nil
}

func (t *PushTool) executeList(ctx context.Context, userID string) (interface{}, error) {
	list, err := t.svc.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"status":    "success",
		"reminders": list,
		"count":     len(list),
	}, nil
}

func (t *PushTool) executeDelete(ctx context.Context, userID string, args map[string]interface{}) (interface{}, error) {
	id := firstCompatString(args, "id", "reminder_id", "reminderId")
	if id == "" {
		return nil, fmt.Errorf("id is required for delete")
	}
	if err := t.svc.Delete(ctx, userID, id); err != nil {
		return nil, err
	}
	return map[string]any{
		"status":  "success",
		"deleted": true,
		"id":      id,
		"message": fmt.Sprintf("Reminder '%s' deleted", id),
	}, nil
}

func (t *PushTool) executeClear(ctx context.Context, userID string) (interface{}, error) {
	count, err := t.svc.Clear(ctx, userID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"status":  "success",
		"cleared": count,
		"message": fmt.Sprintf("Cleared %d reminders", count),
	}, nil
}

// parsePushTime parses a time string as duration, RFC3339, or common format.
func parsePushTime(s string) (time.Time, error) {
	return remindertime.Parse(s)
}

// RegisterPushTool registers the reminder tool with the registry.
func RegisterPushTool(registry *Registry, svc PushServiceInterface) {
	if svc == nil {
		return
	}
	registry.Register(NewPushTool(svc))
}
