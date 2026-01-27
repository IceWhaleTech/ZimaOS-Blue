//go:build !dev

package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/labstack/echo/v4"
)

//go:embed dist/*
var embeddedFiles embed.FS

// GetFileSystem returns the embedded file system for production builds.
// The files are embedded from the dist directory which contains the built frontend.
func GetFileSystem() http.FileSystem {
	fsys, err := fs.Sub(embeddedFiles, "dist")
	if err != nil {
		panic(err)
	}
	return http.FS(fsys)
}

// IsEmbedded returns true if the web assets are embedded (production build).
func IsEmbedded() bool {
	return true
}

// RegisterStaticRoutes registers the static file routes for the embedded frontend.
func RegisterStaticRoutes(e *echo.Echo) {
	fsys := GetFileSystem()
	fileServer := http.FileServer(fsys)

	// Handle all routes that are not API routes
	e.GET("/*", func(c echo.Context) error {
		reqPath := c.Request().URL.Path

		// Skip API routes - they should be handled by other handlers
		if strings.HasPrefix(reqPath, "/api") {
			return echo.ErrNotFound
		}

		// Clean the path
		cleanPath := path.Clean(reqPath)
		if cleanPath == "/" {
			cleanPath = "/index.html"
		}

		// Try to open the file
		f, err := fsys.Open(cleanPath)
		if err != nil {
			// File doesn't exist, serve index.html for SPA routing
			// This handles client-side routing (e.g., /chat, /settings, etc.)
			c.Request().URL.Path = "/index.html"
		} else {
			// Check if it's a directory
			stat, statErr := f.Stat()
			f.Close()
			if statErr == nil && stat.IsDir() {
				c.Request().URL.Path = path.Join(cleanPath, "index.html")
			}
		}

		// Serve the file
		fileServer.ServeHTTP(c.Response(), c.Request())
		return nil
	})
}
