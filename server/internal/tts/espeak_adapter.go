package tts

import (
	"context"
	"io"
)

// EspeakNGAdapter adapts EspeakNGProvider to implement the Provider interface
type EspeakNGAdapter struct {
	provider *EspeakNGProvider
}

// NewEspeakNGAdapter creates a new adapter for EspeakNGProvider
func NewEspeakNGAdapter(dataPath string) *EspeakNGAdapter {
	return &EspeakNGAdapter{
		provider: NewEspeakNGProvider(dataPath),
	}
}

// Name returns the provider name
func (a *EspeakNGAdapter) Name() string {
	return a.provider.Name()
}

// Type returns the provider type
func (a *EspeakNGAdapter) Type() ProviderType {
	return ProviderEspeakNG
}

// Synthesize synthesizes text to speech
func (a *EspeakNGAdapter) Synthesize(ctx context.Context, req *SynthesizeRequest) (*SynthesizeResponse, error) {
	// First try to extract language from voice parameter
	language := extractLanguageCode(req.Voice)

	// If no voice specified or default English, detect language from text
	if req.Voice == "" || language == "en" {
		detectedLang, ratio := DetectLanguage(req.Text)
		// Use detected language if confidence is high enough (>30%)
		if ratio > 0.3 {
			language = detectedLang
		}
	}

	// Convert SynthesizeRequest to EspeakNGRequest
	espeakReq := &EspeakNGRequest{
		Text:     req.Text,
		Language: language,
		Voice:    "f3",  // Default to female voice
		Rate:     175,   // Default rate
		Pitch:    50,    // Default pitch
		Volume:   100,   // Default volume
	}

	// Apply speed adjustment
	if req.Speed > 0 {
		espeakReq.Rate = int(175 * req.Speed)
	}

	// Apply pitch adjustment
	if req.Pitch != 0 {
		espeakReq.Pitch = 50 + int(req.Pitch)
	}

	// Synthesize
	audio, err := a.provider.Synthesize(ctx, espeakReq)
	if err != nil {
		return nil, err
	}

	// Determine format
	format := req.Format
	if format == "" {
		format = FormatWAV
	}

	return &SynthesizeResponse{
		Audio:       audio,
		Format:      format,
		ContentType: "audio/wav",
		Duration:    0,
	}, nil
}

// extractLanguageCode extracts eSpeak-NG language code from various voice formats
func extractLanguageCode(voice string) string {
	if voice == "" {
		return "en"
	}

	// Map of Edge TTS voice prefixes to eSpeak-NG language codes
	// Note: Chinese uses "cmn" (Mandarin) in eSpeak-NG, not "zh"
	voiceMap := map[string]string{
		"zh-CN": "cmn",
		"zh-TW": "cmn",
		"zh-HK": "yue", // Cantonese
		"en-US": "en",
		"en-GB": "en",
		"en-AU": "en",
		"en-CA": "en",
		"en-IN": "en",
		"ja-JP": "ja",
		"ko-KR": "ko",
		"es-ES": "es",
		"es-MX": "es",
		"fr-FR": "fr",
		"fr-CA": "fr",
		"de-DE": "de",
		"it-IT": "it",
		"pt-BR": "pt",
		"pt-PT": "pt",
		"ru-RU": "ru",
		"ar-SA": "ar",
		"hi-IN": "hi",
		"th-TH": "th",
		"vi-VN": "vi",
		"tr-TR": "tr",
		"pl-PL": "pl",
		"nl-NL": "nl",
		"sv-SE": "sv",
		"da-DK": "da",
		"no-NO": "no",
		"fi-FI": "fi",
		"cs-CZ": "cs",
		"el-GR": "el",
		"he-IL": "he",
	}

	// Check if it's an Edge TTS voice format (e.g., "zh-CN-XiaoxiaoNeural")
	for prefix, langCode := range voiceMap {
		if len(voice) >= len(prefix) && voice[:len(prefix)] == prefix {
			return langCode
		}
	}

	// Map simple language codes to eSpeak-NG codes
	simpleMap := map[string]string{
		"zh": "cmn",
	}
	if mapped, ok := simpleMap[voice]; ok {
		return mapped
	}

	// If it's already a valid eSpeak-NG language code, return it
	if len(voice) == 2 || len(voice) == 3 {
		return voice
	}

	// Try to extract the first part before hyphen
	if len(voice) > 2 && voice[2] == '-' {
		code := voice[:2]
		if mapped, ok := simpleMap[code]; ok {
			return mapped
		}
		return code
	}

	// Default to English
	return "en"
}

// SynthesizeStream synthesizes text with streaming audio output
func (a *EspeakNGAdapter) SynthesizeStream(ctx context.Context, req *SynthesizeRequest, callback StreamCallback) error {
	resp, err := a.Synthesize(ctx, req)
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

// ListVoices returns available voices
func (a *EspeakNGAdapter) ListVoices(ctx context.Context) ([]Voice, error) {
	langNames := map[string]string{
		"en":  "English",
		"cmn": "Chinese (Mandarin)",
		"yue": "Chinese (Cantonese)",
		"ja":  "Japanese",
		"ko":  "Korean",
		"es":  "Spanish",
		"fr":  "French",
		"de":  "German",
		"it":  "Italian",
		"pt":  "Portuguese",
		"ru":  "Russian",
		"pl":  "Polish",
		"nl":  "Dutch",
		"sv":  "Swedish",
		"no":  "Norwegian",
		"da":  "Danish",
		"fi":  "Finnish",
		"cs":  "Czech",
		"sk":  "Slovak",
		"hu":  "Hungarian",
		"ro":  "Romanian",
		"el":  "Greek",
		"tr":  "Turkish",
		"ar":  "Arabic",
		"he":  "Hebrew",
		"fa":  "Persian",
		"vi":  "Vietnamese",
		"th":  "Thai",
	}

	voices := make([]Voice, 0, len(langNames))
	for _, lang := range a.provider.SupportedLanguages() {
		name := langNames[lang]
		if name == "" {
			name = lang
		}
		voices = append(voices, Voice{
			ID:       lang,
			Name:     name,
			Language: lang,
		})
	}

	return voices, nil
}

// SupportedFormats returns the supported audio formats
func (a *EspeakNGAdapter) SupportedFormats() []AudioFormat {
	return []AudioFormat{FormatWAV}
}

// MaxTextLength returns the maximum text length
func (a *EspeakNGAdapter) MaxTextLength() int {
	return 5000
}

// GetProvider returns the underlying EspeakNGProvider
func (a *EspeakNGAdapter) GetProvider() *EspeakNGProvider {
	return a.provider
}

// Close cleans up resources
func (a *EspeakNGAdapter) Close() {
	a.provider.Close()
}
