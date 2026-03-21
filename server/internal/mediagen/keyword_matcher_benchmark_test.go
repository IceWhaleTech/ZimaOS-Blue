package mediagen

import (
	"strings"
	"testing"
)

func BenchmarkKeywordCueMatcherBuild(b *testing.B) {
	cues := benchmarkMergedCues()
	factories := []struct {
		name    string
		factory keywordCueMatcherFactory
	}{
		{name: "scan", factory: newScanKeywordCueMatcher},
		{name: "aho", factory: newAhoKeywordCueMatcher},
	}

	for _, factory := range factories {
		b.Run(factory.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = factory.factory(cues)
			}
		})
	}
}

func BenchmarkKeywordCueMatcherContains(b *testing.B) {
	cues := benchmarkMergedCues()
	texts := map[string]string{
		"short_english_prompt": "Please generate a highly detailed image of a moonlit harbor with watercolor textures and warm reflections.",
		"long_chinese_meta":    "IR匹配关键词的时候，也需要考虑关键词命中的密度吧，比如在一大段文本内部出现了生成图片可能就不是这个意图。这里主要是在讨论分类规则、误判风险和命中触发条件，而不是要真正发起图片生成请求。",
		"long_incidental":      strings.Repeat("We are discussing workspace summaries, documentation rules, release notes, and migration steps. ", 8) + "Somewhere in the middle we mention generate image support for a future milestone without making a request.",
	}

	for _, factory := range benchmarkMatcherFactories() {
		matcher := factory.factory(cues)
		for name, text := range texts {
			b.Run(factory.name+"/"+name, func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					_ = matcher.Contains(text)
				}
			})
		}
	}
}

func BenchmarkKeywordCueMatcherFindMatches(b *testing.B) {
	cues := benchmarkMergedCues()
	text := "Please edit this image by replacing the background with a warm sunset beach, lightly retouch the colors, and then generate an illustration-style preview."

	for _, factory := range benchmarkMatcherFactories() {
		matcher := factory.factory(cues)
		b.Run(factory.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = matcher.FindMatches(text)
			}
		})
	}
}

func BenchmarkClassifyMediaIntentSegment(b *testing.B) {
	segment := "Please generate a highly detailed image of a moonlit harbor with watercolor textures, warm reflections, and soft cinematic lighting."
	segmentMeta := "IR匹配关键词的时候，也需要考虑关键词命中的密度吧，比如在一大段文本内部出现了生成图片可能就不是这个意图。"

	for _, factory := range benchmarkMatcherFactories() {
		compiled := compileLangKeywordsWithFactory(allLangKeywords, factory.factory)
		kw := compiled["en"]
		kwZH := compiled["zh"]
		kwEN := compiled["en"]

		b.Run(factory.name+"/english_prompt", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = classifyMediaIntentSegment(segment, false, 0, kw, kwEN, false)
			}
		})

		b.Run(factory.name+"/chinese_meta", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = classifyMediaIntentSegment(segmentMeta, false, 0, kwZH, kwEN, true)
			}
		})
	}
}

func benchmarkMatcherFactories() []struct {
	name    string
	factory keywordCueMatcherFactory
} {
	return []struct {
		name    string
		factory keywordCueMatcherFactory
	}{
		{name: "scan", factory: newScanKeywordCueMatcher},
		{name: "aho", factory: newAhoKeywordCueMatcher},
	}
}

func benchmarkMergedCues() []string {
	merged := make([]string, 0, 256)
	for _, kw := range allLangKeywords {
		merged = append(merged, kw.actions...)
		merged = append(merged, kw.imageNouns...)
		merged = append(merged, kw.videoNouns...)
		merged = append(merged, kw.editVerbs...)
		merged = append(merged, kw.animVerbs...)
		merged = append(merged, kw.metaCues...)
		merged = append(merged, kw.metaStrong...)
	}
	return merged
}
