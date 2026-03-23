import { authFetch } from '@/api/client'

const PROTECTED_API_PREFIX = '/api/v1/'
const OBJECT_URL_REVOKE_DELAY_MS = 60_000

export function isProtectedResourceUrl(value?: string): boolean {
  const trimmed = String(value || '').trim()
  return trimmed.startsWith(PROTECTED_API_PREFIX)
}

export function scheduleObjectUrlRevoke(objectUrl: string, delayMs = OBJECT_URL_REVOKE_DELAY_MS) {
  if (!objectUrl) return
  window.setTimeout(() => {
    URL.revokeObjectURL(objectUrl)
  }, delayMs)
}

export async function createProtectedObjectUrl(value: string): Promise<string | null> {
  const trimmed = String(value || '').trim()
  if (!trimmed) return null
  if (!isProtectedResourceUrl(trimmed)) return trimmed

  try {
    const resp = await authFetch(trimmed)
    if (!resp.ok) return null
    const blob = await resp.blob()
    return URL.createObjectURL(blob)
  } catch {
    return null
  }
}

export async function openProtectedResource(value: string): Promise<boolean> {
  const trimmed = String(value || '').trim()
  if (!trimmed) return false

  if (!isProtectedResourceUrl(trimmed)) {
    window.open(trimmed, '_blank')
    return true
  }

  const objectUrl = await createProtectedObjectUrl(trimmed)
  if (!objectUrl) return false

  window.open(objectUrl, '_blank')
  scheduleObjectUrlRevoke(objectUrl)
  return true
}

export async function downloadProtectedResource(
  value: string,
  filename?: string
): Promise<boolean> {
  const trimmed = String(value || '').trim()
  if (!trimmed) return false

  let href = trimmed
  if (isProtectedResourceUrl(trimmed)) {
    const objectUrl = await createProtectedObjectUrl(trimmed)
    if (!objectUrl) return false
    href = objectUrl
    scheduleObjectUrlRevoke(objectUrl)
  }

  const link = document.createElement('a')
  link.href = href
  if (filename) {
    link.download = filename
  }
  link.rel = 'noreferrer'
  link.click()
  return true
}
