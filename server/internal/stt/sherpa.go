package stt

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SherpaProvider implements the Provider interface using sherpa-onnx ASR.
// It supports multiple ASR models: Whisper, Zipformer, Paraformer, SenseVoice, etc.
type SherpaProvider struct {
	modelDir      string
	modelType     string // "whisper-tiny", "whisper-base", "zipformer-en", "paraformer-zh", "sensevoice-small"
	defaultLang   string
	maxDuration   time.Duration
	modelReady    bool
	streaming     bool // Whether model supports streaming
	mu            sync.RWMutex
	downloadMgr   *SherpaASRDownloadManager
}

// SherpaConfig holds the configuration for the Sherpa ASR provider.
type SherpaConfig struct {
	ModelDir    string
	ModelType   string // "whisper-tiny", "whisper-base", "zipformer-en", "paraformer-zh", "sensevoice-small"
	DefaultLang string
	MaxDuration time.Duration
}

// SherpaASRModelInfo contains information about an ASR model.
type SherpaASRModelInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Languages   []string `json:"languages"`
	Size        string   `json:"size"`
	Streaming   bool     `json:"streaming"`
	Downloaded  bool     `json:"downloaded"`
}

// Model download URLs
const (
	SherpaASRModelBaseURL = "https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models"
)

// Available ASR model packages
var sherpaASRModelPackages = map[string]string{
	"whisper-tiny":     "sherpa-onnx-whisper-tiny.tar.bz2",
	"whisper-base":     "sherpa-onnx-whisper-base.tar.bz2",
	"zipformer-en":     "sherpa-onnx-streaming-zipformer-en-2023-06-26.tar.bz2",
	"sensevoice-small": "sherpa-onnx-sense-voice-zh-en-ja-ko-yue-2024-07-17.tar.bz2",
}

// Model metadata - using i18n keys for name and description
var sherpaASRModelMeta = map[string]SherpaASRModelInfo{
	"whisper-tiny": {
		ID:          "whisper-tiny",
		Name:        "speech.asrModels.whisperTiny.name",
		Description: "speech.asrModels.whisperTiny.description",
		Languages:   []string{"en", "zh", "ja", "ko", "de", "fr", "es", "it", "pt", "ru"},
		Size:        "~75MB",
		Streaming:   false,
	},
	"whisper-base": {
		ID:          "whisper-base",
		Name:        "speech.asrModels.whisperBase.name",
		Description: "speech.asrModels.whisperBase.description",
		Languages:   []string{"en", "zh", "ja", "ko", "de", "fr", "es", "it", "pt", "ru"},
		Size:        "~150MB",
		Streaming:   false,
	},
	"zipformer-en": {
		ID:          "zipformer-en",
		Name:        "speech.asrModels.zipformerEn.name",
		Description: "speech.asrModels.zipformerEn.description",
		Languages:   []string{"en"},
		Size:        "~50MB",
		Streaming:   true,
	},
	"sensevoice-small": {
		ID:          "sensevoice-small",
		Name:        "speech.asrModels.sensevoiceSmall.name",
		Description: "speech.asrModels.sensevoiceSmall.description",
		Languages:   []string{"zh", "en", "ja", "ko", "yue"},
		Size:        "~100MB",
		Streaming:   false,
	},
}

// NewSherpaProvider creates a new Sherpa ASR provider.
func NewSherpaProvider(cfg *SherpaConfig) *SherpaProvider {
	modelDir := cfg.ModelDir
	if modelDir == "" {
		home, _ := os.UserHomeDir()
		modelDir = filepath.Join(home, ".local", "share", "zimaos-echo", "sherpa-asr")
	}

	modelType := cfg.ModelType
	if modelType == "" {
		modelType = "whisper-tiny"
	}

	defaultLang := cfg.DefaultLang
	if defaultLang == "" {
		defaultLang = "zh"
	}

	maxDuration := cfg.MaxDuration
	if maxDuration == 0 {
		maxDuration = 5 * time.Minute
	}

	// Check if model supports streaming
	streaming := false
	if meta, ok := sherpaASRModelMeta[modelType]; ok {
		streaming = meta.Streaming
	}

	p := &SherpaProvider{
		modelDir:    modelDir,
		modelType:   modelType,
		defaultLang: defaultLang,
		maxDuration: maxDuration,
		streaming:   streaming,
	}

	p.downloadMgr = &SherpaASRDownloadManager{
		ModelDir: modelDir,
	}

	// Check if model is already downloaded
	p.modelReady = p.checkModelReady()
	if p.modelReady {
		p.initASR()
	}

	return p
}

// Name returns the provider name.
func (p *SherpaProvider) Name() string {
	return fmt.Sprintf("Sherpa ASR (%s)", p.modelType)
}

// Type returns the provider type.
func (p *SherpaProvider) Type() ProviderType {
	return ProviderSherpa
}

