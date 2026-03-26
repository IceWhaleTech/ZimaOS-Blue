package tools

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
)

type sessionBrowserBackend interface {
	BrowserBackend
	ListSessionInfos(ctx context.Context) ([]browser.SessionInfo, error)
	GetSessionInfo(ctx context.Context, targetID string) (*browser.SessionInfo, error)
	CaptureSessionMonitor(ctx context.Context, targetID string) (*browser.SessionMonitorResponse, error)
	CaptureSessionScreenshot(ctx context.Context, targetID string) (*browser.SessionScreenshotResponse, error)
}

// HybridCapabilityBrowserBackend routes browser work by capability between
// Lightpanda read-only sessions and Chromium runtimes.
type HybridCapabilityBrowserBackend struct {
	config           *browser.Config
	lightpanda       *browser.LightpandaService
	lightpandaBinary sessionBrowserBackend
	managed          sessionBrowserBackend
	relay            sessionBrowserBackend
	preferRelay      func(rawURL string) bool
	relayReady       func(ctx context.Context) bool

	relayAvailabilityMu        sync.RWMutex
	relayAvailabilityValue     bool
	relayAvailabilityCheckedAt time.Time
}

const hybridRelayAvailabilityTTL = 2 * time.Second

// NewHybridCapabilityBrowserBackend creates a hybrid browser router.
func NewHybridCapabilityBrowserBackend(
	config *browser.Config,
	lightpanda *browser.LightpandaService,
	lightpandaBinary sessionBrowserBackend,
	managed sessionBrowserBackend,
	relay sessionBrowserBackend,
	preferRelay func(rawURL string) bool,
	relayReady func(ctx context.Context) bool,
) *HybridCapabilityBrowserBackend {
	return &HybridCapabilityBrowserBackend{
		config:           config.Clone(),
		lightpanda:       lightpanda,
		lightpandaBinary: lightpandaBinary,
		managed:          managed,
		relay:            relay,
		preferRelay:      preferRelay,
		relayReady:       relayReady,
	}
}

func (b *HybridCapabilityBrowserBackend) Start(context.Context) error {
	// Backends are started lazily based on route choice.
	return nil
}

func (b *HybridCapabilityBrowserBackend) lightpandaEnabled() bool {
	return b != nil &&
		b.config != nil &&
		b.config.ResolvedStrategy() == browser.BrowserStrategyHybridCapability &&
		b.config.Lightpanda.Enabled &&
		b.lightpanda != nil
}

func (b *HybridCapabilityBrowserBackend) chromiumCandidates(rawURL string) (sessionBrowserBackend, sessionBrowserBackend) {
	if b == nil {
		return nil, nil
	}
	preferRelay := false
	if b.preferRelay != nil && strings.TrimSpace(rawURL) != "" && b.preferRelay(rawURL) {
		preferRelay = true
	} else if b.config != nil && b.config.PreferLocalChrome() {
		preferRelay = true
	}
	primary := b.managed
	fallback := b.relay
	if preferRelay {
		primary = b.relay
		fallback = b.managed
	}
	if primary == nil {
		primary = fallback
		fallback = nil
	}
	if fallback == primary {
		fallback = nil
	}
	return primary, fallback
}

