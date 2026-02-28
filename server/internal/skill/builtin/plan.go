package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
)

type planTask struct {
	Title   string `json:"title"`
	Checked bool   `json:"checked"`
}

type planState struct {
	Tasks []planTask
}

var globalPlanStore = struct {
	mu    sync.RWMutex
	plans map[string]*planState
}{
	plans: make(map[string]*planState),
}

func planScopeKey(ctx context.Context, input map[string]any) string {
	scope := strings.TrimSpace(stringInput(input, "scope"))
	if scope == "" {
		scope = "default"
	}
	userID := strings.TrimSpace(skill.GetUserID(ctx))
	if userID == "" {
		userID = "default"
	}
	return userID + "::" + scope
}

func extractTasks(input map[string]any) []string {
	if v, ok := input["tasks"]; ok {
		switch t := v.(type) {
		case []any:
			out := make([]string, 0, len(t))
			for _, item := range t {
				s := normalizeTaskText(fmt.Sprintf("%v", item))
				if s != "" {
					out = append(out, s)
				}
			}
			return out
		case []string:
			out := make([]string, 0, len(t))
			for _, item := range t {
				s := normalizeTaskText(item)
				if s != "" {
					out = append(out, s)
				}
			}
			return out
		default:
			if parsed := parseTaskListString(fmt.Sprintf("%v", t)); len(parsed) > 0 {
				return parsed
			}
		}
	}
	if single := normalizeTaskText(stringInput(input, "task")); single != "" {
		return []string{single}
	}
	if query := strings.TrimSpace(stringInput(input, "query")); query != "" {
		if parsed := parseTaskListString(query); len(parsed) > 0 {
			return parsed
		}
	}
	return nil
}

func parseTaskListString(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	// Accept JSON array string payloads from key=value based invocations.
	if strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]") {
		var arr []any
		if err := json.Unmarshal([]byte(raw), &arr); err == nil {
			out := make([]string, 0, len(arr))
			for _, item := range arr {
				s := normalizeTaskText(fmt.Sprintf("%v", item))
				if s != "" {
					out = append(out, s)
				}
			}
			if len(out) > 0 {
				return out
			}
		}
	}

	if strings.Contains(raw, "\n") {
		lines := strings.Split(raw, "\n")
		out := make([]string, 0, len(lines))
		for _, line := range lines {
			if s := normalizeTaskText(line); s != "" {
				out = append(out, s)
			}
		}
		if len(out) > 0 {
			return out
		}
	}

	if strings.ContainsAny(raw, ",;|，") {
		fields := strings.FieldsFunc(raw, func(r rune) bool {
			return r == ',' || r == ';' || r == '|' || r == '，'
		})
		out := make([]string, 0, len(fields))
		for _, f := range fields {
			if s := normalizeTaskText(f); s != "" {
				out = append(out, s)
			}
		}
		if len(out) > 1 {
			return out
		}
	}

	if one := normalizeTaskText(raw); one != "" {
		return []string{one}
	}
	return nil
}

func normalizeTaskText(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	for _, prefix := range []string{
		"- [ ]", "- [x]", "- [X]",
		"* [ ]", "* [x]", "* [X]",
		"-", "*",
	} {
		if strings.HasPrefix(s, prefix) {
			s = strings.TrimSpace(strings.TrimPrefix(s, prefix))
			break
		}
	}

	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i > 0 && i < len(s) && (s[i] == '.' || s[i] == ')') {
		s = strings.TrimSpace(s[i+1:])
	}

	return strings.TrimSpace(s)
}

func buildChecklist(tasks []planTask) string {
	if len(tasks) == 0 {
		return ""
	}
	lines := make([]string, 0, len(tasks))
	for _, t := range tasks {
		box := " "
		if t.Checked {
			box = "x"
		}
		lines = append(lines, fmt.Sprintf("- [%s] %s", box, t.Title))
	}
	return strings.Join(lines, "\n")
}

