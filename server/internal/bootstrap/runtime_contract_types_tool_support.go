package bootstrap

import (
	"database/sql"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentsessions"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sandbox"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/update"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/user"
)

type routeRuntimeContractAskSupportOptions struct {
	writeDB       *sql.DB
	readDB        *sql.DB
	appConfig     *config.Config
	toolRegistry  *tools.Registry
	skillRegistry runtimeSkillRegistrySource
	broker        *sse.Broker
	chatTarget    chatAskRuntimeTarget
	mediaDir      string
	timeout       time.Duration
}

type routeRuntimeContractExecSupportOptions struct {
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
	sandboxManager       *sandbox.Manager
	chatHandler          *serverpkg.ChatHandler
	logger               *zap.Logger
	closers              *[]interface{ Close() error }
	profileRoutes        *echo.Group
	sessionRoutes        *echo.Group
	oauthSource          func() agentsessions.OAuthCredentialSource
	lookupAPIKey         func(providerID string) (string, error)
}

type routeRuntimeContractMgmtOptions struct {
	registry      *tools.Registry
	providerPool  *providerpool.Pool
	skillRegistry runtimeSkillRegistrySource
	workspaceDir  string
	version       string
	userService   *user.Service
	apiKeyService *auth.APIKeyService
}

type routeRuntimeContractMgmtUpgradeOptions struct {
	handler    *update.Handler
	otaChecker *update.OTAChecker
	version    string
}
