package tools

import (
	"context"
	"fmt"
	"strings"
)

type relayURLAwareBrowserBackend interface {
	UsesRelayFor(ctx context.Context, targetID, rawURL string) bool
}

// SitePolicyBrowserBackend routes browser actions between the default browser
// backend and optional managed/relay alternates using a URL policy.
type SitePolicyBrowserBackend struct {
	defaultBackend BrowserBackend
	managedBackend BrowserBackend
	relayBackend   BrowserBackend
	preferRelay    func(rawURL string) bool
	fallbackDriver string
}

// NewSitePolicyBrowserBackend wraps browser backends with per-site relay preference.
func NewSitePolicyBrowserBackend(defaultBackend, managedBackend, relayBackend BrowserBackend, preferRelay func(rawURL string) bool, fallbackDriver string) *SitePolicyBrowserBackend {
	return &SitePolicyBrowserBackend{
		defaultBackend: defaultBackend,
		managedBackend: managedBackend,
		relayBackend:   relayBackend,
		preferRelay:    preferRelay,
		fallbackDriver: strings.ToLower(strings.TrimSpace(fallbackDriver)),
	}
}

func (b *SitePolicyBrowserBackend) Start(context.Context) error {
	// Calls that care about per-site routing start the selected backend lazily.
	return nil
}

func (b *SitePolicyBrowserBackend) UsesRelay(ctx context.Context, targetID string) bool {
	if backend := b.backendForTarget(ctx, targetID); backend != nil {
		return backendUsesRelay(ctx, backend, targetID)
	}
	for _, candidate := range b.candidateBackends() {
		if backendUsesRelay(ctx, candidate, "") {
			return true
		}
	}
	return false
}

func (b *SitePolicyBrowserBackend) UsesRelayFor(ctx context.Context, targetID, rawURL string) bool {
	if backend := b.backendForTarget(ctx, targetID); backend != nil {
		return backendUsesRelay(ctx, backend, targetID)
	}
	primary, fallback := b.backendsForURL(rawURL)
	if primary == nil {
		return false
	}
	if !backendUsesRelay(ctx, primary, "") {
		return false
	}
	if err := primary.Start(ctx); err == nil {
		return true
	}
	return fallback == nil
}

func (b *SitePolicyBrowserBackend) Navigate(ctx context.Context, rawURL string, targetID string) (BrowserNavResult, error) {
	if backend := b.backendForTarget(ctx, targetID); backend != nil {
		if err := backend.Start(ctx); err != nil {
			return BrowserNavResult{}, err
		}
		return backend.Navigate(ctx, rawURL, targetID)
	}
	primary, fallback := b.backendsForURL(rawURL)
	return invokeBrowserWithFallback(ctx, primary, fallback, func(selected BrowserBackend) (BrowserNavResult, error) {
		return selected.Navigate(ctx, rawURL, targetID)
	})
}

func (b *SitePolicyBrowserBackend) CookieHeader(ctx context.Context, targetID string, rawURL string) (string, error) {
	backend := b.backendForTarget(ctx, targetID)
	if backend == nil {
		backend = b.defaultBackend
	}
	if backend == nil {
		return "", fmt.Errorf("browser service not available")
	}
	return backend.CookieHeader(ctx, targetID, rawURL)
}

func (b *SitePolicyBrowserBackend) ObserveNetwork(ctx context.Context, targetID string, maxEntries int, clear bool) (BrowserObservedNetworkResult, error) {
	backend := b.backendForTarget(ctx, targetID)
	if backend == nil {
		backend = b.defaultBackend
	}
	if backend == nil {
		return BrowserObservedNetworkResult{}, fmt.Errorf("browser service not available")
	}
	return backend.ObserveNetwork(ctx, targetID, maxEntries, clear)
}

func (b *SitePolicyBrowserBackend) WaitNetworkIdle(ctx context.Context, targetID string, idleMS int, timeoutMS int) error {
	backend := b.backendForTarget(ctx, targetID)
	if backend == nil {
		backend = b.defaultBackend
	}
	if backend == nil {
		return fmt.Errorf("browser service not available")
	}
	return backend.WaitNetworkIdle(ctx, targetID, idleMS, timeoutMS)
}

