package smallmodel

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	llamaCppDefaultTemperature = 0.2
	llamaServerStartupTimeout  = 45 * time.Second
	defaultPrefixCacheTTL      = 2 * time.Minute
)

type LlamaCppMode string

const (
	LlamaCppModeAuto   LlamaCppMode = "auto"
	LlamaCppModeServer LlamaCppMode = "server"
	LlamaCppModeCGO    LlamaCppMode = "cgo"
	LlamaCppModeFFI    LlamaCppMode = "ffi"
)

// LlamaCppRuntimeOptions controls llama.cpp runtime behavior.
type LlamaCppRuntimeOptions struct {
	Timeout       time.Duration
	MaxParallel   int
	BatchWindow   time.Duration
	BatchMaxSize  int
	CLIPath       string
	Mode          string
	ServerURL     string
	ServerBin     string
	ServerExtra   []string
	ServerStartup time.Duration
}

// LlamaCppRuntime executes fixed GGUF+mmproj inference via llama.cpp backends.
// Current production backend is server mode (llama-server + HTTP), with CLI fallback.
type LlamaCppRuntime struct {
	manager *Manager
	timeout time.Duration
	mode    LlamaCppMode

	parallelSem chan struct{}

	configuredCLI       string
	configuredServerURL string
	configuredServerBin string
	configuredServerArg []string
	serverStartup       time.Duration
	batchWindow         time.Duration
	batchMaxSize        int

	resolveMu          sync.RWMutex
	resolvedCLI        string
	resolvedServerBin  string
	resolveErrorCached error

	serverMu  sync.Mutex
	serverURL string
	serverCmd *exec.Cmd

	httpClient *http.Client
	cgoOnce    sync.Once
	cgoErr     error
	ffiBackend *llamaCppFFIBackend

	prefixMu        sync.Mutex
	prefixEntries   map[string]*llamaPrefixEntry
	prefixByCache   map[string]string
	nextPrefixSlot  int
	prefixHits      atomic.Uint64
	prefixMisses    atomic.Uint64
	prefixEvictions atomic.Uint64

	batcher *llamaBatchWindow
}

var _ PrefixCachingRuntime = (*LlamaCppRuntime)(nil)
var _ BatchingRuntime = (*LlamaCppRuntime)(nil)

type llamaPrefixEntry struct {
	PrefixID     string
	CacheKey     string
	Prefix       string
	SlotID       int
	PrefixTokens int
	ExpiresAt    time.Time
	LastUsedAt   time.Time
}

type llamaBatchTask struct {
	ctx    context.Context
	run    func(context.Context) (string, error)
	result chan llamaBatchResult
}

type llamaBatchResult struct {
	text string
	err  error
}

type llamaBatchWindow struct {
	window       time.Duration
	maxBatchSize int
	tasks        chan llamaBatchTask
	pending      atomic.Int64
	submitted    atomic.Uint64
	executed     atomic.Uint64
	batchCount   atomic.Uint64
	largestBatch atomic.Uint64
}

func NewLlamaCppRuntime(manager *Manager, opts ...LlamaCppRuntimeOptions) *LlamaCppRuntime {
	var opt LlamaCppRuntimeOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	timeout := opt.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	maxParallel := opt.MaxParallel
	if maxParallel <= 0 {
		maxParallel = defaultMaxParallel
	}

	cliPath := strings.TrimSpace(opt.CLIPath)
	if cliPath == "" {
		cliPath = strings.TrimSpace(os.Getenv("SMALL_MODEL_LLAMA_CPP_CLI"))
	}

	mode := normalizeLlamaCppMode(opt.Mode)
	if mode == LlamaCppModeAuto {
		mode = normalizeLlamaCppMode(os.Getenv("SMALL_MODEL_LLAMA_MODE"))
	}
	if mode == "" {
		mode = LlamaCppModeAuto
	}

	serverURL := strings.TrimSpace(opt.ServerURL)
	if serverURL == "" {
		serverURL = strings.TrimSpace(os.Getenv("SMALL_MODEL_LLAMA_SERVER_URL"))
	}
	serverBin := strings.TrimSpace(opt.ServerBin)
	if serverBin == "" {
		serverBin = strings.TrimSpace(os.Getenv("SMALL_MODEL_LLAMA_SERVER_BIN"))
	}
	if serverBin == "" {
		serverBin = "llama-server"
	}

	serverExtra := make([]string, 0, len(opt.ServerExtra))
	serverExtra = append(serverExtra, opt.ServerExtra...)
	if envExtra := strings.TrimSpace(os.Getenv("SMALL_MODEL_LLAMA_SERVER_ARGS")); envExtra != "" {
		serverExtra = append(serverExtra, strings.Fields(envExtra)...)
	}

	startup := opt.ServerStartup
	if startup <= 0 {
		startup = llamaServerStartupTimeout
	}

	batchWindow := opt.BatchWindow
	if batchWindow <= 0 {
		if raw := strings.TrimSpace(os.Getenv("SMALL_MODEL_LLAMA_BATCH_WINDOW_MS")); raw != "" {
			if ms, err := strconv.Atoi(raw); err == nil && ms > 0 {
				batchWindow = time.Duration(ms) * time.Millisecond
			}
		}
	}
	if batchWindow <= 0 {
		batchWindow = 2 * time.Millisecond
	}
	batchMaxSize := opt.BatchMaxSize
	if batchMaxSize <= 0 {
		if raw := strings.TrimSpace(os.Getenv("SMALL_MODEL_LLAMA_BATCH_MAX_SIZE")); raw != "" {
			if size, err := strconv.Atoi(raw); err == nil && size > 0 {
				batchMaxSize = size
			}
		}
	}
	if batchMaxSize <= 0 {
		batchMaxSize = maxParallel
	}

	rt := &LlamaCppRuntime{
		manager:             manager,
		timeout:             timeout,
		mode:                mode,
		parallelSem:         make(chan struct{}, maxParallel),
		configuredCLI:       cliPath,
		configuredServerURL: serverURL,
		configuredServerBin: serverBin,
		configuredServerArg: serverExtra,
		serverStartup:       startup,
		batchWindow:         batchWindow,
		batchMaxSize:        batchMaxSize,
		prefixEntries:       make(map[string]*llamaPrefixEntry),
		prefixByCache:       make(map[string]string),
		nextPrefixSlot:      1,
		httpClient: &http.Client{
			Timeout: 90 * time.Second,
		},
	}
	if batchWindow > 0 && batchMaxSize > 1 {
		rt.batcher = newLlamaBatchWindow(batchWindow, batchMaxSize)
	}
	return rt
}

