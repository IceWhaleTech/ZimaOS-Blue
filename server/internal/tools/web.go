package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/routingcue"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/stt"
)

const (
	webQueryDepthQuick    = "quick"
	webQueryDepthStandard = "standard"
	webQueryDepthDeep     = "deep"

	webQueryStatusOK           = "ok"
	webQueryStatusPartial      = "partial"
	webQueryStatusNeedsBrowser = "needs_browser"
	webQueryStatusError        = "error"

	webQueryNextActionNone         = "none"
	webQueryNextActionRetryBrowser = "retry_browser"
	webQueryNextActionRefineQuery  = "refine_query"
	webQueryNextActionAuthorize    = "authorize_provider"

	webQuerySearchCardStatusSuccess = "success"
	webQuerySearchCardStatusPartial = "partial"
	webQuerySearchCardStatusEmpty   = "empty"

	webQuerySearchSettleWindow = 180 * time.Millisecond
	webQueryReadSettleWindow   = 140 * time.Millisecond
	webQueryProxyHedgeDelay    = 120 * time.Millisecond

	webQueryMediaModeOff  = "off"
	webQueryMediaModeAuto = "auto"
	webQueryMediaModeOCR  = "ocr"
	webQueryMediaModeFull = "full"

	defaultWebQueryMediaPrompt        = "Describe the most informative image(s) on this webpage for a chat assistant. Focus on visible text, charts, screenshots, diagrams, or product visuals, and highlight details that add context beyond the page text. Stay concise and do not speculate beyond what is visible."
	defaultWebQueryMediaMaxItems      = 1
	maxWebQueryMediaMaxItems          = 4
	webQueryMediaSummaryMaxChars      = 1600
	webQueryMediaItemAnalysisMaxChars = 900

	webQuerySearchCardEmptyMessage   = "No matching sources were found."
	webQuerySearchCardFailedMessage  = "Search failed before any sources could be listed."
	webQuerySearchCardPartialMessage = "Showing search summaries because readable page extraction was unavailable."
)

var (
	webQuerySiteHintPattern    = regexp.MustCompile(`(?i)\bsite:([^\s]+)`)
	webQueryFinanceHintPattern = regexp.MustCompile(`(?i)\b(stock(?:\s+price)?|share(?:\s+price)?|quote|ticker|market\s+summary)\b`)
	webQueryTickerParenPattern = regexp.MustCompile(`\(([A-Z]{1,5})\)`)
	webQueryTickerWordPattern  = regexp.MustCompile(`\b[A-Z]{1,5}\b`)
)

// WebTool provides a single public web-query surface with internal orchestration
// plus compatibility routing for the legacy web_* tools.
type WebTool struct {
	name        string
	description string
	search      Tool
	fetch       Tool
	read        Tool
	extract     Tool
	crawl       Tool
	browser     BrowserBackend
	image       Tool
	sttService  stt.Service
}

type webQueryEnvelope struct {
	Status            string              `json:"status"`
	Mode              string              `json:"mode"`
	Input             string              `json:"input"`
	Query             string              `json:"query"`
	Provider          string              `json:"provider,omitempty"`
	TargetURL         string              `json:"target_url"`
	FinalURL          string              `json:"final_url"`
	Title             string              `json:"title"`
	Content           string              `json:"content"`
	ContentFormat     string              `json:"content_format"`
	Sources           []webQuerySource    `json:"sources"`
	Warnings          []webQueryWarning   `json:"warnings"`
	NextAction        string              `json:"next_action"`
	SearchCardEmitted bool                `json:"search_card_emitted,omitempty"`
	Media             *webQueryMedia      `json:"media,omitempty"`
	Page              *webQueryPage       `json:"page,omitempty"`
	Transcript        *webQueryTranscript `json:"transcript,omitempty"`
	Diagnostics       webQueryDiagnostics `json:"diagnostics"`
}

type webQueryMediaItem struct {
	URL      string `json:"url"`
	Alt      string `json:"alt,omitempty"`
	Source   string `json:"source,omitempty"`
	Mode     string `json:"mode,omitempty"`
	Analysis string `json:"analysis,omitempty"`
}

type webQuerySource struct {
	Rank         int      `json:"rank"`
	Kind         string   `json:"kind,omitempty"`
	URL          string   `json:"url"`
	FinalURL     string   `json:"final_url"`
	Title        string   `json:"title"`
	Snippet      string   `json:"snippet"`
	Source       string   `json:"source"`
	ContentChars int      `json:"content_chars"`
	WarningCodes []string `json:"warning_codes"`
	Selected     bool     `json:"selected"`
}

type webQueryWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type webQueryDiagnostics struct {
	Route           string            `json:"route"`
	Attempts        []webQueryAttempt `json:"attempts"`
	CandidateCount  int               `json:"candidate_count"`
	SelectedSource  int               `json:"selected_source"`
	Degraded        bool              `json:"degraded"`
	StrategyUsed    string            `json:"strategy_used,omitempty"`
	SessionReused   bool              `json:"session_reused,omitempty"`
	AdapterID       string            `json:"adapter_id,omitempty"`
	NetworkObserved bool              `json:"network_observed,omitempty"`
	ChallengeState  *ChallengeState   `json:"challenge_state,omitempty"`
}

type webQueryAttempt struct {
	Stage        string   `json:"stage"`
	Mode         string   `json:"mode"`
	URL          string   `json:"url"`
	Status       string   `json:"status"`
	ContentChars int      `json:"content_chars"`
	WarningCodes []string `json:"warning_codes"`
	Error        string   `json:"error"`
}

type webQueryReadResult struct {
	Response     webReadResponse
	Attempts     []webQueryAttempt
	Warnings     []webQueryWarning
	NeedsBrowser bool
	Strong       bool
	HasSuccess   bool
}

type webQueryReuseCoordinator struct {
	mu                  sync.RWMutex
	browserTargetByHost map[string]string
}

type webQueryCandidate struct {
	Rank      int
	Search    WebSearchResult
	Resolved  webQueryReadResult
	Score     float64
	FromCrawl bool
}

type webQueryScoreProfile struct {
	QueryTokens   []string
	QueryTokenSet map[string]struct{}
}

type webQuerySearchProviderResolver interface {
	providerChain(raw interface{}) []string
}

type webQueryBrowserFallbackConfigResolver interface {
	browserFallbackConfig() WebSearchBrowserFallbackConfig
}

type webQuerySearchOutcome struct {
	Provider string
	Response WebSearchResponse
	Results  []WebSearchResult
	Err      error
}

type webQueryReadOutcome struct {
	Lane string
	Resp webReadResponse
	Err  error
	Skip bool
}

type webQueryMediaCandidate struct {
	URL    string
	Alt    string
	Source string
	Score  int
}

type webQueryMediaExtractResponse struct {
	Data map[string]interface{} `json:"data"`
}

type webQueryMediaAnalysisPlan struct {
	Candidates        []webQueryMediaCandidate
	ImageAnalysisMode string
	SkipReason        string
}

// NewWebQueryTool creates the primary visible web_query tool.
func NewWebQueryTool(search, fetch, read, extract, crawl Tool) *WebTool {
	return newWebTool(
		"web_query",
		"Unified web entry point for search or URL reading. Blue handles discovery, reading, retries, and warning/fallback envelopes automatically.",
		search, fetch, read, extract, crawl,
	)
}

func newWebTool(name, description string, search, fetch, read, extract, crawl Tool) *WebTool {
	return &WebTool{
		name:        name,
		description: description,
		search:      search,
		fetch:       fetch,
		read:        read,
		extract:     extract,
		crawl:       crawl,
	}
}

func (t *WebTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        t.name,
		Description: t.description,
		Icon:        "web-search",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"input": map[string]interface{}{
					"type":        "string",
					"description": "Required. Either a search query or a target URL.",
				},
				"depth": map[string]interface{}{
					"type":        "string",
					"description": "Planning depth for discovery and fallback behavior.",
					"enum":        []string{webQueryDepthQuick, webQueryDepthStandard, webQueryDepthDeep},
					"default":     webQueryDepthStandard,
				},
				"max_results": map[string]interface{}{
					"type":        "integer",
					"description": "Optional discovery cap used while finding candidate pages.",
				},
				"max_chars": map[string]interface{}{
					"type":        "integer",
					"description": "Optional maximum number of body characters returned in content.",
				},
				"media": map[string]interface{}{
					"type":        "object",
					"description": "Optional media enrichment config.",
					"properties": map[string]interface{}{
						"enabled":   map[string]interface{}{"type": "boolean", "description": "Enable webpage image analysis."},
						"mode":      map[string]interface{}{"type": "string", "enum": []string{webQueryMediaModeOff, webQueryMediaModeAuto, webQueryMediaModeOCR, webQueryMediaModeFull}, "description": "off, auto, ocr, or full"},
						"prompt":    map[string]interface{}{"type": "string", "description": "Optional image-analysis prompt."},
						"max_items": map[string]interface{}{"type": "integer", "description": "Maximum images to analyze."},
					},
				},
				"language": map[string]interface{}{
					"type":        "string",
					"description": "Optional preferred transcript/subtitle language hint.",
				},
				"allowed_hosts": map[string]interface{}{
					"type":        "array",
					"description": "Optional host allowlist used during discovery and lightweight crawl expansion.",
					"items":       map[string]interface{}{"type": "string"},
				},
				"request": map[string]interface{}{
					"type":        "object",
					"description": "Optional auth/session overrides for reading protected pages.",
					"properties": map[string]interface{}{
						"headers":           map[string]interface{}{"type": "object", "description": "Extra request headers.", "additionalProperties": map[string]interface{}{"type": "string"}},
						"cookies":           map[string]interface{}{"type": "string", "description": "Cookie header value."},
						"auth_bearer":       map[string]interface{}{"type": "string", "description": "Bearer token without or with Bearer prefix."},
						"browser_target_id": map[string]interface{}{"type": "string", "description": "Reuse cookies from an existing browser tab/session."},
					},
				},
			},
			"additionalProperties": true,
		},
	}
}

func (t *WebTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t == nil {
		return nil, errors.New("web tool is not available")
	}
	args = normalizeWebQueryArgs(args)

	if compatAction := inferWebCompatAction(args); compatAction != "" {
		return t.executeLegacyAction(ctx, compatAction, args)
	}

	input := strings.TrimSpace(resolveWebQueryInput(args))
	if input == "" {
		return nil, errors.New("input is required")
	}
	if strings.TrimSpace(firstCompatString(args, "retrieval_profile", "retrievalProfile")) == webRecentRetrievalProfile {
		return t.executeRecentMultiSiteQuery(ctx, args, input)
	}
	depth, err := parseWebQueryDepth(args)
	if err != nil {
		return nil, err
	}
	format, err := parseWebQueryFormat(args)
	if err != nil {
		return nil, err
	}
	maxChars := parseWebFetchMaxChars(args, webFetchDefaultMaxChars, webFetchDefaultMaxCharsCap)
	maxResults := parseWebQueryMaxResults(args, depth)
	allowedHosts := mergeWebQueryAllowedHosts(input, parseStringListArg(args, "allowed_hosts", "allowedHosts", "hosts"))

	envelope := newWebQueryEnvelope(input, format)
	if looksLikeWebQueryURL(input) {
		if looksLikeWebQueryVideoURL(input) {
			envelope = t.executeVideoURLQuery(ctx, args, input, format, maxChars)
		} else {
			envelope = t.executeURLQuery(ctx, args, input, format, maxChars)
		}
	} else {
		if canonicalURL, ok := canonicalWebQueryURL(input); ok {
			envelope = t.executeCanonicalURLQuery(ctx, args, input, canonicalURL, format, maxChars)
		} else {
			envelope = t.executeSearchQuery(ctx, args, input, depth, format, maxChars, maxResults, allowedHosts)
		}
	}
	return marshalWebQueryEnvelope(envelope)
}

func (t *WebTool) executeLegacyAction(ctx context.Context, action string, args map[string]interface{}) (interface{}, error) {
	switch action {
	case "search":
		if t.search == nil {
			return nil, errors.New("web search is not available")
		}
		return t.search.Execute(ctx, normalizeWebSearchCompatArgs("web_query", args))
	case "fetch":
		if t.fetch == nil {
			return nil, errors.New("web fetch is not available")
		}
		return t.fetch.Execute(ctx, normalizeWebFetchCompatArgs("web_query", args))
	case "read":
		if t.read == nil {
			return nil, errors.New("web read is not available")
		}
		return t.read.Execute(ctx, normalizeWebReadCompatArgs(args))
	case "extract":
		if t.extract == nil {
			return nil, errors.New("web extract is not available")
		}
		return t.extract.Execute(ctx, normalizeWebExtractCompatArgs(args))
	case "crawl":
		if t.crawl == nil {
			return nil, errors.New("web crawl is not available")
		}
		return t.crawl.Execute(ctx, normalizeWebCrawlCompatArgs(args))
	default:
		return nil, fmt.Errorf("unknown web action: %s", action)
	}
}

func (t *WebTool) executeURLQuery(ctx context.Context, args map[string]interface{}, input, format string, maxChars int) webQueryEnvelope {
	envelope := newWebQueryEnvelope(input, format)
	envelope.Mode = "read"
	envelope.TargetURL = input
	envelope.Diagnostics.Route = "url_read"

	readResult, err := t.runReadPipeline(ctx, args, input, format, maxChars, nil)
	envelope.Diagnostics.Attempts = append(envelope.Diagnostics.Attempts, readResult.Attempts...)
	envelope.Diagnostics.Degraded = len(readResult.Attempts) > 1
	if err != nil {
		addWebQueryWarning(&envelope.Warnings, "read_failed", err.Error())
		envelope.Status = webQueryStatusError
		envelope.NextAction = webQueryNextActionNone
		return envelope
	}
	if !readResult.HasSuccess {
		addWebQueryWarning(&envelope.Warnings, "read_failed", "no readable content could be extracted")
		envelope.Status = webQueryStatusError
		envelope.NextAction = webQueryNextActionNone
		return envelope
	}

	envelope = applyResolvedReadToEnvelope(envelope, readResult, 1)
	envelope.Diagnostics.CandidateCount = 1
	envelope.Diagnostics.SelectedSource = 1
	t.enrichWebQueryEnvelopeWithMedia(ctx, args, &envelope)
	return envelope
}

