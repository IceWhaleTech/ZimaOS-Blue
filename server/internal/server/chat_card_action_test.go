package server

import (
	"context"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

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
	want := "Open https://www.reddit.com/r/golang with the browser tool. If you only need readable content from an existing logged-in session, prefer web_fetch with browser_target_id or current session cookies first. If the page needs live login, challenge handling, or dynamic interaction, continue in the browser and summarize the relevant content."
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

func TestPendingBrowserLaunchIntent_VisibleOnceForMatchingMessage(t *testing.T) {
	h := NewChatHandler(nil, nil, tools.NewRegistry())
	msg := "Open https://example.com with the browser tool."
	h.registerPendingBrowserLaunchIntent("conv-1", msg, tools.BrowserLaunchModeVisible)

	ctx := h.applyPendingBrowserLaunchIntent(context.Background(), "conv-1", msg)
	if got := tools.GetBrowserLaunchMode(ctx); got != tools.BrowserLaunchModeVisible {
		t.Fatalf("browser launch mode = %q, want %q", got, tools.BrowserLaunchModeVisible)
	}

	ctx = h.applyPendingBrowserLaunchIntent(context.Background(), "conv-1", msg)
	if got := tools.GetBrowserLaunchMode(ctx); got != tools.BrowserLaunchModeDefault {
		t.Fatalf("browser launch mode after consume = %q, want default", got)
	}
}

func TestPendingBrowserLaunchIntent_DoesNotConsumeDifferentMessage(t *testing.T) {
	h := NewChatHandler(nil, nil, tools.NewRegistry())
	h.registerPendingBrowserLaunchIntent("conv-1", "Open https://example.com with the browser tool.", tools.BrowserLaunchModeVisible)

	ctx := h.applyPendingBrowserLaunchIntent(context.Background(), "conv-1", "Something else")
	if got := tools.GetBrowserLaunchMode(ctx); got != tools.BrowserLaunchModeDefault {
		t.Fatalf("browser launch mode = %q, want default", got)
	}

	ctx = h.applyPendingBrowserLaunchIntent(context.Background(), "conv-1", "Open https://example.com with the browser tool.")
	if got := tools.GetBrowserLaunchMode(ctx); got != tools.BrowserLaunchModeVisible {
		t.Fatalf("browser launch mode on matching retry = %q, want %q", got, tools.BrowserLaunchModeVisible)
	}
}
