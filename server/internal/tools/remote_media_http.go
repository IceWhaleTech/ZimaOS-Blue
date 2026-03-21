package tools

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

func newGuardedMediaHTTPClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	baseDial := transport.DialContext
	if baseDial == nil {
		dialer := &net.Dialer{Timeout: timeout}
		baseDial = dialer.DialContext
	}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host := address
		if parsedHost, _, err := net.SplitHostPort(address); err == nil && parsedHost != "" {
			host = parsedHost
		}
		if err := guardWebFetchHost(ctx, host, false); err != nil {
			return nil, err
		}
		return baseDial(ctx, network, address)
	}
	client := &http.Client{Timeout: timeout, Transport: transport}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if err := guardWebFetchHost(req.Context(), req.URL.Hostname(), false); err != nil {
			return err
		}
		if len(via) > webFetchDefaultMaxRedirects {
			return fmt.Errorf("too many redirects (max=%d)", webFetchDefaultMaxRedirects)
		}
		return nil
	}
	return client
}

func guardRemoteMediaURL(ctx context.Context, raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("parse remote media url: %w", err)
	}
	return guardWebFetchHost(ctx, parsed.Hostname(), false)
}
