//go:build !cgo || !whisper

package stt

/*
#cgo CFLAGS: -I${SRCDIR}/../../../third_party/whisper.cpp/include -I${SRCDIR}/../../../third_party/whisper.cpp/ggml/include
#cgo CFLAGS: -I${SRCDIR}/../../../third_party/opus-src/include
#cgo darwin LDFLAGS: ${SRCDIR}/../../../third_party/whisper.cpp/build/src/libwhisper.a ${SRCDIR}/../../../third_party/whisper.cpp/build/ggml/src/libggml-cpu.a ${SRCDIR}/../../../third_party/whisper.cpp/build/ggml/src/libggml-base.a ${SRCDIR}/../../../third_party/whisper.cpp/build/ggml/src/libggml.a ${SRCDIR}/../../../third_party/opus-src/build/libopus.a ${SRCDIR}/../../../third_party/whisper.cpp/build/ggml/src/ggml-metal/libggml-metal.a ${SRCDIR}/../../../third_party/whisper.cpp/build/ggml/src/ggml-blas/libggml-blas.a -framework Accelerate -framework Metal -framework Foundation -framework CoreGraphics -lm -lstdc++
#cgo linux LDFLAGS: ${SRCDIR}/../../../third_party/whisper.cpp/build/src/libwhisper.a ${SRCDIR}/../../../third_party/whisper.cpp/build/ggml/src/libggml-cpu.a ${SRCDIR}/../../../third_party/whisper.cpp/build/ggml/src/libggml-base.a ${SRCDIR}/../../../third_party/whisper.cpp/build/ggml/src/libggml.a ${SRCDIR}/../../../third_party/opus-src/build/libopus.a -lm -lstdc++
#cgo windows LDFLAGS: ${SRCDIR}/../../../third_party/whisper.cpp/build/src/libwhisper.a ${SRCDIR}/../../../third_party/whisper.cpp/build/ggml/src/ggml-cpu.a ${SRCDIR}/../../../third_party/whisper.cpp/build/ggml/src/ggml-base.a ${SRCDIR}/../../../third_party/whisper.cpp/build/ggml/src/ggml.a ${SRCDIR}/../../../third_party/opus-src/build/libopus.a -lstdc++ -lm -lws2_32 -lwinmm -lgomp
#include <whisper.h>
#include <opus.h>
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
	"os/exec"
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
// Note: During app termination, whisper_free may cause issues with Metal/GPU cleanup.
// We set ctx to nil to prevent double-free but skip the actual free call if it might crash.
func (p *WhisperProvider) Close() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.ctx != nil {
		// whisper_free can crash during app termination due to Metal/GPU resource cleanup
		// The OS will reclaim all resources anyway when the process exits
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
	case "webm", "ogg", "opus":
		// Try native opus decoding first, fallback to ffmpeg
		samples, err := p.decodeOpus(data)
		if err == nil {
			return samples, nil
		}
		// Fallback to ffmpeg
		wavData, err := p.convertWithFFmpeg(data, string(format))
		if err != nil {
			return nil, fmt.Errorf("opus decode and ffmpeg both failed: %w", err)
		}
		return p.parseWAV(wavData)
	default:
		// Try to convert using ffmpeg for other formats (mp3, m4a, etc.)
		wavData, err := p.convertWithFFmpeg(data, string(format))
		if err != nil {
			return nil, fmt.Errorf("unsupported format %s and ffmpeg conversion failed: %w", format, err)
		}
		return p.parseWAV(wavData)
	}
}

// decodeOpus decodes opus/webm/ogg audio to PCM float32 samples using CGO libopus.
func (p *WhisperProvider) decodeOpus(data []byte) ([]float32, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("audio data too short: %d bytes", len(data))
	}

	// Try to extract opus frames from webm/ogg container
	opusFrames, err := extractOpusFrames(data)
	if err != nil {
		return nil, fmt.Errorf("failed to extract opus frames: %w", err)
	}

	if len(opusFrames) == 0 {
		return nil, fmt.Errorf("no opus frames found in %d bytes (magic: %02x%02x%02x%02x)",
			len(data), data[0], data[1], data[2], data[3])
	}

	// Create opus decoder (48kHz stereo is standard for opus)
	var cErr C.int
	decoder := C.opus_decoder_create(48000, 2, &cErr)
	if cErr != 0 {
		return nil, fmt.Errorf("failed to create opus decoder: %d", cErr)
	}
	defer C.opus_decoder_destroy(decoder)

	// Decode all frames
	var allSamples []int16
	pcmBuf := make([]int16, 5760*2) // Max frame size * channels
	decodedFrames := 0

	for _, frame := range opusFrames {
		if len(frame) == 0 {
			continue
		}
		n := C.opus_decode(
			decoder,
			(*C.uchar)(unsafe.Pointer(&frame[0])),
			C.opus_int32(len(frame)),
			(*C.opus_int16)(unsafe.Pointer(&pcmBuf[0])),
			5760,
			0,
		)
		if n > 0 {
			allSamples = append(allSamples, pcmBuf[:int(n)*2]...)
			decodedFrames++
		}
	}

	if len(allSamples) == 0 {
		return nil, fmt.Errorf("no samples decoded from %d frames", len(opusFrames))
	}

	// Convert stereo to mono and resample 48kHz -> 16kHz
	samples := resample48to16Mono(allSamples)

	return samples, nil
}

// extractOpusFrames extracts opus frames from webm/ogg container.
func extractOpusFrames(data []byte) ([][]byte, error) {
	// Check for OGG magic
	if len(data) >= 4 && string(data[:4]) == "OggS" {
		return extractOpusFromOgg(data)
	}
	// Check for WebM/EBML magic
	if len(data) >= 4 && data[0] == 0x1A && data[1] == 0x45 && data[2] == 0xDF && data[3] == 0xA3 {
		return extractOpusFromWebM(data)
	}
	// Try raw opus frames (no container)
	if len(data) > 0 {
		return [][]byte{data}, nil
	}
	return nil, fmt.Errorf("unknown container format: %02x%02x%02x%02x", data[0], data[1], data[2], data[3])
}

// extractOpusFromOgg extracts opus frames from OGG container.
func extractOpusFromOgg(data []byte) ([][]byte, error) {
	var frames [][]byte
	offset := 0

	for offset < len(data)-27 {
		// Check OGG page header
		if string(data[offset:offset+4]) != "OggS" {
			break
		}

		segments := int(data[offset+26])
		if offset+27+segments > len(data) {
			break
		}

		// Calculate segment sizes and extract individual packets
		segmentTable := data[offset+27 : offset+27+segments]
		dataStart := offset + 27 + segments

		// Extract each segment as a potential opus frame
		segOffset := 0
		for _, segSize := range segmentTable {
			if segSize == 0 {
				continue
			}
			segEnd := segOffset + int(segSize)
			if dataStart+segEnd > len(data) {
				break
			}

			segData := data[dataStart+segOffset : dataStart+segEnd]

			// Skip OpusHead and OpusTags packets
			if len(segData) >= 8 && (string(segData[:8]) == "OpusHead" || string(segData[:8]) == "OpusTags") {
				segOffset = segEnd
				continue
			}

			if len(segData) > 0 {
				frames = append(frames, segData)
			}
			segOffset = segEnd
		}

		// Calculate total page size
		pageSize := 0
		for _, s := range segmentTable {
			pageSize += int(s)
		}
		offset = dataStart + pageSize
	}

	return frames, nil
}

// extractOpusFromWebM extracts opus frames from WebM container.
func extractOpusFromWebM(data []byte) ([][]byte, error) {
	var frames [][]byte

	// Parse EBML elements to find SimpleBlock (0xA3) and Block (0xA1) in Clusters
	i := 0
	for i < len(data)-4 {
		// Look for SimpleBlock (0xA3) or Block (0xA1)
		if data[i] == 0xA3 || data[i] == 0xA1 {
			elementStart := i
			i++

			// Read VINT size
			if i >= len(data) {
				break
			}
			size, sizeLen := readEBMLVint(data[i:])
			if sizeLen == 0 || size == 0 {
				i = elementStart + 1
				continue
			}
			i += sizeLen

			// Validate size
			if size > 100000 || i+int(size) > len(data) {
				i = elementStart + 1
				continue
			}

			blockData := data[i : i+int(size)]
			i += int(size)

			// Parse block header: track number (VINT) + timecode (2 bytes) + flags (1 byte for SimpleBlock)
			if len(blockData) < 4 {
				continue
			}

			// Read track number (VINT)
			_, trackLen := readEBMLVint(blockData)
			if trackLen == 0 {
				continue
			}

			// Skip track number + timecode (2 bytes) + flags (1 byte)
			headerLen := trackLen + 3
			if headerLen >= len(blockData) {
				continue
			}

			frameData := blockData[headerLen:]
			if len(frameData) > 0 && len(frameData) < 10000 {
				frames = append(frames, frameData)
			}
		} else {
			i++
		}
	}

	return frames, nil
}

// readEBMLVint reads a variable-length integer from EBML data.
// Returns the value and the number of bytes consumed.
func readEBMLVint(data []byte) (uint64, int) {
	if len(data) == 0 {
		return 0, 0
	}

	first := data[0]
	var length int
	var mask byte

	switch {
	case first&0x80 != 0:
		length = 1
		mask = 0x7F
	case first&0x40 != 0:
		length = 2
		mask = 0x3F
	case first&0x20 != 0:
		length = 3
		mask = 0x1F
	case first&0x10 != 0:
		length = 4
		mask = 0x0F
	case first&0x08 != 0:
		length = 5
		mask = 0x07
	case first&0x04 != 0:
		length = 6
		mask = 0x03
	case first&0x02 != 0:
		length = 7
		mask = 0x01
	case first&0x01 != 0:
		length = 8
		mask = 0x00
	default:
		return 0, 0
	}

	if len(data) < length {
		return 0, 0
	}

	var value uint64 = uint64(first & mask)
	for i := 1; i < length; i++ {
		value = (value << 8) | uint64(data[i])
	}

	return value, length
}

// resample48to16Mono converts 48kHz stereo int16 to 16kHz mono float32.
func resample48to16Mono(samples []int16) []float32 {
	// Simple 3:1 decimation with averaging
	outLen := len(samples) / 6 // stereo 48k -> mono 16k = /6
	result := make([]float32, outLen)

	for i := 0; i < outLen; i++ {
		srcIdx := i * 6
		if srcIdx+5 < len(samples) {
			// Average 3 stereo samples, convert to mono
			sum := int32(samples[srcIdx]) + int32(samples[srcIdx+1]) +
				int32(samples[srcIdx+2]) + int32(samples[srcIdx+3]) +
				int32(samples[srcIdx+4]) + int32(samples[srcIdx+5])
			result[i] = float32(sum) / (6.0 * 32768.0)
		}
	}

	return result
}

// convertWithFFmpeg converts audio to WAV format using ffmpeg.
func (p *WhisperProvider) convertWithFFmpeg(data []byte, format string) ([]byte, error) {
	// Try ffmpeg first (most reliable for all formats)
	wavData, err := p.tryFFmpeg(data, format)
	if err == nil {
		return wavData, nil
	}

	// If ffmpeg not available, return error with hint
	return nil, fmt.Errorf("audio conversion failed: %w (install ffmpeg for %s support)", err, format)
}

// tryFFmpeg attempts to convert audio using ffmpeg command.
func (p *WhisperProvider) tryFFmpeg(data []byte, format string) ([]byte, error) {
	// Check if ffmpeg is available
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, fmt.Errorf("ffmpeg not found")
	}

	// Create temp input file
	tmpIn, err := os.CreateTemp("", "audio_in_*."+format)
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpIn.Name())

	if _, err := tmpIn.Write(data); err != nil {
		tmpIn.Close()
		return nil, err
	}
	tmpIn.Close()

	// Create temp output file
	tmpOut, err := os.CreateTemp("", "audio_out_*.wav")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpOut.Name())
	tmpOut.Close()

	// Run ffmpeg to convert to 16kHz mono WAV
	cmd := exec.Command("ffmpeg", "-y", "-i", tmpIn.Name(),
		"-ar", "16000", "-ac", "1", "-f", "wav", tmpOut.Name())
	cmd.Stderr = nil // Suppress stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg failed: %w", err)
	}

	// Read output
	return os.ReadFile(tmpOut.Name())
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
