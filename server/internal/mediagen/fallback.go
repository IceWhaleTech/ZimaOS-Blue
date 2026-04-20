package mediagen

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	stdhtml "html"
	"image"
	"image/color"
	stdDraw "image/draw"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"log"
	"math/rand/v2"
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
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/slidespec"
	basetask "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/task"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tts"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/videogen"
	"github.com/google/uuid"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
	"golang.org/x/net/html"
)

const (
	fallbackProviderName = "fallback"

	FallbackStrategyWebCanvas   = "web_canvas"
	FallbackStrategyPublicSpace = "public_space"
	FallbackStrategyNativeVideo = "native_timeline"

	FallbackModelWebCanvasT2I = "fallback-web-canvas-t2i"
	FallbackModelSpaceT2I     = "fallback-space-t2i"
	FallbackModelSpaceI2I     = "fallback-space-i2i"
	FallbackModelSpaceT2V     = "fallback-space-t2v"
	FallbackModelSpaceI2V     = "fallback-space-i2v"
	FallbackModelSpaceKF2V    = "fallback-space-kf2v"

	fallbackRenderReadySelector = "#fallback-render-ready.ready"
	fallbackMaxHTMLBytes        = 1 << 20
	fallbackMaxImageBytes       = 10 << 20

	fallbackSearchPlannerTimeout       = 30 * time.Second
	fallbackSearchStageTimeout         = 30 * time.Second
	fallbackSearchQueryTimeout         = 12 * time.Second
	fallbackSourceQueryTimeout         = 4 * time.Second
	fallbackSceneComposeTimeout        = 5 * time.Minute
	fallbackImageResolveTimeout        = 3 * time.Second
	fallbackBrowserImageStageTimeout   = 180 * time.Second
	fallbackBrowserSearchTimeout       = 60 * time.Second
	fallbackBrowserCaptureTimeout      = 6 * time.Second
	fallbackBrowserJudgeTimeout        = 8 * time.Second
	fallbackBrowserJudgeMinScore       = 45.0
	fallbackBrowserJudgeMaxImageDim    = 2048
	fallbackBrowserJudgeMaxBase64Len   = 4 << 20
	fallbackCraiyonSearchTimeout       = 18 * time.Second
	fallbackCraiyonPreviewResultLimit  = 10
	fallbackCraiyonJudgeViewportHeight = 960
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

type localizedTitleAffix struct {
	prefix string
	suffix string
}

// Covers the repository's 27 supported locales plus a few punctuation variants
// observed around Google Images result alt text boilerplate.
var fallbackGoogleImageResultTitleAffixes = []localizedTitleAffix{
	{prefix: "Resultat d'imatge per a "},
	{prefix: "Výsledek obrázku pro "},
	{prefix: "Billedresultat for "},
	{prefix: "Bildergebnis für "},
	{prefix: "Αποτέλεσμα εικόνας για "},
	{prefix: "Image result for "},
	{prefix: "Resultado de imagen para "},
	{prefix: "Résultat d'image pour "},
	{prefix: "Résultat de l'image pour "},
	{prefix: "Toradh íomhá do "},
	{prefix: "Rezultat slike za "},
	{prefix: "Képeredmény az ", suffix: " kifejezésre"},
	{prefix: "Risultato immagine per "},
	{suffix: "の画像結果"},
	{prefix: "「", suffix: "」の画像結果"},
	{suffix: "에 대한 이미지 검색결과"},
	{suffix: "-നുള്ള ചിത്ര ഫലം"},
	{prefix: "Bilderesultat for "},
	{prefix: "Afbeeldingsresultaat voor "},
	{prefix: "Wynik obrazu dla "},
	{prefix: "Resultado de imagem para "},
	{prefix: "Rezultat imagine pentru "},
	{prefix: "Результат изображения для "},
	{prefix: "Výsledok obrázka pre "},
	{prefix: "Bildresultat för "},
	{suffix: " 的图像结果"},
	{suffix: " 的圖像結果"},
	{suffix: "的图像结果"},
	{suffix: "的圖像結果"},
}

var fallbackPosterStopWords = map[string]struct{}{
	"a": {}, "an": {}, "the": {}, "and": {}, "for": {}, "with": {}, "from": {}, "into": {}, "that": {}, "this": {},
	"your": {}, "you": {}, "show": {}, "make": {}, "image": {}, "photo": {}, "poster": {}, "video": {}, "draw": {},
	"生成": {}, "图片": {}, "海报": {}, "视频": {}, "一个": {}, "一张": {}, "帮我": {},
}

var fallbackPhotoBlockedHosts = []string{
	"zhidao.baidu.com",
	"baike.baidu.com",
	"tieba.baidu.com",
	"zhihu.com",
	"www.zhihu.com",
}

var fallbackPhotoSyntheticHosts = []string{
	"craiyon.com",
}

var fallbackPhotoPromptTermStopWords = map[string]struct{}{
	"a": {}, "an": {}, "the": {}, "and": {}, "or": {}, "for": {}, "with": {}, "into": {}, "from": {},
	"photo": {}, "photograph": {}, "picture": {}, "image": {}, "images": {}, "real": {}, "realistic": {},
	"background": {}, "landscape": {}, "generate": {}, "create": {}, "make": {}, "draw": {}, "show": {},
	"help": {}, "please": {}, "生成": {}, "创建": {}, "制作": {}, "图片": {}, "照片": {}, "写真": {},
	"写实": {}, "背景": {}, "风景": {}, "帮我": {}, "请": {},
}

var fallbackLowValuePhotoStrongMarkers = []string{
	"百度知道",
	"zhidao",
	"问答",
	"dictionary",
	"definition",
	"pronunciation",
	"radical",
	"stroke order",
	"字典",
	"词典",
	"拼音",
	"部首",
	"笔顺",
	"释义",
	"汉典",
	"新华字典",
	"组词",
	"怎么读",
	"字的意思",
	"字的解释",
}

var fallbackLowValuePhotoForumMarkers = []string{
	"论坛",
	"bbs",
	"forum",
	"forumdisplay",
	"thread",
	"帖子",
	"攻略",
	"秘籍",
	"汉化",
	"mod",
	"贴吧",
}

var fallbackPhotoIntentMarkers = []string{
	"photo",
	"photograph",
	"picture",
	"image",
	"gallery",
	"wallpaper",
	"stock photo",
	"照片",
	"图片",
	"写真",
	"高清",
	"摄影",
	"壁纸",
	"图库",
}

var fallbackPhotoPromptNoiseReplacer = strings.NewReplacer(
	"写实照片", " ",
	"真实照片", " ",
	"照片", " ",
	"写真", " ",
	"图片", " ",
	"图像", " ",
	"写实", " ",
	"真实", " ",
	"背景", " ",
	"风景", " ",
	"壁纸", " ",
	"高清", " ",
	"素材", " ",
	"生成", " ",
	"创建", " ",
	"制作", " ",
	"画", " ",
	"做", " ",
	"帮我", " ",
	"请帮我", " ",
	"请", " ",
	"一张", " ",
	"一幅", " ",
	"一个", " ",
	"一只", " ",
	"一条", " ",
	"在", " ",
	"里", " ",
	"的", " ",
	"和", " ",
	"与", " ",
)

var fallbackBlockedSourceHosts = []string{
	"commons.wikimedia.org",
	"upload.wikimedia.org",
}

var fallbackBrowserRandomIndex = func(size int) int {
	if size <= 1 {
		return 0
	}
	return rand.IntN(size)
}

var fallbackPromptPrefixes = []string{
	"please generate an image of",
	"please generate a photo of",
	"please create an image of",
	"please create a photo of",
	"help me generate an image of",
	"help me create an image of",
	"generate an image of",
	"generate a photo of",
	"create an image of",
	"create a photo of",
	"请帮我生成一张",
	"请帮我生成一个",
	"请帮我生成一幅",
	"请帮我画一张",
	"请帮我画一个",
	"请帮我做一张",
	"帮我生成一张",
	"帮我生成一个",
	"帮我生成一幅",
	"帮我画一张",
	"帮我画一个",
	"帮我做一张",
	"帮我做一个",
	"给我生成一张",
	"给我生成一个",
	"给我画一张",
	"给我做一张",
	"请生成一张",
	"请生成一个",
	"请画一张",
	"请做一张",
	"生成一张",
	"生成一个",
	"画一张",
	"画一个",
	"做一张",
	"做一个",
	"请帮我",
	"帮我",
	"请你",
	"请",
	"给我",
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
	NativeVideo         FallbackNativeVideoConfig
	PublicSpaces        []FallbackPublicSpacePreset
}

// FallbackNativeVideoConfig controls the native macOS video fallback path.
type FallbackNativeVideoConfig struct {
	Enabled            bool
	FPS                int
	DefaultDurationSec int
	MaxDurationSec     int
	PollInterval       time.Duration
	StallTimeout       time.Duration
	MaxRuntime         time.Duration
	HelperPath         string
	AudioMode          string
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
	StallTimeout            time.Duration
	MaxRuntime              time.Duration
	Timeout                 time.Duration // legacy alias retained for compatibility
}

// FallbackResultSelector extracts a finished result URL from a live page.
type FallbackResultSelector struct {
	Selectors []string
	Attribute string
	Kind      string
}

// FallbackSearchResult is the normalized result shape used by the fallback engine.
type FallbackSearchResult struct {
	Title           string
	URL             string
	Description     string
	ImageURL        string
	ThumbnailURL    string
	Provider        string
	Creator         string
	License         string
	LicenseURL      string
	SourceNote      string
	VerifiedLicense bool
	IntentSelected  bool
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

type fallbackBrowserScraper interface {
	Scrape(ctx context.Context, req *browser.ScrapeRequest) (*browser.ScrapeResponse, error)
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

	renderStore  *fallbackRenderStore
	httpClient   *http.Client
	sourceClient *http.Client
	tasks        sync.Map

	sceneComposer    *scenecompose.Engine
	scenePlannerLLM  scenecompose.LLMCaller
	u2netpModel      *scenecompose.U2NetPModelManager
	visionBridge     tools.VLMBridge
	ttsService       tts.Service
	nativeVideo      videogen.Generator
	downloadCardKeys sync.Map
}

// NewFallbackEngine creates a new no-key fallback engine.
func NewFallbackEngine(cfg FallbackConfig, storage *MediaStorage, searcher FallbackSearcher, browserSvc func() FallbackBrowserService, locale string) *FallbackEngine {
	cfg = normalizeFallbackConfig(cfg)
	configureSlideFontRuntimeDataDir(cfg.DataDir)
	engine := &FallbackEngine{
		config:       cfg,
		storage:      storage,
		searcher:     searcher,
		browser:      browserSvc,
		locale:       locale,
		renderStore:  newFallbackRenderStore(),
		httpClient:   network.NewPooledHTTPClient(5 * time.Minute),
		sourceClient: network.NewPooledHTTPClient(5 * time.Minute),
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

// SetVisionBridge injects an optional VLM used to pick the best browser image result by intent match.
func (e *FallbackEngine) SetVisionBridge(bridge tools.VLMBridge) {
	if e == nil {
		return
	}
	e.visionBridge = bridge
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
	cfg.SearchProviderChain = sanitizeFallbackImageSearchProviders(cfg.SearchProviderChain)
	if cfg.NativeVideo.FPS <= 0 {
		cfg.NativeVideo.FPS = 30
	}
	if cfg.NativeVideo.DefaultDurationSec <= 0 {
		cfg.NativeVideo.DefaultDurationSec = 5
	}
	if cfg.NativeVideo.MaxDurationSec <= 0 {
		cfg.NativeVideo.MaxDurationSec = 12
	}
	if cfg.NativeVideo.PollInterval <= 0 {
		cfg.NativeVideo.PollInterval = 2 * time.Second
	}
	if cfg.NativeVideo.StallTimeout <= 0 {
		cfg.NativeVideo.StallTimeout = 15 * time.Minute
	}
	if cfg.NativeVideo.MaxRuntime <= 0 {
		cfg.NativeVideo.MaxRuntime = 6 * time.Hour
	}
	if strings.TrimSpace(cfg.NativeVideo.AudioMode) == "" {
		cfg.NativeVideo.AudioMode = "auto"
	}
	presets := make([]FallbackPublicSpacePreset, 0, len(cfg.PublicSpaces))
	for _, preset := range cfg.PublicSpaces {
		if preset.PollInterval <= 0 {
			preset.PollInterval = 4 * time.Second
		}
		if preset.StallTimeout <= 0 {
			preset.StallTimeout = preset.Timeout
		}
		if preset.StallTimeout <= 0 {
			preset.StallTimeout = 15 * time.Minute
		}
		if preset.MaxRuntime <= 0 {
			preset.MaxRuntime = 6 * time.Hour
		}
		if preset.Timeout <= 0 {
			preset.Timeout = preset.StallTimeout
		}
		presets = append(presets, preset)
	}
	cfg.PublicSpaces = presets
	return cfg
}

func cloneProviderChain(providers []string) []string {
	if len(providers) == 0 {
		return nil
	}
	cloned := make([]string, 0, len(providers))
	for _, provider := range providers {
		provider = strings.ToLower(strings.TrimSpace(provider))
		if provider == "" {
			continue
		}
		cloned = append(cloned, provider)
	}
	return cloned
}

func providerChainContains(providers []string, target string) bool {
	target = strings.ToLower(strings.TrimSpace(target))
	if target == "" {
		return false
	}
	for _, provider := range providers {
		if strings.EqualFold(strings.TrimSpace(provider), target) {
			return true
		}
	}
	return false
}

func providerChainEqual(left, right []string) bool {
	left = cloneProviderChain(left)
	right = cloneProviderChain(right)
	if len(left) != len(right) {
		return false
	}
	for idx := range left {
		if left[idx] != right[idx] {
			return false
		}
	}
	return true
}

func truncateFallbackLogValue(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if maxRunes <= 0 {
		maxRunes = 120
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes]) + "..."
}

func summarizeFallbackResultsForLog(results []FallbackSearchResult, limit int) string {
	if len(results) == 0 {
		return "[]"
	}
	if limit <= 0 {
		limit = 3
	}
	items := make([]string, 0, min(limit, len(results)))
	for idx, item := range results {
		if idx >= limit {
			break
		}
		items = append(items, fmt.Sprintf("%q -> %q",
			truncateFallbackLogValue(item.Title, 48),
			truncateFallbackLogValue(item.URL, 120),
		))
	}
	if len(results) > limit {
		items = append(items, fmt.Sprintf("...+%d more", len(results)-limit))
	}
	return "[" + strings.Join(items, "; ") + "]"
}

func summarizeFallbackCandidatesForLog(candidates []fallbackImageCandidate, limit int) string {
	if len(candidates) == 0 {
		return "[]"
	}
	if limit <= 0 {
		limit = 3
	}
	items := make([]string, 0, min(limit, len(candidates)))
	for idx, candidate := range candidates {
		if idx >= limit {
			break
		}
		items = append(items, fmt.Sprintf("%q -> %q",
			truncateFallbackLogValue(candidate.Title, 48),
			truncateFallbackLogValue(candidate.Source, 120),
		))
	}
	if len(candidates) > limit {
		items = append(items, fmt.Sprintf("...+%d more", len(candidates)-limit))
	}
	return "[" + strings.Join(items, "; ") + "]"
}

func filterFallbackResults(prompt string, results []FallbackSearchResult) []FallbackSearchResult {
	if len(results) == 0 {
		return nil
	}
	filtered := make([]FallbackSearchResult, 0, len(results))
	for _, item := range results {
		if fallbackResultUsesBlockedSource(item) {
			continue
		}
		if item.IntentSelected {
			filtered = append(filtered, item)
			continue
		}
		if !looksLikePhotoPromptText(prompt) {
			filtered = append(filtered, item)
			continue
		}
		if isLowValuePhotoSearchResult(prompt, item) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func fallbackResultUsesBlockedSource(item FallbackSearchResult) bool {
	provider := strings.ToLower(strings.TrimSpace(item.Provider))
	if strings.Contains(provider, "wikimedia") {
		return true
	}
	return fallbackURLUsesBlockedSource(item.URL) || fallbackURLUsesBlockedSource(item.ImageURL)
}

func fallbackURLUsesBlockedSource(rawURL string) bool {
	return fallbackURLMatchesHosts(rawURL, fallbackBlockedSourceHosts)
}

func fallbackURLMatchesHosts(rawURL string, hosts []string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	host = strings.TrimPrefix(host, "www.")
	if host == "" {
		return false
	}
	for _, blocked := range hosts {
		blocked = strings.ToLower(strings.TrimSpace(blocked))
		blocked = strings.TrimPrefix(blocked, "www.")
		if host == blocked || strings.HasSuffix(host, "."+blocked) {
			return true
		}
	}
	return false
}

func isLowValuePhotoSearchResult(prompt string, item FallbackSearchResult) bool {
	rawURL := strings.TrimSpace(item.URL)
	if rawURL == "" && strings.TrimSpace(item.ImageURL) == "" {
		return true
	}
	if fallbackResultUsesSyntheticSource(item) {
		return true
	}
	parsed, err := url.Parse(rawURL)
	if err == nil {
		host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
		for _, blocked := range fallbackPhotoBlockedHosts {
			blocked = strings.ToLower(strings.TrimSpace(blocked))
			if host == blocked || strings.HasSuffix(host, "."+blocked) {
				return true
			}
		}
	}
	resultText := strings.ToLower(strings.Join([]string{
		strings.TrimSpace(item.Title),
		strings.TrimSpace(item.Description),
		rawURL,
		strings.TrimSpace(item.ImageURL),
	}, " "))
	if fallbackContainsAnyMarker(resultText, fallbackLowValuePhotoStrongMarkers) {
		return true
	}
	matchCount, cueCount := fallbackPhotoPromptMatchScore(prompt, item)
	switch {
	case cueCount >= 2 && matchCount < 2:
		return true
	case cueCount == 1 && matchCount == 0:
		return true
	case cueCount == 1 && matchCount == 1 && fallbackContainsAnyMarker(resultText, fallbackLowValuePhotoForumMarkers) && !fallbackPhotoResultHasImageIntent(item):
		return true
	}
	return false
}

func fallbackResultUsesSyntheticSource(item FallbackSearchResult) bool {
	provider := strings.ToLower(strings.TrimSpace(item.Provider))
	if strings.Contains(provider, "craiyon") {
		return true
	}
	return fallbackURLMatchesHosts(item.URL, fallbackPhotoSyntheticHosts) || fallbackURLMatchesHosts(item.ImageURL, fallbackPhotoSyntheticHosts)
}

func fallbackPhotoPromptMatchScore(prompt string, item FallbackSearchResult) (int, int) {
	rawTerms, englishTerms := fallbackPhotoPromptTermSets(prompt)
	cueCount := len(rawTerms)
	if cueCount == 0 {
		cueCount = len(englishTerms)
	}
	if cueCount == 0 {
		return 0, 0
	}
	rawText := strings.ToLower(strings.Join([]string{
		strings.TrimSpace(item.Title),
		strings.TrimSpace(item.Description),
		strings.TrimSpace(item.URL),
		strings.TrimSpace(item.ImageURL),
	}, " "))
	englishizedText := strings.ToLower(fallbackEnglishizeSearchQuery(rawText))
	rawMatches := 0
	for _, term := range rawTerms {
		if strings.Contains(rawText, term) {
			rawMatches++
		}
	}
	englishMatches := 0
	for _, term := range englishTerms {
		if strings.Contains(rawText, term) || strings.Contains(englishizedText, term) {
			englishMatches++
		}
	}
	return max(rawMatches, englishMatches), cueCount
}

func fallbackPhotoPromptTermSets(prompt string) ([]string, []string) {
	semantic := normalizeMediaPrompt(prompt)
	if semantic == "" {
		semantic = strings.TrimSpace(prompt)
	}
	rawTerms := fallbackPhotoPromptTerms(fallbackPhotoPromptNoiseReplacer.Replace(semantic))
	englishTerms := fallbackPhotoPromptTerms(fallbackEnglishizeSearchQuery(semantic))
	return rawTerms, englishTerms
}

func fallbackPhotoPromptTerms(text string) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	parts := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune(",.;:!?/|\\-_()[]{}<>\"'，。！？；：、", r)
	})
	terms := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, blocked := fallbackPhotoPromptTermStopWords[part]; blocked {
			continue
		}
		if utf8.RuneCountInString(part) < 2 {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		terms = append(terms, part)
	}
	return terms
}

func fallbackContainsAnyMarker(text string, markers []string) bool {
	for _, marker := range markers {
		marker = strings.ToLower(strings.TrimSpace(marker))
		if marker != "" && strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func fallbackPhotoResultHasImageIntent(item FallbackSearchResult) bool {
	if strings.TrimSpace(item.ImageURL) != "" || strings.TrimSpace(item.ThumbnailURL) != "" {
		return true
	}
	resultText := strings.ToLower(strings.Join([]string{
		strings.TrimSpace(item.Title),
		strings.TrimSpace(item.Description),
		strings.TrimSpace(item.URL),
	}, " "))
	return fallbackContainsAnyMarker(resultText, fallbackPhotoIntentMarkers)
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
	models := []MediaModelInfo{
		{ID: FallbackModelWebCanvasT2I, Name: "Fallback Web Canvas", Type: MediaTypeImage, Category: CategoryT2I, Provider: fallbackProviderName, IsFallback: true, FallbackStrategy: FallbackStrategyWebCanvas},
		{ID: FallbackModelSpaceT2I, Name: "Fallback Public Space (Image)", Type: MediaTypeImage, Category: CategoryT2I, Provider: fallbackProviderName, IsFallback: true, FallbackStrategy: FallbackStrategyPublicSpace},
		{ID: FallbackModelSpaceI2I, Name: "Fallback Public Space (Edit)", Type: MediaTypeImage, Category: CategoryI2I, Provider: fallbackProviderName, IsFallback: true, FallbackStrategy: FallbackStrategyPublicSpace},
		{ID: FallbackModelSpaceT2V, Name: "Fallback Public Space (Video)", Type: MediaTypeVideo, Category: CategoryT2V, Provider: fallbackProviderName, IsFallback: true, FallbackStrategy: FallbackStrategyPublicSpace},
		{ID: FallbackModelSpaceI2V, Name: "Fallback Public Space (Image to Video)", Type: MediaTypeVideo, Category: CategoryI2V, Provider: fallbackProviderName, IsFallback: true, FallbackStrategy: FallbackStrategyPublicSpace},
		{ID: FallbackModelSpaceKF2V, Name: "Fallback Public Space (Keyframe Video)", Type: MediaTypeVideo, Category: CategoryKF2V, Provider: fallbackProviderName, IsFallback: true, FallbackStrategy: FallbackStrategyPublicSpace},
	}
	if e.nativeVideoAvailable() {
		models = append(models, e.nativeVideoModelInfos()...)
	}
	return models
}

// IsFallbackModel reports whether the given model ID belongs to the built-in fallback catalog.
func (e *FallbackEngine) IsFallbackModel(modelID string) bool {
	switch strings.TrimSpace(modelID) {
	case FallbackModelWebCanvasT2I,
		FallbackModelSpaceT2I,
		FallbackModelSpaceI2I,
		FallbackModelSpaceT2V,
		FallbackModelSpaceI2V,
		FallbackModelSpaceKF2V,
		FallbackModelNativeT2V,
		FallbackModelNativeI2V,
		FallbackModelNativeKF2V:
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
		if e.nativeVideoAvailable() {
			return FallbackModelNativeT2V
		}
		return FallbackModelSpaceT2V
	case CategoryI2V:
		if e.nativeVideoAvailable() {
			return FallbackModelNativeI2V
		}
		return FallbackModelSpaceI2V
	case CategoryKF2V:
		if e.nativeVideoAvailable() {
			return FallbackModelNativeKF2V
		}
		return FallbackModelSpaceKF2V
	default:
		return ""
	}
}

// ModelForRequest resolves the fallback pseudo-model for the request/category pair.
func (e *FallbackEngine) ModelForRequest(req *MediaRequest, category MediaCategory) string {
	if req != nil && e.IsFallbackModel(req.Model) {
		if isNativeFallbackVideoModel(req.Model) && !e.nativeVideoAvailable() {
			return publicSpaceModelForCategory(category)
		}
		return req.Model
	}
	switch category {
	case CategoryT2I:
		if isSlideFallbackRequest(req) {
			return FallbackModelWebCanvasT2I
		}
		if isComplexFallbackPrompt(req, category, e.config.ComplexPromptChars) {
			return FallbackModelSpaceT2I
		}
		return FallbackModelWebCanvasT2I
	case CategoryI2I:
		return FallbackModelSpaceI2I
	case CategoryT2V:
		if e.nativeVideoAvailable() {
			return FallbackModelNativeT2V
		}
		return FallbackModelSpaceT2V
	case CategoryI2V:
		if e.nativeVideoAvailable() {
			return FallbackModelNativeI2V
		}
		return FallbackModelSpaceI2V
	case CategoryKF2V:
		if e.nativeVideoAvailable() {
			return FallbackModelNativeKF2V
		}
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
	} else if isNativeFallbackVideoModel(modelID) {
		strategy = FallbackStrategyNativeVideo
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
	case FallbackModelNativeT2V, FallbackModelNativeI2V, FallbackModelNativeKF2V:
		return e.generateNativeVideo(ctx, req, category, modelID)
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
	if isSlideFallbackRequest(req) {
		return e.generateSlideWebCanvas(ctx, req)
	}
	brief := slideBriefFromMediaRequest(req)
	preferSearchReference := looksLikePhotoPrompt(req)
	searchProviders := e.referenceSearchProviderChain(strings.TrimSpace(req.Prompt), preferSearchReference)
	log.Printf("[mediagen] fallback web_canvas start prompt=%q photo_prompt=%t providers=%v", truncateFallbackLogValue(reqPrompt(req), 160), preferSearchReference, searchProviders)
	results, candidates, sourceURLs, sources := e.searchReferenceAssets(ctx, strings.TrimSpace(req.Prompt), searchProviders)
	if preferSearchReference && len(results) == 0 && !providerChainEqual(searchProviders, e.config.SearchProviderChain) {
		log.Printf("[mediagen] fallback web_canvas retrying search with configured providers after preferred providers returned no results configured_providers=%v", e.config.SearchProviderChain)
		results, candidates, sourceURLs, sources = e.searchReferenceAssets(ctx, strings.TrimSpace(req.Prompt), e.config.SearchProviderChain)
	}
	if e.sceneComposer != nil {
		composeCtx, cancel := fallbackContextWithTimeout(ctx, fallbackSceneComposeTimeout)
		data, composedURLs, composeErr := e.renderSceneCompose(composeCtx, req)
		cancel()
		if composeErr == nil {
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
				FallbackInfo: e.newFallbackInfo(
					FallbackStrategyWebCanvas,
					fallbackDisplayName(FallbackStrategyWebCanvas),
					appendUniqueStrings(sourceURLs, composedURLs...),
					sources,
					"",
					brief,
					"",
				),
			}, nil
		}
		log.Printf("[mediagen] fallback web_canvas scenecompose failed prompt=%q err=%v", truncateFallbackLogValue(reqPrompt(req), 160), composeErr)
	}
	if preferSearchReference {
		if directTask := e.directReferenceImageTask(req, brief, sourceURLs, sources, candidates); directTask != nil {
			if len(directTask.Response.Data) > 0 {
				result := directTask.Response.Data[0]
				log.Printf("[mediagen] fallback web_canvas selected direct reference local_url=%q original_url=%q", truncateFallbackLogValue(result.URL, 120), truncateFallbackLogValue(result.OriginalURL, 120))
			}
			return directTask, nil
		}
	}
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
			FallbackInfo: e.newFallbackInfo(FallbackStrategyWebCanvas, fallbackDisplayName(FallbackStrategyWebCanvas), sourceURLs, sources, "", brief, ""),
		}, nil
	}
	if e.browser != nil && e.browser() != nil {
		if htmlDoc, buildErr := e.buildPosterHTML(req, candidates, results); buildErr == nil {
			captureCtx, cancel := fallbackContextWithTimeout(ctx, fallbackBrowserCaptureTimeout)
			data, captureErr := e.capturePoster(captureCtx, htmlDoc)
			cancel()
			if captureErr == nil {
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
					FallbackInfo: e.newFallbackInfo(FallbackStrategyWebCanvas, fallbackDisplayName(FallbackStrategyWebCanvas), sourceURLs, sources, "", brief, ""),
				}, nil
			}
		}
	}

	if e.browser == nil || e.browser() == nil {
		return nil, fmt.Errorf("fallback web_canvas screenshot stage: browser service unavailable and local reference poster render failed")
	}
	htmlDoc, buildErr := e.buildPosterHTML(req, candidates, results)
	if buildErr != nil {
		return nil, fmt.Errorf("fallback web_canvas render stage: %w", buildErr)
	}
	captureCtx, cancel := fallbackContextWithTimeout(ctx, fallbackBrowserCaptureTimeout)
	data, err := e.capturePoster(captureCtx, htmlDoc)
	cancel()
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
		FallbackInfo: e.newFallbackInfo(FallbackStrategyWebCanvas, fallbackDisplayName(FallbackStrategyWebCanvas), sourceURLs, sources, "", brief, ""),
	}, nil
}

