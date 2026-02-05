package stt

/*
#cgo CFLAGS: -I${SRCDIR}/../../../third_party/whisper.cpp/include -I${SRCDIR}/../../../third_party/whisper.cpp/ggml/include
#cgo LDFLAGS: ${SRCDIR}/../../../third_party/whisper.cpp/build/src/libwhisper.a
#cgo LDFLAGS: ${SRCDIR}/../../../third_party/whisper.cpp/build/ggml/src/libggml.a
#cgo LDFLAGS: ${SRCDIR}/../../../third_party/whisper.cpp/build/ggml/src/libggml-base.a
#cgo LDFLAGS: ${SRCDIR}/../../../third_party/whisper.cpp/build/ggml/src/libggml-cpu.a
#cgo darwin LDFLAGS: ${SRCDIR}/../../../third_party/whisper.cpp/build/ggml/src/ggml-metal/libggml-metal.a
#cgo darwin LDFLAGS: ${SRCDIR}/../../../third_party/whisper.cpp/build/ggml/src/ggml-blas/libggml-blas.a
#cgo darwin LDFLAGS: -framework Accelerate -framework Metal -framework Foundation -framework CoreGraphics
#include <whisper.h>
#include <stdlib.h>
*/
import "C"

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
	"unsafe"
)

// WhisperConfig holds the configuration for the Whisper provider.
type WhisperConfig struct {
	ModelPath   string
	DefaultLang string
	MaxDuration time.Duration
	Threads     int
}

// WhisperProvider implements the Provider interface using whisper.cpp.
type WhisperProvider struct {
	ctx          *C.struct_whisper_context
	config       *WhisperConfig
	modelManager *WhisperModelManager
	mu           sync.Mutex
	initialized  bool
}

// NewWhisperProvider creates a new Whisper provider.
func NewWhisperProvider(cfg *WhisperConfig) *WhisperProvider {
	if cfg.MaxDuration == 0 {
		cfg.MaxDuration = 30 * time.Second
	}
	if cfg.Threads == 0 {
		cfg.Threads = 4
	}
	p := &WhisperProvider{
		config:       cfg,
		modelManager: NewWhisperModelManager(cfg.ModelPath),
	}
	// Auto-initialize with persisted model if available
	if activeModel := p.modelManager.GetActiveModel(); activeModel != "" {
		modelPath := p.modelManager.GetModelPath(activeModel)
		if modelPath != "" {
			if _, err := os.Stat(modelPath); err == nil {
				_ = p.Initialize(modelPath)
			}
		}
	}
	return p
}

// Initialize loads the whisper model.
func (p *WhisperProvider) Initialize(modelPath string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.initialized {
		return nil
	}

	cPath := C.CString(modelPath)
	defer C.free(unsafe.Pointer(cPath))

	p.ctx = C.whisper_init_from_file_with_params(cPath, C.whisper_context_default_params())
	if p.ctx == nil {
		return fmt.Errorf("failed to load whisper model: %s", modelPath)
	}

	p.config.ModelPath = modelPath
	p.initialized = true

	// Set active model in manager based on the loaded model path
	if modelID := p.modelManager.GetModelIDFromPath(modelPath); modelID != "" {
		p.modelManager.SetActiveModel(modelID)
	}

	return nil
}

// Close releases the whisper context.
func (p *WhisperProvider) Close() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.ctx != nil {
		C.whisper_free(p.ctx)
		p.ctx = nil
	}
	p.initialized = false
}

// Name returns the provider name.
func (p *WhisperProvider) Name() string {
	return "Whisper"
}

// Type returns the provider type.
func (p *WhisperProvider) Type() ProviderType {
	return ProviderWhisper
}

// Transcribe transcribes audio to text.
func (p *WhisperProvider) Transcribe(ctx context.Context, req *TranscribeRequest) (*TranscribeResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.initialized {
		return nil, fmt.Errorf("whisper provider not initialized")
	}

	// Read audio data
	audioData, err := io.ReadAll(req.Audio)
	if err != nil {
		return nil, fmt.Errorf("failed to read audio: %w", err)
	}

	// Convert to PCM float32
	samples, err := p.convertToPCM(audioData, req.Format)
	if err != nil {
		return nil, fmt.Errorf("failed to convert audio: %w", err)
	}

	if len(samples) == 0 {
		return nil, ErrAudioTooShort
	}

	// Set up parameters
	params := C.whisper_full_default_params(C.WHISPER_SAMPLING_GREEDY)
	params.print_progress = C.bool(false)
	params.print_special = C.bool(false)
	params.print_realtime = C.bool(false)
	params.print_timestamps = C.bool(false)
	params.translate = C.bool(false)
	params.single_segment = C.bool(false)
	params.n_threads = C.int(p.config.Threads)

	// Set language
	if req.Language != "" {
		cLang := C.CString(req.Language)
		defer C.free(unsafe.Pointer(cLang))
		params.language = cLang
	}

	// Run inference
	ret := C.whisper_full(p.ctx, params, (*C.float)(&samples[0]), C.int(len(samples)))
	if ret != 0 {
		return nil, ErrTranscriptionFailed
	}

	// Collect results
	nSegments := int(C.whisper_full_n_segments(p.ctx))
	var text string
	segments := make([]Segment, 0, nSegments)

	for i := 0; i < nSegments; i++ {
		segText := C.GoString(C.whisper_full_get_segment_text(p.ctx, C.int(i)))
		text += segText

		t0 := float64(C.whisper_full_get_segment_t0(p.ctx, C.int(i))) / 100.0
		t1 := float64(C.whisper_full_get_segment_t1(p.ctx, C.int(i))) / 100.0

		segments = append(segments, Segment{
			ID:    i,
			Start: t0,
			End:   t1,
			Text:  segText,
		})
	}

	return &TranscribeResponse{
		Text:     text,
		Language: req.Language,
		Duration: float64(len(samples)) / 16000.0,
		Segments: segments,
	}, nil
}

