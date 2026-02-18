//go:build kokoro

package tts

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/onnx"
)

// KokoroProvider implements TTS using Kokoro ONNX model
// Supports 9 languages with high-quality voices:
// - English (US): 11 Female, 9 Male
// - English (UK): 4 Female, 4 Male
// - Japanese: 4 Female, 1 Male
// - Mandarin Chinese: 4 Female, 4 Male
// - Spanish: 1 Female, 2 Male
// - French: 1 Female
// - Hindi: 2 Female, 2 Male
// - Italian: 1 Female, 1 Male
// - Brazilian Portuguese: 1 Female, 2 Male
type KokoroProvider struct {
	modelPath   string
	initialized bool
	mu          sync.Mutex
	session     *onnx.DynamicSession
	langMap     map[string]string // language code -> Kokoro language code
}

// NewKokoroProvider creates a new Kokoro provider
func NewKokoroProvider(dataPath string) *KokoroProvider {
	return &KokoroProvider{
		modelPath: findKokoroModel(dataPath),
		langMap: map[string]string{
			"en-US": "en_US",
			"en-GB": "en_GB",
			"ja-JP": "ja_JP",
			"zh-CN": "zh_CN",
			"es-ES": "es_ES",
			"fr-FR": "fr_FR",
			"hi-IN": "hi_IN",
			"it-IT": "it_IT",
			"pt-BR": "pt_BR",
		},
	}
}

// SupportedLanguages returns list of languages Kokoro can handle
func (p *KokoroProvider) SupportedLanguages() []string {
	return []string{
		"en-US", "en-GB", "ja-JP", "zh-CN", "es-ES", "fr-FR", "hi-IN", "it-IT", "pt-BR",
	}
}

// CanHandle checks if this provider can handle the language
func (p *KokoroProvider) CanHandle(language string) bool {
	_, ok := p.langMap[language]
	return ok
}

// Initialize initializes the Kokoro provider
func (p *KokoroProvider) Initialize() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.initialized {
		return nil
	}

	if p.modelPath == "" {
		return fmt.Errorf("kokoro model not found at expected locations")
	}

	// Load ONNX model using shared runtime
	session, err := onnx.NewDynamicSession(
		p.modelPath,
		[]string{"input_ids", "voice_id"},
		[]string{"audio"},
	)
	if err != nil {
		return fmt.Errorf("failed to load Kokoro ONNX model: %w", err)
	}

	p.session = session
	p.initialized = true
	return nil
}

// Synthesize generates speech from text using Kokoro
func (p *KokoroProvider) Synthesize(ctx context.Context, req *SynthesizeRequest) (*SynthesizeResponse, error) {
	if err := p.Initialize(); err != nil {
		return nil, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if req.Text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	// Check if language is supported
	if !p.CanHandle(req.Voice) {
		return nil, fmt.Errorf("language %s not supported by Kokoro, please switch to another provider", req.Voice)
	}

	// Tokenize text
	tokenizer := NewKokoroTokenizer()
	tokens := tokenizer.Tokenize(req.Text)
	voiceID := tokenizer.GetVoiceID(req.Voice)

	// Prepare input tensors
	inputIDs := make([]int64, len(tokens))
	copy(inputIDs, tokens)

	voiceIDs := []int64{voiceID}

	// Run ONNX inference
	outputs, err := p.session.Run(map[string]interface{}{
		"input_ids": inputIDs,
		"voice_id":  voiceIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("kokoro inference failed: %w", err)
	}

	// Extract audio output
	audioOutput, ok := outputs["audio"]
	if !ok {
		return nil, fmt.Errorf("kokoro inference did not produce audio output")
	}

	// Convert to audio data
	audioData, err := p.convertToAudio(audioOutput, req)
	if err != nil {
		return nil, fmt.Errorf("kokoro audio conversion failed: %w", err)
	}

	if len(audioData) == 0 {
		return nil, fmt.Errorf("kokoro synthesis failed to generate audio, please switch to another provider")
	}

	return &SynthesizeResponse{
		Audio:       io.NopCloser(bytes.NewReader(audioData)),
		Format:      FormatWAV,
		ContentType: "audio/wav",
	}, nil
}

// convertToAudio converts ONNX output to WAV audio
func (p *KokoroProvider) convertToAudio(output interface{}, req *SynthesizeRequest) ([]byte, error) {
	// Convert output tensor to float32 slice
	var audioSamples []float32

	switch v := output.(type) {
	case []float32:
		audioSamples = v
	case [][]float32:
		if len(v) > 0 {
			audioSamples = v[0]
		}
	default:
		return nil, fmt.Errorf("unexpected output type from Kokoro")
	}

	if len(audioSamples) == 0 {
		return nil, fmt.Errorf("no audio samples generated")
	}

	// Apply volume adjustment
	volume := req.Volume / 100.0
	if volume == 0 {
		volume = 1.0
	}
	for i := range audioSamples {
		audioSamples[i] *= volume
	}

	// Convert to PCM16 and create WAV
	return pcmToWav(audioSamples, 22050)
}

// SynthesizeStream synthesizes text with streaming audio output
func (p *KokoroProvider) SynthesizeStream(ctx context.Context, req *SynthesizeRequest, callback StreamCallback) error {
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
			return err
		}
	}
	return nil
}

