package bootstrap

import (
	"context"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	networkapi "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/api"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cron"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/ngrok"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/update"
)

type routeRuntimeContractManagementSupportOptions struct {
	authPageV1Group       func(string) *echo.Group
	protected             *echo.Group
	requirePagePermission func(string) echo.MiddlewareFunc
	config                *config.Config
	serverConfig          *ServerConfig
	ctx                   context.Context
	logger                *zap.Logger
	cronHandler           *cron.Handler
	ngrokTunnelMgr        *ngrok.SDKTunnelManager
	ngrokConfigStore      *ngrok.ConfigStore
	jwtService            *auth.JWTService
	providerRegistry      *llm.ProviderRegistry
	configKV              kvstore.Store
}

type routeRuntimeContractManagementSupportResult struct {
	updateHandler    *update.Handler
	otaChecker       *update.OTAChecker
	providerSettings *serverpkg.ProviderSettingsHandler
}

type routeRuntimeRemoteAccessBinding struct {
	handler *networkapi.SDKRemoteAccessHandler
}

type routeRuntimeUpdateBinding struct {
	handler *update.Handler
	checker *update.OTAChecker
}
