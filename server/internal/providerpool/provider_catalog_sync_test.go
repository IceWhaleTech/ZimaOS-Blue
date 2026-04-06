package providerpool_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providercatalogseed"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
)

type officialProviderCatalog struct {
	Models           map[string][]officialProviderCatalogModel `json:"models"`
	PinchBenchModels map[string][]officialProviderCatalogModel `json:"pinchbench_models,omitempty"`
}

type officialProviderCatalogModel struct {
	ID            string `json:"id"`
	PinchBenchURL string `json:"pinchbench_url,omitempty"`
}

func TestEmbeddedProviderCatalogMatchesDocsCatalog(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	pkgDir := filepath.Dir(thisFile)
	docsPath := filepath.Join(pkgDir, "..", "..", "..", "docs", "provider_catalog.json")
	embeddedPath := filepath.Join(pkgDir, "embedded", "provider_catalog.json")

	docsBytes, err := os.ReadFile(docsPath)
	if err != nil {
		t.Fatalf("read docs catalog: %v", err)
	}
	embeddedBytes, err := os.ReadFile(embeddedPath)
	if err != nil {
		t.Fatalf("read embedded catalog: %v", err)
	}

	if string(docsBytes) != string(embeddedBytes) {
		t.Fatal("embedded provider catalog does not match docs/provider_catalog.json")
	}
}

func TestDocsProviderCatalogCoversBuiltinCatalogModels(t *testing.T) {
	catalog := loadDocsProviderCatalog(t)
	builtinModels := providerpool.BuiltinModelsSnapshot()

	for _, providerID := range []string{"openai", "anthropic", "google", "qwen", "deepseek", "moonshot", "grok", "glm", "minimax", "ollama", "bedrock"} {
		docModels := catalog.Models[providerID]
		docIDs := make(map[string]struct{}, len(docModels))
		for _, model := range docModels {
			docIDs[model.ID] = struct{}{}
		}

		for _, model := range builtinModels[providerID] {
			if model == nil || model.ID == "" {
				continue
			}
			if _, ok := docIDs[model.ID]; !ok {
				t.Fatalf("docs catalog is missing builtin model %q for provider %q", model.ID, providerID)
			}
		}
	}
}

func TestDocsProviderCatalogPinchBenchSubmissionURLsAreValidated(t *testing.T) {
	catalog := loadDocsProviderCatalog(t)
	validSubmissionURL := regexp.MustCompile(`^https://pinchbench\.com/submission/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

	for providerID, models := range catalog.PinchBenchModels {
		for _, model := range models {
			if model.PinchBenchURL == "" {
				continue
			}
			if !validSubmissionURL.MatchString(model.PinchBenchURL) {
				t.Fatalf("invalid pinchbench submission url for %s/%s: %q", providerID, model.ID, model.PinchBenchURL)
			}
		}
	}
}

func TestDocsProviderCatalogMatchesGeneratedCatalog(t *testing.T) {
	docsPath := docsProviderCatalogPath(t)
	docsBytes, err := os.ReadFile(docsPath)
	if err != nil {
		t.Fatalf("read docs catalog: %v", err)
	}

	generatedBytes, err := providercatalogseed.GenerateOfficialProviderCatalogJSON()
	if err != nil {
		t.Fatalf("generate provider catalog json: %v", err)
	}

	if string(docsBytes) != string(generatedBytes) {
		t.Fatal("docs/provider_catalog.json does not match generated provider catalog")
	}
}

func loadDocsProviderCatalog(t *testing.T) officialProviderCatalog {
	t.Helper()

	docsPath := docsProviderCatalogPath(t)
	content, err := os.ReadFile(docsPath)
	if err != nil {
		t.Fatalf("read docs catalog: %v", err)
	}

	var catalog officialProviderCatalog
	if err := json.Unmarshal(content, &catalog); err != nil {
		t.Fatalf("unmarshal docs catalog: %v", err)
	}
	return catalog
}

func docsProviderCatalogPath(t *testing.T) string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "docs", "provider_catalog.json")
}
