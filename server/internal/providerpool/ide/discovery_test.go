package ide

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDiscovery(t *testing.T) {
	discovery := NewDiscovery(5 * time.Second)

	t.Run("GetIDEName", func(t *testing.T) {
		tests := []struct {
			ideType  IDEType
			expected string
		}{
			{IDETypeAntigravity, "Antigravity (Google)"},
			{IDETypeCursor, "Cursor"},
			{IDETypeWindsurf, "Windsurf"},
			{IDEType("unknown"), "unknown"},
		}

		for _, tt := range tests {
			name := getIDEName(tt.ideType)
			if name != tt.expected {
				t.Errorf("getIDEName(%s) = %s, want %s", tt.ideType, name, tt.expected)
			}
		}
	})

	t.Run("ExpandPath", func(t *testing.T) {
		home, _ := os.UserHomeDir()

		tests := []struct {
			input    string
			expected string
		}{
			{"~/test", filepath.Join(home, "test")},
			{"/absolute/path", "/absolute/path"},
		}

		for _, tt := range tests {
			result := expandPath(tt.input)
			if result != tt.expected {
				t.Errorf("expandPath(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		}
	})

	t.Run("GetDiscovered", func(t *testing.T) {
		// Initially empty
		discovered := discovery.GetDiscovered()
		if len(discovered) != 0 {
			t.Errorf("Expected 0 discovered IDEs, got %d", len(discovered))
		}
	})

	t.Run("GetIDE", func(t *testing.T) {
		_, exists := discovery.GetIDE(IDETypeAntigravity)
		if exists {
			t.Error("Expected IDE not to exist before scan")
		}
	})
}

func TestFetchModels(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"data": [
				{"id": "gpt-4"},
				{"id": "claude-3-opus"},
				{"id": "gemini-pro"}
			]
		}`))
	}))
	defer server.Close()

	discovery := NewDiscovery(5 * time.Second)

	models, err := discovery.fetchModels(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("fetchModels failed: %v", err)
	}

	if len(models) != 3 {
		t.Errorf("Expected 3 models, got %d", len(models))
	}

	expected := []string{"gpt-4", "claude-3-opus", "gemini-pro"}
	for i, model := range models {
		if model != expected[i] {
			t.Errorf("Model %d: expected %s, got %s", i, expected[i], model)
		}
	}
}

func TestFetchModelsError(t *testing.T) {
	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	discovery := NewDiscovery(5 * time.Second)

	_, err := discovery.fetchModels(context.Background(), server.URL)
	if err == nil {
		t.Error("Expected error for 500 response")
	}
}

func TestTryPorts(t *testing.T) {
	// Create mock server on a specific port
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data": []}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	// tryPorts won't find our test server since it uses random port
	// This test just verifies the function doesn't panic
	result := tryPorts([]int{1, 2, 3})
	if result != "" {
		t.Log("Found unexpected port:", result)
	}
}

func TestIDEPaths(t *testing.T) {
	t.Run("AntigravityPaths", func(t *testing.T) {
		paths := getAntigravityPaths()
		if len(paths) == 0 {
			t.Error("Expected at least one path for Antigravity")
		}
	})

	t.Run("CursorPaths", func(t *testing.T) {
		paths := getCursorPaths()
		if len(paths) == 0 {
			t.Error("Expected at least one path for Cursor")
		}
	})

	t.Run("WindsurfPaths", func(t *testing.T) {
		paths := getWindsurfPaths()
		if len(paths) == 0 {
			t.Error("Expected at least one path for Windsurf")
		}
	})
}

func TestScanWithMockConfig(t *testing.T) {
	// Create temp directory with mock config
	tmpDir, err := os.MkdirTemp("", "ide-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create mock config file
	configPath := filepath.Join(tmpDir, "config.json")
	configData := []byte(`{"proxy_url": "http://localhost:9999/v1"}`)
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Test findProxyURL with mock config
	proxyURL := findProxyURL(IDETypeAntigravity, configPath)
	if proxyURL != "http://localhost:9999/v1" {
		t.Errorf("Expected proxy URL from config, got %s", proxyURL)
	}
}

func TestConnectWithMockServer(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/models" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"data": [{"id": "test-model"}]}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	// Create temp config
	tmpDir, _ := os.MkdirTemp("", "ide-connect-test-*")
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")
	configData := []byte(fmt.Sprintf(`{"proxy_url": "%s"}`, server.URL))
	os.WriteFile(configPath, configData, 0644)

	discovery := NewDiscovery(5 * time.Second)

	// Manually set discovered IDE with mock config
	discovery.mu.Lock()
	discovery.discovered[IDETypeAntigravity] = &IDEInfo{
		Type:       IDETypeAntigravity,
		Name:       "Antigravity (Google)",
		ConfigPath: configPath,
		ProxyURL:   server.URL,
	}
	discovery.mu.Unlock()

	// Test connect
	info, err := discovery.Connect(context.Background(), IDETypeAntigravity)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	if !info.Connected {
		t.Error("Expected Connected to be true")
	}

	if len(info.Models) != 1 {
		t.Errorf("Expected 1 model, got %d", len(info.Models))
	}

	if info.Models[0] != "test-model" {
		t.Errorf("Expected test-model, got %s", info.Models[0])
	}
}
