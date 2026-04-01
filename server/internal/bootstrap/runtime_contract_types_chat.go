package bootstrap

import (
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
)

type routeRuntimeContractChatOptions struct {
	chat          *serverpkg.ChatHandler
	chatPrompt    runtimePromptGuardTarget
	security      runtimePromptGuardTarget
	disabled      bool
	config        *config.Config
	dataDir       string
	workspaceDir  string
	flagEvaluator runtimeChatFlagEvaluator
	logger        *zap.Logger
}

type routeRuntimeContractChatBindingOptions struct {
	target  autoHarnessTurnHookTarget
	handler *serverpkg.ChatHandler
}

type routeRuntimeContractChatBindingResult struct {
	autoHarnessHookBound bool
}
