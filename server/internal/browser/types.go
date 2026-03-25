// Package browser provides browser automation capabilities using Rod.
// This module is designed to be compatible with clawdbot's browser tool interface.
package browser

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

var (
	// ErrBrowserNotAvailable is returned when no browser instance is available.
	ErrBrowserNotAvailable = errors.New("browser not available")
	// ErrBrowserNotRunning is returned when the browser is not running.
	ErrBrowserNotRunning = errors.New("browser not running")
	// ErrPageNotFound is returned when a page is not found.
	ErrPageNotFound = errors.New("page not found")
	// ErrTabNotFound is returned when a tab is not found.
	ErrTabNotFound = errors.New("tab not found")
	// ErrTimeout is returned when an operation times out.
	ErrTimeout = errors.New("operation timed out")
	// ErrURLNotAllowed is returned when a URL is not in the allowlist.
	ErrURLNotAllowed = errors.New("URL not allowed")
	// ErrURLBlocked is returned when a URL is in the blocklist.
	ErrURLBlocked = errors.New("URL blocked")
	// ErrResourceLimitExceeded is returned when resource limits are exceeded.
	ErrResourceLimitExceeded = errors.New("resource limit exceeded")
	// ErrInvalidSelector is returned when a CSS selector is invalid.
	ErrInvalidSelector = errors.New("invalid selector")
	// ErrElementNotFound is returned when an element is not found.
	ErrElementNotFound = errors.New("element not found")
	// ErrInvalidAction is returned when an action is invalid.
	ErrInvalidAction = errors.New("invalid action")
)

// ScreenshotFormat represents the image format for screenshots.
type ScreenshotFormat string

const (
	// FormatPNG is PNG format.
	FormatPNG ScreenshotFormat = "png"
	// FormatJPEG is JPEG format.
	FormatJPEG ScreenshotFormat = "jpeg"
	// FormatWebP is WebP format.
	FormatWebP ScreenshotFormat = "webp"
)

// PDFFormat represents the paper format for PDF generation.
type PDFFormat string

const (
	// PDFFormatA4 is A4 paper size.
	PDFFormatA4 PDFFormat = "A4"
	// PDFFormatLetter is Letter paper size.
	PDFFormatLetter PDFFormat = "Letter"
	// PDFFormatLegal is Legal paper size.
	PDFFormatLegal PDFFormat = "Legal"
)

const (
	// DefaultRelayHost is the default loopback host for Blue's local browser relay.
	DefaultRelayHost = "127.0.0.1"
	// DefaultRelayPort is the default loopback port for Blue's local browser relay.
	DefaultRelayPort = 18792
)

// ScreenshotRequest represents a request to capture a screenshot.
type ScreenshotRequest struct {
	// URL is the page URL to capture.
	URL string `json:"url" validate:"required,url"`
	// Selector is an optional CSS selector to capture a specific element.
	Selector *string `json:"selector,omitempty"`
	// Format is the image format (png, jpeg, webp).
	Format ScreenshotFormat `json:"format,omitempty"`
	// Quality is the image quality (1-100, only for jpeg/webp).
	Quality int `json:"quality,omitempty"`
	// FullPage captures the full scrollable page if true.
	FullPage bool `json:"full_page,omitempty"`
	// Width is the viewport width.
	Width int `json:"width,omitempty"`
	// Height is the viewport height.
	Height int `json:"height,omitempty"`
	// WaitFor is the time to wait after page load (milliseconds).
	WaitFor int `json:"wait_for,omitempty"`
	// WaitForSelector waits for a specific element to appear.
	WaitForSelector *string `json:"wait_for_selector,omitempty"`
	// Timeout is the maximum time to wait (milliseconds).
	Timeout int `json:"timeout,omitempty"`
}

// ScreenshotResponse represents the result of a screenshot capture.
type ScreenshotResponse struct {
	// Data is the base64-encoded image data.
	Data string `json:"data"`
	// Format is the image format.
	Format ScreenshotFormat `json:"format"`
	// Width is the image width.
	Width int `json:"width"`
	// Height is the image height.
	Height int `json:"height"`
	// URL is the captured URL.
	URL string `json:"url"`
	// Title is the page title.
	Title string `json:"title"`
}

// SessionScreenshot stores a captured session frame for monitor playback.
type SessionScreenshot struct {
	// Data is the base64-encoded PNG payload.
	Data string `json:"data"`
	// URL is the tab URL at capture time.
	URL string `json:"url,omitempty"`
	// Title is the tab title at capture time.
	Title string `json:"title,omitempty"`
	// CapturedAt is the RFC3339 timestamp when the frame was stored.
	CapturedAt string `json:"captured_at"`
	// Scope describes the capture mode (for example viewport or full_page).
	Scope string `json:"scope,omitempty"`
}

// SessionEngine identifies which browser engine owns a session.
type SessionEngine string

const (
	// SessionEngineLightpanda is the read-only Lightpanda engine.
	SessionEngineLightpanda SessionEngine = "lightpanda"
	// SessionEngineChromiumManaged is Blue-managed Chromium.
	SessionEngineChromiumManaged SessionEngine = "chromium_managed"
	// SessionEngineChromiumRelay is a relay/local-Chrome Chromium session.
	SessionEngineChromiumRelay SessionEngine = "chromium_relay"
)

// SessionMonitorKind identifies how a session should be monitored.
type SessionMonitorKind string

