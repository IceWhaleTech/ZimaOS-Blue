package tts

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SherpaProvider implements the Provider interface using sherpa-onnx.
// It supports multiple TTS models: Kokoro, VITS, Piper, Matcha, etc.
// Note: Full TTS synthesis requires sherpa-onnx native library to be installed.
type SherpaProvider struct {
	modelDir      string
	modelType     string // "kokoro", "vits", "piper", "matcha"
	defaultVoice  string
	defaultFormat AudioFormat
	maxTextLength int
	modelReady    bool
	mu            sync.RWMutex
	downloadMgr   *SherpaDownloadManager
}

// SherpaConfig holds the configuration for the Sherpa TTS provider.
type SherpaConfig struct {
	ModelDir      string
	ModelType     string // "kokoro", "vits", "piper", "matcha"
	DefaultVoice  string
	DefaultFormat AudioFormat
	MaxTextLength int
}

// SherpaModelInfo contains information about a model file.
type SherpaModelInfo struct {
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Required bool   `json:"required"`
}

// SherpaDownloadProgress represents the current download progress.
type SherpaDownloadProgress struct {
	File       string    `json:"file"`
	Downloaded int64     `json:"downloaded"`
	Total      int64     `json:"total"`
	Percentage float64   `json:"percentage"`
	Speed      float64   `json:"speed"`
	SpeedHuman string    `json:"speed_human"`
	ETA        string    `json:"eta"`
	StartedAt  time.Time `json:"started_at"`
}

// Model download URLs
const (
	SherpaModelBaseURL = "https://github.com/k2-fsa/sherpa-onnx/releases/download/tts-models"
)

// Available model packages
var sherpaModelPackages = map[string]string{
	"kokoro-en":    "kokoro-en-v0_19.tar.bz2",
	"kokoro-multi": "kokoro-multi-lang-v1_0.tar.bz2",
	"piper-en":     "vits-piper-en_US-lessac-medium.tar.bz2",
	"vits-zh":      "vits-zh-aishell3.tar.bz2",
}

// NewSherpaProvider creates a new Sherpa TTS provider.
func NewSherpaProvider(cfg *SherpaConfig) *SherpaProvider {
	modelDir := cfg.ModelDir
	if modelDir == "" {
		home, _ := os.UserHomeDir()
		modelDir = filepath.Join(home, ".local", "share", "zimaos-echo", "sherpa-tts")
	}

	modelType := cfg.ModelType
	if modelType == "" {
		modelType = "kokoro"
	}

	defaultVoice := cfg.DefaultVoice
	if defaultVoice == "" {
		defaultVoice = "af" // American female for Kokoro
	}

	defaultFormat := cfg.DefaultFormat
	if defaultFormat == "" {
		defaultFormat = FormatWAV
	}

	maxTextLength := cfg.MaxTextLength
	if maxTextLength == 0 {
		maxTextLength = 5000
	}

	p := &SherpaProvider{
		modelDir:      modelDir,
		modelType:     modelType,
		defaultVoice:  defaultVoice,
		defaultFormat: defaultFormat,
		maxTextLength: maxTextLength,
	}

	p.downloadMgr = &SherpaDownloadManager{
		ModelDir: modelDir,
	}

	// Check if model is already downloaded and initialize
	p.modelReady = p.checkModelReady()
	if p.modelReady {
		p.initTTS()
	}

	return p
}

// Name returns the provider name.
func (p *SherpaProvider) Name() string {
	return fmt.Sprintf("Sherpa TTS (%s)", p.modelType)
}

// Type returns the provider type.
func (p *SherpaProvider) Type() ProviderType {
	return ProviderSherpa
}

// SupportedFormats returns the supported audio formats.
func (p *SherpaProvider) SupportedFormats() []AudioFormat {
	return []AudioFormat{FormatWAV, FormatMP3}
}

// MaxTextLength returns the maximum text length.
func (p *SherpaProvider) MaxTextLength() int {
	return p.maxTextLength
}

// IsModelReady returns true if the model is downloaded and ready.
func (p *SherpaProvider) IsModelReady() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.modelReady
}

