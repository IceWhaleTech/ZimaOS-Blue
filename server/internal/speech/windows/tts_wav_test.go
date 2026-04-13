//go:build windows && cgo
// +build windows,cgo

package windows

import (
	"context"
	"encoding/hex"
	"testing"
)

// TestWindowsTTSProvider_WAVHeader tests if the output has a valid WAV header
func TestWindowsTTSProvider_WAVHeader(t *testing.T) {
	provider := NewWindowsTTSProvider()
	if provider == nil {
		t.Fatal("Failed to create TTS provider")
	}
	defer provider.Close()

	ctx := context.Background()

	req := &SynthesizeRequest{
		Text:   "Hello world",
		Voice:  "",
		Speed:  1.0,
		Pitch:  1.0,
		Volume: 1.0,
	}

	resp, err := provider.Synthesize(ctx, req)
	if err != nil {
		t.Fatalf("Synthesis failed: %v", err)
	}
	if len(resp.Audio) == 0 {
		t.Fatal("Empty audio")
	}

	// Check WAV header
	// A valid WAV file should start with "RIFF" (0x52494646)
	if len(resp.Audio) < 44 {
		t.Fatalf("Audio too short (%d bytes), should be at least 44 bytes for WAV header", len(resp.Audio))
	}

	header := resp.Audio[:44]
	t.Logf("First 44 bytes (hex): %s", hex.EncodeToString(header))
	t.Logf("First 4 bytes (string): %s", string(header[:4]))

	// Check RIFF header
	if string(header[:4]) != "RIFF" {
		t.Errorf("Invalid WAV header: expected 'RIFF', got '%s'", string(header[:4]))
		t.Logf("First 100 bytes: %s", hex.EncodeToString(resp.Audio[:min(100, len(resp.Audio))]))
	}

	// Check WAVE format
	if string(header[8:12]) != "WAVE" {
		t.Errorf("Invalid WAV format: expected 'WAVE', got '%s'", string(header[8:12]))
	}

	t.Logf("WAV header looks valid, total size: %d bytes", len(resp.Audio))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