func newLlamaBatchWindow(window time.Duration, maxBatchSize int) *llamaBatchWindow {
	if window <= 0 {
		window = 2 * time.Millisecond
	}
	if maxBatchSize <= 0 {
		maxBatchSize = 2
	}
	bw := &llamaBatchWindow{
		window:       window,
		maxBatchSize: maxBatchSize,
		tasks:        make(chan llamaBatchTask, maxBatchSize*4),
	}
	go bw.run()
	return bw
}

func (bw *llamaBatchWindow) Do(ctx context.Context, run func(context.Context) (string, error)) (string, error) {
	result := make(chan llamaBatchResult, 1)
	task := llamaBatchTask{ctx: ctx, run: run, result: result}
	bw.submitted.Add(1)
	bw.pending.Add(1)
	defer bw.pending.Add(-1)

	select {
	case bw.tasks <- task:
	case <-ctx.Done():
		return "", ctx.Err()
	}

	select {
	case out := <-result:
		return out.text, out.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (bw *llamaBatchWindow) Stats() BatchingStats {
	return BatchingStats{
		Supported:    true,
		Window:       bw.window,
		MaxBatchSize: bw.maxBatchSize,
		Pending:      bw.pending.Load(),
		Submitted:    bw.submitted.Load(),
		Executed:     bw.executed.Load(),
		BatchCount:   bw.batchCount.Load(),
		LargestBatch: int(bw.largestBatch.Load()),
	}
}

func (bw *llamaBatchWindow) run() {
	for first := range bw.tasks {
		bw.collectAndRun(first)
	}
}

func (bw *llamaBatchWindow) collectAndRun(first llamaBatchTask) {
	batch := []llamaBatchTask{first}
	timer := time.NewTimer(bw.window)
	defer timer.Stop()

	for len(batch) < bw.maxBatchSize {
		select {
		case task := <-bw.tasks:
			batch = append(batch, task)
		case <-timer.C:
			bw.runBatch(batch)
			return
		}
	}
	bw.runBatch(batch)
}

func (bw *llamaBatchWindow) runBatch(batch []llamaBatchTask) {
	bw.batchCount.Add(1)
	bw.executed.Add(uint64(len(batch)))
	for {
		largest := bw.largestBatch.Load()
		if uint64(len(batch)) <= largest {
			break
		}
		if bw.largestBatch.CompareAndSwap(largest, uint64(len(batch))) {
			break
		}
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	for _, task := range batch {
		wg.Add(1)
		go func(task llamaBatchTask) {
			defer wg.Done()
			select {
			case <-start:
			case <-task.ctx.Done():
				task.result <- llamaBatchResult{err: task.ctx.Err()}
				return
			}
			text, err := task.run(task.ctx)
			task.result <- llamaBatchResult{text: text, err: err}
		}(task)
	}
	close(start)
	wg.Wait()
}

func (r *LlamaCppRuntime) Ready() bool {
	reason, _ := r.readinessState()
	return reason == "ready"
}

func (r *LlamaCppRuntime) ReadinessReason() string {
	reason, _ := r.readinessState()
	return reason
}

func (r *LlamaCppRuntime) ReadinessDetail() string {
	_, detail := r.readinessState()
	return detail
}

func (r *LlamaCppRuntime) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	reason, _ := r.readinessState()
	if reason != "ready" {
		return nil, ErrNotReady
	}

	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return nil, fmt.Errorf("empty prompt")
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}
	if maxTokens > 1024 {
		maxTokens = 1024
	}

	temperature := req.Temperature
	if temperature <= 0 {
		temperature = llamaCppDefaultTemperature
	}

	runCtx := ctx
	cancel := func() {}
	if _, ok := runCtx.Deadline(); !ok {
		runCtx, cancel = context.WithTimeout(runCtx, r.timeout)
	}
	defer cancel()

	switch r.resolveBackend() {
	case LlamaCppModeCGO:
		if err := r.ensureCGOBackend(); err != nil {
			return nil, ErrNotReady
		}
		fallthrough
	case LlamaCppModeFFI:
		if err := r.ensureFFIBackend(); err != nil {
			return nil, ErrNotReady
		}
		fallthrough
	default:
		// server mode with CLI fallback.
		text, err := r.dispatchServerTask(runCtx, func(execCtx context.Context) (string, error) {
			return r.generateViaServer(execCtx, prompt, maxTokens, temperature, req.Images)
		})
		if err == nil && strings.TrimSpace(text) != "" {
			return &GenerateResponse{Text: strings.TrimSpace(text)}, nil
		}
		cliText, cliErr := r.runWithParallelSlot(runCtx, func(execCtx context.Context) (string, error) {
			return r.generateViaCLI(execCtx, prompt, maxTokens, temperature, req.Images)
		})
		if cliErr == nil && strings.TrimSpace(cliText) != "" {
			return &GenerateResponse{Text: strings.TrimSpace(cliText)}, nil
		}
		if errors.Is(cliErr, ErrNotReady) {
			return nil, ErrNotReady
		}
		return nil, fmt.Errorf("llama.cpp generation failed (server=%v, cli=%v)", err, cliErr)
	}
}

func (r *LlamaCppRuntime) Prefill(ctx context.Context, req PrefillRequest) (*PrefillResult, error) {
	reason, _ := r.readinessState()
	if reason != "ready" {
		return nil, ErrNotReady
	}
	if r.resolveBackend() != LlamaCppModeServer {
		return nil, ErrPrefixCachingUnsupported
	}
	if len(req.Images) > 0 {
		return nil, ErrPrefixCachingUnsupported
	}
	prefix := strings.TrimSpace(req.Prefix)
	if prefix == "" {
		return nil, fmt.Errorf("empty prefix")
	}

	runCtx := ctx
	cancel := func() {}
	if _, ok := runCtx.Deadline(); !ok {
		runCtx, cancel = context.WithTimeout(runCtx, r.timeout)
	}
	defer cancel()

	entry, cacheHit := r.upsertPrefixEntry(prefix, req.CacheKey, req.TTL)
	if _, err := r.dispatchServerTask(runCtx, func(execCtx context.Context) (string, error) {
		serverURL, err := r.ensureServer(execCtx)
		if err != nil {
			return "", err
		}
		return r.callServerCompletionWithOptions(execCtx, serverURL, prefix, 0, 0, completionCallOptions{
			CachePrompt: true,
			SlotID:      entry.SlotID,
		})
	}); err != nil {
		if cacheHit {
			r.prefixMisses.Add(1)
		}
		return nil, err
	}
	if cacheHit {
		r.prefixHits.Add(1)
	} else {
		r.prefixMisses.Add(1)
	}
	return r.prefillResultFromEntry(entry, cacheHit), nil
}

func (r *LlamaCppRuntime) GenerateFromPrefix(ctx context.Context, req GenerateFromPrefixRequest) (*GenerateResponse, error) {
	reason, _ := r.readinessState()
	if reason != "ready" {
		return nil, ErrNotReady
	}
	if r.resolveBackend() != LlamaCppModeServer {
		return nil, ErrPrefixCachingUnsupported
	}
	if len(req.Images) > 0 {
		return nil, ErrPrefixCachingUnsupported
	}
	entry, ok := r.lookupPrefixEntry(req.PrefixID, req.CacheKeyHint)
	if !ok {
		r.prefixMisses.Add(1)
		return nil, fmt.Errorf("prefix not found: %q", strings.TrimSpace(req.PrefixID))
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}
	if maxTokens > 1024 {
		maxTokens = 1024
	}
	temperature := req.Temperature
	if temperature <= 0 {
		temperature = llamaCppDefaultTemperature
	}
	prompt := entry.Prefix + req.Suffix

	runCtx := ctx
	cancel := func() {}
	if _, ok := runCtx.Deadline(); !ok {
		runCtx, cancel = context.WithTimeout(runCtx, r.timeout)
	}
	defer cancel()
	text, err := r.dispatchServerTask(runCtx, func(execCtx context.Context) (string, error) {
		serverURL, err := r.ensureServer(execCtx)
		if err != nil {
			return "", err
		}
		return r.callServerCompletionWithOptions(execCtx, serverURL, prompt, maxTokens, temperature, completionCallOptions{
			CachePrompt: true,
			SlotID:      entry.SlotID,
		})
	})
	if err == nil && strings.TrimSpace(text) != "" {
		r.touchPrefixEntry(entry.PrefixID)
		r.prefixHits.Add(1)
		return &GenerateResponse{Text: strings.TrimSpace(text)}, nil
	}

	// Fallback to the regular generation path so correctness is preserved even
	// if the backing llama-server does not support slot-based prompt caching.
	fallbackResp, fallbackErr := r.Generate(runCtx, GenerateRequest{
		Prompt:      prompt,
		MaxTokens:   maxTokens,
		Temperature: temperature,
	})
	if fallbackErr == nil {
		r.touchPrefixEntry(entry.PrefixID)
		r.prefixMisses.Add(1)
		return fallbackResp, nil
	}
	return nil, fmt.Errorf("generate from prefix failed (cached=%v, fallback=%v)", err, fallbackErr)
}

func (r *LlamaCppRuntime) EvictPrefix(prefixID string) bool {
	r.prefixMu.Lock()
	defer r.prefixMu.Unlock()
	entry, ok := r.prefixEntries[prefixID]
	if !ok {
		return false
	}
	delete(r.prefixEntries, prefixID)
	if entry.CacheKey != "" {
		delete(r.prefixByCache, entry.CacheKey)
	}
	r.prefixEvictions.Add(1)
	return true
}

func (r *LlamaCppRuntime) PrefixCacheStats() PrefixCacheStats {
	r.prefixMu.Lock()
	r.evictExpiredPrefixesLocked(time.Now())
	entries := len(r.prefixEntries)
	r.prefixMu.Unlock()
	return PrefixCacheStats{
		Supported: r != nil && r.resolveBackend() == LlamaCppModeServer,
		Entries:   entries,
		Hits:      r.prefixHits.Load(),
		Misses:    r.prefixMisses.Load(),
		Evictions: r.prefixEvictions.Load(),
	}
}

func (r *LlamaCppRuntime) BatchingStats() BatchingStats {
	if r == nil {
		return BatchingStats{}
	}
	if r.batcher == nil {
		return BatchingStats{
			Supported:    false,
			Window:       r.batchWindow,
			MaxBatchSize: r.batchMaxSize,
		}
	}
	return r.batcher.Stats()
}

func (r *LlamaCppRuntime) runWithParallelSlot(ctx context.Context, fn func(context.Context) (string, error)) (string, error) {
	select {
	case r.parallelSem <- struct{}{}:
		defer func() { <-r.parallelSem }()
	case <-ctx.Done():
		return "", ctx.Err()
	}
	return fn(ctx)
}

func (r *LlamaCppRuntime) dispatchServerTask(ctx context.Context, fn func(context.Context) (string, error)) (string, error) {
	if r == nil || r.batcher == nil {
		return r.runWithParallelSlot(ctx, fn)
	}
	return r.batcher.Do(ctx, func(execCtx context.Context) (string, error) {
		return r.runWithParallelSlot(execCtx, fn)
	})
}

type completionCallOptions struct {
	CachePrompt bool
	SlotID      int
}

func (r *LlamaCppRuntime) prefillResultFromEntry(entry *llamaPrefixEntry, cacheHit bool) *PrefillResult {
	if entry == nil {
		return nil
	}
	return &PrefillResult{
		PrefixID:     entry.PrefixID,
		PrefixTokens: entry.PrefixTokens,
		CacheHit:     cacheHit,
		Tier:         "memory",
		ExpiresAt:    entry.ExpiresAt,
	}
}

func (r *LlamaCppRuntime) upsertPrefixEntry(prefix, cacheKey string, ttl time.Duration) (*llamaPrefixEntry, bool) {
	now := time.Now()
	if ttl <= 0 {
		ttl = defaultPrefixCacheTTL
	}
	normalizedKey := normalizePrefixCacheKey(cacheKey, prefix)
	r.prefixMu.Lock()
	defer r.prefixMu.Unlock()
	r.evictExpiredPrefixesLocked(now)
	if prefixID, ok := r.prefixByCache[normalizedKey]; ok {
		if entry, exists := r.prefixEntries[prefixID]; exists && entry.Prefix == prefix {
			entry.ExpiresAt = now.Add(ttl)
			entry.LastUsedAt = now
			return entry, true
		}
	}
	prefixID := buildPrefixID(normalizedKey)
	entry := &llamaPrefixEntry{
		PrefixID:     prefixID,
		CacheKey:     normalizedKey,
		Prefix:       prefix,
		SlotID:       r.nextPrefixSlot,
		PrefixTokens: estimatePromptTokens(prefix),
		ExpiresAt:    now.Add(ttl),
		LastUsedAt:   now,
	}
	r.nextPrefixSlot++
	r.prefixEntries[prefixID] = entry
	r.prefixByCache[normalizedKey] = prefixID
	return entry, false
}

func (r *LlamaCppRuntime) lookupPrefixEntry(prefixID, cacheKeyHint string) (*llamaPrefixEntry, bool) {
	now := time.Now()
	r.prefixMu.Lock()
	defer r.prefixMu.Unlock()
	r.evictExpiredPrefixesLocked(now)
	id := strings.TrimSpace(prefixID)
	if id == "" {
		id = strings.TrimSpace(cacheKeyHint)
		if mapped, ok := r.prefixByCache[id]; ok {
			id = mapped
		}
	}
	if entry, ok := r.prefixEntries[id]; ok {
		entry.LastUsedAt = now
		return entry, true
	}
	if mapped, ok := r.prefixByCache[strings.TrimSpace(cacheKeyHint)]; ok {
		if entry, exists := r.prefixEntries[mapped]; exists {
			entry.LastUsedAt = now
			return entry, true
		}
	}
	return nil, false
}

func (r *LlamaCppRuntime) touchPrefixEntry(prefixID string) {
	r.prefixMu.Lock()
	defer r.prefixMu.Unlock()
	if entry, ok := r.prefixEntries[prefixID]; ok {
		entry.LastUsedAt = time.Now()
	}
}

func (r *LlamaCppRuntime) evictExpiredPrefixesLocked(now time.Time) {
	for prefixID, entry := range r.prefixEntries {
		if now.Before(entry.ExpiresAt) {
			continue
		}
		delete(r.prefixEntries, prefixID)
		if entry.CacheKey != "" {
			delete(r.prefixByCache, entry.CacheKey)
		}
		r.prefixEvictions.Add(1)
	}
}

func (r *LlamaCppRuntime) clearPrefixCacheLocked() {
	r.prefixMu.Lock()
	defer r.prefixMu.Unlock()
	for prefixID, entry := range r.prefixEntries {
		delete(r.prefixEntries, prefixID)
		if entry.CacheKey != "" {
			delete(r.prefixByCache, entry.CacheKey)
		}
	}
}

func normalizePrefixCacheKey(cacheKey, prefix string) string {
	key := strings.TrimSpace(cacheKey)
	if key != "" {
		return key
	}
	return "prefix:" + buildPrefixID(prefix)
}

func buildPrefixID(raw string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(sum[:8])
}

func estimatePromptTokens(prompt string) int {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return 0
	}
	byWords := len(strings.Fields(prompt))
	byRunes := len([]rune(prompt)) / 4
	if byRunes > byWords {
		return byRunes
	}
	if byWords == 0 {
		return 1
	}
	return byWords
}

