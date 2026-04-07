package providerpool

import (
	"testing"
	"time"
)

type countingProviderSaveStorage struct {
	stubStorage
	saveCalls int
}

func (s *countingProviderSaveStorage) SaveProvider(*Provider) error {
	s.saveCalls++
	return nil
}

func TestSyncBuiltinModels_SkipsBuiltinLookupWithoutStoredModels(t *testing.T) {
	originalLookup := builtinModelsProviderLookup
	lookupCalls := 0
	builtinModelsProviderLookup = func(providerID string) []*Model {
		lookupCalls++
		return []*Model{{ID: "gpt-4o", ProviderID: providerID}}
	}
	t.Cleanup(func() {
		builtinModelsProviderLookup = originalLookup
	})

	syncBuiltinModels(stubStorage{}, "openai")

	if lookupCalls != 0 {
		t.Fatalf("builtin model lookup calls = %d, want 0 when storage has no models", lookupCalls)
	}
}

func TestPoolApplyOfficialProviderCatalog_LoadsBuiltinModelSnapshotOnce(t *testing.T) {
	originalSnapshotLookup := builtinModelsSnapshotLookup
	snapshotCalls := 0
	builtinModelsSnapshotLookup = func() map[string][]*Model {
		snapshotCalls++
		return map[string][]*Model{}
	}
	t.Cleanup(func() {
		builtinModelsSnapshotLookup = originalSnapshotLookup
	})

	storage := stubStorage{}
	registry, err := NewRegistry(storage)
	if err != nil {
		t.Fatalf("NewRegistry failed: %v", err)
	}

	pool := &Pool{
		Registry:  registry,
		Storage:   storage,
		Discovery: NewModelDiscovery(registry, storage, time.Hour),
	}

	pool.applyOfficialProviderCatalog()

	if snapshotCalls != 1 {
		t.Fatalf("builtin model snapshot calls = %d, want 1", snapshotCalls)
	}
}

func TestPoolInitBuiltinProviders_DoesNotPersistBuiltinDefaultsIntoEmptyStorage(t *testing.T) {
	storage := &countingProviderSaveStorage{}
	registry, err := NewRegistry(storage)
	if err != nil {
		t.Fatalf("NewRegistry failed: %v", err)
	}

	pool := &Pool{
		Registry:  registry,
		Storage:   storage,
		Discovery: NewModelDiscovery(registry, storage, time.Hour),
	}

	pool.initBuiltinProviders()

	if storage.saveCalls != 0 {
		t.Fatalf("SaveProvider() calls = %d, want 0 for builtin defaults on empty storage", storage.saveCalls)
	}
	if got := len(registry.List()); got == 0 {
		t.Fatal("expected builtin providers to still register in memory")
	}
}

func TestNewPool_DefersEmbeddedOfficialProviderCatalogUntilFirstBuiltinAccess(t *testing.T) {
	ClearOfficialProviderCatalog()
	officialProviderCatalogLoadState.mu.Lock()
	officialProviderCatalogLoadState.loaded = false
	officialProviderCatalogLoadState.mu.Unlock()
	t.Cleanup(func() {
		ClearOfficialProviderCatalog()
		officialProviderCatalogLoadState.mu.Lock()
		officialProviderCatalogLoadState.loaded = false
		officialProviderCatalogLoadState.mu.Unlock()
	})

	pool, err := NewPool(t.TempDir())
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}
	_ = pool

	officialProviderCatalogState.mu.RLock()
	initialProviders := len(officialProviderCatalogState.catalog.Providers)
	initialModels := len(officialProviderCatalogState.catalog.Models)
	officialProviderCatalogState.mu.RUnlock()
	if initialProviders != 0 || initialModels != 0 {
		t.Fatalf("expected official provider catalog to stay cold after NewPool, got providers=%d models=%d", initialProviders, initialModels)
	}

	if got := len(BuiltinProviders()); got == 0 {
		t.Fatal("expected builtin providers to remain available")
	}

	officialProviderCatalogState.mu.RLock()
	loadedProviders := len(officialProviderCatalogState.catalog.Providers)
	loadedModels := len(officialProviderCatalogState.catalog.Models)
	officialProviderCatalogState.mu.RUnlock()
	if loadedProviders == 0 || loadedModels == 0 {
		t.Fatalf("expected official provider catalog to load on first builtin access, got providers=%d models=%d", loadedProviders, loadedModels)
	}
}
