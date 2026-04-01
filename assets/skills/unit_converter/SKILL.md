---
name: unit_converter
version: "0.1.0"
description: "Disabled placeholder for unit conversion helpers. ZimaOS Blue does not currently register a dedicated builtin unit_converter skill in the live runtime."
invocation: "blue unit_converter value=1 from=m to=ft"
examples:
  - "blue unit_converter value=1 from=m to=ft"
capability_tags:
  - units
  - conversion
  - calculator
interaction_mode: stateless
card_support: none
enabled: false
category: internal
tags:
  - units
  - conversion
  - calculator
---

# Unit Converter

This skill is currently disabled.

Earlier drafts listed supported unit categories and parameters as if `unit_converter` were a live builtin interface. In the current ZimaOS Blue runtime, there is no dedicated builtin `unit_converter` skill registration behind that contract.

## Use Instead

- Handle straightforward conversions directly in the normal answer flow when no external verification is needed.
- For exact or audited calculations, use a verified external tool, spreadsheet, or shell workflow rather than relying on this placeholder.
- If a real unit conversion skill is added later, document the supported units, rounding behavior, and invocation contract explicitly.
