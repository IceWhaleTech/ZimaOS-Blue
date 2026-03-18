package mediagen

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
	basetask "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/task"
	"github.com/labstack/echo/v4"
)

type stubFallbackSearcher struct {
	results []FallbackSearchResult
	err     error
}

func (s stubFallbackSearcher) Search(_ context.Context, _ string, _ int, _ []string) ([]FallbackSearchResult, error) {
	if s.err != nil {
		return nil, s.err
	}
	return append([]FallbackSearchResult(nil), s.results...), nil
}

type stubFallbackBrowser struct {
	screenshot func(ctx context.Context, req *browser.ScreenshotRequest) (*browser.ScreenshotResponse, error)
	openTab    func(ctx context.Context, url string) (*browser.Tab, error)
	closeTab   func(ctx context.Context, targetID string) error
	exists     func(ctx context.Context, targetID, selector string) (bool, error)
	extract    func(ctx context.Context, targetID, selector, attribute string) (string, error)
	act        func(ctx context.Context, req *browser.ActRequest) (*browser.ActResponse, error)
	pageInfo   func(ctx context.Context, targetID string) (string, string, error)
}

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
	return NewFallbackEngine(cfg, storage, searcher, func() FallbackBrowserService { return browserSvc }, "zh-CN")
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
	var screenshotURL string
	browserStub := stubFallbackBrowser{
		screenshot: func(_ context.Context, req *browser.ScreenshotRequest) (*browser.ScreenshotResponse, error) {
			screenshotURL = req.URL
			return &browser.ScreenshotResponse{Data: fakeMediaImagePNGBase64, Format: browser.FormatPNG}, nil
		},
	}
	engine := newFallbackEngineForTest(t, storage, stubFallbackSearcher{
		results: []FallbackSearchResult{{Title: "Cat Article", URL: imageServer.URL + "/article"}},
	}, browserStub, FallbackConfig{RenderBaseURL: "http://127.0.0.1:8899"})
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
	if !strings.Contains(screenshotURL, "/api/v1/media/fallback/render/") {
		t.Fatalf("screenshot URL = %q, want fallback render route", screenshotURL)
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
	}, stubFallbackBrowser{
		screenshot: func(_ context.Context, req *browser.ScreenshotRequest) (*browser.ScreenshotResponse, error) {
			return &browser.ScreenshotResponse{Data: fakeMediaImagePNGBase64, Format: browser.FormatPNG}, nil
		},
	}, FallbackConfig{}))

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
