package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/smallmodel"
)

type imageReviewMock struct {
	args map[string]interface{}
	resp interface{}
	err  error
}

func (m *imageReviewMock) Execute(_ context.Context, args map[string]interface{}) (interface{}, error) {
	m.args = args
	if m.resp == nil {
		m.resp = map[string]interface{}{"ok": true}
	}
	return m.resp, m.err
}

type imageVisionMock struct {
	prompt    string
	image     string
	resp      string
	responses []string
	calls     int
	err       error
}

func (m *imageVisionMock) ChatWithVision(_ context.Context, prompt string, imageBase64 string) (string, error) {
	m.prompt = prompt
	m.image = imageBase64
	if m.err != nil {
		return "", m.err
	}
	if m.calls < len(m.responses) {
		resp := m.responses[m.calls]
		m.calls++
		return resp, nil
	}
	if m.resp == "" {
		m.resp = "vision ok"
	}
	m.calls++
	return m.resp, nil
}

type imageProviderVisionMock struct {
	prompt   string
	image    string
	analysis string
	handled  bool
	calls    int
	err      error
}

func (m *imageProviderVisionMock) Analyze(_ context.Context, prompt string, imageBase64 string) (string, bool, error) {
	m.calls++
	m.prompt = prompt
	m.image = imageBase64
	return m.analysis, m.handled, m.err
}

type imageOCRMock struct {
	image      []byte
	calls      int
	resp       ImageOCRResult
	allowEmpty bool
	err        error
}

type imageSmallModelMock struct {
	resp    string
	err     error
	ready   bool
	calls   int
	lastReq smallmodel.GenerateRequest
}

type pptGenerateMock struct {
	req  PPTRequest
	resp *PPTResult
	err  error
}

func (m *pptGenerateMock) Generate(_ context.Context, req PPTRequest) (*PPTResult, error) {
	m.req = req
	if m.err != nil {
		return nil, m.err
	}
	if m.resp == nil {
		m.resp = &PPTResult{Status: "succeeded", TaskID: "slide-1", ImageURLs: []string{"/api/media/generated/images/slide.png"}}
	}
	return m.resp, nil
}

func (m *imageOCRMock) Extract(_ context.Context, imagePNG []byte) (ImageOCRResult, error) {
	m.calls++
	m.image = append([]byte(nil), imagePNG...)
	if m.resp.Text == "" && m.err == nil && !m.allowEmpty {
		m.resp = ImageOCRResult{Text: "ocr text", Engine: "tesseract/wasm", Model: "eng"}
	}
	return m.resp, m.err
}

func (m *imageSmallModelMock) Generate(_ context.Context, req smallmodel.GenerateRequest) (*smallmodel.GenerateResponse, error) {
	m.calls++
	m.lastReq = req
	if m.err != nil {
		return nil, m.err
	}
	return &smallmodel.GenerateResponse{Text: m.resp}, nil
}

func (m *imageSmallModelMock) Ready() bool {
	return m.ready
}

func TestImageDefinition_UsesGroupedWorkflowSchema(t *testing.T) {
	tool := NewImageTool(nil, nil, nil)
	def := tool.Definition()
	props, ok := def.Parameters["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("properties = %#v, want object", def.Parameters["properties"])
	}
	for _, key := range []string{"task", "generate", "review", "slide"} {
		if _, ok := props[key]; !ok {
			t.Fatalf("expected grouped property %q", key)
		}
	}
	for _, legacy := range []string{"task_id", "reference_image", "reference_base64", "url", "urls", "image", "images", "negative_prompt", "wait_timeout_sec"} {
		if _, ok := props[legacy]; ok {
			t.Fatalf("legacy top-level property %q should be hidden", legacy)
		}
	}
}

func TestNormalizeImageArgs_FlattensSlideOptionsAndLanguage(t *testing.T) {
	args := normalizeImageArgs(map[string]interface{}{
		"language": "zh-CN",
		"slide": map[string]interface{}{
			"reference_images":    []interface{}{"https://example.com/a.png"},
			"layout_spec":         map[string]interface{}{"kind": "cover"},
			"style_preset":        "bananaslides",
			"quality_profile":     "ppt",
			"review_threshold":    0.8,
			"review_retry_budget": 2,
			"theme":               "brand",
		},
	})
	if got := asString(args["lang"]); got != "zh-CN" {
		t.Fatalf("lang = %q, want zh-CN", got)
	}
	if got := asString(args["style_preset"]); got != "bananaslides" {
		t.Fatalf("style_preset = %q, want bananaslides", got)
	}
	if got := asString(args["quality_profile"]); got != "ppt" {
		t.Fatalf("quality_profile = %q, want ppt", got)
	}
	if got := asString(args["style_theme"]); got != "brand" {
		t.Fatalf("style_theme = %q, want brand", got)
	}
	if got := args["review_retry_budget"]; got != 2 {
		t.Fatalf("review_retry_budget = %#v, want 2", got)
	}
	if _, ok := args["reference_images"]; !ok {
		t.Fatal("expected reference_images to be flattened")
	}
	if _, ok := args["layout_spec"]; !ok {
		t.Fatal("expected layout_spec to be flattened")
	}
}

func TestNormalizeImageArgs_FlattensGroupedWorkflowOptions(t *testing.T) {
	args := normalizeImageArgs(map[string]interface{}{
		"task": map[string]interface{}{
			"id": "task-7",
		},
		"generate": map[string]interface{}{
			"model": "demo-model",
			"output": map[string]interface{}{
				"path":  "art.png",
				"count": 2,
				"size":  "1024x1024",
			},
			"reference": map[string]interface{}{
				"url":    "https://example.com/ref.png",
				"base64": "abc123",
			},
			"wait": map[string]interface{}{
				"enabled":     false,
				"timeout_sec": 45,
			},
			"options": map[string]interface{}{
				"negative":   "blurry",
				"quality":    "high",
				"style":      "illustration",
				"mode":       "i2i",
				"aspect":     "16:9",
				"resolution": "hd",
			},
		},
		"review": map[string]interface{}{
			"inputs": map[string]interface{}{
				"url":   "https://example.com/page",
				"paths": []interface{}{"/tmp/a.png"},
			},
			"compare":    true,
			"max_images": 3,
			"mode":       "cheap",
			"language":   "en-US",
			"options": map[string]interface{}{
				"device":    "mobile",
				"wait_ms":   1200,
				"threshold": 0.7,
				"format":    "json",
			},
		},
		"slide": map[string]interface{}{
			"review": map[string]interface{}{
				"threshold":    0.8,
				"retry_budget": 2,
			},
		},
	})

	if got := asString(args["task_id"]); got != "task-7" {
		t.Fatalf("task_id = %q, want task-7", got)
	}
	if got := asString(args["model"]); got != "demo-model" {
		t.Fatalf("model = %q, want demo-model", got)
	}
	if got := asString(args["path"]); got != "art.png" {
		t.Fatalf("path = %q, want art.png", got)
	}
	if got := compatInt(args, "count"); got != 2 {
		t.Fatalf("count = %d, want 2", got)
	}
	if got := asString(args["size"]); got != "1024x1024" {
		t.Fatalf("size = %q, want 1024x1024", got)
	}
	if got := asString(args["reference_image"]); got != "https://example.com/ref.png" {
		t.Fatalf("reference_image = %q", got)
	}
	if got := asString(args["reference_base64"]); got != "abc123" {
		t.Fatalf("reference_base64 = %q", got)
	}
	if got, ok := args["poll"].(bool); !ok || got {
		t.Fatalf("poll = %#v, want false", args["poll"])
	}
	if got := compatInt(args, "wait_timeout_sec"); got != 45 {
		t.Fatalf("wait_timeout_sec = %d, want 45", got)
	}
	if got := asString(args["negative_prompt"]); got != "blurry" {
		t.Fatalf("negative_prompt = %q", got)
	}
	if got := asString(args["quality"]); got != "high" {
		t.Fatalf("quality = %q", got)
	}
	if got := asString(args["style"]); got != "illustration" {
		t.Fatalf("style = %q", got)
	}
	if got := asString(args["category"]); got != "i2i" {
		t.Fatalf("category = %q", got)
	}
	if got := asString(args["aspect_ratio"]); got != "16:9" {
		t.Fatalf("aspect_ratio = %q", got)
	}
	if got := asString(args["resolution"]); got != "hd" {
		t.Fatalf("resolution = %q", got)
	}
	if got := asString(args["url"]); got != "https://example.com/page" {
		t.Fatalf("url = %q", got)
	}
	if got := args["image_paths"]; got == nil {
		t.Fatal("expected image_paths to be flattened")
	}
	if got, ok := args["compare"].(bool); !ok || !got {
		t.Fatalf("compare = %#v, want true", args["compare"])
	}
	if got := compatInt(args, "max_images"); got != 3 {
		t.Fatalf("max_images = %d, want 3", got)
	}
	if got := asString(args["analysis_mode"]); got != "cheap" {
		t.Fatalf("analysis_mode = %q, want cheap", got)
	}
	if got := asString(args["lang"]); got != "en-US" {
		t.Fatalf("lang = %q, want en-US", got)
	}
	if got := asString(args["device"]); got != "mobile" {
		t.Fatalf("device = %q, want mobile", got)
	}
	if got := compatFloat64(args, "threshold"); got != 0.7 {
		t.Fatalf("threshold = %v, want 0.7", got)
	}
	if got := asString(args["format"]); got != "json" {
		t.Fatalf("format = %q, want json", got)
	}
	if got := compatInt(args, "wait_ms"); got != 1200 {
		t.Fatalf("wait_ms = %d, want 1200", got)
	}
	if got := compatFloat64(args, "review_threshold"); got != 0.8 {
		t.Fatalf("review_threshold = %v, want 0.8", got)
	}
	if got := compatInt(args, "review_retry_budget"); got != 2 {
		t.Fatalf("review_retry_budget = %d, want 2", got)
	}
}

