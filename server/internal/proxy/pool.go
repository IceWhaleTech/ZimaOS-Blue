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
	if config == nil {
		config = DefaultConnectionConfig()
	}
	effective := *config
	if !effective.KeepAlive {
		// HTTP/2 is inherently connection-oriented and tends to keep a single
		// session warm. When callers explicitly disable keep-alives, honor that
		// intent by also disabling opportunistic HTTP/2 reuse.
		effective.ForceHTTP2 = false
	}

	dnsCache := NewDNSCache(5*time.Minute, 128)
	dialKeepAlive := effective.KeepAliveInterval
	if !effective.KeepAlive {
		dialKeepAlive = -1
	}
	dialer := &net.Dialer{
		Timeout:   effective.DialTimeout,
		KeepAlive: dialKeepAlive,
	}

	transport := &http.Transport{
		DialContext:           dnsCache.DialContext(dialer),
		MaxIdleConns:          effective.MaxIdleConns,
		MaxIdleConnsPerHost:   effective.MaxIdleConnsPerHost,
		MaxConnsPerHost:       effective.MaxConnsPerHost,
		IdleConnTimeout:       effective.IdleConnTimeout,
		DisableKeepAlives:     !effective.KeepAlive,
		TLSHandshakeTimeout:   effective.TLSHandshakeTimeout,
		ResponseHeaderTimeout: effective.ResponseHeaderTimeout,
		ForceAttemptHTTP2:     effective.ForceHTTP2,
		WriteBufferSize:       32 * 1024,
		ReadBufferSize:        64 * 1024,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	// Configure HTTP/2 with PING keepalive to detect dead connections early.
	// Without this, idle HTTP/2 connections may be silently closed by intermediate
	// proxies/LBs, causing the first request after idle to fail.
	if effective.ForceHTTP2 {
		http2Transport, err := http2.ConfigureTransports(transport)
		if err == nil {
			http2Transport.ReadIdleTimeout = 30 * time.Second // send PING after 30s idle
			http2Transport.PingTimeout = 15 * time.Second     // wait 15s for PONG
		}
	}

	insecureTransport := transport.Clone()
	insecureTransport.TLSClientConfig = &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true, //nolint:gosec // user-opted skip for self-signed certs
	}
	// HTTP/2 PING for insecure transport too
	if effective.ForceHTTP2 {
		if h2, err := http2.ConfigureTransports(insecureTransport); err == nil {
			h2.ReadIdleTimeout = 30 * time.Second
			h2.PingTimeout = 15 * time.Second
		}
	}

	return &ConnectionPool{
		config:            &effective,
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
