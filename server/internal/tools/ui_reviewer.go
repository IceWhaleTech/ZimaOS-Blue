package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/i18n"
	"github.com/google/uuid"
)

const (
	maxScrollPages   = 5
	defaultWaitMS    = 2000
	maxWaitMS        = 10000
	maxVLMImageWidth = 1024
	maxA11yTreeChars = 2000
	desktopWidth     = 1920
	desktopHeight    = 1080
	mobileWidth      = 375
	mobileHeight     = 812
)

// mobileChannels maps channel names that should default to mobile viewport.
var mobileChannels = map[string]bool{
	"telegram": true, "whatsapp": true, "discord": true, "slack": true,
	"feishu": true, "qq": true, "wechat": true, "signal": true,
	"viber": true, "messenger": true, "imessage": true, "line": true,
	"dingtalk": true, "zalo": true, "matrix": true, "instagram": true,
	"twitter": true, "twitch": true, "nostr": true, "googlechat": true,
	"mattermost": true, "nextcloudtalk": true,
}

// UIReviewerTool performs automated UI quality review.
type UIReviewerTool struct {
	mu       sync.RWMutex
	browser  UIReviewBrowser
	bridge   VLMBridge
	mediaDir string
}

// UIReviewBrowser defines the browser interface needed by the UI reviewer.
type UIReviewBrowser interface {
	Start(ctx context.Context) error
	NavigateURL(ctx context.Context, url string) (UIReviewNavResult, error)
	GetAccessibilityTree(ctx context.Context, targetID string, maxDepth int) (UIReviewA11yResult, error)
	ScreenshotTab(ctx context.Context, targetID string) (string, error)
	CloseTab(ctx context.Context, targetID string) error
	SetViewport(ctx context.Context, targetID string, width, height int) error
	ScrollTo(ctx context.Context, targetID string, x, y int) error
	PageDimensions(ctx context.Context, targetID string) (viewportH, scrollH int, err error)
	ScreenshotViewportRaw(ctx context.Context, targetID string) ([]byte, error)
}

// VLMBridge defines the interface for making VLM (vision language model) calls.
type VLMBridge interface {
	ChatWithVision(ctx context.Context, prompt string, imageBase64 string) (string, error)
}

// UIReviewNavResult is the result of a browser navigation.
type UIReviewNavResult struct {
	URL      string
	Title    string
	TargetID string
}

// UIReviewA11yResult is the result of an accessibility tree query.
type UIReviewA11yResult struct {
	Tree string
}

// --- Review result types ---

// UIReviewResult is the top-level output of a UI review.
type UIReviewResult struct {
	URL           string          `json:"url,omitempty"`
	Visual        UIScoreDetail   `json:"visual"`
	Functional    UIScoreDetail   `json:"functional"`
	Accessibility UIScoreDetail   `json:"accessibility"`
	Overall       float64         `json:"overall"`
	Pass          bool            `json:"pass"`
	Threshold     float64         `json:"threshold"`
	Issues        []UIReviewIssue `json:"issues"`
	Suggestions   []string        `json:"suggestions,omitempty"`
	Viewports     []string        `json:"viewports,omitempty"`
	Steps         []UIReviewStep  `json:"steps,omitempty"`
	Screenshot    string          `json:"screenshot,omitempty"`
	MediaURL      string          `json:"media_url,omitempty"`
	ThumbnailURL  string          `json:"thumbnail_url,omitempty"`
	Screenshots   []string        `json:"screenshots,omitempty"`
	Device        string          `json:"device,omitempty"`
	Channel       string          `json:"channel,omitempty"`
	Human         string          `json:"human,omitempty"`
}

// UIScoreDetail holds a category score and its sub-scores.
type UIScoreDetail struct {
	Score   float64            `json:"score"`
	Details map[string]float64 `json:"details,omitempty"`
}

// UIReviewIssue represents a single issue found during review.
type UIReviewIssue struct {
	Severity    string `json:"severity"`
	Category    string `json:"category"`
	Rule        string `json:"rule,omitempty"`
	Element     string `json:"element,omitempty"`
	Description string `json:"description"`
	Location    string `json:"location,omitempty"`
}

// UIReviewStep records one phase of the review pipeline.
type UIReviewStep struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Status  string  `json:"status"`
	Score   float64 `json:"score,omitempty"`
	Message string  `json:"message,omitempty"`
	Issues  int     `json:"issues,omitempty"`
}

type vlmParsedResult struct {
	Scores      map[string]float64 `json:"scores"`
	Issues      []UIReviewIssue    `json:"issues"`
	Suggestions []string           `json:"suggestions"`
}

