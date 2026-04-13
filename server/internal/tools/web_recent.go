package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	webRecentRetrievalProfile = "recent_multi_site_v1"
	webRecentLookbackDays     = 30
)

var webRecentDatePattern = regexp.MustCompile(`\b(20\d{2})[-/](\d{1,2})[-/](\d{1,2})\b`)
var webRecentURLPattern = regexp.MustCompile(`https?://[^\s]+`)
var webRecentHTTPClient = &http.Client{Timeout: 8 * time.Second}

type webRecentNormalizedItem struct {
	Source          string `json:"source"`
	Title           string `json:"title"`
	URL             string `json:"url"`
	Snippet         string `json:"snippet,omitempty"`
	PublishedAt     string `json:"published_at,omitempty"`
	Author          string `json:"author,omitempty"`
	EngagementHint  string `json:"engagement_hint,omitempty"`
	BrowserAssisted bool   `json:"browser_assisted,omitempty"`
}

type webRecentReport struct {
	Answer           string                   `json:"answer"`
	Confidence       float64                  `json:"confidence"`
	ItemsBySource    map[string]interface{}   `json:"items_by_source,omitempty"`
	ErrorsBySource   map[string]string        `json:"errors_by_source,omitempty"`
	Clusters         []map[string]interface{} `json:"clusters,omitempty"`
	LookbackDays     int                      `json:"lookback_days"`
	BrowserAssisted  bool                     `json:"browser_assisted,omitempty"`
	RetrievalProfile string                   `json:"retrieval_profile"`
}

type webRecentHNSearchResponse struct {
	Hits []struct {
		Title       string `json:"title"`
		URL         string `json:"url"`
		Author      string `json:"author"`
		Points      int    `json:"points"`
		NumComments int    `json:"num_comments"`
		CreatedAt   string `json:"created_at"`
		ObjectID    string `json:"objectID"`
	} `json:"hits"`
}

func (t *WebTool) executeRecentMultiSiteQuery(ctx context.Context, args map[string]interface{}, input string) (interface{}, error) {
	report := webRecentReport{
		Confidence:       0.2,
		ItemsBySource:    map[string]interface{}{},
		ErrorsBySource:   map[string]string{},
		LookbackDays:     webRecentLookbackDays,
		RetrievalProfile: webRecentRetrievalProfile,
	}

	aggregate := make([]webRecentNormalizedItem, 0, 16)
	appendSource := func(source string, items []webRecentNormalizedItem, browserAssisted bool, err error) {
		if err != nil {
			report.ErrorsBySource[source] = strings.TrimSpace(err.Error())
		}
		if len(items) == 0 {
			return
		}
		sort.SliceStable(items, func(i, j int) bool {
			return items[i].PublishedAt > items[j].PublishedAt
		})
		copied := make([]webRecentNormalizedItem, len(items))
		copy(copied, items)
		report.ItemsBySource[source] = copied
		report.BrowserAssisted = report.BrowserAssisted || browserAssisted
		aggregate = append(aggregate, copied...)
	}

	items, assisted, err := t.collectRecentSiteSearchItems(ctx, args, input, "", "web", 4, false)
	appendSource("web", items, assisted, err)

	items, assisted, err = t.collectRecentSiteSearchItems(ctx, args, input, "reddit.com", "reddit", 3, true)
	appendSource("reddit", items, assisted, err)

	items, assisted, err = t.collectRecentSocialItems(ctx, args, input, "x", []string{"x.com", "twitter.com"}, 3)
	appendSource("x", items, assisted, err)

	items, assisted, err = t.collectRecentSocialItems(ctx, args, input, "tiktok", []string{"tiktok.com"}, 2)
	appendSource("tiktok", items, assisted, err)

	items, assisted, err = t.collectRecentSocialItems(ctx, args, input, "instagram", []string{"instagram.com"}, 2)
	appendSource("instagram", items, assisted, err)

	items, assisted, err = t.collectRecentSocialItems(ctx, args, input, "bluesky", []string{"bsky.app"}, 2)
	appendSource("bluesky", items, assisted, err)

	items, err = t.collectRecentHackerNewsItems(ctx, input, 3)
	appendSource("hacker_news", items, false, err)

	items, err = t.collectRecentGitHubItems(ctx, args, input, 3)
	appendSource("github", items, false, err)

	items, err = t.collectRecentPolymarketItems(ctx, input, 3)
	appendSource("polymarket", items, false, err)

	items, err = t.collectRecentYouTubeItems(ctx, args, input)
	appendSource("youtube", items, false, err)

	report.Clusters = buildWebRecentClusters(aggregate)
	report.Answer = summarizeWebRecentReport(aggregate, report.Clusters)
	if len(aggregate) > 0 {
		report.Confidence = minFloat64(0.88, 0.35+float64(len(aggregate))*0.06)
	}
	if len(report.ItemsBySource) == 0 {
		report.ItemsBySource = nil
	}
	if len(report.ErrorsBySource) == 0 {
		report.ErrorsBySource = nil
	}
	raw, err := json.Marshal(report)
	if err != nil {
		return nil, err
	}
	return string(raw), nil
}

