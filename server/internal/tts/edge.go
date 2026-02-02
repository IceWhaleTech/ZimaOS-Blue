package tts

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/difyz9/edge-tts-go/pkg/communicate"
)

// EdgeTTSProvider implements the Provider interface for Microsoft Edge TTS.
// This uses the pure Go edge-tts library (no Python/CLI required).
type EdgeTTSProvider struct {
	defaultVoice  string
	defaultFormat AudioFormat
	maxTextLength int
}

// EdgeTTSConfig holds the configuration for the Edge TTS provider.
type EdgeTTSConfig struct {
	DefaultVoice  string
	DefaultFormat AudioFormat
	MaxTextLength int
}

// NewEdgeTTSProvider creates a new Edge TTS provider.
func NewEdgeTTSProvider(cfg *EdgeTTSConfig) *EdgeTTSProvider {
	defaultVoice := cfg.DefaultVoice
	if defaultVoice == "" {
		defaultVoice = "en-US-AriaNeural"
	}
	defaultFormat := cfg.DefaultFormat
	if defaultFormat == "" {
		defaultFormat = FormatMP3
	}
	maxTextLength := cfg.MaxTextLength
	if maxTextLength == 0 {
		maxTextLength = 5000
	}

	return &EdgeTTSProvider{
		defaultVoice:  defaultVoice,
		defaultFormat: defaultFormat,
		maxTextLength: maxTextLength,
	}
}

// Name returns the provider name.
func (p *EdgeTTSProvider) Name() string {
	return "Microsoft Edge TTS"
}

// Type returns the provider type.
func (p *EdgeTTSProvider) Type() ProviderType {
	return ProviderEdge
}

// SupportedFormats returns the supported audio formats.
func (p *EdgeTTSProvider) SupportedFormats() []AudioFormat {
	return []AudioFormat{FormatMP3}
}

// MaxTextLength returns the maximum text length.
func (p *EdgeTTSProvider) MaxTextLength() int {
	return p.maxTextLength
}

// Synthesize synthesizes text to speech using pure Go edge-tts library.
func (p *EdgeTTSProvider) Synthesize(ctx context.Context, req *SynthesizeRequest) (*SynthesizeResponse, error) {
	if len(req.Text) > p.maxTextLength {
		return nil, ErrTextTooLong
	}

	voice := req.Voice
	if voice == "" {
		voice = p.defaultVoice
	}

	// Create edge-tts communicate instance
	comm, err := communicate.NewCommunicate(
		req.Text,
		voice,
		"+0%",  // rate
		"+0%",  // volume
		"+0Hz", // pitch
		"",     // proxy
		10,     // connectTimeout
		60,     // receiveTimeout
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create edge-tts communicate: %w", err)
	}

	// Create temp file for output
	tmpFile, err := os.CreateTemp("", "edge-tts-*.mp3")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()

	// Save audio to temp file
	if err := comm.Save(ctx, tmpPath, ""); err != nil {
		os.Remove(tmpPath)
		return nil, fmt.Errorf("edge-tts synthesis failed: %w", err)
	}

	// Read the audio data
	audioData, err := os.ReadFile(tmpPath)
	os.Remove(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read audio file: %w", err)
	}

	return &SynthesizeResponse{
		Audio:       io.NopCloser(bytes.NewReader(audioData)),
		ContentType: "audio/mpeg",
		Format:      FormatMP3,
	}, nil
}

// SynthesizeStream synthesizes text with streaming audio output.
func (p *EdgeTTSProvider) SynthesizeStream(ctx context.Context, req *SynthesizeRequest, callback StreamCallback) error {
	resp, err := p.Synthesize(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Audio.Close()

	buf := make([]byte, 4096)
	for {
		n, err := resp.Audio.Read(buf)
		if n > 0 {
			if err := callback(buf[:n]); err != nil {
				return err
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read audio: %w", err)
		}
	}

	return nil
}

// ListVoices returns available voices.
func (p *EdgeTTSProvider) ListVoices(ctx context.Context) ([]Voice, error) {
	// Common Edge TTS voices
	return []Voice{
		// English (US)
		{ID: "en-US-AriaNeural", Name: "Aria", Language: "en-US", Gender: "female"},
		{ID: "en-US-GuyNeural", Name: "Guy", Language: "en-US", Gender: "male"},
		{ID: "en-US-JennyNeural", Name: "Jenny", Language: "en-US", Gender: "female"},
		{ID: "en-US-ChristopherNeural", Name: "Christopher", Language: "en-US", Gender: "male"},
		// English (UK)
		{ID: "en-GB-SoniaNeural", Name: "Sonia", Language: "en-GB", Gender: "female"},
		{ID: "en-GB-RyanNeural", Name: "Ryan", Language: "en-GB", Gender: "male"},
		// Chinese (Mandarin)
		{ID: "zh-CN-XiaoxiaoNeural", Name: "Xiaoxiao", Language: "zh-CN", Gender: "female"},
		{ID: "zh-CN-YunxiNeural", Name: "Yunxi", Language: "zh-CN", Gender: "male"},
		{ID: "zh-CN-XiaoyiNeural", Name: "Xiaoyi", Language: "zh-CN", Gender: "female"},
		// Japanese
		{ID: "ja-JP-NanamiNeural", Name: "Nanami", Language: "ja-JP", Gender: "female"},
		{ID: "ja-JP-KeitaNeural", Name: "Keita", Language: "ja-JP", Gender: "male"},
		// Korean
		{ID: "ko-KR-SunHiNeural", Name: "SunHi", Language: "ko-KR", Gender: "female"},
		{ID: "ko-KR-InJoonNeural", Name: "InJoon", Language: "ko-KR", Gender: "male"},
		// German
		{ID: "de-DE-KatjaNeural", Name: "Katja", Language: "de-DE", Gender: "female"},
		{ID: "de-DE-ConradNeural", Name: "Conrad", Language: "de-DE", Gender: "male"},
		// French
		{ID: "fr-FR-DeniseNeural", Name: "Denise", Language: "fr-FR", Gender: "female"},
		{ID: "fr-FR-HenriNeural", Name: "Henri", Language: "fr-FR", Gender: "male"},
		// Spanish
		{ID: "es-ES-ElviraNeural", Name: "Elvira", Language: "es-ES", Gender: "female"},
		{ID: "es-ES-AlvaroNeural", Name: "Alvaro", Language: "es-ES", Gender: "male"},
	}, nil
}

// GetVoicesByLanguage returns voices filtered by language prefix.
func GetVoicesByLanguage(voices []Voice, langPrefix string) []Voice {
	var filtered []Voice
	for _, v := range voices {
		if strings.HasPrefix(v.Language, langPrefix) {
			filtered = append(filtered, v)
		}
	}
	return filtered
}