func (e *FallbackEngine) renderSceneCompose(ctx context.Context, req *MediaRequest) (string, []string, error) {
	if e == nil || e.sceneComposer == nil {
		return "", nil, fmt.Errorf("scene compose is not configured")
	}
	result, err := e.sceneComposer.Compose(ctx, scenecompose.ComposeRequest{
		Prompt: reqPrompt(req),
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
		Type:         req.Type,
		Category:     string(category),
		Provider:     fallbackProviderName,
		Model:        modelID,
		FallbackInfo: e.PendingInfoForRequest(req, modelID),
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
	brief := slideBriefFromMediaRequest(req)
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
			task.FallbackInfo = e.newFallbackInfo(FallbackStrategyPublicSpace, preset.DisplayName, nil, nil, preset.URL, brief, brief.TemplateID)
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
	brief := slideBriefFromMediaRequest(req)
	promptValue := strings.TrimSpace(req.Prompt)
	negativePromptValue := strings.TrimSpace(req.NegativePrompt)
	if brief.RenderMode == slidespec.RenderModeSlide {
		promptValue = slidespec.WrapPublicSpacePrompt(brief, promptValue)
		negativePromptValue = slidespec.BuildPublicSpaceNegativePrompt(negativePromptValue)
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
	if err := e.typeIntoFirstSelector(ctx, svc, tab.TargetID, preset.PromptSelectors, promptValue); err != nil {
		return nil, fmt.Errorf("prompt stage: %w", err)
	}
	if negativePromptValue != "" && len(preset.NegativePromptSelectors) > 0 {
		if err := e.typeIntoFirstSelector(ctx, svc, tab.TargetID, preset.NegativePromptSelectors, negativePromptValue); err != nil {
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

	startedAt := time.Now()
	lastSignalAt := startedAt
	lastURL := ""
	for {
		if preset.MaxRuntime > 0 && time.Since(startedAt) > preset.MaxRuntime {
			return nil, fmt.Errorf("max runtime exceeded for %s", preset.DisplayName)
		}
		if preset.StallTimeout > 0 && time.Since(lastSignalAt) > preset.StallTimeout {
			return nil, fmt.Errorf("stalled while waiting for %s", preset.DisplayName)
		}

		urlValue, _, infoErr := svc.PageInfo(ctx, tab.TargetID)
		if infoErr == nil && looksLikeServableMediaURL(urlValue) {
			return e.storePublicSpaceResult(urlValue, mediaTypeForFallbackModel(modelID), preset, brief)
		}
		if infoErr == nil {
			trimmed := strings.TrimSpace(urlValue)
			if trimmed != "" && trimmed != lastURL {
				lastURL = trimmed
				lastSignalAt = time.Now()
			}
		}

		if msg := e.extractFirstText(ctx, svc, tab.TargetID, preset.ErrorSelectors); strings.TrimSpace(msg) != "" {
			return nil, fmt.Errorf("space reported error: %s", msg)
		}

		if resultURL, kind := e.extractResultURL(ctx, svc, tab.TargetID, preset); resultURL != "" {
			return e.storePublicSpaceResult(resultURL, mediaTypeForResultKind(kind, modelID), preset, brief)
		}
		if e.hasAnySelector(ctx, svc, tab.TargetID, preset.ProcessingSelectors) {
			lastSignalAt = time.Now()
		}

		time.Sleep(preset.PollInterval)
	}
}

func (e *FallbackEngine) storePublicSpaceResult(rawURL string, mediaType MediaType, preset FallbackPublicSpacePreset, brief slidespec.Brief) (*MediaTask, error) {
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
		if brief.RenderMode == slidespec.RenderModeSlide {
			canvas := slideCanvasSize(brief.AspectRatio, e.config.ScreenshotWidth)
			result.Width = canvas.width
			result.Height = canvas.height
		} else {
			result.Width = e.config.ScreenshotWidth
			result.Height = e.config.ScreenshotHeight
		}
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
		FallbackInfo: e.newFallbackInfo(FallbackStrategyPublicSpace, preset.DisplayName, nil, nil, preset.URL, brief, brief.TemplateID),
	}, nil
}

func (e *FallbackEngine) searchPrompt(ctx context.Context, prompt string) ([]FallbackSearchResult, error) {
	return e.searchPromptWithProviders(ctx, prompt, e.config.SearchProviderChain)
}

func (e *FallbackEngine) searchPromptWithProviders(ctx context.Context, prompt string, providers []string) ([]FallbackSearchResult, error) {
	if e.searcher == nil {
		return nil, fmt.Errorf("searcher unavailable")
	}
	queries := e.searchQueriesForPrompt(ctx, prompt)
	if len(queries) == 0 {
		return nil, nil
	}

	seen := make(map[string]struct{}, e.config.SearchMaxResults)
	combined := make([]FallbackSearchResult, 0, e.config.SearchMaxResults)
	var firstErr error
	searchProviders := cloneProviderChain(providers)
	if len(searchProviders) == 0 {
		searchProviders = cloneProviderChain(e.config.SearchProviderChain)
	}
	log.Printf("[mediagen] fallback search begin prompt=%q providers=%v queries=%v", truncateFallbackLogValue(prompt, 160), searchProviders, queries)
	for _, query := range queries {
		log.Printf("[mediagen] fallback search query=%q providers=%v", truncateFallbackLogValue(query, 160), searchProviders)
		queryCtx, cancel := fallbackContextWithTimeout(ctx, fallbackSearchQueryTimeout)
		results, err := e.searcher.Search(queryCtx, query, e.config.SearchMaxResults, searchProviders)
		cancel()
		if err != nil {
			log.Printf("[mediagen] fallback search error query=%q providers=%v err=%v", truncateFallbackLogValue(query, 160), searchProviders, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		filteredResults := filterFallbackResults(prompt, results)
		if len(filteredResults) != len(results) {
			log.Printf("[mediagen] fallback search filtered query=%q providers=%v kept=%d dropped=%d top=%s", truncateFallbackLogValue(query, 160), searchProviders, len(filteredResults), len(results)-len(filteredResults), summarizeFallbackResultsForLog(filteredResults, 3))
		} else {
			log.Printf("[mediagen] fallback search results query=%q providers=%v count=%d top=%s", truncateFallbackLogValue(query, 160), searchProviders, len(results), summarizeFallbackResultsForLog(results, 3))
		}
		results = filteredResults
		for _, item := range results {
			key := strings.TrimSpace(firstNonEmptyValue(item.URL, item.Title))
			if key == "" {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			combined = append(combined, item)
			if len(combined) >= e.config.SearchMaxResults {
				log.Printf("[mediagen] fallback search combined count=%d top=%s", len(combined), summarizeFallbackResultsForLog(combined, 3))
				return combined, nil
			}
		}
	}
	if len(combined) > 0 {
		log.Printf("[mediagen] fallback search combined count=%d top=%s", len(combined), summarizeFallbackResultsForLog(combined, 3))
		return combined, nil
	}
	if firstErr != nil {
		log.Printf("[mediagen] fallback search failed prompt=%q providers=%v err=%v", truncateFallbackLogValue(prompt, 160), searchProviders, firstErr)
		return nil, firstErr
	}
	log.Printf("[mediagen] fallback search empty prompt=%q providers=%v", truncateFallbackLogValue(prompt, 160), searchProviders)
	return nil, nil
}

func (e *FallbackEngine) searchLicensedReferenceAssets(ctx context.Context, prompt string) []FallbackSearchResult {
	_ = ctx
	log.Printf("[mediagen] fallback source search disabled prompt=%q", truncateFallbackLogValue(prompt, 160))
	return nil
}

func (e *FallbackEngine) searchSearchEngineImageResults(ctx context.Context, prompt string) []FallbackSearchResult {
	queries := e.referenceSourceQueries(ctx, prompt)
	if len(queries) == 0 {
		return nil
	}
	maxResults := e.config.SearchMaxResults
	if maxResults <= 0 {
		maxResults = 6
	}
	seen := map[string]struct{}{}
	combined := make([]FallbackSearchResult, 0, maxResults)
	appendUnique := func(results []FallbackSearchResult) bool {
		for _, item := range results {
			if fallbackResultUsesBlockedSource(item) {
				continue
			}
			key := strings.TrimSpace(firstNonEmptyValue(item.ImageURL, item.URL, item.Title))
			if key == "" {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			combined = append(combined, item)
			if len(combined) >= maxResults {
				return true
			}
		}
		return false
	}

	shouldTryBaidu := isChineseLocale(e.locale) || containsChinese(prompt)
	for _, query := range queries {
		if ctx != nil && ctx.Err() != nil {
			break
		}
		if !shouldTryBaidu {
			continue
		}
		log.Printf("[mediagen] fallback image search query=%q provider=%q", truncateFallbackLogValue(query, 160), "baidu_images_json")
		results, err := e.searchBaiduImageJSON(ctx, query, maxResults-len(combined))
		if err != nil {
			log.Printf("[mediagen] fallback image search error query=%q provider=%q err=%v", truncateFallbackLogValue(query, 160), "baidu_images_json", err)
			continue
		}
		filteredResults := filterFallbackResults(prompt, results)
		if len(filteredResults) != len(results) {
			log.Printf("[mediagen] fallback image search filtered query=%q provider=%q kept=%d dropped=%d top=%s", truncateFallbackLogValue(query, 160), "baidu_images_json", len(filteredResults), len(results)-len(filteredResults), summarizeFallbackResultsForLog(filteredResults, 3))
		}
		results = filteredResults
		if len(results) == 0 {
			continue
		}
		log.Printf("[mediagen] fallback image search results query=%q provider=%q count=%d top=%s", truncateFallbackLogValue(query, 160), "baidu_images_json", len(results), summarizeFallbackResultsForLog(results, 3))
		if appendUnique(results) {
			break
		}
	}
	return combined
}

type fallbackBrowserImageEngine struct {
	Name                string
	Label               string
	BuildURL            func(query string) string
	Selectors           map[string]browser.SelectorConfig
	WaitForSelector     string
	SkipWaitLoad        bool
	WaitForMS           int
	SearchTimeout       time.Duration
	UsePageJudge        bool
	ResultLimit         int
	JudgeUseViewport    bool
	JudgeViewportHeight int
	Parse               func(*browser.ScrapeResponse, int) []FallbackSearchResult
}

func (e *FallbackEngine) referenceSourceQueries(ctx context.Context, prompt string) []string {
	baseQueries := e.searchQueriesForPrompt(ctx, prompt)
	if len(baseQueries) == 0 {
		return nil
	}
	queries := make([]string, 0, len(baseQueries)*2)
	appendUnique := func(query string) {
		query = strings.TrimSpace(query)
		if query == "" {
			return
		}
		for _, existing := range queries {
			if strings.EqualFold(existing, query) {
				return
			}
		}
		queries = append(queries, query)
	}
	for _, query := range baseQueries {
		translated := fallbackEnglishizeSearchQuery(query)
		if containsChinese(query) && translated != "" && !strings.EqualFold(translated, query) {
			appendUnique(translated)
		}
		appendUnique(query)
	}
	return queries
}

func fallbackEnglishizeSearchQuery(query string) string {
	trimmed := strings.Join(strings.Fields(strings.TrimSpace(query)), " ")
	if trimmed == "" || !containsChinese(trimmed) {
		return trimmed
	}
	replacer := strings.NewReplacer(
		"泰迪犬", "toy poodle",
		"泰迪狗", "toy poodle",
		"泰迪", "toy poodle",
		"贵宾犬", "poodle",
		"贵宾", "poodle",
		"小狗", "dog",
		"狗狗", "dog",
		"狗", "dog",
		"小猫", "cat",
		"猫咪", "cat",
		"猫", "cat",
		"雪地", "snow",
		"雪景", "snow landscape",
		"下雪", "snow",
		"雪", "snow",
		"草地", "grass",
		"沙滩", "beach",
		"森林", "forest",
		"公园", "park",
		"背景", "background",
		"风景", "landscape",
		"写实照片", "photo",
		"写实", "realistic",
		"照片", "photo",
		"玩耍", "playing",
		"奔跑", "running",
		"可爱", "cute",
		"抠图", "cutout",
		"免抠", "isolated",
		"透明背景", "isolated",
		"在", " ",
		"里", " ",
		"的", " ",
		"和", " ",
		"与", " ",
		"一张", " ",
		"一幅", " ",
		"一个", " ",
		"一只", " ",
		"一条", " ",
		"帮我", " ",
		"请", " ",
	)
	translated := replacer.Replace(trimmed)
	translated = strings.Map(func(r rune) rune {
		switch {
		case unicode.IsLetter(r), unicode.IsNumber(r):
			return unicode.ToLower(r)
		case unicode.IsSpace(r):
			return ' '
		case r == '-', r == '_':
			return ' '
		case r >= 0x4e00 && r <= 0x9fff:
			return ' '
		default:
			return ' '
		}
	}, translated)
	return strings.Join(strings.Fields(translated), " ")
}

func (e *FallbackEngine) searchBrowserImageResults(ctx context.Context, prompt string) []FallbackSearchResult {
	if e == nil {
		return nil
	}
	if e.browser == nil {
		log.Printf("[mediagen] fallback browser image search skipped err=%q", "browser service factory unavailable")
		return nil
	}
	svc := e.browser()
	if svc == nil {
		log.Printf("[mediagen] fallback browser image search skipped err=%q", "browser service unavailable")
		return nil
	}
	scraper, ok := svc.(fallbackBrowserScraper)
	if !ok {
		log.Printf("[mediagen] fallback browser image search skipped err=%q service_type=%T", "browser service does not support scrape", svc)
		return nil
	}
	if err := svc.Start(ctx); err != nil {
		log.Printf("[mediagen] fallback browser image search start failed err=%v", err)
		return nil
	}

	queries := e.referenceSourceQueries(ctx, prompt)
	if len(queries) > 2 {
		queries = queries[:2]
	}
	maxResults := e.config.SearchMaxResults
	if maxResults <= 0 {
		maxResults = 6
	}
	engines := fallbackBrowserImageEngines(prompt, e.locale)
	seen := map[string]struct{}{}
	combined := make([]FallbackSearchResult, 0, maxResults)
	for _, engine := range engines {
		for _, query := range queries {
			if ctx != nil && ctx.Err() != nil {
				return combined
			}
			searchURL := engine.BuildURL(query)
			if strings.TrimSpace(searchURL) == "" {
				continue
			}
			waitForSelector := engine.WaitForSelector
			log.Printf("[mediagen] fallback browser image search query=%q engine=%q", truncateFallbackLogValue(query, 160), engine.Name)
			searchTimeout := fallbackBrowserSearchTimeout
			if engine.SearchTimeout > 0 {
				searchTimeout = engine.SearchTimeout
			}
			req := &browser.ScrapeRequest{
				URL:          searchURL,
				Selectors:    engine.Selectors,
				WaitFor:      engine.WaitForMS,
				SkipWaitLoad: engine.SkipWaitLoad,
				Timeout:      int((searchTimeout + 2*time.Second) / time.Millisecond),
			}
			if waitForSelector != "" {
				req.WaitForSelector = &waitForSelector
			}
			var (
				resp *browser.ScrapeResponse
				err  error
			)
			const maxBrowserSearchAttempts = 2
			for attempt := 1; attempt <= maxBrowserSearchAttempts; attempt++ {
				resp, err = scraper.Scrape(ctx, req)
				if err == nil || !fallbackBrowserRetryableError(err) || ctx == nil || ctx.Err() != nil {
					break
				}
				if attempt < maxBrowserSearchAttempts {
					log.Printf("[mediagen] fallback browser image search retry query=%q engine=%q attempt=%d err=%v", truncateFallbackLogValue(query, 160), engine.Name, attempt+1, err)
				}
			}
			if err != nil {
				log.Printf("[mediagen] fallback browser image search error query=%q engine=%q err=%v", truncateFallbackLogValue(query, 160), engine.Name, err)
				continue
			}
			resultBudget := maxResults - len(combined)
			if engine.ResultLimit > 0 && resultBudget > engine.ResultLimit {
				resultBudget = engine.ResultLimit
			}
			results := engine.Parse(resp, resultBudget)
			if len(results) == 0 {
				log.Printf("[mediagen] fallback browser image search empty query=%q engine=%q final_url=%q", truncateFallbackLogValue(query, 160), engine.Name, truncateFallbackLogValue(firstNonEmptyValue(resp.URL, searchURL), 160))
				continue
			}
			if engine.UsePageJudge && len(results) > 1 {
				screenshotBase64 := ""
				fullPageCapture := true
				screenshotHeight := max(e.config.ScreenshotHeight, 960)
				if engine.JudgeUseViewport && engine.JudgeViewportHeight > 0 {
					fullPageCapture = false
					screenshotHeight = engine.JudgeViewportHeight
				}
				screenshotReq := &browser.ScreenshotRequest{
					URL:          searchURL,
					Format:       browser.FormatPNG,
					FullPage:     fullPageCapture,
					Width:        max(e.config.ScreenshotWidth, 1280),
					Height:       screenshotHeight,
					WaitFor:      engine.WaitForMS,
					SkipWaitLoad: engine.SkipWaitLoad,
					Timeout:      int((fallbackBrowserCaptureTimeout + 2*time.Second) / time.Millisecond),
				}
				if waitForSelector != "" {
					screenshotReq.WaitForSelector = &waitForSelector
				}
				var (
					screenshotResp *browser.ScreenshotResponse
					shotErr        error
				)
				const maxBrowserScreenshotAttempts = 2
				for attempt := 1; attempt <= maxBrowserScreenshotAttempts; attempt++ {
					screenshotResp, shotErr = svc.Screenshot(ctx, screenshotReq)
					if shotErr == nil || !fallbackBrowserRetryableError(shotErr) || ctx == nil || ctx.Err() != nil {
						break
					}
					if attempt < maxBrowserScreenshotAttempts {
						log.Printf("[mediagen] fallback browser image screenshot retry query=%q engine=%q attempt=%d err=%v", truncateFallbackLogValue(query, 160), engine.Name, attempt+1, shotErr)
					}
				}
				if shotErr == nil && screenshotResp != nil {
					screenshotBase64 = strings.TrimSpace(screenshotResp.Data)
				} else if shotErr != nil {
					log.Printf("[mediagen] fallback browser image screenshot skipped query=%q engine=%q err=%v", truncateFallbackLogValue(query, 160), engine.Name, shotErr)
				}
				results = e.rankBrowserResultsByIntent(ctx, query, engine, results, screenshotBase64)
			}
			filteredResults := filterFallbackResults(prompt, results)
			if len(filteredResults) != len(results) {
				log.Printf("[mediagen] fallback browser image search filtered query=%q engine=%q kept=%d dropped=%d top=%s", truncateFallbackLogValue(query, 160), engine.Name, len(filteredResults), len(results)-len(filteredResults), summarizeFallbackResultsForLog(filteredResults, 3))
			}
			results = filteredResults
			if len(results) == 0 {
				continue
			}
			log.Printf("[mediagen] fallback browser image search results query=%q engine=%q count=%d top=%s", truncateFallbackLogValue(query, 160), engine.Name, len(results), summarizeFallbackResultsForLog(results, 3))
			for _, item := range results {
				if fallbackResultUsesBlockedSource(item) {
					continue
				}
				key := strings.TrimSpace(firstNonEmptyValue(item.ImageURL, item.URL, item.Title))
				if key == "" {
					continue
				}
				if _, exists := seen[key]; exists {
					continue
				}
				seen[key] = struct{}{}
				combined = append(combined, item)
				if len(combined) >= maxResults {
					return combined
				}
			}
		}
	}
	return combined
}

func fallbackBrowserImageEngines(prompt, locale string) []fallbackBrowserImageEngine {
	_ = prompt
	engines := []fallbackBrowserImageEngine{
		{
			Name:     "craiyon_search",
			Label:    "Craiyon Search",
			BuildURL: fallbackCraiyonSearchURL,
			Selectors: map[string]browser.SelectorConfig{
				"images": {Selector: "main a[href*='/images/'] img[src*='img.craiyon.com'], a[href*='/images/'] img[src*='img.craiyon.com'], main img[src*='img.craiyon.com'], img[src*='img.craiyon.com']", Attribute: "src", Multiple: true, MaxMatches: fallbackCraiyonPreviewResultLimit},
				"alts":   {Selector: "main a[href*='/images/'] img[alt][src*='img.craiyon.com'], a[href*='/images/'] img[alt][src*='img.craiyon.com'], main img[alt][src*='img.craiyon.com'], img[alt][src*='img.craiyon.com']", Attribute: "alt", Multiple: true, MaxMatches: fallbackCraiyonPreviewResultLimit},
				"links":  {Selector: "main a[href*='/images/'], a[href*='/images/']", Attribute: "href", Multiple: true, MaxMatches: fallbackCraiyonPreviewResultLimit},
			},
			WaitForSelector:     "main img[src*='img.craiyon.com'], img[src*='img.craiyon.com']",
			SkipWaitLoad:        true,
			WaitForMS:           2200,
			SearchTimeout:       fallbackCraiyonSearchTimeout,
			UsePageJudge:        true,
			ResultLimit:         fallbackCraiyonPreviewResultLimit,
			JudgeUseViewport:    true,
			JudgeViewportHeight: fallbackCraiyonJudgeViewportHeight,
			Parse:               parseCraiyonImageScrapeResults,
		},
		{
			Name:     "bing_images",
			Label:    "Bing Images",
			BuildURL: func(query string) string { return "https://www.bing.com/images/search?q=" + url.QueryEscape(query) },
			Selectors: map[string]browser.SelectorConfig{
				"metadata": {Selector: "a.iusc", Attribute: "m", Multiple: true},
				"labels":   {Selector: "a.iusc", Attribute: "aria-label", Multiple: true},
			},
			WaitForSelector: "a.iusc",
			WaitForMS:       1200,
			UsePageJudge:    true,
			Parse:           parseBingImageScrapeResults,
		},
		{
			Name:     "google_images",
			Label:    "Google Images",
			BuildURL: func(query string) string { return "https://www.google.com/search?tbm=isch&q=" + url.QueryEscape(query) },
			Selectors: map[string]browser.SelectorConfig{
				"result_links": {Selector: "a[href*='/imgres?']", Attribute: "href", Multiple: true},
				"image_alts":   {Selector: "img[alt]", Attribute: "alt", Multiple: true},
			},
			WaitForSelector: "a[href*='/imgres?']",
			WaitForMS:       1500,
			UsePageJudge:    true,
			Parse:           parseGoogleImageScrapeResults,
		},
	}
	if isChineseLocale(locale) {
		engines = append(engines, fallbackBrowserImageEngine{
			Name:  "baidu_images",
			Label: "Baidu Images",
			BuildURL: func(query string) string {
				return "https://image.baidu.com/search/index?tn=baiduimage&word=" + url.QueryEscape(query)
			},
			Selectors: map[string]browser.SelectorConfig{
				"thumbs": {Selector: ".imgitem img, img.main_img", Attribute: "src", Multiple: true},
				"alts":   {Selector: ".imgitem img, img.main_img", Attribute: "alt", Multiple: true},
				"links":  {Selector: "a.imgitem", Attribute: "href", Multiple: true},
			},
			WaitForSelector: "a.imgitem, .imgitem img, img.main_img",
			WaitForMS:       1500,
			UsePageJudge:    true,
			Parse:           parseBaiduImageScrapeResults,
		})
	}
	return engines
}

func fallbackBrowserRetryableError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	if msg == "" {
		return false
	}
	return strings.Contains(msg, "execution context was destroyed") ||
		strings.Contains(msg, "target closed") ||
		strings.Contains(msg, "session closed")
}

type fallbackBrowserIntentSelection struct {
	SelectedIndex int     `json:"selected_index"`
	Score         float64 `json:"score"`
	Reason        string  `json:"reason"`
}

func (e *FallbackEngine) rankBrowserResultsByIntent(ctx context.Context, prompt string, engine fallbackBrowserImageEngine, results []FallbackSearchResult, screenshotBase64 string) []FallbackSearchResult {
	if len(results) <= 1 {
		return results
	}
	if e == nil || e.visionBridge == nil {
		return fallbackPromoteRandomBrowserResult(engine, results, "vision unavailable")
	}
	screenshotBase64 = prepareFallbackBrowserJudgeScreenshot(screenshotBase64)
	if strings.TrimSpace(screenshotBase64) == "" {
		return fallbackPromoteRandomBrowserResult(engine, results, "page screenshot unavailable")
	}
	judgePrompt := buildFallbackBrowserJudgePrompt(prompt, results)
	judgeCtx, cancel := fallbackContextWithTimeout(ctx, fallbackBrowserJudgeTimeout)
	defer cancel()
	raw, err := e.visionBridge.ChatWithVision(judgeCtx, judgePrompt, screenshotBase64)
	if err != nil {
		log.Printf("[mediagen] fallback browser intent judge skipped engine=%q err=%v", engine.Name, err)
		return fallbackPromoteRandomBrowserResult(engine, results, "vision judge error")
	}
	parsed, ok := parseFallbackBrowserIntentSelection(raw)
	if !ok {
		log.Printf("[mediagen] fallback browser intent judge parse failed engine=%q raw=%q", engine.Name, truncateFallbackLogValue(raw, 200))
		return fallbackPromoteRandomBrowserResult(engine, results, "vision judge parse failed")
	}
	if parsed.SelectedIndex < 1 || parsed.SelectedIndex > len(results) {
		return fallbackPromoteRandomBrowserResult(engine, results, "vision judge index invalid")
	}
	if parsed.Score > 0 && parsed.Score < fallbackBrowserJudgeMinScore {
		log.Printf("[mediagen] fallback browser intent judge low score engine=%q index=%d score=%.1f reason=%q", engine.Name, parsed.SelectedIndex, parsed.Score, truncateFallbackLogValue(parsed.Reason, 160))
		return fallbackPromoteRandomBrowserResult(engine, results, "vision judge low confidence")
	}
	reordered := fallbackPromoteBrowserIntentResult(results, parsed.SelectedIndex-1)
	log.Printf("[mediagen] fallback browser intent judge selected engine=%q index=%d score=%.1f reason=%q", engine.Name, parsed.SelectedIndex, parsed.Score, truncateFallbackLogValue(parsed.Reason, 160))
	return reordered
}

func prepareFallbackBrowserJudgeScreenshot(imageBase64 string) string {
	imageBase64 = strings.TrimSpace(imageBase64)
	if imageBase64 == "" {
		return ""
	}
	if len(imageBase64) <= fallbackBrowserJudgeMaxBase64Len {
		raw, err := base64.StdEncoding.DecodeString(imageBase64)
		if err != nil {
			return imageBase64
		}
		img, _, err := image.Decode(bytes.NewReader(raw))
		if err != nil {
			return imageBase64
		}
		bounds := img.Bounds()
		if bounds.Dx() <= fallbackBrowserJudgeMaxImageDim && bounds.Dy() <= fallbackBrowserJudgeMaxImageDim {
			return imageBase64
		}
	}

	raw, err := base64.StdEncoding.DecodeString(imageBase64)
	if err != nil {
		return ""
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		if len(imageBase64) <= fallbackBrowserJudgeMaxBase64Len {
			return imageBase64
		}
		return ""
	}

	resized := resizeImage(img, fallbackBrowserJudgeMaxImageDim)
	var buf bytes.Buffer
	if err := png.Encode(&buf, resized); err != nil {
		if len(imageBase64) <= fallbackBrowserJudgeMaxBase64Len {
			return imageBase64
		}
		return ""
	}
	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())
	if len(encoded) > fallbackBrowserJudgeMaxBase64Len {
		return ""
	}
	return encoded
}

func fallbackPromoteRandomBrowserResult(engine fallbackBrowserImageEngine, results []FallbackSearchResult, reason string) []FallbackSearchResult {
	if len(results) <= 1 {
		return results
	}
	selected := fallbackBrowserRandomIndex(len(results))
	if selected < 0 || selected >= len(results) {
		selected = 0
	}
	log.Printf("[mediagen] fallback browser intent judge random pick engine=%q index=%d reason=%q", engine.Name, selected+1, reason)
	return fallbackPromoteBrowserResult(results, selected)
}

func fallbackPromoteBrowserResult(results []FallbackSearchResult, selected int) []FallbackSearchResult {
	if len(results) == 0 || selected < 0 || selected >= len(results) {
		return results
	}
	reordered := make([]FallbackSearchResult, 0, len(results))
	reordered = append(reordered, results[selected])
	reordered = append(reordered, results[:selected]...)
	reordered = append(reordered, results[selected+1:]...)
	return reordered
}

func fallbackPromoteBrowserIntentResult(results []FallbackSearchResult, selected int) []FallbackSearchResult {
	reordered := fallbackPromoteBrowserResult(results, selected)
	if len(reordered) == 0 || selected < 0 || selected >= len(results) {
		return reordered
	}
	for idx := range reordered {
		reordered[idx].IntentSelected = false
	}
	reordered[0].IntentSelected = true
	return reordered
}

func buildFallbackBrowserJudgePrompt(prompt string, results []FallbackSearchResult) string {
	var b strings.Builder
	b.WriteString("You are selecting the single image result that best matches the user's intent.\n")
	b.WriteString("Score only semantic intent match, not image quality.\n")
	b.WriteString("Use the screenshot of the full results page.\n")
	b.WriteString("Assume the candidate list below follows reading order on the page: left-to-right, top-to-bottom.\n")
	b.WriteString("Return JSON only: {\"selected_index\":1,\"score\":0-100,\"reason\":\"...\"}\n\n")
	b.WriteString("User intent:\n")
	b.WriteString(strings.TrimSpace(prompt))
	b.WriteString("\n\nCandidates:\n")
	for idx, item := range results {
		title := strings.TrimSpace(item.Title)
		if title == "" {
			title = "(no title)"
		}
		b.WriteString(fmt.Sprintf("%d. title=%q provider=%q url=%q\n", idx+1, title, strings.TrimSpace(item.Provider), truncateFallbackLogValue(item.URL, 160)))
	}
	return b.String()
}

func parseFallbackBrowserIntentSelection(raw string) (fallbackBrowserIntentSelection, bool) {
	content := strings.TrimSpace(raw)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	var parsed fallbackBrowserIntentSelection
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return fallbackBrowserIntentSelection{}, false
	}
	return parsed, true
}

func fallbackCraiyonSearchURL(query string) string {
	slug := fallbackCraiyonQuerySlug(query)
	if slug == "" {
		return ""
	}
	return "https://www.craiyon.com/en/search/" + url.PathEscape(slug)
}

func fallbackCraiyonQuerySlug(query string) string {
	query = fallbackEnglishizeSearchQuery(query)
	query = strings.ToLower(strings.TrimSpace(query))
	query = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case unicode.IsSpace(r), r == '-', r == '_':
			return ' '
		default:
			return ' '
		}
	}, query)
	parts := strings.Fields(query)
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "-")
}

func (e *FallbackEngine) searchQueriesForPrompt(ctx context.Context, prompt string) []string {
	original := strings.TrimSpace(prompt)
	if original == "" {
		return nil
	}
	semantic := normalizeMediaPrompt(original)
	if semantic == "" {
		semantic = original
	}
	queries := make([]string, 0, 4)
	appendUnique := func(query string) {
		query = strings.TrimSpace(query)
		if query == "" {
			return
		}
		for _, existing := range queries {
			if strings.EqualFold(existing, query) {
				return
			}
		}
		queries = append(queries, query)
	}

	planCtx, cancel := fallbackContextWithTimeout(ctx, fallbackSearchPlannerTimeout)
	defer cancel()
	plan, err := scenecompose.NewPlanner(e.scenePlannerLLM).Plan(planCtx, scenecompose.ComposeRequest{
		Prompt: semantic,
		Width:  e.config.ScreenshotWidth,
		Height: e.config.ScreenshotHeight,
		Locale: e.locale,
	})
	preferChinese := containsChinese(semantic) || isChineseLocale(e.locale)
	sceneQuery := fallbackSceneSearchQuery(semantic, plan, preferChinese)
	if fallbackPlanHasExplicitSearchHints(plan) {
		appendUnique(sceneQuery)
		appendUnique(fallbackSceneSearchQueryEN(plan))
	}
	appendUnique(semantic)
	if err == nil && plan != nil {
		appendUnique(sceneQuery)
		appendUnique(fallbackSceneSearchQueryEN(plan))
		for _, query := range fallbackForegroundSearchQueries(semantic, plan, preferChinese) {
			appendUnique(query)
		}
		for _, query := range fallbackForegroundSearchQueriesEN(plan) {
			appendUnique(query)
		}
		appendUnique(fallbackBackgroundSearchQuery(plan, preferChinese))
		appendUnique(fallbackBackgroundSearchQueryEN(plan))
	}
	if !strings.EqualFold(semantic, original) && !looksLikePhotoPromptText(original) {
		appendUnique(original)
	}
	return queries
}

func fallbackPlanHasExplicitSearchHints(plan *scenecompose.ScenePlan) bool {
	if plan == nil {
		return false
	}
	if strings.TrimSpace(plan.SceneQuery) != "" || strings.TrimSpace(plan.SceneQueryEN) != "" {
		return true
	}
	return strings.TrimSpace(plan.BackgroundQuery) != "" || strings.TrimSpace(plan.BackgroundQueryEN) != ""
}

func normalizeMediaPrompt(prompt string) string {
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" {
		return ""
	}
	for {
		next := trimFallbackPromptPrefix(trimmed)
		if next == trimmed {
			break
		}
		trimmed = next
	}
	trimmed = strings.TrimSpace(strings.TrimLeft(trimmed, "，。,:：;；!！?？-—_"))
	if trimmed == "" {
		return strings.TrimSpace(prompt)
	}
	return trimmed
}

