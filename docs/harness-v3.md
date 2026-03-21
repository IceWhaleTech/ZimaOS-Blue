# Harness V3

## Positioning

Harness V2 is already a real runtime control plane in Blue, not a mock layer:

- single-run execution is persisted in `Run`
- batch and evaluation execution is persisted in `RunGroup`, `RunGroupItem`, and `Scorecard`
- agent task, deep research, and subagent execution already flow through Harness-compatible paths
- the in-process dispatcher already schedules queued group items and scores terminal runs

That means V3 should not be another rename. V3 should be the point where Harness becomes a reusable evaluation platform, with stable dataset, spec, baseline, comparison, and scorer-extension models built on top of the current V2 runtime.

## What Exists Today

The current codebase already proves that Harness is active:

- generic Harness APIs exist in `server/internal/harness/handler.go`
- `Run`, `RunGroup`, `RunGroupItem`, and `Scorecard` are defined in `server/internal/harness/types.go`
- SQLite persistence for runs, groups, items, events, artifacts, and scorecards exists in `server/internal/harness/store.go`
- the group scheduler is active in `server/internal/harness/group_dispatcher.go`
- group scoring is active in `server/internal/harness/scoring.go`
- agent tasks are projected through Harness in `server/internal/harness/agent_compat_handler.go`
- deep research HTTP creation is routed through Harness in `server/internal/bootstrap/research_harness_adapter.go`
- tool-side `research_run` creation is routed through Harness in `server/internal/bootstrap/research_tool_adapter.go`
- deep research job events, calibration metadata, and takeaway candidates are synced back into Harness in `server/internal/harness/drivers/research.go`
- the dispatcher and routes are wired from `server/internal/bootstrap/routes.go`
- subagent execution is injected into the agent runner from `server/internal/bootstrap/routes.go`

In short: V2 already gives Blue a durable execution ledger and a minimal evaluation loop.

## Why V2 Is Not V3 Yet

V2 is strong as an execution substrate, but it is still group-centric rather than platform-centric.

Current gaps:

- no reusable dataset registry
- no frozen dataset versioning or manifest import/export lifecycle
- no first-class eval spec object that can be rerun independently of one ad hoc group
- no baseline/candidate comparison model
- no regression report model for "what got better or worse"
- no scorer plugin contract beyond the built-in heuristic pipeline
- no worker-pool abstraction beyond the in-process dispatcher
- no lineage model across repeated experiments, nightly runs, or release gates

So V2 solves execution and persistence, but V3 should solve repeatability, comparison, and extension.

## V3 Goals

Harness V3 should keep `Run` as the execution primitive and `RunGroup` as the execution container, while adding a reusable evaluation layer above them.

V3 goals:

- make datasets reusable and versioned
- make eval definitions reusable and auditable
- make reruns and comparisons first-class
- make scorer composition pluggable
- make scheduling ready for multi-worker execution without replacing SQLite on day one
- make release gating and regression analysis possible from persisted platform objects

## Stable V3 Objects

V3 should add these new objects while keeping V2 models intact.

### Dataset

Represents a named collection of cases.

Suggested fields:

- `id`
- `name`
- `description`
- `owner_user_id`
- `subject`
- `default_run_kind`
- `default_profile`
- `active_version_id`
- `metadata_json`
- `created_at`
- `updated_at`

### DatasetVersion

Represents an immutable snapshot of dataset content.

Suggested fields:

- `id`
- `dataset_id`
- `version`
- `manifest_sha256`
- `item_count`
- `source_type`
- `source_ref`
- `manifest_json`
- `created_by`
- `created_at`

Rule:

- every eval must bind to a concrete `dataset_version_id`, never to a floating dataset head

### EvalSpec

Represents a reusable evaluation template.

Suggested fields:

- `id`
- `name`
- `subject`
- `run_kind`
- `profile`
- `dataset_id`
- `dataset_version_id`
- `scheduler_config_json`
- `scoring_config_json`
- `runtime_policy_json`
- `metadata_json`
- `created_at`
- `updated_at`

Rule:

- `EvalSpec` is the reusable template
- `RunGroup` remains the materialized execution instance

