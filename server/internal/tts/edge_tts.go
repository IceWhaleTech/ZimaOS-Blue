package tts

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/difyz9/edge-tts-go/pkg/communicate"
)

// EdgeTTSProvider implements the Provider interface for Microsoft Edge TTS.
type EdgeTTSProvider struct {
	defaultVoice  string
	defaultFormat AudioFormat
	maxTextLength int
}

// NewEdgeTTSProvider creates a new Edge TTS provider.
func NewEdgeTTSProvider() *EdgeTTSProvider {
	return &EdgeTTSProvider{
		defaultVoice:  "zh-CN-XiaoxiaoNeural",
		defaultFormat: FormatMP3,
		maxTextLength: 5000,
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

// IsAvailable always returns true for Edge TTS.
func (p *EdgeTTSProvider) IsAvailable() bool {
	return true
}

// SupportedFormats returns the supported audio formats.
func (p *EdgeTTSProvider) SupportedFormats() []AudioFormat {
	return []AudioFormat{FormatMP3}
}

// MaxTextLength returns the maximum text length.
func (p *EdgeTTSProvider) MaxTextLength() int {
	return p.maxTextLength
}

// Synthesize synthesizes text to speech using edge-tts-go library.
func (p *EdgeTTSProvider) Synthesize(ctx context.Context, req *SynthesizeRequest) (*SynthesizeResponse, error) {
	if len(req.Text) > p.maxTextLength {
		return nil, ErrTextTooLong
	}

	voice := req.Voice
	if voice == "" {
		voice = p.defaultVoice
	}

	// Convert speed to rate string (e.g., 1.2 -> "+20%", 0.8 -> "-20%")
	rate := "+0%"
	if req.Speed > 0 && req.Speed != 1.0 {
		percent := int((req.Speed - 1.0) * 100)
		if percent >= 0 {
			rate = fmt.Sprintf("+%d%%", percent)
		} else {
			rate = fmt.Sprintf("%d%%", percent)
		}
	}

	// Convert volume (0-100 scale to percentage, default 100 = +0%)
	volume := "+0%"
	if req.Volume > 0 && req.Volume != 100 {
		percent := int(req.Volume) - 100
		if percent >= 0 {
			volume = fmt.Sprintf("+%d%%", percent)
		} else {
			volume = fmt.Sprintf("%d%%", percent)
		}
	}

	// Convert pitch adjustment to Hz string
	pitch := "+0Hz"
	if req.Pitch != 0 {
		pitch = fmt.Sprintf("%+.0fHz", req.Pitch*10)
	}

	const maxRetries = 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(attempt) * 500 * time.Millisecond
			log.Printf("[edge-tts] retry %d/%d after %v (prev error: %v)", attempt, maxRetries-1, delay, lastErr)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		comm, err := communicate.NewCommunicate(
			req.Text, voice, rate, volume, pitch,
			"",  // proxy
			10,  // connectTimeout
			60,  // receiveTimeout
		)
		if err != nil {
			lastErr = fmt.Errorf("failed to create edge-tts communicate: %w", err)
			continue
		}

		tmpFile, err := os.CreateTemp("", "edge-tts-*.mp3")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp file: %w", err)
		}
		tmpPath := tmpFile.Name()
		tmpFile.Close()

		if err := comm.Save(ctx, tmpPath, ""); err != nil {
			os.Remove(tmpPath)
			lastErr = fmt.Errorf("edge-tts synthesis failed: %w", err)
			continue
		}

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

	return nil, lastErr
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
	return []Voice{
		// Chinese (Mandarin)
		{ID: "zh-CN-XiaoxiaoNeural", Name: "晓晓 (女)", Language: "zh-CN", Gender: "female"},
		{ID: "zh-CN-YunxiNeural", Name: "云希 (男)", Language: "zh-CN", Gender: "male"},
		{ID: "zh-CN-XiaoyiNeural", Name: "晓艺 (女)", Language: "zh-CN", Gender: "female"},
		{ID: "zh-CN-YunjianNeural", Name: "云健 (男)", Language: "zh-CN", Gender: "male"},
		// Chinese (Taiwan)
		{ID: "zh-TW-HsiaoChenNeural", Name: "曉臻 (女)", Language: "zh-TW", Gender: "female"},
		{ID: "zh-TW-YunJheNeural", Name: "雲哲 (男)", Language: "zh-TW", Gender: "male"},
		// English (US)
		{ID: "en-US-AriaNeural", Name: "Aria (Female)", Language: "en-US", Gender: "female"},
		{ID: "en-US-GuyNeural", Name: "Guy (Male)", Language: "en-US", Gender: "male"},
		{ID: "en-US-JennyNeural", Name: "Jenny (Female)", Language: "en-US", Gender: "female"},
		{ID: "en-US-ChristopherNeural", Name: "Christopher (Male)", Language: "en-US", Gender: "male"},
		// English (UK)
		{ID: "en-GB-SoniaNeural", Name: "Sonia (Female)", Language: "en-GB", Gender: "female"},
		{ID: "en-GB-RyanNeural", Name: "Ryan (Male)", Language: "en-GB", Gender: "male"},
		// Japanese
		{ID: "ja-JP-NanamiNeural", Name: "七海 (女)", Language: "ja-JP", Gender: "female"},
		{ID: "ja-JP-KeitaNeural", Name: "圭太 (男)", Language: "ja-JP", Gender: "male"},
		// Korean
		{ID: "ko-KR-SunHiNeural", Name: "선히 (여)", Language: "ko-KR", Gender: "female"},
		{ID: "ko-KR-InJoonNeural", Name: "인준 (남)", Language: "ko-KR", Gender: "male"},
		// German
		{ID: "de-DE-KatjaNeural", Name: "Katja (Weiblich)", Language: "de-DE", Gender: "female"},
		{ID: "de-DE-ConradNeural", Name: "Conrad (Männlich)", Language: "de-DE", Gender: "male"},
		// French
		{ID: "fr-FR-DeniseNeural", Name: "Denise (Femme)", Language: "fr-FR", Gender: "female"},
		{ID: "fr-FR-HenriNeural", Name: "Henri (Homme)", Language: "fr-FR", Gender: "male"},
		// Spanish
		{ID: "es-ES-ElviraNeural", Name: "Elvira (Mujer)", Language: "es-ES", Gender: "female"},
		{ID: "es-ES-AlvaroNeural", Name: "Alvaro (Hombre)", Language: "es-ES", Gender: "male"},
		// Portuguese
		{ID: "pt-BR-FranciscaNeural", Name: "Francisca (Feminino)", Language: "pt-BR", Gender: "female"},
		{ID: "pt-BR-AntonioNeural", Name: "Antonio (Masculino)", Language: "pt-BR", Gender: "male"},
		// Russian
		{ID: "ru-RU-SvetlanaNeural", Name: "Светлана (Жен)", Language: "ru-RU", Gender: "female"},
		{ID: "ru-RU-DmitryNeural", Name: "Дмитрий (Муж)", Language: "ru-RU", Gender: "male"},
		// Italian
		{ID: "it-IT-ElsaNeural", Name: "Elsa (Donna)", Language: "it-IT", Gender: "female"},
		{ID: "it-IT-DiegoNeural", Name: "Diego (Uomo)", Language: "it-IT", Gender: "male"},
		// Dutch
		{ID: "nl-NL-ColetteNeural", Name: "Colette (Vrouw)", Language: "nl-NL", Gender: "female"},
		{ID: "nl-NL-MaartenNeural", Name: "Maarten (Man)", Language: "nl-NL", Gender: "male"},
		// Polish
		{ID: "pl-PL-AgnieszkaNeural", Name: "Agnieszka (Kobieta)", Language: "pl-PL", Gender: "female"},
		{ID: "pl-PL-MarekNeural", Name: "Marek (Mężczyzna)", Language: "pl-PL", Gender: "male"},
		// Swedish
		{ID: "sv-SE-SofieNeural", Name: "Sofie (Kvinna)", Language: "sv-SE", Gender: "female"},
		{ID: "sv-SE-MattiasNeural", Name: "Mattias (Man)", Language: "sv-SE", Gender: "male"},
		// Norwegian
		{ID: "nb-NO-PernilleNeural", Name: "Pernille (Kvinne)", Language: "nb-NO", Gender: "female"},
		{ID: "nb-NO-FinnNeural", Name: "Finn (Mann)", Language: "nb-NO", Gender: "male"},
		// Danish
		{ID: "da-DK-ChristelNeural", Name: "Christel (Kvinde)", Language: "da-DK", Gender: "female"},
		{ID: "da-DK-JeppeNeural", Name: "Jeppe (Mand)", Language: "da-DK", Gender: "male"},
		// Finnish
		{ID: "fi-FI-NooraNeural", Name: "Noora (Nainen)", Language: "fi-FI", Gender: "female"},
		{ID: "fi-FI-HarriNeural", Name: "Harri (Mies)", Language: "fi-FI", Gender: "male"},
		// Czech
		{ID: "cs-CZ-VlastaNeural", Name: "Vlasta (Žena)", Language: "cs-CZ", Gender: "female"},
		{ID: "cs-CZ-AntoninNeural", Name: "Antonín (Muž)", Language: "cs-CZ", Gender: "male"},
		// Greek
		{ID: "el-GR-AthinaNeural", Name: "Αθηνά (Γυναίκα)", Language: "el-GR", Gender: "female"},
		{ID: "el-GR-NestorasNeural", Name: "Νέστορας (Άνδρας)", Language: "el-GR", Gender: "male"},
		// Turkish
		{ID: "tr-TR-EmelNeural", Name: "Emel (Kadın)", Language: "tr-TR", Gender: "female"},
		{ID: "tr-TR-AhmetNeural", Name: "Ahmet (Erkek)", Language: "tr-TR", Gender: "male"},
		// Hungarian
		{ID: "hu-HU-NoemiNeural", Name: "Noémi (Nő)", Language: "hu-HU", Gender: "female"},
		{ID: "hu-HU-TamasNeural", Name: "Tamás (Férfi)", Language: "hu-HU", Gender: "male"},
		// Romanian
		{ID: "ro-RO-AlinaNeural", Name: "Alina (Femeie)", Language: "ro-RO", Gender: "female"},
		{ID: "ro-RO-EmilNeural", Name: "Emil (Bărbat)", Language: "ro-RO", Gender: "male"},
		// Ukrainian
		{ID: "uk-UA-PolinaNeural", Name: "Поліна (Жін)", Language: "uk-UA", Gender: "female"},
		{ID: "uk-UA-OstapNeural", Name: "Остап (Чол)", Language: "uk-UA", Gender: "male"},
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