func trimFallbackPromptPrefix(prompt string) string {
	trimmed := strings.TrimSpace(prompt)
	lower := strings.ToLower(trimmed)
	for _, prefix := range fallbackPromptPrefixes {
		if strings.HasPrefix(lower, strings.ToLower(prefix)) {
			next := strings.TrimSpace(trimmed[len(prefix):])
			next = strings.TrimSpace(strings.TrimLeft(next, "，。,:：;；!！?？-—_"))
			if next != "" {
				return next
			}
		}
	}
	return trimmed
}

func fallbackSceneSearchQuery(prompt string, plan *scenecompose.ScenePlan, preferChinese bool) string {
	if plan == nil {
		return ""
	}
	if query := strings.TrimSpace(plan.SceneQuery); query != "" {
		return query
	}
	subjects := fallbackSceneSearchSubjects(prompt, plan, preferChinese)
	background := strings.TrimSpace(plan.Background)
	parts := make([]string, 0, len(subjects)+4)
	parts = append(parts, subjects...)
	if background != "" {
		parts = append(parts, background)
	}
	if weather := strings.TrimSpace(plan.Weather); weather != "" && weather != "clear" {
		parts = append(parts, weather)
	}
	if timeOfDay := strings.TrimSpace(plan.TimeOfDay); timeOfDay != "" && timeOfDay != "day" {
		parts = append(parts, timeOfDay)
	}
	if preferChinese || containsChinese(prompt) || containsChinese(background) {
		parts = append(parts, "照片")
	} else {
		parts = append(parts, "photo")
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func fallbackSceneSearchQueryEN(plan *scenecompose.ScenePlan) string {
	if plan == nil {
		return ""
	}
	return strings.TrimSpace(plan.SceneQueryEN)
}

func fallbackSceneSearchSubjects(prompt string, plan *scenecompose.ScenePlan, preferChinese bool) []string {
	if plan == nil || len(plan.Foreground) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	subjects := make([]string, 0, len(plan.Foreground))
	for _, item := range plan.Foreground {
		subject := strings.TrimSpace(fallbackSceneSubjectSearchTerm(prompt, item, preferChinese))
		if subject == "" {
			continue
		}
		if _, ok := seen[subject]; ok {
			continue
		}
		seen[subject] = struct{}{}
		subjects = append(subjects, subject)
	}
	return subjects
}

func fallbackSceneSubjectSearchTerm(prompt string, item scenecompose.ForegroundPlan, preferChinese bool) string {
	for _, candidate := range []string{item.SearchQuery, item.FallbackQuery} {
		if subject := strings.TrimSpace(fallbackSearchSubjectFromQuery(candidate)); subject != "" {
			return subject
		}
	}
	return fallbackSubjectSearchTerm(prompt, item.Type, preferChinese)
}

func fallbackSearchSubjectFromQuery(query string) string {
	value := strings.TrimSpace(query)
	if value == "" {
		return ""
	}
	replacements := []string{
		"isolated png transparent",
		"transparent png",
		"isolated png",
		"white background photo",
		"white background",
		"landscape background photo",
		"背景 照片 风景",
		"透明背景 png",
		"透明背景",
		"写实照片",
		"transparent",
		"isolated",
		"cutout",
		"background",
		"photo",
		"png",
		"背景",
		"照片",
		"抠图",
		"免抠",
	}
	for _, marker := range replacements {
		value = strings.ReplaceAll(value, marker, " ")
	}
	value = strings.TrimSpace(strings.Trim(value, "，,.;:。!?！？-—_"))
	return strings.Join(strings.Fields(value), " ")
}

func fallbackSubjectSearchTerm(prompt, subjectType string, preferChinese bool) string {
	trimmedType := strings.TrimSpace(subjectType)
	lowerPrompt := strings.ToLower(prompt)
	switch trimmedType {
	case "dog":
		for _, candidate := range []string{"泰迪犬", "泰迪狗", "泰迪", "贵宾犬", "贵宾", "toy poodle", "teddy dog", "poodle", "puppy", "小狗", "狗"} {
			if strings.Contains(lowerPrompt, strings.ToLower(candidate)) {
				return candidate
			}
		}
		if preferChinese || containsChinese(prompt) {
			return "狗"
		}
	case "cat":
		for _, candidate := range []string{"猫咪", "小猫", "cat", "kitten", "猫"} {
			if strings.Contains(lowerPrompt, strings.ToLower(candidate)) {
				return candidate
			}
		}
		if preferChinese || containsChinese(prompt) {
			return "猫"
		}
	}
	return trimmedType
}

func fallbackBackgroundSearchQuery(plan *scenecompose.ScenePlan, preferChinese bool) string {
	if plan == nil {
		return ""
	}
	if query := strings.TrimSpace(plan.BackgroundQuery); query != "" {
		return query
	}
	background := strings.TrimSpace(plan.Background)
	if background == "" {
		return ""
	}
	parts := []string{background}
	if weather := strings.TrimSpace(plan.Weather); weather != "" && weather != "clear" {
		parts = append(parts, weather)
	}
	if timeOfDay := strings.TrimSpace(plan.TimeOfDay); timeOfDay != "" && timeOfDay != "day" {
		parts = append(parts, timeOfDay)
	}
	if style := strings.TrimSpace(plan.Style); style != "" && style != "realistic" {
		parts = append(parts, style)
	}
	if preferChinese || containsChinese(background) {
		parts = append(parts, "背景 照片 风景")
	} else {
		parts = append(parts, "landscape background photo")
	}
	return strings.Join(parts, " ")
}

func fallbackBackgroundSearchQueryEN(plan *scenecompose.ScenePlan) string {
	if plan == nil {
		return ""
	}
	return strings.TrimSpace(plan.BackgroundQueryEN)
}

func fallbackForegroundSearchQueries(prompt string, plan *scenecompose.ScenePlan, preferChinese bool) []string {
	if plan == nil || len(plan.Foreground) == 0 {
		return nil
	}
	queries := make([]string, 0, len(plan.Foreground))
	seen := make(map[string]struct{}, len(plan.Foreground))
	appendUnique := func(query string) {
		query = strings.TrimSpace(query)
		if query == "" {
			return
		}
		if _, ok := seen[query]; ok {
			return
		}
		seen[query] = struct{}{}
		queries = append(queries, query)
	}

	for _, item := range plan.Foreground {
		if query := strings.TrimSpace(item.FallbackQuery); query != "" {
			appendUnique(query)
			continue
		}
		subject := strings.TrimSpace(fallbackSceneSubjectSearchTerm(prompt, item, preferChinese))
		if subject == "" {
			continue
		}
		if preferChinese || containsChinese(subject) {
			appendUnique(strings.TrimSpace(subject + " 照片"))
		} else {
			appendUnique(strings.TrimSpace(subject + " photo"))
		}
	}

	return queries
}

func fallbackForegroundSearchQueriesEN(plan *scenecompose.ScenePlan) []string {
	if plan == nil || len(plan.Foreground) == 0 {
		return nil
	}
	queries := make([]string, 0, len(plan.Foreground))
	seen := make(map[string]struct{}, len(plan.Foreground))
	appendUnique := func(query string) {
		query = strings.TrimSpace(query)
		if query == "" {
			return
		}
		if _, ok := seen[query]; ok {
			return
		}
		seen[query] = struct{}{}
		queries = append(queries, query)
	}

	for _, item := range plan.Foreground {
		if query := strings.TrimSpace(item.FallbackQueryEN); query != "" {
			appendUnique(query)
			continue
		}
		appendUnique(item.SearchQueryEN)
	}

	return queries
}

func looksLikePhotoPrompt(req *MediaRequest) bool {
	return looksLikePhotoPromptText(reqPrompt(req))
}

func looksLikePhotoPromptText(prompt string) bool {
	prompt = strings.ToLower(strings.TrimSpace(prompt))
	if prompt == "" {
		return false
	}
	signals := []string{"photo", "photograph", "real photo", "照片", "写真", "真实照片"}
	for _, signal := range signals {
		if strings.Contains(prompt, signal) {
			return true
		}
	}
	return false
}

func (e *FallbackEngine) referenceSearchProviderChain(prompt string, preferSearchReference bool) []string {
	providers := sanitizeFallbackImageSearchProviders(e.config.SearchProviderChain)
	if !preferSearchReference {
		return providers
	}
	if !looksLikePhotoPromptText(prompt) {
		return providers
	}
	if providerChainContains(providers, "bing") {
		return []string{"bing"}
	}
	return providers
}

func sanitizeFallbackImageSearchProviders(providers []string) []string {
	filtered := make([]string, 0, len(providers))
	seen := make(map[string]struct{}, len(providers))
	for _, provider := range providers {
		provider = strings.ToLower(strings.TrimSpace(provider))
		if provider == "" || provider == "duckduckgo" || provider == "google" || provider == "baidu" {
			continue
		}
		if _, ok := seen[provider]; ok {
			continue
		}
		seen[provider] = struct{}{}
		filtered = append(filtered, provider)
	}
	if len(filtered) > 0 {
		return filtered
	}
	return []string{"bing"}
}

func (e *FallbackEngine) directReferenceImageTask(
	req *MediaRequest,
	brief slidespec.Brief,
	sourceURLs []string,
	sources []MediaFallbackSource,
	candidates []fallbackImageCandidate,
) *MediaTask {
	if len(candidates) == 0 {
		return nil
	}
	candidate := candidates[0]
	if strings.TrimSpace(candidate.LocalURL) == "" {
		return nil
	}
	contentType := strings.TrimSpace(candidate.ContentType)
	if contentType == "" {
		contentType = "image/png"
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
			Data: []MediaResult{{
				URL:           candidate.LocalURL,
				ContentType:   contentType,
				OriginalURL:   candidate.Source,
				RevisedPrompt: strings.TrimSpace(req.Prompt),
			}},
		},
		FallbackInfo: e.newReferenceImageFallbackInfo(FallbackStrategyWebCanvas, fallbackDisplayName(FallbackStrategyWebCanvas), sourceURLs, sources, brief),
	}
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
	Title       string
	Source      string
	ContentType string
	DataURL     string
	LocalURL    string
	Attribution *MediaFallbackSource
}

func (e *FallbackEngine) collectImageCandidates(ctx context.Context, results []FallbackSearchResult) []fallbackImageCandidate {
	if len(results) == 0 || e.storage == nil {
		return nil
	}
	candidates := make([]fallbackImageCandidate, 0, 4)
	for _, item := range results {
		if ctx != nil && ctx.Err() != nil {
			break
		}
		if len(candidates) >= 4 {
			break
		}
		if fallbackResultUsesBlockedSource(item) {
			continue
		}
		log.Printf("[mediagen] fallback candidate resolve start title=%q source=%q", truncateFallbackLogValue(item.Title, 72), truncateFallbackLogValue(item.URL, 120))
		targetURL := strings.TrimSpace(firstNonEmptyValue(item.ImageURL, item.URL))
		if targetURL == "" {
			continue
		}
		resolveCtx, cancel := fallbackContextWithTimeout(ctx, fallbackImageResolveTimeout)
		sourceURL, data, contentType, err := e.resolveSearchImage(resolveCtx, targetURL)
		cancel()
		if (err != nil || len(data) == 0) && strings.TrimSpace(item.ThumbnailURL) != "" && !strings.EqualFold(strings.TrimSpace(item.ThumbnailURL), targetURL) {
			resolveCtx, cancel = fallbackContextWithTimeout(ctx, fallbackImageResolveTimeout)
			sourceURL, data, contentType, err = e.resolveSearchImage(resolveCtx, strings.TrimSpace(item.ThumbnailURL))
			cancel()
		}
		if err != nil || len(data) == 0 {
			if err != nil {
				log.Printf("[mediagen] fallback candidate resolve skipped source=%q err=%v", truncateFallbackLogValue(targetURL, 120), err)
			} else {
				log.Printf("[mediagen] fallback candidate resolve skipped source=%q err=no image bytes", truncateFallbackLogValue(targetURL, 120))
			}
			continue
		}
		localURL, err := e.storage.StoreBytes(data, contentType, MediaTypeImage)
		if err != nil {
			log.Printf("[mediagen] fallback candidate store failed source=%q content_type=%q err=%v", truncateFallbackLogValue(sourceURL, 120), contentType, err)
			continue
		}
		attribution := fallbackResultToSource(item)
		if attribution != nil {
			if strings.TrimSpace(attribution.AssetURL) == "" {
				attribution.AssetURL = strings.TrimSpace(firstNonEmptyValue(item.ImageURL, sourceURL))
			}
			if strings.TrimSpace(attribution.PageURL) == "" {
				attribution.PageURL = strings.TrimSpace(item.URL)
			}
		}
		sourceRef := strings.TrimSpace(item.URL)
		if sourceRef == "" {
			sourceRef = strings.TrimSpace(firstNonEmptyValue(sourceURL, item.ImageURL))
		}
		candidates = append(candidates, fallbackImageCandidate{
			Title:       item.Title,
			Source:      sourceRef,
			ContentType: contentType,
			LocalURL:    localURL,
			DataURL:     bytesToDataURL(data, contentType),
			Attribution: attribution,
		})
		log.Printf("[mediagen] fallback candidate resolved title=%q source=%q content_type=%q local_url=%q", truncateFallbackLogValue(item.Title, 72), truncateFallbackLogValue(sourceRef, 120), contentType, truncateFallbackLogValue(localURL, 120))
	}
	return candidates
}

func (e *FallbackEngine) searchReferenceAssets(ctx context.Context, prompt string, providers []string) ([]FallbackSearchResult, []fallbackImageCandidate, []string, []MediaFallbackSource) {
	sourceSearchCtx, sourceCancel := fallbackContextWithTimeout(ctx, fallbackSearchStageTimeout)
	results := e.searchLicensedReferenceAssets(sourceSearchCtx, prompt)
	sourceCancel()

	candidateCtx, candidateCancel := fallbackContextWithTimeout(ctx, fallbackSearchStageTimeout)
	candidates := e.collectImageCandidates(candidateCtx, results)
	candidateCancel()

	if len(candidates) == 0 {
		browserCtx, browserCancel := fallbackContextWithTimeout(ctx, fallbackBrowserImageStageTimeout)
		browserResults := e.searchBrowserImageResults(browserCtx, prompt)
		browserCancel()
		if len(browserResults) > 0 {
			results = browserResults
			candidateCtx, candidateCancel = fallbackContextWithTimeout(ctx, fallbackSearchStageTimeout)
			candidates = e.collectImageCandidates(candidateCtx, results)
			candidateCancel()
		}
	}
	if len(candidates) == 0 {
		engineCtx, engineCancel := fallbackContextWithTimeout(ctx, fallbackSearchStageTimeout)
		engineResults := e.searchSearchEngineImageResults(engineCtx, prompt)
		engineCancel()
		if len(engineResults) > 0 {
			results = engineResults
			candidateCtx, candidateCancel = fallbackContextWithTimeout(ctx, fallbackSearchStageTimeout)
			candidates = e.collectImageCandidates(candidateCtx, results)
			candidateCancel()
		}
	}
	if len(candidates) == 0 {
		searchCtx, cancel := fallbackContextWithTimeout(ctx, fallbackSearchStageTimeout)
		legacyResults, err := e.searchPromptWithProviders(searchCtx, prompt, sanitizeFallbackImageSearchProviders(providers))
		cancel()
		if err == nil && len(legacyResults) > 0 {
			results = legacyResults
			candidateCtx, candidateCancel = fallbackContextWithTimeout(ctx, fallbackSearchStageTimeout)
			candidates = e.collectImageCandidates(candidateCtx, results)
			candidateCancel()
		}
	}

	sourceURLs := fallbackResultSourceURLs(results)
	sources := fallbackSourcesFromResults(results)
	log.Printf("[mediagen] fallback reference assets prompt=%q providers=%v results=%d candidates=%d result_top=%s candidate_top=%s", truncateFallbackLogValue(prompt, 160), cloneProviderChain(providers), len(results), len(candidates), summarizeFallbackResultsForLog(results, 3), summarizeFallbackCandidatesForLog(candidates, 3))
	return results, candidates, sourceURLs, sources
}

func fallbackResultSourceURLs(results []FallbackSearchResult) []string {
	if len(results) == 0 {
		return nil
	}
	urls := make([]string, 0, len(results))
	seen := map[string]struct{}{}
	for _, item := range results {
		value := strings.TrimSpace(firstNonEmptyValue(item.URL, item.ImageURL))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		urls = append(urls, value)
	}
	return urls
}

func fallbackSourcesFromResults(results []FallbackSearchResult) []MediaFallbackSource {
	if len(results) == 0 {
		return nil
	}
	sources := make([]MediaFallbackSource, 0, len(results))
	seen := map[string]struct{}{}
	for _, item := range results {
		source := fallbackResultToSource(item)
		if source == nil {
			continue
		}
		key := strings.TrimSpace(firstNonEmptyValue(source.PageURL, source.AssetURL, source.Title))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		sources = append(sources, *source)
	}
	return sources
}

func fallbackStringSlice(raw interface{}) []string {
	switch value := raw.(type) {
	case []string:
		out := make([]string, 0, len(value))
		for _, item := range value {
			out = append(out, strings.TrimSpace(item))
		}
		return out
	case []interface{}:
		out := make([]string, 0, len(value))
		for _, item := range value {
			text, ok := item.(string)
			if !ok {
				continue
			}
			out = append(out, strings.TrimSpace(text))
		}
		return out
	default:
		return nil
	}
}

func parseBingImageScrapeResults(resp *browser.ScrapeResponse, maxResults int) []FallbackSearchResult {
	if resp == nil || maxResults <= 0 {
		return nil
	}
	metadata := fallbackStringSlice(resp.Data["metadata"])
	labels := fallbackStringSlice(resp.Data["labels"])
	results := make([]FallbackSearchResult, 0, min(maxResults, len(metadata)))
	for idx, raw := range metadata {
		raw = strings.TrimSpace(stdhtml.UnescapeString(raw))
		if raw == "" {
			continue
		}
		var payload struct {
			PageURL     string `json:"purl"`
			ImageURL    string `json:"murl"`
			Thumbnail   string `json:"turl"`
			Title       string `json:"t"`
			Description string `json:"desc"`
		}
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			continue
		}
		title := strings.TrimSpace(firstNonEmptyValue(payload.Title, payload.Description))
		if idx < len(labels) && strings.TrimSpace(labels[idx]) != "" {
			title = strings.TrimSpace(labels[idx])
		}
		imageURL := strings.TrimSpace(payload.ImageURL)
		pageURL := strings.TrimSpace(payload.PageURL)
		if imageURL == "" || pageURL == "" {
			continue
		}
		results = append(results, FallbackSearchResult{
			Title:           title,
			URL:             pageURL,
			Description:     strings.TrimSpace(payload.Description),
			ImageURL:        imageURL,
			ThumbnailURL:    strings.TrimSpace(payload.Thumbnail),
			Provider:        "Bing Images",
			SourceNote:      "Search engine image result; license not verified",
			VerifiedLicense: false,
		})
		if len(results) >= maxResults {
			break
		}
	}
	return results
}

