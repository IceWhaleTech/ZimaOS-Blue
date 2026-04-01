package bootstrap

import (
	"context"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
)

const runtimeProxyToggleSaveTimeout = 5 * time.Second

type runtimeProxyTogglePersistence struct {
	store    *proxy.ToggleStore
	snapshot func() *proxy.ToggleState
}

type runtimeProxyControlRoutes struct {
	v1             *echo.Group
	authMiddleware echo.MiddlewareFunc
	pageMiddleware echo.MiddlewareFunc
	handler        *proxy.ProxyHandler
	pipelineStats  *proxy.PipelineStatsCollector
	persistence    runtimeProxyTogglePersistence
}

func (p runtimeProxyTogglePersistence) enabled() bool {
	return p.store != nil && p.snapshot != nil
}

func (p runtimeProxyTogglePersistence) Save(ctx context.Context) error {
	if !p.enabled() {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return p.store.Save(ctx, p.snapshot())
}

func (p runtimeProxyTogglePersistence) SaveWithTimeout(parent context.Context) error {
	if !p.enabled() {
		return nil
	}
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, runtimeProxyToggleSaveTimeout)
	defer cancel()
	return p.Save(ctx)
}
