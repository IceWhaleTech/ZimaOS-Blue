package deepresearch

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestAutoresearchBackendRunStructuredJSON(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell test uses sh")
	}
	backend, err := NewAutoresearchBackend(AutoresearchBackendConfig{
		Command:     "sh",
		Args:        []string{"-c", "cat >/dev/null; printf '{\"summary\":\"experiment ok\",\"confidence\":0.7,\"findings\":[\"one\"]}'"},
		Timeout:     2 * time.Second,
		ArtifactDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("NewAutoresearchBackend() error = %v", err)
	}

	result, err := backend.Run(context.Background(), ExperimentRequest{
		JobID:     "job-1",
		Query:     "benchmark this repo",
		Mode:      ModeStandard,
		RouteMode: RouteModeExperiment,
		Budget:    Budget{MaxSeconds: 60},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Summary != "experiment ok" {
		t.Fatalf("Summary = %q, want %q", result.Summary, "experiment ok")
	}
	if result.Confidence != 0.7 {
		t.Fatalf("Confidence = %v, want 0.7", result.Confidence)
	}
	if got := len(result.Findings); got != 1 {
		t.Fatalf("Findings len = %d, want 1", got)
	}
	if got, _ := result.Metadata["backend"].(string); got != "autoresearch" {
		t.Fatalf("metadata.backend = %q, want %q", got, "autoresearch")
	}
	if got, _ := result.Metadata["artifact_dir"].(string); got != filepath.Join(backend.config.ArtifactDir, "job-1") {
		t.Fatalf("metadata.artifact_dir = %q", got)
	}
}

func TestAutoresearchBackendRequiresCommand(t *testing.T) {
	if _, err := NewAutoresearchBackend(AutoresearchBackendConfig{}); err == nil {
		t.Fatal("expected missing-command error")
	}
}
