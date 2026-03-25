package pdf

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxybridge"
)

const (
	visionEngine        = "proxybridge/vision"
	visionMaxTokens     = 1800
	visionMaxImageWidth = 1400
	visionExtractPrompt = "Extract all readable text from this PDF page image. Preserve reading order, headings, paragraphs, bullet points, and table cells as plain text. Return only the extracted text. If the page has no readable text, return an empty string. Do not describe the image or add commentary."
)

// VisionResult contains text returned by a vision-capable LLM.
type VisionResult struct {
	Text     string `json:"text"`
	Engine   string `json:"engine,omitempty"`
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
}

// VisionService performs image-to-text extraction with a vision-capable LLM.
type VisionService interface {
	Extract(ctx context.Context, imagePNG []byte) (VisionResult, error)
}

type visionCaller interface {
	Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error)
}

// ProxyBridgeVisionAdapter adapts proxybridge.Bridge into a PDF VisionService.
type ProxyBridgeVisionAdapter struct {
	caller visionCaller
}

// NewProxyBridgeVisionAdapter creates a new PDF vision adapter.
func NewProxyBridgeVisionAdapter(bridge *proxybridge.Bridge) *ProxyBridgeVisionAdapter {
	return &ProxyBridgeVisionAdapter{caller: bridge}
}

// Extract runs a single image transcription request through the proxy bridge.
func (a *ProxyBridgeVisionAdapter) Extract(ctx context.Context, imagePNG []byte) (VisionResult, error) {
	if a == nil || a.caller == nil {
		return VisionResult{}, fmt.Errorf("vision bridge not available")
	}
	if len(imagePNG) == 0 {
		return VisionResult{}, fmt.Errorf("image bytes are required")
	}
	imageBase64 := resizeAndEncodeVisionPNG(imagePNG, visionMaxImageWidth)
	req := llm.ChatRequest{
		Model: "auto",
		Messages: []llm.Message{{
			Role: llm.RoleUser,
			ContentParts: []llm.ContentPart{
				{Type: "text", Text: visionExtractPrompt},
				{Type: "image", MediaType: "image/png", Data: imageBase64},
			},
		}},
		MaxTokens: visionMaxTokens,
	}
	resp, err := a.caller.Chat(ctx, req)
	if err != nil {
		return VisionResult{}, err
	}
	if resp == nil {
		return VisionResult{}, fmt.Errorf("vision bridge returned empty response")
	}
	return VisionResult{
		Text:     canonicalizeExtractedPDFText(resp.Message.Content),
		Engine:   visionEngine,
		Provider: resp.Provider,
		Model:    resp.Model,
	}, nil
}

func resizeAndEncodeVisionPNG(pngData []byte, maxWidth int) string {
	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return base64.StdEncoding.EncodeToString(pngData)
	}
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if maxWidth <= 0 || width <= maxWidth {
		return base64.StdEncoding.EncodeToString(pngData)
	}
	newWidth := maxWidth
	newHeight := height * maxWidth / width
	dst := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
	for y := 0; y < newHeight; y++ {
		srcY := y * height / newHeight
		for x := 0; x < newWidth; x++ {
			srcX := x * width / newWidth
			dst.Set(x, y, img.At(bounds.Min.X+srcX, bounds.Min.Y+srcY))
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return base64.StdEncoding.EncodeToString(pngData)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}
