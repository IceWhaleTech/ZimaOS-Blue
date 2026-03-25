package browser

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"golang.org/x/net/html"
)

var (
	// ErrLightpandaUnsupportedCapability reports that the requested operation
	// exceeds Lightpanda v1's read-only capability boundary.
	ErrLightpandaUnsupportedCapability = errors.New("unsupported_capability")
	// ErrLightpandaDOMUnstable reports that the fetched DOM is not stable enough
	// to trust for structured read-only extraction.
	ErrLightpandaDOMUnstable = errors.New("dom_unstable")
	// ErrLightpandaEmptyTree reports that no meaningful DOM tree could be built.
	ErrLightpandaEmptyTree = errors.New("empty_tree")
	// ErrLightpandaNavigationBlocked reports that the target page could not be
	// fetched in read-only mode.
	ErrLightpandaNavigationBlocked = errors.New("navigation_blocked")
	// ErrLightpandaJSRequired reports that the target page appears to require a
	// JS-capable browser before content becomes readable.
	ErrLightpandaJSRequired = errors.New("js_required")
)

const (
	lightpandaSessionPrefix      = "lightpanda-"
	lightpandaDefaultSummaryLen  = 360
	lightpandaTreePreviewMaxLen  = 1200
	lightpandaTreeMaxLines       = 120
	lightpandaFetchMaxBodyBytes  = 2 << 20
	lightpandaDefaultUserAgent   = "Mozilla/5.0 (compatible; Blue Lightpanda/1.0; +https://zimaos.com)"
	lightpandaDefaultStatus      = "active"
	lightpandaInteractiveMaxRows = 80
)

type lightpandaSession struct {
	id               string
	url              string
	title            string
	status           string
	createdAt        time.Time
	lastActivity     time.Time
	updatedAt        time.Time
	summary          string
	treePreview      string
	a11yTree         string
	interactiveTree  string
	interactiveCount int
	client           *http.Client
}

type lightpandaDocument struct {
	url              string
	title            string
	content          string
	summary          string
	treePreview      string
	a11yTree         string
	interactiveTree  string
	interactiveCount int
	jsRequired       bool
}

type lightpandaNodeDescriptor struct {
	depth int
	line  string
}

// LightpandaService provides Blue's read-layer Lightpanda shim session model.
type LightpandaService struct {
	config        *Config
	security      *SecurityChecker
	binaryManager *LightpandaBinaryManager

	mu       sync.RWMutex
	started  bool
	sessions map[string]*lightpandaSession
	nextID   atomic.Uint64
}

// NewLightpandaService creates a Lightpanda read-layer session service.
func NewLightpandaService(config *Config) *LightpandaService {
	cfg := config.Clone()
	return &LightpandaService{
		config:        cfg,
		security:      NewSecurityChecker(cfg),
		binaryManager: NewLightpandaBinaryManager(cfg),
		sessions:      make(map[string]*lightpandaSession),
	}
}

// Start starts the Lightpanda service. The shim runtime is HTTP/DOM based, so
// startup is intentionally lightweight.
func (s *LightpandaService) Start(ctx context.Context) error {
	if s == nil {
		return ErrBrowserNotAvailable
	}
	s.mu.Lock()
	s.started = true
	s.mu.Unlock()
	return nil
}

// EnsureBinary resolves or downloads the configured upstream Lightpanda binary.
func (s *LightpandaService) EnsureBinary(ctx context.Context) (string, error) {
	if s == nil || s.binaryManager == nil {
		return "", ErrBrowserNotAvailable
	}
	return s.binaryManager.Ensure(ctx)
}

// BinaryAvailable reports whether a usable upstream Lightpanda binary already
// exists locally without triggering a download attempt.
func (s *LightpandaService) BinaryAvailable() bool {
	if s == nil || s.binaryManager == nil {
		return false
	}
	_, ok, err := s.binaryManager.ReadyPath()
	return err == nil && ok
}

