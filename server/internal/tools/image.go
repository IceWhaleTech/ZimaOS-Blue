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
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/smallmodel"

	_ "golang.org/x/image/webp"
)

const (
	defaultImageDownloadTimeout = 5 * time.Minute
	defaultImageReviewMaxInputs = 20
	maxImageReviewMaxInputs     = 50
	maxRemoteImageBytes         = 10 << 20

	imageAnalysisModeAuto       = "auto"
	imageAnalysisModeCheapFirst = "cheap_first"
	imageAnalysisModeOCRFirst   = "ocr_first"
	imageAnalysisModeOCROnly    = "ocr_only"
	imageAnalysisModeVisionOnly = "vision_only"

	imageAnalysisPreviewMaxChars = 160
)

var (
	errImageURLNotDirect          = errors.New("url does not point to a direct image")
	errImageVisionEmpty           = errors.New("image vision returned empty analysis")
	errImageVisionUnavailable     = errors.New("image vision service not available")
	errImageSmallModelEmpty       = errors.New("image small model returned empty analysis")
	errImageSmallModelUnavailable = errors.New("image small model service not available")
	errImageOCRUnavailable        = errors.New("image OCR service not available")
	errImageOCREmpty              = errors.New("image OCR returned empty text")
)

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

// ProviderAwareImageVision optionally handles provider-specific image understanding.
type ProviderAwareImageVision interface {
	Analyze(ctx context.Context, prompt, imageBase64 string) (analysis string, handled bool, err error)
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
	OutputPath           string
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
	Fallback  *MediaFallbackInfo     `json:"fallback_info,omitempty"`
}

// MediaFallbackInfo mirrors the backend fallback disclosure payload for native tool consumers.
type MediaFallbackInfo struct {
	Used        bool     `json:"used"`
	Strategy    string   `json:"strategy"`
	DisplayName string   `json:"display_name"`
	SourceURLs  []string `json:"source_urls,omitempty"`
	SpaceURL    string   `json:"space_url,omitempty"`
	Disclosure  string   `json:"disclosure"`
	RenderMode  string   `json:"render_mode,omitempty"`
	TemplateID  string   `json:"template_id,omitempty"`
	StylePreset string   `json:"style_preset,omitempty"`
}

// ImageGenerateFunc starts or waits on an image generation task.
type ImageGenerateFunc func(ctx context.Context, req ImageGenerateRequest) (*ImageTaskResult, error)

// ImageTaskLookupFunc fetches the current state of a generation task.
type ImageTaskLookupFunc func(ctx context.Context, taskID string) (*ImageTaskResult, error)

// PPTRequest is the normalized PPT slide-asset request passed from tools to adapters.
type PPTRequest struct {
	Description       string   `json:"description,omitempty"`
	AspectRatio       string   `json:"aspect_ratio,omitempty"`
	ReferenceImages   []string `json:"reference_images,omitempty"`
	LayoutSpec        any      `json:"layout_spec,omitempty"`
	StylePreset       string   `json:"style_preset,omitempty"`
	Theme             string   `json:"style_theme,omitempty"`
	Source            string   `json:"source,omitempty"`
	ReviewThreshold   float64  `json:"review_threshold,omitempty"`
	ReviewRetryBudget int      `json:"review_retry_budget,omitempty"`
	QualityProfile    string   `json:"quality_profile,omitempty"`
	Lang              string   `json:"lang,omitempty"`
}

// PPTResult is the normalized PPT slide-asset response shape returned by adapters.
type PPTResult struct {
	Status         string      `json:"status,omitempty"`
	Skipped        bool        `json:"skipped,omitempty"`
	SkipReason     string      `json:"skip_reason,omitempty"`
	TaskID         string      `json:"task_id,omitempty"`
	Model          string      `json:"model,omitempty"`
	Mode           string      `json:"mode,omitempty"`
	FinalPrompt    string      `json:"final_prompt,omitempty"`
	ReviewScore    float64     `json:"review_score,omitempty"`
	ReviewSummary  string      `json:"review_summary,omitempty"`
	RetryCount     int         `json:"retry_count,omitempty"`
	ImageURLs      []string    `json:"image_urls,omitempty"`
	ThumbnailURLs  []string    `json:"thumbnail_urls,omitempty"`
	Review         interface{} `json:"review,omitempty"`
	UsedFallback   bool        `json:"used_fallback,omitempty"`
	QualityProfile string      `json:"quality_profile,omitempty"`
	StylePreset    string      `json:"style_preset,omitempty"`
	Source         string      `json:"source,omitempty"`
	Error          string      `json:"error,omitempty"`
	Threshold      float64     `json:"threshold,omitempty"`
	Description    string      `json:"description,omitempty"`
	ReferenceCount int         `json:"reference_count,omitempty"`
}

// PPTGenerateService orchestrates PPT slide-asset generation workflows.
type PPTGenerateService interface {
	Generate(ctx context.Context, req PPTRequest) (*PPTResult, error)
}

type imageReviewInput struct {
	Kind  string
	Value string
}

type imageOCRSufficiency struct {
	Sufficient bool
	Reason     string
	Chars      int
	Words      int
	Lines      int
	Digits     int
}

type imageSmallModelSufficiency struct {
	Sufficient bool
	Reason     string
	Chars      int
	Words      int
	Lines      int
	Digits     int
}

