package browser

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// RodService implements the Service interface using Rod.
type RodService struct {
	config              *Config
	pool                *Pool
	security            *SecurityChecker
	recipes             *RecipeRegistry
	tabs                map[string]*tabInfo
	tabsMu              sync.RWMutex
	screenshotHistory   map[string][]SessionScreenshot
	historyMu           sync.RWMutex
	monitorFrameHook    func(targetID string, capturedAt string)
	monitorActivityHook func(targetID string, observedAt string)
	started             bool
	mu                  sync.RWMutex
}

const (
	maxSessionScreenshotHistory   = 24
	browserNetworkBodySampleLimit = 512
	detachedMonitorTargetID       = "monitor-detached-latest"
)

// tabInfo stores information about an open tab.
type tabInfo struct {
	page      *rod.Page
	browser   *rod.Browser
	targetID  string
	url       string
	title     string
	active    bool
	detached  bool
	console   []ConsoleMessage
	network   *networkCaptureState
	networkMu sync.Mutex
}

type networkCaptureState struct {
	mu           sync.Mutex
	cancel       func()
	page         *rod.Page
	lastActivity time.Time
	events       []ObservedNetworkEvent
	pending      map[proto.NetworkRequestID]*observedNetworkBuilder
	targetID     string
	onIdle       func(targetID string, observedAt string)
	lastIdleEmit time.Time
}

type observedNetworkBuilder struct {
	Method       string
	URL          string
	Status       int
	ContentType  string
	ResourceType string
	Initiator    string
	Headers      map[string]string
	StartedAt    time.Time
}

// NewService creates a new browser service.
func NewService(config *Config) (*RodService, error) {
	if config == nil {
		config = DefaultConfig()
	}

	pool, err := NewPool(config)
	if err != nil {
		return nil, err
	}

	return &RodService{
		config:            config,
		pool:              pool,
		security:          NewSecurityChecker(config),
		recipes:           NewRecipeRegistry(),
		tabs:              make(map[string]*tabInfo),
		screenshotHistory: make(map[string][]SessionScreenshot),
	}, nil
}

func (s *RodService) currentBrowserSecurityConfig() BrowserSecurityConfig {
	if s == nil {
		return BrowserSecurityConfig{
			AllowedDomains: []string{},
			BlockedDomains: []string{},
		}
	}
	s.mu.RLock()
	cfg := s.config.Clone()
	s.mu.RUnlock()
	return BrowserSecurityConfig{
		AllowedDomains: append([]string(nil), cfg.AllowedDomains...),
		BlockedDomains: append([]string(nil), cfg.BlockedDomains...),
	}
}

func (s *RodService) replaceBrowserSecurityConfig(config BrowserSecurityConfig) BrowserSecurityConfig {
	if s == nil {
		return BrowserSecurityConfig{
			AllowedDomains: []string{},
			BlockedDomains: []string{},
		}
	}
	normalized := BrowserSecurityConfig{
		AllowedDomains: normalizeBrowserDomainList(config.AllowedDomains),
		BlockedDomains: normalizeBrowserDomainList(config.BlockedDomains),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.config == nil {
		s.config = DefaultConfig()
	}
	s.config.AllowedDomains = append([]string(nil), normalized.AllowedDomains...)
	s.config.BlockedDomains = append([]string(nil), normalized.BlockedDomains...)
	s.security = NewSecurityChecker(s.config)
	return BrowserSecurityConfig{
		AllowedDomains: append([]string(nil), s.config.AllowedDomains...),
		BlockedDomains: append([]string(nil), s.config.BlockedDomains...),
	}
}

func (s *RodService) validateBrowserURL(rawURL string) error {
	if s == nil {
		return ErrBrowserNotAvailable
	}
	s.mu.RLock()
	cfg := s.config.Clone()
	s.mu.RUnlock()
	_, err := NewSecurityChecker(cfg).NormalizeAndCheckURL(rawURL)
	return err
}

// UsesRelayDriver reports whether this service attaches to an external or built-in relay/CDP endpoint.
func (s *RodService) UsesRelayDriver() bool {
	return s != nil && s.config != nil && s.config.UsesRelayDriver()
}

// isConnectionClosed checks if an error indicates a dead WebSocket/TCP connection.
func isConnectionClosed(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, marker := range []string{
		"use of closed network connection",
		"connection reset",
		"broken pipe",
		"websocket: close",
		"eof",
		"connection closed",
		"target closed",
		"target crashed",
		"page crashed",
		"session closed",
		"session not found",
		"target detached",
		"inspector.detached",
		"browser has disconnected",
		"cannot find context with specified id",
		"rod panic during wait",
	} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}

type pageInfoSnapshot struct {
	URL   string
	Title string
}

func snapshotPageInfoFromInfo(info *proto.TargetTargetInfo) pageInfoSnapshot {
	if info == nil {
		return pageInfoSnapshot{}
	}
	return pageInfoSnapshot{
		URL:   info.URL,
		Title: info.Title,
	}
}

func checkedPageInfo(page *rod.Page) (*proto.TargetTargetInfo, error) {
	if page == nil {
		return nil, fmt.Errorf("page is not available")
	}
	info, err := page.Info()
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, fmt.Errorf("page info unavailable")
	}
	return info, nil
}

func snapshotPageInfo(page *rod.Page) pageInfoSnapshot {
	info, err := checkedPageInfo(page)
	if err != nil {
		return pageInfoSnapshot{}
	}
	return snapshotPageInfoFromInfo(info)
}

func safeRodPageCall(action string, fn func() error) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("rod panic during %s: %v", action, recovered)
		}
	}()
	return fn()
}

func safeRodPageResult[T any](action string, fn func() (T, error)) (result T, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("rod panic during %s: %v", action, recovered)
		}
	}()
	return fn()
}

func waitPageLoad(page *rod.Page, timeout time.Duration) error {
	if page == nil {
		return fmt.Errorf("page is not available")
	}
	return safeRodPageCall("wait load", func() error {
		if timeout > 0 {
			return page.Timeout(timeout).WaitLoad()
		}
		return page.WaitLoad()
	})
}

func waitPageIdle(page *rod.Page, timeout time.Duration) error {
	if page == nil {
		return fmt.Errorf("page is not available")
	}
	return safeRodPageCall("wait idle", func() error {
		return page.WaitIdle(timeout)
	})
}

func waitPageDOMStable(page *rod.Page, timeout time.Duration, threshold float64) error {
	if page == nil {
		return fmt.Errorf("page is not available")
	}
	return safeRodPageCall("wait dom stable", func() error {
		return page.WaitDOMStable(timeout, threshold)
	})
}

var (
	waitPageStableLoadFn    = waitPageLoad
	waitPageStableIdleFn    = waitPageIdle
	waitPageStableSleep     = time.Sleep
	waitPageReadyLoadFn     = waitPageLoad
	waitPageReadySelectorFn = func(page *rod.Page, selector string, timeout time.Duration) error {
		if page == nil {
			return fmt.Errorf("page is not available")
		}
		selector = strings.TrimSpace(selector)
		if selector == "" {
			return nil
		}
		return safeRodPageCall("wait selector", func() error {
			_, err := page.Timeout(timeout).Element(selector)
			return err
		})
	}
	waitPageReadySleep = time.Sleep
)

func waitPageStable(page *rod.Page, timeout time.Duration) error {
	if page == nil {
		return fmt.Errorf("page is not available")
	}
	// rod.Page.WaitStable has been observed to panic from an internal helper
	// goroutine under concurrent browser load, which bypasses our recover path
	// and takes down the whole gateway. Use a conservative approximation here
	// so browser fallback stays alive during harness concurrency.
	if timeout <= 0 {
		timeout = 500 * time.Millisecond
	}
	loadTimeout := timeout
	if loadTimeout > 2*time.Second {
		loadTimeout = 2 * time.Second
	}
	if err := waitPageStableLoadFn(page, loadTimeout); err != nil {
		return err
	}
	idleTimeout := timeout / 2
	if idleTimeout <= 0 {
		idleTimeout = 200 * time.Millisecond
	}
	if idleTimeout > time.Second {
		idleTimeout = time.Second
	}
	if err := waitPageStableIdleFn(page, idleTimeout); err == nil {
		return nil
	}
	settleDelay := timeout / 4
	if settleDelay <= 0 {
		settleDelay = 150 * time.Millisecond
	}
	if settleDelay > 500*time.Millisecond {
		settleDelay = 500 * time.Millisecond
	}
	waitPageStableSleep(settleDelay)
	return nil
}

func waitDetachedPageReady(page *rod.Page, timeout time.Duration, waitForSelector *string, skipWaitLoad bool, waitForMS int) error {
	if page == nil {
		return fmt.Errorf("page is not available")
	}
	if !skipWaitLoad {
		if err := waitPageReadyLoadFn(page, timeout); err != nil {
			return err
		}
	}
	if waitForSelector != nil {
		if err := waitPageReadySelectorFn(page, *waitForSelector, timeout); err != nil {
			return err
		}
	}
	if waitForMS > 0 {
		waitPageReadySleep(time.Duration(waitForMS) * time.Millisecond)
	}
	return nil
}

// removeTab cleans up a stale tab entry.
func (s *RodService) removeTab(tab *tabInfo) {
	if tab == nil {
		return
	}
	tab.networkMu.Lock()
	if tab.network != nil && tab.network.cancel != nil {
		tab.network.cancel()
	}
	tab.network = nil
	tab.networkMu.Unlock()
	s.tabsMu.Lock()
	delete(s.tabs, tab.targetID)
	s.tabsMu.Unlock()
	s.clearSessionScreenshotHistory(tab.targetID)
	if !s.config.UsesRelayDriver() {
		s.pool.ReleasePage(tab.page, tab.browser)
		return
	}
	if tab.page != nil {
		_ = tab.page.Close()
	}
}

// Start starts the browser service.
func (s *RodService) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return nil
	}

	if err := s.pool.Start(ctx); err != nil {
		return err
	}

	s.started = true
	return nil
}

