package browser

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

// RodService implements the Service interface using Rod.
type RodService struct {
	config   *Config
	pool     *Pool
	security *SecurityChecker
	tabs     map[string]*tabInfo
	tabsMu   sync.RWMutex
	started  bool
	mu       sync.RWMutex
}

// tabInfo stores information about an open tab.
type tabInfo struct {
	page     *rod.Page
	browser  *rod.Browser
	targetID string
	url      string
	title    string
	active   bool
	console  []ConsoleMessage
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
		config:   config,
		pool:     pool,
		security: NewSecurityChecker(config),
		tabs:     make(map[string]*tabInfo),
	}, nil
}

// isConnectionClosed checks if an error indicates a dead WebSocket/TCP connection.
func isConnectionClosed(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "use of closed network connection") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "websocket: close") ||
		strings.Contains(msg, "EOF")
}

// removeTab cleans up a stale tab entry.
func (s *RodService) removeTab(tab *tabInfo) {
	s.tabsMu.Lock()
	delete(s.tabs, tab.targetID)
	s.tabsMu.Unlock()
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
		if tab.page != nil {
			_ = tab.page.Close()
		}
	}
	s.tabs = make(map[string]*tabInfo)
	s.tabsMu.Unlock()

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

// OpenTab opens a new tab with the given URL.
func (s *RodService) OpenTab(ctx context.Context, url string) (*Tab, error) {
	if err := s.security.CheckURL(url); err != nil {
		return nil, err
	}

	page, browser, err := s.pool.NewPage(ctx)
	if err != nil {
		return nil, err
	}

	// Navigate to URL
	timeout := GetTimeout(0, s.config)
	err = page.Timeout(timeout).Navigate(url)
	if err != nil {
		s.pool.ReleasePage(page, browser)
		return nil, err
	}

	// Wait for page to load
	err = page.WaitLoad()
	if err != nil {
		s.pool.ReleasePage(page, browser)
		return nil, err
	}

	// Get page info
	info, err := page.Info()
	if err != nil {
		s.pool.ReleasePage(page, browser)
		return nil, err
	}

	targetID := fmt.Sprintf("tab-%d", time.Now().UnixNano())

	s.tabsMu.Lock()
	// Deactivate other tabs
	for _, t := range s.tabs {
		t.active = false
	}
	s.tabs[targetID] = &tabInfo{
		page:     page,
		browser:  browser,
		targetID: targetID,
		url:      info.URL,
		title:    info.Title,
		active:   true,
		console:  make([]ConsoleMessage, 0),
	}
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
	if tab.browser != nil {
		s.pool.Release(tab.browser)
	}
	delete(s.tabs, targetID)

	return nil
}

