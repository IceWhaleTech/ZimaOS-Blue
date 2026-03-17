package tools

import (
	"context"
	"fmt"
	"strings"
)

// MessageTool is a native compatibility wrapper around reminder/push delivery.
type MessageTool struct {
	svc PushServiceInterface
}

// NewMessageTool creates a new message tool.
func NewMessageTool(svc PushServiceInterface) *MessageTool {
	return &MessageTool{svc: svc}
}

// Definition returns the tool definition.
func (t *MessageTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "message",
		Description: "Manage timed reminders/messages. Supports add/list/delete/clear actions with the same delivery backend as reminder.",
		Icon:        "message",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"add", "list", "delete", "clear"},
					"description": "Action to perform. Defaults to add/delete/list based on provided fields.",
				},
				"message": map[string]interface{}{
					"type":        "string",
					"description": "Reminder message (required for add)",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "Alias for message",
				},
				"time": map[string]interface{}{
					"type":        "string",
					"description": "When to fire: relative duration (1h, 30m) or absolute timestamp",
				},
				"every": map[string]interface{}{
					"type":        "string",
					"description": "Repeat interval for user reminders, e.g. 2m or 1h",
				},
				"until": map[string]interface{}{
					"type":        "string",
					"description": "Optional end time for repeating reminders",
				},
				"id": map[string]interface{}{
					"type":        "string",
					"description": "Reminder ID (required for delete)",
				},
				"recurring": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"", "daily", "weekly", "monthly"},
					"description": "Recurring schedule (optional for add)",
				},
			},
		},
	}
}

// Execute dispatches message actions onto the push/reminder backend.
func (t *MessageTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t == nil || t.svc == nil {
		return nil, fmt.Errorf("push service not available")
	}
	message, timeText := messageFields(args)
	action := strings.ToLower(strings.TrimSpace(firstCompatString(args, "action", "op", "operation", "command")))
	if action == "" {
		switch {
		case firstCompatString(args, "id", "message_id", "reminder_id") != "":
			action = "delete"
		case message != "" || timeText != "":
			action = "add"
		default:
			action = "list"
		}
	}
	pushTool := NewPushTool(t.svc)
	translated := map[string]interface{}{"action": action}
	if message != "" {
		translated["message"] = message
	}
	if timeText != "" {
		translated["time"] = timeText
	}
	if every := firstCompatString(args, "every", "interval"); every != "" {
		translated["every"] = every
	}
	if until := firstCompatString(args, "until", "until_at", "untilAt"); until != "" {
		translated["until"] = until
	}
	if id := firstCompatString(args, "id", "message_id", "reminder_id"); id != "" {
		translated["id"] = id
	}
	if recurring := firstCompatString(args, "recurring", "repeat"); recurring != "" {
		translated["recurring"] = recurring
	}
	return pushTool.Execute(ctx, translated)
}

func messageFields(args map[string]interface{}) (string, string) {
	return firstCompatString(args, "message", "content", "text", "input"), firstCompatString(args, "time", "at", "when", "delay", "in")
}

// RegisterMessageTool registers the message compatibility tool.
func RegisterMessageTool(registry *Registry, svc PushServiceInterface) {
	if registry == nil || svc == nil {
		return
	}
	registry.Register(NewMessageTool(svc))
}