func TestImageToolReviewFallsBackToOCR(t *testing.T) {
	vision := &imageVisionMock{err: errors.New("vision unavailable")}
	ocr := &imageOCRMock{resp: ImageOCRResult{Text: "hello from ocr", Engine: "tesseract/wasm", Model: "eng"}}
	tool := NewImageTool(nil, nil, nil)
	tool.SetVisionBridge(vision)
	tool.SetOCRService(ocr)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"image": inlinePNGBase64(t),
	})
	if err != nil {
		t.Fatalf("execute ocr fallback failed: %v", err)
	}
	payload := result.(map[string]interface{})
	if payload["mode"] != "ocr" {
		t.Fatalf("mode = %v, want ocr", payload["mode"])
	}
	if payload["analysis"] != "hello from ocr" {
		t.Fatalf("analysis = %v", payload["analysis"])
	}
	if payload["fallback_from"] != "vision" {
		t.Fatalf("fallback_from = %v", payload["fallback_from"])
	}
}

func TestImageToolReviewUsesProviderAwareVisionForMiniMax(t *testing.T) {
	vision := &imageVisionMock{resp: "generic vision should not run"}
	providerVision := &imageProviderVisionMock{handled: true, analysis: "minimax direct vision"}
	tool := NewImageTool(nil, nil, nil)
	tool.SetVisionBridge(vision)
	tool.SetProviderVision(providerVision)

	ctx := WithProviderID(context.Background(), "minimax")
	result, err := tool.Execute(ctx, map[string]interface{}{
		"image":  inlinePNGBase64(t),
		"prompt": "What is shown?",
	})
	if err != nil {
		t.Fatalf("execute minimax direct vision failed: %v", err)
	}
	payload := result.(map[string]interface{})
	if payload["mode"] != "vision" {
		t.Fatalf("mode = %v, want vision", payload["mode"])
	}
	if payload["analysis"] != "minimax direct vision" {
		t.Fatalf("analysis = %v, want minimax direct vision", payload["analysis"])
	}
	if providerVision.calls != 1 {
		t.Fatalf("provider vision calls = %d, want 1", providerVision.calls)
	}
	if vision.calls != 0 {
		t.Fatalf("generic vision calls = %d, want 0", vision.calls)
	}
}

func TestImageToolReviewNonMiniMaxKeepsGenericVisionPath(t *testing.T) {
	vision := &imageVisionMock{resp: "generic vision"}
	providerVision := &imageProviderVisionMock{handled: false, analysis: "minimax direct vision"}
	tool := NewImageTool(nil, nil, nil)
	tool.SetVisionBridge(vision)
	tool.SetProviderVision(providerVision)

	ctx := WithProviderID(context.Background(), "openai")
	result, err := tool.Execute(ctx, map[string]interface{}{
		"image":  inlinePNGBase64(t),
		"prompt": "What is shown?",
	})
	if err != nil {
		t.Fatalf("execute generic vision failed: %v", err)
	}
	payload := result.(map[string]interface{})
	if payload["analysis"] != "generic vision" {
		t.Fatalf("analysis = %v, want generic vision", payload["analysis"])
	}
	if providerVision.calls != 1 {
		t.Fatalf("provider vision calls = %d, want 1", providerVision.calls)
	}
	if vision.calls != 1 {
		t.Fatalf("generic vision calls = %d, want 1", vision.calls)
	}
}

func TestImageToolReviewOCROnlyBypassesVision(t *testing.T) {
	vision := &imageVisionMock{resp: "vision should not run"}
	ocr := &imageOCRMock{resp: ImageOCRResult{Text: "ocr only text", Engine: "tesseract/wasm", Model: "eng"}}
	tool := NewImageTool(nil, nil, nil)
	tool.SetVisionBridge(vision)
	tool.SetOCRService(ocr)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"image":         inlinePNGBase64(t),
		"analysis_mode": "ocr_only",
	})
	if err != nil {
		t.Fatalf("execute ocr_only review failed: %v", err)
	}
	payload := result.(map[string]interface{})
	if payload["mode"] != "ocr" {
		t.Fatalf("mode = %v, want ocr", payload["mode"])
	}
	if vision.calls != 0 {
		t.Fatalf("vision calls = %d, want 0", vision.calls)
	}
}

func TestImageToolReviewOCRFirstUsesOCRBeforeVision(t *testing.T) {
	vision := &imageVisionMock{resp: "vision should not run"}
	ocr := &imageOCRMock{resp: ImageOCRResult{Text: "Revenue 18%\nARR 120k\nActive users 2400", Engine: "tesseract/wasm", Model: "eng"}}
	tool := NewImageTool(nil, nil, nil)
	tool.SetVisionBridge(vision)
	tool.SetOCRService(ocr)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"image":         inlinePNGBase64(t),
		"prompt":        "Extract the visible text from this dashboard screenshot.",
		"analysis_mode": "ocr_first",
	})
	if err != nil {
		t.Fatalf("execute ocr_first review failed: %v", err)
	}
	payload := result.(map[string]interface{})
	if payload["mode"] != "ocr" {
		t.Fatalf("mode = %v, want ocr", payload["mode"])
	}
	if payload["ocr_sufficient"] != true {
		t.Fatalf("ocr_sufficient = %v, want true", payload["ocr_sufficient"])
	}
	if vision.calls != 0 {
		t.Fatalf("vision calls = %d, want 0", vision.calls)
	}
}

func TestImageToolReviewOCRFirstFallsBackToVisionWhenOCRIsSparse(t *testing.T) {
	vision := &imageVisionMock{resp: "The image shows an analytics dashboard with KPI cards and a line chart."}
	ocr := &imageOCRMock{resp: ImageOCRResult{Text: "Q3", Engine: "tesseract/wasm", Model: "eng"}}
	tool := NewImageTool(nil, nil, nil)
	tool.SetVisionBridge(vision)
	tool.SetOCRService(ocr)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"image":         inlinePNGBase64(t),
		"prompt":        "What is shown in this dashboard screenshot?",
		"analysis_mode": "ocr_first",
	})
	if err != nil {
		t.Fatalf("execute sparse-ocr review failed: %v", err)
	}
	payload := result.(map[string]interface{})
	if payload["mode"] != "vision" {
		t.Fatalf("mode = %v, want vision", payload["mode"])
	}
	if payload["fallback_from"] != "ocr" {
		t.Fatalf("fallback_from = %v, want ocr", payload["fallback_from"])
	}
	if payload["fallback_reason"] != "ocr_insufficient" {
		t.Fatalf("fallback_reason = %v, want ocr_insufficient", payload["fallback_reason"])
	}
	if payload["ocr_sufficient"] != false {
		t.Fatalf("ocr_sufficient = %v, want false", payload["ocr_sufficient"])
	}
	if payload["ocr_preview"] != "Q3" {
		t.Fatalf("ocr_preview = %v, want Q3", payload["ocr_preview"])
	}
	if vision.calls != 1 {
		t.Fatalf("vision calls = %d, want 1", vision.calls)
	}
}

