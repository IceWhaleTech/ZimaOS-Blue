package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOllamaDetector_Detect(t *testing.T) {
	// Create a mock Ollama server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			json.NewEncoder(w).Encode(map[string]string{
				"version": "0.1.0",
			})
		case "/api/tags":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"models": []map[string]interface{}{
					{
						"name":   "llama3.2",
						"size":   1234567890,
						"digest": "abc123",
					},
					{
						"name":   "codellama",
						"size":   987654321,
						"digest": "def456",
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	detector := &OllamaDetector{
		Endpoints: []string{server.URL},
		Timeout:   5,
		client:    server.Client(),
	}

	info, err := detector.Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}

	if !info.Available {
		t.Error("Detect() should return Available = true")
	}

	if info.Version != "0.1.0" {
		t.Errorf("Detect() version = %q, want %q", info.Version, "0.1.0")
	}

	if len(info.Models) != 2 {
		t.Errorf("Detect() returned %d models, want 2", len(info.Models))
	}
}

func TestOllamaDetector_DetectNotAvailable(t *testing.T) {
	detector := &OllamaDetector{
		Endpoints: []string{"http://localhost:99999"}, // Invalid port
		Timeout:   1,
		client:    http.DefaultClient,
	}

	info, _ := detector.Detect(context.Background())

	if info.Available {
		t.Error("Detect() should return Available = false for unreachable server")
	}
}

func TestOllamaInfo_GetModelNames(t *testing.T) {
	info := &OllamaInfo{
		Available: true,
		Models: []OllamaModel{
			{Name: "llama3.2"},
			{Name: "codellama"},
			{Name: "mistral"},
		},
	}

	names := info.GetModelNames()
	if len(names) != 3 {
		t.Errorf("GetModelNames() returned %d names, want 3", len(names))
	}

	expected := []string{"llama3.2", "codellama", "mistral"}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("GetModelNames()[%d] = %q, want %q", i, name, expected[i])
		}
	}
}