const (
	// SessionMonitorKindText represents structured text monitoring.
	SessionMonitorKindText SessionMonitorKind = "text"
	// SessionMonitorKindImage represents screenshot-based monitoring.
	SessionMonitorKindImage SessionMonitorKind = "image"
)

// SessionEngineDetail identifies the precise runtime behind a compatibility
// engine bucket.
type SessionEngineDetail string

const (
	// SessionEngineDetailLightpandaShim is Blue's read-layer Lightpanda shim.
	SessionEngineDetailLightpandaShim SessionEngineDetail = "lightpanda_shim"
	// SessionEngineDetailLightpandaBinary is the upstream Lightpanda binary runtime.
	SessionEngineDetailLightpandaBinary SessionEngineDetail = "lightpanda_binary"
	// SessionEngineDetailChromiumManaged is Blue-managed Chromium.
	SessionEngineDetailChromiumManaged SessionEngineDetail = "chromium_managed"
	// SessionEngineDetailChromiumRelay is relay/local-Chrome Chromium.
	SessionEngineDetailChromiumRelay SessionEngineDetail = "chromium_relay"
)

// SessionLayer identifies the capability layer that owns a session.
type SessionLayer string

const (
	// SessionLayerRead is the fetch/read layer.
	SessionLayerRead SessionLayer = "read"
	// SessionLayerBrowserLite is the browser-lite layer.
	SessionLayerBrowserLite SessionLayer = "browser_lite"
	// SessionLayerFullBrowser is the full browser layer.
	SessionLayerFullBrowser SessionLayer = "full_browser"
)

// SessionInfo is the unified browser session summary returned to clients.
type SessionInfo struct {
	ID           string              `json:"id"`
	Status       string              `json:"status"`
	CurrentURL   string              `json:"current_url,omitempty"`
	PageTitle    string              `json:"page_title,omitempty"`
	CreatedAt    string              `json:"created_at"`
	LastActivity string              `json:"last_activity"`
	Engine       SessionEngine       `json:"engine"`
	EngineDetail SessionEngineDetail `json:"engine_detail,omitempty"`
	SessionLayer SessionLayer        `json:"session_layer,omitempty"`
	MonitorKind  SessionMonitorKind  `json:"monitor_kind"`
}

// LightpandaReadDocument is the structured read result emitted by Blue's
// Lightpanda shim for fetch/read-layer use cases.
type LightpandaReadDocument struct {
	URL               string `json:"url,omitempty"`
	Title             string `json:"title,omitempty"`
	Content           string `json:"content,omitempty"`
	Summary           string `json:"summary,omitempty"`
	TreePreview       string `json:"tree_preview,omitempty"`
	AccessibilityTree string `json:"accessibility_tree,omitempty"`
	InteractiveTree   string `json:"interactive_tree,omitempty"`
	InteractiveCount  int    `json:"interactive_count,omitempty"`
}

// SessionTextMonitor contains the lightweight text monitor payload.
type SessionTextMonitor struct {
	Title            string `json:"title,omitempty"`
	URL              string `json:"url,omitempty"`
	Summary          string `json:"summary,omitempty"`
	TreePreview      string `json:"tree_preview,omitempty"`
	InteractiveCount int    `json:"interactive_count,omitempty"`
	UpdatedAt        string `json:"updated_at,omitempty"`
	Status           string `json:"status,omitempty"`
}

// SessionImageMonitor contains screenshot monitor payload for Chromium sessions.
type SessionImageMonitor struct {
	Screenshot string              `json:"screenshot,omitempty"`
	History    []SessionScreenshot `json:"history,omitempty"`
	UpdatedAt  string              `json:"updated_at,omitempty"`
	Status     string              `json:"status,omitempty"`
}

// SessionMonitorResponse is the unified monitor API payload.
type SessionMonitorResponse struct {
	Kind  SessionMonitorKind   `json:"kind"`
	Image *SessionImageMonitor `json:"image,omitempty"`
	Text  *SessionTextMonitor  `json:"text,omitempty"`
	Error string               `json:"error,omitempty"`
}

// SessionScreenshotResponse is the legacy screenshot compatibility payload.
type SessionScreenshotResponse struct {
	Screenshot string              `json:"screenshot,omitempty"`
	History    []SessionScreenshot `json:"history,omitempty"`
	Error      string              `json:"error,omitempty"`
}

// SessionRouteProvider allows browser session HTTP routes to be backed by a
// multi-engine session manager instead of a single browser.Service instance.
type SessionRouteProvider interface {
	ListBrowserSessions(ctx context.Context) ([]SessionInfo, error)
	CreateBrowserSession(ctx context.Context) (*SessionInfo, error)
	GetBrowserSession(ctx context.Context, id string) (*SessionInfo, error)
	CloseBrowserSession(ctx context.Context, id string) error
	NavigateBrowserSession(ctx context.Context, id string, url string) (*NavigateResponse, error)
	CaptureBrowserSessionMonitor(ctx context.Context, id string) (*SessionMonitorResponse, error)
	CaptureBrowserSessionScreenshot(ctx context.Context, id string) (*SessionScreenshotResponse, error)
}

// BrowserStrategy selects how Blue routes browser work.
type BrowserStrategy string

