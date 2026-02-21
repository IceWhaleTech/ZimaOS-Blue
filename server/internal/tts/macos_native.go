//go:build darwin

package tts

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
)

// MacOSNativeTTS implements TTS using macOS `say` command
type MacOSNativeTTS struct {
	mu       sync.Mutex         // serializes speech (held for duration of say)
	cancelMu sync.Mutex         // protects cancelFn (never held during say)
	cancelFn context.CancelFunc // cancel the currently running say process
}

// NewMacOSNativeTTS creates a new macOS native TTS provider
func NewMacOSNativeTTS() *MacOSNativeTTS {
	return &MacOSNativeTTS{}
}

// Initialize is a no-op; `say` is always available on macOS
func (p *MacOSNativeTTS) Initialize() error {
	return nil
}

// Synthesize generates speech from text using the macOS `say` command.
// It writes AIFF-C to a temp file, then converts to WAV in-memory.
func (p *MacOSNativeTTS) Synthesize(ctx context.Context, req *SynthesizeRequest) (*SynthesizeResponse, error) {
	wavData, err := p.synthesizeToWAV(ctx, req)
	if err != nil {
		return nil, err
	}
	return &SynthesizeResponse{
		Audio:       io.NopCloser(bytes.NewReader(wavData)),
		Format:      FormatWAV,
		ContentType: "audio/wav",
	}, nil
}

// synthesizeToWAV is the core synthesis path, returning raw WAV bytes.
func (p *MacOSNativeTTS) synthesizeToWAV(ctx context.Context, req *SynthesizeRequest) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if req.Text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	slog.Info("[macos-tts] synthesize start", "text_len", len(req.Text), "speed", req.Speed)

	// Create temp file for AIFF output
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("tts_%d.aiff", os.Getpid()))
	defer os.Remove(tmpFile)

	// Build say command
	args := []string{"-o", tmpFile}

	// Select voice based on language detection
	if voice := detectMacOSVoice(req.Text); voice != "" {
		args = append(args, "-v", voice)
	}

	if req.Speed > 0 && req.Speed != 1.0 {
		wpm := int(req.Speed * 200)
		if wpm < 80 {
			wpm = 80
		}
		if wpm > 500 {
			wpm = 500
		}
		args = append(args, "-r", strconv.Itoa(wpm))
	}
	args = append(args, req.Text)

	cmd := exec.CommandContext(ctx, "say", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		slog.Error("[macos-tts] say command failed", "error", err, "output", string(output))
		return nil, fmt.Errorf("say command failed: %w", err)
	}

	// Read the AIFF-C file
	aiffData, err := os.ReadFile(tmpFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read TTS output: %w", err)
	}

	if len(aiffData) < 54 {
		return nil, fmt.Errorf("say produced empty audio (%d bytes)", len(aiffData))
	}

	slog.Info("[macos-tts] say produced AIFF", "bytes", len(aiffData))

	// Convert AIFF-C to WAV in-process (avoids forking afconvert)
	wavData, err := aiffcToWav(aiffData)
	if err != nil {
		slog.Error("[macos-tts] AIFF-C to WAV conversion failed", "error", err)
		return nil, fmt.Errorf("AIFF-C to WAV conversion failed: %w", err)
	}

	slog.Info("[macos-tts] synthesize ok", "wav_bytes", len(wavData))
	return wavData, nil
}

// SynthesizeStream synthesizes text with streaming output.
// Sends the complete audio as a single callback because the SSE frontend
// closes the EventSource after the first audio event.
func (p *MacOSNativeTTS) SynthesizeStream(ctx context.Context, req *SynthesizeRequest, callback StreamCallback) error {
	wavData, err := p.synthesizeToWAV(ctx, req)
	if err != nil {
		return err
	}
	return callback(wavData)
}

