package main

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"
)

func setupSessionsCLIServer(t *testing.T, handler func(http.ResponseWriter, *http.Request)) string {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/conversations", handler)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(listener) }()

	t.Cleanup(func() {
		_ = server.Close()
	})

	return listener.Addr().String()
}

func TestRunSessionsListAcceptsArrayResponse(t *testing.T) {
	createdAt := time.Now().UTC().Truncate(time.Second)
	updatedAt := createdAt.Add(time.Minute)
	addr := setupSessionsCLIServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/conversations" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{{
			"id":            "conv-array-12345678",
			"title":         "array session",
			"created_at":    createdAt.Format(time.RFC3339),
			"updated_at":    updatedAt.Format(time.RFC3339),
			"message_count": 3,
		}})
	})

	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("atoi: %v", err)
	}

	t.Setenv("BLUE_SERVER_HOST", host)
	t.Setenv("BLUE_SERVER_PORT", strconv.Itoa(port))

	oldJSON, oldNoColor := jsonOutput, noColor
	jsonOutput, noColor = true, true
	defer func() {
		jsonOutput, noColor = oldJSON, oldNoColor
	}()

	out := captureStdout(t, func() { runSessionsList(nil, nil) })
	var result sessionListResponse
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode list response: %v\n%s", err, out)
	}
	if len(result.Conversations) != 1 {
		t.Fatalf("expected 1 conversation, got %d", len(result.Conversations))
	}
	if result.Conversations[0].ID != "conv-array-12345678" {
		t.Fatalf("unexpected conversation id: %s", result.Conversations[0].ID)
	}
	if result.Conversations[0].Title != "array session" {
		t.Fatalf("unexpected conversation title: %s", result.Conversations[0].Title)
	}
}

func TestRunSessionsListAcceptsWrappedResponse(t *testing.T) {
	addr := setupSessionsCLIServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"conversations": []map[string]any{{
				"id":            "conv-wrapped-1234",
				"title":         "wrapped session",
				"message_count": 1,
			}},
		})
	})

	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("atoi: %v", err)
	}

	t.Setenv("BLUE_SERVER_HOST", host)
	t.Setenv("BLUE_SERVER_PORT", strconv.Itoa(port))

	oldJSON, oldNoColor := jsonOutput, noColor
	jsonOutput, noColor = true, true
	defer func() {
		jsonOutput, noColor = oldJSON, oldNoColor
	}()

	out := captureStdout(t, func() { runSessionsList(nil, nil) })
	var result sessionListResponse
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode list response: %v\n%s", err, out)
	}
	if len(result.Conversations) != 1 {
		t.Fatalf("expected 1 conversation, got %d", len(result.Conversations))
	}
	if result.Conversations[0].ID != "conv-wrapped-1234" {
		t.Fatalf("unexpected conversation id: %s", result.Conversations[0].ID)
	}
}

func TestDispatchSessionsFastPathList(t *testing.T) {
	addr := setupSessionsCLIServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{{
			"id":            "conv-fastpath-1234",
			"title":         "fast path session",
			"message_count": 2,
		}})
	})

	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("atoi: %v", err)
	}

	t.Setenv("BLUE_SERVER_HOST", host)
	t.Setenv("BLUE_SERVER_PORT", strconv.Itoa(port))

	oldJSON, oldNoColor, oldActive := jsonOutput, noColor, sessionsActive
	jsonOutput, noColor, sessionsActive = true, true, false
	defer func() {
		jsonOutput, noColor, sessionsActive = oldJSON, oldNoColor, oldActive
	}()

	out := captureStdout(t, func() {
		if !dispatchSessionsFastPath([]string{"list"}) {
			t.Fatal("expected sessions fast path to handle list")
		}
	})

	var result sessionListResponse
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode fast path response: %v\n%s", err, out)
	}
	if len(result.Conversations) != 1 || result.Conversations[0].ID != "conv-fastpath-1234" {
		t.Fatalf("unexpected fast path response: %+v", result)
	}
}
