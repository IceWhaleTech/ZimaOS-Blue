package deepresearch

import (
	"context"
	"strings"
)

type RoutePolicy struct {
	DefaultMode     RouteMode
	AllowExperiment bool
	AllowHybrid     bool
}

type ExperimentBackend interface {
	Run(ctx context.Context, req ExperimentRequest) (*ExperimentResult, error)
}

func defaultRoutePolicy() RoutePolicy {
	return RoutePolicy{
		DefaultMode:     RouteModeWeb,
		AllowExperiment: true,
		AllowHybrid:     true,
	}
}

func normalizeRouteMode(raw RouteMode) (RouteMode, bool) {
	switch RouteMode(strings.ToLower(strings.TrimSpace(string(raw)))) {
	case "", RouteModeAuto:
		return RouteModeAuto, true
	case RouteModeWeb:
		return RouteModeWeb, true
	case RouteModeExperiment:
		return RouteModeExperiment, true
	case RouteModeHybrid:
		return RouteModeHybrid, true
	default:
		return "", false
	}
}

func normalizeRoutePolicy(policy RoutePolicy) RoutePolicy {
	mode, ok := normalizeRouteMode(policy.DefaultMode)
	if !ok || mode == RouteModeAuto {
		policy.DefaultMode = RouteModeWeb
	} else {
		policy.DefaultMode = mode
	}
	return policy
}

func classifyRouteMode(query string) (RouteMode, string) {
	lower := strings.ToLower(strings.TrimSpace(query))
	if lower == "" {
		return RouteModeWeb, "empty query defaults to web research"
	}

	hybridPhrases := []string{
		"first research",
		"research first",
		"then validate",
		"and then validate",
		"then benchmark",
		"research and benchmark",
		"research and verify",
		"先调研",
		"先研究",
		"再验证",
		"再跑实验",
		"再benchmark",
		"调研后验证",
	}
	if containsAnyCue(lower, hybridPhrases) {
		return RouteModeHybrid, "query explicitly asks for both external research and experiment validation"
	}

	experimentCues := []string{
		"benchmark",
		"ablation",
		"reproduce",
		"replicate",
		"hyperparameter",
		"grid search",
		"tune",
		"training run",
		"run experiment",
		"run experiments",
		"evaluate in repo",
		"validate in repo",
		"train.py",
		"调参",
		"消融",
		"复现",
		"跑实验",
		"实验验证",
		"基准测试",
		"训练验证",
		"在仓库里验证",
	}
	webCues := []string{
		"latest",
		"news",
		"sources",
		"citations",
		"report",
		"summary",
		"deep research",
		"最新",
		"新闻",
		"来源",
		"引用",
		"报告",
		"汇总",
		"资料",
		"调研",
	}

	hasExperiment := containsAnyCue(lower, experimentCues)
	hasWeb := containsAnyCue(lower, webCues)

	if hasExperiment && hasWeb {
		return RouteModeHybrid, "query mixes citation-oriented research with repository or experiment validation cues"
	}
	if hasExperiment {
		return RouteModeExperiment, "query asks for experiments, benchmarking, reproduction, or code validation"
	}
	return RouteModeWeb, "query focuses on external research, evidence gathering, or report synthesis"
}

func resolveRouteMode(query string, requested RouteMode, policy RoutePolicy, hasExperimentBackend bool) (RouteMode, RouteMode, string, error) {
	requestedMode, ok := normalizeRouteMode(requested)
	if !ok {
		return "", "", "", ErrInvalidRouteMode
	}
	policy = normalizeRoutePolicy(policy)

	effective := requestedMode
	reason := ""
	if effective == RouteModeAuto {
		effective, reason = classifyRouteMode(query)
		if effective == RouteModeAuto {
			effective = policy.DefaultMode
		}
	} else {
		reason = "explicit route_mode requested by caller"
	}

	switch effective {
	case RouteModeExperiment:
		if !policy.AllowExperiment {
			return requestedMode, RouteModeWeb, "experiment route disabled by policy; falling back to web research", nil
		}
		if !hasExperimentBackend {
			return requestedMode, RouteModeWeb, "experiment backend is not configured; falling back to web research", nil
		}
	case RouteModeHybrid:
		if !policy.AllowHybrid {
			return requestedMode, RouteModeWeb, "hybrid route disabled by policy; falling back to web research", nil
		}
		if !policy.AllowExperiment {
			return requestedMode, RouteModeWeb, "experiment route disabled by policy; falling back to web research", nil
		}
		if !hasExperimentBackend {
			return requestedMode, RouteModeWeb, "hybrid route needs an experiment backend; falling back to web research", nil
		}
	case RouteModeWeb:
		// ok
	default:
		effective = policy.DefaultMode
		reason = "unknown auto route classified to default web research"
	}
	if reason == "" {
		reason = "route resolved"
	}
	return requestedMode, effective, reason, nil
}

func containsAnyCue(s string, cues []string) bool {
	for _, cue := range cues {
		if strings.Contains(s, cue) {
			return true
		}
	}
	return false
}
