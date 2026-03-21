package bootstrap

import (
	"context"
	"fmt"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill/builtin"
)

// browserSkillAdapter adapts browser.RodService to builtin.BrowserServiceInterface.
// Keeping this adapter in bootstrap avoids a browser<->builtin package cycle.
type browserSkillAdapter struct {
	svc     *browser.RodService
	resolve func() *browser.RodService
}

func newLazyBrowserSkillAdapter(resolve func() *browser.RodService) *browserSkillAdapter {
	return &browserSkillAdapter{resolve: resolve}
}

func (a *browserSkillAdapter) get() (*browser.RodService, error) {
	if a.svc != nil {
		return a.svc, nil
	}
	if a.resolve != nil {
		if svc := a.resolve(); svc != nil {
			return svc, nil
		}
	}
	return nil, fmt.Errorf("browser service not available")
}

func (a *browserSkillAdapter) Start(ctx context.Context) error {
	svc, err := a.get()
	if err != nil {
		return err
	}
	return svc.Start(ctx)
}

func (a *browserSkillAdapter) Navigate(ctx context.Context, url string, targetID string) (builtin.BrowserNavResult, error) {
	svc, err := a.get()
	if err != nil {
		return builtin.BrowserNavResult{}, err
	}
	resp, err := svc.Navigate(ctx, &browser.NavigateRequest{
		URL:      url,
		TargetID: targetID,
	})
	if err != nil {
		return builtin.BrowserNavResult{}, err
	}
	return builtin.BrowserNavResult{
		URL:      resp.URL,
		Title:    resp.Title,
		TargetID: resp.TargetID,
	}, nil
}

func (a *browserSkillAdapter) AccessibilityTree(ctx context.Context, targetID string, maxDepth int) (builtin.BrowserA11yResult, error) {
	svc, err := a.get()
	if err != nil {
		return builtin.BrowserA11yResult{}, err
	}
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
	svc, err := a.get()
	if err != nil {
		return builtin.BrowserInteractiveResult{}, err
	}
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
	svc, err := a.get()
	if err != nil {
		return 0, err
	}
	return svc.CountInteractiveElements(ctx, targetID)
}

func (a *browserSkillAdapter) ActByRef(ctx context.Context, targetID string, ref int, refMap map[int]int, action string, value string) error {
	svc, err := a.get()
	if err != nil {
		return err
	}
	_, err = svc.ActByRef(ctx, targetID, ref, refMap, action, value)
	return err
}

func (a *browserSkillAdapter) ActByInteractiveRef(ctx context.Context, targetID string, ref int, refMap map[int]string, action string, value string) error {
	svc, err := a.get()
	if err != nil {
		return err
	}
	_, err = svc.ActByInteractiveRef(ctx, targetID, ref, refMap, action, value)
	return err
}

func (a *browserSkillAdapter) Screenshot(ctx context.Context, url string) (string, error) {
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

func (a *browserSkillAdapter) ScreenshotTab(ctx context.Context, targetID string) (string, error) {
	svc, err := a.get()
	if err != nil {
		return "", err
	}
	return svc.ScreenshotTab(ctx, targetID)
}

func (a *browserSkillAdapter) CloseTab(ctx context.Context, targetID string) error {
	svc, err := a.get()
	if err != nil {
		return err
	}
	return svc.CloseTab(ctx, targetID)
}

func (a *browserSkillAdapter) Tabs(ctx context.Context) ([]builtin.BrowserTabInfo, error) {
	svc, err := a.get()
	if err != nil {
		return nil, err
	}
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
	svc, err := a.get()
	if err != nil {
		return builtin.BrowserRecipeResult{}, err
	}
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
	svc, err := a.get()
	if err != nil {
		return nil
	}
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
