package scenecompose

import (
	"context"
	"image"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

type ComposeRequest struct {
	Prompt string
	Width  int
	Height int
	Locale string
}

type AssetRef struct {
	Kind      string `json:"kind"`
	Query     string `json:"query,omitempty"`
	SourceURL string `json:"source_url,omitempty"`
	Title     string `json:"title,omitempty"`
}

type ComposeDebugInfo struct {
	Plan              *ScenePlan `json:"plan,omitempty"`
	BackgroundQuery   string     `json:"background_query,omitempty"`
	ForegroundQueries []string   `json:"foreground_queries,omitempty"`
}

type RenderLayer struct {
	ID     string
	Kind   string
	Image  image.Image
	Layout LayoutHint
	Asset  AssetRef
}

type ComposeResult struct {
	Image      image.Image
	Layers     []RenderLayer
	UsedAssets []AssetRef
	Debug      ComposeDebugInfo
}

type ScenePlan struct {
	Background        string           `json:"background"`
	Style             string           `json:"style"`
	Lighting          string           `json:"lighting"`
	TimeOfDay         string           `json:"time_of_day"`
	Weather           string           `json:"weather"`
	CameraView        string           `json:"camera_view"`
	SceneQuery        string           `json:"scene_query,omitempty"`
	SceneQueryEN      string           `json:"scene_query_en,omitempty"`
	BackgroundQuery   string           `json:"background_query,omitempty"`
	BackgroundQueryEN string           `json:"background_query_en,omitempty"`
	Foreground        []ForegroundPlan `json:"foreground"`
}

type ForegroundPlan struct {
	ID              string     `json:"id"`
	Type            string     `json:"type"`
	Attributes      []string   `json:"attributes,omitempty"`
	SearchQuery     string     `json:"search_query,omitempty"`
	SearchQueryEN   string     `json:"search_query_en,omitempty"`
	FallbackQuery   string     `json:"fallback_query,omitempty"`
	FallbackQueryEN string     `json:"fallback_query_en,omitempty"`
	Priority        int        `json:"priority"`
	Layout          LayoutHint `json:"layout"`
}

type LayoutHint struct {
	Horizontal string `json:"horizontal"`
	Vertical   string `json:"vertical"`
	Depth      string `json:"depth"`
	Scale      string `json:"scale"`
	Grounded   bool   `json:"grounded"`
}

type SearchResult struct {
	Title        string
	URL          string
	Description  string
	ImageURL     string
	ThumbnailURL string
}

type ResolvedImage struct {
	Title       string
	PageURL     string
	SourceURL   string
	Description string
	ContentType string
	Image       image.Image
	Width       int
	Height      int
	HasAlpha    bool
}

type Searcher interface {
	Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error)
}

type Resolver interface {
	Resolve(ctx context.Context, result SearchResult) (*ResolvedImage, error)
}

type LLMCaller interface {
	Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error)
}

type CutoutManager interface {
	ModelID() string
	IsReady() bool
	EnsureReadyAsync(ctx context.Context)
	Cutout(ctx context.Context, src image.Image) (*CutoutResult, error)
	GetStatus() *ModelStatus
}

type CutoutResult struct {
	Image image.Image
	Mask  *image.Alpha
}

type ModelStatus struct {
	ModelID     string                 `json:"model_id"`
	Status      string                 `json:"status"`
	Ready       bool                   `json:"ready"`
	Downloading bool                   `json:"downloading"`
	State       string                 `json:"state"`
	Error       string                 `json:"error,omitempty"`
	Progress    *ModelDownloadProgress `json:"progress,omitempty"`
	Files       []ModelFileStatus      `json:"files,omitempty"`
}

type ModelDownloadProgress struct {
	File       string  `json:"file"`
	FileIndex  int     `json:"file_index"`
	TotalFiles int     `json:"total_files"`
	Downloaded int64   `json:"downloaded"`
	Total      int64   `json:"total"`
	Percentage float64 `json:"percentage"`
	SpeedHuman string  `json:"speed_human,omitempty"`
	ETA        string  `json:"eta,omitempty"`
}

type ModelFileStatus struct {
	Filename   string `json:"filename"`
	Downloaded bool   `json:"downloaded"`
	Size       string `json:"size"`
}
