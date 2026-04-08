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
- the bootstrap runtime bundle and binder layer live in `server/internal/bootstrap/runtime_bindings.go`
- `server/internal/bootstrap/routes.go` now stays focused on dependency assembly and route registration
- subagent execution, compat handlers, and research-driver wiring are bound through the bootstrap runtime bundle
- run preflight and stage semantics are normalized in `server/internal/harness/runtime_pipeline.go`

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
- make evidence-backed skill and instruction evolution first-class
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

## Relationship To Evolution

Evolution should not be modeled as a parallel system next to Harness.
It should be the review and adoption layer built on top of Harness V3 evidence.

The practical evidence chain is:

- `EvalRun` materializes a candidate attempt
- `ComparisonReport` turns baseline-vs-candidate results into regressions, improvements, and scorer deltas
- `SkillEvolutionCase` captures why a skill now needs review or optimization
- `SkillRevision` stores the concrete candidate, accepted, promoted, and backup snapshots used for operator decisions
- decision history provides an auditable record of promote and rollback actions
- self-reflect proposals extend the same review loop to instruction-level takeaways and patch previews

Runtime-origin intake is now part of this chain as well:

- terminal Harness runs that selected a canonical built-in skill can emit runtime skill evolution triggers directly
- failed or aborted runs enter the chain as `runtime_failure` fix cases
- successful runs enter the chain as `runtime_capture` cases only after repeated grounded lessons cross the capture threshold
- runtime-origin cases carry source run identity plus runtime evidence such as duration, token usage, quality or verification hints, and compact event summaries
- follow-up eval and gate handling remain the only authority for moving a candidate from `candidate_created` to `accepted` or `rejected`

### SkillEvolutionCase

Represents an actionable evolution issue or capture opportunity attached to a skill.

Suggested fields:

- `id`
- `skill_id`
- `owner_user_id`
- `mode`
- `reason`
- `source_kind`
- `source_id`
- `candidate_id`
- `base_content_sha256`
- `failure_signature`
- `dedup_key`
- `summary`
- `evidence_json`
- `revision_id`
- `status`
- `skipped_reason`
- `created_at`
- `updated_at`

Current reason values already in code:

- `runtime_failure`
- `runtime_capture`
- `selector_gate_failed`
- `execution_gate_failed`
- `budget_gate_failed`
- `manual`

Current status values already in code:

- `open`
- `candidate_created`
- `accepted`
- `rejected`
- `promoted`
- `skipped`

Rules:

- a case may exist before any revision is created
- cases should be dedupable enough to avoid reopening the same failure as unlimited noise
- a case should preserve the source run, gate, or manual trigger that created it

### SkillRevision

Represents a concrete snapshot of candidate or adopted skill content.

Suggested fields:

- `id`
- `skill_id`
- `status`
- `source_path`
- `candidate_id`
- `base_content_sha256`
- `origin_case_id`
- `parent_revision_id`
- `backup_of_revision_id`
- `eval_run_id`
- `optimization_run_id`
- `followup_gate`
- `optimization_surface`
- `decision_action`
- `review_note`
- `reviewed_by`
- `decision_log_json`
- `content`
- `content_sha256`
- `created_at`
- `reviewed_at`
- `promoted_at`

Current status values already in code:

- `candidate`
- `accepted`
- `rejected`
- `promoted`
- `backup`

Rules:

- a revision should keep lineage back to the case, eval run, and base content snapshot that produced it
- promote should write reviewed content back to the canonical skill source path and create a backup revision
- rollback should restore from a backup-derived revision and still emit a new auditable promotion result instead of mutating history in place

### Self-Reflect Proposal

Represents instruction-level review work that lives adjacent to Harness but appears in the same operator evolution loop.

Suggested fields:

- `id`
- `owner_user_id`
- `source_kind`
- `source_id`
- `proposal_mode`
- `target_file`
- `target_section`
- `status`
- `dedup_key`
- `lesson`
- `when_to_apply`
- `evidence`
- `evidence_ids`
- `evaluation_summary`
- `calibration_summary`
- `patch_preview`
- `review_note`
- `created_at`
- `updated_at`
- `reviewed_at`

