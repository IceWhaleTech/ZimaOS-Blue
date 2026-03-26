package bootstrap

import (
	"context"
	"fmt"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
)

type autoHarnessSubmitterAdapter struct {
	controller *harness.Controller
}

func newHarnessAutoHarnessSubmitter(controller *harness.Controller) *autoHarnessSubmitterAdapter {
	if controller == nil {
		return nil
	}
	return &autoHarnessSubmitterAdapter{controller: controller}
}

func (a *autoHarnessSubmitterAdapter) SubmitAutoHarnessConversationQuickEval(ctx context.Context, spec serverpkg.AutoHarnessQuickEvalSpec) (string, error) {
	if a == nil || a.controller == nil {
		return "", fmt.Errorf("harness controller is not configured")
	}

	groupSpec := harness.RunGroupSpec{
		Kind:        harness.RunGroupKindEval,
		Title:       spec.Title,
		Subject:     spec.Subject,
		OwnerUserID: spec.OwnerUserID,
		Metadata:    cloneInterfaceMap(spec.Metadata),
		SchedulerConfig: harness.GroupSchedulerConfig{
			MaxConcurrency: 1,
			MaxAttempts:    1,
		},
		ScoringConfig: harness.GroupScoringConfig{
			Mode:          harness.ScoringModeRule,
			RuleProfile:   spec.Scoring.RuleProfile,
			PassThreshold: spec.Scoring.PassThreshold,
		},
		Items: make([]harness.RunGroupItemSpec, 0, len(spec.Items)),
	}

	for _, item := range spec.Items {
		groupSpec.Items = append(groupSpec.Items, harness.RunGroupItemSpec{
			RunKind:  harness.RunKind(item.RunKind),
			Profile:  item.Profile,
			Input:    cloneInterfaceMap(item.Input),
			Expected: cloneInterfaceMap(item.Expected),
			Metadata: cloneInterfaceMap(item.Metadata),
		})
	}

	group, err := a.controller.SubmitGroup(ctx, groupSpec)
	if err != nil {
		return "", err
	}
	return group.ID, nil
}

func cloneInterfaceMap(in map[string]interface{}) map[string]interface{} {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