type functionalResult struct {
	Score  float64
	Issues []UIReviewIssue
}

type a11yResult struct {
	Score  float64
	Issues []UIReviewIssue
	Tree   string // raw tree for structural fallback
}

// NewUIReviewerTool creates a new UI reviewer tool.
func NewUIReviewerTool() *UIReviewerTool {
	return &UIReviewerTool{}
}

// SetBrowser injects the browser service.
func (t *UIReviewerTool) SetBrowser(svc UIReviewBrowser) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.browser = svc
}

// SetVLMBridge injects the VLM bridge for visual review.
func (t *UIReviewerTool) SetVLMBridge(bridge VLMBridge) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.bridge = bridge
}

// SetMediaDir sets the directory for persisting screenshots.
func (t *UIReviewerTool) SetMediaDir(dir string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.mediaDir = dir
}

// Definition returns the tool's definition.
func (t *UIReviewerTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "ui_reviewer",
		Description: "Score and audit UI/UX quality of a URL or screenshot. Use only when asked to evaluate/rate/review visual design or accessibility. Not for browsing or searching.",
		Icon:        "eye",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"description": "Action: review_url (full review), review_image (VLM review of base64 image), check_accessibility (a11y check only)",
				},
				"url": map[string]interface{}{
					"type":        "string",
					"description": "Target URL (required for review_url, check_accessibility)",
				},
				"image": map[string]interface{}{
					"type":        "string",
					"description": "Base64-encoded screenshot (required for review_image)",
				},
				"lang": map[string]interface{}{
					"type":        "string",
					"description": "Language/locale for output (e.g. en-US, zh-CN). Default: from context or en-US",
				},
				"device": map[string]interface{}{
					"type":        "string",
					"description": "Device type: desktop (1920x1080) or mobile (375x812). Auto-detected from channel if not set.",
				},
				"channel": map[string]interface{}{
					"type":        "string",
					"description": "Source channel name (e.g. telegram, web). Used for device auto-detection.",
				},
				"wait_ms": map[string]interface{}{
					"type":        "number",
					"description": "Wait time in ms after page load (default: 2000, max: 10000). Useful for slow-loading pages.",
				},
				"threshold": map[string]interface{}{
					"type":        "number",
					"description": "Pass threshold (0-100). Default: 75",
					"default":     75.0,
				},
				"format": map[string]interface{}{
					"type":        "string",
					"description": "Output format: json or human. Default: json",
					"default":     "json",
				},
			},
			"required": []string{"action"},
		},
	}
}

// resolveDevice determines the device type from explicit param, channel, or context.
func resolveDevice(args map[string]interface{}, ctx context.Context) (device string, width, height int) {
	if d, ok := args["device"].(string); ok && (d == "desktop" || d == "mobile") {
		device = d
	}
	ch, _ := args["channel"].(string)
	if ch == "" {
		ch = GetChannel(ctx)
	}
	if device == "" {
		if d := GetDevice(ctx); d == "desktop" || d == "mobile" {
			device = d
		}
	}
	if device == "" && ch != "" {
		if mobileChannels[strings.ToLower(ch)] {
			device = "mobile"
		}
	}
	if device == "" {
		device = "desktop"
	}
	if device == "mobile" {
		return device, mobileWidth, mobileHeight
	}
	return device, desktopWidth, desktopHeight
}

// resolveLang gets the language from args or context.
func resolveLang(args map[string]interface{}, ctx context.Context) i18n.Language {
	if l, ok := args["lang"].(string); ok && l != "" {
		return i18n.Language(l)
	}
	return i18n.Language(GetLang(ctx))
}

// resolveWaitMS gets the wait time from args.
func resolveWaitMS(args map[string]interface{}) int {
	if v, ok := args["wait_ms"].(float64); ok && v > 0 {
		ms := int(v)
		if ms > maxWaitMS {
			ms = maxWaitMS
		}
		return ms
	}
	return defaultWaitMS
}

