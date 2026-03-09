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

type imageOCRMock struct {
	image []byte
	resp  ImageOCRResult
	err   error
}

func (m *imageOCRMock) Extract(_ context.Context, imagePNG []byte) (ImageOCRResult, error) {
	m.image = append([]byte(nil), imagePNG...)
	if m.resp.Text == "" && m.err == nil {
		m.resp = ImageOCRResult{Text: "ocr text", Engine: "tesseract/wasm", Model: "eng"}
	}
	return m.resp, m.err
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