const (
	// BrowserStrategySingle keeps Blue on a single browser runtime.
	BrowserStrategySingle BrowserStrategy = "single"
	// BrowserStrategyHybridCapability routes by capability between engines.
	BrowserStrategyHybridCapability BrowserStrategy = "hybrid_capability"
)

// LightpandaConfig configures the read-only Lightpanda runtime.
type LightpandaConfig struct {
	Enabled    bool     `json:"enabled" yaml:"enabled"`
	BinaryPath string   `json:"binary_path" yaml:"binary_path"`
	Args       []string `json:"args" yaml:"args"`
}

// ChromiumConfig configures Chromium-specific routing preferences.
type ChromiumConfig struct {
	PreferLocalChrome *bool `json:"prefer_local_chrome" yaml:"prefer_local_chrome"`
}

// CapabilityRouterConfig configures hybrid browser escalation behavior.
type CapabilityRouterConfig struct {
	EscalateOnFailure *bool `json:"escalate_on_failure" yaml:"escalate_on_failure"`
}

// PDFRequest represents a request to generate a PDF.
type PDFRequest struct {
	// URL is the page URL to convert.
	URL string `json:"url" validate:"required,url"`
	// Format is the paper format.
	Format PDFFormat `json:"format,omitempty"`
	// Landscape sets landscape orientation if true.
	Landscape bool `json:"landscape,omitempty"`
	// PrintBackground includes background graphics if true.
	PrintBackground bool `json:"print_background,omitempty"`
	// Scale is the scale factor (0.1-2.0).
	Scale float64 `json:"scale,omitempty"`
	// MarginTop is the top margin in inches.
	MarginTop float64 `json:"margin_top,omitempty"`
	// MarginBottom is the bottom margin in inches.
	MarginBottom float64 `json:"margin_bottom,omitempty"`
	// MarginLeft is the left margin in inches.
	MarginLeft float64 `json:"margin_left,omitempty"`
	// MarginRight is the right margin in inches.
	MarginRight float64 `json:"margin_right,omitempty"`
	// WaitFor is the time to wait after page load (milliseconds).
	WaitFor int `json:"wait_for,omitempty"`
	// Timeout is the maximum time to wait (milliseconds).
	Timeout int `json:"timeout,omitempty"`
}

// PDFResponse represents the result of PDF generation.
type PDFResponse struct {
	// Data is the base64-encoded PDF data.
	Data string `json:"data"`
	// URL is the source URL.
	URL string `json:"url"`
	// Title is the page title.
	Title string `json:"title"`
	// PageCount is the number of pages.
	PageCount int `json:"page_count"`
}

// ScrapeRequest represents a request to scrape data from a page.
type ScrapeRequest struct {
	// URL is the page URL to scrape.
	URL string `json:"url" validate:"required,url"`
	// Selectors is a map of name to CSS selector for data extraction.
	Selectors map[string]SelectorConfig `json:"selectors" validate:"required"`
	// WaitFor is the time to wait after page load (milliseconds).
	WaitFor int `json:"wait_for,omitempty"`
	// WaitForSelector waits for a specific element to appear.
	WaitForSelector *string `json:"wait_for_selector,omitempty"`
	// Timeout is the maximum time to wait (milliseconds).
	Timeout int `json:"timeout,omitempty"`
}

// SelectorConfig defines how to extract data from an element.
type SelectorConfig struct {
	// Selector is the CSS selector.
	Selector string `json:"selector" validate:"required"`
	// Attribute is the attribute to extract (empty for text content).
	Attribute string `json:"attribute,omitempty"`
	// Multiple extracts all matching elements if true.
	Multiple bool `json:"multiple,omitempty"`
}

// ScrapeResponse represents the result of data scraping.
type ScrapeResponse struct {
	// Data is the extracted data.
	Data map[string]interface{} `json:"data"`
	// URL is the scraped URL.
	URL string `json:"url"`
	// Title is the page title.
	Title string `json:"title"`
}

// AutomateRequest represents a request to run an automation task.
type AutomateRequest struct {
	// URL is the starting URL.
	URL string `json:"url" validate:"required,url"`
	// Steps is the list of automation steps.
	Steps []AutomationStep `json:"steps" validate:"required,min=1"`
	// Timeout is the maximum time for the entire task (milliseconds).
	Timeout int `json:"timeout,omitempty"`
}

// AutomationStep represents a single automation step.
type AutomationStep struct {
	// Action is the action type.
	Action ActionType `json:"action" validate:"required"`
	// Selector is the CSS selector for the target element.
	Selector string `json:"selector,omitempty"`
	// Value is the value for input actions.
	Value string `json:"value,omitempty"`
	// WaitFor is the time to wait after this step (milliseconds).
	WaitFor int `json:"wait_for,omitempty"`
	// Optional marks this step as optional (won't fail if element not found).
	Optional bool `json:"optional,omitempty"`
}

// ActionType represents the type of automation action.
type ActionType string