Current status values already in code:

- `pending`
- `approved`
- `rejected`

Rule:

- proposals are a sibling review plane, not a replacement for Harness objects; they should attach evidence and patch previews to instruction updates without overloading `EvalRun` or `ComparisonReport`

### Current Evolution Workflow

The current operator loop should read like this:

1. Run or compare a candidate through Harness
2. Let runtime failures, gate failures, captures, or manual review create a `SkillEvolutionCase`
3. Generate or inspect a `SkillRevision` linked back to the case, eval run, and candidate id
4. Review comparison evidence, revision content, and decision history before adoption
5. Promote the accepted revision, or roll back through the backup lineage if the promoted state proves wrong later
6. Review self-reflect proposals when the follow-up belongs in instructions or AGENTS-level operational guidance instead of the skill file itself

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
- `POST /api/v1/harness/dataset-bundles/import`
- `POST /api/v1/harness/dataset-bundles/preview-source`
- `POST /api/v1/harness/dataset-bundles/import-source`
- `POST /api/v1/harness/eval-specs`
- `GET /api/v1/harness/eval-specs`
- `GET /api/v1/harness/eval-specs/:id`
- `POST /api/v1/harness/eval-runs`
- `GET /api/v1/harness/eval-runs`
- `GET /api/v1/harness/eval-runs/:id`
- `GET /api/v1/harness/eval-runs/:id/report`
- `POST /api/v1/harness/eval-runs/:id/cancel`
- `POST /api/v1/harness/eval-runs/:id/compare`
- `GET /api/v1/harness/comparison-reports/:id`
- `POST /api/v1/harness/baselines`
- `GET /api/v1/harness/baselines`
- `POST /api/v1/harness/skills/:skill_id/optimize`
- `GET /api/v1/harness/skills/:skill_id/revisions`
- `GET /api/v1/harness/skills/:skill_id/decision-history`
- `GET /api/v1/harness/skills/:skill_id/evolution-cases`
- `GET /api/v1/harness/skill-revisions/:id`
- `GET /api/v1/harness/skill-evolution-cases/:id`
- `POST /api/v1/harness/skill-revisions/:id/promote`
- `POST /api/v1/harness/skill-revisions/:id/rollback`
- `GET /api/v1/self-reflect/proposals`
- `GET /api/v1/self-reflect/proposals/:id`
- `GET /api/v1/self-reflect/proposals/:id/patch`
- `POST /api/v1/self-reflect/proposals/:id/approve`
- `POST /api/v1/self-reflect/proposals/:id/reject`

Compatibility rule:

- `RunGroup` remains visible and directly operable
- `EvalRun` becomes the preferred platform entrypoint for reproducible evaluations
- Evolution remains an evidence consumer built on Harness objects rather than a second execution runtime

## Dataset Bundle Authoring

Harness now supports a declarative dataset bundle format for both first-party and third-party authored datasets.

First-party bundles live in this repo under `harness/datasets/<bundle>/`.

Third-party bundles can live in separate GitHub repos as long as they keep the same layout:

```text
<bundle>/
  dataset.yaml
  versions/
    <version>/
      manifest.json
      eval-specs/
        *.yaml
```

`dataset.yaml` carries stable bundle metadata plus the default version to import.
`manifest.json` uses the existing `DatasetManifest` wire shape.
`eval-specs/*.yaml` are reusable eval templates bound to that imported dataset version.

V1 remains manual-only:

```bash
blue harness dataset import --path harness/datasets/demo-bundle

blue harness dataset import \
  --path harness/datasets/demo-bundle \
  --version v1 \
  --owner <user-id>

blue harness dataset pull \
  --source https://github.com/example/harness-datasets/tree/main/demo-bundle \
  --version v1 \
  --owner <user-id>

blue harness dataset pull \
  --source https://github.com/example/harness-datasets \
  --bundle-path harness/datasets/demo-bundle
```

The web UI now exposes the same manual flow from `Automation -> Harness -> Datasets & versions -> Import bundle`.

The UI supports:

- importing a local repo bundle
- importing a third-party GitHub bundle
- previewing the selected version before import
- showing case count, manifest hash, and bundled eval specs before anything is written

The preview/import source APIs are:

- `POST /api/v1/harness/dataset-bundles/preview-source`
- `POST /api/v1/harness/dataset-bundles/import-source`

For GitHub-hosted bundles, declare the eval spec filenames under `dataset.yaml` `versions.<version>.eval_specs` so the CLI can fetch exact files over the project-standard 4-source raw GitHub fallback chain:

1. `raw.githubusercontent.com`
2. `raw.gitmirror.com`
3. `cdn.jsdelivr.net/gh`
4. `ghproxy.com` proxying raw GitHub

Imported bundles are frozen into ordinary Harness `DatasetVersion` and `EvalSpec` records, so the existing dataset, eval-spec, and eval-run APIs keep working unchanged after import.

## Selector Gate Workflow

The first release gate that now exists end-to-end is the curated selector gate.

What it covers:

- a versioned built-in selector dataset with explicit critical cases
- dry-run route assertions for `selected_tools`, canonical skill id, clarify behavior, and fallback reasons
- multilingual routing coverage aligned with the 27-locale routing-cue catalog
- baseline and candidate comparison metrics such as route agreement, route compatibility, critical regressions, and clarify-rate delta
- locale and primary-route drift breakdowns in selector-gate reports so a passing aggregate does not hide regressions in one language or skill lane

## Cutover Readiness Workflow

Tool-to-skill cutover cannot be judged from one passing selector run or one passing execution run.
Blue now needs a candidate-scoped readiness view that answers "is this candidate actually safe to cut over?"

The current readiness workflow is:

- attach a stable `candidate_id` to selector and batch-1 execution eval runs
- evaluate selector, execution, and budget gates per candidate rather than per ad hoc title
- require consecutive green runs on all evaluated lanes before a candidate is considered gate-ready
- derive budget checks from selector dry-run `selected_tool_surface` snapshots so the same curated cases cover both routing correctness and first-turn tool-surface shrinkage

The resulting readiness report should separate:

- `evaluated_gates_ready`: selector, execution, and budget lanes all satisfied the required consecutive green count
- `ready`: all mandatory release requirements are satisfied with no remaining blocking reasons

This distinction matters because migration safety is correctness-first:

- a candidate with two green behavior lanes but a red budget lane is still not cutover-ready
- a candidate with one recent regression on either lane must have its consecutive counter reset even if earlier runs passed

Relevant APIs:

- `POST /api/v1/harness/selector-curated/ensure`
- `POST /api/v1/harness/eval-runs`
- `GET /api/v1/harness/eval-runs/:id`
- `GET /api/v1/harness/eval-runs/:id/report`
- `POST /api/v1/harness/eval-runs/:id/budget-gate`
- `POST /api/v1/harness/eval-runs/:id/selector-gate`
- `POST /api/v1/harness/cutover-readiness`
- `GET /api/v1/harness/comparison-reports/:id`

Default gate policy is intentionally truth-first rather than baseline-first:

- curated pass rate must be at least `0.98`
- critical curated pass rate must be exactly `1.0`
- route agreement, route compatibility, and clarify delta are always reported, but only fail the gate when thresholds are supplied

Recommended release gate thresholds for cutover candidates:

- `route_compatible_rate >= 0.95`
- `clarify_rate_delta <= 0.01`

This lets Blue reject real selector regressions without blocking improvements that intentionally disagree with a bad baseline.

CLI automation is now available:

```bash
blue harness selector ensure --owner release-gate

blue harness budget gate <selector-eval-run-id> \
  --baseline-id <baseline-id> \
  --min-median-schema-byte-reduction-rate 0.80 \
  --max-median-latency-increase-rate 0.10 \
  --allowed-final-native-tools exec

blue harness selector verify \
  --owner release-gate \
  --title candidate-2026-03-28 \
  --baseline-id <baseline-id>

blue harness selector gate <eval-run-id> \
  --baseline-id <baseline-id> \
  --min-route-compatible-rate 0.95 \
  --max-clarify-rate-delta 0.01
```

