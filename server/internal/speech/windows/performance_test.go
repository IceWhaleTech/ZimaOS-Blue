// +build windows

package windows

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"
)

// BenchmarkTTSMemoryUsage measures memory allocation for TTS
func BenchmarkTTSMemoryUsage(b *testing.B) {
	provider := NewWindowsTTSProvider()
	if provider == nil {
		b.Fatal("Failed to create TTS provider")
	}
	defer provider.Close()

	text := "Hello, this is a test of the Windows native text to speech system."

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := &SynthesizeRequest{
			Text:   text,
			Speed:  1.0,
			Pitch:  1.0,
			Volume: 1.0,
		}
		resp, err := provider.Synthesize(context.Background(), req)
		if err != nil {
			b.Fatalf("Synthesis failed: %v", err)
		}
		if len(resp.Audio) == 0 {
			b.Fatal("Empty audio data")
		}
	}
}

// BenchmarkTTSSpeed measures synthesis speed
func BenchmarkTTSSpeed(b *testing.B) {
	provider := NewWindowsTTSProvider()
	if provider == nil {
		b.Fatal("Failed to create TTS provider")
	}
	defer provider.Close()

	testCases := []struct {
		name string
		text string
	}{
		{"Short", "Hello world"},
		{"Medium", "This is a medium length sentence for testing text to speech performance."},
		{"Long", "This is a much longer sentence that contains multiple clauses and phrases, designed to test the performance of the text to speech system with more substantial input text that would be typical in real-world usage scenarios."},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				req := &SynthesizeRequest{
					Text:   tc.text,
					Speed:  1.0,
					Volume: 1.0,
				}
				_, err := provider.Synthesize(context.Background(), req)
				if err != nil {
					b.Fatalf("Synthesis failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkASRMemoryUsage measures memory allocation for ASR
func BenchmarkASRMemoryUsage(b *testing.B) {
	provider := NewWindowsASRProvider("en-US")
	if provider == nil {
		b.Fatal("Failed to create ASR provider")
	}
	defer provider.Close()

	// Create dummy audio data (16kHz, 16-bit, mono, 1 second)
	sampleRate := 16000
	duration := 1 // second
	audioSize := sampleRate * 2 * duration // 2 bytes per sample
	audioData := make([]byte, audioSize)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := &RecognizeRequest{
			Audio:    audioData,
			Language: "en-US",
		}
		_, err := provider.Recognize(context.Background(), req)
		if err != nil {
			// Expected to fail with dummy data, but we're measuring memory
			continue
		}
	}
}

// TestMemoryLeaks checks for memory leaks in repeated operations
func TestMemoryLeaks(t *testing.T) {
	provider := NewWindowsTTSProvider()
	if provider == nil {
		t.Fatal("Failed to create TTS provider")
	}
	defer provider.Close()

	text := "Memory leak test"

	// Force GC and get baseline
	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Run many iterations
	iterations := 100
	for i := 0; i < iterations; i++ {
		req := &SynthesizeRequest{
			Text:   text,
			Speed:  1.0,
			Volume: 1.0,
		}
		resp, err := req.Synthesize(context.Background(), req)
		if err != nil {
			t.Fatalf("Synthesis failed: %v", err)
		}
		if len(resp.Audio) == 0 {
			t.Fatal("Empty audio data")
		}
	}

	// Force GC and measure
	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	// Calculate memory growth
	allocGrowth := m2.TotalAlloc - m1.TotalAlloc
	heapGrowth := m2.HeapAlloc - m1.HeapAlloc

	t.Logf("After %d iterations:", iterations)
	t.Logf("  Total alloc growth: %d bytes (%.2f KB per iteration)",
		allocGrowth, float64(allocGrowth)/float64(iterations)/1024)
	t.Logf("  Heap growth: %d bytes (%.2f KB)",
		heapGrowth, float64(heapGrowth)/1024)

	// Fail if heap grows too much (more than 100KB per iteration suggests leak)
	maxHeapPerIter := int64(100 * 1024)
	if heapGrowth/int64(iterations) > maxHeapPerIter {
		t.Errorf("Possible memory leak: heap grew by %d bytes per iteration",
			heapGrowth/int64(iterations))
	}
}

// BenchmarkConcurrentTTS measures concurrent TTS performance
func BenchmarkConcurrentTTS(b *testing.B) {
	provider := NewWindowsTTSProvider()
	if provider == nil {
		b.Fatal("Failed to create TTS provider")
	}
	defer provider.Close()

	text := "Concurrent test"

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := &SynthesizeRequest{
				Text:   text,
				Speed:  1.0,
				Volume: 1.0,
			}
			_, err := provider.Synthesize(context.Background(), req)
			if err != nil {
				b.Fatalf("Synthesis failed: %v", err)
			}
		}
	})
}

// TestResponseTime measures actual response times
func TestResponseTime(t *testing.T) {
	provider := NewWindowsTTSProvider()
	if provider == nil {
		t.Fatal("Failed to create TTS provider")
	}
	defer provider.Close()

	testCases := []struct {
		name string
		text string
	}{
		{"Short", "Hello"},
		{"Medium", "This is a medium length test sentence."},
		{"Long", "This is a much longer sentence with multiple words and phrases to test the response time of the text to speech system."},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var totalDuration time.Duration
			iterations := 10

			for i := 0; i < iterations; i++ {
				start := time.Now()
				req := &SynthesizeRequest{
					Text:   tc.text,
					Speed:  1.0,
					Volume: 1.0,
				}
				_, err := provider.Synthesize(context.Background(), req)
				duration := time.Since(start)

				if err != nil {
					t.Fatalf("Synthesis failed: %v", err)
				}

				totalDuration += duration
			}

			avgDuration := totalDuration / time.Duration(iterations)
			t.Logf("Average response time: %v", avgDuration)
			t.Logf("Text length: %d characters", len(tc.text))
			t.Logf("Time per character: %.2f ms", float64(avgDuration.Milliseconds())/float64(len(tc.text)))
		})
	}
}
