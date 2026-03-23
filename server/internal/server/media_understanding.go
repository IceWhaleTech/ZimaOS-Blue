package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/logger"
	pdfextract "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pdf"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/stt"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	_ "golang.org/x/image/webp"
)

const (
	mediaUnderstandingImagePrompt      = "Describe this attachment for a chat assistant. Include visible text, charts, UI, documents, and the most important visual details. Stay concise and do not speculate beyond what is visible."
	mediaUnderstandingVideoPrompt      = "This is a preview frame from a video attachment. Describe the visible scene, readable text, UI, charts, and the most important details. Stay concise and do not speculate beyond what is visible."
	mediaUnderstandingPDFPreviewPrompt = "This is the first-page preview of a PDF attachment. Summarize the visible document structure, readable text, tables, charts, and key information for a chat assistant. Stay concise and do not speculate beyond what is visible."
	mediaUnderstandingPDFMaxPages      = 8
	mediaUnderstandingPDFMaxChars      = 12000
	mediaUnderstandingTextMaxChars     = 10000
	mediaUnderstandingVLMMaxChars      = 4000
)

type mediaAttachment struct {
	Type       string
	Name       string
	MimeType   string
	Data       []byte
	DataBase64 string
	Duration   float64
	Thumbnail  []byte
	DecodeErr  error
}

func (h *ChatHandler) buildRequestAttachmentContentParts(ctx context.Context, attachments []MessageAttachment) []llm.ContentPart {
	if len(attachments) == 0 {
		return nil
	}
	parts := make([]llm.ContentPart, 0, len(attachments)*2)
	for _, att := range attachments {
		parts = append(parts, h.buildAttachmentContentParts(ctx, mediaAttachmentFromRequest(att))...)
	}
	return parts
}

func (h *ChatHandler) buildChannelAttachmentContentParts(ctx context.Context, attachments []channel.Attachment) []llm.ContentPart {
	if len(attachments) == 0 {
		return nil
	}
	parts := make([]llm.ContentPart, 0, len(attachments)*2)
	for _, att := range attachments {
		parts = append(parts, h.buildAttachmentContentParts(ctx, mediaAttachmentFromChannel(att))...)
	}
	return parts
}

func mediaAttachmentFromRequest(att MessageAttachment) mediaAttachment {
	out := mediaAttachment{
		Type:       normalizeMediaAttachmentType(att.Type, att.MimeType, att.Name),
		Name:       strings.TrimSpace(att.Name),
		MimeType:   strings.TrimSpace(att.MimeType),
		DataBase64: strings.TrimSpace(att.Data),
		Duration:   att.Duration,
	}
	if out.DataBase64 == "" {
		return out
	}
	data, err := base64.StdEncoding.DecodeString(out.DataBase64)
	if err != nil {
		out.DecodeErr = err
		out.DataBase64 = ""
		return out
	}
	out.Data = data
	return out
}

func mediaAttachmentFromChannel(att channel.Attachment) mediaAttachment {
	return mediaAttachment{
		Type:      normalizeMediaAttachmentType(string(att.Type), att.MimeType, att.Name),
		Name:      strings.TrimSpace(att.Name),
		MimeType:  strings.TrimSpace(att.MimeType),
		Data:      append([]byte(nil), att.Data...),
		Duration:  0,
		Thumbnail: append([]byte(nil), att.Thumbnail...),
	}
}

func normalizeMediaAttachmentType(rawType, mimeType, name string) string {
	normalized := strings.ToLower(strings.TrimSpace(rawType))
	switch normalized {
	case "image", "audio", "video":
		return normalized
	}
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return "image"
	case strings.HasPrefix(mimeType, "audio/"):
		return "audio"
	case strings.HasPrefix(mimeType, "video/"):
		return "video"
	default:
		if normalized == "file" {
			return "file"
		}
		if strings.HasSuffix(strings.ToLower(strings.TrimSpace(name)), ".pdf") {
			return "file"
		}
		return "file"
	}
}

