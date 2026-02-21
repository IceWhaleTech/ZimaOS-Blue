package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/logger"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/network"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/security"
	"golang.org/x/net/netutil"
)

// actualPort stores the actual port the server is listening on
var actualPort atomic.Int32

// globalServer holds a reference to the running server for dynamic TLS start
var globalServer atomic.Pointer[Server]

// SetGlobalServer stores the server instance for dynamic TLS operations.
func SetGlobalServer(s *Server) { globalServer.Store(s) }

// GetGlobalServer returns the running server instance (may be nil).
func GetGlobalServer() *Server { return globalServer.Load() }

// onServerStartCallbacks stores callbacks to run after server starts
var onServerStartCallbacks []func(port int)
var onServerStartMu sync.Mutex

// GetActualPort returns the actual port the server is listening on.
// This may differ from the configured port if port_auto_fallback is enabled
// and the configured port was already in use.
func GetActualPort() int {
	return int(actualPort.Load())
}

// SetActualPort sets the actual port (used by embedded/bluelib mode).
func SetActualPort(port int) {
	actualPort.Store(int32(port))
}

// OnServerStart registers a callback to be called after the server starts
// and the actual port is known.
func OnServerStart(callback func(port int)) {
	onServerStartMu.Lock()
	defer onServerStartMu.Unlock()
	onServerStartCallbacks = append(onServerStartCallbacks, callback)
}

type Server struct {
	echo           *echo.Echo
	config         *config.ServerConfig
	shutdownChan   chan struct{}
	httpServer     *http.Server
	shutdownMu     sync.Mutex
	isShuttingDown bool
	tlsStarted    atomic.Bool
}

func New(cfg *config.ServerConfig) *Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Disable trailing slash redirect to prevent redirect loops with static files
	e.Pre(middleware.RemoveTrailingSlash())

	// Middleware
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(zerologMiddleware())
	// Configure CORS with dynamic origin validation
	// Origins are validated at runtime to support dynamic port binding
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOriginFunc: func(origin string) (bool, error) {
			return security.CheckOriginDefault(&http.Request{
				Header: http.Header{"Origin": []string{origin}},
			}), nil
		},
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
		AllowCredentials: true,
	}))

	return &Server{
		echo:         e,
		config:       cfg,
		shutdownChan: make(chan struct{}),
	}
}

func (s *Server) Echo() *echo.Echo {
	return s.echo
}

// checkExistingServer checks if there's an existing ZimaOS-Blue server on the port
// Returns true if it's our server, false otherwise
func checkExistingServer(host string, port int) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	url := fmt.Sprintf("http://%s:%d/api/v1/health", host, port)

	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}

	// Check if the response contains our service identifier
	var health map[string]interface{}
	if err := json.Unmarshal(body, &health); err != nil {
		return false
	}

	// Our health endpoint returns {"status": "ok", "service": "zimaos-blue", ...}
	service, ok := health["service"].(string)
	return ok && service == "zimaos-blue"
}

// requestGracefulShutdown requests the existing server to shutdown gracefully
func requestGracefulShutdown(host string, port int) bool {
	client := &http.Client{Timeout: 5 * time.Second}
	url := fmt.Sprintf("http://%s:%d/api/v1/shutdown", host, port)

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	// Wait a bit for the old server to shutdown
	time.Sleep(1 * time.Second)
	return true
}

func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	logger.Info().Str("addr", addr).Msg("Starting HTTP server")

	// Create listener with SO_REUSEADDR
	lc := net.ListenConfig{
		Control: reusePort,
	}

	ln, err := lc.Listen(context.Background(), "tcp", addr)
	if err != nil {
		if isAddrInUse(err) {
			// Check if it's our own previous instance
			if checkExistingServer(s.config.Host, s.config.Port) {
				logger.Info().
					Int("port", s.config.Port).
					Msg("Detected existing ZimaOS-Blue server, requesting graceful shutdown")

				// Request graceful shutdown of the old instance
				if requestGracefulShutdown(s.config.Host, s.config.Port) {
					logger.Info().Msg("Old instance shutdown successfully, reusing port")

					// Retry listening after old instance shutdown
					ln, err = lc.Listen(context.Background(), "tcp", addr)
					if err != nil {
						return fmt.Errorf("failed to listen after graceful shutdown: %w", err)
					}
				} else {
					logger.Warn().Msg("Failed to shutdown old instance, will reuse connection")
					// Old behavior: just reuse the existing server
					actualPort.Store(int32(s.config.Port))
					security.SetServerPort(s.config.Port)
					network.SetDynamicPort(s.config.Port)
					return nil
				}
			} else {
				// Not our server — fallback to random port if enabled
				if s.config.PortAutoFallback {
					logger.Warn().
						Int("configured_port", s.config.Port).
						Msg("Port in use by another process, falling back to random port")
					randomAddr := fmt.Sprintf("%s:0", s.config.Host)
					ln, err = lc.Listen(context.Background(), "tcp", randomAddr)
					if err != nil {
						return fmt.Errorf("failed to create listener on random port: %w", err)
					}
				} else {
					return fmt.Errorf("failed to create listener: %w", err)
				}
			}
		} else {
			return fmt.Errorf("failed to create listener: %w", err)
		}
	}

	// Get the actual port from the listener
	tcpAddr := ln.Addr().(*net.TCPAddr)
	actualPort.Store(int32(tcpAddr.Port))

	// Update security package with actual port for dynamic CORS origins
	security.SetServerPort(tcpAddr.Port)

	// Update network package with actual port for address detection
	network.SetDynamicPort(tcpAddr.Port)

	// Execute registered callbacks with actual port
	onServerStartMu.Lock()
	callbacks := make([]func(int), len(onServerStartCallbacks))
	copy(callbacks, onServerStartCallbacks)
	onServerStartMu.Unlock()
	for _, cb := range callbacks {
		cb(tcpAddr.Port)
	}

	logger.Info().
		Int("actual_port", tcpAddr.Port).
		Int("configured_port", s.config.Port).
		Msg("Server listening")

	logger.Info().
		Str("url", fmt.Sprintf("http://localhost:%d", tcpAddr.Port)).
		Msg("Open in browser to access the web interface")

	// Limit concurrent connections to prevent resource exhaustion
	ln = netutil.LimitListener(ln, 10000)

	server := &http.Server{
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
		IdleTimeout:  s.config.IdleTimeout,
	}

	// Store server reference for graceful shutdown
	s.httpServer = server

	s.echo.Listener = ln
	return s.echo.StartServer(server)
}

