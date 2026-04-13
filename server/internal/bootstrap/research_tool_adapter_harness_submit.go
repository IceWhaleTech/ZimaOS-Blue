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
		Query:            canonicalResearchGoal(req),
		UserID:           req.UserID,
		ConversationID:   req.ConversationID,
		WorkspaceRoot:    strings.TrimSpace(workspaceRoot),
		Mode:             strings.TrimSpace(req.Mode),
		ResearchDepth:    strings.TrimSpace(req.ResearchDepth),
		RetrievalProfile: strings.TrimSpace(req.RetrievalProfile),
		RouteMode:        strings.TrimSpace(req.RouteMode),
		Lang:             strings.TrimSpace(req.Lang),
		ReportStyle:      strings.TrimSpace(req.ReportStyle),
		TimeWindows:      append([]string(nil), req.TimeWindows...),
		StrictEntity:     req.StrictEntity != nil && *req.StrictEntity,
		MaxSources:       toolResearchBudgetMaxSources(req.Budget),
		MaxSeconds:       toolResearchBudgetMaxSeconds(req.Budget),
		Topic:            strings.TrimSpace(req.Topic),
		URLs:             append([]string(nil), req.URLs...),
		Text:             strings.TrimSpace(req.Text),
		SearchQueries:    append([]string(nil), req.SearchQueries...),
		OutputMode:       strings.TrimSpace(req.OutputMode),
		Question:         strings.TrimSpace(req.Question),
		Category:         strings.TrimSpace(req.Category),
		DecisionMode:     strings.TrimSpace(req.DecisionMode),
		Candidates:       append([]string(nil), req.Candidates...),
		Context:          cloneResearchToolMap(req.Context),
		Constraints:      cloneResearchToolMap(req.Constraints),
		Grounding:        strings.TrimSpace(req.Grounding),
		Depth:            strings.TrimSpace(req.Depth),
		Output:           strings.TrimSpace(req.Output),
		ScorecardPack:    strings.TrimSpace(req.ScorecardPack),
		ScorecardWeights: cloneResearchToolWeightMap(req.ScorecardWeights),
		Action:           strings.TrimSpace(req.Action),
		URL:              strings.TrimSpace(req.URL),
		Image:            strings.TrimSpace(req.Image),
		Device:           strings.TrimSpace(req.Device),
		Channel:          strings.TrimSpace(req.Channel),
		WaitMS:           req.WaitMS,
		Threshold:        req.Threshold,
		Format:           strings.TrimSpace(req.Format),
		Profile:          strings.TrimSpace(req.Profile),
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

func cloneResearchToolMap(in map[string]interface{}) map[string]interface{} {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneResearchToolWeightMap(in map[string]float64) map[string]float64 {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]float64, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
