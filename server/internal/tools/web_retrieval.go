package tools

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	webFetchStrategyHTTP    = "http"
	webFetchStrategySession = "session_http"
	webFetchStrategyBrowser = "browser"
	webFetchStrategyProxy   = "proxy_fetcher"
	webFetchStrategyAdapter = "adapter"

	webFetchChallengePolicyTypedHandoff = "typed_handoff"

	webFetchChallengeKindNone            = "none"
	webFetchChallengeKindSoft            = "soft"
	webFetchChallengeKindHard            = "hard"
	webFetchChallengeKindLogin           = "login"
	webFetchChallengeKindBrowserRequired = "browser_required"

	webRetrievalDefaultSessionTTL = 30 * time.Minute
	webRetrievalDefaultDomainTTL  = 6 * time.Hour
	webRetrievalDefaultAdapterTTL = 24 * time.Hour
	webRetrievalDefaultMaxItems   = 256
)

type ChallengeState struct {
	Kind            string `json:"kind,omitempty"`
	Code            string `json:"code,omitempty"`
	Message         string `json:"message,omitempty"`
	ResumeAction    string `json:"resume_action,omitempty"`
	RequiresBrowser bool   `json:"requires_browser,omitempty"`
	RequiresHuman   bool   `json:"requires_human,omitempty"`
}

type FetchSession struct {
	Key              string    `json:"key,omitempty"`
	Host             string    `json:"host,omitempty"`
	BrowserTargetID  string    `json:"browser_target_id,omitempty"`
	LastLane         string    `json:"last_lane,omitempty"`
	SessionReusable  bool      `json:"session_reusable,omitempty"`
	NeedsRealBrowser bool      `json:"needs_real_browser,omitempty"`
	UpdatedAt        time.Time `json:"updated_at,omitempty"`
}

type DomainStrategy struct {
	Host                string    `json:"host,omitempty"`
	PreferredLane       string    `json:"preferred_lane,omitempty"`
	AdapterID           string    `json:"adapter_id,omitempty"`
	NeedsRealBrowser    bool      `json:"needs_real_browser,omitempty"`
	NeedsNetworkObserve bool      `json:"needs_network_observe,omitempty"`
	LoginRequired       bool      `json:"login_required,omitempty"`
	Confidence          float64   `json:"confidence,omitempty"`
	Hits                int       `json:"hits,omitempty"`
	UpdatedAt           time.Time `json:"updated_at,omitempty"`
}

type AdapterManifest struct {
	ID                  string    `json:"id,omitempty"`
	Host                string    `json:"host,omitempty"`
	PreferredLane       string    `json:"preferred_lane,omitempty"`
	WaitSelectors       []string  `json:"wait_selectors,omitempty"`
	APIEndpointPatterns []string  `json:"api_endpoint_patterns,omitempty"`
	ExtractionKeys      []string  `json:"extraction_keys,omitempty"`
	AuthExpectation     string    `json:"auth_expectation,omitempty"`
	Confidence          float64   `json:"confidence,omitempty"`
	NetworkObserved     bool      `json:"network_observed,omitempty"`
	CreatedAt           time.Time `json:"created_at,omitempty"`
	UpdatedAt           time.Time `json:"updated_at,omitempty"`
}

type FetchRequest struct {
	URL               string
	Mode              string
	MaxChars          int
	PreferredLane     string
	Options           webFetchRequestOptions
	AllowBrowser      bool
	AllowProxy        bool
	AllowSession      bool
	AllowAutoFallback bool
	BrowserReasonCode string
	BrowserReason     string
}

type FetchResult struct {
	Payload         webFetchPayload
	StrategyUsed    string
	SessionReused   bool
	AdapterID       string
	NetworkObserved bool
	ChallengeState  *ChallengeState
	BrowserTargetID string
	ObservedNetwork []BrowserNetworkEvent
}

type Fetcher interface {
	Name() string
	Fetch(context.Context, *FetchOrchestrator, FetchRequest) (FetchResult, error)
}

type FetchOrchestrator struct {
	tool     *WebFetchTool
	memory   *webRetrievalMemory
	fetchers map[string]Fetcher
}

type webRetrievalMemory struct {
	mu sync.Mutex

	sessionTTL time.Duration
	domainTTL  time.Duration
	adapterTTL time.Duration

	maxSessions int
	maxDomains  int
	maxAdapters int

	sessions map[string]webRetrievalSessionEntry
	domains  map[string]webRetrievalDomainEntry
	adapters map[string]webRetrievalAdapterEntry
}

type webRetrievalSessionEntry struct {
	Value     FetchSession
	ExpiresAt time.Time
}

