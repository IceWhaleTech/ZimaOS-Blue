/**
 * Format large token numbers with K/M/B suffixes.
 * Returns short string for display; use toLocaleString() for hover tooltip.
 */
export function formatTokens(num: number): string {
  if (num >= 1_000_000_000) return (num / 1_000_000_000).toFixed(1) + 'B'
  if (num >= 1_000_000) return (num / 1_000_000).toFixed(1) + 'M'
  if (num >= 10_000) return (num / 1_000).toFixed(1) + 'K'
  return num.toLocaleString()
}
