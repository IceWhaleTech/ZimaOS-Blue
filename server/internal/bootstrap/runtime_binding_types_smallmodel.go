package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/session"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/smallmodel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type runtimeSmallModelSettingsSource interface {
	GetSmallModelEnabled() bool
	GetSmallModelRouteImageQAEnabled() bool
	GetSmallModelDocExtractEnabled() bool
	SetSmallModelDocExtractEnabled(enabled bool) (bool, error)
}

type runtimeSmallModelChatTarget interface {
	SetSmallModelRuntime(rt smallmodel.Runtime)
}

type runtimeSmallModelAuxiliaryTarget interface {
	SetSmallModel(rt smallmodel.Runtime)
}

type runtimeImageSmallModelTarget interface {
	SetSmallModelRuntime(rt smallmodel.Runtime)
	SetSmallModelEnabledFunc(fn func() bool)
}

type runtimeAnalyzeSmallModelTarget interface {
	SetSmallModelRuntime(rt smallmodel.Runtime)
	SetSmallModelSwitchFuncs(enabledFn, docExtractFn func() bool)
	SetSmallModelDocExtractToggle(setter func(bool) (bool, error))
	SetSmallModelStatsRecorder(recorder tools.SmallModelStatsRecorder)
}

type runtimeSmallModelStatsSource interface {
	GetSmallModelStats() *serverpkg.SmallModelStats
}

type runtimeSkillRerankerSettingsTarget interface {
	SetSkillRerankerModelManager(mgr *agentcore.SkillRerankerModelManager)
	GetSmallModelEnabled() bool
	GetSmallModelRerankEnabled() bool
	GetSkillRerankEnabled() bool
	IsSkillRerankEnabledSet() bool
	GetSkillRerankONNXEnabled() bool
	IsSkillRerankONNXEnabledSet() bool
	GetSkillRerankONNXAutoDownload() bool
	IsSkillRerankONNXAutoDownloadSet() bool
}

type runtimeSkillRerankerTarget interface {
	ModelManager() *agentcore.SkillRerankerModelManager
	SetSwitchFuncs(onnxEnabledFn, autoDownloadFn func() bool)
}

type runtimeSkillRerankerDefaults struct {
	dataDir       string
	modelRepo     string
	rerankEnabled bool
	onnxEnabled   bool
	autoDownload  bool
}

type runtimeCompactorMemoryChatTarget interface {
	SetCompactorMemoryIntegration(integration *session.CompactorMemoryIntegration, sessionMaxTokens int)
}
