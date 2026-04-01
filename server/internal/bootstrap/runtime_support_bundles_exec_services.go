package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentsessions"
	convertsvc "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/convert"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"go.uber.org/zap"
)

func newRuntimeExecConvertSupport(
	options runtimeExecSupportOptions,
	approvals *tools.ApprovalManager,
	dirStore *tools.DirAllowlistStore,
) (*convertsvc.Service, *convertsvc.Handler) {
	var (
		service *convertsvc.Service
		handler *convertsvc.Handler
		err     error
	)
	if options.readDB != nil {
		service, handler, err = newRuntimeConvertSupportWithReadDB(
			options.writeDB,
			options.readDB,
			options.dataDir,
			options.memoryStore,
			options.toolRegistry,
			approvals,
			dirStore,
			options.workspaceAllowedPath,
		)
	} else {
		service, handler, err = newRuntimeConvertSupport(
			options.writeDB,
			options.dataDir,
			options.memoryStore,
			options.toolRegistry,
			approvals,
			dirStore,
			options.workspaceAllowedPath,
		)
	}
	if err != nil {
		if options.logger != nil {
			options.logger.Warn("Failed to initialize convert service", zap.Error(err))
		}
		return nil, nil
	}
	return service, handler
}

func newRuntimeExecAgentSessionsService(
	options runtimeExecSupportOptions,
	bundle runtimeExecSupportBundle,
) (*agentsessions.Service, error) {
	if options.readDB != nil {
		return newRuntimeAgentSessionsServiceWithReadDB(
			options.writeDB,
			options.readDB,
			options.logger,
			options.workspaceAllowedPath,
			bundle.ExecConfig,
			bundle.Approvals,
			bundle.DirStore,
			bundle.AuditStore,
			options.oauthSource,
			options.lookupAPIKey,
		)
	}
	return newRuntimeAgentSessionsService(
		options.writeDB,
		options.logger,
		options.workspaceAllowedPath,
		bundle.ExecConfig,
		bundle.Approvals,
		bundle.DirStore,
		bundle.AuditStore,
		options.oauthSource,
		options.lookupAPIKey,
	)
}
