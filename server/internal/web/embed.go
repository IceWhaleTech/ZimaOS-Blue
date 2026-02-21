//go:build !dev

package web

import (
	"archive/tar"
	"compress/gzip"
	"encoding/binary"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/labstack/echo/v4"
)

var (
	distFS   fs.FS
	distDir  string
	distOnce sync.Once
)

func extractDir() string {
	if runtime.GOOS == "linux" {
		if info, err := os.Stat("/dev/shm"); err == nil && info.IsDir() {
			return "/dev/shm"
		}
	}
	return os.TempDir()
}

// ensureDistFS extracts the appended dist.tar.gz from the binary itself.
// Layout: [ELF/Mach-O binary][dist.tar.gz][8-byte LE offset of tar.gz start]
// Falls back to local dist/ directory if extraction fails (e.g. go run / go build without make).
func ensureDistFS() {
	distOnce.Do(func() {
		if tryExtractAppended() {
			return
		}
		// Fallback: serve from local dist/ relative to executable or working directory
		for _, candidate := range localDistCandidates() {
			if info, err := os.Stat(filepath.Join(candidate, "index.html")); err == nil && !info.IsDir() {
				distFS = os.DirFS(candidate)
				log.Printf("[web] serving dist from local: %s", candidate)
				return
			}
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

	// Read last 8 bytes: tar.gz start offset
	if _, err := f.Seek(-8, io.SeekEnd); err != nil {
		return false
	}
	var offset int64
	if err := binary.Read(f, binary.LittleEndian, &offset); err != nil {
		return false
	}
	// Sanity check: offset must be positive and less than file size
	fi, err := f.Stat()
	if err != nil || offset <= 0 || offset >= fi.Size()-8 {
		return false
	}

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return false
	}

	base := filepath.Join(extractDir(), "zimaos-blue-dist")
	if err := extractTarGzFromReader(f, base); err != nil {
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
			filepath.Join(wd, "internal", "web", "dist"),
			filepath.Join(wd, "server", "internal", "web", "dist"),
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
				filepath.Join(dir, "dist"),           // Next to exe
				filepath.Join(dir, "..", "dist"),     // Parent directory
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
		if distFS == nil {
			return echo.ErrNotFound
		}
		path := c.Param("*")
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
		// Versioned assets (with hash) get long-term caching
		// index.html gets no caching to ensure fresh content
		if path == "index.html" {
			// Never cache index.html - always fetch fresh version
			c.Response().Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			c.Response().Header().Set("Pragma", "no-cache")
			c.Response().Header().Set("Expires", "0")
		} else if isVersionedAsset(path) {
			// Versioned assets (with hash in filename) can be cached forever
			c.Response().Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			// Other files: short cache with revalidation
			c.Response().Header().Set("Cache-Control", "public, max-age=3600, must-revalidate")
		}

		return c.Blob(http.StatusOK, getContentType(path), content)
	})
}
