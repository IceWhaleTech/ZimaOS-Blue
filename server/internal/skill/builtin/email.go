package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
)

// EmailExecutor is the interface for the native email backend.
type EmailExecutor interface {
	Execute(ctx context.Context, args map[string]interface{}) (interface{}, error)
}

// Email is a built-in skill wrapper over the native email tool.
type Email struct {
	manifest *skill.Manifest
	mu       sync.RWMutex
	executor EmailExecutor
}

// NewEmail creates a new email skill.
func NewEmail() *Email {
	return &Email{
		manifest: &skill.Manifest{
			ID:          "email",
			Name:        "Email",
			Version:     "1.0.0",
			Description: "Inbox triage skill. Lists, searches, archives, labels, and summarizes emails from the configured email backend.",
			Category:    "system",
			Icon:        "email",
			Tags:        []string{"email", "mail", "inbox", "triage"},
			Inputs: []skill.Parameter{
				{Name: "action", Type: "string", Description: "list, get, search, filter, archive, label, summarize"},
				{Name: "id", Type: "string", Description: "Message ID for get/archive/label"},
				{Name: "query", Type: "string", Description: "Search query"},
				{Name: "from", Type: "string", Description: "Sender name or email"},
				{Name: "label", Type: "string", Description: "Label filter or single label to add"},
				{Name: "priority", Type: "string", Description: "Priority filter"},
				{Name: "unread", Type: "boolean", Description: "Unread-only filter"},
				{Name: "archived", Type: "boolean", Description: "Archived filter or archive target"},
				{Name: "limit", Type: "integer", Description: "Maximum results to return"},
			},
			Outputs: []skill.Parameter{
				{Name: "email", Type: "object", Description: "Single email result"},
				{Name: "emails", Type: "array", Description: "Email result list"},
				{Name: "summary", Type: "string", Description: "Human-readable inbox summary"},
			},
		},
	}
}

// SetExecutor injects the native email backend.
func (e *Email) SetExecutor(exec EmailExecutor) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.executor = exec
}

func (e *Email) Manifest() *skill.Manifest { return e.manifest }

func (e *Email) Validate(input map[string]any) error {
	normalizeEmailSkillInput(input)

	if input == nil {
		return fmt.Errorf("input is required")
	}
	action, _ := input["action"].(string)
	switch action {
	case "get", "archive", "label":
		if _, ok := input["id"]; !ok {
			return fmt.Errorf("id is required for %s", action)
		}
	}
	return nil
}

func normalizeEmailSkillInput(input map[string]any) {
	normalizeStringAlias(input, "action", "op", "operation", "command")
	normalizeStringAlias(input, "id", "email_id", "emailId", "message_id", "messageId")
	normalizeStringAlias(input, "query", "q", "search")
	normalizeStringAlias(input, "from", "sender")
	normalizeStringAlias(input, "label", "tag")

	action := normalizeEmailSkillAction(firstTrimmedStringValue(input, "action"), input)
	if action != "" {
		input["action"] = action
	}
}

func normalizeEmailSkillAction(raw string, input map[string]any) string {
	action := strings.ToLower(strings.TrimSpace(raw))
	switch action {
	case "open", "read", "show", "detail":
		return "get"
	case "find":
		return "search"
	case "tag", "add_label", "add_labels", "update_labels":
		return "label"
	case "summary", "digest":
		return "summarize"
	case "", "list", "search", "filter", "get", "archive", "label", "summarize":
	default:
		return action
	}
	if action != "" {
		return action
	}
	if firstTrimmedStringValue(input, "id") != "" {
		return "get"
	}
	if firstTrimmedStringValue(input, "query", "from", "label", "priority") != "" {
		return "search"
	}
	return "list"
}

func (e *Email) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	if err := e.Validate(input); err != nil {
		return skill.NewErrorResult(err), nil
	}

	e.mu.RLock()
	exec := e.executor
	e.mu.RUnlock()
	if exec == nil {
		return skill.NewErrorResult(fmt.Errorf("email backend not available")), nil
	}

	args := make(map[string]interface{}, len(input))
	for k, v := range input {
		args[k] = v
	}
	result, err := exec.Execute(ctx, args)
	if err != nil {
		return skill.NewErrorResult(err), nil
	}
	switch typed := result.(type) {
	case string:
		var parsed map[string]any
		if json.Unmarshal([]byte(typed), &parsed) == nil {
			return skill.NewResult(parsed), nil
		}
		return skill.NewResult(map[string]any{"result": typed}), nil
	case map[string]interface{}:
		out := make(map[string]any, len(typed))
		for k, v := range typed {
			out[k] = v
		}
		return skill.NewResult(out), nil
	default:
		b, _ := json.Marshal(typed)
		return skill.NewResult(map[string]any{"result": string(b)}), nil
	}
}
