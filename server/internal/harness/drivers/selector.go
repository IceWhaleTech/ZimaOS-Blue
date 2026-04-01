package drivers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

type selectorDryRunSource interface {
	PreviewSelectorDryRun(ctx context.Context, query string, model string) (map[string]interface{}, error)
}

type SelectorDryRunDriver struct {
	source selectorDryRunSource
}

func NewSelectorDryRunDriver(source selectorDryRunSource) *SelectorDryRunDriver {
	return &SelectorDryRunDriver{source: source}
}

func (d *SelectorDryRunDriver) Kind() harness.RunKind {
	return harness.RunKindAgentTask
}

func (d *SelectorDryRunDriver) Validate(spec harness.RunSpec) error {
	if d == nil || d.source == nil {
		return fmt.Errorf("selector dry-run source is not available")
	}
	if selectorDryRunQuery(spec.Goal, spec.Metadata) == "" {
		return fmt.Errorf("selector dry-run query is required")
	}
	return nil
}

func (d *SelectorDryRunDriver) Start(ctx context.Context, run *harness.Run, env harness.RunEnv) error {
	if d == nil || d.source == nil {
		return fmt.Errorf("selector dry-run source is not available")
	}
	if env.Manager == nil {
		return fmt.Errorf("harness manager is required")
	}
	if run == nil {
		return fmt.Errorf("run is required")
	}

	query := selectorDryRunQuery(run.Goal, run.Metadata)
	if query == "" {
		return fmt.Errorf("selector dry-run query is required")
	}
	model := selectorDryRunModel(run.Model, run.Metadata)
	started := timeutil.NowTime()
	response, err := d.source.PreviewSelectorDryRun(ctx, query, model)
	if err != nil {
		return err
	}
	if err := validateSelectorDryRunResponse(response, run.Metadata); err != nil {
		return err
	}

	raw, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("marshal selector dry-run response: %w", err)
	}
	finished := timeutil.NowTime()

	snapshot := *run
	snapshot.Status = harness.RunStatusCompleted
	snapshot.Result = string(raw)
	snapshot.Error = ""
	snapshot.Progress = 100
	snapshot.UpdatedAt = finished
	if snapshot.StartedAt == nil {
		ts := started
		snapshot.StartedAt = &ts
	}
	snapshot.FinishedAt = &finished
	snapshot.Metadata = mergeRunMetadata(run.Metadata, map[string]interface{}{
		"selector_dry_run_response": response,
		"selector_dry_run_query":    query,
		"selector_dry_run_model":    model,
	})
	if err := env.Manager.SyncSnapshot(ctx, &snapshot); err != nil {
		return err
	}
	return env.Manager.AppendEvent(ctx, harness.RunEvent{
		RunID:       run.ID,
		RootRunID:   run.RootRunID,
		ParentRunID: run.ParentRunID,
		Type:        "selector_dry_run_completed",
		Message:     "selector dry-run completed",
		PayloadJSON: string(raw),
		CreatedAt:   finished,
	})
}

func (d *SelectorDryRunDriver) Cancel(_ context.Context, _ *harness.Run) error {
	return nil
}

func selectorDryRunQuery(goal string, meta map[string]interface{}) string {
	if groupInput := selectorMetadataMap(meta, "group_input"); len(groupInput) > 0 {
		if query := firstMapString(groupInput, "query"); query != "" {
			return query
		}
	}
	if query := firstMapString(meta, "query"); query != "" {
		return query
	}
	return strings.TrimSpace(goal)
}

func selectorDryRunModel(model string, meta map[string]interface{}) string {
	if model = strings.TrimSpace(model); model != "" {
		return model
	}
	if groupInput := selectorMetadataMap(meta, "group_input"); len(groupInput) > 0 {
		if value := firstMapString(groupInput, "model"); value != "" {
			return value
		}
	}
	if defaults := selectorMetadataMap(meta, "request_defaults"); len(defaults) > 0 {
		if value := firstMapString(defaults, "model"); value != "" {
			return value
		}
	}
	return "auto"
}

func validateSelectorDryRunResponse(response map[string]interface{}, meta map[string]interface{}) error {
	for _, field := range selectorMetadataStringSlice(meta, "required_fields") {
		if _, ok := response[field]; !ok {
			return fmt.Errorf("selector dry-run response missing required field %q", field)
		}
	}
	return nil
}

func mergeRunMetadata(base map[string]interface{}, extra map[string]interface{}) map[string]interface{} {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(base)+len(extra))
	for key, value := range base {
		out[key] = value
	}
	for key, value := range extra {
		out[key] = value
	}
	return out
}

func selectorMetadataMap(meta map[string]interface{}, key string) map[string]interface{} {
	if len(meta) == 0 {
		return nil
	}
	value, _ := meta[key].(map[string]interface{})
	return value
}

func selectorMetadataStringSlice(meta map[string]interface{}, key string) []string {
	if len(meta) == 0 {
		return nil
	}
	raw, ok := meta[key]
	if !ok {
		return nil
	}
	switch typed := raw.(type) {
	case []string:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if item = strings.TrimSpace(item); item != "" {
				out = append(out, item)
			}
		}
		return out
	case []interface{}:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if value := strings.TrimSpace(fmt.Sprint(item)); value != "" {
				out = append(out, value)
			}
		}
		return out
	default:
		return nil
	}
}

func firstMapString(meta map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if len(meta) == 0 {
			return ""
		}
		if value, ok := meta[key]; ok {
			if text := strings.TrimSpace(fmt.Sprint(value)); text != "" {
				return text
			}
		}
	}
	return ""
}