### EvalRun

Represents one execution of an `EvalSpec`.

Suggested fields:

- `id`
- `eval_spec_id`
- `group_id`
- `dataset_version_id`
- `baseline_eval_run_id`
- `status`
- `trigger_kind`
- `trigger_ref`
- `summary_json`
- `created_at`
- `updated_at`
- `started_at`
- `finished_at`

Rule:

- every `EvalRun` owns exactly one `RunGroup`
- `RunGroup` remains queryable as today for compatibility

### Baseline

Represents a pinned reference result for comparison.

Suggested fields:

- `id`
- `name`
- `subject`
- `eval_spec_id`
- `eval_run_id`
- `is_default`
- `metadata_json`
- `created_at`
- `updated_at`

### ComparisonReport

Represents a persisted diff between two eval runs.

Suggested fields:

- `id`
- `eval_spec_id`
- `base_eval_run_id`
- `target_eval_run_id`
- `summary_json`
- `regressions_json`
- `improvements_json`
- `scorer_delta_json`
- `created_at`

## Relationship To Existing V2 Objects

V3 should extend V2 rather than replace it.

- `Run` stays the only execution primitive
- `RunGroup` stays the batch container and scheduler target
- `RunGroupItem` stays the case-attempt holder
- `Scorecard` stays the per-attempt scoring record
- `EvalRun` points to a `RunGroup`
- `DatasetVersion` is expanded into `RunGroupItem` rows at materialization time

Recommended mapping:

- `DatasetVersion` -> reusable source snapshot
- `EvalSpec` -> reusable eval template
- `EvalRun` -> concrete execution request
- `RunGroup` -> actual batch run created by `EvalRun`
- `RunGroupItem` -> concrete case attempts
- `Run` -> actual runtime unit
- `Scorecard` -> per-item scoring result
- `ComparisonReport` -> post-run regression analysis

## API Shape

V3 should add a new platform API surface while preserving current V2 routes.

Keep as-is:

- `POST /api/v1/harness/runs`
- `GET /api/v1/harness/runs*`
- `POST /api/v1/harness/groups`
- `GET /api/v1/harness/groups*`
- `/api/v1/agent/tasks/*`
- deep research APIs that already project into Harness

Add:

- `POST /api/v1/harness/datasets`
- `GET /api/v1/harness/datasets`
- `GET /api/v1/harness/datasets/:id`
- `POST /api/v1/harness/datasets/:id/versions`
- `GET /api/v1/harness/datasets/:id/versions`
- `GET /api/v1/harness/dataset-versions/:id`
- `POST /api/v1/harness/eval-specs`
- `GET /api/v1/harness/eval-specs`
- `GET /api/v1/harness/eval-specs/:id`
- `POST /api/v1/harness/eval-runs`
- `GET /api/v1/harness/eval-runs`
- `GET /api/v1/harness/eval-runs/:id`
- `GET /api/v1/harness/eval-runs/:id/report`
- `POST /api/v1/harness/eval-runs/:id/cancel`
- `POST /api/v1/harness/eval-runs/:id/compare`
- `POST /api/v1/harness/baselines`
- `GET /api/v1/harness/baselines`

Compatibility rule:

- `RunGroup` remains visible and directly operable
- `EvalRun` becomes the preferred platform entrypoint for reproducible evaluations

## Execution Model

V3 should keep the V2 dispatcher path for the first migration phase, then make the dispatcher replaceable.

### Phase 1

- continue using the current in-process dispatcher
- materialize `EvalRun` into one `RunGroup`
- materialize dataset-version cases into `RunGroupItem`
- keep item claim and lease behavior in SQLite
- keep run creation through existing `Controller.Submit`

### Phase 2

- introduce a `Worker` abstraction
- move dispatcher logic behind an interface
- keep DB lease as the source of truth
- allow multiple process workers to claim from the same store

Suggested worker contract:

```go
type Worker interface {
	ID() string
	CanRun(kind RunKind, profile string) bool
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}
```

Important rule:

- V3 should not convert the run tree into a DAG scheduler
- the group/eval layer coordinates batches
- the run layer keeps parent/child execution lineage

## Scoring Architecture