func TestImageToolReviewOCRFirstUsesSmallModelBeforeVisionWhenOCRIsSparse(t *testing.T) {
	vision := &imageVisionMock{resp: "vision should not run"}
	ocr := &imageOCRMock{resp: ImageOCRResult{Text: "Q3", Engine: "tesseract/wasm", Model: "eng"}}
	sm := &imageSmallModelMock{ready: true, resp: "A KPI dashboard with cards and a line chart."}
	tool := NewImageTool(nil, nil, nil)
	tool.SetVisionBridge(vision)
	tool.SetOCRService(ocr)
	tool.SetSmallModelRuntime(sm)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"image":         inlinePNGBase64(t),
		"prompt":        "What is shown in this dashboard screenshot?",
		"analysis_mode": "ocr_first",
	})
	if err != nil {
		t.Fatalf("execute sparse-ocr with small model failed: %v", err)
	}
	payload := result.(map[string]interface{})
	if payload["mode"] != "small_model" {
		t.Fatalf("mode = %v, want small_model", payload["mode"])
	}
	if payload["fallback_from"] != "ocr" {
		t.Fatalf("fallback_from = %v, want ocr", payload["fallback_from"])
	}
	if sm.calls != 1 {
		t.Fatalf("small model calls = %d, want 1", sm.calls)
	}
	if len(sm.lastReq.Images) != 1 {
		t.Fatalf("small model images = %d, want 1", len(sm.lastReq.Images))
	}
	if vision.calls != 0 {
		t.Fatalf("vision calls = %d, want 0", vision.calls)
	}
}

func TestImageToolReviewOCRFirstFallsBackToVisionWhenSmallModelIsTooSparse(t *testing.T) {
	vision := &imageVisionMock{resp: "The screenshot shows a KPI dashboard with cards and a line chart."}
	ocr := &imageOCRMock{resp: ImageOCRResult{Text: "Q3", Engine: "tesseract/wasm", Model: "eng"}}
	sm := &imageSmallModelMock{ready: true, resp: "unclear screenshot"}
	tool := NewImageTool(nil, nil, nil)
	tool.SetVisionBridge(vision)
	tool.SetOCRService(ocr)
	tool.SetSmallModelRuntime(sm)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"image":         inlinePNGBase64(t),
		"prompt":        "What is shown in this dashboard screenshot?",
		"analysis_mode": "ocr_first",
	})
	if err != nil {
		t.Fatalf("execute sparse-ocr with sparse small model failed: %v", err)
	}
	payload := result.(map[string]interface{})
	if payload["mode"] != "vision" {
		t.Fatalf("mode = %v, want vision", payload["mode"])
	}
	if payload["fallback_from"] != "small_model" {
		t.Fatalf("fallback_from = %v, want small_model", payload["fallback_from"])
	}
	if payload["fallback_reason"] != "small_model_insufficient_after_ocr" {
		t.Fatalf("fallback_reason = %v, want small_model_insufficient_after_ocr", payload["fallback_reason"])
	}
	if payload["small_model_sufficient"] != false {
		t.Fatalf("small_model_sufficient = %v, want false", payload["small_model_sufficient"])
	}
	if payload["small_model_preview"] != "unclear screenshot" {
		t.Fatalf("small_model_preview = %v, want unclear screenshot", payload["small_model_preview"])
	}
	if vision.calls != 1 {
		t.Fatalf("vision calls = %d, want 1", vision.calls)
	}
}

func TestImageToolReviewOCRFirstFallsBackToVision(t *testing.T) {
	vision := &imageVisionMock{resp: "vision rescue"}
	ocr := &imageOCRMock{resp: ImageOCRResult{}, allowEmpty: true}
	tool := NewImageTool(nil, nil, nil)
	tool.SetVisionBridge(vision)
	tool.SetOCRService(ocr)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"image":         inlinePNGBase64(t),
		"analysis_mode": "ocr_first",
	})
	if err != nil {
		t.Fatalf("execute ocr_first fallback review failed: %v", err)
	}
	payload := result.(map[string]interface{})
	if payload["mode"] != "vision" {
		t.Fatalf("mode = %v, want vision", payload["mode"])
	}
	if payload["fallback_from"] != "ocr" {
		t.Fatalf("fallback_from = %v, want ocr", payload["fallback_from"])
	}
	if vision.calls != 1 {
		t.Fatalf("vision calls = %d, want 1", vision.calls)
	}
}

func TestImageToolReviewOCRFirstReturnsOCRWhenVisionUnavailableAfterSparseOCR(t *testing.T) {
	ocr := &imageOCRMock{resp: ImageOCRResult{Text: "Q3", Engine: "tesseract/wasm", Model: "eng"}}
	tool := NewImageTool(nil, nil, nil)
	tool.SetOCRService(ocr)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"image":         inlinePNGBase64(t),
		"prompt":        "What is shown in this dashboard screenshot?",
		"analysis_mode": "ocr_first",
	})
	if err != nil {
		t.Fatalf("execute sparse-ocr without vision failed: %v", err)
	}
	payload := result.(map[string]interface{})
	if payload["mode"] != "ocr" {
		t.Fatalf("mode = %v, want ocr", payload["mode"])
	}
	if payload["ocr_sufficient"] != false {
		t.Fatalf("ocr_sufficient = %v, want false", payload["ocr_sufficient"])
	}
	if payload["fallback_reason"] != "vision_unavailable_after_insufficient_ocr" {
		t.Fatalf("fallback_reason = %v, want vision_unavailable_after_insufficient_ocr", payload["fallback_reason"])
	}
	warnings, ok := payload["warnings"].([]string)
	if !ok || len(warnings) == 0 {
		t.Fatalf("warnings = %#v, want non-empty []string", payload["warnings"])
	}
}

func TestImageToolReviewOCRFirstReturnsOCRWhenMiniMaxDirectVisionFails(t *testing.T) {
	vision := &imageVisionMock{resp: "generic vision should not run"}
	providerVision := &imageProviderVisionMock{handled: true, err: errors.New("minimax direct vision failed")}
	ocr := &imageOCRMock{resp: ImageOCRResult{Text: "Q3", Engine: "tesseract/wasm", Model: "eng"}}
	tool := NewImageTool(nil, nil, nil)
	tool.SetVisionBridge(vision)
	tool.SetProviderVision(providerVision)
	tool.SetOCRService(ocr)

	ctx := WithProviderID(context.Background(), "minimax")
	result, err := tool.Execute(ctx, map[string]interface{}{
		"image":         inlinePNGBase64(t),
		"prompt":        "What is shown in this dashboard screenshot?",
		"analysis_mode": "ocr_first",
	})
	if err != nil {
		t.Fatalf("execute ocr_first minimax failure fallback failed: %v", err)
	}
	payload := result.(map[string]interface{})
	if payload["mode"] != "ocr" {
		t.Fatalf("mode = %v, want ocr", payload["mode"])
	}
	if payload["fallback_reason"] != "vision_unavailable_after_insufficient_ocr" {
		t.Fatalf("fallback_reason = %v, want vision_unavailable_after_insufficient_ocr", payload["fallback_reason"])
	}
	if providerVision.calls != 1 {
		t.Fatalf("provider vision calls = %d, want 1", providerVision.calls)
	}
	if vision.calls != 0 {
		t.Fatalf("generic vision calls = %d, want 0", vision.calls)
	}
}

func TestImageToolReviewCheapFirstUsesSmallModel(t *testing.T) {
	vision := &imageVisionMock{resp: "vision should not run"}
	sm := &imageSmallModelMock{ready: true, resp: "A product hero image with a server device."}
	tool := NewImageTool(nil, nil, nil)
	tool.SetVisionBridge(vision)
	tool.SetSmallModelRuntime(sm)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"image":         inlinePNGBase64(t),
		"prompt":        "Describe the product image briefly.",
		"analysis_mode": "cheap_first",
	})
	if err != nil {
		t.Fatalf("execute cheap_first review failed: %v", err)
	}
	payload := result.(map[string]interface{})
	if payload["mode"] != "small_model" {
		t.Fatalf("mode = %v, want small_model", payload["mode"])
	}
	if sm.calls != 1 {
		t.Fatalf("small model calls = %d, want 1", sm.calls)
	}
	if vision.calls != 0 {
		t.Fatalf("vision calls = %d, want 0", vision.calls)
	}
}

func TestImageToolReviewCheapFirstFallsBackToVision(t *testing.T) {
	vision := &imageVisionMock{resp: "vision rescue"}
	sm := &imageSmallModelMock{ready: false}
	tool := NewImageTool(nil, nil, nil)
	tool.SetVisionBridge(vision)
	tool.SetSmallModelRuntime(sm)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"image":         inlinePNGBase64(t),
		"prompt":        "Describe the product image briefly.",
		"analysis_mode": "cheap_first",
	})
	if err != nil {
		t.Fatalf("execute cheap_first fallback failed: %v", err)
	}
	payload := result.(map[string]interface{})
	if payload["mode"] != "vision" {
		t.Fatalf("mode = %v, want vision", payload["mode"])
	}
	if payload["fallback_from"] != "small_model" {
		t.Fatalf("fallback_from = %v, want small_model", payload["fallback_from"])
	}
	if vision.calls != 1 {
		t.Fatalf("vision calls = %d, want 1", vision.calls)
	}
}

