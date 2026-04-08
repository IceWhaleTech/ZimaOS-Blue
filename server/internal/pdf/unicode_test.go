package pdf

import "testing"

func TestCanonicalizeExtractedPDFText_RepairsMalformedPDFCharacters(t *testing.T) {
	raw := "智能挃针和其他 C++特性\n觃则乊例外\nC++开収\n难亍阅诺和维护\npath\\demo 'quoted'"

	got := canonicalizeExtractedPDFText(raw)
	want := "智能指针和其他 C++特性\n规则之例外\nC++开发\n难于阅读和维护\npath\\demo 'quoted'"

	if got != want {
		t.Fatalf("canonicalizeExtractedPDFText() = %q, want %q", got, want)
	}
}

func TestCanonicalizeExtractedPDFText_LeavesNormalChineseUntouchedWithoutRareRunes(t *testing.T) {
	raw := "普通中文\npath\\demo 'quoted'"

	got := canonicalizeExtractedPDFText(raw)
	want := "普通中文\npath\\demo 'quoted'"

	if got != want {
		t.Fatalf("canonicalizeExtractedPDFText() = %q, want %q", got, want)
	}
}