func (t *WebTool) collectRecentSiteSearchItems(ctx context.Context, args map[string]interface{}, query, site, source string, maxResults int, allowBrowserFallback bool) ([]webRecentNormalizedItem, bool, error) {
	if t == nil || t.search == nil {
		return nil, false, fmt.Errorf("web search is not available")
	}
	searchQuery := strings.TrimSpace(query)
	if site != "" {
		searchQuery = strings.TrimSpace("site:" + site + " " + query)
	}
	resp, _, err := t.runSearchDiscovery(ctx, args, searchQuery, maxResults)
	if err != nil {
		return nil, false, err
	}
	items := make([]webRecentNormalizedItem, 0, maxResults)
	seen := map[string]struct{}{}
	browserAssisted := false
	for _, result := range resp.Results {
		if len(items) >= maxResults {
			break
		}
		if canonical := strings.TrimSpace(result.URL); canonical != "" {
			if _, ok := seen[canonical]; ok {
				continue
			}
			seen[canonical] = struct{}{}
		}
		item, assisted, ok, itemErr := t.collectRecentReadItem(ctx, args, source, result, allowBrowserFallback)
		if itemErr != nil {
			if len(items) == 0 {
				err = itemErr
			}
			continue
		}
		if !ok {
			continue
		}
		browserAssisted = browserAssisted || assisted
		items = append(items, item)
	}
	if len(items) == 0 && err != nil {
		return nil, browserAssisted, err
	}
	return items, browserAssisted, nil
}

func (t *WebTool) collectRecentReadItem(ctx context.Context, args map[string]interface{}, source string, result WebSearchResult, allowBrowserFallback bool) (webRecentNormalizedItem, bool, bool, error) {
	resp, err := t.readRecentPage(ctx, args, result.URL)
	if err != nil {
		return webRecentNormalizedItem{}, false, false, err
	}
	title := firstNonEmpty(strings.TrimSpace(resp.Title), strings.TrimSpace(result.Title))
	contentText := firstNonEmpty(resp.Content, result.Description)
	snippet := firstNonEmpty(compactWebRecentSnippet(contentText), strings.TrimSpace(result.Description))
	browserAssisted := false
	if allowBrowserFallback && webRecentNeedsBrowserFallback(resp) {
		content, browserErr := t.readRecentBrowserContent(ctx, result.URL)
		if browserErr == nil {
			browserAssisted = true
			contentText = content
			if browserSnippet := compactWebRecentSnippet(content); browserSnippet != "" {
				snippet = browserSnippet
			}
		} else if strings.TrimSpace(snippet) == "" {
			return webRecentNormalizedItem{}, false, false, browserErr
		}
	}
	publishedAt, ok := webRecentPublishedAt(contentText, title)
	if !ok {
		return webRecentNormalizedItem{}, false, false, nil
	}
	return webRecentNormalizedItem{
		Source:          source,
		Title:           title,
		URL:             firstNonEmpty(strings.TrimSpace(resp.FinalURL), strings.TrimSpace(resp.URL), strings.TrimSpace(result.URL)),
		Snippet:         snippet,
		PublishedAt:     publishedAt,
		BrowserAssisted: browserAssisted,
	}, browserAssisted, true, nil
}

func (t *WebTool) readRecentPage(ctx context.Context, args map[string]interface{}, targetURL string) (webReadResponse, error) {
	if t == nil || t.read == nil {
		return webReadResponse{}, fmt.Errorf("web read is not available")
	}
	readArgs := normalizeWebReadCompatArgs(args)
	readArgs["url"] = targetURL
	readArgs["format"] = "text"
	readArgs["max_chars"] = 4000
	readArgs["disable_internal_fallbacks"] = true
	raw, err := t.read.Execute(ctx, readArgs)
	if err != nil {
		return webReadResponse{}, err
	}
	var resp webReadResponse
	if err := decodeToolJSONResult(raw, &resp); err != nil {
		return webReadResponse{}, err
	}
	if strings.TrimSpace(resp.URL) == "" {
		resp.URL = targetURL
	}
	return resp, nil
}