func (b *SitePolicyBrowserBackend) AccessibilityTree(ctx context.Context, targetID string, maxDepth int) (BrowserA11yTreeResult, error) {
	backend := b.backendForTarget(ctx, targetID)
	if backend == nil {
		backend = b.defaultBackend
	}
	if backend == nil {
		return BrowserA11yTreeResult{}, fmt.Errorf("browser service not available")
	}
	return backend.AccessibilityTree(ctx, targetID, maxDepth)
}

func (b *SitePolicyBrowserBackend) InteractiveElements(ctx context.Context, targetID string) (BrowserInteractiveResult, error) {
	backend := b.backendForTarget(ctx, targetID)
	if backend == nil {
		backend = b.defaultBackend
	}
	if backend == nil {
		return BrowserInteractiveResult{}, fmt.Errorf("browser service not available")
	}
	return backend.InteractiveElements(ctx, targetID)
}

func (b *SitePolicyBrowserBackend) CountInteractiveElements(ctx context.Context, targetID string) (int, error) {
	backend := b.backendForTarget(ctx, targetID)
	if backend == nil {
		backend = b.defaultBackend
	}
	if backend == nil {
		return 0, fmt.Errorf("browser service not available")
	}
	return backend.CountInteractiveElements(ctx, targetID)
}

func (b *SitePolicyBrowserBackend) ExtractText(ctx context.Context, targetID, selector string) (string, error) {
	backend := b.backendForTarget(ctx, targetID)
	if backend == nil {
		backend = b.defaultBackend
	}
	if backend == nil {
		return "", fmt.Errorf("browser service not available")
	}
	extractor, ok := backend.(readableContentBrowserBackend)
	if !ok {
		return "", fmt.Errorf("browser text extraction not supported")
	}
	return extractor.ExtractText(ctx, targetID, selector)
}

func (b *SitePolicyBrowserBackend) ActByRef(ctx context.Context, targetID string, ref int, refMap map[int]int, action string, value string) error {
	backend := b.backendForTarget(ctx, targetID)
	if backend == nil {
		backend = b.defaultBackend
	}
	if backend == nil {
		return fmt.Errorf("browser service not available")
	}
	return backend.ActByRef(ctx, targetID, ref, refMap, action, value)
}

func (b *SitePolicyBrowserBackend) ActByInteractiveRef(ctx context.Context, targetID string, ref int, refMap map[int]string, action string, value string) error {
	backend := b.backendForTarget(ctx, targetID)
	if backend == nil {
		backend = b.defaultBackend
	}
	if backend == nil {
		return fmt.Errorf("browser service not available")
	}
	return backend.ActByInteractiveRef(ctx, targetID, ref, refMap, action, value)
}

func (b *SitePolicyBrowserBackend) Screenshot(ctx context.Context, rawURL string) (string, error) {
	primary, fallback := b.backendsForURL(rawURL)
	return invokeBrowserWithFallback(ctx, primary, fallback, func(selected BrowserBackend) (string, error) {
		return selected.Screenshot(ctx, rawURL)
	})
}

func (b *SitePolicyBrowserBackend) ScreenshotTab(ctx context.Context, targetID string) (string, error) {
	backend := b.backendForTarget(ctx, targetID)
	if backend == nil {
		backend = b.defaultBackend
	}
	if backend == nil {
		return "", fmt.Errorf("browser service not available")
	}
	return backend.ScreenshotTab(ctx, targetID)
}

func (b *SitePolicyBrowserBackend) CloseTab(ctx context.Context, targetID string) error {
	backend := b.backendForTarget(ctx, targetID)
	if backend == nil {
		backend = b.defaultBackend
	}
	if backend == nil {
		return fmt.Errorf("browser service not available")
	}
	return backend.CloseTab(ctx, targetID)
}

func (b *SitePolicyBrowserBackend) Tabs(ctx context.Context) ([]BrowserTabResult, error) {
	seen := make(map[string]struct{})
	out := make([]BrowserTabResult, 0)
	var firstErr error
	for _, candidate := range b.candidateBackends() {
		tabs, err := candidate.Tabs(ctx)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		for _, tab := range tabs {
			key := strings.TrimSpace(tab.TargetID)
			if key == "" {
				key = strings.TrimSpace(tab.URL)
			}
			if key != "" {
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
			}
			out = append(out, tab)
		}
	}
	if len(out) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return out, nil
}

