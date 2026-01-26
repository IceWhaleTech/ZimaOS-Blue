package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/logger"
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

	server := &http.Server{
		Addr:         addr,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
		IdleTimeout:  s.config.IdleTimeout,
	}

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

			err := next(c)
			if err != nil {
				c.Error(err)
			}

			logger.Info().
				Str("method", req.Method).
				Str("uri", req.RequestURI).
				Int("status", res.Status).
				Str("remote_ip", c.RealIP()).
				Str("request_id", res.Header().Get(echo.HeaderXRequestID)).
				Msg("request")

			return nil
		}
	}
}
