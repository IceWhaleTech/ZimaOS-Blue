package mediagen

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/scenecompose"
	basetask "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/task"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tts"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/videogen"
	"github.com/labstack/echo/v4"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

type stubFallbackSearcher struct {
	results []FallbackSearchResult
	err     error
	search  func(ctx context.Context, query string, maxResults int, providers []string) ([]FallbackSearchResult, error)
}

func (s stubFallbackSearcher) Search(ctx context.Context, query string, maxResults int, providers []string) ([]FallbackSearchResult, error) {
	if s.search != nil {
		return s.search(ctx, query, maxResults, providers)
	}
	if s.err != nil {
		return nil, s.err
	}
	return append([]FallbackSearchResult(nil), s.results...), nil
}

type stubFallbackBrowser struct {
	screenshot func(ctx context.Context, req *browser.ScreenshotRequest) (*browser.ScreenshotResponse, error)
	scrape     func(ctx context.Context, req *browser.ScrapeRequest) (*browser.ScrapeResponse, error)
	openTab    func(ctx context.Context, url string) (*browser.Tab, error)
	closeTab   func(ctx context.Context, targetID string) error
	exists     func(ctx context.Context, targetID, selector string) (bool, error)
	extract    func(ctx context.Context, targetID, selector, attribute string) (string, error)
	act        func(ctx context.Context, req *browser.ActRequest) (*browser.ActResponse, error)
	pageInfo   func(ctx context.Context, targetID string) (string, string, error)
}

type stubNativeVideoGenerator struct {
	available bool
	generate  func(ctx context.Context, job *videogen.Job, pollInterval, stallTimeout, maxRuntime time.Duration, observer videogen.ProgressObserver) (*videogen.Result, error)
}

type stubSceneComposeSearcher struct {
	results map[string][]scenecompose.SearchResult
}

func (s stubSceneComposeSearcher) Search(_ context.Context, query string, _ int) ([]scenecompose.SearchResult, error) {
	return append([]scenecompose.SearchResult(nil), s.results[query]...), nil
}

type stubSceneComposeResolver struct {
	images map[string]*scenecompose.ResolvedImage
}

func (r stubSceneComposeResolver) Resolve(_ context.Context, result scenecompose.SearchResult) (*scenecompose.ResolvedImage, error) {
	return r.images[result.URL], nil
}

type stubScenePlannerLLM struct {
	content string
	err     error
	chat    func(req llm.ChatRequest) (*llm.ChatResponse, error)
}

type stubFallbackVLM struct {
	chat func(ctx context.Context, prompt string, imageBase64 string) (string, error)
}

func (s stubScenePlannerLLM) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if s.chat != nil {
		return s.chat(req)
	}
	if s.err != nil {
		return nil, s.err
	}
	return &llm.ChatResponse{
		Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: s.content,
		},
	}, nil
}

func (s stubNativeVideoGenerator) Status() videogen.HelperStatus {
	if s.available {
		return videogen.HelperStatus{Available: true, Mode: "test", Path: "stub"}
	}
	return videogen.HelperStatus{Available: false, Error: "unavailable"}
}

func (s stubNativeVideoGenerator) Available() bool { return s.available }

func (s stubNativeVideoGenerator) Generate(ctx context.Context, job *videogen.Job, pollInterval, stallTimeout, maxRuntime time.Duration, observer videogen.ProgressObserver) (*videogen.Result, error) {
	if s.generate != nil {
		return s.generate(ctx, job, pollInterval, stallTimeout, maxRuntime, observer)
	}
	return nil, io.EOF
}

type stubTTSService struct {
	synthesize func(ctx context.Context, req *tts.SynthesizeRequest) (*tts.SynthesizeResponse, error)
}

func (s stubTTSService) Synthesize(ctx context.Context, req *tts.SynthesizeRequest) (*tts.SynthesizeResponse, error) {
	if s.synthesize != nil {
		return s.synthesize(ctx, req)
	}
	return nil, tts.ErrNoProviderConfigured
}

func (s stubTTSService) SynthesizeWithProvider(ctx context.Context, _ tts.ProviderType, req *tts.SynthesizeRequest) (*tts.SynthesizeResponse, error) {
	return s.Synthesize(ctx, req)
}

func (s stubTTSService) SynthesizeStream(_ context.Context, _ *tts.SynthesizeRequest, _ tts.StreamCallback) error {
	return tts.ErrNoProviderConfigured
}

func (s stubTTSService) ListVoices(context.Context) ([]tts.Voice, error) { return nil, nil }
func (s stubTTSService) ListProviders() []tts.ProviderType               { return nil }
func (s stubTTSService) GetDefaultProvider() tts.ProviderType            { return "" }
func (s stubTTSService) SetDefaultProvider(tts.ProviderType) error       { return nil }
func (s stubTTSService) GetProvider(tts.ProviderType) tts.Provider       { return nil }
func (s stubTTSService) GetConfig() (float32, float32, float32)          { return 1, 0, 100 }
func (s stubTTSService) SetConfig(float32, float32, float32)             {}
func (s stubTTSService) GetVocoderStatus() map[string]interface{}        { return nil }
func (s stubTTSService) DownloadVocoderModel(context.Context) error      { return nil }
func (s stubTTSService) CancelVocoderDownload()                          {}
func (s stubTTSService) GetKokoroStatus() map[string]interface{}         { return nil }
func (s stubTTSService) DownloadKokoroModel(context.Context) error       { return nil }
func (s stubTTSService) CancelKokoroDownload()                           {}
func (s stubTTSService) Close()                                          {}

func (s stubFallbackBrowser) Start(context.Context) error { return nil }
func (s stubFallbackBrowser) OpenTab(ctx context.Context, url string) (*browser.Tab, error) {
	if s.openTab != nil {
		return s.openTab(ctx, url)
	}
	return &browser.Tab{TargetID: "tab-1", URL: url, Title: url, Active: true}, nil
}
func (s stubFallbackBrowser) CloseTab(ctx context.Context, targetID string) error {
	if s.closeTab != nil {
		return s.closeTab(ctx, targetID)
	}
	return nil
}
func (s stubFallbackBrowser) Screenshot(ctx context.Context, req *browser.ScreenshotRequest) (*browser.ScreenshotResponse, error) {
	if s.screenshot != nil {
		return s.screenshot(ctx, req)
	}
	return &browser.ScreenshotResponse{Data: fakeMediaImagePNGBase64, Format: browser.FormatPNG}, nil
}
func (s stubFallbackBrowser) Scrape(ctx context.Context, req *browser.ScrapeRequest) (*browser.ScrapeResponse, error) {
	if s.scrape != nil {
		return s.scrape(ctx, req)
	}
	return &browser.ScrapeResponse{Data: map[string]interface{}{}}, nil
}
func (s stubFallbackBrowser) ElementExists(ctx context.Context, targetID, selector string) (bool, error) {
	if s.exists != nil {
		return s.exists(ctx, targetID, selector)
	}
	return false, nil
}
func (s stubFallbackBrowser) ExtractFirstFromTab(ctx context.Context, targetID, selector, attribute string) (string, error) {
	if s.extract != nil {
		return s.extract(ctx, targetID, selector, attribute)
	}
	return "", nil
}
func (s stubFallbackBrowser) Act(ctx context.Context, req *browser.ActRequest) (*browser.ActResponse, error) {
	if s.act != nil {
		return s.act(ctx, req)
	}
	return &browser.ActResponse{Success: true}, nil
}
func (s stubFallbackBrowser) PageInfo(ctx context.Context, targetID string) (string, string, error) {
	if s.pageInfo != nil {
		return s.pageInfo(ctx, targetID)
	}
	return "", "", nil
}

func newFallbackEngineForTest(t *testing.T, storage *MediaStorage, searcher FallbackSearcher, browserSvc FallbackBrowserService, cfg FallbackConfig) *FallbackEngine {
	t.Helper()
	cfg.Enabled = true
	if cfg.RenderBaseURL == "" {
		cfg.RenderBaseURL = "http://127.0.0.1:7777"
	}
	engine := NewFallbackEngine(cfg, storage, searcher, func() FallbackBrowserService { return browserSvc }, "zh-CN")
	engine.sourceClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			_ = req
			return nil, errors.New("source search disabled in tests")
		}),
	}
	return engine
}

