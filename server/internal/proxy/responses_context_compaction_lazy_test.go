package proxy

import (
	"sync"
	"testing"
)

func TestResponsesContinuationRegexes_InitializeOnDemand(t *testing.T) {
	originalChoice := reContinuationChoice
	originalContext := reContinuationContextCue
	originalOrdinal := reContinuationOrdinalCue
	originalOptions := reAssistantOptionLine

	reContinuationChoice = nil
	reContinuationContextCue = nil
	reContinuationOrdinalCue = nil
	reAssistantOptionLine = nil
	responsesContinuationRegexesOnce = sync.Once{}
	t.Cleanup(func() {
		reContinuationChoice = originalChoice
		reContinuationContextCue = originalContext
		reContinuationOrdinalCue = originalOrdinal
		reAssistantOptionLine = originalOptions
		responsesContinuationRegexesOnce = sync.Once{}
	})

	if reContinuationChoice != nil || reContinuationContextCue != nil || reContinuationOrdinalCue != nil || reAssistantOptionLine != nil {
		t.Fatal("expected continuation regexes to start nil")
	}

	if !isLikelyContextDependentContinuationText("第3个") {
		t.Fatal("expected ordinal continuation cue to be recognized after lazy init")
	}
	if reContinuationChoice == nil || reContinuationContextCue == nil || reContinuationOrdinalCue == nil {
		t.Fatal("expected continuation context regexes to initialize on demand")
	}

	compressed, err := heuristicResponsesContextCompressor{}.CompressAssistantContext(ResponsesAssistantCompressionInput{
		AssistantText: "A. first option\nB. second option",
		Mode:          ResponsesAssistantCompressionModeChoice,
		MaxRunes:      100,
	})
	if err != nil {
		t.Fatalf("CompressAssistantContext() error = %v", err)
	}
	if compressed == "" {
		t.Fatal("expected compressed assistant context")
	}
	if reAssistantOptionLine == nil {
		t.Fatal("expected assistant option regex to initialize on demand")
	}
}