func (b *HybridCapabilityBrowserBackend) chromiumCandidateList(rawURL string) []sessionBrowserBackend {
	primary, fallback := b.chromiumCandidates(rawURL)
	out := make([]sessionBrowserBackend, 0, 2)
	appendUnique := func(candidate sessionBrowserBackend) {
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
	appendUnique(primary)
	appendUnique(fallback)
	return out
}

func (b *HybridCapabilityBrowserBackend) chromiumDetailForBackend(candidate sessionBrowserBackend) browser.SessionEngineDetail {
	switch candidate {
	case nil:
		return ""
	case b.relay:
		return browser.SessionEngineDetailChromiumRelay
	default:
		return browser.SessionEngineDetailChromiumManaged
	}
}

func (b *HybridCapabilityBrowserBackend) warmChromiumSession(ctx context.Context, rawURL string) (sessionBrowserBackend, string, browser.SessionEngineDetail, bool) {
	candidates := b.chromiumCandidateList(rawURL)
	if len(candidates) == 0 {
		return nil, "", "", false
	}

	selectSession := func(requireActive bool) (sessionBrowserBackend, string, browser.SessionEngineDetail, bool) {
		for _, candidate := range candidates {
			if candidate == nil {
				continue
			}
			sessions, err := candidate.ListSessionInfos(ctx)
			if err != nil {
				continue
			}
			detail := b.chromiumDetailForBackend(candidate)
			for _, info := range sessions {
				targetID := strings.TrimSpace(info.ID)
				if targetID == "" {
					continue
				}
				active := strings.EqualFold(strings.TrimSpace(info.Status), "active")
				if active != requireActive {
					continue
				}
				if info.EngineDetail != "" {
					detail = info.EngineDetail
				}
				return candidate, targetID, detail, true
			}
		}
		return nil, "", "", false
	}

	if candidate, targetID, detail, ok := selectSession(true); ok {
		return candidate, targetID, detail, true
	}
	return selectSession(false)
}

func (b *HybridCapabilityBrowserBackend) relayAvailable(ctx context.Context) bool {
	if b == nil || b.relay == nil {
		return false
	}
	if b.relayReady == nil {
		return true
	}
	now := time.Now()
	b.relayAvailabilityMu.RLock()
	if !b.relayAvailabilityCheckedAt.IsZero() && now.Sub(b.relayAvailabilityCheckedAt) < hybridRelayAvailabilityTTL {
		available := b.relayAvailabilityValue
		b.relayAvailabilityMu.RUnlock()
		return available
	}
	b.relayAvailabilityMu.RUnlock()

	if ctx == nil {
		ctx = context.Background()
	}
	available := b.relayReady(ctx)
	b.relayAvailabilityMu.Lock()
	b.relayAvailabilityValue = available
	b.relayAvailabilityCheckedAt = now
	b.relayAvailabilityMu.Unlock()
	return available
}

func (b *HybridCapabilityBrowserBackend) chromiumCandidatesForNewSession(ctx context.Context, rawURL string) (sessionBrowserBackend, sessionBrowserBackend) {
	primary, fallback := b.chromiumCandidates(rawURL)
	if b.relay == nil || b.relayAvailable(ctx) {
		return primary, fallback
	}
	if primary == b.relay {
		primary = fallback
		fallback = nil
	} else if fallback == b.relay {
		fallback = nil
	}
	if primary == fallback {
		fallback = nil
	}
	return primary, fallback
}

func (b *HybridCapabilityBrowserBackend) lightpandaPreferredNavigate(ctx context.Context, rawURL, targetID string) bool {
	if !b.lightpandaEnabled() || strings.TrimSpace(targetID) != "" {
		return false
	}
	if _, _, _, ok := b.warmChromiumSession(ctx, rawURL); ok {
		return false
	}
	if host := webFetchHostForURL(rawURL); host != "" && !webFetchSupportsLightpandaHost(host) {
		return false
	}
	hint := GetBrowserRouteHint(ctx)
	if strings.TrimSpace(hint.Action) != "navigate" {
		return false
	}
	switch strings.TrimSpace(hint.FollowupAction) {
	case "snapshot", "snapshot_auto", "snapshot_interactive":
	default:
		return false
	}
	if hint.Vision || hint.RequiresImage || hint.RequiresInteract || hint.RequiresRecipe {
		return false
	}
	if b.preferRelay != nil && strings.TrimSpace(rawURL) != "" && b.preferRelay(rawURL) {
		return false
	}
	return true
}

func (b *HybridCapabilityBrowserBackend) targetDetail(ctx context.Context, targetID string) browser.SessionEngineDetail {
	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		return ""
	}
	if b.lightpanda != nil && b.lightpanda.HasSession(targetID) {
		return browser.SessionEngineDetailLightpandaShim
	}
	if b.lightpandaBinary != nil {
		if _, err := b.lightpandaBinary.GetSessionInfo(ctx, targetID); err == nil {
			return browser.SessionEngineDetailLightpandaBinary
		}
	}
	for _, candidate := range b.chromiumCandidateList("") {
		if candidate == nil {
			continue
		}
		if _, err := candidate.GetSessionInfo(ctx, targetID); err == nil {
			svcEngine := browser.SessionEngineDetailChromiumManaged
			if candidate == b.relay {
				svcEngine = browser.SessionEngineDetailChromiumRelay
			}
			return svcEngine
		}
	}
	return ""
}

