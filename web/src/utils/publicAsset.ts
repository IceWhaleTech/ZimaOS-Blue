function ensureTrailingSlash(value: string): string {
  return value.endsWith('/') ? value : `${value}/`
}

export function publicAsset(path: string): string {
  const trimmed = String(path || '').trim()
  if (!trimmed) return import.meta.env.BASE_URL || '/'
  if (/^(?:[a-z]+:)?\/\//i.test(trimmed) || trimmed.startsWith('data:')) {
    return trimmed
  }

  const normalized = trimmed.replace(/^\/+/, '')
  const base = ensureTrailingSlash(import.meta.env.BASE_URL || '/')
  return `${base}${normalized}`
}
