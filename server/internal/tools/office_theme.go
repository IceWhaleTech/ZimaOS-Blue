package tools

import (
	"fmt"
	"html"
	"strings"
	"sync"
)

// officeTheme defines a complete visual design system for document generation.
// Each theme includes color dominance rules (60-70% primary, 20-30% secondary, 5-10% accent),
// font pairings, and design rationale for content matching.
type officeTheme struct {
	Name          string
	Dark          bool
	Primary       string // 60-70% dominance - main identity
	PrimaryDark   string // shadows/deep accents
	PrimaryTint   string // backgrounds/highlights
	Secondary     string // 20-30% support - contrast
	Accent        string // 5-10% highlights - calls to action
	AccentTint    string // soft accent backgrounds
	Success       string
	SuccessTint   string
	Warning       string
	WarningTint   string
	Danger        string
	DangerTint    string
	Slate         string
	SlateTint     string
	Border        string
	Surface       string
	SurfaceAlt    string
	SurfaceMuted  string
	DisplayFont   string // headings - can be serif for personality
	BodyFont      string // body - clean sans for readability
	EastAsiaFont  string
	MonospaceFont string

	// NEW: Design rationale fields (Grapwork-inspired)
	Personality string   // e.g., "professional yet distinctive"
	UseCases    []string // e.g., ["executive_summary", "board_presentation"]
	Mood        string   // e.g., "trustworthy", "energetic", "calm"
}

type officeThemePreview struct {
	Name           string                      `json:"name"`
	Personality    string                      `json:"personality"`
	Mood           string                      `json:"mood"`
	UseCases       []string                    `json:"use_cases,omitempty"`
	Fonts          officeThemePreviewFonts     `json:"fonts"`
	Dominance      officeThemePreviewDominance `json:"dominance"`
	Swatches       []officeThemePreviewSwatch  `json:"swatches"`
	SampleTitle    string                      `json:"sample_title"`
	SampleSubtitle string                      `json:"sample_subtitle"`
	SampleCallout  string                      `json:"sample_callout"`
	HTML           string                      `json:"html"`
}

type officeThemePreviewFonts struct {
	Display   string `json:"display"`
	Body      string `json:"body"`
	Monospace string `json:"monospace"`
	EastAsia  string `json:"east_asia,omitempty"`
}

type officeThemePreviewDominance struct {
	Primary   float64 `json:"primary"`
	Secondary float64 `json:"secondary"`
	Accent    float64 `json:"accent"`
}

type officeThemePreviewSwatch struct {
	Role      string  `json:"role"`
	Hex       string  `json:"hex"`
	Dominance float64 `json:"dominance,omitempty"`
}

var (
	officeThemeCatalog     map[string]officeTheme
	officeThemeCatalogOnce sync.Once
)

