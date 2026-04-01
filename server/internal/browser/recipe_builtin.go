package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// ---------------------------------------------------------------------------
// Search engine selector templates
// ---------------------------------------------------------------------------

type searchEngine struct {
	URL             string
	SearchBox       string // CSS selector for search input
	ResultContainer string // CSS selector for each result block
	ResultTitle     string // CSS selector within container for title
	ResultLink      string // CSS selector within container for link
	ResultSnippet   string // CSS selector within container for snippet
}

var searchEngines = map[string]searchEngine{
	"google": {
		URL:             "https://www.google.com",
		SearchBox:       `textarea[name="q"], input[name="q"]`,
		ResultContainer: "#rso .g, #search .g",
		ResultTitle:     "h3",
		ResultLink:      "a[href]",
		ResultSnippet:   ".VwiC3b, [data-sncf], .lEBKkf",
	},
	"bing": {
		URL:             "https://www.bing.com",
		SearchBox:       `input[name="q"]`,
		ResultContainer: "#b_results .b_algo",
		ResultTitle:     "h2 a",
		ResultLink:      "h2 a",
		ResultSnippet:   ".b_caption p",
	},
	"duckduckgo": {
		URL:             "https://duckduckgo.com",
		SearchBox:       `input[name="q"]`,
		ResultContainer: "[data-testid='result']",
		ResultTitle:     "[data-testid='result-title-a']",
		ResultLink:      "[data-testid='result-title-a']",
		ResultSnippet:   "[data-testid='result-snippet']",
	},
	"baidu": {
		URL:             "https://www.baidu.com",
		SearchBox:       `input[name="wd"]`,
		ResultContainer: ".result.c-container",
		ResultTitle:     "h3 a",
		ResultLink:      "h3 a",
		ResultSnippet:   ".c-abstract, .content-right_8Zs40",
	},
}

// SearchResult is a single search result entry.
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// ---------------------------------------------------------------------------
// 1. searchRecipe
// ---------------------------------------------------------------------------

type searchRecipe struct{}

func (r *searchRecipe) Name() string { return "search" }
func (r *searchRecipe) Description() string {
	return "Search via search engine and return structured results (title, url, snippet). Params: query (required), engine (optional: google/bing/duckduckgo/baidu, default google), max_results (optional, default 10)"
}
func (r *searchRecipe) KeepTab() bool { return false }

func (r *searchRecipe) Validate(params map[string]string) error {
	if params["query"] == "" {
		return fmt.Errorf("query is required")
	}
	if eng := params["engine"]; eng != "" {
		if _, ok := searchEngines[eng]; !ok {
			return fmt.Errorf("unsupported engine: %s (use google, bing, duckduckgo, baidu)", eng)
		}
	}
	return nil
}

func (r *searchRecipe) Execute(ctx context.Context, svc *RodService, params map[string]string) (*RecipeResult, error) {
	engineName := params["engine"]
	if engineName == "" {
		engineName = "google"
	}
	engine := searchEngines[engineName]
	query := params["query"]

	// Get a page
	page, browser, err := svc.pool.NewPage(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create page: %w", err)
	}
	defer svc.pool.ReleasePage(page, browser)

	timeout := GetTimeout(0, svc.config)

	// Prefer direct navigation to the search results page. This is more robust
	// than relying on homepage DOM search-box selectors, which can drift or be
	// hidden behind anti-bot interstitials.
	targetURL := engine.URL
	if directURL := buildSearchURL(engineName, query); directURL != "" {
		targetURL = directURL
	}
	if err := page.Timeout(timeout).Navigate(targetURL); err != nil {
		return nil, fmt.Errorf("navigate to %s failed: %w", engineName, err)
	}
	if err := waitPageLoad(page, 0); err != nil {
		return nil, fmt.Errorf("page load failed: %w", err)
	}
	_ = waitPageStable(page, 2*time.Second)

	if results, err := extractSearchResults(page, engine); err == nil && len(results) > 0 {
		return finalizeSearchRecipeResult(page, engineName, query, params["max_results"], results)
	}

	// Find and fill search box
	searchBox, err := page.Timeout(timeout).Element(engine.SearchBox)
	if err != nil {
		return nil, fmt.Errorf("search box not found (%s): %w", engine.SearchBox, err)
	}
	if err := searchBox.Input(query); err != nil {
		return nil, fmt.Errorf("failed to type query: %w", err)
	}

	// Submit search
	if err := page.Keyboard.Press(input.Enter); err != nil {
		return nil, fmt.Errorf("failed to submit search: %w", err)
	}

	// Wait for results to load
	time.Sleep(1500 * time.Millisecond)
	_ = waitPageStable(page, 2*time.Second)

	// Extract results via JS for efficiency
	results, err := extractSearchResults(page, engine)
	if err != nil {
		return nil, fmt.Errorf("failed to extract results: %w", err)
	}

	return finalizeSearchRecipeResult(page, engineName, query, params["max_results"], results)
}

