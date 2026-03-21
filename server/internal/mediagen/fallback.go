package mediagen

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	stdDraw "image/draw"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/network"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/scenecompose"
	basetask "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/task"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/google/uuid"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
	"golang.org/x/net/html"
)

const (
	fallbackProviderName = "fallback"

	FallbackStrategyWebCanvas   = "web_canvas"
	FallbackStrategyPublicSpace = "public_space"

	FallbackModelWebCanvasT2I = "fallback-web-canvas-t2i"
	FallbackModelSpaceT2I     = "fallback-space-t2i"
	FallbackModelSpaceI2I     = "fallback-space-i2i"
	FallbackModelSpaceT2V     = "fallback-space-t2v"
	FallbackModelSpaceI2V     = "fallback-space-i2v"
	FallbackModelSpaceKF2V    = "fallback-space-kf2v"

	fallbackRenderReadySelector = "#fallback-render-ready.ready"
	fallbackMaxHTMLBytes        = 1 << 20
	fallbackMaxImageBytes       = 10 << 20
)

var fallbackComplexKeywords = []string{
	"cinematic",
	"storyboard",
	"multi-scene",
	"consistent character",
	"character sheet",
	"连续角色",
	"分镜",
	"多镜头",
}

var fallbackPosterStopWords = map[string]struct{}{
	"a": {}, "an": {}, "the": {}, "and": {}, "for": {}, "with": {}, "from": {}, "into": {}, "that": {}, "this": {},
	"your": {}, "you": {}, "show": {}, "make": {}, "image": {}, "photo": {}, "poster": {}, "video": {}, "draw": {},
	"生成": {}, "图片": {}, "海报": {}, "视频": {}, "一个": {}, "一张": {}, "帮我": {},
}

// FallbackConfig controls the built-in no-key media fallback behavior.
type FallbackConfig struct {
	Enabled             bool
	SearchProviderChain []string
	SearchMaxResults    int
	ScreenshotWidth     int
	ScreenshotHeight    int
	ComplexPromptChars  int
	RenderBaseURL       string
	DataDir             string
	U2NetPStatusURL     string
	PublicSpaces        []FallbackPublicSpacePreset
}

// FallbackPublicSpacePreset defines a public browser-driven creative-space preset.
type FallbackPublicSpacePreset struct {
	ID                      string
	DisplayName             string
	URL                     string
	Categories              []MediaCategory
	ReadySelectors          []string
	PromptSelectors         []string
	NegativePromptSelectors []string
	UploadSelectors         []string
	SubmitSelectors         []string
	SuccessSelectors        []FallbackResultSelector
	ProcessingSelectors     []string
	ErrorSelectors          []string
	PollInterval            time.Duration
	Timeout                 time.Duration
}

// FallbackResultSelector extracts a finished result URL from a live page.
type FallbackResultSelector struct {
	Selectors []string
	Attribute string
	Kind      string
}

// FallbackSearchResult is the normalized result shape used by the fallback engine.
type FallbackSearchResult struct {
	Title       string
	URL         string
	Description string
}

// FallbackSearcher abstracts the existing web_search tool so the engine is testable.
type FallbackSearcher interface {
	Search(ctx context.Context, query string, maxResults int, providers []string) ([]FallbackSearchResult, error)
}

// FallbackBrowserService captures the browser capabilities needed by the fallback engine.
type FallbackBrowserService interface {
	Start(ctx context.Context) error
	OpenTab(ctx context.Context, url string) (*browser.Tab, error)
	CloseTab(ctx context.Context, targetID string) error
	Screenshot(ctx context.Context, req *browser.ScreenshotRequest) (*browser.ScreenshotResponse, error)
	ElementExists(ctx context.Context, targetID, selector string) (bool, error)
	ExtractFirstFromTab(ctx context.Context, targetID, selector, attribute string) (string, error)
	Act(ctx context.Context, req *browser.ActRequest) (*browser.ActResponse, error)
	PageInfo(ctx context.Context, targetID string) (string, string, error)
}

// ToolWebSearcher adapts tools.WebSearchTool into the fallback searcher interface.
type ToolWebSearcher struct {
	tool *tools.WebSearchTool
}

// NewToolWebSearcher creates a tool-backed fallback searcher.
func NewToolWebSearcher(tool *tools.WebSearchTool) *ToolWebSearcher {
	if tool == nil {
		return nil
	}
	return &ToolWebSearcher{tool: tool}
}

