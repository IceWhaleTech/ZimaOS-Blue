package ppt

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mediagen"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/slidespec"
)

const (
	DefaultAspectRatio       = "16:9"
	DefaultStylePreset       = "banana_slides"
	DefaultQualityProfile    = "ppt"
	DefaultReviewThreshold   = 80.0
	DefaultReviewRetryBudget = 1
	defaultWaitTimeout       = 2 * time.Minute
	defaultResolution        = "2K"
)

type Mode string

const (
	ModeTextOnly  Mode = "text-only"
	ModeReference Mode = "reference"
)

type Request struct {
	Description       string
	AspectRatio       string
	ReferenceImages   []string
	LayoutSpec        any
	StylePreset       string
	Theme             string
	Source            string
	ReviewThreshold   float64
	ReviewRetryBudget int
	QualityProfile    string
	Lang              string
}

type Result struct {
	Status         string        `json:"status,omitempty"`
	Skipped        bool          `json:"skipped,omitempty"`
	SkipReason     string        `json:"skip_reason,omitempty"`
	TaskID         string        `json:"task_id,omitempty"`
	Model          string        `json:"model,omitempty"`
	Mode           string        `json:"mode,omitempty"`
	FinalPrompt    string        `json:"final_prompt,omitempty"`
	ReviewScore    float64       `json:"review_score,omitempty"`
	ReviewSummary  string        `json:"review_summary,omitempty"`
	RetryCount     int           `json:"retry_count,omitempty"`
	ImageURLs      []string      `json:"image_urls,omitempty"`
	ThumbnailURLs  []string      `json:"thumbnail_urls,omitempty"`
	Review         *ReviewResult `json:"review,omitempty"`
	UsedFallback   bool          `json:"used_fallback,omitempty"`
	QualityProfile string        `json:"quality_profile,omitempty"`
	StylePreset    string        `json:"style_preset,omitempty"`
	Source         string        `json:"source,omitempty"`
	Error          string        `json:"error,omitempty"`
	Threshold      float64       `json:"threshold,omitempty"`
	Description    string        `json:"description,omitempty"`
	ReferenceCount int           `json:"reference_count,omitempty"`
}

type ReviewOptions struct {
	Threshold float64
	Lang      string
	Profile   string
}

type ReviewIssue struct {
	Severity    string `json:"severity,omitempty"`
	Category    string `json:"category,omitempty"`
	Rule        string `json:"rule,omitempty"`
	Element     string `json:"element,omitempty"`
	Description string `json:"description,omitempty"`
	Location    string `json:"location,omitempty"`
}

type ReviewResult struct {
	Overall     float64            `json:"overall,omitempty"`
	Threshold   float64            `json:"threshold,omitempty"`
	Pass        bool               `json:"pass,omitempty"`
	Scores      map[string]float64 `json:"scores,omitempty"`
	Issues      []ReviewIssue      `json:"issues,omitempty"`
	Suggestions []string           `json:"suggestions,omitempty"`
	Human       string             `json:"human,omitempty"`
}

type Generator interface {
	HasImageProviders() bool
	Models() []mediagen.MediaModelInfo
	CreateTask(ctx context.Context, req *mediagen.MediaRequest, messageID, category, source string) (*mediagen.MediaTask, error)
	WaitForTask(ctx context.Context, taskID string) (*mediagen.MediaTask, error)
}

type StorageReader interface {
	ReadServedURL(servedURL string) ([]byte, error)
}

type Reviewer interface {
	ReviewImage(ctx context.Context, imageBase64 string, opts ReviewOptions) (*ReviewResult, error)
}

type Service struct {
	generator   Generator
	storage     StorageReader
	reviewer    Reviewer
	layoutPlan  LayoutPlanner
	waitTimeout time.Duration
}

func NewService(generator Generator, storage StorageReader, reviewer Reviewer) *Service {
	return &Service{
		generator:   generator,
		storage:     storage,
		reviewer:    reviewer,
		waitTimeout: defaultWaitTimeout,
	}
}

func (s *Service) SetLayoutPlanner(planner LayoutPlanner) {
	if s == nil {
		return
	}
	s.layoutPlan = planner
}