// StartTLS starts an HTTPS server on the configured TLS port using the global TLSManager.
// It creates a separate TLS listener that shares the same Echo router as the HTTP server.
func (s *Server) StartTLS() error {
	tlsManager := security.GetGlobalTLSManager()
	if tlsManager == nil {
		return fmt.Errorf("TLS manager not initialized")
	}

	cert := tlsManager.GetCertificate()
	if cert == nil {
		return fmt.Errorf("no TLS certificate loaded")
	}

	httpsPort := tlsManager.GetHTTPSPort()
	addr := fmt.Sprintf("%s:%d", s.config.Host, httpsPort)
	logger.Info().Str("addr", addr).Msg("Starting HTTPS server")

	// Create TLS listener
	tlsConfig := tlsManager.GetTLSConfig()
	ln, err := tls.Listen("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to create TLS listener on %s: %w", addr, err)
	}

	ln = netutil.LimitListener(ln, 10000)

	logger.Info().
		Int("https_port", httpsPort).
		Msg("HTTPS server listening")

	// Serve using the same Echo handler on the TLS listener
	httpsServer := &http.Server{
		Handler:      s.echo,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
		IdleTimeout:  s.config.IdleTimeout,
	}

	return httpsServer.Serve(ln)
}

// EnsureTLSStarted starts the HTTPS listener if a certificate is available
// and it hasn't been started yet.  Safe to call multiple times — only the
// first successful call actually starts the listener.
func (s *Server) EnsureTLSStarted() {
	if s.tlsStarted.Load() {
		return
	}
	tlsManager := security.GetGlobalTLSManager()
	if tlsManager == nil || tlsManager.GetCertificate() == nil {
		return
	}
	if !s.tlsStarted.CompareAndSwap(false, true) {
		return // another goroutine won the race
	}
	go func() {
		if err := s.StartTLS(); err != nil {
			logger.Error().Err(err).Msg("Dynamic HTTPS server error")
			s.tlsStarted.Store(false) // allow retry
		}
	}()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	s.shutdownMu.Lock()
	if s.isShuttingDown {
		s.shutdownMu.Unlock()
		return fmt.Errorf("shutdown already in progress")
	}
	s.isShuttingDown = true
	s.shutdownMu.Unlock()

	logger.Info().Msg("Initiating graceful shutdown")

	// Signal shutdown
	close(s.shutdownChan)

	// Shutdown HTTP server if it exists
	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			logger.Error().Err(err).Msg("Error during HTTP server shutdown")
			return err
		}
	}

	logger.Info().Msg("Server shutdown complete")
	return nil
}

// RegisterShutdownRoute registers the graceful shutdown endpoint
func (s *Server) RegisterShutdownRoute() {
	s.echo.POST("/api/v1/shutdown", func(c echo.Context) error {
		logger.Info().Msg("Received shutdown request")

		// Start shutdown in background
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if err := s.Shutdown(ctx); err != nil {
				logger.Error().Err(err).Msg("Shutdown failed")
			}
		}()

		return c.JSON(http.StatusOK, map[string]interface{}{
			"status":  "ok",
			"message": "Server shutting down gracefully",
		})
	})
}

// isAddrInUse checks if the error indicates the address is already in use
func isAddrInUse(err error) bool {
	return strings.Contains(err.Error(), "address already in use")
}

func zerologMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			res := c.Response()
			start := time.Now()

			err := next(c)
			if err != nil {
				c.Error(err)
			}

			latency := time.Since(start)
			status := res.Status

			// Choose log level based on status code
			var event *zerolog.Event
			switch {
			case status >= 500:
				event = logger.Error()
			case status >= 400:
				event = logger.Warn()
			default:
				event = logger.Info()
			}

			event.
				Str("tag", "HTTP").
				Str("method", req.Method).
				Str("uri", req.RequestURI).
				Int("status", status).
				Dur("latency", latency).
				Str("remote_ip", c.RealIP()).
				Int64("bytes_in", req.ContentLength).
				Int64("bytes_out", res.Size).
				Str("user_agent", req.UserAgent()).
				Str("request_id", res.Header().Get(echo.HeaderXRequestID)).
				Msg("request")

			return nil
		}
	}
}
