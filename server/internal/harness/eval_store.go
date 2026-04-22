package harness

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	z "github.com/IceWhaleTech/zorm"
)

func monotonicHarnessTime() time.Time {
	return time.Unix(0, timeutil.Monotonic())
}

type datasetRow struct {
	ID              string    `zorm:"id"`
	Name            string    `zorm:"name"`
	Description     string    `zorm:"description"`
	OwnerUserID     string    `zorm:"owner_user_id"`
	Subject         string    `zorm:"subject"`
	DefaultRunKind  string    `zorm:"default_run_kind"`
	DefaultProfile  string    `zorm:"default_profile"`
	ActiveVersionID string    `zorm:"active_version_id"`
	MetadataJSON    string    `zorm:"metadata_json"`
	CreatedAt       time.Time `zorm:"created_at"`
	UpdatedAt       time.Time `zorm:"updated_at"`
}

type datasetVersionRow struct {
	ID             string    `zorm:"id"`
	DatasetID      string    `zorm:"dataset_id"`
	Version        string    `zorm:"version"`
	ManifestSHA256 string    `zorm:"manifest_sha256"`
	ItemCount      int       `zorm:"item_count"`
	SourceType     string    `zorm:"source_type"`
	SourceRef      string    `zorm:"source_ref"`
	ManifestJSON   string    `zorm:"manifest_json"`
	MetadataJSON   string    `zorm:"metadata_json"`
	CreatedBy      string    `zorm:"created_by"`
	CreatedAt      time.Time `zorm:"created_at"`
}

type evalSpecRow struct {
	ID                string    `zorm:"id"`
	Name              string    `zorm:"name"`
	OwnerUserID       string    `zorm:"owner_user_id"`
	Subject           string    `zorm:"subject"`
	RunKind           string    `zorm:"run_kind"`
	Profile           string    `zorm:"profile"`
	DatasetID         string    `zorm:"dataset_id"`
	DatasetVersionID  string    `zorm:"dataset_version_id"`
	SchedulerJSON     string    `zorm:"scheduler_json"`
	ScoringJSON       string    `zorm:"scoring_json"`
	RuntimePolicyJSON string    `zorm:"runtime_policy_json"`
	MetadataJSON      string    `zorm:"metadata_json"`
	CreatedAt         time.Time `zorm:"created_at"`
	UpdatedAt         time.Time `zorm:"updated_at"`
}

type evalRunRow struct {
	ID                string       `zorm:"id"`
	EvalSpecID        string       `zorm:"eval_spec_id"`
	GroupID           string       `zorm:"group_id"`
	DatasetVersionID  string       `zorm:"dataset_version_id"`
	BaselineEvalRunID string       `zorm:"baseline_eval_run_id"`
	Title             string       `zorm:"title"`
	OwnerUserID       string       `zorm:"owner_user_id"`
	Status            string       `zorm:"status"`
	TriggerKind       string       `zorm:"trigger_kind"`
	TriggerRef        string       `zorm:"trigger_ref"`
	MetadataJSON      string       `zorm:"metadata_json"`
	SummaryJSON       string       `zorm:"summary_json"`
	CreatedAt         time.Time    `zorm:"created_at"`
	UpdatedAt         time.Time    `zorm:"updated_at"`
	StartedAt         nullableSQLiteTime `zorm:"started_at"`
	FinishedAt        nullableSQLiteTime `zorm:"finished_at"`
}

type baselineRow struct {
	ID           string    `zorm:"id"`
	Name         string    `zorm:"name"`
	Subject      string    `zorm:"subject"`
	OwnerUserID  string    `zorm:"owner_user_id"`
	EvalSpecID   string    `zorm:"eval_spec_id"`
	EvalRunID    string    `zorm:"eval_run_id"`
	IsDefault    int       `zorm:"is_default"`
	MetadataJSON string    `zorm:"metadata_json"`
	CreatedAt    time.Time `zorm:"created_at"`
	UpdatedAt    time.Time `zorm:"updated_at"`
}

type comparisonReportRow struct {
	ID               string    `zorm:"id"`
	OwnerUserID      string    `zorm:"owner_user_id"`
	BaselineID       string    `zorm:"baseline_id"`
	EvalSpecID       string    `zorm:"eval_spec_id"`
	BaseEvalRunID    string    `zorm:"base_eval_run_id"`
	TargetEvalRunID  string    `zorm:"target_eval_run_id"`
	SummaryJSON      string    `zorm:"summary_json"`
	RegressionsJSON  string    `zorm:"regressions_json"`
	ImprovementsJSON string    `zorm:"improvements_json"`
	ScorerDeltaJSON  string    `zorm:"scorer_delta_json"`
	CreatedAt        time.Time `zorm:"created_at"`
}

