package mediagen

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

const defaultDashScopeBaseURL = "https://dashscope.aliyuncs.com"

// DashScopeProvider implements MediaProvider using Alibaba's DashScope API.
// Uses async task pattern: POST to create → GET to poll status.
type DashScopeProvider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewDashScopeProvider creates a new DashScope media provider.
func NewDashScopeProvider(apiKey, baseURL string) *DashScopeProvider {
	if baseURL == "" {
		baseURL = defaultDashScopeBaseURL
	}
	return &DashScopeProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *DashScopeProvider) Name() string { return "dashscope" }

func (p *DashScopeProvider) SupportedModels() []MediaModelInfo {
	return []MediaModelInfo{
		{ID: "wanx-v1", Name: "Wanx v1", Type: MediaTypeImage, Provider: "dashscope"},
		{ID: "wanx2.1-t2i-turbo", Name: "Wanx 2.1 Turbo", Type: MediaTypeImage, Provider: "dashscope"},
		{ID: "qwen-image-max", Name: "Qwen Image Max", Type: MediaTypeImage, Provider: "dashscope"},
	}
}

func (p *DashScopeProvider) SupportsType(t MediaType) bool {
	return t == MediaTypeImage
}

// Generate creates an async image generation task on DashScope.
func (p *DashScopeProvider) Generate(ctx context.Context, req *MediaRequest) (*MediaTask, error) {
	model := req.Model
	if model == "" {
		model = "wanx-v1"
	}

	dsReq := dashScopeRequest{
		Model: model,
		Input: dashScopeInput{
			Prompt: req.Prompt,
		},
		Parameters: dashScopeParams{
			N: max(req.N, 1),
		},
	}

	if req.NegativePrompt != "" {
		dsReq.Input.NegativePrompt = req.NegativePrompt
	}
	if req.Size != "" {
		dsReq.Parameters.Size = req.Size
	}

	body, err := json.Marshal(dsReq)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/api/v1/services/aigc/text2image/image-synthesis", p.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("X-DashScope-Async", "enable")

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
		return nil, fmt.Errorf("dashscope API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var dsResp dashScopeCreateResponse
	if err := json.Unmarshal(respBody, &dsResp); err != nil {
		return nil, fmt.Errorf("dashscope response parse error: %w", err)
	}

	if dsResp.Output.TaskID == "" {
		return nil, fmt.Errorf("dashscope: no task_id in response")
	}

	return &MediaTask{
		ID:         uuid.New().String(),
		Status:     TaskStatusProcessing,
		Type:       MediaTypeImage,
		UpstreamID: dsResp.Output.TaskID,
		CreatedAt:  time.Now(),
	}, nil
}

// Poll checks the status of an async DashScope task.
func (p *DashScopeProvider) Poll(ctx context.Context, taskID string) (*MediaTask, error) {
	url := fmt.Sprintf("%s/api/v1/tasks/%s", p.baseURL, taskID)
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

	var dsResp dashScopePollResponse
	if err := json.Unmarshal(respBody, &dsResp); err != nil {
		return nil, err
	}

	task := &MediaTask{
		UpstreamID: taskID,
	}

	switch dsResp.Output.TaskStatus {
	case "SUCCEEDED":
		task.Status = TaskStatusSucceeded
		var results []MediaResult
		for _, r := range dsResp.Output.Results {
			results = append(results, MediaResult{
				OriginalURL: r.URL,
				ContentType: "image/png",
			})
		}
		now := time.Now()
		task.Response = &MediaResponse{
			Created: now.Unix(),
			Data:    results,
		}
		task.CompletedAt = &now
	case "FAILED":
		task.Status = TaskStatusFailed
		task.Error = dsResp.Output.Message
	default:
		task.Status = TaskStatusProcessing
	}

	return task, nil
}

// DashScope API types

type dashScopeRequest struct {
	Model      string          `json:"model"`
	Input      dashScopeInput  `json:"input"`
	Parameters dashScopeParams `json:"parameters"`
}

type dashScopeInput struct {
	Prompt         string `json:"prompt"`
	NegativePrompt string `json:"negative_prompt,omitempty"`
}

type dashScopeParams struct {
	Size string `json:"size,omitempty"`
	N    int    `json:"n,omitempty"`
}

type dashScopeCreateResponse struct {
	Output struct {
		TaskID     string `json:"task_id"`
		TaskStatus string `json:"task_status"`
	} `json:"output"`
	RequestID string `json:"request_id"`
}

type dashScopePollResponse struct {
	Output struct {
		TaskID     string `json:"task_id"`
		TaskStatus string `json:"task_status"`
		Message    string `json:"message,omitempty"`
		Results    []struct {
			URL string `json:"url"`
		} `json:"results,omitempty"`
	} `json:"output"`
}