// Stop stops the browser service and kills all Chromium processes.
// The service can be restarted with Start().
func (s *RodService) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return nil
	}

	// Close all tabs
	s.tabsMu.Lock()
	for _, tab := range s.tabs {
		tab.networkMu.Lock()
		if tab.network != nil && tab.network.cancel != nil {
			tab.network.cancel()
		}
		tab.network = nil
		tab.networkMu.Unlock()
		if tab.page != nil && !s.config.UsesRelayDriver() {
			_ = tab.page.Close()
		}
	}
	s.tabs = make(map[string]*tabInfo)
	s.tabsMu.Unlock()
	s.historyMu.Lock()
	s.screenshotHistory = make(map[string][]SessionScreenshot)
	s.historyMu.Unlock()

	// Close the pool (kills Chromium processes)
	_ = s.pool.Close()

	// Recreate pool so Start() can reinitialize
	pool, err := NewPool(s.config)
	if err != nil {
		s.started = false
		return err
	}
	s.pool = pool
	s.started = false
	return nil
}

// Status returns the browser status.
func (s *RodService) Status(ctx context.Context) (*StatusResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := s.pool.Status()
	status.Running = s.started && status.Running

	s.tabsMu.RLock()
	status.TabCount = len(s.tabs)
	for id, tab := range s.tabs {
		if tab.active {
			status.ActiveTabID = id
			break
		}
	}
	s.tabsMu.RUnlock()

	return status, nil
}

// Tabs returns all open tabs.
func (s *RodService) Tabs(ctx context.Context) ([]*Tab, error) {
	s.pruneExpiredSessionScreenshots()
	if err := s.syncRelayTabs(ctx); err != nil {
		return nil, err
	}

	s.tabsMu.RLock()
	defer s.tabsMu.RUnlock()

	tabs := make([]*Tab, 0, len(s.tabs))
	for id, info := range s.tabs {
		tabs = append(tabs, &Tab{
			TargetID: id,
			URL:      info.url,
			Title:    info.title,
			Active:   info.active,
		})
	}
	return tabs, nil
}

func (s *RodService) syncRelayTabs(ctx context.Context) error {
	if !s.config.UsesRelayDriver() {
		return nil
	}

	browser, err := s.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer s.pool.Release(browser)

	pages, err := browser.Pages()
	if err != nil {
		return err
	}

	next := make(map[string]*tabInfo, len(pages))
	s.tabsMu.Lock()
	defer s.tabsMu.Unlock()

	activeID := ""
	firstTargetID := ""
	detachedTabs := make([]*tabInfo, 0, 1)
	for id, tab := range s.tabs {
		if tab != nil && tab.detached {
			detachedTabs = append(detachedTabs, &tabInfo{
				targetID: id,
				url:      tab.url,
				title:    tab.title,
				active:   false,
				detached: true,
				console:  append([]ConsoleMessage(nil), tab.console...),
			})
			continue
		}
		if tab != nil && tab.active {
			activeID = id
			break
		}
	}

	for _, page := range pages {
		if page == nil {
			continue
		}
		info, err := checkedPageInfo(page)
		if err != nil {
			continue
		}
		targetID := string(page.TargetID)
		if firstTargetID == "" {
			firstTargetID = targetID
		}
		prev := s.tabs[targetID]
		tab := &tabInfo{
			page:     page,
			browser:  browser,
			targetID: targetID,
			url:      info.URL,
			title:    info.Title,
			active:   prev != nil && prev.active,
			console:  make([]ConsoleMessage, 0),
		}
		if prev != nil {
			tab.console = prev.console
		}
		next[targetID] = tab
	}

	if len(next) > 0 {
		if activeID != "" {
			if tab := next[activeID]; tab != nil {
				tab.active = true
			}
		}
		hasActive := false
		for _, tab := range next {
			if tab.active {
				hasActive = true
				break
			}
		}
		if !hasActive {
			if tab := next[firstTargetID]; tab != nil {
				tab.active = true
			}
		}
	}

	for id, prev := range s.tabs {
		if nextTab, ok := next[id]; ok && prev != nil {
			if prev.network != nil && prev.page == nextTab.page {
				nextTab.network = prev.network
			} else if prev.network != nil && prev.network.cancel != nil {
				prev.network.cancel()
			}
			continue
		}
		if prev != nil && prev.network != nil && prev.network.cancel != nil {
			prev.network.cancel()
		}
	}

	for _, tab := range detachedTabs {
		if tab == nil || strings.TrimSpace(tab.targetID) == "" {
			continue
		}
		next[tab.targetID] = tab
	}

	s.tabs = next
	return nil
}

func (s *RodService) ensureTabNetworkCapture(tab *tabInfo, reset bool) {
	if s == nil || tab == nil || tab.page == nil || s.config == nil || !s.config.NetworkObserveEnabled {
		return
	}

	tab.networkMu.Lock()
	defer tab.networkMu.Unlock()

	if tab.network != nil && tab.network.page == tab.page {
		if strings.TrimSpace(tab.targetID) != "" {
			tab.network.targetID = strings.TrimSpace(tab.targetID)
		}
		if reset {
			tab.network.reset()
		}
		return
	}
	if tab.network != nil && tab.network.cancel != nil {
		tab.network.cancel()
	}

	listenerPage, cancel := tab.page.WithCancel()
	if err := (proto.NetworkEnable{}).Call(listenerPage); err != nil {
		cancel()
		return
	}

	state := &networkCaptureState{
		cancel:       cancel,
		page:         tab.page,
		lastActivity: time.Now(),
		pending:      make(map[proto.NetworkRequestID]*observedNetworkBuilder),
		targetID:     strings.TrimSpace(tab.targetID),
		onIdle: func(targetID string, observedAt string) {
			s.mu.RLock()
			hook := s.monitorActivityHook
			s.mu.RUnlock()
			if hook != nil {
				hook(targetID, observedAt)
			}
		},
	}
	tab.network = state

	go listenerPage.EachEvent(
		func(e *proto.NetworkRequestWillBeSent) {
			state.recordRequest(e)
		},
		func(e *proto.NetworkResponseReceived) {
			state.recordResponse(e)
		},
		func(e *proto.NetworkLoadingFinished) {
			state.recordFinished(listenerPage, e)
		},
		func(e *proto.NetworkLoadingFailed) {
			state.recordFailed(e)
		},
	)()
}

func (s *networkCaptureState) reset() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = nil
	s.pending = make(map[proto.NetworkRequestID]*observedNetworkBuilder)
	s.lastActivity = time.Now()
}

func (s *networkCaptureState) recordRequest(e *proto.NetworkRequestWillBeSent) {
	if s == nil || e == nil || e.Request == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending == nil {
		s.pending = make(map[proto.NetworkRequestID]*observedNetworkBuilder)
	}
	s.pending[e.RequestID] = &observedNetworkBuilder{
		Method:       strings.TrimSpace(e.Request.Method),
		URL:          strings.TrimSpace(e.Request.URL),
		ResourceType: strings.TrimSpace(string(e.Type)),
		Initiator:    networkInitiatorLabel(e.Initiator),
		Headers:      sanitizeObservedHeaders(e.Request.Headers),
		StartedAt:    time.Now(),
	}
	s.lastActivity = time.Now()
}

func (s *networkCaptureState) recordResponse(e *proto.NetworkResponseReceived) {
	if s == nil || e == nil || e.Response == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending == nil {
		s.pending = make(map[proto.NetworkRequestID]*observedNetworkBuilder)
	}
	headers := sanitizeObservedHeaders(e.Response.Headers)
	builder := s.pending[e.RequestID]
	if builder == nil {
		builder = &observedNetworkBuilder{
			URL:          strings.TrimSpace(e.Response.URL),
			ResourceType: strings.TrimSpace(string(e.Type)),
			StartedAt:    time.Now(),
		}
		s.pending[e.RequestID] = builder
	}
	builder.Status = e.Response.Status
	builder.ContentType = firstObservedNonEmpty(strings.TrimSpace(e.Response.MIMEType), headers["content-type"])
	if builder.URL == "" {
		builder.URL = strings.TrimSpace(e.Response.URL)
	}
	if builder.ResourceType == "" {
		builder.ResourceType = strings.TrimSpace(string(e.Type))
	}
	builder.Headers = mergeObservedHeaderSets(builder.Headers, headers)
	s.lastActivity = time.Now()
}

func (s *networkCaptureState) recordFinished(page *rod.Page, e *proto.NetworkLoadingFinished) {
	if s == nil || e == nil {
		return
	}

	s.mu.Lock()
	builder := s.pending[e.RequestID]
	delete(s.pending, e.RequestID)
	s.lastActivity = time.Now()
	shouldEmitIdle := builder != nil && len(s.pending) == 0
	targetID := strings.TrimSpace(s.targetID)
	now := timeutil.NowTime()
	if shouldEmitIdle && targetID != "" {
		if s.lastIdleEmit.IsZero() || now.Sub(s.lastIdleEmit) >= 250*time.Millisecond {
			s.lastIdleEmit = now
		} else {
			shouldEmitIdle = false
		}
	}
	idleHook := s.onIdle
	s.mu.Unlock()
	if builder == nil {
		return
	}

	event := ObservedNetworkEvent{
		Method:       builder.Method,
		URL:          builder.URL,
		Status:       builder.Status,
		ContentType:  builder.ContentType,
		ResourceType: builder.ResourceType,
		Initiator:    builder.Initiator,
		DurationMS:   time.Since(builder.StartedAt).Milliseconds(),
		Headers:      cloneObservedHeaderMap(builder.Headers),
	}
	if shouldCaptureObservedBody(event.ContentType, event.ResourceType) && page != nil {
		if body, err := (proto.NetworkGetResponseBody{RequestID: e.RequestID}).Call(page); err == nil && body != nil {
			event.BodySample = sanitizeObservedBodySample(body.Body, body.Base64Encoded, event.ContentType)
		}
	}

	s.mu.Lock()
	s.events = append(s.events, event)
	if len(s.events) > 64 {
		s.events = append([]ObservedNetworkEvent(nil), s.events[len(s.events)-64:]...)
	}
	s.lastActivity = time.Now()
	s.mu.Unlock()

	if shouldEmitIdle && idleHook != nil && targetID != "" {
		idleHook(targetID, now.Format(time.RFC3339))
	}
}

