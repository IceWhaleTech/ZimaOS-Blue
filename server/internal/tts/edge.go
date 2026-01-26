package tts

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// EdgeTTSProvider implements the Provider interface for Microsoft Edge TTS.
// This is a free TTS service that uses the Edge browser's TTS capabilities.
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
	return []AudioFormat{FormatMP3, FormatWAV}
}

// MaxTextLength returns the maximum text length.
func (p *EdgeTTSProvider) MaxTextLength() int {
	return p.maxTextLength
}

// Synthesize synthesizes text to speech.
// Note: This is a placeholder implementation. In production, you would use
// the edge-tts library or implement the WebSocket protocol.
func (p *EdgeTTSProvider) Synthesize(ctx context.Context, req *SynthesizeRequest) (*SynthesizeResponse, error) {
	if len(req.Text) > p.maxTextLength {
		return nil, ErrTextTooLong
	}

	voice := req.Voice
	if voice == "" {
		voice = p.defaultVoice
	}

	format := req.Format
	if format == "" {
		format = p.defaultFormat
	}

	// In a real implementation, this would connect to the Edge TTS service
	// For now, return an error indicating the service needs to be configured
	return nil, fmt.Errorf("Edge TTS requires external edge-tts service. Voice: %s, Format: %s", voice, format)
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