func (t *WebTool) readRecentBrowserContent(ctx context.Context, targetURL string) (string, error) {
	if t == nil || t.browser == nil {
		return "", fmt.Errorf("browser fallback is not available")
	}
	return extractReadableBrowserContentViaRecipe(ctx, t.browser, targetURL)
}

func webRecentNeedsBrowserFallback(resp webReadResponse) bool {
	if resp.InteractiveRequired {
		return true
	}
	for _, code := range resp.WarningCodes {
		switch strings.ToLower(strings.TrimSpace(code)) {
		case "login_wall", "challenge", "browser_required":
			return true
		}
	}
	if webRecentHighCouplingURL(firstNonEmpty(strings.TrimSpace(resp.FinalURL), strings.TrimSpace(resp.URL))) &&
		len([]rune(strings.TrimSpace(resp.Content))) < webFetchMinReadableChars {
		return true
	}
	return false
}

func (t *WebTool) collectRecentHackerNewsItems(ctx context.Context, query string, maxResults int) ([]webRecentNormalizedItem, error) {
	params := url.Values{}
	params.Set("query", strings.TrimSpace(query))
	params.Set("tags", "story")
	params.Set("hitsPerPage", strconv.Itoa(maxResults))
	params.Set("numericFilters", fmt.Sprintf("created_at_i>%d", time.Now().UTC().AddDate(0, 0, -webRecentLookbackDays).Unix()))
	var payload webRecentHNSearchResponse
	if err := webRecentHTTPJSON(ctx, "https://hn.algolia.com/api/v1/search?"+params.Encode(), map[string]string{"Accept": "application/json"}, &payload); err != nil {
		return nil, err
	}
	items := make([]webRecentNormalizedItem, 0, len(payload.Hits))
	for _, hit := range payload.Hits {
		itemURL := strings.TrimSpace(hit.URL)
		if itemURL == "" && strings.TrimSpace(hit.ObjectID) != "" {
			itemURL = "https://news.ycombinator.com/item?id=" + strings.TrimSpace(hit.ObjectID)
		}
		publishedAt, ok := webRecentPublishedAt(hit.CreatedAt, hit.Title)
		if !ok {
			continue
		}
		items = append(items, webRecentNormalizedItem{
			Source:         "hacker_news",
			Title:          strings.TrimSpace(hit.Title),
			URL:            itemURL,
			PublishedAt:    publishedAt,
			Author:         strings.TrimSpace(hit.Author),
			EngagementHint: fmt.Sprintf("%d points, %d comments", hit.Points, hit.NumComments),
		})
	}
	return items, nil
}

func (t *WebTool) collectRecentGitHubItems(ctx context.Context, args map[string]interface{}, query string, maxResults int) ([]webRecentNormalizedItem, error) {
	headers := map[string]string{
		"Accept":     "application/vnd.github+json",
		"User-Agent": "ZimaOS-Blue",
	}
	issues, issueErr := t.collectRecentGitHubIssueItems(ctx, query, maxResults, headers)
	repositories, repoErr := t.collectRecentGitHubRepositories(ctx, query, minWebQueryInt(maxResults, 2), headers)
	releaseItems, readmeItems, repoSignalErr := t.collectRecentGitHubRepositorySignals(ctx, repositories, headers)

	items := append([]webRecentNormalizedItem{}, issues...)
	items = append(items, releaseItems...)
	items = append(items, readmeItems...)
	items = webRecentSortDedupLimit(items, maxResults)
	if len(items) > 0 {
		return items, webRecentJoinErrors(issueErr, repoErr, repoSignalErr)
	}

	fallback, _, searchErr := t.collectRecentSiteSearchItems(ctx, args, strings.TrimSpace(query+" github discussions"), "github.com", "github", maxResults, false)
	if len(fallback) > 0 {
		return fallback, nil
	}
	return nil, webRecentJoinErrors(issueErr, repoErr, repoSignalErr, searchErr)
}

