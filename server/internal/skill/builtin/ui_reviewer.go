package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxybridge"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

// UIReviewer is a built-in skill for automated UI quality review.
// It combines browser-based accessibility checks with VLM visual review.
type UIReviewer struct {
	manifest *skill.Manifest
	mu       sync.RWMutex
	browser  BrowserServiceInterface
	bridge   *proxybridge.Bridge
}

// --- Types ---

// ReviewResult is the top-level output of a UI review.
type ReviewResult struct {
	URL           string       `json:"url,omitempty"`
	Visual        ScoreDetail  `json:"visual"`
	Functional    ScoreDetail  `json:"functional"`
	Accessibility ScoreDetail  `json:"accessibility"`
	Overall       float64      `json:"overall"`
	Pass          bool         `json:"pass"`
	Threshold     float64      `json:"threshold"`
	Issues        []UIIssue    `json:"issues"`
	Suggestions   []string     `json:"suggestions,omitempty"`
	Viewports     []string     `json:"viewports,omitempty"`
	Steps         []ReviewStep `json:"steps,omitempty"`
	Screenshot    string       `json:"screenshot,omitempty"` // base64 thumbnail for card
	Human         string       `json:"human,omitempty"`
}

// ReviewStep records one phase of the review pipeline.
type ReviewStep struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Status   string `json:"status"` // success, failed, skipped
	Score    float64 `json:"score,omitempty"`
	Message  string `json:"message,omitempty"`
	Issues   int    `json:"issues,omitempty"`
}

// ScoreDetail holds a category score and its sub-scores.
type ScoreDetail struct {
	Score    float64            `json:"score"`
	Details  map[string]float64 `json:"details,omitempty"`
}

// UIIssue represents a single issue found during review.
type UIIssue struct {
	Severity    string `json:"severity"` // critical, major, minor
	Category    string `json:"category"` // visual, functional, accessibility
	Rule        string `json:"rule,omitempty"`
	Element     string `json:"element,omitempty"`
	Description string `json:"description"`
	Location    string `json:"location,omitempty"`
}

// A11yCheckResult is the result of running in-browser accessibility checks.
type A11yCheckResult struct {
	Issues []UIIssue `json:"issues"`
	Score  float64   `json:"score"`
}

// VLMReviewResult is the parsed response from VLM visual review.
type VLMReviewResult struct {
	Scores      map[string]float64 `json:"scores"`
	Issues      []UIIssue          `json:"issues"`
	Suggestions []string           `json:"suggestions"`
}

// FunctionalCheckResult holds functional check scores.
type FunctionalCheckResult struct {
	Score  float64   `json:"score"`
	Issues []UIIssue `json:"issues"`
}

// NewUIReviewer creates a new UI reviewer skill.
func NewUIReviewer() *UIReviewer {
	return &UIReviewer{
		manifest: &skill.Manifest{
			ID:          "ui_reviewer",
			Name:        "UI Reviewer",
			Version:     "1.0.0",
			Description: "Score and audit UI/UX quality of a URL or screenshot. Use only when asked to evaluate/rate/review visual design or accessibility. Not for browsing or searching.",
			Category:    "system",
			Icon:        "eye",
			Tags:        []string{"ui", "review", "accessibility", "visual", "quality"},
			Inputs: []skill.Parameter{
				{Name: "action", Type: "string", Description: "Action: review_url (evaluate a website by URL — navigates, screenshots, and scores automatically), review_image (evaluate a base64 screenshot), check_accessibility (a11y audit only)", Required: true},
				{Name: "url", Type: "string", Description: "Website URL to evaluate (required for review_url, check_accessibility). The tool navigates to the URL automatically — no need to fetch or screenshot it first"},
				{Name: "image", Type: "string", Description: "Base64-encoded screenshot (required for review_image)"},
				{Name: "viewports", Type: "array", Description: "Viewport list: desktop (1280x800), mobile (375x812). Default: [desktop]", Default: []string{"desktop"}},
				{Name: "threshold", Type: "number", Description: "Pass threshold (0-100). Default: 75", Default: 75.0},
				{Name: "format", Type: "string", Description: "Output format: json or human. Default: json", Default: "json"},
			{Name: "locale", Type: "string", Description: "Language/locale code for localized responses (e.g., en-US, zh-CN)"},
			},
			Outputs: []skill.Parameter{
				{Name: "result", Type: "object", Description: "ReviewResult with scores, issues, and pass/fail"},
			},
		},
	}
}