func (h *ChatHandler) buildAttachmentContentParts(ctx context.Context, att mediaAttachment) []llm.ContentPart {
	switch att.Type {
	case "image":
		return h.buildImageAttachmentContentParts(ctx, att)
	case "audio":
		return h.buildAudioAttachmentContentParts(ctx, att)
	case "video":
		return h.buildVideoAttachmentContentParts(ctx, att)
	default:
		return h.buildFileAttachmentContentParts(ctx, att)
	}
}

func (h *ChatHandler) buildImageAttachmentContentParts(ctx context.Context, att mediaAttachment) []llm.ContentPart {
	parts := make([]llm.ContentPart, 0, 2)
	if imageBase64 := att.inlineBase64(); imageBase64 != "" {
		mediaType := strings.TrimSpace(att.MimeType)
		if mediaType == "" {
			mediaType = "image/png"
		}
		parts = append(parts, llm.ContentPart{
			Type:      "image",
			MediaType: mediaType,
			Data:      imageBase64,
		})
	}

	analysis := strings.TrimSpace(h.analyzeImageInline(ctx, att, mediaUnderstandingImagePrompt))
	switch {
	case analysis != "":
		parts = append(parts, llm.ContentPart{
			Type: "text",
			Text: formatMediaAttachmentSummary("Image attachment", att.displayName("image"), clipAttachmentText(analysis, mediaUnderstandingVLMMaxChars)),
		})
	case att.DecodeErr != nil:
		parts = append(parts, llm.ContentPart{
			Type: "text",
			Text: formatMediaAttachmentSummary("Image attachment", att.displayName("image"), "Image bytes could not be decoded for text fallback."),
		})
	default:
		body := "Image attached; detailed text fallback is unavailable."
		if len(parts) == 0 {
			body = "Image data is unavailable."
		}
		parts = append(parts, llm.ContentPart{
			Type: "text",
			Text: formatMediaAttachmentSummary("Image attachment", att.displayName("image"), body),
		})
	}
	return parts
}

func (h *ChatHandler) buildAudioAttachmentContentParts(ctx context.Context, att mediaAttachment) []llm.ContentPart {
	transcription, transcribed := h.transcribeAudioBytesForLLM(ctx, strings.TrimSpace(att.MimeType), att.Data, att.Duration)
	if transcribed {
		return []llm.ContentPart{{Type: "text", Text: transcription}}
	}

	parts := make([]llm.ContentPart, 0, 2)
	if audioBase64 := att.inlineBase64(); audioBase64 != "" {
		mediaType := strings.TrimSpace(att.MimeType)
		if mediaType == "" {
			mediaType = "audio/webm"
		}
		parts = append(parts, llm.ContentPart{
			Type:      "audio",
			MediaType: mediaType,
			Data:      audioBase64,
		})
	}
	parts = append(parts, llm.ContentPart{Type: "text", Text: transcription})
	return parts
}

func (h *ChatHandler) buildVideoAttachmentContentParts(ctx context.Context, att mediaAttachment) []llm.ContentPart {
	imageBase64, mediaType, err := h.resolveVideoPreview(ctx, att)
	parts := make([]llm.ContentPart, 0, 2)
	if imageBase64 != "" {
		parts = append(parts, llm.ContentPart{
			Type:      "image",
			MediaType: mediaType,
			Data:      imageBase64,
		})
	}

	if imageBase64 == "" {
		reason := "Video preview is unavailable."
		if err != nil {
			reason = "Video preview extraction failed."
			logger.Warn().Err(err).Str("attachment", att.displayName("video")).Msg("Failed to prepare video preview for media understanding")
		}
		return append(parts, llm.ContentPart{
			Type: "text",
			Text: formatMediaAttachmentSummary("Video attachment", att.displayName("video"), reason),
		})
	}

	analysis := strings.TrimSpace(h.analyzeImageBase64(ctx, att.displayName("video"), mediaType, imageBase64, mediaUnderstandingVideoPrompt))
	if analysis == "" {
		analysis = "Preview frame attached."
	}
	return append(parts, llm.ContentPart{
		Type: "text",
		Text: formatMediaAttachmentSummary("Video attachment", att.displayName("video"), clipAttachmentText(analysis, mediaUnderstandingVLMMaxChars)),
	})
}

