const RE_WINDOWS_DRIVE_ABS = /^[a-zA-Z]:[\\/]+/
const RE_WINDOWS_UNC_ABS = /^\\\\[^\\/\s]+[\\/][^\\/\s]+/

function hasUrlScheme(value: string): boolean {
  return /^[a-zA-Z][a-zA-Z0-9+.-]*:\/\//.test(value)
}

export function isApiPath(value: string): boolean {
  const trimmed = value.trim()
  return trimmed.startsWith('/api/')
}

export function isHttpUrl(value: string): boolean {
  const trimmed = value.trim().toLowerCase()
  return trimmed.startsWith('http://') || trimmed.startsWith('https://')
}

export function isLocalAbsolutePath(value: string): boolean {
  const trimmed = value.trim()
  if (!trimmed) return false
  if (isApiPath(trimmed)) return false
  if (hasUrlScheme(trimmed)) return false

  if (RE_WINDOWS_UNC_ABS.test(trimmed)) return true
  if (RE_WINDOWS_DRIVE_ABS.test(trimmed)) return true

  return trimmed.startsWith('/')
}

export function isLoopbackHost(hostname: string): boolean {
  const normalized = hostname
    .trim()
    .toLowerCase()
    .replace(/^\[|\]$/g, '')
  if (!normalized) return false
  if (normalized === 'localhost' || normalized.endsWith('.localhost')) return true
  if (normalized === '::1' || normalized === '0:0:0:0:0:0:0:1') return true
  if (normalized === '127.0.0.1') return true
  return /^127(?:\.\d{1,3}){3}$/.test(normalized)
}

export function isCurrentHostLoopback(): boolean {
  if (typeof window === 'undefined') return false
  return isLoopbackHost(window.location.hostname)
}
