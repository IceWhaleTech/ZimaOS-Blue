//go:build espeak

package tts

/*
#cgo CFLAGS: -I${SRCDIR}/../../../third_party/espeak-ng/src/include
#cgo windows CFLAGS: -DLIBESPEAK_NG_EXPORT
#cgo darwin LDFLAGS: -L${SRCDIR}/../../../third_party/espeak-ng/build/src/libespeak-ng -lespeak-ng -L${SRCDIR}/../../../third_party/espeak-ng/build/src/ucd-tools -lucd -L${SRCDIR}/../../../third_party/espeak-ng/build/src/speechPlayer -lspeechPlayer -L${SRCDIR}/../../../third_party/espeak-ng/build -lsonic
#cgo linux LDFLAGS: -L${SRCDIR}/../../../third_party/espeak-ng/build/src/libespeak-ng -lespeak-ng -L${SRCDIR}/../../../third_party/espeak-ng/build/src/ucd-tools -lucd -L${SRCDIR}/../../../third_party/espeak-ng/build/src/speechPlayer -lspeechPlayer -L${SRCDIR}/../../../third_party/espeak-ng/build -lsonic -lpthread
#cgo windows LDFLAGS: ${SRCDIR}/../../../third_party/espeak-ng/build/src/libespeak-ng/libespeak-ng.a ${SRCDIR}/../../../third_party/espeak-ng/build/src/ucd-tools/libucd.a ${SRCDIR}/../../../third_party/espeak-ng/build/src/speechPlayer/libspeechPlayer.a

#include <stdlib.h>
#include <string.h>
#include <espeak-ng/speak_lib.h>

// Global buffer for audio data
static short* g_audio_buffer = NULL;
static int g_audio_size = 0;
static int g_audio_capacity = 0;
static int g_sample_rate = 0;

// Callback function for synthesis
static int synth_callback(short *wav, int numsamples, espeak_EVENT *events) {
    if (wav == NULL) {
        return 0;
    }

    // Expand buffer if needed
    int new_size = g_audio_size + numsamples;
    if (new_size > g_audio_capacity) {
        int new_capacity = g_audio_capacity == 0 ? 65536 : g_audio_capacity * 2;
        while (new_capacity < new_size) {
            new_capacity *= 2;
        }
        short* new_buffer = (short*)realloc(g_audio_buffer, new_capacity * sizeof(short));
        if (new_buffer == NULL) {
            return 1; // abort
        }
        g_audio_buffer = new_buffer;
        g_audio_capacity = new_capacity;
    }

    // Copy samples
    memcpy(g_audio_buffer + g_audio_size, wav, numsamples * sizeof(short));
    g_audio_size += numsamples;

    return 0;
}

// Initialize eSpeak-NG
int espeak_cgo_init(const char* data_path) {
    g_sample_rate = espeak_Initialize(AUDIO_OUTPUT_SYNCHRONOUS, 0, data_path, 0);
    if (g_sample_rate <= 0) {
        return -1;
    }
    espeak_SetSynthCallback(synth_callback);
    return g_sample_rate;
}

// Synthesize text to audio
int espeak_cgo_synth(const char* text, const char* voice, int rate, int pitch, int volume) {
    // Reset buffer
    g_audio_size = 0;

    // Set voice
    if (espeak_SetVoiceByName(voice) != EE_OK) {
        return -1;
    }

    // Set parameters
    espeak_SetParameter(espeakRATE, rate, 0);
    espeak_SetParameter(espeakPITCH, pitch, 0);
    espeak_SetParameter(espeakVOLUME, volume, 0);

    // Synthesize
    unsigned int flags = espeakCHARS_UTF8 | espeakENDPAUSE;
    if (espeak_Synth(text, strlen(text) + 1, 0, POS_CHARACTER, 0, flags, NULL, NULL) != EE_OK) {
        return -2;
    }

    // Wait for completion
    espeak_Synchronize();

    return g_audio_size;
}

// Get audio buffer pointer
short* espeak_cgo_get_audio() {
    return g_audio_buffer;
}

// Get sample rate
int espeak_cgo_get_sample_rate() {
    return g_sample_rate;
}

// Cleanup
void espeak_cgo_cleanup() {
    if (g_audio_buffer != NULL) {
        free(g_audio_buffer);
        g_audio_buffer = NULL;
    }
    g_audio_size = 0;
    g_audio_capacity = 0;
    espeak_Terminate();
}
*/
import "C"

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"unsafe"
)