func (h *ChatHandler) buildFileAttachmentContentParts(ctx context.Context, att mediaAttachment) []llm.ContentPart {
	summary := h.describeFileAttachment(ctx, att)
	return []llm.ContentPart{{
		Type: "text",
		Text: summary,
	}}
}

func (h *ChatHandler) describeFileAttachment(ctx context.Context, att mediaAttachment) string {
	name := att.displayName("file")
	if att.DecodeErr != nil {
		return formatMediaAttachmentSummary("File attachment", name, "File bytes could not be decoded.")
	}

	if isPDFAttachment(att) {
		if text := h.extractPDFTextViaTool(ctx, att); text != "" {
			return formatMediaAttachmentSummary("PDF attachment", name, text)
		}
		if previewSummary := h.extractPDFPreviewSummary(ctx, att); previewSummary != "" {
			return formatMediaAttachmentSummary("PDF attachment", name, previewSummary)
		}
		if len(att.Data) == 0 {
			return formatMediaAttachmentSummary("PDF attachment", name, "PDF data is unavailable.")
		}
		return formatMediaAttachmentSummary("PDF attachment", name, "PDF text extraction is unavailable.")
	}

	if isTextFile(att.Name, att.MimeType) {
		if len(att.Data) == 0 {
			return formatMediaAttachmentSummary("File attachment", name, "Text file is empty.")
		}
		return formatMediaAttachmentSummary("File attachment", name, clipAttachmentText(string(att.Data), mediaUnderstandingTextMaxChars))
	}

	return formatMediaAttachmentSummary("File attachment", name, "Binary file attached; rich text extraction is not available for this format yet.")
}

func isPDFAttachment(att mediaAttachment) bool {
	if strings.EqualFold(strings.TrimSpace(att.MimeType), "application/pdf") {
		return true
	}
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(att.Name)), ".pdf")
}

func (h *ChatHandler) extractPDFTextViaTool(ctx context.Context, att mediaAttachment) string {
	if h == nil || h.toolRegistry == nil || len(att.Data) == 0 {
		return ""
	}
	tool := h.toolRegistry.Get("pdf")
	if tool == nil {
		return ""
	}

	path, cleanup, err := writeMediaAttachmentTempFile(att, ".pdf")
	if err != nil {
		logger.Warn().Err(err).Str("attachment", att.displayName("pdf")).Msg("Failed to stage PDF attachment for media understanding")
		return ""
	}
	defer cleanup()

	toolCtx := tools.WithFSRootOverride(ctx, []string{filepath.Dir(path)}, nil)
	result, err := tool.Execute(toolCtx, map[string]interface{}{
		"action":    "read",
		"path":      path,
		"max_pages": mediaUnderstandingPDFMaxPages,
		"max_chars": mediaUnderstandingPDFMaxChars,
	})
	if err != nil {
		logger.Warn().Err(err).Str("attachment", att.displayName("pdf")).Msg("PDF media understanding extraction failed")
		return ""
	}

	text, truncated := extractPDFToolText(result)
	if text == "" {
		return ""
	}
	if truncated {
		text = strings.TrimSpace(text) + "\n...[truncated]"
	}
	return text
}

func (h *ChatHandler) extractPDFPreviewSummary(ctx context.Context, att mediaAttachment) string {
	if len(att.Data) == 0 {
		return ""
	}
	path, cleanup, err := writeMediaAttachmentTempFile(att, ".pdf")
	if err != nil {
		logger.Warn().Err(err).Str("attachment", att.displayName("pdf")).Msg("Failed to stage PDF preview for media understanding")
		return ""
	}
	defer cleanup()

	img, err := decodeWithPDFThumbnail(path)
	if err != nil {
		logger.Warn().Err(err).Str("attachment", att.displayName("pdf")).Msg("Failed to render PDF preview for media understanding")
		return ""
	}
	imageBase64, mediaType, err := encodeImageToPNGBase64(img)
	if err != nil {
		logger.Warn().Err(err).Str("attachment", att.displayName("pdf")).Msg("Failed to encode PDF preview for media understanding")
		return ""
	}
	return strings.TrimSpace(h.analyzeImageBase64(ctx, att.displayName("pdf"), mediaType, imageBase64, mediaUnderstandingPDFPreviewPrompt))
}