type webRetrievalDomainEntry struct {
	Value     DomainStrategy
	ExpiresAt time.Time
}

type webRetrievalAdapterEntry struct {
	Value     AdapterManifest
	ExpiresAt time.Time
}

type webFetchHTTPFetcher struct{}
type webFetchSessionFetcher struct{}
type webFetchLightpandaShimFetcher struct{}
type webFetchBrowserFetcher struct{}
type webFetchProxyFetcherImpl struct{}

func newFetchOrchestrator(tool *WebFetchTool) *FetchOrchestrator {
	if tool == nil {
		return nil
	}
	o := &FetchOrchestrator{
		tool:   tool,
		memory: newWebRetrievalMemory(),
		fetchers: map[string]Fetcher{
			webAccessLaneHTTP:           webFetchHTTPFetcher{},
			webAccessLaneHTTPNative:     webFetchHTTPFetcher{},
			webFetchStrategySession:     webFetchSessionFetcher{},
			webAccessLaneLightpandaShim: webFetchLightpandaShimFetcher{},
			webAccessLaneBrowser:        webFetchBrowserFetcher{},
			webAccessLaneProxyFetcher:   webFetchProxyFetcherImpl{},
		},
	}
	return o
}

func newWebRetrievalMemory() *webRetrievalMemory {
	return &webRetrievalMemory{
		sessionTTL:  webRetrievalDefaultSessionTTL,
		domainTTL:   webRetrievalDefaultDomainTTL,
		adapterTTL:  webRetrievalDefaultAdapterTTL,
		maxSessions: webRetrievalDefaultMaxItems,
		maxDomains:  webRetrievalDefaultMaxItems,
		maxAdapters: webRetrievalDefaultMaxItems,
		sessions:    make(map[string]webRetrievalSessionEntry),
		domains:     make(map[string]webRetrievalDomainEntry),
		adapters:    make(map[string]webRetrievalAdapterEntry),
	}
}

func (m *webRetrievalMemory) loadSession(key string) (FetchSession, bool) {
	if m == nil || strings.TrimSpace(key) == "" {
		return FetchSession{}, false
	}
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.sessions[key]
	if !ok {
		return FetchSession{}, false
	}
	if entry.ExpiresAt.Before(now) {
		delete(m.sessions, key)
		return FetchSession{}, false
	}
	return entry.Value, true
}

func (m *webRetrievalMemory) storeSession(value FetchSession) {
	if m == nil || strings.TrimSpace(value.Key) == "" {
		return
	}
	value.UpdatedAt = time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[value.Key] = webRetrievalSessionEntry{
		Value:     value,
		ExpiresAt: value.UpdatedAt.Add(m.sessionTTL),
	}
	m.evictExpiredLocked()
	evictWebRetrievalOverflowLocked(m.maxSessions, m.sessions, func(key string) { delete(m.sessions, key) })
}

func (m *webRetrievalMemory) loadDomain(host string) (DomainStrategy, bool) {
	if m == nil || strings.TrimSpace(host) == "" {
		return DomainStrategy{}, false
	}
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.domains[host]
	if !ok {
		return DomainStrategy{}, false
	}
	if entry.ExpiresAt.Before(now) {
		delete(m.domains, host)
		return DomainStrategy{}, false
	}
	return entry.Value, true
}

func (m *webRetrievalMemory) storeDomain(value DomainStrategy) {
	host := strings.ToLower(strings.TrimSpace(value.Host))
	if m == nil || host == "" {
		return
	}
	value.Host = host
	value.UpdatedAt = time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.domains[host] = webRetrievalDomainEntry{
		Value:     value,
		ExpiresAt: value.UpdatedAt.Add(m.domainTTL),
	}
	m.evictExpiredLocked()
	evictWebRetrievalOverflowLocked(m.maxDomains, m.domains, func(key string) { delete(m.domains, key) })
}

func (m *webRetrievalMemory) loadAdapter(host string) (AdapterManifest, bool) {
	if m == nil || strings.TrimSpace(host) == "" {
		return AdapterManifest{}, false
	}
	now := time.Now()
	host = strings.ToLower(strings.TrimSpace(host))
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.adapters[host]
	if !ok {
		return AdapterManifest{}, false
	}
	if entry.ExpiresAt.Before(now) {
		delete(m.adapters, host)
		return AdapterManifest{}, false
	}
	return entry.Value, true
}