func (t *WebTool) executeSearchQuery(ctx context.Context, args map[string]interface{}, query, depth, format string, maxChars, maxResults int, allowedHosts []string) webQueryEnvelope {
	envelope := newWebQueryEnvelope(query, format)
	envelope.Query = query
	envelope.Mode = "search"
	envelope.Diagnostics.Route = "search_http"
	scoreProfile := newWebQueryScoreProfile(query)

	if financeCandidate, ok := detectWebQueryFinanceFastPath(query); ok {
		fastCandidates := buildWebQueryCandidates([]WebSearchResult{financeCandidate}, allowedHosts)
		if len(fastCandidates) > 0 {
			fastAttempts := t.resolveWebQueryCandidates(ctx, args, scoreProfile, format, maxChars, fastCandidates, 1)
			envelope.Diagnostics.Attempts = append(envelope.Diagnostics.Attempts, fastAttempts...)
			envelope.Diagnostics.Degraded = len(fastAttempts) > 1
			if fastCandidates[0].Resolved.HasSuccess && fastCandidates[0].Resolved.Strong {
				selected := fastCandidates[0]
				envelope.Mode = "search_read"
				envelope.Diagnostics.Route = "search_finance_fast_path"
				envelope.Diagnostics.CandidateCount = len(fastCandidates)
				envelope.Diagnostics.SelectedSource = selected.Rank
				envelope = applyResolvedReadToEnvelope(envelope, selected.Resolved, selected.Rank)
				envelope.Sources = buildWebQuerySources(fastCandidates, strings.TrimSpace(selected.Search.URL))
				emitWebQuerySearchCard(ctx, &envelope, webQuerySearchCardStatusSuccess, "", strings.TrimSpace(selected.Search.URL))
				t.enrichWebQueryEnvelopeWithMedia(ctx, args, &envelope)
				return envelope
			}
		}
	}

	if t.search == nil {
		addWebQueryWarning(&envelope.Warnings, "search_unavailable", "web search is not available")
		envelope.Status = webQueryStatusError
		return envelope
	}

	fallbackCfg := t.webQueryBrowserFallbackConfig()
	browserRetries := 0

	searchResp, searchAttempts, err := t.runSearchDiscovery(ctx, args, query, maxResults)
	envelope.Diagnostics.Attempts = append(envelope.Diagnostics.Attempts, searchAttempts...)
	envelope.Provider = strings.TrimSpace(searchResp.Provider)

	candidates := []webQueryCandidate{}
	if err == nil {
		candidates = buildWebQueryCandidates(searchResp.Results, allowedHosts)
	}

	if err != nil && canAttemptBrowserSearchFallback(t.browser, fallbackCfg, browserRetries) {
		browserRetries++
		browserResp, browserAttempt, browserErr := t.runBrowserSearchDiscovery(ctx, query, maxResults, fallbackCfg)
		envelope.Diagnostics.Attempts = append(envelope.Diagnostics.Attempts, browserAttempt)
		envelope.Diagnostics.Degraded = true
		if browserErr == nil {
			envelope.Diagnostics.Route = "search_browser"
			searchResp = browserResp
			envelope.Provider = strings.TrimSpace(searchResp.Provider)
			err = nil
			candidates = buildWebQueryCandidates(searchResp.Results, allowedHosts)
		}
	}

	if err == nil && len(candidates) == 0 && canAttemptBrowserSearchFallback(t.browser, fallbackCfg, browserRetries) {
		browserRetries++
		browserResp, browserAttempt, browserErr := t.runBrowserSearchDiscovery(ctx, query, maxResults, fallbackCfg)
		envelope.Diagnostics.Attempts = append(envelope.Diagnostics.Attempts, browserAttempt)
		envelope.Diagnostics.Degraded = true
		if browserErr == nil {
			envelope.Diagnostics.Route = "search_browser"
			searchResp = browserResp
			envelope.Provider = strings.TrimSpace(searchResp.Provider)
			candidates = buildWebQueryCandidates(searchResp.Results, allowedHosts)
		}
	}

	envelope.Diagnostics.CandidateCount = len(candidates)
	if err != nil && len(candidates) == 0 {
		addWebQueryWarning(&envelope.Warnings, "search_failed", err.Error())
		envelope.Status = webQueryStatusPartial
		envelope.NextAction = webQueryNextActionRefineQuery
		emitWebQuerySearchCard(ctx, &envelope, webQuerySearchCardStatusEmpty, webQuerySearchCardFailedMessage, "")
		return envelope
	}
	if len(candidates) == 0 {
		addWebQueryWarning(&envelope.Warnings, "no_results", "no matching sources were found")
		envelope.Status = webQueryStatusPartial
		envelope.NextAction = webQueryNextActionRefineQuery
		emitWebQuerySearchCard(ctx, &envelope, webQuerySearchCardStatusEmpty, webQuerySearchCardEmptyMessage, "")
		return envelope
	}

	for idx := range candidates {
		candidates[idx].Score = scoreWebQueryCandidateWithProfile(scoreProfile, candidates[idx])
	}
	if looksLikeWebQueryTranscriptIntentQuery(query) {
		if videoEnvelope, ok := t.tryExecuteVideoSearchQuery(ctx, args, query, format, maxChars, candidates); ok {
			videoEnvelope.Diagnostics.Attempts = append(envelope.Diagnostics.Attempts, videoEnvelope.Diagnostics.Attempts...)
			videoEnvelope.Diagnostics.CandidateCount = len(candidates)
			if videoEnvelope.Diagnostics.Route == "" {
				videoEnvelope.Diagnostics.Route = "search_video"
			}
			if videoEnvelope.Provider == "" {
				videoEnvelope.Provider = envelope.Provider
			}
			if len(videoEnvelope.Sources) > 0 {
				emitWebQuerySearchCard(ctx, &videoEnvelope, searchCardStatusFromEnvelope(videoEnvelope.Status), "", selectedSearchURL(videoEnvelope.Sources))
			}
			return videoEnvelope
		}
	}

	readLimit := minWebQueryInt(parseWebQueryCandidateReadLimit(depth), len(candidates))
	readAttempts := t.resolveWebQueryCandidates(ctx, args, scoreProfile, format, maxChars, candidates, readLimit)
	envelope.Diagnostics.Attempts = append(envelope.Diagnostics.Attempts, readAttempts...)
	if reason, shouldFallback := shouldFallbackToBrowserSearch(candidates, readLimit, fallbackCfg.QualityThreshold); shouldFallback && canAttemptBrowserSearchFallback(t.browser, fallbackCfg, browserRetries) {
		browserRetries++
		browserResp, browserAttempt, browserErr := t.runBrowserSearchDiscovery(ctx, query, maxResults, fallbackCfg)
		envelope.Diagnostics.Attempts = append(envelope.Diagnostics.Attempts, browserAttempt)
		envelope.Diagnostics.Degraded = true
		if browserErr == nil {
			envelope.Diagnostics.Route = "search_browser"
			candidates = mergeWebQueryFallbackCandidates(browserResp, searchResp, allowedHosts, candidates, scoreProfile, maxResults)
			envelope.Diagnostics.CandidateCount = len(candidates)
			readAttempts = t.resolveWebQueryCandidates(ctx, args, scoreProfile, format, maxChars, candidates, readLimit)
			envelope.Diagnostics.Attempts = append(envelope.Diagnostics.Attempts, readAttempts...)
		} else {
			addWebQueryWarning(&envelope.Warnings, "search_browser_failed", fmt.Sprintf("%s: %s", reason, browserErr.Error()))
		}
	}

	bestIndex, bestScore := selectBestWebQueryCandidate(candidates)

	crawlUsed := false
	if depth == webQueryDepthDeep && len(allowedHosts) > 0 && t.crawl != nil {
		crawlCandidate, crawlAttempts := t.expandWebQueryWithCrawl(ctx, args, scoreProfile, format, maxChars, maxResults, allowedHosts, candidates)
		envelope.Diagnostics.Attempts = append(envelope.Diagnostics.Attempts, crawlAttempts...)
		if crawlCandidate != nil {
			crawlUsed = true
			if crawlCandidate.Score > bestScore {
				bestScore = crawlCandidate.Score
				candidates = append(candidates, *crawlCandidate)
				bestIndex = len(candidates) - 1
			}
		}
	}

	selectedURL := ""
	selectedFromCrawl := false
	if bestIndex >= 0 {
		selectedURL = strings.TrimSpace(candidates[bestIndex].Search.URL)
		selectedFromCrawl = candidates[bestIndex].FromCrawl
	}

	sortWebQueryCandidatesByScore(candidates)

	sources, selectedRank := buildWebQuerySourcesFromSortedCandidatesWithSelectedRank(candidates, selectedURL)
	if selectedRank > 0 {
		bestIndex = selectedRank - 1
	} else {
		bestIndex = -1
	}

	if bestIndex < 0 {
		addWebQueryWarning(&envelope.Warnings, "no_results", "no matching sources were found")
		envelope.Status = webQueryStatusPartial
		envelope.NextAction = webQueryNextActionRefineQuery
		return envelope
	}

	selected := candidates[bestIndex]
	envelope.Mode = ternary(selectedFromCrawl, "crawl_read", ternary(selected.Resolved.HasSuccess, "search_read", "search"))
	if selected.Resolved.HasSuccess {
		envelope = applyResolvedReadToEnvelope(envelope, selected.Resolved, selected.Rank)
		t.enrichWebQueryEnvelopeWithMedia(ctx, args, &envelope)
	} else {
		envelope.Title = strings.TrimSpace(selected.Search.Title)
		envelope.TargetURL = strings.TrimSpace(selected.Search.URL)
		envelope.FinalURL = strings.TrimSpace(selected.Search.URL)
		envelope.Content = strings.TrimSpace(selected.Search.Description)
		envelope.ContentFormat = webReadFormatText
		envelope.Status = webQueryStatusPartial
		envelope.NextAction = webQueryNextActionRefineQuery
	}
	envelope.Sources = sources
	envelope.Diagnostics.SelectedSource = selectedRank
	envelope.Diagnostics.Degraded = envelope.Diagnostics.Degraded || crawlUsed || len(envelope.Diagnostics.Attempts) > 1
	if envelope.Status == "" {
		envelope.Status = webQueryStatusPartial
	}
	if envelope.NextAction == "" {
		envelope.NextAction = webQueryNextActionNone
	}
	searchCardMessage := ""
	if !selected.Resolved.HasSuccess {
		searchCardMessage = webQuerySearchCardPartialMessage
	}
	emitWebQuerySearchCard(ctx, &envelope, searchCardStatusFromEnvelope(envelope.Status), searchCardMessage, strings.TrimSpace(selected.Search.URL))
	return envelope
}

func (t *WebTool) runSearchDiscovery(ctx context.Context, args map[string]interface{}, query string, maxResults int) (WebSearchResponse, []webQueryAttempt, error) {
	baseArgs := normalizeWebSearchCompatArgs("web_query", args)
	baseArgs["query"] = query
	baseArgs["format"] = webSearchFormatJSON
	baseArgs["max_results"] = maxResults

	providerRaw := firstCompatValueOrNil(args, "provider")
	providers := parseProviderChainArg(providerRaw)
	if len(providers) == 0 {
		if resolver, ok := t.search.(webQuerySearchProviderResolver); ok {
			providers = resolver.providerChain(providerRaw)
		}
	}
	if len(providers) <= 1 {
		outcome := t.executeSearchProvider(ctx, baseArgs, firstNonEmpty(strings.TrimSpace(asString(providerRaw)), firstProviderOrEmpty(providers)))
		attempts := []webQueryAttempt{{
			Stage:  "search",
			Mode:   firstNonEmpty(outcome.Provider, "search"),
			URL:    query,
			Status: webQueryAttemptStatus(outcome.Err),
			Error:  errorString(outcome.Err),
		}}
		if outcome.Err != nil {
			return WebSearchResponse{}, attempts, outcome.Err
		}
		return outcome.Response, attempts, nil
	}

	searchCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make(chan webQuerySearchOutcome, len(providers))
	for _, provider := range providers {
		provider := provider
		go func() {
			results <- t.executeSearchProvider(searchCtx, baseArgs, provider)
		}()
	}

	var (
		attempts    []webQueryAttempt
		outcomes    []webQuerySearchOutcome
		searchErrs  []error
		settleTimer *time.Timer
	)
	for remaining := len(providers); remaining > 0; {
		var settle <-chan time.Time
		if settleTimer != nil {
			settle = settleTimer.C
		}

		select {
		case outcome := <-results:
			remaining--
			if outcome.Err != nil {
				// Hide provider attempts that were only canceled because another
				// provider already succeeded and the fanout settle window closed.
				if len(outcomes) > 0 && errors.Is(outcome.Err, context.Canceled) {
					continue
				}
				attempts = append(attempts, webQueryAttempt{
					Stage:  "search",
					Mode:   firstNonEmpty(outcome.Provider, "search"),
					URL:    query,
					Status: webQueryAttemptStatus(outcome.Err),
					Error:  errorString(outcome.Err),
				})
				searchErrs = append(searchErrs, outcome.Err)
				continue
			}
			attempts = append(attempts, webQueryAttempt{
				Stage:  "search",
				Mode:   firstNonEmpty(outcome.Provider, "search"),
				URL:    query,
				Status: webQueryAttemptStatus(outcome.Err),
				Error:  errorString(outcome.Err),
			})
			outcomes = append(outcomes, outcome)
			if len(outcome.Results) > 0 && settleTimer == nil && remaining > 0 {
				settleTimer = time.NewTimer(webQuerySearchSettleWindow)
			}
		case <-settle:
			cancel()
			settleTimer = nil
		}
	}
	if settleTimer != nil {
		settleTimer.Stop()
	}
	if len(outcomes) == 0 {
		return WebSearchResponse{}, attempts, firstNonNilErr(searchErrs...)
	}
	attempts = hideCanceledSearchAttemptsAfterSearchSuccess(attempts)
	return mergeWebQuerySearchOutcomes(query, maxResults, outcomes), attempts, nil
}

func (t *WebTool) executeSearchProvider(ctx context.Context, baseArgs map[string]interface{}, provider string) webQuerySearchOutcome {
	searchArgs := cloneStringAnyMap(baseArgs)
	if strings.TrimSpace(provider) != "" {
		searchArgs["provider"] = strings.TrimSpace(provider)
	}
	raw, err := t.search.Execute(ctx, searchArgs)
	if err != nil {
		return webQuerySearchOutcome{Provider: strings.TrimSpace(provider), Err: err}
	}

	var searchResp WebSearchResponse
	if err := decodeToolJSONResult(raw, &searchResp); err != nil {
		return webQuerySearchOutcome{Provider: strings.TrimSpace(provider), Err: err}
	}
	effectiveProvider := firstNonEmpty(strings.TrimSpace(searchResp.Provider), strings.TrimSpace(provider))
	return webQuerySearchOutcome{
		Provider: effectiveProvider,
		Response: searchResp,
		Results:  append([]WebSearchResult(nil), searchResp.Results...),
	}
}