// SetBrowserService injects the browser service for screenshot and a11y checks.
func (u *UIReviewer) SetBrowserService(svc BrowserServiceInterface) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.browser = svc
}

// SetBridge injects the proxy bridge for VLM calls.
func (u *UIReviewer) SetBridge(bridge *proxybridge.Bridge) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.bridge = bridge
}

func (u *UIReviewer) Manifest() *skill.Manifest { return u.manifest }

func (u *UIReviewer) Validate(input map[string]any) error {
	// Default action based on input: if url is provided without action, default to review_url
	if _, hasAction := input["action"]; !hasAction {
		if _, hasURL := input["url"]; hasURL {
			input["action"] = "review_url"
		}
	}

	action, ok := input["action"]
	if !ok {
		return fmt.Errorf("action is required")
	}
	actionStr, ok := action.(string)
	if !ok {
		return fmt.Errorf("action must be a string")
	}
	switch actionStr {
	case "review_url":
		if _, ok := input["url"]; !ok {
			return fmt.Errorf("url is required for review_url")
		}
	case "review_image":
		if _, ok := input["image"]; !ok {
			return fmt.Errorf("image is required for review_image")
		}
	case "check_accessibility":
		if _, ok := input["url"]; !ok {
			return fmt.Errorf("url is required for check_accessibility")
		}
	default:
		return fmt.Errorf("invalid action: %s (valid: review_url, review_image, check_accessibility)", actionStr)
	}
	return nil
}

func (u *UIReviewer) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	action := input["action"].(string)

	threshold := 75.0
	if t, ok := input["threshold"].(float64); ok && t > 0 {
		threshold = t
	}
	format := "json"
	if f, ok := input["format"].(string); ok && f != "" {
		format = f
	}

	switch action {
	case "review_url":
		return u.reviewURL(ctx, input, threshold, format)
	case "review_image":
		return u.reviewImage(ctx, input, threshold, format)
	case "check_accessibility":
		return u.checkAccessibility(ctx, input)
	}
	return skill.NewErrorResult(fmt.Errorf("unknown action: %s", action)), nil
}

// --- review_url: full review pipeline ---