func (m *webRetrievalMemory) storeAdapter(value AdapterManifest) {
	host := strings.ToLower(strings.TrimSpace(value.Host))
	if m == nil || host == "" || strings.TrimSpace(value.ID) == "" {
		return
	}
	value.Host = host
	now := time.Now()
	if value.CreatedAt.IsZero() {
		value.CreatedAt = now
	}
	value.UpdatedAt = now
	m.mu.Lock()
	defer m.mu.Unlock()
	m.adapters[host] = webRetrievalAdapterEntry{
		Value:     value,
		ExpiresAt: value.UpdatedAt.Add(m.adapterTTL),
	}
	m.evictExpiredLocked()
	evictWebRetrievalOverflowLocked(m.maxAdapters, m.adapters, func(key string) { delete(m.adapters, key) })
}

func (m *webRetrievalMemory) evictExpiredLocked() {
	now := time.Now()
	for key, entry := range m.sessions {
		if entry.ExpiresAt.Before(now) {
			delete(m.sessions, key)
		}
	}
	for key, entry := range m.domains {
		if entry.ExpiresAt.Before(now) {
			delete(m.domains, key)
		}
	}
	for key, entry := range m.adapters {
		if entry.ExpiresAt.Before(now) {
			delete(m.adapters, key)
		}
	}
}

func evictWebRetrievalOverflowLocked[T any](limit int, entries map[string]T, remove func(string)) {
	if limit <= 0 || len(entries) <= limit {
		return
	}
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for len(keys) > limit {
		remove(keys[0])
		keys = keys[1:]
	}
}

func (o *FetchOrchestrator) enabled() bool {
	return o != nil && o.tool != nil && o.tool.config.LayeredFetchEnabled
}

func (o *FetchOrchestrator) preferredReadLane(rawURL string) string {
	if o == nil || !o.tool.config.DomainStrategyEnabled {
		return ""
	}
	host := webFetchHostForURL(rawURL)
	if host == "" {
		return ""
	}
	if webFetchPrefersBrowserHost(host) && o.tool != nil && o.tool.browser != nil {
		return webAccessLaneBrowser
	}
	if adapter, ok := o.memory.loadAdapter(host); ok && strings.TrimSpace(adapter.PreferredLane) != "" {
		return strings.TrimSpace(adapter.PreferredLane)
	}
	if strategy, ok := o.memory.loadDomain(host); ok {
		return strings.TrimSpace(strategy.PreferredLane)
	}
	return ""
}

func (o *FetchOrchestrator) Fetch(ctx context.Context, req FetchRequest) (FetchResult, error) {
	if o == nil || o.tool == nil {
		return FetchResult{}, errors.New("web retrieval orchestrator not available")
	}
	host := webFetchHostForURL(req.URL)
	autoAllowed := req.AllowAutoFallback && o.tool.autoFallbackAllowed(host)
	sessionKey := o.tool.retrievalSessionKey(ctx, host)

	session, hasSession := FetchSession{}, false
	if o.tool.config.SessionMemoryEnabled {
		session, hasSession = o.memory.loadSession(sessionKey)
	}
	strategy, hasStrategy := DomainStrategy{}, false
	if o.tool.config.DomainStrategyEnabled {
		strategy, hasStrategy = o.memory.loadDomain(host)
	}
	adapter, hasAdapter := AdapterManifest{}, false
	if o.tool.config.AdapterMemoryEnabled {
		adapter, hasAdapter = o.memory.loadAdapter(host)
	}

	effectiveTargetID := strings.TrimSpace(req.Options.browserTargetID)
	sessionReused := false
	if effectiveTargetID == "" && hasSession && strings.TrimSpace(session.BrowserTargetID) != "" {
		effectiveTargetID = strings.TrimSpace(session.BrowserTargetID)
		sessionReused = true
	}

	lanes := o.planLanes(req, autoAllowed, effectiveTargetID != "", hasStrategy, strategy, hasAdapter, adapter)
	var (
		bestResult FetchResult
		bestScore  = -1
		errs       []error
	)
	for _, lane := range lanes {
		fetcher, ok := o.fetchers[lane]
		if !ok {
			continue
		}
		reqWithSession := req
		reqWithSession.Options = cloneWebFetchRequestOptions(req.Options)
		reqWithSession.Options.browserTargetID = effectiveTargetID
		result, err := fetcher.Fetch(ctx, o, reqWithSession)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", lane, err))
			continue
		}
		result.SessionReused = result.SessionReused || sessionReused
		if result.StrategyUsed == "" {
			result.StrategyUsed = lane
		}
		if result.ChallengeState == nil {
			result.ChallengeState = classifyWebFetchChallengeState(result.Payload.WarningCode, result.Payload.Warning, result.Payload.Content)
		}
		result.Payload.StrategyUsed = result.StrategyUsed
		result.Payload.SessionReused = result.SessionReused
		result.Payload.AdapterID = result.AdapterID
		result.Payload.NetworkObserved = result.NetworkObserved
		result.Payload.ChallengeState = cloneChallengeState(result.ChallengeState)

		score := scoreFetchResult(result)
		if score > bestScore {
			bestScore = score
			bestResult = result
		}

		if fetchResultStrong(result) || (result.StrategyUsed == webFetchStrategyBrowser && strings.TrimSpace(result.Payload.Content) != "") {
			o.rememberSuccess(ctx, host, sessionKey, reqWithSession, result)
			return result, nil
		}
		if result.ChallengeState != nil && result.ChallengeState.Kind == webFetchChallengeKindHard && result.StrategyUsed == webFetchStrategyBrowser {
			o.rememberOutcome(host, result)
			return result, nil
		}
		o.rememberOutcome(host, result)
	}
	if bestScore >= 0 {
		return bestResult, nil
	}
	return FetchResult{}, errors.Join(errs...)
}

