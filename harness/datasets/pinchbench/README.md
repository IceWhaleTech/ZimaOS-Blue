# PinchBench

This bundle adapts the full local PinchBench task set into an importable Blue Harness dataset bundle.

## Coverage

- Task count: 23
- Source benchmark: official PinchBench markdown tasks
- Run kind: `agent_task`
- Default profile: `pinchbench`
- Quality evaluation: Harness `hybrid` scoring with a configured judge model plus task metadata, workspace summaries, and expected-artifact verification

## Mapping Notes

- Each task prompt is mapped into `input.goal` and `input.prompt`.
- Upstream `workspace_files` are preserved as embedded `metadata.harness_workspace_files`.
- Large source assets such as PDFs and spreadsheets are embedded directly in the manifest so imported datasets remain runnable without the original PinchBench checkout.
- Upstream expected behavior, grading criteria, automated checks, judge rubrics, and grading weights are preserved in item metadata for evaluation context.

## Limitations

- Harness currently evaluates these cases through its native hybrid scorer rather than executing the original PinchBench Python grading code directly.
- The preserved upstream grading assets remain available in metadata for future higher-fidelity scorer work.
