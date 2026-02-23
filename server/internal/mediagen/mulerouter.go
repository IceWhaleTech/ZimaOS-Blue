package mediagen

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MuleRouterProvider implements MediaProvider using MuleRouter's unified API.
// Supports OpenAI-compatible image gen + vendor-specific endpoints for Qwen/Wan2.
type MuleRouterProvider struct {
	apiKey  string
	baseURL string
	client  *http.Client
	// Track vendor+model per upstream task for correct polling URL
	taskMeta sync.Map // upstreamID -> muleRouterTaskMeta
}

type muleRouterTaskMeta struct {
	vendor string
	model  string
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
		// Google vendor — image (default, best cost/quality ratio)
		{ID: "nano-banana-pro", Name: "Nano Banana Pro", Type: MediaTypeImage, Provider: "mulerouter"},
		// Alibaba vendor — image
		{ID: "qwen-image-max", Name: "Qwen Image Max", Type: MediaTypeImage, Provider: "mulerouter"},
		{ID: "qwen-image-edit-max", Name: "Qwen Image Edit Max", Type: MediaTypeImage, Provider: "mulerouter"},
		// OpenAI vendor — image
		{ID: "dall-e-3", Name: "DALL-E 3", Type: MediaTypeImage, Provider: "mulerouter"},
		// Midjourney vendor — image
		{ID: "midjourney", Name: "Midjourney", Type: MediaTypeImage, Provider: "mulerouter"},
		// Alibaba vendor — video
		{ID: "wan2.6-t2v", Name: "Wan2 Text-to-Video", Type: MediaTypeVideo, Provider: "mulerouter"},
		{ID: "wan2.6-i2v", Name: "Wan2 Image-to-Video", Type: MediaTypeVideo, Provider: "mulerouter"},
		{ID: "wan2-spark-t2v", Name: "Wan2 Spark Text-to-Video", Type: MediaTypeVideo, Provider: "mulerouter"},
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
		model = "nano-banana-pro"
	}

	vendor := modelToVendor(model)

	// OpenAI vendor uses /v1/images/generations (synchronous)
	if vendor == "openai" {
		return p.generateOpenAI(ctx, req, model)
	}

	// All other vendors use async task pattern via /vendors/{vendor}/v1/{model}/generation
	return p.generateVendor(ctx, req, model, vendor)
}