func (o *FetchOrchestrator) planLanes(req FetchRequest, autoAllowed bool, hasBrowserTarget bool, hasStrategy bool, strategy DomainStrategy, hasAdapter bool, adapter AdapterManifest) []string {
	ordered := make([]string, 0, 5)
	host := webFetchHostForURL(req.URL)
	browserPreferred := webFetchPrefersBrowserHost(host) && req.AllowBrowser
	lightpandaAllowed := webFetchSupportsLightpandaHost(host)
	appendLane := func(lane string) {
		lane = strings.TrimSpace(lane)
		if lane == "" {
			return
		}
		for _, existing := range ordered {
			if existing == lane {
				return
			}
		}
		ordered = append(ordered, lane)
	}

	switch strings.TrimSpace(req.PreferredLane) {
	case webAccessLaneHTTPNative:
		appendLane(webAccessLaneHTTPNative)
	case webAccessLaneHTTP:
		if req.AllowSession && hasBrowserTarget {
			appendLane(webFetchStrategySession)
		}
		appendLane(webAccessLaneHTTP)
	case webAccessLaneLightpandaShim:
		if lightpandaAllowed {
			appendLane(webAccessLaneLightpandaShim)
		}
	case webAccessLaneBrowser:
		appendLane(webAccessLaneBrowser)
	case webAccessLaneProxyFetcher:
		appendLane(webAccessLaneProxyFetcher)
	}
	if len(ordered) > 0 {
		return ordered
	}
	if browserPreferred {
		if req.AllowSession && hasBrowserTarget {
			appendLane(webFetchStrategySession)
		}
		appendLane(webAccessLaneBrowser)
	}

	if autoAllowed && hasAdapter {
		switch strings.TrimSpace(adapter.PreferredLane) {
		case webAccessLaneLightpandaShim:
			if lightpandaAllowed {
				appendLane(webAccessLaneLightpandaShim)
			}
		case webAccessLaneBrowser:
			appendLane(webAccessLaneBrowser)
		case webAccessLaneProxyFetcher:
			appendLane(webAccessLaneProxyFetcher)
		case webFetchStrategySession:
			if req.AllowSession && hasBrowserTarget {
				appendLane(webFetchStrategySession)
			}
		case webAccessLaneHTTP, webAccessLaneHTTPNative:
			appendLane(adapter.PreferredLane)
		}
	}
	if autoAllowed && hasStrategy {
		switch strings.TrimSpace(strategy.PreferredLane) {
		case webAccessLaneBrowser, webAccessLaneProxyFetcher, webAccessLaneHTTPNative, webAccessLaneHTTP:
			appendLane(strategy.PreferredLane)
		case webAccessLaneLightpandaShim:
			if lightpandaAllowed {
				appendLane(strategy.PreferredLane)
			}
		case webFetchStrategySession:
			if req.AllowSession && hasBrowserTarget {
				appendLane(webFetchStrategySession)
			}
		}
	}
	if req.AllowSession && hasBrowserTarget {
		appendLane(webFetchStrategySession)
	}
	appendLane(webAccessLaneHTTP)
	appendLane(webAccessLaneHTTPNative)
	if lightpandaAllowed {
		appendLane(webAccessLaneLightpandaShim)
	}
	if req.AllowProxy {
		appendLane(webAccessLaneProxyFetcher)
	}
	if req.AllowBrowser {
		appendLane(webAccessLaneBrowser)
	}
	return ordered
}

