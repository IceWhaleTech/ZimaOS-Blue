package tools

import (
	"context"
	"strconv"
	"strings"
)

type WebTaskKind string

const (
	WebTaskKindSearch   WebTaskKind = "search"
	WebTaskKindRetrieve WebTaskKind = "retrieve"
	WebTaskKindOperate  WebTaskKind = "operate"
)

type WebCapability struct {
	NeedsRawHTML        bool
	NeedsJS             bool
	NeedsLogin          bool
	NeedsInteraction    bool
	NeedsNetworkObserve bool
	HasBrowserSession   bool
	AntiBotLevel        int
}

type WebTask struct {
	Kind       WebTaskKind
	Query      string
	URL        string
	Action     string
	MaxResults int
	MaxChars   int
	Capability WebCapability
	SessionID  string
	TargetID   string
}

type PlanStep struct {
	Kind     WebTaskKind `json:"kind"`
	Runtime  string      `json:"runtime"`
	Provider string      `json:"provider,omitempty"`
	Reason   string      `json:"reason,omitempty"`
}

type ExecutionPlan struct {
	Kind           WebTaskKind       `json:"kind"`
	PrimaryRuntime string            `json:"primary_runtime"`
	Steps          []PlanStep        `json:"steps,omitempty"`
	ExtractorMode  string            `json:"extractor_mode,omitempty"`
	TraceLabels    map[string]string `json:"trace_labels,omitempty"`
}

type RuntimeExecutor interface {
	ExecuteStep(context.Context, PlanStep, WebTask) (any, error)
}

type webExecutionPlanContextKey struct{}

func WithWebExecutionPlan(ctx context.Context, plan ExecutionPlan) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, webExecutionPlanContextKey{}, cloneExecutionPlan(plan))
}

func WebExecutionPlanFromContext(ctx context.Context) (ExecutionPlan, bool) {
	if ctx == nil {
		return ExecutionPlan{}, false
	}
	plan, ok := ctx.Value(webExecutionPlanContextKey{}).(ExecutionPlan)
	if !ok {
		return ExecutionPlan{}, false
	}
	return cloneExecutionPlan(plan), true
}

type webRetrievePlanningHints struct {
	AutoAllowed       bool
	EffectiveTargetID string
	HasBrowserTarget  bool
	HasStrategy       bool
	Strategy          DomainStrategy
	HasAdapter        bool
	Adapter           AdapterManifest
	HasSessionCore    bool
}

func buildWebSearchProviderPlan(providers []string, query string, maxResults int) ExecutionPlan {
	providers = normalizeProviderList(providers)
	steps := make([]PlanStep, 0, len(providers))
	for _, provider := range providers {
		if strings.TrimSpace(provider) == "" {
			continue
		}
		steps = append(steps, PlanStep{
			Kind:     WebTaskKindSearch,
			Runtime:  "search_provider:" + strings.TrimSpace(provider),
			Provider: strings.TrimSpace(provider),
			Reason:   "search provider fanout",
		})
	}
	primary := ""
	if len(steps) > 0 {
		primary = steps[0].Runtime
	}
	return ExecutionPlan{
		Kind:           WebTaskKindSearch,
		PrimaryRuntime: primary,
		Steps:          steps,
		TraceLabels: map[string]string{
			"query":       strings.TrimSpace(query),
			"max_results": itoaSafe(maxResults),
		},
	}
}

func buildWebSearchBrowserFallbackPlan(engine, query string, maxResults int, reason string) ExecutionPlan {
	engine = normalizeBrowserSearchFallbackEngine(engine)
	return ExecutionPlan{
		Kind:           WebTaskKindSearch,
		PrimaryRuntime: webAccessLaneBrowser,
		Steps: []PlanStep{{
			Kind:    WebTaskKindSearch,
			Runtime: webAccessLaneBrowser,
			Reason:  firstNonEmpty(strings.TrimSpace(reason), "browser-backed search rescue"),
		}},
		TraceLabels: map[string]string{
			"engine":      engine,
			"query":       strings.TrimSpace(query),
			"max_results": itoaSafe(maxResults),
			"reason":      strings.TrimSpace(reason),
		},
	}
}