func (b *HybridCapabilityBrowserBackend) targetEngine(ctx context.Context, targetID string) browser.SessionEngine {
	switch b.targetDetail(ctx, targetID) {
	case browser.SessionEngineDetailLightpandaShim, browser.SessionEngineDetailLightpandaBinary:
		return browser.SessionEngineLightpanda
	case browser.SessionEngineDetailChromiumManaged:
		return browser.SessionEngineChromiumManaged
	case browser.SessionEngineDetailChromiumRelay:
		return browser.SessionEngineChromiumRelay
	default:
		return ""
	}
}

func (b *HybridCapabilityBrowserBackend) isLightpandaEscalationError(err error) bool {
	return errors.Is(err, browser.ErrLightpandaUnsupportedCapability) ||
		errors.Is(err, browser.ErrLightpandaDOMUnstable) ||
		errors.Is(err, browser.ErrLightpandaEmptyTree) ||
		errors.Is(err, browser.ErrLightpandaNavigationBlocked) ||
		errors.Is(err, browser.ErrLightpandaJSRequired)
}

func (b *HybridCapabilityBrowserBackend) navigateChromium(ctx context.Context, rawURL string, targetID string) (BrowserNavResult, error) {
	primary, fallback := b.chromiumCandidates(rawURL)
	if strings.TrimSpace(targetID) == "" {
		primary, fallback = b.chromiumCandidatesForNewSession(ctx, rawURL)
	}
	return invokeBrowserWithFallback(ctx, primary, fallback, func(selected BrowserBackend) (BrowserNavResult, error) {
		return selected.Navigate(ctx, rawURL, targetID)
	})
}

func (b *HybridCapabilityBrowserBackend) chromiumForTarget(ctx context.Context, targetID string) sessionBrowserBackend {
	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		primary, _ := b.chromiumCandidates("")
		return primary
	}
	for _, candidate := range b.chromiumCandidateList("") {
		if candidate == nil {
			continue
		}
		if _, err := candidate.GetSessionInfo(ctx, targetID); err == nil {
			return candidate
		}
	}
	return nil
}

func (b *HybridCapabilityBrowserBackend) sessionInfoForTarget(ctx context.Context, targetID string) (*browser.SessionInfo, error) {
	switch b.targetDetail(ctx, targetID) {
	case browser.SessionEngineDetailLightpandaShim:
		return b.lightpanda.SessionInfo(targetID)
	case browser.SessionEngineDetailLightpandaBinary:
		if b.lightpandaBinary == nil {
			return nil, browser.ErrTabNotFound
		}
		return b.lightpandaBinary.GetSessionInfo(ctx, targetID)
	case browser.SessionEngineDetailChromiumManaged, browser.SessionEngineDetailChromiumRelay:
		chromiumBackend := b.chromiumForTarget(ctx, targetID)
		if chromiumBackend == nil {
			return nil, browser.ErrTabNotFound
		}
		return chromiumBackend.GetSessionInfo(ctx, targetID)
	default:
		return nil, browser.ErrTabNotFound
	}
}

func unsupportedLightpandaAction(action string) error {
	action = strings.TrimSpace(action)
	if action == "" {
		action = "requested operation"
	}
	return fmt.Errorf("%w: %s requires Chromium", browser.ErrLightpandaUnsupportedCapability, action)
}

// UsesRelay reports whether the selected engine is relay/local Chrome.
func (b *HybridCapabilityBrowserBackend) UsesRelay(ctx context.Context, targetID string) bool {
	switch b.targetDetail(ctx, targetID) {
	case browser.SessionEngineDetailChromiumRelay:
		return true
	case browser.SessionEngineDetailLightpandaShim, browser.SessionEngineDetailLightpandaBinary:
		return false
	default:
		if candidate, _, _, ok := b.warmChromiumSession(ctx, ""); ok {
			return candidate == b.relay
		}
		primary, _ := b.chromiumCandidatesForNewSession(ctx, "")
		return primary != nil && primary == b.relay
	}
}

// UsesRelayFor reports whether the selected route for a URL will use relay/local Chrome.
func (b *HybridCapabilityBrowserBackend) UsesRelayFor(ctx context.Context, targetID, rawURL string) bool {
	switch b.targetDetail(ctx, targetID) {
	case browser.SessionEngineDetailChromiumRelay:
		return true
	case browser.SessionEngineDetailLightpandaShim, browser.SessionEngineDetailLightpandaBinary:
		return false
	}
	if candidate, _, _, ok := b.warmChromiumSession(ctx, rawURL); ok {
		return candidate == b.relay
	}
	if b.lightpandaPreferredNavigate(ctx, rawURL, targetID) {
		return false
	}
	primary, _ := b.chromiumCandidatesForNewSession(ctx, rawURL)
	return primary != nil && primary == b.relay
}

