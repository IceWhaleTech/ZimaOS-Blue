package tts

import (
	"archive/tar"
	"bytes"
	"compress/bzip2"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// SherpaProvider implements the Provider interface using sherpa-onnx CLI.
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
}

// SherpaConfig holds the configuration for the Sherpa TTS provider.
type SherpaConfig struct {
	ModelDir      string
	ModelType     string
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

const (
	SherpaModelBaseURL = "https://github.com/k2-fsa/sherpa-onnx/releases/download/tts-models"
)

var sherpaModelPackages = map[string]string{
	"kokoro-en":    "kokoro-en-v0_19.tar.bz2",
	"kokoro-multi": "kokoro-multi-lang-v1_0.tar.bz2",
	"piper-en":     "vits-piper-en_US-lessac-medium.tar.bz2",
	"vits-zh":      "vits-zh-aishell3.tar.bz2",
}

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
	}

	p.downloadMgr = &SherpaDownloadManager{ModelDir: modelDir}
	p.modelReady = p.checkModelReady()

	return p
}

func (p *SherpaProvider) Name() string {
	return fmt.Sprintf("Sherpa TTS (%s)", p.modelType)
}

func (p *SherpaProvider) Type() ProviderType {
	return ProviderSherpa
}

func (p *SherpaProvider) SupportedFormats() []AudioFormat {
	return []AudioFormat{FormatWAV}
}

func (p *SherpaProvider) MaxTextLength() int {
	return p.maxTextLength
}

func (p *SherpaProvider) IsModelReady() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.modelReady
}

func (p *SherpaProvider) checkModelReady() bool {
	modelPath := p.getModelPath()
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return false
	}
	if p.downloadMgr != nil && !p.downloadMgr.IsModelComplete(p.modelType) {
		return p.verifyModelFiles()
	}
	return true
}

func (p *SherpaProvider) verifyModelFiles() bool {
	modelPath := p.getModelPath()
	essentialFiles := []string{"model.onnx", "tokens.txt"}
	for _, file := range essentialFiles {
		if _, err := os.Stat(filepath.Join(modelPath, file)); os.IsNotExist(err) {
			return false
		}
	}
	if p.downloadMgr != nil {
		p.downloadMgr.markComplete(p.modelType)
	}
	return true
}

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

func (p *SherpaProvider) getTTSExePath() string {
	binDir := filepath.Join(p.modelDir, "bin")
	if runtime.GOOS == "windows" {
		return filepath.Join(binDir, "sherpa-onnx-offline-tts.exe")
	}
	return filepath.Join(binDir, "sherpa-onnx-offline-tts")
}

func (p *SherpaProvider) Close() {}

func (p *SherpaProvider) Synthesize(ctx context.Context, req *SynthesizeRequest) (*SynthesizeResponse, error) {
	if !p.IsModelReady() {
		return nil, fmt.Errorf("TTS model not ready. Please download a model from Settings > Speech")
	}

	if len(req.Text) > p.maxTextLength {
		return nil, ErrTextTooLong
	}

	// Ensure CLI tool exists
	if err := ensureSherpaLibraries(p.modelDir); err != nil {
		return nil, fmt.Errorf("failed to setup sherpa: %w", err)
	}

	exePath := p.getTTSExePath()
	if _, err := os.Stat(exePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("TTS executable not found: %s", exePath)
	}

	modelPath := p.getModelPath()

	// Create temp output file
	tmpFile, err := os.CreateTemp("", "tts-*.wav")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpFile.Close()
	outputPath := tmpFile.Name()
	defer os.Remove(outputPath)

	// Build command args based on model type
	var args []string
	switch p.modelType {
	case "kokoro", "kokoro-multi":
		args = []string{
			"--kokoro-model=" + filepath.Join(modelPath, "model.onnx"),
			"--kokoro-voices=" + filepath.Join(modelPath, "voices.bin"),
			"--kokoro-tokens=" + filepath.Join(modelPath, "tokens.txt"),
			"--kokoro-data-dir=" + filepath.Join(modelPath, "espeak-ng-data"),
			"--sid=0",
			"--output-filename=" + outputPath,
			"--text=" + req.Text,
		}
	case "piper":
		args = []string{
			"--vits-model=" + filepath.Join(modelPath, "en_US-lessac-medium.onnx"),
			"--vits-tokens=" + filepath.Join(modelPath, "tokens.txt"),
			"--vits-data-dir=" + filepath.Join(modelPath, "espeak-ng-data"),
			"--output-filename=" + outputPath,
			"--text=" + req.Text,
		}
	case "vits-zh":
		args = []string{
			"--vits-model=" + filepath.Join(modelPath, "model.onnx"),
			"--vits-tokens=" + filepath.Join(modelPath, "tokens.txt"),
			"--vits-lexicon=" + filepath.Join(modelPath, "lexicon.txt"),
			"--output-filename=" + outputPath,
			"--text=" + req.Text,
		}
	default:
		return nil, fmt.Errorf("unsupported model type: %s", p.modelType)
	}

	// Run TTS
	cmd := exec.CommandContext(ctx, exePath, args...)
	cmd.Dir = filepath.Dir(exePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("TTS failed: %w, output: %s", err, string(output))
	}

	// Read output file
	wavData, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read output: %w", err)
	}

	return &SynthesizeResponse{
		Audio:       io.NopCloser(bytes.NewReader(wavData)),
		ContentType: "audio/wav",
		Format:      FormatWAV,
	}, nil
}

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

