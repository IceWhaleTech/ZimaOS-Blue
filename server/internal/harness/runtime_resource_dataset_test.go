package harness

import (
	"encoding/json"
	"os"
	"testing"
)

func TestRuntimeResourceDataset(t *testing.T) {
	raw, err := os.ReadFile("testdata/runtime-resource-limits.json")
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]interface{}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	manifest, err := decodeDatasetManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Items) != 3 {
		t.Fatal("resource fixture must cover pool leases, task duration and disconnected browser cleanup")
	}
}
