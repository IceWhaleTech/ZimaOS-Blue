package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
)

type rodServiceAcquireFunc func() (*browser.RodService, func(), error)

type rodServiceSource struct {
	svc     *browser.RodService
	resolve func() *browser.RodService
	acquire rodServiceAcquireFunc
}

type rodServiceLease struct {
	svc     *browser.RodService
	release func()
}

func (l rodServiceLease) close() {
	if l.release != nil {
		l.release()
	}
}

func acquireRodServiceSource(source rodServiceSource, required bool) (rodServiceLease, error) {
	switch {
	case source.acquire != nil:
		svc, release, err := source.acquire()
		if err != nil {
			return rodServiceLease{}, err
		}
		if release == nil {
			release = func() {}
		}
		if svc == nil {
			release()
		} else {
			return rodServiceLease{svc: svc, release: release}, nil
		}
	case source.svc != nil:
		return rodServiceLease{svc: source.svc, release: func() {}}, nil
	case source.resolve != nil:
		if svc := source.resolve(); svc != nil {
			return rodServiceLease{svc: svc, release: func() {}}, nil
		}
	}

	if !required {
		return rodServiceLease{}, nil
	}
	return rodServiceLease{}, fmt.Errorf("browser service not available")
}

// RodBrowserBackend adapts browser.RodService to the BrowserBackend interface.
// Supports lazy initialization and mode-aware routing via context.
type RodBrowserBackend struct {
	defaultSource rodServiceSource
	visibleSource rodServiceSource
}

// NewRodBrowserBackend creates a new adapter with a direct service reference.
func NewRodBrowserBackend(svc *browser.RodService) *RodBrowserBackend {
	return &RodBrowserBackend{defaultSource: rodServiceSource{svc: svc}}
}

// NewLazyRodBrowserBackend creates a lazy adapter that resolves the default service on demand.
func NewLazyRodBrowserBackend(resolve func() *browser.RodService) *RodBrowserBackend {
	return &RodBrowserBackend{defaultSource: rodServiceSource{resolve: resolve}}
}

// NewModeAwareRodBrowserBackend creates an adapter that can route requests to
// different browser services based on per-request context or tab affinity.
func NewModeAwareRodBrowserBackend(resolveDefault, resolveVisible func() *browser.RodService) *RodBrowserBackend {
	return &RodBrowserBackend{
		defaultSource: rodServiceSource{resolve: resolveDefault},
		visibleSource: rodServiceSource{resolve: resolveVisible},
	}
}

// NewLeaseAwareRodBrowserBackend creates an adapter that keeps a managed browser
// instance alive for the duration of each backend call.
func NewLeaseAwareRodBrowserBackend(acquireDefault, acquireVisible rodServiceAcquireFunc) *RodBrowserBackend {
	return &RodBrowserBackend{
		defaultSource: rodServiceSource{acquire: acquireDefault},
		visibleSource: rodServiceSource{acquire: acquireVisible},
	}
}

func (a *RodBrowserBackend) acquireSource(source rodServiceSource, required bool) (rodServiceLease, error) {
	return acquireRodServiceSource(source, required)
}

func (a *RodBrowserBackend) acquireDefault() (rodServiceLease, error) {
	return a.acquireSource(a.defaultSource, true)
}

