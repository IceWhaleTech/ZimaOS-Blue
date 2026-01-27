package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/logger"
	"golang.org/x/net/netutil"
)

type Server struct {
	echo   *echo.Echo
	config *config.ServerConfig
}

func New(cfg *config.ServerConfig) *Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Middleware
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(zerologMiddleware())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
	}))

	return &Server{
		echo:   e,
		config: cfg,
	}
}

func (s *Server) Echo() *echo.Echo {
	return s.echo
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
		return fmt.Errorf("failed to create listener: %w", err)
	}

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