func (b *HybridCapabilityBrowserBackend) Navigate(ctx context.Context, rawURL string, targetID string) (BrowserNavResult, error) {
	switch b.targetDetail(ctx, targetID) {
	case browser.SessionEngineDetailLightpandaShim:
		resp, err := b.lightpanda.Navigate(ctx, &browser.NavigateRequest{URL: rawURL, TargetID: targetID})
		if err != nil {
			return BrowserNavResult{}, err
		}
		return BrowserNavResult{URL: resp.URL, Title: resp.Title, TargetID: resp.TargetID}, nil
	case browser.SessionEngineDetailLightpandaBinary:
		if b.lightpandaBinary == nil {
			return BrowserNavResult{}, browser.ErrTabNotFound
		}
		return b.lightpandaBinary.Navigate(ctx, rawURL, targetID)
	case browser.SessionEngineDetailChromiumManaged, browser.SessionEngineDetailChromiumRelay:
		return b.navigateChromium(ctx, rawURL, targetID)
	}

	if strings.TrimSpace(targetID) == "" {
		if candidate, warmTargetID, _, ok := b.warmChromiumSession(ctx, rawURL); ok && candidate != nil {
			if nav, err := candidate.Navigate(ctx, rawURL, warmTargetID); err == nil {
				return nav, nil
			}
		}
	}

	if b.lightpandaPreferredNavigate(ctx, rawURL, targetID) {
		resp, err := b.lightpanda.Navigate(ctx, &browser.NavigateRequest{URL: rawURL})
		if err == nil {
			return BrowserNavResult{URL: resp.URL, Title: resp.Title, TargetID: resp.TargetID}, nil
		}
		if !b.config.CapabilityEscalateOnFailure() || !b.isLightpandaEscalationError(err) {
			return BrowserNavResult{}, err
		}
		if b.lightpandaBinary != nil {
			if nav, binaryErr := b.lightpandaBinary.Navigate(ctx, rawURL, targetID); binaryErr == nil {
				return nav, nil
			}
		}
	}
	return b.navigateChromium(ctx, rawURL, targetID)
}

func (b *HybridCapabilityBrowserBackend) CookieHeader(ctx context.Context, targetID string, rawURL string) (string, error) {
	switch b.targetDetail(ctx, targetID) {
	case browser.SessionEngineDetailLightpandaShim, browser.SessionEngineDetailLightpandaBinary:
		return "", unsupportedLightpandaAction("cookie/session continuity")
	default:
		primary, fallback := b.chromiumCandidates(rawURL)
		if strings.TrimSpace(targetID) == "" {
			primary, fallback = b.chromiumCandidatesForNewSession(ctx, rawURL)
		}
		return invokeBrowserWithFallback(ctx, primary, fallback, func(selected BrowserBackend) (string, error) {
			return selected.CookieHeader(ctx, targetID, rawURL)
		})
	}
}

func (b *HybridCapabilityBrowserBackend) ObserveNetwork(ctx context.Context, targetID string, maxEntries int, clear bool) (BrowserObservedNetworkResult, error) {
	switch b.targetDetail(ctx, targetID) {
	case browser.SessionEngineDetailLightpandaShim, browser.SessionEngineDetailLightpandaBinary:
		return BrowserObservedNetworkResult{}, unsupportedLightpandaAction("network observation")
	}
	backend := b.chromiumForTarget(ctx, targetID)
	if backend == nil {
		backend, _ = b.chromiumCandidatesForNewSession(ctx, "")
	}
	if backend == nil {
		return BrowserObservedNetworkResult{}, fmt.Errorf("browser service not available")
	}
	return backend.ObserveNetwork(ctx, targetID, maxEntries, clear)
}