func mergeWebQuerySearchOutcomes(query string, maxResults int, outcomes []webQuerySearchOutcome) WebSearchResponse {
	type mergedResult struct {
		Result        WebSearchResult
		Hits          int
		BestRank      int
		BestProvider  int
		AggregateRank float64
	}

	merged := make(map[string]*mergedResult, len(outcomes)*maxWebQueryInt(maxResults, 1))
	providers := make([]string, 0, len(outcomes))
	for providerIdx, outcome := range outcomes {
		providerName := firstNonEmpty(strings.TrimSpace(outcome.Provider), strings.TrimSpace(outcome.Response.Provider))
		if providerName != "" {
			providers = append(providers, providerName)
		}
		for resultIdx, result := range outcome.Results {
			targetURL := strings.TrimSpace(result.URL)
			if targetURL == "" {
				continue
			}
			canonical, err := canonicalizeCrawlURL(targetURL)
			if err != nil {
				canonical = targetURL
			}
			entry, ok := merged[canonical]
			if !ok {
				entry = &mergedResult{
					Result:       result,
					Hits:         0,
					BestRank:     resultIdx + 1,
					BestProvider: providerIdx,
				}
				if strings.TrimSpace(entry.Result.Source) == "" {
					entry.Result.Source = providerName
				}
				merged[canonical] = entry
			}
			entry.Hits++
			entry.AggregateRank += float64((len(outcomes)-providerIdx)*32) + float64(maxWebQueryInt(maxResults-resultIdx, 1))*9
			if providerIdx < entry.BestProvider || (providerIdx == entry.BestProvider && resultIdx+1 < entry.BestRank) {
				entry.BestProvider = providerIdx
				entry.BestRank = resultIdx + 1
				entry.Result = result
				if strings.TrimSpace(entry.Result.Source) == "" {
					entry.Result.Source = providerName
				}
			} else {
				if strings.TrimSpace(entry.Result.Title) == "" && strings.TrimSpace(result.Title) != "" {
					entry.Result.Title = result.Title
				}
				if strings.TrimSpace(entry.Result.Description) == "" && strings.TrimSpace(result.Description) != "" {
					entry.Result.Description = result.Description
				}
			}
		}
	}

	ranked := make([]mergedResult, 0, len(merged))
	for _, entry := range merged {
		ranked = append(ranked, *entry)
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].Hits == ranked[j].Hits {
			if ranked[i].AggregateRank == ranked[j].AggregateRank {
				if ranked[i].BestProvider == ranked[j].BestProvider {
					return ranked[i].BestRank < ranked[j].BestRank
				}
				return ranked[i].BestProvider < ranked[j].BestProvider
			}
			return ranked[i].AggregateRank > ranked[j].AggregateRank
		}
		return ranked[i].Hits > ranked[j].Hits
	})

	limit := minWebQueryInt(maxResults, len(ranked))
	results := make([]WebSearchResult, 0, limit)
	for idx := 0; idx < limit; idx++ {
		results = append(results, ranked[idx].Result)
	}
	return WebSearchResponse{
		Query:      query,
		Results:    results,
		TotalCount: len(results),
		Provider:   strings.Join(providers, ","),
	}
}

func (t *WebTool) webQueryBrowserFallbackConfig() WebSearchBrowserFallbackConfig {
	if resolver, ok := t.search.(webQueryBrowserFallbackConfigResolver); ok {
		return resolver.browserFallbackConfig()
	}
	return normalizeWebSearchBrowserFallbackConfig(WebSearchBrowserFallbackConfig{})
}

func canAttemptBrowserSearchFallback(browser BrowserBackend, cfg WebSearchBrowserFallbackConfig, retries int) bool {
	if browser == nil || retries >= cfg.MaxBrowserRetries {
		return false
	}
	return cfg.Enabled == nil || *cfg.Enabled
}

func (t *WebTool) runBrowserSearchDiscovery(ctx context.Context, query string, maxResults int, cfg WebSearchBrowserFallbackConfig) (WebSearchResponse, webQueryAttempt, error) {
	attempt := webQueryAttempt{
		Stage: "search_browser",
		Mode:  normalizeBrowserSearchFallbackEngine(cfg.Engine),
		URL:   query,
	}
	if t.browser == nil {
		err := errors.New("browser search fallback unavailable")
		attempt.Status = webQueryAttemptStatus(err)
		attempt.Error = err.Error()
		return WebSearchResponse{}, attempt, err
	}
	if err := t.browser.Start(ctx); err != nil {
		attempt.Status = webQueryAttemptStatus(err)
		attempt.Error = err.Error()
		return WebSearchResponse{}, attempt, err
	}

	recipeResult, err := t.browser.ExecuteRecipe(ctx, "search", map[string]string{
		"query":       query,
		"engine":      normalizeBrowserSearchFallbackEngine(cfg.Engine),
		"max_results": strconv.Itoa(maxResults),
	})
	if err != nil {
		attempt.Status = webQueryAttemptStatus(err)
		attempt.Error = err.Error()
		return WebSearchResponse{}, attempt, err
	}
	if !recipeResult.Success {
		err = errors.New(firstNonEmpty(strings.TrimSpace(recipeResult.Message), "browser search failed"))
		attempt.Status = webQueryAttemptStatus(err)
		attempt.Error = err.Error()
		return WebSearchResponse{}, attempt, err
	}

	var payload struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Snippet string `json:"snippet"`
		} `json:"results"`
		Engine string `json:"engine"`
	}
	if err := decodeToolJSONResult(recipeResult.Data, &payload); err != nil {
		attempt.Status = webQueryAttemptStatus(err)
		attempt.Error = err.Error()
		return WebSearchResponse{}, attempt, err
	}

	engine := normalizeBrowserSearchFallbackEngine(firstNonEmpty(payload.Engine, cfg.Engine))
	results := make([]WebSearchResult, 0, len(payload.Results))
	for _, result := range payload.Results {
		results = append(results, WebSearchResult{
			Title:       strings.TrimSpace(result.Title),
			URL:         normalizeWebQueryCandidateURL(strings.TrimSpace(result.URL)),
			Description: strings.TrimSpace(result.Snippet),
			Source:      "browser:" + engine,
		})
	}

	attempt.Status = "ok"
	return WebSearchResponse{
		Query:      query,
		Results:    results,
		TotalCount: len(results),
		Provider:   "browser:" + engine,
	}, attempt, nil
}

func mergeWebQuerySearchResponses(primary, secondary WebSearchResponse, maxResults int) WebSearchResponse {
	combined := append([]WebSearchResult(nil), primary.Results...)
	combined = append(combined, secondary.Results...)
	mergedResults := buildWebQueryCandidates(combined, nil)
	limit := len(mergedResults)
	if maxResults > 0 {
		limit = minWebQueryInt(maxResults, len(mergedResults))
	}
	results := make([]WebSearchResult, 0, limit)
	for idx := 0; idx < limit; idx++ {
		results = append(results, mergedResults[idx].Search)
	}
	return WebSearchResponse{
		Query:      firstNonEmpty(strings.TrimSpace(primary.Query), strings.TrimSpace(secondary.Query)),
		Results:    results,
		TotalCount: len(results),
		Provider:   strings.Trim(strings.Join([]string{strings.TrimSpace(primary.Provider), strings.TrimSpace(secondary.Provider)}, ","), ","),
	}
}

func mergeWebQueryFallbackCandidates(primary, secondary WebSearchResponse, allowedHosts []string, previous []webQueryCandidate, scoreProfile webQueryScoreProfile, maxResults int) []webQueryCandidate {
	return buildWebQueryFallbackCandidates(primary.Results, secondary.Results, allowedHosts, previous, scoreProfile, maxResults)
}

func buildWebQueryFallbackCandidates(primary, secondary []WebSearchResult, allowedHosts []string, previous []webQueryCandidate, scoreProfile webQueryScoreProfile, maxResults int) []webQueryCandidate {
	totalResults := len(primary) + len(secondary)
	if totalResults == 0 {
		return nil
	}

	capacity := totalResults
	if maxResults > 0 && capacity > maxResults {
		capacity = maxResults
	}
	merged := make([]webQueryCandidate, 0, capacity)
	seen := make(map[string]struct{}, capacity)
	previousByURL := webQueryCandidateStateByURL(previous)

	appendResult := func(result WebSearchResult, rank int) bool {
		targetURL, canonical, ok := webQueryCandidateURLKey(result.URL)
		if !ok {
			return false
		}
		if _, exists := seen[canonical]; exists {
			return false
		}
		seen[canonical] = struct{}{}
		result.URL = targetURL
		candidate := webQueryCandidate{
			Rank:   rank,
			Search: result,
		}
		if prior, ok := previousByURL[canonical]; ok {
			candidate.Resolved = prior.Resolved
		}
		merged = append(merged, candidate)
		return maxResults > 0 && len(merged) >= maxResults
	}

	rank := 0
	for _, result := range primary {
		rank++
		if appendResult(result, rank) {
			break
		}
	}
	if !(maxResults > 0 && len(merged) >= maxResults) {
		for _, result := range secondary {
			rank++
			if appendResult(result, rank) {
				break
			}
		}
	}
	if len(merged) == 0 {
		return nil
	}
	if len(allowedHosts) > 0 {
		filtered := merged[:0]
		for _, candidate := range merged {
			if !crawlHostAllowed(candidate.Search.URL, allowedHosts) {
				continue
			}
			filtered = append(filtered, candidate)
		}
		merged = filtered
	}
	if len(merged) == 0 {
		return nil
	}
	for idx := range merged {
		merged[idx].Score = scoreWebQueryCandidateWithProfile(scoreProfile, merged[idx])
	}
	return merged
}

func rebuildWebQueryCandidates(results []WebSearchResult, allowedHosts []string, previous []webQueryCandidate, query string) []webQueryCandidate {
	candidates := buildWebQueryCandidates(results, allowedHosts)
	if len(candidates) == 0 {
		return candidates
	}
	scoreProfile := newWebQueryScoreProfile(query)
	previousByURL := webQueryCandidateStateByURL(previous)
	for idx := range candidates {
		_, canonical, ok := webQueryCandidateURLKey(candidates[idx].Search.URL)
		if !ok {
			continue
		}
		if previous, ok := previousByURL[canonical]; ok {
			candidates[idx].Resolved = previous.Resolved
		}
		candidates[idx].Score = scoreWebQueryCandidateWithProfile(scoreProfile, candidates[idx])
	}
	return candidates
}

func hasResolvedWebQueryCandidate(candidate webQueryCandidate) bool {
	if candidate.Resolved.HasSuccess || len(candidate.Resolved.Attempts) > 0 {
		return true
	}
	if strings.TrimSpace(candidate.Resolved.Response.URL) != "" {
		return true
	}
	return false
}

func shouldFallbackToBrowserSearch(candidates []webQueryCandidate, readLimit int, qualityThreshold float64) (string, bool) {
	if len(candidates) == 0 {
		return "no_results", true
	}
	bestIndex, bestScore := selectBestWebQueryCandidate(candidates)
	if bestIndex < 0 {
		return "no_results", true
	}
	strongReads := 0
	considered := minWebQueryInt(readLimit, len(candidates))
	for idx := 0; idx < considered; idx++ {
		if !hasResolvedWebQueryCandidate(candidates[idx]) {
			continue
		}
		if candidates[idx].Resolved.Strong {
			strongReads++
		}
	}
	if considered > 0 && strongReads == 0 {
		return "unreadable_candidates", true
	}
	if !candidates[bestIndex].Resolved.HasSuccess && bestScore < qualityThreshold {
		return "low_confidence_snippets", true
	}
	return "", false
}

func selectBestWebQueryCandidate(candidates []webQueryCandidate) (int, float64) {
	bestIndex := -1
	bestScore := math.Inf(-1)
	for idx := range candidates {
		if candidates[idx].Score > bestScore {
			bestScore = candidates[idx].Score
			bestIndex = idx
		}
	}
	return bestIndex, bestScore
}

func (t *WebTool) resolveWebQueryCandidates(ctx context.Context, args map[string]interface{}, scoreProfile webQueryScoreProfile, format string, maxChars int, candidates []webQueryCandidate, readLimit int) []webQueryAttempt {
	if readLimit <= 0 {
		return nil
	}

	type candidateOutcome struct {
		Index int
		Read  webQueryReadResult
	}

	results := make(chan candidateOutcome, readLimit)
	var wg sync.WaitGroup
	reuse := newWebQueryReuseCoordinator()
	for idx := 0; idx < readLimit; idx++ {
		if hasResolvedWebQueryCandidate(candidates[idx]) {
			continue
		}
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			readResult, _ := t.runReadPipeline(ctx, args, candidates[idx].Search.URL, format, maxChars, reuse)
			results <- candidateOutcome{Index: idx, Read: readResult}
		}(idx)
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	attemptByIndex := make([][]webQueryAttempt, readLimit)
	for outcome := range results {
		candidates[outcome.Index].Resolved = outcome.Read
		candidates[outcome.Index].Score = scoreWebQueryCandidateWithProfile(scoreProfile, candidates[outcome.Index])
		attemptByIndex[outcome.Index] = append(attemptByIndex[outcome.Index], outcome.Read.Attempts...)
	}

	attempts := make([]webQueryAttempt, 0, readLimit*2)
	for idx := 0; idx < readLimit; idx++ {
		attempts = append(attempts, attemptByIndex[idx]...)
	}
	return attempts
}

