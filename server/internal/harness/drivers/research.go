package drivers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

type ResearchDriver struct {
	service *deepresearch.Service
	manager *harness.Controller
	next    deepresearch.EventPublisher
}

func NewResearchDriver(service *deepresearch.Service, manager *harness.Controller) *ResearchDriver {
	return &ResearchDriver{service: service, manager: manager}
}

func (d *ResearchDriver) Kind() harness.RunKind { return harness.RunKindResearch }

func (d *ResearchDriver) Validate(spec harness.RunSpec) error {
	if d == nil || d.service == nil || d.manager == nil {
		return fmt.Errorf("research runtime is not available")
	}
	if strings.TrimSpace(spec.Goal) == "" {
		return fmt.Errorf("goal is required")
	}
	return nil
}

func (d *ResearchDriver) Start(ctx context.Context, run *harness.Run, _ harness.RunEnv) error {
	if d == nil || d.service == nil {
		return fmt.Errorf("research runtime is not available")
	}
	req := deepresearch.CreateJobRequest{
		RequestedID:    run.ID,
		UserID:         run.UserID,
		ConversationID: run.ConversationID,
		Query:          run.Goal,
		Mode:           deepresearch.Mode(metadataString(run.Metadata, "mode")),
		RouteMode:      deepresearch.RouteMode(metadataString(run.Metadata, "route_mode")),
		Lang:           metadataString(run.Metadata, "lang"),
		ReportStyle:    metadataString(run.Metadata, "report_style"),
		TimeWindows:    metadataStringSlice(run.Metadata["time_windows"]),
	}
	if maxSources := metadataInt(run.Metadata["max_sources"]); maxSources > 0 || run.MaxSteps > 0 || run.MaxDuration > 0 {
		req.Budget = &deepresearch.Budget{
			MaxSteps:   run.MaxSteps,
			MaxSources: maxSources,
			MaxSeconds: maxDurationSeconds(run.MaxDuration, run.Metadata["max_seconds"]),
		}
	}
	if strict, ok := metadataBool(run.Metadata["strict_entity"]); ok {
		req.StrictEntity = &strict
	}
	_, err := d.service.CreateJob(ctx, req)
	return err
}

func (d *ResearchDriver) Cancel(_ context.Context, run *harness.Run) error {
	if d == nil || d.service == nil || run == nil {
		return fmt.Errorf("research runtime is not available")
	}
	if strings.TrimSpace(run.UserID) != "" {
		return d.service.CancelJobForUser(run.ID, run.UserID, "")
	}
	return d.service.CancelJob(run.ID)
}

func (d *ResearchDriver) Sync(ctx context.Context, run *harness.Run) (*harness.Run, error) {
	if d == nil || d.service == nil || run == nil {
		return run, nil
	}
	job, err := d.service.GetJob(run.ID)
	if err != nil {
		return run, nil
	}
	snapshot := jobToRun(run, job)
	if err := d.manager.SyncSnapshot(ctx, snapshot); err != nil {
		return nil, err
	}
	return d.manager.GetStored(ctx, run.ID)
}

func (d *ResearchDriver) SetNextPublisher(next deepresearch.EventPublisher) {
	d.next = next
}

func (d *ResearchDriver) Publish(userID string, eventType string, data any) {
	if d.next != nil {
		d.next.Publish(userID, eventType, data)
	}
	if d == nil || d.manager == nil {
		return
	}
	jobID := extractResearchJobID(data)
	if jobID == "" {
		return
	}
	run, err := d.manager.GetStored(context.Background(), jobID)
	if err == nil {
		if _, syncErr := d.Sync(context.Background(), run); syncErr == nil {
			run, _ = d.manager.GetStored(context.Background(), jobID)
		}
	}
	payloadJSON := ""
	if raw, err := json.Marshal(data); err == nil {
		payloadJSON = string(raw)
	}
	_ = d.manager.AppendEvent(context.Background(), harness.RunEvent{
		RunID:       jobID,
		Type:        mapResearchEventType(eventType),
		Message:     strings.TrimSpace(eventType),
		PayloadJSON: payloadJSON,
		CreatedAt:   timeutil.NowTime(),
	})
	if strings.HasSuffix(strings.TrimSpace(eventType), ".job_completed") {
		d.attachJobArtifacts(context.Background(), jobID)
	}
}