// Search runs a web search using the configured tool.
func (s *ToolWebSearcher) Search(ctx context.Context, query string, maxResults int, providers []string) ([]FallbackSearchResult, error) {
	if s == nil || s.tool == nil {
		return nil, fmt.Errorf("web search tool unavailable")
	}
	args := map[string]interface{}{
		"query":       query,
		"max_results": maxResults,
		"format":      "json",
	}
	if len(providers) > 0 {
		args["provider"] = strings.Join(providers, ",")
	}
	raw, err := s.tool.Execute(ctx, args)
	if err != nil {
		return nil, err
	}
	text, ok := raw.(string)
	if !ok {
		return nil, fmt.Errorf("unexpected web search result type %T", raw)
	}
	var payload struct {
		Results []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			Description string `json:"description"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		return nil, err
	}
	results := make([]FallbackSearchResult, 0, len(payload.Results))
	for _, item := range payload.Results {
		results = append(results, FallbackSearchResult{
			Title:       strings.TrimSpace(item.Title),
			URL:         strings.TrimSpace(item.URL),
			Description: strings.TrimSpace(item.Description),
		})
	}
	return results, nil
}

// FallbackEngine is the built-in no-key media fallback executor.
type FallbackEngine struct {
	config   FallbackConfig
	storage  *MediaStorage
	searcher FallbackSearcher
	browser  func() FallbackBrowserService
	locale   string

	renderStore *fallbackRenderStore
	httpClient  *http.Client
	tasks       sync.Map

	sceneComposer    *scenecompose.Engine
	scenePlannerLLM  scenecompose.LLMCaller
	u2netpModel      *scenecompose.U2NetPModelManager
	downloadCardKeys sync.Map
}

// NewFallbackEngine creates a new no-key fallback engine.
func NewFallbackEngine(cfg FallbackConfig, storage *MediaStorage, searcher FallbackSearcher, browserSvc func() FallbackBrowserService, locale string) *FallbackEngine {
	cfg = normalizeFallbackConfig(cfg)
	engine := &FallbackEngine{
		config:      cfg,
		storage:     storage,
		searcher:    searcher,
		browser:     browserSvc,
		locale:      locale,
		renderStore: newFallbackRenderStore(),
		httpClient:  network.NewPooledHTTPClient(5 * time.Minute),
	}
	engine.initSceneComposer()
	return engine
}

func (e *FallbackEngine) initSceneComposer() {
	if e == nil {
		return
	}
	var cutout *scenecompose.CutoutStrategy
	if strings.TrimSpace(e.config.DataDir) != "" {
		modelDir := filepath.Join(e.config.DataDir, "media", "models", "u2netp")
		e.u2netpModel = scenecompose.NewU2NetPModelManager(modelDir, e.config.U2NetPStatusURL)
		e.u2netpModel.SetDownloadStartHook(func(ctx context.Context) {
			e.emitU2NetPDownloadCard(ctx)
		})
		cutout = scenecompose.NewCutoutStrategy(e.u2netpModel)
	}
	if e.searcher == nil {
		return
	}
	e.sceneComposer = scenecompose.NewEngine(
		scenecompose.NewPlanner(e.scenePlannerLLM),
		scenecompose.NewAssetSearcher(fallbackSceneSearcher{engine: e}, fallbackSceneResolver{engine: e}),
		cutout,
		scenecompose.NewRenderer(),
	)
}

func (e *FallbackEngine) SetScenePlannerLLM(llmCaller scenecompose.LLMCaller) {
	if e == nil {
		return
	}
	e.scenePlannerLLM = llmCaller
	if e.sceneComposer != nil {
		e.sceneComposer.SetLLM(llmCaller)
	}
}

func (e *FallbackEngine) emitU2NetPDownloadCard(ctx context.Context) {
	if e == nil || e.u2netpModel == nil || strings.TrimSpace(e.config.U2NetPStatusURL) == "" {
		return
	}
	sessionKey := strings.TrimSpace(tools.GetSessionID(ctx))
	if sessionKey == "" {
		sessionKey = "global"
	}
	if _, loaded := e.downloadCardKeys.LoadOrStore(sessionKey, true); loaded {
		return
	}
	tools.EmitCard(ctx, map[string]interface{}{
		"type":             "model-download-progress",
		"id":               "model-download-u2netp",
		"model_id":         e.u2netpModel.ModelID(),
		"title":            "Downloading lightweight cutout model",
		"message":          "Preparing subject cutout for future renders",
		"status":           "not_downloaded",
		"downloading":      false,
		"ready":            false,
		"state":            "not_downloaded",
		"status_url":       e.config.U2NetPStatusURL,
		"poll_interval_ms": 1500,
	})
}

func normalizeFallbackConfig(cfg FallbackConfig) FallbackConfig {
	if cfg.SearchMaxResults <= 0 {
		cfg.SearchMaxResults = 6
	}
	if cfg.ScreenshotWidth <= 0 {
		cfg.ScreenshotWidth = 1280
	}
	if cfg.ScreenshotHeight <= 0 {
		cfg.ScreenshotHeight = 896
	}
	if cfg.ComplexPromptChars <= 0 {
		cfg.ComplexPromptChars = 500
	}
	if len(cfg.SearchProviderChain) == 0 {
		cfg.SearchProviderChain = []string{"duckduckgo"}
	}
	presets := make([]FallbackPublicSpacePreset, 0, len(cfg.PublicSpaces))
	for _, preset := range cfg.PublicSpaces {
		if preset.PollInterval <= 0 {
			preset.PollInterval = 4 * time.Second
		}
		if preset.Timeout <= 0 {
			preset.Timeout = 4 * time.Minute
		}
		presets = append(presets, preset)
	}
	cfg.PublicSpaces = presets
	return cfg
}

// Enabled reports whether fallback execution is available.
func (e *FallbackEngine) Enabled() bool {
	return e != nil && e.config.Enabled
}

// Name returns the provider-like name used in task metadata.
func (e *FallbackEngine) Name() string { return fallbackProviderName }

// SupportsType reports whether the fallback engine can cover the media type.
func (e *FallbackEngine) SupportsType(t MediaType) bool {
	return t == MediaTypeImage || t == MediaTypeVideo
}

// SupportedModels returns all built-in fallback pseudo-models.
func (e *FallbackEngine) SupportedModels() []MediaModelInfo {
	return []MediaModelInfo{
		{ID: FallbackModelWebCanvasT2I, Name: "Fallback Web Canvas", Type: MediaTypeImage, Category: CategoryT2I, Provider: fallbackProviderName, IsFallback: true, FallbackStrategy: FallbackStrategyWebCanvas},
		{ID: FallbackModelSpaceT2I, Name: "Fallback Public Space (Image)", Type: MediaTypeImage, Category: CategoryT2I, Provider: fallbackProviderName, IsFallback: true, FallbackStrategy: FallbackStrategyPublicSpace},
		{ID: FallbackModelSpaceI2I, Name: "Fallback Public Space (Edit)", Type: MediaTypeImage, Category: CategoryI2I, Provider: fallbackProviderName, IsFallback: true, FallbackStrategy: FallbackStrategyPublicSpace},
		{ID: FallbackModelSpaceT2V, Name: "Fallback Public Space (Video)", Type: MediaTypeVideo, Category: CategoryT2V, Provider: fallbackProviderName, IsFallback: true, FallbackStrategy: FallbackStrategyPublicSpace},
		{ID: FallbackModelSpaceI2V, Name: "Fallback Public Space (Image to Video)", Type: MediaTypeVideo, Category: CategoryI2V, Provider: fallbackProviderName, IsFallback: true, FallbackStrategy: FallbackStrategyPublicSpace},
		{ID: FallbackModelSpaceKF2V, Name: "Fallback Public Space (Keyframe Video)", Type: MediaTypeVideo, Category: CategoryKF2V, Provider: fallbackProviderName, IsFallback: true, FallbackStrategy: FallbackStrategyPublicSpace},
	}
}

// IsFallbackModel reports whether the given model ID belongs to the built-in fallback catalog.
func (e *FallbackEngine) IsFallbackModel(modelID string) bool {
	switch strings.TrimSpace(modelID) {
	case FallbackModelWebCanvasT2I, FallbackModelSpaceT2I, FallbackModelSpaceI2I, FallbackModelSpaceT2V, FallbackModelSpaceI2V, FallbackModelSpaceKF2V:
		return true
	default:
		return false
	}
}

// DefaultModelForCategory returns the default fallback pseudo-model for a category.
func (e *FallbackEngine) DefaultModelForCategory(category MediaCategory) string {
	switch category {
	case CategoryT2I:
		return FallbackModelWebCanvasT2I
	case CategoryI2I:
		return FallbackModelSpaceI2I
	case CategoryT2V:
		return FallbackModelSpaceT2V
	case CategoryI2V:
		return FallbackModelSpaceI2V
	case CategoryKF2V:
		return FallbackModelSpaceKF2V
	default:
		return ""
	}
}

// ModelForRequest resolves the fallback pseudo-model for the request/category pair.
func (e *FallbackEngine) ModelForRequest(req *MediaRequest, category MediaCategory) string {
	if req != nil && e.IsFallbackModel(req.Model) {
		return req.Model
	}
	switch category {
	case CategoryT2I:
		if isComplexFallbackPrompt(req, category, e.config.ComplexPromptChars) {
			return FallbackModelSpaceT2I
		}
		return FallbackModelWebCanvasT2I
	case CategoryI2I:
		return FallbackModelSpaceI2I
	case CategoryT2V:
		return FallbackModelSpaceT2V
	case CategoryI2V:
		return FallbackModelSpaceI2V
	case CategoryKF2V:
		return FallbackModelSpaceKF2V
	default:
		return ""
	}
}

// PendingInfoForModel returns the disclosure payload that should be visible while a fallback task is running.
func (e *FallbackEngine) PendingInfoForModel(modelID string) *MediaFallbackInfo {
	if !e.IsFallbackModel(modelID) {
		return nil
	}
	strategy := FallbackStrategyPublicSpace
	displayName := fallbackDisplayName(strategy)
	spaceURL := ""
	if modelID == FallbackModelWebCanvasT2I {
		strategy = FallbackStrategyWebCanvas
		displayName = fallbackDisplayName(strategy)
	} else {
		if preset := e.firstPresetForModel(modelID); preset != nil {
			displayName = preset.DisplayName
			spaceURL = preset.URL
		}
	}
	return &MediaFallbackInfo{
		Used:        true,
		Strategy:    strategy,
		DisplayName: displayName,
		SpaceURL:    spaceURL,
		Disclosure:  fallbackDisclosure(e.locale, strategy),
	}
}

// Generate executes a built-in fallback request.
func (e *FallbackEngine) Generate(ctx context.Context, req *MediaRequest) (*MediaTask, error) {
	if !e.Enabled() {
		return nil, ErrProviderNotFound
	}
	category := inferCategoryFromRequest(req)
	modelID := e.ModelForRequest(req, category)
	switch modelID {
	case FallbackModelWebCanvasT2I:
		return e.generateWebCanvas(ctx, req)
	case FallbackModelSpaceT2I, FallbackModelSpaceI2I, FallbackModelSpaceT2V, FallbackModelSpaceI2V, FallbackModelSpaceKF2V:
		return e.generatePublicSpace(ctx, req, category, modelID)
	default:
		return nil, fmt.Errorf("%w: fallback category %s", ErrProviderNotFound, category)
	}
}

// Poll returns the state of an internal asynchronous fallback task.
func (e *FallbackEngine) Poll(ctx context.Context, taskID string) (*MediaTask, error) {
	_ = ctx
	value, ok := e.tasks.Load(strings.TrimSpace(taskID))
	if !ok {
		return nil, ErrTaskNotFound
	}
	task, ok := value.(*MediaTask)
	if !ok || task == nil {
		return nil, ErrTaskNotFound
	}
	return cloneMediaTask(task), nil
}

// RenderPage returns a previously staged internal render document.
func (e *FallbackEngine) RenderPage(token string) (string, bool) {
	if e == nil || e.renderStore == nil {
		return "", false
	}
	return e.renderStore.Get(token)
}

func (e *FallbackEngine) generateWebCanvas(ctx context.Context, req *MediaRequest) (*MediaTask, error) {
	if e.storage == nil {
		return nil, fmt.Errorf("fallback web_canvas render stage: media storage unavailable")
	}

	if e.sceneComposer != nil {
		if data, sourceURLs, composeErr := e.renderSceneCompose(ctx, req); composeErr == nil {
			return &MediaTask{
				BaseTask: basetask.BaseTask{
					Status:   TaskStatusSucceeded,
					Progress: 1,
				},
				Type:     MediaTypeImage,
				Provider: fallbackProviderName,
				Model:    FallbackModelWebCanvasT2I,
				Response: &MediaResponse{
					Created: timeutil.NowTime().Unix(),
					Data: []MediaResult{
						{
							B64JSON:       data,
							ContentType:   "image/png",
							Width:         e.config.ScreenshotWidth,
							Height:        e.config.ScreenshotHeight,
							RevisedPrompt: strings.TrimSpace(req.Prompt),
						},
					},
				},
				FallbackInfo: &MediaFallbackInfo{
					Used:        true,
					Strategy:    FallbackStrategyWebCanvas,
					DisplayName: fallbackDisplayName(FallbackStrategyWebCanvas),
					SourceURLs:  sourceURLs,
					Disclosure:  fallbackDisclosure(e.locale, FallbackStrategyWebCanvas),
				},
			}, nil
		}
	}

	results, err := e.searchPrompt(ctx, strings.TrimSpace(req.Prompt))
	if err != nil {
		results = nil
	}
	sourceURLs := make([]string, 0, len(results))
	for _, item := range results {
		if item.URL == "" {
			continue
		}
		sourceURLs = append(sourceURLs, item.URL)
	}
	candidates := e.collectImageCandidates(ctx, results)
	if data, renderErr := e.renderPosterPNG(req, candidates, results); renderErr == nil {
		return &MediaTask{
			BaseTask: basetask.BaseTask{
				Status:   TaskStatusSucceeded,
				Progress: 1,
			},
			Type:     MediaTypeImage,
			Provider: fallbackProviderName,
			Model:    FallbackModelWebCanvasT2I,
			Response: &MediaResponse{
				Created: timeutil.NowTime().Unix(),
				Data: []MediaResult{
					{
						B64JSON:       data,
						ContentType:   "image/png",
						Width:         e.config.ScreenshotWidth,
						Height:        e.config.ScreenshotHeight,
						RevisedPrompt: strings.TrimSpace(req.Prompt),
					},
				},
			},
			FallbackInfo: &MediaFallbackInfo{
				Used:        true,
				Strategy:    FallbackStrategyWebCanvas,
				DisplayName: fallbackDisplayName(FallbackStrategyWebCanvas),
				SourceURLs:  sourceURLs,
				Disclosure:  fallbackDisclosure(e.locale, FallbackStrategyWebCanvas),
			},
		}, nil
	}

	if e.browser == nil || e.browser() == nil {
		return nil, fmt.Errorf("fallback web_canvas screenshot stage: browser service unavailable")
	}
	htmlDoc, buildErr := e.buildPosterHTML(req, candidates, results)
	if buildErr != nil {
		return nil, fmt.Errorf("fallback web_canvas render stage: %w", buildErr)
	}
	data, err := e.capturePoster(ctx, htmlDoc)
	if err != nil {
		return nil, err
	}

	return &MediaTask{
		BaseTask: basetask.BaseTask{
			Status:   TaskStatusSucceeded,
			Progress: 1,
		},
		Type:     MediaTypeImage,
		Provider: fallbackProviderName,
		Model:    FallbackModelWebCanvasT2I,
		Response: &MediaResponse{
			Created: timeutil.NowTime().Unix(),
			Data: []MediaResult{
				{
					B64JSON:       data,
					ContentType:   "image/png",
					Width:         e.config.ScreenshotWidth,
					Height:        e.config.ScreenshotHeight,
					RevisedPrompt: strings.TrimSpace(req.Prompt),
				},
			},
		},
		FallbackInfo: &MediaFallbackInfo{
			Used:        true,
			Strategy:    FallbackStrategyWebCanvas,
			DisplayName: fallbackDisplayName(FallbackStrategyWebCanvas),
			SourceURLs:  sourceURLs,
			Disclosure:  fallbackDisclosure(e.locale, FallbackStrategyWebCanvas),
		},
	}, nil
}

func (e *FallbackEngine) renderSceneCompose(ctx context.Context, req *MediaRequest) (string, []string, error) {
	if e == nil || e.sceneComposer == nil {
		return "", nil, fmt.Errorf("scene compose is not configured")
	}
	result, err := e.sceneComposer.Compose(ctx, scenecompose.ComposeRequest{
		Prompt: strings.TrimSpace(req.Prompt),
		Width:  e.config.ScreenshotWidth,
		Height: e.config.ScreenshotHeight,
		Locale: e.locale,
	})
	if err != nil {
		return "", nil, err
	}
	if result == nil || result.Image == nil {
		return "", nil, fmt.Errorf("scene compose returned no image")
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, result.Image); err != nil {
		return "", nil, fmt.Errorf("encode scene compose image: %w", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), assetRefsToSourceURLs(result.UsedAssets), nil
}

func (e *FallbackEngine) generatePublicSpace(ctx context.Context, req *MediaRequest, category MediaCategory, modelID string) (*MediaTask, error) {
	internalID := uuid.New().String()
	task := &MediaTask{
		BaseTask: basetask.BaseTask{
			ID:       internalID,
			Status:   TaskStatusProcessing,
			Progress: 0.05,
		},
		Type:     req.Type,
		Category: string(category),
		Provider: fallbackProviderName,
		Model:    modelID,
		FallbackInfo: &MediaFallbackInfo{
			Used:        true,
			Strategy:    FallbackStrategyPublicSpace,
			DisplayName: fallbackDisplayName(FallbackStrategyPublicSpace),
			Disclosure:  fallbackDisclosure(e.locale, FallbackStrategyPublicSpace),
		},
	}
	e.tasks.Store(internalID, cloneMediaTask(task))

	go e.runPublicSpaceTask(internalID, cloneMediaRequest(req), category, modelID)

	return &MediaTask{
		BaseTask: basetask.BaseTask{
			Status:   TaskStatusProcessing,
			Progress: 0.05,
		},
		Type:         req.Type,
		Provider:     fallbackProviderName,
		Model:        modelID,
		UpstreamID:   internalID,
		FallbackInfo: cloneFallbackInfo(task.FallbackInfo),
	}, nil
}

func (e *FallbackEngine) runPublicSpaceTask(taskID string, req *MediaRequest, category MediaCategory, modelID string) {
	files, cleanup, err := e.prepareReferenceFiles(context.Background(), req)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		e.failInternalTask(taskID, fmt.Sprintf("fallback public_space input stage: %v", err))
		return
	}

	presets := e.presetsForCategory(category)
	if len(presets) == 0 {
		e.failInternalTask(taskID, fmt.Sprintf("fallback public_space preset stage: no preset for category %s", category))
		return
	}

	var failures []string
	for idx, preset := range presets {
		progress := 0.1 + (float64(idx)/float64(len(presets)+1))*0.25
		e.updateInternalTask(taskID, func(task *MediaTask) {
			task.Progress = progress
			task.FallbackInfo = &MediaFallbackInfo{
				Used:        true,
				Strategy:    FallbackStrategyPublicSpace,
				DisplayName: preset.DisplayName,
				SpaceURL:    preset.URL,
				Disclosure:  fallbackDisclosure(e.locale, FallbackStrategyPublicSpace),
			}
		})

		result, runErr := e.runSpacePreset(context.Background(), preset, req, files, modelID)
		if runErr != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", preset.DisplayName, runErr))
			continue
		}

		e.updateInternalTask(taskID, func(task *MediaTask) {
			task.Status = TaskStatusSucceeded
			task.Progress = 1
			task.Response = result.Response
			task.FallbackInfo = result.FallbackInfo
		})
		return
	}

	if len(failures) == 0 {
		failures = append(failures, "no public space preset produced a result")
	}
	e.failInternalTask(taskID, "fallback public_space failed: "+strings.Join(failures, "; "))
}

func (e *FallbackEngine) runSpacePreset(ctx context.Context, preset FallbackPublicSpacePreset, req *MediaRequest, files []string, modelID string) (*MediaTask, error) {
	svc := e.browser()
	if svc == nil {
		return nil, fmt.Errorf("browser service unavailable")
	}
	if err := svc.Start(ctx); err != nil {
		return nil, fmt.Errorf("browser start: %w", err)
	}
	tab, err := svc.OpenTab(ctx, preset.URL)
	if err != nil {
		return nil, fmt.Errorf("open preset: %w", err)
	}
	defer svc.CloseTab(context.Background(), tab.TargetID)

	if err := e.waitForAnySelector(ctx, svc, tab.TargetID, preset.ReadySelectors, preset.Timeout/6); err != nil {
		return nil, fmt.Errorf("ready stage: %w", err)
	}
	if err := e.typeIntoFirstSelector(ctx, svc, tab.TargetID, preset.PromptSelectors, strings.TrimSpace(req.Prompt)); err != nil {
		return nil, fmt.Errorf("prompt stage: %w", err)
	}
	if strings.TrimSpace(req.NegativePrompt) != "" && len(preset.NegativePromptSelectors) > 0 {
		if err := e.typeIntoFirstSelector(ctx, svc, tab.TargetID, preset.NegativePromptSelectors, strings.TrimSpace(req.NegativePrompt)); err != nil {
			return nil, fmt.Errorf("negative prompt stage: %w", err)
		}
	}
	if len(files) > 0 {
		if err := e.uploadFiles(ctx, svc, tab.TargetID, preset.UploadSelectors, files); err != nil {
			return nil, fmt.Errorf("upload stage: %w", err)
		}
	}
	if err := e.clickFirstSelector(ctx, svc, tab.TargetID, preset.SubmitSelectors); err != nil {
		return nil, fmt.Errorf("submit stage: %w", err)
	}

	deadline := time.Now().Add(preset.Timeout)
	for time.Now().Before(deadline) {
		urlValue, _, infoErr := svc.PageInfo(ctx, tab.TargetID)
		if infoErr == nil && looksLikeServableMediaURL(urlValue) {
			return e.storePublicSpaceResult(urlValue, mediaTypeForFallbackModel(modelID), preset)
		}

		if msg := e.extractFirstText(ctx, svc, tab.TargetID, preset.ErrorSelectors); strings.TrimSpace(msg) != "" {
			return nil, fmt.Errorf("space reported error: %s", msg)
		}

		if resultURL, kind := e.extractResultURL(ctx, svc, tab.TargetID, preset); resultURL != "" {
			return e.storePublicSpaceResult(resultURL, mediaTypeForResultKind(kind, modelID), preset)
		}

		time.Sleep(preset.PollInterval)
	}

	return nil, fmt.Errorf("timeout while waiting for %s", preset.DisplayName)
}

func (e *FallbackEngine) storePublicSpaceResult(rawURL string, mediaType MediaType, preset FallbackPublicSpacePreset) (*MediaTask, error) {
	if e.storage == nil {
		return nil, fmt.Errorf("media storage unavailable")
	}
	contentType, payload, err := e.downloadAsset(context.Background(), rawURL)
	if err != nil {
		if mediaType == MediaTypeImage && strings.HasPrefix(rawURL, "blob:") {
			return nil, fmt.Errorf("download result: blob URLs are not persistable")
		}
		return nil, fmt.Errorf("download result: %w", err)
	}
	localURL, err := e.storage.StoreBytes(payload, contentType, mediaType)
	if err != nil {
		return nil, fmt.Errorf("store result: %w", err)
	}

	result := MediaResult{
		URL:         localURL,
		ContentType: contentType,
	}
	if mediaType == MediaTypeImage {
		result.Width = e.config.ScreenshotWidth
		result.Height = e.config.ScreenshotHeight
	}
	return &MediaTask{
		BaseTask: basetask.BaseTask{
			Status:   TaskStatusSucceeded,
			Progress: 1,
		},
		Provider: fallbackProviderName,
		Response: &MediaResponse{
			Created: timeutil.NowTime().Unix(),
			Data:    []MediaResult{result},
		},
		FallbackInfo: &MediaFallbackInfo{
			Used:        true,
			Strategy:    FallbackStrategyPublicSpace,
			DisplayName: preset.DisplayName,
			SpaceURL:    preset.URL,
			Disclosure:  fallbackDisclosure(e.locale, FallbackStrategyPublicSpace),
		},
	}, nil
}

func (e *FallbackEngine) searchPrompt(ctx context.Context, prompt string) ([]FallbackSearchResult, error) {
	if e.searcher == nil {
		return nil, fmt.Errorf("searcher unavailable")
	}
	return e.searcher.Search(ctx, prompt, e.config.SearchMaxResults, e.config.SearchProviderChain)
}

func (e *FallbackEngine) GetFallbackModelStatus(modelID string) (*scenecompose.ModelStatus, error) {
	if e == nil {
		return nil, fmt.Errorf("fallback engine unavailable")
	}
	switch strings.TrimSpace(strings.ToLower(modelID)) {
	case "u2netp":
		if e.u2netpModel == nil {
			return nil, fmt.Errorf("fallback model %s unavailable", modelID)
		}
		return e.u2netpModel.GetStatus(), nil
	default:
		return nil, fmt.Errorf("unsupported fallback model %s", modelID)
	}
}

type fallbackImageCandidate struct {
	Title    string
	Source   string
	DataURL  string
	LocalURL string
}

func (e *FallbackEngine) collectImageCandidates(ctx context.Context, results []FallbackSearchResult) []fallbackImageCandidate {
	if len(results) == 0 || e.storage == nil {
		return nil
	}
	candidates := make([]fallbackImageCandidate, 0, 4)
	for _, item := range results {
		if len(candidates) >= 4 {
			break
		}
		sourceURL, data, contentType, err := e.resolveSearchImage(ctx, item.URL)
		if err != nil || len(data) == 0 {
			continue
		}
		localURL, err := e.storage.StoreBytes(data, contentType, MediaTypeImage)
		if err != nil {
			continue
		}
		candidates = append(candidates, fallbackImageCandidate{
			Title:    item.Title,
			Source:   sourceURL,
			LocalURL: localURL,
			DataURL:  bytesToDataURL(data, contentType),
		})
	}
	return candidates
}

func (e *FallbackEngine) resolveSearchImage(ctx context.Context, rawURL string) (string, []byte, string, error) {
	contentType, payload, err := e.downloadAsset(ctx, rawURL)
	if err == nil && strings.HasPrefix(contentType, "image/") {
		return rawURL, payload, contentType, nil
	}

	body, finalURL, err := e.fetchHTML(ctx, rawURL)
	if err != nil {
		return "", nil, "", err
	}
	imageURL := extractOGImage(body, finalURL)
	if imageURL == "" {
		return "", nil, "", fmt.Errorf("no og:image")
	}
	contentType, payload, err = e.downloadAsset(ctx, imageURL)
	if err != nil {
		return "", nil, "", err
	}
	if !strings.HasPrefix(contentType, "image/") {
		return "", nil, "", fmt.Errorf("resolved asset is not image")
	}
	return imageURL, payload, contentType, nil
}

func (e *FallbackEngine) buildPosterHTML(req *MediaRequest, candidates []fallbackImageCandidate, results []FallbackSearchResult) (string, error) {
	promptJSON, _ := json.Marshal(strings.TrimSpace(req.Prompt))
	keywordsJSON, _ := json.Marshal(extractPosterKeywords(req.Prompt))
	images := make([]map[string]string, 0, len(candidates))
	for _, candidate := range candidates {
		images = append(images, map[string]string{
			"title": candidate.Title,
			"src":   candidate.DataURL,
		})
	}
	imagesJSON, _ := json.Marshal(images)
	sources := make([]map[string]string, 0, len(results))
	for _, item := range results {
		if item.URL == "" {
			continue
		}
		sources = append(sources, map[string]string{
			"title": item.Title,
			"url":   item.URL,
		})
	}
	sourcesJSON, _ := json.Marshal(sources)

	return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Fallback Render</title>
<style>
html, body { margin:0; padding:0; background:#0f172a; width:100%%; height:100%%; overflow:hidden; }
body { display:flex; align-items:center; justify-content:center; }
canvas { width:%dpx; height:%dpx; display:block; }
#fallback-render-ready { position:fixed; left:-9999px; top:-9999px; }
</style>
</head>
<body>
<canvas id="poster" width="%d" height="%d"></canvas>
<div id="fallback-render-ready"></div>
<script>
const promptText = %s;
const keywords = %s;
const images = %s;
const sources = %s;
const canvas = document.getElementById('poster');
const ctx = canvas.getContext('2d');

function roundRect(ctx, x, y, w, h, r) {
  ctx.beginPath();
  ctx.moveTo(x + r, y);
  ctx.arcTo(x + w, y, x + w, y + h, r);
  ctx.arcTo(x + w, y + h, x, y + h, r);
  ctx.arcTo(x, y + h, x, y, r);
  ctx.arcTo(x, y, x + w, y, r);
  ctx.closePath();
}

function wrapText(text, maxChars) {
  const words = text.split(/\s+/).filter(Boolean);
  const lines = [];
  let line = '';
  for (const word of words) {
    const next = line ? line + ' ' + word : word;
    if (next.length > maxChars && line) {
      lines.push(line);
      line = word;
    } else {
      line = next;
    }
  }
  if (line) lines.push(line);
  return lines.slice(0, 4);
}

function loadImage(src) {
  return new Promise((resolve) => {
    const img = new Image();
    img.onload = () => resolve(img);
    img.onerror = () => resolve(null);
    img.src = src;
  });
}

async function render() {
  const width = canvas.width;
  const height = canvas.height;
  const gradient = ctx.createLinearGradient(0, 0, width, height);
  gradient.addColorStop(0, '#0f172a');
  gradient.addColorStop(0.5, '#1d4ed8');
  gradient.addColorStop(1, '#0b1120');
  ctx.fillStyle = gradient;
  ctx.fillRect(0, 0, width, height);

  ctx.fillStyle = 'rgba(255,255,255,0.08)';
  for (let i = 0; i < 18; i++) {
    const x = (i * 97) %% width;
    const y = (i * 53) %% height;
    ctx.beginPath();
    ctx.arc(x, y, 60 + (i %% 4) * 15, 0, Math.PI * 2);
    ctx.fill();
  }

  ctx.fillStyle = '#f8fafc';
  ctx.font = 'bold 60px sans-serif';
  ctx.fillText('Fallback Poster', 72, 108);
  ctx.font = '28px sans-serif';
  ctx.fillStyle = 'rgba(248,250,252,0.88)';
  ctx.fillText('Web search + local canvas render', 74, 150);

  ctx.fillStyle = 'rgba(255,255,255,0.14)';
  roundRect(ctx, 64, 186, 540, 612, 28);
  ctx.fill();
  ctx.fillStyle = '#e2e8f0';
  ctx.font = 'bold 34px sans-serif';
  ctx.fillText('Prompt', 94, 236);

  ctx.font = '26px sans-serif';
  const promptLines = wrapText(promptText, 24);
  promptLines.forEach((line, idx) => {
    ctx.fillStyle = '#f8fafc';
    ctx.fillText(line, 94, 292 + idx * 40);
  });

  ctx.fillStyle = '#93c5fd';
  ctx.font = 'bold 24px sans-serif';
  ctx.fillText('Keywords', 94, 468);

  const keywordPills = keywords.slice(0, 6);
  let px = 94;
  let py = 506;
  ctx.font = '20px sans-serif';
  for (const keyword of keywordPills) {
    const pillWidth = Math.min(220, Math.max(96, ctx.measureText(keyword).width + 34));
    if (px + pillWidth > 560) {
      px = 94;
      py += 54;
    }
    ctx.fillStyle = 'rgba(15,23,42,0.38)';
    roundRect(ctx, px, py - 28, pillWidth, 38, 18);
    ctx.fill();
    ctx.fillStyle = '#bfdbfe';
    ctx.fillText(keyword, px + 16, py);
    px += pillWidth + 12;
  }

  ctx.fillStyle = '#cbd5e1';
  ctx.font = '22px sans-serif';
  ctx.fillText(images.length > 0 ? 'Collected image references' : 'No usable image found, text poster rendered instead', 94, 718);

  const loadedImages = await Promise.all(images.map((item) => loadImage(item.src)));
  const frameX = 650;
  const frameY = 110;
  const frameW = 560;
  const frameH = 680;
  ctx.fillStyle = 'rgba(255,255,255,0.12)';
  roundRect(ctx, frameX, frameY, frameW, frameH, 32);
  ctx.fill();

  if (loadedImages.some(Boolean)) {
    const cells = [
      [frameX + 26, frameY + 26, 240, 220],
      [frameX + 294, frameY + 26, 240, 220],
      [frameX + 26, frameY + 274, 240, 360],
      [frameX + 294, frameY + 274, 240, 360],
    ];
    loadedImages.forEach((img, idx) => {
      if (!img || !cells[idx]) return;
      const [x, y, w, h] = cells[idx];
      ctx.save();
      roundRect(ctx, x, y, w, h, 24);
      ctx.clip();
      const scale = Math.max(w / img.width, h / img.height);
      const drawW = img.width * scale;
      const drawH = img.height * scale;
      ctx.drawImage(img, x + (w - drawW) / 2, y + (h - drawH) / 2, drawW, drawH);
      ctx.restore();
      ctx.strokeStyle = 'rgba(255,255,255,0.18)';
      ctx.lineWidth = 2;
      roundRect(ctx, x, y, w, h, 24);
      ctx.stroke();
    });
  } else {
    ctx.fillStyle = 'rgba(15,23,42,0.42)';
    roundRect(ctx, frameX + 34, frameY + 34, frameW - 68, frameH - 68, 24);
    ctx.fill();
    ctx.strokeStyle = 'rgba(147,197,253,0.35)';
    ctx.lineWidth = 3;
    roundRect(ctx, frameX + 34, frameY + 34, frameW - 68, frameH - 68, 24);
    ctx.stroke();
    ctx.fillStyle = '#dbeafe';
    ctx.font = 'bold 44px sans-serif';
    ctx.fillText('Text-only', frameX + 160, frameY + 280);
    ctx.font = '28px sans-serif';
    ctx.fillStyle = '#cbd5e1';
    ctx.fillText('Search results had no directly usable image.', frameX + 62, frameY + 340);
  }

  ctx.fillStyle = 'rgba(255,255,255,0.86)';
  ctx.font = '20px sans-serif';
  ctx.fillText('Sources', 74, 840);
  ctx.fillStyle = 'rgba(226,232,240,0.76)';
  sources.slice(0, 3).forEach((item, idx) => {
    const label = (item.title || item.url || '').slice(0, 52);
    ctx.fillText((idx + 1) + '. ' + label, 146, 840 + idx * 24);
  });

  document.getElementById('fallback-render-ready').className = 'ready';
}
render();
</script>
</body>
</html>`, e.config.ScreenshotWidth, e.config.ScreenshotHeight, e.config.ScreenshotWidth, e.config.ScreenshotHeight, string(promptJSON), string(keywordsJSON), string(imagesJSON), string(sourcesJSON)), nil
}

