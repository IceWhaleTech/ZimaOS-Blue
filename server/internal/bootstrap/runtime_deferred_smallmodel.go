package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/smallmodel"
)

func bindRuntimeSmallModel(
	settings runtimeSmallModelSettingsSource,
	runtime smallmodel.Runtime,
	chat runtimeSmallModelChatTarget,
	auxiliary runtimeSmallModelAuxiliaryTarget,
	image runtimeImageSmallModelTarget,
	analyze runtimeAnalyzeSmallModelTarget,
	stats runtimeSmallModelStatsSource,
) {
	if chat != nil {
		chat.SetSmallModelRuntime(runtime)
	}
	if auxiliary != nil {
		auxiliary.SetSmallModel(runtime)
	}
	if image != nil {
		image.SetSmallModelRuntime(runtime)
		if settings != nil {
			image.SetSmallModelEnabledFunc(settings.GetSmallModelEnabled)
		}
	}
	if analyze != nil {
		analyze.SetSmallModelRuntime(runtime)
		if settings != nil {
			analyze.SetSmallModelSwitchFuncs(
				settings.GetSmallModelEnabled,
				settings.GetSmallModelDocExtractEnabled,
			)
			analyze.SetSmallModelDocExtractToggle(settings.SetSmallModelDocExtractEnabled)
		}
		if stats != nil {
			if recorder := stats.GetSmallModelStats(); recorder != nil {
				analyze.SetSmallModelStatsRecorder(recorder)
			}
		}
	}
}

func bindRuntimeSkillReranker(
	settings runtimeSkillRerankerSettingsTarget,
	reranker runtimeSkillRerankerTarget,
	defaults runtimeSkillRerankerDefaults,
) {
	if settings == nil {
		return
	}

	manager := (*agentcore.SkillRerankerModelManager)(nil)
	if reranker != nil {
		manager = reranker.ModelManager()
	}
	if manager == nil {
		manager = agentcore.NewSkillRerankerModelManager(defaults.dataDir, defaults.modelRepo)
	}
	settings.SetSkillRerankerModelManager(manager)

	if reranker == nil {
		return
	}

	reranker.SetSwitchFuncs(
		func() bool {
			rerankEnabled := defaults.rerankEnabled
			if settings.IsSkillRerankEnabledSet() {
				rerankEnabled = settings.GetSkillRerankEnabled()
			}
			if settings.GetSmallModelEnabled() && !settings.GetSmallModelRerankEnabled() {
				rerankEnabled = false
			}

			onnxEnabled := defaults.onnxEnabled
			if settings.IsSkillRerankONNXEnabledSet() {
				onnxEnabled = settings.GetSkillRerankONNXEnabled()
			}
			return rerankEnabled && onnxEnabled
		},
		func() bool {
			autoDownload := defaults.autoDownload
			if settings.IsSkillRerankONNXAutoDownloadSet() {
				autoDownload = settings.GetSkillRerankONNXAutoDownload()
			}
			return autoDownload
		},
	)
}
