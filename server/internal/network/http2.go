package network

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/http2"
)

// HTTP2Config holds HTTP/2 server configuration.
type HTTP2Config struct {
	// Enable HTTP/2 support
	Enabled bool

	// MaxConcurrentStreams limits the number of concurrent streams per connection
	MaxConcurrentStreams uint32

	// MaxReadFrameSize is the maximum size of a frame payload
	MaxReadFrameSize uint32

	// PermitProhibitedCipherSuites allows using cipher suites prohibited by HTTP/2
	PermitProhibitedCipherSuites bool

	// IdleTimeout is the maximum time a connection can be idle
	IdleTimeout time.Duration

	// MaxUploadBufferPerConnection is the size of the upload buffer per connection
	MaxUploadBufferPerConnection int32

	// MaxUploadBufferPerStream is the size of the upload buffer per stream
	MaxUploadBufferPerStream int32
}

// DefaultHTTP2Config returns default HTTP/2 configuration.
func DefaultHTTP2Config() HTTP2Config {
	return HTTP2Config{
		Enabled:                      true,
		MaxConcurrentStreams:         250,
		MaxReadFrameSize:             1 << 20, // 1MB
		PermitProhibitedCipherSuites: false,
		IdleTimeout:                  120 * time.Second,
		MaxUploadBufferPerConnection: 1 << 20, // 1MB
		MaxUploadBufferPerStream:     1 << 20, // 1MB
	}
}

// HTTP2Server wraps an HTTP server with HTTP/2 support.
type HTTP2Server struct {
	config     HTTP2Config
	server     *http.Server
	http2Srv   *http2.Server
}

// NewHTTP2Server creates a new HTTP/2 enabled server.
func NewHTTP2Server(handler http.Handler, config HTTP2Config) *HTTP2Server {
	server := &http.Server{
		Handler: handler,
	}

	h2s := &http2.Server{
		MaxConcurrentStreams:         config.MaxConcurrentStreams,
		MaxReadFrameSize:             config.MaxReadFrameSize,
		PermitProhibitedCipherSuites: config.PermitProhibitedCipherSuites,
		IdleTimeout:                  config.IdleTimeout,
		MaxUploadBufferPerConnection: config.MaxUploadBufferPerConnection,
		MaxUploadBufferPerStream:     config.MaxUploadBufferPerStream,
	}

	return &HTTP2Server{
		config:   config,
		server:   server,
		http2Srv: h2s,
	}
}

// ConfigureHTTP2 configures HTTP/2 on an existing server.
func (s *HTTP2Server) ConfigureHTTP2() error {
	if !s.config.Enabled {
		return nil
	}

	return http2.ConfigureServer(s.server, s.http2Srv)
}

// Server returns the underlying HTTP server.
func (s *HTTP2Server) Server() *http.Server {
	return s.server
}

// ListenAndServe starts the server on the given address.
func (s *HTTP2Server) ListenAndServe(addr string) error {
	s.server.Addr = addr
	if err := s.ConfigureHTTP2(); err != nil {
		return err
	}
	return s.server.ListenAndServe()
}

// ListenAndServeTLS starts the server with TLS on the given address.
func (s *HTTP2Server) ListenAndServeTLS(addr, certFile, keyFile string) error {
	s.server.Addr = addr
	if err := s.ConfigureHTTP2(); err != nil {
		return err
	}
	return s.server.ListenAndServeTLS(certFile, keyFile)
}

// Serve accepts connections on the listener.
func (s *HTTP2Server) Serve(l net.Listener) error {
	if err := s.ConfigureHTTP2(); err != nil {
		return err
	}
	return s.server.Serve(l)
}

// ServeTLS accepts TLS connections on the listener.
func (s *HTTP2Server) ServeTLS(l net.Listener, certFile, keyFile string) error {
	if err := s.ConfigureHTTP2(); err != nil {
		return err
	}
	return s.server.ServeTLS(l, certFile, keyFile)
}

// Shutdown gracefully shuts down the server.
func (s *HTTP2Server) Shutdown(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.server.Shutdown(ctx)
}

// TLSConfig returns recommended TLS configuration for HTTP/2.
func RecommendedTLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
		PreferServerCipherSuites: true,
		NextProtos:               []string{"h2", "http/1.1"},
	}
}

// ServerConfig holds general server configuration.
type ServerConfig struct {
	// Address to listen on
	Address string

	// ReadTimeout is the maximum duration for reading the entire request
	ReadTimeout time.Duration

	// WriteTimeout is the maximum duration before timing out writes of the response
	WriteTimeout time.Duration

	// IdleTimeout is the maximum amount of time to wait for the next request
	IdleTimeout time.Duration

	// ReadHeaderTimeout is the amount of time allowed to read request headers
	ReadHeaderTimeout time.Duration

	// MaxHeaderBytes controls the maximum number of bytes the server will read
	MaxHeaderBytes int

	// HTTP2 configuration
	HTTP2 HTTP2Config

	// TLS configuration
	TLS *tls.Config

	// KeepAlive settings
	KeepAlive KeepAliveConfig
}

// KeepAliveConfig holds keep-alive settings.
type KeepAliveConfig struct {
	// Enable keep-alive
	Enabled bool

	// Period between keep-alive probes
	Period time.Duration

	// Count of keep-alive probes before closing
	Count int
}

// DefaultServerConfig returns default server configuration.
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Address:           ":8080",
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1MB
		HTTP2:             DefaultHTTP2Config(),
		KeepAlive: KeepAliveConfig{
			Enabled: true,
			Period:  30 * time.Second,
			Count:   3,
		},
	}
}

// NewServer creates a new HTTP server with the given configuration.
func NewServer(handler http.Handler, config ServerConfig) *http.Server {
	server := &http.Server{
		Addr:              config.Address,
		Handler:           handler,
		ReadTimeout:       config.ReadTimeout,
		WriteTimeout:      config.WriteTimeout,
		IdleTimeout:       config.IdleTimeout,
		ReadHeaderTimeout: config.ReadHeaderTimeout,
		MaxHeaderBytes:    config.MaxHeaderBytes,
		TLSConfig:         config.TLS,
	}

	// Configure HTTP/2 if enabled
	if config.HTTP2.Enabled {
		h2s := &http2.Server{
			MaxConcurrentStreams:         config.HTTP2.MaxConcurrentStreams,
			MaxReadFrameSize:             config.HTTP2.MaxReadFrameSize,
			PermitProhibitedCipherSuites: config.HTTP2.PermitProhibitedCipherSuites,
			IdleTimeout:                  config.HTTP2.IdleTimeout,
			MaxUploadBufferPerConnection: config.HTTP2.MaxUploadBufferPerConnection,
			MaxUploadBufferPerStream:     config.HTTP2.MaxUploadBufferPerStream,
		}
		http2.ConfigureServer(server, h2s)
	}

	return server
}
