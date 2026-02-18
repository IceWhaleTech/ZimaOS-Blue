//go:build darwin

package tts

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"unsafe"
)

/*
#cgo LDFLAGS: -framework AVFoundation -framework Foundation
#include <stdlib.h>

// Wrapper for AVSpeechSynthesizer
typedef struct {
    void* synthesizer;
    void* audioEngine;
} macos_tts_t;

// Initialize TTS
macos_tts_t* macos_tts_init(void);

// Synthesize text to audio
unsigned char* macos_tts_synthesize(macos_tts_t* tts, const char* text, const char* voice, float rate, float pitch, float volume, int* output_len);

// Get available voices
const char** macos_tts_get_voices(int* count);

// Cleanup
void macos_tts_cleanup(macos_tts_t* tts);
void macos_tts_free_audio(unsigned char* audio);
void macos_tts_free_voices(const char** voices, int count);
*/
import "C"

// MacOSNativeTTS implements TTS using macOS native AVSpeechSynthesizer
type MacOSNativeTTS struct {
	initialized bool
	mu          sync.Mutex
	tts         *C.macos_tts_t
	voices      []string
}

// NewMacOSNativeTTS creates a new macOS native TTS provider
func NewMacOSNativeTTS() *MacOSNativeTTS {
	return &MacOSNativeTTS{}
}

// Initialize initializes the macOS TTS provider
func (p *MacOSNativeTTS) Initialize() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.initialized {
		return nil
	}

	tts := C.macos_tts_init()
	if tts == nil {
		return fmt.Errorf("failed to initialize macOS TTS")
	}

	p.tts = tts
	p.initialized = true

	// Load available voices
	var count C.int
	voicesPtr := C.macos_tts_get_voices(&count)
	if voicesPtr != nil {
		defer C.macos_tts_free_voices(voicesPtr, count)
		for i := 0; i < int(count); i++ {
			voicePtr := *(**C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(voicesPtr)) + uintptr(i)*unsafe.Sizeof(uintptr(0))))
			p.voices = append(p.voices, C.GoString(voicePtr))
		}
	}

	return nil
}

// Synthesize generates speech from text
func (p *MacOSNativeTTS) Synthesize(ctx context.Context, req *SynthesizeRequest) (*SynthesizeResponse, error) {
	if err := p.Initialize(); err != nil {
		return nil, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if req.Text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	voice := req.Voice
	if voice == "" {
		voice = "com.apple.speech.synthesis.voice.Alex"
	}

	cText := C.CString(req.Text)
	defer C.free(unsafe.Pointer(cText))

	cVoice := C.CString(voice)
	defer C.free(unsafe.Pointer(cVoice))

	var outputLen C.int
	audioPtr := C.macos_tts_synthesize(p.tts, cText, cVoice, C.float(req.Speed), C.float(req.Pitch), C.float(req.Volume), &outputLen)
	if audioPtr == nil {
		return nil, fmt.Errorf("synthesis failed")
	}
	defer C.macos_tts_free_audio(audioPtr)

	audioData := C.GoBytes(unsafe.Pointer(audioPtr), outputLen)
	return &SynthesizeResponse{
		Audio:       io.NopCloser(bytes.NewReader(audioData)),
		Format:      FormatWAV,
		ContentType: "audio/wav",
	}, nil
}

// SynthesizeStream synthesizes text with streaming output
func (p *MacOSNativeTTS) SynthesizeStream(ctx context.Context, req *SynthesizeRequest, callback StreamCallback) error {
	resp, err := p.Synthesize(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Audio.Close()

	buf := make([]byte, 4096)
	for {
		n, err := resp.Audio.Read(buf)
		if n > 0 {
			if err := callback(buf[:n]); err != nil {
				return err
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// ListVoices returns available system voices
func (p *MacOSNativeTTS) ListVoices(ctx context.Context) ([]Voice, error) {
	if err := p.Initialize(); err != nil {
		return nil, err
	}

	voices := []Voice{
		{ID: "com.apple.speech.synthesis.voice.Alex", Name: "Alex", Language: "en-US", Gender: "male", Provider: "macOS Native", Quality: "high"},
		{ID: "com.apple.speech.synthesis.voice.Victoria", Name: "Victoria", Language: "en-US", Gender: "female", Provider: "macOS Native", Quality: "high"},
		{ID: "com.apple.speech.synthesis.voice.Samantha", Name: "Samantha", Language: "en-US", Gender: "female", Provider: "macOS Native", Quality: "high"},
	}
	return voices, nil
}

// SupportedFormats returns supported audio formats
func (p *MacOSNativeTTS) SupportedFormats() []AudioFormat {
	return []AudioFormat{FormatWAV}
}

// MaxTextLength returns maximum text length
func (p *MacOSNativeTTS) MaxTextLength() int {
	return 32000
}

// Name returns provider name
func (p *MacOSNativeTTS) Name() string {
	return "macOS Native"
}

// Type returns provider type
func (p *MacOSNativeTTS) Type() ProviderType {
	return "macos-native"
}

// Close cleans up resources
func (p *MacOSNativeTTS) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.tts != nil {
		C.macos_tts_cleanup(p.tts)
		p.tts = nil
	}
	p.initialized = false
}

// Available returns true if macOS native TTS is available (always true on darwin)
func (p *MacOSNativeTTS) Available() bool {
	return true
}
