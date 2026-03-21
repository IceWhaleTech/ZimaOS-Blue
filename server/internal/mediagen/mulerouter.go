package mediagen

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/network"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/task"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
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
	isEdit bool
}

// NewMuleRouterProvider creates a new MuleRouter media provider.
func NewMuleRouterProvider(apiKey, baseURL string) *MuleRouterProvider {
	return &MuleRouterProvider{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  network.NewPooledHTTPClient(5 * time.Minute),
	}
}

func (p *MuleRouterProvider) Name() string { return "mulerouter" }

func (p *MuleRouterProvider) SupportedModels() []MediaModelInfo {
	return []MediaModelInfo{
		// --- Text-to-Image (t2i) ---
		{ID: "nano-banana-pro", Name: "Nano Banana Pro", Type: MediaTypeImage, Category: CategoryT2I, Provider: "mulerouter"},
		{ID: "qwen-image-max", Name: "Qwen Image Max", Type: MediaTypeImage, Category: CategoryT2I, Provider: "mulerouter"},
		{ID: "dall-e-3", Name: "DALL-E 3", Type: MediaTypeImage, Category: CategoryT2I, Provider: "mulerouter"},
		{ID: "midjourney", Name: "Midjourney", Type: MediaTypeImage, Category: CategoryT2I, Provider: "mulerouter"},
		{ID: "wan2.5-t2i-preview", Name: "Wan 2.5 T2I", Type: MediaTypeImage, Category: CategoryT2I, Provider: "mulerouter"},
		{ID: "wan2.6-t2i", Name: "Wan 2.6 T2I", Type: MediaTypeImage, Category: CategoryT2I, Provider: "mulerouter"},
		// --- Image Editing (i2i) ---
		{ID: "nano-banana-pro", Name: "Nano Banana Pro Edit", Type: MediaTypeImage, Category: CategoryI2I, Provider: "mulerouter"},
		{ID: "qwen-image-edit-max", Name: "Qwen Image Edit", Type: MediaTypeImage, Category: CategoryI2I, Provider: "mulerouter"},
		{ID: "wan2.5-i2i-preview", Name: "Wan 2.5 I2I", Type: MediaTypeImage, Category: CategoryI2I, Provider: "mulerouter"},
		{ID: "wan2.6-image", Name: "Wan 2.6 Image Edit", Type: MediaTypeImage, Category: CategoryI2I, Provider: "mulerouter"},
		// --- Text-to-Video (t2v) ---
		{ID: "wan2.6-t2v", Name: "Wan 2.6 T2V", Type: MediaTypeVideo, Category: CategoryT2V, Provider: "mulerouter"},
		{ID: "wan2.5-t2v-spark", Name: "Wan 2.5 T2V Spark", Type: MediaTypeVideo, Category: CategoryT2V, Provider: "mulerouter"},
		{ID: "wan2.6-t2v-spark", Name: "Wan 2.6 T2V Spark", Type: MediaTypeVideo, Category: CategoryT2V, Provider: "mulerouter"},
		{ID: "wan2.5-t2v-preview", Name: "Wan 2.5 T2V", Type: MediaTypeVideo, Category: CategoryT2V, Provider: "mulerouter"},
		{ID: "wan2.2-t2v-plus", Name: "Wan 2.2 T2V Plus", Type: MediaTypeVideo, Category: CategoryT2V, Provider: "mulerouter"},
		{ID: "wan2-spark-t2v", Name: "Wan2 Spark T2V", Type: MediaTypeVideo, Category: CategoryT2V, Provider: "mulerouter"},
		// --- Image-to-Video (i2v) ---
		{ID: "wan2.6-i2v", Name: "Wan 2.6 I2V", Type: MediaTypeVideo, Category: CategoryI2V, Provider: "mulerouter"},
		{ID: "wan2.5-i2v-spark", Name: "Wan 2.5 I2V Spark", Type: MediaTypeVideo, Category: CategoryI2V, Provider: "mulerouter"},
		{ID: "wan2.6-i2v-spark", Name: "Wan 2.6 I2V Spark", Type: MediaTypeVideo, Category: CategoryI2V, Provider: "mulerouter"},
		{ID: "wan2.5-i2v-preview", Name: "Wan 2.5 I2V", Type: MediaTypeVideo, Category: CategoryI2V, Provider: "mulerouter"},
		{ID: "wan2.2-i2v-plus", Name: "Wan 2.2 I2V Plus", Type: MediaTypeVideo, Category: CategoryI2V, Provider: "mulerouter"},
		{ID: "wan2.2-i2v-flash", Name: "Wan 2.2 I2V Flash", Type: MediaTypeVideo, Category: CategoryI2V, Provider: "mulerouter"},
		{ID: "midjourney-video", Name: "Midjourney Video", Type: MediaTypeVideo, Category: CategoryI2V, Provider: "mulerouter"},
		// --- Keyframe-to-Video (kf2v) ---
		{ID: "wan2.1-kf2v-plus", Name: "Wan 2.1 KF2V", Type: MediaTypeVideo, Category: CategoryKF2V, Provider: "mulerouter"},
		{ID: "wan2.1-vace-plus", Name: "Wan 2.1 VACE", Type: MediaTypeVideo, Category: CategoryKF2V, Provider: "mulerouter"},
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

	url := fmt.Sprintf("%s/vendors/%s/v1/%s/%s", p.baseURL, meta.vendor, vendorEndpoint(meta.vendor, meta.model, meta.isEdit), taskID)
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
		// If the body indicates a terminal status but we can't parse the full
		// response (e.g. error.code is number instead of string), extract what
		// we can and return a failed task instead of a parse error — otherwise
		// pollTask retries forever.
		bodyStr := string(respBody)
		if strings.Contains(bodyStr, `"status":"failed"`) || strings.Contains(bodyStr, `"status":"canceled"`) {
			failedTask := &MediaTask{UpstreamID: taskID}
			failedTask.Status = TaskStatusFailed
			failedTask.Error = fmt.Sprintf("upstream task failed (parse error: %v, body: %s)", err, truncate(bodyStr, 300))
			return failedTask, nil
		}
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
			// Try multiple URL field names — different vendors use different keys
			u := r.URL
			if u == "" {
				u = r.ResultURL
			}
			if u == "" {
				u = r.ImageURL
			}
			if u == "" {
				u = r.VideoURL
			}
			results = append(results, MediaResult{
				OriginalURL: u,
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
		// Fallback: some Alibaba models return a single URL at output level (not in results[])
		if len(results) == 0 || (len(results) > 0 && results[0].OriginalURL == "") {
			outURL := taskResp.Output.ResultURL
			if outURL == "" {
				outURL = taskResp.Output.ImageURL
			}
			if outURL == "" {
				outURL = taskResp.Output.VideoURL
			}
			if outURL != "" {
				if len(results) == 0 {
					results = append(results, MediaResult{OriginalURL: outURL, ContentType: "image/png"})
				} else {
					results[0].OriginalURL = outURL
				}
			}
		}
		now := timeutil.NowTime()
		task.Response = &MediaResponse{Created: now.Unix(), Data: results}
		task.CompletedAt = &now
		// Log if no URLs found — helps debug vendor response format issues
		hasURL := false
		for _, r := range results {
			if r.OriginalURL != "" {
				hasURL = true
				break
			}
		}
		if !hasURL {
			// Upstream says "succeeded" but no URL yet (e.g. only content_type).
			// Treat as still processing so the poller keeps going.
			log.Printf("[mediagen] poll succeeded but no URLs found for task %s, keeping as processing. raw body: %s", taskID, truncate(string(respBody), 500))
			task.Status = TaskStatusProcessing
			task.Response = nil
			task.CompletedAt = nil
			task.Progress = 0.9 // signal near-completion
			return task, nil
		}
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

// RestoreTaskMeta re-populates the in-memory vendor/model mapping for a recovered task.
// Called during startup recovery so that Poll() can find the correct vendor endpoint.
func (p *MuleRouterProvider) RestoreTaskMeta(upstreamID, model, category string) {
	vendor := modelToVendor(model)
	isEdit := category == string(CategoryI2I)
	p.taskMeta.Store(upstreamID, &muleRouterTaskMeta{vendor: vendor, model: model, isEdit: isEdit})
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

	now := timeutil.NowTime()
	return &MediaTask{
		BaseTask: task.BaseTask{
			ID:          uuid.New().String(),
			Status:      TaskStatusSucceeded,
			Progress:    1.0,
			CreatedAt:   now,
			CompletedAt: &now,
		},
		Type:     MediaTypeImage,
		Provider: "mulerouter",
		Model:    model,
		Response: &MediaResponse{Created: oaiResp.Created, Data: results},
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

	isEdit := req.ReferenceURL != ""

	// Pass reference images based on model category:
	// - kf2v models: "first_frame_image" + optional "last_frame_image"
	// - google edit (nano-banana-pro): "images" array
	// - other i2v/i2i models: "image" field (single image)
	if isKF2VModel(model) {
		if len(req.ReferenceURLs) > 0 {
			vendorReq["first_frame_image"] = req.ReferenceURLs[0]
		}
		if len(req.ReferenceURLs) > 1 {
			vendorReq["last_frame_image"] = req.ReferenceURLs[1]
		}
	} else if req.ReferenceURL != "" {
		if isEdit && vendor == "google" {
			vendorReq["images"] = []string{req.ReferenceURL}
		} else {
			vendorReq["image"] = req.ReferenceURL
		}
	}

	body, err := json.Marshal(vendorReq)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s/vendors/%s/v1/%s", p.baseURL, vendor, vendorEndpoint(vendor, model, isEdit))
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
	p.taskMeta.Store(upstreamID, &muleRouterTaskMeta{vendor: vendor, model: model, isEdit: isEdit})

	return &MediaTask{
		BaseTask:   task.BaseTask{ID: uuid.New().String(), Status: TaskStatusProcessing, CreatedAt: timeutil.NowTime()},
		Type:       req.Type,
		Provider:   "mulerouter",
		Model:      model,
		UpstreamID: upstreamID,
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
	case "wan2-spark-t2v", "wan2.5-t2v-spark", "wan2.6-t2v-spark",
		"wan2.5-i2v-spark", "wan2.6-i2v-spark":
		return "mulerouter"
	default:
		// Alibaba covers: qwen-*, wan2.1-*, wan2.2-*, wan2.5-*, wan2.6-* (non-spark)
		return "alibaba"
	}
}

// vendorEndpoint returns the API path suffix for a given vendor+model.
// Most vendors use /{model}/generation, but Midjourney uses /tob/diffusion.
// When isEdit is true and the vendor supports a separate edit endpoint, use that instead.
func vendorEndpoint(vendor, model string, isEdit bool) string {
	if vendor == "midjourney" {
		if model == "midjourney-video" {
			return "tob/video-diffusion"
		}
		return "tob/diffusion"
	}
	// Google nano-banana-pro has a separate /edit endpoint for image editing
	if isEdit && vendor == "google" && model == "nano-banana-pro" {
		return model + "/edit"
	}
	return model + "/generation"
}

// isKF2VModel returns true if the model is a keyframe-to-video model.
func isKF2VModel(model string) bool {
	return strings.Contains(model, "kf2v") || strings.Contains(model, "vace")
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
			Code   any    `json:"code,omitempty"` // string or number depending on vendor
			Title  string `json:"title,omitempty"`
			Detail string `json:"detail,omitempty"`
		} `json:"error,omitempty"`
	} `json:"task_info"`
	Progress float64 `json:"progress"`
	Output   struct {
		Results []struct {
			URL       string `json:"url"`
			ResultURL string `json:"result_url,omitempty"`
			ImageURL  string `json:"image_url,omitempty"`
			VideoURL  string `json:"video_url,omitempty"`
			Type      string `json:"type,omitempty"`
		} `json:"results"`
		// Some Alibaba models return a single URL at output level
		ResultURL string `json:"result_url,omitempty"`
		ImageURL  string `json:"image_url,omitempty"`
		VideoURL  string `json:"video_url,omitempty"`
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
