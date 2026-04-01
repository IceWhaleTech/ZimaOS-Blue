package bootstrap

import (
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/web"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/worker"
)

func registerRouteRuntimeHealthSurface(v1 *echo.Group, cfg *ServerConfig, workerPool runtimeWorkerStatsSource, logger *zap.Logger) {
	if v1 == nil {
		return
	}

	v1.GET("/health", func(c echo.Context) error {
		if StartupTraceEnabled() {
			if mark := strings.TrimSpace(c.QueryParam("startup_mark")); mark != "" && logger != nil {
				fields := []zap.Field{
					zap.String("component", "web.bootstrap"),
					zap.String("label", mark),
					zap.String("path", c.QueryParam("path")),
				}
				if msRaw := strings.TrimSpace(c.QueryParam("startup_ms")); msRaw != "" {
					if ms, err := strconv.ParseFloat(msRaw, 64); err == nil {
						fields = append(fields, zap.Float64("client_ms", ms))
					}
				}
				logger.Info("startup-trace", fields...)
			}
		}
		return c.JSON(http.StatusOK, map[string]interface{}{
			"status":  "ok",
			"service": "zimaos-blue",
			"version": routeRuntimeServerVersion(cfg),
		})
	})

	v1.GET("/health/stats", func(c echo.Context) error {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		uptime := timeutil.SinceTime(routesStartTime).Truncate(time.Second)
		return c.JSON(http.StatusOK, map[string]interface{}{
			"status":          "ok",
			"service":         "zimaos-blue",
			"timestamp":       timeutil.NowTime(),
			"uptime":          uptime.String(),
			"uptime_seconds":  uptime.Seconds(),
			"version":         routeRuntimeServerVersion(cfg),
			"go_version":      runtime.Version(),
			"num_cpu":         runtime.NumCPU(),
			"goroutines":      runtime.NumGoroutine(),
			"mem_alloc_bytes": m.Alloc,
		})
	})

	v1.GET("/workers/stats", func(c echo.Context) error {
		if workerPool == nil {
			return c.JSON(http.StatusOK, worker.Stats{})
		}
		return c.JSON(http.StatusOK, workerPool.Stats())
	})
}

func registerRouteRuntimeStaticSurface(e *echo.Echo, v1 *echo.Group, mediaDir string) {
	if v1 != nil && mediaDir != "" {
		_ = os.MkdirAll(mediaDir, 0o750)
		v1.Static("/media", mediaDir)
	}
	if e != nil {
		web.RegisterStaticRoutes(e)
	}
}
