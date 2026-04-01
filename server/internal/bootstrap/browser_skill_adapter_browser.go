package bootstrap

import (
	"context"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill/builtin"
)

// browserSkillAdapter adapts browser.RodService to builtin.BrowserServiceInterface.
// Keeping this adapter in bootstrap avoids a browser<->builtin package cycle.
type browserSkillAdapter struct {
	provider rodServiceProvider
}

func newLazyBrowserSkillAdapter(resolve func() *browser.RodService) *browserSkillAdapter {
	return &browserSkillAdapter{provider: rodServiceProvider{resolve: resolve}}
}

func newLeaseAwareBrowserSkillAdapter(acquire func() (*browser.RodService, func(), error)) *browserSkillAdapter { return &browserSkillAdapter{provider: rodServiceProvider{acquire: acquire}} }

func (a *browserSkillAdapter) get() (*browser.RodService, error) { return a.provider.get() }

func (a *browserSkillAdapter) acquire() (*browser.RodService, func(), error) { return a.provider.acquireLease() }

func (a *browserSkillAdapter) Start(ctx context.Context) error {
	svc, release, err := a.acquire()
	if err != nil {
		return err
	}
	defer release()
	return svc.Start(ctx)
}

func (a *browserSkillAdapter) Navigate(ctx context.Context, url string, targetID string) (builtin.BrowserNavResult, error) {
	svc, release, err := a.acquire()
	if err != nil {
		return builtin.BrowserNavResult{}, err
	}
	defer release()
	resp, err := svc.Navigate(ctx, &browser.NavigateRequest{URL: url, TargetID: targetID})
	if err != nil {
		return builtin.BrowserNavResult{}, err
	}
	return builtin.BrowserNavResult{
		URL:      resp.URL,
		Title:    resp.Title,
		TargetID: resp.TargetID,
	}, nil
}

func (a *browserSkillAdapter) ExtractText(ctx context.Context, targetID, selector string) (string, error) {
	svc, release, err := a.acquire()
	if err != nil {
		return "", err
	}
	defer release()
	return svc.ExtractFirstFromTab(ctx, targetID, selector, "")
}

func (a *browserSkillAdapter) AccessibilityTree(ctx context.Context, targetID string, maxDepth int) (builtin.BrowserA11yResult, error) {
	svc, release, err := a.acquire()
	if err != nil {
		return builtin.BrowserA11yResult{}, err
	}
	defer release()
	resp, err := svc.AccessibilityTree(ctx, targetID, maxDepth)
	if err != nil {
		return builtin.BrowserA11yResult{}, err
	}
	return builtin.BrowserA11yResult{
		Tree:     resp.Tree,
		URL:      resp.URL,
		Title:    resp.Title,
		TargetID: resp.TargetID,
		RefMap:   resp.RefMap,
	}, nil
}

func (a *browserSkillAdapter) InteractiveElements(ctx context.Context, targetID string) (builtin.BrowserInteractiveResult, error) {
	svc, release, err := a.acquire()
	if err != nil {
		return builtin.BrowserInteractiveResult{}, err
	}
	defer release()
	resp, err := svc.InteractiveElements(ctx, targetID)
	if err != nil {
		return builtin.BrowserInteractiveResult{}, err
	}
	return builtin.BrowserInteractiveResult{
		Tree:     resp.Tree,
		URL:      resp.URL,
		Title:    resp.Title,
		TargetID: resp.TargetID,
		RefMap:   resp.RefMap,
		Count:    resp.Count,
	}, nil
}

func (a *browserSkillAdapter) CountInteractiveElements(ctx context.Context, targetID string) (int, error) {
	svc, release, err := a.acquire()
	if err != nil {
		return 0, err
	}
	defer release()
	return svc.CountInteractiveElements(ctx, targetID)
}

func (a *browserSkillAdapter) ActByRef(ctx context.Context, targetID string, ref int, refMap map[int]int, action string, value string) error {
	svc, release, err := a.acquire()
	if err != nil {
		return err
	}
	defer release()
	_, err = svc.ActByRef(ctx, targetID, ref, refMap, action, value)
	return err
}

func (a *browserSkillAdapter) ActByInteractiveRef(ctx context.Context, targetID string, ref int, refMap map[int]string, action string, value string) error {
	svc, release, err := a.acquire()
	if err != nil {
		return err
	}
	defer release()
	_, err = svc.ActByInteractiveRef(ctx, targetID, ref, refMap, action, value)
	return err
}

func (a *browserSkillAdapter) Screenshot(ctx context.Context, url string) (string, error) {
	svc, release, err := a.acquire()
	if err != nil {
		return "", err
	}
	defer release()
	resp, err := svc.Screenshot(ctx, &browser.ScreenshotRequest{URL: url})
	if err != nil {
		return "", err
	}
	return resp.Data, nil
}

func (a *browserSkillAdapter) ScreenshotTab(ctx context.Context, targetID string) (string, error) {
	svc, release, err := a.acquire()
	if err != nil {
		return "", err
	}
	defer release()
	return svc.ScreenshotTab(ctx, targetID)
}

func (a *browserSkillAdapter) CloseTab(ctx context.Context, targetID string) error {
	svc, release, err := a.acquire()
	if err != nil {
		return err
	}
	defer release()
	return svc.CloseTab(ctx, targetID)
}

func (a *browserSkillAdapter) Tabs(ctx context.Context) ([]builtin.BrowserTabInfo, error) {
	svc, release, err := a.acquire()
	if err != nil {
		return nil, err
	}
	defer release()
	tabs, err := svc.Tabs(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]builtin.BrowserTabInfo, len(tabs))
	for i, t := range tabs {
		result[i] = builtin.BrowserTabInfo{
			TargetID: t.TargetID,
			URL:      t.URL,
			Title:    t.Title,
			Active:   t.Active,
		}
	}
	return result, nil
}

func (a *browserSkillAdapter) ExecuteRecipe(ctx context.Context, recipe string, params map[string]string) (builtin.BrowserRecipeResult, error) {
	svc, release, err := a.acquire()
	if err != nil {
		return builtin.BrowserRecipeResult{}, err
	}
	defer release()
	resp, err := svc.ExecuteRecipe(ctx, &browser.RecipeRequest{Recipe: recipe, Params: params})
	if err != nil {
		return builtin.BrowserRecipeResult{}, err
	}
	return builtin.BrowserRecipeResult{
		Success:  resp.Success,
		Data:     resp.Data,
		TargetID: resp.TargetID,
		Message:  resp.Message,
	}, nil
}

func (a *browserSkillAdapter) ListRecipes(ctx context.Context) []builtin.BrowserRecipeInfo {
	svc, release, err := a.acquire()
	if err != nil {
		return nil
	}
	defer release()
	infos := svc.Recipes().List()
	result := make([]builtin.BrowserRecipeInfo, len(infos))
	for i, info := range infos {
		result[i] = builtin.BrowserRecipeInfo{
			Name:        info.Name,
			Description: info.Description,
			KeepTab:     info.KeepTab,
		}
	}
	return result
}