// ListVoices returns available system voices
func (p *MacOSNativeTTS) ListVoices(ctx context.Context) ([]Voice, error) {
	return []Voice{
		{ID: "default", Name: "System Default", Language: "en-US", Gender: "neutral", Provider: "macOS Native", Quality: "high"},
	}, nil
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

// SpeakLocally plays text through local audio output (blocking until done).
func (p *MacOSNativeTTS) SpeakLocally(ctx context.Context, text string, speed float32) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	cmdCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	p.cancelMu.Lock()
	p.cancelFn = cancel
	p.cancelMu.Unlock()

	defer func() {
		p.cancelMu.Lock()
		p.cancelFn = nil
		p.cancelMu.Unlock()
	}()

	args := []string{}

	// Select voice based on language detection
	if voice := detectMacOSVoice(text); voice != "" {
		args = append(args, "-v", voice)
	}

	if speed > 0 && speed != 1.0 {
		wpm := int(speed * 200)
		if wpm < 80 {
			wpm = 80
		}
		if wpm > 500 {
			wpm = 500
		}
		args = append(args, "-r", strconv.Itoa(wpm))
	}
	args = append(args, text)

	slog.Info("[macos-tts] speak locally", "text_len", len(text), "speed", speed)
	cmd := exec.CommandContext(cmdCtx, "say", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		if cmdCtx.Err() != nil {
			slog.Info("[macos-tts] speak locally cancelled")
			return cmdCtx.Err()
		}
		slog.Error("[macos-tts] say failed", "error", err, "output", string(output))
		return fmt.Errorf("say failed: %w", err)
	}
	slog.Info("[macos-tts] speak locally done")
	return nil
}

// StopSpeaking cancels any currently running local speech.
func (p *MacOSNativeTTS) StopSpeaking() {
	p.cancelMu.Lock()
	defer p.cancelMu.Unlock()
	if p.cancelFn != nil {
		p.cancelFn()
		p.cancelFn = nil
	}
}

// Close is a no-op
func (p *MacOSNativeTTS) Close() {}

// Available returns true if macOS native TTS is available
func (p *MacOSNativeTTS) Available() bool {
	_, err := exec.LookPath("say")
	return err == nil
}

// detectMacOSVoice auto-detects language and returns a macOS voice name.
// macOS ships with voices for many languages; these are common built-in ones.
func detectMacOSVoice(text string) string {
	lang, _ := DetectLanguage(text)
	switch lang {
	case "ja":
		return "Kyoko" // Japanese female (built-in)
	case "ko":
		return "Yuna" // Korean female (built-in)
	case "cmn", "zh":
		return "Tingting" // Chinese female (built-in)
	default:
		return "" // system default
	}
}

