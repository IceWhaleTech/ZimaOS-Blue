package bootstrap

import (
	"context"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	networkapi "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/api"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/ngrok"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/smallmodel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
)

type routeRuntimeContractBootstrapSupportOptions struct {
	apiProtected          *echo.Group
	protected             *echo.Group
	authPageV1Group       func(string) *echo.Group
	requirePagePermission func(string) echo.MiddlewareFunc
	serverConfig          *ServerConfig
	configKV              kvstore.Store
	ctx                   context.Context
	logger                *zap.Logger
	chatHandler           *serverpkg.ChatHandler
	sseBroker             *sse.Broker
	ngrokConfigStore      *ngrok.ConfigStore
	jwtService            *auth.JWTService
}

type routeRuntimeContractBootstrapSupportResult struct {
	settingsHandler   *serverpkg.SettingsHandler
	smallModelManager *smallmodel.Manager
	approvalHandler   *networkapi.ApprovalHandler
}
