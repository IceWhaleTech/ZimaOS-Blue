import { describe, expect, it, vi } from 'vitest'

import { isAccessTokenExpiredOrExpiring } from './client'

function toBase64Url(value: string): string {
  return btoa(value).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/g, '')
}

function makeToken(payload: Record<string, unknown>): string {
  const header = toBase64Url(JSON.stringify({ alg: 'HS256', typ: 'JWT' }))
  const body = toBase64Url(JSON.stringify(payload))
  return `${header}.${body}.signature`
}

describe('isAccessTokenExpiredOrExpiring', () => {
  it('returns false for tokens that are still valid beyond the skew window', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-03-17T10:00:00.000Z'))

    const token = makeToken({ exp: Math.floor(Date.now() / 1000) + 120 })

    expect(isAccessTokenExpiredOrExpiring(token)).toBe(false)

    vi.useRealTimers()
  })

  it('returns true for tokens that are already expired or about to expire', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-03-17T10:00:00.000Z'))

    const expiringSoonToken = makeToken({ exp: Math.floor((Date.now() + 10_000) / 1000) })
    const expiredToken = makeToken({ exp: Math.floor((Date.now() - 10_000) / 1000) })

    expect(isAccessTokenExpiredOrExpiring(expiringSoonToken)).toBe(true)
    expect(isAccessTokenExpiredOrExpiring(expiredToken)).toBe(true)

    vi.useRealTimers()
  })

  it('treats malformed tokens as not decodable', () => {
    expect(isAccessTokenExpiredOrExpiring('not-a-jwt')).toBe(false)
  })
})
