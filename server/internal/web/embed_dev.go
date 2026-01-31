//go:build dev

package web

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/labstack/echo/v4"
)

// GetFileSystem returns nil for development builds.
// In development mode, we proxy to the Vite dev server instead.
func GetFileSystem() http.FileSystem {
	return nil
}

// IsEmbedded returns false for development builds.
func IsEmbedded() bool {
	return false
}

// RegisterStaticRoutes sets up a reverse proxy to the Vite dev server for development.
func RegisterStaticRoutes(e *echo.Echo) {
	// Proxy to Vite dev server
	viteURL, _ := url.Parse("http://localhost:5173")
	proxy := httputil.NewSingleHostReverseProxy(viteURL)

	// Proxy all non-API routes to Vite
	e.Any("/*", func(c echo.Context) error {
		path := c.Request().URL.Path

		// Skip API routes
		if strings.HasPrefix(path, "/api") {
			return echo.ErrNotFound
		}

		// Legacy/incorrect request for index.js - redirect to root
		if path == "/index.js" {
			return c.Redirect(http.StatusFound, "/")
		}

		proxy.ServeHTTP(c.Response(), c.Request())
		return nil
	})
}