func parseCraiyonImageScrapeResults(resp *browser.ScrapeResponse, maxResults int) []FallbackSearchResult {
	if resp == nil || maxResults <= 0 {
		return nil
	}
	images := fallbackStringSlice(resp.Data["images"])
	alts := fallbackStringSlice(resp.Data["alts"])
	links := fallbackStringSlice(resp.Data["links"])
	results := make([]FallbackSearchResult, 0, min(maxResults, len(images)))
	seen := map[string]struct{}{}
	for idx, raw := range images {
		imageURL := strings.TrimSpace(resolveRelativeURL(firstNonEmptyValue(resp.URL, "https://www.craiyon.com/"), raw))
		if !fallbackLooksLikeCraiyonImageURL(imageURL) {
			continue
		}
		if _, ok := seen[imageURL]; ok {
			continue
		}
		seen[imageURL] = struct{}{}
		title := ""
		if idx < len(alts) {
			title = strings.TrimSpace(alts[idx])
		}
		pageURL := strings.TrimSpace(firstNonEmptyValue(resp.URL, "https://www.craiyon.com/"))
		if idx < len(links) {
			if resolved := strings.TrimSpace(resolveRelativeURL(pageURL, links[idx])); resolved != "" && strings.Contains(resolved, "craiyon.com") {
				pageURL = resolved
			}
		}
		results = append(results, FallbackSearchResult{
			Title:           title,
			URL:             pageURL,
			ImageURL:        imageURL,
			ThumbnailURL:    imageURL,
			Provider:        "Craiyon Search",
			SourceNote:      "Browser search result; license not verified",
			VerifiedLicense: false,
		})
		if len(results) >= maxResults {
			break
		}
	}
	return results
}

