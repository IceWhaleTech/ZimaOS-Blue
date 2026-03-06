package sockipc

import (
	"strings"
	"testing"
)

// startServerOrSkip allows sockipc tests to run in restricted sandboxes where
// Unix socket bind may be prohibited by policy.
func startServerOrSkip(t *testing.T, srv *Server) {
	t.Helper()
	if err := srv.Start(); err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "bind: operation not permitted") ||
			strings.Contains(msg, "bind: permission denied") {
			t.Skipf("skip sockipc test in restricted runtime: %v", err)
		}
		t.Fatal(err)
	}
}
