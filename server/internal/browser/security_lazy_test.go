package browser

import (
	"sync"
	"testing"
)

func TestSecurityChecker_URLRegexInitializesOnDemand(t *testing.T) {
	originalPattern := urlInTextPattern
	originalOnce := urlInTextPatternOnce

	urlInTextPattern = nil
	urlInTextPatternOnce = sync.Once{}
	t.Cleanup(func() {
		urlInTextPattern = originalPattern
		urlInTextPatternOnce = originalOnce
	})

	if urlInTextPattern != nil {
		t.Fatal("expected browser URL regex to start nil")
	}

	got := extractURLCandidate("请打开 https://www.zimaos.com/docs，帮我看一下")
	if got != "https://www.zimaos.com/docs" {
		t.Fatalf("extractURLCandidate() = %q, want %q", got, "https://www.zimaos.com/docs")
	}
	if urlInTextPattern == nil {
		t.Fatal("expected browser URL regex to initialize on first extraction")
	}
}
