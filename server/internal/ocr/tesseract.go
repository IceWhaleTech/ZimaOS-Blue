package ocr

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/danlock/gogosseract"
	"github.com/tetratelabs/wazero"
	"go.uber.org/zap"
)

const (
	engineName            = "tesseract/wasm"
	defaultWorkerCount    = 1
	defaultModelBaseURL   = "https://cdn.jsdelivr.net/gh/tesseract-ocr/tessdata_best@main/"
	defaultRequestTimeout = 2 * time.Minute
)

type Result struct {
	Text           string   `json:"text"`
	Engine         string   `json:"engine,omitempty"`
	Model          string   `json:"model,omitempty"`
	Warnings       []string `json:"warnings,omitempty"`
	AutoDownloaded []string `json:"auto_downloaded,omitempty"`
}

type Config struct {
	ModelDir        string
	AutoDownload    bool
	WorkerCount     uint
	PreferredModels []string
	HTTPClient      *http.Client
}

type modelSpec struct {
	Name string
	URL  string
}

type parsePool interface {
	ParseImage(ctx context.Context, img io.Reader, opts gogosseract.ParseImageOptions) (string, error)
	Close()
}

type poolFactory func(ctx context.Context, count uint, cfg gogosseract.PoolConfig) (parsePool, error)
type modelDownloader func(ctx context.Context, url, path string) error

// TesseractService provides OCR over images using Tesseract WASM.
type TesseractService struct {
	logger          *zap.Logger
	modelDir        string
	autoDownload    bool
	workerCount     uint
	preferredModels []string
	httpClient      *http.Client
	cache           wazero.CompilationCache

	mu       sync.Mutex
	pools    map[string]parsePool
	newPool  poolFactory
	download modelDownloader
}

func NewTesseractService(logger *zap.Logger, cfg Config) *TesseractService {
	if logger == nil {
		logger = zap.NewNop()
	}
	if cfg.WorkerCount == 0 {
		cfg.WorkerCount = defaultWorkerCount
	}
	if len(cfg.PreferredModels) == 0 {
		cfg.PreferredModels = []string{"chi_sim", "eng"}
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: defaultRequestTimeout}
	}
	service := &TesseractService{
		logger:          logger,
		modelDir:        cfg.ModelDir,
		autoDownload:    cfg.AutoDownload,
		workerCount:     cfg.WorkerCount,
		preferredModels: append([]string(nil), cfg.PreferredModels...),
		httpClient:      cfg.HTTPClient,
		cache:           wazero.NewCompilationCache(),
		pools:           map[string]parsePool{},
	}
	service.newPool = service.defaultPoolFactory
	service.download = service.downloadModel
	return service
}

func (s *TesseractService) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	pools := make([]parsePool, 0, len(s.pools))
	for _, pool := range s.pools {
		pools = append(pools, pool)
	}
	s.pools = map[string]parsePool{}
	cache := s.cache
	s.cache = nil
	s.mu.Unlock()

	for _, pool := range pools {
		pool.Close()
	}
	if cache != nil {
		return cache.Close(context.Background())
	}
	return nil
}

func (s *TesseractService) Extract(ctx context.Context, imagePNG []byte) (Result, error) {
	if s == nil {
		return Result{}, fmt.Errorf("ocr service not available")
	}
	if len(imagePNG) == 0 {
		return Result{}, fmt.Errorf("image bytes are required")
	}
	var (
		best       Result
		bestScore  = -1 << 30
		errs       []string
		warnings   []string
		downloaded []string
	)
	for _, spec := range s.specs() {
		pool, autoDownloaded, err := s.ensurePool(ctx, spec)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", spec.Name, err))
			continue
		}
		if autoDownloaded {
			downloaded = appendUniqueString(downloaded, spec.Name)
			warnings = append(warnings, fmt.Sprintf("downloaded OCR model %s", spec.Name))
		}
		text, err := pool.ParseImage(ctx, bytes.NewReader(imagePNG), gogosseract.ParseImageOptions{})
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s parse: %v", spec.Name, err))
			continue
		}
		text = normalizeText(text)
		score := scoreText(text)
		if score > bestScore || (score == bestScore && utf8.RuneCountInString(text) > utf8.RuneCountInString(best.Text)) {
			bestScore = score
			best = Result{Text: text, Engine: engineName, Model: spec.Name}
		}
	}
	best.Warnings = warnings
	best.AutoDownloaded = downloaded
	if strings.TrimSpace(best.Text) != "" {
		return best, nil
	}
	if len(errs) > 0 {
		return best, fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return best, fmt.Errorf("no OCR model available")
}