// ListVoices returns available voices for all supported languages
func (p *KokoroProvider) ListVoices(ctx context.Context) ([]Voice, error) {
	voices := []Voice{
		{ID: "af_heart", Name: "af_heart", Language: "en-US", Gender: "neutral", Provider: "Kokoro", Quality: "high"},
		{ID: "af_heart", Name: "af_heart", Language: "en-GB", Gender: "neutral", Provider: "Kokoro", Quality: "high"},
		{ID: "af_heart", Name: "af_heart", Language: "ja-JP", Gender: "neutral", Provider: "Kokoro", Quality: "high"},
		{ID: "af_heart", Name: "af_heart", Language: "zh-CN", Gender: "neutral", Provider: "Kokoro", Quality: "high"},
		{ID: "af_heart", Name: "af_heart", Language: "es-ES", Gender: "neutral", Provider: "Kokoro", Quality: "high"},
		{ID: "af_heart", Name: "af_heart", Language: "fr-FR", Gender: "neutral", Provider: "Kokoro", Quality: "high"},
		{ID: "af_heart", Name: "af_heart", Language: "hi-IN", Gender: "neutral", Provider: "Kokoro", Quality: "high"},
		{ID: "af_heart", Name: "af_heart", Language: "it-IT", Gender: "neutral", Provider: "Kokoro", Quality: "high"},
		{ID: "af_heart", Name: "af_heart", Language: "pt-BR", Gender: "neutral", Provider: "Kokoro", Quality: "high"},
	}
	return voices, nil
}

// SupportedFormats returns supported audio formats
func (p *KokoroProvider) SupportedFormats() []AudioFormat {
	return []AudioFormat{FormatWAV, FormatPCM}
}

// MaxTextLength returns maximum text length
func (p *KokoroProvider) MaxTextLength() int {
	return 5000
}

// Name returns provider name
func (p *KokoroProvider) Name() string {
	return "Kokoro"
}

// Type returns provider type
func (p *KokoroProvider) Type() ProviderType {
	return "kokoro"
}

// Close cleans up resources
func (p *KokoroProvider) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.session != nil {
		p.session.Close()
		p.session = nil
	}
	p.initialized = false
}

// findKokoroModel searches for Kokoro ONNX model
func findKokoroModel(dataPath string) string {
	candidates := []string{
		filepath.Join(os.Getenv("HOME"), ".zimaos-blue", "data", "kokoro", "model_q8f16.onnx"),
		filepath.Join(os.Getenv("HOME"), ".zimaos-blue-dev", "data", "kokoro", "model_q8f16.onnx"),
	}

	if dataPath != "" {
		candidates = append(candidates,
			filepath.Join(dataPath, "kokoro", "model_q8f16.onnx"),
			filepath.Join(filepath.Dir(dataPath), "kokoro", "model_q8f16.onnx"),
		)
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}
