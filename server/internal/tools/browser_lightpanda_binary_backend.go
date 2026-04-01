package tools

import (
	"context"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
)

type lightpandaBinaryRuntime interface {
	Start(ctx context.Context) (*browser.RodService, error)
	PeekService() *browser.RodService
	Stop(ctx context.Context) error
}

// LightpandaBinaryBrowserBackend adapts the upstream Lightpanda binary into
// Blue's browser-lite session model.
type LightpandaBinaryBrowserBackend struct {
	runtime lightpandaBinaryRuntime
}

// NewLightpandaBinaryBrowserBackend creates a browser-lite backend backed by a
// local upstream Lightpanda binary runtime.
func NewLightpandaBinaryBrowserBackend(runtime lightpandaBinaryRuntime) *LightpandaBinaryBrowserBackend {
	return &LightpandaBinaryBrowserBackend{runtime: runtime}
}

func (b *LightpandaBinaryBrowserBackend) ensureService(ctx context.Context) (*browser.RodService, error) {
	if b == nil || b.runtime == nil {
		return nil, browser.ErrBrowserNotAvailable
	}
	svc, err := b.runtime.Start(ctx)
	if err != nil {
		return nil, err
	}
	if err := svc.Start(ctx); err != nil {
		_ = b.runtime.Stop(context.Background())
		return nil, err
	}
	return svc, nil
}

func (b *LightpandaBinaryBrowserBackend) peekService() *browser.RodService {
	if b == nil || b.runtime == nil {
		return nil
	}
	return b.runtime.PeekService()
}

func (b *LightpandaBinaryBrowserBackend) Start(ctx context.Context) error {
	_, err := b.ensureService(ctx)
	return err
}

func (b *LightpandaBinaryBrowserBackend) Navigate(ctx context.Context, url string, targetID string) (BrowserNavResult, error) {
	svc, err := b.ensureService(ctx)
	if err != nil {
		return BrowserNavResult{}, err
	}
	resp, err := svc.Navigate(ctx, &browser.NavigateRequest{URL: url, TargetID: targetID})
	if err != nil {
		return BrowserNavResult{}, err
	}
	return BrowserNavResult{URL: resp.URL, Title: resp.Title, TargetID: resp.TargetID}, nil
}

func (b *LightpandaBinaryBrowserBackend) CookieHeader(context.Context, string, string) (string, error) {
	return "", unsupportedLightpandaAction("cookie/session continuity")
}

func (b *LightpandaBinaryBrowserBackend) ObserveNetwork(context.Context, string, int, bool) (BrowserObservedNetworkResult, error) {
	return BrowserObservedNetworkResult{}, unsupportedLightpandaAction("network observation")
}

func (b *LightpandaBinaryBrowserBackend) WaitNetworkIdle(context.Context, string, int, int) error {
	return unsupportedLightpandaAction("network-idle waiting")
}

func (b *LightpandaBinaryBrowserBackend) AccessibilityTree(ctx context.Context, targetID string, maxDepth int) (BrowserA11yTreeResult, error) {
	svc, err := b.ensureService(ctx)
	if err != nil {
		return BrowserA11yTreeResult{}, err
	}
	resp, err := svc.AccessibilityTree(ctx, targetID, maxDepth)
	if err != nil {
		return BrowserA11yTreeResult{}, err
	}
	return BrowserA11yTreeResult{
		Tree:     resp.Tree,
		URL:      resp.URL,
		Title:    resp.Title,
		TargetID: resp.TargetID,
		RefMap:   resp.RefMap,
	}, nil
}

func (b *LightpandaBinaryBrowserBackend) InteractiveElements(ctx context.Context, targetID string) (BrowserInteractiveResult, error) {
	svc, err := b.ensureService(ctx)
	if err != nil {
		return BrowserInteractiveResult{}, err
	}
	resp, err := svc.InteractiveElements(ctx, targetID)
	if err != nil {
		return BrowserInteractiveResult{}, err
	}
	return BrowserInteractiveResult{
		Tree:     resp.Tree,
		URL:      resp.URL,
		Title:    resp.Title,
		TargetID: resp.TargetID,
		RefMap:   resp.RefMap,
		Count:    resp.Count,
	}, nil
}

func (b *LightpandaBinaryBrowserBackend) CountInteractiveElements(ctx context.Context, targetID string) (int, error) {
	svc, err := b.ensureService(ctx)
	if err != nil {
		return 0, err
	}
	return svc.CountInteractiveElements(ctx, targetID)
}

func (b *LightpandaBinaryBrowserBackend) ExtractText(ctx context.Context, targetID, selector string) (string, error) {
	svc, err := b.ensureService(ctx)
	if err != nil {
		return "", err
	}
	return svc.ExtractFirstFromTab(ctx, targetID, selector, "")
}

func (b *LightpandaBinaryBrowserBackend) ActByRef(context.Context, string, int, map[int]int, string, string) error {
	return unsupportedLightpandaAction("interactive actions")
}

func (b *LightpandaBinaryBrowserBackend) ActByInteractiveRef(context.Context, string, int, map[int]string, string, string) error {
	return unsupportedLightpandaAction("interactive actions")
}

