package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
)

func bindHarnessRuntimeAutoHarnessTurnHook(target autoHarnessTurnHookTarget, hook serverpkg.TurnHook) {
	if target == nil || hook == nil {
		return
	}
	target.RegisterTurnHook(hook)
}

func bindHarnessRuntimeJudgeEvaluator(target harnessJudgeEvaluatorTarget, llmCaller harnessJudgeLLMCaller) {
	if target == nil || llmCaller == nil {
		return
	}
	target.SetJudgeEvaluator(harness.NewLLMJudgeEvaluator(llmCaller))
}
