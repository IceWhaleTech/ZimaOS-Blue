package server

import (
	"fmt"
	"net/http"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/labstack/echo/v4"
)

type HealthStatus struct {
	Status        string    `json:"status"`
	Service       string    `json:"service"`
	Timestamp     time.Time `json:"timestamp"`
	Uptime        string    `json:"uptime"`
	UptimeSeconds float64   `json:"uptime_seconds"`
	Version       string    `json:"version"`
	GoVersion     string    `json:"go_version"`
	NumCPU        int       `json:"num_cpu"`
	Goroutines    int       `json:"goroutines"`
	MemAlloc      uint64    `json:"mem_alloc_bytes"`
}

var (
	startTime   = time.Now()
	version     = "0.10.17"
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

// RegisterHealthRoutesOnGroup registers health routes on an echo.Group (e.g., /api/v1).
func (s *Server) RegisterHealthRoutesOnGroup(g *echo.Group) {
	g.GET("/health", healthHandler)
	g.GET("/health/live", livenessHandler)
	g.GET("/health/ready", readinessHandler)
}

func healthHandler(c echo.Context) error {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	uptime := time.Since(startTime)
	status := HealthStatus{
		Status:        "ok",
		Service:       "zimaos-blue",
		Timestamp:     time.Now(),
		Uptime:        formatUptime(uptime),
		UptimeSeconds: uptime.Seconds(),
		Version:       version,
		GoVersion:     runtime.Version(),
		NumCPU:        runtime.NumCPU(),
		Goroutines:    runtime.NumGoroutine(),
		MemAlloc:      m.Alloc,
	}

	return c.JSON(http.StatusOK, status)
}

// formatUptime formats duration as "Xd Xh Xm Xs" with 2 decimal places for seconds
func formatUptime(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := d.Seconds() - float64(int(d.Seconds())/60*60)

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm %.2fs", days, hours, minutes, seconds)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm %.2fs", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %.2fs", minutes, seconds)
	}
	return fmt.Sprintf("%.2fs", seconds)
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