func (s *networkCaptureState) recordFailed(e *proto.NetworkLoadingFailed) {
	if s == nil || e == nil {
		return
	}
	s.mu.Lock()
	delete(s.pending, e.RequestID)
	s.lastActivity = time.Now()
	shouldEmitIdle := len(s.pending) == 0
	targetID := strings.TrimSpace(s.targetID)
	now := timeutil.NowTime()
	if shouldEmitIdle && targetID != "" {
		if s.lastIdleEmit.IsZero() || now.Sub(s.lastIdleEmit) >= 250*time.Millisecond {
			s.lastIdleEmit = now
		} else {
			shouldEmitIdle = false
		}
	}
	idleHook := s.onIdle
	s.mu.Unlock()

	if shouldEmitIdle && idleHook != nil && targetID != "" {
		idleHook(targetID, now.Format(time.RFC3339))
	}
}

func (s *networkCaptureState) snapshot(maxEntries int, clear bool) []ObservedNetworkEvent {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	events := append([]ObservedNetworkEvent(nil), s.events...)
	if maxEntries > 0 && len(events) > maxEntries {
		events = append([]ObservedNetworkEvent(nil), events[len(events)-maxEntries:]...)
	}
	if clear {
		s.events = nil
	}
	return events
}

func (s *networkCaptureState) idleState() (time.Time, int) {
	if s == nil {
		return time.Time{}, 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastActivity, len(s.pending)
}

func networkInitiatorLabel(initiator *proto.NetworkInitiator) string {
	if initiator == nil {
		return ""
	}
	return strings.TrimSpace(string(initiator.Type))
}

func sanitizeObservedHeaders(headers proto.NetworkHeaders) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	out := make(map[string]string)
	for key, value := range headers {
		lower := strings.ToLower(strings.TrimSpace(key))
		switch lower {
		case "accept", "content-type", "location", "x-requested-with", "cf-ray", "cf-cache-status", "server", "via":
			if text := sanitizeObservedHeaderValue(value); text != "" {
				out[lower] = text
			}
		case "authorization", "cookie", "set-cookie":
			out[lower] = "<redacted>"
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func sanitizeObservedHeaderValue(value interface{}) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	text = strings.Trim(text, "\"")
	if text == "" || text == "<nil>" {
		return ""
	}
	return truncateObservedString(text, 160)
}

func mergeObservedHeaderSets(base, extra map[string]string) map[string]string {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}
	out := cloneObservedHeaderMap(base)
	if out == nil {
		out = make(map[string]string, len(extra))
	}
	for key, value := range extra {
		out[key] = value
	}
	return out
}

func cloneObservedHeaderMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func shouldCaptureObservedBody(contentType, resourceType string) bool {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	resourceType = strings.ToLower(strings.TrimSpace(resourceType))
	if strings.Contains(contentType, "json") || strings.Contains(contentType, "javascript") || strings.Contains(contentType, "text/") {
		return true
	}
	switch resourceType {
	case "xhr", "fetch", "document":
		return true
	default:
		return false
	}
}

func sanitizeObservedBodySample(raw string, base64Encoded bool, contentType string) string {
	data := []byte(raw)
	if base64Encoded {
		decoded, err := base64.StdEncoding.DecodeString(raw)
		if err == nil {
			data = decoded
		}
	}
	if len(data) == 0 {
		return ""
	}
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if strings.Contains(contentType, "json") && !json.Valid(data) {
		return ""
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return ""
	}
	text = strings.NewReplacer("\r", " ", "\n", " ", "\t", " ").Replace(text)
	return truncateObservedString(strings.Join(strings.Fields(text), " "), browserNetworkBodySampleLimit)
}

func truncateObservedString(text string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit]) + "..."
}

func firstObservedNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func openedTabTargetID(page *rod.Page) string {
	if page != nil {
		if targetID := strings.TrimSpace(string(page.TargetID)); targetID != "" {
			return targetID
		}
	}
	return fmt.Sprintf("tab-%d", timeutil.NowNano())
}

// OpenTab opens a new tab with the given URL.
func (s *RodService) OpenTab(ctx context.Context, url string) (*Tab, error) {
	normalizedURL, err := s.security.NormalizeAndCheckURL(url)
	if err != nil {
		return nil, err
	}
	return s.openTabNormalized(ctx, normalizedURL, true)
}

func (s *RodService) openTabNormalized(ctx context.Context, normalizedURL string, allowRetry bool) (*Tab, error) {
	page, browser, err := s.pool.NewPage(ctx)
	if err != nil {
		return nil, err
	}
	tab := &tabInfo{
		page:    page,
		browser: browser,
		console: make([]ConsoleMessage, 0),
	}
	s.ensureTabNetworkCapture(tab, true)

	// Navigate to URL
	timeout := GetTimeout(0, s.config)
	err = page.Timeout(timeout).Navigate(normalizedURL)
	if err != nil {
		s.pool.ReleasePage(page, browser)
		if allowRetry && isConnectionClosed(err) {
			return s.openTabNormalized(ctx, normalizedURL, false)
		}
		return nil, err
	}

	// Wait for page to load
	err = waitPageLoad(page, timeout)
	if err != nil {
		s.pool.ReleasePage(page, browser)
		if allowRetry && isConnectionClosed(err) {
			return s.openTabNormalized(ctx, normalizedURL, false)
		}
		return nil, err
	}

	// Get page info
	info, err := checkedPageInfo(page)
	if err != nil {
		s.pool.ReleasePage(page, browser)
		if allowRetry && isConnectionClosed(err) {
			return s.openTabNormalized(ctx, normalizedURL, false)
		}
		return nil, err
	}

	targetID := openedTabTargetID(page)

	s.tabsMu.Lock()
	// Deactivate other tabs
	for _, t := range s.tabs {
		t.active = false
	}
	tab.targetID = targetID
	tab.url = info.URL
	tab.title = info.Title
	tab.active = true
	s.tabs[targetID] = tab
	s.tabsMu.Unlock()

	return &Tab{
		TargetID: targetID,
		URL:      info.URL,
		Title:    info.Title,
		Active:   true,
	}, nil
}

// FocusTab focuses a tab by target ID.
func (s *RodService) FocusTab(ctx context.Context, targetID string) error {
	s.tabsMu.Lock()
	defer s.tabsMu.Unlock()

	tab, ok := s.tabs[targetID]
	if !ok {
		return ErrTabNotFound
	}

	// Deactivate other tabs
	for _, t := range s.tabs {
		t.active = false
	}
	tab.active = true

	return nil
}

// CloseTab closes a tab by target ID.
func (s *RodService) CloseTab(ctx context.Context, targetID string) error {
	s.tabsMu.Lock()
	defer s.tabsMu.Unlock()

	tab, ok := s.tabs[targetID]
	if !ok {
		return ErrTabNotFound
	}

	if tab.page != nil {
		// Ignore close errors — the connection may already be dead.
		_ = tab.page.Close()
	}
	tab.networkMu.Lock()
	if tab.network != nil && tab.network.cancel != nil {
		tab.network.cancel()
	}
	tab.network = nil
	tab.networkMu.Unlock()
	if tab.browser != nil {
		s.pool.Release(tab.browser)
	}
	delete(s.tabs, targetID)
	s.clearSessionScreenshotHistory(targetID)

	return nil
}

func (s *RodService) rememberSessionScreenshot(tab *tabInfo, data string, scope string) {
	if s == nil || tab == nil || strings.TrimSpace(tab.targetID) == "" || strings.TrimSpace(data) == "" {
		return
	}

	s.pruneExpiredSessionScreenshots()

	entry := SessionScreenshot{
		Data:       data,
		URL:        strings.TrimSpace(tab.url),
		Title:      strings.TrimSpace(tab.title),
		CapturedAt: timeutil.NowTime().Format(time.RFC3339),
		Scope:      strings.TrimSpace(scope),
	}

	s.mu.RLock()
	hook := s.monitorFrameHook
	s.mu.RUnlock()
	targetID := tab.targetID
	capturedAt := entry.CapturedAt

	s.historyMu.Lock()

	frames := s.screenshotHistory[targetID]
	if count := len(frames); count > 0 && frames[count-1].Data == entry.Data {
		frames[count-1].CapturedAt = entry.CapturedAt
		if entry.URL != "" {
			frames[count-1].URL = entry.URL
		}
		if entry.Title != "" {
			frames[count-1].Title = entry.Title
		}
		if entry.Scope != "" {
			frames[count-1].Scope = entry.Scope
		}
		s.screenshotHistory[targetID] = frames
		s.historyMu.Unlock()
		return
	}

	frames = append(frames, entry)
	if len(frames) > maxSessionScreenshotHistory {
		frames = append([]SessionScreenshot(nil), frames[len(frames)-maxSessionScreenshotHistory:]...)
	}
	s.screenshotHistory[targetID] = frames
	s.historyMu.Unlock()
	if hook != nil {
		hook(targetID, capturedAt)
	}
}

func (s *RodService) ensureDetachedMonitorTab(url string, title string) *tabInfo {
	if s == nil {
		return nil
	}

	s.tabsMu.Lock()
	defer s.tabsMu.Unlock()

	tab := s.tabs[detachedMonitorTargetID]
	if tab == nil {
		tab = &tabInfo{
			targetID: detachedMonitorTargetID,
			active:   false,
			detached: true,
			console:  make([]ConsoleMessage, 0),
		}
		s.tabs[detachedMonitorTargetID] = tab
	}
	tab.url = strings.TrimSpace(url)
	tab.title = strings.TrimSpace(title)
	tab.active = false
	tab.detached = true
	return tab
}

func (s *RodService) rememberDetachedMonitorScreenshot(url string, title string, data string, scope string) {
	if s == nil || strings.TrimSpace(data) == "" {
		return
	}
	tab := s.ensureDetachedMonitorTab(url, title)
	s.rememberSessionScreenshot(tab, data, scope)
}

func (s *RodService) captureDetachedMonitorPageScreenshot(page *rod.Page, scope string) {
	if s == nil || page == nil {
		return
	}

	data, err := page.Screenshot(false, nil)
	if err != nil || len(data) == 0 {
		return
	}

	info := snapshotPageInfo(page)
	s.rememberDetachedMonitorScreenshot(
		info.URL,
		info.Title,
		base64.StdEncoding.EncodeToString(data),
		scope,
	)
}

func (s *RodService) sessionScreenshotRetention() time.Duration {
	if s == nil || s.config == nil {
		return 0
	}
	return s.config.SessionScreenshotRetention
}

