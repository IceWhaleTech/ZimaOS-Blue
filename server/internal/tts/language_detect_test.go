package tts

import (
	"testing"
)

func TestDetectLanguage_English(t *testing.T) {
	for _, tt := range []struct{ text, want string }{
		{"Hello, world!", "en"},
		{"The quick brown fox jumps over the lazy dog.", "en"},
		{"Go is an open source programming language.", "en"},
	} {
		lang, ratio := DetectLanguage(tt.text)
		if lang != tt.want {
			t.Errorf("DetectLanguage(%q) = %q (%.2f), want %q", tt.text, lang, ratio, tt.want)
		}
		if ratio < 0.5 {
			t.Errorf("DetectLanguage(%q) ratio = %.2f, want >= 0.5", tt.text, ratio)
		}
	}
}

func TestDetectLanguage_Chinese(t *testing.T) {
	for _, tt := range []struct{ text, want string }{
		{"你好世界", "cmn"},
		{"今天天气真好", "cmn"},
		{"中华人民共和国", "cmn"},
		{"机器学习是人工智能的一个分支", "cmn"},
	} {
		lang, ratio := DetectLanguage(tt.text)
		if lang != tt.want {
			t.Errorf("DetectLanguage(%q) = %q (%.2f), want %q", tt.text, lang, ratio, tt.want)
		}
	}
}

func TestDetectLanguage_Japanese(t *testing.T) {
	for _, tt := range []struct{ text, want string }{
		{"こんにちは世界", "ja"},
		{"カタカナテスト", "ja"},
		{"東京都は日本の首都です", "ja"},
		{"プログラミング言語", "ja"},
		{"お元気ですか", "ja"},
		{"ありがとうございます", "ja"},
	} {
		lang, ratio := DetectLanguage(tt.text)
		if lang != tt.want {
			t.Errorf("DetectLanguage(%q) = %q (%.2f), want %q", tt.text, lang, ratio, tt.want)
		}
	}
}

func TestDetectLanguage_Korean(t *testing.T) {
	for _, tt := range []struct{ text, want string }{
		{"안녕하세요", "ko"},
		{"한국어 테스트입니다", "ko"},
		{"서울은 대한민국의 수도입니다", "ko"},
	} {
		lang, ratio := DetectLanguage(tt.text)
		if lang != tt.want {
			t.Errorf("DetectLanguage(%q) = %q (%.2f), want %q", tt.text, lang, ratio, tt.want)
		}
	}
}

func TestDetectLanguage_Russian(t *testing.T) {
	for _, tt := range []struct{ text, want string }{
		{"Привет мир", "ru"},
		{"Добро пожаловать", "ru"},
	} {
		lang, ratio := DetectLanguage(tt.text)
		if lang != tt.want {
			t.Errorf("DetectLanguage(%q) = %q (%.2f), want %q", tt.text, lang, ratio, tt.want)
		}
	}
}

func TestDetectLanguage_Arabic(t *testing.T) {
	lang, ratio := DetectLanguage("مرحبا بالعالم")
	if lang != "ar" {
		t.Errorf("DetectLanguage(Arabic) = %q (%.2f), want ar", lang, ratio)
	}
}

func TestDetectLanguage_Thai(t *testing.T) {
	lang, ratio := DetectLanguage("สวัสดีครับ")
	if lang != "th" {
		t.Errorf("DetectLanguage(Thai) = %q (%.2f), want th", lang, ratio)
	}
}

func TestDetectLanguage_Hindi(t *testing.T) {
	lang, ratio := DetectLanguage("नमस्ते दुनिया")
	if lang != "hi" {
		t.Errorf("DetectLanguage(Hindi) = %q (%.2f), want hi", lang, ratio)
	}
}

func TestDetectLanguage_EmptyAndShort(t *testing.T) {
	lang, ratio := DetectLanguage("")
	if lang != "en" || ratio != 0.0 {
		t.Errorf("DetectLanguage empty = %q (%.2f), want en (0.0)", lang, ratio)
	}

	lang, ratio = DetectLanguage("   ")
	if lang != "en" || ratio != 0.0 {
		t.Errorf("DetectLanguage spaces = %q (%.2f), want en (0.0)", lang, ratio)
	}

	lang, _ = DetectLanguage("你")
	if lang != "cmn" {
		t.Errorf("DetectLanguage single Han = %q, want cmn", lang)
	}
}

func TestDetectLanguage_MixedCJK(t *testing.T) {
	// Kanji + Hiragana → Japanese
	lang, _ := DetectLanguage("漢字とひらがな")
	if lang != "ja" {
		t.Errorf("Kanji+Hiragana = %q, want ja", lang)
	}

	// Pure Kanji without Kana → Chinese
	lang, _ = DetectLanguage("人工智能")
	if lang != "cmn" {
		t.Errorf("pure Kanji = %q, want cmn", lang)
	}
}

func TestDetectLanguage_MixedLatinCJK(t *testing.T) {
	lang, _ := DetectLanguage("今天学习Go语言")
	if lang != "cmn" {
		t.Errorf("Chinese+English = %q, want cmn", lang)
	}

	lang, _ = DetectLanguage("I love すし")
	if lang != "en" {
		t.Errorf("English+Japanese = %q, want en", lang)
	}
}

func TestAnalyzeText_CJKHanShared(t *testing.T) {
	stats := AnalyzeText("中文测试")
	if stats.CJKHan != 4 {
		t.Errorf("CJKHan = %d, want 4", stats.CJKHan)
	}
	if stats.Kana != 0 {
		t.Errorf("Kana = %d, want 0", stats.Kana)
	}

	stats = AnalyzeText("日本語のテスト")
	if stats.Kana == 0 {
		t.Error("Kana should be > 0 for Japanese text")
	}
	if stats.CJKHan == 0 {
		t.Error("CJKHan should be > 0 for Japanese text with Kanji")
	}
}

func TestGetLanguageRatios(t *testing.T) {
	stats := AnalyzeText("Hello 你好 こんにちは")
	ratios := stats.GetLanguageRatios()
	if _, ok := ratios["en"]; !ok {
		t.Error("expected en in ratios")
	}
	if _, ok := ratios["ja"]; !ok {
		t.Error("expected ja in ratios")
	}
}

func TestIsMixedLanguage(t *testing.T) {
	stats := AnalyzeText("Hello world")
	if stats.IsMixedLanguage() {
		t.Error("pure English should not be mixed")
	}

	stats = AnalyzeText("Hello 你好世界")
	if !stats.IsMixedLanguage() {
		t.Error("English + Chinese should be mixed")
	}
}
