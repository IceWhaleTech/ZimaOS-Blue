package harness

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

func (c *Controller) CreateDataset(ctx context.Context, spec DatasetSpec) (*Dataset, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	if strings.TrimSpace(spec.Name) == "" {
		return nil, fmt.Errorf("name is required")
	}
	now := timeutil.NowTime()
	dataset := &Dataset{
		ID:             uuid.NewString(),
		Name:           strings.TrimSpace(spec.Name),
		Description:    strings.TrimSpace(spec.Description),
		OwnerUserID:    strings.TrimSpace(spec.OwnerUserID),
		Subject:        strings.TrimSpace(spec.Subject),
		DefaultRunKind: spec.DefaultRunKind,
		DefaultProfile: strings.TrimSpace(spec.DefaultProfile),
		Metadata:       cloneMetadataMap(spec.Metadata),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := c.store.CreateDataset(ctx, dataset); err != nil {
		return nil, err
	}
	return c.store.GetDataset(ctx, dataset.ID)
}

func (c *Controller) GetDataset(ctx context.Context, id string) (*Dataset, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	return c.store.GetDataset(ctx, strings.TrimSpace(id))
}

func (c *Controller) ListDatasets(ctx context.Context, filter DatasetFilter) ([]Dataset, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	return c.store.ListDatasets(ctx, filter)
}

func (c *Controller) CreateDatasetVersion(ctx context.Context, datasetID string, spec DatasetVersionSpec) (*DatasetVersion, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	dataset, err := c.store.GetDataset(ctx, strings.TrimSpace(datasetID))
	if err != nil {
		return nil, err
	}
	itemCount, err := manifestItemCount(spec.Manifest)
	if err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}
	now := timeutil.NowTime()
	version := &DatasetVersion{
		ID:             uuid.NewString(),
		DatasetID:      dataset.ID,
		Version:        normalizeDatasetVersion(spec.Version),
		ManifestSHA256: manifestSHA256(spec.Manifest),
		ItemCount:      itemCount,
		SourceType:     strings.TrimSpace(spec.SourceType),
		SourceRef:      strings.TrimSpace(spec.SourceRef),
		Manifest:       cloneMetadataMap(spec.Manifest),
		Metadata:       cloneMetadataMap(spec.Metadata),
		CreatedBy:      strings.TrimSpace(spec.CreatedBy),
		CreatedAt:      now,
	}
	if err := c.store.CreateDatasetVersion(ctx, version); err != nil {
		return nil, err
	}
	dataset.ActiveVersionID = version.ID
	if err := c.store.UpdateDataset(ctx, dataset); err != nil {
		return nil, err
	}
	return c.store.GetDatasetVersion(ctx, version.ID)
}

func (c *Controller) GetDatasetVersion(ctx context.Context, id string) (*DatasetVersion, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	return c.store.GetDatasetVersion(ctx, strings.TrimSpace(id))
}

func (c *Controller) ListDatasetVersions(ctx context.Context, datasetID string, limit int) ([]DatasetVersion, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	return c.store.ListDatasetVersions(ctx, strings.TrimSpace(datasetID), limit)
}

