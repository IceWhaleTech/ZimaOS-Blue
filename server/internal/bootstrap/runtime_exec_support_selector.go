package bootstrap

import (
	"context"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func newRuntimeExecSkillSelector(source runtimeExecSkillSelectionSource) tools.SkillSelectFunc {
	if source == nil {
		return nil
	}
	return func(ctx context.Context, query string) tools.SkillSelectionDecision {
		settings := source.GetSettingsHandler()
		selector := source.GetSkillSelector()
		if selector == nil {
			return tools.SkillSelectionDecision{}
		}
		decision, err := selector.Select(ctx, query, newRuntimeExecSelectionOptions(settings))
		if err != nil {
			return tools.SkillSelectionDecision{}
		}
		out := tools.SkillSelectionDecision{
			SelectedSkill: decision.SelectedSkill,
			Confidence:    decision.Confidence,
			NeedClarify:   decision.NeedClarify,
			Reason:        decision.Reason,
		}
		for _, candidate := range decision.Candidates {
			out.Candidates = append(out.Candidates, candidate.Name)
		}
		return out
	}
}

func newRuntimeExecSelectionOptions(settings *serverpkg.SettingsHandler) agentcore.SelectOptions {
	opts := agentcore.SelectOptions{
		Mode:                agentcore.SkillSelectorModeHybrid,
		EnableRerank:        true,
		ConfidenceThreshold: 0.78,
	}
	if settings == nil {
		return opts
	}
	opts.Mode = settings.GetSkillSelectorMode()
	opts.EnableRerank = settings.GetEffectiveSkillRerankEnabled()
	opts.ConfidenceThreshold = settings.GetSkillSelectorConfidenceThreshold()
	return opts
}