func (s *RodService) pruneExpiredSessionScreenshots() {
	if s == nil {
		return
	}
	retention := s.sessionScreenshotRetention()
	if retention <= 0 {
		return
	}

	cutoff := timeutil.NowTime().Add(-retention)
	staleDetachedIDs := make([]string, 0)

	s.historyMu.Lock()
	for targetID, frames := range s.screenshotHistory {
		if len(frames) == 0 {
			delete(s.screenshotHistory, targetID)
			staleDetachedIDs = append(staleDetachedIDs, targetID)
			continue
		}

		kept := make([]SessionScreenshot, 0, len(frames))
		for _, frame := range frames {
			capturedAt := strings.TrimSpace(frame.CapturedAt)
			if capturedAt == "" {
				kept = append(kept, frame)
				continue
			}

			ts, err := time.Parse(time.RFC3339, capturedAt)
			if err != nil || !ts.Before(cutoff) {
				kept = append(kept, frame)
			}
		}

		if len(kept) == 0 {
			delete(s.screenshotHistory, targetID)
			staleDetachedIDs = append(staleDetachedIDs, targetID)
			continue
		}

		s.screenshotHistory[targetID] = kept
	}
	s.historyMu.Unlock()

	if len(staleDetachedIDs) == 0 {
		return
	}

	s.tabsMu.Lock()
	for _, targetID := range staleDetachedIDs {
		tab := s.tabs[targetID]
		if tab != nil && tab.detached {
			delete(s.tabs, targetID)
		}
	}
	s.tabsMu.Unlock()
}

// CleanupExpiredMonitorFrames removes expired monitor frame history using the
// configured retention window.
func (s *RodService) CleanupExpiredMonitorFrames() {
	s.pruneExpiredSessionScreenshots()
}

// SetSessionScreenshotRetention updates the monitor frame retention window and
// immediately prunes any expired history.
func (s *RodService) SetSessionScreenshotRetention(retention time.Duration) {
	if s == nil {
		return
	}
	if retention < 0 {
		retention = 0
	}
	if s.config != nil {
		s.config.SessionScreenshotRetention = retention
	}
	s.pruneExpiredSessionScreenshots()
}

// SetMonitorFrameListener sets an optional hook called when a new screenshot frame
// is captured or refreshed for a session. The hook must be fast and non-blocking.
func (s *RodService) SetMonitorFrameListener(listener func(targetID string, capturedAt string)) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.monitorFrameHook = listener
	s.mu.Unlock()
}

// SetMonitorActivityListener sets an optional hook called when a session reports
// network activity reaching an idle point. This can be used to trigger fast UI refreshes
// without pushing screenshot binaries over SSE.
func (s *RodService) SetMonitorActivityListener(listener func(targetID string, observedAt string)) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.monitorActivityHook = listener
	s.mu.Unlock()
}

func (s *RodService) clearSessionScreenshotHistory(targetID string) {
	if s == nil || strings.TrimSpace(targetID) == "" {
		return
	}
	s.historyMu.Lock()
	delete(s.screenshotHistory, targetID)
	s.historyMu.Unlock()
}

// SessionScreenshotHistory returns newest-first screenshots captured for a tab.
func (s *RodService) SessionScreenshotHistory(targetID string) []SessionScreenshot {
	if s == nil || strings.TrimSpace(targetID) == "" {
		return nil
	}

	s.pruneExpiredSessionScreenshots()

	s.historyMu.RLock()
	frames := s.screenshotHistory[targetID]
	s.historyMu.RUnlock()
	if len(frames) == 0 {
		return nil
	}

	reversed := make([]SessionScreenshot, 0, len(frames))
	for idx := len(frames) - 1; idx >= 0; idx-- {
		reversed = append(reversed, frames[idx])
	}
	return reversed
}

// Navigate navigates to a URL in the current or specified tab.
func (s *RodService) Navigate(ctx context.Context, req *NavigateRequest) (*NavigateResponse, error) {
	normalizedURL, err := s.security.NormalizeAndCheckURL(req.URL)
	if err != nil {
		return nil, err
	}

	tab, err := s.getTab(ctx, req.TargetID)
	if err != nil && !(errors.Is(err, ErrTabNotFound) && req.TargetID == "") {
		return nil, err
	}
	if errors.Is(err, ErrTabNotFound) && req.TargetID == "" {
		tab = nil
	}

	reopenTab := func() (*NavigateResponse, error) {
		s.removeTab(tab)
		newTab, retryErr := s.openTabNormalized(ctx, normalizedURL, true)
		if retryErr != nil {
			return nil, retryErr
		}
		return &NavigateResponse{
			URL:      newTab.URL,
			Title:    newTab.Title,
			TargetID: newTab.TargetID,
		}, nil
	}

	if tab == nil {
		// Open new tab
		newTab, err := s.openTabNormalized(ctx, normalizedURL, true)
		if err != nil {
			return nil, err
		}
		return &NavigateResponse{
			URL:      newTab.URL,
			Title:    newTab.Title,
			TargetID: newTab.TargetID,
		}, nil
	}

	s.ensureTabNetworkCapture(tab, true)

	timeout := GetTimeout(req.Timeout, s.config)
	err = tab.page.Timeout(timeout).Navigate(normalizedURL)
	if err != nil {
		// Connection may be dead (Chrome crashed, WebSocket closed).
		// Clean up the stale tab and retry with a fresh one.
		if isConnectionClosed(err) {
			return reopenTab()
		}
		return nil, err
	}

	// Wait based on WaitUntil
	switch req.WaitUntil {
	case "networkidle":
		err = waitPageIdle(tab.page, timeout)
	case "domcontentloaded":
		err = waitPageDOMStable(tab.page, timeout, 0.5)
	default: // "load" or empty
		err = waitPageLoad(tab.page, timeout)
	}
	if err != nil {
		if isConnectionClosed(err) {
			return reopenTab()
		}
		return nil, err
	}

	info, err := checkedPageInfo(tab.page)
	if err != nil {
		if isConnectionClosed(err) {
			return reopenTab()
		}
		return nil, err
	}

	s.tabsMu.Lock()
	tab.url = info.URL
	tab.title = info.Title
	s.tabsMu.Unlock()

	return &NavigateResponse{
		URL:      info.URL,
		Title:    info.Title,
		TargetID: tab.targetID,
	}, nil
}

// Screenshot captures a screenshot of a page.
func (s *RodService) Screenshot(ctx context.Context, req *ScreenshotRequest) (*ScreenshotResponse, error) {
	normalizedURL, err := s.security.NormalizeAndCheckURL(req.URL)
	if err != nil {
		return nil, err
	}

	page, browser, err := s.pool.NewPage(ctx)
	if err != nil {
		return nil, err
	}
	defer s.pool.ReleasePage(page, browser)

	// Set viewport if specified
	if req.Width > 0 && req.Height > 0 {
		err = page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
			Width:  req.Width,
			Height: req.Height,
		})
		if err != nil {
			return nil, err
		}
	}

	timeout := GetTimeout(req.Timeout, s.config)
	err = page.Timeout(timeout).Navigate(normalizedURL)
	if err != nil {
		return nil, err
	}

	err = waitDetachedPageReady(page, timeout, req.WaitForSelector, req.SkipWaitLoad, req.WaitFor)
	if err != nil {
		return nil, err
	}

	var data []byte
	format := req.Format
	if format == "" {
		format = FormatPNG
	}

	quality := req.Quality
	if quality <= 0 {
		quality = 90
	}

	if req.Selector != nil && *req.Selector != "" {
		// Screenshot specific element
		el, err := page.Element(*req.Selector)
		if err != nil {
			return nil, ErrElementNotFound
		}
		data, err = el.Screenshot(format.toProto(), quality)
		if err != nil {
			return nil, err
		}
	} else if req.FullPage {
		// Full page screenshot
		data, err = page.Screenshot(req.FullPage, &proto.PageCaptureScreenshot{
			Format:  format.toProto(),
			Quality: &quality,
		})
		if err != nil {
			return nil, err
		}
	} else {
		// Viewport screenshot
		data, err = page.Screenshot(false, &proto.PageCaptureScreenshot{
			Format:  format.toProto(),
			Quality: &quality,
		})
		if err != nil {
			return nil, err
		}
	}

	info := snapshotPageInfo(page)
	encoded := base64.StdEncoding.EncodeToString(data)
	scope := "viewport"
	if req.FullPage {
		scope = "full_page"
	} else if req.Selector != nil && strings.TrimSpace(*req.Selector) != "" {
		scope = "element"
	}
	s.rememberDetachedMonitorScreenshot(info.URL, info.Title, encoded, scope)

	return &ScreenshotResponse{
		Data:   encoded,
		Format: format,
		URL:    info.URL,
		Title:  info.Title,
	}, nil
}

// toProto converts ScreenshotFormat to proto format.
func (f ScreenshotFormat) toProto() proto.PageCaptureScreenshotFormat {
	switch f {
	case FormatJPEG:
		return proto.PageCaptureScreenshotFormatJpeg
	case FormatWebP:
		return proto.PageCaptureScreenshotFormatWebp
	default:
		return proto.PageCaptureScreenshotFormatPng
	}
}

// PDF generates a PDF from a page.
func (s *RodService) PDF(ctx context.Context, req *PDFRequest) (*PDFResponse, error) {
	normalizedURL, err := s.security.NormalizeAndCheckURL(req.URL)
	if err != nil {
		return nil, err
	}

	page, browser, err := s.pool.NewPage(ctx)
	if err != nil {
		return nil, err
	}
	defer s.pool.ReleasePage(page, browser)

	timeout := GetTimeout(req.Timeout, s.config)
	err = page.Timeout(timeout).Navigate(normalizedURL)
	if err != nil {
		return nil, err
	}

	err = waitPageLoad(page, timeout)
	if err != nil {
		return nil, err
	}

	// Additional wait time
	if req.WaitFor > 0 {
		time.Sleep(time.Duration(req.WaitFor) * time.Millisecond)
	}

	// Build PDF options
	pdfReq := &proto.PagePrintToPDF{
		PrintBackground: req.PrintBackground,
		Landscape:       req.Landscape,
	}

	if req.Scale > 0 {
		pdfReq.Scale = &req.Scale
	}

	// Set paper size based on format
	switch req.Format {
	case PDFFormatLetter:
		w, h := 8.5, 11.0
		pdfReq.PaperWidth = &w
		pdfReq.PaperHeight = &h
	case PDFFormatLegal:
		w, h := 8.5, 14.0
		pdfReq.PaperWidth = &w
		pdfReq.PaperHeight = &h
	default: // A4
		w, h := 8.27, 11.69
		pdfReq.PaperWidth = &w
		pdfReq.PaperHeight = &h
	}

	// Set margins
	if req.MarginTop > 0 {
		pdfReq.MarginTop = &req.MarginTop
	}
	if req.MarginBottom > 0 {
		pdfReq.MarginBottom = &req.MarginBottom
	}
	if req.MarginLeft > 0 {
		pdfReq.MarginLeft = &req.MarginLeft
	}
	if req.MarginRight > 0 {
		pdfReq.MarginRight = &req.MarginRight
	}

	reader, err := page.PDF(pdfReq)
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	info := snapshotPageInfo(page)

	return &PDFResponse{
		Data:  base64.StdEncoding.EncodeToString(data),
		URL:   info.URL,
		Title: info.Title,
	}, nil
}