func datasetFromRow(row datasetRow) Dataset {
	return Dataset{
		ID:              row.ID,
		Name:            row.Name,
		Description:     row.Description,
		OwnerUserID:     row.OwnerUserID,
		Subject:         row.Subject,
		DefaultRunKind:  RunKind(row.DefaultRunKind),
		DefaultProfile:  row.DefaultProfile,
		ActiveVersionID: row.ActiveVersionID,
		Metadata:        unmarshalMetadata(row.MetadataJSON),
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}

func datasetVersionFromRow(row datasetVersionRow) DatasetVersion {
	return DatasetVersion{
		ID:             row.ID,
		DatasetID:      row.DatasetID,
		Version:        row.Version,
		ManifestSHA256: row.ManifestSHA256,
		ItemCount:      row.ItemCount,
		SourceType:     row.SourceType,
		SourceRef:      row.SourceRef,
		Manifest:       unmarshalMetadata(row.ManifestJSON),
		Metadata:       unmarshalMetadata(row.MetadataJSON),
		CreatedBy:      row.CreatedBy,
		CreatedAt:      row.CreatedAt,
	}
}

func evalSpecFromRow(row evalSpecRow) EvalSpec {
	spec := EvalSpec{
		ID:               row.ID,
		Name:             row.Name,
		OwnerUserID:      row.OwnerUserID,
		Subject:          row.Subject,
		RunKind:          RunKind(row.RunKind),
		Profile:          row.Profile,
		DatasetID:        row.DatasetID,
		DatasetVersionID: row.DatasetVersionID,
		RuntimePolicy:    unmarshalMetadata(row.RuntimePolicyJSON),
		Metadata:         unmarshalMetadata(row.MetadataJSON),
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}
	_ = unmarshalInto(row.SchedulerJSON, &spec.SchedulerConfig)
	_ = unmarshalInto(row.ScoringJSON, &spec.ScoringConfig)
	return spec
}

func evalRunFromRow(row evalRunRow) EvalRun {
	evalRun := EvalRun{
		ID:                row.ID,
		EvalSpecID:        row.EvalSpecID,
		GroupID:           row.GroupID,
		DatasetVersionID:  row.DatasetVersionID,
		BaselineEvalRunID: row.BaselineEvalRunID,
		Title:             row.Title,
		OwnerUserID:       row.OwnerUserID,
		Status:            RunGroupStatus(row.Status),
		TriggerKind:       row.TriggerKind,
		TriggerRef:        row.TriggerRef,
		Metadata:          unmarshalMetadata(row.MetadataJSON),
		Summary:           unmarshalMetadata(row.SummaryJSON),
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}
	if row.StartedAt.Valid {
		ts := row.StartedAt.Time
		evalRun.StartedAt = &ts
	}
	if row.FinishedAt.Valid {
		ts := row.FinishedAt.Time
		evalRun.FinishedAt = &ts
	}
	return evalRun
}

func baselineFromRow(row baselineRow) Baseline {
	return Baseline{
		ID:          row.ID,
		Name:        row.Name,
		Subject:     row.Subject,
		OwnerUserID: row.OwnerUserID,
		EvalSpecID:  row.EvalSpecID,
		EvalRunID:   row.EvalRunID,
		IsDefault:   row.IsDefault > 0,
		Metadata:    unmarshalMetadata(row.MetadataJSON),
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}

func comparisonReportFromRow(row comparisonReportRow) ComparisonReport {
	report := ComparisonReport{
		ID:              row.ID,
		OwnerUserID:     row.OwnerUserID,
		BaselineID:      row.BaselineID,
		EvalSpecID:      row.EvalSpecID,
		BaseEvalRunID:   row.BaseEvalRunID,
		TargetEvalRunID: row.TargetEvalRunID,
		Summary:         unmarshalMetadata(row.SummaryJSON),
		ScorerDelta:     unmarshalMetadata(row.ScorerDeltaJSON),
		CreatedAt:       row.CreatedAt,
	}
	_ = json.Unmarshal([]byte(strings.TrimSpace(row.RegressionsJSON)), &report.Regressions)
	_ = json.Unmarshal([]byte(strings.TrimSpace(row.ImprovementsJSON)), &report.Improvements)
	return report
}

func (s *SQLiteStore) CreateDataset(ctx context.Context, dataset *Dataset) error {
	if dataset == nil {
		return fmt.Errorf("dataset is required")
	}
	now := timeutil.NowTime()
	if dataset.CreatedAt.IsZero() {
		dataset.CreatedAt = now
	}
	if dataset.UpdatedAt.IsZero() {
		dataset.UpdatedAt = dataset.CreatedAt
	}
	_, err := s.execContext(ctx, `INSERT INTO harness_datasets (
		id, name, description, owner_user_id, subject, default_run_kind, default_profile, active_version_id, metadata_json,
		created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		dataset.ID, dataset.Name, dataset.Description, dataset.OwnerUserID, dataset.Subject, string(dataset.DefaultRunKind),
		dataset.DefaultProfile, dataset.ActiveVersionID, marshalMetadata(dataset.Metadata), dataset.CreatedAt, dataset.UpdatedAt,
	)
	return err
}

func (s *SQLiteStore) createDatasetTx(ctx context.Context, tx *sql.Tx, dataset *Dataset) error {
	if dataset == nil {
		return fmt.Errorf("dataset is required")
	}
	now := timeutil.NowTime()
	if dataset.CreatedAt.IsZero() {
		dataset.CreatedAt = now
	}
	if dataset.UpdatedAt.IsZero() {
		dataset.UpdatedAt = dataset.CreatedAt
	}
	_, err := txExecContextWithBusyRetry(ctx, tx, `INSERT INTO harness_datasets (
		id, name, description, owner_user_id, subject, default_run_kind, default_profile, active_version_id, metadata_json,
		created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		dataset.ID, dataset.Name, dataset.Description, dataset.OwnerUserID, dataset.Subject, string(dataset.DefaultRunKind),
		dataset.DefaultProfile, dataset.ActiveVersionID, marshalMetadata(dataset.Metadata), dataset.CreatedAt, dataset.UpdatedAt,
	)
	return err
}

func (s *SQLiteStore) UpdateDataset(ctx context.Context, dataset *Dataset) error {
	if dataset == nil {
		return fmt.Errorf("dataset is required")
	}
	dataset.UpdatedAt = timeutil.NowTime()
	_, err := s.execContext(ctx, `UPDATE harness_datasets SET
		name=?, description=?, owner_user_id=?, subject=?, default_run_kind=?, default_profile=?, active_version_id=?, metadata_json=?, updated_at=?
		WHERE id=?`,
		dataset.Name, dataset.Description, dataset.OwnerUserID, dataset.Subject, string(dataset.DefaultRunKind), dataset.DefaultProfile,
		dataset.ActiveVersionID, marshalMetadata(dataset.Metadata), dataset.UpdatedAt, dataset.ID,
	)
	return err
}

func (s *SQLiteStore) updateDatasetTx(ctx context.Context, tx *sql.Tx, dataset *Dataset) error {
	if dataset == nil {
		return fmt.Errorf("dataset is required")
	}
	dataset.UpdatedAt = timeutil.NowTime()
	_, err := txExecContextWithBusyRetry(ctx, tx, `UPDATE harness_datasets SET
		name=?, description=?, owner_user_id=?, subject=?, default_run_kind=?, default_profile=?, active_version_id=?, metadata_json=?, updated_at=?
		WHERE id=?`,
		dataset.Name, dataset.Description, dataset.OwnerUserID, dataset.Subject, string(dataset.DefaultRunKind), dataset.DefaultProfile,
		dataset.ActiveVersionID, marshalMetadata(dataset.Metadata), dataset.UpdatedAt, dataset.ID,
	)
	return err
}

func (s *SQLiteStore) GetDataset(ctx context.Context, id string) (*Dataset, error) {
	rows, err := s.selectDatasetRows(ctx, s.reader(), id)
	if err != nil {
		if !s.shouldFallbackToWriter(err, false) {
			return nil, err
		}
		rows, err = s.selectDatasetRows(ctx, s.db, id)
		if err != nil {
			return nil, err
		}
	} else if len(rows) == 0 && s.hasSeparateReader() {
		rows, err = s.selectDatasetRows(ctx, s.db, id)
		if err != nil {
			return nil, err
		}
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	dataset := datasetFromRow(rows[0])
	return &dataset, nil
}

func (s *SQLiteStore) selectDatasetRows(ctx context.Context, db *sql.DB, id string) ([]datasetRow, error) {
	var rows []datasetRow
	if _, err := z.TableContext(ctx, db, "harness_datasets").Select(&rows,
		z.Where(z.Eq("id", id)),
		z.Limit(1),
	); err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *SQLiteStore) ListDatasets(ctx context.Context, filter DatasetFilter) ([]Dataset, error) {
	var conds []interface{}
	if v := strings.TrimSpace(filter.OwnerUserID); v != "" {
		conds = append(conds, z.Eq("owner_user_id", v))
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	opts := []z.ZormItem{
		z.OrderBy("created_at DESC, updated_at DESC, rowid DESC"),
		z.Limit(limit),
	}
	if len(conds) > 0 {
		opts = append([]z.ZormItem{z.Where(conds...)}, opts...)
	}
	var rows []datasetRow
	if _, err := z.TableContext(ctx, s.reader(), "harness_datasets").Select(&rows, opts...); err != nil {
		return nil, err
	}
	out := make([]Dataset, 0, len(rows))
	for i := range rows {
		out = append(out, datasetFromRow(rows[i]))
	}
	return out, nil
}

func (s *SQLiteStore) FindDatasetByOwnerAndName(ctx context.Context, ownerUserID, name string) (*Dataset, error) {
	return s.findDatasetByOwnerAndNameWithDB(ctx, s.reader(), ownerUserID, name)
}

func (s *SQLiteStore) findDatasetByOwnerAndNameWithDB(ctx context.Context, db z.ZormDBIFace, ownerUserID, name string) (*Dataset, error) {
	var rows []datasetRow
	if _, err := z.TableContext(ctx, db, "harness_datasets").Select(&rows,
		z.Where(
			z.Eq("owner_user_id", strings.TrimSpace(ownerUserID)),
			z.Eq("name", strings.TrimSpace(name)),
		),
		z.OrderBy("created_at DESC, updated_at DESC, rowid DESC"),
		z.Limit(1),
	); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	dataset := datasetFromRow(rows[0])
	return &dataset, nil
}

func (s *SQLiteStore) CreateDatasetVersion(ctx context.Context, version *DatasetVersion) error {
	if version == nil {
		return fmt.Errorf("dataset version is required")
	}
	if version.CreatedAt.IsZero() {
		version.CreatedAt = timeutil.NowTime()
	}
	_, err := s.execContext(ctx, `INSERT INTO harness_dataset_versions (
		id, dataset_id, version, manifest_sha256, item_count, source_type, source_ref, manifest_json, metadata_json, created_by, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		version.ID, version.DatasetID, version.Version, version.ManifestSHA256, version.ItemCount, version.SourceType, version.SourceRef,
		marshalMetadata(version.Manifest), marshalMetadata(version.Metadata), version.CreatedBy, version.CreatedAt,
	)
	return err
}

func (s *SQLiteStore) createDatasetVersionTx(ctx context.Context, tx *sql.Tx, version *DatasetVersion) error {
	if version == nil {
		return fmt.Errorf("dataset version is required")
	}
	if version.CreatedAt.IsZero() {
		version.CreatedAt = timeutil.NowTime()
	}
	_, err := txExecContextWithBusyRetry(ctx, tx, `INSERT INTO harness_dataset_versions (
		id, dataset_id, version, manifest_sha256, item_count, source_type, source_ref, manifest_json, metadata_json, created_by, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		version.ID, version.DatasetID, version.Version, version.ManifestSHA256, version.ItemCount, version.SourceType, version.SourceRef,
		marshalMetadata(version.Manifest), marshalMetadata(version.Metadata), version.CreatedBy, version.CreatedAt,
	)
	return err
}

func (s *SQLiteStore) GetDatasetVersion(ctx context.Context, id string) (*DatasetVersion, error) {
	rows, err := s.selectDatasetVersionRows(ctx, s.reader(), id)
	if err != nil {
		if !s.shouldFallbackToWriter(err, false) {
			return nil, err
		}
		rows, err = s.selectDatasetVersionRows(ctx, s.db, id)
		if err != nil {
			return nil, err
		}
	} else if len(rows) == 0 && s.hasSeparateReader() {
		rows, err = s.selectDatasetVersionRows(ctx, s.db, id)
		if err != nil {
			return nil, err
		}
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	version := datasetVersionFromRow(rows[0])
	return &version, nil
}

func (s *SQLiteStore) selectDatasetVersionRows(ctx context.Context, db *sql.DB, id string) ([]datasetVersionRow, error) {
	var rows []datasetVersionRow
	if _, err := z.TableContext(ctx, db, "harness_dataset_versions").Select(&rows,
		z.Where(z.Eq("id", id)),
		z.Limit(1),
	); err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *SQLiteStore) ListDatasetVersions(ctx context.Context, datasetID string, limit int) ([]DatasetVersion, error) {
	if strings.TrimSpace(datasetID) == "" {
		return nil, sql.ErrNoRows
	}
	if limit <= 0 {
		limit = 50
	}
	var rows []datasetVersionRow
	if _, err := z.TableContext(ctx, s.reader(), "harness_dataset_versions").Select(&rows,
		z.Where(z.Eq("dataset_id", datasetID)),
		z.OrderBy("created_at DESC"),
		z.Limit(limit),
	); err != nil {
		return nil, err
	}
	out := make([]DatasetVersion, 0, len(rows))
	for i := range rows {
		out = append(out, datasetVersionFromRow(rows[i]))
	}
	return out, nil
}

func (s *SQLiteStore) FindDatasetVersionByDatasetAndVersion(ctx context.Context, datasetID, version string) (*DatasetVersion, error) {
	return s.findDatasetVersionByDatasetAndVersionWithDB(ctx, s.reader(), datasetID, version)
}

func (s *SQLiteStore) findDatasetVersionByDatasetAndVersionWithDB(ctx context.Context, db z.ZormDBIFace, datasetID, version string) (*DatasetVersion, error) {
	var rows []datasetVersionRow
	if _, err := z.TableContext(ctx, db, "harness_dataset_versions").Select(&rows,
		z.Where(
			z.Eq("dataset_id", strings.TrimSpace(datasetID)),
			z.Eq("version", strings.TrimSpace(version)),
		),
		z.OrderBy("created_at DESC"),
		z.Limit(1),
	); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	versionRow := datasetVersionFromRow(rows[0])
	return &versionRow, nil
}

func (s *SQLiteStore) CreateEvalSpec(ctx context.Context, spec *EvalSpec) error {
	if spec == nil {
		return fmt.Errorf("eval spec is required")
	}
	now := timeutil.NowTime()
	if spec.CreatedAt.IsZero() {
		spec.CreatedAt = now
	}
	if spec.UpdatedAt.IsZero() {
		spec.UpdatedAt = spec.CreatedAt
	}
	_, err := s.execContext(ctx, `INSERT INTO harness_eval_specs (
		id, name, owner_user_id, subject, run_kind, profile, dataset_id, dataset_version_id, scheduler_json, scoring_json, runtime_policy_json, metadata_json,
		created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		spec.ID, spec.Name, spec.OwnerUserID, spec.Subject, string(spec.RunKind), spec.Profile, spec.DatasetID, spec.DatasetVersionID,
		marshalInterface(spec.SchedulerConfig), marshalInterface(spec.ScoringConfig), marshalMetadata(spec.RuntimePolicy), marshalMetadata(spec.Metadata),
		spec.CreatedAt, spec.UpdatedAt,
	)
	return err
}

func (s *SQLiteStore) createEvalSpecTx(ctx context.Context, tx *sql.Tx, spec *EvalSpec) error {
	if spec == nil {
		return fmt.Errorf("eval spec is required")
	}
	now := timeutil.NowTime()
	if spec.CreatedAt.IsZero() {
		spec.CreatedAt = now
	}
	if spec.UpdatedAt.IsZero() {
		spec.UpdatedAt = spec.CreatedAt
	}
	_, err := txExecContextWithBusyRetry(ctx, tx, `INSERT INTO harness_eval_specs (
		id, name, owner_user_id, subject, run_kind, profile, dataset_id, dataset_version_id, scheduler_json, scoring_json, runtime_policy_json, metadata_json,
		created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		spec.ID, spec.Name, spec.OwnerUserID, spec.Subject, string(spec.RunKind), spec.Profile, spec.DatasetID, spec.DatasetVersionID,
		marshalInterface(spec.SchedulerConfig), marshalInterface(spec.ScoringConfig), marshalMetadata(spec.RuntimePolicy), marshalMetadata(spec.Metadata),
		spec.CreatedAt, spec.UpdatedAt,
	)
	return err
}

func (s *SQLiteStore) UpdateEvalSpec(ctx context.Context, spec *EvalSpec) error {
	if spec == nil {
		return fmt.Errorf("eval spec is required")
	}
	spec.UpdatedAt = timeutil.NowTime()
	_, err := s.execContext(ctx, `UPDATE harness_eval_specs SET
		name=?, owner_user_id=?, subject=?, run_kind=?, profile=?, dataset_id=?, dataset_version_id=?, scheduler_json=?, scoring_json=?, runtime_policy_json=?, metadata_json=?, updated_at=?
		WHERE id=?`,
		spec.Name, spec.OwnerUserID, spec.Subject, string(spec.RunKind), spec.Profile, spec.DatasetID, spec.DatasetVersionID,
		marshalInterface(spec.SchedulerConfig), marshalInterface(spec.ScoringConfig), marshalMetadata(spec.RuntimePolicy), marshalMetadata(spec.Metadata),
		spec.UpdatedAt, spec.ID,
	)
	return err
}

func (s *SQLiteStore) updateEvalSpecTx(ctx context.Context, tx *sql.Tx, spec *EvalSpec) error {
	if spec == nil {
		return fmt.Errorf("eval spec is required")
	}
	spec.UpdatedAt = timeutil.NowTime()
	_, err := txExecContextWithBusyRetry(ctx, tx, `UPDATE harness_eval_specs SET
		name=?, owner_user_id=?, subject=?, run_kind=?, profile=?, dataset_id=?, dataset_version_id=?, scheduler_json=?, scoring_json=?, runtime_policy_json=?, metadata_json=?, updated_at=?
		WHERE id=?`,
		spec.Name, spec.OwnerUserID, spec.Subject, string(spec.RunKind), spec.Profile, spec.DatasetID, spec.DatasetVersionID,
		marshalInterface(spec.SchedulerConfig), marshalInterface(spec.ScoringConfig), marshalMetadata(spec.RuntimePolicy), marshalMetadata(spec.Metadata),
		spec.UpdatedAt, spec.ID,
	)
	return err
}

func (s *SQLiteStore) GetEvalSpec(ctx context.Context, id string) (*EvalSpec, error) {
	rows, err := s.selectEvalSpecRows(ctx, s.reader(), id)
	if err != nil {
		if !s.shouldFallbackToWriter(err, false) {
			return nil, err
		}
		rows, err = s.selectEvalSpecRows(ctx, s.db, id)
		if err != nil {
			return nil, err
		}
	} else if len(rows) == 0 && s.hasSeparateReader() {
		rows, err = s.selectEvalSpecRows(ctx, s.db, id)
		if err != nil {
			return nil, err
		}
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	spec := evalSpecFromRow(rows[0])
	return &spec, nil
}

func (s *SQLiteStore) selectEvalSpecRows(ctx context.Context, db *sql.DB, id string) ([]evalSpecRow, error) {
	var rows []evalSpecRow
	if _, err := z.TableContext(ctx, db, "harness_eval_specs").Select(&rows,
		z.Where(z.Eq("id", id)),
		z.Limit(1),
	); err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *SQLiteStore) ListEvalSpecs(ctx context.Context, filter EvalSpecFilter) ([]EvalSpec, error) {
	var conds []interface{}
	if v := strings.TrimSpace(filter.OwnerUserID); v != "" {
		conds = append(conds, z.Eq("owner_user_id", v))
	}
	if v := strings.TrimSpace(filter.DatasetID); v != "" {
		conds = append(conds, z.Eq("dataset_id", v))
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	opts := []z.ZormItem{
		z.OrderBy("created_at DESC"),
		z.Limit(limit),
	}
	if len(conds) > 0 {
		opts = append([]z.ZormItem{z.Where(conds...)}, opts...)
	}
	var rows []evalSpecRow
	if _, err := z.TableContext(ctx, s.reader(), "harness_eval_specs").Select(&rows, opts...); err != nil {
		return nil, err
	}
	out := make([]EvalSpec, 0, len(rows))
	for i := range rows {
		out = append(out, evalSpecFromRow(rows[i]))
	}
	return out, nil
}

func (s *SQLiteStore) FindEvalSpecByOwnerDatasetAndName(ctx context.Context, ownerUserID, datasetID, name string) (*EvalSpec, error) {
	return s.findEvalSpecByOwnerDatasetAndNameWithDB(ctx, s.reader(), ownerUserID, datasetID, name)
}

func (s *SQLiteStore) findEvalSpecByOwnerDatasetAndNameWithDB(ctx context.Context, db z.ZormDBIFace, ownerUserID, datasetID, name string) (*EvalSpec, error) {
	var rows []evalSpecRow
	if _, err := z.TableContext(ctx, db, "harness_eval_specs").Select(&rows,
		z.Where(
			z.Eq("owner_user_id", strings.TrimSpace(ownerUserID)),
			z.Eq("dataset_id", strings.TrimSpace(datasetID)),
			z.Eq("name", strings.TrimSpace(name)),
		),
		z.OrderBy("created_at DESC"),
		z.Limit(1),
	); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	spec := evalSpecFromRow(rows[0])
	return &spec, nil
}

func (s *SQLiteStore) CreateEvalRun(ctx context.Context, evalRun *EvalRun) error {
	if evalRun == nil {
		return fmt.Errorf("eval run is required")
	}
	now := monotonicHarnessTime()
	if evalRun.CreatedAt.IsZero() {
		evalRun.CreatedAt = now
	}
	if evalRun.UpdatedAt.IsZero() {
		evalRun.UpdatedAt = evalRun.CreatedAt
	}
	_, err := s.execContext(ctx, `INSERT INTO harness_eval_runs (
		id, eval_spec_id, group_id, dataset_version_id, baseline_eval_run_id, title, owner_user_id, status, trigger_kind, trigger_ref, metadata_json, summary_json,
		created_at, updated_at, started_at, finished_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		evalRun.ID, evalRun.EvalSpecID, evalRun.GroupID, evalRun.DatasetVersionID, evalRun.BaselineEvalRunID, evalRun.Title, evalRun.OwnerUserID,
		string(evalRun.Status), evalRun.TriggerKind, evalRun.TriggerRef, marshalMetadata(evalRun.Metadata), marshalMetadata(evalRun.Summary),
		evalRun.CreatedAt, evalRun.UpdatedAt, nullableTime(evalRun.StartedAt), nullableTime(evalRun.FinishedAt),
	)
	return err
}

func (s *SQLiteStore) UpdateEvalRun(ctx context.Context, evalRun *EvalRun) error {
	if evalRun == nil {
		return fmt.Errorf("eval run is required")
	}
	evalRun.UpdatedAt = monotonicHarnessTime()
	_, err := s.execContext(ctx, `UPDATE harness_eval_runs SET
		eval_spec_id=?, group_id=?, dataset_version_id=?, baseline_eval_run_id=?, title=?, owner_user_id=?, status=?, trigger_kind=?, trigger_ref=?, metadata_json=?, summary_json=?,
		updated_at=?, started_at=?, finished_at=?
		WHERE id=?`,
		evalRun.EvalSpecID, evalRun.GroupID, evalRun.DatasetVersionID, evalRun.BaselineEvalRunID, evalRun.Title, evalRun.OwnerUserID,
		string(evalRun.Status), evalRun.TriggerKind, evalRun.TriggerRef, marshalMetadata(evalRun.Metadata), marshalMetadata(evalRun.Summary),
		evalRun.UpdatedAt, nullableTime(evalRun.StartedAt), nullableTime(evalRun.FinishedAt), evalRun.ID,
	)
	return err
}

func (s *SQLiteStore) GetEvalRun(ctx context.Context, id string) (*EvalRun, error) {
	rows, err := s.selectEvalRunRows(ctx, s.reader(), id)
	if err != nil {
		if !s.shouldFallbackToWriter(err, false) {
			return nil, err
		}
		rows, err = s.selectEvalRunRows(ctx, s.db, id)
		if err != nil {
			return nil, err
		}
	} else if len(rows) == 0 && s.hasSeparateReader() {
		rows, err = s.selectEvalRunRows(ctx, s.db, id)
		if err != nil {
			return nil, err
		}
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	evalRun := evalRunFromRow(rows[0])
	return &evalRun, nil
}

func (s *SQLiteStore) selectEvalRunRows(ctx context.Context, db *sql.DB, id string) ([]evalRunRow, error) {
	var rows []evalRunRow
	if _, err := z.TableContext(ctx, db, "harness_eval_runs").Select(&rows,
		z.Where(z.Eq("id", id)),
		z.Limit(1),
	); err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *SQLiteStore) ListEvalRuns(ctx context.Context, filter EvalRunFilter) ([]EvalRun, error) {
	rows, err := s.listEvalRunRows(ctx, s.reader(), filter)
	if err != nil {
		if !s.shouldFallbackToWriter(err, false) {
			return nil, err
		}
		rows, err = s.listEvalRunRows(ctx, s.db, filter)
		if err != nil {
			return nil, err
		}
	} else if s.hasSeparateReader() {
		writerRows, writerErr := s.listEvalRunRows(ctx, s.db, filter)
		if writerErr == nil {
			rows = mergeUniqueRowsByKey(rows, writerRows, filter.Limit, func(row evalRunRow) string { return row.ID })
		}
	}
	out := make([]EvalRun, 0, len(rows))
	for i := range rows {
		out = append(out, evalRunFromRow(rows[i]))
	}
	return out, nil
}

func (s *SQLiteStore) listEvalRunRows(ctx context.Context, db *sql.DB, filter EvalRunFilter) ([]evalRunRow, error) {
	var conds []interface{}
	if v := strings.TrimSpace(filter.OwnerUserID); v != "" {
		conds = append(conds, z.Eq("owner_user_id", v))
	}
	if v := strings.TrimSpace(filter.EvalSpecID); v != "" {
		conds = append(conds, z.Eq("eval_spec_id", v))
	}
	if len(filter.Statuses) > 0 {
		statuses := make([]interface{}, 0, len(filter.Statuses))
		for _, status := range filter.Statuses {
			if status == "" {
				continue
			}
			statuses = append(statuses, string(status))
		}
		if len(statuses) > 0 {
			conds = append(conds, z.In("status", statuses...))
		}
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	opts := []z.ZormItem{
		z.OrderBy("created_at DESC"),
		z.Limit(limit),
	}
	if len(conds) > 0 {
		opts = append([]z.ZormItem{z.Where(conds...)}, opts...)
	}
	var rows []evalRunRow
	if _, err := z.TableContext(ctx, db, "harness_eval_runs").Select(&rows, opts...); err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *SQLiteStore) ClearDefaultBaseline(ctx context.Context, evalSpecID string) error {
	if strings.TrimSpace(evalSpecID) == "" {
		return nil
	}
	_, err := s.execContext(ctx, `UPDATE harness_baselines SET is_default = 0, updated_at = ? WHERE eval_spec_id = ? AND is_default = 1`,
		timeutil.NowTime(), strings.TrimSpace(evalSpecID))
	return err
}

func (s *SQLiteStore) CreateBaseline(ctx context.Context, baseline *Baseline) error {
	if baseline == nil {
		return fmt.Errorf("baseline is required")
	}
	now := timeutil.NowTime()
	if baseline.CreatedAt.IsZero() {
		baseline.CreatedAt = now
	}
	if baseline.UpdatedAt.IsZero() {
		baseline.UpdatedAt = baseline.CreatedAt
	}
	isDefault := 0
	if baseline.IsDefault {
		isDefault = 1
	}
	_, err := s.execContext(ctx, `INSERT INTO harness_baselines (
		id, name, subject, owner_user_id, eval_spec_id, eval_run_id, is_default, metadata_json, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		baseline.ID, baseline.Name, baseline.Subject, baseline.OwnerUserID, baseline.EvalSpecID, baseline.EvalRunID, isDefault,
		marshalMetadata(baseline.Metadata), baseline.CreatedAt, baseline.UpdatedAt,
	)
	return err
}

func (s *SQLiteStore) GetBaseline(ctx context.Context, id string) (*Baseline, error) {
	var rows []baselineRow
	if _, err := z.TableContext(ctx, s.reader(), "harness_baselines").Select(&rows,
		z.Where(z.Eq("id", id)),
		z.Limit(1),
	); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	baseline := baselineFromRow(rows[0])
	return &baseline, nil
}

func (s *SQLiteStore) ListBaselines(ctx context.Context, filter BaselineFilter) ([]Baseline, error) {
	var conds []interface{}
	if v := strings.TrimSpace(filter.OwnerUserID); v != "" {
		conds = append(conds, z.Eq("owner_user_id", v))
	}
	if v := strings.TrimSpace(filter.EvalSpecID); v != "" {
		conds = append(conds, z.Eq("eval_spec_id", v))
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	opts := []z.ZormItem{
		z.OrderBy("is_default DESC", "updated_at DESC"),
		z.Limit(limit),
	}
	if len(conds) > 0 {
		opts = append([]z.ZormItem{z.Where(conds...)}, opts...)
	}
	var rows []baselineRow
	if _, err := z.TableContext(ctx, s.reader(), "harness_baselines").Select(&rows, opts...); err != nil {
		return nil, err
	}
	out := make([]Baseline, 0, len(rows))
	for i := range rows {
		out = append(out, baselineFromRow(rows[i]))
	}
	return out, nil
}

func (s *SQLiteStore) CreateComparisonReport(ctx context.Context, report *ComparisonReport) error {
	if report == nil {
		return fmt.Errorf("comparison report is required")
	}
	if report.CreatedAt.IsZero() {
		report.CreatedAt = timeutil.NowTime()
	}
	_, err := s.execContext(ctx, `INSERT INTO harness_comparison_reports (
		id, owner_user_id, baseline_id, eval_spec_id, base_eval_run_id, target_eval_run_id, summary_json, regressions_json, improvements_json, scorer_delta_json, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		report.ID, report.OwnerUserID, report.BaselineID, report.EvalSpecID, report.BaseEvalRunID, report.TargetEvalRunID,
		marshalMetadata(report.Summary), marshalInterface(report.Regressions), marshalInterface(report.Improvements), marshalMetadata(report.ScorerDelta), report.CreatedAt,
	)
	return err
}

func (s *SQLiteStore) GetComparisonReport(ctx context.Context, id string) (*ComparisonReport, error) {
	var rows []comparisonReportRow
	if _, err := z.TableContext(ctx, s.reader(), "harness_comparison_reports").Select(&rows,
		z.Where(z.Eq("id", id)),
		z.Limit(1),
	); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	report := comparisonReportFromRow(rows[0])
	return &report, nil
}

func scanDataset(scanner rowScanner) (*Dataset, error) {
	var (
		dataset     Dataset
		defaultKind string
		metadata    string
	)
	if err := scanner.Scan(
		&dataset.ID, &dataset.Name, &dataset.Description, &dataset.OwnerUserID, &dataset.Subject, &defaultKind, &dataset.DefaultProfile, &dataset.ActiveVersionID, &metadata,
		&dataset.CreatedAt, &dataset.UpdatedAt,
	); err != nil {
		return nil, err
	}
	dataset.DefaultRunKind = RunKind(defaultKind)
	dataset.Metadata = unmarshalMetadata(metadata)
	return &dataset, nil
}

func scanDatasetVersion(scanner rowScanner) (*DatasetVersion, error) {
	var (
		version                DatasetVersion
		manifestJSON, metaJSON string
	)
	if err := scanner.Scan(
		&version.ID, &version.DatasetID, &version.Version, &version.ManifestSHA256, &version.ItemCount, &version.SourceType, &version.SourceRef, &manifestJSON, &metaJSON, &version.CreatedBy, &version.CreatedAt,
	); err != nil {
		return nil, err
	}
	version.Manifest = unmarshalMetadata(manifestJSON)
	version.Metadata = unmarshalMetadata(metaJSON)
	return &version, nil
}

func scanEvalSpec(scanner rowScanner) (*EvalSpec, error) {
	var (
		spec                                                    EvalSpec
		runKind                                                 string
		schedulerJSON, scoringJSON, runtimePolicyJSON, metaJSON string
	)
	if err := scanner.Scan(
		&spec.ID, &spec.Name, &spec.OwnerUserID, &spec.Subject, &runKind, &spec.Profile, &spec.DatasetID, &spec.DatasetVersionID,
		&schedulerJSON, &scoringJSON, &runtimePolicyJSON, &metaJSON, &spec.CreatedAt, &spec.UpdatedAt,
	); err != nil {
		return nil, err
	}
	spec.RunKind = RunKind(runKind)
	_ = unmarshalInto(schedulerJSON, &spec.SchedulerConfig)
	_ = unmarshalInto(scoringJSON, &spec.ScoringConfig)
	spec.RuntimePolicy = unmarshalMetadata(runtimePolicyJSON)
	spec.Metadata = unmarshalMetadata(metaJSON)
	return &spec, nil
}

func scanEvalRun(scanner rowScanner) (*EvalRun, error) {
	var (
		evalRun                   EvalRun
		status                    string
		metadataJSON, summaryJSON string
		startedAt, finishedAt     nullableSQLiteTime
	)
	if err := scanner.Scan(
		&evalRun.ID, &evalRun.EvalSpecID, &evalRun.GroupID, &evalRun.DatasetVersionID, &evalRun.BaselineEvalRunID, &evalRun.Title, &evalRun.OwnerUserID,
		&status, &evalRun.TriggerKind, &evalRun.TriggerRef, &metadataJSON, &summaryJSON,
		&evalRun.CreatedAt, &evalRun.UpdatedAt, &startedAt, &finishedAt,
	); err != nil {
		return nil, err
	}
	evalRun.Status = RunGroupStatus(status)
	evalRun.Metadata = unmarshalMetadata(metadataJSON)
	evalRun.Summary = unmarshalMetadata(summaryJSON)
	evalRun.StartedAt = nullableSQLiteTimePtr(startedAt)
	evalRun.FinishedAt = nullableSQLiteTimePtr(finishedAt)
	return &evalRun, nil
}

func scanBaseline(scanner rowScanner) (*Baseline, error) {
	var (
		baseline     Baseline
		isDefault    int
		metadataJSON string
	)
	if err := scanner.Scan(
		&baseline.ID, &baseline.Name, &baseline.Subject, &baseline.OwnerUserID, &baseline.EvalSpecID, &baseline.EvalRunID, &isDefault, &metadataJSON,
		&baseline.CreatedAt, &baseline.UpdatedAt,
	); err != nil {
		return nil, err
	}
	baseline.IsDefault = isDefault > 0
	baseline.Metadata = unmarshalMetadata(metadataJSON)
	return &baseline, nil
}

func scanComparisonReport(scanner rowScanner) (*ComparisonReport, error) {
	var (
		report                                                          ComparisonReport
		summaryJSON, regressionsJSON, improvementsJSON, scorerDeltaJSON string
	)
	if err := scanner.Scan(
		&report.ID, &report.OwnerUserID, &report.BaselineID, &report.EvalSpecID, &report.BaseEvalRunID, &report.TargetEvalRunID,
		&summaryJSON, &regressionsJSON, &improvementsJSON, &scorerDeltaJSON, &report.CreatedAt,
	); err != nil {
		return nil, err
	}
	report.Summary = unmarshalMetadata(summaryJSON)
	_ = json.Unmarshal([]byte(strings.TrimSpace(regressionsJSON)), &report.Regressions)
	_ = json.Unmarshal([]byte(strings.TrimSpace(improvementsJSON)), &report.Improvements)
	report.ScorerDelta = unmarshalMetadata(scorerDeltaJSON)
	return &report, nil
}
