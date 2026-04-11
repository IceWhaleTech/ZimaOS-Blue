package providerpool

import "testing"

func TestUsesResponsesIntegration(t *testing.T) {
	tests := []struct {
		name     string
		provider *Provider
		want     bool
	}{
		{
			name:     "codex responses endpoint base url should be detected",
			provider: &Provider{BaseURL: "https://chatgpt.com/backend-api/codex/responses"},
			want:     true,
		},
		{
			name:     "builtin codex provider should be detected",
			provider: &Provider{ID: "openai-codex", APIFormat: APIFormatResponses},
			want:     true,
		},
		{
			name: "codex oauth provider should be detected",
			provider: &Provider{
				ID:    "custom-codex",
				OAuth: &OAuthConfig{ProviderType: "codex"},
			},
			want: true,
		},
		{
			name: "generic responses relay should not be treated as codex integration",
			provider: &Provider{
				ID:        "relay",
				Type:      ProviderTypeCustom,
				APIFormat: APIFormatResponses,
				BaseURL:   "https://relay.example.com/responses",
			},
			want: false,
		},
		{
			name: "generic relay with detected responses format should not be treated as codex integration",
			provider: &Provider{
				ID:             "relay-detected-format",
				Type:           ProviderTypeCustom,
				BaseURL:        "https://relay.example.com",
				DetectedFormat: APIFormatResponses,
			},
			want: false,
		},
		{
			name: "generic relay with detected responses endpoint should not be treated as codex integration",
			provider: &Provider{
				ID:               "relay-detected-endpoint",
				Type:             ProviderTypeCustom,
				BaseURL:          "https://relay.example.com",
				DetectedEndpoint: "https://relay.example.com/v1/responses",
			},
			want: false,
		},
		{
			name:     "plain openai provider should not be treated as responses integration",
			provider: &Provider{ID: "openai", APIFormat: APIFormatOpenAI, BaseURL: "https://api.openai.com/v1"},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := UsesResponsesIntegration(tt.provider); got != tt.want {
				t.Fatalf("UsesResponsesIntegration() = %v, want %v", got, tt.want)
			}
		})
	}
}
