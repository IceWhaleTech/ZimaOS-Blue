package tools

import (
	"context"
	"fmt"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxybridge"
)

// RodBrowserAdapter adapts browser.RodService to the UIReviewBrowser interface.
// Supports lazy initialization via a resolve function.
type RodBrowserAdapter struct {
	svc     *browser.RodService
	resolve func() *browser.RodService
}

// NewRodBrowserAdapter creates a new adapter with a direct service reference.
func NewRodBrowserAdapter(svc *browser.RodService) *RodBrowserAdapter {
	return &RodBrowserAdapter{svc: svc}
}

// NewLazyRodBrowserAdapter creates a lazy adapter that resolves the service on first use.
func NewLazyRodBrowserAdapter(resolve func() *browser.RodService) *RodBrowserAdapter {
	return &RodBrowserAdapter{resolve: resolve}
}

func (a *RodBrowserAdapter) get() (*browser.RodService, error) {
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

func (a *RodBrowserAdapter) Start(ctx context.Context) error {
	svc, err := a.get()
	if err != nil {
		return err
	}
	return svc.Start(ctx)
}

func (a *RodBrowserAdapter) NavigateURL(ctx context.Context, url string) (UIReviewNavResult, error) {
	svc, err := a.get()
	if err != nil {
		return UIReviewNavResult{}, err
	}
	resp, err := svc.Navigate(ctx, &browser.NavigateRequest{URL: url})
	if err != nil {
		return UIReviewNavResult{}, err
	}
	return UIReviewNavResult{
		URL:      resp.URL,
		Title:    resp.Title,
		TargetID: resp.TargetID,
	}, nil
}

func (a *RodBrowserAdapter) GetAccessibilityTree(ctx context.Context, targetID string, maxDepth int) (UIReviewA11yResult, error) {
	svc, err := a.get()
	if err != nil {
		return UIReviewA11yResult{}, err
	}
	resp, err := svc.AccessibilityTree(ctx, targetID, maxDepth)
	if err != nil {
		return UIReviewA11yResult{}, err
	}
	return UIReviewA11yResult{Tree: resp.Tree}, nil
}

func (a *RodBrowserAdapter) ScreenshotTab(ctx context.Context, targetID string) (string, error) {
	svc, err := a.get()
	if err != nil {
		return "", err
	}
	return svc.ScreenshotTab(ctx, targetID)
}

func (a *RodBrowserAdapter) CloseTab(ctx context.Context, targetID string) error {
	svc, err := a.get()
	if err != nil {
		return err
	}
	return svc.CloseTab(ctx, targetID)
}

func (a *RodBrowserAdapter) SetViewport(ctx context.Context, targetID string, width, height int) error {
	svc, err := a.get()
	if err != nil {
		return err
	}
	return svc.SetViewport(ctx, targetID, width, height)
}

func (a *RodBrowserAdapter) ScrollTo(ctx context.Context, targetID string, x, y int) error {
	svc, err := a.get()
	if err != nil {
		return err
	}
	return svc.ScrollTo(ctx, targetID, x, y)
}

func (a *RodBrowserAdapter) PageDimensions(ctx context.Context, targetID string) (int, int, error) {
	svc, err := a.get()
	if err != nil {
		return 0, 0, err
	}
	return svc.PageDimensions(ctx, targetID)
}

func (a *RodBrowserAdapter) ScreenshotViewportRaw(ctx context.Context, targetID string) ([]byte, error) {
	svc, err := a.get()
	if err != nil {
		return nil, err
	}
	return svc.ScreenshotViewportRaw(ctx, targetID)
}

// ProxyBridgeVLMAdapter adapts proxybridge.Bridge to the VLMBridge interface.
type ProxyBridgeVLMAdapter struct {
	bridge *proxybridge.Bridge
}

// NewProxyBridgeVLMAdapter creates a new adapter.
func NewProxyBridgeVLMAdapter(bridge *proxybridge.Bridge) *ProxyBridgeVLMAdapter {
	return &ProxyBridgeVLMAdapter{bridge: bridge}
}

func (a *ProxyBridgeVLMAdapter) ChatWithVision(ctx context.Context, prompt string, imageBase64 string) (string, error) {
	req := llm.ChatRequest{
		Model: "auto",
		Messages: []llm.Message{{
			Role: llm.RoleUser,
			ContentParts: []llm.ContentPart{
				{Type: "text", Text: prompt},
				{Type: "image", MediaType: "image/png", Data: imageBase64},
			},
		}},
		MaxTokens: 2000,
	}

	resp, err := a.bridge.Chat(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.Message.Content, nil
}