// SupportedFormats returns the supported audio formats.
func (p *SherpaProvider) SupportedFormats() []AudioFormat {
	return []AudioFormat{FormatWAV, FormatPCM, FormatMP3, FormatOGG, FormatFLAC}
}

// MaxDuration returns the maximum audio duration.
func (p *SherpaProvider) MaxDuration() time.Duration {
	return p.maxDuration
}

// IsModelReady returns true if the model is downloaded and ready.
func (p *SherpaProvider) IsModelReady() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.modelReady
}

// checkModelReady checks if required model files exist.
func (p *SherpaProvider) checkModelReady() bool {
	modelPath := p.getModelPath()
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return false
	}
	// Check for completion marker
	if p.downloadMgr != nil && !p.downloadMgr.IsModelComplete(p.modelType) {
		return p.verifyModelFiles()
	}
	return true
}

// verifyModelFiles checks if essential model files exist.
// Files with .tmp suffix are considered incomplete.
func (p *SherpaProvider) verifyModelFiles() bool {
	modelPath := p.getModelPath()
	// Check for common model files
	essentialFiles := []string{"encoder.onnx", "decoder.onnx"}
	// Whisper models have different structure
	if p.modelType == "whisper-tiny" || p.modelType == "whisper-base" {
		essentialFiles = []string{"tiny-encoder.onnx", "tiny-decoder.onnx"}
		if p.modelType == "whisper-base" {
			essentialFiles = []string{"base-encoder.onnx", "base-decoder.onnx"}
		}
	}

	for _, file := range essentialFiles {
		filePath := filepath.Join(modelPath, file)
		// Check if file exists
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return false
		}
		// Check if .tmp version exists (incomplete download)
		tmpPath := filePath + ".tmp"
		if _, err := os.Stat(tmpPath); err == nil {
			return false
		}
	}
	// Mark as complete for future checks
	if p.downloadMgr != nil {
		p.downloadMgr.markComplete(p.modelType)
	}
	return true
}

// getModelPath returns the path to the model directory.
func (p *SherpaProvider) getModelPath() string {
	switch p.modelType {
	case "whisper-tiny":
		return filepath.Join(p.modelDir, "sherpa-onnx-whisper-tiny")
	case "whisper-base":
		return filepath.Join(p.modelDir, "sherpa-onnx-whisper-base")
	case "zipformer-en":
		return filepath.Join(p.modelDir, "sherpa-onnx-streaming-zipformer-en-2023-06-26")
	case "paraformer-zh":
		return filepath.Join(p.modelDir, "sherpa-onnx-paraformer-zh-2023-09-14")
	case "sensevoice-small":
		return filepath.Join(p.modelDir, "sherpa-onnx-sense-voice-zh-en-ja-ko-yue-2024-07-17")
	default:
		return filepath.Join(p.modelDir, "sherpa-onnx-whisper-tiny")
	}
}

// initASR initializes the sherpa-onnx ASR engine.
func (p *SherpaProvider) initASR() error {
	// Placeholder - sherpa-onnx native library integration would go here
	return nil
}

// Close releases resources.
func (p *SherpaProvider) Close() {
	// Placeholder - cleanup would go here
}

// Transcribe transcribes audio to text.
func (p *SherpaProvider) Transcribe(ctx context.Context, req *TranscribeRequest) (*TranscribeResponse, error) {
	if !p.IsModelReady() {
		return nil, fmt.Errorf("model not downloaded. Please download the model first")
	}

	// Read audio data
	audioData, err := io.ReadAll(req.Audio)
	if err != nil {
		return nil, fmt.Errorf("failed to read audio: %w", err)
	}

	// TODO: Implement actual ASR using sherpa-onnx native library
	// For now, return a placeholder response
	_ = audioData

	return nil, fmt.Errorf("ASR transcription not yet implemented. Model files are ready at: %s", p.getModelPath())
}

// TranscribeStream transcribes audio with streaming results.
func (p *SherpaProvider) TranscribeStream(ctx context.Context, req *TranscribeRequest, callback StreamCallback) error {
	if !p.streaming {
		// Fall back to batch transcription
		resp, err := p.Transcribe(ctx, req)
		if err != nil {
			return err
		}
		return callback(resp)
	}

	// TODO: Implement streaming ASR
	return fmt.Errorf("streaming ASR not yet implemented")
}

// GetModelStatus returns the current model status.
func (p *SherpaProvider) GetModelStatus() *SherpaASRModelStatus {
	externalType := p.modelType

	status := &SherpaASRModelStatus{
		Ready:              p.IsModelReady(),
		ModelDir:          p.modelDir,
		ModelType:         externalType,
		StreamingSupported: p.streaming,
		Downloading:       p.downloadMgr.IsDownloading(),
		HasPending:        p.downloadMgr.HasPendingDownload(),
	}

	if status.Downloading {
		progress := p.downloadMgr.GetProgress()
		status.Progress = &progress
	} else if status.HasPending {
		status.PendingModel = p.downloadMgr.GetPendingModelType()
		status.SavedProgress = p.downloadMgr.GetSavedProgress()
	}

	return status
}