func ensureOfficeThemeCatalog() {
	officeThemeCatalogOnce.Do(func() {
		officeThemeCatalog = map[string]officeTheme{
			// === EXISTING THEMES (preserved) ===
			"analysis": {
				Name: "analysis", Primary: "#2563EB", PrimaryDark: "#153E75", PrimaryTint: "#E7F0FF",
				Secondary: "#5B708B", Accent: "#06B6D4", AccentTint: "#DDF8FF",
				Success: "#16A34A", SuccessTint: "#DCFCE7", Warning: "#D97706", WarningTint: "#FFEDD5",
				Danger: "#DC2626", DangerTint: "#FEE2E2", Slate: "#3F4F63", SlateTint: "#D8E1EE",
				Border: "#D9E2F0", Surface: "#FFFFFF", SurfaceAlt: "#F4F7FB", SurfaceMuted: "#EEF3FA",
				DisplayFont: "Aptos Display", BodyFont: "Aptos", EastAsiaFont: "PingFang SC", MonospaceFont: "Aptos Mono",
				Personality: "analytical and clear",
				UseCases:    []string{"technical_review", "data_analysis", "research_report"},
				Mood:        "focused",
			},
			"ui_review": {
				Name: "ui_review", Primary: "#0F172A", PrimaryDark: "#0B1120", PrimaryTint: "#E0F2FE",
				Secondary: "#475569", Accent: "#8B5CF6", AccentTint: "#EEE7FF",
				Success: "#16A34A", SuccessTint: "#DCFCE7", Warning: "#D97706", WarningTint: "#FFEDD5",
				Danger: "#DC2626", DangerTint: "#FEE2E2", Slate: "#475569", SlateTint: "#DDE6F1",
				Border: "#C9D5E4", Surface: "#FFFFFF", SurfaceAlt: "#F7FAFC", SurfaceMuted: "#EFF4FA",
				DisplayFont: "Aptos Display", BodyFont: "Aptos", EastAsiaFont: "PingFang SC", MonospaceFont: "Aptos Mono",
				Personality: "precise and evaluative",
				UseCases:    []string{"design_review", "usability_audit", "accessibility_report"},
				Mood:        "critical",
			},
			"executive": {
				Name: "executive", Primary: "#123B5D", PrimaryDark: "#0B2236", PrimaryTint: "#E8F1F7",
				Secondary: "#60758A", Accent: "#10B981", AccentTint: "#DDF7EF",
				Success: "#15803D", SuccessTint: "#DCFCE7", Warning: "#B45309", WarningTint: "#FDE6C8",
				Danger: "#B91C1C", DangerTint: "#FECACA", Slate: "#31475C", SlateTint: "#DAE6F0",
				Border: "#D6E0EA", Surface: "#FFFFFF", SurfaceAlt: "#F5F8FB", SurfaceMuted: "#EEF3F7",
				DisplayFont: "Aptos Display", BodyFont: "Aptos", EastAsiaFont: "PingFang SC", MonospaceFont: "Aptos Mono",
				Personality: "authoritative and refined",
				UseCases:    []string{"board_presentation", "executive_summary", "strategic_review"},
				Mood:        "confident",
			},
			"clean": {
				Name: "clean", Primary: "#1F2937", PrimaryDark: "#0F172A", PrimaryTint: "#F3F4F6",
				Secondary: "#6B7280", Accent: "#2563EB", AccentTint: "#DBEAFE",
				Success: "#15803D", SuccessTint: "#DCFCE7", Warning: "#B45309", WarningTint: "#FFEDD5",
				Danger: "#B91C1C", DangerTint: "#FEE2E2", Slate: "#4B5563", SlateTint: "#E5E7EB",
				Border: "#D1D5DB", Surface: "#FFFFFF", SurfaceAlt: "#F9FAFB", SurfaceMuted: "#F3F4F6",
				DisplayFont: "Aptos Display", BodyFont: "Aptos", EastAsiaFont: "PingFang SC", MonospaceFont: "Aptos Mono",
				Personality: "neutral and versatile",
				UseCases:    []string{"general_document", "internal_memo", "neutral_report"},
				Mood:        "balanced",
			},

			// === NEW THEMES (Grapwork-inspired) ===
			"midnight": {
				Name: "midnight", Dark: true,
				Primary: "#242C38", PrimaryDark: "#F5F3EE", PrimaryTint: "#11141A",
				Secondary: "#8A93A2", Accent: "#4D7CFE", AccentTint: "#101C3D",
				Success: "#5CC38B", SuccessTint: "#102017", Warning: "#C89A45", WarningTint: "#2A2114",
				Danger: "#E27B7B", DangerTint: "#2C1818", Slate: "#D8DCE5", SlateTint: "#BEC6D2",
				Border: "#2B313C", Surface: "#0B0D12", SurfaceAlt: "#05070B", SurfaceMuted: "#11151B",
				DisplayFont: "Helvetica Neue", BodyFont: "Helvetica", EastAsiaFont: "PingFang SC", MonospaceFont: "Menlo",
				Personality: "professional yet distinctive",
				UseCases:    []string{"executive_summary", "board_presentation", "annual_report", "investor_deck"},
				Mood:        "trustworthy",
			},
			"terracotta": {
				Name: "terracotta", Primary: "#B85C38", PrimaryDark: "#6B341F", PrimaryTint: "#FBEDE7",
				Secondary: "#B08968", Accent: "#1F8A70", AccentTint: "#DFF4EE",
				Success: "#15803D", SuccessTint: "#DCFCE7", Warning: "#B45309", WarningTint: "#FFEDD5",
				Danger: "#B91C1C", DangerTint: "#FEE2E2", Slate: "#5F5249", SlateTint: "#EAE1DA",
				Border: "#E3D6CE", Surface: "#FFF9F5", SurfaceAlt: "#FFF4EE", SurfaceMuted: "#F7EFE8",
				DisplayFont: "Palatino Linotype", BodyFont: "Inter", EastAsiaFont: "PingFang SC", MonospaceFont: "Fira Code",
				Personality: "approachable and creative",
				UseCases:    []string{"creative_brief", "design_review", "product_roadmap", "brand_guidelines"},
				Mood:        "warm",
			},
			"forest": {
				Name: "forest", Primary: "#1F5F47", PrimaryDark: "#123829", PrimaryTint: "#E9F3EE",
				Secondary: "#7D9488", Accent: "#D4A63A", AccentTint: "#FBF3DB",
				Success: "#16A34A", SuccessTint: "#DCFCE7", Warning: "#A16207", WarningTint: "#FEF3C7",
				Danger: "#B91C1C", DangerTint: "#FEE2E2", Slate: "#42554A", SlateTint: "#E4ECE6",
				Border: "#D4DED8", Surface: "#F9FCFA", SurfaceAlt: "#F3F8F5", SurfaceMuted: "#ECF3EE",
				DisplayFont: "Times New Roman", BodyFont: "Open Sans", EastAsiaFont: "PingFang SC", MonospaceFont: "Cascadia Code",
				Personality: "grounded and trustworthy",
				UseCases:    []string{"sustainability_report", "environmental_review", "wellness_document", "csr_report"},
				Mood:        "natural",
			},
			"coral": {
				Name: "coral", Primary: "#FF6B6B", PrimaryDark: "#B83F56", PrimaryTint: "#FFF0F2",
				Secondary: "#556987", Accent: "#16C7B7", AccentTint: "#E0FCF9",
				Success: "#16A34A", SuccessTint: "#DCFCE7", Warning: "#D97706", WarningTint: "#FFEDD5",
				Danger: "#EF4444", DangerTint: "#FEE2E2", Slate: "#4C596C", SlateTint: "#DDE6EF",
				Border: "#D9E2EC", Surface: "#FFFFFF", SurfaceAlt: "#FFF7F8", SurfaceMuted: "#FFF1F3",
				DisplayFont: "Poppins", BodyFont: "Nunito", EastAsiaFont: "PingFang SC", MonospaceFont: "SF Mono",
				Personality: "energetic and modern",
				UseCases:    []string{"startup_pitch", "marketing_deck", "product_launch", "growth_report"},
				Mood:        "energetic",
			},
		}
	})
}

