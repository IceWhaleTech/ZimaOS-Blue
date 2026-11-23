package harness

import (
	"encoding/json"
	"os"
	"testing"
)

func TestShellEvidenceDataset(t *testing.T) {
	raw, err := os.ReadFile("testdata/shell-evidence.json")
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
	if len(manifest.Items) != 12 {
		t.Fatalf("cases = %d, want 12", len(manifest.Items))
	}
	for _, item := range manifest.Items {
		if len(expectedStrings(item.Expected, "contains")) != 1 || len(decodeStringSlice(item.Expected["required_tool_calls"])) < 1 {
			t.Fatalf("case %s must verify an answer and shell execution", item.ID)
		}
	}
}
