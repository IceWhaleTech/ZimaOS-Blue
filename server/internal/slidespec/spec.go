package slidespec

import (
	"regexp"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

const (
	DefaultAspectRatio = "16:9"

	RenderModePoster = "poster"
	RenderModeSlide  = "slide"

	StylePresetBananaSlides = "banana_slides"
	StylePresetNanoSlides   = "nano_slides"

	TemplatePoster   = "poster"
	TemplateCover    = "cover"
	TemplateSplit    = "split"
	TemplateTextOnly = "text_only"
	TemplateAgenda   = "agenda"
	TemplateMetrics  = "metrics"
)

type Input struct {
	Prompt         string
	Description    string
	StylePreset    string
	QualityProfile string
	Theme          string
	AspectRatio    string
	Source         string
	Lang           string
}

type Brief struct {
	RenderMode  string
	StylePreset string
	TemplateID  string
	Title       string
	Subtitle    string
	Bullets     []string
	Theme       string
	AspectRatio string
	VisualQuery string
}

var (
	bulletPrefixREOnce     sync.Once
	bulletPrefixRE         *regexp.Regexp
	metricValueTokenREOnce sync.Once
	metricValueTokenRE     *regexp.Regexp
)

func ensureBulletPrefixRE() {
	bulletPrefixREOnce.Do(func() {
		bulletPrefixRE = regexp.MustCompile(`^\s*(?:[-*•]+|\d+[.)]|[一二三四五六七八九十]+[、.])\s*`)
	})
}

func ensureMetricValueTokenRE() {
	metricValueTokenREOnce.Do(func() {
		metricValueTokenRE = regexp.MustCompile(`(?i)[+\-]?\d+(?:[.,]\d+)?\s*(?:%|x|倍|ms|s|sec|secs|min|mins|h|hr|hrs|天|日|周|月|年|k|m|b|pt|pts|万|亿|家|人|个|次)?`)
	})
}

func Build(input Input) Brief {
	prompt := cleanSpace(firstNonEmpty(input.Description, input.Prompt))
	stylePreset := NormalizeStylePreset(input.StylePreset)
	brief := Brief{
		RenderMode:  RenderModePoster,
		StylePreset: stylePreset,
		TemplateID:  TemplatePoster,
		Theme:       cleanSpace(input.Theme),
		AspectRatio: normalizeAspectRatio(input.AspectRatio),
		VisualQuery: prompt,
	}
	if !isSlideIntent(prompt, input) {
		return brief
	}

	title, subtitle, bullets, narrative := extractStructuredCopy(prompt)
	if title == "" {
		title, subtitle = deriveTitleSubtitle(prompt, narrative, subtitle)
	}
	if subtitle == "" {
		subtitle = defaultSubtitleFor(title)
	}

	title = trimToRunes(cleanTitle(title), 64)
	subtitle = trimToRunes(cleanLine(subtitle), 96)
	bullets = normalizeBullets(bullets, 3)

	brief.RenderMode = RenderModeSlide
	brief.Title = title
	brief.Subtitle = subtitle
	brief.Bullets = bullets
	brief.VisualQuery = buildVisualQuery(prompt, title, subtitle, bullets, brief.Theme)
	brief.TemplateID = recommendTemplate(brief)
	return brief
}

func DecideTemplate(brief Brief, hasVisual bool) string {
	if brief.RenderMode != RenderModeSlide {
		return TemplatePoster
	}
	recommended := recommendTemplate(brief)
	switch recommended {
	case TemplateAgenda, TemplateMetrics:
		return recommended
	}
	if !hasVisual {
		return TemplateTextOnly
	}
	if recommended == TemplateSplit {
		return TemplateSplit
	}
	return TemplateCover
}

func WrapPublicSpacePrompt(brief Brief, rawPrompt string) string {
	if brief.RenderMode != RenderModeSlide {
		return cleanSpace(rawPrompt)
	}

	lines := []string{
		"Create a presentation-ready PPT slide visual in a " + slideStyleDescriptor(brief.StylePreset) + " style.",
		"- Canvas: " + firstNonEmpty(brief.AspectRatio, DefaultAspectRatio),
		"- Use clear hierarchy, aligned layout, generous negative space, and polished presentation aesthetics.",
		"- Keep on-image copy concise and legible. No dense paragraphs, no UI chrome, no browser frames, no watermarks.",
		"- Avoid text overflow, cluttered poster layouts, fake dashboards, and awkward crops.",
	}
	lines = append(lines, slideStylePromptLines(brief.StylePreset)...)
	lines = append(lines, slideTemplatePromptLines(brief.TemplateID)...)
	if brief.Title != "" {
		lines = append(lines, "- Title: "+brief.Title)
	}
	if brief.Subtitle != "" {
		lines = append(lines, "- Subtitle: "+brief.Subtitle)
	}
	if len(brief.Bullets) > 0 {
		lines = append(lines, "- Key points:")
		for _, bullet := range brief.Bullets {
			lines = append(lines, "  - "+bullet)
		}
	}
	if brief.Theme != "" {
		lines = append(lines, "- Theme: "+brief.Theme)
	}
	if brief.VisualQuery != "" {
		lines = append(lines, "- Visual direction: "+brief.VisualQuery)
	}
	if trimmed := cleanSpace(rawPrompt); trimmed != "" {
		lines = append(lines, "- Original intent: "+trimmed)
	}
	return strings.Join(lines, "\n")
}

func BuildPublicSpaceNegativePrompt(existing string) string {
	items := []string{
		"browser frame",
		"webpage screenshot",
		"UI chrome",
		"dense poster text",
		"paragraph text",
		"text overflow",
		"misaligned layout",
		"watermark",
		"logo wall",
		"clutter",
	}
	if trimmed := cleanSpace(existing); trimmed != "" {
		items = append([]string{trimmed}, items...)
	}
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = cleanSpace(item)
		if item == "" {
			continue
		}
		key := strings.ToLower(item)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return strings.Join(out, ", ")
}

func recommendTemplate(brief Brief) string {
	if brief.RenderMode != RenderModeSlide {
		return TemplatePoster
	}
	if shouldUseMetricsTemplate(brief) {
		return TemplateMetrics
	}
	if shouldUseAgendaTemplate(brief) {
		return TemplateAgenda
	}
	if len(brief.Bullets) > 0 || brief.Subtitle != "" {
		return TemplateSplit
	}
	return TemplateCover
}

func shouldUseAgendaTemplate(brief Brief) bool {
	if len(brief.Bullets) < 2 {
		return false
	}
	signalText := strings.Join([]string{brief.Title, brief.Subtitle, brief.VisualQuery}, " ")
	return containsAnyFold(signalText, []string{
		"agenda",
		"outline",
		"roadmap",
		"timeline",
		"milestone",
		"phase",
		"plan",
		"steps",
		"目录",
		"议程",
		"路线图",
		"时间线",
		"里程碑",
		"阶段",
		"步骤",
		"计划",
	})
}

func shouldUseMetricsTemplate(brief Brief) bool {
	if len(brief.Bullets) < 2 {
		return false
	}
	metricBullets := 0
	for _, bullet := range brief.Bullets {
		if looksLikeMetricBullet(bullet) {
			metricBullets++
		}
	}
	if metricBullets >= 2 {
		return true
	}
	signalText := strings.Join([]string{brief.Title, brief.Subtitle, brief.VisualQuery}, " ")
	return metricBullets >= 1 && containsAnyFold(signalText, []string{
		"metric",
		"metrics",
		"kpi",
		"kpis",
		"dashboard",
		"performance",
		"results",
		"growth",
		"roi",
		"cac",
		"arr",
		"gmv",
		"nps",
		"dau",
		"mau",
		"arpu",
		"ltv",
		"指标",
		"数据",
		"增长",
		"转化",
		"效率",
		"成本",
		"营收",
		"留存",
		"利润",
	})
}

func looksLikeMetricBullet(value string) bool {
	cleaned := strings.ToLower(cleanLine(value))
	if cleaned == "" {
		return false
	}
	if strings.Contains(cleaned, "%") {
		return true
	}
	if containsAnyFold(cleaned, []string{
		"roi",
		"cac",
		"arr",
		"gmv",
		"nps",
		"dau",
		"mau",
		"arpu",
		"ltv",
		"ms",
		"sec",
		"min",
		"小时",
		"分钟",
		"倍",
		"万",
		"亿",
	}) && strings.ContainsAny(cleaned, "0123456789") {
		return true
	}
	ensureMetricValueTokenRE()
	if metricValueTokenRE.MatchString(cleaned) && containsAnyFold(cleaned, []string{
		"growth",
		"conversion",
		"revenue",
		"margin",
		"profit",
		"retention",
		"latency",
		"uptime",
		"cost",
		"效率",
		"增长",
		"转化",
		"营收",
		"留存",
		"成本",
		"利润",
		"时延",
	}) {
		return true
	}
	return false
}

func isSlideIntent(prompt string, input Input) bool {
	if prompt == "" {
		return false
	}
	if isSlideStylePreset(input.StylePreset) {
		return true
	}
	if strings.EqualFold(cleanSpace(input.QualityProfile), "ppt") {
		return true
	}
	if containsAnyFold(cleanSpace(input.Source), slideSignalWords()) {
		return true
	}
	return containsAnyFold(prompt, slideSignalWords())
}

func slideSignalWords() []string {
	return []string{
		StylePresetBananaSlides,
		StylePresetNanoSlides,
		"bananaslides",
		"banana slides",
		"banana-slides",
		"nanoslides",
		"nano slides",
		"nano-slides",
		"ppt",
		"slide",
		"slides",
		"deck",
		"presentation",
		"material",
		"report slide",
		"幻灯片",
		"汇报",
		"封面",
		"目录",
		"演示文稿",
		"演示稿",
		"路演",
	}
}

func NormalizeStylePreset(value string) string {
	switch strings.ToLower(cleanSpace(value)) {
	case "", "default":
		return ""
	case StylePresetBananaSlides, "bananaslides", "banana slides", "banana-slides":
		return StylePresetBananaSlides
	case StylePresetNanoSlides, "nanoslides", "nano slides", "nano-slides":
		return StylePresetNanoSlides
	default:
		return cleanSpace(value)
	}
}

func IsNanoSlidesPreset(value string) bool {
	return NormalizeStylePreset(value) == StylePresetNanoSlides
}

func IsBananaSlidesPreset(value string) bool {
	return NormalizeStylePreset(value) == StylePresetBananaSlides
}

func StylePresetDisplayName(value string) string {
	switch NormalizeStylePreset(value) {
	case StylePresetNanoSlides:
		return "nanoslides"
	case StylePresetBananaSlides:
		return "bananaslides"
	default:
		return cleanSpace(value)
	}
}

func isSlideStylePreset(value string) bool {
	return IsBananaSlidesPreset(value) || IsNanoSlidesPreset(value)
}

func slideStyleDescriptor(stylePreset string) string {
	switch name := StylePresetDisplayName(stylePreset); name {
	case "nanoslides", "bananaslides":
		return name + "-inspired"
	default:
		return "presentation-system"
	}
}

func slideStylePromptLines(stylePreset string) []string {
	switch NormalizeStylePreset(stylePreset) {
	case StylePresetNanoSlides:
		return []string{
			"- Favor a first-class nanoslides visual system: oversized editorial headline, premium restraint, glass information cards, and confident negative space.",
			"- Use a calm dark-luxury palette with one precise accent color instead of playful rainbow gradients.",
			"- Keep the composition boardroom-ready: sophisticated, sharp, and presentation-native rather than poster-like or illustrative.",
		}
	case StylePresetBananaSlides:
		return []string{
			"- Favor the bananaslides system: bold headline scale, crisp composition, polished gradients, and clean overlay-safe zones.",
			"- Keep the aesthetic expressive but controlled: premium slide design, not a noisy social poster.",
		}
	default:
		return nil
	}
}

func slideTemplatePromptLines(templateID string) []string {
	switch cleanSpace(templateID) {
	case TemplateCover:
		return []string{
			"- Layout motif: oversized headline block on the left with one cinematic hero visual on the right.",
		}
	case TemplateSplit:
		return []string{
			"- Layout motif: structured narrative column on the left and a framed visual panel on the right.",
		}
	case TemplateTextOnly:
		return []string{
			"- Layout motif: executive-summary board with a strong headline stack and a grid of concise insight cards.",
		}
	case TemplateAgenda:
		return []string{
			"- Layout motif: clean agenda page with a top headline band and a horizontal sequence of numbered step cards.",
		}
	case TemplateMetrics:
		return []string{
			"- Layout motif: KPI slide with a bold headline and a row of large metric cards showing numbers first, labels second.",
		}
	default:
		return nil
	}
}

func extractStructuredCopy(prompt string) (string, string, []string, []string) {
	normalized := strings.ReplaceAll(strings.ReplaceAll(prompt, "\r\n", "\n"), "\r", "\n")
	normalized = normalizeSectionBreaks(normalized)
	lines := strings.Split(normalized, "\n")

	var title string
	var subtitle string
	bullets := make([]string, 0, 3)
	narrative := make([]string, 0, len(lines))
	for _, rawLine := range lines {
		line := cleanLine(rawLine)
		if line == "" {
			continue
		}
		if value, ok := stripLabelValue(line, []string{"title", "标题", "主题"}); ok {
			title = cleanTitle(value)
			continue
		}
		if value, ok := stripLabelValue(line, []string{"subtitle", "副标题"}); ok {
			subtitle = cleanLine(value)
			continue
		}
		if value, ok := stripLabelValue(line, []string{"bullets", "bullet", "points", "key points", "outline", "agenda", "要点", "重点", "目录"}); ok {
			bullets = append(bullets, splitBulletText(value)...)
			continue
		}
		if bullet := cleanBulletLine(line); bullet != "" {
			bullets = append(bullets, bullet)
			continue
		}
		narrative = append(narrative, line)
	}
	return title, subtitle, bullets, narrative
}

func normalizeSectionBreaks(value string) string {
	if value == "" {
		return ""
	}
	labels := []string{"title", "subtitle", "bullets", "bullet", "points", "key points", "outline", "agenda", "标题", "副标题", "要点", "重点", "目录"}
	var out strings.Builder
	runes := []rune(value)
	for idx, r := range runes {
		out.WriteRune(r)
		if !strings.ContainsRune(";；|,，", r) {
			continue
		}
		rest := strings.TrimLeftFunc(string(runes[idx+1:]), unicode.IsSpace)
		for _, label := range labels {
			if strings.HasPrefix(strings.ToLower(rest), strings.ToLower(label)) {
				out.WriteRune('\n')
				break
			}
		}
	}
	return out.String()
}

func deriveTitleSubtitle(prompt string, narrative []string, subtitle string) (string, string) {
	lines := make([]string, 0, len(narrative))
	for _, line := range narrative {
		cleaned := stripSlideVerbs(line)
		if cleaned != "" {
			lines = append(lines, cleaned)
		}
	}
	if len(lines) == 0 {
		lines = append(lines, stripSlideVerbs(prompt))
	}
	title := cleanTitle(firstNonEmpty(lines...))
	if subtitle == "" && len(lines) > 1 {
		subtitle = cleanLine(lines[1])
	}
	if subtitle == "" {
		splitTitle, splitSubtitle := splitLongTitle(title)
		if splitTitle != "" {
			title = splitTitle
		}
		if splitSubtitle != "" {
			subtitle = splitSubtitle
		}
	}
	return title, subtitle
}

func buildVisualQuery(prompt, title, subtitle string, bullets []string, theme string) string {
	candidates := []string{
		stripSlideVerbs(prompt),
		title,
		subtitle,
		theme,
	}
	for _, bullet := range bullets {
		candidates = append(candidates, bullet)
	}
	parts := make([]string, 0, len(candidates))
	seen := map[string]struct{}{}
	for _, item := range candidates {
		item = cleanLine(item)
		if item == "" {
			continue
		}
		key := strings.ToLower(item)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		parts = append(parts, item)
		if len(parts) == 3 {
			break
		}
	}
	return trimToRunes(strings.Join(parts, ", "), 160)
}

func normalizeBullets(items []string, limit int) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = cleanBulletLine(item)
		if item == "" {
			continue
		}
		key := strings.ToLower(item)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimToRunes(item, 72))
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func splitBulletText(value string) []string {
	value = cleanLine(value)
	if value == "" {
		return nil
	}
	replacer := strings.NewReplacer("\n", ";", "；", ";", "•", ";", "|", ";", "、", ";")
	normalized := replacer.Replace(value)
	if strings.Count(normalized, ",") > 0 && strings.Count(normalized, ";") == 0 {
		normalized = strings.ReplaceAll(normalized, ",", ";")
		normalized = strings.ReplaceAll(normalized, "，", ";")
	}
	rawParts := strings.Split(normalized, ";")
	out := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		cleaned := cleanBulletLine(part)
		if cleaned != "" {
			out = append(out, cleaned)
		}
	}
	return out
}

