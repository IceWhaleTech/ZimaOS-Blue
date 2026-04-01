package bootstrap

import (
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/promptguard"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func bindHarnessRuntimeToolObserver(bundle *HarnessRuntimeBundle, target runtimeToolEventObserverTarget) {
	if target == nil {
		return
	}
	if observer := harnessRuntimeObserver(bundle); observer != nil {
		target.SetToolEventObserver(observer)
	}
}

func bindHarnessRuntimeObserver(bundle *HarnessRuntimeBundle, target runtimeObserverTarget) {
	if target == nil {
		return
	}
	if observer := harnessRuntimeObserver(bundle); observer != nil {
		target.SetObserver(observer)
	}
}

func bindRuntimeToolApprover(approver tools.ToolApprover, target runtimeToolApproverTarget) {
	if approver == nil || target == nil {
		return
	}
	target.SetToolApprover(approver)
}

func bindRuntimePromptGuard(
	chat runtimePromptGuardTarget,
	security runtimePromptGuardTarget,
	disabled bool,
	logger *zap.Logger,
) {
	if disabled {
		if chat != nil {
			chat.SetPromptGuard(nil)
		}
		if security != nil {
			security.SetPromptGuard(nil)
		}
		if logger != nil {
			logger.Warn("Chat prompt interception disabled via CLI flag")
		}
		return
	}

	guard := promptguard.NewDetector(promptguard.DefaultDetectorConfig())
	if chat != nil {
		chat.SetPromptGuard(guard)
	}
	if security != nil {
		security.SetPromptGuard(guard)
	}
}

func bindRuntimeToolSelection(
	chat *serverpkg.ChatHandler,
	cfg *config.Config,
	dataDir string,
	workspaceDir string,
	flagEvaluator runtimeChatFlagEvaluator,
) *agentcore.AutoSkillReranker {
	if chat == nil || cfg == nil {
		return nil
	}

	selector := tools.DefaultToolSelector()
	if cfg.ToolCalling.SmartSelectionMaxTools > 0 {
		selector.MaxTools = cfg.ToolCalling.SmartSelectionMaxTools
	}
	chat.SetToolSelector(selector)
	chat.SetToolPolicyResolver(tools.NewToolPolicyResolver(cfg))
	chat.SetToolTraceStore(tools.NewToolTraceStore(1000))
	chat.SetFlagEvaluator(flagEvaluator)
	chat.SetToolRouter(tools.DefaultToolRouter())

	// Keep a selector available even when startup config leaves smart selection off.
	// The runtime setting still gates actual usage, but dry-run and later toggles
	// should not require a process restart just to materialize the selector.
	baseSelector := agentcore.NewSkillSelector(workspaceDir, nil)
	baseSelector.SetDynamicExposureEnabledFunc(func() bool {
		if settings := chat.GetSettingsHandler(); settings != nil {
			if enabled, ok := settings.GetSkillDynamicExposureExplicit(); ok {
				return enabled
			}
		}
		return cfg.ToolCalling.SkillDynamicExposure
	})
	chat.SetSkillSelector(baseSelector)

	if !cfg.ToolCalling.SmartSkillSelection {
		return nil
	}

	reranker := agentcore.NewAutoSkillReranker(dataDir, cfg.ToolCalling.SkillRerankModel, agentcore.AutoSkillRerankerOptions{
		ONNXEnabled:  cfg.ToolCalling.SkillRerankEnabled && cfg.ToolCalling.SkillRerankONNXEnabled,
		AutoDownload: cfg.ToolCalling.SkillRerankONNXAutoDownload,
	})
	rerankSelector := agentcore.NewSkillSelector(workspaceDir, reranker)
	rerankSelector.SetDynamicExposureEnabledFunc(func() bool {
		if settings := chat.GetSettingsHandler(); settings != nil {
			if enabled, ok := settings.GetSkillDynamicExposureExplicit(); ok {
				return enabled
			}
		}
		return cfg.ToolCalling.SkillDynamicExposure
	})
	chat.SetSkillSelector(rerankSelector)
	return reranker
}
