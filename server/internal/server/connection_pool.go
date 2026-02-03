package server

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// ConnectionPool manages HTTP connections to providers for connection reuse
type ConnectionPool struct {
	// HTTP client with connection pooling
	client *http.Client

	// Per-provider connection limits
	providerLimits map[string]int
	limitMu        sync.RWMutex

	// Metrics
	activeConnections int64
	reuseCount        int64
	newConnections    int64
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(maxIdleConns, maxConnsPerHost int) *ConnectionPool {
	transport := &http.Transport{
		MaxIdleConns:        maxIdleConns,
		MaxIdleConnsPerHost: maxConnsPerHost,
		MaxConnsPerHost:     maxConnsPerHost * 2,
		IdleConnTimeout:     90 * time.Second,
		DisableKeepAlives:   false,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &ConnectionPool{
		client: &http.Client{
			Transport: transport,
			Timeout:   60 * time.Second,
		},
		providerLimits: make(map[string]int),
	}
}

// GetClient returns the HTTP client for making requests
func (cp *ConnectionPool) GetClient() *http.Client {
	return cp.client
}

// SetProviderLimit sets connection limit for a specific provider
func (cp *ConnectionPool) SetProviderLimit(provider string, limit int) {
	cp.limitMu.Lock()
	defer cp.limitMu.Unlock()
	cp.providerLimits[provider] = limit
}

// GetProviderLimit gets connection limit for a provider
func (cp *ConnectionPool) GetProviderLimit(provider string) int {
	cp.limitMu.RLock()
	defer cp.limitMu.RUnlock()

	if limit, exists := cp.providerLimits[provider]; exists {
		return limit
	}
	return 10 // Default limit
}

// Close closes all idle connections
func (cp *ConnectionPool) Close() {
	if transport, ok := cp.client.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}
}

// GetMetrics returns connection pool metrics
func (cp *ConnectionPool) GetMetrics() map[string]interface{} {
	cp.limitMu.RLock()
	defer cp.limitMu.RUnlock()

	return map[string]interface{}{
		"provider_limits": cp.providerLimits,
	}
}