func TestImageToolReviewCheapFirstUsesOCRWhenMiniMaxDirectVisionFails(t *testing.T) {
	vision := &imageVisionMock{resp: "generic vision should not run"}
	providerVision := &imageProviderVisionMock{handled: true, err: errors.New("minimax direct vision failed")}
	ocr := &imageOCRMock{resp: ImageOCRResult{Text: "ocr fallback text", Engine: "tesseract/wasm", Model: "eng"}}
	sm := &imageSmallModelMock{ready: false}
	tool := NewImageTool(nil, nil, nil)
	tool.SetVisionBridge(vision)
	tool.SetProviderVision(providerVision)
	tool.SetOCRService(ocr)
	tool.SetSmallModelRuntime(sm)

	ctx := WithProviderID(context.Background(), "minimax")
	result, err := tool.Execute(ctx, map[string]interface{}{
		"image":         inlinePNGBase64(t),
		"prompt":        "Describe the product image briefly.",
		"analysis_mode": "cheap_first",
	})
	if err != nil {
		t.Fatalf("execute cheap_first minimax fallback failed: %v", err)
	}
	payload := result.(map[string]interface{})
	if payload["mode"] != "ocr" {
		t.Fatalf("mode = %v, want ocr", payload["mode"])
	}
	if payload["fallback_from"] != "vision" {
		t.Fatalf("fallback_from = %v, want vision", payload["fallback_from"])
	}
	if providerVision.calls != 1 {
		t.Fatalf("provider vision calls = %d, want 1", providerVision.calls)
	}
	if vision.calls != 0 {
		t.Fatalf("generic vision calls = %d, want 0", vision.calls)
	}
}

func TestImageToolReviewCheapFirstFallsBackToVisionWhenSmallModelIsTooSparse(t *testing.T) {
	vision := &imageVisionMock{resp: "A product hero image with a server device on a clean backdrop."}
	sm := &imageSmallModelMock{ready: true, resp: "unclear"}
	tool := NewImageTool(nil, nil, nil)
	tool.SetVisionBridge(vision)
	tool.SetSmallModelRuntime(sm)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"image":         inlinePNGBase64(t),
		"prompt":        "Describe the product image briefly.",
		"analysis_mode": "cheap_first",
	})
	if err != nil {
		t.Fatalf("execute cheap_first sparse-small-model fallback failed: %v", err)
	}
	payload := result.(map[string]interface{})
	if payload["mode"] != "vision" {
		t.Fatalf("mode = %v, want vision", payload["mode"])
	}
	if payload["fallback_from"] != "small_model" {
		t.Fatalf("fallback_from = %v, want small_model", payload["fallback_from"])
	}
	if payload["fallback_reason"] != "small_model_insufficient" {
		t.Fatalf("fallback_reason = %v, want small_model_insufficient", payload["fallback_reason"])
	}
	if payload["small_model_sufficient"] != false {
		t.Fatalf("small_model_sufficient = %v, want false", payload["small_model_sufficient"])
	}
	if payload["small_model_preview"] != "unclear" {
		t.Fatalf("small_model_preview = %v, want unclear", payload["small_model_preview"])
	}
	if vision.calls != 1 {
		t.Fatalf("vision calls = %d, want 1", vision.calls)
	}
}

func TestNewImageToolUsesLongerDefaultDownloadTimeout(t *testing.T) {
	tool := NewImageTool(nil, nil, nil)
	if tool.httpClient == nil {
		t.Fatal("expected http client")
	}
	if tool.httpClient.Timeout != 5*time.Minute {
		t.Fatalf("http timeout = %v, want %v", tool.httpClient.Timeout, 5*time.Minute)
	}
}

func TestRegisterImageToolExposesSplitGenerateAndOCRTools(t *testing.T) {
	registry := NewRegistry()
	RegisterImageTool(registry, nil, func(_ context.Context, _ ImageGenerateRequest) (*ImageTaskResult, error) {
		return &ImageTaskResult{
			ID:     "img-1",
			Status: "succeeded",
			Outputs: []ImageAsset{{
				URL: "/tmp/robot_cafe.png",
			}},
		}, nil
	}, nil)

	if registry.Get("image") == nil {
		t.Fatal("expected native image tool to be registered")
	}
	if registry.Get("image_generation") == nil {
		t.Fatal("expected image_generation alias to be registered")
	}
	if registry.Get("generate_image") == nil {
		t.Fatal("expected generate_image split tool to be registered")
	}
	if registry.Get("generateImage") == nil {
		t.Fatal("expected generateImage alias to remain callable")
	}
	if registry.Get("ocr") == nil {
		t.Fatal("expected ocr split tool to be registered")
	}
	if !registry.IsDisabled("image_generation") || !registry.IsDisabled("generateImage") {
		t.Fatal("expected legacy aliases to stay hidden from model exposure")
	}
	if registry.IsDisabled("generate_image") || registry.IsDisabled("ocr") {
		t.Fatal("expected split tools to stay visible to model exposure")
	}

	visible := registry.List()
	if !containsString(visible, "image") {
		t.Fatalf("expected visible image tools to include native tool, got=%v", visible)
	}
	if !containsString(visible, "generate_image") || !containsString(visible, "ocr") {
		t.Fatalf("expected split tools to be visible, got=%v", visible)
	}
	if containsString(visible, "image_generation") || containsString(visible, "generateImage") {
		t.Fatalf("expected hidden legacy aliases to stay hidden, got=%v", visible)
	}
}

func TestGenerateImageToolForcesGenerateAction(t *testing.T) {
	var captured ImageGenerateRequest
	native := NewImageTool(nil, func(_ context.Context, req ImageGenerateRequest) (*ImageTaskResult, error) {
		captured = req
		return &ImageTaskResult{
			ID:     "img-2",
			Status: "succeeded",
			Outputs: []ImageAsset{{
				URL: req.OutputPath,
			}},
		}, nil
	}, nil)

	result, err := newGenerateImageTool(native).Execute(context.Background(), map[string]interface{}{
		"action": "review",
		"prompt": "friendly robot in a cozy cafe reading a book",
		"path":   "/tmp/robot_cafe.png",
	})
	if err != nil {
		t.Fatalf("generate_image execute failed: %v", err)
	}

	payload, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("payload = %#v, want map", result)
	}
	if captured.Prompt != "friendly robot in a cozy cafe reading a book" {
		t.Fatalf("prompt = %q", captured.Prompt)
	}
	if captured.OutputPath != "/tmp/robot_cafe.png" {
		t.Fatalf("output path = %q", captured.OutputPath)
	}
	if payload["status"] != "succeeded" {
		t.Fatalf("status = %v, want succeeded", payload["status"])
	}
}

func TestGenerateImageToolSupportsExplicitStatusAction(t *testing.T) {
	native := NewImageTool(nil, nil, func(_ context.Context, taskID string) (*ImageTaskResult, error) {
		if taskID != "task-status-1" {
			t.Fatalf("task id = %q, want task-status-1", taskID)
		}
		return &ImageTaskResult{ID: taskID, Status: "processing"}, nil
	})

	result, err := newGenerateImageTool(native).Execute(context.Background(), map[string]interface{}{
		"action": "status",
		"task":   map[string]interface{}{"id": "task-status-1"},
	})
	if err != nil {
		t.Fatalf("generate_image status failed: %v", err)
	}

	payload, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("payload = %#v, want map", result)
	}
	if payload["task_id"] != "task-status-1" {
		t.Fatalf("task_id = %v, want task-status-1", payload["task_id"])
	}
	if payload["status"] != "processing" {
		t.Fatalf("status = %v, want processing", payload["status"])
	}
}

func TestOCRToolForcesOCROnlyReview(t *testing.T) {
	ocr := &imageOCRMock{resp: ImageOCRResult{Text: "Quarterly revenue 119,900", Engine: "tesseract/wasm", Model: "eng"}}
	native := NewImageTool(nil, nil, nil)
	native.SetOCRService(ocr)

	result, err := newOCRTool(native).Execute(context.Background(), map[string]interface{}{
		"action": "generate",
		"image":  inlinePNGBase64(t),
		"prompt": "Extract every visible word from this screenshot.",
	})
	if err != nil {
		t.Fatalf("ocr execute failed: %v", err)
	}

	payload, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("payload = %#v, want map", result)
	}
	if payload["mode"] != "ocr" {
		t.Fatalf("mode = %v, want ocr", payload["mode"])
	}
	if got := payload["text"]; got != "Quarterly revenue 119,900" {
		t.Fatalf("text = %v", got)
	}
	if ocr.calls != 1 {
		t.Fatalf("ocr calls = %d, want 1", ocr.calls)
	}
}

