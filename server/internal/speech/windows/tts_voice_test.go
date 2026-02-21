// +build windows

package windows

import (
	"context"
	"testing"
)

// TestWindowsTTSProvider_WithDifferentVoices tests synthesis with different voice parameters
func TestWindowsTTSProvider_WithDifferentVoices(t *testing.T) {
	provider := NewWindowsTTSProvider()
	if provider == nil {
		t.Fatal("Failed to create TTS provider")
	}
	defer provider.Close()

	ctx := context.Background()

	// Get available voices
	voices, err := provider.ListVoices(ctx)
	if err != nil {
		t.Fatalf("Failed to list voices: %v", err)
	}
	if len(voices) == 0 {
		t.Fatal("No voices available")
	}

	t.Logf("Found %d voices", len(voices))

	// Test with first voice explicitly set
	req1 := &SynthesizeRequest{
		Text:   "Testing with explicit voice",
		Voice:  voices[0].ID,
		Speed:  1.0,
		Pitch:  1.0,
		Volume: 1.0,
	}
	resp1, err := provider.Synthesize(ctx, req1)
	if err != nil {
		t.Fatalf("Synthesis with explicit voice failed: %v", err)
	}
	if len(resp1.Audio) == 0 {
		t.Fatal("Empty audio with explicit voice")
	}
	t.Logf("Explicit voice: generated %d bytes", len(resp1.Audio))

	// Test with empty voice (auto-detect)
	req2 := &SynthesizeRequest{
		Text:   "Testing with auto voice detection",
		Voice:  "",
		Speed:  1.0,
		Pitch:  1.0,
		Volume: 1.0,
	}
	resp2, err := provider.Synthesize(ctx, req2)
	if err != nil {
		t.Fatalf("Synthesis with auto voice failed: %v", err)
	}
	if len(resp2.Audio) == 0 {
		t.Fatal("Empty audio with auto voice")
	}
	t.Logf("Auto voice: generated %d bytes", len(resp2.Audio))

	// Test with different parameters
	req3 := &SynthesizeRequest{
		Text:   "Testing with different parameters",
		Voice:  "",
		Speed:  1.5,
		Pitch:  1.2,
		Volume: 0.8,
	}
	resp3, err := provider.Synthesize(ctx, req3)
	if err != nil {
		t.Fatalf("Synthesis with different params failed: %v", err)
	}
	if len(resp3.Audio) == 0 {
		t.Fatal("Empty audio with different params")
	}
	t.Logf("Different params: generated %d bytes", len(resp3.Audio))
}

// TestWindowsTTSProvider_EmptyVoiceRepeated tests repeated calls with empty voice
func TestWindowsTTSProvider_EmptyVoiceRepeated(t *testing.T) {
	provider := NewWindowsTTSProvider()
	if provider == nil {
		t.Fatal("Failed to create TTS provider")
	}
	defer provider.Close()

	ctx := context.Background()

	for i := 0; i < 10; i++ {
		req := &SynthesizeRequest{
			Text:   "Testing repeated synthesis with empty voice",
			Voice:  "", // Empty voice - triggers auto-detection
			Speed:  1.0,
			Pitch:  1.0,
			Volume: 1.0,
		}
		resp, err := provider.Synthesize(ctx, req)
		if err != nil {
			t.Fatalf("Iteration %d failed: %v", i, err)
		}
		if len(resp.Audio) == 0 {
			t.Fatalf("Iteration %d: empty audio", i)
		}
		t.Logf("Iteration %d: generated %d bytes", i, len(resp.Audio))
	}
}
