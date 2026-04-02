//go:build kokoro && !windows

package tts

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/onnx"
	ort "github.com/yalue/onnxruntime_go"
)

// kokoroIdleTimeout is how long the Kokoro model stays resident after the last use.
// After this duration with no synthesis calls, the ONNX session, voice data, and
// G2P dictionaries are unloaded to reclaim ~150MB of memory.
const kokoroIdleTimeout = 10 * time.Minute

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
	dataPath    string
	initialized bool
	initStage   string // current initialization stage for progress reporting
	mu          sync.Mutex
	session     *onnx.DynamicSession
	voices      map[string][]float32 // lang -> voice style embeddings, loaded lazily per-language
	langMap     map[string]bool      // supported BCP-47 language codes
	g2p         *G2PDispatcher
	tokenizer   *KokoroTokenizer
	idleTimer   *time.Timer // fires after kokoroIdleTimeout to unload model
}

// langVoiceMap maps BCP-47 language codes to Kokoro voice file names (female voices).
var langVoiceMap = map[string]string{
	"en-US": "af_heart",
	"en-GB": "bf_emma",
	"ja-JP": "jf_alpha",
	"zh-CN": "zf_xiaobei",
	"es-ES": "ef_dora",
	"fr-FR": "ff_siwis",
	"hi-IN": "hf_alpha",
	"it-IT": "if_sara",
	"pt-BR": "pf_dora",
}

// NewKokoroProvider creates a new Kokoro provider
func NewKokoroProvider(dataPath string) *KokoroProvider {
	p := &KokoroProvider{
		modelPath: findKokoroModel(dataPath),
		dataPath:  dataPath,
		tokenizer: NewKokoroTokenizer(),
		g2p:       NewG2PDispatcher(),
		langMap: map[string]bool{
			"en-US": true, "en-GB": true, "ja-JP": true, "zh-CN": true,
			"es-ES": true, "fr-FR": true, "hi-IN": true, "it-IT": true, "pt-BR": true,
		},
	}

	// Pre-warm G2P dictionary and ONNX model in background
	go func() {
		p.mu.Lock()
		p.initStage = "loading_dictionary"
		p.mu.Unlock()
		p.g2p.Warmup()

		p.mu.Lock()
		p.initStage = "loading_model"
		p.mu.Unlock()
		_ = p.Initialize()
	}()

	return p
}

// touchActivity resets the idle unload timer. Must be called on every synthesis.
func (p *KokoroProvider) touchActivity() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.idleTimer != nil {
		p.idleTimer.Reset(kokoroIdleTimeout)
	} else {
		p.idleTimer = time.AfterFunc(kokoroIdleTimeout, p.unloadModel)
	}
}

// unloadModel releases the ONNX session, voice data, and G2P dictionaries
// to reclaim memory after a period of inactivity.
func (p *KokoroProvider) unloadModel() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.initialized {
		return
	}
	slog.Info("[kokoro] idle timeout, unloading model to reclaim memory")
	if p.session != nil {
		p.session.Close()
		p.session = nil
	}
	p.voices = nil
	p.g2p = nil
	p.initialized = false
	p.initStage = ""
}

// SupportedLanguages returns list of languages Kokoro can handle
func (p *KokoroProvider) SupportedLanguages() []string {
	return []string{
		"en-US", "en-GB", "ja-JP", "zh-CN", "es-ES", "fr-FR", "hi-IN", "it-IT", "pt-BR",
	}
}

// CanHandle checks if this provider can handle the language
func (p *KokoroProvider) CanHandle(language string) bool {
	return p.langMap[language]
}

