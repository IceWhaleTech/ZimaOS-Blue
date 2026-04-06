package bootstrap

import (
	"context"
	"fmt"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func submitCanonicalResearchHarnessJob(
	ctx context.Context,
	submitter researchHarnessRunSubmitter,
	service researchHarnessJobStore,
	req tools.ResearchCreateJobRequest,
	workspaceRoot string,
) (*deepresearch.Job, *harness.Run, error) {
	submitter = normalizeResearchHarnessSubmitter(submitter)
	if submitter == nil {
		return nil, nil, fmt.Errorf("research harness submitter is required")
	}
	run, err := submitter.Submit(ctx, newResearchHarnessRunSpec(researchHarnessRunInputFromToolRequest(req, workspaceRoot)))
	if err != nil {
		return nil, nil, err
	}
	return loadSubmittedResearchJob(run, service, req.UserID, ""), run, nil
}

func researchHarnessRunInputFromToolRequest(req tools.ResearchCreateJobRequest, workspaceRoot string) researchHarnessRunInput {
	return researchHarnessRunInput{
		Query:          canonicalResearchGoal(req),
		UserID:         req.UserID,
		ConversationID: req.ConversationID,
		WorkspaceRoot:  strings.TrimSpace(workspaceRoot),
		Mode:           strings.TrimSpace(req.Mode),
		ResearchDepth:  strings.TrimSpace(req.ResearchDepth),
		RouteMode:      strings.TrimSpace(req.RouteMode),
		Lang:           strings.TrimSpace(req.Lang),
		ReportStyle:    strings.TrimSpace(req.ReportStyle),
		TimeWindows:    append([]string(nil), req.TimeWindows...),
		StrictEntity:   req.StrictEntity != nil && *req.StrictEntity,
		MaxSources:     toolResearchBudgetMaxSources(req.Budget),
		MaxSeconds:     toolResearchBudgetMaxSeconds(req.Budget),
		Topic:          strings.TrimSpace(req.Topic),
		URLs:           append([]string(nil), req.URLs...),
		Text:           strings.TrimSpace(req.Text),
		SearchQueries:  append([]string(nil), req.SearchQueries...),
		OutputMode:     strings.TrimSpace(req.OutputMode),
		Action:         strings.TrimSpace(req.Action),
		URL:            strings.TrimSpace(req.URL),
		Image:          strings.TrimSpace(req.Image),
		Device:         strings.TrimSpace(req.Device),
		Channel:        strings.TrimSpace(req.Channel),
		WaitMS:         req.WaitMS,
		Threshold:      req.Threshold,
		Format:         strings.TrimSpace(req.Format),
		Profile:        strings.TrimSpace(req.Profile),
	}
}

func canonicalResearchGoal(req tools.ResearchCreateJobRequest) string {
	if query := strings.TrimSpace(req.Query); query != "" {
		return query
	}
	if topic := strings.TrimSpace(req.Topic); topic != "" {
		return topic
	}
	if len(req.URLs) > 0 {
		if first := strings.TrimSpace(req.URLs[0]); first != "" {
			return first
		}
	}
	if len(req.SearchQueries) > 0 {
		if first := strings.TrimSpace(req.SearchQueries[0]); first != "" {
			return first
		}
	}
	if url := strings.TrimSpace(req.URL); url != "" {
		return url
	}
	if strings.TrimSpace(req.Image) != "" {
		return "Review provided image"
	}
	if strings.TrimSpace(req.Text) != "" {
		return "Analyze provided text"
	}
	return ""
}

func toolResearchBudgetMaxSources(budget *tools.ResearchBudget) int {
	if budget == nil {
		return 0
	}
	return budget.MaxSources
}

func toolResearchBudgetMaxSeconds(budget *tools.ResearchBudget) int {
	if budget == nil {
		return 0
	}
	return budget.MaxSeconds
}