func (r *LlamaCppRuntime) readinessState() (string, string) {
	if r == nil {
		return "runtime_nil", "small model runtime is nil"
	}
	if r.manager == nil {
		return "manager_nil", "small model manager is nil"
	}
	st := r.manager.GetStatus()
	if !st.Ready {
		if st.Downloading {
			return "model_downloading", "small model files are downloading"
		}
		if strings.TrimSpace(st.Error) != "" {
			return "model_unready", st.Error
		}
		return "model_files_missing_or_incomplete", "required small model files are missing"
	}
	if _, err := os.Stat(r.manager.ModelPath()); err != nil {
		return "model_file_missing", err.Error()
	}
	if mmprojPath := strings.TrimSpace(r.manager.MMProjPath()); mmprojPath != "" {
		if _, err := os.Stat(mmprojPath); err != nil {
			return "mmproj_file_missing", err.Error()
		}
	}

	switch r.resolveBackend() {
	case LlamaCppModeCGO:
		if err := r.ensureCGOBackend(); err != nil {
			return "llama_cpp_cgo_unavailable", err.Error()
		}
		fallthrough
	case LlamaCppModeFFI:
		if err := r.ensureFFIBackend(); err != nil {
			return "llama_cpp_ffi_unavailable", err.Error()
		}
		fallthrough
	default:
		if r.configuredServerURL != "" {
			if _, err := normalizeServerURL(r.configuredServerURL); err != nil {
				return "llama_cpp_server_url_invalid", err.Error()
			}
			return "ready", ""
		}
		if _, err := r.resolveServerBinary(); err != nil {
			if _, cliErr := r.resolveCLIPath(); cliErr != nil {
				return "llama_cpp_server_or_cli_not_found", fmt.Sprintf("%v; %v", err, cliErr)
			}
		}
		return "ready", ""
	}
}

