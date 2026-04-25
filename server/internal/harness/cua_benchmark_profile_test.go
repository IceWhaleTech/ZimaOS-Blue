package harness

import "testing"

func TestCUAOSWorldMacBenchmarkManifestDefinesDefaultCUAProfile(t *testing.T) {
	manifest := CUAOSWorldMacBenchmarkManifest()
	if manifest.Dataset.Name != CUAOSWorldMacBenchmarkDatasetName {
		t.Fatalf("dataset name = %q, want %q", manifest.Dataset.Name, CUAOSWorldMacBenchmarkDatasetName)
	}
	if manifest.Defaults.Profile != CUAOSWorldMacBenchmarkProfile {
		t.Fatalf("profile = %q, want %q", manifest.Defaults.Profile, CUAOSWorldMacBenchmarkProfile)
	}
	if len(manifest.Items) < 5 {
		t.Fatalf("items len = %d, want at least 5", len(manifest.Items))
	}
	families := map[string]bool{}
	critical := 0
	for _, item := range manifest.Items {
		families[metadataString(item.Metadata, "family")] = true
		if item.Metadata["critical"] == true {
			critical++
		}
		for _, key := range []string{"mode", "cua_" + "mode"} {
			if _, ok := item.Input[key]; ok {
				t.Fatalf("item %s input must not carry a mode switch: %#v", item.ID, item.Input)
			}
		}
	}
	for _, family := range []string{"chat", "browser", "settings", "files", "visual_drag"} {
		if !families[family] {
			t.Fatalf("families = %#v, want %s", families, family)
		}
	}
	if critical == 0 {
		t.Fatal("expected at least one critical case")
	}
}

func TestCUAOSWorldMacBenchmarkSpecsCarryLineage(t *testing.T) {
	dataset := CUAOSWorldMacBenchmarkDatasetSpec("user-1")
	if dataset.DefaultProfile != CUAOSWorldMacBenchmarkProfile {
		t.Fatalf("dataset profile = %q, want %q", dataset.DefaultProfile, CUAOSWorldMacBenchmarkProfile)
	}
	if metadataString(dataset.Metadata, "benchmark_style") != "osworld" {
		t.Fatalf("dataset benchmark_style = %q, want osworld", metadataString(dataset.Metadata, "benchmark_style"))
	}
	version, err := CUAOSWorldMacBenchmarkDatasetVersionSpec("user-1")
	if err != nil {
		t.Fatalf("version spec error = %v", err)
	}
	if version.Version != CUAOSWorldMacBenchmarkDatasetVersion {
		t.Fatalf("version = %q, want %q", version.Version, CUAOSWorldMacBenchmarkDatasetVersion)
	}
	eval := CUAOSWorldMacBenchmarkEvalSpecSpec("dataset-1", "version-1", "user-1")
	for _, key := range []string{"mode", "cua_" + "mode"} {
		if _, ok := eval.RuntimePolicy[key]; ok {
			t.Fatalf("eval runtime_policy must not carry a mode switch: %#v", eval.RuntimePolicy)
		}
	}
	if eval.ScoringConfig.Mode != ScoringModeRule {
		t.Fatalf("scoring mode = %q, want rule", eval.ScoringConfig.Mode)
	}
}
