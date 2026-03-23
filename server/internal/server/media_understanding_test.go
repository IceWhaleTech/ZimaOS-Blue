package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	pdfextract "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pdf"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/stt"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type mediaImageToolMock struct {
	analysis   string
	inputs     []tools.ToolImageInput
	args       map[string]interface{}
	provider   string
	providerID string
	model      string
}

func (m *mediaImageToolMock) Definition() tools.ToolDefinition {
	return tools.ToolDefinition{Name: "image", Description: "mock image tool"}
}

func (m *mediaImageToolMock) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	m.args = cloneToolArgs(args)
	m.inputs = tools.GetImageInputs(ctx)
	m.provider = tools.GetProvider(ctx)
	m.providerID = tools.GetProviderID(ctx)
	m.model = tools.GetModel(ctx)
	return map[string]interface{}{
		"mode":     "vision",
		"analysis": m.analysis,
	}, nil
}

type mediaPDFToolMock struct {
	result interface{}
	args   map[string]interface{}
}

func (m *mediaPDFToolMock) Definition() tools.ToolDefinition {
	return tools.ToolDefinition{Name: "pdf", Description: "mock pdf tool"}
}

func (m *mediaPDFToolMock) Execute(_ context.Context, args map[string]interface{}) (interface{}, error) {
	m.args = cloneToolArgs(args)
	return m.result, nil
}

type mediaSTTMock struct {
	text   string
	format stt.AudioFormat
}

func (m *mediaSTTMock) Transcribe(_ context.Context, req *stt.TranscribeRequest) (*stt.TranscribeResponse, error) {
	if req != nil {
		m.format = req.Format
	}
	return &stt.TranscribeResponse{Text: m.text}, nil
}

func (m *mediaSTTMock) TranscribeWithProvider(ctx context.Context, _ stt.ProviderType, req *stt.TranscribeRequest) (*stt.TranscribeResponse, error) {
	return m.Transcribe(ctx, req)
}

func (m *mediaSTTMock) TranscribeStream(_ context.Context, _ *stt.TranscribeRequest, _ stt.StreamCallback) error {
	return nil
}

func (m *mediaSTTMock) ListProviders() []stt.ProviderType {
	return nil
}

func (m *mediaSTTMock) GetDefaultProvider() stt.ProviderType {
	return ""
}

func (m *mediaSTTMock) GetWhisperProvider() *stt.WhisperProvider {
	return nil
}

func (m *mediaSTTMock) PeekWhisperProvider() *stt.WhisperProvider {
	return nil
}

func (m *mediaSTTMock) Close() error {
	return nil
}

func TestApplyRequestAttachmentsToMessagesUsesMediaUnderstandingTools(t *testing.T) {
	registry := tools.NewRegistry()
	imageTool := &mediaImageToolMock{analysis: "A dashboard with KPI cards and a line chart."}
	pdfTool := &mediaPDFToolMock{result: pdfextract.ExtractResult{Text: "Quarterly revenue reached 42 units."}}
	registry.Register(imageTool)
	registry.Register(pdfTool)

	handler := NewChatHandler(nil, llm.NewProviderRegistry(), registry)
	req := SendMessageRequest{
		Message: "Summarize the attachments.",
		Attachments: []MessageAttachment{
			{
				Type:     "image",
				Name:     "chart.png",
				MimeType: "image/png",
				Data:     base64.StdEncoding.EncodeToString([]byte("image-bytes")),
			},
			{
				Type:     "file",
				Name:     "report.pdf",
				MimeType: "application/pdf",
				Data:     base64.StdEncoding.EncodeToString([]byte("%PDF-1.7")),
			},
		},
	}

	messages := handler.applyRequestAttachmentsToMessages(context.Background(), req, nil)
	if len(messages) != 1 {
		t.Fatalf("messages len = %d, want 1", len(messages))
	}
	parts := messages[0].ContentParts
	if len(parts) != 4 {
		t.Fatalf("content parts len = %d, want 4", len(parts))
	}
	if parts[1].Type != "image" {
		t.Fatalf("parts[1].Type = %q, want image", parts[1].Type)
	}
	if !strings.Contains(parts[2].Text, "KPI cards") {
		t.Fatalf("image summary = %q, want analysis text", parts[2].Text)
	}
	if !strings.Contains(parts[3].Text, "Quarterly revenue reached 42 units.") {
		t.Fatalf("pdf summary = %q, want extracted pdf text", parts[3].Text)
	}
	if len(imageTool.inputs) != 1 || imageTool.inputs[0].Name != "chart.png" {
		t.Fatalf("image tool inputs = %+v, want single chart.png context input", imageTool.inputs)
	}
	if got := strings.TrimSpace(asString(imageTool.args["analysis_mode"])); got != "ocr_first" {
		t.Fatalf("image tool analysis_mode = %q, want ocr_first", got)
	}
	if got := strings.TrimSpace(asString(pdfTool.args["path"])); got == "" || filepath.Ext(got) != ".pdf" {
		t.Fatalf("pdf tool path = %q, want temp .pdf path", got)
	}
}

