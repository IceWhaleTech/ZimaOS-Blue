package bootstrap

import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/smallmodel"

func newRuntimeActivationDeferredWiring(
	activation runtimeActivationResult,
	options runtimeActivationDeferredSupportOptions,
	runner agentRuntimePolicyTarget,
	autoDownload bool,
) runtimeDeferredWiring {
	return runtimeDeferredWiring{
		settings:               options.settings,
		smallRuntime:           smallmodel.NewLlamaCppRuntime(options.smallModelManager, smallmodel.LlamaCppRuntimeOptions{AutoDownload: autoDownload}),
		chatSmallModel:         options.chatSmallModel,
		auxiliarySmallModel:    activation.auxiliaryLLM,
		analyzeSmallModel:      options.analyzeSmallModel,
		smallModelStats:        options.smallModelStats,
		harnessRuntime:         options.harnessRuntime,
		reflectionTarget:       options.reflectionTarget,
		reflectionLLM:          activation.auxiliaryLLM,
		reflectionProposalGate: options.reflectionProposalGate,
		compactorChat:          options.compactorChat,
		memoryHandler:          options.memoryHandler,
		auxiliaryLLM:           activation.auxiliaryLLM,
		sessionCompaction:      runtimeActivationSessionCompaction(options.deps),
		sessionMaxTokens:       runtimeActivationSessionMaxTokens(options.deps),
		providerSettings:       options.providerSettings,
		chatSettings:           options.chatSettings,
		researchSettings:       options.researchSettings,
		skillReranker:          options.skillReranker,
		skillRerankerDefaults:  options.skillRerankerDefaults,
		execAutoConfirm:        runtimeActivationExecAutoConfirm(options.services),
		promptSettings:         options.promptSettings,
		pushLocale:             options.pushLocale,
		maskerLocale:           activation.dataMasker,
		mgmtSettings:           options.mgmtSettings,
		questionMgr:            options.questionMgr,
		agentRunner:            runner,
	}
}