func (s *Service) Generate(ctx context.Context, req Request) (*Result, error) {
	prepared := normalizeRequest(req)
	result := &Result{
		Status:         "pending",
		QualityProfile: prepared.QualityProfile,
		StylePreset:    prepared.StylePreset,
		Source:         prepared.Source,
		Threshold:      prepared.ReviewThreshold,
		Description:    prepared.Description,
		ReferenceCount: len(prepared.ReferenceImages),
	}

	if s == nil || s.generator == nil {
		result.Status = "skipped"
		result.Skipped = true
		result.SkipReason = "ppt slide-asset service is not configured"
		return result, nil
	}
	if !s.generator.HasImageProviders() {
		result.Status = "skipped"
		result.Skipped = true
		result.SkipReason = "no active image provider"
		return result, nil
	}

	selection, ok := chooseModel(s.generator.Models(), prepared.ReferenceImages)
	if !ok {
		result.Status = "skipped"
		result.Skipped = true
		result.SkipReason = "no compatible image model"
		return result, nil
	}
	result.Mode = string(selection.Mode)
	result.Model = selection.Model
	result.UsedFallback = selection.UsedFallback

	baseTask, basePrompt, _, err := s.generateOnce(ctx, prepared, selection, "", 0)
	if err != nil {
		result.Status = "failed"
		result.Error = err.Error()
		return result, err
	}
	result.TaskID = baseTask.ID
	result.FinalPrompt = basePrompt
	applyTaskOutputs(result, baseTask)

	baseReview, reviewSummary, reviewErr := s.reviewTask(ctx, baseTask, prepared)
	if baseReview != nil {
		result.Review = baseReview
		result.ReviewScore = baseReview.Overall
	}
	if reviewSummary != "" {
		result.ReviewSummary = reviewSummary
	}
	if reviewErr != nil {
		result.Status = "succeeded"
		result.Error = reviewErr.Error()
		return result, nil
	}

	if !needsRetry(baseReview, prepared.ReviewThreshold, prepared.ReviewRetryBudget) {
		result.Status = "succeeded"
		return result, nil
	}

	repairDelta := buildRepairDelta(baseReview)
	if repairDelta == "" {
		result.Status = "succeeded"
		return result, nil
	}

	retryTask, retryPrompt, _, retryErr := s.generateOnce(ctx, prepared, selection, repairDelta, 1)
	if retryErr != nil {
		result.Status = "succeeded"
		result.Error = retryErr.Error()
		return result, nil
	}
	result.RetryCount = 1
	result.TaskID = retryTask.ID
	result.FinalPrompt = retryPrompt
	applyTaskOutputs(result, retryTask)

	retryReview, retrySummary, retryReviewErr := s.reviewTask(ctx, retryTask, prepared)
	if retryReview != nil {
		result.Review = retryReview
		result.ReviewScore = retryReview.Overall
	}
	if retrySummary != "" {
		result.ReviewSummary = retrySummary
	}
	if retryReviewErr != nil {
		result.Status = "succeeded"
		result.Error = retryReviewErr.Error()
		return result, nil
	}

	result.Status = "succeeded"
	return result, nil
}

type modelSelection struct {
	Model        string
	Mode         Mode
	UsedFallback bool
}

func normalizeRequest(req Request) Request {
	req.Description = strings.TrimSpace(req.Description)
	req.AspectRatio = strings.TrimSpace(req.AspectRatio)
	if req.AspectRatio == "" {
		req.AspectRatio = DefaultAspectRatio
	}
	req.StylePreset = slidespec.NormalizeStylePreset(req.StylePreset)
	if req.StylePreset == "" {
		req.StylePreset = DefaultStylePreset
	}
	req.QualityProfile = strings.TrimSpace(req.QualityProfile)
	if req.QualityProfile == "" {
		req.QualityProfile = DefaultQualityProfile
	}
	req.Source = strings.TrimSpace(req.Source)
	if req.Source == "" {
		req.Source = "ppt"
	}
	req.Theme = strings.TrimSpace(req.Theme)
	req.Lang = strings.TrimSpace(req.Lang)
	if req.ReviewThreshold <= 0 {
		req.ReviewThreshold = DefaultReviewThreshold
	}
	if req.ReviewRetryBudget < 0 {
		req.ReviewRetryBudget = 0
	}
	if req.ReviewRetryBudget == 0 {
		req.ReviewRetryBudget = DefaultReviewRetryBudget
	}
	if len(req.ReferenceImages) > 0 {
		cleaned := make([]string, 0, len(req.ReferenceImages))
		seen := map[string]struct{}{}
		for _, item := range req.ReferenceImages {
			trimmed := strings.TrimSpace(item)
			if trimmed == "" {
				continue
			}
			if _, ok := seen[trimmed]; ok {
				continue
			}
			seen[trimmed] = struct{}{}
			cleaned = append(cleaned, trimmed)
		}
		req.ReferenceImages = cleaned
	}
	return req
}