func stringInput(input map[string]any, key string) string {
	if v, ok := input[key]; ok && v != nil {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func parseTaskIndex(input map[string]any) (int, error) {
	raw := strings.TrimSpace(stringInput(input, "task_index"))
	if raw == "" {
		raw = strings.TrimSpace(stringInput(input, "index"))
	}
	if raw == "" {
		return 0, fmt.Errorf("task_index is required")
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("task_index must be an integer")
	}
	if n <= 0 {
		return 0, fmt.Errorf("task_index must be >= 1")
	}
	return n - 1, nil
}

func parseChecked(input map[string]any) (bool, error) {
	raw := strings.TrimSpace(strings.ToLower(stringInput(input, "checked")))
	if raw == "" {
		raw = strings.TrimSpace(strings.ToLower(stringInput(input, "done")))
	}
	switch raw {
	case "true", "1", "yes", "y":
		return true, nil
	case "false", "0", "no", "n":
		return false, nil
	default:
		return false, fmt.Errorf("checked must be boolean")
	}
}

type PlanCreate struct {
	manifest *skill.Manifest
}

func NewPlanCreate() *PlanCreate {
	return &PlanCreate{
		manifest: &skill.Manifest{
			ID:          "plan_create",
			Name:        "Plan Create",
			Version:     "1.0.0",
			Description: "Create a TODO checklist plan once per scope. Fails if a plan already exists.",
			Category:    "system",
			Icon:        "checklist",
			Tags:        []string{"plan", "todo", "tasks"},
			Inputs: []skill.Parameter{
				{Name: "tasks", Type: "array", Description: "Task list", Required: true},
				{Name: "scope", Type: "string", Description: "Optional scope key (defaults to session)"},
			},
		},
	}
}

func (p *PlanCreate) Manifest() *skill.Manifest { return p.manifest }

func (p *PlanCreate) Validate(input map[string]any) error {
	if len(extractTasks(input)) == 0 {
		return fmt.Errorf("tasks is required")
	}
	return nil
}

func (p *PlanCreate) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	if err := p.Validate(input); err != nil {
		return skill.NewErrorResult(err), nil
	}

	key := planScopeKey(ctx, input)
	tasksRaw := extractTasks(input)
	tasks := make([]planTask, 0, len(tasksRaw))
	for _, t := range tasksRaw {
		tasks = append(tasks, planTask{Title: t, Checked: false})
	}

	globalPlanStore.mu.Lock()
	defer globalPlanStore.mu.Unlock()
	if existing, ok := globalPlanStore.plans[key]; ok && len(existing.Tasks) > 0 {
		return skill.NewErrorResult(fmt.Errorf("plan already exists; use plan_update or plan_append")), nil
	}
	globalPlanStore.plans[key] = &planState{Tasks: tasks}

	return skill.NewResult(map[string]any{
		"scope":      key,
		"operation":  "create",
		"task_count": len(tasks),
		"checklist":  buildChecklist(tasks),
	}), nil
}

type PlanUpdate struct {
	manifest *skill.Manifest
}

func NewPlanUpdate() *PlanUpdate {
	return &PlanUpdate{
		manifest: &skill.Manifest{
			ID:          "plan_update",
			Name:        "Plan Update",
			Version:     "1.0.0",
			Description: "Update an existing plan task incrementally (e.g. checked/unchecked, rename).",
			Category:    "system",
			Icon:        "checklist",
			Tags:        []string{"plan", "todo", "tasks"},
			Inputs: []skill.Parameter{
				{Name: "task_index", Type: "integer", Description: "1-based task index", Required: true},
				{Name: "checked", Type: "boolean", Description: "Mark task checked/unchecked", Required: true},
				{Name: "title", Type: "string", Description: "Optional new title"},
				{Name: "scope", Type: "string", Description: "Optional scope key (defaults to session)"},
			},
		},
	}
}

func (p *PlanUpdate) Manifest() *skill.Manifest { return p.manifest }

func (p *PlanUpdate) Validate(input map[string]any) error {
	if _, err := parseTaskIndex(input); err != nil {
		return err
	}
	if _, err := parseChecked(input); err != nil {
		return err
	}
	return nil
}

func (p *PlanUpdate) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	if err := p.Validate(input); err != nil {
		return skill.NewErrorResult(err), nil
	}

	key := planScopeKey(ctx, input)
	idx, _ := parseTaskIndex(input)
	checked, _ := parseChecked(input)
	newTitle := strings.TrimSpace(stringInput(input, "title"))

	globalPlanStore.mu.Lock()
	defer globalPlanStore.mu.Unlock()
	st, ok := globalPlanStore.plans[key]
	if !ok || len(st.Tasks) == 0 {
		return skill.NewErrorResult(fmt.Errorf("no plan exists; call plan_create first")), nil
	}
	if idx < 0 || idx >= len(st.Tasks) {
		return skill.NewErrorResult(fmt.Errorf("task_index out of range")), nil
	}

	st.Tasks[idx].Checked = checked
	if newTitle != "" {
		st.Tasks[idx].Title = newTitle
	}

	return skill.NewResult(map[string]any{
		"scope":      key,
		"operation":  "update",
		"task_count": len(st.Tasks),
		"checklist":  buildChecklist(st.Tasks),
	}), nil
}

type PlanAppend struct {
	manifest *skill.Manifest
}

func NewPlanAppend() *PlanAppend {
	return &PlanAppend{
		manifest: &skill.Manifest{
			ID:          "plan_append",
			Name:        "Plan Append",
			Version:     "1.0.0",
			Description: "Append a new task to an existing plan. Never recreates a plan.",
			Category:    "system",
			Icon:        "checklist",
			Tags:        []string{"plan", "todo", "tasks"},
			Inputs: []skill.Parameter{
				{Name: "task", Type: "string", Description: "Task text to append", Required: true},
				{Name: "scope", Type: "string", Description: "Optional scope key (defaults to session)"},
			},
		},
	}
}

func (p *PlanAppend) Manifest() *skill.Manifest { return p.manifest }

func (p *PlanAppend) Validate(input map[string]any) error {
	task := strings.TrimSpace(stringInput(input, "task"))
	if task == "" {
		return fmt.Errorf("task is required")
	}
	return nil
}

func (p *PlanAppend) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	if err := p.Validate(input); err != nil {
		return skill.NewErrorResult(err), nil
	}

	key := planScopeKey(ctx, input)
	task := strings.TrimSpace(stringInput(input, "task"))

	globalPlanStore.mu.Lock()
	defer globalPlanStore.mu.Unlock()
	st, ok := globalPlanStore.plans[key]
	if !ok || len(st.Tasks) == 0 {
		return skill.NewErrorResult(fmt.Errorf("no plan exists; call plan_create first")), nil
	}

	st.Tasks = append(st.Tasks, planTask{Title: task, Checked: false})
	return skill.NewResult(map[string]any{
		"scope":      key,
		"operation":  "append",
		"task_count": len(st.Tasks),
		"checklist":  buildChecklist(st.Tasks),
	}), nil
}
