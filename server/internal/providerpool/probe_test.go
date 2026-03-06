package providerpool

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// mockClaudeAWSProvider simulates a provider (like right.codes/claude-aws) that
// returns a model list via /v1/models but only some models are actually configured.
// Unconfigured models return HTTP 400 with {"error":"端点/claude-aws未配置模型xxx"}.
func mockClaudeAWSProvider(t *testing.T, configuredModels map[string]bool) *httptest.Server {
	return newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Model list endpoint
		if r.URL.Path == "/v1/models" {
			w.Header().Set("Content-Type", "application/json")
			// Return ALL models (both configured and unconfigured)
			allModels := []string{
				"claude-3-7-sonnet",
				"claude-3-7-sonnet-20250219",
				"claude-haiku-4-5",
				"claude-haiku-4-5-20251001",
				"claude-opus-4-5",
				"claude-opus-4-5-20251101",
				"claude-opus-4-6",
				"claude-opus-4-6-20260205",
				"claude-sonnet-4-20250514",
				"claude-sonnet-4-5",
				"claude-sonnet-4-5-20250929",
				"claude-sonnet-4-6",
			}
			var data []map[string]interface{}
			for _, id := range allModels {
				data = append(data, map[string]interface{}{
					"id":       id,
					"object":   "model",
					"created":  1626777600,
					"owned_by": "custom",
				})
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"data": data})
			return
		}

		// Chat completions endpoint — only configured models succeed
		if r.URL.Path == "/v1/chat/completions" {
			var req struct {
				Model string `json:"model"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
				return
			}

			if !configuredModels[req.Model] {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprintf(w, `{"error":"端点/claude-aws未配置模型%s"}`, req.Model)
				return
			}

			// Configured model — return minimal success response
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":      "chatcmpl-test",
				"object":  "chat.completion",
				"created": time.Now().Unix(),
				"model":   req.Model,
				"choices": []map[string]interface{}{
					{
						"index": 0,
						"message": map[string]string{
							"role":    "assistant",
							"content": "H",
						},
						"finish_reason": "length",
					},
				},
				"usage": map[string]int{
					"prompt_tokens":     5,
					"completion_tokens": 1,
					"total_tokens":      6,
				},
			})
			return
		}

		http.NotFound(w, r)
	}))
}

func TestProbeModels(t *testing.T) {
	// Models that are actually configured (matches real test results)
	configured := map[string]bool{
		"claude-haiku-4-5":           true,
		"claude-haiku-4-5-20251001":  true,
		"claude-opus-4-5":            true,
		"claude-opus-4-5-20251101":   true,
		"claude-opus-4-6":            true,
		"claude-sonnet-4-5":          true,
		"claude-sonnet-4-5-20250929": true,
		"claude-sonnet-4-6":          true,
	}

	server := mockClaudeAWSProvider(t, configured)
	defer server.Close()

	tmpDir, err := os.MkdirTemp("", "probe-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)

	provider := &Provider{
		ID:      "test-claude-aws",
		Name:    "Test Claude AWS",
		Type:    ProviderTypeCustom,
		Enabled: true,
		BaseURL: server.URL + "/v1",
		APIKeys: []APIKey{{ID: "key1", Key: "sk-test", Enabled: true}},
	}
	if err := registry.Register(provider); err != nil {
		t.Fatal(err)
	}

	discovery := NewModelDiscovery(registry, storage, time.Hour)

	// First fetch models (populates cache)
	models, err := discovery.FetchModels(context.Background(), "test-claude-aws")
	if err != nil {
		t.Fatalf("FetchModels: %v", err)
	}
	if len(models) != 12 {
		t.Fatalf("expected 12 models from list, got %d", len(models))
	}

	// All models should be enabled initially
	for _, m := range models {
		if !m.Enabled {
			t.Errorf("model %s should be enabled before probe", m.ID)
		}
	}

	// Probe models
	results, err := discovery.ProbeModels(context.Background(), "test-claude-aws", 4)
	if err != nil {
		t.Fatalf("ProbeModels: %v", err)
	}

	if len(results) != 12 {
		t.Fatalf("expected 12 probe results, got %d", len(results))
	}

	// Check results match expected
	availableCount := 0
	for _, r := range results {
		if r.Available {
			availableCount++
			if !configured[r.ModelID] {
				t.Errorf("model %s marked available but not configured", r.ModelID)
			}
			if r.StatusCode != 200 {
				t.Errorf("model %s available but status=%d", r.ModelID, r.StatusCode)
			}
		} else {
			if configured[r.ModelID] {
				t.Errorf("model %s marked unavailable but is configured", r.ModelID)
			}
			if r.StatusCode != 400 {
				t.Errorf("model %s unavailable but status=%d", r.ModelID, r.StatusCode)
			}
			if !strings.Contains(r.Error, "未配置模型") {
				t.Errorf("model %s error should contain '未配置模型', got: %s", r.ModelID, r.Error)
			}
		}
	}

	if availableCount != 8 {
		t.Errorf("expected 8 available models, got %d", availableCount)
	}

	// Verify models in cache are updated
	cachedModels, err := discovery.GetModels("test-claude-aws")
	if err != nil {
		t.Fatalf("GetModels after probe: %v", err)
	}

	enabledCount := 0
	for _, m := range cachedModels {
		if m.Enabled {
			enabledCount++
			if !configured[m.ID] {
				t.Errorf("cached model %s enabled but not configured", m.ID)
			}
		}
	}
	if enabledCount != 8 {
		t.Errorf("expected 8 enabled models in cache, got %d", enabledCount)
	}
}

func TestProbeModels_RouterIntegration(t *testing.T) {
	// Only 2 of 4 models configured
	configured := map[string]bool{
		"model-a": true,
		"model-c": true,
	}

	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]interface{}{
					{"id": "model-a", "object": "model"},
					{"id": "model-b", "object": "model"},
					{"id": "model-c", "object": "model"},
					{"id": "model-d", "object": "model"},
				},
			})
			return
		}
		if r.URL.Path == "/v1/chat/completions" {
			var req struct {
				Model string `json:"model"`
			}
			json.NewDecoder(r.Body).Decode(&req)
			if !configured[req.Model] {
				w.WriteHeader(400)
				fmt.Fprintf(w, `{"error":"model not found: %s"}`, req.Model)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"choices": []map[string]interface{}{{"message": map[string]string{"content": "ok"}}},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	tmpDir, _ := os.MkdirTemp("", "probe-router-*")
	defer os.RemoveAll(tmpDir)

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)

	provider := &Provider{
		ID:      "test-provider",
		Name:    "Test",
		Type:    ProviderTypeCustom,
		Enabled: true,
		Status:  ProviderStatusActive,
		BaseURL: server.URL + "/v1",
		APIKeys: []APIKey{{ID: "k1", Key: "test", Enabled: true}},
	}
	registry.Register(provider)

	discovery := NewModelDiscovery(registry, storage, time.Hour)
	discovery.FetchModels(context.Background(), "test-provider")

	router := NewRouter(registry, discovery, RoutingStrategyPriority)

	// Before probe: all 4 models should be routable
	for _, id := range []string{"model-a", "model-b", "model-c", "model-d"} {
		_, err := router.Route(&RouteRequest{ModelID: id})
		if err != nil {
			t.Errorf("before probe: model %s should be routable, got: %v", id, err)
		}
	}

	// Probe
	results, err := discovery.ProbeModels(context.Background(), "test-provider", 4)
	if err != nil {
		t.Fatal(err)
	}

	available := 0
	for _, r := range results {
		if r.Available {
			available++
		}
	}
	if available != 2 {
		t.Errorf("expected 2 available, got %d", available)
	}

	// Rebuild candidates after probe
	router.RebuildCandidates()

	// After probe: only configured models should be routable
	for _, id := range []string{"model-a", "model-c"} {
		_, err := router.Route(&RouteRequest{ModelID: id})
		if err != nil {
			t.Errorf("after probe: model %s should be routable, got: %v", id, err)
		}
	}
	for _, id := range []string{"model-b", "model-d"} {
		_, err := router.Route(&RouteRequest{ModelID: id})
		if err == nil {
			t.Errorf("after probe: model %s should NOT be routable", id)
		}
	}
}

func TestProbeModels_TransientErrors(t *testing.T) {
	// Test that transient errors (429, 500) don't disable models
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]interface{}{
					{"id": "model-ok", "object": "model"},
					{"id": "model-ratelimit", "object": "model"},
					{"id": "model-servererr", "object": "model"},
					{"id": "model-notconfigured", "object": "model"},
				},
			})
			return
		}
		if r.URL.Path == "/v1/chat/completions" {
			var req struct {
				Model string `json:"model"`
			}
			json.NewDecoder(r.Body).Decode(&req)
			switch req.Model {
			case "model-ok":
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"choices": []map[string]interface{}{{"message": map[string]string{"content": "ok"}}},
				})
			case "model-ratelimit":
				w.WriteHeader(429)
				fmt.Fprint(w, `{"error":"rate limit exceeded"}`)
			case "model-servererr":
				w.WriteHeader(500)
				fmt.Fprint(w, `{"error":"internal server error"}`)
			case "model-notconfigured":
				w.WriteHeader(400)
				fmt.Fprint(w, `{"error":"model not configured"}`)
			}
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	tmpDir, _ := os.MkdirTemp("", "probe-transient-*")
	defer os.RemoveAll(tmpDir)

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)
	registry.Register(&Provider{
		ID:      "test",
		Name:    "Test",
		Type:    ProviderTypeCustom,
		Enabled: true,
		BaseURL: server.URL + "/v1",
		APIKeys: []APIKey{{ID: "k1", Key: "test", Enabled: true}},
	})

	discovery := NewModelDiscovery(registry, storage, time.Hour)
	discovery.FetchModels(context.Background(), "test")

	results, err := discovery.ProbeModels(context.Background(), "test", 4)
	if err != nil {
		t.Fatal(err)
	}

	for _, r := range results {
		switch r.ModelID {
		case "model-ok":
			if !r.Available {
				t.Error("model-ok should be available")
			}
		case "model-ratelimit":
			if !r.Available {
				t.Error("model-ratelimit should still be available (transient 429)")
			}
		case "model-servererr":
			if !r.Available {
				t.Error("model-servererr should still be available (transient 500)")
			}
		case "model-notconfigured":
			if r.Available {
				t.Error("model-notconfigured should NOT be available")
			}
		}
	}
}

func TestIsModelNotConfiguredError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		errMsg     string
		expected   bool
	}{
		{"chinese not configured", 400, `{"error":"端点/claude-aws未配置模型claude-3-7-sonnet"}`, true},
		{"english not configured", 400, `{"error":"model not configured: gpt-4"}`, true},
		{"model not found 404", 404, `{"error":"model not found"}`, true},
		{"does not exist", 400, `{"error":"model does not exist"}`, true},
		{"unknown model", 400, `{"error":"unknown model: test"}`, true},
		{"rate limit 429", 429, `{"error":"rate limit exceeded"}`, false},
		{"server error 500", 500, `{"error":"internal server error"}`, false},
		{"auth error 401", 401, `{"error":"unauthorized"}`, false},
		{"generic 400", 400, `{"error":"bad request"}`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isModelNotConfiguredError(tt.statusCode, tt.errMsg)
			if result != tt.expected {
				t.Errorf("isModelNotConfiguredError(%d, %q) = %v, want %v",
					tt.statusCode, tt.errMsg, result, tt.expected)
			}
		})
	}
}
