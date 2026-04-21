package server

import (
	"sync"
	"testing"
)

func TestArtifactRegexes_InitializeOnDemand(t *testing.T) {
	originalRequested := requestedArtifactPathRegex
	originalSaved := savedGeneratedImagePathRegex
	originalPrep := artifactWriteTargetPrepCueRegex
	originalAs := artifactWriteTargetAsCueRegex
	originalDirect := artifactWriteTargetDirectCueRegex

	requestedArtifactPathRegex = nil
	savedGeneratedImagePathRegex = nil
	artifactWriteTargetPrepCueRegex = nil
	artifactWriteTargetAsCueRegex = nil
	artifactWriteTargetDirectCueRegex = nil
	artifactPathRegexesOnce = sync.Once{}
	artifactWriteTargetRegexesOnce = sync.Once{}
	t.Cleanup(func() {
		requestedArtifactPathRegex = originalRequested
		savedGeneratedImagePathRegex = originalSaved
		artifactWriteTargetPrepCueRegex = originalPrep
		artifactWriteTargetAsCueRegex = originalAs
		artifactWriteTargetDirectCueRegex = originalDirect
		artifactPathRegexesOnce = sync.Once{}
		artifactWriteTargetRegexesOnce = sync.Once{}
	})

	if requestedArtifactPathRegex != nil || savedGeneratedImagePathRegex != nil {
		t.Fatal("expected artifact path regexes to start nil")
	}
	if artifactWriteTargetPrepCueRegex != nil || artifactWriteTargetAsCueRegex != nil || artifactWriteTargetDirectCueRegex != nil {
		t.Fatal("expected artifact write-target regexes to start nil")
	}

	message := "Please write the summary to reports/daily.md and leave scratch.txt untouched."
	candidates := extractArtifactPathCandidates(message)
	if len(candidates) != 2 {
		t.Fatalf("extractArtifactPathCandidates() len = %d, want %d", len(candidates), 2)
	}
	if score := scoreArtifactWriteTargetCandidate(message, candidates[0]); score <= 0 {
		t.Fatalf("scoreArtifactWriteTargetCandidate() = %d, want positive score", score)
	}
	got := extractRequestedArtifactWriteTarget(message)
	if got != "reports/daily.md" {
		t.Fatalf("extractRequestedArtifactWriteTarget() = %q, want %q", got, "reports/daily.md")
	}
	if requestedArtifactPathRegex == nil {
		t.Fatal("expected requested artifact path regex to initialize on first write-target extraction")
	}
	if artifactWriteTargetPrepCueRegex == nil || artifactWriteTargetAsCueRegex == nil || artifactWriteTargetDirectCueRegex == nil {
		t.Fatal("expected artifact write-target regexes to initialize on demand")
	}

	imagePath := extractImageArtifactPathFromMessage(`Saved generated image to "artifacts/cover.png"`)
	if imagePath != "artifacts/cover.png" {
		t.Fatalf("extractImageArtifactPathFromMessage() = %q, want %q", imagePath, "artifacts/cover.png")
	}
	if savedGeneratedImagePathRegex == nil {
		t.Fatal("expected saved image path regex to initialize on demand")
	}
}

func TestExtractRequestedArtifactPath_IgnoresShellSnippetPaths(t *testing.T) {
	message := "/html>\n\nprintf '%s\\n' '<!DOCTYPE html>' '<html lang=\"zh-CN\">' > /Users/orca/.zimaos-blue/data/workspace/solar-system.html && cat /Users/orca/.zimaos-blue/data/workspace/solar-system.html\n\n解析到的目录不对"

	if got := extractRequestedArtifactPath(message); got != "" {
		t.Fatalf("extractRequestedArtifactPath() = %q, want empty for shell snippet", got)
	}
	if got := extractRequestedArtifactWriteTarget(message); got != "" {
		t.Fatalf("extractRequestedArtifactWriteTarget() = %q, want empty for shell snippet", got)
	}
}