func (r *LlamaCppRuntime) resolveBackend() LlamaCppMode {
	switch r.mode {
	case LlamaCppModeCGO:
		return LlamaCppModeCGO
	case LlamaCppModeFFI:
		return LlamaCppModeFFI
	case LlamaCppModeServer:
		return LlamaCppModeServer
	default:
		// Auto mode: prefer server mode today.
		return LlamaCppModeServer
	}
}

func normalizeLlamaCppMode(raw string) LlamaCppMode {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return ""
	case "auto":
		return LlamaCppModeAuto
	case "server":
		return LlamaCppModeServer
	case "cgo":
		return LlamaCppModeCGO
	case "ffi":
		return LlamaCppModeFFI
	default:
		return LlamaCppModeAuto
	}
}

func (r *LlamaCppRuntime) ensureCGOBackend() error {
	if r == nil {
		return fmt.Errorf("nil runtime")
	}
	r.cgoOnce.Do(func() {
		r.cgoErr = ensureLlamaCppCGOBackend()
	})
	return r.cgoErr
}

func (r *LlamaCppRuntime) ensureFFIBackend() error {
	if r == nil {
		return fmt.Errorf("nil runtime")
	}
	r.serverMu.Lock()
	if r.ffiBackend == nil {
		r.ffiBackend = newLlamaCppFFIBackend()
	}
	backend := r.ffiBackend
	r.serverMu.Unlock()
	return backend.EnsureLoaded()
}