// Execute runs the UI review tool.
func (t *UIReviewerTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	action, _ := args["action"].(string)
	if action == "" {
		return nil, errors.New("action is required")
	}

	threshold := 75.0
	if v, ok := args["threshold"].(float64); ok && v > 0 {
		threshold = v
	}
	format := "json"
	if f, ok := args["format"].(string); ok && f != "" {
		format = f
	}
	lang := resolveLang(args, ctx)

	var result *UIReviewResult
	var err error

	switch action {
	case "review_url":
		url, _ := args["url"].(string)
		if url == "" {
			return nil, errors.New("url is required for review_url")
		}
		result, err = t.reviewURL(ctx, url, args, threshold, format, lang)
	case "review_image":
		img, _ := args["image"].(string)
		if img == "" {
			return nil, errors.New("image is required for review_image")
		}
		result, err = t.reviewImage(ctx, img, threshold, format, lang)
	case "check_accessibility":
		url, _ := args["url"].(string)
		if url == "" {
			return nil, errors.New("url is required for check_accessibility")
		}
		result, err = t.checkAccessibility(ctx, url, lang)
		if err == nil {
			b, _ := json.Marshal(result)
			return string(b), nil
		}
		return nil, err
	default:
		return nil, fmt.Errorf("invalid action: %s (valid: review_url, review_image, check_accessibility)", action)
	}

	if err != nil {
		return nil, err
	}

	b, _ := json.Marshal(result)
	return string(b), nil
}

// --- review_url: full review pipeline ---