func TestBuildChannelAttachmentContentPartsAddsVideoPreviewAndSummary(t *testing.T) {
	registry := tools.NewRegistry()
	imageTool := &mediaImageToolMock{analysis: "Two people stand on a stage in front of a presentation slide."}
	registry.Register(imageTool)

	handler := NewChatHandler(nil, llm.NewProviderRegistry(), registry)
	parts := handler.buildChannelAttachmentContentParts(context.Background(), []channel.Attachment{{
		Type:      channel.MessageTypeVideo,
		Name:      "demo.mp4",
		MimeType:  "video/mp4",
		Thumbnail: tinyPNGBytes(t),
	}})

	if len(parts) != 2 {
		t.Fatalf("content parts len = %d, want 2", len(parts))
	}
	if parts[0].Type != "image" || parts[0].MediaType != "image/png" {
		t.Fatalf("preview part = %+v, want image/png preview", parts[0])
	}
	if parts[1].Type != "text" || !strings.Contains(parts[1].Text, "stage") {
		t.Fatalf("video summary = %+v, want preview analysis text", parts[1])
	}
	if len(imageTool.inputs) != 1 {
		t.Fatalf("image tool inputs len = %d, want 1", len(imageTool.inputs))
	}
	if got := strings.TrimSpace(asString(imageTool.args["analysis_mode"])); got != "cheap_first" {
		t.Fatalf("image tool analysis_mode = %q, want cheap_first", got)
	}
}

func TestAttachmentProviderContextPrefersSelectedProviderID(t *testing.T) {
	ctx := withAttachmentProviderContext(context.Background(), "minimax", "openai", "MiniMax-M2.5")
	if got := tools.GetProviderID(ctx); got != "minimax" {
		t.Fatalf("provider_id = %q, want minimax", got)
	}
	if got := tools.GetProvider(ctx); got != "minimax" {
		t.Fatalf("provider = %q, want minimax", got)
	}
	if got := tools.GetModel(ctx); got != "MiniMax-M2.5" {
		t.Fatalf("model = %q, want MiniMax-M2.5", got)
	}
}

func TestWithToolProviderContextSetsProviderMetadata(t *testing.T) {
	ctx := withToolProviderContext(context.Background(), "minimax", "minimax", "MiniMax-M2.5")
	if got := tools.GetProvider(ctx); got != "minimax" {
		t.Fatalf("provider = %q, want minimax", got)
	}
	if got := tools.GetProviderID(ctx); got != "minimax" {
		t.Fatalf("provider_id = %q, want minimax", got)
	}
	if got := tools.GetModel(ctx); got != "MiniMax-M2.5" {
		t.Fatalf("model = %q, want MiniMax-M2.5", got)
	}
}

func TestApplyRequestAttachmentsToMessagesPropagatesAttachmentProviderContext(t *testing.T) {
	registry := tools.NewRegistry()
	imageTool := &mediaImageToolMock{analysis: "A product photo with visible text."}
	registry.Register(imageTool)

	handler := NewChatHandler(nil, llm.NewProviderRegistry(), registry)
	req := SendMessageRequest{
		Message:  "Summarize the attachment.",
		Model:    "MiniMax-M2.5",
		Provider: "openai",
		Attachments: []MessageAttachment{{
			Type:     "image",
			Name:     "photo.png",
			MimeType: "image/png",
			Data:     base64.StdEncoding.EncodeToString([]byte("image-bytes")),
		}},
	}

	ctx := withAttachmentProviderContext(context.Background(), "minimax", req.Provider, req.Model)
	_ = handler.applyRequestAttachmentsToMessages(ctx, req, nil)

	if imageTool.providerID != "minimax" {
		t.Fatalf("image tool provider_id = %q, want minimax", imageTool.providerID)
	}
	if imageTool.provider != "minimax" {
		t.Fatalf("image tool provider = %q, want minimax", imageTool.provider)
	}
	if imageTool.model != "MiniMax-M2.5" {
		t.Fatalf("image tool model = %q, want MiniMax-M2.5", imageTool.model)
	}
}