func chooseModel(models []mediagen.MediaModelInfo, referenceImages []string) (modelSelection, bool) {
	if model := chooseReferenceModel(models, referenceImages); model != "" {
		return modelSelection{Model: model, Mode: ModeReference, UsedFallback: model != "nano-banana-pro"}, true
	}
	model := chooseTextModel(models)
	if model == "" {
		return modelSelection{}, false
	}
	return modelSelection{Model: model, Mode: ModeTextOnly, UsedFallback: model != "nano-banana-pro"}, true
}

func chooseReferenceModel(models []mediagen.MediaModelInfo, referenceImages []string) string {
	if len(referenceImages) == 0 {
		return ""
	}
	preferred := []string{"nano-banana-pro", "qwen-image-edit-max", "wan2.6-image", "wan2.5-i2i-preview"}
	if model := firstPreferredModel(models, preferred, func(m mediagen.MediaModelInfo) bool {
		return m.Type == mediagen.MediaTypeImage && m.Category == mediagen.CategoryI2I
	}); model != "" {
		return model
	}
	for _, model := range models {
		if model.Type == mediagen.MediaTypeImage && model.Category == mediagen.CategoryI2I {
			return model.ID
		}
	}
	return ""
}

func chooseTextModel(models []mediagen.MediaModelInfo) string {
	preferred := []string{"nano-banana-pro", "qwen-image-max", "midjourney", "gemini-2.0-flash-exp-image-generation", "imagen-3.0-generate-002", "wanx2.1-t2i-turbo", "wanx-v1"}
	if model := firstPreferredModel(models, preferred, func(m mediagen.MediaModelInfo) bool {
		return m.Type == mediagen.MediaTypeImage && (m.Category == mediagen.CategoryT2I || m.Category == "")
	}); model != "" {
		return model
	}
	for _, model := range models {
		if model.Type == mediagen.MediaTypeImage && (model.Category == mediagen.CategoryT2I || model.Category == "") {
			return model.ID
		}
	}
	return ""
}

func firstPreferredModel(models []mediagen.MediaModelInfo, preferred []string, allow func(mediagen.MediaModelInfo) bool) string {
	for _, wanted := range preferred {
		for _, model := range models {
			if model.ID == wanted && allow(model) {
				return model.ID
			}
		}
	}
	return ""
}

