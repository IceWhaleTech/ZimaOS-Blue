package harness

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

type BatchTrajectoryCase struct {
	Key      string                 `json:"key,omitempty"`
	Prompt   string                 `json:"prompt,omitempty"`
	Input    map[string]interface{} `json:"input,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type BatchTrajectoryCaseResult struct {
	Input        map[string]interface{} `json:"input,omitempty"`
	Expected     map[string]interface{} `json:"expected,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	TraceSummary string                 `json:"trace_summary,omitempty"`
	Provenance   map[string]interface{} `json:"provenance,omitempty"`
}

type BatchTrajectoryExecutor interface {
	ExecuteBatchTrajectoryCase(ctx context.Context, c BatchTrajectoryCase) (*BatchTrajectoryCaseResult, error)
}

type BatchTrajectoryRunnerConfig struct {
	DatasetName    string                 `json:"dataset_name"`
	DatasetSubject string                 `json:"dataset_subject"`
	DatasetVersion string                 `json:"dataset_version"`
	SourceType     string                 `json:"source_type,omitempty"`
	SourceRef      string                 `json:"source_ref,omitempty"`
	CreatedBy      string                 `json:"created_by,omitempty"`
	CandidateID    string                 `json:"candidate_id,omitempty"`
	CheckpointPath string                 `json:"checkpoint_path"`
	DefaultRunKind RunKind                `json:"default_run_kind,omitempty"`
	DefaultProfile string                 `json:"default_profile,omitempty"`
	Scheduler      GroupSchedulerConfig   `json:"scheduler,omitempty"`
	Scoring        GroupScoringConfig     `json:"scoring,omitempty"`
	RuntimePolicy  map[string]interface{} `json:"runtime_policy,omitempty"`
	MaxRetries     int                    `json:"max_retries,omitempty"`
}

type BatchTrajectoryRunResult struct {
	Manifest           DatasetManifest    `json:"manifest"`
	DatasetVersionSpec DatasetVersionSpec `json:"dataset_version_spec"`
	CompletedCaseKeys  []string           `json:"completed_case_keys,omitempty"`
	FailedCaseKeys     []string           `json:"failed_case_keys,omitempty"`
	CheckpointPath     string             `json:"checkpoint_path,omitempty"`
}

type BatchTrajectoryRunner struct {
	cfg      BatchTrajectoryRunnerConfig
	executor BatchTrajectoryExecutor
}

type batchTrajectoryCheckpoint struct {
	Version     int                                      `json:"version"`
	DatasetName string                                   `json:"dataset_name,omitempty"`
	CandidateID string                                   `json:"candidate_id,omitempty"`
	Cases       map[string]batchTrajectoryCheckpointCase `json:"cases,omitempty"`
	UpdatedAt   time.Time                                `json:"updated_at"`
}

type batchTrajectoryCheckpointCase struct {
	Key       string              `json:"key"`
	Status    string              `json:"status"`
	Attempts  int                 `json:"attempts"`
	Error     string              `json:"error,omitempty"`
	Item      DatasetManifestItem `json:"item"`
	UpdatedAt time.Time           `json:"updated_at"`
}

const (
	batchTrajectoryCheckpointVersion         = 1
	batchTrajectoryCheckpointStatusCompleted = "completed"
	batchTrajectoryCheckpointStatusFailed    = "failed"
)

func NewBatchTrajectoryRunner(cfg BatchTrajectoryRunnerConfig, executor BatchTrajectoryExecutor) *BatchTrajectoryRunner {
	cfg.DatasetName = strings.TrimSpace(cfg.DatasetName)
	cfg.DatasetSubject = strings.TrimSpace(cfg.DatasetSubject)
	cfg.DatasetVersion = strings.TrimSpace(cfg.DatasetVersion)
	cfg.SourceType = strings.TrimSpace(cfg.SourceType)
	cfg.SourceRef = strings.TrimSpace(cfg.SourceRef)
	cfg.CreatedBy = strings.TrimSpace(cfg.CreatedBy)
	cfg.CandidateID = strings.TrimSpace(cfg.CandidateID)
	cfg.CheckpointPath = strings.TrimSpace(cfg.CheckpointPath)
	cfg.DefaultProfile = strings.TrimSpace(cfg.DefaultProfile)
	if cfg.SourceType == "" {
		cfg.SourceType = "batch_trajectory"
	}
	return &BatchTrajectoryRunner{
		cfg:      cfg,
		executor: executor,
	}
}

