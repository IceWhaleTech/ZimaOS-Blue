//go:build windows

package tts

import (
	"archive/tar"
	"bytes"
	"compress/bzip2"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

// SherpaProvider implements the Provider interface using sherpa-onnx via purego.
type SherpaProvider struct {
	modelDir      string
	modelType     string
	defaultVoice  string
	defaultFormat AudioFormat
	maxTextLength int
	modelReady    bool
	mu            sync.RWMutex
	downloadMgr   *SherpaDownloadManager
	tts           uintptr
	sampleRate    int
	initialized   bool
	initializing  bool      // Track if async initialization is in progress
	initErr       error     // Store initialization error
	initDone      chan struct{} // Signal when initialization completes
}

// sherpa-onnx library state
var (
	sherpaLibHandle uintptr
	sherpaLibMu     sync.Mutex
	sherpaLibLoaded bool

	// C API functions
	sherpaCreateOfflineTts                func(config uintptr) uintptr
	sherpaDestroyOfflineTts               func(tts uintptr)
	sherpaOfflineTtsGenerate              func(tts uintptr, text uintptr, sid int32, speed float32) uintptr
	sherpaDestroyOfflineTtsGeneratedAudio func(audio uintptr)
)

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
	}
	p.downloadMgr = &SherpaDownloadManager{ModelDir: modelDir}
	p.modelReady = p.checkModelReady()

	// Pre-initialize TTS engine if model is ready
	if p.modelReady {
		go func() {
			if err := p.initTTS(); err != nil {
				fmt.Printf("Failed to pre-initialize TTS: %v\n", err)
			} else {
				fmt.Println("TTS engine pre-initialized successfully")
			}
		}()
	}

	return p
}

func (p *SherpaProvider) Name() string                        { return fmt.Sprintf("Sherpa TTS (%s)", p.modelType) }
func (p *SherpaProvider) Type() ProviderType                  { return ProviderSherpa }
func (p *SherpaProvider) SupportedFormats() []AudioFormat     { return []AudioFormat{FormatWAV} }
func (p *SherpaProvider) MaxTextLength() int                  { return p.maxTextLength }
func (p *SherpaProvider) GetModelDir() string                 { return p.modelDir }
func (p *SherpaProvider) GetDownloadManager() *SherpaDownloadManager { return p.downloadMgr }

func (p *SherpaProvider) IsModelReady() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.modelReady
}

// RefreshModelStatus re-checks if the model is ready (call after download completes).
func (p *SherpaProvider) RefreshModelStatus() {
	ready := p.checkModelReady()
	p.mu.Lock()
	p.modelReady = ready
	p.mu.Unlock()

	// Pre-initialize TTS engine if model is now ready
	if ready && !p.initialized {
		go func() {
			if err := p.initTTS(); err != nil {
				fmt.Printf("Failed to initialize TTS after download: %v\n", err)
			} else {
				fmt.Println("TTS engine initialized after download")
			}
		}()
	}
}

func (p *SherpaProvider) checkModelReady() bool {
	modelPath := p.getModelPath()
	fmt.Printf("TTS checkModelReady: modelType=%s, modelPath=%s\n", p.modelType, modelPath)
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		fmt.Printf("TTS checkModelReady: model path does not exist\n")
		return false
	}
	ready := p.verifyModelFiles()
	fmt.Printf("TTS checkModelReady: verifyModelFiles=%v\n", ready)
	return ready
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
			fmt.Printf("TTS verifyModelFiles: missing file %s\n", filePath)
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

func loadSherpaLibrary(libDir string) error {
	sherpaLibMu.Lock()
	defer sherpaLibMu.Unlock()
	if sherpaLibLoaded {
		return nil
	}

	// Load onnxruntime first
	onnxPath := filepath.Join(libDir, "onnxruntime.dll")
	if _, err := os.Stat(onnxPath); err == nil {
		syscall.LoadLibrary(onnxPath)
	}

	libPath := filepath.Join(libDir, "sherpa-onnx-c-api.dll")
	if _, err := os.Stat(libPath); os.IsNotExist(err) {
		return fmt.Errorf("sherpa library not found: %s", libPath)
	}

	handle, err := syscall.LoadLibrary(libPath)
	if err != nil {
		return fmt.Errorf("failed to load sherpa library: %w", err)
	}
	sherpaLibHandle = uintptr(handle)

	purego.RegisterLibFunc(&sherpaCreateOfflineTts, sherpaLibHandle, "SherpaOnnxCreateOfflineTts")
	purego.RegisterLibFunc(&sherpaDestroyOfflineTts, sherpaLibHandle, "SherpaOnnxDestroyOfflineTts")
	purego.RegisterLibFunc(&sherpaOfflineTtsGenerate, sherpaLibHandle, "SherpaOnnxOfflineTtsGenerate")
	purego.RegisterLibFunc(&sherpaDestroyOfflineTtsGeneratedAudio, sherpaLibHandle, "SherpaOnnxDestroyOfflineTtsGeneratedAudio")

	sherpaLibLoaded = true
	return nil
}

