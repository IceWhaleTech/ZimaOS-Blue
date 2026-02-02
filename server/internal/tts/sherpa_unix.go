//go:build !windows

package tts

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SherpaProvider implements the Provider interface using sherpa-onnx via purego.
// This is a stub implementation for non-Windows platforms.
type SherpaProvider struct {
	modelDir      string
	modelType     string
	defaultVoice  string
	defaultFormat AudioFormat
	maxTextLength int
	modelReady    bool
	mu            sync.RWMutex
	downloadMgr   *SherpaDownloadManager
	sampleRate    int
	initialized   bool
}

// SherpaConfig holds the configuration for the Sherpa TTS provider.
type SherpaConfig struct {
	ModelDir      string
	ModelType     string
	DefaultVoice  string
	DefaultFormat AudioFormat
	MaxTextLength int
}

type SherpaModelInfo struct {
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Required bool   `json:"required"`
}

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

const SherpaModelBaseURL = "https://github.com/k2-fsa/sherpa-onnx/releases/download/tts-models"

var sherpaModelPackages = map[string]string{
	"piper-en":     "vits-piper-en_US-lessac-medium.tar.bz2",
	"piper-en-hfc": "vits-piper-en_US-hfc_female-medium.tar.bz2",
	"piper-de":     "vits-piper-de_DE-thorsten-medium.tar.bz2",
	"piper-es":     "vits-piper-es_ES-davefx-medium.tar.bz2",
}

func NewSherpaProvider(cfg *SherpaConfig) *SherpaProvider {
	modelDir := cfg.ModelDir
	if modelDir == "" {
		home, _ := os.UserHomeDir()
		modelDir = filepath.Join(home, ".local", "share", "zimaos-echo", "sherpa-tts")
	}
	modelType := cfg.ModelType
	if modelType == "" {
		modelType = "piper-en"
	}
	defaultVoice := cfg.DefaultVoice
	if defaultVoice == "" {
		defaultVoice = "0"
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
		sampleRate:    22050,
		modelReady:    false,
	}
	p.downloadMgr = &SherpaDownloadManager{ModelDir: modelDir}
	return p
}

func (p *SherpaProvider) Name() string {
	return fmt.Sprintf("Sherpa TTS (%s) - Not supported on this platform", p.modelType)
}

func (p *SherpaProvider) Type() ProviderType {
	return ProviderSherpa
}

func (p *SherpaProvider) SupportedFormats() []AudioFormat {
	return []AudioFormat{}
}

func (p *SherpaProvider) MaxTextLength() int {
	return p.maxTextLength
}

func (p *SherpaProvider) GetModelDir() string {
	return p.modelDir
}

func (p *SherpaProvider) GetDownloadManager() *SherpaDownloadManager {
	return p.downloadMgr
}

func (p *SherpaProvider) IsModelReady() bool {
	return false
}

func (p *SherpaProvider) RefreshModelStatus() {
	ready := p.checkModelReady()
	p.mu.Lock()
	p.modelReady = ready
	p.mu.Unlock()
}

func (p *SherpaProvider) checkModelReady() bool {
	modelPath := p.getModelPath()
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return false
	}
	return p.verifyModelFiles()
}

func (p *SherpaProvider) verifyModelFiles() bool {
	modelPath := p.getModelPath()

	// Piper models have different naming: {voice}.onnx instead of model.onnx
	modelFiles := map[string]string{
		"piper-en":     "en_US-lessac-medium.onnx",
		"piper-en-hfc": "en_US-hfc_female-medium.onnx",
		"piper-de":     "de_DE-thorsten-medium.onnx",
		"piper-es":     "es_ES-davefx-medium.onnx",
	}

	onnxFile := "model.onnx"
	if f, ok := modelFiles[p.modelType]; ok {
		onnxFile = f
	}

	requiredFiles := []string{onnxFile, "tokens.txt"}
	for _, file := range requiredFiles {
		filePath := filepath.Join(modelPath, file)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return false
		}
	}
	if p.downloadMgr != nil {
		p.downloadMgr.markComplete(p.modelType)
	}
	return true
}

