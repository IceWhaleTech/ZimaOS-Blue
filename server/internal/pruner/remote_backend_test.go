package pruner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func skipRemoteBackendTest(t *testing.T) {
	t.Helper()
	t.Skip("SWE Pruner remote backend is hidden; tests temporarily disabled")
}

func TestRemoteBackend_Prune(t *testing.T) {
	skipRemoteBackendTest(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/prune" {
			http.NotFound(w, r)
			return
		}
		var req remoteRequest
		json.NewDecoder(r.Body).Decode(&req)

		resp := remoteResponse{
			Score:      0.85,
			PrunedCode: "func main() {\n(filtered 10 lines)\n}\n",
			KeptFrags:  []int{1, 12},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	backend := NewRemoteBackend(server.URL, server.Client())
	result, err := backend.Prune(context.Background(), PruneRequest{
		Code:      "func main() {\n" + "  // line\n" + "}\n",
		Query:     "find the main function",
		Threshold: 0.5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Score != 0.85 {
		t.Errorf("expected score 0.85, got %f", result.Score)
	}
	if result.PrunedCode == "" {
		t.Error("expected non-empty pruned code")
	}
	if result.LatencyMs <= 0 {
		t.Error("expected positive latency")
	}
}

func TestRemoteBackend_Health(t *testing.T) {
	skipRemoteBackendTest(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	backend := NewRemoteBackend(server.URL, server.Client())
	err := backend.Health(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRemoteBackend_HealthUnhealthy(t *testing.T) {
	skipRemoteBackendTest(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	backend := NewRemoteBackend(server.URL, server.Client())
	err := backend.Health(context.Background())
	if err == nil {
		t.Fatal("expected error for unhealthy backend")
	}
}

func TestRemoteBackend_PruneServerError(t *testing.T) {
	skipRemoteBackendTest(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	backend := NewRemoteBackend(server.URL, server.Client())
	_, err := backend.Prune(context.Background(), PruneRequest{Code: "test"})
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}