// Keep strings alive
var configStrings [][]byte

func cStr(s string) uintptr {
	if s == "" {
		return 0
	}
	b := append([]byte(s), 0)
	configStrings = append(configStrings, b)
	return uintptr(unsafe.Pointer(&b[0]))
}

func (p *SherpaProvider) initTTS() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.initialized && p.tts != 0 {
		return nil
	}

	configStrings = nil
	libDir := filepath.Join(p.modelDir, "lib")
	if err := ensureSherpaLibraries(p.modelDir); err != nil {
		return fmt.Errorf("failed to setup sherpa libraries: %w", err)
	}
	if err := loadSherpaLibrary(libDir); err != nil {
		return fmt.Errorf("failed to load sherpa library: %w", err)
	}

	modelPath := p.getModelPath()

	// C struct layout on 64-bit Windows:
	// SherpaOnnxOfflineTtsModelConfig {
	//   SherpaOnnxOfflineTtsVitsModelConfig vits;   // offset 0
	//   int32_t num_threads;                        // offset 56
	//   int32_t debug;                              // offset 60
	//   const char *provider;                       // offset 64
	//   ...
	// }
	//
	// SherpaOnnxOfflineTtsVitsModelConfig {
	//   const char *model;       // offset 0
	//   const char *lexicon;     // offset 8
	//   const char *tokens;      // offset 16
	//   const char *data_dir;    // offset 24
	//   float noise_scale;       // offset 32
	//   float noise_scale_w;     // offset 36
	//   float length_scale;      // offset 40
	//   const char *dict_dir;    // offset 48
	// }

	// Allocate enough space for the full config
	configBuf := make([]byte, 512)
	configStrings = append(configStrings, configBuf)
	configPtr := uintptr(unsafe.Pointer(&configBuf[0]))

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

	// VITS config is at offset 0 within ModelConfig
	modelOnnx := cStr(filepath.Join(modelPath, onnxFile))
	tokensTxt := cStr(filepath.Join(modelPath, "tokens.txt"))
	dataDir := cStr(filepath.Join(modelPath, "espeak-ng-data"))

	// Set VITS fields (Piper models use VITS)
	*(*uintptr)(unsafe.Pointer(configPtr + 0)) = modelOnnx   // model
	*(*uintptr)(unsafe.Pointer(configPtr + 8)) = 0           // lexicon (not used)
	*(*uintptr)(unsafe.Pointer(configPtr + 16)) = tokensTxt  // tokens
	*(*uintptr)(unsafe.Pointer(configPtr + 24)) = dataDir    // data_dir
	*(*float32)(unsafe.Pointer(configPtr + 32)) = 0.667      // noise_scale
	*(*float32)(unsafe.Pointer(configPtr + 36)) = 0.8        // noise_scale_w
	*(*float32)(unsafe.Pointer(configPtr + 40)) = 1.0        // length_scale

	// Set num_threads at offset 56 in ModelConfig
	numCPU := runtime.NumCPU()
	if numCPU > 4 {
		numCPU = 4
	}
	*(*int32)(unsafe.Pointer(configPtr + 56)) = int32(numCPU)

	fmt.Printf("Creating TTS engine with model: %s (threads=%d)\n", modelPath, numCPU)
	p.tts = sherpaCreateOfflineTts(configPtr)
	if p.tts == 0 {
		return fmt.Errorf("failed to create TTS engine")
	}
	fmt.Printf("TTS engine created: %x\n", p.tts)

	p.initialized = true
	return nil
}

func (p *SherpaProvider) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.tts != 0 && sherpaDestroyOfflineTts != nil {
		sherpaDestroyOfflineTts(p.tts)
		p.tts = 0
	}
	p.initialized = false
}

