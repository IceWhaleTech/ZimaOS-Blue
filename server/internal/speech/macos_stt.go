//go:build darwin

package speech

import (
	"context"
	"fmt"
	"sync"
	"unsafe"
)

/*
#cgo LDFLAGS: -framework Speech -framework Foundation
#include <stdlib.h>

// Wrapper for SFSpeechRecognizer
typedef struct {
    void* recognizer;
    void* audioEngine;
} macos_stt_t;

// Initialize STT
macos_stt_t* macos_stt_init(void);

// Recognize audio from file
char* macos_stt_recognize(macos_stt_t* stt, const char* audioPath, int* error_code);

// Cleanup
void macos_stt_cleanup(macos_stt_t* stt);
void macos_stt_free_string(char* str);
*/
import "C"

// MacOSNativeSTT implements speech recognition using macOS native Speech Framework
type MacOSNativeSTT struct {
	initialized bool
	mu          sync.Mutex
	stt         *C.macos_stt_t
}

// NewMacOSNativeSTT creates a new macOS native STT provider
func NewMacOSNativeSTT() *MacOSNativeSTT {
	return &MacOSNativeSTT{}
}

// Initialize initializes the macOS STT provider
func (p *MacOSNativeSTT) Initialize() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.initialized {
		return nil
	}

	stt := C.macos_stt_init()
	if stt == nil {
		return fmt.Errorf("failed to initialize macOS STT")
	}

	p.stt = stt
	p.initialized = true
	return nil
}

// Recognize recognizes speech from audio file
func (p *MacOSNativeSTT) Recognize(ctx context.Context, audioPath string) (string, error) {
	if err := p.Initialize(); err != nil {
		return "", err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if audioPath == "" {
		return "", fmt.Errorf("audio path cannot be empty")
	}

	cPath := C.CString(audioPath)
	defer C.free(unsafe.Pointer(cPath))

	var errorCode C.int
	resultPtr := C.macos_stt_recognize(p.stt, cPath, &errorCode)
	if resultPtr == nil {
		return "", fmt.Errorf("speech recognition failed with error code %d", errorCode)
	}
	defer C.macos_stt_free_string(resultPtr)

	result := C.GoString(resultPtr)
	return result, nil
}

// Name returns provider name
func (p *MacOSNativeSTT) Name() string {
	return "macOS Native"
}

// Available returns true if macOS native STT is available
func (p *MacOSNativeSTT) Available() bool {
	return true
}

// Close cleans up resources
func (p *MacOSNativeSTT) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stt != nil {
		C.macos_stt_cleanup(p.stt)
		p.stt = nil
	}
	p.initialized = false
}
