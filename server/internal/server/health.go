package server

import (
	"net/http"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/labstack/echo/v4"
)

type HealthStatus struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Uptime    string    `json:"uptime"`
	Version   string    `json:"version"`
	GoVersion string    `json:"go_version"`
	NumCPU    int       `json:"num_cpu"`
	Goroutines int      `json:"goroutines"`
	MemAlloc  uint64    `json:"mem_alloc_bytes"`
}

var (
	startTime   = time.Now()
	version     = "0.1.0-dev"
	readyStatus atomic.Bool
)

func init() {
	readyStatus.Store(true)
}

func SetReady(ready bool) {
	readyStatus.Store(ready)
}

func SetVersion(v string) {
	version = v
}

func (s *Server) RegisterHealthRoutes() {
	s.echo.GET("/health", healthHandler)
	s.echo.GET("/health/live", livenessHandler)
	s.echo.GET("/health/ready", readinessHandler)
}

func healthHandler(c echo.Context) error {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	status := HealthStatus{
		Status:     "ok",
		Timestamp:  time.Now(),
		Uptime:     time.Since(startTime).String(),
		Version:    version,
		GoVersion:  runtime.Version(),
		NumCPU:     runtime.NumCPU(),
		Goroutines: runtime.NumGoroutine(),
		MemAlloc:   m.Alloc,
	}

	return c.JSON(http.StatusOK, status)
}

func livenessHandler(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "alive"})
}

func readinessHandler(c echo.Context) error {
	if readyStatus.Load() {
		return c.JSON(http.StatusOK, map[string]string{"status": "ready"})
	}
	return c.JSON(http.StatusServiceUnavailable, map[string]string{"status": "not ready"})
}
