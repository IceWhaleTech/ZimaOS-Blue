package builtin

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
)

// AutoreplyServiceInterface defines the interface for autoreply service used by the skill.
type AutoreplyServiceInterface interface {
	Create(name string, triggerType string, triggerValue string, responses []string, priority int) (AutoreplyRuleInfo, error)
	List() []AutoreplyRuleInfo
	Delete(id string) error
	Enable(id string) error
	Disable(id string) error
	Test(message, channel string) (string, string, error) // response, ruleID, error
}

// AutoreplyRuleInfo represents an autoreply rule returned by the interface.
type AutoreplyRuleInfo struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	TriggerType  string   `json:"trigger_type"`
	TriggerValue string   `json:"trigger_value"`
	Responses    []string `json:"responses"`
	Priority     int      `json:"priority"`
	Enabled      bool     `json:"enabled"`
	MatchCount   int64    `json:"match_count"`
}

// AutoReply is a built-in skill for managing auto-reply rules.
type AutoReply struct {
	manifest *skill.Manifest
	mu       sync.RWMutex
	svc      AutoreplyServiceInterface
}

// NewAutoReply creates a new autoreply skill.
func NewAutoReply() *AutoReply {
	return &AutoReply{
		manifest: &skill.Manifest{
			ID:          "autoreply",
			Name:        "Auto Reply",
			Version:     "1.0.0",
			Description: "Create and manage auto-reply rules. When a message matches a trigger (keyword, regex, contains, prefix, suffix), an automatic response is sent. Supports template variables like {{user}}, {{message}}, {{time}}.",
			Category:    "system",
			Icon:        "autoreply",
			Tags:        []string{"autoreply", "automation", "trigger", "response", "rule"},
			Inputs: []skill.Parameter{
				{
					Name:        "action",
					Type:        "string",
					Description: "Action: create, list, delete, enable, disable, test",
					Required:    true,
				},
				{
					Name:        "name",
					Type:        "string",
					Description: "Rule name (required for create)",
					Required:    false,
				},
				{
					Name:        "trigger_type",
					Type:        "string",
					Description: "Trigger type: keyword (exact match), contains, prefix, suffix, regex (required for create)",
					Required:    false,
				},
				{
					Name:        "trigger_value",
					Type:        "string",
					Description: "Trigger value — the text or regex pattern to match (required for create)",
					Required:    false,
				},
				{
					Name:        "responses",
					Type:        "array",
					Description: "Response messages (one picked randomly). Supports {{user}}, {{message}}, {{time}}, {{date}}, {{match}} templates. (required for create)",
					Required:    false,
				},
				{
					Name:        "priority",
					Type:        "number",
					Description: "Rule priority — higher values are checked first (default: 0)",
					Required:    false,
				},
				{
					Name:        "id",
					Type:        "string",
					Description: "Rule ID (required for delete, enable, disable)",
					Required:    false,
				},
				{
					Name:        "message",
					Type:        "string",
					Description: "Test message to check against rules (required for test)",
					Required:    false,
				},
			},
			Outputs: []skill.Parameter{
				{
					Name:        "rule",
					Type:        "object",
					Description: "Created/matched rule",
				},
				{
					Name:        "rules",
					Type:        "array",
					Description: "List of rules",
				},
			},
		},
	}
}

// SetAutoreplyService injects the autoreply service.
func (a *AutoReply) SetAutoreplyService(svc AutoreplyServiceInterface) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.svc = svc
}

func (a *AutoReply) Manifest() *skill.Manifest { return a.manifest }

func (a *AutoReply) Validate(input map[string]any) error {
	action, ok := input["action"]
	if !ok {
		return fmt.Errorf("action is required")
	}
	actionStr, ok := action.(string)
	if !ok {
		return fmt.Errorf("action must be a string")
	}
	valid := map[string]bool{
		"create": true, "list": true, "delete": true,
		"enable": true, "disable": true, "test": true,
	}
	if !valid[actionStr] {
		return fmt.Errorf("invalid action: %s", actionStr)
	}
	switch actionStr {
	case "create":
		for _, field := range []string{"name", "trigger_type", "trigger_value", "responses"} {
			if _, ok := input[field]; !ok {
				return fmt.Errorf("%s is required for create", field)
			}
		}
	case "delete", "enable", "disable":
		if _, ok := input["id"]; !ok {
			return fmt.Errorf("id is required for %s", actionStr)
		}
	case "test":
		if _, ok := input["message"]; !ok {
			return fmt.Errorf("message is required for test")
		}
	}
	return nil
}