func TestImageToolGenerateRoutesBananaSlidesToPPTService(t *testing.T) {
	tool := NewImageTool(nil, func(context.Context, ImageGenerateRequest) (*ImageTaskResult, error) {
		t.Fatal("plain image generation should not run when ppt slide-asset service is selected")
		return nil, nil
	}, nil)
	service := &pptGenerateMock{}
	tool.SetPPTService(service)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":              "ppt",
		"prompt":              "Create a revenue growth PPT slide visual",
		"style_preset":        "banana_slides",
		"aspect_ratio":        "16:9",
		"quality_profile":     "ppt",
		"review_threshold":    82.0,
		"review_retry_budget": 1.0,
		"reference_images":    []interface{}{"https://example.com/ref-1.png", "https://example.com/ref-2.png"},
		"layout_spec": map[string]interface{}{
			"template_id": "split",
			"elements": []map[string]interface{}{
				{
					"kind":      "text",
					"text":      "Revenue growth",
					"font_role": "title",
					"x":         96,
					"y":         120,
					"width":     420,
					"height":    96,
				},
			},
		},
		"source": "ppt",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	typed, ok := result.(*PPTResult)
	if !ok {
		t.Fatalf("result type = %T, want *PPTResult", result)
	}
	if typed.TaskID != "slide-1" {
		t.Fatalf("task_id = %q, want slide-1", typed.TaskID)
	}
	if service.req.StylePreset != "banana_slides" {
		t.Fatalf("style_preset = %q, want banana_slides", service.req.StylePreset)
	}
	if service.req.QualityProfile != "ppt" {
		t.Fatalf("quality_profile = %q, want ppt", service.req.QualityProfile)
	}
	if len(service.req.ReferenceImages) != 2 {
		t.Fatalf("reference_images = %#v, want 2", service.req.ReferenceImages)
	}
	if service.req.Source != "ppt" {
		t.Fatalf("source = %q, want ppt", service.req.Source)
	}
	if service.req.LayoutSpec == nil {
		t.Fatal("expected layout_spec to be forwarded to PPT service")
	}
}

func TestImageToolGenerateRoutesNanoSlidesAliasToPPTService(t *testing.T) {
	tool := NewImageTool(nil, func(context.Context, ImageGenerateRequest) (*ImageTaskResult, error) {
		t.Fatal("plain image generation should not run when nano slides ppt service is selected")
		return nil, nil
	}, nil)
	service := &pptGenerateMock{}
	tool.SetPPTService(service)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"prompt":       "Create a strategy summary slide",
		"style_preset": "nano slides",
		"source":       "slides",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if _, ok := result.(*PPTResult); !ok {
		t.Fatalf("result type = %T, want *PPTResult", result)
	}
	if service.req.StylePreset != "nano slides" {
		t.Fatalf("style_preset = %q, want original nano alias preserved for service normalization", service.req.StylePreset)
	}
}

func TestImageToolGenerateUsesFallbackDescriptionForPPTWithoutPrompt(t *testing.T) {
	tool := NewImageTool(nil, func(context.Context, ImageGenerateRequest) (*ImageTaskResult, error) {
		t.Fatal("plain image generation should not run when ppt slide-asset service is selected")
		return nil, nil
	}, nil)
	service := &pptGenerateMock{}
	tool.SetPPTService(service)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "ppt",
		"style_preset": "banana_slides",
		"theme":        "Aurora brand",
		"layout_spec": map[string]interface{}{
			"template_id": "cover",
		},
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if _, ok := result.(*PPTResult); !ok {
		t.Fatalf("result type = %T, want *PPTResult", result)
	}
	if service.req.Description == "" {
		t.Fatal("expected fallback description to be forwarded to PPT service")
	}
	if !strings.Contains(service.req.Description, "layout") {
		t.Fatalf("description = %q, want layout-aware fallback", service.req.Description)
	}
}

func TestImageToolReviewUsesVisionForInlineImage(t *testing.T) {
	vision := &imageVisionMock{resp: "a blue login screen"}
	tool := NewImageTool(&imageReviewMock{}, nil, nil)
	tool.SetVisionBridge(vision)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"image":  inlinePNGBase64(t),
		"prompt": "What is shown?",
	})
	if err != nil {
		t.Fatalf("execute vision review failed: %v", err)
	}
	payload, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("result type = %T, want map", result)
	}
	if payload["mode"] != "vision" {
		t.Fatalf("mode = %v, want vision", payload["mode"])
	}
	if vision.prompt != "What is shown?" {
		t.Fatalf("prompt = %q", vision.prompt)
	}
	decoded, err := base64.StdEncoding.DecodeString(vision.image)
	if err != nil {
		t.Fatalf("decode vision image: %v", err)
	}
	if _, err := png.Decode(bytes.NewReader(decoded)); err != nil {
		t.Fatalf("vision image should be normalized PNG: %v", err)
	}
}

func TestImageToolReviewUsesContextImageInputsWhenArgsOmitImage(t *testing.T) {
	vision := &imageVisionMock{resp: "context image description"}
	tool := NewImageTool(nil, nil, nil)
	tool.SetVisionBridge(vision)

	ctx := WithImageInputs(context.Background(), []ToolImageInput{{
		Name:     "upload.png",
		MimeType: "image/png",
		Data:     inlinePNGBase64(t),
	}})
	result, err := tool.Execute(ctx, map[string]interface{}{
		"action": "review",
		"prompt": "What is shown?",
	})
	if err != nil {
		t.Fatalf("execute context review failed: %v", err)
	}
	payload := result.(map[string]interface{})
	if payload["mode"] != "vision" {
		t.Fatalf("mode = %v, want vision", payload["mode"])
	}
	if vision.prompt != "What is shown?" {
		t.Fatalf("prompt = %q, want What is shown?", vision.prompt)
	}
}

func TestImageToolReviewSignedImageURLUsesVision(t *testing.T) {
	vision := &imageVisionMock{resp: "signed image description"}
	reviewer := &imageReviewMock{}
	tool := NewImageTool(reviewer, nil, nil)
	tool.SetVisionBridge(vision)
	tool.SetHTTPClient(newStaticImageClient("image/png", testPNGBytes(t)))

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"url":    "https://cdn.example.com/blob?id=123&token=abc",
		"prompt": "what is in this image?",
	})
	if err != nil {
		t.Fatalf("execute signed image review failed: %v", err)
	}
	payload := result.(map[string]interface{})
	if payload["mode"] != "vision" {
		t.Fatalf("mode = %v, want vision", payload["mode"])
	}
	if payload["source_url"] != "https://cdn.example.com/blob?id=123&token=abc" {
		t.Fatalf("source_url = %v", payload["source_url"])
	}
	if reviewer.args != nil {
		t.Fatalf("reviewer should not be called for signed image url")
	}
}

func TestImageToolReviewUsesVisionForLocalFilePath(t *testing.T) {
	vision := &imageVisionMock{resp: "local image description"}
	tool := NewImageTool(nil, nil, nil)
	tool.SetVisionBridge(vision)
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.png")
	if err := os.WriteFile(path, testPNGBytes(t), 0o644); err != nil {
		t.Fatalf("write local image: %v", err)
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"image":  path,
		"prompt": "what is in this local image?",
	})
	if err != nil {
		t.Fatalf("execute local image review failed: %v", err)
	}
	payload := result.(map[string]interface{})
	if payload["mode"] != "vision" {
		t.Fatalf("mode = %v, want vision", payload["mode"])
	}
	if payload["source"] != "file" {
		t.Fatalf("source = %v, want file", payload["source"])
	}
	if payload["source_path"] != path {
		t.Fatalf("source_path = %v, want %q", payload["source_path"], path)
	}
}

func TestImageToolReviewUsesVisionForFileURL(t *testing.T) {
	vision := &imageVisionMock{resp: "file url image description"}
	tool := NewImageTool(nil, nil, nil)
	tool.SetVisionBridge(vision)
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.png")
	if err := os.WriteFile(path, testPNGBytes(t), 0o644); err != nil {
		t.Fatalf("write local image: %v", err)
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"image_path": "file://" + path,
		"prompt":     "what is in this local image?",
	})
	if err != nil {
		t.Fatalf("execute file url image review failed: %v", err)
	}
	payload := result.(map[string]interface{})
	if payload["mode"] != "vision" {
		t.Fatalf("mode = %v, want vision", payload["mode"])
	}
	if payload["source_path"] != path {
		t.Fatalf("source_path = %v, want %q", payload["source_path"], path)
	}
}

