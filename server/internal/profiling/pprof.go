package profiling

import (
	"net/http"
	"net/http/pprof"
	"strings"

	"github.com/labstack/echo/v4"
)

// Config holds profiling configuration
type Config struct {
	Enabled        bool   `yaml:"enabled"`
	EndpointPrefix string `yaml:"endpoint_prefix"`
	AuthRequired   bool   `yaml:"auth_required"`
}

// Profiler provides pprof profiling endpoints
type Profiler struct {
	config *Config
}

// New creates a new Profiler instance
func New(cfg *Config) *Profiler {
	if cfg == nil {
		cfg = &Config{
			Enabled:        false,
			EndpointPrefix: "/debug/pprof",
			AuthRequired:   true,
		}
	}

	// Ensure prefix starts with /
	if !strings.HasPrefix(cfg.EndpointPrefix, "/") {
		cfg.EndpointPrefix = "/" + cfg.EndpointPrefix
	}

	// Remove trailing slash
	cfg.EndpointPrefix = strings.TrimSuffix(cfg.EndpointPrefix, "/")

	return &Profiler{
		config: cfg,
	}
}

// RegisterRoutes registers pprof endpoints on the Echo instance
func (p *Profiler) RegisterRoutes(e *echo.Echo, authMiddleware ...echo.MiddlewareFunc) {
	if !p.config.Enabled {
		return
	}

	prefix := p.config.EndpointPrefix

	// Create a group for pprof endpoints
	var g *echo.Group
	if p.config.AuthRequired && len(authMiddleware) > 0 {
		g = e.Group(prefix, authMiddleware...)
	} else {
		g = e.Group(prefix)
	}

	// Index page
	g.GET("/", echo.WrapHandler(http.HandlerFunc(pprof.Index)))
	g.GET("", echo.WrapHandler(http.HandlerFunc(pprof.Index)))

	// Cmdline
	g.GET("/cmdline", echo.WrapHandler(http.HandlerFunc(pprof.Cmdline)))

	// Profile (CPU)
	g.GET("/profile", echo.WrapHandler(http.HandlerFunc(pprof.Profile)))

	// Symbol
	g.GET("/symbol", echo.WrapHandler(http.HandlerFunc(pprof.Symbol)))
	g.POST("/symbol", echo.WrapHandler(http.HandlerFunc(pprof.Symbol)))

	// Trace
	g.GET("/trace", echo.WrapHandler(http.HandlerFunc(pprof.Trace)))

	// Heap
	g.GET("/heap", echo.WrapHandler(pprof.Handler("heap")))

	// Goroutine
	g.GET("/goroutine", echo.WrapHandler(pprof.Handler("goroutine")))

	// Allocs
	g.GET("/allocs", echo.WrapHandler(pprof.Handler("allocs")))

	// Block
	g.GET("/block", echo.WrapHandler(pprof.Handler("block")))

	// Mutex
	g.GET("/mutex", echo.WrapHandler(pprof.Handler("mutex")))

	// Threadcreate
	g.GET("/threadcreate", echo.WrapHandler(pprof.Handler("threadcreate")))
}

// IsEnabled returns whether profiling is enabled
func (p *Profiler) IsEnabled() bool {
	return p.config.Enabled
}

// EndpointPrefix returns the configured endpoint prefix
func (p *Profiler) EndpointPrefix() string {
	return p.config.EndpointPrefix
}

// Endpoints returns a list of available profiling endpoints
func (p *Profiler) Endpoints() []string {
	if !p.config.Enabled {
		return nil
	}

	prefix := p.config.EndpointPrefix
	return []string{
		prefix + "/",
		prefix + "/cmdline",
		prefix + "/profile",
		prefix + "/symbol",
		prefix + "/trace",
		prefix + "/heap",
		prefix + "/goroutine",
		prefix + "/allocs",
		prefix + "/block",
		prefix + "/mutex",
		prefix + "/threadcreate",
	}
}
