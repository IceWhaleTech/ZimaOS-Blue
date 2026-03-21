package onnx

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/network"
)

const ortVersion = "1.24.1"

var onnxRuntimeHTTPClient = network.NewPooledHTTPClient(10 * time.Minute)

// RuntimeLibPath returns the path to the ONNX Runtime shared library
// in the given data directory, or empty string if not present.
func RuntimeLibPath(dataDir string) string {
	p := filepath.Join(dataDir, "onnxruntime", libName())
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return ""
}

// RuntimeTgzURL returns the primary GitHub URL for the ONNX Runtime tgz.
func RuntimeTgzURL() string {
	fn := runtimeFilename()
	if fn == "" {
		return ""
	}
	return "https://github.com/microsoft/onnxruntime/releases/download/v" + ortVersion + "/" + fn
}

// RuntimeTgzMirrors returns CDN mirror URLs for the ONNX Runtime tgz.
func RuntimeTgzMirrors() []string {
	ghURL := RuntimeTgzURL()
	if ghURL == "" {
		return nil
	}
	return []string{
		"https://mirror.ghproxy.com/" + ghURL,
		"https://gh-proxy.com/" + ghURL,
	}
}

// RuntimeTgzFilename returns the platform-specific tgz filename.
func RuntimeTgzFilename() string {
	return runtimeFilename()
}

// ExtractRuntimeFromTgz extracts the ONNX Runtime dylib from a downloaded tgz file
// into the onnxruntime subdirectory under dataDir. Returns the dylib path.
func ExtractRuntimeFromTgz(tgzPath, dataDir string) (string, error) {
	destDir := filepath.Join(dataDir, "onnxruntime")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", err
	}

	f, err := os.Open(tgzPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if err := extractLib(f, destDir, libName()); err != nil {
		return "", err
	}

	// Remove the tgz after extraction
	os.Remove(tgzPath)

	libPath := filepath.Join(destDir, libName())
	log.Printf("[onnx] extracted ONNX Runtime to %s", libPath)
	return libPath, nil
}

// EnsureRuntime downloads the ONNX Runtime shared library if not present.
// Returns the path to the library file.
func EnsureRuntime(dataDir string) (string, error) {
	destDir := filepath.Join(dataDir, "onnxruntime")
	libPath := filepath.Join(destDir, libName())

	if _, err := os.Stat(libPath); err == nil {
		return libPath, nil
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("create onnxruntime dir: %w", err)
	}

	urls := runtimeURLs()
	if len(urls) == 0 {
		return "", fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	var lastErr error
	for _, url := range urls {
		log.Printf("[onnx] trying %s", url)
		resp, err := onnxRuntimeHTTPClient.Get(url)
		if err != nil {
			lastErr = err
			log.Printf("[onnx] failed: %v", err)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
			log.Printf("[onnx] %v", lastErr)
			continue
		}

		err = extractLib(resp.Body, destDir, libName())
		resp.Body.Close()
		if err != nil {
			lastErr = err
			log.Printf("[onnx] extract failed: %v", err)
			continue
		}

		if _, err := os.Stat(libPath); err == nil {
			log.Printf("[onnx] downloaded ONNX Runtime to %s", libPath)
			return libPath, nil
		}
		lastErr = fmt.Errorf("library not found after extraction")
	}
	return "", fmt.Errorf("all download mirrors failed: %w", lastErr)
}

// runtimeURLs returns download URLs with CDN mirrors first for acceleration.
func runtimeURLs() []string {
	filename := runtimeFilename()
	if filename == "" {
		return nil
	}

	ghURL := "https://github.com/microsoft/onnxruntime/releases/download/v" + ortVersion + "/" + filename
	return []string{
		// China-friendly mirrors first
		"https://mirror.ghproxy.com/" + ghURL,
		"https://gh-proxy.com/" + ghURL,
		// GitHub direct
		ghURL,
	}
}

func runtimeFilename() string {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "darwin/arm64":
		return "onnxruntime-osx-arm64-" + ortVersion + ".tgz"
	case "linux/amd64":
		return "onnxruntime-linux-x64-" + ortVersion + ".tgz"
	case "linux/arm64":
		return "onnxruntime-linux-aarch64-" + ortVersion + ".tgz"
	}
	return ""
}

// extractLib extracts the shared library from the onnxruntime tgz archive.
// The tgz contains: onnxruntime-<platform>/lib/libonnxruntime.dylib (or .so)
func extractLib(r io.Reader, destDir, libFileName string) error {
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

		base := filepath.Base(hdr.Name)
		if hdr.Typeflag == tar.TypeReg && strings.HasPrefix(base, "libonnxruntime") {
			outPath := filepath.Join(destDir, libFileName)
			out, err := os.Create(outPath)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				os.Remove(outPath)
				return err
			}
			out.Close()
			os.Chmod(outPath, 0755)
			return nil
		}
	}
	return fmt.Errorf("library %s not found in archive", libFileName)
}