func TestImageToolReviewRejectsLocalFileOutsideFSScope(t *testing.T) {
	vision := &imageVisionMock{resp: "external image description"}
	tool := NewImageTool(nil, nil, nil)
	tool.SetVisionBridge(vision)

	workspaceRoot := t.TempDir()
	externalRoot := t.TempDir()
	path := filepath.Join(externalRoot, "sample.png")
	if err := os.WriteFile(path, testPNGBytes(t), 0o644); err != nil {
		t.Fatalf("write external image: %v", err)
	}

	ctx := WithFSScope(context.Background(), []string{workspaceRoot}, map[string]string{"workspace": workspaceRoot})
	_, err := tool.Execute(ctx, map[string]interface{}{
		"image_path": path,
		"prompt":     "what is in this local image?",
	})
	if err == nil {
		t.Fatal("expected scope rejection for external local image")
	}
	if !strings.Contains(err.Error(), "path escapes workspace root") {
		t.Fatalf("error = %v, want path escapes workspace root", err)
	}
}

func TestOCRToolAllowsTempFileWithinFSScope(t *testing.T) {
	ocr := &imageOCRMock{resp: ImageOCRResult{Text: "Quarterly revenue 119,900", Engine: "tesseract/wasm", Model: "eng"}}
	native := NewImageTool(nil, nil, nil)
	native.SetOCRService(ocr)

	file, err := os.CreateTemp("", "ocr-scope-*.png")
	if err != nil {
		t.Fatalf("create temp image: %v", err)
	}
	tempPath := file.Name()
	t.Cleanup(func() {
		_ = os.Remove(tempPath)
	})
	if _, err := file.Write(testPNGBytes(t)); err != nil {
		_ = file.Close()
		t.Fatalf("write temp image: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close temp image: %v", err)
	}

	ctx := WithFSScope(context.Background(), []string{os.TempDir()}, map[string]string{"tmp": os.TempDir()})
	result, err := newOCRTool(native).Execute(ctx, map[string]interface{}{
		"image_path": tempPath,
	})
	if err != nil {
		t.Fatalf("ocr execute with temp scope failed: %v", err)
	}
	payload := result.(map[string]interface{})
	if payload["mode"] != "ocr" {
		t.Fatalf("mode = %v, want ocr", payload["mode"])
	}
	if payload["source_path"] != tempPath {
		t.Fatalf("source_path = %v, want %q", payload["source_path"], tempPath)
	}
}

func TestImageToolReviewRejectsPrivateRemoteURL(t *testing.T) {
	tool := NewImageTool(nil, nil, nil)
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"url":    "http://127.0.0.1:8080/private.png",
		"prompt": "what is in this image?",
	})
	if err == nil {
		t.Fatal("expected private host error")
	}
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "blocked") && !strings.Contains(msg, "private") {
		t.Fatalf("error = %v", err)
	}
}

func TestImageToolReviewMultiInlineAggregatesResults(t *testing.T) {
	vision := &imageVisionMock{responses: []string{"red chart dashboard", "blue chart dashboard"}}
	tool := NewImageTool(nil, nil, nil)
	tool.SetVisionBridge(vision)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"images": []interface{}{inlinePNGBase64(t), inlineJPEGBase64(t)},
		"prompt": "describe these images",
	})
	if err != nil {
		t.Fatalf("execute multi inline review failed: %v", err)
	}
	payload := result.(map[string]interface{})
	if payload["mode"] != "multi" {
		t.Fatalf("mode = %v, want multi", payload["mode"])
	}
	if payload["count"] != 2 {
		t.Fatalf("count = %v, want 2", payload["count"])
	}
	if payload["success_count"] != 2 {
		t.Fatalf("success_count = %v, want 2", payload["success_count"])
	}
	if summary, _ := payload["summary"].(string); !strings.Contains(summary, "Processed 2 images") {
		t.Fatalf("summary = %q", summary)
	}
	if analysis, _ := payload["analysis"].(string); !strings.Contains(analysis, "red chart dashboard") || !strings.Contains(analysis, "blue chart dashboard") {
		t.Fatalf("analysis = %q", analysis)
	}
	compare, ok := payload["compare"].(map[string]interface{})
	if !ok {
		t.Fatalf("compare = %#v", payload["compare"])
	}
	common, ok := compare["common_keywords"].([]string)
	if !ok || len(common) == 0 || common[0] != "chart" {
		t.Fatalf("common_keywords = %#v", compare["common_keywords"])
	}
	if compare["has_differences"] != true {
		t.Fatalf("has_differences = %v", compare["has_differences"])
	}
	compareSummary, _ := compare["summary"].(string)
	if !strings.Contains(compareSummary, "Shared themes") || !strings.Contains(compareSummary, "Distinct details") {
		t.Fatalf("compare summary = %q", compareSummary)
	}
	commonThemes, ok := compare["common_themes"].([]string)
	if !ok || len(commonThemes) == 0 || commonThemes[0] != "chart" {
		t.Fatalf("common_themes = %#v", compare["common_themes"])
	}
	distinctByImage, ok := compare["distinct_keywords_by_image"].([]map[string]interface{})
	if !ok || len(distinctByImage) != 2 {
		t.Fatalf("distinct_keywords_by_image = %#v", compare["distinct_keywords_by_image"])
	}
	if payload["compare_requested"] != false {
		t.Fatalf("compare_requested = %v", payload["compare_requested"])
	}
	if payload["review_mode"] != "batch" {
		t.Fatalf("review_mode = %v, want batch", payload["review_mode"])
	}
	items, ok := payload["items"].([]map[string]interface{})
	if !ok {
		t.Fatalf("items type = %T", payload["items"])
	}
	if len(items) != 2 {
		t.Fatalf("items len = %d, want 2", len(items))
	}
	for i, item := range items {
		if item["ok"] != true {
			t.Fatalf("item %d ok = %v", i, item["ok"])
		}
		if item["mode"] != "vision" {
			t.Fatalf("item %d mode = %v", i, item["mode"])
		}
	}
}

func TestImageToolReviewMultiMixedInputsAggregatesResults(t *testing.T) {
	reviewer := &imageReviewMock{resp: map[string]interface{}{"reviewed": true}}
	vision := &imageVisionMock{resp: "remote image description"}
	tool := NewImageTool(reviewer, nil, nil)
	tool.SetVisionBridge(vision)
	tool.SetHTTPClient(&http.Client{Transport: routingRoundTripper(func(req *http.Request) (*http.Response, error) {
		switch req.URL.String() {
		case "https://example.com/img?id=1":
			return staticRoundTripper{contentType: "image/png", body: testPNGBytes(t)}.RoundTrip(req)
		case "https://example.com/page":
			return staticRoundTripper{contentType: "text/html; charset=utf-8", body: []byte("<html></html>")}.RoundTrip(req)
		default:
			return staticRoundTripper{contentType: "text/plain", body: []byte("missing"), statusCode: http.StatusNotFound}.RoundTrip(req)
		}
	})})

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"urls":   []interface{}{"https://example.com/img?id=1", "https://example.com/page"},
		"prompt": "check them",
	})
	if err != nil {
		t.Fatalf("execute multi mixed review failed: %v", err)
	}
	payload := result.(map[string]interface{})
	if payload["mode"] != "multi" {
		t.Fatalf("mode = %v, want multi", payload["mode"])
	}
	if payload["success_count"] != 2 {
		t.Fatalf("success_count = %v, want 2", payload["success_count"])
	}
	if summary, _ := payload["summary"].(string); !strings.Contains(summary, "remote image description") {
		t.Fatalf("summary = %q", summary)
	}
	compare, ok := payload["compare"].(map[string]interface{})
	if !ok {
		t.Fatalf("compare = %#v", payload["compare"])
	}
	if compare["compared_count"] != 1 {
		t.Fatalf("compared_count = %v", compare["compared_count"])
	}
	compareSummary, _ := compare["summary"].(string)
	if !strings.Contains(compareSummary, "Comparison unavailable") {
		t.Fatalf("compare summary = %q", compareSummary)
	}
	indices, ok := payload["successful_indices"].([]int)
	if !ok || len(indices) != 2 || indices[0] != 0 || indices[1] != 1 {
		t.Fatalf("successful_indices = %#v", payload["successful_indices"])
	}
	items, ok := payload["items"].([]map[string]interface{})
	if !ok || len(items) != 2 {
		t.Fatalf("items = %#v", payload["items"])
	}
	if items[0]["mode"] != "vision" {
		t.Fatalf("first item mode = %v", items[0]["mode"])
	}
	secondResult, ok := items[1]["result"].(map[string]interface{})
	if !ok || secondResult["reviewed"] != true {
		t.Fatalf("second item result = %#v", items[1]["result"])
	}
}

