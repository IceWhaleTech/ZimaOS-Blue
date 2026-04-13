//go:build windows && cgo
// +build windows,cgo

package windows

import (
	"context"
	"testing"
	"time"
)

// TestWindowsTTSProvider_FirstCall tests the very first synthesis call
// This simulates the real-world scenario where the first call might fail
func TestWindowsTTSProvider_FirstCall(t *testing.T) {
	provider := NewWindowsTTSProvider()
	if provider == nil {
		t.Fatal("Failed to create TTS provider")
	}
	defer provider.Close()

	ctx := context.Background()

	// First call - this is where the issue might occur
	req1 := &SynthesizeRequest{
		Text:   "我主要擅长的是软件开发相关的事情，比如：写代码、调试、代码审查",
		Voice:  "",
		Speed:  1.0,
		Pitch:  1.0,
		Volume: 1.0,
	}

	t.Log("Making first synthesis call...")
	resp1, err := provider.Synthesize(ctx, req1)
	if err != nil {
		t.Fatalf("First call failed: %v", err)
	}
	if len(resp1.Audio) == 0 {
		t.Fatal("First call: empty audio")
	}
	t.Logf("First call: generated %d bytes", len(resp1.Audio))

	// Second call - should work
	req2 := &SynthesizeRequest{
		Text:   "这个问题更偏向生活习惯和个人发展方面，不太在我的专长范围内。",
		Voice:  "",
		Speed:  1.0,
		Pitch:  1.0,
		Volume: 1.0,
	}

	t.Log("Making second synthesis call...")
	resp2, err := provider.Synthesize(ctx, req2)
	if err != nil {
		t.Fatalf("Second call failed: %v", err)
	}
	if len(resp2.Audio) == 0 {
		t.Fatal("Second call: empty audio")
	}
	t.Logf("Second call: generated %d bytes", len(resp2.Audio))
}

// TestWindowsTTSProvider_FreshProviderEachTime tests creating a new provider for each call
func TestWindowsTTSProvider_FreshProviderEachTime(t *testing.T) {
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		t.Logf("Iteration %d: creating fresh provider", i)
		provider := NewWindowsTTSProvider()
		if provider == nil {
			t.Fatalf("Iteration %d: Failed to create TTS provider", i)
		}

		req := &SynthesizeRequest{
			Text:   "测试文本，这是第一次调用新创建的provider",
			Voice:  "",
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

		provider.Close()

		// Small delay between iterations
		time.Sleep(100 * time.Millisecond)
	}
}