// DownloadModel downloads the model files.
func (p *SherpaProvider) DownloadModel(ctx context.Context, modelType string) error {
	if modelType == "" {
		modelType = p.modelType
	}

	err := p.downloadMgr.Download(ctx, modelType)
	if err != nil {
		return err
	}

	// Update model type and ready status
	p.mu.Lock()
	p.modelType = modelType
	if meta, ok := sherpaASRModelMeta[modelType]; ok {
		p.streaming = meta.Streaming
	}
	p.modelReady = p.checkModelReady()
	p.mu.Unlock()

	// Initialize ASR if model is ready
	if p.modelReady {
		return p.initASR()
	}

	return nil
}

// SetOnProgress sets the progress callback for downloads.
func (p *SherpaProvider) SetOnProgress(callback func(progress SherpaASRDownloadProgress)) {
	p.downloadMgr.OnProgress = callback
}

// ListModels returns all available ASR models with download status.
func (p *SherpaProvider) ListModels() []interface{} {
	models := make([]interface{}, 0, len(sherpaASRModelMeta))
	for _, meta := range sherpaASRModelMeta {
		model := meta
		model.Downloaded = p.downloadMgr.IsModelComplete(meta.ID)
		models = append(models, model)
	}
	return models
}

// ListASRModels returns all available ASR models with download status (typed version).
func (p *SherpaProvider) ListASRModels() []SherpaASRModelInfo {
	models := make([]SherpaASRModelInfo, 0, len(sherpaASRModelMeta))
	for _, meta := range sherpaASRModelMeta {
		model := meta
		model.Downloaded = p.downloadMgr.IsModelComplete(meta.ID)
		models = append(models, model)
	}
	return models
}

// SwitchModel switches to a different ASR model.
func (p *SherpaProvider) SwitchModel(modelType string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if model is downloaded
	oldModelType := p.modelType
	p.modelType = modelType

	p.mu.Unlock()
	ready := p.checkModelReady()
	p.mu.Lock()

	if !ready {
		p.modelType = oldModelType
		return fmt.Errorf("model %s is not downloaded", modelType)
	}

	p.modelReady = ready
	if meta, ok := sherpaASRModelMeta[modelType]; ok {
		p.streaming = meta.Streaming
	}

	p.mu.Unlock()
	err := p.initASR()
	p.mu.Lock()

	return err
}

// DeleteModel deletes a downloaded model.
func (p *SherpaProvider) DeleteModel(modelType string) error {
	if modelType == "" {
		modelType = p.modelType
	}

	// Don't delete the currently active model
	if modelType == p.modelType && p.modelReady {
		return fmt.Errorf("cannot delete the currently active model")
	}

	return p.downloadMgr.DeleteModel(modelType)
}

// GetModelDir returns the model directory path.
func (p *SherpaProvider) GetModelDir() string {
	return p.modelDir
}

// GetDownloadManager returns the download manager.
func (p *SherpaProvider) GetDownloadManager() *SherpaASRDownloadManager {
	return p.downloadMgr
}

// SherpaASRModelStatus represents the model status response.
type SherpaASRModelStatus struct {
	Ready              bool                      `json:"ready"`
	ModelDir           string                    `json:"model_dir"`
	ModelType          string                    `json:"model_type"`
	StreamingSupported bool                      `json:"streaming_supported"`
	Downloading        bool                      `json:"downloading"`
	Progress           *SherpaASRDownloadProgress `json:"progress,omitempty"`
	HasPending         bool                      `json:"has_pending"`
	PendingModel       string                    `json:"pending_model,omitempty"`
	SavedProgress      *SherpaASRDownloadProgress `json:"saved_progress,omitempty"`
}

// GetAvailableASRModels returns all available ASR models metadata (without download status).
// This can be called without an initialized provider.
func GetAvailableASRModels() []SherpaASRModelInfo {
	models := make([]SherpaASRModelInfo, 0, len(sherpaASRModelMeta))
	for _, meta := range sherpaASRModelMeta {
		models = append(models, meta)
	}
	return models
}

// audioToWAV converts audio bytes to WAV format if needed.
func audioToWAV(data []byte, format AudioFormat) ([]byte, error) {
	if format == FormatWAV {
		return data, nil
	}
	// TODO: Implement audio format conversion
	return nil, fmt.Errorf("audio format conversion not yet implemented for %s", format)
}

// Helper to create a reader from bytes
func bytesToReader(data []byte) io.Reader {
	return bytes.NewReader(data)
}