func TestImageToolReviewMultiExplicitCompareMode(t *testing.T) {
	vision := &imageVisionMock{responses: []string{"red chart dashboard", "blue chart dashboard"}}
	tool := NewImageTool(nil, nil, nil)
	tool.SetVisionBridge(vision)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "compare",
		"images": []interface{}{inlinePNGBase64(t), inlineJPEGBase64(t)},
		"prompt": "compare these images",
	})
	if err != nil {
		t.Fatalf("execute compare review failed: %v", err)
	}
	payload := result.(map[string]interface{})
	if payload["mode"] != "compare" {
		t.Fatalf("mode = %v, want compare", payload["mode"])
	}
	if payload["review_mode"] != "compare" {
		t.Fatalf("review_mode = %v, want compare", payload["review_mode"])
	}
	if payload["compare_requested"] != true {
		t.Fatalf("compare_requested = %v", payload["compare_requested"])
	}
	compare, ok := payload["compare"].(map[string]interface{})
	if !ok {
		t.Fatalf("compare = %#v", payload["compare"])
	}
	commonThemes, ok := compare["common_themes"].([]string)
	if !ok || len(commonThemes) == 0 || commonThemes[0] != "chart" {
		t.Fatalf("common_themes = %#v", compare["common_themes"])
	}
	distinctByImage, ok := compare["distinct_keywords_by_image"].([]map[string]interface{})
	if !ok || len(distinctByImage) != 2 {
		t.Fatalf("distinct_keywords_by_image = %#v", compare["distinct_keywords_by_image"])
	}
}

func TestImageToolReviewDedupesRepeatedImages(t *testing.T) {
	vision := &imageVisionMock{responses: []string{"first image", "second image"}}
	tool := NewImageTool(nil, nil, nil)
	tool.SetVisionBridge(vision)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"image":  inlinePNGBase64(t),
		"images": []interface{}{inlinePNGBase64(t), inlineJPEGBase64(t), inlineJPEGBase64(t)},
		"prompt": "describe these images",
	})
	if err != nil {
		t.Fatalf("execute dedupe review failed: %v", err)
	}
	payload := result.(map[string]interface{})
	if payload["count"] != 2 {
		t.Fatalf("count = %v, want 2", payload["count"])
	}
	if vision.calls != 2 {
		t.Fatalf("vision calls = %d, want 2", vision.calls)
	}
}

func TestImageToolReviewRejectsTooManyImages(t *testing.T) {
	vision := &imageVisionMock{}
	tool := NewImageTool(nil, nil, nil)
	tool.SetVisionBridge(vision)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"images":     []interface{}{inlinePNGBase64(t), inlineJPEGBase64(t)},
		"max_images": 1,
		"prompt":     "compare these images",
	})
	if err == nil {
		t.Fatal("expected too many images error")
	}
	if !strings.Contains(err.Error(), "too many image inputs") || !strings.Contains(err.Error(), "max_images=1") {
		t.Fatalf("error = %v", err)
	}
	if vision.calls != 0 {
		t.Fatalf("vision calls = %d, want 0", vision.calls)
	}
}

func TestImageToolReviewNonImageURLStillDelegatesToReviewer(t *testing.T) {
	reviewer := &imageReviewMock{}
	tool := NewImageTool(reviewer, nil, nil)
	tool.SetHTTPClient(newStaticContentClient("text/html; charset=utf-8", []byte("<html></html>")))

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"url":    "https://example.com/blob?id=123",
		"prompt": "what is on this page?",
	})
	if err != nil {
		t.Fatalf("execute non-image url review failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if reviewer.args["action"] != "review_url" {
		t.Fatalf("action = %v, want review_url", reviewer.args["action"])
	}
	if reviewer.args["url"] != "https://example.com/blob?id=123" {
		t.Fatalf("url = %v", reviewer.args["url"])
	}
}

func TestImageToolReviewUsesVisionForInlineJPEG(t *testing.T) {
	vision := &imageVisionMock{resp: "jpeg description"}
	tool := NewImageTool(nil, nil, nil)
	tool.SetVisionBridge(vision)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"image":  inlineJPEGBase64(t),
		"prompt": "What is shown?",
	})
	if err != nil {
		t.Fatalf("execute jpeg vision review failed: %v", err)
	}
	payload := result.(map[string]interface{})
	if payload["mode"] != "vision" {
		t.Fatalf("mode = %v, want vision", payload["mode"])
	}
	decoded, err := base64.StdEncoding.DecodeString(vision.image)
	if err != nil {
		t.Fatalf("decode vision image: %v", err)
	}
	if _, err := png.Decode(bytes.NewReader(decoded)); err != nil {
		t.Fatalf("jpeg should be normalized to png before vision: %v", err)
	}
}

func TestImageToolReviewDirectImageURLUsesVision(t *testing.T) {
	vision := &imageVisionMock{resp: "remote image description"}
	reviewer := &imageReviewMock{}
	tool := NewImageTool(reviewer, nil, nil)
	tool.SetVisionBridge(vision)
	tool.SetHTTPClient(newStaticImageClient("image/png", testPNGBytes(t)))

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"url":    "https://example.com/sample.png",
		"prompt": "what is in this remote image?",
	})
	if err != nil {
		t.Fatalf("execute remote vision review failed: %v", err)
	}
	payload := result.(map[string]interface{})
	if payload["mode"] != "vision" {
		t.Fatalf("mode = %v, want vision", payload["mode"])
	}
	if payload["source"] != "url" {
		t.Fatalf("source = %v, want url", payload["source"])
	}
	if payload["source_url"] != "https://example.com/sample.png" {
		t.Fatalf("source_url = %v", payload["source_url"])
	}
	if reviewer.args != nil {
		t.Fatalf("reviewer should not be called for direct image url")
	}
}

func TestImageToolReviewDirectImageURLFallsBackToOCR(t *testing.T) {
	vision := &imageVisionMock{err: errors.New("vision unavailable")}
	ocr := &imageOCRMock{resp: ImageOCRResult{Text: "remote ocr text", Engine: "tesseract/wasm", Model: "eng"}}
	tool := NewImageTool(nil, nil, nil)
	tool.SetVisionBridge(vision)
	tool.SetOCRService(ocr)
	tool.SetHTTPClient(newStaticImageClient("image/png", testPNGBytes(t)))

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"url": "https://example.com/sample.png",
	})
	if err != nil {
		t.Fatalf("execute remote ocr fallback failed: %v", err)
	}
	payload := result.(map[string]interface{})
	if payload["mode"] != "ocr" {
		t.Fatalf("mode = %v, want ocr", payload["mode"])
	}
	if payload["analysis"] != "remote ocr text" {
		t.Fatalf("analysis = %v", payload["analysis"])
	}
	if payload["source"] != "url" {
		t.Fatalf("source = %v, want url", payload["source"])
	}
}

func TestImageToolReviewDelegatesURLToReviewer(t *testing.T) {
	reviewer := &imageReviewMock{}
	tool := NewImageTool(reviewer, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"url":    "https://example.com/page",
		"lang":   "zh-CN",
		"format": "json",
	})
	if err != nil {
		t.Fatalf("execute review failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if reviewer.args["action"] != "review_url" {
		t.Fatalf("action = %v, want review_url", reviewer.args["action"])
	}
	if reviewer.args["url"] != "https://example.com/page" {
		t.Fatalf("url = %v", reviewer.args["url"])
	}
	if reviewer.args["lang"] != "zh-CN" {
		t.Fatalf("lang = %v, want zh-CN", reviewer.args["lang"])
	}
}

func TestImageToolPromptAndURLDefaultsToReview(t *testing.T) {
	reviewer := &imageReviewMock{}
	tool := NewImageTool(reviewer, func(_ context.Context, req ImageGenerateRequest) (*ImageTaskResult, error) {
		t.Fatalf("generate should not run, got %#v", req)
		return nil, nil
	}, nil)
	tool.SetHTTPClient(newStaticContentClient("text/html; charset=utf-8", []byte("<html></html>")))

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"prompt": "what is in the image?",
		"url":    "https://example.com/page",
	})
	if err != nil {
		t.Fatalf("execute review failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if reviewer.args["action"] != "review_url" {
		t.Fatalf("action = %v, want review_url", reviewer.args["action"])
	}
}

func TestImageToolGenerateUsesReferenceInputsForEdit(t *testing.T) {
	var captured ImageGenerateRequest
	tool := NewImageTool(nil, func(_ context.Context, req ImageGenerateRequest) (*ImageTaskResult, error) {
		captured = req
		return &ImageTaskResult{ID: "task-2", Status: "processing", Message: "still running"}, nil
	}, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "edit",
		"prompt": "remove background",
		"url":    "https://example.com/input.png",
		"poll":   false,
	})
	if err != nil {
		t.Fatalf("execute edit failed: %v", err)
	}
	if captured.ReferenceImageURL != "https://example.com/input.png" {
		t.Fatalf("reference image url = %q", captured.ReferenceImageURL)
	}
	if captured.Category != "i2i" {
		t.Fatalf("category = %q, want i2i", captured.Category)
	}
	payload := result.(map[string]interface{})
	if payload["task_id"] != "task-2" {
		t.Fatalf("task_id = %v, want task-2", payload["task_id"])
	}
}

