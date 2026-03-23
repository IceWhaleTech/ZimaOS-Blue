package ppt

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/scenecompose"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/slidespec"
)

type LayoutPlanRequest struct {
	Description    string
	Theme          string
	StylePreset    string
	AspectRatio    string
	Lang           string
	Width          int
	Height         int
	HasVisual      bool
	ReferenceCount int
	Brief          slidespec.Brief
}

type LayoutPlanner interface {
	Plan(ctx context.Context, req LayoutPlanRequest) (*slidespec.LayoutSpec, error)
}

type LLMLayoutPlanner struct {
	llm scenecompose.LLMCaller
}

func NewLayoutPlanner(llmCaller scenecompose.LLMCaller) *LLMLayoutPlanner {
	return &LLMLayoutPlanner{llm: llmCaller}
}

func (p *LLMLayoutPlanner) SetLLM(llmCaller scenecompose.LLMCaller) {
	if p == nil {
		return
	}
	p.llm = llmCaller
}

func (p *LLMLayoutPlanner) Plan(ctx context.Context, req LayoutPlanRequest) (*slidespec.LayoutSpec, error) {
	prepared := normalizeLayoutPlanRequest(req)
	if p != nil && p.llm != nil {
		planned, err := p.planWithLLM(ctx, prepared)
		if err == nil && planned != nil {
			spec := slidespec.NormalizeLayoutSpec(*planned, prepared.Brief, prepared.Width, prepared.Height, prepared.HasVisual)
			return &spec, nil
		}
	}
	spec := slidespec.BuildLayoutSpec(prepared.Brief, prepared.Width, prepared.Height, prepared.HasVisual)
	return &spec, nil
}

func (p *LLMLayoutPlanner) planWithLLM(ctx context.Context, req LayoutPlanRequest) (*slidespec.LayoutSpec, error) {
	resp, err := p.llm.Chat(ctx, llm.ChatRequest{
		Model: "auto",
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: layoutPlannerSystemPrompt()},
			{Role: llm.RoleUser, Content: layoutPlannerUserPrompt(req)},
		},
		MaxTokens:   1200,
		Temperature: 0,
	})
	if err != nil {
		return nil, err
	}
	payload := trimJSONPayload(resp.Message.Content)
	if payload == "" {
		return nil, fmt.Errorf("layout planner returned empty content")
	}
	var spec slidespec.LayoutSpec
	if err := json.Unmarshal([]byte(payload), &spec); err != nil {
		return nil, fmt.Errorf("layout planner json: %w", err)
	}
	return &spec, nil
}

func normalizeLayoutPlanRequest(req LayoutPlanRequest) LayoutPlanRequest {
	req.Description = strings.TrimSpace(req.Description)
	req.Theme = strings.TrimSpace(req.Theme)
	req.StylePreset = slidespec.NormalizeStylePreset(req.StylePreset)
	req.AspectRatio = strings.TrimSpace(req.AspectRatio)
	req.Lang = strings.TrimSpace(req.Lang)
	if req.ReferenceCount < 0 {
		req.ReferenceCount = 0
	}
	if req.Brief.RenderMode == "" {
		req.Brief = slidespec.Build(slidespec.Input{
			Prompt:         req.Description,
			Description:    req.Description,
			StylePreset:    req.StylePreset,
			QualityProfile: DefaultQualityProfile,
			Theme:          req.Theme,
			AspectRatio:    req.AspectRatio,
			Source:         "ppt",
			Lang:           req.Lang,
		})
	}
	if req.Brief.RenderMode == "" || req.Brief.RenderMode == slidespec.RenderModePoster {
		req.Brief.RenderMode = slidespec.RenderModeSlide
	}
	if req.Brief.StylePreset == "" {
		req.Brief.StylePreset = firstNonEmptyString(req.StylePreset, DefaultStylePreset)
	}
	if req.Brief.Theme == "" {
		req.Brief.Theme = req.Theme
	}
	if req.Brief.AspectRatio == "" {
		req.Brief.AspectRatio = firstNonEmptyString(req.AspectRatio, slidespec.DefaultAspectRatio)
	}
	if req.Brief.TemplateID == "" || req.Brief.TemplateID == slidespec.TemplatePoster {
		req.Brief.TemplateID = slidespec.DecideTemplate(req.Brief, req.HasVisual)
	}
	if req.Width <= 0 || req.Height <= 0 {
		req.Width, req.Height = slideCanvasDimensions(req.Brief.AspectRatio)
	}
	return req
}