func (t *UIReviewerTool) reviewURL(ctx context.Context, url string, args map[string]interface{}, threshold float64, format string, lang i18n.Language) (*UIReviewResult, error) {
	t.mu.RLock()
	browser := t.browser
	bridge := t.bridge
	mediaDir := t.mediaDir
	t.mu.RUnlock()

	if browser == nil {
		return nil, errors.New("browser service not available — cannot review URL")
	}

	device, vpWidth, vpHeight := resolveDevice(args, ctx)
	ch, _ := args["channel"].(string)
	if ch == "" {
		ch = GetChannel(ctx)
	}
	waitMS := resolveWaitMS(args)
	var steps []UIReviewStep

	// 1. Start browser
	if err := browser.Start(ctx); err != nil {
		return nil, fmt.Errorf("browser start failed: %w", err)
	}

	// 2. Navigate
	emitUIProgress(ctx, "navigate", i18n.T(lang, i18n.MsgStepPageLoad), "running", url, nil)
	nav, err := browser.NavigateURL(ctx, url)
	if err != nil {
		emitUIProgress(ctx, "navigate", i18n.T(lang, i18n.MsgStepPageLoad), "failed", url, nil)
		return nil, fmt.Errorf("navigation failed: %w", err)
	}
	steps = append(steps, UIReviewStep{ID: "navigate", Name: i18n.T(lang, i18n.MsgStepPageLoad), Status: "success"})
	emitUIProgress(ctx, "navigate", i18n.T(lang, i18n.MsgStepPageLoad), "success", url, nil)

	// 3. Set viewport based on device
	if err := browser.SetViewport(ctx, nav.TargetID, vpWidth, vpHeight); err == nil {
		steps = append(steps, UIReviewStep{ID: "viewport", Name: i18n.T(lang, i18n.MsgStepViewport), Status: "success", Message: fmt.Sprintf("%s %dx%d", device, vpWidth, vpHeight)})
	}

	// 4. Wait for page load
	if waitMS > 0 {
		time.Sleep(time.Duration(waitMS) * time.Millisecond)
	}
	emitUIStageCard(ctx, lang, "info", uiReviewLocalized(lang,
		"Page loaded and viewport ready",
		"页面已加载，视口已就绪",
	), []map[string]interface{}{
		{"label": "url", "value": url},
		{"label": "device", "value": device},
		{"label": "viewport", "value": fmt.Sprintf("%dx%d", vpWidth, vpHeight)},
		{"label": "wait_ms", "value": waitMS},
	})

	// 5. Functional checks
	emitUIProgress(ctx, "functional", i18n.T(lang, i18n.MsgStepFunctional), "running", url, nil)
	funcResult := t.runFunctionalChecks(ctx, browser, nav.TargetID, lang)
	steps = append(steps, UIReviewStep{
		ID: "functional", Name: i18n.T(lang, i18n.MsgStepFunctional), Status: "success",
		Score: funcResult.Score, Issues: len(funcResult.Issues),
	})
	emitUIProgress(ctx, "functional", i18n.T(lang, i18n.MsgStepFunctional), "success", url, &funcResult.Score)

	// 6. Accessibility checks
	emitUIProgress(ctx, "accessibility", i18n.T(lang, i18n.MsgStepAccessibility), "running", url, nil)
	a11y := t.runA11yChecks(ctx, browser, nav.TargetID, lang)
	steps = append(steps, UIReviewStep{
		ID: "accessibility", Name: i18n.T(lang, i18n.MsgStepAccessibility), Status: "success",
		Score: a11y.Score, Issues: len(a11y.Issues),
	})
	emitUIProgress(ctx, "accessibility", i18n.T(lang, i18n.MsgStepAccessibility), "success", url, &a11y.Score)
	emitUIStageCard(ctx, lang, "info", uiReviewLocalized(lang,
		"Core audits completed",
		"核心审查已完成",
	), []map[string]interface{}{
		{"label": "functional_score", "value": funcResult.Score},
		{"label": "functional_issues", "value": len(funcResult.Issues)},
		{"label": "accessibility_score", "value": a11y.Score},
		{"label": "accessibility_issues", "value": len(a11y.Issues)},
	})

	// 7. Multi-page scroll + screenshots
	emitUIProgress(ctx, "screenshot", i18n.T(lang, i18n.MsgStepScreenshot), "running", url, nil)
	screenshots, firstScreenshot := t.captureScrollScreenshots(ctx, browser, nav.TargetID, mediaDir, &steps, lang)
	emitUIProgress(ctx, "screenshot", i18n.T(lang, i18n.MsgStepScreenshot), "success", url, nil)
	emitUIStageCard(ctx, lang, "info", uiReviewLocalized(lang,
		"Page screenshots captured",
		"页面截图已捕获",
	), []map[string]interface{}{
		{"label": "screenshots", "value": len(screenshots)},
	})

	// 8. VLM visual review (or structural fallback)
	var vlm *vlmParsedResult
	var structuralScore float64
	structuralUsed := false
	if bridge != nil && firstScreenshot != "" {
		emitUIProgress(ctx, "visual", i18n.T(lang, i18n.MsgStepVisualReview), "running", url, nil)
		vlm = t.runVLMReview(ctx, bridge, firstScreenshot, url, lang)
		if vlm != nil {
			vlmScore := uiAvgScores(vlm.Scores)
			steps = append(steps, UIReviewStep{
				ID: "visual", Name: i18n.T(lang, i18n.MsgStepVisualReview), Status: "success",
				Score: vlmScore, Issues: len(vlm.Issues),
			})
			emitUIProgress(ctx, "visual", i18n.T(lang, i18n.MsgStepVisualReview), "success", url, &vlmScore)
		} else {
			steps = append(steps, UIReviewStep{ID: "visual", Name: i18n.T(lang, i18n.MsgStepVisualReview), Status: "failed", Message: i18n.T(lang, i18n.MsgStepVLMFailed)})
			emitUIProgress(ctx, "visual", i18n.T(lang, i18n.MsgStepVisualReview), "failed", url, nil)
		}
	} else if bridge == nil {
		steps = append(steps, UIReviewStep{ID: "visual", Name: i18n.T(lang, i18n.MsgStepVisualReview), Status: "skipped", Message: i18n.T(lang, i18n.MsgStepVLMSkipped)})
	}

	// If no VLM, run structural fallback from a11y tree
	if vlm == nil && a11y.Tree != "" {
		structuralScore = t.structuralScore(a11y.Tree)
		structuralUsed = true
		steps = append(steps, UIReviewStep{
			ID: "structural", Name: i18n.T(lang, i18n.MsgStepStructural), Status: "success",
			Score: structuralScore, Message: i18n.T(lang, i18n.MsgStepStructuralMsg),
		})
	}

	switch {
	case vlm != nil:
		emitUIStageCard(ctx, lang, "info", uiReviewLocalized(lang,
			"Visual review completed",
			"视觉评审已完成",
		), []map[string]interface{}{
			{"label": "visual_score", "value": uiAvgScores(vlm.Scores)},
			{"label": "visual_issues", "value": len(vlm.Issues)},
		})
	case structuralUsed:
		emitUIStageCard(ctx, lang, "warning", uiReviewLocalized(lang,
			"Visual model unavailable; used structural fallback",
			"视觉模型不可用，已改用结构化兜底",
		), []map[string]interface{}{
			{"label": "structural_score", "value": structuralScore},
		})
	}

	// 9. Compute scores
	result := t.computeScores(url, []string{device}, funcResult, a11y, vlm, threshold)
	result.Steps = steps
	result.Device = device
	result.Channel = ch
	result.Screenshots = screenshots

	// Set primary media URL (first screenshot)
	if len(screenshots) > 0 {
		result.MediaURL = screenshots[0]
		result.ThumbnailURL = screenshots[0] // same file, frontend can CSS-resize
	}

	if vlm == nil {
		result.Suggestions = append(result.Suggestions, i18n.T(lang, i18n.MsgSuggestionNoVLM))
	}

	_ = browser.CloseTab(ctx, nav.TargetID)

	if format == "human" {
		result.Human = formatUIHumanReport(result, lang)
	}

	return result, nil
}

