package tools

import (
	"sync"
	"testing"
)

func TestStructuredWorkspaceArtifactRegex_InitializeOnDemand(t *testing.T) {
	original := structuredWorkspaceArtifactPathRegex
	originalOnce := structuredWorkspaceArtifactRegexOnce

	structuredWorkspaceArtifactPathRegex = nil
	structuredWorkspaceArtifactRegexOnce = sync.Once{}
	t.Cleanup(func() {
		structuredWorkspaceArtifactPathRegex = original
		structuredWorkspaceArtifactRegexOnce = originalOnce
	})

	if structuredWorkspaceArtifactPathRegex != nil {
		t.Fatal("expected structured workspace artifact regex to start nil")
	}

	got := countStructuredWorkspaceArtifactPaths("read notes.md and data/report.csv, then write summary.md")
	if got != 3 {
		t.Fatalf("countStructuredWorkspaceArtifactPaths() = %d, want 3", got)
	}

	if structuredWorkspaceArtifactPathRegex == nil {
		t.Fatal("expected structured workspace artifact regex to initialize on first path count")
	}
}
