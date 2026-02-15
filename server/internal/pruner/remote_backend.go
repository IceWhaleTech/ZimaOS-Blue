package pruner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// remoteRequest matches the SWE-Pruner FastAPI /prune endpoint.
type remoteRequest struct {
	Code      string  `json:"code"`
	Query     string  `json:"query,omitempty"`
	Threshold float64 `json:"threshold,omitempty"`
}

// remoteResponse matches the SWE-Pruner FastAPI /prune response.
type remoteResponse struct {
	Score      float64     `json:"score"`
	PrunedCode string      `json:"pruned_code"`
	KeptFrags  []int       `json:"kept_frags"`
	TokenScores [][]interface{} `json:"token_scores,omitempty"`
}

// RemoteBackend calls an external SWE-Pruner FastAPI service.
type RemoteBackend struct {
	client  *http.Client
	baseURL string
}

// NewRemoteBackend creates a new remote backend.
func NewRemoteBackend(baseURL string, client *http.Client) *RemoteBackend {
	return &RemoteBackend{
		client:  client,
		baseURL: strings.TrimRight(baseURL, "/"),
	}
}

// Prune sends code to the remote SWE-Pruner service and returns pruned output.
func (r *RemoteBackend) Prune(ctx context.Context, req PruneRequest) (*PruneResponse, error) {
	start := time.Now()

	body, err := json.Marshal(remoteRequest{
		Code:      req.Code,
		Query:     req.Query,
		Threshold: req.Threshold,
	})
	if err != nil {
		return nil, fmt.Errorf("pruner: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, r.baseURL+"/prune", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("pruner: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("pruner: remote call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("pruner: remote returned %d: %s", resp.StatusCode, string(respBody))
	}

	var rr remoteResponse
	if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
		return nil, fmt.Errorf("pruner: decode response: %w", err)
	}

	originalLines := strings.Count(req.Code, "\n") + 1
	prunedLines := strings.Count(rr.PrunedCode, "\n") + 1
	keptLines := len(rr.KeptFrags)
	if keptLines == 0 {
		keptLines = prunedLines
	}

	origTokens := EstimateTokens(req.Code)
	prunedTokens := EstimateTokens(rr.PrunedCode)
	var compressionRate float64
	if origTokens > 0 {
		compressionRate = float64(prunedTokens) / float64(origTokens)
	}

	return &PruneResponse{
		PrunedCode:      rr.PrunedCode,
		Score:           rr.Score,
		OriginalLines:   originalLines,
		KeptLines:       keptLines,
		PrunedLines:     originalLines - keptLines,
		OriginalTokens:  origTokens,
		PrunedTokens:    prunedTokens,
		CompressionRate: compressionRate,
		LatencyMs:       float64(time.Since(start).Microseconds()) / 1000.0,
	}, nil
}

// Health checks if the remote SWE-Pruner service is available.
func (r *RemoteBackend) Health(ctx context.Context) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, r.baseURL+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := r.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("pruner: health check failed: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("pruner: health check returned %d", resp.StatusCode)
	}
	return nil
}

// Close releases resources (no-op for remote backend).
func (r *RemoteBackend) Close() error {
	return nil
}