func splitLongTitle(title string) (string, string) {
	title = cleanTitle(title)
	if title == "" {
		return "", ""
	}
	for _, sep := range []string{" - ", ": ", " | ", " -", "："} {
		if idx := strings.Index(title, sep); idx > 0 && idx < len(title)-len(sep) {
			return cleanTitle(title[:idx]), cleanLine(title[idx+len(sep):])
		}
	}
	words := strings.Fields(title)
	if len(words) >= 7 {
		return strings.Join(words[:5], " "), strings.Join(words[5:], " ")
	}
	if utf8.RuneCountInString(title) > 18 && containsCJK(title) {
		runes := []rune(title)
		mid := len(runes) / 2
		return strings.TrimSpace(string(runes[:mid])), strings.TrimSpace(string(runes[mid:]))
	}
	return title, ""
}

func stripSlideVerbs(value string) string {
	value = cleanLine(value)
	if value == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"make me", "",
		"make a", "",
		"create a", "",
		"create", "",
		"generate", "",
		"design", "",
		"draw", "",
		"做一页", "",
		"做个", "",
		"做一张", "",
		"帮我做", "",
		"帮我生成", "",
		"PPT", "",
		"ppt", "",
		"slide", "",
		"slides", "",
		"deck", "",
		"presentation", "",
		"幻灯片", "",
		"汇报", "",
		"封面", "",
	)
	return cleanLine(replacer.Replace(value))
}