// checkModelReady checks if required model files exist and download is complete.
func (p *SherpaProvider) checkModelReady() bool {
	modelPath := p.getModelPath()
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return false
	}
	// Also check for completion marker to ensure download was successful
	if p.downloadMgr != nil && !p.downloadMgr.IsModelComplete(p.modelType) {
		// Model directory exists but no completion marker - might be partial download
		// Check if it's an old installation (before completion markers were added)
		// by verifying essential files exist
		return p.verifyModelFiles()
	}
	return true
}

// verifyModelFiles checks if essential model files exist (for backward compatibility).
func (p *SherpaProvider) verifyModelFiles() bool {
	modelPath := p.getModelPath()
	// Check for common model files that should exist
	essentialFiles := []string{"model.onnx", "tokens.txt"}
	for _, file := range essentialFiles {
		if _, err := os.Stat(filepath.Join(modelPath, file)); os.IsNotExist(err) {
			return false
		}
	}
	// If essential files exist, create completion marker for future checks
	if p.downloadMgr != nil {
		p.downloadMgr.markComplete(p.modelType)
	}
	return true
}

// getModelPath returns the path to the model directory based on model type.
func (p *SherpaProvider) getModelPath() string {
	switch p.modelType {
	case "kokoro":
		return filepath.Join(p.modelDir, "kokoro-en-v0_19")
	case "kokoro-multi":
		return filepath.Join(p.modelDir, "kokoro-multi-lang-v1_0")
	case "piper":
		return filepath.Join(p.modelDir, "vits-piper-en_US-lessac-medium")
	case "vits-zh":
		return filepath.Join(p.modelDir, "vits-zh-aishell3")
	default:
		return filepath.Join(p.modelDir, "kokoro-en-v0_19")
	}
}

// initTTS initializes the sherpa-onnx TTS engine.
// Note: This is a placeholder. Full implementation requires sherpa-onnx native library.
func (p *SherpaProvider) initTTS() error {
	// Placeholder - sherpa-onnx native library integration would go here
	// For now, we just verify the model files exist
	return nil
}

// Close releases resources.
func (p *SherpaProvider) Close() {
	// Placeholder - cleanup would go here when using native library
}

// Synthesize synthesizes text to speech.
// Note: Full TTS synthesis requires sherpa-onnx native library to be installed.
func (p *SherpaProvider) Synthesize(ctx context.Context, req *SynthesizeRequest) (*SynthesizeResponse, error) {
	if !p.IsModelReady() {
		return nil, fmt.Errorf("model not downloaded. Please download the model first")
	}

	if len(req.Text) > p.maxTextLength {
		return nil, ErrTextTooLong
	}

	// TODO: Implement actual TTS synthesis using sherpa-onnx native library
	// For now, return an error indicating the feature is not yet available
	return nil, fmt.Errorf("TTS synthesis not yet implemented. Model files are ready at: %s", p.getModelPath())
}

// voiceToSpeakerID converts voice name to speaker ID.
func (p *SherpaProvider) voiceToSpeakerID(voice string) int {
	// Kokoro voice mapping
	voiceMap := map[string]int{
		"af":         0, // American female (default)
		"af_bella":   1,
		"af_sarah":   2,
		"am_adam":    3,
		"am_michael": 4,
		"bf_emma":    5,
		"bf_isabella": 6,
		"bm_george":  7,
		"bm_lewis":   8,
	}

	if sid, ok := voiceMap[voice]; ok {
		return sid
	}
	return 0 // Default to first voice
}

