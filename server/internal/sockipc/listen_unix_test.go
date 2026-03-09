//go:build !windows

package sockipc

import (
	"net"
	"path/filepath"
	"testing"
	"time"
)

func TestListen_RejectsActiveSocket(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "blue.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen active socket: %v", err)
	}
	defer ln.Close()

	if _, err := listen(sock); err == nil {
		t.Fatal("expected error for active socket")
	}
}

func TestListen_ReplacesStaleSocket(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "blue.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen stale socket seed: %v", err)
	}
	ln.Close()
	time.Sleep(50 * time.Millisecond)

	replaced, err := listen(sock)
	if err != nil {
		t.Fatalf("listen stale socket replacement: %v", err)
	}
	defer replaced.Close()
}