func (p *SherpaProvider) ListVoices(ctx context.Context) ([]Voice, error) {
	switch p.modelType {
	case "kokoro", "kokoro-multi":
		return []Voice{
			{ID: "0", Name: "Default", Language: "en-US", Gender: "female"},
		}, nil
	case "piper":
		return []Voice{{ID: "0", Name: "Lessac", Language: "en-US", Gender: "neutral"}}, nil
	case "vits-zh":
		return []Voice{{ID: "0", Name: "Speaker 0", Language: "zh-CN", Gender: "female"}}, nil
	default:
		return []Voice{}, nil
	}
}

func (p *SherpaProvider) GetModelDir() string {
	return p.modelDir
}

func (p *SherpaProvider) GetDownloadManager() *SherpaDownloadManager {
	return p.downloadMgr
}

func (p *SherpaProvider) SwitchModel(modelType string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	internalType := modelType
	switch modelType {
	case "kokoro-en":
		internalType = "kokoro"
	case "piper-en":
		internalType = "piper"
	}

	oldModelType := p.modelType
	p.modelType = internalType

	p.mu.Unlock()
	ready := p.checkModelReady()
	p.mu.Lock()

	if !ready {
		p.modelType = oldModelType
		return fmt.Errorf("model %s is not downloaded", modelType)
	}

	p.modelReady = ready
	return nil
}

type TTSModelInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Languages   []string `json:"languages"`
	Size        string   `json:"size"`
	Downloaded  bool     `json:"downloaded"`
}

func (p *SherpaProvider) ListModels() []TTSModelInfo {
	models := []TTSModelInfo{
		{ID: "kokoro-en", Name: "Kokoro English", Description: "High-quality English TTS", Languages: []string{"en-US", "en-GB"}, Size: "~350MB"},
		{ID: "kokoro-multi", Name: "Kokoro Multilingual", Description: "English and Chinese TTS", Languages: []string{"en-US", "zh-CN"}, Size: "~400MB"},
		{ID: "piper-en", Name: "Piper English", Description: "Fast English TTS", Languages: []string{"en-US"}, Size: "~60MB"},
		{ID: "vits-zh", Name: "VITS Chinese", Description: "Chinese TTS", Languages: []string{"zh-CN"}, Size: "~100MB"},
	}

	for i, model := range models {
		var checkPath string
		switch model.ID {
		case "kokoro-en":
			checkPath = filepath.Join(p.modelDir, "kokoro-en-v0_19")
		case "kokoro-multi":
			checkPath = filepath.Join(p.modelDir, "kokoro-multi-lang-v1_0")
		case "piper-en":
			checkPath = filepath.Join(p.modelDir, "vits-piper-en_US-lessac-medium")
		case "vits-zh":
			checkPath = filepath.Join(p.modelDir, "vits-zh-aishell3")
		}
		if _, err := os.Stat(checkPath); err == nil {
			models[i].Downloaded = true
		}
	}

	return models
}

