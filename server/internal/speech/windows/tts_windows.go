// +build windows

package windows

/*
#cgo CXXFLAGS: -std=c++17 -I${SRCDIR}
#cgo LDFLAGS: -lole32 -loleaut32 -luuid

#include <stdlib.h>
#include "speech_tts_windows.h"
*/
import "C"
import (
	"context"
	"crypto/md5"
	"fmt"
	"time"
	"unsafe"
)

// WindowsTTSProvider implements TTS using Windows.Media.SpeechSynthesis
type WindowsTTSProvider struct {
	handle unsafe.Pointer
	cache  *TTSCache
}

// NewWindowsTTSProvider creates a new Windows TTS provider
func NewWindowsTTSProvider() *WindowsTTSProvider {
	handle := C.tts_create()
	if handle == nil {
		return nil
	}
	return &WindowsTTSProvider{
		handle: handle,
		cache:  NewTTSCache(100, 5*time.Minute), // Cache up to 100 items for 5 minutes
	}
}

// ListVoices returns available TTS voices
func (p *WindowsTTSProvider) ListVoices(ctx context.Context) ([]Voice, error) {
	if p.handle == nil {
		return nil, fmt.Errorf("TTS provider not initialized")
	}

	var count C.int
	voicesPtr := C.tts_list_voices(p.handle, &count)
	if voicesPtr == nil {
		return nil, fmt.Errorf("failed to list voices")
	}
	defer C.tts_free_voices(voicesPtr, count)

	voices := make([]Voice, int(count))
	voiceArray := (*[1 << 30]C.VoiceInfo)(unsafe.Pointer(voicesPtr))[:count:count]

	for i := 0; i < int(count); i++ {
		voices[i] = Voice{
			ID:       C.GoString(voiceArray[i].id),
			Name:     C.GoString(voiceArray[i].name),
			Language: C.GoString(voiceArray[i].language),
			Gender:   C.GoString(voiceArray[i].gender),
		}
	}

	return voices, nil
}

// Synthesize converts text to speech
func (p *WindowsTTSProvider) Synthesize(ctx context.Context, req *SynthesizeRequest) (*SynthesizeResponse, error) {
	if p.handle == nil {
		return nil, fmt.Errorf("TTS provider not initialized")
	}

	if req.Text == "" {
		return nil, fmt.Errorf("text is empty")
	}

	// Auto-select voice based on language if not specified
	voice := req.Voice
	if voice == "" {
		detectedVoice, err := p.detectAndSelectVoice(ctx, req.Text)
		if err == nil && detectedVoice != "" {
			voice = detectedVoice
		}
	}

	// Generate cache key
	speed := req.Speed
	if speed == 0 {
		speed = 1.0
	}
	pitch := req.Pitch
	if pitch == 0 {
		pitch = 1.0
	}
	volume := req.Volume
	if volume == 0 {
		volume = 1.0
	}

	cacheKey := fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s|%s|%.2f|%.2f|%.2f", req.Text, voice, speed, pitch, volume))))

	// Check cache
	if p.cache != nil {
		if cached, ok := p.cache.Get(cacheKey); ok {
			return &SynthesizeResponse{
				Audio:       cached.Audio,
				ContentType: "audio/wav",
				SampleRate:  cached.SampleRate,
			}, nil
		}
	}

	cText := C.CString(req.Text)
	defer C.free(unsafe.Pointer(cText))

	cVoice := C.CString(voice)
	defer C.free(unsafe.Pointer(cVoice))

	var audioData *C.char
	var audioSize C.int
	var sampleRate C.int

	result := C.tts_synthesize(
		p.handle,
		cText,
		cVoice,
		C.float(speed),
		C.float(pitch),
		C.float(volume),
		&audioData,
		&audioSize,
		&sampleRate,
	)

	if result != 0 {
		// Provide more detailed error messages
		var errMsg string
		switch result {
		case -1:
			errMsg = "invalid handle or text"
		case -2:
			errMsg = "failed to create COM stream (COM initialization issue)"
		case -3:
			errMsg = "failed to set wave format"
		case -4:
			errMsg = "failed to set output stream"
		case -5:
			errMsg = "speech synthesis failed (Speak() call failed)"
		case -6:
			errMsg = "no audio data generated (empty stream or seek failed)"
		case -7:
			errMsg = "failed to read audio data from stream"
		default:
			errMsg = fmt.Sprintf("unknown error code %d", result)
		}
		return nil, fmt.Errorf("synthesis failed: %s (code %d)", errMsg, result)
	}

	if audioData == nil || audioSize == 0 {
		return nil, fmt.Errorf("no audio data generated")
	}

	audio := C.GoBytes(unsafe.Pointer(audioData), audioSize)
	C.tts_free_audio(audioData)

	// Store in cache
	if p.cache != nil {
		p.cache.Set(cacheKey, audio, int(sampleRate))
	}

	return &SynthesizeResponse{
		Audio:       audio,
		ContentType: "audio/wav",
		SampleRate:  int(sampleRate),
	}, nil
}

// detectAndSelectVoice detects language and selects appropriate voice
func (p *WindowsTTSProvider) detectAndSelectVoice(ctx context.Context, text string) (string, error) {
	voices, err := p.ListVoices(ctx)
	if err != nil || len(voices) == 0 {
		return "", err
	}

	// Simple language detection based on character ranges
	lang := detectLanguageFromText(text)

	// Match voice by language
	for _, v := range voices {
		if matchesLanguage(v.Language, lang) {
			return v.ID, nil
		}
	}

	// Fallback to first voice
	return voices[0].ID, nil
}

// detectLanguageFromText performs simple language detection
func detectLanguageFromText(text string) string {
	if len(text) == 0 {
		return "en"
	}

	// Count character types
	hasChinese := false
	hasJapanese := false
	hasKorean := false

	for _, r := range text {
		if r >= 0x4E00 && r <= 0x9FFF {
			hasChinese = true
		} else if (r >= 0x3040 && r <= 0x309F) || (r >= 0x30A0 && r <= 0x30FF) {
			hasJapanese = true
		} else if r >= 0xAC00 && r <= 0xD7AF {
			hasKorean = true
		}
	}

	if hasChinese {
		return "zh"
	}
	if hasJapanese {
		return "ja"
	}
	if hasKorean {
		return "ko"
	}

	return "en"
}

// matchesLanguage checks if voice language matches detected language
func matchesLanguage(voiceLang, detectedLang string) bool {
	if len(voiceLang) < 2 || len(detectedLang) < 2 {
		return false
	}

	// Match language prefix (e.g., "zh-CN" matches "zh")
	return voiceLang[:2] == detectedLang[:2]
}

// Close releases resources
func (p *WindowsTTSProvider) Close() {
	if p.handle != nil {
		C.tts_destroy(p.handle)
		p.handle = nil
	}
}
