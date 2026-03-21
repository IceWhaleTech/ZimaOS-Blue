package proxy

import (
	"crypto/tls"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/net/http2"
)

// ConnectionPool manages HTTP connections to upstream providers
type ConnectionPool struct {
	config            *ConnectionConfig
	transport         *http.Transport
	insecureTransport *http.Transport
	clients           sync.Map // map[string]*http.Client — read-heavy, write-once per key
	dnsCache          *DNSCache
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(config *ConnectionConfig) *ConnectionPool {
	dnsCache := NewDNSCache(5*time.Minute, 128)
	dialer := &net.Dialer{
		Timeout:   config.DialTimeout,
		KeepAlive: config.KeepAliveInterval,
	}

	transport := &http.Transport{
		DialContext:           dnsCache.DialContext(dialer),
		MaxIdleConns:          config.MaxIdleConns,
		MaxIdleConnsPerHost:   config.MaxIdleConnsPerHost,
		MaxConnsPerHost:       config.MaxConnsPerHost,
		IdleConnTimeout:       config.IdleConnTimeout,
		TLSHandshakeTimeout:   config.TLSHandshakeTimeout,
		ResponseHeaderTimeout: config.ResponseHeaderTimeout,
		ForceAttemptHTTP2:     config.ForceHTTP2,
		WriteBufferSize:       32 * 1024,
		ReadBufferSize:        64 * 1024,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	// Configure HTTP/2 with PING keepalive to detect dead connections early.
	// Without this, idle HTTP/2 connections may be silently closed by intermediate
	// proxies/LBs, causing the first request after idle to fail.
	http2Transport, err := http2.ConfigureTransports(transport)
	if err == nil {
		http2Transport.ReadIdleTimeout = 30 * time.Second // send PING after 30s idle
		http2Transport.PingTimeout = 15 * time.Second     // wait 15s for PONG
	}

	insecureTransport := transport.Clone()
	insecureTransport.TLSClientConfig = &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true, //nolint:gosec // user-opted skip for self-signed certs
	}
	// HTTP/2 PING for insecure transport too
	if h2, err := http2.ConfigureTransports(insecureTransport); err == nil {
		h2.ReadIdleTimeout = 30 * time.Second
		h2.PingTimeout = 15 * time.Second
	}

	return &ConnectionPool{
		config:            config,
		transport:         transport,
		insecureTransport: insecureTransport,
		dnsCache:          dnsCache,
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
	if v, ok := cp.clients.Load(key); ok {
		return v.(*http.Client)
	}

	t := cp.transport
	if insecure {
		t = cp.insecureTransport
	}

	client := &http.Client{
		Transport: t,
		Timeout:   0,
	}
	actual, _ := cp.clients.LoadOrStore(key, client)
	return actual.(*http.Client)
}

// GetTransport returns the underlying transport
func (cp *ConnectionPool) GetTransport() *http.Transport {
	return cp.transport
}

// Stats returns connection pool statistics
func (cp *ConnectionPool) Stats() map[string]interface{} {
	clientCount := 0
	cp.clients.Range(func(_, _ interface{}) bool {
		clientCount++
		return true
	})

	return map[string]interface{}{
		"max_idle_conns":          cp.config.MaxIdleConns,
		"max_idle_conns_per_host": cp.config.MaxIdleConnsPerHost,
		"max_conns_per_host":      cp.config.MaxConnsPerHost,
		"idle_conn_timeout":       cp.config.IdleConnTimeout.String(),
		"keep_alive":              cp.config.KeepAlive,
		"force_http2":             cp.config.ForceHTTP2,
		"client_count":            clientCount,
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
		ResponseHeaderTimeout: 10 * time.Minute,
		ForceHTTP2:            true,
	}
}
