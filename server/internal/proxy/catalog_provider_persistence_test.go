package proxy

import (
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
)

func TestShouldPersistDetectedFormatForProviderSkipsCatalogProviders(t *testing.T) {
	provider := &providerpool.Provider{
		ID:           "openai",
		Type:         providerpool.ProviderTypeBuiltin,
		BaseURL:      "https://api.openai.com/v1",
		APIFormat:    providerpool.APIFormatOpenAI,
		MetadataMode: providerpool.ProviderMetadataModeCatalog,
	}

	if shouldPersistDetectedFormatForProvider(provider) {
		t.Fatal("expected catalog provider to skip detected format persistence")
	}
}

func TestPersistDetectedEndpointSkipsCatalogProviders(t *testing.T) {
	handler := &ProxyHandler{}
	provider := &providerpool.Provider{
		ID:               "openai",
		Type:             providerpool.ProviderTypeBuiltin,
		BaseURL:          "https://api.openai.com/v1",
		MetadataMode:     providerpool.ProviderMetadataModeCatalog,
		DetectedEndpoint: "https://stale.example/v1",
	}

	handler.persistDetectedEndpoint(provider, "https://alternate.example/v1")

	if provider.DetectedEndpoint != "" {
		t.Fatalf("detected_endpoint = %q, want empty", provider.DetectedEndpoint)
	}
}
