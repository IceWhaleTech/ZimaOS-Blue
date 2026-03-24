package main

import (
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"
)

func reserveTCPPort(t *testing.T) int {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	defer ln.Close()

	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("unexpected listener addr type: %T", ln.Addr())
	}
	return addr.Port
}

func waitForEmbeddedHealth(t *testing.T, port int, timeout time.Duration) {
	t.Helper()

	client := &http.Client{Timeout: 500 * time.Millisecond}
	deadline := time.Now().Add(timeout)
	url := fmt.Sprintf("http://127.0.0.1:%d/api/v1/health", port)

	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return
			}
		}
		time.Sleep(25 * time.Millisecond)
	}

	t.Fatalf("embedded server did not become healthy on port %d within %s", port, timeout)
}

func TestBlueServerStopCompletesWithinTwoSeconds(t *testing.T) {
	t.Setenv("BLUE_DISABLE_BROWSER_MONITOR", "1")

	dataDir := t.TempDir()
	port := reserveTCPPort(t)

	serverMu.Lock()
	if isRunning {
		serverMu.Unlock()
		t.Fatal("embedded server unexpectedly already running")
	}
	setupSignalHandler()
	startEmbeddedServerLocked(port, dataDir, "")
	serverMu.Unlock()

	t.Cleanup(func() {
		if isRunning {
			_ = BlueServerStop()
		}
	})

	waitForEmbeddedHealth(t, port, 20*time.Second)

	started := time.Now()
	if rc := BlueServerStop(); rc != 0 {
		t.Fatalf("BlueServerStop() = %d, want 0", rc)
	}
	elapsed := time.Since(started)

	if elapsed > 2*time.Second {
		t.Fatalf("BlueServerStop() took %s, want <= 2s", elapsed)
	}
}
