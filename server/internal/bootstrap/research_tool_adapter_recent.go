package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

const researchRecentRetrievalProfile = "recent_multi_site_v1"

func (a *deepResearchToolAdapter) SetRecentExecutor(executor tools.Tool) {
	if a == nil {
		return
	}
	a.recent = executor
}

func (a *deepResearchToolAdapter) canRunLocalRecent(req tools.ResearchCreateJobRequest) bool {
	return a != nil && a.manager == nil && a.recent != nil && strings.TrimSpace(req.RetrievalProfile) == researchRecentRetrievalProfile
}

func (a *deepResearchToolAdapter) createLocalRecentJob(ctx context.Context, req tools.ResearchCreateJobRequest) (*tools.ResearchJob, error) {
	raw, err := a.recent.Execute(ctx, map[string]interface{}{
		"input":             canonicalResearchGoal(req),
		"retrieval_profile": req.RetrievalProfile,
	})
	if err != nil {
		return nil, err
	}
	report, err := toolResearchLocalRecentReport(raw)
	if err != nil {
		return nil, err
	}
	job := &tools.ResearchJob{
		ID:                 fmt.Sprintf("research_local_recent_%d", time.Now().UTC().UnixNano()),
		ConversationID:     req.ConversationID,
		Status:             "completed",
		Query:              canonicalResearchGoal(req),
		Mode:               "deep_research",
		ResearchDepth:      toolResearchRequestMode(req),
		RetrievalProfile:   req.RetrievalProfile,
		RequestedRouteMode: toolResearchRouteMode(req.RouteMode),
		EffectiveRouteMode: "web",
		Progress:           100,
		Answer:             toolResearchReportString(report["answer"]),
		Confidence:         toolResearchReportFloat(report["confidence"]),
		Report:             report,
	}
	a.storeLocalJob(req.UserID, job)
	return job, nil
}

func toolResearchLocalRecentReport(raw interface{}) (map[string]interface{}, error) {
	switch typed := raw.(type) {
	case string:
		var report map[string]interface{}
		if err := json.Unmarshal([]byte(typed), &report); err != nil {
			return nil, err
		}
		return report, nil
	case []byte:
		var report map[string]interface{}
		if err := json.Unmarshal(typed, &report); err != nil {
			return nil, err
		}
		return report, nil
	case map[string]interface{}:
		return cloneResearchToolMap(typed), nil
	default:
		return nil, fmt.Errorf("unexpected local recent executor result %T", raw)
	}
}

func toolResearchReportFloat(raw interface{}) float64 {
	switch typed := raw.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	default:
		return 0
	}
}

func toolResearchReportString(raw interface{}) string {
	if value, ok := raw.(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func toolResearchRouteMode(raw string) string {
	if strings.TrimSpace(raw) != "" {
		return strings.TrimSpace(raw)
	}
	return "web"
}
