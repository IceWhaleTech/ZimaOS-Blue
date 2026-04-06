package routingcue

import (
	"sync"
	"testing"
)

func resetRoutingCueCatalogForTest(t *testing.T) {
	t.Helper()

	originalSupportedLocales := supportedLocales
	originalLocalizedSkillExamples := localizedSkillExamples
	originalLocalizedURLBypassExamples := localizedURLBypassExamples
	originalSupportedLocalesOnce := supportedLocalesOnce
	originalLocalizedSkillExamplesOnce := localizedSkillExamplesOnce
	originalLocalizedURLBypassExamplesOnce := localizedURLBypassExamplesOnce

	supportedLocales = nil
	localizedSkillExamples = nil
	localizedURLBypassExamples = nil
	supportedLocalesOnce = sync.Once{}
	localizedSkillExamplesOnce = sync.Once{}
	localizedURLBypassExamplesOnce = sync.Once{}

	t.Cleanup(func() {
		supportedLocales = originalSupportedLocales
		localizedSkillExamples = originalLocalizedSkillExamples
		localizedURLBypassExamples = originalLocalizedURLBypassExamples
		supportedLocalesOnce = originalSupportedLocalesOnce
		localizedSkillExamplesOnce = originalLocalizedSkillExamplesOnce
		localizedURLBypassExamplesOnce = originalLocalizedURLBypassExamplesOnce
	})
}

func TestRoutingCueCatalog_InitializesOnDemand(t *testing.T) {
	resetRoutingCueCatalogForTest(t)

	if len(supportedLocales) != 0 || localizedSkillExamples != nil || localizedURLBypassExamples != nil {
		t.Fatal("expected routing cue catalog globals to start uninitialized")
	}

	locales := SupportedLocales()
	if len(locales) != 27 {
		t.Fatalf("SupportedLocales() len = %d, want %d", len(locales), 27)
	}
	if len(supportedLocales) == 0 {
		t.Fatal("expected supported locales to initialize on first access")
	}
	if localizedSkillExamples != nil || localizedURLBypassExamples != nil {
		t.Fatal("expected localized example maps to stay uninitialized until requested")
	}

	examples := LocalizedExamples("web_search")
	if len(examples) != len(locales) {
		t.Fatalf("LocalizedExamples(web_search) len = %d, want %d", len(examples), len(locales))
	}
	if len(localizedSkillExamples) == 0 {
		t.Fatal("expected localized skill examples to initialize when requested")
	}
	if localizedURLBypassExamples != nil {
		t.Fatal("expected url bypass examples to stay uninitialized until requested")
	}

	bypass := LocalizedURLBypassExamples("analyze")
	if len(bypass) != len(locales) {
		t.Fatalf("LocalizedURLBypassExamples(analyze) len = %d, want %d", len(bypass), len(locales))
	}
	if len(localizedURLBypassExamples) == 0 {
		t.Fatal("expected localized url bypass examples to initialize when requested")
	}

	if skill, ok := InferSkill("Search the latest OpenAI Responses API documentation."); !ok || skill != "web_query" {
		t.Fatalf("InferSkill() = (%q, %v), want (%q, true)", skill, ok, "web_query")
	}
}
