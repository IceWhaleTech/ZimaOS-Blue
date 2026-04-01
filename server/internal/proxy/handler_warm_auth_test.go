package proxy

import "testing"

func TestWarmAuthProbeModelsURL(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		want    string
	}{
		{name: "plain root", baseURL: "https://relay.example.com", want: "https://relay.example.com/v1/models"},
		{name: "v1 base", baseURL: "https://relay.example.com/v1", want: "https://relay.example.com/v1/models"},
		{name: "models base", baseURL: "https://relay.example.com/v1/models", want: "https://relay.example.com/v1/models"},
		{name: "trailing slash", baseURL: "https://relay.example.com/v1/", want: "https://relay.example.com/v1/models"},
		{name: "empty", baseURL: "", want: "/v1/models"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := warmAuthProbeModelsURL(tc.baseURL); got != tc.want {
				t.Fatalf("warmAuthProbeModelsURL(%q) = %q, want %q", tc.baseURL, got, tc.want)
			}
		})
	}
}