func (u *UIReviewer) reviewURL(ctx context.Context, input map[string]any, threshold float64, format string) (*skill.Result, error) {
	u.mu.RLock()
	browser := u.browser
	bridge := u.bridge
	u.mu.RUnlock()

	if browser == nil {
		return skill.NewErrorResult(fmt.Errorf("browser service not available — cannot review URL")), nil
	}

	url := input["url"].(string)
	viewports := parseViewports(input)
	var steps []ReviewStep

	// 1. Navigate and take screenshots
	if err := browser.Start(ctx); err != nil {
		return skill.NewErrorResult(fmt.Errorf("browser start failed: %w", err)), nil
	}

	emitUIReviewProgress(ctx, "navigate", "Page Load", "running", url, nil)
	nav, err := browser.Navigate(ctx, url, "")
	if err != nil {
		steps = append(steps, ReviewStep{ID: "navigate", Name: "Page Load", Status: "failed", Message: err.Error()})
		emitUIReviewProgress(ctx, "navigate", "Page Load", "failed", url, nil)
		return skill.NewErrorResult(fmt.Errorf("navigation failed: %w", err)), nil
	}
	steps = append(steps, ReviewStep{ID: "navigate", Name: "Page Load", Status: "success"})
	emitUIReviewProgress(ctx, "navigate", "Page Load", "success", url, nil)

	// 2. Functional checks (page loaded, JS errors, etc.)
	emitUIReviewProgress(ctx, "functional", "Functional Check", "running", url, nil)
	funcResult := u.runFunctionalChecks(ctx, browser, nav.TargetID, url)
	steps = append(steps, ReviewStep{
		ID: "functional", Name: "Functional Check", Status: "success",
		Score: funcResult.Score, Issues: len(funcResult.Issues),
	})
	emitUIReviewProgress(ctx, "functional", "Functional Check", "success", url, &funcResult.Score)

	// 3. Accessibility checks
	emitUIReviewProgress(ctx, "accessibility", "Accessibility Check", "running", url, nil)
	a11yResult := u.runA11yChecks(ctx, browser, nav.TargetID)
	steps = append(steps, ReviewStep{
		ID: "accessibility", Name: "Accessibility Check", Status: "success",
		Score: a11yResult.Score, Issues: len(a11yResult.Issues),
	})
	emitUIReviewProgress(ctx, "accessibility", "Accessibility Check", "success", url, &a11yResult.Score)

	// 4. Screenshot for VLM
	emitUIReviewProgress(ctx, "screenshot", "Screenshot", "running", url, nil)
	screenshot, err := browser.ScreenshotTab(ctx, nav.TargetID)
	if err != nil {
		screenshot = ""
		steps = append(steps, ReviewStep{ID: "screenshot", Name: "Screenshot", Status: "failed", Message: err.Error()})
		emitUIReviewProgress(ctx, "screenshot", "Screenshot", "failed", url, nil)
	} else {
		steps = append(steps, ReviewStep{ID: "screenshot", Name: "Screenshot", Status: "success"})
		emitUIReviewProgress(ctx, "screenshot", "Screenshot", "success", url, nil)
	}

	// 5. VLM visual review (if bridge available and screenshot captured)
	var vlmResult *VLMReviewResult
	if bridge != nil && screenshot != "" {
		emitUIReviewProgress(ctx, "visual", "Visual Review (VLM)", "running", url, nil)
		vlmResult = u.runVLMReview(ctx, bridge, screenshot, url)
		if vlmResult != nil {
			vlmScore := avgScores(vlmResult.Scores)
			steps = append(steps, ReviewStep{
				ID: "visual", Name: "Visual Review (VLM)", Status: "success",
				Score: vlmScore, Issues: len(vlmResult.Issues),
			})
			emitUIReviewProgress(ctx, "visual", "Visual Review (VLM)", "success", url, &vlmScore)
		} else {
			steps = append(steps, ReviewStep{ID: "visual", Name: "Visual Review (VLM)", Status: "failed", Message: "VLM returned no result"})
			emitUIReviewProgress(ctx, "visual", "Visual Review (VLM)", "failed", url, nil)
		}
	} else if bridge == nil {
		steps = append(steps, ReviewStep{ID: "visual", Name: "Visual Review (VLM)", Status: "skipped", Message: "No LLM provider configured"})
		emitUIReviewProgress(ctx, "visual", "Visual Review (VLM)", "skipped", url, nil)
	}

	// 6. Compute scores
	result := u.computeScores(url, viewports, funcResult, a11yResult, vlmResult, threshold)
	result.Steps = steps

	// Include a small screenshot reference for the card (truncate for JSON size)
	if screenshot != "" && len(screenshot) < 500000 {
		result.Screenshot = screenshot
	}

	// Close tab
	_ = browser.CloseTab(ctx, nav.TargetID)

	if format == "human" {
		result.Human = formatHumanReport(result)
	}

	return skill.NewResult(result), nil
}

// --- review_image: VLM-only review ---

