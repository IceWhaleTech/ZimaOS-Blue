package server

import "testing"

func TestMergeStreamingToolCallArguments_PreservesRicherPayloadOverEmptyObject(t *testing.T) {
	current := `{"path":"notes.md","content":"hello"}`
	got := mergeStreamingToolCallArguments(current, "{}")
	if got != current {
		t.Fatalf("mergeStreamingToolCallArguments() = %q, want %q", got, current)
	}
}

func TestMergeStreamingToolCallArguments_ReplacesPlaceholderWithCompletePayload(t *testing.T) {
	next := `{"path":"notes.md","content":"hello"}`
	got := mergeStreamingToolCallArguments("{}", next)
	if got != next {
		t.Fatalf("mergeStreamingToolCallArguments() = %q, want %q", got, next)
	}
}

func TestMergeStreamingToolCallArguments_AppendsPartialFragments(t *testing.T) {
	current := `{"path":"notes.md","content":"he`
	next := `llo"}`
	want := `{"path":"notes.md","content":"hello"}`
	got := mergeStreamingToolCallArguments(current, next)
	if got != want {
		t.Fatalf("mergeStreamingToolCallArguments() = %q, want %q", got, want)
	}
}

func TestMergeStreamingToolCallArguments_DoesNotDropClosingQuoteAfterEscapedQuote(t *testing.T) {
	current := `{"command":"echo \"Hello from exec\"`
	next := `"`
	want := `{"command":"echo \"Hello from exec\""`
	got := mergeStreamingToolCallArguments(current, next)
	if got != want {
		t.Fatalf("mergeStreamingToolCallArguments() = %q, want %q", got, want)
	}
}

func TestMergeStreamingToolCallArguments_PreservesClosingQuoteBeforeObjectClosure(t *testing.T) {
	current := `{"command":"echo \"Hello from exec\"`
	current = mergeStreamingToolCallArguments(current, `"`)
	got := mergeStreamingToolCallArguments(current, `}`)
	want := `{"command":"echo \"Hello from exec\""}`
	if got != want {
		t.Fatalf("mergeStreamingToolCallArguments() = %q, want %q", got, want)
	}
}

func TestNormalizeToolCallArgumentsForExecution_CanonicalizesConcatenatedObjects(t *testing.T) {
	got := normalizeToolCallArgumentsForExecution(`{}{"query":"blue"}`)
	if got != `{"query":"blue"}` {
		t.Fatalf("normalizeToolCallArgumentsForExecution() = %q, want %q", got, `{"query":"blue"}`)
	}
}

func TestNormalizeToolCallArgumentsForExecution_EmptyBecomesJSONObject(t *testing.T) {
	got := normalizeToolCallArgumentsForExecution("")
	if got != "{}" {
		t.Fatalf("normalizeToolCallArgumentsForExecution() = %q, want %q", got, "{}")
	}
}