func TestHandlerListModelsIncludesFallbackWhenNoProvider(t *testing.T) {
	manager := NewManager(nil, nil, "")
	manager.SetFallbackEngine(NewFallbackEngine(FallbackConfig{Enabled: true}, nil, nil, func() FallbackBrowserService { return nil }, ""))
	handler := NewHandler(manager, nil, "")
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/media/models", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.ListModels(c); err != nil {
		t.Fatalf("ListModels returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body struct {
		Data []MediaModelInfo `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(body.Data) == 0 {
		t.Fatal("expected fallback models to be returned")
	}
	foundWebCanvas := false
	foundVideo := false
	for _, model := range body.Data {
		if model.ID == FallbackModelWebCanvasT2I && model.IsFallback {
			foundWebCanvas = true
		}
		if model.ID == FallbackModelSpaceT2V && model.IsFallback {
			foundVideo = true
		}
	}
	if !foundWebCanvas || !foundVideo {
		t.Fatalf("models = %#v, want fallback web canvas + video entries", body.Data)
	}
}

func (s stubFallbackVLM) ChatWithVision(ctx context.Context, prompt string, imageBase64 string) (string, error) {
	if s.chat != nil {
		return s.chat(ctx, prompt, imageBase64)
	}
	return `{"selected_index":1,"score":100,"reason":"default"}`, nil
}

func TestHandlerClassifyIntentReturnsFallbackModels(t *testing.T) {
	manager := NewManager(nil, nil, "")
	manager.SetFallbackEngine(NewFallbackEngine(FallbackConfig{Enabled: true}, nil, nil, func() FallbackBrowserService { return nil }, "zh-CN"))
	handler := NewHandler(manager, nil, "zh-CN")
	e := echo.New()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/media/classify", bytes.NewBufferString(`{"message":"生成一张猫的图片"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.ClassifyIntent(c); err != nil {
		t.Fatalf("ClassifyIntent returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body struct {
		Intent *MediaIntent     `json:"intent"`
		Models []MediaModelInfo `json:"models"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Intent == nil || body.Intent.Category != CategoryT2I {
		t.Fatalf("intent = %#v, want t2i", body.Intent)
	}
	if len(body.Models) == 0 || body.Models[0].ID != FallbackModelWebCanvasT2I {
		t.Fatalf("models = %#v, want fallback t2i models", body.Models)
	}
}

func TestManagerGenerateUsesWebCanvasFallback(t *testing.T) {
	tmp := t.TempDir()
	storage := NewMediaStorage(tmp, "/api/media/generated")
	if err := storage.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	_, imgData, _ := parseDataURLPayload("data:image/png;base64," + fakeMediaImagePNGBase64)
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/article":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<html><head><meta property="og:image" content="/img.png"></head><body>ok</body></html>`))
		case "/img.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(imgData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer imageServer.Close()

	manager := NewManager(storage, nil, "zh-CN")
	engine := newFallbackEngineForTest(t, storage, stubFallbackSearcher{
		results: []FallbackSearchResult{{Title: "Cat Article", URL: imageServer.URL + "/article"}},
	}, nil, FallbackConfig{RenderBaseURL: "http://127.0.0.1:8899"})
	manager.SetFallbackEngine(engine)

	task, err := manager.Generate(context.Background(), &MediaRequest{
		Type:   MediaTypeImage,
		Prompt: "draw a playful cat poster",
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	waitCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	task, err = manager.WaitForTask(waitCtx, task.ID)
	if err != nil {
		t.Fatalf("WaitForTask returned error: %v", err)
	}
	if task.FallbackInfo == nil || task.FallbackInfo.Strategy != FallbackStrategyWebCanvas {
		t.Fatalf("fallback_info = %#v, want web_canvas", task.FallbackInfo)
	}
	if len(task.Response.Data) == 0 || !strings.HasPrefix(task.Response.Data[0].URL, "/api/media/generated/images/") {
		t.Fatalf("response = %#v, want locally stored image", task.Response)
	}
}

func TestFallbackSearchPromptSanitizesPrefixAndAddsStructuredQueries(t *testing.T) {
	engine := NewFallbackEngine(FallbackConfig{
		Enabled:          true,
		SearchMaxResults: 6,
	}, nil, stubFallbackSearcher{
		search: func(_ context.Context, query string, _ int, _ []string) ([]FallbackSearchResult, error) {
			return nil, nil
		},
	}, func() FallbackBrowserService { return nil }, "zh-CN")

	var queries []string
	engine.searcher = stubFallbackSearcher{
		search: func(_ context.Context, query string, _ int, _ []string) ([]FallbackSearchResult, error) {
			queries = append(queries, query)
			return nil, nil
		},
	}

	rawPrompt := "帮我生成一张泰迪在雪地里玩耍的照片"
	if _, err := engine.searchPrompt(context.Background(), rawPrompt); err != nil {
		t.Fatalf("searchPrompt returned error: %v", err)
	}
	if len(queries) < 4 {
		t.Fatalf("queries = %v, want sanitized prompt + scene + foreground + background query", queries)
	}
	if queries[0] != "泰迪在雪地里玩耍的照片" {
		t.Fatalf("first query = %q, want sanitized prompt first", queries[0])
	}
	if !strings.Contains(strings.Join(queries, "\n"), "泰迪") || !strings.Contains(strings.Join(queries, "\n"), "雪地") {
		t.Fatalf("queries = %v, want structured teddy + snow retrieval cues", queries)
	}
	hasForegroundQuery := false
	hasBackgroundQuery := false
	for _, query := range queries {
		if strings.Contains(query, "泰迪") && !strings.Contains(query, "背景") && query != queries[0] {
			hasForegroundQuery = true
		}
		if strings.Contains(query, "雪地") && strings.Contains(query, "背景") {
			hasBackgroundQuery = true
		}
	}
	if !hasForegroundQuery {
		t.Fatalf("queries = %v, want isolated subject supplemental query", queries)
	}
	if !hasBackgroundQuery {
		t.Fatalf("queries = %v, want snow background supplemental query", queries)
	}
	if containsString(queries, rawPrompt) {
		t.Fatalf("queries = %v, want raw photo prompt removed from fallback search list", queries)
	}
}

func TestFallbackSearchPromptPrefersPlannerSearchHintsWhenAvailable(t *testing.T) {
	engine := NewFallbackEngine(FallbackConfig{
		Enabled:          true,
		SearchMaxResults: 6,
	}, nil, stubFallbackSearcher{
		search: func(_ context.Context, query string, _ int, _ []string) ([]FallbackSearchResult, error) {
			return nil, nil
		},
	}, func() FallbackBrowserService { return nil }, "zh-CN")
	engine.SetScenePlannerLLM(stubScenePlannerLLM{content: `{
		"background": "雪地",
		"style": "realistic",
		"lighting": "soft natural light",
		"time_of_day": "day",
		"weather": "snowy",
		"camera_view": "eye-level",
		"scene_query": "泰迪犬 雪地 玩耍 写实照片",
		"background_query": "雪地 冬季 户外 背景 照片",
		"foreground": [{
			"id": "dog_1",
			"type": "dog",
			"search_query": "泰迪犬 奔跑 isolated png transparent",
			"fallback_query": "泰迪犬 奔跑 写实照片",
			"priority": 1,
			"layout": {
				"horizontal": "center",
				"vertical": "low",
				"depth": "midground",
				"scale": "medium",
				"grounded": true
			}
		}]
	}`})

	var queries []string
	engine.searcher = stubFallbackSearcher{
		search: func(_ context.Context, query string, _ int, _ []string) ([]FallbackSearchResult, error) {
			queries = append(queries, query)
			return nil, nil
		},
	}

	rawPrompt := "帮我生成一张泰迪在雪地里玩耍的照片"
	if _, err := engine.searchPrompt(context.Background(), rawPrompt); err != nil {
		t.Fatalf("searchPrompt returned error: %v", err)
	}
	if len(queries) < 4 {
		t.Fatalf("queries = %v, want planner hint + semantic + foreground + background", queries)
	}
	if queries[0] != "泰迪犬 雪地 玩耍 写实照片" {
		t.Fatalf("first query = %q, want planner scene query first", queries[0])
	}
	if queries[1] != "泰迪在雪地里玩耍的照片" {
		t.Fatalf("second query = %q, want sanitized semantic prompt second", queries[1])
	}
	if queries[2] != "泰迪犬 奔跑 写实照片" {
		t.Fatalf("third query = %q, want planner foreground query third", queries[2])
	}
	if queries[3] != "雪地 冬季 户外 背景 照片" {
		t.Fatalf("fourth query = %q, want planner background query fourth", queries[3])
	}
	if containsString(queries, rawPrompt) {
		t.Fatalf("queries = %v, want raw photo prompt omitted when planner hints are available", queries)
	}
}

func TestFallbackSearchPromptAppendsPlannerEnglishSearchHintsWhenAvailable(t *testing.T) {
	engine := NewFallbackEngine(FallbackConfig{
		Enabled:          true,
		SearchMaxResults: 8,
	}, nil, stubFallbackSearcher{
		search: func(_ context.Context, query string, _ int, _ []string) ([]FallbackSearchResult, error) {
			return nil, nil
		},
	}, func() FallbackBrowserService { return nil }, "zh-CN")
	engine.SetScenePlannerLLM(stubScenePlannerLLM{content: `{
		"background": "雪地",
		"style": "realistic",
		"lighting": "soft natural light",
		"time_of_day": "day",
		"weather": "snowy",
		"camera_view": "eye-level",
		"scene_query": "泰迪犬 雪地 玩耍 写实照片",
		"scene_query_en": "toy poodle playing in snow realistic photo",
		"background_query": "雪地 冬季 户外 背景 照片",
		"background_query_en": "snowy winter outdoor background landscape photo",
		"foreground": [{
			"id": "dog_1",
			"type": "dog",
			"search_query": "泰迪犬 奔跑 isolated png transparent",
			"search_query_en": "toy poodle running isolated png transparent",
			"fallback_query": "泰迪犬 奔跑 写实照片",
			"fallback_query_en": "toy poodle running realistic photo",
			"priority": 1,
			"layout": {
				"horizontal": "center",
				"vertical": "low",
				"depth": "midground",
				"scale": "medium",
				"grounded": true
			}
		}]
	}`})

	var queries []string
	engine.searcher = stubFallbackSearcher{
		search: func(_ context.Context, query string, _ int, _ []string) ([]FallbackSearchResult, error) {
			queries = append(queries, query)
			return nil, nil
		},
	}

	if _, err := engine.searchPrompt(context.Background(), "帮我生成一张泰迪在雪地里玩耍的照片"); err != nil {
		t.Fatalf("searchPrompt returned error: %v", err)
	}
	if len(queries) < 7 {
		t.Fatalf("queries = %v, want localized and English planner hints", queries)
	}
	if queries[0] != "泰迪犬 雪地 玩耍 写实照片" {
		t.Fatalf("first query = %q, want localized planner scene query first", queries[0])
	}
	if queries[1] != "toy poodle playing in snow realistic photo" {
		t.Fatalf("second query = %q, want English planner scene query second", queries[1])
	}
	if !containsString(queries, "toy poodle running realistic photo") {
		t.Fatalf("queries = %v, want English foreground fallback query", queries)
	}
	if !containsString(queries, "snowy winter outdoor background landscape photo") {
		t.Fatalf("queries = %v, want English background query", queries)
	}
}

func TestFallbackSearchPromptFiltersLowValuePhotoResultsAndContinuesToForegroundQuery(t *testing.T) {
	engine := NewFallbackEngine(FallbackConfig{
		Enabled:          true,
		SearchMaxResults: 6,
	}, nil, stubFallbackSearcher{
		search: func(_ context.Context, query string, _ int, _ []string) ([]FallbackSearchResult, error) {
			switch query {
			case "泰迪在雪地里玩耍的照片":
				return []FallbackSearchResult{
					{Title: "泰的意思 - 百度知道", URL: "https://zhidao.baidu.com/question/2213756133395366308.html"},
				}, nil
			case "泰迪 写实照片":
				return []FallbackSearchResult{
					{Title: "雪地中的泰迪犬正版高清图片下载-视觉中国vcg.com", URL: "https://www.vcg.com/creative/1396445079.html"},
				}, nil
			default:
				return nil, nil
			}
		},
	}, func() FallbackBrowserService { return nil }, "zh-CN")

	results, err := engine.searchPrompt(context.Background(), "帮我生成一张泰迪在雪地里玩耍的照片")
	if err != nil {
		t.Fatalf("searchPrompt returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %v, want one filtered foreground result", results)
	}
	if results[0].URL != "https://www.vcg.com/creative/1396445079.html" {
		t.Fatalf("result url = %q, want filtered foreground image source", results[0].URL)
	}
}

func TestFilterFallbackResultsDropsBlockedSourceHosts(t *testing.T) {
	results := filterFallbackResults("泰迪在雪地里玩耍", []FallbackSearchResult{
		{
			Title: "Toy poodle in snow",
			URL:   "https://commons.wikimedia.org/wiki/File:Toy_poodle_in_snow.jpg",
		},
		{
			Title: "Toy poodle in snow",
			URL:   "https://example.com/teddy-in-snow",
		},
	})

	if len(results) != 1 {
		t.Fatalf("results = %#v, want one non-blocked result", results)
	}
	if results[0].URL != "https://example.com/teddy-in-snow" {
		t.Fatalf("result url = %q, want non-blocked host", results[0].URL)
	}
}

func TestFilterFallbackResultsDropsDictionaryAndForumNoiseForPhotoPrompt(t *testing.T) {
	results := filterFallbackResults("帮我生成一张泰迪在雪地里玩耍的照片", []FallbackSearchResult{
		{
			Title: "资源 - 雪地奔驰_汉化版下载_攻略秘籍_中文版下载_3DM论坛 ...",
			URL:   "https://bbs.3dmgame.com/forum.php?mod=forumdisplay&fid=3209&filter=typeid&typeid=42548",
		},
		{
			Title: "帮 的意思, 帮 的解释, 帮 的拼音, 帮 的部首, 帮 的笔顺-汉语国学",
			URL:   "https://www.hanyuguoxue.com/zidian/zi-24110",
		},
		{
			Title: "雪地中的泰迪犬正版高清图片下载-视觉中国vcg.com",
			URL:   "https://www.vcg.com/creative/1396445079.html",
		},
	})

	if len(results) != 1 {
		t.Fatalf("results = %#v, want only the photo-relevant result", results)
	}
	if results[0].URL != "https://www.vcg.com/creative/1396445079.html" {
		t.Fatalf("result url = %q, want relevant teddy-in-snow photo result", results[0].URL)
	}
}

func TestFilterFallbackResultsKeepsIntentSelectedPhotoResult(t *testing.T) {
	prompt := "帮我生成一张泰迪在雪地里玩耍的照片"
	result := FallbackSearchResult{
		Title:    "Generated image",
		URL:      "https://www.craiyon.com/en/images/abc123",
		ImageURL: "https://img.craiyon.com/abc123.png",
		Provider: "Craiyon Search",
	}

	filtered := filterFallbackResults(prompt, []FallbackSearchResult{result})
	if len(filtered) != 0 {
		t.Fatalf("filtered = %#v, want generic browser result dropped without intent selection", filtered)
	}

	result.IntentSelected = true
	filtered = filterFallbackResults(prompt, []FallbackSearchResult{result})
	if len(filtered) != 1 {
		t.Fatalf("filtered = %#v, want intent-selected browser result preserved", filtered)
	}
	if !filtered[0].IntentSelected {
		t.Fatalf("intent_selected = %v, want true", filtered[0].IntentSelected)
	}
}

func TestGenerateWebCanvasPhotoPromptUsesSanitizedSearchPrompt(t *testing.T) {
	tmp := t.TempDir()
	storage := NewMediaStorage(tmp, "/api/media/generated")
	if err := storage.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	_, imgData, _ := parseDataURLPayload("data:image/png;base64," + fakeMediaImagePNGBase64)
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/snow":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<html><head><meta property="og:image" content="/snow.png"></head><body>snow</body></html>`))
		case "/snow.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(imgData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer imageServer.Close()

	var queries []string
	engine := newFallbackEngineForTest(t, storage, stubFallbackSearcher{
		search: func(_ context.Context, query string, _ int, _ []string) ([]FallbackSearchResult, error) {
			queries = append(queries, query)
			if query == "泰迪在雪地里玩耍的照片" {
				return []FallbackSearchResult{{Title: "Toy poodle playing in snow", URL: imageServer.URL + "/snow"}}, nil
			}
			return nil, nil
		},
	}, stubFallbackBrowser{}, FallbackConfig{RenderBaseURL: "http://127.0.0.1:8899"})

	task, err := engine.generateWebCanvas(context.Background(), &MediaRequest{
		Type:   MediaTypeImage,
		Prompt: "帮我生成一张泰迪在雪地里玩耍的照片",
	})
	if err != nil {
		t.Fatalf("generateWebCanvas returned error: %v", err)
	}
	if len(queries) == 0 || queries[0] != "泰迪在雪地里玩耍的照片" {
		t.Fatalf("queries = %v, want sanitized photo prompt first", queries)
	}
	if task == nil || task.Response == nil || len(task.Response.Data) == 0 {
		t.Fatalf("task = %#v, want direct reference result", task)
	}
	if !strings.HasPrefix(task.Response.Data[0].URL, "/api/media/generated/images/") {
		t.Fatalf("result url = %q, want stored direct image", task.Response.Data[0].URL)
	}
}

func TestRenderSceneComposeUsesSanitizedPromptForPlanner(t *testing.T) {
	dogURL := newCompositeAssetURL(t, "dog")
	bgURL := newCompositeAssetURL(t, "beach")

	engine := NewFallbackEngine(FallbackConfig{Enabled: true}, nil, stubFallbackSearcher{
		search: func(_ context.Context, query string, _ int, _ []string) ([]FallbackSearchResult, error) {
			switch {
			case strings.Contains(query, "背景"):
				return []FallbackSearchResult{{Title: "Snowy field background photo", URL: bgURL}}, nil
			case strings.Contains(query, "png") || strings.Contains(query, "transparent") || strings.Contains(query, "照片") || strings.Contains(query, "photo"):
				return []FallbackSearchResult{{Title: "Toy poodle photo", URL: dogURL}}, nil
			default:
				return nil, nil
			}
		},
	}, func() FallbackBrowserService { return nil }, "zh-CN")

	var userPrompt string
	engine.SetScenePlannerLLM(stubScenePlannerLLM{
		chat: func(req llm.ChatRequest) (*llm.ChatResponse, error) {
			if userPrompt == "" && len(req.Messages) >= 2 {
				userPrompt = req.Messages[1].Content
			}
			return &llm.ChatResponse{
				Message: llm.Message{
					Role: llm.RoleAssistant,
					Content: `{
						"background": "雪地",
						"style": "realistic",
						"lighting": "soft natural light",
						"time_of_day": "day",
						"weather": "snowy",
						"camera_view": "eye-level",
						"background_query": "雪地 背景 照片",
						"foreground": [{
							"id": "dog_1",
							"type": "dog",
							"search_query": "泰迪 透明背景 png",
							"fallback_query": "泰迪 照片",
							"priority": 1,
							"layout": {
								"horizontal": "center",
								"vertical": "low",
								"depth": "midground",
								"scale": "medium",
								"grounded": true
							}
						}]
					}`,
				},
			}, nil
		},
	})

	if _, _, err := engine.renderSceneCompose(context.Background(), &MediaRequest{
		Type:   MediaTypeImage,
		Prompt: "帮我生成一张泰迪在雪地里玩耍的照片",
	}); err != nil {
		t.Fatalf("renderSceneCompose returned error: %v", err)
	}
	if strings.Contains(userPrompt, "帮我生成一张") {
		t.Fatalf("planner prompt = %q, want sanitized request prompt", userPrompt)
	}
	if !strings.Contains(userPrompt, "泰迪在雪地里玩耍的照片") {
		t.Fatalf("planner prompt = %q, want sanitized semantic prompt", userPrompt)
	}
}

func TestFallbackSceneResolverPrefersDirectImageURL(t *testing.T) {
	_, imgData, _ := parseDataURLPayload("data:image/png;base64," + fakeMediaImagePNGBase64)
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(imgData)
	}))
	defer imageServer.Close()

	engine := newFallbackEngineForTest(t, nil, stubFallbackSearcher{}, stubFallbackBrowser{}, FallbackConfig{})
	resolved, err := (fallbackSceneResolver{engine: engine}).Resolve(context.Background(), scenecompose.SearchResult{
		Title:    "Snowy field",
		URL:      "https://example.com/page",
		ImageURL: imageServer.URL + "/snow.png",
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved == nil || resolved.Image == nil {
		t.Fatalf("resolved = %#v, want decoded image", resolved)
	}
	if resolved.PageURL != "https://example.com/page" {
		t.Fatalf("page_url = %q, want original page url", resolved.PageURL)
	}
	if resolved.SourceURL != imageServer.URL+"/snow.png" {
		t.Fatalf("source_url = %q, want direct image url", resolved.SourceURL)
	}
}

func TestFallbackSceneResolverFallsBackToThumbnailURL(t *testing.T) {
	_, imgData, _ := parseDataURLPayload("data:image/png;base64," + fakeMediaImagePNGBase64)
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/primary.png":
			http.Error(w, "blocked", http.StatusForbidden)
		case "/thumb.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(imgData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer imageServer.Close()

	engine := newFallbackEngineForTest(t, nil, stubFallbackSearcher{}, stubFallbackBrowser{}, FallbackConfig{})
	resolved, err := (fallbackSceneResolver{engine: engine}).Resolve(context.Background(), scenecompose.SearchResult{
		Title:        "Toy poodle isolated",
		URL:          "https://example.com/page",
		ImageURL:     imageServer.URL + "/primary.png",
		ThumbnailURL: imageServer.URL + "/thumb.png",
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved == nil || resolved.Image == nil {
		t.Fatalf("resolved = %#v, want decoded image", resolved)
	}
	if resolved.PageURL != "https://example.com/page" {
		t.Fatalf("page_url = %q, want original page url", resolved.PageURL)
	}
	if resolved.SourceURL != imageServer.URL+"/thumb.png" {
		t.Fatalf("source_url = %q, want thumbnail fallback url", resolved.SourceURL)
	}
}

func TestGenerateWebCanvasPhotoPromptPrefersSceneComposeOverDirectReference(t *testing.T) {
	tmp := t.TempDir()
	storage := NewMediaStorage(tmp, "/api/media/generated")
	if err := storage.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	bgURL := newCompositeAssetURL(t, "beach")
	dogURL := newCompositeAssetURL(t, "dog")

	engine := newFallbackEngineForTest(t, storage, stubFallbackSearcher{
		search: func(_ context.Context, query string, _ int, _ []string) ([]FallbackSearchResult, error) {
			normalized := strings.TrimSpace(strings.ToLower(query))
			switch {
			case query == "泰迪在雪地里玩耍的照片":
				return []FallbackSearchResult{{Title: "Toy poodle playing in snow", URL: dogURL}}, nil
			case strings.Contains(normalized, "雪地") && strings.Contains(normalized, "背景"):
				return []FallbackSearchResult{{Title: "Snowy snow field background photo", URL: bgURL}}, nil
			case strings.Contains(normalized, "泰迪") && (strings.Contains(normalized, "transparent") || strings.Contains(normalized, "png") || strings.Contains(normalized, "照片")):
				return []FallbackSearchResult{{Title: "Toy poodle", URL: dogURL}}, nil
			default:
				return nil, nil
			}
		},
	}, stubFallbackBrowser{}, FallbackConfig{
		RenderBaseURL: "http://127.0.0.1:8899",
	})

	task, err := engine.generateWebCanvas(context.Background(), &MediaRequest{
		Type:   MediaTypeImage,
		Prompt: "帮我生成一张泰迪在雪地里玩耍的照片",
	})
	if err != nil {
		t.Fatalf("generateWebCanvas returned error: %v", err)
	}
	if task == nil || task.Response == nil || len(task.Response.Data) == 0 {
		t.Fatalf("task = %#v, want result", task)
	}
	result := task.Response.Data[0]
	if result.B64JSON == "" {
		t.Fatalf("result = %#v, want composed inline image before direct reference fallback", result)
	}
	if len(task.FallbackInfo.SourceURLs) == 0 {
		t.Fatalf("fallback_info = %#v, want composed source disclosures", task.FallbackInfo)
	}
	if !containsString(task.FallbackInfo.SourceURLs, bgURL) {
		t.Fatalf("source_urls = %#v, want background source URL", task.FallbackInfo.SourceURLs)
	}
	if !containsString(task.FallbackInfo.SourceURLs, dogURL) {
		t.Fatalf("source_urls = %#v, want foreground source URL", task.FallbackInfo.SourceURLs)
	}
}

func TestFallbackDisplayNameUsesReferenceCompositionForWebCanvas(t *testing.T) {
	if got := fallbackDisplayName(FallbackStrategyWebCanvas); got != "Reference Composition" {
		t.Fatalf("fallbackDisplayName(web_canvas) = %q, want %q", got, "Reference Composition")
	}
}

func TestGenerateWebCanvasPrefersLocalPosterBeforeBrowserCapture(t *testing.T) {
	tmp := t.TempDir()
	storage := NewMediaStorage(tmp, "/api/media/generated")
	if err := storage.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	screenshotCalls := 0
	engine := newFallbackEngineForTest(t, storage, stubFallbackSearcher{}, stubFallbackBrowser{
		screenshot: func(_ context.Context, req *browser.ScreenshotRequest) (*browser.ScreenshotResponse, error) {
			screenshotCalls++
			return &browser.ScreenshotResponse{Data: fakeMediaImagePNGBase64, Format: browser.FormatPNG}, nil
		},
	}, FallbackConfig{RenderBaseURL: "http://127.0.0.1:8899"})

	task, err := engine.generateWebCanvas(context.Background(), &MediaRequest{
		Type:   MediaTypeImage,
		Prompt: "teddy playing in snow",
	})
	if err != nil {
		t.Fatalf("generateWebCanvas returned error: %v", err)
	}
	if screenshotCalls != 0 {
		t.Fatalf("screenshotCalls = %d, want 0 when local poster render succeeds", screenshotCalls)
	}
	if task == nil || task.Response == nil || len(task.Response.Data) == 0 || task.Response.Data[0].B64JSON == "" {
		t.Fatalf("task = %#v, want inline png result", task)
	}
}

func TestGenerateWebCanvasReturnsDirectReferenceImageForPhotoPrompt(t *testing.T) {
	tmp := t.TempDir()
	storage := NewMediaStorage(tmp, "/api/media/generated")
	if err := storage.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	_, imgData, _ := parseDataURLPayload("data:image/png;base64," + fakeMediaImagePNGBase64)
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/snow":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<html><head><meta property="og:image" content="/snow.png"></head><body>snow</body></html>`))
		case "/snow.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(imgData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer imageServer.Close()

	engine := newFallbackEngineForTest(t, storage, stubFallbackSearcher{
		results: []FallbackSearchResult{{Title: "Toy poodle playing in snow", URL: imageServer.URL + "/snow"}},
	}, stubFallbackBrowser{}, FallbackConfig{RenderBaseURL: "http://127.0.0.1:8899"})
	engine.sceneComposer = nil

	task, err := engine.generateWebCanvas(context.Background(), &MediaRequest{
		Type:   MediaTypeImage,
		Prompt: "帮我生成一张泰迪在雪地里玩耍的照片",
	})
	if err != nil {
		t.Fatalf("generateWebCanvas returned error: %v", err)
	}
	if task == nil || task.Response == nil || len(task.Response.Data) == 0 {
		t.Fatalf("task = %#v, want result", task)
	}
	result := task.Response.Data[0]
	if !strings.HasPrefix(result.URL, "/api/media/generated/images/") {
		t.Fatalf("result url = %q, want stored direct image", result.URL)
	}
	if result.B64JSON != "" {
		t.Fatalf("b64_json = %q, want direct reference image URL", result.B64JSON)
	}
	if task.FallbackInfo == nil {
		t.Fatal("expected fallback info")
	}
	if task.FallbackInfo.RenderMode != "" || task.FallbackInfo.TemplateID != "" {
		t.Fatalf("fallback_info = %#v, want no slide/poster template metadata for direct image", task.FallbackInfo)
	}
}

func TestFallbackCraiyonQuerySlugUsesEnglishHyphenatedQuery(t *testing.T) {
	got := fallbackCraiyonQuerySlug("帮我生成一张泰迪在雪地里奔跑的照片")
	if got != "toy-poodle-snow-running-photo" {
		t.Fatalf("slug = %q, want toy-poodle-snow-running-photo", got)
	}
}

func TestSearchBrowserImageResultsCraiyonScopesScrapeSelectorsToPreviewWindow(t *testing.T) {
	engine := newFallbackEngineForTest(t, nil, stubFallbackSearcher{}, stubFallbackBrowser{
		scrape: func(_ context.Context, req *browser.ScrapeRequest) (*browser.ScrapeResponse, error) {
			if req == nil {
				t.Fatal("expected scrape request")
			}
			if !strings.Contains(req.URL, "craiyon.com/en/search/toy-poodle-running-in-snow") {
				return nil, fmt.Errorf("unexpected scrape url: %s", req.URL)
			}
			imagesCfg, ok := req.Selectors["images"]
			if !ok {
				t.Fatal("expected images selector")
			}
			if !strings.Contains(imagesCfg.Selector, "img.craiyon.com") {
				t.Fatalf("images selector = %q, want img.craiyon.com-scoped selector", imagesCfg.Selector)
			}
			if imagesCfg.MaxMatches != fallbackCraiyonPreviewResultLimit {
				t.Fatalf("images max_matches = %d, want %d", imagesCfg.MaxMatches, fallbackCraiyonPreviewResultLimit)
			}
			if !req.SkipWaitLoad {
				t.Fatalf("scrape request = %#v, want skip_wait_load for Craiyon first-screen scrape", req)
			}
			linksCfg, ok := req.Selectors["links"]
			if !ok {
				t.Fatal("expected links selector")
			}
			if !strings.Contains(linksCfg.Selector, "/images/") {
				t.Fatalf("links selector = %q, want result-link scoped selector", linksCfg.Selector)
			}
			if linksCfg.MaxMatches != fallbackCraiyonPreviewResultLimit {
				t.Fatalf("links max_matches = %d, want %d", linksCfg.MaxMatches, fallbackCraiyonPreviewResultLimit)
			}
			return &browser.ScrapeResponse{URL: req.URL, Data: map[string]interface{}{}}, nil
		},
	}, FallbackConfig{SearchMaxResults: 20})

	results := engine.searchBrowserImageResults(context.Background(), "toy poodle running in snow")
	if len(results) != 0 {
		t.Fatalf("results = %#v, want no parsed image results from empty scrape payload", results)
	}
}

func TestSearchBrowserImageResultsCraiyonUsesIntentJudgeToReorder(t *testing.T) {
	engine := newFallbackEngineForTest(t, nil, stubFallbackSearcher{}, stubFallbackBrowser{
		scrape: func(_ context.Context, req *browser.ScrapeRequest) (*browser.ScrapeResponse, error) {
			if !strings.Contains(req.URL, "craiyon.com/en/search/toy-poodle-running-in-snow") {
				return nil, fmt.Errorf("unexpected scrape url: %s", req.URL)
			}
			return &browser.ScrapeResponse{
				URL: "https://www.craiyon.com/en/search/toy-poodle-running-in-snow",
				Data: map[string]interface{}{
					"images": []string{
						"https://img.craiyon.com/a.png",
						"https://img.craiyon.com/b.png",
					},
					"alts": []string{"toy poodle sitting", "toy poodle running in snow"},
				},
			}, nil
		},
		screenshot: func(_ context.Context, req *browser.ScreenshotRequest) (*browser.ScreenshotResponse, error) {
			if req == nil || req.FullPage {
				t.Fatalf("expected capped viewport screenshot request, got %#v", req)
			}
			if !req.SkipWaitLoad {
				t.Fatalf("screenshot request = %#v, want skip_wait_load for Craiyon judge screenshot", req)
			}
			if req.Height != fallbackCraiyonJudgeViewportHeight {
				t.Fatalf("screenshot height = %d, want %d", req.Height, fallbackCraiyonJudgeViewportHeight)
			}
			return &browser.ScreenshotResponse{Data: fakeMediaImagePNGBase64, Format: browser.FormatPNG}, nil
		},
	}, FallbackConfig{SearchMaxResults: 2})
	engine.SetVisionBridge(stubFallbackVLM{
		chat: func(_ context.Context, prompt string, imageBase64 string) (string, error) {
			if !strings.Contains(prompt, "semantic intent match") {
				t.Fatalf("judge prompt = %q, want intent-match instruction", prompt)
			}
			if !strings.Contains(prompt, "toy poodle running in snow") {
				t.Fatalf("judge prompt = %q, want normalized intent", prompt)
			}
			if imageBase64 == "" {
				t.Fatal("expected screenshot payload")
			}
			return `{"selected_index":2,"score":92,"reason":"matches the running dog in snow"}`, nil
		},
	})

	results := engine.searchBrowserImageResults(context.Background(), "toy poodle running in snow")
	if len(results) != 2 {
		t.Fatalf("results = %#v, want 2 entries", results)
	}
	if results[0].ImageURL != "https://img.craiyon.com/b.png" {
		t.Fatalf("first result = %q, want craiyon result reordered to selected image", results[0].ImageURL)
	}
	if results[0].Provider != "Craiyon Search" {
		t.Fatalf("provider = %q, want Craiyon Search", results[0].Provider)
	}
}

func TestSearchBrowserImageResultsCraiyonCapsCandidatesBeforeIntentJudge(t *testing.T) {
	imageURLs := make([]string, 0, fallbackCraiyonPreviewResultLimit+6)
	alts := make([]string, 0, fallbackCraiyonPreviewResultLimit+6)
	for i := 0; i < fallbackCraiyonPreviewResultLimit+6; i++ {
		imageURLs = append(imageURLs, fmt.Sprintf("https://img.craiyon.com/%02d.png", i))
		alts = append(alts, fmt.Sprintf("toy poodle variant %02d", i))
	}

	engine := newFallbackEngineForTest(t, nil, stubFallbackSearcher{}, stubFallbackBrowser{
		scrape: func(_ context.Context, req *browser.ScrapeRequest) (*browser.ScrapeResponse, error) {
			if !strings.Contains(req.URL, "craiyon.com/en/search/toy-poodle-running-in-snow") {
				return nil, fmt.Errorf("unexpected scrape url: %s", req.URL)
			}
			return &browser.ScrapeResponse{
				URL: "https://www.craiyon.com/en/search/toy-poodle-running-in-snow",
				Data: map[string]interface{}{
					"images": imageURLs,
					"alts":   alts,
				},
			}, nil
		},
		screenshot: func(_ context.Context, req *browser.ScreenshotRequest) (*browser.ScreenshotResponse, error) {
			return &browser.ScreenshotResponse{Data: fakeMediaImagePNGBase64, Format: browser.FormatPNG}, nil
		},
	}, FallbackConfig{SearchMaxResults: fallbackCraiyonPreviewResultLimit + 6})
	engine.SetVisionBridge(stubFallbackVLM{
		chat: func(_ context.Context, prompt string, imageBase64 string) (string, error) {
			if strings.Contains(prompt, fmt.Sprintf("%d. title=", fallbackCraiyonPreviewResultLimit+1)) {
				t.Fatalf("judge prompt = %q, want candidates capped at %d", prompt, fallbackCraiyonPreviewResultLimit)
			}
			return `{"selected_index":1,"score":90,"reason":"top result still matches intent"}`, nil
		},
	})

	results := engine.searchBrowserImageResults(context.Background(), "toy poodle running in snow")
	if len(results) != fallbackCraiyonPreviewResultLimit {
		t.Fatalf("result count = %d, want %d", len(results), fallbackCraiyonPreviewResultLimit)
	}
}

func TestSearchBrowserImageResultsCraiyonRandomizesWhenVisionUnavailable(t *testing.T) {
	prevRandomIndex := fallbackBrowserRandomIndex
	fallbackBrowserRandomIndex = func(size int) int {
		if size != 2 {
			t.Fatalf("random size = %d, want 2", size)
		}
		return 1
	}
	t.Cleanup(func() {
		fallbackBrowserRandomIndex = prevRandomIndex
	})

	engine := newFallbackEngineForTest(t, nil, stubFallbackSearcher{}, stubFallbackBrowser{
		scrape: func(_ context.Context, req *browser.ScrapeRequest) (*browser.ScrapeResponse, error) {
			if !strings.Contains(req.URL, "craiyon.com/en/search/toy-poodle-running-in-snow") {
				return nil, fmt.Errorf("unexpected scrape url: %s", req.URL)
			}
			return &browser.ScrapeResponse{
				URL: "https://www.craiyon.com/en/search/toy-poodle-running-in-snow",
				Data: map[string]interface{}{
					"images": []string{
						"https://img.craiyon.com/a.png",
						"https://img.craiyon.com/b.png",
					},
					"alts": []string{"toy poodle sitting", "toy poodle running in snow"},
				},
			}, nil
		},
		screenshot: func(_ context.Context, req *browser.ScreenshotRequest) (*browser.ScreenshotResponse, error) {
			if req == nil || req.FullPage {
				t.Fatalf("expected capped viewport screenshot request, got %#v", req)
			}
			if req.Height != fallbackCraiyonJudgeViewportHeight {
				t.Fatalf("screenshot height = %d, want %d", req.Height, fallbackCraiyonJudgeViewportHeight)
			}
			return &browser.ScreenshotResponse{Data: fakeMediaImagePNGBase64, Format: browser.FormatPNG}, nil
		},
	}, FallbackConfig{SearchMaxResults: 2})

	results := engine.searchBrowserImageResults(context.Background(), "toy poodle running in snow")
	if len(results) != 2 {
		t.Fatalf("results = %#v, want 2 entries", results)
	}
	if results[0].ImageURL != "https://img.craiyon.com/b.png" {
		t.Fatalf("first result = %q, want random fallback to promote second image", results[0].ImageURL)
	}
}

func TestSearchBrowserImageResultsCraiyonJudgeScreenshotUsesCappedViewport(t *testing.T) {
	var screenshotReq *browser.ScreenshotRequest
	engine := newFallbackEngineForTest(t, nil, stubFallbackSearcher{}, stubFallbackBrowser{
		scrape: func(_ context.Context, req *browser.ScrapeRequest) (*browser.ScrapeResponse, error) {
			if !strings.Contains(req.URL, "craiyon.com/en/search/toy-poodle-running-in-snow") {
				return nil, fmt.Errorf("unexpected scrape url: %s", req.URL)
			}
			return &browser.ScrapeResponse{
				URL: "https://www.craiyon.com/en/search/toy-poodle-running-in-snow",
				Data: map[string]interface{}{
					"images": []string{
						"https://img.craiyon.com/a.png",
						"https://img.craiyon.com/b.png",
					},
					"alts": []string{"toy poodle sitting", "toy poodle running in snow"},
				},
			}, nil
		},
		screenshot: func(_ context.Context, req *browser.ScreenshotRequest) (*browser.ScreenshotResponse, error) {
			screenshotReq = req
			return &browser.ScreenshotResponse{Data: fakeMediaImagePNGBase64, Format: browser.FormatPNG}, nil
		},
	}, FallbackConfig{SearchMaxResults: 2, ScreenshotHeight: fallbackCraiyonJudgeViewportHeight + 800})
	engine.SetVisionBridge(stubFallbackVLM{
		chat: func(_ context.Context, _ string, _ string) (string, error) {
			return `{"selected_index":2,"score":92,"reason":"matches the running dog in snow"}`, nil
		},
	})

	results := engine.searchBrowserImageResults(context.Background(), "toy poodle running in snow")
	if len(results) != 2 {
		t.Fatalf("results = %#v, want 2 entries", results)
	}
	if screenshotReq == nil {
		t.Fatal("expected screenshot request")
	}
	if screenshotReq.FullPage {
		t.Fatalf("screenshot request = %#v, want viewport capture instead of full page", screenshotReq)
	}
	if screenshotReq.Height != fallbackCraiyonJudgeViewportHeight {
		t.Fatalf("screenshot height = %d, want %d", screenshotReq.Height, fallbackCraiyonJudgeViewportHeight)
	}
}

func TestSearchBrowserImageResultsPhotoPromptDropsRandomCraiyonPickAfterScreenshotFailure(t *testing.T) {
	prevRandomIndex := fallbackBrowserRandomIndex
	fallbackBrowserRandomIndex = func(size int) int {
		if size != 2 {
			t.Fatalf("random size = %d, want 2", size)
		}
		return 1
	}
	t.Cleanup(func() {
		fallbackBrowserRandomIndex = prevRandomIndex
	})

	engine := newFallbackEngineForTest(t, nil, stubFallbackSearcher{}, stubFallbackBrowser{
		scrape: func(_ context.Context, req *browser.ScrapeRequest) (*browser.ScrapeResponse, error) {
			if req == nil || !strings.Contains(req.URL, "craiyon.com/en/search/") {
				return &browser.ScrapeResponse{URL: req.URL, Data: map[string]interface{}{}}, nil
			}
			return &browser.ScrapeResponse{
				URL: req.URL,
				Data: map[string]interface{}{
					"images": []string{
						"https://img.craiyon.com/a.png",
						"https://img.craiyon.com/b.png",
					},
					"alts": []string{"Generated image", "Generated image"},
				},
			}, nil
		},
		screenshot: func(_ context.Context, req *browser.ScreenshotRequest) (*browser.ScreenshotResponse, error) {
			if req == nil || req.FullPage {
				t.Fatalf("expected capped viewport screenshot request, got %#v", req)
			}
			if req.Height != fallbackCraiyonJudgeViewportHeight {
				t.Fatalf("screenshot height = %d, want %d", req.Height, fallbackCraiyonJudgeViewportHeight)
			}
			return nil, context.DeadlineExceeded
		},
	}, FallbackConfig{SearchMaxResults: 2})

	results := engine.searchBrowserImageResults(context.Background(), "帮我生成一张泰迪在雪地里玩耍的照片")
	if len(results) != 0 {
		t.Fatalf("results = %#v, want random-picked craiyon result filtered for photo prompt", results)
	}
}

func TestFallbackBrowserImageEnginesPreferCraiyonFirstForPhotoPrompt(t *testing.T) {
	engines := fallbackBrowserImageEngines("帮我生成一张泰迪在雪地里玩耍的照片", "zh-CN")
	if len(engines) < 4 {
		t.Fatalf("engines = %#v, want craiyon + bing + google + baidu", engines)
	}
	if engines[0].Name != "craiyon_search" {
		t.Fatalf("first engine = %q, want craiyon_search", engines[0].Name)
	}
	if engines[1].Name != "bing_images" {
		t.Fatalf("second engine = %q, want bing_images", engines[1].Name)
	}
	if engines[2].Name != "google_images" {
		t.Fatalf("third engine = %q, want google_images", engines[2].Name)
	}
	if engines[3].Name != "baidu_images" {
		t.Fatalf("fourth engine = %q, want baidu_images", engines[3].Name)
	}
}

func TestFallbackBrowserImageEnginesCraiyonUsesTrimmedPreviewWindow(t *testing.T) {
	engines := fallbackBrowserImageEngines("toy poodle running in snow", "en-US")
	if len(engines) == 0 {
		t.Fatal("expected browser image engines")
	}
	craiyon := engines[0]
	if craiyon.Name != "craiyon_search" {
		t.Fatalf("first engine = %q, want craiyon_search", craiyon.Name)
	}
	if craiyon.ResultLimit != 10 {
		t.Fatalf("result limit = %d, want 10", craiyon.ResultLimit)
	}
	if craiyon.JudgeViewportHeight != 960 {
		t.Fatalf("judge viewport height = %d, want 960", craiyon.JudgeViewportHeight)
	}
	imagesCfg, ok := craiyon.Selectors["images"]
	if !ok {
		t.Fatal("expected images selector")
	}
	if imagesCfg.MaxMatches != 10 {
		t.Fatalf("images max_matches = %d, want 10", imagesCfg.MaxMatches)
	}
	linksCfg, ok := craiyon.Selectors["links"]
	if !ok {
		t.Fatal("expected links selector")
	}
	if linksCfg.MaxMatches != 10 {
		t.Fatalf("links max_matches = %d, want 10", linksCfg.MaxMatches)
	}
}

func TestPrepareFallbackBrowserJudgeScreenshotResizesTallPayload(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 1600, 4800))
	for y := 0; y < 4800; y++ {
		for x := 0; x < 1600; x++ {
			src.Set(x, y, color.RGBA{
				R: uint8((x + y) % 255),
				G: uint8((x*3 + y*5) % 255),
				B: uint8((x*7 + y*11) % 255),
				A: 255,
			})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	prepared := prepareFallbackBrowserJudgeScreenshot(base64.StdEncoding.EncodeToString(buf.Bytes()))
	if prepared == "" {
		t.Fatal("expected prepared screenshot payload")
	}

	raw, err := base64.StdEncoding.DecodeString(prepared)
	if err != nil {
		t.Fatalf("decode prepared payload: %v", err)
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("decode prepared image: %v", err)
	}
	bounds := img.Bounds()
	if got := max(bounds.Dx(), bounds.Dy()); got > fallbackBrowserJudgeMaxImageDim {
		t.Fatalf("max dimension = %d, want <= %d", got, fallbackBrowserJudgeMaxImageDim)
	}
	if bounds.Dy() >= 4800 {
		t.Fatalf("height = %d, want resized payload shorter than original", bounds.Dy())
	}
}

func TestSearchBrowserImageResultsPhotoPromptUsesCraiyonAndShorterTimeoutBudget(t *testing.T) {
	var visitedURLs []string
	var timeouts []int
	engine := newFallbackEngineForTest(t, nil, stubFallbackSearcher{}, stubFallbackBrowser{
		scrape: func(_ context.Context, req *browser.ScrapeRequest) (*browser.ScrapeResponse, error) {
			if req == nil {
				t.Fatal("expected scrape request")
			}
			visitedURLs = append(visitedURLs, req.URL)
			timeouts = append(timeouts, req.Timeout)
			return &browser.ScrapeResponse{
				URL:  req.URL,
				Data: map[string]interface{}{},
			}, nil
		},
	}, FallbackConfig{SearchMaxResults: 2})

	results := engine.searchBrowserImageResults(context.Background(), "帮我生成一张泰迪在雪地里玩耍的照片")
	if len(results) != 0 {
		t.Fatalf("results = %#v, want no parsed image results from empty scrape payloads", results)
	}
	if len(visitedURLs) == 0 {
		t.Fatal("expected browser image engines to run")
	}
	if !strings.Contains(visitedURLs[0], "craiyon.com") {
		t.Fatalf("visited urls = %v, want craiyon search to run first", visitedURLs)
	}
	wantCraiyonTimeout := 20_000
	wantGenericTimeout := int((fallbackBrowserSearchTimeout + 2*time.Second) / time.Millisecond)
	for idx, visitedURL := range visitedURLs {
		if strings.Contains(visitedURL, "craiyon.com") {
			if timeouts[idx] != wantCraiyonTimeout {
				t.Fatalf("craiyon timeouts = %v for urls %v, want %dms for craiyon", timeouts, visitedURLs, wantCraiyonTimeout)
			}
			continue
		}
		if timeouts[idx] != wantGenericTimeout {
			t.Fatalf("timeouts = %v for urls %v, want non-craiyon searches to use %dms", timeouts, visitedURLs, wantGenericTimeout)
		}
	}
}

func TestGenerateWebCanvasPhotoPromptFallsBackToBingImageBrowserResults(t *testing.T) {
	tmp := t.TempDir()
	storage := NewMediaStorage(tmp, "/api/media/generated")
	if err := storage.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	_, imgData, _ := parseDataURLPayload("data:image/png;base64," + fakeMediaImagePNGBase64)
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/snow.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(imgData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer imageServer.Close()

	engine := newFallbackEngineForTest(t, storage, stubFallbackSearcher{}, stubFallbackBrowser{
		scrape: func(_ context.Context, req *browser.ScrapeRequest) (*browser.ScrapeResponse, error) {
			if req == nil || !strings.Contains(req.URL, "bing.com/images/search") {
				return &browser.ScrapeResponse{Data: map[string]interface{}{}}, nil
			}
			return &browser.ScrapeResponse{
				URL: req.URL,
				Data: map[string]interface{}{
					"metadata": []string{
						`{&quot;purl&quot;:&quot;https://example.com/teddy-in-snow&quot;,&quot;murl&quot;:&quot;` + imageServer.URL + `/snow.png&quot;,&quot;turl&quot;:&quot;` + imageServer.URL + `/snow.png&quot;,&quot;t&quot;:&quot;Toy poodle in snow&quot;,&quot;desc&quot;:&quot;Toy poodle in snow&quot;}`,
					},
					"labels": []string{"Toy poodle in snow"},
				},
			}, nil
		},
	}, FallbackConfig{
		RenderBaseURL:    "http://127.0.0.1:8899",
		SearchMaxResults: 1,
	})

	task, err := engine.generateWebCanvas(context.Background(), &MediaRequest{
		Type:   MediaTypeImage,
		Prompt: "帮我生成一张泰迪在雪地里玩耍的照片",
	})
	if err != nil {
		t.Fatalf("generateWebCanvas returned error: %v", err)
	}
	if task == nil || task.Response == nil || len(task.Response.Data) == 0 {
		t.Fatalf("task = %#v, want result", task)
	}
	if task.FallbackInfo == nil || len(task.FallbackInfo.Sources) != 1 {
		t.Fatalf("fallback_info = %#v, want one disclosed source", task.FallbackInfo)
	}
	source := task.FallbackInfo.Sources[0]
	if source.Provider != "Bing Images" {
		t.Fatalf("provider = %q, want Bing Images", source.Provider)
	}
	if source.VerifiedLicense {
		t.Fatalf("verified_license = %v, want false", source.VerifiedLicense)
	}
	if !strings.Contains(source.Note, "license not verified") {
		t.Fatalf("note = %q, want unverified license note", source.Note)
	}
	if len(task.FallbackInfo.SourceURLs) != 1 || task.FallbackInfo.SourceURLs[0] != "https://example.com/teddy-in-snow" {
		t.Fatalf("source_urls = %#v", task.FallbackInfo.SourceURLs)
	}
}

func TestGenerateWebCanvasPhotoPromptFallsBackToGoogleImageBrowserResults(t *testing.T) {
	tmp := t.TempDir()
	storage := NewMediaStorage(tmp, "/api/media/generated")
	if err := storage.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	_, imgData, _ := parseDataURLPayload("data:image/png;base64," + fakeMediaImagePNGBase64)
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/snow.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(imgData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer imageServer.Close()

	engine := newFallbackEngineForTest(t, storage, stubFallbackSearcher{}, stubFallbackBrowser{
		scrape: func(_ context.Context, req *browser.ScrapeRequest) (*browser.ScrapeResponse, error) {
			if req == nil || !strings.Contains(req.URL, "google.com/search?tbm=isch") {
				return &browser.ScrapeResponse{Data: map[string]interface{}{}}, nil
			}
			return &browser.ScrapeResponse{
				URL: req.URL,
				Data: map[string]interface{}{
					"result_links": []string{
						"/imgres?imgurl=" + url.QueryEscape(imageServer.URL+"/snow.png") + "&imgrefurl=" + url.QueryEscape("https://example.com/teddy-in-snow"),
					},
					"image_alts": []string{"Toy poodle in snow"},
				},
			}, nil
		},
	}, FallbackConfig{
		RenderBaseURL:    "http://127.0.0.1:8899",
		SearchMaxResults: 1,
	})

	task, err := engine.generateWebCanvas(context.Background(), &MediaRequest{
		Type:   MediaTypeImage,
		Prompt: "帮我生成一张泰迪在雪地里玩耍的照片",
	})
	if err != nil {
		t.Fatalf("generateWebCanvas returned error: %v", err)
	}
	if task == nil || task.Response == nil || len(task.Response.Data) == 0 {
		t.Fatalf("task = %#v, want result", task)
	}
	if task.FallbackInfo == nil || len(task.FallbackInfo.Sources) != 1 {
		t.Fatalf("fallback_info = %#v, want one disclosed source", task.FallbackInfo)
	}
	source := task.FallbackInfo.Sources[0]
	if source.Provider != "Google Images" {
		t.Fatalf("provider = %q, want Google Images", source.Provider)
	}
	if source.VerifiedLicense {
		t.Fatalf("verified_license = %v, want false", source.VerifiedLicense)
	}
	if !strings.Contains(source.Note, "license not verified") {
		t.Fatalf("note = %q, want unverified license note", source.Note)
	}
	if len(task.FallbackInfo.SourceURLs) != 1 || task.FallbackInfo.SourceURLs[0] != "https://example.com/teddy-in-snow" {
		t.Fatalf("source_urls = %#v", task.FallbackInfo.SourceURLs)
	}
}

func TestGenerateWebCanvasPhotoPromptDirectDownloadsRandomBrowserPick(t *testing.T) {
	prevRandomIndex := fallbackBrowserRandomIndex
	fallbackBrowserRandomIndex = func(size int) int {
		if size != 2 {
			t.Fatalf("random size = %d, want 2", size)
		}
		return 1
	}
	t.Cleanup(func() {
		fallbackBrowserRandomIndex = prevRandomIndex
	})

	tmp := t.TempDir()
	storage := NewMediaStorage(tmp, "/api/media/generated")
	if err := storage.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	_, imgData, _ := parseDataURLPayload("data:image/png;base64," + fakeMediaImagePNGBase64)
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/picked.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(imgData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer imageServer.Close()

	engine := newFallbackEngineForTest(t, storage, stubFallbackSearcher{}, stubFallbackBrowser{
		scrape: func(_ context.Context, req *browser.ScrapeRequest) (*browser.ScrapeResponse, error) {
			if req == nil || !strings.Contains(req.URL, "google.com/search?tbm=isch") {
				return &browser.ScrapeResponse{URL: req.URL, Data: map[string]interface{}{}}, nil
			}
			return &browser.ScrapeResponse{
				URL: req.URL,
				Data: map[string]interface{}{
					"result_links": []string{
						"/imgres?imgurl=" + url.QueryEscape(imageServer.URL+"/missing.png") + "&imgrefurl=" + url.QueryEscape("https://example.com/toy-poodle-snow-ignored"),
						"/imgres?imgurl=" + url.QueryEscape(imageServer.URL+"/picked.png") + "&imgrefurl=" + url.QueryEscape("https://example.com/toy-poodle-snow-picked"),
					},
					"image_alts": []string{"Toy poodle in snow", "Toy poodle playing in snow"},
				},
			}, nil
		},
		screenshot: func(_ context.Context, req *browser.ScreenshotRequest) (*browser.ScreenshotResponse, error) {
			if req == nil || !req.FullPage {
				t.Fatalf("expected full-page screenshot request")
			}
			return nil, context.DeadlineExceeded
		},
	}, FallbackConfig{
		RenderBaseURL:    "http://127.0.0.1:8899",
		SearchMaxResults: 2,
	})
	engine.sceneComposer = nil

	task, err := engine.generateWebCanvas(context.Background(), &MediaRequest{
		Type:   MediaTypeImage,
		Prompt: "帮我生成一张泰迪在雪地里玩耍的照片",
	})
	if err != nil {
		t.Fatalf("generateWebCanvas returned error: %v", err)
	}
	if task == nil || task.Response == nil || len(task.Response.Data) == 0 {
		t.Fatalf("task = %#v, want direct image result", task)
	}
	result := task.Response.Data[0]
	if result.OriginalURL != "https://example.com/toy-poodle-snow-picked" {
		t.Fatalf("original_url = %q, want random-picked result page url", result.OriginalURL)
	}
	if strings.TrimSpace(result.URL) == "" {
		t.Fatalf("url = %q, want stored local media url", result.URL)
	}
	if task.FallbackInfo == nil || len(task.FallbackInfo.Sources) == 0 {
		t.Fatalf("fallback_info = %#v, want disclosed sources", task.FallbackInfo)
	}
	source := task.FallbackInfo.Sources[0]
	if source.PageURL != "https://example.com/toy-poodle-snow-picked" {
		t.Fatalf("page_url = %q, want picked page url", source.PageURL)
	}
	if source.AssetURL != imageServer.URL+"/picked.png" {
		t.Fatalf("asset_url = %q, want picked image url", source.AssetURL)
	}
	if len(task.FallbackInfo.SourceURLs) == 0 || task.FallbackInfo.SourceURLs[0] != "https://example.com/toy-poodle-snow-picked" {
		t.Fatalf("source_urls = %#v, want picked source url first", task.FallbackInfo.SourceURLs)
	}
}

func TestGenerateWebCanvasPhotoPromptFallsBackToBaiduImageJSONResults(t *testing.T) {
	tmp := t.TempDir()
	storage := NewMediaStorage(tmp, "/api/media/generated")
	if err := storage.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	_, imgData, _ := parseDataURLPayload("data:image/png;base64," + fakeMediaImagePNGBase64)
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/snow.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(imgData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer imageServer.Close()

	engine := newFallbackEngineForTest(t, storage, stubFallbackSearcher{}, stubFallbackBrowser{}, FallbackConfig{
		RenderBaseURL:    "http://127.0.0.1:8899",
		SearchMaxResults: 1,
	})
	engine.sourceClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			switch {
			case strings.Contains(req.URL.Host, "image.baidu.com") && strings.Contains(req.URL.Path, "/search/acjson"):
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body: io.NopCloser(strings.NewReader(`{
						"data": [{
							"fromPageTitle": "下雪啦 <strong>泰迪</strong> 在雪地里玩耍",
							"thumbURL": "` + imageServer.URL + `/snow.png",
							"middleURL": "` + imageServer.URL + `/snow.png",
							"replaceUrl": [{
								"ObjURL": "` + imageServer.URL + `/snow.png",
								"FromURL": "https://example.cn/teddy-snow"
							}],
							"fromURLHost": "example.cn"
						}]
					}`)),
					Request: req,
				}, nil
			default:
				return nil, errors.New("unexpected source request: " + req.URL.String())
			}
		}),
	}

	task, err := engine.generateWebCanvas(context.Background(), &MediaRequest{
		Type:   MediaTypeImage,
		Prompt: "帮我生成一张泰迪在雪地里玩耍的照片",
	})
	if err != nil {
		t.Fatalf("generateWebCanvas returned error: %v", err)
	}
	if task == nil || task.Response == nil || len(task.Response.Data) == 0 {
		t.Fatalf("task = %#v, want result", task)
	}
	if task.FallbackInfo == nil || len(task.FallbackInfo.Sources) != 1 {
		t.Fatalf("fallback_info = %#v, want one disclosed source", task.FallbackInfo)
	}
	source := task.FallbackInfo.Sources[0]
	if source.Provider != "Baidu Images" {
		t.Fatalf("provider = %q, want Baidu Images", source.Provider)
	}
	if source.VerifiedLicense {
		t.Fatalf("verified_license = %v, want false", source.VerifiedLicense)
	}
	if !strings.Contains(source.Note, "license not verified") {
		t.Fatalf("note = %q, want unverified license note", source.Note)
	}
	if len(task.FallbackInfo.SourceURLs) != 1 || task.FallbackInfo.SourceURLs[0] != "https://example.cn/teddy-snow" {
		t.Fatalf("source_urls = %#v", task.FallbackInfo.SourceURLs)
	}
}

func TestGenerateWebCanvasPhotoPromptPrefersBingSearchProvider(t *testing.T) {
	tmp := t.TempDir()
	storage := NewMediaStorage(tmp, "/api/media/generated")
	if err := storage.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	_, imgData, _ := parseDataURLPayload("data:image/png;base64," + fakeMediaImagePNGBase64)
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/snow":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<html><head><meta property="og:image" content="/snow.png"></head><body>snow</body></html>`))
		case "/snow.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(imgData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer imageServer.Close()

	var providerChains [][]string
	engine := newFallbackEngineForTest(t, storage, stubFallbackSearcher{
		search: func(_ context.Context, query string, _ int, providers []string) ([]FallbackSearchResult, error) {
			providerChains = append(providerChains, append([]string(nil), providers...))
			if len(providers) == 1 && providers[0] == "bing" && query == "泰迪在雪地里玩耍的照片" {
				return []FallbackSearchResult{{Title: "Toy poodle playing in snow", URL: imageServer.URL + "/snow"}}, nil
			}
			return nil, nil
		},
	}, stubFallbackBrowser{}, FallbackConfig{
		RenderBaseURL:       "http://127.0.0.1:8899",
		SearchProviderChain: []string{"bing", "duckduckgo"},
	})
	engine.sceneComposer = nil

	task, err := engine.generateWebCanvas(context.Background(), &MediaRequest{
		Type:   MediaTypeImage,
		Prompt: "帮我生成一张泰迪在雪地里玩耍的照片",
	})
	if err != nil {
		t.Fatalf("generateWebCanvas returned error: %v", err)
	}
	if len(providerChains) == 0 {
		t.Fatal("expected search provider chains to be recorded")
	}
	if len(providerChains[0]) != 1 || providerChains[0][0] != "bing" {
		t.Fatalf("providerChains = %v, want bing-only search for photo prompt", providerChains)
	}
	for _, chain := range providerChains {
		if containsString(chain, "duckduckgo") {
			t.Fatalf("providerChains = %v, want duckduckgo removed from image search", providerChains)
		}
	}
	if task == nil || task.Response == nil || len(task.Response.Data) == 0 || task.Response.Data[0].URL == "" {
		t.Fatalf("task = %#v, want direct reference result", task)
	}
}

func TestGenerateWebCanvasPhotoPromptFallsBackToSanitizedConfiguredProvidersWhenPreferredProviderHasNoResults(t *testing.T) {
	tmp := t.TempDir()
	storage := NewMediaStorage(tmp, "/api/media/generated")
	if err := storage.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	_, imgData, _ := parseDataURLPayload("data:image/png;base64," + fakeMediaImagePNGBase64)
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/snow":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<html><head><meta property="og:image" content="/snow.png"></head><body>snow</body></html>`))
		case "/snow.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(imgData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer imageServer.Close()

	var providerChains [][]string
	engine := newFallbackEngineForTest(t, storage, stubFallbackSearcher{
		search: func(_ context.Context, query string, _ int, providers []string) ([]FallbackSearchResult, error) {
			providerChains = append(providerChains, append([]string(nil), providers...))
			if query != "泰迪在雪地里玩耍的照片" {
				return nil, nil
			}
			if len(providers) == 1 && providers[0] == "bing" {
				return nil, nil
			}
			if len(providers) == 2 && providers[0] == "bing" && providers[1] == "brave" {
				return []FallbackSearchResult{{Title: "Toy poodle playing in snow", URL: imageServer.URL + "/snow"}}, nil
			}
			return nil, nil
		},
	}, stubFallbackBrowser{}, FallbackConfig{
		RenderBaseURL:       "http://127.0.0.1:8899",
		SearchProviderChain: []string{"bing", "brave", "duckduckgo"},
	})
	engine.sceneComposer = nil

	task, err := engine.generateWebCanvas(context.Background(), &MediaRequest{
		Type:   MediaTypeImage,
		Prompt: "帮我生成一张泰迪在雪地里玩耍的照片",
	})
	if err != nil {
		t.Fatalf("generateWebCanvas returned error: %v", err)
	}
	if len(providerChains) < 2 {
		t.Fatalf("providerChains = %v, want preferred provider probe followed by sanitized configured provider fallback", providerChains)
	}
	if len(providerChains[0]) != 1 || providerChains[0][0] != "bing" {
		t.Fatalf("first provider chain = %v, want bing-only", providerChains[0])
	}
	foundConfiguredChain := false
	for _, chain := range providerChains {
		if containsString(chain, "duckduckgo") {
			t.Fatalf("providerChains = %v, want duckduckgo removed from image search fallback", providerChains)
		}
		if len(chain) == 2 && chain[0] == "bing" && chain[1] == "brave" {
			foundConfiguredChain = true
		}
	}
	if !foundConfiguredChain {
		t.Fatalf("providerChains = %v, want sanitized configured provider chain after bing-only probe", providerChains)
	}
	if task == nil || task.Response == nil || len(task.Response.Data) == 0 || task.Response.Data[0].URL == "" {
		t.Fatalf("task = %#v, want direct reference result after provider fallback", task)
	}
}

func TestNewFallbackEngineSanitizesImageSearchProviderChain(t *testing.T) {
	engine := NewFallbackEngine(FallbackConfig{
		Enabled:             true,
		SearchProviderChain: []string{"duckduckgo", "bing", "duckduckgo"},
	}, nil, stubFallbackSearcher{}, func() FallbackBrowserService { return nil }, "zh-CN")

	if engine == nil {
		t.Fatal("expected engine")
	}
	if len(engine.config.SearchProviderChain) != 1 || engine.config.SearchProviderChain[0] != "bing" {
		t.Fatalf("search provider chain = %#v, want [bing]", engine.config.SearchProviderChain)
	}
}

func TestManagerGenerateUsesTextPosterWhenSearchHasNoImage(t *testing.T) {
	tmp := t.TempDir()
	storage := NewMediaStorage(tmp, "/api/media/generated")
	if err := storage.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	manager := NewManager(storage, nil, "zh-CN")
	manager.SetFallbackEngine(newFallbackEngineForTest(t, storage, stubFallbackSearcher{
		results: nil,
	}, nil, FallbackConfig{}))

	task, err := manager.Generate(context.Background(), &MediaRequest{
		Type:   MediaTypeImage,
		Prompt: "请做一张没有图片来源的纯文字海报",
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	waitCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	task, err = manager.WaitForTask(waitCtx, task.ID)
	if err != nil {
		t.Fatalf("WaitForTask returned error: %v", err)
	}
	if task.FallbackInfo == nil || task.FallbackInfo.Strategy != FallbackStrategyWebCanvas {
		t.Fatalf("fallback_info = %#v, want web_canvas", task.FallbackInfo)
	}
	if len(task.Response.Data) == 0 || task.Response.Data[0].URL == "" {
		t.Fatalf("response = %#v, want generated poster asset", task.Response)
	}
}

func TestManagerGenerateUsesWebCanvasForDetailedSingleScenePrompt(t *testing.T) {
	tmp := t.TempDir()
	storage := NewMediaStorage(tmp, "/api/media/generated")
	if err := storage.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	manager := NewManager(storage, nil, "zh-CN")
	manager.SetFallbackEngine(newFallbackEngineForTest(t, storage, stubFallbackSearcher{
		results: nil,
	}, nil, FallbackConfig{}))

	task, err := manager.Generate(context.Background(), &MediaRequest{
		Type:   MediaTypeImage,
		Prompt: "A friendly robot with warm glowing eyes sitting at a wooden table in a cozy coffee shop, reading an open book. A steaming coffee cup sits nearby. Soft ambient lighting, wooden furniture, bookshelves lining the walls, large windows with warm afternoon light, and a welcoming cheerful atmosphere. Detailed digital illustration.",
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	waitCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	task, err = manager.WaitForTask(waitCtx, task.ID)
	if err != nil {
		t.Fatalf("WaitForTask returned error: %v", err)
	}
	if task.FallbackInfo == nil || task.FallbackInfo.Strategy != FallbackStrategyWebCanvas {
		t.Fatalf("fallback_info = %#v, want web_canvas", task.FallbackInfo)
	}
}

func TestManagerCreateTaskUsesPublicSpaceFallbackWithPresetFailover(t *testing.T) {
	tmp := t.TempDir()
	storage := NewMediaStorage(tmp, "/api/media/generated")
	if err := storage.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	resultServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write([]byte("fake-video"))
	}))
	defer resultServer.Close()

	var mu sync.Mutex
	currentURL := ""
	submitted := false
	browserStub := stubFallbackBrowser{
		openTab: func(_ context.Context, url string) (*browser.Tab, error) {
			mu.Lock()
			defer mu.Unlock()
			currentURL = url
			submitted = false
			return &browser.Tab{TargetID: "tab-" + strings.ReplaceAll(url, "https://", ""), URL: url, Title: url, Active: true}, nil
		},
		exists: func(_ context.Context, _ string, selector string) (bool, error) {
			mu.Lock()
			defer mu.Unlock()
			switch selector {
			case ".ready", "textarea", "button.generate":
				return true, nil
			case ".preset-one-error":
				return submitted && strings.Contains(currentURL, "preset-one"), nil
			case "video source":
				return submitted && strings.Contains(currentURL, "preset-two"), nil
			default:
				return false, nil
			}
		},
		act: func(_ context.Context, req *browser.ActRequest) (*browser.ActResponse, error) {
			if req.Kind == "click" {
				mu.Lock()
				submitted = true
				mu.Unlock()
			}
			return &browser.ActResponse{Success: true}, nil
		},
		extract: func(_ context.Context, _ string, selector, attribute string) (string, error) {
			mu.Lock()
			defer mu.Unlock()
			switch selector {
			case ".preset-one-error":
				if submitted && strings.Contains(currentURL, "preset-one") {
					return "preset one failed", nil
				}
			case "video source":
				if submitted && strings.Contains(currentURL, "preset-two") {
					return resultServer.URL + "/final.mp4", nil
				}
			}
			return "", nil
		},
		pageInfo: func(_ context.Context, _ string) (string, string, error) {
			mu.Lock()
			defer mu.Unlock()
			return currentURL, currentURL, nil
		},
	}

	engine := newFallbackEngineForTest(t, storage, nil, browserStub, FallbackConfig{
		PublicSpaces: []FallbackPublicSpacePreset{
			{
				ID:               "preset-one",
				DisplayName:      "Preset One",
				URL:              "https://spaces.example/preset-one",
				Categories:       []MediaCategory{CategoryT2V},
				ReadySelectors:   []string{".ready"},
				PromptSelectors:  []string{"textarea"},
				SubmitSelectors:  []string{"button.generate"},
				ErrorSelectors:   []string{".preset-one-error"},
				PollInterval:     50 * time.Millisecond,
				Timeout:          400 * time.Millisecond,
				SuccessSelectors: nil,
			},
			{
				ID:              "preset-two",
				DisplayName:     "Preset Two",
				URL:             "https://spaces.example/preset-two",
				Categories:      []MediaCategory{CategoryT2V},
				ReadySelectors:  []string{".ready"},
				PromptSelectors: []string{"textarea"},
				SubmitSelectors: []string{"button.generate"},
				SuccessSelectors: []FallbackResultSelector{
					{Selectors: []string{"video source"}, Attribute: "src", Kind: "video"},
				},
				PollInterval: 50 * time.Millisecond,
				Timeout:      time.Second,
			},
		},
	})

	manager := NewManager(storage, nil, "zh-CN")
	manager.SetFallbackEngine(engine)

	task, err := manager.CreateTask(context.Background(), &MediaRequest{
		Type:   MediaTypeVideo,
		Prompt: "做一个短视频片头",
	}, "", string(CategoryT2V), "web")
	if err != nil {
		t.Fatalf("CreateTask returned error: %v", err)
	}
	if task.FallbackInfo == nil || task.FallbackInfo.Strategy != FallbackStrategyPublicSpace {
		t.Fatalf("fallback_info = %#v, want public_space", task.FallbackInfo)
	}

	waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	task, err = manager.WaitForTask(waitCtx, task.ID)
	if err != nil {
		t.Fatalf("WaitForTask returned error: %v", err)
	}
	if task.Status != TaskStatusSucceeded {
		t.Fatalf("status = %q, want succeeded", task.Status)
	}
	if task.FallbackInfo == nil || task.FallbackInfo.DisplayName != "Preset Two" {
		t.Fatalf("fallback_info = %#v, want preset two metadata", task.FallbackInfo)
	}
	if task.Response == nil || len(task.Response.Data) != 1 || !strings.HasPrefix(task.Response.Data[0].URL, "/api/media/generated/videos/") {
		t.Fatalf("response = %#v, want local stored video", task.Response)
	}
}

func TestResolveExecutorPrefersWebCanvasForSlideSignals(t *testing.T) {
	manager := NewManager(nil, nil, "zh-CN")
	manager.SetFallbackEngine(NewFallbackEngine(FallbackConfig{Enabled: true}, nil, nil, func() FallbackBrowserService { return nil }, "zh-CN"))

	longPrompt := strings.Repeat("detailed market sizing, user segmentation, operating metrics, and rollout notes ", 12)
	cases := []struct {
		name   string
		prompt string
		extra  map[string]any
	}{
		{
			name:   "prompt cue",
			prompt: "帮我做一页幻灯片封面，主题是 2026 产品战略",
		},
		{
			name:   "style preset",
			prompt: longPrompt,
			extra:  map[string]any{"style_preset": "banana_slides"},
		},
		{
			name:   "quality profile",
			prompt: longPrompt,
			extra:  map[string]any{"quality_profile": "ppt"},
		},
		{
			name:   "source signal",
			prompt: longPrompt,
			extra:  map[string]any{"source": "slides"},
		},
	}

	for _, tc := range cases {
		req := &MediaRequest{
			Type:   MediaTypeImage,
			Prompt: tc.prompt,
			Extra:  tc.extra,
		}
		provider, modelID, err := manager.resolveExecutor(req, string(CategoryT2I))
		if err != nil {
			t.Fatalf("%s resolveExecutor returned error: %v", tc.name, err)
		}
		if provider == nil || provider.Name() != fallbackProviderName {
			t.Fatalf("%s provider = %#v, want fallback", tc.name, provider)
		}
		if modelID != FallbackModelWebCanvasT2I {
			t.Fatalf("%s model = %q, want %q", tc.name, modelID, FallbackModelWebCanvasT2I)
		}
		info := manager.fallback.PendingInfoForRequest(req, modelID)
		if info == nil || info.RenderMode != "slide" {
			t.Fatalf("%s fallback_info = %#v, want slide render mode", tc.name, info)
		}
	}
}

func TestManagerGenerateSlideFallbackUsesTextOnlyTemplateWhenNoVisual(t *testing.T) {
	tmp := t.TempDir()
	storage := NewMediaStorage(tmp, "/api/media/generated")
	if err := storage.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	manager := NewManager(storage, nil, "zh-CN")
	manager.SetFallbackEngine(newFallbackEngineForTest(t, storage, stubFallbackSearcher{}, stubFallbackBrowser{}, FallbackConfig{}))

	task, err := manager.Generate(context.Background(), &MediaRequest{
		Type:   MediaTypeImage,
		Prompt: "帮我做一页PPT，标题：2026 产品战略；副标题：AI 驱动增长；要点：提升转化率；降低成本；全球化扩张",
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	waitCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	task, err = manager.WaitForTask(waitCtx, task.ID)
	if err != nil {
		t.Fatalf("WaitForTask returned error: %v", err)
	}
	if task.FallbackInfo == nil || task.FallbackInfo.RenderMode != "slide" {
		t.Fatalf("fallback_info = %#v, want slide", task.FallbackInfo)
	}
	if task.FallbackInfo.TemplateID != "text_only" {
		t.Fatalf("template_id = %q, want text_only", task.FallbackInfo.TemplateID)
	}
}

func TestManagerGenerateSlideFallbackUsesSplitTemplateWhenVisualExists(t *testing.T) {
	tmp := t.TempDir()
	storage := NewMediaStorage(tmp, "/api/media/generated")
	if err := storage.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	_, imgData, _ := parseDataURLPayload("data:image/png;base64," + fakeMediaImagePNGBase64)
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(imgData)
	}))
	defer imageServer.Close()

	manager := NewManager(storage, nil, "zh-CN")
	manager.SetFallbackEngine(newFallbackEngineForTest(t, storage, stubFallbackSearcher{
		results: []FallbackSearchResult{{Title: "Market visual", URL: imageServer.URL + "/hero.png"}},
	}, stubFallbackBrowser{}, FallbackConfig{}))

	task, err := manager.Generate(context.Background(), &MediaRequest{
		Type:   MediaTypeImage,
		Prompt: "帮我做一页PPT，标题：市场格局；副标题：2026 增长路径；要点：企业客户；渠道拓展",
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	waitCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	task, err = manager.WaitForTask(waitCtx, task.ID)
	if err != nil {
		t.Fatalf("WaitForTask returned error: %v", err)
	}
	if task.FallbackInfo == nil || task.FallbackInfo.TemplateID != "split" {
		t.Fatalf("fallback_info = %#v, want split template", task.FallbackInfo)
	}
}

func TestRunSpacePresetWrapsSlidePromptAndNegativePrompt(t *testing.T) {
	tmp := t.TempDir()
	storage := NewMediaStorage(tmp, "/api/media/generated")
	if err := storage.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	_, imgData, _ := parseDataURLPayload("data:image/png;base64," + fakeMediaImagePNGBase64)
	resultServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(imgData)
	}))
	defer resultServer.Close()

	var typedPrompt string
	var typedNegative string
	submitted := false
	browserStub := stubFallbackBrowser{
		openTab: func(_ context.Context, url string) (*browser.Tab, error) {
			submitted = false
			return &browser.Tab{TargetID: "tab-slide", URL: url, Title: url, Active: true}, nil
		},
		exists: func(_ context.Context, _ string, selector string) (bool, error) {
			switch selector {
			case ".ready", "textarea", "textarea.neg", "button.generate":
				return true, nil
			default:
				return false, nil
			}
		},
		act: func(_ context.Context, req *browser.ActRequest) (*browser.ActResponse, error) {
			if req.Kind == "type" && req.Selector == "textarea" {
				typedPrompt = req.Text
			}
			if req.Kind == "type" && req.Selector == "textarea.neg" {
				typedNegative = req.Text
			}
			if req.Kind == "click" {
				submitted = true
			}
			return &browser.ActResponse{Success: true}, nil
		},
		pageInfo: func(_ context.Context, _ string) (string, string, error) {
			if submitted {
				return resultServer.URL + "/final.png", resultServer.URL + "/final.png", nil
			}
			return "https://spaces.example/slide", "https://spaces.example/slide", nil
		},
	}

	engine := newFallbackEngineForTest(t, storage, nil, browserStub, FallbackConfig{
		PublicSpaces: []FallbackPublicSpacePreset{{
			ID:                      "slide-space",
			DisplayName:             "Slide Space",
			URL:                     "https://spaces.example/slide",
			Categories:              []MediaCategory{CategoryT2I},
			ReadySelectors:          []string{".ready"},
			PromptSelectors:         []string{"textarea"},
			NegativePromptSelectors: []string{"textarea.neg"},
			SubmitSelectors:         []string{"button.generate"},
			PollInterval:            20 * time.Millisecond,
			Timeout:                 time.Second,
		}},
	})

	result, err := engine.runSpacePreset(context.Background(), engine.config.PublicSpaces[0], &MediaRequest{
		Type:           MediaTypeImage,
		Prompt:         "帮我做一页PPT，标题：2026 产品战略；副标题：AI 驱动增长；要点：提升转化率；降低成本",
		NegativePrompt: "grain",
		Extra: map[string]any{
			"style_preset": "banana_slides",
			"aspect_ratio": "16:9",
		},
	}, nil, FallbackModelSpaceT2I)
	if err != nil {
		t.Fatalf("runSpacePreset returned error: %v", err)
	}
	if !strings.Contains(typedPrompt, "presentation-ready PPT slide visual") {
		t.Fatalf("wrapped prompt = %q", typedPrompt)
	}
	if !strings.Contains(typedPrompt, "Title: 2026 产品战略") {
		t.Fatalf("wrapped prompt missing title: %q", typedPrompt)
	}
	if !strings.Contains(typedNegative, "browser frame") || !strings.Contains(typedNegative, "grain") {
		t.Fatalf("wrapped negative prompt = %q", typedNegative)
	}
	if result.FallbackInfo == nil || result.FallbackInfo.RenderMode != "slide" {
		t.Fatalf("fallback_info = %#v, want slide", result.FallbackInfo)
	}
}

type categoryProvider struct {
	last  *MediaRequest
	model MediaModelInfo
}

func (p *categoryProvider) Name() string { return "real-provider" }
func (p *categoryProvider) SupportedModels() []MediaModelInfo {
	return []MediaModelInfo{p.model}
}
func (p *categoryProvider) SupportsType(t MediaType) bool { return t == p.model.Type }
func (p *categoryProvider) Generate(_ context.Context, req *MediaRequest) (*MediaTask, error) {
	cp := *req
	p.last = &cp
	return &MediaTask{
		BaseTask: basetask.BaseTask{Status: TaskStatusSucceeded, Progress: 1},
		Response: &MediaResponse{Created: time.Now().Unix(), Data: []MediaResult{{B64JSON: fakeMediaImagePNGBase64, ContentType: "image/png"}}},
	}, nil
}
func (p *categoryProvider) Poll(_ context.Context, _ string) (*MediaTask, error) {
	return nil, ErrTaskNotFound
}

func TestManagerPrefersRealProviderWhenAvailable(t *testing.T) {
	tmp := t.TempDir()
	storage := NewMediaStorage(tmp, "/api/media/generated")
	if err := storage.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	manager := NewManager(storage, nil, "")
	provider := &categoryProvider{
		model: MediaModelInfo{ID: "real-image", Name: "Real Image", Type: MediaTypeImage, Category: CategoryT2I, Provider: "real-provider"},
	}
	manager.RegisterProvider(provider)
	manager.SetFallbackEngine(NewFallbackEngine(FallbackConfig{Enabled: true}, storage, nil, func() FallbackBrowserService { return nil }, ""))

	task, err := manager.Generate(context.Background(), &MediaRequest{
		Type:   MediaTypeImage,
		Prompt: "draw a cat",
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	waitCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	task, err = manager.WaitForTask(waitCtx, task.ID)
	if err != nil {
		t.Fatalf("WaitForTask returned error: %v", err)
	}
	if provider.last == nil {
		t.Fatal("expected real provider to be called")
	}
	if task.Provider != "real-provider" {
		t.Fatalf("provider = %q, want real-provider", task.Provider)
	}
	if task.FallbackInfo != nil {
		t.Fatalf("fallback_info = %#v, want nil when real provider exists", task.FallbackInfo)
	}
}

func TestFallbackEngineExposesU2NetPStatus(t *testing.T) {
	engine := NewFallbackEngine(FallbackConfig{
		Enabled:         true,
		DataDir:         t.TempDir(),
		U2NetPStatusURL: "/api/v1/media/fallback/models/u2netp/status",
	}, nil, nil, func() FallbackBrowserService { return nil }, "")
	status, err := engine.GetFallbackModelStatus("u2netp")
	if err != nil {
		t.Fatalf("GetFallbackModelStatus returned error: %v", err)
	}
	if status == nil {
		t.Fatal("expected status")
	}
	if status.ModelID != "u2netp" {
		t.Fatalf("model_id = %q, want u2netp", status.ModelID)
	}
	if status.State != "not_downloaded" {
		t.Fatalf("state = %q, want not_downloaded", status.State)
	}
}

func TestFallbackEnginePrefersNativeVideoWhenAvailable(t *testing.T) {
	previous := nativeVideoOSAvailable
	nativeVideoOSAvailable = func() bool { return true }
	defer func() { nativeVideoOSAvailable = previous }()

	engine := NewFallbackEngine(FallbackConfig{
		Enabled: true,
		NativeVideo: FallbackNativeVideoConfig{
			Enabled: true,
		},
	}, nil, nil, func() FallbackBrowserService { return nil }, "zh-CN")
	engine.SetNativeVideoGenerator(stubNativeVideoGenerator{available: true})

	if got := engine.ModelForRequest(&MediaRequest{Type: MediaTypeVideo, Prompt: "做一个短视频"}, CategoryT2V); got != FallbackModelNativeT2V {
		t.Fatalf("ModelForRequest(t2v) = %q, want %q", got, FallbackModelNativeT2V)
	}
	if got := engine.ModelForRequest(&MediaRequest{Type: MediaTypeVideo, Prompt: "让图片动起来", ReferenceURLs: []string{"https://example.com/a.png"}}, CategoryI2V); got != FallbackModelNativeI2V {
		t.Fatalf("ModelForRequest(i2v) = %q, want %q", got, FallbackModelNativeI2V)
	}
	models := engine.SupportedModels()
	found := map[string]bool{}
	for _, model := range models {
		found[model.ID] = true
	}
	for _, modelID := range []string{FallbackModelNativeT2V, FallbackModelNativeI2V, FallbackModelNativeKF2V} {
		if !found[modelID] {
			t.Fatalf("SupportedModels missing %s", modelID)
		}
	}
}

func TestFallbackEngineFallsBackToPublicSpaceWhenNativeUnavailable(t *testing.T) {
	previous := nativeVideoOSAvailable
	nativeVideoOSAvailable = func() bool { return true }
	defer func() { nativeVideoOSAvailable = previous }()

	engine := NewFallbackEngine(FallbackConfig{
		Enabled: true,
		NativeVideo: FallbackNativeVideoConfig{
			Enabled: true,
		},
	}, nil, nil, func() FallbackBrowserService { return nil }, "zh-CN")

	if got := engine.ModelForRequest(&MediaRequest{Type: MediaTypeVideo, Prompt: "做一个短视频"}, CategoryT2V); got != FallbackModelSpaceT2V {
		t.Fatalf("ModelForRequest(t2v) = %q, want %q", got, FallbackModelSpaceT2V)
	}
	if got := engine.ModelForRequest(&MediaRequest{Type: MediaTypeVideo, Prompt: "做一个短视频", Model: FallbackModelNativeT2V}, CategoryT2V); got != FallbackModelSpaceT2V {
		t.Fatalf("ModelForRequest(native explicit) = %q, want %q", got, FallbackModelSpaceT2V)
	}
}

func TestFallbackAudioPlanHandlesCustomAutoAndSilentFallback(t *testing.T) {
	engine := NewFallbackEngine(FallbackConfig{
		Enabled: true,
		NativeVideo: FallbackNativeVideoConfig{
			Enabled:   true,
			AudioMode: "auto",
		},
	}, nil, nil, func() FallbackBrowserService { return nil }, "zh-CN")

	custom := engine.buildAudioPlan(&MediaRequest{
		Prompt: "做一个森林里的猫",
		Extra:  map[string]any{"narration": "这是自定义旁白"},
	}, CategoryT2V, nil)
	if custom.Narration != "这是自定义旁白" {
		t.Fatalf("custom narration = %q", custom.Narration)
	}
	if custom.EffectiveMode != "tts" {
		t.Fatalf("custom EffectiveMode = %q, want tts", custom.EffectiveMode)
	}

	auto := engine.buildAudioPlan(&MediaRequest{
		Prompt: "一只猫在森林里漫步",
	}, CategoryT2V, &scenecompose.ScenePlan{
		Background: "forest",
		Foreground: []scenecompose.ForegroundPlan{{Type: "cat"}},
	})
	if strings.TrimSpace(auto.Narration) == "" {
		t.Fatal("expected auto narration")
	}

	audioPath, err := engine.materializeAudioPlan(context.Background(), t.TempDir(), &auto)
	if err != nil {
		t.Fatalf("materializeAudioPlan returned error: %v", err)
	}
	if audioPath != "" {
		t.Fatalf("audioPath = %q, want empty when TTS is unavailable", audioPath)
	}
	if auto.EffectiveMode != "silent" {
		t.Fatalf("EffectiveMode = %q, want silent", auto.EffectiveMode)
	}
}

func TestManagerCreateTaskNativeFallbackCompletes(t *testing.T) {
	previous := nativeVideoOSAvailable
	nativeVideoOSAvailable = func() bool { return true }
	defer func() { nativeVideoOSAvailable = previous }()

	tmp := t.TempDir()
	storage := NewMediaStorage(tmp, "/api/media/generated")
	if err := storage.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs returned error: %v", err)
	}
	engine := newFallbackEngineForTest(t, storage, nil, nil, FallbackConfig{
		Enabled: true,
		NativeVideo: FallbackNativeVideoConfig{
			Enabled:            true,
			PollInterval:       10 * time.Millisecond,
			StallTimeout:       time.Minute,
			MaxRuntime:         time.Minute,
			DefaultDurationSec: 2,
			MaxDurationSec:     2,
			AudioMode:          "silent",
		},
	})
	engine.SetNativeVideoGenerator(stubNativeVideoGenerator{
		available: true,
		generate: func(ctx context.Context, job *videogen.Job, pollInterval, stallTimeout, maxRuntime time.Duration, observer videogen.ProgressObserver) (*videogen.Result, error) {
			if observer != nil {
				observer(videogen.Progress{Stage: "render_video", Progress: 0.7, Message: "rendering", UpdatedAt: time.Now()})
			}
			if err := os.WriteFile(job.OutputPath, []byte("fake mp4 payload"), 0644); err != nil {
				return nil, err
			}
			_, thumbBytes, ok := parseDataURLPayload("data:image/png;base64," + fakeMediaImagePNGBase64)
			if !ok {
				t.Fatal("failed to decode fake thumbnail payload")
			}
			if err := os.WriteFile(job.ThumbnailPath, thumbBytes, 0644); err != nil {
				return nil, err
			}
			return &videogen.Result{
				OutputPath:    job.OutputPath,
				ThumbnailPath: job.ThumbnailPath,
				DurationSec:   job.DurationSec,
			}, nil
		},
	})

	manager := NewManager(storage, nil, "zh-CN")
	manager.SetFallbackEngine(engine)

	task, err := manager.CreateTask(context.Background(), &MediaRequest{
		Type:     MediaTypeVideo,
		Prompt:   "做一个蓝色抽象动态背景",
		Model:    FallbackModelNativeT2V,
		Size:     "640x360",
		Duration: 2,
		Extra:    map[string]any{"audio_mode": "silent"},
	}, "", string(CategoryT2V), "web")
	if err != nil {
		t.Fatalf("CreateTask returned error: %v", err)
	}

	waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	task, err = manager.WaitForTask(waitCtx, task.ID)
	if err != nil {
		t.Fatalf("WaitForTask returned error: %v", err)
	}
	if task.Status != TaskStatusSucceeded {
		t.Fatalf("status = %q, want succeeded", task.Status)
	}
	if task.FallbackInfo == nil || task.FallbackInfo.Strategy != FallbackStrategyNativeVideo {
		t.Fatalf("fallback_info = %#v, want native_timeline", task.FallbackInfo)
	}
	if task.Response == nil || len(task.Response.Data) != 1 {
		t.Fatalf("response = %#v, want one result", task.Response)
	}
	if strings.TrimSpace(task.Response.Data[0].URL) == "" {
		t.Fatalf("result url = %q, want stored local URL", task.Response.Data[0].URL)
	}
	if strings.TrimSpace(task.Response.Data[0].ThumbnailURL) == "" {
		t.Fatalf("thumbnail_url = %q, want stored thumbnail URL", task.Response.Data[0].ThumbnailURL)
	}
}

func TestRunSpacePresetDoesNotFailOnLegacyShortTimeoutWhenProgressContinues(t *testing.T) {
	tmp := t.TempDir()
	storage := NewMediaStorage(tmp, "/api/media/generated")
	if err := storage.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	resultServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write([]byte("video"))
	}))
	defer resultServer.Close()

	startedAt := time.Now()
	submitted := false
	browserStub := stubFallbackBrowser{
		exists: func(_ context.Context, _ string, selector string) (bool, error) {
			switch selector {
			case ".ready", "textarea", "button.generate", ".still-processing":
				return true, nil
			default:
				return false, nil
			}
		},
		act: func(_ context.Context, req *browser.ActRequest) (*browser.ActResponse, error) {
			if req.Kind == "click" {
				submitted = true
				startedAt = time.Now()
			}
			return &browser.ActResponse{Success: true}, nil
		},
		pageInfo: func(_ context.Context, _ string) (string, string, error) {
			if submitted && time.Since(startedAt) > 120*time.Millisecond {
				return resultServer.URL + "/late.mp4", resultServer.URL + "/late.mp4", nil
			}
			return "https://spaces.example/native-wait", "https://spaces.example/native-wait", nil
		},
	}

	engine := newFallbackEngineForTest(t, storage, nil, browserStub, FallbackConfig{
		PublicSpaces: []FallbackPublicSpacePreset{{
			ID:                  "slow-space",
			DisplayName:         "Slow Space",
			URL:                 "https://spaces.example/native-wait",
			Categories:          []MediaCategory{CategoryT2V},
			ReadySelectors:      []string{".ready"},
			PromptSelectors:     []string{"textarea"},
			SubmitSelectors:     []string{"button.generate"},
			ProcessingSelectors: []string{".still-processing"},
			PollInterval:        20 * time.Millisecond,
			Timeout:             50 * time.Millisecond,
			StallTimeout:        400 * time.Millisecond,
			MaxRuntime:          2 * time.Second,
		}},
	})

	result, err := engine.runSpacePreset(context.Background(), engine.config.PublicSpaces[0], &MediaRequest{
		Type:   MediaTypeVideo,
		Prompt: "做一个慢一点的视频",
	}, nil, FallbackModelSpaceT2V)
	if err != nil {
		t.Fatalf("runSpacePreset returned error: %v", err)
	}
	if result == nil || result.Response == nil || len(result.Response.Data) == 0 {
		t.Fatalf("result = %#v, want stored video response", result)
	}
}