func (t *WebTool) collectRecentPolymarketItems(ctx context.Context, query string, maxResults int) ([]webRecentNormalizedItem, error) {
	params := url.Values{}
	params.Set("q", strings.TrimSpace(query))
	params.Set("page", "1")
	params.Set("events_status", "active")
	var payload interface{}
	if err := webRecentHTTPJSON(ctx, "https://gamma-api.polymarket.com/public-search?"+params.Encode(), map[string]string{"Accept": "application/json"}, &payload); err != nil {
		return nil, err
	}
	events := webRecentPolymarketEvents(payload)
	items := make([]webRecentNormalizedItem, 0, minWebQueryInt(maxResults, len(events)))
	for _, event := range events {
		if len(items) >= maxResults {
			break
		}
		title := firstNonEmpty(strings.TrimSpace(asString(event["title"])), strings.TrimSpace(asString(event["question"])))
		if title == "" {
			continue
		}
		publishedAt, ok := webRecentPublishedAt(
			asString(event["createdAt"]),
			asString(event["created_at"]),
			asString(event["endDate"]),
			asString(event["end_date"]),
			title,
		)
		if !ok {
			continue
		}
		itemURL := strings.TrimSpace(asString(event["url"]))
		if itemURL == "" {
			if slug := strings.TrimSpace(asString(event["slug"])); slug != "" {
				itemURL = "https://polymarket.com/event/" + slug
			}
		}
		engagement := firstNonEmpty(strings.TrimSpace(asString(event["liquidity"])), strings.TrimSpace(asString(event["volume"])))
		items = append(items, webRecentNormalizedItem{
			Source:         "polymarket",
			Title:          title,
			URL:            itemURL,
			Snippet:        compactWebRecentSnippet(asString(event["description"])),
			PublishedAt:    publishedAt,
			EngagementHint: engagement,
		})
	}
	return items, nil
}

func (t *WebTool) collectRecentYouTubeItems(ctx context.Context, args map[string]interface{}, input string) ([]webRecentNormalizedItem, error) {
	urls := extractURLs(input)
	items := make([]webRecentNormalizedItem, 0, len(urls))
	for _, candidate := range urls {
		if !looksLikeWebQueryVideoURL(candidate) {
			continue
		}
		envelope := t.executeVideoURLQuery(ctx, args, candidate, webSearchFormatJSON, 3000)
		title := strings.TrimSpace(envelope.Title)
		if title == "" && envelope.Media != nil {
			title = strings.TrimSpace(envelope.Media.Platform)
		}
		publishedAt := ""
		if envelope.Media != nil {
			publishedAt = strings.TrimSpace(envelope.Media.PublishedAt)
		}
		if publishedAt == "" {
			continue
		}
		items = append(items, webRecentNormalizedItem{
			Source:         "youtube",
			Title:          firstNonEmpty(title, candidate),
			URL:            candidate,
			Snippet:        compactWebRecentSnippet(firstNonEmpty(envelope.Content, envelope.Title)),
			PublishedAt:    publishedAt,
			Author:         firstNonEmpty(envelope.Media.Author),
			EngagementHint: "known URL enrichment",
		})
	}
	return items, nil
}

func buildWebRecentClusters(items []webRecentNormalizedItem) []map[string]interface{} {
	if len(items) == 0 {
		return nil
	}
	stopwords := map[string]struct{}{
		"about": {}, "after": {}, "blue": {}, "days": {}, "have": {}, "last": {}, "recent": {}, "saying": {}, "that": {}, "their": {}, "they": {}, "what": {}, "with": {}, "zimaos": {},
	}
	type clusterState struct {
		count   int
		sources map[string]struct{}
	}
	counts := map[string]*clusterState{}
	for _, item := range items {
		seen := map[string]struct{}{}
		text := strings.ToLower(strings.TrimSpace(item.Title + " " + item.Snippet))
		for _, token := range splitWebRecentTokens(text) {
			if len(token) < 4 {
				continue
			}
			if _, skip := stopwords[token]; skip {
				continue
			}
			if _, ok := seen[token]; ok {
				continue
			}
			seen[token] = struct{}{}
			state := counts[token]
			if state == nil {
				state = &clusterState{sources: map[string]struct{}{}}
				counts[token] = state
			}
			state.count++
			state.sources[item.Source] = struct{}{}
		}
	}
	type scoredCluster struct {
		label string
		state *clusterState
	}
	scored := make([]scoredCluster, 0, len(counts))
	for label, state := range counts {
		if state.count < 2 {
			continue
		}
		scored = append(scored, scoredCluster{label: label, state: state})
	}
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].state.count == scored[j].state.count {
			return scored[i].label < scored[j].label
		}
		return scored[i].state.count > scored[j].state.count
	})
	clusters := make([]map[string]interface{}, 0, minWebQueryInt(3, len(scored)))
	for _, entry := range scored[:minWebQueryInt(3, len(scored))] {
		sources := make([]string, 0, len(entry.state.sources))
		for source := range entry.state.sources {
			sources = append(sources, source)
		}
		sort.Strings(sources)
		clusters = append(clusters, map[string]interface{}{
			"label":      entry.label,
			"item_count": entry.state.count,
			"sources":    sources,
		})
	}
	return clusters
}

