package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

func (c *Controller) PromoteGroup(ctx context.Context, groupID string, spec GroupPromotionSpec) (*GroupPromotionResult, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	group, err := c.GetGroup(ctx, strings.TrimSpace(groupID))
	if err != nil {
		return nil, err
	}
	items, err := c.store.ListGroupItems(ctx, group.ID)
	if err != nil {
		return nil, err
	}

	datasetName := strings.TrimSpace(spec.DatasetName)
	if datasetName == "" {
		return nil, fmt.Errorf("dataset_name is required")
	}
	evalName := strings.TrimSpace(spec.EvalName)
	if evalName == "" {
		return nil, fmt.Errorf("eval_name is required")
	}

	subject := firstNonEmpty(spec.Subject, group.Subject)
	manifest, err := buildPromotionManifest(group, items, datasetName, subject)
	if err != nil {
		return nil, err
	}
	manifestRaw, err := datasetManifestMap(manifest)
	if err != nil {
		return nil, err
	}

	defaultRunKind := manifest.Defaults.RunKind
	if defaultRunKind == "" && len(items) > 0 {
		defaultRunKind = items[0].RunKind
	}
	defaultProfile := strings.TrimSpace(manifest.Defaults.Profile)
	if defaultProfile == "" && len(items) > 0 {
		defaultProfile = strings.TrimSpace(items[0].Profile)
	}

	dataset, err := c.CreateDataset(ctx, DatasetSpec{
		Name:           datasetName,
		Description:    strings.TrimSpace(spec.Description),
		OwnerUserID:    group.OwnerUserID,
		Subject:        subject,
		DefaultRunKind: defaultRunKind,
		DefaultProfile: defaultProfile,
		Metadata: map[string]interface{}{
			"promoted_from_group_id": group.ID,
		},
	})
	if err != nil {
		return nil, err
	}

	version, err := c.CreateDatasetVersion(ctx, dataset.ID, DatasetVersionSpec{
		Version:    fmt.Sprintf("promoted-%s", timeutil.NowTime().UTC().Format("20060102-150405")),
		SourceType: "group_promotion",
		SourceRef:  group.ID,
		Manifest:   manifestRaw,
		Metadata: map[string]interface{}{
			"promoted_from_group_id": group.ID,
		},
		CreatedBy: group.OwnerUserID,
	})
	if err != nil {
		return nil, err
	}

	evalSpec, err := c.CreateEvalSpec(ctx, EvalSpecSpec{
		Name:             evalName,
		OwnerUserID:      group.OwnerUserID,
		Subject:          subject,
		RunKind:          firstRunKind(manifest.Defaults.RunKind, defaultRunKind),
		Profile:          firstNonEmpty(manifest.Defaults.Profile, defaultProfile),
		DatasetID:        dataset.ID,
		DatasetVersionID: version.ID,
		SchedulerConfig:  group.SchedulerConfig,
		ScoringConfig:    group.ScoringConfig,
		Metadata: map[string]interface{}{
			"promoted_from_group_id": group.ID,
		},
	})
	if err != nil {
		return nil, err
	}

	return &GroupPromotionResult{
		Dataset:        dataset,
		DatasetVersion: version,
		EvalSpec:       evalSpec,
	}, nil
}

func buildPromotionManifest(group *RunGroup, items []RunGroupItem, datasetName string, subject string) (DatasetManifest, error) {
	manifest := DatasetManifest{
		Dataset: DatasetManifestMeta{
			Name:    strings.TrimSpace(datasetName),
			Subject: strings.TrimSpace(subject),
		},
		Defaults: DatasetManifestDefaults{
			Scheduler: group.SchedulerConfig,
			Scoring:   group.ScoringConfig,
		},
		Items: make([]DatasetManifestItem, 0, len(items)),
	}
	if len(items) == 0 {
		return manifest, nil
	}

	firstRunKind := items[0].RunKind
	sameRunKind := firstRunKind != ""
	firstProfile := strings.TrimSpace(items[0].Profile)
	sameProfile := true
	for _, item := range items[1:] {
		if item.RunKind != firstRunKind {
			sameRunKind = false
		}
		if strings.TrimSpace(item.Profile) != firstProfile {
			sameProfile = false
		}
	}
	if sameRunKind {
		manifest.Defaults.RunKind = firstRunKind
	}
	if sameProfile && firstProfile != "" {
		manifest.Defaults.Profile = firstProfile
	}

	for _, item := range items {
		caseID := metadataString(item.Metadata, "dataset_case_id")
		if caseID == "" {
			caseID = fmt.Sprintf("group-%s-item-%d", group.ID, item.Index)
		}
		manifestItem := DatasetManifestItem{
			ID:       caseID,
			Input:    cloneMetadataMap(item.Input),
			Expected: cloneMetadataMap(item.Expected),
			Metadata: cloneMetadataMap(item.Metadata),
		}
		if !sameRunKind {
			manifestItem.RunKind = item.RunKind
		}
		if !sameProfile && strings.TrimSpace(item.Profile) != "" {
			manifestItem.Profile = strings.TrimSpace(item.Profile)
		}
		manifest.Items = append(manifest.Items, manifestItem)
	}
	return manifest, nil
}

func datasetManifestMap(manifest DatasetManifest) (map[string]interface{}, error) {
	raw, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}