func (p *SherpaProvider) DeleteModel(modelType string) error {
	var modelPath string
	switch modelType {
	case "kokoro-en":
		modelPath = filepath.Join(p.modelDir, "kokoro-en-v0_19")
	case "kokoro-multi":
		modelPath = filepath.Join(p.modelDir, "kokoro-multi-lang-v1_0")
	case "piper-en":
		modelPath = filepath.Join(p.modelDir, "vits-piper-en_US-lessac-medium")
	case "vits-zh":
		modelPath = filepath.Join(p.modelDir, "vits-zh-aishell3")
	default:
		return fmt.Errorf("unknown model type: %s", modelType)
	}

	if err := os.RemoveAll(modelPath); err != nil {
		return fmt.Errorf("failed to delete model: %w", err)
	}

	p.mu.Lock()
	if p.modelType == modelType || (modelType == "kokoro-en" && p.modelType == "kokoro") {
		p.modelReady = false
	}
	p.mu.Unlock()

	return nil
}

var sherpaLibURLs = map[string]map[string][]string{
	"windows": {
		"amd64": {"https://github.com/k2-fsa/sherpa-onnx/releases/download/v1.12.23/sherpa-onnx-v1.12.23-win-x64-shared.tar.bz2"},
	},
	"linux": {
		"amd64": {"https://github.com/k2-fsa/sherpa-onnx/releases/download/v1.12.23/sherpa-onnx-v1.12.23-linux-x64-shared.tar.bz2"},
		"arm64": {"https://github.com/k2-fsa/sherpa-onnx/releases/download/v1.12.23/sherpa-onnx-v1.12.23-linux-aarch64-shared.tar.bz2"},
	},
	"darwin": {
		"amd64": {"https://github.com/k2-fsa/sherpa-onnx/releases/download/v1.12.23/sherpa-onnx-v1.12.23-osx-x86_64-shared.tar.bz2"},
		"arm64": {"https://github.com/k2-fsa/sherpa-onnx/releases/download/v1.12.23/sherpa-onnx-v1.12.23-osx-arm64-shared.tar.bz2"},
	},
}

func ensureSherpaLibraries(modelDir string) error {
	binDir := filepath.Join(modelDir, "bin")
	libDir := filepath.Join(modelDir, "lib")

	// Check if CLI tool exists
	var exeName string
	if runtime.GOOS == "windows" {
		exeName = "sherpa-onnx-offline-tts.exe"
	} else {
		exeName = "sherpa-onnx-offline-tts"
	}

	if _, err := os.Stat(filepath.Join(binDir, exeName)); err == nil {
		return nil
	}

	urls, ok := sherpaLibURLs[runtime.GOOS]
	if !ok {
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
	archURLs, ok := urls[runtime.GOARCH]
	if !ok {
		return fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}

	os.MkdirAll(binDir, 0755)
	os.MkdirAll(libDir, 0755)

	for _, url := range archURLs {
		if err := downloadAndExtractSherpa(url, modelDir); err != nil {
			continue
		}
		return nil
	}

	return fmt.Errorf("failed to download sherpa")
}

func downloadAndExtractSherpa(url, modelDir string) error {
	resp, err := http.DefaultClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	bzr := bzip2.NewReader(resp.Body)
	tr := tar.NewReader(bzr)

	binDir := filepath.Join(modelDir, "bin")
	libDir := filepath.Join(modelDir, "lib")

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if header.Typeflag != tar.TypeReg {
			continue
		}

		name := filepath.Base(header.Name)
		dir := filepath.Dir(header.Name)

		var outPath string
		if filepath.Base(dir) == "bin" {
			outPath = filepath.Join(binDir, name)
		} else if filepath.Base(dir) == "lib" {
			outPath = filepath.Join(libDir, name)
		} else {
			continue
		}

		outFile, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return err
		}
		_, err = io.Copy(outFile, tr)
		outFile.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

func areSherpaLibrariesPresent(libDir string) bool {
	return false // Force re-check via ensureSherpaLibraries
}
