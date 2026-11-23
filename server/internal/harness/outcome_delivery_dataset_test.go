package harness

import (
	"encoding/json"
	"os"
	"testing"
)

func TestOutcomeDeliveryDataset(t *testing.T) {
	raw, err := os.ReadFile("testdata/outcome-delivery.json")
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
	if len(manifest.Items) != 9 || len(expectedStrings(manifest.Items[0].Expected, "contains")) != 1 {
		t.Fatal("analysis fixture must check delivery of actual source content")
	}
	if len(expectedStrings(manifest.Items[1].Expected, "required_observations")) != 1 || len(expectedStrings(manifest.Items[1].Expected, "forbidden_observations")) != 2 {
		t.Fatal("web fixture must require actual evidence and reject empty collection")
	}
	item := manifest.Items[6]
	if item.ID != "chat-xinhua-report-recovers-rejected-native-call" || len(expectedStrings(item.Expected, "required_tool_calls")) != 1 || len(expectedStrings(item.Expected, "required_observations")) != 1 {
		t.Fatal("chat news fixture must require execution and evidence before report completion")
	}
}
