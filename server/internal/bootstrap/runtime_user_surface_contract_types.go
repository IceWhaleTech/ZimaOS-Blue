package bootstrap

import (
	"database/sql"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/metrics"
)

type companionGroupRouteRegistrar interface {
	RegisterGroupRoutes(*echo.Group)
	RegisterCompatGroupRoutes(*echo.Group)
}

type companionRouteRegistration struct {
	handlerRegistered bool
	wsRegistered      bool
}

type routeRuntimeContractUserSurfaceOptions struct {
	v1                    *echo.Group
	protected             *echo.Group
	authPageV1Group       func(string) *echo.Group
	authPageAPIGroup      func(string) *echo.Group
	requirePagePermission func(string) echo.MiddlewareFunc
	workspace             routeRegistrar
	companionHandler      companionGroupRouteRegistrar
	companionWSHandler    companionGroupRouteRegistrar
	writeDB               *sql.DB
	readDB                *sql.DB
	llmRegistry           *llm.ProviderRegistry
	skillsDir             string
	metricsWriter         *metrics.MetricsWriter
	logger                *zap.Logger
	trace                 *StartupTrace
}
