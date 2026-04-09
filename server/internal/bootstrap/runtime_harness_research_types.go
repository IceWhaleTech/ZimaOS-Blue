package bootstrap

import (
	"context"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
)

type deepResearchJobCreatorTarget interface {
	SetJobCreator(creator interface {
		CreateJob(ctx context.Context, req deepresearch.CreateJobRequest) (*deepresearch.Job, error)
	})
	SetJobService(service interface {
		ListJobsForUser(userID, tenantID string, activeOnly bool) ([]deepresearch.JobSummary, error)
		GetJobForUser(id, userID, tenantID string) (*deepresearch.Job, error)
		GetReportForUser(id, userID, tenantID string) (*deepresearch.Report, error)
		CancelJobForUser(id, userID, tenantID string) error
		SubscribeForUser(jobID, userID, tenantID string) (<-chan deepresearch.Event, func(), error)
	})
}

type deepResearchRuntimeRouteTarget interface {
	deepResearchJobCreatorTarget
	RegisterGroup(group *echo.Group)
}

type autoHarnessTurnHookTarget interface{ RegisterTurnHook(serverpkg.TurnHook) }

type harnessJudgeLLMCaller interface {
	Chat(context.Context, llm.ChatRequest) (*llm.ChatResponse, error)
}

type harnessJudgeEvaluatorTarget interface{ SetJudgeEvaluator(harness.JudgeEvaluator) }
