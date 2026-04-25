package tools

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxybridge"
)

// RodBrowserAdapter adapts browser.RodService to the UIReviewBrowser interface.
// Supports lazy initialization via a resolve function.
type RodBrowserAdapter struct {
	source rodServiceSource
}

// NewRodBrowserAdapter creates a new adapter with a direct service reference.
func NewRodBrowserAdapter(svc *browser.RodService) *RodBrowserAdapter {
	return &RodBrowserAdapter{source: rodServiceSource{svc: svc}}
}

// NewLazyRodBrowserAdapter creates a lazy adapter that resolves the service on demand.
func NewLazyRodBrowserAdapter(resolve func() *browser.RodService) *RodBrowserAdapter {
	return &RodBrowserAdapter{source: rodServiceSource{resolve: resolve}}
}

// NewLeaseAwareRodBrowserAdapter creates a lazy adapter that keeps a managed
// browser instance alive for the duration of each UI review call.
func NewLeaseAwareRodBrowserAdapter(acquire rodServiceAcquireFunc) *RodBrowserAdapter {
	return &RodBrowserAdapter{source: rodServiceSource{acquire: acquire}}
}

func (a *RodBrowserAdapter) get() (*browser.RodService, error) {
	if a == nil {
		return nil, fmt.Errorf("browser service not available")
	}
	switch {
	case a.source.svc != nil:
		return a.source.svc, nil
	case a.source.resolve != nil:
		if svc := a.source.resolve(); svc != nil {
			return svc, nil
		}
	}
	return nil, fmt.Errorf("browser service not available")
}

func (a *RodBrowserAdapter) acquire() (rodServiceLease, error) {
	if a == nil {
		return rodServiceLease{}, fmt.Errorf("browser service not available")
	}
	return acquireRodServiceSource(a.source, true)
}

func (a *RodBrowserAdapter) Start(ctx context.Context) error {
	lease, err := a.acquire()
	if err != nil {
		return err
	}
	defer lease.close()
	return lease.svc.Start(ctx)
}

func (a *RodBrowserAdapter) NavigateURL(ctx context.Context, url string) (UIReviewNavResult, error) {
	lease, err := a.acquire()
	if err != nil {
		return UIReviewNavResult{}, err
	}
	defer lease.close()
	resp, err := lease.svc.Navigate(ctx, &browser.NavigateRequest{URL: url})
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
	lease, err := a.acquire()
	if err != nil {
		return UIReviewA11yResult{}, err
	}
	defer lease.close()
	resp, err := lease.svc.AccessibilityTree(ctx, targetID, maxDepth)
	if err != nil {
		return UIReviewA11yResult{}, err
	}
	return UIReviewA11yResult{Tree: resp.Tree}, nil
}

func (a *RodBrowserAdapter) ScreenshotTab(ctx context.Context, targetID string) (string, error) {
	lease, err := a.acquire()
	if err != nil {
		return "", err
	}
	defer lease.close()
	return lease.svc.ScreenshotTab(ctx, targetID)
}

func (a *RodBrowserAdapter) CloseTab(ctx context.Context, targetID string) error {
	lease, err := a.acquire()
	if err != nil {
		return err
	}
	defer lease.close()
	return lease.svc.CloseTab(ctx, targetID)
}

func (a *RodBrowserAdapter) SetViewport(ctx context.Context, targetID string, width, height int) error {
	lease, err := a.acquire()
	if err != nil {
		return err
	}
	defer lease.close()
	return lease.svc.SetViewport(ctx, targetID, width, height)
}

func (a *RodBrowserAdapter) ScrollTo(ctx context.Context, targetID string, x, y int) error {
	lease, err := a.acquire()
	if err != nil {
		return err
	}
	defer lease.close()
	return lease.svc.ScrollTo(ctx, targetID, x, y)
}

func (a *RodBrowserAdapter) PageDimensions(ctx context.Context, targetID string) (int, int, error) {
	lease, err := a.acquire()
	if err != nil {
		return 0, 0, err
	}
	defer lease.close()
	return lease.svc.PageDimensions(ctx, targetID)
}

func (a *RodBrowserAdapter) ScreenshotViewportRaw(ctx context.Context, targetID string) ([]byte, error) {
	lease, err := a.acquire()
	if err != nil {
		return nil, err
	}
	defer lease.close()
	return lease.svc.ScreenshotViewportRaw(ctx, targetID)
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

// LLMBridge defines the interface for making LLM calls from tools.
type LLMBridge interface {
	Chat(ctx context.Context, prompt string, maxTokens int) (string, error)
}

// ProxyBridgeLLMAdapter adapts proxybridge.Bridge to the LLMBridge interface.
type ProxyBridgeLLMAdapter struct {
	bridge *proxybridge.Bridge
}

// NewProxyBridgeLLMAdapter creates a new LLM adapter.
func NewProxyBridgeLLMAdapter(bridge *proxybridge.Bridge) *ProxyBridgeLLMAdapter {
	return &ProxyBridgeLLMAdapter{bridge: bridge}
}

func (a *ProxyBridgeLLMAdapter) Chat(ctx context.Context, prompt string, maxTokens int) (string, error) {
	if maxTokens <= 0 {
		maxTokens = 4000
	}
	message := llm.Message{Role: llm.RoleUser, Content: prompt}
	if parts := proxyBridgePromptContentParts(prompt); len(parts) > 0 {
		message.Content = ""
		message.ContentParts = parts
	}
	req := llm.ChatRequest{
		Model:     "auto",
		Messages:  []llm.Message{message},
		MaxTokens: maxTokens,
	}

	resp, err := a.bridge.Chat(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.Message.Content, nil
}

var proxyBridgePromptImagePathPattern = regexp.MustCompile(`/[^\s"'<>]+\.(?:png|jpg|jpeg|webp)`)

func proxyBridgePromptContentParts(prompt string) []llm.ContentPart {
	paths := proxyBridgePromptImagePaths(prompt)
	if len(paths) == 0 {
		return nil
	}
	parts := []llm.ContentPart{{Type: "text", Text: prompt}}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil || len(data) == 0 {
			continue
		}
		parts = append(parts, llm.ContentPart{
			Type:      "image",
			MediaType: proxyBridgeImageMediaType(path),
			Data:      base64.StdEncoding.EncodeToString(data),
		})
	}
	if len(parts) == 1 {
		return nil
	}
	return parts
}

func proxyBridgePromptImagePaths(prompt string) []string {
	matches := proxyBridgePromptImagePathPattern.FindAllString(prompt, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		path := strings.TrimSpace(strings.Trim(match, `.,);]}`))
		if path == "" {
			continue
		}
		if _, exists := seen[path]; exists {
			continue
		}
		if stat, err := os.Stat(path); err != nil || stat.IsDir() {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}
	return out
}

func proxyBridgeImageMediaType(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	default:
		return "image/png"
	}
}