// aiffcToWav converts AIFF-C (compressed, ima4/sowt) data to 16-bit mono PCM WAV in-process.
// macOS `say -o` produces AIFF-C with "sowt" (little-endian int16) compression type.
func aiffcToWav(data []byte) ([]byte, error) {
	if len(data) < 12 || string(data[0:4]) != "FORM" {
		return nil, fmt.Errorf("not an AIFF file")
	}
	formType := string(data[8:12])
	if formType != "AIFC" && formType != "AIFF" {
		return nil, fmt.Errorf("unsupported FORM type: %s", formType)
	}

	var numChannels, bitsPerSample int
	var sampleRate int
	var soundData []byte
	var compressionType string // AIFF-C: "twos" (big-endian), "sowt" (little-endian), "NONE"

	// Parse IFF chunks
	offset := 12
	for offset+8 <= len(data) {
		chunkID := string(data[offset : offset+4])
		chunkSize := int(binary.BigEndian.Uint32(data[offset+4 : offset+8]))
		chunkData := offset + 8
		if chunkData+chunkSize > len(data) {
			chunkSize = len(data) - chunkData
		}

		switch chunkID {
		case "COMM":
			if chunkSize < 18 {
				break
			}
			numChannels = int(binary.BigEndian.Uint16(data[chunkData : chunkData+2]))
			// bytes 2-5: numSampleFrames (uint32)
			bitsPerSample = int(binary.BigEndian.Uint16(data[chunkData+6 : chunkData+8]))
			// bytes 8-17: sample rate as 80-bit extended float
			sampleRate = int(parseIEEE754Extended(data[chunkData+8 : chunkData+18]))
			// AIFF-C has compression type at offset 18
			if formType == "AIFC" && chunkSize >= 22 {
				compressionType = string(data[chunkData+18 : chunkData+22])
			}

		case "SSND":
			if chunkSize < 8 {
				break
			}
			ssndOffset := int(binary.BigEndian.Uint32(data[chunkData : chunkData+4]))
			// blockSize at chunkData+4 (ignored)
			soundStart := chunkData + 8 + ssndOffset
			if soundStart < len(data) {
				soundData = data[soundStart : chunkData+chunkSize]
			}
		}

		// Chunks are padded to even size
		offset += 8 + chunkSize
		if chunkSize%2 != 0 {
			offset++
		}
	}

	if soundData == nil || sampleRate == 0 {
		return nil, fmt.Errorf("missing COMM or SSND chunk")
	}

	// Determine if PCM data needs byte-swapping to little-endian (WAV format).
	// "twos" = big-endian int16, "sowt" = little-endian int16, "NONE" = big-endian.
	// Plain AIFF (not AIFC) is always big-endian.
	needSwap := formType == "AIFF" || compressionType == "twos" || compressionType == "NONE"
	pcmData := soundData
	if needSwap && bitsPerSample == 16 {
		pcmData = make([]byte, len(soundData))
		// Swap 8 bytes at a time for throughput, then handle remainder
		n := len(soundData)
		i := 0
		for ; i+7 < n; i += 8 {
			pcmData[i] = soundData[i+1]
			pcmData[i+1] = soundData[i]
			pcmData[i+2] = soundData[i+3]
			pcmData[i+3] = soundData[i+2]
			pcmData[i+4] = soundData[i+5]
			pcmData[i+5] = soundData[i+4]
			pcmData[i+6] = soundData[i+7]
			pcmData[i+7] = soundData[i+6]
		}
		for ; i+1 < n; i += 2 {
			pcmData[i] = soundData[i+1]
			pcmData[i+1] = soundData[i]
		}
	}

	// Downmix to mono if stereo
	if numChannels == 2 && bitsPerSample == 16 {
		mono := make([]byte, len(pcmData)/2)
		for i := 0; i+3 < len(pcmData); i += 4 {
			l := int16(binary.LittleEndian.Uint16(pcmData[i:]))
			r := int16(binary.LittleEndian.Uint16(pcmData[i+2:]))
			m := int16((int32(l) + int32(r)) / 2)
			binary.LittleEndian.PutUint16(mono[i/2:], uint16(m))
		}
		pcmData = mono
		numChannels = 1
	}

	// Build WAV header directly (avoids 13x binary.Write reflection overhead)
	dataSize := len(pcmData)
	bytesPerSample := bitsPerSample / 8
	out := make([]byte, 44+dataSize)
	copy(out[0:4], "RIFF")
	binary.LittleEndian.PutUint32(out[4:8], uint32(36+dataSize))
	copy(out[8:12], "WAVE")
	copy(out[12:16], "fmt ")
	binary.LittleEndian.PutUint32(out[16:20], 16)
	binary.LittleEndian.PutUint16(out[20:22], 1) // PCM
	binary.LittleEndian.PutUint16(out[22:24], uint16(numChannels))
	binary.LittleEndian.PutUint32(out[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(out[28:32], uint32(sampleRate*numChannels*bytesPerSample))
	binary.LittleEndian.PutUint16(out[32:34], uint16(numChannels*bytesPerSample))
	binary.LittleEndian.PutUint16(out[34:36], uint16(bitsPerSample))
	copy(out[36:40], "data")
	binary.LittleEndian.PutUint32(out[40:44], uint32(dataSize))
	copy(out[44:], pcmData)
	return out, nil
}

// parseIEEE754Extended parses an 80-bit IEEE 754 extended precision float.
// Used for AIFF sample rate field.
func parseIEEE754Extended(b []byte) float64 {
	if len(b) < 10 {
		return 0
	}
	sign := int(b[0] >> 7)
	exponent := int(binary.BigEndian.Uint16(b[0:2])) & 0x7FFF
	mantissa := binary.BigEndian.Uint64(b[2:10])

	if exponent == 0 && mantissa == 0 {
		return 0
	}

	f := float64(mantissa) / (1 << 63)
	f = math.Ldexp(f, exponent-16383)
	if sign == 1 {
		f = -f
	}
	return f
}
