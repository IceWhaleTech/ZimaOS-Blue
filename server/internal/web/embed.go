package web

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
)

var (
	distFS   fs.FS
	distDir  string
	distOnce sync.Once
)

func startupAssetTraceEnabled() bool {
	for _, key := range []string{"ZIMAOS_STARTUP_TRACE", "BLUE_STARTUP_TRACE"} {
		switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
		case "1", "true", "yes", "on":
			return true
		}
	}
	return false
}

func shouldLogStartupAsset(path string) bool {
	if path == "index.html" {
		return true
	}
	if !isVersionedAsset(path) {
		return false
	}
	return strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".css")
}

func extractDir() string {
	if runtime.GOOS == "linux" {
		if info, err := os.Stat("/dev/shm"); err == nil && info.IsDir() {
			return "/dev/shm"
		}
	}
	return os.TempDir()
}

// ensureDistFS locates the frontend dist directory.
// Checks local candidates first (fast path for Tauri/dev), then tries
// extracting appended tar.gz (Linux pack-dist builds).
func ensureDistFS() {
	distOnce.Do(func() {
		// Fast path: check local dist/ candidates first (Tauri bundle, dev layout)
		for _, candidate := range localDistCandidates() {
			if info, err := os.Stat(filepath.Join(candidate, "index.html")); err == nil && !info.IsDir() {
				distFS = os.DirFS(candidate)
				log.Printf("[web] serving dist from local: %s", candidate)
				return
			}
		}
		// Slow path: try extracting appended tar.gz (Linux/Windows pack-dist)
		if tryExtractAppended() {
			return
		}
		log.Printf("[web] no dist available (build with 'make build' to embed)")
	})
}

func tryExtractAppended() bool {
	exe, err := os.Executable()
	if err != nil {
		log.Printf("[web] os.Executable: %v", err)
		return false
	}
	f, err := os.Open(exe)
	if err != nil {
		log.Printf("[web] open self: %v", err)
		return false
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return false
	}

	layout, ok := readAppendedLayoutFromReader(f, fi.Size())
	if !ok {
		return false
	}

	section := io.NewSectionReader(f, layout.offset, layout.dataEnd-layout.offset)

	base := filepath.Join(extractDir(), "zimaos-blue-dist")
	if err := extractTarGzFromReader(section, base); err != nil {
		log.Printf("[web] extract: %v", err)
		return false
	}
	distDir = base
	distFS = os.DirFS(base)
	log.Printf("[web] dist extracted to %s", base)
	return true
}

func localDistCandidates() []string {
	var candidates []string
	// Relative to working directory
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(wd, ".dist"),
			filepath.Join(wd, "internal", "web", "dist"),
			filepath.Join(wd, "server", "internal", "web", "dist"),
			filepath.Join(wd, "web", "dist"),       // Project root: web/dist (Vite output)
			filepath.Join(wd, "..", "web", "dist"), // Running from server/: ../web/dist
		)
	}
	// Relative to executable
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(dir, ".dist"),                         // Launcher sidecar: bin/.dist (hidden)
			filepath.Join(dir, "dist"),                          // Sidecar: bin/dist (legacy)
			filepath.Join(dir, "..", "internal", "web", "dist"), // Dev layout
			filepath.Join(dir, "internal", "web", "dist"),
		)
		// macOS .app bundle: Contents/MacOS/exe → Contents/Resources/dist
		if runtime.GOOS == "darwin" {
			candidates = append(candidates,
				filepath.Join(dir, "..", "Resources", "dist"),
			)
		}
		// Windows Tauri bundle: check multiple locations
		if runtime.GOOS == "windows" {
			candidates = append(candidates,
				filepath.Join(dir, "dist"),       // Next to exe
				filepath.Join(dir, "..", "dist"), // Parent directory
			)
		}
	}
	return candidates
}

func extractTarGzFromReader(r io.Reader, dst string) error {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		target := filepath.Join(dst, hdr.Name)
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(dst)) {
			continue
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, 0o755)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), 0o755)
			out, err := os.Create(target)
			if err != nil {
				return err
			}
			io.Copy(out, tr)
			out.Close()
		}
	}
	return nil
}

// CleanupDist removes the extracted dist directory.
func CleanupDist() {
	if distDir != "" {
		os.RemoveAll(distDir)
	}
}

func GetFileSystem() http.FileSystem {
	ensureDistFS()
	if distFS == nil {
		return nil
	}
	return http.FS(distFS)
}

func GetEmbeddedFS() fs.FS {
	ensureDistFS()
	return distFS
}

func IsEmbedded() bool {
	return true
}

func RegisterStaticRoutes(e *echo.Echo) {
	ensureDistFS()
	e.GET("/*", func(c echo.Context) error {
		started := time.Now()
		requestedPath := c.Param("*")
		if distFS == nil {
			return echo.ErrNotFound
		}
		path := requestedPath
		if path == "" {
			path = "index.html"
		}
		if strings.HasPrefix(path, "api/") {
			return echo.ErrNotFound
		}
		if path == "index.js" {
			return c.Redirect(http.StatusFound, "/")
		}

		content, err := fs.ReadFile(distFS, path)
		if err != nil {
			if isAssetPath(path) {
				return echo.ErrNotFound
			}
			content, err = fs.ReadFile(distFS, "index.html")
			if err != nil {
				return echo.ErrNotFound
			}
			path = "index.html"
		}

		// Set cache headers based on file type
		// Strategy:
		// 1. Images: cache for 24 hours
		// 2. Versioned assets (JS/CSS with hash): cache forever (immutable)
		// 3. All other files (HTML, non-versioned JS/CSS, JSON): no-cache
		if isImageFile(path) {
			// Images can be cached for a reasonable time
			c.Response().Header().Set("Cache-Control", "public, max-age=86400") // 24 hours
		} else if isVersionedAsset(path) {
			// Versioned assets (with hash in filename) can be cached forever
			// These files have content hashes and will never change
			c.Response().Header().Set("Cache-Control", "public, max-age=31536000, immutable") // 1 year
		} else {
			// All other files: no cache to ensure users always get latest version
			c.Response().Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			c.Response().Header().Set("Pragma", "no-cache")
			c.Response().Header().Set("Expires", "0")
		}

		if startupAssetTraceEnabled() && shouldLogStartupAsset(path) {
			log.Printf(
				"[web][startup-asset] request=%q served=%q bytes=%d duration_ms=%d",
				requestedPath,
				path,
				len(content),
				time.Since(started).Milliseconds(),
			)
		}

		return c.Blob(http.StatusOK, getContentType(path), content)
	})
}