// Poll checks async task status via MuleRouter's vendor-specific endpoint.
// URL: GET /vendors/{vendor}/v1/{model}/generation/{task_id}
func (p *MuleRouterProvider) Poll(ctx context.Context, taskID string) (*MediaTask, error) {
	// Look up vendor+model for this task
	metaVal, ok := p.taskMeta.Load(taskID)
	if !ok {
		return nil, fmt.Errorf("unknown task: %s (no vendor/model metadata)", taskID)
	}
	meta := metaVal.(*muleRouterTaskMeta)

	url := fmt.Sprintf("%s/vendors/%s/v1/%s/%s", p.baseURL, meta.vendor, vendorEndpoint(meta.vendor, meta.model), taskID)
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
		return nil, fmt.Errorf("poll parse error: %w (body: %s)", err, truncate(string(respBody), 200))
	}

	task := &MediaTask{UpstreamID: taskID}
	status := taskResp.TaskInfo.Status

	switch status {
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
		// Also check images/videos arrays (some models use these instead of output.results)
		for _, imgURL := range taskResp.Images {
			results = append(results, MediaResult{
				OriginalURL:   imgURL,
				ContentType:   "image/png",
				RevisedPrompt: taskResp.Description,
			})
		}
		for _, vidURL := range taskResp.Videos {
			results = append(results, MediaResult{
				OriginalURL: vidURL,
				ContentType: "video/mp4",
			})
		}
		now := time.Now()
		task.Response = &MediaResponse{Created: now.Unix(), Data: results}
		task.CompletedAt = &now
		// Clean up metadata
		p.taskMeta.Delete(taskID)
	case "failed":
		task.Status = TaskStatusFailed
		if taskResp.TaskInfo.Error != nil {
			task.Error = taskResp.TaskInfo.Error.Detail
			if task.Error == "" {
				task.Error = taskResp.TaskInfo.Error.Title
			}
		}
		p.taskMeta.Delete(taskID)
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

	// Google vendor (nano-banana-pro) uses aspect_ratio + resolution instead of size
	if vendor == "google" {
		if ar := req.Extra["aspect_ratio"]; ar != nil {
			vendorReq["aspect_ratio"] = ar
		} else if req.Size != "" {
			vendorReq["aspect_ratio"] = sizeToAspectRatio(req.Size)
		}
		if res := req.Extra["resolution"]; res != nil {
			vendorReq["resolution"] = res
		}
	} else {
		if req.Size != "" {
			vendorReq["size"] = req.Size
		}
	}

	if req.Duration > 0 {
		vendorReq["duration"] = req.Duration
	}

	body, err := json.Marshal(vendorReq)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/vendors/%s/v1/%s", p.baseURL, vendor, vendorEndpoint(vendor, model))
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

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("mulerouter vendor API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	// MuleRouter returns { "task_info": { "id": "...", "status": "pending" } }
	var vendorResp muleRouterVendorResponse
	if err := json.Unmarshal(respBody, &vendorResp); err != nil {
		return nil, fmt.Errorf("parse error: %w (body: %s)", err, truncate(string(respBody), 200))
	}

	upstreamID := vendorResp.TaskInfo.ID
	if upstreamID == "" {
		// Fallback: some endpoints may use top-level task_id
		upstreamID = vendorResp.TaskID
	}
	if upstreamID == "" {
		return nil, fmt.Errorf("mulerouter: no task ID in response: %s", truncate(string(respBody), 200))
	}

	// Store vendor+model for polling
	p.taskMeta.Store(upstreamID, &muleRouterTaskMeta{vendor: vendor, model: model})

	return &MediaTask{
		ID:         uuid.New().String(),
		Status:     TaskStatusProcessing,
		Type:       req.Type,
		Provider:   "mulerouter",
		Model:      model,
		UpstreamID: upstreamID,
		CreatedAt:  time.Now(),
	}, nil
}

// modelToVendor maps a model ID to its MuleRouter vendor path segment.
func modelToVendor(model string) string {
	switch model {
	case "dall-e-3", "dall-e-2":
		return "openai"
	case "nano-banana-pro":
		return "google"
	case "midjourney", "midjourney-video":
		return "midjourney"
	case "wan2-spark-t2v":
		return "mulerouter"
	default:
		// Alibaba covers: qwen-image-max, qwen-image-edit-max,
		// wan2.6-t2v, wan2.6-i2v, etc.
		return "alibaba"
	}
}

// vendorEndpoint returns the API path suffix for a given vendor+model.
// Most vendors use /{model}/generation, but Midjourney uses /tob/diffusion.
func vendorEndpoint(vendor, model string) string {
	if vendor == "midjourney" {
		if model == "midjourney-video" {
			return "tob/video-diffusion"
		}
		return "tob/diffusion"
	}
	return model + "/generation"
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// sizeToAspectRatio converts a WxH size string to the closest standard aspect ratio.
func sizeToAspectRatio(size string) string {
	// Parse WxH
	var w, h int
	if _, err := fmt.Sscanf(size, "%dx%d", &w, &h); err != nil || w == 0 || h == 0 {
		return "1:1"
	}
	// Common aspect ratios: 1:1, 3:4, 4:3, 9:16, 16:9, 5:4, 4:5
	type ar struct {
		label string
		ratio float64
	}
	ratios := []ar{
		{"1:1", 1.0},
		{"4:5", 0.8},
		{"3:4", 0.75},
		{"9:16", 0.5625},
		{"5:4", 1.25},
		{"4:3", 1.333},
		{"16:9", 1.778},
	}
	r := float64(w) / float64(h)
	best := ratios[0]
	bestDiff := abs(r - best.ratio)
	for _, a := range ratios[1:] {
		d := abs(r - a.ratio)
		if d < bestDiff {
			best = a
			bestDiff = d
		}
	}
	return best.label
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
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

// muleRouterVendorResponse is the creation response.
// Format: { "task_info": { "id": "...", "status": "pending" } }
type muleRouterVendorResponse struct {
	TaskInfo struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	} `json:"task_info"`
	// Fallback for older API format
	TaskID string `json:"task_id,omitempty"`
}

// muleRouterTaskResponse is the polling response.
// Format: { "task_info": { "id": "...", "status": "succeeded" }, "output": { "results": [...] } }
type muleRouterTaskResponse struct {
	TaskInfo struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Error  *struct {
			Code   string `json:"code,omitempty"`
			Title  string `json:"title,omitempty"`
			Detail string `json:"detail,omitempty"`
		} `json:"error,omitempty"`
	} `json:"task_info"`
	Progress float64 `json:"progress"`
	Output   struct {
		Results []struct {
			URL  string `json:"url"`
			Type string `json:"type,omitempty"`
		} `json:"results"`
	} `json:"output"`
	// Some models return images/videos as []string URLs directly (e.g. nano-banana-pro)
	Images      muleRouterMediaURLs `json:"images,omitempty"`
	Videos      muleRouterMediaURLs `json:"videos,omitempty"`
	Description string              `json:"description,omitempty"`
}

// muleRouterMediaURLs handles both []string and []{url:string} formats.
type muleRouterMediaURLs []string

func (m *muleRouterMediaURLs) UnmarshalJSON(data []byte) error {
	// Try []string first (nano-banana-pro format)
	var strs []string
	if err := json.Unmarshal(data, &strs); err == nil {
		*m = strs
		return nil
	}
	// Fallback: [{url: "..."}]
	var objs []struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(data, &objs); err != nil {
		return err
	}
	for _, o := range objs {
		*m = append(*m, o.URL)
	}
	return nil
}
