package tools

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
)

type SessionCoreState struct {
	ID             string                       `json:"id"`
	UserAgent      string                       `json:"user_agent,omitempty"`
	Headers        map[string]string            `json:"headers,omitempty"`
	Cookies        []browser.Cookie             `json:"cookies,omitempty"`
	LocalStorage   map[string]map[string]string `json:"local_storage,omitempty"`
	SessionStorage map[string]map[string]string `json:"session_storage,omitempty"`
	PrimaryRuntime string                       `json:"primary_runtime,omitempty"`
	MirrorTargets  []string                     `json:"mirror_targets,omitempty"`
	UpdatedAt      time.Time                    `json:"updated_at,omitempty"`
}

type sessionStateBrowserBackend interface {
	ExportSessionState(ctx context.Context, targetID string, rawURL string) (*SessionCoreState, error)
	ApplySessionState(ctx context.Context, targetID string, rawURL string, state SessionCoreState) error
}

type sessionStateMirrorBrowserBackend interface {
	MirrorSessionState(ctx context.Context, rawURL string, state SessionCoreState) error
}

type webSessionCoreStore struct {
	mu           sync.Mutex
	entries      map[string]SessionCoreState
	targetToKeys map[string]string
	ttl          time.Duration
}

func newWebSessionCoreStore() *webSessionCoreStore {
	return &webSessionCoreStore{
		entries:      make(map[string]SessionCoreState),
		targetToKeys: make(map[string]string),
		ttl:          webRetrievalDefaultSessionTTL,
	}
}

func (s *webSessionCoreStore) load(key string) (SessionCoreState, bool) {
	if s == nil || strings.TrimSpace(key) == "" {
		return SessionCoreState{}, false
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.entries[key]
	if !ok {
		return SessionCoreState{}, false
	}
	if !state.UpdatedAt.IsZero() && s.ttl > 0 && now.Sub(state.UpdatedAt) > s.ttl {
		delete(s.entries, key)
		return SessionCoreState{}, false
	}
	return cloneSessionCoreState(state), true
}

func (s *webSessionCoreStore) store(key string, state SessionCoreState) {
	if s == nil || strings.TrimSpace(key) == "" {
		return
	}
	state.ID = firstNonEmpty(strings.TrimSpace(state.ID), strings.TrimSpace(key))
	state.UpdatedAt = time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[key] = cloneSessionCoreState(state)
	for _, target := range state.MirrorTargets {
		target = strings.TrimSpace(target)
		if target == "" {
			continue
		}
		s.targetToKeys[target] = key
	}
}

func (s *webSessionCoreStore) storeTargetMapping(targetID, key string) {
	if s == nil || strings.TrimSpace(targetID) == "" || strings.TrimSpace(key) == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.targetToKeys[strings.TrimSpace(targetID)] = strings.TrimSpace(key)
}

func (s *webSessionCoreStore) loadByTarget(targetID string) (SessionCoreState, bool) {
	if s == nil || strings.TrimSpace(targetID) == "" {
		return SessionCoreState{}, false
	}
	s.mu.Lock()
	key := s.targetToKeys[strings.TrimSpace(targetID)]
	s.mu.Unlock()
	if strings.TrimSpace(key) == "" {
		return SessionCoreState{}, false
	}
	return s.load(key)
}

func cloneSessionCoreState(state SessionCoreState) SessionCoreState {
	cloned := state
	if len(state.Headers) > 0 {
		cloned.Headers = cloneStringMap(state.Headers)
	}
	if len(state.Cookies) > 0 {
		cloned.Cookies = append([]browser.Cookie(nil), state.Cookies...)
	}
	if len(state.LocalStorage) > 0 {
		cloned.LocalStorage = cloneNestedStringMap(state.LocalStorage)
	}
	if len(state.SessionStorage) > 0 {
		cloned.SessionStorage = cloneNestedStringMap(state.SessionStorage)
	}
	if len(state.MirrorTargets) > 0 {
		cloned.MirrorTargets = append([]string(nil), state.MirrorTargets...)
	}
	return cloned
}

func cloneNestedStringMap(src map[string]map[string]string) map[string]map[string]string {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]map[string]string, len(src))
	for outer, inner := range src {
		if len(inner) == 0 {
			out[outer] = map[string]string{}
			continue
		}
		out[outer] = cloneStringMap(inner)
	}
	return out
}

func (w *WebFetchTool) sessionCoreKey(ctx context.Context, host, browserTargetID string) string {
	if key := strings.TrimSpace(w.retrievalSessionKey(ctx, host)); key != "" {
		return key
	}
	if targetID := strings.TrimSpace(browserTargetID); targetID != "" {
		return "target:" + targetID + "|" + strings.ToLower(strings.TrimSpace(host))
	}
	return ""
}