func (r *LlamaCppRuntime) generateViaServer(
	ctx context.Context,
	prompt string,
	maxTokens int,
	temperature float64,
	images []ImageInput,
) (string, error) {
	serverURL, err := r.ensureServer(ctx)
	if err != nil {
		return "", err
	}

	text, err := r.callServerChatCompletions(ctx, serverURL, prompt, maxTokens, temperature, images)
	if err == nil && strings.TrimSpace(text) != "" {
		return strings.TrimSpace(text), nil
	}
	// Text-only fallback endpoint for older llama-server releases.
	if len(images) == 0 {
		fallbackText, fallbackErr := r.callServerCompletion(ctx, serverURL, prompt, maxTokens, temperature)
		if fallbackErr == nil && strings.TrimSpace(fallbackText) != "" {
			return strings.TrimSpace(fallbackText), nil
		}
		if err == nil {
			err = fallbackErr
		}
	}
	if err == nil {
		err = fmt.Errorf("empty response from llama-server")
	}
	return "", err
}

func (r *LlamaCppRuntime) generateViaCLI(
	ctx context.Context,
	prompt string,
	maxTokens int,
	temperature float64,
	images []ImageInput,
) (string, error) {
	cliPath, err := r.resolveCLIPath()
	if err != nil {
		return "", ErrNotReady
	}

	imagePaths, cleanup, err := prepareLlamaImageFiles(images)
	if err != nil {
		return "", err
	}
	defer cleanup()

	args := []string{
		"-m", r.manager.ModelPath(),
		"-n", strconv.Itoa(maxTokens),
		"--temp", strconv.FormatFloat(temperature, 'f', 3, 64),
		"--simple-io",
		"--no-display-prompt",
		"-p", prompt,
	}
	if mmprojPath := strings.TrimSpace(r.manager.MMProjPath()); mmprojPath != "" {
		args = append(args, "--mmproj", mmprojPath)
	}
	for _, imgPath := range imagePaths {
		args = append(args, "--image", imgPath)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, cliPath, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := compactSingleLine(stderr.String())
		if detail == "" {
			detail = compactSingleLine(stdout.String())
		}
		if detail == "" {
			detail = err.Error()
		}
		return "", fmt.Errorf("llama-cli run failed: %s", truncateText(detail, 512))
	}

	text := sanitizeLlamaOutput(prompt, stdout.String())
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("empty output from llama.cpp CLI")
	}
	return strings.TrimSpace(text), nil
}

