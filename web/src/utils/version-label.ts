export function formatVersionLabel(value?: string | null): string {
  const normalized = value?.trim()
  if (!normalized) return '-'

  if (/^v\d/i.test(normalized)) return normalized
  if (/^\d/.test(normalized)) return `v${normalized}`

  return normalized
}
