package bootstrap

import (
	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workflow"
)

func registerCronRoutes(apiProtected *echo.Group, cronHandler routeRegistrar, pageMiddleware echo.MiddlewareFunc) {
	if apiProtected == nil {
		return
	}
	if routeRuntimeHasValue(cronHandler) {
		cronHandler.RegisterRoutes(apiProtected.Group("", filterRouteMiddlewares(pageMiddleware)...))
		return
	}

	stub := featureDisabled("cron")
	cronGroup := apiProtected.Group("/cron", filterRouteMiddlewares(pageMiddleware)...)
	cronGroup.GET("", stub)
	cronGroup.Any("/*", stub)
}

func registerBrowserAutomationRoutes(apiProtected *echo.Group, browserHandler routeRegistrar, pageMiddleware echo.MiddlewareFunc) {
	if apiProtected == nil {
		return
	}

	browserGroup := apiProtected.Group("/browser", filterRouteMiddlewares(pageMiddleware)...)
	if routeRuntimeHasValue(browserHandler) {
		browserHandler.RegisterRoutes(browserGroup)
		return
	}

	stub := featureDisabled("browser")
	browserGroup.GET("/tasks", stub)
	browserGroup.GET("/sessions", stub)
	browserGroup.GET("/security", stub)
	browserGroup.Any("/*", stub)
}

func registerWorkflowRoutes(
	e *echo.Echo,
	v1 *echo.Group,
	workflowHandler workflowRouteRegistrar,
	authMiddleware, pageMiddleware echo.MiddlewareFunc,
	flagEvaluator *config.FlagEvaluator,
) {
	if e == nil || v1 == nil {
		return
	}
	if routeRuntimeHasValue(workflowHandler) {
		workflowHandler.SetServiceInitHook(func(svc *workflow.WorkflowService) {
			svc.SetFlagEvaluator(flagEvaluator)
		})
		workflowHandler.SetRouteMiddlewares(filterRouteMiddlewares(authMiddleware, pageMiddleware)...)
		workflowHandler.RegisterRoutes(e)
		return
	}

	stub := featureDisabled("workflow")
	wfGroup := v1.Group("/workflows", filterRouteMiddlewares(authMiddleware, pageMiddleware)...)
	wfGroup.GET("", stub)
	wfGroup.GET("/stats", stub)
	wfGroup.GET("/templates", stub)
	wfGroup.Any("/*", stub)
}

func registerVoiceRoutes(
	v1, protected *echo.Group,
	voiceHandler, voiceWSHandler routeRegistrar,
	authMiddleware, pageMiddleware echo.MiddlewareFunc,
) {
	if v1 == nil {
		return
	}
	if routeRuntimeHasValue(voiceHandler) {
		voiceGroup := v1.Group("/voice", filterRouteMiddlewares(authMiddleware, pageMiddleware)...)
		voiceHandler.RegisterRoutes(voiceGroup)
		if protected != nil && routeRuntimeHasValue(voiceWSHandler) {
			voiceWSGroup := protected.Group("/voice", filterRouteMiddlewares(pageMiddleware)...)
			voiceWSHandler.RegisterRoutes(voiceWSGroup)
		}
		return
	}

	stub := featureDisabled("voice")
	voiceGroup := v1.Group("/voice", filterRouteMiddlewares(authMiddleware, pageMiddleware)...)
	voiceGroup.GET("/voices", stub)
	voiceGroup.GET("/sessions", stub)
	voiceGroup.POST("/transcribe", stub)
	voiceGroup.POST("/synthesize", stub)
	voiceGroup.Any("/*", stub)
}

func registerSpeechRoutes(v1 *echo.Group, speechHandler routeRegistrar, authMiddleware, pageMiddleware echo.MiddlewareFunc) {
	if v1 == nil {
		return
	}
	if routeRuntimeHasValue(speechHandler) {
		speechHandler.RegisterRoutes(v1.Group("/speech", filterRouteMiddlewares(authMiddleware, pageMiddleware)...))
		return
	}

	stub := featureDisabled("speech")
	speechGroup := v1.Group("/speech", filterRouteMiddlewares(authMiddleware, pageMiddleware)...)
	speechGroup.GET("/status", stub)
	speechGroup.GET("/models", stub)
	speechGroup.GET("/asr/status", stub)
	speechGroup.GET("/asr/models", stub)
	speechGroup.GET("/tts/status", stub)
	speechGroup.GET("/tts/models", stub)
	speechGroup.Any("/*", stub)
}