func (o *FetchOrchestrator) rememberSuccess(ctx context.Context, host, sessionKey string, req FetchRequest, result FetchResult) {
	o.rememberOutcome(host, result)
	if !o.tool.config.SessionMemoryEnabled {
		return
	}
	targetID := strings.TrimSpace(firstNonEmpty(result.BrowserTargetID, req.Options.browserTargetID))
	if targetID == "" || strings.TrimSpace(sessionKey) == "" {
		return
	}
	o.memory.storeSession(FetchSession{
		Key:              sessionKey,
		Host:             host,
		BrowserTargetID:  targetID,
		LastLane:         result.StrategyUsed,
		SessionReusable:  true,
		NeedsRealBrowser: result.StrategyUsed == webFetchStrategyBrowser,
	})
}

func (o *FetchOrchestrator) rememberOutcome(host string, result FetchResult) {
	if !o.tool.config.DomainStrategyEnabled || strings.TrimSpace(host) == "" {
		return
	}
	strategy, _ := o.memory.loadDomain(host)
	strategy.Host = host
	strategy.Hits++
	strategy.PreferredLane = normalizeFetchStrategyLane(result.StrategyUsed)
	strategy.AdapterID = strings.TrimSpace(result.AdapterID)
	strategy.NeedsRealBrowser = result.StrategyUsed == webFetchStrategyBrowser || (result.ChallengeState != nil && result.ChallengeState.RequiresBrowser)
	strategy.NeedsNetworkObserve = result.NetworkObserved
	if result.ChallengeState != nil {
		strategy.LoginRequired = strategy.LoginRequired || result.ChallengeState.Kind == webFetchChallengeKindLogin
	}
	strategy.Confidence = maxFetchConfidence(strategy.Confidence, confidenceForFetchResult(result))
	o.memory.storeDomain(strategy)

	if !o.tool.config.AdapterMemoryEnabled || strings.TrimSpace(result.AdapterID) == "" {
		return
	}
	if len(result.ObservedNetwork) == 0 {
		return
	}
	adapter := synthesizeAdapterManifest(host, result)
	if strings.TrimSpace(adapter.ID) == "" {
		return
	}
	o.memory.storeAdapter(adapter)
}

func (o *FetchOrchestrator) noteSessionUse(ctx context.Context, targetURL, targetID, strategy string) {
	if !o.tool.config.SessionMemoryEnabled {
		return
	}
	host := webFetchHostForURL(targetURL)
	sessionKey := o.tool.retrievalSessionKey(ctx, host)
	if strings.TrimSpace(sessionKey) == "" || strings.TrimSpace(targetID) == "" {
		return
	}
	o.memory.storeSession(FetchSession{
		Key:             sessionKey,
		Host:            host,
		BrowserTargetID: targetID,
		LastLane:        strategy,
		SessionReusable: true,
	})
}

func (webFetchHTTPFetcher) Name() string { return webAccessLaneHTTP }

func (webFetchHTTPFetcher) Fetch(ctx context.Context, o *FetchOrchestrator, req FetchRequest) (FetchResult, error) {
	backend := strings.TrimSpace(req.PreferredLane)
	if backend != webAccessLaneHTTPNative {
		backend = ""
	}
	opts := cloneWebFetchRequestOptions(req.Options)
	result, err := o.tool.fetchHTTPResultViaExplicitBackend(ctx, req.URL, opts, backend)
	if err != nil {
		return FetchResult{}, err
	}
	payload, err := o.tool.buildPayloadFromHTTPResult(ctx, req.Mode, result)
	strategyUsed := webFetchStrategyHTTP
	if strings.TrimSpace(result.Source) == webAccessSourceHTTPNative || strings.TrimSpace(backend) == webAccessLaneHTTPNative {
		strategyUsed = webAccessLaneHTTPNative
	}
	if err != nil {
		if payload.Source == "" {
			payload.Source = firstNonEmpty(strings.TrimSpace(result.Source), webAccessSourceHTTP)
		}
		payload.StrategyUsed = strategyUsed
		return FetchResult{Payload: payload, StrategyUsed: strategyUsed}, err
	}
	payload.StrategyUsed = strategyUsed
	return FetchResult{
		Payload:        payload,
		StrategyUsed:   strategyUsed,
		ChallengeState: classifyWebFetchChallengeState(payload.WarningCode, payload.Warning, payload.Content),
	}, nil
}

func (webFetchSessionFetcher) Name() string { return webFetchStrategySession }