const (
	// ActionClick clicks an element.
	ActionClick ActionType = "click"
	// ActionTypeText types text into an element.
	ActionTypeText ActionType = "type"
	// ActionSelect selects an option from a dropdown.
	ActionSelect ActionType = "select"
	// ActionWait waits for a specified time.
	ActionWait ActionType = "wait"
	// ActionWaitFor waits for an element to appear.
	ActionWaitFor ActionType = "wait_for"
	// ActionScroll scrolls the page or element.
	ActionScroll ActionType = "scroll"
	// ActionScreenshot takes a screenshot.
	ActionScreenshot ActionType = "screenshot"
	// ActionNavigate navigates to a URL.
	ActionNavigate ActionType = "navigate"
	// ActionEval evaluates JavaScript.
	ActionEval ActionType = "eval"
	// ActionHover hovers over an element.
	ActionHover ActionType = "hover"
	// ActionDrag drags from one element to another.
	ActionDrag ActionType = "drag"
	// ActionPress presses a key.
	ActionPress ActionType = "press"
	// ActionFill fills form fields.
	ActionFill ActionType = "fill"
	// ActionUpload uploads one or more local files to a file input.
	ActionUpload ActionType = "upload"
)

// AutomateResponse represents the result of an automation task.
type AutomateResponse struct {
	// Success indicates if all steps completed successfully.
	Success bool `json:"success"`
	// StepsCompleted is the number of steps completed.
	StepsCompleted int `json:"steps_completed"`
	// Results contains results from each step.
	Results []StepResult `json:"results"`
	// FinalURL is the URL after all steps.
	FinalURL string `json:"final_url"`
	// FinalTitle is the page title after all steps.
	FinalTitle string `json:"final_title"`
}

// StepResult represents the result of a single automation step.
type StepResult struct {
	// Step is the step index.
	Step int `json:"step"`
	// Action is the action type.
	Action ActionType `json:"action"`
	// Success indicates if the step succeeded.
	Success bool `json:"success"`
	// Error is the error message if failed.
	Error string `json:"error,omitempty"`
	// Data contains any data returned by the step.
	Data interface{} `json:"data,omitempty"`
	// Duration is the step duration in milliseconds.
	Duration int64 `json:"duration"`
}

