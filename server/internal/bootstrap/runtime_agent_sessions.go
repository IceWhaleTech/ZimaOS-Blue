package bootstrap

import (
	"database/sql"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentsessions"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func newRuntimeAgentSessionsService(
	db *sql.DB,
	logger *zap.Logger,
	allowedPaths []string,
	execConfig tools.ExecConfig,
	execApprovals *tools.ApprovalManager,
	execDirStore *tools.DirAllowlistStore,
	execAuditStore *tools.ExecAuditStore,
	oauthSource func() agentsessions.OAuthCredentialSource,
	lookupAPIKey func(providerID string) (string, error),
) (*agentsessions.Service, error) {
	return newRuntimeAgentSessionsServiceWithReadDB(db, db, logger, allowedPaths, execConfig, execApprovals, execDirStore, execAuditStore, oauthSource, lookupAPIKey)
}

func newRuntimeAgentSessionsServiceWithReadDB(
	writeDB *sql.DB,
	readDB *sql.DB,
	logger *zap.Logger,
	allowedPaths []string,
	execConfig tools.ExecConfig,
	execApprovals *tools.ApprovalManager,
	execDirStore *tools.DirAllowlistStore,
	execAuditStore *tools.ExecAuditStore,
	oauthSource func() agentsessions.OAuthCredentialSource,
	lookupAPIKey func(providerID string) (string, error),
) (*agentsessions.Service, error) {
	if writeDB == nil {
		return nil, nil
	}

	store, err := agentsessions.NewSQLiteStoreWithReadDB(writeDB, readDB)
	if err != nil {
		return nil, err
	}

	authority := tools.NewACPAuthority(tools.ACPAuthorityConfig{
		AllowedPaths: allowedPaths,
		MaxFileSize:  0,
		ExecConfig:   execConfig,
		Approvals:    execApprovals,
		DirStore:     execDirStore,
	})
	if execAuditStore != nil {
		authority.SetAuditStore(execAuditStore)
	}

	credentialResolver := agentsessions.NewProviderPoolCredentialResolver(oauthSource, lookupAPIKey, logger)
	return agentsessions.NewService(
		store,
		logger,
		map[agentsessions.ProtocolKind]agentsessions.ProtocolRuntime{
			agentsessions.ProtocolACP: agentsessions.NewACPRuntime(authority, logger, credentialResolver),
			agentsessions.ProtocolA2A: agentsessions.NewA2ARuntime(logger),
		},
	)
}

func bindRuntimeAgentSessions(
	registry *tools.Registry,
	memoryStore *memory.Store,
	service *agentsessions.Service,
	profileRoutes *echo.Group,
	sessionRoutes *echo.Group,
) {
	if service == nil {
		return
	}
	handler := agentsessions.NewHandler(service)
	handler.RegisterProfileRoutes(profileRoutes)
	handler.RegisterSessionRoutes(sessionRoutes)
	if registry != nil {
		tools.RegisterSessionTools(registry, sessionListAdapter{
			store:         memoryStore,
			agentSessions: service,
		})
	}
}