func (webFetchSessionFetcher) Fetch(ctx context.Context, o *FetchOrchestrator, req FetchRequest) (FetchResult, error) {
	if strings.TrimSpace(req.Options.browserTargetID) == "" {
		return FetchResult{}, errors.New("browser session target is required")
	}
	opts := cloneWebFetchRequestOptions(req.Options)
	if opts.extraHeaders == nil {
		opts.extraHeaders = make(map[string]string)
	}
	if err := o.tool.applyBrowserSessionCookies(ctx, req.URL, &opts); err != nil {
		return FetchResult{}, err
	}
	result, err := o.tool.fetchHTTPResult(ctx, req.URL, opts, defaultWebFetchAcceptHeader())
	if err != nil {
		return FetchResult{}, err
	}
	payload, buildErr := o.tool.buildPayloadFromHTTPResult(ctx, req.Mode, result)
	if buildErr != nil {
		payload.StrategyUsed = webFetchStrategySession
		payload.SessionReused = true
		return FetchResult{
			Payload:         payload,
			StrategyUsed:    webFetchStrategySession,
			SessionReused:   true,
			BrowserTargetID: req.Options.browserTargetID,
			ChallengeState:  classifyWebFetchChallengeState(payload.WarningCode, payload.Warning, payload.Content),
		}, buildErr
	}
	o.noteSessionUse(ctx, req.URL, req.Options.browserTargetID, webFetchStrategySession)
	payload.StrategyUsed = webFetchStrategySession
	payload.SessionReused = true
	return FetchResult{
		Payload:         payload,
		StrategyUsed:    webFetchStrategySession,
		SessionReused:   true,
		BrowserTargetID: req.Options.browserTargetID,
		ChallengeState:  classifyWebFetchChallengeState(payload.WarningCode, payload.Warning, payload.Content),
	}, nil
}

func (webFetchLightpandaShimFetcher) Name() string { return webAccessLaneLightpandaShim }

func (webFetchLightpandaShimFetcher) Fetch(ctx context.Context, o *FetchOrchestrator, req FetchRequest) (FetchResult, error) {
	payload, err := o.tool.fetchViaLightpandaShim(ctx, req.URL, req.Mode)
	if err != nil {
		return FetchResult{}, err
	}
	payload.StrategyUsed = webAccessLaneLightpandaShim
	return FetchResult{
		Payload:        payload,
		StrategyUsed:   webAccessLaneLightpandaShim,
		ChallengeState: classifyWebFetchChallengeState(payload.WarningCode, payload.Warning, payload.Content),
	}, nil
}

func (webFetchBrowserFetcher) Name() string { return webAccessLaneBrowser }

func (webFetchBrowserFetcher) Fetch(ctx context.Context, o *FetchOrchestrator, req FetchRequest) (FetchResult, error) {
	detailed, err := o.tool.fetchViaBrowserSessionDetailed(ctx, req.URL, req.Mode, req.Options.browserTargetID, req.BrowserReasonCode, req.BrowserReason)
	if err != nil {
		return FetchResult{}, err
	}
	result := FetchResult{
		Payload:         detailed.Payload,
		StrategyUsed:    webFetchStrategyBrowser,
		BrowserTargetID: detailed.TargetID,
		ChallengeState:  classifyWebFetchChallengeState(detailed.Payload.WarningCode, detailed.Payload.Warning, detailed.Payload.Content),
	}
	if detailed.TargetID != "" {
		o.noteSessionUse(ctx, req.URL, detailed.TargetID, webFetchStrategyBrowser)
	}
	if o.tool.config.NetworkObserveEnabled && o.tool.browser != nil && detailed.TargetID != "" {
		_ = o.tool.browser.WaitNetworkIdle(ctx, detailed.TargetID, 400, 2_000)
		if observed, observeErr := o.tool.browser.ObserveNetwork(ctx, detailed.TargetID, 24, false); observeErr == nil {
			result.NetworkObserved = len(observed.Events) > 0
			result.ObservedNetwork = append(result.ObservedNetwork, observed.Events...)
			if adapter := synthesizeAdapterManifest(webFetchHostForURL(req.URL), result); strings.TrimSpace(adapter.ID) != "" {
				result.AdapterID = adapter.ID
			}
		}
	}
	result.Payload.StrategyUsed = webFetchStrategyBrowser
	result.Payload.SessionReused = strings.TrimSpace(detailed.TargetID) == strings.TrimSpace(req.Options.browserTargetID) && strings.TrimSpace(req.Options.browserTargetID) != ""
	result.Payload.AdapterID = result.AdapterID
	result.Payload.NetworkObserved = result.NetworkObserved
	return result, nil
}

func (webFetchProxyFetcherImpl) Name() string { return webAccessLaneProxyFetcher }

