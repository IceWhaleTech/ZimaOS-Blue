package api

import (
	"context"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/ngrok"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tunnel"
)

func TestTunnelHandler_EnsureTunnelURL_ReusesActiveTunnel(t *testing.T) {
	h := NewTunnelHandler(nil, 8080)
	active := &stubTunnelManager{provider: tunnel.ProviderAuto, running: true, url: "https://blue.example.com"}
	h.managers = map[tunnel.Provider]tunnel.Manager{
		tunnel.ProviderAuto: active,
	}
	h.active = active

	url, err := h.EnsureTunnelURL(context.Background())
	if err != nil {
		t.Fatalf("EnsureTunnelURL error = %v", err)
	}
	if url != "https://blue.example.com" {
		t.Fatalf("url = %q, want %q", url, "https://blue.example.com")
	}
	if active.startCfg != nil {
		t.Fatal("expected EnsureTunnelURL not to restart an already active tunnel")
	}
}

func TestTunnelHandler_EnsureTunnelURL_StartsDefaultProviderWhenInactive(t *testing.T) {
	store := ngrok.NewConfigStore(kvstore.NewMemoryStore())
	_ = store.SaveConfig(context.Background(), &ngrok.RemoteAccessConfig{
		DefaultProvider: "cloudflare",
		CloudflareToken: "cloudflare-token",
	})

	h := NewTunnelHandler(store, 8080)
	autoMgr := &stubTunnelManager{provider: tunnel.ProviderAuto}
	cloudflareMgr := &stubTunnelManager{provider: tunnel.ProviderCloudflare, url: "https://cf.example.com"}
	h.managers = map[tunnel.Provider]tunnel.Manager{
		tunnel.ProviderAuto:       autoMgr,
		tunnel.ProviderCloudflare: cloudflareMgr,
	}

	url, err := h.EnsureTunnelURL(context.Background())
	if err != nil {
		t.Fatalf("EnsureTunnelURL error = %v", err)
	}
	if url != "https://cf.example.com" {
		t.Fatalf("url = %q, want %q", url, "https://cf.example.com")
	}
	if cloudflareMgr.startCfg == nil {
		t.Fatal("expected default cloudflare manager to be started")
	}
	if cloudflareMgr.startCfg.CloudflareToken != "cloudflare-token" {
		t.Fatalf("cloudflare token = %q, want %q", cloudflareMgr.startCfg.CloudflareToken, "cloudflare-token")
	}
	if autoMgr.startCfg != nil {
		t.Fatal("expected auto manager to remain idle when default provider is cloudflare")
	}
}