// Snapshot returns a structured snapshot of the page.
func (s *RodService) Snapshot(ctx context.Context, req *SnapshotRequest) (*SnapshotResponse, error) {
	tab, err := s.getTab(ctx, req.TargetID)
	if err != nil {
		return nil, err
	}

	// Get page content
	var snapshot string
	var elementCount int

	// Default to HTML format
	html, err := tab.page.HTML()
	if err != nil {
		return nil, err
	}
	snapshot = html
	if req.MaxChars > 0 && len(snapshot) > req.MaxChars {
		snapshot = snapshot[:req.MaxChars]
	}

	info := snapshotPageInfo(tab.page)

	return &SnapshotResponse{
		Snapshot:     snapshot,
		Format:       req.Format,
		URL:          info.URL,
		Title:        info.Title,
		TargetID:     tab.targetID,
		ElementCount: elementCount,
	}, nil
}

// Scrape extracts data from a page.
func (s *RodService) Scrape(ctx context.Context, req *ScrapeRequest) (*ScrapeResponse, error) {
	normalizedURL, err := s.security.NormalizeAndCheckURL(req.URL)
	if err != nil {
		return nil, err
	}

	page, browser, err := s.pool.NewPage(ctx)
	if err != nil {
		return nil, err
	}
	defer s.pool.ReleasePage(page, browser)

	timeout := GetTimeout(req.Timeout, s.config)
	err = page.Timeout(timeout).Navigate(normalizedURL)
	if err != nil {
		return nil, err
	}

	err = waitDetachedPageReady(page, timeout, req.WaitForSelector, req.SkipWaitLoad, req.WaitFor)
	if err != nil {
		return nil, err
	}

	// Extract data
	data := make(map[string]interface{})
	for name, config := range req.Selectors {
		if config.Multiple {
			elements, err := page.Elements(config.Selector)
			if err != nil {
				data[name] = []string{}
				continue
			}
			if config.MaxMatches > 0 && len(elements) > config.MaxMatches {
				elements = elements[:config.MaxMatches]
			}
			values := make([]string, 0, len(elements))
			for _, el := range elements {
				var val string
				if config.Attribute != "" {
					attr, _ := el.Attribute(config.Attribute)
					if attr != nil {
						val = *attr
					}
				} else {
					val, _ = el.Text()
				}
				values = append(values, val)
			}
			data[name] = values
		} else {
			el, err := page.Element(config.Selector)
			if err != nil {
				data[name] = ""
				continue
			}
			var val string
			if config.Attribute != "" {
				attr, _ := el.Attribute(config.Attribute)
				if attr != nil {
					val = *attr
				}
			} else {
				val, _ = el.Text()
			}
			data[name] = val
		}
	}

	info := snapshotPageInfo(page)

	return &ScrapeResponse{
		Data:  data,
		URL:   info.URL,
		Title: info.Title,
	}, nil
}

// Act performs an action on the page.
func (s *RodService) Act(ctx context.Context, req *ActRequest) (*ActResponse, error) {
	tab, err := s.getTab(ctx, req.TargetID)
	if err != nil {
		return nil, err
	}

	timeout := GetTimeout(req.Timeout, s.config)
	page := tab.page.Timeout(timeout)

	switch req.Kind {
	case "click":
		el, err := page.Element(req.Selector)
		if err != nil {
			return nil, ErrElementNotFound
		}
		if req.Double {
			err = el.Click(proto.InputMouseButtonLeft, 2)
		} else {
			err = el.Click(proto.InputMouseButtonLeft, 1)
		}
		if err != nil {
			return nil, err
		}

	case "type":
		el, err := page.Element(req.Selector)
		if err != nil {
			return nil, ErrElementNotFound
		}
		text := req.Text
		if text == "" {
			text = req.Value
		}
		err = el.Input(text)
		if err != nil {
			return nil, err
		}
		if req.Submit {
			err = page.Keyboard.Press(input.Enter)
			if err != nil {
				return nil, err
			}
		}

	case "select":
		el, err := page.Element(req.Selector)
		if err != nil {
			return nil, ErrElementNotFound
		}
		err = el.Select(req.Options, true, rod.SelectorTypeText)
		if err != nil {
			return nil, err
		}

	case "scroll":
		if req.Selector != "" {
			el, err := page.Element(req.Selector)
			if err != nil {
				return nil, ErrElementNotFound
			}
			err = el.ScrollIntoView()
			if err != nil {
				return nil, err
			}
		} else {
			_, err := page.Eval(fmt.Sprintf("window.scrollBy(%d, %d)", req.X, req.Y))
			if err != nil {
				return nil, err
			}
		}

	case "hover":
		el, err := page.Element(req.Selector)
		if err != nil {
			return nil, ErrElementNotFound
		}
		err = el.Hover()
		if err != nil {
			return nil, err
		}

	case "press":
		err := page.Keyboard.Type(input.Key([]rune(req.Key)[0]))
		if err != nil {
			return nil, err
		}

	case "upload":
		el, err := page.Element(req.Selector)
		if err != nil {
			return nil, ErrElementNotFound
		}
		files := append([]string(nil), req.Files...)
		if len(files) == 0 && req.Value != "" {
			files = []string{req.Value}
		}
		if len(files) == 0 {
			return nil, ErrInvalidAction
		}
		if err := el.SetFiles(files); err != nil {
			return nil, err
		}

	case "wait":
		if req.Duration > 0 {
			time.Sleep(time.Duration(req.Duration) * time.Millisecond)
		}

	case "close":
		return nil, s.CloseTab(ctx, tab.targetID)

	default:
		return nil, ErrInvalidAction
	}

	return &ActResponse{
		Success: true,
	}, nil
}

// Automate runs a multi-step automation task.
func (s *RodService) Automate(ctx context.Context, req *AutomateRequest) (*AutomateResponse, error) {
	normalizedURL, err := s.security.NormalizeAndCheckURL(req.URL)
	if err != nil {
		return nil, err
	}

	page, browser, err := s.pool.NewPage(ctx)
	if err != nil {
		return nil, err
	}
	defer s.pool.ReleasePage(page, browser)

	timeout := GetTimeout(req.Timeout, s.config)
	err = page.Timeout(timeout).Navigate(normalizedURL)
	if err != nil {
		return nil, err
	}

	err = waitPageLoad(page, timeout)
	if err != nil {
		return nil, err
	}

	s.captureDetachedMonitorPageScreenshot(page, "automate")

	results := make([]StepResult, 0, len(req.Steps))
	success := true

	for i, step := range req.Steps {
		start := time.Now()
		result := StepResult{
			Step:   i,
			Action: step.Action,
		}

		stepErr := s.executeStep(ctx, page, &step)
		if stepErr == nil && step.WaitFor > 0 {
			time.Sleep(time.Duration(step.WaitFor) * time.Millisecond)
		}
		result.Duration = time.Since(start).Milliseconds()

		if stepErr != nil {
			result.Success = false
			result.Error = stepErr.Error()
			s.captureDetachedMonitorPageScreenshot(page, "automate")
			if !step.Optional {
				success = false
				results = append(results, result)
				break
			}
		} else {
			result.Success = true
			s.captureDetachedMonitorPageScreenshot(page, "automate")
		}

		results = append(results, result)
	}

	info := snapshotPageInfo(page)
	s.captureDetachedMonitorPageScreenshot(page, "automate")

	return &AutomateResponse{
		Success:        success,
		StepsCompleted: len(results),
		Results:        results,
		FinalURL:       info.URL,
		FinalTitle:     info.Title,
	}, nil
}

// executeStep executes a single automation step.
func (s *RodService) executeStep(ctx context.Context, page *rod.Page, step *AutomationStep) error {
	switch step.Action {
	case ActionClick:
		el, err := page.Element(step.Selector)
		if err != nil {
			return ErrElementNotFound
		}
		return el.Click(proto.InputMouseButtonLeft, 1)

	case ActionTypeText:
		el, err := page.Element(step.Selector)
		if err != nil {
			return ErrElementNotFound
		}
		return el.Input(step.Value)

	case ActionSelect:
		el, err := page.Element(step.Selector)
		if err != nil {
			return ErrElementNotFound
		}
		return el.Select([]string{step.Value}, true, rod.SelectorTypeText)

	case ActionWait:
		return nil

	case ActionWaitFor:
		_, err := page.Element(step.Selector)
		return err

	case ActionScroll:
		if step.Selector != "" {
			el, err := page.Element(step.Selector)
			if err != nil {
				return ErrElementNotFound
			}
			return el.ScrollIntoView()
		}
		return nil

	case ActionNavigate:
		normalizedURL, err := s.security.NormalizeAndCheckURL(step.Value)
		if err != nil {
			return err
		}
		return page.Navigate(normalizedURL)

	case ActionEval:
		_, err := page.Eval(step.Value)
		return err

	case ActionHover:
		el, err := page.Element(step.Selector)
		if err != nil {
			return ErrElementNotFound
		}
		return el.Hover()

	case ActionPress:
		if len(step.Value) > 0 {
			return page.Keyboard.Type(input.Key([]rune(step.Value)[0]))
		}
		return nil

	case ActionUpload:
		el, err := page.Element(step.Selector)
		if err != nil {
			return ErrElementNotFound
		}
		if step.Value == "" {
			return ErrInvalidAction
		}
		return el.SetFiles([]string{step.Value})

	default:
		return ErrInvalidAction
	}
}