func (p *SherpaProvider) getModelPath() string {
	modelDirs := map[string]string{
		"piper":        "vits-piper-en_US-lessac-medium",
		"piper-en":     "vits-piper-en_US-lessac-medium",
		"piper-en-hfc": "vits-piper-en_US-hfc_female-medium",
		"piper-de":     "vits-piper-de_DE-thorsten-medium",
		"piper-es":     "vits-piper-es_ES-davefx-medium",
	}
	if dir, ok := modelDirs[p.modelType]; ok {
		return filepath.Join(p.modelDir, dir)
	}
	return filepath.Join(p.modelDir, "vits-piper-en_US-lessac-medium")
}

func (p *SherpaProvider) initTTS() error {
	return fmt.Errorf("Sherpa TTS is only supported on Windows")
}

func (p *SherpaProvider) Close() {
	// No-op on non-Windows platforms
}

func (p *SherpaProvider) Synthesize(ctx context.Context, req *SynthesizeRequest) (*SynthesizeResponse, error) {
	return nil, fmt.Errorf("Sherpa TTS is only supported on Windows")
}

func (p *SherpaProvider) SynthesizeStream(ctx context.Context, req *SynthesizeRequest, cb StreamCallback) error {
	return fmt.Errorf("Sherpa TTS is only supported on Windows")
}

func (p *SherpaProvider) ListVoices(ctx context.Context) ([]Voice, error) {
	return nil, fmt.Errorf("Sherpa TTS is only supported on Windows")
}

func (p *SherpaProvider) SwitchModel(modelType string) error {
	return fmt.Errorf("Sherpa TTS is only supported on Windows")
}

type TTSModelInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Languages   []string `json:"languages"`
	Size        string   `json:"size"`
	Downloaded  bool     `json:"downloaded"`
	Active      bool     `json:"active"`
}

func (p *SherpaProvider) ListModels() []TTSModelInfo {
	// Map internal model type to external type for comparison
	currentModel := p.modelType
	if currentModel == "kokoro" {
		currentModel = "kokoro-en"
	} else if currentModel == "piper" {
		currentModel = "piper-en"
	}

	models := []TTSModelInfo{
		{ID: "piper-en", Name: "Piper English (Lessac)", Description: "Fast English TTS - Lessac voice (Windows only)", Languages: []string{"en-US"}, Size: "~60MB"},
		{ID: "piper-en-hfc", Name: "Piper English (HFC Female)", Description: "Fast English TTS - HFC Female voice (Windows only)", Languages: []string{"en-US"}, Size: "~75MB"},
		{ID: "piper-de", Name: "Piper German (Thorsten)", Description: "German TTS - Thorsten voice (Windows only)", Languages: []string{"de-DE"}, Size: "~75MB"},
		{ID: "piper-es", Name: "Piper Spanish (Davefx)", Description: "Spanish TTS - Davefx voice (Windows only)", Languages: []string{"es-ES"}, Size: "~75MB"},
	}

	// Model directory mappings
	modelDirs := map[string]string{
		"piper-en":     "vits-piper-en_US-lessac-medium",
		"piper-en-hfc": "vits-piper-en_US-hfc_female-medium",
		"piper-de":     "vits-piper-de_DE-thorsten-medium",
		"piper-es":     "vits-piper-es_ES-davefx-medium",
	}

	for i, m := range models {
		if dir, ok := modelDirs[m.ID]; ok {
			path := filepath.Join(p.modelDir, dir)
			if _, err := os.Stat(path); err == nil {
				models[i].Downloaded = true
			}
		}
		// Mark active model
		if m.ID == currentModel {
			models[i].Active = true
		}
	}
	return models
}

func (p *SherpaProvider) DeleteModel(modelType string) error {
	// Get the correct directory for this model type
	modelDirs := map[string]string{
		"piper-en":     "vits-piper-en_US-lessac-medium",
		"piper-en-hfc": "vits-piper-en_US-hfc_female-medium",
		"piper-de":     "vits-piper-de_DE-thorsten-medium",
		"piper-es":     "vits-piper-es_ES-davefx-medium",
	}

	dir, ok := modelDirs[modelType]
	if !ok {
		return fmt.Errorf("unknown model type: %s", modelType)
	}

	// Delete model directory
	modelPath := filepath.Join(p.modelDir, dir)
	if err := os.RemoveAll(modelPath); err != nil {
		return err
	}

	// Delete completion marker
	markerPath := filepath.Join(p.modelDir, fmt.Sprintf(".%s.complete", modelType))
	os.Remove(markerPath)

	// Update state if this was the active model
	p.mu.Lock()
	if p.modelType == modelType {
		p.modelReady = false
		p.initialized = false
	}
	p.mu.Unlock()

	return nil
}
