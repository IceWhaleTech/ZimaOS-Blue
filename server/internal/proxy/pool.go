package proxy

import (
	"crypto/tls"
	"net"
	"net/http"
	"sync"
	"time"
)

// ConnectionPool manages HTTP connections to upstream providers
type ConnectionPool struct {
	config    *ConnectionConfig
	transport *http.Transport
	clients   map[string]*http.Client
	mu        sync.RWMutex
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(config *ConnectionConfig) *ConnectionPool {
	dialer := &net.Dialer{
		Timeout:   config.DialTimeout,
		KeepAlive: config.KeepAliveInterval,
	}

	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		MaxIdleConns:          config.MaxIdleConns,
		MaxIdleConnsPerHost:   config.MaxIdleConnsPerHost,
		MaxConnsPerHost:       config.MaxConnsPerHost,
		IdleConnTimeout:       config.IdleConnTimeout,
		TLSHandshakeTimeout:   config.TLSHandshakeTimeout,
		ResponseHeaderTimeout: config.ResponseHeaderTimeout,
		ForceAttemptHTTP2:     config.ForceHTTP2,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	return &ConnectionPool{
		config:    config,
		transport: transport,
		clients:   make(map[string]*http.Client),
	}
}

// GetClient returns an HTTP client for the given provider
func (cp *ConnectionPool) GetClient(provider string) *http.Client {
	cp.mu.RLock()
	if client, ok := cp.clients[provider]; ok {
		cp.mu.RUnlock()
		return client
	}
	cp.mu.RUnlock()

	cp.mu.Lock()
	defer cp.mu.Unlock()

	// Double-check after acquiring write lock
	if client, ok := cp.clients[provider]; ok {
		return client
	}

	client := &http.Client{
		Transport: cp.transport,
		Timeout:   0, // No timeout, handled by context
	}
	cp.clients[provider] = client
	return client
}

// GetTransport returns the underlying transport
func (cp *ConnectionPool) GetTransport() *http.Transport {
	return cp.transport
}

// Stats returns connection pool statistics
func (cp *ConnectionPool) Stats() map[string]interface{} {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	return map[string]interface{}{
		"max_idle_conns":          cp.config.MaxIdleConns,
		"max_idle_conns_per_host": cp.config.MaxIdleConnsPerHost,
		"max_conns_per_host":      cp.config.MaxConnsPerHost,
		"idle_conn_timeout":       cp.config.IdleConnTimeout.String(),
		"keep_alive":              cp.config.KeepAlive,
		"force_http2":             cp.config.ForceHTTP2,
		"client_count":            len(cp.clients),
	}
}

// Close closes all idle connections
func (cp *ConnectionPool) Close() {
	cp.transport.CloseIdleConnections()
}

// DefaultConnectionConfig returns default connection configuration
func DefaultConnectionConfig() *ConnectionConfig {
	return &ConnectionConfig{
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		MaxConnsPerHost:       100,
		IdleConnTimeout:       90 * time.Second,
		KeepAlive:             true,
		KeepAliveInterval:     30 * time.Second,
		DialTimeout:           30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
		ForceHTTP2:            true,
	}
}
