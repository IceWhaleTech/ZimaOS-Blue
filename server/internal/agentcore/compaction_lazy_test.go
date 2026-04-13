package agentcore

import (
	"sync"
	"testing"
)

func TestCompactionRegexes_InitializeOnDemand(t *testing.T) {
	originalMarkdownLink := reMarkdownFileLink
	originalMarkdownLinkOnce := reMarkdownFileLinkOnce
	originalPatchFileLine := rePatchFileLine
	originalPatchFileLineOnce := rePatchFileLineOnce
	originalPathLineRefSuffix := rePathLineRefSuffix
	originalPathLineRefSuffixOnce := rePathLineRefSuffixOnce
	originalToolNameToken := reToolNameToken
	originalToolNameTokenOnce := reToolNameTokenOnce

	reMarkdownFileLink = nil
	reMarkdownFileLinkOnce = sync.Once{}
	rePatchFileLine = nil
	rePatchFileLineOnce = sync.Once{}
	rePathLineRefSuffix = nil
	rePathLineRefSuffixOnce = sync.Once{}
	reToolNameToken = nil
	reToolNameTokenOnce = sync.Once{}
	t.Cleanup(func() {
		reMarkdownFileLink = originalMarkdownLink
		reMarkdownFileLinkOnce = originalMarkdownLinkOnce
		rePatchFileLine = originalPatchFileLine
		rePatchFileLineOnce = originalPatchFileLineOnce
		rePathLineRefSuffix = originalPathLineRefSuffix
		rePathLineRefSuffixOnce = originalPathLineRefSuffixOnce
		reToolNameToken = originalToolNameToken
		reToolNameTokenOnce = originalToolNameTokenOnce
	})

	if reMarkdownFileLink != nil || rePatchFileLine != nil || rePathLineRefSuffix != nil || reToolNameToken != nil {
		t.Fatal("expected compaction regexes to start nil")
	}

	if got := classifyToolFileAccess("edit-file"); got != "modified" {
		t.Fatalf("classifyToolFileAccess() = %q, want %q", got, "modified")
	}
	if reToolNameToken == nil {
		t.Fatal("expected tool name token regex to initialize on first classifyToolFileAccess call")
	}
	if rePatchFileLine != nil || reMarkdownFileLink != nil || rePathLineRefSuffix != nil {
		t.Fatal("expected other compaction regexes to remain nil after classifyToolFileAccess")
	}

	patches := extractPatchPaths("*** Update File: server/internal/agentcore/compaction.go\n")
	if len(patches) != 1 || patches[0] != "server/internal/agentcore/compaction.go" {
		t.Fatalf("extractPatchPaths() = %v, want extracted patch path", patches)
	}
	if rePatchFileLine == nil {
		t.Fatal("expected patch file regex to initialize on first extractPatchPaths call")
	}
	if reMarkdownFileLink != nil || rePathLineRefSuffix != nil {
		t.Fatal("expected markdown/path suffix regexes to remain nil after extractPatchPaths")
	}

	paths := extractLikelyPathsFromText("[compaction](server/internal/agentcore/compaction.go)")
	if len(paths) == 0 {
		t.Fatal("expected extractLikelyPathsFromText to find at least one path candidate")
	}
	if reMarkdownFileLink == nil {
		t.Fatal("expected markdown link regex to initialize on first extractLikelyPathsFromText call")
	}
	if rePathLineRefSuffix != nil {
		t.Fatal("expected path suffix regex to remain nil until normalizePathCandidate runs")
	}

	if got := normalizePathCandidate("server/internal/agentcore/compaction.go:42"); got != "server/internal/agentcore/compaction.go" {
		t.Fatalf("normalizePathCandidate() = %q, want %q", got, "server/internal/agentcore/compaction.go")
	}
	if rePathLineRefSuffix == nil {
		t.Fatal("expected path line ref suffix regex to initialize on first normalizePathCandidate call")
	}
}
