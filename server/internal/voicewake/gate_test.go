package voicewake

import "testing"

func TestTranscriptHelpers(t *testing.T) {
	transcript := "  BLUE, open the settings!  "
	triggers := []string{"Blue"}

	if !TranscriptContainsWakeWord(transcript, triggers) {
		t.Fatal("expected wake word match")
	}

	if got := StripWakeWordFromTranscript(transcript, triggers); got != ", open the settings!" {
		t.Fatalf("StripWakeWordFromTranscript() = %q", got)
	}
}

func TestMatchSegmentsExtractsCommandAfterTriggerGap(t *testing.T) {
	match := MatchSegments(
		"Blue open settings",
		[]Segment{
			{Text: "Blue", Start: 0.00, Duration: 0.20},
			{Text: "open", Start: 0.80, Duration: 0.18},
			{Text: "settings", Start: 1.05, Duration: 0.20},
		},
		GateConfig{
			Triggers:          []string{"Blue"},
			MinPostTriggerGap: 0.45,
			MinCommandLength:  1,
		},
	)
	if match == nil {
		t.Fatal("expected match")
	}
	if !match.TriggerFound {
		t.Fatal("expected trigger_found=true")
	}
	if got := match.Command; got != "open settings" {
		t.Fatalf("match.Command = %q, want %q", got, "open settings")
	}
	if match.PostGapSeconds < 0.45 {
		t.Fatalf("match.PostGapSeconds = %v, want >= 0.45", match.PostGapSeconds)
	}
}

func TestMatchSegmentsKeepsTriggerOnlyWithoutCommand(t *testing.T) {
	match := MatchSegments(
		"Blue",
		[]Segment{{Text: "Blue", Start: 0.0, Duration: 0.2}},
		GateConfig{
			Triggers:          []string{"Blue"},
			MinPostTriggerGap: 0.45,
			MinCommandLength:  1,
		},
	)
	if match == nil {
		t.Fatal("expected match")
	}
	if !match.TriggerFound {
		t.Fatal("expected trigger_found=true")
	}
	if match.Command != "" {
		t.Fatalf("match.Command = %q, want empty", match.Command)
	}
}

func TestCommandTextAfterTriggerFallsBackToTranscript(t *testing.T) {
	got := CommandTextAfterTrigger("turn on the lights", nil, 0.4)
	if got != "turn on the lights" {
		t.Fatalf("CommandTextAfterTrigger() = %q", got)
	}
}