// Initialize initializes the Kokoro provider
func (p *KokoroProvider) Initialize() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.initialized {
		return nil
	}

	if p.modelPath == "" {
		p.initStage = "error"
		return fmt.Errorf("kokoro model not found at expected locations")
	}

	// Stage: loading ONNX runtime
	p.initStage = "loading_runtime"

	// Tell ONNX runtime where to find the shared library
	onnx.SetDataDir(p.dataPath)
	if libPath := onnx.RuntimeLibPath(p.dataPath); libPath != "" {
		onnx.SetLibraryPath(libPath)
	} else {
		// Auto-download ONNX Runtime if not present
		if libPath, err := onnx.EnsureRuntime(p.dataPath); err == nil {
			onnx.SetLibraryPath(libPath)
		}
	}

	// Voices are loaded lazily per-language in loadVoice() to save memory
	if p.voices == nil {
		p.voices = make(map[string][]float32)
	}

	// Re-create G2P dispatcher if it was unloaded by idle timeout
	if p.g2p == nil {
		p.g2p = NewG2PDispatcher()
		p.g2p.Warmup()
	}

	// Load ONNX model — inputs: input_ids[1,seq], style[1,256], speed[1]
	p.initStage = "loading_model"
	session, err := onnx.NewDynamicSession(
		p.modelPath,
		[]string{"input_ids", "style", "speed"},
		[]string{"waveform"},
	)
	if err != nil {
		return fmt.Errorf("failed to load Kokoro ONNX model: %w", err)
	}

	p.session = session
	p.initialized = true
	p.initStage = "ready"

	// Start idle timer
	if p.idleTimer != nil {
		p.idleTimer.Reset(kokoroIdleTimeout)
	} else {
		p.idleTimer = time.AfterFunc(kokoroIdleTimeout, p.unloadModel)
	}

	return nil
}

// loadVoice lazily loads a single voice file for the given language.
// Must be called with p.mu held.
func (p *KokoroProvider) loadVoice(lang string) {
	if _, ok := p.voices[lang]; ok {
		return // already loaded
	}
	voiceName, ok := langVoiceMap[lang]
	if !ok {
		return
	}
	voicesDir := filepath.Join(filepath.Dir(p.modelPath), "voices")
	voicePath := filepath.Join(voicesDir, voiceName+".bin")
	voiceRaw, err := os.ReadFile(voicePath)
	if err != nil {
		return
	}
	data := make([]float32, len(voiceRaw)/4)
	for i := range data {
		bits := uint32(voiceRaw[i*4]) | uint32(voiceRaw[i*4+1])<<8 | uint32(voiceRaw[i*4+2])<<16 | uint32(voiceRaw[i*4+3])<<24
		data[i] = math.Float32frombits(bits)
	}
	p.voices[lang] = data
}

// kokoroMaxContentTokens is the maximum number of content tokens per chunk.
// Voice data has 510 rows of style vectors, so content tokens must be ≤ 510.
// Total sequence = content + 2 PAD tokens ≤ 512.
const kokoroMaxContentTokens = 510

// Synthesize generates speech from text using Kokoro
func (p *KokoroProvider) Synthesize(ctx context.Context, req *SynthesizeRequest) (*SynthesizeResponse, error) {
	if err := p.Initialize(); err != nil {
		return nil, err
	}
	p.touchActivity()

	if req.Text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	// Resolve language: Voice field (set from config) > auto-detect from text
	lang := req.Voice
	if lang == "" {
		lang = detectKokoroLanguage(req.Text)
	}

	// Check if language is supported
	if !p.CanHandle(lang) {
		return nil, fmt.Errorf("language %s not supported by Kokoro, please switch to another provider", lang)
	}

	speedVal := req.Speed
	if speedVal <= 0 {
		speedVal = 1.0
	}
	volume := req.Volume / 100.0
	if volume == 0 {
		volume = 1.0
	}

	// Phase 1: G2P + tokenization (no lock needed — pure computation)
	chunks := p.prepareChunks(req.Text, lang)
	if len(chunks) == 0 {
		return nil, fmt.Errorf("kokoro synthesis failed to generate audio, please switch to another provider")
	}

	// Phase 2: ONNX inference (needs lock)
	p.mu.Lock()
	defer p.mu.Unlock()

	var allSamples []float32
	for _, tokens := range chunks {
		samples, err := p.synthesizeTokens(tokens, speedVal, lang)
		if err != nil {
			return nil, err
		}
		allSamples = append(allSamples, samples...)
	}

	if len(allSamples) == 0 {
		return nil, fmt.Errorf("kokoro synthesis failed to generate audio, please switch to another provider")
	}

	for i := range allSamples {
		allSamples[i] *= volume
	}

	wavData := float32ToWav(allSamples, 24000)
	return &SynthesizeResponse{
		Audio:       io.NopCloser(bytes.NewReader(wavData)),
		Format:      FormatWAV,
		ContentType: "audio/wav",
	}, nil
}

