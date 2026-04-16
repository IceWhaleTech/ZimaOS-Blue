package convert

import "strings"

type nativeMarkdownPPTXTheme struct {
	Name         string
	Primary      string
	PrimaryDark  string
	PrimaryTint  string
	Secondary    string
	Accent       string
	Success      string
	Warning      string
	Danger       string
	Slate        string
	Surface      string
	SurfaceAlt   string
	DisplayFont  string
	BodyFont     string
	EastAsiaFont string
	Monospace    string
}

func nativeMarkdownPPTXThemeCatalog() map[string]nativeMarkdownPPTXTheme {
	return map[string]nativeMarkdownPPTXTheme{
		"analysis": {
			Name: "analysis", Primary: "#1A6FC4", PrimaryDark: "#0D4A8A", PrimaryTint: "#EAF4FF",
			Secondary: "#64748B", Accent: "#0EA5E9", Success: "#166534", Warning: "#B45309", Danger: "#B91C1C",
			Slate: "#475569", Surface: "#FFFFFF", SurfaceAlt: "#F8FAFC",
			DisplayFont: "Aptos Display", BodyFont: "Aptos", EastAsiaFont: "PingFang SC", Monospace: "Aptos Mono",
		},
		"ui_review": {
			Name: "ui_review", Primary: "#0369A1", PrimaryDark: "#0F172A", PrimaryTint: "#E0F2FE",
			Secondary: "#64748B", Accent: "#6D28D9", Success: "#166534", Warning: "#B45309", Danger: "#B91C1C",
			Slate: "#475569", Surface: "#FFFFFF", SurfaceAlt: "#F8FAFC",
			DisplayFont: "Aptos Display", BodyFont: "Aptos", EastAsiaFont: "PingFang SC", Monospace: "Aptos Mono",
		},
		"executive": {
			Name: "executive", Primary: "#1E3A8A", PrimaryDark: "#172554", PrimaryTint: "#DBEAFE",
			Secondary: "#475569", Accent: "#0F766E", Success: "#166534", Warning: "#92400E", Danger: "#991B1B",
			Slate: "#334155", Surface: "#FFFFFF", SurfaceAlt: "#F8FAFC",
			DisplayFont: "Aptos Display", BodyFont: "Aptos", EastAsiaFont: "PingFang SC", Monospace: "Aptos Mono",
		},
		"clean": {
			Name: "clean", Primary: "#334155", PrimaryDark: "#0F172A", PrimaryTint: "#F1F5F9",
			Secondary: "#64748B", Accent: "#2563EB", Success: "#15803D", Warning: "#A16207", Danger: "#B91C1C",
			Slate: "#475569", Surface: "#FFFFFF", SurfaceAlt: "#F8FAFC",
			DisplayFont: "Aptos Display", BodyFont: "Aptos", EastAsiaFont: "PingFang SC", Monospace: "Aptos Mono",
		},
		"midnight": {
			Name: "midnight", Primary: "#1E3A5F", PrimaryDark: "#0F1D2F", PrimaryTint: "#E8EEF4",
			Secondary: "#4A5568", Accent: "#00D4AA", Success: "#059669", Warning: "#D97706", Danger: "#DC2626",
			Slate: "#334155", Surface: "#FFFFFF", SurfaceAlt: "#F8FAFC",
			DisplayFont: "Georgia", BodyFont: "Segoe UI", EastAsiaFont: "PingFang SC", Monospace: "Consolas",
		},
		"editorial": {
			Name: "editorial", Primary: "#0A0D14", PrimaryDark: "#11131A", PrimaryTint: "#FFF4D1",
			Secondary: "#00E5FF", Accent: "#FFB700", Success: "#2FA56B", Warning: "#FF8A3D", Danger: "#F25B45",
			Slate: "#727A8F", Surface: "#FFFFFF", SurfaceAlt: "#F5F6F8",
			DisplayFont: "Arial Black", BodyFont: "Helvetica", EastAsiaFont: "PingFang SC", Monospace: "Menlo",
		},
		"terracotta": {
			Name: "terracotta", Primary: "#C65D3B", PrimaryDark: "#8B4513", PrimaryTint: "#FDF2ED",
			Secondary: "#D4A574", Accent: "#2E8B57", Success: "#15803D", Warning: "#B45309", Danger: "#B91C1C",
			Slate: "#57534E", Surface: "#FFFCFA", SurfaceAlt: "#FFF8F5",
			DisplayFont: "Palatino Linotype", BodyFont: "Inter", EastAsiaFont: "PingFang SC", Monospace: "Fira Code",
		},
		"forest": {
			Name: "forest", Primary: "#2D5A3D", PrimaryDark: "#1A3A28", PrimaryTint: "#E8F0E9",
			Secondary: "#8B9D83", Accent: "#D4A017", Success: "#166534", Warning: "#A16207", Danger: "#B91C1C",
			Slate: "#3F4F3A", Surface: "#FAFDFA", SurfaceAlt: "#F5F7F5",
			DisplayFont: "Times New Roman", BodyFont: "Open Sans", EastAsiaFont: "PingFang SC", Monospace: "Cascadia Code",
		},
		"coral": {
			Name: "coral", Primary: "#FF6B6B", PrimaryDark: "#E85555", PrimaryTint: "#FFF0F0",
			Secondary: "#4ECDC4", Accent: "#FFE66D", Success: "#16A34A", Warning: "#D97706", Danger: "#EF4444",
			Slate: "#475569", Surface: "#FFFFFF", SurfaceAlt: "#FFFAFA",
			DisplayFont: "Poppins", BodyFont: "Nunito", EastAsiaFont: "PingFang SC", Monospace: "SF Mono",
		},
	}
}

