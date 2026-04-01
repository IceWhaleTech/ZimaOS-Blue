package bootstrap

import (
	"database/sql"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool/oauth"
)

type routeRuntimeContractProviderPoolOptions struct {
	e                     *echo.Echo
	protected             *echo.Group
	providerPool          *providerpool.Pool
	writeDB               *sql.DB
	readDB                *sql.DB
	dataDir               string
	logger                *zap.Logger
	trace                 *StartupTrace
	requirePagePermission func(string) echo.MiddlewareFunc
}

type routeRuntimeContractProviderPoolResult struct {
	oauthManager *oauth.LazyManager
}