// Config contains browser service configuration.
type Config struct {
	// Strategy selects the runtime routing strategy.
	Strategy BrowserStrategy `json:"strategy" yaml:"strategy"`
	// Driver selects how Blue connects to the browser.
	// Supported values: "managed" (default), "relay", "cdp".
	Driver string `json:"driver" yaml:"driver"`
	// RelayEnabled starts Blue's built-in loopback relay server and lets Blue attach
	// to tabs from the companion Chrome extension.
	RelayEnabled bool `json:"relay_enabled" yaml:"relay_enabled"`
	// RelayHost is the loopback host for Blue's built-in relay server.
	RelayHost string `json:"relay_host" yaml:"relay_host"`
	// RelayPort is the loopback port for Blue's built-in relay server.
	RelayPort int `json:"relay_port" yaml:"relay_port"`
	// RelayToken authenticates CDP and extension clients against Blue's relay server.
	RelayToken string `json:"relay_token" yaml:"relay_token"`
	// PoolSize is the maximum number of browser instances.
	PoolSize int `json:"pool_size" yaml:"pool_size"`
	// Headless controls whether browsers run without a visible window.
	Headless bool `json:"headless" yaml:"headless"`
	// DefaultTimeout is the default timeout in milliseconds.
	DefaultTimeout int `json:"default_timeout" yaml:"default_timeout"`
	// MaxTimeout is the maximum allowed timeout in milliseconds.
	MaxTimeout int `json:"max_timeout" yaml:"max_timeout"`
	// DefaultViewportWidth is the default viewport width.
	DefaultViewportWidth int `json:"default_viewport_width" yaml:"default_viewport_width"`
	// DefaultViewportHeight is the default viewport height.
	DefaultViewportHeight int `json:"default_viewport_height" yaml:"default_viewport_height"`
	// AllowedDomains is a list of allowed domains (empty allows all).
	AllowedDomains []string `json:"allowed_domains" yaml:"allowed_domains"`
	// BlockedDomains is a list of blocked domains.
	BlockedDomains []string `json:"blocked_domains" yaml:"blocked_domains"`
	// UserAgent is the custom user agent string.
	UserAgent string `json:"user_agent" yaml:"user_agent"`
	// ProxyURL is the proxy server URL.
	ProxyURL string `json:"proxy_url" yaml:"proxy_url"`
	// BrowserPath is the path to the browser executable.
	BrowserPath string `json:"browser_path" yaml:"browser_path"`
	// CDPURL attaches Blue to an existing browser or relay via Chrome DevTools Protocol.
	// Examples:
	//   http://127.0.0.1:18792?token=...
	//   ws://127.0.0.1:9222/devtools/browser/<id>
	CDPURL string `json:"cdp_url" yaml:"cdp_url"`
	// TrustedSites seeds browser checkpoint auto-approval with exact origins or hosts.
	TrustedSites []string `json:"trusted_sites" yaml:"trusted_sites"`
	// TrustedSitePresets expands built-in trusted-site origin bundles.
	// Supported presets: "browser_common", "relay_focused".
	TrustedSitePresets []string `json:"trusted_site_presets" yaml:"trusted_site_presets"`
	// RelayPreferredSites lists hosts/origins that should prefer relay/local Chrome.
	RelayPreferredSites []string `json:"relay_preferred_sites" yaml:"relay_preferred_sites"`
	// RelayPreferredSitePresets expands built-in relay site bundles.
	// Supported presets: "browser_common", "relay_focused".
	RelayPreferredSitePresets []string `json:"relay_preferred_site_presets" yaml:"relay_preferred_site_presets"`
	// RelayPreferredFallbackDriver controls which driver Blue should retry with when
	// a relay-preferred site cannot be served by relay/local Chrome.
	// Supported values: "managed", "relay", "" (defaults to managed).
	RelayPreferredFallbackDriver string `json:"relay_preferred_fallback_driver" yaml:"relay_preferred_fallback_driver"`
	// EvaluateEnabled controls whether JavaScript evaluation is allowed.
	// When false, act:evaluate and wait --fn are disabled to prevent
	// prompt injection attacks from executing arbitrary JavaScript.
	// Default: true
	EvaluateEnabled *bool `json:"evaluate_enabled" yaml:"evaluate_enabled"`
	// NetworkObserveEnabled controls whether Blue records lightweight browser
	// network telemetry for retrieval fallback and adapter synthesis.
	NetworkObserveEnabled bool `json:"network_observe_enabled" yaml:"network_observe_enabled"`
	// SessionScreenshotRetention controls how long monitor frame history is kept
	// in memory. It is derived from companion session retention at runtime.
	SessionScreenshotRetention time.Duration `json:"-" yaml:"-"`
	// Lightpanda configures the lightweight read-only browser runtime.
	Lightpanda LightpandaConfig `json:"lightpanda" yaml:"lightpanda"`
	// Chromium configures Chromium routing preferences.
	Chromium ChromiumConfig `json:"chromium" yaml:"chromium"`
	// CapabilityRouter configures hybrid escalation behavior.
	CapabilityRouter CapabilityRouterConfig `json:"capability_router" yaml:"capability_router"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	evaluateEnabled := true
	preferLocalChrome := true
	escalateOnFailure := true
	return &Config{
		Strategy:                     BrowserStrategySingle,
		Driver:                       "managed",
		RelayEnabled:                 false,
		RelayHost:                    DefaultRelayHost,
		RelayPort:                    DefaultRelayPort,
		RelayToken:                   "",
		PoolSize:                     3,
		Headless:                     true,
		DefaultTimeout:               60000,
		MaxTimeout:                   300000,
		DefaultViewportWidth:         1920,
		DefaultViewportHeight:        1080,
		AllowedDomains:               []string{},
		BlockedDomains:               []string{},
		UserAgent:                    "",
		ProxyURL:                     "",
		BrowserPath:                  "",
		CDPURL:                       "",
		TrustedSites:                 []string{},
		TrustedSitePresets:           []string{},
		RelayPreferredSites:          []string{},
		RelayPreferredSitePresets:    []string{},
		RelayPreferredFallbackDriver: "managed",
		EvaluateEnabled:              &evaluateEnabled,
		NetworkObserveEnabled:        true,
		SessionScreenshotRetention:   30 * 24 * time.Hour,
		Lightpanda: LightpandaConfig{
			Enabled:    false,
			BinaryPath: "",
			Args:       nil,
		},
		Chromium: ChromiumConfig{
			PreferLocalChrome: &preferLocalChrome,
		},
		CapabilityRouter: CapabilityRouterConfig{
			EscalateOnFailure: &escalateOnFailure,
		},
	}
}

// Clone returns a deep copy of the browser configuration.
func (c *Config) Clone() *Config {
	if c == nil {
		return DefaultConfig()
	}
	cloned := *c
	cloned.AllowedDomains = append([]string(nil), c.AllowedDomains...)
	cloned.BlockedDomains = append([]string(nil), c.BlockedDomains...)
	cloned.TrustedSites = append([]string(nil), c.TrustedSites...)
	cloned.TrustedSitePresets = append([]string(nil), c.TrustedSitePresets...)
	cloned.RelayPreferredSites = append([]string(nil), c.RelayPreferredSites...)
	cloned.RelayPreferredSitePresets = append([]string(nil), c.RelayPreferredSitePresets...)
	cloned.Lightpanda.Args = append([]string(nil), c.Lightpanda.Args...)
	if c.EvaluateEnabled != nil {
		enabled := *c.EvaluateEnabled
		cloned.EvaluateEnabled = &enabled
	}
	if c.Chromium.PreferLocalChrome != nil {
		preferLocalChrome := *c.Chromium.PreferLocalChrome
		cloned.Chromium.PreferLocalChrome = &preferLocalChrome
	}
	if c.CapabilityRouter.EscalateOnFailure != nil {
		escalateOnFailure := *c.CapabilityRouter.EscalateOnFailure
		cloned.CapabilityRouter.EscalateOnFailure = &escalateOnFailure
	}
	return &cloned
}

// CloneForDriver returns a deep copy of the config forced to the requested driver.
func (c *Config) CloneForDriver(driver string) *Config {
	cloned := c.Clone()
	switch normalizeBrowserDriverName(driver) {
	case "managed":
		cloned.Driver = "managed"
		cloned.RelayEnabled = false
		cloned.CDPURL = ""
	case "relay":
		cloned.Driver = "relay"
	}
	return cloned
}

func normalizeBrowserDriverName(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "relay", "cdp":
		return "relay"
	case "", "managed":
		return "managed"
	default:
		return ""
	}
}

// ResolvedStrategy returns the normalized browser routing strategy.
func (c *Config) ResolvedStrategy() BrowserStrategy {
	if c == nil {
		return BrowserStrategySingle
	}
	switch BrowserStrategy(strings.ToLower(strings.TrimSpace(string(c.Strategy)))) {
	case BrowserStrategyHybridCapability:
		return BrowserStrategyHybridCapability
	default:
		return BrowserStrategySingle
	}
}

// PreferLocalChrome reports whether Chromium routes should try relay/local Chrome first.
func (c *Config) PreferLocalChrome() bool {
	if c == nil || c.Chromium.PreferLocalChrome == nil {
		return true
	}
	return *c.Chromium.PreferLocalChrome
}

// CapabilityEscalateOnFailure reports whether Lightpanda failures should be
// upgraded to Chromium once during new-session routing.
func (c *Config) CapabilityEscalateOnFailure() bool {
	if c == nil || c.CapabilityRouter.EscalateOnFailure == nil {
		return true
	}
	return *c.CapabilityRouter.EscalateOnFailure
}

// ExpandedTrustedSites returns the merged explicit trusted sites and preset origins.
func (c *Config) ExpandedTrustedSites() []string {
	if c == nil {
		return nil
	}
	return mergeStringLists(c.TrustedSites, ExpandTrustedSitePresets(c.TrustedSitePresets))
}

// ExpandedRelayPreferredSites returns the merged explicit relay-preferred site
// patterns and preset host bundles.
func (c *Config) ExpandedRelayPreferredSites() []string {
	if c == nil {
		return nil
	}
	return mergeStringLists(c.RelayPreferredSites, ExpandRelayPreferredSitePresets(c.RelayPreferredSitePresets))
}

// RelayPreferredFallback resolves the configured fallback driver for relay-preferred sites.
func (c *Config) RelayPreferredFallback() string {
	if c == nil {
		return "managed"
	}
	switch normalizeBrowserDriverName(c.RelayPreferredFallbackDriver) {
	case "relay":
		return "relay"
	case "managed":
		return "managed"
	default:
		return "managed"
	}
}

func mergeStringLists(primary, secondary []string) []string {
	if len(primary) == 0 && len(secondary) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(primary)+len(secondary))
	out := make([]string, 0, len(primary)+len(secondary))
	for _, raw := range append(append([]string(nil), primary...), secondary...) {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

// IsEvaluateEnabled returns whether JavaScript evaluation is enabled.
func (c *Config) IsEvaluateEnabled() bool {
	if c.EvaluateEnabled == nil {
		return true // Default to enabled for backwards compatibility
	}
	return *c.EvaluateEnabled
}

// RelayHostOrDefault returns the configured relay host or the loopback default.
func (c *Config) RelayHostOrDefault() string {
	host := strings.TrimSpace(c.RelayHost)
	if host == "" {
		return DefaultRelayHost
	}
	return host
}

// RelayPortOrDefault returns the configured relay port or the default port.
func (c *Config) RelayPortOrDefault() int {
	if c.RelayPort <= 0 || c.RelayPort > 65535 {
		return DefaultRelayPort
	}
	return c.RelayPort
}

// RelayBaseURL returns the built-in relay HTTP base URL.
func (c *Config) RelayBaseURL() string {
	return fmt.Sprintf("http://%s:%d", c.RelayHostOrDefault(), c.RelayPortOrDefault())
}

// RelayCDPURL returns the built-in relay HTTP endpoint Blue can resolve into a WS URL.
func (c *Config) RelayCDPURL() string {
	base := c.RelayBaseURL()
	token := strings.TrimSpace(c.RelayToken)
	if token == "" {
		return base
	}
	return base + "?token=" + url.QueryEscape(token)
}

// EffectiveCDPURL returns the external or built-in relay CDP endpoint Blue should use.
func (c *Config) EffectiveCDPURL() string {
	if trimmed := strings.TrimSpace(c.CDPURL); trimmed != "" {
		return trimmed
	}
	if c.RelayEnabled {
		return c.RelayCDPURL()
	}
	return ""
}

// ResolvedDriver returns the normalized browser driver name.
func (c *Config) ResolvedDriver() string {
	driver := strings.ToLower(strings.TrimSpace(c.Driver))
	switch driver {
	case "", "managed":
		if strings.TrimSpace(c.EffectiveCDPURL()) != "" {
			return "relay"
		}
		return "managed"
	case "relay", "cdp":
		return "relay"
	default:
		if strings.TrimSpace(c.EffectiveCDPURL()) != "" {
			return "relay"
		}
		return "managed"
	}
}

// UsesRelayDriver reports whether Blue should attach to an external CDP/relay endpoint.
func (c *Config) UsesRelayDriver() bool {
	return c.ResolvedDriver() == "relay"
}

// EffectivePoolSize returns the runtime pool size after driver-specific normalization.
func (c *Config) EffectivePoolSize() int {
	if c.UsesRelayDriver() {
		return 1
	}
	if c.PoolSize <= 0 {
		return 1
	}
	return c.PoolSize
}

// Service defines the browser automation service interface.
// This interface is designed to be compatible with clawdbot's browser tool.
type Service interface {
	// Status returns the browser status.
	Status(ctx context.Context) (*StatusResponse, error)
	// Start starts the browser.
	Start(ctx context.Context) error
	// Stop stops the browser.
	Stop(ctx context.Context) error

	// Tab management
	// Tabs returns all open tabs.
	Tabs(ctx context.Context) ([]*Tab, error)
	// OpenTab opens a new tab with the given URL.
	OpenTab(ctx context.Context, url string) (*Tab, error)
	// FocusTab focuses a tab by target ID.
	FocusTab(ctx context.Context, targetID string) error
	// CloseTab closes a tab by target ID.
	CloseTab(ctx context.Context, targetID string) error

	// Navigation
	// Navigate navigates to a URL in the current or specified tab.
	Navigate(ctx context.Context, req *NavigateRequest) (*NavigateResponse, error)

	// Content capture
	// Screenshot captures a screenshot of a page.
	Screenshot(ctx context.Context, req *ScreenshotRequest) (*ScreenshotResponse, error)
	// PDF generates a PDF from a page.
	PDF(ctx context.Context, req *PDFRequest) (*PDFResponse, error)
	// Snapshot returns a structured snapshot of the page (for AI interaction).
	Snapshot(ctx context.Context, req *SnapshotRequest) (*SnapshotResponse, error)

	// Data extraction
	// Scrape extracts data from a page.
	Scrape(ctx context.Context, req *ScrapeRequest) (*ScrapeResponse, error)

	// Actions
	// Act performs an action on the page (click, type, etc.).
	Act(ctx context.Context, req *ActRequest) (*ActResponse, error)
	// Automate runs a multi-step automation task.
	Automate(ctx context.Context, req *AutomateRequest) (*AutomateResponse, error)

	// Console
	// Console returns console messages from the page.
	Console(ctx context.Context, req *ConsoleRequest) (*ConsoleResponse, error)

	// Recipes
	// ExecuteRecipe runs a named recipe with the given params.
	ExecuteRecipe(ctx context.Context, req *RecipeRequest) (*RecipeResponse, error)
	// Recipes returns the recipe registry for listing available recipes.
	Recipes() *RecipeRegistry

	// Close closes the browser service and releases resources.
	Close() error
}

// StatusResponse represents the browser status.
type StatusResponse struct {
	// Running indicates if the browser is running.
	Running bool `json:"running"`
	// Version is the browser version.
	Version string `json:"version,omitempty"`
	// TabCount is the number of open tabs.
	TabCount int `json:"tab_count"`
	// ActiveTabID is the ID of the active tab.
	ActiveTabID string `json:"active_tab_id,omitempty"`
	// Uptime is the browser uptime in seconds.
	Uptime int64 `json:"uptime,omitempty"`
}

// Tab represents a browser tab.
type Tab struct {
	// TargetID is the unique identifier for the tab.
	TargetID string `json:"target_id"`
	// URL is the current URL.
	URL string `json:"url"`
	// Title is the page title.
	Title string `json:"title"`
	// Active indicates if this is the active tab.
	Active bool `json:"active"`
}

// NavigateRequest represents a navigation request.
type NavigateRequest struct {
	// URL is the URL to navigate to.
	URL string `json:"url" validate:"required,url"`
	// TargetID is the optional tab ID to navigate in.
	TargetID string `json:"target_id,omitempty"`
	// WaitUntil specifies when navigation is considered complete.
	// Options: "load", "domcontentloaded", "networkidle"
	WaitUntil string `json:"wait_until,omitempty"`
	// Timeout is the navigation timeout in milliseconds.
	Timeout int `json:"timeout,omitempty"`
}

// NavigateResponse represents the result of navigation.
type NavigateResponse struct {
	// URL is the final URL after navigation.
	URL string `json:"url"`
	// Title is the page title.
	Title string `json:"title"`
	// TargetID is the tab ID.
	TargetID string `json:"target_id"`
}

// ObservedNetworkEvent is a lightweight network capture entry from a tab.
type ObservedNetworkEvent struct {
	Method       string            `json:"method,omitempty"`
	URL          string            `json:"url,omitempty"`
	Status       int               `json:"status,omitempty"`
	ContentType  string            `json:"content_type,omitempty"`
	ResourceType string            `json:"resource_type,omitempty"`
	Initiator    string            `json:"initiator,omitempty"`
	DurationMS   int64             `json:"duration_ms,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	BodySample   string            `json:"body_sample,omitempty"`
}

