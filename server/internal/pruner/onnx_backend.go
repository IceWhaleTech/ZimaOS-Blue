package pruner

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/onnx"
	ort "github.com/yalue/onnxruntime_go"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// ErrOnnxNotReady is returned when the ONNX model is still loading.
// Callers should fall back to local/IR pruning or passthrough.
var ErrOnnxNotReady = errors.New("onnx pruner model not ready")

// OnnxBackend implements Backend using ONNX Runtime for local neural pruning.
// Model loading is async — Prune() returns ErrOnnxNotReady until loaded.
type OnnxBackend struct {
	session       *onnx.Session
	tokenizer     *BPETokenizer
	config        Config
	modelDir      string
	maxLen        int
	mu            sync.Mutex
	inputIDs      *ort.Tensor[int64]
	attentionMask *ort.Tensor[int64]
	outputScores  *ort.Tensor[float32]
	ready         atomic.Bool
	initErr       error
	startOnce     sync.Once
}

// NewOnnxBackend creates a new ONNX-based pruning backend.
// Model loading is deferred — call StartAsync() or it auto-starts on first Prune().
// modelDir should contain: model.onnx, vocab.json, merges.txt
func NewOnnxBackend(cfg Config, modelDir string) (*OnnxBackend, error) {
	return &OnnxBackend{
		config:   cfg,
		modelDir: modelDir,
		maxLen:   4096,
	}, nil
}

// StartAsync begins model loading in the background.
func (b *OnnxBackend) StartAsync() {
	b.startOnce.Do(func() {
		go b.loadModel()
	})
}

// IsReady returns true if the ONNX model is loaded and ready for inference.
func (b *OnnxBackend) IsReady() bool {
	return b.ready.Load()
}

// loadModel downloads and loads the ONNX model synchronously (called from goroutine).
func (b *OnnxBackend) loadModel() {
	// Ensure ONNX Runtime library path is set (data dir is parent of model dir)
	dataPath := filepath.Dir(b.modelDir)
	onnx.SetDataDir(dataPath)
	if libPath := onnx.RuntimeLibPath(dataPath); libPath != "" {
		onnx.SetLibraryPath(libPath)
	}

	vocabPath := filepath.Join(b.modelDir, "vocab.json")
	mergesPath := filepath.Join(b.modelDir, "merges.txt")

	tok, err := LoadBPETokenizer(vocabPath, mergesPath)
	if err != nil {
		b.mu.Lock()
		b.initErr = fmt.Errorf("load tokenizer: %w", err)
		b.mu.Unlock()
		return
	}

	modelPath := filepath.Join(b.modelDir, "model.onnx")

	// Create input/output tensors
	inputShape := ort.NewShape(1, int64(b.maxLen))
	inputIDs, err := ort.NewEmptyTensor[int64](inputShape)
	if err != nil {
		b.mu.Lock()
		b.initErr = fmt.Errorf("create input_ids tensor: %w", err)
		b.mu.Unlock()
		return
	}
	attentionMask, err := ort.NewEmptyTensor[int64](inputShape)
	if err != nil {
		inputIDs.Destroy()
		b.mu.Lock()
		b.initErr = fmt.Errorf("create attention_mask tensor: %w", err)
		b.mu.Unlock()
		return
	}

	outputShape := ort.NewShape(1, int64(b.maxLen))
	outputScores, err := ort.NewEmptyTensor[float32](outputShape)
	if err != nil {
		inputIDs.Destroy()
		attentionMask.Destroy()
		b.mu.Lock()
		b.initErr = fmt.Errorf("create output tensor: %w", err)
		b.mu.Unlock()
		return
	}

	session, err := onnx.NewSession(
		modelPath,
		[]string{"input_ids", "attention_mask"},
		[]string{"token_scores"},
		[]ort.ArbitraryTensor{inputIDs, attentionMask},
		[]ort.ArbitraryTensor{outputScores},
	)
	if err != nil {
		inputIDs.Destroy()
		attentionMask.Destroy()
		outputScores.Destroy()
		b.mu.Lock()
		b.initErr = fmt.Errorf("create ONNX session: %w", err)
		b.mu.Unlock()
		return
	}

	b.mu.Lock()
	b.tokenizer = tok
	b.session = session
	b.inputIDs = inputIDs
	b.attentionMask = attentionMask
	b.outputScores = outputScores
	b.mu.Unlock()
	b.ready.Store(true)
}