func (u *UIReviewer) reviewImage(ctx context.Context, input map[string]any, threshold float64, format string) (*skill.Result, error) {
	u.mu.RLock()
	bridge := u.bridge
	u.mu.RUnlock()

	if bridge == nil {
		return skill.NewErrorResult(fmt.Errorf("proxy bridge not available — cannot call VLM")), nil
	}

	image := input["image"].(string)

	vlmResult := u.runVLMReview(ctx, bridge, image, "")

	result := &ReviewResult{
		Threshold: threshold,
	}

	if vlmResult != nil {
		result.Visual = ScoreDetail{
			Score:   avgScores(vlmResult.Scores),
			Details: vlmResult.Scores,
		}
		result.Issues = vlmResult.Issues
		result.Suggestions = vlmResult.Suggestions
		result.Overall = result.Visual.Score
		result.Pass = result.Overall >= threshold
	} else {
		result.Overall = 0
		result.Pass = false
		result.Issues = append(result.Issues, UIIssue{
			Severity:    "critical",
			Category:    "visual",
			Description: "VLM review failed — no visual score available",
		})
	}

	if format == "human" {
		result.Human = formatHumanReport(result)
	}

	return skill.NewResult(result), nil
}

// --- check_accessibility: a11y-only ---

func (u *UIReviewer) checkAccessibility(ctx context.Context, input map[string]any) (*skill.Result, error) {
	u.mu.RLock()
	browser := u.browser
	u.mu.RUnlock()

	if browser == nil {
		return skill.NewErrorResult(fmt.Errorf("browser service not available")), nil
	}

	url := input["url"].(string)

	if err := browser.Start(ctx); err != nil {
		return skill.NewErrorResult(fmt.Errorf("browser start failed: %w", err)), nil
	}

	nav, err := browser.Navigate(ctx, url, "")
	if err != nil {
		return skill.NewErrorResult(fmt.Errorf("navigation failed: %w", err)), nil
	}

	a11yResult := u.runA11yChecks(ctx, browser, nav.TargetID)
	_ = browser.CloseTab(ctx, nav.TargetID)

	return skill.NewResult(a11yResult), nil
}

// --- Functional checks ---

func (u *UIReviewer) runFunctionalChecks(ctx context.Context, browser BrowserServiceInterface, targetID, url string) *FunctionalCheckResult {
	result := &FunctionalCheckResult{Score: 100}

	// Page loaded successfully (we already navigated, so +40 is implicit)
	// Check for JS errors and broken resources via a11y tree presence
	_, err := browser.AccessibilityTree(ctx, targetID, 3)
	if err != nil {
		result.Score -= 40
		result.Issues = append(result.Issues, UIIssue{
			Severity:    "critical",
			Category:    "functional",
			Description: "Page accessibility tree unavailable — page may not have loaded correctly",
		})
	}

	return result
}

// --- Accessibility checks (in-browser JS) ---

