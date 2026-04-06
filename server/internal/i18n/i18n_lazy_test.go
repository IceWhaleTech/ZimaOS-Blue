package i18n

import (
	"sync"
	"testing"
)

func TestT_InitializesTranslationsOnDemand(t *testing.T) {
	mu.Lock()
	originalTranslations := translations
	originalBuildTarget := translationBuildTarget
	translations = nil
	translationBuildTarget = nil
	translationsOnce = sync.Once{}
	mu.Unlock()

	t.Cleanup(func() {
		mu.Lock()
		translations = originalTranslations
		translationBuildTarget = originalBuildTarget
		translationsOnce = sync.Once{}
		mu.Unlock()

		ensureTranslations()
	})

	got := T(LangZhCN, MsgMediaGenerating)
	if got == MsgMediaGenerating {
		t.Fatalf("T(%q, %q) returned key itself, want lazy initialization to restore translations", LangZhCN, MsgMediaGenerating)
	}
}
