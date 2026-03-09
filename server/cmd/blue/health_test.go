package main

import (
	"encoding/json"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestRunHealthUsesResolvedServiceURL(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":    "ok",
			"timestamp": time.Now().Format(time.RFC3339),
		})
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(listener) }()
	defer func() { _ = server.Close() }()

	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}

	t.Setenv("BLUE_SERVER_HOST", host)
	t.Setenv("BLUE_SERVER_PORT", port)

	oldJSON, oldNoColor, oldVerbose, oldTimeout := jsonOutput, noColor, verbose, healthTimeout
	jsonOutput, noColor, verbose, healthTimeout = true, true, false, 2
	defer func() {
		jsonOutput, noColor, verbose, healthTimeout = oldJSON, oldNoColor, oldVerbose, oldTimeout
	}()

	out := captureStdout(t, func() { runHealth(nil, nil) })
	var resp HealthResponse
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("decode health response: %v\n%s", err, out)
	}
	if resp.Status != "ok" && resp.Status != "healthy" {
		t.Fatalf("unexpected health status: %+v", resp)
	}
}