// ImageTool provides a native compatibility surface for legacy image-style requests.
type ImageTool struct {
	reviewer          ImageReviewService
	vision            VLMBridge
	providerVision    ProviderAwareImageVision
	smallModel        smallmodel.Runtime
	smallModelEnabled func() bool
	ocr               ImageOCRService
	generate          ImageGenerateFunc
	lookup            ImageTaskLookupFunc
	ppt               PPTGenerateService
	httpClient        *http.Client
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

// SetProviderVision injects optional provider-aware image understanding.
func (t *ImageTool) SetProviderVision(vision ProviderAwareImageVision) {
	if t == nil {
		return
	}
	t.providerVision = vision
}

// SetSmallModelRuntime injects the optional local multimodal runtime used for cheap image QA.
func (t *ImageTool) SetSmallModelRuntime(rt smallmodel.Runtime) {
	if t == nil {
		return
	}
	t.smallModel = rt
}

// SetSmallModelEnabledFunc injects an optional gate for small-model image routing.
func (t *ImageTool) SetSmallModelEnabledFunc(fn func() bool) {
	if t == nil {
		return
	}
	t.smallModelEnabled = fn
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

// SetPPTService injects the PPT slide-asset orchestration service.
func (t *ImageTool) SetPPTService(svc PPTGenerateService) {
	if t == nil {
		return
	}
	t.ppt = svc
}

// Definition returns the tool definition.
func (t *ImageTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "image",
		Description: "Generate images, review image or page inputs, check task status, or build PPT slide assets.",
		Icon:        "image",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"generate", "review", "status", "ppt"},
					"description": "Action to perform. Usually omitted because prompt, task, and review inputs auto-route.",
				},
				"prompt": map[string]interface{}{
					"type":        "string",
					"description": "Prompt for generation, review focus, or PPT slide assets.",
				},
				"task": map[string]interface{}{
					"type":        "object",
					"description": "Task lookup for action=status.",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{"type": "string", "description": "Task ID to inspect."},
					},
				},
				"generate": map[string]interface{}{
					"type":        "object",
					"description": "Optional image-generation settings.",
					"properties": map[string]interface{}{
						"model": map[string]interface{}{"type": "string", "description": "Optional model ID."},
						"size":  map[string]interface{}{"type": "string", "description": "Output size such as 1024x1024."},
						"count": map[string]interface{}{"type": "integer", "description": "Number of images to generate."},
						"path":  map[string]interface{}{"type": "string", "description": "Optional output path for the primary image."},
						"reference": map[string]interface{}{
							"type":        "object",
							"description": "Optional edit or image-to-image source.",
							"properties": map[string]interface{}{
								"url":    map[string]interface{}{"type": "string", "description": "Remote image URL."},
								"base64": map[string]interface{}{"type": "string", "description": "Inline base64 image content."},
							},
						},
						"wait": map[string]interface{}{
							"type":        "object",
							"description": "Optional wait behavior for generation.",
							"properties": map[string]interface{}{
								"enabled":     map[string]interface{}{"type": "boolean", "description": "Wait for completion before returning."},
								"timeout_sec": map[string]interface{}{"type": "integer", "description": "Maximum wait time in seconds."},
							},
						},
					},
				},
				"review": map[string]interface{}{
					"type":        "object",
					"description": "Optional image/page review settings.",
					"properties": map[string]interface{}{
						"input":   map[string]interface{}{"type": "string", "description": "Single review input such as base64, URL, or local path."},
						"inputs":  map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Multiple review inputs. Each item may be base64, URL, or local path."},
						"compare": map[string]interface{}{"type": "boolean", "description": "Emphasize similarities and differences across multiple inputs."},
						"max_images": map[string]interface{}{
							"type":        "integer",
							"description": "Maximum unique inputs to review after dedupe.",
						},
						"mode": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"auto", "cheap", "ocr_first", "ocr", "vision"},
							"description": "Review strategy for direct images.",
						},
						"language": map[string]interface{}{"type": "string", "description": "Preferred output language."},
					},
				},
				"slide": map[string]interface{}{
					"type":        "object",
					"description": "Optional PPT slide-asset options.",
					"properties": map[string]interface{}{
						"reference_images": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Reference images for PPT slide visuals."},
						"layout_spec":      map[string]interface{}{"type": "object", "description": "Structured PPT layout spec."},
						"style_preset":     map[string]interface{}{"type": "string", "description": "Preset such as bananaslides or nanoslides."},
						"quality_profile":  map[string]interface{}{"type": "string", "description": "Quality profile. Use ppt to force slide-asset mode."},
						"theme":            map[string]interface{}{"type": "string", "description": "Theme or brand guidance."},
						"review": map[string]interface{}{
							"type":        "object",
							"description": "Optional PPT review controls.",
							"properties": map[string]interface{}{
								"threshold":    map[string]interface{}{"type": "number", "description": "Optional PPT review threshold."},
								"retry_budget": map[string]interface{}{"type": "integer", "description": "Optional PPT retry budget."},
							},
						},
					},
				},
			},
		},
	}
}

