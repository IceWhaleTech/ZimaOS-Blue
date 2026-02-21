package proxy

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net"
	"net/url"
	"sync"
	"time"
)

// ConnWarmup pre-establishes TCP+TLS connections to provider endpoints
// so the first real request doesn't pay the handshake cost.
type ConnWarmup struct {
	pool    *ConnectionPool
	dnsCache *DNSCache
}

// NewConnWarmup creates a new connection warmup manager.
func NewConnWarmup(pool *ConnectionPool) *ConnWarmup {
	return &ConnWarmup{
		pool:     pool,
		dnsCache: NewDNSCache(5*time.Minute, 128),
	}
}

// WarmProviders pre-connects to all provider base URLs concurrently.
// Called once at startup after providers are loaded.
func (cw *ConnWarmup) WarmProviders(baseURLs []string) {
	if len(baseURLs) == 0 {
		return
	}

	var wg sync.WaitGroup
	for _, rawURL := range baseURLs {
		u, err := url.Parse(rawURL)
		if err != nil || u.Host == "" {
			continue
		}
		host := u.Host
		if u.Port() == "" {
			if u.Scheme == "https" {
				host += ":443"
			} else {
				host += ":80"
			}
		}

		wg.Add(1)
		go func(host, scheme string) {
			defer wg.Done()
			cw.warmOne(host, scheme)
		}(host, u.Scheme)
	}
	wg.Wait()
	slog.Info("[conn-warmup] finished", "providers", len(baseURLs))
}

func (cw *ConnWarmup) warmOne(hostPort, scheme string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// DNS pre-resolve
	host, _, _ := net.SplitHostPort(hostPort)
	if host != "" {
		cw.dnsCache.Resolve(host)
	}

	// TCP + TLS handshake
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", hostPort)
	if err != nil {
		slog.Debug("[conn-warmup] dial failed", "host", hostPort, "error", err)
		return
	}

	if scheme == "https" {
		tlsConn := tls.Client(conn, &tls.Config{
			ServerName: host,
			MinVersion: tls.VersionTLS12,
		})
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			conn.Close()
			slog.Debug("[conn-warmup] TLS handshake failed", "host", hostPort, "error", err)
			return
		}
		tlsConn.Close()
	} else {
		conn.Close()
	}
	slog.Debug("[conn-warmup] warmed", "host", hostPort)
}

// DNSCache returns the DNS cache for use by the transport dialer.
func (cw *ConnWarmup) DNSCache() *DNSCache {
	return cw.dnsCache
}

// DNSCache caches DNS lookups to avoid repeated resolution on the hot path.
type DNSCache struct {
	mu      sync.RWMutex
	entries map[string]*dnsEntry
	ttl     time.Duration
	maxSize int
}

type dnsEntry struct {
	addrs     []string
	expiresAt time.Time
}

// NewDNSCache creates a DNS cache with the given TTL and max entries.
func NewDNSCache(ttl time.Duration, maxSize int) *DNSCache {
	return &DNSCache{
		entries: make(map[string]*dnsEntry, maxSize),
		ttl:     ttl,
		maxSize: maxSize,
	}
}

// Resolve looks up a hostname, returning cached results when available.
func (dc *DNSCache) Resolve(host string) ([]string, error) {
	dc.mu.RLock()
	if e, ok := dc.entries[host]; ok && time.Now().Before(e.expiresAt) {
		addrs := e.addrs
		dc.mu.RUnlock()
		return addrs, nil
	}
	dc.mu.RUnlock()

	addrs, err := net.LookupHost(host)
	if err != nil {
		return nil, err
	}

	dc.mu.Lock()
	if len(dc.entries) >= dc.maxSize {
		// Evict one random entry
		for k := range dc.entries {
			delete(dc.entries, k)
			break
		}
	}
	dc.entries[host] = &dnsEntry{
		addrs:     addrs,
		expiresAt: time.Now().Add(dc.ttl),
	}
	dc.mu.Unlock()
	return addrs, nil
}

// DialContext is a drop-in replacement for net.Dialer.DialContext that uses cached DNS.
func (dc *DNSCache) DialContext(dialer *net.Dialer) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return dialer.DialContext(ctx, network, addr)
		}

		addrs, lookupErr := dc.Resolve(host)
		if lookupErr != nil || len(addrs) == 0 {
			// Fallback to standard resolution
			return dialer.DialContext(ctx, network, addr)
		}

		// Try resolved addresses
		var lastErr error
		for _, ip := range addrs {
			conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip, port))
			if dialErr == nil {
				return conn, nil
			}
			lastErr = dialErr
		}
		return nil, lastErr
	}
}