func (b *HybridCapabilityBrowserBackend) WaitNetworkIdle(ctx context.Context, targetID string, idleMS int, timeoutMS int) error {
	switch b.targetDetail(ctx, targetID) {
	case browser.SessionEngineDetailLightpandaShim, browser.SessionEngineDetailLightpandaBinary:
		return unsupportedLightpandaAction("network-idle waiting")
	}
	backend := b.chromiumForTarget(ctx, targetID)
	if backend == nil {
		backend, _ = b.chromiumCandidatesForNewSession(ctx, "")
	}
	if backend == nil {
		return fmt.Errorf("browser service not available")
	}
	return backend.WaitNetworkIdle(ctx, targetID, idleMS, timeoutMS)
}

func (b *HybridCapabilityBrowserBackend) AccessibilityTree(ctx context.Context, targetID string, maxDepth int) (BrowserA11yTreeResult, error) {
	switch b.targetDetail(ctx, targetID) {
	case browser.SessionEngineDetailLightpandaShim:
		resp, err := b.lightpanda.AccessibilityTree(ctx, targetID, maxDepth)
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
	case browser.SessionEngineDetailLightpandaBinary:
		if b.lightpandaBinary == nil {
			return BrowserA11yTreeResult{}, browser.ErrTabNotFound
		}
		return b.lightpandaBinary.AccessibilityTree(ctx, targetID, maxDepth)
	default:
		backend := b.chromiumForTarget(ctx, targetID)
		if backend == nil {
			return BrowserA11yTreeResult{}, browser.ErrTabNotFound
		}
		return backend.AccessibilityTree(ctx, targetID, maxDepth)
	}
}

func (b *HybridCapabilityBrowserBackend) InteractiveElements(ctx context.Context, targetID string) (BrowserInteractiveResult, error) {
	switch b.targetDetail(ctx, targetID) {
	case browser.SessionEngineDetailLightpandaShim:
		resp, err := b.lightpanda.InteractiveElements(ctx, targetID)
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
	case browser.SessionEngineDetailLightpandaBinary:
		if b.lightpandaBinary == nil {
			return BrowserInteractiveResult{}, browser.ErrTabNotFound
		}
		return b.lightpandaBinary.InteractiveElements(ctx, targetID)
	default:
		backend := b.chromiumForTarget(ctx, targetID)
		if backend == nil {
			return BrowserInteractiveResult{}, browser.ErrTabNotFound
		}
		return backend.InteractiveElements(ctx, targetID)
	}
}

func (b *HybridCapabilityBrowserBackend) CountInteractiveElements(ctx context.Context, targetID string) (int, error) {
	switch b.targetDetail(ctx, targetID) {
	case browser.SessionEngineDetailLightpandaShim:
		return b.lightpanda.CountInteractiveElements(ctx, targetID)
	case browser.SessionEngineDetailLightpandaBinary:
		if b.lightpandaBinary == nil {
			return 0, browser.ErrTabNotFound
		}
		return b.lightpandaBinary.CountInteractiveElements(ctx, targetID)
	default:
		backend := b.chromiumForTarget(ctx, targetID)
		if backend == nil {
			return 0, browser.ErrTabNotFound
		}
		return backend.CountInteractiveElements(ctx, targetID)
	}
}

func (b *HybridCapabilityBrowserBackend) ActByRef(ctx context.Context, targetID string, ref int, refMap map[int]int, action string, value string) error {
	switch b.targetDetail(ctx, targetID) {
	case browser.SessionEngineDetailLightpandaShim, browser.SessionEngineDetailLightpandaBinary:
		return unsupportedLightpandaAction("interactive actions")
	}
	backend := b.chromiumForTarget(ctx, targetID)
	if backend == nil {
		return browser.ErrTabNotFound
	}
	return backend.ActByRef(ctx, targetID, ref, refMap, action, value)
}

func (b *HybridCapabilityBrowserBackend) ActByInteractiveRef(ctx context.Context, targetID string, ref int, refMap map[int]string, action string, value string) error {
	switch b.targetDetail(ctx, targetID) {
	case browser.SessionEngineDetailLightpandaShim, browser.SessionEngineDetailLightpandaBinary:
		return unsupportedLightpandaAction("interactive actions")
	}
	backend := b.chromiumForTarget(ctx, targetID)
	if backend == nil {
		return browser.ErrTabNotFound
	}
	return backend.ActByInteractiveRef(ctx, targetID, ref, refMap, action, value)
}

func (b *HybridCapabilityBrowserBackend) Screenshot(ctx context.Context, rawURL string) (string, error) {
	primary, fallback := b.chromiumCandidatesForNewSession(ctx, rawURL)
	return invokeBrowserWithFallback(ctx, primary, fallback, func(selected BrowserBackend) (string, error) {
		return selected.Screenshot(ctx, rawURL)
	})
}

