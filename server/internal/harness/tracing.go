package harness

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type RunTrace struct {
	RunID       string          `json:"run_id"`
	RootRunID   string          `json:"root_run_id,omitempty"`
	ParentRunID string          `json:"parent_run_id,omitempty"`
	Kind        RunKind         `json:"kind,omitempty"`
	Status      RunStatus       `json:"status,omitempty"`
	StartedAt   *time.Time      `json:"started_at,omitempty"`
	FinishedAt  *time.Time      `json:"finished_at,omitempty"`
	LatencyMs   int64           `json:"latency_ms,omitempty"`
	Stages      []RunTraceStage `json:"stages,omitempty"`
	Events      []RunTraceEvent `json:"events,omitempty"`
	Artifacts   []ArtifactRef   `json:"artifacts,omitempty"`
}

type RunTraceStage struct {
	Stage     RuntimeStage           `json:"stage"`
	Message   string                 `json:"message,omitempty"`
	Status    string                 `json:"status,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
}

type RunTraceEvent struct {
	Type           string    `json:"type"`
	Message        string    `json:"message,omitempty"`
	StepIndex      int       `json:"step_index,omitempty"`
	ToolName       string    `json:"tool_name,omitempty"`
	CapabilityKind string    `json:"capability_kind,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type RunTraceCollector struct {
	manager *Controller
}

func NewRunTraceCollector(manager *Controller) *RunTraceCollector {
	if manager == nil {
		return nil
	}
	return &RunTraceCollector{manager: manager}
}

func (c *RunTraceCollector) Middleware() ExecutionMiddleware {
	if c == nil {
		return nil
	}
	return ExecutionMiddlewareHooks{
		BeforeStartFunc: func(ctx context.Context, runCtx *RunContext) error {
			c.append(ctx, traceRun(runCtx), "trace_requested", "driver start requested", map[string]interface{}{
				"driver_type": traceDriverType(runCtx),
			})
			return nil
		},
		AfterStartFunc: func(ctx context.Context, runCtx *RunContext) {
			c.append(ctx, traceRun(runCtx), "trace_started", "driver start completed", map[string]interface{}{
				"driver_type": traceDriverType(runCtx),
			})
		},
		OnStartErrorFunc: func(ctx context.Context, runCtx *RunContext, runErr error) {
			payload := map[string]interface{}{
				"driver_type": traceDriverType(runCtx),
			}
			if runErr != nil {
				payload["error"] = strings.TrimSpace(runErr.Error())
			}
			c.append(ctx, traceRun(runCtx), "trace_start_failed", "driver start failed", payload)
		},
	}
}

func (c *RunTraceCollector) Snapshot(ctx context.Context, runID string) (*RunTrace, error) {
	if c == nil || c.manager == nil {
		return nil, fmt.Errorf("run trace collector is not configured")
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, fmt.Errorf("run_id is required")
	}
	run, err := c.manager.GetStored(ctx, runID)
	if err != nil {
		return nil, err
	}
	events, err := c.manager.ListEvents(ctx, runID, 1000)
	if err != nil {
		return nil, err
	}
	artifacts, err := c.manager.ListArtifacts(ctx, runID)
	if err != nil {
		return nil, err
	}

	trace := &RunTrace{
		RunID:       run.ID,
		RootRunID:   run.RootRunID,
		ParentRunID: run.ParentRunID,
		Kind:        run.Kind,
		Status:      run.Status,
		StartedAt:   cloneRunTimePtr(run.StartedAt),
		FinishedAt:  cloneRunTimePtr(run.FinishedAt),
		Artifacts:   append([]ArtifactRef(nil), artifacts...),
	}
	if run.StartedAt != nil && run.FinishedAt != nil && !run.FinishedAt.Before(*run.StartedAt) {
		trace.LatencyMs = run.FinishedAt.Sub(*run.StartedAt).Milliseconds()
	}
	for _, event := range events {
		if event.Type == "stage_changed" {
			payload := decodeJSONMap(event.PayloadJSON)
			stageName, _ := payload["stage"].(string)
			status, _ := payload["status"].(string)
			delete(payload, "stage")
			delete(payload, "status")
			trace.Stages = append(trace.Stages, RunTraceStage{
				Stage:     RuntimeStage(strings.TrimSpace(stageName)),
				Message:   strings.TrimSpace(event.Message),
				Status:    strings.TrimSpace(status),
				Details:   payload,
				CreatedAt: event.CreatedAt,
			})
			continue
		}
		trace.Events = append(trace.Events, RunTraceEvent{
			Type:           strings.TrimSpace(event.Type),
			Message:        strings.TrimSpace(event.Message),
			StepIndex:      event.StepIndex,
			ToolName:       strings.TrimSpace(event.ToolName),
			CapabilityKind: strings.TrimSpace(event.CapabilityKind),
			CreatedAt:      event.CreatedAt,
		})
	}
	return trace, nil
}

func (c *RunTraceCollector) append(ctx context.Context, run *Run, eventType, message string, payload map[string]interface{}) {
	if c == nil || c.manager == nil || run == nil || strings.TrimSpace(run.ID) == "" {
		return
	}
	_ = c.manager.AppendEvent(ctx, RunEvent{
		RunID:       run.ID,
		RootRunID:   run.RootRunID,
		ParentRunID: run.ParentRunID,
		Type:        strings.TrimSpace(eventType),
		Message:     strings.TrimSpace(message),
		PayloadJSON: observerPayloadJSON(payload),
		CreatedAt:   time.Now().UTC(),
	})
}

func traceRun(runCtx *RunContext) *Run {
	if runCtx == nil {
		return nil
	}
	return runCtx.Run
}

func traceDriverType(runCtx *RunContext) string {
	if runCtx == nil || runCtx.Driver == nil {
		return ""
	}
	return fmt.Sprintf("%T", runCtx.Driver)
}