func (r *LlamaCppRuntime) ensureServer(ctx context.Context) (string, error) {
	if r.configuredServerURL != "" {
		return normalizeServerURL(r.configuredServerURL)
	}

	r.serverMu.Lock()
	defer r.serverMu.Unlock()

	if r.serverURL != "" && r.isServerHealthy(r.serverURL) {
		return r.serverURL, nil
	}
	if r.serverURL != "" && !r.isServerHealthy(r.serverURL) {
		r.stopServerLocked()
	}

	binPath, err := r.resolveServerBinary()
	if err != nil {
		return "", err
	}

	host, port, err := resolveServerBindAddress()
	if err != nil {
		return "", err
	}
	serverURL := fmt.Sprintf("http://%s:%d", host, port)
	args := []string{
		"-m", r.manager.ModelPath(),
		"--host", host,
		"--port", strconv.Itoa(port),
	}
	if mmprojPath := strings.TrimSpace(r.manager.MMProjPath()); mmprojPath != "" {
		args = append(args, "--mmproj", mmprojPath)
	}
	args = append(args, r.configuredServerArg...)

	cmd := exec.CommandContext(context.Background(), binPath, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start llama-server: %w", err)
	}

	r.serverCmd = cmd
	r.serverURL = serverURL

	// Reap process and clear cached state when server exits.
	go func(proc *exec.Cmd, url string) {
		_ = proc.Wait()
		r.serverMu.Lock()
		defer r.serverMu.Unlock()
		if r.serverCmd == proc {
			r.serverCmd = nil
		}
		if r.serverURL == url {
			r.serverURL = ""
		}
		r.clearPrefixCacheLocked()
	}(cmd, serverURL)

	waitTimeout := r.serverStartup
	if waitTimeout <= 0 {
		waitTimeout = llamaServerStartupTimeout
	}
	waitCtx := ctx
	if _, ok := waitCtx.Deadline(); !ok {
		var cancel context.CancelFunc
		waitCtx, cancel = context.WithTimeout(context.Background(), waitTimeout)
		defer cancel()
	}
	if err := r.waitServerHealthy(waitCtx, serverURL); err != nil {
		r.stopServerLocked()
		return "", err
	}

	return serverURL, nil
}

func (r *LlamaCppRuntime) stopServerLocked() {
	if r.serverCmd != nil && r.serverCmd.Process != nil {
		_ = r.serverCmd.Process.Kill()
	}
	r.serverCmd = nil
	r.serverURL = ""
	r.clearPrefixCacheLocked()
}