func layoutPlannerSystemPrompt() string {
	return strings.TrimSpace(`
You are a deterministic PPT slide layout planner for a direct-rendering engine.
Return only one JSON object with this exact schema:
{
  "render_mode": "slide",
  "style_preset": "string",
  "template_id": "cover|split|text_only|agenda|metrics",
  "theme": "string",
  "canvas": {
    "width": 1280,
    "height": 720,
    "aspect_ratio": "16:9"
  },
  "background": {
    "fill": "#0F172A",
    "gradient": ["#0F172A", "#1E293B"],
    "image_source": "visual",
    "image_fit": "cover|contain",
    "overlay": "rgba(15,23,42,0.28)"
  },
  "elements": [
    {
      "kind": "text|image|shape|chart",
      "name": "string",
      "x": 96,
      "y": 120,
      "width": 420,
      "height": 120,
      "z_index": 1,
      "radius": 24,
      "fill": "#111827",
      "stroke": "rgba(255,255,255,0.16)",
      "stroke_width": 1,
      "text": "string",
      "font_family": "sans|mono|display",
      "font_role": "eyebrow|title|subtitle|body|chip",
      "font_size": 52,
      "font_weight": "regular|medium|semibold|bold|italic",
      "color": "#F8FAFC",
      "align": "left|center|right",
      "max_lines": 3,
      "line_height": 1.15,
      "source": "visual",
      "fit": "cover|contain",
      "chart_type": "divider|progress|sparkline|bars",
      "value": 72,
      "max_value": 100,
      "values": [18, 24, 30, 44, 52],
      "labels": ["Q1", "Q2", "Q3", "Q4", "Q5"],
      "value_format": "number|integer|decimal1|decimal2|percent|currency|multiplier",
      "direction": "horizontal|vertical",
      "accent": "#7DD3FC"
    }
  ]
}
Rules:
- Use absolute pixel coordinates inside the provided canvas.
- Keep every element fully inside the canvas.
- Use at most 7 elements total.
- Prefer 1 image slot at most.
- Preserve the user's concrete slide copy. Do not invent long paragraphs.
- Keep text concise: title, optional subtitle, and up to 3 short bullets or labels.
- Use only supported font families: "sans", "mono", or "display".
- Use "mono" mainly for KPI values or code-like labels, and "display" mainly for short eyebrow/caps labels.
- Use "visual" as the only image source alias when reserving a hero/background image slot.
- If no visual is allowed, do not emit image elements and do not set background.image_source.
- If the template is "cover", emphasize a large title and one strong visual area.
- If the template is "split", keep a clear text column and a clear visual column.
- If the template is "text_only", use shapes, gradients, and spacing instead of image slots.
- If the template is "agenda", arrange the bullets as sequential step cards or section cards.
- If the template is "metrics", arrange the bullets as stat cards with big numeric values and short labels.
- Use chart primitives sparingly and only when they clearly help: divider for section separation, sparkline for trends, progress for a single ratio, bars for compact comparisons.
- Use "labels" only for short chart captions or category labels, and keep it aligned with the "values" array.
- Use "value_format" when the chart should expose a formatted number, such as percent, currency, or multiplier.
- Use "direction" only for progress and bars. Prefer horizontal progress bars and vertical comparison bars unless the copy is easier to read the other way.
- Prefer high contrast and presentation-safe margins.
- Output valid JSON only. No markdown, no comments.`)
}

func layoutPlannerUserPrompt(req LayoutPlanRequest) string {
	brief := req.Brief
	var sb strings.Builder
	sb.WriteString("Slide intent:\n")
	sb.WriteString(firstNonEmptyString(req.Description, brief.Title, brief.VisualQuery))
	sb.WriteString("\n\nCanvas:\n")
	sb.WriteString(fmt.Sprintf("%d x %d (%s)", req.Width, req.Height, brief.AspectRatio))
	sb.WriteString("\n\nPlanner context:\n")
	sb.WriteString(fmt.Sprintf("- style_preset: %s", firstNonEmptyString(brief.StylePreset, DefaultStylePreset)))
	sb.WriteString(fmt.Sprintf("\n- template_hint: %s", firstNonEmptyString(brief.TemplateID, slidespec.TemplateSplit)))
	sb.WriteString(fmt.Sprintf("\n- lang: %s", firstNonEmptyString(req.Lang, "auto")))
	if brief.Theme != "" {
		sb.WriteString(fmt.Sprintf("\n- theme: %s", brief.Theme))
	}
	sb.WriteString("\n\nStructured brief:\n")
	sb.WriteString(fmt.Sprintf("- title: %s", firstNonEmptyString(brief.Title, "(none)")))
	sb.WriteString(fmt.Sprintf("\n- subtitle: %s", firstNonEmptyString(brief.Subtitle, "(none)")))
	if len(brief.Bullets) == 0 {
		sb.WriteString("\n- bullets: (none)")
	} else {
		sb.WriteString("\n- bullets:")
		for _, bullet := range brief.Bullets {
			sb.WriteString("\n  - ")
			sb.WriteString(strings.TrimSpace(bullet))
		}
	}
	sb.WriteString(fmt.Sprintf("\n- visual_query: %s", firstNonEmptyString(brief.VisualQuery, "(none)")))
	sb.WriteString("\n\nVisual availability:\n")
	if req.HasVisual {
		sb.WriteString("A visual slot is allowed. Use source=\"visual\" if a hero image or background image helps the composition.")
		if req.ReferenceCount > 0 {
			sb.WriteString(fmt.Sprintf(" There are %d reference images attached.", req.ReferenceCount))
		}
	} else {
		sb.WriteString("No visual slot is allowed. Use shapes, panels, and gradients only.")
	}
	return sb.String()
}

func slideCanvasDimensions(aspectRatio string) (int, int) {
	ratio := strings.TrimSpace(aspectRatio)
	if ratio == "" {
		ratio = slidespec.DefaultAspectRatio
	}
	parts := strings.Split(ratio, ":")
	if len(parts) != 2 {
		return 1280, 720
	}
	left, errLeft := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	right, errRight := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if errLeft != nil || errRight != nil || left <= 0 || right <= 0 {
		return 1280, 720
	}
	const longSide = 1280.0
	if left >= right {
		return int(longSide), maxInt(int(math.Round(longSide*right/left)), 1)
	}
	return maxInt(int(math.Round(longSide*left/right)), 1), int(longSide)
}

func trimJSONPayload(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "```") {
		trimmed = strings.TrimPrefix(trimmed, "```json")
		trimmed = strings.TrimPrefix(trimmed, "```JSON")
		trimmed = strings.TrimPrefix(trimmed, "```")
		trimmed = strings.TrimSuffix(trimmed, "```")
		trimmed = strings.TrimSpace(trimmed)
	}
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end > start {
		return trimmed[start : end+1]
	}
	return trimmed
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func maxInt(a, b int) int {
	if a >= b {
		return a
	}
	return b
}
