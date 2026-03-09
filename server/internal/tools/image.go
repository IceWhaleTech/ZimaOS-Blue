package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strings"
	"time"
	"unicode"

	_ "golang.org/x/image/webp"
)

const (
	defaultImageDownloadTimeout = 20 * time.Second
	defaultImageReviewMaxInputs = 20
	maxImageReviewMaxInputs     = 50
	maxRemoteImageBytes         = 10 << 20
)

var errImageURLNotDirect = errors.New("url does not point to a direct image")

var imageCompareStopWords = map[string]struct{}{
	"the": {}, "this": {}, "that": {}, "with": {}, "from": {}, "into": {}, "there": {}, "their": {}, "about": {},
	"what": {}, "when": {}, "where": {}, "which": {}, "while": {}, "have": {}, "has": {}, "been": {}, "were": {},
	"your": {}, "you": {}, "and": {}, "for": {}, "are": {}, "but": {}, "not": {}, "its": {}, "onto": {},
	"than": {}, "then": {}, "them": {}, "they": {}, "these": {}, "those": {}, "image": {}, "images": {},
	"photo": {}, "picture": {}, "shows": {}, "showing": {}, "visible": {},
}

// ImageReviewService executes screenshot / URL review requests.
type ImageReviewService interface {
	Execute(ctx context.Context, args map[string]interface{}) (interface{}, error)
}

// ImageOCRResult is the normalized OCR output used by image fallback.
type ImageOCRResult struct {
	Text     string   `json:"text,omitempty"`
	Engine   string   `json:"engine,omitempty"`
	Model    string   `json:"model,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// ImageOCRService extracts text from inline images as a local fallback.
type ImageOCRService interface {
	Extract(ctx context.Context, imagePNG []byte) (ImageOCRResult, error)
}

// ImageGenerateRequest is the normalized request shape used by the native image tool.
type ImageGenerateRequest struct {
	Prompt               string
	NegativePrompt       string
	Model                string
	Size                 string
	Quality              string
	Style                string
	Category             string
	Count                int
	ReferenceImageURL    string
	ReferenceImageBase64 string
	Extra                map[string]interface{}
	Wait                 bool
	WaitTimeout          time.Duration
}

// ImageAsset describes a generated image result.
type ImageAsset struct {
	URL           string `json:"url,omitempty"`
	ThumbnailURL  string `json:"thumbnail_url,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
	ContentType   string `json:"content_type,omitempty"`
	Width         int    `json:"width,omitempty"`
	Height        int    `json:"height,omitempty"`
	DurationSec   int    `json:"duration_sec,omitempty"`
}