// Execute dispatches the requested image action.
func (t *ImageTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	args = normalizeImageArgs(args)
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
	taskID := firstCompatString(args, "task_id", "taskId", "id")
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
		inputs = collectContextImageReviewInputs(ctx)
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
	analysisMode := parseImageAnalysisMode(args)
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
			return t.analyzeImageBytes(ctx, prompt, input.Value, decoded, meta, analysisMode)
		}
		return t.analyzeImageBytes(ctx, prompt, base64.StdEncoding.EncodeToString(normalized), normalized, meta, analysisMode)
	case "file":
		localPath, imageBytes, err := readLocalImageBytes(ctx, input.Value)
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
		return t.analyzeImageBytes(ctx, prompt, visionBase64, ocrBytes, meta, analysisMode)
	case "url":
		targetURL := input.Value
		if looksLikeDirectImageURL(targetURL) {
			return t.analyzeRemoteImageURL(ctx, prompt, targetURL, analysisMode)
		}
		if t.shouldProbeRemoteImageURL(args, targetURL) {
			result, err := t.tryAnalyzeRemoteImageURL(ctx, prompt, targetURL, analysisMode)
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
	for _, entry := range []struct {
		kind string
		keys []string
	}{
		{kind: "auto", keys: []string{"image"}},
		{kind: "inline", keys: []string{"image_base64", "imageBase64"}},
		{kind: "inline", keys: []string{"base64"}},
		{kind: "auto", keys: []string{"images"}},
		{kind: "file", keys: []string{"image_path", "imagePath"}},
		{kind: "file", keys: []string{"image_paths", "imagePaths"}},
		{kind: "auto", keys: []string{"url"}},
		{kind: "auto", keys: []string{"href"}},
		{kind: "auto", keys: []string{"source"}},
		{kind: "auto", keys: []string{"link"}},
		{kind: "auto", keys: []string{"urls"}},
		{kind: "auto", keys: []string{"image_urls", "imageUrls"}},
	} {
		value, ok := compatArgValue(args, entry.keys...)
		if !ok {
			continue
		}
		if err := appendValues(entry.kind, value); err != nil {
			return nil, err
		}
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

func collectContextImageReviewInputs(ctx context.Context) []imageReviewInput {
	contextInputs := GetImageInputs(ctx)
	if len(contextInputs) == 0 {
		return nil
	}
	inputs := make([]imageReviewInput, 0, len(contextInputs))
	seen := make(map[string]struct{}, len(contextInputs))
	for _, input := range contextInputs {
		value := strings.TrimSpace(input.Data)
		if value == "" {
			continue
		}
		key := "inline::" + value
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		inputs = append(inputs, imageReviewInput{Kind: "inline", Value: value})
	}
	return inputs
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

func readLocalImageBytes(ctx context.Context, raw string) (string, []byte, error) {
	localPath, err := resolveImageLocalPath(ctx, raw)
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

func resolveImageLocalPath(ctx context.Context, raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(strings.ToLower(trimmed), "file://") {
		parsed, err := url.Parse(trimmed)
		if err != nil {
			return "", fmt.Errorf("parse image file url: %w", err)
		}
		resolved, err := fileURLToPath(parsed)
		if err != nil {
			return "", err
		}
		trimmed = resolved
	}

	if roots := imageScopeRootsFromContext(ctx); len(roots) > 0 {
		scope := &fsToolScope{roots: roots}
		resolved, _, _, err := scope.resolvePathWithContext(ctx, "image", trimmed, false)
		if err != nil {
			return "", err
		}
		return resolved, nil
	}

	return trimmed, nil
}

func imageScopeRootsFromContext(ctx context.Context) []string {
	roots, aliases := GetFSScope(ctx)
	if len(roots) == 0 && len(aliases) == 0 {
		return nil
	}

	out := make([]string, 0, len(roots)+len(aliases))
	seen := make(map[string]struct{}, len(roots)+len(aliases))
	add := func(raw string) {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			return
		}
		abs, err := filepath.Abs(trimmed)
		if err != nil {
			return
		}
		clean := filepath.Clean(abs)
		if _, ok := seen[clean]; ok {
			return
		}
		seen[clean] = struct{}{}
		out = append(out, clean)
	}

	for _, root := range roots {
		add(root)
	}
	for _, root := range aliases {
		add(root)
	}
	return out
}

func (t *ImageTool) tryAnalyzeRemoteImageURL(ctx context.Context, prompt, targetURL, analysisMode string) (interface{}, error) {
	isImage, err := t.probeRemoteImageURL(ctx, targetURL)
	if err != nil {
		return nil, err
	}
	if !isImage {
		return nil, errImageURLNotDirect
	}
	return t.analyzeRemoteImageURL(ctx, prompt, targetURL, analysisMode)
}

func (t *ImageTool) analyzeRemoteImageURL(ctx context.Context, prompt, targetURL, analysisMode string) (interface{}, error) {
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
	return t.analyzeImageBytes(ctx, prompt, visionBase64, ocrBytes, meta, analysisMode)
}

func (t *ImageTool) analyzeImageBytes(ctx context.Context, prompt, visionBase64 string, ocrBytes []byte, meta map[string]interface{}, analysisMode string) (interface{}, error) {
	switch normalizeImageAnalysisMode(analysisMode) {
	case imageAnalysisModeOCROnly:
		return t.performOCRImageAnalysis(ctx, prompt, ocrBytes, meta)
	case imageAnalysisModeVisionOnly:
		return t.performVisionImageAnalysis(ctx, prompt, visionBase64, meta)
	case imageAnalysisModeCheapFirst:
		smallPayload, smallErr := t.performSmallModelImageAnalysis(ctx, prompt, visionBase64, meta)
		if smallErr == nil {
			assessment := assessSmallModelImageSufficiency(prompt, asString(smallPayload["analysis"]))
			annotateSmallModelPayloadWithSufficiency(smallPayload, assessment)
			if assessment.Sufficient {
				return smallPayload, nil
			}
			visionPayload, visionErr := t.performVisionImageAnalysis(ctx, prompt, visionBase64, meta)
			if visionErr == nil {
				visionPayload["fallback_from"] = "small_model"
				visionPayload["fallback_reason"] = "small_model_insufficient"
				annotatePayloadWithSmallModelFallback(visionPayload, smallPayload, assessment)
				return visionPayload, nil
			}
			ocrPayload, ocrErr := t.performOCRImageAnalysis(ctx, prompt, ocrBytes, meta)
			if ocrErr == nil {
				ocrPayload["fallback_from"] = "vision"
				ocrPayload["fallback_reason"] = "vision_unavailable_after_insufficient_small_model"
				annotatePayloadWithSmallModelFallback(ocrPayload, smallPayload, assessment)
				ocrPayload["warnings"] = appendStringSlices(ocrPayload["warnings"], []string{
					"small model summary may be incomplete for this prompt; returning OCR fallback after vision was unavailable",
				})
				return ocrPayload, nil
			}
			smallPayload["fallback_reason"] = "vision_unavailable_after_insufficient_small_model"
			smallPayload["warnings"] = appendStringSlices(smallPayload["warnings"], []string{
				"small model summary may be incomplete for this prompt; higher-fidelity fallback unavailable",
			})
			return smallPayload, nil
		}
		visionPayload, visionErr := t.performVisionImageAnalysis(ctx, prompt, visionBase64, meta)
		if visionErr == nil {
			visionPayload["fallback_from"] = "small_model"
			return visionPayload, nil
		}
		ocrPayload, ocrErr := t.performOCRImageAnalysis(ctx, prompt, ocrBytes, meta)
		if ocrErr == nil {
			ocrPayload["fallback_from"] = "vision"
			ocrPayload["warnings"] = appendStringSlices(ocrPayload["warnings"], []string{
				"small model and vision analysis were unavailable; returning OCR fallback",
			})
			return ocrPayload, nil
		}
		return nil, fmt.Errorf("image analysis failed: small_model: %v; vision: %v; ocr: %w", smallErr, visionErr, ocrErr)
	case imageAnalysisModeOCRFirst:
		ocrPayload, ocrErr := t.performOCRImageAnalysis(ctx, prompt, ocrBytes, meta)
		if ocrErr == nil {
			assessment := assessImageOCRSufficiency(prompt, asString(ocrPayload["text"]))
			annotateOCRPayloadWithSufficiency(ocrPayload, assessment)
			if assessment.Sufficient {
				return ocrPayload, nil
			}
			smallPayload, smallErr := t.performSmallModelImageAnalysis(ctx, prompt, visionBase64, meta)
			if smallErr == nil {
				smallAssessment := assessSmallModelImageSufficiency(prompt, asString(smallPayload["analysis"]))
				annotateSmallModelPayloadWithSufficiency(smallPayload, smallAssessment)
				smallPayload["fallback_from"] = "ocr"
				smallPayload["fallback_reason"] = "ocr_insufficient"
				annotatePayloadWithOCRFallback(smallPayload, ocrPayload, assessment)
				if smallAssessment.Sufficient {
					return smallPayload, nil
				}
				visionPayload, visionErr := t.performVisionImageAnalysis(ctx, prompt, visionBase64, meta)
				if visionErr == nil {
					visionPayload["fallback_from"] = "small_model"
					visionPayload["fallback_reason"] = "small_model_insufficient_after_ocr"
					annotatePayloadWithOCRFallback(visionPayload, ocrPayload, assessment)
					annotatePayloadWithSmallModelFallback(visionPayload, smallPayload, smallAssessment)
					return visionPayload, nil
				}
				smallPayload["fallback_reason"] = "vision_unavailable_after_insufficient_small_model"
				smallPayload["warnings"] = appendStringSlices(smallPayload["warnings"], []string{
					"small model summary may be incomplete for this prompt; vision fallback unavailable",
				})
				return smallPayload, nil
			}
			visionPayload, visionErr := t.performVisionImageAnalysis(ctx, prompt, visionBase64, meta)
			if visionErr == nil {
				visionPayload["fallback_from"] = "ocr"
				visionPayload["fallback_reason"] = "ocr_insufficient"
				annotatePayloadWithOCRFallback(visionPayload, ocrPayload, assessment)
				return visionPayload, nil
			}
			ocrPayload["warnings"] = appendStringSlices(ocrPayload["warnings"], []string{
				"OCR result may be incomplete for this prompt; vision fallback unavailable",
			})
			ocrPayload["fallback_reason"] = "vision_unavailable_after_insufficient_ocr"
			return ocrPayload, nil
		}
		smallPayload, smallErr := t.performSmallModelImageAnalysis(ctx, prompt, visionBase64, meta)
		if smallErr == nil {
			smallPayload["fallback_from"] = "ocr"
			return smallPayload, nil
		}
		visionPayload, visionErr := t.performVisionImageAnalysis(ctx, prompt, visionBase64, meta)
		if visionErr == nil {
			visionPayload["fallback_from"] = "ocr"
			return visionPayload, nil
		}
		return nil, fmt.Errorf("image analysis failed: ocr: %v; small_model: %v; vision: %w", ocrErr, smallErr, visionErr)
	default:
		visionPayload, visionErr := t.performVisionImageAnalysis(ctx, prompt, visionBase64, meta)
		if visionErr == nil {
			return visionPayload, nil
		}
		ocrPayload, ocrErr := t.performOCRImageAnalysis(ctx, prompt, ocrBytes, meta)
		if ocrErr == nil {
			ocrPayload["fallback_from"] = "vision"
			return ocrPayload, nil
		}
		return nil, fmt.Errorf("image analysis failed: vision: %v; ocr: %w", visionErr, ocrErr)
	}
}

func parseImageAnalysisMode(args map[string]interface{}) string {
	return normalizeImageAnalysisMode(firstCompatString(args, "analysis_mode", "analysisMode"))
}

func normalizeImageAnalysisMode(raw string) string {
	mode := strings.ToLower(strings.TrimSpace(raw))
	switch mode {
	case "", imageAnalysisModeAuto, "full", "vision_first", "vision-first":
		return imageAnalysisModeAuto
	case imageAnalysisModeCheapFirst, "cheap-first", "small_model", "small-model", "smallmodel", "mini":
		return imageAnalysisModeCheapFirst
	case "ocr", "text", imageAnalysisModeOCROnly:
		return imageAnalysisModeOCROnly
	case imageAnalysisModeOCRFirst, "ocr-first", "prefer_ocr", "prefer-ocr", "cheap":
		return imageAnalysisModeOCRFirst
	case imageAnalysisModeVisionOnly, "vision", "vlm":
		return imageAnalysisModeVisionOnly
	default:
		return imageAnalysisModeAuto
	}
}

func (t *ImageTool) smallModelReady() bool {
	if t == nil || t.smallModel == nil || !t.smallModel.Ready() {
		return false
	}
	if t.smallModelEnabled != nil && !t.smallModelEnabled() {
		return false
	}
	return true
}

func (t *ImageTool) performSmallModelImageAnalysis(ctx context.Context, prompt, visionBase64 string, meta map[string]interface{}) (map[string]interface{}, error) {
	if !t.smallModelReady() {
		return nil, errImageSmallModelUnavailable
	}
	if strings.TrimSpace(visionBase64) == "" {
		return nil, errImageSmallModelUnavailable
	}
	trimmedPrompt := strings.TrimSpace(prompt)
	if trimmedPrompt == "" {
		trimmedPrompt = "Describe the image briefly."
	}
	resp, err := t.smallModel.Generate(ctx, smallmodel.GenerateRequest{
		Prompt:      "You are a concise visual assistant. Answer briefly using the attached image and the user's request. If the image is unclear, say so instead of guessing.\n\nUser: " + trimmedPrompt + "\nAssistant:",
		MaxTokens:   400,
		Temperature: 0.1,
		Images: []smallmodel.ImageInput{{
			MimeType: "image/png",
			Data:     visionBase64,
		}},
	})
	if err != nil {
		return nil, err
	}
	if resp == nil || strings.TrimSpace(resp.Text) == "" {
		return nil, errImageSmallModelEmpty
	}
	payload := map[string]interface{}{
		"mode":     "small_model",
		"prompt":   prompt,
		"analysis": strings.TrimSpace(resp.Text),
		"provider": "smallmodel",
	}
	mergeImageMeta(payload, meta)
	return payload, nil
}

func assessImageOCRSufficiency(prompt, text string) imageOCRSufficiency {
	text = strings.TrimSpace(text)
	if text == "" {
		return imageOCRSufficiency{Reason: "ocr_empty"}
	}
	chars := len([]rune(text))
	words := len(strings.Fields(text))
	lines := countNonEmptyImageLines(text)
	digits := countImageDigits(text)
	preferOCR := promptPrefersImageOCR(prompt)
	preferVision := promptPrefersImageVision(prompt)

	switch {
	case chars >= 140:
		return imageOCRSufficiency{Sufficient: true, Reason: "rich_ocr_text", Chars: chars, Words: words, Lines: lines, Digits: digits}
	case lines >= 4 && chars >= 60:
		return imageOCRSufficiency{Sufficient: true, Reason: "multi_line_ocr_text", Chars: chars, Words: words, Lines: lines, Digits: digits}
	case digits >= 8 && chars >= 32:
		return imageOCRSufficiency{Sufficient: true, Reason: "numeric_ocr_signal", Chars: chars, Words: words, Lines: lines, Digits: digits}
	}

	if preferVision {
		if chars >= 80 && (lines >= 3 || words >= 12) {
			return imageOCRSufficiency{Sufficient: true, Reason: "vision_prompt_but_ocr_is_rich", Chars: chars, Words: words, Lines: lines, Digits: digits}
		}
		return imageOCRSufficiency{Reason: "prompt_requests_visual_details_beyond_sparse_ocr", Chars: chars, Words: words, Lines: lines, Digits: digits}
	}

	if preferOCR {
		if chars >= 18 && (words >= 3 || digits >= 2 || lines >= 2) {
			return imageOCRSufficiency{Sufficient: true, Reason: "prompt_prefers_text_and_ocr_has_signal", Chars: chars, Words: words, Lines: lines, Digits: digits}
		}
		if chars >= 32 {
			return imageOCRSufficiency{Sufficient: true, Reason: "prompt_prefers_text_and_ocr_has_length", Chars: chars, Words: words, Lines: lines, Digits: digits}
		}
	}

	switch {
	case chars < 18 && lines <= 1 && digits < 2:
		return imageOCRSufficiency{Reason: "ocr_text_too_short", Chars: chars, Words: words, Lines: lines, Digits: digits}
	case words < 4 && digits < 4 && lines <= 1:
		return imageOCRSufficiency{Reason: "ocr_text_too_sparse", Chars: chars, Words: words, Lines: lines, Digits: digits}
	case chars >= 36 && (words >= 6 || digits >= 4 || lines >= 2):
		return imageOCRSufficiency{Sufficient: true, Reason: "moderate_ocr_signal", Chars: chars, Words: words, Lines: lines, Digits: digits}
	default:
		return imageOCRSufficiency{Reason: "ocr_text_may_be_incomplete", Chars: chars, Words: words, Lines: lines, Digits: digits}
	}
}

func annotateOCRPayloadWithSufficiency(payload map[string]interface{}, assessment imageOCRSufficiency) {
	if payload == nil {
		return
	}
	payload["ocr_sufficient"] = assessment.Sufficient
	if assessment.Reason != "" {
		payload["ocr_sufficiency_reason"] = assessment.Reason
	}
	if assessment.Chars > 0 {
		payload["ocr_chars"] = assessment.Chars
	}
}

func annotatePayloadWithOCRFallback(payload, ocrPayload map[string]interface{}, assessment imageOCRSufficiency) {
	if payload == nil {
		return
	}
	payload["ocr_sufficient"] = assessment.Sufficient
	if assessment.Reason != "" {
		payload["ocr_sufficiency_reason"] = assessment.Reason
	}
	if assessment.Chars > 0 {
		payload["ocr_chars"] = assessment.Chars
	}
	if preview := strings.TrimSpace(asString(ocrPayload["text"])); preview != "" {
		payload["ocr_preview"] = truncateRunes(preview, imageAnalysisPreviewMaxChars)
	}
}

func assessSmallModelImageSufficiency(prompt, text string) imageSmallModelSufficiency {
	text = strings.TrimSpace(text)
	if text == "" {
		return imageSmallModelSufficiency{Reason: "small_model_empty"}
	}
	chars := len([]rune(text))
	words := len(strings.Fields(text))
	lines := countNonEmptyImageLines(text)
	digits := countImageDigits(text)
	lower := strings.ToLower(text)
	preferOCR := promptPrefersImageOCR(prompt)
	preferVision := promptPrefersImageVision(prompt)

	if looksLikeUncertainImageAnalysis(lower) && chars < 96 {
		return imageSmallModelSufficiency{Reason: "small_model_uncertain", Chars: chars, Words: words, Lines: lines, Digits: digits}
	}

	switch {
	case chars >= 80:
		return imageSmallModelSufficiency{Sufficient: true, Reason: "rich_small_model_summary", Chars: chars, Words: words, Lines: lines, Digits: digits}
	case lines >= 2 && chars >= 48:
		return imageSmallModelSufficiency{Sufficient: true, Reason: "multi_line_small_model_summary", Chars: chars, Words: words, Lines: lines, Digits: digits}
	}

	if preferVision {
		if chars >= 24 && words >= 5 {
			return imageSmallModelSufficiency{Sufficient: true, Reason: "visual_summary_present", Chars: chars, Words: words, Lines: lines, Digits: digits}
		}
		return imageSmallModelSufficiency{Reason: "small_model_visual_summary_too_sparse", Chars: chars, Words: words, Lines: lines, Digits: digits}
	}

	if preferOCR {
		if chars >= 36 && (words >= 6 || digits >= 4 || lines >= 2) {
			return imageSmallModelSufficiency{Sufficient: true, Reason: "text_heavy_summary_present", Chars: chars, Words: words, Lines: lines, Digits: digits}
		}
		return imageSmallModelSufficiency{Reason: "small_model_summary_too_sparse_for_text_prompt", Chars: chars, Words: words, Lines: lines, Digits: digits}
	}

	if chars >= 28 && words >= 5 {
		return imageSmallModelSufficiency{Sufficient: true, Reason: "moderate_small_model_signal", Chars: chars, Words: words, Lines: lines, Digits: digits}
	}
	return imageSmallModelSufficiency{Reason: "small_model_summary_too_short", Chars: chars, Words: words, Lines: lines, Digits: digits}
}

func annotateSmallModelPayloadWithSufficiency(payload map[string]interface{}, assessment imageSmallModelSufficiency) {
	if payload == nil {
		return
	}
	payload["small_model_sufficient"] = assessment.Sufficient
	if assessment.Reason != "" {
		payload["small_model_sufficiency_reason"] = assessment.Reason
	}
	if assessment.Chars > 0 {
		payload["small_model_chars"] = assessment.Chars
	}
}

func annotatePayloadWithSmallModelFallback(payload, smallPayload map[string]interface{}, assessment imageSmallModelSufficiency) {
	if payload == nil {
		return
	}
	payload["small_model_sufficient"] = assessment.Sufficient
	if assessment.Reason != "" {
		payload["small_model_sufficiency_reason"] = assessment.Reason
	}
	if assessment.Chars > 0 {
		payload["small_model_chars"] = assessment.Chars
	}
	if preview := strings.TrimSpace(asString(smallPayload["analysis"])); preview != "" {
		payload["small_model_preview"] = truncateRunes(preview, imageAnalysisPreviewMaxChars)
	}
}

func looksLikeUncertainImageAnalysis(lower string) bool {
	return stringContainsAnyImage(lower,
		"can't tell", "cannot tell", "not sure", "unclear", "uncertain",
		"unable to determine", "difficult to determine", "hard to tell",
		"hard to see", "hard to read", "too blurry", "blurry", "low resolution")
}

func promptPrefersImageOCR(prompt string) bool {
	lower := strings.ToLower(strings.TrimSpace(prompt))
	return stringContainsAnyImage(lower,
		"ocr", "extract the text", "read the text", "transcribe", "visible text",
		"screenshot", "chart", "table", "dashboard", "diagram", "document", "invoice", "receipt", "menu", "form")
}

func promptPrefersImageVision(prompt string) bool {
	lower := strings.ToLower(strings.TrimSpace(prompt))
	return stringContainsAnyImage(lower,
		"describe the image", "describe the visual", "what is shown", "what's shown", "photo",
		"person", "people", "scene", "product", "device", "appearance", "look like", "color", "style")
}

func stringContainsAnyImage(input string, markers ...string) bool {
	for _, marker := range markers {
		if marker != "" && strings.Contains(input, marker) {
			return true
		}
	}
	return false
}

func countNonEmptyImageLines(text string) int {
	count := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

func countImageDigits(text string) int {
	count := 0
	for _, r := range text {
		if unicode.IsDigit(r) {
			count++
		}
	}
	return count
}

func (t *ImageTool) performVisionImageAnalysis(ctx context.Context, prompt, visionBase64 string, meta map[string]interface{}) (map[string]interface{}, error) {
	if strings.TrimSpace(visionBase64) == "" {
		return nil, errors.New("image format is not supported by vision path")
	}
	if t.providerVision != nil {
		analysis, handled, err := t.providerVision.Analyze(ctx, prompt, visionBase64)
		if handled {
			if err != nil {
				return nil, err
			}
			if strings.TrimSpace(analysis) == "" {
				return nil, errImageVisionEmpty
			}
			payload := map[string]interface{}{
				"mode":     "vision",
				"prompt":   prompt,
				"analysis": analysis,
			}
			mergeImageMeta(payload, meta)
			return payload, nil
		}
	}
	if t.vision == nil {
		return nil, errImageVisionUnavailable
	}
	analysis, err := t.vision.ChatWithVision(ctx, prompt, visionBase64)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(analysis) == "" {
		return nil, errImageVisionEmpty
	}
	payload := map[string]interface{}{
		"mode":     "vision",
		"prompt":   prompt,
		"analysis": analysis,
	}
	mergeImageMeta(payload, meta)
	return payload, nil
}

func (t *ImageTool) performOCRImageAnalysis(ctx context.Context, prompt string, ocrBytes []byte, meta map[string]interface{}) (map[string]interface{}, error) {
	if t.ocr == nil {
		return nil, errImageOCRUnavailable
	}
	if len(ocrBytes) == 0 {
		return nil, errImageOCRUnavailable
	}
	ocrResult, err := t.ocr.Extract(ctx, ocrBytes)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(ocrResult.Text) == "" {
		return nil, errImageOCREmpty
	}
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
	return payload, nil
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
	if compare, ok := compatBoolArg(args, "compare"); ok && compare {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(firstCompatString(args, "mode", "review_mode", "reviewMode"))) {
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
	if shouldUsePPT(args) && t.ppt != nil {
		result, err := executePPTService(ctx, args, t.ppt)
		if err != nil {
			return nil, err
		}
		if typed, ok := result.(*PPTResult); !ok || typed == nil || !typed.Skipped {
			return result, nil
		}
	}
	if t == nil || t.generate == nil {
		return nil, errors.New("image generation service not available")
	}
	prompt := firstCompatString(args, "prompt", "query", "input", "text", "message", "content")
	if prompt == "" {
		return nil, errors.New("prompt/query is required for image generation")
	}
	request := ImageGenerateRequest{
		Prompt:               prompt,
		NegativePrompt:       firstCompatString(args, "negative_prompt", "negativePrompt"),
		Model:                firstCompatString(args, "model"),
		Size:                 firstCompatString(args, "size"),
		Quality:              firstCompatString(args, "quality"),
		Style:                firstCompatString(args, "style"),
		Category:             firstCompatString(args, "category", "mode", "type"),
		Count:                compatInt(args, "n", "count", "num_images", "numImages"),
		OutputPath:           strings.TrimSpace(firstCompatPathString(args)),
		ReferenceImageURL:    firstCompatString(args, "reference_image", "referenceImage", "reference_url", "referenceUrl", "image_url", "imageUrl"),
		ReferenceImageBase64: firstCompatString(args, "reference_base64", "referenceBase64", "reference_image_base64", "referenceImageBase64"),
		Wait:                 true,
		WaitTimeout:          300 * time.Second,
	}
	if request.Count <= 0 {
		request.Count = 1
	}
	if request.ReferenceImageURL == "" && request.ReferenceImageBase64 == "" {
		if explicit := strings.ToLower(strings.TrimSpace(firstCompatString(args, "action", "op", "operation", "command"))); explicit == "generate" || explicit == "edit" || explicit == "create" || explicit == "draw" {
			request.ReferenceImageURL = firstCompatString(args, "url", "href", "source", "link")
			request.ReferenceImageBase64 = firstCompatString(args, "image", "image_base64", "imageBase64", "base64")
		}
	}
	if request.Category == "" {
		if request.ReferenceImageURL != "" || request.ReferenceImageBase64 != "" {
			request.Category = "i2i"
		} else {
			request.Category = "t2i"
		}
	}
	if request.OutputPath != "" {
		if request.Extra == nil {
			request.Extra = make(map[string]interface{})
		}
		request.Extra["path"] = request.OutputPath
	}
	if wait, ok := compatBoolArg(args, "poll"); ok {
		request.Wait = wait
	} else if wait, ok := compatBoolArg(args, "wait"); ok {
		request.Wait = wait
	} else if wait, ok := compatBoolArg(args, "sync"); ok {
		request.Wait = wait
	}
	if timeoutSec := compatInt(args, "wait_timeout_sec", "waitTimeoutSec", "timeout_sec", "timeoutSec", "timeout_seconds", "timeoutSeconds"); timeoutSec > 0 {
		request.WaitTimeout = time.Duration(fsClamp(timeoutSec, 1, 300)) * time.Second
	}
	if aspectRatio := firstCompatString(args, "aspect_ratio", "aspectRatio"); aspectRatio != "" {
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

func executePPTService(ctx context.Context, args map[string]interface{}, service PPTGenerateService) (interface{}, error) {
	if service == nil {
		return nil, errors.New("ppt slide-asset service not available")
	}
	req, err := buildPPTRequest(args)
	if err != nil {
		return nil, err
	}
	return service.Generate(ctx, req)
}

func buildPPTRequest(args map[string]interface{}) (PPTRequest, error) {
	description := strings.TrimSpace(firstCompatString(args, "description", "prompt", "query", "input", "text", "message", "content"))
	if description == "" {
		return PPTRequest{}, errors.New("description/prompt is required for ppt slide-asset generation")
	}
	referenceImages, err := collectPPTReferenceImages(args)
	if err != nil {
		return PPTRequest{}, err
	}
	return PPTRequest{
		Description:       description,
		AspectRatio:       firstCompatString(args, "aspect_ratio", "aspectRatio"),
		ReferenceImages:   referenceImages,
		LayoutSpec:        compatValue(args, "layout_spec", "layoutSpec", "ppt_layout_spec", "slide_layout_spec"),
		StylePreset:       firstCompatString(args, "style_preset", "stylePreset"),
		Theme:             firstCompatString(args, "style_theme", "styleTheme", "theme", "brand_guidance", "brandGuidance"),
		Source:            firstCompatString(args, "source", "origin"),
		ReviewThreshold:   compatFloat64(args, "review_threshold"),
		ReviewRetryBudget: compatInt(args, "review_retry_budget", "reviewRetryBudget"),
		QualityProfile:    firstCompatString(args, "quality_profile", "qualityProfile"),
		Lang:              firstCompatString(args, "language", "lang"),
	}, nil
}

func collectPPTReferenceImages(args map[string]interface{}) ([]string, error) {
	values := make([]string, 0, 4)
	for _, keys := range [][]string{{"reference_images", "referenceImages"}, {"reference_urls", "referenceUrls"}} {
		value, ok := compatArgValue(args, keys...)
		if !ok {
			continue
		}
		items, err := collectCompatStringValues(value)
		if err != nil {
			return nil, err
		}
		values = append(values, items...)
	}
	if len(values) == 0 {
		if single := firstCompatString(args, "reference_image", "referenceImage", "reference_url", "referenceUrl", "image_url", "imageUrl"); single != "" {
			values = append(values, single)
		}
	}
	unique := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		unique = append(unique, trimmed)
	}
	return unique, nil
}

func shouldUsePPT(args map[string]interface{}) bool {
	stylePreset := normalizePPTStylePreset(firstCompatString(args, "style_preset", "stylePreset"))
	qualityProfile := strings.ToLower(strings.TrimSpace(firstCompatString(args, "quality_profile", "qualityProfile")))
	source := strings.ToLower(strings.TrimSpace(firstCompatString(args, "source", "origin")))
	action := strings.ToLower(strings.TrimSpace(firstCompatString(args, "action", "op", "operation", "command")))
	if action == "ppt" {
		return true
	}
	if stylePreset == "banana_slides" || stylePreset == "nano_slides" || qualityProfile == "ppt" {
		return true
	}
	if _, ok := compatArgValue(args, "layout_spec", "layoutSpec", "ppt_layout_spec", "slide_layout_spec"); ok {
		return true
	}
	for _, hint := range []string{"ppt", "slide", "deck", "material"} {
		if strings.Contains(source, hint) {
			return true
		}
	}
	return false
}

func normalizePPTStylePreset(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "banana_slides", "bananaslides", "banana slides", "banana-slides":
		return "banana_slides"
	case "nano_slides", "nanoslides", "nano slides", "nano-slides":
		return "nano_slides"
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}

func compatFloat64(args map[string]interface{}, keys ...string) float64 {
	for _, key := range keys {
		value, ok := compatArgValue(args, key)
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case float64:
			return typed
		case float32:
			return float64(typed)
		case int:
			return float64(typed)
		case int64:
			return float64(typed)
		case string:
			parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
			if err == nil {
				return parsed
			}
		}
	}
	return 0
}

func compatValue(args map[string]interface{}, keys ...string) any {
	for _, key := range keys {
		if value, ok := compatArgValue(args, key); ok {
			return value
		}
	}
	return nil
}

func normalizeImageArgs(args map[string]interface{}) map[string]interface{} {
	normalized := make(map[string]interface{}, len(args)+8)
	for k, v := range args {
		normalized[k] = v
	}
	if strings.TrimSpace(asString(normalized["lang"])) == "" {
		if language := firstCompatString(normalized, "language", "locale"); language != "" {
			normalized["lang"] = language
		}
	}

	flattenImageTaskArgs(normalized)
	flattenImageGenerateArgs(normalized)
	flattenImageReviewArgs(normalized)
	flattenImageSlideArgs(normalized)
	return normalized
}

func flattenImageTaskArgs(normalized map[string]interface{}) {
	task, ok := coerceCompatMap(normalized["task"])
	if !ok || len(task) == 0 {
		return
	}
	if strings.TrimSpace(firstCompatString(normalized, "task_id", "taskId", "id")) == "" {
		if taskID := firstCompatString(task, "id", "task_id", "taskId"); taskID != "" {
			normalized["task_id"] = taskID
		}
	}
}

func flattenImageGenerateArgs(normalized map[string]interface{}) {
	generate, ok := coerceCompatMap(normalized["generate"])
	if !ok || len(generate) == 0 {
		return
	}
	if strings.TrimSpace(firstCompatString(normalized, "prompt", "query", "input", "text", "message", "content")) == "" {
		if prompt := firstCompatString(generate, "prompt", "query", "input", "text", "message", "content"); prompt != "" {
			normalized["prompt"] = prompt
		}
	}
	if strings.TrimSpace(firstCompatString(normalized, "model")) == "" {
		if value := firstCompatString(generate, "model"); value != "" {
			normalized["model"] = value
		}
	}
	if strings.TrimSpace(firstCompatString(normalized, "size")) == "" {
		if value := firstCompatString(generate, "size"); value != "" {
			normalized["size"] = value
		}
	}
	if compatInt(normalized, "n", "count", "num_images", "numImages") <= 0 {
		if value, ok := compatArgValue(generate, "count", "n", "num_images", "numImages"); ok {
			normalized["count"] = value
		}
	}
	if strings.TrimSpace(firstCompatPathString(normalized)) == "" {
		if value := firstCompatString(generate, "path", "output_path", "outputPath"); value != "" {
			normalized["path"] = value
		}
	}
	if strings.TrimSpace(firstCompatString(normalized, "negative_prompt", "negativePrompt")) == "" {
		if value := firstCompatString(generate, "negative_prompt", "negativePrompt", "negative"); value != "" {
			normalized["negative_prompt"] = value
		}
	}
	if strings.TrimSpace(firstCompatString(normalized, "quality")) == "" {
		if value := firstCompatString(generate, "quality"); value != "" {
			normalized["quality"] = value
		}
	}
	if strings.TrimSpace(firstCompatString(normalized, "style")) == "" {
		if value := firstCompatString(generate, "style"); value != "" {
			normalized["style"] = value
		}
	}
	if strings.TrimSpace(firstCompatString(normalized, "category", "mode", "type")) == "" {
		if value := firstCompatString(generate, "category", "mode", "type"); value != "" {
			normalized["category"] = value
		}
	}
	if strings.TrimSpace(firstCompatString(normalized, "aspect_ratio", "aspectRatio")) == "" {
		if value := firstCompatString(generate, "aspect_ratio", "aspectRatio", "aspect"); value != "" {
			normalized["aspect_ratio"] = value
		}
	}
	if strings.TrimSpace(firstCompatString(normalized, "resolution")) == "" {
		if value := firstCompatString(generate, "resolution"); value != "" {
			normalized["resolution"] = value
		}
	}
	if _, ok := compatArgValue(normalized, "poll", "wait", "sync"); !ok {
		if value, ok := compatArgValue(generate, "poll"); ok {
			normalized["poll"] = value
		} else if rawWait, exists := generate["wait"]; exists {
			if _, nested := coerceCompatMap(rawWait); !nested {
				normalized["poll"] = rawWait
			}
		}
	}
	if compatInt(normalized, "wait_timeout_sec", "waitTimeoutSec", "timeout_sec", "timeoutSec", "timeout_seconds", "timeoutSeconds") <= 0 {
		if value, ok := compatArgValue(generate, "wait_timeout_sec", "waitTimeoutSec", "timeout_sec", "timeoutSec", "timeout_seconds", "timeoutSeconds"); ok {
			normalized["wait_timeout_sec"] = value
		}
	}

	if output, ok := coerceCompatMap(generate["output"]); ok && len(output) > 0 {
		if strings.TrimSpace(firstCompatPathString(normalized)) == "" {
			if value := firstCompatString(output, "path", "output_path", "outputPath"); value != "" {
				normalized["path"] = value
			}
		}
		if strings.TrimSpace(firstCompatString(normalized, "size")) == "" {
			if value := firstCompatString(output, "size"); value != "" {
				normalized["size"] = value
			}
		}
		if compatInt(normalized, "n", "count", "num_images", "numImages") <= 0 {
			if value, ok := compatArgValue(output, "count", "n", "num_images", "numImages"); ok {
				normalized["count"] = value
			}
		}
	}

	if reference, ok := coerceCompatMap(generate["reference"]); ok && len(reference) > 0 {
		if strings.TrimSpace(firstCompatString(normalized, "reference_image", "referenceImage", "reference_url", "referenceUrl", "image_url", "imageUrl")) == "" {
			if value := firstCompatString(reference, "url", "reference_image", "referenceImage", "reference_url", "referenceUrl", "image_url", "imageUrl"); value != "" {
				normalized["reference_image"] = value
			}
		}
		if strings.TrimSpace(firstCompatString(normalized, "reference_base64", "referenceBase64", "reference_image_base64", "referenceImageBase64")) == "" {
			if value := firstCompatString(reference, "base64", "reference_base64", "referenceBase64", "reference_image_base64", "referenceImageBase64"); value != "" {
				normalized["reference_base64"] = value
			}
		}
	}

	if wait, ok := coerceCompatMap(generate["wait"]); ok && len(wait) > 0 {
		if _, ok := compatArgValue(normalized, "poll", "wait", "sync"); !ok {
			if value, ok := compatArgValue(wait, "enabled", "poll", "wait"); ok {
				normalized["poll"] = value
			}
		}
		if compatInt(normalized, "wait_timeout_sec", "waitTimeoutSec", "timeout_sec", "timeoutSec", "timeout_seconds", "timeoutSeconds") <= 0 {
			if value, ok := compatArgValue(wait, "timeout_sec", "timeoutSec", "wait_timeout_sec", "waitTimeoutSec"); ok {
				normalized["wait_timeout_sec"] = value
			}
		}
	}

	if options, ok := coerceCompatMap(generate["options"]); ok && len(options) > 0 {
		if strings.TrimSpace(firstCompatString(normalized, "negative_prompt", "negativePrompt")) == "" {
			if value := firstCompatString(options, "negative_prompt", "negativePrompt", "negative"); value != "" {
				normalized["negative_prompt"] = value
			}
		}
		if strings.TrimSpace(firstCompatString(normalized, "quality")) == "" {
			if value := firstCompatString(options, "quality"); value != "" {
				normalized["quality"] = value
			}
		}
		if strings.TrimSpace(firstCompatString(normalized, "style")) == "" {
			if value := firstCompatString(options, "style"); value != "" {
				normalized["style"] = value
			}
		}
		if strings.TrimSpace(firstCompatString(normalized, "category", "mode", "type")) == "" {
			if value := firstCompatString(options, "category", "mode", "type"); value != "" {
				normalized["category"] = value
			}
		}
		if strings.TrimSpace(firstCompatString(normalized, "aspect_ratio", "aspectRatio")) == "" {
			if value := firstCompatString(options, "aspect_ratio", "aspectRatio", "aspect"); value != "" {
				normalized["aspect_ratio"] = value
			}
		}
		if strings.TrimSpace(firstCompatString(normalized, "resolution")) == "" {
			if value := firstCompatString(options, "resolution"); value != "" {
				normalized["resolution"] = value
			}
		}
	}
}

func flattenImageReviewArgs(normalized map[string]interface{}) {
	review, ok := coerceCompatMap(normalized["review"])
	if !ok || len(review) == 0 {
		return
	}
	if strings.TrimSpace(firstCompatString(normalized, "prompt", "query", "input", "text", "message", "content")) == "" {
		if prompt := firstCompatString(review, "prompt", "query", "text", "message", "content"); prompt != "" {
			normalized["prompt"] = prompt
		}
	}
	if _, ok := compatArgValue(normalized, "compare"); !ok {
		if value, ok := compatArgValue(review, "compare"); ok {
			normalized["compare"] = value
		}
	}
	if compatInt(normalized, "max_images", "maxImages") <= 0 {
		if value, ok := compatArgValue(review, "max_images", "maxImages", "limit"); ok {
			normalized["max_images"] = value
		}
	}
	if strings.TrimSpace(firstCompatString(normalized, "analysis_mode", "analysisMode")) == "" {
		if value := firstCompatString(review, "analysis_mode", "analysisMode", "mode"); value != "" {
			normalized["analysis_mode"] = value
		}
	}
	if strings.TrimSpace(asString(normalized["lang"])) == "" {
		if language := firstCompatString(review, "language", "lang"); language != "" {
			normalized["lang"] = language
		}
	}
	if strings.TrimSpace(firstCompatString(normalized, "device")) == "" {
		if value := firstCompatString(review, "device"); value != "" {
			normalized["device"] = value
		}
	}
	if _, ok := compatArgValue(normalized, "wait_ms"); !ok {
		if value, ok := compatArgValue(review, "wait_ms", "waitMs"); ok {
			normalized["wait_ms"] = value
		}
	}
	if compatFloat64(normalized, "threshold") == 0 {
		if value, ok := compatArgValue(review, "threshold"); ok {
			normalized["threshold"] = value
		}
	}
	if strings.TrimSpace(firstCompatString(normalized, "format")) == "" {
		if value := firstCompatString(review, "format"); value != "" {
			normalized["format"] = value
		}
	}

	if inputs, ok := coerceCompatMap(review["inputs"]); ok && len(inputs) > 0 {
		flattenImageReviewInputFields(normalized, inputs)
	}
	flattenImageReviewInputFields(normalized, review)

	if options, ok := coerceCompatMap(review["options"]); ok && len(options) > 0 {
		if strings.TrimSpace(firstCompatString(normalized, "device")) == "" {
			if value := firstCompatString(options, "device"); value != "" {
				normalized["device"] = value
			}
		}
		if _, ok := compatArgValue(normalized, "wait_ms"); !ok {
			if value, ok := compatArgValue(options, "wait_ms", "waitMs"); ok {
				normalized["wait_ms"] = value
			}
		}
		if compatFloat64(normalized, "threshold") == 0 {
			if value, ok := compatArgValue(options, "threshold"); ok {
				normalized["threshold"] = value
			}
		}
		if strings.TrimSpace(firstCompatString(normalized, "format")) == "" {
			if value := firstCompatString(options, "format"); value != "" {
				normalized["format"] = value
			}
		}
	}
}

func flattenImageReviewInputFields(normalized, source map[string]interface{}) {
	if source == nil {
		return
	}
	if strings.TrimSpace(firstCompatString(normalized, "image", "image_base64", "imageBase64", "base64", "url", "href", "link", "image_path", "imagePath")) == "" {
		if value := firstCompatString(source, "image", "input"); value != "" {
			normalized["image"] = value
		}
		if value := firstCompatString(source, "url"); value != "" && strings.TrimSpace(firstCompatString(normalized, "url", "href", "link")) == "" {
			normalized["url"] = value
		}
		if value := firstCompatString(source, "path", "image_path", "imagePath", "file"); value != "" && strings.TrimSpace(firstCompatString(normalized, "image_path", "imagePath")) == "" {
			normalized["image_path"] = value
		}
	}
	if _, ok := compatArgValue(normalized, "images"); !ok {
		if value, ok := compatArgValue(source, "images", "inputs"); ok {
			if _, nested := coerceCompatMap(value); !nested {
				normalized["images"] = value
			}
		}
	}
	if _, ok := compatArgValue(normalized, "urls"); !ok {
		if value, ok := compatArgValue(source, "urls"); ok {
			normalized["urls"] = value
		}
	}
	if _, ok := compatArgValue(normalized, "image_paths", "imagePaths"); !ok {
		if value, ok := compatArgValue(source, "paths", "image_paths", "imagePaths", "files"); ok {
			normalized["image_paths"] = value
		}
	}
}

func flattenImageSlideArgs(normalized map[string]interface{}) {
	slide, ok := coerceCompatMap(normalized["slide"])
	if !ok || len(slide) == 0 {
		return
	}
	if _, ok := normalized["reference_images"]; !ok {
		if value, ok := compatArgValue(slide, "reference_images", "referenceImages", "reference_urls", "referenceUrls"); ok {
			normalized["reference_images"] = value
		}
	}
	if _, ok := normalized["layout_spec"]; !ok {
		if value, ok := compatArgValue(slide, "layout_spec", "layoutSpec", "ppt_layout_spec", "slide_layout_spec"); ok {
			normalized["layout_spec"] = value
		}
	}
	if strings.TrimSpace(asString(normalized["style_preset"])) == "" {
		if value := firstCompatString(slide, "style_preset", "stylePreset"); value != "" {
			normalized["style_preset"] = value
		}
	}
	if strings.TrimSpace(asString(normalized["quality_profile"])) == "" {
		if value := firstCompatString(slide, "quality_profile", "qualityProfile"); value != "" {
			normalized["quality_profile"] = value
		}
	}
	if _, ok := normalized["review_threshold"]; !ok {
		if value, ok := compatArgValue(slide, "review_threshold", "reviewThreshold"); ok {
			normalized["review_threshold"] = value
		}
	}
	if _, ok := normalized["review_retry_budget"]; !ok {
		if value, ok := compatArgValue(slide, "review_retry_budget", "reviewRetryBudget"); ok {
			normalized["review_retry_budget"] = value
		}
	}
	if review, ok := coerceCompatMap(slide["review"]); ok && len(review) > 0 {
		if _, ok := normalized["review_threshold"]; !ok {
			if value, ok := compatArgValue(review, "threshold", "review_threshold", "reviewThreshold"); ok {
				normalized["review_threshold"] = value
			}
		}
		if _, ok := normalized["review_retry_budget"]; !ok {
			if value, ok := compatArgValue(review, "retry_budget", "review_retry_budget", "reviewRetryBudget"); ok {
				normalized["review_retry_budget"] = value
			}
		}
	}
	if strings.TrimSpace(asString(normalized["style_theme"])) == "" {
		if value := firstCompatString(slide, "theme", "style_theme", "styleTheme", "brand_guidance", "brandGuidance"); value != "" {
			normalized["style_theme"] = value
		}
	}
	if strings.TrimSpace(asString(normalized["lang"])) == "" {
		if language := firstCompatString(slide, "language", "lang"); language != "" {
			normalized["lang"] = language
		}
	}
}

func imageAction(args map[string]interface{}) string {
	action := strings.ToLower(strings.TrimSpace(firstCompatString(args, "action", "op", "operation", "command")))
	switch action {
	case "", "auto":
		if firstCompatString(args, "task_id", "taskId", "id") != "" {
			return "status"
		}
		if firstCompatString(args, "reference_image", "referenceImage", "reference_url", "referenceUrl", "image_url", "imageUrl", "reference_base64", "referenceBase64", "reference_image_base64", "referenceImageBase64") != "" {
			return "generate"
		}
		if _, ok := compatArgValue(args, "images"); ok {
			return "review"
		}
		if _, ok := compatArgValue(args, "urls"); ok {
			return "review"
		}
		if _, ok := compatArgValue(args, "image_urls", "imageUrls"); ok {
			return "review"
		}
		if _, ok := compatArgValue(args, "image_paths", "imagePaths"); ok {
			return "review"
		}
		if firstCompatString(args, "image", "image_base64", "imageBase64", "base64", "url", "href", "link", "image_path", "imagePath") != "" {
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
	case "generate", "create", "draw", "edit", "ppt":
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
	native := NewImageTool(reviewer, generate, lookup)
	registry.Register(native)
	registry.Register(newGenerateImageTool(native))
	registry.Register(newOCRTool(native))
	if webTool := GetWebQueryTool(registry); webTool != nil {
		webTool.SetImageTool(native)
	}
	for _, alias := range []string{"image_generation", "generateImage"} {
		registry.Register(newImageCompatTool(alias, "Hidden legacy image generation alias.", native))
		registry.Disable(alias)
	}
}
