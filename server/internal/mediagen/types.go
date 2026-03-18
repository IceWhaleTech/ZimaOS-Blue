package mediagen

import (
	"context"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/task"
)

// MediaType distinguishes image vs video generation.
type MediaType string

const (
	MediaTypeImage MediaType = "image"
	MediaTypeVideo MediaType = "video"
)

// TaskStatus is an alias for task.Status so existing code compiles unchanged.
type TaskStatus = task.Status

// Status constants — aliases for the shared task package values.
const (
	TaskStatusPending    = task.StatusPending
	TaskStatusProcessing = task.StatusProcessing
	TaskStatusSucceeded  = task.StatusSucceeded
	TaskStatusFailed     = task.StatusFailed
	TaskStatusCancelled  = task.StatusCancelled
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
	ReferenceURL   string         `json:"-"`
	ReferenceURLs  []string       `json:"-"` // multiple images (i2v, kf2v, i2i)
	Duration       int            `json:"duration,omitempty"`
	Extra          map[string]any `json:"extra,omitempty"`
}

// MediaResult represents a single generated media item.
type MediaResult struct {
	URL           string `json:"url,omitempty"`
	ThumbnailURL  string `json:"thumbnail_url,omitempty"`
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
// Embeds task.BaseTask for shared lifecycle fields (ID, Status, Error, Progress, timestamps).
type MediaTask struct {
	task.BaseTask

	UserID       string             `json:"user_id,omitempty"`
	MessageID    string             `json:"message_id,omitempty"`
	Type         MediaType          `json:"type"`
	Category     string             `json:"category,omitempty"`
	Provider     string             `json:"provider"`
	Model        string             `json:"model"`
	Request      *MediaRequest      `json:"request,omitempty"`
	Response     *MediaResponse     `json:"response,omitempty"`
	Source       string             `json:"source,omitempty"` // "web" or "channel"
	FallbackInfo *MediaFallbackInfo `json:"fallback_info,omitempty"`

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
	ID               string        `json:"id"`
	Name             string        `json:"name"`
	Type             MediaType     `json:"type"`
	Category         MediaCategory `json:"category,omitempty"` // t2i, t2v, i2v, i2i, kf2v
	Provider         string        `json:"provider"`
	MaxResolution    string        `json:"max_resolution,omitempty"`
	SupportedSizes   []string      `json:"supported_sizes,omitempty"`
	Price            float64       `json:"price,omitempty"`        // per-unit price (USD)
	PricingUnit      string        `json:"pricing_unit,omitempty"` // "image", "second", "video"
	IsFallback       bool          `json:"is_fallback,omitempty"`
	FallbackStrategy string        `json:"fallback_strategy,omitempty"`
}

// MediaFallbackInfo describes how a task was fulfilled without a configured upstream API key.
type MediaFallbackInfo struct {
	Used        bool     `json:"used"`
	Strategy    string   `json:"strategy"`
	DisplayName string   `json:"display_name"`
	SourceURLs  []string `json:"source_urls,omitempty"`
	SpaceURL    string   `json:"space_url,omitempty"`
	Disclosure  string   `json:"disclosure"`
}
