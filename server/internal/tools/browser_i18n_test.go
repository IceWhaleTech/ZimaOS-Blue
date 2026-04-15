package tools

import (
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/i18n"
)

func TestBrowserPageScrolledMessageLocalizesDirection(t *testing.T) {
	got := BrowserPageScrolledMessage(i18n.LangZhCN, "down")
	want := "页面已向下滚动"
	if got != want {
		t.Fatalf("BrowserPageScrolledMessage() = %q, want %q", got, want)
	}
}

func TestBrowserPageMessageLocalizesInteractiveCount(t *testing.T) {
	got := BrowserPageMessage(i18n.LangJaJP, "Example", "https://example.com", "TREE", 3)
	want := "ページ: Example (https://example.com) — 3 個のインタラクティブ要素\n\nTREE"
	if got != want {
		t.Fatalf("BrowserPageMessage() = %q, want %q", got, want)
	}
}

func TestBrowserReadableContentWithTreeMessageUsesLocalizedSection(t *testing.T) {
	got := BrowserReadableContentWithTreeMessage(
		i18n.LangZhTW,
		"Example",
		"https://example.com",
		"CONTENT",
		"screenshot+interactive",
		"TREE",
	)
	want := "頁面：Example (https://example.com)\n\n主要內容：\nCONTENT\n\n互動元素：\nTREE"
	if got != want {
		t.Fatalf("BrowserReadableContentWithTreeMessage() = %q, want %q", got, want)
	}
}