Script automation is also available for CI and nightly jobs:

```bash
python3 scripts/selector_gate_runner.py \
  --blue-base-url http://127.0.0.1:18080/api/v1 \
  --owner release-gate \
  --candidate-id batch-1 \
  --candidate-label batch-1 \
  --baseline-id <baseline-id> \
  --min-route-compatible-rate 0.95 \
  --max-clarify-rate-delta 0.01 \
  --output-json docs/reports/selector_gate_report.json \
  --output-md docs/reports/selector_gate_report.md

python3 scripts/budget_gate_runner.py \
  --blue-base-url http://127.0.0.1:18080/api/v1 \
  --owner release-gate \
  --candidate-id batch-1 \
  --candidate-label batch-1 \
  --baseline-id <baseline-id> \
  --min-median-schema-byte-reduction-rate 0.80 \
  --max-median-latency-increase-rate 0.10 \
  --allowed-final-native-tools exec \
  --output-json docs/reports/budget_gate_report.json \
  --output-md docs/reports/budget_gate_report.md

# Reuse the selector eval run when you want one candidate attempt to produce
# one selector trace and one budget assessment without launching a second
# selector dry-run.
python3 scripts/budget_gate_runner.py \
  --blue-base-url http://127.0.0.1:18080/api/v1 \
  --owner release-gate \
  --candidate-id batch-1 \
  --eval-run-id <selector-eval-run-id> \
  --baseline-id <baseline-id> \
  --min-median-schema-byte-reduction-rate 0.80 \
  --max-median-latency-increase-rate 0.10 \
  --allowed-final-native-tools exec \
  --output-json docs/reports/budget_gate_report.json \
  --output-md docs/reports/budget_gate_report.md

python3 scripts/budget_gate_history_report.py \
  --reports "docs/reports/budget_gate_report*.json" \
  --candidate-id batch-1 \
  --candidate-label batch-1 \
  --require-consecutive-green 2 \
  --output-json docs/reports/budget_gate_history_report.json \
  --output-md docs/reports/budget_gate_history_report.md

python3 scripts/cutover_readiness_runner.py \
  --blue-base-url http://127.0.0.1:18080/api/v1 \
  --owner release-gate \
  --candidate-id batch-1 \
  --selector-baseline-id <selector-baseline-id> \
  --execution-baseline-id <execution-baseline-id> \
  --budget-baseline-id <budget-baseline-id> \
  --min-route-compatible-rate 0.95 \
  --max-clarify-rate-delta 0.01 \
  --max-pass-rate-drop 0.01 \
  --max-verification-pass-rate-drop 0 \
  --max-evidence-backed-pass-rate-drop 0 \
  --min-median-schema-byte-reduction-rate 0.80 \
  --max-median-latency-increase-rate 0.10 \
  --allowed-final-native-tools exec \
  --output-json docs/reports/cutover_readiness_report.json \
  --output-md docs/reports/cutover_readiness_report.md
```

To evaluate historical release readiness for one candidate, use the history summarizer:

```bash
python3 scripts/selector_gate_history_report.py \
  --reports "docs/reports/selector_gate_report*.json" \
  --candidate-id batch-1 \
  --candidate-label batch-1 \
  --require-consecutive-green 2 \
  --output-json docs/reports/selector_gate_history_report.json \
  --output-md docs/reports/selector_gate_history_report.md
```

The history summary now highlights recurring locale and primary-route drift hotspots for the focused candidate, so repeated regressions in one of the 27 routing locales are visible even when aggregate pass/fail streaks still look healthy.

For a single candidate-level verdict that matches the readiness API, use `scripts/cutover_readiness_runner.py` or [cutover-readiness.yml](/Users/orca/Documents/GitHub/ZimaOS-Blue/.github/workflows/cutover-readiness.yml). Unlike the per-lane history summaries, this report asks the server to evaluate selector, execution, and budget lanes together under one `candidate_id`.