// resolveOfficeTheme selects the best theme for a document based on explicit name or content hints.
// Grapwork-inspired: matches purpose keywords to theme personality and use cases.
func resolveOfficeTheme(name, styleHint string) officeTheme {
	ensureOfficeThemeCatalog()

	key := strings.ToLower(strings.TrimSpace(name))
	if theme, ok := officeThemeCatalog[key]; ok {
		return theme
	}

	hint := strings.ToLower(strings.TrimSpace(styleHint))
	if hint == "" {
		// Default to a more polished preset when users omit theme selection entirely.
		return officeThemeCatalog["midnight"]
	}

	// NEW: Grapwork-inspired purpose-based theme matching
	switch {
	// Sustainability/eco contexts → forest (natural, trustworthy)
	case officeContainsAny(hint, "sustainability", "environment", "green", "eco", "carbon", "climate",
		"wellness", "health", "csr", "esg",
		"可持续", "环境", "绿色", "碳", "气候", "健康", "社会责任"):
		return officeThemeCatalog["forest"]

	// Creative/design contexts → terracotta (warm, creative)
	case officeContainsAny(hint, "creative", "design", "brand", "visual", "aesthetic", "illustration",
		"创意", "设计", "品牌", "视觉", "艺术"):
		return officeThemeCatalog["terracotta"]

	// Executive/board contexts → midnight (professional, distinctive)
	case officeContainsAny(hint, "board", "annual", "investor", "stakeholder", "governance",
		"高管", "董事会", "年报", "投资者", "股东"):
		return officeThemeCatalog["midnight"]

	// Original matching preserved
	case officeContainsAny(hint,
		"ui", "ux", "review", "audit", "score", "评级", "评估", "评审", "审查", "界面", "可用性", "体验",
	):
		return officeThemeCatalog["ui_review"]
	case officeContainsAny(hint,
		"executive", "briefing", "formal", "management", "汇报", "正式", "商业",
	):
		return officeThemeCatalog["executive"]
	case officeContainsAny(hint,
		"clean", "minimal", "neutral", "simple", "极简", "简洁", "中性",
	):
		return officeThemeCatalog["clean"]

	// Energy/startup contexts → coral (energetic, modern)
	case officeContainsAny(hint, "pitch", "launch", "startup", "growth", "marketing", "sales",
		"融资", "路演", "发布", "增长", "营销"):
		return officeThemeCatalog["coral"]
	default:
		return officeThemeCatalog["analysis"]
	}
}