func TestImageToolGenerateCapturesOutputPath(t *testing.T) {
	var captured ImageGenerateRequest
	tool := NewImageTool(nil, func(_ context.Context, req ImageGenerateRequest) (*ImageTaskResult, error) {
		captured = req
		return &ImageTaskResult{ID: "task-output-path", Status: "succeeded"}, nil
	}, nil)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "generate",
		"prompt": "a friendly robot in a cafe",
		"path":   "robot_cafe.png",
		"poll":   false,
	}); err != nil {
		t.Fatalf("execute generate failed: %v", err)
	}

	if captured.OutputPath != "robot_cafe.png" {
		t.Fatalf("output path = %q, want robot_cafe.png", captured.OutputPath)
	}
	if got, _ := captured.Extra["path"].(string); got != "robot_cafe.png" {
		t.Fatalf("extra path = %q, want robot_cafe.png", got)
	}
}

func TestImageToolGenerateUsesLongerDefaultWaitTimeout(t *testing.T) {
	var captured ImageGenerateRequest
	tool := NewImageTool(nil, func(_ context.Context, req ImageGenerateRequest) (*ImageTaskResult, error) {
		captured = req
		return &ImageTaskResult{ID: "task-default-wait", Status: "processing"}, nil
	}, nil)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "generate",
		"prompt": "a calm landscape",
		"poll":   false,
	}); err != nil {
		t.Fatalf("execute generate failed: %v", err)
	}

	if captured.WaitTimeout != 300*time.Second {
		t.Fatalf("wait timeout = %v, want %v", captured.WaitTimeout, 300*time.Second)
	}
}

func TestImageToolGenerateSupportsGroupedArgs(t *testing.T) {
	var captured ImageGenerateRequest
	tool := NewImageTool(nil, func(_ context.Context, req ImageGenerateRequest) (*ImageTaskResult, error) {
		captured = req
		return &ImageTaskResult{ID: "task-grouped-generate", Status: "processing"}, nil
	}, nil)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"prompt": "make this image cleaner",
		"generate": map[string]interface{}{
			"model": "demo-model",
			"path":  "grouped.png",
			"count": 2,
			"reference": map[string]interface{}{
				"url": "https://example.com/input.png",
			},
			"wait": map[string]interface{}{
				"enabled":     false,
				"timeout_sec": 45,
			},
		},
	}); err != nil {
		t.Fatalf("execute grouped generate failed: %v", err)
	}

	if captured.Model != "demo-model" {
		t.Fatalf("model = %q, want demo-model", captured.Model)
	}
	if captured.OutputPath != "grouped.png" {
		t.Fatalf("output path = %q, want grouped.png", captured.OutputPath)
	}
	if captured.Count != 2 {
		t.Fatalf("count = %d, want 2", captured.Count)
	}
	if captured.ReferenceImageURL != "https://example.com/input.png" {
		t.Fatalf("reference image url = %q", captured.ReferenceImageURL)
	}
	if captured.Wait {
		t.Fatal("wait should be false")
	}
	if captured.WaitTimeout != 45*time.Second {
		t.Fatalf("wait timeout = %v, want 45s", captured.WaitTimeout)
	}
}

func TestImageToolReviewSupportsGroupedArgs(t *testing.T) {
	vision := &imageVisionMock{resp: "grouped review ok"}
	tool := NewImageTool(nil, nil, nil)
	tool.SetVisionBridge(vision)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"prompt": "describe this image",
		"review": map[string]interface{}{
			"input":    inlinePNGBase64(t),
			"mode":     "vision",
			"language": "en-US",
		},
	})
	if err != nil {
		t.Fatalf("execute grouped review failed: %v", err)
	}
	payload := result.(map[string]interface{})
	if payload["mode"] != "vision" {
		t.Fatalf("mode = %v, want vision", payload["mode"])
	}
	if payload["analysis"] != "grouped review ok" {
		t.Fatalf("analysis = %v", payload["analysis"])
	}
	if got := payload["prompt"]; got != "describe this image" {
		t.Fatalf("prompt = %v, want describe this image", got)
	}
}

func TestImageToolSupportsNestedCamelCaseReviewArgs(t *testing.T) {
	vision := &imageVisionMock{responses: []string{"red chart dashboard", "blue chart dashboard"}}
	tool := NewImageTool(nil, nil, nil)
	tool.SetVisionBridge(vision)

	tmpDir := t.TempDir()
	localPath := filepath.Join(tmpDir, "sample.png")
	if err := os.WriteFile(localPath, testPNGBytes(t), 0o644); err != nil {
		t.Fatalf("write local image: %v", err)
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": map[string]interface{}{
			"imageBase64": inlinePNGBase64(t),
			"imagePaths":  []interface{}{localPath},
			"compare":     true,
			"prompt":      "compare these images",
		},
	})
	if err != nil {
		t.Fatalf("execute nested review failed: %v", err)
	}
	payload := result.(map[string]interface{})
	if payload["mode"] != "compare" {
		t.Fatalf("mode = %v, want compare", payload["mode"])
	}
	if payload["compare_requested"] != true {
		t.Fatalf("compare_requested = %v, want true", payload["compare_requested"])
	}
	if payload["count"] != 2 {
		t.Fatalf("count = %v, want 2", payload["count"])
	}
}

func TestImageToolStatusSupportsTaskObject(t *testing.T) {
	tool := NewImageTool(nil, nil, func(_ context.Context, taskID string) (*ImageTaskResult, error) {
		if taskID != "task-grouped-status" {
			t.Fatalf("task id = %q, want task-grouped-status", taskID)
		}
		return &ImageTaskResult{ID: taskID, Status: "succeeded"}, nil
	})

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"task": map[string]interface{}{"id": "task-grouped-status"},
	})
	if err != nil {
		t.Fatalf("execute grouped status failed: %v", err)
	}
	payload := result.(map[string]interface{})
	if payload["task_id"] != "task-grouped-status" {
		t.Fatalf("task_id = %v, want task-grouped-status", payload["task_id"])
	}
}

func TestImageToolStatusUsesLookup(t *testing.T) {
	tool := NewImageTool(nil, nil, func(_ context.Context, taskID string) (*ImageTaskResult, error) {
		if taskID != "task-3" {
			t.Fatalf("task id = %q, want task-3", taskID)
		}
		return &ImageTaskResult{
			ID:        taskID,
			Status:    "succeeded",
			Provider:  "demo",
			Model:     "model-a",
			CreatedAt: time.Unix(10, 0).UTC(),
			Outputs:   []ImageAsset{{URL: "/img/final.png"}},
		}, nil
	})

	result, err := tool.Execute(context.Background(), map[string]interface{}{"task_id": "task-3"})
	if err != nil {
		t.Fatalf("execute status failed: %v", err)
	}
	payload := result.(map[string]interface{})
	if payload["status"] != "succeeded" {
		t.Fatalf("status = %v, want succeeded", payload["status"])
	}
	urls, ok := payload["image_urls"].([]string)
	if !ok || len(urls) != 1 || urls[0] != "/img/final.png" {
		t.Fatalf("image_urls = %#v", payload["image_urls"])
	}
}

func testPNGBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	img.Set(1, 1, color.RGBA{R: 0, G: 0, B: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

type routingRoundTripper func(req *http.Request) (*http.Response, error)

func (rt routingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return rt(req)
}

type staticRoundTripper struct {
	contentType string
	body        []byte
	statusCode  int
	headOnly    bool
}

func (rt staticRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	status := rt.statusCode
	if status == 0 {
		status = http.StatusOK
	}
	body := rt.body
	if req.Method == http.MethodHead || rt.headOnly {
		body = nil
	}
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     http.Header{"Content-Type": []string{rt.contentType}},
		Body:       io.NopCloser(bytes.NewReader(body)),
		Request:    req,
	}, nil
}

func newStaticImageClient(contentType string, body []byte) *http.Client {
	return &http.Client{Transport: staticRoundTripper{contentType: contentType, body: append([]byte(nil), body...)}}
}

func newStaticContentClient(contentType string, body []byte) *http.Client {
	return &http.Client{Transport: staticRoundTripper{contentType: contentType, body: append([]byte(nil), body...)}}
}

func inlinePNGBase64(t *testing.T) string {
	t.Helper()
	return base64.StdEncoding.EncodeToString(testPNGBytes(t))
}

func inlineJPEGBase64(t *testing.T) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 0, G: 255, B: 0, A: 255})
	img.Set(1, 1, color.RGBA{R: 255, G: 255, B: 0, A: 255})
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}
