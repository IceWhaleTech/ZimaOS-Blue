package bootstrap

import (
	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/a2ui"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mfa"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func registerRouteRuntimeConfigSurface(
	authPageV1Group func(string) *echo.Group,
	v1 *echo.Group,
	hotReloader *config.HotReloader,
	configStore *config.ConfigStore,
) {
	if authPageV1Group != nil {
		configHandler := serverpkg.NewConfigHandler(hotReloader)
		if configStore != nil {
			configHandler.SetConfigStore(configStore)
		}
		configHandler.RegisterRoutes(authPageV1Group(permission.PageSettings))
	}

	if v1 != nil {
		serverpkg.NewTemplatesHandler().RegisterRoutes(v1)
	}
}

func registerRouteRuntimeCanvasSurface(
	protected *echo.Group,
	toolRegistry *tools.Registry,
	a2uiManager *a2ui.Manager,
	pdfService tools.PDFService,
) {
	if a2uiManager != nil {
		tools.RegisterCanvasTools(toolRegistry, a2uiManager)
		if protected != nil {
			a2ui.NewHandler(a2uiManager).RegisterRoutes(protected.Group("/a2ui"))
		}
	}
	if pdfService != nil {
		tools.RegisterPDFTool(toolRegistry, pdfService)
	}
}

func registerRouteRuntimeMFASurface(protected *echo.Group) {
	if protected == nil {
		return
	}
	mfa.NewHandler(nil, nil).RegisterRoutes(protected)
}
