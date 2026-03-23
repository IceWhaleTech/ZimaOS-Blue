package mediagen

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	stdDraw "image/draw"
	"image/png"
	"math"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/scenecompose"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/slidespec"
	basetask "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/task"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gobolditalic"
	"golang.org/x/image/font/gofont/goitalic"
	"golang.org/x/image/font/gofont/gomedium"
	"golang.org/x/image/font/gofont/gomediumitalic"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/gofont/gomonobolditalic"
	"golang.org/x/image/font/gofont/gomonoitalic"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/gofont/gosmallcaps"
	"golang.org/x/image/font/gofont/gosmallcapsitalic"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

type slideCanvasSpec struct {
	width  int
	height int
}

type slidePalette struct {
	Top    color.RGBA
	Bottom color.RGBA
	Panel  color.RGBA
	Accent color.RGBA
	Text   color.RGBA
	Muted  color.RGBA
}

type slideFontStyle struct {
	face   font.Face
	size   float64
	family string
	weight string
}

type slideFontPack struct {
	eyebrow  slideFontStyle
	title    slideFontStyle
	subtitle slideFontStyle
	body     slideFontStyle
	chip     slideFontStyle
}

func (fonts slideFontPack) styleForRole(role string) slideFontStyle {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "eyebrow":
		return fonts.eyebrow
	case "title":
		return fonts.title
	case "subtitle":
		return fonts.subtitle
	case "chip":
		return fonts.chip
	default:
		return fonts.body
	}
}

var (
	slideFontRegistryMu sync.Mutex
	slideFontRegistry   = map[string]*opentype.Font{}
	slideFontErrors     = map[string]error{}
)

func isSlideFallbackRequest(req *MediaRequest) bool {
	return slideBriefFromMediaRequest(req).RenderMode == slidespec.RenderModeSlide
}

func slideBriefFromMediaRequest(req *MediaRequest) slidespec.Brief {
	if req == nil {
		return slidespec.Build(slidespec.Input{})
	}
	description := firstNonEmptyValue(
		mediaRequestExtraString(req, "ppt_description", "slide_description", "description"),
		strings.TrimSpace(req.Prompt),
	)
	brief := slidespec.Build(slidespec.Input{
		Prompt:         description,
		Description:    description,
		StylePreset:    mediaRequestExtraString(req, "style_preset", "stylePreset"),
		QualityProfile: mediaRequestExtraString(req, "quality_profile", "qualityProfile"),
		Theme: mediaRequestExtraString(req,
			"theme",
			"style_theme",
			"styleTheme",
			"brand_guidance",
			"brandGuidance",
		),
		AspectRatio: mediaRequestExtraString(req, "aspect_ratio", "aspectRatio"),
		Source:      mediaRequestExtraString(req, "source", "origin"),
		Lang:        mediaRequestExtraString(req, "lang"),
	})
	if !hasExplicitSlideRequest(req) {
		brief.RenderMode = slidespec.RenderModePoster
		brief.TemplateID = slidespec.TemplatePoster
		brief.Title = ""
		brief.Subtitle = ""
		brief.Bullets = nil
		brief.VisualQuery = description
	}
	if title := mediaRequestExtraString(req, "ppt_title", "slide_title", "title"); title != "" {
		brief.RenderMode = slidespec.RenderModeSlide
		brief.Title = title
	}
	if subtitle := mediaRequestExtraString(req, "ppt_subtitle", "slide_subtitle", "subtitle"); subtitle != "" {
		brief.RenderMode = slidespec.RenderModeSlide
		brief.Subtitle = subtitle
	}
	if bullets := mediaRequestExtraStrings(req, "ppt_bullets", "slide_bullets", "bullets"); len(bullets) > 0 {
		brief.RenderMode = slidespec.RenderModeSlide
		brief.Bullets = append([]string(nil), bullets...)
	}
	if visualQuery := mediaRequestExtraString(req, "ppt_visual_query", "slide_visual_query", "visual_query"); visualQuery != "" {
		brief.RenderMode = slidespec.RenderModeSlide
		brief.VisualQuery = visualQuery
	}
	if layout, ok := mediaRequestLayoutSpec(req); ok {
		brief.RenderMode = slidespec.RenderModeSlide
		if strings.TrimSpace(layout.TemplateID) != "" {
			brief.TemplateID = strings.TrimSpace(layout.TemplateID)
		}
		if strings.TrimSpace(layout.StylePreset) != "" {
			brief.StylePreset = strings.TrimSpace(layout.StylePreset)
		}
		if strings.TrimSpace(layout.Theme) != "" {
			brief.Theme = strings.TrimSpace(layout.Theme)
		}
	}
	if renderMode := mediaRequestExtraString(req, "render_mode"); renderMode != "" {
		brief.RenderMode = renderMode
	}
	if templateID := mediaRequestExtraString(req, "template_id"); templateID != "" {
		brief.TemplateID = templateID
	}
	return brief
}

func hasExplicitSlideRequest(req *MediaRequest) bool {
	if req == nil {
		return false
	}
	if renderMode := mediaRequestExtraString(req, "render_mode"); strings.EqualFold(renderMode, slidespec.RenderModeSlide) {
		return true
	}
	if templateID := mediaRequestExtraString(req, "template_id"); templateID != "" {
		return true
	}
	if mediaRequestExtraString(req, "ppt_title", "slide_title", "title") != "" {
		return true
	}
	if mediaRequestExtraString(req, "ppt_subtitle", "slide_subtitle", "subtitle") != "" {
		return true
	}
	if len(mediaRequestExtraStrings(req, "ppt_bullets", "slide_bullets", "bullets")) > 0 {
		return true
	}
	if mediaRequestExtraString(req, "ppt_visual_query", "slide_visual_query", "visual_query") != "" {
		return true
	}
	if _, ok := mediaRequestLayoutSpec(req); ok {
		return true
	}
	if slidespec.NormalizeStylePreset(mediaRequestExtraString(req, "style_preset", "stylePreset")) != "" {
		return true
	}
	if strings.EqualFold(mediaRequestExtraString(req, "quality_profile", "qualityProfile"), "ppt") {
		return true
	}
	if containsExplicitSlidePromptSignal(mediaRequestExtraString(req, "source", "origin")) {
		return true
	}
	return containsExplicitSlidePromptSignal(strings.TrimSpace(req.Prompt))
}

func containsExplicitSlidePromptSignal(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	signals := []string{
		"ppt",
		"slide",
		"slides",
		"deck",
		"presentation",
		"幻灯片",
		"演示文稿",
		"演示稿",
		"路演",
		"汇报",
	}
	for _, signal := range signals {
		if strings.Contains(lower, strings.ToLower(signal)) {
			return true
		}
	}
	return false
}