func (t *WebTool) expandWebQueryWithCrawl(ctx context.Context, args map[string]interface{}, scoreProfile webQueryScoreProfile, format string, maxChars, maxResults int, allowedHosts []string, candidates []webQueryCandidate) (*webQueryCandidate, []webQueryAttempt) {
	attempts := []webQueryAttempt{}
	if t.crawl == nil {
		return nil, attempts
	}
	seeds := make([]string, 0, 2)
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		seed := strings.TrimSpace(candidate.Search.URL)
		if seed == "" || !crawlHostAllowed(seed, allowedHosts) {
			continue
		}
		canonical, err := canonicalizeCrawlURL(seed)
		if err != nil {
			continue
		}
		if _, ok := seen[canonical]; ok {
			continue
		}
		seen[canonical] = struct{}{}
		seeds = append(seeds, canonical)
		if len(seeds) >= 2 {
			break
		}
	}
	if len(seeds) == 0 {
		return nil, attempts
	}

	crawlArgs := normalizeWebCrawlCompatArgs(args)
	crawlArgs["seeds"] = seeds
	crawlArgs["allowed_hosts"] = allowedHosts
	crawlArgs["max_depth"] = 1
	crawlArgs["max_pages"] = minWebQueryInt(maxResults, 4)
	crawlArgs["page_max_chars"] = minWebQueryInt(maxChars, 4000)
	crawlArgs["lane"] = webAccessLaneHTTP

	raw, err := t.crawl.Execute(ctx, crawlArgs)
	attempt := webQueryAttempt{
		Stage:  "crawl",
		Mode:   "crawl",
		URL:    strings.Join(seeds, ","),
		Status: ternary(err == nil, "ok", "error"),
		Error:  errorString(err),
	}
	attempts = append(attempts, attempt)
	if err != nil {
		return nil, attempts
	}

	var crawlResp webCrawlResponse
	if err := decodeToolJSONResult(raw, &crawlResp); err != nil {
		attempts = append(attempts, webQueryAttempt{
			Stage:  "crawl",
			Mode:   "crawl",
			URL:    strings.Join(seeds, ","),
			Status: "error",
			Error:  err.Error(),
		})
		return nil, attempts
	}
	if len(crawlResp.Pages) == 0 {
		return nil, attempts
	}

	best := &webQueryCandidate{}
	best.Score = math.Inf(-1)
	for _, page := range crawlResp.Pages {
		if strings.TrimSpace(page.Content) == "" {
			continue
		}
		candidate := webQueryCandidate{
			Rank: 0,
			Search: WebSearchResult{
				Title:       page.Title,
				URL:         firstNonEmpty(page.FinalURL, page.URL),
				Description: strings.TrimSpace(page.Content),
				Source:      page.Source,
			},
			Resolved: webQueryReadResult{
				Response: webReadResponse{
					URL:          page.URL,
					Title:        page.Title,
					FinalURL:     page.FinalURL,
					Format:       format,
					Content:      page.Content,
					Source:       firstNonEmpty(page.Source, webAccessSourceHTTP),
					Warnings:     append([]string(nil), page.Warnings...),
					WarningCodes: nil,
					StatusCode:   page.StatusCode,
					Truncated:    page.Truncated,
				},
				HasSuccess: true,
				Strong:     len([]rune(strings.TrimSpace(page.Content))) >= webFetchMinReadableChars,
				Warnings:   warningsFromLists(nil, page.Warnings),
			},
			FromCrawl: true,
		}
		candidate.Score = scoreWebQueryCandidateWithProfile(scoreProfile, candidate) + 4
		if candidate.Score > best.Score {
			copyCandidate := candidate
			best = &copyCandidate
		}
	}
	if best == nil || math.IsInf(best.Score, -1) {
		return nil, attempts
	}
	return best, attempts
}

func (t *WebTool) runReadPipeline(ctx context.Context, args map[string]interface{}, targetURL, format string, maxChars int, reuse *webQueryReuseCoordinator) (webQueryReadResult, error) {
	result := webQueryReadResult{Attempts: []webQueryAttempt{}, Warnings: []webQueryWarning{}}
	if t.read == nil {
		return result, errors.New("web read is not available")
	}

	lane, err := parseWebAccessLane(args)
	if err != nil {
		return result, err
	}
	if lane == "" {
		lane = webAccessLaneAuto
	}
	if lane == webAccessLaneAuto {
		if preferredLane := t.preferredReadLane(ctx, args, targetURL); preferredLane != "" {
			lane = preferredLane
		}
	}

	allowedProxy := lane == webAccessLaneHTTP || lane == webAccessLaneAuto || lane == webAccessLaneProxyFetcher
	allowedBrowser := lane == webAccessLaneHTTP || lane == webAccessLaneAuto || lane == webAccessLaneLightpandaShim || lane == webAccessLaneProxyFetcher || lane == webAccessLaneBrowser

	bestScore := math.Inf(-1)
	lastErrs := []error{}

	applyOutcome := func(outcome webQueryReadOutcome) (bool, bool) {
		if outcome.Skip {
			return false, false
		}
		attempt := webQueryAttempt{
			Stage:        "read",
			Mode:         webQueryReadAttemptMode(outcome.Lane, outcome.Resp.Source),
			URL:          targetURL,
			Status:       webQueryAttemptStatus(outcome.Err),
			ContentChars: len([]rune(strings.TrimSpace(outcome.Resp.Content))),
			WarningCodes: append([]string(nil), outcome.Resp.WarningCodes...),
			Error:        errorString(outcome.Err),
		}
		result.Attempts = append(result.Attempts, attempt)
		if outcome.Err != nil {
			lastErrs = append(lastErrs, outcome.Err)
			return false, false
		}
		if reuse != nil {
			reuse.remember(targetURL, outcome.Resp)
		}
		result.HasSuccess = true
		score := scoreWebQueryReadResponse(outcome.Resp)
		if score > bestScore {
			bestScore = score
			result.Response = outcome.Resp
			result.Warnings = warningsFromLists(outcome.Resp.WarningCodes, outcome.Resp.Warnings)
			result.NeedsBrowser, result.Strong = analyzeWebQueryReadResponse(outcome.Resp)
			refinedResult, refined := t.maybeRefineLegacyReadableRead(ctx, args, targetURL, outcome.Lane, format, maxChars, result, reuse)
			if refined {
				result = refinedResult
				if reuse != nil {
					reuse.remember(targetURL, result.Response)
				}
				bestScore = scoreWebQueryReadResponse(result.Response)
			}
		}
		return result.NeedsBrowser, result.Strong
	}

	if lane == webFetchStrategySession || lane == webAccessLaneHTTPNative || lane == webAccessLaneLightpandaShim {
		resp, readErr := t.executeReadLane(ctx, args, targetURL, lane, format, maxChars, reuse)
		needsBrowser, strong := applyOutcome(webQueryReadOutcome{Lane: lane, Resp: resp, Err: readErr})
		if needsBrowser && allowedBrowser && lane != webAccessLaneBrowser {
			browserResp, browserErr := t.executeReadLane(ctx, args, targetURL, webAccessLaneBrowser, format, maxChars, reuse)
			_, _ = applyOutcome(webQueryReadOutcome{Lane: webAccessLaneBrowser, Resp: browserResp, Err: browserErr})
		}
		if strong || result.HasSuccess {
			return result, nil
		}
		return result, firstNonNilErr(append(lastErrs, errors.New("web read failed"))...)
	}

	if lane == webAccessLaneBrowser {
		resp, readErr := t.executeReadLane(ctx, args, targetURL, webAccessLaneBrowser, format, maxChars, reuse)
		_, _ = applyOutcome(webQueryReadOutcome{Lane: webAccessLaneBrowser, Resp: resp, Err: readErr})
		if result.HasSuccess {
			return result, nil
		}
		return result, firstNonNilErr(append(lastErrs, errors.New("web read failed"))...)
	}

	if lane == webAccessLaneProxyFetcher {
		resp, readErr := t.executeReadLane(ctx, args, targetURL, webAccessLaneProxyFetcher, format, maxChars, reuse)
		needsBrowser, strong := applyOutcome(webQueryReadOutcome{Lane: webAccessLaneProxyFetcher, Resp: resp, Err: readErr})
		if strong {
			return result, nil
		}
		if needsBrowser && allowedBrowser {
			browserResp, browserErr := t.executeReadLane(ctx, args, targetURL, webAccessLaneBrowser, format, maxChars, reuse)
			_, _ = applyOutcome(webQueryReadOutcome{Lane: webAccessLaneBrowser, Resp: browserResp, Err: browserErr})
		}
		if result.HasSuccess {
			return result, nil
		}
		return result, firstNonNilErr(append(lastErrs, errors.New("web read failed"))...)
	}

	readCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make(chan webQueryReadOutcome, 3)
	startedBrowser := false
	pending := 0
	startLane := func(lane string, delay time.Duration) {
		if strings.TrimSpace(lane) == "" {
			return
		}
		pending++
		go func() {
			if delay > 0 {
				timer := time.NewTimer(delay)
				defer timer.Stop()
				select {
				case <-timer.C:
				case <-readCtx.Done():
					results <- webQueryReadOutcome{Lane: lane, Skip: true}
					return
				}
			}
			resp, readErr := t.executeReadLane(readCtx, args, targetURL, lane, format, maxChars, reuse)
			results <- webQueryReadOutcome{Lane: lane, Resp: resp, Err: readErr}
		}()
	}

	startLane(webAccessLaneHTTP, 0)
	if allowedProxy {
		startLane(webAccessLaneProxyFetcher, webQueryProxyHedgeDelay)
	}

	var settleTimer *time.Timer
	for pending > 0 {
		var settle <-chan time.Time
		if settleTimer != nil {
			settle = settleTimer.C
		}

		select {
		case outcome := <-results:
			pending--
			needsBrowser, strong := applyOutcome(outcome)
			if needsBrowser && allowedBrowser && !startedBrowser {
				startedBrowser = true
				startLane(webAccessLaneBrowser, 0)
			}
			if strong && settleTimer == nil && pending > 0 {
				settleTimer = time.NewTimer(webQueryReadSettleWindow)
			}
		case <-settle:
			cancel()
			settleTimer = nil
		}
	}
	if settleTimer != nil {
		settleTimer.Stop()
	}

	if !result.HasSuccess {
		return result, firstNonNilErr(append(lastErrs, errors.New("web read failed"))...)
	}
	result = t.maybeRefineBrowserRedirectRead(ctx, args, targetURL, format, maxChars, result, reuse)
	if reuse != nil {
		reuse.remember(targetURL, result.Response)
	}
	return result, nil
}

func (t *WebTool) maybeRefineBrowserRedirectRead(ctx context.Context, args map[string]interface{}, targetURL, format string, maxChars int, result webQueryReadResult, reuse *webQueryReuseCoordinator) webQueryReadResult {
	if !result.HasSuccess {
		return result
	}
	resp := result.Response
	finalURL := strings.TrimSpace(resp.FinalURL)
	if finalURL == "" || sameWebQueryCanonicalURL(finalURL, targetURL) {
		return result
	}
	if !webQueryLooksLikeBrowserScaffold(resp.Content) {
		return result
	}

	for _, lane := range []string{webAccessLaneHTTP, webAccessLaneProxyFetcher} {
		refinedResp, err := t.executeReadLane(ctx, args, finalURL, lane, format, maxChars, reuse)
		attempt := webQueryAttempt{
			Stage:        "read_refine",
			Mode:         webQueryReadAttemptMode(lane, refinedResp.Source),
			URL:          finalURL,
			Status:       webQueryAttemptStatus(err),
			ContentChars: len([]rune(strings.TrimSpace(refinedResp.Content))),
			WarningCodes: append([]string(nil), refinedResp.WarningCodes...),
			Error:        errorString(err),
		}
		result.Attempts = append(result.Attempts, attempt)
		if err != nil {
			continue
		}
		if reuse != nil {
			reuse.remember(finalURL, refinedResp)
		}
		needsBrowser, strong := analyzeWebQueryReadResponse(refinedResp)
		if !strong || strings.EqualFold(strings.TrimSpace(refinedResp.Source), webAccessSourceBrowser) {
			continue
		}
		result.Response = refinedResp
		result.Warnings = warningsFromLists(refinedResp.WarningCodes, refinedResp.Warnings)
		result.NeedsBrowser = needsBrowser
		result.Strong = strong
		return result
	}
	return result
}

func (t *WebTool) maybeRefineLegacyReadableRead(ctx context.Context, args map[string]interface{}, targetURL, requestedLane, format string, maxChars int, result webQueryReadResult, reuse *webQueryReuseCoordinator) (webQueryReadResult, bool) {
	if !result.HasSuccess {
		return result, false
	}
	if !result.NeedsBrowser && !webQueryLooksLikeBrowserScaffold(result.Response.Content) {
		return result, false
	}
	fallbackURL := webQueryLegacyReadableFallbackURL(firstNonEmpty(strings.TrimSpace(result.Response.FinalURL), strings.TrimSpace(result.Response.URL), targetURL))
	if fallbackURL == "" || sameWebQueryCanonicalURL(fallbackURL, targetURL) {
		return result, false
	}
	if requestedLane == webAccessLaneBrowser {
		requestedLane = webAccessLaneHTTP
	}
	refinedResp, err := t.executeReadLane(ctx, args, fallbackURL, requestedLane, format, maxChars, reuse)
	attempt := webQueryAttempt{
		Stage:        "read_refine",
		Mode:         webQueryReadAttemptMode(requestedLane, refinedResp.Source),
		URL:          fallbackURL,
		Status:       webQueryAttemptStatus(err),
		ContentChars: len([]rune(strings.TrimSpace(refinedResp.Content))),
		WarningCodes: append([]string(nil), refinedResp.WarningCodes...),
		Error:        errorString(err),
	}
	result.Attempts = append(result.Attempts, attempt)
	if err != nil {
		return result, false
	}
	if reuse != nil {
		reuse.remember(fallbackURL, refinedResp)
	}
	needsBrowser, strong := analyzeWebQueryReadResponse(refinedResp)
	if !strong || strings.EqualFold(strings.TrimSpace(refinedResp.Source), webAccessSourceBrowser) {
		return result, false
	}
	result.Response = refinedResp
	result.Warnings = warningsFromLists(refinedResp.WarningCodes, refinedResp.Warnings)
	result.NeedsBrowser = needsBrowser
	result.Strong = strong
	return result, true
}

func (t *WebTool) preferredReadLane(ctx context.Context, args map[string]interface{}, targetURL string) string {
	readTool, ok := t.read.(*WebReadTool)
	if !ok || readTool == nil || readTool.runtime == nil || readTool.runtime.base == nil {
		return ""
	}
	browserTargetID := strings.TrimSpace(firstCompatStringDeep(args, "browser_target_id", "browserTargetId"))
	return readTool.runtime.base.preferredReadLane(ctx, targetURL, browserTargetID)
}

func (t *WebTool) executeReadLane(ctx context.Context, args map[string]interface{}, targetURL, lane, format string, maxChars int, reuse *webQueryReuseCoordinator) (webReadResponse, error) {
	readArgs := normalizeWebReadCompatArgs(args)
	if reuseTargetID := reuse.browserTargetID(targetURL); reuseTargetID != "" && strings.TrimSpace(asString(readArgs["browser_target_id"])) == "" {
		readArgs["browser_target_id"] = reuseTargetID
	}
	readArgs["url"] = targetURL
	readArgs["lane"] = lane
	readArgs["format"] = format
	readArgs["max_chars"] = maxChars
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

func webQueryReadAttemptMode(requestedLane, source string) string {
	switch strings.TrimSpace(source) {
	case webAccessSourceHTTPNative:
		return webAccessLaneHTTPNative
	case webAccessSourceLightpandaShim:
		return webAccessLaneLightpandaShim
	case webAccessSourceBrowser:
		return webAccessLaneBrowser
	case webAccessSourceProxyFetcher:
		return webAccessLaneProxyFetcher
	default:
		return requestedLane
	}
}

func sameWebQueryCanonicalURL(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" || b == "" {
		return false
	}
	canonicalA, errA := canonicalizeCrawlURL(a)
	if errA != nil {
		canonicalA = a
	}
	canonicalB, errB := canonicalizeCrawlURL(b)
	if errB != nil {
		canonicalB = b
	}
	return canonicalA == canonicalB
}

func webQueryLegacyReadableFallbackURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	path := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(parsed.EscapedPath())), "/")
	switch {
	case host == "platform.openai.com" && strings.HasPrefix(path, "/docs/api-reference/responses"):
		return "https://developers.openai.com/api/reference/resources/responses"
	default:
		return ""
	}
}

func webQueryLooksLikeBrowserScaffold(content string) bool {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, "[RootWebArea]") {
		return true
	}
	if strings.HasPrefix(trimmed, "Page: ") && strings.Contains(trimmed, "interactive elements") {
		return true
	}
	if strings.Contains(trimmed, "\n@1 [") && strings.Contains(trimmed, "interactive elements") {
		return true
	}
	return false
}

func (t *WebTool) SetBrowser(browser BrowserBackend) {
	t.browser = browser
	for _, candidate := range []Tool{t.fetch, t.read, t.extract} {
		if setter, ok := candidate.(interface{ SetBrowser(BrowserBackend) }); ok {
			setter.SetBrowser(browser)
		}
	}
}

func (t *WebTool) SetLightpandaShim(service *browser.LightpandaService) {
	for _, candidate := range []Tool{t.fetch, t.read, t.extract} {
		if setter, ok := candidate.(interface {
			SetLightpandaShim(*browser.LightpandaService)
		}); ok {
			setter.SetLightpandaShim(service)
		}
	}
}

func (t *WebTool) SetImageTool(tool Tool) {
	if t == nil {
		return
	}
	t.image = tool
}

func (t *WebTool) SetSTTService(service stt.Service) {
	t.sttService = service
}

func (t *WebTool) SetPDFService(service PDFService) {
	for _, candidate := range []Tool{t.fetch, t.read, t.extract, t.crawl} {
		if setter, ok := candidate.(interface{ SetPDFService(PDFService) }); ok {
			setter.SetPDFService(service)
		}
	}
}

func (t *WebTool) SetDocumentReadService(service DocumentReadService) {
	for _, candidate := range []Tool{t.fetch, t.read} {
		if setter, ok := candidate.(interface{ SetDocumentReadService(DocumentReadService) }); ok {
			setter.SetDocumentReadService(service)
		}
	}
}

func inferWebCompatAction(args map[string]interface{}) string {
	action := strings.ToLower(strings.TrimSpace(firstCompatStringDeep(args, "action", "op", "operation")))
	switch action {
	case "search", "fetch", "read", "extract", "crawl":
		return action
	}
	if hasLegacyWebExtractCompatArgs(args) {
		return "extract"
	}
	if hasLegacyWebCrawlCompatArgs(args) {
		return "crawl"
	}
	return ""
}

func hasLegacyWebExtractCompatArgs(args map[string]interface{}) bool {
	_, hasFields := firstCompatValueDeep(args, "fields")
	_, hasHTML := firstCompatValueDeep(args, "html")
	return hasFields || hasHTML
}

func hasLegacyWebCrawlCompatArgs(args map[string]interface{}) bool {
	if _, ok := firstCompatValueDeep(args, "seeds", "checkpoint", "max_pages", "maxPages", "max_concurrency", "maxConcurrency", "max_retries", "maxRetries", "rate_limit_ms", "rateLimitMs", "page_max_chars", "pageMaxChars"); ok {
		return true
	}
	if _, ok := firstCompatValueDeep(args, "max_depth", "maxDepth"); ok {
		_, hasSeeds := firstCompatValueDeep(args, "seeds")
		return hasSeeds
	}
	return false
}

func normalizeWebQueryArgs(args map[string]interface{}) map[string]interface{} {
	normalized := normalizeWebFetchCompatArgs("web_query", args)

	rawMedia, ok := compatArgValue(normalized, "media")
	if !ok {
		return normalized
	}
	media, ok := coerceCompatMap(rawMedia)
	if !ok || len(media) == 0 {
		return normalized
	}

	if _, exists := normalized["include_media"]; !exists {
		if enabled, ok := firstCompatValueDeep(media, "enabled"); ok {
			normalized["include_media"] = enabled
		} else {
			normalized["include_media"] = true
		}
	}
	if strings.TrimSpace(asString(normalized["media_mode"])) == "" {
		if mode := firstCompatStringDeep(media, "mode"); mode != "" {
			normalized["media_mode"] = mode
		}
	}
	if strings.TrimSpace(asString(normalized["media_prompt"])) == "" {
		if prompt := firstCompatStringDeep(media, "prompt"); prompt != "" {
			normalized["media_prompt"] = prompt
		}
	}
	if _, exists := normalized["max_media"]; !exists {
		if value, ok := firstCompatValueDeep(media, "max_items", "maxItems", "limit"); ok {
			normalized["max_media"] = value
		}
	}

	return normalized
}

func normalizeWebReadCompatArgs(args map[string]interface{}) map[string]interface{} {
	normalized := normalizeWebFetchCompatArgs("web_query", args)
	if strings.TrimSpace(asString(normalized["format"])) == "" {
		if format := firstCompatStringDeep(normalized, "format"); format != "" {
			normalized["format"] = format
		}
	}
	if strings.TrimSpace(asString(normalized["lane"])) == "" {
		if lane := firstCompatStringDeep(normalized, "lane"); lane != "" {
			normalized["lane"] = lane
		}
	}
	return normalized
}

func normalizeWebExtractCompatArgs(args map[string]interface{}) map[string]interface{} {
	normalized := normalizeWebReadCompatArgs(args)
	if _, ok := normalized["fields"]; !ok {
		if fields, exists := firstCompatValueDeep(normalized, "fields"); exists {
			normalized["fields"] = fields
		}
	}
	if strings.TrimSpace(asString(normalized["html"])) == "" {
		if rawHTML := firstCompatStringDeep(normalized, "html"); rawHTML != "" {
			normalized["html"] = rawHTML
		}
	}
	return normalized
}

func normalizeWebCrawlCompatArgs(args map[string]interface{}) map[string]interface{} {
	normalized := normalizeWebReadCompatArgs(args)
	for _, key := range []string{
		"seeds", "allowed_hosts", "checkpoint", "max_depth", "max_pages", "max_concurrency", "max_retries", "rate_limit_ms", "page_max_chars",
	} {
		if _, ok := normalized[key]; ok {
			continue
		}
		if value, exists := firstCompatValueDeep(normalized, key); exists {
			normalized[key] = value
		}
	}
	if _, ok := normalized["allowed_hosts"]; !ok {
		if value, exists := firstCompatValueDeep(normalized, "allowedHosts", "hosts"); exists {
			normalized["allowed_hosts"] = value
		}
	}
	if _, ok := normalized["max_depth"]; !ok {
		if value, exists := firstCompatValueDeep(normalized, "maxDepth", "depth"); exists {
			normalized["max_depth"] = value
		}
	}
	if _, ok := normalized["max_pages"]; !ok {
		if value, exists := firstCompatValueDeep(normalized, "maxPages"); exists {
			normalized["max_pages"] = value
		}
	}
	if _, ok := normalized["max_concurrency"]; !ok {
		if value, exists := firstCompatValueDeep(normalized, "maxConcurrency"); exists {
			normalized["max_concurrency"] = value
		}
	}
	if _, ok := normalized["max_retries"]; !ok {
		if value, exists := firstCompatValueDeep(normalized, "maxRetries"); exists {
			normalized["max_retries"] = value
		}
	}
	if _, ok := normalized["rate_limit_ms"]; !ok {
		if value, exists := firstCompatValueDeep(normalized, "rateLimitMs"); exists {
			normalized["rate_limit_ms"] = value
		}
	}
	if _, ok := normalized["page_max_chars"]; !ok {
		if value, exists := firstCompatValueDeep(normalized, "pageMaxChars"); exists {
			normalized["page_max_chars"] = value
		}
	}
	return normalized
}

func newWebQueryEnvelope(input, format string) webQueryEnvelope {
	return webQueryEnvelope{
		Status:            webQueryStatusPartial,
		Mode:              "",
		Input:             strings.TrimSpace(input),
		Query:             "",
		Provider:          "",
		TargetURL:         "",
		FinalURL:          "",
		Title:             "",
		Content:           "",
		ContentFormat:     firstNonEmpty(strings.TrimSpace(format), webReadFormatText),
		Sources:           []webQuerySource{},
		Warnings:          []webQueryWarning{},
		NextAction:        webQueryNextActionNone,
		SearchCardEmitted: false,
		Diagnostics: webQueryDiagnostics{
			Route:          "",
			Attempts:       []webQueryAttempt{},
			CandidateCount: 0,
			SelectedSource: 0,
			Degraded:       false,
		},
	}
}

func marshalWebQueryEnvelope(envelope webQueryEnvelope) (string, error) {
	b, err := json.Marshal(envelope)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func resolveWebQueryInput(args map[string]interface{}) string {
	if value := strings.TrimSpace(firstCompatStringDeep(args, "input")); value != "" {
		return value
	}
	if value := strings.TrimSpace(firstCompatStringDeep(args, "url", "href", "target")); value != "" {
		return value
	}
	return strings.TrimSpace(firstCompatStringDeep(args, "query", "q", "search", "keyword", "text"))
}

func parseWebQueryDepth(args map[string]interface{}) (string, error) {
	depth := strings.ToLower(strings.TrimSpace(firstCompatStringDeep(args, "depth")))
	if depth == "" {
		return webQueryDepthStandard, nil
	}
	switch depth {
	case webQueryDepthQuick, webQueryDepthStandard, webQueryDepthDeep:
		return depth, nil
	default:
		return "", fmt.Errorf("depth must be one of: %s, %s, %s", webQueryDepthQuick, webQueryDepthStandard, webQueryDepthDeep)
	}
}

func parseWebQueryFormat(args map[string]interface{}) (string, error) {
	raw := strings.ToLower(strings.TrimSpace(firstCompatStringDeep(args, "format", "extract_mode", "extractMode", "mode")))
	if raw == "" {
		return webReadFormatText, nil
	}
	switch raw {
	case webReadFormatMarkdown, webReadFormatText:
		return raw, nil
	default:
		return "", fmt.Errorf("format must be one of: %s, %s", webReadFormatMarkdown, webReadFormatText)
	}
}

func parseWebQueryMaxResults(args map[string]interface{}, depth string) int {
	fallback := 5
	switch depth {
	case webQueryDepthQuick:
		fallback = 3
	case webQueryDepthDeep:
		fallback = 8
	}
	return parseWebSearchMaxResults(firstCompatValueOrNil(args, "max_results", "maxResults", "limit", "n"), fallback)
}

func parseWebQueryCandidateReadLimit(depth string) int {
	switch depth {
	case webQueryDepthQuick:
		return 1
	case webQueryDepthDeep:
		return 3
	default:
		return 2
	}
}

func canonicalWebQueryURL(query string) (string, bool) {
	if !canonicalWebQuerySiteHintsAllowShortcut(query) {
		return "", false
	}
	if looksLikeOpenAIChatCompletionsDocsQuery(query) {
		return "https://developers.openai.com/api/reference/chat-completions/overview", true
	}
	if !looksLikeOpenAIResponsesDocsQuery(query) {
		return "", false
	}
	return "https://developers.openai.com/api/reference/resources/responses", true
}

func canonicalWebQuerySiteHintsAllowShortcut(query string) bool {
	matches := webQuerySiteHintPattern.FindAllStringSubmatch(query, -1)
	if len(matches) == 0 {
		return true
	}
	if len(matches) != 1 {
		return false
	}
	host := normalizeWebQueryHostHint(matches[0][1])
	if host == "" {
		return false
	}
	return host == "openai.com" || strings.HasSuffix(host, ".openai.com")
}

func looksLikeOpenAIResponsesDocsQuery(query string) bool {
	query = strings.TrimSpace(query)
	if query == "" || looksLikeWebQueryURL(query) {
		return false
	}
	if matchesKnownLocalizedWebQueryExample(query) {
		return true
	}
	lower := strings.ToLower(query)
	if !strings.Contains(lower, "openai") || !strings.Contains(lower, "api") {
		return false
	}
	responseDocsIntent := strings.Contains(lower, "responses")
	if !responseDocsIntent && strings.Contains(lower, "response") && strings.Contains(lower, "format") {
		responseDocsIntent = true
	}
	if !responseDocsIntent {
		return false
	}
	for _, marker := range []string{"doc", "documentation", "latest", "official", "reference"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func looksLikeOpenAIChatCompletionsDocsQuery(query string) bool {
	query = strings.TrimSpace(query)
	if query == "" || looksLikeWebQueryURL(query) {
		return false
	}
	lower := strings.ToLower(query)
	if !strings.Contains(lower, "openai") || !strings.Contains(lower, "api") {
		return false
	}
	if !strings.Contains(lower, "chat completion") {
		return false
	}
	responseStructureIntent := strings.Contains(lower, "response") && (strings.Contains(lower, "object") || strings.Contains(lower, "structure") || strings.Contains(lower, "schema"))
	if !responseStructureIntent {
		return false
	}
	for _, marker := range []string{"doc", "documentation", "official", "reference"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func matchesKnownLocalizedWebQueryExample(query string) bool {
	normalized := normalizeWebQueryIntentText(query)
	if normalized == "" {
		return false
	}
	for _, example := range routingcue.LocalizedExamples("web_query") {
		if normalizeWebQueryIntentText(example.Query) == normalized {
			return true
		}
	}
	return false
}

func normalizeWebQueryIntentText(input string) string {
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "" {
		return ""
	}
	fields := strings.FieldsFunc(input, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r)
	})
	return strings.Join(fields, " ")
}

func buildWebQueryCandidates(results []WebSearchResult, allowedHosts []string) []webQueryCandidate {
	if len(results) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(results))
	out := make([]webQueryCandidate, 0, len(results))
	for idx, result := range results {
		targetURL, canonical, ok := webQueryCandidateURLKey(result.URL)
		if !ok {
			continue
		}
		if len(allowedHosts) > 0 && !crawlHostAllowed(targetURL, allowedHosts) {
			continue
		}
		if _, ok := seen[canonical]; ok {
			continue
		}
		seen[canonical] = struct{}{}
		result.URL = targetURL
		out = append(out, webQueryCandidate{
			Rank:   idx + 1,
			Search: result,
		})
	}
	return out
}

func webQueryCandidateURLKey(raw string) (string, string, bool) {
	targetURL := normalizeWebQueryCandidateURL(strings.TrimSpace(raw))
	if targetURL == "" {
		return "", "", false
	}
	canonical, err := canonicalizeCrawlURL(targetURL)
	if err != nil {
		canonical = targetURL
	}
	return targetURL, canonical, true
}

func webQueryCandidateStateByURL(previous []webQueryCandidate) map[string]webQueryCandidate {
	if len(previous) == 0 {
		return nil
	}
	previousByURL := make(map[string]webQueryCandidate, len(previous))
	for _, candidate := range previous {
		_, canonical, ok := webQueryCandidateURLKey(candidate.Search.URL)
		if !ok {
			continue
		}
		previousByURL[canonical] = candidate
	}
	if len(previousByURL) == 0 {
		return nil
	}
	return previousByURL
}

func normalizeWebQueryCandidateURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if normalized := normalizeBingResultURL(raw); normalized != "" {
		return normalized
	}
	return raw
}

func detectWebQueryFinanceFastPath(query string) (WebSearchResult, bool) {
	ticker := extractWebQueryTicker(query)
	if ticker == "" || !looksLikeWebQueryStockQuote(query) {
		return WebSearchResult{}, false
	}
	return WebSearchResult{
		Title:       ticker + " stock price",
		URL:         "https://stockanalysis.com/stocks/" + strings.ToLower(ticker) + "/",
		Description: ticker + " stock quote and market summary",
		Source:      "finance_fast_path",
	}, true
}

func looksLikeWebQueryStockQuote(query string) bool {
	query = strings.TrimSpace(query)
	if query == "" {
		return false
	}
	if webQueryFinanceHintPattern.MatchString(query) {
		return true
	}
	return strings.Contains(query, "股价") || strings.Contains(query, "股票") || strings.Contains(query, "行情")
}

func extractWebQueryTicker(query string) string {
	if match := webQueryTickerParenPattern.FindStringSubmatch(query); len(match) == 2 {
		return strings.ToUpper(strings.TrimSpace(match[1]))
	}
	for _, token := range webQueryTickerWordPattern.FindAllString(query, -1) {
		token = strings.ToUpper(strings.TrimSpace(token))
		if _, blocked := webQueryTickerStopwords[token]; blocked {
			continue
		}
		return token
	}
	return ""
}

var webQueryTickerStopwords = map[string]struct{}{
	"A":   {},
	"AI":  {},
	"CEO": {},
	"I":   {},
	"THE": {},
	"USD": {},
}

func mergeWebQueryAllowedHosts(query string, allowedHosts []string) []string {
	merged := make([]string, 0, len(allowedHosts)+2)
	seen := make(map[string]struct{}, len(allowedHosts)+2)
	add := func(raw string) {
		host := normalizeWebQueryHostHint(raw)
		if host == "" {
			return
		}
		if _, ok := seen[host]; ok {
			return
		}
		seen[host] = struct{}{}
		merged = append(merged, host)
	}
	for _, host := range allowedHosts {
		add(host)
	}
	for _, match := range webQuerySiteHintPattern.FindAllStringSubmatch(query, -1) {
		if len(match) < 2 {
			continue
		}
		add(match[1])
	}
	return merged
}

func normalizeWebQueryHostHint(raw string) string {
	raw = strings.TrimSpace(strings.Trim(raw, "\"'()[]{}<>,.;!?"))
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(parsed.Hostname()))
}