// ImageTaskResult is the normalized status/result payload for image generation.
type ImageTaskResult struct {
	ID        string                 `json:"id,omitempty"`
	Status    string                 `json:"status,omitempty"`
	Type      string                 `json:"type,omitempty"`
	Category  string                 `json:"category,omitempty"`
	Provider  string                 `json:"provider,omitempty"`
	Model     string                 `json:"model,omitempty"`
	Progress  float64                `json:"progress,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Message   string                 `json:"message,omitempty"`
	CreatedAt time.Time              `json:"created_at,omitempty"`
	UpdatedAt time.Time              `json:"updated_at,omitempty"`
	Request   map[string]interface{} `json:"request,omitempty"`
	Outputs   []ImageAsset           `json:"outputs,omitempty"`
}

// ImageGenerateFunc starts or waits on an image generation task.
type ImageGenerateFunc func(ctx context.Context, req ImageGenerateRequest) (*ImageTaskResult, error)

// ImageTaskLookupFunc fetches the current state of a generation task.
type ImageTaskLookupFunc func(ctx context.Context, taskID string) (*ImageTaskResult, error)

type imageReviewInput struct {
	Kind  string
	Value string
}

// ImageTool provides a native compatibility surface for OpenClaw-style image usage.
type ImageTool struct {
	reviewer   ImageReviewService
	vision     VLMBridge
	ocr        ImageOCRService
	generate   ImageGenerateFunc
	lookup     ImageTaskLookupFunc
	httpClient *http.Client
}

// NewImageTool creates a new native image tool.
func NewImageTool(reviewer ImageReviewService, generate ImageGenerateFunc, lookup ImageTaskLookupFunc) *ImageTool {
	return &ImageTool{
		reviewer:   reviewer,
		generate:   generate,
		lookup:     lookup,
		httpClient: newGuardedMediaHTTPClient(defaultImageDownloadTimeout),
	}
}

// SetVisionBridge injects a generic vision model for raw image understanding.
func (t *ImageTool) SetVisionBridge(bridge VLMBridge) {
	if t == nil {
		return
	}
	t.vision = bridge
}

// SetOCRService injects the local OCR fallback used for inline image recognition.
func (t *ImageTool) SetOCRService(svc ImageOCRService) {
	if t == nil {
		return
	}
	t.ocr = svc
}

// SetHTTPClient overrides the HTTP client used for remote direct-image downloads.
func (t *ImageTool) SetHTTPClient(client *http.Client) {
	if t == nil || client == nil {
		return
	}
	t.httpClient = client
}

// Definition returns the tool definition.
func (t *ImageTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "image",
		Description: "Generate or edit images, recognize one or more inline/remote images with VLM plus local OCR fallback, or check generation status. Defaults to waiting for generation results to reduce duplicate retries.",
		Icon:        "image",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"generate", "edit", "review", "analyze", "compare", "status", "get"},
					"description": "Action to perform. Auto-detected from task_id, prompt, and image/url inputs.",
				},
				"prompt":           map[string]interface{}{"type": "string", "description": "Prompt for image generation or editing."},
				"negative_prompt":  map[string]interface{}{"type": "string", "description": "Optional negative prompt for generation."},
				"model":            map[string]interface{}{"type": "string", "description": "Optional image model ID."},
				"size":             map[string]interface{}{"type": "string", "description": "Requested output size, e.g. 1024x1024."},
				"quality":          map[string]interface{}{"type": "string", "description": "Optional quality hint."},
				"style":            map[string]interface{}{"type": "string", "description": "Optional style hint."},
				"n":                map[string]interface{}{"type": "integer", "description": "Number of images to generate."},
				"category":         map[string]interface{}{"type": "string", "description": "Generation category, e.g. t2i or i2i."},
				"reference_image":  map[string]interface{}{"type": "string", "description": "Reference image URL for edit/i2i generation."},
				"image_url":        map[string]interface{}{"type": "string", "description": "Alias for reference_image when generating edits."},
				"reference_base64": map[string]interface{}{"type": "string", "description": "Base64 image content for edit/i2i generation."},
				"task_id":          map[string]interface{}{"type": "string", "description": "Task ID for status/get."},
				"poll":             map[string]interface{}{"type": "boolean", "description": "Wait for generation completion before returning. Defaults to true."},
				"wait_timeout_sec": map[string]interface{}{"type": "integer", "description": "Generation wait timeout in seconds. Defaults to 120."},
				"url":              map[string]interface{}{"type": "string", "description": "Target page/image URL for review."},
				"urls":             map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Multiple page/image URLs for review or recognition."},
				"image_urls":       map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Multiple direct or signed image URLs for review or recognition."},
				"image":            map[string]interface{}{"type": "string", "description": "Base64 image data for review."},
				"images":           map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Multiple base64 image payloads for recognition in one call."},
				"compare":          map[string]interface{}{"type": "boolean", "description": "When reviewing multiple images, emphasize shared themes and differences in a structured compare payload."},
				"max_images":       map[string]interface{}{"type": "integer", "description": "Maximum number of unique images/URLs to review after dedupe. Defaults to 20."},
				"lang":             map[string]interface{}{"type": "string", "description": "Output language for review."},
				"device":           map[string]interface{}{"type": "string", "description": "desktop or mobile for URL review."},
				"wait_ms":          map[string]interface{}{"type": "number", "description": "Extra page wait time for review_url."},
				"threshold":        map[string]interface{}{"type": "number", "description": "Review threshold score."},
				"format":           map[string]interface{}{"type": "string", "description": "Review output format: json or human."},
				"aspect_ratio":     map[string]interface{}{"type": "string", "description": "Optional aspect ratio for supported generation models."},
				"resolution":       map[string]interface{}{"type": "string", "description": "Optional resolution tier for supported generation models."},
			},
		},
	}
}

// Execute dispatches the requested image action.
func (t *ImageTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	switch imageAction(args) {
	case "status":
		return t.executeStatus(ctx, args)
	case "review":
		return t.executeReview(ctx, args)
	case "generate":
		return t.executeGenerate(ctx, args)
	default:
		return nil, fmt.Errorf("unsupported image action")
	}
}

func (t *ImageTool) executeStatus(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t == nil || t.lookup == nil {
		return nil, errors.New("image status service not available")
	}
	taskID := firstCompatString(args, "task_id", "id")
	if taskID == "" {
		return nil, errors.New("task_id/id is required")
	}
	task, err := t.lookup(ctx, taskID)
	if err != nil {
		return nil, err
	}
	return imageTaskEnvelope(task), nil
}

func (t *ImageTool) executeReview(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t == nil {
		return nil, errors.New("image review service not available")
	}
	prompt := imageRecognitionPrompt(args)
	compareRequested := imageCompareRequested(args)
	inputs, err := collectImageReviewInputs(args)
	if err != nil {
		return nil, err
	}
	if len(inputs) == 0 {
		return nil, errors.New("image/base64 or url is required for review")
	}
	if len(inputs) == 1 {
		return t.executeSingleReviewInput(ctx, prompt, args, inputs[0])
	}
	items := make([]map[string]interface{}, 0, len(inputs))
	analyses := make([]string, 0, len(inputs))
	successCount := 0
	errorCount := 0
	for index, input := range inputs {
		item := map[string]interface{}{
			"index":      index,
			"input_type": input.Kind,
		}
		if input.Kind == "url" {
			item["input"] = input.Value
		} else {
			item["input"] = "inline"
		}
		result, err := t.executeSingleReviewInput(ctx, prompt, args, input)
		if err != nil {
			errorCount++
			item["ok"] = false
			item["error"] = err.Error()
			items = append(items, item)
			continue
		}
		successCount++
		item["ok"] = true
		item["result"] = result
		if payload, ok := result.(map[string]interface{}); ok {
			if mode, ok := payload["mode"].(string); ok && mode != "" {
				item["mode"] = mode
			}
			if analysis, ok := payload["analysis"].(string); ok && strings.TrimSpace(analysis) != "" {
				analyses = append(analyses, analysis)
			}
		}
		items = append(items, item)
	}
	summary := buildMultiImageSummary(prompt, items, analyses)
	compare := buildMultiImageCompare(items)
	reviewMode := "batch"
	responseMode := "multi"
	if compareRequested {
		reviewMode = "compare"
		responseMode = "compare"
	}
	return map[string]interface{}{
		"mode":               responseMode,
		"review_mode":        reviewMode,
		"compare_requested":  compareRequested,
		"prompt":             prompt,
		"count":              len(inputs),
		"success_count":      successCount,
		"error_count":        errorCount,
		"items":              items,
		"analyses":           analyses,
		"analysis":           summary,
		"summary":            summary,
		"compare":            compare,
		"successful_indices": collectSuccessfulIndices(items),
	}, nil
}

func buildMultiImageSummary(prompt string, items []map[string]interface{}, analyses []string) string {
	if len(items) == 0 {
		return ""
	}
	if len(analyses) == 0 {
		return fmt.Sprintf("Processed %d images for prompt %q, but no textual analysis was produced.", len(items), prompt)
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Processed %d images for prompt %q.\n", len(items), prompt))
	for _, item := range items {
		index, _ := item["index"].(int)
		mode, _ := item["mode"].(string)
		if result, ok := item["result"].(map[string]interface{}); ok {
			analysis, _ := result["analysis"].(string)
			if strings.TrimSpace(analysis) == "" {
				continue
			}
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString(fmt.Sprintf("[%d][%s] %s", index, firstNonEmptyImage(mode, "unknown"), strings.TrimSpace(analysis)))
		}
	}
	return strings.TrimSpace(b.String())
}

func buildMultiImageCompare(items []map[string]interface{}) map[string]interface{} {
	entries := make([]map[string]interface{}, 0, len(items))
	docFreq := make(map[string]int)
	for _, item := range items {
		result, ok := item["result"].(map[string]interface{})
		if !ok {
			continue
		}
		analysis, _ := result["analysis"].(string)
		if strings.TrimSpace(analysis) == "" {
			continue
		}
		terms := extractImageCompareTerms(analysis)
		entry := map[string]interface{}{
			"index":    item["index"],
			"mode":     item["mode"],
			"keywords": terms,
		}
		entries = append(entries, entry)
		seen := map[string]struct{}{}
		for _, term := range terms {
			if _, ok := seen[term]; ok {
				continue
			}
			seen[term] = struct{}{}
			docFreq[term]++
		}
	}
	common := make([]string, 0)
	for term, count := range docFreq {
		if count >= 2 {
			common = append(common, term)
		}
	}
	sort.Strings(common)
	hasDifferences := false
	distinctByImage := make([]map[string]interface{}, 0, len(entries))
	for _, entry := range entries {
		keywords, _ := entry["keywords"].([]string)
		unique := make([]string, 0, len(keywords))
		for _, term := range keywords {
			if docFreq[term] < 2 {
				unique = append(unique, term)
			}
		}
		entry["unique_keywords"] = unique
		if len(unique) > 0 {
			hasDifferences = true
			distinctByImage = append(distinctByImage, map[string]interface{}{
				"index":    entry["index"],
				"mode":     entry["mode"],
				"keywords": unique,
			})
		}
	}
	return map[string]interface{}{
		"compared_count":             len(entries),
		"common_keywords":            common,
		"common_themes":              common,
		"items":                      entries,
		"distinct_keywords_by_image": distinctByImage,
		"has_differences":            hasDifferences,
		"summary":                    buildMultiImageCompareSummary(entries, common),
	}
}

func buildMultiImageCompareSummary(entries []map[string]interface{}, common []string) string {
	if len(entries) < 2 {
		return "Comparison unavailable: fewer than 2 analyzed images."
	}
	parts := make([]string, 0, 2)
	if len(common) > 0 {
		parts = append(parts, "Shared themes: "+strings.Join(common, ", "))
	}
	diffs := make([]string, 0, len(entries))
	for _, entry := range entries {
		index, _ := entry["index"].(int)
		unique, _ := entry["unique_keywords"].([]string)
		if len(unique) == 0 {
			continue
		}
		diffs = append(diffs, fmt.Sprintf("[%d] %s", index, strings.Join(unique, ", ")))
	}
	if len(diffs) > 0 {
		parts = append(parts, "Distinct details: "+strings.Join(diffs, "; "))
	}
	if len(parts) == 0 {
		return "Compared analyzed images but found no stable shared or distinct keywords."
	}
	return strings.Join(parts, ". ")
}

func extractImageCompareTerms(text string) []string {
	tokens := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	freq := make(map[string]int)
	for _, token := range tokens {
		if len(token) < 3 {
			continue
		}
		if _, skip := imageCompareStopWords[token]; skip {
			continue
		}
		freq[token]++
	}
	type termCount struct {
		term  string
		count int
	}
	pairs := make([]termCount, 0, len(freq))
	for term, count := range freq {
		pairs = append(pairs, termCount{term: term, count: count})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].count == pairs[j].count {
			return pairs[i].term < pairs[j].term
		}
		return pairs[i].count > pairs[j].count
	})
	if len(pairs) > 8 {
		pairs = pairs[:8]
	}
	out := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		out = append(out, pair.term)
	}
	return out
}

func collectSuccessfulIndices(items []map[string]interface{}) []int {
	out := make([]int, 0, len(items))
	for _, item := range items {
		ok, _ := item["ok"].(bool)
		if !ok {
			continue
		}
		if index, ok := item["index"].(int); ok {
			out = append(out, index)
		}
	}
	return out
}

func firstNonEmptyImage(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (t *ImageTool) executeSingleReviewInput(ctx context.Context, prompt string, args map[string]interface{}, input imageReviewInput) (interface{}, error) {
	switch input.Kind {
	case "inline":
		decoded, err := decodeCompatInlineImage(input.Value)
		if err != nil {
			return nil, err
		}
		normalized, normalizeErr := normalizeImageForVision(decoded)
		meta := map[string]interface{}{"source": "inline"}
		if normalizeErr != nil {
			meta["warnings"] = []string{"inline image could not be normalized; trying raw bytes fallback"}
			return t.analyzeImageBytes(ctx, prompt, input.Value, decoded, meta)
		}
		return t.analyzeImageBytes(ctx, prompt, base64.StdEncoding.EncodeToString(normalized), normalized, meta)
	case "file":
		localPath, imageBytes, err := readLocalImageBytes(input.Value)
		if err != nil {
			return nil, err
		}
		visionBytes, normalizeErr := normalizeImageForVision(imageBytes)
		visionBase64 := ""
		ocrBytes := imageBytes
		meta := map[string]interface{}{
			"source":      "file",
			"source_path": localPath,
		}
		if normalizeErr == nil {
			visionBase64 = base64.StdEncoding.EncodeToString(visionBytes)
			ocrBytes = visionBytes
		} else {
			meta["warnings"] = []string{"local image could not be normalized for vision; trying OCR fallback"}
		}
		return t.analyzeImageBytes(ctx, prompt, visionBase64, ocrBytes, meta)
	case "url":
		targetURL := input.Value
		if looksLikeDirectImageURL(targetURL) {
			return t.analyzeRemoteImageURL(ctx, prompt, targetURL)
		}
		if t.shouldProbeRemoteImageURL(args, targetURL) {
			result, err := t.tryAnalyzeRemoteImageURL(ctx, prompt, targetURL)
			if err == nil {
				return result, nil
			}
			if !errors.Is(err, errImageURLNotDirect) {
				return nil, err
			}
		}
		if t.reviewer == nil {
			return nil, errors.New("url review service not available")
		}
		reviewArgs := map[string]interface{}{
			"action": "review_url",
			"url":    targetURL,
		}
		copyCompatArg(reviewArgs, args, "lang")
		copyCompatArg(reviewArgs, args, "device")
		copyCompatArg(reviewArgs, args, "channel")
		copyCompatArg(reviewArgs, args, "wait_ms")
		copyCompatArg(reviewArgs, args, "threshold")
		copyCompatArg(reviewArgs, args, "format")
		return t.reviewer.Execute(ctx, reviewArgs)
	default:
		return nil, fmt.Errorf("unsupported image input type %q", input.Kind)
	}
}

func collectImageReviewInputs(args map[string]interface{}) ([]imageReviewInput, error) {
	inputs := make([]imageReviewInput, 0, 4)
	appendValues := func(kind string, value interface{}) error {
		items, err := collectCompatStringValues(value)
		if err != nil {
			return err
		}
		for _, item := range items {
			trimmed := strings.TrimSpace(item)
			if trimmed == "" {
				continue
			}
			if kind == "auto" {
				inputs = append(inputs, classifyImageReviewInput(trimmed))
				continue
			}
			inputs = append(inputs, imageReviewInput{Kind: kind, Value: trimmed})
		}
		return nil
	}
	if err := appendValues("auto", args["image"]); err != nil {
		return nil, err
	}
	if err := appendValues("inline", args["image_base64"]); err != nil {
		return nil, err
	}
	if err := appendValues("inline", args["base64"]); err != nil {
		return nil, err
	}
	if err := appendValues("auto", args["images"]); err != nil {
		return nil, err
	}
	if err := appendValues("file", args["image_path"]); err != nil {
		return nil, err
	}
	if err := appendValues("file", args["image_paths"]); err != nil {
		return nil, err
	}
	for _, key := range []string{"url", "href", "source", "link"} {
		if err := appendValues("auto", args[key]); err != nil {
			return nil, err
		}
	}
	if err := appendValues("auto", args["urls"]); err != nil {
		return nil, err
	}
	if err := appendValues("auto", args["image_urls"]); err != nil {
		return nil, err
	}
	if len(inputs) == 0 {
		return nil, nil
	}
	unique := make([]imageReviewInput, 0, len(inputs))
	seen := make(map[string]struct{}, len(inputs))
	for _, input := range inputs {
		key := input.Kind + "::" + input.Value
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, input)
	}
	limit := compatInt(args, "max_images", "maxImages")
	if limit <= 0 {
		limit = defaultImageReviewMaxInputs
	} else {
		limit = fsClamp(limit, 1, maxImageReviewMaxInputs)
	}
	if len(unique) > limit {
		return nil, fmt.Errorf("too many image inputs: %d exceeds max_images=%d", len(unique), limit)
	}
	return unique, nil
}

func collectCompatStringValues(value interface{}) ([]string, error) {
	switch typed := value.(type) {
	case nil:
		return nil, nil
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil, nil
		}
		return []string{typed}, nil
	case []string:
		return append([]string(nil), typed...), nil
	case []interface{}:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			values, err := collectCompatStringValues(item)
			if err != nil {
				return nil, err
			}
			out = append(out, values...)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported image input format")
	}
}

func classifyImageReviewInput(raw string) imageReviewInput {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(strings.ToLower(trimmed), "file://") {
		return imageReviewInput{Kind: "file", Value: trimmed}
	}
	if looksLikeRemoteHTTPURL(trimmed) {
		return imageReviewInput{Kind: "url", Value: trimmed}
	}
	if stat, err := os.Stat(trimmed); err == nil && !stat.IsDir() {
		return imageReviewInput{Kind: "file", Value: trimmed}
	}
	return imageReviewInput{Kind: "inline", Value: trimmed}
}

func looksLikeRemoteHTTPURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	scheme := strings.ToLower(parsed.Scheme)
	return scheme == "http" || scheme == "https"
}

func readLocalImageBytes(raw string) (string, []byte, error) {
	localPath, err := resolveImageLocalPath(raw)
	if err != nil {
		return "", nil, err
	}
	file, err := os.Open(localPath)
	if err != nil {
		return "", nil, fmt.Errorf("open local image: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxRemoteImageBytes+1))
	if err != nil {
		return "", nil, fmt.Errorf("read local image: %w", err)
	}
	if len(data) > maxRemoteImageBytes {
		return "", nil, fmt.Errorf("local image exceeds %d bytes", maxRemoteImageBytes)
	}
	return localPath, data, nil
}

func resolveImageLocalPath(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(strings.ToLower(trimmed), "file://") {
		parsed, err := url.Parse(trimmed)
		if err != nil {
			return "", fmt.Errorf("parse image file url: %w", err)
		}
		return fileURLToPath(parsed)
	}
	return trimmed, nil
}

func (t *ImageTool) tryAnalyzeRemoteImageURL(ctx context.Context, prompt, targetURL string) (interface{}, error) {
	isImage, err := t.probeRemoteImageURL(ctx, targetURL)
	if err != nil {
		return nil, err
	}
	if !isImage {
		return nil, errImageURLNotDirect
	}
	return t.analyzeRemoteImageURL(ctx, prompt, targetURL)
}

func (t *ImageTool) analyzeRemoteImageURL(ctx context.Context, prompt, targetURL string) (interface{}, error) {
	imageBytes, err := t.downloadRemoteImage(ctx, targetURL)
	if err != nil {
		return nil, err
	}
	visionBytes, normalizeErr := normalizeImageForVision(imageBytes)
	visionBase64 := ""
	ocrBytes := imageBytes
	meta := map[string]interface{}{
		"source":     "url",
		"source_url": targetURL,
	}
	if normalizeErr == nil {
		visionBase64 = base64.StdEncoding.EncodeToString(visionBytes)
		ocrBytes = visionBytes
	} else {
		meta["warnings"] = []string{"remote image could not be normalized for vision; trying OCR fallback"}
	}
	return t.analyzeImageBytes(ctx, prompt, visionBase64, ocrBytes, meta)
}

func (t *ImageTool) analyzeImageBytes(ctx context.Context, prompt, visionBase64 string, ocrBytes []byte, meta map[string]interface{}) (interface{}, error) {
	var visionErr error
	if t.vision != nil && strings.TrimSpace(visionBase64) != "" {
		analysis, err := t.vision.ChatWithVision(ctx, prompt, visionBase64)
		if err == nil && strings.TrimSpace(analysis) != "" {
			payload := map[string]interface{}{
				"mode":     "vision",
				"prompt":   prompt,
				"analysis": analysis,
			}
			mergeImageMeta(payload, meta)
			return payload, nil
		}
		visionErr = err
	} else if t.vision != nil && strings.TrimSpace(visionBase64) == "" {
		visionErr = errors.New("image format is not supported by vision path")
	}
	if t.ocr != nil && len(ocrBytes) > 0 {
		ocrResult, err := t.ocr.Extract(ctx, ocrBytes)
		if err == nil && strings.TrimSpace(ocrResult.Text) != "" {
			payload := map[string]interface{}{
				"mode":       "ocr",
				"prompt":     prompt,
				"analysis":   ocrResult.Text,
				"text":       ocrResult.Text,
				"ocr_engine": ocrResult.Engine,
				"ocr_model":  ocrResult.Model,
			}
			mergeImageMeta(payload, meta)
			if len(ocrResult.Warnings) > 0 {
				payload["warnings"] = appendStringSlices(payload["warnings"], ocrResult.Warnings)
			}
			if visionErr != nil {
				payload["fallback_from"] = "vision"
			}
			return payload, nil
		}
		if err != nil {
			if visionErr != nil {
				return nil, fmt.Errorf("image analysis failed: vision: %v; ocr: %w", visionErr, err)
			}
			return nil, err
		}
		if visionErr == nil {
			return nil, errors.New("image OCR returned empty text")
		}
	}
	if visionErr != nil {
		return nil, visionErr
	}
	return nil, errors.New("image recognition service not available")
}

func (t *ImageTool) probeRemoteImageURL(ctx context.Context, targetURL string) (bool, error) {
	client := t.httpClient
	if client == nil {
		client = newGuardedMediaHTTPClient(defaultImageDownloadTimeout)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, targetURL, nil)
	if err != nil {
		return false, fmt.Errorf("build image probe request: %w", err)
	}
	req.Header.Set("Accept", "image/*,*/*;q=0.8")
	resp, err := client.Do(req)
	if err == nil && resp != nil {
		defer resp.Body.Close()
		if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
			ctype := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
			if strings.HasPrefix(ctype, "image/") {
				return true, nil
			}
			if ctype != "" {
				return false, nil
			}
		}
	}
	data, err := t.downloadRemoteImage(ctx, targetURL)
	if err != nil {
		if errors.Is(err, errImageURLNotDirect) {
			return false, nil
		}
		return false, err
	}
	return len(data) > 0, nil
}

func (t *ImageTool) downloadRemoteImage(ctx context.Context, targetURL string) ([]byte, error) {
	client := t.httpClient
	if client == nil {
		client = newGuardedMediaHTTPClient(defaultImageDownloadTimeout)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build image request: %w", err)
	}
	req.Header.Set("Accept", "image/*,*/*;q=0.8")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download remote image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("download remote image: unexpected status %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxRemoteImageBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read remote image: %w", err)
	}
	if len(data) > maxRemoteImageBytes {
		return nil, fmt.Errorf("remote image exceeds %d bytes", maxRemoteImageBytes)
	}
	headerType := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	sniffedType := strings.ToLower(http.DetectContentType(data))
	if !strings.HasPrefix(headerType, "image/") && !strings.HasPrefix(sniffedType, "image/") {
		return nil, errImageURLNotDirect
	}
	return data, nil
}

func normalizeImageForVision(raw []byte) ([]byte, error) {
	decoded, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, decoded); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func imageRecognitionPrompt(args map[string]interface{}) string {
	prompt := firstCompatString(args, "prompt", "query", "input", "text", "message", "content")
	if strings.TrimSpace(prompt) == "" {
		return "Describe the image."
	}
	return prompt
}

func imageCompareRequested(args map[string]interface{}) bool {
	action := strings.ToLower(strings.TrimSpace(firstCompatString(args, "action", "op", "operation", "command")))
	if action == "compare" {
		return true
	}
	if compare, ok := asCompatBool(args["compare"]); ok && compare {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(firstCompatString(args, "mode", "review_mode"))) {
	case "compare", "comparison", "diff":
		return true
	default:
		return false
	}
}

func (t *ImageTool) shouldProbeRemoteImageURL(args map[string]interface{}, targetURL string) bool {
	if looksLikeDirectImageURL(targetURL) {
		return true
	}
	if explicit := strings.ToLower(strings.TrimSpace(firstCompatString(args, "action", "op", "operation", "command"))); explicit == "review" || explicit == "analyze" || explicit == "inspect" {
		return true
	}
	prompt := strings.TrimSpace(firstCompatString(args, "prompt", "query", "input", "text", "message", "content"))
	if prompt != "" {
		return true
	}
	parsed, err := url.Parse(strings.TrimSpace(targetURL))
	if err != nil {
		return false
	}
	for _, key := range []string{"format", "ext", "filename", "file", "mime", "content_type"} {
		value := strings.ToLower(strings.TrimSpace(parsed.Query().Get(key)))
		if strings.Contains(value, "png") || strings.Contains(value, "jpg") || strings.Contains(value, "jpeg") || strings.Contains(value, "gif") || strings.Contains(value, "webp") || strings.Contains(value, "bmp") || strings.Contains(value, "tiff") {
			return true
		}
	}
	return false
}

func looksLikeDirectImageURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	switch strings.ToLower(path.Ext(parsed.Path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".bmp", ".tif", ".tiff", ".webp":
		return true
	default:
		return false
	}
}

func mergeImageMeta(dst, meta map[string]interface{}) {
	for key, value := range meta {
		if value != nil {
			dst[key] = value
		}
	}
}

func appendStringSlices(existing interface{}, extra []string) []string {
	out := make([]string, 0, len(extra)+2)
	switch typed := existing.(type) {
	case []string:
		out = append(out, typed...)
	case []interface{}:
		for _, item := range typed {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
	}
	out = append(out, extra...)
	return out
}

func (t *ImageTool) executeGenerate(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t == nil || t.generate == nil {
		return nil, errors.New("image generation service not available")
	}
	prompt := firstCompatString(args, "prompt", "query", "input", "text", "message", "content")
	if prompt == "" {
		return nil, errors.New("prompt/query is required for image generation")
	}
	request := ImageGenerateRequest{
		Prompt:               prompt,
		NegativePrompt:       firstCompatString(args, "negative_prompt"),
		Model:                firstCompatString(args, "model"),
		Size:                 firstCompatString(args, "size"),
		Quality:              firstCompatString(args, "quality"),
		Style:                firstCompatString(args, "style"),
		Category:             firstCompatString(args, "category", "mode", "type"),
		Count:                compatInt(args, "n", "count", "num_images"),
		ReferenceImageURL:    firstCompatString(args, "reference_image", "reference_url", "image_url"),
		ReferenceImageBase64: firstCompatString(args, "reference_base64", "reference_image_base64"),
		Wait:                 true,
		WaitTimeout:          120 * time.Second,
	}
	if request.Count <= 0 {
		request.Count = 1
	}
	if request.ReferenceImageURL == "" && request.ReferenceImageBase64 == "" {
		if explicit := strings.ToLower(strings.TrimSpace(firstCompatString(args, "action", "op", "operation", "command"))); explicit == "generate" || explicit == "edit" || explicit == "create" || explicit == "draw" {
			request.ReferenceImageURL = firstCompatString(args, "url", "href", "source", "link")
			request.ReferenceImageBase64 = firstCompatString(args, "image", "image_base64", "base64")
		}
	}
	if request.Category == "" {
		if request.ReferenceImageURL != "" || request.ReferenceImageBase64 != "" {
			request.Category = "i2i"
		} else {
			request.Category = "t2i"
		}
	}
	if wait, ok := asCompatBool(args["poll"]); ok {
		request.Wait = wait
	} else if wait, ok := asCompatBool(args["wait"]); ok {
		request.Wait = wait
	} else if wait, ok := asCompatBool(args["sync"]); ok {
		request.Wait = wait
	}
	if timeoutSec := compatInt(args, "wait_timeout_sec", "timeout_sec", "timeout_seconds"); timeoutSec > 0 {
		request.WaitTimeout = time.Duration(fsClamp(timeoutSec, 1, 300)) * time.Second
	}
	if aspectRatio := firstCompatString(args, "aspect_ratio"); aspectRatio != "" {
		if request.Extra == nil {
			request.Extra = make(map[string]interface{})
		}
		request.Extra["aspect_ratio"] = aspectRatio
	}
	if resolution := firstCompatString(args, "resolution"); resolution != "" {
		if request.Extra == nil {
			request.Extra = make(map[string]interface{})
		}
		request.Extra["resolution"] = resolution
	}
	task, err := t.generate(ctx, request)
	if err != nil {
		return nil, err
	}
	return imageTaskEnvelope(task), nil
}

func imageAction(args map[string]interface{}) string {
	action := strings.ToLower(strings.TrimSpace(firstCompatString(args, "action", "op", "operation", "command")))
	switch action {
	case "", "auto":
		if firstCompatString(args, "task_id", "id") != "" {
			return "status"
		}
		if firstCompatString(args, "reference_image", "reference_url", "image_url", "reference_base64", "reference_image_base64") != "" {
			return "generate"
		}
		if args["images"] != nil || args["urls"] != nil || args["image_urls"] != nil || args["image_paths"] != nil {
			return "review"
		}
		if firstCompatString(args, "image", "image_base64", "base64", "url", "href", "source", "link", "image_path") != "" {
			return "review"
		}
		if firstCompatString(args, "prompt", "query", "input", "text", "message", "content") != "" {
			return "generate"
		}
		return "generate"
	case "status", "get", "task", "progress":
		return "status"
	case "review", "analyze", "inspect", "audit", "compare":
		return "review"
	case "generate", "create", "draw", "edit":
		return "generate"
	default:
		return action
	}
}

func imageTaskEnvelope(task *ImageTaskResult) map[string]interface{} {
	if task == nil {
		return map[string]interface{}{
			"task":       nil,
			"task_id":    "",
			"status":     "unknown",
			"images":     []ImageAsset{},
			"image_urls": []string{},
			"count":      0,
		}
	}
	urls := make([]string, 0, len(task.Outputs))
	for _, item := range task.Outputs {
		if item.URL != "" {
			urls = append(urls, item.URL)
		}
	}
	return map[string]interface{}{
		"task":       task,
		"task_id":    task.ID,
		"status":     task.Status,
		"provider":   task.Provider,
		"model":      task.Model,
		"message":    task.Message,
		"error":      task.Error,
		"images":     task.Outputs,
		"image_urls": urls,
		"count":      len(task.Outputs),
	}
}

func decodeCompatInlineImage(raw string) ([]byte, error) {
	trimmed := strings.TrimSpace(raw)
	if idx := strings.Index(trimmed, ","); idx >= 0 && strings.Contains(trimmed[:idx], ";base64") {
		trimmed = trimmed[idx+1:]
	}
	return base64.StdEncoding.DecodeString(trimmed)
}

func copyCompatArg(dst, src map[string]interface{}, key string) {
	if dst == nil || src == nil {
		return
	}
	if value, ok := src[key]; ok && value != nil {
		dst[key] = value
	}
}

// RegisterImageTool registers the native image tool.
func RegisterImageTool(registry *Registry, reviewer ImageReviewService, generate ImageGenerateFunc, lookup ImageTaskLookupFunc) {
	if registry == nil {
		return
	}
	if reviewer == nil && generate == nil && lookup == nil {
		return
	}
	registry.Register(NewImageTool(reviewer, generate, lookup))
}
