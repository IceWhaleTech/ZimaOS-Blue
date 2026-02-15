//go:build onnx

package pruner

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	ort "github.com/yalue/onnxruntime_go"
)

// OnnxBackend implements Backend using ONNX Runtime for local neural pruning.
type OnnxBackend struct {
	session   *ort.AdvancedSession
	tokenizer *BPETokenizer
	config    Config
	maxLen    int
	mu        sync.Mutex
}

// NewOnnxBackend creates a new ONNX-based pruning backend.
// modelDir should contain: model.onnx, vocab.json, merges.txt
func NewOnnxBackend(cfg Config, modelDir string) (*OnnxBackend, error) {
	vocabPath := filepath.Join(modelDir, "vocab.json")
	mergesPath := filepath.Join(modelDir, "merges.txt")

	tok, err := LoadBPETokenizer(vocabPath, mergesPath)
	if err != nil {
		return nil, fmt.Errorf("load tokenizer: %w", err)
	}

	modelPath := filepath.Join(modelDir, "model.onnx")
	maxLen := 4096

	// Create input/output tensors
	inputShape := ort.NewShape(1, int64(maxLen))
	inputIDs, err := ort.NewEmptyTensor[int64](inputShape)
	if err != nil {
		return nil, fmt.Errorf("create input_ids tensor: %w", err)
	}
	attentionMask, err := ort.NewEmptyTensor[int64](inputShape)
	if err != nil {
		inputIDs.Destroy()
		return nil, fmt.Errorf("create attention_mask tensor: %w", err)
	}

	outputShape := ort.NewShape(1, int64(maxLen))
	outputScores, err := ort.NewEmptyTensor[float32](outputShape)
	if err != nil {
		inputIDs.Destroy()
		attentionMask.Destroy()
		return nil, fmt.Errorf("create output tensor: %w", err)
	}

	session, err := ort.NewAdvancedSession(
		modelPath,
		[]string{"input_ids", "attention_mask"},
		[]string{"token_scores"},
		[]ort.ArbitraryTensor{inputIDs, attentionMask},
		[]ort.ArbitraryTensor{outputScores},
		nil,
	)
	if err != nil {
		inputIDs.Destroy()
		attentionMask.Destroy()
		outputScores.Destroy()
		return nil, fmt.Errorf("create ONNX session: %w", err)
	}

	return &OnnxBackend{
		session:   session,
		tokenizer: tok,
		config:    cfg,
		maxLen:    maxLen,
	}, nil
}

// Prune implements Backend using ONNX neural inference.
func (b *OnnxBackend) Prune(ctx context.Context, req PruneRequest) (*PruneResponse, error) {
	start := time.Now()

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
		LatencyMs:       float64(time.Since(start).Microseconds()) / 1000.0,
	}, nil
}

// runInference copies data into pre-allocated tensors and runs the ONNX session.
func (b *OnnxBackend) runInference(inputIDs, attentionMask []int64) ([]float32, error) {
	// Get the underlying tensor data slices and copy input data
	inputs := b.session.Inputs()
	if len(inputs) < 2 {
		return nil, fmt.Errorf("expected 2 inputs, got %d", len(inputs))
	}

	idsTensor, ok := inputs[0].(*ort.Tensor[int64])
	if !ok {
		return nil, fmt.Errorf("input_ids tensor type mismatch")
	}
	maskTensor, ok := inputs[1].(*ort.Tensor[int64])
	if !ok {
		return nil, fmt.Errorf("attention_mask tensor type mismatch")
	}

	copy(idsTensor.GetData(), inputIDs)
	copy(maskTensor.GetData(), attentionMask)

	if err := b.session.Run(); err != nil {
		return nil, err
	}

	outputs := b.session.Outputs()
	if len(outputs) < 1 {
		return nil, fmt.Errorf("no outputs from session")
	}
	scoresTensor, ok := outputs[0].(*ort.Tensor[float32])
	if !ok {
		return nil, fmt.Errorf("output tensor type mismatch")
	}

	data := scoresTensor.GetData()
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
	if b.session == nil {
		return fmt.Errorf("onnx session not loaded")
	}
	return nil
}

// Close releases ONNX runtime resources.
func (b *OnnxBackend) Close() error {
	if b.session != nil {
		b.session.Destroy()
	}
	return nil
}