func (a *AutoReply) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	a.mu.RLock()
	svc := a.svc
	a.mu.RUnlock()

	if svc == nil {
		return skill.NewErrorResult(fmt.Errorf("autoreply service not available")), nil
	}

	action := input["action"].(string)

	switch action {
	case "create":
		name := input["name"].(string)
		triggerType := input["trigger_type"].(string)
		triggerValue := input["trigger_value"].(string)

		// Extract responses — handle both []string and []interface{}
		var responses []string
		switch r := input["responses"].(type) {
		case []string:
			responses = r
		case []interface{}:
			for _, v := range r {
				if s, ok := v.(string); ok {
					responses = append(responses, s)
				}
			}
		}
		if len(responses) == 0 {
			return skill.NewErrorResult(fmt.Errorf("at least one response is required")), nil
		}

		priority := 0
		if p, ok := input["priority"].(float64); ok {
			priority = int(p)
		}

		rule, err := svc.Create(name, triggerType, triggerValue, responses, priority)
		if err != nil {
			return skill.NewErrorResult(err), nil
		}
		return skill.NewResult(map[string]any{
			"rule":    rule,
			"message": fmt.Sprintf("Auto-reply rule '%s' created: %s '%s' → %d responses", name, triggerType, triggerValue, len(responses)),
		}), nil

	case "list":
		rules := svc.List()
		if len(rules) == 0 {
			return skill.NewResult(map[string]any{"rules": []AutoreplyRuleInfo{}, "count": 0, "message": "No auto-reply rules"}), nil
		}
		var lines []string
		for _, r := range rules {
			status := "enabled"
			if !r.Enabled {
				status = "disabled"
			}
			lines = append(lines, fmt.Sprintf("- %s (%s): %s '%s' [%s] matched:%d",
				r.Name, r.ID, r.TriggerType, r.TriggerValue, status, r.MatchCount))
		}
		return skill.NewResult(map[string]any{
			"rules": rules, "count": len(rules),
			"message": fmt.Sprintf("%d rules:\n%s", len(rules), strings.Join(lines, "\n")),
		}), nil

	case "delete":
		id := input["id"].(string)
		if err := svc.Delete(id); err != nil {
			return skill.NewErrorResult(err), nil
		}
		return skill.NewResult(map[string]any{"id": id, "deleted": true, "message": fmt.Sprintf("Rule %s deleted", id)}), nil

	case "enable":
		id := input["id"].(string)
		if err := svc.Enable(id); err != nil {
			return skill.NewErrorResult(err), nil
		}
		return skill.NewResult(map[string]any{"id": id, "enabled": true, "message": fmt.Sprintf("Rule %s enabled", id)}), nil

	case "disable":
		id := input["id"].(string)
		if err := svc.Disable(id); err != nil {
			return skill.NewErrorResult(err), nil
		}
		return skill.NewResult(map[string]any{"id": id, "disabled": true, "message": fmt.Sprintf("Rule %s disabled", id)}), nil

	case "test":
		msg := input["message"].(string)
		channel := ""
		if ch, ok := input["channel"].(string); ok {
			channel = ch
		}
		response, ruleID, err := svc.Test(msg, channel)
		if err != nil {
			return skill.NewErrorResult(err), nil
		}
		if response == "" {
			return skill.NewResult(map[string]any{"matched": false, "message": "No rule matched"}), nil
		}
		return skill.NewResult(map[string]any{
			"matched":  true,
			"rule_id":  ruleID,
			"response": response,
			"message":  fmt.Sprintf("Matched rule %s → %s", ruleID, response),
		}), nil
	}

	return skill.NewErrorResult(fmt.Errorf("unknown action: %s", action)), nil
}