func mediaRequestExtraString(req *MediaRequest, keys ...string) string {
	if req == nil || req.Extra == nil {
		return ""
	}
	for _, key := range keys {
		value, ok := req.Extra[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case string:
			if trimmed := strings.TrimSpace(typed); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

func mediaRequestExtraStrings(req *MediaRequest, keys ...string) []string {
	if req == nil || req.Extra == nil {
		return nil
	}
	for _, key := range keys {
		value, ok := req.Extra[key]
		if !ok || value == nil {
			continue
		}
		items := normalizeExtraStrings(value)
		if len(items) > 0 {
			return items
		}
	}
	return nil
}

func normalizeExtraStrings(value any) []string {
	switch typed := value.(type) {
	case []string:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if trimmed := strings.TrimSpace(item); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				if trimmed := strings.TrimSpace(text); trimmed != "" {
					out = append(out, trimmed)
				}
			}
		}
		return out
	default:
		return nil
	}
}

func (e *FallbackEngine) PendingInfoForRequest(req *MediaRequest, modelID string) *MediaFallbackInfo {
	info := e.PendingInfoForModel(modelID)
	if info == nil {
		return nil
	}
	brief := slideBriefFromMediaRequest(req)
	switch {
	case brief.RenderMode == slidespec.RenderModeSlide:
		info.RenderMode = slidespec.RenderModeSlide
		info.TemplateID = brief.TemplateID
		info.StylePreset = brief.StylePreset
	case modelID == FallbackModelWebCanvasT2I:
		info.RenderMode = slidespec.RenderModePoster
		info.TemplateID = slidespec.TemplatePoster
	}
	return info
}

func (e *FallbackEngine) newFallbackInfo(
	strategy string,
	displayName string,
	sourceURLs []string,
	sources []MediaFallbackSource,
	spaceURL string,
	brief slidespec.Brief,
	templateID string,
) *MediaFallbackInfo {
	info := &MediaFallbackInfo{
		Used:        true,
		Strategy:    strategy,
		DisplayName: displayName,
		Disclosure:  fallbackDisclosure(e.locale, strategy),
	}
	if len(sourceURLs) > 0 {
		info.SourceURLs = append([]string(nil), sourceURLs...)
	}
	if len(sources) > 0 {
		info.Sources = append([]MediaFallbackSource(nil), sources...)
	}
	if strings.TrimSpace(spaceURL) != "" {
		info.SpaceURL = strings.TrimSpace(spaceURL)
	}
	switch {
	case brief.RenderMode == slidespec.RenderModeSlide:
		info.RenderMode = slidespec.RenderModeSlide
		info.TemplateID = strings.TrimSpace(firstNonEmptyValue(templateID, brief.TemplateID))
		info.StylePreset = brief.StylePreset
	case strategy == FallbackStrategyWebCanvas:
		info.RenderMode = slidespec.RenderModePoster
		info.TemplateID = slidespec.TemplatePoster
	}
	return info
}

func (e *FallbackEngine) newReferenceImageFallbackInfo(
	strategy string,
	displayName string,
	sourceURLs []string,
	sources []MediaFallbackSource,
	brief slidespec.Brief,
) *MediaFallbackInfo {
	info := &MediaFallbackInfo{
		Used:        true,
		Strategy:    strategy,
		DisplayName: displayName,
		Disclosure:  fallbackDisclosure(e.locale, strategy),
	}
	if len(sourceURLs) > 0 {
		info.SourceURLs = append([]string(nil), sourceURLs...)
	}
	if len(sources) > 0 {
		info.Sources = append([]MediaFallbackSource(nil), sources...)
	}
	if brief.RenderMode == slidespec.RenderModeSlide {
		info.RenderMode = slidespec.RenderModeSlide
		info.TemplateID = strings.TrimSpace(brief.TemplateID)
		info.StylePreset = brief.StylePreset
	}
	return info
}

func (e *FallbackEngine) generateSlideWebCanvas(ctx context.Context, req *MediaRequest) (*MediaTask, error) {
	brief := slideBriefFromMediaRequest(req)
	canvas := slideCanvasSize(brief.AspectRatio, e.config.ScreenshotWidth)
	visualReq := cloneMediaRequest(req)
	if visualReq != nil && strings.TrimSpace(brief.VisualQuery) != "" {
		visualReq.Prompt = brief.VisualQuery
	}

	var visualDataURL string
	sourceURLs := make([]string, 0, 6)
	sourceRefs := make([]MediaFallbackSource, 0, 6)
	query := strings.TrimSpace(brief.VisualQuery)
	if query == "" {
		query = strings.TrimSpace(req.Prompt)
	}
	_, candidates, urls, sources := e.searchReferenceAssets(ctx, query, e.config.SearchProviderChain)
	sourceURLs = appendUniqueStrings(sourceURLs, urls...)
	sourceRefs = append(sourceRefs, sources...)
	if len(candidates) > 0 {
		visualDataURL = strings.TrimSpace(candidates[0].DataURL)
	}
	if visualDataURL == "" && e.sceneComposer != nil {
		composeCtx, cancel := fallbackContextWithTimeout(ctx, fallbackSceneComposeTimeout)
		data, composedURLs, err := e.renderSceneComposeSized(composeCtx, visualReq, canvas.width, canvas.height)
		cancel()
		if err == nil {
			visualDataURL = "data:image/png;base64," + data
			sourceURLs = appendUniqueStrings(sourceURLs, composedURLs...)
		}
	}

	templateID := strings.TrimSpace(brief.TemplateID)
	switch templateID {
	case slidespec.TemplateCover, slidespec.TemplateSplit:
		if visualDataURL == "" {
			templateID = slidespec.TemplateTextOnly
		}
	case slidespec.TemplateTextOnly:
	default:
		templateID = slidespec.DecideTemplate(brief, visualDataURL != "")
	}
	brief.TemplateID = templateID
	layoutSpec := slideLayoutSpecFromMediaRequest(req, brief, canvas, visualDataURL != "")
	data, err := renderSlidePNGWithLayout(layoutSpec, brief, visualDataURL, canvas)
	if err == nil {
		return e.inlineFallbackImageTask(req, data, canvas, sourceURLs, sourceRefs, brief, templateID), nil
	}

	if canCaptureSlide(e) {
		if htmlDoc, buildErr := e.buildSlideHTML(brief, templateID, visualDataURL, canvas); buildErr == nil {
			captureCtx, cancel := fallbackContextWithTimeout(ctx, fallbackBrowserCaptureTimeout)
			data, captureErr := e.capturePoster(captureCtx, htmlDoc)
			cancel()
			if captureErr == nil {
				return e.inlineFallbackImageTask(req, data, canvas, sourceURLs, sourceRefs, brief, templateID), nil
			}
		}
	}
	return nil, err
}

func canCaptureSlide(e *FallbackEngine) bool {
	return e != nil &&
		e.browser != nil &&
		e.browser() != nil &&
		strings.TrimSpace(e.config.RenderBaseURL) != ""
}

func (e *FallbackEngine) inlineFallbackImageTask(
	req *MediaRequest,
	data string,
	canvas slideCanvasSpec,
	sourceURLs []string,
	sources []MediaFallbackSource,
	brief slidespec.Brief,
	templateID string,
) *MediaTask {
	revisedPrompt := ""
	if req != nil {
		revisedPrompt = strings.TrimSpace(req.Prompt)
	}
	return &MediaTask{
		BaseTask: basetask.BaseTask{
			Status:   TaskStatusSucceeded,
			Progress: 1,
		},
		Type:     MediaTypeImage,
		Provider: fallbackProviderName,
		Model:    FallbackModelWebCanvasT2I,
		Response: &MediaResponse{
			Created: timeutil.NowTime().Unix(),
			Data: []MediaResult{
				{
					B64JSON:       data,
					ContentType:   "image/png",
					Width:         canvas.width,
					Height:        canvas.height,
					RevisedPrompt: revisedPrompt,
				},
			},
		},
		FallbackInfo: e.newFallbackInfo(
			FallbackStrategyWebCanvas,
			fallbackDisplayName(FallbackStrategyWebCanvas),
			sourceURLs,
			sources,
			"",
			brief,
			templateID,
		),
	}
}

func (e *FallbackEngine) buildSlideHTML(
	brief slidespec.Brief,
	templateID string,
	visualDataURL string,
	canvas slideCanvasSpec,
) (string, error) {
	eyebrowJSON, _ := json.Marshal(slideEyebrowText(brief, templateID))
	titleJSON, _ := json.Marshal(strings.TrimSpace(brief.Title))
	subtitleJSON, _ := json.Marshal(strings.TrimSpace(brief.Subtitle))
	bulletsJSON, _ := json.Marshal(append([]string(nil), brief.Bullets...))
	themeJSON, _ := json.Marshal(strings.TrimSpace(brief.Theme))
	visualJSON, _ := json.Marshal(strings.TrimSpace(visualDataURL))
	templateJSON, _ := json.Marshal(strings.TrimSpace(templateID))
	styleClass := slideStyleClass(brief.StylePreset)

	return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Fallback Slide Render</title>
<style>
html, body { margin:0; padding:0; width:100%%; height:100%%; background:#0b1220; overflow:hidden; }
body {
  display:flex;
  align-items:center;
  justify-content:center;
  font-family:"Avenir Next","Segoe UI","Helvetica Neue",Arial,sans-serif;
}
.slide {
  position:relative;
  width:%dpx;
  height:%dpx;
  overflow:hidden;
  color:#f8fafc;
  background:
    radial-gradient(circle at 10%% 16%%, rgba(125, 211, 252, 0.22), transparent 32%%),
    radial-gradient(circle at 85%% 10%%, rgba(250, 204, 21, 0.16), transparent 28%%),
    linear-gradient(140deg, #0f172a 0%%, #10213f 45%%, #07111f 100%%);
}
.slide::before {
  content:"";
  position:absolute;
  inset:0;
  background:
    linear-gradient(125deg, rgba(255,255,255,0.06) 0%%, rgba(255,255,255,0) 28%%),
    linear-gradient(90deg, rgba(255,255,255,0.04) 1px, transparent 1px),
    linear-gradient(rgba(255,255,255,0.03) 1px, transparent 1px);
  background-size:auto, 44px 44px, 44px 44px;
  opacity:0.42;
}
.slide.style-nano {
  background:
    radial-gradient(circle at 14%% 18%%, rgba(244, 196, 92, 0.12), transparent 30%%),
    radial-gradient(circle at 88%% 12%%, rgba(125, 211, 252, 0.10), transparent 26%%),
    linear-gradient(145deg, #0c1424 0%%, #10192d 42%%, #060b15 100%%);
}
.slide.style-nano::before {
  opacity:0.24;
}
.slide.style-nano::after {
  content:"";
  position:absolute;
  inset:24px;
  border-radius:32px;
  border:1px solid rgba(244, 196, 92, 0.18);
  box-shadow:inset 0 0 0 1px rgba(255,255,255,0.03);
  pointer-events:none;
}
.slide.style-nano .title {
  max-width:6.8em;
  letter-spacing:-0.048em;
}
.slide.style-nano .content {
  padding-left:30px;
}
.slide.style-nano .content::after {
  content:"";
  position:absolute;
  left:0;
  top:18px;
  bottom:18px;
  width:2px;
  border-radius:999px;
  background:linear-gradient(180deg, rgba(244,196,92,0.94) 0%%, rgba(244,196,92,0.10) 100%%);
}
.slide.style-nano .eyebrow {
  background:rgba(244, 196, 92, 0.08);
  border-color:rgba(244, 196, 92, 0.18);
  color:rgba(245, 232, 197, 0.92);
}
.slide.style-nano .eyebrow-dot {
  background:#f4c45c;
  box-shadow:0 0 22px rgba(244, 196, 92, 0.32);
}
.slide.style-nano .bullets li {
  background:rgba(255,255,255,0.045);
  border-color:rgba(255,255,255,0.08);
  box-shadow:0 22px 48px rgba(2, 6, 23, 0.22);
}
.slide.style-nano .bullets li::after {
  content:"";
  position:absolute;
  left:54px;
  right:22px;
  top:0;
  height:3px;
  border-radius:999px;
  background:linear-gradient(90deg, rgba(244,196,92,0.95) 0%%, rgba(244,196,92,0.14) 100%%);
}
.slide.style-nano.template-split .hero {
  border-radius:24px;
  box-shadow:0 30px 80px rgba(2, 6, 23, 0.44);
}
.slide.style-nano .hero::before {
  content:"";
  position:absolute;
  inset:12px;
  border-radius:22px;
  border:1px solid rgba(244, 196, 92, 0.18);
  z-index:1;
}
.slide.style-nano.template-text_only .content::before,
.slide.style-nano.no-visual .content::before {
  background:
    linear-gradient(135deg, rgba(255,255,255,0.08) 0%%, rgba(255,255,255,0.03) 100%%),
    radial-gradient(circle at 100%% 0%%, rgba(244,196,92,0.10), transparent 34%%);
}
.slide.style-nano.template-text_only .content,
.slide.style-nano.no-visual .content {
  column-gap:42px;
}
.slide.style-nano.template-text_only .bullets li,
.slide.style-nano.no-visual .bullets li {
  min-height:124px;
  border-radius:24px;
}
.slide.style-banana .bullets li {
  background:rgba(15,23,42,0.30);
}
.hero {
  position:absolute;
  right:0;
  top:0;
  bottom:0;
  width:55%%;
  background-position:center;
  background-size:cover;
  background-repeat:no-repeat;
  filter:saturate(1.06) contrast(1.03);
}
.hero::after {
  content:"";
  position:absolute;
  inset:0;
  background:linear-gradient(90deg, rgba(8,15,28,0.76) 0%%, rgba(8,15,28,0.14) 40%%, rgba(8,15,28,0.20) 100%%);
}
.content {
  position:absolute;
  inset:72px;
  display:flex;
  flex-direction:column;
  justify-content:center;
  max-width:44%%;
  z-index:2;
}
.eyebrow {
  display:inline-flex;
  align-items:center;
  gap:10px;
  align-self:flex-start;
  padding:10px 16px;
  border-radius:999px;
  background:rgba(255,255,255,0.10);
  border:1px solid rgba(255,255,255,0.12);
  color:rgba(226,232,240,0.88);
  font-size:14px;
  font-weight:600;
  letter-spacing:0.08em;
  margin-bottom:20px;
}
.eyebrow-dot {
  width:10px;
  height:10px;
  border-radius:999px;
  background:#facc15;
  box-shadow:0 0 18px rgba(250, 204, 21, 0.45);
}
.title {
  margin:0;
  font-size:66px;
  line-height:1.03;
  letter-spacing:-0.04em;
  max-width:7.6em;
}
.subtitle {
  margin:20px 0 0;
  font-size:27px;
  line-height:1.42;
  color:rgba(226,232,240,0.84);
  max-width:24em;
}
.bullets {
  margin:30px 0 0;
  padding:0;
  list-style:none;
  display:grid;
  gap:14px;
}
.bullets li {
  position:relative;
  padding:18px 18px 18px 54px;
  border-radius:22px;
  border:1px solid rgba(255,255,255,0.10);
  background:rgba(15,23,42,0.30);
  box-shadow:0 18px 40px rgba(2, 6, 23, 0.16);
  font-size:22px;
  line-height:1.35;
  color:rgba(248,250,252,0.94);
}
.bullets li::before {
  content:"";
  position:absolute;
  left:20px;
  top:22px;
  width:18px;
  height:18px;
  border-radius:12px;
  background:linear-gradient(180deg, #7dd3fc 0%%, #38bdf8 100%%);
  box-shadow:0 0 20px rgba(56, 189, 248, 0.30);
}
.theme-chip {
  display:none;
  align-self:flex-start;
  margin-top:18px;
  padding:10px 14px;
  border-radius:16px;
  background:rgba(125, 211, 252, 0.16);
  border:1px solid rgba(125, 211, 252, 0.22);
  color:#dbeafe;
  font-size:18px;
}
.slide.template-split .hero {
  top:58px;
  right:58px;
  bottom:58px;
  width:44%%;
  border-radius:28px;
  border:1px solid rgba(255,255,255,0.14);
  box-shadow:0 28px 70px rgba(2, 6, 23, 0.38);
}
.slide.template-split .hero::after {
  background:linear-gradient(180deg, rgba(8,15,28,0.08) 0%%, rgba(8,15,28,0.18) 100%%);
}
.slide.template-split .content {
  justify-content:center;
  max-width:42%%;
}
.slide.template-split .bullets li {
  background:rgba(255,255,255,0.08);
}
.slide.template-text_only .hero,
.slide.no-visual .hero {
  display:none;
}
.slide.template-text_only .content,
.slide.no-visual .content {
  max-width:none;
  inset:74px 88px;
  display:grid;
  grid-template-columns:minmax(0, 1.15fr) minmax(320px, 0.95fr);
  grid-template-areas:
    "eyebrow bullets"
    "title bullets"
    "subtitle bullets"
    "theme bullets";
  align-items:start;
  column-gap:34px;
}
.slide.template-text_only .content::before,
.slide.no-visual .content::before {
  content:"";
  position:absolute;
  inset:0;
  border-radius:34px;
  background:
    linear-gradient(135deg, rgba(255,255,255,0.10) 0%%, rgba(255,255,255,0.04) 100%%),
    radial-gradient(circle at 100%% 0%%, rgba(125,211,252,0.18), transparent 34%%);
  border:1px solid rgba(255,255,255,0.10);
  box-shadow:0 24px 70px rgba(2, 6, 23, 0.30);
}
.slide.template-text_only .content > *,
.slide.no-visual .content > * {
  position:relative;
  z-index:1;
}
.slide.template-text_only .eyebrow,
.slide.no-visual .eyebrow {
  grid-area:eyebrow;
  margin-bottom:14px;
}
.slide.template-text_only .title,
.slide.no-visual .title {
  grid-area:title;
  max-width:9.5em;
  margin-top:4px;
}
.slide.template-text_only .subtitle,
.slide.no-visual .subtitle {
  grid-area:subtitle;
  max-width:18em;
}
.slide.template-text_only .bullets,
.slide.no-visual .bullets {
  grid-area:bullets;
  margin:0;
  grid-template-columns:repeat(2, minmax(0, 1fr));
  align-self:stretch;
}
.slide.template-text_only .bullets li,
.slide.no-visual .bullets li {
  min-height:108px;
  background:rgba(15,23,42,0.34);
}
.slide.template-text_only .theme-chip,
.slide.no-visual .theme-chip {
  grid-area:theme;
}
#fallback-render-ready { position:fixed; left:-9999px; top:-9999px; }
</style>
</head>
<body>
  <main id="slide" class="slide %s">
    <div id="hero" class="hero"></div>
    <section class="content">
      <div class="eyebrow"><span class="eyebrow-dot"></span><span id="eyebrow"></span></div>
      <h1 id="title" class="title"></h1>
      <p id="subtitle" class="subtitle"></p>
      <ul id="bullets" class="bullets"></ul>
      <div id="theme" class="theme-chip"></div>
    </section>
  </main>
  <div id="fallback-render-ready"></div>
<script>
const slide = {
  eyebrow: %s,
  title: %s,
  subtitle: %s,
  bullets: %s,
  theme: %s,
  visual: %s,
  template: %s
};
const root = document.getElementById('slide');
const hero = document.getElementById('hero');
const eyebrowEl = document.getElementById('eyebrow');
const titleEl = document.getElementById('title');
const subtitleEl = document.getElementById('subtitle');
const bulletsEl = document.getElementById('bullets');
const themeEl = document.getElementById('theme');

root.classList.add('template-' + (slide.template || 'cover'));
eyebrowEl.textContent = slide.eyebrow || 'Presentation Overview';
if (!slide.visual) {
  root.classList.add('no-visual');
} else {
  hero.style.backgroundImage = 'url(' + slide.visual + ')';
}
titleEl.textContent = slide.title || 'Presentation overview';
subtitleEl.textContent = slide.subtitle || '';
if (!slide.subtitle) {
  subtitleEl.style.display = 'none';
}
if (!Array.isArray(slide.bullets) || slide.bullets.length === 0) {
  bulletsEl.style.display = 'none';
} else {
  slide.bullets.forEach((item) => {
    const li = document.createElement('li');
    li.textContent = item;
    bulletsEl.appendChild(li);
  });
}
if (slide.theme) {
  themeEl.textContent = slide.theme;
  themeEl.style.display = 'inline-flex';
}
document.getElementById('fallback-render-ready').className = 'ready';
</script>
</body>
</html>`, canvas.width, canvas.height, styleClass, string(eyebrowJSON), string(titleJSON), string(subtitleJSON), string(bulletsJSON), string(themeJSON), string(visualJSON), string(templateJSON)), nil
}

func (e *FallbackEngine) renderSlidePNG(
	brief slidespec.Brief,
	templateID string,
	visualDataURL string,
	canvas slideCanvasSpec,
) (string, error) {
	brief.TemplateID = templateID
	layout := slidespec.BuildLayoutSpec(brief, canvas.width, canvas.height, visualDataURL != "")
	return renderSlidePNGWithLayout(layout, brief, visualDataURL, canvas)
}

func renderSlideCoverPNG(
	img *image.RGBA,
	brief slidespec.Brief,
	palette slidePalette,
	visualImg image.Image,
	canvas slideCanvasSpec,
	fonts slideFontPack,
) {
	heroRect := image.Rect(int(float64(canvas.width)*0.44), 52, canvas.width-52, canvas.height-52)
	drawSlideVisualPanel(img, heroRect, visualImg, palette, brief)

	contentRect := image.Rect(92, 92, int(float64(canvas.width)*0.39), canvas.height-92)
	nextY := drawSlideTextColumn(img, contentRect, brief, slidespec.TemplateCover, palette, fonts, 3)

	if len(brief.Bullets) > 0 {
		cardsRect := image.Rect(contentRect.Min.X, minInt(nextY+24, contentRect.Max.Y-168), contentRect.Max.X, contentRect.Max.Y)
		drawSlideBulletCards(img, cardsRect, brief.Bullets[:minInt(len(brief.Bullets), 2)], palette, fonts, brief.StylePreset)
		return
	}

	note := trimToASCIIWithFallback(firstNonEmptyValue(brief.VisualQuery, brief.Subtitle), "")
	if note != "" {
		drawSlideWrappedText(img, fonts.body.face, image.Rect(contentRect.Min.X, minInt(nextY+20, contentRect.Max.Y-120), contentRect.Max.X, contentRect.Max.Y), note, palette.Muted, 4)
	}
}

func renderSlideSplitPNG(
	img *image.RGBA,
	brief slidespec.Brief,
	palette slidePalette,
	visualImg image.Image,
	canvas slideCanvasSpec,
	fonts slideFontPack,
) {
	contentRect := image.Rect(84, 84, int(float64(canvas.width)*0.46), canvas.height-84)
	panelRect := image.Rect(int(float64(canvas.width)*0.54), 56, canvas.width-56, canvas.height-56)

	drawSlideVisualPanel(img, panelRect, visualImg, palette, brief)
	nextY := drawSlideTextColumn(img, contentRect, brief, slidespec.TemplateSplit, palette, fonts, 3)

	if len(brief.Bullets) > 0 {
		cardsRect := image.Rect(contentRect.Min.X, minInt(nextY+22, contentRect.Max.Y-196), contentRect.Max.X, contentRect.Max.Y)
		drawSlideBulletCards(img, cardsRect, brief.Bullets, palette, fonts, brief.StylePreset)
		return
	}

	note := trimToASCIIWithFallback(firstNonEmptyValue(brief.VisualQuery, brief.Subtitle), "")
	if note != "" {
		drawSlideWrappedText(img, fonts.body.face, image.Rect(contentRect.Min.X, minInt(nextY+20, contentRect.Max.Y-120), contentRect.Max.X, contentRect.Max.Y), note, palette.Muted, 4)
	}
}

func renderSlideTextOnlyPNG(
	img *image.RGBA,
	brief slidespec.Brief,
	palette slidePalette,
	canvas slideCanvasSpec,
	fonts slideFontPack,
) {
	boardRect := image.Rect(56, 52, canvas.width-56, canvas.height-52)
	overlayRect(img, boardRect, color.RGBA{R: 255, G: 255, B: 255, A: 18})
	strokeRect(img, boardRect, color.RGBA{R: 255, G: 255, B: 255, A: 24}, 1)
	if slidespec.IsNanoSlidesPreset(brief.StylePreset) {
		strokeRect(img, insetRect(boardRect, 16), color.RGBA{R: palette.Accent.R, G: palette.Accent.G, B: palette.Accent.B, A: 34}, 1)
	}

	leftRect := image.Rect(boardRect.Min.X+40, boardRect.Min.Y+38, boardRect.Min.X+int(float64(boardRect.Dx())*0.43), boardRect.Max.Y-40)
	rightRect := image.Rect(boardRect.Min.X+int(float64(boardRect.Dx())*0.50), boardRect.Min.Y+34, boardRect.Max.X-34, boardRect.Max.Y-34)
	if slidespec.IsNanoSlidesPreset(brief.StylePreset) {
		overlayRect(img, image.Rect(leftRect.Min.X-18, leftRect.Min.Y, leftRect.Min.X-8, leftRect.Max.Y), color.RGBA{
			R: palette.Accent.R, G: palette.Accent.G, B: palette.Accent.B, A: 112,
		})
	}

	nextY := drawSlideTextColumn(img, leftRect, brief, slidespec.TemplateTextOnly, palette, fonts, 4)
	if len(brief.Bullets) > 0 {
		drawSlideBulletCards(img, rightRect, brief.Bullets, palette, fonts, brief.StylePreset)
		return
	}

	note := trimToASCIIWithFallback(firstNonEmptyValue(brief.VisualQuery, brief.Subtitle), "Structured presentation-ready summary.")
	drawSlideWrappedText(img, fonts.body.face, image.Rect(rightRect.Min.X+12, minInt(nextY, rightRect.Min.Y+110), rightRect.Max.X-12, rightRect.Max.Y), note, palette.Muted, 6)
}

func drawSlideBackdrop(
	img *image.RGBA,
	canvas slideCanvasSpec,
	palette slidePalette,
	brief slidespec.Brief,
	templateID string,
) {
	overlayRect(img, image.Rect(0, 0, canvas.width, maxInt(8, canvas.height/90)), color.RGBA{
		R: palette.Accent.R, G: palette.Accent.G, B: palette.Accent.B, A: 176,
	})
	frame := image.Rect(34, 34, canvas.width-34, canvas.height-34)
	overlayRect(img, frame, color.RGBA{R: palette.Panel.R, G: palette.Panel.G, B: palette.Panel.B, A: 24})
	strokeRect(img, frame, color.RGBA{R: 255, G: 255, B: 255, A: 18}, 1)

	if slidespec.IsNanoSlidesPreset(brief.StylePreset) {
		strokeRect(img, insetRect(frame, 14), color.RGBA{R: palette.Accent.R, G: palette.Accent.G, B: palette.Accent.B, A: 40}, 1)
		overlayRect(img, image.Rect(76, 72, 84, canvas.height-72), color.RGBA{
			R: palette.Accent.R, G: palette.Accent.G, B: palette.Accent.B, A: 88,
		})
		overlayRect(img, image.Rect(canvas.width-248, 72, canvas.width-88, 80), color.RGBA{
			R: palette.Accent.R, G: palette.Accent.G, B: palette.Accent.B, A: 42,
		})
		return
	}

	if templateID == slidespec.TemplateCover {
		overlayRect(img, image.Rect(canvas.width-228, 64, canvas.width-72, 86), color.RGBA{
			R: palette.Accent.R, G: palette.Accent.G, B: palette.Accent.B, A: 46,
		})
	}
}

func drawSlideVisualPanel(
	img *image.RGBA,
	rect image.Rectangle,
	visualImg image.Image,
	palette slidePalette,
	brief slidespec.Brief,
) {
	overlayRect(img, rect, color.RGBA{R: 255, G: 255, B: 255, A: 18})
	strokeRect(img, rect, color.RGBA{R: 255, G: 255, B: 255, A: 28}, 1)
	inner := insetRect(rect, 10)

	if visualImg != nil {
		drawImageCoverRect(img, visualImg, inner)
		overlayRect(img, inner, color.RGBA{R: 8, G: 15, B: 28, A: 68})
	} else {
		drawSlideAbstractPanel(img, inner, palette, brief.StylePreset)
	}

	if slidespec.IsNanoSlidesPreset(brief.StylePreset) {
		strokeRect(img, insetRect(rect, 18), color.RGBA{R: palette.Accent.R, G: palette.Accent.G, B: palette.Accent.B, A: 42}, 1)
	}
}

func drawSlideAbstractPanel(img *image.RGBA, rect image.Rectangle, palette slidePalette, stylePreset string) {
	fillRect(img, rect, color.RGBA{R: palette.Panel.R, G: palette.Panel.G, B: palette.Panel.B, A: 255})
	overlayRect(img, image.Rect(rect.Min.X+20, rect.Min.Y+22, rect.Max.X-20, rect.Min.Y+40), color.RGBA{
		R: palette.Accent.R, G: palette.Accent.G, B: palette.Accent.B, A: 86,
	})
	overlayRect(img, image.Rect(rect.Min.X+20, rect.Min.Y+62, rect.Max.X-72, rect.Min.Y+80), color.RGBA{
		R: 255, G: 255, B: 255, A: 26,
	})
	overlayRect(img, image.Rect(rect.Min.X+20, rect.Min.Y+96, rect.Min.X+int(float64(rect.Dx())*0.58), rect.Min.Y+112), color.RGBA{
		R: 255, G: 255, B: 255, A: 22,
	})
	chartRect := image.Rect(rect.Min.X+28, rect.Max.Y-148, rect.Max.X-28, rect.Max.Y-28)
	overlayRect(img, chartRect, color.RGBA{R: 255, G: 255, B: 255, A: 14})
	strokeRect(img, chartRect, color.RGBA{R: 255, G: 255, B: 255, A: 20}, 1)

	barCount := 4
	barGap := 18
	barWidth := maxInt(20, (chartRect.Dx()-barGap*(barCount+1))/barCount)
	for idx := 0; idx < barCount; idx++ {
		heightFactor := 0.28 + float64(idx)*0.16
		if slidespec.IsNanoSlidesPreset(stylePreset) {
			heightFactor = 0.35 + float64(idx)*0.12
		}
		barHeight := int(float64(chartRect.Dy()-36) * minFloat(0.88, heightFactor))
		bar := image.Rect(
			chartRect.Min.X+barGap+idx*(barWidth+barGap),
			chartRect.Max.Y-18-barHeight,
			chartRect.Min.X+barGap+idx*(barWidth+barGap)+barWidth,
			chartRect.Max.Y-18,
		)
		fillRect(img, bar, palette.Accent)
	}
}

func drawSlideTextColumn(
	img *image.RGBA,
	rect image.Rectangle,
	brief slidespec.Brief,
	templateID string,
	palette slidePalette,
	fonts slideFontPack,
	maxTitleLines int,
) int {
	y := rect.Min.Y
	eyebrow := trimToASCIIWithFallback(slideEyebrowText(brief, templateID), "PRESENTATION")
	y += drawSlideEyebrowChip(img, image.Rect(rect.Min.X, y, rect.Max.X, y+42), eyebrow, palette, fonts.eyebrow.face, brief.StylePreset)
	y += 18
	y += drawSlideWrappedText(
		img,
		fonts.title.face,
		image.Rect(rect.Min.X, y, rect.Max.X, minInt(rect.Max.Y, y+maxInt(220, rect.Dy()/2))),
		trimToASCIIWithFallback(brief.Title, slideDefaultTitle(brief, templateID)),
		palette.Text,
		maxTitleLines,
	)

	subtitle := trimToASCIIWithFallback(brief.Subtitle, "")
	if subtitle != "" {
		y += 14
		y += drawSlideWrappedText(
			img,
			fonts.subtitle.face,
			image.Rect(rect.Min.X, y, rect.Max.X, minInt(rect.Max.Y, y+144)),
			subtitle,
			palette.Muted,
			3,
		)
	}

	themeLabel := trimToASCIIWithFallback(brief.Theme, "")
	if themeLabel != "" && !strings.EqualFold(strings.TrimSpace(themeLabel), strings.TrimSpace(eyebrow)) {
		y += 18
		y += drawSlideThemeChip(img, image.Rect(rect.Min.X, y, rect.Max.X, y+38), themeLabel, palette, fonts.chip.face)
	}
	return y
}

func drawSlideBulletCards(
	img *image.RGBA,
	rect image.Rectangle,
	bullets []string,
	palette slidePalette,
	fonts slideFontPack,
	stylePreset string,
) {
	if len(bullets) == 0 || rect.Empty() {
		return
	}
	items := append([]string(nil), bullets...)
	if len(items) > 4 {
		items = items[:4]
	}

	cols := 1
	if len(items) > 1 && rect.Dx() >= 420 {
		cols = 2
	}
	rows := (len(items) + cols - 1) / cols
	gap := 18
	cardW := (rect.Dx() - gap*(cols-1)) / cols
	cardH := (rect.Dy() - gap*(rows-1)) / rows
	cardH = maxInt(cardH, 118)

	for idx, bullet := range items {
		row := idx / cols
		col := idx % cols
		x := rect.Min.X + col*(cardW+gap)
		y := rect.Min.Y + row*(cardH+gap)
		card := image.Rect(x, y, minInt(rect.Max.X, x+cardW), minInt(rect.Max.Y, y+cardH))
		overlayRect(img, card, color.RGBA{R: 255, G: 255, B: 255, A: 18})
		strokeRect(img, card, color.RGBA{R: 255, G: 255, B: 255, A: 26}, 1)
		overlayRect(img, image.Rect(card.Min.X, card.Min.Y, card.Max.X, minInt(card.Max.Y, card.Min.Y+6)), color.RGBA{
			R: palette.Accent.R, G: palette.Accent.G, B: palette.Accent.B, A: 188,
		})

		labelColor := palette.Muted
		if slidespec.IsNanoSlidesPreset(stylePreset) {
			labelColor = palette.Accent
		}
		drawSlideTextLine(img, fonts.eyebrow.face, card.Min.X+16, card.Min.Y+18, fmt.Sprintf("%02d", idx+1), labelColor)
		drawSlideWrappedText(
			img,
			fonts.body.face,
			image.Rect(card.Min.X+16, card.Min.Y+38, card.Max.X-16, card.Max.Y-16),
			trimToASCIIWithFallback(bullet, ""),
			palette.Text,
			3,
		)
	}
}

func drawSlideEyebrowChip(
	img *image.RGBA,
	rect image.Rectangle,
	label string,
	palette slidePalette,
	face font.Face,
	stylePreset string,
) int {
	label = trimToASCIIWithFallback(strings.ToUpper(label), "PRESENTATION")
	if label == "" {
		return 0
	}
	height := maxInt(34, slideLineHeight(face)+12)
	width := minInt(rect.Max.X, rect.Min.X+measureSlideText(face, label)+46)
	chip := image.Rect(rect.Min.X, rect.Min.Y, width, rect.Min.Y+height)

	background := color.RGBA{R: 255, G: 255, B: 255, A: 18}
	border := color.RGBA{R: 255, G: 255, B: 255, A: 28}
	textColor := palette.Muted
	if slidespec.IsNanoSlidesPreset(stylePreset) {
		background = color.RGBA{R: palette.Accent.R, G: palette.Accent.G, B: palette.Accent.B, A: 20}
		border = color.RGBA{R: palette.Accent.R, G: palette.Accent.G, B: palette.Accent.B, A: 54}
		textColor = color.RGBA{R: 245, G: 232, B: 197, A: 255}
	}

	overlayRect(img, chip, background)
	strokeRect(img, chip, border, 1)
	fillCircle(img, chip.Min.X+14, chip.Min.Y+height/2, 4, palette.Accent)
	drawSlideTextLine(img, face, chip.Min.X+26, chip.Min.Y+maxInt(7, (height-slideLineHeight(face))/2), label, textColor)
	return chip.Dy()
}

func drawSlideThemeChip(
	img *image.RGBA,
	rect image.Rectangle,
	label string,
	palette slidePalette,
	face font.Face,
) int {
	label = trimToASCIIWithFallback(label, "")
	if label == "" {
		return 0
	}
	height := maxInt(32, slideLineHeight(face)+10)
	width := minInt(rect.Max.X, rect.Min.X+measureSlideText(face, label)+34)
	chip := image.Rect(rect.Min.X, rect.Min.Y, width, rect.Min.Y+height)
	overlayRect(img, chip, color.RGBA{R: palette.Accent.R, G: palette.Accent.G, B: palette.Accent.B, A: 24})
	strokeRect(img, chip, color.RGBA{R: palette.Accent.R, G: palette.Accent.G, B: palette.Accent.B, A: 34}, 1)
	drawSlideTextLine(img, face, chip.Min.X+16, chip.Min.Y+maxInt(6, (height-slideLineHeight(face))/2), label, palette.Text)
	return chip.Dy()
}

func newSlideFontPack(canvas slideCanvasSpec, templateID string) slideFontPack {
	scale := float64(canvas.width) / 1280
	scale = minFloat(1.35, maxFloat(0.85, scale))

	titleSize := 44 * scale
	if templateID == slidespec.TemplateTextOnly {
		titleSize = 48 * scale
	}
	return slideFontPack{
		eyebrow:  newSlideFontStyle(14*scale, "display", "medium"),
		title:    newSlideFontStyle(titleSize, "sans", "bold"),
		subtitle: newSlideFontStyle(22*scale, "sans", "regular"),
		body:     newSlideFontStyle(18*scale, "sans", "regular"),
		chip:     newSlideFontStyle(16*scale, "sans", "medium"),
	}
}

func (fonts slideFontPack) close() {
	closeSlideFace(fonts.eyebrow.face)
	closeSlideFace(fonts.title.face)
	closeSlideFace(fonts.subtitle.face)
	closeSlideFace(fonts.body.face)
	closeSlideFace(fonts.chip.face)
}

func newSlideFontStyle(size float64, family, weight string) slideFontStyle {
	return slideFontStyle{
		face:   newSlideFontFaceVariant(size, family, weight),
		size:   size,
		family: normalizeSlideFontFamily(family),
		weight: normalizeSlideFontWeight(weight),
	}
}

func newSlideFontFace(size float64) font.Face {
	return newSlideFontFaceVariant(size, "sans", "regular")
}

func newSlideFontFaceVariant(size float64, family, weight string) font.Face {
	parsed, err := loadSlideFontVariant(family, weight)
	if err != nil || parsed == nil || size <= 0 {
		return basicfont.Face7x13
	}
	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return basicfont.Face7x13
	}
	return face
}

func loadSlideFontVariant(family, weight string) (*opentype.Font, error) {
	key := slideFontVariantKey(family, weight)

	slideFontRegistryMu.Lock()
	parsed, ok := slideFontRegistry[key]
	err, hasErr := slideFontErrors[key]
	slideFontRegistryMu.Unlock()
	if ok || hasErr {
		return parsed, err
	}

	parsed, err = opentype.Parse(slideFontTTFBytes(family, weight))

	slideFontRegistryMu.Lock()
	slideFontRegistry[key] = parsed
	slideFontErrors[key] = err
	slideFontRegistryMu.Unlock()

	return parsed, err
}

func slideFontVariantKey(family, weight string) string {
	return normalizeSlideFontFamily(family) + ":" + normalizeSlideFontWeight(weight)
}

func slideFontTTFBytes(family, weight string) []byte {
	switch normalizeSlideFontFamily(family) {
	case "mono":
		switch normalizeSlideFontWeight(weight) {
		case "bold":
			return gomonobold.TTF
		case "italic":
			return gomonoitalic.TTF
		case "bold_italic":
			return gomonobolditalic.TTF
		default:
			return gomono.TTF
		}
	case "display":
		switch normalizeSlideFontWeight(weight) {
		case "italic", "bold_italic":
			return gosmallcapsitalic.TTF
		default:
			return gosmallcaps.TTF
		}
	default:
		switch normalizeSlideFontWeight(weight) {
		case "medium":
			return gomedium.TTF
		case "bold":
			return gobold.TTF
		case "italic":
			return goitalic.TTF
		case "medium_italic":
			return gomediumitalic.TTF
		case "bold_italic":
			return gobolditalic.TTF
		default:
			return goregular.TTF
		}
	}
}

func normalizeSlideFontFamily(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "default", "sans", "sans-serif", "sans_serif", "system", "ui", "text":
		return "sans"
	case "mono", "monospace", "code":
		return "mono"
	case "display", "smallcaps", "caps", "eyebrow":
		return "display"
	default:
		lower := strings.ToLower(strings.TrimSpace(value))
		switch {
		case strings.Contains(lower, "mono"), strings.Contains(lower, "code"):
			return "mono"
		case strings.Contains(lower, "display"), strings.Contains(lower, "smallcaps"), strings.Contains(lower, "caps"):
			return "display"
		default:
			return "sans"
		}
	}
}

func normalizeSlideFontWeight(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "normal", "regular", "book":
		return "regular"
	case "medium", "500":
		return "medium"
	case "semibold", "semi-bold", "semi_bold", "600":
		return "medium"
	case "bold", "700", "800", "900", "black", "heavy":
		return "bold"
	case "italic":
		return "italic"
	case "medium_italic", "mediumitalic", "semiitalic", "semibolditalic":
		return "medium_italic"
	case "bold_italic", "bolditalic", "italic_bold":
		return "bold_italic"
	default:
		lower := strings.ToLower(strings.TrimSpace(value))
		switch {
		case strings.Contains(lower, "bold") && strings.Contains(lower, "italic"):
			return "bold_italic"
		case strings.Contains(lower, "medium") && strings.Contains(lower, "italic"):
			return "medium_italic"
		case strings.Contains(lower, "italic"):
			return "italic"
		case strings.Contains(lower, "bold"), strings.Contains(lower, "black"), strings.Contains(lower, "heavy"):
			return "bold"
		case strings.Contains(lower, "medium"), strings.Contains(lower, "semi"):
			return "medium"
		default:
			return "regular"
		}
	}
}

func closeSlideFace(face font.Face) {
	type faceCloser interface {
		Close() error
	}
	if closer, ok := face.(faceCloser); ok {
		_ = closer.Close()
	}
}

func drawSlideWrappedText(
	img *image.RGBA,
	face font.Face,
	rect image.Rectangle,
	text string,
	c color.Color,
	maxLines int,
) int {
	lines := wrapSlideText(face, text, rect.Dx(), maxLines)
	if len(lines) == 0 {
		return 0
	}
	lineHeight := slideLineHeightWithMultiplier(face, 0)
	y := rect.Min.Y
	drawn := 0
	for _, line := range lines {
		if y+lineHeight > rect.Max.Y {
			break
		}
		drawSlideTextLine(img, face, rect.Min.X, y, line, c)
		y += lineHeight
		drawn++
	}
	return drawn * lineHeight
}

func drawSlideTextLine(img *image.RGBA, face font.Face, x, y int, text string, c color.Color) {
	if img == nil || face == nil || strings.TrimSpace(text) == "" {
		return
	}
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(c),
		Face: face,
		Dot:  fixed.P(x, y+face.Metrics().Ascent.Ceil()),
	}
	d.DrawString(text)
}

func wrapSlideText(face font.Face, text string, maxWidth int, maxLines int) []string {
	text = trimToASCIIWithFallback(text, "")
	if text == "" || face == nil || maxWidth <= 0 {
		return nil
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	lines := make([]string, 0, 4)
	line := ""
	truncated := false

	appendLine := func(value string) bool {
		value = strings.TrimSpace(value)
		if value == "" {
			return false
		}
		lines = append(lines, value)
		if maxLines > 0 && len(lines) >= maxLines {
			truncated = true
			return true
		}
		return false
	}

	for _, word := range words {
		candidate := word
		if line != "" {
			candidate = line + " " + word
		}
		if measureSlideText(face, candidate) <= maxWidth {
			line = candidate
			continue
		}
		if line != "" {
			if appendLine(line) {
				break
			}
			line = ""
		}
		for measureSlideText(face, word) > maxWidth {
			prefix, rest := splitSlideTextToWidth(face, word, maxWidth)
			if prefix == "" {
				break
			}
			if appendLine(prefix) {
				word = ""
				break
			}
			word = strings.TrimSpace(rest)
			if word == "" {
				break
			}
		}
		if word == "" {
			break
		}
		line = word
	}

	if line != "" && (!truncated || maxLines <= 0 || len(lines) < maxLines) {
		lines = append(lines, line)
	}
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[:maxLines]
		truncated = true
	}
	if truncated && len(lines) > 0 {
		last := strings.TrimRight(lines[len(lines)-1], ". ")
		for last != "" && measureSlideText(face, last+"...") > maxWidth {
			last = strings.TrimSpace(last[:len(last)-1])
		}
		lines[len(lines)-1] = strings.TrimSpace(last + "...")
	}
	return lines
}

func splitSlideTextToWidth(face font.Face, text string, maxWidth int) (string, string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", ""
	}
	for idx := len(text); idx > 0; idx-- {
		part := strings.TrimSpace(text[:idx])
		if part == "" {
			continue
		}
		if measureSlideText(face, part) <= maxWidth {
			return part, strings.TrimSpace(text[idx:])
		}
	}
	return text, ""
}

func measureSlideText(face font.Face, text string) int {
	if face == nil || strings.TrimSpace(text) == "" {
		return 0
	}
	d := &font.Drawer{Face: face}
	return d.MeasureString(text).Ceil()
}

func slideLineHeight(face font.Face) int {
	return slideLineHeightWithMultiplier(face, 0)
}

func slideLineHeightWithMultiplier(face font.Face, multiplier float64) int {
	if face == nil {
		return 18
	}
	height := face.Metrics().Height.Ceil()
	if height <= 0 {
		return 18
	}
	lineHeight := height + maxInt(4, height/6)
	if multiplier > 0 {
		lineHeight = int(math.Round(float64(height) * multiplier))
	}
	return maxInt(lineHeight, height)
}

func overlayRect(img *image.RGBA, rect image.Rectangle, c color.Color) {
	if img == nil {
		return
	}
	rect = rect.Intersect(img.Bounds())
	if rect.Empty() {
		return
	}
	stdDraw.Draw(img, rect, &image.Uniform{C: c}, image.Point{}, stdDraw.Over)
}

func overlayRoundedRect(img *image.RGBA, rect image.Rectangle, radius int, c color.Color) {
	if img == nil {
		return
	}
	rect = rect.Intersect(img.Bounds())
	if rect.Empty() {
		return
	}
	if radius <= 0 {
		overlayRect(img, rect, c)
		return
	}
	mask := roundedRectMask(rect, radius)
	stdDraw.DrawMask(img, rect, &image.Uniform{C: c}, image.Point{}, mask, rect.Min, stdDraw.Over)
}

func strokeRect(img *image.RGBA, rect image.Rectangle, c color.Color, width int) {
	if img == nil || width <= 0 || rect.Empty() {
		return
	}
	overlayRect(img, image.Rect(rect.Min.X, rect.Min.Y, rect.Max.X, minInt(rect.Max.Y, rect.Min.Y+width)), c)
	overlayRect(img, image.Rect(rect.Min.X, maxInt(rect.Min.Y, rect.Max.Y-width), rect.Max.X, rect.Max.Y), c)
	overlayRect(img, image.Rect(rect.Min.X, rect.Min.Y, minInt(rect.Max.X, rect.Min.X+width), rect.Max.Y), c)
	overlayRect(img, image.Rect(maxInt(rect.Min.X, rect.Max.X-width), rect.Min.Y, rect.Max.X, rect.Max.Y), c)
}

func strokeRoundedRect(img *image.RGBA, rect image.Rectangle, c color.Color, width int, radius int) {
	if img == nil || width <= 0 {
		return
	}
	rect = rect.Intersect(img.Bounds())
	if rect.Empty() {
		return
	}
	if radius <= 0 {
		strokeRect(img, rect, c, width)
		return
	}
	mask := roundedRectBorderMask(rect, radius, width)
	stdDraw.DrawMask(img, rect, &image.Uniform{C: c}, image.Point{}, mask, rect.Min, stdDraw.Over)
}

func roundedRectMask(rect image.Rectangle, radius int) *image.Alpha {
	mask := image.NewAlpha(rect)
	rect = rect.Intersect(mask.Bounds())
	if rect.Empty() {
		return mask
	}
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if pointInRoundedRect(x, y, rect, radius) {
				mask.SetAlpha(x, y, color.Alpha{A: 255})
			}
		}
	}
	return mask
}

func roundedRectBorderMask(rect image.Rectangle, radius, width int) *image.Alpha {
	mask := image.NewAlpha(rect)
	rect = rect.Intersect(mask.Bounds())
	if rect.Empty() {
		return mask
	}
	inner := insetRect(rect, width)
	innerRadius := maxInt(0, radius-width)
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if !pointInRoundedRect(x, y, rect, radius) {
				continue
			}
			if !inner.Empty() && pointInRoundedRect(x, y, inner, innerRadius) {
				continue
			}
			mask.SetAlpha(x, y, color.Alpha{A: 255})
		}
	}
	return mask
}

func pointInRoundedRect(x, y int, rect image.Rectangle, radius int) bool {
	if rect.Empty() {
		return false
	}
	radius = normalizeRoundedRadius(rect, radius)
	if radius <= 0 {
		return x >= rect.Min.X && x < rect.Max.X && y >= rect.Min.Y && y < rect.Max.Y
	}
	if x < rect.Min.X || x >= rect.Max.X || y < rect.Min.Y || y >= rect.Max.Y {
		return false
	}
	left := rect.Min.X + radius
	right := rect.Max.X - radius - 1
	top := rect.Min.Y + radius
	bottom := rect.Max.Y - radius - 1
	if x >= left && x <= right {
		return true
	}
	if y >= top && y <= bottom {
		return true
	}
	var cx, cy int
	switch {
	case x < left && y < top:
		cx, cy = left, top
	case x > right && y < top:
		cx, cy = right, top
	case x < left && y > bottom:
		cx, cy = left, bottom
	default:
		cx, cy = right, bottom
	}
	dx := x - cx
	dy := y - cy
	return dx*dx+dy*dy <= radius*radius
}

func normalizeRoundedRadius(rect image.Rectangle, radius int) int {
	if radius <= 0 {
		return 0
	}
	maxRadius := minInt(rect.Dx()/2, rect.Dy()/2)
	if radius > maxRadius {
		return maxRadius
	}
	return radius
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func (e *FallbackEngine) renderSceneComposeSized(ctx context.Context, req *MediaRequest, width, height int) (string, []string, error) {
	if e == nil || e.sceneComposer == nil {
		return "", nil, fmt.Errorf("scene compose is not configured")
	}
	result, err := e.sceneComposer.Compose(ctx, scenecompose.ComposeRequest{
		Prompt: strings.TrimSpace(req.Prompt),
		Width:  width,
		Height: height,
		Locale: e.locale,
	})
	if err != nil {
		return "", nil, err
	}
	if result == nil || result.Image == nil {
		return "", nil, fmt.Errorf("scene compose returned no image")
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, result.Image); err != nil {
		return "", nil, fmt.Errorf("encode scene compose image: %w", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), assetRefsToSourceURLs(result.UsedAssets), nil
}

func slideCanvasSize(aspectRatio string, baseWidth int) slideCanvasSpec {
	width := baseWidth
	if width <= 0 {
		width = 1280
	}
	wRatio, hRatio := 16.0, 9.0
	trimmed := strings.TrimSpace(aspectRatio)
	if trimmed != "" {
		for _, sep := range []string{":", "/"} {
			if parts := strings.Split(trimmed, sep); len(parts) == 2 {
				left := parsePositiveFloat(parts[0])
				right := parsePositiveFloat(parts[1])
				if left > 0 && right > 0 {
					wRatio, hRatio = left, right
				}
				break
			}
		}
	}
	height := int(math.Round(float64(width) * hRatio / wRatio))
	if height <= 0 {
		height = 720
	}
	return slideCanvasSpec{width: width, height: height}
}

func parsePositiveFloat(raw string) float64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value <= 0 {
		return 0
	}
	return value
}

func appendUniqueStrings(existing []string, values ...string) []string {
	seen := make(map[string]struct{}, len(existing))
	for _, item := range existing {
		seen[item] = struct{}{}
	}
	out := append([]string(nil), existing...)
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func pickSlidePalette(brief slidespec.Brief) slidePalette {
	lower := strings.ToLower(strings.Join([]string{brief.Theme, brief.VisualQuery, brief.Title}, " "))
	switch {
	case strings.Contains(lower, "green") || strings.Contains(lower, "sustain") || strings.Contains(lower, "生态"):
		return slidePalette{
			Top:    color.RGBA{R: 12, G: 48, B: 41, A: 255},
			Bottom: color.RGBA{R: 4, G: 21, B: 18, A: 255},
			Panel:  color.RGBA{R: 18, G: 76, B: 66, A: 220},
			Accent: color.RGBA{R: 110, G: 231, B: 183, A: 255},
			Text:   color.RGBA{R: 246, G: 253, B: 250, A: 255},
			Muted:  color.RGBA{R: 207, G: 250, B: 236, A: 255},
		}
	case strings.Contains(lower, "finance") || strings.Contains(lower, "strategy") || strings.Contains(lower, "战略"):
		return slidePalette{
			Top:    color.RGBA{R: 55, G: 31, B: 24, A: 255},
			Bottom: color.RGBA{R: 16, G: 11, B: 10, A: 255},
			Panel:  color.RGBA{R: 90, G: 52, B: 34, A: 220},
			Accent: color.RGBA{R: 251, G: 191, B: 36, A: 255},
			Text:   color.RGBA{R: 255, G: 251, B: 235, A: 255},
			Muted:  color.RGBA{R: 254, G: 240, B: 204, A: 255},
		}
	case slidespec.IsNanoSlidesPreset(brief.StylePreset):
		return slidePalette{
			Top:    color.RGBA{R: 12, G: 20, B: 36, A: 255},
			Bottom: color.RGBA{R: 6, G: 11, B: 21, A: 255},
			Panel:  color.RGBA{R: 34, G: 43, B: 62, A: 220},
			Accent: color.RGBA{R: 244, G: 196, B: 92, A: 255},
			Text:   color.RGBA{R: 248, G: 246, B: 240, A: 255},
			Muted:  color.RGBA{R: 211, G: 218, B: 230, A: 255},
		}
	default:
		return slidePalette{
			Top:    color.RGBA{R: 17, G: 36, B: 71, A: 255},
			Bottom: color.RGBA{R: 7, G: 17, B: 31, A: 255},
			Panel:  color.RGBA{R: 27, G: 59, B: 119, A: 220},
			Accent: color.RGBA{R: 125, G: 211, B: 252, A: 255},
			Text:   color.RGBA{R: 248, G: 250, B: 252, A: 255},
			Muted:  color.RGBA{R: 219, G: 234, B: 254, A: 255},
		}
	}
}

func decodeDataURLImage(raw string) image.Image {
	_, payload, ok := parseDataURLPayload(raw)
	if !ok || len(payload) == 0 {
		return nil
	}
	img, _, err := image.Decode(bytes.NewReader(payload))
	if err != nil {
		return nil
	}
	return img
}

func drawImageCoverRect(dst *image.RGBA, src image.Image, rect image.Rectangle) {
	if dst == nil || src == nil {
		return
	}
	rect = rect.Intersect(dst.Bounds())
	if rect.Empty() {
		return
	}
	sw := src.Bounds().Dx()
	sh := src.Bounds().Dy()
	if sw <= 0 || sh <= 0 {
		return
	}
	scale := math.Max(float64(rect.Dx())/float64(sw), float64(rect.Dy())/float64(sh))
	tw := maxInt(1, int(float64(sw)*scale))
	th := maxInt(1, int(float64(sh)*scale))
	scaled := image.NewRGBA(image.Rect(0, 0, tw, th))
	draw.CatmullRom.Scale(scaled, scaled.Bounds(), src, src.Bounds(), stdDraw.Over, nil)
	offset := image.Pt((tw-rect.Dx())/2, (th-rect.Dy())/2)
	stdDraw.Draw(dst, rect, scaled, offset, stdDraw.Src)
}

func slideEyebrowText(brief slidespec.Brief, templateID string) string {
	if theme := strings.TrimSpace(brief.Theme); theme != "" {
		return theme
	}
	cjk := containsSlideCJK(strings.Join([]string{brief.Title, brief.Subtitle, brief.VisualQuery}, " "))
	switch templateID {
	case slidespec.TemplateTextOnly:
		if cjk {
			if slidespec.IsNanoSlidesPreset(brief.StylePreset) {
				return "战略摘要"
			}
			return "执行摘要"
		}
		if slidespec.IsNanoSlidesPreset(brief.StylePreset) {
			return "Strategy Summary"
		}
		return "Executive Summary"
	case slidespec.TemplateSplit:
		if cjk {
			if slidespec.IsNanoSlidesPreset(brief.StylePreset) {
				return "关键洞察"
			}
			return "核心要点"
		}
		if slidespec.IsNanoSlidesPreset(brief.StylePreset) {
			return "Key Insights"
		}
		return "Key Highlights"
	default:
		if cjk {
			if slidespec.IsNanoSlidesPreset(brief.StylePreset) {
				return "战略概览"
			}
			return "演示概览"
		}
		if slidespec.IsNanoSlidesPreset(brief.StylePreset) {
			return "Strategy Overview"
		}
		return "Presentation Overview"
	}
}

func slideDefaultTitle(brief slidespec.Brief, templateID string) string {
	if slidespec.IsNanoSlidesPreset(brief.StylePreset) {
		switch templateID {
		case slidespec.TemplateTextOnly:
			return "Strategy summary"
		case slidespec.TemplateSplit:
			return "Key insights"
		default:
			return "Strategy overview"
		}
	}
	switch templateID {
	case slidespec.TemplateTextOnly:
		return "Executive summary"
	case slidespec.TemplateSplit:
		return "Key highlights"
	default:
		return "Presentation overview"
	}
}

func containsSlideCJK(value string) bool {
	for _, r := range value {
		if unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul) {
			return true
		}
	}
	return false
}

func trimToASCII(value string) string {
	return trimToASCIIWithFallback(value, "Presentation overview")
}

func trimToASCIIWithFallback(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return strings.TrimSpace(fallback)
	}
	var b strings.Builder
	for _, r := range value {
		if r > unicode.MaxASCII {
			b.WriteRune(' ')
			continue
		}
		b.WriteRune(r)
	}
	trimmed := strings.TrimSpace(strings.Join(strings.Fields(b.String()), " "))
	if trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(fallback)
}

func firstNonEmptyValue(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func slideStyleClass(stylePreset string) string {
	switch slidespec.NormalizeStylePreset(stylePreset) {
	case slidespec.StylePresetNanoSlides:
		return "style-nano"
	case slidespec.StylePresetBananaSlides:
		return "style-banana"
	default:
		return "style-default"
	}
}
