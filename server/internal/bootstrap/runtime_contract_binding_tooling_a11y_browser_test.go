//go:build darwin || windows

package bootstrap

import (
	"context"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type stubA11yBrowserBackend struct{}

func (stubA11yBrowserBackend) Start(context.Context) error { return nil }
func (stubA11yBrowserBackend) Navigate(context.Context, string, string) (tools.BrowserNavResult, error) {
	return tools.BrowserNavResult{}, nil
}
func (stubA11yBrowserBackend) CookieHeader(context.Context, string, string) (string, error) {
	return "", nil
}
func (stubA11yBrowserBackend) ObserveNetwork(context.Context, string, int, bool) (tools.BrowserObservedNetworkResult, error) {
	return tools.BrowserObservedNetworkResult{}, nil
}
func (stubA11yBrowserBackend) WaitNetworkIdle(context.Context, string, int, int) error { return nil }
func (stubA11yBrowserBackend) AccessibilityTree(context.Context, string, int) (tools.BrowserA11yTreeResult, error) {
	return tools.BrowserA11yTreeResult{}, nil
}
func (stubA11yBrowserBackend) InteractiveElements(context.Context, string) (tools.BrowserInteractiveResult, error) {
	return tools.BrowserInteractiveResult{}, nil
}
func (stubA11yBrowserBackend) CountInteractiveElements(context.Context, string) (int, error) {
	return 0, nil
}
func (stubA11yBrowserBackend) ActByRef(context.Context, string, int, map[int]int, string, string) error {
	return nil
}
func (stubA11yBrowserBackend) ActByInteractiveRef(context.Context, string, int, map[int]string, string, string) error {
	return nil
}
func (stubA11yBrowserBackend) Screenshot(context.Context, string) (string, error)    { return "", nil }
func (stubA11yBrowserBackend) ScreenshotTab(context.Context, string) (string, error) { return "", nil }
func (stubA11yBrowserBackend) CloseTab(context.Context, string) error                { return nil }
func (stubA11yBrowserBackend) Tabs(context.Context) ([]tools.BrowserTabResult, error) {
	return nil, nil
}
func (stubA11yBrowserBackend) ExecuteRecipe(context.Context, string, map[string]string) (tools.BrowserRecipeResult, error) {
	return tools.BrowserRecipeResult{}, nil
}
func (stubA11yBrowserBackend) ListRecipes(context.Context) []tools.BrowserRecipeInfo { return nil }

func TestBindTooling_InjectsBrowserBackendIntoComputerUseTool(t *testing.T) {
	registry := tools.NewRegistry()
	binding := &runtimeContractBinding{}

	binding.BindTooling(routeRuntimeContractToolingOptions{
		registry:       registry,
		browserBackend: stubA11yBrowserBackend{},
	})

	tool, ok := registry.Get("computer_use").(*tools.A11yTool)
	if !ok || tool == nil {
		t.Fatalf("registry.Get(computer_use) = %#v, want *tools.A11yTool", registry.Get("computer_use"))
	}
	if tool.BrowserBackend() == nil {
		t.Fatal("expected browser backend to be injected into computer_use tool")
	}
}