func (e *FallbackEngine) renderPosterPNG(req *MediaRequest, candidates []fallbackImageCandidate, results []FallbackSearchResult) (string, error) {
	width := e.config.ScreenshotWidth
	height := e.config.ScreenshotHeight
	if width <= 0 {
		width = 1280
	}
	if height <= 0 {
		height = 896
	}

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	top := color.RGBA{R: 69, G: 39, B: 24, A: 255}
	bottom := color.RGBA{R: 24, G: 17, B: 13, A: 255}
	if looksCoolPrompt(req) {
		top = color.RGBA{R: 33, G: 40, B: 63, A: 255}
		bottom = color.RGBA{R: 12, G: 18, B: 31, A: 255}
	}
	fillVerticalGradient(img, img.Bounds(), top, bottom)

	fillRect(img, image.Rect(0, int(float64(height)*0.64), width, height), color.RGBA{R: 58, G: 36, B: 24, A: 255})
	fillRect(img, image.Rect(0, int(float64(height)*0.68), width, height), color.RGBA{R: 78, G: 48, B: 30, A: 255})

	for i := 0; i < 10; i++ {
		x := 70 + i*115
		fillCircle(img, x, 88+(i%2)*8, 7, color.RGBA{R: 255, G: 224, B: 147, A: 220})
	}

	windowRect := image.Rect(width-300, 120, width-70, 410)
	fillRect(img, windowRect, color.RGBA{R: 120, G: 98, B: 86, A: 255})
	fillRect(img, insetRect(windowRect, 14), color.RGBA{R: 236, G: 198, B: 137, A: 255})
	fillRect(img, image.Rect(windowRect.Min.X+(windowRect.Dx()/2)-6, windowRect.Min.Y+14, windowRect.Min.X+(windowRect.Dx()/2)+6, windowRect.Max.Y-14), color.RGBA{R: 120, G: 98, B: 86, A: 255})
	fillRect(img, image.Rect(windowRect.Min.X+14, windowRect.Min.Y+(windowRect.Dy()/2)-6, windowRect.Max.X-14, windowRect.Min.Y+(windowRect.Dy()/2)+6), color.RGBA{R: 120, G: 98, B: 86, A: 255})

	for row := 0; row < 3; row++ {
		shelfY := 170 + row*105
		fillRect(img, image.Rect(90, shelfY, 390, shelfY+12), color.RGBA{R: 110, G: 75, B: 47, A: 255})
		for col := 0; col < 9; col++ {
			bookX := 100 + col*31
			bookH := 54 + (col%3)*14
			bookColor := []color.RGBA{
				{R: 142, G: 87, B: 62, A: 255},
				{R: 82, G: 121, B: 118, A: 255},
				{R: 162, G: 118, B: 64, A: 255},
			}[col%3]
			fillRect(img, image.Rect(bookX, shelfY-bookH, bookX+20, shelfY), bookColor)
		}
	}

	tableTop := image.Rect(210, 600, 1080, 660)
	fillRect(img, tableTop, color.RGBA{R: 126, G: 83, B: 50, A: 255})
	fillRect(img, image.Rect(250, 660, 290, 860), color.RGBA{R: 92, G: 60, B: 38, A: 255})
	fillRect(img, image.Rect(960, 660, 1000, 860), color.RGBA{R: 92, G: 60, B: 38, A: 255})

	drawRobotCafeScene(img, width, height, req, candidates, results)

	labelFace := basicfont.Face7x13
	drawTextLine(img, labelFace, 62, 52, "Blue Local Render", color.RGBA{R: 250, G: 241, B: 228, A: 255})
	drawWrappedText(img, labelFace, image.Rect(56, height-140, width-56, height-32), strings.TrimSpace(req.Prompt), color.RGBA{R: 245, G: 235, B: 220, A: 255}, 2)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func drawRobotCafeScene(img *image.RGBA, width, height int, req *MediaRequest, candidates []fallbackImageCandidate, results []FallbackSearchResult) {
	centerX := width / 2
	headRect := image.Rect(centerX-96, 250, centerX+96, 410)
	fillRect(img, headRect, color.RGBA{R: 185, G: 196, B: 205, A: 255})
	fillRect(img, insetRect(headRect, 10), color.RGBA{R: 206, G: 214, B: 221, A: 255})

	fillRect(img, image.Rect(centerX-9, 205, centerX+9, 250), color.RGBA{R: 180, G: 190, B: 198, A: 255})
	fillCircle(img, centerX, 195, 18, color.RGBA{R: 255, G: 201, B: 120, A: 255})
	fillCircle(img, centerX-44, 320, 18, color.RGBA{R: 255, G: 230, B: 164, A: 255})
	fillCircle(img, centerX+44, 320, 18, color.RGBA{R: 255, G: 230, B: 164, A: 255})
	fillCircle(img, centerX-44, 320, 9, color.RGBA{R: 96, G: 129, B: 179, A: 255})
	fillCircle(img, centerX+44, 320, 9, color.RGBA{R: 96, G: 129, B: 179, A: 255})
	fillRect(img, image.Rect(centerX-36, 364, centerX+36, 372), color.RGBA{R: 104, G: 117, B: 129, A: 255})

	bodyRect := image.Rect(centerX-120, 420, centerX+120, 610)
	fillRect(img, bodyRect, color.RGBA{R: 170, G: 183, B: 194, A: 255})
	fillRect(img, insetRect(bodyRect, 12), color.RGBA{R: 193, G: 204, B: 214, A: 255})
	fillRect(img, image.Rect(centerX-20, 452, centerX+20, 590), color.RGBA{R: 135, G: 149, B: 161, A: 255})

	fillRect(img, image.Rect(centerX-196, 470, centerX-118, 495), color.RGBA{R: 168, G: 181, B: 192, A: 255})
	fillRect(img, image.Rect(centerX+118, 470, centerX+196, 495), color.RGBA{R: 168, G: 181, B: 192, A: 255})
	fillRect(img, image.Rect(centerX-162, 494, centerX-142, 572), color.RGBA{R: 168, G: 181, B: 192, A: 255})
	fillRect(img, image.Rect(centerX+142, 494, centerX+162, 572), color.RGBA{R: 168, G: 181, B: 192, A: 255})
	fillCircle(img, centerX-152, 582, 16, color.RGBA{R: 255, G: 214, B: 170, A: 255})
	fillCircle(img, centerX+152, 582, 16, color.RGBA{R: 255, G: 214, B: 170, A: 255})

	fillRect(img, image.Rect(centerX-86, 606, centerX-58, 760), color.RGBA{R: 168, G: 181, B: 192, A: 255})
	fillRect(img, image.Rect(centerX+58, 606, centerX+86, 760), color.RGBA{R: 168, G: 181, B: 192, A: 255})
	fillRect(img, image.Rect(centerX-118, 756, centerX-34, 786), color.RGBA{R: 95, G: 103, B: 110, A: 255})
	fillRect(img, image.Rect(centerX+34, 756, centerX+118, 786), color.RGBA{R: 95, G: 103, B: 110, A: 255})

	bookLeft := image.Rect(centerX-132, 520, centerX-16, 610)
	bookRight := image.Rect(centerX+16, 520, centerX+132, 610)
	fillRect(img, bookLeft, color.RGBA{R: 247, G: 238, B: 222, A: 255})
	fillRect(img, bookRight, color.RGBA{R: 247, G: 238, B: 222, A: 255})
	fillRect(img, image.Rect(centerX-4, 518, centerX+4, 614), color.RGBA{R: 171, G: 140, B: 96, A: 255})
	fillRect(img, image.Rect(centerX-116, 540, centerX-24, 544), color.RGBA{R: 190, G: 174, B: 141, A: 255})
	fillRect(img, image.Rect(centerX+24, 540, centerX+116, 544), color.RGBA{R: 190, G: 174, B: 141, A: 255})
	fillRect(img, image.Rect(centerX-110, 566, centerX-18, 570), color.RGBA{R: 190, G: 174, B: 141, A: 255})
	fillRect(img, image.Rect(centerX+18, 566, centerX+110, 570), color.RGBA{R: 190, G: 174, B: 141, A: 255})

	fillRect(img, image.Rect(centerX-275, 535, centerX-210, 592), color.RGBA{R: 216, G: 238, B: 244, A: 255})
	fillRect(img, image.Rect(centerX-284, 592, centerX-201, 603), color.RGBA{R: 214, G: 197, B: 162, A: 255})
	fillRect(img, image.Rect(centerX-230, 516, centerX-213, 538), color.RGBA{R: 216, G: 238, B: 244, A: 255})
	fillCircle(img, centerX-222, 518, 12, color.RGBA{R: 216, G: 238, B: 244, A: 255})

	if looksLikeBookPrompt(req) {
		fillRect(img, image.Rect(120, 725, 330, 775), color.RGBA{R: 92, G: 67, B: 55, A: 210})
	}
	if len(candidates) > 0 || len(results) > 0 {
		fillRect(img, image.Rect(width-248, 490, width-94, 700), color.RGBA{R: 244, G: 233, B: 209, A: 230})
		fillRect(img, image.Rect(width-230, 512, width-112, 590), color.RGBA{R: 206, G: 173, B: 118, A: 255})
		fillRect(img, image.Rect(width-230, 606, width-112, 678), color.RGBA{R: 157, G: 184, B: 183, A: 255})
	}
}

func looksCoolPrompt(req *MediaRequest) bool {
	prompt := ""
	if req != nil {
		prompt = strings.ToLower(strings.TrimSpace(req.Prompt))
	}
	return strings.Contains(prompt, "night") || strings.Contains(prompt, "neon") || strings.Contains(prompt, "cyber")
}

func looksLikeBookPrompt(req *MediaRequest) bool {
	prompt := ""
	if req != nil {
		prompt = strings.ToLower(strings.TrimSpace(req.Prompt))
	}
	return strings.Contains(prompt, "book") || strings.Contains(prompt, "read") || strings.Contains(prompt, "阅读") || strings.Contains(prompt, "书")
}

func fillVerticalGradient(img *image.RGBA, rect image.Rectangle, top, bottom color.RGBA) {
	if rect.Dy() <= 0 {
		return
	}
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		t := float64(y-rect.Min.Y) / float64(rect.Dy())
		c := color.RGBA{
			R: uint8(float64(top.R)*(1-t) + float64(bottom.R)*t),
			G: uint8(float64(top.G)*(1-t) + float64(bottom.G)*t),
			B: uint8(float64(top.B)*(1-t) + float64(bottom.B)*t),
			A: 255,
		}
		fillRect(img, image.Rect(rect.Min.X, y, rect.Max.X, y+1), c)
	}
}