// captureScrollScreenshots scrolls through the page and captures screenshots.
// Returns media URLs and the first screenshot as base64 for VLM.
func (t *UIReviewerTool) captureScrollScreenshots(ctx context.Context, browser UIReviewBrowser, targetID, mediaDir string, steps *[]UIReviewStep, lang i18n.Language) (mediaURLs []string, firstBase64 string) {
	viewportH, scrollH, err := browser.PageDimensions(ctx, targetID)
	if err != nil || viewportH <= 0 {
		// Fallback: single screenshot
		raw, err := browser.ScreenshotViewportRaw(ctx, targetID)
		if err != nil {
			*steps = append(*steps, UIReviewStep{ID: "screenshot", Name: i18n.T(lang, i18n.MsgStepScreenshot), Status: "failed", Message: err.Error()})
			return nil, ""
		}
		url := t.saveScreenshot(raw, mediaDir)
		b64 := resizeAndEncode(raw, maxVLMImageWidth)
		*steps = append(*steps, UIReviewStep{ID: "screenshot", Name: i18n.T(lang, i18n.MsgStepScreenshot), Status: "success"})
		if url != "" {
			return []string{url}, b64
		}
		return nil, b64
	}

	pages := (scrollH + viewportH - 1) / viewportH
	if pages > maxScrollPages {
		pages = maxScrollPages
	}
	if pages < 1 {
		pages = 1
	}

	for p := 0; p < pages; p++ {
		if p > 0 {
			_ = browser.ScrollTo(ctx, targetID, 0, p*viewportH)
			time.Sleep(300 * time.Millisecond) // brief settle
		}
		raw, err := browser.ScreenshotViewportRaw(ctx, targetID)
		if err != nil {
			continue
		}
		url := t.saveScreenshot(raw, mediaDir)
		if url != "" {
			mediaURLs = append(mediaURLs, url)
		}
		if p == 0 {
			firstBase64 = resizeAndEncode(raw, maxVLMImageWidth)
		}
	}

	stepMsg := fmt.Sprintf("%d/%d", len(mediaURLs), pages)
	if pages > 1 {
		*steps = append(*steps, UIReviewStep{ID: "scroll", Name: i18n.T(lang, i18n.MsgStepScroll), Status: "success", Message: stepMsg})
	}
	*steps = append(*steps, UIReviewStep{ID: "screenshot", Name: i18n.T(lang, i18n.MsgStepScreenshot), Status: "success", Message: stepMsg})

	return mediaURLs, firstBase64
}

// saveScreenshot persists a PNG to disk and returns the media URL path.
func (t *UIReviewerTool) saveScreenshot(pngData []byte, mediaDir string) string {
	if mediaDir == "" || len(pngData) == 0 {
		return ""
	}
	dir := filepath.Join(mediaDir, "ui-review")
	_ = os.MkdirAll(dir, 0750)
	id := uuid.New().String()
	filename := id + ".png"
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, pngData, 0644); err != nil {
		return ""
	}
	return "/api/v1/media/ui-review/" + filename
}