func (p *SherpaProvider) Synthesize(ctx context.Context, req *SynthesizeRequest) (*SynthesizeResponse, error) {
	if !p.IsModelReady() {
		return nil, fmt.Errorf("TTS model not ready. Please download a model from Settings > Speech")
	}
	if len(req.Text) > p.maxTextLength {
		return nil, ErrTextTooLong
	}
	if err := p.initTTS(); err != nil {
		return nil, fmt.Errorf("failed to initialize TTS: %w", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	textBytes := append([]byte(req.Text), 0)
	textPtr := uintptr(unsafe.Pointer(&textBytes[0]))

	fmt.Printf("Generating speech for: %s (len=%d)\n", req.Text, len(req.Text))
	startTime := time.Now()

	// Run TTS generation in a goroutine with timeout
	type result struct {
		audio uintptr
	}
	resultCh := make(chan result, 1)

	go func() {
		audio := sherpaOfflineTtsGenerate(p.tts, textPtr, 0, 1.0)
		resultCh <- result{audio: audio}
	}()

	// Wait with timeout (30 seconds max)
	var audio uintptr
	select {
	case r := <-resultCh:
		audio = r.audio
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("TTS generation timed out after 30s")
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	fmt.Printf("TTS generation took: %v\n", time.Since(startTime))

	if audio == 0 {
		return nil, fmt.Errorf("TTS generation failed")
	}
	defer sherpaDestroyOfflineTtsGeneratedAudio(audio)

	// Read struct: samples(ptr), n(int32), sample_rate(int32)
	samplesPtr := *(*uintptr)(unsafe.Pointer(audio))
	n := *(*int32)(unsafe.Pointer(audio + 8))
	sampleRate := *(*int32)(unsafe.Pointer(audio + 12))

	fmt.Printf("Generated %d samples at %d Hz\n", n, sampleRate)
	if n <= 0 || samplesPtr == 0 {
		return nil, fmt.Errorf("TTS returned empty audio")
	}

	samples := make([]float32, n)
	for i := int32(0); i < n; i++ {
		samples[i] = *(*float32)(unsafe.Pointer(samplesPtr + uintptr(i)*4))
	}

	wavData := samplesToWAV(samples, int(sampleRate))
	return &SynthesizeResponse{
		Audio:       io.NopCloser(bytes.NewReader(wavData)),
		ContentType: "audio/wav",
		Format:      FormatWAV,
	}, nil
}

func samplesToWAV(samples []float32, sampleRate int) []byte {
	buf := new(bytes.Buffer)
	n := len(samples)
	dataSize := n * 2
	buf.WriteString("RIFF")
	binary.Write(buf, binary.LittleEndian, uint32(36+dataSize))
	buf.WriteString("WAVEfmt ")
	binary.Write(buf, binary.LittleEndian, uint32(16))
	binary.Write(buf, binary.LittleEndian, uint16(1))
	binary.Write(buf, binary.LittleEndian, uint16(1))
	binary.Write(buf, binary.LittleEndian, uint32(sampleRate))
	binary.Write(buf, binary.LittleEndian, uint32(sampleRate*2))
	binary.Write(buf, binary.LittleEndian, uint16(2))
	binary.Write(buf, binary.LittleEndian, uint16(16))
	buf.WriteString("data")
	binary.Write(buf, binary.LittleEndian, uint32(dataSize))
	for _, s := range samples {
		if s > 1 { s = 1 } else if s < -1 { s = -1 }
		binary.Write(buf, binary.LittleEndian, int16(s*32767))
	}
	return buf.Bytes()
}

func (p *SherpaProvider) SynthesizeStream(ctx context.Context, req *SynthesizeRequest, cb StreamCallback) error {
	if !p.IsModelReady() {
		return fmt.Errorf("TTS model not ready")
	}
	if err := p.initTTS(); err != nil {
		return fmt.Errorf("failed to initialize TTS: %w", err)
	}

	// Split text into sentences for streaming
	sentences := splitIntoSentences(req.Text)
	if len(sentences) == 0 {
		return nil
	}

	// For single sentence or short text, use regular synthesis
	if len(sentences) == 1 {
		resp, err := p.Synthesize(ctx, req)
		if err != nil {
			return err
		}
		defer resp.Audio.Close()
		buf := make([]byte, 4096)
		for {
			n, err := resp.Audio.Read(buf)
			if n > 0 {
				if e := cb(buf[:n]); e != nil {
					return e
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

	// Stream multiple sentences
	for i, sentence := range sentences {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if sentence == "" {
			continue
		}

		fmt.Printf("Streaming sentence %d/%d: %s\n", i+1, len(sentences), sentence)

		resp, err := p.Synthesize(ctx, &SynthesizeRequest{
			Text:   sentence,
			Voice:  req.Voice,
			Format: req.Format,
			Speed:  req.Speed,
		})
		if err != nil {
			fmt.Printf("Failed to synthesize sentence %d: %v\n", i+1, err)
			continue
		}

		buf := make([]byte, 4096)
		for {
			n, err := resp.Audio.Read(buf)
			if n > 0 {
				if e := cb(buf[:n]); e != nil {
					resp.Audio.Close()
					return e
				}
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				resp.Audio.Close()
				return err
			}
		}
		resp.Audio.Close()
	}

	return nil
}

// splitIntoSentences splits text into sentences for streaming TTS
func splitIntoSentences(text string) []string {
	var sentences []string
	var current []rune

	for _, r := range text {
		current = append(current, r)
		// Split on sentence-ending punctuation
		if r == '.' || r == '!' || r == '?' || r == '。' || r == '！' || r == '？' || r == '；' || r == '\n' {
			s := strings.TrimSpace(string(current))
			if s != "" {
				sentences = append(sentences, s)
			}
			current = nil
		}
	}

	// Add remaining text
	if len(current) > 0 {
		s := strings.TrimSpace(string(current))
		if s != "" {
			sentences = append(sentences, s)
		}
	}

	return sentences
}

func (p *SherpaProvider) ListVoices(ctx context.Context) ([]Voice, error) {
	return []Voice{{ID: "0", Name: "Default", Language: "en-US", Gender: "female"}}, nil
}

func (p *SherpaProvider) SwitchModel(modelType string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Store the model type as-is (don't convert)
	old := p.modelType
	p.modelType = modelType

	// Temporarily unlock to check model ready (it acquires lock)
	p.mu.Unlock()
	ready := p.checkModelReady()
	p.mu.Lock()

	if !ready {
		p.modelType = old
		return fmt.Errorf("model not downloaded")
	}

	// Destroy old TTS engine
	if p.tts != 0 {
		sherpaDestroyOfflineTts(p.tts)
		p.tts = 0
	}
	p.initialized = false
	p.modelReady = ready

	// Pre-initialize new TTS engine in background
	go func() {
		if err := p.initTTS(); err != nil {
			fmt.Printf("Failed to initialize TTS after switch: %v\n", err)
		} else {
			fmt.Printf("TTS engine initialized for model: %s\n", modelType)
		}
	}()

	return nil
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
		{ID: "piper-en", Name: "Piper English (Lessac)", Description: "Fast English TTS - Lessac voice", Languages: []string{"en-US"}, Size: "~60MB"},
		{ID: "piper-en-hfc", Name: "Piper English (HFC Female)", Description: "Fast English TTS - HFC Female voice", Languages: []string{"en-US"}, Size: "~75MB"},
		{ID: "piper-de", Name: "Piper German (Thorsten)", Description: "German TTS - Thorsten voice", Languages: []string{"de-DE"}, Size: "~75MB"},
		{ID: "piper-es", Name: "Piper Spanish (Davefx)", Description: "Spanish TTS - Davefx voice", Languages: []string{"es-ES"}, Size: "~75MB"},
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
		if p.tts != 0 {
			sherpaDestroyOfflineTts(p.tts)
			p.tts = 0
		}
		p.initialized = false
	}
	p.mu.Unlock()

	return nil
}

var sherpaLibURLs = map[string]map[string][]string{
	"windows": {"amd64": {"https://github.com/k2-fsa/sherpa-onnx/releases/download/v1.12.23/sherpa-onnx-v1.12.23-win-x64-shared.tar.bz2"}},
}

func ensureSherpaLibraries(modelDir string) error {
	libDir := filepath.Join(modelDir, "lib")
	if _, err := os.Stat(filepath.Join(libDir, "sherpa-onnx-c-api.dll")); err == nil {
		return nil
	}
	urls := sherpaLibURLs[runtime.GOOS][runtime.GOARCH]
	os.MkdirAll(libDir, 0755)
	for _, url := range urls {
		if err := downloadAndExtractLibs(url, libDir); err == nil { return nil }
	}
	return fmt.Errorf("failed to download sherpa libraries")
}

func downloadAndExtractLibs(url, libDir string) error {
	resp, err := http.DefaultClient.Get(url)
	if err != nil { return err }
	defer resp.Body.Close()
	if resp.StatusCode != 200 { return fmt.Errorf("download failed") }
	bzr := bzip2.NewReader(resp.Body)
	tr := tar.NewReader(bzr)
	for {
		h, err := tr.Next()
		if err == io.EOF { break }
		if err != nil { return err }
		if h.Typeflag != tar.TypeReg { continue }
		name := filepath.Base(h.Name)
		if filepath.Ext(name) != ".dll" { continue }
		f, _ := os.OpenFile(filepath.Join(libDir, name), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		io.Copy(f, tr)
		f.Close()
	}
	return nil
}