// EspeakNGRequest represents a synthesis request
type EspeakNGRequest struct {
	Text     string `json:"text"`
	Language string `json:"language"` // e.g., "en", "cmn", "ja"
	Voice    string `json:"voice"`    // e.g., "f3", "m1", "whisper" (optional variant)
	Rate     int    `json:"rate"`     // 80-450 (words per minute)
	Pitch    int    `json:"pitch"`    // 0-99
	Volume   int    `json:"volume"`   // 0-200
}

// EspeakNGProvider implements TTS using eSpeak-NG via CGO static linking
type EspeakNGProvider struct {
	dataPath    string
	initialized bool
	sampleRate  int
	mu          sync.Mutex
}

// NewEspeakNGProvider creates a new eSpeak-NG provider
func NewEspeakNGProvider(dataPath string) *EspeakNGProvider {
	// Auto-detect data path if not provided
	if dataPath == "" {
		dataPath = findEspeakDataPath()
	}
	return &EspeakNGProvider{
		dataPath: dataPath,
	}
}

// findEspeakDataPath searches for espeak-ng-data in common locations
func findEspeakDataPath() string {
	// Check environment variable first
	if envPath := os.Getenv("ESPEAK_DATA_PATH"); envPath != "" {
		return envPath
	}

	// Get executable directory
	execPath, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(execPath)
		// Check relative to executable
		candidates := []string{
			filepath.Join(execDir, "espeak-ng-data"),
			filepath.Join(execDir, "..", "third_party", "espeak-ng", "build"),
			filepath.Join(execDir, "..", "..", "third_party", "espeak-ng", "build"),
		}
		for _, path := range candidates {
			if _, err := os.Stat(filepath.Join(path, "espeak-ng-data", "phontab")); err == nil {
				return path
			}
			if _, err := os.Stat(filepath.Join(path, "phontab")); err == nil {
				return filepath.Dir(path)
			}
		}
	}

	// Check working directory
	if cwd, err := os.Getwd(); err == nil {
		candidates := []string{
			filepath.Join(cwd, "data", "espeak-ng-data"),
			filepath.Join(cwd, "data", "espeak-ng"),
			filepath.Join(cwd, "..", "third_party", "espeak-ng", "build"),
			filepath.Join(cwd, "third_party", "espeak-ng", "build"),
		}
		for _, path := range candidates {
			if _, err := os.Stat(filepath.Join(path, "phontab")); err == nil {
				return filepath.Dir(path)
			}
			if _, err := os.Stat(filepath.Join(path, "espeak-ng-data", "phontab")); err == nil {
				return path
			}
		}
	}

	// System paths
	systemPaths := []string{
		"/usr/share/espeak-ng-data",
		"/usr/local/share/espeak-ng-data",
		"/opt/homebrew/share/espeak-ng-data",
	}
	for _, path := range systemPaths {
		if _, err := os.Stat(filepath.Join(path, "phontab")); err == nil {
			return filepath.Dir(path)
		}
	}

	return ""
}

// Initialize initializes the eSpeak-NG library
func (p *EspeakNGProvider) Initialize() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.initialized {
		return nil
	}

	var cDataPath *C.char
	if p.dataPath != "" {
		cDataPath = C.CString(p.dataPath)
		defer C.free(unsafe.Pointer(cDataPath))
	}

	sampleRate := C.espeak_cgo_init(cDataPath)
	if sampleRate < 0 {
		return fmt.Errorf("failed to initialize eSpeak-NG")
	}

	p.sampleRate = int(sampleRate)
	p.initialized = true
	return nil
}