func (o *FetchOrchestrator) planExecution(_ context.Context, req FetchRequest, hints webRetrievePlanningHints) ExecutionPlan {
	if o == nil {
		return ExecutionPlan{}
	}
	task := WebTask{
		Kind:      WebTaskKindRetrieve,
		URL:       req.URL,
		MaxChars:  req.MaxChars,
		TargetID:  hints.EffectiveTargetID,
		SessionID: hints.EffectiveTargetID,
		Capability: WebCapability{
			NeedsLogin:          hints.HasSessionCore || hints.HasBrowserTarget || (hints.HasStrategy && hints.Strategy.LoginRequired),
			NeedsJS:             strings.TrimSpace(req.PreferredLane) == webAccessLaneBrowser,
			NeedsInteraction:    strings.TrimSpace(req.PreferredLane) == webAccessLaneBrowser,
			NeedsNetworkObserve: hints.HasAdapter || (hints.HasStrategy && hints.Strategy.NeedsNetworkObserve),
			HasBrowserSession:   hints.HasBrowserTarget,
			AntiBotLevel:        antiBotLevelForPlanning(hints),
		},
	}
	_ = task

	ordered := make([]PlanStep, 0, 5)
	host := webFetchHostForURL(req.URL)
	browserPreferred := webFetchPrefersBrowserHost(host) && req.AllowBrowser
	lightpandaAllowed := webFetchSupportsLightpandaHost(host)
	appendStep := func(runtime, reason string) {
		runtime = strings.TrimSpace(runtime)
		if runtime == "" {
			return
		}
		for _, existing := range ordered {
			if existing.Runtime == runtime {
				return
			}
		}
		ordered = append(ordered, PlanStep{
			Kind:    WebTaskKindRetrieve,
			Runtime: runtime,
			Reason:  reason,
		})
	}

	switch strings.TrimSpace(req.PreferredLane) {
	case webAccessLaneHTTPNative:
		appendStep(webAccessLaneHTTPNative, "explicit preferred lane")
	case webAccessLaneHTTP:
		if req.AllowSession && hints.HasBrowserTarget {
			appendStep(webFetchStrategySession, "explicit preferred lane with browser session")
		}
		appendStep(webAccessLaneHTTP, "explicit preferred lane")
	case webAccessLaneLightpandaShim:
		if lightpandaAllowed {
			appendStep(webAccessLaneLightpandaShim, "explicit preferred lane")
		}
	case webAccessLaneBrowser:
		appendStep(webAccessLaneBrowser, "explicit preferred lane")
	case webAccessLaneProxyFetcher:
		appendStep(webAccessLaneProxyFetcher, "explicit preferred lane")
	}
	if len(ordered) == 0 {
		if browserPreferred {
			if req.AllowSession && hints.HasBrowserTarget {
				appendStep(webFetchStrategySession, "host prefers browser with reusable session")
			}
			appendStep(webAccessLaneBrowser, "host prefers browser runtime")
		}
		if hints.AutoAllowed && hints.HasAdapter {
			switch strings.TrimSpace(hints.Adapter.PreferredLane) {
			case webAccessLaneLightpandaShim:
				if lightpandaAllowed {
					appendStep(webAccessLaneLightpandaShim, "adapter preferred lane")
				}
			case webAccessLaneBrowser, webAccessLaneProxyFetcher, webAccessLaneHTTP, webAccessLaneHTTPNative:
				appendStep(hints.Adapter.PreferredLane, "adapter preferred lane")
			case webFetchStrategySession:
				if req.AllowSession && hints.HasBrowserTarget {
					appendStep(webFetchStrategySession, "adapter preferred reusable session")
				}
			}
		}
		if hints.AutoAllowed && hints.HasStrategy {
			switch strings.TrimSpace(hints.Strategy.PreferredLane) {
			case webAccessLaneBrowser, webAccessLaneProxyFetcher, webAccessLaneHTTPNative, webAccessLaneHTTP:
				appendStep(hints.Strategy.PreferredLane, "domain strategy preferred lane")
			case webAccessLaneLightpandaShim:
				if lightpandaAllowed {
					appendStep(webAccessLaneLightpandaShim, "domain strategy preferred lane")
				}
			case webFetchStrategySession:
				if req.AllowSession && hints.HasBrowserTarget {
					appendStep(webFetchStrategySession, "domain strategy reusable session")
				}
			}
		}
		if req.AllowSession && hints.HasBrowserTarget {
			appendStep(webFetchStrategySession, ternaryString(hints.HasSessionCore, "session core reusable HTTP lane", "browser session reusable HTTP lane"))
		}
		appendStep(webAccessLaneHTTP, "baseline HTTP retrieve")
		appendStep(webAccessLaneHTTPNative, "native HTTP hedge")
		if lightpandaAllowed {
			appendStep(webAccessLaneLightpandaShim, ternaryString(hints.HasSessionCore, "lightpanda with session core", "lightpanda readable fallback"))
		}
		if req.AllowProxy {
			appendStep(webAccessLaneProxyFetcher, "proxy readable fallback")
		}
		if req.AllowBrowser {
			appendStep(webAccessLaneBrowser, "interactive browser fallback")
		}
	}

	primary := ""
	if len(ordered) > 0 {
		primary = ordered[0].Runtime
	}
	return ExecutionPlan{
		Kind:           WebTaskKindRetrieve,
		PrimaryRuntime: primary,
		Steps:          ordered,
		ExtractorMode:  req.Mode,
		TraceLabels: map[string]string{
			"host":               host,
			"preferred_lane":     strings.TrimSpace(req.PreferredLane),
			"auto_allowed":       boolString(hints.AutoAllowed),
			"has_browser_target": boolString(hints.HasBrowserTarget),
			"session_core":       boolString(hints.HasSessionCore),
		},
	}
}

func antiBotLevelForPlanning(hints webRetrievePlanningHints) int {
	level := 0
	if hints.HasSessionCore {
		level++
	}
	if hints.HasStrategy && hints.Strategy.LoginRequired {
		level++
	}
	if hints.HasAdapter && hints.Adapter.NetworkObserved {
		level++
	}
	if hints.HasStrategy && hints.Strategy.NeedsRealBrowser {
		level++
	}
	return level
}

func cloneExecutionPlan(plan ExecutionPlan) ExecutionPlan {
	cloned := plan
	if len(plan.Steps) > 0 {
		cloned.Steps = append([]PlanStep(nil), plan.Steps...)
	}
	if len(plan.TraceLabels) > 0 {
		cloned.TraceLabels = cloneStringMap(plan.TraceLabels)
	}
	return cloned
}

func boolString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func ternaryString(condition bool, yes, no string) string {
	if condition {
		return yes
	}
	return no
}

func itoaSafe(v int) string {
	return strconv.Itoa(v)
}
