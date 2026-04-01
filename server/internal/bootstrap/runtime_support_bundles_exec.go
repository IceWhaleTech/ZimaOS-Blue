package bootstrap

import (
	"strings"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func newRuntimeExecSupportBundle(options runtimeExecSupportOptions) runtimeExecSupportBundle {
	bundle := runtimeExecSupportBundle{
		ExecConfig: tools.DefaultExecConfig(),
		Approvals:  newHarnessRuntimeExecApprovals(options.harnessRuntime, options.broker),
		DirStore:   newRuntimeExecDirStore(options.writeDB, options.readDB),
	}

	bundle.ExecConfig.DataDir = strings.TrimSpace(options.dataDir)
	bundle.ExecConfig.AllowedDirs = append([]string(nil), options.workspaceAllowedPath...)

	tools.RegisterExecTools(
		options.toolRegistry,
		bundle.ExecConfig,
		bundle.Approvals,
		options.broker,
		bundle.DirStore,
		newRuntimeExecSandboxExecutor(options.sandboxManager),
	)
	tools.RegisterApprovalAwareFileToolsWithRuntimeConfig(
		options.toolRegistry,
		options.workspaceAllowedPath,
		0,
		bundle.Approvals,
		bundle.DirStore,
		tools.BuiltinRuntimeConfig{
			DataDir:      strings.TrimSpace(options.dataDir),
			WorkspaceDir: strings.TrimSpace(options.workspaceDir),
			Ripgrep:      options.ripgrep,
		},
	)

	convertService, convertHandler := newRuntimeExecConvertSupport(options, bundle.Approvals, bundle.DirStore)
	bundle.ConvertHandler = convertHandler
	if convertService != nil {
		if options.closers != nil {
			*options.closers = append(*options.closers, convertService)
		}
		if options.chatHandler != nil {
			options.chatHandler.SetConvertSourceProvider(convertService)
		}
	}

	bundle.AuditStore = newRuntimeExecAuditStore(options.writeDB, options.readDB)
	bindRuntimeExecTool(tools.GetExecTool(options.toolRegistry), options.toolRegistry, options.skillRegistry, options.workspaceDir, options.selectorSource, bundle.AuditStore)

	service, err := newRuntimeExecAgentSessionsService(options, bundle)
	if err != nil {
		if options.logger != nil {
			options.logger.Warn("Failed to initialize external agent sessions service", zap.Error(err))
		}
	} else {
		bindRuntimeAgentSessions(
			options.toolRegistry,
			options.memoryStore,
			service,
			options.profileRoutes,
			options.sessionRoutes,
		)
	}

	return bundle
}
