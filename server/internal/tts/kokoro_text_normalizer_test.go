//go:build kokoro

package tts

import (
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeKokoroStructuredTextChinese(t *testing.T) {
	got := normalizeKokoroStructuredText("今天2个版本v1.2在2026-03-01 16:45发布，温度32°C，速度100km/h，电量85%，预算$12.5。", "zh-CN")

	for _, want := range []string{
		"两个",
		"v一点二",
		"二零二六年三月一日",
		"十六点四十五分",
		"三十二摄氏度",
		"一百公里每小时",
		"百分之八十五",
		"十二点五美元",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("normalizeKokoroStructuredText() = %q, missing %q", got, want)
		}
	}
}

func TestNormalizeKokoroStructuredTextEnglish(t *testing.T) {
	got := normalizeKokoroStructuredText("Meeting on Mon 03/01/2026 at 4:05 PM, temp 32°C, speed 100km/h, budget USD 12.5K.", "en-US")

	for _, want := range []string{
		"Monday",
		"March 1 2026",
		"4 05 PM",
		"32 degrees celsius",
		"100 kilometers per hour",
		"12.5 thousand US dollars",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("normalizeKokoroStructuredText() = %q, missing %q", got, want)
		}
	}
}

func TestKokoroPrepareChunksUsesStructuredTextNormalization(t *testing.T) {
	p := &KokoroProvider{
		g2p:       NewG2PDispatcher(),
		tokenizer: NewKokoroTokenizer(),
	}

	raw := "今天2个版本v1.2在2026-03-01 16:45发布，温度32°C，速度100km/h。"
	normalized := normalizeKokoroStructuredText(raw, "zh-CN")

	rawChunks := p.prepareChunks(raw, "zh-CN")
	normalizedChunks := p.prepareChunks(normalized, "zh-CN")

	if len(rawChunks) == 0 {
		t.Fatalf("prepareChunks(%q) returned no chunks", raw)
	}
	if !reflect.DeepEqual(rawChunks, normalizedChunks) {
		t.Fatalf("prepareChunks should normalize structured text before G2P\nraw: %#v\nnormalized: %#v", rawChunks, normalizedChunks)
	}
}
