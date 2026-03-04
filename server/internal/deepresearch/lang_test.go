package deepresearch

import (
	"strings"
	"testing"
)

func TestPlanQueriesForLang_ZH(t *testing.T) {
	queries := planQueriesForLang("ZimaOS", "zh-CN")
	if len(queries) < 3 {
		t.Fatalf("queries len = %d, want >= 3", len(queries))
	}
	if queries[1] != "ZimaOS 最新进展" {
		t.Fatalf("queries[1] = %q, want Chinese latest-updates suffix", queries[1])
	}
	if queries[2] != "ZimaOS 官方文档" {
		t.Fatalf("queries[2] = %q, want Chinese official-doc suffix", queries[2])
	}
}

func TestSearchRegionForLang(t *testing.T) {
	cases := []struct {
		lang  string
		query string
		want  string
	}{
		{lang: "en-US", query: "test", want: "us-en"},
		{lang: "en-GB", query: "test", want: "uk-en"},
		{lang: "zh-CN", query: "测试", want: "cn-zh"},
		{lang: "ja-JP", query: "テスト", want: "jp-jp"},
		{lang: "ko-KR", query: "테스트", want: "kr-kr"},
	}
	for _, tc := range cases {
		if got := searchRegionForLang(tc.lang, tc.query); got != tc.want {
			t.Fatalf("searchRegionForLang(%q,%q) = %q, want %q", tc.lang, tc.query, got, tc.want)
		}
	}
}

func TestSynthesizeReport_LocalizedText(t *testing.T) {
	report := synthesizeReport("ZimaOS", "zh-CN", []Evidence{
		{
			ID:               "ev1",
			Title:            "文档",
			URL:              "https://example.com",
			Domain:           "example.com",
			RelevanceScore:   0.9,
			CredibilityScore: 0.8,
		},
	})
	if report.Answer == "" {
		t.Fatal("expected non-empty localized answer")
	}
	if report.OpenQuestions == nil || len(report.OpenQuestions) == 0 {
		t.Fatal("expected localized open question")
	}
	if got := report.OpenQuestions[0]; got != "建议回查一手来源并进行最终核验。" {
		t.Fatalf("open question = %q, want Chinese text", got)
	}
}

func TestLocalizedConflictOpenQuestion(t *testing.T) {
	got := localizedConflictOpenQuestion("zh-CN", "ZimaOS")
	if got == "" {
		t.Fatal("expected non-empty conflict open question")
	}
}

func TestLocalizedSourceLabel_CoversAllSupportedLocales(t *testing.T) {
	cases := []struct {
		lang string
		want string
	}{
		{lang: "en-US", want: "Sources"},
		{lang: "en-GB", want: "Sources"},
		{lang: "zh-CN", want: "来源"},
		{lang: "zh-TW", want: "來源"},
		{lang: "ja-JP", want: "情報源"},
		{lang: "ko-KR", want: "출처"},
		{lang: "de-DE", want: "Quellen"},
		{lang: "fr-FR", want: "Sources"},
		{lang: "es-ES", want: "Fuentes"},
		{lang: "it-IT", want: "Fonti"},
		{lang: "pt-BR", want: "Fontes"},
		{lang: "pt-PT", want: "Fontes"},
		{lang: "ru-RU", want: "Источники"},
		{lang: "pl-PL", want: "Źródła"},
		{lang: "nl-NL", want: "Bronnen"},
		{lang: "sv-SE", want: "Källor"},
		{lang: "da-DK", want: "Kilder"},
		{lang: "nb-NO", want: "Kilder"},
		{lang: "cs-CZ", want: "Zdroje"},
		{lang: "sk-SK", want: "Zdroje"},
		{lang: "hu-HU", want: "Források"},
		{lang: "ro-RO", want: "Surse"},
		{lang: "hr-HR", want: "Izvori"},
		{lang: "el-GR", want: "Πηγές"},
		{lang: "ca-ES", want: "Fonts"},
		{lang: "ga-IE", want: "Foinsí"},
		{lang: "ml-IN", want: "ഉറവിടങ്ങൾ"},
	}
	for _, tc := range cases {
		if got := localizedSourceLabel(tc.lang, "test"); got != tc.want {
			t.Fatalf("localizedSourceLabel(%q)=%q, want %q", tc.lang, got, tc.want)
		}
	}
}

func TestSynthesizeReport_LocalizedCitationLabel_NonChinese(t *testing.T) {
	report := synthesizeReport("ZimaOS", "de-DE", []Evidence{
		{
			ID:               "ev1",
			Title:            "Dokument",
			URL:              "https://example.com",
			Domain:           "example.com",
			RelevanceScore:   0.9,
			CredibilityScore: 0.8,
			Quote:            "Zitattext",
		},
	})
	if got := report.Answer; !strings.Contains(got, "[Quellen#1]") {
		t.Fatalf("answer = %q, want localized source label", got)
	}
	if got := report.Answer; !strings.Contains(got, "Zitat: Zitattext") {
		t.Fatalf("answer = %q, want localized quote label", got)
	}
}
