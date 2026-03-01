package server

import "testing"

func TestSupportsResponsesContinuationModel(t *testing.T) {
	tests := []struct {
		name  string
		model string
		want  bool
	}{
		{name: "explicit responses model", model: "gpt-5-responses", want: true},
		{name: "codex model", model: "gpt-5.3-codex-spark", want: true},
		{name: "regular model", model: "gpt-4.1", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := supportsResponsesContinuation(tc.model); got != tc.want {
				t.Fatalf("supportsResponsesContinuation(%q) = %v, want %v", tc.model, got, tc.want)
			}
		})
	}
}

func TestShouldSkipCompressedTierRecallForContinuation(t *testing.T) {
	tests := []struct {
		name               string
		model              string
		previousResponseID string
		reason             MemoryRecallReason
		want               bool
	}{
		{
			name:               "compressed tier with continuation on codex",
			model:              "gpt-5.3-codex-spark",
			previousResponseID: "resp_123",
			reason:             MemoryRecallReasonCompressedTier,
			want:               true,
		},
		{
			name:               "compressed tier without previous response",
			model:              "gpt-5.3-codex-spark",
			previousResponseID: "",
			reason:             MemoryRecallReasonCompressedTier,
			want:               false,
		},
		{
			name:               "memory cue should not be skipped",
			model:              "gpt-5.3-codex-spark",
			previousResponseID: "resp_123",
			reason:             MemoryRecallReasonMemoryCue,
			want:               false,
		},
		{
			name:               "non responses model should not skip",
			model:              "gpt-4.1",
			previousResponseID: "resp_123",
			reason:             MemoryRecallReasonCompressedTier,
			want:               false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldSkipCompressedTierRecallForContinuation(tc.model, tc.previousResponseID, tc.reason); got != tc.want {
				t.Fatalf("shouldSkipCompressedTierRecallForContinuation(%q,%q,%q) = %v, want %v", tc.model, tc.previousResponseID, tc.reason, got, tc.want)
			}
		})
	}
}