func (r *BatchTrajectoryRunner) Run(ctx context.Context, cases []BatchTrajectoryCase) (*BatchTrajectoryRunResult, error) {
	if r == nil || r.executor == nil {
		return nil, fmt.Errorf("batch trajectory runner executor is required")
	}
	if strings.TrimSpace(r.cfg.DatasetName) == "" {
		return nil, fmt.Errorf("batch trajectory dataset_name is required")
	}
	if strings.TrimSpace(r.cfg.DatasetSubject) == "" {
		return nil, fmt.Errorf("batch trajectory dataset_subject is required")
	}
	if strings.TrimSpace(r.cfg.DatasetVersion) == "" {
		return nil, fmt.Errorf("batch trajectory dataset_version is required")
	}
	if strings.TrimSpace(r.cfg.CheckpointPath) == "" {
		return nil, fmt.Errorf("batch trajectory checkpoint_path is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	checkpoint, err := loadBatchTrajectoryCheckpoint(r.cfg.CheckpointPath)
	if err != nil {
		return nil, err
	}
	if checkpoint == nil {
		checkpoint = &batchTrajectoryCheckpoint{
			Version:     batchTrajectoryCheckpointVersion,
			DatasetName: r.cfg.DatasetName,
			CandidateID: r.cfg.CandidateID,
			Cases:       make(map[string]batchTrajectoryCheckpointCase),
		}
	}
	if checkpoint.Cases == nil {
		checkpoint.Cases = make(map[string]batchTrajectoryCheckpointCase)
	}

	completed := make([]string, 0, len(cases))
	failed := make([]string, 0)
	manifestItems := make([]DatasetManifestItem, 0, len(cases))
	for _, c := range cases {
		key := batchTrajectoryCaseKey(c)
		if stored, ok := checkpoint.Cases[key]; ok && stored.Status == batchTrajectoryCheckpointStatusCompleted {
			manifestItems = append(manifestItems, cloneBatchTrajectoryManifestItem(stored.Item))
			completed = append(completed, key)
			continue
		}

		var (
			item   DatasetManifestItem
			runErr error
		)
		for attempt := 0; attempt <= r.cfg.MaxRetries; attempt++ {
			var result *BatchTrajectoryCaseResult
			result, runErr = r.executor.ExecuteBatchTrajectoryCase(ctx, c)
			if runErr == nil {
				item = r.buildManifestItem(key, c, result)
				checkpoint.Cases[key] = batchTrajectoryCheckpointCase{
					Key:       key,
					Status:    batchTrajectoryCheckpointStatusCompleted,
					Attempts:  attempt + 1,
					Item:      item,
					UpdatedAt: timeutil.NowTime(),
				}
				if err := saveBatchTrajectoryCheckpoint(r.cfg.CheckpointPath, checkpoint); err != nil {
					return nil, err
				}
				manifestItems = append(manifestItems, cloneBatchTrajectoryManifestItem(item))
				completed = append(completed, key)
				break
			}
		}
		if runErr == nil {
			continue
		}
		checkpoint.Cases[key] = batchTrajectoryCheckpointCase{
			Key:       key,
			Status:    batchTrajectoryCheckpointStatusFailed,
			Attempts:  r.cfg.MaxRetries + 1,
			Error:     runErr.Error(),
			UpdatedAt: timeutil.NowTime(),
		}
		if err := saveBatchTrajectoryCheckpoint(r.cfg.CheckpointPath, checkpoint); err != nil {
			return nil, err
		}
		failed = append(failed, key)
	}

	manifest := DatasetManifest{
		Dataset: DatasetManifestMeta{
			Name:    r.cfg.DatasetName,
			Subject: r.cfg.DatasetSubject,
		},
		Defaults: DatasetManifestDefaults{
			RunKind:       r.cfg.DefaultRunKind,
			Profile:       r.cfg.DefaultProfile,
			Scheduler:     r.cfg.Scheduler,
			Scoring:       r.cfg.Scoring,
			RuntimePolicy: cloneMetadataMap(r.cfg.RuntimePolicy),
		},
		Items: manifestItems,
	}
	manifestRaw, err := datasetManifestMap(manifest)
	if err != nil {
		return nil, err
	}
	versionSpec := DatasetVersionSpec{
		Version:    r.cfg.DatasetVersion,
		SourceType: r.cfg.SourceType,
		SourceRef:  r.cfg.SourceRef,
		Manifest:   manifestRaw,
		Metadata: map[string]interface{}{
			"candidate_id":         r.cfg.CandidateID,
			"case_count":           len(cases),
			"completed_case_count": len(completed),
			"failed_case_count":    len(failed),
			"source_type":          r.cfg.SourceType,
			"source_ref":           r.cfg.SourceRef,
			"checkpoint_path":      r.cfg.CheckpointPath,
		},
		CreatedBy: r.cfg.CreatedBy,
	}
	return &BatchTrajectoryRunResult{
		Manifest:           manifest,
		DatasetVersionSpec: versionSpec,
		CompletedCaseKeys:  completed,
		FailedCaseKeys:     failed,
		CheckpointPath:     r.cfg.CheckpointPath,
	}, nil
}

func (r *BatchTrajectoryRunner) buildManifestItem(key string, c BatchTrajectoryCase, result *BatchTrajectoryCaseResult) DatasetManifestItem {
	metadata := cloneMetadataMap(c.Metadata)
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	metadata = mergeMetadataMaps(metadata, cloneMetadataMap(result.Metadata))
	if r.cfg.CandidateID != "" {
		metadata["candidate_id"] = r.cfg.CandidateID
	}
	if strings.TrimSpace(result.TraceSummary) != "" {
		metadata["trajectory_summary"] = strings.TrimSpace(result.TraceSummary)
	}
	provenance := cloneMetadataMap(result.Provenance)
	if provenance == nil {
		provenance = map[string]interface{}{}
	}
	provenance["source_type"] = r.cfg.SourceType
	provenance["source_ref"] = r.cfg.SourceRef
	provenance["case_key"] = key
	metadata["trajectory_provenance"] = provenance
	input := cloneMetadataMap(result.Input)
	if input == nil {
		input = cloneMetadataMap(c.Input)
	}
	if input == nil {
		input = map[string]interface{}{}
	}
	if _, ok := input["goal"]; !ok && strings.TrimSpace(c.Prompt) != "" {
		input["goal"] = strings.TrimSpace(c.Prompt)
	}
	return DatasetManifestItem{
		ID:       key,
		RunKind:  r.cfg.DefaultRunKind,
		Profile:  r.cfg.DefaultProfile,
		Input:    input,
		Expected: cloneMetadataMap(result.Expected),
		Metadata: metadata,
	}
}

func batchTrajectoryCaseKey(c BatchTrajectoryCase) string {
	if key := strings.TrimSpace(c.Key); key != "" {
		return key
	}
	payload := map[string]interface{}{
		"prompt":   strings.TrimSpace(c.Prompt),
		"input":    cloneMetadataMap(c.Input),
		"metadata": cloneMetadataMap(c.Metadata),
	}
	sum := sha256.Sum256([]byte(marshalMetadata(payload)))
	return "case-" + hex.EncodeToString(sum[:8])
}

func loadBatchTrajectoryCheckpoint(path string) (*batchTrajectoryCheckpoint, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("batch trajectory checkpoint path is required")
	}
	blob, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var checkpoint batchTrajectoryCheckpoint
	if err := json.Unmarshal(blob, &checkpoint); err != nil {
		return nil, err
	}
	if checkpoint.Cases == nil {
		checkpoint.Cases = make(map[string]batchTrajectoryCheckpointCase)
	}
	return &checkpoint, nil
}

func saveBatchTrajectoryCheckpoint(path string, checkpoint *batchTrajectoryCheckpoint) error {
	if checkpoint == nil {
		return fmt.Errorf("batch trajectory checkpoint is required")
	}
	checkpoint.Version = batchTrajectoryCheckpointVersion
	checkpoint.UpdatedAt = timeutil.NowTime()
	if checkpoint.Cases == nil {
		checkpoint.Cases = make(map[string]batchTrajectoryCheckpointCase)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	blob, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		return err
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, blob, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func cloneBatchTrajectoryManifestItem(item DatasetManifestItem) DatasetManifestItem {
	return DatasetManifestItem{
		ID:       item.ID,
		RunKind:  item.RunKind,
		Profile:  item.Profile,
		Input:    cloneMetadataMap(item.Input),
		Expected: cloneMetadataMap(item.Expected),
		Metadata: cloneMetadataMap(item.Metadata),
	}
}
