package harness

import (
	"fmt"
	"net/url"
	"strings"
)

type ActionDescriptor struct {
	ID            string                 `json:"id"`
	Label         string                 `json:"label"`
	Method        string                 `json:"method"`
	Path          string                 `json:"path"`
	Variant       string                 `json:"variant,omitempty"`
	RequiresInput bool                   `json:"requires_input,omitempty"`
	Input         *ActionInputDescriptor `json:"input,omitempty"`
}

type ActionInputDescriptor struct {
	Title       string                       `json:"title,omitempty"`
	Description string                       `json:"description,omitempty"`
	SubmitLabel string                       `json:"submit_label,omitempty"`
	Fields      []ActionInputFieldDescriptor `json:"fields,omitempty"`
}

type ActionInputFieldDescriptor struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Kind        string   `json:"kind,omitempty"`
	Target      string   `json:"target,omitempty"`
	PayloadKey  string   `json:"payload_key,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Placeholder string   `json:"placeholder,omitempty"`
	Options     []string `json:"options,omitempty"`
}

type RunActionAvailability struct {
	Items []ActionDescriptor `json:"items,omitempty"`
}

func runActionAvailability(run *Run, basePath string) RunActionAvailability {
	return RunActionAvailability{
		Items: runActionDescriptors(run, basePath),
	}
}

func runActionDescriptors(run *Run, basePath string) []ActionDescriptor {
	items := make([]ActionDescriptor, 0, 2)
	if canCancelRun(run) {
		items = append(items, newActionDescriptor("cancel", "Cancel", actionPath(basePath, "cancel"), "danger", false, nil))
	}
	if canResumeRun(run) {
		items = append(items, newActionDescriptor("resume", "Resume", actionPath(basePath, "resume"), "primary", workflowResumeRequiresInput(run), workflowResumeInputDescriptor(run)))
	}
	return items
}

func taskActionDescriptors(run *Run, group *RunGroup, taskID string, scope string) []ActionDescriptor {
	items := make([]ActionDescriptor, 0, 3)
	basePath := taskActionBasePath(taskID)
	if run != nil {
		if canCancelUserTask(run) {
			items = append(items, newActionDescriptor("cancel", "Cancel", actionPath(basePath, "cancel"), "danger", false, nil))
		}
		if canResumeUserTask(run) {
			items = append(items, newActionDescriptor("resume", "Resume", actionPath(basePath, "resume"), "primary", workflowResumeRequiresInput(run), workflowResumeInputDescriptor(run)))
		}
		if scope == "current" && run.Kind == RunKindAgentTask && canSendUpdate(run) {
			items = append(items, newActionDescriptor("send_update", "Send update", agentTaskMessagePath(taskID), "default", true, agentTaskSendUpdateInputDescriptor()))
		}
		return items
	}
	if canCancelUserTaskGroup(group) {
		items = append(items, newActionDescriptor("cancel", "Cancel", actionPath(basePath, "cancel"), "danger", false, nil))
	}
	return items
}

func newActionDescriptor(
	id string,
	label string,
	path string,
	variant string,
	requiresInput bool,
	input *ActionInputDescriptor,
) ActionDescriptor {
	return ActionDescriptor{
		ID:            strings.TrimSpace(id),
		Label:         strings.TrimSpace(label),
		Method:        "POST",
		Path:          strings.TrimSpace(path),
		Variant:       strings.TrimSpace(variant),
		RequiresInput: requiresInput,
		Input:         input,
	}
}

func actionPath(basePath string, action string) string {
	base := strings.TrimRight(strings.TrimSpace(basePath), "/")
	name := strings.TrimSpace(action)
	if base == "" || name == "" {
		return ""
	}
	return base + "/actions/" + url.PathEscape(name)
}

func taskActionBasePath(taskID string) string {
	id := strings.TrimSpace(taskID)
	if id == "" {
		return ""
	}
	return "/tasks/" + url.PathEscape(id)
}

func agentTaskMessagePath(taskID string) string {
	id := strings.TrimSpace(taskID)
	if id == "" {
		return ""
	}
	return "/agent/tasks/" + url.PathEscape(id) + "/message"
}

func canCancelRun(run *Run) bool {
	if run == nil {
		return false
	}
	switch run.Status {
	case RunStatusPending, RunStatusPlanning, RunStatusWaitingInput, RunStatusExecuting, RunStatusVerifying:
		return true
	default:
		return false
	}
}

func canResumeRun(run *Run) bool {
	if run == nil || run.Kind != RunKindWorkflow || run.Status != RunStatusWaitingInput {
		return false
	}
	if runActionMetadataString(run.Metadata, "workflow_execution_id") == "" {
		return false
	}
	return runActionMetadataString(run.Metadata, "workflow_checkpoint_kind") != ""
}

func workflowResumeRequiresInput(run *Run) bool {
	if run == nil || run.Kind != RunKindWorkflow {
		return false
	}
	return runActionMetadataString(run.Metadata, "workflow_checkpoint_kind") != ""
}

func workflowResumeInputDescriptor(run *Run) *ActionInputDescriptor {
	if !workflowResumeRequiresInput(run) {
		return nil
	}
	checkpointKind := strings.ToLower(runActionMetadataString(run.Metadata, "workflow_checkpoint_kind"))
	statusReason := strings.ToLower(runActionMetadataString(run.Metadata, "workflow_status_reason"))
	nodeName := runActionMetadataString(run.Metadata, "workflow_checkpoint_node_name")
	contextSuffix := workflowCheckpointContextSuffix(nodeName)
	switch {
	case strings.Contains(checkpointKind, "approval") || strings.Contains(statusReason, "approval"):
		return &ActionInputDescriptor{
			Title:       "Review approval" + contextSuffix,
			Description: workflowCheckpointDescription("Review the workflow checkpoint and choose whether to continue.", nodeName),
			SubmitLabel: "Submit decision",
			Fields: []ActionInputFieldDescriptor{
				{
					Key:         "decision",
					Label:       "Decision",
					Kind:        "choice",
					Target:      "decision",
					Required:    true,
					Placeholder: "approve",
					Options:     []string{"approve", "reject"},
				},
				{
					Key:         "comment",
					Label:       "Comment",
					Kind:        "textarea",
					Target:      "payload",
					PayloadKey:  "comment",
					Placeholder: "Optional comment",
				},
			},
		}
	case strings.Contains(checkpointKind, "json"):
		return &ActionInputDescriptor{
			Title:       "Provide JSON payload" + contextSuffix,
			Description: workflowCheckpointDescription("Resume the workflow by supplying the requested JSON object.", nodeName),
			SubmitLabel: "Submit payload",
			Fields: []ActionInputFieldDescriptor{
				{
					Key:         "payload",
					Label:       "JSON payload",
					Kind:        "json",
					Target:      "payload_root",
					Required:    true,
					Placeholder: `{"result":"Provide the requested JSON payload"}`,
				},
			},
		}
	case strings.Contains(checkpointKind, "tool") || strings.Contains(statusReason, "tool"):
		return &ActionInputDescriptor{
			Title:       "Provide tool result" + contextSuffix,
			Description: workflowCheckpointDescription("Resume the workflow by supplying the requested tool result payload.", nodeName),
			SubmitLabel: "Submit result",
			Fields: []ActionInputFieldDescriptor{
				{
					Key:         "result",
					Label:       "Tool result",
					Kind:        "json",
					Target:      "payload_root",
					Required:    true,
					Placeholder: `{"result":"Provide the tool result payload"}`,
				},
			},
		}
	case strings.Contains(checkpointKind, "input") || strings.Contains(checkpointKind, "question") || strings.Contains(statusReason, "clarify"):
		return &ActionInputDescriptor{
			Title:       "Provide clarification" + contextSuffix,
			Description: workflowCheckpointDescription("The workflow is waiting for additional input before it can continue.", nodeName),
			SubmitLabel: "Send response",
			Fields: []ActionInputFieldDescriptor{
				{
					Key:         "response",
					Label:       "Response",
					Kind:        "textarea",
					Target:      "payload",
					PayloadKey:  "response",
					Required:    true,
					Placeholder: "Provide the missing detail",
				},
			},
		}
	default:
		return &ActionInputDescriptor{
			Title:       "Resume workflow" + contextSuffix,
			Description: workflowCheckpointDescription("Provide the required input to continue the workflow.", nodeName),
			SubmitLabel: "Resume workflow",
			Fields: []ActionInputFieldDescriptor{
				{
					Key:         "context",
					Label:       "Context",
					Kind:        "textarea",
					Target:      "payload",
					PayloadKey:  "context",
					Required:    true,
					Placeholder: "Additional input",
				},
			},
		}
	}
}

func workflowCheckpointContextSuffix(nodeName string) string {
	name := strings.TrimSpace(nodeName)
	if name == "" {
		return ""
	}
	return " for " + name
}

func workflowCheckpointDescription(base string, nodeName string) string {
	description := strings.TrimSpace(base)
	name := strings.TrimSpace(nodeName)
	if description == "" || name == "" {
		return description
	}
	return description + " Node: " + name + "."
}

func agentTaskSendUpdateInputDescriptor() *ActionInputDescriptor {
	return &ActionInputDescriptor{
		Title:       "Send update",
		Description: "Send a follow-up instruction or clarification to the running task.",
		SubmitLabel: "Send update",
		Fields: []ActionInputFieldDescriptor{
			{
				Key:         "message",
				Label:       "Message",
				Kind:        "textarea",
				Target:      "root",
				Required:    true,
				Placeholder: "Send an update to this task",
			},
		},
	}
}

func runActionMetadataString(meta map[string]interface{}, key string) string {
	if len(meta) == 0 {
		return ""
	}
	raw, ok := meta[key]
	if !ok || raw == nil {
		return ""
	}
	if value, ok := raw.(string); ok {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(fmt.Sprint(raw))
}