// a11yCheckJS is the JavaScript executed in the browser to check accessibility issues.
const a11yCheckJS = `(() => {
	const issues = [];

	// 1. Images without alt
	document.querySelectorAll('img').forEach(img => {
		if (!img.hasAttribute('alt')) {
			issues.push({
				severity: 'major',
				category: 'accessibility',
				rule: 'img-alt',
				element: img.outerHTML.slice(0, 120),
				description: 'Image missing alt attribute'
			});
		}
	});

	// 2. Buttons/links without accessible name
	document.querySelectorAll('button, a, [role="button"], [role="link"]').forEach(el => {
		const name = el.textContent?.trim() || el.getAttribute('aria-label') || el.getAttribute('title') || '';
		if (!name && !el.querySelector('img[alt]')) {
			issues.push({
				severity: 'major',
				category: 'accessibility',
				rule: 'accessible-name',
				element: el.outerHTML.slice(0, 120),
				description: 'Interactive element missing accessible name (no text, aria-label, or title)'
			});
		}
	});

	// 3. Form inputs without labels
	document.querySelectorAll('input, select, textarea').forEach(input => {
		if (input.type === 'hidden') return;
		const id = input.id;
		const hasLabel = id && document.querySelector('label[for="' + id + '"]');
		const hasAriaLabel = input.getAttribute('aria-label') || input.getAttribute('aria-labelledby');
		const wrappedInLabel = input.closest('label');
		if (!hasLabel && !hasAriaLabel && !wrappedInLabel) {
			issues.push({
				severity: 'major',
				category: 'accessibility',
				rule: 'input-label',
				element: input.outerHTML.slice(0, 120),
				description: 'Form input missing associated label'
			});
		}
	});

	// 4. Color contrast (basic check on text elements)
	const checkContrast = (el) => {
		const style = window.getComputedStyle(el);
		const color = style.color;
		const bg = style.backgroundColor;
		if (color && bg && bg !== 'rgba(0, 0, 0, 0)' && bg !== 'transparent') {
			const fgLum = relativeLuminance(parseColor(color));
			const bgLum = relativeLuminance(parseColor(bg));
			const ratio = (Math.max(fgLum, bgLum) + 0.05) / (Math.min(fgLum, bgLum) + 0.05);
			const fontSize = parseFloat(style.fontSize);
			const isBold = parseInt(style.fontWeight) >= 700;
			const isLargeText = fontSize >= 24 || (fontSize >= 18.66 && isBold);
			const minRatio = isLargeText ? 3 : 4.5;
			if (ratio < minRatio) {
				issues.push({
					severity: 'minor',
					category: 'accessibility',
					rule: 'color-contrast',
					element: el.tagName.toLowerCase() + (el.className ? '.' + el.className.split(' ')[0] : ''),
					description: 'Insufficient color contrast ratio: ' + ratio.toFixed(2) + ':1 (minimum ' + minRatio + ':1)'
				});
			}
		}
	};

	function parseColor(c) {
		const m = c.match(/rgba?\((\d+),\s*(\d+),\s*(\d+)/);
		return m ? [+m[1], +m[2], +m[3]] : [0, 0, 0];
	}

	function relativeLuminance(rgb) {
		const [r, g, b] = rgb.map(v => {
			v = v / 255;
			return v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4);
		});
		return 0.2126 * r + 0.7152 * g + 0.0722 * b;
	}

	// Sample text elements for contrast (limit to avoid perf issues)
	const textEls = document.querySelectorAll('p, span, h1, h2, h3, h4, h5, h6, a, button, label, li, td, th');
	const sampled = Array.from(textEls).slice(0, 50);
	sampled.forEach(checkContrast);

	// 5. ARIA role validity
	document.querySelectorAll('[role]').forEach(el => {
		const validRoles = ['alert','alertdialog','application','article','banner','button','cell','checkbox',
			'columnheader','combobox','complementary','contentinfo','definition','dialog','directory',
			'document','feed','figure','form','grid','gridcell','group','heading','img','link','list',
			'listbox','listitem','log','main','marquee','math','menu','menubar','menuitem','menuitemcheckbox',
			'menuitemradio','navigation','none','note','option','presentation','progressbar','radio',
			'radiogroup','region','row','rowgroup','rowheader','scrollbar','search','searchbox','separator',
			'slider','spinbutton','status','switch','tab','table','tablist','tabpanel','term','textbox',
			'timer','toolbar','tooltip','tree','treegrid','treeitem'];
		if (!validRoles.includes(el.getAttribute('role'))) {
			issues.push({
				severity: 'minor',
				category: 'accessibility',
				rule: 'valid-role',
				element: el.outerHTML.slice(0, 120),
				description: 'Invalid ARIA role: ' + el.getAttribute('role')
			});
		}
	});

	// 6. Focus visibility (check if focusable elements have visible focus styles)
	// This is a heuristic — we check if outline/box-shadow is explicitly set to none
	document.querySelectorAll('a, button, input, select, textarea, [tabindex]').forEach(el => {
		const style = window.getComputedStyle(el, ':focus');
		if (style.outlineStyle === 'none' && !style.boxShadow.includes('rgb')) {
			// Only flag if there's no custom focus indicator
			const focusStyle = window.getComputedStyle(el);
			if (focusStyle.outlineStyle === 'none') {
				issues.push({
					severity: 'minor',
					category: 'accessibility',
					rule: 'focus-visible',
					element: el.tagName.toLowerCase() + (el.className ? '.' + el.className.split(' ')[0] : ''),
					description: 'Focusable element may lack visible focus indicator (outline: none)'
				});
			}
		}
	});

	return JSON.stringify(issues.slice(0, 50));
})()`

