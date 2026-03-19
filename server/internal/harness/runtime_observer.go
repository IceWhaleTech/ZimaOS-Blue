package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

const maxRuntimeObserverPayloadStringBytes = 8 * 1024

type RuntimeObserver struct {
	manager *Controller
}

func NewRuntimeObserver(manager *Controller) *RuntimeObserver {
	if manager == nil {
		return nil
	}
	return &RuntimeObserver{manager: manager}
}

func (o *RuntimeObserver) OnToolRequested(event tools.ToolRuntimeEvent) {
	o.append(event.RunID, RunEvent{
		RunID:          event.RunID,
		Type:           "tool_requested",
		StepIndex:      event.StepIndex,
		ToolName:       strings.TrimSpace(event.ToolName),
		CapabilityKind: strings.TrimSpace(event.CapabilityKind),
		Message:        strings.TrimSpace(event.ToolName),
		PayloadJSON: observerPayloadJSON(map[string]interface{}{
			"tool_call_id":    strings.TrimSpace(event.ToolCallID),
			"tool_name":       strings.TrimSpace(event.ToolName),
			"capability_kind": strings.TrimSpace(event.CapabilityKind),
			"session_id":      strings.TrimSpace(event.SessionID),
			"route_kind":      string(event.RouteKind),
			"provider":        strings.TrimSpace(event.Provider),
			"provider_id":     strings.TrimSpace(event.ProviderID),
			"model":           strings.TrimSpace(event.Model),
			"agent_id":        strings.TrimSpace(event.AgentID),
			"arguments":       tools.SafeToolPayloadValue(event.Arguments, maxRuntimeObserverPayloadStringBytes),
		}),
		CreatedAt: timeutil.NowTime(),
	})
}

func (o *RuntimeObserver) OnToolFinished(event tools.ToolRuntimeEvent) {
	message := strings.TrimSpace(event.ToolName)
	if strings.TrimSpace(event.Error) != "" {
		message = strings.TrimSpace(event.Error)
	}
	o.append(event.RunID, RunEvent{
		RunID:          event.RunID,
		Type:           "tool_finished",
		StepIndex:      event.StepIndex,
		ToolName:       strings.TrimSpace(event.ToolName),
		CapabilityKind: strings.TrimSpace(event.CapabilityKind),
		Message:        message,
		PayloadJSON: observerPayloadJSON(map[string]interface{}{
			"tool_call_id":    strings.TrimSpace(event.ToolCallID),
			"tool_name":       strings.TrimSpace(event.ToolName),
			"capability_kind": strings.TrimSpace(event.CapabilityKind),
			"session_id":      strings.TrimSpace(event.SessionID),
			"route_kind":      string(event.RouteKind),
			"result":          tools.SafeToolPayloadValue(event.Result, maxRuntimeObserverPayloadStringBytes),
			"error":           strings.TrimSpace(event.Error),
		}),
		CreatedAt: timeutil.NowTime(),
	})
}

func (o *RuntimeObserver) OnApprovalRequested(event tools.ApprovalRuntimeEvent) {
	o.append(event.RunID, RunEvent{
		RunID:       event.RunID,
		Type:        "approval_requested",
		StepIndex:   event.StepIndex,
		ToolName:    strings.TrimSpace(event.ToolName),
		Message:     approvalRequestedMessage(event),
		PayloadJSON: observerPayloadJSON(event),
		CreatedAt:   timeutil.NowTime(),
	})
}

func (o *RuntimeObserver) OnApprovalResolved(event tools.ApprovalRuntimeEvent) {
	message := strings.TrimSpace(event.Decision)
	if strings.TrimSpace(event.Error) != "" {
		message = strings.TrimSpace(event.Error)
	}
	if message == "" {
		message = "resolved"
	}
	o.append(event.RunID, RunEvent{
		RunID:       event.RunID,
		Type:        "approval_resolved",
		StepIndex:   event.StepIndex,
		ToolName:    strings.TrimSpace(event.ToolName),
		Message:     message,
		PayloadJSON: observerPayloadJSON(event),
		CreatedAt:   timeutil.NowTime(),
	})
}

func (o *RuntimeObserver) OnQuestionRequested(event tools.QuestionRuntimeEvent) {
	o.append(event.RunID, RunEvent{
		RunID:     event.RunID,
		Type:      "question_requested",
		StepIndex: event.StepIndex,
		Message:   questionRequestedMessage(event),
		PayloadJSON: observerPayloadJSON(map[string]interface{}{
			"id":         event.ID,
			"session_id": strings.TrimSpace(event.SessionID),
			"questions":  tools.SafeToolPayloadValue(event.Questions, maxRuntimeObserverPayloadStringBytes),
			"expires_at": event.ExpiresAt,
			"context":    tools.SafeToolPayloadValue(event.Context, maxRuntimeObserverPayloadStringBytes),
		}),
		CreatedAt: timeutil.NowTime(),
	})
}

func (o *RuntimeObserver) OnQuestionResolved(event tools.QuestionRuntimeEvent) {
	message := "resolved"
	switch {
	case strings.TrimSpace(event.Error) != "":
		message = strings.TrimSpace(event.Error)
	case event.TimedOut:
		message = "timed out"
	case event.Silent:
		message = "resolved silently"
	}
	o.append(event.RunID, RunEvent{
		RunID:     event.RunID,
		Type:      "question_resolved",
		StepIndex: event.StepIndex,
		Message:   message,
		PayloadJSON: observerPayloadJSON(map[string]interface{}{
			"id":         event.ID,
			"session_id": strings.TrimSpace(event.SessionID),
			"answers":    tools.SafeToolPayloadValue(event.Answers, maxRuntimeObserverPayloadStringBytes),
			"silent":     event.Silent,
			"timed_out":  event.TimedOut,
			"error":      strings.TrimSpace(event.Error),
		}),
		CreatedAt: timeutil.NowTime(),
	})
}

func (o *RuntimeObserver) append(runID string, event RunEvent) {
	if o == nil || o.manager == nil || strings.TrimSpace(runID) == "" {
		return
	}
	event.RunID = strings.TrimSpace(runID)
	_ = o.manager.AppendEvent(context.Background(), event)
}

func observerPayloadJSON(payload interface{}) string {
	if payload == nil {
		return ""
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		raw, _ = json.Marshal(map[string]string{
			"error": fmt.Sprintf("failed to marshal observer payload: %v", err),
		})
	}
	return string(raw)
}

func approvalRequestedMessage(event tools.ApprovalRuntimeEvent) string {
	if toolName := strings.TrimSpace(event.ToolName); toolName != "" {
		return toolName
	}
	if kind := strings.TrimSpace(event.Kind); kind != "" {
		return kind
	}
	return "approval requested"
}

func questionRequestedMessage(event tools.QuestionRuntimeEvent) string {
	if len(event.Questions) == 1 {
		return strings.TrimSpace(event.Questions[0].Question)
	}
	if len(event.Questions) > 1 {
		return fmt.Sprintf("%d questions", len(event.Questions))
	}
	return "question requested"
}