func parseGoogleImageScrapeResults(resp *browser.ScrapeResponse, maxResults int) []FallbackSearchResult {
	if resp == nil || maxResults <= 0 {
		return nil
	}
	links := fallbackStringSlice(resp.Data["result_links"])
	alts := fallbackStringSlice(resp.Data["image_alts"])
	results := make([]FallbackSearchResult, 0, min(maxResults, len(links)))
	seen := map[string]struct{}{}
	for idx, rawLink := range links {
		if strings.TrimSpace(rawLink) == "" {
			continue
		}
		resolved := resolveRelativeURL(firstNonEmptyValue(resp.URL, "https://www.google.com/"), rawLink)
		parsed, err := url.Parse(resolved)
		if err != nil {
			continue
		}
		values := parsed.Query()
		imageURL := strings.TrimSpace(firstNonEmptyValue(values.Get("imgurl"), values.Get("imgrefurl")))
		pageURL := strings.TrimSpace(firstNonEmptyValue(values.Get("imgrefurl"), values.Get("imgurl")))
		if imageURL == "" || pageURL == "" {
			continue
		}
		if _, ok := seen[imageURL]; ok {
			continue
		}
		seen[imageURL] = struct{}{}
		title := ""
		if idx < len(alts) {
			title = normalizeGoogleImageResultTitle(alts[idx])
		}
		results = append(results, FallbackSearchResult{
			Title:           title,
			URL:             pageURL,
			ImageURL:        imageURL,
			Provider:        "Google Images",
			SourceNote:      "Search engine image result; license not verified",
			VerifiedLicense: false,
		})
		if len(results) >= maxResults {
			break
		}
	}
	return results
}

