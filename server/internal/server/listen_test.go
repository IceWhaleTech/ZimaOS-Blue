package server

import (
	"fmt"
	"net"
	"testing"
)

func TestListenWithFallback_NormalBind(t *testing.T) {
	// Port 0 lets OS pick a free port — should always succeed.
	ln, port, err := ListenWithFallback(":0", 0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer ln.Close()

	if port == 0 {
		t.Fatal("expected non-zero port")
	}
}

func TestListenWithFallback_FallbackOnConflict(t *testing.T) {
	// Occupy a port first.
	blocker, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("failed to create blocker listener: %v", err)
	}
	defer blocker.Close()
	blockedPort := blocker.Addr().(*net.TCPAddr).Port

	// Try to bind the same port with fallback enabled.
	addr := fmt.Sprintf(":%d", blockedPort)
	ln, port, err := ListenWithFallback(addr, blockedPort, true)
	if err != nil {
		t.Fatalf("expected fallback to succeed, got: %v", err)
	}
	defer ln.Close()

	if port == blockedPort {
		t.Fatalf("expected different port after fallback, got same port %d", port)
	}
	if port == 0 {
		t.Fatal("expected non-zero fallback port")
	}
}

func TestListenWithFallback_NoFallback(t *testing.T) {
	// Occupy a port first.
	blocker, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("failed to create blocker listener: %v", err)
	}
	defer blocker.Close()
	blockedPort := blocker.Addr().(*net.TCPAddr).Port

	// Try to bind the same port with fallback disabled — should fail.
	addr := fmt.Sprintf(":%d", blockedPort)
	ln, _, err := ListenWithFallback(addr, blockedPort, false)
	if err == nil {
		ln.Close()
		t.Fatal("expected error when fallback is disabled and port is occupied")
	}
}

func TestIsAddrInUse(t *testing.T) {
	// Occupy a port, then try to listen again to get the real error.
	blocker, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("failed to create blocker: %v", err)
	}
	defer blocker.Close()

	addr := blocker.Addr().String()
	_, err = net.Listen("tcp", addr)
	if err == nil {
		t.Fatal("expected error for double-bind")
	}
	if !IsAddrInUse(err) {
		t.Fatalf("expected IsAddrInUse=true for %q", err)
	}
}