func (t *WebTool) executeCanonicalURLQuery(ctx context.Context, args map[string]interface{}, originalInput, canonicalURL, format string, maxChars int) webQueryEnvelope {
	envelope := t.executeURLQuery(ctx, args, canonicalURL, format, maxChars)
	envelope.Input = strings.TrimSpace(originalInput)
	envelope.Query = strings.TrimSpace(originalInput)
	envelope.Diagnostics.Route = "canonical_url"
	return envelope
}

func buildWebQuerySources(candidates []webQueryCandidate, selectedURL string) []webQuerySource {
	if len(candidates) == 0 {
		return []webQuerySource{}
	}
	ordered := append([]webQueryCandidate(nil), candidates...)
	sortWebQueryCandidatesByScore(ordered)
	return buildWebQuerySourcesFromSortedCandidates(ordered, selectedURL)
}

func buildWebQuerySourcesFromSortedCandidates(candidates []webQueryCandidate, selectedURL string) []webQuerySource {
	sources, _ := buildWebQuerySourcesFromSortedCandidatesWithSelectedRank(candidates, selectedURL)
	return sources
}

func buildWebQuerySourcesFromSortedCandidatesWithSelectedRank(candidates []webQueryCandidate, selectedURL string) ([]webQuerySource, int) {
	if len(candidates) == 0 {
		return []webQuerySource{}, 0
	}
	sources := make([]webQuerySource, 0, len(candidates))
	selectedURL = strings.TrimSpace(selectedURL)
	selectedRank := 0
	for idx, candidate := range candidates {
		candidateURL := strings.TrimSpace(candidate.Search.URL)
		source := webQuerySource{
			Rank:         idx + 1,
			Kind:         "search",
			URL:          candidateURL,
			FinalURL:     strings.TrimSpace(candidate.Resolved.Response.FinalURL),
			Title:        firstNonEmpty(strings.TrimSpace(candidate.Resolved.Response.Title), strings.TrimSpace(candidate.Search.Title)),
			Snippet:      firstNonEmpty(strings.TrimSpace(candidate.Search.Description), truncateRunes(strings.TrimSpace(candidate.Resolved.Response.Content), 280)),
			Source:       firstNonEmpty(strings.TrimSpace(candidate.Resolved.Response.Source), strings.TrimSpace(candidate.Search.Source)),
			ContentChars: len([]rune(strings.TrimSpace(candidate.Resolved.Response.Content))),
			WarningCodes: append([]string(nil), candidate.Resolved.Response.WarningCodes...),
			Selected:     candidateURL == selectedURL,
		}
		if source.Selected {
			selectedRank = idx + 1
		}
		if source.FinalURL == "" {
			source.FinalURL = source.URL
		}
		sources = append(sources, source)
	}
	return sources, selectedRank
}

func sortWebQueryCandidatesByScore(candidates []webQueryCandidate) {
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			return candidates[i].Rank < candidates[j].Rank
		}
		return candidates[i].Score > candidates[j].Score
	})
}

func searchCardStatusFromEnvelope(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case webQueryStatusOK:
		return webQuerySearchCardStatusSuccess
	case webQueryStatusPartial, webQueryStatusNeedsBrowser, webQueryStatusError:
		return webQuerySearchCardStatusPartial
	default:
		return webQuerySearchCardStatusPartial
	}
}

func selectedSearchURL(sources []webQuerySource) string {
	for _, source := range sources {
		if !source.Selected {
			continue
		}
		if target := firstNonEmpty(strings.TrimSpace(source.FinalURL), strings.TrimSpace(source.URL)); target != "" {
			return target
		}
	}
	return ""
}

func buildWebQuerySearchCard(query, provider string, sources []webQuerySource, status, message, selectedURL string) map[string]interface{} {
	trimmedQuery := strings.TrimSpace(query)
	results := make([]map[string]interface{}, 0, len(sources))
	for _, source := range sources {
		target := firstNonEmpty(strings.TrimSpace(source.FinalURL), strings.TrimSpace(source.URL))
		if target == "" {
			continue
		}
		title := firstNonEmpty(strings.TrimSpace(source.Title), target)
		result := map[string]interface{}{
			"title": title,
			"url":   target,
		}
		if description := firstNonEmpty(strings.TrimSpace(source.Snippet), strings.TrimSpace(source.Source)); description != "" {
			result["description"] = description
		}
		if sourceLabel := strings.TrimSpace(source.Source); sourceLabel != "" {
			result["source"] = sourceLabel
		}
		results = append(results, result)
	}

	card := map[string]interface{}{
		"type":        "search",
		"query":       trimmedQuery,
		"results":     results,
		"total_count": len(results),
		"status":      firstNonEmpty(strings.TrimSpace(status), webQuerySearchCardStatusSuccess),
	}
	if trimmedProvider := strings.TrimSpace(provider); trimmedProvider != "" {
		card["provider"] = trimmedProvider
	}
	if trimmedMessage := strings.TrimSpace(message); trimmedMessage != "" {
		card["message"] = trimmedMessage
	}
	if trimmedSelectedURL := strings.TrimSpace(selectedURL); trimmedSelectedURL != "" {
		card["selectedUrl"] = trimmedSelectedURL
	}
	if trimmedQuery != "" {
		card["id"] = "web-query-search-" + url.QueryEscape(trimmedQuery)
	}
	return card
}

func emitWebQuerySearchCard(ctx context.Context, envelope *webQueryEnvelope, status, message, selectedURL string) {
	if envelope == nil || ctx == nil {
		return
	}
	emitter, _ := ctx.Value(cardEmitKey).(CardEmitFunc)
	if emitter == nil {
		return
	}
	card := buildWebQuerySearchCard(
		firstNonEmpty(strings.TrimSpace(envelope.Query), strings.TrimSpace(envelope.Input), strings.TrimSpace(envelope.Title)),
		strings.TrimSpace(envelope.Provider),
		envelope.Sources,
		status,
		message,
		selectedURL,
	)
	EmitCard(ctx, card)
	envelope.SearchCardEmitted = true
}

func applyResolvedReadToEnvelope(envelope webQueryEnvelope, readResult webQueryReadResult, rank int) webQueryEnvelope {
	resp := readResult.Response
	strategyUsed := strings.TrimSpace(resp.StrategyUsed)
	if strategyUsed == "" {
		switch strings.TrimSpace(resp.Source) {
		case webAccessSourceBrowser:
			strategyUsed = webFetchStrategyBrowser
		case webAccessSourceProxyFetcher:
			strategyUsed = webFetchStrategyProxy
		case webAccessSourceHTTPNative:
			strategyUsed = webAccessLaneHTTPNative
		case webAccessSourceLightpandaShim:
			strategyUsed = webAccessLaneLightpandaShim
		default:
			strategyUsed = webFetchStrategyHTTP
		}
	}
	envelope.TargetURL = strings.TrimSpace(resp.URL)
	envelope.FinalURL = firstNonEmpty(strings.TrimSpace(resp.FinalURL), strings.TrimSpace(resp.URL))
	envelope.Title = strings.TrimSpace(resp.Title)
	envelope.Content = strings.TrimSpace(resp.Content)
	envelope.ContentFormat = firstNonEmpty(strings.TrimSpace(resp.Format), envelope.ContentFormat, webReadFormatText)
	envelope.Page = &webQueryPage{
		Title:         strings.TrimSpace(resp.Title),
		Content:       strings.TrimSpace(resp.Content),
		ContentFormat: firstNonEmpty(strings.TrimSpace(resp.Format), webReadFormatText),
		TargetURL:     strings.TrimSpace(resp.URL),
		FinalURL:      firstNonEmpty(strings.TrimSpace(resp.FinalURL), strings.TrimSpace(resp.URL)),
		Source:        strings.TrimSpace(resp.Source),
	}
	envelope.Sources = []webQuerySource{{
		Rank:         rank,
		Kind:         "page",
		URL:          strings.TrimSpace(resp.URL),
		FinalURL:     firstNonEmpty(strings.TrimSpace(resp.FinalURL), strings.TrimSpace(resp.URL)),
		Title:        strings.TrimSpace(resp.Title),
		Snippet:      truncateRunes(strings.TrimSpace(resp.Content), 280),
		Source:       strings.TrimSpace(resp.Source),
		ContentChars: len([]rune(strings.TrimSpace(resp.Content))),
		WarningCodes: append([]string(nil), resp.WarningCodes...),
		Selected:     true,
	}}
	envelope.Diagnostics.StrategyUsed = strategyUsed
	envelope.Diagnostics.SessionReused = resp.SessionReused
	envelope.Diagnostics.AdapterID = strings.TrimSpace(resp.AdapterID)
	envelope.Diagnostics.NetworkObserved = resp.NetworkObserved
	envelope.Diagnostics.ChallengeState = cloneChallengeState(resp.ChallengeState)
	envelope.Warnings = append(envelope.Warnings, readResult.Warnings...)
	switch {
	case readResult.NeedsBrowser:
		envelope.Status = webQueryStatusNeedsBrowser
		envelope.NextAction = webQueryNextActionRetryBrowser
	case readResult.Strong:
		envelope.Status = webQueryStatusOK
		envelope.NextAction = webQueryNextActionNone
	default:
		envelope.Status = webQueryStatusPartial
		envelope.NextAction = webQueryNextActionNone
	}
	return envelope
}

func decodeToolJSONResult(raw interface{}, out interface{}) error {
	switch typed := raw.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return errors.New("empty JSON payload")
		}
		return json.Unmarshal([]byte(typed), out)
	case []byte:
		if len(typed) == 0 {
			return errors.New("empty JSON payload")
		}
		return json.Unmarshal(typed, out)
	default:
		b, err := json.Marshal(typed)
		if err != nil {
			return err
		}
		return json.Unmarshal(b, out)
	}
}

func (t *WebTool) enrichWebQueryEnvelopeWithMedia(ctx context.Context, args map[string]interface{}, envelope *webQueryEnvelope) {
	if t == nil || envelope == nil || !shouldIncludeWebQueryMedia(args) || t.extract == nil || t.image == nil {
		return
	}
	targetURL := strings.TrimSpace(firstNonEmpty(envelope.FinalURL, envelope.TargetURL))
	if targetURL == "" {
		return
	}
	maxItems := parseWebQueryMediaMaxItems(args)
	candidates, err := t.extractWebQueryMediaCandidates(ctx, args, targetURL, maxItems)
	if err != nil {
		addWebQueryWarning(&envelope.Warnings, "media_extract_failed", err.Error())
		return
	}
	plan := buildWebQueryMediaAnalysisPlan(args, envelope, candidates, maxItems)
	if len(plan.Candidates) == 0 {
		if plan.SkipReason != "" {
			addWebQueryWarning(&envelope.Warnings, "media_skipped", plan.SkipReason)
		}
		return
	}
	items, summary, err := t.analyzeWebQueryMediaCandidates(ctx, args, plan.Candidates, plan.ImageAnalysisMode)
	if err != nil {
		addWebQueryWarning(&envelope.Warnings, "media_analysis_failed", err.Error())
		return
	}
	if len(items) == 0 {
		return
	}
	envelope.Media = &webQueryMedia{
		AnalysisMode: plan.ImageAnalysisMode,
		Summary:      truncateRunes(strings.TrimSpace(summary), webQueryMediaSummaryMaxChars),
		Items:        items,
	}
}