func finalizeSearchRecipeResult(page *rod.Page, engineName, query, rawMaxResults string, results []SearchResult) (*RecipeResult, error) {
	// Limit results
	maxResults := 10
	if v := rawMaxResults; v != "" {
		if n := parseInt(v); n > 0 && n < 50 {
			maxResults = n
		}
	}
	if len(results) > maxResults {
		results = results[:maxResults]
	}

	info := snapshotPageInfo(page)

	msg := fmt.Sprintf("Search '%s' on %s — %d results", query, engineName, len(results))
	return &RecipeResult{
		Success: true,
		Data: map[string]interface{}{
			"results":  results,
			"count":    len(results),
			"engine":   engineName,
			"query":    query,
			"page_url": info.URL,
		},
		Message: msg,
	}, nil
}

func buildSearchURL(engineName, query string) string {
	trimmedQuery := strings.TrimSpace(query)
	if trimmedQuery == "" {
		return ""
	}
	escapedQuery := url.QueryEscape(trimmedQuery)
	switch strings.ToLower(strings.TrimSpace(engineName)) {
	case "google":
		return "https://www.google.com/search?q=" + escapedQuery
	case "bing":
		return "https://www.bing.com/search?q=" + escapedQuery
	case "duckduckgo":
		return "https://duckduckgo.com/?q=" + escapedQuery
	case "baidu":
		return "https://www.baidu.com/s?wd=" + escapedQuery
	default:
		return ""
	}
}

func extractSearchResults(page *rod.Page, engine searchEngine) ([]SearchResult, error) {
	containers, err := page.Elements(engine.ResultContainer)
	if err != nil || len(containers) == 0 {
		return nil, fmt.Errorf("no results found")
	}

	results := make([]SearchResult, 0, len(containers))
	for _, container := range containers {
		var title, link, snippet string

		if el, err := container.Element(engine.ResultTitle); err == nil {
			title, _ = el.Text()
		}
		if el, err := container.Element(engine.ResultLink); err == nil {
			if href, err := el.Attribute("href"); err == nil && href != nil {
				link = *href
			}
		}
		if el, err := container.Element(engine.ResultSnippet); err == nil {
			snippet, _ = el.Text()
		}

		if title == "" && link == "" {
			continue
		}

		results = append(results, SearchResult{
			Title:   strings.TrimSpace(title),
			URL:     link,
			Snippet: strings.TrimSpace(snippet),
		})
	}

	return results, nil
}

// ---------------------------------------------------------------------------
// 2. fillFormRecipe
// ---------------------------------------------------------------------------

type fillFormRecipe struct{}

func (r *fillFormRecipe) Name() string { return "fill_form" }
func (r *fillFormRecipe) Description() string {
	return `Fill form fields on a page. Params: url (required), fields (required, JSON object: {"field_name_or_selector": "value"}), submit (optional: "true" to submit after filling)`
}
func (r *fillFormRecipe) KeepTab() bool { return true }

func (r *fillFormRecipe) Validate(params map[string]string) error {
	if params["url"] == "" {
		return fmt.Errorf("url is required")
	}
	if params["fields"] == "" {
		return fmt.Errorf("fields is required (JSON object)")
	}
	var fields map[string]string
	if err := json.Unmarshal([]byte(params["fields"]), &fields); err != nil {
		return fmt.Errorf("fields must be valid JSON object: %w", err)
	}
	if len(fields) == 0 {
		return fmt.Errorf("fields must not be empty")
	}
	return nil
}