func normalizeGoogleImageResultTitle(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	for {
		next := trimGoogleImageResultBoilerplate(trimmed)
		if next == trimmed {
			break
		}
		trimmed = next
	}
	return trimmed
}

func trimGoogleImageResultBoilerplate(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	for _, affix := range fallbackGoogleImageResultTitleAffixes {
		next := trimmed
		if affix.prefix != "" {
			if !strings.HasPrefix(next, affix.prefix) {
				continue
			}
			next = strings.TrimSpace(next[len(affix.prefix):])
		}
		if affix.suffix != "" {
			if !strings.HasSuffix(next, affix.suffix) {
				continue
			}
			next = strings.TrimSpace(next[:len(next)-len(affix.suffix)])
		}
		next = strings.TrimSpace(strings.Trim(next, "\"'`“”‘’[](){}<>《》〈〉「」『』【】"))
		if next != "" && next != trimmed {
			return next
		}
	}
	return trimmed
}

func fallbackLooksLikeCraiyonImageURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if host == "" {
		return false
	}
	if !strings.Contains(host, "craiyon.com") {
		return false
	}
	pathValue := strings.ToLower(strings.TrimSpace(parsed.Path))
	if pathValue == "" || strings.HasSuffix(pathValue, ".svg") {
		return false
	}
	return true
}