func (s *TesseractService) specs() []modelSpec {
	out := make([]modelSpec, 0, len(s.preferredModels))
	for _, name := range s.preferredModels {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		out = append(out, modelSpec{Name: trimmed, URL: defaultModelBaseURL + trimmed + ".traineddata"})
	}
	return out
}

func (s *TesseractService) ensurePool(ctx context.Context, spec modelSpec) (parsePool, bool, error) {
	s.mu.Lock()
	if pool, ok := s.pools[spec.Name]; ok {
		s.mu.Unlock()
		return pool, false, nil
	}
	s.mu.Unlock()

	modelPath := filepath.Join(s.modelDir, spec.Name+".traineddata")
	autoDownloaded := false
	if _, err := os.Stat(modelPath); err != nil {
		if !os.IsNotExist(err) {
			return nil, false, fmt.Errorf("stat model: %w", err)
		}
		if !s.autoDownload {
			return nil, false, fmt.Errorf("missing OCR model %s", spec.Name)
		}
		if err := os.MkdirAll(s.modelDir, 0o750); err != nil {
			return nil, false, fmt.Errorf("create OCR model dir: %w", err)
		}
		if err := s.download(ctx, spec.URL, modelPath); err != nil {
			return nil, false, err
		}
		autoDownloaded = true
	}
	modelBytes, err := os.ReadFile(modelPath)
	if err != nil {
		return nil, false, fmt.Errorf("read OCR model: %w", err)
	}
	ocrCfg := gogosseract.Config{Language: spec.Name, WASMCache: s.cache}
	ocrCfg.Stdout = io.Discard
	ocrCfg.Stderr = io.Discard
	pool, err := s.newPool(ctx, s.workerCount, gogosseract.PoolConfig{
		Config:            ocrCfg,
		TrainingDataBytes: modelBytes,
	})
	if err != nil {
		return nil, false, fmt.Errorf("init OCR model %s: %w", spec.Name, err)
	}

	s.mu.Lock()
	if existing, ok := s.pools[spec.Name]; ok {
		s.mu.Unlock()
		pool.Close()
		return existing, autoDownloaded, nil
	}
	s.pools[spec.Name] = pool
	s.mu.Unlock()
	return pool, autoDownloaded, nil
}

func (s *TesseractService) defaultPoolFactory(ctx context.Context, count uint, cfg gogosseract.PoolConfig) (parsePool, error) {
	return gogosseract.NewPool(ctx, count, cfg)
}

func (s *TesseractService) downloadModel(ctx context.Context, url, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build OCR model request: %w", err)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("download OCR model: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download OCR model: unexpected status %s", resp.Status)
	}
	tmpPath := path + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create OCR model file: %w", err)
	}
	if _, err := io.Copy(file, resp.Body); err != nil {
		file.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write OCR model: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close OCR model file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("store OCR model: %w", err)
	}
	s.logger.Info("downloaded OCR model", zap.String("path", path), zap.String("url", url))
	return nil
}

func normalizeText(text string) string {
	text = strings.ReplaceAll(text, "\x00", "")
	return strings.TrimSpace(text)
}

func scoreText(text string) int {
	score := 0
	for _, r := range text {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			score += 3
		case unicode.IsSpace(r):
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			score--
		default:
			score--
		}
	}
	return score
}

func appendUniqueString(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
