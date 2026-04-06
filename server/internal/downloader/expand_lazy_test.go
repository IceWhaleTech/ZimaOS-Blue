package downloader

import (
	"sync"
	"testing"
)

func TestExpandGitHubURL_InitializesRegexesOnDemand(t *testing.T) {
	originalJsDelivr := reJsDelivr
	originalGitHubRaw := reGitHubRaw

	reJsDelivr = nil
	reGitHubRaw = nil
	expandGitHubRegexesOnce = sync.Once{}
	t.Cleanup(func() {
		reJsDelivr = originalJsDelivr
		reGitHubRaw = originalGitHubRaw
		expandGitHubRegexesOnce = sync.Once{}
	})

	if reJsDelivr != nil || reGitHubRaw != nil {
		t.Fatal("expected downloader expand regexes to start nil")
	}

	got := ExpandGitHubURL("https://raw.githubusercontent.com/openai/openai-go/main/README.md")
	if len(got) != 2 {
		t.Fatalf("ExpandGitHubURL() len = %d, want %d", len(got), 2)
	}
	if reJsDelivr == nil || reGitHubRaw == nil {
		t.Fatal("expected downloader expand regexes to initialize on demand")
	}
}
