package mediagen

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// MuleRouterProvider implements MediaProvider using MuleRouter's unified API.
// Supports OpenAI-compatible image gen + vendor-specific endpoints for Qwen/Wan2.
type MuleRouterProvider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewMuleRouterProvider creates a new MuleRouter media provider.
func NewMuleRouterProvider(apiKey, baseURL string) *MuleRouterProvider {
	return &MuleRouterProvider{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

func (p *MuleRouterProvider) Name() string { return "mulerouter" }

func (p *MuleRouterProvider) SupportedModels() []MediaModelInfo {
	return []MediaModelInfo{
		// OpenAI vendor — image
		{ID: "dall-e-3", Name: "DALL-E 3", Type: MediaTypeImage, Provider: "mulerouter"},
		// Alibaba vendor — image
		{ID: "qwen-image-max", Name: "Qwen Image Max", Type: MediaTypeImage, Provider: "mulerouter"},
		{ID: "qwen-image-edit-max", Name: "Qwen Image Edit Max", Type: MediaTypeImage, Provider: "mulerouter"},
		{ID: "nano-banana-pro", Name: "Nano Banana Pro", Type: MediaTypeImage, Provider: "mulerouter"},
		// Alibaba vendor — video
		{ID: "wan2.6-t2v", Name: "Wan2 Text-to-Video", Type: MediaTypeVideo, Provider: "mulerouter"},
		{ID: "wan2.6-i2v", Name: "Wan2 Image-to-Video", Type: MediaTypeVideo, Provider: "mulerouter"},
		{ID: "wan2-spark-t2v", Name: "Wan2 Spark Text-to-Video", Type: MediaTypeVideo, Provider: "mulerouter"},
		// Midjourney vendor — image
		{ID: "midjourney", Name: "Midjourney", Type: MediaTypeImage, Provider: "mulerouter"},
		// Midjourney vendor — video
		{ID: "midjourney-video", Name: "Midjourney Video", Type: MediaTypeVideo, Provider: "mulerouter"},
	}
}

func (p *MuleRouterProvider) SupportsType(t MediaType) bool {
	return t == MediaTypeImage || t == MediaTypeVideo
}

// Generate dispatches to the appropriate MuleRouter endpoint based on model.
func (p *MuleRouterProvider) Generate(ctx context.Context, req *MediaRequest) (*MediaTask, error) {
	model := req.Model
	if model == "" {
		model = "dall-e-3"
	}

	vendor := modelToVendor(model)

	// OpenAI vendor uses /v1/images/generations (synchronous)
	if vendor == "openai" {
		return p.generateOpenAI(ctx, req, model)
	}

	// All other vendors use async task pattern via /vendors/{vendor}/v1/{model}/generation
	return p.generateVendor(ctx, req, model, vendor)
}

// Poll checks async task status via MuleRouter's task endpoint.
func (p *MuleRouterProvider) Poll(ctx context.Context, taskID string) (*MediaTask, error) {
	url := fmt.Sprintf("%s/tasks/%s", p.baseURL, taskID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var taskResp muleRouterTaskResponse
	if err := json.Unmarshal(respBody, &taskResp); err != nil {
		return nil, err
	}

	task := &MediaTask{UpstreamID: taskID}

	switch taskResp.Status {
	case "succeeded", "completed":
		task.Status = TaskStatusSucceeded
		var results []MediaResult
		for _, r := range taskResp.Output.Results {
			mt := "image/png"
			if r.Type == "video" {
				mt = "video/mp4"
			}
			results = append(results, MediaResult{
				OriginalURL: r.URL,
				ContentType: mt,
			})
		}
		now := time.Now()
		task.Response = &MediaResponse{Created: now.Unix(), Data: results}
		task.CompletedAt = &now
	case "failed":
		task.Status = TaskStatusFailed
		task.Error = taskResp.Error
	default:
		task.Status = TaskStatusProcessing
		task.Progress = taskResp.Progress
	}

	return task, nil
}

// generateOpenAI handles OpenAI-compatible image generation (synchronous).
func (p *MuleRouterProvider) generateOpenAI(ctx context.Context, req *MediaRequest, model string) (*MediaTask, error) {
	oaiReq := map[string]any{
		"model":  model,
		"prompt": req.Prompt,
		"n":      max(req.N, 1),
	}
	if req.Size != "" {
		oaiReq["size"] = req.Size
	}
	if req.Quality != "" {
		oaiReq["quality"] = req.Quality
	}
	if req.Style != "" {
		oaiReq["style"] = req.Style
	}

	body, err := json.Marshal(oaiReq)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/v1/images/generations", p.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mulerouter API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var oaiResp openAIImageResponse
	if err := json.Unmarshal(respBody, &oaiResp); err != nil {
		return nil, err
	}

	var results []MediaResult
	for _, d := range oaiResp.Data {
		results = append(results, MediaResult{
			OriginalURL:   d.URL,
			B64JSON:       d.B64JSON,
			RevisedPrompt: d.RevisedPrompt,
			ContentType:   "image/png",
		})
	}

	if len(results) == 0 {
		return nil, ErrNoResults
	}

	now := time.Now()
	return &MediaTask{
		ID:       uuid.New().String(),
		Status:   TaskStatusSucceeded,
		Type:     MediaTypeImage,
		Provider: "mulerouter",
		Model:    model,
		Response: &MediaResponse{Created: oaiResp.Created, Data: results},
		CreatedAt:   now,
		CompletedAt: &now,
	}, nil
}

// generateVendor handles vendor-specific async generation.
func (p *MuleRouterProvider) generateVendor(ctx context.Context, req *MediaRequest, model, vendor string) (*MediaTask, error) {
	vendorReq := map[string]any{
		"prompt": req.Prompt,
	}
	if req.NegativePrompt != "" {
		vendorReq["negative_prompt"] = req.NegativePrompt
	}
	if req.Size != "" {
		vendorReq["size"] = req.Size
	}
	if req.Duration > 0 {
		vendorReq["duration"] = req.Duration
	}

	body, err := json.Marshal(vendorReq)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/vendors/%s/v1/%s/generation", p.baseURL, vendor, model)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("mulerouter vendor API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var vendorResp muleRouterVendorResponse
	if err := json.Unmarshal(respBody, &vendorResp); err != nil {
		return nil, err
	}

	return &MediaTask{
		ID:         uuid.New().String(),
		Status:     TaskStatusProcessing,
		Type:       req.Type,
		Provider:   "mulerouter",
		Model:      model,
		UpstreamID: vendorResp.TaskID,
		CreatedAt:  time.Now(),
	}, nil
}

// modelToVendor maps a model ID to its MuleRouter vendor path segment.
func modelToVendor(model string) string {
	switch model {
	case "dall-e-3", "dall-e-2":
		return "openai"
	case "midjourney", "midjourney-video":
		return "midjourney"
	default:
		// Alibaba covers: qwen-image-max, qwen-image-edit-max, nano-banana-pro,
		// wan2.6-t2v, wan2.6-i2v, wan2-spark-t2v, etc.
		return "alibaba"
	}
}

// MuleRouter API types

type openAIImageResponse struct {
	Created int64 `json:"created"`
	Data    []struct {
		URL           string `json:"url,omitempty"`
		B64JSON       string `json:"b64_json,omitempty"`
		RevisedPrompt string `json:"revised_prompt,omitempty"`
	} `json:"data"`
}

type muleRouterVendorResponse struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
}

type muleRouterTaskResponse struct {
	Status   string  `json:"status"`
	Progress float64 `json:"progress"`
	Error    string  `json:"error,omitempty"`
	Output   struct {
		Results []struct {
			URL  string `json:"url"`
			Type string `json:"type,omitempty"`
		} `json:"results"`
	} `json:"output"`
}
