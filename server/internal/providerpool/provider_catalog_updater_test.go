package providerpool

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/downloader"
)

func TestProviderCatalogUpdaterFetchAndApply(t *testing.T) {
	ClearOfficialProviderCatalog()
	defer ClearOfficialProviderCatalog()

	applied := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"catalog-v1"`)
		_, _ = w.Write([]byte(`{
			"providers": {
				"openai": {
					"base_url": "https://catalog.example/v1",
					"metadata_mode": "catalog"
				}
			},
			"models": {
				"openai": [
					{
						"id": "gpt-4o",
						"display_name": "Catalog GPT-4o",
						"context_window": 999,
						"max_output": 111,
						"capabilities": ["chat", "streaming"]
					}
				]
			}
		}`))
	}))
	defer srv.Close()

	updater := &ProviderCatalogUpdater{
		dl: &downloader.Downloader{
			URL:     srv.URL,
			Timeout: 2 * time.Second,
		},
		interval:      6 * time.Hour,
		stopCh:        make(chan struct{}),
		fallbackInUse: true,
		onApplied: func() {
			applied++
		},
	}

	updater.fetchAndApply()
	status := updater.Status()
	if status.FallbackInUse {
		t.Fatal("expected remote catalog to replace fallback")
	}
	if status.ETag != `"catalog-v1"` {
		t.Fatalf("expected ETag to be stored, got %q", status.ETag)
	}
	if status.LastUpdatedAt.IsZero() {
		t.Fatal("expected last updated time to be set")
	}
	if applied != 1 {
		t.Fatalf("expected apply callback once, got %d", applied)
	}

	openAIProvider := GetBuiltinProvider("openai")
	if openAIProvider == nil {
		t.Fatal("expected openai provider")
	}
	if openAIProvider.BaseURL != "https://catalog.example/v1" {
		t.Fatalf("expected provider override to apply, got %q", openAIProvider.BaseURL)
	}

	models := GetBuiltinModels("openai")
	if len(models) == 0 {
		t.Fatal("expected openai models")
	}
	if models[0].DisplayName != "Catalog GPT-4o" {
		t.Fatalf("expected model display name override, got %q", models[0].DisplayName)
	}
	if models[0].ContextWindow != 999 {
		t.Fatalf("expected context window override, got %d", models[0].ContextWindow)
	}

	updater.fetchAndApply()
	if applied != 1 {
		t.Fatalf("expected unchanged ETag to skip reapply, got %d applies", applied)
	}
}
