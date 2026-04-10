function toFiniteNumber(value: unknown): number | null {
  const numeric = Number(value)
  return Number.isFinite(numeric) ? numeric : null
}

export function formatHarnessScore(value: unknown, digits = 2): string {
  const numeric = toFiniteNumber(value)
  return numeric == null ? '' : numeric.toFixed(digits)
}

export function formatSignedHarnessScore(value: unknown, digits = 2): string {
  const numeric = toFiniteNumber(value)
  if (numeric == null) return ''
  const label = numeric.toFixed(digits)
  return numeric > 0 ? `+${label}` : label
}
