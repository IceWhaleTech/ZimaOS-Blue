package tools

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const defaultToolTraceStoreLimit = 500
const maxToolTraceSummaryBytes = 2048

// ToolTraceRecord captures one tool execution summary.
type ToolTraceRecord struct {
	ID             string    `json:"id"`
	RunID          string    `json:"run_id,omitempty"`
	StepIndex      int       `json:"step_index,omitempty"`
	CapabilityKind string    `json:"capability_kind,omitempty"`
	RequestedTool  string    `json:"requested_tool"`
	ActualTool     string    `json:"actual_tool"`
	StartedAt      time.Time `json:"started_at"`
	FinishedAt     time.Time `json:"finished_at"`
	DurationMs     int64     `json:"duration_ms"`
	Success        bool      `json:"success"`
	Error          string    `json:"error,omitempty"`
	InputSummary   string    `json:"input_summary,omitempty"`
	OutputSummary  string    `json:"output_summary,omitempty"`
}

// ToolTraceStore keeps recent in-memory tool traces.
type ToolTraceStore struct {
	mu      sync.RWMutex
	limit   int
	records []ToolTraceRecord
}

// NewToolTraceStore creates a bounded in-memory trace store.
func NewToolTraceStore(limit int) *ToolTraceStore {
	if limit <= 0 {
		limit = defaultToolTraceStoreLimit
	}
	return &ToolTraceStore{limit: limit}
}

// Record appends a finished tool trace.
func (s *ToolTraceStore) Record(ctx context.Context, startedAt time.Time, requestedTool, actualTool string, args map[string]interface{}, result interface{}, err error) string {
	if s == nil {
		return ""
	}
	finishedAt := time.Now().UTC()
	if startedAt.IsZero() {
		startedAt = finishedAt
	}
	record := ToolTraceRecord{
		ID:             uuid.NewString(),
		RunID:          GetRunID(ctx),
		StepIndex:      GetRunStep(ctx),
		CapabilityKind: inferToolTraceCapabilityKind(actualTool, args),
		RequestedTool:  strings.TrimSpace(requestedTool),
		ActualTool:     strings.TrimSpace(actualTool),
		StartedAt:      startedAt,
		FinishedAt:     finishedAt,
		DurationMs:     finishedAt.Sub(startedAt).Milliseconds(),
		Success:        err == nil,
		InputSummary:   summarizeToolTraceValue(args),
		OutputSummary:  summarizeToolTraceValue(result),
	}
	if err != nil {
		record.Error = err.Error()
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, record)
	if overflow := len(s.records) - s.limit; overflow > 0 {
		s.records = append([]ToolTraceRecord(nil), s.records[overflow:]...)
	}
	return record.ID
}

func inferToolTraceCapabilityKind(actualTool string, args map[string]interface{}) string {
	name := strings.TrimSpace(strings.ToLower(actualTool))
	switch {
	case strings.HasPrefix(name, "mcp"), strings.HasPrefix(name, "mcp_"):
		return "mcp"
	case name == "exec":
		if cmd, _ := args["command"].(string); strings.HasPrefix(strings.TrimSpace(cmd), "blue ") {
			return "skill"
		}
		return "tool"
	default:
		return "tool"
	}
}

// List returns the most recent traces, newest first.
func (s *ToolTraceStore) List(limit int) []ToolTraceRecord {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > len(s.records) {
		limit = len(s.records)
	}
	out := make([]ToolTraceRecord, 0, limit)
	for i := len(s.records) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, s.records[i])
	}
	return out
}

func summarizeToolTraceValue(value interface{}) string {
	if value == nil {
		return ""
	}
	summary := SafeToolPayloadString(value, maxToolTraceSummaryBytes)
	if len(summary) <= maxToolTraceSummaryBytes {
		return summary
	}
	return summary[:maxToolTraceSummaryBytes] + "…"
}
