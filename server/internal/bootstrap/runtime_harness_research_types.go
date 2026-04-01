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
}

type deepResearchRuntimeRouteTarget interface {
	deepResearchJobCreatorTarget
	RegisterGroup(group *echo.Group)
}

type autoHarnessTurnHookTarget interface {
	RegisterTurnHook(hook serverpkg.TurnHook)
}

type harnessJudgeLLMCaller interface {
	Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error)
}

type harnessJudgeEvaluatorTarget interface {
	SetJudgeEvaluator(evaluator harness.JudgeEvaluator)
}