func stripLabelValue(line string, labels []string) (string, bool) {
	trimmed := cleanLine(line)
	if trimmed == "" {
		return "", false
	}
	for _, label := range labels {
		if len(trimmed) < len(label)+1 {
			continue
		}
		if !strings.EqualFold(trimmed[:len(label)], label) {
			continue
		}
		rest := strings.TrimSpace(trimmed[len(label):])
		if strings.HasPrefix(rest, ":") {
			return cleanLine(strings.TrimSpace(rest[1:])), true
		}
		if strings.HasPrefix(rest, "：") {
			return cleanLine(strings.TrimSpace(strings.TrimPrefix(rest, "："))), true
		}
	}
	return "", false
}

func cleanBulletLine(line string) string {
	ensureBulletPrefixRE()
	line = bulletPrefixRE.ReplaceAllString(cleanLine(line), "")
	return trimToRunes(cleanLine(line), 72)
}

func cleanTitle(value string) string {
	value = cleanLine(value)
	value = strings.Trim(value, "-:|")
	return cleanLine(value)
}

func cleanLine(value string) string {
	value = strings.TrimSpace(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
	return strings.Trim(value, " ,;；|")
}

func cleanSpace(value string) string {
	return cleanLine(value)
}

func defaultSubtitleFor(title string) string {
	if title == "" {
		return ""
	}
	if containsCJK(title) {
		return "关键更新与亮点"
	}
	return "Key updates and highlights"
}

func normalizeAspectRatio(value string) string {
	value = cleanSpace(value)
	if value == "" {
		return DefaultAspectRatio
	}
	return value
}

func containsAnyFold(value string, items []string) bool {
	value = strings.ToLower(value)
	for _, item := range items {
		if strings.Contains(value, strings.ToLower(item)) {
			return true
		}
	}
	return false
}

func containsCJK(value string) bool {
	for _, r := range value {
		if unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul) {
			return true
		}
	}
	return false
}

func trimToRunes(value string, limit int) string {
	if limit <= 0 {
		return cleanLine(value)
	}
	value = cleanLine(value)
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return strings.TrimSpace(string(runes[:limit]))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := cleanSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