func fillRect(img *image.RGBA, rect image.Rectangle, c color.Color) {
	if img == nil {
		return
	}
	rect = rect.Intersect(img.Bounds())
	if rect.Empty() {
		return
	}
	stdDraw.Draw(img, rect, &image.Uniform{C: c}, image.Point{}, stdDraw.Src)
}

func fillCircle(img *image.RGBA, cx, cy, r int, c color.Color) {
	if img == nil || r <= 0 {
		return
	}
	bounds := img.Bounds()
	for y := cy - r; y <= cy+r; y++ {
		if y < bounds.Min.Y || y >= bounds.Max.Y {
			continue
		}
		for x := cx - r; x <= cx+r; x++ {
			if x < bounds.Min.X || x >= bounds.Max.X {
				continue
			}
			dx := x - cx
			dy := y - cy
			if dx*dx+dy*dy <= r*r {
				img.Set(x, y, c)
			}
		}
	}
}

func insetRect(rect image.Rectangle, inset int) image.Rectangle {
	return image.Rect(rect.Min.X+inset, rect.Min.Y+inset, rect.Max.X-inset, rect.Max.Y-inset)
}

func drawWrappedText(img *image.RGBA, face font.Face, rect image.Rectangle, text string, c color.Color, maxLines int) {
	lines := wrapPosterText(text, 42)
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[:maxLines]
		last := strings.TrimSpace(lines[len(lines)-1])
		if !strings.HasSuffix(last, "...") {
			lines[len(lines)-1] = strings.TrimRight(last, ". ") + "..."
		}
	}
	for idx, line := range lines {
		y := rect.Min.Y + idx*18
		if y > rect.Max.Y {
			break
		}
		drawTextLine(img, face, rect.Min.X, y, line, c)
	}
}