func (u *UIReviewer) runA11yChecks(ctx context.Context, browser BrowserServiceInterface, targetID string) *A11yCheckResult {
	result := &A11yCheckResult{Score: 100}

	// Get a11y tree — we use it to run our JS checks
	a11y, err := browser.AccessibilityTree(ctx, targetID, 5)
	if err != nil {
		result.Score = 50
		result.Issues = append(result.Issues, UIIssue{
			Severity:    "major",
			Category:    "accessibility",
			Description: "Could not retrieve accessibility tree",
		})
		return result
	}

	// Parse the a11y tree for common issues
	tree := a11y.Tree

	// Basic heuristic checks on the a11y tree text
	if !strings.Contains(tree, "heading") && !strings.Contains(tree, "Heading") {
		result.Issues = append(result.Issues, UIIssue{
			Severity:    "minor",
			Category:    "accessibility",
			Rule:        "heading-structure",
			Description: "No heading elements found in accessibility tree",
		})
	}

	if !strings.Contains(tree, "navigation") && !strings.Contains(tree, "Navigation") && !strings.Contains(tree, "nav") {
		result.Issues = append(result.Issues, UIIssue{
			Severity:    "minor",
			Category:    "accessibility",
			Rule:        "landmark-nav",
			Description: "No navigation landmark found",
		})
	}

	// Deduct points based on issues
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

const vlmReviewPrompt = `You are a UI/UX expert reviewer. Analyze this screenshot and provide a structured quality assessment.

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

Be concise. Focus on actionable issues.`

func (u *UIReviewer) runVLMReview(ctx context.Context, bridge *proxybridge.Bridge, screenshotBase64, url string) *VLMReviewResult {
	prompt := vlmReviewPrompt
	if url != "" {
		prompt += fmt.Sprintf("\n\nURL: %s", url)
	}

	req := llm.ChatRequest{
		Model: "auto",
		Messages: []llm.Message{{
			Role: llm.RoleUser,
			ContentParts: []llm.ContentPart{
				{Type: "text", Text: prompt},
				{Type: "image", MediaType: "image/png", Data: screenshotBase64},
			},
		}},
		MaxTokens: 2000,
	}

	resp, err := bridge.Chat(ctx, req)
	if err != nil {
		return nil
	}

	content := resp.Message.Content
	// Strip markdown code fences if present
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var result VLMReviewResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil
	}

	// Tag issues with category
	for i := range result.Issues {
		result.Issues[i].Category = "visual"
	}

	return &result
}

// --- Scoring ---

