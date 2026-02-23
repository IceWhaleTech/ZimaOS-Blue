package mediagen

import (
	"context"
	"time"
)

// MediaType distinguishes image vs video generation.
type MediaType string

const (
	MediaTypeImage MediaType = "image"
	MediaTypeVideo MediaType = "video"
)

// TaskStatus tracks async generation lifecycle.
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusProcessing TaskStatus = "processing"
	TaskStatusSucceeded  TaskStatus = "succeeded"
	TaskStatusFailed     TaskStatus = "failed"
)

// MediaRequest is the unified request for all providers.
type MediaRequest struct {
	Type           MediaType      `json:"type"`
	Prompt         string         `json:"prompt"`
	NegativePrompt string         `json:"negative_prompt,omitempty"`
	Model          string         `json:"model,omitempty"`
	N              int            `json:"n,omitempty"`
	Size           string         `json:"size,omitempty"`
	Quality        string         `json:"quality,omitempty"`
	Style          string         `json:"style,omitempty"`
	ResponseFormat string         `json:"response_format,omitempty"`
	ReferenceImage []byte         `json:"-"`
	ReferenceURL   string         `json:"reference_url,omitempty"`
	Duration       int            `json:"duration,omitempty"`
	Extra          map[string]any `json:"extra,omitempty"`
}

// MediaResult represents a single generated media item.
type MediaResult struct {
	URL           string `json:"url,omitempty"`
	B64JSON       string `json:"b64_json,omitempty"`
	OriginalURL   string `json:"-"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
	ContentType   string `json:"content_type,omitempty"`
	Width         int    `json:"width,omitempty"`
	Height        int    `json:"height,omitempty"`
	DurationSec   int    `json:"duration_sec,omitempty"`
}

// MediaResponse is the unified response.
type MediaResponse struct {
	Created int64         `json:"created"`
	Data    []MediaResult `json:"data"`
}

// MediaTask tracks an async generation job.
type MediaTask struct {
	ID          string         `json:"id"`
	Status      TaskStatus     `json:"status"`
	Type        MediaType      `json:"type"`
	Provider    string         `json:"provider"`
	Model       string         `json:"model"`
	Request     *MediaRequest  `json:"request,omitempty"`
	Response    *MediaResponse `json:"response,omitempty"`
	Error       string         `json:"error,omitempty"`
	Progress    float64        `json:"progress"`
	CreatedAt   time.Time      `json:"created_at"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`

	// Internal: upstream task ID for async providers
	UpstreamID string `json:"-"`
}

// MediaProvider is the interface each backend implements.
type MediaProvider interface {
	Name() string
	SupportedModels() []MediaModelInfo
	SupportsType(t MediaType) bool
	Generate(ctx context.Context, req *MediaRequest) (*MediaTask, error)
	Poll(ctx context.Context, taskID string) (*MediaTask, error)
}

// MediaModelInfo describes a model's media generation capabilities.
type MediaModelInfo struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Type           MediaType `json:"type"`
	Provider       string    `json:"provider"`
	MaxResolution  string    `json:"max_resolution,omitempty"`
	SupportedSizes []string  `json:"supported_sizes,omitempty"`
}
