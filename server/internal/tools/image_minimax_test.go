package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
)

func TestMiniMaxImageVisionEndpoint(t *testing.T) {
	cases := []struct {
		name    string
		baseURL string
		want    string
	}{
		{
			name:    "anthropic china endpoint",
			baseURL: "https://api.minimaxi.com/anthropic",
			want:    "https://api.minimaxi.com/v1/coding_plan/vlm",
		},
		{
			name:    "anthropic global endpoint",
			baseURL: "https://api.minimax.io/anthropic",
			want:    "https://api.minimax.io/v1/coding_plan/vlm",
		},
		{
			name:    "v1 endpoint",
			baseURL: "https://api.minimax.io/v1",
			want:    "https://api.minimax.io/v1/coding_plan/vlm",
		},
		{
			name:    "origin endpoint",
			baseURL: "https://api.minimax.io",
			want:    "https://api.minimax.io/v1/coding_plan/vlm",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := miniMaxImageVisionEndpoint(tc.baseURL)
			if err != nil {
				t.Fatalf("miniMaxImageVisionEndpoint(%q) error = %v", tc.baseURL, err)
			}
			if got != tc.want {
				t.Fatalf("miniMaxImageVisionEndpoint(%q) = %q, want %q", tc.baseURL, got, tc.want)
			}
		})
	}
}

func TestMiniMaxImageVisionAnalyzeUsesDirectEndpoint(t *testing.T) {
	baseURLVariants := []string{"/anthropic", "/v1", ""}
	for _, suffix := range baseURLVariants {
		t.Run("suffix_"+strings.Trim(strings.ReplaceAll(suffix, "/", "_"), "_"), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/coding_plan/vlm" {
					t.Fatalf("request path = %q, want /v1/coding_plan/vlm", r.URL.Path)
				}
				if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
					t.Fatalf("authorization header = %q, want Bearer test-key", got)
				}
				if got := r.Header.Get(minimaxImageVisionSourceHeader); got != minimaxImageVisionSourceValue {
					t.Fatalf("source header = %q, want %q", got, minimaxImageVisionSourceValue)
				}
				var req miniMaxImageVisionRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Fatalf("decode request: %v", err)
				}
				if req.Prompt != "Describe the image." {
					t.Fatalf("prompt = %q, want Describe the image.", req.Prompt)
				}
				if req.ImageURL != "data:image/png;base64,ZmFrZS1wbmc=" {
					t.Fatalf("image_url = %q, want data URL", req.ImageURL)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"base_resp":{"status_code":0},"content":"direct minimax analysis"}`))
			}))
			defer server.Close()

			vision := newMiniMaxImageVisionForTest(t, server.URL+suffix)
			analysis, handled, err := vision.Analyze(WithProviderID(context.Background(), "minimax"), "Describe the image.", "ZmFrZS1wbmc=")
			if err != nil {
				t.Fatalf("Analyze error = %v", err)
			}
			if !handled {
				t.Fatal("handled = false, want true")
			}
			if analysis != "direct minimax analysis" {
				t.Fatalf("analysis = %q, want direct minimax analysis", analysis)
			}
		})
	}
}

func TestMiniMaxImageVisionAnalyzeSkipsOtherProviders(t *testing.T) {
	vision := NewMiniMaxImageVision(nil)
	analysis, handled, err := vision.Analyze(WithProviderID(context.Background(), "openai"), "Describe the image.", "ZmFrZS1wbmc=")
	if err != nil {
		t.Fatalf("Analyze error = %v", err)
	}
	if handled {
		t.Fatal("handled = true, want false")
	}
	if analysis != "" {
		t.Fatalf("analysis = %q, want empty", analysis)
	}
}

func newMiniMaxImageVisionForTest(t *testing.T, baseURL string) ProviderAwareImageVision {
	t.Helper()

	storage, err := providerpool.NewFileStorage(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStorage error = %v", err)
	}
	registry, err := providerpool.NewRegistry(storage)
	if err != nil {
		t.Fatalf("NewRegistry error = %v", err)
	}
	if err := registry.Register(&providerpool.Provider{
		ID:      "minimax",
		Name:    "MiniMax",
		Enabled: true,
		BaseURL: baseURL,
		APIKeys: []providerpool.APIKey{{
			Key:     "test-key",
			Enabled: true,
		}},
	}); err != nil {
		t.Fatalf("Register provider error = %v", err)
	}
	return NewMiniMaxImageVision(&providerpool.Pool{Registry: registry})
}