func (s *Service) generateOnce(ctx context.Context, req Request, selection modelSelection, repairDelta string, attempt int) (*mediagen.MediaTask, string, *mediagen.MediaRequest, error) {
	brief := buildSlideBrief(req)
	allowVisual := slideAllowsVisualSlot(brief, req, selection)
	layoutSpec := s.resolveLayoutSpec(ctx, req, brief, allowVisual)
	promptBrief := briefWithLayoutSpec(brief, layoutSpec)
	prompt := buildPromptWithBrief(req, selection.Mode, repairDelta, promptBrief)
	mediaReq := &mediagen.MediaRequest{
		Type:           mediagen.MediaTypeImage,
		Prompt:         prompt,
		NegativePrompt: buildNegativePrompt(),
		Model:          selection.Model,
		N:              1,
		Extra: map[string]any{
			"aspect_ratio":        req.AspectRatio,
			"resolution":          defaultResolution,
			"style_preset":        promptBrief.StylePreset,
			"quality_profile":     req.QualityProfile,
			"ppt_attempt":         attempt,
			"ppt_description":     req.Description,
			"ppt_title":           brief.Title,
			"ppt_subtitle":        brief.Subtitle,
			"ppt_bullets":         append([]string(nil), brief.Bullets...),
			"ppt_visual_query":    brief.VisualQuery,
			"render_mode":         promptBrief.RenderMode,
			"template_id":         promptBrief.TemplateID,
			"layout_spec":         layoutSpec,
			"review_threshold":    req.ReviewThreshold,
			"review_retry_budget": req.ReviewRetryBudget,
			"source":              req.Source,
		},
	}
	category := string(mediagen.CategoryT2I)
	if selection.Mode == ModeReference && len(req.ReferenceImages) > 0 {
		mediaReq.ReferenceURL = req.ReferenceImages[0]
		mediaReq.ReferenceURLs = append([]string(nil), req.ReferenceImages...)
		category = string(mediagen.CategoryI2I)
	}
	if attempt > 0 {
		mediaReq.Extra["repair_prompt_delta"] = repairDelta
	}
	if promptBrief.Theme != "" {
		mediaReq.Extra["theme"] = promptBrief.Theme
	}
	if req.Lang != "" {
		mediaReq.Extra["lang"] = req.Lang
	}

	task, err := s.generator.CreateTask(ctx, mediaReq, "", category, req.Source)
	if err != nil {
		return nil, prompt, mediaReq, err
	}
	waitTimeout := s.waitTimeout
	if waitTimeout <= 0 {
		waitTimeout = defaultWaitTimeout
	}
	waitCtx, cancel := context.WithTimeout(ctx, waitTimeout)
	defer cancel()
	waited, waitErr := s.generator.WaitForTask(waitCtx, task.ID)
	if waited != nil {
		task = waited
	}
	if waitErr != nil {
		return task, prompt, mediaReq, waitErr
	}
	if task == nil {
		return nil, prompt, mediaReq, errors.New("ppt slide-asset generation returned no task")
	}
	return task, prompt, mediaReq, nil
}

func (s *Service) resolveLayoutSpec(ctx context.Context, req Request, brief slidespec.Brief, hasVisual bool) slidespec.LayoutSpec {
	canvasWidth, canvasHeight := slideCanvasDimensions(brief.AspectRatio)
	if explicit, ok := slidespec.DecodeLayoutSpec(req.LayoutSpec); ok {
		return slidespec.NormalizeLayoutSpec(explicit, brief, canvasWidth, canvasHeight, hasVisual)
	}
	if s != nil && s.layoutPlan != nil {
		planned, err := s.layoutPlan.Plan(ctx, LayoutPlanRequest{
			Description:    req.Description,
			Theme:          req.Theme,
			StylePreset:    req.StylePreset,
			AspectRatio:    brief.AspectRatio,
			Lang:           req.Lang,
			Width:          canvasWidth,
			Height:         canvasHeight,
			HasVisual:      hasVisual,
			ReferenceCount: len(req.ReferenceImages),
			Brief:          brief,
		})
		if err == nil && planned != nil {
			return slidespec.NormalizeLayoutSpec(*planned, brief, canvasWidth, canvasHeight, hasVisual)
		}
	}
	return slidespec.BuildLayoutSpec(brief, canvasWidth, canvasHeight, hasVisual)
}

func slideAllowsVisualSlot(brief slidespec.Brief, req Request, selection modelSelection) bool {
	if selection.Mode == ModeReference && len(req.ReferenceImages) > 0 {
		return true
	}
	if strings.TrimSpace(brief.VisualQuery) != "" {
		return true
	}
	return strings.TrimSpace(req.Description) != ""
}

func briefWithLayoutSpec(brief slidespec.Brief, layout slidespec.LayoutSpec) slidespec.Brief {
	if layout.RenderMode != "" {
		brief.RenderMode = layout.RenderMode
	}
	if layout.StylePreset != "" {
		brief.StylePreset = layout.StylePreset
	}
	if layout.TemplateID != "" {
		brief.TemplateID = layout.TemplateID
	}
	if layout.Theme != "" {
		brief.Theme = layout.Theme
	}
	if layout.Canvas.AspectRatio != "" {
		brief.AspectRatio = layout.Canvas.AspectRatio
	}
	return brief
}