func (r *fillFormRecipe) Execute(ctx context.Context, svc *RodService, params map[string]string) (*RecipeResult, error) {
	if err := svc.security.CheckURL(params["url"]); err != nil {
		return nil, err
	}

	var fields map[string]string
	_ = json.Unmarshal([]byte(params["fields"]), &fields)

	page, browser, err := svc.pool.NewPage(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create page: %w", err)
	}
	// Don't release — KeepTab=true, we'll register as a tab

	timeout := GetTimeout(0, svc.config)
	if err := page.Timeout(timeout).Navigate(params["url"]); err != nil {
		svc.pool.ReleasePage(page, browser)
		return nil, fmt.Errorf("navigate failed: %w", err)
	}
	if err := waitPageLoad(page, 0); err != nil {
		svc.pool.ReleasePage(page, browser)
		return nil, fmt.Errorf("page load failed: %w", err)
	}

	// Fill each field
	filled := make([]string, 0, len(fields))
	var fillErrors []string
	for key, value := range fields {
		el, err := findFormField(page, key)
		if err != nil {
			fillErrors = append(fillErrors, fmt.Sprintf("%s: %s", key, err.Error()))
			continue
		}
		_ = el.SelectAllText()
		if err := el.Input(value); err != nil {
			fillErrors = append(fillErrors, fmt.Sprintf("%s: input failed: %s", key, err.Error()))
			continue
		}
		filled = append(filled, key)
	}

	// Submit if requested
	submitted := false
	if params["submit"] == "true" {
		if err := page.Keyboard.Press(input.Enter); err == nil {
			submitted = true
			time.Sleep(1 * time.Second)
			_ = waitPageStable(page, 2*time.Second)
		}
	}

	info := snapshotPageInfo(page)

	// Register as tab
	targetID := fmt.Sprintf("tab-%d", timeutil.NowNano())
	svc.tabsMu.Lock()
	for _, t := range svc.tabs {
		t.active = false
	}
	svc.tabs[targetID] = &tabInfo{
		page:     page,
		browser:  browser,
		targetID: targetID,
		url:      info.URL,
		title:    info.Title,
		active:   true,
		console:  make([]ConsoleMessage, 0),
	}
	svc.tabsMu.Unlock()

	msg := fmt.Sprintf("Filled %d/%d fields on %s", len(filled), len(fields), info.URL)
	if submitted {
		msg += " (submitted)"
	}

	return &RecipeResult{
		Success:  len(fillErrors) == 0,
		TargetID: targetID,
		Data: map[string]interface{}{
			"filled":    filled,
			"errors":    fillErrors,
			"submitted": submitted,
			"url":       info.URL,
			"title":     info.Title,
		},
		Message: msg,
	}, nil
}

// findFormField tries multiple strategies to locate a form field.
func findFormField(page *rod.Page, key string) (*rod.Element, error) {
	// 1. Direct CSS selector
	if strings.HasPrefix(key, "#") || strings.HasPrefix(key, ".") || strings.Contains(key, "[") {
		el, err := page.Element(key)
		if err == nil {
			return el, nil
		}
	}

	// 2. input[name="key"]
	if el, err := page.Element(fmt.Sprintf(`input[name="%s"], textarea[name="%s"], select[name="%s"]`, key, key, key)); err == nil {
		return el, nil
	}

	// 3. input[id="key"]
	if el, err := page.Element(fmt.Sprintf(`#%s`, key)); err == nil {
		return el, nil
	}

	// 4. Label text match → associated input (via JS)
	el, err := page.ElementByJS(rod.Eval(fmt.Sprintf(`() => {
		const labels = document.querySelectorAll('label');
		for (const label of labels) {
			if (label.textContent.trim().toLowerCase().includes(%q)) {
				const forAttr = label.getAttribute('for');
				if (forAttr) return document.getElementById(forAttr);
				const input = label.querySelector('input, textarea, select');
				if (input) return input;
			}
		}
		return null;
	}`, strings.ToLower(key))))
	if err == nil && el != nil {
		return el, nil
	}

	// 5. Placeholder text match
	if el, err := page.Element(fmt.Sprintf(`input[placeholder*="%s" i], textarea[placeholder*="%s" i]`, key, key)); err == nil {
		return el, nil
	}

	return nil, fmt.Errorf("field not found: %s", key)
}

