package proxy

import (
	"crypto/tls"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/net/http2"
)

type ConnectionProfile string

const (
	ConnectionProfileLong  ConnectionProfile = "long"
	ConnectionProfileProbe ConnectionProfile = "probe"
)

const (
	defaultProbeResponseHeaderTimeout = 15 * time.Second
	defaultProbeIdleConnTimeout       = 30 * time.Second
	defaultProbeClientTimeout         = 20 * time.Second
)

type transportKey struct {
	profile  ConnectionProfile
	insecure bool
}

// ConnectionPool manages HTTP connections to upstream providers
type ConnectionPool struct {
	config     *ConnectionConfig
	transports map[transportKey]*http.Transport
	clients    sync.Map // map[string]*http.Client — read-heavy, write-once per key
	dnsCache   *DNSCache
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
	transports := map[transportKey]*http.Transport{
		{profile: ConnectionProfileLong, insecure: false}:  buildTransport(dnsCache, effectiveConfigForProfile(effective, ConnectionProfileLong), false),
		{profile: ConnectionProfileLong, insecure: true}:   buildTransport(dnsCache, effectiveConfigForProfile(effective, ConnectionProfileLong), true),
		{profile: ConnectionProfileProbe, insecure: false}: buildTransport(dnsCache, effectiveConfigForProfile(effective, ConnectionProfileProbe), false),
		{profile: ConnectionProfileProbe, insecure: true}:  buildTransport(dnsCache, effectiveConfigForProfile(effective, ConnectionProfileProbe), true),
	}

	return &ConnectionPool{
		config:     &effective,
		transports: transports,
		dnsCache:   dnsCache,
	}
}

// GetClient returns an HTTP client for the given provider
func (cp *ConnectionPool) GetClient(provider string, profiles ...ConnectionProfile) *http.Client {
	return cp.getClient(provider, false, normalizeConnectionProfile(profiles...))
}

// GetInsecureClient returns an HTTP client that skips TLS verification
func (cp *ConnectionPool) GetInsecureClient(provider string, profiles ...ConnectionProfile) *http.Client {
	return cp.getClient(provider, true, normalizeConnectionProfile(profiles...))
}

func (cp *ConnectionPool) getClient(provider string, insecure bool, profile ConnectionProfile) *http.Client {
	key := provider + "|" + string(profile)
	if insecure {
		key += "|insecure"
	}
	if v, ok := cp.clients.Load(key); ok {
		return v.(*http.Client)
	}

	client := &http.Client{
		Transport: cp.transportForProfile(profile, insecure),
		Timeout:   clientTimeoutForProfile(cp.config, profile),
	}
	actual, _ := cp.clients.LoadOrStore(key, client)
	return actual.(*http.Client)
}

// GetTransport returns the underlying transport
func (cp *ConnectionPool) GetTransport() *http.Transport {
	return cp.GetTransportForProfile(ConnectionProfileLong)
}

// GetTransportForProfile returns the transport for the given profile.
func (cp *ConnectionPool) GetTransportForProfile(profile ConnectionProfile) *http.Transport {
	return cp.transportForProfile(profile, false)
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
	for _, transport := range cp.transports {
		transport.CloseIdleConnections()
	}
}

// CloseIdleConnectionsForProfile closes idle connections for both secure and insecure transports of a profile.
func (cp *ConnectionPool) CloseIdleConnectionsForProfile(profile ConnectionProfile) {
	profile = normalizeConnectionProfile(profile)
	for key, transport := range cp.transports {
		if key.profile == profile {
			transport.CloseIdleConnections()
		}
	}
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
		ForceHTTP2:            false,
	}
}

func normalizeConnectionProfile(profiles ...ConnectionProfile) ConnectionProfile {
	if len(profiles) == 0 {
		return ConnectionProfileLong
	}
	switch profiles[0] {
	case ConnectionProfileProbe:
		return ConnectionProfileProbe
	default:
		return ConnectionProfileLong
	}
}

func effectiveConfigForProfile(base ConnectionConfig, profile ConnectionProfile) ConnectionConfig {
	effective := base
	switch normalizeConnectionProfile(profile) {
	case ConnectionProfileProbe:
		if effective.ResponseHeaderTimeout <= 0 || effective.ResponseHeaderTimeout > defaultProbeResponseHeaderTimeout {
			effective.ResponseHeaderTimeout = defaultProbeResponseHeaderTimeout
		}
		if effective.IdleConnTimeout <= 0 || effective.IdleConnTimeout > defaultProbeIdleConnTimeout {
			effective.IdleConnTimeout = defaultProbeIdleConnTimeout
		}
	}
	if !effective.KeepAlive {
		effective.ForceHTTP2 = false
	}
	return effective
}

func clientTimeoutForProfile(base *ConnectionConfig, profile ConnectionProfile) time.Duration {
	if normalizeConnectionProfile(profile) != ConnectionProfileProbe {
		return 0
	}
	if base == nil {
		return defaultProbeClientTimeout
	}
	effective := effectiveConfigForProfile(*base, ConnectionProfileProbe)
	timeout := effective.ResponseHeaderTimeout + 5*time.Second
	if timeout <= 0 {
		return defaultProbeClientTimeout
	}
	if timeout < defaultProbeClientTimeout {
		return defaultProbeClientTimeout
	}
	return timeout
}

func (cp *ConnectionPool) transportForProfile(profile ConnectionProfile, insecure bool) *http.Transport {
	return cp.transports[transportKey{profile: normalizeConnectionProfile(profile), insecure: insecure}]
}

func buildTransport(dnsCache *DNSCache, effective ConnectionConfig, insecure bool) *http.Transport {
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
	if insecure {
		transport.TLSClientConfig = &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: true, //nolint:gosec // user-opted skip for self-signed certs
		}
	}

	// Configure HTTP/2 with PING keepalive to detect dead connections early.
	// Without this, idle HTTP/2 connections may be silently closed by intermediate
	// proxies/LBs, causing the first request after idle to fail.
	if effective.ForceHTTP2 {
		if h2, err := http2.ConfigureTransports(transport); err == nil {
			h2.ReadIdleTimeout = 30 * time.Second
			h2.PingTimeout = 15 * time.Second
		}
	}

	return transport
}
