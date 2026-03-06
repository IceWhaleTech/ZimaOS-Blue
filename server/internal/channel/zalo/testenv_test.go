package zalo

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTCP4Server(tb testing.TB, handler http.Handler) *httptest.Server {
	tb.Helper()
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		tb.Skipf("skip test server setup (tcp4 unavailable): %v", err)
	}
	srv := httptest.NewUnstartedServer(handler)
	srv.Listener = ln
	srv.Start()
	return srv
}