// GetThemeByMood returns themes matching a specific mood for content-based selection.
func GetThemeByMood(mood string) []officeTheme {
	ensureOfficeThemeCatalog()
	var matches []officeTheme
	mood = strings.ToLower(mood)
	for _, theme := range officeThemeCatalog {
		if strings.ToLower(theme.Mood) == mood {
			matches = append(matches, theme)
		}
	}
	return matches
}

// GetThemeUseCases returns the use case list for a theme.
func GetThemeUseCases(name string) []string {
	ensureOfficeThemeCatalog()
	key := strings.ToLower(strings.TrimSpace(name))
	if theme, ok := officeThemeCatalog[key]; ok {
		return theme.UseCases
	}
	return nil
}

// ListThemes returns all available theme names.
func ListThemes() []string {
	ensureOfficeThemeCatalog()
	names := make([]string, 0, len(officeThemeCatalog))
	for name := range officeThemeCatalog {
		names = append(names, name)
	}
	sortStringsCaseInsensitive(names)
	return names
}

// GetThemePreview returns a renderable preview payload for downstream UI and QA flows.
func GetThemePreview(name string) (officeThemePreview, error) {
	theme, err := officeThemeByName(name)
	if err != nil {
		return officeThemePreview{}, err
	}
	return buildOfficeThemePreview(theme), nil
}

func buildOfficeThemePreview(theme officeTheme) officeThemePreview {
	preview := officeThemePreview{
		Name:        theme.Name,
		Personality: theme.Personality,
		Mood:        theme.Mood,
		UseCases:    append([]string(nil), theme.UseCases...),
		Fonts: officeThemePreviewFonts{
			Display:   theme.DisplayFont,
			Body:      theme.BodyFont,
			Monospace: theme.MonospaceFont,
			EastAsia:  theme.EastAsiaFont,
		},
		Dominance: officeThemePreviewDominance{
			Primary:   theme.PrimaryDominance(),
			Secondary: theme.SecondaryDominance(),
			Accent:    theme.AccentDominance(),
		},
		Swatches: []officeThemePreviewSwatch{
			{Role: "primary", Hex: theme.Primary, Dominance: theme.PrimaryDominance()},
			{Role: "secondary", Hex: theme.Secondary, Dominance: theme.SecondaryDominance()},
			{Role: "accent", Hex: theme.Accent, Dominance: theme.AccentDominance()},
			{Role: "surface", Hex: theme.Surface},
			{Role: "border", Hex: theme.Border},
		},
		SampleTitle:    officeThemePreviewTitle(theme),
		SampleSubtitle: officeThemePreviewSubtitle(theme),
		SampleCallout:  "Accent-driven highlights keep the most important takeaways visible without overwhelming the page.",
	}
	preview.HTML = officeThemePreviewHTML(theme, preview)
	return preview
}

func officeThemePreviewTitle(theme officeTheme) string {
	if len(theme.UseCases) > 0 {
		return officeHumanizeThemeToken(theme.UseCases[0])
	}
	return officeHumanizeThemeToken(theme.Name) + " Preview"
}

func officeThemePreviewSubtitle(theme officeTheme) string {
	personality := strings.TrimSpace(theme.Personality)
	mood := strings.TrimSpace(theme.Mood)
	switch {
	case personality != "" && mood != "":
		return officeHumanizeThemeSentence(personality) + " for " + mood + " narratives."
	case personality != "":
		return officeHumanizeThemeSentence(personality) + "."
	case mood != "":
		return "Built for " + mood + " narratives."
	default:
		return "A curated office document theme preview."
	}
}