// prepareChunks runs G2P and tokenization outside the mutex.
// Returns a list of token sequences, each ready for ONNX inference.
func (p *KokoroProvider) prepareChunks(text, lang string) [][]int64 {
	text = normalizeKokoroStructuredText(text, lang)
	if text == "" {
		return nil
	}

	sentences := splitSentences(text)
	var allChunks [][]int64

	// Phonemize each sentence independently, then merge tokens that fit
	var pendingTokens []int64

	flush := func() {
		if len(pendingTokens) <= 2 {
			return
		}
		contentLen := len(pendingTokens) - 2
		if contentLen <= kokoroMaxContentTokens {
			allChunks = append(allChunks, pendingTokens)
		} else {
			allChunks = append(allChunks, splitPhonemeChunks(pendingTokens, kokoroMaxContentTokens)...)
		}
		pendingTokens = nil
	}

	for _, sent := range sentences {
		sent = strings.TrimSpace(sent)
		if sent == "" {
			continue
		}

		phonemes := p.g2p.Phonemize(sent, lang)
		tokens := p.tokenizer.Tokenize(phonemes)
		contentLen := len(tokens) - 2
		if contentLen <= 0 {
			continue
		}

		if len(pendingTokens) == 0 {
			// First sentence
			pendingTokens = tokens
			continue
		}

		// Try merging: strip PAD from both, concatenate with space, re-wrap
		prevContent := pendingTokens[1 : len(pendingTokens)-1]
		newContent := tokens[1 : len(tokens)-1]
		mergedLen := len(prevContent) + 1 + len(newContent) // +1 for space token

		if mergedLen <= kokoroMaxContentTokens {
			// Merge tokens directly (avoid re-phonemizing)
			merged := make([]int64, 0, mergedLen+2)
			merged = append(merged, 0) // PAD
			merged = append(merged, prevContent...)
			merged = append(merged, 16) // space token
			merged = append(merged, newContent...)
			merged = append(merged, 0) // PAD
			pendingTokens = merged
		} else {
			// Doesn't fit — flush pending, start new
			flush()
			pendingTokens = tokens
		}
	}
	flush()

	return allChunks
}

// synthesizeTokens runs ONNX inference on a single token sequence (already wrapped with PAD).
// Must be called with p.mu held.
func (p *KokoroProvider) synthesizeTokens(tokens []int64, speed float32, lang string) ([]float32, error) {
	// Lazy-load voice data for the requested language
	p.loadVoice(lang)
	p.loadVoice("en-US") // fallback voice

	// Select voice data for the language, fall back to en-US
	voiceData := p.voices[lang]
	if voiceData == nil {
		voiceData = p.voices["en-US"]
	}
	if voiceData == nil {
		// Use whatever voice is available
		for _, v := range p.voices {
			voiceData = v
			break
		}
	}

	if voiceData == nil {
		return nil, fmt.Errorf("no voice data available")
	}

	// Get style vector from voice data: voice[len(tokens)] -> 256 floats
	styleIdx := len(tokens)
	maxIdx := len(voiceData) / 256
	if styleIdx >= maxIdx {
		styleIdx = maxIdx - 1
	}
	style := voiceData[styleIdx*256 : (styleIdx+1)*256]

	inputIDsTensor, err := ort.NewTensor(ort.NewShape(1, int64(len(tokens))), tokens)
	if err != nil {
		return nil, fmt.Errorf("kokoro create input_ids tensor: %w", err)
	}
	defer inputIDsTensor.Destroy()

	styleTensor, err := ort.NewTensor(ort.NewShape(1, 256), style)
	if err != nil {
		return nil, fmt.Errorf("kokoro create style tensor: %w", err)
	}
	defer styleTensor.Destroy()

	speedData := []float32{speed}
	speedTensor, err := ort.NewTensor(ort.NewShape(1), speedData)
	if err != nil {
		return nil, fmt.Errorf("kokoro create speed tensor: %w", err)
	}
	defer speedTensor.Destroy()

	outputValues := []ort.Value{nil}
	err = p.session.Run([]ort.Value{inputIDsTensor, styleTensor, speedTensor}, outputValues)
	if err != nil {
		return nil, fmt.Errorf("kokoro inference failed: %w", err)
	}
	if outputValues[0] != nil {
		defer outputValues[0].Destroy()
	}

	return p.extractAudioSamples(outputValues[0])
}