For a one-command candidate attempt that runs selector, execution, budget, and readiness in sequence, use `scripts/cutover_candidate_pipeline.py` or [cutover-candidate-pipeline.yml](/Users/orca/Documents/GitHub/ZimaOS-Blue/.github/workflows/cutover-candidate-pipeline.yml). This is the preferred path for collecting real cutover evidence because it keeps one shared `candidate_id`, reuses the selector eval run for the budget gate, surfaces selector and execution drift in the single-run report, and now enforces top-level consecutive-green checks across complete candidate attempts instead of only looking at isolated lane reports.

Script automation is available for local or CI candidate attempts:

```bash
python3 scripts/cutover_candidate_pipeline.py \
  --blue-base-url http://127.0.0.1:18080/api/v1 \
  --owner release-gate \
  --candidate-id batch-1 \
  --candidate-label batch-1 \
  --selector-baseline-id <selector-baseline-id> \
  --execution-baseline-id <execution-baseline-id> \
  --budget-baseline-id <budget-baseline-id> \
  --output-dir docs/reports/cutover_candidate_pipeline
```

The same script can now enforce top-level candidate history locally when you want one command to answer "is this candidate actually at two complete greens yet?":

```bash
python3 scripts/cutover_candidate_pipeline.py \
  --blue-base-url http://127.0.0.1:18080/api/v1 \
  --owner release-gate \
  --candidate-id batch-1 \
  --candidate-label batch-1 \
  --selector-baseline-id <selector-baseline-id> \
  --execution-baseline-id <execution-baseline-id> \
  --budget-baseline-id <budget-baseline-id> \
  --require-consecutive-green 2 \
  --history-reports "docs/reports/cutover_candidate_pipeline_history_artifacts/downloaded/pipeline_reports/*.json" \
  --output-dir docs/reports/cutover_candidate_pipeline
```

When `--github-repo` is provided, `scripts/cutover_candidate_pipeline.py` can also download prior workflow artifacts before evaluating the local history gate, so the local command mirrors the GitHub workflow more closely. The aggregate `cutover_candidate_pipeline_report.json` and Markdown summary now include the evaluated history-gate snapshot as well, so one artifact can answer both "did this run pass?" and "does this candidate actually satisfy the consecutive-green rule?"

To evaluate whether complete candidate attempts have gone green often enough to cut over, use the top-level pipeline history tools:

```bash
python3 scripts/cutover_candidate_pipeline_artifact_fetcher.py \
  --repo your-org/your-repo \
  --workflow cutover-candidate-pipeline.yml \
  --branch main \
  --exclude-run-id 123456789 \
  --limit-runs 10 \
  --output-dir docs/reports/cutover_candidate_pipeline_history_artifacts/downloaded \
  --output-json docs/reports/cutover_candidate_pipeline_history_artifacts/download-manifest.json

python3 scripts/cutover_candidate_pipeline_history_report.py \
  --reports \
    "docs/reports/cutover_candidate_pipeline_history_artifacts/current/*.json" \
    "docs/reports/cutover_candidate_pipeline_history_artifacts/downloaded/pipeline_reports/*.json" \
  --candidate-id batch-1 \
  --candidate-label batch-1 \
  --require-consecutive-green 2 \
  --output-json docs/reports/cutover_candidate_pipeline_history_report.json \
  --output-md docs/reports/cutover_candidate_pipeline_history_report.md
```

The pipeline artifact fetcher keeps the raw extracted workflow artifacts, but it also stages only canonical cutover candidate pipeline reports into `downloaded/pipeline_reports/` so the history summarizer ignores prior history summaries and download manifests.

The top-level pipeline history summary now surfaces selector and execution locale or primary-route drift directly from each pipeline attempt, so a candidate does not look "green enough" just because the aggregate streak is healthy while one of the 27 locales is repeatedly regressing inside a nested lane report.

To pull prior workflow artifacts into a local history cache before summarizing, use the GitHub artifact fetcher:

```bash
python3 scripts/selector_gate_artifact_fetcher.py \
  --repo your-org/your-repo \
  --workflow selector-gate.yml \
  --branch main \
  --exclude-run-id 123456789 \
  --limit-runs 10 \
  --output-dir docs/reports/selector_gate_history_artifacts/downloaded \
  --output-json docs/reports/selector_gate_history_artifacts/download-manifest.json
```