func officeThemePreviewHTML(theme officeTheme, preview officeThemePreview) string {
	title := html.EscapeString(preview.SampleTitle)
	subtitle := html.EscapeString(preview.SampleSubtitle)
	callout := html.EscapeString(preview.SampleCallout)
	personality := html.EscapeString(preview.Personality)
	mood := html.EscapeString(preview.Mood)
	displayFont := html.EscapeString(theme.DisplayFont)
	bodyFont := html.EscapeString(theme.BodyFont)
	monoFont := html.EscapeString(theme.MonospaceFont)

	var swatches strings.Builder
	for _, swatch := range preview.Swatches {
		label := html.EscapeString(officeHumanizeThemeToken(swatch.Role))
		swatches.WriteString(`<div style="min-width:84px">`)
		swatches.WriteString(`<div style="height:16px;border-radius:999px;background:` + html.EscapeString(swatch.Hex) + `;border:1px solid ` + html.EscapeString(theme.Border) + `"></div>`)
		swatches.WriteString(`<div style="margin-top:6px;font-size:11px;color:` + html.EscapeString(theme.Slate) + `">` + label + `</div>`)
		swatches.WriteString(`</div>`)
	}

	var sb strings.Builder
	sb.WriteString(`<div style="max-width:560px;border:1px solid ` + html.EscapeString(theme.Border) + `;border-radius:18px;overflow:hidden;background:` + html.EscapeString(theme.Surface) + `;font-family:` + bodyFont + `;color:` + html.EscapeString(theme.Slate) + `">`)
	sb.WriteString(`<div style="padding:22px 24px;background:` + html.EscapeString(theme.Primary) + `;color:#FFFFFF">`)
	sb.WriteString(`<div style="font-size:12px;letter-spacing:0.12em;text-transform:uppercase;opacity:0.82">` + html.EscapeString(theme.Name) + `</div>`)
	sb.WriteString(`<div style="margin-top:8px;font-family:` + displayFont + `;font-size:28px;line-height:1.2;font-weight:700">` + title + `</div>`)
	sb.WriteString(`<div style="margin-top:10px;font-size:14px;line-height:1.5;opacity:0.92">` + subtitle + `</div>`)
	sb.WriteString(`</div>`)
	sb.WriteString(`<div style="padding:18px 24px;background:` + html.EscapeString(theme.PrimaryTint) + `;border-top:4px solid ` + html.EscapeString(theme.Accent) + `">`)
	sb.WriteString(`<div style="font-size:13px;font-weight:600;color:` + html.EscapeString(theme.PrimaryDark) + `">` + personality + `</div>`)
	sb.WriteString(`<div style="margin-top:6px;font-size:14px;line-height:1.6;color:` + html.EscapeString(theme.Slate) + `">` + callout + `</div>`)
	if mood != "" {
		sb.WriteString(`<div style="margin-top:10px;font-size:12px;color:` + html.EscapeString(theme.Secondary) + `">Mood: ` + mood + `</div>`)
	}
	sb.WriteString(`</div>`)
	sb.WriteString(`<div style="padding:18px 24px">`)
	sb.WriteString(`<div style="display:flex;gap:10px;flex-wrap:wrap">` + swatches.String() + `</div>`)
	sb.WriteString(`<div style="margin-top:16px;font-size:12px;color:` + html.EscapeString(theme.Slate) + `">Display: <span style="font-family:` + displayFont + `">` + displayFont + `</span> · Body: <span style="font-family:` + bodyFont + `">` + bodyFont + `</span> · Mono: <span style="font-family:` + monoFont + `">` + monoFont + `</span></div>`)
	sb.WriteString(`</div></div>`)
	return sb.String()
}

func officeHumanizeThemeToken(value string) string {
	value = strings.ReplaceAll(strings.TrimSpace(value), "_", " ")
	value = strings.ReplaceAll(value, "-", " ")
	parts := strings.Fields(value)
	for i, part := range parts {
		runes := []rune(strings.ToLower(part))
		if len(runes) == 0 {
			continue
		}
		runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
		parts[i] = string(runes)
	}
	return strings.Join(parts, " ")
}

func officeHumanizeThemeSentence(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) == 0 {
		return ""
	}
	runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
	return string(runes)
}

// Color dominance helpers for validation
func (t officeTheme) PrimaryDominance() float64   { return 0.65 } // 60-70%
func (t officeTheme) SecondaryDominance() float64 { return 0.25 } // 20-30%
func (t officeTheme) AccentDominance() float64    { return 0.10 } // 5-10%

func officeARGB(raw string) string {
	hex := officeHex(raw)
	if len(hex) == 6 {
		return "FF" + hex
	}
	if len(hex) == 8 {
		return hex
	}
	return "FF000000"
}

func officeWordHex(raw string) string {
	hex := officeHex(raw)
	if len(hex) == 8 {
		return hex[2:]
	}
	if len(hex) == 6 {
		return hex
	}
	return "000000"
}

func officeHex(raw string) string {
	trimmed := strings.TrimSpace(strings.TrimPrefix(raw, "#"))
	return strings.ToUpper(trimmed)
}

func officeThemeByName(name string) (officeTheme, error) {
	ensureOfficeThemeCatalog()

	key := strings.ToLower(strings.TrimSpace(name))
	theme, ok := officeThemeCatalog[key]
	if !ok {
		return officeTheme{}, fmt.Errorf("unknown office theme: %s", name)
	}
	return theme, nil
}
