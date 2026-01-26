package browser

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
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

// Stop stops the browser service.
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
		_ = tab.page.Close()
	}
	s.pool.Release(tab.browser)
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
