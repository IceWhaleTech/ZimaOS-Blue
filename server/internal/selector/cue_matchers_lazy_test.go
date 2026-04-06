package selector

import "testing"

func TestLazyCueMatchers_InitializesOnDemand(t *testing.T) {
	matchers := newLazyCueMatchers()

	if matchers.questionPrefix.value != nil {
		t.Fatal("expected question prefix matcher to start nil")
	}
	if matchers.usageMeta.value != nil {
		t.Fatal("expected usage meta matcher to start nil")
	}

	if !matchers.questionPrefixMatcher().HasAnyPrefix("what is this") {
		t.Fatal("expected question prefix matcher to work after first access")
	}
	if matchers.questionPrefix.value == nil {
		t.Fatal("expected question prefix matcher to initialize on first access")
	}
	if matchers.usageMeta.value != nil {
		t.Fatal("expected unrelated matcher to remain nil until accessed")
	}

	if !matchers.usageMetaTermMatcher().ContainsAnyFold("how to use this tool mode") {
		t.Fatal("expected usage meta matcher to work after first access")
	}
	if matchers.usageMeta.value == nil {
		t.Fatal("expected usage meta matcher to initialize on first access")
	}
}

func TestAnalyzeQuery_UsesLazyStaticCueMatchers(t *testing.T) {
	previous := staticCueMatchers
	matchers := newLazyCueMatchers()
	staticCueMatchers = matchers
	defer func() {
		staticCueMatchers = previous
	}()

	if matchers.questionPrefix.value != nil || matchers.liveWeb.value != nil || matchers.urlPresent.value != nil {
		t.Fatal("expected static cue matchers to start cold")
	}

	signals := AnalyzeQuery("What is the latest official documentation website?")
	if !signals.QuestionPrefix || !signals.LiveWeb {
		t.Fatalf("expected lazy static cue matchers to preserve AnalyzeQuery behavior, got %+v", signals)
	}
	if matchers.questionPrefix.value == nil {
		t.Fatal("expected AnalyzeQuery to initialize question prefix matcher on demand")
	}
	if matchers.liveWeb.value == nil {
		t.Fatal("expected AnalyzeQuery to initialize live web matcher on demand")
	}
	if matchers.urlPresent.value == nil {
		t.Fatal("expected AnalyzeQuery to initialize URL matcher on demand")
	}
}