func wrapPosterText(text string, maxChars int) []string {
	words := strings.Fields(strings.TrimSpace(text))
	if len(words) == 0 {
		return nil
	}
	lines := make([]string, 0, 4)
	line := ""
	for _, word := range words {
		candidate := word
		if line != "" {
			candidate = line + " " + word
		}
		if utf8.RuneCountInString(candidate) > maxChars && line != "" {
			lines = append(lines, line)
			line = word
			continue
		}
		line = candidate
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

func drawTextLine(img *image.RGBA, face font.Face, x, y int, text string, c color.Color) {
	if img == nil || face == nil || strings.TrimSpace(text) == "" {
		return
	}
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(c),
		Face: face,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(text)
}

func (e *FallbackEngine) capturePoster(ctx context.Context, htmlDoc string) (string, error) {
	if strings.TrimSpace(e.config.RenderBaseURL) == "" {
		return "", fmt.Errorf("fallback web_canvas render stage: render base URL is not configured")
	}
	token := e.renderStore.Put(htmlDoc)
	defer e.renderStore.Delete(token)

	renderURL := strings.TrimRight(e.config.RenderBaseURL, "/") + "/api/v1/media/fallback/render/" + token
	readySelector := fallbackRenderReadySelector
	resp, err := e.browser().Screenshot(ctx, &browser.ScreenshotRequest{
		URL:             renderURL,
		Format:          browser.FormatPNG,
		Width:           e.config.ScreenshotWidth,
		Height:          e.config.ScreenshotHeight,
		WaitFor:         250,
		WaitForSelector: &readySelector,
		Timeout:         int((45 * time.Second).Milliseconds()),
	})
	if err != nil {
		return "", fmt.Errorf("fallback web_canvas screenshot stage: %w", err)
	}
	return resp.Data, nil
}

func (e *FallbackEngine) presetsForCategory(category MediaCategory) []FallbackPublicSpacePreset {
	available := make([]FallbackPublicSpacePreset, 0, len(e.config.PublicSpaces))
	for _, preset := range e.config.PublicSpaces {
		for _, supported := range preset.Categories {
			if supported == category {
				available = append(available, preset)
				break
			}
		}
	}
	return available
}

func (e *FallbackEngine) firstPresetForModel(modelID string) *FallbackPublicSpacePreset {
	category := inferCategoryFromFallbackModel(modelID)
	if category == CategoryNone {
		return nil
	}
	presets := e.presetsForCategory(category)
	if len(presets) == 0 {
		return nil
	}
	return &presets[0]
}

func (e *FallbackEngine) waitForAnySelector(ctx context.Context, svc FallbackBrowserService, targetID string, selectors []string, timeout time.Duration) error {
	if len(selectors) == 0 {
		return nil
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, selector := range selectors {
			ok, err := svc.ElementExists(ctx, targetID, selector)
			if err == nil && ok {
				return nil
			}
		}
		time.Sleep(600 * time.Millisecond)
	}
	return fmt.Errorf("no ready selector matched")
}

func (e *FallbackEngine) typeIntoFirstSelector(ctx context.Context, svc FallbackBrowserService, targetID string, selectors []string, value string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	for _, selector := range selectors {
		ok, err := svc.ElementExists(ctx, targetID, selector)
		if err != nil || !ok {
			continue
		}
		_, err = svc.Act(ctx, &browser.ActRequest{
			Kind:     "type",
			TargetID: targetID,
			Selector: selector,
			Text:     value,
			Value:    value,
		})
		if err == nil {
			return nil
		}
	}
	return fmt.Errorf("no prompt selector matched")
}

func (e *FallbackEngine) uploadFiles(ctx context.Context, svc FallbackBrowserService, targetID string, selectors []string, files []string) error {
	if len(files) == 0 {
		return nil
	}
	if len(selectors) == 0 {
		return fmt.Errorf("no upload selector configured")
	}
	for _, selector := range selectors {
		ok, err := svc.ElementExists(ctx, targetID, selector)
		if err != nil || !ok {
			continue
		}
		_, err = svc.Act(ctx, &browser.ActRequest{
			Kind:     "upload",
			TargetID: targetID,
			Selector: selector,
			Files:    files,
		})
		if err == nil {
			return nil
		}
	}
	return fmt.Errorf("no upload selector matched")
}

func (e *FallbackEngine) clickFirstSelector(ctx context.Context, svc FallbackBrowserService, targetID string, selectors []string) error {
	for _, selector := range selectors {
		ok, err := svc.ElementExists(ctx, targetID, selector)
		if err != nil || !ok {
			continue
		}
		_, err = svc.Act(ctx, &browser.ActRequest{
			Kind:     "click",
			TargetID: targetID,
			Selector: selector,
		})
		if err == nil {
			return nil
		}
	}
	return fmt.Errorf("no submit selector matched")
}

func (e *FallbackEngine) extractFirstText(ctx context.Context, svc FallbackBrowserService, targetID string, selectors []string) string {
	for _, selector := range selectors {
		text, err := svc.ExtractFirstFromTab(ctx, targetID, selector, "")
		if err == nil && strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

func (e *FallbackEngine) extractResultURL(ctx context.Context, svc FallbackBrowserService, targetID string, preset FallbackPublicSpacePreset) (string, string) {
	currentURL, _, _ := svc.PageInfo(ctx, targetID)
	for _, selector := range preset.SuccessSelectors {
		for _, css := range selector.Selectors {
			value, err := svc.ExtractFirstFromTab(ctx, targetID, css, selector.Attribute)
			if err != nil || strings.TrimSpace(value) == "" {
				continue
			}
			resolved := resolveRelativeURL(currentURL, value)
			if resolved != "" {
				return resolved, selector.Kind
			}
		}
	}
	return "", ""
}

func (e *FallbackEngine) prepareReferenceFiles(ctx context.Context, req *MediaRequest) ([]string, func(), error) {
	references := make([]string, 0, len(req.ReferenceURLs)+1)
	for _, item := range req.ReferenceURLs {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			references = append(references, trimmed)
		}
	}
	if len(references) == 0 && strings.TrimSpace(req.ReferenceURL) != "" {
		references = append(references, strings.TrimSpace(req.ReferenceURL))
	}
	if len(references) == 0 && len(req.ReferenceImage) == 0 {
		return nil, nil, nil
	}

	dir, err := os.MkdirTemp("", "mediagen-fallback-*")
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }

	paths := make([]string, 0, len(references)+1)
	if len(req.ReferenceImage) > 0 {
		path, err := writeReferenceTempFile(dir, req.ReferenceImage, "image/png")
		if err != nil {
			cleanup()
			return nil, nil, err
		}
		paths = append(paths, path)
	}
	for _, item := range references {
		contentType, payload, err := e.readReferencePayload(ctx, item)
		if err != nil {
			cleanup()
			return nil, nil, err
		}
		path, err := writeReferenceTempFile(dir, payload, contentType)
		if err != nil {
			cleanup()
			return nil, nil, err
		}
		paths = append(paths, path)
	}
	return paths, cleanup, nil
}

func writeReferenceTempFile(dir string, payload []byte, contentType string) (string, error) {
	exts, _ := mime.ExtensionsByType(contentType)
	ext := ".png"
	if len(exts) > 0 {
		ext = exts[0]
	}
	path := filepath.Join(dir, uuid.New().String()+ext)
	if err := os.WriteFile(path, payload, 0644); err != nil {
		return "", err
	}
	return path, nil
}

func (e *FallbackEngine) readReferencePayload(ctx context.Context, ref string) (string, []byte, error) {
	if contentType, payload, ok := parseDataURLPayload(ref); ok {
		return contentType, payload, nil
	}
	if e.storage != nil {
		if strings.HasPrefix(ref, "/api/media/generated/") || strings.HasPrefix(ref, e.storage.baseURL) {
			payload, err := e.storage.ReadServedURL(ref)
			if err != nil {
				return "", nil, err
			}
			return http.DetectContentType(payload), payload, nil
		}
	}
	return e.downloadAsset(ctx, ref)
}

func parseDataURLPayload(raw string) (string, []byte, bool) {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "data:") {
		return "", nil, false
	}
	comma := strings.Index(trimmed, ",")
	if comma < 0 {
		return "", nil, false
	}
	meta := trimmed[5:comma]
	body := trimmed[comma+1:]
	if !strings.HasSuffix(meta, ";base64") {
		return "", nil, false
	}
	contentType := strings.TrimSuffix(meta, ";base64")
	if contentType == "" {
		contentType = "image/png"
	}
	payload, err := base64.StdEncoding.DecodeString(body)
	if err != nil {
		return "", nil, false
	}
	return contentType, payload, true
}

func (e *FallbackEngine) updateInternalTask(taskID string, mutate func(task *MediaTask)) {
	value, ok := e.tasks.Load(taskID)
	if !ok {
		return
	}
	task, ok := value.(*MediaTask)
	if !ok || task == nil {
		return
	}
	next := cloneMediaTask(task)
	mutate(next)
	e.tasks.Store(taskID, next)
}

func (e *FallbackEngine) failInternalTask(taskID, errMsg string) {
	e.updateInternalTask(taskID, func(task *MediaTask) {
		task.Status = TaskStatusFailed
		task.Error = errMsg
		task.Progress = 1
		now := timeutil.NowTime()
		task.CompletedAt = &now
	})
}

func (e *FallbackEngine) downloadAsset(ctx context.Context, rawURL string) (string, []byte, error) {
	if contentType, payload, ok := parseDataURLPayload(rawURL); ok {
		return contentType, payload, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; ZimaOS-Blue/1.0)")
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	reader := io.LimitReader(resp.Body, fallbackMaxImageBytes)
	payload, err := io.ReadAll(reader)
	if err != nil {
		return "", nil, err
	}
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = http.DetectContentType(payload)
	}
	if idx := strings.Index(contentType, ";"); idx >= 0 {
		contentType = strings.TrimSpace(contentType[:idx])
	}
	return contentType, payload, nil
}

