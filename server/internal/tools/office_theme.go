package tools

import (
	"fmt"
	"strings"
	"sync"
)

type officeTheme struct {
	Name          string
	Primary       string
	PrimaryDark   string
	PrimaryTint   string
	Accent        string
	AccentTint    string
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
	DisplayFont   string
	BodyFont      string
	EastAsiaFont  string
	MonospaceFont string
}

var (
	officeThemeCatalog     map[string]officeTheme
	officeThemeCatalogOnce sync.Once
)

func ensureOfficeThemeCatalog() {
	officeThemeCatalogOnce.Do(func() {
		officeThemeCatalog = map[string]officeTheme{
			"analysis": {
				Name:          "analysis",
				Primary:       "#1A6FC4",
				PrimaryDark:   "#0D4A8A",
				PrimaryTint:   "#EAF4FF",
				Accent:        "#0EA5E9",
				AccentTint:    "#E0F2FE",
				Success:       "#166534",
				SuccessTint:   "#DCFCE7",
				Warning:       "#B45309",
				WarningTint:   "#FEF3C7",
				Danger:        "#B91C1C",
				DangerTint:    "#FEE2E2",
				Slate:         "#475569",
				SlateTint:     "#E2E8F0",
				Border:        "#CBD5E1",
				Surface:       "#FFFFFF",
				SurfaceAlt:    "#F8FAFC",
				SurfaceMuted:  "#F1F5F9",
				DisplayFont:   "Aptos Display",
				BodyFont:      "Aptos",
				EastAsiaFont:  "PingFang SC",
				MonospaceFont: "Aptos Mono",
			},
			"ui_review": {
				Name:          "ui_review",
				Primary:       "#0369A1",
				PrimaryDark:   "#0F172A",
				PrimaryTint:   "#E0F2FE",
				Accent:        "#6D28D9",
				AccentTint:    "#EDE9FE",
				Success:       "#166534",
				SuccessTint:   "#DCFCE7",
				Warning:       "#B45309",
				WarningTint:   "#FEF3C7",
				Danger:        "#B91C1C",
				DangerTint:    "#FEE2E2",
				Slate:         "#475569",
				SlateTint:     "#E2E8F0",
				Border:        "#CBD5E1",
				Surface:       "#FFFFFF",
				SurfaceAlt:    "#F8FAFC",
				SurfaceMuted:  "#F1F5F9",
				DisplayFont:   "Aptos Display",
				BodyFont:      "Aptos",
				EastAsiaFont:  "PingFang SC",
				MonospaceFont: "Aptos Mono",
			},
			"executive": {
				Name:          "executive",
				Primary:       "#1E3A8A",
				PrimaryDark:   "#172554",
				PrimaryTint:   "#DBEAFE",
				Accent:        "#0F766E",
				AccentTint:    "#CCFBF1",
				Success:       "#166534",
				SuccessTint:   "#DCFCE7",
				Warning:       "#92400E",
				WarningTint:   "#FDE68A",
				Danger:        "#991B1B",
				DangerTint:    "#FECACA",
				Slate:         "#334155",
				SlateTint:     "#E2E8F0",
				Border:        "#CBD5E1",
				Surface:       "#FFFFFF",
				SurfaceAlt:    "#F8FAFC",
				SurfaceMuted:  "#F1F5F9",
				DisplayFont:   "Aptos Display",
				BodyFont:      "Aptos",
				EastAsiaFont:  "PingFang SC",
				MonospaceFont: "Aptos Mono",
			},
			"clean": {
				Name:          "clean",
				Primary:       "#334155",
				PrimaryDark:   "#0F172A",
				PrimaryTint:   "#F1F5F9",
				Accent:        "#2563EB",
				AccentTint:    "#DBEAFE",
				Success:       "#15803D",
				SuccessTint:   "#DCFCE7",
				Warning:       "#A16207",
				WarningTint:   "#FEF3C7",
				Danger:        "#B91C1C",
				DangerTint:    "#FEE2E2",
				Slate:         "#475569",
				SlateTint:     "#E2E8F0",
				Border:        "#CBD5E1",
				Surface:       "#FFFFFF",
				SurfaceAlt:    "#F8FAFC",
				SurfaceMuted:  "#F1F5F9",
				DisplayFont:   "Aptos Display",
				BodyFont:      "Aptos",
				EastAsiaFont:  "PingFang SC",
				MonospaceFont: "Aptos Mono",
			},
		}
	})
}

func resolveOfficeTheme(name, styleHint string) officeTheme {
	ensureOfficeThemeCatalog()

	key := strings.ToLower(strings.TrimSpace(name))
	if theme, ok := officeThemeCatalog[key]; ok {
		return theme
	}

	hint := strings.ToLower(strings.TrimSpace(styleHint))
	switch {
	case officeContainsAny(hint,
		"ui", "ux", "review", "audit", "score", "评级", "评估", "评审", "审查", "界面", "可用性", "体验",
	):
		return officeThemeCatalog["ui_review"]
	case officeContainsAny(hint,
		"executive", "board", "briefing", "formal", "management", "高管", "董事会", "汇报", "正式", "商业",
	):
		return officeThemeCatalog["executive"]
	case officeContainsAny(hint,
		"clean", "minimal", "neutral", "simple", "极简", "简洁", "中性",
	):
		return officeThemeCatalog["clean"]
	default:
		return officeThemeCatalog["analysis"]
	}
}

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