// ObservedNetworkResult returns recent browser network activity for a target tab.
type ObservedNetworkResult struct {
	TargetID string                 `json:"target_id,omitempty"`
	Events   []ObservedNetworkEvent `json:"events,omitempty"`
}

// SnapshotRequest represents a request for a page snapshot.
type SnapshotRequest struct {
	// TargetID is the optional tab ID.
	TargetID string `json:"target_id,omitempty"`
	// Format is the snapshot format: "ai" or "aria".
	Format string `json:"format,omitempty"`
	// Selector limits the snapshot to a specific element.
	Selector string `json:"selector,omitempty"`
	// Interactive only includes interactive elements.
	Interactive bool `json:"interactive,omitempty"`
	// Compact uses compact output format.
	Compact bool `json:"compact,omitempty"`
	// MaxChars limits the output character count.
	MaxChars int `json:"max_chars,omitempty"`
	// Depth limits the tree depth.
	Depth int `json:"depth,omitempty"`
}

// SnapshotResponse represents a page snapshot.
type SnapshotResponse struct {
	// Snapshot is the text representation of the page.
	Snapshot string `json:"snapshot"`
	// Format is the snapshot format used.
	Format string `json:"format"`
	// URL is the page URL.
	URL string `json:"url"`
	// Title is the page title.
	Title string `json:"title"`
	// TargetID is the tab ID.
	TargetID string `json:"target_id"`
	// ElementCount is the number of elements in the snapshot.
	ElementCount int `json:"element_count"`
}

