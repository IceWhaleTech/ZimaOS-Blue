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
	provider rodServiceProvider
}

type rodServiceProvider struct {
	svc     *browser.RodService
	resolve func() *browser.RodService
	acquire func() (*browser.RodService, func(), error)
}

func newLazyBrowserSkillAdapter(resolve func() *browser.RodService) *browserSkillAdapter {
	return &browserSkillAdapter{provider: rodServiceProvider{resolve: resolve}}
}

func newLeaseAwareBrowserSkillAdapter(acquire func() (*browser.RodService, func(), error)) *browserSkillAdapter {
	return &browserSkillAdapter{provider: rodServiceProvider{acquire: acquire}}
}

func (p rodServiceProvider) get() (*browser.RodService, error) {
	if p.svc != nil {
		return p.svc, nil
	}
	if p.resolve != nil {
		if svc := p.resolve(); svc != nil {
			return svc, nil
		}
	}
	return nil, fmt.Errorf("browser service not available")
}

func (p rodServiceProvider) acquireLease() (*browser.RodService, func(), error) {
	if p.acquire != nil {
		svc, release, err := p.acquire()
		if err != nil {
			return nil, nil, err
		}
		if release == nil {
			release = func() {}
		}
		if svc == nil {
			release()
			return nil, nil, fmt.Errorf("browser service not available")
		}
		return svc, release, nil
	}
	svc, err := p.get()
	if err != nil {
		return nil, nil, err
	}
	return svc, func() {}, nil
}

func (a *browserSkillAdapter) get() (*browser.RodService, error) {
	return a.provider.get()
}

func (a *browserSkillAdapter) acquire() (*browser.RodService, func(), error) {
	return a.provider.acquireLease()
}

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

type fallbackBrowserAdapter struct {
	provider rodServiceProvider
}

var _ interface {
	Scrape(context.Context, *browser.ScrapeRequest) (*browser.ScrapeResponse, error)
} = (*fallbackBrowserAdapter)(nil)

func newLeaseAwareFallbackBrowserAdapter(acquire func() (*browser.RodService, func(), error)) *fallbackBrowserAdapter {
	return &fallbackBrowserAdapter{provider: rodServiceProvider{acquire: acquire}}
}

func (a *fallbackBrowserAdapter) acquire() (*browser.RodService, func(), error) {
	return a.provider.acquireLease()
}

func (a *fallbackBrowserAdapter) Start(ctx context.Context) error {
	svc, release, err := a.acquire()
	if err != nil {
		return err
	}
	defer release()
	return svc.Start(ctx)
}

func (a *fallbackBrowserAdapter) OpenTab(ctx context.Context, url string) (*browser.Tab, error) {
	svc, release, err := a.acquire()
	if err != nil {
		return nil, err
	}
	defer release()
	return svc.OpenTab(ctx, url)
}

func (a *fallbackBrowserAdapter) CloseTab(ctx context.Context, targetID string) error {
	svc, release, err := a.acquire()
	if err != nil {
		return err
	}
	defer release()
	return svc.CloseTab(ctx, targetID)
}

func (a *fallbackBrowserAdapter) Screenshot(ctx context.Context, req *browser.ScreenshotRequest) (*browser.ScreenshotResponse, error) {
	svc, release, err := a.acquire()
	if err != nil {
		return nil, err
	}
	defer release()
	return svc.Screenshot(ctx, req)
}

func (a *fallbackBrowserAdapter) Scrape(ctx context.Context, req *browser.ScrapeRequest) (*browser.ScrapeResponse, error) {
	svc, release, err := a.acquire()
	if err != nil {
		return nil, err
	}
	defer release()
	return svc.Scrape(ctx, req)
}

func (a *fallbackBrowserAdapter) ElementExists(ctx context.Context, targetID, selector string) (bool, error) {
	svc, release, err := a.acquire()
	if err != nil {
		return false, err
	}
	defer release()
	return svc.ElementExists(ctx, targetID, selector)
}

func (a *fallbackBrowserAdapter) ExtractFirstFromTab(ctx context.Context, targetID, selector, attribute string) (string, error) {
	svc, release, err := a.acquire()
	if err != nil {
		return "", err
	}
	defer release()
	return svc.ExtractFirstFromTab(ctx, targetID, selector, attribute)
}

func (a *fallbackBrowserAdapter) Act(ctx context.Context, req *browser.ActRequest) (*browser.ActResponse, error) {
	svc, release, err := a.acquire()
	if err != nil {
		return nil, err
	}
	defer release()
	return svc.Act(ctx, req)
}

func (a *fallbackBrowserAdapter) PageInfo(ctx context.Context, targetID string) (string, string, error) {
	svc, release, err := a.acquire()
	if err != nil {
		return "", "", err
	}
	defer release()
	return svc.PageInfo(ctx, targetID)
}
