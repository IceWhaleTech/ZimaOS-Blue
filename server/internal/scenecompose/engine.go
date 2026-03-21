package scenecompose

import (
	"context"
	"fmt"
)

type Engine struct {
	planner  *Planner
	searcher *AssetSearcher
	cutout   *CutoutStrategy
	renderer *Renderer
}

func NewEngine(planner *Planner, searcher *AssetSearcher, cutout *CutoutStrategy, renderer *Renderer) *Engine {
	return &Engine{
		planner:  planner,
		searcher: searcher,
		cutout:   cutout,
		renderer: renderer,
	}
}

func (e *Engine) SetLLM(llmCaller LLMCaller) {
	if e == nil || e.planner == nil {
		return
	}
	e.planner.SetLLM(llmCaller)
}

func (e *Engine) Compose(ctx context.Context, req ComposeRequest) (*ComposeResult, error) {
	if e == nil || e.planner == nil || e.searcher == nil || e.renderer == nil {
		return nil, fmt.Errorf("scene compose engine is not configured")
	}
	plan, err := e.planner.Plan(ctx, req)
	if err != nil {
		return nil, err
	}
	background, bgQuery, bgRef, err := e.searcher.FindBackground(ctx, plan)
	if err != nil {
		return nil, err
	}

	usedAssets := []AssetRef{bgRef}
	debug := ComposeDebugInfo{
		Plan:            plan,
		BackgroundQuery: bgQuery,
	}

	renderInputs := make([]renderForeground, 0, len(plan.Foreground))
	for _, fg := range plan.Foreground {
		asset, query, assetRef, findErr := e.searcher.FindForeground(ctx, plan, fg)
		if findErr != nil {
			return nil, findErr
		}
		usedAssets = append(usedAssets, assetRef)
		debug.ForegroundQueries = append(debug.ForegroundQueries, query)

		cutResult := &CutoutResult{Image: cloneImage(asset.Image)}
		if !asset.HasAlpha && e.cutout != nil {
			cutResult, findErr = e.cutout.Apply(ctx, asset)
			if findErr != nil {
				cutResult = &CutoutResult{Image: cloneImage(asset.Image)}
			}
		}
		renderInputs = append(renderInputs, renderForeground{
			Plan:  fg,
			Asset: asset,
			Cut:   cutResult,
		})
	}

	return &ComposeResult{
		Image:      e.renderer.Render(req.Width, req.Height, background, renderInputs),
		UsedAssets: usedAssets,
		Debug:      debug,
	}, nil
}