// ActRequest represents an action request.
type ActRequest struct {
	// Kind is the action type: click, type, select, scroll, hover, drag, press, fill, close, wait.
	Kind string `json:"kind" validate:"required"`
	// Ref is the element reference from snapshot.
	Ref string `json:"ref,omitempty"`
	// Selector is a CSS selector (alternative to ref).
	Selector string `json:"selector,omitempty"`
	// TargetID is the optional tab ID.
	TargetID string `json:"target_id,omitempty"`
	// Value is the value for type/select actions.
	Value string `json:"value,omitempty"`
	// Files are local file paths used by upload actions.
	Files []string `json:"files,omitempty"`
	// Text is the text to type.
	Text string `json:"text,omitempty"`
	// Key is the key to press.
	Key string `json:"key,omitempty"`
	// X is the X coordinate for scroll/click.
	X int `json:"x,omitempty"`
	// Y is the Y coordinate for scroll/click.
	Y int `json:"y,omitempty"`
	// Double indicates a double-click.
	Double bool `json:"double,omitempty"`
	// Submit submits the form after typing.
	Submit bool `json:"submit,omitempty"`
	// Options are select options.
	Options []string `json:"options,omitempty"`
	// ToRef is the target ref for drag actions.
	ToRef string `json:"to_ref,omitempty"`
	// Duration is the wait duration in milliseconds.
	Duration int `json:"duration,omitempty"`
	// Timeout is the action timeout in milliseconds.
	Timeout int `json:"timeout,omitempty"`
}