// ReadDocument fetches a page through the Lightpanda shim without persisting a
// browser session. This is used by fetch/read-layer routing.
func (s *LightpandaService) ReadDocument(ctx context.Context, rawURL string, timeoutMS int) (*LightpandaReadDocument, error) {
	if s == nil {
		return nil, ErrBrowserNotAvailable
	}
	if err := s.Start(ctx); err != nil {
		return nil, err
	}
	urlString, err := s.security.NormalizeAndCheckURL(rawURL)
	if err != nil {
		return nil, err
	}
	client, err := s.newHTTPClient()
	if err != nil {
		return nil, err
	}
	doc, err := s.fetchDocument(ctx, &lightpandaSession{client: client}, urlString, timeoutMS)
	if err != nil {
		return nil, err
	}
	return &LightpandaReadDocument{
		URL:               doc.url,
		Title:             doc.title,
		Content:           doc.content,
		Summary:           doc.summary,
		TreePreview:       doc.treePreview,
		AccessibilityTree: doc.a11yTree,
		InteractiveTree:   doc.interactiveTree,
		InteractiveCount:  doc.interactiveCount,
	}, nil
}

// HasSession reports whether targetID belongs to a Lightpanda session.
func (s *LightpandaService) HasSession(targetID string) bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.sessions[strings.TrimSpace(targetID)]
	return ok
}

// Tabs returns all active Lightpanda sessions as browser tabs.
func (s *LightpandaService) Tabs(context.Context) ([]*Tab, error) {
	if s == nil {
		return nil, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	tabs := make([]*Tab, 0, len(s.sessions))
	for _, session := range s.sessions {
		if session == nil {
			continue
		}
		tabs = append(tabs, &Tab{
			TargetID: session.id,
			URL:      session.url,
			Title:    session.title,
			Active:   session.status == lightpandaDefaultStatus,
		})
	}
	return tabs, nil
}

// Navigate fetches a page in read-only mode and stores/update a Lightpanda session.
func (s *LightpandaService) Navigate(ctx context.Context, req *NavigateRequest) (*NavigateResponse, error) {
	if s == nil {
		return nil, ErrBrowserNotAvailable
	}
	if req == nil {
		return nil, lightpandaCapabilityError(ErrLightpandaNavigationBlocked, "missing request")
	}
	if err := s.Start(ctx); err != nil {
		return nil, err
	}

	targetID := strings.TrimSpace(req.TargetID)
	urlString, err := s.security.NormalizeAndCheckURL(req.URL)
	if err != nil {
		return nil, err
	}

	session, err := s.ensureSession(targetID)
	if err != nil {
		return nil, err
	}
	doc, err := s.fetchDocument(ctx, session, urlString, req.Timeout)
	if err != nil {
		return nil, err
	}

	now := timeutil.NowTime().UTC()
	s.mu.Lock()
	session.url = doc.url
	session.title = doc.title
	session.summary = doc.summary
	session.treePreview = doc.treePreview
	session.a11yTree = doc.a11yTree
	session.interactiveTree = doc.interactiveTree
	session.interactiveCount = doc.interactiveCount
	session.updatedAt = now
	session.lastActivity = now
	session.status = lightpandaDefaultStatus
	s.sessions[session.id] = session
	s.mu.Unlock()

	return &NavigateResponse{
		URL:      doc.url,
		Title:    doc.title,
		TargetID: session.id,
	}, nil
}

// AccessibilityTree returns the current Lightpanda tree for a session.
func (s *LightpandaService) AccessibilityTree(ctx context.Context, targetID string, _ int) (*AccessibilityTreeResponse, error) {
	session, err := s.session(targetID)
	if err != nil {
		return nil, err
	}
	s.touchSession(targetID)
	tree := strings.TrimSpace(session.a11yTree)
	if tree == "" {
		return nil, lightpandaCapabilityError(ErrLightpandaEmptyTree, "no readable tree")
	}
	return &AccessibilityTreeResponse{
		Tree:     tree,
		URL:      session.url,
		Title:    session.title,
		TargetID: session.id,
	}, nil
}

// InteractiveElements returns the current interactive element list for a session.
func (s *LightpandaService) InteractiveElements(ctx context.Context, targetID string) (*InteractiveElementsResponse, error) {
	session, err := s.session(targetID)
	if err != nil {
		return nil, err
	}
	s.touchSession(targetID)
	tree := strings.TrimSpace(session.interactiveTree)
	if tree == "" && session.interactiveCount == 0 {
		return nil, lightpandaCapabilityError(ErrLightpandaEmptyTree, "no interactive elements found")
	}
	return &InteractiveElementsResponse{
		Tree:     tree,
		URL:      session.url,
		Title:    session.title,
		TargetID: session.id,
		Count:    session.interactiveCount,
	}, nil
}

// CountInteractiveElements returns the cached interactive element count for a session.
func (s *LightpandaService) CountInteractiveElements(ctx context.Context, targetID string) (int, error) {
	session, err := s.session(targetID)
	if err != nil {
		return 0, err
	}
	s.touchSession(targetID)
	return session.interactiveCount, nil
}

// CloseTab closes a Lightpanda session.
func (s *LightpandaService) CloseTab(_ context.Context, targetID string) error {
	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, targetID)
	return nil
}