// resizeAndEncode resizes a PNG to maxWidth (if wider) and returns base64.
func resizeAndEncode(pngData []byte, maxWidth int) string {
	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return base64.StdEncoding.EncodeToString(pngData)
	}
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= maxWidth {
		return base64.StdEncoding.EncodeToString(pngData)
	}

	// Simple nearest-neighbor downscale
	newW := maxWidth
	newH := h * maxWidth / w
	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	for y := 0; y < newH; y++ {
		srcY := y * h / newH
		for x := 0; x < newW; x++ {
			srcX := x * w / newW
			dst.Set(x, y, img.At(bounds.Min.X+srcX, bounds.Min.Y+srcY))
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return base64.StdEncoding.EncodeToString(pngData)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

// --- review_image: VLM-only review ---

func (t *UIReviewerTool) reviewImage(ctx context.Context, imageB64 string, threshold float64, format string, lang i18n.Language) (*UIReviewResult, error) {
	t.mu.RLock()
	bridge := t.bridge
	t.mu.RUnlock()

	if bridge == nil {
		return nil, errors.New("VLM bridge not available — cannot review image")
	}

	vlm := t.runVLMReview(ctx, bridge, imageB64, "", lang)

	result := &UIReviewResult{Threshold: threshold}

	if vlm != nil {
		result.Visual = UIScoreDetail{
			Score:   uiAvgScores(vlm.Scores),
			Details: vlm.Scores,
		}
		result.Issues = vlm.Issues
		result.Suggestions = vlm.Suggestions
		result.Overall = result.Visual.Score
		result.Pass = result.Overall >= threshold
	} else {
		result.Overall = 0
		result.Pass = false
		result.Issues = append(result.Issues, UIReviewIssue{
			Severity:    "critical",
			Category:    "visual",
			Description: i18n.T(lang, i18n.MsgIssueVLMFailed),
		})
	}

	if format == "human" {
		result.Human = formatUIHumanReport(result, lang)
	}

	return result, nil
}

// --- check_accessibility: a11y-only ---

func (t *UIReviewerTool) checkAccessibility(ctx context.Context, url string, lang i18n.Language) (*UIReviewResult, error) {
	t.mu.RLock()
	browser := t.browser
	t.mu.RUnlock()

	if browser == nil {
		return nil, errors.New("browser service not available")
	}

	if err := browser.Start(ctx); err != nil {
		return nil, fmt.Errorf("browser start failed: %w", err)
	}

	nav, err := browser.NavigateURL(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("navigation failed: %w", err)
	}

	a11y := t.runA11yChecks(ctx, browser, nav.TargetID, lang)
	_ = browser.CloseTab(ctx, nav.TargetID)

	return &UIReviewResult{
		URL:           url,
		Accessibility: UIScoreDetail{Score: a11y.Score},
		Overall:       a11y.Score,
		Pass:          a11y.Score >= 75,
		Threshold:     75,
		Issues:        a11y.Issues,
	}, nil
}

// --- Functional checks ---

func (t *UIReviewerTool) runFunctionalChecks(ctx context.Context, browser UIReviewBrowser, targetID string, lang i18n.Language) *functionalResult {
	result := &functionalResult{Score: 100}

	_, err := browser.GetAccessibilityTree(ctx, targetID, 3)
	if err != nil {
		result.Score -= 40
		result.Issues = append(result.Issues, UIReviewIssue{
			Severity:    "critical",
			Category:    "functional",
			Description: i18n.T(lang, i18n.MsgIssueNoA11yTree),
		})
	}

	return result
}

// --- Accessibility checks ---

func (t *UIReviewerTool) runA11yChecks(ctx context.Context, browser UIReviewBrowser, targetID string, lang i18n.Language) *a11yResult {
	result := &a11yResult{Score: 100}

	a11y, err := browser.GetAccessibilityTree(ctx, targetID, 5)
	if err != nil {
		result.Score = 50
		result.Issues = append(result.Issues, UIReviewIssue{
			Severity:    "major",
			Category:    "accessibility",
			Description: i18n.T(lang, i18n.MsgIssueA11yTreeFail),
		})
		return result
	}

	tree := a11y.Tree
	result.Tree = tree

	if !strings.Contains(tree, "heading") && !strings.Contains(tree, "Heading") {
		result.Issues = append(result.Issues, UIReviewIssue{
			Severity:    "minor",
			Category:    "accessibility",
			Rule:        "heading-structure",
			Description: i18n.T(lang, i18n.MsgIssueNoHeadingA11y),
		})
	}

	if !strings.Contains(tree, "navigation") && !strings.Contains(tree, "Navigation") && !strings.Contains(tree, "nav") {
		result.Issues = append(result.Issues, UIReviewIssue{
			Severity:    "minor",
			Category:    "accessibility",
			Rule:        "landmark-nav",
			Description: i18n.T(lang, i18n.MsgIssueNoNavA11y),
		})
	}

	for _, issue := range result.Issues {
		switch issue.Severity {
		case "critical":
			result.Score -= 25
		case "major":
			result.Score -= 10
		case "minor":
			result.Score -= 3
		}
	}
	if result.Score < 0 {
		result.Score = 0
	}

	return result
}

// --- VLM visual review ---

func (t *UIReviewerTool) runVLMReview(ctx context.Context, bridge VLMBridge, screenshotBase64, url string, lang i18n.Language) *vlmParsedResult {
	prompt := buildVLMPrompt(url, lang)

	content, err := bridge.ChatWithVision(ctx, prompt, screenshotBase64)
	if err != nil {
		return nil
	}

	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var result vlmParsedResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil
	}

	for i := range result.Issues {
		result.Issues[i].Category = "visual"
	}

	return &result
}

func buildVLMPrompt(url string, lang i18n.Language) string {
	langName := "English"
	if strings.HasPrefix(string(lang), "zh") {
		langName = "Chinese"
	} else if strings.HasPrefix(string(lang), "ja") {
		langName = "Japanese"
	} else if strings.HasPrefix(string(lang), "ko") {
		langName = "Korean"
	}

	var b strings.Builder
	b.WriteString(`You are a UI/UX expert reviewer. Analyze this screenshot and provide a structured quality assessment.

Return ONLY valid JSON (no markdown, no code fences) in this exact format:
{
  "scores": {
    "visual_hierarchy": <0-100>,
    "layout_alignment": <0-100>,
    "color_harmony": <0-100>,
    "typography": <0-100>,
    "professionalism": <0-100>
  },
  "issues": [
    {"severity": "critical|major|minor", "description": "...", "location": "..."}
  ],
  "suggestions": ["..."]
}

Scoring criteria:
- visual_hierarchy: Clear content hierarchy, proper heading sizes, visual weight distribution
- layout_alignment: Consistent spacing, grid alignment, proper margins
- color_harmony: Cohesive palette, sufficient contrast, appropriate use of color
- typography: Readable fonts, proper line height, consistent sizing
- professionalism: Overall polish, no broken layouts, production-ready appearance

Be concise. Focus on actionable issues.`)

	fmt.Fprintf(&b, "\n\nRespond in %s. Return all description and suggestion text in %s.", langName, langName)

	if url != "" {
		fmt.Fprintf(&b, "\n\nURL: %s", url)
	}

	return b.String()
}

// --- Structural fallback score (when no VLM) ---

func (t *UIReviewerTool) structuralScore(tree string) float64 {
	if len(tree) > maxA11yTreeChars {
		tree = tree[:maxA11yTreeChars]
	}

	score := 50.0 // base

	headings := strings.Count(strings.ToLower(tree), "heading")
	if headings > 0 {
		score += 10
	}
	if headings > 3 {
		score += 5
	}

	landmarks := strings.Count(strings.ToLower(tree), "navigation") +
		strings.Count(strings.ToLower(tree), "banner") +
		strings.Count(strings.ToLower(tree), "main") +
		strings.Count(strings.ToLower(tree), "contentinfo")
	if landmarks > 0 {
		score += 10
	}

	interactive := strings.Count(strings.ToLower(tree), "button") +
		strings.Count(strings.ToLower(tree), "link") +
		strings.Count(strings.ToLower(tree), "textbox")
	if interactive > 3 {
		score += 10
	}

	// Content density
	lines := strings.Count(tree, "\n")
	if lines > 20 {
		score += 10
	}
	if lines > 50 {
		score += 5
	}

	if score > 100 {
		score = 100
	}
	return score
}

// --- Scoring ---

func (t *UIReviewerTool) computeScores(url string, viewports []string, funcRes *functionalResult, a11yRes *a11yResult, vlm *vlmParsedResult, threshold float64) *UIReviewResult {
	result := &UIReviewResult{
		URL:       url,
		Threshold: threshold,
		Viewports: viewports,
	}

	result.Functional = UIScoreDetail{Score: funcRes.Score}
	result.Issues = append(result.Issues, funcRes.Issues...)

	result.Accessibility = UIScoreDetail{Score: a11yRes.Score}
	result.Issues = append(result.Issues, a11yRes.Issues...)

	if vlm != nil {
		result.Visual = UIScoreDetail{
			Score:   uiAvgScores(vlm.Scores),
			Details: vlm.Scores,
		}
		result.Issues = append(result.Issues, vlm.Issues...)
		result.Suggestions = vlm.Suggestions
	} else {
		result.Overall = result.Functional.Score*0.55 + result.Accessibility.Score*0.45
		result.Pass = result.Overall >= threshold && result.Functional.Score >= 70 && !uiHasCritical(result.Issues)
		return result
	}

	result.Overall = result.Visual.Score*0.30 + result.Functional.Score*0.40 + result.Accessibility.Score*0.30
	result.Pass = result.Overall >= threshold && result.Functional.Score >= 70 && !uiHasCritical(result.Issues)

	return result
}

// --- Output formatting ---

func formatUIHumanReport(r *UIReviewResult, lang i18n.Language) string {
	var b strings.Builder

	b.WriteString("═══════════════════════════════════════\n")
	b.WriteString("  ")
	b.WriteString(i18n.T(lang, i18n.MsgUIReviewTitle))
	if r.URL != "" {
		b.WriteString(" - ")
		b.WriteString(r.URL)
	}
	b.WriteByte('\n')
	b.WriteString("═══════════════════════════════════════\n")

	passStr := i18n.T(lang, i18n.MsgUIReviewFail)
	if r.Pass {
		passStr = i18n.T(lang, i18n.MsgUIReviewPass)
	}
	b.WriteString(i18n.T(lang, i18n.MsgUIReviewOverall))
	b.WriteString(": ")
	b.WriteString(strconv.FormatFloat(r.Overall, 'f', 1, 64))
	b.WriteString(" / 100  ")
	b.WriteString(passStr)
	b.WriteString("\n\n")

	if r.Visual.Score > 0 {
		b.WriteString("  ")
		b.WriteString(i18n.T(lang, i18n.MsgUIReviewVisual))
		b.WriteString("  ")
		b.WriteString(strconv.FormatFloat(r.Visual.Score, 'f', 0, 64))
		b.WriteByte('\n')
	}
	b.WriteString("  ")
	b.WriteString(i18n.T(lang, i18n.MsgUIReviewFunctional))
	b.WriteString("  ")
	b.WriteString(strconv.FormatFloat(r.Functional.Score, 'f', 0, 64))
	b.WriteString("\n  ")
	b.WriteString(i18n.T(lang, i18n.MsgUIReviewAccessibility))
	b.WriteString("  ")
	b.WriteString(strconv.FormatFloat(r.Accessibility.Score, 'f', 0, 64))
	b.WriteByte('\n')

	if r.Device != "" {
		b.WriteString("\n  Device: ")
		b.WriteString(r.Device)
		if r.Channel != "" {
			b.WriteString(" (")
			b.WriteString(r.Channel)
			b.WriteByte(')')
		}
		b.WriteByte('\n')
	}

	if len(r.Issues) > 0 {
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf(i18n.T(lang, i18n.MsgUIReviewIssues), len(r.Issues)))
		b.WriteString(":\n")
		for _, issue := range r.Issues {
			icon := i18n.T(lang, i18n.MsgUIReviewMinor)
			switch issue.Severity {
			case "critical":
				icon = i18n.T(lang, i18n.MsgUIReviewCritical)
			case "major":
				icon = i18n.T(lang, i18n.MsgUIReviewMajor)
			}
			b.WriteString("  [")
			b.WriteString(icon)
			b.WriteString("] ")
			b.WriteString(issue.Description)
			b.WriteByte('\n')
		}
	}

	if len(r.Suggestions) > 0 {
		b.WriteString("\n")
		b.WriteString(i18n.T(lang, i18n.MsgUIReviewSuggestions))
		b.WriteString(":\n")
		for _, s := range r.Suggestions {
			b.WriteString("  - ")
			b.WriteString(s)
			b.WriteByte('\n')
		}
	}

	if len(r.Screenshots) > 0 {
		b.WriteString("\nScreenshots:\n")
		for i, url := range r.Screenshots {
			fmt.Fprintf(&b, "  [%d] %s\n", i+1, url)
		}
	}

	b.WriteString("═══════════════════════════════════════\n")
	return b.String()
}