V2 scoring is already useful, but it is still hardcoded and heuristic-heavy. V3 should keep the current rule-ground-truth-judge ordering while making the scorer chain pluggable.

Suggested scorer interfaces:

```go
type Scorer interface {
	Name() string
	Supports(caseProfile string, runKind RunKind) bool
	Score(ctx context.Context, input ScoreInput) (*ScoreResult, error)
}

type ScorePipeline interface {
	Score(ctx context.Context, input ScoreInput) (*Scorecard, error)
}
```

Suggested `ScoreInput`:

- group metadata
- item profile
- expected payload
- latest run
- run events
- run artifacts
- prior attempt scorecards

Suggested V3 scorer families:

- rule scorer
- ground-truth comparer
- rubric scorer
- judge scorer
- safety policy scorer
- tool-trace scorer
- artifact verifier

Rules:

- deterministic scorers run first
- judge scorers are fallback or augmentation, not the only truth source
- `Scorecard` remains the persisted result boundary
- comparison and regression should consume scorecards, not rewrite runs

## Dataset And Manifest Format

V3 should introduce a stable manifest format instead of storing every experiment as ad hoc `RunGroupSpec` JSON.

Suggested manifest shape:

```json
{
  "dataset": {
    "name": "agent-task-smoke",
    "subject": "agent_task"
  },
  "defaults": {
    "run_kind": "agent_task",
    "profile": "general",
    "scoring": {
      "mode": "hybrid",
      "pass_threshold": 0.7
    }
  },
  "items": [
    {
      "id": "case-001",
      "input": {
        "goal": "Summarize the latest backup status"
      },
      "expected": {
        "contains": ["backup"],
        "status": "completed"
      },
      "metadata": {
        "tier": "smoke"
      }
    }
  ]
}
```

Materialization rule:

- importing a manifest creates `Dataset` and `DatasetVersion`
- starting an eval expands the bound dataset version into `RunGroupItem` rows

## Comparison And Regression

This is the biggest gap between V2 and V3.

V3 should support:

- compare two `EvalRun`s from the same `EvalSpec`
- compare candidate vs baseline by verdict delta
- compare scorer breakdown delta
- compare failure clusters by profile, tool, model, or runtime path

Suggested regression output:

- overall score delta
- pass rate delta
- new failures
- resolved failures
- unstable cases
- scorer disagreement cases
- linked run and artifact evidence

`ComparisonReport` should be stored, not only rendered on demand.

## Migration From V2

V3 should ship incrementally.

### Step 1

- add new SQLite tables for `datasets`, `dataset_versions`, `eval_specs`, `eval_runs`, `baselines`, and `comparison_reports`
- do not rewrite existing V2 tables

### Step 2

- add manifest import/export helpers
- add dataset-version materialization into `RunGroupSpec`

### Step 3

- add `EvalSpec` and `EvalRun` manager APIs
- create `EvalRun` by generating a `RunGroup`

### Step 4

- move scoring behind a scorer registry while preserving current V2 behavior as the default pipeline

### Step 5

- add comparison report generation
- expose baseline pinning and release-gate checks

### Step 6

- add optional multi-worker execution
- keep SQLite lease semantics unless real throughput demands a new store

## Minimum Success Criteria For V3

Harness V3 is successful only if Blue can do all of the following using persisted platform objects:

- rerun the same frozen dataset against the same eval spec
- compare the result against a named baseline
- explain which cases regressed and why
- attribute regressions to runtime output, tool behavior, or scorer differences
- keep all results traceable back to `Run`, `RunEvent`, `ArtifactRef`, and `Scorecard`

If a design does not improve repeatability or comparison, it is still V2.x, not V3.

## Recommended First Implementation Slice

The safest V3-first slice on top of the current codebase is:

1. add `Dataset`, `DatasetVersion`, `EvalSpec`, and `EvalRun` models plus SQLite persistence
2. materialize `EvalRun` into existing `RunGroup` and `RunGroupItem`
3. keep the current dispatcher and scoring pipeline unchanged
4. add `ComparisonReport` after repeated eval runs are available

This order preserves the already-working V2 runtime while unlocking real platform leverage.