// Prune implements Backend using ONNX neural inference.
// Returns ErrOnnxNotReady if the model is still loading.
func (b *OnnxBackend) Prune(ctx context.Context, req PruneRequest) (*PruneResponse, error) {
	// Auto-start on first call if not already started
	b.StartAsync()

	if !b.ready.Load() {
		b.mu.Lock()
		initErr := b.initErr
		b.mu.Unlock()
		if initErr != nil {
			return nil, initErr
		}
		return nil, ErrOnnxNotReady
	}

	start := timeutil.NowTime()

	content := req.GetContent()
	if strings.TrimSpace(content) == "" {
		return &PruneResponse{PrunedContent: "", PrunedCode: ""}, nil
	}

	origLines := strings.Split(content, "\n")
	origTokens := EstimateTokens(content)

	threshold := req.Threshold
	if threshold <= 0 {
		threshold = b.config.Threshold
	}

	// Tokenize and build input
	inputIDs, attentionMask, codeStart, codeEnd := b.tokenizer.BuildPrunerInput(req.Query, content, b.maxLen)

	// Run inference (session is not thread-safe)
	b.mu.Lock()
	scores, err := b.runInference(inputIDs, attentionMask)
	b.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("onnx inference: %w", err)
	}

	// Extract code token scores and aggregate to line level
	lineScores := b.aggregateToLines(content, scores, codeStart, codeEnd)

	// Select lines above threshold
	var builder strings.Builder
	builder.Grow(len(content) / 2)
	keptLines := 0
	lastKept := -1

	for i, score := range lineScores {
		if score >= threshold {
			gap := i - lastKept - 1
			if gap > 0 {
				fmt.Fprintf(&builder, "(filtered %d lines)\n", gap)
			}
			if i < len(origLines) {
				builder.WriteString(origLines[i])
				builder.WriteByte('\n')
			}
			keptLines++
			lastKept = i
		}
	}
	if lastKept < len(origLines)-1 {
		gap := len(origLines) - 1 - lastKept
		if gap > 0 {
			fmt.Fprintf(&builder, "(filtered %d lines)\n", gap)
		}
	}

	prunedContent := builder.String()
	prunedTokens := EstimateTokens(prunedContent)
	var compressionRate float64
	if origTokens > 0 {
		compressionRate = float64(prunedTokens) / float64(origTokens)
	}

	return &PruneResponse{
		PrunedContent:   prunedContent,
		PrunedCode:      prunedContent,
		ContentType:     ContentCode,
		Score:           1.0,
		OriginalLines:   len(origLines),
		KeptLines:       keptLines,
		PrunedLines:     len(origLines) - keptLines,
		OriginalTokens:  origTokens,
		PrunedTokens:    prunedTokens,
		CompressionRate: compressionRate,
		LatencyMs:       float64(timeutil.SinceTime(start).Microseconds()) / 1000.0,
	}, nil
}

// runInference copies data into pre-allocated tensors and runs the ONNX session.
func (b *OnnxBackend) runInference(inputIDs, attentionMask []int64) ([]float32, error) {
	copy(b.inputIDs.GetData(), inputIDs)
	copy(b.attentionMask.GetData(), attentionMask)

	if err := b.session.Run(); err != nil {
		return nil, err
	}

	data := b.outputScores.GetData()
	result := make([]float32, len(data))
	copy(result, data)
	return result, nil
}

// aggregateToLines maps token-level scores to line-level scores.
func (b *OnnxBackend) aggregateToLines(content string, scores []float32, codeStart, codeEnd int) []float64 {
	lines := strings.Split(content, "\n")
	lineScores := make([]float64, len(lines))

	if codeStart >= codeEnd || codeEnd > len(scores) {
		// Fallback: keep all lines
		for i := range lineScores {
			lineScores[i] = 1.0
		}
		return lineScores
	}

	// Map code tokens back to lines by re-encoding each line
	tokenIdx := codeStart
	for lineIdx, line := range lines {
		lineTokens := b.tokenizer.Encode(line + "\n")
		if len(lineTokens) == 0 {
			lineScores[lineIdx] = 0.5 // empty lines get neutral score
			continue
		}

		var sum float64
		count := 0
		for range lineTokens {
			if tokenIdx < codeEnd && tokenIdx < len(scores) {
				sum += float64(scores[tokenIdx])
				count++
				tokenIdx++
			}
		}
		if count > 0 {
			lineScores[lineIdx] = sum / float64(count)
		}
	}
	return lineScores
}

// Health checks if the ONNX session is loaded.
func (b *OnnxBackend) Health(_ context.Context) error {
	if !b.ready.Load() {
		b.mu.Lock()
		initErr := b.initErr
		b.mu.Unlock()
		if initErr != nil {
			return initErr
		}
		return ErrOnnxNotReady
	}
	return nil
}

// Close releases ONNX runtime resources.
func (b *OnnxBackend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.session != nil {
		b.session.Destroy()
	}
	if b.inputIDs != nil {
		b.inputIDs.Destroy()
	}
	if b.attentionMask != nil {
		b.attentionMask.Destroy()
	}
	if b.outputScores != nil {
		b.outputScores.Destroy()
	}
	return nil
}
