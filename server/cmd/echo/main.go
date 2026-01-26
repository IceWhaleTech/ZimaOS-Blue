package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/lifecycle"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/logger"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/worker"
)

var (
	version   = "0.1.0-dev"
	buildTime = "unknown"
	gitCommit = "unknown"
)

func main() {
	// Parse flags
	configPath := flag.String("config", "", "Path to config file")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *showVersion {
		fmt.Printf("ZimaOS-Echo %s\n", version)
		fmt.Printf("Build time: %s\n", buildTime)
		fmt.Printf("Git commit: %s\n", gitCommit)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	if err := logger.Init(&cfg.Log); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	logger.Info().
		Str("version", version).
		Str("build_time", buildTime).
		Str("git_commit", gitCommit).
		Msg("Starting ZimaOS-Echo")

	// Initialize lifecycle manager
	lm := lifecycle.New()

	// Initialize worker pool
	pool := worker.NewPool(lm.Context(), cfg.Worker.PoolSize)
	logger.Info().Int("pool_size", cfg.Worker.PoolSize).Msg("Worker pool initialized")

	// Initialize HTTP server
	srv := server.New(&cfg.Server)
	srv.RegisterHealthRoutes()

	// Register API routes
	registerAPIRoutes(srv, pool)

	// Register shutdown hook for server
	lm.RegisterShutdownHook(func(ctx context.Context) error {
		return srv.Shutdown(ctx)
	})

	// Start server in background
	lm.Go(func(ctx context.Context) {
		if err := srv.Start(); err != nil {
			logger.Error().Err(err).Msg("Server error")
		}
	})

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		logger.Info().Str("signal", sig.String()).Msg("Received shutdown signal")
	case <-lm.Done():
		logger.Info().Msg("Lifecycle manager done")
	}

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	server.SetReady(false)

	if err := lm.Shutdown(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("Shutdown error")
		os.Exit(1)
	}

	// Wait for worker pool
	if err := pool.Wait(); err != nil {
		logger.Warn().Err(err).Msg("Worker pool error during shutdown")
	}

	logger.Info().Msg("ZimaOS-Echo stopped")
}

func registerAPIRoutes(srv *server.Server, pool *worker.Pool) {
	e := srv.Echo()

	// API v1 group
	v1 := e.Group("/api/v1")

	// Worker stats endpoint
	v1.GET("/workers/stats", func(c echo.Context) error {
		return c.JSON(http.StatusOK, pool.Stats())
	})
}