// --- Helpers ---

func uiAvgScores(scores map[string]float64) float64 {
	if len(scores) == 0 {
		return 0
	}
	var sum float64
	for _, v := range scores {
		sum += v
	}
	return sum / float64(len(scores))
}

func uiHasCritical(issues []UIReviewIssue) bool {
	for _, issue := range issues {
		if issue.Severity == "critical" {
			return true
		}
	}
	return false
}

// emitUIProgress pushes a streaming progress card to the client via the
// context's card emitter. This lets the frontend show real-time step updates
// while the review is running.
func emitUIProgress(ctx context.Context, stepID, stepName, status, url string, score *float64) {
	card := map[string]interface{}{
		"type":   "ui-review-progress",
		"step":   stepID,
		"name":   stepName,
		"status": status,
	}
	if url != "" {
		card["url"] = url
	}
	if score != nil {
		card["score"] = *score
	}
	EmitCard(ctx, card)
}

func emitUIStageCard(ctx context.Context, lang i18n.Language, status, message string, details []map[string]interface{}) {
	card := map[string]interface{}{
		"type":    "result",
		"title":   "ui_review",
		"status":  status,
		"message": message,
	}
	if len(details) > 0 {
		card["details"] = details
	}
	EmitCard(ctx, card)
}

func uiReviewLocalized(lang i18n.Language, en, zh string) string {
	if strings.HasPrefix(strings.ToLower(string(lang)), "zh") {
		return zh
	}
	return en
}