func (d *ResearchDriver) attachJobArtifacts(ctx context.Context, jobID string) {
	if d == nil || d.service == nil || d.manager == nil || strings.TrimSpace(jobID) == "" {
		return
	}
	job, err := d.service.GetJob(jobID)
	if err != nil || job == nil || job.Report == nil || job.Report.Experiment == nil {
		return
	}
	for _, artifact := range job.Report.Experiment.Artifacts {
		pathOrURL := strings.TrimSpace(artifact.Path)
		if pathOrURL == "" {
			pathOrURL = strings.TrimSpace(artifact.URI)
		}
		if pathOrURL == "" {
			continue
		}
		metaJSON := ""
		if raw, err := json.Marshal(map[string]interface{}{"label": artifact.Label, "kind": artifact.Kind}); err == nil {
			metaJSON = string(raw)
		}
		_ = d.manager.AttachArtifact(ctx, harness.ArtifactRef{
			ID:           uuid.NewString(),
			RunID:        jobID,
			Kind:         normalizeArtifactKind(artifact.Kind),
			Label:        strings.TrimSpace(artifact.Label),
			PathOrURL:    pathOrURL,
			MetadataJSON: metaJSON,
		})
	}
}

func normalizeArtifactKind(kind string) string {
	switch strings.TrimSpace(strings.ToLower(kind)) {
	case "directory", "dir":
		return "dir"
	case "url", "uri":
		return "url"
	case "report":
		return "report"
	case "log":
		return "log"
	case "snapshot":
		return "snapshot"
	default:
		return "file"
	}
}

func mapResearchEventType(eventType string) string {
	switch strings.TrimSpace(eventType) {
	case "deep_research.job_created":
		return "run_created"
	case "deep_research.job_updated":
		return "state_changed"
	case "deep_research.job_completed":
		return "run_completed"
	case "deep_research.job_failed":
		return "run_failed"
	case "deep_research.job_cancelled":
		return "run_cancelled"
	default:
		return eventType
	}
}

func extractResearchJobID(data any) string {
	switch payload := data.(type) {
	case map[string]interface{}:
		if v, ok := payload["job_id"].(string); ok {
			return strings.TrimSpace(v)
		}
		if v, ok := payload["id"].(string); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func jobToRun(existing *harness.Run, job *deepresearch.Job) *harness.Run {
	run := &harness.Run{}
	if existing != nil {
		*run = *existing
		run.Metadata = cloneMap(existing.Metadata)
	}
	run.ID = job.ID
	run.Kind = harness.RunKindResearch
	run.UserID = job.UserID
	run.ConversationID = job.ConversationID
	run.SessionID = job.ConversationID
	run.Goal = job.Query
	run.Status = jobStatusToRunStatus(job.Status)
	run.Progress = job.Progress
	run.Result = ""
	if job.Report != nil {
		run.Result = strings.TrimSpace(job.Report.Answer)
	}
	run.Error = strings.TrimSpace(job.Error)
	run.CreatedAt = job.CreatedAt
	run.UpdatedAt = job.UpdatedAt
	if !job.CreatedAt.IsZero() {
		started := job.CreatedAt
		run.StartedAt = &started
	}
	if job.CompletedAt != nil && !job.CompletedAt.IsZero() {
		finished := *job.CompletedAt
		run.FinishedAt = &finished
	}
	return run
}

func jobStatusToRunStatus(status deepresearch.JobStatus) harness.RunStatus {
	switch status {
	case deepresearch.JobStatusRunning, deepresearch.JobStatusSynthesizing:
		return harness.RunStatusExecuting
	case deepresearch.JobStatusCompleted:
		return harness.RunStatusCompleted
	case deepresearch.JobStatusFailed:
		return harness.RunStatusFailed
	case deepresearch.JobStatusCancelled:
		return harness.RunStatusCancelled
	default:
		return harness.RunStatusPending
	}
}

func metadataStringSlice(raw any) []string {
	items, ok := raw.([]interface{})
	if !ok {
		if typed, ok := raw.([]string); ok {
			return append([]string(nil), typed...)
		}
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if value := strings.TrimSpace(fmt.Sprint(item)); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func metadataInt(raw any) int {
	switch value := raw.(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return 0
	}
}

func metadataBool(raw any) (bool, bool) {
	v, ok := raw.(bool)
	return v, ok
}

func maxDurationSeconds(duration time.Duration, fallback any) int {
	if duration > 0 {
		return int(duration.Seconds())
	}
	return metadataInt(fallback)
}
