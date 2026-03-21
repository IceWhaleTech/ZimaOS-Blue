package browser

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/cdp"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// Pool manages a pool of browser instances.
type Pool struct {
	config    *Config
	browsers  []*browserInstance
	available chan *browserInstance
	mu        sync.RWMutex
	closed    bool
	startTime time.Time
}

// browserInstance represents a single browser instance in the pool.
type browserInstance struct {
	browser         *rod.Browser
	launcher        *launcher.Launcher // each instance owns its launcher for cleanup
	transportCloser io.Closer
	managed         bool
	inUse           bool
	createdAt       time.Time
	lastUsed        time.Time
}

// NewPool creates a new browser pool.
func NewPool(config *Config) (*Pool, error) {
	if config == nil {
		config = DefaultConfig()
	}

	p := &Pool{
		config:    config,
		available: make(chan *browserInstance, config.EffectivePoolSize()),
		browsers:  make([]*browserInstance, 0, config.EffectivePoolSize()),
		startTime: timeutil.NowTime(),
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

	// Pre-create browser instances
	for i := 0; i < p.config.EffectivePoolSize(); i++ {
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

// newLauncher creates a configured launcher from pool config.
func (p *Pool) newLauncher() *launcher.Launcher {
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
	return l
}

// createInstance creates a new browser instance with its own launcher.
func (p *Pool) createInstance(ctx context.Context) (*browserInstance, error) {
	if p.config.UsesRelayDriver() {
		cdpURL := p.config.EffectiveCDPURL()
		if cdpURL == "" {
			return nil, fmt.Errorf("browser cdp_url or built-in relay configuration is required when driver=relay")
		}

		wsURL, err := resolveCDPWebSocketURL(ctx, cdpURL)
		if err != nil {
			return nil, err
		}

		ws := &cdp.WebSocket{}
		if err := ws.Connect(ctx, wsURL, nil); err != nil {
			return nil, err
		}

		client := cdp.New().Start(ws)
		browser := rod.New().Client(client)
		if err := browser.Connect(); err != nil {
			_ = ws.Close()
			return nil, err
		}

		return &browserInstance{
			browser:         browser,
			transportCloser: ws,
			managed:         false,
			createdAt:       timeutil.NowTime(),
			lastUsed:        timeutil.NowTime(),
		}, nil
	}

	l := p.newLauncher()

	url, err := l.Launch()
	if err != nil {
		l.Cleanup()
		return nil, err
	}

	browser := rod.New().ControlURL(url)
	if err := browser.Connect(); err != nil {
		l.Cleanup()
		return nil, err
	}

	return &browserInstance{
		browser:   browser,
		launcher:  l,
		managed:   true,
		createdAt: timeutil.NowTime(),
		lastUsed:  timeutil.NowTime(),
	}, nil
}

// Acquire gets a browser instance from the pool.
func (p *Pool) Acquire(ctx context.Context) (*rod.Browser, error) {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return nil, ErrBrowserNotRunning
	}
	if p.config.UsesRelayDriver() {
		if len(p.browsers) == 0 || p.browsers[0] == nil || p.browsers[0].browser == nil {
			p.mu.RUnlock()
			return nil, ErrBrowserNotRunning
		}
		browser := p.browsers[0].browser
		p.mu.RUnlock()
		return browser, nil
	}
	p.mu.RUnlock()

	select {
	case instance := <-p.available:
		p.mu.Lock()
		instance.inUse = true
		instance.lastUsed = timeutil.NowTime()
		p.mu.Unlock()
		return instance.browser, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Release returns a browser instance to the pool.
func (p *Pool) Release(browser *rod.Browser) {
	if p.config.UsesRelayDriver() {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	for _, instance := range p.browsers {
		if instance.browser == browser {
			instance.inUse = false
			instance.lastUsed = timeutil.NowTime()
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

// closeAllInstances closes all browser instances and their launchers.
func (p *Pool) closeAllInstances() {
	for _, instance := range p.browsers {
		if instance.managed && instance.browser != nil {
			_ = instance.browser.Close()
		}
		if instance.transportCloser != nil {
			_ = instance.transportCloser.Close()
		}
		if instance.launcher != nil {
			instance.launcher.Cleanup()
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

	return nil
}

// NewPage creates a new page in a browser instance.
// If the acquired browser is dead (Chrome crashed), it replaces the instance and retries once.
func (p *Pool) NewPage(ctx context.Context) (*rod.Page, *rod.Browser, error) {
	browser, err := p.Acquire(ctx)
	if err != nil {
		return nil, nil, err
	}

	page, err := browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		if isConnectionClosed(err) {
			// Browser is dead — replace the instance and retry
			newBrowser, replaceErr := p.replaceInstance(ctx, browser)
			if replaceErr != nil {
				return nil, nil, fmt.Errorf("browser died and replacement failed: %w", replaceErr)
			}
			page, err = newBrowser.Page(proto.TargetCreateTarget{URL: "about:blank"})
			if err != nil {
				p.Release(newBrowser)
				return nil, nil, err
			}
			browser = newBrowser
		} else {
			p.Release(browser)
			return nil, nil, err
		}
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

// replaceInstance replaces a dead browser instance with a fresh one.
// The old browser is closed and removed from the pool, and a new one is created.
func (p *Pool) replaceInstance(ctx context.Context, deadBrowser *rod.Browser) (*rod.Browser, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil, ErrBrowserNotRunning
	}

	// Find and replace the dead instance
	for i, instance := range p.browsers {
		if instance.browser == deadBrowser {
			if instance.managed && instance.browser != nil {
				_ = instance.browser.Close()
			}
			if instance.transportCloser != nil {
				_ = instance.transportCloser.Close()
			}
			if instance.launcher != nil {
				instance.launcher.Cleanup()
			}

			newInstance, err := p.createInstance(ctx)
			if err != nil {
				// Remove the dead slot entirely
				p.browsers = append(p.browsers[:i], p.browsers[i+1:]...)
				return nil, fmt.Errorf("failed to replace browser: %w", err)
			}
			newInstance.inUse = true
			newInstance.lastUsed = timeutil.NowTime()
			p.browsers[i] = newInstance
			return newInstance.browser, nil
		}
	}

	return nil, ErrBrowserNotAvailable
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
