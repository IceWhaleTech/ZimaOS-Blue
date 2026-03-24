import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  clearStoredAuthSession,
  getStoredAccessToken,
  getStoredRefreshToken,
  getStoredPreviewToken,
  hasStoredSessionHint,
  setStoredAccessToken,
  setStoredPreviewToken,
  setStoredRefreshToken,
  syncAuthSessionStorage,
} from '@/utils/authStorage'

const localStorageMock = (() => {
  let store: Record<string, string> = {}
  return {
    getItem: (key: string) => (key in store ? store[key] : null),
    setItem: (key: string, value: string) => {
      store[key] = String(value)
    },
    removeItem: (key: string) => {
      delete store[key]
    },
    clear: () => {
      store = {}
    },
  }
})()

vi.stubGlobal('localStorage', localStorageMock)

function readCookie(name: string): string | null {
  const prefix = `${name}=`
  for (const part of document.cookie.split(';')) {
    const normalized = part.trim()
    if (!normalized.startsWith(prefix)) continue
    const decoded = decodeURIComponent(normalized.slice(prefix.length))
    return decoded || null
  }
  return null
}

function clearAuthCookies() {
  for (const name of ['zima_blue_token', 'zima_blue_refresh_token', 'zima_blue_preview_token']) {
    document.cookie = `${name}=; Max-Age=0; Path=/; SameSite=Lax`
  }
}

describe('authStorage', () => {
  beforeEach(() => {
    localStorage.clear()
    clearAuthCookies()
  })

  it('stores auth values in both localStorage and cookies', () => {
    setStoredAccessToken('access-token')
    setStoredRefreshToken('refresh-token')
    setStoredPreviewToken('preview-token')

    expect(localStorage.getItem('token')).toBe('access-token')
    expect(localStorage.getItem('refresh_token')).toBe('refresh-token')
    expect(localStorage.getItem('preview_token')).toBe('preview-token')
    expect(readCookie('zima_blue_token')).toBe('access-token')
    expect(readCookie('zima_blue_refresh_token')).toBe('refresh-token')
    expect(readCookie('zima_blue_preview_token')).toBe('preview-token')
  })

  it('hydrates missing localStorage values from cookie fallback', () => {
    document.cookie = 'zima_blue_token=cookie-access; Path=/; SameSite=Lax'
    document.cookie = 'zima_blue_refresh_token=cookie-refresh; Path=/; SameSite=Lax'
    document.cookie = 'zima_blue_preview_token=cookie-preview; Path=/; SameSite=Lax'

    expect(getStoredAccessToken()).toBe('cookie-access')
    expect(getStoredRefreshToken()).toBe('cookie-refresh')
    expect(getStoredPreviewToken()).toBe('cookie-preview')
    expect(localStorage.getItem('token')).toBe('cookie-access')
    expect(localStorage.getItem('refresh_token')).toBe('cookie-refresh')
    expect(localStorage.getItem('preview_token')).toBe('cookie-preview')
    expect(hasStoredSessionHint()).toBe(true)
  })

  it('syncs existing localStorage values back to cookies', () => {
    localStorage.setItem('token', 'local-access')
    localStorage.setItem('refresh_token', 'local-refresh')

    syncAuthSessionStorage()

    expect(readCookie('zima_blue_token')).toBe('local-access')
    expect(readCookie('zima_blue_refresh_token')).toBe('local-refresh')
  })

  it('clears cookies and localStorage together', () => {
    setStoredAccessToken('access-token')
    setStoredRefreshToken('refresh-token')
    setStoredPreviewToken('preview-token')

    clearStoredAuthSession()

    expect(localStorage.getItem('token')).toBeNull()
    expect(localStorage.getItem('refresh_token')).toBeNull()
    expect(localStorage.getItem('preview_token')).toBeNull()
    expect(readCookie('zima_blue_token')).toBeNull()
    expect(readCookie('zima_blue_refresh_token')).toBeNull()
    expect(readCookie('zima_blue_preview_token')).toBeNull()
    expect(hasStoredSessionHint()).toBe(false)
  })
})
