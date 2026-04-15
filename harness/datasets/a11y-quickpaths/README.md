# A11y Quick Paths

This bundle evaluates 24 deterministic desktop a11y quick paths on real macOS and Windows hosts.

## Scope

- IM / chat delivery
- System settings toggle
- Browser native chrome control
- File manager selection
- Mail compose entry point
- Window focus
- Browser address typing
- Settings and file-manager scrolling

Each case asks the agent to return only the raw JSON from the final `blue a11y ...` call so Harness can score `target_hit` and `verification_passed` directly from `run.Result`.

## Import

```bash
blue harness dataset import --path harness/datasets/a11y-quickpaths
```

## Runtime Notes

- Configure the target provider through Settings UI or runtime config before the eval run.
- Use the `114.xxx` provider family during validation.
- Use model `claude-sonnet-4.6`.
- These cases are meant for real desktop environments with the corresponding apps installed and accessibility permissions granted.

## Reporting

After exporting a Harness eval report JSON, summarize success and accuracy with:

```bash
python3 scripts/a11y_quickpaths_report.py <report.json> \
  --output-json docs/reports/a11y_quickpaths_summary.json \
  --output-md docs/reports/a11y_quickpaths_summary.md
```

The summary reports:

- `success_rate`: share of cases with `verification_passed=true`
- `accuracy_rate`: share of cases with `target_hit=true`
- `cache_hit_rate`: share of cases where the runtime reused a warmed structured snapshot
- `fallback_rate`: share of cases where fallback paths were triggered
- latency percentile tables for native runtime and approximate LLM time
- OS breakdown for `macos` and `windows`
