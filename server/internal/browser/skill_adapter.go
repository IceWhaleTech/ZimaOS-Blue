package browser

import (
	"context"
	"fmt"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill/builtin"
)

// SkillAdapter adapts RodService to builtin.BrowserServiceInterface.
type SkillAdapter struct {
	svc     *RodService
	resolve func() *RodService
}

// NewSkillAdapter creates a new skill adapter for the browser service.
func NewSkillAdapter(svc *RodService) *SkillAdapter {
	return &SkillAdapter{svc: svc}
}

// NewLazySkillAdapter creates a lazy skill adapter that resolves the service on first use.
func NewLazySkillAdapter(resolve func() *RodService) *SkillAdapter {
	return &SkillAdapter{resolve: resolve}
}

func (a *SkillAdapter) get() (*RodService, error) {
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

func (a *SkillAdapter) Start(ctx context.Context) error {
	svc, err := a.get()
	if err != nil {
		return err
	}
	return svc.Start(ctx)
}

func (a *SkillAdapter) Stop(ctx context.Context) error {
	svc, err := a.get()
	if err != nil {
		return err
	}
	return svc.Stop(ctx)
}

func (a *SkillAdapter) Navigate(ctx context.Context, url string, targetID string) (builtin.BrowserNavResult, error) {
	svc, err := a.get()
	if err != nil {
		return builtin.BrowserNavResult{}, err
	}
	resp, err := svc.Navigate(ctx, &NavigateRequest{
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

func (a *SkillAdapter) AccessibilityTree(ctx context.Context, targetID string, maxDepth int) (builtin.BrowserA11yResult, error) {
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

func (a *SkillAdapter) InteractiveElements(ctx context.Context, targetID string) (builtin.BrowserInteractiveResult, error) {
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

func (a *SkillAdapter) CountInteractiveElements(ctx context.Context, targetID string) (int, error) {
	svc, err := a.get()
	if err != nil {
		return 0, err
	}
	return svc.CountInteractiveElements(ctx, targetID)
}

func (a *SkillAdapter) ActByRef(ctx context.Context, targetID string, ref int, refMap map[int]int, action string, value string) error {
	svc, err := a.get()
	if err != nil {
		return err
	}
	_, err = svc.ActByRef(ctx, targetID, ref, refMap, action, value)
	return err
}

func (a *SkillAdapter) ActByInteractiveRef(ctx context.Context, targetID string, ref int, refMap map[int]string, action string, value string) error {
	svc, err := a.get()
	if err != nil {
		return err
	}
	_, err = svc.ActByInteractiveRef(ctx, targetID, ref, refMap, action, value)
	return err
}

func (a *SkillAdapter) Screenshot(ctx context.Context, url string) (string, error) {
	svc, err := a.get()
	if err != nil {
		return "", err
	}
	resp, err := svc.Screenshot(ctx, &ScreenshotRequest{URL: url})
	if err != nil {
		return "", err
	}
	return resp.Data, nil
}

func (a *SkillAdapter) ScreenshotTab(ctx context.Context, targetID string) (string, error) {
	svc, err := a.get()
	if err != nil {
		return "", err
	}
	return svc.ScreenshotTab(ctx, targetID)
}

func (a *SkillAdapter) ScreenshotViewport(ctx context.Context, targetID string) (string, error) {
	svc, err := a.get()
	if err != nil {
		return "", err
	}
	return svc.ScreenshotViewport(ctx, targetID)
}

func (a *SkillAdapter) ScreenshotViewportRaw(ctx context.Context, targetID string) ([]byte, error) {
	svc, err := a.get()
	if err != nil {
		return nil, err
	}
	return svc.ScreenshotViewportRaw(ctx, targetID)
}

func (a *SkillAdapter) SetViewport(ctx context.Context, targetID string, width, height int) error {
	svc, err := a.get()
	if err != nil {
		return err
	}
	return svc.SetViewport(ctx, targetID, width, height)
}

func (a *SkillAdapter) ScrollTo(ctx context.Context, targetID string, x, y int) error {
	svc, err := a.get()
	if err != nil {
		return err
	}
	return svc.ScrollTo(ctx, targetID, x, y)
}

func (a *SkillAdapter) PageDimensions(ctx context.Context, targetID string) (int, int, error) {
	svc, err := a.get()
	if err != nil {
		return 0, 0, err
	}
	return svc.PageDimensions(ctx, targetID)
}

func (a *SkillAdapter) CloseTab(ctx context.Context, targetID string) error {
	svc, err := a.get()
	if err != nil {
		return err
	}
	return svc.CloseTab(ctx, targetID)
}

func (a *SkillAdapter) Tabs(ctx context.Context) ([]builtin.BrowserTabInfo, error) {
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
