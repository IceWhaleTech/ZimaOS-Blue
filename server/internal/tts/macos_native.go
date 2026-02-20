//go:build darwin

package tts

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
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

	// Convert AIFF-C to WAV using afconvert (built-in macOS tool)
	wavFile := tmpFile + ".wav"
	defer os.Remove(wavFile)

	convertCmd := exec.CommandContext(ctx, "afconvert",
		"-f", "WAVE", "-d", "LEI16", tmpFile, wavFile)
	if output, err := convertCmd.CombinedOutput(); err != nil {
		slog.Error("[macos-tts] afconvert failed", "error", err, "output", string(output))
		return nil, fmt.Errorf("afconvert failed: %w", err)
	}

	wavData, err := os.ReadFile(wavFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read WAV output: %w", err)
	}

	slog.Info("[macos-tts] synthesize ok", "wav_bytes", len(wavData))

	return &SynthesizeResponse{
		Audio:       io.NopCloser(bytes.NewReader(wavData)),
		Format:      FormatWAV,
		ContentType: "audio/wav",
	}, nil
}

// SynthesizeStream synthesizes text with streaming output.
// Sends the complete audio as a single callback because the SSE frontend
// closes the EventSource after the first audio event.
func (p *MacOSNativeTTS) SynthesizeStream(ctx context.Context, req *SynthesizeRequest, callback StreamCallback) error {
	resp, err := p.Synthesize(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Audio.Close()

	data, err := io.ReadAll(resp.Audio)
	if err != nil {
		return err
	}
	return callback(data)
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
