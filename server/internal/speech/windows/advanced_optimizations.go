//go:build windows

package windows

import (
	"context"
	"sync"
	"time"
	"unsafe"
)

// StreamPool manages reusable ISpStream objects for TTS
type StreamPool struct {
	pool chan unsafe.Pointer
	size int
	mu   sync.Mutex
}

// NewStreamPool creates a new stream pool
func NewStreamPool(size int) *StreamPool {
	return &StreamPool{
		pool: make(chan unsafe.Pointer, size),
		size: size,
	}
}

// Acquire gets a stream from the pool or creates a new one
func (p *StreamPool) Acquire() unsafe.Pointer {
	select {
	case stream := <-p.pool:
		return stream
	default:
		// Pool is empty, caller will create new stream
		return nil
	}
}

// Release returns a stream to the pool
func (p *StreamPool) Release(stream unsafe.Pointer) {
	if stream == nil {
		return
	}
	select {
	case p.pool <- stream:
		// Successfully returned to pool
	default:
		// Pool is full, let it be garbage collected
		// In C++ side, stream will be released
	}
}

// Close releases all pooled streams
func (p *StreamPool) Close() {
	close(p.pool)
	for stream := range p.pool {
		if stream != nil {
			// Release stream in C++ side
			_ = stream
		}
	}
}

// TTSCache caches frequently used synthesis results
type TTSCache struct {
	cache map[string]*CachedAudio
	mu    sync.RWMutex
	maxSize int
	ttl   time.Duration
}

type CachedAudio struct {
	Audio      []byte
	SampleRate int
	Timestamp  time.Time
}

// NewTTSCache creates a new TTS cache
func NewTTSCache(maxSize int, ttl time.Duration) *TTSCache {
	cache := &TTSCache{
		cache:   make(map[string]*CachedAudio),
		maxSize: maxSize,
		ttl:     ttl,
	}
	// Start cleanup goroutine
	go cache.cleanup()
	return cache
}

// Get retrieves cached audio
func (c *TTSCache) Get(key string) (*CachedAudio, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	audio, ok := c.cache[key]
	if !ok {
		return nil, false
	}

	// Check if expired
	if time.Since(audio.Timestamp) > c.ttl {
		return nil, false
	}

	return audio, true
}

// Set stores audio in cache
func (c *TTSCache) Set(key string, audio []byte, sampleRate int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict oldest if cache is full
	if len(c.cache) >= c.maxSize {
		var oldestKey string
		var oldestTime time.Time
		first := true

		for k, v := range c.cache {
			if first || v.Timestamp.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.Timestamp
				first = false
			}
		}

		if oldestKey != "" {
			delete(c.cache, oldestKey)
		}
	}

	c.cache[key] = &CachedAudio{
		Audio:      audio,
		SampleRate: sampleRate,
		Timestamp:  time.Now(),
	}
}

// cleanup removes expired entries
func (c *TTSCache) cleanup() {
	ticker := time.NewTicker(c.ttl / 2)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, audio := range c.cache {
			if now.Sub(audio.Timestamp) > c.ttl {
				delete(c.cache, key)
			}
		}
		c.mu.Unlock()
	}
}

// WarmupManager handles preloading of resources
type WarmupManager struct {
	ttsProvider *WindowsTTSProvider
	asrProvider *WindowsASRProvider
	warmedUp    bool
	mu          sync.Mutex
}

// NewWarmupManager creates a new warmup manager
func NewWarmupManager() *WarmupManager {
	return &WarmupManager{}
}

// Warmup preloads resources in background
func (w *WarmupManager) Warmup(ctx context.Context) error {
	w.mu.Lock()
	if w.warmedUp {
		w.mu.Unlock()
		return nil
	}
	w.mu.Unlock()

	// Warmup TTS
	go func() {
		provider := NewWindowsTTSProvider()
		if provider != nil {
			// Synthesize a short test phrase to warm up the engine
			req := &SynthesizeRequest{
				Text:   "warmup",
				Speed:  1.0,
				Volume: 0.01, // Very low volume
			}
			_, _ = provider.Synthesize(context.Background(), req)
			provider.Close()
		}
	}()

	// Warmup ASR
	go func() {
		provider := NewWindowsASRProvider("en-US")
		if provider != nil {
			// Grammar is already preloaded in asr_create
			provider.Close()
		}
	}()

	w.mu.Lock()
	w.warmedUp = true
	w.mu.Unlock()

	return nil
}

// AsyncTTSRequest represents an async TTS request
type AsyncTTSRequest struct {
	Request  *SynthesizeRequest
	Response chan *AsyncTTSResponse
}

// AsyncTTSResponse represents an async TTS response
type AsyncTTSResponse struct {
	Audio []byte
	Error error
}

// AsyncTTSProcessor processes TTS requests asynchronously
type AsyncTTSProcessor struct {
	provider  *WindowsTTSProvider
	queue     chan *AsyncTTSRequest
	workers   int
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// NewAsyncTTSProcessor creates a new async TTS processor
func NewAsyncTTSProcessor(workers int) *AsyncTTSProcessor {
	ctx, cancel := context.WithCancel(context.Background())

	processor := &AsyncTTSProcessor{
		provider: NewWindowsTTSProvider(),
		queue:    make(chan *AsyncTTSRequest, workers*2),
		workers:  workers,
		ctx:      ctx,
		cancel:   cancel,
	}

	// Start worker goroutines
	for i := 0; i < workers; i++ {
		processor.wg.Add(1)
		go processor.worker()
	}

	return processor
}

// worker processes requests from the queue
func (p *AsyncTTSProcessor) worker() {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			return
		case req := <-p.queue:
			if req == nil {
				return
			}

			resp, err := p.provider.Synthesize(p.ctx, req.Request)

			result := &AsyncTTSResponse{Error: err}
			if resp != nil && resp.Audio != nil {
				result.Audio = resp.Audio
			}

			// Send response
			select {
			case req.Response <- result:
			case <-p.ctx.Done():
				return
			}
		}
	}
}

// Submit submits a TTS request for async processing
func (p *AsyncTTSProcessor) Submit(req *SynthesizeRequest) <-chan *AsyncTTSResponse {
	respChan := make(chan *AsyncTTSResponse, 1)

	asyncReq := &AsyncTTSRequest{
		Request:  req,
		Response: respChan,
	}

	select {
	case p.queue <- asyncReq:
	case <-p.ctx.Done():
		respChan <- &AsyncTTSResponse{Error: context.Canceled}
	}

	return respChan
}

// Close shuts down the processor
func (p *AsyncTTSProcessor) Close() {
	p.cancel()
	close(p.queue)
	p.wg.Wait()
	if p.provider != nil {
		p.provider.Close()
	}
}
