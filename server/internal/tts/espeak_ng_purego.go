package tts

import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// eSpeak-NG library functions (via purego)
var (
	espeakLibHandle uintptr
	espeakLibMu     sync.Mutex
	espeakLibLoaded bool

	// C API functions
	espeak_Initialize   func(output int, buflength int, path uintptr, options int) int
	espeak_Synth        func(text uintptr, size int, position int, position_type int, end_position int, flags int, unique_identifier uintptr, user_data uintptr) int
	espeak_Cancel       func() int
	espeak_Terminate    func() int
	espeak_SetParameter func(parameter int, value int, relative int) int
)

// eSpeak-NG parameter constants
const (
	espeakRATE   = 0
	espeakPITCH  = 1
	espeakRANGE  = 2
	espeakVOLUME = 3
)

// LoadEspeakLibrary loads the eSpeak-NG library dynamically (cross-platform)
func LoadEspeakLibrary(libPath string) error {
	espeakLibMu.Lock()
	defer espeakLibMu.Unlock()

	if espeakLibLoaded {
		return nil
	}

	// Load library based on platform
	var handle uintptr
	var err error

	switch runtime.GOOS {
	case "windows":
		handle, err = loadLibraryWindows(libPath)
	case "darwin":
		handle, err = loadLibraryUnix(libPath)
	case "linux":
		handle, err = loadLibraryUnix(libPath)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	if err != nil {
		return fmt.Errorf("failed to load eSpeak-NG library at %s: %w", libPath, err)
	}

	espeakLibHandle = handle

	// Register functions
	purego.RegisterLibFunc(&espeak_Initialize, espeakLibHandle, "espeak_Initialize")
	purego.RegisterLibFunc(&espeak_Synth, espeakLibHandle, "espeak_Synth")
	purego.RegisterLibFunc(&espeak_Cancel, espeakLibHandle, "espeak_Cancel")
	purego.RegisterLibFunc(&espeak_Terminate, espeakLibHandle, "espeak_Terminate")
	purego.RegisterLibFunc(&espeak_SetParameter, espeakLibHandle, "espeak_SetParameter")

	espeakLibLoaded = true
	return nil
}

// GetEspeakLibraryPath returns the platform-specific library path
func GetEspeakLibraryPath(baseDir string) string {
	switch runtime.GOOS {
	case "windows":
		return fmt.Sprintf("%s/espeak-ng.dll", baseDir)
	case "darwin":
		return fmt.Sprintf("%s/libespeak-ng.dylib", baseDir)
	case "linux":
		return fmt.Sprintf("%s/libespeak-ng.so", baseDir)
	default:
		return fmt.Sprintf("%s/libespeak-ng.so", baseDir)
	}
}

// EspeakNGSynthesizer wraps eSpeak-NG synthesis
type EspeakNGSynthesizer struct {
	initialized bool
	mu          sync.Mutex
}

// NewEspeakNGSynthesizer creates a new synthesizer
func NewEspeakNGSynthesizer(dataPath string) (*EspeakNGSynthesizer, error) {
	synth := &EspeakNGSynthesizer{}

	// Initialize eSpeak-NG
	// output: 0 = playback, 1 = synchronous
	// buflength: buffer length in ms
	// path: data directory path
	// options: 0 = default
	dataPathCStr := cStrEspeak(dataPath)
	result := espeak_Initialize(1, 200, dataPathCStr, 0)
	if result < 0 {
		return nil, fmt.Errorf("espeak_Initialize failed with code %d", result)
	}

	synth.initialized = true
	return synth, nil
}

// SetRate sets speech rate (80-500 WPM)
func (s *EspeakNGSynthesizer) SetRate(rate int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.initialized {
		return fmt.Errorf("synthesizer not initialized")
	}

	// Convert WPM to eSpeak rate (default 150)
	espeakRate := (rate * 150) / 150
	result := espeak_SetParameter(espeakRATE, espeakRate, 0)
	if result < 0 {
		return fmt.Errorf("SetParameter(RATE) failed")
	}
	return nil
}

// SetPitch sets pitch (0-99)
func (s *EspeakNGSynthesizer) SetPitch(pitch int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.initialized {
		return fmt.Errorf("synthesizer not initialized")
	}

	// Convert 0-99 to eSpeak pitch (0-100)
	espeakPitch := pitch
	result := espeak_SetParameter(espeakPITCH, espeakPitch, 0)
	if result < 0 {
		return fmt.Errorf("SetParameter(PITCH) failed")
	}
	return nil
}

// SetVolume sets volume (0-100)
func (s *EspeakNGSynthesizer) SetVolume(volume int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.initialized {
		return fmt.Errorf("synthesizer not initialized")
	}

	// Convert 0-100 to eSpeak volume (0-200)
	espeakVolume := (volume * 200) / 100
	result := espeak_SetParameter(espeakVOLUME, espeakVolume, 0)
	if result < 0 {
		return fmt.Errorf("SetParameter(VOLUME) failed")
	}
	return nil
}

// Synthesize generates speech from text
func (s *EspeakNGSynthesizer) Synthesize(text string, language string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.initialized {
		return nil, fmt.Errorf("synthesizer not initialized")
	}

	// TODO: Implement actual synthesis
	// This requires setting up callbacks for audio data
	// For now, return placeholder
	return nil, fmt.Errorf("synthesis not yet implemented")
}

// Close terminates the synthesizer
func (s *EspeakNGSynthesizer) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.initialized {
		return nil
	}

	result := espeak_Terminate()
	if result < 0 {
		return fmt.Errorf("espeak_Terminate failed")
	}

	s.initialized = false
	return nil
}

// cStrEspeak converts Go string to C string for eSpeak
func cStrEspeak(s string) uintptr {
	if s == "" {
		return 0
	}
	b := append([]byte(s), 0)
	return uintptr(unsafe.Pointer(&b[0]))
}
