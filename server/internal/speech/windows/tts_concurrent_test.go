// +build windows

package windows

import (
	"context"
	"sync"
	"testing"
)

// TestWindowsTTSProvider_Concurrent tests concurrent synthesis calls
// This simulates the real-world scenario where multiple goroutines
// use the same TTS provider instance
func TestWindowsTTSProvider_Concurrent(t *testing.T) {
	provider := NewWindowsTTSProvider()
	if provider == nil {
		t.Fatal("Failed to create TTS provider")
	}
	defer provider.Close()

	ctx := context.Background()
	numGoroutines := 10
	numCallsPerGoroutine := 5

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*numCallsPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numCallsPerGoroutine; j++ {
				req := &SynthesizeRequest{
					Text:   "Hello from goroutine",
					Speed:  1.0,
					Pitch:  1.0,
					Volume: 1.0,
				}
				resp, err := provider.Synthesize(ctx, req)
				if err != nil {
					errors <- err
					t.Logf("Goroutine %d, call %d failed: %v", id, j, err)
				} else if len(resp.Audio) == 0 {
					t.Logf("Goroutine %d, call %d: empty audio", id, j)
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	errorCount := 0
	for err := range errors {
		errorCount++
		t.Logf("Error: %v", err)
	}

	if errorCount > 0 {
		t.Fatalf("Got %d errors out of %d total calls", errorCount, numGoroutines*numCallsPerGoroutine)
	}
}

// TestWindowsTTSProvider_SequentialReuse tests sequential reuse of the same provider
func TestWindowsTTSProvider_SequentialReuse(t *testing.T) {
	provider := NewWindowsTTSProvider()
	if provider == nil {
		t.Fatal("Failed to create TTS provider")
	}
	defer provider.Close()

	ctx := context.Background()

	for i := 0; i < 20; i++ {
		req := &SynthesizeRequest{
			Text:   "Sequential test iteration",
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