func (b *LightpandaBinaryBrowserBackend) Screenshot(context.Context, string) (string, error) {
	return "", unsupportedLightpandaAction("screenshots")
}

func (b *LightpandaBinaryBrowserBackend) ScreenshotTab(context.Context, string) (string, error) {
	return "", unsupportedLightpandaAction("screenshots")
}

func (b *LightpandaBinaryBrowserBackend) CloseTab(ctx context.Context, targetID string) error {
	svc := b.peekService()
	if svc == nil {
		return nil
	}
	return svc.CloseTab(ctx, targetID)
}

func (b *LightpandaBinaryBrowserBackend) Tabs(ctx context.Context) ([]BrowserTabResult, error) {
	svc := b.peekService()
	if svc == nil {
		return nil, nil
	}
	tabs, err := svc.Tabs(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]BrowserTabResult, 0, len(tabs))
	for _, tab := range tabs {
		if tab == nil {
			continue
		}
		out = append(out, BrowserTabResult{
			TargetID: tab.TargetID,
			URL:      tab.URL,
			Title:    tab.Title,
			Active:   tab.Active,
		})
	}
	return out, nil
}

func (b *LightpandaBinaryBrowserBackend) ExecuteRecipe(context.Context, string, map[string]string) (BrowserRecipeResult, error) {
	return BrowserRecipeResult{}, unsupportedLightpandaAction("recipes")
}

func (b *LightpandaBinaryBrowserBackend) ListRecipes(context.Context) []BrowserRecipeInfo {
	return nil
}

func lightpandaBinaryTabToSessionInfo(tab *browser.Tab, now time.Time) browser.SessionInfo {
	info := browser.SessionInfo{
		CreatedAt:    now.Format(time.RFC3339),
		LastActivity: now.Format(time.RFC3339),
		Engine:       browser.SessionEngineLightpanda,
		EngineDetail: browser.SessionEngineDetailLightpandaBinary,
		SessionLayer: browser.SessionLayerBrowserLite,
		MonitorKind:  browser.SessionMonitorKindText,
		Status:       "idle",
	}
	if tab == nil {
		return info
	}
	info.ID = tab.TargetID
	info.CurrentURL = tab.URL
	info.PageTitle = tab.Title
	if tab.Active {
		info.Status = "active"
	}
	return info
}

func (b *LightpandaBinaryBrowserBackend) ListSessionInfos(ctx context.Context) ([]browser.SessionInfo, error) {
	svc := b.peekService()
	if svc == nil {
		return nil, nil
	}
	tabs, err := svc.Tabs(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	out := make([]browser.SessionInfo, 0, len(tabs))
	for _, tab := range tabs {
		out = append(out, lightpandaBinaryTabToSessionInfo(tab, now))
	}
	return out, nil
}

func (b *LightpandaBinaryBrowserBackend) GetSessionInfo(ctx context.Context, targetID string) (*browser.SessionInfo, error) {
	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		return nil, browser.ErrTabNotFound
	}
	sessions, err := b.ListSessionInfos(ctx)
	if err != nil {
		return nil, err
	}
	for i := range sessions {
		if strings.TrimSpace(sessions[i].ID) == targetID {
			session := sessions[i]
			return &session, nil
		}
	}
	return nil, browser.ErrTabNotFound
}

func browserLiteTreePreview(tree string) string {
	tree = strings.TrimSpace(tree)
	if tree == "" {
		return ""
	}
	if len(tree) <= 1200 {
		return tree
	}
	return strings.TrimSpace(tree[:1199]) + "…"
}

func (b *LightpandaBinaryBrowserBackend) CaptureSessionMonitor(ctx context.Context, targetID string) (*browser.SessionMonitorResponse, error) {
	svc := b.peekService()
	if svc == nil {
		return nil, browser.ErrTabNotFound
	}
	info, err := b.GetSessionInfo(ctx, targetID)
	if err != nil {
		return nil, err
	}
	a11y, err := svc.AccessibilityTree(ctx, targetID, 10)
	if err != nil {
		return nil, err
	}
	count, countErr := svc.CountInteractiveElements(ctx, targetID)
	if countErr != nil {
		count = 0
	}
	return &browser.SessionMonitorResponse{
		Kind: browser.SessionMonitorKindText,
		Text: &browser.SessionTextMonitor{
			Title:            info.PageTitle,
			URL:              info.CurrentURL,
			TreePreview:      browserLiteTreePreview(a11y.Tree),
			InteractiveCount: count,
			UpdatedAt:        info.LastActivity,
			Status:           info.Status,
		},
	}, nil
}

func (b *LightpandaBinaryBrowserBackend) CaptureSessionScreenshot(ctx context.Context, targetID string) (*browser.SessionScreenshotResponse, error) {
	if _, err := b.GetSessionInfo(ctx, targetID); err != nil {
		return nil, err
	}
	return &browser.SessionScreenshotResponse{
		Error: unsupportedLightpandaAction("lightpanda browser-lite monitor is text-only").Error(),
	}, nil
}
