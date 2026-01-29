//go:build !dev

package web

import (
	"embed"
	"io"
	"io/fs"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

//go:embed dist/*
var embeddedFiles embed.FS

// distFS is the sub-filesystem for the dist directory
var distFS fs.FS

func init() {
	var err error
	distFS, err = fs.Sub(embeddedFiles, "dist")
	if err != nil {
		panic(err)
	}
}

// GetFileSystem returns the embedded file system for production builds.
// The files are embedded from the dist directory which contains the built frontend.
func GetFileSystem() http.FileSystem {
	return http.FS(distFS)
}

// GetEmbeddedFS returns the raw fs.FS for the dist directory.
func GetEmbeddedFS() fs.FS {
	return distFS
}

// IsEmbedded returns true if the web assets are embedded (production build).
func IsEmbedded() bool {
	return true
}

// RegisterStaticRoutes registers the static file routes for the embedded frontend.
func RegisterStaticRoutes(e *echo.Echo) {
	// Serve static files directly from embedded filesystem
	// This avoids Echo's routing issues with wildcard routes
	e.GET("/*", func(c echo.Context) error {
		path := c.Param("*")
		if path == "" {
			path = "index.html"
		}

		// Skip API routes
		if strings.HasPrefix(path, "api/") {
			return echo.ErrNotFound
		}

		// Try to open the file
		f, err := distFS.Open(path)
		if err != nil {
			// File not found - serve index.html for SPA routing
			// But only for non-asset paths
			if isAssetPath(path) {
				return echo.ErrNotFound
			}
			f, err = distFS.Open("index.html")
			if err != nil {
				return echo.ErrNotFound
			}
			path = "index.html"
		}
		defer f.Close()

		// Check if it's a directory
		stat, err := f.Stat()
		if err != nil {
			return echo.ErrNotFound
		}

		if stat.IsDir() {
			// Try index.html in the directory
			f.Close()
			indexPath := path + "/index.html"
			f, err = distFS.Open(indexPath)
			if err != nil {
				// Serve root index.html for SPA
				f, err = distFS.Open("index.html")
				if err != nil {
					return echo.ErrNotFound
				}
				path = "index.html"
			} else {
				path = indexPath
			}
			stat, _ = f.Stat()
		}

		// Read and serve the file
		content, err := io.ReadAll(f)
		if err != nil {
			return echo.ErrNotFound
		}

		contentType := getContentType(path)
		return c.Blob(http.StatusOK, contentType, content)
	})
}

// isAssetPath returns true if the path looks like a static asset
func isAssetPath(path string) bool {
	exts := []string{".js", ".css", ".png", ".jpg", ".jpeg", ".svg", ".ico", ".woff", ".woff2", ".ttf", ".map", ".json"}
	for _, ext := range exts {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}

// getContentType returns the content type for a file path
func getContentType(path string) string {
	switch {
	case strings.HasSuffix(path, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(path, ".js"):
		return "application/javascript; charset=utf-8"
	case strings.HasSuffix(path, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(path, ".json"):
		return "application/json; charset=utf-8"
	case strings.HasSuffix(path, ".png"):
		return "image/png"
	case strings.HasSuffix(path, ".jpg"), strings.HasSuffix(path, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(path, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(path, ".ico"):
		return "image/x-icon"
	case strings.HasSuffix(path, ".woff"):
		return "font/woff"
	case strings.HasSuffix(path, ".woff2"):
		return "font/woff2"
	case strings.HasSuffix(path, ".ttf"):
		return "font/ttf"
	case strings.HasSuffix(path, ".map"):
		return "application/json"
	default:
		return "application/octet-stream"
	}
}
