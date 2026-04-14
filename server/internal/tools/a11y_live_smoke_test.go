//go:build darwin

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	a11yruntime "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/a11y"
)

func TestLiveA11yToolGreetingPath(t *testing.T) {
	if strings.TrimSpace(os.Getenv("BLUE_A11Y_LIVE_SEND")) != "1" {
		t.Skip("set BLUE_A11Y_LIVE_SEND=1 to run the live tool-level send smoke")
	}

	backend := a11yruntime.DefaultHostBackend(filepath.Join(t.TempDir(), "media"))
	if backend == nil {
		t.Fatal("expected darwin a11y backend")
	}

	ctx := context.Background()
	windows, err := backend.ListWindows(ctx)
	if err != nil {
		t.Fatalf("ListWindows() error: %v", err)
	}

	selectorKind := strings.ToLower(strings.TrimSpace(os.Getenv("BLUE_A11Y_LIVE_SELECTOR_KIND")))
	if selectorKind == "" {
		selectorKind = "frontmost"
	}
	selectorValue := strings.TrimSpace(os.Getenv("BLUE_A11Y_LIVE_SELECTOR"))
	if selectorValue == "" {
		selectorValue = "Feishu 飞书 Lark"
	}
	aliases := parseA11yWindowMatchTerms(selectorValue)
	if len(aliases) == 0 {
		t.Fatalf("selector aliases are empty for %q", selectorValue)
	}

	args := map[string]interface{}{
		"action": "act",
		"params": map[string]interface{}{
			"value":  liveA11yToolMessage(),
			"submit": true,
		},
	}

	switch selectorKind {
	case "frontmost":
		focused, ok := liveA11yToolFocusedWindow(windows)
		if !ok {
			t.Skip("no focused window detected; bring the target chat app to the front first")
		}
		if !liveA11yToolWindowMatches(focused, aliases...) {
			t.Skipf("focused window %q / %q does not look like the target app; bring Feishu/Lark frontmost first", focused.AppName, focused.Title)
		}
	case "app_name":
		args["app_name"] = selectorValue
		if !liveA11yToolAnyAppNameMatches(windows, aliases...) {
			t.Skipf("no visible window matched app_name aliases %q", selectorValue)
		}
	case "window_title":
		args["window_title"] = selectorValue
		if !liveA11yToolAnyWindowTitleMatches(windows, aliases...) {
			t.Skipf("no visible window matched window_title aliases %q", selectorValue)
		}
	default:
		t.Fatalf("unsupported BLUE_A11Y_LIVE_SELECTOR_KIND=%q", selectorKind)
	}

	tool := NewA11yTool()
	tool.SetBackend(backend)

	start := time.Now()
	raw, err := tool.Execute(ctx, args)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Execute() error after %s: %v", elapsed, err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error: %v", err)
	}
	if code := strings.TrimSpace(fmt.Sprint(out["error_code"])); code != "" && code != "<nil>" {
		t.Fatalf("live send failed after %s: %s (%v)", elapsed, code, out["error"])
	}
	if message := strings.TrimSpace(fmt.Sprint(out["message"])); message != "Host action completed and submitted" {
		t.Fatalf("message = %q, want Host action completed and submitted", message)
	}

	if maxMS := liveA11yToolEnvInt("BLUE_A11Y_LIVE_MAX_MS"); maxMS > 0 && elapsed > time.Duration(maxMS)*time.Millisecond {
		t.Fatalf("elapsed = %s, want <= %dms", elapsed, maxMS)
	}

	t.Logf("selector_kind=%s selector=%q elapsed=%s response=%s", selectorKind, selectorValue, elapsed, raw.(string))
}

func liveA11yToolMessage() string {
	if value := strings.TrimSpace(os.Getenv("BLUE_A11Y_LIVE_MESSAGE")); value != "" {
		return value
	}
	return fmt.Sprintf("你好，Orca %s", time.Now().Format("2006-01-02 15:04:05"))
}

func liveA11yToolEnvInt(name string) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0
	}
	return parsed
}

func liveA11yToolFocusedWindow(windows []a11yruntime.WindowInfo) (a11yruntime.WindowInfo, bool) {
	for _, window := range windows {
		if window.Focused {
			return window, true
		}
	}
	return a11yruntime.WindowInfo{}, false
}

func liveA11yToolWindowMatches(window a11yruntime.WindowInfo, aliases ...string) bool {
	return liveA11yToolFieldMatches(window.AppName, aliases...) || liveA11yToolFieldMatches(window.Title, aliases...)
}

func liveA11yToolAnyAppNameMatches(windows []a11yruntime.WindowInfo, aliases ...string) bool {
	for _, window := range windows {
		if liveA11yToolFieldMatches(window.AppName, aliases...) {
			return true
		}
	}
	return false
}

func liveA11yToolAnyWindowTitleMatches(windows []a11yruntime.WindowInfo, aliases ...string) bool {
	for _, window := range windows {
		if liveA11yToolFieldMatches(window.Title, aliases...) {
			return true
		}
	}
	return false
}

func liveA11yToolFieldMatches(value string, aliases ...string) bool {
	normalizedValue := normalizeA11yWindowMatchValue(value)
	if normalizedValue == "" {
		return false
	}
	for _, alias := range aliases {
		if alias == "" {
			continue
		}
		if strings.Contains(normalizedValue, alias) {
			return true
		}
	}
	return false
}
