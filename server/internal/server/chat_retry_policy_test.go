package server

import (
	"errors"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxybridge"
)

func TestShouldSkipPreContentRetry(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "non proxy error keeps retry",
			err:  errors.New("temporary network hiccup"),
			want: false,
		},
		{
			name: "client error skips retry",
			err: &proxybridge.ProxyError{
				StatusCode: 400,
				Body:       "bad request",
			},
			want: true,
		},
		{
			name: "overloaded skips retry",
			err: &proxybridge.ProxyError{
				StatusCode: 429,
				Body:       "rate limited",
			},
			want: true,
		},
		{
			name: "no provider skips retry",
			err: &proxybridge.ProxyError{
				StatusCode: 503,
				Body:       "no available provider",
			},
			want: true,
		},
		{
			name: "tool unsupported skips retry",
			err: &proxybridge.ProxyError{
				StatusCode: 400,
				Body:       "provider does not support tool calls",
			},
			want: true,
		},
		{
			name: "upstream error payload keeps retry",
			err: &proxybridge.ProxyError{
				StatusCode: 502,
				Body:       `upstream 502: {"error":{"message":"Upstream request failed","type":"upstream_error"}}`,
			},
			want: false,
		},
		{
			name: "generic 502 still retries",
			err: &proxybridge.ProxyError{
				StatusCode: 502,
				Body:       "bad gateway",
			},
			want: false,
		},
		{
			name: "provider no response skips retry",
			err: &proxybridge.ProxyError{
				StatusCode: 502,
				Body:       "provider prov_x returned no response",
			},
			want: true,
		},
		{
			name: "empty streaming response skips retry",
			err: &proxybridge.ProxyError{
				StatusCode: 502,
				Body:       "provider prov_x returned empty streaming response",
			},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldSkipPreContentRetry(tc.err)
			if got != tc.want {
				t.Fatalf("shouldSkipPreContentRetry() = %v, want %v", got, tc.want)
			}
		})
	}
}
