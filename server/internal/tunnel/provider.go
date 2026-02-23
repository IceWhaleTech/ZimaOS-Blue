// Package tunnel provides a unified interface for various tunnel providers.
package tunnel

import (
	"context"
	"time"
)

// Provider represents a tunnel provider type.
type Provider string

const (
	ProviderAuto       Provider = "auto"
	ProviderNgrok      Provider = "ngrok"
	ProviderCloudflare Provider = "cloudflare"
)

// Status represents the current tunnel status.
type Status struct {
	Active        bool      `json:"active"`
	Connecting    bool      `json:"connecting,omitempty"`
	URL           string    `json:"url,omitempty"`
	StartedAt     time.Time `json:"started_at,omitempty"`
	ExpiresAt     time.Time `json:"expires_at,omitempty"`
	RemainingTime string    `json:"remaining_time,omitempty"`
	RenewedCount  int       `json:"renewed_count"`
	Provider      Provider  `json:"provider,omitempty"`
}

// Config represents provider-specific configuration.
type Config struct {
	Provider  Provider `json:"provider"`
	Port      int      `json:"port"`
	Subdomain string   `json:"subdomain,omitempty"` // Custom subdomain for SSH-based tunnels (e.g., "echo-abc123")

	// ngrok specific
	NgrokAuthtoken string `json:"ngrok_authtoken,omitempty"`
	NgrokDomain    string `json:"ngrok_domain,omitempty"` // Custom domain for ngrok (e.g., "myapp.ngrok-free.app")

	// Cloudflare specific
	CloudflareToken    string `json:"cloudflare_token,omitempty"`
	CloudflareTunnelID string `json:"cloudflare_tunnel_id,omitempty"`
}

// Manager defines the interface for tunnel providers.
type Manager interface {
	// Start starts the tunnel with the given configuration.
	Start(ctx context.Context, cfg *Config) error

	// Stop stops the tunnel.
	Stop() error

	// IsRunning returns true if the tunnel is running.
	IsRunning() bool

	// GetStatus returns the current tunnel status.
	GetStatus() Status

	// GetURL returns the current tunnel URL.
	GetURL() string

	// GetProvider returns the provider type.
	GetProvider() Provider

	// SetOnURLChange sets a callback for URL changes.
	SetOnURLChange(fn func(url string))

	// SetOnError sets a callback for errors.
	SetOnError(fn func(err error))
}

// ProviderInfo contains information about a tunnel provider.
type ProviderInfo struct {
	ID          Provider `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	RequiresKey bool     `json:"requires_key"`
	KeyLabel    string   `json:"key_label,omitempty"`
	KeyHint     string   `json:"key_hint,omitempty"`
	DocURL      string   `json:"doc_url,omitempty"`
}

// GetProviderInfos returns information about all available providers for the UI.
func GetProviderInfos() []ProviderInfo {
	return []ProviderInfo{
		{
			ID:          ProviderAuto,
			Name:        "Auto",
			Description: "Cloudflare Quick Tunnel (no signup required)",
			RequiresKey: false,
			DocURL:      "https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/do-more-with-tunnels/trycloudflare/",
		},
		{
			ID:          ProviderNgrok,
			Name:        "ngrok",
			Description: "Popular tunneling service with dashboard and analytics",
			RequiresKey: true,
			KeyLabel:    "Authtoken",
			KeyHint:     "Get your authtoken from ngrok.com/dashboard",
			DocURL:      "https://ngrok.com/docs",
		},
		{
			ID:          ProviderCloudflare,
			Name:        "Cloudflare Tunnel",
			Description: "Enterprise-grade tunnel with Cloudflare's global network",
			RequiresKey: true,
			KeyLabel:    "Tunnel Token",
			KeyHint:     "Create a tunnel at dash.cloudflare.com/zero-trust",
			DocURL:      "https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/",
		},
	}
}
