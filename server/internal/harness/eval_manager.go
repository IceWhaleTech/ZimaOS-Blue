package harness

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

type evalRunReportContext struct {
	evalRun        *EvalRun
	group          *RunGroup
	evalSpec       *EvalSpec
	dataset        *Dataset
	datasetVersion *DatasetVersion
	groupReport    *groupReportContext
}

type evalRunGroupSnapshot struct {
	evalRun *EvalRun
	group   *RunGroup
}

type evalRunSpecContext struct {
	snapshot *evalRunGroupSnapshot
	evalSpec *EvalSpec
}

type evalRunDatasetContext struct {
	specContext    *evalRunSpecContext
	dataset        *Dataset
	datasetVersion *DatasetVersion
}

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

func (c *Controller) ImportDatasetBundle(ctx context.Context, req ImportDatasetBundleRequest) (*ImportDatasetBundleResult, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	if _, err := validateImportDatasetBundleRequest(&req); err != nil {
		return nil, err
	}

	var (
		datasetID   string
		versionID   string
		evalSpecIDs []string
	)
	err := c.store.withTx(ctx, func(tx *sql.Tx) error {
		dataset, err := c.upsertImportedDataset(ctx, tx, req.Dataset)
		if err != nil {
			return err
		}
		version, err := c.upsertImportedDatasetVersion(ctx, tx, dataset, req)
		if err != nil {
			return err
		}
		if req.MakeActive && dataset.ActiveVersionID != version.ID {
			dataset.ActiveVersionID = version.ID
			if err := c.store.updateDatasetTx(ctx, tx, dataset); err != nil {
				return err
			}
		}
		datasetID = dataset.ID
		versionID = version.ID
		evalSpecIDs = evalSpecIDs[:0]
		for _, spec := range req.EvalSpecs {
			evalSpec, err := c.upsertImportedEvalSpec(ctx, tx, dataset, version, spec)
			if err != nil {
				return err
			}
			evalSpecIDs = append(evalSpecIDs, evalSpec.ID)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	dataset, err := c.store.GetDataset(ctx, datasetID)
	if err != nil {
		return nil, err
	}
	version, err := c.store.GetDatasetVersion(ctx, versionID)
	if err != nil {
		return nil, err
	}
	result := &ImportDatasetBundleResult{
		Dataset:        dataset,
		DatasetVersion: version,
		EvalSpecs:      make([]EvalSpec, 0, len(evalSpecIDs)),
	}
	for _, evalSpecID := range evalSpecIDs {
		evalSpec, err := c.store.GetEvalSpec(ctx, evalSpecID)
		if err != nil {
			return nil, err
		}
		result.EvalSpecs = append(result.EvalSpecs, *evalSpec)
	}
	return result, nil
}

func validateImportDatasetBundleRequest(req *ImportDatasetBundleRequest) (int, error) {
	if req == nil {
		return 0, fmt.Errorf("dataset bundle request is required")
	}
	if strings.TrimSpace(req.Dataset.Name) == "" {
		return 0, fmt.Errorf("dataset name is required")
	}
	if len(req.Version.Manifest) == 0 {
		return 0, fmt.Errorf("dataset version manifest is required")
	}
	itemCount, err := manifestItemCount(req.Version.Manifest)
	if err != nil {
		return 0, fmt.Errorf("invalid manifest: %w", err)
	}
	for _, spec := range req.EvalSpecs {
		name := strings.TrimSpace(spec.Name)
		if name == "" {
			return 0, fmt.Errorf("eval spec name is required")
		}
		runKind := spec.RunKind
		if runKind == "" {
			runKind = req.Dataset.DefaultRunKind
		}
		if runKind == "" {
			return 0, fmt.Errorf("eval spec %q run kind is required", name)
		}
	}
	return itemCount, nil
}

func (c *Controller) GetEvalSpec(ctx context.Context, id string) (*EvalSpec, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	return c.store.GetEvalSpec(ctx, strings.TrimSpace(id))
}

func (c *Controller) upsertImportedDataset(ctx context.Context, tx *sql.Tx, spec DatasetSpec) (*Dataset, error) {
	ownerUserID := strings.TrimSpace(spec.OwnerUserID)
	name := strings.TrimSpace(spec.Name)
	existing, err := c.store.findDatasetByOwnerAndNameWithDB(ctx, tx, ownerUserID, name)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if existing == nil {
		now := timeutil.NowTime()
		dataset := &Dataset{
			ID:             uuid.NewString(),
			Name:           name,
			Description:    strings.TrimSpace(spec.Description),
			OwnerUserID:    ownerUserID,
			Subject:        strings.TrimSpace(spec.Subject),
			DefaultRunKind: spec.DefaultRunKind,
			DefaultProfile: strings.TrimSpace(spec.DefaultProfile),
			Metadata:       cloneMetadataMap(spec.Metadata),
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if err := c.store.createDatasetTx(ctx, tx, dataset); err != nil {
			return nil, err
		}
		return dataset, nil
	}
	next := *existing
	next.Description = strings.TrimSpace(spec.Description)
	next.Subject = strings.TrimSpace(spec.Subject)
	if strings.TrimSpace(spec.OwnerUserID) != "" {
		next.OwnerUserID = strings.TrimSpace(spec.OwnerUserID)
	}
	if spec.DefaultRunKind != "" {
		next.DefaultRunKind = spec.DefaultRunKind
	}
	next.DefaultProfile = strings.TrimSpace(spec.DefaultProfile)
	next.Metadata = cloneMetadataMap(spec.Metadata)
	if err := c.store.updateDatasetTx(ctx, tx, &next); err != nil {
		return nil, err
	}
	return &next, nil
}

func (c *Controller) upsertImportedDatasetVersion(ctx context.Context, tx *sql.Tx, dataset *Dataset, req ImportDatasetBundleRequest) (*DatasetVersion, error) {
	if dataset == nil {
		return nil, fmt.Errorf("dataset is required")
	}
	versionSpec := req.Version
	versionSpec.SourceType = firstNonEmpty(strings.TrimSpace(req.SourceType), strings.TrimSpace(versionSpec.SourceType))
	versionSpec.SourceRef = firstNonEmpty(strings.TrimSpace(req.SourceRef), strings.TrimSpace(versionSpec.SourceRef))
	versionSpec.CreatedBy = firstNonEmpty(strings.TrimSpace(versionSpec.CreatedBy), strings.TrimSpace(dataset.OwnerUserID))
	versionSpec.Version = normalizeDatasetVersion(versionSpec.Version)

	existing, err := c.store.findDatasetVersionByDatasetAndVersionWithDB(ctx, tx, dataset.ID, versionSpec.Version)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	nextHash := manifestSHA256(versionSpec.Manifest)
	if existing != nil {
		if strings.TrimSpace(existing.ManifestSHA256) != strings.TrimSpace(nextHash) {
			return nil, fmt.Errorf("dataset version %s already exists with a different manifest hash; use a version bump", versionSpec.Version)
		}
		return existing, nil
	}

	itemCount, err := manifestItemCount(versionSpec.Manifest)
	if err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}
	version := &DatasetVersion{
		ID:             uuid.NewString(),
		DatasetID:      dataset.ID,
		Version:        versionSpec.Version,
		ManifestSHA256: nextHash,
		ItemCount:      itemCount,
		SourceType:     strings.TrimSpace(versionSpec.SourceType),
		SourceRef:      strings.TrimSpace(versionSpec.SourceRef),
		Manifest:       cloneMetadataMap(versionSpec.Manifest),
		Metadata:       cloneMetadataMap(versionSpec.Metadata),
		CreatedBy:      strings.TrimSpace(versionSpec.CreatedBy),
		CreatedAt:      timeutil.NowTime(),
	}
	if err := c.store.createDatasetVersionTx(ctx, tx, version); err != nil {
		return nil, err
	}
	return version, nil
}

func (c *Controller) upsertImportedEvalSpec(ctx context.Context, tx *sql.Tx, dataset *Dataset, version *DatasetVersion, spec ImportDatasetBundleEvalSpec) (*EvalSpec, error) {
	if dataset == nil || version == nil {
		return nil, fmt.Errorf("dataset and dataset version are required")
	}
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		return nil, fmt.Errorf("eval spec name is required")
	}
	ownerUserID := firstNonEmpty(strings.TrimSpace(spec.OwnerUserID), strings.TrimSpace(dataset.OwnerUserID))
	runKind := spec.RunKind
	if runKind == "" {
		runKind = dataset.DefaultRunKind
	}
	if runKind == "" {
		return nil, fmt.Errorf("run kind is required")
	}
	subject := firstNonEmpty(strings.TrimSpace(spec.Subject), strings.TrimSpace(dataset.Subject))
	profile := firstNonEmpty(strings.TrimSpace(spec.Profile), strings.TrimSpace(dataset.DefaultProfile))

	existing, err := c.store.findEvalSpecByOwnerDatasetAndNameWithDB(ctx, tx, ownerUserID, dataset.ID, name)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if existing == nil {
		now := timeutil.NowTime()
		evalSpec := &EvalSpec{
			ID:               uuid.NewString(),
			Name:             name,
			OwnerUserID:      ownerUserID,
			Subject:          subject,
			RunKind:          runKind,
			Profile:          profile,
			DatasetID:        dataset.ID,
			DatasetVersionID: version.ID,
			SchedulerConfig:  spec.SchedulerConfig,
			ScoringConfig:    spec.ScoringConfig,
			RuntimePolicy:    cloneMetadataMap(spec.RuntimePolicy),
			Metadata:         cloneMetadataMap(spec.Metadata),
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		if err := c.store.createEvalSpecTx(ctx, tx, evalSpec); err != nil {
			return nil, err
		}
		return evalSpec, nil
	}

	next := *existing
	next.Name = name
	next.OwnerUserID = ownerUserID
	next.Subject = subject
	next.RunKind = runKind
	next.Profile = profile
	next.DatasetID = dataset.ID
	next.DatasetVersionID = version.ID
	next.SchedulerConfig = spec.SchedulerConfig
	next.ScoringConfig = spec.ScoringConfig
	next.RuntimePolicy = cloneMetadataMap(spec.RuntimePolicy)
	next.Metadata = cloneMetadataMap(spec.Metadata)
	if err := c.store.updateEvalSpecTx(ctx, tx, &next); err != nil {
		return nil, err
	}
	return &next, nil
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
	now := monotonicHarnessTime()
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

func (c *Controller) GetComparisonReport(ctx context.Context, id string) (*ComparisonReport, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	return c.store.GetComparisonReport(ctx, strings.TrimSpace(id))
}

func (c *Controller) CancelEvalRun(ctx context.Context, id string, reason string) error {
	if c == nil || c.store == nil {
		return fmt.Errorf("harness controller is not configured")
	}
	evalRun, err := c.store.GetEvalRun(ctx, strings.TrimSpace(id))
	if err != nil {
		return err
	}
	snapshot, err := c.loadEvalRunGroupSnapshot(ctx, evalRun)
	if err != nil {
		return err
	}
	if err := c.CancelGroup(ctx, snapshot.group.ID, reason); err != nil {
		return err
	}
	_, err = c.loadEvalRunGroupSnapshot(ctx, snapshot.evalRun)
	return err
}

func (c *Controller) CreateBaseline(ctx context.Context, spec BaselineSpec) (*Baseline, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	if strings.TrimSpace(spec.Name) == "" {
		return nil, fmt.Errorf("name is required")
	}
	evalRun, err := c.store.GetEvalRun(ctx, strings.TrimSpace(spec.EvalRunID))
	if err != nil {
		return nil, err
	}
	specContext, err := c.loadEvalRunSpecContext(ctx, evalRun)
	if err != nil {
		return nil, err
	}
	evalRun = specContext.snapshot.evalRun
	evalSpec := specContext.evalSpec
	if spec.EvalSpecID != "" && strings.TrimSpace(spec.EvalSpecID) != evalSpec.ID {
		return nil, fmt.Errorf("eval run does not belong to eval spec")
	}
	now := timeutil.NowTime()
	baseline := &Baseline{
		ID:          uuid.NewString(),
		Name:        strings.TrimSpace(spec.Name),
		Subject:     firstNonEmpty(spec.Subject, evalSpec.Subject),
		OwnerUserID: firstNonEmpty(spec.OwnerUserID, evalRun.OwnerUserID, evalSpec.OwnerUserID),
		EvalSpecID:  evalSpec.ID,
		EvalRunID:   evalRun.ID,
		IsDefault:   spec.IsDefault,
		Metadata:    cloneMetadataMap(spec.Metadata),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if baseline.IsDefault {
		if err := c.store.ClearDefaultBaseline(ctx, baseline.EvalSpecID); err != nil {
			return nil, err
		}
	}
	if err := c.store.CreateBaseline(ctx, baseline); err != nil {
		return nil, err
	}
	return c.store.GetBaseline(ctx, baseline.ID)
}

func (c *Controller) ListBaselines(ctx context.Context, filter BaselineFilter) ([]Baseline, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	return c.store.ListBaselines(ctx, filter)
}

func (c *Controller) GetEvalRunReport(ctx context.Context, id string) (*EvalRunReport, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	evalRun, err := c.store.GetEvalRun(ctx, strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	return c.buildEvalRunReport(ctx, evalRun)
}

func (c *Controller) buildEvalRunReport(ctx context.Context, evalRun *EvalRun) (*EvalRunReport, error) {
	reportCtx, err := c.loadEvalRunReportContext(ctx, evalRun)
	if err != nil {
		return nil, err
	}
	return c.buildEvalRunReportFromContext(reportCtx)
}

func (c *Controller) loadEvalRunReportContext(ctx context.Context, evalRun *EvalRun) (*evalRunReportContext, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	datasetContext, err := c.loadEvalRunDatasetContext(ctx, evalRun)
	if err != nil {
		return nil, err
	}
	groupReport, err := c.loadGroupReportContext(ctx, datasetContext.specContext.snapshot.group)
	if err != nil {
		return nil, err
	}
	return &evalRunReportContext{
		evalRun:        datasetContext.specContext.snapshot.evalRun,
		group:          datasetContext.specContext.snapshot.group,
		evalSpec:       datasetContext.specContext.evalSpec,
		dataset:        datasetContext.dataset,
		datasetVersion: datasetContext.datasetVersion,
		groupReport:    groupReport,
	}, nil
}

func (c *Controller) buildEvalRunReportFromContext(reportCtx *evalRunReportContext) (*EvalRunReport, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	if reportCtx == nil {
		return nil, fmt.Errorf("eval run report context is required")
	}
	groupReport, err := buildGroupReportFromContext(reportCtx.groupReport)
	if err != nil {
		return nil, err
	}
	return &EvalRunReport{
		EvalRun:        reportCtx.evalRun,
		EvalSpec:       reportCtx.evalSpec,
		Dataset:        reportCtx.dataset,
		DatasetVersion: reportCtx.datasetVersion,
		GroupReport:    groupReport,
	}, nil
}

func resolveEvalRunDatasetVersionID(evalRun *EvalRun, evalSpec *EvalSpec) string {
	evalRunVersionID := ""
	if evalRun != nil {
		evalRunVersionID = strings.TrimSpace(evalRun.DatasetVersionID)
	}
	evalSpecVersionID := ""
	if evalSpec != nil {
		evalSpecVersionID = strings.TrimSpace(evalSpec.DatasetVersionID)
	}
	return firstNonEmpty(
		evalRunVersionID,
		evalSpecVersionID,
	)
}

func (c *Controller) syncEvalRun(ctx context.Context, evalRun *EvalRun) (*EvalRun, error) {
	if evalRun == nil || strings.TrimSpace(evalRun.GroupID) == "" {
		return evalRun, nil
	}
	snapshot, err := c.loadEvalRunGroupSnapshot(ctx, evalRun)
	if err != nil {
		return evalRun, nil
	}
	return snapshot.evalRun, nil
}

func (c *Controller) syncEvalRunWithGroup(ctx context.Context, evalRun *EvalRun, group *RunGroup) (*EvalRun, error) {
	if evalRun == nil || group == nil {
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

func (c *Controller) loadEvalRunGroupSnapshot(ctx context.Context, evalRun *EvalRun) (*evalRunGroupSnapshot, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	if evalRun == nil {
		return nil, fmt.Errorf("eval run is required")
	}
	group, err := c.GetGroup(ctx, evalRun.GroupID)
	if errors.Is(err, sql.ErrNoRows) {
		group, err = c.repairMissingEvalRunGroup(ctx, evalRun)
	}
	if err != nil {
		return nil, err
	}
	evalRun, err = c.syncEvalRunWithGroup(ctx, evalRun, group)
	if err != nil {
		return nil, err
	}
	return &evalRunGroupSnapshot{
		evalRun: evalRun,
		group:   group,
	}, nil
}

func (c *Controller) repairMissingEvalRunGroup(ctx context.Context, evalRun *EvalRun) (*RunGroup, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	if evalRun == nil || strings.TrimSpace(evalRun.GroupID) == "" {
		return nil, sql.ErrNoRows
	}

	items, err := c.store.ListGroupItems(ctx, evalRun.GroupID)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, sql.ErrNoRows
	}

	evalSpec, err := c.store.GetEvalSpec(ctx, evalRun.EvalSpecID)
	if err != nil {
		return nil, err
	}
	dataset, err := c.store.GetDataset(ctx, evalSpec.DatasetID)
	if err != nil {
		return nil, err
	}
	versionID := resolveEvalRunDatasetVersionID(evalRun, evalSpec)
	if versionID == "" {
		return nil, fmt.Errorf("dataset version is required")
	}
	version, err := c.store.GetDatasetVersion(ctx, versionID)
	if err != nil {
		return nil, err
	}

	groupSpec, err := materializeEvalGroupSpec(evalSpec, dataset, version, EvalRunSpec{
		EvalSpecID:        evalRun.EvalSpecID,
		OwnerUserID:       evalRun.OwnerUserID,
		Title:             evalRun.Title,
		BaselineEvalRunID: evalRun.BaselineEvalRunID,
		TriggerKind:       evalRun.TriggerKind,
		TriggerRef:        evalRun.TriggerRef,
		Metadata:          cloneMetadataMap(evalRun.Metadata),
	})
	if err != nil {
		return nil, err
	}

	group := &RunGroup{
		ID:              evalRun.GroupID,
		Kind:            groupSpec.Kind,
		Title:           groupSpec.Title,
		Status:          evalRun.Status,
		OwnerUserID:     groupSpec.OwnerUserID,
		Subject:         groupSpec.Subject,
		SchedulerConfig: groupSpec.SchedulerConfig,
		ScoringConfig:   groupSpec.ScoringConfig,
		Metadata:        cloneMetadataMap(groupSpec.Metadata),
		Summary:         cloneMetadataMap(evalRun.Summary),
		CreatedAt:       evalRun.CreatedAt,
		UpdatedAt:       evalRun.UpdatedAt,
		StartedAt:       cloneTimePtr(evalRun.StartedAt),
		FinishedAt:      cloneTimePtr(evalRun.FinishedAt),
	}
	if group.Metadata == nil {
		group.Metadata = map[string]interface{}{}
	}
	group.Metadata["repaired_from_eval_run"] = true

	if err := c.store.CreateGroup(ctx, group); err != nil {
		if existing, getErr := c.store.GetGroup(ctx, group.ID); getErr == nil && existing != nil {
			return c.refreshLoadedGroupSummary(ctx, existing)
		}
		return nil, err
	}

	return c.refreshLoadedGroupSummary(ctx, group)
}

func (c *Controller) loadEvalRunSpecContext(ctx context.Context, evalRun *EvalRun) (*evalRunSpecContext, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	snapshot, err := c.loadEvalRunGroupSnapshot(ctx, evalRun)
	if err != nil {
		return nil, err
	}
	evalSpec, err := c.store.GetEvalSpec(ctx, snapshot.evalRun.EvalSpecID)
	if err != nil {
		return nil, err
	}
	return &evalRunSpecContext{
		snapshot: snapshot,
		evalSpec: evalSpec,
	}, nil
}

func (c *Controller) loadEvalRunDatasetContext(ctx context.Context, evalRun *EvalRun) (*evalRunDatasetContext, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	specContext, err := c.loadEvalRunSpecContext(ctx, evalRun)
	if err != nil {
		return nil, err
	}
	dataset, err := c.store.GetDataset(ctx, specContext.evalSpec.DatasetID)
	if err != nil {
		return nil, err
	}
	versionID := resolveEvalRunDatasetVersionID(specContext.snapshot.evalRun, specContext.evalSpec)
	if versionID == "" {
		return nil, fmt.Errorf("dataset version is required")
	}
	version, err := c.store.GetDatasetVersion(ctx, versionID)
	if err != nil {
		return nil, err
	}
	return &evalRunDatasetContext{
		specContext:    specContext,
		dataset:        dataset,
		datasetVersion: version,
	}, nil
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
	groupContract := DecodeHarnessContract(evalSpec.RuntimePolicy, manifest.Defaults.RuntimePolicy, runSpec.Metadata)
	if contractMeta := HarnessContractMetadata(groupContract); len(contractMeta) > 0 {
		groupMetadata["harness_contract"] = contractMeta
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
		itemContract := DecodeHarnessContract(HarnessContractMetadata(groupContract), item.Input, item.Expected, item.Metadata)
		if contractMeta := HarnessContractMetadata(itemContract); len(contractMeta) > 0 {
			metadata["harness_contract"] = contractMeta
		}
		if criteria := HarnessContractSuccessCriteria(itemContract); len(criteria) > 0 {
			metadata["task_success_criteria"] = append([]string(nil), criteria...)
		}
		if fallback := HarnessContractFallbackPlan(itemContract); len(fallback) > 0 {
			metadata["task_fallback_plan"] = append([]string(nil), fallback...)
		}
		if caseID := strings.TrimSpace(item.ID); caseID != "" {
			metadata["dataset_case_id"] = caseID
		}
		items = append(items, RunGroupItemSpec{
			RunKind:  runKind,
			Profile:  firstNonEmpty(item.Profile, manifest.Defaults.Profile, evalSpec.Profile, dataset.DefaultProfile),
			Input:    cloneMetadataMap(item.Input),
			Expected: ApplyHarnessContractToExpected(item.Expected, itemContract),
			Metadata: metadata,
		})
	}

	schedulerConfig := mergeSchedulerConfig(
		manifest.Defaults.Scheduler,
		evalSpec.SchedulerConfig,
	)
	schedulerConfig = applyRuntimeSchedulerConcurrencyOverride(
		firstNonEmpty(evalSpec.Subject, dataset.Subject, manifest.Dataset.Subject),
		schedulerConfig,
	)

	return RunGroupSpec{
		Kind:            RunGroupKindEval,
		Title:           firstNonEmpty(runSpec.Title, evalSpec.Name, dataset.Name, version.Version),
		Subject:         firstNonEmpty(evalSpec.Subject, dataset.Subject, manifest.Dataset.Subject),
		OwnerUserID:     firstNonEmpty(runSpec.OwnerUserID, evalSpec.OwnerUserID, dataset.OwnerUserID),
		Metadata:        groupMetadata,
		SchedulerConfig: schedulerConfig,
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
