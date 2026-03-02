package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/update"
	"github.com/labstack/echo/v4"
)

// version is injected via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	port := mustInt("OTA_SMOKE_PORT")
	dataDir := mustEnv("OTA_SMOKE_DATA_DIR")
	storageDir := mustEnv("OTA_SMOKE_STORAGE_DIR")

	updateCfg := &update.Config{
		Enabled:        true,
		CheckInterval:  24 * time.Hour,
		AutoDownload:   false,
		AutoApply:      false,
		ReleaseChannel: update.ChannelStable,
		BackupCount:    3,
		StoragePath:    storageDir,
	}

	handler := update.NewHandler(version, updateCfg)
	otaChecker := update.NewOTAChecker(version, dataDir, "en_US")
	handler.SetOTAChecker(otaChecker)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go otaChecker.Run(ctx)

	e := echo.New()
	v1 := e.Group("/api/v1")
	handler.RegisterRoutes(v1)

	v1.GET("/meta", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"version":       version,
			"pid":           os.Getpid(),
			"uptime_ms":     update.GetUptime().Milliseconds(),
			"blue_start_ts": os.Getenv("BLUE_START_TIME"),
		})
	})

	srv := &http.Server{
		Addr:              fmt.Sprintf("127.0.0.1:%d", port),
		Handler:           e,
		ReadHeaderTimeout: 3 * time.Second,
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
		_ = srv.Shutdown(context.Background())
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}

func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		panic(fmt.Sprintf("missing env %s", k))
	}
	return v
}

func mustInt(k string) int {
	v := mustEnv(k)
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil || n <= 0 {
		panic(fmt.Sprintf("invalid int env %s=%q", k, v))
	}
	return n
}
