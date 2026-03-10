# Autoresearch Integration

Blue deep research supports `route_mode=auto|web|experiment|hybrid`, and can call an external experiment backend (for example `karpathy/autoresearch`) through a command wrapper.

## Route mode decision

`route_mode=auto` is **not** free-picked by the LLM. Blue resolves route mode with deterministic logic:

1. Query cue classifier (`web|experiment|hybrid`)
2. Policy gates (`research.router.*`)
3. Backend availability checks
4. Graceful fallback to `web` when experiment backend is unavailable/disabled

## Enable backend

```yaml
research:
  router:
    default_mode: web
    allow_experiment: true
    allow_hybrid: true
  autoresearch:
    enabled: true
    command: bash
    args:
      - docs/examples/autoresearch-wrapper.sh
    working_dir: .
    timeout: 20m
    artifact_dir: ./data/deep-research/experiments
```

## Environment overrides

You can override routing/backend behavior at runtime with environment variables:

- `BLUE_RESEARCH_DEFAULT_ROUTE_MODE`
- `BLUE_RESEARCH_ALLOW_EXPERIMENT`
- `BLUE_RESEARCH_ALLOW_HYBRID`
- `BLUE_AUTORESEARCH_ENABLED`
- `BLUE_AUTORESEARCH_COMMAND`
- `BLUE_AUTORESEARCH_WORKING_DIR`
- `BLUE_AUTORESEARCH_ARGS` (JSON array preferred, comma-separated fallback)
- `BLUE_AUTORESEARCH_ARTIFACT_DIR`
- `BLUE_AUTORESEARCH_ENV_JSON` (JSON object map)
- `BLUE_AUTORESEARCH_TIMEOUT`

## Prepare local `karpathy/autoresearch`

```bash
git clone https://github.com/karpathy/autoresearch.git
cd autoresearch
uv sync
```

Recommended environment variables for the wrapper:

```bash
export AUTORESEARCH_REPO=/absolute/path/to/autoresearch
export AUTORESEARCH_RUN_CMD='uv run train.py'
# Optional one-time or pre-run command:
# export AUTORESEARCH_PREPARE_CMD='uv run prepare.py'
```

The sample wrapper in `docs/examples/autoresearch-wrapper.sh`:

- reads Blue JSON request from `stdin`
- writes request/log artifacts into `artifact_dir`
- executes `AUTORESEARCH_RUN_CMD` (default `uv run train.py`)
- parses metrics from training logs (`val_bpb`, `num_steps`, `total_seconds`, `peak_vram_mb`, `mfu_percent`)
- prints normalized JSON result to `stdout`

`karpathy/autoresearch` does not expose a stable non-interactive "agent loop" CLI contract. The wrapper therefore uses a deterministic command execution model by default, while still allowing custom orchestration through `AUTORESEARCH_RUN_CMD`.

## Smoke check wrapper contract

Run local smoke verification:

```bash
bash server/tools/verify_autoresearch_wrapper_smoke.sh
```

This script builds a temporary mock autoresearch repo, invokes the wrapper, and validates:

- output JSON contract fields
- metric parsing (`val_bpb`, `num_steps`, etc.)
- environment propagation (`BLUE_RESEARCH_QUERY/MODE/ROUTE_MODE`)
- generated artifacts (`request_payload`, `train_log`)

## Input contract (stdin)

Blue sends JSON to backend command with fields such as:

- `job_id`
- `query`
- `mode`
- `route_mode`
- `lang`
- `budget`
- `strict_entity`
- `time_windows`
- `report_style`
- `artifact_dir`
- `web_report` and `web_evidence` (for `hybrid`)

## Output contract (stdout)

The backend command should return JSON:

```json
{
  "summary": "...",
  "confidence": 0.8,
  "findings": ["..."],
  "artifacts": [{"label":"...","kind":"file|directory","path":"..."}],
  "open_questions": ["..."],
  "metadata": {"key":"value"}
}
```

If plain text is returned instead of JSON, Blue treats it as `summary`.

## Agent-loop tools

Blue now exposes native tools:

- `research_run`: create/run a deep-research job (`route_mode` supported)
- `research_status`: fetch status/report by `job_id`