func resolveNativeMarkdownPPTXTheme(name, styleHint string) nativeMarkdownPPTXTheme {
	catalog := nativeMarkdownPPTXThemeCatalog()

	key := strings.ToLower(strings.TrimSpace(name))
	if theme, ok := catalog[key]; ok {
		return nativeMarkdownPPTXResolvedTheme(theme)
	}

	hint := strings.ToLower(strings.TrimSpace(styleHint))
	switch {
	case nativeMarkdownContainsAny(hint, "editorial", "poster", "manifesto", "magazine", "typographic", "high contrast", "bold type", "海报", "宣言", "杂志感", "编排感", "高对比", "强对比", "大字标题", "粗体标题"):
		return nativeMarkdownPPTXResolvedTheme(catalog["editorial"])
	case nativeMarkdownContainsAny(hint, "sustainability", "environment", "green", "eco", "carbon", "climate", "wellness", "health", "csr", "esg", "可持续", "环境", "绿色", "碳", "气候", "健康", "社会责任"):
		return nativeMarkdownPPTXResolvedTheme(catalog["forest"])
	case nativeMarkdownContainsAny(hint, "creative", "design", "brand", "visual", "aesthetic", "illustration", "创意", "设计", "品牌", "视觉", "艺术"):
		return nativeMarkdownPPTXResolvedTheme(catalog["terracotta"])
	case nativeMarkdownContainsAny(hint, "board", "annual", "investor", "stakeholder", "governance", "高管", "董事会", "年报", "投资者", "股东"):
		return nativeMarkdownPPTXResolvedTheme(catalog["midnight"])
	case nativeMarkdownContainsAny(hint, "ui", "ux", "review", "audit", "score", "评级", "评估", "评审", "审查", "界面", "可用性", "体验"):
		return nativeMarkdownPPTXResolvedTheme(catalog["ui_review"])
	case nativeMarkdownContainsAny(hint, "executive", "briefing", "formal", "management", "汇报", "正式", "商业"):
		return nativeMarkdownPPTXResolvedTheme(catalog["executive"])
	case nativeMarkdownContainsAny(hint, "clean", "minimal", "neutral", "simple", "极简", "简洁", "中性"):
		return nativeMarkdownPPTXResolvedTheme(catalog["clean"])
	case nativeMarkdownContainsAny(hint, "pitch", "launch", "startup", "growth", "marketing", "sales", "融资", "路演", "发布", "增长", "营销"):
		return nativeMarkdownPPTXResolvedTheme(catalog["coral"])
	default:
		return nativeMarkdownPPTXResolvedTheme(catalog["midnight"])
	}
}

func nativeMarkdownPPTXResolvedTheme(theme nativeMarkdownPPTXTheme) nativeMarkdownPPTXTheme {
	if strings.TrimSpace(theme.Name) == "" {
		theme = nativeMarkdownPPTXThemeCatalog()["midnight"]
	}
	theme.DisplayFont = firstNonEmptyDeckValue(nativeMarkdownPPTXSafeFontName(theme.DisplayFont), "Georgia")
	theme.BodyFont = firstNonEmptyDeckValue(nativeMarkdownPPTXSafeFontName(theme.BodyFont), "Helvetica")
	theme.EastAsiaFont = firstNonEmptyDeckValue(nativeMarkdownPPTXSafeEastAsiaFont(theme.EastAsiaFont), "Hiragino Sans GB")
	theme.Monospace = firstNonEmptyDeckValue(nativeMarkdownPPTXSafeFontName(theme.Monospace), "Menlo")
	return theme
}

func nativeMarkdownPPTXSafeFontName(font string) string {
	switch strings.TrimSpace(font) {
	case "Aptos Display", "Palatino Linotype":
		return "Georgia"
	case "Aptos", "Segoe UI", "Open Sans", "Nunito", "Inter":
		return "Helvetica"
	case "Poppins":
		return "Helvetica Neue"
	case "Aptos Mono", "Consolas", "Fira Code", "Cascadia Code", "SF Mono":
		return "Menlo"
	default:
		return strings.TrimSpace(font)
	}
}

func nativeMarkdownPPTXSafeEastAsiaFont(font string) string {
	switch strings.TrimSpace(font) {
	case "", "PingFang SC":
		return "Hiragino Sans GB"
	default:
		return strings.TrimSpace(font)
	}
}

func nativeMarkdownHumanizeThemeToken(value string) string {
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

func nativeMarkdownContainsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if needle != "" && strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