func (e *FallbackEngine) fetchHTML(ctx context.Context, rawURL string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; ZimaOS-Blue/1.0)")
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("status %d", resp.StatusCode)
	}
	payload, err := io.ReadAll(io.LimitReader(resp.Body, fallbackMaxHTMLBytes))
	if err != nil {
		return "", "", err
	}
	return string(payload), resp.Request.URL.String(), nil
}

func extractOGImage(rawHTML string, base string) string {
	doc, err := html.Parse(strings.NewReader(rawHTML))
	if err != nil {
		return ""
	}
	var image string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if image != "" || n == nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "meta" {
			var property, name, content string
			for _, attr := range n.Attr {
				switch strings.ToLower(attr.Key) {
				case "property":
					property = strings.ToLower(strings.TrimSpace(attr.Val))
				case "name":
					name = strings.ToLower(strings.TrimSpace(attr.Val))
				case "content":
					content = strings.TrimSpace(attr.Val)
				}
			}
			if content != "" && (property == "og:image" || name == "twitter:image") {
				image = resolveRelativeURL(base, content)
				return
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return image
}

type fallbackSceneSearcher struct {
	engine *FallbackEngine
}

func (s fallbackSceneSearcher) Search(ctx context.Context, query string, maxResults int) ([]scenecompose.SearchResult, error) {
	if s.engine == nil || s.engine.searcher == nil {
		return nil, fmt.Errorf("searcher unavailable")
	}
	results, err := s.engine.searcher.Search(ctx, query, maxResults, s.engine.config.SearchProviderChain)
	if err != nil {
		return nil, err
	}
	normalized := make([]scenecompose.SearchResult, 0, len(results))
	for _, item := range results {
		normalized = append(normalized, scenecompose.SearchResult{
			Title:       item.Title,
			URL:         item.URL,
			Description: item.Description,
		})
	}
	return normalized, nil
}

type fallbackSceneResolver struct {
	engine *FallbackEngine
}

func (r fallbackSceneResolver) Resolve(ctx context.Context, result scenecompose.SearchResult) (*scenecompose.ResolvedImage, error) {
	if r.engine == nil {
		return nil, fmt.Errorf("resolver unavailable")
	}
	sourceURL, payload, contentType, err := r.engine.resolveSearchImage(ctx, result.URL)
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	return &scenecompose.ResolvedImage{
		Title:       result.Title,
		PageURL:     result.URL,
		SourceURL:   sourceURL,
		Description: result.Description,
		ContentType: contentType,
		Image:       img,
		Width:       img.Bounds().Dx(),
		Height:      img.Bounds().Dy(),
		HasAlpha:    fallbackImageHasAlpha(img),
	}, nil
}

func fallbackImageHasAlpha(img image.Image) bool {
	if img == nil {
		return false
	}
	bounds := img.Bounds()
	stepX := 1
	stepY := 1
	if bounds.Dx() > 24 {
		stepX = bounds.Dx() / 24
	}
	if bounds.Dy() > 24 {
		stepY = bounds.Dy() / 24
	}
	for y := bounds.Min.Y; y < bounds.Max.Y; y += stepY {
		for x := bounds.Min.X; x < bounds.Max.X; x += stepX {
			_, _, _, alpha := img.At(x, y).RGBA()
			if alpha < 0xffff {
				return true
			}
		}
	}
	return false
}

func assetRefsToSourceURLs(refs []scenecompose.AssetRef) []string {
	seen := make(map[string]struct{}, len(refs))
	urls := make([]string, 0, len(refs))
	for _, ref := range refs {
		u := strings.TrimSpace(ref.SourceURL)
		if u == "" {
			continue
		}
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		urls = append(urls, u)
	}
	return urls
}

func resolveRelativeURL(baseURL string, raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err == nil && parsed.IsAbs() {
		return parsed.String()
	}
	base, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return trimmed
	}
	ref, err := url.Parse(trimmed)
	if err != nil {
		return trimmed
	}
	return base.ResolveReference(ref).String()
}

func mediaTypeForFallbackModel(modelID string) MediaType {
	switch modelID {
	case FallbackModelSpaceT2V, FallbackModelSpaceI2V, FallbackModelSpaceKF2V:
		return MediaTypeVideo
	default:
		return MediaTypeImage
	}
}

func inferCategoryFromFallbackModel(modelID string) MediaCategory {
	switch modelID {
	case FallbackModelWebCanvasT2I, FallbackModelSpaceT2I:
		return CategoryT2I
	case FallbackModelSpaceI2I:
		return CategoryI2I
	case FallbackModelSpaceT2V:
		return CategoryT2V
	case FallbackModelSpaceI2V:
		return CategoryI2V
	case FallbackModelSpaceKF2V:
		return CategoryKF2V
	default:
		return CategoryNone
	}
}

func mediaTypeForResultKind(kind string, fallbackModel string) MediaType {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "video":
		return MediaTypeVideo
	case "image":
		return MediaTypeImage
	default:
		return mediaTypeForFallbackModel(fallbackModel)
	}
}

