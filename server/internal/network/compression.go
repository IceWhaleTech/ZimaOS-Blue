package network

import (
	"bufio"
	"compress/flate"
	"compress/gzip"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
)

// CompressionConfig holds compression middleware configuration.
type CompressionConfig struct {
	// Level is the compression level (1-9, where 9 is best compression)
	Level int

	// MinSize is the minimum response size to compress
	MinSize int

	// Types is the list of content types to compress
	Types []string

	// ExcludedPaths are paths that should not be compressed
	ExcludedPaths []string

	// ExcludedExtensions are file extensions that should not be compressed
	ExcludedExtensions []string
}

// DefaultCompressionConfig returns default compression configuration.
func DefaultCompressionConfig() CompressionConfig {
	return CompressionConfig{
		Level:   6,
		MinSize: 1024, // 1KB
		Types: []string{
			"text/html",
			"text/css",
			"text/plain",
			"text/javascript",
			"application/javascript",
			"application/json",
			"application/xml",
			"application/xhtml+xml",
			"image/svg+xml",
		},
		ExcludedPaths: []string{},
		ExcludedExtensions: []string{
			".png", ".jpg", ".jpeg", ".gif", ".webp",
			".mp4", ".webm", ".ogg",
			".zip", ".gz", ".br", ".zst",
		},
	}
}

// gzipPool is a pool of gzip writers.
var gzipPool = sync.Pool{
	New: func() interface{} {
		w, _ := gzip.NewWriterLevel(io.Discard, gzip.DefaultCompression)
		return w
	},
}

// flatePool is a pool of flate writers.
var flatePool = sync.Pool{
	New: func() interface{} {
		w, _ := flate.NewWriter(io.Discard, flate.DefaultCompression)
		return w
	},
}

// compressedResponseWriter wraps http.ResponseWriter with compression.
type compressedResponseWriter struct {
	http.ResponseWriter
	writer      io.Writer
	encoding    string
	config      CompressionConfig
	wroteHeader bool
	statusCode  int
	buffer      []byte
}

func (w *compressedResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.wroteHeader = true
}

func (w *compressedResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}

	// Buffer small responses
	if len(w.buffer)+len(b) < w.config.MinSize {
		w.buffer = append(w.buffer, b...)
		return len(b), nil
	}

	// Flush buffer if needed
	if len(w.buffer) > 0 {
		w.flushBuffer()
	}

	return w.writer.Write(b)
}

func (w *compressedResponseWriter) flushBuffer() {
	if len(w.buffer) == 0 {
		return
	}

	// Check if we should compress
	contentType := w.Header().Get("Content-Type")
	if w.shouldCompress(contentType) && len(w.buffer) >= w.config.MinSize {
		w.Header().Set("Content-Encoding", w.encoding)
		w.Header().Del("Content-Length")
	} else {
		// Don't compress, write directly
		w.writer = w.ResponseWriter
	}

	w.ResponseWriter.WriteHeader(w.statusCode)
	w.writer.Write(w.buffer)
	w.buffer = nil
}

func (w *compressedResponseWriter) shouldCompress(contentType string) bool {
	if contentType == "" {
		return false
	}

	// Extract base content type
	if idx := strings.Index(contentType, ";"); idx != -1 {
		contentType = contentType[:idx]
	}
	contentType = strings.TrimSpace(contentType)

	for _, t := range w.config.Types {
		if strings.EqualFold(contentType, t) {
			return true
		}
	}
	return false
}

func (w *compressedResponseWriter) Close() error {
	// Flush any remaining buffer
	if len(w.buffer) > 0 {
		w.flushBuffer()
	}

	if closer, ok := w.writer.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// Flush implements http.Flusher.
func (w *compressedResponseWriter) Flush() {
	if len(w.buffer) > 0 {
		w.flushBuffer()
	}
	if flusher, ok := w.writer.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Hijack implements http.Hijacker.
func (w *compressedResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := w.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

// Push implements http.Pusher.
func (w *compressedResponseWriter) Push(target string, opts *http.PushOptions) error {
	if pusher, ok := w.ResponseWriter.(http.Pusher); ok {
		return pusher.Push(target, opts)
	}
	return http.ErrNotSupported
}

// CompressionMiddleware returns an HTTP middleware that compresses responses.
func CompressionMiddleware(config CompressionConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if path is excluded
			for _, path := range config.ExcludedPaths {
				if strings.HasPrefix(r.URL.Path, path) {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Check if extension is excluded
			for _, ext := range config.ExcludedExtensions {
				if strings.HasSuffix(r.URL.Path, ext) {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Check Accept-Encoding header
			acceptEncoding := r.Header.Get("Accept-Encoding")
			if acceptEncoding == "" {
				next.ServeHTTP(w, r)
				return
			}

			var encoding string
			var writer io.Writer

			// Prefer gzip over deflate
			if strings.Contains(acceptEncoding, "gzip") {
				encoding = "gzip"
				gz := gzipPool.Get().(*gzip.Writer)
				gz.Reset(w)
				writer = gz
				defer func() {
					gz.Close()
					gzipPool.Put(gz)
				}()
			} else if strings.Contains(acceptEncoding, "deflate") {
				encoding = "deflate"
				fl := flatePool.Get().(*flate.Writer)
				fl.Reset(w)
				writer = fl
				defer func() {
					fl.Close()
					flatePool.Put(fl)
				}()
			} else {
				next.ServeHTTP(w, r)
				return
			}

			cw := &compressedResponseWriter{
				ResponseWriter: w,
				writer:         writer,
				encoding:       encoding,
				config:         config,
				statusCode:     http.StatusOK,
				buffer:         make([]byte, 0, config.MinSize),
			}

			defer cw.Close()

			// Remove Accept-Encoding to prevent double compression
			r.Header.Del("Accept-Encoding")

			next.ServeHTTP(cw, r)
		})
	}
}

// GzipHandler wraps an http.Handler with gzip compression.
func GzipHandler(h http.Handler) http.Handler {
	return CompressionMiddleware(DefaultCompressionConfig())(h)
}

// GzipHandlerWithLevel wraps an http.Handler with gzip compression at the specified level.
func GzipHandlerWithLevel(h http.Handler, level int) http.Handler {
	config := DefaultCompressionConfig()
	config.Level = level
	return CompressionMiddleware(config)(h)
}
