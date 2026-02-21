//go:build espeak && !windows

package tts

/*
#cgo CFLAGS: -I${SRCDIR}/../../../third_party/espeak-ng/src/include
#cgo windows CFLAGS: -DLIBESPEAK_NG_EXPORT
#cgo darwin LDFLAGS: -L${SRCDIR}/../../../third_party/espeak-ng/build/src/libespeak-ng -lespeak-ng -L${SRCDIR}/../../../third_party/espeak-ng/build/src/ucd-tools -lucd -L${SRCDIR}/../../../third_party/espeak-ng/build/src/speechPlayer -lspeechPlayer -L${SRCDIR}/../../../third_party/espeak-ng/build -lsonic -lc++
#cgo linux LDFLAGS: -L${SRCDIR}/../../../third_party/espeak-ng/build/src/libespeak-ng -lespeak-ng -L${SRCDIR}/../../../third_party/espeak-ng/build/src/ucd-tools -lucd -L${SRCDIR}/../../../third_party/espeak-ng/build/src/speechPlayer -lspeechPlayer -L${SRCDIR}/../../../third_party/espeak-ng/build -lsonic -lstdc++ -lpthread
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

// Synthesize text to audio with optimized parameters
int espeak_cgo_synth(const char* text, const char* voice, int rate, int pitch, int pitch_range, int volume) {
    // Reset buffer
    g_audio_size = 0;

    // Set voice
    if (espeak_SetVoiceByName(voice) != EE_OK) {
        return -1;
    }

    // Core parameters
    espeak_SetParameter(espeakRATE, rate, 0);
    espeak_SetParameter(espeakPITCH, pitch, 0);
    espeak_SetParameter(espeakRANGE, pitch_range, 0);
    espeak_SetParameter(espeakVOLUME, volume, 0);
    espeak_SetParameter(espeakWORDGAP, 5, 0);

    // Synthesize
    unsigned int flags = espeakCHARS_UTF8 | espeakENDPAUSE | espeakSSML;
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
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unsafe"
)

// EspeakNGRequest represents a synthesis request
type EspeakNGRequest struct {
	Text     string  `json:"text"`
	Language string  `json:"language"` // e.g., "en", "cmn", "ja"
	Rate     float32 `json:"rate"`     // multiplier (1.0 = default 130 wpm)
	Pitch    float32 `json:"pitch"`    // adjustment (-50 to 50, default 0)
	Volume   float32 `json:"volume"`   // multiplier (1.0 = default 110)
}

// EspeakNGProvider implements TTS using eSpeak-NG via CGO static linking
type EspeakNGProvider struct {
	dataPath      string
	vocoderPath   string
	initialized   bool
	sampleRate    int
	mu            sync.Mutex
	vocoder       *VocoderInstance
	vocoderOnce   sync.Once
	preprocessor  *AudioPreprocessor
}

// NewEspeakNGProvider creates a new eSpeak-NG provider
func NewEspeakNGProvider(dataPath string) *EspeakNGProvider {
	// Auto-detect data path if not provided
	if dataPath == "" {
		dataPath = findEspeakDataPath()
	}
	return &EspeakNGProvider{
		dataPath:    dataPath,
		vocoderPath: findVocoderModel(dataPath),
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

	// System paths (no working directory dependency)
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
	log.Println("[tts/espeak] Initialize: acquiring lock...")
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.initialized {
		log.Println("[tts/espeak] Initialize: already initialized")
		return nil
	}

	log.Printf("[tts/espeak] Initialize: dataPath=%q", p.dataPath)

	var cDataPath *C.char
	if p.dataPath != "" {
		cDataPath = C.CString(p.dataPath)
		defer C.free(unsafe.Pointer(cDataPath))
	}

	log.Println("[tts/espeak] Initialize: calling espeak_cgo_init...")
	sampleRate := C.espeak_cgo_init(cDataPath)
	log.Printf("[tts/espeak] Initialize: espeak_cgo_init returned %d", sampleRate)
	if sampleRate < 0 {
		return fmt.Errorf("failed to initialize eSpeak-NG")
	}

	p.sampleRate = int(sampleRate)
	p.initialized = true
	p.preprocessor = NewAudioPreprocessor(p.sampleRate)
	log.Printf("[tts/espeak] Initialize: done, sampleRate=%d", p.sampleRate)
	return nil
}

// Synthesize generates speech from text
func (p *EspeakNGProvider) Synthesize(ctx context.Context, req *EspeakNGRequest) (io.ReadCloser, error) {
	start := time.Now()
	log.Printf("[tts/espeak] Synthesize: start, text_len=%d lang=%q", len(req.Text), req.Language)

	if err := p.Initialize(); err != nil {
		return nil, err
	}

	log.Println("[tts/espeak] Synthesize: acquiring mu lock...")
	p.mu.Lock()
	defer p.mu.Unlock()
	log.Printf("[tts/espeak] Synthesize: lock acquired (%.1fms)", float64(time.Since(start).Microseconds())/1000)

	// Validate parameters
	if req.Text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	voice := req.Language
	if voice == "" {
		voice = "en"
	}

	// Always use "zima" voice variant for optimized formant settings
	voice = voice + "+zima"

	// Apply multipliers to optimized base values
	// Base rate: 160 wpm (slightly brisk, natural female pace)
	rate := req.Rate
	if rate <= 0 {
		rate = 1.0  // default multiplier
	}
	finalRate := int(160 * rate)
	if finalRate < 80 {
		finalRate = 80
	}
	if finalRate > 450 {
		finalRate = 450
	}

	pitch := req.Pitch
	// Pitch base: 70 (higher for female voice, espeak range 0-99)
	// Pitch is adjustment (-50 to 50), default 0
	finalPitch := int(70 + pitch)
	if finalPitch < 0 {
		finalPitch = 0
	}
	if finalPitch > 99 {
		finalPitch = 99
	}

	// Pitch range: 50 (wider variation for natural female intonation)
	finalPitchRange := 50

	volume := req.Volume
	if volume <= 0 {
		volume = 1.0  // default multiplier
	}
	finalVolume := int(110 * volume)
	if finalVolume < 0 {
		finalVolume = 0
	}
	if finalVolume > 200 {
		finalVolume = 200
	}

	log.Printf("[tts/espeak] Synthesize: voice=%q rate=%d pitch=%d range=%d vol=%d", voice, finalRate, finalPitch, finalPitchRange, finalVolume)

	// Convert strings to C
	cText := C.CString(req.Text)
	defer C.free(unsafe.Pointer(cText))

	cVoice := C.CString(voice)
	defer C.free(unsafe.Pointer(cVoice))

	// Synthesize
	log.Println("[tts/espeak] Synthesize: calling espeak_cgo_synth...")
	synthStart := time.Now()
	numSamples := C.espeak_cgo_synth(cText, cVoice, C.int(finalRate), C.int(finalPitch), C.int(finalPitchRange), C.int(finalVolume))
	log.Printf("[tts/espeak] Synthesize: espeak_cgo_synth returned %d samples (%.1fms)", numSamples, float64(time.Since(synthStart).Microseconds())/1000)
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
	log.Printf("[tts/espeak] Synthesize: copied %d bytes PCM data", len(pcmData))

	// Apply vocoder if model exists (lazy load)
	if p.vocoderPath != "" {
		log.Printf("[tts/espeak] Synthesize: vocoder path=%q", p.vocoderPath)
		p.vocoderOnce.Do(func() {
			log.Println("[tts/espeak] Synthesize: loading vocoder model...")
			loadStart := time.Now()
			p.vocoder, _ = InitVocoder(p.vocoderPath, p.sampleRate)
			log.Printf("[tts/espeak] Synthesize: vocoder loaded (%.1fms), ok=%v", float64(time.Since(loadStart).Microseconds())/1000, p.vocoder != nil)
		})
		if p.vocoder != nil {
			// Convert bytes to int16 samples
			int16Samples := make([]int16, len(pcmData)/2)
			for i := 0; i < len(int16Samples); i++ {
				int16Samples[i] = int16(pcmData[i*2]) | (int16(pcmData[i*2+1]) << 8)
			}

			log.Printf("[tts/espeak] Synthesize: applying lowpass to %d samples...", len(int16Samples))
			lpStart := time.Now()
			// Apply lowpass filter before mel extraction (skip EQ — HiFi-GAN regenerates audio)
			int16Samples = p.preprocessor.ApplyLowpass(int16Samples)
			log.Printf("[tts/espeak] Synthesize: lowpass done (%.1fms)", float64(time.Since(lpStart).Microseconds())/1000)

			// Process with vocoder (eSpeak 22050 Hz = HiFi-GAN V3 native rate)
			log.Printf("[tts/espeak] Synthesize: running vocoder on %d samples...", len(int16Samples))
			vocStart := time.Now()
			if processed, err := p.vocoder.Process(int16Samples); err == nil {
				log.Printf("[tts/espeak] Synthesize: vocoder done, %d→%d samples (%.1fms)", len(int16Samples), len(processed), float64(time.Since(vocStart).Microseconds())/1000)
				// Convert back to bytes
				pcmData = make([]byte, len(processed)*2)
				for i, sample := range processed {
					pcmData[i*2] = byte(sample)
					pcmData[i*2+1] = byte(sample >> 8)
				}
			} else {
				log.Printf("[tts/espeak] Synthesize: vocoder error: %v (%.1fms)", err, float64(time.Since(vocStart).Microseconds())/1000)
			}
		} else {
			log.Println("[tts/espeak] Synthesize: vocoder not available, passthrough")
		}
	} else {
		log.Println("[tts/espeak] Synthesize: no vocoder path, passthrough")
	}

	// Convert to WAV (always 22050 Hz — eSpeak native = HiFi-GAN V3 native)
	log.Printf("[tts/espeak] Synthesize: encoding WAV, %d bytes PCM, sr=%d", len(pcmData), p.sampleRate)
	wavData := pcmToWav(pcmData, p.sampleRate)
	log.Printf("[tts/espeak] Synthesize: done, total %.1fms, wav=%d bytes", float64(time.Since(start).Microseconds())/1000, len(wavData))
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

// ListVoices returns available voices (marked as robotic)
func (p *EspeakNGProvider) ListVoices(ctx context.Context) ([]Voice, error) {
	voices := []Voice{
		{ID: "en", Name: "English", Language: "en", Gender: "neutral", Provider: "eSpeak-NG (Robotic)", Quality: "low"},
		{ID: "cmn", Name: "Mandarin Chinese", Language: "cmn", Gender: "neutral", Provider: "eSpeak-NG (Robotic)", Quality: "low"},
		{ID: "yue", Name: "Cantonese", Language: "yue", Gender: "neutral", Provider: "eSpeak-NG (Robotic)", Quality: "low"},
		{ID: "ja", Name: "Japanese", Language: "ja", Gender: "neutral", Provider: "eSpeak-NG (Robotic)", Quality: "low"},
		{ID: "ko", Name: "Korean", Language: "ko", Gender: "neutral", Provider: "eSpeak-NG (Robotic)", Quality: "low"},
		{ID: "es", Name: "Spanish", Language: "es", Gender: "neutral", Provider: "eSpeak-NG (Robotic)", Quality: "low"},
		{ID: "fr", Name: "French", Language: "fr", Gender: "neutral", Provider: "eSpeak-NG (Robotic)", Quality: "low"},
		{ID: "de", Name: "German", Language: "de", Gender: "neutral", Provider: "eSpeak-NG (Robotic)", Quality: "low"},
		{ID: "it", Name: "Italian", Language: "it", Gender: "neutral", Provider: "eSpeak-NG (Robotic)", Quality: "low"},
		{ID: "pt", Name: "Portuguese", Language: "pt", Gender: "neutral", Provider: "eSpeak-NG (Robotic)", Quality: "low"},
		{ID: "ru", Name: "Russian", Language: "ru", Gender: "neutral", Provider: "eSpeak-NG (Robotic)", Quality: "low"},
		{ID: "pl", Name: "Polish", Language: "pl", Gender: "neutral", Provider: "eSpeak-NG (Robotic)", Quality: "low"},
		{ID: "nl", Name: "Dutch", Language: "nl", Gender: "neutral", Provider: "eSpeak-NG (Robotic)", Quality: "low"},
		{ID: "sv", Name: "Swedish", Language: "sv", Gender: "neutral", Provider: "eSpeak-NG (Robotic)", Quality: "low"},
		{ID: "no", Name: "Norwegian", Language: "no", Gender: "neutral", Provider: "eSpeak-NG (Robotic)", Quality: "low"},
		{ID: "da", Name: "Danish", Language: "da", Gender: "neutral", Provider: "eSpeak-NG (Robotic)", Quality: "low"},
		{ID: "fi", Name: "Finnish", Language: "fi", Gender: "neutral", Provider: "eSpeak-NG (Robotic)", Quality: "low"},
		{ID: "cs", Name: "Czech", Language: "cs", Gender: "neutral", Provider: "eSpeak-NG (Robotic)", Quality: "low"},
		{ID: "sk", Name: "Slovak", Language: "sk", Gender: "neutral", Provider: "eSpeak-NG (Robotic)", Quality: "low"},
		{ID: "hu", Name: "Hungarian", Language: "hu", Gender: "neutral", Provider: "eSpeak-NG (Robotic)", Quality: "low"},
		{ID: "ro", Name: "Romanian", Language: "ro", Gender: "neutral", Provider: "eSpeak-NG (Robotic)", Quality: "low"},
		{ID: "el", Name: "Greek", Language: "el", Gender: "neutral", Provider: "eSpeak-NG (Robotic)", Quality: "low"},
		{ID: "tr", Name: "Turkish", Language: "tr", Gender: "neutral", Provider: "eSpeak-NG (Robotic)", Quality: "low"},
		{ID: "ar", Name: "Arabic", Language: "ar", Gender: "neutral", Provider: "eSpeak-NG (Robotic)", Quality: "low"},
		{ID: "he", Name: "Hebrew", Language: "he", Gender: "neutral", Provider: "eSpeak-NG (Robotic)", Quality: "low"},
		{ID: "fa", Name: "Persian", Language: "fa", Gender: "neutral", Provider: "eSpeak-NG (Robotic)", Quality: "low"},
		{ID: "vi", Name: "Vietnamese", Language: "vi", Gender: "neutral", Provider: "eSpeak-NG (Robotic)", Quality: "low"},
		{ID: "th", Name: "Thai", Language: "th", Gender: "neutral", Provider: "eSpeak-NG (Robotic)", Quality: "low"},
	}
	return voices, nil
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
	if p.vocoder != nil {
		p.vocoder.Close()
		p.vocoder = nil
	}

	if p.initialized {
		C.espeak_cgo_cleanup()
		p.initialized = false
	}
}


// findVocoderModel searches for HiFi-GAN ONNX model file relative to dataPath
func findVocoderModel(dataPath string) string {
	const modelFile = "hifigan_v3.onnx"
	candidates := []string{
		filepath.Join(os.Getenv("HOME"), ".zimaos-blue", "data", "vocoder", modelFile),
		filepath.Join(os.Getenv("HOME"), ".zimaos-blue-dev", "data", "vocoder", modelFile),
	}

	// Add dataPath-relative candidates if dataPath is provided
	if dataPath != "" {
		candidates = append(candidates,
			filepath.Join(dataPath, "vocoder", modelFile),
			filepath.Join(filepath.Dir(dataPath), "vocoder", modelFile),
		)
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}