func (w *WebFetchTool) loadSessionCore(ctx context.Context, targetURL, browserTargetID string) (SessionCoreState, bool) {
	if w == nil || w.sessionCore == nil {
		return SessionCoreState{}, false
	}
	host := webFetchHostForURL(targetURL)
	if state, ok := w.sessionCore.load(w.sessionCoreKey(ctx, host, browserTargetID)); ok {
		return state, true
	}
	if state, ok := w.sessionCore.loadByTarget(browserTargetID); ok {
		return state, true
	}
	return SessionCoreState{}, false
}

func (w *WebFetchTool) rememberSessionCore(ctx context.Context, targetURL, browserTargetID, primaryRuntime string) {
	if w == nil || w.sessionCore == nil || w.browser == nil {
		return
	}
	targetID := strings.TrimSpace(browserTargetID)
	if targetID == "" {
		return
	}
	backend, ok := w.browser.(sessionStateBrowserBackend)
	if !ok {
		return
	}
	state, err := backend.ExportSessionState(ctx, targetID, targetURL)
	if err != nil || state == nil {
		return
	}
	state.PrimaryRuntime = firstNonEmpty(strings.TrimSpace(state.PrimaryRuntime), strings.TrimSpace(primaryRuntime))
	if !stringSliceContains(state.MirrorTargets, targetID) {
		state.MirrorTargets = append(state.MirrorTargets, targetID)
	}
	key := w.sessionCoreKey(ctx, webFetchHostForURL(targetURL), targetID)
	if strings.TrimSpace(key) == "" {
		return
	}
	w.sessionCore.store(key, *state)
	w.sessionCore.storeTargetMapping(targetID, key)
	if mirror, ok := w.browser.(sessionStateMirrorBrowserBackend); ok {
		_ = mirror.MirrorSessionState(ctx, targetURL, *state)
	}
}

func (w *WebFetchTool) applySessionCoreToRequestOptions(ctx context.Context, normalizedURL string, opts *webFetchRequestOptions) error {
	if w == nil || opts == nil {
		return nil
	}
	state, ok := w.loadSessionCore(ctx, normalizedURL, opts.browserTargetID)
	if !ok {
		return nil
	}
	if opts.extraHeaders == nil {
		opts.extraHeaders = make(map[string]string)
	}
	for name, value := range state.Headers {
		if strings.EqualFold(name, "Cookie") {
			continue
		}
		if strings.TrimSpace(value) == "" {
			continue
		}
		if _, exists := opts.extraHeaders[http.CanonicalHeaderKey(name)]; exists {
			continue
		}
		if err := setWebFetchHeader(opts.extraHeaders, name, value); err != nil {
			return err
		}
	}
	if strings.TrimSpace(state.UserAgent) != "" && strings.TrimSpace(opts.extraHeaders[http.CanonicalHeaderKey("User-Agent")]) == "" {
		if err := setWebFetchHeader(opts.extraHeaders, "User-Agent", state.UserAgent); err != nil {
			return err
		}
	}
	if cookieHeader := sessionCoreCookieHeader(state, normalizedURL); strings.TrimSpace(cookieHeader) != "" {
		if existing := strings.TrimSpace(opts.extraHeaders[http.CanonicalHeaderKey("Cookie")]); existing != "" {
			cookieHeader = mergeWebFetchCookieHeaders(cookieHeader, existing)
		}
		if err := setWebFetchHeader(opts.extraHeaders, "Cookie", cookieHeader); err != nil {
			return err
		}
	}
	opts.cacheable = false
	return nil
}

func sessionCoreCookieHeader(state SessionCoreState, rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	path := parsed.EscapedPath()
	if path == "" {
		path = "/"
	}
	pairs := make([]string, 0, len(state.Cookies))
	for _, cookie := range state.Cookies {
		if strings.TrimSpace(cookie.Name) == "" {
			continue
		}
		if domain := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(cookie.Domain, "."))); domain != "" && host != domain && !strings.HasSuffix(host, "."+domain) {
			continue
		}
		cookiePath := strings.TrimSpace(cookie.Path)
		if cookiePath != "" && !strings.HasPrefix(path, cookiePath) {
			continue
		}
		pairs = append(pairs, cookie.Name+"="+cookie.Value)
	}
	return strings.Join(pairs, "; ")
}

func sessionCoreAsBrowserProfile(state SessionCoreState) *browser.Profile {
	return &browser.Profile{
		ID:             state.ID,
		UserAgent:      state.UserAgent,
		Cookies:        append([]browser.Cookie(nil), state.Cookies...),
		Headers:        cloneStringMap(state.Headers),
		LocalStorage:   cloneNestedStringMap(state.LocalStorage),
		SessionStorage: cloneNestedStringMap(state.SessionStorage),
	}
}

func stringSliceContains(values []string, needle string) bool {
	needle = strings.TrimSpace(needle)
	if needle == "" {
		return false
	}
	for _, value := range values {
		if strings.TrimSpace(value) == needle {
			return true
		}
	}
	return false
}

func compactStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