func parseBaiduImageScrapeResults(resp *browser.ScrapeResponse, maxResults int) []FallbackSearchResult {
	if resp == nil || maxResults <= 0 {
		return nil
	}
	thumbs := fallbackStringSlice(resp.Data["thumbs"])
	alts := fallbackStringSlice(resp.Data["alts"])
	links := fallbackStringSlice(resp.Data["links"])
	results := make([]FallbackSearchResult, 0, min(maxResults, len(thumbs)))
	for idx, thumb := range thumbs {
		thumb = strings.TrimSpace(thumb)
		if thumb == "" || strings.HasPrefix(thumb, "data:") {
			continue
		}
		title := ""
		if idx < len(alts) {
			title = strings.TrimSpace(alts[idx])
		}
		pageURL := ""
		if idx < len(links) {
			pageURL = strings.TrimSpace(resolveRelativeURL(firstNonEmptyValue(resp.URL, "https://image.baidu.com/"), links[idx]))
		}
		if pageURL == "" {
			pageURL = strings.TrimSpace(firstNonEmptyValue(resp.URL, "https://image.baidu.com/"))
		}
		results = append(results, FallbackSearchResult{
			Title:           title,
			URL:             pageURL,
			ImageURL:        thumb,
			ThumbnailURL:    thumb,
			Provider:        "Baidu Images",
			SourceNote:      "Search engine image result; license not verified",
			VerifiedLicense: false,
		})
		if len(results) >= maxResults {
			break
		}
	}
	return results
}

func (e *FallbackEngine) searchBaiduImageJSON(ctx context.Context, query string, maxResults int) ([]FallbackSearchResult, error) {
	if maxResults <= 0 {
		return nil, nil
	}
	params := url.Values{}
	params.Set("tn", "resultjson_com")
	params.Set("ipn", "rj")
	params.Set("word", query)
	params.Set("pn", "0")
	params.Set("rn", fmt.Sprintf("%d", max(3, min(maxResults*2, 12))))

	var payload struct {
		Data []struct {
			FromPageTitleEnc string `json:"fromPageTitleEnc"`
			FromPageTitle    string `json:"fromPageTitle"`
			ThumbURL         string `json:"thumbURL"`
			MiddleURL        string `json:"middleURL"`
			ObjURL           string `json:"objURL"`
			FromURLHost      string `json:"fromURLHost"`
			FromPageURL      string `json:"fromPageUrl"`
			ReplaceURL       []struct {
				ObjURL  string `json:"ObjURL"`
				ObjUrl  string `json:"ObjUrl"`
				FromURL string `json:"FromURL"`
				FromUrl string `json:"FromUrl"`
			} `json:"replaceUrl"`
		} `json:"data"`
	}
	if err := e.fetchFallbackSourceJSON(ctx, "https://image.baidu.com/search/acjson?"+params.Encode(), &payload); err != nil {
		return nil, err
	}
	results := make([]FallbackSearchResult, 0, min(maxResults, len(payload.Data)))
	for _, item := range payload.Data {
		pageURL := strings.TrimSpace(item.FromPageURL)
		imageURL := strings.TrimSpace(firstNonEmptyValue(item.MiddleURL, item.ThumbURL))
		thumbnailURL := strings.TrimSpace(firstNonEmptyValue(item.ThumbURL, item.MiddleURL))
		for _, replacement := range item.ReplaceURL {
			if pageURL == "" {
				pageURL = strings.TrimSpace(firstNonEmptyValue(replacement.FromURL, replacement.FromUrl))
			}
			if imageURL == "" {
				imageURL = strings.TrimSpace(firstNonEmptyValue(replacement.ObjURL, replacement.ObjUrl))
			}
		}
		if pageURL == "" && item.FromURLHost != "" {
			pageURL = "https://" + strings.TrimSpace(item.FromURLHost)
		}
		if imageURL == "" {
			imageURL = strings.TrimSpace(firstNonEmptyValue(item.ObjURL, item.MiddleURL, item.ThumbURL))
		}
		if imageURL == "" || pageURL == "" {
			continue
		}
		title := strings.TrimSpace(firstNonEmptyValue(
			fallbackPlainTextFromHTMLFragment(item.FromPageTitle),
			item.FromPageTitleEnc,
		))
		results = append(results, FallbackSearchResult{
			Title:           title,
			URL:             pageURL,
			ImageURL:        imageURL,
			ThumbnailURL:    thumbnailURL,
			Provider:        "Baidu Images",
			SourceNote:      "Search engine image result; license not verified",
			VerifiedLicense: false,
		})
		if len(results) >= maxResults {
			break
		}
	}
	return results, nil
}

func fallbackResultToSource(item FallbackSearchResult) *MediaFallbackSource {
	pageURL := strings.TrimSpace(item.URL)
	assetURL := strings.TrimSpace(item.ImageURL)
	if pageURL == "" && assetURL == "" {
		return nil
	}
	provider := strings.TrimSpace(item.Provider)
	if provider == "" {
		provider = fallbackSourceProviderLabel(firstNonEmptyValue(pageURL, assetURL))
	}
	note := strings.TrimSpace(item.SourceNote)
	if !item.VerifiedLicense && note == "" && strings.TrimSpace(item.License) == "" {
		note = "License not verified"
	}
	return &MediaFallbackSource{
		Provider:        provider,
		Title:           strings.TrimSpace(item.Title),
		PageURL:         pageURL,
		AssetURL:        assetURL,
		ThumbnailURL:    strings.TrimSpace(item.ThumbnailURL),
		Creator:         strings.TrimSpace(item.Creator),
		License:         strings.TrimSpace(item.License),
		LicenseURL:      strings.TrimSpace(item.LicenseURL),
		Note:            note,
		VerifiedLicense: item.VerifiedLicense,
	}
}

func fallbackSourceProviderLabel(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	host = strings.TrimPrefix(host, "www.")
	switch host {
	case "openverse.org", "api.openverse.org":
		return "Openverse"
	case "images.search.yahoo.com":
		return "Yahoo Images"
	case "image.baidu.com":
		return "Baidu Images"
	case "www.google.com", "google.com":
		return "Google Images"
	case "www.bing.com", "bing.com":
		return "Bing Images"
	default:
		return host
	}
}

func (e *FallbackEngine) searchOpenverseImages(ctx context.Context, query string, maxResults int) ([]FallbackSearchResult, error) {
	if maxResults <= 0 {
		return nil, nil
	}
	params := url.Values{}
	params.Set("q", query)
	params.Set("page_size", fmt.Sprintf("%d", max(3, min(maxResults*2, 10))))
	params.Set("license_type", "commercial")

	var payload struct {
		Results []struct {
			Title             string `json:"title"`
			Creator           string `json:"creator"`
			License           string `json:"license"`
			LicenseVersion    string `json:"license_version"`
			LicenseURL        string `json:"license_url"`
			ForeignLandingURL string `json:"foreign_landing_url"`
			Provider          string `json:"provider"`
			Source            string `json:"source"`
			Thumbnail         string `json:"thumbnail"`
			URL               string `json:"url"`
		} `json:"results"`
	}
	if err := e.fetchFallbackSourceJSON(ctx, "https://api.openverse.org/v1/images?"+params.Encode(), &payload); err != nil {
		return nil, err
	}
	results := make([]FallbackSearchResult, 0, min(maxResults, len(payload.Results)))
	for _, item := range payload.Results {
		imageURL := strings.TrimSpace(firstNonEmptyValue(item.Thumbnail, item.URL))
		if imageURL == "" {
			continue
		}
		results = append(results, FallbackSearchResult{
			Title:           strings.TrimSpace(item.Title),
			URL:             strings.TrimSpace(firstNonEmptyValue(item.ForeignLandingURL, item.URL)),
			ImageURL:        imageURL,
			ThumbnailURL:    strings.TrimSpace(item.Thumbnail),
			Provider:        fallbackOpenverseProviderLabel(item.Provider, item.Source),
			Creator:         strings.TrimSpace(item.Creator),
			License:         fallbackOpenverseLicenseLabel(item.License, item.LicenseVersion),
			LicenseURL:      strings.TrimSpace(item.LicenseURL),
			VerifiedLicense: strings.TrimSpace(item.License) != "" || strings.TrimSpace(item.LicenseURL) != "",
		})
		if len(results) >= maxResults {
			break
		}
	}
	return results, nil
}

