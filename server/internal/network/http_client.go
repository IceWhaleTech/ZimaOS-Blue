package network

import (
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	defaultClientDialTimeout           = 30 * time.Second
	defaultClientKeepAlive             = 30 * time.Second
	defaultClientMaxIdleConns          = 100
	defaultClientMaxIdleConnsPerHost   = 20
	defaultClientMaxConnsPerHost       = 100
	defaultClientIdleConnTimeout       = 90 * time.Second
	defaultClientTLSHandshakeTimeout   = 10 * time.Second
	defaultClientExpectContinueTimeout = 1 * time.Second
)

var (
	sharedPooledTransportMu            sync.Mutex
	sharedPooledTransport              *http.Transport
	sharedPooledTransportNoCompression *http.Transport
)

// HTTPClientOptions configures a pooled outbound HTTP client.
type HTTPClientOptions struct {
	Timeout            time.Duration
	DisableCompression bool
}

// NewPooledHTTPClient creates an outbound client with a reusable transport.
func NewPooledHTTPClient(timeout time.Duration) *http.Client {
	return NewPooledHTTPClientWithOptions(HTTPClientOptions{Timeout: timeout})
}

// NewPooledHTTPClientWithOptions creates an outbound client with tuned pooling defaults.
func NewPooledHTTPClientWithOptions(opts HTTPClientOptions) *http.Client {
	return &http.Client{
		Timeout:   opts.Timeout,
		Transport: sharedDefaultPooledTransport(opts.DisableCompression),
	}
}

// NewPooledTransport creates a reusable outbound transport tuned for remote API calls.
func NewPooledTransport(disableCompression bool) *http.Transport {
	return buildPooledTransport(disableCompression)
}

func sharedDefaultPooledTransport(disableCompression bool) *http.Transport {
	sharedPooledTransportMu.Lock()
	defer sharedPooledTransportMu.Unlock()

	if disableCompression {
		if sharedPooledTransportNoCompression == nil {
			sharedPooledTransportNoCompression = buildPooledTransport(true)
		}
		return sharedPooledTransportNoCompression
	}

	if sharedPooledTransport == nil {
		sharedPooledTransport = buildPooledTransport(false)
	}
	return sharedPooledTransport
}

func buildPooledTransport(disableCompression bool) *http.Transport {
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: defaultClientDialTimeout, KeepAlive: defaultClientKeepAlive}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          defaultClientMaxIdleConns,
		MaxIdleConnsPerHost:   defaultClientMaxIdleConnsPerHost,
		MaxConnsPerHost:       defaultClientMaxConnsPerHost,
		IdleConnTimeout:       defaultClientIdleConnTimeout,
		TLSHandshakeTimeout:   defaultClientTLSHandshakeTimeout,
		ExpectContinueTimeout: defaultClientExpectContinueTimeout,
		DisableCompression:    disableCompression,
	}
}
