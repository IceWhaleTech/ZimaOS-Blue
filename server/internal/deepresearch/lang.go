package deepresearch

import (
	"fmt"
	"strings"
	"unicode"
)

type researchLang string

const (
	researchLangEN researchLang = "en"
	researchLangZH researchLang = "zh"
	researchLangJA researchLang = "ja"
	researchLangKO researchLang = "ko"
)

func normalizeResearchLang(lang, fallbackText string) researchLang {
	raw := strings.ToLower(strings.TrimSpace(lang))
	switch {
	case strings.HasPrefix(raw, "zh"):
		return researchLangZH
	case strings.HasPrefix(raw, "ja"):
		return researchLangJA
	case strings.HasPrefix(raw, "ko"):
		return researchLangKO
	case strings.HasPrefix(raw, "en"):
		return researchLangEN
	}

	detected := detectResearchLangFromText(fallbackText)
	switch detected {
	case researchLangZH, researchLangJA, researchLangKO:
		return detected
	default:
		return researchLangEN
	}
}

func detectResearchLangFromText(text string) researchLang {
	for _, r := range text {
		switch {
		case unicode.In(r, unicode.Hiragana, unicode.Katakana):
			return researchLangJA
		case unicode.In(r, unicode.Hangul):
			return researchLangKO
		case unicode.In(r, unicode.Han):
			return researchLangZH
		}
	}
	return researchLangEN
}

func planQueriesForLang(query, lang string) []string {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil
	}

	switch normalizeResearchLang(lang, q) {
	case researchLangZH:
		return []string{
			q,
			fmt.Sprintf("%s 最新进展", q),
			fmt.Sprintf("%s 官方文档", q),
			fmt.Sprintf("%s 基准对比", q),
			fmt.Sprintf("%s 最佳实践", q),
		}
	case researchLangJA:
		return []string{
			q,
			fmt.Sprintf("%s 最新動向", q),
			fmt.Sprintf("%s 公式ドキュメント", q),
			fmt.Sprintf("%s ベンチマーク比較", q),
			fmt.Sprintf("%s ベストプラクティス", q),
		}
	case researchLangKO:
		return []string{
			q,
			fmt.Sprintf("%s 최신 동향", q),
			fmt.Sprintf("%s 공식 문서", q),
			fmt.Sprintf("%s 벤치마크 비교", q),
			fmt.Sprintf("%s 모범 사례", q),
		}
	default:
		return []string{
			q,
			fmt.Sprintf("%s latest updates", q),
			fmt.Sprintf("%s official documentation", q),
			fmt.Sprintf("%s benchmark comparison", q),
			fmt.Sprintf("%s best practices", q),
		}
	}
}

func searchRegionForLang(lang, query string) string {
	raw := strings.ToLower(strings.TrimSpace(lang))
	switch {
	case strings.HasPrefix(raw, "en-us"):
		return "us-en"
	case strings.HasPrefix(raw, "en-gb"):
		return "uk-en"
	case strings.HasPrefix(raw, "ja"):
		return "jp-jp"
	case strings.HasPrefix(raw, "ko"):
		return "kr-kr"
	case strings.HasPrefix(raw, "zh-cn"), strings.HasPrefix(raw, "zh-hans"):
		return "cn-zh"
	case strings.HasPrefix(raw, "zh-tw"), strings.HasPrefix(raw, "zh-hant"), strings.HasPrefix(raw, "zh-hk"):
		return "tw-tzh"
	}

	switch normalizeResearchLang(lang, query) {
	case researchLangJA:
		return "jp-jp"
	case researchLangKO:
		return "kr-kr"
	case researchLangZH:
		return "cn-zh"
	default:
		return "wt-wt"
	}
}

func localizedSummaryTitle(query, lang string) string {
	switch normalizeResearchLang(lang, query) {
	case researchLangZH:
		return fmt.Sprintf("调研总结：%s", query)
	case researchLangJA:
		return fmt.Sprintf("調査サマリー: %s", query)
	case researchLangKO:
		return fmt.Sprintf("리서치 요약: %s", query)
	default:
		return fmt.Sprintf("Research summary for: %s", query)
	}
}

func localizedNoEvidence(lang, query string) string {
	switch normalizeResearchLang(lang, query) {
	case researchLangZH:
		return "未收集到足够证据。"
	case researchLangJA:
		return "十分な根拠を収集できませんでした。"
	case researchLangKO:
		return "충분한 근거를 수집하지 못했습니다."
	default:
		return "No sufficient evidence was collected."
	}
}

func localizedOpenQuestion(lang, query string) string {
	switch normalizeResearchLang(lang, query) {
	case researchLangZH:
		return "建议回查一手来源并进行最终核验。"
	case researchLangJA:
		return "最終確認のため一次情報を確認してください。"
	case researchLangKO:
		return "최종 검증을 위해 1차 출처를 확인하세요."
	default:
		return "Check primary sources for final verification."
	}
}

func localizedConflictOpenQuestion(lang, query string) string {
	switch normalizeResearchLang(lang, query) {
	case researchLangZH:
		return "不同来源存在冲突，建议优先核对官方或原始发布时间。"
	case researchLangJA:
		return "情報源の内容に不一致があります。公式一次情報と公開日時を優先して確認してください。"
	case researchLangKO:
		return "출처 간 내용이 상충합니다. 공식 1차 출처와 게시 시점을 우선 확인하세요."
	default:
		return "Sources contain conflicting claims. Verify against official primary sources and publication dates."
	}
}