func (r *LlamaCppRuntime) waitServerHealthy(ctx context.Context, serverURL string) error {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		if r.isServerHealthy(serverURL) {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("llama-server startup timeout: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func (r *LlamaCppRuntime) isServerHealthy(serverURL string) bool {
	healthURL := strings.TrimRight(serverURL, "/") + "/health"
	if r.getOK(healthURL) {
		return true
	}
	modelsURL := strings.TrimRight(serverURL, "/") + "/v1/models"
	return r.getOK(modelsURL)
}

func (r *LlamaCppRuntime) getOK(rawURL string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return false
	}
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func (r *LlamaCppRuntime) callServerChatCompletions(
	ctx context.Context,
	serverURL, prompt string,
	maxTokens int,
	temperature float64,
	images []ImageInput,
) (string, error) {
	type reqMessage struct {
		Role    string      `json:"role"`
		Content interface{} `json:"content"`
	}
	type reqBody struct {
		Model       string       `json:"model"`
		Messages    []reqMessage `json:"messages"`
		MaxTokens   int          `json:"max_tokens"`
		Temperature float64      `json:"temperature"`
		Stream      bool         `json:"stream"`
	}

	content := interface{}(prompt)
	if len(images) > 0 {
		parts := make([]map[string]interface{}, 0, 1+len(images))
		parts = append(parts, map[string]interface{}{
			"type": "text",
			"text": prompt,
		})
		for _, img := range images {
			mime := strings.TrimSpace(img.MimeType)
			if mime == "" {
				mime = "image/png"
			}
			b64 := strings.TrimSpace(img.Data)
			if strings.HasPrefix(strings.ToLower(b64), "data:") {
				if idx := strings.Index(b64, ","); idx >= 0 {
					b64 = b64[idx+1:]
				}
			}
			parts = append(parts, map[string]interface{}{
				"type": "image_url",
				"image_url": map[string]string{
					"url": "data:" + mime + ";base64," + b64,
				},
			})
		}
		content = parts
	}

	payload := reqBody{
		Model: "local",
		Messages: []reqMessage{{
			Role:    "user",
			Content: content,
		}},
		MaxTokens:   maxTokens,
		Temperature: temperature,
		Stream:      false,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	rawURL := strings.TrimRight(serverURL, "/") + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respData, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("chat completions status=%d body=%s", resp.StatusCode, truncateText(compactSingleLine(string(respData)), 240))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respData, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("chat completions returned no choices")
	}

	return extractContentFromRawJSON(parsed.Choices[0].Message.Content), nil
}

func (r *LlamaCppRuntime) callServerCompletion(
	ctx context.Context,
	serverURL, prompt string,
	maxTokens int,
	temperature float64,
) (string, error) {
	return r.callServerCompletionWithOptions(ctx, serverURL, prompt, maxTokens, temperature, completionCallOptions{})
}

func (r *LlamaCppRuntime) callServerCompletionWithOptions(
	ctx context.Context,
	serverURL, prompt string,
	maxTokens int,
	temperature float64,
	options completionCallOptions,
) (string, error) {
	payload := map[string]interface{}{
		"prompt":      prompt,
		"n_predict":   maxTokens,
		"temperature": temperature,
		"stream":      false,
	}
	if options.CachePrompt {
		payload["cache_prompt"] = true
	}
	if options.SlotID > 0 {
		payload["id_slot"] = options.SlotID
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	rawURL := strings.TrimRight(serverURL, "/") + "/completion"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respData, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("completion status=%d body=%s", resp.StatusCode, truncateText(compactSingleLine(string(respData)), 240))
	}

	var parsed struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(respData, &parsed); err != nil {
		return "", err
	}
	return strings.TrimSpace(parsed.Content), nil
}

func extractContentFromRawJSON(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return ""
	}

	var plain string
	if err := json.Unmarshal(raw, &plain); err == nil {
		return strings.TrimSpace(plain)
	}

	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err == nil {
		var sb strings.Builder
		for _, p := range parts {
			if strings.TrimSpace(p.Text) == "" {
				continue
			}
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(strings.TrimSpace(p.Text))
		}
		return strings.TrimSpace(sb.String())
	}

	var obj struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil {
		return strings.TrimSpace(obj.Text)
	}
	return ""
}

func (r *LlamaCppRuntime) resolveCLIPath() (string, error) {
	r.resolveMu.RLock()
	if r.resolvedCLI != "" {
		defer r.resolveMu.RUnlock()
		return r.resolvedCLI, nil
	}
	r.resolveMu.RUnlock()

	r.resolveMu.Lock()
	defer r.resolveMu.Unlock()
	if r.resolvedCLI != "" {
		return r.resolvedCLI, nil
	}

	candidates := make([]string, 0, 4)
	if cli := strings.TrimSpace(r.configuredCLI); cli != "" {
		candidates = append(candidates, cli)
	}
	candidates = append(candidates, "llama-cli", "llama.cpp-cli", "main")

	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		if p, err := exec.LookPath(candidate); err == nil {
			r.resolvedCLI = p
			return p, nil
		}
	}
	return "", fmt.Errorf("llama.cpp CLI not found in PATH (expected `llama-cli`; optional env SMALL_MODEL_LLAMA_CPP_CLI)")
}

