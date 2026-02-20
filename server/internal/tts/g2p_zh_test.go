//go:build kokoro

package tts

import (
	"strings"
	"testing"
)

func TestChineseG2P_WordBoundaries(t *testing.T) {
	g2p := NewChineseG2P(nil)

	// A long Chinese phrase should have space tokens inserted every 2 syllables
	// "我是你的个人助手运行在系统上" = 14 characters → spaces at positions 2, 4, 6, ...
	ipa := g2p.Phonemize("我是你的个人助手运行在系统上")
	spaces := strings.Count(ipa, " ")
	if spaces < 2 {
		t.Errorf("long Chinese phrase should have word boundary spaces, got %d spaces in: %s", spaces, ipa)
	}
}

func TestChineseG2P_ShortPhrase(t *testing.T) {
	g2p := NewChineseG2P(nil)

	// 4 chars with 2-syllable grouping → 2 groups with 1 space between
	ipa := g2p.Phonemize("你好世界")
	if ipa == "" {
		t.Error("expected non-empty IPA for 你好世界")
	}
	// Should have at least 1 space (2+2 grouping)
	if !strings.Contains(ipa, " ") {
		t.Errorf("expected space in 4-char phrase with 2-syllable grouping, got: %s", ipa)
	}
}

func TestChineseG2P_PunctuationPreserved(t *testing.T) {
	g2p := NewChineseG2P(nil)

	ipa := g2p.Phonemize("你好，世界！")
	if !strings.Contains(ipa, ",") {
		t.Errorf("expected comma in IPA output, got: %s", ipa)
	}
	if !strings.Contains(ipa, "!") {
		t.Errorf("expected exclamation in IPA output, got: %s", ipa)
	}
}

func TestChineseG2P_MixedChineseEnglish(t *testing.T) {
	en := NewEnglishG2P()
	g2p := NewChineseG2P(en)

	ipa := g2p.Phonemize("运行在 ZimaOS 上")
	if ipa == "" {
		t.Error("expected non-empty IPA for mixed Chinese/English")
	}
	// Should contain space separating Chinese and English segments
	if !strings.Contains(ipa, " ") {
		t.Errorf("expected spaces in mixed text IPA, got: %s", ipa)
	}
}

func TestChineseG2P_Erhua(t *testing.T) {
	g2p := NewChineseG2P(nil)

	// 花儿 → erhua: 儿 should be absorbed, producing rhotacized ɚ
	ipa := g2p.Phonemize("花儿")
	if !strings.Contains(ipa, "ɚ") {
		t.Errorf("expected rhotacized ɚ in 花儿, got: %s", ipa)
	}

	// 女儿 → NOT erhua (in notErhua set), 儿 should be a separate syllable
	ipa2 := g2p.Phonemize("女儿")
	// Should have IPA for both syllables, no rhotacization
	if strings.Contains(ipa2, "ɚ") {
		t.Errorf("女儿 should NOT have erhua rhotacization, got: %s", ipa2)
	}
}

func TestChineseG2P_ToneSandhi(t *testing.T) {
	g2p := NewChineseG2P(nil)

	// 你好 = ni3 hao3 → tone sandhi: ni2 hao3
	// The ↗ (rising) marker should appear for the first syllable
	ipa := g2p.Phonemize("你好")
	if !strings.Contains(ipa, "↗") {
		t.Errorf("expected rising tone marker ↗ from 3+3 sandhi in 你好, got: %s", ipa)
	}
}