// ---------------------------------------------------------------------------
// 3. extractRecipe
// ---------------------------------------------------------------------------

type extractRecipe struct{}

func (r *extractRecipe) Name() string { return "extract" }
func (r *extractRecipe) Description() string {
	return `Extract data from a page using CSS selectors. Params: url (required), selectors (required, JSON object: {"name": "css_selector"}), multiple (optional: "true" to extract all matches)`
}
func (r *extractRecipe) KeepTab() bool { return false }

func (r *extractRecipe) Validate(params map[string]string) error {
	if params["url"] == "" {
		return fmt.Errorf("url is required")
	}
	if params["selectors"] == "" {
		return fmt.Errorf("selectors is required (JSON object)")
	}
	var selectors map[string]string
	if err := json.Unmarshal([]byte(params["selectors"]), &selectors); err != nil {
		return fmt.Errorf("selectors must be valid JSON object: %w", err)
	}
	if len(selectors) == 0 {
		return fmt.Errorf("selectors must not be empty")
	}
	return nil
}

func (r *extractRecipe) Execute(ctx context.Context, svc *RodService, params map[string]string) (*RecipeResult, error) {
	if err := svc.security.CheckURL(params["url"]); err != nil {
		return nil, err
	}

	var selectors map[string]string
	_ = json.Unmarshal([]byte(params["selectors"]), &selectors)
	multiple := params["multiple"] == "true"

	page, browser, err := svc.pool.NewPage(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create page: %w", err)
	}
	defer svc.pool.ReleasePage(page, browser)

	timeout := GetTimeout(0, svc.config)
	if err := page.Timeout(timeout).Navigate(params["url"]); err != nil {
		return nil, fmt.Errorf("navigate failed: %w", err)
	}
	if err := waitPageLoad(page, 0); err != nil {
		return nil, fmt.Errorf("page load failed: %w", err)
	}

	data := make(map[string]interface{}, len(selectors))
	for name, selector := range selectors {
		if multiple {
			elements, err := page.Elements(selector)
			if err != nil {
				data[name] = []string{}
				continue
			}
			values := make([]string, 0, len(elements))
			for _, el := range elements {
				text, _ := el.Text()
				values = append(values, strings.TrimSpace(text))
			}
			data[name] = values
		} else {
			el, err := page.Element(selector)
			if err != nil {
				data[name] = ""
				continue
			}
			text, _ := el.Text()
			data[name] = strings.TrimSpace(text)
		}
	}

	info := snapshotPageInfo(page)

	return &RecipeResult{
		Success: true,
		Data: map[string]interface{}{
			"extracted": data,
			"url":       info.URL,
			"title":     info.Title,
		},
		Message: fmt.Sprintf("Extracted %d fields from %s", len(selectors), info.URL),
	}, nil
}

// ---------------------------------------------------------------------------
// 4. loginRecipe
// ---------------------------------------------------------------------------

type loginRecipe struct{}

func (r *loginRecipe) Name() string { return "login" }
func (r *loginRecipe) Description() string {
	return `Log into a website. Params: url (required), username (required), password (required), username_selector (optional), password_selector (optional), submit_selector (optional)`
}
func (r *loginRecipe) KeepTab() bool { return true }

func (r *loginRecipe) Validate(params map[string]string) error {
	if params["url"] == "" {
		return fmt.Errorf("url is required")
	}
	if params["username"] == "" {
		return fmt.Errorf("username is required")
	}
	if params["password"] == "" {
		return fmt.Errorf("password is required")
	}
	return nil
}