func (b *SitePolicyBrowserBackend) ExecuteRecipe(ctx context.Context, recipe string, params map[string]string) (BrowserRecipeResult, error) {
	targetID := strings.TrimSpace(params["target_id"])
	if backend := b.backendForTarget(ctx, targetID); backend != nil {
		if err := backend.Start(ctx); err != nil {
			return BrowserRecipeResult{}, err
		}
		return backend.ExecuteRecipe(ctx, recipe, params)
	}
	rawURL := strings.TrimSpace(params["url"])
	primary, fallback := b.backendsForURL(rawURL)
	return invokeBrowserWithFallback(ctx, primary, fallback, func(selected BrowserBackend) (BrowserRecipeResult, error) {
		return selected.ExecuteRecipe(ctx, recipe, params)
	})
}

func (b *SitePolicyBrowserBackend) ListRecipes(ctx context.Context) []BrowserRecipeInfo {
	if b.defaultBackend == nil {
		return nil
	}
	return b.defaultBackend.ListRecipes(ctx)
}

func (b *SitePolicyBrowserBackend) backendForTarget(ctx context.Context, targetID string) BrowserBackend {
	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		return nil
	}
	for _, candidate := range b.candidateBackends() {
		if browserBackendHasTarget(ctx, candidate, targetID) {
			return candidate
		}
	}
	return nil
}

func (b *SitePolicyBrowserBackend) candidateBackends() []BrowserBackend {
	out := make([]BrowserBackend, 0, 3)
	appendUniqueBackend := func(candidate BrowserBackend) {
		if candidate == nil {
			return
		}
		for _, existing := range out {
			if existing == candidate {
				return
			}
		}
		out = append(out, candidate)
	}
	appendUniqueBackend(b.defaultBackend)
	appendUniqueBackend(b.managedBackend)
	appendUniqueBackend(b.relayBackend)
	return out
}

func (b *SitePolicyBrowserBackend) backendsForURL(rawURL string) (BrowserBackend, BrowserBackend) {
	if b.preferRelay == nil || !b.preferRelay(rawURL) {
		return b.defaultBackend, nil
	}
	primary := b.relayBackend
	if primary == nil {
		primary = b.defaultBackend
	}
	var fallback BrowserBackend
	switch b.fallbackDriver {
	case "managed":
		fallback = b.managedBackend
	case "relay":
		fallback = b.relayBackend
	}
	if fallback == primary {
		fallback = nil
	}
	return primary, fallback
}

func browserBackendHasTarget(ctx context.Context, backend BrowserBackend, targetID string) bool {
	if backend == nil || strings.TrimSpace(targetID) == "" {
		return false
	}
	tabs, err := backend.Tabs(ctx)
	if err != nil {
		return false
	}
	for _, tab := range tabs {
		if strings.TrimSpace(tab.TargetID) == strings.TrimSpace(targetID) {
			return true
		}
	}
	return false
}

func backendUsesRelay(ctx context.Context, backend BrowserBackend, targetID string) bool {
	relayBackend, ok := backend.(relayAwareBrowserBackend)
	return ok && relayBackend.UsesRelay(ctx, targetID)
}

func invokeBrowserWithFallback[T any](ctx context.Context, primary, fallback BrowserBackend, invoke func(selected BrowserBackend) (T, error)) (T, error) {
	var zero T
	if primary == nil {
		return zero, fmt.Errorf("browser service not available")
	}
	if err := primary.Start(ctx); err != nil {
		if fallback != nil && shouldFallbackRelayError(err) {
			if retryErr := fallback.Start(ctx); retryErr == nil {
				return invoke(fallback)
			}
		}
		return zero, err
	}
	result, err := invoke(primary)
	if err == nil || fallback == nil || !shouldFallbackRelayError(err) {
		return result, err
	}
	if retryErr := fallback.Start(ctx); retryErr != nil {
		return result, err
	}
	return invoke(fallback)
}

func shouldFallbackRelayError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(lower, "chrome extension not connected"):
		return true
	case strings.Contains(lower, "browser not running"):
		return true
	case strings.Contains(lower, "browser service not available"):
		return true
	case strings.Contains(lower, "cdp_url"):
		return true
	case strings.Contains(lower, "connection refused"):
		return true
	case strings.Contains(lower, "broken pipe"):
		return true
	case strings.Contains(lower, "connection reset"):
		return true
	case strings.Contains(lower, "websocket"):
		return true
	case strings.Contains(lower, "eof"):
		return true
	default:
		return false
	}
}
