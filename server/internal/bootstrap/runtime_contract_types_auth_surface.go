package bootstrap

import (
	"database/sql"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/user"
)

type routeRuntimeContractAuthSurfaceOptions struct {
	v1             *echo.Group
	api            *echo.Group
	writeDB        *sql.DB
	readDB         *sql.DB
	authMiddleware *auth.AuthMiddleware
	userRepo       user.Repository
	logger         *zap.Logger
}

type routeRuntimeContractAuthSurfaceResult struct {
	protected             *echo.Group
	apiProtected          *echo.Group
	authPageV1Group       func(string) *echo.Group
	authPageAPIGroup      func(string) *echo.Group
	requirePagePermission func(string) echo.MiddlewareFunc
	authMiddleware        echo.MiddlewareFunc
	permissionHandler     permissionRouteRegistrar
}