// TranscribeStream transcribes audio with streaming results.
func (p *WhisperProvider) TranscribeStream(ctx context.Context, req *TranscribeRequest, callback StreamCallback) error {
	// For now, use non-streaming transcription
	resp, err := p.Transcribe(ctx, req)
	if err != nil {
		return err
	}
	return callback(resp)
}

// SupportedFormats returns the supported audio formats.
func (p *WhisperProvider) SupportedFormats() []AudioFormat {
	return []AudioFormat{FormatWAV, FormatPCM}
}

// MaxDuration returns the maximum audio duration.
func (p *WhisperProvider) MaxDuration() time.Duration {
	return p.config.MaxDuration
}

// IsInitialized returns whether the provider is initialized.
func (p *WhisperProvider) IsInitialized() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.initialized
}

// convertToPCM converts audio data to PCM float32 samples at 16kHz.
func (p *WhisperProvider) convertToPCM(data []byte, format AudioFormat) ([]float32, error) {
	switch format {
	case FormatWAV:
		return p.parseWAV(data)
	case FormatPCM:
		return p.parsePCM16(data)
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

// parseWAV parses WAV audio data.
func (p *WhisperProvider) parseWAV(data []byte) ([]float32, error) {
	if len(data) < 44 {
		return nil, fmt.Errorf("WAV data too short")
	}

	// Skip WAV header (44 bytes for standard WAV)
	// Find "data" chunk
	dataOffset := 12
	for dataOffset < len(data)-8 {
		chunkID := string(data[dataOffset : dataOffset+4])
		chunkSize := binary.LittleEndian.Uint32(data[dataOffset+4 : dataOffset+8])
		if chunkID == "data" {
			dataOffset += 8
			break
		}
		dataOffset += 8 + int(chunkSize)
	}

	if dataOffset >= len(data) {
		return nil, fmt.Errorf("no data chunk found in WAV")
	}

	return p.parsePCM16(data[dataOffset:])
}

// parsePCM16 parses 16-bit PCM audio data.
func (p *WhisperProvider) parsePCM16(data []byte) ([]float32, error) {
	reader := bytes.NewReader(data)
	samples := make([]float32, 0, len(data)/2)

	for {
		var sample int16
		err := binary.Read(reader, binary.LittleEndian, &sample)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		samples = append(samples, float32(sample)/32768.0)
	}

	return samples, nil
}

// GetModelStatus returns the status of the active model.
func (p *WhisperProvider) GetModelStatus() ModelStatus {
	p.mu.Lock()
	initialized := p.initialized
	p.mu.Unlock()

	status := p.modelManager.GetModelStatus()
	// Only ready if provider is actually initialized with a loaded model
	status.Ready = initialized && status.Ready
	return status
}

// ListModels returns all models with download status.
func (p *WhisperProvider) ListModels() []interface{} {
	return p.modelManager.ListModels()
}

// DownloadModel downloads a whisper model.
func (p *WhisperProvider) DownloadModel(ctx context.Context, modelType string) error {
	return p.modelManager.DownloadModel(ctx, modelType)
}

// CancelDownload cancels the current download.
func (p *WhisperProvider) CancelDownload() {
	p.modelManager.CancelDownload()
}

// SwitchModel switches to a different whisper model.
func (p *WhisperProvider) SwitchModel(modelType string) error {
	modelPath := p.modelManager.GetModelPath(modelType)
	if modelPath == "" {
		return fmt.Errorf("unknown model: %s", modelType)
	}

	// Check if model is downloaded
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return fmt.Errorf("model not downloaded: %s", modelType)
	}

	// Close current model
	p.Close()

	// Initialize with new model
	if err := p.Initialize(modelPath); err != nil {
		return err
	}

	// Update active model in manager
	p.modelManager.SetActiveModel(modelType)
	return nil
}
