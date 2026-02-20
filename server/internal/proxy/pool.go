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
	config            *ConnectionConfig
	transport         *http.Transport
	insecureTransport *http.Transport
	clients           map[string]*http.Client
	mu                sync.RWMutex
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

	insecureTransport := transport.Clone()
	insecureTransport.TLSClientConfig = &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true, //nolint:gosec // user-opted skip for self-signed certs
	}

	return &ConnectionPool{
		config:            config,
		transport:         transport,
		insecureTransport: insecureTransport,
		clients:           make(map[string]*http.Client),
	}
}

// GetClient returns an HTTP client for the given provider
func (cp *ConnectionPool) GetClient(provider string) *http.Client {
	return cp.getClient(provider, false)
}

// GetInsecureClient returns an HTTP client that skips TLS verification
func (cp *ConnectionPool) GetInsecureClient(provider string) *http.Client {
	return cp.getClient(provider+":insecure", true)
}

func (cp *ConnectionPool) getClient(key string, insecure bool) *http.Client {
	cp.mu.RLock()
	if client, ok := cp.clients[key]; ok {
		cp.mu.RUnlock()
		return client
	}
	cp.mu.RUnlock()

	cp.mu.Lock()
	defer cp.mu.Unlock()

	if client, ok := cp.clients[key]; ok {
		return client
	}

	t := cp.transport
	if insecure {
		t = cp.insecureTransport
	}

	client := &http.Client{
		Transport: t,
		Timeout:   0,
	}
	cp.clients[key] = client
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
		MaxIdleConnsPerHost:   20,
		MaxConnsPerHost:       100,
		IdleConnTimeout:       90 * time.Second,
		KeepAlive:             true,
		KeepAliveInterval:     30 * time.Second,
		DialTimeout:           10 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
		ForceHTTP2:            true,
	}
}
