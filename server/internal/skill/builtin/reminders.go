package builtin

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
)

// Reminder represents a single reminder
type ReminderItem struct {
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	Time      time.Time `json:"time"`
	Recurring string    `json:"recurring,omitempty"` // daily, weekly, monthly, or empty
	Created   time.Time `json:"created"`
}

// Reminders is a built-in reminders skill
type Reminders struct {
	manifest  *skill.Manifest
	reminders map[string]*ReminderItem
	mu        sync.RWMutex
	counter   int
}

// NewReminders creates a new reminders skill
func NewReminders() *Reminders {
	return &Reminders{
		manifest: &skill.Manifest{
			ID:          "reminders",
			Name:        "Reminders",
			Version:     "1.0.0",
			Description: "Set and manage reminders",
			Category:    "productivity",
			Icon:        "reminders",
			Tags:        []string{"reminder", "alert", "schedule", "productivity"},
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
					Description: "Reminder time in RFC3339 format or relative (e.g., '1h', '30m')",
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
					Description: "Recurring schedule: daily, weekly, monthly",
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

// Manifest returns the skill manifest
func (r *Reminders) Manifest() *skill.Manifest {
	return r.manifest
}

// Validate validates the input parameters
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

// Execute executes the reminders skill
func (r *Reminders) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	action := input["action"].(string)

	switch action {
	case "add":
		return r.addReminder(input)
	case "list":
		return r.listReminders()
	case "delete":
		return r.deleteReminder(input)
	case "clear":
		return r.clearReminders()
	}

	return skill.NewErrorResult(fmt.Errorf("unknown action: %s", action)), nil
}

func (r *Reminders) addReminder(input map[string]any) (*skill.Result, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	message := input["message"].(string)
	timeStr := input["time"].(string)

	var reminderTime time.Time
	var err error

	// Try parsing as duration first
	if duration, dErr := time.ParseDuration(timeStr); dErr == nil {
		reminderTime = time.Now().Add(duration)
	} else {
		// Try parsing as RFC3339
		reminderTime, err = time.Parse(time.RFC3339, timeStr)
		if err != nil {
			return skill.NewErrorResult(fmt.Errorf("invalid time format: %v", err)), nil
		}
	}

	r.counter++
	id := fmt.Sprintf("reminder-%d", r.counter)

	reminder := &ReminderItem{
		ID:      id,
		Message: message,
		Time:    reminderTime,
		Created: time.Now(),
	}

	if recurring, ok := input["recurring"].(string); ok {
		reminder.Recurring = recurring
	}

	r.reminders[id] = reminder

	return skill.NewResult(map[string]any{
		"reminder": reminder,
		"message":  fmt.Sprintf("Reminder '%s' set for %s", message, reminderTime.Format(time.RFC3339)),
	}), nil
}

func (r *Reminders) listReminders() (*skill.Result, error) {
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

func (r *Reminders) deleteReminder(input map[string]any) (*skill.Result, error) {
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

func (r *Reminders) clearReminders() (*skill.Result, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	count := len(r.reminders)
	r.reminders = make(map[string]*ReminderItem)

	return skill.NewResult(map[string]any{
		"cleared": count,
		"message": fmt.Sprintf("Cleared %d reminders", count),
	}), nil
}
