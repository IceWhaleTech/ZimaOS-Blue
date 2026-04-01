package bootstrap

import (
	"database/sql"
	"log/slog"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

const (
	defaultRuntimeAskTimeout              = 5 * time.Minute
	defaultRuntimeBrowserCheckpointWindow = 5 * time.Minute
)

type runtimeAskSupportBundle struct {
	QuestionManager          *tools.QuestionManager
	BrowserCheckpointManager *tools.BrowserCheckpointManager
	BrowserSiteStore         *tools.BrowserSiteAllowlistStore
}

type runtimeAskSupportOptions struct {
	writeDB        *sql.DB
	readDB         *sql.DB
	appConfig      *config.Config
	toolRegistry   *tools.Registry
	skillRegistry  *skill.Registry
	broker         *sse.Broker
	harnessRuntime *HarnessRuntimeBundle
	chatTarget     chatAskRuntimeTarget
	mediaDir       string
	timeout        time.Duration
}

func newRuntimeAskSupportBundle(options runtimeAskSupportOptions) runtimeAskSupportBundle {
	timeout := options.timeout
	if timeout <= 0 {
		timeout = defaultRuntimeAskTimeout
	}

	bundle := runtimeAskSupportBundle{
		QuestionManager:          wireAskSupport(options.toolRegistry, options.skillRegistry, options.broker, timeout),
		BrowserCheckpointManager: tools.NewBrowserCheckpointManager(defaultRuntimeBrowserCheckpointWindow),
		BrowserSiteStore:         newRuntimeBrowserSiteStore(options.writeDB, options.readDB, options.appConfig),
	}

	bindHarnessRuntimeAskSupport(
		options.harnessRuntime,
		options.chatTarget,
		options.mediaDir,
		bundle.QuestionManager,
		bundle.BrowserCheckpointManager,
		bundle.BrowserSiteStore,
	)

	return bundle
}

func registerRuntimeAskSupportRoutes(v1 *echo.Group, authMiddleware *auth.AuthMiddleware, pageMiddleware echo.MiddlewareFunc, bundle runtimeAskSupportBundle) {
	var authRouteMiddleware echo.MiddlewareFunc
	if authMiddleware != nil {
		authRouteMiddleware = authMiddleware.Authenticate()
	}
	registerBrowserApprovalRoutes(v1, authRouteMiddleware, pageMiddleware, bundle.BrowserSiteStore)
	registerAskUserQuestionRoutes(v1, authRouteMiddleware, pageMiddleware, bundle.QuestionManager)
}

func newRuntimeBrowserSiteStore(writeDB, readDB *sql.DB, appConfig *config.Config) *tools.BrowserSiteAllowlistStore {
	if writeDB == nil {
		return nil
	}

	var (
		store *tools.BrowserSiteAllowlistStore
		err   error
	)
	if readDB != nil {
		store, err = tools.NewBrowserSiteAllowlistStoreWithReadDB(writeDB, readDB)
	} else {
		store, err = tools.NewBrowserSiteAllowlistStore(writeDB)
	}
	if err != nil {
		slog.Warn("failed to create browser site allowlist store", "error", err)
		return nil
	}

	if appConfig != nil {
		for _, site := range appConfig.Browser.ExpandedTrustedSites() {
			if err := store.Add(site, ""); err != nil {
				slog.Warn("failed to seed trusted browser site", "site", site, "error", err)
			}
		}
	}

	return store
}