func (r *loginRecipe) Execute(ctx context.Context, svc *RodService, params map[string]string) (*RecipeResult, error) {
	if err := svc.security.CheckURL(params["url"]); err != nil {
		return nil, err
	}

	page, browser, err := svc.pool.NewPage(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create page: %w", err)
	}

	timeout := GetTimeout(0, svc.config)
	if err := page.Timeout(timeout).Navigate(params["url"]); err != nil {
		svc.pool.ReleasePage(page, browser)
		return nil, fmt.Errorf("navigate failed: %w", err)
	}
	if err := waitPageLoad(page, 0); err != nil {
		svc.pool.ReleasePage(page, browser)
		return nil, fmt.Errorf("page load failed: %w", err)
	}

	startURL := params["url"]

	// Find username field
	usernameEl, err := findLoginField(page, params["username_selector"], "username")
	if err != nil {
		svc.pool.ReleasePage(page, browser)
		return nil, fmt.Errorf("username field not found: %w", err)
	}
	_ = usernameEl.SelectAllText()
	if err := usernameEl.Input(params["username"]); err != nil {
		svc.pool.ReleasePage(page, browser)
		return nil, fmt.Errorf("failed to type username: %w", err)
	}

	// Find password field
	passwordEl, err := findLoginField(page, params["password_selector"], "password")
	if err != nil {
		svc.pool.ReleasePage(page, browser)
		return nil, fmt.Errorf("password field not found: %w", err)
	}
	_ = passwordEl.SelectAllText()
	if err := passwordEl.Input(params["password"]); err != nil {
		svc.pool.ReleasePage(page, browser)
		return nil, fmt.Errorf("failed to type password: %w", err)
	}

	// Submit
	if sel := params["submit_selector"]; sel != "" {
		if btn, err := page.Element(sel); err == nil {
			_ = btn.Click(proto.InputMouseButtonLeft, 1)
		}
	} else {
		// Try common submit patterns
		submitted := false
		for _, sel := range []string{
			`button[type="submit"]`,
			`input[type="submit"]`,
			`button:has-text("Log in")`,
			`button:has-text("Sign in")`,
			`button:has-text("登录")`,
		} {
			if btn, err := page.Element(sel); err == nil {
				_ = btn.Click(proto.InputMouseButtonLeft, 1)
				submitted = true
				break
			}
		}
		if !submitted {
			_ = page.Keyboard.Press(input.Enter)
		}
	}

	// Wait for navigation
	time.Sleep(2 * time.Second)
	_ = waitPageStable(page, 3*time.Second)

	info := snapshotPageInfo(page)

	// Determine success by URL change
	urlChanged := info.URL != startURL

	// Register as tab
	targetID := fmt.Sprintf("tab-%d", timeutil.NowNano())
	svc.tabsMu.Lock()
	for _, t := range svc.tabs {
		t.active = false
	}
	svc.tabs[targetID] = &tabInfo{
		page:     page,
		browser:  browser,
		targetID: targetID,
		url:      info.URL,
		title:    info.Title,
		active:   true,
		console:  make([]ConsoleMessage, 0),
	}
	svc.tabsMu.Unlock()

	// Don't include password in result
	return &RecipeResult{
		Success:  urlChanged,
		TargetID: targetID,
		Data: map[string]interface{}{
			"url":         info.URL,
			"title":       info.Title,
			"url_changed": urlChanged,
		},
		Message: fmt.Sprintf("Login attempted on %s — URL %s", info.Title, ternary(urlChanged, "changed (likely success)", "unchanged (may need verification)")),
	}, nil
}

// findLoginField finds a login form field by explicit selector or common patterns.
func findLoginField(page *rod.Page, explicitSelector, fieldType string) (*rod.Element, error) {
	if explicitSelector != "" {
		return page.Element(explicitSelector)
	}

	// Common selectors for login fields
	var selectors []string
	switch fieldType {
	case "username":
		selectors = []string{
			`input[type="email"]`,
			`input[name="username"]`,
			`input[name="email"]`,
			`input[name="user"]`,
			`input[name="login"]`,
			`input[id="username"]`,
			`input[id="email"]`,
			`input[autocomplete="username"]`,
			`input[autocomplete="email"]`,
			`input[type="text"]:first-of-type`,
		}
	case "password":
		selectors = []string{
			`input[type="password"]`,
			`input[name="password"]`,
			`input[name="passwd"]`,
			`input[name="pass"]`,
			`input[autocomplete="current-password"]`,
		}
	}

	for _, sel := range selectors {
		if el, err := page.Element(sel); err == nil {
			return el, nil
		}
	}

	return nil, fmt.Errorf("no %s field found", fieldType)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func parseInt(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			break
		}
	}
	return n
}

func ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}
