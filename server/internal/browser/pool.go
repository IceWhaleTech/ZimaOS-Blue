package browser

import (
	"context"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// Pool manages a pool of browser instances.
type Pool struct {
	config    *Config
	launcher  *launcher.Launcher
	browsers  []*browserInstance
	available chan *browserInstance
	mu        sync.RWMutex
	closed    bool
	startTime time.Time
}

// browserInstance represents a single browser instance in the pool.
type browserInstance struct {
	browser   *rod.Browser
	inUse     bool
	createdAt time.Time
	lastUsed  time.Time
}

// NewPool creates a new browser pool.
func NewPool(config *Config) (*Pool, error) {
	if config == nil {
		config = DefaultConfig()
	}

	p := &Pool{
		config:    config,
		available: make(chan *browserInstance, config.PoolSize),
		browsers:  make([]*browserInstance, 0, config.PoolSize),
		startTime: time.Now(),
	}

	return p, nil
}

// Start initializes the browser pool.
func (p *Pool) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return ErrBrowserNotRunning
	}

	// Create launcher
	l := launcher.New()
	if p.config.Headless {
		l = l.Headless(true)
	}
	if p.config.BrowserPath != "" {
		l = l.Bin(p.config.BrowserPath)
	}
	if p.config.ProxyURL != "" {
		l = l.Proxy(p.config.ProxyURL)
	}

	p.launcher = l

	// Pre-create browser instances
	for i := 0; i < p.config.PoolSize; i++ {
		instance, err := p.createInstance(ctx)
		if err != nil {
			// Clean up any created instances
			p.closeAllInstances()
			return err
		}
		p.browsers = append(p.browsers, instance)
		p.available <- instance
	}

	return nil
}

// createInstance creates a new browser instance.
func (p *Pool) createInstance(ctx context.Context) (*browserInstance, error) {
	url, err := p.launcher.Launch()
	if err != nil {
		return nil, err
	}

	browser := rod.New().ControlURL(url)
	if err := browser.Connect(); err != nil {
		return nil, err
	}

	// Set default viewport
	if p.config.DefaultViewportWidth > 0 && p.config.DefaultViewportHeight > 0 {
		// Viewport is set per page, not per browser
	}

	return &browserInstance{
		browser:   browser,
		createdAt: time.Now(),
		lastUsed:  time.Now(),
	}, nil
}

// Acquire gets a browser instance from the pool.
func (p *Pool) Acquire(ctx context.Context) (*rod.Browser, error) {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return nil, ErrBrowserNotRunning
	}
	p.mu.RUnlock()

	select {
	case instance := <-p.available:
		p.mu.Lock()
		instance.inUse = true
		instance.lastUsed = time.Now()
		p.mu.Unlock()
		return instance.browser, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Release returns a browser instance to the pool.
func (p *Pool) Release(browser *rod.Browser) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	for _, instance := range p.browsers {
		if instance.browser == browser {
			instance.inUse = false
			instance.lastUsed = time.Now()
			// Clean up pages
			pages, _ := browser.Pages()
			for _, page := range pages {
				if page != nil {
					_ = page.Close()
				}
			}
			select {
			case p.available <- instance:
			default:
				// Pool is full, shouldn't happen
			}
			return
		}
	}
}

// Status returns the pool status.
func (p *Pool) Status() *StatusResponse {
	p.mu.RLock()
	defer p.mu.RUnlock()

	tabCount := 0
	for _, instance := range p.browsers {
		if instance.browser != nil {
			pages, _ := instance.browser.Pages()
			tabCount += len(pages)
		}
	}

	return &StatusResponse{
		Running:  !p.closed && len(p.browsers) > 0,
		TabCount: tabCount,
		Uptime:   int64(time.Since(p.startTime).Seconds()),
	}
}

// closeAllInstances closes all browser instances.
func (p *Pool) closeAllInstances() {
	for _, instance := range p.browsers {
		if instance.browser != nil {
			_ = instance.browser.Close()
		}
	}
	p.browsers = nil
}

// Close shuts down the browser pool.
func (p *Pool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true
	close(p.available)
	p.closeAllInstances()

	if p.launcher != nil {
		p.launcher.Cleanup()
	}

	return nil
}

// NewPage creates a new page in a browser instance.
func (p *Pool) NewPage(ctx context.Context) (*rod.Page, *rod.Browser, error) {
	browser, err := p.Acquire(ctx)
	if err != nil {
		return nil, nil, err
	}

	page, err := browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		p.Release(browser)
		return nil, nil, err
	}

	// Set viewport
	if p.config.DefaultViewportWidth > 0 && p.config.DefaultViewportHeight > 0 {
		err = page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
			Width:  p.config.DefaultViewportWidth,
			Height: p.config.DefaultViewportHeight,
		})
		if err != nil {
			_ = page.Close()
			p.Release(browser)
			return nil, nil, err
		}
	}

	// Set user agent if configured
	if p.config.UserAgent != "" {
		err = page.SetUserAgent(&proto.NetworkSetUserAgentOverride{
			UserAgent: p.config.UserAgent,
		})
		if err != nil {
			_ = page.Close()
			p.Release(browser)
			return nil, nil, err
		}
	}

	return page, browser, nil
}

// ReleasePage closes a page and releases the browser back to the pool.
func (p *Pool) ReleasePage(page *rod.Page, browser *rod.Browser) {
	if page != nil {
		_ = page.Close()
	}
	if browser != nil {
		p.Release(browser)
	}
}