func looksLikeWebQueryURL(input string) bool {
	_, err := normalizeWebFetchURL(input)
	return err == nil
}

func shouldIncludeWebQueryMedia(args map[string]interface{}) bool {
	if mode, ok := parseWebQueryMediaModeArg(args); ok {
		return mode != webQueryMediaModeOff
	}
	if raw, ok := compatArgValue(args, "include_media", "includeMedia", "analyze_media", "analyzeMedia"); ok {
		return compatBool(raw)
	}
	if strings.TrimSpace(firstCompatString(args, "media_prompt", "mediaPrompt")) != "" {
		return true
	}
	if raw, ok := compatArgValue(args, "max_media", "maxMedia"); ok {
		return parsePositiveInt(raw) > 0
	}
	return false
}

func parseWebQueryMediaModeArg(args map[string]interface{}) (string, bool) {
	raw, ok := compatArgValue(args, "media_mode", "mediaMode")
	if !ok {
		return "", false
	}
	mode := strings.ToLower(strings.TrimSpace(asString(raw)))
	switch mode {
	case "", webQueryMediaModeAuto:
		return webQueryMediaModeAuto, true
	case "none", "disabled", webQueryMediaModeOff:
		return webQueryMediaModeOff, true
	case "ocr", "text", "ocr_only":
		return webQueryMediaModeOCR, true
	case "full", "vision", "vision_first", "vision-first":
		return webQueryMediaModeFull, true
	default:
		return webQueryMediaModeAuto, true
	}
}

func parseWebQueryMediaMode(args map[string]interface{}) string {
	if mode, ok := parseWebQueryMediaModeArg(args); ok {
		return mode
	}
	return webQueryMediaModeAuto
}

func parseWebQueryMediaMaxItems(args map[string]interface{}) int {
	raw, ok := compatArgValue(args, "max_media", "maxMedia")
	if !ok {
		return defaultWebQueryMediaMaxItems
	}
	value := parsePositiveInt(raw)
	if value <= 0 {
		return defaultWebQueryMediaMaxItems
	}
	if value > maxWebQueryMediaMaxItems {
		return maxWebQueryMediaMaxItems
	}
	return value
}

func parseWebQueryMediaPrompt(args map[string]interface{}) string {
	if prompt := strings.TrimSpace(firstCompatString(args, "media_prompt", "mediaPrompt")); prompt != "" {
		return prompt
	}
	return defaultWebQueryMediaPrompt
}

func hasCustomWebQueryMediaPrompt(args map[string]interface{}) bool {
	return strings.TrimSpace(firstCompatString(args, "media_prompt", "mediaPrompt")) != ""
}

func parsePositiveInt(raw interface{}) int {
	switch typed := raw.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float32:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		if value, err := typed.Int64(); err == nil {
			return int(value)
		}
	case string:
		if value, err := strconv.Atoi(strings.TrimSpace(typed)); err == nil {
			return value
		}
	}
	return 0
}

func (t *WebTool) extractWebQueryMediaCandidates(ctx context.Context, args map[string]interface{}, targetURL string, maxItems int) ([]webQueryMediaCandidate, error) {
	extractArgs := normalizeWebExtractCompatArgs(args)
	extractArgs["url"] = targetURL
	lane := strings.ToLower(strings.TrimSpace(asString(extractArgs["lane"])))
	switch lane {
	case "", webAccessLaneBrowser, webAccessLaneProxyFetcher, webAccessLaneLightpandaShim:
		extractArgs["lane"] = webAccessLaneHTTP
	}
	extractArgs["fields"] = map[string]interface{}{
		"og_image": map[string]interface{}{
			"selectors": []string{"meta[property='og:image']", "meta[name='og:image']", "meta[property='og:image:url']"},
			"attribute": "content",
		},
		"og_image_alt": map[string]interface{}{
			"selectors": []string{"meta[property='og:image:alt']", "meta[name='og:image:alt']"},
			"attribute": "content",
		},
		"twitter_image": map[string]interface{}{
			"selectors": []string{"meta[name='twitter:image']", "meta[property='twitter:image']"},
			"attribute": "content",
		},
		"twitter_image_alt": map[string]interface{}{
			"selectors": []string{"meta[name='twitter:image:alt']", "meta[property='twitter:image:alt']"},
			"attribute": "content",
		},
		"link_image": map[string]interface{}{
			"selectors": []string{"link[rel='image_src']", "link[rel='preload'][as='image']"},
			"attribute": "href",
		},
		"inline_images": map[string]interface{}{
			"selector":  "img",
			"attribute": "src",
			"multiple":  true,
		},
		"inline_srcsets": map[string]interface{}{
			"selector":  "img[srcset]",
			"attribute": "srcset",
			"multiple":  true,
		},
		"inline_data_src_images": map[string]interface{}{
			"selector":  "img[data-src]",
			"attribute": "data-src",
			"multiple":  true,
		},
		"inline_lazy_images": map[string]interface{}{
			"selector":  "img[data-lazy-src]",
			"attribute": "data-lazy-src",
			"multiple":  true,
		},
		"inline_alts": map[string]interface{}{
			"selector":  "img",
			"attribute": "alt",
			"multiple":  true,
		},
	}
	raw, err := t.extract.Execute(ctx, extractArgs)
	if err != nil {
		return nil, err
	}
	var resp webQueryMediaExtractResponse
	if err := decodeToolJSONResult(raw, &resp); err != nil {
		return nil, err
	}
	candidates := collectWebQueryMediaCandidates(targetURL, resp.Data)
	poolSize := maxItems * 3
	if poolSize < maxItems {
		poolSize = maxItems
	}
	if maxPool := maxWebQueryMediaMaxItems * 3; poolSize > maxPool {
		poolSize = maxPool
	}
	if len(candidates) > poolSize {
		candidates = candidates[:poolSize]
	}
	return candidates, nil
}

func collectWebQueryMediaCandidates(pageURL string, data map[string]interface{}) []webQueryMediaCandidate {
	if len(data) == 0 {
		return nil
	}
	candidateByURL := make(map[string]webQueryMediaCandidate)
	register := func(rawURL, alt, source string, score int, skipDecorative bool) {
		resolved := resolveWebQueryMediaURL(pageURL, rawURL)
		if resolved == "" || shouldSkipWebQueryMediaCandidate(resolved, alt, skipDecorative) {
			return
		}
		alt = strings.TrimSpace(alt)
		current, exists := candidateByURL[resolved]
		candidate := webQueryMediaCandidate{
			URL:    resolved,
			Alt:    alt,
			Source: source,
			Score:  score,
		}
		if !exists || candidate.Score > current.Score {
			if current.Alt != "" && candidate.Alt == "" {
				candidate.Alt = current.Alt
			}
			candidateByURL[resolved] = candidate
			return
		}
		if current.Alt == "" && alt != "" {
			current.Alt = alt
			candidateByURL[resolved] = current
		}
	}

	register(asString(data["og_image"]), asString(data["og_image_alt"]), "og:image", 120, false)
	register(asString(data["twitter_image"]), asString(data["twitter_image_alt"]), "twitter:image", 112, false)
	register(asString(data["link_image"]), "", "link:image_src", 100, false)

	inlineAlts := stringSliceFromCompatValue(data["inline_alts"])
	for index, rawURL := range stringSliceFromCompatValue(data["inline_images"]) {
		register(rawURL, stringAt(inlineAlts, index), "img", scoreInlineWebQueryMediaCandidate(rawURL, stringAt(inlineAlts, index), 0), true)
	}
	for index, rawURL := range stringSliceFromCompatValue(data["inline_data_src_images"]) {
		register(rawURL, stringAt(inlineAlts, index), "img[data-src]", scoreInlineWebQueryMediaCandidate(rawURL, stringAt(inlineAlts, index), -2), true)
	}
	for index, rawURL := range stringSliceFromCompatValue(data["inline_lazy_images"]) {
		register(rawURL, stringAt(inlineAlts, index), "img[data-lazy-src]", scoreInlineWebQueryMediaCandidate(rawURL, stringAt(inlineAlts, index), -3), true)
	}
	for index, rawSrcset := range stringSliceFromCompatValue(data["inline_srcsets"]) {
		register(selectWebQueryMediaSrcsetURL(rawSrcset), stringAt(inlineAlts, index), "img[srcset]", scoreInlineWebQueryMediaCandidate(rawSrcset, stringAt(inlineAlts, index), -4), true)
	}

	out := make([]webQueryMediaCandidate, 0, len(candidateByURL))
	for _, candidate := range candidateByURL {
		out = append(out, candidate)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].URL < out[j].URL
		}
		return out[i].Score > out[j].Score
	})
	return out
}

func resolveWebQueryMediaURL(pageURL, raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	base, err := url.Parse(strings.TrimSpace(pageURL))
	if err != nil {
		return ""
	}
	ref, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	resolved := base.ResolveReference(ref)
	resolved.Fragment = ""
	if resolved.Scheme != "http" && resolved.Scheme != "https" {
		return ""
	}
	return resolved.String()
}

func shouldSkipWebQueryMediaCandidate(resolvedURL, alt string, skipDecorative bool) bool {
	lowerURL := strings.ToLower(strings.TrimSpace(resolvedURL))
	if lowerURL == "" {
		return true
	}
	parsed, err := url.Parse(lowerURL)
	if err != nil {
		return true
	}
	switch strings.ToLower(path.Ext(parsed.Path)) {
	case ".svg", ".ico":
		return true
	}
	if !skipDecorative {
		return false
	}
	lowerAlt := strings.ToLower(strings.TrimSpace(alt))
	for _, marker := range []string{"logo", "icon", "avatar", "favicon", "emoji", "sprite", "badge", "placeholder"} {
		if strings.Contains(lowerURL, marker) || strings.Contains(lowerAlt, marker) {
			return true
		}
	}
	return false
}

func scoreInlineWebQueryMediaCandidate(rawURL, alt string, bonus int) int {
	score := 60 + bonus
	lowerURL := strings.ToLower(strings.TrimSpace(rawURL))
	lowerAlt := strings.ToLower(strings.TrimSpace(alt))
	for _, marker := range []string{"hero", "cover", "banner", "feature", "featured", "main", "primary"} {
		if strings.Contains(lowerURL, marker) || strings.Contains(lowerAlt, marker) {
			score += 12
			break
		}
	}
	if len([]rune(strings.TrimSpace(alt))) >= 12 {
		score += 6
	}
	if strings.Contains(lowerURL, "thumbnail") || strings.Contains(lowerURL, "thumb") {
		score -= 6
	}
	return score
}

func selectWebQueryMediaSrcsetURL(srcset string) string {
	for _, item := range strings.Split(strings.TrimSpace(srcset), ",") {
		parts := strings.Fields(strings.TrimSpace(item))
		if len(parts) == 0 {
			continue
		}
		if candidate := strings.TrimSpace(parts[0]); candidate != "" {
			return candidate
		}
	}
	return ""
}

func stringSliceFromCompatValue(raw interface{}) []string {
	items, ok := coerceCompatStringList(raw)
	if !ok {
		if single := strings.TrimSpace(asString(raw)); single != "" {
			return []string{single}
		}
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if value := strings.TrimSpace(asString(item)); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func stringAt(values []string, index int) string {
	if index < 0 || index >= len(values) {
		return ""
	}
	return strings.TrimSpace(values[index])
}

func buildWebQueryMediaAnalysisPlan(args map[string]interface{}, envelope *webQueryEnvelope, candidates []webQueryMediaCandidate, maxItems int) webQueryMediaAnalysisPlan {
	limited := limitWebQueryMediaCandidates(candidates, maxItems)
	switch parseWebQueryMediaMode(args) {
	case webQueryMediaModeOff:
		return webQueryMediaAnalysisPlan{SkipReason: "media enrichment disabled by media_mode=off"}
	case webQueryMediaModeOCR:
		return webQueryMediaAnalysisPlan{
			Candidates:        limited,
			ImageAnalysisMode: imageAnalysisModeOCROnly,
		}
	case webQueryMediaModeFull:
		return webQueryMediaAnalysisPlan{
			Candidates:        limited,
			ImageAnalysisMode: imageAnalysisModeAuto,
		}
	}

	customPrompt := hasCustomWebQueryMediaPrompt(args)
	prompt := parseWebQueryMediaPrompt(args)
	filtered := candidates
	if !customPrompt && isWebQueryMediaPageTextStrong(envelope) {
		informative := make([]webQueryMediaCandidate, 0, len(candidates))
		for _, candidate := range candidates {
			if !looksLowValueWebQueryMediaCandidate(candidate) {
				informative = append(informative, candidate)
			}
		}
		if len(informative) == 0 {
			return webQueryMediaAnalysisPlan{
				SkipReason: "skipped low-value preview images because the page already has strong readable text",
			}
		}
		filtered = informative
	}
	filtered = limitWebQueryMediaCandidates(filtered, maxItems)
	if len(filtered) == 0 {
		return webQueryMediaAnalysisPlan{}
	}

	if customPrompt && promptRequestsWebQueryMediaVision(prompt) {
		return webQueryMediaAnalysisPlan{
			Candidates:        filtered,
			ImageAnalysisMode: imageAnalysisModeAuto,
		}
	}

	textHeavyCount := 0
	for _, candidate := range filtered {
		if looksTextHeavyWebQueryMediaCandidate(candidate) {
			textHeavyCount++
		}
	}
	if (customPrompt && promptPrefersWebQueryMediaOCR(prompt)) || textHeavyCount*2 >= len(filtered) {
		return webQueryMediaAnalysisPlan{
			Candidates:        filtered,
			ImageAnalysisMode: imageAnalysisModeOCRFirst,
		}
	}
	return webQueryMediaAnalysisPlan{
		Candidates:        filtered,
		ImageAnalysisMode: imageAnalysisModeCheapFirst,
	}
}

func limitWebQueryMediaCandidates(candidates []webQueryMediaCandidate, maxItems int) []webQueryMediaCandidate {
	if maxItems <= 0 || len(candidates) <= maxItems {
		return append([]webQueryMediaCandidate(nil), candidates...)
	}
	return append([]webQueryMediaCandidate(nil), candidates[:maxItems]...)
}

func isWebQueryMediaPageTextStrong(envelope *webQueryEnvelope) bool {
	if envelope == nil {
		return false
	}
	content := strings.TrimSpace(envelope.Content)
	if envelope.Page != nil && strings.TrimSpace(envelope.Page.Content) != "" {
		content = strings.TrimSpace(envelope.Page.Content)
	}
	return len([]rune(content)) >= webFetchMinReadableChars
}

func looksTextHeavyWebQueryMediaCandidate(candidate webQueryMediaCandidate) bool {
	haystack := strings.ToLower(strings.Join([]string{candidate.URL, candidate.Alt, candidate.Source}, " "))
	return stringContainsAny(haystack,
		"chart", "graph", "dashboard", "screenshot", "screen", "ui", "interface", "diagram",
		"table", "spreadsheet", "sheet", "document", "doc", "pdf", "slide", "slides",
		"report", "receipt", "invoice", "menu", "form", "code", "console", "terminal", "figure", "infographic")
}

func looksLowValueWebQueryMediaCandidate(candidate webQueryMediaCandidate) bool {
	haystack := strings.ToLower(strings.Join([]string{candidate.URL, candidate.Alt, candidate.Source}, " "))
	if looksTextHeavyWebQueryMediaCandidate(candidate) {
		return false
	}
	if stringContainsAny(haystack, "hero", "cover", "banner", "thumbnail", "thumb", "social", "share", "preview", "featured", "header") {
		return true
	}
	if (candidate.Source == "og:image" || candidate.Source == "twitter:image") && len([]rune(strings.TrimSpace(candidate.Alt))) < 10 {
		return true
	}
	return false
}

func promptPrefersWebQueryMediaOCR(prompt string) bool {
	lower := strings.ToLower(strings.TrimSpace(prompt))
	return stringContainsAny(lower,
		"ocr", "extract the text", "read the text", "transcribe", "text in", "screenshot",
		"chart", "table", "diagram", "document", "invoice", "receipt", "menu", "dashboard", "ui")
}

func promptRequestsWebQueryMediaVision(prompt string) bool {
	lower := strings.ToLower(strings.TrimSpace(prompt))
	return stringContainsAny(lower,
		"what is shown", "describe the image", "describe the visual", "photo", "person", "people",
		"scene", "product", "device", "layout", "color", "style", "appearance", "look like")
}

func stringContainsAny(input string, markers ...string) bool {
	for _, marker := range markers {
		if marker != "" && strings.Contains(input, marker) {
			return true
		}
	}
	return false
}

func (t *WebTool) analyzeWebQueryMediaCandidates(ctx context.Context, args map[string]interface{}, candidates []webQueryMediaCandidate, analysisMode string) ([]webQueryMediaItem, string, error) {
	if len(candidates) == 0 {
		return nil, "", nil
	}
	urls := make([]string, 0, len(candidates))
	candidateByURL := make(map[string]webQueryMediaCandidate, len(candidates))
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.URL) == "" {
			continue
		}
		urls = append(urls, candidate.URL)
		candidateByURL[candidate.URL] = candidate
	}
	if len(urls) == 0 {
		return nil, "", nil
	}
	raw, err := t.image.Execute(ctx, map[string]interface{}{
		"action":        "review",
		"urls":          urls,
		"prompt":        parseWebQueryMediaPrompt(args),
		"max_images":    len(urls),
		"analysis_mode": analysisMode,
	})
	if err != nil {
		return nil, "", err
	}
	payload := map[string]interface{}{}
	if err := decodeToolJSONResult(raw, &payload); err != nil {
		return nil, "", err
	}
	items := collectWebQueryMediaItemsFromImagePayload(payload, candidateByURL)
	summary := strings.TrimSpace(firstNonEmpty(asString(payload["summary"]), asString(payload["analysis"])))
	if summary == "" && len(items) == 1 {
		summary = items[0].Analysis
	}
	return items, summary, nil
}

