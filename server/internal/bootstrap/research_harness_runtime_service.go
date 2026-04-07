package bootstrap

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
)

type harnessResearchRuntimeService struct {
	controller *harness.Controller
	fallback   *deepresearch.Service
}

func newHarnessResearchRuntimeService(controller *harness.Controller, fallback *deepresearch.Service) *harnessResearchRuntimeService {
	if controller == nil {
		return nil
	}
	return &harnessResearchRuntimeService{
		controller: controller,
		fallback:   fallback,
	}
}

func (s *harnessResearchRuntimeService) ListJobsForUser(userID, tenantID string, activeOnly bool) ([]deepresearch.JobSummary, error) {
	if s == nil || s.controller == nil {
		if s != nil && s.fallback != nil {
			return s.fallback.ListJobsForUser(userID, tenantID, activeOnly)
		}
		return nil, deepresearch.ErrJobNotFound
	}
	filter := harness.RunFilter{
		UserID: strings.TrimSpace(userID),
		Kind:   harness.RunKindResearch,
		Limit:  256,
	}
	if activeOnly {
		filter.Statuses = []harness.RunStatus{
			harness.RunStatusPending,
			harness.RunStatusPlanning,
			harness.RunStatusWaitingInput,
			harness.RunStatusExecuting,
			harness.RunStatusVerifying,
		}
	}
	runs, err := s.controller.List(context.Background(), filter)
	if err != nil {
		return nil, err
	}
	if len(runs) == 0 && s.fallback != nil {
		return s.fallback.ListJobsForUser(userID, tenantID, activeOnly)
	}
	out := make([]deepresearch.JobSummary, 0, len(runs))
	for i := range runs {
		if !researchRunVisibleToUser(&runs[i], userID) {
			continue
		}
		out = append(out, researchRunToJobSummary(&runs[i]))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out, nil
}

func (s *harnessResearchRuntimeService) GetJobForUser(id, userID, tenantID string) (*deepresearch.Job, error) {
	if s == nil || s.controller == nil {
		if s != nil && s.fallback != nil {
			return s.fallback.GetJobForUser(id, userID, tenantID)
		}
		return nil, deepresearch.ErrJobNotFound
	}
	run, err := s.controller.Get(context.Background(), strings.TrimSpace(id))
	if err != nil || run == nil {
		if s.fallback != nil {
			return s.fallback.GetJobForUser(id, userID, tenantID)
		}
		return nil, deepresearch.ErrJobNotFound
	}
	if !researchRunVisibleToUser(run, userID) {
		return nil, deepresearch.ErrJobForbidden
	}
	if researchRunFamilyMode(run) == "deep_research" && s.fallback != nil {
		if job, fallbackErr := s.fallback.GetJobForUser(id, userID, tenantID); fallbackErr == nil && job != nil {
			return job, nil
		}
	}
	return researchRunToJob(run), nil
}

func (s *harnessResearchRuntimeService) GetReportForUser(id, userID, tenantID string) (*deepresearch.Report, error) {
	job, err := s.GetJobForUser(id, userID, tenantID)
	if err != nil {
		return nil, err
	}
	if job.Report != nil {
		return job.Report, nil
	}
	if answer := researchJobResultText(job); answer != "" {
		return &deepresearch.Report{Answer: answer}, nil
	}
	return nil, deepresearch.ErrReportNotReady
}

func (s *harnessResearchRuntimeService) CancelJobForUser(id, userID, tenantID string) error {
	if s == nil || s.controller == nil {
		if s != nil && s.fallback != nil {
			return s.fallback.CancelJobForUser(id, userID, tenantID)
		}
		return deepresearch.ErrJobNotFound
	}
	run, err := s.controller.GetStored(context.Background(), strings.TrimSpace(id))
	if err != nil || run == nil {
		if s.fallback != nil {
			return s.fallback.CancelJobForUser(id, userID, tenantID)
		}
		return deepresearch.ErrJobNotFound
	}
	if !researchRunVisibleToUser(run, userID) {
		return deepresearch.ErrJobForbidden
	}
	return s.controller.Cancel(context.Background(), run.ID, "cancelled by user")
}

func (s *harnessResearchRuntimeService) SubscribeForUser(jobID, userID, tenantID string) (<-chan deepresearch.Event, func(), error) {
	if s != nil && s.fallback != nil {
		if events, unsubscribe, err := s.fallback.SubscribeForUser(jobID, userID, tenantID); err == nil {
			return events, unsubscribe, nil
		}
	}
	return nil, nil, deepresearch.ErrJobNotFound
}

func researchRunVisibleToUser(run *harness.Run, userID string) bool {
	if run == nil {
		return false
	}
	normalizedUser := strings.TrimSpace(userID)
	if normalizedUser == "" {
		return true
	}
	return strings.TrimSpace(run.UserID) == normalizedUser
}

func researchRunToJobSummary(run *harness.Run) deepresearch.JobSummary {
	return deepresearch.JobSummary{
		ID:             run.ID,
		JobID:          run.ID,
		Query:          strings.TrimSpace(run.Goal),
		Status:         researchRunToJobStatus(run.Status),
		Stage:          researchRunMetadataString(run, "stage"),
		Progress:       run.Progress,
		Iteration:      researchRunMetadataInt(run, "iteration"),
		LatestAction:   researchRunMetadataString(run, "latest_action"),
		LatestGap:      researchRunMetadataString(run, "latest_gap"),
		ConversationID: strings.TrimSpace(run.ConversationID),
		UpdatedAt:      run.UpdatedAt,
	}
}

func researchRunToJob(run *harness.Run) *deepresearch.Job {
	if run == nil {
		return nil
	}
	job := &deepresearch.Job{
		ID:             run.ID,
		ConversationID: strings.TrimSpace(run.ConversationID),
		UserID:         strings.TrimSpace(run.UserID),
		ProviderID:     strings.TrimSpace(run.ProviderID),
		Query:          strings.TrimSpace(run.Goal),
		Mode:           deepresearch.Mode(researchRunMode(run)),
		Status:         researchRunToJobStatus(run.Status),
		Progress:       run.Progress,
		Stage:          researchRunMetadataString(run, "stage"),
		Iteration:      researchRunMetadataInt(run, "iteration"),
		LatestAction:   researchRunMetadataString(run, "latest_action"),
		LatestGap:      researchRunMetadataString(run, "latest_gap"),
		Error:          strings.TrimSpace(run.Error),
		CreatedAt:      run.CreatedAt,
		UpdatedAt:      run.UpdatedAt,
	}
	if run.FinishedAt != nil {
		finished := *run.FinishedAt
		job.CompletedAt = &finished
	}
	if answer := strings.TrimSpace(run.Result); answer != "" {
		job.Report = &deepresearch.Report{Answer: answer}
	}
	return job
}

func researchRunMode(run *harness.Run) string {
	familyMode := researchRunFamilyMode(run)
	if familyMode == "deep_research" {
		if depth := researchRunMetadataString(run, "research_depth"); depth != "" {
			return depth
		}
	}
	return familyMode
}

func researchRunFamilyMode(run *harness.Run) string {
	mode := researchRunMetadataString(run, "mode")
	switch mode {
	case "analyze", "ui_review", "deep_research":
		return mode
	case "fast", "standard", "deep":
		return "deep_research"
	default:
		return mode
	}
}

func researchRunToJobStatus(status harness.RunStatus) deepresearch.JobStatus {
	switch status {
	case harness.RunStatusExecuting, harness.RunStatusVerifying:
		return deepresearch.JobStatusRunning
	case harness.RunStatusCompleted:
		return deepresearch.JobStatusCompleted
	case harness.RunStatusFailed, harness.RunStatusAborted:
		return deepresearch.JobStatusFailed
	case harness.RunStatusCancelled:
		return deepresearch.JobStatusCancelled
	default:
		return deepresearch.JobStatusPending
	}
}

func researchRunMetadataString(run *harness.Run, key string) string {
	if run == nil || run.Metadata == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(run.Metadata[key]))
}

func researchRunMetadataInt(run *harness.Run, key string) int {
	if run == nil || run.Metadata == nil {
		return 0
	}
	switch value := run.Metadata[key].(type) {
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

func researchJobResultText(job *deepresearch.Job) string {
	if job == nil || job.Report == nil {
		return ""
	}
	return strings.TrimSpace(job.Report.Answer)
}
