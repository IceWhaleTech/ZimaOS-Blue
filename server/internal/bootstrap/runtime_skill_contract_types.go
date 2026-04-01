package bootstrap

import (
	"context"
	"database/sql"
	"io/fs"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sockipc"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workspace"
)

type routeRuntimeContractSkillOptions struct {
	writeDB         *sql.DB
	readDB          *sql.DB
	dataDir         string
	appConfig       *config.Config
	services        *Services
	settings        *serverpkg.SettingsHandler
	chat            *serverpkg.ChatHandler
	authPageV1Group func(string) *echo.Group
	ipcServer       *sockipc.Server
	ctx             context.Context
	logger          *zap.Logger
	closers         *[]interface{ Close() error }
	eventBroker     *sse.Broker
	skillEmbedFS    fs.FS
	workspace       *workspace.Handler
}

type routeRuntimeContractSkillResult struct {
	skillsDir                string
	storeBound               bool
	featuredLoaderConfigured bool
	localScannerConfigured   bool
	marketplaceConfigured    bool
	routesRegistered         bool
	ipcHandlersRegistered    bool
	closerRegistered         bool
}
