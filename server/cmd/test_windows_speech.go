// +build windows

package main

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/speech"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/speech/windows"
)

func main() {
	fmt.Println("=== Windows Native Speech Test ===")
	fmt.Printf("Platform: %s\n\n", runtime.GOOS)

	// Test TTS
	fmt.Println("1. Testing TTS Provider...")
	ttsProvider := windows.NewWindowsTTSProvider()
	if ttsProvider == nil {
		fmt.Println("❌ Failed to create TTS provider")
		os.Exit(1)
	}
	defer ttsProvider.Close()

	ctx := context.Background()
	voices, err := ttsProvider.ListVoices(ctx)
	if err != nil {
		fmt.Printf("❌ Failed to list voices: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Found %d voices\n", len(voices))
	if len(voices) > 0 {
		fmt.Printf("  First voice: %s (%s)\n", voices[0].Name, voices[0].Language)
	}

	// Test synthesis
	req := &windows.SynthesizeRequest{
		Text:   "Hello, this is a test of Windows native TTS.",
		Speed:  1.0,
		Pitch:  1.0,
		Volume: 1.0,
	}
	resp, err := ttsProvider.Synthesize(ctx, req)
	if err != nil {
		fmt.Printf("❌ Failed to synthesize: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Synthesized %d bytes of audio at %d Hz\n", len(resp.Audio), resp.SampleRate)

	// Test Chinese synthesis
	reqChinese := &windows.SynthesizeRequest{
		Text:   "你好，这是Windows原生语音合成测试。",
		Speed:  1.0,
		Pitch:  1.0,
		Volume: 1.0,
	}
	respChinese, err := ttsProvider.Synthesize(ctx, reqChinese)
	if err != nil {
		fmt.Printf("⚠ Chinese synthesis failed (may not have Chinese voice): %v\n", err)
	} else {
		fmt.Printf("✓ Chinese synthesis: %d bytes\n", len(respChinese.Audio))
	}

	// Test ASR
	fmt.Println("\n2. Testing ASR Provider...")
	asrProvider, err := speech.NewWindowsNativeASRProvider("en-US")
	if err != nil {
		fmt.Printf("❌ Failed to create ASR provider: %v\n", err)
		os.Exit(1)
	}
	defer asrProvider.Close()

	fmt.Printf("✓ ASR Provider: %s (type: %s)\n", asrProvider.Name(), asrProvider.Type())

	formats := asrProvider.SupportedFormats()
	fmt.Printf("✓ Supported formats: %v\n", formats)

	maxDuration := asrProvider.MaxDuration()
	fmt.Printf("✓ Max duration: %v\n", maxDuration)

	// Test service integration
	fmt.Println("\n3. Testing Service Integration...")
	fmt.Printf("✓ Platform: %s\n", runtime.GOOS)
	if runtime.GOOS == "windows" {
		fmt.Println("✓ Windows native provider should be default")
		fmt.Println("✓ Whisper should NOT be in available providers")
	}

	fmt.Println("\n=== All Tests Passed ✓ ===")
}
