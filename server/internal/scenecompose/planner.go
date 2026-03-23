package scenecompose

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

type Planner struct {
	llm LLMCaller
}

func NewPlanner(llmCaller LLMCaller) *Planner {
	return &Planner{llm: llmCaller}
}

func (p *Planner) SetLLM(llmCaller LLMCaller) {
	if p == nil {
		return
	}
	p.llm = llmCaller
}

func (p *Planner) Plan(ctx context.Context, req ComposeRequest) (*ScenePlan, error) {
	if p != nil && p.llm != nil {
		planned, err := p.planWithLLM(ctx, req)
		if err == nil && planned != nil {
			return normalizeScenePlan(planned), nil
		}
	}
	return normalizeScenePlan(heuristicPlan(req.Prompt)), nil
}

func (p *Planner) planWithLLM(ctx context.Context, req ComposeRequest) (*ScenePlan, error) {
	resp, err := p.llm.Chat(ctx, llm.ChatRequest{
		Model: "auto",
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: scenePlannerSystemPrompt()},
			{Role: llm.RoleUser, Content: scenePlannerUserPrompt(req)},
		},
		MaxTokens:   700,
		Temperature: 0,
	})
	if err != nil {
		return nil, err
	}
	payload := trimJSONPayload(resp.Message.Content)
	if payload == "" {
		return nil, fmt.Errorf("scene planner returned empty content")
	}
	var plan ScenePlan
	if err := json.Unmarshal([]byte(payload), &plan); err != nil {
		return nil, fmt.Errorf("scene planner json: %w", err)
	}
	return &plan, nil
}

func scenePlannerSystemPrompt() string {
	return strings.TrimSpace(`
You are a scene planner for a lightweight image composition engine.
Return only one JSON object with this exact schema:
{
  "background": "string",
  "style": "string",
  "lighting": "string",
  "time_of_day": "string",
  "weather": "string",
  "camera_view": "string",
  "scene_query": "string",
  "scene_query_en": "string",
  "background_query": "string",
  "background_query_en": "string",
  "foreground": [
    {
      "id": "string",
      "type": "string",
      "attributes": ["string"],
      "search_query": "string",
      "search_query_en": "string",
      "fallback_query": "string",
      "fallback_query_en": "string",
      "priority": 1,
      "layout": {
        "horizontal": "left|center|right",
        "vertical": "low|middle|high",
        "depth": "foreground|midground",
        "scale": "small|medium|large",
        "grounded": true
      }
    }
  ]
}
Rules:
- Keep at most 2 foreground objects.
- The main object must have priority 1.
- Use discrete layout labels only.
- Default to realistic / soft natural light / day / clear / eye-level when unspecified.
- Prefer midground for the main object.
- "scene_query" should be a concise search phrase for a full-scene reference image.
- "background_query" should search only for the background/environment.
- Each foreground "search_query" should target a compositable subject asset and preserve the user's concrete subject wording when possible.
- Each foreground "fallback_query" should be a simpler alternate subject search without relying on transparent PNG results.
- When the user prompt is not already pure English, also provide concise pure-English variants in "scene_query_en", "background_query_en", "search_query_en", and "fallback_query_en" for Bing/Google image search.
- Keep the English variants natural and search-friendly rather than literal word-by-word translations.
- Remove assistant phrasing such as "please generate" or "帮我生成" from search queries.
- For Chinese prompts, keep the search queries in Chinese when possible.
- Output valid JSON only, no markdown.`)
}

