package autoreply

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill/builtin"
)

// SkillAdapter adapts autoreply Service to builtin.AutoreplyServiceInterface.
type SkillAdapter struct {
	svc *Service
}

// NewSkillAdapter creates a new skill adapter for the autoreply service.
func NewSkillAdapter(svc *Service) *SkillAdapter {
	return &SkillAdapter{svc: svc}
}

func (a *SkillAdapter) Create(name string, triggerType string, triggerValue string, responses []string, priority int) (builtin.AutoreplyRuleInfo, error) {
	rule, err := a.svc.Create(name, TriggerType(triggerType), triggerValue, responses, priority)
	if err != nil {
		return builtin.AutoreplyRuleInfo{}, err
	}
	return toRuleInfo(rule), nil
}

func (a *SkillAdapter) List() []builtin.AutoreplyRuleInfo {
	rules := a.svc.List()
	results := make([]builtin.AutoreplyRuleInfo, len(rules))
	for i, r := range rules {
		results[i] = toRuleInfo(r)
	}
	return results
}

func (a *SkillAdapter) Delete(id string) error {
	return a.svc.Delete(id)
}

func (a *SkillAdapter) Enable(id string) error {
	return a.svc.Enable(id)
}

func (a *SkillAdapter) Disable(id string) error {
	return a.svc.Disable(id)
}

func (a *SkillAdapter) Test(message, channel string) (string, string, error) {
	response, rule, err := a.svc.Test(message, channel)
	if err != nil {
		return "", "", err
	}
	ruleID := ""
	if rule != nil {
		ruleID = rule.ID
	}
	return response, ruleID, nil
}

func toRuleInfo(r *Rule) builtin.AutoreplyRuleInfo {
	return builtin.AutoreplyRuleInfo{
		ID:           r.ID,
		Name:         r.Name,
		TriggerType:  string(r.TriggerType),
		TriggerValue: r.TriggerValue,
		Responses:    r.Responses,
		Priority:     r.Priority,
		Enabled:      r.Enabled,
		MatchCount:   r.MatchCount,
	}
}
