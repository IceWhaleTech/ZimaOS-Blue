package tools

import (
	"testing"
	"time"
)

func TestRegisterBuiltinToolsWithConfig_AppliesWebFetchConfig(t *testing.T) {
	registry := NewRegistry()
	RegisterBuiltinToolsWithConfig(registry, WebSearchConfig{}, WebFetchConfig{
		AllowPrivateHosts:     true,
		Timeout:               7 * time.Second,
		HTTPNativeEnabled:     true,
		HTTPNativeLibrary:     "/usr/lib/libcurl.4.dylib",
		HTTPNativePreferHosts: []string{"thepaper.cn"},
		FirecrawlTimeout:      11 * time.Second,
		JinaReaderEnabled:     true,
		JinaReaderTimeout:     13 * time.Second,
		ProxyFetcherProviders: []string{webFetchProxyProviderFirecrawl, webFetchProxyProviderJinaReader},
	}, nil, 0)

	tool := GetWebFetchTool(registry)
	if tool == nil {
		t.Fatal("expected web_fetch tool to be registered")
	}
	if !registry.IsDisabled("web_fetch") {
		t.Fatal("expected legacy web_fetch tool to be hidden")
	}
	if GetWebQueryTool(registry) == nil {
		t.Fatal("expected unified web_query tool to be registered")
	}
	if registry.Get("web") != nil {
		t.Fatal("expected legacy web alias to be removed entirely")
	}
	if !tool.config.AllowPrivateHosts {
		t.Fatal("expected AllowPrivateHosts to be applied")
	}
	if tool.config.Timeout != 7*time.Second {
		t.Fatalf("timeout = %v, want %v", tool.config.Timeout, 7*time.Second)
	}
	if !tool.config.HTTPNativeEnabled {
		t.Fatal("expected HTTPNativeEnabled to be applied")
	}
	if tool.config.HTTPNativeLibrary != "/usr/lib/libcurl.4.dylib" {
		t.Fatalf("http native library = %q, want %q", tool.config.HTTPNativeLibrary, "/usr/lib/libcurl.4.dylib")
	}
	if len(tool.config.HTTPNativePreferHosts) != 1 || tool.config.HTTPNativePreferHosts[0] != "thepaper.cn" {
		t.Fatalf("http native prefer hosts = %v, want [thepaper.cn]", tool.config.HTTPNativePreferHosts)
	}
	if tool.config.FirecrawlTimeout != 11*time.Second {
		t.Fatalf("firecrawl timeout = %v, want %v", tool.config.FirecrawlTimeout, 11*time.Second)
	}
	if !tool.config.JinaReaderEnabled {
		t.Fatal("expected JinaReaderEnabled to be applied")
	}
	if tool.config.JinaReaderTimeout != 13*time.Second {
		t.Fatalf("jina reader timeout = %v, want %v", tool.config.JinaReaderTimeout, 13*time.Second)
	}
	if len(tool.config.ProxyFetcherProviders) != 2 {
		t.Fatalf("proxy fetcher providers = %v, want firecrawl+jina_reader", tool.config.ProxyFetcherProviders)
	}
}