func summarizeWebRecentReport(items []webRecentNormalizedItem, clusters []map[string]interface{}) string {
	if len(items) == 0 {
		return "I couldn't gather dated public discussion from the last 30 days across the supported sources."
	}
	sourceSet := map[string]struct{}{}
	for _, item := range items {
		sourceSet[item.Source] = struct{}{}
	}
	sources := make([]string, 0, len(sourceSet))
	for source := range sourceSet {
		sources = append(sources, source)
	}
	sort.Strings(sources)
	if len(clusters) == 0 {
		return fmt.Sprintf("Collected %d recent items across %d public sources over the last 30 days.", len(items), len(sources))
	}
	labels := make([]string, 0, len(clusters))
	for _, cluster := range clusters {
		if label := strings.TrimSpace(asString(cluster["label"])); label != "" {
			labels = append(labels, label)
		}
	}
	return fmt.Sprintf("Collected %d recent items across %d public sources over the last 30 days. Main discussion clusters: %s.", len(items), len(sources), strings.Join(labels, ", "))
}

func webRecentPublishedAt(values ...string) (string, bool) {
	threshold := time.Now().UTC().AddDate(0, 0, -webRecentLookbackDays)
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if ts, ok := webRecentParseTime(trimmed); ok {
			if ts.Before(threshold) || ts.After(time.Now().UTC().Add(24*time.Hour)) {
				continue
			}
			return ts.UTC().Format(time.RFC3339), true
		}
		matches := webRecentDatePattern.FindAllString(trimmed, 4)
		for _, match := range matches {
			if ts, ok := webRecentParseTime(match); ok {
				if ts.Before(threshold) || ts.After(time.Now().UTC().Add(24*time.Hour)) {
					continue
				}
				return ts.UTC().Format(time.RFC3339), true
			}
		}
	}
	return "", false
}

func webRecentParseTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02",
		"2006/01/02",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006/01/02 15:04:05",
	}
	for _, layout := range layouts {
		if ts, err := time.Parse(layout, raw); err == nil {
			return ts.UTC(), true
		}
	}
	return time.Time{}, false
}

func compactWebRecentSnippet(raw string) string {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, "\n", " "))
	if raw == "" {
		return ""
	}
	raw = strings.Join(strings.Fields(raw), " ")
	if len([]rune(raw)) > 240 {
		return truncateRunes(raw, 240)
	}
	return raw
}

func splitWebRecentTokens(raw string) []string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return nil
	}
	cleaned := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= '0' && r <= '9':
			return r
		default:
			return ' '
		}
	}, raw)
	return strings.Fields(cleaned)
}

func webRecentHTTPJSON(ctx context.Context, targetURL string, headers map[string]string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return err
	}
	for key, value := range headers {
		if strings.TrimSpace(value) != "" {
			req.Header.Set(key, value)
		}
	}
	resp, err := webRecentHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("http %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}

func webRecentPolymarketEvents(payload interface{}) []map[string]interface{} {
	switch typed := payload.(type) {
	case []interface{}:
		events := make([]map[string]interface{}, 0, len(typed))
		for _, item := range typed {
			if event, ok := coerceCompatMap(item); ok {
				events = append(events, event)
			}
		}
		return events
	case map[string]interface{}:
		if rawEvents, ok := typed["events"].([]interface{}); ok {
			events := make([]map[string]interface{}, 0, len(rawEvents))
			for _, item := range rawEvents {
				if event, ok := coerceCompatMap(item); ok {
					events = append(events, event)
				}
			}
			return events
		}
	}
	return nil
}

func extractURLs(raw string) []string {
	matches := webRecentURLPattern.FindAllString(strings.TrimSpace(raw), -1)
	if len(matches) == 0 {
		return nil
	}
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		out = append(out, strings.TrimSpace(strings.TrimRight(match, ".,)")))
	}
	return out
}

func minFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
