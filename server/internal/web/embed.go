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
func ensureDistFS() {
	distOnce.Do(func() {
		exe, err := os.Executable()
		if err != nil {
			log.Printf("[web] os.Executable: %v", err)
			return
		}
		f, err := os.Open(exe)
		if err != nil {
			log.Printf("[web] open self: %v", err)
			return
		}
		defer f.Close()

		// Read last 8 bytes: tar.gz start offset
		if _, err := f.Seek(-8, io.SeekEnd); err != nil {
			log.Printf("[web] seek trailer: %v", err)
			return
		}
		var offset int64
		if err := binary.Read(f, binary.LittleEndian, &offset); err != nil {
			log.Printf("[web] read trailer: %v", err)
			return
		}

		// Seek to tar.gz start and extract
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			log.Printf("[web] seek payload: %v", err)
			return
		}

		base := filepath.Join(extractDir(), "zimaos-blue-dist")
		if err := extractTarGzFromReader(f, base); err != nil {
			log.Printf("[web] extract: %v", err)
			return
		}
		distDir = base
		distFS = os.DirFS(base)
		log.Printf("[web] dist extracted to %s", base)
	})
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
		return c.Blob(http.StatusOK, getContentType(path), content)
	})
}
