package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
)

// RodBrowserBackend adapts browser.RodService to the BrowserBackend interface.
// Supports lazy initialization and mode-aware routing via context.
type RodBrowserBackend struct {
	defaultSvc     *browser.RodService
	visibleSvc     *browser.RodService
	resolveDefault func() *browser.RodService
	resolveVisible func() *browser.RodService
}

// NewRodBrowserBackend creates a new adapter with a direct service reference.
func NewRodBrowserBackend(svc *browser.RodService) *RodBrowserBackend {
	return &RodBrowserBackend{defaultSvc: svc}
}

// NewLazyRodBrowserBackend creates a lazy adapter that resolves the default service on first use.
func NewLazyRodBrowserBackend(resolve func() *browser.RodService) *RodBrowserBackend {
	return &RodBrowserBackend{resolveDefault: resolve}
}

// NewModeAwareRodBrowserBackend creates an adapter that can route requests to
// different browser services based on per-request context or tab affinity.
func NewModeAwareRodBrowserBackend(resolveDefault, resolveVisible func() *browser.RodService) *RodBrowserBackend {
	return &RodBrowserBackend{resolveDefault: resolveDefault, resolveVisible: resolveVisible}
}

func (a *RodBrowserBackend) getDefault() (*browser.RodService, error) {
	if a.defaultSvc != nil {
		return a.defaultSvc, nil
	}
	if a.resolveDefault != nil {
		a.defaultSvc = a.resolveDefault()
	}
	if a.defaultSvc == nil {
		return nil, fmt.Errorf("browser service not available")
	}
	return a.defaultSvc, nil
}

func (a *RodBrowserBackend) getVisible() (*browser.RodService, error) {
	if a.visibleSvc != nil {
		return a.visibleSvc, nil
	}
	if a.resolveVisible != nil {
		a.visibleSvc = a.resolveVisible()
	}
	if a.visibleSvc == nil {
		return nil, nil
	}
	return a.visibleSvc, nil
}

func browserHasTarget(ctx context.Context, svc *browser.RodService, targetID string) bool {
	targetID = strings.TrimSpace(targetID)
	if svc == nil || targetID == "" {
		return false
	}
	tabs, err := svc.Tabs(ctx)
	if err != nil {
		return false
	}
	for _, tab := range tabs {
		if strings.TrimSpace(tab.TargetID) == targetID {
			return true
		}
	}
	return false
}

func (a *RodBrowserBackend) getForTarget(ctx context.Context, targetID string) (*browser.RodService, error) {
	defaultSvc, defaultErr := a.getDefault()
	visibleSvc, _ := a.getVisible()
	mode := GetBrowserLaunchMode(ctx)
	targetID = strings.TrimSpace(targetID)

	if targetID != "" {
		if mode == BrowserLaunchModeVisible {
			if browserHasTarget(ctx, visibleSvc, targetID) {
				return visibleSvc, nil
			}
			if browserHasTarget(ctx, defaultSvc, targetID) {
				return defaultSvc, nil
			}
			if visibleSvc != nil {
				return visibleSvc, nil
			}
			if defaultSvc != nil {
				return defaultSvc, nil
			}
			return nil, defaultErr
		}
		if browserHasTarget(ctx, defaultSvc, targetID) {
			return defaultSvc, nil
		}
		if browserHasTarget(ctx, visibleSvc, targetID) {
			return visibleSvc, nil
		}
	}

	if mode == BrowserLaunchModeVisible && visibleSvc != nil {
		return visibleSvc, nil
	}
	if defaultSvc != nil {
		return defaultSvc, nil
	}
	if visibleSvc != nil {
		return visibleSvc, nil
	}
	if defaultErr != nil {
		return nil, defaultErr
	}
	return nil, fmt.Errorf("browser service not available")
}

func (a *RodBrowserBackend) Start(ctx context.Context) error {
	svc, err := a.getForTarget(ctx, "")
	if err != nil {
		return err
	}
	return svc.Start(ctx)
}

