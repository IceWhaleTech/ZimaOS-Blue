package optimization

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"
)

func TestNormalizeRequestedEvolvablePartsCanonicalizesAndOrdersValues(t *testing.T) {
	got, err := NormalizeRequestedEvolvableParts([]string{
		" context_assembly ",
		"runner_code",
		"prompt_template",
		"runner_code",
	})
	if err != nil {
		t.Fatalf("NormalizeRequestedEvolvableParts returned error: %v", err)
	}
	want := []string{"prompt_template", "context_assembly", "runner_code"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeRequestedEvolvableParts = %#v, want %#v", got, want)
	}
}

func TestNormalizeRequestedEvolvablePartsRejectsUnknownValues(t *testing.T) {
	if _, err := NormalizeRequestedEvolvableParts([]string{"prompt_template", "mystery_surface"}); err == nil {
		t.Fatal("NormalizeRequestedEvolvableParts succeeded, want error")
	}
}

func TestGetStatusIncludesEvolvablePartsFromRunnerManifest(t *testing.T) {
	root := t.TempDir()
	binaryPath := filepath.Join(root, "bin", RunnerBinaryName(runtime.GOOS))
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
		t.Fatalf("mkdir bin dir: %v", err)
	}
	if err := os.WriteFile(binaryPath, []byte("runner"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	writeOptimizationStatusFixture(t, root, Status{
		BinaryReady: true,
		BinaryPath:  binaryPath,
	})
	builtAt := time.Date(2026, 4, 4, 10, 11, 12, 0, time.UTC)
	writeRunnerManifestFixture(t, binaryPath, RunnerArtifactManifest{
		SchemaVersion:         RunnerArtifactManifestSchemaVersion,
		BinarySHA256:          "deadbeef",
		RepoURL:               "https://github.com/example/runner",
		Ref:                   "main",
		Commit:                "abc123",
		SupportedParts:        DefaultSupportedEvolvableParts(),
		OptimizedParts:        []string{"prompt_template", "context_assembly"},
		PrimaryPart:           "prompt_template",
		SourceOptimizationRun: "opt-123",
		SourceEvalRun:         "eval-456",
		BuiltAt:               builtAt,
	})

	manager, err := NewManager(root)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	status := manager.GetStatus(context.Background())
	if got := status.ManifestPath; got != RunnerArtifactManifestPath(binaryPath) {
		t.Fatalf("ManifestPath = %q, want %q", got, RunnerArtifactManifestPath(binaryPath))
	}
	if !reflect.DeepEqual(status.SupportedParts, DefaultSupportedEvolvableParts()) {
		t.Fatalf("SupportedParts = %#v", status.SupportedParts)
	}
	if want := []string{"prompt_template", "context_assembly"}; !reflect.DeepEqual(status.OptimizedParts, want) {
		t.Fatalf("OptimizedParts = %#v, want %#v", status.OptimizedParts, want)
	}
	if got := status.PrimaryPart; got != "prompt_template" {
		t.Fatalf("PrimaryPart = %q, want prompt_template", got)
	}
	if got := status.SourceOptimizationRunID; got != "opt-123" {
		t.Fatalf("SourceOptimizationRunID = %q, want opt-123", got)
	}
	if got := status.SourceEvalRunID; got != "eval-456" {
		t.Fatalf("SourceEvalRunID = %q, want eval-456", got)
	}
}

func TestEnrichOptimizationRunRecordEvolvablePartsFallsBackToLegacySurface(t *testing.T) {
	record := OptimizationRunRecord{
		"id":                   "opt-legacy",
		"optimization_surface": "runner_code",
	}

	enrichOptimizationRunRecordEvolvableParts(record, "")

	if got := record["primary_part"]; got != "runner_code" {
		t.Fatalf("primary_part = %#v, want runner_code", got)
	}
	gotParts, _ := record["optimized_parts"].([]string)
	if !reflect.DeepEqual(gotParts, []string{"runner_code"}) {
		t.Fatalf("optimized_parts = %#v, want [runner_code]", record["optimized_parts"])
	}
	gotSupported, _ := record["supported_parts"].([]string)
	if !reflect.DeepEqual(gotSupported, DefaultSupportedEvolvableParts()) {
		t.Fatalf("supported_parts = %#v", record["supported_parts"])
	}
}

func writeRunnerManifestFixture(t *testing.T, binaryPath string, manifest RunnerArtifactManifest) {
	t.Helper()
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	path := RunnerArtifactManifestPath(binaryPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir manifest dir: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}
