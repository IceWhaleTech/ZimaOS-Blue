package harness

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

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
	_, err := s.db.ExecContext(ctx, `INSERT INTO harness_datasets (
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
	_, err := s.db.ExecContext(ctx, `UPDATE harness_datasets SET
		name=?, description=?, owner_user_id=?, subject=?, default_run_kind=?, default_profile=?, active_version_id=?, metadata_json=?, updated_at=?
		WHERE id=?`,
		dataset.Name, dataset.Description, dataset.OwnerUserID, dataset.Subject, string(dataset.DefaultRunKind), dataset.DefaultProfile,
		dataset.ActiveVersionID, marshalMetadata(dataset.Metadata), dataset.UpdatedAt, dataset.ID,
	)
	return err
}

func (s *SQLiteStore) GetDataset(ctx context.Context, id string) (*Dataset, error) {
	row := s.db.QueryRowContext(ctx, `SELECT
		id, name, description, owner_user_id, subject, default_run_kind, default_profile, active_version_id, metadata_json,
		created_at, updated_at
		FROM harness_datasets WHERE id = ?`, id)
	return scanDataset(row)
}

func (s *SQLiteStore) ListDatasets(ctx context.Context, filter DatasetFilter) ([]Dataset, error) {
	query := `SELECT
		id, name, description, owner_user_id, subject, default_run_kind, default_profile, active_version_id, metadata_json,
		created_at, updated_at
		FROM harness_datasets`
	var (
		clauses []string
		args    []interface{}
	)
	if v := strings.TrimSpace(filter.OwnerUserID); v != "" {
		clauses = append(clauses, "owner_user_id = ?")
		args = append(args, v)
	}
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY created_at DESC"
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	query += " LIMIT ?"
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Dataset
	for rows.Next() {
		dataset, scanErr := scanDataset(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, *dataset)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) CreateDatasetVersion(ctx context.Context, version *DatasetVersion) error {
	if version == nil {
		return fmt.Errorf("dataset version is required")
	}
	if version.CreatedAt.IsZero() {
		version.CreatedAt = timeutil.NowTime()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO harness_dataset_versions (
		id, dataset_id, version, manifest_sha256, item_count, source_type, source_ref, manifest_json, metadata_json, created_by, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		version.ID, version.DatasetID, version.Version, version.ManifestSHA256, version.ItemCount, version.SourceType, version.SourceRef,
		marshalMetadata(version.Manifest), marshalMetadata(version.Metadata), version.CreatedBy, version.CreatedAt,
	)
	return err
}

func (s *SQLiteStore) GetDatasetVersion(ctx context.Context, id string) (*DatasetVersion, error) {
	row := s.db.QueryRowContext(ctx, `SELECT
		id, dataset_id, version, manifest_sha256, item_count, source_type, source_ref, manifest_json, metadata_json, created_by, created_at
		FROM harness_dataset_versions WHERE id = ?`, id)
	return scanDatasetVersion(row)
}

func (s *SQLiteStore) ListDatasetVersions(ctx context.Context, datasetID string, limit int) ([]DatasetVersion, error) {
	if strings.TrimSpace(datasetID) == "" {
		return nil, sql.ErrNoRows
	}
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT
		id, dataset_id, version, manifest_sha256, item_count, source_type, source_ref, manifest_json, metadata_json, created_by, created_at
		FROM harness_dataset_versions
		WHERE dataset_id = ?
		ORDER BY created_at DESC
		LIMIT ?`, datasetID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DatasetVersion
	for rows.Next() {
		version, scanErr := scanDatasetVersion(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, *version)
	}
	return out, rows.Err()
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
	_, err := s.db.ExecContext(ctx, `INSERT INTO harness_eval_specs (
		id, name, owner_user_id, subject, run_kind, profile, dataset_id, dataset_version_id, scheduler_json, scoring_json, runtime_policy_json, metadata_json,
		created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		spec.ID, spec.Name, spec.OwnerUserID, spec.Subject, string(spec.RunKind), spec.Profile, spec.DatasetID, spec.DatasetVersionID,
		marshalInterface(spec.SchedulerConfig), marshalInterface(spec.ScoringConfig), marshalMetadata(spec.RuntimePolicy), marshalMetadata(spec.Metadata),
		spec.CreatedAt, spec.UpdatedAt,
	)
	return err
}

func (s *SQLiteStore) GetEvalSpec(ctx context.Context, id string) (*EvalSpec, error) {
	row := s.db.QueryRowContext(ctx, `SELECT
		id, name, owner_user_id, subject, run_kind, profile, dataset_id, dataset_version_id, scheduler_json, scoring_json, runtime_policy_json, metadata_json,
		created_at, updated_at
		FROM harness_eval_specs WHERE id = ?`, id)
	return scanEvalSpec(row)
}

func (s *SQLiteStore) ListEvalSpecs(ctx context.Context, filter EvalSpecFilter) ([]EvalSpec, error) {
	query := `SELECT
		id, name, owner_user_id, subject, run_kind, profile, dataset_id, dataset_version_id, scheduler_json, scoring_json, runtime_policy_json, metadata_json,
		created_at, updated_at
		FROM harness_eval_specs`
	var (
		clauses []string
		args    []interface{}
	)
	if v := strings.TrimSpace(filter.OwnerUserID); v != "" {
		clauses = append(clauses, "owner_user_id = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(filter.DatasetID); v != "" {
		clauses = append(clauses, "dataset_id = ?")
		args = append(args, v)
	}
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY created_at DESC"
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	query += " LIMIT ?"
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EvalSpec
	for rows.Next() {
		spec, scanErr := scanEvalSpec(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, *spec)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) CreateEvalRun(ctx context.Context, evalRun *EvalRun) error {
	if evalRun == nil {
		return fmt.Errorf("eval run is required")
	}
	now := timeutil.NowTime()
	if evalRun.CreatedAt.IsZero() {
		evalRun.CreatedAt = now
	}
	if evalRun.UpdatedAt.IsZero() {
		evalRun.UpdatedAt = evalRun.CreatedAt
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO harness_eval_runs (
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
	evalRun.UpdatedAt = timeutil.NowTime()
	_, err := s.db.ExecContext(ctx, `UPDATE harness_eval_runs SET
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
	row := s.db.QueryRowContext(ctx, `SELECT
		id, eval_spec_id, group_id, dataset_version_id, baseline_eval_run_id, title, owner_user_id, status, trigger_kind, trigger_ref, metadata_json, summary_json,
		created_at, updated_at, started_at, finished_at
		FROM harness_eval_runs WHERE id = ?`, id)
	return scanEvalRun(row)
}

func (s *SQLiteStore) ListEvalRuns(ctx context.Context, filter EvalRunFilter) ([]EvalRun, error) {
	query := `SELECT
		id, eval_spec_id, group_id, dataset_version_id, baseline_eval_run_id, title, owner_user_id, status, trigger_kind, trigger_ref, metadata_json, summary_json,
		created_at, updated_at, started_at, finished_at
		FROM harness_eval_runs`
	var (
		clauses []string
		args    []interface{}
	)
	if v := strings.TrimSpace(filter.OwnerUserID); v != "" {
		clauses = append(clauses, "owner_user_id = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(filter.EvalSpecID); v != "" {
		clauses = append(clauses, "eval_spec_id = ?")
		args = append(args, v)
	}
	if len(filter.Statuses) > 0 {
		parts := make([]string, 0, len(filter.Statuses))
		for _, status := range filter.Statuses {
			if status == "" {
				continue
			}
			parts = append(parts, "?")
			args = append(args, string(status))
		}
		if len(parts) > 0 {
			clauses = append(clauses, "status IN ("+strings.Join(parts, ",")+")")
		}
	}
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY created_at DESC"
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	query += " LIMIT ?"
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EvalRun
	for rows.Next() {
		evalRun, scanErr := scanEvalRun(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, *evalRun)
	}
	return out, rows.Err()
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
		startedAt, finishedAt     sql.NullTime
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
	if startedAt.Valid {
		ts := startedAt.Time
		evalRun.StartedAt = &ts
	}
	if finishedAt.Valid {
		ts := finishedAt.Time
		evalRun.FinishedAt = &ts
	}
	return &evalRun, nil
}
