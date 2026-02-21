// +build windows

package windows

/*
#cgo CXXFLAGS: -std=c++17 -I${SRCDIR}
#cgo LDFLAGS: -lole32 -loleaut32 -luuid

#include <stdlib.h>
#include "speech_asr_windows.h"
*/
import "C"
import (
	"context"
	"fmt"
	"io"
	"unsafe"
)

// WindowsASRProvider implements ASR using Windows.Media.SpeechRecognition
type WindowsASRProvider struct {
	handle unsafe.Pointer
}

// NewWindowsASRProvider creates a new Windows ASR provider
func NewWindowsASRProvider(language string) *WindowsASRProvider {
	cLang := C.CString(language)
	defer C.free(unsafe.Pointer(cLang))

	handle := C.asr_create(cLang)
	if handle == nil {
		return nil
	}
	return &WindowsASRProvider{handle: handle}
}

// Recognize performs speech recognition on audio data
func (p *WindowsASRProvider) Recognize(ctx context.Context, req *RecognizeRequest) (*RecognizeResponse, error) {
	if p.handle == nil {
		return nil, fmt.Errorf("ASR provider not initialized")
	}

	// Read audio data
	audioData, err := io.ReadAll(req.Audio)
	if err != nil {
		return nil, fmt.Errorf("failed to read audio: %w", err)
	}

	var resultText *C.char
	var confidence C.float

	result := C.asr_recognize(
		p.handle,
		(*C.char)(unsafe.Pointer(&audioData[0])),
		C.int(len(audioData)),
		&resultText,
		&confidence,
	)

	if result != 0 {
		return nil, fmt.Errorf("recognition failed with code %d", result)
	}

	if resultText == nil {
		return nil, fmt.Errorf("no recognition result")
	}

	text := C.GoString(resultText)
	C.asr_free_result(resultText)

	return &RecognizeResponse{
		Text:       text,
		Confidence: float32(confidence),
		Language:   req.Language,
	}, nil
}

// Close releases resources
func (p *WindowsASRProvider) Close() {
	if p.handle != nil {
		C.asr_destroy(p.handle)
		p.handle = nil
	}
}

// GetInstalledLanguages returns a list of installed speech recognition languages
func GetInstalledLanguages() []string {
	var count C.int
	cLanguages := C.asr_get_installed_languages(&count)
	if cLanguages == nil || count == 0 {
		return []string{}
	}
	defer C.asr_free_languages(cLanguages, count)

	// Convert C array to Go slice
	languages := make([]string, int(count))
	cArray := (*[1 << 30]*C.char)(unsafe.Pointer(cLanguages))[:count:count]
	for i := 0; i < int(count); i++ {
		languages[i] = C.GoString(cArray[i])
	}
	return languages
}
