package providerpool

import (
	"sync"
	"testing"
)

func TestProviderpoolRegexes_InitializeOnDemand(t *testing.T) {
	originalProviderIDPattern := providerIDPattern
	originalModelVersionNumberRe := modelVersionNumberRe
	originalDatePatternRe := datePatternRe
	originalDelimiterSplitRe := delimiterSplitRe
	originalPureDigitsRe := pureDigitsRe
	originalMultiHyphenRe := multiHyphenRe
	originalVersionSuffixRe := versionSuffixRe
	originalProviderPrefixRe := providerPrefixRe

	providerIDPattern = nil
	modelVersionNumberRe = nil
	datePatternRe = nil
	delimiterSplitRe = nil
	pureDigitsRe = nil
	multiHyphenRe = nil
	versionSuffixRe = nil
	providerPrefixRe = nil
	providerIDPatternOnce = sync.Once{}
	modelVersionNumberReOnce = sync.Once{}
	pricingMatcherRegexesOnce = sync.Once{}
	t.Cleanup(func() {
		providerIDPattern = originalProviderIDPattern
		modelVersionNumberRe = originalModelVersionNumberRe
		datePatternRe = originalDatePatternRe
		delimiterSplitRe = originalDelimiterSplitRe
		pureDigitsRe = originalPureDigitsRe
		multiHyphenRe = originalMultiHyphenRe
		versionSuffixRe = originalVersionSuffixRe
		providerPrefixRe = originalProviderPrefixRe
		providerIDPatternOnce = sync.Once{}
		modelVersionNumberReOnce = sync.Once{}
		pricingMatcherRegexesOnce = sync.Once{}
	})

	if providerIDPattern != nil || modelVersionNumberRe != nil {
		t.Fatal("expected providerpool identifier and model-version regexes to start nil")
	}
	if datePatternRe != nil || delimiterSplitRe != nil || pureDigitsRe != nil || multiHyphenRe != nil || versionSuffixRe != nil || providerPrefixRe != nil {
		t.Fatal("expected pricing matcher regexes to start nil")
	}

	normalized := normalizeModelID("azure/gpt-4o-2024-08-06")
	if normalized != "gpt-4o" {
		t.Fatalf("normalizeModelID() = %q, want %q", normalized, "gpt-4o")
	}
	if datePatternRe == nil || delimiterSplitRe == nil || pureDigitsRe == nil || multiHyphenRe == nil || versionSuffixRe == nil || providerPrefixRe == nil {
		t.Fatal("expected pricing matcher regexes to initialize on first model normalization")
	}

	keywords := extractKeywords("gpt-4o-mini")
	if len(keywords) != 3 {
		t.Fatalf("extractKeywords() len = %d, want %d", len(keywords), 3)
	}

	if _, err := ValidateProviderID("provider_1"); err != nil {
		t.Fatalf("ValidateProviderID() error = %v, want nil", err)
	}
	if providerIDPattern == nil {
		t.Fatal("expected provider ID regex to initialize on demand")
	}

	if score := modelVersionScore("gpt-5.4-pro"); score <= 0 {
		t.Fatalf("modelVersionScore() = %d, want positive score", score)
	}
	if modelVersionNumberRe == nil {
		t.Fatal("expected model version regex to initialize on demand")
	}
}
