// Package browser provides browser automation capabilities using Rod.
// This module is designed to be compatible with clawdbot's browser tool interface.
package browser

import (
	"context"
	"errors"
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
	// EvaluateEnabled controls whether JavaScript evaluation is allowed.
	// When false, act:evaluate and wait --fn are disabled to prevent
	// prompt injection attacks from executing arbitrary JavaScript.
	// Default: true
	EvaluateEnabled *bool `json:"evaluate_enabled" yaml:"evaluate_enabled"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	evaluateEnabled := true
	return &Config{
		PoolSize:              3,
		Headless:              true,
		DefaultTimeout:        30000,
		MaxTimeout:            120000,
		DefaultViewportWidth:  1920,
		DefaultViewportHeight: 1080,
		AllowedDomains:        []string{},
		BlockedDomains:        []string{},
		UserAgent:             "",
		ProxyURL:              "",
		BrowserPath:           "",
		EvaluateEnabled:       &evaluateEnabled,
	}
}

// IsEvaluateEnabled returns whether JavaScript evaluation is enabled.
func (c *Config) IsEvaluateEnabled() bool {
	if c.EvaluateEnabled == nil {
		return true // Default to enabled for backwards compatibility
	}
	return *c.EvaluateEnabled
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
