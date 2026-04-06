package session

import (
	"sync"
	"testing"
)

func resetMemoryHygieneRegexForTest(t *testing.T) {
	t.Helper()

	originalBullet := memoryBulletPrefixRE
	originalWhitespace := memoryWhitespaceRE
	originalTransient := transientMemoryLinePatterns

	memoryBulletPrefixRE = nil
	memoryWhitespaceRE = nil
	transientMemoryLinePatterns = nil
	memoryHygieneRegexOnce = sync.Once{}

	t.Cleanup(func() {
		memoryBulletPrefixRE = originalBullet
		memoryWhitespaceRE = originalWhitespace
		transientMemoryLinePatterns = originalTransient
		memoryHygieneRegexOnce = sync.Once{}
		if originalBullet != nil || originalWhitespace != nil || len(originalTransient) > 0 {
			memoryHygieneRegexOnce.Do(func() {})
		}
	})
}

func TestNormalizeExtractedMemoryForStorage_InitializesRegexOnDemand(t *testing.T) {
	resetMemoryHygieneRegexForTest(t)

	if memoryBulletPrefixRE != nil || memoryWhitespaceRE != nil || transientMemoryLinePatterns != nil {
		t.Fatal("expected memory hygiene regexes to start uninitialized")
	}

	got := NormalizeExtractedMemoryForStorage("- User preference: concise output")
	if got == "" || got == "NO_MEMORY_NEEDED" {
		t.Fatalf("NormalizeExtractedMemoryForStorage() = %q, want normalized durable memory after lazy init", got)
	}
	if memoryBulletPrefixRE == nil || memoryWhitespaceRE == nil || len(transientMemoryLinePatterns) == 0 {
		t.Fatal("expected memory hygiene regexes to initialize on first normalization")
	}
}