// ActResponse represents the result of an action.
type ActResponse struct {
	// Success indicates if the action succeeded.
	Success bool `json:"success"`
	// Message is an optional message.
	Message string `json:"message,omitempty"`
	// Data contains any data returned by the action.
	Data interface{} `json:"data,omitempty"`
}

// ConsoleRequest represents a request for console messages.
type ConsoleRequest struct {
	// TargetID is the optional tab ID.
	TargetID string `json:"target_id,omitempty"`
	// Level filters by log level: "log", "info", "warn", "error".
	Level string `json:"level,omitempty"`
	// Clear clears the console after returning messages.
	Clear bool `json:"clear,omitempty"`
}

// ConsoleResponse represents console messages.
type ConsoleResponse struct {
	// Messages is the list of console messages.
	Messages []ConsoleMessage `json:"messages"`
}

// ConsoleMessage represents a single console message.
type ConsoleMessage struct {
	// Level is the log level.
	Level string `json:"level"`
	// Text is the message text.
	Text string `json:"text"`
	// Timestamp is when the message was logged.
	Timestamp time.Time `json:"timestamp"`
	// URL is the source URL.
	URL string `json:"url,omitempty"`
	// Line is the source line number.
	Line int `json:"line,omitempty"`
}

// PageInfo contains basic page information.
type PageInfo struct {
	URL   string
	Title string
}

// AccessibilityTreeResponse represents the accessibility tree of a page.
type AccessibilityTreeResponse struct {
	// Tree is the DSL representation of the accessibility tree.
	// Format: @ref [role] "name" properties
	// The @ref can be used in Act() to target elements.
	Tree string `json:"tree"`
	// URL is the page URL.
	URL string `json:"url"`
	// Title is the page title.
	Title string `json:"title"`
	// TargetID is the tab ID.
	TargetID string `json:"target_id"`
	// RefMap maps @ref numbers to backend DOM node IDs for action targeting.
	RefMap map[int]int `json:"ref_map,omitempty"`
}

// InteractiveElementsResponse represents the interactive elements extracted via JS.
// Lighter than the full accessibility tree — only returns actionable elements.
type InteractiveElementsResponse struct {
	Tree     string         `json:"tree"`
	URL      string         `json:"url"`
	Title    string         `json:"title"`
	TargetID string         `json:"target_id"`
	RefMap   map[int]string `json:"ref_map,omitempty"`
	Count    int            `json:"count"`
}

// Viewport represents browser viewport dimensions.
type Viewport struct {
	Width  int
	Height int
}

// DefaultViewport returns the default viewport.
func DefaultViewport() Viewport {
	return Viewport{
		Width:  1920,
		Height: 1080,
	}
}

// GetTimeout returns the timeout duration, applying defaults and limits.
func GetTimeout(requested int, config *Config) time.Duration {
	if requested <= 0 {
		return time.Duration(config.DefaultTimeout) * time.Millisecond
	}
	if requested > config.MaxTimeout {
		return time.Duration(config.MaxTimeout) * time.Millisecond
	}
	return time.Duration(requested) * time.Millisecond
}

// RecipeRequest represents a request to execute a browser recipe.
type RecipeRequest struct {
	// Recipe is the recipe name (search, fill_form, extract, login).
	Recipe string `json:"recipe" validate:"required"`
	// Params are the recipe-specific parameters.
	Params map[string]string `json:"params" validate:"required"`
}

// RecipeResponse represents the result of a recipe execution.
type RecipeResponse struct {
	// Success indicates if the recipe completed successfully.
	Success bool `json:"success"`
	// Recipe is the recipe name that was executed.
	Recipe string `json:"recipe"`
	// Data contains the structured result data.
	Data map[string]interface{} `json:"data,omitempty"`
	// TargetID is the tab ID if the recipe kept the tab open.
	TargetID string `json:"target_id,omitempty"`
	// Message is a human-readable summary.
	Message string `json:"message"`
}
