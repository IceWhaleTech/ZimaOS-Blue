# Tool Calling Notes

- Stable tool names reduce routing drift.
- Tool arguments should remain machine-parseable and compact.
- Tool results should preserve enough evidence for the next round without flooding context.
- Multi-round search should keep a summary trail, not every raw payload.