// extractAudioSamples extracts float32 audio data from an ONNX output Value.
func (p *KokoroProvider) extractAudioSamples(v ort.Value) ([]float32, error) {
	if v == nil {
		return nil, fmt.Errorf("nil output value")
	}
	tensor, ok := v.(*ort.Tensor[float32])
	if !ok {
		return nil, fmt.Errorf("unexpected output tensor type: %T", v)
	}
	return tensor.GetData(), nil
}

// SynthesizeStream synthesizes text with true sentence-level streaming.
// Each chunk is phonemized, inferred, and streamed immediately — the client
// receives audio for the first sentence while later sentences are still being processed.
func (p *KokoroProvider) SynthesizeStream(ctx context.Context, req *SynthesizeRequest, callback StreamCallback) error {
	if err := p.Initialize(); err != nil {
		return err
	}
	p.touchActivity()

	if req.Text == "" {
		return fmt.Errorf("text cannot be empty")
	}

	lang := req.Voice
	if lang == "" {
		lang = detectKokoroLanguage(req.Text)
	}
	if !p.CanHandle(lang) {
		return fmt.Errorf("language %s not supported by Kokoro, please switch to another provider", lang)
	}

	speedVal := req.Speed
	if speedVal <= 0 {
		speedVal = 1.0
	}
	volume := req.Volume / 100.0
	if volume == 0 {
		volume = 1.0
	}

	// Prepare all chunks outside the lock (G2P is pure computation)
	chunks := p.prepareChunks(req.Text, lang)
	if len(chunks) == 0 {
		return fmt.Errorf("kokoro synthesis failed to generate audio")
	}

	// Stream each chunk: infer → WAV → callback, one at a time
	for _, tokens := range chunks {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		p.mu.Lock()
		samples, err := p.synthesizeTokens(tokens, speedVal, lang)
		p.mu.Unlock()
		if err != nil {
			return err
		}

		if volume != 1.0 {
			for i := range samples {
				samples[i] *= volume
			}
		}

		wavData := float32ToWav(samples, 24000)
		if err := callback(wavData); err != nil {
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
	if p.idleTimer != nil {
		p.idleTimer.Stop()
		p.idleTimer = nil
	}
	if p.session != nil {
		p.session.Close()
		p.session = nil
	}
	p.voices = nil
	p.g2p = nil
	p.initialized = false
}

// findKokoroModel searches for Kokoro ONNX model
func findKokoroModel(dataPath string) string {
	candidates := []string{
		filepath.Join(os.Getenv("HOME"), ".zimaos-blue", "data", "kokoro", "model_quantized.onnx"),
		filepath.Join(os.Getenv("HOME"), ".zimaos-blue-dev", "data", "kokoro", "model_quantized.onnx"),
	}

	if dataPath != "" {
		candidates = append(candidates,
			filepath.Join(dataPath, "kokoro", "model_quantized.onnx"),
			filepath.Join(filepath.Dir(dataPath), "kokoro", "model_quantized.onnx"),
		)
	}

	// Also check legacy filename for backward compatibility
	legacyCandidates := make([]string, len(candidates))
	for i, c := range candidates {
		legacyCandidates[i] = strings.Replace(c, "model_quantized.onnx", "model_q8f16.onnx", 1)
	}
	candidates = append(candidates, legacyCandidates...)

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// KokoroAvailable reports whether Kokoro was compiled in.
func KokoroAvailable() bool { return true }

// GetInitStage returns the current initialization stage.
// Possible values: "", "loading_dictionary", "loading_runtime", "loading_voice", "loading_model", "ready", "error"
func (p *KokoroProvider) GetInitStage() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.initStage
}

// detectKokoroLanguage auto-detects language from text and returns a Kokoro-compatible BCP-47 code.
func detectKokoroLanguage(text string) string {
	lang, _ := DetectLanguage(text)
	switch lang {
	case "cmn", "zh":
		return "zh-CN"
	case "ja":
		return "ja-JP"
	case "ko":
		// Korean not supported by Kokoro, fall back to en-US
		return "en-US"
	default:
		return "en-US"
	}
}

// splitSentences splits text into segments for Kokoro synthesis.
// Matches the reference KPipeline behavior:
// 1. Split on newlines first (split_pattern=r'\n+')
// 2. For non-English text, split on sentence-ending punctuation [.!?。！？]
// 3. Merge short sentences into chunks of ~400 characters
// Commas and other clause-level punctuation are NOT split points — they flow
// through as tokens to give the model prosody cues for natural pauses.
func splitSentences(text string) []string {
	// Step 1: Split on newlines (matching Kokoro's split_pattern=r'\n+')
	lines := strings.FieldsFunc(text, func(r rune) bool { return r == '\n' })

	var result []string
	const chunkSize = 400

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Step 2: Split on sentence-ending punctuation
		sentences := splitOnSentenceEnd(line)

		// Step 3: Merge short sentences into chunks of ~chunkSize characters
		var current strings.Builder
		for _, sent := range sentences {
			sent = strings.TrimSpace(sent)
			if sent == "" {
				continue
			}
			if current.Len()+len(sent) <= chunkSize {
				current.WriteString(sent)
			} else {
				if current.Len() > 0 {
					result = append(result, current.String())
					current.Reset()
				}
				current.WriteString(sent)
			}
		}
		if current.Len() > 0 {
			result = append(result, current.String())
		}
	}

	// If nothing was split, return the original text as a single segment
	if len(result) == 0 && strings.TrimSpace(text) != "" {
		result = append(result, strings.TrimSpace(text))
	}
	return result
}

// splitOnSentenceEnd splits text at sentence-ending punctuation (.!?。！？),
// keeping the punctuation attached to the preceding sentence.
func splitOnSentenceEnd(text string) []string {
	var sentences []string
	var cur strings.Builder

	for _, r := range text {
		cur.WriteRune(r)
		if r == '.' || r == '!' || r == '?' || r == '。' || r == '！' || r == '？' {
			s := strings.TrimSpace(cur.String())
			if s != "" {
				sentences = append(sentences, s)
			}
			cur.Reset()
		}
	}
	if s := strings.TrimSpace(cur.String()); s != "" {
		sentences = append(sentences, s)
	}
	return sentences
}

// splitPhonemeChunks splits a token sequence (with PAD) into chunks at space token boundaries.
// Each returned chunk is wrapped with PAD tokens and fits within maxContent content tokens.
func splitPhonemeChunks(tokens []int64, maxContent int) [][]int64 {
	content := tokens[1 : len(tokens)-1] // strip PAD
	const spaceToken int64 = 16          // space in Kokoro vocab

	var chunks [][]int64
	start := 0

	for start < len(content) {
		end := start + maxContent
		if end >= len(content) {
			end = len(content)
		} else {
			// Search backward for a space token to split at a word boundary
			best := -1
			for i := end - 1; i > start; i-- {
				if content[i] == spaceToken {
					best = i + 1 // include the space in current chunk
					break
				}
			}
			if best > start {
				end = best
			}
		}

		chunk := make([]int64, 0, end-start+2)
		chunk = append(chunk, 0) // PAD start
		chunk = append(chunk, content[start:end]...)
		chunk = append(chunk, 0) // PAD end
		chunks = append(chunks, chunk)
		start = end
	}

	return chunks
}