func (webFetchProxyFetcherImpl) Fetch(ctx context.Context, o *FetchOrchestrator, req FetchRequest) (FetchResult, error) {
	payload, err := o.tool.tryProxyFetchFamily(ctx, req.URL, req.Mode)
	if err != nil {
		return FetchResult{}, err
	}
	payload.StrategyUsed = webFetchStrategyProxy
	return FetchResult{
		Payload:        payload,
		StrategyUsed:   webFetchStrategyProxy,
		ChallengeState: classifyWebFetchChallengeState(payload.WarningCode, payload.Warning, payload.Content),
	}, nil
}

func (w *WebFetchTool) retrievalSessionKey(ctx context.Context, host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return ""
	}
	sessionID := strings.TrimSpace(GetSessionID(ctx))
	userID := strings.TrimSpace(GetUserID(ctx))
	switch {
	case sessionID != "":
		return sessionID + "|" + host
	case userID != "":
		return userID + "|" + host
	default:
		return ""
	}
}

func (w *WebFetchTool) autoFallbackAllowed(host string) bool {
	if w == nil {
		return false
	}
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return false
	}
	if len(w.config.AutoFallbackHosts) == 0 {
		return true
	}
	return webFetchHostListContains(w.config.AutoFallbackHosts, host)
}

func (w *WebFetchTool) preferredReadLane(ctx context.Context, rawURL string, browserTargetID string) string {
	if w == nil || w.orchestrator == nil || !w.config.LayeredFetchEnabled {
		return ""
	}
	host := webFetchHostForURL(rawURL)
	if host == "" {
		return ""
	}
	if strings.TrimSpace(browserTargetID) != "" && w.config.SessionMemoryEnabled {
		return webFetchStrategySession
	}
	return w.orchestrator.preferredReadLane(rawURL)
}

func cloneChallengeState(state *ChallengeState) *ChallengeState {
	if state == nil {
		return nil
	}
	cloned := *state
	return &cloned
}

func cloneWebFetchRequestOptions(opts webFetchRequestOptions) webFetchRequestOptions {
	cloned := opts
	if len(opts.extraHeaders) > 0 {
		cloned.extraHeaders = make(map[string]string, len(opts.extraHeaders))
		for key, value := range opts.extraHeaders {
			cloned.extraHeaders[key] = value
		}
	}
	return cloned
}

func normalizeFetchStrategyLane(strategy string) string {
	switch strings.TrimSpace(strategy) {
	case webFetchStrategySession:
		return webFetchStrategySession
	case webFetchStrategyBrowser:
		return webAccessLaneBrowser
	case webFetchStrategyProxy:
		return webAccessLaneProxyFetcher
	case webAccessLaneHTTPNative:
		return webAccessLaneHTTPNative
	case webAccessLaneLightpandaShim:
		return webAccessLaneLightpandaShim
	default:
		return webAccessLaneHTTP
	}
}

func confidenceForFetchResult(result FetchResult) float64 {
	switch strings.TrimSpace(result.StrategyUsed) {
	case webFetchStrategyBrowser:
		if result.NetworkObserved {
			return 0.96
		}
		return 0.9
	case webFetchStrategySession:
		return 0.88
	case webAccessLaneLightpandaShim:
		return 0.74
	case webFetchStrategyProxy:
		return 0.8
	default:
		if fetchResultStrong(result) {
			return 0.72
		}
		return 0.4
	}
}

func maxFetchConfidence(current, next float64) float64 {
	if next > current {
		return next
	}
	return current
}

func scoreFetchResult(result FetchResult) int {
	score := len([]rune(strings.TrimSpace(result.Payload.Content)))
	switch strings.TrimSpace(result.StrategyUsed) {
	case webFetchStrategyBrowser:
		score += 200
	case webFetchStrategySession:
		score += 160
	case webAccessLaneLightpandaShim:
		score += 110
	case webFetchStrategyProxy:
		score += 120
	}
	if result.NetworkObserved {
		score += 24
	}
	if strings.TrimSpace(result.AdapterID) != "" {
		score += 18
	}
	if result.ChallengeState != nil {
		switch result.ChallengeState.Kind {
		case webFetchChallengeKindHard:
			score -= 260
		case webFetchChallengeKindSoft, webFetchChallengeKindBrowserRequired, webFetchChallengeKindLogin:
			score -= 120
		}
	}
	if strings.TrimSpace(result.Payload.WarningCode) != "" {
		score -= 80
	}
	return score
}

func fetchResultStrong(result FetchResult) bool {
	if strings.TrimSpace(result.Payload.WarningCode) != "" {
		return false
	}
	return len([]rune(strings.TrimSpace(result.Payload.Content))) >= webFetchMinReadableChars
}