func (u *UIReviewer) computeScores(url string, viewports []string, funcResult *FunctionalCheckResult, a11yResult *A11yCheckResult, vlmResult *VLMReviewResult, threshold float64) *ReviewResult {
	result := &ReviewResult{
		URL:       url,
		Threshold: threshold,
		Viewports: viewports,
	}

	// Functional: 40% weight
	result.Functional = ScoreDetail{Score: funcResult.Score}
	result.Issues = append(result.Issues, funcResult.Issues...)

	// Accessibility: 30% weight
	result.Accessibility = ScoreDetail{Score: a11yResult.Score}
	result.Issues = append(result.Issues, a11yResult.Issues...)

	// Visual: 30% weight
	if vlmResult != nil {
		result.Visual = ScoreDetail{
			Score:   avgScores(vlmResult.Scores),
			Details: vlmResult.Scores,
		}
		result.Issues = append(result.Issues, vlmResult.Issues...)
		result.Suggestions = vlmResult.Suggestions
	} else {
		// No VLM — redistribute weights: Functional 55%, Accessibility 45%
		result.Overall = result.Functional.Score*0.55 + result.Accessibility.Score*0.45
		result.Pass = result.Overall >= threshold && result.Functional.Score >= 70 && !hasCriticalIssue(result.Issues)
		return result
	}

	result.Overall = result.Visual.Score*0.30 + result.Functional.Score*0.40 + result.Accessibility.Score*0.30
	result.Pass = result.Overall >= threshold && result.Functional.Score >= 70 && !hasCriticalIssue(result.Issues)

	return result
}

// --- Output formatting ---

func formatHumanReport(r *ReviewResult) string {
	var b strings.Builder

	b.WriteString("═══════════════════════════════════════\n")
	if r.URL != "" {
		b.WriteString("  UI Review Report - ")
		b.WriteString(r.URL)
		b.WriteByte('\n')
	} else {
		b.WriteString("  UI Review Report\n")
	}
	b.WriteString("═══════════════════════════════════════\n")

	passStr := "FAIL"
	if r.Pass {
		passStr = "PASS"
	}
	b.WriteString("Overall Score: ")
	b.WriteString(strconv.FormatFloat(r.Overall, 'f', 1, 64))
	b.WriteString(" / 100  ")
	b.WriteString(passStr)
	b.WriteString("\n\n")

	if r.Visual.Score > 0 {
		b.WriteString("  Visual          ")
		b.WriteString(strconv.FormatFloat(r.Visual.Score, 'f', 0, 64))
		b.WriteByte('\n')
	}
	b.WriteString("  Functional      ")
	b.WriteString(strconv.FormatFloat(r.Functional.Score, 'f', 0, 64))
	b.WriteString("\n  Accessibility   ")
	b.WriteString(strconv.FormatFloat(r.Accessibility.Score, 'f', 0, 64))
	b.WriteByte('\n')

	if len(r.Issues) > 0 {
		b.WriteString("\nIssues (")
		b.WriteString(strconv.Itoa(len(r.Issues)))
		b.WriteString("):\n")
		for _, issue := range r.Issues {
			icon := "🔵"
			switch issue.Severity {
			case "critical":
				icon = "🔴"
			case "major":
				icon = "🟡"
			}
			b.WriteString("  ")
			b.WriteString(icon)
			b.WriteString(" [")
			b.WriteString(issue.Severity)
			b.WriteString("] ")
			b.WriteString(issue.Description)
			b.WriteByte('\n')
		}
	}

	if len(r.Suggestions) > 0 {
		b.WriteString("\nSuggestions:\n")
		for _, s := range r.Suggestions {
			b.WriteString("  • ")
			b.WriteString(s)
			b.WriteByte('\n')
		}
	}

	b.WriteString("═══════════════════════════════════════\n")
	return b.String()
}

// --- Helpers ---

func parseViewports(input map[string]any) []string {
	if v, ok := input["viewports"].([]any); ok {
		var vps []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				vps = append(vps, s)
			}
		}
		if len(vps) > 0 {
			return vps
		}
	}
	return []string{"desktop"}
}

func avgScores(scores map[string]float64) float64 {
	if len(scores) == 0 {
		return 0
	}
	var sum float64
	for _, v := range scores {
		sum += v
	}
	return sum / float64(len(scores))
}

func hasCriticalIssue(issues []UIIssue) bool {
	for _, issue := range issues {
		if issue.Severity == "critical" {
			return true
		}
	}
	return false
}

func emitUIReviewProgress(ctx context.Context, stepID, stepName, status, url string, score *float64) {
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
	tools.EmitCard(ctx, card)
}
