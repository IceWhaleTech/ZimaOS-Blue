const TOKEN_KEY = 'token'
const REFRESH_TOKEN_KEY = 'refresh_token'
const PREVIEW_TOKEN_KEY = 'preview_token'

const AUTH_STORAGE_KEYS = [TOKEN_KEY, REFRESH_TOKEN_KEY, PREVIEW_TOKEN_KEY] as const
const COOKIE_PREFIX = 'zima_blue_'
const AUTH_COOKIE_MAX_AGE_SECONDS = 60 * 60 * 24 * 365

function canUseBrowserStorage(): boolean {
  return typeof window !== 'undefined' && typeof document !== 'undefined'
}

function shouldUseCookieMirror(): boolean {
  if (!canUseBrowserStorage()) return false
  const host = String(window.location.hostname || '').trim().toLowerCase()
  return host === 'localhost' || host === '127.0.0.1'
}

function cookieNameFor(key: string): string {
  return `${COOKIE_PREFIX}${key}`
}

function readLocalStorage(key: string): string | null {
  if (!canUseBrowserStorage()) return null
  try {
    return localStorage.getItem(key)
  } catch {
    return null
  }
}

function writeLocalStorage(key: string, value: string): void {
  if (!canUseBrowserStorage()) return
  try {
    localStorage.setItem(key, value)
  } catch {
    // Ignore storage failures in restricted environments.
  }
}

function removeLocalStorage(key: string): void {
  if (!canUseBrowserStorage()) return
  try {
    localStorage.removeItem(key)
  } catch {
    // Ignore storage failures in restricted environments.
  }
}

function readCookie(name: string): string | null {
  if (!shouldUseCookieMirror()) return null

  const prefix = `${name}=`
  for (const part of document.cookie.split(';')) {
    const normalized = part.trim()
    if (!normalized.startsWith(prefix)) continue
    const raw = normalized.slice(prefix.length)
    try {
      const decoded = decodeURIComponent(raw)
      return decoded || null
    } catch {
      return raw || null
    }
  }

  return null
}

function writeCookie(name: string, value: string): void {
  if (!shouldUseCookieMirror()) return
  document.cookie = `${name}=${encodeURIComponent(value)}; Max-Age=${AUTH_COOKIE_MAX_AGE_SECONDS}; Path=/; SameSite=Lax`
}

function clearCookie(name: string): void {
  if (!shouldUseCookieMirror()) return
  document.cookie = `${name}=; Max-Age=0; Path=/; SameSite=Lax`
}

function getStoredValue(key: (typeof AUTH_STORAGE_KEYS)[number]): string | null {
  const localValue = readLocalStorage(key)
  if (localValue) {
    if (readCookie(cookieNameFor(key)) !== localValue) {
      writeCookie(cookieNameFor(key), localValue)
    }
    return localValue
  }

  const cookieValue = readCookie(cookieNameFor(key))
  if (cookieValue) {
    writeLocalStorage(key, cookieValue)
    return cookieValue
  }

  return null
}

function setStoredValue(key: (typeof AUTH_STORAGE_KEYS)[number], value: string): void {
  writeLocalStorage(key, value)
  writeCookie(cookieNameFor(key), value)
}

function clearStoredValue(key: (typeof AUTH_STORAGE_KEYS)[number]): void {
  removeLocalStorage(key)
  clearCookie(cookieNameFor(key))
}

export function syncAuthSessionStorage(): void {
  for (const key of AUTH_STORAGE_KEYS) {
    const localValue = readLocalStorage(key)
    const cookieValue = readCookie(cookieNameFor(key))

    if (localValue) {
      if (cookieValue !== localValue) {
        writeCookie(cookieNameFor(key), localValue)
      }
      continue
    }

    if (cookieValue) {
      writeLocalStorage(key, cookieValue)
    }
  }
}

export function getStoredAccessToken(): string | null {
  return getStoredValue(TOKEN_KEY)
}

export function getStoredRefreshToken(): string | null {
  return getStoredValue(REFRESH_TOKEN_KEY)
}

export function getStoredPreviewToken(): string | null {
  return getStoredValue(PREVIEW_TOKEN_KEY)
}

export function setStoredAccessToken(value: string): void {
  setStoredValue(TOKEN_KEY, value)
}

export function setStoredRefreshToken(value: string): void {
  setStoredValue(REFRESH_TOKEN_KEY, value)
}

export function setStoredPreviewToken(value: string): void {
  setStoredValue(PREVIEW_TOKEN_KEY, value)
}

export function clearStoredAccessToken(): void {
  clearStoredValue(TOKEN_KEY)
}

export function clearStoredRefreshToken(): void {
  clearStoredValue(REFRESH_TOKEN_KEY)
}

export function clearStoredPreviewToken(): void {
  clearStoredValue(PREVIEW_TOKEN_KEY)
}

export function clearStoredAuthSession(): void {
  clearStoredAccessToken()
  clearStoredRefreshToken()
  clearStoredPreviewToken()
}

export function hasStoredSessionHint(): boolean {
  return !!(getStoredAccessToken() || getStoredPreviewToken())
}
