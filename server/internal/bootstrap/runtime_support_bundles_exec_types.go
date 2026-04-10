package bootstrap

import (
	"database/sql"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentsessions"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	convertsvc "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/convert"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sandbox"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type runtimeExecSupportBundle struct {
	ExecConfig     tools.ExecConfig
	Approvals      *tools.ApprovalManager
	DirStore       *tools.DirAllowlistStore
	AuditStore     *tools.ExecAuditStore
	ConvertHandler *convertsvc.Handler
}

type runtimeExecSupportOptions struct {
	writeDB              *sql.DB
	readDB               *sql.DB
	dataDir              string
	workspaceDir         string
	serverConfig         *ServerConfig
	ripgrep              config.ToolCallingRipgrepConfig
	workspaceAllowedPath []string
	memoryStore          *memory.Store
	toolRegistry         *tools.Registry
	skillRegistry        runtimeSkillRegistrySource
	selectorSource       runtimeExecSkillSelectionSource
	broker               *sse.Broker
	harnessRuntime       *HarnessRuntimeBundle
	sandboxManager       *sandbox.Manager
	chatHandler          *serverpkg.ChatHandler
	logger               *zap.Logger
	closers              *[]interface{ Close() error }
	profileRoutes        *echo.Group
	sessionRoutes        *echo.Group
	oauthSource          func() agentsessions.OAuthCredentialSource
	lookupAPIKey         func(providerID string) (string, error)
}
