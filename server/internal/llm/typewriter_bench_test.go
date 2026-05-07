package llm

import (
	"context"
	"testing"
	"time"
)

// TestTypewriterLatency measures the total time to stream text through sendTextChunks.
// Before fix: 1000 chars → ~1-6 seconds of artificial delay from chunking + time.After.
// After fix: 1000 chars → <1ms, single channel send.
func TestTypewriterLatency(t *testing.T) {
	p := &ClaudeProvider{}
	ctx := context.Background()

	text := make([]byte, 1000)
	for i := range text {
		text[i] = 'a' + byte(i%26)
	}
	input := string(text)

	ch := make(chan StreamChunk, 1024)

	start := time.Now()
	p.sendTextChunks(ctx, ch, input)
	elapsed := time.Since(start)

	// Drain channel
	var totalChars int
	for {
		select {
		case chunk := <-ch:
			totalChars += len([]rune(chunk.Delta))
		default:
			goto done
		}
	}
done:

	t.Logf("Input: %d chars", len([]rune(input)))
	t.Logf("Output: %d chars across chunks", totalChars)
	t.Logf("Elapsed: %v", elapsed)

	// After fix: should complete in under 5ms (single channel send)
	// Before fix: would take 1-6 seconds due to time.After delays
	if elapsed > 50*time.Millisecond {
		t.Errorf("sendTextChunks took %v — typewriter delay still present?", elapsed)
	}
}

// TestTypewriterCallbackLatency measures the callback-based variant.
func TestTypewriterCallbackLatency(t *testing.T) {
	p := &ClaudeProvider{}
	ctx := context.Background()

	text := make([]byte, 1000)
	for i := range text {
		text[i] = 'a' + byte(i%26)
	}
	input := string(text)

	var totalChars int
	callback := func(chunk StreamChunk) error {
		totalChars += len([]rune(chunk.Delta))
		return nil
	}

	start := time.Now()
	err := p.sendTextChunksCallback(ctx, input, callback)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Logf("Input: %d chars", len([]rune(input)))
	t.Logf("Output: %d chars", totalChars)
	t.Logf("Elapsed: %v", elapsed)

	if elapsed > 50*time.Millisecond {
		t.Errorf("sendTextChunksCallback took %v — typewriter delay still present?", elapsed)
	}
}

// TestOpenAITypewriterLatency measures the OpenAI provider variant.
func TestOpenAITypewriterLatency(t *testing.T) {
	p := &OpenAIProvider{}
	ctx := context.Background()

	text := make([]byte, 1000)
	for i := range text {
		text[i] = 'a' + byte(i%26)
	}
	input := string(text)

	ch := make(chan StreamChunk, 1024)

	start := time.Now()
	p.sendOpenAITextChunks(ctx, ch, input)
	elapsed := time.Since(start)

	var totalChars int
	for {
		select {
		case chunk := <-ch:
			totalChars += len([]rune(chunk.Delta))
		default:
			goto done
		}
	}
done:

	t.Logf("Input: %d chars", len([]rune(input)))
	t.Logf("Output: %d chars across chunks", totalChars)
	t.Logf("Elapsed: %v", elapsed)

	if elapsed > 50*time.Millisecond {
		t.Errorf("sendOpenAITextChunks took %v — typewriter delay still present?", elapsed)
	}
}

// TestTypewriterCharIntegrity verifies all characters are preserved after streaming.
func TestTypewriterCharIntegrity(t *testing.T) {
	p := &ClaudeProvider{}
	ctx := context.Background()

	// Mix of ASCII and multi-byte runes
	input := "Hello世界🎉" + string(make([]byte, 500))

	ch := make(chan StreamChunk, 1024)
	p.sendTextChunks(ctx, ch, input)

	var output []rune
	for {
		select {
		case chunk := <-ch:
			output = append(output, []rune(chunk.Delta)...)
		default:
			goto done
		}
	}
done:

	inputRunes := []rune(input)
	if len(output) != len(inputRunes) {
		t.Errorf("char count mismatch: input=%d output=%d", len(inputRunes), len(output))
	}
	for i, r := range output {
		if r != inputRunes[i] {
			t.Errorf("char mismatch at position %d: got %c want %c", i, r, inputRunes[i])
			break
		}
	}
}