func (a *RodBrowserBackend) acquireVisible() (rodServiceLease, error) {
	return a.acquireSource(a.visibleSource, false)
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

func browserServiceUnavailable(defaultErr error) error {
	if defaultErr != nil {
		return defaultErr
	}
	return fmt.Errorf("browser service not available")
}

func (a *RodBrowserBackend) acquireForTarget(ctx context.Context, targetID string) (rodServiceLease, error) {
	mode := GetBrowserLaunchMode(ctx)
	targetID = strings.TrimSpace(targetID)

	if mode == BrowserLaunchModeVisible {
		visibleLease, _ := a.acquireVisible()
		if targetID == "" {
			defaultLease, defaultErr := a.acquireDefault()
			if visibleLease.svc != nil {
				defaultLease.close()
				return visibleLease, nil
			}
			visibleLease.close()
			if defaultLease.svc != nil {
				return defaultLease, nil
			}
			defaultLease.close()
			return rodServiceLease{}, browserServiceUnavailable(defaultErr)
		}

		if browserHasTarget(ctx, visibleLease.svc, targetID) {
			return visibleLease, nil
		}

		defaultLease, defaultErr := a.acquireDefault()
		if browserHasTarget(ctx, defaultLease.svc, targetID) {
			visibleLease.close()
			return defaultLease, nil
		}
		if visibleLease.svc != nil {
			defaultLease.close()
			return visibleLease, nil
		}
		if defaultLease.svc != nil {
			visibleLease.close()
			return defaultLease, nil
		}
		visibleLease.close()
		defaultLease.close()
		return rodServiceLease{}, browserServiceUnavailable(defaultErr)
	}

	defaultLease, defaultErr := a.acquireDefault()
	if targetID == "" {
		if defaultLease.svc != nil {
			return defaultLease, nil
		}
		defaultLease.close()
		return rodServiceLease{}, browserServiceUnavailable(defaultErr)
	}

	if browserHasTarget(ctx, defaultLease.svc, targetID) {
		return defaultLease, nil
	}

	visibleLease, _ := a.acquireVisible()
	if browserHasTarget(ctx, visibleLease.svc, targetID) {
		defaultLease.close()
		return visibleLease, nil
	}
	if defaultLease.svc != nil {
		visibleLease.close()
		return defaultLease, nil
	}
	if visibleLease.svc != nil {
		defaultLease.close()
		return visibleLease, nil
	}
	defaultLease.close()
	visibleLease.close()
	return rodServiceLease{}, browserServiceUnavailable(defaultErr)
}

func (a *RodBrowserBackend) Start(ctx context.Context) error {
	lease, err := a.acquireForTarget(ctx, "")
	if err != nil {
		return err
	}
	defer lease.close()
	return lease.svc.Start(ctx)
}

// UsesRelay reports whether the selected browser service is running in relay/CDP attach mode.
func (a *RodBrowserBackend) UsesRelay(ctx context.Context, targetID string) bool {
	lease, err := a.acquireForTarget(ctx, targetID)
	if err != nil || lease.svc == nil {
		return false
	}
	defer lease.close()
	return lease.svc.UsesRelayDriver()
}

func (a *RodBrowserBackend) Navigate(ctx context.Context, url string, targetID string) (BrowserNavResult, error) {
	lease, err := a.acquireForTarget(ctx, targetID)
	if err != nil {
		return BrowserNavResult{}, err
	}
	defer lease.close()
	resp, err := lease.svc.Navigate(ctx, &browser.NavigateRequest{URL: url, TargetID: targetID})
	if err != nil {
		return BrowserNavResult{}, err
	}
	return BrowserNavResult{URL: resp.URL, Title: resp.Title, TargetID: resp.TargetID}, nil
}

func (a *RodBrowserBackend) CookieHeader(ctx context.Context, targetID string, url string) (string, error) {
	lease, err := a.acquireForTarget(ctx, targetID)
	if err != nil {
		return "", err
	}
	defer lease.close()
	return lease.svc.CookieHeader(ctx, targetID, url)
}

func (a *RodBrowserBackend) ObserveNetwork(ctx context.Context, targetID string, maxEntries int, clear bool) (BrowserObservedNetworkResult, error) {
	lease, err := a.acquireForTarget(ctx, targetID)
	if err != nil {
		return BrowserObservedNetworkResult{}, err
	}
	defer lease.close()
	resp, err := lease.svc.ObserveNetwork(ctx, targetID, maxEntries, clear)
	if err != nil {
		return BrowserObservedNetworkResult{}, err
	}
	result := BrowserObservedNetworkResult{
		TargetID: resp.TargetID,
		Events:   make([]BrowserNetworkEvent, 0, len(resp.Events)),
	}
	for _, event := range resp.Events {
		result.Events = append(result.Events, BrowserNetworkEvent{
			Method:       event.Method,
			URL:          event.URL,
			Status:       event.Status,
			ContentType:  event.ContentType,
			ResourceType: event.ResourceType,
			Initiator:    event.Initiator,
			DurationMS:   event.DurationMS,
			Headers:      cloneStringMap(event.Headers),
			BodySample:   event.BodySample,
		})
	}
	return result, nil
}

func (a *RodBrowserBackend) WaitNetworkIdle(ctx context.Context, targetID string, idleMS int, timeoutMS int) error {
	lease, err := a.acquireForTarget(ctx, targetID)
	if err != nil {
		return err
	}
	defer lease.close()
	return lease.svc.WaitNetworkIdle(ctx, targetID, idleMS, timeoutMS)
}

func (a *RodBrowserBackend) AccessibilityTree(ctx context.Context, targetID string, maxDepth int) (BrowserA11yTreeResult, error) {
	lease, err := a.acquireForTarget(ctx, targetID)
	if err != nil {
		return BrowserA11yTreeResult{}, err
	}
	defer lease.close()
	resp, err := lease.svc.AccessibilityTree(ctx, targetID, maxDepth)
	if err != nil {
		return BrowserA11yTreeResult{}, err
	}
	return BrowserA11yTreeResult{
		Tree: resp.Tree, URL: resp.URL, Title: resp.Title,
		TargetID: resp.TargetID, RefMap: resp.RefMap,
	}, nil
}

func (a *RodBrowserBackend) InteractiveElements(ctx context.Context, targetID string) (BrowserInteractiveResult, error) {
	lease, err := a.acquireForTarget(ctx, targetID)
	if err != nil {
		return BrowserInteractiveResult{}, err
	}
	defer lease.close()
	resp, err := lease.svc.InteractiveElements(ctx, targetID)
	if err != nil {
		return BrowserInteractiveResult{}, err
	}
	return BrowserInteractiveResult{
		Tree: resp.Tree, URL: resp.URL, Title: resp.Title,
		TargetID: resp.TargetID, RefMap: resp.RefMap, Count: resp.Count,
	}, nil
}

func (a *RodBrowserBackend) CountInteractiveElements(ctx context.Context, targetID string) (int, error) {
	lease, err := a.acquireForTarget(ctx, targetID)
	if err != nil {
		return 0, err
	}
	defer lease.close()
	return lease.svc.CountInteractiveElements(ctx, targetID)
}

func (a *RodBrowserBackend) ActByRef(ctx context.Context, targetID string, ref int, refMap map[int]int, action string, value string) error {
	lease, err := a.acquireForTarget(ctx, targetID)
	if err != nil {
		return err
	}
	defer lease.close()
	_, err = lease.svc.ActByRef(ctx, targetID, ref, refMap, action, value)
	return err
}

func (a *RodBrowserBackend) ActByInteractiveRef(ctx context.Context, targetID string, ref int, refMap map[int]string, action string, value string) error {
	lease, err := a.acquireForTarget(ctx, targetID)
	if err != nil {
		return err
	}
	defer lease.close()
	_, err = lease.svc.ActByInteractiveRef(ctx, targetID, ref, refMap, action, value)
	return err
}

func (a *RodBrowserBackend) Screenshot(ctx context.Context, url string) (string, error) {
	lease, err := a.acquireForTarget(ctx, "")
	if err != nil {
		return "", err
	}
	defer lease.close()
	resp, err := lease.svc.Screenshot(ctx, &browser.ScreenshotRequest{URL: url})
	if err != nil {
		return "", err
	}
	return resp.Data, nil
}

func (a *RodBrowserBackend) ScreenshotTab(ctx context.Context, targetID string) (string, error) {
	lease, err := a.acquireForTarget(ctx, targetID)
	if err != nil {
		return "", err
	}
	defer lease.close()
	return lease.svc.ScreenshotTab(ctx, targetID)
}

func (a *RodBrowserBackend) CloseTab(ctx context.Context, targetID string) error {
	lease, err := a.acquireForTarget(ctx, targetID)
	if err != nil {
		return err
	}
	defer lease.close()
	return lease.svc.CloseTab(ctx, targetID)
}

func (a *RodBrowserBackend) Tabs(ctx context.Context) ([]BrowserTabResult, error) {
	defaultLease, defaultErr := a.acquireDefault()
	visibleLease, _ := a.acquireVisible()
	defer defaultLease.close()
	defer visibleLease.close()

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
	if defaultLease.svc != nil {
		if tabs, err := defaultLease.svc.Tabs(ctx); err == nil {
			appendTabs(tabs)
		}
	}
	if visibleLease.svc != nil {
		if tabs, err := visibleLease.svc.Tabs(ctx); err == nil {
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
	lease, err := a.acquireForTarget(ctx, "")
	if err != nil {
		return BrowserRecipeResult{}, err
	}
	defer lease.close()
	resp, err := lease.svc.ExecuteRecipe(ctx, &browser.RecipeRequest{Recipe: recipe, Params: params})
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
	lease, err := a.acquireForTarget(ctx, "")
	if err != nil {
		return nil
	}
	defer lease.close()
	infos := lease.svc.Recipes().List()
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

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}
