package tools

import (
	"context"
	"errors"
	"testing"
)

type policyTestBrowserBackend struct {
	relay         bool
	startErr      error
	navErr        error
	startCalls    int
	navigateCalls int
	cookieCalls   int
	tabs          []BrowserTabResult
	cookieValue   string
}

func (m *policyTestBrowserBackend) Start(context.Context) error {
	m.startCalls++
	return m.startErr
}

func (m *policyTestBrowserBackend) Navigate(_ context.Context, url string, targetID string) (BrowserNavResult, error) {
	m.navigateCalls++
	if m.navErr != nil {
		return BrowserNavResult{}, m.navErr
	}
	if targetID == "" {
		targetID = "tab-1"
	}
	return BrowserNavResult{URL: url, TargetID: targetID}, nil
}

func (m *policyTestBrowserBackend) CookieHeader(_ context.Context, targetID string, url string) (string, error) {
	m.cookieCalls++
	return m.cookieValue + ":" + targetID + ":" + url, nil
}

func (m *policyTestBrowserBackend) ObserveNetwork(context.Context, string, int, bool) (BrowserObservedNetworkResult, error) {
	return BrowserObservedNetworkResult{}, nil
}

func (m *policyTestBrowserBackend) WaitNetworkIdle(context.Context, string, int, int) error {
	return nil
}

func (m *policyTestBrowserBackend) AccessibilityTree(context.Context, string, int) (BrowserA11yTreeResult, error) {
	return BrowserA11yTreeResult{}, nil
}

func (m *policyTestBrowserBackend) InteractiveElements(context.Context, string) (BrowserInteractiveResult, error) {
	return BrowserInteractiveResult{}, nil
}

func (m *policyTestBrowserBackend) CountInteractiveElements(context.Context, string) (int, error) {
	return 0, nil
}

func (m *policyTestBrowserBackend) ActByRef(context.Context, string, int, map[int]int, string, string) error {
	return nil
}

func (m *policyTestBrowserBackend) ActByInteractiveRef(context.Context, string, int, map[int]string, string, string) error {
	return nil
}

func (m *policyTestBrowserBackend) Screenshot(context.Context, string) (string, error) {
	return "", nil
}

func (m *policyTestBrowserBackend) ScreenshotTab(context.Context, string) (string, error) {
	return "", nil
}

func (m *policyTestBrowserBackend) CloseTab(context.Context, string) error {
	return nil
}

func (m *policyTestBrowserBackend) Tabs(context.Context) ([]BrowserTabResult, error) {
	return append([]BrowserTabResult(nil), m.tabs...), nil
}

func (m *policyTestBrowserBackend) ExecuteRecipe(context.Context, string, map[string]string) (BrowserRecipeResult, error) {
	return BrowserRecipeResult{}, nil
}

func (m *policyTestBrowserBackend) ListRecipes(context.Context) []BrowserRecipeInfo {
	return nil
}

func (m *policyTestBrowserBackend) UsesRelay(context.Context, string) bool {
	return m.relay
}

func TestSitePolicyBrowserBackendFallsBackToManagedWhenRelayUnavailable(t *testing.T) {
	ctx := context.Background()
	managed := &policyTestBrowserBackend{}
	relay := &policyTestBrowserBackend{
		relay:    true,
		startErr: errors.New("Chrome extension not connected"),
	}
	backend := NewSitePolicyBrowserBackend(
		managed,
		managed,
		relay,
		func(rawURL string) bool { return rawURL == "https://www.zhihu.com" },
		"managed",
	)

	nav, err := backend.Navigate(ctx, "https://www.zhihu.com", "")
	if err != nil {
		t.Fatalf("Navigate() error = %v", err)
	}
	if nav.TargetID == "" {
		t.Fatal("Navigate() returned empty target id")
	}
	if relay.startCalls != 1 {
		t.Fatalf("relay start calls = %d, want 1", relay.startCalls)
	}
	if managed.navigateCalls != 1 {
		t.Fatalf("managed navigate calls = %d, want 1", managed.navigateCalls)
	}
}

func TestSitePolicyBrowserBackendRoutesTargetBoundCalls(t *testing.T) {
	ctx := context.Background()
	managed := &policyTestBrowserBackend{cookieValue: "managed"}
	relay := &policyTestBrowserBackend{
		relay:       true,
		cookieValue: "relay",
		tabs: []BrowserTabResult{
			{TargetID: "relay-tab", URL: "https://www.zhihu.com"},
		},
	}
	backend := NewSitePolicyBrowserBackend(
		managed,
		managed,
		relay,
		func(string) bool { return false },
		"managed",
	)

	got, err := backend.CookieHeader(ctx, "relay-tab", "https://www.zhihu.com")
	if err != nil {
		t.Fatalf("CookieHeader() error = %v", err)
	}
	if got != "relay:relay-tab:https://www.zhihu.com" {
		t.Fatalf("CookieHeader() = %q, want relay backend result", got)
	}
	if relay.cookieCalls != 1 {
		t.Fatalf("relay cookie calls = %d, want 1", relay.cookieCalls)
	}
	if managed.cookieCalls != 0 {
		t.Fatalf("managed cookie calls = %d, want 0", managed.cookieCalls)
	}
}