// Synthesize generates speech from text
func (p *EspeakNGProvider) Synthesize(ctx context.Context, req *EspeakNGRequest) (io.ReadCloser, error) {
	if err := p.Initialize(); err != nil {
		return nil, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Validate parameters
	if req.Text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	voice := req.Language
	if voice == "" {
		voice = "en"
	}

	// Add voice variant if specified (e.g., "en+f3", "cmn+m1")
	if req.Voice != "" {
		voice = voice + "+" + req.Voice
	}

	rate := req.Rate
	if rate < 80 {
		rate = 80
	}
	if rate > 450 {
		rate = 450
	}

	pitch := req.Pitch
	if pitch < 0 {
		pitch = 0
	}
	if pitch > 99 {
		pitch = 99
	}

	volume := req.Volume
	if volume < 0 {
		volume = 0
	}
	if volume > 200 {
		volume = 200
	}

	// Convert strings to C
	cText := C.CString(req.Text)
	defer C.free(unsafe.Pointer(cText))

	cVoice := C.CString(voice)
	defer C.free(unsafe.Pointer(cVoice))

	// Synthesize
	numSamples := C.espeak_cgo_synth(cText, cVoice, C.int(rate), C.int(pitch), C.int(volume))
	if numSamples < 0 {
		return nil, fmt.Errorf("synthesis failed with code: %d", numSamples)
	}

	// Get audio data
	audioPtr := C.espeak_cgo_get_audio()
	if audioPtr == nil || numSamples == 0 {
		return nil, fmt.Errorf("no audio data generated")
	}

	// Copy audio data to Go slice
	pcmData := make([]byte, int(numSamples)*2)
	samples := unsafe.Slice((*int16)(unsafe.Pointer(audioPtr)), int(numSamples))
	for i, sample := range samples {
		pcmData[i*2] = byte(sample)
		pcmData[i*2+1] = byte(sample >> 8)
	}

	// Convert to WAV
	wavData := pcmToWav(pcmData, p.sampleRate)
	return io.NopCloser(bytes.NewReader(wavData)), nil
}

// Name returns the provider name
func (p *EspeakNGProvider) Name() string {
	return "eSpeak-NG"
}

// Type returns the provider type
func (p *EspeakNGProvider) Type() string {
	return "espeak-ng"
}

// SupportedLanguages returns list of supported languages
func (p *EspeakNGProvider) SupportedLanguages() []string {
	return []string{
		"en", "cmn", "yue", "ja", "ko", "es", "fr", "de", "it", "pt",
		"ru", "pl", "nl", "sv", "no", "da", "fi", "cs", "sk", "hu",
		"ro", "el", "tr", "ar", "he", "fa", "vi", "th",
	}
}

// Close cleans up resources
func (p *EspeakNGProvider) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.initialized {
		C.espeak_cgo_cleanup()
		p.initialized = false
	}
}

// pcmToWav converts PCM data to WAV format
func pcmToWav(pcmData []byte, sampleRate int) []byte {
	dataSize := len(pcmData)
	buf := new(bytes.Buffer)

	// RIFF header
	buf.WriteString("RIFF")
	binary.Write(buf, binary.LittleEndian, uint32(36+dataSize))
	buf.WriteString("WAVE")

	// fmt chunk
	buf.WriteString("fmt ")
	binary.Write(buf, binary.LittleEndian, uint32(16))           // chunk size
	binary.Write(buf, binary.LittleEndian, uint16(1))            // audio format (PCM)
	binary.Write(buf, binary.LittleEndian, uint16(1))            // num channels (mono)
	binary.Write(buf, binary.LittleEndian, uint32(sampleRate))   // sample rate
	binary.Write(buf, binary.LittleEndian, uint32(sampleRate*2)) // byte rate
	binary.Write(buf, binary.LittleEndian, uint16(2))            // block align
	binary.Write(buf, binary.LittleEndian, uint16(16))           // bits per sample

	// data chunk
	buf.WriteString("data")
	binary.Write(buf, binary.LittleEndian, uint32(dataSize))
	buf.Write(pcmData)

	return buf.Bytes()
}
