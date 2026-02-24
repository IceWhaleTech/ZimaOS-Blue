/**
 * Format token numbers with locale-aware thousand separators.
 * e.g. 10000 → "10,000" (en) or "10.000" (de)
 */
export function formatTokens(num: number): string {
  return num.toLocaleString()
}
