package providerpool

import (
	"testing"
	"time"
)

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
