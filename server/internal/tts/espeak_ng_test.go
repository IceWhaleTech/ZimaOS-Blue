//go:build espeak

package tts

import (
	"context"
	"io"
	"os"
	"testing"
)

func TestEspeakNGProvider(t *testing.T) {
	// Use the built espeak-ng-data path
	dataPath := "../../../third_party/espeak-ng/build"

	provider := NewEspeakNGProvider(dataPath)
	defer provider.Close()

	// Test English
	t.Run("English", func(t *testing.T) {
		req := &EspeakNGRequest{
			Text:     "Hello, this is a test of eSpeak NG.",
			Language: "en",
			Rate:     175,
			Pitch:    50,
			Volume:   100,
		}

		audio, err := provider.Synthesize(context.Background(), req)
		if err != nil {
			t.Fatalf("Synthesize failed: %v", err)
		}
		defer audio.Close()

		data, err := io.ReadAll(audio)
		if err != nil {
			t.Fatalf("ReadAll failed: %v", err)
		}

		if len(data) < 1000 {
			t.Errorf("Audio data too small: %d bytes", len(data))
		}

		t.Logf("Generated %d bytes of audio", len(data))

		if os.Getenv("SAVE_TEST_AUDIO") == "1" {
			os.WriteFile("/tmp/test_espeak_en.wav", data, 0644)
			t.Log("Saved to /tmp/test_espeak_en.wav")
		}
	})

	// Test Chinese (Mandarin)
	t.Run("Chinese", func(t *testing.T) {
		req := &EspeakNGRequest{
			Text:     "你好，这是一个测试。",
			Language: "cmn",
			Rate:     175,
			Pitch:    50,
			Volume:   100,
		}

		audio, err := provider.Synthesize(context.Background(), req)
		if err != nil {
			t.Fatalf("Synthesize failed: %v", err)
		}
		defer audio.Close()

		data, err := io.ReadAll(audio)
		if err != nil {
			t.Fatalf("ReadAll failed: %v", err)
		}

		if len(data) < 1000 {
			t.Errorf("Audio data too small: %d bytes", len(data))
		}

		t.Logf("Generated %d bytes of audio", len(data))

		if os.Getenv("SAVE_TEST_AUDIO") == "1" {
			os.WriteFile("/tmp/test_espeak_zh.wav", data, 0644)
			t.Log("Saved to /tmp/test_espeak_zh.wav")
		}
	})
}
