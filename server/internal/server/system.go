package server

import (
	"net/http"
	"runtime"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/resources"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/scheduler"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/watcher"
)

// SystemStatus represents the complete system status.
type SystemStatus struct {
	Status     string                   `json:"status"`
	Timestamp  time.Time                `json:"timestamp"`
	Uptime     string                   `json:"uptime"`
	Version    string                   `json:"version"`
	Runtime    RuntimeStatus            `json:"runtime"`
	Resources  *resources.ResourceStats `json:"resources,omitempty"`
	Scheduler  *scheduler.SchedulerStats `json:"scheduler,omitempty"`
	Watcher    *WatcherStatus           `json:"watcher,omitempty"`
}

// RuntimeStatus holds Go runtime information.
type RuntimeStatus struct {
	GoVersion    string `json:"go_version"`
	NumCPU       int    `json:"num_cpu"`
	GOMAXPROCS   int    `json:"gomaxprocs"`
	NumGoroutine int    `json:"num_goroutine"`
	MemAllocMB   uint64 `json:"mem_alloc_mb"`
	MemSysMB     uint64 `json:"mem_sys_mb"`
	NumGC        uint32 `json:"num_gc"`
}

// WatcherStatus holds file watcher status.
type WatcherStatus struct {
	Enabled      bool     `json:"enabled"`
	WatchedPaths []string `json:"watched_paths"`
}

// SystemStatusHandler holds dependencies for system status endpoints.
type SystemStatusHandler struct {
	resourceLimiter *resources.Limiter
	scheduler       *scheduler.Scheduler
	watcher         *watcher.Watcher
}

// NewSystemStatusHandler creates a new system status handler.
func NewSystemStatusHandler(
	limiter *resources.Limiter,
	sched *scheduler.Scheduler,
	watch *watcher.Watcher,
) *SystemStatusHandler {
	return &SystemStatusHandler{
		resourceLimiter: limiter,
		scheduler:       sched,
		watcher:         watch,
	}
}

// RegisterSystemRoutes registers system status routes.
func (h *SystemStatusHandler) RegisterSystemRoutes(e *echo.Echo) {
	g := e.Group("/api/v1/system")
	g.GET("/status", h.getSystemStatus)
	g.GET("/resources", h.getResourceStatus)
}

// getSystemStatus returns the complete system status.
func (h *SystemStatusHandler) getSystemStatus(c echo.Context) error {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	status := SystemStatus{
		Status:    "ok",
		Timestamp: time.Now(),
		Uptime:    time.Since(startTime).String(),
		Version:   version,
		Runtime: RuntimeStatus{
			GoVersion:    runtime.Version(),
			NumCPU:       runtime.NumCPU(),
			GOMAXPROCS:   runtime.GOMAXPROCS(0),
			NumGoroutine: runtime.NumGoroutine(),
			MemAllocMB:   m.Alloc / 1024 / 1024,
			MemSysMB:     m.Sys / 1024 / 1024,
			NumGC:        m.NumGC,
		},
	}

	// Add resource stats if available
	if h.resourceLimiter != nil {
		stats := h.resourceLimiter.Stats()
		status.Resources = &stats
	}

	// Add scheduler stats if available
	if h.scheduler != nil {
		stats := h.scheduler.Stats()
		status.Scheduler = &stats
	}

	// Add watcher status if available
	if h.watcher != nil {
		status.Watcher = &WatcherStatus{
			Enabled:      h.watcher.Config().Enabled,
			WatchedPaths: h.watcher.WatchedPaths(),
		}
	}

	return c.JSON(http.StatusOK, status)
}

// getResourceStatus returns resource usage details.
func (h *SystemStatusHandler) getResourceStatus(c echo.Context) error {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	response := map[string]interface{}{
		"memory": map[string]interface{}{
			"alloc_mb":       m.Alloc / 1024 / 1024,
			"total_alloc_mb": m.TotalAlloc / 1024 / 1024,
			"sys_mb":         m.Sys / 1024 / 1024,
			"heap_alloc_mb":  m.HeapAlloc / 1024 / 1024,
			"heap_sys_mb":    m.HeapSys / 1024 / 1024,
			"heap_idle_mb":   m.HeapIdle / 1024 / 1024,
			"heap_inuse_mb":  m.HeapInuse / 1024 / 1024,
			"stack_inuse_mb": m.StackInuse / 1024 / 1024,
		},
		"gc": map[string]interface{}{
			"num_gc":           m.NumGC,
			"pause_total_ns":   m.PauseTotalNs,
			"last_pause_ns":    m.PauseNs[(m.NumGC+255)%256],
			"gc_cpu_fraction":  m.GCCPUFraction,
		},
		"goroutines": runtime.NumGoroutine(),
		"cpu": map[string]interface{}{
			"num_cpu":    runtime.NumCPU(),
			"gomaxprocs": runtime.GOMAXPROCS(0),
		},
	}

	// Add rlimits if available
	rlimits, err := resources.GetRlimits()
	if err == nil && len(rlimits) > 0 {
		response["rlimits"] = rlimits
	}

	return c.JSON(http.StatusOK, response)
}