// SessionInfo returns a single Lightpanda session summary.
func (s *LightpandaService) SessionInfo(targetID string) (*SessionInfo, error) {
	session, err := s.session(targetID)
	if err != nil {
		return nil, err
	}
	info := lightpandaSessionInfo(session)
	return &info, nil
}

// ListSessionInfos returns all Lightpanda session summaries.
func (s *LightpandaService) ListSessionInfos() []SessionInfo {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]SessionInfo, 0, len(s.sessions))
	for _, session := range s.sessions {
		if session == nil {
			continue
		}
		out = append(out, lightpandaSessionInfo(session))
	}
	return out
}

// CaptureMonitor returns the text-based monitor payload for a Lightpanda session.
func (s *LightpandaService) CaptureMonitor(targetID string) (*SessionMonitorResponse, error) {
	session, err := s.session(targetID)
	if err != nil {
		return nil, err
	}
	return &SessionMonitorResponse{
		Kind: SessionMonitorKindText,
		Text: &SessionTextMonitor{
			Title:            session.title,
			URL:              session.url,
			Summary:          session.summary,
			TreePreview:      session.treePreview,
			InteractiveCount: session.interactiveCount,
			UpdatedAt:        session.updatedAt.UTC().Format(time.RFC3339),
			Status:           session.status,
		},
	}, nil
}

// CaptureScreenshot returns the compatibility payload for the legacy screenshot
// endpoint. Lightpanda does not provide image frames in v1.
func (s *LightpandaService) CaptureScreenshot(targetID string) (*SessionScreenshotResponse, error) {
	if _, err := s.session(targetID); err != nil {
		return nil, err
	}
	return &SessionScreenshotResponse{
		Screenshot: "",
		History:    nil,
		Error:      lightpandaCapabilityError(ErrLightpandaUnsupportedCapability, "lightpanda monitor is text-only").Error(),
	}, nil
}

func (s *LightpandaService) ensureSession(targetID string) (*lightpandaSession, error) {
	targetID = strings.TrimSpace(targetID)
	if targetID != "" {
		return s.session(targetID)
	}
	client, err := s.newHTTPClient()
	if err != nil {
		return nil, err
	}
	now := timeutil.NowTime().UTC()
	id := lightpandaSessionPrefix + strconv.FormatUint(s.nextID.Add(1), 10)
	return &lightpandaSession{
		id:           id,
		status:       lightpandaDefaultStatus,
		createdAt:    now,
		lastActivity: now,
		updatedAt:    now,
		client:       client,
	}, nil
}

func (s *LightpandaService) session(targetID string) (*lightpandaSession, error) {
	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		return nil, lightpandaCapabilityError(ErrLightpandaUnsupportedCapability, "lightpanda sessions require target_id continuity")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	session := s.sessions[targetID]
	if session == nil {
		return nil, ErrTabNotFound
	}
	return cloneLightpandaSession(session), nil
}

func (s *LightpandaService) touchSession(targetID string) {
	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if session := s.sessions[targetID]; session != nil {
		session.lastActivity = timeutil.NowTime().UTC()
	}
}