func (e *FallbackEngine) fetchFallbackSourceJSON(ctx context.Context, rawURL string, target any) error {
	client := e.sourceClient
	if client == nil {
		client = e.httpClient
	}
	if client == nil {
		return fmt.Errorf("source client unavailable")
	}
	reqCtx, cancel := fallbackContextWithTimeout(ctx, fallbackSourceQueryTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; ZimaOS-Blue/1.0)")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, fallbackMaxHTMLBytes)).Decode(target)
}

func fallbackPlainTextFromHTMLFragment(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	doc, err := html.Parse(strings.NewReader("<div>" + raw + "</div>"))
	if err != nil {
		return strings.Join(strings.Fields(stdhtml.UnescapeString(raw)), " ")
	}
	var parts []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n == nil {
			return
		}
		if n.Type == html.TextNode {
			value := strings.TrimSpace(stdhtml.UnescapeString(n.Data))
			if value != "" {
				parts = append(parts, value)
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return strings.Join(strings.Fields(strings.Join(parts, " ")), " ")
}

func fallbackOpenverseProviderLabel(provider, source string) string {
	upstream := strings.TrimSpace(firstNonEmptyValue(source, provider))
	if upstream == "" {
		return "Openverse"
	}
	upstream = strings.NewReplacer("_", " ", "-", " ").Replace(upstream)
	upstream = strings.Join(strings.Fields(upstream), " ")
	if upstream == "" {
		return "Openverse"
	}
	return "Openverse / " + strings.ToUpper(upstream[:1]) + upstream[1:]
}

func fallbackOpenverseLicenseLabel(license, version string) string {
	license = strings.TrimSpace(strings.ToUpper(license))
	version = strings.TrimSpace(version)
	switch {
	case license == "" && version == "":
		return ""
	case version == "":
		return license
	default:
		return strings.TrimSpace(license + " " + version)
	}
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

  ctx.fillStyle = 'rgba(255,255,255,0.14)';
  roundRect(ctx, 64, 100, 540, 698, 28);
  ctx.fill();
  ctx.fillStyle = '#e2e8f0';
  ctx.font = 'bold 34px sans-serif';
  ctx.fillText('Prompt / search focus', 94, 150);

  ctx.font = '26px sans-serif';
  const promptLines = wrapText(promptText, 24);
  promptLines.forEach((line, idx) => {
    ctx.fillStyle = '#f8fafc';
    ctx.fillText(line, 94, 206 + idx * 40);
  });

  ctx.fillStyle = '#93c5fd';
  ctx.font = 'bold 24px sans-serif';
  ctx.fillText('Search cues', 94, 382);

  const keywordPills = keywords.slice(0, 6);
  let px = 94;
  let py = 420;
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
  ctx.fillText(images.length > 0 ? 'Reference images matched to the scene/background' : 'No usable image found, reference-only card rendered', 94, 632);

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
    ctx.fillText('Reference-only', frameX + 132, frameY + 280);
    ctx.font = '28px sans-serif';
    ctx.fillStyle = '#cbd5e1';
    ctx.fillText('Search results had no directly usable image.', frameX + 58, frameY + 340);
  }

  ctx.fillStyle = 'rgba(255,255,255,0.86)';
  ctx.font = '20px sans-serif';
  ctx.fillText('Web sources', 74, 840);
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
	_ = candidates
	_ = results
	width := e.config.ScreenshotWidth
	height := e.config.ScreenshotHeight
	if width <= 0 {
		width = 1280
	}
	if height <= 0 {
		height = 896
	}

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	fill := color.RGBA{R: 226, G: 232, B: 240, A: 255}
	border := color.RGBA{R: 203, G: 213, B: 225, A: 255}
	if looksCoolPrompt(req) {
		fill = color.RGBA{R: 219, G: 234, B: 254, A: 255}
		border = color.RGBA{R: 191, G: 219, B: 254, A: 255}
	}
	stdDraw.Draw(img, img.Bounds(), &image.Uniform{C: fill}, image.Point{}, stdDraw.Src)
	strokeRect(img, image.Rect(0, 0, width, height), border, 2)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func posterPromptPreview(prompt string) string {
	return trimToASCIIWithFallback(prompt, "See the chat card for the full prompt. This local fallback only shows reference imagery.")
}

func posterKeywordLabels(prompt string) []string {
	keywords := extractPosterKeywords(prompt)
	labels := make([]string, 0, len(keywords))
	seen := map[string]struct{}{}
	for _, keyword := range keywords {
		label := trimToASCIIWithFallback(keyword, "")
		if label == "" {
			continue
		}
		if _, ok := seen[label]; ok {
			continue
		}
		seen[label] = struct{}{}
		labels = append(labels, label)
		if len(labels) >= 5 {
			return labels
		}
	}
	if len(labels) > 0 {
		return labels
	}
	return []string{"scene refs", "background", "web search", "preview"}
}

func decodePosterCandidateImage(candidate fallbackImageCandidate) image.Image {
	_, payload, ok := parseDataURLPayload(candidate.DataURL)
	if !ok || len(payload) == 0 {
		return nil
	}
	img, _, err := image.Decode(bytes.NewReader(payload))
	if err != nil {
		return nil
	}
	return img
}

func posterSourceLabels(results []FallbackSearchResult) []string {
	labels := make([]string, 0, minInt(4, len(results)))
	for idx, item := range results {
		if len(labels) >= 4 {
			break
		}
		host := ""
		if parsed, err := url.Parse(strings.TrimSpace(item.URL)); err == nil {
			host = strings.TrimSpace(parsed.Host)
		}
		title := trimToASCIIWithFallback(item.Title, "")
		label := trimToASCIIWithFallback(firstNonEmptyValue(host, title), fmt.Sprintf("source %d", idx+1))
		if host != "" && title != "" && !strings.EqualFold(host, title) {
			label = truncateSentence(host+" - "+title, 72)
		}
		labels = append(labels, fmt.Sprintf("%d. %s", idx+1, label))
	}
	if len(labels) > 0 {
		return labels
	}
	return []string{"1. No source URLs were retained from search results."}
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
	timeout := 45 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining > 0 && remaining < timeout {
			timeout = remaining
		}
	}
	resp, err := e.browser().Screenshot(ctx, &browser.ScreenshotRequest{
		URL:             renderURL,
		Format:          browser.FormatPNG,
		Width:           e.config.ScreenshotWidth,
		Height:          e.config.ScreenshotHeight,
		WaitFor:         250,
		WaitForSelector: &readySelector,
		Timeout:         int(timeout.Milliseconds()),
	})
	if err != nil {
		return "", fmt.Errorf("fallback web_canvas screenshot stage: %w", err)
	}
	return resp.Data, nil
}

func fallbackContextWithTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return ctx, func() {}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, timeout)
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

func (e *FallbackEngine) hasAnySelector(ctx context.Context, svc FallbackBrowserService, targetID string, selectors []string) bool {
	for _, selector := range selectors {
		ok, err := svc.ElementExists(ctx, targetID, selector)
		if err == nil && ok {
			return true
		}
	}
	return false
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
	if s.engine == nil {
		return nil, fmt.Errorf("searcher unavailable")
	}

	appendUnique := func(dst []FallbackSearchResult, src []FallbackSearchResult, limit int) []FallbackSearchResult {
		seen := make(map[string]struct{}, len(dst)+len(src))
		for _, item := range dst {
			key := strings.TrimSpace(firstNonEmptyValue(item.ImageURL, item.URL, item.Title))
			if key == "" {
				continue
			}
			seen[key] = struct{}{}
		}
		for _, item := range src {
			key := strings.TrimSpace(firstNonEmptyValue(item.ImageURL, item.URL, item.Title))
			if key == "" {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			dst = append(dst, item)
			if len(dst) >= limit {
				break
			}
		}
		return dst
	}

	if maxResults <= 0 {
		maxResults = 3
	}
	results := make([]FallbackSearchResult, 0, maxResults)
	results = appendUnique(results, s.engine.searchLicensedReferenceAssets(ctx, query), maxResults)
	if len(results) < maxResults {
		results = appendUnique(results, s.engine.searchBrowserImageResults(ctx, query), maxResults)
	}
	if len(results) < maxResults {
		results = appendUnique(results, s.engine.searchSearchEngineImageResults(ctx, query), maxResults)
	}
	if len(results) < maxResults && s.engine.searcher != nil {
		webResults, err := s.engine.searcher.Search(ctx, query, maxResults-len(results), sanitizeFallbackImageSearchProviders(s.engine.config.SearchProviderChain))
		if err != nil && len(results) == 0 {
			return nil, err
		}
		results = appendUnique(results, filterFallbackResults(query, webResults), maxResults)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no search results")
	}
	normalized := make([]scenecompose.SearchResult, 0, len(results))
	for _, item := range results {
		normalized = append(normalized, scenecompose.SearchResult{
			Title:        item.Title,
			URL:          item.URL,
			Description:  item.Description,
			ImageURL:     item.ImageURL,
			ThumbnailURL: item.ThumbnailURL,
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
	candidateURLs := []string{
		strings.TrimSpace(result.ImageURL),
		strings.TrimSpace(result.URL),
		strings.TrimSpace(result.ThumbnailURL),
	}
	dedupedURLs := make([]string, 0, len(candidateURLs))
	seen := make(map[string]struct{}, len(candidateURLs))
	for _, raw := range candidateURLs {
		if raw == "" {
			continue
		}
		if _, ok := seen[raw]; ok {
			continue
		}
		seen[raw] = struct{}{}
		dedupedURLs = append(dedupedURLs, raw)
	}
	if len(dedupedURLs) == 0 {
		return nil, fmt.Errorf("search result has no resolvable url")
	}
	var (
		sourceURL   string
		contentType string
		img         image.Image
		lastErr     error
	)
	for idx, resolveURL := range dedupedURLs {
		resolvedSourceURL, payload, resolvedContentType, err := r.engine.resolveSearchImage(ctx, resolveURL)
		if err != nil {
			lastErr = err
			continue
		}
		decoded, _, decodeErr := image.Decode(bytes.NewReader(payload))
		if decodeErr != nil {
			lastErr = decodeErr
			continue
		}
		if idx > 0 {
			log.Printf(
				"[mediagen] fallback scene resolver fallback title=%q page=%q asset=%q",
				truncateFallbackLogValue(result.Title, 120),
				truncateFallbackLogValue(result.URL, 160),
				truncateFallbackLogValue(resolveURL, 160),
			)
		}
		sourceURL = resolvedSourceURL
		contentType = resolvedContentType
		img = decoded
		break
	}
	if img == nil {
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, fmt.Errorf("search result did not resolve to an image")
	}
	return &scenecompose.ResolvedImage{
		Title:       result.Title,
		PageURL:     firstNonEmptyValue(result.URL, sourceURL, dedupedURLs[0]),
		SourceURL:   firstNonEmptyValue(sourceURL, result.ImageURL, result.URL),
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
	case FallbackModelSpaceT2V,
		FallbackModelSpaceI2V,
		FallbackModelSpaceKF2V,
		FallbackModelNativeT2V,
		FallbackModelNativeI2V,
		FallbackModelNativeKF2V:
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
	case FallbackModelNativeT2V:
		return CategoryT2V
	case FallbackModelNativeI2V:
		return CategoryI2V
	case FallbackModelNativeKF2V:
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
	if len(info.Sources) > 0 {
		cp.Sources = append([]MediaFallbackSource(nil), info.Sources...)
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
		return "Reference Composition"
	case FallbackStrategyPublicSpace:
		return "Public Creative Space"
	case FallbackStrategyNativeVideo:
		return "Native Timeline Renderer"
	default:
		return "Fallback"
	}
}

func fallbackDisclosure(locale, strategy string) string {
	if isChineseLocale(locale) {
		switch strategy {
		case FallbackStrategyWebCanvas:
			return "未检测到可用媒体生成 API Key，当前结果不是按提示词新生成的图片，而是使用了内建的参考/占位预览兜底路径。下方会尽量披露参考素材来源与许可信息；若来自搜索引擎兜底，则会标注许可未核验。"
		case FallbackStrategyPublicSpace:
			return "未检测到可用媒体生成 API Key，已改用公开创意空间进行实验性、尽力而为的生成。"
		case FallbackStrategyNativeVideo:
			return "未检测到可用媒体生成 API Key，已改用本机时间轴渲染生成视频。"
		default:
			return "未检测到可用媒体生成 API Key，已使用内建降级能力。"
		}
	}
	switch strategy {
	case FallbackStrategyWebCanvas:
		return "No configured media API key was available, so this result used a built-in reference or placeholder preview path instead of a newly generated AI image. Source and license details are shown below when available; search-engine fallbacks are marked as unverified."
	case FallbackStrategyPublicSpace:
		return "No configured media API key was available, so the request used a public creative space on a best-effort experimental basis."
	case FallbackStrategyNativeVideo:
		return "No configured media API key was available, so the request used the local native timeline renderer."
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