// Console returns console messages from the page.
func (s *RodService) Console(ctx context.Context, req *ConsoleRequest) (*ConsoleResponse, error) {
	tab, err := s.getTab(ctx, req.TargetID)
	if err != nil {
		return nil, err
	}

	messages := make([]ConsoleMessage, 0)
	for _, msg := range tab.console {
		if req.Level == "" || msg.Level == req.Level {
			messages = append(messages, msg)
		}
	}

	if req.Clear {
		s.tabsMu.Lock()
		tab.console = make([]ConsoleMessage, 0)
		s.tabsMu.Unlock()
	}

	return &ConsoleResponse{
		Messages: messages,
	}, nil
}

// getTab returns the tab for the given targetID, or the active tab if targetID is empty.
func (s *RodService) getTab(ctx context.Context, targetID string) (*tabInfo, error) {
	if err := s.syncRelayTabs(ctx); err != nil {
		return nil, err
	}

	s.tabsMu.RLock()
	defer s.tabsMu.RUnlock()
	if targetID != "" {
		tab := s.tabs[targetID]
		if tab == nil || tab.page == nil {
			return nil, ErrTabNotFound
		}
		return tab, nil
	}
	for _, t := range s.tabs {
		if t.active {
			if t.page == nil {
				return nil, ErrTabNotFound
			}
			return t, nil
		}
	}
	return nil, ErrTabNotFound
}

// ElementExists reports whether the given selector matches in the current tab.
func (s *RodService) ElementExists(ctx context.Context, targetID, selector string) (bool, error) {
	tab, err := s.getTab(ctx, targetID)
	if err != nil {
		return false, err
	}
	timeout := GetTimeout(0, s.config)
	_, err = safeRodPageResult("element exists lookup", func() (*rod.Element, error) {
		return tab.page.Timeout(timeout).Element(selector)
	})
	if err != nil {
		return false, nil
	}
	return true, nil
}

// ExtractFirstFromTab returns the first matching attribute or text content from the current tab.
func (s *RodService) ExtractFirstFromTab(ctx context.Context, targetID, selector, attribute string) (string, error) {
	tab, err := s.getTab(ctx, targetID)
	if err != nil {
		return "", err
	}
	timeout := GetTimeout(0, s.config)
	el, err := safeRodPageResult("extract element lookup", func() (*rod.Element, error) {
		return tab.page.Timeout(timeout).Element(selector)
	})
	if err != nil {
		return "", err
	}
	if attribute != "" {
		attr, err := safeRodPageResult("extract attribute", func() (*string, error) {
			return el.Attribute(attribute)
		})
		if err != nil || attr == nil {
			return "", err
		}
		return *attr, nil
	}
	return safeRodPageResult("extract text", func() (string, error) {
		return el.Text()
	})
}

// PageInfo returns the current URL and title for a tab.
func (s *RodService) PageInfo(ctx context.Context, targetID string) (string, string, error) {
	tab, err := s.getTab(ctx, targetID)
	if err != nil {
		return "", "", err
	}
	info, err := checkedPageInfo(tab.page)
	if err != nil {
		return "", "", err
	}
	return info.URL, info.Title, nil
}

// CookieHeader returns cookies applicable to the target URL from the tab's browser context.
func (s *RodService) CookieHeader(ctx context.Context, targetID string, targetURL string) (string, error) {
	normalizedURL, err := s.security.NormalizeAndCheckURL(targetURL)
	if err != nil {
		return "", err
	}
	tab, err := s.getTab(ctx, targetID)
	if err != nil {
		return "", err
	}
	cookies, err := tab.page.Cookies([]string{normalizedURL})
	if err != nil {
		if isConnectionClosed(err) {
			s.removeTab(tab)
		}
		return "", err
	}
	parts := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		if cookie == nil || strings.TrimSpace(cookie.Name) == "" {
			continue
		}
		parts = append(parts, cookie.Name+"="+cookie.Value)
	}
	return strings.Join(parts, "; "), nil
}

// ObserveNetwork returns recent network activity recorded for a tab.
func (s *RodService) ObserveNetwork(ctx context.Context, targetID string, maxEntries int, clear bool) (*ObservedNetworkResult, error) {
	tab, err := s.getTab(ctx, targetID)
	if err != nil {
		return nil, err
	}
	s.ensureTabNetworkCapture(tab, false)

	tab.networkMu.Lock()
	state := tab.network
	tab.networkMu.Unlock()
	if state == nil {
		return &ObservedNetworkResult{TargetID: tab.targetID}, nil
	}
	return &ObservedNetworkResult{
		TargetID: tab.targetID,
		Events:   state.snapshot(maxEntries, clear),
	}, nil
}

// WaitNetworkIdle waits until the tab has no pending requests and has been idle
// for the requested duration.
func (s *RodService) WaitNetworkIdle(ctx context.Context, targetID string, idleMS int, timeoutMS int) error {
	tab, err := s.getTab(ctx, targetID)
	if err != nil {
		return err
	}
	s.ensureTabNetworkCapture(tab, false)

	tab.networkMu.Lock()
	state := tab.network
	tab.networkMu.Unlock()
	if state == nil {
		return nil
	}

	idle := time.Duration(idleMS) * time.Millisecond
	if idle <= 0 {
		idle = 400 * time.Millisecond
	}
	waitCtx := ctx
	cancel := func() {}
	if timeoutMS > 0 {
		waitCtx, cancel = context.WithTimeout(ctx, time.Duration(timeoutMS)*time.Millisecond)
	}
	defer cancel()

	ticker := time.NewTicker(75 * time.Millisecond)
	defer ticker.Stop()

	for {
		lastActivity, pending := state.idleState()
		if pending == 0 && !lastActivity.IsZero() && time.Since(lastActivity) >= idle {
			return nil
		}
		select {
		case <-waitCtx.Done():
			return waitCtx.Err()
		case <-ticker.C:
		}
	}
}

// AccessibilityTree returns a compact DSL representation of the page's accessibility tree.
// This is much more token-efficient than raw HTML for LLM consumption.
// Each interactive element gets an @ref that can be used in Act() to target it.
func (s *RodService) AccessibilityTree(ctx context.Context, targetID string, maxDepth int) (*AccessibilityTreeResponse, error) {
	tab, err := s.getTab(ctx, targetID)
	if err != nil {
		return nil, err
	}

	depth := maxDepth
	if depth <= 0 {
		depth = 10
	}

	rawResult, err := tab.page.Call(
		ctx,
		string(tab.page.GetSessionID()),
		"Accessibility.getFullAXTree",
		proto.AccessibilityGetFullAXTree{Depth: &depth},
	)
	if err != nil {
		if isConnectionClosed(err) {
			s.removeTab(tab)
			return nil, fmt.Errorf("browser connection lost (tab removed): %w", err)
		}
		return nil, fmt.Errorf("failed to get accessibility tree: %w", err)
	}
	nodes, err := decodeAccessibilityTreeNodes(rawResult)
	if err != nil {
		return nil, fmt.Errorf("failed to decode accessibility tree: %w", err)
	}

	// Build DSL with @ref references
	builder := newAXTreeBuilder(nodes)
	tree := builder.build()

	info := snapshotPageInfo(tab.page)

	return &AccessibilityTreeResponse{
		Tree:     tree,
		URL:      info.URL,
		Title:    info.Title,
		TargetID: tab.targetID,
		RefMap:   builder.refMap,
	}, nil
}

type accessibilityTreeCompatResult struct {
	Nodes []accessibilityTreeCompatNode `json:"nodes"`
}

type accessibilityTreeCompatNode struct {
	NodeID           json.RawMessage                  `json:"nodeId"`
	Ignored          bool                             `json:"ignored"`
	Role             *proto.AccessibilityAXValue      `json:"role,omitempty"`
	ChromeRole       *proto.AccessibilityAXValue      `json:"chromeRole,omitempty"`
	Name             *proto.AccessibilityAXValue      `json:"name,omitempty"`
	Description      *proto.AccessibilityAXValue      `json:"description,omitempty"`
	Value            *proto.AccessibilityAXValue      `json:"value,omitempty"`
	Properties       []*proto.AccessibilityAXProperty `json:"properties,omitempty"`
	ParentID         json.RawMessage                  `json:"parentId,omitempty"`
	ChildIDs         []json.RawMessage                `json:"childIds,omitempty"`
	BackendDOMNodeID proto.DOMBackendNodeID           `json:"backendDOMNodeId,omitempty"`
	FrameID          proto.PageFrameID                `json:"frameId,omitempty"`
}

func decodeAccessibilityTreeNodes(raw []byte) ([]*proto.AccessibilityAXNode, error) {
	var direct proto.AccessibilityGetFullAXTreeResult
	if err := json.Unmarshal(raw, &direct); err == nil {
		return direct.Nodes, nil
	}

	var compat accessibilityTreeCompatResult
	if err := json.Unmarshal(raw, &compat); err != nil {
		return nil, err
	}

	nodes := make([]*proto.AccessibilityAXNode, 0, len(compat.Nodes))
	for _, node := range compat.Nodes {
		nodeID, err := normalizeAccessibilityAXNodeID(node.NodeID)
		if err != nil {
			return nil, err
		}
		parentID, err := normalizeAccessibilityAXNodeID(node.ParentID)
		if err != nil {
			return nil, err
		}
		childIDs, err := normalizeAccessibilityAXNodeIDs(node.ChildIDs)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, &proto.AccessibilityAXNode{
			NodeID:           nodeID,
			Ignored:          node.Ignored,
			Role:             node.Role,
			ChromeRole:       node.ChromeRole,
			Name:             node.Name,
			Description:      node.Description,
			Value:            node.Value,
			Properties:       node.Properties,
			ParentID:         parentID,
			ChildIDs:         childIDs,
			BackendDOMNodeID: node.BackendDOMNodeID,
			FrameID:          node.FrameID,
		})
	}
	return nodes, nil
}