func (s *Service) reviewTask(ctx context.Context, task *mediagen.MediaTask, req Request) (*ReviewResult, string, error) {
	if s.reviewer == nil {
		return nil, "review skipped: reviewer unavailable", nil
	}
	if s.storage == nil {
		return nil, "review skipped: storage unavailable", nil
	}
	if task == nil || task.Response == nil || len(task.Response.Data) == 0 {
		return nil, "review skipped: no generated image", nil
	}
	first := task.Response.Data[0]
	if first.URL == "" {
		return nil, "review skipped: generated image URL unavailable", nil
	}
	data, err := s.storage.ReadServedURL(first.URL)
	if err != nil {
		return nil, "review skipped: failed to read generated image", err
	}
	review, err := s.reviewer.ReviewImage(ctx, base64.StdEncoding.EncodeToString(data), ReviewOptions{
		Threshold: req.ReviewThreshold,
		Lang:      req.Lang,
		Profile:   req.QualityProfile,
	})
	if err != nil {
		return nil, "review failed", err
	}
	return review, summarizeReview(review), nil
}

func needsRetry(review *ReviewResult, threshold float64, budget int) bool {
	if review == nil || budget <= 0 {
		return false
	}
	return review.Overall < threshold
}

func applyTaskOutputs(result *Result, task *mediagen.MediaTask) {
	if result == nil {
		return
	}
	result.ImageURLs = result.ImageURLs[:0]
	result.ThumbnailURLs = result.ThumbnailURLs[:0]
	if task == nil || task.Response == nil {
		return
	}
	for _, item := range task.Response.Data {
		if item.URL != "" {
			result.ImageURLs = append(result.ImageURLs, item.URL)
		}
		if item.ThumbnailURL != "" {
			result.ThumbnailURLs = append(result.ThumbnailURLs, item.ThumbnailURL)
		}
	}
}

func buildPrompt(req Request, mode Mode, repairDelta string) string {
	return buildPromptWithBrief(req, mode, repairDelta, buildSlideBrief(req))
}

func buildSlideBrief(req Request) slidespec.Brief {
	return slidespec.Build(slidespec.Input{
		Prompt:         req.Description,
		Description:    req.Description,
		StylePreset:    req.StylePreset,
		QualityProfile: req.QualityProfile,
		Theme:          req.Theme,
		AspectRatio:    req.AspectRatio,
		Source:         req.Source,
		Lang:           req.Lang,
	})
}

func buildPromptWithBrief(req Request, mode Mode, repairDelta string, brief slidespec.Brief) string {
	sections := []string{
		"Create a presentation-ready PPT slide visual for this intent:",
		req.Description,
	}
	if req.Theme != "" {
		sections = append(sections, "Theme or brand context:", req.Theme)
	}
	sections = append(sections,
		"Structured slide brief:",
		fmt.Sprintf("- template_id: %s", brief.TemplateID),
		fmt.Sprintf("- title: %s", brief.Title),
	)
	if brief.Subtitle != "" {
		sections = append(sections, fmt.Sprintf("- subtitle: %s", brief.Subtitle))
	}
	if len(brief.Bullets) > 0 {
		sections = append(sections, fmt.Sprintf("- bullets: %s", strings.Join(brief.Bullets, " | ")))
	}
	if brief.VisualQuery != "" {
		sections = append(sections, fmt.Sprintf("- visual_query: %s", brief.VisualQuery))
	}
	sections = append(sections,
		slideStyleConstraintTitle(brief.StylePreset),
		fmt.Sprintf("- Design for a %s presentation canvas with strong composition and polished, presentation-ready aesthetics.", req.AspectRatio),
		"- Keep the image suitable for slide overlays: no body paragraphs, no captions, no UI chrome, no watermarks.",
		"- Reserve clean negative space for future title and content placement with safe contrast.",
		"- Use a cohesive palette, crisp edges, balanced layout, professional hierarchy, and high consistency across the frame.",
		"- Avoid visual clutter, distorted anatomy, messy details, awkward crops, noisy backgrounds, and low-contrast focal areas.",
		"Quality constraints:",
		"- visual_hierarchy: clear focal point and strong layout hierarchy.",
		"- layout_alignment: balanced spacing, stable alignment, presentation-safe margins.",
		"- color_harmony: cohesive palette and strong contrast in text-safe areas.",
		"- typography_or_text_safety: leave clean zones that can safely receive titles or bullets later.",
		"- professionalism: polished, production-ready, presentation-grade finish.",
	)
	sections = append(sections, slideStyleConstraintLines(brief.StylePreset)...)
	sections = append(sections, slideTemplateConstraintLines(brief.TemplateID)...)
	if mode == ModeReference {
		sections = append(sections,
			"Reference image guidance:",
			"- Use the provided reference images to borrow visual language, palette, texture, and compositional cues.",
			"- Preserve the user's content intent while matching reference style consistency.",
		)
	} else {
		sections = append(sections,
			"Text-only slide mode:",
			"- Infer a strong visual system from the content intent alone while keeping the output slide-friendly and structured.",
		)
	}
	if repairDelta != "" {
		sections = append(sections, "Repair directives:", repairDelta)
	}
	return strings.Join(sections, "\n")
}