func (s *LightpandaService) fetchDocument(ctx context.Context, session *lightpandaSession, rawURL string, timeoutMS int) (*lightpandaDocument, error) {
	if session == nil || session.client == nil {
		return nil, lightpandaCapabilityError(ErrLightpandaNavigationBlocked, "session client unavailable")
	}
	timeout := GetTimeout(timeoutMS, s.config)
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, lightpandaCapabilityError(ErrLightpandaNavigationBlocked, err.Error())
	}
	req.Header.Set("User-Agent", s.userAgent())
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,text/plain;q=0.8,*/*;q=0.7")

	resp, err := session.client.Do(req)
	if err != nil {
		return nil, lightpandaCapabilityError(ErrLightpandaNavigationBlocked, err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, lightpandaCapabilityError(ErrLightpandaNavigationBlocked, fmt.Sprintf("http %d", resp.StatusCode))
	}

	contentType := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	switch {
	case strings.Contains(contentType, "text/html"):
	case strings.Contains(contentType, "application/xhtml+xml"):
	case strings.Contains(contentType, "text/plain"):
	default:
		if contentType != "" {
			return nil, lightpandaCapabilityError(ErrLightpandaUnsupportedCapability, "unsupported content type "+contentType)
		}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, lightpandaFetchMaxBodyBytes))
	if err != nil {
		return nil, lightpandaCapabilityError(ErrLightpandaNavigationBlocked, err.Error())
	}
	docNode, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, lightpandaCapabilityError(ErrLightpandaNavigationBlocked, err.Error())
	}
	doc := analyzeLightpandaDocument(docNode, resp.Request.URL.String())
	if doc.jsRequired {
		return nil, lightpandaCapabilityError(ErrLightpandaJSRequired, "page content appears to require javascript")
	}
	if strings.TrimSpace(doc.a11yTree) == "" {
		return nil, lightpandaCapabilityError(ErrLightpandaEmptyTree, "no readable tree extracted")
	}
	return doc, nil
}

func (s *LightpandaService) newHTTPClient() (*http.Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxyURL := strings.TrimSpace(s.config.ProxyURL); proxyURL != "" {
		parsed, err := url.Parse(proxyURL)
		if err != nil {
			return nil, err
		}
		transport.Proxy = http.ProxyURL(parsed)
	}
	return &http.Client{
		Jar:       jar,
		Timeout:   GetTimeout(0, s.config),
		Transport: transport,
	}, nil
}

func (s *LightpandaService) userAgent() string {
	if s == nil || s.config == nil {
		return lightpandaDefaultUserAgent
	}
	if ua := strings.TrimSpace(s.config.UserAgent); ua != "" {
		return ua
	}
	return lightpandaDefaultUserAgent
}

func lightpandaSessionInfo(session *lightpandaSession) SessionInfo {
	if session == nil {
		now := timeutil.NowTime().UTC().Format(time.RFC3339)
		return SessionInfo{
			Status:       "idle",
			CreatedAt:    now,
			LastActivity: now,
			Engine:       SessionEngineLightpanda,
			EngineDetail: SessionEngineDetailLightpandaShim,
			SessionLayer: SessionLayerRead,
			MonitorKind:  SessionMonitorKindText,
		}
	}
	return SessionInfo{
		ID:           session.id,
		Status:       session.status,
		CurrentURL:   session.url,
		PageTitle:    session.title,
		CreatedAt:    session.createdAt.UTC().Format(time.RFC3339),
		LastActivity: session.lastActivity.UTC().Format(time.RFC3339),
		Engine:       SessionEngineLightpanda,
		EngineDetail: SessionEngineDetailLightpandaShim,
		SessionLayer: SessionLayerRead,
		MonitorKind:  SessionMonitorKindText,
	}
}

func cloneLightpandaSession(session *lightpandaSession) *lightpandaSession {
	if session == nil {
		return nil
	}
	cloned := *session
	return &cloned
}

func lightpandaCapabilityError(base error, detail string) error {
	detail = strings.TrimSpace(detail)
	if detail == "" {
		return base
	}
	return fmt.Errorf("%w: %s", base, detail)
}

func analyzeLightpandaDocument(doc *html.Node, rawURL string) *lightpandaDocument {
	title := strings.TrimSpace(extractHTMLTitle(doc))
	summaryText := strings.TrimSpace(strings.Join(collectReadableText(doc, 0, nil), " "))
	summaryText = squashWhitespace(summaryText)
	if title == "" {
		title = fallbackTitle(rawURL, summaryText)
	}

	interactiveNodes := collectInteractiveNodes(doc)
	interactiveLines := make([]string, 0, len(interactiveNodes))
	for i, node := range interactiveNodes {
		if i >= lightpandaInteractiveMaxRows {
			break
		}
		interactiveLines = append(interactiveLines, fmt.Sprintf("@%d %s", i+1, node.line))
	}

	a11yNodes := collectA11yNodes(doc)
	a11yLines := make([]string, 0, len(a11yNodes)+1)
	if title != "" {
		a11yLines = append(a11yLines, fmt.Sprintf("[document] %q", title))
	}
	for _, node := range a11yNodes {
		if len(a11yLines) >= lightpandaTreeMaxLines {
			break
		}
		prefix := strings.Repeat("  ", maxInt(node.depth, 0))
		a11yLines = append(a11yLines, prefix+node.line)
	}

	a11yTree := strings.TrimSpace(strings.Join(a11yLines, "\n"))
	if a11yTree == "" && summaryText != "" {
		a11yTree = fmt.Sprintf("[document] %q\n  [paragraph] %q", title, truncateText(summaryText, 240))
	}
	treePreview := truncateText(a11yTree, lightpandaTreePreviewMaxLen)

	jsRequired := looksLikeJSRequired(doc, summaryText, len(interactiveNodes), len(a11yNodes))
	summary := truncateText(summaryText, lightpandaDefaultSummaryLen)
	content := summaryText
	if strings.TrimSpace(content) == "" {
		content = a11yTree
	}

	return &lightpandaDocument{
		url:              rawURL,
		title:            title,
		content:          content,
		summary:          summary,
		treePreview:      treePreview,
		a11yTree:         a11yTree,
		interactiveTree:  strings.TrimSpace(strings.Join(interactiveLines, "\n")),
		interactiveCount: len(interactiveNodes),
		jsRequired:       jsRequired,
	}
}

func collectReadableText(node *html.Node, depth int, parent *html.Node) []string {
	if node == nil {
		return nil
	}
	switch node.Type {
	case html.TextNode:
		if parent != nil && isTextSuppressedTag(parent.Data) {
			return nil
		}
		text := squashWhitespace(node.Data)
		if text == "" {
			return nil
		}
		return []string{text}
	case html.ElementNode:
		if isTextSuppressedTag(node.Data) {
			return nil
		}
	}

	var out []string
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		out = append(out, collectReadableText(child, depth+1, node)...)
	}
	return out
}

func collectInteractiveNodes(root *html.Node) []lightpandaNodeDescriptor {
	var out []lightpandaNodeDescriptor
	var walk func(node *html.Node, depth int)
	walk = func(node *html.Node, depth int) {
		if node == nil {
			return
		}
		if node.Type == html.ElementNode && isInteractiveTag(node.Data) {
			label := preferredNodeLabel(node)
			line := strings.TrimSpace(interactiveDescriptor(node, label))
			if line != "" {
				out = append(out, lightpandaNodeDescriptor{depth: depth, line: line})
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child, depth+1)
		}
	}
	walk(root, 0)
	return out
}

func collectA11yNodes(root *html.Node) []lightpandaNodeDescriptor {
	var out []lightpandaNodeDescriptor
	var walk func(node *html.Node, depth int)
	walk = func(node *html.Node, depth int) {
		if node == nil {
			return
		}
		if node.Type == html.ElementNode {
			if line := semanticDescriptor(node); line != "" {
				out = append(out, lightpandaNodeDescriptor{depth: depth, line: line})
				if len(out) >= lightpandaTreeMaxLines {
					return
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child, depth+1)
			if len(out) >= lightpandaTreeMaxLines {
				return
			}
		}
	}
	walk(root, 0)
	return out
}

func semanticDescriptor(node *html.Node) string {
	if node == nil || node.Type != html.ElementNode {
		return ""
	}
	tag := strings.ToLower(strings.TrimSpace(node.Data))
	label := preferredNodeLabel(node)
	switch {
	case strings.HasPrefix(tag, "h") && len(tag) == 2 && tag[1] >= '1' && tag[1] <= '6':
		return fmt.Sprintf("[heading%s] %q", tag[1:], label)
	case tag == "main" || tag == "article" || tag == "section" || tag == "nav" || tag == "header" || tag == "footer":
		if label == "" {
			label = tag
		}
		return fmt.Sprintf("[%s] %q", tag, label)
	case tag == "p":
		if label == "" {
			return ""
		}
		return fmt.Sprintf("[paragraph] %q", truncateText(label, 180))
	case tag == "ul" || tag == "ol":
		return fmt.Sprintf("[list] %q", optionalLabel(label, tag))
	case tag == "li":
		if label == "" {
			return ""
		}
		return fmt.Sprintf("[listitem] %q", truncateText(label, 140))
	case isInteractiveTag(tag):
		return fmt.Sprintf("[%s] %q", interactiveRole(tag), optionalLabel(label, tag))
	case tag == "img":
		if alt := attr(node, "alt"); alt != "" {
			return fmt.Sprintf("[image] %q", alt)
		}
	}
	return ""
}

func interactiveDescriptor(node *html.Node, label string) string {
	tag := strings.ToLower(strings.TrimSpace(node.Data))
	role := interactiveRole(tag)
	if label == "" {
		label = tag
	}
	switch tag {
	case "a":
		href := strings.TrimSpace(attr(node, "href"))
		if href != "" {
			return fmt.Sprintf("%s %q href=%q", role, label, href)
		}
	case "input":
		inputType := strings.TrimSpace(attr(node, "type"))
		if inputType == "" {
			inputType = "text"
		}
		return fmt.Sprintf("%s %q type=%q", role, label, inputType)
	}
	return fmt.Sprintf("%s %q", role, label)
}

func interactiveRole(tag string) string {
	switch tag {
	case "a":
		return "link"
	case "button":
		return "button"
	case "select":
		return "select"
	case "textarea":
		return "textarea"
	case "summary":
		return "summary"
	default:
		if tag == "input" {
			return "input"
		}
		return tag
	}
}

func preferredNodeLabel(node *html.Node) string {
	if node == nil {
		return ""
	}
	for _, key := range []string{"aria-label", "title", "placeholder", "alt", "value", "name"} {
		if value := strings.TrimSpace(attr(node, key)); value != "" {
			return truncateText(value, 180)
		}
	}
	text := squashWhitespace(strings.Join(collectReadableText(node, 0, node), " "))
	return truncateText(text, 180)
}

func extractHTMLTitle(root *html.Node) string {
	if root == nil {
		return ""
	}
	var title string
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node == nil || title != "" {
			return
		}
		if node.Type == html.ElementNode && node.Data == "title" && node.FirstChild != nil {
			title = squashWhitespace(node.FirstChild.Data)
			return
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
			if title != "" {
				return
			}
		}
	}
	walk(root)
	return title
}

func looksLikeJSRequired(root *html.Node, summary string, interactiveCount, semanticCount int) bool {
	if len(summary) >= 120 || interactiveCount > 0 || semanticCount > 3 {
		return false
	}
	scriptCount := 0
	appRoot := false
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node == nil {
			return
		}
		if node.Type == html.ElementNode {
			if node.Data == "script" {
				scriptCount++
			}
			id := strings.ToLower(strings.TrimSpace(attr(node, "id")))
			if id == "app" || id == "root" || id == "__next" || id == "__nuxt" {
				appRoot = true
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return appRoot && scriptCount >= 3
}

func isTextSuppressedTag(tag string) bool {
	switch strings.ToLower(strings.TrimSpace(tag)) {
	case "script", "style", "noscript", "svg":
		return true
	default:
		return false
	}
}

func isInteractiveTag(tag string) bool {
	switch strings.ToLower(strings.TrimSpace(tag)) {
	case "a", "button", "input", "select", "textarea", "summary":
		return true
	default:
		return false
	}
}

func attr(node *html.Node, key string) string {
	if node == nil {
		return ""
	}
	key = strings.ToLower(strings.TrimSpace(key))
	for _, attr := range node.Attr {
		if strings.ToLower(strings.TrimSpace(attr.Key)) == key {
			return strings.TrimSpace(attr.Val)
		}
	}
	return ""
}

func squashWhitespace(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func truncateText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if value == "" || limit <= 0 {
		return value
	}
	if len(value) <= limit {
		return value
	}
	if limit <= 1 {
		return value[:limit]
	}
	return strings.TrimSpace(value[:limit-1]) + "…"
}

func fallbackTitle(rawURL, summary string) string {
	if parsed, err := url.Parse(rawURL); err == nil {
		if base := strings.TrimSpace(path.Base(parsed.Path)); base != "" && base != "/" && base != "." {
			return base
		}
		if host := strings.TrimSpace(parsed.Hostname()); host != "" {
			return host
		}
	}
	if summary != "" {
		return truncateText(summary, 80)
	}
	return "Untitled"
}

func optionalLabel(label, fallback string) string {
	label = strings.TrimSpace(label)
	if label != "" {
		return label
	}
	return fallback
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