func normalizeAccessibilityAXNodeIDs(rawValues []json.RawMessage) ([]proto.AccessibilityAXNodeID, error) {
	if len(rawValues) == 0 {
		return nil, nil
	}
	ids := make([]proto.AccessibilityAXNodeID, 0, len(rawValues))
	for _, rawValue := range rawValues {
		id, err := normalizeAccessibilityAXNodeID(rawValue)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func normalizeAccessibilityAXNodeID(raw json.RawMessage) (proto.AccessibilityAXNodeID, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return "", nil
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return proto.AccessibilityAXNodeID(text), nil
	}

	var number json.Number
	if err := json.Unmarshal(raw, &number); err == nil {
		return proto.AccessibilityAXNodeID(number.String()), nil
	}

	return "", fmt.Errorf("unsupported accessibility node id %s", trimmed)
}

// axTreeBuilder builds a compact DSL from accessibility tree nodes.
//
// Output format (each interactive element gets an @ref):
//
//	[document] "My Page"
//	  [nav] "Main Nav"
//	    @1 [link] "Home" href=/
//	    @2 [link] "About" href=/about
//	  [main]
//	    [heading:1] "Welcome"
//	    @3 [textbox] "Search..." focused
//	    @4 [button] "Submit"
//
// The @ref numbers map to backend DOM node IDs via RefMap,
// so the LLM can say "click @4" and we resolve it to the actual element.
type axTreeBuilder struct {
	nodeMap  map[string]*proto.AccessibilityAXNode
	childMap map[string][]string
	rootID   string
	refMap   map[int]int // @ref → backend DOM node ID
	nextRef  int
	buf      []byte
}

func newAXTreeBuilder(nodes []*proto.AccessibilityAXNode) *axTreeBuilder {
	b := &axTreeBuilder{
		nodeMap:  make(map[string]*proto.AccessibilityAXNode, len(nodes)),
		childMap: make(map[string][]string, len(nodes)),
		refMap:   make(map[int]int),
		nextRef:  1,
	}

	for _, node := range nodes {
		id := string(node.NodeID)
		b.nodeMap[id] = node
		if node.ParentID == "" {
			b.rootID = id
		} else {
			pid := string(node.ParentID)
			b.childMap[pid] = append(b.childMap[pid], id)
		}
	}

	if b.rootID == "" && len(nodes) > 0 {
		b.rootID = string(nodes[0].NodeID)
	}

	return b
}

// interactiveSelector is the CSS selector for interactive elements.
// Used by CountInteractiveElements, InteractiveElements, and ActByInteractiveRef.
const interactiveSelector = `a, button, input, select, textarea, [role="button"], [role="link"], [role="tab"], [role="menuitem"], [onclick], [contenteditable="true"]`

// interactiveRoles are roles that get @ref assignments for LLM targeting.
var interactiveRoles = map[string]bool{
	"link":             true,
	"button":           true,
	"textbox":          true,
	"searchbox":        true,
	"combobox":         true,
	"checkbox":         true,
	"radio":            true,
	"switch":           true,
	"slider":           true,
	"spinbutton":       true,
	"tab":              true,
	"menuitem":         true,
	"menuitemcheckbox": true,
	"menuitemradio":    true,
	"option":           true,
	"treeitem":         true,
}

// skipRoles are roles that add noise without useful info for LLM.
var skipRoles = map[string]bool{
	"none":          true,
	"generic":       true,
	"InlineTextBox": true,
	"LineBreak":     true,
}

func (b *axTreeBuilder) build() string {
	b.appendNode(b.rootID, 0)

	// Cap at ~8K chars to stay token-friendly
	if len(b.buf) > 8192 {
		b.buf = b.buf[:8192]
		b.buf = append(b.buf, "\n... (truncated)"...)
	}

	return string(b.buf)
}

// axValueStr converts a CDP AXValue (interface{}) to string without fmt.Sprintf.
func axValueStr(v interface{}) string {
	switch s := v.(type) {
	case string:
		return s
	case float64:
		return strconv.FormatFloat(s, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(s)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func (b *axTreeBuilder) appendNode(id string, depth int) {
	node, ok := b.nodeMap[id]
	if !ok {
		return
	}

	// Skip ignored nodes but still process children
	if node.Ignored {
		for _, childID := range b.childMap[id] {
			b.appendNode(childID, depth)
		}
		return
	}

	role := ""
	if node.Role != nil {
		role = axValueStr(node.Role.Value)
	}

	// Skip noisy roles but process children
	if skipRoles[role] {
		for _, childID := range b.childMap[id] {
			b.appendNode(childID, depth)
		}
		return
	}

	name := ""
	if node.Name != nil {
		name = axValueStr(node.Name.Value)
	}

	value := ""
	if node.Value != nil {
		value = axValueStr(node.Value.Value)
	}

	// Skip empty leaf nodes
	if role == "" && name == "" && value == "" && len(b.childMap[id]) == 0 {
		return
	}

	// Indent
	for i := 0; i < depth; i++ {
		b.buf = append(b.buf, ' ', ' ')
	}

	// Assign @ref for interactive elements
	if interactiveRoles[role] && node.BackendDOMNodeID != 0 {
		ref := b.nextRef
		b.nextRef++
		b.refMap[ref] = int(node.BackendDOMNodeID)
		b.buf = append(b.buf, '@')
		b.buf = append(b.buf, strconv.Itoa(ref)...)
		b.buf = append(b.buf, ' ')
	}

	// Role (with level suffix for headings)
	if role != "" {
		b.buf = append(b.buf, '[')
		b.buf = append(b.buf, role...)
		// Append level for headings inline: [heading:2]
		for _, prop := range node.Properties {
			if prop != nil && string(prop.Name) == "level" && prop.Value != nil {
				b.buf = append(b.buf, ':')
				b.buf = append(b.buf, axValueStr(prop.Value.Value)...)
				break
			}
		}
		b.buf = append(b.buf, ']')
	}

	// Name
	if name != "" {
		b.buf = append(b.buf, ' ', '"')
		b.buf = append(b.buf, name...)
		b.buf = append(b.buf, '"')
	}

	// Value (for inputs, if different from name)
	if value != "" && value != name {
		b.buf = append(b.buf, " val="...)
		b.buf = append(b.buf, value...)
	}

	// Key properties (skip level — already in role suffix)
	for _, prop := range node.Properties {
		if prop == nil || prop.Name == "" {
			continue
		}
		switch string(prop.Name) {
		case "focused":
			if prop.Value != nil && axValueStr(prop.Value.Value) == "true" {
				b.buf = append(b.buf, " focused"...)
			}
		case "checked":
			if prop.Value != nil {
				b.buf = append(b.buf, " checked="...)
				b.buf = append(b.buf, axValueStr(prop.Value.Value)...)
			}
		case "disabled":
			if prop.Value != nil && axValueStr(prop.Value.Value) == "true" {
				b.buf = append(b.buf, " disabled"...)
			}
		case "required":
			if prop.Value != nil && axValueStr(prop.Value.Value) == "true" {
				b.buf = append(b.buf, " required"...)
			}
		case "url":
			if prop.Value != nil {
				b.buf = append(b.buf, " href="...)
				b.buf = append(b.buf, axValueStr(prop.Value.Value)...)
			}
		}
	}

	b.buf = append(b.buf, '\n')

	// Children
	for _, childID := range b.childMap[id] {
		b.appendNode(childID, depth+1)
	}
}

// ActByRef performs an action on an element identified by @ref from the accessibility tree DSL.
// This resolves the ref to a backend DOM node ID and uses CDP to interact with it.
func (s *RodService) ActByRef(ctx context.Context, targetID string, ref int, refMap map[int]int, action string, value string) (*ActResponse, error) {
	backendNodeID, ok := refMap[ref]
	if !ok {
		return nil, fmt.Errorf("unknown ref @%d", ref)
	}

	tab, err := s.getTab(ctx, targetID)
	if err != nil {
		return nil, err
	}

	// Resolve backend node ID to a rod Element
	el, err := tab.page.ElementFromNode(&proto.DOMNode{
		BackendNodeID: proto.DOMBackendNodeID(backendNodeID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to resolve @%d: %w", ref, err)
	}

	switch action {
	case "click":
		err = el.Click(proto.InputMouseButtonLeft, 1)
	case "type":
		err = el.Input(value)
	case "focus":
		_, err = el.Eval(`() => this.focus()`)
	case "hover":
		err = el.Hover()
	case "scroll":
		err = el.ScrollIntoView()
	case "select":
		err = el.Select([]string{value}, true, rod.SelectorTypeText)
	default:
		return nil, fmt.Errorf("unsupported action: %s (use click, type, focus, hover, scroll, select)", action)
	}

	if err != nil {
		return nil, err
	}

	return &ActResponse{Success: true}, nil
}

// PageScroll performs a relative page scroll using the existing scroll action
// path so compatibility shims can reuse browser-native scrolling semantics.
func (s *RodService) PageScroll(ctx context.Context, targetID string, x, y int) error {
	_, err := s.Act(ctx, &ActRequest{
		Kind:     "scroll",
		TargetID: targetID,
		X:        x,
		Y:        y,
	})
	return err
}

// CountInteractiveElements returns the count of interactive elements on the page.
// This is a lightweight JS call — no DSL building, no ref map.
func (s *RodService) CountInteractiveElements(ctx context.Context, targetID string) (int, error) {
	tab, err := s.getTab(ctx, targetID)
	if err != nil {
		return 0, err
	}

	result, err := tab.page.Eval(`() => {
		const selectors = '` + interactiveSelector + `';
		const els = document.querySelectorAll(selectors);
		let count = 0;
		for (let i = 0; i < els.length; i++) {
			const el = els[i];
			if (el.offsetParent === null && el.tagName !== 'INPUT' && el.type !== 'hidden') continue;
			const rect = el.getBoundingClientRect();
			if (rect.width === 0 && rect.height === 0) continue;
			count++;
		}
		return count;
	}`)
	if err != nil {
		return 0, fmt.Errorf("failed to count interactive elements: %w", err)
	}

	return result.Value.Int(), nil
}

// ScreenshotTab takes a screenshot of an existing tab by target ID.
func (s *RodService) ScreenshotTab(ctx context.Context, targetID string) (string, error) {
	tab, err := s.getTab(ctx, targetID)
	if err != nil {
		return "", err
	}

	data, err := tab.page.Screenshot(true, nil)
	if err != nil {
		if isConnectionClosed(err) {
			s.removeTab(tab)
			return "", fmt.Errorf("browser connection lost (tab removed): %w", err)
		}
		return "", fmt.Errorf("screenshot failed: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString(data)
	s.rememberSessionScreenshot(tab, encoded, "full_page")
	return encoded, nil
}

// ScreenshotViewport takes a viewport-only screenshot (no full-page scroll capture).
func (s *RodService) ScreenshotViewport(ctx context.Context, targetID string) (string, error) {
	data, err := s.ScreenshotViewportRaw(ctx, targetID)
	if err != nil {
		return "", err
	}
	tab, tabErr := s.getTab(ctx, targetID)
	if tabErr != nil {
		return "", tabErr
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	s.rememberSessionScreenshot(tab, encoded, "viewport")
	return encoded, nil
}

// ScreenshotViewportRaw takes a viewport-only screenshot and returns raw PNG bytes.
func (s *RodService) ScreenshotViewportRaw(ctx context.Context, targetID string) ([]byte, error) {
	tab, err := s.getTab(ctx, targetID)
	if err != nil {
		return nil, err
	}

	data, err := tab.page.Screenshot(false, nil)
	if err != nil {
		if isConnectionClosed(err) {
			s.removeTab(tab)
			return nil, fmt.Errorf("browser connection lost (tab removed): %w", err)
		}
		return nil, fmt.Errorf("screenshot failed: %w", err)
	}

	return data, nil
}

// ScrollTo scrolls the page to the given absolute position.
func (s *RodService) ScrollTo(ctx context.Context, targetID string, x, y int) error {
	tab, err := s.getTab(ctx, targetID)
	if err != nil {
		return err
	}
	// Use behavior:'instant' to bypass smooth-scroll CSS that can cause timing issues.
	// Verify scroll position after — some pages override or block scrollTo.
	js := fmt.Sprintf(`() => {
		window.scrollTo({left: %d, top: %d, behavior: 'instant'});
		return Math.abs(window.scrollY - %d) < 50;
	}`, x, y, y)
	result, err := tab.page.Eval(js)
	if err != nil {
		return err
	}
	if !result.Value.Bool() {
		// Scroll didn't reach target — page may have fixed/sticky elements or limited scroll height.
		// Not a hard error, caller can still screenshot at current position.
	}
	return nil
}

// PageDimensions returns the viewport height and total scroll height of the page.
func (s *RodService) PageDimensions(ctx context.Context, targetID string) (viewportH, scrollH int, err error) {
	tab, err := s.getTab(ctx, targetID)
	if err != nil {
		return 0, 0, err
	}
	result, err := tab.page.Eval(`() => ({ vh: window.innerHeight, sh: document.documentElement.scrollHeight })`)
	if err != nil {
		return 0, 0, fmt.Errorf("page dimensions eval failed: %w", err)
	}
	vh := result.Value.Get("vh").Int()
	sh := result.Value.Get("sh").Int()
	return int(vh), int(sh), nil
}

// SetViewport changes the viewport size of an existing tab.
func (s *RodService) SetViewport(ctx context.Context, targetID string, width, height int) error {
	tab, err := s.getTab(ctx, targetID)
	if err != nil {
		return err
	}
	return tab.page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width:  width,
		Height: height,
	})
}

// InteractiveElements extracts only interactive elements from the page using JS.
// Much lighter than the full accessibility tree — returns a compact DSL with @ref IDs.
// Each @ref maps to a CSS selector for action targeting.
func (s *RodService) InteractiveElements(ctx context.Context, targetID string) (*InteractiveElementsResponse, error) {
	tab, err := s.getTab(ctx, targetID)
	if err != nil {
		return nil, err
	}

	page := tab.page

	// Wait for page to be stable before extracting
	_ = waitPageStable(page, 500*time.Millisecond)

	// JS extraction: only interactive elements
	result, err := page.Eval(`() => {
		const selectors = '` + interactiveSelector + `';
		const els = document.querySelectorAll(selectors);
		const items = [];
		for (let i = 0; i < els.length; i++) {
			const el = els[i];
			if (el.offsetParent === null && el.tagName !== 'INPUT' && el.type !== 'hidden') continue; // skip hidden
			const rect = el.getBoundingClientRect();
			if (rect.width === 0 && rect.height === 0) continue; // skip zero-size
			const item = {
				tag: el.tagName.toLowerCase(),
				text: (el.innerText || el.textContent || '').trim().slice(0, 80),
				role: el.getAttribute('role') || '',
				type: el.type || '',
				name: el.name || '',
				placeholder: el.placeholder || '',
				href: el.href || '',
				value: el.value || '',
				checked: el.checked || false,
				disabled: el.disabled || false,
				ariaLabel: el.getAttribute('aria-label') || '',
			};
			items.push(item);
		}
		return { items: items, url: location.href, title: document.title };
	}`)
	if err != nil {
		return nil, fmt.Errorf("failed to extract interactive elements: %w", err)
	}

	url := result.Value.Get("url").Str()
	title := result.Value.Get("title").Str()
	rawItems := result.Value.Get("items").Arr()

	var b interactiveBuilder
	b.refMap = make(map[int]string)
	b.grow(len(rawItems) * 40)

	for i, item := range rawItems {
		ref := i + 1
		tag := item.Get("tag").Str()
		text := item.Get("text").Str()
		role := item.Get("role").Str()
		typ := item.Get("type").Str()
		placeholder := item.Get("placeholder").Str()
		href := item.Get("href").Str()
		ariaLabel := item.Get("ariaLabel").Str()
		disabled := item.Get("disabled").Bool()
		checked := item.Get("checked").Bool()

		b.writeRef(ref)

		// Tag/role
		if role != "" {
			b.writeString("[" + role + "]")
		} else {
			b.writeString("[" + tag + "]")
		}

		// Label: prefer aria-label > text > placeholder
		label := ariaLabel
		if label == "" {
			label = text
		}
		if label == "" {
			label = placeholder
		}
		if label != "" {
			b.writeString(" \"" + label + "\"")
		}

		// Type info for inputs
		if typ != "" && typ != "submit" && typ != "button" {
			b.writeString(" type=" + typ)
		}

		// Href for links
		if href != "" && tag == "a" {
			b.writeString(" href=" + href)
		}

		// State
		if disabled {
			b.writeString(" disabled")
		}
		if checked {
			b.writeString(" checked")
		}

		b.writeString("\n")

		// Build selector for this element (by index in querySelectorAll result)
		b.refMap[ref] = fmt.Sprintf("__interactive_ref_%d", i)
	}

	return &InteractiveElementsResponse{
		Tree:     b.String(),
		URL:      url,
		Title:    title,
		TargetID: tab.targetID,
		RefMap:   b.refMap,
		Count:    len(rawItems),
	}, nil
}

// ActByInteractiveRef performs an action on an element identified by @ref from InteractiveElements.
func (s *RodService) ActByInteractiveRef(ctx context.Context, targetID string, ref int, refMap map[int]string, action string, value string) (*ActResponse, error) {
	selectorKey, ok := refMap[ref]
	if !ok {
		return nil, fmt.Errorf("unknown ref @%d", ref)
	}

	tab, err := s.getTab(ctx, targetID)
	if err != nil {
		return nil, err
	}

	// Extract the index from the selector key
	var idx int
	if _, err := fmt.Sscanf(selectorKey, "__interactive_ref_%d", &idx); err != nil {
		return nil, fmt.Errorf("invalid ref selector: %s", selectorKey)
	}

	// Use JS to find and act on the element by index
	page := tab.page
	jsAction := ""
	switch action {
	case "click":
		jsAction = "el.click()"
	case "focus":
		jsAction = "el.focus()"
	case "hover":
		jsAction = "el.dispatchEvent(new MouseEvent('mouseover', {bubbles: true}))"
	case "scroll":
		jsAction = "el.scrollIntoView({behavior: 'smooth', block: 'center'})"
	case "type":
		// For type, we use rod's Input method for proper event dispatch
	case "select":
		// For select, we use rod's Select method
	default:
		return nil, fmt.Errorf("unsupported action: %s", action)
	}

	if action == "type" || action == "select" {
		// Use rod element for these — need proper event dispatch
		selector := fmt.Sprintf(`document.querySelectorAll('%s')[%d]`, interactiveSelector, idx)
		el, err := page.ElementByJS(rod.Eval(fmt.Sprintf(`() => %s`, selector)))
		if err != nil {
			return nil, fmt.Errorf("failed to find element @%d: %w", ref, err)
		}
		if action == "type" {
			_ = el.SelectAllText()
			_ = el.Input(value)
		} else {
			_ = el.Select([]string{value}, true, rod.SelectorTypeText)
		}
	} else {
		_, err := page.Eval(fmt.Sprintf(`() => {
			const els = document.querySelectorAll('%s');
			const el = els[%d];`, interactiveSelector, idx) + `
			if (!el) throw new Error('element not found');
			` + jsAction + `;
		}`)
		if err != nil {
			return nil, fmt.Errorf("action %s on @%d failed: %w", action, ref, err)
		}
	}

	return &ActResponse{Success: true}, nil
}

// interactiveBuilder builds the compact DSL for interactive elements.
type interactiveBuilder struct {
	buf    []byte
	refMap map[int]string
}

func (b *interactiveBuilder) grow(n int) {
	if cap(b.buf)-len(b.buf) < n {
		newBuf := make([]byte, len(b.buf), len(b.buf)+n)
		copy(newBuf, b.buf)
		b.buf = newBuf
	}
}

func (b *interactiveBuilder) writeString(s string) {
	b.buf = append(b.buf, s...)
}

func (b *interactiveBuilder) writeRef(ref int) {
	b.buf = append(b.buf, '@')
	b.buf = append(b.buf, strconv.Itoa(ref)...)
	b.buf = append(b.buf, ' ')
}

func (b *interactiveBuilder) String() string {
	return string(b.buf)
}

// ExecuteRecipe runs a named recipe with the given params.
func (s *RodService) ExecuteRecipe(ctx context.Context, req *RecipeRequest) (*RecipeResponse, error) {
	if err := s.Start(ctx); err != nil {
		return nil, err
	}

	result, err := s.recipes.Execute(ctx, s, req.Recipe, req.Params)
	if err != nil {
		return nil, err
	}

	return &RecipeResponse{
		Success:  result.Success,
		Recipe:   req.Recipe,
		Data:     result.Data,
		TargetID: result.TargetID,
		Message:  result.Message,
	}, nil
}

// Recipes returns the recipe registry.
func (s *RodService) Recipes() *RecipeRegistry {
	return s.recipes
}

// Close closes the browser service.
func (s *RodService) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Close all tabs
	s.tabsMu.Lock()
	for _, tab := range s.tabs {
		if tab.page != nil {
			_ = tab.page.Close()
		}
	}
	s.tabs = make(map[string]*tabInfo)
	s.tabsMu.Unlock()

	s.started = false
	return s.pool.Close()
}
