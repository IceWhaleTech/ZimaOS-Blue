package server

import "testing"

func TestMapCardAction_WebFetchUseBrowser(t *testing.T) {
	h := &ChatHandler{}
	msg := h.mapCardAction(
		"web-fetch-https%3A%2F%2Fwww.reddit.com%2Fr%2Fgolang",
		"use_browser",
		"Use browser",
		"web-fetch",
		"Reddit",
		map[string]interface{}{"url": "https://www.reddit.com/r/golang"},
	)
	want := "Open https://www.reddit.com/r/golang with the browser tool. If the page needs login, challenge handling, or dynamic interaction, continue in the browser and summarize the relevant content."
	if msg != want {
		t.Fatalf("mapCardAction returned %q, want %q", msg, want)
	}
}

func TestMapCardAction_BrowserExtractWithWebFetch(t *testing.T) {
	h := &ChatHandler{}
	msg := h.mapCardAction(
		"browser-tab-tab-42",
		"extract_with_web_fetch",
		"Extract with web_fetch",
		"result",
		"r/test",
		map[string]interface{}{
			"url":               "https://www.reddit.com/r/test",
			"browser_target_id": "tab-42",
		},
	)
	want := "Use web_fetch on https://www.reddit.com/r/test with browser_target_id=tab-42 to extract readable content using the current browser session cookies, then summarize the relevant content."
	if msg != want {
		t.Fatalf("mapCardAction returned %q, want %q", msg, want)
	}
}
