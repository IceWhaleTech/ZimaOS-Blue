package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/logger"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/security"
	"golang.org/x/net/netutil"
)

// actualPort stores the actual port the server is listening on
var actualPort atomic.Int32

// GetActualPort returns the actual port the server is listening on.
// This may differ from the configured port if port_auto_fallback is enabled
// and the configured port was already in use.
func GetActualPort() int {
	return int(actualPort.Load())
}

type Server struct {
	echo   *echo.Echo
	config *config.ServerConfig
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
		echo:   e,
		config: cfg,
	}
}

func (s *Server) Echo() *echo.Echo {
	return s.echo
}

// checkExistingEchoServer checks if there's an existing ZimaOS-Echo server on the port
// Returns true if it's our server, false otherwise
func checkExistingEchoServer(host string, port int) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	url := fmt.Sprintf("http://%s:%d/health", host, port)

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

	// Our health endpoint returns {"status": "ok", "service": "zimaos-echo", ...}
	service, ok := health["service"].(string)
	return ok && service == "zimaos-echo"
}

func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	logger.Info().Str("addr", addr).Msg("Starting HTTP server")

	// Check if port is in use by another ZimaOS-Echo instance
	if checkExistingEchoServer(s.config.Host, s.config.Port) {
		logger.Info().
			Int("port", s.config.Port).
			Msg("Found existing ZimaOS-Echo server on port, will reuse")
	}

	// Create listener with SO_REUSEADDR
	lc := net.ListenConfig{
		Control: reusePort,
	}

	ln, err := lc.Listen(context.Background(), "tcp", addr)
	if err != nil {
		// If port is in use and auto fallback is enabled, try port 0 (random)
		if s.config.PortAutoFallback && isAddrInUse(err) {
			logger.Warn().
				Int("configured_port", s.config.Port).
				Err(err).
				Msg("Configured port is in use, falling back to random port")

			randomAddr := fmt.Sprintf("%s:0", s.config.Host)
			ln, err = lc.Listen(context.Background(), "tcp", randomAddr)
			if err != nil {
				return fmt.Errorf("failed to create listener on random port: %w", err)
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

	logger.Info().
		Int("actual_port", tcpAddr.Port).
		Int("configured_port", s.config.Port).
		Msg("Server listening")

	// Limit concurrent connections to prevent resource exhaustion
	ln = netutil.LimitListener(ln, 10000)

	server := &http.Server{
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
		IdleTimeout:  s.config.IdleTimeout,
	}

	s.echo.Listener = ln
	return s.echo.StartServer(server)
}

// isAddrInUse checks if the error indicates the address is already in use
func isAddrInUse(err error) bool {
	return strings.Contains(err.Error(), "address already in use")
}

func (s *Server) Shutdown(ctx context.Context) error {
	logger.Info().Msg("Shutting down HTTP server")
	return s.echo.Shutdown(ctx)
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
