# PinchBench Samples

This bundle adapts a small deterministic subset of official PinchBench task ideas into Harness dataset cases.

## Source Mapping

- `daily_briefing`
  - Adapted from PinchBench `task_15_daily_summary`
  - Reduced to prompt-only agenda synthesis plus an expected markdown artifact
- `memory_recall`
  - Adapted from PinchBench `task_22_second_brain`
  - Reduced to single-run memory file persistence plus immediate deterministic recall
- `memory_guard`
  - Inspired by the same memory-oriented workflow as `task_22_second_brain`
  - Adds conflicting `harness_memory_seed` so Harness can verify prompt-grounded memory handling

## Deliberate Simplifications

- No live web or external research dependencies
- No email or calendar fixture injection
- No true multi-session replay
- No pre-seeded workspace tree or `workspace_files` manifest support

## Why Some PinchBench Shapes Are Deferred

The current Harness dataset bundle format can carry prompts, expected artifacts, scoring hints, and memory seed metadata, but it does not yet provide a first-class way to inject per-case workspace file trees, email fixtures, calendar fixtures, or shared multi-session task state.

That means v1 focuses on cases that remain useful and repeatable with current capabilities:

- prompt-only summarization and briefing
- single-run memory persistence and recall
- memory conflict detection via `harness_memory_seed`

If this bundle proves useful, later versions can split these families into dedicated bundles or promote them into built-in curated datasets after the missing fixture-injection paths exist.