func (a *RodBrowserBackend) Navigate(ctx context.Context, url string, targetID string) (BrowserNavResult, error) {
	svc, err := a.getForTarget(ctx, targetID)
	if err != nil {
		return BrowserNavResult{}, err
	}
	resp, err := svc.Navigate(ctx, &browser.NavigateRequest{URL: url, TargetID: targetID})
	if err != nil {
		return BrowserNavResult{}, err
	}
	return BrowserNavResult{URL: resp.URL, Title: resp.Title, TargetID: resp.TargetID}, nil
}

func (a *RodBrowserBackend) CookieHeader(ctx context.Context, targetID string, url string) (string, error) {
	svc, err := a.getForTarget(ctx, targetID)
	if err != nil {
		return "", err
	}
	return svc.CookieHeader(ctx, targetID, url)
}

func (a *RodBrowserBackend) AccessibilityTree(ctx context.Context, targetID string, maxDepth int) (BrowserA11yTreeResult, error) {
	svc, err := a.getForTarget(ctx, targetID)
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
	svc, err := a.getForTarget(ctx, targetID)
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
	svc, err := a.getForTarget(ctx, targetID)
	if err != nil {
		return 0, err
	}
	return svc.CountInteractiveElements(ctx, targetID)
}

func (a *RodBrowserBackend) ActByRef(ctx context.Context, targetID string, ref int, refMap map[int]int, action string, value string) error {
	svc, err := a.getForTarget(ctx, targetID)
	if err != nil {
		return err
	}
	_, err = svc.ActByRef(ctx, targetID, ref, refMap, action, value)
	return err
}

func (a *RodBrowserBackend) ActByInteractiveRef(ctx context.Context, targetID string, ref int, refMap map[int]string, action string, value string) error {
	svc, err := a.getForTarget(ctx, targetID)
	if err != nil {
		return err
	}
	_, err = svc.ActByInteractiveRef(ctx, targetID, ref, refMap, action, value)
	return err
}

func (a *RodBrowserBackend) Screenshot(ctx context.Context, url string) (string, error) {
	svc, err := a.getForTarget(ctx, "")
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
	svc, err := a.getForTarget(ctx, targetID)
	if err != nil {
		return "", err
	}
	return svc.ScreenshotTab(ctx, targetID)
}

func (a *RodBrowserBackend) CloseTab(ctx context.Context, targetID string) error {
	svc, err := a.getForTarget(ctx, targetID)
	if err != nil {
		return err
	}
	return svc.CloseTab(ctx, targetID)
}

func (a *RodBrowserBackend) Tabs(ctx context.Context) ([]BrowserTabResult, error) {
	defaultSvc, defaultErr := a.getDefault()
	visibleSvc, _ := a.getVisible()
	seen := make(map[string]struct{})
	result := make([]BrowserTabResult, 0, 8)
	appendTabs := func(tabs []*browser.Tab) {
		for _, tab := range tabs {
			if tab == nil {
				continue
			}
			key := strings.TrimSpace(tab.TargetID)
			if key != "" {
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
			}
			result = append(result, BrowserTabResult{TargetID: tab.TargetID, URL: tab.URL, Title: tab.Title, Active: tab.Active})
		}
	}
	if defaultSvc != nil {
		if tabs, err := defaultSvc.Tabs(ctx); err == nil {
			appendTabs(tabs)
		}
	}
	if visibleSvc != nil {
		if tabs, err := visibleSvc.Tabs(ctx); err == nil {
			appendTabs(tabs)
		}
	}
	if len(result) > 0 {
		return result, nil
	}
	if defaultErr != nil {
		return nil, defaultErr
	}
	return result, nil
}

func (a *RodBrowserBackend) ExecuteRecipe(ctx context.Context, recipe string, params map[string]string) (BrowserRecipeResult, error) {
	svc, err := a.getForTarget(ctx, "")
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
	svc, err := a.getForTarget(ctx, "")
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