func looksLikeServableMediaURL(raw string) bool {
	lower := strings.ToLower(strings.TrimSpace(raw))
	return strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") || strings.HasSuffix(lower, ".webp") || strings.HasSuffix(lower, ".mp4") || strings.HasPrefix(lower, "data:")
}

func cloneMediaRequest(req *MediaRequest) *MediaRequest {
	if req == nil {
		return nil
	}
	cp := *req
	if len(req.ReferenceImage) > 0 {
		cp.ReferenceImage = append([]byte(nil), req.ReferenceImage...)
	}
	if len(req.ReferenceURLs) > 0 {
		cp.ReferenceURLs = append([]string(nil), req.ReferenceURLs...)
	}
	if req.Extra != nil {
		cp.Extra = make(map[string]any, len(req.Extra))
		for key, value := range req.Extra {
			cp.Extra[key] = value
		}
	}
	return &cp
}

func cloneMediaTask(task *MediaTask) *MediaTask {
	if task == nil {
		return nil
	}
	cp := *task
	cp.Request = cloneMediaRequest(task.Request)
	if task.Response != nil {
		resp := *task.Response
		resp.Data = append([]MediaResult(nil), task.Response.Data...)
		cp.Response = &resp
	}
	cp.FallbackInfo = cloneFallbackInfo(task.FallbackInfo)
	return &cp
}