func (c *Controller) CreateEvalSpec(ctx context.Context, spec EvalSpecSpec) (*EvalSpec, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	if strings.TrimSpace(spec.Name) == "" {
		return nil, fmt.Errorf("name is required")
	}
	dataset, err := c.store.GetDataset(ctx, strings.TrimSpace(spec.DatasetID))
	if err != nil {
		return nil, err
	}
	versionID := firstNonEmpty(spec.DatasetVersionID, dataset.ActiveVersionID)
	if versionID == "" {
		return nil, fmt.Errorf("dataset version is required")
	}
	version, err := c.store.GetDatasetVersion(ctx, versionID)
	if err != nil {
		return nil, err
	}
	if version.DatasetID != dataset.ID {
		return nil, fmt.Errorf("dataset version does not belong to dataset")
	}
	runKind := spec.RunKind
	if runKind == "" {
		runKind = dataset.DefaultRunKind
	}
	if runKind == "" {
		return nil, fmt.Errorf("run kind is required")
	}
	now := timeutil.NowTime()
	evalSpec := &EvalSpec{
		ID:               uuid.NewString(),
		Name:             strings.TrimSpace(spec.Name),
		OwnerUserID:      firstNonEmpty(spec.OwnerUserID, dataset.OwnerUserID),
		Subject:          firstNonEmpty(spec.Subject, dataset.Subject),
		RunKind:          runKind,
		Profile:          firstNonEmpty(spec.Profile, dataset.DefaultProfile),
		DatasetID:        dataset.ID,
		DatasetVersionID: version.ID,
		SchedulerConfig:  spec.SchedulerConfig,
		ScoringConfig:    spec.ScoringConfig,
		RuntimePolicy:    cloneMetadataMap(spec.RuntimePolicy),
		Metadata:         cloneMetadataMap(spec.Metadata),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := c.store.CreateEvalSpec(ctx, evalSpec); err != nil {
		return nil, err
	}
	return c.store.GetEvalSpec(ctx, evalSpec.ID)
}

func (c *Controller) GetEvalSpec(ctx context.Context, id string) (*EvalSpec, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	return c.store.GetEvalSpec(ctx, strings.TrimSpace(id))
}

func (c *Controller) ListEvalSpecs(ctx context.Context, filter EvalSpecFilter) ([]EvalSpec, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	return c.store.ListEvalSpecs(ctx, filter)
}

func (c *Controller) SubmitEvalRun(ctx context.Context, spec EvalRunSpec) (*EvalRun, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	if strings.TrimSpace(spec.EvalSpecID) == "" {
		return nil, fmt.Errorf("eval_spec_id is required")
	}
	evalSpec, err := c.store.GetEvalSpec(ctx, strings.TrimSpace(spec.EvalSpecID))
	if err != nil {
		return nil, err
	}
	dataset, err := c.store.GetDataset(ctx, evalSpec.DatasetID)
	if err != nil {
		return nil, err
	}
	version, err := c.store.GetDatasetVersion(ctx, evalSpec.DatasetVersionID)
	if err != nil {
		return nil, err
	}
	groupSpec, err := materializeEvalGroupSpec(evalSpec, dataset, version, spec)
	if err != nil {
		return nil, err
	}
	group, err := c.SubmitGroup(ctx, groupSpec)
	if err != nil {
		return nil, err
	}
	now := timeutil.NowTime()
	evalRun := &EvalRun{
		ID:                uuid.NewString(),
		EvalSpecID:        evalSpec.ID,
		GroupID:           group.ID,
		DatasetVersionID:  version.ID,
		BaselineEvalRunID: strings.TrimSpace(spec.BaselineEvalRunID),
		Title:             firstNonEmpty(spec.Title, group.Title),
		OwnerUserID:       firstNonEmpty(spec.OwnerUserID, evalSpec.OwnerUserID, dataset.OwnerUserID),
		Status:            group.Status,
		TriggerKind:       strings.TrimSpace(spec.TriggerKind),
		TriggerRef:        strings.TrimSpace(spec.TriggerRef),
		Metadata:          cloneMetadataMap(spec.Metadata),
		Summary:           cloneMetadataMap(group.Summary),
		CreatedAt:         now,
		UpdatedAt:         now,
		StartedAt:         group.StartedAt,
		FinishedAt:        group.FinishedAt,
	}
	if err := c.store.CreateEvalRun(ctx, evalRun); err != nil {
		return nil, err
	}
	return c.GetEvalRun(ctx, evalRun.ID)
}

func (c *Controller) GetEvalRun(ctx context.Context, id string) (*EvalRun, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	evalRun, err := c.store.GetEvalRun(ctx, strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	return c.syncEvalRun(ctx, evalRun)
}

func (c *Controller) ListEvalRuns(ctx context.Context, filter EvalRunFilter) ([]EvalRun, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	evalRuns, err := c.store.ListEvalRuns(ctx, filter)
	if err != nil {
		return nil, err
	}
	for i := range evalRuns {
		synced, syncErr := c.syncEvalRun(ctx, &evalRuns[i])
		if syncErr != nil || synced == nil {
			continue
		}
		evalRuns[i] = *synced
	}
	return evalRuns, nil
}

func (c *Controller) CancelEvalRun(ctx context.Context, id string, reason string) error {
	evalRun, err := c.GetEvalRun(ctx, id)
	if err != nil {
		return err
	}
	if err := c.CancelGroup(ctx, evalRun.GroupID, reason); err != nil {
		return err
	}
	_, err = c.syncEvalRun(ctx, evalRun)
	return err
}

func (c *Controller) GetEvalRunReport(ctx context.Context, id string) (*EvalRunReport, error) {
	evalRun, err := c.GetEvalRun(ctx, id)
	if err != nil {
		return nil, err
	}
	evalSpec, err := c.store.GetEvalSpec(ctx, evalRun.EvalSpecID)
	if err != nil {
		return nil, err
	}
	dataset, err := c.store.GetDataset(ctx, evalSpec.DatasetID)
	if err != nil {
		return nil, err
	}
	version, err := c.store.GetDatasetVersion(ctx, evalRun.DatasetVersionID)
	if err != nil {
		return nil, err
	}
	groupReport, err := c.GetGroupReport(ctx, evalRun.GroupID)
	if err != nil {
		return nil, err
	}
	return &EvalRunReport{
		EvalRun:        evalRun,
		EvalSpec:       evalSpec,
		Dataset:        dataset,
		DatasetVersion: version,
		GroupReport:    groupReport,
	}, nil
}

func (c *Controller) syncEvalRun(ctx context.Context, evalRun *EvalRun) (*EvalRun, error) {
	if evalRun == nil || strings.TrimSpace(evalRun.GroupID) == "" {
		return evalRun, nil
	}
	group, err := c.store.GetGroup(ctx, evalRun.GroupID)
	if err != nil {
		return evalRun, nil
	}
	changed := false
	if evalRun.Status != group.Status {
		evalRun.Status = group.Status
		changed = true
	}
	if !metadataMapsEqual(evalRun.Summary, group.Summary) {
		evalRun.Summary = cloneMetadataMap(group.Summary)
		changed = true
	}
	if !timesEqual(evalRun.StartedAt, group.StartedAt) {
		evalRun.StartedAt = cloneTimePtr(group.StartedAt)
		changed = true
	}
	if !timesEqual(evalRun.FinishedAt, group.FinishedAt) {
		evalRun.FinishedAt = cloneTimePtr(group.FinishedAt)
		changed = true
	}
	if changed {
		if err := c.store.UpdateEvalRun(ctx, evalRun); err != nil {
			return nil, err
		}
	}
	return evalRun, nil
}

func materializeEvalGroupSpec(evalSpec *EvalSpec, dataset *Dataset, version *DatasetVersion, runSpec EvalRunSpec) (RunGroupSpec, error) {
	if evalSpec == nil || dataset == nil || version == nil {
		return RunGroupSpec{}, fmt.Errorf("eval spec, dataset, and dataset version are required")
	}
	manifest, err := version.DecodeManifest()
	if err != nil {
		return RunGroupSpec{}, fmt.Errorf("decode manifest: %w", err)
	}
	groupMetadata := mergeMetadataMaps(evalSpec.RuntimePolicy, evalSpec.Metadata)
	groupMetadata = mergeMetadataMaps(groupMetadata, runSpec.Metadata)
	if groupMetadata == nil {
		groupMetadata = map[string]interface{}{}
	}
	groupMetadata["eval_spec_id"] = evalSpec.ID
	groupMetadata["dataset_id"] = dataset.ID
	groupMetadata["dataset_version_id"] = version.ID
	groupMetadata["dataset_version"] = version.Version

	items := make([]RunGroupItemSpec, 0, len(manifest.Items))
	for _, item := range manifest.Items {
		runKind := firstRunKind(item.RunKind, manifest.Defaults.RunKind, evalSpec.RunKind, dataset.DefaultRunKind)
		if runKind == "" {
			return RunGroupSpec{}, fmt.Errorf("dataset item %q does not define a run kind", strings.TrimSpace(item.ID))
		}
		metadata := mergeMetadataMaps(manifest.Defaults.RuntimePolicy, item.Metadata)
		if metadata == nil {
			metadata = map[string]interface{}{}
		}
		if caseID := strings.TrimSpace(item.ID); caseID != "" {
			metadata["dataset_case_id"] = caseID
		}
		items = append(items, RunGroupItemSpec{
			RunKind:  runKind,
			Profile:  firstNonEmpty(item.Profile, manifest.Defaults.Profile, evalSpec.Profile, dataset.DefaultProfile),
			Input:    cloneMetadataMap(item.Input),
			Expected: cloneMetadataMap(item.Expected),
			Metadata: metadata,
		})
	}

	return RunGroupSpec{
		Kind:        RunGroupKindEval,
		Title:       firstNonEmpty(runSpec.Title, evalSpec.Name, dataset.Name, version.Version),
		Subject:     firstNonEmpty(evalSpec.Subject, dataset.Subject, manifest.Dataset.Subject),
		OwnerUserID: firstNonEmpty(runSpec.OwnerUserID, evalSpec.OwnerUserID, dataset.OwnerUserID),
		Metadata:    groupMetadata,
		SchedulerConfig: mergeSchedulerConfig(
			manifest.Defaults.Scheduler,
			evalSpec.SchedulerConfig,
		),
		ScoringConfig: mergeScoringConfig(
			manifest.Defaults.Scoring,
			evalSpec.ScoringConfig,
		),
		Items: items,
	}, nil
}

func mergeSchedulerConfig(base GroupSchedulerConfig, override GroupSchedulerConfig) GroupSchedulerConfig {
	out := base
	if override.MaxConcurrency > 0 {
		out.MaxConcurrency = override.MaxConcurrency
	}
	if override.MaxAttempts > 0 {
		out.MaxAttempts = override.MaxAttempts
	}
	if override.LeaseTTL > 0 {
		out.LeaseTTL = override.LeaseTTL
	}
	if override.RetryBackoff > 0 {
		out.RetryBackoff = override.RetryBackoff
	}
	return out
}

func mergeScoringConfig(base GroupScoringConfig, override GroupScoringConfig) GroupScoringConfig {
	out := base
	if override.Mode != "" {
		out.Mode = override.Mode
	}
	if strings.TrimSpace(override.RuleProfile) != "" {
		out.RuleProfile = override.RuleProfile
	}
	if strings.TrimSpace(override.JudgeModel) != "" {
		out.JudgeModel = override.JudgeModel
	}
	if override.PassThreshold > 0 {
		out.PassThreshold = override.PassThreshold
	}
	return out
}

func firstRunKind(values ...RunKind) RunKind {
	for _, value := range values {
		if strings.TrimSpace(string(value)) != "" {
			return value
		}
	}
	return ""
}

func metadataMapsEqual(left map[string]interface{}, right map[string]interface{}) bool {
	return marshalMetadata(left) == marshalMetadata(right)
}

func timesEqual(left *time.Time, right *time.Time) bool {
	switch {
	case left == nil && right == nil:
		return true
	case left == nil || right == nil:
		return false
	default:
		return left.Equal(*right)
	}
}

func cloneTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