The fetcher keeps the raw extracted artifacts, but also stages only selector-gate candidate reports into `downloaded/selector_reports/` so history checks do not accidentally ingest prior history summaries or download manifests as if they were eval results.

GitHub Actions automation is available in [selector-gate.yml](/Users/orca/Documents/GitHub/ZimaOS-Blue/.github/workflows/selector-gate.yml):

- `workflow_dispatch` accepts explicit `server_url`, baseline, and threshold inputs
- `candidate_id` is the authoritative streak/readiness key and should stay stable across reruns; when omitted it falls back to the resolved `candidate_label`
- `candidate_label` remains available as a human-readable display label and otherwise falls back to a stable branch-derived value
- nightly `schedule` can reuse repository secrets such as `BLUE_SELECTOR_GATE_SERVER_URL`, `BLUE_SELECTOR_GATE_BASELINE_ID`, and `BLUE_SELECTOR_GATE_API_KEY`
- the workflow defaults now enforce `route_compatible_rate >= 0.95` and `clarify_rate_delta <= 0.01`, while still reporting raw route agreement for diagnosis
- the workflow now downloads prior `selector-gate-report` artifacts from recent runs, computes a consecutive-green history summary, and enforces `require_consecutive_green` before cutover
- the workflow publishes both the single-run selector gate report and the history summary into the Actions step summary and uploads their JSON/Markdown artifacts
- the workflow intentionally skips cleanly when no comparison base is configured, instead of failing noisily on an unconfigured repo

GitHub Actions automation is also available in [cutover-candidate-pipeline.yml](/Users/orca/Documents/GitHub/ZimaOS-Blue/.github/workflows/cutover-candidate-pipeline.yml):

- `workflow_dispatch` accepts the server, owner, candidate/baseline identifiers, readiness knobs, and pipeline history controls while keeping threshold policy pinned to the default release gate values
- use `scripts/cutover_candidate_pipeline.py` when you need custom threshold experiments that would otherwise overflow GitHub's 25-input `workflow_dispatch` limit
- one run now produces both the aggregate single-attempt pipeline report and a candidate-scoped pipeline history summary
- the workflow downloads prior `cutover-candidate-pipeline-report` artifacts, stages canonical reports into a local history cache, and fails if the focused candidate has not reached the required consecutive-green count
- the history summary also pulls selector and execution locale or route drift hotspots up to the candidate level, so multilingual regressions remain visible during cutover review
- this makes the workflow the preferred evidence collection entrypoint when answering "can we switch from tool to skill yet?"

Recommended CI or nightly flow:

1. ensure the curated selector assets exist for the release-gate owner
2. run `blue harness selector verify` for the candidate
3. fail the build on a non-zero exit code
4. only add stricter compatibility thresholds like route agreement after the truth-based gate is stable

## Execution Equivalence Gate Workflow

The second release gate now in place is the batch-1 execution equivalence gate.

What it covers:

- a versioned built-in execution dataset for the first migration batch (`web_search`, `reminder`, and `analyze`)
- contract-backed verification for expected route, clarify behavior, and observation shape instead of only checking `status=completed`
- route-equivalence checks that catch `exec` traces which silently ran the wrong skill, such as `blue analyze` when the case expected `blue web_search`
- locale and primary-route drift breakdowns so regressions stay visible across the 27-locale routing matrix
- consecutive-green history summaries so old `SkillToolAdapter` fallback paths are not retired after one lucky run

Relevant APIs:

- `POST /api/v1/harness/execution-batch1/ensure`
- `POST /api/v1/harness/eval-runs`
- `GET /api/v1/harness/eval-runs/:id`
- `GET /api/v1/harness/eval-runs/:id/report`
- `POST /api/v1/harness/eval-runs/:id/execution-gate`
- `GET /api/v1/harness/comparison-reports/:id`

Default gate policy stays correctness-first:

- pass-rate drop must stay within `0.01`
- critical regression count must stay at `0`
- verification and evidence-backed pass-rate deltas are always reported, and can be promoted to hard thresholds for cutover candidates

Recommended release gate thresholds for cutover candidates:

- `max_pass_rate_drop <= 0.01`
- `max_critical_regressions = 0`
- `max_verification_pass_rate_drop <= 0`
- `max_evidence_backed_pass_rate_drop <= 0`

CLI automation is available:

```bash
blue harness execution ensure --owner release-gate

blue harness execution verify \
  --owner release-gate \
  --title candidate-2026-03-28 \
  --baseline-id <baseline-id> \
  --max-pass-rate-drop 0.01 \
  --max-critical-regressions 0 \
  --max-verification-pass-rate-drop 0 \
  --max-evidence-backed-pass-rate-drop 0

blue harness execution gate <eval-run-id> \
  --baseline-id <baseline-id> \
  --max-pass-rate-drop 0.01 \
  --max-critical-regressions 0 \
  --max-verification-pass-rate-drop 0 \
  --max-evidence-backed-pass-rate-drop 0
```

Script automation is also available for CI and nightly jobs:

```bash
python3 scripts/execution_gate_runner.py \
  --blue-base-url http://127.0.0.1:18080/api/v1 \
  --owner release-gate \
  --candidate-id batch-1 \
  --candidate-label batch-1 \
  --baseline-id <baseline-id> \
  --max-pass-rate-drop 0.01 \
  --max-critical-regressions 0 \
  --max-verification-pass-rate-drop 0 \
  --max-evidence-backed-pass-rate-drop 0 \
  --output-json docs/reports/execution_gate_report.json \
  --output-md docs/reports/execution_gate_report.md
```

To evaluate historical release readiness for one candidate, use the history summarizer:

```bash
python3 scripts/execution_gate_history_report.py \
  --reports "docs/reports/execution_gate_report*.json" \
  --candidate-id batch-1 \
  --candidate-label batch-1 \
  --require-consecutive-green 2 \
  --output-json docs/reports/execution_gate_history_report.json \
  --output-md docs/reports/execution_gate_history_report.md
```

The history summary highlights recurring locale and primary-route drift hotspots, so repeated execution regressions in one route lane remain visible even when aggregate pass rate still looks acceptable.

To pull prior workflow artifacts into a local history cache before summarizing, use the GitHub artifact fetcher:

```bash
python3 scripts/execution_gate_artifact_fetcher.py \
  --repo your-org/your-repo \
  --workflow execution-gate.yml \
  --branch main \
  --exclude-run-id 123456789 \
  --limit-runs 10 \
  --output-dir docs/reports/execution_gate_history_artifacts/downloaded \
  --output-json docs/reports/execution_gate_history_artifacts/download-manifest.json
```

The fetcher keeps the raw extracted artifacts, but also stages only execution-gate candidate reports into `downloaded/execution_reports/` so history checks do not accidentally ingest prior history summaries or download manifests as if they were eval results.

GitHub Actions automation is available in [execution-gate.yml](/Users/orca/Documents/GitHub/ZimaOS-Blue/.github/workflows/execution-gate.yml):

- `workflow_dispatch` accepts explicit `server_url`, baseline, and threshold inputs
- `candidate_id` is the authoritative streak/readiness key and should stay stable across reruns; when omitted it falls back to the resolved `candidate_label`
- `candidate_label` remains available as a human-readable display label and otherwise falls back to a stable branch-derived value
- nightly `schedule` can reuse repository secrets such as `BLUE_EXECUTION_GATE_SERVER_URL`, `BLUE_EXECUTION_GATE_BASELINE_ID`, and `BLUE_EXECUTION_GATE_API_KEY`
- the workflow defaults enforce `max_pass_rate_drop <= 0.01`, `max_critical_regressions = 0`, `max_verification_pass_rate_drop <= 0`, and `max_evidence_backed_pass_rate_drop <= 0`
- the workflow downloads prior `execution-gate-report` artifacts from recent runs, computes a consecutive-green history summary, and enforces `require_consecutive_green` before cutover
- the workflow publishes both the single-run execution gate report and the history summary into the Actions step summary and uploads their JSON/Markdown artifacts
- the workflow intentionally skips cleanly when no comparison base is configured, instead of failing noisily on an unconfigured repo

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