func classifyWebFetchChallengeState(code, warning, content string) *ChallengeState {
	code = strings.TrimSpace(code)
	warning = strings.TrimSpace(warning)
	contentLower := strings.ToLower(content)
	switch code {
	case webFetchWarningCodeLoginWall:
		return &ChallengeState{
			Kind:            webFetchChallengeKindLogin,
			Code:            code,
			Message:         firstNonEmpty(warning, "login wall detected"),
			ResumeAction:    "browser",
			RequiresBrowser: true,
		}
	case webFetchWarningCodeBrowserRequired:
		return &ChallengeState{
			Kind:            webFetchChallengeKindBrowserRequired,
			Code:            code,
			Message:         firstNonEmpty(warning, "verified browser session required"),
			ResumeAction:    "browser",
			RequiresBrowser: true,
		}
	case webFetchWarningCodeChallenge:
		if containsAnyWebFetchCue(webFetchHardChallengeMatcher, contentLower, strings.ToLower(warning)) {
			return &ChallengeState{
				Kind:            webFetchChallengeKindHard,
				Code:            code,
				Message:         firstNonEmpty(warning, "hard verification challenge detected"),
				ResumeAction:    "browser",
				RequiresBrowser: true,
				RequiresHuman:   true,
			}
		}
		return &ChallengeState{
			Kind:            webFetchChallengeKindSoft,
			Code:            code,
			Message:         firstNonEmpty(warning, "anti-bot or verification challenge detected"),
			ResumeAction:    "browser",
			RequiresBrowser: true,
		}
	}
	if containsAnyWebFetchCue(webFetchHardChallengeMatcher, contentLower) {
		return &ChallengeState{
			Kind:            webFetchChallengeKindHard,
			Code:            webFetchWarningCodeChallenge,
			Message:         "hard verification challenge detected",
			ResumeAction:    "browser",
			RequiresBrowser: true,
			RequiresHuman:   true,
		}
	}
	return nil
}

func synthesizeAdapterManifest(host string, result FetchResult) AdapterManifest {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" || len(result.ObservedNetwork) == 0 {
		return AdapterManifest{}
	}
	patterns := make([]string, 0, 4)
	seenPatterns := make(map[string]struct{})
	keys := make([]string, 0, 6)
	seenKeys := make(map[string]struct{})
	networkObserved := false
	for _, event := range result.ObservedNetwork {
		if strings.TrimSpace(event.URL) == "" {
			continue
		}
		networkObserved = true
		if pattern := webFetchEndpointPattern(event.URL); pattern != "" {
			if _, ok := seenPatterns[pattern]; !ok {
				seenPatterns[pattern] = struct{}{}
				patterns = append(patterns, pattern)
			}
		}
		for _, token := range webFetchSampleKeys(event.BodySample) {
			if _, ok := seenKeys[token]; ok {
				continue
			}
			seenKeys[token] = struct{}{}
			keys = append(keys, token)
			if len(keys) >= 6 {
				break
			}
		}
	}
	if !networkObserved {
		return AdapterManifest{}
	}
	preferredLane := webAccessLaneBrowser
	if strings.TrimSpace(result.StrategyUsed) == webFetchStrategyProxy {
		preferredLane = webAccessLaneProxyFetcher
	}
	id := fmt.Sprintf("adapter:%s:%s", host, preferredLane)
	authExpectation := "public"
	if result.SessionReused || strings.TrimSpace(result.BrowserTargetID) != "" {
		authExpectation = "browser_session"
	}
	return AdapterManifest{
		ID:                  id,
		Host:                host,
		PreferredLane:       preferredLane,
		APIEndpointPatterns: patterns,
		ExtractionKeys:      keys,
		AuthExpectation:     authExpectation,
		Confidence:          confidenceForFetchResult(result),
		NetworkObserved:     true,
	}
}

func webFetchEndpointPattern(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if host == "" {
		return ""
	}
	segments := strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/")
	if len(segments) > 4 {
		segments = segments[:4]
	}
	if len(segments) == 0 {
		return host
	}
	return host + "/" + strings.Join(segments, "/")
}

func webFetchSampleKeys(sample string) []string {
	sample = strings.TrimSpace(sample)
	if sample == "" {
		return nil
	}
	replacer := strings.NewReplacer("{", " ", "}", " ", "[", " ", "]", " ", "\"", " ", ",", " ", ":", " ")
	parts := strings.Fields(replacer.Replace(sample))
	keys := make([]string, 0, 4)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || len(part) < 3 {
			continue
		}
		if !strings.ContainsAny(part, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_") {
			continue
		}
		keys = append(keys, part)
		if len(keys) >= 4 {
			break
		}
	}
	return keys
}

func webFetchHostForURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(parsed.Hostname()), "."))
}
