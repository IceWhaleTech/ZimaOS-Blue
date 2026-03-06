package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestCapabilityDetector_GetCapability(t *testing.T) {
	detector := NewCapabilityDetector()

	tests := []struct {
		provider    string
		wantFound   bool
		wantTool    ToolCallingSupport
		wantAdapter bool
	}{
		{"anthropic", true, ToolCallingNative, false},
		{"openai", true, ToolCallingNative, false},
		{"ollama", true, ToolCallingPartial, true},
		{"deepseek", true, ToolCallingNative, false},
		{"local", true, ToolCallingNone, true},
		{"unknown", false, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			cap, found := detector.GetCapability(tt.provider)
			if found != tt.wantFound {
				t.Errorf("GetCapability(%q) found = %v, want %v", tt.provider, found, tt.wantFound)
			}
			if found {
				if cap.ToolCalling != tt.wantTool {
					t.Errorf("ToolCalling = %v, want %v", cap.ToolCalling, tt.wantTool)
				}
				if cap.AdapterRequired != tt.wantAdapter {
					t.Errorf("AdapterRequired = %v, want %v", cap.AdapterRequired, tt.wantAdapter)
				}
			}
		})
	}
}

func TestCapabilityDetector_GetCapabilityMatrix(t *testing.T) {
	detector := NewCapabilityDetector()
	matrix := detector.GetCapabilityMatrix()

	if len(matrix) == 0 {
		t.Error("GetCapabilityMatrix() returned empty matrix")
	}

	// Check that all known providers are in the matrix
	expectedProviders := []string{"anthropic", "openai", "ollama", "deepseek", "gemini", "groq", "local"}
	for _, p := range expectedProviders {
		if _, ok := matrix[p]; !ok {
			t.Errorf("GetCapabilityMatrix() missing provider %q", p)
		}
	}
}

func TestCapabilityDetector_NeedsAdapter(t *testing.T) {
	detector := NewCapabilityDetector()

	tests := []struct {
		provider string
		want     bool
	}{
		{"anthropic", false},
		{"openai", false},
		{"ollama", true},
		{"local", true},
		{"unknown", true},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			got := detector.NeedsAdapter(tt.provider)
			if got != tt.want {
				t.Errorf("NeedsAdapter(%q) = %v, want %v", tt.provider, got, tt.want)
			}
		})
	}
}

func TestCapabilityDetector_GetAdapterType(t *testing.T) {
	detector := NewCapabilityDetector()

	tests := []struct {
		provider string
		want     string
	}{
		{"anthropic", ""},
		{"openai", ""},
		{"ollama", "ccnexus"},
		{"local", "cliproxy"},
		{"unknown", "cliproxy"},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			got := detector.GetAdapterType(tt.provider)
			if got != tt.want {
				t.Errorf("GetAdapterType(%q) = %q, want %q", tt.provider, got, tt.want)
			}
		})
	}
}

func TestCapabilityDetector_DetectCapabilities(t *testing.T) {
	detector := NewCapabilityDetector()
	ctx := context.Background()

	// Test known provider
	cap, err := detector.DetectCapabilities(ctx, "anthropic", "")
	if err != nil {
		t.Fatalf("DetectCapabilities() error = %v", err)
	}
	if cap.ToolCalling != ToolCallingNative {
		t.Errorf("ToolCalling = %v, want %v", cap.ToolCalling, ToolCallingNative)
	}

	// Test unknown provider without URL
	cap, err = detector.DetectCapabilities(ctx, "unknown", "")
	if err != nil {
		t.Fatalf("DetectCapabilities() error = %v", err)
	}
	if cap.ToolCalling != ToolCallingNone {
		t.Errorf("ToolCalling = %v, want %v", cap.ToolCalling, ToolCallingNone)
	}
}

func TestCapabilityDetector_DetectFromAPI(t *testing.T) {
	// Create mock OpenAI-compatible API server
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]string{
					{"id": "model-1"},
					{"id": "model-2"},
				},
			})
		}
	}))
	defer server.Close()

	detector := NewCapabilityDetector()
	ctx := context.Background()

	cap, err := detector.DetectCapabilities(ctx, "custom", server.URL)
	if err != nil {
		t.Fatalf("DetectCapabilities() error = %v", err)
	}

	// Should detect partial tool calling support for OpenAI-compatible API
	if cap.ToolCalling != ToolCallingPartial {
		t.Errorf("ToolCalling = %v, want %v", cap.ToolCalling, ToolCallingPartial)
	}

	if len(cap.SupportedModels) != 2 {
		t.Errorf("len(SupportedModels) = %d, want 2", len(cap.SupportedModels))
	}
}

func TestCapabilityDetector_CheckCompatibility(t *testing.T) {
	detector := NewCapabilityDetector()

	tests := []struct {
		name        string
		provider    string
		toolCalling bool
		vision      bool
		wantErr     bool
	}{
		{"anthropic with tool calling", "anthropic", true, false, false},
		{"anthropic with vision", "anthropic", false, true, false},
		{"local with tool calling", "local", true, false, true},
		{"deepseek with vision", "deepseek", false, true, true},
		{"unknown provider", "unknown", false, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := detector.CheckCompatibility(tt.provider, tt.toolCalling, tt.vision)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckCompatibility() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestProviderCapability_ToInfo(t *testing.T) {
	cap := &ProviderCapability{
		Provider:        "test",
		ToolCalling:     ToolCallingNative,
		Streaming:       true,
		Vision:          true,
		AdapterRequired: false,
		SupportedModels: []string{"model-1", "model-2"},
	}

	info := cap.ToInfo()

	if info.Provider != "test" {
		t.Errorf("Provider = %q, want %q", info.Provider, "test")
	}
	if info.ToolCalling != "native" {
		t.Errorf("ToolCalling = %q, want %q", info.ToolCalling, "native")
	}
	if !info.Streaming {
		t.Error("Streaming should be true")
	}
	if !info.Vision {
		t.Error("Vision should be true")
	}
	if len(info.Models) != 2 {
		t.Errorf("len(Models) = %d, want 2", len(info.Models))
	}
}