func collectWebQueryMediaItemsFromImagePayload(payload map[string]interface{}, candidateByURL map[string]webQueryMediaCandidate) []webQueryMediaItem {
	if len(payload) == 0 {
		return nil
	}
	if rawItems, ok := payload["items"].([]interface{}); ok {
		out := make([]webQueryMediaItem, 0, len(rawItems))
		for _, rawItem := range rawItems {
			item, ok := rawItem.(map[string]interface{})
			if !ok {
				continue
			}
			okValue, _ := item["ok"].(bool)
			if !okValue {
				continue
			}
			result, _ := item["result"].(map[string]interface{})
			analysis := strings.TrimSpace(firstNonEmpty(asString(result["analysis"]), asString(result["summary"]), asString(result["text"])))
			if analysis == "" {
				continue
			}
			targetURL := strings.TrimSpace(firstNonEmpty(asString(result["source_url"]), asString(item["input"])))
			candidate := candidateByURL[targetURL]
			out = append(out, webQueryMediaItem{
				URL:      targetURL,
				Alt:      candidate.Alt,
				Source:   candidate.Source,
				Mode:     strings.TrimSpace(firstNonEmpty(asString(result["mode"]), asString(item["mode"]))),
				Analysis: truncateRunes(analysis, webQueryMediaItemAnalysisMaxChars),
			})
		}
		return out
	}
	analysis := strings.TrimSpace(firstNonEmpty(asString(payload["analysis"]), asString(payload["summary"]), asString(payload["text"])))
	if analysis == "" {
		return nil
	}
	targetURL := strings.TrimSpace(asString(payload["source_url"]))
	if targetURL == "" {
		for url := range candidateByURL {
			targetURL = url
			break
		}
	}
	candidate := candidateByURL[targetURL]
	return []webQueryMediaItem{{
		URL:      targetURL,
		Alt:      candidate.Alt,
		Source:   candidate.Source,
		Mode:     strings.TrimSpace(asString(payload["mode"])),
		Analysis: truncateRunes(analysis, webQueryMediaItemAnalysisMaxChars),
	}}
}

func analyzeWebQueryReadResponse(resp webReadResponse) (needsBrowser bool, strong bool) {
	criticalWarnings := false
	for _, code := range resp.WarningCodes {
		switch strings.TrimSpace(code) {
		case webFetchWarningCodeLoginWall, webFetchWarningCodeChallenge, webFetchWarningCodeBrowserRequired:
			needsBrowser = true
			criticalWarnings = true
		case "http_status":
			criticalWarnings = true
		}
	}
	contentChars := len([]rune(strings.TrimSpace(resp.Content)))
	if webQueryLooksLikeRedirectInterstitial(resp.Content) {
		return needsBrowser, false
	}
	if strings.EqualFold(strings.TrimSpace(resp.Source), webAccessSourceBrowser) && contentChars > 0 {
		return needsBrowser, true
	}
	if resp.StatusCode >= 400 || contentChars == 0 || criticalWarnings {
		return needsBrowser, false
	}
	if contentChars >= webFetchMinReadableChars {
		return needsBrowser, true
	}
	if contentChars >= 160 && strings.TrimSpace(resp.Title) != "" && len(resp.WarningCodes) == 0 {
		return needsBrowser, true
	}
	return needsBrowser, false
}

func scoreWebQueryReadResponse(resp webReadResponse) float64 {
	score := float64(len([]rune(strings.TrimSpace(resp.Content))))
	if strings.TrimSpace(resp.Title) != "" {
		score += 80
	}
	switch strings.TrimSpace(resp.Source) {
	case webAccessSourceBrowser:
		score += 35
	case webAccessSourceProxyFetcher:
		score += 20
	case webAccessSourceLightpandaShim:
		score += 16
	case webAccessSourceHTTP:
		score += 10
	}
	score -= float64(len(resp.WarningCodes) * 60)
	if resp.StatusCode >= 400 {
		score -= 120
	}
	if resp.Truncated {
		score -= 25
	}
	if webQueryLooksLikeRedirectInterstitial(resp.Content) {
		score -= 240
	}
	return score
}

func scoreWebQueryCandidate(query string, candidate webQueryCandidate) float64 {
	return scoreWebQueryCandidateWithProfile(newWebQueryScoreProfile(query), candidate)
}

func scoreWebQueryCandidateWithProfile(profile webQueryScoreProfile, candidate webQueryCandidate) float64 {
	title := firstNonEmpty(candidate.Resolved.Response.Title, candidate.Search.Title)
	snippet := firstNonEmpty(candidate.Search.Description, candidate.Resolved.Response.Content)
	content := strings.TrimSpace(candidate.Resolved.Response.Content)
	score := float64(36 - minWebQueryInt(candidate.Rank*5, 24))
	score += 28 * webQueryTextOverlapWithProfile(profile, title)
	score += 12 * webQueryTextOverlapWithProfile(profile, snippet)
	score += 16 * webQueryTextOverlapWithProfile(profile, truncateRunes(content, 600))
	score += math.Min(20, float64(len([]rune(content)))/180)
	score -= float64(len(candidate.Resolved.Response.WarningCodes) * 6)
	if webQueryLooksLikeRedirectInterstitial(content) {
		score -= 42
	}
	return score
}

func newWebQueryReuseCoordinator() *webQueryReuseCoordinator {
	return &webQueryReuseCoordinator{browserTargetByHost: make(map[string]string)}
}

func (c *webQueryReuseCoordinator) browserTargetID(targetURL string) string {
	if c == nil {
		return ""
	}
	host := webQueryReuseHost(targetURL)
	if host == "" {
		return ""
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return strings.TrimSpace(c.browserTargetByHost[host])
}

func (c *webQueryReuseCoordinator) remember(targetURL string, resp webReadResponse) {
	if c == nil {
		return
	}
	targetID := strings.TrimSpace(resp.BrowserTargetID)
	if targetID == "" {
		return
	}
	host := firstNonEmpty(webQueryReuseHost(resp.FinalURL), webQueryReuseHost(resp.URL), webQueryReuseHost(targetURL))
	if host == "" {
		return
	}
	c.mu.Lock()
	c.browserTargetByHost[host] = targetID
	c.mu.Unlock()
}

func webQueryReuseHost(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(parsed.Hostname()))
}

func webQueryLooksLikeRedirectInterstitial(content string) bool {
	normalized := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(content)), " "))
	if normalized == "" {
		return false
	}
	if strings.Contains(normalized, "please click here if the page does not redirect automatically") {
		return true
	}
	if strings.Contains(normalized, "if the page does not redirect automatically") && strings.Contains(normalized, "click here") {
		return true
	}
	if strings.Contains(normalized, "you are being redirected") && strings.Contains(normalized, "click here") {
		return true
	}
	return false
}

func webQueryTextOverlap(query, text string) float64 {
	return webQueryTextOverlapWithProfile(newWebQueryScoreProfile(query), text)
}

func newWebQueryScoreProfile(query string) webQueryScoreProfile {
	queryTokens := uniqueWebQueryTokens(webQueryTokens(query))
	if len(queryTokens) == 0 {
		return webQueryScoreProfile{}
	}
	queryTokenSet := make(map[string]struct{}, len(queryTokens))
	for _, token := range queryTokens {
		queryTokenSet[token] = struct{}{}
	}
	return webQueryScoreProfile{
		QueryTokens:   queryTokens,
		QueryTokenSet: queryTokenSet,
	}
}

func webQueryTextOverlapWithProfile(profile webQueryScoreProfile, text string) float64 {
	if len(profile.QueryTokens) == 0 {
		return 0
	}
	textTokens := webQueryTokens(text)
	if len(textTokens) == 0 {
		return 0
	}
	matches := 0
	matched := make(map[string]struct{}, len(profile.QueryTokens))
	queryTokenSet := profile.QueryTokenSet
	if len(queryTokenSet) == 0 {
		queryTokenSet = make(map[string]struct{}, len(profile.QueryTokens))
		for _, token := range profile.QueryTokens {
			queryTokenSet[token] = struct{}{}
		}
	}
	for _, token := range textTokens {
		if _, ok := queryTokenSet[token]; !ok {
			continue
		}
		if _, seen := matched[token]; seen {
			continue
		}
		matched[token] = struct{}{}
		matches++
	}
	return float64(matches) / float64(len(profile.QueryTokens))
}

func webQueryTokens(text string) []string {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return nil
	}
	return strings.FieldsFunc(text, func(r rune) bool {
		switch {
		case r >= 'a' && r <= 'z':
			return false
		case r >= '0' && r <= '9':
			return false
		case r >= 'A' && r <= 'Z':
			return false
		default:
			return true
		}
	})
}

func uniqueWebQueryTokens(tokens []string) []string {
	if len(tokens) == 0 {
		return nil
	}
	out := make([]string, 0, len(tokens))
	seen := make(map[string]struct{}, len(tokens))
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		out = append(out, token)
	}
	return out
}

func warningsFromLists(codes, messages []string) []webQueryWarning {
	if len(codes) == 0 && len(messages) == 0 {
		return []webQueryWarning{}
	}
	out := make([]webQueryWarning, 0, maxWebQueryInt(len(codes), len(messages)))
	for idx := 0; idx < maxWebQueryInt(len(codes), len(messages)); idx++ {
		code := ""
		if idx < len(codes) {
			code = strings.TrimSpace(codes[idx])
		}
		message := ""
		if idx < len(messages) {
			message = strings.TrimSpace(messages[idx])
		}
		if code == "" && message == "" {
			continue
		}
		out = append(out, webQueryWarning{Code: code, Message: message})
	}
	return out
}

func addWebQueryWarning(warnings *[]webQueryWarning, code, message string) {
	code = strings.TrimSpace(code)
	message = strings.TrimSpace(message)
	if warnings == nil || (code == "" && message == "") {
		return
	}
	*warnings = append(*warnings, webQueryWarning{Code: code, Message: message})
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return strings.TrimSpace(err.Error())
}

func truncateRunes(input string, limit int) string {
	if limit <= 0 {
		return input
	}
	runes := []rune(input)
	if len(runes) <= limit {
		return input
	}
	return string(runes[:limit])
}

func firstCompatValueOrNil(args map[string]interface{}, keys ...string) interface{} {
	value, ok := compatArgValue(args, keys...)
	if !ok {
		return nil
	}
	return value
}

func firstNonNilErr(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func cloneStringAnyMap(input map[string]interface{}) map[string]interface{} {
	if len(input) == 0 {
		return map[string]interface{}{}
	}
	cloned := make(map[string]interface{}, len(input))
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
}

func firstProviderOrEmpty(providers []string) string {
	if len(providers) == 0 {
		return ""
	}
	return strings.TrimSpace(providers[0])
}

func webQueryAttemptStatus(err error) string {
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "canceled"
	default:
		return "error"
	}
}

func hideCanceledSearchAttemptsAfterSearchSuccess(attempts []webQueryAttempt) []webQueryAttempt {
	if len(attempts) == 0 {
		return attempts
	}

	filtered := attempts[:0]
	for _, attempt := range attempts {
		if attempt.Stage == "search" && attempt.Status == "canceled" {
			continue
		}
		filtered = append(filtered, attempt)
	}
	return filtered
}

func minWebQueryInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxWebQueryInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func ternary[T any](cond bool, a, b T) T {
	if cond {
		return a
	}
	return b
}