func extractPDFToolText(result interface{}) (string, bool) {
	switch typed := result.(type) {
	case pdfextract.ExtractResult:
		return strings.TrimSpace(typed.Text), typed.Truncated
	case *pdfextract.ExtractResult:
		if typed == nil {
			return "", false
		}
		return strings.TrimSpace(typed.Text), typed.Truncated
	case map[string]interface{}:
		text := mediaToolString(typed, "text")
		truncated := false
		if value, ok := typed["truncated"].(bool); ok {
			truncated = value
		}
		return strings.TrimSpace(text), truncated
	default:
		return "", false
	}
}

func mediaToolString(payload map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		value, ok := payload[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return typed
			}
		}
	}
	return ""
}

func (h *ChatHandler) analyzeImageInline(ctx context.Context, att mediaAttachment, prompt string) string {
	imageBase64 := att.inlineBase64()
	if imageBase64 == "" {
		return ""
	}
	mediaType := strings.TrimSpace(att.MimeType)
	if mediaType == "" {
		mediaType = "image/png"
	}
	return h.analyzeImageBase64(ctx, att.displayName("image"), mediaType, imageBase64, prompt)
}

func (h *ChatHandler) analyzeImageBase64(ctx context.Context, name, mimeType, imageBase64, prompt string) string {
	if h == nil || h.toolRegistry == nil || strings.TrimSpace(imageBase64) == "" {
		return ""
	}
	tool := h.toolRegistry.Get("image")
	if tool == nil {
		return ""
	}
	toolCtx := tools.WithImageInputs(ctx, []tools.ToolImageInput{{
		Name:     strings.TrimSpace(name),
		MimeType: strings.TrimSpace(mimeType),
		Data:     strings.TrimSpace(imageBase64),
	}})
	result, err := tool.Execute(toolCtx, map[string]interface{}{
		"action":        "review",
		"prompt":        prompt,
		"analysis_mode": selectMediaUnderstandingImageAnalysisMode(name, mimeType, prompt),
	})
	if err != nil {
		logger.Warn().Err(err).Str("attachment", strings.TrimSpace(name)).Msg("Image media understanding analysis failed")
		return ""
	}
	switch typed := result.(type) {
	case map[string]interface{}:
		return strings.TrimSpace(mediaToolString(typed, "analysis", "summary", "text"))
	default:
		return ""
	}
}

func selectMediaUnderstandingImageAnalysisMode(name, mimeType, prompt string) string {
	lowerName := strings.ToLower(strings.TrimSpace(name))
	lowerMime := strings.ToLower(strings.TrimSpace(mimeType))
	lowerPrompt := strings.ToLower(strings.TrimSpace(prompt))

	if strings.HasPrefix(lowerMime, "video/") || strings.Contains(lowerPrompt, "preview frame from a video") {
		return "cheap_first"
	}
	if strings.Contains(lowerPrompt, "first-page preview of a pdf") || strings.HasSuffix(lowerName, ".pdf") {
		return "ocr_first"
	}
	if mediaUnderstandingLooksTextHeavyImage(strings.Join([]string{lowerName, lowerMime}, " ")) {
		return "ocr_first"
	}
	return "cheap_first"
}

func mediaUnderstandingLooksTextHeavyImage(haystack string) bool {
	for _, marker := range []string{
		"screenshot", "screen-shot", "screen_shot", "dashboard", "chart", "graph", "table", "spreadsheet",
		"document", "invoice", "receipt", "form", "menu", "report", "slide", "slides", "terminal",
		"console", "ui", "interface", "pdf", "code",
	} {
		if strings.Contains(haystack, marker) {
			return true
		}
	}
	return false
}

