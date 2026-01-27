package builtin

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/skill"
)

// TaskItem represents a single task
type TaskItem struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Status      string    `json:"status"` // pending, in_progress, completed, cancelled
	Priority    string    `json:"priority,omitempty"` // low, medium, high
	DueDate     string    `json:"due_date,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	Created     time.Time `json:"created"`
	Updated     time.Time `json:"updated"`
	Completed   time.Time `json:"completed,omitempty"`
}

// Tasks is a built-in task management skill
type Tasks struct {
	manifest *skill.Manifest
	tasks    map[string]*TaskItem
	mu       sync.RWMutex
	counter  int
}

// NewTasks creates a new tasks skill
func NewTasks() *Tasks {
	return &Tasks{
		manifest: &skill.Manifest{
			ID:          "tasks",
			Name:        "Tasks",
			Version:     "1.0.0",
			Description: "Task and todo management",
			Category:    "productivity",
			Tags:        []string{"tasks", "todo", "productivity", "project"},
			Inputs: []skill.Parameter{
				{
					Name:        "action",
					Type:        "string",
					Description: "Action: create, read, update, delete, list, complete, reopen",
					Required:    true,
				},
				{
					Name:        "id",
					Type:        "string",
					Description: "Task ID",
					Required:    false,
				},
				{
					Name:        "title",
					Type:        "string",
					Description: "Task title",
					Required:    false,
				},
				{
					Name:        "description",
					Type:        "string",
					Description: "Task description",
					Required:    false,
				},
				{
					Name:        "priority",
					Type:        "string",
					Description: "Priority: low, medium, high",
					Required:    false,
				},
				{
					Name:        "due_date",
					Type:        "string",
					Description: "Due date (RFC3339 format)",
					Required:    false,
				},
				{
					Name:        "tags",
					Type:        "array",
					Description: "Task tags",
					Required:    false,
				},
				{
					Name:        "status",
					Type:        "string",
					Description: "Filter by status (for list action)",
					Required:    false,
				},
			},
			Outputs: []skill.Parameter{
				{
					Name:        "task",
					Type:        "object",
					Description: "Task object",
				},
				{
					Name:        "tasks",
					Type:        "array",
					Description: "List of tasks",
				},
			},
		},
		tasks: make(map[string]*TaskItem),
	}
}

// Manifest returns the skill manifest
func (t *Tasks) Manifest() *skill.Manifest {
	return t.manifest
}

// Validate validates the input parameters
func (t *Tasks) Validate(input map[string]any) error {
	action, ok := input["action"]
	if !ok {
		return fmt.Errorf("action is required")
	}

	actionStr, ok := action.(string)
	if !ok {
		return fmt.Errorf("action must be a string")
	}

	validActions := map[string]bool{
		"create": true, "read": true, "update": true,
		"delete": true, "list": true, "complete": true, "reopen": true,
	}
	if !validActions[actionStr] {
		return fmt.Errorf("invalid action: %s", actionStr)
	}

	switch actionStr {
	case "create":
		if _, ok := input["title"]; !ok {
			return fmt.Errorf("title is required for create action")
		}
	case "read", "delete", "complete", "reopen":
		if _, ok := input["id"]; !ok {
			return fmt.Errorf("id is required for %s action", actionStr)
		}
	case "update":
		if _, ok := input["id"]; !ok {
			return fmt.Errorf("id is required for update action")
		}
	}

	return nil
}

// Execute executes the tasks skill
func (t *Tasks) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	action := input["action"].(string)

	switch action {
	case "create":
		return t.createTask(input)
	case "read":
		return t.readTask(input)
	case "update":
		return t.updateTask(input)
	case "delete":
		return t.deleteTask(input)
	case "list":
		return t.listTasks(input)
	case "complete":
		return t.completeTask(input)
	case "reopen":
		return t.reopenTask(input)
	}

	return skill.NewErrorResult(fmt.Errorf("unknown action: %s", action)), nil
}

func (t *Tasks) createTask(input map[string]any) (*skill.Result, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.counter++
	id := fmt.Sprintf("task-%d", t.counter)
	now := time.Now()

	task := &TaskItem{
		ID:      id,
		Title:   input["title"].(string),
		Status:  "pending",
		Created: now,
		Updated: now,
	}

	if desc, ok := input["description"].(string); ok {
		task.Description = desc
	}
	if priority, ok := input["priority"].(string); ok {
		task.Priority = priority
	} else {
		task.Priority = "medium"
	}
	if dueDate, ok := input["due_date"].(string); ok {
		task.DueDate = dueDate
	}
	if tags, ok := input["tags"].([]interface{}); ok {
		for _, tag := range tags {
			if s, ok := tag.(string); ok {
				task.Tags = append(task.Tags, s)
			}
		}
	}

	t.tasks[id] = task

	return skill.NewResult(map[string]any{
		"task":    task,
		"message": fmt.Sprintf("Task '%s' created", task.Title),
	}), nil
}

func (t *Tasks) readTask(input map[string]any) (*skill.Result, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	id := input["id"].(string)

	if task, ok := t.tasks[id]; ok {
		return skill.NewResult(map[string]any{
			"task": task,
		}), nil
	}

	return skill.NewResult(map[string]any{
		"task":    nil,
		"message": fmt.Sprintf("Task '%s' not found", id),
	}), nil
}

func (t *Tasks) updateTask(input map[string]any) (*skill.Result, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	id := input["id"].(string)

	task, ok := t.tasks[id]
	if !ok {
		return skill.NewResult(map[string]any{
			"updated": false,
			"message": fmt.Sprintf("Task '%s' not found", id),
		}), nil
	}

	if title, ok := input["title"].(string); ok {
		task.Title = title
	}
	if desc, ok := input["description"].(string); ok {
		task.Description = desc
	}
	if priority, ok := input["priority"].(string); ok {
		task.Priority = priority
	}
	if dueDate, ok := input["due_date"].(string); ok {
		task.DueDate = dueDate
	}
	if tags, ok := input["tags"].([]interface{}); ok {
		task.Tags = nil
		for _, tag := range tags {
			if s, ok := tag.(string); ok {
				task.Tags = append(task.Tags, s)
			}
		}
	}
	task.Updated = time.Now()

	return skill.NewResult(map[string]any{
		"updated": true,
		"task":    task,
		"message": fmt.Sprintf("Task '%s' updated", id),
	}), nil
}

func (t *Tasks) deleteTask(input map[string]any) (*skill.Result, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	id := input["id"].(string)

	if task, ok := t.tasks[id]; ok {
		delete(t.tasks, id)
		return skill.NewResult(map[string]any{
			"deleted": true,
			"task":    task,
			"message": fmt.Sprintf("Task '%s' deleted", id),
		}), nil
	}

	return skill.NewResult(map[string]any{
		"deleted": false,
		"message": fmt.Sprintf("Task '%s' not found", id),
	}), nil
}

func (t *Tasks) listTasks(input map[string]any) (*skill.Result, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	statusFilter := ""
	if status, ok := input["status"].(string); ok {
		statusFilter = status
	}

	var tasks []*TaskItem
	for _, task := range t.tasks {
		if statusFilter == "" || task.Status == statusFilter {
			tasks = append(tasks, task)
		}
	}

	return skill.NewResult(map[string]any{
		"tasks": tasks,
		"count": len(tasks),
	}), nil
}

func (t *Tasks) completeTask(input map[string]any) (*skill.Result, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	id := input["id"].(string)

	task, ok := t.tasks[id]
	if !ok {
		return skill.NewResult(map[string]any{
			"completed": false,
			"message":   fmt.Sprintf("Task '%s' not found", id),
		}), nil
	}

	now := time.Now()
	task.Status = "completed"
	task.Completed = now
	task.Updated = now

	return skill.NewResult(map[string]any{
		"completed": true,
		"task":      task,
		"message":   fmt.Sprintf("Task '%s' completed", task.Title),
	}), nil
}

func (t *Tasks) reopenTask(input map[string]any) (*skill.Result, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	id := input["id"].(string)

	task, ok := t.tasks[id]
	if !ok {
		return skill.NewResult(map[string]any{
			"reopened": false,
			"message":  fmt.Sprintf("Task '%s' not found", id),
		}), nil
	}

	task.Status = "pending"
	task.Completed = time.Time{}
	task.Updated = time.Now()

	return skill.NewResult(map[string]any{
		"reopened": true,
		"task":     task,
		"message":  fmt.Sprintf("Task '%s' reopened", task.Title),
	}), nil
}