func (b *HybridCapabilityBrowserBackend) ScreenshotTab(ctx context.Context, targetID string) (string, error) {
	switch b.targetDetail(ctx, targetID) {
	case browser.SessionEngineDetailLightpandaShim, browser.SessionEngineDetailLightpandaBinary:
		return "", unsupportedLightpandaAction("screenshots")
	}
	backend := b.chromiumForTarget(ctx, targetID)
	if backend == nil {
		return "", browser.ErrTabNotFound
	}
	return backend.ScreenshotTab(ctx, targetID)
}

func (b *HybridCapabilityBrowserBackend) CloseTab(ctx context.Context, targetID string) error {
	switch b.targetDetail(ctx, targetID) {
	case browser.SessionEngineDetailLightpandaShim:
		return b.lightpanda.CloseTab(ctx, targetID)
	case browser.SessionEngineDetailLightpandaBinary:
		if b.lightpandaBinary == nil {
			return nil
		}
		return b.lightpandaBinary.CloseTab(ctx, targetID)
	default:
		backend := b.chromiumForTarget(ctx, targetID)
		if backend == nil {
			return nil
		}
		return backend.CloseTab(ctx, targetID)
	}
}

func (b *HybridCapabilityBrowserBackend) Tabs(ctx context.Context) ([]BrowserTabResult, error) {
	seen := make(map[string]struct{})
	out := make([]BrowserTabResult, 0, 8)
	if b.lightpanda != nil {
		if tabs, err := b.lightpanda.Tabs(ctx); err == nil {
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
				out = append(out, BrowserTabResult{
					TargetID: tab.TargetID,
					URL:      tab.URL,
					Title:    tab.Title,
					Active:   tab.Active,
				})
			}
		}
	}
	if b.lightpandaBinary != nil {
		if tabs, err := b.lightpandaBinary.Tabs(ctx); err == nil {
			for _, tab := range tabs {
				key := strings.TrimSpace(tab.TargetID)
				if key != "" {
					if _, ok := seen[key]; ok {
						continue
					}
					seen[key] = struct{}{}
				}
				out = append(out, tab)
			}
		}
	}
	for _, candidate := range b.chromiumCandidateList("") {
		if candidate == nil {
			continue
		}
		tabs, err := candidate.Tabs(ctx)
		if err != nil {
			continue
		}
		for _, tab := range tabs {
			key := strings.TrimSpace(tab.TargetID)
			if key != "" {
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
			}
			out = append(out, tab)
		}
	}
	return out, nil
}

func (b *HybridCapabilityBrowserBackend) ExecuteRecipe(ctx context.Context, recipe string, params map[string]string) (BrowserRecipeResult, error) {
	targetID := strings.TrimSpace(params["target_id"])
	switch b.targetDetail(ctx, targetID) {
	case browser.SessionEngineDetailLightpandaShim, browser.SessionEngineDetailLightpandaBinary:
		return BrowserRecipeResult{}, unsupportedLightpandaAction("recipes")
	}
	rawURL := strings.TrimSpace(params["url"])
	primary, fallback := b.chromiumCandidates(rawURL)
	if targetID == "" {
		primary, fallback = b.chromiumCandidatesForNewSession(ctx, rawURL)
	}
	return invokeBrowserWithFallback(ctx, primary, fallback, func(selected BrowserBackend) (BrowserRecipeResult, error) {
		return selected.ExecuteRecipe(ctx, recipe, params)
	})
}

func (b *HybridCapabilityBrowserBackend) ListRecipes(ctx context.Context) []BrowserRecipeInfo {
	primary, fallback := b.chromiumCandidatesForNewSession(ctx, "")
	if primary != nil {
		if infos := primary.ListRecipes(ctx); len(infos) > 0 {
			return infos
		}
	}
	if fallback != nil {
		return fallback.ListRecipes(ctx)
	}
	return nil
}