func (h *ChatHandler) resolveVideoPreview(ctx context.Context, att mediaAttachment) (string, string, error) {
	if len(att.Thumbnail) > 0 {
		return encodePreviewImage(att.Thumbnail)
	}
	if len(att.Data) == 0 {
		return "", "", nil
	}

	path, cleanup, err := writeMediaAttachmentTempFile(att, filepath.Ext(strings.TrimSpace(att.Name)))
	if err != nil {
		return "", "", err
	}
	defer cleanup()

	img, err := decodeWithVideoThumbnail(path)
	if err != nil {
		return "", "", err
	}
	return encodeImageToPNGBase64(img)
}

func writeMediaAttachmentTempFile(att mediaAttachment, fallbackExt string) (string, func(), error) {
	ext := strings.TrimSpace(filepath.Ext(att.Name))
	if ext == "" {
		ext = strings.TrimSpace(fallbackExt)
	}
	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	file, err := os.CreateTemp("", "blue-media-*"+ext)
	if err != nil {
		return "", func() {}, err
	}
	path := file.Name()
	if _, err := file.Write(att.Data); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", func() {}, err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", func() {}, err
	}
	return path, func() { _ = os.Remove(path) }, nil
}

func encodePreviewImage(raw []byte) (string, string, error) {
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return "", "", err
	}
	return encodeImageToPNGBase64(img)
}

func encodeImageToPNGBase64(img image.Image) (string, string, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), "image/png", nil
}

func clipAttachmentText(text string, limit int) string {
	trimmed := strings.TrimSpace(text)
	if limit <= 0 {
		return trimmed
	}
	runes := []rune(trimmed)
	if len(runes) <= limit {
		return trimmed
	}
	return strings.TrimSpace(string(runes[:limit])) + "\n...[truncated]"
}

func formatMediaAttachmentSummary(label, name, body string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "unnamed"
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return fmt.Sprintf("[%s: %s]", label, name)
	}
	return fmt.Sprintf("[%s: %s]\n%s", label, name, body)
}

func (att mediaAttachment) displayName(fallback string) string {
	name := strings.TrimSpace(att.Name)
	if name != "" {
		return name
	}
	if fallback != "" {
		return fallback
	}
	return "attachment"
}

func (att mediaAttachment) inlineBase64() string {
	if att.DecodeErr != nil && len(att.Data) == 0 {
		return ""
	}
	if strings.TrimSpace(att.DataBase64) != "" {
		return strings.TrimSpace(att.DataBase64)
	}
	if len(att.Data) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(att.Data)
}

func (h *ChatHandler) transcribeAudioBytesForLLM(ctx context.Context, mimeType string, audioBytes []byte, duration float64) (string, bool) {
	if len(audioBytes) == 0 {
		return "[Voice message, audio data unavailable]", false
	}

	if h.sttService == nil {
		durationHint := ""
		if duration > 0 {
			durationHint = fmt.Sprintf(" (%ds)", int(duration))
		}
		return fmt.Sprintf("[Voice message%s, transcription unavailable]", durationHint), false
	}

	format := inferAudioFormat(mimeType)

	resp, err := h.sttService.Transcribe(ctx, &stt.TranscribeRequest{
		Audio:  bytes.NewReader(audioBytes),
		Format: format,
	})
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to transcribe audio attachment")
		return "[Voice message, transcription failed]", false
	}
	if strings.TrimSpace(resp.Text) == "" {
		return "[Voice message, no speech detected]", true
	}
	return fmt.Sprintf("[Voice message]: %s", resp.Text), true
}

func inferAudioFormat(mimeType string) stt.AudioFormat {
	lowerMime := strings.ToLower(strings.TrimSpace(mimeType))
	switch {
	case strings.Contains(lowerMime, "wav"):
		return stt.FormatWAV
	case strings.Contains(lowerMime, "mp3"), strings.Contains(lowerMime, "mpeg"):
		return stt.FormatMP3
	case strings.Contains(lowerMime, "webm"):
		return stt.FormatWebM
	case strings.Contains(lowerMime, "flac"):
		return stt.FormatFLAC
	case strings.Contains(lowerMime, "ogg"), strings.Contains(lowerMime, "opus"):
		return stt.FormatOGG
	default:
		return stt.FormatOGG
	}
}