// Navigate navigates to a URL in the current or specified tab.
func (s *RodService) Navigate(ctx context.Context, req *NavigateRequest) (*NavigateResponse, error) {
	if err := s.security.CheckURL(req.URL); err != nil {
		return nil, err
	}

	s.tabsMu.Lock()
	var tab *tabInfo
	if req.TargetID != "" {
		var ok bool
		tab, ok = s.tabs[req.TargetID]
		if !ok {
			s.tabsMu.Unlock()
			return nil, ErrTabNotFound
		}
	} else {
		// Find active tab
		for _, t := range s.tabs {
			if t.active {
				tab = t
				break
			}
		}
	}
	s.tabsMu.Unlock()

	if tab == nil {
		// Open new tab
		newTab, err := s.OpenTab(ctx, req.URL)
		if err != nil {
			return nil, err
		}
		return &NavigateResponse{
			URL:      newTab.URL,
			Title:    newTab.Title,
			TargetID: newTab.TargetID,
		}, nil
	}

	timeout := GetTimeout(req.Timeout, s.config)
	err := tab.page.Timeout(timeout).Navigate(req.URL)
	if err != nil {
		// Connection may be dead (Chrome crashed, WebSocket closed).
		// Clean up the stale tab and retry with a fresh one.
		if isConnectionClosed(err) {
			s.removeTab(tab)
			newTab, retryErr := s.OpenTab(ctx, req.URL)
			if retryErr != nil {
				return nil, retryErr
			}
			return &NavigateResponse{
				URL:      newTab.URL,
				Title:    newTab.Title,
				TargetID: newTab.TargetID,
			}, nil
		}
		return nil, err
	}

	// Wait based on WaitUntil
	switch req.WaitUntil {
	case "networkidle":
		err = tab.page.WaitIdle(timeout)
	case "domcontentloaded":
		err = tab.page.WaitDOMStable(timeout, 0.5)
	default: // "load" or empty
		err = tab.page.WaitLoad()
	}
	if err != nil {
		return nil, err
	}

	info, err := tab.page.Info()
	if err != nil {
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
	if err := s.security.CheckURL(req.URL); err != nil {
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
	err = page.Timeout(timeout).Navigate(req.URL)
	if err != nil {
		return nil, err
	}

	err = page.WaitLoad()
	if err != nil {
		return nil, err
	}

	// Wait for selector if specified
	if req.WaitForSelector != nil && *req.WaitForSelector != "" {
		_, err = page.Timeout(timeout).Element(*req.WaitForSelector)
		if err != nil {
			return nil, err
		}
	}

	// Additional wait time
	if req.WaitFor > 0 {
		time.Sleep(time.Duration(req.WaitFor) * time.Millisecond)
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

	info, _ := page.Info()

	return &ScreenshotResponse{
		Data:   base64.StdEncoding.EncodeToString(data),
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
	if err := s.security.CheckURL(req.URL); err != nil {
		return nil, err
	}

	page, browser, err := s.pool.NewPage(ctx)
	if err != nil {
		return nil, err
	}
	defer s.pool.ReleasePage(page, browser)

	timeout := GetTimeout(req.Timeout, s.config)
	err = page.Timeout(timeout).Navigate(req.URL)
	if err != nil {
		return nil, err
	}

	err = page.WaitLoad()
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

	info, _ := page.Info()

	return &PDFResponse{
		Data:  base64.StdEncoding.EncodeToString(data),
		URL:   info.URL,
		Title: info.Title,
	}, nil
}

// Snapshot returns a structured snapshot of the page.
func (s *RodService) Snapshot(ctx context.Context, req *SnapshotRequest) (*SnapshotResponse, error) {
	s.tabsMu.RLock()
	var tab *tabInfo
	if req.TargetID != "" {
		tab = s.tabs[req.TargetID]
	} else {
		for _, t := range s.tabs {
			if t.active {
				tab = t
				break
			}
		}
	}
	s.tabsMu.RUnlock()

	if tab == nil || tab.page == nil {
		return nil, ErrTabNotFound
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

	info, _ := tab.page.Info()

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
	if err := s.security.CheckURL(req.URL); err != nil {
		return nil, err
	}

	page, browser, err := s.pool.NewPage(ctx)
	if err != nil {
		return nil, err
	}
	defer s.pool.ReleasePage(page, browser)

	timeout := GetTimeout(req.Timeout, s.config)
	err = page.Timeout(timeout).Navigate(req.URL)
	if err != nil {
		return nil, err
	}

	err = page.WaitLoad()
	if err != nil {
		return nil, err
	}

	// Wait for selector if specified
	if req.WaitForSelector != nil && *req.WaitForSelector != "" {
		_, err = page.Timeout(timeout).Element(*req.WaitForSelector)
		if err != nil {
			return nil, err
		}
	}

	// Additional wait time
	if req.WaitFor > 0 {
		time.Sleep(time.Duration(req.WaitFor) * time.Millisecond)
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

	info, _ := page.Info()

	return &ScrapeResponse{
		Data:  data,
		URL:   info.URL,
		Title: info.Title,
	}, nil
}

// Act performs an action on the page.
func (s *RodService) Act(ctx context.Context, req *ActRequest) (*ActResponse, error) {
	s.tabsMu.RLock()
	var tab *tabInfo
	if req.TargetID != "" {
		tab = s.tabs[req.TargetID]
	} else {
		for _, t := range s.tabs {
			if t.active {
				tab = t
				break
			}
		}
	}
	s.tabsMu.RUnlock()

	if tab == nil || tab.page == nil {
		return nil, ErrTabNotFound
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
	if err := s.security.CheckURL(req.URL); err != nil {
		return nil, err
	}

	page, browser, err := s.pool.NewPage(ctx)
	if err != nil {
		return nil, err
	}
	defer s.pool.ReleasePage(page, browser)

	timeout := GetTimeout(req.Timeout, s.config)
	err = page.Timeout(timeout).Navigate(req.URL)
	if err != nil {
		return nil, err
	}

	err = page.WaitLoad()
	if err != nil {
		return nil, err
	}

	results := make([]StepResult, 0, len(req.Steps))
	success := true

	for i, step := range req.Steps {
		start := time.Now()
		result := StepResult{
			Step:   i,
			Action: step.Action,
		}

		stepErr := s.executeStep(ctx, page, &step)
		result.Duration = time.Since(start).Milliseconds()

		if stepErr != nil {
			result.Success = false
			result.Error = stepErr.Error()
			if !step.Optional {
				success = false
				results = append(results, result)
				break
			}
		} else {
			result.Success = true
		}

		results = append(results, result)

		// Wait after step if specified
		if step.WaitFor > 0 {
			time.Sleep(time.Duration(step.WaitFor) * time.Millisecond)
		}
	}

	info, _ := page.Info()

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
		if step.WaitFor > 0 {
			time.Sleep(time.Duration(step.WaitFor) * time.Millisecond)
		}
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
		if err := s.security.CheckURL(step.Value); err != nil {
			return err
		}
		return page.Navigate(step.Value)

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

	default:
		return ErrInvalidAction
	}
}

// Console returns console messages from the page.
func (s *RodService) Console(ctx context.Context, req *ConsoleRequest) (*ConsoleResponse, error) {
	s.tabsMu.RLock()
	var tab *tabInfo
	if req.TargetID != "" {
		tab = s.tabs[req.TargetID]
	} else {
		for _, t := range s.tabs {
			if t.active {
				tab = t
				break
			}
		}
	}
	s.tabsMu.RUnlock()

	if tab == nil {
		return nil, ErrTabNotFound
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
func (s *RodService) getTab(targetID string) (*tabInfo, error) {
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

// AccessibilityTree returns a compact DSL representation of the page's accessibility tree.
// This is much more token-efficient than raw HTML for LLM consumption.
// Each interactive element gets an @ref that can be used in Act() to target it.
func (s *RodService) AccessibilityTree(ctx context.Context, targetID string, maxDepth int) (*AccessibilityTreeResponse, error) {
	tab, err := s.getTab(targetID)
	if err != nil {
		return nil, err
	}

	depth := maxDepth
	if depth <= 0 {
		depth = 10
	}

	result, err := proto.AccessibilityGetFullAXTree{Depth: &depth}.Call(tab.page)
	if err != nil {
		if isConnectionClosed(err) {
			s.removeTab(tab)
			return nil, fmt.Errorf("browser connection lost (tab removed): %w", err)
		}
		return nil, fmt.Errorf("failed to get accessibility tree: %w", err)
	}

	// Build DSL with @ref references
	builder := newAXTreeBuilder(result.Nodes)
	tree := builder.build()

	info, _ := tab.page.Info()

	return &AccessibilityTreeResponse{
		Tree:     tree,
		URL:      info.URL,
		Title:    info.Title,
		TargetID: tab.targetID,
		RefMap:   builder.refMap,
	}, nil
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
	"link":          true,
	"button":        true,
	"textbox":       true,
	"searchbox":     true,
	"combobox":      true,
	"checkbox":      true,
	"radio":         true,
	"switch":        true,
	"slider":        true,
	"spinbutton":    true,
	"tab":           true,
	"menuitem":      true,
	"menuitemcheckbox": true,
	"menuitemradio": true,
	"option":        true,
	"treeitem":      true,
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

	tab, err := s.getTab(targetID)
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

// CountInteractiveElements returns the count of interactive elements on the page.
// This is a lightweight JS call — no DSL building, no ref map.
func (s *RodService) CountInteractiveElements(ctx context.Context, targetID string) (int, error) {
	tab, err := s.getTab(targetID)
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
	tab, err := s.getTab(targetID)
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

	return base64.StdEncoding.EncodeToString(data), nil
}

// ScreenshotViewport takes a viewport-only screenshot (no full-page scroll capture).
func (s *RodService) ScreenshotViewport(ctx context.Context, targetID string) (string, error) {
	data, err := s.ScreenshotViewportRaw(ctx, targetID)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// ScreenshotViewportRaw takes a viewport-only screenshot and returns raw PNG bytes.
func (s *RodService) ScreenshotViewportRaw(ctx context.Context, targetID string) ([]byte, error) {
	tab, err := s.getTab(targetID)
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
	tab, err := s.getTab(targetID)
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
	tab, err := s.getTab(targetID)
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
	tab, err := s.getTab(targetID)
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
	tab, err := s.getTab(targetID)
	if err != nil {
		return nil, err
	}

	page := tab.page

	// Wait for page to be stable before extracting
	_ = page.WaitStable(500 * time.Millisecond)

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

	tab, err := s.getTab(targetID)
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