// samplesToWAV converts float32 samples to WAV format.
func samplesToWAV(samples []float32, sampleRate int) ([]byte, error) {
	buf := new(bytes.Buffer)

	// WAV header
	numSamples := len(samples)
	dataSize := numSamples * 2 // 16-bit samples
	fileSize := 36 + dataSize

	// RIFF header
	buf.WriteString("RIFF")
	binary.Write(buf, binary.LittleEndian, uint32(fileSize))
	buf.WriteString("WAVE")

	// fmt chunk
	buf.WriteString("fmt ")
	binary.Write(buf, binary.LittleEndian, uint32(16))        // chunk size
	binary.Write(buf, binary.LittleEndian, uint16(1))         // audio format (PCM)
	binary.Write(buf, binary.LittleEndian, uint16(1))         // num channels
	binary.Write(buf, binary.LittleEndian, uint32(sampleRate)) // sample rate
	binary.Write(buf, binary.LittleEndian, uint32(sampleRate*2)) // byte rate
	binary.Write(buf, binary.LittleEndian, uint16(2))         // block align
	binary.Write(buf, binary.LittleEndian, uint16(16))        // bits per sample

	// data chunk
	buf.WriteString("data")
	binary.Write(buf, binary.LittleEndian, uint32(dataSize))

	// Convert float32 samples to int16
	for _, sample := range samples {
		// Clamp to [-1, 1]
		if sample > 1.0 {
			sample = 1.0
		} else if sample < -1.0 {
			sample = -1.0
		}
		// Convert to int16
		intSample := int16(sample * 32767)
		binary.Write(buf, binary.LittleEndian, intSample)
	}

	return buf.Bytes(), nil
}

// SynthesizeStream synthesizes text with streaming audio output.
func (p *SherpaProvider) SynthesizeStream(ctx context.Context, req *SynthesizeRequest, callback StreamCallback) error {
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
func (p *SherpaProvider) ListVoices(ctx context.Context) ([]Voice, error) {
	switch p.modelType {
	case "kokoro", "kokoro-multi":
		return []Voice{
			{ID: "af", Name: "American Female", Language: "en-US", Gender: "female", Description: "Default American female voice"},
			{ID: "af_bella", Name: "Bella", Language: "en-US", Gender: "female", Description: "American female - Bella"},
			{ID: "af_sarah", Name: "Sarah", Language: "en-US", Gender: "female", Description: "American female - Sarah"},
			{ID: "am_adam", Name: "Adam", Language: "en-US", Gender: "male", Description: "American male - Adam"},
			{ID: "am_michael", Name: "Michael", Language: "en-US", Gender: "male", Description: "American male - Michael"},
			{ID: "bf_emma", Name: "Emma", Language: "en-GB", Gender: "female", Description: "British female - Emma"},
			{ID: "bf_isabella", Name: "Isabella", Language: "en-GB", Gender: "female", Description: "British female - Isabella"},
			{ID: "bm_george", Name: "George", Language: "en-GB", Gender: "male", Description: "British male - George"},
			{ID: "bm_lewis", Name: "Lewis", Language: "en-GB", Gender: "male", Description: "British male - Lewis"},
		}, nil
	case "piper":
		return []Voice{
			{ID: "lessac", Name: "Lessac", Language: "en-US", Gender: "neutral", Description: "English US Lessac voice"},
		}, nil
	case "vits-zh":
		return []Voice{
			{ID: "0", Name: "Speaker 0", Language: "zh-CN", Gender: "female", Description: "Chinese female voice"},
		}, nil
	default:
		return []Voice{}, nil
	}
}

// GetModelDir returns the model directory path.
func (p *SherpaProvider) GetModelDir() string {
	return p.modelDir
}

// GetDownloadManager returns the download manager.
func (p *SherpaProvider) GetDownloadManager() *SherpaDownloadManager {
	return p.downloadMgr
}

// SwitchModel switches to a different TTS model.
func (p *SherpaProvider) SwitchModel(modelType string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Map model type to internal type
	internalType := modelType
	switch modelType {
	case "kokoro-en":
		internalType = "kokoro"
	case "piper-en":
		internalType = "piper"
	}

	// Check if the model is downloaded (either by marker or by verifying files)
	oldModelType := p.modelType
	p.modelType = internalType

	// Check if model files exist
	p.mu.Unlock()
	ready := p.checkModelReady()
	p.mu.Lock()

	if !ready {
		// Restore old model type
		p.modelType = oldModelType
		return fmt.Errorf("model %s is not downloaded", modelType)
	}

	p.modelReady = ready

	// Re-initialize TTS with new model
	p.mu.Unlock()
	err := p.initTTS()
	p.mu.Lock()

	return err
}