func (r *LlamaCppRuntime) resolveServerBinary() (string, error) {
	r.resolveMu.RLock()
	if r.resolvedServerBin != "" {
		defer r.resolveMu.RUnlock()
		return r.resolvedServerBin, nil
	}
	r.resolveMu.RUnlock()

	r.resolveMu.Lock()
	defer r.resolveMu.Unlock()
	if r.resolvedServerBin != "" {
		return r.resolvedServerBin, nil
	}
	if r.resolveErrorCached != nil {
		return "", r.resolveErrorCached
	}

	candidates := make([]string, 0, 4)
	if bin := strings.TrimSpace(r.configuredServerBin); bin != "" {
		candidates = append(candidates, bin)
	}
	candidates = append(candidates, "llama-server", "llama.cpp-server", "server")

	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		if p, err := exec.LookPath(candidate); err == nil {
			r.resolvedServerBin = p
			return p, nil
		}
	}
	r.resolveErrorCached = fmt.Errorf("llama-server binary not found in PATH (set SMALL_MODEL_LLAMA_SERVER_BIN)")
	return "", r.resolveErrorCached
}

func resolveServerBindAddress() (string, int, error) {
	host := strings.TrimSpace(os.Getenv("SMALL_MODEL_LLAMA_SERVER_HOST"))
	if host == "" {
		host = "127.0.0.1"
	}
	if rawPort := strings.TrimSpace(os.Getenv("SMALL_MODEL_LLAMA_SERVER_PORT")); rawPort != "" {
		port, err := strconv.Atoi(rawPort)
		if err != nil || port <= 0 || port > 65535 {
			return "", 0, fmt.Errorf("invalid SMALL_MODEL_LLAMA_SERVER_PORT: %q", rawPort)
		}
		return host, port, nil
	}
	port, err := reservePort(host)
	if err != nil {
		return "", 0, err
	}
	return host, port, nil
}

func reservePort(host string) (int, error) {
	ln, err := net.Listen("tcp", net.JoinHostPort(host, "0"))
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok || addr.Port <= 0 {
		return 0, fmt.Errorf("failed to reserve ephemeral port")
	}
	return addr.Port, nil
}

func normalizeServerURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("empty llama server url")
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return "", err
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid llama server url: %s", trimmed)
	}
	return strings.TrimRight(u.String(), "/"), nil
}

func prepareLlamaImageFiles(images []ImageInput) ([]string, func(), error) {
	if len(images) == 0 {
		return nil, func() {}, nil
	}

	dir, err := os.MkdirTemp("", "smallmodel-llama-images-*")
	if err != nil {
		return nil, nil, fmt.Errorf("create temp dir for images: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(dir) }

	paths := make([]string, 0, len(images))
	for i, img := range images {
		payload, decodeErr := decodeBase64PayloadLlama(img.Data)
		if decodeErr != nil {
			cleanup()
			return nil, nil, fmt.Errorf("decode image payload: %w", decodeErr)
		}
		ext := imageExtForMime(img.MimeType)
		path := filepath.Join(dir, fmt.Sprintf("image-%02d%s", i, ext))
		if writeErr := os.WriteFile(path, payload, 0o600); writeErr != nil {
			cleanup()
			return nil, nil, fmt.Errorf("write temp image: %w", writeErr)
		}
		paths = append(paths, path)
	}
	return paths, cleanup, nil
}

func decodeBase64PayloadLlama(raw string) ([]byte, error) {
	payload := strings.TrimSpace(raw)
	if strings.HasPrefix(strings.ToLower(payload), "data:") {
		if idx := strings.Index(payload, ","); idx >= 0 {
			payload = payload[idx+1:]
		}
	}
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return nil, fmt.Errorf("empty image payload")
	}
	if b, err := base64.StdEncoding.DecodeString(payload); err == nil {
		return b, nil
	}
	b, err := base64.RawStdEncoding.DecodeString(payload)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func imageExtForMime(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ".png"
	}
}

func sanitizeLlamaOutput(prompt, raw string) string {
	out := strings.TrimSpace(raw)
	if out == "" {
		return ""
	}
	trimPrompt := strings.TrimSpace(prompt)
	if trimPrompt != "" && strings.HasPrefix(out, trimPrompt) {
		out = strings.TrimSpace(strings.TrimPrefix(out, trimPrompt))
	}
	if idx := strings.Index(strings.ToLower(out), "\nassistant:"); idx >= 0 {
		out = strings.TrimSpace(out[idx+len("\nassistant:"):])
	}
	if strings.HasPrefix(strings.ToLower(out), "assistant:") {
		out = strings.TrimSpace(out[len("assistant:"):])
	}
	return out
}

func compactSingleLine(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

func truncateText(s string, maxLen int) string {
	if maxLen <= 0 || len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
