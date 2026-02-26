package tools

import (
	"context"
	"fmt"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
)

// RodBrowserBackend adapts browser.RodService to the BrowserBackend interface.
// Supports lazy initialization via a resolve function.
type RodBrowserBackend struct {
	svc     *browser.RodService
	resolve func() *browser.RodService
}

// NewRodBrowserBackend creates a new adapter with a direct service reference.
func NewRodBrowserBackend(svc *browser.RodService) *RodBrowserBackend {
	return &RodBrowserBackend{svc: svc}
}

// NewLazyRodBrowserBackend creates a lazy adapter that resolves the service on first use.
func NewLazyRodBrowserBackend(resolve func() *browser.RodService) *RodBrowserBackend {
	return &RodBrowserBackend{resolve: resolve}
}

func (a *RodBrowserBackend) get() (*browser.RodService, error) {
	if a.svc != nil {
		return a.svc, nil
	}
	if a.resolve != nil {
		a.svc = a.resolve()
	}
	if a.svc == nil {
		return nil, fmt.Errorf("browser service not available")
	}
	return a.svc, nil
}

func (a *RodBrowserBackend) Start(ctx context.Context) error {
	svc, err := a.get()
	if err != nil {
		return err
	}
	return svc.Start(ctx)
}

func (a *RodBrowserBackend) Navigate(ctx context.Context, url string, targetID string) (BrowserNavResult, error) {
	svc, err := a.get()
	if err != nil {
		return BrowserNavResult{}, err
	}
	resp, err := svc.Navigate(ctx, &browser.NavigateRequest{URL: url, TargetID: targetID})
	if err != nil {
		return BrowserNavResult{}, err
	}
	return BrowserNavResult{URL: resp.URL, Title: resp.Title, TargetID: resp.TargetID}, nil
}

func (a *RodBrowserBackend) AccessibilityTree(ctx context.Context, targetID string, maxDepth int) (BrowserA11yTreeResult, error) {
	svc, err := a.get()
	if err != nil {
		return BrowserA11yTreeResult{}, err
	}
	resp, err := svc.AccessibilityTree(ctx, targetID, maxDepth)
	if err != nil {
		return BrowserA11yTreeResult{}, err
	}
	return BrowserA11yTreeResult{
		Tree: resp.Tree, URL: resp.URL, Title: resp.Title,
		TargetID: resp.TargetID, RefMap: resp.RefMap,
	}, nil
}

func (a *RodBrowserBackend) InteractiveElements(ctx context.Context, targetID string) (BrowserInteractiveResult, error) {
	svc, err := a.get()
	if err != nil {
		return BrowserInteractiveResult{}, err
	}
	resp, err := svc.InteractiveElements(ctx, targetID)
	if err != nil {
		return BrowserInteractiveResult{}, err
	}
	return BrowserInteractiveResult{
		Tree: resp.Tree, URL: resp.URL, Title: resp.Title,
		TargetID: resp.TargetID, RefMap: resp.RefMap, Count: resp.Count,
	}, nil
}

func (a *RodBrowserBackend) CountInteractiveElements(ctx context.Context, targetID string) (int, error) {
	svc, err := a.get()
	if err != nil {
		return 0, err
	}
	return svc.CountInteractiveElements(ctx, targetID)
}

func (a *RodBrowserBackend) ActByRef(ctx context.Context, targetID string, ref int, refMap map[int]int, action string, value string) error {
	svc, err := a.get()
	if err != nil {
		return err
	}
	_, err = svc.ActByRef(ctx, targetID, ref, refMap, action, value)
	return err
}

func (a *RodBrowserBackend) ActByInteractiveRef(ctx context.Context, targetID string, ref int, refMap map[int]string, action string, value string) error {
	svc, err := a.get()
	if err != nil {
		return err
	}
	_, err = svc.ActByInteractiveRef(ctx, targetID, ref, refMap, action, value)
	return err
}

func (a *RodBrowserBackend) Screenshot(ctx context.Context, url string) (string, error) {
	svc, err := a.get()
	if err != nil {
		return "", err
	}
	resp, err := svc.Screenshot(ctx, &browser.ScreenshotRequest{URL: url})
	if err != nil {
		return "", err
	}
	return resp.Data, nil
}

func (a *RodBrowserBackend) ScreenshotTab(ctx context.Context, targetID string) (string, error) {
	svc, err := a.get()
	if err != nil {
		return "", err
	}
	return svc.ScreenshotTab(ctx, targetID)
}

func (a *RodBrowserBackend) CloseTab(ctx context.Context, targetID string) error {
	svc, err := a.get()
	if err != nil {
		return err
	}
	return svc.CloseTab(ctx, targetID)
}

func (a *RodBrowserBackend) Tabs(ctx context.Context) ([]BrowserTabResult, error) {
	svc, err := a.get()
	if err != nil {
		return nil, err
	}
	tabs, err := svc.Tabs(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]BrowserTabResult, len(tabs))
	for i, t := range tabs {
		result[i] = BrowserTabResult{
			TargetID: t.TargetID, URL: t.URL, Title: t.Title, Active: t.Active,
		}
	}
	return result, nil
}

func (a *RodBrowserBackend) ExecuteRecipe(ctx context.Context, recipe string, params map[string]string) (BrowserRecipeResult, error) {
	svc, err := a.get()
	if err != nil {
		return BrowserRecipeResult{}, err
	}
	resp, err := svc.ExecuteRecipe(ctx, &browser.RecipeRequest{Recipe: recipe, Params: params})
	if err != nil {
		return BrowserRecipeResult{}, err
	}
	return BrowserRecipeResult{
		Success:  resp.Success,
		Data:     resp.Data,
		TargetID: resp.TargetID,
		Message:  resp.Message,
	}, nil
}

func (a *RodBrowserBackend) ListRecipes(ctx context.Context) []BrowserRecipeInfo {
	svc, err := a.get()
	if err != nil {
		return nil
	}
	infos := svc.Recipes().List()
	result := make([]BrowserRecipeInfo, len(infos))
	for i, info := range infos {
		result[i] = BrowserRecipeInfo{
			Name:        info.Name,
			Description: info.Description,
			KeepTab:     info.KeepTab,
		}
	}
	return result
}
