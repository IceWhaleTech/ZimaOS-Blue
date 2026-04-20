package mediagen

import (
	"sync"
	"testing"
)

func resetMediaIntentKeywordsForTest(t *testing.T) {
	t.Helper()

	originalAllLangKeywords := allLangKeywords
	originalCompiledIRLangKeywords := compiledIRLangKeywords

	allLangKeywords = nil
	compiledIRLangKeywords = nil
	mediaIntentKeywordsOnce = sync.Once{}

	t.Cleanup(func() {
		allLangKeywords = originalAllLangKeywords
		compiledIRLangKeywords = originalCompiledIRLangKeywords
		mediaIntentKeywordsOnce = sync.Once{}
		if len(originalAllLangKeywords) > 0 || len(originalCompiledIRLangKeywords) > 0 {
			mediaIntentKeywordsOnce.Do(func() {})
		}
	})
}

func resetMediaPricingForTest(t *testing.T) {
	t.Helper()

	originalBuiltinMediaPricing := builtinMediaPricing

	builtinMediaPricing = nil
	mediaPricingOnce = sync.Once{}

	t.Cleanup(func() {
		builtinMediaPricing = originalBuiltinMediaPricing
		mediaPricingOnce = sync.Once{}
		if len(originalBuiltinMediaPricing) > 0 {
			mediaPricingOnce.Do(func() {})
		}
	})
}

func TestClassifyMediaIntent_InitializesKeywordCatalogOnDemand(t *testing.T) {
	resetMediaIntentKeywordsForTest(t)

	if allLangKeywords != nil || compiledIRLangKeywords != nil {
		t.Fatal("expected media intent keyword globals to start uninitialized")
	}

	intent := ClassifyMediaIntent("generate an image of a cat", false, 0, "en-US")
	if intent == nil {
		t.Fatal("ClassifyMediaIntent() = nil, want lazy initialization to restore keyword catalog")
	}
	if intent.Category != CategoryT2I {
		t.Fatalf("ClassifyMediaIntent().Category = %q, want %q", intent.Category, CategoryT2I)
	}
	if len(allLangKeywords) == 0 || len(compiledIRLangKeywords) == 0 {
		t.Fatal("expected media intent keyword globals to initialize on first classification")
	}
}

func TestGetMediaModelPricing_InitializesBuiltinPricingOnDemand(t *testing.T) {
	resetMediaPricingForTest(t)

	if builtinMediaPricing != nil {
		t.Fatal("expected media pricing map to start uninitialized")
	}

	pricing := GetMediaModelPricing("nano-banana-pro")
	if pricing == nil {
		t.Fatal("GetMediaModelPricing() = nil, want lazy initialization to restore builtin media pricing")
	}
	if pricing.Unit != PricingPerImage {
		t.Fatalf("GetMediaModelPricing().Unit = %q, want %q", pricing.Unit, PricingPerImage)
	}
	if len(builtinMediaPricing) == 0 {
		t.Fatal("expected builtin media pricing map to initialize on first lookup")
	}
}