func scenePlannerUserPrompt(req ComposeRequest) string {
	prompt := strings.TrimSpace(trimSearchPromptPrefix(req.Prompt))
	if prompt == "" {
		prompt = strings.TrimSpace(req.Prompt)
	}
	var sb strings.Builder
	sb.WriteString("Prompt:\n")
	sb.WriteString(prompt)
	if strings.TrimSpace(req.Locale) != "" {
		sb.WriteString("\n\nLocale:\n")
		sb.WriteString(strings.TrimSpace(req.Locale))
	}
	if req.Width > 0 && req.Height > 0 {
		sb.WriteString(fmt.Sprintf("\n\nCanvas:\n%d x %d", req.Width, req.Height))
	}
	return sb.String()
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

func heuristicPlan(prompt string) *ScenePlan {
	normalized := strings.TrimSpace(prompt)
	if normalized == "" {
		normalized = "a realistic subject in a natural setting"
	}
	lower := strings.ToLower(normalized)
	style := detectStyle(lower)
	segments := detectForegroundSegments(normalized)
	background := detectBackground(normalized)
	if background == "" {
		background = inferBackgroundFromText(lower)
	}
	if background == "" {
		background = "natural outdoor scene"
	}

	foreground := make([]ForegroundPlan, 0, 2)
	for idx, segment := range segments {
		if len(foreground) >= 2 {
			break
		}
		objType := detectObjectType(segment)
		if objType == "" {
			objType = defaultObjectType(idx)
		}
		subjectQuery := detectSubjectSearchPhrase(segment, background, objType)
		foreground = append(foreground, ForegroundPlan{
			ID:            fmt.Sprintf("%s_%d", sanitizeIDToken(objType), idx+1),
			Type:          objType,
			Attributes:    detectAttributes(segment),
			SearchQuery:   buildHeuristicForegroundSearchQuery(subjectQuery, style),
			FallbackQuery: buildHeuristicForegroundFallbackQuery(subjectQuery, style),
			Priority:      idx + 1,
			Layout:        detectLayout(segment, idx),
		})
	}
	if len(foreground) == 0 {
		objType := detectObjectType(normalized)
		subjectQuery := detectSubjectSearchPhrase(normalized, background, objType)
		foreground = []ForegroundPlan{{
			ID:            "subject_1",
			Type:          objType,
			SearchQuery:   buildHeuristicForegroundSearchQuery(subjectQuery, style),
			FallbackQuery: buildHeuristicForegroundFallbackQuery(subjectQuery, style),
			Priority:      1,
			Layout:        detectLayout(normalized, 0),
		}}
	}

	return &ScenePlan{
		Background: background,
		Style:      style,
		Lighting:   detectLighting(lower),
		TimeOfDay:  detectTimeOfDay(lower),
		Weather:    detectWeather(lower),
		CameraView: detectCameraView(lower),
		Foreground: foreground,
	}
}

var (
	chineseSplitRE = regexp.MustCompile(`[，,、和与并及]+`)
	spaceSplitRE   = regexp.MustCompile(`\s+`)
)

func detectForegroundSegments(prompt string) []string {
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" {
		return nil
	}
	if directionalCueMatcher.Contains(trimmed) {
		parts := chineseSplitRE.Split(trimmed, -1)
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if directionalCueMatcher.Contains(part) {
				out = append(out, part)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return []string{trimmed}
}

func detectBackground(prompt string) string {
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	patterns := []struct {
		markers []string
		trimSet string
	}{
		{markers: []string{" in ", " inside ", " within ", " at ", " on "}, trimSet: " ,.;:"},
		{markers: []string{"在", "里", "中"}, trimSet: " ，。；;"},
	}
	for _, group := range patterns {
		for _, marker := range group.markers {
			var idx int
			if strings.Contains(marker, " ") {
				idx = strings.LastIndex(lower, marker)
			} else {
				idx = strings.LastIndex(trimmed, marker)
			}
			if idx < 0 {
				continue
			}
			value := trimmed[idx+len(strings.TrimSpace(marker)):]
			value = strings.Trim(value, group.trimSet)
			if value != "" {
				return compactBackgroundPhrase(value)
			}
		}
	}
	return ""
}

func inferBackgroundFromText(lower string) string {
	return matchCueChoice(lower, backgroundInferenceChoices)
}

func detectObjectType(segment string) string {
	lower := strings.ToLower(segment)
	if matched := matchCueChoice(lower, objectTypeChoices); matched != "" {
		return matched
	}
	tokens := spaceSplitRE.Split(lower, -1)
	for _, token := range tokens {
		token = strings.Trim(token, " ,.;:!?()[]{}")
		if token == "" {
			continue
		}
		if _, skip := objectTypeStopwords[token]; skip {
			continue
		}
		if len(token) >= 3 {
			return token
		}
	}
	return "subject"
}

func detectAttributes(segment string) []string {
	return collectCueChoices(segment, attributeChoices)
}

func detectSubjectSearchPhrase(segment, background, fallbackType string) string {
	value := strings.TrimSpace(segment)
	if value == "" {
		return strings.TrimSpace(fallbackType)
	}
	value = trimSearchPromptPrefix(value)
	value = trimLeadingSubjectPositionCue(value)
	value = trimSubjectBackgroundClause(value)
	if background != "" {
		value = strings.TrimSpace(strings.TrimSuffix(value, background))
	}
	value = trimSubjectActionTail(value)
	value = trimLeadingSubjectQuantifier(value)
	value = strings.TrimSpace(strings.Trim(value, "，,.;:。!?！？-—_"))
	value = normalizeSearchText(value)
	if value != "" {
		return value
	}
	return strings.TrimSpace(fallbackType)
}

func trimSearchPromptPrefix(raw string) string {
	value := strings.TrimSpace(raw)
	for _, prefix := range []string{
		"请帮我生成一张", "请帮我生成一个", "帮我生成一张", "帮我生成一个",
		"给我生成一张", "请生成一张", "生成一张", "帮我", "请你", "请", "给我",
		"please generate an image of", "please create an image of", "generate an image of", "create an image of",
		"please generate a photo of", "please create a photo of", "generate a photo of", "create a photo of",
	} {
		if strings.HasPrefix(strings.ToLower(value), strings.ToLower(prefix)) {
			value = strings.TrimSpace(value[len(prefix):])
			value = strings.TrimLeft(value, "，,.;:。!?！？-—_ ")
			if value != "" {
				return value
			}
		}
	}
	return value
}

func trimLeadingSubjectPositionCue(raw string) string {
	value := strings.TrimSpace(raw)
	for _, prefix := range []string{
		"左边", "右边", "左侧", "右侧", "中间", "中央", "居中",
		"on the left", "on the right", "at the center",
		"left side", "right side", "center",
		"left", "right",
	} {
		if strings.HasPrefix(strings.ToLower(value), strings.ToLower(prefix)) {
			value = strings.TrimSpace(value[len(prefix):])
			value = strings.TrimLeft(value, "，,.;:。!?！？-—_ ")
		}
	}
	return value
}

func trimSubjectBackgroundClause(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	patterns := []string{" in ", " inside ", " within ", " at ", " on ", "在"}
	best := value
	for _, marker := range patterns {
		var idx int
		if strings.Contains(marker, " ") {
			idx = strings.LastIndex(strings.ToLower(value), marker)
		} else {
			idx = strings.LastIndex(value, marker)
		}
		if idx <= 0 {
			continue
		}
		candidate := strings.TrimSpace(value[:idx])
		candidate = strings.TrimRight(candidate, "的在于和与及、")
		if candidate != "" {
			best = candidate
			break
		}
	}
	return best
}

func trimLeadingSubjectQuantifier(raw string) string {
	value := strings.TrimSpace(raw)
	for _, prefix := range []string{
		"一只", "一条", "一个", "一名", "一位", "一头",
		"a ", "an ", "the ",
	} {
		if strings.HasPrefix(strings.ToLower(value), strings.ToLower(prefix)) {
			value = strings.TrimSpace(value[len(prefix):])
			break
		}
	}
	return value
}

func trimSubjectActionTail(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	for _, marker := range []string{
		"玩耍", "玩着", "奔跑", "跑着", "跳跃", "跳着", "漫步", "散步", "行走",
		"站着", "坐着", "躺着", "飞着", "漂浮", "看着", "读着", "睡着",
		"playing", "running", "jumping", "walking", "standing", "sitting", "floating", "reading", "sleeping",
	} {
		idx := strings.Index(strings.ToLower(value), strings.ToLower(marker))
		if idx <= 0 {
			continue
		}
		candidate := strings.TrimSpace(value[:idx])
		candidate = strings.TrimRight(candidate, "的在于和与及、")
		if candidate != "" {
			return candidate
		}
	}
	return value
}

func buildHeuristicForegroundSearchQuery(subject, style string) string {
	base := normalizeSearchText(subject)
	if base == "" {
		return ""
	}
	style = heuristicSearchStyle(style)
	if containsCJK(base) {
		return compactQuery(base, style, "透明背景 png")
	}
	return compactQuery(base, style, "isolated png transparent")
}

func buildHeuristicForegroundFallbackQuery(subject, style string) string {
	base := normalizeSearchText(subject)
	if base == "" {
		return ""
	}
	style = heuristicSearchStyle(style)
	if containsCJK(base) {
		return compactQuery(base, style, "写实照片")
	}
	return compactQuery(base, style, "photo")
}

func heuristicSearchStyle(style string) string {
	value := strings.TrimSpace(style)
	if value == "" || value == "realistic" {
		return ""
	}
	return value
}

func detectLayout(segment string, idx int) LayoutHint {
	horizontal := "center"
	if matched := matchCueChoice(segment, layoutHorizontalChoices); matched != "" {
		horizontal = matched
	}

	vertical := "low"
	if matched := matchCueChoice(segment, layoutVerticalChoices); matched != "" {
		vertical = matched
	}

	scale := "medium"
	if matched := matchCueChoice(segment, layoutScaleChoices); matched != "" {
		scale = matched
	}

	depth := "midground"
	if idx > 0 {
		depth = "foreground"
	}

	grounded := true
	if layoutFloatingCueMatcher.Contains(segment) {
		grounded = false
		vertical = "high"
	}

	return LayoutHint{
		Horizontal: horizontal,
		Vertical:   vertical,
		Depth:      depth,
		Scale:      scale,
		Grounded:   grounded,
	}
}

func detectStyle(lower string) string {
	if matched := matchCueChoice(lower, styleChoices); matched != "" {
		return matched
	}
	return "realistic"
}

func detectLighting(lower string) string {
	if matched := matchCueChoice(lower, lightingChoices); matched != "" {
		return matched
	}
	return "soft natural light"
}

func detectTimeOfDay(lower string) string {
	if matched := matchCueChoice(lower, timeOfDayChoices); matched != "" {
		return matched
	}
	return "day"
}

func detectWeather(lower string) string {
	if matched := matchCueChoice(lower, weatherChoices); matched != "" {
		return matched
	}
	return "clear"
}

func detectCameraView(lower string) string {
	if matched := matchCueChoice(lower, cameraViewChoices); matched != "" {
		return matched
	}
	return "eye-level"
}

func normalizeScenePlan(plan *ScenePlan) *ScenePlan {
	if plan == nil {
		plan = &ScenePlan{}
	}
	plan.Background = strings.TrimSpace(plan.Background)
	if plan.Background == "" {
		plan.Background = "natural outdoor scene"
	}
	plan.Style = normalizeEnum(plan.Style, []string{"realistic", "cartoon", "illustration"}, "realistic")
	if strings.TrimSpace(plan.Style) == "" {
		plan.Style = "realistic"
	}
	if strings.TrimSpace(plan.Lighting) == "" {
		plan.Lighting = "soft natural light"
	}
	if strings.TrimSpace(plan.TimeOfDay) == "" {
		plan.TimeOfDay = "day"
	}
	if strings.TrimSpace(plan.Weather) == "" {
		plan.Weather = "clear"
	}
	if strings.TrimSpace(plan.CameraView) == "" {
		plan.CameraView = "eye-level"
	}
	plan.SceneQuery = normalizeSearchText(plan.SceneQuery)
	plan.SceneQueryEN = normalizeSearchText(plan.SceneQueryEN)
	plan.BackgroundQuery = normalizeSearchText(plan.BackgroundQuery)
	plan.BackgroundQueryEN = normalizeSearchText(plan.BackgroundQueryEN)

	normalized := make([]ForegroundPlan, 0, len(plan.Foreground))
	for idx, item := range plan.Foreground {
		item.Type = strings.TrimSpace(item.Type)
		if item.Type == "" {
			item.Type = defaultObjectType(idx)
		}
		item.ID = strings.TrimSpace(item.ID)
		if item.ID == "" {
			item.ID = fmt.Sprintf("%s_%d", sanitizeIDToken(item.Type), idx+1)
		}
		if item.Priority <= 0 {
			item.Priority = idx + 1
		}
		item.SearchQuery = normalizeSearchText(item.SearchQuery)
		item.SearchQueryEN = normalizeSearchText(item.SearchQueryEN)
		item.FallbackQuery = normalizeSearchText(item.FallbackQuery)
		item.FallbackQueryEN = normalizeSearchText(item.FallbackQueryEN)
		item.Layout.Horizontal = normalizeEnum(item.Layout.Horizontal, []string{"left", "center", "right"}, "center")
		item.Layout.Vertical = normalizeEnum(item.Layout.Vertical, []string{"low", "middle", "high"}, "low")
		item.Layout.Depth = normalizeEnum(item.Layout.Depth, []string{"foreground", "midground"}, "midground")
		item.Layout.Scale = normalizeEnum(item.Layout.Scale, []string{"small", "medium", "large"}, "medium")
		if idx == 0 {
			item.Priority = 1
			item.Layout.Depth = "midground"
			if item.Layout.Horizontal == "" {
				item.Layout.Horizontal = "center"
			}
		}
		normalized = append(normalized, item)
		if len(normalized) >= 2 {
			break
		}
	}
	if len(normalized) == 0 {
		normalized = append(normalized, ForegroundPlan{
			ID:       "subject_1",
			Type:     "subject",
			Priority: 1,
			Layout: LayoutHint{
				Horizontal: "center",
				Vertical:   "low",
				Depth:      "midground",
				Scale:      "medium",
				Grounded:   true,
			},
		})
	}
	if !normalized[0].Layout.Grounded && normalized[0].Layout.Vertical == "" {
		normalized[0].Layout.Vertical = "middle"
	}
	plan.Foreground = normalized
	return plan
}

func normalizeSearchText(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	return strings.Join(strings.Fields(trimmed), " ")
}

func normalizeEnum(raw string, allowed []string, fallback string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	for _, item := range allowed {
		if value == item {
			return value
		}
	}
	return fallback
}

func sanitizeIDToken(raw string) string {
	lower := strings.ToLower(strings.TrimSpace(raw))
	if lower == "" {
		return "subject"
	}
	var b strings.Builder
	for _, r := range lower {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	token := strings.Trim(b.String(), "_")
	if token == "" {
		return "subject"
	}
	return token
}

func compactBackgroundPhrase(raw string) string {
	value := strings.TrimSpace(raw)
	value = strings.Trim(value, "，,.;:。!?！？")
	value = trimBackgroundActionTail(value)
	if len([]rune(value)) > 48 {
		value = string([]rune(value)[:48])
	}
	return value
}

func trimBackgroundActionTail(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	actionMarkers := []string{
		"玩耍", "玩着", "奔跑", "跑着", "跳跃", "跳着", "漫步", "散步", "行走",
		"站着", "坐着", "躺着", "飞着", "漂浮", "看着", "读着", "睡着",
		"playing", "running", "jumping", "walking", "standing", "sitting", "floating", "reading", "sleeping",
	}
	for _, marker := range actionMarkers {
		idx := strings.Index(strings.ToLower(value), strings.ToLower(marker))
		if idx <= 0 {
			continue
		}
		trimmed := strings.TrimSpace(value[:idx])
		trimmed = strings.TrimRight(trimmed, "的在于和与及、")
		for _, suffix := range []string{"里面", "里", "中", "上", "旁边", "旁", "边", "前", "后", "内", "外"} {
			trimmed = strings.TrimSuffix(trimmed, suffix)
		}
		trimmed = strings.TrimSpace(trimmed)
		if trimmed != "" {
			return trimmed
		}
	}
	return value
}

func defaultObjectType(idx int) string {
	if idx == 0 {
		return "subject"
	}
	return fmt.Sprintf("subject_%d", idx+1)
}
