package pdf

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

type fakeVisionCaller struct {
	lastReq llm.ChatRequest
	resp    *llm.ChatResponse
	err     error
}

func (f *fakeVisionCaller) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	_ = ctx
	f.lastReq = req
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

func TestProxyBridgeVisionAdapterExtractBuildsVisionRequest(t *testing.T) {
	pngData := makePNG(t, 20, 10)
	caller := &fakeVisionCaller{resp: &llm.ChatResponse{Provider: "openai", Model: "gpt-4.1-mini", Message: llm.Message{Content: "hello\nworld"}}}
	adapter := &ProxyBridgeVisionAdapter{caller: caller}

	result, err := adapter.Extract(context.Background(), pngData)
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	if result.Text != "hello\nworld" {
		t.Fatalf("text = %q, want %q", result.Text, "hello\nworld")
	}
	if result.Provider != "openai" {
		t.Fatalf("provider = %q, want %q", result.Provider, "openai")
	}
	if caller.lastReq.Model != "auto" {
		t.Fatalf("model = %q, want auto", caller.lastReq.Model)
	}
	parts := caller.lastReq.Messages[0].ContentParts
	if len(parts) != 2 {
		t.Fatalf("content parts len = %d, want 2", len(parts))
	}
	if parts[0].Type != "text" || parts[1].Type != "image" {
		t.Fatalf("unexpected content parts: %#v", parts)
	}
	if _, err := base64.StdEncoding.DecodeString(parts[1].Data); err != nil {
		t.Fatalf("image data is not base64: %v", err)
	}
}

func TestPreferVisionText(t *testing.T) {
	if !preferVisionText("", "clear extracted text", "none") {
		t.Fatal("expected empty current text to prefer vision")
	}
	if !preferVisionText("! ? -", "clear extracted text", "ocr") {
		t.Fatal("expected stronger vision text to beat weak OCR")
	}
	if preferVisionText("solid text layer", "better maybe", "text") {
		t.Fatal("did not expect vision to replace native text layer")
	}
}

func makePNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}