func cloneFallbackInfo(info *MediaFallbackInfo) *MediaFallbackInfo {
	if info == nil {
		return nil
	}
	cp := *info
	if len(info.SourceURLs) > 0 {
		cp.SourceURLs = append([]string(nil), info.SourceURLs...)
	}
	return &cp
}

func inferCategoryFromRequest(req *MediaRequest) MediaCategory {
	if req == nil {
		return CategoryNone
	}
	if req.Type == MediaTypeVideo {
		switch len(req.ReferenceURLs) {
		case 0:
			if strings.TrimSpace(req.ReferenceURL) != "" || len(req.ReferenceImage) > 0 {
				return CategoryI2V
			}
			return CategoryT2V
		case 1:
			return CategoryI2V
		default:
			return CategoryKF2V
		}
	}
	if len(req.ReferenceURLs) > 0 || strings.TrimSpace(req.ReferenceURL) != "" || len(req.ReferenceImage) > 0 {
		return CategoryI2I
	}
	return CategoryT2I
}

func isComplexFallbackPrompt(req *MediaRequest, category MediaCategory, threshold int) bool {
	switch category {
	case CategoryT2V, CategoryI2V, CategoryKF2V:
		return true
	case CategoryI2I:
		return true
	case CategoryT2I:
		prompt := ""
		if req != nil {
			prompt = strings.TrimSpace(req.Prompt)
		}
		if utf8.RuneCountInString(prompt) > threshold {
			return true
		}
		lower := strings.ToLower(prompt)
		for _, keyword := range fallbackComplexKeywords {
			if strings.Contains(lower, strings.ToLower(keyword)) {
				return true
			}
		}
	}
	return false
}

func extractPosterKeywords(prompt string) []string {
	tokens := strings.FieldsFunc(strings.ToLower(prompt), func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune(",.;:!?/|\\-_()[]{}<>\"'，。！？；：、", r)
	})
	seen := map[string]struct{}{}
	keywords := make([]string, 0, 8)
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		if _, blocked := fallbackPosterStopWords[token]; blocked {
			continue
		}
		if utf8.RuneCountInString(token) < 2 {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		keywords = append(keywords, token)
		if len(keywords) == 8 {
			break
		}
	}
	if len(keywords) == 0 {
		keywords = []string{"fallback", "poster"}
	}
	sort.Strings(keywords)
	return keywords
}

func bytesToDataURL(payload []byte, contentType string) string {
	if contentType == "" {
		contentType = http.DetectContentType(payload)
	}
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(payload)
}

func fallbackDisplayName(strategy string) string {
	switch strategy {
	case FallbackStrategyWebCanvas:
		return "Web Search + Canvas"
	case FallbackStrategyPublicSpace:
		return "Public Creative Space"
	default:
		return "Fallback"
	}
}

func fallbackDisclosure(locale, strategy string) string {
	if isChineseLocale(locale) {
		switch strategy {
		case FallbackStrategyWebCanvas:
			return "未检测到可用媒体生成 API Key，已改用网页搜索候选图和本地 canvas 截图生成结果。"
		case FallbackStrategyPublicSpace:
			return "未检测到可用媒体生成 API Key，已改用公开创意空间进行实验性、尽力而为的生成。"
		default:
			return "未检测到可用媒体生成 API Key，已使用内建降级能力。"
		}
	}
	switch strategy {
	case FallbackStrategyWebCanvas:
		return "No configured media API key was available, so the result was generated from web search references and a local canvas render."
	case FallbackStrategyPublicSpace:
		return "No configured media API key was available, so the request used a public creative space on a best-effort experimental basis."
	default:
		return "No configured media API key was available, so a built-in fallback path was used."
	}
}

type fallbackRenderStore struct {
	mu    sync.RWMutex
	pages map[string]fallbackRenderPage
}

type fallbackRenderPage struct {
	HTML      string
	ExpiresAt time.Time
}

func newFallbackRenderStore() *fallbackRenderStore {
	return &fallbackRenderStore{pages: make(map[string]fallbackRenderPage)}
}

func (s *fallbackRenderStore) Put(doc string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	token := uuid.New().String()
	s.pages[token] = fallbackRenderPage{
		HTML:      doc,
		ExpiresAt: timeutil.NowTime().Add(10 * time.Minute),
	}
	return token
}

func (s *fallbackRenderStore) Get(token string) (string, bool) {
	s.mu.RLock()
	page, ok := s.pages[token]
	s.mu.RUnlock()
	if !ok {
		return "", false
	}
	if timeutil.NowTime().After(page.ExpiresAt) {
		s.Delete(token)
		return "", false
	}
	return page.HTML, true
}

func (s *fallbackRenderStore) Delete(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pages, token)
}