func slideStyleConstraintTitle(stylePreset string) string {
	switch slidespec.StylePresetDisplayName(stylePreset) {
	case "nanoslides":
		return "nanoslides style constraints:"
	case "bananaslides":
		return "bananaslides style constraints:"
	default:
		return "Presentation style constraints:"
	}
}

func slideStyleConstraintLines(stylePreset string) []string {
	switch slidespec.NormalizeStylePreset(stylePreset) {
	case slidespec.StylePresetNanoSlides:
		return []string{
			"- nanoslides_signature: editorial headline scale, restrained luxury palette, crisp asymmetric composition, glass-like info cards.",
			"- mood: premium, boardroom-ready, intentional, minimal-noise, presentation-native.",
			"- avoid: playful marketing poster tropes, meme aesthetics, overly illustrative scenes, crowded decorative gradients.",
		}
	case slidespec.StylePresetBananaSlides:
		return []string{
			"- bananaslides_signature: bold contrast, polished gradients, confident focal point, clean presentation-safe framing.",
			"- mood: energetic but controlled, polished, modern, high-clarity.",
		}
	default:
		return nil
	}
}

func slideTemplateConstraintLines(templateID string) []string {
	switch strings.TrimSpace(templateID) {
	case slidespec.TemplateCover:
		return []string{
			"- template_motif: headline-led cover slide with a single hero visual and large clean title zone.",
		}
	case slidespec.TemplateSplit:
		return []string{
			"- template_motif: left text narrative plus right framed visual panel with stable card edges.",
		}
	case slidespec.TemplateTextOnly:
		return []string{
			"- template_motif: executive summary layout with concise insight cards instead of paragraph bullets.",
		}
	case slidespec.TemplateAgenda:
		return []string{
			"- template_motif: agenda page with a stable title band and sequential numbered cards for steps or sections.",
		}
	case slidespec.TemplateMetrics:
		return []string{
			"- template_motif: KPI page with oversized metric values and short labels in polished stat cards.",
		}
	default:
		return nil
	}
}

func buildNegativePrompt() string {
	return strings.Join([]string{
		"paragraph text",
		"body copy",
		"watermark",
		"logo",
		"UI chrome",
		"artifact",
		"distorted geometry",
		"messy composition",
		"clutter",
		"low contrast",
	}, ", ")
}

func buildRepairDelta(review *ReviewResult) string {
	if review == nil {
		return ""
	}
	parts := make([]string, 0, 4)
	issues := append([]ReviewIssue(nil), review.Issues...)
	sort.SliceStable(issues, func(i, j int) bool {
		return severityRank(issues[i].Severity) < severityRank(issues[j].Severity)
	})
	for _, issue := range issues {
		if strings.TrimSpace(issue.Description) == "" {
			continue
		}
		parts = append(parts, "- Fix: "+strings.TrimSpace(issue.Description))
		if len(parts) >= 3 {
			break
		}
	}
	for _, suggestion := range review.Suggestions {
		if strings.TrimSpace(suggestion) == "" {
			continue
		}
		parts = append(parts, "- Improve: "+strings.TrimSpace(suggestion))
		if len(parts) >= 5 {
			break
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n")
}

func summarizeReview(review *ReviewResult) string {
	if review == nil {
		return ""
	}
	parts := []string{fmt.Sprintf("score %.1f/100", review.Overall)}
	if len(review.Issues) > 0 {
		first := strings.TrimSpace(review.Issues[0].Description)
		if first != "" {
			parts = append(parts, "top issue: "+first)
		}
	}
	if len(review.Suggestions) > 0 {
		first := strings.TrimSpace(review.Suggestions[0])
		if first != "" {
			parts = append(parts, "top suggestion: "+first)
		}
	}
	return strings.Join(parts, "; ")
}

func severityRank(severity string) int {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical":
		return 0
	case "major":
		return 1
	case "minor":
		return 2
	default:
		return 3
	}
}