// ListBrowserSessions aggregates sessions from Lightpanda and both Chromium runtimes.
func (b *HybridCapabilityBrowserBackend) ListBrowserSessions(ctx context.Context) ([]browser.SessionInfo, error) {
	seen := make(map[string]struct{})
	out := make([]browser.SessionInfo, 0, 8)
	appendUnique := func(info browser.SessionInfo) {
		key := strings.TrimSpace(info.ID)
		if key == "" {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, info)
	}
	if b.lightpanda != nil {
		for _, info := range b.lightpanda.ListSessionInfos() {
			appendUnique(info)
		}
	}
	if b.lightpandaBinary != nil {
		sessions, err := b.lightpandaBinary.ListSessionInfos(ctx)
		if err == nil {
			for _, info := range sessions {
				appendUnique(info)
			}
		}
	}
	for _, candidate := range b.chromiumCandidateList("") {
		if candidate == nil {
			continue
		}
		sessions, err := candidate.ListSessionInfos(ctx)
		if err != nil {
			continue
		}
		for _, info := range sessions {
			appendUnique(info)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Status == out[j].Status {
			return out[i].ID < out[j].ID
		}
		return out[i].Status == "active"
	})
	return out, nil
}

// CreateBrowserSession creates a Chromium-backed session for UI continuity.
func (b *HybridCapabilityBrowserBackend) CreateBrowserSession(ctx context.Context) (*browser.SessionInfo, error) {
	nav, err := b.navigateChromium(ctx, "about:blank", "")
	if err != nil {
		return nil, err
	}
	return b.GetBrowserSession(ctx, nav.TargetID)
}

// GetBrowserSession returns a single session summary.
func (b *HybridCapabilityBrowserBackend) GetBrowserSession(ctx context.Context, id string) (*browser.SessionInfo, error) {
	return b.sessionInfoForTarget(ctx, id)
}

// CloseBrowserSession closes an existing session.
func (b *HybridCapabilityBrowserBackend) CloseBrowserSession(ctx context.Context, id string) error {
	return b.CloseTab(ctx, id)
}

// NavigateBrowserSession navigates within an existing session.
func (b *HybridCapabilityBrowserBackend) NavigateBrowserSession(ctx context.Context, id string, rawURL string) (*browser.NavigateResponse, error) {
	nav, err := b.Navigate(ctx, rawURL, id)
	if err != nil {
		return nil, err
	}
	return &browser.NavigateResponse{
		URL:      nav.URL,
		Title:    nav.Title,
		TargetID: nav.TargetID,
	}, nil
}

// CaptureBrowserSessionMonitor captures either text or image monitor payloads.
func (b *HybridCapabilityBrowserBackend) CaptureBrowserSessionMonitor(ctx context.Context, id string) (*browser.SessionMonitorResponse, error) {
	switch b.targetDetail(ctx, id) {
	case browser.SessionEngineDetailLightpandaShim:
		return b.lightpanda.CaptureMonitor(id)
	case browser.SessionEngineDetailLightpandaBinary:
		if b.lightpandaBinary == nil {
			return nil, browser.ErrTabNotFound
		}
		return b.lightpandaBinary.CaptureSessionMonitor(ctx, id)
	case browser.SessionEngineDetailChromiumManaged, browser.SessionEngineDetailChromiumRelay:
		chromiumBackend := b.chromiumForTarget(ctx, id)
		if chromiumBackend == nil {
			return nil, browser.ErrTabNotFound
		}
		return chromiumBackend.CaptureSessionMonitor(ctx, id)
	default:
		return nil, browser.ErrTabNotFound
	}
}

// CaptureBrowserSessionScreenshot preserves the legacy screenshot endpoint.
func (b *HybridCapabilityBrowserBackend) CaptureBrowserSessionScreenshot(ctx context.Context, id string) (*browser.SessionScreenshotResponse, error) {
	switch b.targetDetail(ctx, id) {
	case browser.SessionEngineDetailLightpandaShim:
		return b.lightpanda.CaptureScreenshot(id)
	case browser.SessionEngineDetailLightpandaBinary:
		if b.lightpandaBinary == nil {
			return nil, browser.ErrTabNotFound
		}
		return b.lightpandaBinary.CaptureSessionScreenshot(ctx, id)
	case browser.SessionEngineDetailChromiumManaged, browser.SessionEngineDetailChromiumRelay:
		chromiumBackend := b.chromiumForTarget(ctx, id)
		if chromiumBackend == nil {
			return nil, browser.ErrTabNotFound
		}
		return chromiumBackend.CaptureSessionScreenshot(ctx, id)
	default:
		return nil, browser.ErrTabNotFound
	}
}