func TestBuildRequestAttachmentContentPartsTranscribesAudio(t *testing.T) {
	handler := NewChatHandler(nil, llm.NewProviderRegistry(), tools.NewRegistry())
	sttMock := &mediaSTTMock{text: "hello from audio"}
	handler.SetSTTService(sttMock)

	parts := handler.buildRequestAttachmentContentParts(context.Background(), []MessageAttachment{{
		Type:     "audio",
		Name:     "voice.mp3",
		MimeType: "audio/mp3",
		Data:     base64.StdEncoding.EncodeToString([]byte("mp3-bytes")),
	}})

	if len(parts) != 1 {
		t.Fatalf("content parts len = %d, want 1", len(parts))
	}
	if parts[0].Type != "text" || !strings.Contains(parts[0].Text, "hello from audio") {
		t.Fatalf("audio part = %+v, want transcribed text", parts[0])
	}
	if sttMock.format != stt.FormatMP3 {
		t.Fatalf("stt format = %q, want %q", sttMock.format, stt.FormatMP3)
	}
}

func TestBuildRequestAttachmentContentPartsSkipsBrokenInlineBase64(t *testing.T) {
	handler := NewChatHandler(nil, llm.NewProviderRegistry(), tools.NewRegistry())

	parts := handler.buildRequestAttachmentContentParts(context.Background(), []MessageAttachment{{
		Type:     "image",
		Name:     "broken.png",
		MimeType: "image/png",
		Data:     "!!!not-base64!!!",
	}})

	if len(parts) != 1 {
		t.Fatalf("content parts len = %d, want 1", len(parts))
	}
	if parts[0].Type != "text" {
		t.Fatalf("part type = %q, want text", parts[0].Type)
	}
	if strings.Contains(parts[0].Text, "not-base64") {
		t.Fatalf("text part leaked raw invalid base64: %q", parts[0].Text)
	}
}

func TestInferAudioFormatRecognizesWebM(t *testing.T) {
	if got := inferAudioFormat("audio/webm; codecs=opus"); got != stt.FormatWebM {
		t.Fatalf("inferAudioFormat(webm) = %q, want %q", got, stt.FormatWebM)
	}
}

func TestSelectMediaUnderstandingImageAnalysisMode(t *testing.T) {
	cases := []struct {
		name     string
		fileName string
		mimeType string
		prompt   string
		want     string
	}{
		{
			name:     "chart image prefers OCR",
			fileName: "sales-chart.png",
			mimeType: "image/png",
			prompt:   mediaUnderstandingImagePrompt,
			want:     "ocr_first",
		},
		{
			name:     "video preview prefers cheap model",
			fileName: "demo.mp4",
			mimeType: "image/png",
			prompt:   mediaUnderstandingVideoPrompt,
			want:     "cheap_first",
		},
		{
			name:     "pdf preview prefers OCR",
			fileName: "report.pdf",
			mimeType: "image/png",
			prompt:   mediaUnderstandingPDFPreviewPrompt,
			want:     "ocr_first",
		},
		{
			name:     "generic photo prefers cheap model",
			fileName: "photo.png",
			mimeType: "image/png",
			prompt:   mediaUnderstandingImagePrompt,
			want:     "cheap_first",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := selectMediaUnderstandingImageAnalysisMode(tc.fileName, tc.mimeType, tc.prompt); got != tc.want {
				t.Fatalf("selectMediaUnderstandingImageAnalysisMode() = %q, want %q", got, tc.want)
			}
		})
	}
}

func cloneToolArgs(args map[string]interface{}) map[string]interface{} {
	if len(args) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(args))
	for k, v := range args {
		out[k] = v
	}
	return out
}

func asString(value interface{}) string {
	s, _ := value.(string)
	return s
}

func tinyPNGBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 255, G: 64, B: 64, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}
